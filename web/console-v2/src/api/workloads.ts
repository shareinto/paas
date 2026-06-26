import { versionSourceConfig, type VersionSourcePipeline, type VersionSourceWorkloadConfig } from '../data/mock';
import { APIError, actorBody, hasAPIBaseURL, request } from './client';

export type WorkloadSourceResult = {
  source: 'api' | 'mock';
  config: {
    updatedAt: string;
    freightSerial: number;
    workloads: VersionSourceWorkloadConfig[];
  };
  error?: string;
};

export type BackendWorkload = {
  id: string;
  name?: string;
  display_name?: string;
  displayName?: string;
  workload_type?: string;
  workloadType?: string;
  description?: string;
  status?: string;
  image_source_mode?: string;
  imageSourceMode?: string;
  pipeline_id?: string;
  pipelineId?: string;
};

export type BackendWorkloadStageConfig = {
  replicas?: number;
  service_ports?: Array<{ name?: string; port?: number; target_port?: number; targetPort?: number; protocol?: string }>;
  servicePorts?: Array<{ name?: string; port?: number; target_port?: number; targetPort?: number; protocol?: string }>;
  resource_requests?: { cpu?: string; memory?: string };
  resourceRequests?: { cpu?: string; memory?: string };
  resource_limits?: { cpu?: string; memory?: string };
  resourceLimits?: { cpu?: string; memory?: string };
  probes?: Array<{
    name?: string;
    type?: string;
    path?: string;
    port?: number;
    initial_delay_seconds?: number;
    initialDelaySeconds?: number;
    period_seconds?: number;
    periodSeconds?: number;
    timeout_seconds?: number;
    timeoutSeconds?: number;
  }>;
  ingress_hosts?: Array<{
    server_name?: string;
    serverName?: string;
    host?: string;
    path?: string;
    service_port?: string;
    servicePort?: string;
    tls?: boolean;
    rewrite?: boolean;
    rewrite_path?: string;
    rewritePath?: string;
    tls_redirect?: boolean;
    tlsRedirect?: boolean;
  }>;
  ingressHosts?: BackendWorkloadStageConfig['ingress_hosts'];
  env_vars?: Array<{ name?: string; value?: string }>;
  envVars?: Array<{ name?: string; value?: string }>;
  secret_refs?: Array<{ name?: string; secret_ref?: string; secretRef?: string }>;
  secretRefs?: Array<{ name?: string; secret_ref?: string; secretRef?: string }>;
  config_files?: Array<{ mount_path?: string; mountPath?: string; content?: string; base64_encoded?: boolean; base64Encoded?: boolean }>;
  configFiles?: Array<{ mount_path?: string; mountPath?: string; content?: string; base64_encoded?: boolean; base64Encoded?: boolean }>;
  writable_dirs?: Array<{ mount_path?: string; mountPath?: string; owner_group?: string; ownerGroup?: string; mode?: string; size_limit?: string; sizeLimit?: string }>;
  writableDirs?: Array<{ mount_path?: string; mountPath?: string; owner_group?: string; ownerGroup?: string; mode?: string; size_limit?: string; sizeLimit?: string }>;
  values_override?: {
    containers?: BackendContainerOverride[];
    serviceType?: string;
    terminationGracePeriodSeconds?: number;
    [key: string]: unknown;
  };
  valuesOverride?: {
    containers?: BackendContainerOverride[];
    serviceType?: string;
    terminationGracePeriodSeconds?: number;
    [key: string]: unknown;
  };
};

