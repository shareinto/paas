import {
  deliveryTopology as mockTopology,
  freights as mockFreights,
  versionSourceConfig as mockVersionSourceConfig,
  versionSourcePipelines as mockPipelines,
  type Status,
  type VersionSourcePipeline,
  type VersionSourceWorkloadConfig
} from '../data/mock';
import {
  mapPipelineFormOptionsFromWorkspace,
  mapVersionSourcePipelinesFromWorkspace,
  type BackendBuildEnvironment,
  type BackendBuildPipeline,
  type BackendBuildPipelineSource,
  type BackendBuildRun,
  type BackendRuntimeEnvironment,
  type BuildPipelineSourceResult,
  type PipelineFormOptionsResult
} from './buildPipelines';
import { APIError, actorBody, actorQuery, hasAPIBaseURL, request, type PageResult } from './client';
import {
  mapVersionSourceWorkloadsFromWorkspace,
  type BackendWorkload,
  type BackendWorkloadStageConfig,
  type WorkloadSourceResult
} from './workloads';

export type DeploymentTopology = typeof mockTopology;
export type DeploymentStage = DeploymentTopology['stages'][number];
export type DeploymentEdge = DeploymentTopology['edges'][number];
export type DeploymentFreightContainer = {
  name: string;
  pipeline: string;
  version: string;
  image: string;
  digest: string;
  status: Status;
  sourceMode?: 'pipeline' | 'custom';
};
export type DeploymentFreightWorkload = Omit<(typeof mockFreights)[number]['workloads'][number], 'containers'> & {
  containers?: DeploymentFreightContainer[];
};
export type DeploymentFreight = Omit<(typeof mockFreights)[number], 'workloads'> & {
  name?: string;
  sourceFingerprint?: string;
  workloads: DeploymentFreightWorkload[];
};

export type DeploymentWorkspace = {
  source: 'api' | 'mock';
  topology: DeploymentTopology;
  freights: DeploymentFreight[];
  approvalGates?: ApprovalGateSummary[];
  publishGates?: PublishGateSummary[];
  error?: string;
};

export type ApprovalGateSummary = {
  targetStageKey: string;
  targetStageName: string;
  pendingCount: number;
  canReview: boolean;
  requiredPermissionCode: string;
};

export type PublishGateSummary = {
  targetStageKey: string;
  targetStageName: string;
  pendingCount: number;
  canPublish: boolean;
  requiredPermissionCode: string;
};

export type DeploymentPageBundle = {
  workspace: DeploymentWorkspace;
  pipelines: BuildPipelineSourceResult;
  pipelineOptions: PipelineFormOptionsResult;
  workloads: WorkloadSourceResult;
};

type BackendStage = {
  delivery_stage_id?: string;
  deliveryStageId?: string;
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
  requires_approval?: boolean;
  requiresApproval?: boolean;
  requires_verification?: boolean;
  requiresVerification?: boolean;
  bound_cluster_id?: string;
  boundClusterId?: string;
  bound_cluster_name?: string;
  boundClusterName?: string;
  current_freight_id?: string;
  currentFreightId?: string;
  current_freight_version?: string;
  currentFreightVersion?: string;
  sync_status?: string;
  syncStatus?: string;
  health_status?: string;
  healthStatus?: string;
  operation_state?: string;
  operationState?: string;
  runtime_message?: string;
  runtimeMessage?: string;
  upstream_stage_keys?: string[];
  upstreamStageKeys?: string[];
  downstream_stage_keys?: string[];
  downstreamStageKeys?: string[];
  approve_roles?: string[];
  approveRoles?: string[];
  verify_roles?: string[];
  verifyRoles?: string[];
};

type BackendFreight = {
  id: string;
  name?: string;
  status?: string;
  source_fingerprint?: string;
  sourceFingerprint?: string;
  created_at?: string;
  createdAt?: string;
  commit_sha?: string;
  commitSHA?: string;
  commit?: string;
  items?: BackendFreightItem[];
  freight_items?: BackendFreightItem[];
  freightItems?: BackendFreightItem[];
  freight?: BackendFreight;
};

type BackendFreightItem = {
  workload_id?: string;
  workloadId?: string;
  workload_name?: string;
  workloadName?: string;
  workload_display_name?: string;
  workloadDisplayName?: string;
  container_name?: string;
  containerName?: string;
  name?: string;
  source_type?: string;
  sourceType?: string;
  source_key?: string;
  sourceKey?: string;
  image_source_mode?: string;
  imageSourceMode?: string;
  type?: string;
  uri?: string;
  image?: string;
  image_ref?: string;
  imageRef?: string;
  image_repository?: string;
  imageRepository?: string;
  image_tag?: string;
  imageTag?: string;
  image_digest?: string;
  imageDigest?: string;
  digest?: string;
  release_id?: string;
  releaseId?: string;
  build_artifact_id?: string;
  buildArtifactId?: string;
};

type BackendFreightCreationContext = {
  latest_releases_by_workload?: Record<string, BackendReleaseRef>;
  latestReleasesByWorkload?: Record<string, BackendReleaseRef>;
  latest_artifacts_by_workload?: Record<string, BackendArtifactRef>;
  latestArtifactsByWorkload?: Record<string, BackendArtifactRef>;
  latest_releases_by_workload_container?: Record<string, BackendReleaseRef>;
  latestReleasesByWorkloadContainer?: Record<string, BackendReleaseRef>;
  latest_artifacts_by_workload_container?: Record<string, BackendArtifactRef>;
  latestArtifactsByWorkloadContainer?: Record<string, BackendArtifactRef>;
};

type BackendReleaseRef = {
  id: string;
  workload_id?: string;
  workloadId?: string;
  container_name?: string;
  containerName?: string;
  build_artifact_id?: string;
  buildArtifactId?: string;
};

