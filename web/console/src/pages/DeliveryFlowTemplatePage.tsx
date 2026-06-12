import { DeleteOutlined, LinkOutlined, PlusOutlined, SaveOutlined } from '@ant-design/icons';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { Background, Controls, Handle, MarkerType, MiniMap, Position, ReactFlow, ReactFlowProvider, type Connection, type Edge, type Node, type NodeProps } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Alert, Button, Form, Input, Modal, Popconfirm, Select, Space, Statistic, Switch, Tag, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { getDeliveryFlowTemplate, listClusterOptions, listStageClusterBindings, listTenants, replaceDeliveryFlowTemplateGraph, replaceStageClusterBindings, type ClusterOption, type DeliveryFlowTemplateStage } from '../api';
import { PageHeader } from '../components/PageHeader';

const ROLE_OPTIONS = [
  { value: 'tenant_admin', label: '租户管理员' },
  { value: 'developer', label: '开发人员' },
  { value: 'operator', label: '运维人员' },
  { value: 'prod_approver', label: '生产审批人' }
];

const COLUMN_WIDTH = 240;
const ROW_HEIGHT = 150;
const COLUMN_COLORS = ['#ED204E', '#FD5352', '#FE7537', '#e78a00', '#DFC546', '#9bce22', '#84DF75', '#1CAC77', '#1bc1a7', '#1DCECA', '#0DAFD3', '#3882EA', '#2D5EDC', '#6380E1', '#7851AA', '#A9499D', '#D0469D', '#E573A2', '#f1619b', '#FE43A3', '#6a7382'];

type LocalStage = DeliveryFlowTemplateStage & {
  clientId: string;
  originalStageKey?: string;
};

type LocalEdge = {
  sourceId: string;
  targetId: string;
};

type StageNodeData = {
  stage: LocalStage;
};

const nodeTypes = { stage: StageNode };

export function DeliveryFlowTemplatePage() {
  return (
    <ReactFlowProvider>
      <DeliveryFlowTemplateContent />
    </ReactFlowProvider>
  );
}

