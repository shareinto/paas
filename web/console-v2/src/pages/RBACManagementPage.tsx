import { useEffect, useMemo, useState } from 'react';
import { Check, KeyRound, Plus, RefreshCcw, Save, ShieldCheck, Trash2, UserCog, Users } from 'lucide-react';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import {
  createRole,
  deleteRoleBinding,
  deleteRole,
  listRoleBindings,
  loadRBACBundle,
  replaceRoleBinding,
  updateRole,
  updateRolePermissions,
  type RBACBundle,
  type Role,
  type RoleBinding,
  type ScopeKind,
  type SubjectType,
  type User
} from '../api/rbac';
import { cn } from '../lib/utils';

type BindingMode = 'scope' | 'subject';
type BindingDraft = Omit<RoleBinding, 'id' | 'createdAt'>;
type RoleDraft = {
  mode: 'create' | 'edit';
  id: string;
  name: string;
  description: string;
  disabled: boolean;
  suggestedScopes: ScopeKind[];
  permissions: string[];
  builtIn?: boolean;
};

const emptyBindingDraft: BindingDraft = {
  subjectType: 'user',
  subjectId: '',
  roleId: 'viewer',
  scopeKind: 'project',
  scopeId: ''
};

const scopeOptions: { value: ScopeKind; label: string; hint: string }[] = [
  { value: 'platform', label: '平台', hint: '全局平台权限' },
  { value: 'tenant', label: '租户', hint: '租户级资源' },
  { value: 'project', label: '项目', hint: '项目内应用与流水线' },
  { value: 'application', label: '应用', hint: '单个应用权限' },
  { value: 'stage', label: 'Stage', hint: '单个发布阶段' }
];

const subjectOptions: { value: SubjectType; label: string }[] = [
  { value: 'user', label: '用户' },
  { value: 'group', label: '用户组' },
  { value: 'service_account', label: '服务账号' }
];