type BackendArtifactRef = {
  id: string;
  workload_id?: string;
  workloadId?: string;
  container_name?: string;
  containerName?: string;
};

type BackendRuntimeResource = {
  id?: string;
  kind?: string;
  namespace?: string;
  name?: string;
  node_name?: string;
  nodeName?: string;
  pod_ip?: string;
  podIP?: string;
  podIp?: string;
  parent_kind?: string;
  parentKind?: string;
  parent_namespace?: string;
  parentNamespace?: string;
  parent_name?: string;
  parentName?: string;
  status?: string;
  health_status?: string;
  healthStatus?: string;
  desired?: number;
  ready?: number;
  containers?: Array<{
    name?: string;
    image?: string;
    ready?: boolean;
    restart_count?: number;
    restartCount?: number;
    state?: string;
  }>;
};

type BackendDeploymentWorkspaceError = {
  scope?: string;
  key?: string;
  code?: string;
  message?: string;
};

type BackendDeploymentWorkspace = {
  stages?: BackendStage[];
  freights?: PageResult<BackendFreight>;
  freight_details?: Record<string, BackendFreight>;
  freightDetails?: Record<string, BackendFreight>;
  eligible_freights_by_stage?: Record<string, BackendFreight[]>;
  eligibleFreightsByStage?: Record<string, BackendFreight[]>;
  runtime_resources_by_stage?: Record<string, BackendRuntimeResource[]>;
  runtimeResourcesByStage?: Record<string, BackendRuntimeResource[]>;
  approval_gates?: BackendApprovalGate[];
  approvalGates?: BackendApprovalGate[];
  publish_gates?: BackendPublishGate[];
  publishGates?: BackendPublishGate[];
  build_pipelines?: PageResult<BackendBuildPipeline>;
  buildPipelines?: PageResult<BackendBuildPipeline>;
  build_runs?: PageResult<BackendBuildRun>;
  buildRuns?: PageResult<BackendBuildRun>;
  pipeline_sources?: Record<string, BackendBuildPipelineSource[]>;
  pipelineSources?: Record<string, BackendBuildPipelineSource[]>;
  runtime_environments?: PageResult<BackendRuntimeEnvironment>;
  runtimeEnvironments?: PageResult<BackendRuntimeEnvironment>;
  build_environments?: PageResult<BackendBuildEnvironment>;
  buildEnvironments?: PageResult<BackendBuildEnvironment>;
  workloads?: BackendWorkload[];
  workload_default_configs?: Record<string, BackendWorkloadStageConfig>;
  workloadDefaultConfigs?: Record<string, BackendWorkloadStageConfig>;
  errors?: BackendDeploymentWorkspaceError[];
};

type BackendApprovalGate = {
  target_stage_key?: string;
  targetStageKey?: string;
  target_stage_name?: string;
  targetStageName?: string;
  pending_count?: number;
  pendingCount?: number;
  can_review?: boolean;
  canReview?: boolean;
  required_permission_code?: string;
  requiredPermissionCode?: string;
};

type BackendPublishGate = {
  target_stage_key?: string;
  targetStageKey?: string;
  target_stage_name?: string;
  targetStageName?: string;
  pending_count?: number;
  pendingCount?: number;
  can_publish?: boolean;
  canPublish?: boolean;
  required_permission_code?: string;
  requiredPermissionCode?: string;
};

export type ApprovalTaskSummary = {
  id: string;
  promotionId: string;
  freightId: string;
  freightName: string;
  targetStageKey: string;
  targetStageName: string;
  requestedBy: string;
  requestedAt: string;
  message: string;
  diffType: 'first_deploy' | 'compare';
};

export type ApprovalTaskDetail = {
  task: ApprovalTaskSummary;
  diffType: 'first_deploy' | 'compare';
  currentFreight?: { id: string; name: string; createdAt: string };
  pendingFreight: { id: string; name: string; createdAt: string };
  imageChanges: Array<{
    workloadId: string;
    workloadName: string;
    containerName: string;
    currentImage: string;
    pendingImage: string;
    currentVersion: string;
    pendingVersion: string;
  }>;
  configDiff: string;
  deployItems: Array<{
    workloadId: string;
    workloadName: string;
    containerName: string;
    version: string;
    image: string;
  }>;
};

export async function loadDeploymentWorkspace(applicationId: string): Promise<DeploymentWorkspace> {
  if (!hasAPIBaseURL()) {
    return { source: 'mock', topology: mockTopology, freights: mockFreights };
  }

  const [stageData, freightData] = await Promise.all([
    request<{ items: BackendStage[] }>(`/api/apps/${encodeURIComponent(applicationId)}/stages`),
    request<PageResult<BackendFreight>>(`/api/apps/${encodeURIComponent(applicationId)}/freights?page=1&page_size=50`)
  ]);
  const stagesRaw = stageData.items || [];
  const freights = await mapFreights(applicationId, freightData.items || [], stagesRaw);
  const stageResults = await Promise.all(stagesRaw.map((stage, index) => mapStage(applicationId, stage, index)));
  const runtimeErrors = stageResults.flatMap((result) => result.error ? [result.error] : []);
  const stages = stageResults.map((result) => result.stage);
  return {
    source: 'api',
    topology: {
      ...mockTopology,
      applicationId,
      applicationName: applicationId,
      topologyVersion: `api-${applicationId}`,
      generatedAt: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
      stages,
      edges: mapEdges(stagesRaw),
      stageConfigSchema: mockTopology.stageConfigSchema
    },
    freights,
    error: runtimeErrors.join('；')
  };
}

