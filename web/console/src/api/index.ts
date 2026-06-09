import * as mock from './mock';
import { hasAPIBaseURL, request, requestText, streamSSE, type PageResult } from './client';

export type { Tenant, Project, Application, ApplicationSource, BuildPipeline, BuildPipelineSource, BuildRun, AuditLog, Freight, SourceRepository, RepositoryBranch, RepositoryTreeItem, BuildSpecSuggestion, JenkinsJobTemplate, BuildType, BuildEnvironment, RuntimeEnvironment, BuildTemplate } from './mock';

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
  return data.items;
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

export async function listApplications() {
  if (!hasAPIBaseURL()) return mock.listApplications();
  const data = await request<PageResult<mock.Application>>('/api/applications?page=1&page_size=50');
  return data.items;
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

export async function updateApplication(id: string, input: { displayName: string; description?: string; disabled?: boolean; runtimeEnvironmentId?: string; runtimeEnvironmentIds?: string[] }) {
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
  const data = await request<PageResult<mock.BuildRun>>('/api/builds?page=1&page_size=50');
  return data.items;
}

export async function buildLog(buildRunId = DEFAULT_BUILD_RUN_ID) {
  if (!hasAPIBaseURL()) return mock.buildLog();
  return parseSSELog(await requestText(`/api/builds/${encodeURIComponent(buildRunId)}/logs/stream`));
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

export async function createBuildPipeline(applicationId: string, input: { name: string; displayName: string; description?: string; sources: mock.BuildPipelineSource[] }) {
  if (!hasAPIBaseURL()) return mock.createBuildPipeline(applicationId, input as any);
  const item = await request<any>(`/api/apps/${encodeURIComponent(applicationId)}/build-pipelines`, {
    method: 'POST',
    body: JSON.stringify({
      actor: { type: 'user', id: 'usr_admin' },
      name: input.name,
      display_name: input.displayName,
      description: input.description || '',
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

export async function listFreights(applicationId = DEFAULT_APP_ID) {
  if (!hasAPIBaseURL()) return mock.listFreights();
  const data = await request<PageResult<any>>(`/api/apps/${encodeURIComponent(applicationId)}/freights?page=1&page_size=50`);
  return data.items.map((item) => ({
    id: item.id,
    version: item.version || item.name || item.id,
    image: item.image || item.image_uri || item.imageURI || item.uri || '-',
    digest: item.digest || item.image_digest || item.imageDigest || '-',
    commit: item.commit || item.commit_sha || item.commitSHA || '-',
    createdAt: item.createdAt || formatTime(item.created_at || item.createdAt)
  }));
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

export async function createApplication(input: { projectId: string; name: string; displayName: string; description?: string; runtimeEnvironmentId?: string; runtimeEnvironmentIds?: string[] }) {
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

function applicationPayload(input: { displayName: string; description?: string; disabled?: boolean; runtimeEnvironmentId?: string; runtimeEnvironmentIds?: string[] }) {
  return {
    actor: { type: 'user', id: 'usr_admin' },
    display_name: input.displayName,
    description: input.description || '',
    disabled: !!input.disabled,
    runtime_environment_id: input.runtimeEnvironmentId || '',
    runtime_environment_ids: input.runtimeEnvironmentIds || []
  };
}

function formatTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
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
      dockerfilePath: runtime.dockerfile_path || runtime.dockerfilePath || ''
    })),
    status: item.status || 'active',
    type: item.type || '-',
    envStatus: item.envStatus || '-',
    build: item.build || '-',
    release: item.release || '-',
    owner: item.owner || '-',
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
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
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
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
    status: item.status || 'enabled',
    isDefault: !!(item.is_default || item.isDefault),
    updatedAt: item.updatedAt || formatTime(item.updated_at || item.updatedAt)
  };
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
  const map: Record<string, string> = { queued: '排队中', running: '运行中', succeeded: '成功', failed: '失败', aborted: '已中止', unstable: '不稳定', unknown: '未知' };
  return map[status] || status || '-';
}

function parseSSELog(text: string) {
  if (!text.includes('data:')) return text;
  return text.split('\n').filter((line) => line.startsWith('data:')).map((line) => line.replace(/^data:\s?/, '')).join('\n');
}
