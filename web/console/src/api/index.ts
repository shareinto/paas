import * as mock from './mock';
import { APIError, hasAPIBaseURL, openWebSocket, request, requestText, streamSSE, type PageResult, type WebSocketConnection } from './client';

export type { Tenant, Project, Application, ApplicationSource, BuildPipeline, BuildPipelineSource, BuildRun, AuditLog, Freight, FreightItem, ImageBundleImage, Workload, WorkloadType, WorkloadImageSourceMode, WorkloadStageConfig, ReleaseCandidate, BuildArtifactCandidate, StageDefinition, DeliveryFlowTemplate, DeliveryFlowTemplateStage, DeliveryFlowTemplateEdge, StageClusterBinding, ClusterOption, AppStage, RuntimeResource, FreightCreationContext, CreateFreightInput, CreatePromotionInput, FreightApprovalInput, StageVerificationInput, SourceRepository, RepositoryBranch, RepositoryTreeItem, BuildSpecSuggestion, JenkinsJobTemplate, BuildType, BuildEnvironment, RuntimeEnvironment, BuildTemplate } from './mock';

const DEFAULT_APP_ID = 'app_1';
const DEFAULT_BUILD_RUN_ID = 'build_128';
const SOURCE_REPOSITORY_PAGE_SIZE = 100;

export async function login(account: string, password: string) {
  if (!hasAPIBaseURL()) return mock.login(account, password);
  return request<{ token: string; userName: string }>('/api/auth/local/login', { method: 'POST', body: JSON.stringify({ account, password }) });
}

export async function oidcLoginURL() {
  if (!hasAPIBaseURL()) return mock.oidcLoginURL();
  const data = await request<{ redirect_url: string }>('/api/auth/oidc/start', { method: 'POST', body: '{}' });
  return data.redirect_url;
}

export async function listProjects() {
  if (!hasAPIBaseURL()) return mock.listProjects();
  const data = await request<PageResult<mock.Project>>('/api/projects?page=1&page_size=50');
  return data.items;
}

export async function listTenants() {
  if (!hasAPIBaseURL()) return mock.listTenants();
  const data = await request<PageResult<mock.Tenant>>('/api/tenants?page=1&page_size=100');
  return data.items.map(mapTenant);
}

export async function createTenant(input: { name: string; displayName: string; description?: string }) {
  if (!hasAPIBaseURL()) return mock.createTenant(input);
  const item = await request<any>('/api/tenants', {
    method: 'POST',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, name: input.name, display_name: input.displayName, description: input.description || '' })
  });
  return mapTenant(item);
}

export async function updateTenant(id: string, input: { displayName: string; description?: string }) {
  if (!hasAPIBaseURL()) return mock.updateTenant(id, input);
  const item = await request<any>(`/api/tenants/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, display_name: input.displayName, description: input.description || '' })
  });
  return mapTenant(item);
}

export async function createProject(input: { tenantId: string; name: string; displayName: string; description?: string }) {
  if (!hasAPIBaseURL()) return mock.createProject(input);
  const item = await request<any>('/api/projects', {
    method: 'POST',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, tenant_id: input.tenantId, name: input.name, display_name: input.displayName, description: input.description || '' })
  });
  return {
    id: item.id,
    tenantId: item.tenant_id || item.tenantId || input.tenantId,
    name: item.name,
    displayName: item.display_name || item.displayName || item.name,
    description: item.description || '',
    tenant: item.tenant || '',
    owner: item.owner || '平台管理员',
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

export async function deleteProject(id: string) {
  if (!hasAPIBaseURL()) return mock.deleteProject(id);
  await request<void>(`/api/projects/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } })
  });
}

export async function listApplications(projectId?: string) {
  if (!hasAPIBaseURL()) return mock.listApplications(projectId);
  const path = projectId
    ? `/api/projects/${encodeURIComponent(projectId)}/applications?page=1&page_size=50`
    : '/api/applications?page=1&page_size=50';
  const data = await request<PageResult<mock.Application>>(path);
  return data.items.map(mapApplication);
}

export async function getApplication(id: string) {
  if (!hasAPIBaseURL()) return mock.getApplication(id);
  const item = await request<any>(`/api/applications/${encodeURIComponent(id)}`);
  return mapApplication(item);
}

export async function getApplicationSources(id: string) {
  if (!hasAPIBaseURL()) return mock.getApplicationSources(id);
  const data = await request<{ items: any[] }>(`/api/applications/${encodeURIComponent(id)}/source`);
  return data.items.map(mapApplicationSource);
}

export async function updateApplication(id: string, input: { displayName: string; description?: string; disabled?: boolean }) {
  if (!hasAPIBaseURL()) return mock.updateApplication(id, input);
  const item = await request<any>(`/api/applications/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(applicationPayload(input))
  });
  return mapApplication(item);
}

export async function deleteApplication(id: string) {
  if (!hasAPIBaseURL()) return mock.deleteApplication(id);
  await request<void>(`/api/applications/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } })
  });
}

export async function listBuilds() {
  if (!hasAPIBaseURL()) return mock.listBuilds();
  const data = await request<PageResult<any>>('/api/builds?page=1&page_size=50');
  return data.items.map(mapBuildRun);
}

export async function listApplicationBuilds(applicationId: string) {
  if (!hasAPIBaseURL()) return mock.listApplicationBuilds(applicationId);
  const data = await request<PageResult<any>>(`/api/apps/${encodeURIComponent(applicationId)}/builds?page=1&page_size=50`);
  return data.items.map(mapBuildRun);
}

export async function buildLog(buildRunId = DEFAULT_BUILD_RUN_ID) {
  if (!hasAPIBaseURL()) return mock.buildLog();
  return parseSSELog(await requestText(`/api/builds/${encodeURIComponent(buildRunId)}/logs/stream`));
}

export async function cancelBuild(buildRunId: string) {
  if (!hasAPIBaseURL()) return mock.cancelBuild(buildRunId);
  const item = await request<any>(`/api/builds/${encodeURIComponent(buildRunId)}/cancel`, {
    method: 'POST',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } })
  });
  return mapBuildRun(item);
}

export function streamBuildLog(buildRunId: string, onLog: (text: string) => void, onStatus?: (status: string) => void) {
  if (!hasAPIBaseURL()) {
    let closed = false;
    mock.buildLog().then((text) => {
      if (!closed) onLog(text);
    });
    return () => {
      closed = true;
    };
  }
  let close: () => void = () => undefined;
  close = streamSSE(`/api/builds/${encodeURIComponent(buildRunId)}/logs/stream`, (event) => {
    if (event.event === 'log') {
      onLog(event.data);
      return;
    }
    if (event.event === 'status') {
      onStatus?.(event.data);
      if (['succeeded', 'failed', 'aborted', 'unstable'].includes(event.data)) close();
      return;
    }
    if (event.event === 'error') {
      onStatus?.('error');
    }
  }, () => {
    onStatus?.('reconnecting');
  });
  return close;
}

export async function triggerBuild(applicationId: string, input: { gitRef?: string; commitSha?: string } = {}) {
  if (!hasAPIBaseURL()) return mock.triggerBuild(applicationId, input);
  const item = await request<any>(`/api/apps/${encodeURIComponent(applicationId)}/builds`, {
    method: 'POST',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, git_ref: input.gitRef || '', commit_sha: input.commitSha || '' })
  });
  return mapBuildRun(item);
}

export async function listBuildPipelines(applicationId: string) {
  if (!hasAPIBaseURL()) return mock.listBuildPipelines(applicationId);
  const data = await request<PageResult<any>>(`/api/apps/${encodeURIComponent(applicationId)}/build-pipelines?page=1&page_size=50`);
  const pipelines = data.items.map(mapBuildPipeline);
  const withSources = await Promise.all(pipelines.map(async (pipeline) => {
    const sources = await listBuildPipelineSources(pipeline.id);
    return { ...pipeline, sources };
  }));
  return withSources;
}

export async function listBuildPipelineSources(pipelineId: string) {
  if (!hasAPIBaseURL()) return mock.listBuildPipelineSources(pipelineId);
  const data = await request<{ items: any[] }>(`/api/build-pipelines/${encodeURIComponent(pipelineId)}/sources`);
  return data.items.map(mapBuildPipelineSource);
}

