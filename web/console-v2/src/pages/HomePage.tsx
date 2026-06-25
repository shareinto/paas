import { ArrowUpRight, ChevronRight, Plus, RefreshCcw } from 'lucide-react';
import { activities, applications, overviewStats, topology } from '../data/mock';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Progress } from '../components/ui/progress';
import { StatusBadge } from '../components/StatusBadge';

export function HomePage() {
  return (
    <div className="space-y-5">
      <section className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <div className="dense-label">控制面总览</div>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">平台工作台</h1>
          <p className="mt-1 text-sm text-muted-foreground">统一查看应用、构建、部署和集群侧 Agent 状态。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline">
            <RefreshCcw className="h-4 w-4" />
            刷新
          </Button>
          <Button>
            <Plus className="h-4 w-4" />
            创建应用
          </Button>
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {overviewStats.map((stat) => (
          <Card key={stat.label}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardDescription>{stat.label}</CardDescription>
              <stat.icon className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="mono text-2xl font-semibold">{stat.value}</div>
              <p className="mt-1 text-xs text-muted-foreground">
                <span className="font-medium text-foreground">{stat.delta}</span> {stat.note}
              </p>
            </CardContent>
          </Card>
        ))}
      </section>

      <section className="grid gap-4 xl:grid-cols-[1.3fr_0.7fr]">
        <Card>
          <CardHeader className="flex flex-row items-start justify-between gap-4">
            <div>
              <CardTitle>关键应用</CardTitle>
              <CardDescription>按最近变更和运行风险排序。</CardDescription>
            </div>
            <Button variant="outline" size="sm">
              查看全部
              <ArrowUpRight className="h-4 w-4" />
            </Button>
          </CardHeader>
          <CardContent>
            <div className="overflow-hidden rounded-md border">
              <table className="w-full text-left text-sm">
                <thead className="bg-muted/70 text-xs text-muted-foreground">
                  <tr>
                    <th className="px-3 py-2 font-medium">应用</th>
                    <th className="px-3 py-2 font-medium">项目</th>
                    <th className="px-3 py-2 font-medium">环境</th>
                    <th className="px-3 py-2 font-medium">版本</th>
                    <th className="px-3 py-2 font-medium">状态</th>
                    <th className="px-3 py-2 font-medium">负责人</th>
                  </tr>
                </thead>
                <tbody>
                  {applications.map((app) => (
                    <tr key={app.name} className="border-t bg-card">
                      <td className="px-3 py-3">
                        <div className="font-medium">{app.displayName}</div>
                        <div className="text-xs text-muted-foreground">{app.name}</div>
                      </td>
                      <td className="px-3 py-3">{app.project}</td>
                      <td className="px-3 py-3">
                        <Badge variant="outline">{app.env}</Badge>
                      </td>
                      <td className="mono px-3 py-3 text-xs">{app.version}</td>
                      <td className="px-3 py-3">
                        <StatusBadge status={app.status} />
                      </td>
                      <td className="px-3 py-3 text-muted-foreground">{app.owner}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>平台链路</CardTitle>
            <CardDescription>底层系统状态只在控制面聚合展示。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {topology.map((node, index) => (
              <div key={node.label} className="flex items-center gap-3 rounded-md border bg-card p-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-md bg-accent text-accent-foreground">
                  <node.icon className="h-4 w-4" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium">{node.label}</div>
                  <div className="text-xs text-muted-foreground">链路节点 {index + 1}</div>
                </div>
                <StatusBadge status={node.status} />
              </div>
            ))}
            <div className="rounded-md border bg-muted/40 p-3">
              <div className="flex items-center justify-between text-sm">
                <span className="font-medium">今日发布容量</span>
                <span className="mono text-muted-foreground">62%</span>
              </div>
              <Progress value={62} className="mt-3" />
            </div>
          </CardContent>
        </Card>
      </section>

      <section className="grid gap-4 xl:grid-cols-[0.8fr_1.2fr]">
        <Card>
          <CardHeader>
            <CardTitle>待办</CardTitle>
            <CardDescription>需要人工确认的控制面动作。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {['生产发布审批 3 个', '失败构建复核 1 个', 'Stage 集群绑定待确认 2 个'].map((item) => (
              <button key={item} className="flex w-full items-center justify-between rounded-md border bg-card p-3 text-left text-sm hover:bg-accent">
                <span>{item}</span>
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              </button>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>近期事件</CardTitle>
            <CardDescription>构建、部署和权限变更审计。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {activities.map((item) => (
              <div key={item.title} className="grid grid-cols-[72px_1fr_auto] items-center gap-3 rounded-md border bg-card p-3">
                <div className="mono text-xs text-muted-foreground">{item.time}</div>
                <div>
                  <div className="text-sm font-medium">{item.title}</div>
                  <div className="text-xs text-muted-foreground">{item.desc}</div>
                </div>
                <StatusBadge status={item.status} />
              </div>
            ))}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
