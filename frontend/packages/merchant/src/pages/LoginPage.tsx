import { useState } from 'react';
import { Button, Card, Form, Input, Typography, message } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { api, setRefreshToken, setToken } from '@flash-mall/shared';
import type { LoginResp } from '@flash-mall/shared';

const { Title, Paragraph } = Typography;

export default function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [loading, setLoading] = useState(false);

  const login = async (values: { phone: string; password: string }) => {
    setLoading(true);
    try {
      const response = await api<LoginResp>('/api/auth/login', { method: 'POST', jsonBody: values });
      if (!response.ok || !response.data.access_token) {
        message.error('登录失败，请检查手机号和密码');
        return;
      }
      setToken(response.data.access_token);
      if (response.data.refresh_token) setRefreshToken(response.data.refresh_token);
      onLogin();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: 'flex', minHeight: '100vh', alignItems: 'center', justifyContent: 'center', background: '#f0f5ff' }}>
      <Card style={{ width: 420 }}>
        <Title level={3} style={{ textAlign: 'center' }}>Flash Mall 商家后台</Title>
        <Paragraph type="secondary" style={{ textAlign: 'center' }}>使用已绑定商家的普通账号登录</Paragraph>
        <Form onFinish={login} autoComplete="off">
          <Form.Item name="phone" rules={[{ required: true, message: '请输入手机号' }]}>
            <Input prefix={<UserOutlined />} placeholder="手机号" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>登录商家后台</Button>
        </Form>
      </Card>
    </div>
  );
}
