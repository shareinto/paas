import { Breadcrumb, Space, Typography } from 'antd';
import type { ReactNode } from 'react';

export function PageHeader({ title, extra, breadcrumb }: { title: string; extra?: ReactNode; breadcrumb?: { title: ReactNode }[] }) {
  return (
    <div className="page-header">
      <Space direction="vertical" size={2}>
        {breadcrumb && breadcrumb.length > 0 && <Breadcrumb items={breadcrumb} />}
        <Typography.Title level={3}>{title}</Typography.Title>
      </Space>
      <div>{extra}</div>
    </div>
  );
}
