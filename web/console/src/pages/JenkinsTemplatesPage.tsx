import Editor, { type BeforeMount } from '@monaco-editor/react';
import { DeleteOutlined, EditOutlined, SaveOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Card, Form, Input, List, Modal, Popconfirm, Select, Space, Switch, Tabs, Tag, Typography, message } from 'antd';
import { useEffect, useState } from 'react';
import { createBuildEnvironment, createRuntimeEnvironment, deleteBuildEnvironment, deleteRuntimeEnvironment, getBuildTemplate, listAdminBuildEnvironments, listAdminRuntimeEnvironments, updateBuildEnvironment, updateBuildTemplate, updateRuntimeEnvironment, type BuildEnvironment, type RuntimeEnvironment } from '../api';
import { PageHeader } from '../components/PageHeader';

const STATUS_OPTIONS = [
  { value: 'enabled', label: '已启用' },
  { value: 'disabled', label: '已停用' }
];

export function JenkinsTemplatesPage() {
  return (
    <>
      <PageHeader title="构建管理" />
      <Typography.Paragraph className="page-subtitle">平台管理员统一维护构建环境、运行时环境和全局构建模板。</Typography.Paragraph>
      <Tabs
        items={[
          { key: 'build-env', label: '构建环境', children: <BuildEnvironmentTab /> },
          { key: 'runtime-env', label: '运行时环境', children: <RuntimeEnvironmentTab /> },
          { key: 'template', label: '构建模板', children: <BuildTemplateTab /> }
        ]}
      />
    </>
  );
}

function BuildEnvironmentTab() {
  const [form] = Form.useForm();
  const [editForm] = Form.useForm();
  const [editing, setEditing] = useState<BuildEnvironment | null>(null);
  const queryClient = useQueryClient();
  const { data = [], isLoading } = useQuery({ queryKey: ['build-environments-admin'], queryFn: listAdminBuildEnvironments });
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['build-environments-admin'] });
    queryClient.invalidateQueries({ queryKey: ['build-environments'] });
  };
  const createMutation = useMutation({ mutationFn: createBuildEnvironment, onSuccess: () => { message.success('构建环境已创建'); form.resetFields(); refresh(); }, onError: showError('构建环境创建失败') });
  const updateMutation = useMutation({ mutationFn: ({ id, input }: { id: string; input: Partial<BuildEnvironment> }) => updateBuildEnvironment(id, input), onSuccess: () => { message.success('构建环境已更新'); setEditing(null); refresh(); }, onError: showError('构建环境更新失败') });
  const deleteMutation = useMutation({ mutationFn: deleteBuildEnvironment, onSuccess: () => { message.success('构建环境已删除'); refresh(); }, onError: showError('构建环境删除失败') });

  useEffect(() => {
    if (editing) editForm.setFieldsValue(editing);
  }, [editForm, editing]);

  return (
    <div className="template-grid">
      <Card className="summary-card" title="新增构建环境">
        <Form form={form} layout="vertical" initialValues={defaultBuildEnvironmentValues()} onFinish={(values) => createMutation.mutate({ ...values, status: 'enabled' })}>
          <BuildEnvironmentFields />
          <Button type="primary" htmlType="submit" loading={createMutation.isPending}>创建构建环境</Button>
        </Form>
      </Card>
      <Card className="summary-card build-type-list-card" title="构建环境列表">
        <List<BuildEnvironment> loading={isLoading} dataSource={data} rowKey="id" renderItem={(item) => (
          <List.Item className="build-type-list-item">
            <div className="build-type-list-main">
              <Space size={6} wrap>
                <Typography.Text strong>{item.name}</Typography.Text>
                <Tag color={item.status === 'enabled' ? 'green' : 'default'}>{item.status === 'enabled' ? '已启用' : '已停用'}</Tag>
                {item.isDefault && <Tag color="blue">默认</Tag>}
              </Space>
              <Typography.Text type="secondary">{item.buildImage || '-'}</Typography.Text>
            </div>
            <Space wrap>
              <Button size="small" icon={<EditOutlined />} onClick={() => setEditing(item)}>编辑</Button>
              <Button size="small" onClick={() => updateMutation.mutate({ id: item.id, input: { ...item, status: item.status === 'enabled' ? 'disabled' : 'enabled' } })}>{item.status === 'enabled' ? '停用' : '启用'}</Button>
              <Button size="small" disabled={item.isDefault} onClick={() => updateMutation.mutate({ id: item.id, input: { ...item, isDefault: true, status: item.status } })}>设为默认</Button>
              <Popconfirm title="确认删除该构建环境？" okText="删除" cancelText="取消" onConfirm={() => deleteMutation.mutate(item.id)}><Button size="small" danger icon={<DeleteOutlined />}>删除</Button></Popconfirm>
            </Space>
          </List.Item>
        )} />
      </Card>
      <Modal title="编辑构建环境" open={!!editing} okText="保存" cancelText="取消" confirmLoading={updateMutation.isPending} onCancel={() => setEditing(null)} onOk={() => editForm.submit()}>
        <Form form={editForm} layout="vertical" onFinish={(values) => editing && updateMutation.mutate({ id: editing.id, input: { ...editing, ...values } })}>
          <BuildEnvironmentFields editing />
        </Form>
      </Modal>
    </div>
  );
}

