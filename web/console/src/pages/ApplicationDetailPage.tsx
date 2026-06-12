import { DeleteOutlined, EditOutlined, PlusOutlined, PlayCircleOutlined, StopOutlined } from '@ant-design/icons';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Badge, Button, Card, Descriptions, Empty, Form, Input, InputNumber, Modal, Popconfirm, Select, Segmented, Space, Spin, Steps, Tabs, Tag, Typography, message } from 'antd';
import { type ReactNode, useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import {
  createBuildPipeline,
  createWorkload,
  cancelBuild,
  deleteBuildPipeline,
  deleteWorkload,
  getApplication,
  listApplicationBuilds,
  listApplicationEnvironments,
  listBuildEnvironments,
  listBuildPipelines,
  listRuntimeEnvironments,
  listSourceRepositories,
  listWorkloadEnvironmentConfigs,
  listWorkloads,
  saveWorkloadEnvironmentConfig,
  triggerBuildPipeline,
  updateBuildPipeline,
  updateWorkload,
  type BuildPipeline,
  type BuildRun,
  type Environment,
  type RuntimeEnvironment,
  type Workload,
  type WorkloadEnvironmentConfig,
  type WorkloadImageSourceMode,
  type WorkloadType
} from '../api';
import { BuildLogViewer } from '../components/BuildLogViewer';
import { PageHeader } from '../components/PageHeader';
import { PromotionContent } from './PromotionPage';

const EMPTY_LIST: any[] = [];
const WORKLOAD_TYPE_OPTIONS = [
  { label: 'Deployment', value: 'deployment' },
  { label: 'StatefulSet', value: 'statefulset' }
];
const IMAGE_SOURCE_OPTIONS = [
  { label: '流水线产物', value: 'pipeline_artifact' },
  { label: '发布时选择自定义镜像', value: 'custom_image' },
  { label: '混合来源', value: 'mixed' },
  { label: '暂不绑定', value: 'none' }
];
const DELIVERY_FLOW_STEPS = ['创建 Workload', '配置环境差异', '创建完整 Freight', '选择目标 Stage', '发布晋级', '回滚历史 Freight'];
const WORKLOAD_WIZARD_STEPS = ['基础信息', '镜像来源', '运行参数', '网络访问', '配置与目录', '预览校验'];

export function ApplicationDetailPage() {
  const { id = 'app_1' } = useParams();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState('build');
  const [createdPipelines, setCreatedPipelines] = useState<BuildPipeline[]>([]);
  const { data: app } = useQuery({ queryKey: ['application', id], queryFn: () => getApplication(id), enabled: !!id });

  return (
    <>
      <PageHeader
        title={app?.displayName || '应用详情'}
        breadcrumb={[
          { title: <Link to="/projects">项目</Link> },
          { title: app?.projectId ? <Link to={`/projects/${app.projectId}`}>{app?.project || app.projectId}</Link> : (app?.project || '-') },
          { title: app?.displayName || '应用详情' }
        ]}
        extra={(
          <Space>
            <Button icon={<EditOutlined />} onClick={() => navigate(`/apps/${id}/edit`)}>编辑应用</Button>
          </Space>
        )}
      />
      <Card className="summary-card">
        <Descriptions column={4} size="small" items={[
          { key: 'name', label: '应用标识', children: app?.name || '-' },
          { key: 'project', label: '所属项目', children: app?.project || app?.projectId || '-' },
          { key: 'type', label: '应用类型', children: <Tag color="blue">{app?.type || '-'}</Tag> },
          { key: 'status', label: '状态', children: <Badge status={app?.status === 'disabled' ? 'default' : 'success'} text={app?.status === 'disabled' ? '已禁用' : '启用'} /> }
        ]} />
      </Card>
      <DeliveryFlow />
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        className="detail-tabs"
        items={[
        { key: 'build', label: '构建', children: <BuildTab applicationId={id} projectId={app?.projectId} localPipelines={createdPipelines} onPipelineChanged={(pipeline) => setCreatedPipelines((current) => [pipeline, ...current.filter((item) => item.id !== pipeline.id)])} onPipelineDeleted={(pipelineId) => setCreatedPipelines((current) => current.filter((item) => item.id !== pipelineId))} /> },
        { key: 'deploy', label: '部署', children: <PromotionContent applicationId={id} /> }
      ]} />
    </>
  );
}

function DeliveryFlow() {
  return (
    <div className="delivery-flow" aria-label="交付流程">
      {DELIVERY_FLOW_STEPS.map((step, index) => (
        <div key={step} className="delivery-flow-step">
          <span className={index === 0 ? 'delivery-flow-node active' : 'delivery-flow-node'}>{step}</span>
          {index < DELIVERY_FLOW_STEPS.length - 1 && <span className="delivery-flow-arrow">→</span>}
        </div>
      ))}
    </div>
  );
}

function BuildTab({ applicationId, projectId, localPipelines, onPipelineChanged, onPipelineDeleted }: { applicationId: string; projectId?: string; localPipelines?: BuildPipeline[]; onPipelineChanged?: (pipeline: BuildPipeline) => void; onPipelineDeleted?: (pipelineId: string) => void }) {
  return (
    <div data-testid="build-tab" className="build-tab-layout">
      <BuildPipelinePanel applicationId={applicationId} projectId={projectId} localPipelines={localPipelines} onPipelineChanged={onPipelineChanged} onPipelineDeleted={onPipelineDeleted} />
      <WorkloadPanel applicationId={applicationId} />
    </div>
  );
}

function WorkloadPanel({ applicationId }: { applicationId: string }) {
  const [createOpen, setCreateOpen] = useState(false);
  const [editingWorkload, setEditingWorkload] = useState<Workload | null>(null);
  const queryClient = useQueryClient();
  const { data: workloads = EMPTY_LIST, isLoading } = useQuery({ queryKey: ['workloads', applicationId], queryFn: () => listWorkloads(applicationId), enabled: !!applicationId });
  const configQueries = useQueries({
    queries: (workloads as Workload[]).map((workload) => ({
      queryKey: ['workload-environment-configs', applicationId, workload.id, 'summary'],
      queryFn: () => listWorkloadEnvironmentConfigs(applicationId, workload.id),
      enabled: !!applicationId && !!workload.id
    }))
  });
  const configByWorkloadId = Object.fromEntries((workloads as Workload[]).map((workload, index) => [workload.id, (configQueries[index]?.data || []) as WorkloadEnvironmentConfig[]]));
  const deleteMutation = useMutation({
    mutationFn: (workloadId: string) => deleteWorkload(applicationId, workloadId),
    onSuccess: (workload) => {
      message.success('Workload 已删除');
      queryClient.setQueryData<Workload[]>(['workloads', applicationId], (current = []) => current.filter((item) => item.id !== workload.id));
      queryClient.invalidateQueries({ queryKey: ['workloads', applicationId], refetchType: 'none' });
      if (editingWorkload?.id === workload.id) setEditingWorkload(null);
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除 Workload 失败')
  });

  return (
    <section data-testid="workload-panel" className="resource-section">
      <div className="section-heading">
        <div>
          <Typography.Title level={4}>工作负载管理</Typography.Title>
          <Typography.Text type="secondary">按最小可独立部署单元管理 Workload。</Typography.Text>
        </div>
        <Button type="primary" aria-label="创建 Workload" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>创建 Workload</Button>
      </div>
      {isLoading ? <Spin /> : (workloads as Workload[]).length === 0 ? <Empty description="暂无 Workload" /> : (
        <div className="resource-card-grid">
          {(workloads as Workload[]).map((item) => (
            <article key={item.id} className="resource-card">
              <div className="resource-card-head">
                <Space direction="vertical" size={2}>
                  <Typography.Text strong>{item.name}</Typography.Text>
                  <Typography.Text type="secondary">{item.displayName || item.description || '-'}</Typography.Text>
                </Space>
                <Badge status={item.status === 'enabled' ? 'success' : 'default'} text={item.status === 'enabled' ? '启用' : '停用'} />
              </div>
              <div className="resource-meta-grid">
                <MetaItem label="类型" value={<Tag color="blue">{workloadTypeText(item.workloadType)}</Tag>} />
                <MetaItem label="镜像来源" value={<Space direction="vertical" size={0}><span>{imageSourceText(item.imageSourceMode)}</span><Typography.Text type="secondary">{item.imageSourceName || '-'}</Typography.Text></Space>} />
                <MetaItem label="端口" value={portSummary(configByWorkloadId[item.id])} />
                <MetaItem label="域名" value={domainSummary(configByWorkloadId[item.id])} />
              </div>
              <div className="workload-env-status">
                {(item.envStatuses || []).length === 0 ? <Typography.Text type="secondary">暂无环境状态</Typography.Text> : item.envStatuses.map((env) => <Tag key={env.envName} color={env.healthStatus === '健康' ? 'green' : 'default'}>{env.envName} {env.syncStatus}</Tag>)}
              </div>
              <div className="resource-card-actions">
                <Button aria-label="编辑" icon={<EditOutlined />} onClick={() => setEditingWorkload(item)}>编辑</Button>
                <Popconfirm
                  title="删除 Workload"
                  description={`确认删除 ${item.name}？删除后不会进入新的 Freight。`}
                  okText="确认删除"
                  cancelText="取消"
                  onConfirm={() => deleteMutation.mutate(item.id)}
                >
                  <Button danger aria-label="删除" icon={<DeleteOutlined />} loading={deleteMutation.isPending}>删除</Button>
                </Popconfirm>
              </div>
            </article>
          ))}
        </div>
      )}
      <WorkloadWizardModal applicationId={applicationId} open={createOpen} onClose={() => setCreateOpen(false)} />
      <WorkloadWizardModal applicationId={applicationId} workload={editingWorkload} open={!!editingWorkload} onClose={() => setEditingWorkload(null)} />
    </section>
  );
}

function WorkloadWizardModal({ applicationId, workload, open, onClose }: { applicationId: string; workload?: Workload | null; open: boolean; onClose: () => void }) {
  const [form] = Form.useForm();
  const [step, setStep] = useState(0);
  const initializedRef = useRef(false);
  const queryClient = useQueryClient();
  const isEditing = !!workload;
  const imageSourceMode = Form.useWatch('imageSourceMode', form);
  const { data: environments = EMPTY_LIST } = useQuery({ queryKey: ['application-environments', applicationId], queryFn: () => listApplicationEnvironments(applicationId), enabled: open });
  const { data: configs = EMPTY_LIST } = useQuery({ queryKey: ['workload-environment-configs', applicationId, workload?.id, 'wizard'], queryFn: () => listWorkloadEnvironmentConfigs(applicationId, workload!.id), enabled: open && !!workload });
  const mutation = useMutation({
    mutationFn: async () => {
      await form.validateFields();
      const values = form.getFieldsValue(true);
      const saved = isEditing
        ? await updateWorkload(applicationId, workload!.id, values)
        : await createWorkload(applicationId, values);
      if (values.environmentId) {
        await saveWorkloadEnvironmentConfig(applicationId, saved.id, values.environmentId, wizardConfigPayload(values));
      }
      return saved;
    },
    onSuccess: (workload) => {
      message.success(isEditing ? 'Workload 已保存' : 'Workload 已创建');
      queryClient.setQueryData<Workload[]>(['workloads', applicationId], (current = []) => [workload, ...current.filter((item) => item.id !== workload.id)]);
      queryClient.invalidateQueries({ queryKey: ['workloads', applicationId], refetchType: 'none' });
      queryClient.invalidateQueries({ queryKey: ['workload-environment-configs', applicationId, workload.id], refetchType: 'none' });
      onClose();
      form.resetFields();
      setStep(0);
    },
    onError: (error) => message.error(error instanceof Error ? error.message : (isEditing ? '保存 Workload 失败' : '创建 Workload 失败'))
  });

  useEffect(() => {
    if (!open) {
      form.resetFields();
      setStep(0);
      initializedRef.current = false;
      return;
    }
    if (initializedRef.current) return;
    const firstConfig = (configs as WorkloadEnvironmentConfig[])[0];
    const firstEnv = (environments as Environment[])[0];
    form.setFieldsValue({
      name: workload?.name,
      displayName: workload?.displayName,
      description: workload?.description,
      workloadType: workload?.workloadType || 'deployment',
      imageSourceMode: workload?.imageSourceMode || 'pipeline_artifact',
      environmentId: firstConfig?.environmentId || firstEnv?.id,
      replicas: firstConfig?.replicas ?? 1,
      containerPort: firstConfig?.servicePorts?.[0]?.targetPort || 8080,
      servicePort: firstConfig?.servicePorts?.[0]?.port || 80,
      domain: firstConfig?.ingressHosts?.[0]?.host || '',
      envVarsText: (firstConfig?.envVars || []).map((item) => `${item.name}=${item.value}`).join('\n'),
      configPath: firstConfig?.configFiles?.[0]?.mountPath || '',
      configContent: firstConfig?.configFiles?.[0]?.content || '',
      writableDir: firstConfig?.writableDirs?.[0]?.mountPath || '',
      writableDirSize: firstConfig?.writableDirs?.[0]?.sizeLimit || ''
    });
    initializedRef.current = true;
  }, [configs, environments, form, open, workload]);

  useEffect(() => {
    if (!open || form.getFieldValue('environmentId')) return;
    const firstEnv = (environments as Environment[])[0];
    if (firstEnv?.id) form.setFieldValue('environmentId', firstEnv.id);
  }, [environments, form, open]);

  const footer = (
    <Space>
      <Button onClick={onClose}>取消</Button>
      {step > 0 && <Button onClick={() => setStep((current) => current - 1)}>上一步</Button>}
      {step < WORKLOAD_WIZARD_STEPS.length - 1 ? (
        <Button type="primary" onClick={() => setStep((current) => current + 1)}>下一步</Button>
      ) : (
        <Button type="primary" aria-label={isEditing ? '保存' : '创建'} loading={mutation.isPending} onClick={() => mutation.mutate()}>{isEditing ? '保存' : '创建'}</Button>
      )}
    </Space>
  );

  return (
    <Modal title={isEditing ? '编辑 Workload' : '创建 Workload'} open={open} onCancel={onClose} width={980} destroyOnHidden footer={footer}>
      <Steps size="small" current={step} items={WORKLOAD_WIZARD_STEPS.map((title) => ({ title }))} />
      <div className="workload-create-grid">
        <Form form={form} layout="vertical" className="workload-create-form" preserve>
          {step === 0 && <><Form.Item label="Workload 标识" name="name" rules={[{ required: true, message: '请输入 Workload 标识' }, { pattern: /^[a-z][a-z0-9-]{0,62}$/, message: '仅支持小写字母、数字和连字符' }]}><Input placeholder="order-api" disabled={isEditing} /></Form.Item><Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}><Input placeholder="订单接口" /></Form.Item><Form.Item label="Workload 类型" name="workloadType" rules={[{ required: true, message: '请选择 Workload 类型' }]}><Segmented aria-label="Workload 类型" options={WORKLOAD_TYPE_OPTIONS} /></Form.Item><Form.Item label="描述" name="description"><Input.TextArea rows={2} /></Form.Item></>}
          {step === 1 && <><Form.Item label="镜像来源偏好" name="imageSourceMode" rules={[{ required: true, message: '请选择镜像来源偏好' }]}><Select options={IMAGE_SOURCE_OPTIONS} /></Form.Item>{imageSourceMode === 'custom_image' && <Alert showIcon type="warning" message="自定义镜像只保存来源偏好，实际镜像版本在创建 Freight 时选择。" description="镜像 tag 可能被覆盖，创建 Freight 时建议使用 digest。" />}</>}
          {step === 2 && <><Form.Item label="环境" name="environmentId" rules={[{ required: true, message: '请选择环境' }]}><Select options={(environments as Environment[]).map((item) => ({ value: item.id, label: `${item.displayName || item.name} (${item.name})` }))} /></Form.Item><Form.Item label="副本数" name="replicas"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item><Form.Item label="容器端口" name="containerPort"><InputNumber min={1} max={65535} precision={0} style={{ width: '100%' }} /></Form.Item><Form.Item label="Service 端口" name="servicePort"><InputNumber min={1} max={65535} precision={0} style={{ width: '100%' }} /></Form.Item></>}
          {step === 3 && <><Form.Item label="访问域名" name="domain"><Input placeholder="dev-order.example.com" /></Form.Item></>}
          {step === 4 && <><Form.Item label="环境变量" name="envVarsText"><Input.TextArea rows={4} placeholder="SPRING_PROFILES_ACTIVE=dev" /></Form.Item><Form.Item label="配置文件路径" name="configPath"><Input placeholder="/app/config/application.yaml" /></Form.Item><Form.Item label="配置文件内容" name="configContent"><Input.TextArea rows={4} /></Form.Item><Form.Item label="可写目录" name="writableDir"><Input placeholder="/data" /></Form.Item><Form.Item label="目录容量" name="writableDirSize"><Input placeholder="5Gi" /></Form.Item></>}
          {step === 5 && <Input.TextArea readOnly rows={10} value={valuesPreviewFromForm(form.getFieldsValue())} />}
        </Form>
        <aside className="workload-check-panel">
          <Typography.Title level={5}>校验清单</Typography.Title>
          <Alert showIcon type="success" message="基础信息完整" />
          <Alert showIcon type={imageSourceMode === 'custom_image' ? 'warning' : 'success'} message={imageSourceMode === 'custom_image' ? '自定义镜像将在 Freight 中选择' : '默认使用流水线产物'} />
          <Alert showIcon type="success" message="环境配置将保存到选中环境" />
        </aside>
      </div>
    </Modal>
  );
}

function MetaItem({ label, value }: { label: string; value: ReactNode }) {
  return <div className="resource-meta-item"><span>{label}</span><div>{value}</div></div>;
}

function wizardConfigPayload(values: any): Partial<WorkloadEnvironmentConfig> {
  const containerPort = Number(values.containerPort || 0);
  const servicePort = Number(values.servicePort || containerPort || 0);
  return {
    replicas: Number(values.replicas ?? 1),
    servicePorts: containerPort ? [{ name: 'http', port: servicePort || containerPort, targetPort: containerPort, protocol: 'TCP' }] : [],
    resourceRequests: {},
    resourceLimits: {},
    probes: [],
    ingressHosts: values.domain ? [{ host: values.domain, path: '/', servicePort: 'http', tls: false }] : [],
    envVars: parseEnvVars(values.envVarsText),
    configFiles: values.configPath ? [{ mountPath: values.configPath, content: values.configContent || '' }] : [],
    writableDirs: values.writableDir ? [{ mountPath: values.writableDir, sizeLimit: values.writableDirSize || '' }] : []
  };
}

function parseEnvVars(text?: string): { name: string; value: string }[] {
  return String(text || '').split('\n').map((line) => line.trim()).filter(Boolean).map((line) => {
    const [name, ...rest] = line.split('=');
    return { name: name.trim(), value: rest.join('=').trim() };
  }).filter((item) => item.name);
}

function valuesPreviewFromForm(values: any) {
  return valuesPreview({ name: values.name, workloadType: values.workloadType || 'deployment' } as Workload, wizardConfigPayload(values) as WorkloadEnvironmentConfig);
}

function workloadTypeText(type: WorkloadType) {
  return type === 'statefulset' ? 'StatefulSet' : 'Deployment';
}

function imageSourceText(mode: WorkloadImageSourceMode) {
  if (mode === 'custom_image') return '自定义镜像';
  if (mode === 'mixed') return '混合来源';
  if (mode === 'none') return '暂不绑定';
  return '流水线产物';
}

function portSummary(configs: WorkloadEnvironmentConfig[] = []) {
  const ports = configs.flatMap((config) => config.servicePorts || []);
  if (ports.length === 0) return <Typography.Text type="secondary">暂无端口</Typography.Text>;
  const unique = Array.from(new Set(ports.map((port) => `${port.targetPort || port.port}/${port.protocol || 'TCP'}`)));
  return <Space wrap size={[4, 4]}>{unique.map((item) => <Tag key={item}>{item}</Tag>)}</Space>;
}

function domainSummary(configs: WorkloadEnvironmentConfig[] = []) {
  const hosts = configs.flatMap((config) => config.ingressHosts || []).map((host) => host.host).filter(Boolean);
  if (hosts.length === 0) return <Tag>集群内访问</Tag>;
  return <Space wrap size={[4, 4]}>{Array.from(new Set(hosts)).map((host) => <Tag color="green" key={host}>{host}</Tag>)}</Space>;
}

function valuesPreview(workload: Workload | null, config?: WorkloadEnvironmentConfig) {
  return [
    'workload:',
    `  name: ${workload?.name || '-'}`,
    `  type: ${workload ? workloadTypeText(workload.workloadType) : 'Deployment'}`,
    `  replicas: ${config?.replicas ?? 1}`,
    'image:',
    '  source: freight',
    'service:',
    `  enabled: ${(config?.servicePorts || []).length > 0}`,
    'ingress:',
    `  enabled: ${(config?.ingressHosts || []).length > 0}`,
    'configFiles:',
    ...((config?.configFiles || []).length ? (config?.configFiles || []).map((file) => `  - mountPath: ${file.mountPath}`) : ['  []']),
    'writableDirs:',
    ...((config?.writableDirs || []).length ? (config?.writableDirs || []).map((dir) => `  - mountPath: ${dir.mountPath}`) : ['  []'])
  ].join('\n');
}

function BuildPipelinePanel({ applicationId, projectId, localPipelines = [], onPipelineChanged, onPipelineDeleted }: { applicationId: string; projectId?: string; localPipelines?: BuildPipeline[]; onPipelineChanged?: (pipeline: BuildPipeline) => void; onPipelineDeleted?: (pipelineId: string) => void }) {
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [editingPipeline, setEditingPipeline] = useState<BuildPipeline | null>(null);
  const [historyPipeline, setHistoryPipeline] = useState<BuildPipeline | null>(null);
  const { data: pipelines = EMPTY_LIST, isLoading } = useQuery({ queryKey: ['build-pipelines', applicationId], queryFn: () => listBuildPipelines(applicationId), enabled: !!applicationId, staleTime: 1000 });
  const { data: workloads = EMPTY_LIST } = useQuery({ queryKey: ['workloads', applicationId, 'pipeline-cards'], queryFn: () => listWorkloads(applicationId), enabled: !!applicationId });
  const workloadNameById = Object.fromEntries((workloads as Workload[]).map((item) => [item.id, item.displayName || item.name]));
  const visiblePipelines = mergePipelines(localPipelines, pipelines as BuildPipeline[]);
  const deleteMutation = useMutation({
    mutationFn: deleteBuildPipeline,
    onSuccess: (_, pipelineId) => {
      message.success('流水线已删除');
      onPipelineDeleted?.(pipelineId);
      queryClient.setQueryData<BuildPipeline[]>(['build-pipelines', applicationId], (current = []) => current.filter((item) => item.id !== pipelineId));
      queryClient.invalidateQueries({ queryKey: ['build-pipelines', applicationId] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除流水线失败')
  });

  return (
    <section data-testid="pipeline-panel" className="resource-section">
      <div className="section-heading">
        <div>
          <Typography.Title level={4}>流水线</Typography.Title>
          <Typography.Text type="secondary">流水线绑定 Workload、代码源和运行时环境。</Typography.Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>创建流水线</Button>
      </div>
      {isLoading ? <Spin /> : visiblePipelines.length === 0 ? <Empty description="暂无流水线" /> : (
        <div className="resource-card-grid">
          {visiblePipelines.map((item) => (
            <article key={item.id} className="resource-card">
              <div className="resource-card-head">
                <Space direction="vertical" size={2}>
                  <Typography.Text strong>{item.displayName || item.name}</Typography.Text>
                  <Typography.Text type="secondary">{item.name}</Typography.Text>
                </Space>
                <Badge status={item.status === 'active' ? 'success' : 'default'} text={item.status === 'active' ? '启用' : '停用'} />
              </div>
              <div className="resource-meta-grid">
                <MetaItem label="绑定 Workload" value={workloadNameById[item.workloadId || ''] || item.workloadId || '-'} />
                <MetaItem label="代码源" value={<Space wrap size={[4, 4]}>{(item.sources || []).map((source) => <Tag key={source.key}>{source.displayName || source.key}</Tag>)}</Space>} />
                <MetaItem label="运行时环境" value={<Space wrap size={[4, 4]}>{(item.runtimeEnvironments || []).map((runtime) => <Tag key={runtime.id}>{runtime.name}</Tag>)}</Space>} />
                <MetaItem label="更新时间" value={item.updatedAt || '-'} />
              </div>
              <div className="resource-card-actions">
                <Button icon={<PlayCircleOutlined />} onClick={() => setHistoryPipeline(item)}>触发构建</Button>
                <Button icon={<EditOutlined />} onClick={() => setEditingPipeline(item)}>编辑</Button>
                <Button danger icon={<DeleteOutlined />} loading={deleteMutation.isPending} onClick={() => deleteMutation.mutate(item.id)}>删除</Button>
              </div>
            </article>
          ))}
        </div>
      )}
      <CreatePipelineModal
        applicationId={applicationId}
        projectId={projectId}
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onSaved={(pipeline) => {
          onPipelineChanged?.(pipeline);
          setCreateOpen(false);
        }}
      />
      <CreatePipelineModal
        applicationId={applicationId}
        projectId={projectId}
        open={!!editingPipeline}
        editingPipeline={editingPipeline}
        onClose={() => setEditingPipeline(null)}
        onSaved={(pipeline) => {
          onPipelineChanged?.(pipeline);
          setEditingPipeline(null);
        }}
      />
      <BuildHistoryModal applicationId={applicationId} pipeline={historyPipeline} onClose={() => setHistoryPipeline(null)} />
    </section>
  );
}

function BuildHistoryModal({ applicationId, pipeline, onClose }: { applicationId: string; pipeline: BuildPipeline | null; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [selectedBuild, setSelectedBuild] = useState<BuildRun | null>(null);
  const buildsQueryKey = ['application-builds', applicationId, pipeline?.id];
  const { data: builds = EMPTY_LIST, isLoading } = useQuery({ queryKey: buildsQueryKey, queryFn: () => listApplicationBuilds(applicationId), enabled: !!pipeline });
  const filteredBuilds = ((builds as BuildRun[]) || [])
    .filter((build) => !pipeline || build.pipelineId === pipeline.id)
    .map((build, index) => ({ build, index }))
    .sort((left, right) => {
      const timeDelta = buildStartedAtValue(right.build.startedAt) - buildStartedAtValue(left.build.startedAt);
      return timeDelta || left.index - right.index;
    })
    .map((item) => item.build);
  const activeBuild = filteredBuilds.find(isUnfinishedBuild);
  const triggerMutation = useMutation({
    mutationFn: () => triggerBuildPipeline(pipeline!.id, { gitRef: pipeline?.sources?.[0]?.defaultRef || 'main' }),
    onSuccess: (run) => {
      setSelectedBuild(run);
      queryClient.setQueryData<BuildRun[]>(buildsQueryKey, (current = []) => [run, ...current.filter((item) => item.id !== run.id)]);
      queryClient.invalidateQueries({ queryKey: buildsQueryKey });
      message.success('构建已触发');
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '触发构建失败')
  });
  const cancelMutation = useMutation({
    mutationFn: (buildRunId: string) => cancelBuild(buildRunId),
    onSuccess: (run) => {
      setSelectedBuild(run);
      queryClient.setQueryData<BuildRun[]>(buildsQueryKey, (current = []) => current.map((item) => item.id === run.id ? run : item));
      queryClient.invalidateQueries({ queryKey: buildsQueryKey });
      message.success('构建已取消');
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '取消构建失败')
  });
  const updateBuildStatusFromStream = useCallback((buildRunId: string, status: string) => {
    const displayStatus = buildStatusFromStream(status);
    if (!displayStatus) return;
    const patchBuild = (build: BuildRun) => build.id === buildRunId ? { ...build, status: displayStatus } : build;
    setSelectedBuild((current) => current?.id === buildRunId ? patchBuild(current) : current);
    queryClient.setQueryData<BuildRun[]>(buildsQueryKey, (current = []) => current.map(patchBuild));
  }, [buildsQueryKey, queryClient]);

  useEffect(() => {
    setSelectedBuild(null);
  }, [pipeline?.id]);

  useEffect(() => {
    if (selectedBuild || filteredBuilds.length === 0) return;
    setSelectedBuild(filteredBuilds[0]);
  }, [filteredBuilds, selectedBuild]);

  return (
    <Modal title={`${pipeline?.displayName || pipeline?.name || ''} 构建历史`} open={!!pipeline} onCancel={onClose} width={1180} footer={<Button onClick={onClose}>关闭</Button>} destroyOnHidden>
      <div className="build-history-modal-toolbar">
        <Space direction="vertical" size={0}>
          <Typography.Text strong>历史构建</Typography.Text>
          <Typography.Text type="secondary">{activeBuild ? '当前有未完成构建，完成或取消后可再次触发。' : '可直接触发新的流水线构建。'}</Typography.Text>
        </Space>
        <Button
          type="primary"
          aria-label="触发构建"
          icon={<PlayCircleOutlined />}
          disabled={!!activeBuild}
          loading={triggerMutation.isPending}
          onClick={() => triggerMutation.mutate()}
        >
          触发构建
        </Button>
      </div>
      {isLoading ? <Spin /> : filteredBuilds.length === 0 ? (
        <div className="build-history-empty">
          <Empty description="暂无构建记录" />
          <BuildLogViewer />
        </div>
      ) : (
        <div className="build-history-layout">
          <div className="build-history-list">
            <Typography.Text strong>构建时间</Typography.Text>
            {filteredBuilds.map((build, index) => (
              <button key={build.id} type="button" className={selectedBuild?.id === build.id ? 'build-history-item active' : 'build-history-item'} onClick={() => setSelectedBuild(build)}>
                <span>构建 {filteredBuilds.length - index}</span>
                <span>{build.startedAt}</span>
                <Tag color={buildStatusColor(build.status)}>{build.status}</Tag>
              </button>
            ))}
          </div>
          <div className="build-history-log-panel">
            <div className="build-history-log-actions">
              <Space>
                {selectedBuild && <Tag color={buildStatusColor(selectedBuild.status)}>{selectedBuild.status}</Tag>}
                {selectedBuild && isUnfinishedBuild(selectedBuild) && (
                  <Button danger icon={<StopOutlined />} loading={cancelMutation.isPending} onClick={() => cancelMutation.mutate(selectedBuild.id)}>取消构建</Button>
                )}
              </Space>
            </div>
            <BuildLogViewer buildRunId={selectedBuild?.id} onStatusChange={(status) => selectedBuild?.id && updateBuildStatusFromStream(selectedBuild.id, status)} />
          </div>
        </div>
      )}
    </Modal>
  );
}

function isUnfinishedBuild(build: BuildRun) {
  return ['构建中', '排队中', '运行中', 'queued', 'running'].includes(build.status);
}

function buildStatusColor(status: string) {
  if (['成功', 'succeeded'].includes(status)) return 'green';
  if (['失败', 'failed'].includes(status)) return 'red';
  if (['已取消', '已中止', 'aborted'].includes(status)) return 'default';
  if (['不稳定', 'unstable'].includes(status)) return 'orange';
  return 'blue';
}

function buildStatusFromStream(status: string) {
  const map: Record<string, string> = {
    queued: '构建中',
    running: '构建中',
    succeeded: '成功',
    failed: '失败',
    aborted: '已取消',
    unstable: '不稳定'
  };
  return map[status] || '';
}

function buildStartedAtValue(value?: string) {
  if (!value) return 0;
  if (value === '刚刚') return Number.MAX_SAFE_INTEGER;
  const timestamp = Date.parse(value.includes('T') ? value : value.replace(' ', 'T'));
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

function CreatePipelineModal({ applicationId, projectId, open, onClose, editingPipeline, onSaved }: { applicationId: string; projectId?: string; open: boolean; onClose: () => void; editingPipeline?: BuildPipeline | null; onSaved?: (pipeline: BuildPipeline) => void }) {
  const [form] = Form.useForm();
  const queryClient = useQueryClient();
  const initializedRef = useRef(false);
  const isEditing = !!editingPipeline;
  const { data: repositories = EMPTY_LIST } = useQuery({ queryKey: ['source-repositories', projectId], queryFn: () => listSourceRepositories(projectId), enabled: open });
  const { data: workloads = EMPTY_LIST } = useQuery({ queryKey: ['workloads', applicationId, 'pipeline-modal'], queryFn: () => listWorkloads(applicationId), enabled: open && !!applicationId });
  const { data: buildEnvironments = EMPTY_LIST } = useQuery({ queryKey: ['build-environments'], queryFn: listBuildEnvironments, enabled: open });
  const { data: runtimeEnvironments = EMPTY_LIST } = useQuery({ queryKey: ['runtime-environments'], queryFn: listRuntimeEnvironments, enabled: open });
  const { data: pipelines = EMPTY_LIST, isFetched: pipelinesFetched } = useQuery({ queryKey: ['build-pipelines', applicationId], queryFn: () => listBuildPipelines(applicationId), enabled: open && !!applicationId });
  const selectedRepositoryId = Form.useWatch(['sources', 0, 'sourceRepositoryId'], form);
  const selectedRuntimeIds = Form.useWatch('runtimeEnvironmentIds', form) || [];
  const selectedRepository = repositories.find((item: any) => item.id === selectedRepositoryId);
  const selectedRuntimes = runtimeEnvironments.filter((item: any) => selectedRuntimeIds.includes(item.id));

  useEffect(() => {
    if (!open) {
      initializedRef.current = false;
      form.resetFields();
      return;
    }
    if (initializedRef.current || !repositories.length || !buildEnvironments.length || !runtimeEnvironments.length || !workloads.length) return;
    if (editingPipeline) {
      form.setFieldsValue({
        workloadId: editingPipeline.workloadId,
        name: editingPipeline.name,
        displayName: editingPipeline.displayName || editingPipeline.name,
        description: editingPipeline.description,
        runtimeEnvironmentIds: editingPipeline.runtimeEnvironments?.map((runtime) => runtime.id) || [],
        sources: (editingPipeline.sources || []).map((source) => ({
          key: source.key,
          displayName: source.displayName,
          sourceRepositoryId: source.sourceRepositoryId,
          buildEnvironmentId: source.buildEnvironmentId,
          sourcePath: source.sourcePath || source.buildSpec?.sourcePath || '.',
          defaultRef: source.defaultRef || source.buildSpec?.defaultRef || 'main',
          buildCommand: source.buildSpec?.buildCommand || '',
          artifactCopyCommand: source.buildSpec?.artifactCopyCommand || ''
        }))
      });
      initializedRef.current = true;
      return;
    }
    if (!pipelinesFetched) return;
    const runtime = runtimeEnvironments.find((item: any) => item.isDefault) || runtimeEnvironments[0];
    const buildEnv = buildEnvironments.find((item: any) => item.isDefault) || buildEnvironments[0];
    const repo = repositories.find((item: any) => item.status === 'ready') || repositories[0];
    const workload = workloads.find((item: any) => item.status === 'enabled') || workloads[0];
    const defaultPipeline = nextPipelineDefaults(pipelines as BuildPipeline[]);
    form.setFieldsValue({
      workloadId: workload?.id,
      name: defaultPipeline.name,
      displayName: defaultPipeline.displayName,
      runtimeEnvironmentIds: runtime?.id ? [runtime.id] : [],
      sources: [{
        key: 'main',
        displayName: '主代码源',
        sourceRepositoryId: repo?.id,
        buildEnvironmentId: buildEnv?.id,
        sourcePath: '.',
        defaultRef: repo?.defaultBranch || 'main',
        buildCommand: 'mvn clean package -DskipTests',
        artifactCopyCommand: 'cp -ar target/*.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"'
      }]
    });
    initializedRef.current = true;
  }, [buildEnvironments, editingPipeline, form, open, pipelines, pipelinesFetched, repositories, runtimeEnvironments, workloads]);

  const mutation = useMutation({
    mutationFn: async () => {
      const values = await form.validateFields();
      const runtimeEnvironmentIds = values.runtimeEnvironmentIds || [];
      const primaryRuntime = runtimeEnvironments.find((item: RuntimeEnvironment) => item.id === runtimeEnvironmentIds[0]);
      const sources = (values.sources || []).map((source: any, index: number) => {
        return {
          key: source.key,
          displayName: source.displayName,
          sourceRepositoryId: source.sourceRepositoryId,
          buildEnvironmentId: source.buildEnvironmentId,
          sourcePath: source.sourcePath,
          defaultRef: source.defaultRef,
          isPrimary: index === 0,
          buildSpec: {
            sourcePath: source.sourcePath,
            buildCommand: source.buildCommand,
            artifactCopyCommand: source.artifactCopyCommand,
            runtimeBaseImage: primaryRuntime?.runtimeBaseImage || '',
            artifactDeployPath: primaryRuntime?.artifactDeployPath || '',
            defaultRef: source.defaultRef
          }
        };
      });
      const pipeline = editingPipeline
        ? await updateBuildPipeline(editingPipeline.id, { workloadId: values.workloadId, displayName: values.displayName, description: values.description, runtimeEnvironmentIds, sources })
        : await createBuildPipeline(applicationId, { workloadId: values.workloadId, name: values.name, displayName: values.displayName, description: values.description, runtimeEnvironmentIds, sources });
      return { ...pipeline, sources };
    },
    onSuccess: (pipeline) => {
      message.success(editingPipeline ? '流水线已保存' : '流水线已创建');
      onSaved?.(pipeline);
      queryClient.setQueryData<BuildPipeline[]>(['build-pipelines', applicationId], (current = []) => {
        const existing = current.filter((item) => item.id !== pipeline.id);
        return [pipeline, ...existing];
      });
      queryClient.invalidateQueries({ queryKey: ['build-pipelines', applicationId], refetchType: 'none' });
      onClose();
    },
    onError: (error) => message.error(error instanceof Error ? error.message : (editingPipeline ? '保存流水线失败' : '创建流水线失败'))
  });

  return (
    <Modal title={isEditing ? '编辑构建流水线' : '创建构建流水线'} open={open} onCancel={onClose} onOk={() => mutation.mutate()} confirmLoading={mutation.isPending} width={760} okText={isEditing ? '保存' : '创建'} cancelText="取消">
      <Form form={form} layout="vertical">
        <Form.Item label="绑定 Workload" name="workloadId" rules={[{ required: true, message: '请选择绑定 Workload' }]}>
          <Select options={workloads.map((item: Workload) => ({ value: item.id, label: `${item.displayName || item.name} (${item.name})`, disabled: item.status !== 'enabled' }))} />
        </Form.Item>
        <Form.Item label="流水线标识" name="name" rules={[{ required: true, message: '请输入流水线标识' }, { pattern: /^[a-z][a-z0-9-]{0,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
          <Input placeholder="main" disabled={isEditing} />
        </Form.Item>
        <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}>
          <Input placeholder="主流水线" />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input.TextArea rows={2} />
        </Form.Item>
        <Form.Item label="运行时环境" name="runtimeEnvironmentIds" rules={[{ required: true, message: '请选择运行时环境' }]}>
          <Select
            mode="multiple"
            options={runtimeEnvironments.map((item: any) => ({
              value: item.id,
              label: `${item.name} ${item.runtimeBaseImage ? `(${item.runtimeBaseImage})` : ''}`
            }))}
          />
        </Form.Item>
        <Space wrap size={[8, 8]} style={{ marginBottom: 16 }}>
          {selectedRuntimes.length > 0 ? selectedRuntimes.map((runtime: RuntimeEnvironment, index: number) => (
            <Tag key={runtime.id} color={index === 0 ? 'blue' : 'default'}>{index === 0 ? '主产物' : '附加'} {runtime.name}</Tag>
          )) : <Typography.Text type="secondary">请选择流水线运行时环境</Typography.Text>}
        </Space>
        <Form.List name="sources">
          {(fields, { add, remove }) => (
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              {fields.map((field, index) => (
                <Card key={field.key} size="small" title={index === 0 ? '主代码源' : `代码源 ${index + 1}`} extra={<Button danger type="text" icon={<DeleteOutlined />} disabled={fields.length <= 1} onClick={() => remove(field.name)}>删除</Button>}>
                  <Form.Item label="代码源标识" name={[field.name, 'key']} rules={[{ required: true, message: '请输入代码源标识' }, { pattern: /^[a-z][a-z0-9-]{0,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
                    <Input />
                  </Form.Item>
                  <Form.Item label="显示名称" name={[field.name, 'displayName']}>
                    <Input />
                  </Form.Item>
                  <Form.Item label="源码仓库" name={[field.name, 'sourceRepositoryId']} rules={[{ required: true, message: '请选择源码仓库' }]}>
                    <Select options={repositories.map((repo: any) => ({ value: repo.id, label: repo.displayName || repo.name, disabled: repo.status !== 'ready' }))} />
                  </Form.Item>
                  <Form.Item label="默认分支" name={[field.name, 'defaultRef']} rules={[{ required: true, message: '请输入默认分支' }]}>
                    <Input placeholder={selectedRepository?.defaultBranch || 'main'} />
                  </Form.Item>
                  <Form.Item label="源码子目录" name={[field.name, 'sourcePath']} rules={[{ required: true, message: '请输入源码子目录' }]}>
                    <Input placeholder="services/order-api" />
                  </Form.Item>
                  <Form.Item label="构建环境" name={[field.name, 'buildEnvironmentId']} rules={[{ required: true, message: '请选择构建环境' }]}>
                    <Select options={buildEnvironments.map((item: any) => ({ value: item.id, label: item.name }))} />
                  </Form.Item>
                  <Form.Item label="构建命令" name={[field.name, 'buildCommand']} rules={[{ required: true, message: '请输入构建命令' }]}>
                    <Input.TextArea rows={3} />
                  </Form.Item>
                  <Form.Item label="产物拷贝命令" name={[field.name, 'artifactCopyCommand']} rules={[{ required: true, message: '请输入产物拷贝命令' }]}>
                    <Input.TextArea rows={3} />
                  </Form.Item>
                </Card>
              ))}
              <Button icon={<PlusOutlined />} onClick={() => add({ key: `source-${fields.length + 1}`, displayName: '代码源', sourcePath: '.', defaultRef: 'main' })}>添加代码源</Button>
            </Space>
          )}
        </Form.List>
      </Form>
    </Modal>
  );
}

function mergePipelines(localPipelines: BuildPipeline[], remotePipelines: BuildPipeline[]) {
  const seen = new Set<string>();
  return [...localPipelines, ...remotePipelines].filter((pipeline) => {
    if (seen.has(pipeline.id)) return false;
    seen.add(pipeline.id);
    return true;
  });
}

function nextPipelineDefaults(pipelines: BuildPipeline[]) {
  const used = new Set((pipelines || []).map((pipeline) => pipeline.name));
  if (!used.has('main')) return { name: 'main', displayName: '主流水线' };
  let index = used.size + 1;
  while (used.has(`pipeline-${index}`)) index += 1;
  return { name: `pipeline-${index}`, displayName: `流水线 ${index}` };
}
