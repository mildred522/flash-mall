import { useEffect, useState } from 'react';
import { Alert, Button, Card, Form, Input, Space, Tabs, Typography, message } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { api, setRefreshToken, setToken } from '@flash-mall/shared';
import type { LoginResp, SendCodeResp } from '@flash-mall/shared';

const { Title, Paragraph } = Typography;

type AuthTab = 'login' | 'register';
type LoginValues = { phone: string; password: string };
type RegisterValues = { phone: string; code: string; password: string };

function persistLogin(response: LoginResp) {
  setToken(response.access_token);
  if (response.refresh_token) setRefreshToken(response.refresh_token);
}

export default function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [tab, setTab] = useState<AuthTab>('login');
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [debugCode, setDebugCode] = useState('');
  const [registerForm] = Form.useForm<RegisterValues>();

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = window.setTimeout(() => setCooldown((value) => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [cooldown]);

  const login = async (values: LoginValues) => {
    setLoading(true);
    try {
      const response = await api<LoginResp>('/api/auth/login', { method: 'POST', jsonBody: values });
      if (!response.ok || !response.data.access_token) {
        message.error('登录失败，请检查手机号和密码');
        return;
      }
      persistLogin(response.data);
      onLogin();
    } finally {
      setLoading(false);
    }
  };

  const sendCode = async () => {
    let phone: string;
    try {
      phone = await registerForm.validateFields(['phone']).then((values) => values.phone);
    } catch {
      return;
    }
    setSending(true);
    try {
      const response = await api<SendCodeResp>('/api/auth/code/send', {
        method: 'POST',
        jsonBody: { phone, scene: 'register' },
      });
      if (!response.ok) {
        message.error(response.status === 429 ? '验证码发送过于频繁，请稍后再试' : '验证码发送失败');
        return;
      }
      setDebugCode(response.data.debug_code || '');
      setCooldown(60);
      message.success('验证码已发送');
    } finally {
      setSending(false);
    }
  };

  const register = async (values: RegisterValues) => {
    setLoading(true);
    try {
      const response = await api<LoginResp>('/api/auth/register', {
        method: 'POST',
        jsonBody: { ...values, display_name: '' },
      });
      if (!response.ok || !response.data.access_token) {
        message.error(response.status === 409 ? '该手机号已经注册，请直接登录' : '注册失败，请检查验证码和填写内容');
        return;
      }
      persistLogin(response.data);
      message.success('注册成功');
      onLogin();
    } finally {
      setLoading(false);
    }
  };

  const loginForm = (
    <Form<LoginValues> onFinish={login} autoComplete="off" layout="vertical">
      <Form.Item label="手机号" name="phone" rules={[{ required: true, message: '请输入手机号' }]}>
        <Input prefix={<UserOutlined />} placeholder="手机号" />
      </Form.Item>
      <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
        <Input.Password prefix={<LockOutlined />} placeholder="密码" />
      </Form.Item>
      <Button type="primary" htmlType="submit" loading={loading} block>登录商家后台</Button>
    </Form>
  );

  const registerFormContent = (
    <Form<RegisterValues> form={registerForm} onFinish={register} autoComplete="off" layout="vertical">
      <Form.Item
        label="手机号"
        name="phone"
        rules={[
          { required: true, message: '请输入手机号' },
          { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号' },
        ]}
      >
        <Input prefix={<UserOutlined />} placeholder="手机号" />
      </Form.Item>
      <Form.Item label="验证码" required>
        <Space.Compact block>
          <Form.Item name="code" noStyle rules={[{ required: true, message: '请输入验证码' }]}>
            <Input aria-label="验证码" placeholder="验证码" />
          </Form.Item>
          <Button onClick={sendCode} loading={sending} disabled={cooldown > 0}>
            {cooldown > 0 ? `重新发送（${cooldown}s）` : '发送验证码'}
          </Button>
        </Space.Compact>
      </Form.Item>
      {debugCode ? <Alert type="info" showIcon message={`演示验证码：${debugCode}`} style={{ marginBottom: 16 }} /> : null}
      <Form.Item
        label="密码"
        name="password"
        rules={[{ required: true, min: 6, message: '密码至少需要 6 位' }]}
      >
        <Input.Password prefix={<LockOutlined />} placeholder="至少 6 位密码" />
      </Form.Item>
      <Button type="primary" htmlType="submit" loading={loading} block>注册账号</Button>
    </Form>
  );

  return (
    <div style={{ display: 'flex', minHeight: '100vh', alignItems: 'center', justifyContent: 'center', background: '#f0f5ff' }}>
      <Card style={{ width: 440 }}>
        <Title level={3} style={{ textAlign: 'center' }}>Flash Mall 商家后台</Title>
        <Paragraph type="secondary" style={{ textAlign: 'center' }}>注册普通账号，审核通过后即可经营店铺</Paragraph>
        <Tabs
          activeKey={tab}
          destroyOnHidden
          onChange={(key) => setTab(key as AuthTab)}
          items={[
            { key: 'login', label: '登录', children: loginForm },
            { key: 'register', label: '注册', children: registerFormContent },
          ]}
        />
      </Card>
    </div>
  );
}
