import { useEffect, useMemo, useState, type CSSProperties, type DragEvent } from 'react';
import { CheckCircleOutlined, EditOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Descriptions, Drawer, Empty, Form, Input, InputNumber, Modal, Select, Space, Spin, Table, Tag, Tooltip, Typography, message } from 'antd';
import { Background, Handle, MarkerType, Position, ReactFlow, type Edge, type Node, type NodeProps } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { useParams } from 'react-router-dom';
import { completeFreightApproval, completeStageVerification, createFreight, createPromotion, getApplication, getFreight, getFreightCreationContext, listAppStages, listEligibleFreights, listFreights, listWorkloadEnvironmentConfigs, listWorkloads, saveWorkloadEnvironmentConfig, type AppStage, type CreateFreightInput, type Freight, type FreightItem, type ImageBundleImage, type Workload, type WorkloadEnvironmentConfig } from '../api';
import { ConfigValueLists, WorkloadRuntimeFields, workloadConfigFormValues, workloadConfigPayload } from './workloadConfigForm';

const DEFAULT_APPLICATION_ID = 'app_1';
const COLUMN_WIDTH = 340;
const ROW_HEIGHT = 280;

type FreightDraftItem = {
  sourceType: 'pipeline_artifact' | 'custom_image';
  releaseId?: string;
  buildArtifactId?: string;
  imageRef?: string;
};

type StageView = AppStage & {
  id: string;
  name: string;
  replicasSummary?: string;
  domainSummary?: string;
  configSummary?: string;
};

type PendingPromotion = {
  stage: StageView;
  freight: Freight;
};

type StageNodeData = {
  stage: StageView;
  active: boolean;
  dropState: 'idle' | 'ready' | 'blocked';
  pending?: PendingPromotion;
  onDropFreight: (stage: StageView, freightId: string) => void;
  onVerify: (stage: StageView) => void;
  onEditConfig: (stage: StageView) => void;
  onConfirm: () => void;
  onCancel: () => void;
  confirming: boolean;
};

const nodeTypes = { deployStage: DeployStageNode };

export function PromotionPage() {
  const { id } = useParams();
  const applicationId = id || DEFAULT_APPLICATION_ID;
  return <PromotionContent applicationId={applicationId} showHeader />;
}

