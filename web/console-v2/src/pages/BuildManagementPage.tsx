import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Code2, Edit3, Hammer, Layers3, MoreHorizontal, Plus, RefreshCcw, Save, ServerCog, Trash2 } from 'lucide-react';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import {
  createBuildEnvironment,
  createRuntimeEnvironment,
  deleteBuildEnvironment,
  deleteRuntimeEnvironment,
  loadBuildManagement,
  saveBuildTemplate,
  updateBuildEnvironment,
  updateRuntimeEnvironment,
  type BuildEnvironment,
  type BuildManagementBundle,
  type RuntimeEnvironment,
  type RuntimeImage
} from '../api/buildManagement';
import { cn } from '../lib/utils';

type LabelRow = { key: string; value: string };
type BuildEnvironmentDraft = Pick<BuildEnvironment, 'id' | 'name' | 'description' | 'buildImage' | 'status' | 'isDefault'>;
type RuntimeEnvironmentDraft = RuntimeEnvironment & { imageRows: RuntimeImageDraft[] };
type RuntimeImageDraft = Omit<RuntimeImage, 'selectorLabels'> & { labelRows: LabelRow[] };

const emptyBuildEnvironment: BuildEnvironmentDraft = {
  id: '',
  name: '',
  description: '',
  buildImage: '',
  status: 'enabled',
  isDefault: false
};

const emptyRuntimeEnvironment: RuntimeEnvironmentDraft = {
  id: '',
  name: '',
  description: '',
  runtimeBaseImage: '',
  artifactDeployPath: '/app',
  dockerfilePath: 'java/jar/Dockerfile',
  selectorLabels: { cloud: 'aliyun' },
  images: [],
  imageRows: [emptyRuntimeImage()],
  status: 'enabled'
};