export async function loadDeploymentPageBundle(applicationId: string): Promise<DeploymentPageBundle> {
  if (!hasAPIBaseURL()) {
    return {
      workspace: { source: 'mock', topology: mockTopology, freights: mockFreights },
      pipelines: { source: 'mock', pipelines: mockPipelines },
      pipelineOptions: {
        source: 'mock',
        runtimeOptions: [],
        buildEnvironmentOptions: []
      },
      workloads: { source: 'mock', config: mockVersionSourceConfig }
    };
  }

  const data = await request<BackendDeploymentWorkspace>(
    `/api/console-v2/apps/${encodeURIComponent(applicationId)}/deployment-workspace?${actorQuery()}`
  );
  const stagesRaw = data.stages || [];
  const freightsRaw = (data.freights?.items || []);
  const freightDetails = data.freight_details || data.freightDetails || {};
  const eligibleByStage = data.eligible_freights_by_stage || data.eligibleFreightsByStage || {};
  const runtimeByStage = data.runtime_resources_by_stage || data.runtimeResourcesByStage || {};
  const errors = data.errors || [];
  const runtimeErrors = errors
    .filter((error) => error.scope === 'runtime_resources')
    .reduce<Record<string, string>>((acc, error) => {
      if (error.key) acc[error.key] = `${error.key} 运行态加载失败：${error.message || '请求处理失败'}`;
      return acc;
    }, {});
  const freights = mapFreightsFromWorkspace(freightsRaw, stagesRaw, freightDetails, eligibleByStage);
  const stages = stagesRaw.map((stage, index) => mapStageFromWorkspace(stage, index, runtimeByStage[stageKeyOf(stage)] || [], runtimeErrors[stageKeyOf(stage)]));
  const workspace: DeploymentWorkspace = {
    source: 'api',
    topology: {
      ...mockTopology,
      applicationId,
      applicationName: applicationId,
      topologyVersion: `api-${applicationId}`,
      generatedAt: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
      stages,
      edges: mapEdges(stagesRaw),
      stageConfigSchema: mockTopology.stageConfigSchema
    },
    freights,
    approvalGates: mapApprovalGates(data.approval_gates || data.approvalGates || []),
    publishGates: mapPublishGates(data.publish_gates || data.publishGates || []),
    error: errors.map((error) => [error.scope, error.key, error.message].filter(Boolean).join(' / ')).join('；')
  };

  const pipelinePage = data.build_pipelines || data.buildPipelines || { items: [] };
  const runPage = data.build_runs || data.buildRuns || { items: [] };
  const pipelineSources = data.pipeline_sources || data.pipelineSources || {};
  const runtimePage = data.runtime_environments || data.runtimeEnvironments || { items: [] };
  const buildPage = data.build_environments || data.buildEnvironments || { items: [] };
  const pipelines = mapVersionSourcePipelinesFromWorkspace(pipelinePage.items || [], pipelineSources, runPage.items || []);
  return {
    workspace,
    pipelines,
    pipelineOptions: mapPipelineFormOptionsFromWorkspace(runtimePage.items || [], buildPage.items || []),
    workloads: mapVersionSourceWorkloadsFromWorkspace(
      data.workloads || [],
      data.workload_default_configs || data.workloadDefaultConfigs || {},
      pipelines.pipelines
    )
  };
}

export async function listApprovalTasks(applicationId: string, stageKey: string): Promise<ApprovalTaskSummary[]> {
  if (!hasAPIBaseURL()) return [];
  const data = await request<{ items?: unknown[] }>(
    `/api/console-v2/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/approval-tasks?${actorQuery()}`
  );
  return (data.items || []).map(mapApprovalTaskSummary);
}

export async function getApprovalTask(taskId: string): Promise<ApprovalTaskDetail> {
  const data = await request<Record<string, unknown>>(`/api/console-v2/approval-tasks/${encodeURIComponent(taskId)}?${actorQuery()}`);
  return mapApprovalTaskDetail(data);
}

export async function approveApprovalTask(taskId: string, comment: string) {
  return request(`/api/console-v2/approval-tasks/${encodeURIComponent(taskId)}/approve?${actorQuery()}`, {
    method: 'POST',
    body: JSON.stringify({ actor: actorBody(), comment })
  });
}

export async function rejectApprovalTask(taskId: string, comment: string) {
  return request(`/api/console-v2/approval-tasks/${encodeURIComponent(taskId)}/reject?${actorQuery()}`, {
    method: 'POST',
    body: JSON.stringify({ actor: actorBody(), comment })
  });
}

export async function listPublishTasks(applicationId: string, stageKey: string): Promise<ApprovalTaskSummary[]> {
  if (!hasAPIBaseURL()) return [];
  const data = await request<{ items?: unknown[] }>(
    `/api/console-v2/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/publish-tasks?${actorQuery()}`
  );
  return (data.items || []).map(mapApprovalTaskSummary);
}

export async function getPublishTask(taskId: string): Promise<ApprovalTaskDetail> {
  const data = await request<Record<string, unknown>>(`/api/console-v2/publish-tasks/${encodeURIComponent(taskId)}?${actorQuery()}`);
  return mapApprovalTaskDetail(data);
}

export async function publishTask(taskId: string, comment: string) {
  return request(`/api/console-v2/publish-tasks/${encodeURIComponent(taskId)}/publish?${actorQuery()}`, {
    method: 'POST',
    body: JSON.stringify({ actor: actorBody(), comment })
  });
}

export async function rejectPublishTask(taskId: string, comment: string) {
  return request(`/api/console-v2/publish-tasks/${encodeURIComponent(taskId)}/reject?${actorQuery()}`, {
    method: 'POST',
    body: JSON.stringify({ actor: actorBody(), comment })
  });
}

export async function createStagePromotion(applicationId: string, stageId: string, freightId: string, autoPublish = true) {
  if (!hasAPIBaseURL()) return undefined;
  return request(`/api/apps/${encodeURIComponent(applicationId)}/delivery/stages/${encodeURIComponent(stageId)}/promotions`, {
    method: 'POST',
    body: JSON.stringify({
      actor: actorBody(),
      freight_id: freightId,
      auto_publish: autoPublish
    })
  });
}

