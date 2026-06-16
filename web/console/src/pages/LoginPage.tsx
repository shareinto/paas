import { IdcardOutlined, LockOutlined, MailOutlined, UserOutlined } from '@ant-design/icons';
import { Alert, Button, Form, Input, Segmented, Typography } from 'antd';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { login, oidcLoginURL, register } from '../api';
import { useSession } from '../app/store';

type LoginValues = { account: string; password: string };
type RegisterValues = LoginValues & { displayName: string; email: string; confirmPassword: string };

export function LoginPage() {
  const navigate = useNavigate();
  const setSession = useSession((state) => state.setSession);
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const localLogin = useMutation({
    mutationFn: (values: LoginValues) => login(values.account, values.password),
    onSuccess: (data) => {
      setSession(data.token, data.userName);
      navigate('/projects');
    }
  });
  const localRegister = useMutation({
    mutationFn: (values: RegisterValues) => register({
      account: values.account,
      displayName: values.displayName,
      email: values.email,
      password: values.password
    }),
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
        <Typography.Title level={2}>CloudDeliver</Typography.Title>
        <Typography.Text type="secondary">统一应用交付入口</Typography.Text>
        <Segmented
          block
          className="login-form"
          value={mode}
          onChange={(value) => setMode(value as 'login' | 'register')}
          options={[
            { label: '账号登录', value: 'login' },
            { label: '注册账号', value: 'register' }
          ]}
        />
        {mode === 'login' ? (
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
        ) : (
          <Form layout="vertical" className="login-form" onFinish={(values) => localRegister.mutate(values)}>
            <Form.Item label="账号" name="account" rules={[{ required: true, message: '请输入账号' }]}>
              <Input prefix={<UserOutlined />} placeholder="请输入账号" />
            </Form.Item>
            <Form.Item label="显示名称" name="displayName" rules={[{ required: true, message: '请输入显示名称' }]}>
              <Input prefix={<IdcardOutlined />} placeholder="请输入显示名称" />
            </Form.Item>
            <Form.Item
              label="邮箱"
              name="email"
              rules={[
                { required: true, message: '请输入邮箱' },
                { type: 'email', message: '请输入有效的邮箱地址' }
              ]}
            >
              <Input prefix={<MailOutlined />} placeholder="请输入邮箱" />
            </Form.Item>
            <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password prefix={<LockOutlined />} placeholder="请输入密码" />
            </Form.Item>
            <Form.Item
              label="确认密码"
              name="confirmPassword"
              dependencies={['password']}
              rules={[
                { required: true, message: '请再次输入密码' },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue('password') === value) return Promise.resolve();
                    return Promise.reject(new Error('两次输入的密码不一致'));
                  }
                })
              ]}
            >
              <Input.Password prefix={<LockOutlined />} placeholder="请再次输入密码" />
            </Form.Item>
            {localRegister.isError && <Alert type="error" showIcon message="注册失败，请检查账号信息" />}
            <Button type="primary" htmlType="submit" block loading={localRegister.isPending}>创建账号并登录</Button>
          </Form>
        )}
      </div>
    </div>
  );
}
