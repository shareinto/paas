import { DeleteOutlined, EditOutlined, PlusOutlined, PlayCircleOutlined, SettingOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Badge, Button, Card, Descriptions, Divider, Drawer, Form, Input, InputNumber, Modal, Popconfirm, Select, Segmented, Space, Table, Tabs, Tag, Timeline, Typography, message } from 'antd';
import { type ReactNode, useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  createBuildPipeline,
  createWorkload,
  deleteBuildPipeline,
  deleteWorkload,
  getApplication,
  listBuildEnvironments,
  listBuildPipelines,
  listRuntimeEnvironments,
  listSourceRepositories,
  listWorkloadEnvironmentConfigs,
  listWorkloads,
  triggerBuildPipeline,
  type BuildPipeline,
  type RuntimeEnvironment,
  type Workload,
  type WorkloadEnvironmentConfig,
  type WorkloadImageSourceMode,
  type WorkloadType
} from '../api';
import { PageHeader } from '../components/PageHeader';

const EMPTY_LIST: any[] = [];
const WORKLOAD_TYPE_OPTIONS = [
  { label: 'Deployment', value: 'deployment' },
  { label: 'StatefulSet', value: 'statefulset' }
];
const IMAGE_SOURCE_OPTIONS = [
  { label: '流水线产物', value: 'pipeline_artifact' },
  { label: '发布时选择自定义镜像', value: 'custom_image' }
];