function RuntimeEnvironmentTab() {
  const [form] = Form.useForm();
  const [editForm] = Form.useForm();
  const [editing, setEditing] = useState<RuntimeEnvironment | null>(null);
  const queryClient = useQueryClient();
  const { data = [], isLoading } = useQuery({ queryKey: ['runtime-environments-admin'], queryFn: listAdminRuntimeEnvironments });
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['runtime-environments-admin'] });
    queryClient.invalidateQueries({ queryKey: ['runtime-environments'] });
  };
  const createMutation = useMutation({ mutationFn: createRuntimeEnvironment, onSuccess: () => { message.success('运行时环境已创建'); form.resetFields(); refresh(); }, onError: showError('运行时环境创建失败') });
  const updateMutation = useMutation({ mutationFn: ({ id, input }: { id: string; input: Partial<RuntimeEnvironment> }) => updateRuntimeEnvironment(id, input), onSuccess: () => { message.success('运行时环境已更新'); setEditing(null); refresh(); }, onError: showError('运行时环境更新失败') });
  const deleteMutation = useMutation({ mutationFn: deleteRuntimeEnvironment, onSuccess: () => { message.success('运行时环境已删除'); refresh(); }, onError: showError('运行时环境删除失败') });

  useEffect(() => {
    if (editing) editForm.setFieldsValue(editing);
  }, [editForm, editing]);

  return (
    <div className="template-grid">
      <Card className="summary-card" title="新增运行时环境">
        <Form form={form} layout="vertical" initialValues={defaultRuntimeEnvironmentValues()} onFinish={(values) => createMutation.mutate({ ...values, status: 'enabled' })}>
          <RuntimeEnvironmentFields />
          <Button type="primary" htmlType="submit" loading={createMutation.isPending}>创建运行时环境</Button>
        </Form>
      </Card>
      <Card className="summary-card build-type-list-card" title="运行时环境列表">
        <List<RuntimeEnvironment> loading={isLoading} dataSource={data} rowKey="id" renderItem={(item) => (
          <List.Item className="build-type-list-item">
            <div className="build-type-list-main">
              <Space size={6} wrap>
                <Typography.Text strong>{item.name}</Typography.Text>
                <Tag color={item.status === 'enabled' ? 'green' : 'default'}>{item.status === 'enabled' ? '已启用' : '已停用'}</Tag>
                {item.isDefault && <Tag color="blue">默认</Tag>}
              </Space>
              <Typography.Text type="secondary">{item.runtimeBaseImage} · {item.artifactDeployPath || '-'} · {item.dockerfilePath || '-'}</Typography.Text>
            </div>
            <Space wrap>
              <Button size="small" icon={<EditOutlined />} onClick={() => setEditing(item)}>编辑</Button>
              <Button size="small" onClick={() => updateMutation.mutate({ id: item.id, input: { ...item, status: item.status === 'enabled' ? 'disabled' : 'enabled' } })}>{item.status === 'enabled' ? '停用' : '启用'}</Button>
              <Button size="small" disabled={item.isDefault} onClick={() => updateMutation.mutate({ id: item.id, input: { ...item, isDefault: true, status: item.status } })}>设为默认</Button>
              <Popconfirm title="确认删除该运行时环境？" okText="删除" cancelText="取消" onConfirm={() => deleteMutation.mutate(item.id)}><Button size="small" danger icon={<DeleteOutlined />}>删除</Button></Popconfirm>
            </Space>
          </List.Item>
        )} />
      </Card>
      <Modal title="编辑运行时环境" open={!!editing} okText="保存" cancelText="取消" confirmLoading={updateMutation.isPending} onCancel={() => setEditing(null)} onOk={() => editForm.submit()}>
        <Form form={editForm} layout="vertical" onFinish={(values) => editing && updateMutation.mutate({ id: editing.id, input: { ...editing, ...values } })}>
          <RuntimeEnvironmentFields editing />
        </Form>
      </Modal>
    </div>
  );
}