export async function createFreightFromVersionSource(
  applicationId: string,
  name: string,
  config: { workloads: VersionSourceWorkloadConfig[] },
  pipelines: VersionSourcePipeline[],
  sourceFingerprint: string
) {
  if (!hasAPIBaseURL()) return undefined;
  const context = await request<BackendFreightCreationContext>(
    `/api/apps/${encodeURIComponent(applicationId)}/freights/creation-context`
  );
  const items = freightItemsFromVersionSource(context, config.workloads, pipelines);
  return request<BackendFreight>(`/api/apps/${encodeURIComponent(applicationId)}/freights`, {
    method: 'POST',
    body: JSON.stringify({
      actor: actorBody(),
      name,
      source_fingerprint: sourceFingerprint,
      items
    })
  });
}

export async function restartRuntimeResource(applicationId: string, stageKey: string, resourceId: string) {
  if (!hasAPIBaseURL()) return undefined;
  return request(`/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/resources/${encodeURIComponent(resourceId)}/restart?${actorQuery()}`, {
    method: 'POST',
    body: '{}'
  });
}

export async function checkRuntimePodLogs(applicationId: string, stageKey: string, resourceId: string, container?: string) {
  if (!hasAPIBaseURL()) return { supported: false, message: '当前为 Mock 数据，无法连接日志流' };
  const query = `${actorQuery()}${container ? `&container=${encodeURIComponent(container)}` : ''}`;
  return request<{ supported?: boolean; message?: string }>(
    `/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/resources/${encodeURIComponent(resourceId)}/logs?${query}`
  );
}

export async function checkRuntimePodTerminal(applicationId: string, stageKey: string, resourceId: string, container?: string) {
  if (!hasAPIBaseURL()) return { supported: false, message: '当前为 Mock 数据，无法连接终端' };
  return request<{ supported?: boolean; message?: string }>(
    `/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/resources/${encodeURIComponent(resourceId)}/terminal?${actorQuery()}`,
    {
      method: 'POST',
      body: JSON.stringify({
        actor: actorBody(),
        container: container || ''
      })
    }
  );
}

export async function completeFreightApproval(freightId: string, targetStageKey: string) {
  if (!hasAPIBaseURL()) return undefined;
  return request(`/api/freights/${encodeURIComponent(freightId)}/approvals`, {
    method: 'POST',
    body: JSON.stringify({
      actor: actorBody(),
      target_stage_key: targetStageKey,
      decision: 'approved',
      comment: '控制台审批通过'
    })
  });
}

export async function completeStageVerification(applicationId: string, stageKey: string, freightId: string) {
  if (!hasAPIBaseURL()) return undefined;
  return request(`/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/verification`, {
    method: 'POST',
    body: JSON.stringify({
      actor: actorBody(),
      freight_id: freightId,
      status: 'passed',
      comment: '控制台人工验证通过',
      sync_status: 'Synced',
      health_status: 'Healthy',
      agent_status: 'ready'
    })
  });
}

async function mapFreights(applicationId: string, freights: BackendFreight[], stages: BackendStage[]): Promise<DeploymentFreight[]> {
  const details = await Promise.all(
    freights.map((freight) => request<BackendFreight>(`/api/freights/${encodeURIComponent(freight.id)}`))
  );
  const stageByFreight = new Map<string, string[]>();
  const eligibleByFreight = new Map<string, string[]>();
  stages.forEach((stage) => {
    const stageId = stageKeyOf(stage);
    const freightId = stage.current_freight_id || stage.currentFreightId || '';
    if (freightId) stageByFreight.set(freightId, [...(stageByFreight.get(freightId) || []), stageId]);
  });
  await Promise.all(stages.map(async (stage) => {
    const stageId = stageKeyOf(stage);
    const data = await request<{ items: BackendFreight[] }>(
      `/api/apps/${encodeURIComponent(applicationId)}/delivery/stages/${encodeURIComponent(stageId)}/eligible-freights`
    );
    (data.items || []).forEach((freight) => {
      if (!freight.id) return;
      eligibleByFreight.set(freight.id, [...new Set([...(eligibleByFreight.get(freight.id) || []), stageId])]);
    });
  }));
  return details.map((detail) => {
    const freight = detail.freight || detail;
    const items = detail.items || detail.freight_items || detail.freightItems || freight.items || [];
    const id = freight.id;
    return {
      id,
      name: freight.name || id,
      createdAt: formatTime(freight.created_at || freight.createdAt),
      source: freight.status === 'archived' ? '已归档' : '后端 Freight',
      sourceFingerprint: freight.source_fingerprint || freight.sourceFingerprint || '',
      commit: freight.commit_sha || freight.commitSHA || freight.commit || '-',
      currentStages: stageByFreight.get(id) || [],
      eligibleStages: eligibleByFreight.get(id) || [],
      workloads: items.length ? mapFreightItems(items) : [{
        name: id,
        displayName: freight.name || id,
        pipeline: '后端版本包',
        version: freight.name || id,
        image: '-',
        digest: '-',
        status: 'healthy' as Status,
        containers: [{
          name: 'app',
          pipeline: '后端版本包',
          version: freight.name || id,
          image: '-',
          digest: '-',
          status: 'healthy' as Status
        }]
      }]
    };
  });
}

