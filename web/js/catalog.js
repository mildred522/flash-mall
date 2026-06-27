import { state, $, log } from "./state.js";
import { api } from "./api-client.js";
import { addToCart } from "./cart.js";
import { openCheckout } from "./checkout.js";

export const PRODUCT_META = new Map([
  [100, {
    image: "https://images.unsplash.com/photo-1591047139829-d91aecb6caea?w=400&h=400&fit=crop&auto=format",
    icon: "🧥",
    desc: "利落版型配轻盈面料，通勤出门都好搭",
    tags: ["限时秒杀", "包邮"],
    accent: "linear-gradient(135deg, #fff1f4, #ffe8e0)",
  }],
  [101, {
    image: "https://images.unsplash.com/photo-1551028719-00167b16eac5?w=400&h=400&fit=crop&auto=format",
    icon: "🧥",
    desc: "90%白鸭绒填充，轻暖不臃肿",
    tags: ["限时秒杀", "保暖"],
    accent: "linear-gradient(135deg, #e8f0ff, #d8e8ff)",
  }],
  [102, {
    image: "https://images.unsplash.com/photo-1521572163474-6864f9cf17ab?w=400&h=400&fit=crop&auto=format",
    icon: "👕",
    desc: "100%纯棉面料，三色随心搭配",
    tags: ["热卖", "满减"],
    accent: "linear-gradient(135deg, #fff8f0, #fff0e0)",
  }],
  [103, {
    image: "https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=400&h=400&fit=crop&auto=format",
    icon: "👟",
    desc: "飞织透气鞋面，缓震回弹鞋底",
    tags: ["人气款", "包邮"],
    accent: "linear-gradient(135deg, #f0fff4, #e0ffe8)",
  }],
  [104, {
    image: "https://images.unsplash.com/photo-1609091839311-d5365f9ff1c5?w=400&h=400&fit=crop&auto=format",
    icon: "🔋",
    desc: "20000mAh大容量，22.5W快充",
    tags: ["新品首发", "包邮"],
    accent: "linear-gradient(135deg, #fef4ff, #f0e8ff)",
  }],
]);

export function formatPriceFen(value) {
  return (Number(value || 0) / 100).toFixed(2);
}

export function renderProducts(items) {
  state.products = Array.isArray(items) ? items : [];
  const grid = $("product-grid");

  if (!state.products.length) {
    grid.innerHTML = `
      <div class="product" style="grid-column:1/-1;text-align:center;padding:40px">
        <p style="color:var(--muted)">正在加载商品...</p>
      </div>`;
    return;
  }

  grid.innerHTML = state.products
    .map((product, index) => {
      const meta = PRODUCT_META.get(product.product_id) || PRODUCT_META.values().next().value;
      const hasDiscount = product.origin_price_fen > product.final_price_fen;
      const discountPercent = hasDiscount
        ? Math.round((1 - product.final_price_fen / product.origin_price_fen) * 100)
        : 0;
      const badge = product.promotion_tag || (hasDiscount ? `${discountPercent}% OFF` : meta.tags[0]);
      const inStock = product.stock_available > 0;
      const imageUrl = product.image_url || meta.image;

      return `
      <article class="product">
        <div class="thumb" style="background:${meta.accent}">
          <div class="badge">
            <span class="pill">${badge}</span>
            ${meta.tags.length > 1 ? `<span class="pill orange">${meta.tags[1]}</span>` : ""}
          </div>
          <img class="thumb-img" src="${imageUrl}" alt="${product.name || '商品图片'}" loading="lazy" onerror="this.style.display='none';this.nextElementSibling.style.display='block'" />
          <span class="thumb-icon" style="display:none">${meta.icon}</span>
        </div>
        <div class="product-info">
          <div class="product-name">${product.name || "精选好物"}</div>
          <div class="product-tags">
            ${meta.tags.map((t) => `<span class="tag">${t}</span>`).join("")}
          </div>
          <div class="price-row">
            <span class="price-sale"><span class="symbol">¥</span>${formatPriceFen(product.final_price_fen)}</span>
            ${hasDiscount ? `<span class="price-origin">¥${formatPriceFen(product.origin_price_fen)}</span>` : ""}
          </div>
          <div class="product-meta">
            <span>${inStock ? `库存 ${product.stock_available}` : "暂时缺货"}</span>
          </div>
          <div class="product-actions">
            <button class="btn-cart" type="button" data-cart-index="${index}" ${inStock ? "" : "disabled"}>加入购物车</button>
            <button class="btn-buy" type="button" data-buy-index="${index}" ${inStock ? "" : "disabled"}>${inStock ? "立即购买" : "已售罄"}</button>
          </div>
        </div>
      </article>`;
    })
    .join("");

  document.querySelectorAll("[data-buy-index]").forEach((button) =>
    button.addEventListener("click", async (e) => {
      e.stopPropagation();
      const index = Number(button.getAttribute("data-buy-index"));
      const product = state.products[index];
      await openCheckout([{ ...product, qty: 1 }]);
    })
  );
  document.querySelectorAll("[data-cart-index]").forEach((button) =>
    button.addEventListener("click", (e) => {
      e.stopPropagation();
      const index = Number(button.getAttribute("data-cart-index"));
      addToCart(state.products[index], 1);
    })
  );
}

export async function loadCatalog() {
  const response = await api("/api/shop/catalog");
  if (!response.ok) {
    renderProducts([]);
    log(`商品目录拉取失败 status=${response.status}`);
    return false;
  }
  renderProducts(response.data.items || []);
  log(`商品目录已刷新 items=${(response.data.items || []).length}`);
  return true;
}