export function PromotionContent({ applicationId = DEFAULT_APPLICATION_ID, showHeader = false }: { applicationId?: string; showHeader?: boolean }) {
  const queryClient = useQueryClient();
  const [activeStage, setActiveStage] = useState<StageView | null>(null);
  const [pendingPromotion, setPendingPromotion] = useState<PendingPromotion | undefined>();
  const [approvalFreight, setApprovalFreight] = useState<Freight | null>(null);
  const [approvalTargetStage, setApprovalTargetStage] = useState('prod');
  const [approvalComment, setApprovalComment] = useState('');
  const [verificationStage, setVerificationStage] = useState<StageView | null>(null);
  const [verificationComment, setVerificationComment] = useState('');
  const [configStage, setConfigStage] = useState<StageView | null>(null);
  const [selectedConfigWorkloadId, setSelectedConfigWorkloadId] = useState('');
  const [configForm] = Form.useForm();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [draftName, setDraftName] = useState('');
  const [draftItems, setDraftItems] = useState<Record<string, FreightDraftItem>>({});
  const [stageFreights, setStageFreights] = useState<Record<string, string>>({});
  const [publishResult, setPublishResult] = useState('');
  const [draggingFreightId, setDraggingFreightId] = useState('');

  const freightsQuery = useQuery({ queryKey: ['freights', applicationId], queryFn: () => listFreights(applicationId) });
  const applicationQuery = useQuery({ queryKey: ['application', applicationId], queryFn: () => getApplication(applicationId) });
  const contextQuery = useQuery({ queryKey: ['freight-creation-context', applicationId], queryFn: () => getFreightCreationContext(applicationId) });
  const appStagesQuery = useQuery({ queryKey: ['app-stages', applicationId], queryFn: () => listAppStages(applicationId) });
  const workloadsQuery = useQuery({ queryKey: ['workloads', applicationId, 'stage-config'], queryFn: () => listWorkloads(applicationId), enabled: !!applicationId });
  const stageConfigQuery = useQuery({
    queryKey: ['workload-environment-configs', applicationId, selectedConfigWorkloadId, configStage?.environmentId, 'stage-config'],
    queryFn: () => listWorkloadEnvironmentConfigs(applicationId, selectedConfigWorkloadId),
    enabled: !!configStage?.environmentId && !!selectedConfigWorkloadId
  });
  const eligibleMutation = useMutation({ mutationFn: (stageId: string) => listEligibleFreights(applicationId, stageId) });
  const createPromotionMutation = useMutation({
    mutationFn: (input: PendingPromotion) => createPromotion({
      freightId: input.freight.id,
      targetStageKey: input.stage.stageKey,
      targetClusterIds: input.stage.boundClusterId ? [input.stage.boundClusterId] : [],
      namespaceOverride: defaultNamespace
    }),
    onSuccess: (_, input) => {
      setStageFreights((current) => ({ ...current, [input.stage.stageKey]: input.freight.version }));
      setPublishResult(`${input.freight.version} 已提交到 ${input.stage.stageKey}，等待同步结果。`);
      setPendingPromotion(undefined);
      queryClient.invalidateQueries({ queryKey: ['app-stages', applicationId] });
      queryClient.invalidateQueries({ queryKey: ['freights', applicationId] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '发布失败')
  });
  const approvalMutation = useMutation({
    mutationFn: (decision: 'approved' | 'rejected') => approvalFreight ? completeFreightApproval(approvalFreight.id, { targetStageKey: approvalTargetStage, decision, comment: approvalComment }) : Promise.resolve(null),
    onSuccess: () => {
      message.success('Freight 审批已提交');
      setApprovalFreight(null);
      setApprovalComment('');
    }
  });
  const verificationMutation = useMutation({
    mutationFn: (status: 'passed' | 'failed') => {
      const freight = currentVerificationFreight(verificationStage, sortedFreights);
      return verificationStage && freight ? completeStageVerification(applicationId, verificationStage.stageKey, { freightId: freight.id, status, comment: verificationComment, syncStatus: 'Synced', healthStatus: 'Healthy', agentStatus: 'ready' }) : Promise.resolve(null);
    },
    onSuccess: () => {
      message.success('人工验证已提交');
      setVerificationStage(null);
      setVerificationComment('');
      queryClient.invalidateQueries({ queryKey: ['app-stages', applicationId] });
    }
  });
  const stageConfigMutation = useMutation({
    mutationFn: async () => {
      if (!configStage?.environmentId || !selectedConfigWorkloadId) throw new Error('请选择工作负载和 Stage');
      await configForm.validateFields();
      return saveWorkloadEnvironmentConfig(applicationId, selectedConfigWorkloadId, configStage.environmentId, workloadConfigPayload(configForm.getFieldsValue(true)));
    },
    onSuccess: () => {
      message.success('Stage 配置已保存');
      queryClient.invalidateQueries({ queryKey: ['workload-environment-configs', applicationId] });
      setConfigStage(null);
      setSelectedConfigWorkloadId('');
      configForm.resetFields();
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存 Stage 配置失败')
  });
  const createFreightMutation = useMutation({
    mutationFn: (input: CreateFreightInput) => createFreight(applicationId, input),
    onSuccess: () => {
      setDrawerOpen(false);
      setDraftItems({});
      setDraftName('');
      queryClient.invalidateQueries({ queryKey: ['freights', applicationId] });
      queryClient.invalidateQueries({ queryKey: ['freight-creation-context', applicationId] });
    }
  });

  const sortedFreights = useMemo(() => [...(freightsQuery.data || [])].sort((a, b) => timeValue(a.createdAt) - timeValue(b.createdAt)), [freightsQuery.data]);
  const enabledWorkloads = contextQuery.data?.enabledWorkloads || [];
  const configWorkloads = workloadsQuery.data || enabledWorkloads;
  const workloadNameById = useMemo(() => Object.fromEntries(enabledWorkloads.map((workload) => [workload.id, workload.displayName || workload.name])), [enabledWorkloads]);
  const stages = useMemo(() => (appStagesQuery.data || []).map((stage) => withStageDefaults(stage, sortedFreights, stageFreights)), [appStagesQuery.data, sortedFreights, stageFreights]);
  const defaultNamespace = applicationQuery.data?.project || applicationQuery.data?.projectId || 'default';
  const selectedCount = enabledWorkloads.filter((workload) => draftItemComplete(draftItems[workload.id])).length;
  const submitDisabled = enabledWorkloads.length === 0 || selectedCount < enabledWorkloads.length;

  const nodes = useMemo<Node<StageNodeData>[]>(() => stages.map((stage) => ({
    id: stage.stageKey,
    type: 'deployStage',
    position: { x: Math.max(0, stage.layoutColumn || 0) * COLUMN_WIDTH, y: (stage.layoutRow || 0) * ROW_HEIGHT },
    data: {
      stage,
      active: activeStage?.stageKey === stage.stageKey,
      dropState: draggingFreightId ? stageDropState(stage, draggingFreightId, contextQuery.data?.stageEligibility) : 'idle',
      pending: pendingPromotion?.stage.stageKey === stage.stageKey ? pendingPromotion : undefined,
      onDropFreight: handleDropFreight,
      onVerify: setVerificationStage,
      onEditConfig: handleOpenStageConfig,
      onConfirm: () => pendingPromotion && createPromotionMutation.mutate(pendingPromotion),
      onCancel: () => setPendingPromotion(undefined),
      confirming: createPromotionMutation.isPending && pendingPromotion?.stage.stageKey === stage.stageKey
    }
  })), [stages, activeStage?.stageKey, draggingFreightId, contextQuery.data?.stageEligibility, pendingPromotion, createPromotionMutation.isPending]);

  const edges = useMemo<Edge[]>(() => {
    const out: Edge[] = [];
    for (const stage of stages) {
      for (const downstream of stage.downstreamStageKeys || []) {
        out.push({ id: `${stage.stageKey}->${downstream}`, source: stage.stageKey, target: downstream, markerEnd: { type: MarkerType.ArrowClosed }, className: 'delivery-dag-edge' });
      }
    }
    return out;
  }, [stages]);

  async function handleDropFreight(stage: StageView, freightId: string) {
    const freight = sortedFreights.find((item) => item.id === freightId);
    if (!freight) return;
    setActiveStage(stage);
    setPendingPromotion(undefined);
    setPublishResult('');
    if (stage.status === 'disabled') {
      message.warning('禁用 Stage 不能发布');
      return;
    }
    if (!stage.boundClusterId) {
      message.warning('该 Stage 未绑定集群，请先在交付流模板中绑定集群');
      return;
    }
    try {
      const eligible = await eligibleMutation.mutateAsync(stage.deliveryStageId || stage.id);
      if (!eligible.some((item) => item.id === freight.id)) {
        message.warning('该 Freight 当前不能发布到目标 Stage');
        return;
      }
      const detail = freight.items?.length ? freight : await getFreight(freight.id);
      setPendingPromotion({ stage, freight: detail });
    } catch (error) {
      message.error(error instanceof Error ? error.message : '校验可发布 Freight 失败');
    }
  }

  const handleOpenApproval = (freight: Freight) => {
    setApprovalFreight(freight);
    setApprovalTargetStage(activeStage?.stageKey || stages.find((stage) => stage.requiresApproval)?.stageKey || stages[0]?.stageKey || 'prod');
    setApprovalComment('');
  };

  const handleRollback = () => {
    const prodStage = stages.find((stage) => stage.stageKey === 'prod') || stages[0];
    if (prodStage) setActiveStage(prodStage);
  };

  const updateDraftItem = (workloadId: string, patch: Partial<FreightDraftItem>) => {
    setDraftItems((current) => ({
      ...current,
      [workloadId]: { ...current[workloadId], sourceType: current[workloadId]?.sourceType || 'pipeline_artifact', ...patch }
    }));
  };

  const fillLatest = () => {
    setDraftItems(Object.fromEntries(enabledWorkloads.map((workload) => {
      const release = contextQuery.data?.latestReleasesByWorkload[workload.id];
      return [workload.id, { sourceType: 'pipeline_artifact', releaseId: release?.id, buildArtifactId: release?.buildArtifactId }];
    })));
  };

  const copyHistory = () => {
    const source = [...sortedFreights].reverse().find((freight) => freight.items?.length);
    if (!source) return fillLatest();
    setDraftItems(Object.fromEntries(enabledWorkloads.map((workload) => {
      const item = source.items?.find((candidate) => candidate.workloadId === workload.id);
      return [workload.id, item?.sourceType === 'custom_image'
        ? { sourceType: 'custom_image', imageRef: item.image }
        : { sourceType: 'pipeline_artifact', releaseId: contextQuery.data?.latestReleasesByWorkload[workload.id]?.id, buildArtifactId: contextQuery.data?.latestReleasesByWorkload[workload.id]?.buildArtifactId }];
    })));
  };

  function handleOpenStageConfig(stage: StageView) {
    setConfigStage(stage);
    setSelectedConfigWorkloadId(configWorkloads[0]?.id || '');
    configForm.setFieldsValue(workloadConfigFormValues());
  }

  useEffect(() => {
    if (!configStage) return;
    if (!selectedConfigWorkloadId && configWorkloads[0]?.id) {
      setSelectedConfigWorkloadId(configWorkloads[0].id);
    }
  }, [configStage, configWorkloads, selectedConfigWorkloadId]);

  useEffect(() => {
    if (!configStage || !selectedConfigWorkloadId) return;
    const currentConfig = (stageConfigQuery.data || []).find((item: WorkloadEnvironmentConfig) => item.environmentId === configStage.environmentId);
    configForm.setFieldsValue(workloadConfigFormValues(currentConfig));
  }, [configForm, configStage, selectedConfigWorkloadId, stageConfigQuery.data]);

  const handleCreateFreight = () => {
    createFreightMutation.mutate({
      name: draftName.trim() || `freight-${Date.now()}`,
      items: enabledWorkloads.map((workload) => {
        const item = draftItems[workload.id];
        return { workloadId: workload.id, sourceType: item.sourceType, releaseId: item.releaseId, buildArtifactId: item.buildArtifactId, imageRef: item.imageRef };
      })
    });
  };

  return (
    <div data-testid={showHeader ? undefined : 'promotion-confirm-panel'}>
      <div className="embedded-section-head promotion-actions-only">{contextQuery.isLoading ? null : <Button type="primary" aria-label="创建 Freight" onClick={() => setDrawerOpen(true)}>创建 Freight</Button>}</div>

      <div className="promotion-workspace">
        <div className="promotion-main-column">
          <Card title="Freight 时间轴" className="promotion-timeline-card">
            {freightsQuery.isLoading ? <Spin /> : (
              <div className="freight-timeline" aria-label="Freight 时间轴">
                {sortedFreights.length === 0 ? <Empty description="暂无 Freight" /> : sortedFreights.map((freight) => {
                  const stageColors = freightStageColors(freight, stages);
                  return (
                    <article
                      key={freight.id}
                      className={draggingFreightId === freight.id ? 'freight-timeline-card dragging' : 'freight-timeline-card'}
                      data-testid="freight-card"
                      draggable
                      onDragStart={(event) => {
                        event.dataTransfer.setData('text/plain', freight.id);
                        event.dataTransfer.effectAllowed = 'move';
                        setDraggingFreightId(freight.id);
                        setFreightDragImage(event, freight.version);
                      }}
                      onDragEnd={() => setDraggingFreightId('')}
                    >
                      <div className="freight-stage-rail" aria-label={stageColors.length ? `当前部署 Stage：${stageColors.map((item) => item.name).join('、')}` : '当前未部署到 Stage'}>
                        {stageColors.length ? stageColors.map((item) => <span key={item.key} style={{ backgroundColor: item.color }} />) : <span />}
                      </div>
                      <div className="freight-card-head">
                        <Typography.Text strong data-testid="freight-name">{freight.version}</Typography.Text>
                        <Tag color="blue">拖拽</Tag>
                      </div>
                      <div className="muted">{freight.createdAt}</div>
                      <div className="freight-card-items">
                        {(freight.items || []).map((item) => <div key={item.id} className="freight-card-item"><span>{item.workloadDisplayName}</span><Typography.Text ellipsis>{item.image}</Typography.Text>{item.bundleImages?.length ? <Tag color="blue">{item.bundleImages.length} 个镜像</Tag> : null}</div>)}
                      </div>
                      <Space className="freight-card-actions">
                        <Button aria-label="审批" className="nodrag nopan" onMouseDown={(event) => event.stopPropagation()} onClick={(event) => { event.stopPropagation(); handleOpenApproval(freight); }}>审批</Button>
                      </Space>
                    </article>
                  );
                })}
              </div>
            )}
          </Card>

          <section className="deployment-dag-canvas" aria-label="应用部署 DAG">
            {appStagesQuery.isLoading ? <Spin /> : (
              <ReactFlow
                nodes={nodes}
                edges={edges}
                nodeTypes={nodeTypes}
                nodesDraggable={false}
                nodesConnectable={false}
                panOnDrag={false}
                panOnScroll={false}
                zoomOnScroll={false}
                zoomOnDoubleClick={false}
                zoomOnPinch={false}
                selectNodesOnDrag={false}
                fitView
              >
                <Background />
              </ReactFlow>
            )}
          </section>

          <Card title="近期发布记录" className="compact-card">
            <div className="promotion-history-row">
              <Space direction="vertical" size={2}><Typography.Text strong>生产审批</Typography.Text><Typography.Text type="secondary">最新生产发布待审批，禁止发起人自审批。</Typography.Text></Space>
              <Button onClick={handleRollback}>回滚</Button>
            </div>
          </Card>
        </div>
      </div>
      {publishResult && <Alert className="form-alert" type="success" showIcon message={publishResult} />}

      <Modal title="Freight 审批" open={!!approvalFreight} onCancel={() => setApprovalFreight(null)} destroyOnHidden footer={[
        <Button key="reject" danger loading={approvalMutation.isPending} onClick={() => approvalMutation.mutate('rejected')}>审批拒绝</Button>,
        <Button key="approve" type="primary" loading={approvalMutation.isPending} onClick={() => approvalMutation.mutate('approved')}>审批通过</Button>
      ]}>
        <Space direction="vertical" size={14} className="full-width">
          <Descriptions size="small" column={1} items={[
            { key: 'freight', label: '审批 Freight', children: approvalFreight?.version || '-' },
            { key: 'source', label: '晋级来源', children: activeStage?.stageKey ? `${activeStage.stageKey} Stage` : '最近发布记录' },
            { key: 'roles', label: '审批角色', children: stages.find((stage) => stage.stageKey === approvalTargetStage)?.approveRoles?.join('、') || '租户管理员 / 生产审批人' }
          ]} />
          <Form layout="vertical">
            <Form.Item label="目标 Stage"><Select aria-label="目标 Stage" value={approvalTargetStage} onChange={setApprovalTargetStage} options={stages.map((stage) => ({ value: stage.stageKey, label: stage.displayName || stage.stageKey }))} /></Form.Item>
            <Form.Item label="审批意见"><Input.TextArea aria-label="审批意见" value={approvalComment} onChange={(event) => setApprovalComment(event.target.value)} rows={3} /></Form.Item>
          </Form>
        </Space>
      </Modal>

      <Modal title="人工验证" open={!!verificationStage} onCancel={() => setVerificationStage(null)} destroyOnHidden footer={[
        <Button key="fail" danger loading={verificationMutation.isPending} onClick={() => verificationMutation.mutate('failed')}>验证不通过</Button>,
        <Button key="pass" type="primary" loading={verificationMutation.isPending} onClick={() => verificationMutation.mutate('passed')}>验证通过</Button>
      ]}>
        <Space direction="vertical" size={14} className="full-width">
          <Descriptions size="small" column={1} items={[
            { key: 'stage', label: '验证 Stage', children: verificationStage?.stageKey || '-' },
            { key: 'freight', label: '当前 Freight', children: verificationStage?.currentFreightVersion || '-' },
            { key: 'sync', label: 'Argo CD 同步', children: <Tag color="green">{verificationStage?.syncStatus || 'Synced'}</Tag> },
            { key: 'health', label: '健康状态', children: <Tag color="green">{verificationStage?.healthStatus || 'Healthy'}</Tag> },
            { key: 'agent', label: 'Agent 状态', children: <Tag color="blue">ready</Tag> }
          ]} />
          <Form layout="vertical">
            <Form.Item label="验证备注"><Input.TextArea aria-label="验证备注" value={verificationComment} onChange={(event) => setVerificationComment(event.target.value)} rows={3} /></Form.Item>
          </Form>
        </Space>
      </Modal>

      <Modal title="编辑 Stage 配置" open={!!configStage} onCancel={() => setConfigStage(null)} width={920} destroyOnHidden footer={[
        <Button key="cancel" onClick={() => setConfigStage(null)}>取消</Button>,
        <Button key="save" aria-label="保存" type="primary" loading={stageConfigMutation.isPending} onClick={() => stageConfigMutation.mutate()}>保存</Button>
      ]}>
        <Form form={configForm} layout="vertical" className="workload-large-form">
          <Form.Item label="选择工作负载" required>
            <select
              aria-label="选择工作负载"
              className="native-select"
              value={selectedConfigWorkloadId}
              disabled={workloadsQuery.isLoading}
              onChange={(event) => setSelectedConfigWorkloadId(event.target.value)}
            >
              {configWorkloads.map((workload) => <option key={workload.id} value={workload.id}>{workload.displayName || workload.name}</option>)}
            </select>
          </Form.Item>
          <WorkloadRuntimeFields />
          <ConfigValueLists />
        </Form>
      </Modal>

      <Drawer title="创建 Freight" open={drawerOpen} width={980} onClose={() => setDrawerOpen(false)} extra={<Button type="primary" aria-label="创建 Freight" disabled={submitDisabled} loading={createFreightMutation.isPending} onClick={handleCreateFreight}>创建 Freight</Button>}>
        <Space direction="vertical" size={16} className="full-width">
          <Form layout="vertical"><Form.Item label="Freight 名称"><Input value={draftName} onChange={(event) => setDraftName(event.target.value)} placeholder="请输入 Freight 名称" /></Form.Item></Form>
          <Alert type="info" showIcon message="系统会自动列出全部启用 Workload。每个 Workload 必须选择镜像版本，不能少选。" />
          <Table pagination={false} rowKey="id" dataSource={enabledWorkloads} columns={freightDraftColumns(enabledWorkloads, draftItems, contextQuery.data?.latestReleasesByWorkload || {}, updateDraftItem)} />
          <div className="validation-bar">
            <div><Typography.Text strong>{submitDisabled ? '尚未覆盖全部 Workload' : '已覆盖全部 Workload'}</Typography.Text><div className="muted">已选择 {selectedCount} / {enabledWorkloads.length} 个 Workload，需要全部选择后才能创建 Freight。</div></div>
            <Space><Button onClick={fillLatest}>从最新成功版本填充</Button><Button onClick={copyHistory}>从历史 Freight 复制</Button></Space>
          </div>
        </Space>
      </Drawer>
    </div>
  );
}

function DeployStageNode({ data }: NodeProps<Node<StageNodeData>>) {
  const stage = data.stage;
  const pending = data.pending;
  const className = ['deployment-stage-node', 'nodrag', 'nopan'];
  if (data.active) className.push('active');
  if (data.dropState === 'ready') className.push('drop-ready');
  if (data.dropState === 'blocked') className.push('drop-blocked');
  return (
    <div
      className={className.join(' ')}
      style={{ '--stage-color': stage.color } as CSSProperties}
      aria-label={`${stage.stageKey} Stage`}
      onMouseDown={(event) => event.stopPropagation()}
      onDragOver={(event) => {
        event.preventDefault();
        event.dataTransfer.dropEffect = data.dropState === 'blocked' ? 'none' : 'move';
      }}
      onDrop={(event) => {
        event.preventDefault();
        event.stopPropagation();
        data.onDropFreight(stage, event.dataTransfer.getData('text/plain'));
      }}
    >
      <Handle type="target" position={Position.Left} />
      <div className="deployment-stage-strip">
        <div>
          <span className="deployment-stage-title">{stage.displayName || stage.stageKey}</span>
        </div>
        <Space size={4}>
          <Tooltip title="编辑配置">
            <Button
              aria-label="编辑配置"
              className="stage-verify-button nodrag nopan"
              icon={<EditOutlined />}
              shape="circle"
              size="small"
              onClick={(event) => { event.stopPropagation(); data.onEditConfig(stage); }}
            />
          </Tooltip>
          <Tooltip title="人工验证">
            <Button
              aria-label="验证"
              className="stage-verify-button nodrag nopan"
              icon={<CheckCircleOutlined />}
              shape="circle"
              size="small"
              onClick={(event) => { event.stopPropagation(); data.onVerify(stage); }}
            />
          </Tooltip>
        </Space>
      </div>
      {data.dropState === 'blocked' && <div className="stage-drop-mask">未达成部署条件</div>}
      <Descriptions size="small" column={1} items={[
        { key: 'cluster', label: '绑定集群', children: stage.boundClusterName || '未绑定' },
        { key: 'freight', label: '当前 Freight', children: stage.currentFreightVersion || '-' },
        { key: 'upstream', label: '上游 Stage', children: stage.upstreamStageKeys?.length ? stage.upstreamStageKeys.join('、') : '无' },
        { key: 'verification', label: '验证状态', children: stage.requiresVerification ? '需要验证' : '无需验证' }
      ]} />
      <Space size={4} wrap>
        {stage.requiresApproval && <Tag color="orange">需审批</Tag>}
        {stage.requiresVerification && <Tag color="blue">需验证</Tag>}
        {stage.status === 'disabled' && <Tag>禁用</Tag>}
      </Space>
      {pending && (
        <div className="stage-drop-confirm nodrag nopan" aria-label={`${stage.stageKey} 发布确认`} onMouseDown={(event) => event.stopPropagation()} onClick={(event) => event.stopPropagation()}>
          <Typography.Text strong>{pending.freight.version}</Typography.Text>
          <Typography.Text type="secondary">发布到 {stage.boundClusterName}</Typography.Text>
          {(pending.freight.items || []).length > 0 && (
            <div className="stage-drop-items">
              {(pending.freight.items || []).map((item) => (
                <div key={item.id} className="stage-drop-item">
                  <span>{item.workloadDisplayName || item.workloadName}</span>
                  <Typography.Text ellipsis>{item.image}</Typography.Text>
                  {item.bundleImages?.length ? <Tag color="blue">ImageBundle · {item.bundleImages.length}</Tag> : null}
                </div>
              ))}
            </div>
          )}
          <Space>
            <Button size="small" className="nodrag nopan" onClick={(event) => { event.stopPropagation(); data.onCancel(); }}>取消</Button>
            <Button size="small" className="nodrag nopan" type="primary" loading={data.confirming} onClick={(event) => { event.stopPropagation(); data.onConfirm(); }}>确认发布</Button>
          </Space>
        </div>
      )}
      <Handle type="source" position={Position.Right} />
    </div>
  );
}

function freightDraftColumns(workloads: Workload[], draftItems: Record<string, FreightDraftItem>, releases: Record<string, any>, updateDraftItem: (workloadId: string, patch: Partial<FreightDraftItem>) => void) {
  return [
    { title: 'Workload', dataIndex: 'displayName', render: (_: string, workload: Workload) => <Space direction="vertical" size={0}><Typography.Text strong>{workload.displayName || workload.name}</Typography.Text><Typography.Text type="secondary">必须包含</Typography.Text></Space> },
    { title: '镜像来源', key: 'source', render: (_: unknown, workload: Workload) => {
      const draft = draftItems[workload.id] || { sourceType: 'pipeline_artifact' };
      return <Select aria-label={`${workload.displayName}镜像来源`} value={draft.sourceType} style={{ width: 132 }} options={[{ value: 'pipeline_artifact', label: '流水线产物', title: '流水线产物' }, { value: 'custom_image', label: '自定义镜像', title: '自定义镜像' }]} onChange={(value) => updateDraftItem(workload.id, { sourceType: value, releaseId: undefined, buildArtifactId: undefined, imageRef: '' })} />;
    } },
    { title: '流水线产物', key: 'release', render: (_: unknown, workload: Workload) => {
      const release = releases[workload.id];
      const draft = draftItems[workload.id] || { sourceType: 'pipeline_artifact' };
      return <Select disabled={draft.sourceType !== 'pipeline_artifact'} value={draft.releaseId} placeholder="选择成功构建镜像" style={{ width: 260 }} options={release ? [{ value: release.id, label: <Space direction="vertical" size={0}><Typography.Text>{release.image}</Typography.Text><Typography.Text type="secondary">{bundleSummary(release.bundleImages)}</Typography.Text></Space>, title: release.image }] : []} onChange={(value) => updateDraftItem(workload.id, { releaseId: value, buildArtifactId: release?.buildArtifactId })} />;
    } },
    { title: '自定义镜像', key: 'custom', render: (_: unknown, workload: Workload) => {
      const draft = draftItems[workload.id] || { sourceType: 'pipeline_artifact' };
      const tagRisk = draft.sourceType === 'custom_image' && !!draft.imageRef && !draft.imageRef.includes('@sha256:') && /:[^/]+$/.test(draft.imageRef);
      return <Space direction="vertical" size={6} className="full-width"><Input aria-label={`${workload.displayName}自定义镜像`} disabled={draft.sourceType !== 'custom_image'} value={draft.imageRef} placeholder="registry.example.com/app/service:v1" onChange={(event) => updateDraftItem(workload.id, { imageRef: event.target.value })} />{tagRisk && <Alert type="warning" showIcon message="镜像 tag 可能被覆盖，建议使用 digest。" />}</Space>;
    } },
    { title: '校验', key: 'status', render: (_: unknown, workload: Workload) => draftItemComplete(draftItems[workload.id]) ? <Tag color="green">已选择</Tag> : <Tag color="red">待选择</Tag> },
    { title: '说明', key: 'desc', render: (_: unknown, workload: Workload) => <Typography.Text type="secondary">{workload.description || workload.displayName || workloads.find((item) => item.id === workload.id)?.name}</Typography.Text> }
  ];
}

function draftItemComplete(item?: FreightDraftItem) {
  if (!item) return false;
  if (item.sourceType === 'pipeline_artifact') return !!item.releaseId;
  return !!item.imageRef?.trim();
}

function timeValue(value: string) {
  const parsed = new Date(value).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
}

function currentVerificationFreight(stage: StageView | null, freights: Freight[]) {
  if (!stage) return null;
  return freights.find((freight) => freight.version === stage.currentFreightVersion) || freights[freights.length - 1] || null;
}

function withStageDefaults(stage: AppStage, freights: Freight[], current: Record<string, string>): StageView {
  const fallback = freights[freights.length - 1]?.version || '-';
  const defaults: Record<string, Partial<StageView>> = {
    dev: { replicasSummary: '1 / 1 / 1', domainSummary: 'dev.example.com', configSummary: 'dev values' },
    test: { replicasSummary: '1 / 1 / 1', domainSummary: 'test.example.com', configSummary: 'test values' },
    staging: { replicasSummary: '2 / 2 / 1', domainSummary: 'staging.example.com', configSummary: 'staging values' },
    prod: { replicasSummary: '2 / 4 / 2', domainSummary: 'prod.example.com', configSummary: 'prod values' }
  };
  return { ...defaults[stage.stageKey], ...stage, id: stage.deliveryStageId || stage.stageKey, name: stage.stageKey, color: stage.color, currentFreightVersion: current[stage.stageKey] || stage.currentFreightVersion || fallback };
}

function bundleSummary(images?: ImageBundleImage[]) {
  return images?.length ? `ImageBundle · ${images.length} 个镜像` : '流水线产物';
}

function stageDropState(stage: StageView, freightId: string, stageEligibility?: Record<string, string[]>): 'ready' | 'blocked' {
  if (stage.status === 'disabled' || !stage.boundClusterId) return 'blocked';
  if (!stageEligibility) return 'ready';
  const keys = [stage.deliveryStageId, stage.id, stage.stageKey, stage.environmentId].filter(Boolean) as string[];
  const eligibleIds = new Set(keys.flatMap((key) => stageEligibility[key] || []));
  return eligibleIds.has(freightId) ? 'ready' : 'blocked';
}

function freightStageColors(freight: Freight, stages: StageView[]) {
  return stages
    .filter((stage) => stage.currentFreightId === freight.id || stage.currentFreightVersion === freight.version)
    .map((stage) => ({ key: stage.stageKey, name: stage.displayName || stage.stageKey, color: stage.color }));
}

function setFreightDragImage(event: DragEvent<HTMLElement>, version: string) {
  if (!event.dataTransfer.setDragImage || typeof document === 'undefined') return;
  const packageNode = document.createElement('div');
  packageNode.className = 'freight-drag-package';
  packageNode.textContent = version;
  document.body.appendChild(packageNode);
  event.dataTransfer.setDragImage(packageNode, 28, 24);
  window.setTimeout(() => packageNode.remove(), 0);
}
