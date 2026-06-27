import { versionSourcePipelines, type Status, type VersionSourcePipeline } from '../data/mock';
import { actorBody, hasAPIBaseURL, request, requestText, streamSSE, type PageResult } from './client';

export type BuildPipelineSourceResult = {
  source: 'api' | 'mock';
  pipelines: VersionSourcePipeline[];
  error?: string;
};

export type PipelineEnvironmentOption = {
  id: string;
  name: string;
};

export type PipelineFormOptionsResult = {
  source: 'api' | 'mock';
  runtimeOptions: PipelineEnvironmentOption[];
  buildEnvironmentOptions: PipelineEnvironmentOption[];
  error?: string;
};

export type SourceBranchOption = {
  name: string;
  default?: boolean;
};

export type BackendBuildPipeline = {
  id: string;
  application_id?: string;
  applicationId?: string;
  name?: string;
  display_name?: string;
  displayName?: string;
  description?: string;
  status?: string;
  runtime_environments?: BackendRuntimeEnvironment[];
  runtimeEnvironments?: BackendRuntimeEnvironment[];
};

export type BackendRuntimeEnvironment = {
  id?: string;
  name?: string;
  display_name?: string;
  displayName?: string;
  runtime_base_image?: string;
  runtimeBaseImage?: string;
  artifact_deploy_path?: string;
  artifactDeployPath?: string;
};

export type BackendBuildPipelineSource = {
  id: string;
  key?: string;
  display_name?: string;
  displayName?: string;
  source_type?: string;
  sourceType?: string;
  source_url?: string;
  sourceUrl?: string;
  source_ref?: string;
  sourceRef?: string;
  svn_revision?: string;
  svnRevision?: string;
  build_environment_id?: string;
  buildEnvironmentId?: string;
  source_path?: string;
  sourcePath?: string;
  build_spec?: BackendBuildSpec;
  buildSpec?: BackendBuildSpec;
  is_primary?: boolean;
  isPrimary?: boolean;
};

export type BackendBuildSpec = {
  source_path?: string;
  sourcePath?: string;
  build_command?: string;
  buildCommand?: string;
  artifact_copy_command?: string;
  artifactCopyCommand?: string;
  runtime_base_image?: string;
  runtimeBaseImage?: string;
  artifact_deploy_path?: string;
  artifactDeployPath?: string;
  default_ref?: string;
  defaultRef?: string;
};

export type BackendBuildRun = {
  id: string;
  pipeline_id?: string;
  pipelineId?: string;
  pipeline_name?: string;
  pipelineName?: string;
  pipeline_display_name?: string;
  pipelineDisplayName?: string;
  source_type?: string;
  sourceType?: string;
  source_url?: string;
  sourceUrl?: string;
  source_ref?: string;
  sourceRef?: string;
  commit_sha?: string;
  commitSha?: string;
  version?: string;
  status?: string;
  created_at?: string;
  createdAt?: string;
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
};

export type BackendBuildEnvironment = {
  id: string;
  name?: string;
  display_name?: string;
  displayName?: string;
};

export async function loadVersionSourcePipelines(applicationId: string): Promise<BuildPipelineSourceResult> {
  if (!hasAPIBaseURL()) {
    return { source: 'mock', pipelines: versionSourcePipelines };
  }

  const [pipelinePage, runPage] = await Promise.all([
    request<PageResult<BackendBuildPipeline>>(`/api/apps/${encodeURIComponent(applicationId)}/build-pipelines?page=1&page_size=100`),
    request<PageResult<BackendBuildRun>>(`/api/apps/${encodeURIComponent(applicationId)}/builds?page=1&page_size=100`)
  ]);
  const runs = runPage.items || [];
  const pipelines = await Promise.all((pipelinePage.items || []).map(async (pipeline) => {
    const sources = await listPipelineSources(pipeline.id);
    return mapPipeline(pipeline, sources, runs);
  }));
  return { source: 'api', pipelines };
}

