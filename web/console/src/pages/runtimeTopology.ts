import { type RuntimeResource } from '../api';

// 控制器组：一个控制器（Deployment/StatefulSet/DaemonSet）及其归属的 Pod 列表
export type ControllerGroup = {
  controller: RuntimeResource;
  pods: RuntimeResource[];
};

// 运行资源拓扑：已排序的控制器组列表 + 未归类资源列表
export type RuntimeTopology = {
  controllers: ControllerGroup[];
  uncategorized: RuntimeResource[];
};

// 汇总统计：控制器总数与运行中 Pod 数
export type RuntimeSummary = {
  controllerTotal: number;
  runningPodCount: number;
};

// Pod 运行状态归一化后的五种取值之一
export type PodPhase = 'Running' | 'Pending' | 'Succeeded' | 'Failed' | 'Unknown';

const CONTROLLER_KINDS = ['Deployment', 'StatefulSet', 'DaemonSet'];

// 控制器判定：kind 为 Deployment/StatefulSet/DaemonSet 之一
// 语义与 PromotionPage.tsx 中既有的 isRestartableRuntimeKind 一致
export function isControllerKind(kind?: string): boolean {
  return kind !== undefined && CONTROLLER_KINDS.includes(kind);
}

// Pod 判定：kind 精确等于 'Pod'
export function isPodKind(kind?: string): boolean {
  return kind === 'Pod';
}

// 按 Unicode 码点（而非 UTF-16 码元）逐位比较两个字符串，正确排序星平面字符
// 返回负数表示 a < b，0 表示相等，正数表示 a > b
export function compareByCodePoint(a: string, b: string): number {
  const ca = Array.from(a);
  const cb = Array.from(b);
  const len = Math.min(ca.length, cb.length);
  for (let i = 0; i < len; i += 1) {
    const pa = ca[i].codePointAt(0) ?? 0;
    const pb = cb[i].codePointAt(0) ?? 0;
    if (pa !== pb) return pa < pb ? -1 : 1;
  }
  if (ca.length === cb.length) return 0;
  return ca.length < cb.length ? -1 : 1;
}

// 构建控制器索引键：以 \u0000 分隔 kind 与 name，避免拼接歧义
function controllerKey(kind: string, name: string): string {
  return `${kind}\u0000${name}`;
}

// 携带原始输入索引的装饰项，用于「装饰-排序-还原」实现稳定排序
type Indexed = { resource: RuntimeResource; index: number };

// 将快照划分为「控制器组」与「未归类资源」，并进行稳定排序。
//
// 分组规则（区分大小写）：
//  - kind 为 Deployment/StatefulSet/DaemonSet 的资源各自成为一个控制器组（每个资源实例一张卡片，
//    即使两个控制器拥有相同 kind+name 也各自成组）。
//  - Pod 当且仅当其 parentKind 与 parentName 区分大小写精确等于某控制器的 kind 与 name 时归入该组；
//    同键控制器以首个为匹配目标。未匹配的 Pod 归入未归类。
//  - 其他（既非控制器也非 Pod）资源全部归入未归类。
//
// 排序（均不依赖引擎 Array.sort 稳定性，统一携带原始索引作为最终 tiebreaker）：
//  - 控制器：name 码点升序 → kind 码点升序 → 原始输入顺序。
//  - 组内 Pod：name 码点升序 → 原始输入顺序。
//  - 未归类：name 码点升序 → 原始输入顺序。
//
// 不变量：输出中「全部控制器」∪「全部组内 Pod」∪「全部未归类」作为多重集合恰好等于输入快照，
// 不丢弃、不重复、不臆造任何资源。
export function buildRuntimeTopology(resources: RuntimeResource[]): RuntimeTopology {
  const controllers: Indexed[] = [];
  const pods: Indexed[] = [];
  const others: Indexed[] = [];

  resources.forEach((resource, index) => {
    if (isControllerKind(resource.kind)) {
      controllers.push({ resource, index });
    } else if (isPodKind(resource.kind)) {
      pods.push({ resource, index });
    } else {
      others.push({ resource, index });
    }
  });

  // 控制器排序：name 码点升序 → kind 码点升序 → 原始输入顺序
  const sortedControllers = [...controllers].sort((a, b) => {
    const byName = compareByCodePoint(a.resource.name, b.resource.name);
    if (byName !== 0) return byName;
    const byKind = compareByCodePoint(a.resource.kind, b.resource.kind);
    if (byKind !== 0) return byKind;
    return a.index - b.index;
  });

  // 每个控制器资源各自成组，保持排序后的顺序
  const groups: ControllerGroup[] = sortedControllers.map(({ resource }) => ({
    controller: resource,
    pods: [],
  }));

  // 构建键 → 首个控制器组 的索引，用于 Pod 匹配（同键保留首个）
  const groupByKey = new Map<string, ControllerGroup>();
  groups.forEach((group) => {
    const key = controllerKey(group.controller.kind, group.controller.name);
    if (!groupByKey.has(key)) {
      groupByKey.set(key, group);
    }
  });

  // 组内 Pod 与未归类 Pod 的装饰项收集
  const groupPodEntries = new Map<ControllerGroup, Indexed[]>();
  const uncategorized: Indexed[] = [...others];

  pods.forEach((entry) => {
    const { parentKind, parentName } = entry.resource;
    const matched =
      parentKind !== undefined && parentName !== undefined && parentKind !== '' && parentName !== ''
        ? groupByKey.get(controllerKey(parentKind, parentName))
        : undefined;
    if (matched) {
      const list = groupPodEntries.get(matched);
      if (list) {
        list.push(entry);
      } else {
        groupPodEntries.set(matched, [entry]);
      }
    } else {
      uncategorized.push(entry);
    }
  });

  // 组内 Pod 排序：name 码点升序 → 原始输入顺序
  groups.forEach((group) => {
    const entries = groupPodEntries.get(group) ?? [];
    group.pods = [...entries]
      .sort((a, b) => {
        const byName = compareByCodePoint(a.resource.name, b.resource.name);
        if (byName !== 0) return byName;
        return a.index - b.index;
      })
      .map((entry) => entry.resource);
  });

  // 未归类排序：name 码点升序 → 原始输入顺序
  const sortedUncategorized = [...uncategorized]
    .sort((a, b) => {
      const byName = compareByCodePoint(a.resource.name, b.resource.name);
      if (byName !== 0) return byName;
      return a.index - b.index;
    })
    .map((entry) => entry.resource);

  return { controllers: groups, uncategorized: sortedUncategorized };
}

