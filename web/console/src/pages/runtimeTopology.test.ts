import { describe, it, expect } from 'vitest';
import fc from 'fast-check';
import type { RuntimeResource } from '../api';
import {
  buildRuntimeTopology,
  computeRuntimeSummary,
  formatControllerReplicas,
  formatPodReady,
  sumRestartCount,
  truncateMessage,
  normalizePodPhase,
  uncategorizedStatusText,
  compareByCodePoint,
  isControllerKind,
  isPodKind,
  type PodPhase,
} from './runtimeTopology';

const RUNS = { numRuns: 100 } as const;
const REPLICA_MAX = 2147483647;
const CONTROLLER_KINDS = ['Deployment', 'StatefulSet', 'DaemonSet'];

// ---------------------------------------------------------------------------
// 可复用的 RuntimeResource 生成器：覆盖设计文档「Correctness Properties」要求的
// 全部边界（kind 多态、name/parent 大小写差异与星平面字符、status Running 大小写
// 变体与非法值、ready/desired 的 undefined/null/负数/非整数/越界、containers 缺
// restartCount/缺 image、超长/边界长度 message）。
// 部分生成值（如 null、undefined 的必填字段）刻意越出 RuntimeResource 的静态类型，
// 用以验证纯函数对脏数据的归一化，因此最终通过 as 强制转型。
// ---------------------------------------------------------------------------

// kind：控制器三态 + Pod + 其他已知 kind + 随机串 + 空串
const kindArb: fc.Arbitrary<string> = fc.oneof(
  fc.constantFrom('Deployment', 'StatefulSet', 'DaemonSet', 'Pod'),
  fc.constantFrom('ReplicaSet', 'Service', 'ConfigMap', ''),
  fc.string({ maxLength: 6 }),
);

// 名称：空串、ASCII、大小写变体、星平面字符（😀 / 𝔘）以及短随机串。
// 故意使用较小的取值池以提升排序键碰撞概率，便于验证稳定排序的 tiebreaker。
const nameArb: fc.Arbitrary<string> = fc.oneof(
  fc.constant(''),
  fc.constantFrom('api', 'API', 'Api', 'web', 'Web', 'db', 'a', 'A', 'z'),
  fc.constantFrom('😀', '𝔘', '🍎', 'a😀', 'A𝔘', '日志'),
  fc.string({ maxLength: 4 }),
);

// 状态：Running 大小写变体、其他合法相位、非法值、空串、含首尾空白
const statusArb: fc.Arbitrary<string> = fc.oneof(
  fc.constantFrom('Running', 'running', 'RUNNING', 'RuNnInG'),
  fc.constantFrom('Pending', 'Succeeded', 'Failed', 'Unknown'),
  fc.constantFrom('Healthy', 'Degraded', 'CrashLoopBackOff', ''),
  fc.constantFrom(' running ', '  ', 'Running '),
  fc.string({ maxLength: 8 }),
);

// ready / desired：undefined、null、负数、非整数、范围内整数、越界整数、边界值
const replicaArb: fc.Arbitrary<number | null | undefined> = fc.oneof(
  fc.constant(undefined),
  fc.constant(null),
  fc.integer({ min: -50, max: -1 }),
  fc.constantFrom(1.5, 2.7, 0.1, -0.5),
  fc.integer({ min: 0, max: 32 }),
  fc.constant(REPLICA_MAX),
  fc.constantFrom(REPLICA_MAX + 1, 3000000000, 9999999999),
) as fc.Arbitrary<number | null | undefined>;

// 单个容器：restartCount 取 undefined（缺失）或非负整数；image 可有可无
const containerArb = fc.record({
  name: fc.string({ maxLength: 6 }),
  image: fc.option(fc.string({ maxLength: 10 }), { nil: undefined }),
  ready: fc.boolean(),
  restartCount: fc.oneof(fc.constant(undefined), fc.nat({ max: 100 })),
});

// containers：undefined、空数组、含缺 restartCount/缺 image 的容器列表
const containersArb = fc.oneof(
  fc.constant(undefined),
  fc.constant([]),
  fc.array(containerArb, { maxLength: 5 }),
);