export async function loadPipelineFormOptions(): Promise<PipelineFormOptionsResult> {
  if (!hasAPIBaseURL()) {
    return {
      source: 'mock',
      runtimeOptions: fallbackRuntimeOptions(),
      buildEnvironmentOptions: fallbackBuildEnvironmentOptions()
    };
  }

  const [runtimePage, buildPage] = await Promise.all([
    request<PageResult<BackendRuntimeEnvironment>>('/api/runtime-environments?page=1&page_size=100'),
    request<PageResult<BackendBuildEnvironment>>('/api/build-environments?page=1&page_size=100')
  ]);
  return {
    source: 'api',
    runtimeOptions: mapEnvironmentOptions(runtimePage.items || []),
    buildEnvironmentOptions: mapEnvironmentOptions(buildPage.items || [])
  };
}

export async function previewGitSourceBranches(projectId: string, sourceUrl: string): Promise<SourceBranchOption[]> {
  if (!hasAPIBaseURL() || !projectId || !sourceUrl.trim()) {
    return [{ name: 'main', default: true }];
  }
  const data = await request<{ items?: SourceBranchOption[] } | SourceBranchOption[]>(`/api/projects/${encodeURIComponent(projectId)}/build-source-branches/preview`, {
    method: 'POST',
    body: JSON.stringify({ actor: actorBody(), source_url: sourceUrl.trim() })
  });
  return Array.isArray(data) ? data : (data.items || []);
}

export function mapVersionSourcePipelinesFromWorkspace(
  pipelines: BackendBuildPipeline[],
  sourcesByPipeline: Record<string, BackendBuildPipelineSource[]>,
  runs: BackendBuildRun[]
): BuildPipelineSourceResult {
  return {
    source: 'api',
    pipelines: (pipelines || []).map((pipeline) => mapPipeline(pipeline, sourcesByPipeline[pipeline.id] || [], runs || []))
  };
}

export function mapPipelineFormOptionsFromWorkspace(
  runtimeEnvironments: BackendRuntimeEnvironment[],
  buildEnvironments: BackendBuildEnvironment[]
): PipelineFormOptionsResult {
  return {
    source: 'api',
    runtimeOptions: mapEnvironmentOptions(runtimeEnvironments || []),
    buildEnvironmentOptions: mapEnvironmentOptions(buildEnvironments || [])
  };
}

export async function createVersionSourcePipeline(applicationId: string, projectId: string, pipeline: VersionSourcePipeline) {
  const body = await pipelinePayload(projectId, pipeline, true);
  const item = await request<BackendBuildPipeline>(`/api/apps/${encodeURIComponent(applicationId)}/build-pipelines`, {
    method: 'POST',
    body: JSON.stringify(body)
  });
  const sources = await listPipelineSources(item.id);
  const runs = await listApplicationBuildRuns(applicationId);
  return mapPipeline(item, sources, runs);
}

export async function updateVersionSourcePipeline(projectId: string, pipeline: VersionSourcePipeline) {
  const body = await pipelinePayload(projectId, pipeline, false);
  const item = await request<BackendBuildPipeline>(`/api/build-pipelines/${encodeURIComponent(pipeline.id)}`, {
    method: 'PATCH',
    body: JSON.stringify(body)
  });
  const sources = await listPipelineSources(item.id);
  const runs = await listApplicationBuildRuns(applicationIdOf(item));
  return mapPipeline(item, sources, runs);
}

