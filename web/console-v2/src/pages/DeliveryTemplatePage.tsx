import { useEffect, useMemo, useState, type Dispatch, type SetStateAction } from 'react';
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
  type Connection,
  type Edge,
  type Node,
  type NodeProps
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  AlertTriangle,
  CheckCircle2,
  Link2,
  Plus,
  Save,
  Settings2,
  Trash2
} from 'lucide-react';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { usePlatformSelection } from '../contexts/PlatformSelectionContext';
import {
  deliveryFlowTemplates,
  type DeliveryTemplateStage
} from '../data/mock';
import {
  loadDeliveryTemplate,
  saveDeliveryTemplateGraph,
  saveStageClusterBinding,
  type DeliveryTemplate,
  type DeliveryTemplateSource,
  type StageClusterBinding
} from '../api/delivery';
import { loadClusters, type Cluster } from '../api/clusters';
import { cn } from '../lib/utils';

const COLUMN_WIDTH = 290;
const ROW_HEIGHT = 178;

const roleOptions = [
  { value: 'tenant_admin', label: '租户管理员' },
  { value: 'developer', label: '开发人员' },
  { value: 'operator', label: '运维人员' },
  { value: 'prod_approver', label: '生产审批人' }
];

const stagePalette = {
  'geek-blue': '#0072B2',
  'sky-blue': '#56B4E9',
  'mint-green': '#009E73',
  turquoise: '#44AA99',
  'lemon-yellow': '#F0E442',
  'amber-orange': '#E69F00',
  'rust-red': '#D55E00',
  'wine-purple': '#882255',
  lilac: '#CC79A7',
  'smoke-blue': '#77AADD'
} as const;

type StageColorToken = keyof typeof stagePalette;

type LocalStage = DeliveryTemplateStage & {
  clientId: string;
  originalStageKey?: string;
};

type LocalEdge = {
  sourceId: string;
  targetId: string;
  rule: string;
};

type StageNodeData = {
  stage: LocalStage;
  selected: boolean;
  bindingLabel: string;
  onSelect: (stageId: string) => void;
};

type StageTemplateNode = Node<StageNodeData, 'stageTemplate'>;

const nodeTypes = {
  stageTemplate: StageTemplateNodeCard
};

export function DeliveryTemplatePage() {
  return (
    <ReactFlowProvider>
      <DeliveryTemplateContent />
    </ReactFlowProvider>
  );
}

