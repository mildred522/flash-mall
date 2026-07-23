import { useEffect, useRef, useState } from 'react';
import { message } from 'antd';
import { ProTable, type ActionType } from '@ant-design/pro-components';
import { authed } from '@flash-mall/shared';
import type {
  ActionResp,
  AdminOrderListItem,
  AdminOrderListResp,
  AdminOrderStatusLogItem,
  AdminOrderStatusLogResp,
  OrderDetailResp,
} from '@flash-mall/shared';
import OrderDetailModal from '../components/orders/OrderDetailModal';
import OrderStatusLogsModal from '../components/orders/OrderStatusLogsModal';
import { createOrderColumns } from '../components/orders/orderColumns';
import { orderActionError } from '../components/orders/orderModel';

export default function OrdersPage() {
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<OrderDetailResp | null>(null);
  const [logsOpen, setLogsOpen] = useState(false);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logs, setLogs] = useState<AdminOrderStatusLogItem[]>([]);
  const [logsOrderId, setLogsOrderId] = useState('');
  const [initialOrderId] = useState(() => {
    const orderId = window.__flashAdminOrderOrderId || '';
    window.__flashAdminOrderOrderId = '';
    return orderId;
  });
  const [initialProductId] = useState(() => {
    const productId = window.__flashAdminOrderProductId || 0;
    window.__flashAdminOrderProductId = 0;
    return productId;
  });
  const [initialUserId] = useState(() => {
    const userId = window.__flashAdminOrderUserId || 0;
    window.__flashAdminOrderUserId = 0;
    return userId;
  });
  const [initialStatus] = useState(() => {
    const status = window.__flashAdminOrderStatus;
    window.__flashAdminOrderStatus = undefined;
    return status;
  });

  const reload = () => actionRef.current?.reload();
  const navigate = (path: string, key: 'userId' | 'productId', value: number) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path, [key]: value } }));
  };
  const openSecurityLogs = (orderId: string) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/security', securityKeyword: `order:${orderId}` },
    }));
  };

  const openDetail = async (orderId: string) => {
    setDetailOpen(true);
    setDetailLoading(true);
    setDetail(null);
    try {
      const res = await authed<OrderDetailResp>(`/api/admin/orders/detail?order_id=${encodeURIComponent(orderId)}`);
      const error = orderActionError(res.data);
      if (res.ok && !error) setDetail(res.data);
      else message.error(error || '订单详情加载失败');
    } finally {
      setDetailLoading(false);
    }
  };

  const handleShip = async (orderId: string) => {
    const res = await authed<ActionResp>('/api/admin/orders/ship', { method: 'POST', jsonBody: { order_id: orderId } });
    const error = orderActionError(res.data);
    if (res.ok && !error) {
      message.success('发货成功');
      if (detail?.order_id === orderId) void openDetail(orderId);
      reload();
    } else {
      message.error(error || '发货失败');
    }
  };

  const handleRefund = async (orderId: string) => {
    const res = await authed<ActionResp>('/api/admin/orders/refund', {
      method: 'POST', jsonBody: { order_id: orderId, reason: 'admin refund' },
    });
    const error = orderActionError(res.data);
    if (res.ok && !error) {
      message.success('退款成功');
      if (detail?.order_id === orderId) void openDetail(orderId);
      reload();
    } else {
      message.error(error || '退款失败');
    }
  };

  useEffect(() => {
    if (initialOrderId) void openDetail(initialOrderId);
  }, [initialOrderId]);

  const openLogs = async (orderId: string) => {
    setLogsOpen(true);
    setLogsLoading(true);
    setLogsOrderId(orderId);
    setLogs([]);
    try {
      const res = await authed<AdminOrderStatusLogResp>(
        `/api/admin/orders/status-logs?order_id=${encodeURIComponent(orderId)}`,
      );
      const error = orderActionError(res.data);
      if (res.ok && !error) setLogs(res.data.items || []);
      else message.error(error || '状态日志加载失败');
    } finally {
      setLogsLoading(false);
    }
  };

  const columns = createOrderColumns({
    detail: (orderId) => void openDetail(orderId),
    logs: (orderId) => void openLogs(orderId),
    security: openSecurityLogs,
    ship: (orderId) => void handleShip(orderId),
    refund: (orderId) => void handleRefund(orderId),
    navigate,
  });

  return (
    <>
      <ProTable<AdminOrderListItem>
        actionRef={actionRef}
        columns={columns}
        rowKey="order_id"
        params={{ initialOrderId, initialProductId, initialUserId, initialStatus }}
        request={async (params) => {
          const query = new URLSearchParams();
          query.set('page', String(params.current || 1));
          query.set('page_size', String(params.pageSize || 20));
          const status = params.status !== undefined ? params.status : params.initialStatus;
          if (status !== undefined && String(status) !== '-1') query.set('status', String(status));
          const userId = params.user_id || params.initialUserId;
          if (userId) query.set('user_id', String(userId));
          const productId = params.product_id || params.initialProductId;
          if (productId) query.set('product_id', String(productId));
          if (params.product_name) query.set('product_name', String(params.product_name));
          if (params.created_from) query.set('created_from', String(params.created_from));
          if (params.created_to) query.set('created_to', String(params.created_to));
          const orderId = params.order_id || params.initialOrderId;
          if (orderId) query.set('order_id', String(orderId));
          const res = await authed<AdminOrderListResp>(`/api/admin/orders?${query}`);
          return { data: res.ok ? res.data.items || [] : [], total: res.ok ? res.data.total : 0, success: res.ok };
        }}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20 }}
      />
      <OrderDetailModal open={detailOpen} loading={detailLoading} detail={detail} onClose={() => setDetailOpen(false)}
        onSecurity={openSecurityLogs} onShip={(orderId) => void handleShip(orderId)}
        onRefund={(orderId) => void handleRefund(orderId)} onNavigate={navigate} />
      <OrderStatusLogsModal open={logsOpen} loading={logsLoading} orderId={logsOrderId} logs={logs}
        onClose={() => setLogsOpen(false)} onSecurity={openSecurityLogs}
        onUser={(userId) => navigate('/admin/users', 'userId', userId)} />
    </>
  );
}
