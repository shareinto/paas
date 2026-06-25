import { CheckCircle2, CircleDashed, Loader2, TriangleAlert, XCircle } from 'lucide-react';
import { Badge } from './ui/badge';
import type { Status } from '../data/mock';

const statusMap = {
  healthy: { label: '健康', variant: 'success' as const, icon: CheckCircle2 },
  warning: { label: '需关注', variant: 'warning' as const, icon: TriangleAlert },
  danger: { label: '失败', variant: 'destructive' as const, icon: XCircle },
  running: { label: '进行中', variant: 'secondary' as const, icon: Loader2 },
  pending: { label: '等待中', variant: 'muted' as const, icon: CircleDashed }
};

export function StatusBadge({ status }: { status: Status }) {
  const item = statusMap[status];
  const Icon = item.icon;
  return (
    <Badge variant={item.variant} className="gap-1">
      <Icon className={status === 'running' ? 'h-3 w-3 animate-spin' : 'h-3 w-3'} />
      {item.label}
    </Badge>
  );
}
