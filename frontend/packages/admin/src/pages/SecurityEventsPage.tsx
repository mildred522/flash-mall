import { useEffect, useMemo, useState } from 'react';
import { Button, Input, Select, Space, Table, Tag, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { authed } from '@flash-mall/shared';
import type { SecurityEventItem, SecurityEventsRecentResp } from '@flash-mall/shared';

type SubjectNavigation = {
  path: string;
  orderId?: string;
  productId?: number;
  supplierId?: number;
  promotionId?: number;
  userId?: number;
};

const SECURITY_EVENT_TEXT: Record<string, string> = {
  login_password_success: '密码登录成功',
  login_password_fail: '密码登录失败',
  login_code_success: '验证码登录成功',
  login_code_fail: '验证码登录失败',
  send_code_success: '发送验证码',
  send_code_blocked: '验证码限流',
  refresh_success: '刷新登录',
  logout_success: '退出登录',
  logout_all_success: '退出全部设备',
  reset_password_success: '重置密码',
  reset_password_fail: '重置密码失败',
  register_success: '注册成功',
  register_fail: '注册失败',
  admin_user_enabled: '管理员启用用户',
  admin_user_disabled: '管理员禁用用户',
  admin_user_status_update_failed: '管理员更新用户状态失败',
  admin_order_shipped: '管理员发货',
  admin_order_refunded: '管理员退款',
  admin_product_created: '管理员创建商品',
  admin_product_updated: '管理员更新商品',
  admin_product_enabled: '管理员上架商品',
  admin_product_disabled: '管理员下架商品',
  admin_product_stock_adjusted: '管理员调整库存',
  admin_supplier_created: '管理员创建供应商',
  admin_supplier_updated: '管理员更新供应商',
  admin_supplier_enabled: '管理员启用供应商',
  admin_supplier_disabled: '管理员停用供应商',
  admin_promotion_created: '管理员创建促销',
  admin_promotion_updated: '管理员更新促销',
  admin_promotion_enabled: '管理员启用促销',
  admin_promotion_disabled: '管理员停用促销',
};

function eventText(type: string): string {
  return SECURITY_EVENT_TEXT[type] || type || '-';
}

function resultTag(result: string) {
  if (result === 'success') return <Tag color="green">成功</Tag>;
  if (result === 'blocked') return <Tag color="orange">拦截</Tag>;
  if (result === 'fail' || result === 'failed') return <Tag color="red">失败</Tag>;
  return <Tag>{result || '-'}</Tag>;
}

function resultMatches(actual: string, expected: string): boolean {
  actual = (actual || '').toLowerCase();
  expected = (expected || '').toLowerCase();
  return actual === expected || ((actual === 'fail' || actual === 'failed') && (expected === 'fail' || expected === 'failed'));
}

function reasonText(subject: string): string {
  const reason = (subject || '').match(/\breason:([A-Za-z0-9_-]+)/)?.[1] || '';
  const map: Record<string, string> = {
    not_found: '对象不存在',
    product_not_found: '商品不存在',
    invalid_status: '状态不允许',
    status_changed: '状态已变化',
    not_paid_status: '订单未支付',
    invalid_price: '价格不合法',
    invalid_discount: '折扣价不合法',
    invalid_window: '时间窗口不合法',
    window_conflict: '活动时间冲突',
    active_supplier_not_found: '启用供应商不存在',
    has_active_products: '仍有关联启用商品',
    insufficient_or_missing_bucket: '库存不足或分桶不存在',
    invalid_user_id: '用户ID不合法',
    invalid_status_value: '状态值不合法',
    self_disable_blocked: '禁止禁用当前管理员',
    store_failed: '存储更新失败',
  };
  return map[reason] || reason || '-';
}

function formatTime(seconds: number): string {
  if (!seconds) return '-';
  return new Date(seconds * 1000).toLocaleString();
}

function subjectNavigation(subject: string): SubjectNavigation | null {
  const value = subject || '';
  const order = value.match(/\border:([A-Za-z0-9_-]+)/);
  if (order?.[1]) return { path: '/admin/orders', orderId: order[1] };
  const promotion = value.match(/\bpromotion:(\d+)/);
  if (promotion?.[1]) return { path: '/admin/promotions', promotionId: Number(promotion[1]) };
  const supplier = value.match(/\bsupplier:(\d+)/);
  if (supplier?.[1]) return { path: '/admin/suppliers', supplierId: Number(supplier[1]) };
  const product = value.match(/\bproduct:(\d+)/);
  if (product?.[1]) return { path: '/admin/products', productId: Number(product[1]) };
  const operator = value.match(/\boperator:(\d+)/);
  if (operator?.[1]) return { path: '/admin/users', userId: Number(operator[1]) };
  const user = value.match(/\btarget_user:(\d+)/);
  if (user?.[1]) return { path: '/admin/users', userId: Number(user[1]) };
  return null;
}

export default function SecurityEventsPage() {
  const [items, setItems] = useState<SecurityEventItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [userId, setUserId] = useState(() => {
    const initialUserId = window.__flashAdminSecurityUserId || 0;
    window.__flashAdminSecurityUserId = 0;
    return initialUserId ? String(initialUserId) : '';
  });
  const [result, setResult] = useState<string>();
  const [eventType, setEventType] = useState<string>();
  const [keyword, setKeyword] = useState(() => {
    const initialKeyword = window.__flashAdminSecurityKeyword || '';
    window.__flashAdminSecurityKeyword = '';
    return initialKeyword;
  });

  const load = async () => {
    setLoading(true);
    try {
      const query = new URLSearchParams({ limit: '100' });
      if (userId) query.set('user_id', userId);
      if (result) query.set('result', result);
      if (eventType) query.set('event_type', eventType);
      if (keyword.trim()) query.set('keyword', keyword.trim());
      const res = await authed<SecurityEventsRecentResp>(`/api/admin/security/events/recent?${query}`);
      if (res.ok && !res.data.error) {
        setItems(res.data.items || []);
      } else {
        message.error(res.data.error || '安全日志加载失败');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const eventOptions = useMemo(() => Array.from(new Set([...Object.keys(SECURITY_EVENT_TEXT), ...items.map((item) => item.event_type).filter(Boolean)]))
    .map((type) => ({ value: type, label: eventText(type) })), [items]);

  const filteredItems = useMemo(() => {
    const userIdValue = Number(userId || 0);
    const keywordValue = keyword.trim().toLowerCase();
    return items.filter((item) => {
      if (userIdValue && item.user_id !== userIdValue) return false;
      if (result && !resultMatches(item.result, result)) return false;
      if (eventType && item.event_type !== eventType) return false;
      if (keywordValue) {
        const source = `${item.user_id || ''} ${item.subject || ''} ${item.ip || ''} ${item.user_agent || ''} ${item.event_type || ''} ${item.result || ''}`.toLowerCase();
        if (!source.includes(keywordValue)) return false;
      }
      return true;
    });
  }, [eventType, items, keyword, result, userId]);

  return (
    <Table<SecurityEventItem>
      rowKey={(row) => `${row.created_at}-${row.user_id}-${row.event_type}-${row.subject}`}
      loading={loading}
      dataSource={filteredItems}
      size="small"
      pagination={false}
      title={() => (
        <Space wrap>
          <Input
            allowClear
            inputMode="numeric"
            placeholder="用户ID"
            style={{ width: 120 }}
            value={userId}
            onChange={(event) => setUserId(event.target.value)}
          />
          <Select
            allowClear
            placeholder="事件"
            style={{ width: 180 }}
            options={eventOptions}
            value={eventType}
            onChange={setEventType}
          />
          <Select
            allowClear
            placeholder="结果"
            style={{ width: 120 }}
            options={[
              { value: 'success', label: '成功' },
              { value: 'blocked', label: '拦截' },
              { value: 'fail', label: '失败' },
              { value: 'failed', label: '失败' },
            ]}
            value={result}
            onChange={setResult}
          />
          <Input
            allowClear
            placeholder="用户ID/主体/IP/事件"
            style={{ width: 180 }}
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button>
          <Tag>{filteredItems.length}/{items.length}</Tag>
        </Space>
      )}
      columns={[
        { title: '时间', dataIndex: 'created_at', width: 180, render: (_, row) => formatTime(row.created_at) },
        { title: '事件', dataIndex: 'event_type', ellipsis: true, render: (_, row) => eventText(row.event_type) },
        { title: '结果', dataIndex: 'result', width: 90, render: (_, row) => resultTag(row.result) },
        {
          title: '用户ID',
          dataIndex: 'user_id',
          width: 100,
          render: (_, row) => row.user_id ? (
            <Button
              type="link"
              size="small"
              onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/users', userId: row.user_id } }))}
            >
              {row.user_id}
            </Button>
          ) : '-',
        },
        {
          title: '主体',
          dataIndex: 'subject',
          ellipsis: true,
          render: (_, row) => {
            const subject = row.subject || '';
            const navigation = subjectNavigation(subject);
            if (!subject) return '-';
            if (!navigation) return subject;
            return (
              <Button
                type="link"
                size="small"
                onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: navigation }))}
              >
                {subject}
              </Button>
            );
          },
        },
        { title: 'IP', dataIndex: 'ip', width: 150, render: (_, row) => row.ip || '-' },
        { title: '原因', dataIndex: 'subject', width: 160, render: (_, row) => reasonText(row.subject) },
        { title: 'User-Agent', dataIndex: 'user_agent', width: 220, ellipsis: true, render: (_, row) => row.user_agent || '-' },
      ]}
    />
  );
}