export async function createBuildPipeline(applicationId: string, input: { name: string; displayName: string; description?: string; runtimeEnvironmentIds?: string[]; sources: mock.BuildPipelineSource[] }) {
  if (!hasAPIBaseURL()) return mock.createBuildPipeline(applicationId, input as any);
  const runtimeEnvironmentIds = cleanRuntimeEnvironmentIds(input.runtimeEnvironmentIds);
  const item = await request<any>(`/api/apps/${encodeURIComponent(applicationId)}/build-pipelines`, {
    method: 'POST',
    body: JSON.stringify({
      actor: { type: 'user', id: 'usr_admin' },
      name: input.name,
      display_name: input.displayName,
      description: input.description || '',
      runtime_environment_ids: runtimeEnvironmentIds,
      sources: input.sources.map(pipelineSourcePayload)
    })
  });
  return mapBuildPipeline(item);
}

export async function updateBuildPipeline(pipelineId: string, input: { displayName: string; description?: string; runtimeEnvironmentIds?: string[]; sources: mock.BuildPipelineSource[] }) {
  if (!hasAPIBaseURL()) return mock.updateBuildPipeline(pipelineId, input as any);
  const runtimeEnvironmentIds = cleanRuntimeEnvironmentIds(input.runtimeEnvironmentIds);
  const item = await request<any>(`/api/build-pipelines/${encodeURIComponent(pipelineId)}`, {
    method: 'PATCH',
    body: JSON.stringify({
      actor: { type: 'user', id: 'usr_admin' },
      display_name: input.displayName,
      description: input.description || '',
      runtime_environment_ids: runtimeEnvironmentIds,
      sources: input.sources.map(pipelineSourcePayload)
    })
  });
  return mapBuildPipeline(item);
}

export async function deleteBuildPipeline(pipelineId: string) {
  if (!hasAPIBaseURL()) return mock.deleteBuildPipeline(pipelineId);
  await request<void>(`/api/build-pipelines/${encodeURIComponent(pipelineId)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } })
  });
}

export async function triggerBuildPipeline(pipelineId: string, input: { gitRef?: string; commitSha?: string } = {}) {
  if (!hasAPIBaseURL()) return mock.triggerBuildPipeline(pipelineId, input);
  const item = await request<any>(`/api/build-pipelines/${encodeURIComponent(pipelineId)}/builds`, {
    method: 'POST',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, git_ref: input.gitRef || '', commit_sha: input.commitSha || '' })
  });
  return mapBuildRun(item);
}

export async function listAuditLogs() {
  if (!hasAPIBaseURL()) return mock.listAuditLogs();
  const data = await request<PageResult<any>>('/api/audit/logs?page=1&page_size=50');
  return data.items.map((item) => ({
    id: item.id,
    actor: item.actor || item.actor_id || '平台用户',
    action: item.action,
    resource: item.resource || [item.resource_type, item.resource_id].filter(Boolean).join(' '),
    result: item.result === 'succeeded' ? '成功' : item.result || '-',
    summary: item.summary,
    time: item.time || formatTime(item.occurred_at || item.occurredAt)
  }));
}

export async function listFreights(applicationId?: string) {
  if (!hasAPIBaseURL()) return mock.listFreights(applicationId);
  const targetApplicationId = applicationId || DEFAULT_APP_ID;
  const data = await request<PageResult<any>>(`/api/apps/${encodeURIComponent(targetApplicationId)}/freights?page=1&page_size=50`);
  return data.items.map(mapFreight);
}

export async function getFreightCreationContext(applicationId = DEFAULT_APP_ID) {
  if (!hasAPIBaseURL()) return mock.getFreightCreationContext(applicationId);
  const item = await request<any>(`/api/apps/${encodeURIComponent(applicationId)}/freights/creation-context`);
  return mapFreightCreationContext(item);
}

export async function listEligibleFreights(applicationId: string, stageId: string) {
  if (!hasAPIBaseURL()) return mock.listEligibleFreights(applicationId, stageId);
  const items = await request<any[]>(`/api/apps/${encodeURIComponent(applicationId)}/delivery/stages/${encodeURIComponent(stageId)}/eligible-freights`);
  return items.map(mapFreight);
}

export async function getFreight(freightId: string) {
  if (!hasAPIBaseURL()) return mock.getFreight(freightId);
  const detail = await request<any>(`/api/freights/${encodeURIComponent(freightId)}`);
  return mapFreightDetail(detail);
}

export async function createFreight(applicationId: string, input: mock.CreateFreightInput) {
  if (!hasAPIBaseURL()) return mock.createFreight(applicationId, input);
  return mapFreight(await request<any>(`/api/apps/${encodeURIComponent(applicationId)}/freights`, {
    method: 'POST',
    body: JSON.stringify({
      actor: { type: 'user', id: 'usr_admin' },
      name: input.name,
      items: input.items.map((item) => ({
        workload_id: item.workloadId,
        source_type: item.sourceType,
        release_id: item.releaseId || '',
        build_artifact_id: item.buildArtifactId || '',
        image_ref: item.imageRef || ''
      }))
    })
  }));
}

export async function deleteFreight(freightId: string) {
  if (!hasAPIBaseURL()) return mock.deleteFreight(freightId);
  return mapFreight(await request<any>(`/api/freights/${encodeURIComponent(freightId)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } })
  }));
}

export async function listWorkloads(applicationId: string) {
  if (!hasAPIBaseURL()) return mock.listWorkloads(applicationId);
  const data = await request<{ items: any[] }>(`/api/applications/${encodeURIComponent(applicationId)}/workloads`);
  return data.items.map(mapWorkload);
}

export async function createWorkload(applicationId: string, input: { name: string; displayName: string; description?: string; workloadType: mock.WorkloadType; imageSourceMode?: mock.WorkloadImageSourceMode; pipelineId?: string; customImage?: string; pipelineName?: string; replicas?: number }) {
  if (!hasAPIBaseURL()) return mock.createWorkload(applicationId, input);
  const item = await request<any>(`/api/applications/${encodeURIComponent(applicationId)}/workloads`, {
    method: 'POST',
    body: JSON.stringify({
      actor: { type: 'user', id: 'usr_admin' },
      name: input.name,
      display_name: input.displayName,
      description: input.description || '',
      workload_type: input.workloadType,
      image_source_mode: input.imageSourceMode || 'pipeline_artifact',
      pipeline_id: input.pipelineId || '',
      custom_image: input.customImage || '',
      pipeline_name: input.pipelineName || '',
      replicas: input.replicas
    })
  });
  return mapWorkload(item);
}

export async function updateWorkload(applicationId: string, workloadId: string, input: { name?: string; displayName: string; description?: string; workloadType?: mock.WorkloadType; imageSourceMode?: mock.WorkloadImageSourceMode; pipelineId?: string }) {
  if (!hasAPIBaseURL()) return mock.updateWorkload(applicationId, workloadId, input);
  const item = await request<any>(`/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}`, {
    method: 'PUT',
    body: JSON.stringify({
      actor: { type: 'user', id: 'usr_admin' },
      name: input.name || '',
      display_name: input.displayName,
      description: input.description || '',
      workload_type: input.workloadType || '',
      image_source_mode: input.imageSourceMode || '',
      pipeline_id: input.pipelineId || ''
    })
  });
  return mapWorkload(item);
}

