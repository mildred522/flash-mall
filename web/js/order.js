import { state, $, log, rid } from "./state.js";
import { authed, openAuth } from "./auth.js";
import { PRODUCT_META, formatPriceFen } from "./catalog.js";

export { rid };

export function latest(orderId, requestId) {
  if (orderId) $("latest-order").textContent = orderId;
  if (requestId) $("latest-request").textContent = requestId;
}

// --- Payment state polling ---
let pollingTimer = null;

function clearPolling() {
  if (pollingTimer) {
    clearInterval(pollingTimer);
    pollingTimer = null;
  }
}

function showPaymentBanner(orderId, status) {
  let banner = $("payment-state-banner");
  if (!banner) {
    banner = document.createElement("div");
    banner.id = "payment-state-banner";
    $("orders-section")?.insertBefore(banner, $("orders-list"));
  }
  const STATUS_MAP = {
    0: { text: "待支付", cls: "pending", icon: "⏳" },
    1: { text: "已支付", cls: "paid", icon: "✅" },
    2: { text: "已关闭", cls: "closed", icon: "🔒" },
  };
  const st = STATUS_MAP[status] || { text: "处理中", cls: "pending", icon: "⏳" };
  banner.className = `payment-banner ${st.cls}`;
  banner.innerHTML = `
    <span class="payment-banner-icon">${st.icon}</span>
    <span class="payment-banner-text">订单 ${orderId} · ${st.text}</span>
    ${status === 0 ? `<button class="btn-pay" data-pay-order="${orderId}" style="margin-left:12px">去付款</button>` : ""}
  `;
  // Wire pay button inside banner
  banner.querySelectorAll("[data-pay-order]").forEach((btn) =>
    btn.addEventListener("click", () => openPayModal(orderId))
  );
}

function hidePaymentBanner() {
  const banner = $("payment-state-banner");
  if (banner) banner.remove();
}

/**
 * Poll order status via /api/order/status?request_id=...
 * Stops when status is no longer 0 (pending).
 */
export async function pollOrderStatus(requestId, orderId) {
  clearPolling();
  if (orderId) showPaymentBanner(orderId, 0);
  let attempts = 0;
  const maxAttempts = 30; // 30s max
  pollingTimer = setInterval(async () => {
    attempts++;
    if (attempts > maxAttempts) {
      clearPolling();
      log(`轮询超时 order=${orderId}`);
      return;
    }
    try {
      const params = orderId ? `order_id=${orderId}` : `request_id=${requestId}`;
      const resp = await fetch(`/api/order/status?${params}`);
      if (!resp.ok) return;
      const data = await resp.json();
      const status = typeof data.status === "string" ? parseInt(data.status, 10) : data.status;
      if (orderId) showPaymentBanner(orderId, status);
      if (status !== 0) {
        clearPolling();
        const statusText = { 1: "已支付", 2: "已关闭", 3: "已发货", 4: "已收货", 5: "退款中", 6: "已退款" };
        log(`订单状态变更 order=${orderId || data.order_id} status=${statusText[status] || status}`);
        await showOrders();
        if (status === 1) {
          setTimeout(hidePaymentBanner, 3000);
        }
      }
    } catch (_) {
      // ignore transient errors
    }
  }, 1000);
}

// Navigate to orders page and show banner
export async function goToOrders() {
  if ($("featured-products")) $("featured-products").style.display = "none";
  if ($("campaign-strip")) $("campaign-strip").style.display = "none";
  const heroParent = document.querySelector(".hero")?.parentElement;
  if (heroParent) heroParent.style.display = "none";
  $("orders-section").style.display = "";
  await showOrders();
}

export async function order(productId, amount, scene) {
  if (!state.token) {
    openAuth("password");
    log("下单被拦截：请先登录");
    return null;
  }
  const requestId = rid();
  const payload = { request_id: requestId, user_id: 999999, product_id: productId, amount };
  const t0 = performance.now();
  const response = await authed("/api/order/create", { method: "POST", jsonBody: payload });
  const cost = (performance.now() - t0).toFixed(1);
  latest(response.ok ? response.data.order_id : "失败", requestId);
  if (response.ok) {
    const orderId = response.data.order_id;
    log(`下单成功 scene=${scene} product=${productId} qty=${amount} req=${requestId} order=${orderId} latency=${cost}ms`);
    // Auto-navigate to orders and start polling
    await goToOrders();
    pollOrderStatus(requestId, orderId);
  } else {
    if (response.status === 401) openAuth("password");
    log(`下单失败 scene=${scene} status=${response.status} req=${requestId} body=${JSON.stringify(response.data)} latency=${cost}ms`);
  }
  return response;
}

export async function burst() {
  if (!state.token) {
    openAuth("password");
    return;
  }
  log("连续下单开始 rounds=3 qty=1");
  for (let i = 0; i < 3; i += 1) {
    await order(100, 1, `burst-${i + 1}/3`);
    if (i < 2) await new Promise((resolve) => setTimeout(resolve, 140));
  }
  log("连续下单结束 rounds=3");
}

