import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { Alert, Button, Form, Input, Space, Typography } from 'antd';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { login, oidcLoginURL } from '../api';
import { useSession } from '../app/store';

export function LoginPage() {
  const navigate = useNavigate();
  const setSession = useSession((state) => state.setSession);
  const localLogin = useMutation({
    mutationFn: (values: { account: string; password: string }) => login(values.account, values.password),
    onSuccess: (data) => {
      setSession(data.token, data.userName);
      navigate('/projects');
    }
  });
  const oidc = useMutation({
    mutationFn: oidcLoginURL,
    onSuccess: (url) => navigate(url)
  });
  return (
    <div className="login-page">
      <div className="login-panel">
        <Typography.Title level={2}>平台控制台</Typography.Title>
        <Typography.Text type="secondary">统一应用交付入口</Typography.Text>
        <Form layout="vertical" className="login-form" onFinish={(values) => localLogin.mutate(values)}>
          <Form.Item label="账号" name="account" rules={[{ required: true, message: '请输入账号' }]}>
            <Input prefix={<UserOutlined />} placeholder="请输入账号" />
          </Form.Item>
          <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="请输入密码" />
          </Form.Item>
          {localLogin.isError && <Alert type="error" showIcon message="登录失败，请检查账号或密码" />}
          <Button type="primary" htmlType="submit" block loading={localLogin.isPending}>登录</Button>
          <Button block onClick={() => oidc.mutate()} loading={oidc.isPending}>使用企业身份登录</Button>
        </Form>
      </div>
    </div>
  );
}