function mapFreightsFromWorkspace(
  freights: BackendFreight[],
  stages: BackendStage[],
  detailsById: Record<string, BackendFreight>,
  eligibleByStage: Record<string, BackendFreight[]>
): DeploymentFreight[] {
  const stageByFreight = new Map<string, string[]>();
  const eligibleByFreight = new Map<string, string[]>();
  stages.forEach((stage) => {
    const stageId = stageKeyOf(stage);
    const freightId = stage.current_freight_id || stage.currentFreightId || '';
    if (freightId) stageByFreight.set(freightId, [...(stageByFreight.get(freightId) || []), stageId]);
    (eligibleByStage[stageId] || []).forEach((freight) => {
      if (!freight.id) return;
      eligibleByFreight.set(freight.id, [...new Set([...(eligibleByFreight.get(freight.id) || []), stageId])]);
    });
  });
  return freights.map((summary) => {
    const detail = detailsById[summary.id] || summary;
    const freight = detail.freight || detail;
    const items = detail.items || detail.freight_items || detail.freightItems || freight.items || [];
    const id = freight.id || summary.id;
    return {
      id,
      name: freight.name || summary.name || id,
      createdAt: formatTime(freight.created_at || freight.createdAt || summary.created_at || summary.createdAt),
      source: freight.status === 'archived' ? '已归档' : '后端 Freight',
      sourceFingerprint: freight.source_fingerprint || freight.sourceFingerprint || summary.source_fingerprint || summary.sourceFingerprint || '',
      commit: freight.commit_sha || freight.commitSHA || freight.commit || summary.commit_sha || summary.commitSHA || '-',
      currentStages: stageByFreight.get(id) || [],
      eligibleStages: eligibleByFreight.get(id) || [],
      workloads: items.length ? mapFreightItems(items) : [{
        name: id,
        displayName: freight.name || summary.name || id,
        pipeline: '后端版本包',
        version: freight.name || summary.name || id,
        image: '-',
        digest: '-',
        status: 'healthy' as Status,
        containers: [{
          name: 'app',
          pipeline: '后端版本包',
          version: freight.name || summary.name || id,
          image: '-',
          digest: '-',
          status: 'healthy' as Status
        }]
      }]
    };
  });
}

function mapFreightItems(items: BackendFreightItem[]): DeploymentFreight['workloads'] {
  const groups = new Map<string, DeploymentFreight['workloads'][number]>();
  items.forEach((item) => {
    const container = mapFreightItemContainer(item);
    const workloadId = item.workload_id || item.workloadId || item.workload_name || item.workloadName || workloadNameFromItemName(item.name, container.name) || 'workload';
    const displayName = freightWorkloadDisplayName(item, workloadId, container.name);
    const existing = groups.get(workloadId);
    if (existing) {
      existing.containers = [...(existing.containers || []), container];
      existing.pipeline = freightWorkloadSummary(existing.containers, 'pipeline');
      existing.version = freightWorkloadSummary(existing.containers, 'version');
      existing.image = freightWorkloadSummary(existing.containers, 'image');
      existing.digest = freightWorkloadSummary(existing.containers, 'digest');
      return;
    }
    groups.set(workloadId, {
      name: workloadId,
      displayName,
      pipeline: container.pipeline,
      version: container.version,
      image: container.image,
      digest: container.digest,
      status: 'healthy' as Status,
      containers: [container]
    });
  });
  return [...groups.values()];
}

function freightItemsFromVersionSource(
  context: BackendFreightCreationContext,
  workloads: VersionSourceWorkloadConfig[],
  pipelines: VersionSourcePipeline[]
) {
  const releasesByContainer = context.latest_releases_by_workload_container || context.latestReleasesByWorkloadContainer || {};
  const artifactsByContainer = context.latest_artifacts_by_workload_container || context.latestArtifactsByWorkloadContainer || {};
  const releasesByWorkload = context.latest_releases_by_workload || context.latestReleasesByWorkload || {};
  const artifactsByWorkload = context.latest_artifacts_by_workload || context.latestArtifactsByWorkload || {};
  return workloads.flatMap((workload) => workload.containers.map((container) => {
    const containerName = normalizeFreightContainerName(container.name);
    if (container.imageSource.mode === 'custom') {
      const imageRef = (container.imageSource.customImage || '').trim();
      if (!imageRef) {
        throw new APIError('invalid_argument', `${workload.name} / ${containerName} 缺少自定义镜像`);
      }
      return {
        workload_id: workload.id,
        container_name: containerName,
        source_type: 'custom_image',
        image_ref: imageRef
      };
    }

    const key = freightTargetKey(workload.id, containerName);
    const release = releasesByContainer[key] || (containerName === 'app' ? releasesByWorkload[workload.id] : undefined);
    const artifact = artifactsByContainer[key] || (containerName === 'app' ? artifactsByWorkload[workload.id] : undefined);
    if (!release?.id && !artifact?.id) {
      const pipelineName = pipelines.find((pipeline) => pipeline.id === container.imageSource.pipelineId)?.name || container.imageSource.pipelineId || '未选择流水线';
      throw new APIError('failed_precondition', `${workload.name} / ${containerName} 未找到流水线 ${pipelineName} 的成功构建产物`);
    }
    return {
      workload_id: workload.id,
      container_name: containerName,
      source_type: 'pipeline_artifact',
      release_id: release?.id || '',
      build_artifact_id: release?.id ? '' : artifact?.id || ''
    };
  }));
}

function freightTargetKey(workloadId: string, containerName: string) {
  return `${workloadId}/${normalizeFreightContainerName(containerName)}`;
}

function normalizeFreightContainerName(name: string) {
  return name.trim() || 'app';
}

function mapFreightItemContainer(item: BackendFreightItem): DeploymentFreightContainer {
  const containerName = item.container_name || item.containerName || containerNameFromItemName(item.name) || 'app';
  const image = item.image || item.image_ref || item.imageRef || item.uri || imageFromRepositoryTag(item) || '-';
  const digest = item.digest || item.image_digest || item.imageDigest || '-';
  const sourceMode = freightContainerSourceMode(item);
  return {
    name: containerName,
    pipeline: item.source_key || item.sourceKey || item.source_type || item.sourceType || item.type || '版本源',
    version: sourceMode === 'custom' ? '自定义' : (versionFromImage(image) || item.release_id || item.releaseId || '-'),
    image,
    digest,
    status: 'healthy',
    sourceMode
  };
}

