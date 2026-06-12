import { DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Checkbox, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Statistic, Switch, Tag, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { deleteDeliveryFlowTemplateStage, getDeliveryFlowTemplate, listClusterOptions, listStageClusterBindings, listTenants, replaceStageClusterBindings, saveDeliveryFlowTemplateStage, type ClusterOption, type DeliveryFlowTemplateStage } from '../api';
import { PageHeader } from '../components/PageHeader';

const ROLE_OPTIONS = [
  { value: 'tenant_admin', label: '租户管理员' },
  { value: 'developer', label: '开发人员' },
  { value: 'operator', label: '运维人员' },
  { value: 'prod_approver', label: '生产审批人' }
];

export function DeliveryFlowTemplatePage() {
  const queryClient = useQueryClient();
  const [editingStage, setEditingStage] = useState<DeliveryFlowTemplateStage | null>(null);
  const [stageModalOpen, setStageModalOpen] = useState(false);
  const [bindingStage, setBindingStage] = useState<DeliveryFlowTemplateStage | null>(null);
  const [selectedClusters, setSelectedClusters] = useState<string[]>([]);
  const [tenantId, setTenantId] = useState<string>();
  const [form] = Form.useForm();
  const { data: tenants = [] } = useQuery({ queryKey: ['tenants'], queryFn: listTenants });
  const currentTenantId = tenantId || tenants[0]?.id || '';
  const tenantOptions = useMemo(() => tenants.map((tenant) => ({ value: tenant.id, label: tenant.displayName || tenant.name })), [tenants]);
  const { data: template, isLoading } = useQuery({
    queryKey: ['delivery-flow-template', currentTenantId],
    queryFn: () => getDeliveryFlowTemplate(currentTenantId),
    enabled: !!currentTenantId
  });
  const { data: clusters = [] } = useQuery({
    queryKey: ['cluster-options', currentTenantId],
    queryFn: () => listClusterOptions(currentTenantId),
    enabled: !!currentTenantId
  });
  const { data: bindingClusters } = useQuery({
    queryKey: ['stage-cluster-bindings', currentTenantId, bindingStage?.stageKey],
    queryFn: () => bindingStage ? listStageClusterBindings(currentTenantId, bindingStage.stageKey) : Promise.resolve([]),
    enabled: !!currentTenantId && !!bindingStage
  });
  const stages = useMemo(() => [...(template?.stages || [])].sort((a, b) => a.order - b.order), [template?.stages]);
  const stageBindingQueries = useQueries({
    queries: stages.map((stage) => ({
      queryKey: ['stage-cluster-bindings', currentTenantId, stage.stageKey],
      queryFn: () => listStageClusterBindings(currentTenantId, stage.stageKey),
      enabled: !!currentTenantId
    }))
  });
  const stageBindingsByKey = new Map(stages.map((stage, index) => [stage.stageKey, stageBindingQueries[index]?.data || []]));

  const saveMutation = useMutation({
    mutationFn: (values: any) => saveDeliveryFlowTemplateStage(currentTenantId, { ...editingStage, ...values, stageKey: editingStage?.stageKey || values.stageKey }),
    onSuccess: () => {
      message.success('Stage 模板已保存');
      setStageModalOpen(false);
      setEditingStage(null);
      queryClient.invalidateQueries({ queryKey: ['delivery-flow-template', currentTenantId] });
    }
  });
  const deleteMutation = useMutation({
    mutationFn: (stageKey: string) => deleteDeliveryFlowTemplateStage(currentTenantId, stageKey),
    onSuccess: () => {
      message.success('Stage 已删除');
      queryClient.invalidateQueries({ queryKey: ['delivery-flow-template', currentTenantId] });
    }
  });
  const bindingMutation = useMutation({
    mutationFn: () => bindingStage ? replaceStageClusterBindings(currentTenantId, bindingStage.stageKey, selectedClusters) : Promise.resolve([]),
    onSuccess: () => {
      message.success('集群绑定已保存');
      setBindingStage(null);
      setSelectedClusters([]);
      queryClient.invalidateQueries({ queryKey: ['delivery-flow-template', currentTenantId] });
      queryClient.invalidateQueries({ queryKey: ['stage-cluster-bindings', currentTenantId] });
    }
  });

  useEffect(() => {
    if (!tenantId && tenants[0]) {
      setTenantId(tenants[0].id);
    }
  }, [tenantId, tenants]);

  useEffect(() => {
    if (!stageModalOpen) return;
    form.setFieldsValue(editingStage || { stageKey: '', displayName: '', color: '#1677ff', order: stages.length + 1, status: 'enabled', requiresApproval: false, requiresVerification: false, approveRoles: [], verifyRoles: [] });
  }, [editingStage, form, stageModalOpen, stages.length]);

  useEffect(() => {
    if (!bindingStage || !bindingClusters) return;
    setSelectedClusters(bindingClusters.map((binding) => binding.clusterId));
  }, [bindingClusters, bindingStage]);

  const openAdd = () => {
    setEditingStage(null);
    setStageModalOpen(true);
  };
  const openEdit = (stage: DeliveryFlowTemplateStage) => {
    setEditingStage(stage);
    setStageModalOpen(true);
  };

  return (
    <>
      <PageHeader title="租户交付流模板" extra={<Button type="primary" aria-label="添加 Stage" icon={<PlusOutlined />} disabled={!currentTenantId} onClick={openAdd}>添加 Stage</Button>} />
      <Typography.Paragraph type="secondary">维护租户级 Stage、审批验证策略和可选集群池。</Typography.Paragraph>
      <div className="toolbar">
        <Select aria-label="租户" placeholder="选择租户" options={tenantOptions} value={currentTenantId || undefined} onChange={(value) => { setTenantId(value); setBindingStage(null); setSelectedClusters([]); }} />
      </div>
      <div className="delivery-template-stats">
        <Card><Statistic title="当前模板" value={template?.name || '-'} loading={isLoading} /></Card>
        <Card><Statistic title="Stage 数量" value={stages.length} /></Card>
        <Card><Statistic title="引用应用" value={2} suffix="个" /></Card>
        <Card><Statistic title="生效方式" value="自动生效" /></Card>
      </div>
      <Alert className="form-alert" type="info" showIcon message="Stage key 创建后不可修改" description="删除 Stage 会物理删除模板项和该 Stage 的集群池绑定；历史发布记录继续保留。" />
      <div className="delivery-template-grid">
        {stages.map((stage) => (
          <article key={stage.stageKey} className="delivery-stage-template-card" aria-label={`${stage.stageKey} Stage 模板`}>
            <div className="stage-color-strip" style={{ backgroundColor: stage.color }}><span className="stage-strip-title">{stage.displayName}</span><Tag color={stage.status === 'enabled' ? 'green' : 'default'}>{stage.status === 'enabled' ? '启用' : '禁用'}</Tag></div>
            <div className="delivery-stage-template-body">
              <Space direction="vertical" size={6}>
                <Typography.Text type="secondary">Stage key：{stage.stageKey}</Typography.Text>
                <Typography.Text>顺序：{stage.order}</Typography.Text>
                <Typography.Text>部署前审批：{stage.requiresApproval ? '需要' : '不需要'}</Typography.Text>
                <Typography.Text>部署后验证：{stage.requiresVerification ? '需要' : '不需要'}</Typography.Text>
                <Typography.Text type="secondary">已绑定集群</Typography.Text>
                {(stageBindingsByKey.get(stage.stageKey) || []).length > 0 ? (
                  <Space size={[4, 4]} wrap>
                    {(stageBindingsByKey.get(stage.stageKey) || []).map((binding) => <Tag key={binding.clusterId}>{binding.clusterName}</Tag>)}
                  </Space>
                ) : (
                  <Typography.Text type="secondary">未绑定集群</Typography.Text>
                )}
              </Space>
              <div className="delivery-stage-template-actions">
                <Button aria-label="绑定集群" icon={<LinkOutlined />} onClick={() => { setBindingStage(stage); setSelectedClusters([]); }}>绑定集群</Button>
                <Button aria-label="编辑" icon={<EditOutlined />} onClick={() => openEdit(stage)}>编辑</Button>
                <Popconfirm
                  title="删除 Stage"
                  description="确认删除该 Stage？该 Stage 的集群池绑定也会被删除。"
                  okText="删除"
                  cancelText="取消"
                  okButtonProps={{ danger: true, loading: deleteMutation.isPending }}
                  onConfirm={() => deleteMutation.mutate(stage.stageKey)}
                >
                  <Button danger aria-label="删除" icon={<DeleteOutlined />}>删除</Button>
                </Popconfirm>
              </div>
            </div>
          </article>
        ))}
      </div>

      <Modal title={editingStage ? '编辑 Stage' : '添加 Stage'} open={stageModalOpen} onCancel={() => setStageModalOpen(false)} onOk={() => form.validateFields().then((values) => saveMutation.mutate(values))} confirmLoading={saveMutation.isPending} destroyOnHidden>
        <Alert className="form-alert" type="info" showIcon message="Stage key 创建后不可修改" />
        <Form form={form} layout="vertical">
          <Form.Item label="Stage key" name="stageKey" rules={[{ required: true, message: '请输入 Stage key' }]}><Input disabled={!!editingStage} /></Form.Item>
          <Form.Item label="显示名" name="displayName" rules={[{ required: true, message: '请输入显示名' }]}><Input /></Form.Item>
          <Form.Item label="Stage 颜色" name="color"><Input type="color" /></Form.Item>
          <Form.Item label="顺序" name="order"><InputNumber min={1} className="full-width" /></Form.Item>
          <Form.Item label="状态" name="status"><Select options={[{ value: 'enabled', label: '启用' }, { value: 'disabled', label: '禁用' }]} /></Form.Item>
          <Form.Item label="部署前审批" name="requiresApproval" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item label="部署后验证" name="requiresVerification" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item label="允许审批角色" name="approveRoles"><Select mode="multiple" options={ROLE_OPTIONS} /></Form.Item>
          <Form.Item label="允许验证角色" name="verifyRoles"><Select mode="multiple" options={ROLE_OPTIONS} /></Form.Item>
        </Form>
      </Modal>

      <Modal title="绑定集群" open={!!bindingStage} onCancel={() => setBindingStage(null)} onOk={() => bindingMutation.mutate()} confirmLoading={bindingMutation.isPending} destroyOnHidden>
        <Space direction="vertical" className="full-width" size={12}>
          <Alert type="info" showIcon message="绑定到租户级 Stage，保存后进入该 Stage 的可选集群池。" description="同一集群可绑定多个 Stage，绑定变更仅影响后续发布。" />
          <Checkbox.Group aria-label="可选集群" className="cluster-checkbox-list" value={selectedClusters} onChange={(values) => setSelectedClusters(values.map(String))}>
            {(clusters as ClusterOption[]).map((cluster) => <Checkbox key={cluster.id} value={cluster.id}>{cluster.name}（{cluster.region}）</Checkbox>)}
          </Checkbox.Group>
        </Space>
      </Modal>
    </>
  );
}
