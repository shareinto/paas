import { EditOutlined, MinusCircleOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Card, Form, Input, Modal, Select, Space, Table, Tag, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { listClusters, listTenants, updateCluster, type Cluster, type StringMap } from '../api';
import { PageHeader } from '../components/PageHeader';

type LabelRow = { key?: string; value?: string };
type ClusterForm = { name: string; region: string; labels: LabelRow[] };

export function ClustersPage() {
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ClusterForm>();
  const [tenantId, setTenantId] = useState('');
  const [editing, setEditing] = useState<Cluster>();
  const { data: tenants = [], isLoading: tenantsLoading } = useQuery({ queryKey: ['tenants'], queryFn: listTenants });

  useEffect(() => {
    if (!tenantId && tenants.length > 0) setTenantId(tenants[0].id);
  }, [tenantId, tenants]);

  const currentTenant = useMemo(() => tenants.find((tenant) => tenant.id === tenantId), [tenantId, tenants]);
  const { data: clusters = [], isLoading: clustersLoading } = useQuery({
    queryKey: ['clusters', tenantId],
    queryFn: () => listClusters(tenantId),
    enabled: !!tenantId
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, input }: { id: string; input: { name: string; region: string; labels: StringMap } }) => updateCluster(id, input),
    onSuccess: async () => {
      message.success('集群已更新');
      closeDialog();
      await queryClient.invalidateQueries({ queryKey: ['clusters', tenantId] });
      await queryClient.invalidateQueries({ queryKey: ['cluster-options'] });
    }
  });

  const openEdit = (cluster: Cluster) => {
    setEditing(cluster);
    form.setFieldsValue({ name: cluster.name, region: cluster.region, labels: labelsToRows(cluster.labels) });
  };

  const closeDialog = () => {
    setEditing(undefined);
    form.resetFields();
  };

  const submit = (values: ClusterForm) => {
    if (!editing) return;
    updateMutation.mutate({
      id: editing.id,
      input: {
        name: values.name.trim(),
        region: values.region.trim(),
        labels: rowsToLabels(values.labels)
      }
    });
  };

  return (
    <>
      <PageHeader title="集群管理" subtitle="按租户查看已接入的 Kubernetes 集群，维护展示名称、区域和调度标签。" />
      <div className="toolbar">
        <Select
          aria-label="选择租户"
          loading={tenantsLoading}
          placeholder="请选择租户"
          value={tenantId || undefined}
          onChange={setTenantId}
          options={tenants.map((tenant) => ({ value: tenant.id, label: tenant.displayName }))}
          style={{ width: 240 }}
        />
        {currentTenant && <Typography.Text type="secondary">当前租户：{currentTenant.displayName}</Typography.Text>}
      </div>
      <Card className="compact-card">
        <Table<Cluster>
          rowKey="id"
          loading={clustersLoading || tenantsLoading}
          dataSource={clusters}
          pagination={false}
          locale={{ emptyText: tenantId ? '暂无集群' : '请先选择租户' }}
          columns={[
            { title: '集群名称', dataIndex: 'name', render: (_, record) => <Typography.Text strong>{record.name}</Typography.Text> },
            { title: '区域', dataIndex: 'region', render: (value) => value || '-' },
            { title: '状态', dataIndex: 'status', render: (value) => <Tag color={statusColor(value)}>{statusText(value)}</Tag> },
            { title: '最后心跳', dataIndex: 'lastHeartbeatAt', render: (value) => value || '-' },
            { title: 'Label', dataIndex: 'labels', render: (labels) => <LabelTags labels={labels} /> },
            { title: '更新时间', dataIndex: 'updatedAt', render: (value) => value || '-' },
            {
              title: '操作',
              key: 'actions',
              width: 100,
              render: (_, record) => <Button type="text" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
            }
          ]}
        />
      </Card>
      <Modal
        title="编辑集群"
        open={!!editing}
        onCancel={closeDialog}
        onOk={() => form.submit()}
        okText="保存"
        cancelText="取消"
        confirmLoading={updateMutation.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={submit} initialValues={{ labels: [{ key: '', value: '' }] }}>
          <Form.Item label="集群名称" name="name" rules={[{ required: true, message: '请输入集群名称' }]}>
            <Input placeholder="上海集群" />
          </Form.Item>
          <Form.Item label="区域" name="region" rules={[{ required: true, message: '请输入区域' }]}>
            <Input placeholder="cn-shanghai" />
          </Form.Item>
          <Form.List name="labels">
            {(fields, { add, remove }) => (
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Typography.Text strong>标签</Typography.Text>
                {fields.map(({ key, ...field }) => (
                  <Space key={key} align="baseline" style={{ display: 'flex' }}>
                    <Form.Item {...field} name={[field.name, 'key']} noStyle>
                      <Input aria-label="标签键" placeholder="键" />
                    </Form.Item>
                    <Form.Item {...field} name={[field.name, 'value']} noStyle>
                      <Input aria-label="标签值" placeholder="值" />
                    </Form.Item>
                    <Button aria-label="移除标签" type="text" icon={<MinusCircleOutlined />} onClick={() => remove(field.name)} />
                  </Space>
                ))}
                <Button aria-label="添加标签" icon={<PlusOutlined />} onClick={() => add({ key: '', value: '' })}>添加标签</Button>
              </Space>
            )}
          </Form.List>
        </Form>
      </Modal>
    </>
  );
}

function LabelTags({ labels }: { labels?: StringMap }) {
  const entries = Object.entries(labels || {});
  if (entries.length === 0) return <Typography.Text type="secondary">无</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {entries.map(([key, value]) => <Tag key={key}>{key}={value}</Tag>)}
    </Space>
  );
}

function labelsToRows(labels?: StringMap): LabelRow[] {
  const rows = Object.entries(labels || {}).map(([key, value]) => ({ key, value }));
  return rows.length ? rows : [{ key: '', value: '' }];
}

function rowsToLabels(rows: LabelRow[] = []): StringMap {
  return Object.fromEntries(
    rows
      .map((row) => [row.key?.trim() || '', row.value?.trim() || ''] as const)
      .filter(([key, value]) => key && value)
  );
}

function statusText(status: string) {
  const text: Record<string, string> = {
    ready: '就绪',
    degraded: '异常',
    unreachable: '离线',
    draining: '迁移中',
    disabled: '已禁用'
  };
  return text[status] || status || '-';
}

function statusColor(status: string) {
  const color: Record<string, string> = {
    ready: 'success',
    degraded: 'warning',
    unreachable: 'error',
    draining: 'processing',
    disabled: 'default'
  };
  return color[status] || 'default';
}
