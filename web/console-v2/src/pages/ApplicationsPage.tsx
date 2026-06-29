import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowUpRight, Boxes, Pencil, Plus, RefreshCcw, Rocket, Workflow } from 'lucide-react';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { createApplication, listApplications, updateApplication, type Application, type SaveApplicationInput } from '../api/applications';
import { usePlatformSelection } from '../contexts/PlatformSelectionContext';

type ApplicationTab = 'joined' | 'project' | 'disabled';

type ApplicationDraft = {
  id?: string;
  projectId: string;
  name: string;
  displayName: string;
  description: string;
  disabled: boolean;
};

const emptyDraft: ApplicationDraft = {
  projectId: '',
  name: '',
  displayName: '',
  description: '',
  disabled: false
};

export function ApplicationsPage() {
  const navigate = useNavigate();
  const { tenants, currentTenant, currentProject, loading: contextLoading, refreshContexts, setApplication } = usePlatformSelection();
  const [activeTab, setActiveTab] = useState<ApplicationTab>('joined');
  const [joinedApps, setJoinedApps] = useState<Application[]>([]);
  const [projectApps, setProjectApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [draft, setDraft] = useState<ApplicationDraft | null>(null);

  const projects = useMemo(
    () => tenants.flatMap((tenant) => tenant.projects.map((project) => ({ ...project, tenantId: tenant.id, tenantName: tenant.name }))),
    [tenants]
  );
  const currentProjectName = projects.find((project) => project.id === currentProject.id)?.name || currentProject.name;
  const visibleApps = activeTab === 'joined'
    ? joinedApps.filter((app) => app.status !== 'disabled')
    : activeTab === 'disabled'
      ? projectApps.filter((app) => app.status === 'disabled')
      : projectApps.filter((app) => app.status !== 'disabled');

  useEffect(() => {
    void reload();
  }, [currentProject.id]);

  async function reload() {
    setLoading(true);
    setError('');
    try {
      const [joined, project] = await Promise.all([
        listApplications('joined'),
        currentProject.id ? listApplications('accessible', currentProject.id) : Promise.resolve({ items: [], total: 0 })
      ]);
      setJoinedApps(joined.items);
      setProjectApps(project.items);
    } catch (err) {
      setError(err instanceof Error ? err.message : '应用列表加载失败');
    } finally {
      setLoading(false);
    }
  }

  function openCreateDialog() {
    setDraft({
      ...emptyDraft,
      projectId: currentProject.id
    });
  }

  function openEditDialog(app: Application) {
    setDraft({
      id: app.id,
      projectId: app.projectId,
      name: app.name,
      displayName: app.displayName,
      description: app.description,
      disabled: app.status === 'disabled'
    });
  }

  async function saveDraft() {
    if (!draft) return;
    setSaving(true);
    setError('');
    const input: SaveApplicationInput = {
      projectId: draft.projectId,
      name: draft.name,
      displayName: draft.displayName,
      description: draft.description,
      disabled: draft.disabled
    };
    try {
      const app = draft.id ? await updateApplication(draft.id, input) : await createApplication(input);
      setDraft(null);
      await reload();
      await refreshContexts();
      if (!draft.id) {
        setApplication(app.id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '应用保存失败');
    } finally {
      setSaving(false);
    }
  }

  function enterApp(app: Application, path: '/deployments' | '/pipelines') {
    const project = projects.find((item) => item.id === app.projectId);
    const tenantId = app.tenantId || project?.tenantId || currentTenant.id;
    const projectId = app.projectId || currentProject.id;
    const params = new URLSearchParams({
      tenantId,
      projectId,
      applicationId: app.id
    });
    navigate(`${path}?${params.toString()}`);
  }

  return (
    <div className="space-y-5">
      <section className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <div className="dense-label">应用管理</div>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">应用</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            “我加入的应用”只展示你拥有应用级角色绑定的应用；项目权限可访问的应用在“项目内”查看。
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={reload} disabled={loading || contextLoading}>
            <RefreshCcw className="h-4 w-4" />
            刷新
          </Button>
          <Button onClick={openCreateDialog}>
            <Plus className="h-4 w-4" />
            创建应用
          </Button>
        </div>
      </section>

      {error && <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">{error}</div>}

      <section className="grid gap-4 md:grid-cols-3">
        <SummaryCard label="我加入的" value={joinedApps.filter((app) => app.status !== 'disabled').length} note="应用级角色绑定" />
        <SummaryCard label="项目内" value={projectApps.filter((app) => app.status !== 'disabled').length} note={currentProjectName} />
        <SummaryCard label="已禁用" value={projectApps.filter((app) => app.status === 'disabled').length} note="当前项目" />
      </section>

      <Card>
        <CardHeader className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <CardTitle>应用列表</CardTitle>
            <CardDescription>创建应用后，创建人会自动获得应用管理员角色。</CardDescription>
          </div>
          <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as ApplicationTab)}>
            <TabsList>
              <TabsTrigger value="joined">我加入的</TabsTrigger>
              <TabsTrigger value="project">项目内</TabsTrigger>
              <TabsTrigger value="disabled">已禁用</TabsTrigger>
            </TabsList>
          </Tabs>
        </CardHeader>
        <CardContent>
          <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as ApplicationTab)}>
            <TabsContent value={activeTab} className="mt-0">
              <ApplicationTable
                applications={visibleApps}
                projects={projects}
                loading={loading || contextLoading}
                emptyText={activeTab === 'joined' ? '暂无明确加入的应用' : activeTab === 'disabled' ? '当前项目暂无已禁用应用' : '当前项目暂无应用'}
                onEdit={openEditDialog}
                onDeploy={(app) => enterApp(app, '/deployments')}
                onPipeline={(app) => enterApp(app, '/pipelines')}
              />
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {draft && (
        <ApplicationDialog
          draft={draft}
          projects={projects}
          saving={saving}
          onChange={setDraft}
          onClose={() => setDraft(null)}
          onSubmit={saveDraft}
        />
      )}
    </div>
  );
}

