import { useQuery } from '@tanstack/react-query';
import { Card, DatePicker, Input, Select, Table, Tag } from 'antd';
import { listAuditLogs } from '../api';
import { PageHeader } from '../components/PageHeader';

export function AuditPage() {
  const { data = [], isLoading } = useQuery({ queryKey: ['audit'], queryFn: listAuditLogs });
  return (
    <>
      <PageHeader title="审计日志" />
      <div className="toolbar">
        <Input.Search placeholder="搜索资源或摘要" />
        <Select placeholder="动作" options={[{ value: 'promotion.approve', label: '审批发布' }, { value: 'cluster.disable', label: '禁用集群' }]} />
        <DatePicker.RangePicker />
      </div>
      <Card className="compact-card">
        <Table rowKey="id" loading={isLoading} dataSource={data} columns={[
          { title: '时间', dataIndex: 'time' },
          { title: '操作者', dataIndex: 'actor' },
          { title: '动作', dataIndex: 'action' },
          { title: '资源', dataIndex: 'resource' },
          { title: '结果', dataIndex: 'result', render: (v) => <Tag color="green">{v}</Tag> },
          { title: '摘要', dataIndex: 'summary' }
        ]} />
      </Card>
    </>
  );
}
