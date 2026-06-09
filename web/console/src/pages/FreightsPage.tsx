import { useQuery } from '@tanstack/react-query';
import { Button, Card, Table, Tag } from 'antd';
import { listFreights } from '../api';
import { PageHeader } from '../components/PageHeader';

export function FreightsPage() {
  const { data = [], isLoading } = useQuery({ queryKey: ['freights'], queryFn: () => listFreights() });
  return (
    <>
      <PageHeader title="版本" extra={<Button>创建发布晋级</Button>} />
      <Card className="compact-card">
        <Table rowKey="id" loading={isLoading} dataSource={data} columns={[
          { title: '变更包', dataIndex: 'id' },
          { title: '版本', dataIndex: 'version', render: (value) => <Tag color="blue">{value}</Tag> },
          { title: '镜像', dataIndex: 'image' },
          { title: '镜像 digest', dataIndex: 'digest' },
          { title: '提交', dataIndex: 'commit' },
          { title: '创建时间', dataIndex: 'createdAt' }
        ]} />
      </Card>
    </>
  );
}