function DeliveryFlowTemplateContent() {
  const queryClient = useQueryClient();
  const [tenantId, setTenantId] = useState<string>();
  const [selectedStageId, setSelectedStageId] = useState<string>();
  const [bindingStage, setBindingStage] = useState<LocalStage | null>(null);
  const [selectedClusterId, setSelectedClusterId] = useState<string>();
  const [localStages, setLocalStages] = useState<LocalStage[]>([]);
  const [localEdges, setLocalEdges] = useState<LocalEdge[]>([]);
  const [deletedStageKeys, setDeletedStageKeys] = useState<string[]>([]);

  const { data: tenants = [] } = useQuery({ queryKey: ['tenants'], queryFn: listTenants });
  const currentTenantId = tenantId || tenants[0]?.id || '';
  const tenantOptions = useMemo(() => tenants.map((tenant) => ({ value: tenant.id, label: tenant.displayName || tenant.name })), [tenants]);
  const { data: template, isLoading } = useQuery({
    queryKey: ['delivery-flow-template', currentTenantId],
    queryFn: () => getDeliveryFlowTemplate(currentTenantId),
    enabled: !!currentTenantId
  });
  const { data: clusters = [] } = useQuery({
    queryKey: ['cluster-options', currentTenantId],
    queryFn: () => listClusterOptions(currentTenantId),
    enabled: !!currentTenantId
  });
  const bindingStageKey = bindingStage?.originalStageKey || bindingStage?.stageKey;
  const { data: bindingClusters } = useQuery({
    queryKey: ['stage-cluster-bindings', currentTenantId, bindingStageKey],
    queryFn: () => bindingStageKey ? listStageClusterBindings(currentTenantId, bindingStageKey) : Promise.resolve([]),
    enabled: !!currentTenantId && !!bindingStageKey
  });

  useEffect(() => {
    if (!tenantId && tenants[0]) setTenantId(tenants[0].id);
  }, [tenantId, tenants]);

  useEffect(() => {
    if (!template) return;
    const stages = [...template.stages].sort((a, b) => a.order - b.order);
    const local = stages.map((stage) => ({ ...stage, clientId: stage.id || stage.stageKey, originalStageKey: stage.stageKey, color: columnColor(stage.layoutColumn || 0) }));
    const clientByStageKey = new Map(local.map((stage) => [stage.stageKey, stage.clientId]));
    setLocalStages(local);
    setDeletedStageKeys([]);
    setLocalEdges((template.edges || []).flatMap((edge) => {
      const sourceId = clientByStageKey.get(edge.fromStageKey);
      const targetId = clientByStageKey.get(edge.toStageKey);
      return sourceId && targetId ? [{ sourceId, targetId }] : [];
    }));
    setSelectedStageId((current) => current && local.some((stage) => stage.clientId === current) ? current : local[0]?.clientId);
  }, [template]);

  useEffect(() => {
    if (!bindingStage || !bindingClusters) return;
    setSelectedClusterId(bindingClusters[0]?.clusterId);
  }, [bindingClusters, bindingStage]);

  const stagesWithColors = useMemo(() => localStages.map((stage, index) => ({ ...stage, order: index + 1, color: columnColor(stage.layoutColumn || 0) })), [localStages]);
  const selectedStage = stagesWithColors.find((stage) => stage.clientId === selectedStageId) || null;
  const stageBindingsByKey = useStageBindings(currentTenantId, localStages);

  const saveGraphMutation = useMutation({
    mutationFn: () => {
      const validation = validateLocalGraph(stagesWithColors, localEdges);
      if (validation) throw new Error(validation);
      return replaceDeliveryFlowTemplateGraph(currentTenantId, {
        stages: stagesWithColors.map(({ clientId, originalStageKey, ...stage }) => stage),
        edges: localEdges.map((edge) => ({
          fromStageKey: stagesWithColors.find((stage) => stage.clientId === edge.sourceId)?.stageKey || '',
          toStageKey: stagesWithColors.find((stage) => stage.clientId === edge.targetId)?.stageKey || ''
        })),
        deletedStageKeys: deletedStageKeys.concat(stagesWithColors.flatMap((stage) => stage.originalStageKey && stage.originalStageKey !== stage.stageKey ? [stage.originalStageKey] : []))
      });
    },
    onSuccess: () => {
      message.success('交付流 DAG 模板已保存');
      queryClient.invalidateQueries({ queryKey: ['delivery-flow-template', currentTenantId] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存 DAG 失败')
  });

  const bindingMutation = useMutation({
    mutationFn: () => bindingStageKey ? replaceStageClusterBindings(currentTenantId, bindingStageKey, selectedClusterId ? [selectedClusterId] : []) : Promise.resolve([]),
    onSuccess: () => {
      message.success('集群绑定已保存');
      setBindingStage(null);
      setSelectedClusterId(undefined);
      queryClient.invalidateQueries({ queryKey: ['stage-cluster-bindings', currentTenantId] });
    }
  });

  const nodes = useMemo<Node<StageNodeData>[]>(() => layoutNodes(stagesWithColors), [stagesWithColors]);
  const edges = useMemo<Edge[]>(() => localEdges.map((edge) => ({
    id: `${edge.sourceId}->${edge.targetId}`,
    source: edge.sourceId,
    target: edge.targetId,
    markerEnd: { type: MarkerType.ArrowClosed },
    className: 'delivery-dag-edge'
  })), [localEdges]);

  const onConnect = (connection: Connection) => {
    if (!connection.source || !connection.target || connection.source === connection.target) return;
    const candidate = { sourceId: connection.source, targetId: connection.target };
    setLocalEdges((current) => {
      if (current.some((edge) => edge.sourceId === candidate.sourceId && edge.targetId === candidate.targetId)) return current;
      const next = [...current, candidate];
      if (hasCycle(localStages.map((stage) => stage.clientId), next)) {
        message.warning('Stage 依赖不能形成环');
        return current;
      }
      return next;
    });
  };

  const addLocalStage = () => {
    const column = Math.max(-1, ...localStages.map((stage) => stage.layoutColumn || 0)) + 1;
    const stage: LocalStage = {
      id: '',
      clientId: `local_stage_${Date.now()}`,
      tenantId: currentTenantId,
      templateId: template?.id || '',
      stageKey: nextStageKey(localStages),
      displayName: '新 Stage',
      color: columnColor(column),
      order: localStages.length + 1,
      layoutColumn: column,
      layoutRow: 0,
      status: 'enabled',
      requiresApproval: false,
      requiresVerification: false,
      approveRoles: [],
      verifyRoles: []
    };
    setLocalStages((current) => [...current, stage]);
    setSelectedStageId(stage.clientId);
  };

  const updateSelectedStage = (patch: Partial<LocalStage>) => {
    if (!selectedStage) return;
    setLocalStages((current) => current.map((stage) => stage.clientId === selectedStage.clientId ? { ...stage, ...patch } : stage));
  };

  const deleteLocalStage = (clientId: string) => {
    const stage = localStages.find((item) => item.clientId === clientId);
    if (stage?.originalStageKey) {
      setDeletedStageKeys((current) => current.includes(stage.originalStageKey || '') ? current : [...current, stage.originalStageKey || '']);
    }
    setLocalStages((current) => current.filter((stage) => stage.clientId !== clientId));
    setLocalEdges((current) => current.filter((edge) => edge.sourceId !== clientId && edge.targetId !== clientId));
    setSelectedStageId((current) => current === clientId ? undefined : current);
  };

  const updateStageSlot = (clientId: string, position: { x: number; y: number }) => {
    setLocalStages((current) => {
      const occupied = current.filter((stage) => stage.clientId !== clientId).map((stage) => ({ column: stage.layoutColumn || 0, row: stage.layoutRow || 0 }));
      const slot = nearestFreeSlot(position, occupied);
      return current.map((stage) => stage.clientId === clientId ? { ...stage, layoutColumn: slot.column, layoutRow: slot.row, color: columnColor(slot.column) } : stage);
    });
  };

  return (
    <>
      <PageHeader title="租户交付流模板" extra={<Space><Button aria-label="添加 Stage" icon={<PlusOutlined />} disabled={!currentTenantId} onClick={addLocalStage}>添加 Stage</Button><Button aria-label="保存 DAG" type="primary" icon={<SaveOutlined />} loading={saveGraphMutation.isPending} disabled={!currentTenantId || localStages.length === 0} onClick={() => saveGraphMutation.mutate()}>保存 DAG</Button></Space>} />
      <Typography.Paragraph type="secondary">拖拽卡片调整固定槽位，拖拽连线配置 Stage 依赖；点击 Stage 后在右侧配置审批、验证和集群池。Stage 顶部色条按列自动生成。</Typography.Paragraph>
      <div className="toolbar">
        <Select aria-label="租户" placeholder="选择租户" options={tenantOptions} value={currentTenantId || undefined} onChange={(value) => { setTenantId(value); setSelectedStageId(undefined); }} />
      </div>
      <div className="delivery-template-stats">
        <Statistic title="当前模板" value={template?.name || '-'} loading={isLoading} />
        <Statistic title="Stage 数量" value={localStages.length} />
        <Statistic title="依赖边" value={localEdges.length} />
        <Statistic title="生效方式" value="保存后自动生效" />
      </div>
      <Alert className="form-alert" type="info" showIcon message="DAG 必须无环" description="没有上游的 Stage 可直接发布；有多个上游时，Freight 必须先通过全部直接上游 Stage。" />

      <div className="delivery-dag-editor">
        <section className="delivery-dag-canvas" aria-label="交付流 DAG 画布">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            fitView
            onConnect={onConnect}
            onNodeClick={(_, node) => setSelectedStageId(node.id)}
            onNodeDragStop={(_, node) => updateStageSlot(node.id, node.position)}
            onEdgesDelete={(deleted) => setLocalEdges((current) => current.filter((edge) => !deleted.some((item) => item.id === `${edge.sourceId}->${edge.targetId}`)))}
          >
            <Background />
            <MiniMap pannable zoomable />
            <Controls />
          </ReactFlow>
        </section>
        <aside className="delivery-dag-inspector" aria-label="Stage 属性面板">
          {selectedStage ? (
            <Space direction="vertical" size={12} className="full-width">
              <div className="inspector-color-strip" style={{ backgroundColor: selectedStage.color }} />
              <Typography.Title level={4}>{selectedStage.displayName}</Typography.Title>
              <Form layout="vertical">
                <Form.Item label="Stage key"><Input value={selectedStage.stageKey} onChange={(event) => updateSelectedStage({ stageKey: event.target.value })} /></Form.Item>
                <Form.Item label="显示名"><Input value={selectedStage.displayName} onChange={(event) => updateSelectedStage({ displayName: event.target.value })} /></Form.Item>
                <Form.Item label="状态"><Select value={selectedStage.status} options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '禁用' }]} onChange={(status) => updateSelectedStage({ status })} /></Form.Item>
                <Form.Item label="部署前审批"><Switch checked={selectedStage.requiresApproval} onChange={(checked) => updateSelectedStage({ requiresApproval: checked })} /></Form.Item>
                <Form.Item label="部署后验证"><Switch checked={selectedStage.requiresVerification} onChange={(checked) => updateSelectedStage({ requiresVerification: checked })} /></Form.Item>
                <Form.Item label="允许审批角色"><Select mode="multiple" value={selectedStage.approveRoles} options={ROLE_OPTIONS} onChange={(approveRoles) => updateSelectedStage({ approveRoles })} /></Form.Item>
                <Form.Item label="允许验证角色"><Select mode="multiple" value={selectedStage.verifyRoles} options={ROLE_OPTIONS} onChange={(verifyRoles) => updateSelectedStage({ verifyRoles })} /></Form.Item>
              </Form>
              <Space wrap>
                <Button aria-label="绑定集群" icon={<LinkOutlined />} disabled={!selectedStage.originalStageKey || selectedStage.originalStageKey !== selectedStage.stageKey} onClick={() => { setBindingStage(selectedStage); setSelectedClusterId(undefined); }}>绑定集群</Button>
                <Popconfirm title="删除 Stage" description="确认删除该 Stage？保存 DAG 后生效。" okText="删除" cancelText="取消" okButtonProps={{ danger: true }} onConfirm={() => deleteLocalStage(selectedStage.clientId)}>
                  <Button aria-label="删除" danger icon={<DeleteOutlined />}>删除</Button>
                </Popconfirm>
              </Space>
              <div className="dag-dependency-summary">
                <Typography.Text strong>依赖关系</Typography.Text>
                <div>上游：{localEdges.filter((edge) => edge.targetId === selectedStage.clientId).map((edge) => stagesWithColors.find((stage) => stage.clientId === edge.sourceId)?.stageKey).filter(Boolean).join('、') || '无'}</div>
                <div>下游：{localEdges.filter((edge) => edge.sourceId === selectedStage.clientId).map((edge) => stagesWithColors.find((stage) => stage.clientId === edge.targetId)?.stageKey).filter(Boolean).join('、') || '无'}</div>
                <div>固定槽位：第 {selectedStage.layoutColumn + 1} 列 / 第 {selectedStage.layoutRow} 行</div>
                <div>已绑定集群：{(stageBindingsByKey.get(selectedStage.originalStageKey || selectedStage.stageKey) || []).map((binding) => binding.clusterName).join('、') || '无'}</div>
                {!selectedStage.originalStageKey && <Typography.Text type="secondary">保存 DAG 后可绑定集群。</Typography.Text>}
              </div>
            </Space>
          ) : (
            <Alert type="info" showIcon message="请选择一个 Stage" />
          )}
        </aside>
      </div>

      <Modal title="绑定集群" open={!!bindingStage} onCancel={() => setBindingStage(null)} onOk={() => bindingMutation.mutate()} confirmLoading={bindingMutation.isPending} destroyOnHidden>
        <Space direction="vertical" className="full-width" size={12}>
          <Alert type="info" showIcon message="绑定到租户级 Stage，保存后作为该 Stage 的唯一目标集群。" description="同一集群可以绑定多个 Stage；一个 Stage 最多绑定一个集群，清空后该 Stage 暂不可部署。" />
          <Select
            aria-label="绑定集群"
            allowClear
            placeholder="选择集群"
            value={selectedClusterId}
            options={(clusters as ClusterOption[]).map((cluster) => ({ value: cluster.id, label: `${cluster.name}（${cluster.region}，${formatLabels(cluster.labels)}）` }))}
            onChange={(value) => setSelectedClusterId(value)}
          />
        </Space>
      </Modal>
    </>
  );
}

