import { DeleteOutlined, EditOutlined, PlusOutlined, PlayCircleOutlined, StopOutlined } from '@ant-design/icons';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { Badge, Button, Card, Checkbox, Empty, Form, Input, Modal, Popconfirm, Select, Segmented, Space, Spin, Tag, Typography, message } from 'antd';
import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import {
  createBuildPipeline,
  createWorkload,
  cancelBuild,
  deleteBuildPipeline,
  deleteWorkload,
  getApplication,
  getWorkloadDefaultConfig,
  listApplications,
  listApplicationBuilds,
  listBuildEnvironments,
  listBuildPipelines,
  listProjects,
  listRuntimeEnvironments,
  listSourceRepositories,
  listWorkloadEnvironmentConfigs,
  listWorkloads,
  saveWorkloadDefaultConfig,
  triggerBuildPipeline,
  updateBuildPipeline,
  updateWorkload,
  type Application,
  type BuildPipeline,
  type BuildRun,
  type Project,
  type RuntimeEnvironment,
  type Workload,
  type WorkloadEnvironmentConfig,
  type WorkloadImageSourceMode,
  type WorkloadType
} from '../api';
import { BuildLogViewer } from '../components/BuildLogViewer';
import { PromotionContent } from './PromotionPage';
import { ConfigValueLists, WorkloadRuntimeFields, workloadConfigFormValues, workloadConfigPayload } from './workloadConfigForm';

const EMPTY_LIST: any[] = [];
const WORKLOAD_TYPE_OPTIONS = [
  { label: '无状态', value: 'deployment' },
  { label: '有状态', value: 'statefulset' }
];
const IMAGE_SOURCE_OPTIONS = [
  { label: '流水线', value: 'pipeline_artifact' },
  { label: '自定义镜像', value: 'custom_image' }
];
type ApplicationSection = 'build' | 'deploy' | 'config';

export function ApplicationDetailPage() {
  return <ApplicationWorkspacePage section="build" />;
}

export function ApplicationWorkspacePage({ section }: { section: ApplicationSection }) {
  const { id = 'app_1' } = useParams();
  const [createdPipelines, setCreatedPipelines] = useState<BuildPipeline[]>([]);
  const { data: app } = useQuery({ queryKey: ['application', id], queryFn: () => getApplication(id), enabled: !!id });

  return (
    <>
      <ApplicationContextSelector applicationId={id} currentProjectId={app?.projectId} section={section} />
      {section === 'build' && (
        <BuildPipelinePanel
          applicationId={id}
          projectId={app?.projectId}
          localPipelines={createdPipelines}
          onPipelineChanged={(pipeline) => setCreatedPipelines((current) => [pipeline, ...current.filter((item) => item.id !== pipeline.id)])}
          onPipelineDeleted={(pipelineId) => setCreatedPipelines((current) => current.filter((item) => item.id !== pipelineId))}
        />
      )}
      {section === 'deploy' && <PromotionContent applicationId={id} />}
      {section === 'config' && <WorkloadPanel applicationId={id} />}
    </>
  );
}

