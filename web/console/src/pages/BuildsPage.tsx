import { useQuery } from '@tanstack/react-query';
import { Card, Input, Select, Table, Tag } from 'antd';
import { useNavigate } from 'react-router-dom';
import { listBuilds } from '../api';
import { PageHeader } from '../components/PageHeader';

export function BuildsPage() {
  const navigate = useNavigate();
  const { data = [], isLoading } = useQuery({ queryKey: ['builds'], queryFn: listBuilds });
  return (
    <>
      <PageHeader title="构建" />
      <div className="toolbar">
        <Input.Search placeholder="搜索构建或应用" />
        <Select placeholder="构建状态" options={[{ value: '成功', label: '成功' }, { value: '失败', label: '失败' }]} />
      </div>
      <Card className="compact-card">
        <Table rowKey="id" loading={isLoading} dataSource={data} onRow={(record) => ({ onClick: () => navigate(`/builds/${record.id}`) })} columns={[
          { title: '构建编号', dataIndex: 'id', render: (value) => <a>{value}</a> },
          { title: '应用', dataIndex: 'application' },
          { title: '状态', dataIndex: 'status', render: (value) => <Tag color={value === '成功' ? 'green' : 'red'}>{value}</Tag> },
          { title: '构建引用', dataIndex: 'ref' },
          { title: '提交', dataIndex: 'commit' },
          { title: '开始时间', dataIndex: 'startedAt' },
          { title: '耗时', dataIndex: 'duration' }
        ]} />
      </Card>
    </>
  );
}
