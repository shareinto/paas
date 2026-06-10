import { SaveOutlined } from '@ant-design/icons';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Alert, Button, Card, Form, Input, Space, Switch, Typography, message } from 'antd';
import { useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getApplication, updateApplication } from '../api';
import { PageHeader } from '../components/PageHeader';

export function EditApplicationPage() {
  const { id = '' } = useParams();
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const { data: app, isLoading: appLoading } = useQuery({ queryKey: ['application', id], queryFn: () => getApplication(id), enabled: !!id });

  useEffect(() => {
    if (!app) return;
    form.setFieldsValue({
      displayName: app.displayName,
      description: app.description,
      disabled: app.status === 'disabled'
    });
  }, [app, form]);

  const mutation = useMutation({
    mutationFn: (values: any) => updateApplication(id, {
      displayName: values.displayName,
      description: values.description,
      disabled: values.disabled
    }),
    onSuccess: () => {
      message.success('应用已保存');
      navigate(`/apps/${id}`);
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '应用保存失败')
  });

  return (
    <>
      <PageHeader title="编辑应用" extra={<Button type="primary" icon={<SaveOutlined />} loading={mutation.isPending} onClick={() => form.submit()}>保存</Button>} />
      {mutation.error && <Alert className="form-alert" type="error" showIcon message={(mutation.error as Error).message || '应用保存失败'} />}
      <Form form={form} layout="vertical" disabled={appLoading} onFinish={(values) => mutation.mutate(values)}>
        <Card className="summary-card" title="基础信息">
          <Space direction="vertical" size={0}>
            <Typography.Text type="secondary">应用标识</Typography.Text>
            <Typography.Text strong>{app?.name || '-'}</Typography.Text>
          </Space>
          <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="描述" name="description">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item label="禁用应用" name="disabled" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Card>

        <Card className="summary-card" title="构建流水线">
          <Typography.Text type="secondary">代码源和 BuildSpec 已从应用配置中拆出，请在应用详情的构建页签维护命名流水线。</Typography.Text>
        </Card>
      </Form>
    </>
  );
}