export function BuildManagementPage() {
  const [bundle, setBundle] = useState<BuildManagementBundle | null>(null);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [buildEditing, setBuildEditing] = useState<BuildEnvironmentDraft | null>(null);
  const [runtimeEditing, setRuntimeEditing] = useState<RuntimeEnvironmentDraft | null>(null);
  const [templateDraft, setTemplateDraft] = useState('');
  const [savingTemplate, setSavingTemplate] = useState(false);

  useEffect(() => {
    void reload();
  }, []);

  useEffect(() => {
    if (bundle?.buildTemplate) {
      setTemplateDraft(bundle.buildTemplate.content);
    }
  }, [bundle?.buildTemplate]);

  const stats = useMemo(() => {
    const buildEnabled = bundle?.buildEnvironments.filter((item) => item.status === 'enabled').length || 0;
    const runtimeEnabled = bundle?.runtimeEnvironments.filter((item) => item.status === 'enabled').length || 0;
    const runtimeImages = bundle?.runtimeEnvironments.reduce((sum, item) => sum + item.images.length, 0) || 0;
    return { buildEnabled, runtimeEnabled, runtimeImages, templateVersion: bundle?.buildTemplate.version || 0 };
  }, [bundle]);

  async function reload() {
    setLoading(true);
    setMessage('');
    try {
      await refreshData();
    } catch (error) {
      setMessage(`构建管理数据加载失败：${errorMessage(error)}`);
    } finally {
      setLoading(false);
    }
  }

  async function refreshData() {
    const next = await loadBuildManagement();
    setBundle(next);
  }

  function openBuildEnvironment(environment?: BuildEnvironment) {
    setBuildEditing(environment ? { ...environment } : { ...emptyBuildEnvironment });
    setMessage('');
  }

  function openRuntimeEnvironment(environment?: RuntimeEnvironment) {
    setRuntimeEditing(environment ? runtimeToDraft(environment) : { ...emptyRuntimeEnvironment, imageRows: [emptyRuntimeImage()] });
    setMessage('');
  }

  async function saveBuildEnvironmentDraft() {
    if (!buildEditing) return;
    if (!buildEditing.name.trim() || !buildEditing.buildImage.trim()) {
      setMessage('构建环境名称和构建镜像不能为空');
      return;
    }
    try {
      if (buildEditing.id) {
        await updateBuildEnvironment({ ...buildEditing, updatedAt: undefined });
      } else {
        await createBuildEnvironment(buildEditing);
      }
      setBuildEditing(null);
      await refreshData();
      setMessage(buildEditing.id ? '构建环境已保存' : '构建环境已创建');
    } catch (error) {
      setMessage(`构建环境保存失败：${errorMessage(error)}`);
    }
  }

  async function removeBuildEnvironment(environment: BuildEnvironment) {
    if (!window.confirm(`确认删除构建环境「${environment.name}」？`)) return;
    try {
      await deleteBuildEnvironment(environment.id);
      await refreshData();
      setMessage('构建环境已删除');
    } catch (error) {
      setMessage(`构建环境删除失败：${errorMessage(error)}`);
    }
  }

  async function saveRuntimeEnvironmentDraft() {
    if (!runtimeEditing) return;
    const input = draftToRuntime(runtimeEditing);
    if (!input.name.trim() || input.images.some((image) => !image.name.trim() || !image.runtimeBaseImage.trim())) {
      setMessage('运行环境名称、镜像名称和基础镜像不能为空');
      return;
    }
    if (input.images.some((image) => Object.keys(image.selectorLabels).length === 0)) {
      setMessage('每个运行时镜像至少需要一个匹配标签');
      return;
    }
    try {
      if (input.id) {
        await updateRuntimeEnvironment(input);
      } else {
        await createRuntimeEnvironment(input);
      }
      setRuntimeEditing(null);
      await refreshData();
      setMessage(input.id ? '运行环境已保存' : '运行环境已创建');
    } catch (error) {
      setMessage(`运行环境保存失败：${errorMessage(error)}`);
    }
  }

  async function removeRuntimeEnvironment(environment: RuntimeEnvironment) {
    if (!window.confirm(`确认删除运行环境「${environment.name}」？`)) return;
    try {
      await deleteRuntimeEnvironment(environment.id);
      await refreshData();
      setMessage('运行环境已删除');
    } catch (error) {
      setMessage(`运行环境删除失败：${errorMessage(error)}`);
    }
  }

  async function saveTemplateDraft() {
    if (!templateDraft.trim()) {
      setMessage('构建模板内容不能为空');
      return;
    }
    setSavingTemplate(true);
    try {
      const template = await saveBuildTemplate(templateDraft);
      setBundle((current) => current ? { ...current, buildTemplate: template } : current);
      setMessage('构建模板已保存');
    } catch (error) {
      setMessage(`构建模板保存失败：${errorMessage(error)}`);
    } finally {
      setSavingTemplate(false);
    }
  }

  const buildEnvironments = bundle?.buildEnvironments || [];
  const runtimeEnvironments = bundle?.runtimeEnvironments || [];
  const template = bundle?.buildTemplate;

  return (
    <div className="flex min-h-[calc(100vh-96px)] flex-col gap-4">
      <section className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="dense-label">平台级配置</div>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">构建管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">维护构建镜像、运行时镜像和全局 Jenkins 构建模板，创建流水线时会读取这些配置。</p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline">真实后端</Badge>
          <Button variant="outline" onClick={reload} disabled={loading}>
            <RefreshCcw className={cn('h-4 w-4', loading && 'animate-spin')} />
            刷新
          </Button>
        </div>
      </section>

      {message && (
        <div className="rounded-md border bg-muted/35 px-3 py-2 text-sm text-muted-foreground">
          {message}
        </div>
      )}

      <section className="grid gap-3 lg:grid-cols-4">
        <Metric label="启用构建环境" value={`${stats.buildEnabled} 个`} icon={Hammer} tone="success" />
        <Metric label="启用运行环境" value={`${stats.runtimeEnabled} 个`} icon={ServerCog} />
        <Metric label="运行时镜像" value={`${stats.runtimeImages} 个`} icon={Layers3} />
        <Metric label="模板版本" value={`v${stats.templateVersion}`} icon={Code2} />
      </section>

      <Tabs defaultValue="build-env" className="min-h-0 flex-1">
        <TabsList>
          <TabsTrigger value="build-env">构建环境</TabsTrigger>
          <TabsTrigger value="runtime-env">运行环境</TabsTrigger>
          <TabsTrigger value="template">构建模板</TabsTrigger>
        </TabsList>

        <TabsContent value="build-env">
          <Card>
            <CardHeader className="flex flex-row items-start justify-between gap-4 border-b">
              <div>
                <CardTitle>构建环境</CardTitle>
                <CardDescription>构建环境决定 Jenkins 构建阶段使用的工具镜像，例如 Maven、Gradle、Node。</CardDescription>
              </div>
              <Button onClick={() => openBuildEnvironment()}>
                <Plus className="h-4 w-4" />
                新建构建环境
              </Button>
            </CardHeader>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[920px] border-collapse text-sm">
                  <thead className="bg-muted/45 text-left text-xs font-semibold text-muted-foreground">
                    <tr>
                      <th className="px-4 py-3">名称</th>
                      <th className="px-4 py-3">构建镜像</th>
                      <th className="px-4 py-3">状态</th>
                      <th className="px-4 py-3">默认</th>
                      <th className="px-4 py-3">更新时间</th>
                      <th className="px-4 py-3 text-right">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {buildEnvironments.map((environment) => (
                      <tr key={environment.id} className="border-t hover:bg-muted/25">
                        <td className="px-4 py-3">
                          <div className="font-medium">{environment.name}</div>
                          <div className="text-xs text-muted-foreground">{environment.description || '无描述'}</div>
                        </td>
                        <td className="mono max-w-[420px] truncate px-4 py-3 text-xs" title={environment.buildImage}>{environment.buildImage}</td>
                        <td className="px-4 py-3"><StatusBadge status={environment.status} /></td>
                        <td className="px-4 py-3">{environment.isDefault ? <Badge variant="success">默认</Badge> : <span className="text-muted-foreground">否</span>}</td>
                        <td className="px-4 py-3 text-muted-foreground">{environment.updatedAt || '-'}</td>
                        <td className="px-4 py-3">
                          <div className="flex justify-end gap-1.5">
                            <Button variant="outline" size="sm" onClick={() => openBuildEnvironment(environment)}><Edit3 className="h-3.5 w-3.5" />编辑</Button>
                            <Button variant="outline" size="sm" onClick={() => removeBuildEnvironment(environment)}><Trash2 className="h-3.5 w-3.5" />删除</Button>
                          </div>
                        </td>
                      </tr>
                    ))}
                    {loading && <EmptyRow colSpan={6} text="正在加载构建环境..." />}
                    {!loading && buildEnvironments.length === 0 && <EmptyRow colSpan={6} text="暂无构建环境" />}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="runtime-env">
          <Card>
            <CardHeader className="flex flex-row items-start justify-between gap-4 border-b">
              <div>
                <CardTitle>运行环境</CardTitle>
                <CardDescription>运行环境定义最终镜像的基础镜像、Dockerfile 路径和按集群标签匹配的镜像集合。</CardDescription>
              </div>
              <Button onClick={() => openRuntimeEnvironment()}>
                <Plus className="h-4 w-4" />
                新建运行环境
              </Button>
            </CardHeader>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[1100px] border-collapse text-sm">
                  <thead className="bg-muted/45 text-left text-xs font-semibold text-muted-foreground">
                    <tr>
                      <th className="px-4 py-3">名称</th>
                      <th className="px-4 py-3">运行时镜像</th>
                      <th className="px-4 py-3">Dockerfile</th>
                      <th className="px-4 py-3">部署路径</th>
                      <th className="px-4 py-3">状态</th>
                      <th className="px-4 py-3 text-right">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {runtimeEnvironments.map((environment) => (
                      <tr key={environment.id} className="border-t hover:bg-muted/25">
                        <td className="px-4 py-3 align-top">
                          <div className="font-medium">{environment.name}</div>
                          <div className="text-xs text-muted-foreground">{environment.description || '无描述'}</div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="space-y-1">
                            {environment.images.map((image) => (
                              <div key={image.id || image.name} className="rounded-md border bg-muted/25 px-2 py-1.5">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="font-medium">{image.displayName || image.name}</span>
                                  <LabelTags labels={image.selectorLabels} />
                                </div>
                                <div className="mono mt-1 max-w-[420px] truncate text-xs text-muted-foreground" title={image.runtimeBaseImage}>{image.runtimeBaseImage}</div>
                              </div>
                            ))}
                          </div>
                        </td>
                        <td className="mono px-4 py-3 text-xs">{environment.dockerfilePath || '-'}</td>
                        <td className="mono px-4 py-3 text-xs">{environment.artifactDeployPath || '-'}</td>
                        <td className="px-4 py-3"><StatusBadge status={environment.status} /></td>
                        <td className="px-4 py-3">
                          <div className="flex justify-end gap-1.5">
                            <Button variant="outline" size="sm" onClick={() => openRuntimeEnvironment(environment)}><Edit3 className="h-3.5 w-3.5" />编辑</Button>
                            <Button variant="outline" size="sm" onClick={() => removeRuntimeEnvironment(environment)}><Trash2 className="h-3.5 w-3.5" />删除</Button>
                          </div>
                        </td>
                      </tr>
                    ))}
                    {loading && <EmptyRow colSpan={6} text="正在加载运行环境..." />}
                    {!loading && runtimeEnvironments.length === 0 && <EmptyRow colSpan={6} text="暂无运行环境" />}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="template">
          <Card className="min-h-[620px]">
            <CardHeader className="flex flex-row items-start justify-between gap-4 border-b">
              <div>
                <CardTitle>全局构建模板</CardTitle>
                <CardDescription>当前流水线使用 `global-build-template` 渲染 Jenkinsfile。保存前服务端会执行模板校验。</CardDescription>
              </div>
              <div className="flex items-center gap-2">
                {template && <Badge variant="outline">版本 v{template.version}</Badge>}
                <Button onClick={saveTemplateDraft} disabled={savingTemplate || !templateDraft.trim()}>
                  <Save className="h-4 w-4" />
                  {savingTemplate ? '保存中' : '保存模板'}
                </Button>
              </div>
            </CardHeader>
            <CardContent className="grid min-h-[560px] gap-4 p-4 lg:grid-cols-[minmax(0,1fr)_320px]">
              <textarea
                value={templateDraft}
                onChange={(event) => setTemplateDraft(event.target.value)}
                spellCheck={false}
                className="mono min-h-[520px] w-full resize-none rounded-md border bg-slate-950 p-4 text-xs leading-5 text-slate-100 outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring"
              />
              <aside className="space-y-3 rounded-md border bg-muted/25 p-4 text-sm">
                <div>
                  <div className="font-medium">模板变量</div>
                  <p className="mt-1 text-muted-foreground">模板由后端 Go template 渲染，常用变量包括源码、运行环境、镜像目标和回调地址。</p>
                </div>
                <InfoRow label="模板 ID" value={template?.id || 'global-build-template'} />
                <InfoRow label="更新时间" value={template?.updatedAt || '-'} />
                <div className="rounded-md border bg-card p-3 text-xs text-muted-foreground">
                  Git 和 SVN checkout 凭据由模板统一维护，不在应用创建流水线时暴露给普通用户。
                </div>
              </aside>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {buildEditing && (
        <BuildEnvironmentDialog
          draft={buildEditing}
          onChange={setBuildEditing}
          onClose={() => setBuildEditing(null)}
          onSave={saveBuildEnvironmentDraft}
        />
      )}

      {runtimeEditing && (
        <RuntimeEnvironmentDialog
          draft={runtimeEditing}
          onChange={setRuntimeEditing}
          onClose={() => setRuntimeEditing(null)}
          onSave={saveRuntimeEnvironmentDraft}
        />
      )}
    </div>
  );
}