const STATUS_MAP = {
  0: { text: "待支付", cls: "pending" },
  1: { text: "已支付", cls: "paid" },
  2: { text: "已关闭", cls: "closed" },
  3: { text: "已发货", cls: "shipped" },
  4: { text: "已收货", cls: "completed" },
  5: { text: "退款中", cls: "refund" },
  6: { text: "已退款", cls: "refunded" },
};

export async function loadOrders() {
  const response = await authed("/api/orders");
  if (!response.ok) {
    log(`订单列表拉取失败 status=${response.status}`);
    return [];
  }
  const items = response.data.items || [];
  log(`订单列表已刷新 items=${items.length}`);
  return items;
}

export async function payOrder(orderId) {
  const t0 = performance.now();
  const response = await authed("/api/order/pay", { method: "POST", jsonBody: { order_id: orderId } });
  const cost = (performance.now() - t0).toFixed(1);
  if (response.ok) {
    log(`支付成功 order=${orderId} latency=${cost}ms`);
  } else {
    log(`支付失败 order=${orderId} status=${response.status} body=${JSON.stringify(response.data)} latency=${cost}ms`);
  }
  return response;
}

export async function cancelOrder(orderId) {
  const t0 = performance.now();
  const response = await authed("/api/order/cancel", { method: "POST", jsonBody: { order_id: orderId, reason: "user cancel" } });
  const cost = (performance.now() - t0).toFixed(1);
  if (response.ok) {
    log(`取消订单成功 order=${orderId} latency=${cost}ms`);
  } else {
    log(`取消订单失败 order=${orderId} status=${response.status} body=${JSON.stringify(response.data)} latency=${cost}ms`);
  }
  return response;
}

export async function confirmReceipt(orderId) {
  const t0 = performance.now();
  const response = await authed("/api/order/confirm-receipt", { method: "POST", jsonBody: { order_id: orderId } });
  const cost = (performance.now() - t0).toFixed(1);
  if (response.ok) {
    log(`确认收货 order=${orderId} latency=${cost}ms`);
  } else {
    log(`确认收货失败 order=${orderId} status=${response.status} body=${JSON.stringify(response.data)} latency=${cost}ms`);
  }
  return response;
}

export async function refundOrder(orderId) {
  const reason = $("refund-reason")?.value.trim() || "";
  if (!reason) {
    const box = $("refund-error");
    if (box) box.textContent = "请输入退款原因";
    return { ok: false, status: 0, data: { error: "reason required" } };
  }
  const t0 = performance.now();
  const response = await authed("/api/order/refund", { method: "POST", jsonBody: { order_id: orderId, reason } });
  const cost = (performance.now() - t0).toFixed(1);
  if (response.ok) {
    log(`退款成功 order=${orderId} latency=${cost}ms`);
  } else {
    log(`退款失败 order=${orderId} status=${response.status} body=${JSON.stringify(response.data)} latency=${cost}ms`);
  }
  return response;
}

export function openPayModal(orderId) {
  state.payOrderId = orderId;
  const label = $("pay-order-label");
  if (label) label.textContent = `订单 ${orderId}`;
  const error = $("pay-error");
  if (error) error.textContent = "";
  $("pay-modal")?.classList.remove("hidden");
  $("pay-modal")?.setAttribute("aria-hidden", "false");
}

export function closePayModal() {
  $("pay-modal")?.classList.add("hidden");
  $("pay-modal")?.setAttribute("aria-hidden", "true");
}

export async function submitPayModal() {
  if (!state.payOrderId) return;
  const btn = $("submit-pay");
  if (btn) {
    btn.disabled = true;
    btn.textContent = "支付中...";
  }
  const resp = await payOrder(state.payOrderId);
  if (resp.ok) {
    closePayModal();
    pollOrderStatus(null, state.payOrderId);
    await showOrders();
  } else {
    const error = $("pay-error");
    if (error) error.textContent = `支付失败 status=${resp.status}`;
  }
  if (btn) {
    btn.disabled = false;
    btn.textContent = "确认支付";
  }
}

export function openRefundModal(orderId) {
  state.refundOrderId = orderId;
  const label = $("refund-order-label");
  if (label) label.textContent = `订单 ${orderId}`;
  const reason = $("refund-reason");
  if (reason) reason.value = "user refund";
  const error = $("refund-error");
  if (error) error.textContent = "";
  $("refund-modal")?.classList.remove("hidden");
  $("refund-modal")?.setAttribute("aria-hidden", "false");
}

export function closeRefundModal() {
  $("refund-modal")?.classList.add("hidden");
  $("refund-modal")?.setAttribute("aria-hidden", "true");
}

