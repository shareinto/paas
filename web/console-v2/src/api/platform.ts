import { platformContexts } from '../data/mock';
import { hasAPIBaseURL, request, type PageResult } from './client';

export type PlatformWorkload = {
  id: string;
  name: string;
  kind: string;
  pipelineId?: string;
};

export type PlatformApplication = {
  id: string;
  name: string;
  code: string;
  owner: string;
  workloads: PlatformWorkload[];
};

export type PlatformProject = {
  id: string;
  name: string;
  applications: PlatformApplication[];
};

export type PlatformTenant = {
  id: string;
  name: string;
  projects: PlatformProject[];
};

export type PlatformContextSource = 'mock' | 'api';

export type PlatformContextResult = {
  source: PlatformContextSource;
  tenants: PlatformTenant[];
  error?: string;
};

type BackendTenant = {
  id: string;
  name?: string;
  display_name?: string;
  displayName?: string;
};

type BackendProject = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name?: string;
  display_name?: string;
  displayName?: string;
};

type BackendApplication = {
  id: string;
  name?: string;
  display_name?: string;
  displayName?: string;
  owner?: string;
};

type BackendWorkload = {
  id: string;
  name?: string;
  display_name?: string;
  displayName?: string;
  workload_type?: string;
  workloadType?: string;
  pipeline_id?: string;
  pipelineId?: string;
};

export async function loadPlatformContexts(): Promise<PlatformContextResult> {
  if (!hasAPIBaseURL()) {
    return { source: 'mock', tenants: platformContexts };
  }

  try {
    const tenants = await request<PageResult<BackendTenant>>('/api/tenants?page=1&page_size=100');
    const projects = await request<PageResult<BackendProject>>('/api/projects?page=1&page_size=200');
    const projectsWithApplications = await Promise.all(
      projects.items.map(async (project) => ({
        project,
        applications: await loadApplications(project.id)
      }))
    );

    return {
      source: 'api',
      tenants: tenants.items.map((tenant) => ({
        id: tenant.id,
        name: displayNameOf(tenant),
        projects: projectsWithApplications
          .filter(({ project }) => tenantIdOf(project) === tenant.id)
          .map(({ project, applications }) => ({
            id: project.id,
            name: displayNameOf(project),
            applications
          }))
      }))
    };
  } catch (error) {
    return {
      source: 'mock',
      tenants: platformContexts,
      error: error instanceof Error ? error.message : '后端上下文加载失败，已回退 mock 数据'
    };
  }
}

async function loadApplications(projectId: string): Promise<PlatformApplication[]> {
  const data = await request<PageResult<BackendApplication>>(`/api/projects/${encodeURIComponent(projectId)}/applications?page=1&page_size=100`);
  return Promise.all(
    data.items.map(async (application) => ({
      id: application.id,
      name: displayNameOf(application),
      code: application.name || application.id,
      owner: application.owner || '平台用户',
      workloads: await loadWorkloads(application.id)
    }))
  );
}

async function loadWorkloads(applicationId: string): Promise<PlatformWorkload[]> {
  const data = await request<{ items: BackendWorkload[] }>(`/api/applications/${encodeURIComponent(applicationId)}/workloads`);
  return data.items.map((workload) => ({
    id: workload.id,
    name: displayNameOf(workload),
    kind: workload.workload_type || workload.workloadType || 'Deployment',
    pipelineId: workload.pipeline_id || workload.pipelineId || undefined
  }));
}

function tenantIdOf(project: BackendProject) {
  return project.tenant_id || project.tenantId || '';
}

function displayNameOf(item: { id: string; name?: string; display_name?: string; displayName?: string }) {
  return item.display_name || item.displayName || item.name || item.id;
}