function BuildEnvironmentDialog({ draft, onChange, onClose, onSave }: { draft: BuildEnvironmentDraft; onChange: (draft: BuildEnvironmentDraft) => void; onClose: () => void; onSave: () => void }) {
  return (
    <DialogShell title={draft.id ? '编辑构建环境' : '新建构建环境'} description="构建环境会作为代码源构建阶段的工具镜像。">
      <div className="flex-1 space-y-4 overflow-y-auto p-5">
        <InlineField label="名称">
          <Input value={draft.name} disabled={!!draft.id} placeholder="maven-jdk17" onChange={(event) => onChange({ ...draft, name: event.target.value })} />
        </InlineField>
        <InlineField label="描述">
          <Input value={draft.description} placeholder="Maven + JDK 17 构建镜像" onChange={(event) => onChange({ ...draft, description: event.target.value })} />
        </InlineField>
        <InlineField label="构建镜像">
          <Input value={draft.buildImage} placeholder="registry.example.com/build/maven:3.9-jdk17" onChange={(event) => onChange({ ...draft, buildImage: event.target.value })} />
        </InlineField>
        <InlineField label="状态">
          <select value={draft.status} onChange={(event) => onChange({ ...draft, status: event.target.value as BuildEnvironment['status'] })} className="h-10 rounded-md border bg-card px-3 text-sm">
            <option value="enabled">启用</option>
            <option value="disabled">禁用</option>
          </select>
        </InlineField>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={draft.isDefault} onChange={(event) => onChange({ ...draft, isDefault: event.target.checked })} />
          设为默认构建环境
        </label>
      </div>
      <DialogFooter onClose={onClose} onSave={onSave} />
    </DialogShell>
  );
}

