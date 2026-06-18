import { useEffect, useMemo, useRef, useState, type CSSProperties, type DragEvent, type ReactNode } from 'react';
import { CheckCircleFilled, CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, DeleteOutlined, DownOutlined, EditOutlined, InboxOutlined, PlusOutlined, QuestionCircleOutlined, ReloadOutlined, RightOutlined, SettingOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Badge, Button, Card, Descriptions, Drawer, Empty, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Spin, Statistic, Table, Tag, Tooltip, Typography, message } from 'antd';
import { Background, Controls, Handle, MarkerType, Position, ReactFlow, type Edge, type Node, type NodeProps } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { useParams } from 'react-router-dom';
import { completeFreightApproval, completeStageVerification, createFreight, createPromotion, deleteFreight, getApplication, getFreight, getFreightCreationContext, listAppStages, listEligibleFreights, listFreights, listWorkloadStageConfigs, listWorkloads, openRuntimePodTerminal, restartRuntimeResource, saveWorkloadStageConfig, streamRuntimePodLogs, streamRuntimeResources, type AppStage, type CreateFreightInput, type Freight, type FreightItem, type ImageBundleImage, type RuntimeResource, type Workload, type WorkloadStageConfig } from '../api';
import { ConfigValueLists, WorkloadRuntimeFields, workloadConfigFormValues, workloadConfigPayload } from './workloadConfigForm';
import { buildRuntimeTopology, computeRuntimeSummary, formatControllerReplicas, formatPodReady, normalizePodPhase, sumRestartCount, truncateMessage, uncategorizedStatusText, type ControllerGroup, type PodPhase, type RuntimeSummary, type RuntimeTopology } from './runtimeTopology';

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

type RuntimePanel = {
  resource: RuntimeResource;
  container?: string;
};

type TerminalStatus = 'connecting' | 'connected' | 'disconnected' | 'forbidden';

