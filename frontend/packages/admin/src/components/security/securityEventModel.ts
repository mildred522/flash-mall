import type { SecurityEventItem } from '@flash-mall/shared';

export type SubjectNavigation = {
  path: string;
  orderId?: string;
  productId?: number;
  supplierId?: number;
  promotionId?: number;
  userId?: number;
};

export const SECURITY_EVENT_TEXT: Record<string, string> = {
  login_password_success: '密码登录成功', login_password_fail: '密码登录失败',
  login_code_success: '验证码登录成功', login_code_fail: '验证码登录失败',
  send_code_success: '发送验证码', send_code_blocked: '验证码限流', refresh_success: '刷新登录',
  logout_success: '退出登录', logout_all_success: '退出全部设备', reset_password_success: '重置密码',
  reset_password_fail: '重置密码失败', register_success: '注册成功', register_fail: '注册失败',
  admin_user_enabled: '管理员启用用户', admin_user_disabled: '管理员禁用用户',
  admin_user_status_update_failed: '管理员更新用户状态失败', admin_order_shipped: '管理员发货',
  admin_order_refunded: '管理员退款', admin_product_created: '管理员创建商品',
  admin_product_updated: '管理员更新商品', admin_product_enabled: '管理员上架商品',
  admin_product_disabled: '管理员下架商品', admin_product_stock_adjusted: '管理员调整库存',
  admin_supplier_created: '管理员创建供应商', admin_supplier_updated: '管理员更新供应商',
  admin_supplier_enabled: '管理员启用供应商', admin_supplier_disabled: '管理员停用供应商',
  admin_promotion_created: '管理员创建促销', admin_promotion_updated: '管理员更新促销',
  admin_promotion_enabled: '管理员启用促销', admin_promotion_disabled: '管理员停用促销',
};

const REASON_TEXT: Record<string, string> = {
  not_found: '对象不存在', product_not_found: '商品不存在', invalid_status: '状态不允许',
  status_changed: '状态已变化', not_paid_status: '订单未支付', invalid_price: '价格不合法',
  invalid_discount: '折扣价不合法', invalid_window: '时间窗口不合法', window_conflict: '活动时间冲突',
  active_supplier_not_found: '启用供应商不存在', has_active_products: '仍有关联启用商品',
  insufficient_or_missing_bucket: '库存不足或分桶不存在', invalid_user_id: '用户ID不合法',
  invalid_status_value: '状态值不合法', self_disable_blocked: '禁止禁用当前管理员',
  store_failed: '存储更新失败',
};

export function eventText(type: string): string {
  return SECURITY_EVENT_TEXT[type] || type || '-';
}

export function resultMatches(actual: string, expected: string): boolean {
  const normalizedActual = (actual || '').toLowerCase();
  const normalizedExpected = (expected || '').toLowerCase();
  return normalizedActual === normalizedExpected
    || (['fail', 'failed'].includes(normalizedActual) && ['fail', 'failed'].includes(normalizedExpected));
}

export function reasonText(subject: string): string {
  const reason = (subject || '').match(/\breason:([A-Za-z0-9_-]+)/)?.[1] || '';
  return REASON_TEXT[reason] || reason || '-';
}

export function formatEventTime(seconds: number): string {
  return seconds ? new Date(seconds * 1000).toLocaleString() : '-';
}

export function subjectNavigation(subject: string): SubjectNavigation | null {
  const patterns: Array<[RegExp, (value: string) => SubjectNavigation]> = [
    [/\border:([A-Za-z0-9_-]+)/, (value) => ({ path: '/admin/orders', orderId: value })],
    [/\bpromotion:(\d+)/, (value) => ({ path: '/admin/promotions', promotionId: Number(value) })],
    [/\bsupplier:(\d+)/, (value) => ({ path: '/admin/suppliers', supplierId: Number(value) })],
    [/\bproduct:(\d+)/, (value) => ({ path: '/admin/products', productId: Number(value) })],
    [/\boperator:(\d+)/, (value) => ({ path: '/admin/users', userId: Number(value) })],
    [/\btarget_user:(\d+)/, (value) => ({ path: '/admin/users', userId: Number(value) })],
  ];
  for (const [pattern, createNavigation] of patterns) {
    const value = (subject || '').match(pattern)?.[1];
    if (value) return createNavigation(value);
  }
  return null;
}

export function eventOptions(items: SecurityEventItem[]) {
  return Array.from(new Set([...Object.keys(SECURITY_EVENT_TEXT), ...items.map((item) => item.event_type).filter(Boolean)]))
    .map((type) => ({ value: type, label: eventText(type) }));
}

export function filterSecurityEvents(
  items: SecurityEventItem[], userId: string, result?: string, eventType?: string, keyword = '',
): SecurityEventItem[] {
  const userIdValue = Number(userId || 0);
  const keywordValue = keyword.trim().toLowerCase();
  return items.filter((item) => {
    if (userIdValue && item.user_id !== userIdValue) return false;
    if (result && !resultMatches(item.result, result)) return false;
    if (eventType && item.event_type !== eventType) return false;
    if (!keywordValue) return true;
    const source = `${item.user_id || ''} ${item.subject || ''} ${item.ip || ''} ${item.user_agent || ''} ${item.event_type || ''} ${item.result || ''}`.toLowerCase();
    return source.includes(keywordValue);
  });
}