function RuntimeEnvironmentDialog({ draft, onChange, onClose, onSave }: { draft: RuntimeEnvironmentDraft; onChange: (draft: RuntimeEnvironmentDraft) => void; onClose: () => void; onSave: () => void }) {
  function updateImage(index: number, patch: Partial<RuntimeImageDraft>) {
    onChange({ ...draft, imageRows: draft.imageRows.map((image, itemIndex) => itemIndex === index ? { ...image, ...patch } : image) });
  }

  return (
    <DialogShell title={draft.id ? '编辑运行环境' : '新建运行环境'} description="运行环境决定最终业务镜像使用的基础镜像、Dockerfile 和集群标签匹配规则。" wide>
      <div className="flex-1 space-y-5 overflow-y-auto p-5">
        <div className="grid gap-4 lg:grid-cols-2">
          <InlineField label="名称">
            <Input value={draft.name} disabled={!!draft.id} placeholder="springboot-jdk11" onChange={(event) => onChange({ ...draft, name: event.target.value })} />
          </InlineField>
          <InlineField label="状态">
            <select value={draft.status} onChange={(event) => onChange({ ...draft, status: event.target.value as RuntimeEnvironment['status'] })} className="h-10 rounded-md border bg-card px-3 text-sm">
              <option value="enabled">启用</option>
              <option value="disabled">禁用</option>
            </select>
          </InlineField>
        </div>
        <InlineField label="描述">
          <Input value={draft.description} placeholder="Spring Boot JAR 运行环境" onChange={(event) => onChange({ ...draft, description: event.target.value })} />
        </InlineField>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <div className="font-medium">运行时镜像</div>
              <div className="text-sm text-muted-foreground">每个镜像通过标签匹配集群，例如 cloud=aliyun。</div>
            </div>
            <Button variant="outline" size="sm" onClick={() => onChange({ ...draft, imageRows: [...draft.imageRows, emptyRuntimeImage()] })}>
              <Plus className="h-3.5 w-3.5" />
              添加镜像
            </Button>
          </div>
          {draft.imageRows.map((image, index) => (
            <div key={index} className="space-y-3 rounded-md border bg-muted/20 p-3">
              <div className="grid gap-3 lg:grid-cols-2">
                <InlineField label="镜像名">
                  <Input value={image.name} placeholder="aliyun" onChange={(event) => updateImage(index, { name: event.target.value })} />
                </InlineField>
                <InlineField label="显示名">
                  <Input value={image.displayName} placeholder="阿里云 Dragonwell" onChange={(event) => updateImage(index, { displayName: event.target.value })} />
                </InlineField>
              </div>
              <InlineField label="基础镜像">
                <Input value={image.runtimeBaseImage} placeholder="registry.example/runtime/java:11" onChange={(event) => updateImage(index, { runtimeBaseImage: event.target.value })} />
              </InlineField>
              <div className="grid gap-3 lg:grid-cols-2">
                <InlineField label="Dockerfile">
                  <Input value={image.dockerfilePath} placeholder="java/jar/Dockerfile" onChange={(event) => updateImage(index, { dockerfilePath: event.target.value })} />
                </InlineField>
                <InlineField label="部署路径">
                  <Input value={image.artifactDeployPath} placeholder="/app" onChange={(event) => updateImage(index, { artifactDeployPath: event.target.value })} />
                </InlineField>
              </div>
              <LabelRowsEditor rows={image.labelRows} onChange={(rows) => updateImage(index, { labelRows: rows })} />
              <div className="flex justify-end">
                <Button variant="outline" size="sm" disabled={draft.imageRows.length <= 1} onClick={() => onChange({ ...draft, imageRows: draft.imageRows.filter((_, itemIndex) => itemIndex !== index) })}>
                  删除镜像
                </Button>
              </div>
            </div>
          ))}
        </div>
      </div>
      <DialogFooter onClose={onClose} onSave={onSave} />
    </DialogShell>
  );
}