// 副本数上界：Kubernetes 副本字段使用 int32，故上界取 2147483647
const REPLICA_MAX = 2147483647;

// 将副本相关数值归一化为有效非负整数：缺失、null、非整数或越界（非 0..2147483647）一律按 0 计。
function normalizeReplicaValue(value?: number | null): number {
  if (value === undefined || value === null) return 0;
  if (!Number.isInteger(value)) return 0;
  if (value < 0 || value > REPLICA_MAX) return 0;
  return value;
}

// 汇总统计：
//  - controllerTotal：快照中控制器（isControllerKind 为真）的数量。
//  - runningPodCount：快照中 kind === 'Pod' 且 status 大小写无关等于 'Running' 的资源数量。
// 两者均为大于等于 0 的整数。
export function computeRuntimeSummary(resources: RuntimeResource[]): RuntimeSummary {
  let controllerTotal = 0;
  let runningPodCount = 0;
  resources.forEach((resource) => {
    if (isControllerKind(resource.kind)) {
      controllerTotal += 1;
    } else if (isPodKind(resource.kind) && (resource.status ?? '').toLowerCase() === 'running') {
      runningPodCount += 1;
    }
  });
  return { controllerTotal, runningPodCount };
}

// 控制器副本摘要：返回 "{ready}/{desired}"。
// 对 ready/desired 分别归一化：缺失、null、非整数或越界按 0，否则取其整数值。
export function formatControllerReplicas(resource: RuntimeResource): string {
  const ready = normalizeReplicaValue(resource.ready);
  const desired = normalizeReplicaValue(resource.desired);
  return `${ready}/${desired}`;
}

// Pod 就绪计数：当 ready 与 desired 均为大于等于 0 的整数时返回 "{ready}/{desired}"；
// 任一缺失或非法（非整数/为负）时返回 null（渲染层据此显示统一占位符「-」）。
export function formatPodReady(resource: RuntimeResource): string | null {
  const { ready, desired } = resource;
  if (!Number.isInteger(ready) || !Number.isInteger(desired)) return null;
  if ((ready as number) < 0 || (desired as number) < 0) return null;
  return `${ready}/${desired}`;
}

// Pod 重启累计：累加 containers[].restartCount；某容器缺少 restartCount 按 0 计；
// containers 缺失或为空返回 0。结果恒为大于等于 0 的整数。
export function sumRestartCount(resource: RuntimeResource): number {
  const containers = resource.containers;
  if (!containers || containers.length === 0) return 0;
  let total = 0;
  containers.forEach((container) => {
    const count = container.restartCount;
    if (Number.isInteger(count) && count >= 0) {
      total += count;
    }
  });
  return total;
}

// 状态消息截断：message 缺失视为空字符串；当长度大于 limit 时返回前 limit 个字符且 truncated=true；
// 否则返回原文本且 truncated=false。使用普通字符串 length/slice，与属性测试的长度校验口径一致。
export function truncateMessage(
  message?: string,
  limit = 200,
): { text: string; truncated: boolean } {
  const text = message ?? '';
  if (text.length > limit) {
    return { text: text.slice(0, limit), truncated: true };
  }
  return { text, truncated: false };
}

// Pod 运行状态归一化：大小写无关地映射到 Running/Pending/Succeeded/Failed/Unknown 之一；
// 缺失、空字符串或不在已知四态中的取值一律归为 Unknown。
export function normalizePodPhase(status?: string): PodPhase {
  switch ((status ?? '').trim().toLowerCase()) {
    case 'running':
      return 'Running';
    case 'pending':
      return 'Pending';
    case 'succeeded':
      return 'Succeeded';
    case 'failed':
      return 'Failed';
    default:
      return 'Unknown';
  }
}

// 未归类资源状态展示：status 缺失或去除首尾空白后为空字符串时返回「未知」，否则返回原始状态值。
export function uncategorizedStatusText(resource: RuntimeResource): string {
  const status = resource.status;
  if (status === undefined || status === null || status.trim() === '') {
    return '未知';
  }
  return status;
}