export function ApplicationDetailPage() {
  const { id = 'app_1' } = useParams();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState('workloads');
  const [pipelineModalOpen, setPipelineModalOpen] = useState(false);
  const [createdPipelines, setCreatedPipelines] = useState<BuildPipeline[]>([]);
  const { data: app } = useQuery({ queryKey: ['application', id], queryFn: () => getApplication(id), enabled: !!id });

  return (
    <>
      <PageHeader
        title={app?.displayName || '应用详情'}
        extra={(
          <Space>
            <Button icon={<EditOutlined />} onClick={() => navigate(`/apps/${id}/edit`)}>编辑应用</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setPipelineModalOpen(true)}>创建流水线</Button>
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
      <Tabs
        activeKey={activeTab}
        onChange={(key) => {
          if (key === 'promotion') {
            navigate(`/apps/${id}/promotions`);
            return;
          }
          setActiveTab(key);
        }}
        className="detail-tabs"
        items={[
        { key: 'workloads', label: '应用 Workload', children: <WorkloadPanel applicationId={id} /> },
        { key: 'promotion', label: '发布晋级', children: null },
        { key: 'overview', label: '总览', children: <Overview /> },
        { key: 'env', label: '环境', children: <EnvironmentPanel /> },
        { key: 'versions', label: '版本', children: <Table pagination={false} dataSource={[{ id: 'v1.8.2', digest: 'sha256:91ab', commit: '8c1a09f' }]} columns={[{ title: '版本', dataIndex: 'id' }, { title: '镜像 digest', dataIndex: 'digest' }, { title: '提交', dataIndex: 'commit' }]} /> },
        { key: 'builds', label: '构建', children: <BuildPipelinePanel applicationId={id} localPipelines={createdPipelines} /> },
        { key: 'config', label: '配置', children: '环境变量和密钥只展示元数据。' },
        { key: 'logs', label: '日志', children: '请从构建详情或环境事件查看日志。' },
        { key: 'monitor', label: '监控', children: '实例趋势和健康状态。' },
        { key: 'settings', label: '设置', children: '应用基础设置。' }
      ]} />
      <CreatePipelineModal
        applicationId={id}
        projectId={app?.projectId}
        open={pipelineModalOpen}
        onClose={() => setPipelineModalOpen(false)}
        onCreated={(pipeline) => {
          setCreatedPipelines((current) => [pipeline, ...current.filter((item) => item.id !== pipeline.id)]);
          setActiveTab('builds');
        }}
      />
    </>
  );
}

function WorkloadPanel({ applicationId }: { applicationId: string }) {
  const [createOpen, setCreateOpen] = useState(false);
  const [configWorkload, setConfigWorkload] = useState<Workload | null>(null);
  const queryClient = useQueryClient();
  const { data: workloads = EMPTY_LIST, isLoading } = useQuery({ queryKey: ['workloads', applicationId], queryFn: () => listWorkloads(applicationId), enabled: !!applicationId });
  const deleteMutation = useMutation({
    mutationFn: (workloadId: string) => deleteWorkload(applicationId, workloadId),
    onSuccess: (workload) => {
      message.success('Workload 已删除');
      queryClient.setQueryData<Workload[]>(['workloads', applicationId], (current = []) => current.filter((item) => item.id !== workload.id));
      queryClient.invalidateQueries({ queryKey: ['workloads', applicationId], refetchType: 'none' });
      if (configWorkload?.id === workload.id) setConfigWorkload(null);
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除 Workload 失败')
  });

  return (
    <div data-testid="workload-panel">
      <Card
        className="summary-card compact-card"
        title="Workload 列表"
        extra={<Button type="primary" aria-label="创建 Workload" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>创建 Workload</Button>}
      >
        <Table
          rowKey="id"
          loading={isLoading}
          pagination={false}
          locale={{ emptyText: '暂无 Workload' }}
          dataSource={workloads}
          columns={[
            {
              title: 'Workload',
              dataIndex: 'displayName',
              width: 180,
              render: (_: string, item: Workload) => (
                <Space direction="vertical" size={0}>
                  <Typography.Text strong>{item.name}</Typography.Text>
                  <Typography.Text type="secondary">{item.displayName || item.description || '-'}</Typography.Text>
                </Space>
              )
            },
            { title: '类型', dataIndex: 'workloadType', width: 130, render: (type: WorkloadType) => <Tag color="blue">{workloadTypeText(type)}</Tag> },
            {
              title: '镜像来源',
              dataIndex: 'imageSourceMode',
              width: 160,
              render: (mode: WorkloadImageSourceMode, item: Workload) => (
                <Space direction="vertical" size={0}>
                  <Typography.Text>{imageSourceText(mode)}</Typography.Text>
                  <Typography.Text type="secondary">{item.imageSourceName || '-'}</Typography.Text>
                </Space>
              )
            },
            { title: '最近 Release', dataIndex: 'latestRelease', width: 140, render: (value: string) => value || '暂无 Release' },
            {
              title: '各环境状态',
              dataIndex: 'envStatuses',
              render: (statuses: Workload['envStatuses'] = []) => statuses.length > 0 ? (
                <Space wrap size={[6, 6]}>
                  {statuses.map((status) => (
                    <Tag key={status.envName} color={status.healthStatus === '健康' ? 'green' : status.healthStatus === '未知' ? 'default' : 'orange'}>
                      {status.envName} {status.syncStatus} / {status.healthStatus}
                    </Tag>
                  ))}
                </Space>
              ) : <Typography.Text type="secondary">暂无环境状态</Typography.Text>
            },
            { title: '状态', dataIndex: 'status', width: 100, render: (status: string) => <Badge status={status === 'enabled' ? 'success' : 'default'} text={status === 'enabled' ? '启用' : '停用'} /> },
            {
              title: '操作',
              key: 'actions',
              width: 210,
              render: (_: unknown, item: Workload) => (
                <Space>
                  <Button aria-label="部署配置" icon={<SettingOutlined />} onClick={() => setConfigWorkload(item)}>部署配置</Button>
                  <Popconfirm
                    title="删除 Workload"
                    description={`确认删除 ${item.name}？删除后不会进入新的 Freight。`}
                    okText="确认删除"
                    cancelText="取消"
                    onConfirm={() => deleteMutation.mutate(item.id)}
                  >
                    <Button danger aria-label="删除 Workload" icon={<DeleteOutlined />} loading={deleteMutation.isPending}>删除</Button>
                  </Popconfirm>
                </Space>
              )
            }
          ]}
        />
      </Card>
      <CreateWorkloadDrawer applicationId={applicationId} open={createOpen} onClose={() => setCreateOpen(false)} />
      <DeployConfigModal applicationId={applicationId} workload={configWorkload} onClose={() => setConfigWorkload(null)} />
    </div>
  );
}

function CreateWorkloadDrawer({ applicationId, open, onClose }: { applicationId: string; open: boolean; onClose: () => void }) {
  const [form] = Form.useForm();
  const queryClient = useQueryClient();
  const imageSourceMode = Form.useWatch('imageSourceMode', form);
  const mutation = useMutation({
    mutationFn: async () => {
      const values = await form.validateFields();
      return createWorkload(applicationId, values);
    },
    onSuccess: (workload) => {
      message.success('Workload 已创建');
      queryClient.setQueryData<Workload[]>(['workloads', applicationId], (current = []) => [workload, ...current.filter((item) => item.id !== workload.id)]);
      queryClient.invalidateQueries({ queryKey: ['workloads', applicationId], refetchType: 'none' });
      onClose();
      form.resetFields();
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '创建 Workload 失败')
  });

  useEffect(() => {
    if (open) {
      form.setFieldsValue({ workloadType: 'deployment', imageSourceMode: 'pipeline_artifact', replicas: 1 });
    } else {
      form.resetFields();
    }
  }, [form, open]);

  return (
    <Drawer title="创建 Workload" open={open} onClose={onClose} width={640} destroyOnClose extra={<Space><Button onClick={onClose}>取消</Button><Button type="primary" loading={mutation.isPending} onClick={() => mutation.mutate()}>创建</Button></Space>}>
      <Form form={form} layout="vertical">
        <Form.Item label="Workload 标识" name="name" rules={[{ required: true, message: '请输入 Workload 标识' }, { pattern: /^[a-z][a-z0-9-]{0,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
          <Input placeholder="order-api" />
        </Form.Item>
        <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}>
          <Input placeholder="订单接口" />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input.TextArea rows={2} placeholder="说明该 Workload 的职责" />
        </Form.Item>
        <Form.Item label="Workload 类型" name="workloadType" rules={[{ required: true, message: '请选择 Workload 类型' }]}>
          <Segmented aria-label="Workload 类型" options={WORKLOAD_TYPE_OPTIONS} />
        </Form.Item>
        <Form.Item label="镜像来源偏好" name="imageSourceMode" rules={[{ required: true, message: '请选择镜像来源偏好' }]}>
          <Select options={IMAGE_SOURCE_OPTIONS} />
        </Form.Item>
        {imageSourceMode === 'custom_image' ? (
          <>
            <Form.Item label="自定义镜像地址" name="customImage" rules={[{ required: true, message: '请输入自定义镜像地址' }]}>
              <Input aria-label="自定义镜像地址" placeholder="registry.example.com/order/api:v1" />
            </Form.Item>
            <Alert showIcon type="warning" message="当前 Workload 创建只保存 Workload 基础信息，自定义镜像请在创建 Freight 时选择；镜像 tag 可能被覆盖，建议使用 digest。" />
          </>
        ) : (
          <Form.Item label="绑定流水线" name="pipelineName">
            <Input placeholder="主流水线" />
          </Form.Item>
        )}
        <Form.Item label="副本数" name="replicas">
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Drawer>
  );
}

function DeployConfigModal({ applicationId, workload, onClose }: { applicationId: string; workload: Workload | null; onClose: () => void }) {
  const { data: configs = EMPTY_LIST, isLoading } = useQuery({
    queryKey: ['workload-environment-configs', applicationId, workload?.id],
    queryFn: () => listWorkloadEnvironmentConfigs(applicationId, workload!.id),
    enabled: !!applicationId && !!workload
  });
  const config = (configs as WorkloadEnvironmentConfig[])[0];

  return (
    <Modal
      title={`${workload?.name || ''} 部署配置`}
      open={!!workload}
      onCancel={onClose}
      width={960}
      destroyOnHidden
      footer={<Button onClick={onClose}>关闭</Button>}
    >
      <Space direction="vertical" size={18} style={{ width: '100%' }}>
        <Descriptions bordered size="small" column={2} items={[
          { key: 'environment', label: '环境', children: config?.envName || 'prod' },
          { key: 'type', label: 'Workload 类型', children: workload ? workloadTypeText(workload.workloadType) : '-' },
          { key: 'replicas', label: '副本数', children: config?.replicas ?? '-' },
          { key: 'loading', label: '加载状态', children: isLoading ? '读取中' : '已就绪' }
        ]} />
        <ConfigSection title="端口">
          <Table size="small" rowKey={(item) => item.name} pagination={false} locale={{ emptyText: '暂无端口' }} dataSource={config?.servicePorts || []} columns={[
            { title: '名称', dataIndex: 'name' },
            { title: 'Service 端口', dataIndex: 'port' },
            { title: '容器端口', dataIndex: 'targetPort' },
            { title: '协议', dataIndex: 'protocol' }
          ]} />
        </ConfigSection>
        <ConfigSection title="资源">
          <Descriptions size="small" column={2} items={[
            { key: 'requests', label: '请求资源', children: resourceText(config?.resourceRequests) },
            { key: 'limits', label: '限制资源', children: resourceText(config?.resourceLimits) }
          ]} />
        </ConfigSection>
        <ConfigSection title="探针">
          <Table size="small" rowKey={(item) => item.name} pagination={false} locale={{ emptyText: '暂无探针' }} dataSource={config?.probes || []} columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '类型', dataIndex: 'type' },
            { title: '路径或命令', dataIndex: 'path' },
            { title: '端口', dataIndex: 'port' }
          ]} />
        </ConfigSection>
        <ConfigSection title="域名">
          <Table size="small" rowKey={(item) => item.host} pagination={false} locale={{ emptyText: '暂无域名' }} dataSource={config?.ingressHosts || []} columns={[
            { title: '域名', dataIndex: 'host' },
            { title: '路径', dataIndex: 'path' },
            { title: '服务端口', dataIndex: 'servicePort' },
            { title: 'TLS', dataIndex: 'tls', render: (tls: boolean) => tls ? '启用' : '未启用' }
          ]} />
        </ConfigSection>
        <ConfigSection title="配置文件">
          <Table size="small" rowKey={(item) => item.mountPath} pagination={false} locale={{ emptyText: '暂无配置文件' }} dataSource={config?.configFiles || []} columns={[
            { title: '挂载路径', dataIndex: 'mountPath' },
            { title: '配置内容摘要', dataIndex: 'content', render: (content: string) => summaryText(content) }
          ]} />
        </ConfigSection>
        <ConfigSection title="环境变量">
          <Table size="small" rowKey={(item) => item.name} pagination={false} locale={{ emptyText: '暂无环境变量' }} dataSource={config?.envVars || []} columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '值', dataIndex: 'value' }
          ]} />
        </ConfigSection>
        <ConfigSection title="可写目录">
          <Table size="small" rowKey={(item) => item.mountPath} pagination={false} locale={{ emptyText: '暂无可写目录' }} dataSource={config?.writableDirs || []} columns={[
            { title: '挂载路径', dataIndex: 'mountPath' },
            { title: '容量限制', dataIndex: 'sizeLimit', render: (value: string) => value || '-' }
          ]} />
        </ConfigSection>
      </Space>
    </Modal>
  );
}

function ConfigSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section>
      <Divider orientation="left" plain>{title}</Divider>
      {children}
    </section>
  );
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

function resourceText(resource?: { cpu?: string; memory?: string }) {
  if (!resource || (!resource.cpu && !resource.memory)) return '-';
  return `CPU ${resource.cpu || '-'} / 内存 ${resource.memory || '-'}`;
}

function summaryText(content?: string) {
  const normalized = (content || '').replace(/\s+/g, ' ').trim();
  if (!normalized) return '-';
  return normalized.length > 40 ? `${normalized.slice(0, 40)}...` : normalized;
}

function BuildPipelinePanel({ applicationId, localPipelines = [] }: { applicationId: string; localPipelines?: BuildPipeline[] }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: pipelines = EMPTY_LIST, isLoading } = useQuery({ queryKey: ['build-pipelines', applicationId], queryFn: () => listBuildPipelines(applicationId), enabled: !!applicationId, staleTime: 1000 });
  const visiblePipelines = mergePipelines(localPipelines, pipelines as BuildPipeline[]);
  const triggerMutation = useMutation({
    mutationFn: (pipeline: BuildPipeline) => triggerBuildPipeline(pipeline.id, { gitRef: pipeline.sources?.[0]?.defaultRef || 'main' }),
    onSuccess: (run) => navigate(`/builds/${run.id}`),
    onError: (error) => message.error(error instanceof Error ? error.message : '触发构建失败')
  });
  const deleteMutation = useMutation({
    mutationFn: deleteBuildPipeline,
    onSuccess: () => {
      message.success('流水线已删除');
      queryClient.invalidateQueries({ queryKey: ['build-pipelines', applicationId] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除流水线失败')
  });

  return (
    <>
      <Card
        className="summary-card"
        title="构建流水线"
      >
        <Table
          rowKey="id"
          loading={isLoading}
          pagination={false}
          locale={{ emptyText: '暂无流水线' }}
          dataSource={visiblePipelines}
          columns={[
            { title: '流水线', dataIndex: 'displayName', render: (_: string, item: BuildPipeline) => <Space direction="vertical" size={0}><Typography.Text strong>{item.displayName || item.name}</Typography.Text><Typography.Text type="secondary">{item.name}</Typography.Text></Space> },
            { title: '代码源', dataIndex: 'sources', render: (sources: any[] = []) => sources.map((source) => <Tag key={source.key}>{source.displayName || source.key}</Tag>) },
            { title: '状态', dataIndex: 'status', render: (status: string) => <Badge status={status === 'active' ? 'success' : 'default'} text={status === 'active' ? '启用' : '停用'} /> },
            { title: '更新时间', dataIndex: 'updatedAt' },
            {
              title: '操作',
              key: 'actions',
              render: (_: unknown, item: BuildPipeline) => (
                <Space>
                  <Button icon={<PlayCircleOutlined />} loading={triggerMutation.isPending} onClick={() => triggerMutation.mutate(item)}>触发构建</Button>
                  <Button danger icon={<DeleteOutlined />} loading={deleteMutation.isPending} onClick={() => deleteMutation.mutate(item.id)}>删除</Button>
                </Space>
              )
            }
          ]}
        />
      </Card>
    </>
  );
}

function CreatePipelineModal({ applicationId, projectId, open, onClose, onCreated }: { applicationId: string; projectId?: string; open: boolean; onClose: () => void; onCreated?: (pipeline: BuildPipeline) => void }) {
  const [form] = Form.useForm();
  const queryClient = useQueryClient();
  const initializedRef = useRef(false);
  const { data: repositories = EMPTY_LIST } = useQuery({ queryKey: ['source-repositories', projectId], queryFn: () => listSourceRepositories(projectId), enabled: open && !!projectId });
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
    if (initializedRef.current || !pipelinesFetched || !repositories.length || !buildEnvironments.length || !runtimeEnvironments.length) return;
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
  }, [buildEnvironments, form, open, pipelines, pipelinesFetched, repositories, runtimeEnvironments]);

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
      const pipeline = await createBuildPipeline(applicationId, { name: values.name, displayName: values.displayName, description: values.description, runtimeEnvironmentIds, sources });
      return { ...pipeline, sources };
    },
    onSuccess: (pipeline) => {
      message.success('流水线已创建');
      onCreated?.(pipeline);
      queryClient.setQueryData<BuildPipeline[]>(['build-pipelines', applicationId], (current = []) => {
        const existing = current.filter((item) => item.id !== pipeline.id);
        return [pipeline, ...existing];
      });
      queryClient.invalidateQueries({ queryKey: ['build-pipelines', applicationId], refetchType: 'none' });
      onClose();
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '创建流水线失败')
  });

  return (
    <Modal title="创建构建流水线" open={open} onCancel={onClose} onOk={() => mutation.mutate()} confirmLoading={mutation.isPending} width={760} okText="创建" cancelText="取消">
      <Form form={form} layout="vertical">
        <Form.Item label="流水线标识" name="name" rules={[{ required: true, message: '请输入流水线标识' }, { pattern: /^[a-z][a-z0-9-]{0,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
          <Input placeholder="main" />
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

function Overview() {
  return <Card className="summary-card"><Timeline items={[{ color: 'green', children: 'dev 已同步' }, { color: 'blue', children: 'test 正在部署' }, { color: 'gray', children: 'prod 等待审批' }]} /></Card>;
}

function EnvironmentPanel() {
  return <Table pagination={false} dataSource={[{ env: 'dev', state: '运行中', desired: '3', ready: '3', sync: 'Synced', health: 'Healthy' }]} columns={[{ title: '环境', dataIndex: 'env' }, { title: '状态', dataIndex: 'state' }, { title: '期望实例', dataIndex: 'desired' }, { title: '可用实例', dataIndex: 'ready' }, { title: '同步状态', dataIndex: 'sync' }, { title: '健康状态', dataIndex: 'health' }]} />;
}