function DialogShell({ title, description, children, wide }: { title: string; description: string; children: ReactNode; wide?: boolean }) {
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/35 p-4">
      <div className={cn('mt-6 flex h-[min(780px,calc(100vh-48px))] flex-col overflow-hidden rounded-lg border bg-card shadow-2xl', wide ? 'w-[960px]' : 'w-[720px]')}>
        <div className="flex items-start justify-between border-b px-5 py-4">
          <div>
            <h2 className="text-lg font-semibold">{title}</h2>
            <p className="text-sm text-muted-foreground">{description}</p>
          </div>
          <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
        </div>
        {children}
      </div>
    </div>
  );
}

function DialogFooter({ onClose, onSave }: { onClose: () => void; onSave: () => void }) {
  return (
    <div className="flex justify-end gap-2 border-t px-5 py-4">
      <Button variant="outline" onClick={onClose}>取消</Button>
      <Button onClick={onSave}><Save className="h-4 w-4" />保存</Button>
    </div>
  );
}

function LabelRowsEditor({ rows, onChange }: { rows: LabelRow[]; onChange: (rows: LabelRow[]) => void }) {
  return (
    <div className="space-y-2">
      <div className="text-sm font-medium">匹配标签</div>
      {rows.map((row, index) => (
        <div key={index} className="grid grid-cols-[1fr_1fr_auto] gap-2">
          <Input value={row.key} placeholder="键" onChange={(event) => onChange(rows.map((item, itemIndex) => itemIndex === index ? { ...item, key: event.target.value } : item))} />
          <Input value={row.value} placeholder="值" onChange={(event) => onChange(rows.map((item, itemIndex) => itemIndex === index ? { ...item, value: event.target.value } : item))} />
          <Button variant="outline" onClick={() => onChange(rows.filter((_, itemIndex) => itemIndex !== index))}>删除</Button>
        </div>
      ))}
      <Button variant="outline" size="sm" onClick={() => onChange([...rows, { key: '', value: '' }])}>添加标签</Button>
    </div>
  );
}

