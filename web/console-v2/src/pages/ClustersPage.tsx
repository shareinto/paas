import { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, Edit3, KeyRound, MoreHorizontal, PauseCircle, RefreshCcw, Save, Server, ShieldCheck } from 'lucide-react';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { usePlatformSelection } from '../contexts/PlatformSelectionContext';
import { loadClusters, updateCluster, updateClusterAction, type Cluster, type ClusterStatus } from '../api/clusters';
import { cn } from '../lib/utils';

type LabelRow = { key: string; value: string };

export function ClustersPage() {
  const { currentTenant } = usePlatformSelection();
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [source, setSource] = useState<'api' | 'mock'>('mock');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<Cluster | null>(null);
  const [labelRows, setLabelRows] = useState<LabelRow[]>([]);
  const [actionMessage, setActionMessage] = useState('');

  const stats = useMemo(() => {
    const ready = clusters.filter((cluster) => cluster.status === 'ready').length;
    const degraded = clusters.filter((cluster) => cluster.status === 'degraded' || cluster.status === 'unreachable').length;
    const disabled = clusters.filter((cluster) => cluster.status === 'disabled').length;
    return { ready, degraded, disabled };
  }, [clusters]);

  useEffect(() => {
    void reload();
  }, [currentTenant.id]);

  async function reload() {
    setLoading(true);
    setError('');
    const result = await loadClusters(currentTenant.id);
    setClusters(result.clusters);
    setSource(result.source);
    setError(result.error || '');
    setLoading(false);
  }

  function openEdit(cluster: Cluster) {
    setEditing(cluster);
    setLabelRows(labelsToRows(cluster.labels));
    setActionMessage('');
  }

  async function saveEdit() {
    if (!editing) return;
    const input = { name: editing.name.trim(), region: editing.region.trim(), labels: rowsToLabels(labelRows) };
    await updateCluster(editing.id, input);
    setClusters((current) => current.map((cluster) => cluster.id === editing.id ? { ...cluster, ...input } : cluster));
    setEditing(null);
    setActionMessage('集群信息已保存');
  }

  async function runAction(cluster: Cluster, action: 'disable' | 'drain' | 'rotate-token') {
    const result = await updateClusterAction(cluster.id, action);
    if (action === 'disable') {
      setClusters((current) => current.map((item) => item.id === cluster.id ? { ...item, status: 'disabled' } : item));
      setActionMessage(`${cluster.name} 已禁用`);
    } else if (action === 'drain') {
      setClusters((current) => current.map((item) => item.id === cluster.id ? { ...item, status: 'draining' } : item));
      setActionMessage(`${cluster.name} 已进入排空状态`);
    } else {
      const token = extractToken(result);
      setActionMessage(token ? `新 Agent Token：${token}` : `${cluster.name} 已轮转 Agent Token`);
    }
  }

  return (
    <div className="flex min-h-[calc(100vh-96px)] flex-col gap-4">
      <section className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="dense-label">租户级资源</div>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">集群管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">按当前租户查看已接入的 Kubernetes 集群，维护区域、标签和 Agent 状态。</p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline">{source === 'api' ? '后端' : 'Mock'}</Badge>
          <Button variant="outline" onClick={reload} disabled={loading}>
            <RefreshCcw className={cn('h-4 w-4', loading && 'animate-spin')} />
            刷新
          </Button>
        </div>
      </section>

      {(error || actionMessage) && (
        <div className="rounded-md border bg-muted/35 px-3 py-2 text-sm text-muted-foreground">
          {actionMessage || error}
        </div>
      )}

      <section className="grid gap-3 lg:grid-cols-4">
        <ClusterMetric label="当前租户" value={currentTenant.name} icon={Server} />
        <ClusterMetric label="就绪集群" value={`${stats.ready} 个`} icon={ShieldCheck} tone="success" />
        <ClusterMetric label="异常/离线" value={`${stats.degraded} 个`} icon={AlertTriangle} tone="warning" />
        <ClusterMetric label="已禁用" value={`${stats.disabled} 个`} icon={PauseCircle} />
      </section>

      <Card className="min-h-0 flex-1">
        <CardHeader className="border-b">
          <CardTitle>集群列表</CardTitle>
          <CardDescription>平台控制面只管理集群接入状态，不直接暴露 kubeconfig。</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[960px] border-collapse text-sm">
              <thead className="bg-muted/45 text-left text-xs font-semibold text-muted-foreground">
                <tr>
                  <th className="px-4 py-3">集群</th>
                  <th className="px-4 py-3">区域</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">版本</th>
                  <th className="px-4 py-3">最后心跳</th>
                  <th className="px-4 py-3">标签</th>
                  <th className="px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {clusters.map((cluster) => (
                  <tr key={cluster.id} className="border-t hover:bg-muted/25">
                    <td className="px-4 py-3">
                      <div className="font-medium text-foreground">{cluster.name}</div>
                      <div className="mono text-xs text-muted-foreground">{cluster.id}</div>
                    </td>
                    <td className="px-4 py-3">{cluster.region || '-'}</td>
                    <td className="px-4 py-3"><ClusterStatusBadge status={cluster.status} /></td>
                    <td className="mono px-4 py-3">{cluster.serverVersion || '-'}</td>
                    <td className="px-4 py-3 text-muted-foreground">{cluster.lastHeartbeatAt || '-'}</td>
                    <td className="px-4 py-3"><LabelTags labels={cluster.labels} /></td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1.5">
                        <Button variant="outline" size="sm" onClick={() => openEdit(cluster)}>
                          <Edit3 className="h-3.5 w-3.5" />
                          编辑
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => runAction(cluster, 'drain')} disabled={cluster.status === 'disabled'}>
                          排空
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => runAction(cluster, 'rotate-token')}>
                          <KeyRound className="h-3.5 w-3.5" />
                          轮转
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => runAction(cluster, 'disable')} disabled={cluster.status === 'disabled'}>
                          禁用
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
                {!loading && clusters.length === 0 && (
                  <tr>
                    <td colSpan={7} className="px-4 py-14 text-center text-muted-foreground">当前租户暂无集群</td>
                  </tr>
                )}
                {loading && (
                  <tr>
                    <td colSpan={7} className="px-4 py-14 text-center text-muted-foreground">正在加载集群...</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {editing && (
        <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/35 p-4">
          <div className="mt-6 flex h-[min(720px,calc(100vh-48px))] w-[720px] flex-col overflow-hidden rounded-lg border bg-card shadow-2xl">
            <div className="flex items-start justify-between border-b px-5 py-4">
              <div>
                <h2 className="text-lg font-semibold">编辑集群</h2>
                <p className="text-sm text-muted-foreground">维护展示名称、区域和调度标签。</p>
              </div>
              <Button variant="ghost" size="icon" onClick={() => setEditing(null)}>
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </div>
            <div className="flex-1 space-y-4 overflow-y-auto p-5">
              <InlineField label="集群名称">
                <Input value={editing.name} onChange={(event) => setEditing({ ...editing, name: event.target.value })} />
              </InlineField>
              <InlineField label="区域">
                <Input value={editing.region} onChange={(event) => setEditing({ ...editing, region: event.target.value })} />
              </InlineField>
              <div className="space-y-2">
                <div className="text-sm font-medium">标签</div>
                {labelRows.map((row, index) => (
                  <div key={index} className="grid grid-cols-[1fr_1fr_auto] gap-2">
                    <Input value={row.key} placeholder="键" onChange={(event) => setLabelRows((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, key: event.target.value } : item))} />
                    <Input value={row.value} placeholder="值" onChange={(event) => setLabelRows((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, value: event.target.value } : item))} />
                    <Button variant="outline" onClick={() => setLabelRows((current) => current.filter((_, itemIndex) => itemIndex !== index))}>删除</Button>
                  </div>
                ))}
                <Button variant="outline" onClick={() => setLabelRows((current) => [...current, { key: '', value: '' }])}>添加标签</Button>
              </div>
            </div>
            <div className="flex justify-end gap-2 border-t px-5 py-4">
              <Button variant="outline" onClick={() => setEditing(null)}>取消</Button>
              <Button onClick={saveEdit}>
                <Save className="h-4 w-4" />
                保存
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function ClusterMetric({ label, value, icon: Icon, tone }: { label: string; value: string; icon: typeof Server; tone?: 'success' | 'warning' }) {
  return (
    <Card>
      <CardContent className="flex items-center justify-between p-4">
        <div>
          <div className="text-sm text-muted-foreground">{label}</div>
          <div className="mt-1 text-xl font-semibold">{value}</div>
        </div>
        <div className={cn('flex h-10 w-10 items-center justify-center rounded-md border bg-muted text-muted-foreground', tone === 'success' && 'text-success', tone === 'warning' && 'text-warning')}>
          <Icon className="h-5 w-5" />
        </div>
      </CardContent>
    </Card>
  );
}

function ClusterStatusBadge({ status }: { status: ClusterStatus }) {
  const label: Record<ClusterStatus, string> = {
    ready: '就绪',
    degraded: '异常',
    unreachable: '离线',
    draining: '排空中',
    disabled: '已禁用'
  };
  const variant = status === 'ready' ? 'success' : status === 'degraded' || status === 'unreachable' ? 'warning' : 'muted';
  return <Badge variant={variant}>{label[status] || status}</Badge>;
}

function LabelTags({ labels }: { labels: Record<string, string> }) {
  const entries = Object.entries(labels || {});
  if (!entries.length) return <span className="text-muted-foreground">无</span>;
  return (
    <div className="flex flex-wrap gap-1">
      {entries.map(([key, value]) => (
        <span key={key} className="rounded border bg-muted/45 px-1.5 py-0.5 text-xs text-muted-foreground">{key}={value}</span>
      ))}
    </div>
  );
}

function InlineField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="grid grid-cols-[96px_minmax(0,1fr)] items-center gap-3 text-sm">
      <span className="font-medium text-muted-foreground">{label}</span>
      {children}
    </label>
  );
}

function labelsToRows(labels: Record<string, string>): LabelRow[] {
  const rows = Object.entries(labels || {}).map(([key, value]) => ({ key, value }));
  return rows.length ? rows : [{ key: '', value: '' }];
}

function rowsToLabels(rows: LabelRow[]) {
  return Object.fromEntries(rows.map((row) => [row.key.trim(), row.value.trim()] as const).filter(([key, value]) => key && value));
}

function extractToken(result: unknown) {
  if (result && typeof result === 'object' && 'agent_token' in result) {
    return String((result as { agent_token?: string }).agent_token || '');
  }
  return '';
}