type StageNodeData = {
  stage: StageView;
  active: boolean;
  dropState: 'idle' | 'ready' | 'blocked';
  pending?: PendingPromotion;
  onDropFreight: (stage: StageView, freightId: string) => void;
  onVerify: (stage: StageView) => void;
  onEditConfig: (stage: StageView) => void;
  onOpenRuntime: (stage: StageView) => void;
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

type PromotionContentProps = {
  applicationId?: string;
  showHeader?: boolean;
  mode?: 'default' | 'applicationWorkspace';
  leftAfterTimeline?: ReactNode;
};

export function PromotionContent({ applicationId = DEFAULT_APPLICATION_ID, showHeader = false, mode = 'default', leftAfterTimeline }: PromotionContentProps) {
  const queryClient = useQueryClient();
  const [activeStage, setActiveStage] = useState<StageView | null>(null);
  const [pendingPromotion, setPendingPromotion] = useState<PendingPromotion | undefined>();
  const [approvalFreight, setApprovalFreight] = useState<Freight | null>(null);
  const [approvalTargetStage, setApprovalTargetStage] = useState('prod');
  const [approvalComment, setApprovalComment] = useState('');
  const [verificationStage, setVerificationStage] = useState<StageView | null>(null);
  const [verificationComment, setVerificationComment] = useState('');
  const [configStage, setConfigStage] = useState<StageView | null>(null);
  const [runtimeStage, setRuntimeStage] = useState<StageView | null>(null);
  const [runtimeResources, setRuntimeResources] = useState<RuntimeResource[]>([]);
  const [runtimeLoading, setRuntimeLoading] = useState(false);
  const [runtimeStreamStatus, setRuntimeStreamStatus] = useState('');
  const [runtimeError, setRuntimeError] = useState('');
  const [expandedControllers, setExpandedControllers] = useState<Record<string, boolean>>({});
  const [logPanel, setLogPanel] = useState<RuntimePanel | null>(null);
  const [runtimeLogs, setRuntimeLogs] = useState('');
  const [logStreamStatus, setLogStreamStatus] = useState('');
  const [terminalPanel, setTerminalPanel] = useState<RuntimePanel | null>(null);
  const [terminalStatus, setTerminalStatus] = useState<TerminalStatus>('disconnected');
  const [terminalOutput, setTerminalOutput] = useState('');
  const [terminalInput, setTerminalInput] = useState('');
  const terminalConnectionRef = useRef<{ send: (text: string) => void; close: () => void } | null>(null);
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
    queryKey: ['workload-stage-configs', applicationId, selectedConfigWorkloadId, configStage?.stageKey],
    queryFn: () => listWorkloadStageConfigs(applicationId, selectedConfigWorkloadId),
    enabled: !!configStage?.stageKey && !!selectedConfigWorkloadId
  });
  const eligibleMutation = useMutation({ mutationFn: (stageId: string) => listEligibleFreights(applicationId, stageId) });
  const createPromotionMutation = useMutation({
    mutationFn: (input: PendingPromotion) => createPromotion(
      {
        freightId: input.freight.id,
        targetStageKey: input.stage.stageKey,
        targetClusterIds: input.stage.boundClusterId ? [input.stage.boundClusterId] : [],
        namespaceOverride: defaultNamespace
      },
      applicationId,
      input.stage.deliveryStageId || input.stage.id
    ),
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
      if (!configStage?.stageKey || !selectedConfigWorkloadId) throw new Error('请选择工作负载和 Stage');
      await configForm.validateFields();
      return saveWorkloadStageConfig(applicationId, selectedConfigWorkloadId, configStage.stageKey, workloadConfigPayload(configForm.getFieldsValue(true)));
    },
    onSuccess: () => {
      message.success('Stage 配置已保存');
      queryClient.invalidateQueries({ queryKey: ['workload-stage-configs', applicationId] });
      setConfigStage(null);
      setSelectedConfigWorkloadId('');
      configForm.resetFields();
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存 Stage 配置失败')
  });
  const restartRuntimeMutation = useMutation({
    mutationFn: (resource: RuntimeResource) => restartRuntimeResource(applicationId, resource.stageKey, resource.id),
    onSuccess: () => {
      message.success('重启任务已提交');
      queryClient.invalidateQueries({ queryKey: ['runtime-resources', applicationId, runtimeStage?.stageKey] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '提交重启任务失败')
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
  const archiveFreightMutation = useMutation({
    mutationFn: (freightId: string) => deleteFreight(freightId),
    onSuccess: () => {
      message.success('Freight 已归档');
      queryClient.invalidateQueries({ queryKey: ['freights', applicationId] });
      queryClient.invalidateQueries({ queryKey: ['freight-creation-context', applicationId] });
      queryClient.invalidateQueries({ queryKey: ['app-stages', applicationId] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '归档 Freight 失败')
  });

  const sortedFreights = useMemo(() => [...(freightsQuery.data || [])].sort((a, b) => timeValue(b.createdAt) - timeValue(a.createdAt)), [freightsQuery.data]);
  const enabledWorkloads = contextQuery.data?.enabledWorkloads || [];
  const configWorkloads = (workloadsQuery.data || enabledWorkloads).filter((workload) => workload.status !== 'deleted');
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
      onOpenRuntime: setRuntimeStage,
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
    const source = sortedFreights.find((freight) => freight.items?.length);
    if (!source) return fillLatest();
    setDraftItems(Object.fromEntries(enabledWorkloads.map((workload) => {
      const item = source.items?.find((candidate) => candidate.workloadId === workload.id);
      return [workload.id, item?.sourceType === 'custom_image'
        ? { sourceType: 'custom_image', imageRef: item.image }
        : { sourceType: 'pipeline_artifact', releaseId: contextQuery.data?.latestReleasesByWorkload[workload.id]?.id, buildArtifactId: contextQuery.data?.latestReleasesByWorkload[workload.id]?.buildArtifactId }];
    })));
  };

  function handleOpenStageConfig(stage: StageView, preferredWorkloadName?: string) {
    queryClient.invalidateQueries({ queryKey: ['workloads', applicationId, 'stage-config'] });
    setConfigStage(stage);
    const matched = preferredWorkloadName
      ? configWorkloads.find((workload) => workload.name === preferredWorkloadName || workload.displayName === preferredWorkloadName)
      : undefined;
    setSelectedConfigWorkloadId(matched?.id || configWorkloads[0]?.id || '');
    configForm.setFieldsValue(workloadConfigFormValues());
  }

  const runtimeTopology: RuntimeTopology = useMemo(() => buildRuntimeTopology(runtimeResources), [runtimeResources]);
  const runtimeSummary: RuntimeSummary = useMemo(() => computeRuntimeSummary(runtimeResources), [runtimeResources]);

  function handleCloseRuntimeDrawer() {
    setRuntimeStage(null);
    setRuntimeResources([]);
    setRuntimeLoading(false);
    setRuntimeStreamStatus('');
    setRuntimeError('');
    setExpandedControllers({});
    setLogPanel(null);
    setTerminalPanel(null);
  }

  useEffect(() => {
    if (!configStage) return;
    if (!configWorkloads.some((workload) => workload.id === selectedConfigWorkloadId)) {
      setSelectedConfigWorkloadId(configWorkloads[0]?.id || '');
    }
  }, [configStage, configWorkloads, selectedConfigWorkloadId]);

  useEffect(() => {
    if (!configStage || !selectedConfigWorkloadId) return;
    const currentConfig = (stageConfigQuery.data || []).find((item: WorkloadStageConfig) => item.stageKey === configStage.stageKey);
    configForm.setFieldsValue(workloadConfigFormValues(currentConfig));
  }, [configForm, configStage, selectedConfigWorkloadId, stageConfigQuery.data]);

  useEffect(() => {
    if (!runtimeStage) {
      setRuntimeResources([]);
      setRuntimeStreamStatus('');
      setRuntimeError('');
      return;
    }
    let closed = false;
    setRuntimeResources([]);
    setRuntimeLoading(true);
    setRuntimeStreamStatus('连接中');
    setRuntimeError('');
    const close = streamRuntimeResources(applicationId, runtimeStage.stageKey, (items) => {
      if (closed) return;
      setRuntimeResources(items);
      setRuntimeLoading(false);
      setRuntimeError('');
    }, (status) => {
      if (closed) return;
      setRuntimeStreamStatus(runtimeStreamStatusText(status));
      if (status === 'error' || status === 'closed' || status === 'agent_offline') {
        setRuntimeLoading(false);
        setRuntimeError('运行资源数据加载失败，请稍后重试');
      } else {
        setRuntimeError('');
      }
    });
    return () => {
      closed = true;
      close();
    };
  }, [applicationId, runtimeStage?.stageKey]);

  useEffect(() => {
    if (!logPanel) {
      setRuntimeLogs('');
      setLogStreamStatus('');
      return;
    }
    setRuntimeLogs('');
    setLogStreamStatus('连接中');
    const close = streamRuntimePodLogs(
      applicationId,
      logPanel.resource.stageKey,
      logPanel.resource.namespace,
      logPanel.resource.name,
      logPanel.container,
      (text) => setRuntimeLogs((current) => current + text),
      (status) => setLogStreamStatus(runtimeStreamStatusText(status))
    );
    return close;
  }, [applicationId, logPanel]);

  useEffect(() => {
    terminalConnectionRef.current?.close();
    terminalConnectionRef.current = null;
    if (!terminalPanel) {
      setTerminalStatus('disconnected');
      setTerminalOutput('');
      setTerminalInput('');
      return;
    }
    setTerminalStatus('connecting');
    setTerminalOutput('');
    setTerminalInput('');
    terminalConnectionRef.current = openRuntimePodTerminal(applicationId, terminalPanel.resource.stageKey, terminalPanel.resource.namespace, terminalPanel.resource.name, terminalPanel.container, {
      onOpen: () => setTerminalStatus('connected'),
      onMessage: (text) => setTerminalOutput((current) => current + text),
      onClose: () => setTerminalStatus('disconnected'),
      onError: () => {
        setTerminalStatus('forbidden');
        setTerminalOutput((current) => current || '无权限或终端连接失败\n');
      }
    });
    return () => {
      terminalConnectionRef.current?.close();
      terminalConnectionRef.current = null;
    };
  }, [applicationId, terminalPanel]);

  const handleCreateFreight = () => {
    createFreightMutation.mutate({
      name: draftName.trim() || `freight-${Date.now()}`,
      items: enabledWorkloads.map((workload) => {
        const item = draftItems[workload.id];
        return { workloadId: workload.id, sourceType: item.sourceType, releaseId: item.releaseId, buildArtifactId: item.buildArtifactId, imageRef: item.imageRef };
      })
    });
  };

  const openRuntimeLogPanel = (resource: RuntimeResource) => setLogPanel({ resource, container: resource.containers?.[0]?.name });
  const openRuntimeTerminalPanel = (resource: RuntimeResource) => setTerminalPanel({ resource, container: resource.containers?.[0]?.name });
  const sendTerminalInput = () => {
    const text = terminalInput;
    if (!text.trim() || terminalStatus !== 'connected') return;
    terminalConnectionRef.current?.send(text);
    setTerminalInput('');
  };

  const timelineSection = (
    <section className="workspace-section-card promotion-timeline-card">
      <div className="workspace-section-head">
        <div className="workspace-section-title"><InboxOutlined /><Typography.Text strong>1 发布包</Typography.Text></div>
        <Button
          type="primary"
          aria-label="创建 Freight"
          icon={<PlusOutlined />}
          disabled={contextQuery.isLoading}
          onClick={() => {
            queryClient.invalidateQueries({ queryKey: ['freight-creation-context', applicationId] });
            setDrawerOpen(true);
          }}
        />
      </div>
      <div className="workspace-section-body">
        {freightsQuery.isLoading ? <Spin /> : (
          <div className="freight-timeline" aria-label="Freight 时间轴">
            {sortedFreights.length === 0 ? <Empty description="暂无 Freight" /> : sortedFreights.map((freight) => (
              <article
                key={freight.id}
                className={draggingFreightId === freight.id ? 'freight-timeline-card workspace-item-card workspace-item-card--freight dragging' : 'freight-timeline-card workspace-item-card workspace-item-card--freight'}
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
                <div className="freight-stage-rail" aria-label="发布包标识色" />
                <div className="freight-card-head">
                  <Typography.Text strong data-testid="freight-name">{freight.version}</Typography.Text>
                </div>
                <Space className="freight-card-actions">
                  <Button
                    aria-label="审批"
                    icon={<CheckCircleOutlined />}
                    className="nodrag nopan"
                    onMouseDown={(event) => event.stopPropagation()}
                    onClick={(event) => { event.stopPropagation(); handleOpenApproval(freight); }}
                  />
                  <Popconfirm
                    title="归档 Freight"
                    description="归档后将从发布包时间轴隐藏，历史发布记录仍保留。"
                    okText="归档"
                    cancelText="取消"
                    okButtonProps={{ danger: true, loading: archiveFreightMutation.isPending }}
                    onConfirm={(event) => {
                      event?.stopPropagation();
                      archiveFreightMutation.mutate(freight.id);
                    }}
                    onCancel={(event) => event?.stopPropagation()}
                  >
                    <Button
                      aria-label="归档"
                      danger
                      icon={<DeleteOutlined />}
                      className="nodrag nopan"
                      onMouseDown={(event) => event.stopPropagation()}
                      onClick={(event) => event.stopPropagation()}
                    />
                  </Popconfirm>
                </Space>
              </article>
            ))}
          </div>
        )}
      </div>
    </section>
  );

  const dagSection = (
    <section className="deployment-dag-canvas" aria-label="应用部署 DAG">
      {appStagesQuery.isLoading ? <Spin /> : (
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          nodesDraggable={false}
          nodesConnectable={false}
          panOnDrag
          panOnScroll={false}
          zoomOnScroll
          zoomOnDoubleClick
          zoomOnPinch
          selectNodesOnDrag={false}
          ariaLabelConfig={{
            'controls.ariaLabel': '画布控制',
            'controls.zoomIn.ariaLabel': '放大',
            'controls.zoomOut.ariaLabel': '缩小',
            'controls.fitView.ariaLabel': '适配视图',
            'controls.interactive.ariaLabel': '切换交互'
          }}
          fitView
        >
          <Controls showInteractive={false} position="bottom-right" />
          <Background />
        </ReactFlow>
      )}
    </section>
  );

  const publishAlert = publishResult ? <Alert className="form-alert" type="success" showIcon message={publishResult} /> : null;
  const workspaceContent = mode === 'applicationWorkspace' ? (
    <div className="application-delivery-workspace">
      <div className="application-delivery-left" data-testid="delivery-workspace-left">
        {timelineSection}
        {leftAfterTimeline}
      </div>
      <div className="application-delivery-right" data-testid="delivery-workspace-right">
        {dagSection}
        {publishAlert}
      </div>
    </div>
  ) : (
    <>
      <div className="promotion-workspace">
        <div className="promotion-main-column">
          {timelineSection}
          {dagSection}
        </div>
      </div>
      {publishAlert}
    </>
  );

  const podStatusIcon = (phase: PodPhase) => {
    switch (phase) {
      case 'Running':
        return <CheckCircleOutlined className="runtime-pod-status-icon runtime-pod-status-running" style={{ color: '#52c41a' }} aria-label="运行中" />;
      case 'Pending':
        return <ClockCircleOutlined className="runtime-pod-status-icon runtime-pod-status-pending" style={{ color: '#1677ff' }} aria-label="启动中" />;
      case 'Succeeded':
        return <CheckCircleFilled className="runtime-pod-status-icon runtime-pod-status-succeeded" style={{ color: '#52c41a' }} aria-label="已完成" />;
      case 'Failed':
        return <CloseCircleOutlined className="runtime-pod-status-icon runtime-pod-status-failed" style={{ color: '#ff4d4f' }} aria-label="失败" />;
      default:
        return <QuestionCircleOutlined className="runtime-pod-status-icon runtime-pod-status-unknown" style={{ color: '#8c8c8c' }} aria-label="未知" />;
    }
  };

  const renderPodRow = (pod: RuntimeResource) => {
    const phase = normalizePodPhase(pod.status);
    const isRunning = phase === 'Running';
    const ready = formatPodReady(pod);
    const restarts = sumRestartCount(pod);
    const { text: messageText, truncated } = truncateMessage(pod.message);
    const disabledTip = 'Pod 当前未处于运行中状态，暂不可执行该操作';
    const logButton = (
      <Button size="small" aria-label="日志" disabled={!isRunning} onClick={() => openRuntimeLogPanel(pod)}>日志</Button>
    );
    const terminalButton = (
      <Button size="small" aria-label="终端" disabled={!isRunning} onClick={() => openRuntimeTerminalPanel(pod)}>终端</Button>
    );
    return (
      <div className="runtime-pod-row" key={pod.id}>
        <div className="runtime-pod-row-main">
          {podStatusIcon(phase)}
          <Typography.Text strong className="runtime-pod-name">{pod.name && pod.name.trim() ? pod.name : '-'}</Typography.Text>
          <Typography.Text type="secondary">就绪 {ready ?? '-'}</Typography.Text>
          <Typography.Text type="secondary">重启 {restarts}</Typography.Text>
        </div>
        {messageText && (
          <Typography.Text type="secondary" className="runtime-pod-message">
            {truncated ? <Tooltip title={pod.message}>{messageText}…（已截断）</Tooltip> : messageText}
          </Typography.Text>
        )}
        <Space size={6} className="runtime-pod-actions">
          {isRunning ? logButton : <Tooltip title={disabledTip}><span>{logButton}</span></Tooltip>}
          {isRunning ? terminalButton : <Tooltip title={disabledTip}><span>{terminalButton}</span></Tooltip>}
        </Space>
      </div>
    );
  };

  const renderControllerCard = (group: ControllerGroup) => {
    const controller = group.controller;
    const expanded = !!expandedControllers[controller.id];
    const images = (controller.containers || [])
      .map((container) => container.image)
      .filter((image): image is string => !!image && image.trim() !== '');
    const healthValue = controller.healthStatus || controller.status || '';
    const healthColor = runtimeTagColor(healthValue);
    const badgeColor = !healthColor || healthColor === 'default' ? '#bfbfbf' : healthColor;
    const messageTrim = (controller.message || '').trim();
    const kindText = controller.kind && controller.kind.trim() ? controller.kind : '—';
    const nameText = controller.name && controller.name.trim() ? controller.name : '—';
    const toggle = () => setExpandedControllers((current) => ({ ...current, [controller.id]: !current[controller.id] }));
    return (
      <Card key={controller.id} size="small" className="runtime-controller-card">
        <div className="runtime-controller-head">
          <Space size={8} align="center" wrap>
            <Button
              type="text"
              size="small"
              aria-label={expanded ? '折叠' : '展开'}
              aria-expanded={expanded}
              icon={expanded ? <DownOutlined /> : <RightOutlined />}
              onClick={toggle}
            />
            <Tag color={runtimeKindColor(controller.kind)}>{kindText}</Tag>
            <Typography.Text strong>{nameText}</Typography.Text>
            <Badge color={badgeColor} text={healthValue || '未知'} />
            <Typography.Text type="secondary">副本 {formatControllerReplicas(controller)}</Typography.Text>
          </Space>
          <Space size={6}>
            <Button
              size="small"
              aria-label="重启"
              icon={<ReloadOutlined />}
              loading={restartRuntimeMutation.isPending}
              disabled={restartRuntimeMutation.isPending}
              onClick={() => restartRuntimeMutation.mutate(controller)}
            >重启</Button>
            <Button
              size="small"
              aria-label="设置"
              icon={<SettingOutlined />}
              onClick={() => runtimeStage && handleOpenStageConfig(runtimeStage, controller.name)}
            >设置</Button>
          </Space>
        </div>
        <div className="runtime-controller-meta">
          <div className="runtime-controller-images">
            {images.length
              ? images.map((image, index) => <Typography.Text key={`${controller.id}-image-${index}`} type="secondary" className="runtime-controller-image">{image}</Typography.Text>)
              : <Typography.Text type="secondary">—</Typography.Text>}
          </div>
          {messageTrim && <Typography.Text type="secondary" className="runtime-controller-message">{messageTrim}</Typography.Text>}
        </div>
        {expanded && (
          <div className="runtime-controller-pods">
            {group.pods.length
              ? group.pods.map(renderPodRow)
              : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无关联 Pod" />}
          </div>
        )}
      </Card>
    );
  };

  const renderTopologyContent = () => (
    <div className="runtime-topology">
      {runtimeTopology.controllers.map(renderControllerCard)}
      {runtimeTopology.uncategorized.length > 0 && (
        <div className="runtime-uncategorized-group">
          <Typography.Text strong className="runtime-uncategorized-title">未归类资源</Typography.Text>
          {runtimeTopology.uncategorized.map((resource) => (
            <div className="runtime-uncategorized-row" key={resource.id}>
              <Tag color={runtimeKindColor(resource.kind)}>{resource.kind && resource.kind.trim() ? resource.kind : '—'}</Tag>
              <Typography.Text className="runtime-uncategorized-name">{resource.name && resource.name.trim() ? resource.name : '-'}</Typography.Text>
              <Tag color={runtimeTagColor(resource.status)}>{uncategorizedStatusText(resource)}</Tag>
            </div>
          ))}
        </div>
      )}
    </div>
  );

  const renderSummaryStatValue = (value: number) => {
    if (runtimeLoading && runtimeResources.length === 0) return <Spin size="small" />;
    if (runtimeError && runtimeResources.length === 0) return <Typography.Text type="secondary">数据不可用</Typography.Text>;
    return <Statistic value={value} />;
  };

  const runtimeSummaryStats = (
    <div className="runtime-summary-cards">
      <Card size="small" className="runtime-summary-card">
        <div className="runtime-summary-card-title">控制器</div>
        {renderSummaryStatValue(runtimeSummary.controllerTotal)}
      </Card>
      <Card size="small" className="runtime-summary-card">
        <div className="runtime-summary-card-title">运行中 Pod</div>
        {renderSummaryStatValue(runtimeSummary.runningPodCount)}
      </Card>
    </div>
  );

  const runtimeHasTopology = runtimeTopology.controllers.length > 0 || runtimeTopology.uncategorized.length > 0;
  let runtimeTopologyBody: ReactNode;
  if (runtimeLoading && !runtimeError && runtimeResources.length === 0) {
    runtimeTopologyBody = <Empty description="暂无运行资源，等待快照更新" />;
  } else if (runtimeLoading && !runtimeError) {
    runtimeTopologyBody = <div className="runtime-topology-loading"><Spin /></div>;
  } else if (runtimeError) {
    runtimeTopologyBody = (
      <Space direction="vertical" size={12} className="full-width">
        <Alert type="error" showIcon message={runtimeError} />
        {runtimeHasTopology && renderTopologyContent()}
      </Space>
    );
  } else if (!runtimeHasTopology) {
    runtimeTopologyBody = <Empty description="暂无运行资源" />;
  } else {
    runtimeTopologyBody = renderTopologyContent();
  }

  return (
    <div data-testid={showHeader ? undefined : 'promotion-confirm-panel'}>
      {workspaceContent}

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
        <Button key="cancel" aria-label="取消" onClick={() => setConfigStage(null)}>取消</Button>,
        <Button key="save" aria-label="保存" type="primary" disabled={configWorkloads.length === 0 || !selectedConfigWorkloadId} loading={stageConfigMutation.isPending} onClick={() => stageConfigMutation.mutate()}>保存</Button>
      ]}>
        {configWorkloads.length === 0 ? <Empty description="暂无工作负载" /> : (
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
        )}
      </Modal>

      <Drawer title={`${runtimeStage?.stageKey || ''} 运行资源`} open={!!runtimeStage} width={980} onClose={handleCloseRuntimeDrawer}>
        <Space direction="vertical" size={16} className="full-width">
          <Descriptions size="small" column={2} items={[
            { key: 'stage', label: 'Stage', children: runtimeStage?.displayName || runtimeStage?.stageKey || '-' },
            { key: 'cluster', label: '绑定集群', children: runtimeStage?.boundClusterName || '未绑定' },
            { key: 'sync', label: 'Argo CD 同步', children: <Tag color={runtimeTagColor(runtimeStage?.syncStatus)}>{runtimeStage?.syncStatus || '未知'}</Tag> },
            { key: 'health', label: '健康状态', children: <Tag color={runtimeTagColor(runtimeStage?.healthStatus)}>{runtimeStage?.healthStatus || '未知'}</Tag> }
          ]} />
          <Space className="full-width" align="center" style={{ justifyContent: 'space-between' }}>
            <Typography.Text strong>K8s 资源</Typography.Text>
            {runtimeStreamStatus && <Tag color={runtimeStreamStatusColor(runtimeStreamStatus)}>{runtimeStreamStatus}</Tag>}
          </Space>
          {runtimeSummaryStats}
          {runtimeTopologyBody}
        </Space>
      </Drawer>

      <Drawer title={logPanel ? `${logPanel.resource.name} 日志` : 'Pod 日志'} open={!!logPanel} width={780} destroyOnHidden onClose={() => setLogPanel(null)}>
        <Space direction="vertical" size={12} className="full-width runtime-stream-panel">
          <Descriptions size="small" column={2} items={[
            { key: 'pod', label: 'Pod', children: logPanel?.resource.name || '-' },
            { key: 'namespace', label: '命名空间', children: logPanel?.resource.namespace || '-' },
            { key: 'container', label: '容器', children: logPanel?.container || '-' },
            { key: 'status', label: '连接状态', children: <Tag color={runtimeStreamStatusColor(logStreamStatus)}>{logStreamStatus || '连接中'}</Tag> }
          ]} />
          <pre className="terminal-log runtime-log-output" role="log">{runtimeLogs || '等待日志输出...'}</pre>
        </Space>
      </Drawer>

      <Drawer title={terminalPanel ? `${terminalPanel.resource.name} 终端` : 'Pod 终端'} open={!!terminalPanel} width={780} destroyOnHidden onClose={() => setTerminalPanel(null)}>
        <Space direction="vertical" size={12} className="full-width runtime-stream-panel">
          <Descriptions size="small" column={2} items={[
            { key: 'pod', label: 'Pod', children: terminalPanel?.resource.name || '-' },
            { key: 'namespace', label: '命名空间', children: terminalPanel?.resource.namespace || '-' },
            { key: 'container', label: '容器', children: terminalPanel?.container || '-' },
            { key: 'status', label: '连接状态', children: <Tag color={terminalStatusColor(terminalStatus)}>{terminalStatusText(terminalStatus)}</Tag> }
          ]} />
          {terminalStatus === 'forbidden' && <Alert type="warning" showIcon message="无权限或终端连接失败，请确认后端已授权该 Pod 终端访问。" />}
          <pre className="terminal-log runtime-terminal-output" role="log">{terminalOutput || terminalStatusText(terminalStatus)}</pre>
          <Input.Search
            aria-label="终端输入"
            enterButton={<Button aria-label="发送">发送</Button>}
            value={terminalInput}
            disabled={terminalStatus !== 'connected'}
            placeholder={terminalStatus === 'connected' ? '输入命令后发送' : terminalStatusText(terminalStatus)}
            onChange={(event) => setTerminalInput(event.target.value)}
            onSearch={sendTerminalInput}
          />
        </Space>
      </Drawer>

      <Drawer
        title="创建 Freight"
        open={drawerOpen}
        width={980}
        destroyOnHidden
        onClose={() => setDrawerOpen(false)}
        extra={(
          <Space>
            <Button
              aria-label="取消"
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
                setDrawerOpen(false);
              }}
            >取消</Button>
            <Button type="primary" aria-label="创建 Freight" disabled={submitDisabled} loading={createFreightMutation.isPending} onClick={handleCreateFreight}>创建 Freight</Button>
          </Space>
        )}
      >
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
      onClick={() => data.onOpenRuntime(stage)}
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
        { key: 'sync', label: 'Argo CD 同步', children: <Tag color={runtimeTagColor(stage.syncStatus)}>{stage.syncStatus || '未知'}</Tag> },
        { key: 'health', label: '健康状态', children: <Tag color={runtimeTagColor(stage.healthStatus)}>{stage.healthStatus || '未知'}</Tag> },
        { key: 'upstream', label: '上游 Stage', children: stage.upstreamStageKeys?.length ? stage.upstreamStageKeys.join('、') : '无' },
        { key: 'verification', label: '验证状态', children: stage.requiresVerification ? '需要验证' : '无需验证' }
      ]} />
      {stage.runtimeMessage && <Typography.Text className="stage-runtime-message" type="secondary" ellipsis>{stage.runtimeMessage}</Typography.Text>}
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
                  {item.bundleImages?.length ? <Tag color="blue">多集群适配</Tag> : null}
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
    { title: '说明', key: 'desc', render: (_: unknown, workload: Workload) => <Typography.Text type="secondary">{workload.description || workload.name}</Typography.Text> }
  ];
}