function Metric({ label, value, icon: Icon, tone }: { label: string; value: string; icon: typeof Hammer; tone?: 'success' }) {
  return (
    <Card>
      <CardContent className="flex items-center justify-between p-4">
        <div>
          <div className="text-sm text-muted-foreground">{label}</div>
          <div className="mt-1 text-xl font-semibold">{value}</div>
        </div>
        <div className={cn('flex h-10 w-10 items-center justify-center rounded-md border bg-muted text-muted-foreground', tone === 'success' && 'text-success')}>
          <Icon className="h-5 w-5" />
        </div>
      </CardContent>
    </Card>
  );
}

function StatusBadge({ status }: { status: BuildEnvironment['status'] | RuntimeEnvironment['status'] }) {
  if (status === 'enabled') return <Badge variant="success">启用</Badge>;
  if (status === 'disabled') return <Badge variant="muted">禁用</Badge>;
  return <Badge variant="destructive">已删除</Badge>;
}

function EmptyRow({ colSpan, text }: { colSpan: number; text: string }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-4 py-14 text-center text-muted-foreground">{text}</td>
    </tr>
  );
}

function InlineField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="grid grid-cols-[96px_minmax(0,1fr)] items-center gap-3 text-sm">
      <span className="font-medium text-muted-foreground">{label}</span>
      {children}
    </label>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3 border-t pt-3 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="mono text-right">{value}</span>
    </div>
  );
}