function DeliveryTemplateContent() {
  const { currentTenant } = usePlatformSelection();
  const initialTemplate = mockTemplateForTenant(currentTenant.id);

  const [sourceTemplate, setSourceTemplate] = useState<DeliveryTemplate>(initialTemplate);
  const [templateSource, setTemplateSource] = useState<DeliveryTemplateSource>('mock');
  const [templateLoading, setTemplateLoading] = useState(false);
  const [templateError, setTemplateError] = useState('');
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [stages, setStages] = useState<LocalStage[]>(() => toLocalStages(initialTemplate));
  const [edges, setEdges] = useState<LocalEdge[]>(() => toLocalEdges(initialTemplate, toLocalStages(initialTemplate)));
  const [selectedStageId, setSelectedStageId] = useState(stages[0]?.clientId || '');
  const [selectedEdgeId, setSelectedEdgeId] = useState('');
  const [saveMessage, setSaveMessage] = useState('');
  const [saving, setSaving] = useState(false);
  const [deletedStageKeys, setDeletedStageKeys] = useState<string[]>([]);

  const selectedStage = stages.find((stage) => stage.clientId === selectedStageId) || stages[0];
  const selectedEdge = edges.find((edge) => edgeKey(edge) === selectedEdgeId);
  const validation = useMemo(() => validateTemplate(stages, edges), [stages, edges]);
  const stagesById = useMemo(() => new Map(stages.map((stage) => [stage.clientId, stage])), [stages]);
  const stageList = useMemo(
    () => stages.slice().sort((a, b) => a.layoutColumn - b.layoutColumn || a.layoutRow - b.layoutRow),
    [stages]
  );
  const templateEdges = useMemo(
    () => edges.map((edge) => ({
      fromStageKey: stagesById.get(edge.sourceId)?.stageKey || '',
      toStageKey: stagesById.get(edge.targetId)?.stageKey || '',
      rule: edge.rule
    })).filter((edge) => edge.fromStageKey && edge.toStageKey),
    [edges, stagesById]
  );

  useEffect(() => {
    let cancelled = false;
    const tenantId = currentTenant.id;
    setTemplateLoading(true);
    setTemplateError('');
    setSaveMessage('');

    Promise.all([loadDeliveryTemplate(tenantId), loadClusters(tenantId)])
      .then(([templateResult, clusterResult]) => {
        if (cancelled) return;
        setSourceTemplate(templateResult.template);
        setTemplateSource(templateResult.source);
        setTemplateError(templateResult.error || clusterResult.error || '');
        setClusters(clusterResult.clusters);
        hydrateTemplate(templateResult.template);
      })
      .catch((error) => {
        if (cancelled) return;
        const fallback = mockTemplateForTenant(tenantId);
        setSourceTemplate(fallback);
        setTemplateSource('mock');
        setTemplateError(error instanceof Error ? error.message : '交付模板加载失败，已回退 mock 数据');
        hydrateTemplate(fallback);
      })
      .finally(() => {
        if (!cancelled) setTemplateLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [currentTenant.id]);

  const nodes = useMemo<StageTemplateNode[]>(
    () => stages.map((stage) => ({
      id: stage.clientId,
      type: 'stageTemplate',
      position: {
        x: stage.layoutColumn * COLUMN_WIDTH,
        y: stage.layoutRow * ROW_HEIGHT
      },
      data: {
        stage,
        selected: stage.clientId === selectedStageId,
        bindingLabel: clusterBindingLabel(stage, clusters),
        onSelect: setSelectedStageId
      }
    })),
    [clusters, selectedStageId, stages]
  );

  const flowEdges = useMemo<Edge[]>(
    () => edges.map((edge) => {
      const id = edgeKey(edge);
      const selected = selectedEdgeId === id;
      return {
        id,
        source: edge.sourceId,
        target: edge.targetId,
        type: 'default',
        markerEnd: { type: MarkerType.ArrowClosed, width: 12, height: 12 },
        className: cn('template-flow-edge', selected && 'template-flow-edge-selected'),
        selected,
        interactionWidth: 18
      };
    }),
    [edges, selectedEdgeId]
  );

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (!selectedEdgeId || (event.key !== 'Delete' && event.key !== 'Backspace')) return;
      if (isEditableTarget(event.target)) return;
      event.preventDefault();
      deleteEdge(selectedEdgeId);
    }

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [selectedEdgeId]);

  function onConnect(connection: Connection) {
    if (!connection.source || !connection.target || connection.source === connection.target) return;
    const nextEdge: LocalEdge = { sourceId: connection.source, targetId: connection.target, rule: '手动晋级' };
    setEdges((current) => {
      if (current.some((edge) => edge.sourceId === nextEdge.sourceId && edge.targetId === nextEdge.targetId)) return current;
      const next = [...current, nextEdge];
      if (hasCycle(stages.map((stage) => stage.clientId), next)) return current;
      return next;
    });
  }

  function updateSelectedStage(patch: Partial<LocalStage>) {
    if (!selectedStage) return;
    setStages((current) => current.map((stage) => stage.clientId === selectedStage.clientId ? { ...stage, ...patch } : stage));
  }

  function addStage() {
    const column = Math.max(-1, ...stages.map((stage) => stage.layoutColumn)) + 1;
    const stage: LocalStage = {
      id: '',
      clientId: `local-stage-${Date.now()}`,
      stageKey: nextStageKey(stages),
      displayName: '新 Stage',
      description: '新增交付阶段，请补充准入规则和集群绑定。',
      colorToken: 'sky-blue',
      order: stages.length + 1,
      layoutColumn: column,
      layoutRow: 1,
      status: 'enabled',
      requiresApproval: false,
      requiresVerification: false,
      approveRoles: [],
      verifyRoles: [],
      clusterBinding: null
    };
    setStages((current) => [...current, stage]);
    setSelectedStageId(stage.clientId);
  }

  function deleteStage(stageId: string) {
    deleteStages([stageId]);
  }

  function deleteStages(stageIds: string[]) {
    const stageIdSet = new Set(stageIds);
    if (stageIdSet.size === 0) return;

    const removedStages = stages.filter((stage) => stageIdSet.has(stage.clientId));
    const removedOriginalKeys = removedStages
      .map((stage) => stage.originalStageKey)
      .filter((stageKey): stageKey is string => Boolean(stageKey));
    if (removedOriginalKeys.length > 0) {
      setDeletedStageKeys((current) => Array.from(new Set([...current, ...removedOriginalKeys])));
    }

    const remainingStages = stages.filter((stage) => !stageIdSet.has(stage.clientId));
    setStages(remainingStages);
    setEdges((current) => current.filter((edge) => !stageIdSet.has(edge.sourceId) && !stageIdSet.has(edge.targetId)));
    setSelectedStageId((current) => stageIdSet.has(current) ? remainingStages[0]?.clientId || '' : current);
    setSelectedEdgeId('');
  }

  function deleteEdge(edgeId: string) {
    setEdges((current) => current.filter((edge) => edgeKey(edge) !== edgeId));
    setSelectedEdgeId('');
  }

  function handleEdgesDelete(deleted: Edge[]) {
    const deletedIds = new Set(deleted.map((edge) => edge.id));
    setEdges((current) => current.filter((edge) => !deletedIds.has(edgeKey(edge))));
    setSelectedEdgeId((current) => deletedIds.has(current) ? '' : current);
  }

  function handleNodesDelete(deleted: Node[]) {
    deleteStages(deleted.map((node) => node.id));
  }

  function saveTemplate() {
    if (validation.status === 'danger') {
      setSaveMessage(validation.message);
      return;
    }
    if (templateEdges.length !== edges.length) {
      setSaveMessage('存在无效依赖边，请删除后重新保存');
      return;
    }
    const nextTemplate: DeliveryTemplate = {
      ...sourceTemplate,
      tenantId: currentTenant.id,
      stages: stages.map(({ clientId, originalStageKey, ...stage }, index) => ({ ...stage, order: index + 1 })),
      edges: templateEdges
    };
    const renamedKeys = stages
      .filter((s) => s.originalStageKey && s.stageKey !== s.originalStageKey)
      .map((s) => s.originalStageKey!);
    const allDeletedKeys = Array.from(new Set([...deletedStageKeys, ...renamedKeys]));
    setSaving(true);
    saveDeliveryTemplateGraph(currentTenant.id, nextTemplate, allDeletedKeys)
      .then(async (result) => {
        if (result.source === 'api') {
          await Promise.all(stages.map((stage) => saveStageClusterBinding(currentTenant.id, stage.stageKey, resolveStageClusterBinding(stage, clusters))));
        }
        setSourceTemplate(result.template);
        setTemplateSource(result.source);
        setSaveMessage(`模板已保存：${stages.length} 个 Stage，${templateEdges.length} 条依赖边，版本 ${result.template.version}`);
        setDeletedStageKeys([]);
      })
      .catch((error) => {
        setSaveMessage(error instanceof Error ? error.message : '保存模板失败');
      })
      .finally(() => setSaving(false));
  }

  function hydrateTemplate(template: DeliveryTemplate) {
    const nextStages = toLocalStages(template);
    setStages(nextStages);
    setEdges(toLocalEdges(template, nextStages));
    setDeletedStageKeys([]);
    setSelectedStageId((current) => nextStages.some((stage) => stage.clientId === current) ? current : nextStages[0]?.clientId || '');
    setSelectedEdgeId('');
  }

  return (
    <div className="flex min-h-[calc(100vh-96px)] flex-col gap-4">
      <section className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="dense-label">租户级交付流模板</div>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">{sourceTemplate.name}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            模板生成部署页的 Stage DAG 投影；应用只消费拓扑版本、节点、依赖边和 Stage 策略。
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={addStage}>
            <Plus className="h-4 w-4" />
            添加 Stage
          </Button>
          <Button onClick={saveTemplate} disabled={saving || templateLoading}>
            <Save className="h-4 w-4" />
            {saving ? '保存中' : '保存模板'}
          </Button>
        </div>
      </section>
      {(templateLoading || templateError) && (
        <div className="rounded-md border bg-muted/35 px-3 py-2 text-sm text-muted-foreground">
          {templateLoading ? '正在加载交付模板...' : templateError}
        </div>
      )}

      <div className="grid min-h-0 flex-1 gap-4 xl:grid-cols-[300px_minmax(0,1fr)_340px]">
        <aside className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>模板状态</CardTitle>
              <CardDescription>{currentTenant.name} · {sourceTemplate.version} · {templateSource === 'api' ? '后端' : 'Mock'}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <MetricRow label="生效应用" value={`${sourceTemplate.effectiveApps} 个`} />
              <MetricRow label="Stage" value={`${stages.length} 个`} />
              <MetricRow label="依赖边" value={`${templateEdges.length} 条`} />
              <MetricRow label="最后更新" value={sourceTemplate.updatedAt} />
              <MetricRow label="数据来源" value={templateSource === 'api' ? '后端接口' : 'Mock 回退'} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>发布约束</CardTitle>
              <CardDescription>保存前必须满足这些约束。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <CheckItem ok={validation.status !== 'danger'} text={validation.message} />
              <CheckItem ok={stages.every((stage) => stage.clusterBinding)} text="每个启用 Stage 绑定一个集群" />
              <CheckItem ok={stages.some((stage) => stage.requiresApproval)} text="生产链路配置审批角色" />
              <CheckItem ok={stages.some((stage) => stage.requiresVerification)} text="至少一个 Stage 配置人工验证" />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Stage 列表</CardTitle>
              <CardDescription>点击定位到右侧属性面板。</CardDescription>
            </CardHeader>
            <CardContent className="max-h-[360px] space-y-2 overflow-y-auto pr-1">
              {stageList.length === 0 && (
                <div className="rounded-md border border-dashed bg-muted/30 px-3 py-6 text-center text-sm text-muted-foreground">
                  暂无 Stage，请从右上角添加。
                </div>
              )}
              {stageList.map((stage) => (
                  <button
                    key={stage.clientId}
                    type="button"
                    onClick={() => setSelectedStageId(stage.clientId)}
                    className={cn(
                      'flex w-full items-center gap-2 rounded-md border bg-card p-2 text-left text-sm transition-colors hover:border-primary/50',
                      selectedStageId === stage.clientId && 'border-primary bg-accent'
                    )}
                  >
                    <span className="h-8 w-1.5 rounded-full" style={{ backgroundColor: stageColor(stage) }} />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate font-medium">{stage.displayName}</span>
                      <span className="mono block text-xs text-muted-foreground">{stage.stageKey}</span>
                    </span>
                    <span className="flex shrink-0 gap-1">
                      {stage.requiresApproval && <PolicyBadge>审批</PolicyBadge>}
                      {stage.requiresVerification && <PolicyBadge>验证</PolicyBadge>}
                    </span>
                  </button>
                ))}
            </CardContent>
          </Card>
        </aside>

        <Card className="min-h-[640px] overflow-hidden">
          <CardHeader className="flex-row items-center justify-between space-y-0 border-b">
            <div>
              <CardTitle>Stage DAG 画布</CardTitle>
              <CardDescription>拖动 Stage 调整固定槽位；从右侧 Handle 连到下游 Stage。</CardDescription>
            </div>
            <div className="flex items-center gap-2">
              {selectedEdge && (
                <div className="flex items-center gap-2 rounded-md border bg-accent px-2 py-1 text-xs text-accent-foreground">
                  <span className="mono">
                    已选依赖：{stagesById.get(selectedEdge.sourceId)?.displayName || selectedEdge.sourceId}
                    {' -> '}
                    {stagesById.get(selectedEdge.targetId)?.displayName || selectedEdge.targetId}
                  </span>
                  <Button variant="outline" size="sm" onClick={() => deleteEdge(edgeKey(selectedEdge))}>
                    <Trash2 className="h-3.5 w-3.5" />
                    删除连线
                  </Button>
                </div>
              )}
              <Badge variant="outline">行列槽位</Badge>
              <Badge variant="outline">无环校验</Badge>
            </div>
          </CardHeader>
          <CardContent className="h-[calc(100%-73px)] p-0">
            <ReactFlow
              className="template-flow"
              nodes={nodes}
              edges={flowEdges}
              nodeTypes={nodeTypes}
              fitView
              fitViewOptions={{ padding: 0.18 }}
              minZoom={0.45}
              maxZoom={1.7}
              onConnect={onConnect}
              onEdgeClick={(event, edge) => {
                event.stopPropagation();
                setSelectedEdgeId(edge.id);
              }}
              onNodeClick={(_, node) => {
                setSelectedEdgeId('');
                setSelectedStageId(node.id);
              }}
              onPaneClick={() => setSelectedEdgeId('')}
              onNodeDragStop={(_, node) => updateStageSlot(node.id, node.position, stages, setStages)}
              onNodesDelete={handleNodesDelete}
              onEdgesDelete={handleEdgesDelete}
            >
              <Background gap={22} size={1} />
              <MiniMap pannable zoomable nodeStrokeWidth={3} />
              <Controls />
            </ReactFlow>
          </CardContent>
        </Card>

        <StageInspector
          stage={selectedStage}
          clusters={clusters}
          saveMessage={saveMessage}
          onChange={updateSelectedStage}
          onDelete={() => selectedStage && deleteStage(selectedStage.clientId)}
        />
      </div>
    </div>
  );
}

function StageTemplateNodeCard({ data }: NodeProps<StageTemplateNode>) {
  const { stage, selected, bindingLabel, onSelect } = data;
  return (
    <div
      className={cn(
        'h-[166px] w-[238px] overflow-hidden rounded-lg border bg-card shadow-control transition-colors',
        selected ? 'border-primary ring-2 ring-primary/20' : 'border-border'
      )}
      onClick={() => onSelect(stage.clientId)}
    >
      <Handle type="target" position={Position.Left} className="!h-3 !w-3 !border-2 !border-card !bg-primary" />
      <div className="flex h-11 items-center justify-between px-3 text-white" style={{ backgroundColor: stageColor(stage) }}>
        <span className="truncate text-sm font-semibold">{stage.displayName || '新 Stage'}</span>
        <span className="mono rounded bg-white/18 px-1.5 py-0.5 text-[11px]">{stage.stageKey || '-'}</span>
      </div>
      <div className="flex h-[122px] flex-col gap-2 p-3">
        <p className="line-clamp-2 min-h-[32px] text-xs leading-4 text-muted-foreground">{stage.description || '未填写描述'}</p>
        <div className="flex h-5 flex-wrap gap-1 overflow-hidden">
          {stage.requiresApproval && <PolicyBadge>审批</PolicyBadge>}
          {stage.requiresVerification && <PolicyBadge>验证</PolicyBadge>}
          {stage.status === 'disabled' && <Badge variant="muted">禁用</Badge>}
        </div>
        <div className="mt-auto flex items-center gap-1.5 rounded border bg-muted/40 px-2 py-1 text-xs text-muted-foreground">
          <Link2 className="h-3.5 w-3.5" />
          <span className="truncate">{bindingLabel}</span>
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!h-3 !w-3 !border-2 !border-card !bg-primary" />
    </div>
  );
}

function StageInspector({
  stage,
  clusters,
  saveMessage,
  onChange,
  onDelete
}: {
  stage?: LocalStage;
  clusters: Cluster[];
  saveMessage: string;
  onChange: (patch: Partial<LocalStage>) => void;
  onDelete: () => void;
}) {
  if (!stage) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Stage 属性</CardTitle>
          <CardDescription>请选择一个 Stage。</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <aside className="min-h-0 space-y-4 overflow-y-auto pr-1">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Settings2 className="h-4 w-4" />
            Stage 属性
          </CardTitle>
          <CardDescription>修改后需要保存模板才会生成新的拓扑版本。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <LabelledInput label="Stage key" value={stage.stageKey} onChange={(stageKey) => onChange({ stageKey: normalizeStageKeyInput(stageKey) })} />
          <LabelledInput label="显示名" value={stage.displayName} onChange={(displayName) => onChange({ displayName })} />
          <LabelledInput label="描述" value={stage.description} onChange={(description) => onChange({ description })} />

          <label className="space-y-1.5 text-sm">
            <span className="font-medium">环境色</span>
            <select
              value={stage.colorToken}
              onChange={(event) => onChange({ colorToken: event.target.value })}
              className="h-9 w-full rounded-md border bg-card px-2 text-sm shadow-control"
            >
              {Object.entries(stagePalette).map(([token, color]) => (
                <option key={token} value={token}>{token} · {color}</option>
              ))}
            </select>
          </label>

          <div className="grid grid-cols-2 gap-2">
            <ToggleRow label="部署前审批" checked={stage.requiresApproval} onChange={(requiresApproval) => onChange({ requiresApproval })} />
            <ToggleRow label="部署后验证" checked={stage.requiresVerification} onChange={(requiresVerification) => onChange({ requiresVerification })} />
          </div>

          <RoleSelector label="审批角色" values={stage.approveRoles} onChange={(approveRoles) => onChange({ approveRoles })} />
          <RoleSelector label="验证角色" values={stage.verifyRoles} onChange={(verifyRoles) => onChange({ verifyRoles })} />

          <label className="space-y-1.5 text-sm">
            <span className="font-medium">绑定集群</span>
            <select
              value={stage.clusterBinding?.clusterId || ''}
              onChange={(event) => {
                const cluster = clusters.find((item) => item.id === event.target.value);
                onChange({
                  clusterBinding: cluster
                    ? { clusterId: cluster.id, clusterName: cluster.name, region: cluster.region, status: 'active' }
                    : null
                });
              }}
              className="h-9 w-full rounded-md border bg-card px-2 text-sm shadow-control"
            >
              <option value="">未绑定</option>
              {clusters.map((cluster) => (
                <option key={cluster.id} value={cluster.id}>{cluster.name} · {cluster.region}</option>
              ))}
            </select>
          </label>

          <div className="rounded-md border bg-muted/35 p-3 text-xs text-muted-foreground">
            <div>固定槽位：第 {stage.layoutColumn + 1} 列 / 第 {stage.layoutRow + 1} 行</div>
            <div>绑定集群：{clusterBindingLabel(stage, clusters)}</div>
            <div>集群标签：{stage.clusterBinding ? labelsText(clusters.find((item) => item.id === stage.clusterBinding?.clusterId)?.labels) : '无'}</div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>保存与风险</CardTitle>
          <CardDescription>模板变更会影响该租户后续创建和打开的应用交付页。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {saveMessage && (
            <div className={cn('rounded-md border p-3 text-sm', saveMessage.includes('已保存') ? 'border-success/30 bg-success/10 text-success' : 'border-destructive/30 bg-destructive/10 text-destructive')}>
              {saveMessage}
            </div>
          )}
          <Button variant="destructive" className="w-full" onClick={onDelete}>
            <Trash2 className="h-4 w-4" />
            删除当前 Stage
          </Button>
        </CardContent>
      </Card>
    </aside>
  );
}

function MetricRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between rounded-md border bg-muted/30 px-3 py-2">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="mono text-sm font-semibold">{value}</span>
    </div>
  );
}

function mockTemplateForTenant(tenantId: string): DeliveryTemplate {
  const template = deliveryFlowTemplates.find((item) => item.tenantId === tenantId) || deliveryFlowTemplates[0];
  return {
    id: template.id,
    tenantId: template.tenantId,
    tenantName: template.tenantName,
    name: template.name,
    version: template.version,
    updatedAt: template.updatedAt,
    effectiveApps: template.effectiveApps,
    status: template.status,
    stages: template.stages,
    edges: template.edges
  };
}

function toLocalStages(template: DeliveryTemplate): LocalStage[] {
  return template.stages.map((stage) => ({
    ...stage,
    clientId: stage.id || stage.stageKey,
    originalStageKey: stage.stageKey
  }));
}

function toLocalEdges(template: DeliveryTemplate, stages: LocalStage[]): LocalEdge[] {
  const stageByKey = new Map(stages.map((stage) => [stage.stageKey, stage.clientId]));
  return template.edges.flatMap((edge) => {
    const sourceId = stageByKey.get(edge.fromStageKey);
    const targetId = stageByKey.get(edge.toStageKey);
    return sourceId && targetId ? [{ sourceId, targetId, rule: edge.rule }] : [];
  });
}

