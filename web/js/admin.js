import { api } from "./api-client.js";

// ─── State ───
const state = {
  token: localStorage.getItem("fm_token") || "",
  tab: "dashboard",
};

const $ = (id) => document.getElementById(id);

function log(msg) {
  const box = $("console-log");
  if (box) box.textContent = `[${new Date().toLocaleTimeString()}] ${msg}\n${box.textContent}`;
}

// ─── Auth helpers ───
async function authed(path, opts = {}) {
  const headers = { "Content-Type": "application/json" };
  if (state.token) headers["Authorization"] = `Bearer ${state.token}`;
  if (opts.jsonBody) {
    opts.body = JSON.stringify(opts.jsonBody);
    delete opts.jsonBody;
  }
  const resp = await fetch(path, { ...opts, headers: { ...headers, ...(opts.headers || {}) } });
  if (resp.status === 401) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      headers["Authorization"] = `Bearer ${state.token}`;
      return fetch(path, { ...opts, headers: { ...headers, ...(opts.headers || {}) } }).then((r) => r.json().then((data) => ({ ok: r.ok, status: r.status, data })));
    }
  }
  const data = await resp.json().catch(() => ({}));
  return { ok: resp.ok, status: resp.status, data };
}

async function tryRefresh() {
  const resp = await fetch("/api/auth/refresh", { method: "POST" });
  if (!resp.ok) return false;
  const data = await resp.json();
  if (data.access_token) {
    state.token = data.access_token;
    localStorage.setItem("fm_token", state.token);
    return true;
  }
  return false;
}

function formatPriceFen(fen) {
  if (!fen && fen !== 0) return "0.00";
  return (fen / 100).toFixed(2);
}

// ─── Tab navigation ───
function switchTab(tab) {
  state.tab = tab;
  document.querySelectorAll(".admin-tab").forEach((el) => el.classList.remove("active"));
  document.querySelectorAll(".admin-panel").forEach((el) => el.classList.remove("active"));
  const tabBtn = document.querySelector(`[data-tab="${tab}"]`);
  if (tabBtn) tabBtn.classList.add("active");
  const panel = $(`panel-${tab}`);
  if (panel) panel.classList.add("active");

  if (tab === "dashboard") loadDashboard();
  else if (tab === "orders") loadOrders();
  else if (tab === "products") loadProducts();
  else if (tab === "users") loadUsers();
}

// ─── Dashboard ───
async function loadDashboard() {
  $("dashboard-content").innerHTML = '<div class="loading">加载中...</div>';
  const resp = await authed("/api/admin/dashboard/stats");
  if (!resp.ok) {
    $("dashboard-content").innerHTML = `<div class="error">加载失败: ${resp.status}</div>`;
    return;
  }
  const d = resp.data;
  $("dashboard-content").innerHTML = `
    <div class="stats-grid">
      <div class="stat-card"><div class="stat-val">${d.total_orders || 0}</div><div class="stat-label">总订单</div></div>
      <div class="stat-card revenue"><div class="stat-val">¥${formatPriceFen(d.total_revenue_fen)}</div><div class="stat-label">总收入</div></div>
      <div class="stat-card"><div class="stat-val">${d.total_products || 0}</div><div class="stat-label">商品数</div></div>
      <div class="stat-card"><div class="stat-val">${d.total_users || 0}</div><div class="stat-label">用户数</div></div>
      <div class="stat-card pending"><div class="stat-val">${d.pending_orders || 0}</div><div class="stat-label">待支付</div></div>
      <div class="stat-card paid"><div class="stat-val">${d.paid_orders || 0}</div><div class="stat-label">已支付</div></div>
      <div class="stat-card shipped"><div class="stat-val">${d.shipped_orders || 0}</div><div class="stat-label">已发货</div></div>
      <div class="stat-card completed"><div class="stat-val">${d.completed_orders || 0}</div><div class="stat-label">已完成</div></div>
    </div>`;
  log("仪表盘数据已刷新");
}

// ─── Orders ───
const STATUS_MAP = {
  0: { text: "待支付", cls: "pending" },
  1: { text: "已支付", cls: "paid" },
  2: { text: "已关闭", cls: "closed" },
  3: { text: "已发货", cls: "shipped" },
  4: { text: "已收货", cls: "completed" },
  5: { text: "退款中", cls: "refund" },
  6: { text: "已退款", cls: "refunded" },
};