function SummaryCard({ label, value, note }: { label: string; value: number; note: string }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{label}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="mono text-2xl font-semibold">{value}</div>
        <p className="mt-1 text-xs text-muted-foreground">{note}</p>
      </CardContent>
    </Card>
  );
}

function ApplicationTable({
  applications,
  projects,
  loading,
  emptyText,
  onEdit,
  onDeploy,
  onPipeline
}: {
  applications: Application[];
  projects: Array<{ id: string; name: string; tenantId: string; tenantName: string }>;
  loading: boolean;
  emptyText: string;
  onEdit: (app: Application) => void;
  onDeploy: (app: Application) => void;
  onPipeline: (app: Application) => void;
}) {
  if (loading) {
    return <div className="rounded-md border bg-muted/30 px-4 py-8 text-center text-sm text-muted-foreground">应用列表加载中...</div>;
  }
  if (applications.length === 0) {
    return <div className="rounded-md border bg-muted/30 px-4 py-8 text-center text-sm text-muted-foreground">{emptyText}</div>;
  }
  return (
    <div className="overflow-hidden rounded-md border">
      <table className="w-full text-left text-sm">
        <thead className="bg-muted/70 text-xs text-muted-foreground">
          <tr>
            <th className="px-3 py-2 font-medium">应用</th>
            <th className="px-3 py-2 font-medium">所属项目</th>
            <th className="px-3 py-2 font-medium">我的角色</th>
            <th className="px-3 py-2 font-medium">状态</th>
            <th className="px-3 py-2 font-medium">更新时间</th>
            <th className="px-3 py-2 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody>
          {applications.map((app) => {
            const project = projects.find((item) => item.id === app.projectId);
            return (
              <tr key={app.id} className="border-t bg-card align-middle">
                <td className="px-3 py-3">
                  <div className="font-medium">{app.displayName}</div>
                  <div className="mono text-xs text-muted-foreground">{app.name}</div>
                </td>
                <td className="px-3 py-3">
                  <div>{project?.name || app.projectId}</div>
                  <div className="text-xs text-muted-foreground">{project?.tenantName || '租户'}</div>
                </td>
                <td className="px-3 py-3">
                  {app.myRoleId ? <Badge variant="outline">{roleLabel(app.myRoleId)}</Badge> : <span className="text-muted-foreground">项目权限</span>}
                </td>
                <td className="px-3 py-3">
                  <Badge variant={app.status === 'disabled' ? 'muted' : 'success'}>{app.status === 'disabled' ? '已禁用' : '启用中'}</Badge>
                </td>
                <td className="mono px-3 py-3 text-xs text-muted-foreground">{formatTime(app.updatedAt || app.createdAt)}</td>
                <td className="px-3 py-3">
                  <div className="flex justify-end gap-2">
                    <Button variant="outline" size="sm" onClick={() => onDeploy(app)}>
                      <Rocket className="h-4 w-4" />
                      部署
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => onPipeline(app)}>
                      <Workflow className="h-4 w-4" />
                      流水线
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => onEdit(app)}>
                      <Pencil className="h-4 w-4" />
                      编辑
                    </Button>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function ApplicationDialog({
  draft,
  projects,
  saving,
  onChange,
  onClose,
  onSubmit
}: {
  draft: ApplicationDraft;
  projects: Array<{ id: string; name: string; tenantId: string; tenantName: string }>;
  saving: boolean;
  onChange: (draft: ApplicationDraft) => void;
  onClose: () => void;
  onSubmit: () => void;
}) {
  const editing = Boolean(draft.id);
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 px-4">
      <div className="w-full max-w-2xl rounded-lg border bg-card shadow-xl">
        <div className="border-b p-4">
          <div className="flex items-center gap-2">
            <Boxes className="h-5 w-5 text-primary" />
            <h2 className="text-base font-semibold">{editing ? '编辑应用' : '创建应用'}</h2>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">这里只维护应用元信息；Workload、源码和流水线在应用详情流程中配置。</p>
        </div>
        <div className="grid gap-4 p-4">
          <label className="space-y-1.5 text-sm">
            <span className="font-medium">所属项目</span>
            <select
              value={draft.projectId}
              disabled={editing}
              onChange={(event) => onChange({ ...draft, projectId: event.target.value })}
              className="h-9 w-full rounded-md border bg-card px-3 text-sm shadow-control disabled:opacity-60"
            >
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.tenantName} / {project.name}
                </option>
              ))}
            </select>
          </label>
          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-1.5 text-sm">
              <span className="font-medium">应用标识</span>
              <Input
                value={draft.name}
                disabled={editing}
                placeholder="例如：log-receiver"
                onChange={(event) => onChange({ ...draft, name: event.target.value })}
              />
              <span className="text-xs text-muted-foreground">小写字母、数字和中划线，创建后不再修改。</span>
            </label>
            <label className="space-y-1.5 text-sm">
              <span className="font-medium">应用名称</span>
              <Input
                value={draft.displayName}
                placeholder="例如：日志接收服务"
                onChange={(event) => onChange({ ...draft, displayName: event.target.value })}
              />
            </label>
          </div>
          <label className="space-y-1.5 text-sm">
            <span className="font-medium">描述</span>
            <textarea
              value={draft.description}
              rows={4}
              placeholder="应用职责、归属团队、重要说明..."
              onChange={(event) => onChange({ ...draft, description: event.target.value })}
              className="w-full rounded-md border bg-card px-3 py-2 text-sm shadow-control placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </label>
          {editing && (
            <label className="flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2 text-sm">
              <input
                type="checkbox"
                checked={draft.disabled}
                onChange={(event) => onChange({ ...draft, disabled: event.target.checked })}
              />
              <span>禁用应用</span>
            </label>
          )}
        </div>
        <div className="flex items-center justify-end gap-2 border-t p-4">
          <Button variant="outline" onClick={onClose} disabled={saving}>
            取消
          </Button>
          <Button onClick={onSubmit} disabled={saving || !draft.projectId || !draft.name.trim()}>
            {saving ? '保存中...' : editing ? '保存' : '创建'}
            {!saving && <ArrowUpRight className="h-4 w-4" />}
          </Button>
        </div>
      </div>
    </div>
  );
}

function roleLabel(roleID: string) {
  const labels: Record<string, string> = {
    application_admin: '应用管理员',
    developer: '开发者',
    operator: '运维',
    viewer: '只读'
  };
  return labels[roleID] || roleID;
}

function formatTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString('zh-CN', { hour12: false });
}
