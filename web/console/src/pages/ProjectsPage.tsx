import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Card, Form, Input, Modal, Popconfirm, Select, Space, Table, Tag, Typography, message } from 'antd';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { createProject, deleteProject, listProjects, listTenants, type Project } from '../api';
import { PageHeader } from '../components/PageHeader';

export function ProjectsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm();
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [tenantId, setTenantId] = useState<string>();
  const { data = [], isLoading } = useQuery({ queryKey: ['projects'], queryFn: listProjects });
  const { data: tenants = [] } = useQuery({ queryKey: ['tenants'], queryFn: listTenants });
  const tenantOptions = useMemo(() => tenants.map((tenant) => ({ value: tenant.id, label: tenant.displayName || tenant.name })), [tenants]);
  const filteredProjects = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return data.filter((project) => {
      const matchTenant = !tenantId || project.tenantId === tenantId;
      const matchKeyword = !normalizedKeyword || [project.displayName, project.name, project.description].some((value) => (value || '').toLowerCase().includes(normalizedKeyword));
      return matchTenant && matchKeyword;
    });
  }, [data, keyword, tenantId]);
  const createMutation = useMutation({
    mutationFn: createProject,
    onSuccess: async () => {
      message.success('项目已创建');
      setOpen(false);
      form.resetFields();
      await queryClient.invalidateQueries({ queryKey: ['projects'] });
    }
  });
  const deleteMutation = useMutation({
    mutationFn: deleteProject,
    onSuccess: async () => {
      message.success('项目已删除');
      await queryClient.invalidateQueries({ queryKey: ['projects'] });
    }
  });

  return (
    <>
      <PageHeader
        title="项目"
        subtitle="按租户组织应用和源码仓库，作为交付工作的入口。"
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => {
          if (!form.getFieldValue('tenantId') && tenantOptions[0]) {
            form.setFieldValue('tenantId', tenantOptions[0].value);
          }
          setOpen(true);
        }}>创建项目</Button>}
      />
      <div className="toolbar">
        <Input.Search placeholder="搜索项目名称或标识" allowClear onSearch={setKeyword} onChange={(event) => setKeyword(event.target.value)} />
        <Select allowClear placeholder="选择租户" options={tenantOptions} value={tenantId} onChange={setTenantId} />
      </div>
      <Card className="compact-card">
        <Table<Project> rowKey="id" loading={isLoading} dataSource={filteredProjects} pagination={false} columns={[
          { title: '项目名称', dataIndex: 'displayName', render: (_, record) => <Space direction="vertical" size={0}><a onClick={() => navigate(`/projects/${record.id}`)}>{record.displayName}</a>{record.description && <Typography.Text className="muted">{record.description}</Typography.Text>}</Space> },
          { title: '标识', dataIndex: 'name' },
          { title: '租户', dataIndex: 'tenant', render: (v) => <Tag>{v}</Tag> },
          { title: '负责人', dataIndex: 'owner' },
          { title: '更新时间', dataIndex: 'updatedAt' },
          {
            title: '操作',
            key: 'actions',
            width: 120,
            render: (_, record) => (
              <Popconfirm
                title="删除项目"
                description="确认删除该项目？有关联应用或源码仓库时会被拒绝。"
                okText="删除"
                cancelText="取消"
                okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
                onConfirm={() => deleteMutation.mutate(record.id)}
              >
                <Button danger type="text" icon={<DeleteOutlined />}>删除</Button>
              </Popconfirm>
            )
          }
        ]} />
      </Card>
      <Modal title="创建项目" open={open} onCancel={() => setOpen(false)} onOk={() => form.submit()} confirmLoading={createMutation.isPending} okText="创建" cancelText="取消">
        <Form layout="vertical" form={form} onFinish={(values) => createMutation.mutate(values)} initialValues={{ tenantId: tenantOptions[0]?.value }}>
          <Form.Item label="所属租户" name="tenantId" rules={[{ required: true, message: '请选择所属租户' }]}>
            <Select options={tenantOptions} placeholder="选择租户" />
          </Form.Item>
          <Form.Item label="项目标识" name="name" rules={[{ required: true, message: '请输入项目标识' }, { pattern: /^[a-z][a-z0-9-]{1,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
            <Input placeholder="order" />
          </Form.Item>
          <Form.Item label="项目名称" name="displayName" rules={[{ required: true, message: '请输入项目名称' }]}>
            <Input placeholder="订单平台" />
          </Form.Item>
          <Form.Item label="描述" name="description">
            <Input.TextArea rows={3} placeholder="说明项目用途" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
