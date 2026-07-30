import { useState } from 'react';
import { Card, Form, Input, Button, Typography, message } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { DEMO_ADMIN_CREDENTIALS, loginAdmin } from './admin-login';

const { Title } = Typography;

interface Props {
  onLogin: () => void;
}

export default function LoginPage({ onLogin }: Props) {
  const [loading, setLoading] = useState<'form' | 'demo' | null>(null);

  const onFinish = async (values: { phone: string; password: string }) => {
    setLoading('form');
    const loggedIn = await loginAdmin(values);
    setLoading(null);

    if (loggedIn) {
      onLogin();
    } else {
      message.error('登录失败，请检查手机号和密码');
    }
  };

  const onDemoLogin = async () => {
    setLoading('demo');
    const loggedIn = await loginAdmin(DEMO_ADMIN_CREDENTIALS);
    setLoading(null);
    if (loggedIn) {
      onLogin();
    } else {
      message.error('演示管理员登录失败，请确认认证服务和演示数据已初始化');
    }
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
      <Card style={{ width: 400 }}>
        <Title level={3} style={{ textAlign: 'center', marginBottom: 24 }}>Flash Mall 管理后台</Title>
        <Form onFinish={onFinish} autoComplete="off">
          <Form.Item name="phone" rules={[{ required: true, message: '请输入手机号' }]}>
            <Input prefix={<UserOutlined />} placeholder="手机号" autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" autoComplete="current-password" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading === 'form'} disabled={loading === 'demo'} block>
              登录
            </Button>
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button onClick={onDemoLogin} loading={loading === 'demo'} disabled={loading === 'form'} block>
              演示管理员一键登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
