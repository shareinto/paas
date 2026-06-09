import { DeleteOutlined, EditOutlined, PlusOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Badge, Button, Card, Descriptions, Form, Input, Modal, Select, Space, Table, Tabs, Tag, Timeline, Typography, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  createBuildPipeline,
  deleteBuildPipeline,
  getApplication,
  listBuildEnvironments,
  listBuildPipelines,
  listRuntimeEnvironments,
  listSourceRepositories,
  triggerBuildPipeline,
  type BuildPipeline,
  type RuntimeEnvironment
} from '../api';
import { PageHeader } from '../components/PageHeader';

const EMPTY_LIST: any[] = [];

export function ApplicationDetailPage() {
  const { id = 'app_1' } = useParams();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState('builds');
  const [pipelineModalOpen, setPipelineModalOpen] = useState(false);
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
      <Tabs activeKey={activeTab} onChange={setActiveTab} className="detail-tabs" items={[
        { key: 'overview', label: '总览', children: <Overview /> },
        { key: 'env', label: '环境', children: <EnvironmentPanel /> },
        { key: 'versions', label: '版本', children: <Table pagination={false} dataSource={[{ id: 'v1.8.2', digest: 'sha256:91ab', commit: '8c1a09f' }]} columns={[{ title: '版本', dataIndex: 'id' }, { title: '镜像 digest', dataIndex: 'digest' }, { title: '提交', dataIndex: 'commit' }]} /> },
        { key: 'builds', label: '构建', children: <BuildPipelinePanel applicationId={id} /> },
        { key: 'promotions', label: '发布', children: '生产发布需要审批，禁止自审批。' },
        { key: 'config', label: '配置', children: '环境变量和密钥只展示元数据。' },
        { key: 'logs', label: '日志', children: '请从构建详情或环境事件查看日志。' },
        { key: 'monitor', label: '监控', children: '实例趋势和健康状态。' },
        { key: 'settings', label: '设置', children: '应用基础设置。' }
      ]} />
      <CreatePipelineModal applicationId={id} projectId={app?.projectId} open={pipelineModalOpen} onClose={() => setPipelineModalOpen(false)} />
    </>
  );
}

function BuildPipelinePanel({ applicationId }: { applicationId: string }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: pipelines = EMPTY_LIST, isLoading } = useQuery({ queryKey: ['build-pipelines', applicationId], queryFn: () => listBuildPipelines(applicationId), enabled: !!applicationId });
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
          dataSource={pipelines}
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

function CreatePipelineModal({ applicationId, projectId, open, onClose }: { applicationId: string; projectId?: string; open: boolean; onClose: () => void }) {
  const [form] = Form.useForm();
  const queryClient = useQueryClient();
  const initializedRef = useRef(false);
  const { data: repositories = EMPTY_LIST } = useQuery({ queryKey: ['source-repositories', projectId], queryFn: () => listSourceRepositories(projectId), enabled: open && !!projectId });
  const { data: buildEnvironments = EMPTY_LIST } = useQuery({ queryKey: ['build-environments'], queryFn: listBuildEnvironments, enabled: open });
  const { data: runtimeEnvironments = EMPTY_LIST } = useQuery({ queryKey: ['runtime-environments'], queryFn: listRuntimeEnvironments, enabled: open });
  const { data: pipelines = EMPTY_LIST, isFetched: pipelinesFetched } = useQuery({ queryKey: ['build-pipelines', applicationId], queryFn: () => listBuildPipelines(applicationId), enabled: open && !!applicationId });
  const selectedRepositoryId = Form.useWatch(['sources', 0, 'sourceRepositoryId'], form);
  const selectedRuntimeId = Form.useWatch(['sources', 0, 'runtimeEnvironmentId'], form);
  const selectedRepository = repositories.find((item: any) => item.id === selectedRepositoryId);
  const selectedRuntime = runtimeEnvironments.find((item: any) => item.id === selectedRuntimeId);

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
      sources: [{
        key: 'main',
        displayName: '主代码源',
        sourceRepositoryId: repo?.id,
        buildEnvironmentId: buildEnv?.id,
        runtimeEnvironmentId: runtime?.id,
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
      const sources = (values.sources || []).map((source: any, index: number) => {
        const runtime = runtimeEnvironments.find((item: RuntimeEnvironment) => item.id === source.runtimeEnvironmentId);
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
            runtimeBaseImage: runtime?.runtimeBaseImage || '',
            artifactDeployPath: runtime?.artifactDeployPath || '',
            defaultRef: source.defaultRef
          }
        };
      });
      return createBuildPipeline(applicationId, { name: values.name, displayName: values.displayName, description: values.description, sources });
    },
    onSuccess: () => {
      message.success('流水线已创建');
      queryClient.invalidateQueries({ queryKey: ['build-pipelines', applicationId] });
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
                  <Form.Item label="运行时环境" name={[field.name, 'runtimeEnvironmentId']} rules={[{ required: true, message: '请选择运行时环境' }]}>
                    <Select options={runtimeEnvironments.map((item: any) => ({ value: item.id, label: `${item.name} ${item.runtimeBaseImage ? `(${item.runtimeBaseImage})` : ''}` }))} />
                  </Form.Item>
                  <Form.Item label="构建命令" name={[field.name, 'buildCommand']} rules={[{ required: true, message: '请输入构建命令' }]}>
                    <Input.TextArea rows={3} />
                  </Form.Item>
                  <Form.Item label="产物拷贝命令" name={[field.name, 'artifactCopyCommand']} rules={[{ required: true, message: '请输入产物拷贝命令' }]}>
                    <Input.TextArea rows={3} />
                  </Form.Item>
                  <Typography.Text type="secondary">镜像运行时：{selectedRuntime?.runtimeBaseImage || '-'}</Typography.Text>
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