function labelsText(labels?: Record<string, string>) {
  const entries = Object.entries(labels || {});
  return entries.length ? entries.map(([key, value]) => `${key}=${value}`).join('、') : '无';
}

function PolicyBadge({ children }: { children: string }) {
  return (
    <span className="inline-flex h-5 items-center rounded border border-slate-200 bg-slate-50 px-1.5 text-[11px] font-medium text-slate-700">
      {children}
    </span>
  );
}

function CheckItem({ ok, text }: { ok: boolean; text: string }) {
  return (
    <div className="flex items-start gap-2 rounded-md border bg-card px-3 py-2 text-sm">
      {ok ? <CheckCircle2 className="mt-0.5 h-4 w-4 text-success" /> : <AlertTriangle className="mt-0.5 h-4 w-4 text-warning" />}
      <span className={ok ? 'text-foreground' : 'text-warning'}>{text}</span>
    </div>
  );
}

function LabelledInput({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="space-y-1.5 text-sm">
      <span className="font-medium">{label}</span>
      <Input value={value} onChange={(event) => onChange(event.target.value)} />
    </label>
  );
}

function ToggleRow({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="flex h-10 items-center justify-between rounded-md border bg-card px-3 text-sm">
      <span>{label}</span>
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
    </label>
  );
}