export async function deleteWorkload(applicationId: string, workloadId: string) {
  if (!hasAPIBaseURL()) return mock.deleteWorkload(applicationId, workloadId);
  const item = await request<any>(`/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } })
  });
  return mapWorkload(item);
}

export async function listWorkloadStageConfigs(applicationId: string, workloadId: string) {
  if (!hasAPIBaseURL()) return mock.listWorkloadStageConfigs(applicationId, workloadId);
  const data = await request<{ items: any[] }>(`/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}/stage-configs`);
  return data.items.map(mapWorkloadStageConfig);
}

export async function getWorkloadDefaultConfig(applicationId: string, workloadId: string) {
  if (!hasAPIBaseURL()) return mock.getWorkloadDefaultConfig(applicationId, workloadId);
  const item = await request<any>(`/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}/default-config`);
  return mapWorkloadStageConfig(item);
}

export async function getDeliveryFlowTemplate(tenantId = 'tenant_1') {
	if (!hasAPIBaseURL()) return mock.getDeliveryFlowTemplate(tenantId);
	return mapDeliveryFlowTemplate(await request<any>(`/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template`));
}

export async function replaceDeliveryFlowTemplateGraph(tenantId: string, input: { stages: mock.DeliveryFlowTemplateStage[]; edges: Pick<mock.DeliveryFlowTemplateEdge, 'fromStageKey' | 'toStageKey'>[]; deletedStageKeys?: string[] }) {
	if (!hasAPIBaseURL()) return mock.replaceDeliveryFlowTemplateGraph(tenantId, input);
	return mapDeliveryFlowTemplate(await request<any>(`/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/graph`, {
		method: 'PUT',
		body: JSON.stringify({
			actor: { type: 'user', id: 'usr_admin' },
			stages: input.stages.map(stageTemplatePayload),
			edges: input.edges.map((edge) => ({ from_stage_key: edge.fromStageKey, to_stage_key: edge.toStageKey })),
			deleted_stage_keys: input.deletedStageKeys || []
		})
	}));
}

export async function saveDeliveryFlowTemplateStage(tenantId: string, input: Partial<mock.DeliveryFlowTemplateStage> & { stageKey: string }) {
  if (!hasAPIBaseURL()) return mock.saveDeliveryFlowTemplateStage(tenantId, input);
  const path = input.id
    ? `/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/stages/${encodeURIComponent(input.stageKey)}`
    : `/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/stages`;
  const item = await request<any>(path, { method: input.id ? 'PATCH' : 'POST', body: JSON.stringify(stageTemplatePayload(input)) });
  return mapDeliveryFlowTemplateStage(item);
}

export async function deleteDeliveryFlowTemplateStage(tenantId: string, stageKey: string) {
  if (!hasAPIBaseURL()) return mock.deleteDeliveryFlowTemplateStage(tenantId, stageKey);
  const item = await request<any>(`/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/stages/${encodeURIComponent(stageKey)}`, { method: 'DELETE', body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } }) });
  return mapDeliveryFlowTemplateStage(item);
}

export async function listClusterOptions(tenantId?: string) {
  if (!hasAPIBaseURL()) return mock.listClusterOptions(tenantId);
  const tenantQuery = tenantId ? `tenant_id=${encodeURIComponent(tenantId)}&` : '';
  const data = await request<PageResult<any>>(`/api/clusters?${tenantQuery}page=1&page_size=100&actor_id=usr_admin`);
  return data.items.map(mapClusterOption);
}

export async function replaceStageClusterBindings(tenantId: string, stageKey: string, clusterIds: string[]) {
  if (!hasAPIBaseURL()) return mock.replaceStageClusterBindings(tenantId, stageKey, clusterIds);
  const clusters = await listClusterOptions(tenantId);
  const data = await request<{ items: any[] }>(`/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/stages/${encodeURIComponent(stageKey)}/cluster-bindings`, {
    method: 'PUT',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, clusters: clusterIds.map((clusterId) => ({ cluster_id: clusterId, cluster_name: clusters.find((cluster) => cluster.id === clusterId)?.name || clusterId })) })
  });
  return data.items.map(mapStageClusterBinding);
}

export async function listStageClusterBindings(tenantId: string, stageKey: string) {
  if (!hasAPIBaseURL()) return mock.listStageClusterBindings(tenantId, stageKey);
  const data = await request<{ items: any[] }>(`/api/tenants/${encodeURIComponent(tenantId)}/delivery-flow-template/stages/${encodeURIComponent(stageKey)}/cluster-bindings`);
  return data.items.map(mapStageClusterBinding);
}

export async function listAppStages(applicationId: string) {
  if (!hasAPIBaseURL()) return mock.listAppStages(applicationId);
  const data = await request<{ items: any[] }>(`/api/apps/${encodeURIComponent(applicationId)}/stages`);
  return data.items.map(mapAppStage);
}

export async function listRuntimeResources(applicationId: string, stageKey: string) {
  if (!hasAPIBaseURL()) return mock.listRuntimeResources(applicationId, stageKey);
  const data = await request<{ items: any[] }>(`/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/resources?actor_id=usr_admin`);
  return data.items.map(mapRuntimeResource).filter(isVisibleRuntimeResource);
}

export function streamRuntimeResources(applicationId: string, stageKey: string, onSnapshot: (items: mock.RuntimeResource[]) => void, onStatus?: (status: string) => void) {
  if (!hasAPIBaseURL()) return mock.streamRuntimeResources(applicationId, stageKey, onSnapshot, onStatus);
  return streamSSE(`/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/resources/stream?actor_id=usr_admin`, (event) => {
    if (event.event === 'snapshot') {
      try {
        const data = JSON.parse(event.data);
        onSnapshot((data.items || []).map(mapRuntimeResource).filter(isVisibleRuntimeResource));
      } catch {
        onStatus?.('error');
      }
      return;
    }
    if (event.event === 'status') {
      onStatus?.(event.data);
      return;
    }
    if (event.event === 'error') {
      onStatus?.(event.data || 'error');
    }
  }, () => onStatus?.('reconnecting'));
}

export async function restartRuntimeResource(applicationId: string, stageKey: string, resourceId: string) {
  if (!hasAPIBaseURL()) return mock.restartRuntimeResource(applicationId, stageKey, resourceId);
  return request<any>(`/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/resources/${encodeURIComponent(resourceId)}/restart?actor_id=usr_admin`, { method: 'POST', body: '{}' });
}

export function streamRuntimePodLogs(applicationId: string, stageKey: string, namespace: string, pod: string, container: string | undefined, onLog: (text: string) => void, onStatus?: (status: string) => void) {
  if (!hasAPIBaseURL()) return mock.streamRuntimePodLogs(applicationId, stageKey, namespace, pod, container, onLog, onStatus);
  const query = `?actor_id=usr_admin${container ? `&container=${encodeURIComponent(container)}` : ''}`;
  return streamSSE(`/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(pod)}/logs/stream${query}`, (event) => {
    if (event.event === 'log') {
      onLog(event.data);
      return;
    }
    if (event.event === 'status') {
      onStatus?.(event.data);
      return;
    }
    if (event.event === 'error') {
      onStatus?.(event.data || 'error');
    }
  }, () => onStatus?.('reconnecting'));
}

export function openRuntimePodTerminal(applicationId: string, stageKey: string, namespace: string, pod: string, container: string | undefined, handlers: { onOpen?: () => void; onMessage?: (text: string) => void; onClose?: () => void; onError?: () => void }): WebSocketConnection {
  if (!hasAPIBaseURL()) return mock.openRuntimePodTerminal(applicationId, stageKey, namespace, pod, container, handlers);
  const query = `?actor_id=usr_admin${container ? `&container=${encodeURIComponent(container)}` : ''}`;
  return openWebSocket(`/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/runtime/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(pod)}/terminal${query}`, {
    onOpen: handlers.onOpen,
    onMessage: handlers.onMessage,
    onClose: handlers.onClose,
    onError: () => handlers.onError?.()
  });
}

export async function saveWorkloadStageConfig(applicationId: string, workloadId: string, stageKey: string, input: Partial<mock.WorkloadStageConfig>) {
  if (!hasAPIBaseURL()) return mock.saveWorkloadStageConfig(applicationId, workloadId, stageKey, input);
  const item = await request<any>(`/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}/stage-configs/${encodeURIComponent(stageKey)}`, {
    method: 'PUT',
    body: JSON.stringify(workloadStageConfigPayload(input))
  });
  return mapWorkloadStageConfig(item);
}

export async function saveWorkloadDefaultConfig(applicationId: string, workloadId: string, input: Partial<mock.WorkloadStageConfig>) {
  if (!hasAPIBaseURL()) return mock.saveWorkloadDefaultConfig(applicationId, workloadId, input);
  const item = await request<any>(`/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}/default-config`, {
    method: 'PUT',
    body: JSON.stringify(workloadStageConfigPayload(input))
  });
  return mapWorkloadStageConfig(item);
}

export async function createPromotion(input: mock.CreatePromotionInput, applicationId?: string, stageId?: string) {
  if (!hasAPIBaseURL()) return mock.createPromotion(input);
  const options = {
    method: 'POST',
    body: JSON.stringify(promotionPayload(input))
  };
  const path = applicationId && stageId
    ? `/api/apps/${encodeURIComponent(applicationId)}/delivery/stages/${encodeURIComponent(stageId)}/promotions`
    : '/api/promotions';
  try {
    return await request<any>(path, options);
  } catch (error) {
    if (!applicationId || !stageId || !isRouteNotFound(error)) throw error;
    return request<any>('/api/promotions', options);
  }
}

export async function completeFreightApproval(freightId: string, input: mock.FreightApprovalInput) {
  if (!hasAPIBaseURL()) return mock.completeFreightApproval(freightId, input);
  return request<any>(`/api/freights/${encodeURIComponent(freightId)}/approvals`, {
    method: 'POST',
    body: JSON.stringify({
      actor: { type: 'user', id: 'usr_admin' },
      target_stage_key: input.targetStageKey,
      decision: input.decision,
      comment: input.comment || ''
    })
  });
}

export async function completeStageVerification(applicationId: string, stageKey: string, input: mock.StageVerificationInput) {
  if (!hasAPIBaseURL()) return mock.completeStageVerification(applicationId, stageKey, input);
  return request<any>(`/api/apps/${encodeURIComponent(applicationId)}/stages/${encodeURIComponent(stageKey)}/verification`, {
    method: 'POST',
    body: JSON.stringify({
      actor: { type: 'user', id: 'usr_admin' },
      freight_id: input.freightId,
      status: input.status,
      comment: input.comment || '',
      sync_status: input.syncStatus || '',
      health_status: input.healthStatus || '',
      agent_status: input.agentStatus || ''
    })
  });
}

export async function listSourceRepositories(projectId?: string) {
  if (!hasAPIBaseURL()) return mock.listSourceRepositories(projectId);
  const projects = await listProjects();
  const matchedProject = projectId ? projects.find((project) => project.id === projectId) : undefined;
  const targetProjects = projectId ? [matchedProject || { id: projectId, displayName: '' }] : projects;
  const all = await Promise.all(targetProjects.map((project: any) => listProjectSourceRepositories(project)));
  return all.flat();
}

async function listProjectSourceRepositories(project: { id: string; displayName?: string }) {
  const repositories: mock.SourceRepository[] = [];
  let page = 1;
  while (true) {
    const data = await request<PageResult<any>>(`/api/projects/${encodeURIComponent(project.id)}/source-repositories?page=${page}&page_size=${SOURCE_REPOSITORY_PAGE_SIZE}`);
    repositories.push(...data.items.map((item) => mapSourceRepository(item, project.displayName || '')));
    const pageSize = data.page_size || SOURCE_REPOSITORY_PAGE_SIZE;
    const total = data.total ?? repositories.length;
    if (repositories.length >= total || data.items.length < pageSize) break;
    page = (data.page || page) + 1;
  }
  return repositories;
}

export async function getSourceRepository(id: string) {
  if (!hasAPIBaseURL()) return mock.getSourceRepository(id);
  const item = await request<any>(`/api/source-repositories/${encodeURIComponent(id)}`);
  return mapSourceRepository(item);
}

export async function createSourceRepository(input: { projectId: string; name: string; displayName: string; description?: string; defaultBranch: string }) {
  if (!hasAPIBaseURL()) return mock.createSourceRepository(input);
  const actor = { type: 'user', id: 'usr_admin' };
  const item = await request<any>('/api/source-repositories', {
    method: 'POST',
    body: JSON.stringify({ actor, project_id: input.projectId, name: input.name, display_name: input.displayName, description: input.description || '', default_branch: input.defaultBranch })
  });
  return mapSourceRepository(item);
}

export async function deleteSourceRepository(id: string) {
  if (!hasAPIBaseURL()) return mock.deleteSourceRepository(id);
  await request<void>(`/api/source-repositories/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } })
  });
}

