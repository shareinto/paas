import { actorBody, request, type PageResult } from './client';

export type SubjectType = 'user' | 'group' | 'service_account';
export type ScopeKind = 'platform' | 'tenant' | 'project' | 'application' | 'stage';

export type Role = {
  id: string;
  name: string;
  description: string;
  builtIn: boolean;
  disabled: boolean;
  permissions: string[];
  suggestedScopes?: ScopeKind[];
};

export type User = {
  id: string;
  username: string;
  displayName: string;
  email: string;
  disabled: boolean;
};

export type RoleBinding = {
  id: string;
  subjectType: SubjectType;
  subjectId: string;
  roleId: string;
  scopeKind: ScopeKind;
  scopeId: string;
  createdAt?: string;
};

export type RBACBundle = {
  roles: Role[];
  permissions: string[];
  users: User[];
};

type BackendRole = {
  id: string;
  name?: string;
  description?: string;
  built_in?: boolean;
  builtIn?: boolean;
  disabled?: boolean;
  permissions?: string[];
  suggestedScopes?: ScopeKind[];
  suggested_scopes?: ScopeKind[];
};

type BackendUser = {
  id: string;
  username?: string;
  display_name?: string;
  displayName?: string;
  email?: string;
  disabled?: boolean;
};

type BackendRoleBinding = {
  id: string;
  subject_type?: SubjectType;
  subjectType?: SubjectType;
  subject_id?: string;
  subjectId?: string;
  role_id?: string;
  roleId?: string;
  scope_kind?: ScopeKind;
  scopeKind?: ScopeKind;
  scope_id?: string;
  scopeId?: string;
  created_at?: string;
  createdAt?: string;
};

export async function loadRBACBundle(): Promise<RBACBundle> {
  const [rolesData, permissionsData, usersData] = await Promise.all([
    request<{ items: BackendRole[] }>('/api/roles'),
    request<{ items: string[] }>('/api/permissions'),
    request<PageResult<BackendUser>>('/api/users?page=1&page_size=200')
  ]);
  return {
    roles: (rolesData.items || []).map(mapRole),
    permissions: permissionsData.items || [],
    users: (usersData.items || []).map(mapUser)
  };
}

export async function listRoleBindings(input: { scopeKind?: ScopeKind; scopeId?: string; subjectType?: SubjectType; subjectId?: string }) {
  const params = new URLSearchParams();
  if (input.scopeKind) params.set('scope_kind', input.scopeKind);
  if (input.scopeId) params.set('scope_id', input.scopeId);
  if (input.subjectType) params.set('subject_type', input.subjectType);
  if (input.subjectId) params.set('subject_id', input.subjectId);
  const data = await request<PageResult<BackendRoleBinding>>(`/api/role-bindings?${params.toString()}`);
  return {
    items: (data.items || []).map(mapRoleBinding),
    total: data.total || data.items.length
  };
}

export async function replaceRoleBinding(input: Omit<RoleBinding, 'id' | 'createdAt'>) {
  const binding = await request<BackendRoleBinding>('/api/role-bindings', {
    method: 'PUT',
    body: JSON.stringify(roleBindingBody(input))
  });
  return mapRoleBinding(binding);
}

export async function createRole(input: {
  id: string;
  name: string;
  description?: string;
  permissions: string[];
  suggestedScopes?: ScopeKind[];
}) {
  const role = await request<BackendRole>('/api/roles', {
    method: 'POST',
    body: JSON.stringify({
      actor: actorBody(),
      id: input.id,
      name: input.name,
      description: input.description || '',
      permissions: input.permissions,
      suggested_scopes: input.suggestedScopes || []
    })
  });
  return mapRole(role);
}

export async function updateRole(input: {
  id: string;
  name: string;
  description?: string;
  disabled?: boolean;
  suggestedScopes?: ScopeKind[];
}) {
  const role = await request<BackendRole>(`/api/roles/${encodeURIComponent(input.id)}`, {
    method: 'PUT',
    body: JSON.stringify({
      actor: actorBody(),
      name: input.name,
      description: input.description || '',
      disabled: input.disabled,
      suggested_scopes: input.suggestedScopes || []
    })
  });
  return mapRole(role);
}

export async function deleteRole(roleId: string) {
  await request(`/api/roles/${encodeURIComponent(roleId)}?actor_id=${encodeURIComponent(actorBody().id)}`, { method: 'DELETE' });
}

export async function deleteRoleBinding(input: Pick<RoleBinding, 'subjectType' | 'subjectId' | 'scopeKind' | 'scopeId'>) {
  const params = new URLSearchParams({
    subject_type: input.subjectType,
    subject_id: input.subjectId,
    scope_kind: input.scopeKind
  });
  if (input.scopeId) params.set('scope_id', input.scopeId);
  await request(`/api/role-bindings?${params.toString()}`, { method: 'DELETE' });
}

export async function updateRolePermissions(roleId: string, permissions: string[]) {
  const role = await request<BackendRole>(`/api/roles/${encodeURIComponent(roleId)}/permissions`, {
    method: 'PATCH',
    body: JSON.stringify({ actor: actorBody(), permissions })
  });
  return mapRole(role);
}

function roleBindingBody(input: Omit<RoleBinding, 'id' | 'createdAt'>) {
  return {
    subject_type: input.subjectType,
    subject_id: input.subjectId,
    role_id: input.roleId,
    scope_kind: input.scopeKind,
    scope_id: input.scopeKind === 'platform' ? '' : input.scopeId
  };
}

function mapRole(role: BackendRole): Role {
  return {
    id: role.id,
    name: role.name || role.id,
    description: role.description || '',
    builtIn: Boolean(role.built_in ?? role.builtIn),
    disabled: Boolean(role.disabled),
    permissions: role.permissions || [],
    suggestedScopes: role.suggestedScopes || role.suggested_scopes || []
  };
}

function mapUser(user: BackendUser): User {
  return {
    id: user.id,
    username: user.username || user.id,
    displayName: user.display_name || user.displayName || user.username || user.id,
    email: user.email || '',
    disabled: Boolean(user.disabled)
  };
}

function mapRoleBinding(binding: BackendRoleBinding): RoleBinding {
  return {
    id: binding.id,
    subjectType: binding.subject_type || binding.subjectType || 'user',
    subjectId: binding.subject_id || binding.subjectId || '',
    roleId: binding.role_id || binding.roleId || '',
    scopeKind: binding.scope_kind || binding.scopeKind || 'platform',
    scopeId: binding.scope_id || binding.scopeId || '',
    createdAt: binding.created_at || binding.createdAt
  };
}