function RoleSelector({ label, values, onChange }: { label: string; values: string[]; onChange: (value: string[]) => void }) {
  return (
    <div className="space-y-2">
      <div className="text-sm font-medium">{label}</div>
      <div className="grid grid-cols-2 gap-2">
        {roleOptions.map((role) => (
          <label key={role.value} className="flex items-center gap-2 rounded-md border bg-card px-3 py-2 text-xs">
            <input
              type="checkbox"
              checked={values.includes(role.value)}
              onChange={(event) => {
                onChange(event.target.checked ? [...values, role.value] : values.filter((value) => value !== role.value));
              }}
            />
            <span>{role.label}</span>
          </label>
        ))}
      </div>
    </div>
  );
}

function resolveStageClusterBinding(stage: Pick<LocalStage, 'clusterBinding'>, clusters: Cluster[]): StageClusterBinding | null {
  if (!stage.clusterBinding) return null;
  const cluster = clusters.find((item) => item.id === stage.clusterBinding?.clusterId);
  return {
    clusterId: stage.clusterBinding.clusterId,
    clusterName: cluster?.name || stage.clusterBinding.clusterName || stage.clusterBinding.clusterId,
    region: cluster?.region || stage.clusterBinding.region || '-',
    status: stage.clusterBinding.status === 'disabled' || stage.clusterBinding.status === 'empty' ? stage.clusterBinding.status : 'active'
  };
}

