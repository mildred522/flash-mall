import { state, $, log } from "./state.js";

const COUPON_KEY = "fm_coupons";

export const COUPONS = {
  FULL_199_30: { id: "FULL_199_30", name: "满199减30", threshold_fen: 19900, discount_fen: 3000, scope: "全场精选" },
  NEW_USER_10: { id: "NEW_USER_10", name: "新人立减10", threshold_fen: 0, discount_fen: 1000, scope: "新人专享" },
};

function money(fen) {
  return (Number(fen || 0) / 100).toFixed(2);
}

function loadStoredCoupons() {
  try {
    const parsed = JSON.parse(localStorage.getItem(COUPON_KEY) || "[]");
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function saveCoupons() {
  localStorage.setItem(COUPON_KEY, JSON.stringify(state.coupons || []));
}

export function initCoupons() {
  state.coupons = loadStoredCoupons();
}

export function claimCoupon(id) {
  const coupon = COUPONS[id];
  if (!coupon) return;
  state.coupons = state.coupons || [];
  if (state.coupons.some((item) => item.id === id)) {
    log(`优惠券已领取 ${coupon.name}`);
    return;
  }
  state.coupons.push({ ...coupon, claimed_at: Date.now(), used: false });
  saveCoupons();
  log(`领取优惠券成功 ${coupon.name}`);
}

export function availableCoupons(totalFen) {
  return (state.coupons || [])
    .filter((coupon) => !coupon.used)
    .map((coupon) => ({
      ...coupon,
      available: Number(totalFen || 0) >= Number(coupon.threshold_fen || 0),
    }));
}

export function bestCoupon(totalFen) {
  return availableCoupons(totalFen)
    .filter((coupon) => coupon.available)
    .sort((a, b) => Number(b.discount_fen || 0) - Number(a.discount_fen || 0))[0] || null;
}

export function renderCheckoutCoupons(totalFen) {
  const list = $("checkout-coupons");
  const saving = $("coupon-saving");
  if (!list) return;
  const coupons = availableCoupons(totalFen);
  const selected = bestCoupon(totalFen);
  state.selectedCouponId = selected ? selected.id : "";
  if (saving) {
    saving.textContent = selected ? `预计优惠 ¥${money(selected.discount_fen)}，支付时仍按后端价格保护` : "暂无可用优惠";
  }
  if (!coupons.length) {
    list.innerHTML = '<div class="orders-empty">暂无优惠券，可在今日会场领取</div>';
    return;
  }
  list.innerHTML = coupons.map((coupon) => `
    <article class="coupon-card ${coupon.available ? "available" : ""}">
      <div>
        <strong>${coupon.name}</strong>
        <div class="cart-item-meta">${coupon.scope} · ${coupon.threshold_fen > 0 ? `满 ¥${money(coupon.threshold_fen)} 可用` : "无门槛"}</div>
      </div>
      <span>${coupon.available ? `-¥${money(coupon.discount_fen)}` : "未满足"}</span>
    </article>
  `).join("");
}

export function showCouponWallet() {
  const coupons = state.coupons || [];
  if (!coupons.length) {
    log("暂无优惠券，请先在今日会场领取");
    return;
  }
  log(`我的优惠券: ${coupons.map((coupon) => coupon.name).join(", ")}`);
}