export function RBACManagementPage() {
  const [bundle, setBundle] = useState<RBACBundle | null>(null);
  const [bindings, setBindings] = useState<RoleBinding[]>([]);
  const [loading, setLoading] = useState(false);
  const [bindingLoading, setBindingLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [activeTab, setActiveTab] = useState('bindings');
  const [bindingMode, setBindingMode] = useState<BindingMode>('scope');
  const [scopeKind, setScopeKind] = useState<ScopeKind>('platform');
  const [scopeId, setScopeId] = useState('');
  const [subjectType, setSubjectType] = useState<SubjectType>('user');
  const [subjectId, setSubjectId] = useState('');
  const [bindingDraft, setBindingDraft] = useState<BindingDraft | null>(null);
  const [roleDraft, setRoleDraft] = useState<RoleDraft | null>(null);
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [permissionDraft, setPermissionDraft] = useState<string[]>([]);

  useEffect(() => {
    void reloadAll();
  }, []);

  async function reloadAll() {
    setLoading(true);
    setMessage('');
    try {
      const next = await loadRBACBundle();
      setBundle(next);
      await reloadBindings();
    } catch (error) {
      setMessage(`权限数据加载失败：${errorMessage(error)}`);
    } finally {
      setLoading(false);
    }
  }

  async function reloadBindings() {
    setBindingLoading(true);
    setMessage('');
    try {
      const data = bindingMode === 'scope'
        ? await listRoleBindings({ scopeKind, scopeId: scopeKind === 'platform' ? '' : scopeId })
        : await listRoleBindings({ subjectType, subjectId });
      setBindings(data.items);
    } catch (error) {
      setBindings([]);
      setMessage(`授权关系加载失败：${errorMessage(error)}`);
    } finally {
      setBindingLoading(false);
    }
  }

  const stats = useMemo(() => {
    const users = bundle?.users.length || 0;
    const roles = bundle?.roles.length || 0;
    const permissions = bundle?.permissions.length || 0;
    return { users, roles, permissions, bindings: bindings.length };
  }, [bundle, bindings]);

  const roles = bundle?.roles || [];
  const permissions = bundle?.permissions || [];
  const users = bundle?.users || [];

  function openCreateBinding(seed?: Partial<BindingDraft>) {
    setBindingDraft({ ...emptyBindingDraft, ...seed, roleId: seed?.roleId || roles[0]?.id || 'viewer' });
  }

  function openEditBinding(binding: RoleBinding) {
    setBindingDraft({
      subjectType: binding.subjectType,
      subjectId: binding.subjectId,
      roleId: binding.roleId,
      scopeKind: binding.scopeKind,
      scopeId: binding.scopeId
    });
  }

  async function saveBindingDraft() {
    if (!bindingDraft) return;
    if (!bindingDraft.subjectId.trim() || (bindingDraft.scopeKind !== 'platform' && !bindingDraft.scopeId.trim())) {
      setMessage('主体 ID 和作用域 ID 不能为空');
      return;
    }
    try {
      await replaceRoleBinding(bindingDraft);
      setBindingDraft(null);
      await reloadBindings();
      setMessage('授权关系已保存');
    } catch (error) {
      setMessage(`授权关系保存失败：${errorMessage(error)}`);
    }
  }

  async function removeBinding(binding: RoleBinding) {
    if (!window.confirm(`确认移除 ${subjectLabel(binding.subjectType)}「${binding.subjectId}」在该作用域上的授权？`)) return;
    try {
      await deleteRoleBinding(binding);
      await reloadBindings();
      setMessage('授权关系已移除');
    } catch (error) {
      setMessage(`授权关系移除失败：${errorMessage(error)}`);
    }
  }

  function openRoleEditor(role: Role) {
    setEditingRole(role);
    setPermissionDraft([...role.permissions]);
  }

  function openCreateRole() {
    setRoleDraft({ mode: 'create', id: '', name: '', description: '', disabled: false, suggestedScopes: ['application'], permissions: ['application:read'] });
  }

  function openRoleInfoEditor(role: Role) {
    setRoleDraft({
      mode: 'edit',
      id: role.id,
      name: role.name,
      description: role.description || '',
      disabled: role.disabled,
      suggestedScopes: role.suggestedScopes || [],
      permissions: role.permissions,
      builtIn: role.builtIn
    });
  }

  async function saveRolePermissions() {
    if (!editingRole) return;
    try {
      const role = await updateRolePermissions(editingRole.id, permissionDraft);
      setBundle((current) => current ? { ...current, roles: current.roles.map((item) => item.id === role.id ? role : item) } : current);
      setEditingRole(null);
      setMessage('角色权限已保存');
    } catch (error) {
      setMessage(`角色权限保存失败：${errorMessage(error)}`);
    }
  }

  async function saveRoleDraft() {
    if (!roleDraft) return;
    if (!roleDraft.name.trim() || (roleDraft.mode === 'create' && !roleDraft.id.trim())) {
      setMessage('角色 ID 和角色名称不能为空');
      return;
    }
    try {
      const role = roleDraft.mode === 'create'
        ? await createRole({
          id: roleDraft.id,
          name: roleDraft.name,
          description: roleDraft.description,
          permissions: roleDraft.permissions,
          suggestedScopes: roleDraft.suggestedScopes
        })
        : await updateRole({
          id: roleDraft.id,
          name: roleDraft.name,
          description: roleDraft.description,
          disabled: roleDraft.disabled,
          suggestedScopes: roleDraft.suggestedScopes
        });
      setBundle((current) => {
        if (!current) return current;
        const exists = current.roles.some((item) => item.id === role.id);
        return { ...current, roles: exists ? current.roles.map((item) => item.id === role.id ? role : item) : [...current.roles, role] };
      });
      setRoleDraft(null);
      setMessage(roleDraft.mode === 'create' ? '角色已创建' : '角色信息已保存');
    } catch (error) {
      setMessage(`角色保存失败：${errorMessage(error)}`);
    }
  }

  async function toggleRoleDisabled(role: Role) {
    try {
      const next = await updateRole({
        id: role.id,
        name: role.name,
        description: role.description,
        disabled: !role.disabled,
        suggestedScopes: role.suggestedScopes || []
      });
      setBundle((current) => current ? { ...current, roles: current.roles.map((item) => item.id === next.id ? next : item) } : current);
      setMessage(next.disabled ? '角色已停用' : '角色已启用');
    } catch (error) {
      setMessage(`角色状态更新失败：${errorMessage(error)}`);
    }
  }

  async function removeRole(role: Role) {
    if (!window.confirm(`确认删除自定义角色「${role.name}」？已有授权引用时后端会拒绝删除。`)) return;
    try {
      await deleteRole(role.id);
      setBundle((current) => current ? { ...current, roles: current.roles.filter((item) => item.id !== role.id) } : current);
      setMessage('角色已删除');
    } catch (error) {
      setMessage(`角色删除失败：${errorMessage(error)}`);
    }
  }

  function viewUserBindings(user: User) {
    setSubjectType('user');
    setSubjectId(user.id);
    setBindingMode('subject');
    setActiveTab('bindings');
    void reloadBindingsForSubject(user.id);
  }

  async function reloadBindingsForSubject(nextSubjectId: string) {
    setBindingLoading(true);
    setMessage('');
    try {
      const data = await listRoleBindings({ subjectType: 'user', subjectId: nextSubjectId });
      setBindings(data.items);
    } catch (error) {
      setBindings([]);
      setMessage(`用户授权加载失败：${errorMessage(error)}`);
    } finally {
      setBindingLoading(false);
    }
  }

  return (
    <div className="flex min-h-[calc(100vh-96px)] flex-col gap-4">
      <section className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="dense-label">平台级权限</div>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">权限管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">集中管理角色、权限点和 RBAC 授权关系；应用成员页后续可复用 application scope 的过滤视图。</p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline">真实后端</Badge>
          <Button variant="outline" onClick={reloadAll} disabled={loading}>
            <RefreshCcw className={cn('h-4 w-4', loading && 'animate-spin')} />
            刷新
          </Button>
        </div>
      </section>

      {message && <div className="rounded-md border bg-muted/35 px-3 py-2 text-sm text-muted-foreground">{message}</div>}

      <section className="grid gap-3 lg:grid-cols-4">
        <Metric label="授权关系" value={`${stats.bindings} 条`} icon={ShieldCheck} />
        <Metric label="角色" value={`${stats.roles} 个`} icon={KeyRound} />
        <Metric label="权限点" value={`${stats.permissions} 个`} icon={Check} />
        <Metric label="用户" value={`${stats.users} 个`} icon={Users} />
      </section>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="min-h-0 flex-1">
        <TabsList>
          <TabsTrigger value="bindings">授权关系</TabsTrigger>
          <TabsTrigger value="roles">角色权限</TabsTrigger>
          <TabsTrigger value="subjects">用户与主体</TabsTrigger>
          <TabsTrigger value="audit">审计记录</TabsTrigger>
        </TabsList>

        <TabsContent value="bindings">
          <Card>
            <CardHeader className="gap-3">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <CardTitle>授权关系</CardTitle>
                  <CardDescription>一个主体在同一作用域下只保留一个角色；变更角色会替换原授权。</CardDescription>
                </div>
                <Button onClick={() => openCreateBinding(defaultBindingSeed())}>
                  <Plus className="h-4 w-4" />
                  新增授权
                </Button>
              </div>
              <div className="grid gap-3 rounded-md border bg-muted/25 p-3 lg:grid-cols-[160px_180px_1fr_auto]">
                <select value={bindingMode} onChange={(event) => setBindingMode(event.target.value as BindingMode)} className="h-10 rounded-md border bg-card px-3 text-sm">
                  <option value="scope">按作用域查看</option>
                  <option value="subject">按主体查看</option>
                </select>
                {bindingMode === 'scope' ? (
                  <>
                    <select value={scopeKind} onChange={(event) => setScopeKind(event.target.value as ScopeKind)} className="h-10 rounded-md border bg-card px-3 text-sm">
                      {scopeOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
                    </select>
                    <Input value={scopeId} disabled={scopeKind === 'platform'} onChange={(event) => setScopeId(event.target.value)} placeholder={scopeKind === 'platform' ? '平台作用域无需 ID' : '输入租户 / 项目 / 应用 / Stage ID'} />
                  </>
                ) : (
                  <>
                    <select value={subjectType} onChange={(event) => setSubjectType(event.target.value as SubjectType)} className="h-10 rounded-md border bg-card px-3 text-sm">
                      {subjectOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
                    </select>
                    <Input value={subjectId} onChange={(event) => setSubjectId(event.target.value)} placeholder="输入用户 / 用户组 / 服务账号 ID" />
                  </>
                )}
                <Button variant="outline" onClick={reloadBindings} disabled={bindingLoading}>
                  <RefreshCcw className={cn('h-4 w-4', bindingLoading && 'animate-spin')} />
                  查询
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto rounded-md border">
                <table className="w-full min-w-[980px] border-collapse text-sm">
                  <thead className="bg-muted/45 text-xs text-muted-foreground">
                    <tr>
                      <th className="px-3 py-2 text-left font-medium">主体</th>
                      <th className="px-3 py-2 text-left font-medium">角色</th>
                      <th className="px-3 py-2 text-left font-medium">作用域</th>
                      <th className="px-3 py-2 text-left font-medium">权限摘要</th>
                      <th className="px-3 py-2 text-left font-medium">授权时间</th>
                      <th className="px-3 py-2 text-right font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {bindings.map((binding) => {
                      const role = roleById(roles, binding.roleId);
                      return (
                        <tr key={binding.id} className="border-t">
                          <td className="px-3 py-3">
                            <div className="font-medium">{subjectName(binding, users)}</div>
                            <div className="mt-0.5 text-xs text-muted-foreground">{subjectLabel(binding.subjectType)} · {binding.subjectId}</div>
                          </td>
                          <td className="px-3 py-3">
                            <Badge variant="outline">{role?.name || binding.roleId}</Badge>
                          </td>
                          <td className="px-3 py-3">
                            <div>{scopeLabel(binding.scopeKind)}</div>
                            <div className="mt-0.5 font-mono text-xs text-muted-foreground">{binding.scopeKind === 'platform' ? 'platform' : binding.scopeId}</div>
                          </td>
                          <td className="px-3 py-3 text-muted-foreground">{permissionSummary(role?.permissions || [])}</td>
                          <td className="px-3 py-3 font-mono text-xs text-muted-foreground">{formatTime(binding.createdAt)}</td>
                          <td className="px-3 py-3 text-right">
                            <div className="flex justify-end gap-2">
                              <Button size="sm" variant="outline" onClick={() => openEditBinding(binding)}>变更角色</Button>
                              <Button size="sm" variant="outline" onClick={() => removeBinding(binding)}>
                                <Trash2 className="h-3.5 w-3.5" />
                                移除
                              </Button>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                    {bindings.length === 0 && (
                      <tr>
                        <td colSpan={6} className="px-3 py-10 text-center text-sm text-muted-foreground">
                          暂无授权关系。选择作用域或主体后查询，或新建一条授权。
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="roles">
          <Card>
            <CardHeader className="gap-3">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <CardTitle>角色权限</CardTitle>
                  <CardDescription>角色可动态创建，权限点由系统提供；内置角色受保护，不能停用或删除。</CardDescription>
                </div>
                <Button onClick={openCreateRole}>
                  <Plus className="h-4 w-4" />
                  新建角色
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid gap-3 xl:grid-cols-2">
                {roles.map((role) => (
                  <div key={role.id} className={cn('rounded-md border bg-card p-4', role.disabled && 'bg-muted/25 opacity-75')}>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2 font-semibold">
                          <span>{role.name}</span>
                          {role.builtIn && <Badge variant="outline">内置</Badge>}
                          {role.disabled && <Badge variant="secondary">已停用</Badge>}
                        </div>
                        <div className="mt-1 font-mono text-xs text-muted-foreground">{role.id}</div>
                        {role.description && <div className="mt-2 text-sm text-muted-foreground">{role.description}</div>}
                      </div>
                      <div className="flex flex-wrap justify-end gap-2">
                        <Button size="sm" variant="outline" onClick={() => openRoleInfoEditor(role)}>编辑信息</Button>
                        <Button size="sm" variant="outline" onClick={() => openRoleEditor(role)}>编辑权限</Button>
                      </div>
                    </div>
                    <div className="mt-3 flex flex-wrap gap-1.5">
                      {role.permissions.slice(0, 8).map((permission) => <Badge key={permission} variant="secondary">{permission}</Badge>)}
                      {role.permissions.length > 8 && <Badge variant="outline">+{role.permissions.length - 8}</Badge>}
                    </div>
                    <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                      <span>适用作用域：{role.suggestedScopes?.length ? role.suggestedScopes.map(scopeLabel).join(' / ') : '按权限点判定'}</span>
                      <span className="flex gap-2">
                        {!role.builtIn && (
                          <>
                            <button type="button" className="text-foreground hover:underline" onClick={() => toggleRoleDisabled(role)}>
                              {role.disabled ? '启用' : '停用'}
                            </button>
                            <button type="button" className="text-destructive hover:underline" onClick={() => removeRole(role)}>
                              删除
                            </button>
                          </>
                        )}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="subjects">
          <Card>
            <CardHeader>
              <CardTitle>用户与主体</CardTitle>
              <CardDescription>第一版先管理用户主体；用户组和服务账号可以通过授权弹窗手动输入主体 ID。</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto rounded-md border">
                <table className="w-full min-w-[820px] border-collapse text-sm">
                  <thead className="bg-muted/45 text-xs text-muted-foreground">
                    <tr>
                      <th className="px-3 py-2 text-left font-medium">用户</th>
                      <th className="px-3 py-2 text-left font-medium">邮箱</th>
                      <th className="px-3 py-2 text-left font-medium">状态</th>
                      <th className="px-3 py-2 text-right font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {users.map((user) => (
                      <tr key={user.id} className="border-t">
                        <td className="px-3 py-3">
                          <div className="font-medium">{user.displayName}</div>
                          <div className="mt-0.5 font-mono text-xs text-muted-foreground">{user.id} · {user.username}</div>
                        </td>
                        <td className="px-3 py-3 text-muted-foreground">{user.email || '未填写'}</td>
                        <td className="px-3 py-3">
                          <Badge variant={user.disabled ? 'secondary' : 'outline'}>{user.disabled ? '已禁用' : '启用中'}</Badge>
                        </td>
                        <td className="px-3 py-3 text-right">
                          <Button size="sm" variant="outline" onClick={() => viewUserBindings(user)}>查看授权</Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="audit">
          <Card>
            <CardHeader>
              <CardTitle>RBAC 审计</CardTitle>
              <CardDescription>后续接入审计模块后，这里过滤展示授权创建、角色权限变更、授权移除和用户状态变更。</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="rounded-md border border-dashed bg-muted/25 px-4 py-10 text-center text-sm text-muted-foreground">
                审计接口待接入。当前授权关系和角色权限变更会直接通过真实后端接口提交。
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {bindingDraft && (
        <BindingDialog
          draft={bindingDraft}
          roles={roles}
          users={users}
          onChange={setBindingDraft}
          onCancel={() => setBindingDraft(null)}
          onSave={saveBindingDraft}
        />
      )}

      {roleDraft && (
        <RoleMetadataDialog
          draft={roleDraft}
          permissions={permissions}
          onChange={setRoleDraft}
          onCancel={() => setRoleDraft(null)}
          onSave={saveRoleDraft}
        />
      )}

      {editingRole && (
        <RolePermissionsDialog
          role={editingRole}
          permissions={permissions}
          values={permissionDraft}
          onChange={setPermissionDraft}
          onCancel={() => setEditingRole(null)}
          onSave={saveRolePermissions}
        />
      )}
    </div>
  );

  function defaultBindingSeed(): Partial<BindingDraft> {
    if (bindingMode === 'scope') {
      return { scopeKind, scopeId: scopeKind === 'platform' ? '' : scopeId };
    }
    return { subjectType, subjectId };
  }
}

function BindingDialog({ draft, roles, users, onChange, onCancel, onSave }: {
  draft: BindingDraft;
  roles: Role[];
  users: User[];
  onChange: (draft: BindingDraft) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/35 p-4">
      <div className="mt-10 w-full max-w-3xl rounded-md border bg-card shadow-xl">
        <div className="border-b px-5 py-4">
          <h2 className="text-lg font-semibold">授权关系</h2>
          <p className="mt-1 text-sm text-muted-foreground">保存后会替换同一主体在同一作用域下的原角色。</p>
        </div>
        <div className="grid gap-4 p-5 md:grid-cols-2">
          <label className="grid gap-1.5 text-sm">
            <span className="text-muted-foreground">主体类型</span>
            <select value={draft.subjectType} onChange={(event) => onChange({ ...draft, subjectType: event.target.value as SubjectType, subjectId: '' })} className="h-10 rounded-md border bg-card px-3 text-sm">
              {subjectOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
            </select>
          </label>
          <label className="grid gap-1.5 text-sm">
            <span className="text-muted-foreground">主体</span>
            {draft.subjectType === 'user' ? (
              <select value={draft.subjectId} onChange={(event) => onChange({ ...draft, subjectId: event.target.value })} className="h-10 rounded-md border bg-card px-3 text-sm">
                <option value="">选择用户</option>
                {users.map((user) => <option key={user.id} value={user.id}>{user.displayName} · {user.id}</option>)}
              </select>
            ) : (
              <Input value={draft.subjectId} onChange={(event) => onChange({ ...draft, subjectId: event.target.value })} placeholder="输入主体 ID" />
            )}
          </label>
          <label className="grid gap-1.5 text-sm">
            <span className="text-muted-foreground">作用域类型</span>
            <select value={draft.scopeKind} onChange={(event) => onChange({ ...draft, scopeKind: event.target.value as ScopeKind, scopeId: '' })} className="h-10 rounded-md border bg-card px-3 text-sm">
              {scopeOptions.map((item) => <option key={item.value} value={item.value}>{item.label} · {item.hint}</option>)}
            </select>
          </label>
          <label className="grid gap-1.5 text-sm">
            <span className="text-muted-foreground">作用域 ID</span>
            <Input value={draft.scopeKind === 'platform' ? '' : draft.scopeId} disabled={draft.scopeKind === 'platform'} onChange={(event) => onChange({ ...draft, scopeId: event.target.value })} placeholder={draft.scopeKind === 'platform' ? '平台作用域无需填写' : '输入具体资源 ID'} />
          </label>
          <label className="grid gap-1.5 text-sm md:col-span-2">
            <span className="text-muted-foreground">角色</span>
            <select value={draft.roleId} onChange={(event) => onChange({ ...draft, roleId: event.target.value })} className="h-10 rounded-md border bg-card px-3 text-sm">
              {roles.map((role) => <option key={role.id} value={role.id}>{role.name} · {role.id}</option>)}
            </select>
          </label>
        </div>
        <div className="flex justify-end gap-2 border-t px-5 py-4">
          <Button variant="outline" onClick={onCancel}>取消</Button>
          <Button onClick={onSave}>保存授权</Button>
        </div>
      </div>
    </div>
  );
}

function RoleMetadataDialog({ draft, permissions, onChange, onCancel, onSave }: {
  draft: RoleDraft;
  permissions: string[];
  onChange: (draft: RoleDraft) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  const grouped = groupPermissions(permissions);
  const selectedPermissions = new Set(draft.permissions);
  const selectedScopes = new Set(draft.suggestedScopes);
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/35 p-4">
      <div className="mt-8 flex max-h-[calc(100vh-64px)] w-full max-w-5xl flex-col rounded-md border bg-card shadow-xl">
        <div className="border-b px-5 py-4">
          <h2 className="text-lg font-semibold">{draft.mode === 'create' ? '新建角色' : '编辑角色信息'}</h2>
          <p className="mt-1 text-sm text-muted-foreground">角色可以动态管理；权限点仍由系统提供，避免出现接口无法识别的权限。</p>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          <div className="grid gap-4 lg:grid-cols-2">
            <label className="grid gap-1.5 text-sm">
              <span className="text-muted-foreground">角色 ID</span>
              <Input
                value={draft.id}
                disabled={draft.mode === 'edit'}
                onChange={(event) => onChange({ ...draft, id: event.target.value })}
                placeholder="例如 qa_tester"
              />
              <span className="text-xs text-muted-foreground">仅支持小写字母、数字、下划线和短横线。</span>
            </label>
            <label className="grid gap-1.5 text-sm">
              <span className="text-muted-foreground">角色名称</span>
              <Input value={draft.name} onChange={(event) => onChange({ ...draft, name: event.target.value })} placeholder="例如 测试人员" />
            </label>
            <label className="grid gap-1.5 text-sm lg:col-span-2">
              <span className="text-muted-foreground">角色说明</span>
              <Input value={draft.description} onChange={(event) => onChange({ ...draft, description: event.target.value })} placeholder="说明这个角色适用的岗位和边界" />
            </label>
          </div>

          <section className="mt-5 rounded-md border p-3">
            <div className="mb-3 text-sm font-semibold">建议作用域</div>
            <div className="flex flex-wrap gap-2">
              {scopeOptions.map((scope) => (
                <label key={scope.value} className="flex items-center gap-2 rounded-md border bg-muted/20 px-3 py-2 text-sm">
                  <input
                    type="checkbox"
                    checked={selectedScopes.has(scope.value)}
                    onChange={(event) => {
                      if (event.target.checked) onChange({ ...draft, suggestedScopes: [...draft.suggestedScopes, scope.value] });
                      else onChange({ ...draft, suggestedScopes: draft.suggestedScopes.filter((item) => item !== scope.value) });
                    }}
                  />
                  <span>{scope.label}</span>
                </label>
              ))}
            </div>
          </section>

          <section className="mt-5 rounded-md border p-3">
            <div className="mb-3 text-sm font-semibold">状态</div>
            <label className={cn('flex items-center gap-2 text-sm', draft.builtIn && 'text-muted-foreground')}>
              <input
                type="checkbox"
                checked={draft.disabled}
                disabled={draft.builtIn}
                onChange={(event) => onChange({ ...draft, disabled: event.target.checked })}
              />
              <span>停用角色。停用后不能再授予新主体，已有授权不会继续生效。</span>
            </label>
          </section>

          {draft.mode === 'create' && (
            <section className="mt-5">
              <div className="mb-3 text-sm font-semibold">权限点</div>
              <div className="grid gap-4 lg:grid-cols-2">
                {Object.entries(grouped).map(([group, items]) => (
                  <section key={group} className="rounded-md border p-3">
                    <div className="mb-3 text-sm font-semibold">{permissionGroupLabel(group)}</div>
                    <div className="grid gap-2">
                      {items.map((permission) => (
                        <label key={permission} className="flex items-center justify-between gap-3 rounded-md border bg-muted/20 px-3 py-2 text-sm">
                          <span className="font-mono text-xs">{permission}</span>
                          <input
                            type="checkbox"
                            checked={selectedPermissions.has(permission)}
                            onChange={(event) => {
                              if (event.target.checked) onChange({ ...draft, permissions: [...draft.permissions, permission] });
                              else onChange({ ...draft, permissions: draft.permissions.filter((item) => item !== permission) });
                            }}
                          />
                        </label>
                      ))}
                    </div>
                  </section>
                ))}
              </div>
            </section>
          )}
        </div>
        <div className="flex justify-end gap-2 border-t px-5 py-4">
          <Button variant="outline" onClick={onCancel}>取消</Button>
          <Button onClick={onSave}>
            <Save className="h-4 w-4" />
            保存角色
          </Button>
        </div>
      </div>
    </div>
  );
}

function RolePermissionsDialog({ role, permissions, values, onChange, onCancel, onSave }: {
  role: Role;
  permissions: string[];
  values: string[];
  onChange: (values: string[]) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  const grouped = groupPermissions(permissions);
  const selected = new Set(values);
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/35 p-4">
      <div className="mt-8 flex max-h-[calc(100vh-64px)] w-full max-w-5xl flex-col rounded-md border bg-card shadow-xl">
        <div className="border-b px-5 py-4">
          <h2 className="text-lg font-semibold">编辑角色权限</h2>
          <p className="mt-1 text-sm text-muted-foreground">{role.name} · {role.id}</p>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          <div className="grid gap-4 lg:grid-cols-2">
            {Object.entries(grouped).map(([group, items]) => (
              <section key={group} className="rounded-md border p-3">
                <div className="mb-3 text-sm font-semibold">{permissionGroupLabel(group)}</div>
                <div className="grid gap-2">
                  {items.map((permission) => (
                    <label key={permission} className="flex items-center justify-between gap-3 rounded-md border bg-muted/20 px-3 py-2 text-sm">
                      <span className="font-mono text-xs">{permission}</span>
                      <input
                        type="checkbox"
                        checked={selected.has(permission)}
                        onChange={(event) => {
                          if (event.target.checked) onChange([...values, permission]);
                          else onChange(values.filter((item) => item !== permission));
                        }}
                      />
                    </label>
                  ))}
                </div>
              </section>
            ))}
          </div>
        </div>
        <div className="flex justify-end gap-2 border-t px-5 py-4">
          <Button variant="outline" onClick={onCancel}>取消</Button>
          <Button onClick={onSave}>
            <Save className="h-4 w-4" />
            保存权限
          </Button>
        </div>
      </div>
    </div>
  );
}

function Metric({ label, value, icon: Icon }: { label: string; value: string; icon: typeof ShieldCheck }) {
  return (
    <Card>
      <CardContent className="flex items-center justify-between p-4">
        <div>
          <div className="text-xs text-muted-foreground">{label}</div>
          <div className="mt-1 text-xl font-semibold">{value}</div>
        </div>
        <div className="rounded-md border bg-muted/30 p-2 text-muted-foreground">
          <Icon className="h-4 w-4" />
        </div>
      </CardContent>
    </Card>
  );
}

function roleById(roles: Role[], roleId: string) {
  return roles.find((role) => role.id === roleId);
}

function subjectName(binding: RoleBinding, users: User[]) {
  if (binding.subjectType !== 'user') return binding.subjectId;
  return users.find((user) => user.id === binding.subjectId)?.displayName || binding.subjectId;
}

function subjectLabel(type: SubjectType) {
  if (type === 'user') return '用户';
  if (type === 'group') return '用户组';
  return '服务账号';
}

function scopeLabel(kind: ScopeKind) {
  return scopeOptions.find((item) => item.value === kind)?.label || kind;
}

function permissionSummary(permissions: string[]) {
  if (permissions.includes('*:*')) return '全部平台权限';
  if (permissions.length === 0) return '未配置权限';
  return `${permissions.slice(0, 4).join(' / ')}${permissions.length > 4 ? ` / +${permissions.length - 4}` : ''}`;
}

function groupPermissions(permissions: string[]) {
  return permissions.reduce<Record<string, string[]>>((groups, permission) => {
    const key = permission.split(':')[0] || 'other';
    groups[key] = groups[key] || [];
    groups[key].push(permission);
    return groups;
  }, {});
}

function permissionGroupLabel(group: string) {
  const labels: Record<string, string> = {
    tenant: '租户',
    project: '项目',
    application: '应用',
    stage: 'Stage',
    build: '构建',
    freight: 'Freight',
    deployment: '发布',
    runtime: '运行态',
    cluster: '集群',
    audit: '审计',
    role: '角色',
    user: '用户'
  };
  return labels[group] || group;
}

function formatTime(value?: string) {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败';
}
