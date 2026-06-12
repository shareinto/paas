import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Descriptions, Drawer, Empty, Form, Input, Select, Space, Spin, Table, Tag, Typography } from 'antd';
import { useParams } from 'react-router-dom';
import { createFreight, createPromotion, getFreight, getFreightCreationContext, listEligibleFreights, listFreights, type CreateFreightInput, type Freight, type FreightItem, type StageDefinition, type Workload } from '../api';
import { PageHeader } from '../components/PageHeader';

const DEFAULT_APPLICATION_ID = 'app_1';
const DELIVERY_FLOW_STEPS = ['创建 Workload', '配置环境差异', '创建完整 Freight', '选择目标 Stage', '发布晋级', '回滚历史 Freight'];

type FreightDraftItem = {
  sourceType: 'pipeline_artifact' | 'custom_image';
  releaseId?: string;
  buildArtifactId?: string;
  imageRef?: string;
};

type StageView = StageDefinition & {
  currentFreightVersion?: string;
  replicasSummary?: string;
  domainSummary?: string;
  configSummary?: string;
};

export function PromotionPage() {
  const { id } = useParams();
  const applicationId = id || DEFAULT_APPLICATION_ID;
  return <PromotionContent applicationId={applicationId} showHeader />;
}

export function PromotionContent({ applicationId = DEFAULT_APPLICATION_ID, showHeader = false }: { applicationId?: string; showHeader?: boolean }) {
  const queryClient = useQueryClient();
  const [activeStage, setActiveStage] = useState<StageView | null>(null);
  const [selectedFreight, setSelectedFreight] = useState<Freight | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [draftName, setDraftName] = useState('');
  const [draftItems, setDraftItems] = useState<Record<string, FreightDraftItem>>({});
  const [stageFreights, setStageFreights] = useState<Record<string, string>>({});
  const [publishResult, setPublishResult] = useState('');

  const freightsQuery = useQuery({ queryKey: ['freights', applicationId], queryFn: () => listFreights(applicationId) });
  const contextQuery = useQuery({ queryKey: ['freight-creation-context', applicationId], queryFn: () => getFreightCreationContext(applicationId) });
  const eligibleMutation = useMutation({ mutationFn: (stageId: string) => listEligibleFreights(applicationId, stageId) });
  const freightDetailMutation = useMutation({
    mutationFn: (freightId: string) => getFreight(freightId),
    onSuccess: (freight) => setSelectedFreight(freight)
  });
  const createPromotionMutation = useMutation({
    mutationFn: (input: { freightId: string; targetEnvironmentId: string }) => createPromotion(input),
    onSuccess: () => {
      if (activeStage && selectedFreight) {
        setStageFreights((current) => ({ ...current, [activeStage.id]: selectedFreight.version }));
        setPublishResult(`${selectedFreight.version} 已提交到 ${activeStage.name}，等待同步结果。`);
      }
    }
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
  const workloadNameById = useMemo(() => Object.fromEntries(enabledWorkloads.map((workload) => [workload.id, workload.displayName || workload.name])), [enabledWorkloads]);
  const stages = useMemo(() => (contextQuery.data?.stages || []).map((stage) => withStageDefaults(stage as StageView, sortedFreights, stageFreights)), [contextQuery.data?.stages, sortedFreights, stageFreights]);
  const eligibleIds = useMemo(() => {
    if (activeStage && contextQuery.data?.stageEligibility[activeStage.id]) return new Set(contextQuery.data.stageEligibility[activeStage.id]);
    return new Set((eligibleMutation.data || []).map((item) => item.id));
  }, [activeStage, contextQuery.data, eligibleMutation.data]);
  const selectedCount = enabledWorkloads.filter((workload) => draftItemComplete(draftItems[workload.id])).length;
  const submitDisabled = enabledWorkloads.length === 0 || selectedCount < enabledWorkloads.length;

  const handleStagePublish = (stage: StageView) => {
    setActiveStage(stage);
    setSelectedFreight(null);
    setPublishResult('');
    eligibleMutation.mutate(stage.id);
  };

  const handleRollback = () => {
    const prodStage = stages.find((stage) => stage.name === 'prod') || stages[0];
    if (prodStage) handleStagePublish(prodStage);
  };

  const handleSelectFreight = (freight: Freight) => {
    setPublishResult('');
    if (freight.items?.length) {
      setSelectedFreight(freight);
      return;
    }
    freightDetailMutation.mutate(freight.id);
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
    <>
      {showHeader && <PageHeader title="发布晋级" extra={contextQuery.isLoading ? null : <Button type="primary" aria-label="创建 Freight" onClick={() => setDrawerOpen(true)}>创建 Freight</Button>} />}
      {!showHeader && <div className="embedded-section-head"><Typography.Title level={4}>发布晋级</Typography.Title>{contextQuery.isLoading ? null : <Button type="primary" aria-label="创建 Freight" onClick={() => setDrawerOpen(true)}>创建 Freight</Button>}</div>}
      <Typography.Paragraph type="secondary">按完整 Freight 在 dev、test、staging、prod 中流转。</Typography.Paragraph>
      <DeliveryFlow />

      <div className="promotion-workspace">
        <div className="promotion-main-column">
          <Card title="Freight 时间轴" className="promotion-timeline-card">
            {freightsQuery.isLoading ? <Spin /> : (
              <div className="freight-timeline" aria-label="Freight 时间轴">
                {sortedFreights.length === 0 ? <Empty description="暂无 Freight" /> : sortedFreights.map((freight) => {
                  const eligible = !!activeStage && eligibleIds.has(freight.id);
                  const disabled = !!activeStage && !eligible;
                  return (
                    <button key={freight.id} type="button" className={`freight-timeline-card ${eligible ? 'eligible' : ''} ${disabled ? 'disabled' : ''}`} data-testid="freight-card" data-eligible={activeStage ? String(eligible) : undefined} disabled={disabled || !activeStage} aria-label={`选择 Freight ${freight.version}`} onClick={() => handleSelectFreight(freight)}>
                      <div className="freight-card-head">
                        <Typography.Text strong data-testid="freight-name">{freight.version}</Typography.Text>
                        <Tag color={eligible ? 'blue' : 'default'}>{eligible ? '可发布' : '待选择 Stage'}</Tag>
                      </div>
                      <div className="muted">{freight.createdAt}</div>
                      <div className="freight-card-items">
                        {(freight.items || []).map((item) => <div key={item.id} className="freight-card-item"><span>{item.workloadDisplayName}</span><Typography.Text ellipsis>{item.image}</Typography.Text></div>)}
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </Card>

          <div className="stage-grid">
            {stages.map((stage) => (
              <Card key={stage.id} className={activeStage?.id === stage.id ? 'stage-card active' : 'stage-card'} aria-label={`${stage.name} Stage`}>
                <div className="stage-card-head">
                  <Space direction="vertical" size={2}><Typography.Text strong>{stage.name}</Typography.Text><Typography.Text type="secondary">{stage.name === 'prod' ? '生产环境' : '标准环境'}</Typography.Text></Space>
                  <Button aria-label="发布" type={activeStage?.id === stage.id ? 'primary' : 'default'} onClick={() => handleStagePublish(stage)} loading={activeStage?.id === stage.id && eligibleMutation.isPending}>发布</Button>
                </div>
                <Descriptions size="small" column={1} items={[
                  { key: 'freight', label: '当前 Freight', children: stage.currentFreightVersion || '-' },
                  { key: 'replicas', label: '副本', children: stage.replicasSummary || '-' },
                  { key: 'domain', label: '域名', children: stage.domainSummary || '-' },
                  { key: 'config', label: '配置', children: stage.configSummary || '-' }
                ]} />
                {stage.name === 'prod' && <Tag color="orange">生产需审批</Tag>}
              </Card>
            ))}
          </div>

          <Card title="近期发布记录" className="compact-card">
            <div className="promotion-history-row">
              <Space direction="vertical" size={2}><Typography.Text strong>生产审批</Typography.Text><Typography.Text type="secondary">最新生产发布待审批，禁止发起人自审批。</Typography.Text></Space>
              <Button onClick={handleRollback}>回滚</Button>
            </div>
          </Card>
        </div>

        <PromotionConfirmPanel activeStage={activeStage} selectedFreight={selectedFreight} workloadNameById={workloadNameById} publishResult={publishResult} loading={createPromotionMutation.isPending} onConfirm={() => selectedFreight && activeStage && createPromotionMutation.mutate({ freightId: selectedFreight.id, targetEnvironmentId: activeStage.environmentId })} />
      </div>

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
    </>
  );
}

function DeliveryFlow() {
  return <div className="delivery-flow" aria-label="交付流程">{DELIVERY_FLOW_STEPS.map((step, index) => <div key={step} className="delivery-flow-step"><span className={index === 3 ? 'delivery-flow-node active' : 'delivery-flow-node'}>{step}</span>{index < DELIVERY_FLOW_STEPS.length - 1 && <span className="delivery-flow-arrow">→</span>}</div>)}</div>;
}

function PromotionConfirmPanel({ activeStage, selectedFreight, workloadNameById, publishResult, loading, onConfirm }: { activeStage: StageView | null; selectedFreight: Freight | null; workloadNameById: Record<string, string>; publishResult: string; loading: boolean; onConfirm: () => void }) {
  return (
    <aside className="promotion-confirm-panel" data-testid="promotion-confirm-panel">
      <Typography.Title level={5}>发布确认</Typography.Title>
      {!activeStage && <Typography.Text type="secondary">请先点击 Stage 上的发布按钮，再从左侧选择点亮的 Freight。</Typography.Text>}
      {activeStage && !selectedFreight && <Alert type="info" showIcon message={`发布到 ${activeStage.name}`} description="请选择一个已点亮的 Freight。" />}
      {activeStage && selectedFreight && <Space direction="vertical" size={14} className="full-width">
        <Descriptions size="small" column={1} items={[{ key: 'stage', label: '目标 Stage', children: activeStage.name }, { key: 'freight', label: '选择 Freight', children: selectedFreight.version }, { key: 'config', label: '环境配置', children: activeStage.configSummary || `${activeStage.name} values` }]} />
        {activeStage.name === 'prod' && <Alert type="warning" showIcon message="生产发布审批" description={<Space direction="vertical" size={2}><span>审批人数：至少 {activeStage.approvalCount || 2} 人</span><span>审批人范围：{activeStage.approverScope || '生产审批人'}</span>{activeStage.selfApprovalForbidden !== false && <span>禁止发起人自审批</span>}</Space>} />}
        <div className="confirm-workload-list">{withWorkloadDisplayNames(selectedFreight.items || [], workloadNameById).map((item) => <div key={item.id} className="confirm-workload-row"><Typography.Text strong>{item.workloadDisplayName}</Typography.Text><Typography.Text copyable>{item.image}</Typography.Text><Tag color={item.sourceType === 'custom_image' ? 'orange' : 'green'}>{item.sourceType === 'custom_image' ? '自定义镜像' : '流水线产物'}</Tag></div>)}</div>
        {publishResult && <Alert type="success" showIcon message={publishResult} />}
        <Button type="primary" block loading={loading} onClick={onConfirm}>确认发布</Button>
      </Space>}
    </aside>
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
      return <Select disabled={draft.sourceType !== 'pipeline_artifact'} value={draft.releaseId} placeholder="选择成功构建镜像" style={{ width: 220 }} options={release ? [{ value: release.id, label: release.image, title: release.image }] : []} onChange={(value) => updateDraftItem(workload.id, { releaseId: value, buildArtifactId: release?.buildArtifactId })} />;
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

function withWorkloadDisplayNames(items: FreightItem[], workloadNameById: Record<string, string>) {
  return items.map((item) => ({ ...item, workloadDisplayName: workloadNameById[item.workloadId] || item.workloadDisplayName || item.workloadName || item.workloadId }));
}

function withStageDefaults(stage: StageView, freights: Freight[], current: Record<string, string>): StageView {
  const fallback = freights[freights.length - 1]?.version || '-';
  const defaults: Record<string, Partial<StageView>> = {
    dev: { replicasSummary: '1 / 1 / 1', domainSummary: 'dev.example.com', configSummary: 'dev values' },
    test: { replicasSummary: '1 / 1 / 1', domainSummary: 'test.example.com', configSummary: 'test values' },
    staging: { replicasSummary: '2 / 2 / 1', domainSummary: 'staging.example.com', configSummary: 'staging values' },
    prod: { replicasSummary: '2 / 4 / 2', domainSummary: 'prod.example.com', configSummary: 'prod values' }
  };
  return { ...defaults[stage.name], ...stage, currentFreightVersion: current[stage.id] || stage.currentFreightVersion || fallback };
}
