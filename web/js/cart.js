import { state, $, log } from "./state.js";

const CART_KEY_PREFIX = "fm_cart";

function cartOwnerKey() {
  if (state.user && state.user.user_id) return `${CART_KEY_PREFIX}:user:${state.user.user_id}`;
  return `${CART_KEY_PREFIX}:guest`;
}

function money(fen) {
  return (Number(fen || 0) / 100).toFixed(2);
}

function loadStoredCart() {
  try {
    const parsed = JSON.parse(localStorage.getItem(cartOwnerKey()) || "[]");
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function saveCart() {
  localStorage.setItem(cartOwnerKey(), JSON.stringify(state.cart || []));
}

export function initCartState() {
  state.cart = loadStoredCart();
  renderCart();
}

export function switchCartOwner() {
  state.cart = loadStoredCart();
  renderCart();
}

export function cartItems() {
  return state.cart || [];
}

export function cartTotalFen() {
  return cartItems().reduce((sum, item) => sum + Number(item.final_price_fen || 0) * Number(item.qty || 0), 0);
}

export function addToCart(product, qty = 1) {
  if (!product || !product.product_id) return;
  state.cart = state.cart || [];
  const existing = state.cart.find((item) => Number(item.product_id) === Number(product.product_id));
  if (existing) {
    existing.qty = Math.min(Number(product.stock_available || 1), Number(existing.qty || 0) + qty);
    existing.final_price_fen = product.final_price_fen;
    existing.origin_price_fen = product.origin_price_fen;
    existing.stock_available = product.stock_available;
  } else {
    state.cart.push({
      product_id: product.product_id,
      name: product.name,
      final_price_fen: product.final_price_fen,
      origin_price_fen: product.origin_price_fen,
      stock_available: product.stock_available,
      promotion_tag: product.promotion_tag || "",
      qty,
    });
  }
  saveCart();
  renderCart();
  log(`已加入购物车 product=${product.product_id} qty=${qty}`);
}

export function setCartQty(productId, qty) {
  state.cart = (state.cart || [])
    .map((item) => Number(item.product_id) === Number(productId)
      ? { ...item, qty: Math.max(1, Math.min(Number(item.stock_available || 1), Number(qty || 1))) }
      : item)
    .filter((item) => Number(item.qty || 0) > 0);
  saveCart();
  renderCart();
}

export function removeCartItem(productId) {
  state.cart = (state.cart || []).filter((item) => Number(item.product_id) !== Number(productId));
  saveCart();
  renderCart();
}

export function clearCart() {
  state.cart = [];
  saveCart();
  renderCart();
}

export function openCart() {
  $("cart-drawer")?.classList.add("open");
  $("cart-backdrop")?.classList.remove("hidden");
}

export function closeCart() {
  $("cart-drawer")?.classList.remove("open");
  $("cart-backdrop")?.classList.add("hidden");
}

export function renderCart() {
  const items = cartItems();
  const count = items.reduce((sum, item) => sum + Number(item.qty || 0), 0);
  const countEl = $("cart-count");
  if (countEl) countEl.textContent = String(count);
  const summary = $("cart-summary");
  if (summary) summary.textContent = `共 ${count} 件商品`;
  const total = $("cart-total");
  if (total) total.textContent = `¥${money(cartTotalFen())}`;
  const list = $("cart-list");
  if (!list) return;
  if (!items.length) {
    list.innerHTML = '<div class="orders-empty">购物车为空，先挑选商品</div>';
    return;
  }
  list.innerHTML = items.map((item) => `
    <article class="cart-item">
      <div class="cart-item-main">
        <div>
          <div class="cart-item-name">${item.name || "商品"}</div>
          <div class="cart-item-meta">单价 ¥${money(item.final_price_fen)} ${item.promotion_tag ? `· ${item.promotion_tag}` : ""}</div>
        </div>
        <button class="link-action" data-cart-remove="${item.product_id}" type="button">移除</button>
      </div>
      <div class="cart-qty">
        <button data-cart-dec="${item.product_id}" type="button">-</button>
        <strong>${item.qty}</strong>
        <button data-cart-inc="${item.product_id}" type="button">+</button>
        <span class="cart-item-meta">小计 ¥${money(Number(item.final_price_fen || 0) * Number(item.qty || 0))}</span>
      </div>
    </article>
  `).join("");
  list.querySelectorAll("[data-cart-dec]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const item = items.find((entry) => String(entry.product_id) === btn.getAttribute("data-cart-dec"));
      if (item) setCartQty(item.product_id, Number(item.qty || 1) - 1);
    });
  });
  list.querySelectorAll("[data-cart-inc]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const item = items.find((entry) => String(entry.product_id) === btn.getAttribute("data-cart-inc"));
      if (item) setCartQty(item.product_id, Number(item.qty || 1) + 1);
    });
  });
  list.querySelectorAll("[data-cart-remove]").forEach((btn) => {
    btn.addEventListener("click", () => removeCartItem(btn.getAttribute("data-cart-remove")));
  });
}
