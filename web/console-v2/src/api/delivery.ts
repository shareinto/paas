import { clusterOptions, deliveryFlowTemplates, type DeliveryTemplateEdge, type DeliveryTemplateStage } from '../data/mock';
import { actorBody, hasAPIBaseURL, request } from './client';

export type DeliveryTemplate = {
  id: string;
  tenantId: string;
  tenantName?: string;
  name: string;
  version: string;
  updatedAt: string;
  effectiveApps: number;
  status: 'enabled' | 'disabled';
  stages: DeliveryTemplateStage[];
  edges: DeliveryTemplateEdge[];
};

export type DeliveryTemplateSource = 'api' | 'mock';

export type DeliveryTemplateResult = {
  source: DeliveryTemplateSource;
  template: DeliveryTemplate;
  error?: string;
};

export type StageClusterBinding = {
  clusterId: string;
  clusterName: string;
  region: string;
  status: 'active' | 'empty' | 'disabled';
};

type BackendDeliveryTemplate = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name?: string;
  stages?: BackendTemplateStage[];
  edges?: BackendTemplateEdge[];
  updated_at?: string;
  updatedAt?: string;
};

type BackendTemplateStage = {
  id?: string;
  stage_key?: string;
  stageKey?: string;
  display_name?: string;
  displayName?: string;
  color?: string;
  order?: number;
  layout_column?: number;
  layoutColumn?: number;
  layout_row?: number;
  layoutRow?: number;
  status?: 'enabled' | 'disabled';
  requires_approval?: boolean;
  requiresApproval?: boolean;
  requires_verification?: boolean;
  requiresVerification?: boolean;
  approve_roles?: string[];
  approveRoles?: string[];
  verify_roles?: string[];
  verifyRoles?: string[];
};

type BackendTemplateEdge = {
  from_stage_key?: string;
  fromStageKey?: string;
  to_stage_key?: string;
  toStageKey?: string;
};

type BackendStageClusterBinding = {
  cluster_id?: string;
  clusterId?: string;
  cluster_name?: string;
  clusterName?: string;
  status?: 'active' | 'disabled';
};

export async function loadDeliveryTemplate(tenantId: string): Promise<DeliveryTemplateResult> {
  if (!hasAPIBaseURL()) {
    return { source: 'mock', template: mockDeliveryTemplate(tenantId) };
  }

  try {
    const template = await request<BackendDeliveryTemplate>(`/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template`);
    const mapped = mapDeliveryTemplate(template, tenantId);
    const stagesWithBindings = await Promise.all(
      mapped.stages.map(async (stage) => ({ ...stage, clusterBinding: await loadStageClusterBinding(tenantId, stage.stageKey) }))
    );
    return { source: 'api', template: { ...mapped, stages: stagesWithBindings } };
  } catch (error) {
    return {
      source: 'mock',
      template: mockDeliveryTemplate(tenantId),
      error: error instanceof Error ? error.message : '交付模板加载失败，已回退 mock 数据'
    };
  }
}

