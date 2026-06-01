import { useState } from 'react';
import { Card, Form, Input, Button, Typography, message } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { api, setToken, setRefreshToken } from '@flash-mall/shared';
import type { LoginResp } from '@flash-mall/shared';

const { Title } = Typography;

interface Props {
  onLogin: () => void;
}

export default function LoginPage({ onLogin }: Props) {
  const [loading, setLoading] = useState(false);

  const onFinish = async (values: { phone: string; password: string }) => {
    setLoading(true);
    const res = await api<LoginResp>('/api/auth/login', {
      method: 'POST',
      jsonBody: values,
    });
    setLoading(false);

    if (res.ok && res.data.access_token) {
      setToken(res.data.access_token);
      if (res.data.refresh_token) setRefreshToken(res.data.refresh_token);
      onLogin();
    } else {
      message.error('登录失败，请检查手机号和密码');
    }
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
      <Card style={{ width: 400 }}>
        <Title level={3} style={{ textAlign: 'center', marginBottom: 24 }}>Flash Mall 管理后台</Title>
        <Form onFinish={onFinish} autoComplete="off">
          <Form.Item name="phone" rules={[{ required: true, message: '请输入手机号' }]}>
            <Input prefix={<UserOutlined />} placeholder="手机号" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
