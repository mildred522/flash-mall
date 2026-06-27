import { useEffect, useRef, useState } from 'react';
import { Button, Descriptions, Modal, Popconfirm, Space, Table, Tag, message } from 'antd';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed, STATUS_MAP, formatPriceFen } from '@flash-mall/shared';
import type {
  ActionResp,
  AdminOrderListItem,
  AdminOrderListResp,
  AdminOrderStatusLogItem,
  AdminOrderStatusLogResp,
  OrderDetailResp,
} from '@flash-mall/shared';

const statusColors: Record<string, string> = {
  pending: 'orange', paid: 'blue', closed: 'default', shipped: 'purple',
  completed: 'green', refund: 'orange', refunded: 'red',
};

function statusTag(status: number, fallback?: string) {
  const st = STATUS_MAP[status];
  return st ? <Tag color={statusColors[st.cls] || 'default'}>{st.text}</Tag> : <Tag>{fallback || '未知'}</Tag>;
}

function actionError(data: ActionResp | OrderDetailResp | AdminOrderStatusLogResp): string {
  if (data.error === 'order not in paid status' || data.error === 'order is not in paid status') {
    return '订单不是已支付状态，不能发货';
  }
  if (data.error === 'order cannot be refunded' || data.error === 'order cannot be refunded in current status') {
    return '当前订单状态不能退款';
  }
  if (data.error === 'status changed concurrently' || data.error === 'order status changed concurrently') {
    return '订单状态已变化，请刷新后重试';
  }
  if (data.error === 'order not found') {
    return '订单不存在';
  }
  return data.error || '';
}

