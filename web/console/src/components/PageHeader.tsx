import { Typography } from 'antd';
import type { ReactNode } from 'react';

export function PageHeader({ title, subtitle, extra, breadcrumb }: { title: string; subtitle?: ReactNode; extra?: ReactNode; breadcrumb?: { title: ReactNode }[] }) {
  void breadcrumb;
  return (
    <div className="page-header">
      <div className="page-heading">
        <Typography.Title level={3}>{title}</Typography.Title>
        {subtitle && <Typography.Paragraph className="page-subtitle">{subtitle}</Typography.Paragraph>}
      </div>
      {extra && <div className="page-actions">{extra}</div>}
    </div>
  );
}
