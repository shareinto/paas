import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Card, Form, Modal, Popconfirm, Select, Space, Table, Tag, Typography, message } from 'antd';
import { useMemo, useState } from 'react';
import { listProjectMembers, listRoles, listTenantMembers, listUsers, removeProjectMember, removeTenantMember, upsertProjectMember, upsertTenantMember, type Member, type Role } from '../api';

type MemberManagementProps = {
  scopeKind: 'tenant' | 'project';
  scopeId: string;
  title?: string;
};

type MemberForm = {
  userId: string;
  roleId: string;
};

export function MemberManagement({ scopeKind, scopeId, title = '成员管理' }: MemberManagementProps) {
  const queryClient = useQueryClient();
  const [form] = Form.useForm<MemberForm>();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Member>();
  const membersQueryKey = [scopeKind, scopeId, 'members'];
  const { data: members = [], isLoading } = useQuery({ queryKey: membersQueryKey, queryFn: () => scopeKind === 'tenant' ? listTenantMembers(scopeId) : listProjectMembers(scopeId), enabled: !!scopeId });
  const { data: users = [] } = useQuery({ queryKey: ['users'], queryFn: listUsers });
  const { data: roles = [] } = useQuery({ queryKey: ['roles'], queryFn: listRoles });
  const roleMap = useMemo(() => new Map(roles.map((role) => [role.id, role])), [roles]);
  const roleOptions = useMemo(() => roles
    .filter((role) => role.suggestedScopes.includes(scopeKind))
    .map((role) => ({ value: role.id, label: role.name })), [roles, scopeKind]);
  const userOptions = useMemo(() => users.map((user) => ({
    value: user.id,
    label: `${user.displayName || user.username}（${user.username}）`,
    disabled: user.disabled
  })), [users]);

  const saveMutation = useMutation({
    mutationFn: (values: MemberForm) => scopeKind === 'tenant'
      ? upsertTenantMember(scopeId, { userId: values.userId, roleId: values.roleId })
      : upsertProjectMember(scopeId, { userId: values.userId, roleId: values.roleId }),
    onSuccess: async () => {
      message.success(scopeKind === 'tenant' ? '租户成员已保存' : '项目成员已保存，可在源码仓库详情页同步 GitLab 权限');
      closeDialog();
      await queryClient.invalidateQueries({ queryKey: membersQueryKey });
    }
  });

  const removeMutation = useMutation({
    mutationFn: (userId: string) => scopeKind === 'tenant' ? removeTenantMember(scopeId, userId) : removeProjectMember(scopeId, userId),
    onSuccess: async () => {
      message.success(scopeKind === 'tenant' ? '租户成员已移除' : '项目成员已移除，可在源码仓库详情页同步 GitLab 权限');
      await queryClient.invalidateQueries({ queryKey: membersQueryKey });
    }
  });

  const openCreate = () => {
    setEditing(undefined);
    form.resetFields();
    setOpen(true);
  };

  const openEdit = (member: Member) => {
    setEditing(member);
    form.setFieldsValue({ userId: member.userId, roleId: member.roleId });
    setOpen(true);
  };

  const closeDialog = () => {
    setOpen(false);
    setEditing(undefined);
    form.resetFields();
  };

  return (
    <>
      <Card
        className="compact-card"
        title={title}
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>添加成员</Button>}
      >
        <Table<Member>
          rowKey="userId"
          loading={isLoading}
          dataSource={members}
          pagination={false}
          columns={[
            {
              title: '成员',
              dataIndex: 'displayName',
              render: (_, record) => (
                <Space direction="vertical" size={0}>
                  <Typography.Text strong>{record.displayName || record.username || record.userId}</Typography.Text>
                  <Typography.Text className="muted">{record.email || '-'}</Typography.Text>
                </Space>
              )
            },
            { title: '账号', dataIndex: 'username' },
            { title: '角色', dataIndex: 'roleId', render: (roleId) => <Tag color="blue">{roleName(roleMap.get(roleId), roleId)}</Tag> },
            { title: '状态', dataIndex: 'disabled', width: 100, render: (disabled) => disabled ? <Tag color="red">已禁用</Tag> : <Tag color="green">启用中</Tag> },
            { title: '更新时间', dataIndex: 'updatedAt' },
            {
              title: '操作',
              key: 'actions',
              width: 180,
              render: (_, record) => (
                <Space>
                  <Button type="link" onClick={() => openEdit(record)}>修改角色</Button>
                  <Popconfirm
                    title="移除成员"
                    description="确认移除该成员？移除后会影响其 PaaS 访问权限。"
                    okText="移除"
                    cancelText="取消"
                    okButtonProps={{ danger: true, loading: removeMutation.isPending }}
                    onConfirm={() => removeMutation.mutate(record.userId)}
                  >
                    <Button danger type="text" icon={<DeleteOutlined />}>移除</Button>
                  </Popconfirm>
                </Space>
              )
            }
          ]}
        />
      </Card>
      <Modal
        title={editing ? '修改成员角色' : '添加成员'}
        open={open}
        onCancel={closeDialog}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        okText="保存"
        cancelText="取消"
      >
        <Form layout="vertical" form={form} onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item label="平台用户" name="userId" rules={[{ required: true, message: '请选择平台用户' }]}>
            <Select showSearch disabled={!!editing} placeholder="选择平台用户" options={userOptions} optionFilterProp="label" />
          </Form.Item>
          <Form.Item label="成员角色" name="roleId" rules={[{ required: true, message: '请选择成员角色' }]}>
            <Select placeholder="选择成员角色" options={roleOptions} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}

function roleName(role: Role | undefined, fallback: string) {
  return role?.name || fallback;
}