function freightContainerSourceMode(item: BackendFreightItem): 'pipeline' | 'custom' {
  const value = [
    item.image_source_mode,
    item.imageSourceMode,
    item.source_type,
    item.sourceType,
    item.type
  ].filter(Boolean).join(' ').toLowerCase();
  return value.includes('custom') ? 'custom' : 'pipeline';
}

function freightWorkloadDisplayName(item: BackendFreightItem, workloadId: string, containerName: string) {
  const explicit = item.workload_display_name || item.workloadDisplayName || item.workload_name || item.workloadName;
  if (explicit) return explicit;
  const name = item.name || workloadId;
  const suffix = ` / ${containerName}`;
  return name.endsWith(suffix) ? name.slice(0, -suffix.length) : name;
}

function containerNameFromItemName(name?: string) {
  const parts = (name || '').split(' / ');
  return parts.length > 1 ? parts[parts.length - 1] : '';
}

function workloadNameFromItemName(name: string | undefined, containerName: string) {
  if (!name) return '';
  const suffix = ` / ${containerName}`;
  return name.endsWith(suffix) ? name.slice(0, -suffix.length) : name;
}

function imageFromRepositoryTag(item: BackendFreightItem) {
  const repository = item.image_repository || item.imageRepository || '';
  const tag = item.image_tag || item.imageTag || '';
  if (!repository) return '';
  return tag ? `${repository}:${tag}` : repository;
}

function freightWorkloadSummary(
  containers: DeploymentFreightContainer[] | undefined,
  field: keyof Pick<DeploymentFreightContainer, 'pipeline' | 'version' | 'image' | 'digest'>
) {
  if (!containers?.length) return '-';
  if (containers.length === 1) return containers[0][field];
  return `${containers.length} 个镜像`;
}

function mapApprovalGates(items: BackendApprovalGate[]): ApprovalGateSummary[] {
  return items.map((item) => ({
    targetStageKey: item.target_stage_key || item.targetStageKey || '',
    targetStageName: item.target_stage_name || item.targetStageName || item.target_stage_key || item.targetStageKey || '',
    pendingCount: item.pending_count ?? item.pendingCount ?? 0,
    canReview: item.can_review ?? item.canReview ?? false,
    requiredPermissionCode: item.required_permission_code || item.requiredPermissionCode || ''
  })).filter((item) => item.targetStageKey);
}

function mapPublishGates(items: BackendPublishGate[]): PublishGateSummary[] {
  return items.map((item) => ({
    targetStageKey: item.target_stage_key || item.targetStageKey || '',
    targetStageName: item.target_stage_name || item.targetStageName || item.target_stage_key || item.targetStageKey || '',
    pendingCount: item.pending_count ?? item.pendingCount ?? 0,
    canPublish: item.can_publish ?? item.canPublish ?? false,
    requiredPermissionCode: item.required_permission_code || item.requiredPermissionCode || ''
  })).filter((item) => item.targetStageKey);
}

function mapApprovalTaskSummary(input: unknown): ApprovalTaskSummary {
  const data = input as Record<string, any>;
  const diffType = data.diff_type || data.diffType || 'compare';
  return {
    id: data.id || data.promotion_id || data.promotionId || '',
    promotionId: data.promotion_id || data.promotionId || data.id || '',
    freightId: data.freight_id || data.freightId || '',
    freightName: data.freight_name || data.freightName || data.freight_id || data.freightId || '',
    targetStageKey: data.target_stage_key || data.targetStageKey || '',
    targetStageName: data.target_stage_name || data.targetStageName || data.target_stage_key || data.targetStageKey || '',
    requestedBy: data.requested_by || data.requestedBy || '',
    requestedAt: data.requested_at || data.requestedAt || '',
    message: data.message || '',
    diffType: diffType === 'first_deploy' ? 'first_deploy' : 'compare'
  };
}

function mapApprovalTaskDetail(data: Record<string, any>): ApprovalTaskDetail {
  const diffType = data.diff_type || data.diffType || 'compare';
  return {
    task: mapApprovalTaskSummary(data.task || {}),
    diffType: diffType === 'first_deploy' ? 'first_deploy' : 'compare',
    currentFreight: mapApprovalFreightSummary(data.current_freight || data.currentFreight),
    pendingFreight: mapApprovalFreightSummary(data.pending_freight || data.pendingFreight) || { id: '', name: '', createdAt: '' },
    imageChanges: (data.image_changes || data.imageChanges || []).map((item: Record<string, any>) => ({
      workloadId: item.workload_id || item.workloadId || '',
      workloadName: item.workload_name || item.workloadName || '',
      containerName: item.container_name || item.containerName || '',
      currentImage: item.current_image || item.currentImage || '-',
      pendingImage: item.pending_image || item.pendingImage || '-',
      currentVersion: item.current_version || item.currentVersion || '-',
      pendingVersion: item.pending_version || item.pendingVersion || '-'
    })),
    configDiff: data.config_diff || data.configDiff || '',
    deployItems: (data.deploy_items || data.deployItems || []).map((item: Record<string, any>) => ({
      workloadId: item.workload_id || item.workloadId || '',
      workloadName: item.workload_name || item.workloadName || '',
      containerName: item.container_name || item.containerName || '',
      version: item.version || '-',
      image: item.image || '-'
    }))
  };
}

function mapApprovalFreightSummary(input: unknown): { id: string; name: string; createdAt: string } | undefined {
  if (!input) return undefined;
  const data = input as Record<string, any>;
  return {
    id: data.id || '',
    name: data.name || data.id || '',
    createdAt: data.created_at || data.createdAt || ''
  };
}