function clusterBindingLabel(stage: Pick<LocalStage, 'clusterBinding'>, clusters: Cluster[]) {
  return resolveStageClusterBinding(stage, clusters)?.clusterName || '未绑定集群';
}

function stageColor(stage: Pick<LocalStage, 'colorToken'>) {
  return stagePalette[(stage.colorToken as StageColorToken) || 'geek-blue'] || stagePalette['geek-blue'];
}

function edgeKey(edge: Pick<LocalEdge, 'sourceId' | 'targetId'>) {
  return `${edge.sourceId}->${edge.targetId}`;
}

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  return tagName === 'input' || tagName === 'textarea' || tagName === 'select' || target.isContentEditable;
}

function nextStageKey(stages: LocalStage[]) {
  const existing = new Set(stages.map((stage) => stage.stageKey));
  for (let index = stages.length + 1; ; index += 1) {
    const key = `stage-${index}`;
    if (!existing.has(key)) return key;
  }
}

function normalizeStageKeyInput(value: string) {
  return value.trim().toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-');
}

function updateStageSlot(
  stageId: string,
  position: { x: number; y: number },
  stages: LocalStage[],
  setStages: Dispatch<SetStateAction<LocalStage[]>>
) {
  const occupied = stages
    .filter((stage) => stage.clientId !== stageId)
    .map((stage) => ({ column: stage.layoutColumn, row: stage.layoutRow }));
  const slot = nearestFreeSlot(position, occupied);
  setStages((current) => current.map((stage) => stage.clientId === stageId ? { ...stage, layoutColumn: slot.column, layoutRow: slot.row } : stage));
}

