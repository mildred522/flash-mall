import { state, $, log, rid } from "./state.js";
import { authed, openAuth } from "./auth.js";
import { cartItems, cartTotalFen, clearCart, closeCart } from "./cart.js";
import { bestCoupon, renderCheckoutCoupons } from "./coupon.js";
import { goToOrders, latest, pollOrderStatus } from "./order.js";

function money(fen) {
  return (Number(fen || 0) / 100).toFixed(2);
}

function checkoutError(message) {
  const box = $("checkout-error");
  if (box) box.textContent = message || "";
}

export async function openCheckout(items = cartItems()) {
  if (!state.token) {
    openAuth("password");
    log("结算被拦截：请先登录");
    return;
  }
  if (!items.length) {
    log("结算失败：购物车为空");
    return;
  }
  state.checkoutItems = items.map((item) => ({ ...item }));
  $("checkout-modal")?.classList.remove("hidden");
  $("checkout-modal")?.setAttribute("aria-hidden", "false");
  checkoutError("");
  renderCheckoutItems();
  await loadAddresses();
}

export function closeCheckout() {
  $("checkout-modal")?.classList.add("hidden");
  $("checkout-modal")?.setAttribute("aria-hidden", "true");
}

function renderCheckoutItems() {
  const items = state.checkoutItems || [];
  const list = $("checkout-items");
  if (!list) return;
  list.innerHTML = items.map((item) => `
    <article class="checkout-item">
      <div>
        <strong>${item.name || "商品"}</strong>
        <div class="cart-item-meta">数量 ${item.qty} · 单价 ¥${money(item.final_price_fen)}</div>
      </div>
      <strong>¥${money(Number(item.final_price_fen || 0) * Number(item.qty || 0))}</strong>
    </article>
  `).join("");
  const total = items.reduce((sum, item) => sum + Number(item.final_price_fen || 0) * Number(item.qty || 0), 0);
  renderCheckoutCoupons(total);
  const totalEl = $("checkout-total");
  const coupon = bestCoupon(total);
  if (totalEl) totalEl.textContent = `¥${money(Math.max(0, (total || cartTotalFen()) - Number(coupon?.discount_fen || 0)))}`;
}

async function loadAddresses() {
  const resp = await authed("/api/user/addresses");
  if (!resp.ok) {
    checkoutError(`地址加载失败 status=${resp.status}`);
    return;
  }
  state.addresses = resp.data.items || [];
  const current = state.addresses.find((item) => item.is_default) || state.addresses[0];
  state.selectedAddressId = current ? current.address_id : 0;
  renderAddresses();
}

function renderAddresses() {
  const list = $("address-list");
  if (!list) return;
  const items = state.addresses || [];
  if (!items.length) {
    list.innerHTML = '<div class="orders-empty">暂无收货地址，请新增一个地址</div>';
    return;
  }
  list.innerHTML = items.map((item) => `
    <article class="address-card ${Number(item.address_id) === Number(state.selectedAddressId) ? "selected" : ""}" data-address-id="${item.address_id}">
      <strong>${item.receiver_name} · ${item.receiver_phone}</strong>
      <div class="cart-item-meta">${[item.province, item.city, item.district, item.detail].filter(Boolean).join(" ")}</div>
    </article>
  `).join("");
  list.querySelectorAll("[data-address-id]").forEach((card) => {
    card.addEventListener("click", () => {
      state.selectedAddressId = Number(card.getAttribute("data-address-id") || 0);
      renderAddresses();
    });
  });
}

export function toggleAddressForm(show) {
  $("address-form")?.classList.toggle("hidden", !show);
}

export async function saveAddressFromForm() {
  const payload = {
    receiver_name: $("addr-name")?.value.trim() || "",
    receiver_phone: $("addr-phone")?.value.trim() || "",
    province: $("addr-province")?.value.trim() || "",
    city: $("addr-city")?.value.trim() || "",
    district: $("addr-district")?.value.trim() || "",
    detail: $("addr-detail")?.value.trim() || "",
    is_default: true,
  };
  if (!payload.receiver_name || !payload.receiver_phone || !payload.detail) {
    checkoutError("请填写收货人、手机号和详细地址");
    return;
  }
  const resp = await authed("/api/user/addresses/upsert", { method: "POST", jsonBody: payload });
  if (!resp.ok) {
    checkoutError(`地址保存失败 status=${resp.status}`);
    return;
  }
  ["addr-name", "addr-phone", "addr-province", "addr-city", "addr-district", "addr-detail"].forEach((id) => {
    const el = $(id);
    if (el) el.value = "";
  });
  toggleAddressForm(false);
  log(`地址已保存 address=${resp.data.address_id}`);
  await loadAddresses();
}

export async function submitCheckout() {
  const items = state.checkoutItems || [];
  if (!items.length) {
    checkoutError("没有可结算商品");
    return;
  }
  if (!state.selectedAddressId) {
    checkoutError("请选择或新增收货地址");
    return;
  }
  const submit = $("submit-checkout");
  if (submit) {
    submit.disabled = true;
    submit.textContent = "提交中...";
  }
  const created = [];
  for (const item of items) {
    const requestId = rid();
    const payload = {
      request_id: requestId,
      user_id: 999999,
      product_id: Number(item.product_id),
      amount: Number(item.qty || 1),
      expected_price_fen: Number(item.final_price_fen || 0) * Number(item.qty || 1),
    };
    const resp = await authed("/api/order/create", { method: "POST", jsonBody: payload });
    if (!resp.ok) {
      checkoutError(`订单提交失败：${item.name || item.product_id} status=${resp.status}`);
      if (submit) {
        submit.disabled = false;
        submit.textContent = "提交订单";
      }
      return;
    }
    created.push({ orderId: resp.data.order_id, requestId });
    latest(resp.data.order_id, requestId);
  }
  clearCart();
  closeCart();
  closeCheckout();
  log(`结算完成 orders=${created.length} address=${state.selectedAddressId}`);
  await goToOrders();
  if (created[0]) pollOrderStatus(created[0].requestId, created[0].orderId);
  if (submit) {
    submit.disabled = false;
    submit.textContent = "提交订单";
  }
}