function StageNode({ data }: NodeProps<Node<StageNodeData>>) {
  const stage = data.stage;
  return (
    <div className="dag-stage-node" aria-label={`${stage.stageKey || '未命名'} Stage 模板`}>
      <Handle type="target" position={Position.Left} />
      <div className="dag-stage-strip" style={{ backgroundColor: stage.color }} />
      <div className="dag-stage-body">
        <Typography.Text strong>{stage.displayName || '新 Stage'}</Typography.Text>
        <Typography.Text type="secondary">{stage.stageKey || '未填写 key'}</Typography.Text>
        <Space size={4} wrap>
          {stage.requiresApproval && <Tag color="orange">审批</Tag>}
          {stage.requiresVerification && <Tag color="blue">验证</Tag>}
          {stage.status === 'disabled' && <Tag>禁用</Tag>}
        </Space>
      </div>
      <Handle type="source" position={Position.Right} />
    </div>
  );
}

function useStageBindings(tenantId: string, stages: LocalStage[]) {
  const queryStages = stages.filter((stage) => stage.originalStageKey);
  const queries = useQueries({
    queries: queryStages.map((stage) => ({
      queryKey: ['stage-cluster-bindings', tenantId, stage.originalStageKey],
      queryFn: () => listStageClusterBindings(tenantId, stage.originalStageKey || stage.stageKey),
      enabled: !!tenantId && !!stage.originalStageKey
    }))
  });
  return new Map(queryStages.map((stage, index) => [stage.originalStageKey || stage.stageKey, queries[index]?.data || []]));
}

