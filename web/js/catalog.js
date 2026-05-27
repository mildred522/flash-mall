import { state, $, log } from "./state.js";
import { api } from "./api-client.js";
import { order } from "./order.js";

export const PRODUCT_META = [
  {
    desc: "利落版型配轻盈面料，通勤出门都好搭。",
    badge: "限时秒杀",
    accent: "linear-gradient(160deg,#fff1f4 0%,#ffd9d9 100%)",
  },
];

export function formatPriceFen(value) {
  return `¥${(Number(value || 0) / 100).toFixed(2)}`;
}

export function renderProducts(items) {
  state.products = Array.isArray(items) ? items : [];
  const grid = $("product-grid");

  if (!state.products.length) {
    grid.innerHTML = `<article class="card product"><div class="thumb"></div><span class="pill">加载中</span>
      <h3 style="margin:0;font-size:22px">商品加载中</h3>
      <p>正在从 /api/shop/catalog 拉取今日主推商品。</p>
      <div class="row" style="justify-content:space-between"><span class="price">--</span><span class="chip">请稍候</span></div>
      <div class="row"><input class="qty" type="number" min="1" value="1" disabled />
      <button class="button primary" type="button" disabled>立即购买</button></div></article>`;
    return;
  }

  grid.innerHTML = state.products
    .map((product, index) => {
      const meta = PRODUCT_META[index % PRODUCT_META.length] || PRODUCT_META[0];
      const badge = product.promotion_tag || meta.badge;
      const stockText = product.stock_available > 0 ? `库存 ${product.stock_available}` : "暂时缺货";
      const originText =
        product.origin_price_fen > product.final_price_fen
          ? `<span class="note" style="text-decoration:line-through">${formatPriceFen(product.origin_price_fen)}</span>`
          : "";
      return `<article class="card product"><div class="thumb" style="background:${meta.accent}"></div>
        <span class="pill">${badge}</span>
        <h3 style="margin:0;font-size:22px">${product.name}</h3>
        <p>${meta.desc}</p>
        <div class="row" style="justify-content:space-between;align-items:flex-end">
          <div><span class="price">${formatPriceFen(product.final_price_fen)}</span>${originText}</div>
          <span class="chip">${stockText}</span>
        </div>
        <div class="row"><input class="qty" id="qty-${index}" type="number" min="1" value="1" ${product.stock_available > 0 ? "" : "disabled"} />
        <button class="button primary" type="button" data-buy-index="${index}" ${product.stock_available > 0 ? "" : "disabled"}>立即购买</button></div></article>`;
    })
    .join("");

  document.querySelectorAll("[data-buy-index]").forEach((button) =>
    button.addEventListener("click", async () => {
      const index = Number(button.getAttribute("data-buy-index"));
      const qty = Math.max(1, Number($(`qty-${index}`).value || 1));
      await order(state.products[index].product_id, qty, "card-buy");
    })
  );
}

export async function loadCatalog() {
  const response = await api("/api/shop/catalog");
  if (!response.ok) {
    renderProducts([]);
    log(`商品目录拉取失败 status=${response.status} body=${JSON.stringify(response.data)}`);
    return false;
  }
  renderProducts(response.data.items || []);
  log(`商品目录已刷新 items=${(response.data.items || []).length}`);
  return true;
}
