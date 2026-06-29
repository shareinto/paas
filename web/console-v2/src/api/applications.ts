import { actorBody, actorQuery, request, type PageResult } from './client';

export type ApplicationStatus = 'active' | 'disabled';

export type Application = {
  id: string;
  tenantId: string;
  projectId: string;
  name: string;
  displayName: string;
  description: string;
  status: ApplicationStatus;
  myRoleId?: string;
  createdAt?: string;
  updatedAt?: string;
};

type BackendApplication = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  project_id?: string;
  projectId?: string;
  name?: string;
  display_name?: string;
  displayName?: string;
  description?: string;
  status?: ApplicationStatus;
  my_role_id?: string;
  myRoleId?: string;
  created_at?: string;
  createdAt?: string;
  updated_at?: string;
  updatedAt?: string;
};

export type ApplicationListScope = 'joined' | 'accessible';

export type SaveApplicationInput = {
  projectId: string;
  name: string;
  displayName: string;
  description: string;
  disabled?: boolean;
};

export async function listApplications(scope: ApplicationListScope, projectId?: string) {
  const params = new URLSearchParams(actorQuery());
  params.set('scope', scope);
  params.set('page', '1');
  params.set('page_size', '200');
  if (projectId) params.set('project_id', projectId);
  const data = await request<PageResult<BackendApplication>>(`/api/me/applications?${params.toString()}`);
  return {
    items: data.items.map(toApplication),
    total: data.total || data.items.length
  };
}

export async function createApplication(input: SaveApplicationInput) {
  const app = await request<BackendApplication>('/api/applications', {
    method: 'POST',
    body: JSON.stringify({
      actor: actorBody(),
      project_id: input.projectId,
      name: input.name,
      display_name: input.displayName,
      description: input.description
    })
  });
  return toApplication(app);
}

export async function updateApplication(applicationId: string, input: SaveApplicationInput) {
  const app = await request<BackendApplication>(`/api/applications/${encodeURIComponent(applicationId)}`, {
    method: 'PATCH',
    body: JSON.stringify({
      actor: actorBody(),
      display_name: input.displayName,
      description: input.description,
      disabled: Boolean(input.disabled)
    })
  });
  return toApplication(app);
}

function toApplication(app: BackendApplication): Application {
  return {
    id: app.id,
    tenantId: app.tenant_id || app.tenantId || '',
    projectId: app.project_id || app.projectId || '',
    name: app.name || app.id,
    displayName: app.display_name || app.displayName || app.name || app.id,
    description: app.description || '',
    status: app.status || 'active',
    myRoleId: app.my_role_id || app.myRoleId || undefined,
    createdAt: app.created_at || app.createdAt,
    updatedAt: app.updated_at || app.updatedAt
  };
}
