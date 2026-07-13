import { useEffect, useState } from 'react';
import { Card, Col, Row, Spin, Statistic } from 'antd';
import { DollarOutlined, ShoppingCartOutlined, TruckOutlined, UndoOutlined } from '@ant-design/icons';
import { authed } from '@flash-mall/shared';
import type { MerchantDashboardStats } from '@flash-mall/shared';

export default function DashboardPage() {
  const [stats, setStats] = useState<MerchantDashboardStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    authed<MerchantDashboardStats>('/api/merchant/dashboard/stats').then((response) => {
      if (response.ok) setStats(response.data);
      setLoading(false);
    });
  }, []);

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />;
  if (!stats) return <div>经营数据加载失败</div>;

  const navigate = (path: string) => window.dispatchEvent(new CustomEvent('flash-merchant:navigate', { detail: { path } }));
  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} md={12} xl={6}>
        <Card hoverable onClick={() => navigate('/merchant/orders')}>
          <Statistic title="总订单" value={stats.order_count} prefix={<ShoppingCartOutlined />} />
        </Card>
      </Col>
      <Col xs={24} md={12} xl={6}>
        <Card>
          <Statistic title="销售额(元)" value={(stats.sales_amount_fen / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })} prefix={<DollarOutlined />} />
        </Card>
      </Col>
      <Col xs={24} md={12} xl={6}>
        <Card hoverable onClick={() => navigate('/merchant/orders')}>
          <Statistic title="待发货" value={stats.ship_pending_count} prefix={<TruckOutlined />} valueStyle={{ color: '#1677ff' }} />
        </Card>
      </Col>
      <Col xs={24} md={12} xl={6}>
        <Card hoverable onClick={() => navigate('/merchant/refunds')}>
          <Statistic title="退款处理中" value={stats.refund_pending_count} prefix={<UndoOutlined />} valueStyle={{ color: '#fa8c16' }} />
        </Card>
      </Col>
    </Row>
  );
}