export async function saveDeliveryTemplateGraph(tenantId: string, template: DeliveryTemplate, deletedStageKeys: string[] = []): Promise<DeliveryTemplateResult> {
  if (!hasAPIBaseURL()) {
    return { source: 'mock', template };
  }

  const payload = {
    actor: actorBody(),
    stages: template.stages.map((stage) => ({
      actor: actorBody(),
      id: stage.id.startsWith('local-') ? '' : stage.id,
      stage_key: stage.stageKey,
      display_name: stage.displayName,
      color: colorTokenToBackend(stage.colorToken),
      order: stage.order,
      layout_column: stage.layoutColumn,
      layout_row: stage.layoutRow,
      status: stage.status,
      requires_approval: stage.requiresApproval,
      requires_verification: stage.requiresVerification,
      approve_roles: stage.approveRoles,
      verify_roles: stage.verifyRoles
    })),
    edges: template.edges.map((edge) => ({
      from_stage_key: edge.fromStageKey,
      to_stage_key: edge.toStageKey
    })),
    deleted_stage_keys: deletedStageKeys
  };

  const saved = await request<BackendDeliveryTemplate>(`/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/graph`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
  return { source: 'api', template: mapDeliveryTemplate(saved, tenantId) };
}

export async function saveStageClusterBinding(tenantId: string, stageKey: string, binding: StageClusterBinding | null | undefined) {
  if (!hasAPIBaseURL()) return;
  await request(`/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/stages/${encodeURIComponent(stageKey)}/cluster-bindings`, {
    method: 'PUT',
    body: JSON.stringify({
      actor: actorBody(),
      clusters: binding ? [{ cluster_id: binding.clusterId, cluster_name: binding.clusterName }] : []
    })
  });
}

function mockDeliveryTemplate(tenantId: string): DeliveryTemplate {
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

function mapDeliveryTemplate(template: BackendDeliveryTemplate, tenantId: string): DeliveryTemplate {
  const stages = (template.stages || [])
    .slice()
    .sort((a, b) => (a.order || 0) - (b.order || 0))
    .map((stage, index) => mapStage(stage, index));
  return {
    id: template.id,
    tenantId: template.tenant_id || template.tenantId || tenantId,
    name: template.name || '租户交付流模板',
    version: `template-${new Date(template.updated_at || template.updatedAt || Date.now()).getTime()}`,
    updatedAt: formatTime(template.updated_at || template.updatedAt),
    effectiveApps: 0,
    status: 'enabled',
    stages,
    edges: (template.edges || []).map((edge) => ({
      fromStageKey: edge.from_stage_key || edge.fromStageKey || '',
      toStageKey: edge.to_stage_key || edge.toStageKey || '',
      rule: '手动晋级'
    })).filter((edge) => edge.fromStageKey && edge.toStageKey)
  };
}

async function loadStageClusterBinding(tenantId: string, stageKey: string): Promise<StageClusterBinding | null> {
  try {
    const data = await request<{ items: BackendStageClusterBinding[] }>(
      `/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/stages/${encodeURIComponent(stageKey)}/cluster-bindings`
    );
    const binding = data.items[0];
    if (!binding) return null;
    const clusterId = binding.cluster_id || binding.clusterId || '';
    const option = clusterOptions.find((cluster) => cluster.id === clusterId);
    return {
      clusterId,
      clusterName: binding.cluster_name || binding.clusterName || option?.name || clusterId,
      region: option?.region || '-',
      status: binding.status === 'disabled' ? 'disabled' : 'active'
    };
  } catch {
    return null;
  }
}

function mapStage(stage: BackendTemplateStage, index: number): DeliveryTemplateStage {
  const stageKey = stage.stage_key || stage.stageKey || `stage-${index + 1}`;
  return {
    id: stage.id || stageKey,
    stageKey,
    displayName: stage.display_name || stage.displayName || stageKey,
    description: '由后端交付模板下发，控制该租户应用的交付拓扑。',
    colorToken: colorTokenFromBackend(stage.color, stage.layout_column ?? stage.layoutColumn ?? index),
    order: stage.order || index + 1,
    layoutColumn: stage.layout_column ?? stage.layoutColumn ?? index,
    layoutRow: stage.layout_row ?? stage.layoutRow ?? 0,
    status: stage.status || 'enabled',
    requiresApproval: stage.requires_approval ?? stage.requiresApproval ?? false,
    requiresVerification: stage.requires_verification ?? stage.requiresVerification ?? false,
    approveRoles: stage.approve_roles || stage.approveRoles || [],
    verifyRoles: stage.verify_roles || stage.verifyRoles || [],
    clusterBinding: null
  };
}

function colorTokenFromBackend(color: string | undefined, column: number) {
  if (!color) return ['geek-blue', 'mint-green', 'lemon-yellow', 'amber-orange', 'wine-purple'][column % 5] || 'geek-blue';
  if (color.startsWith('#')) {
    const hexMap: Record<string, string> = {
      '#0072B2': 'geek-blue',
      '#56B4E9': 'sky-blue',
      '#009E73': 'mint-green',
      '#44AA99': 'turquoise',
      '#F0E442': 'lemon-yellow',
      '#E69F00': 'amber-orange',
      '#D55E00': 'rust-red',
      '#882255': 'wine-purple',
      '#CC79A7': 'lilac',
      '#77AADD': 'smoke-blue'
    };
    return hexMap[color.toUpperCase()] || 'geek-blue';
  }
  return color;
}

function colorTokenToBackend(token: string) {
  const hexMap: Record<string, string> = {
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
  };
  return hexMap[token] || token || '#0072B2';
}

function formatTime(value: string | undefined) {
  if (!value) return '刚刚';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}