function layoutNodes(stages: LocalStage[]) {
  return stages.map((stage) => ({
    id: stage.clientId,
    type: 'stage',
    data: { stage },
    position: { x: (stage.layoutColumn || 0) * COLUMN_WIDTH, y: (stage.layoutRow || 0) * ROW_HEIGHT }
  }));
}

function nearestFreeSlot(position: { x: number; y: number }, occupied: { column: number; row: number }[]) {
  const target = { column: Math.max(0, Math.round(position.x / COLUMN_WIDTH)), row: Math.round(position.y / ROW_HEIGHT) };
  if (!isSlotOccupied(target, occupied)) return target;
  let best = target;
  let bestDistance = Number.POSITIVE_INFINITY;
  for (let radius = 1; radius <= 24; radius += 1) {
    for (let dc = -radius; dc <= radius; dc += 1) {
      for (let dr = -radius; dr <= radius; dr += 1) {
        if (Math.max(Math.abs(dc), Math.abs(dr)) !== radius) continue;
        const candidate = { column: target.column + dc, row: target.row + dr };
        if (candidate.column < 0 || isSlotOccupied(candidate, occupied)) continue;
        const distance = dc * dc + dr * dr;
        if (distance < bestDistance) {
          best = candidate;
          bestDistance = distance;
        }
      }
    }
    if (bestDistance < Number.POSITIVE_INFINITY) return best;
  }
  return best;
}