function nearestFreeSlot(position: { x: number; y: number }, occupied: { column: number; row: number }[]) {
  const target = {
    column: Math.max(0, Math.round(position.x / COLUMN_WIDTH)),
    row: Math.max(0, Math.round(position.y / ROW_HEIGHT))
  };
  if (!isSlotOccupied(target, occupied)) return target;
  for (let radius = 1; radius <= 20; radius += 1) {
    for (let column = target.column - radius; column <= target.column + radius; column += 1) {
      for (let row = Math.max(0, target.row - radius); row <= target.row + radius; row += 1) {
        const candidate = { column, row };
        if (candidate.column >= 0 && !isSlotOccupied(candidate, occupied)) return candidate;
      }
    }
  }
  return target;
}

function isSlotOccupied(slot: { column: number; row: number }, occupied: { column: number; row: number }[]) {
  return occupied.some((item) => item.column === slot.column && item.row === slot.row);
}

function validateTemplate(stages: LocalStage[], edges: LocalEdge[]) {
  if (stages.length === 0) return { status: 'danger' as const, message: '至少需要一个 Stage' };
  const keys = new Set<string>();
  for (const stage of stages) {
    if (!/^[a-z0-9-]+$/.test(stage.stageKey)) return { status: 'danger' as const, message: 'Stage key 只能使用小写字母、数字和短横线' };
    if (keys.has(stage.stageKey)) return { status: 'danger' as const, message: 'Stage key 不能重复' };
    keys.add(stage.stageKey);
  }
  if (hasCycle(stages.map((stage) => stage.clientId), edges)) return { status: 'danger' as const, message: 'Stage 依赖不能形成环' };
  return { status: 'success' as const, message: 'DAG 无环，Stage key 合法' };
}

function hasCycle(stageIds: string[], edges: LocalEdge[]) {
  const children = new Map<string, string[]>();
  for (const edge of edges) {
    children.set(edge.sourceId, [...(children.get(edge.sourceId) || []), edge.targetId]);
  }
  const visiting = new Set<string>();
  const visited = new Set<string>();
  const visit = (id: string): boolean => {
    if (visiting.has(id)) return true;
    if (visited.has(id)) return false;
    visiting.add(id);
    for (const child of children.get(id) || []) {
      if (visit(child)) return true;
    }
    visiting.delete(id);
    visited.add(id);
    return false;
  };
  return stageIds.some((id) => visit(id));
}
