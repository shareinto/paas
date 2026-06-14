import { Space, Typography } from 'antd';
import type { ReactNode } from 'react';

export function PageHeader({ title, extra, breadcrumb }: { title: string; extra?: ReactNode; breadcrumb?: { title: ReactNode }[] }) {
  void breadcrumb;
  return (
    <div className="page-header">
      <Space direction="vertical" size={2}>
        <Typography.Title level={3}>{title}</Typography.Title>
      </Space>
      <div>{extra}</div>
    </div>
  );
}
