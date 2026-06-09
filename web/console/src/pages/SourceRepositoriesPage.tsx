import { BranchesOutlined, CloudSyncOutlined, CodeOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Card, Form, Input, Modal, Popconfirm, Select, Space, Table, Tag, Typography, message } from 'antd';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { createSourceRepository, deleteSourceRepository, listProjects, listSourceRepositories, type SourceRepository } from '../api';
import { PageHeader } from '../components/PageHeader';

const statusText: Record<string, string> = {
  ready: '可用',
  provisioning: '创建中',
  migrating: '迁移中',
  failed: '失败',
  disabled: '已禁用'
};

export function SourceRepositoriesPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [projectId, setProjectId] = useState<string>();
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState<string>();
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();
  const { data: projects = [] } = useQuery({ queryKey: ['projects'], queryFn: listProjects });
  const { data = [], isLoading } = useQuery({ queryKey: ['source-repositories', projectId], queryFn: () => listSourceRepositories(projectId) });
  const projectOptions = useMemo(() => projects.map((project) => ({ value: project.id, label: project.displayName })), [projects]);
  const filteredData = useMemo(() => data.filter((repo) => {
    const text = `${repo.displayName} ${repo.name} ${repo.httpUrl} ${repo.sshUrl}`.toLowerCase();
    return (!keyword || text.includes(keyword.trim().toLowerCase())) && (!status || repo.status === status);
  }), [data, keyword, status]);
  const createMutation = useMutation({
    mutationFn: createSourceRepository,
    onSuccess: async () => {
      message.success('源码仓库已创建');
      setOpen(false);
      form.resetFields();
      await queryClient.invalidateQueries({ queryKey: ['source-repositories'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '源码仓库创建失败')
  });
  const deleteMutation = useMutation({
    mutationFn: deleteSourceRepository,
    onSuccess: async () => {
      message.success('源码仓库已删除');
      await queryClient.invalidateQueries({ queryKey: ['source-repositories'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '源码仓库删除失败')
  });

  const openCreateModal = () => {
    setOpen(true);
    form.setFieldsValue({ defaultBranch: 'main', projectId: projectId || projectOptions[0]?.value });
  };

  return (
    <>
      <PageHeader
        title="源码仓库"
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreateModal}>创建仓库</Button>}
      />
      <div className="toolbar">
        <Input.Search placeholder="搜索仓库名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} allowClear />
        <Select allowClear placeholder="所属项目" options={projectOptions} value={projectId} onChange={setProjectId} />
        <Select allowClear placeholder="状态" options={[{ value: 'ready', label: '可用' }, { value: 'failed', label: '失败' }, { value: 'migrating', label: '迁移中' }, { value: 'provisioning', label: '创建中' }]} value={status} onChange={setStatus} />
      </div>
      <Card className="compact-card">
        <Table<SourceRepository>
          rowKey="id"
          loading={isLoading}
          dataSource={filteredData}
          onRow={(record) => ({ onClick: () => navigate(`/source-repositories/${record.id}`) })}
          columns={[
            { title: '仓库', dataIndex: 'displayName', render: (_, record) => <Space direction="vertical" size={0}><Typography.Text strong><CodeOutlined /> {record.displayName}</Typography.Text><Typography.Text className="muted">{record.name}</Typography.Text></Space> },
            { title: '所属项目', dataIndex: 'projectName', render: (value) => value || '-' },
            { title: '默认分支', dataIndex: 'defaultBranch', render: (value) => <Tag icon={<BranchesOutlined />}>{value}</Tag> },
            { title: 'Provider', dataIndex: 'gitProvider', render: (value) => <Tag color="blue">{value}</Tag> },
            { title: '状态', dataIndex: 'status', render: (value) => <Tag color={value === 'ready' ? 'green' : value === 'failed' ? 'red' : 'gold'}>{statusText[value] || value}</Tag> },
            { title: '关联应用', dataIndex: 'associatedApplications', render: (value) => `${value || 0} 个` },
            { title: '更新时间', dataIndex: 'updatedAt' },
            {
              title: '操作',
              key: 'actions',
              render: (_, record) => (
                <Popconfirm
                  title="删除源码仓库"
                  description="确认删除该源码仓库？有关联应用时会被拒绝。"
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
          ]}
        />
      </Card>
      <Modal title="创建平台托管源码仓库" open={open} onCancel={() => setOpen(false)} onOk={() => form.submit()} confirmLoading={createMutation.isPending} okText="创建" cancelText="取消">
        <Form layout="vertical" form={form} onFinish={(values) => createMutation.mutate(values)} initialValues={{ defaultBranch: 'main', projectId: projectOptions[0]?.value }}>
          <Form.Item label="所属项目" name="projectId" rules={[{ required: true, message: '请选择所属项目' }]}>
            <Select options={projectOptions} placeholder="选择项目" />
          </Form.Item>
          <Form.Item label="仓库名称" name="name" rules={[{ required: true, message: '请输入仓库名称' }, { pattern: /^[a-z][a-z0-9-]{1,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
            <Input placeholder="order-api" />
          </Form.Item>
          <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}>
            <Input placeholder="订单服务仓库" />
          </Form.Item>
          <Form.Item label="默认分支" name="defaultBranch" rules={[{ required: true, message: '请输入默认分支' }]}>
            <Input prefix={<CloudSyncOutlined />} />
          </Form.Item>
          <Form.Item label="描述" name="description">
            <Input.TextArea rows={3} placeholder="说明仓库用途" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