function draftItemComplete(item?: FreightDraftItem) {
  if (!item) return false;
  if (item.sourceType === 'pipeline_artifact') return !!item.releaseId;
  return !!item.imageRef?.trim();
}

function timeValue(value: string) {
  if (value === '刚刚') return Number.MAX_SAFE_INTEGER;
  const parsed = new Date(value).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
}

function currentVerificationFreight(stage: StageView | null, freights: Freight[]) {
	if (!stage) return null;
	return freights.find((freight) => freight.version === stage.currentFreightVersion) || freights[0] || null;
}

function runtimeTagColor(status?: string) {
	const value = (status || '').toLowerCase();
	if (value === 'healthy' || value === 'synced' || value === 'succeeded') return 'green';
	if (value === 'degraded' || value === 'failed' || value === 'outofsync') return 'red';
	if (value === 'progressing' || value === 'running') return 'blue';
	if (value === 'terminating') return 'orange';
	if (value === 'warning') return 'orange';
	return 'default';
}

function runtimeKindColor(kind?: string) {
  if (kind === 'Pod') return 'blue';
  if (isRestartableRuntimeKind(kind || '')) return 'purple';
  return 'default';
}

function isRestartableRuntimeKind(kind: string) {
  return ['Deployment', 'StatefulSet', 'DaemonSet'].includes(kind);
}

