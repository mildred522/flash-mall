import { useState, useEffect } from 'react';
import { Card, Col, Row, Statistic, Spin } from 'antd';
import { ShoppingCartOutlined, DollarOutlined, UserOutlined, ShoppingOutlined, ShopOutlined, TagsOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { authed } from '@flash-mall/shared';
import type { AdminDashboardStats } from '@flash-mall/shared';

export default function DashboardPage() {
  const [stats, setStats] = useState<AdminDashboardStats | null>(null);
  const [loading, setLoading] = useState(true);

  const navigate = (detail: Record<string, unknown>) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail }));
  };

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
          <Card hoverable onClick={() => navigate({ path: '/admin/orders' })}>
            <Statistic title="总订单数" value={stats.total_orders} prefix={<ShoppingCartOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="总营收(元)" value={(stats.total_revenue_fen / 100).toFixed(2)} prefix={<DollarOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/users' })}>
            <Statistic title="用户数" value={stats.total_users} prefix={<UserOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/products' })}>
            <Statistic title="商品数" value={stats.total_products} prefix={<ShoppingOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/suppliers' })}>
            <Statistic title="供应商数" value={stats.total_suppliers} prefix={<ShopOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/promotions' })}>
            <Statistic title="促销数" value={stats.total_promotions} prefix={<TagsOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/promotions', effectStatus: 'active' })}>
            <Statistic title="生效活动" value={stats.active_promotions} prefix={<TagsOutlined />} valueStyle={{ color: '#1677ff' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/security' })}>
            <Statistic title="安全日志" value="查看" prefix={<SafetyCertificateOutlined />} />
          </Card>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/products', stockStatus: 2 })}><Statistic title="低库存商品" value={stats.low_stock_products} valueStyle={{ color: '#fa8c16' }} /></Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/products', stockStatus: 3 })}><Statistic title="缺货商品" value={stats.out_of_stock_products} valueStyle={{ color: '#cf1322' }} /></Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/orders', orderStatus: 0 })}><Statistic title="待支付" value={stats.pending_orders} valueStyle={{ color: '#fa8c16' }} /></Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/orders', orderStatus: 1 })}><Statistic title="已支付" value={stats.paid_orders} valueStyle={{ color: '#1890ff' }} /></Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/orders', orderStatus: 3 })}><Statistic title="已发货" value={stats.shipped_orders} valueStyle={{ color: '#722ed1' }} /></Card>
        </Col>
        <Col span={6}>
          <Card hoverable onClick={() => navigate({ path: '/admin/orders', orderStatus: 4 })}><Statistic title="已完成" value={stats.completed_orders} valueStyle={{ color: '#52c41a' }} /></Card>
        </Col>
      </Row>
    </div>
  );
}