export default function OrdersPage() {
  const actionRef = useRef<ActionType>();
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

  const openSecurityLogs = (orderId: string) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/security', securityKeyword: `order:${orderId}` },
    }));
  };

  const handleShip = async (orderId: string) => {
    const res = await authed<ActionResp>('/api/admin/orders/ship', { method: 'POST', jsonBody: { order_id: orderId } });
    const error = actionError(res.data);
    if (res.ok && !error) {
      message.success('发货成功');
      if (detail?.order_id === orderId) void openDetail(orderId);
      reload();
    }
    else message.error(error || '发货失败');
  };

  const handleRefund = async (orderId: string) => {
    const res = await authed<ActionResp>('/api/admin/orders/refund', {
      method: 'POST',
      jsonBody: { order_id: orderId, reason: 'admin refund' },
    });
    const error = actionError(res.data);
    if (res.ok && !error) {
      message.success('退款成功');
      if (detail?.order_id === orderId) void openDetail(orderId);
      reload();
    }
    else message.error(error || '退款失败');
  };

  const openDetail = async (orderId: string) => {
    setDetailOpen(true);
    setDetailLoading(true);
    setDetail(null);
    try {
      const res = await authed<OrderDetailResp>(`/api/admin/orders/detail?order_id=${encodeURIComponent(orderId)}`);
      const error = actionError(res.data);
      if (res.ok && !error) setDetail(res.data);
      else message.error(error || '订单详情加载失败');
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    if (initialOrderId) {
      void openDetail(initialOrderId);
    }
  }, [initialOrderId]);

  const openLogs = async (orderId: string) => {
    setLogsOpen(true);
    setLogsLoading(true);
    setLogsOrderId(orderId);
    setLogs([]);
    try {
      const res = await authed<AdminOrderStatusLogResp>(`/api/admin/orders/status-logs?order_id=${encodeURIComponent(orderId)}`);
      const error = actionError(res.data);
      if (res.ok && !error) setLogs(res.data.items || []);
      else message.error(error || '状态日志加载失败');
    } finally {
      setLogsLoading(false);
    }
  };

  const columns: ProColumns<AdminOrderListItem>[] = [
    { title: '订单号', dataIndex: 'order_id', ellipsis: true, width: 200 },
    {
      title: '用户ID',
      dataIndex: 'user_id',
      width: 100,
      valueType: 'digit',
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/users', userId: row.user_id } }))}
        >
          {row.user_id}
        </Button>
      ),
    },
    { title: '商品ID', dataIndex: 'product_id', hideInTable: true, valueType: 'digit' },
    { title: '开始日期', dataIndex: 'created_from', hideInTable: true, valueType: 'date' },
    { title: '结束日期', dataIndex: 'created_to', hideInTable: true, valueType: 'date' },
    {
      title: '商品',
      dataIndex: 'product_name',
      ellipsis: true,
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/products', productId: row.product_id } }))}
        >
          {row.product_name || row.product_id}
        </Button>
      ),
    },
    { title: '数量', dataIndex: 'amount', width: 80, search: false },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) => statusTag(row.status, row.status_text),
      valueEnum: {
        '-1': { text: '全部' },
        '0': { text: '待支付' }, '1': { text: '已支付' }, '2': { text: '已关闭' },
        '3': { text: '已发货' }, '4': { text: '已收货' }, '5': { text: '退款中' }, '6': { text: '已退款' },
      },
    },
    {
      title: '金额', dataIndex: 'payable_amount_fen', width: 120, search: false,
      render: (_, row) => `¥${formatPriceFen(row.payable_amount_fen)}`,
    },
    { title: '下单时间', dataIndex: 'create_time', width: 180, search: false },
    {
      title: '操作',
      width: 300,
      search: false,
      render: (_, row) => (
        <Space size={4}>
          <Button type="link" size="small" onClick={() => openDetail(row.order_id)}>详情</Button>
          <Button type="link" size="small" onClick={() => openLogs(row.order_id)}>日志</Button>
          <Button type="link" size="small" onClick={() => openSecurityLogs(row.order_id)}>安全</Button>
          {row.status === 1 && (
            <Popconfirm title="确认发货？" onConfirm={() => handleShip(row.order_id)}>
              <Button type="link" size="small">发货</Button>
            </Popconfirm>
          )}
          {(row.status === 0 || row.status === 1) && (
            <Popconfirm title="确认退款？" onConfirm={() => handleRefund(row.order_id)}>
              <Button type="link" size="small" danger>退款</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

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
          return {
            data: res.ok ? res.data.items || [] : [],
            total: res.ok ? res.data.total : 0,
            success: res.ok,
          };
        }}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20 }}
      />

      <Modal
        title="订单详情"
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={[
          <Button key="close" onClick={() => setDetailOpen(false)}>关闭</Button>,
          <Button
            key="security"
            disabled={!detail?.order_id}
            onClick={() => {
              if (!detail?.order_id) return;
              setDetailOpen(false);
              openSecurityLogs(detail.order_id);
            }}
          >
            安全日志
          </Button>,
          detail?.status === 1 && (
            <Popconfirm key="ship" title="确认发货？" onConfirm={() => detail && handleShip(detail.order_id)}>
              <Button>发货</Button>
            </Popconfirm>
          ),
          (detail?.status === 0 || detail?.status === 1) && (
            <Popconfirm key="refund" title="确认退款？" onConfirm={() => detail && handleRefund(detail.order_id)}>
              <Button danger>退款</Button>
            </Popconfirm>
          ),
          <Button
            key="user"
            disabled={!detail?.user_id}
            onClick={() => {
              if (!detail?.user_id) return;
              setDetailOpen(false);
              window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/users', userId: detail.user_id } }));
            }}
          >
            查看用户
          </Button>,
          <Button
            key="product"
            type="primary"
            disabled={!detail?.product_id}
            onClick={() => {
              if (!detail?.product_id) return;
              setDetailOpen(false);
              window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/products', productId: detail.product_id } }));
            }}
          >
            查看商品
          </Button>,
        ]}
        loading={detailLoading}
        width={760}
      >
        {detail && (
          <Descriptions column={2} bordered size="small">
            <Descriptions.Item label="订单号" span={2}>{detail.order_id}</Descriptions.Item>
            <Descriptions.Item label="用户ID">{detail.user_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="商品ID">{detail.product_id}</Descriptions.Item>
            <Descriptions.Item label="商品">{detail.product_name}</Descriptions.Item>
            <Descriptions.Item label="数量">{detail.amount}</Descriptions.Item>
            <Descriptions.Item label="订单状态">{statusTag(detail.status, detail.status_text)}</Descriptions.Item>
            <Descriptions.Item label="原单价">¥{formatPriceFen(detail.origin_unit_price_fen)}</Descriptions.Item>
            <Descriptions.Item label="活动单价">¥{formatPriceFen(detail.sale_unit_price_fen)}</Descriptions.Item>
            <Descriptions.Item label="优惠">¥{formatPriceFen(detail.discount_amount_fen)}</Descriptions.Item>
            <Descriptions.Item label="应付">¥{formatPriceFen(detail.payable_amount_fen)}</Descriptions.Item>
            <Descriptions.Item label="支付单号">{detail.payment_order_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="支付状态">{detail.payment_status_text || detail.payment_status || '-'}</Descriptions.Item>
            <Descriptions.Item label="活动类型">{detail.promotion_type || '-'}</Descriptions.Item>
            <Descriptions.Item label="活动标签">{detail.promotion_tag || '-'}</Descriptions.Item>
            <Descriptions.Item label="下单时间" span={2}>{detail.create_time}</Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      <Modal
        title={`状态日志 ${logsOrderId}`}
        open={logsOpen}
        onCancel={() => setLogsOpen(false)}
        footer={[
          <Button key="close" onClick={() => setLogsOpen(false)}>关闭</Button>,
          <Button
            key="security"
            type="primary"
            disabled={!logsOrderId}
            onClick={() => {
              setLogsOpen(false);
              openSecurityLogs(logsOrderId);
            }}
          >
            安全日志
          </Button>,
        ]}
        width={820}
      >
        <Table<AdminOrderStatusLogItem>
          rowKey="id"
          loading={logsLoading}
          dataSource={logs}
          pagination={false}
          size="small"
          columns={[
            { title: '时间', dataIndex: 'create_time', width: 170 },
            { title: '原状态', dataIndex: 'from_status_text', width: 100 },
            { title: '新状态', dataIndex: 'to_status_text', width: 100 },
            {
              title: '操作人',
              dataIndex: 'operator_id',
              width: 100,
              render: (_, row) => row.operator_id ? (
                <Button
                  type="link"
                  size="small"
                  onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/users', userId: row.operator_id } }))}
                >
                  {row.operator_id}
                </Button>
              ) : '-',
            },
            { title: '备注', dataIndex: 'remark', ellipsis: true },
          ]}
        />
      </Modal>
    </>
  );
}
