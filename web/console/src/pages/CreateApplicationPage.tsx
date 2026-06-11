import { SaveOutlined } from '@ant-design/icons';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Alert, Button, Card, Form, Input, Select, Typography } from 'antd';
import { useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { createApplication, listProjects } from '../api';

const EMPTY_LIST: any[] = [];

export function CreateApplicationPage() {
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const fixedProjectId = searchParams.get('projectId') || '';
  const { data: projects = EMPTY_LIST } = useQuery({ queryKey: ['projects'], queryFn: listProjects });
  const fixedProject = fixedProjectId ? projects.find((project: any) => project.id === fixedProjectId) : undefined;

  useEffect(() => {
    if (fixedProjectId) {
      form.setFieldValue('projectId', fixedProjectId);
      return;
    }
    if (!form.getFieldValue('projectId') && projects[0]) {
      form.setFieldValue('projectId', projects[0].id);
    }
  }, [fixedProjectId, form, projects]);

  const createMutation = useMutation({
    mutationFn: createApplication,
    onSuccess: (app) => navigate(`/apps/${app.id}`)
  });

  const submit = async () => {
    const values = await form.validateFields();
    createMutation.mutate({
      projectId: values.projectId,
      name: values.name,
      displayName: values.displayName,
      description: values.description
    });
  };

  return (
    <div className="create-app-page">
      <div className="wizard-title">
        <Typography.Title level={3}>创建应用</Typography.Title>
        <Typography.Paragraph className="wizard-desc">
          应用只维护交付单元和默认环境；代码源、BuildSpec、运行时环境和触发规则在应用详情中创建构建流水线。
        </Typography.Paragraph>
      </div>

      {createMutation.error && <Alert className="form-alert" type="error" showIcon message={(createMutation.error as Error).message || '创建应用失败'} />}

      <Form
        form={form}
        layout="horizontal"
        className="wizard-form"
        labelCol={{ flex: '140px' }}
        wrapperCol={{ flex: 1 }}
        colon={false}
        initialValues={{ name: 'order-api', displayName: '订单服务' }}
      >
        <div className="create-app-grid">
          <div className="create-app-main">
            <Card title="基础信息">
              {fixedProjectId ? (
                <>
                  <Form.Item hidden name="projectId" rules={[{ required: true, message: '请选择项目' }]}>
                    <Input />
                  </Form.Item>
                  <div className="fixed-context-row">
                    <Typography.Text type="secondary">所属项目</Typography.Text>
                    <Typography.Text strong>{fixedProject?.displayName || fixedProject?.name || fixedProjectId}</Typography.Text>
                  </div>
                </>
              ) : (
                <Form.Item label="所属项目" name="projectId" rules={[{ required: true, message: '请选择项目' }]}>
                  <Select options={projects.map((project: any) => ({ value: project.id, label: project.displayName || project.name }))} />
                </Form.Item>
              )}
              <Form.Item label="应用标识" name="name" rules={[{ required: true, message: '请输入应用标识' }, { pattern: /^[a-z][a-z0-9-]{1,62}$/, message: '仅支持小写字母、数字和连字符' }]}>
                <Input placeholder="order-api" />
              </Form.Item>
              <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}>
                <Input placeholder="订单服务" />
              </Form.Item>
              <Form.Item label="描述" name="description">
                <Input.TextArea rows={3} placeholder="应用用途、负责人或交付边界" />
              </Form.Item>
            </Card>

            <Card title="默认环境">
              <div className="environment-preview">
                {['dev', 'test', 'staging', 'prod'].map((env) => <span key={env}>{env}</span>)}
              </div>
            </Card>
          </div>

          <div className="create-app-side">
            <Card title="后续配置">
              <Typography.Paragraph type="secondary">
                应用创建完成后，在应用详情的构建页签中创建一条或多条命名流水线，并为每条流水线配置代码源与 BuildSpec。
              </Typography.Paragraph>
            </Card>
          </div>
        </div>
        <div className="wizard-actions">
          <Button onClick={() => navigate(fixedProjectId ? `/projects/${fixedProjectId}` : '/apps')}>取消</Button>
          <Button type="primary" icon={<SaveOutlined />} loading={createMutation.isPending} onClick={submit}>创建应用</Button>
        </div>
      </Form>
    </div>
  );
}