export async function listRepositoryApplications(repositoryId: string) {
  if (!hasAPIBaseURL()) return mock.listRepositoryApplications();
  const data = await request<{ items: any[] }>(`/api/source-repositories/${encodeURIComponent(repositoryId)}/applications`);
  return data.items.map((item) => ({ id: item.id, name: item.name, displayName: item.display_name || item.displayName || item.name }));
}

export async function scanRepositoryJava(repositoryId: string, ref: string) {
  if (!hasAPIBaseURL()) return mock.scanRepositoryJava();
  const data = await request<{ items: any[] }>(`/api/source-repositories/${encodeURIComponent(repositoryId)}/scan/java`, { method: 'POST', body: JSON.stringify({ ref }) });
  return data.items.map((item) => ({
    sourcePath: item.source_path,
    buildCommand: item.build_command,
    artifactCopyCommand: item.artifact_copy_command || item.artifactCopyCommand || '',
    runtimeBaseImage: item.runtime_base_image,
    evidence: item.evidence || []
  }));
}

export async function listRepositoryBranches(repositoryId: string) {
  if (!hasAPIBaseURL()) return mock.listRepositoryBranches(repositoryId);
  const data = await request<{ items: any[] }>(`/api/source-repositories/${encodeURIComponent(repositoryId)}/branches`);
  return data.items.map((item) => ({
    name: item.name,
    default: !!item.default
  }));
}

export async function listRepositoryTree(repositoryId: string, ref: string, path = '') {
  if (!hasAPIBaseURL()) return mock.listRepositoryTree(repositoryId, ref, path);
  const params = new URLSearchParams();
  if (ref) params.set('ref', ref);
  if (path) params.set('path', path);
  const suffix = params.toString() ? `?${params.toString()}` : '';
  const data = await request<{ items: any[] }>(`/api/source-repositories/${encodeURIComponent(repositoryId)}/tree${suffix}`);
  return data.items.map((item) => ({
    name: item.name,
    path: item.path,
    type: item.type
  }));
}

export async function syncRepositoryPermissions(repositoryId: string) {
  if (!hasAPIBaseURL()) return mock.syncRepositoryPermissions();
  return request<any>(`/api/source-repositories/${encodeURIComponent(repositoryId)}/permission-sync`, { method: 'POST', body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } }) });
}

export async function listJenkinsJobTemplates() {
  if (!hasAPIBaseURL()) return mock.listJenkinsJobTemplates();
  const data = await request<PageResult<any>>('/api/jenkins-job-templates?page=1&page_size=100');
  return data.items.map(mapJenkinsJobTemplate);
}

export async function listBuildTypes() {
  return listBuildEnvironments();
}

export async function listAdminJenkinsJobTemplates() {
  if (!hasAPIBaseURL()) return mock.listAdminJenkinsJobTemplates();
  const data = await request<PageResult<any>>('/api/admin/jenkins-job-templates?page=1&page_size=100');
  return data.items.map(mapJenkinsJobTemplate);
}

export async function listAdminBuildTypes() {
  return listAdminBuildEnvironments();
}

export async function listBuildEnvironments() {
  if (!hasAPIBaseURL()) return mock.listBuildEnvironments();
  const data = await request<PageResult<any>>('/api/build-environments?page=1&page_size=100');
  return data.items.map(mapBuildEnvironment);
}

export async function listAdminBuildEnvironments() {
  if (!hasAPIBaseURL()) return mock.listAdminBuildEnvironments();
  const data = await request<PageResult<any>>('/api/admin/build-environments?page=1&page_size=100');
  return data.items.map(mapBuildEnvironment);
}

export async function createBuildEnvironment(input: Partial<mock.BuildEnvironment>) {
  if (!hasAPIBaseURL()) return mock.createBuildEnvironment(input);
  const item = await request<any>('/api/admin/build-environments', {
    method: 'POST',
    body: JSON.stringify(buildEnvironmentPayload(input, true))
  });
  return mapBuildEnvironment(item);
}

export async function updateBuildEnvironment(id: string, input: Partial<mock.BuildEnvironment>) {
  if (!hasAPIBaseURL()) return mock.updateBuildEnvironment(id, input);
  const item = await request<any>(`/api/admin/build-environments/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(buildEnvironmentPayload(input, false))
  });
  return mapBuildEnvironment(item);
}

export async function deleteBuildEnvironment(id: string) {
  if (!hasAPIBaseURL()) return mock.deleteBuildEnvironment(id);
  await request<void>(`/api/admin/build-environments/${encodeURIComponent(id)}`, { method: 'DELETE', body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } }) });
}

export async function listRuntimeEnvironments() {
  if (!hasAPIBaseURL()) return mock.listRuntimeEnvironments();
  const data = await request<PageResult<any>>('/api/runtime-environments?page=1&page_size=100');
  return data.items.map(mapRuntimeEnvironment);
}

export async function listAdminRuntimeEnvironments() {
  if (!hasAPIBaseURL()) return mock.listAdminRuntimeEnvironments();
  const data = await request<PageResult<any>>('/api/admin/runtime-environments?page=1&page_size=100');
  return data.items.map(mapRuntimeEnvironment);
}

export async function createRuntimeEnvironment(input: Partial<mock.RuntimeEnvironment>) {
  if (!hasAPIBaseURL()) return mock.createRuntimeEnvironment(input);
  const item = await request<any>('/api/admin/runtime-environments', {
    method: 'POST',
    body: JSON.stringify(runtimeEnvironmentPayload(input, true))
  });
  return mapRuntimeEnvironment(item);
}

export async function updateRuntimeEnvironment(id: string, input: Partial<mock.RuntimeEnvironment>) {
  if (!hasAPIBaseURL()) return mock.updateRuntimeEnvironment(id, input);
  const item = await request<any>(`/api/admin/runtime-environments/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(runtimeEnvironmentPayload(input, false))
  });
  return mapRuntimeEnvironment(item);
}

