import { useQuery } from '@tanstack/react-query';
import { Button, Card, Space, Table, Tag, Typography } from 'antd';
import { listFreights, type Freight } from '../api';
import { PageHeader } from '../components/PageHeader';

export function FreightsPage() {
  const { data = [], isLoading } = useQuery({ queryKey: ['freights'], queryFn: () => listFreights() });
  return (
    <>
      <PageHeader title="版本" extra={<Button>创建发布晋级</Button>} />
      <Card className="compact-card">
        <Table rowKey="id" loading={isLoading} dataSource={data} columns={[
          { title: 'Freight', dataIndex: 'id' },
          { title: '版本', dataIndex: 'version', render: (value) => <Tag color="blue">{value}</Tag> },
          { title: '覆盖 Workload', key: 'workloads', render: (_: unknown, item: Freight) => item.items?.length ? `${item.items.length} 个 Workload` : item.image },
          {
            title: '镜像摘要',
            key: 'images',
            render: (_: unknown, item: Freight) => item.items?.length ? (
              <Space direction="vertical" size={2}>
                {item.items.map((freightItem) => (
                  <Typography.Text key={freightItem.id} ellipsis>{freightItem.workloadDisplayName}：{freightItem.image}</Typography.Text>
                ))}
              </Space>
            ) : <Typography.Text type="secondary">暂无镜像摘要</Typography.Text>
          },
          { title: '创建时间', dataIndex: 'createdAt' }
        ]} />
      </Card>
    </>
  );
}