// message：短串、跨 200 边界的随机长度、恰好 200/201、远超 200
const messageArb: fc.Arbitrary<string | undefined> = fc.oneof(
  fc.constant(undefined),
  fc.string({ maxLength: 30 }),
  fc.string({ minLength: 195, maxLength: 260 }),
  fc.constant('x'.repeat(200)),
  fc.constant('y'.repeat(201)),
  fc.constant('z'.repeat(500)),
);

// parentKind / parentName：含 undefined、空串、控制器 kind 与其大小写变体、name 取值池
const parentKindArb: fc.Arbitrary<string | undefined> = fc.oneof(
  fc.constant(undefined),
  fc.constant(''),
  fc.constantFrom('Deployment', 'StatefulSet', 'DaemonSet', 'deployment', 'Pod', 'ReplicaSet'),
);
const parentNameArb: fc.Arbitrary<string | undefined> = fc.oneof(
  fc.constant(undefined),
  fc.constant(''),
  nameArb,
);

// 完整 RuntimeResource 生成器（脏数据经 as 转型）
const resourceArb = fc
  .record({
    id: fc.uuid(),
    clusterId: fc.constant('cluster-1'),
    tenantId: fc.constant('tenant-1'),
    applicationId: fc.constant('app-1'),
    stageKey: fc.constant('stage-1'),
    kind: kindArb,
    namespace: fc.constant('default'),
    name: nameArb,
    parentKind: parentKindArb,
    parentName: parentNameArb,
    status: statusArb,
    healthStatus: fc.option(fc.string({ maxLength: 8 }), { nil: undefined }),
    message: messageArb,
    desired: replicaArb,
    ready: replicaArb,
    containers: containersArb,
  })
  .map((r) => r as unknown as RuntimeResource);

const resourcesArb = fc.array(resourceArb, { maxLength: 12 });

// 仅控制器资源（kind 固定为控制器三态），用于副本摘要属性
const controllerResourceArb = resourceArb.map((r) => ({
  ...r,
  kind: 'Deployment',
})) as unknown as fc.Arbitrary<RuntimeResource>;

// 仅 Pod 资源，用于 Pod 相关属性
const podResourceArb = resourceArb.map((r) => ({ ...r, kind: 'Pod' })) as unknown as fc.Arbitrary<RuntimeResource>;

// --- 多重集合（按对象引用计数）辅助 ---
function refCount(list: RuntimeResource[]): Map<RuntimeResource, number> {
  const m = new Map<RuntimeResource, number>();
  list.forEach((r) => m.set(r, (m.get(r) ?? 0) + 1));
  return m;
}

function collectAll(input: RuntimeResource[]): RuntimeResource[] {
  const topo = buildRuntimeTopology(input);
  const out: RuntimeResource[] = [];
  topo.controllers.forEach((g) => {
    out.push(g.controller);
    g.pods.forEach((p) => out.push(p));
  });
  topo.uncategorized.forEach((u) => out.push(u));
  return out;
}

// 期望的副本归一化（与设计文档口径一致：缺失/null/非整数/越界按 0）
function expectedReplica(v: number | null | undefined): number {
  if (v === undefined || v === null) return 0;
  if (!Number.isInteger(v)) return 0;
  if (v < 0 || v > REPLICA_MAX) return 0;
  return v;
}

