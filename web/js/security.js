import { state, $ } from "./state.js";
import { authed } from "./auth.js";

export function summarizeSecurity(items, matcher) {
  const hit = items.find((item) => matcher(item));
  if (!hit) return "normal";
  return `${securityEventText(hit.event_type)}:${securityResultText(hit.result)}`;
}

function securityEventText(type) {
  const map = {
    login_password_success: "密码登录成功",
    login_password_fail: "密码登录失败",
    login_code_success: "验证码登录成功",
    login_code_fail: "验证码登录失败",
    send_code_success: "发送验证码",
    send_code_blocked: "验证码限流",
    refresh_success: "刷新登录",
    logout_success: "退出登录",
    logout_all_success: "退出全部设备",
    reset_password_success: "重置密码",
    reset_password_fail: "重置密码失败",
    register_success: "注册成功",
    register_fail: "注册失败",
    admin_user_enabled: "管理员启用用户",
    admin_user_disabled: "管理员禁用用户",
    admin_user_status_update_failed: "管理员更新用户状态失败",
    admin_order_shipped: "管理员发货",
    admin_order_refunded: "管理员退款",
    admin_product_created: "管理员创建商品",
    admin_product_updated: "管理员更新商品",
    admin_product_enabled: "管理员上架商品",
    admin_product_disabled: "管理员下架商品",
    admin_product_stock_adjusted: "管理员调整库存",
    admin_supplier_created: "管理员创建供应商",
    admin_supplier_updated: "管理员更新供应商",
    admin_supplier_enabled: "管理员启用供应商",
    admin_supplier_disabled: "管理员停用供应商",
    admin_promotion_created: "管理员创建促销",
    admin_promotion_updated: "管理员更新促销",
    admin_promotion_enabled: "管理员启用促销",
    admin_promotion_disabled: "管理员停用促销",
  };
  return map[type] || type || "-";
}

function securityResultText(result) {
  if (result === "success") return "成功";
  if (result === "blocked") return "拦截";
  if (result === "fail" || result === "failed") return "失败";
  return result || "-";
}

export function renderSecurityEvents(items) {
  const lines = (items || []).map((item) => {
    const when = item.created_at ? new Date(item.created_at * 1000).toLocaleTimeString() : "--:--:--";
    const who = item.subject || `user:${item.user_id || 0}`;
    const ip = item.ip || "-";
    return `[${when}] ${securityEventText(item.event_type)} ${securityResultText(item.result)} ${who} ${ip}`;
  });
  $("security-events").textContent = lines.length ? lines.join("\n") : "No recent security events yet.";
  $("login-risk-summary").textContent = summarizeSecurity(items, (item) =>
    String(item.event_type || "").includes("login")
  );
  $("code-risk-summary").textContent = summarizeSecurity(items, (item) => {
    const eventType = String(item.event_type || "");
    return eventType.includes("code") || eventType.includes("password");
  });
}

export async function loadSecurityEvents() {
  if (!state.token) {
    renderSecurityEvents([]);
    return false;
  }
  const response = await authed("/api/auth/security/events/recent");
  if (!response.ok) {
    renderSecurityEvents([]);
    return false;
  }
  renderSecurityEvents(response.data.items || []);
  return true;
}