async function mapStage(applicationId: string, stage: BackendStage, index: number): Promise<{ stage: DeploymentStage; error?: string }> {
  const stageKey = stageKeyOf(stage);
  let runtimeError = '';
  let runtime: DeploymentStage['workloads'] = [];
  try {
    runtime = await loadStageRuntime(applicationId, stageKey);
  } catch (error) {
    runtimeError = `${stage.display_name || stage.displayName || stageKey} 运行态加载失败：${formatAPIError(error)}`;
  }
  const workloads = runtime.length ? runtime : [{
    name: 'runtime',
    displayName: runtimeError ? '运行态不可用' : '运行态未上报',
    kind: 'Deployment',
    image: '-',
    replicas: '0/0',
    pods: '0 Running',
    cpu: '-',
    memory: '-',
    status: (runtimeError ? 'warning' : 'pending') as Status,
    podDetails: []
  }];
  const currentFreight = stage.current_freight_id || stage.currentFreightId || '';
  return {
    stage: {
    id: stageKey,
    key: stageKey,
    name: stage.display_name || stage.displayName || stageKey,
    colorToken: colorTokenFromHex(stage.color),
    x: 0,
    y: 0,
    row: stage.layout_row ?? stage.layoutRow ?? 0,
    col: stage.layout_column ?? stage.layoutColumn ?? index,
    lane: stage.layout_column ?? stage.layoutColumn ?? index,
    freightId: currentFreight,
    promotionPolicy: (stage.requires_approval || stage.requiresApproval) ? 'approval_required' : 'manual',
    requiresVerification: Boolean(stage.requires_verification || stage.requiresVerification),
    clusterBindingId: stage.bound_cluster_id || stage.boundClusterId || '',
    cluster: stage.bound_cluster_name || stage.boundClusterName || '未绑定集群',
    namespace: stageKey,
    sync: normalizeSync(stage.sync_status || stage.syncStatus),
    syncStatus: normalizeArgoSync(stage.sync_status || stage.syncStatus),
    healthStatus: normalizeArgoHealth(stage.health_status || stage.healthStatus || stage.operation_state || stage.operationState),
    status: normalizeStatus(stage.health_status || stage.healthStatus || stage.operation_state || stage.operationState),
    progress: (stage.health_status || stage.healthStatus) === 'Healthy' ? 100 : 0,
    configurableWorkloadIds: workloads.map((workload) => workload.name),
    checks: [
      (stage.requires_approval || stage.requiresApproval) ? '需要审批' : '无需审批',
      (stage.requires_verification || stage.requiresVerification) ? '需要人工验证' : '无需验证'
    ],
    workloads
    },
    error: runtimeError
  };
}

function mapStageFromWorkspace(
  stage: BackendStage,
  index: number,
  runtimeResources: BackendRuntimeResource[],
  runtimeError?: string
): DeploymentStage {
  const stageKey = stageKeyOf(stage);
  const runtime = mapRuntimeResources(runtimeResources || []);
  const workloads = runtime.length ? runtime : [{
    name: 'runtime',
    displayName: runtimeError ? '运行态不可用' : '运行态未上报',
    kind: 'Deployment',
    image: '-',
    replicas: '0/0',
    pods: '0 Running',
    cpu: '-',
    memory: '-',
    status: (runtimeError ? 'warning' : 'pending') as Status,
    podDetails: []
  }];
  const currentFreight = stage.current_freight_id || stage.currentFreightId || '';
  return {
    id: stageKey,
    key: stageKey,
    name: stage.display_name || stage.displayName || stageKey,
    colorToken: colorTokenFromHex(stage.color),
    x: 0,
    y: 0,
    row: stage.layout_row ?? stage.layoutRow ?? 0,
    col: stage.layout_column ?? stage.layoutColumn ?? index,
    lane: stage.layout_column ?? stage.layoutColumn ?? index,
    freightId: currentFreight,
    promotionPolicy: (stage.requires_approval || stage.requiresApproval) ? 'approval_required' : 'manual',
    requiresVerification: Boolean(stage.requires_verification || stage.requiresVerification),
    clusterBindingId: stage.bound_cluster_id || stage.boundClusterId || '',
    cluster: stage.bound_cluster_name || stage.boundClusterName || '未绑定集群',
    namespace: stageKey,
    sync: normalizeSync(stage.sync_status || stage.syncStatus),
    syncStatus: normalizeArgoSync(stage.sync_status || stage.syncStatus),
    healthStatus: normalizeArgoHealth(stage.health_status || stage.healthStatus || stage.operation_state || stage.operationState),
    status: normalizeStatus(stage.health_status || stage.healthStatus || stage.operation_state || stage.operationState),
    progress: (stage.health_status || stage.healthStatus) === 'Healthy' ? 100 : 0,
    configurableWorkloadIds: workloads.map((workload) => workload.name),
    checks: [
      (stage.requires_approval || stage.requiresApproval) ? '需要审批' : '无需审批',
      (stage.requires_verification || stage.requiresVerification) ? '需要人工验证' : '无需验证'
    ],
    workloads
  };
}

async function loadStageRuntime(applicationId: string, stageKey: string): Promise<DeploymentStage['workloads']> {
  const data = await request<{ items: BackendRuntimeResource[] }>(
    `/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/resources?${actorQuery()}`
  );
  return mapRuntimeResources(data.items || []);
}