let orderFilters = { status: "", page: 1, page_size: 20 };

async function loadOrders() {
  $("orders-tbody").innerHTML = '<tr><td colspan="7" class="loading">加载中...</td></tr>';
  const params = new URLSearchParams();
  if (orderFilters.status) params.set("status", orderFilters.status);
  params.set("page", orderFilters.page);
  params.set("page_size", orderFilters.page_size);
  const resp = await authed(`/api/admin/orders?${params}`);
  if (!resp.ok) {
    $("orders-tbody").innerHTML = `<tr><td colspan="7" class="error">加载失败</td></tr>`;
    return;
  }
  const items = resp.data.items || [];
  const total = resp.data.total || 0;
  $("orders-tbody").innerHTML = items.length === 0
    ? '<tr><td colspan="7" class="empty">暂无订单</td></tr>'
    : items.map((o) => {
      const st = STATUS_MAP[o.status] || { text: "未知", cls: "" };
      return `<tr>
        <td>${o.order_id}</td>
        <td>${o.user_id}</td>
        <td>${o.product_name || "—"}</td>
        <td>${o.amount}</td>
        <td>¥${formatPriceFen(o.payable_amount_fen)}</td>
        <td><span class="badge ${st.cls}">${st.text}</span></td>
        <td>${o.create_time || ""}</td>
      </tr>`;
    }).join("");
  $("orders-total").textContent = `共 ${total} 条`;
  log(`订单列表已刷新 items=${items.length} total=${total}`);
}

// ─── Products ───
async function loadProducts() {
  $("products-tbody").innerHTML = '<tr><td colspan="6" class="loading">加载中...</td></tr>';
  const resp = await authed("/api/admin/products?page=1&page_size=50");
  if (!resp.ok) {
    $("products-tbody").innerHTML = `<tr><td colspan="6" class="error">加载失败</td></tr>`;
    return;
  }
  const items = resp.data.items || [];
  $("products-tbody").innerHTML = items.length === 0
    ? '<tr><td colspan="6" class="empty">暂无商品</td></tr>'
    : items.map((p) => `<tr>
        <td>${p.product_id}</td>
        <td>${p.name}</td>
        <td>¥${formatPriceFen(p.origin_price_fen)}</td>
        <td>¥${formatPriceFen(p.sale_price_fen)}</td>
        <td>${p.stock_available ?? "—"}</td>
        <td>${p.status === 1 ? '<span class="badge paid">上架</span>' : '<span class="badge closed">下架</span>'}</td>
      </tr>`).join("");
  log(`商品列表已刷新 items=${items.length}`);
}

// ─── Users ───
async function loadUsers() {
  $("users-tbody").innerHTML = '<tr><td colspan="4" class="loading">加载中...</td></tr>';
  const resp = await authed("/api/admin/users?page=1&page_size=50");
  if (!resp.ok) {
    $("users-tbody").innerHTML = `<tr><td colspan="4" class="error">加载失败 (可能需要 auth-api 支持)</td></tr>`;
    return;
  }
  const items = resp.data.items || resp.data.users || [];
  $("users-tbody").innerHTML = items.length === 0
    ? '<tr><td colspan="4" class="empty">暂无用户数据</td></tr>'
    : items.map((u) => `<tr>
        <td>${u.user_id || u.id || "—"}</td>
        <td>${u.display_name || u.username || "—"}</td>
        <td>${u.phone || "—"}</td>
        <td>${u.created_at || u.create_time || "—"}</td>
      </tr>`).join("");
  log(`用户列表已刷新 items=${items.length}`);
}

// ─── Init ───
document.querySelectorAll("[data-tab]").forEach((btn) =>
  btn.addEventListener("click", () => switchTab(btn.getAttribute("data-tab")))
);

// Order filter buttons
$("order-filter-apply")?.addEventListener("click", () => {
  orderFilters.status = $("order-status-filter")?.value || "";
  orderFilters.page = 1;
  loadOrders();
});

// Check admin token
if (!state.token) {
  $("login-hint").style.display = "";
  $("admin-main").style.display = "none";
} else {
  $("login-hint").style.display = "none";
  $("admin-main").style.display = "";
  switchTab("dashboard");
}

log("后台管理已就绪");