function ApplicationContextSelector({ applicationId, currentProjectId, section }: { applicationId: string; currentProjectId?: string; section: ApplicationSection }) {
  const navigate = useNavigate();
  const [selectedProjectId, setSelectedProjectId] = useState<string>();
  const [pendingProjectId, setPendingProjectId] = useState('');
  const { data: projects = EMPTY_LIST } = useQuery({ queryKey: ['projects'], queryFn: listProjects });
  const activeProjectId = selectedProjectId || currentProjectId || (projects as Project[])[0]?.id;
  const { data: applications = EMPTY_LIST, isFetching } = useQuery({
    queryKey: ['apps', activeProjectId || 'none'],
    queryFn: () => listApplications(activeProjectId),
    enabled: !!activeProjectId
  });
  const projectOptions = useMemo(() => (projects as Project[]).map((project) => ({ value: project.id, label: project.displayName || project.name })), [projects]);
  const applicationOptions = useMemo(() => {
    const options = (applications as Application[]).map((app) => ({ value: app.id, label: app.displayName || app.name }));
    if (applicationId && !options.some((item) => item.value === applicationId)) {
      options.unshift({ value: applicationId, label: applicationId });
    }
    return options;
  }, [applicationId, applications]);

  useEffect(() => {
    if (currentProjectId) setSelectedProjectId(currentProjectId);
  }, [applicationId, currentProjectId]);

  useEffect(() => {
    if (!pendingProjectId || pendingProjectId !== activeProjectId || isFetching) return;
    const firstApp = (applications as Application[])[0];
    if (firstApp) navigate(`/apps/${firstApp.id}/${section}`);
    setPendingProjectId('');
  }, [activeProjectId, applications, isFetching, navigate, pendingProjectId, section]);

  return (
    <div className="application-context-selector">
      <Select
        aria-label="选择项目"
        placeholder="请选择项目"
        value={activeProjectId}
        options={projectOptions}
        onChange={(nextProjectId) => {
          setSelectedProjectId(nextProjectId);
          setPendingProjectId(nextProjectId);
        }}
      />
      <Select
        aria-label="选择应用"
        placeholder="请选择应用"
        value={applicationId}
        loading={isFetching}
        options={applicationOptions}
        onChange={(nextApplicationId) => navigate(`/apps/${nextApplicationId}/${section}`)}
      />
    </div>
  );
}

