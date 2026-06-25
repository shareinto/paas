import { cn } from '../../lib/utils';

type ProgressProps = {
  value: number;
  className?: string;
};

export function Progress({ value, className }: ProgressProps) {
  const width = `${Math.max(0, Math.min(100, value))}%`;
  return (
    <div className={cn('h-2 overflow-hidden rounded-full bg-muted', className)}>
      <div className="h-full rounded-full bg-primary transition-all" style={{ width }} />
    </div>
  );
}