function BuildEnvironmentFields({ editing = false }: { editing?: boolean }) {
  return (
    <>
      <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入名称' }]}><Input disabled={editing} /></Form.Item>
      <Form.Item label="描述" name="description"><Input.TextArea rows={2} /></Form.Item>
      <Form.Item label="构建镜像" name="buildImage" rules={[{ required: true, message: '请输入构建镜像' }]}><Input placeholder="cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/gradle:7-jdk11" /></Form.Item>
      {editing && <Form.Item label="状态" name="status" rules={[{ required: true, message: '请选择状态' }]}><Select options={STATUS_OPTIONS} /></Form.Item>}
      <Form.Item label="默认环境" name="isDefault" valuePropName="checked"><Switch checkedChildren="是" unCheckedChildren="否" /></Form.Item>
    </>
  );
}

function RuntimeEnvironmentFields({ editing = false }: { editing?: boolean }) {
  return (
    <>
      <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入名称' }]}><Input disabled={editing} /></Form.Item>
      <Form.Item label="描述" name="description"><Input.TextArea rows={2} /></Form.Item>
      <Form.Item label="运行时镜像" name="runtimeBaseImage" rules={[{ required: true, message: '请输入运行时镜像' }]}><Input /></Form.Item>
      <Form.Item label="产物放置路径" name="artifactDeployPath"><Input /></Form.Item>
      <Form.Item label="Dockerfile路径" name="dockerfilePath"><Input placeholder="java/jar/Dockerfile" /></Form.Item>
      {editing && <Form.Item label="状态" name="status" rules={[{ required: true, message: '请选择状态' }]}><Select options={STATUS_OPTIONS} /></Form.Item>}
      <Form.Item label="默认环境" name="isDefault" valuePropName="checked"><Switch checkedChildren="是" unCheckedChildren="否" /></Form.Item>
    </>
  );
}

function BuildTemplateTab() {
  const [form] = Form.useForm();
  const { data, isLoading, refetch } = useQuery({ queryKey: ['build-template'], queryFn: getBuildTemplate });
  const mutation = useMutation({ mutationFn: updateBuildTemplate, onSuccess: () => { message.success('构建模板已保存'); refetch(); }, onError: showError('构建模板保存失败') });

  if (data && !form.getFieldValue('content')) {
    form.setFieldsValue({ content: data.content });
  }

  return (
    <Card className="summary-card" loading={isLoading} title={`全局构建模板${data ? ` v${data.version}` : ''}`}>
      <Form form={form} layout="vertical" onFinish={(values) => mutation.mutate({ content: values.content })}>
        <Form.Item label="模板内容" name="content" rules={[{ required: true, message: '请输入模板内容' }]}>
          <GroovyTemplateEditor />
        </Form.Item>
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={mutation.isPending}>保存构建模板</Button>
      </Form>
    </Card>
  );
}

function GroovyTemplateEditor({ value = '', onChange }: { value?: string; onChange?: (value: string) => void }) {
  return (
    <div className="groovy-template-editor">
      <Editor
        height="560px"
        language="groovy"
        value={value}
        beforeMount={registerGroovyLanguage}
        onChange={(next) => onChange?.(next || '')}
        options={{
          minimap: { enabled: false },
          fontSize: 13,
          lineNumbersMinChars: 3,
          scrollBeyondLastLine: false,
          tabSize: 2,
          wordWrap: 'on',
          renderWhitespace: 'selection',
          automaticLayout: true
        }}
      />
    </div>
  );
}