export async function submitRefundModal() {
  if (!state.refundOrderId) return;
  const btn = $("submit-refund");
  if (btn) {
    btn.disabled = true;
    btn.textContent = "提交中...";
  }
  const resp = await refundOrder(state.refundOrderId);
  if (resp.ok) {
    closeRefundModal();
    await showOrders();
  }
  if (btn) {
    btn.disabled = false;
    btn.textContent = "提交退款";
  }
}

export async function loadOrderDetail(orderId) {
  const response = await authed(`/api/orders/detail?order_id=${encodeURIComponent(orderId)}`);
  if (!response.ok) {
    log(`订单详情拉取失败 order=${orderId} status=${response.status}`);
    return null;
  }
  return response.data;
}

export function renderOrders(items) {
  const container = $("orders-list");
  if (!container) return;

  if (!items.length) {
    container.innerHTML = `<div class="orders-empty">暂无订单，去逛逛吧</div>`;
    return;
  }

  container.innerHTML = items.map((item) => {
    const st = STATUS_MAP[item.status] || { text: "未知", cls: "unknown" };
    const meta = PRODUCT_META.get(item.product_id);
    const icon = meta ? meta.icon : "📦";
    return `
      <div class="order-card">
        <div class="order-card-thumb">
          <span style="font-size:32px">${icon}</span>
        </div>
        <div class="order-card-info">
          <div class="order-card-name">${item.product_name || "商品"}</div>
          <div class="order-card-meta">
            <span>数量: ${item.amount}</span>
            <span>订单号: ${item.order_id}</span>
          </div>
          <div class="order-card-time">${item.create_time || ""}</div>
        </div>
        <div class="order-card-right">
          <div class="order-card-price">¥${formatPriceFen(item.payable_amount_fen)}</div>
          <span class="order-status-badge ${st.cls}">${st.text}</span>
          <button class="btn-pay" data-detail-order="${item.order_id}">详情</button>
          ${item.status === 0 ? `<button class="btn-pay" data-pay-order="${item.order_id}">去付款</button>` : ""}
          ${item.status === 0 ? `<button class="btn-refund" data-cancel-order="${item.order_id}">取消订单</button>` : ""}
          ${item.status === 3 ? `<button class="btn-confirm" data-confirm-order="${item.order_id}">确认收货</button>` : ""}
          ${(item.status === 1 || item.status === 3) ? `<button class="btn-refund" data-refund-order="${item.order_id}">申请退款</button>` : ""}
        </div>
      </div>`;
  }).join("");

  container.querySelectorAll("[data-detail-order]").forEach((btn) =>
    btn.addEventListener("click", async () => {
      const orderId = btn.getAttribute("data-detail-order");
      const detail = await loadOrderDetail(orderId);
      if (!detail) return;
      const card = btn.closest(".order-card");
      let panel = card.querySelector(".order-detail-inline");
      if (panel) {
        panel.remove();
        return;
      }
      panel = document.createElement("div");
      panel.className = "order-detail-inline";
      panel.innerHTML = `
        <div><span>订单号</span><strong>${detail.order_id}</strong></div>
        <div><span>支付单</span><strong>${detail.payment_order_id || "—"}</strong></div>
        <div><span>支付状态</span><strong>${detail.payment_status_text || "—"}</strong></div>
        <div><span>商品单价</span><strong>¥${formatPriceFen(detail.sale_unit_price_fen)}</strong></div>
        <div><span>优惠</span><strong>${detail.promotion_tag || "无"} ¥${formatPriceFen(detail.discount_amount_fen)}</strong></div>
        <div><span>应付</span><strong>¥${formatPriceFen(detail.payable_amount_fen)}</strong></div>
      `;
      card.appendChild(panel);
    })
  );

  container.querySelectorAll("[data-pay-order]").forEach((btn) =>
    btn.addEventListener("click", () => openPayModal(btn.getAttribute("data-pay-order")))
  );

  container.querySelectorAll("[data-cancel-order]").forEach((btn) =>
    btn.addEventListener("click", async () => {
      const orderId = btn.getAttribute("data-cancel-order");
      btn.disabled = true;
      btn.textContent = "取消中...";
      const resp = await cancelOrder(orderId);
      if (resp.ok) {
        await showOrders();
      } else {
        btn.disabled = false;
        btn.textContent = "取消订单";
      }
    })
  );

  container.querySelectorAll("[data-confirm-order]").forEach((btn) =>
    btn.addEventListener("click", async () => {
      const orderId = btn.getAttribute("data-confirm-order");
      btn.disabled = true;
      btn.textContent = "确认中...";
      const resp = await confirmReceipt(orderId);
      if (resp.ok) {
        await showOrders();
      } else {
        btn.disabled = false;
        btn.textContent = "确认收货";
      }
    })
  );

  container.querySelectorAll("[data-refund-order]").forEach((btn) =>
    btn.addEventListener("click", () => openRefundModal(btn.getAttribute("data-refund-order")))
  );
}

export async function showOrders() {
  const items = await loadOrders();
  renderOrders(items);
}
