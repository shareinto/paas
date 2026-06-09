import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Badge, Button, Card, Input, Popconfirm, Select, Space, Table, Tag, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { deleteApplication, listApplications } from '../api';
import { PageHeader } from '../components/PageHeader';

export function ApplicationsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data = [], isLoading } = useQuery({ queryKey: ['apps'], queryFn: listApplications });
  const deleteMutation = useMutation({
    mutationFn: deleteApplication,
    onSuccess: async () => {
      message.success('应用已删除');
      await queryClient.invalidateQueries({ queryKey: ['apps'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '应用删除失败')
  });
  return (
    <>
      <PageHeader title="应用" extra={<Button icon={<PlusOutlined />} type="primary" onClick={() => navigate('/apps/new')}>创建应用</Button>} />
      <div className="toolbar">
        <Input.Search placeholder="搜索应用名称" />
        <Select placeholder="环境状态" options={[{ value: '运行中', label: '运行中' }, { value: '待绑定集群', label: '待绑定集群' }]} />
      </div>
      <Card className="compact-card">
        <Table rowKey="id" loading={isLoading} dataSource={data} onRow={(record) => ({ onClick: () => navigate(`/apps/${record.id}`) })} columns={[
          { title: '应用名称', dataIndex: 'displayName', render: (text, row) => <Space direction="vertical" size={0}><a>{text}</a><span className="muted">{row.name}</span></Space> },
          { title: '项目', dataIndex: 'project' },
          { title: '应用类型', dataIndex: 'type', render: (v) => <Tag color="blue">{v}</Tag> },
          { title: '环境状态', dataIndex: 'envStatus', render: (v) => <Badge status={v === '运行中' ? 'success' : 'warning'} text={v} /> },
          { title: '最近构建', dataIndex: 'build' },
          { title: '最近发布', dataIndex: 'release' },
          { title: '负责人', dataIndex: 'owner' },
          { title: '更新时间', dataIndex: 'updatedAt' },
          {
            title: '操作',
            key: 'actions',
            render: (_, record) => (
              <Popconfirm
                title="删除应用"
                description="确认删除该应用及其环境数据？"
                okText="删除"
                cancelText="取消"
                onConfirm={(event) => {
                  event?.stopPropagation();
                  deleteMutation.mutate(record.id);
                }}
              >
                <Button danger type="text" icon={<DeleteOutlined />} loading={deleteMutation.isPending} onClick={(event) => event.stopPropagation()}>删除</Button>
              </Popconfirm>
            )
          }
        ]} />
      </Card>
    </>
  );
}
