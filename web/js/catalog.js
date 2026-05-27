import { state, $, log } from "./state.js";
import { api } from "./api-client.js";
import { order } from "./order.js";

export const PRODUCT_META = [
  {
    desc: "利落版型配轻盈面料，通勤出门都好搭",
    tags: ["限时秒杀", "包邮"],
    accent: "linear-gradient(135deg, #fff1f4, #ffe8e0)",
  },
  {
    desc: "轻薄透气，春季必备百搭款",
    tags: ["新品首发", "包邮"],
    accent: "linear-gradient(135deg, #f0f4ff, #e8f0ff)",
  },
  {
    desc: "经典款型，品质面料，日常通勤首选",
    tags: ["热卖", "满减"],
    accent: "linear-gradient(135deg, #fff8f0, #fff0e0)",
  },
  {
    desc: "时尚设计，舒适穿着，换季必备",
    tags: ["人气款", "包邮"],
    accent: "linear-gradient(135deg, #f0fff4, #e0ffe8)",
  },
];

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
      const meta = PRODUCT_META[index % PRODUCT_META.length] || PRODUCT_META[0];
      const hasDiscount = product.origin_price_fen > product.final_price_fen;
      const discountPercent = hasDiscount
        ? Math.round((1 - product.final_price_fen / product.origin_price_fen) * 100)
        : 0;
      const badge = product.promotion_tag || (hasDiscount ? `${discountPercent}% OFF` : meta.tags[0]);
      const inStock = product.stock_available > 0;

      return `
      <article class="product">
        <div class="thumb" style="background:${meta.accent}">
          <div class="badge">
            <span class="pill">${badge}</span>
            ${meta.tags.length > 1 ? `<span class="pill orange">${meta.tags[1]}</span>` : ""}
          </div>
          <span class="placeholder-icon">📦</span>
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
            <button class="btn-buy" type="button" data-buy-index="${index}" ${inStock ? "" : "disabled"}>
              ${inStock ? "立即购买" : "已售罄"}
            </button>
          </div>
        </div>
      </article>`;
    })
    .join("");

  document.querySelectorAll("[data-buy-index]").forEach((button) =>
    button.addEventListener("click", async (e) => {
      e.stopPropagation();
      const index = Number(button.getAttribute("data-buy-index"));
      await order(state.products[index].product_id, 1, "card-buy");
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