export async function deleteVersionSourcePipeline(pipelineId: string) {
  await request<void>(`/api/build-pipelines/${encodeURIComponent(pipelineId)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: actorBody() })
  });
}

export async function triggerVersionSourcePipelineBuild(applicationId: string, pipeline: VersionSourcePipeline, sourceRef: string, buildCommand: string, version: string) {
  await updateVersionSourcePipeline('', {
    ...pipeline,
    branch: sourceRef || pipeline.branch,
    buildCommand,
    sources: pipeline.sources.map((source, index) => index === 0 ? { ...source, branch: sourceRef || source.branch, sourceRef: sourceRef || source.sourceRef || source.branch, buildCommand } : source)
  });
  const run = await request<BackendBuildRun>(`/api/build-pipelines/${encodeURIComponent(pipeline.id)}/builds`, {
    method: 'POST',
    body: JSON.stringify({ actor: actorBody(), source_ref: sourceRef || pipeline.branch, version })
  });
  const runs = await listApplicationBuildRuns(applicationId);
  return { run: mapBuildHistory(run), runs: runs.map(mapBuildHistory) };
}

export async function loadBuildRunLog(buildRunId: string): Promise<string[]> {
  if (!hasAPIBaseURL()) {
    const fallback = versionSourcePipelines.flatMap((pipeline) => pipeline.logs);
    return fallback.length ? fallback : ['暂无构建日志'];
  }
  const text = await requestText(`/api/builds/${encodeURIComponent(buildRunId)}/logs/stream`);
  const parsed = parseSSELog(text);
  return parsed ? parsed.split(/\r?\n/) : ['暂无构建日志'];
}

export async function cancelVersionSourcePipelineBuild(buildRunId: string) {
  if (!hasAPIBaseURL()) {
    return {
      id: buildRunId,
      branch: 'main',
      status: 'danger' as const,
      version: buildRunId,
      startedAt: '刚刚',
      duration: '已取消'
    };
  }
  const run = await request<BackendBuildRun>(`/api/builds/${encodeURIComponent(buildRunId)}/cancel`, {
    method: 'POST',
    body: JSON.stringify({ actor: actorBody() })
  });
  return mapBuildHistory(run);
}

export function streamBuildRunLog(
  buildRunId: string,
  onLog: (text: string) => void,
  onStatus?: (status: string) => void,
  onError?: (error: Error) => void
) {
  if (!hasAPIBaseURL()) {
    let closed = false;
    const fallback = versionSourcePipelines.flatMap((pipeline) => pipeline.logs);
    window.setTimeout(() => {
      if (!closed) onLog(fallback.length ? fallback.join('\n') : '暂无构建日志');
    }, 120);
    return () => {
      closed = true;
    };
  }

  let close: () => void = () => undefined;
  close = streamSSE(`/api/builds/${encodeURIComponent(buildRunId)}/logs/stream`, (event) => {
    if (event.event === 'log' || event.event === 'message') {
      onLog(event.data);
      return;
    }
    if (event.event === 'status') {
      onStatus?.(event.data);
      if (['succeeded', 'failed', 'aborted', 'unstable'].includes(event.data)) close();
      return;
    }
    if (event.event === 'error') {
      onError?.(new Error(event.data || '构建日志读取失败'));
    }
  }, onError);
  return close;
}

async function listPipelineSources(pipelineId: string) {
  const data = await request<{ items: BackendBuildPipelineSource[] }>(`/api/build-pipelines/${encodeURIComponent(pipelineId)}/sources`);
  return data.items || [];
}

async function listApplicationBuildRuns(applicationId: string) {
  if (!applicationId) return [];
  const data = await request<PageResult<BackendBuildRun>>(`/api/apps/${encodeURIComponent(applicationId)}/builds?page=1&page_size=100`);
  return data.items || [];
}

async function pipelinePayload(projectId: string, pipeline: VersionSourcePipeline, includeName: boolean) {
  const runtimeId = await resolveRuntimeEnvironmentId(pipeline.runtime, pipeline.runtimeEnvironmentIds?.[0]);
  return {
    actor: actorBody(),
    ...(includeName ? { name: slugOf(pipeline.name || pipeline.id) } : {}),
    display_name: pipeline.name,
    description: pipeline.description || '',
    runtime_environment_ids: [runtimeId].filter(Boolean),
    sources: await Promise.all(pipeline.sources.map((source, index) => sourcePayload(projectId, source, index)))
  };
}

async function sourcePayload(_projectId: string, source: VersionSourcePipeline['sources'][number], index: number) {
  const buildEnvironmentId = await resolveBuildEnvironmentId(source.buildEnvironment, source.buildEnvironmentId);
  const sourceType = normalizeSourceType(source.sourceType);
  const sourceUrl = source.sourceUrl || source.repository || '';
  const defaultRef = source.sourceRef || source.branch || (sourceType === 'svn' ? 'HEAD' : 'main');
  const sourcePath = source.sourcePath || '.';
  return {
    key: source.key || (index === 0 ? 'main' : `source-${index + 1}`),
    display_name: source.name || source.key || `代码源 ${index + 1}`,
    source_type: sourceType,
    source_url: sourceUrl,
    source_ref: defaultRef,
    svn_revision: sourceType === 'svn' ? (source.svnRevision || '') : '',
    build_environment_id: buildEnvironmentId,
    source_path: sourcePath,
    default_ref: defaultRef,
    is_primary: index === 0,
    build_spec: {
      source_path: sourcePath,
      build_command: source.buildCommand,
      artifact_copy_command: source.artifactCopyCommand || 'cp -ar target "$PAAS_ARTIFACT_OUTPUT"',
      runtime_base_image: source.runtimeBaseImage || 'registry.local/runtime/default:latest',
      artifact_deploy_path: source.artifactDeployPath || '/app',
      default_ref: defaultRef
    }
  };
}

async function resolveRuntimeEnvironmentId(runtimeName: string, current?: string) {
  if (current) return current;
  const data = await request<PageResult<BackendRuntimeEnvironment>>('/api/runtime-environments?page=1&page_size=100');
  const items = data.items || [];
  return matchByName(items, runtimeName)?.id || items[0]?.id || '';
}

async function resolveBuildEnvironmentId(buildEnvironmentName: string, current?: string) {
  if (current) return current;
  const data = await request<PageResult<BackendBuildEnvironment>>('/api/build-environments?page=1&page_size=100');
  const items = data.items || [];
  return matchByName(items, buildEnvironmentName)?.id || items[0]?.id || '';
}

function mapPipeline(pipeline: BackendBuildPipeline, sources: BackendBuildPipelineSource[], runs: BackendBuildRun[]): VersionSourcePipeline {
  const pipelineRuns = runs
    .filter((run) => pipelineIdOf(run) === pipeline.id)
    .sort(compareBuildRunsDesc);
  const primarySource = sources.find((source) => source.is_primary || source.isPrimary) || sources[0];
  const runtimeEnvironments = pipeline.runtime_environments || pipeline.runtimeEnvironments || [];
  const runtime = runtimeEnvironments.map(displayNameOf).filter(Boolean).join(' / ') || '默认运行时';
  const mappedSources = sources.length ? sources.map(mapSource) : [fallbackSource()];
  const latestRun = pipelineRuns[0];
  const latestSuccessfulRun = pipelineRuns.find((run) => statusOfRun(run.status || '') === 'healthy');
  return {
    id: pipeline.id,
    name: pipeline.display_name || pipeline.displayName || pipeline.name || pipeline.id,
    description: pipeline.description || '',
    branch: defaultRefOf(primarySource) || 'main',
    runtime,
    runtimeEnvironmentIds: runtimeEnvironments.map((item) => item.id || '').filter(Boolean),
    sourcePath: sourcePathOf(primarySource),
    buildCommand: buildCommandOf(primarySource),
    artifactCopyCommand: artifactCopyCommandOf(primarySource),
    sources: mappedSources,
    buildHistory: pipelineRuns.map(mapBuildHistory),
    logs: latestRun ? [`[${formatDateTime(latestRun.created_at || latestRun.createdAt)}] 构建 ${buildStatusLabel(latestRun.status || '')}`, latestRun.commit_sha || latestRun.commitSha ? `Commit ${latestRun.commit_sha || latestRun.commitSha}` : '等待构建日志回写'] : [],
    latestVersion: latestSuccessfulRun ? versionOf(latestSuccessfulRun) : '暂无版本',
    status: latestRun ? statusOfRun(latestRun.status || '') : statusOfPipeline(pipeline.status || '')
  };
}

function mapSource(source: BackendBuildPipelineSource): VersionSourcePipeline['sources'][number] {
  const spec = source.build_spec || source.buildSpec || {};
  const sourceType = normalizeSourceType(source.source_type || source.sourceType);
  const sourceRef = source.source_ref || source.sourceRef || spec.default_ref || spec.defaultRef || (sourceType === 'svn' ? 'HEAD' : 'main');
  const sourceUrl = source.source_url || source.sourceUrl || '';
  return {
    id: source.id,
    key: source.key || 'main',
    name: source.display_name || source.displayName || source.key || '主代码源',
    sourceType,
    repository: sourceUrl,
    sourceUrl,
    sourceRef,
    svnRevision: source.svn_revision || source.svnRevision || '',
    branch: sourceRef,
    sourcePath: spec.source_path || spec.sourcePath || source.source_path || source.sourcePath || '.',
    buildEnvironment: source.build_environment_id || source.buildEnvironmentId || '默认构建环境',
    buildEnvironmentId: source.build_environment_id || source.buildEnvironmentId || '',
    buildCommand: spec.build_command || spec.buildCommand || '',
    artifactCopyCommand: spec.artifact_copy_command || spec.artifactCopyCommand || '',
    runtimeBaseImage: spec.runtime_base_image || spec.runtimeBaseImage || '',
    artifactDeployPath: spec.artifact_deploy_path || spec.artifactDeployPath || ''
  };
}

function fallbackSource(): VersionSourcePipeline['sources'][number] {
  return {
    id: 'source-main',
    key: 'main',
    name: '主代码源',
    sourceType: 'git',
    repository: '',
    sourceUrl: '',
    sourceRef: 'main',
    branch: 'main',
    sourcePath: '.',
    buildEnvironment: '',
    buildCommand: '',
    artifactCopyCommand: ''
  };
}

function mapBuildHistory(run: BackendBuildRun): VersionSourcePipeline['buildHistory'][number] {
  return {
    id: run.id,
    branch: run.source_ref || run.sourceRef || 'main',
    status: statusOfRun(run.status || ''),
    version: versionOf(run),
    startedAt: formatDateTime(run.started_at || run.startedAt || run.created_at || run.createdAt),
    duration: durationOf(run)
  };
}

function statusOfPipeline(status: string): Status {
  if (status === 'disabled') return 'danger';
  return 'healthy';
}

function statusOfRun(status: string): Status {
  if (status === 'succeeded') return 'healthy';
  if (status === 'failed' || status === 'aborted') return 'danger';
  if (status === 'unstable') return 'warning';
  if (status === 'running' || status === 'queued' || status === 'pending') return 'running';
  return 'pending';
}

function compareBuildRunsDesc(left: BackendBuildRun, right: BackendBuildRun) {
  return buildRunTimestamp(right) - buildRunTimestamp(left);
}

function buildRunTimestamp(run: BackendBuildRun) {
  const value = run.started_at || run.startedAt || run.created_at || run.createdAt || '';
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function versionOf(run?: BackendBuildRun) {
  if (!run) return '暂无版本';
  if (run.version) return run.version;
  const commit = run.commit_sha || run.commitSha || '';
  return commit ? commit.slice(0, 8) : run.id;
}

function buildStatusLabel(status: string) {
  const labels: Record<string, string> = {
    queued: '排队中',
    running: '构建中',
    succeeded: '成功',
    failed: '失败',
    aborted: '已取消',
    unstable: '不稳定'
  };
  return labels[status] || status || '未知';
}

function durationOf(run: BackendBuildRun) {
  const start = parseDate(run.started_at || run.startedAt || run.created_at || run.createdAt);
  const end = parseDate(run.finished_at || run.finishedAt);
  if (!start) return '-';
  if (!end) return ['running', 'queued'].includes(run.status || '') ? '进行中' : '-';
  const seconds = Math.max(0, Math.round((end.getTime() - start.getTime()) / 1000));
  if (seconds < 60) return `${seconds} 秒`;
  return `${Math.floor(seconds / 60)} 分 ${String(seconds % 60).padStart(2, '0')} 秒`;
}

function defaultRefOf(source?: BackendBuildPipelineSource) {
  if (!source) return '';
  const spec = source.build_spec || source.buildSpec || {};
  return source.source_ref || source.sourceRef || spec.default_ref || spec.defaultRef || '';
}

function normalizeSourceType(value?: string) {
  return value === 'svn' ? 'svn' : 'git';
}

function sourcePathOf(source?: BackendBuildPipelineSource) {
  if (!source) return '.';
  const spec = source.build_spec || source.buildSpec || {};
  return spec.source_path || spec.sourcePath || source.source_path || source.sourcePath || '.';
}

function buildCommandOf(source?: BackendBuildPipelineSource) {
  const spec = source?.build_spec || source?.buildSpec || {};
  return spec.build_command || spec.buildCommand || '';
}

function artifactCopyCommandOf(source?: BackendBuildPipelineSource) {
  const spec = source?.build_spec || source?.buildSpec || {};
  return spec.artifact_copy_command || spec.artifactCopyCommand || '';
}

function applicationIdOf(pipeline: BackendBuildPipeline) {
  return pipeline.application_id || pipeline.applicationId || '';
}

function pipelineIdOf(run: BackendBuildRun) {
  return run.pipeline_id || run.pipelineId || '';
}

function displayNameOf(item: { id?: string; name?: string; display_name?: string; displayName?: string }) {
  return item.display_name || item.displayName || item.name || item.id || '';
}

function mapEnvironmentOptions<T extends { id?: string; name?: string; display_name?: string; displayName?: string }>(items: T[]) {
  return items
    .map((item) => ({ id: item.id || displayNameOf(item), name: displayNameOf(item) }))
    .filter((item) => item.id && item.name);
}

function fallbackRuntimeOptions(): PipelineEnvironmentOption[] {
  return ['Java 17 / Maven', 'Java 17 / Gradle', 'Node 22 / pnpm', 'Go 1.23', 'Python 3.12']
    .map((name) => ({ id: name, name }));
}

function fallbackBuildEnvironmentOptions(): PipelineEnvironmentOption[] {
  return ['Maven JDK17 构建环境', 'Gradle JDK17 构建环境', 'Node 22 构建环境', 'Go 构建环境', 'Python 构建环境']
    .map((name) => ({ id: name, name }));
}

function matchByName<T extends { id?: string; name?: string; display_name?: string; displayName?: string }>(items: T[], name: string) {
  const needle = name.trim().toLowerCase();
  return items.find((item) => [item.id, item.name, item.display_name, item.displayName].some((value) => value && value.toLowerCase() === needle))
    || items.find((item) => [item.id, item.name, item.display_name, item.displayName].some((value) => value && value.toLowerCase().includes(needle)));
}

function slugOf(value: string) {
  const slug = value.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '');
  return slug || `pipeline-${Date.now().toString(36)}`;
}

function parseDate(value?: string) {
  if (!value) return null;
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function formatDateTime(value?: string) {
  const parsed = parseDate(value);
  if (!parsed) return '-';
  const pad = (input: number) => String(input).padStart(2, '0');
  return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())} ${pad(parsed.getHours())}:${pad(parsed.getMinutes())}:${pad(parsed.getSeconds())}`;
}

function parseSSELog(text: string) {
  if (!text.includes('data:')) return text;
  return text
    .split(/\r?\n/)
    .filter((line) => line.startsWith('data:'))
    .map((line) => line.replace(/^data:\s?/, ''))
    .join('\n');
}
