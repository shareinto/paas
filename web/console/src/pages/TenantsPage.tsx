import { EditOutlined, PlusOutlined, TeamOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Card, Drawer, Form, Input, Modal, Space, Table, Typography, message } from 'antd';
import { useMemo, useState } from 'react';
import { createTenant, listTenants, updateTenant, type Tenant } from '../api';
import { MemberManagement } from '../components/MemberManagement';
import { PageHeader } from '../components/PageHeader';

type TenantForm = {
  name: string;
  displayName: string;
  description?: string;
};

export function TenantsPage() {
  const queryClient = useQueryClient();
  const [form] = Form.useForm<TenantForm>();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Tenant>();
  const [memberTenant, setMemberTenant] = useState<Tenant>();
  const [keyword, setKeyword] = useState('');
  const { data = [], isLoading } = useQuery({ queryKey: ['tenants'], queryFn: listTenants });
  const filteredTenants = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return data.filter((tenant) => !normalizedKeyword || [tenant.displayName, tenant.name, tenant.description].some((value) => (value || '').toLowerCase().includes(normalizedKeyword)));
  }, [data, keyword]);

  const createMutation = useMutation({
    mutationFn: createTenant,
    onSuccess: async () => {
      message.success('租户已创建');
      closeDialog();
      await queryClient.invalidateQueries({ queryKey: ['tenants'] });
      await queryClient.invalidateQueries({ queryKey: ['projects'] });
    }
  });
  const updateMutation = useMutation({
    mutationFn: ({ id, input }: { id: string; input: { displayName: string; description?: string } }) => updateTenant(id, input),
    onSuccess: async () => {
      message.success('租户已更新');
      closeDialog();
      await queryClient.invalidateQueries({ queryKey: ['tenants'] });
      await queryClient.invalidateQueries({ queryKey: ['projects'] });
    }
  });

  const submit = (values: TenantForm) => {
    if (editing) {
      updateMutation.mutate({ id: editing.id, input: { displayName: values.displayName, description: values.description } });
      return;
    }
    createMutation.mutate(values);
  };

  const openCreate = () => {
    setEditing(undefined);
    form.resetFields();
    setOpen(true);
  };

  const openEdit = (tenant: Tenant) => {
    setEditing(tenant);
    form.setFieldsValue({ name: tenant.name, displayName: tenant.displayName, description: tenant.description });
    setOpen(true);
  };

  const closeDialog = () => {
    setOpen(false);
    setEditing(undefined);
    form.resetFields();
  };

  return (
    <>
      <PageHeader title="租户管理" subtitle="维护平台内组织边界，关联项目、模板和权限配置。" extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>创建租户</Button>} />
      <div className="toolbar">
        <Input.Search placeholder="搜索租户名称或标识" allowClear onSearch={setKeyword} onChange={(event) => setKeyword(event.target.value)} />
      </div>
      <Card className="compact-card">
        <Table<Tenant> rowKey="id" loading={isLoading} dataSource={filteredTenants} pagination={false} columns={[
          { title: '租户名称', dataIndex: 'displayName', render: (_, record) => <Space direction="vertical" size={0}><Typography.Text strong>{record.displayName}</Typography.Text>{record.description && <Typography.Text className="muted">{record.description}</Typography.Text>}</Space> },
          { title: '标识', dataIndex: 'name' },
          { title: '更新时间', dataIndex: 'updatedAt' },
          {
            title: '操作',
            key: 'actions',
            width: 180,
            render: (_, record) => (
              <Space>
                <Button type="text" icon={<TeamOutlined />} onClick={() => setMemberTenant(record)}>成员</Button>
                <Button type="text" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
              </Space>
            )
          }
        ]} />
      </Card>
      <Drawer title={`${memberTenant?.displayName || '租户'}成员`} open={!!memberTenant} onClose={() => setMemberTenant(undefined)} width={860} destroyOnClose>
        {memberTenant && <MemberManagement scopeKind="tenant" scopeId={memberTenant.id} title="租户成员" />}
      </Drawer>
      <Modal title={editing ? '编辑租户' : '创建租户'} open={open} onCancel={closeDialog} onOk={() => form.submit()} confirmLoading={createMutation.isPending || updateMutation.isPending} okText={editing ? '保存' : '创建'} cancelText="取消">
        <Form layout="vertical" form={form} onFinish={submit}>
          <Form.Item label="租户标识" name="name" rules={[{ required: true, message: '请输入租户标识' }, { pattern: /^[a-z][a-z0-9-]{1,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
            <Input placeholder="rnd" disabled={!!editing} />
          </Form.Item>
          <Form.Item label="租户名称" name="displayName" rules={[{ required: true, message: '请输入租户名称' }]}>
            <Input placeholder="研发中心" />
          </Form.Item>
          <Form.Item label="描述" name="description">
            <Input.TextArea rows={3} placeholder="说明租户用途" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
