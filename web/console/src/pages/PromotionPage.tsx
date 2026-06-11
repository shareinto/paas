import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, Descriptions, Drawer, Empty, Form, Input, Modal, Radio, Select, Space, Spin, Tag, Typography } from 'antd';
import { useParams } from 'react-router-dom';
import { createFreight, createPromotion, getFreight, getFreightCreationContext, listEligibleFreights, listFreights, type CreateFreightInput, type Freight, type FreightItem, type StageDefinition } from '../api';
import { PageHeader } from '../components/PageHeader';

const DEFAULT_APPLICATION_ID = 'app_1';

type FreightDraftItem = {
  sourceType: 'pipeline_artifact' | 'custom_image';
  releaseId?: string;
  buildArtifactId?: string;
  imageRef?: string;
};

export function PromotionPage() {
  const { id } = useParams();
  const applicationId = id || DEFAULT_APPLICATION_ID;
  const queryClient = useQueryClient();
  const [activeStage, setActiveStage] = useState<StageDefinition | null>(null);
  const [selectedFreight, setSelectedFreight] = useState<Freight | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [draftName, setDraftName] = useState('');
  const [draftItems, setDraftItems] = useState<Record<string, FreightDraftItem>>({});

  const freightsQuery = useQuery({ queryKey: ['freights', applicationId], queryFn: () => listFreights(applicationId) });
  const contextQuery = useQuery({ queryKey: ['freight-creation-context', applicationId], queryFn: () => getFreightCreationContext(applicationId) });
  const eligibleMutation = useMutation({ mutationFn: (stageId: string) => listEligibleFreights(applicationId, stageId) });
  const freightDetailMutation = useMutation({
    mutationFn: (freightId: string) => getFreight(freightId),
    onSuccess: (freight) => setSelectedFreight(freight)
  });
  const createPromotionMutation = useMutation({
    mutationFn: (input: { freightId: string; targetEnvironmentId: string }) => createPromotion(input),
    onSuccess: () => setSelectedFreight(null)
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

  const stages = contextQuery.data?.stages || [];
  const eligibleIds = useMemo(() => {
    if (activeStage && contextQuery.data?.stageEligibility[activeStage.id]) {
      return new Set(contextQuery.data.stageEligibility[activeStage.id]);
    }
    return new Set((eligibleMutation.data || []).map((item) => item.id));
  }, [activeStage, contextQuery.data, eligibleMutation.data]);
  const sortedFreights = useMemo(() => [...(freightsQuery.data || [])].sort((a, b) => timeValue(a.createdAt) - timeValue(b.createdAt)), [freightsQuery.data]);
  const enabledWorkloads = contextQuery.data?.enabledWorkloads || [];
  const workloadNameById = useMemo(() => Object.fromEntries(enabledWorkloads.map((workload) => [workload.id, workload.displayName || workload.name])), [enabledWorkloads]);
  const submitDisabled = enabledWorkloads.length === 0 || enabledWorkloads.some((workload) => !draftItemComplete(draftItems[workload.id]));

  const handleStagePublish = (stage: StageDefinition) => {
    setActiveStage(stage);
    setSelectedFreight(null);
    eligibleMutation.mutate(stage.id);
  };

  const handleSelectFreight = (freight: Freight) => {
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

  const handleCreateFreight = () => {
    createFreightMutation.mutate({
      name: draftName.trim() || `freight-${Date.now()}`,
      items: enabledWorkloads.map((workload) => {
        const item = draftItems[workload.id];
        return {
          workloadId: workload.id,
          sourceType: item.sourceType,
          releaseId: item.releaseId,
          buildArtifactId: item.buildArtifactId,
          imageRef: item.imageRef
        };
      })
    });
  };

  return (
    <>
      <PageHeader title="发布晋级" extra={contextQuery.isLoading ? null : <Button type="primary" aria-label="创建 Freight" onClick={() => setDrawerOpen(true)}>创建 Freight</Button>} />
      <Typography.Paragraph type="secondary">按完整 Freight 在 dev、test、staging、prod 中流转。</Typography.Paragraph>

      <div className="promotion-layout">
        <Card title="Freight 时间轴" className="promotion-timeline-card">
          {freightsQuery.isLoading ? <Spin /> : (
            <div className="freight-timeline" aria-label="Freight 时间轴">
              {sortedFreights.length === 0 ? <Empty description="暂无 Freight" /> : sortedFreights.map((freight) => {
                const eligible = !!activeStage && eligibleIds.has(freight.id);
                const disabled = !!activeStage && !eligible;
                return (
                  <button
                    key={freight.id}
                    type="button"
                    className={`freight-timeline-card ${eligible ? 'eligible' : ''} ${disabled ? 'disabled' : ''}`}
                    data-testid="freight-card"
                    data-eligible={activeStage ? String(eligible) : undefined}
                    disabled={disabled || !activeStage}
                    aria-label={`选择 Freight ${freight.version}`}
                    onClick={() => handleSelectFreight(freight)}
                  >
                    <div className="freight-card-head">
                      <Typography.Text strong data-testid="freight-name">{freight.version}</Typography.Text>
                      <Tag color={eligible ? 'blue' : 'default'}>{eligible ? '可发布' : '待选择 Stage'}</Tag>
                    </div>
                    <div className="muted">{freight.createdAt}</div>
                    <div className="freight-card-items">
                      {(freight.items || []).map((item) => (
                        <div key={item.id} className="freight-card-item">
                          <span>{item.workloadDisplayName}</span>
                          <Typography.Text ellipsis>{item.image}</Typography.Text>
                        </div>
                      ))}
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
                <Space direction="vertical" size={2}>
                  <Typography.Text strong>{stage.name}</Typography.Text>
                  <Typography.Text type="secondary">{stage.name === 'prod' ? '生产环境' : '标准环境'}</Typography.Text>
                </Space>
                <Button aria-label="发布" type={activeStage?.id === stage.id ? 'primary' : 'default'} onClick={() => handleStagePublish(stage)} loading={activeStage?.id === stage.id && eligibleMutation.isPending}>发布</Button>
              </div>
              {stage.name === 'prod' ? (
                <Alert type="warning" showIcon message="生产发布需要审批" description="审批人数、审批人范围和禁止自审批会在确认发布时再次展示。" />
              ) : (
                <Alert type="info" showIcon message="从此 Stage 发起发布" description="点击发布后，仅当前 Stage 可发布的 Freight 会点亮。" />
              )}
            </Card>
          ))}
        </div>

        <Card title="近期发布记录" className="compact-card">
          <div className="promotion-history-row">
            <Space direction="vertical" size={2}>
              <Typography.Text strong>生产审批</Typography.Text>
              <Typography.Text type="secondary">最新生产发布待审批，禁止发起人自审批。</Typography.Text>
            </Space>
            <Button>回滚</Button>
          </div>
        </Card>
      </div>

      <Modal
        title="确认发布"
        open={!!selectedFreight}
        okText="确认发布"
        cancelText="取消"
        confirmLoading={createPromotionMutation.isPending}
        onCancel={() => setSelectedFreight(null)}
        onOk={() => selectedFreight && activeStage && createPromotionMutation.mutate({ freightId: selectedFreight.id, targetEnvironmentId: activeStage.environmentId })}
        width={820}
      >
        {selectedFreight && (
          <Space direction="vertical" size={14} className="full-width">
            <Descriptions size="small" column={2} items={[
              { key: 'freight', label: 'Freight', children: selectedFreight.version },
              { key: 'stage', label: '目标 Stage', children: activeStage?.name || '-' }
            ]} />
            {activeStage?.name === 'prod' && (
              <Alert
                type="warning"
                showIcon
                message="生产发布审批"
                description={(
                  <Space direction="vertical" size={2}>
                    <span>审批人数：至少 {activeStage.approvalCount || 2} 人</span>
                    <span>审批人范围：{activeStage.approverScope || '生产审批人'}</span>
                    {activeStage.selfApprovalForbidden !== false && <span>禁止发起人自审批</span>}
                  </Space>
                )}
              />
            )}
            <div className="confirm-workload-list">
              {withWorkloadDisplayNames(selectedFreight.items || [], workloadNameById).map((item) => (
                <div key={item.id} className="confirm-workload-row">
                  <Typography.Text strong>{item.workloadDisplayName}</Typography.Text>
                  <Typography.Text copyable>{item.image}</Typography.Text>
                  <Tag color={item.sourceType === 'custom_image' ? 'orange' : 'green'}>{item.sourceType === 'custom_image' ? '自定义镜像' : '流水线产物'}</Tag>
                </div>
              ))}
            </div>
          </Space>
        )}
      </Modal>

      <Drawer
        title="创建 Freight"
        open={drawerOpen}
        width={760}
        onClose={() => setDrawerOpen(false)}
        extra={<Button type="primary" aria-label="创建" disabled={submitDisabled} loading={createFreightMutation.isPending} onClick={handleCreateFreight}>创建</Button>}
      >
        <Space direction="vertical" size={16} className="full-width">
          <Form layout="vertical">
            <Form.Item label="Freight 名称">
              <Input value={draftName} onChange={(event) => setDraftName(event.target.value)} placeholder="请输入 Freight 名称" />
            </Form.Item>
          </Form>
          <Alert type="info" showIcon message="Freight 必须覆盖所有启用 Workload" />
          {enabledWorkloads.map((workload) => {
            const draft = draftItems[workload.id] || { sourceType: 'pipeline_artifact' };
            const release = contextQuery.data?.latestReleasesByWorkload[workload.id];
            const tagRisk = draft.sourceType === 'custom_image' && !!draft.imageRef && !draft.imageRef.includes('@sha256:') && /:[^/]+$/.test(draft.imageRef);
            return (
              <Card key={workload.id} size="small" title={workload.displayName} className="freight-draft-workload">
                <Radio.Group value={draft.sourceType} onChange={(event) => updateDraftItem(workload.id, { sourceType: event.target.value, releaseId: undefined, buildArtifactId: undefined, imageRef: '' })}>
                  <Radio aria-label={`${workload.displayName}流水线产物`} value="pipeline_artifact">流水线产物</Radio>
                  <Radio aria-label={`${workload.displayName}自定义镜像`} value="custom_image">自定义镜像</Radio>
                </Radio.Group>
                {draft.sourceType === 'pipeline_artifact' ? (
                  <>
                    <Select
                      className="full-width freight-source-control"
                      placeholder="请选择镜像版本"
                      value={draft.releaseId}
                      notFoundContent="暂无可选镜像版本"
                      options={release ? [{ value: release.id, label: release.image, title: release.image }] : []}
                      onChange={(value) => updateDraftItem(workload.id, { releaseId: value, buildArtifactId: release?.buildArtifactId })}
                    />
                    {release && (
                      <button type="button" className="freight-release-option" title={release.image} onClick={() => updateDraftItem(workload.id, { releaseId: release.id, buildArtifactId: release.buildArtifactId })}>
                        {release.image}
                      </button>
                    )}
                  </>
                ) : (
                  <>
                    <Input className="freight-source-control" placeholder="请输入完整镜像地址" value={draft.imageRef} onChange={(event) => updateDraftItem(workload.id, { imageRef: event.target.value })} />
                    {tagRisk && <Alert className="freight-risk-alert" type="warning" showIcon message="镜像 tag 可能被覆盖，建议使用 digest。" />}
                  </>
                )}
              </Card>
            );
          })}
        </Space>
      </Drawer>
    </>
  );
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
  return items.map((item) => ({
    ...item,
    workloadDisplayName: workloadNameById[item.workloadId] || item.workloadDisplayName || item.workloadName || item.workloadId
  }));
}