type BackendContainerOverride = {
  id?: string;
  name?: string;
  image_source?: unknown;
  imageSource?: unknown;
  port?: number;
  cpu?: string;
  memory?: string;
  limit_cpu?: string;
  limitCpu?: string;
  limit_memory?: string;
  limitMemory?: string;
  command?: string[] | string;
  args?: string[] | string;
  liveness_probe?: unknown;
  livenessProbe?: unknown;
  readiness_probe?: unknown;
  readinessProbe?: unknown;
  startup_probe?: unknown;
  startupProbe?: unknown;
  env_vars?: Array<{ name?: string; value?: string }>;
  envVars?: Array<{ name?: string; value?: string }>;
  secret_refs?: Array<{ name?: string; secret_ref?: string; secretRef?: string }>;
  secretRefs?: Array<{ name?: string; secret_ref?: string; secretRef?: string }>;
  config_files?: Array<{ mount_path?: string; mountPath?: string; content?: string; base64_encoded?: boolean; base64Encoded?: boolean }>;
  configFiles?: Array<{ mount_path?: string; mountPath?: string; content?: string; base64_encoded?: boolean; base64Encoded?: boolean }>;
  writable_dirs?: Array<{ mount_path?: string; mountPath?: string; owner_group?: string; ownerGroup?: string; mode?: string; size_limit?: string; sizeLimit?: string }>;
  writableDirs?: Array<{ mount_path?: string; mountPath?: string; owner_group?: string; ownerGroup?: string; mode?: string; size_limit?: string; sizeLimit?: string }>;
  nas_mount?: { enabled?: boolean; nas_path?: string; nasPath?: string; mount_path?: string; mountPath?: string };
  nasMount?: { enabled?: boolean; nas_path?: string; nasPath?: string; mount_path?: string; mountPath?: string };
};

export async function loadVersionSourceWorkloads(applicationId: string, pipelines: VersionSourcePipeline[]): Promise<WorkloadSourceResult> {
  if (!hasAPIBaseURL()) {
    return { source: 'mock', config: versionSourceConfig };
  }

  const data = await request<{ items: BackendWorkload[] }>(`/api/applications/${encodeURIComponent(applicationId)}/workloads?enabled=true`);
  const workloads = await Promise.all((data.items || []).map(async (workload) => {
    const defaultConfig = await loadOptionalWorkloadDefaultConfig(applicationId, workload.id);
    return mapWorkloadConfig(workload, defaultConfig, pipelines);
  }));
  return {
    source: 'api',
    config: {
      updatedAt: new Date().toLocaleString('zh-CN', { hour12: false }),
      freightSerial: versionSourceConfig.freightSerial,
      workloads
    }
  };
}

async function loadOptionalWorkloadDefaultConfig(applicationId: string, workloadId: string): Promise<BackendWorkloadStageConfig | undefined> {
  try {
    return await request<BackendWorkloadStageConfig>(
      `/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}/default-config`
    );
  } catch (error) {
    if (error instanceof APIError && error.status === 404) {
      return undefined;
    }
    throw error;
  }
}

export function mapVersionSourceWorkloadsFromWorkspace(
  workloads: BackendWorkload[],
  defaultConfigsByWorkload: Record<string, BackendWorkloadStageConfig>,
  pipelines: VersionSourcePipeline[]
): WorkloadSourceResult {
  return {
    source: 'api',
    config: {
      updatedAt: new Date().toLocaleString('zh-CN', { hour12: false }),
      freightSerial: versionSourceConfig.freightSerial,
      workloads: (workloads || []).map((workload) => mapWorkloadConfig(workload, defaultConfigsByWorkload?.[workload.id], pipelines))
    }
  };
}

