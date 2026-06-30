import { actorBody, request, type PageResult } from './client';

export type BuildEnvironmentStatus = 'enabled' | 'disabled' | 'deleted';
export type RuntimeEnvironmentStatus = 'enabled' | 'disabled' | 'deleted';

export type BuildEnvironment = {
  id: string;
  name: string;
  description: string;
  buildImage: string;
  status: BuildEnvironmentStatus;
  isDefault: boolean;
  updatedAt?: string;
};

export type RuntimeImage = {
  id?: string;
  name: string;
  displayName: string;
  architectures: string[];
  runtimeBaseImage: string;
  artifactDeployPath: string;
  dockerfilePath: string;
  selectorLabels: Record<string, string>;
  status?: string;
};

export type RuntimeEnvironment = {
  id: string;
  name: string;
  description: string;
  runtimeBaseImage: string;
  artifactDeployPath: string;
  dockerfilePath: string;
  selectorLabels: Record<string, string>;
  images: RuntimeImage[];
  status: RuntimeEnvironmentStatus;
  updatedAt?: string;
};

export type BuildTemplate = {
  id: string;
  name: string;
  version: number;
  content: string;
  updatedAt?: string;
};

export type BuildManagementBundle = {
  buildEnvironments: BuildEnvironment[];
  runtimeEnvironments: RuntimeEnvironment[];
  buildTemplate: BuildTemplate;
};

type BackendBuildEnvironment = {
  id: string;
  name?: string;
  description?: string;
  build_image?: string;
  buildImage?: string;
  status?: BuildEnvironmentStatus;
  is_default?: boolean;
  isDefault?: boolean;
  updated_at?: string;
  updatedAt?: string;
};

type BackendRuntimeImage = {
  id?: string;
  name?: string;
  display_name?: string;
  displayName?: string;
  architectures?: string[];
  runtime_base_image?: string;
  runtimeBaseImage?: string;
  artifact_deploy_path?: string;
  artifactDeployPath?: string;
  dockerfile_path?: string;
  dockerfilePath?: string;
  selector_labels?: Record<string, string>;
  selectorLabels?: Record<string, string>;
  status?: string;
};

type BackendRuntimeEnvironment = {
  id: string;
  name?: string;
  description?: string;
  runtime_base_image?: string;
  runtimeBaseImage?: string;
  artifact_deploy_path?: string;
  artifactDeployPath?: string;
  dockerfile_path?: string;
  dockerfilePath?: string;
  selector_labels?: Record<string, string>;
  selectorLabels?: Record<string, string>;
  images?: BackendRuntimeImage[];
  status?: RuntimeEnvironmentStatus;
  updated_at?: string;
  updatedAt?: string;
};

type BackendBuildTemplate = {
  id: string;
  name?: string;
  version?: number;
  content?: string;
  updated_at?: string;
  updatedAt?: string;
};

export async function loadBuildManagement(): Promise<BuildManagementBundle> {
  const [buildPage, runtimePage, template] = await Promise.all([
    request<PageResult<BackendBuildEnvironment>>('/api/admin/build-environments?page=1&page_size=100'),
    request<PageResult<BackendRuntimeEnvironment>>('/api/admin/runtime-environments?page=1&page_size=100'),
    request<BackendBuildTemplate>('/api/admin/build-template')
  ]);
  return {
    buildEnvironments: (buildPage.items || []).map(mapBuildEnvironment),
    runtimeEnvironments: (runtimePage.items || []).map(mapRuntimeEnvironment),
    buildTemplate: mapBuildTemplate(template)
  };
}

export async function createBuildEnvironment(input: Pick<BuildEnvironment, 'name' | 'description' | 'buildImage' | 'isDefault'>) {
  await request('/api/admin/build-environments', {
    method: 'POST',
    body: JSON.stringify({ actor: actorBody(), name: input.name, description: input.description, build_image: input.buildImage, is_default: input.isDefault })
  });
}

export async function updateBuildEnvironment(input: BuildEnvironment) {
  await request(`/api/admin/build-environments/${encodeURIComponent(input.id)}`, {
    method: 'PATCH',
    body: JSON.stringify({
      actor: actorBody(),
      description: input.description,
      build_image: input.buildImage,
      status: input.status,
      is_default: input.isDefault
    })
  });
}

