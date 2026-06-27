import { useEffect, useMemo, useState } from 'react';
import { CheckCircle2, CirclePlay, GitBranch, Play, Search, XCircle } from 'lucide-react';
import { pipelines, type Status } from '../data/mock';
import { StatusBadge } from '../components/StatusBadge';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Progress } from '../components/ui/progress';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { usePlatformSelection } from '../contexts/PlatformSelectionContext';
import { cn } from '../lib/utils';

const filters: Array<{ label: string; value: 'all' | Status }> = [
  { label: '全部', value: 'all' },
  { label: '进行中', value: 'running' },
  { label: '成功', value: 'healthy' },
  { label: '失败', value: 'danger' }
];

export function PipelinePage() {
  const { currentTenant, currentProject, currentApplication } = usePlatformSelection();
  const [activeWorkloadId, setActiveWorkloadId] = useState(currentApplication.workloads[0]?.id || '');
  const [activeId, setActiveId] = useState('');
  const [filter, setFilter] = useState<'all' | Status>('all');
  const [query, setQuery] = useState('');
  const [sourceRef, setSourceRef] = useState('main');
  const [hasTriggered, setHasTriggered] = useState(false);

  useEffect(() => {
    setActiveWorkloadId(currentApplication.workloads[0]?.id || '');
    setActiveId('');
    setFilter('all');
    setQuery('');
    setSourceRef('main');
    setHasTriggered(false);
  }, [currentApplication.id, currentApplication.workloads]);

  const applicationPipelines = useMemo(() => {
    return pipelines.filter((pipeline) => pipeline.applicationId === currentApplication.id);
  }, [currentApplication.id]);

  const currentWorkload =
    currentApplication.workloads.find((workload) => workload.id === activeWorkloadId) || currentApplication.workloads[0];

  const workloadPipelines = useMemo(() => {
    if (!currentWorkload) return [];
    return applicationPipelines.filter((pipeline) => pipeline.workloadId === currentWorkload.id);
  }, [applicationPipelines, currentWorkload]);

  const visiblePipelines = useMemo(() => {
    return workloadPipelines.filter((pipeline) => {
      const matchStatus = filter === 'all' || pipeline.status === filter;
      const matchQuery = `${pipeline.name}${pipeline.app}${pipeline.workloadName}${pipeline.branch}`.toLowerCase().includes(query.toLowerCase());
      return matchStatus && matchQuery;
    });
  }, [filter, query, workloadPipelines]);

  const activePipeline = visiblePipelines.find((item) => item.id === activeId) || visiblePipelines[0];

  return (
    <div className="space-y-5">
      <section className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <div className="dense-label">构建与制品</div>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">{currentApplication.name}流水线</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            一个应用包含多个 Workload；每个 Workload 对应一条流水线，流水线产出镜像并进入 Freight 选择。
          </p>
          <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground">
            <Badge variant="outline">租户：{currentTenant.name}</Badge>
            <Badge variant="outline">项目：{currentProject.name}</Badge>
            <Badge variant="outline">应用：{currentApplication.code}</Badge>
          </div>
        </div>
        <Button onClick={() => setHasTriggered(true)} disabled={!activePipeline}>
          <Play className="h-4 w-4" />
          触发构建
        </Button>
      </section>

      <section className="grid gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="xl:min-h-[720px]">
          <CardHeader>
            <CardTitle>Workload 与流水线</CardTitle>
            <CardDescription>先选择 Workload，再查看对应流水线。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              {currentApplication.workloads.map((workload) => (
                <button
                  key={workload.id}
                  onClick={() => {
                    setActiveWorkloadId(workload.id);
                    setActiveId('');
                    setHasTriggered(false);
                  }}
                  className={cn(
                    'w-full rounded-md border bg-card p-3 text-left transition-colors hover:bg-accent',
                    currentWorkload?.id === workload.id && 'border-primary bg-accent'
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="font-medium">{workload.name}</div>
                    <Badge variant="outline">{workloadKindLabel(workload.kind)}</Badge>
                  </div>
                  <div className="mono mt-1 text-xs text-muted-foreground">{workload.pipelineId}</div>
                </button>
              ))}
            </div>
            <div className="h-px bg-border" />
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-9" placeholder="搜索流水线或分支" />
            </div>
            <div className="flex flex-wrap gap-2">
              {filters.map((item) => (
                <Button
                  key={item.value}
                  variant={filter === item.value ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setFilter(item.value)}
                >
                  {item.label}
                </Button>
              ))}
            </div>
            <div className="space-y-2">
              {visiblePipelines.map((pipeline) => (
                <button
                  key={pipeline.id}
                  onClick={() => setActiveId(pipeline.id)}
                  className={cn(
                    'w-full rounded-md border bg-card p-3 text-left transition-colors hover:bg-accent',
                    activePipeline.id === pipeline.id && 'border-primary bg-accent'
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="font-medium">{pipeline.name}</div>
                    <StatusBadge status={pipeline.status} />
                  </div>
                  <div className="mt-1 text-xs text-muted-foreground">{pipeline.app}</div>
                  <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
                    <span className="inline-flex items-center gap-1">
                      <GitBranch className="h-3 w-3" />
                      {pipeline.branch}
                    </span>
                    <span className="mono">{pipeline.commit}</span>
                  </div>
                </button>
              ))}
              {visiblePipelines.length === 0 && (
                <div className="rounded-md border border-dashed bg-muted/40 p-4 text-sm text-muted-foreground">
                  当前 Workload 暂无匹配流水线。真实接入后会按 applicationId 与 workloadId 查询运行记录。
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {activePipeline ? (
        <div className="space-y-4">
          <Card>
            <CardHeader className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div>
                <CardTitle>{activePipeline.name}</CardTitle>
                <CardDescription>
                  {activePipeline.workloadName} · {activePipeline.workloadKind} · {activePipeline.branch} · {activePipeline.commit}
                </CardDescription>
              </div>
              <div className="flex gap-2">
                <StatusBadge status={activePipeline.status} />
                <Badge variant="outline" className="mono">{activePipeline.duration}</Badge>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <div className="mb-2 flex items-center justify-between text-sm">
                  <span className="font-medium">执行进度</span>
                  <span className="mono text-muted-foreground">{activePipeline.progress}%</span>
                </div>
                <Progress value={activePipeline.progress} />
              </div>
              <div className="grid gap-3 md:grid-cols-4">
                {activePipeline.stages.map((stage, index) => (
                  <div key={stage.name} className="rounded-md border bg-card p-3">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex h-7 w-7 items-center justify-center rounded-full bg-muted text-xs font-semibold">{index + 1}</div>
                      <StageIcon status={stage.status} />
                    </div>
                    <div className="mt-3 text-sm font-medium">{stage.name}</div>
                    <div className="mt-1 mono text-xs text-muted-foreground">{stage.duration}</div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          <Tabs defaultValue="log">
            <TabsList>
              <TabsTrigger value="log">运行日志</TabsTrigger>
              <TabsTrigger value="config">构建配置</TabsTrigger>
              <TabsTrigger value="artifact">制品</TabsTrigger>
            </TabsList>
            <TabsContent value="log">
              <Card>
                <CardContent className="pt-4">
                  <pre className="max-h-[320px] overflow-auto rounded-md bg-slate-950 p-4 text-xs leading-6 text-slate-100">
{`[10:01:12] checkout ${activePipeline.branch} @ ${activePipeline.commit}
[10:01:24] restore build cache
[10:02:03] run tests: 128 passed, 0 skipped
[10:02:58] docker buildx build --platform linux/amd64,linux/arm64
[10:03:30] push image registry.local/${activePipeline.app}:20260622.4
[10:03:42] waiting for PaaS callback ...`}
                  </pre>
                </CardContent>
              </Card>
            </TabsContent>
            <TabsContent value="config">
              <Card>
                <CardContent className="grid gap-3 pt-4 md:grid-cols-2">
                  <ConfigRow label="代码源" value="platform-monorepo" />
                  <ConfigRow label="构建路径" value={`services/${activePipeline.workloadId}`} />
                  <ConfigRow label="构建镜像" value="gradle7-jdk11" />
                  <ConfigRow label="产物路径" value="$PAAS_ARTIFACT_OUTPUT/app.jar" />
                </CardContent>
              </Card>
            </TabsContent>
            <TabsContent value="artifact">
              <Card>
                <CardContent className="pt-4">
                  <div className="rounded-md border">
                    {[`registry.local/${activePipeline.workloadId}:20260622.4`, 'sha256:4f9a2c...91de', 'freight-20260622.4'].map((item, index) => (
                      <div key={item} className="flex items-center justify-between border-b px-3 py-3 last:border-b-0">
                        <span className="mono text-sm">{item}</span>
                        <Badge variant={index === 2 ? 'success' : 'outline'}>{index === 2 ? '已生成发布单' : '已锁定'}</Badge>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>

          <Card>
            <CardHeader>
              <CardTitle>手动触发</CardTitle>
              <CardDescription>输入分支或标签，构建完成后生成可晋级的 Freight。</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 md:grid-cols-[1fr_auto]">
              <Input value={sourceRef} onChange={(event) => setSourceRef(event.target.value)} aria-label="源码引用" />
              <Button onClick={() => setHasTriggered(true)}>
                <CirclePlay className="h-4 w-4" />
                运行
              </Button>
              {hasTriggered && (
                <div className="rounded-md border border-primary/30 bg-accent px-3 py-2 text-sm text-accent-foreground md:col-span-2">
                  已提交构建请求：{activePipeline.name} · {sourceRef}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
        ) : (
          <Card className="min-h-[520px]">
            <CardContent className="flex h-full min-h-[520px] flex-col items-center justify-center text-center">
              <CirclePlay className="h-10 w-10 text-muted-foreground" />
              <h2 className="mt-4 text-lg font-semibold">当前 Workload 暂无流水线数据</h2>
              <p className="mt-2 max-w-xl text-sm text-muted-foreground">
                已选择 {currentApplication.name} / {currentWorkload?.name || '未选择 Workload'}。真实接入后这里会展示该 Workload 的运行记录、构建产物和镜像版本。
              </p>
            </CardContent>
          </Card>
        )}
      </section>
    </div>
  );
}

function workloadKindLabel(kind?: string) {
  return String(kind || '').toLowerCase() === 'statefulset' ? '有状态' : '无状态';
}

function StageIcon({ status }: { status: Status }) {
  if (status === 'healthy') return <CheckCircle2 className="h-5 w-5 text-success" />;
  if (status === 'danger') return <XCircle className="h-5 w-5 text-destructive" />;
  if (status === 'running') return <CirclePlay className="h-5 w-5 text-primary" />;
  return <span className="h-5 w-5 rounded-full border border-muted-foreground/40" />;
}

function ConfigRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border bg-card p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 mono text-sm">{value}</div>
    </div>
  );
}
