import { useState, useEffect } from 'react';
import { Card, Col, Row, Statistic, Spin } from 'antd';
import { ShoppingCartOutlined, DollarOutlined, UserOutlined, ShoppingOutlined } from '@ant-design/icons';
import { authed } from '@flash-mall/shared';
import type { AdminDashboardStats } from '@flash-mall/shared';

export default function DashboardPage() {
  const [stats, setStats] = useState<AdminDashboardStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    authed<AdminDashboardStats>('/api/admin/dashboard/stats').then((res) => {
      if (res.ok) setStats(res.data);
      setLoading(false);
    });
  }, []);

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />;
  if (!stats) return <div>加载失败</div>;

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总订单数" value={stats.total_orders} prefix={<ShoppingCartOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="总营收(元)" value={(stats.total_revenue_fen / 100).toFixed(2)} prefix={<DollarOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="用户数" value={stats.total_users} prefix={<UserOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="商品数" value={stats.total_products} prefix={<ShoppingOutlined />} />
          </Card>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={6}>
          <Card><Statistic title="待支付" value={stats.pending_orders} valueStyle={{ color: '#fa8c16' }} /></Card>
        </Col>
        <Col span={6}>
          <Card><Statistic title="已支付" value={stats.paid_orders} valueStyle={{ color: '#1890ff' }} /></Card>
        </Col>
        <Col span={6}>
          <Card><Statistic title="已发货" value={stats.shipped_orders} valueStyle={{ color: '#722ed1' }} /></Card>
        </Col>
        <Col span={6}>
          <Card><Statistic title="已完成" value={stats.completed_orders} valueStyle={{ color: '#52c41a' }} /></Card>
        </Col>
      </Row>
    </div>
  );
}