export async function saveVersionSourceWorkloads(applicationId: string, config: WorkloadSourceResult['config'], previousWorkloadIds?: string[]) {
  await Promise.all(config.workloads.map(async (workload) => {
    const primaryContainer = workload.containers[0];
    const saved = await upsertWorkload(applicationId, workload, primaryContainer);

    await request<BackendWorkloadStageConfig>(
      `/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(saved.id)}/default-config`,
      {
        method: 'PUT',
        body: JSON.stringify(workloadDefaultConfigPayload(workload))
      }
    );
  }));

  // Delete workloads that were removed from the config
  if (previousWorkloadIds) {
    const currentIds = new Set(config.workloads.map((w) => w.id));
    const removedIds = previousWorkloadIds.filter((id) => !currentIds.has(id));
    await Promise.all(removedIds.map((id) =>
      request(`/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        body: JSON.stringify({ actor: actorBody() })
      }).catch(() => {})
    ));
  }
}

async function upsertWorkload(
  applicationId: string,
  workload: VersionSourceWorkloadConfig,
  primaryContainer: VersionSourceWorkloadConfig['containers'][number] | undefined
) {
  const payload = workloadPayload(workload, primaryContainer);
  try {
    return await request<BackendWorkload>(`/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workload.id)}`, {
      method: 'PUT',
      body: JSON.stringify(payload)
    });
  } catch (error) {
    if (!(error instanceof APIError) || error.status !== 404) {
      throw error;
    }
    return request<BackendWorkload>(`/api/applications/${encodeURIComponent(applicationId)}/workloads`, {
      method: 'POST',
      body: JSON.stringify(payload)
    });
  }
}

function workloadPayload(
  workload: VersionSourceWorkloadConfig,
  primaryContainer: VersionSourceWorkloadConfig['containers'][number] | undefined
) {
  return {
    actor: actorBody(),
    name: workload.name,
    display_name: workload.name,
    workload_type: workload.kind,
    description: '',
    image_source_mode: primaryContainer?.imageSource.mode === 'custom' ? 'custom_image' : 'pipeline_artifact',
    pipeline_id: primaryContainer?.imageSource.mode === 'pipeline' ? primaryContainer.imageSource.pipelineId || '' : ''
  };
}

function mapWorkloadConfig(
  workload: BackendWorkload,
  defaultConfig: BackendWorkloadStageConfig | undefined,
  pipelines: VersionSourcePipeline[]
): VersionSourceWorkloadConfig {
  const pipelineId = workload.pipeline_id || workload.pipelineId || '';
  const imageSourceMode = workload.image_source_mode || workload.imageSourceMode || '';
  const sourcePort = (defaultConfig?.service_ports || defaultConfig?.servicePorts || [])[0];
  const probes = defaultConfig?.probes || [];
  const livenessProbe = probes.find((item) => (item.name || '').toLowerCase() === 'liveness') || probes[0];
  const readinessProbe = probes.find((item) => (item.name || '').toLowerCase() === 'readiness') || livenessProbe;
  const startupProbe = probes.find((item) => (item.name || '').toLowerCase() === 'startup');
  const requests = defaultConfig?.resource_requests || defaultConfig?.resourceRequests || {};
  const limits = defaultConfig?.resource_limits || defaultConfig?.resourceLimits || {};
  const pipeline = pipelines.find((item) => item.id === pipelineId);
  const id = workload.id;
  const displayName = workload.display_name || workload.displayName || workload.name || id;
  const valuesOverride = normalizeValuesOverride(defaultConfig);
  const ingress = (defaultConfig?.ingress_hosts || defaultConfig?.ingressHosts || [])[0];
  const legacyContainerConfig = {
    envVars: normalizeEnvVars(defaultConfig?.env_vars || defaultConfig?.envVars),
    secretRefs: normalizeSecretRefs(defaultConfig?.secret_refs || defaultConfig?.secretRefs),
    configFiles: normalizeConfigFiles(defaultConfig?.config_files || defaultConfig?.configFiles),
    writableDirs: normalizeWritableDirs(defaultConfig?.writable_dirs || defaultConfig?.writableDirs)
  };
  const fallbackProbes = {
    livenessProbe: normalizeProbeConfig(livenessProbe, {
      path: livenessProbe?.path || readinessProbe?.path || '/healthz',
      port: Number(livenessProbe?.port || readinessProbe?.port || sourcePort?.target_port || sourcePort?.targetPort || 8080),
      initialDelaySeconds: Number(livenessProbe?.initial_delay_seconds ?? livenessProbe?.initialDelaySeconds ?? 20),
      periodSeconds: Number(livenessProbe?.period_seconds ?? livenessProbe?.periodSeconds ?? readinessProbe?.period_seconds ?? readinessProbe?.periodSeconds ?? 10),
      timeoutSeconds: Number(livenessProbe?.timeout_seconds ?? livenessProbe?.timeoutSeconds ?? readinessProbe?.timeout_seconds ?? readinessProbe?.timeoutSeconds ?? 1),
      failureThreshold: 3,
      successThreshold: 1
    }, true),
    readinessProbe: normalizeProbeConfig(readinessProbe, {
      path: readinessProbe?.path || livenessProbe?.path || '/healthz',
      port: Number(readinessProbe?.port || livenessProbe?.port || sourcePort?.target_port || sourcePort?.targetPort || 8080),
      initialDelaySeconds: Number(readinessProbe?.initial_delay_seconds ?? readinessProbe?.initialDelaySeconds ?? 10),
      periodSeconds: Number(readinessProbe?.period_seconds ?? readinessProbe?.periodSeconds ?? livenessProbe?.period_seconds ?? livenessProbe?.periodSeconds ?? 10),
      timeoutSeconds: Number(readinessProbe?.timeout_seconds ?? readinessProbe?.timeoutSeconds ?? livenessProbe?.timeout_seconds ?? livenessProbe?.timeoutSeconds ?? 1),
      failureThreshold: 5,
      successThreshold: 1
    }, true),
    startupProbe: normalizeProbeConfig(startupProbe, {
      enabled: false,
      path: startupProbe?.path || livenessProbe?.path || readinessProbe?.path || '/healthz',
      port: Number(startupProbe?.port || livenessProbe?.port || readinessProbe?.port || sourcePort?.target_port || sourcePort?.targetPort || 8080),
      initialDelaySeconds: Number(startupProbe?.initial_delay_seconds ?? startupProbe?.initialDelaySeconds ?? 0),
      periodSeconds: Number(startupProbe?.period_seconds ?? startupProbe?.periodSeconds ?? 10),
      timeoutSeconds: Number(startupProbe?.timeout_seconds ?? startupProbe?.timeoutSeconds ?? 1),
      failureThreshold: 30,
      successThreshold: 1
    }, !!startupProbe)
  };
  const overrideContainers = normalizeOverrideContainers(id, defaultConfig, fallbackProbes, legacyContainerConfig);
  return {
    id,
    name: displayName,
    kind: normalizeWorkloadKind(workload.workload_type || workload.workloadType),
    replicas: Number(defaultConfig?.replicas ?? 1),
    serviceType: normalizeServiceType(valuesOverride.serviceType),
    servicePort: Number(sourcePort?.port || sourcePort?.target_port || sourcePort?.targetPort || 8080),
    enableDomainAccess: !!ingress?.host,
    serverName: ingress?.server_name || ingress?.serverName || displayName,
    domain: ingress?.host || '',
    ingressPath: ingress?.path || '/',
    ingressRewrite: !!ingress?.rewrite,
    ingressRewritePath: ingress?.rewrite_path || ingress?.rewritePath || '/',
    ingressTls: !!ingress?.tls,
    ingressTlsRedirect: !!(ingress?.tls_redirect || ingress?.tlsRedirect),
    probePath: fallbackProbes.livenessProbe.path,
    probePort: fallbackProbes.livenessProbe.port,
    livenessInitialDelaySeconds: fallbackProbes.livenessProbe.initialDelaySeconds,
    readinessInitialDelaySeconds: fallbackProbes.readinessProbe.initialDelaySeconds,
    probePeriodSeconds: fallbackProbes.livenessProbe.periodSeconds,
    probeTimeoutSeconds: fallbackProbes.livenessProbe.timeoutSeconds,
    terminationGracePeriodSeconds: Number(valuesOverride.terminationGracePeriodSeconds ?? 30),
    nodeType: (valuesOverride.nodeType as VersionSourceWorkloadConfig['nodeType']) || 'general',
    exclusive: !!valuesOverride.exclusive,
    envVars: [],
    secretRefs: [],
    configFiles: [],
    writableDirs: [],
    containers: overrideContainers.length > 0 ? overrideContainers : [
      {
        id: `${id}-app`,
        name: 'app',
        imageSource: imageSourceMode === 'custom_image' || !pipelineId
          ? { mode: 'custom', customImage: `registry.local/${workload.name || id}:latest` }
          : { mode: 'pipeline', pipelineId },
        port: Number(sourcePort?.target_port || sourcePort?.targetPort || sourcePort?.port || 8080),
        cpu: requests.cpu || '250m',
        memory: requests.memory || '256Mi',
        limitCpu: limits.cpu || '',
        limitMemory: limits.memory || '',
        ...fallbackProbes,
        ...legacyContainerConfig
      }
    ],
    ...(pipeline ? {} : {})
  };
}

function normalizeOverrideContainers(
  workloadId: string,
  defaultConfig: BackendWorkloadStageConfig | undefined,
  fallbackProbes?: Pick<VersionSourceWorkloadConfig['containers'][number], 'livenessProbe' | 'readinessProbe' | 'startupProbe'>,
  legacyContainerConfig?: Pick<VersionSourceWorkloadConfig['containers'][number], 'envVars' | 'secretRefs' | 'configFiles' | 'writableDirs'>
): VersionSourceWorkloadConfig['containers'] {
  const containers = defaultConfig?.values_override?.containers || defaultConfig?.valuesOverride?.containers || [];
  return containers.map((container, index) => {
    const port = Number(container.port || 8080);
    const name = container.name || (index === 0 ? 'app' : `container-${index + 1}`);
    return {
      id: container.id || `${workloadId}-${name}-${index + 1}`,
      name,
      imageSource: normalizeContainerImageSource(container.image_source || container.imageSource),
      port,
      cpu: container.cpu || '250m',
      memory: container.memory || '256Mi',
      limitCpu: container.limit_cpu || container.limitCpu || '',
      limitMemory: container.limit_memory || container.limitMemory || '',
      command: normalizeCommand(container.command),
      livenessProbe: normalizeProbeConfig(container.liveness_probe || container.livenessProbe, fallbackProbes?.livenessProbe, true),
      readinessProbe: normalizeProbeConfig(container.readiness_probe || container.readinessProbe, fallbackProbes?.readinessProbe, true),
      startupProbe: normalizeProbeConfig(container.startup_probe || container.startupProbe, fallbackProbes?.startupProbe, !!fallbackProbes?.startupProbe?.enabled),
      envVars: normalizeEnvVars(container.env_vars || container.envVars || (index === 0 ? legacyContainerConfig?.envVars : undefined)),
      secretRefs: normalizeSecretRefs(container.secret_refs || container.secretRefs || (index === 0 ? legacyContainerConfig?.secretRefs : undefined)),
      configFiles: normalizeConfigFiles(container.config_files || container.configFiles || (index === 0 ? legacyContainerConfig?.configFiles : undefined)),
      writableDirs: normalizeWritableDirs(container.writable_dirs || container.writableDirs || (index === 0 ? legacyContainerConfig?.writableDirs : undefined)),
      nasMount: normalizeNasMount(container.nas_mount || container.nasMount)
    };
  });
}

function normalizeProbeConfig(
  raw: unknown,
  fallback?: VersionSourceWorkloadConfig['containers'][number]['livenessProbe'],
  defaultEnabled = true
): NonNullable<VersionSourceWorkloadConfig['containers'][number]['livenessProbe']> {
  const source = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {};
  const enabledRaw = source.enabled;
  const enabled = enabledRaw === undefined ? (fallback?.enabled ?? defaultEnabled) : !!enabledRaw;
  return {
    enabled,
    probeType: normalizeProbeType(source.type || source.probe_type || source.probeType || fallback?.probeType),
    path: String(source.path || fallback?.path || '/healthz'),
    port: Number(source.port || fallback?.port || 8080),
    initialDelaySeconds: Number(source.initial_delay_seconds ?? source.initialDelaySeconds ?? fallback?.initialDelaySeconds ?? 10),
    periodSeconds: Number(source.period_seconds ?? source.periodSeconds ?? fallback?.periodSeconds ?? 10),
    timeoutSeconds: Number(source.timeout_seconds ?? source.timeoutSeconds ?? fallback?.timeoutSeconds ?? 1),
    failureThreshold: Number(source.failure_threshold ?? source.failureThreshold ?? fallback?.failureThreshold ?? 3),
    successThreshold: Number(source.success_threshold ?? source.successThreshold ?? fallback?.successThreshold ?? 1)
  };
}

function normalizeProbeType(value: unknown): 'http' | 'tcp' {
  const normalized = String(value || '').toLowerCase();
  return normalized === 'tcp' ? 'tcp' : 'http';
}

function normalizeCommand(value: unknown): string {
  if (Array.isArray(value)) return value.map((item) => String(item)).filter(Boolean).join(' ');
  if (typeof value === 'string') return value;
  return '';
}

function normalizeContainerImageSource(value: unknown): VersionSourceWorkloadConfig['containers'][number]['imageSource'] {
  if (!value || typeof value !== 'object') {
    return { mode: 'custom', customImage: 'registry.local/app:latest' };
  }
  const source = value as Record<string, unknown>;
  const mode = String(source.mode || '');
  if (mode === 'pipeline') {
    const pipelineId = String(source.pipelineId || source.pipeline_id || '');
    return pipelineId
      ? { mode: 'pipeline', pipelineId }
      : { mode: 'custom', customImage: 'registry.local/app:latest' };
  }
  return {
    mode: 'custom',
    customImage: String(source.customImage || source.custom_image || 'registry.local/app:latest')
  };
}

function workloadDefaultConfigPayload(workload: VersionSourceWorkloadConfig) {
  const primaryContainer = workload.containers[0];
  const valuesOverride = {} as Record<string, any>;
  const exposesService = workload.serviceType !== 'None';
  if (workload.serviceType && workload.serviceType !== 'ClusterIP') valuesOverride.serviceType = workload.serviceType;
  else delete valuesOverride.serviceType;
  if (workload.terminationGracePeriodSeconds !== undefined) valuesOverride.terminationGracePeriodSeconds = Number(workload.terminationGracePeriodSeconds);
  if (workload.nodeType && workload.nodeType !== 'general') valuesOverride.nodeType = workload.nodeType;
  else delete valuesOverride.nodeType;
  if (workload.exclusive) valuesOverride.exclusive = true;
  else delete valuesOverride.exclusive;
  delete valuesOverride.nginxSidecar;
  return {
    actor: actorBody(),
    replicas: workload.replicas,
    service_ports: exposesService ? [
      {
        name: 'http',
        port: workload.servicePort,
        target_port: primaryContainer?.port || workload.servicePort,
        protocol: 'TCP'
      }
    ] : [],
    resource_requests: {
      cpu: primaryContainer?.cpu || '',
      memory: primaryContainer?.memory || ''
    },
    resource_limits: {
      cpu: primaryContainer?.limitCpu || '',
      memory: primaryContainer?.limitMemory || ''
    },
    probes: [],
    ingress_hosts: exposesService ? cleanIngressHosts(workload) : [],
    env_vars: [],
    secret_refs: [],
    config_files: [],
    writable_dirs: [],
    volume_mounts: [],
    init_containers: [],
    values_override: {
      ...valuesOverride,
      containers: workload.containers.map((container) => ({
        name: container.name,
        image_source: container.imageSource,
        port: container.port,
        cpu: container.cpu,
        memory: container.memory,
        limit_cpu: container.limitCpu || '',
        limit_memory: container.limitMemory || '',
        command: cleanCommand(container.command),
        liveness_probe: probePayload(container.livenessProbe, {
          path: '/healthz',
          port: container.port,
          initialDelaySeconds: 20,
          periodSeconds: 10,
          timeoutSeconds: 1,
          failureThreshold: 3,
          successThreshold: 1
        }),
        readiness_probe: probePayload(container.readinessProbe, {
          path: '/healthz',
          port: container.port,
          initialDelaySeconds: 10,
          periodSeconds: 10,
          timeoutSeconds: 1,
          failureThreshold: 5,
          successThreshold: 1
        }),
        startup_probe: probePayload(container.startupProbe, {
          enabled: false,
          path: '/healthz',
          port: container.port,
          initialDelaySeconds: 0,
          periodSeconds: 10,
          timeoutSeconds: 1,
          failureThreshold: 30,
          successThreshold: 1
        }),
        env_vars: cleanEnvVars(container.envVars),
        secret_refs: cleanSecretRefs(container.secretRefs),
        config_files: cleanConfigFiles(container.configFiles),
        writable_dirs: cleanWritableDirs(container.writableDirs),
        nas_mount: cleanNasMount(container.nasMount)
      }))
    }
  };
}

function probePayload(
  probe: VersionSourceWorkloadConfig['containers'][number]['livenessProbe'],
  fallback: NonNullable<VersionSourceWorkloadConfig['containers'][number]['livenessProbe']>
) {
  const merged = { ...fallback, ...probe };
  const probeType = normalizeProbeType(merged.probeType);
  return {
    enabled: !!merged.enabled,
    type: probeType,
    ...(probeType === 'http' ? { path: merged.path || fallback.path } : {}),
    port: Number(merged.port || fallback.port || 8080),
    initial_delay_seconds: Number(merged.initialDelaySeconds ?? fallback.initialDelaySeconds ?? 0),
    period_seconds: Number(merged.periodSeconds ?? fallback.periodSeconds ?? 10),
    timeout_seconds: Number(merged.timeoutSeconds ?? fallback.timeoutSeconds ?? 1),
    failure_threshold: Number(merged.failureThreshold ?? fallback.failureThreshold ?? 3),
    success_threshold: Number(merged.successThreshold ?? fallback.successThreshold ?? 1)
  };
}

function cleanCommand(command?: string) {
  const trimmed = (command || '').trim();
  return trimmed ? trimmed.split(/\s+/) : [];
}

function normalizeValuesOverride(defaultConfig: BackendWorkloadStageConfig | undefined): Record<string, any> {
  const raw = defaultConfig?.values_override || defaultConfig?.valuesOverride || {};
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {};
  return { ...(raw as Record<string, any>) };
}

function normalizeServiceType(value: unknown): VersionSourceWorkloadConfig['serviceType'] {
  if (value === 'None') return 'None';
  if (value === 'NodePort' || value === 'LoadBalancer') return value;
  return 'ClusterIP';
}

function normalizeEnvVars(items?: Array<{ name?: string; value?: string }>) {
  return (items || []).map((item, index) => ({ id: `env-${index + 1}`, name: item.name || '', value: item.value || '' }));
}

function normalizeSecretRefs(items?: Array<{ name?: string; secret_ref?: string; secretRef?: string }>) {
  return (items || []).map((item, index) => ({ id: `secret-${index + 1}`, name: item.name || '', secretRef: item.secret_ref || item.secretRef || '' }));
}

function normalizeConfigFiles(items?: Array<{ mount_path?: string; mountPath?: string; content?: string; base64_encoded?: boolean; base64Encoded?: boolean }>) {
  return (items || []).map((item, index) => ({
    id: `config-${index + 1}`,
    mountPath: item.mount_path || item.mountPath || '',
    content: item.content || '',
    base64Encoded: !!(item.base64_encoded || item.base64Encoded)
  }));
}

function normalizeWritableDirs(items?: Array<{ mount_path?: string; mountPath?: string; owner_group?: string; ownerGroup?: string; mode?: string; size_limit?: string; sizeLimit?: string }>) {
  return (items || []).map((item, index) => ({
    id: `dir-${index + 1}`,
    mountPath: item.mount_path || item.mountPath || '',
    ownerGroup: item.owner_group || item.ownerGroup || '',
    mode: item.mode || '',
    sizeLimit: item.size_limit || item.sizeLimit || ''
  }));
}

function normalizeNasMount(item?: { enabled?: boolean; nas_path?: string; nasPath?: string; mount_path?: string; mountPath?: string }) {
  return {
    enabled: !!item?.enabled,
    nasPath: item?.nas_path || item?.nasPath || '',
    mountPath: item?.mount_path || item?.mountPath || ''
  };
}


function cleanIngressHosts(workload: VersionSourceWorkloadConfig) {
  if (workload.serviceType === 'None') return [];
  if (!workload.enableDomainAccess) return [];
  if (!workload.domain?.trim()) return [];
  return [{
    server_name: (workload.serverName || workload.name).trim(),
    host: workload.domain.trim(),
    path: workload.ingressPath || '/',
    service_port: 'http',
    tls: !!workload.ingressTls,
    path_type: 'Prefix',
    rewrite: !!workload.ingressRewrite,
    rewrite_path: workload.ingressRewrite ? (workload.ingressRewritePath || '/') : '',
    tls_redirect: !!(workload.ingressTls && workload.ingressTlsRedirect)
  }];
}

function cleanEnvVars(items?: VersionSourceWorkloadConfig['envVars']) {
  return (items || []).map((item) => ({ name: item.name.trim(), value: item.value })).filter((item) => item.name);
}

function cleanSecretRefs(items?: VersionSourceWorkloadConfig['secretRefs']) {
  return (items || []).map((item) => ({ name: item.name.trim(), secret_ref: item.secretRef.trim() })).filter((item) => item.name && item.secret_ref);
}

function cleanConfigFiles(items?: VersionSourceWorkloadConfig['configFiles']) {
  return (items || []).map((item) => ({
    mount_path: item.mountPath.trim(),
    content: item.content,
    base64_encoded: !!item.base64Encoded
  })).filter((item) => item.mount_path);
}

function cleanWritableDirs(items?: VersionSourceWorkloadConfig['writableDirs']) {
  return (items || []).map((item) => ({
    mount_path: item.mountPath.trim(),
    owner_group: item.ownerGroup || '',
    mode: item.mode || '',
    size_limit: item.sizeLimit || ''
  })).filter((item) => item.mount_path);
}

function cleanNasMount(item?: VersionSourceWorkloadConfig['containers'][number]['nasMount']) {
  if (!item?.enabled) return undefined;
  const nasPath = item.nasPath?.trim() || '';
  const mountPath = item.mountPath?.trim() || '';
  if (!nasPath || !mountPath) return undefined;
  return {
    enabled: true,
    nas_path: nasPath,
    mount_path: mountPath
  };
}

function normalizeWorkloadKind(value?: string): VersionSourceWorkloadConfig['kind'] {
  const normalized = String(value || '').toLowerCase();
  if (normalized === 'statefulset') return 'StatefulSet';
  return 'Deployment';
}

// --- Stage config overlay API ---

export async function loadWorkloadStageConfig(
  applicationId: string,
  workloadId: string,
  stageKey: string
): Promise<BackendWorkloadStageConfig | null> {
  if (!hasAPIBaseURL()) return null;
  try {
    return await request<BackendWorkloadStageConfig>(
      `/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}/stage-configs/${encodeURIComponent(stageKey)}`
    );
  } catch (error) {
    if (error instanceof APIError && (error.status === 404 || error.status === 204)) return null;
    throw error;
  }
}

export async function saveWorkloadStageConfig(
  applicationId: string,
  workloadId: string,
  stageKey: string,
  config: BackendWorkloadStageConfig
): Promise<void> {
  await request(
    `/api/applications/${encodeURIComponent(applicationId)}/workloads/${encodeURIComponent(workloadId)}/stage-configs/${encodeURIComponent(stageKey)}`,
    { method: 'PUT', body: JSON.stringify({ actor: actorBody(), ...config }) }
  );
}