function runtimeStreamStatusText(status: string) {
  const map: Record<string, string> = {
    connecting: '连接中',
    connected: '已连接',
    refreshing: '刷新中',
    agent_offline: 'Agent 未连接',
    reconnecting: '重连中',
    closed: '已断开',
    error: '连接异常',
    'mock-streaming': '已连接'
  };
  return map[status] || status || '未知';
}

function runtimeStreamStatusColor(status?: string) {
  if (status === '已连接') return 'green';
  if (status === '连接中' || status === '重连中') return 'blue';
  if (status === '连接异常' || status === '加载失败') return 'red';
  if (status === '已断开') return 'default';
  return 'default';
}

function terminalStatusText(status: TerminalStatus) {
  const map: Record<TerminalStatus, string> = {
    connecting: '连接中',
    connected: '已连接',
    disconnected: '已断开',
    forbidden: '无权限'
  };
  return map[status];
}

function terminalStatusColor(status: TerminalStatus) {
  if (status === 'connected') return 'green';
  if (status === 'connecting') return 'blue';
  if (status === 'forbidden') return 'red';
  return 'default';
}

function withStageDefaults(stage: AppStage, freights: Freight[], current: Record<string, string>): StageView {
  const fallback = freights[0]?.version || '-';
  const defaults: Record<string, Partial<StageView>> = {
    dev: { replicasSummary: '1 / 1 / 1', domainSummary: 'dev.example.com', configSummary: 'dev values' },
    test: { replicasSummary: '1 / 1 / 1', domainSummary: 'test.example.com', configSummary: 'test values' },
    staging: { replicasSummary: '2 / 2 / 1', domainSummary: 'staging.example.com', configSummary: 'staging values' },
    prod: { replicasSummary: '2 / 4 / 2', domainSummary: 'prod.example.com', configSummary: 'prod values' }
  };
  return { ...defaults[stage.stageKey], ...stage, id: stage.deliveryStageId || stage.stageKey, name: stage.stageKey, color: stage.color, currentFreightVersion: current[stage.stageKey] || stage.currentFreightVersion || fallback };
}

function bundleSummary(images?: ImageBundleImage[]) {
  return images?.length ? '流水线产物 · 多集群适配' : '流水线产物';
}

function stageDropState(stage: StageView, freightId: string, stageEligibility?: Record<string, string[]>): 'ready' | 'blocked' {
  if (stage.status === 'disabled' || !stage.boundClusterId) return 'blocked';
  if (!stageEligibility) return 'ready';
  const keys = [stage.deliveryStageId, stage.id, stage.stageKey].filter(Boolean) as string[];
  const eligibleIds = new Set(keys.flatMap((key) => stageEligibility[key] || []));
  return eligibleIds.has(freightId) ? 'ready' : 'blocked';
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