export async function deleteRuntimeEnvironment(id: string) {
  if (!hasAPIBaseURL()) return mock.deleteRuntimeEnvironment(id);
  await request<void>(`/api/admin/runtime-environments/${encodeURIComponent(id)}`, { method: 'DELETE', body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } }) });
}

function buildEnvironmentPayload(input: Partial<mock.BuildEnvironment>, includeName: boolean) {
  return {
    actor: { type: 'user', id: 'usr_admin' },
	    ...(includeName ? { name: input.name } : {}),
	    description: input.description || '',
	    build_image: input.buildImage,
    status: input.status,
    is_default: !!input.isDefault
  };
}

function runtimeEnvironmentPayload(input: Partial<mock.RuntimeEnvironment>, includeName: boolean) {
  return {
    actor: { type: 'user', id: 'usr_admin' },
	    ...(includeName ? { name: input.name } : {}),
	    description: input.description || '',
	    runtime_base_image: input.runtimeBaseImage,
	    artifact_deploy_path: input.artifactDeployPath,
    dockerfile_path: input.dockerfilePath,
    selector_labels: cleanStringRecord(input.selectorLabels),
    status: input.status,
    is_default: !!input.isDefault
  };
}

export async function getBuildTemplate() {
  if (!hasAPIBaseURL()) return mock.getBuildTemplate();
  const item = await request<any>('/api/admin/build-template');
  return mapBuildTemplate(item);
}

export async function updateBuildTemplate(input: { content: string }) {
  if (!hasAPIBaseURL()) return mock.updateBuildTemplate(input);
  const item = await request<any>('/api/admin/build-template', { method: 'PUT', body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, content: input.content }) });
  return mapBuildTemplate(item);
}

export async function createJenkinsJobTemplate(input: { name: string; jenkinsfileContent?: string; xmlContent?: string; isDefault?: boolean }) {
  if (!hasAPIBaseURL()) return mock.createJenkinsJobTemplate(input);
  const form = new FormData();
  form.set('actor_id', 'usr_admin');
  form.set('name', input.name);
  form.set('is_default', input.isDefault ? 'true' : 'false');
  form.set('jenkinsfile', new Blob([input.jenkinsfileContent || input.xmlContent || ''], { type: 'text/plain' }), 'Jenkinsfile');
  const item = await request<any>('/api/admin/jenkins-job-templates', { method: 'POST', body: form });
  return mapJenkinsJobTemplate(item);
}

export async function createBuildType(input: { name: string; jenkinsfileContent?: string; xmlContent?: string; isDefault?: boolean }) {
  if (!hasAPIBaseURL()) return mock.createBuildType(input);
  const form = new FormData();
  form.set('actor_id', 'usr_admin');
  form.set('name', input.name);
  form.set('is_default', input.isDefault ? 'true' : 'false');
  form.set('jenkinsfile', new Blob([input.jenkinsfileContent || input.xmlContent || ''], { type: 'text/plain' }), 'Jenkinsfile');
  const item = await request<any>('/api/admin/build-types', { method: 'POST', body: form });
  return mapJenkinsJobTemplate(item);
}

export async function updateJenkinsJobTemplate(id: string, input: { status: string; isDefault?: boolean }) {
  if (!hasAPIBaseURL()) return mock.updateJenkinsJobTemplate(id, input);
  const item = await request<any>(`/api/admin/jenkins-job-templates/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, status: input.status, is_default: !!input.isDefault })
  });
  return mapJenkinsJobTemplate(item);
}

export async function updateBuildType(id: string, input: { status: string; isDefault?: boolean }) {
  if (!hasAPIBaseURL()) return mock.updateBuildType(id, input);
  const item = await request<any>(`/api/admin/build-types/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, status: input.status, is_default: !!input.isDefault })
  });
  return mapJenkinsJobTemplate(item);
}

export async function uploadJenkinsJobTemplateRevision(id: string, input: { jenkinsfileContent?: string; xmlContent?: string }) {
  if (!hasAPIBaseURL()) return mock.uploadJenkinsJobTemplateRevision(id, input);
  const form = new FormData();
  form.set('actor_id', 'usr_admin');
  form.set('jenkinsfile', new Blob([input.jenkinsfileContent || input.xmlContent || ''], { type: 'text/plain' }), 'Jenkinsfile');
  const item = await request<any>(`/api/admin/jenkins-job-templates/${encodeURIComponent(id)}/revisions`, { method: 'POST', body: form });
  return mapJenkinsJobTemplate(item);
}

export async function uploadBuildTypeRevision(id: string, input: { jenkinsfileContent?: string; xmlContent?: string }) {
  if (!hasAPIBaseURL()) return mock.uploadBuildTypeRevision(id, input);
  const form = new FormData();
  form.set('actor_id', 'usr_admin');
  form.set('jenkinsfile', new Blob([input.jenkinsfileContent || input.xmlContent || ''], { type: 'text/plain' }), 'Jenkinsfile');
  const item = await request<any>(`/api/admin/build-types/${encodeURIComponent(id)}/revisions`, { method: 'POST', body: form });
  return mapJenkinsJobTemplate(item);
}