export async function deleteBuildEnvironment(id: string) {
  await request(`/api/admin/build-environments/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: actorBody() })
  });
}

export async function createRuntimeEnvironment(input: Omit<RuntimeEnvironment, 'id' | 'status' | 'updatedAt'>) {
  await request('/api/admin/runtime-environments', {
    method: 'POST',
    body: JSON.stringify(runtimeEnvironmentBody(input))
  });
}

export async function updateRuntimeEnvironment(input: RuntimeEnvironment) {
  await request(`/api/admin/runtime-environments/${encodeURIComponent(input.id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ ...runtimeEnvironmentBody(input), status: input.status })
  });
}

export async function deleteRuntimeEnvironment(id: string) {
  await request(`/api/admin/runtime-environments/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    body: JSON.stringify({ actor: actorBody() })
  });
}

export async function saveBuildTemplate(content: string) {
  const template = await request<BackendBuildTemplate>('/api/admin/build-template', {
    method: 'PUT',
    body: JSON.stringify({ actor: actorBody(), content })
  });
  return mapBuildTemplate(template);
}

function runtimeEnvironmentBody(input: Omit<RuntimeEnvironment, 'id' | 'status' | 'updatedAt'> | RuntimeEnvironment) {
  return {
    actor: actorBody(),
    name: input.name,
    description: input.description,
    runtime_base_image: input.runtimeBaseImage,
    artifact_deploy_path: input.artifactDeployPath,
    dockerfile_path: input.dockerfilePath,
    selector_labels: input.selectorLabels,
    images: input.images.map((image) => ({
      id: image.id,
      name: image.name,
      display_name: image.displayName,
      architectures: image.architectures,
      runtime_base_image: image.runtimeBaseImage,
      artifact_deploy_path: image.artifactDeployPath,
      dockerfile_path: image.dockerfilePath,
      selector_labels: image.selectorLabels,
      status: image.status || 'enabled'
    }))
  };
}

function mapBuildEnvironment(item: BackendBuildEnvironment): BuildEnvironment {
  return {
    id: item.id,
    name: item.name || item.id,
    description: item.description || '',
    buildImage: item.build_image || item.buildImage || '',
    status: item.status || 'enabled',
    isDefault: Boolean(item.is_default ?? item.isDefault),
    updatedAt: formatTime(item.updated_at || item.updatedAt)
  };
}

function mapRuntimeEnvironment(item: BackendRuntimeEnvironment): RuntimeEnvironment {
  const images = (item.images || []).map(mapRuntimeImage);
  const firstImage = images[0];
  return {
    id: item.id,
    name: item.name || item.id,
    description: item.description || '',
    runtimeBaseImage: item.runtime_base_image || item.runtimeBaseImage || firstImage?.runtimeBaseImage || '',
    artifactDeployPath: item.artifact_deploy_path || item.artifactDeployPath || firstImage?.artifactDeployPath || '',
    dockerfilePath: item.dockerfile_path || item.dockerfilePath || firstImage?.dockerfilePath || '',
    selectorLabels: item.selector_labels || item.selectorLabels || firstImage?.selectorLabels || {},
    images,
    status: item.status || 'enabled',
    updatedAt: formatTime(item.updated_at || item.updatedAt)
  };
}

function mapRuntimeImage(item: BackendRuntimeImage): RuntimeImage {
  return {
    id: item.id,
    name: item.name || '',
    displayName: item.display_name || item.displayName || item.name || '',
    architectures: normalizeArchitectures(item.architectures),
    runtimeBaseImage: item.runtime_base_image || item.runtimeBaseImage || '',
    artifactDeployPath: item.artifact_deploy_path || item.artifactDeployPath || '',
    dockerfilePath: item.dockerfile_path || item.dockerfilePath || '',
    selectorLabels: item.selector_labels || item.selectorLabels || {},
    status: item.status || 'enabled'
  };
}

function normalizeArchitectures(values: string[] | undefined) {
  const normalized = (values || []).map((item) => item === 'amd64' || item === 'linux/amd64' ? 'x86' : item === 'arm64' || item === 'linux/arm64' ? 'arm' : item).filter((item) => item === 'x86' || item === 'arm');
  return normalized.length ? Array.from(new Set(normalized)) : ['x86', 'arm'];
}

function mapBuildTemplate(item: BackendBuildTemplate): BuildTemplate {
  return {
    id: item.id,
    name: item.name || item.id,
    version: item.version || 0,
    content: item.content || '',
    updatedAt: formatTime(item.updated_at || item.updatedAt)
  };
}

function formatTime(value: string | undefined) {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}