function isSlotOccupied(slot: { column: number; row: number }, occupied: { column: number; row: number }[]) {
  return occupied.some((item) => item.column === slot.column && item.row === slot.row);
}

function validateLocalGraph(stages: LocalStage[], edges: LocalEdge[]) {
  const stageKeys = new Map<string, string>();
  for (const stage of stages) {
    const key = normalizeStageKey(stage.stageKey);
    if (!key) return 'Stage key 只能使用小写字母、数字和短横线';
    if (stageKeys.has(key)) return 'Stage key 不能重复';
    stageKeys.set(key, stage.clientId);
  }
  for (const edge of edges) {
    const source = stages.find((stage) => stage.clientId === edge.sourceId);
    const target = stages.find((stage) => stage.clientId === edge.targetId);
    if (!source || !target || source.clientId === target.clientId) return '依赖边引用了不存在的 Stage';
    if (source.status === 'disabled' || target.status === 'disabled') return '禁用 Stage 不能参与依赖边';
  }
  if (hasCycle(stages.map((stage) => stage.clientId), edges)) return 'Stage 依赖不能形成环';
  stages.forEach((stage) => {
    stage.stageKey = normalizeStageKey(stage.stageKey);
  });
  return '';
}

function hasCycle(stageIds: string[], edges: LocalEdge[]) {
  const stageIdSet = new Set(stageIds);
  const children = new Map<string, string[]>();
  for (const edge of edges) {
    if (!stageIdSet.has(edge.sourceId) || !stageIdSet.has(edge.targetId)) continue;
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

function nextStageKey(stages: LocalStage[]) {
  const existing = new Set(stages.map((stage) => stage.stageKey));
  for (let index = stages.length + 1; ; index += 1) {
    const key = `stage-${index}`;
    if (!existing.has(key)) return key;
  }
}

function normalizeStageKey(value: string) {
  const key = value.trim().toLowerCase();
  return /^[a-z0-9-]+$/.test(key) ? key : '';
}

function columnColor(column: number) {
  return COLUMN_COLORS[Math.max(0, column) % COLUMN_COLORS.length];
}

function formatLabels(labels?: Record<string, string>) {
  const entries = Object.entries(labels || {});
  return entries.length > 0 ? entries.map(([key, value]) => `${key}=${value}`).join(', ') : '无标签';
}