export async function deleteBuildType(id: string) {
  if (!hasAPIBaseURL()) return mock.deleteBuildType(id);
  await request<void>(`/api/admin/build-types/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' } })
  });
}

export type CreateApplicationSource = {
  key: string;
  displayName?: string;
  sourceRepositoryId: string;
  jenkinsTemplateId?: string;
  buildEnvironmentId?: string;
  sourcePath: string;
  defaultRef: string;
  isPrimary?: boolean;
  buildSpec: { sourcePath: string; buildCommand: string; artifactCopyCommand: string; runtimeBaseImage?: string; artifactDeployPath?: string; defaultRef: string };
};

export async function createApplication(input: { projectId: string; name: string; displayName: string; description?: string }) {
  if (!hasAPIBaseURL()) return mock.createApplication(input);
  const item = await request<any>('/api/applications', {
    method: 'POST',
    body: JSON.stringify({
      project_id: input.projectId,
      name: input.name,
      ...applicationPayload(input)
    })
  });
  return { id: item.id, name: item.name, displayName: item.display_name || item.displayName || item.name };
}

function applicationPayload(input: { displayName: string; description?: string; disabled?: boolean }) {
  return {
    actor: { type: 'user', id: 'usr_admin' },
    display_name: input.displayName,
    description: input.description || '',
    disabled: !!input.disabled
  };
}

function promotionPayload(input: mock.CreatePromotionInput) {
  return {
    actor: { type: 'user', id: 'usr_admin' },
    freight_id: input.freightId,
    target_stage_key: input.targetStageKey || '',
    target_cluster_ids: input.targetClusterIds || [],
    namespace_override: input.namespaceOverride || '',
    message: input.message || ''
  };
}

function isRouteNotFound(error: unknown) {
  return error instanceof APIError && error.status === 404 && error.code === 'request_failed';
}

function formatTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}

function mapFreight(item: any): mock.Freight {
  if (item.freight) return mapFreightDetail(item);
  const items = item.items || item.freight_items || item.freightItems || [];
  return {
    id: item.id,
    version: item.version || item.name || item.id,
    image: item.image || item.image_uri || item.imageURI || item.uri || (items.length ? `${items.length} 个 Workload` : '-'),
    digest: item.digest || item.image_digest || item.imageDigest || '-',
    commit: item.commit || item.commit_sha || item.commitSHA || '-',
    createdAt: item.createdAt || formatTime(item.created_at || item.createdAt),
    status: item.status || 'available',
    items: items.map(mapFreightItem)
  };
}

function mapFreightDetail(detail: any): mock.Freight {
  const freight = mapFreight({ ...(detail.freight || detail), items: detail.items || detail.freight_items || detail.freightItems || [] });
  return freight;
}

function mapFreightItem(item: any): mock.FreightItem {
  const bundleImages = mapImageBundleImages(item.bundleImages || item.bundle_images || item.imageBundleImages || item.image_bundle_images || []);
  return {
    id: item.id,
    workloadId: item.workloadId || item.workload_id || '',
    workloadName: item.workloadName || item.workload_name || item.name || '',
    workloadDisplayName: item.workloadDisplayName || item.workload_display_name || item.workload_name || item.name || item.workload_id || '',
    sourceType: item.sourceType || item.source_type || 'pipeline_artifact',
    releaseId: item.releaseId || item.release_id || '',
    buildArtifactId: item.buildArtifactId || item.build_artifact_id || '',
    imageBundleId: item.imageBundleId || item.image_bundle_id || '',
    bundleImages,
    image: item.image || item.imageRef || item.image_ref || item.uri || [item.image_repository, item.image_tag].filter(Boolean).join(':') || '-',
    digest: item.digest || item.image_digest || item.imageDigest || '',
    commit: item.commit || item.commit_sha || item.commitSHA || ''
  };
}

function mapFreightCreationContext(item: any): mock.FreightCreationContext {
  const stageEligibility = item.stageEligibility || item.stage_eligibility || {};
  return {
    enabledWorkloads: (item.enabledWorkloads || item.enabled_workloads || []).map(mapWorkload),
    latestReleasesByWorkload: mapRecord(item.latestReleasesByWorkload || item.latest_releases_by_workload || {}, mapReleaseCandidate),
    latestArtifactsByWorkload: mapRecord(item.latestArtifactsByWorkload || item.latest_artifacts_by_workload || {}, mapBuildArtifactCandidate),
    stageEligibility: Object.fromEntries(Object.entries(stageEligibility).map(([key, value]) => [key, Array.isArray(value) ? value.map(String) : []])),
    stages: (item.stages || []).map(mapStageDefinition)
  };
}

function mapRecord<T>(record: Record<string, any>, mapper: (item: any) => T): Record<string, T> {
  return Object.fromEntries(Object.entries(record).map(([key, value]) => [key, mapper(value)]));
}

function mapReleaseCandidate(item: any): mock.ReleaseCandidate {
  const bundleImages = mapImageBundleImages(item.bundleImages || item.bundle_images || item.imageBundleImages || item.image_bundle_images || []);
  return {
    id: item.id,
    workloadId: item.workloadId || item.workload_id || '',
    version: item.version || item.name || item.image_tag || item.imageTag || '',
    image: item.image || item.image_uri || item.imageURI || item.uri || [item.image_repository, item.image_tag].filter(Boolean).join(':') || '-',
    digest: item.digest || item.image_digest || item.imageDigest || '',
    commit: item.commit || item.commit_sha || item.commitSHA || '',
    buildArtifactId: item.buildArtifactId || item.build_artifact_id || '',
    imageBundleId: item.imageBundleId || item.image_bundle_id || '',
    bundleImages,
    createdAt: item.createdAt || formatTime(item.created_at || item.createdAt)
  };
}

function mapBuildArtifactCandidate(item: any): mock.BuildArtifactCandidate {
  return {
    id: item.id,
    workloadId: item.workloadId || item.workload_id || '',
    image: item.image || item.uri || item.image_uri || '-',
    digest: item.digest || item.image_digest || '',
    createdAt: item.createdAt || formatTime(item.created_at || item.createdAt)
  };
}

function mapStageDefinition(item: any): mock.StageDefinition {
  return {
    id: item.id,
    name: item.name,
    approvalRequired: !!(item.approvalRequired || item.approval_required || item.requires_approval),
    approvalCount: item.approvalCount || item.approval_count,
    approverScope: item.approverScope || item.approver_scope,
    selfApprovalForbidden: item.selfApprovalForbidden || item.self_approval_forbidden,
    currentFreightVersion: item.currentFreightVersion || item.current_freight_version || item.current_freight || '',
    replicasSummary: item.replicasSummary || item.replicas_summary || '',
    domainSummary: item.domainSummary || item.domain_summary || '',
    configSummary: item.configSummary || item.config_summary || ''
  };
}

function mapTenant(item: any): mock.Tenant {
  return {
    id: item.id,
    name: item.name,
    displayName: item.display_name || item.displayName || item.name,
    description: item.description || '',
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapSourceRepository(item: any, projectName = ''): mock.SourceRepository {
  return {
    id: item.id,
    projectId: item.project_id || item.projectId || '',
    projectName,
    name: item.name,
    displayName: item.display_name || item.displayName || item.name,
    description: item.description || '',
    gitProvider: item.git_provider || item.gitProvider || 'gitlab',
    httpUrl: item.http_url || item.httpUrl || '',
    sshUrl: item.ssh_url || item.sshUrl || '',
    defaultBranch: item.default_branch || item.defaultBranch || 'main',
    status: item.status || '-',
    associatedApplications: item.associatedApplications || 0,
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapApplication(item: any): mock.Application {
  return {
    id: item.id,
    name: item.name,
    displayName: item.display_name || item.displayName || item.name,
    project: item.project || item.project_name || '',
    projectId: item.project_id || item.projectId || '',
    description: item.description || '',
    runtimeEnvironmentId: item.runtime_environment_id || item.runtimeEnvironmentId || '',
    runtimeEnvironments: (item.runtime_environments || item.runtimeEnvironments || []).map((runtime: any) => ({
	      id: runtime.id,
	      name: runtime.name,
	      runtimeBaseImage: runtime.runtime_base_image || runtime.runtimeBaseImage || '',
	      artifactDeployPath: runtime.artifact_deploy_path || runtime.artifactDeployPath || '',
      dockerfilePath: runtime.dockerfile_path || runtime.dockerfilePath || '',
      selectorLabels: cleanStringRecord(runtime.selector_labels || runtime.selectorLabels)
    })),
    status: item.status || 'active',
    type: item.type || '-',
    stageStatus: item.stage_status || item.stageStatus || '-',
    build: item.build || '-',
    release: item.release || '-',
    owner: item.owner || '-',
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapWorkload(item: any): mock.Workload {
  const stageStatuses = item.stage_statuses || item.stageStatuses || item.env_statuses || item.environments || [];
  return {
    id: item.id,
    applicationId: item.application_id || item.applicationId || '',
    name: item.name || '',
    displayName: item.display_name || item.displayName || item.name || '',
    description: item.description || '',
    workloadType: normalizeWorkloadType(item.workload_type || item.workloadType),
    imageSourceMode: normalizeImageSourceMode(item.image_source_mode || item.imageSourceMode),
    pipelineId: item.pipeline_id || item.pipelineId || '',
    imageSourceName: item.image_source_name || item.imageSourceName || '',
    latestRelease: item.latest_release || item.latestRelease || item.release || '',
    status: item.status || 'enabled',
    stageStatuses: stageStatuses.map((stage: any) => ({
      stageName: stage.stage_name || stage.stageName || stage.stage_key || stage.stageKey || stage.env_name || stage.name || stage.environment || '-',
      releaseVersion: stage.release_version || stage.releaseVersion || stage.release || '-',
      syncStatus: stage.sync_status || stage.syncStatus || stage.sync || '未知',
      healthStatus: stage.health_status || stage.healthStatus || stage.health || '未知'
    })),
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapDeliveryFlowTemplate(item: any): mock.DeliveryFlowTemplate {
	return {
		id: item.id,
		tenantId: item.tenant_id || item.tenantId || '',
		name: item.name || '默认交付流模板',
		stages: (item.stages || []).map(mapDeliveryFlowTemplateStage),
		edges: (item.edges || []).map(mapDeliveryFlowTemplateEdge),
		createdAt: item.createdAt || formatTime(item.created_at),
		updatedAt: item.updatedAt || formatTime(item.updated_at)
	};
}

function mapDeliveryFlowTemplateEdge(item: any): mock.DeliveryFlowTemplateEdge {
	return {
		id: item.id || `${item.from_stage_key || item.fromStageKey}-${item.to_stage_key || item.toStageKey}`,
		tenantId: item.tenant_id || item.tenantId || '',
		templateId: item.template_id || item.templateId || '',
		fromStageKey: item.from_stage_key || item.fromStageKey || '',
		toStageKey: item.to_stage_key || item.toStageKey || ''
	};
}

function mapDeliveryFlowTemplateStage(item: any): mock.DeliveryFlowTemplateStage {
  return {
    id: item.id,
    tenantId: item.tenant_id || item.tenantId || '',
    templateId: item.template_id || item.templateId || '',
    stageKey: item.stage_key || item.stageKey || '',
    displayName: item.display_name || item.displayName || item.stage_key || item.stageKey || '',
    color: item.color || '#1677ff',
    order: item.order || item.stage_order || 1,
    layoutColumn: item.layout_column ?? item.layoutColumn ?? 0,
    layoutRow: item.layout_row ?? item.layoutRow ?? 0,
    status: item.status || 'enabled',
    requiresApproval: !!(item.requires_approval || item.requiresApproval),
    requiresVerification: !!(item.requires_verification || item.requiresVerification),
    approveRoles: item.approve_roles || item.approveRoles || [],
    verifyRoles: item.verify_roles || item.verifyRoles || []
  };
}

function mapStageClusterBinding(item: any): mock.StageClusterBinding {
  return {
    id: item.id,
    tenantId: item.tenant_id || item.tenantId || '',
    stageKey: item.stage_key || item.stageKey || '',
    clusterId: item.cluster_id || item.clusterId || '',
    clusterName: item.cluster_name || item.clusterName || '',
    status: item.status || 'active'
  };
}

function mapRuntimeResource(item: any): mock.RuntimeResource {
  return {
    id: item.id || '',
    clusterId: item.cluster_id || item.clusterId || '',
    tenantId: item.tenant_id || item.tenantId || '',
    applicationId: item.application_id || item.applicationId || '',
    stageKey: item.stage_key || item.stageKey || '',
    group: item.group || '',
    version: item.version || '',
    kind: item.kind || '',
    namespace: item.namespace || '',
    name: item.name || '',
    parentKind: item.parent_kind || item.parentKind || '',
    parentNamespace: item.parent_namespace || item.parentNamespace || '',
    parentName: item.parent_name || item.parentName || '',
    status: item.status || '',
    healthStatus: item.health_status || item.healthStatus || '',
    message: item.message || '',
    desired: item.desired ?? 0,
    ready: item.ready ?? 0,
    containers: (item.containers || []).map((container: any) => ({
      name: container.name || '',
      image: container.image || '',
      ready: !!container.ready,
      restartCount: container.restart_count ?? container.restartCount ?? 0,
      state: container.state || '',
      message: container.message || ''
    })),
    events: (item.events || []).map((event: any) => ({
      type: event.type || '',
      reason: event.reason || '',
      message: event.message || '',
      count: event.count ?? 0,
      occurredAt: event.occurred_at || event.occurredAt || ''
    })),
    reportedAt: item.reported_at || item.reportedAt || '',
    updatedAt: item.updated_at || item.updatedAt || ''
  };
}

function isVisibleRuntimeResource(resource: mock.RuntimeResource) {
  return ['Pod', 'Deployment', 'StatefulSet', 'DaemonSet'].includes(resource.kind);
}

function mapAppStage(item: any): mock.AppStage {
  return {
    tenantId: item.tenant_id || item.tenantId || '',
    projectId: item.project_id || item.projectId || '',
    applicationId: item.application_id || item.applicationId || '',
    deliveryStageId: item.delivery_stage_id || item.deliveryStageId || '',
    stageKey: item.stage_key || item.stageKey || '',
    displayName: item.display_name || item.displayName || item.stage_key || item.stageKey || '',
    color: item.color || '#1677ff',
    order: item.order || 1,
    layoutColumn: item.layout_column ?? item.layoutColumn ?? 0,
    layoutRow: item.layout_row ?? item.layoutRow ?? 0,
    status: item.status || 'enabled',
    requiresApproval: !!(item.requires_approval || item.requiresApproval),
		requiresVerification: !!(item.requires_verification || item.requiresVerification),
		approveRoles: item.approve_roles || item.approveRoles || [],
		verifyRoles: item.verify_roles || item.verifyRoles || [],
		clusterPoolSize: item.cluster_pool_size || item.clusterPoolSize || 0,
		boundClusterId: item.bound_cluster_id || item.boundClusterId || '',
		boundClusterName: item.bound_cluster_name || item.boundClusterName || '',
			currentFreightId: item.current_freight_id || item.currentFreightId || '',
			currentFreightVersion: item.current_freight_version || item.currentFreightVersion || '',
			syncStatus: item.sync_status || item.syncStatus || '',
			healthStatus: item.health_status || item.healthStatus || '',
			operationState: item.operation_state || item.operationState || '',
			runtimeMessage: item.runtime_message || item.runtimeMessage || '',
			upstreamStageKeys: item.upstream_stage_keys || item.upstreamStageKeys || [],
			downstreamStageKeys: item.downstream_stage_keys || item.downstreamStageKeys || []
		};
	}

function stageTemplatePayload(input: Partial<mock.DeliveryFlowTemplateStage> & { stageKey: string }) {
  return {
    actor: { type: 'user', id: 'usr_admin' },
    stage_key: input.stageKey,
    display_name: input.displayName || input.stageKey,
    color: input.color || '#1677ff',
    order: input.order || 1,
    layout_column: input.layoutColumn || 0,
    layout_row: input.layoutRow || 0,
    status: input.status || 'enabled',
    requires_approval: !!input.requiresApproval,
    requires_verification: !!input.requiresVerification,
    approve_roles: input.approveRoles || [],
    verify_roles: input.verifyRoles || []
  };
}

function mapWorkloadStageConfig(item: any): mock.WorkloadStageConfig {
  return {
    id: item.id || [item.workload_id || item.workloadId, item.stage_key || item.stageKey].filter(Boolean).join(':'),
    workloadId: item.workload_id || item.workloadId || '',
    stageKey: item.stage_key || item.stageKey || '',
    stageName: item.stage_name || item.stageName || item.stage_key || item.stageKey || item.env_name || '-',
    replicas: item.replicas || 1,
    servicePorts: (item.service_ports || item.servicePorts || []).map((port: any) => ({
      name: port.name || 'http',
      port: Number(port.port || 0),
      targetPort: Number(port.target_port || port.targetPort || port.port || 0),
      protocol: port.protocol || 'TCP'
    })),
    resourceRequests: mapResourceList(item.resource_requests || item.resourceRequests),
    resourceLimits: mapResourceList(item.resource_limits || item.resourceLimits),
    probes: (item.probes || []).map((probe: any) => ({
      name: probe.name || probe.type || 'probe',
      type: probe.type || 'HTTP',
      path: probe.path || '',
      port: probe.port ? Number(probe.port) : undefined,
      initialDelaySeconds: probe.initial_delay_seconds || probe.initialDelaySeconds
    })),
    ingressHosts: (item.ingress_hosts || item.ingressHosts || []).map((host: any) => ({
      host: host.host || '',
      path: host.path || '/',
      servicePort: host.service_port || host.servicePort || '',
      tls: !!host.tls
    })),
    envVars: (item.env_vars || item.envVars || []).map((env: any) => ({ name: env.name || env.key || '', value: env.value || '' })),
    configFiles: (item.config_files || item.configFiles || []).map((file: any) => ({ mountPath: file.mount_path || file.mountPath || '', content: file.content || '', base64Encoded: !!(file.base64_encoded || file.base64Encoded) })),
    writableDirs: (item.writable_dirs || item.writableDirs || []).map((dir: any) => ({ mountPath: dir.mount_path || dir.mountPath || '', sizeLimit: dir.size_limit || dir.sizeLimit || '', ownerGroup: dir.owner_group || dir.ownerGroup || '', mode: dir.mode || '' }))
  };
}

function mapResourceList(value: any): mock.WorkloadResourceList {
  return { cpu: value?.cpu || '', memory: value?.memory || '' };
}

function workloadStageConfigPayload(input: Partial<mock.WorkloadStageConfig>) {
  return {
    actor: { type: 'user', id: 'usr_admin' },
    replicas: input.replicas ?? 1,
    service_ports: (input.servicePorts || []).map((port) => ({
      name: port.name,
      port: port.port,
      target_port: port.targetPort,
      protocol: port.protocol || 'TCP'
    })),
    resource_requests: input.resourceRequests || {},
    resource_limits: input.resourceLimits || {},
    probes: (input.probes || []).map((probe) => ({
      name: probe.name,
      type: probe.type,
      path: probe.path || '',
      port: probe.port || 0,
      initial_delay_seconds: probe.initialDelaySeconds || 0
    })),
    ingress_hosts: (input.ingressHosts || []).map((host) => ({
      host: host.host,
      path: host.path || '/',
      service_port: host.servicePort || '',
      tls: !!host.tls
    })),
    env_vars: (input.envVars || []).map((env) => ({ name: env.name, value: env.value })),
    config_files: (input.configFiles || []).map((file) => ({ mount_path: file.mountPath, content: file.content, base64_encoded: !!file.base64Encoded })),
    writable_dirs: (input.writableDirs || []).map((dir) => ({ mount_path: dir.mountPath, size_limit: dir.sizeLimit || '', owner_group: dir.ownerGroup || '', mode: dir.mode || '' }))
  };
}

function normalizeWorkloadType(value: string): mock.WorkloadType {
  return String(value || '').toLowerCase() === 'statefulset' ? 'statefulset' : 'deployment';
}

function normalizeImageSourceMode(value: string): mock.WorkloadImageSourceMode {
  const mode = String(value || '').toLowerCase();
  if (mode === 'custom_image') return 'custom_image';
  if (mode === 'mixed') return 'mixed';
  if (mode === 'none') return 'none';
  return 'pipeline_artifact';
}

function mapApplicationSource(item: any): mock.ApplicationSource {
  const spec = item.build_spec || item.buildSpec || {};
  return {
    id: item.id,
    key: item.key || item.source_key || 'main',
    displayName: item.display_name || item.displayName || item.key || item.source_key || 'main',
    sourceRepositoryId: item.source_repository_id || item.sourceRepositoryId || '',
    jenkinsTemplateId: item.jenkins_template_id || item.jenkinsTemplateId || '',
    buildEnvironmentId: item.build_environment_id || item.buildEnvironmentId || '',
    sourcePath: item.source_path || item.sourcePath || spec.source_path || spec.sourcePath || '.',
    defaultRef: spec.default_ref || spec.defaultRef || item.default_ref || item.defaultRef || 'main',
    isPrimary: !!(item.is_primary || item.isPrimary),
    buildSpec: {
      sourcePath: spec.source_path || spec.sourcePath || item.source_path || item.sourcePath || '.',
      buildCommand: spec.build_command || spec.buildCommand || '',
      artifactCopyCommand: spec.artifact_copy_command || spec.artifactCopyCommand || '',
	      runtimeBaseImage: spec.runtime_base_image || spec.runtimeBaseImage || '',
	      artifactDeployPath: spec.artifact_deploy_path || spec.artifactDeployPath || '',
	      defaultRef: spec.default_ref || spec.defaultRef || item.default_ref || item.defaultRef || 'main'
    }
  };
}

function mapBuildPipeline(item: any): mock.BuildPipeline {
  return {
    id: item.id,
    applicationId: item.application_id || item.applicationId || '',
    name: item.name || '',
    displayName: item.display_name || item.displayName || item.name || '',
    description: item.description || '',
    status: item.status || 'active',
    externalJobName: item.external_job_name || item.externalJobName || '',
    runtimeEnvironments: (item.runtime_environments || item.runtimeEnvironments || []).map(mapRuntimeEnvironmentSnapshot),
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapRuntimeEnvironmentSnapshot(runtime: any): mock.RuntimeEnvironment {
  return {
    id: runtime.id || runtime.runtime_environment_id || runtime.runtimeEnvironmentId || runtime.ID || '',
    name: runtime.name || runtime.Name || '',
    description: runtime.description || '',
    runtimeBaseImage: runtime.runtime_base_image || runtime.runtimeBaseImage || runtime.RuntimeBaseImage || '',
    artifactDeployPath: runtime.artifact_deploy_path || runtime.artifactDeployPath || runtime.ArtifactDeployPath || '',
    dockerfilePath: runtime.dockerfile_path || runtime.dockerfilePath || runtime.DockerfilePath || '',
    selectorLabels: cleanStringRecord(runtime.selector_labels || runtime.selectorLabels || runtime.SelectorLabels),
    status: runtime.status || 'enabled',
    isDefault: !!(runtime.is_default || runtime.isDefault),
    updatedAt: runtime.updatedAt || formatTime(runtime.updated_at || runtime.updatedAt)
  };
}

function cleanRuntimeEnvironmentIds(ids?: string[]) {
  return (ids || []).map((id) => id.trim()).filter(Boolean);
}

function mapBuildPipelineSource(item: any): mock.BuildPipelineSource {
  return {
    ...mapApplicationSource(item),
    pipelineId: item.pipeline_id || item.pipelineId || ''
  };
}

function pipelineSourcePayload(source: mock.BuildPipelineSource) {
  return {
    key: source.key,
    display_name: source.displayName || source.key,
    source_repository_id: source.sourceRepositoryId,
    build_environment_id: source.buildEnvironmentId || '',
    source_path: source.sourcePath,
    default_ref: source.defaultRef,
    is_primary: !!source.isPrimary,
    build_spec: {
      source_path: source.buildSpec.sourcePath || source.sourcePath,
      build_command: source.buildSpec.buildCommand,
      artifact_copy_command: source.buildSpec.artifactCopyCommand,
      runtime_base_image: source.buildSpec.runtimeBaseImage,
      artifact_deploy_path: source.buildSpec.artifactDeployPath,
      default_ref: source.buildSpec.defaultRef || source.defaultRef
    }
  };
}

function mapJenkinsJobTemplate(item: any): mock.JenkinsJobTemplate {
  return {
    id: item.id,
    name: item.name,
    version: item.version || 1,
    status: item.status || 'enabled',
    isDefault: !!(item.is_default || item.isDefault),
    jenkinsfileContent: item.jenkinsfile_content || item.jenkinsfileContent || item.xml_content || item.xmlContent,
    xmlContent: item.jenkinsfile_content || item.jenkinsfileContent || item.xml_content || item.xmlContent,
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapBuildEnvironment(item: any): mock.BuildEnvironment {
  return {
	    id: item.id,
	    name: item.name,
	    description: item.description || '',
	    buildImage: item.build_image || item.buildImage || '',
    status: item.status || 'enabled',
    isDefault: !!(item.is_default || item.isDefault),
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapRuntimeEnvironment(item: any): mock.RuntimeEnvironment {
  return {
    id: item.id,
	    name: item.name,
	    description: item.description || '',
	    runtimeBaseImage: item.runtime_base_image || item.runtimeBaseImage || '',
	    artifactDeployPath: item.artifact_deploy_path || item.artifactDeployPath || '',
    dockerfilePath: item.dockerfile_path || item.dockerfilePath || '',
    selectorLabels: cleanStringRecord(item.selector_labels || item.selectorLabels),
    status: item.status || 'enabled',
    isDefault: !!(item.is_default || item.isDefault),
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapClusterOption(item: any): mock.ClusterOption {
  return {
    id: item.id,
    name: item.name,
    region: item.region || '',
    status: item.status || 'ready',
    labels: cleanStringRecord(item.labels || item.labels_json || item.Labels)
  };
}

function mapImageBundleImages(items: any[]): mock.ImageBundleImage[] {
  return (Array.isArray(items) ? items : []).map((item) => ({
    id: item.id,
    imageBundleId: item.imageBundleId || item.image_bundle_id || item.bundle_id || '',
    buildArtifactId: item.buildArtifactId || item.build_artifact_id || '',
    runtimeEnvironmentId: item.runtimeEnvironmentId || item.runtime_environment_id || '',
    runtimeEnvironmentName: item.runtimeEnvironmentName || item.runtime_environment_name || '',
    image: item.image || item.uri || item.image_uri || [item.image_repository, item.image_tag].filter(Boolean).join(':') || '-',
    digest: item.digest || item.image_digest || '',
    selectorLabels: cleanStringRecord(item.selectorLabels || item.selector_labels),
    isPrimary: !!(item.isPrimary || item.is_primary)
  }));
}

function cleanStringRecord(value: any): mock.StringMap | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const entries = Object.entries(value)
    .map(([key, item]) => [key.trim(), String(item).trim()] as const)
    .filter(([key, item]) => key && item);
  return entries.length > 0 ? Object.fromEntries(entries) : undefined;
}

function mapBuildTemplate(item: any): mock.BuildTemplate {
  return {
    id: item.id,
    name: item.name,
    version: item.version || 1,
    content: item.content || '',
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
}

function mapBuildRun(item: any): mock.BuildRun {
  return {
    id: item.id,
    application: item.application || item.application_id || '',
    pipeline: item.pipeline_display_name || item.pipelineDisplayName || item.pipeline_name || item.pipelineName || '',
    pipelineId: item.pipeline_id || item.pipelineId || '',
    status: buildStatusText(item.status),
    ref: item.ref || item.git_ref || 'main',
    commit: item.commit || item.commit_sha || '',
    startedAt: item.startedAt || formatTime(item.started_at || item.created_at),
    duration: item.duration || '-'
  };
}

function buildStatusText(status: string) {
  const map: Record<string, string> = { queued: '构建中', running: '构建中', succeeded: '成功', failed: '失败', aborted: '已取消', unstable: '不稳定', unknown: '未知' };
  return map[status] || status || '-';
}

function parseSSELog(text: string) {
  if (!text.includes('data:')) return text;
  return text.split('\n').filter((line) => line.startsWith('data:')).map((line) => line.replace(/^data:\s?/, '')).join('\n');
}