const registerGroovyLanguage: BeforeMount = (monaco) => {
  if (monaco.languages.getLanguages().some((language) => language.id === 'groovy')) return;
  monaco.languages.register({ id: 'groovy', extensions: ['.groovy', '.Jenkinsfile'], aliases: ['Groovy', 'groovy', 'Jenkinsfile'] });
  monaco.languages.setLanguageConfiguration('groovy', {
    comments: { lineComment: '//', blockComment: ['/*', '*/'] },
    brackets: [['{', '}'], ['[', ']'], ['(', ')']],
    autoClosingPairs: [
      { open: '{', close: '}' },
      { open: '[', close: ']' },
      { open: '(', close: ')' },
      { open: "'", close: "'", notIn: ['string', 'comment'] },
      { open: '"', close: '"', notIn: ['string', 'comment'] }
    ],
    surroundingPairs: [
      { open: '{', close: '}' },
      { open: '[', close: ']' },
      { open: '(', close: ')' },
      { open: "'", close: "'" },
      { open: '"', close: '"' }
    ]
  });
  monaco.languages.setMonarchTokensProvider('groovy', {
    defaultToken: '',
    tokenPostfix: '.groovy',
    keywords: [
      'abstract', 'as', 'assert', 'break', 'case', 'catch', 'class', 'const', 'continue', 'def', 'default', 'do',
      'else', 'enum', 'extends', 'final', 'finally', 'for', 'goto', 'if', 'implements', 'import', 'in', 'instanceof',
      'interface', 'new', 'package', 'private', 'protected', 'public', 'return', 'static', 'super', 'switch',
      'this', 'throw', 'throws', 'trait', 'try', 'while'
    ],
    pipelineKeywords: [
      'agent', 'any', 'environment', 'options', 'parameters', 'pipeline', 'post', 'stage', 'stages', 'steps', 'when'
    ],
    constants: ['true', 'false', 'null'],
    steps: [
      'archiveArtifacts', 'checkout', 'cleanWs', 'dir', 'echo', 'error', 'git', 'input', 'junit', 'parallel', 'retry',
      'script', 'sh', 'stash', 'timeout', 'timestamps', 'unstash', 'withCredentials', 'withEnv'
    ],
    operators: [
      '=', '>', '<', '!', '~', '?', ':', '==', '<=', '>=', '!=', '&&', '||', '++', '--', '+', '-', '*', '/', '&', '|', '^', '%',
      '+=', '-=', '*=', '/=', '&=', '|=', '^=', '%=', '<<', '>>', '>>>', '...', '..', '?.', '*.', '.@', '.&'
    ],
    symbols: /[=><!~?:&|+\-*\/\^%]+/,
    escapes: /\\(?:[btnfr"'\\]|u[0-9A-Fa-f]{4})/,
    tokenizer: {
      root: [
        [/[a-zA-Z_$][\w$]*/, {
          cases: {
            '@keywords': 'keyword',
            '@pipelineKeywords': 'keyword.flow',
            '@constants': 'constant',
            '@steps': 'type.identifier',
            '@default': 'identifier'
          }
        }],
        { include: '@whitespace' },
        [/[{}()\[\]]/, '@brackets'],
        [/[<>](?!@symbols)/, '@brackets'],
        [/@symbols/, { cases: { '@operators': 'operator', '@default': '' } }],
        [/\d*\.\d+([eE][\-+]?\d+)?[fFdD]?/, 'number.float'],
        [/0[xX][0-9a-fA-F_]+[lL]?/, 'number.hex'],
        [/\d+[lLfFdD]?/, 'number'],
        [/[;,.]/, 'delimiter'],
        [/"([^"\\]|\\.)*$/, 'string.invalid'],
        [/"""/, 'string', '@multilineString'],
        [/"/, 'string', '@stringDouble'],
        [/'([^'\\]|\\.)*$/, 'string.invalid'],
        [/'''/, 'string', '@multilineSingleString'],
        [/'/, 'string', '@stringSingle']
      ],
      whitespace: [
        [/[ \t\r\n]+/, ''],
        [/\/\*/, 'comment', '@comment'],
        [/\/\/.*$/, 'comment']
      ],
      comment: [
        [/[^\/*]+/, 'comment'],
        [/\*\//, 'comment', '@pop'],
        [/[\/*]/, 'comment']
      ],
      stringDouble: [
        [/[^\\"$]+/, 'string'],
        [/@escapes/, 'string.escape'],
        [/\\./, 'string.escape.invalid'],
        [/\$\{/, 'delimiter.bracket', '@bracketCounting'],
        [/\$/, 'string'],
        [/"/, 'string', '@pop']
      ],
      stringSingle: [
        [/[^\\']+/, 'string'],
        [/@escapes/, 'string.escape'],
        [/\\./, 'string.escape.invalid'],
        [/'/, 'string', '@pop']
      ],
      multilineString: [
        [/"""/, 'string', '@pop'],
        [/\$\{/, 'delimiter.bracket', '@bracketCounting'],
        [/./, 'string']
      ],
      multilineSingleString: [
        [/'''/, 'string', '@pop'],
        [/./, 'string']
      ],
      bracketCounting: [
        [/\{/, 'delimiter.bracket', '@bracketCounting'],
        [/\}/, 'delimiter.bracket', '@pop'],
        { include: 'root' }
      ]
    }
  });
};

function defaultBuildEnvironmentValues() {
  return {
    name: 'gradle7-jdk11',
    buildImage: 'cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/gradle:7-jdk11',
    isDefault: false
  };
}

function defaultRuntimeEnvironmentValues() {
  return {
    name: 'springboot-jdk11-aliyun',
    runtimeBaseImage: 'cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/dragonwell:11-anolis',
    artifactDeployPath: '',
    dockerfilePath: 'java/jar/Dockerfile',
    isDefault: false
  };
}

function showError(fallback: string) {
  return (error: Error) => message.error(error instanceof Error ? error.message : fallback);
}