describe('runtimeTopology property-based tests', () => {
  // Feature: stage-runtime-topology-drawer, Property 1: 分组保留全部资源且每个资源恰好出现一次
  it('Property 1: 分组保留全部资源且每个资源恰好出现一次', () => {
    fc.assert(
      fc.property(resourcesArb, (input) => {
        const out = collectAll(input);
        const inCount = refCount(input);
        const outCount = refCount(out);
        expect(out.length).toBe(input.length);
        expect(outCount.size).toBe(inCount.size);
        inCount.forEach((count, resource) => {
          expect(outCount.get(resource)).toBe(count);
        });
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 2: 资源归类正确性
  it('Property 2: 资源归类正确性', () => {
    fc.assert(
      fc.property(resourcesArb, (input) => {
        const topo = buildRuntimeTopology(input);
        const uncategorizedSet = new Set(topo.uncategorized);
        // 控制器键 → 该键存在的首个控制器组（与实现的「同键取首个」一致）
        const groupByKey = new Map<string, (typeof topo.controllers)[number]>();
        topo.controllers.forEach((g) => {
          const key = `${g.controller.kind}\u0000${g.controller.name}`;
          if (!groupByKey.has(key)) groupByKey.set(key, g);
        });
        // 反查每个 pod 所在的组
        const groupOfPod = new Map<RuntimeResource, (typeof topo.controllers)[number]>();
        topo.controllers.forEach((g) => g.pods.forEach((p) => groupOfPod.set(p, g)));

        input.forEach((r) => {
          if (isControllerKind(r.kind)) {
            // 控制器自身既不在 pods 也不在 uncategorized
            expect(groupOfPod.has(r)).toBe(false);
            expect(uncategorizedSet.has(r)).toBe(false);
            return;
          }
          const pk = r.parentKind;
          const pn = r.parentName;
          const matchKey = pk !== undefined && pk !== '' && pn !== undefined && pn !== '' ? `${pk}\u0000${pn}` : undefined;
          const matchedGroup = matchKey !== undefined ? groupByKey.get(matchKey) : undefined;
          if (isPodKind(r.kind) && matchedGroup) {
            // 归入匹配组，且该组控制器 kind/name 区分大小写等于 parent
            const g = groupOfPod.get(r);
            expect(g).toBe(matchedGroup);
            expect(g!.controller.kind).toBe(pk);
            expect(g!.controller.name).toBe(pn);
            expect(uncategorizedSet.has(r)).toBe(false);
          } else {
            // 其余一律未归类
            expect(uncategorizedSet.has(r)).toBe(true);
            expect(groupOfPod.has(r)).toBe(false);
          }
        });
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 3: 控制器卡片按 name 再 kind 码点升序且稳定排序
  it('Property 3: 控制器卡片按 name 再 kind 码点升序且稳定排序', () => {
    fc.assert(
      fc.property(resourcesArb, (input) => {
        const topo = buildRuntimeTopology(input);
        const controllers = topo.controllers.map((g) => g.controller);
        for (let i = 1; i < controllers.length; i += 1) {
          const prev = controllers[i - 1];
          const cur = controllers[i];
          const byName = compareByCodePoint(prev.name, cur.name);
          expect(byName).toBeLessThanOrEqual(0);
          if (byName === 0) {
            const byKind = compareByCodePoint(prev.kind, cur.kind);
            expect(byKind).toBeLessThanOrEqual(0);
            if (byKind === 0) {
              // 稳定性：name 与 kind 均相等时保持输入相对顺序
              expect(input.indexOf(prev)).toBeLessThan(input.indexOf(cur));
            }
          }
        }
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 4: 兄弟资源按 name 码点升序且稳定排序
  it('Property 4: 兄弟资源按 name 码点升序且稳定排序', () => {
    fc.assert(
      fc.property(resourcesArb, (input) => {
        const topo = buildRuntimeTopology(input);
        const lists: RuntimeResource[][] = [...topo.controllers.map((g) => g.pods), topo.uncategorized];
        lists.forEach((list) => {
          for (let i = 1; i < list.length; i += 1) {
            const prev = list[i - 1];
            const cur = list[i];
            const byName = compareByCodePoint(prev.name, cur.name);
            expect(byName).toBeLessThanOrEqual(0);
            if (byName === 0) {
              // 稳定性：name 相等时保持输入相对顺序
              expect(input.indexOf(prev)).toBeLessThan(input.indexOf(cur));
            }
          }
        });
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 5: 控制器副本摘要归一化
  it('Property 5: 控制器副本摘要归一化', () => {
    fc.assert(
      fc.property(controllerResourceArb, (resource) => {
        const expected = `${expectedReplica(resource.ready)}/${expectedReplica(resource.desired)}`;
        expect(formatControllerReplicas(resource)).toBe(expected);
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 6: Pod 就绪计数格式化与缺失占位
  it('Property 6: Pod 就绪计数格式化与缺失占位', () => {
    fc.assert(
      fc.property(podResourceArb, (resource) => {
        const { ready, desired } = resource;
        const result = formatPodReady(resource);
        if (
          Number.isInteger(ready) &&
          Number.isInteger(desired) &&
          (ready as number) >= 0 &&
          (desired as number) >= 0
        ) {
          expect(result).toBe(`${ready}/${desired}`);
        } else {
          expect(result).toBeNull();
        }
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 7: Pod 重启次数累计等于各容器重启次数之和
  it('Property 7: Pod 重启次数累计等于各容器重启次数之和', () => {
    fc.assert(
      fc.property(podResourceArb, (resource) => {
        const containers = resource.containers ?? [];
        const expected = containers.reduce((acc, c) => acc + (c.restartCount ?? 0), 0);
        const result = sumRestartCount(resource);
        expect(result).toBe(expected);
        expect(Number.isInteger(result)).toBe(true);
        expect(result).toBeGreaterThanOrEqual(0);
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 8: 状态消息超长截断
  it('Property 8: 状态消息超长截断', () => {
    fc.assert(
      fc.property(messageArb, (message) => {
        const result = truncateMessage(message, 200);
        const text = message ?? '';
        if (text.length > 200) {
          expect(result.truncated).toBe(true);
          expect(result.text).toBe(text.slice(0, 200));
          expect(result.text.length).toBe(200);
        } else {
          expect(result.truncated).toBe(false);
          expect(result.text).toBe(text);
        }
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 9: Pod 运行状态归一化为五态之一
  it('Property 9: Pod 运行状态归一化为五态之一', () => {
    const allowed: PodPhase[] = ['Running', 'Pending', 'Succeeded', 'Failed', 'Unknown'];
    fc.assert(
      fc.property(fc.oneof(statusArb, fc.constant(undefined)), (status) => {
        const phase = normalizePodPhase(status);
        expect(allowed).toContain(phase);
        const normalized = (status ?? '').trim().toLowerCase();
        const known: Record<string, PodPhase> = {
          running: 'Running',
          pending: 'Pending',
          succeeded: 'Succeeded',
          failed: 'Failed',
        };
        expect(phase).toBe(known[normalized] ?? 'Unknown');
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 10: 汇总统计计数正确
  it('Property 10: 汇总统计计数正确', () => {
    fc.assert(
      fc.property(resourcesArb, (input) => {
        const summary = computeRuntimeSummary(input);
        const expectedControllers = input.filter((r) => isControllerKind(r.kind)).length;
        const expectedRunning = input.filter(
          (r) => isPodKind(r.kind) && (r.status ?? '').toLowerCase() === 'running',
        ).length;
        expect(summary.controllerTotal).toBe(expectedControllers);
        expect(summary.runningPodCount).toBe(expectedRunning);
        expect(Number.isInteger(summary.controllerTotal)).toBe(true);
        expect(summary.controllerTotal).toBeGreaterThanOrEqual(0);
        expect(Number.isInteger(summary.runningPodCount)).toBe(true);
        expect(summary.runningPodCount).toBeGreaterThanOrEqual(0);
      }),
      RUNS,
    );
  });

  // Feature: stage-runtime-topology-drawer, Property 11: 未归类资源状态占位
  it('Property 11: 未归类资源状态占位', () => {
    const uncategorizedStatusArb: fc.Arbitrary<string | null | undefined> = fc.oneof(
      fc.constant(undefined),
      fc.constant(null),
      fc.constant(''),
      fc.constantFrom('   ', '\t', '\n '),
      statusArb,
    ) as fc.Arbitrary<string | null | undefined>;
    fc.assert(
      fc.property(uncategorizedStatusArb, (status) => {
        const resource = { status } as unknown as RuntimeResource;
        const text = uncategorizedStatusText(resource);
        if (status === undefined || status === null || status.trim() === '') {
          expect(text).toBe('未知');
        } else {
          expect(text).toBe(status);
        }
      }),
      RUNS,
    );
  });
});

// ---------------------------------------------------------------------------
// 3.12 示例 / 边界单元测试（纯 Vitest）
// 覆盖：空快照、单个无父级 Pod、controller 重复同键、parent 大小写不匹配、
// message 恰好 200/201 字符、ready/desired 为 0、status 为空字符串。
// Requirements: 3.7, 4.4, 6.6, 6.8, 8.3, 9.6
// ---------------------------------------------------------------------------

function makeResource(overrides: Partial<RuntimeResource> & { kind: string; name: string }): RuntimeResource {
  return {
    id: overrides.id ?? `id-${Math.random().toString(36).slice(2)}`,
    clusterId: 'c1',
    tenantId: 't1',
    applicationId: 'a1',
    stageKey: 's1',
    namespace: 'default',
    status: 'Running',
    ...overrides,
  } as RuntimeResource;
}

describe('runtimeTopology example / boundary unit tests', () => {
  it('空快照：返回空控制器组与空未归类列表，统计为 0', () => {
    const topo = buildRuntimeTopology([]);
    expect(topo.controllers).toEqual([]);
    expect(topo.uncategorized).toEqual([]);
    const summary = computeRuntimeSummary([]);
    expect(summary.controllerTotal).toBe(0);
    expect(summary.runningPodCount).toBe(0);
  });

  it('单个无父级 Pod：归入未归类且不丢失', () => {
    const pod = makeResource({ kind: 'Pod', name: 'order-api-live', parentKind: undefined, parentName: undefined });
    const topo = buildRuntimeTopology([pod]);
    expect(topo.controllers).toEqual([]);
    expect(topo.uncategorized).toHaveLength(1);
    expect(topo.uncategorized[0]).toBe(pod);
  });

  it('重复同键控制器：各自成卡，Pod 仅归入首个匹配组', () => {
    const c1 = makeResource({ kind: 'Deployment', name: 'web', id: 'c1' });
    const c2 = makeResource({ kind: 'Deployment', name: 'web', id: 'c2' });
    const pod = makeResource({ kind: 'Pod', name: 'web-pod', parentKind: 'Deployment', parentName: 'web', id: 'p1' });
    const topo = buildRuntimeTopology([c1, c2, pod]);
    expect(topo.controllers).toHaveLength(2);
    const podCounts = topo.controllers.map((g) => g.pods.length);
    expect(podCounts).toEqual([1, 0]);
    expect(topo.controllers[0].pods[0]).toBe(pod);
    expect(topo.uncategorized).toHaveLength(0);
  });

  it('parent 大小写不匹配：Pod 归入未归类', () => {
    const controller = makeResource({ kind: 'Deployment', name: 'Web' });
    const pod = makeResource({ kind: 'Pod', name: 'web-pod', parentKind: 'deployment', parentName: 'web' });
    const topo = buildRuntimeTopology([controller, pod]);
    expect(topo.controllers).toHaveLength(1);
    expect(topo.controllers[0].pods).toHaveLength(0);
    expect(topo.uncategorized).toEqual([pod]);
  });

  it('控制器组内无 Pod：pods 为空数组（渲染层据此显示空状态）', () => {
    const controller = makeResource({ kind: 'StatefulSet', name: 'cache' });
    const topo = buildRuntimeTopology([controller]);
    expect(topo.controllers[0].pods).toEqual([]);
  });

  it('message 恰好 200 字符不截断，201 字符截断', () => {
    const exact200 = 'a'.repeat(200);
    const over201 = 'b'.repeat(201);
    expect(truncateMessage(exact200, 200)).toEqual({ text: exact200, truncated: false });
    const r = truncateMessage(over201, 200);
    expect(r.truncated).toBe(true);
    expect(r.text).toBe('b'.repeat(200));
    expect(r.text.length).toBe(200);
  });

  it('ready/desired 为 0：副本与就绪计数显示 0/0', () => {
    const controller = makeResource({ kind: 'Deployment', name: 'zero', ready: 0, desired: 0 });
    expect(formatControllerReplicas(controller)).toBe('0/0');
    const pod = makeResource({ kind: 'Pod', name: 'zero-pod', ready: 0, desired: 0 });
    expect(formatPodReady(pod)).toBe('0/0');
  });

  it('ready/desired 缺失：副本按 0，就绪计数返回 null', () => {
    const controller = makeResource({ kind: 'Deployment', name: 'missing', ready: undefined, desired: undefined });
    expect(formatControllerReplicas(controller)).toBe('0/0');
    const pod = makeResource({ kind: 'Pod', name: 'missing-pod', ready: undefined, desired: undefined });
    expect(formatPodReady(pod)).toBeNull();
  });

  it('status 为空字符串：未归类状态占位为「未知」，相位归一化为 Unknown', () => {
    const resource = makeResource({ kind: 'Service', name: 'svc', status: '' });
    expect(uncategorizedStatusText(resource)).toBe('未知');
    expect(normalizePodPhase('')).toBe('Unknown');
    expect(normalizePodPhase(undefined)).toBe('Unknown');
  });

  it('summary 计算为 0 时返回数字 0 而非占位', () => {
    const pod = makeResource({ kind: 'Pod', name: 'p', status: 'Pending' });
    const summary = computeRuntimeSummary([pod]);
    expect(summary.controllerTotal).toBe(0);
    expect(summary.runningPodCount).toBe(0);
  });
});