function mapRuntimeResources(resources: BackendRuntimeResource[]): DeploymentStage['workloads'] {
  const owners = resources.filter((resource) => ['Deployment', 'StatefulSet', 'DaemonSet'].includes(resource.kind || ''));
  const pods = resources.filter((resource) => resource.kind === 'Pod');
  return owners.map((owner) => {
    const ownerPods = pods.filter((pod) => visibleParentName(pod) === owner.name || pod.name?.startsWith(`${owner.name}-`));
    const ready = owner.ready ?? ownerPods.filter((pod) => normalizeStatus(pod.health_status || pod.healthStatus || pod.status) === 'healthy').length;
    const desired = owner.desired ?? Math.max(ownerPods.length, ready);
    const image = owner.containers?.[0]?.image || ownerPods[0]?.containers?.[0]?.image || '-';
    return {
      resourceId: owner.id || '',
      namespace: owner.namespace || '-',
      name: owner.name || 'workload',
      displayName: owner.name || 'Workload',
      kind: owner.kind || 'Deployment',
      image: versionFromImage(image) || image,
      replicas: `${ready}/${desired}`,
      pods: `${ready}/${Math.max(ownerPods.length, desired)} Running`,
      cpu: '-',
      memory: '-',
      status: normalizeStatus(owner.health_status || owner.healthStatus || owner.status),
      podDetails: ownerPods.map((pod) => ({
        resourceId: pod.id || '',
        namespace: pod.namespace || owner.namespace || '',
        containerName: pod.containers?.[0]?.name || '',
        name: pod.name || 'pod',
        status: normalizeStatus(pod.health_status || pod.healthStatus || pod.status),
        ready: pod.containers?.every((container) => container.ready) ? '1/1' : '0/1',
        restarts: pod.containers?.reduce((sum, container) => sum + (container.restart_count ?? container.restartCount ?? 0), 0) || 0,
        node: pod.node_name || pod.nodeName || '-',
        podIp: pod.pod_ip || pod.podIP || pod.podIp || '-',
        image: pod.containers?.[0]?.image || image,
        cpu: '-',
        memory: '-',
        age: '-'
      }))
    };
  });
}

function mapEdges(stages: BackendStage[]): DeploymentEdge[] {
  const stageKeys = new Set(stages.map(stageKeyOf));
  const edges = new Map<string, DeploymentEdge>();
  stages.forEach((stage) => {
    const to = stageKeyOf(stage);
    const upstream = stage.upstream_stage_keys || stage.upstreamStageKeys || [];
    upstream.forEach((from) => {
      if (stageKeys.has(from)) edges.set(`${from}-${to}`, { fromStageId: from, toStageId: to, rule: '晋级' });
    });
    const downstream = stage.downstream_stage_keys || stage.downstreamStageKeys || [];
    downstream.forEach((target) => {
      if (stageKeys.has(target)) edges.set(`${to}-${target}`, { fromStageId: to, toStageId: target, rule: '晋级' });
    });
  });
  if (!edges.size) {
    const ordered = stages.slice().sort((a, b) => (a.order || 0) - (b.order || 0));
    ordered.forEach((stage, index) => {
      const next = ordered[index + 1];
      if (next) edges.set(`${stageKeyOf(stage)}-${stageKeyOf(next)}`, { fromStageId: stageKeyOf(stage), toStageId: stageKeyOf(next), rule: '晋级' });
    });
  }
  return [...edges.values()];
}

function stageKeyOf(stage: BackendStage) {
  return stage.stage_key || stage.stageKey || stage.delivery_stage_id || stage.deliveryStageId || '';
}

function visibleParentName(resource: BackendRuntimeResource) {
  return resource.parent_name || resource.parentName || '';
}

function colorTokenFromHex(value?: string) {
  const normalized = (value || '').toUpperCase();
  const map: Record<string, string> = {
    '#0072B2': 'geek-blue',
    '#56B4E9': 'sky-blue',
    '#009E73': 'mint-green',
    '#44AA99': 'turquoise-green',
    '#F0E442': 'lemon-yellow',
    '#E69F00': 'amber-orange',
    '#D55E00': 'rust-orange',
    '#882255': 'wine-purple',
    '#CC79A7': 'lilac-purple',
    '#77AADD': 'smoky-blue'
  };
  return map[normalized] || 'smoky-blue';
}

function normalizeStatus(value?: string): Status {
  const normalized = (value || '').toLowerCase();
  if (normalized.includes('healthy') || normalized.includes('running') || normalized.includes('synced') || normalized === 'ready') return 'healthy';
  if (normalized.includes('progress') || normalized.includes('pending') || normalized.includes('waiting')) return 'running';
  if (normalized.includes('degrad') || normalized.includes('fail') || normalized.includes('error')) return 'danger';
  if (normalized.includes('warn')) return 'warning';
  return 'pending';
}

function normalizeSync(value?: string) {
  if (!value) return '未知';
  if (value === 'Synced') return 'Synced';
  if (value === 'OutOfSync') return 'OutOfSync';
  return value;
}

function normalizeArgoSync(value?: string) {
  const normalized = (value || '').toLowerCase();
  if (normalized === 'synced' || normalized.includes('已同步')) return 'Synced';
  if (normalized === 'outofsync' || normalized.includes('out of sync') || normalized.includes('未同步') || normalized.includes('不同步')) return 'OutOfSync';
  return 'Unknown';
}

function normalizeArgoHealth(value?: string) {
  const normalized = (value || '').toLowerCase();
  if (normalized.includes('healthy') || normalized.includes('健康')) return 'Healthy';
  if (normalized.includes('progress') || normalized.includes('running') || normalized.includes('进行中')) return 'Progressing';
  if (normalized.includes('degrad') || normalized.includes('fail') || normalized.includes('error') || normalized.includes('降级')) return 'Degraded';
  if (normalized.includes('suspend') || normalized.includes('暂停')) return 'Suspended';
  if (normalized.includes('missing') || normalized.includes('缺失')) return 'Missing';
  return 'Unknown';
}

function versionFromImage(value?: string) {
  if (!value) return '';
  const tag = value.split(':').pop();
  return tag && tag !== value ? tag : value;
}

function formatAPIError(error: unknown) {
  if (error instanceof APIError) {
    return `${error.message}${error.status ? ` (${error.status}${error.code ? ` ${error.code}` : ''})` : ''}`;
  }
  return error instanceof Error ? error.message : '请求处理失败';
}

function formatTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (input: number) => String(input).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