function LabelTags({ labels }: { labels: Record<string, string> }) {
  const entries = Object.entries(labels || {});
  if (!entries.length) return <span className="text-muted-foreground">无</span>;
  return (
    <div className="flex flex-wrap gap-1">
      {entries.map(([key, value]) => (
        <span key={key} className="rounded border bg-card px-1.5 py-0.5 text-xs text-muted-foreground">{key}={value}</span>
      ))}
    </div>
  );
}

function emptyRuntimeImage(): RuntimeImageDraft {
  return {
    id: undefined,
    name: 'aliyun',
    displayName: '阿里云镜像',
    runtimeBaseImage: '',
    artifactDeployPath: '/app',
    dockerfilePath: 'java/jar/Dockerfile',
    labelRows: [{ key: 'cloud', value: 'aliyun' }],
    status: 'enabled'
  };
}

function runtimeToDraft(environment: RuntimeEnvironment): RuntimeEnvironmentDraft {
  return {
    ...environment,
    imageRows: (environment.images.length ? environment.images : [{
      name: environment.name,
      displayName: environment.name,
      runtimeBaseImage: environment.runtimeBaseImage,
      artifactDeployPath: environment.artifactDeployPath,
      dockerfilePath: environment.dockerfilePath,
      selectorLabels: environment.selectorLabels,
      status: 'enabled'
    }]).map((image) => ({ ...image, labelRows: labelsToRows(image.selectorLabels) }))
  };
}

function draftToRuntime(draft: RuntimeEnvironmentDraft): RuntimeEnvironment {
  const images = draft.imageRows.map((image) => ({
    id: image.id,
    name: image.name.trim(),
    displayName: image.displayName.trim() || image.name.trim(),
    runtimeBaseImage: image.runtimeBaseImage.trim(),
    artifactDeployPath: image.artifactDeployPath.trim(),
    dockerfilePath: image.dockerfilePath.trim(),
    selectorLabels: rowsToLabels(image.labelRows),
    status: image.status || 'enabled'
  }));
  const firstImage = images[0];
  return {
    ...draft,
    runtimeBaseImage: firstImage?.runtimeBaseImage || '',
    artifactDeployPath: firstImage?.artifactDeployPath || '',
    dockerfilePath: firstImage?.dockerfilePath || '',
    selectorLabels: firstImage?.selectorLabels || {},
    images
  };
}

function labelsToRows(labels: Record<string, string>): LabelRow[] {
  const rows = Object.entries(labels || {}).map(([key, value]) => ({ key, value }));
  return rows.length ? rows : [{ key: '', value: '' }];
}

function rowsToLabels(rows: LabelRow[]) {
  return Object.fromEntries(rows.map((row) => [row.key.trim(), row.value.trim()] as const).filter(([key, value]) => key && value));
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求处理失败';
}