export function ApplicationSectionRedirect({ section }: { section: ApplicationSection }) {
  const { id } = useParams();
  return <Navigate to={id ? `/apps/${id}/${section}` : '/apps'} replace />;
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
      message.success('工作负载已删除');
      queryClient.setQueryData<Workload[]>(['workloads', applicationId], (current = []) => current.filter((item) => item.id !== workload.id));
      queryClient.invalidateQueries({ queryKey: ['workloads', applicationId], refetchType: 'none' });
      if (editingWorkload?.id === workload.id) setEditingWorkload(null);
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除工作负载失败')
  });

  return (
    <section data-testid="workload-panel" className="resource-section">
      <div className="section-heading">
        <div />
        <Button type="primary" aria-label="创建工作负载" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>创建工作负载</Button>
      </div>
      {isLoading ? <Spin /> : (workloads as Workload[]).length === 0 ? <Empty description="暂无工作负载" /> : (
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
                  title="删除工作负载"
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
  const initializedRef = useRef(false);
  const configInitializedRef = useRef(false);
  const queryClient = useQueryClient();
  const isEditing = !!workload;
  const imageSourceMode = Form.useWatch('imageSourceMode', form);
  const { data: pipelines = EMPTY_LIST } = useQuery({ queryKey: ['build-pipelines', applicationId, 'workload-wizard'], queryFn: () => listBuildPipelines(applicationId), enabled: open && !!applicationId });
  const { data: defaultConfig } = useQuery({ queryKey: ['workload-default-config', applicationId, workload?.id], queryFn: () => getWorkloadDefaultConfig(applicationId, workload!.id), enabled: open && !!workload });
  const needsPipeline = imageSourceMode === 'pipeline_artifact';
  const mutation = useMutation({
    mutationFn: async () => {
      await form.validateFields();
      const values = form.getFieldsValue(true);
      const workloadInput = {
        name: values.name,
        displayName: values.displayName,
        description: values.description,
        workloadType: values.workloadType,
        imageSourceMode: values.imageSourceMode,
        pipelineId: values.imageSourceMode === 'pipeline_artifact' ? values.pipelineId : ''
      };
      const saved = isEditing
        ? await updateWorkload(applicationId, workload!.id, workloadInput)
        : await createWorkload(applicationId, workloadInput);
      await saveWorkloadDefaultConfig(applicationId, saved.id, workloadConfigPayload(values));
      return saved;
    },
    onSuccess: (workload) => {
      message.success(isEditing ? '工作负载已保存' : '工作负载已创建');
      queryClient.setQueryData<Workload[]>(['workloads', applicationId], (current = []) => [workload, ...current.filter((item) => item.id !== workload.id)]);
      queryClient.invalidateQueries({ queryKey: ['workloads', applicationId], refetchType: 'none' });
      queryClient.invalidateQueries({ queryKey: ['workloads', applicationId, 'pipeline-cards'], refetchType: 'none' });
      queryClient.invalidateQueries({ queryKey: ['workload-default-config', applicationId, workload.id], refetchType: 'none' });
      queryClient.invalidateQueries({ queryKey: ['workload-environment-configs', applicationId, workload.id], refetchType: 'none' });
      onClose();
      form.resetFields();
    },
    onError: (error) => message.error(error instanceof Error ? error.message : (isEditing ? '保存工作负载失败' : '创建工作负载失败'))
  });

  useEffect(() => {
    if (!open) {
      form.resetFields();
      initializedRef.current = false;
      configInitializedRef.current = false;
      return;
    }
    if (initializedRef.current) return;
    form.setFieldsValue({
      name: workload?.name,
      displayName: workload?.displayName,
      description: workload?.description,
      workloadType: workload?.workloadType || 'deployment',
      imageSourceMode: workload?.imageSourceMode === 'custom_image' ? 'custom_image' : 'pipeline_artifact',
      pipelineId: workload?.pipelineId || (pipelines as BuildPipeline[])[0]?.id,
      ...workloadConfigFormValues()
    });
    initializedRef.current = true;
  }, [form, open, pipelines, workload]);

  useEffect(() => {
    if (!open || configInitializedRef.current) return;
    if (isEditing && !defaultConfig) return;
    const config = defaultConfig;
    form.setFieldsValue(config ? workloadConfigFormValues(config) : workloadConfigFormValues());
    configInitializedRef.current = true;
  }, [defaultConfig, form, isEditing, open]);

  useEffect(() => {
    if (!open || !needsPipeline || form.getFieldValue('pipelineId')) return;
    const firstPipeline = (pipelines as BuildPipeline[]).find((item) => item.status === 'active') || (pipelines as BuildPipeline[])[0];
    if (firstPipeline?.id) form.setFieldValue('pipelineId', firstPipeline.id);
  }, [form, needsPipeline, open, pipelines]);

  const footer = (
    <Space>
      <Button onClick={onClose}>取消</Button>
      <Button type="primary" aria-label={isEditing ? '保存' : '创建'} loading={mutation.isPending} onClick={() => mutation.mutate()}>{isEditing ? '保存' : '创建'}</Button>
    </Space>
  );

  return (
    <Modal title={isEditing ? '编辑工作负载' : '创建工作负载'} open={open} onCancel={onClose} width={920} destroyOnHidden footer={footer}>
      <Form form={form} layout="vertical" className="workload-large-form" data-testid="workload-large-form" preserve>
        <Form.Item label="工作负载标识" name="name" rules={[{ required: true, message: '请输入工作负载标识' }, { pattern: /^[a-z][a-z0-9-]{0,62}$/, message: '仅支持小写字母、数字和连字符' }]}><Input aria-label="工作负载标识" placeholder="order-api" disabled={isEditing} /></Form.Item>
        <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}><Input aria-label="显示名称" placeholder="订单接口" /></Form.Item>
        <Form.Item label="工作负载类型" name="workloadType" rules={[{ required: true, message: '请选择工作负载类型' }]}><Segmented aria-label="工作负载类型" options={WORKLOAD_TYPE_OPTIONS} block /></Form.Item>
        <Form.Item label="镜像来源" name="imageSourceMode" rules={[{ required: true, message: '请选择镜像来源' }]}><Segmented aria-label="镜像来源" options={IMAGE_SOURCE_OPTIONS} block /></Form.Item>
        {needsPipeline && <Form.Item label="关联流水线" name="pipelineId" rules={[{ required: true, message: '请选择关联流水线' }]}><Select placeholder="请选择流水线" options={(pipelines as BuildPipeline[]).map((item) => ({ value: item.id, label: `${item.displayName || item.name} (${item.name})`, disabled: item.status !== 'active' }))} /></Form.Item>}
        <Form.Item label="描述" name="description"><Input.TextArea rows={2} /></Form.Item>
        <WorkloadRuntimeFields />
        <ConfigValueLists />
      </Form>
    </Modal>
  );
}

function MetaItem({ label, value }: { label: string; value: ReactNode }) {
  return <div className="resource-meta-item"><span>{label}</span><div>{value}</div></div>;
}

function pipelineWorkloadSummary(workloads?: Workload[]) {
  if (!workloads?.length) return <Typography.Text type="secondary">暂无关联</Typography.Text>;
  return <Space wrap size={[4, 4]}>{workloads.map((workload) => <Tag key={workload.id}>{workload.displayName || workload.name}</Tag>)}</Space>;
}

function workloadTypeText(type: WorkloadType) {
  return type === 'statefulset' ? '有状态' : '无状态';
}

function imageSourceText(mode: WorkloadImageSourceMode) {
  if (mode === 'custom_image') return '自定义镜像';
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

function BuildPipelinePanel({ applicationId, projectId, localPipelines = [], onPipelineChanged, onPipelineDeleted }: { applicationId: string; projectId?: string; localPipelines?: BuildPipeline[]; onPipelineChanged?: (pipeline: BuildPipeline) => void; onPipelineDeleted?: (pipelineId: string) => void }) {
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [editingPipeline, setEditingPipeline] = useState<BuildPipeline | null>(null);
  const [historyPipeline, setHistoryPipeline] = useState<BuildPipeline | null>(null);
  const { data: pipelines = EMPTY_LIST, isLoading } = useQuery({ queryKey: ['build-pipelines', applicationId], queryFn: () => listBuildPipelines(applicationId), enabled: !!applicationId, staleTime: 1000 });
  const { data: workloads = EMPTY_LIST } = useQuery({ queryKey: ['workloads', applicationId, 'pipeline-cards'], queryFn: () => listWorkloads(applicationId), enabled: !!applicationId });
  const workloadsByPipelineId = useMemo(() => {
    const grouped: Record<string, Workload[]> = {};
    (workloads as Workload[]).forEach((workload) => {
      if (!workload.pipelineId) return;
      grouped[workload.pipelineId] = [...(grouped[workload.pipelineId] || []), workload];
    });
    return grouped;
  }, [workloads]);
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
        <div />
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
                <MetaItem label="关联 Workload" value={pipelineWorkloadSummary(workloadsByPipelineId[item.id])} />
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
    if (initializedRef.current || !repositories.length || !buildEnvironments.length || !runtimeEnvironments.length) return;
    if (editingPipeline) {
      form.setFieldsValue({
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
    const defaultPipeline = nextPipelineDefaults(pipelines as BuildPipeline[]);
    form.setFieldsValue({
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
  }, [buildEnvironments, editingPipeline, form, open, pipelines, pipelinesFetched, repositories, runtimeEnvironments]);

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
        ? await updateBuildPipeline(editingPipeline.id, { displayName: values.displayName, description: values.description, runtimeEnvironmentIds, sources })
        : await createBuildPipeline(applicationId, { name: values.name, displayName: values.displayName, description: values.description, runtimeEnvironmentIds, sources });
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
    <Modal title={isEditing ? '编辑构建流水线' : '创建构建流水线'} open={open} onCancel={onClose} onOk={() => mutation.mutate()} confirmLoading={mutation.isPending} width={980} okText={isEditing ? '保存' : '创建'} cancelText="取消">
      <Form form={form} layout="horizontal" labelCol={{ flex: '120px' }} wrapperCol={{ flex: '1 1 auto' }} colon={false} className="pipeline-editor-form">
        <section className="pipeline-form-section">
          <div className="pipeline-form-section-title">基本信息</div>
          <div className="pipeline-form-stack">
            <Form.Item label="流水线标识" name="name" rules={[{ required: true, message: '请输入流水线标识' }, { pattern: /^[a-z][a-z0-9-]{0,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
              <Input placeholder="main" disabled={isEditing} />
            </Form.Item>
            <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}>
              <Input placeholder="主流水线" />
            </Form.Item>
            <Form.Item label="描述" name="description" className="pipeline-form-span">
              <Input.TextArea rows={2} />
            </Form.Item>
          </div>
        </section>

        <section className="pipeline-form-section">
          <div className="pipeline-form-section-title">运行时环境</div>
          <Form.Item name="runtimeEnvironmentIds" rules={[{ required: true, message: '请选择运行时环境' }]} className="pipeline-runtime-field">
            <div className="pipeline-runtime-table">
              {(runtimeEnvironments as RuntimeEnvironment[]).map((runtime) => {
                const checked = selectedRuntimeIds.includes(runtime.id);
                const nextIds = checked ? selectedRuntimeIds.filter((id: string) => id !== runtime.id) : [...selectedRuntimeIds, runtime.id];
                const isPrimary = selectedRuntimeIds[0] === runtime.id;
                return (
                  <button key={runtime.id} type="button" className={checked ? 'pipeline-runtime-row selected' : 'pipeline-runtime-row'} onClick={() => form.setFieldValue('runtimeEnvironmentIds', nextIds)}>
                    <Checkbox checked={checked} onChange={() => form.setFieldValue('runtimeEnvironmentIds', nextIds)} />
                    <span className="pipeline-runtime-name">{runtime.name}</span>
                    <span className="pipeline-runtime-image">{runtime.runtimeBaseImage || '-'}</span>
                    <span className="pipeline-runtime-state">{isPrimary ? '主产物' : checked ? '附加' : '未选择'}</span>
                  </button>
                );
              })}
            </div>
          </Form.Item>
          <Space wrap size={[8, 8]}>
            {selectedRuntimes.length > 0 ? selectedRuntimes.map((runtime: RuntimeEnvironment, index: number) => (
              <Tag key={runtime.id} color={index === 0 ? 'blue' : 'default'}>{index === 0 ? '主产物' : '附加'} {runtime.name}</Tag>
            )) : <Typography.Text type="secondary">请选择流水线运行时环境</Typography.Text>}
          </Space>
        </section>

        <section className="pipeline-form-section">
          <div className="pipeline-form-section-head">
            <div className="pipeline-form-section-title">代码源</div>
          </div>
          <Form.List name="sources">
            {(fields, { add, remove }) => (
              <div className="pipeline-source-list">
                {fields.map((field, index) => (
                  <div key={field.key} className="pipeline-source-row">
                    <div className="pipeline-source-row-head">
                      <Typography.Text strong>{index === 0 ? '主代码源' : `代码源 ${index + 1}`}</Typography.Text>
                      <Button danger type="text" icon={<DeleteOutlined />} disabled={fields.length <= 1} onClick={() => remove(field.name)}>删除</Button>
                    </div>
                    <div className="pipeline-form-stack">
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
                      <Form.Item label="构建命令" name={[field.name, 'buildCommand']} rules={[{ required: true, message: '请输入构建命令' }]} className="pipeline-form-span">
                        <Input.TextArea rows={3} />
                      </Form.Item>
                      <Form.Item label="产物拷贝命令" name={[field.name, 'artifactCopyCommand']} rules={[{ required: true, message: '请输入产物拷贝命令' }]} className="pipeline-form-span">
                        <Input.TextArea rows={3} />
                      </Form.Item>
                    </div>
                  </div>
                ))}
                <Button className="pipeline-source-add" icon={<PlusOutlined />} onClick={() => add({ key: `source-${fields.length + 1}`, displayName: '代码源', sourcePath: '.', defaultRef: 'main' })}>添加代码源</Button>
              </div>
            )}
          </Form.List>
        </section>
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
