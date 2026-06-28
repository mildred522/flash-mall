const state = {
  token: localStorage.getItem("fm_token") || "",
  refreshToken: localStorage.getItem("fm_refresh") || "",
  tab: "dashboard",
  workspace: "platform",
  productFilters: { status: "", keyword: "", productId: "", supplierId: "", promotionStatus: "", stockStatus: "", page: 1, page_size: 20 },
  supplierFilters: { status: "", keyword: "", page: 1, page_size: 20 },
  promotionFilters: { status: "", keyword: "", productId: "", effectStatus: "", page: 1, page_size: 20 },
  refundFilters: { status: "0", orderId: "", userId: "", page: 1, page_size: 50 },
  supplierOptions: [],
  productOptions: [],
  userFilters: { status: "", role: "", keyword: "", page: 1, page_size: 20 },
  securityItems: [],
  selectedOrderId: "",
};

const $ = (id) => document.getElementById(id);

let toastTimer = 0;

function showToast(message) {
  const toast = $("admin-toast");
  if (!toast) return;
  toast.textContent = message;
  toast.classList.remove("hidden");
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => toast.classList.add("hidden"), 4200);
}

function log(msg) {
  const box = $("console-log");
  if (box) box.textContent = `[${new Date().toLocaleTimeString()}] ${msg}\n${box.textContent}`;
  if (/失败|错误|不能|请选择|请输入|无效|不存在|加载失败/.test(msg)) showToast(msg);
}

function openAdminFormModal({ title, fields, submitText = "确定", validate }) {
  const backdrop = $("admin-modal");
  const form = $("admin-modal-form");
  const fieldsEl = $("admin-modal-fields");
  const titleEl = $("admin-modal-title");
  const submit = $("admin-modal-submit");
  const errorBox = $("admin-modal-error");
  if (!backdrop || !form || !fieldsEl || !titleEl || !submit) return Promise.resolve(null);

  titleEl.textContent = title || "编辑";
  submit.textContent = submitText;
  if (errorBox) {
    errorBox.textContent = "";
    errorBox.classList.add("hidden");
  }
  fieldsEl.innerHTML = fields.map((field) => {
    const value = escapeHtml(field.value ?? "");
    const common = `id="modal-field-${escapeHtml(field.name)}" name="${escapeHtml(field.name)}"`;
    let control = "";
    if (field.type === "select") {
      control = `<select ${common}>${(field.options || []).map((option) => {
        const optionValue = String(option.value ?? "");
        return `<option value="${escapeHtml(optionValue)}" ${optionValue === String(field.value ?? "") ? "selected" : ""}>${escapeHtml(option.label ?? optionValue)}</option>`;
      }).join("")}</select>`;
    } else if (field.type === "textarea") {
      control = `<textarea ${common} ${field.required ? "required" : ""}>${value}</textarea>`;
    } else {
      control = `<input ${common} type="${escapeHtml(field.type || "text")}" value="${value}" ${field.min !== undefined ? `min="${escapeHtml(field.min)}"` : ""} ${field.required ? "required" : ""} />`;
    }
    return `<div class="modal-field"><label for="modal-field-${escapeHtml(field.name)}">${escapeHtml(field.label || field.name)}</label>${control}</div>`;
  }).join("");
  backdrop.classList.remove("hidden");
  backdrop.setAttribute("aria-hidden", "false");
  const first = fieldsEl.querySelector("input, select, textarea");
  if (first) first.focus();

  return new Promise((resolve) => {
    let done = false;
    const showError = (message) => {
      if (!errorBox) return;
      errorBox.textContent = message || "";
      errorBox.classList.toggle("hidden", !message);
      if (message) showToast(message);
    };
    const finish = (value) => {
      if (done) return;
      done = true;
      backdrop.classList.add("hidden");
      backdrop.setAttribute("aria-hidden", "true");
      form.removeEventListener("submit", onSubmit);
      $("admin-modal-cancel")?.removeEventListener("click", onCancel);
      $("admin-modal-close")?.removeEventListener("click", onCancel);
      backdrop.removeEventListener("click", onBackdrop);
      document.removeEventListener("keydown", onKeydown);
      resolve(value);
    };
    const onSubmit = (event) => {
      event.preventDefault();
      const data = {};
      for (const field of fields) {
        const input = form.elements[field.name];
        data[field.name] = input ? input.value : "";
      }
      const error = validate ? validate(data) : "";
      if (error) {
        showError(error);
        return;
      }
      finish(data);
    };
    const onCancel = () => finish(null);
    const onBackdrop = (event) => {
      if (event.target === backdrop) finish(null);
    };
    const onKeydown = (event) => {
      if (event.key === "Escape") finish(null);
    };
    form.addEventListener("submit", onSubmit);
    $("admin-modal-cancel")?.addEventListener("click", onCancel);
    $("admin-modal-close")?.addEventListener("click", onCancel);
    backdrop.addEventListener("click", onBackdrop);
    document.addEventListener("keydown", onKeydown);
  });
}

function productErrorMessage(error) {
  if (error === "sale_price_fen must be <= origin_price_fen") return "现价不能高于原价";
  if (error === "active supplier not found") return "请选择启用中的供应商";
  return error || "";
}

function supplierErrorMessage(error) {
  if (error === "supplier has active products") return "该供应商仍有关联的启用商品，请先下架或迁移商品";
  return error || "";
}

function userErrorMessage(error) {
  if (error === "cannot disable current admin") return "不能禁用当前登录的管理员账号";
  return error || "";
}

function orderErrorMessage(error) {
  if (error === "order not in paid status" || error === "order is not in paid status") return "订单不是已支付状态，不能发货";
  if (error === "order cannot be refunded" || error === "order cannot be refunded in current status") return "当前订单状态不能退款";
  if (error === "order cannot be closed") return "当前订单状态不能关闭";
  if (error === "status changed concurrently" || error === "order status changed concurrently") return "订单状态已变化，请刷新后重试";
  if (error === "order not found") return "订单不存在";
  return error || "";
}

async function authed(path, opts = {}) {
  const headers = { "Content-Type": "application/json" };
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  if (opts.jsonBody) {
    opts.body = JSON.stringify(opts.jsonBody);
    delete opts.jsonBody;
  }

  const resp = await fetch(path, { ...opts, headers: { ...headers, ...(opts.headers || {}) } });
  if (resp.status === 401) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      headers.Authorization = `Bearer ${state.token}`;
      const retry = await fetch(path, { ...opts, headers: { ...headers, ...(opts.headers || {}) } });
      const data = await retry.json().catch(() => ({}));
      return { ok: retry.ok, status: retry.status, data };
    }
  }
  const data = await resp.json().catch(() => ({}));
  return { ok: resp.ok, status: resp.status, data };
}

async function authedForm(path, formData) {
  const headers = {};
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  const resp = await fetch(path, { method: "POST", headers, body: formData });
  if (resp.status === 401) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      headers.Authorization = `Bearer ${state.token}`;
      const retry = await fetch(path, { method: "POST", headers, body: formData });
      const data = await retry.json().catch(() => ({}));
      return { ok: retry.ok, status: retry.status, data };
    }
  }
  const data = await resp.json().catch(() => ({}));
  return { ok: resp.ok, status: resp.status, data };
}

async function tryRefresh() {
  if (!state.refreshToken) return false;
  const resp = await fetch("/api/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${state.token}` },
    body: JSON.stringify({ refresh_token: state.refreshToken }),
  });
  if (!resp.ok) return false;
  const data = await resp.json().catch(() => ({}));
  if (!data.access_token) return false;
  state.token = data.access_token;
  localStorage.setItem("fm_token", state.token);
  if (data.refresh_token) {
    state.refreshToken = data.refresh_token;
    localStorage.setItem("fm_refresh", data.refresh_token);
  }
  return true;
}

function formatPriceFen(fen) {
  if (!fen && fen !== 0) return "0.00";
  return (fen / 100).toFixed(2);
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

const WORKSPACE_TABS = {
  platform: ["dashboard", "merchant-applications", "users", "reconciliation", "events", "security"],
  merchant: ["orders", "refunds", "products", "suppliers", "promotions"],
};

function workspaceForTab(tab) {
  for (const [workspace, tabs] of Object.entries(WORKSPACE_TABS)) {
    if (tabs.includes(tab)) return workspace;
  }
  return "platform";
}

function firstTabForWorkspace(workspace) {
  return (WORKSPACE_TABS[workspace] || WORKSPACE_TABS.platform)[0];
}

function setWorkspace(workspace, options = {}) {
  const nextWorkspace = WORKSPACE_TABS[workspace] ? workspace : "platform";
  state.workspace = nextWorkspace;
  document.querySelectorAll(".workspace-tab").forEach((el) => {
    el.classList.toggle("active", el.getAttribute("data-workspace") === nextWorkspace);
  });
  document.querySelectorAll(".admin-tab").forEach((el) => {
    const visible = el.getAttribute("data-workspace-scope") === nextWorkspace;
    el.hidden = !visible;
  });
  if (options.switchDefault) switchTab(firstTabForWorkspace(nextWorkspace));
}

function currentUserId() {
  try {
    const payload = state.token.split(".")[1];
    if (!payload) return 0;
    const normalized = payload.replaceAll("-", "+").replaceAll("_", "/");
    const parsed = JSON.parse(atob(normalized));
    return Number(parsed.user_id || 0);
  } catch {
    return 0;
  }
}

function switchTab(tab) {
  const normalizedTab = document.querySelector(`[data-tab="${tab}"]`) ? tab : firstTabForWorkspace(state.workspace);
  const targetWorkspace = workspaceForTab(normalizedTab);
  if (state.workspace !== targetWorkspace) setWorkspace(targetWorkspace);
  state.tab = normalizedTab;
  document.querySelectorAll(".admin-tab").forEach((el) => el.classList.remove("active"));
  document.querySelectorAll(".admin-panel").forEach((el) => el.classList.remove("active"));
  const tabBtn = document.querySelector(`[data-tab="${normalizedTab}"]`);
  if (tabBtn) tabBtn.classList.add("active");
  const panel = $(`panel-${normalizedTab}`);
  if (panel) panel.classList.add("active");

  tab = normalizedTab;
  if (tab === "dashboard") loadDashboard();
  else if (tab === "orders") loadOrders();
  else if (tab === "refunds") loadRefunds();
  else if (tab === "products") loadProducts();
  else if (tab === "suppliers") loadSuppliers();
  else if (tab === "promotions") loadPromotions();
  else if (tab === "merchant-applications") loadMerchantApplications();
  else if (tab === "users") loadUsers();
  else if (tab === "reconciliation") loadReconciliationIssues();
  else if (tab === "events") loadAdminEvents();
  else if (tab === "security") loadSecurityEvents();
}

async function loadDashboard() {
  $("dashboard-content").innerHTML = '<div class="loading">加载中...</div>';
  const resp = await authed("/api/admin/dashboard/stats");
  if (!resp.ok || resp.data.error) {
    $("dashboard-content").innerHTML = `<div class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</div>`;
    return;
  }
  const d = resp.data;
  $("dashboard-content").innerHTML = `
    <div class="stats-grid">
      <div class="stat-card clickable" data-dashboard-orders=""><div class="stat-val">${d.total_orders || 0}</div><div class="stat-label">总订单</div></div>
      <div class="stat-card revenue"><div class="stat-val">¥${formatPriceFen(d.total_revenue_fen)}</div><div class="stat-label">总收入</div></div>
      <div class="stat-card clickable" data-dashboard-tab="products"><div class="stat-val">${d.total_products || 0}</div><div class="stat-label">商品数</div></div>
      <div class="stat-card clickable" data-dashboard-tab="suppliers"><div class="stat-val">${d.total_suppliers || 0}</div><div class="stat-label">供应商数</div></div>
      <div class="stat-card clickable" data-dashboard-tab="promotions"><div class="stat-val">${d.total_promotions || 0}</div><div class="stat-label">促销数</div></div>
      <div class="stat-card shipped clickable" data-dashboard-promotions="active"><div class="stat-val">${d.active_promotions || 0}</div><div class="stat-label">生效活动</div></div>
      <div class="stat-card clickable" data-dashboard-tab="users"><div class="stat-val">${d.total_users || 0}</div><div class="stat-label">用户数</div></div>
      <div class="stat-card clickable" data-dashboard-tab="security"><div class="stat-val">查看</div><div class="stat-label">安全日志</div></div>
      <div class="stat-card pending clickable" data-dashboard-products-stock="2"><div class="stat-val">${d.low_stock_products || 0}</div><div class="stat-label">低库存商品</div></div>
      <div class="stat-card closed clickable" data-dashboard-products-stock="3"><div class="stat-val">${d.out_of_stock_products || 0}</div><div class="stat-label">缺货商品</div></div>
      <div class="stat-card pending clickable" data-dashboard-orders="0"><div class="stat-val">${d.pending_orders || 0}</div><div class="stat-label">待支付</div></div>
      <div class="stat-card paid clickable" data-dashboard-orders="1"><div class="stat-val">${d.paid_orders || 0}</div><div class="stat-label">已支付</div></div>
      <div class="stat-card shipped clickable" data-dashboard-orders="3"><div class="stat-val">${d.shipped_orders || 0}</div><div class="stat-label">已发货</div></div>
      <div class="stat-card completed clickable" data-dashboard-orders="4"><div class="stat-val">${d.completed_orders || 0}</div><div class="stat-label">已完成</div></div>
      <div class="stat-card refund clickable" data-dashboard-tab="refunds"><div class="stat-val">${d.refund_requested || 0}</div><div class="stat-label">待处理退款</div></div>
      <div class="stat-card refunded clickable" data-dashboard-orders="6"><div class="stat-val">${d.refunded_orders || 0}</div><div class="stat-label">已退款</div></div>
      <div class="stat-card pending clickable" data-dashboard-tab="reconciliation"><div class="stat-val">${d.open_reconciliation_issues || 0}</div><div class="stat-label">对账风险</div></div>
      <div class="stat-card pending clickable" data-dashboard-tab="events"><div class="stat-val">${d.pending_events || 0}</div><div class="stat-label">待处理事件</div></div>
      <div class="stat-card closed clickable" data-dashboard-tab="events"><div class="stat-val">${d.dead_events || 0}</div><div class="stat-label">死信事件</div></div>
    </div>`;
  document.querySelectorAll("[data-dashboard-tab]").forEach((card) => card.addEventListener("click", () => switchTab(card.getAttribute("data-dashboard-tab"))));
  document.querySelectorAll("[data-dashboard-orders]").forEach((card) => card.addEventListener("click", () => openDashboardOrders(card.getAttribute("data-dashboard-orders"))));
  document.querySelectorAll("[data-dashboard-products-stock]").forEach((card) => card.addEventListener("click", () => openDashboardProducts(card.getAttribute("data-dashboard-products-stock"))));
  document.querySelectorAll("[data-dashboard-promotions]").forEach((card) => card.addEventListener("click", () => openDashboardPromotions(card.getAttribute("data-dashboard-promotions"))));
  log("仪表盘数据已刷新");
}

async function loadMerchantApplications() {
  const tbody = $("merchant-applications-tbody");
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="8" class="loading">加载中...</td></tr>';
  const resp = await authed("/api/admin/merchants/applications");
  if (!resp.ok || resp.data.error) {
    tbody.innerHTML = `<tr><td colspan="8" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }
  const statusText = { 0: "待审核", 1: "已通过", 2: "已拒绝" };
  const items = resp.data.items || [];
  tbody.innerHTML = items.length === 0 ? '<tr><td colspan="8" class="empty">暂无商家申请</td></tr>' : items.map((item) => {
    const pending = Number(item.status) === 0;
    return `<tr>
      <td>${item.id || ""}</td>
      <td>${item.user_id || ""}</td>
      <td>${escapeHtml(item.merchant_name || "")}</td>
      <td>${escapeHtml(item.contact_phone || "")}</td>
      <td>${statusText[item.status] || item.status}</td>
      <td>${item.merchant_id || "—"}</td>
      <td>${escapeHtml(item.create_time || "")}</td>
      <td><div class="table-actions">
        ${pending ? `<button class="button small primary" data-merchant-apply-approve="${item.id}">通过</button>` : ""}
        ${pending ? `<button class="button small danger" data-merchant-apply-reject="${item.id}">拒绝</button>` : ""}
      </div></td>
    </tr>`;
  }).join("");
  document.querySelectorAll("[data-merchant-apply-approve]").forEach((btn) => btn.addEventListener("click", () => auditMerchantApplication(btn.getAttribute("data-merchant-apply-approve"), true)));
  document.querySelectorAll("[data-merchant-apply-reject]").forEach((btn) => btn.addEventListener("click", () => auditMerchantApplication(btn.getAttribute("data-merchant-apply-reject"), false)));
}

async function auditMerchantApplication(applyId, approve) {
  const values = await openAdminFormModal({
    title: approve ? "通过商家入驻" : "拒绝商家入驻",
    submitText: approve ? "通过" : "拒绝",
    fields: [{ name: "remark", label: "备注", type: "textarea", value: approve ? "审核通过" : "资料不完整", required: true }],
  });
  if (!values) return;
  const resp = await authed("/api/admin/merchants/applications/audit", {
    method: "POST",
    jsonBody: { apply_id: Number(applyId), approve, remark: values.remark.trim() },
  });
  if (!resp.ok || resp.data.error) {
    log(`商家审核失败 apply_id=${applyId} body=${JSON.stringify(resp.data)}`);
    return;
  }
  log(`商家审核完成 apply_id=${applyId}`);
  loadMerchantApplications();
}

function openDashboardOrders(status) {
  orderFilters = { status: status || "", order_id: "", user_id: "", product_id: "", product_name: "", created_from: "", created_to: "", page: 1, page_size: 20 };
  const statusFilter = $("order-status-filter");
  if (statusFilter) statusFilter.value = orderFilters.status;
  const orderFilter = $("order-id-filter");
  if (orderFilter) orderFilter.value = "";
  const userFilter = $("order-user-filter");
  if (userFilter) userFilter.value = "";
  const productFilter = $("order-product-filter");
  if (productFilter) productFilter.value = "";
  const productNameFilter = $("order-product-name-filter");
  if (productNameFilter) productNameFilter.value = "";
  const fromFilter = $("order-from-filter");
  if (fromFilter) fromFilter.value = "";
  const toFilter = $("order-to-filter");
  if (toFilter) toFilter.value = "";
  switchTab("orders");
}

function openDashboardProducts(stockStatus) {
  state.productFilters.status = "";
  state.productFilters.keyword = "";
  state.productFilters.productId = "";
  state.productFilters.supplierId = "";
  state.productFilters.promotionStatus = "";
  state.productFilters.stockStatus = String(stockStatus || "");
  state.productFilters.page = 1;
  const statusFilter = $("product-status-filter");
  if (statusFilter) statusFilter.value = "";
  const keywordFilter = $("product-keyword-filter");
  if (keywordFilter) keywordFilter.value = "";
  const productFilter = $("product-id-filter");
  if (productFilter) productFilter.value = "";
  const supplierFilter = $("product-supplier-filter");
  if (supplierFilter) supplierFilter.value = "";
  const promotionFilter = $("product-promotion-filter");
  if (promotionFilter) promotionFilter.value = "";
  const stockFilter = $("product-stock-filter");
  if (stockFilter) stockFilter.value = state.productFilters.stockStatus;
  switchTab("products");
}

function openDashboardPromotions(effectStatus) {
  state.promotionFilters.status = "";
  state.promotionFilters.keyword = "";
  state.promotionFilters.productId = "";
  state.promotionFilters.effectStatus = String(effectStatus || "");
  state.promotionFilters.page = 1;
  const statusFilter = $("promotion-status-filter");
  if (statusFilter) statusFilter.value = "";
  const keywordFilter = $("promotion-keyword-filter");
  if (keywordFilter) keywordFilter.value = "";
  const productFilter = $("promotion-product-filter");
  if (productFilter) productFilter.value = "";
  const effectFilter = $("promotion-effect-filter");
  if (effectFilter) effectFilter.value = state.promotionFilters.effectStatus;
  switchTab("promotions");
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

const ORDER_ACTION = {
  canClose: (status) => Number(status) === 0,
  canShip: (status) => Number(status) === 1,
  canRefund: (status) => Number(status) === 1 || Number(status) === 3,
};

let orderFilters = { status: "", order_id: "", user_id: "", product_id: "", product_name: "", created_from: "", created_to: "", page: 1, page_size: 20 };

async function loadOrders() {
  $("orders-tbody").innerHTML = '<tr><td colspan="8" class="loading">加载中...</td></tr>';
  const params = new URLSearchParams();
  if (orderFilters.status) params.set("status", orderFilters.status);
  if (orderFilters.order_id) params.set("order_id", orderFilters.order_id);
  if (orderFilters.user_id) params.set("user_id", orderFilters.user_id);
  if (orderFilters.product_id) params.set("product_id", orderFilters.product_id);
  if (orderFilters.product_name) params.set("product_name", orderFilters.product_name);
  if (orderFilters.created_from) params.set("created_from", orderFilters.created_from);
  if (orderFilters.created_to) params.set("created_to", orderFilters.created_to);
  params.set("page", orderFilters.page);
  params.set("page_size", orderFilters.page_size);
  const orderListPath = state.workspace === "merchant" ? "/api/merchant/orders" : "/api/admin/orders";
  const resp = await authed(`${orderListPath}?${params}`);
  if (!resp.ok || resp.data.error) {
    $("orders-tbody").innerHTML = `<tr><td colspan="8" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }

  const items = resp.data.items || [];
  const total = resp.data.total || 0;
  $("orders-tbody").innerHTML = items.length === 0 ? '<tr><td colspan="8" class="empty">暂无订单</td></tr>' : items.map((o) => {
    const st = STATUS_MAP[o.status] || { text: o.status_text || "未知", cls: "" };
    const isMerchantWorkspace = state.workspace === "merchant";
    const canShip = ORDER_ACTION.canShip(o.status);
    const canClose = !isMerchantWorkspace && ORDER_ACTION.canClose(o.status);
    const canRefund = !isMerchantWorkspace && ORDER_ACTION.canRefund(o.status);
    return `<tr>
        <td>${escapeHtml(o.order_id)}</td>
        <td><button class="link-button" data-order-user="${o.user_id}">${o.user_id}</button></td>
        <td><button class="link-button" data-order-product="${o.product_id}">${escapeHtml(o.product_name || "—")}</button></td>
        <td>${o.amount}</td>
        <td>¥${formatPriceFen(o.payable_amount_fen)}</td>
        <td><span class="badge ${st.cls}">${st.text}</span></td>
        <td>${escapeHtml(o.create_time || "")}</td>
        <td>
          <div class="table-actions">
            <button class="button small ghost" data-order-detail="${escapeHtml(o.order_id)}">详情</button>
            <button class="button small ghost" data-order-security="${escapeHtml(o.order_id)}">安全</button>
            ${canClose ? `<button class="button small danger" data-order-close="${escapeHtml(o.order_id)}">关闭</button>` : ""}
            ${canShip ? `<button class="button small ship" data-order-ship="${escapeHtml(o.order_id)}">发货</button>` : ""}
            ${canRefund ? `<button class="button small danger" data-order-refund="${escapeHtml(o.order_id)}">直接退款</button>` : ""}
          </div>
        </td>
      </tr>`;
  }).join("");
  document.querySelectorAll("[data-order-detail]").forEach((btn) => btn.addEventListener("click", () => loadOrderDetail(btn.getAttribute("data-order-detail"))));
  document.querySelectorAll("[data-order-security]").forEach((btn) => btn.addEventListener("click", () => openSecurityKeyword(`order:${btn.getAttribute("data-order-security")}`)));
  document.querySelectorAll("[data-order-user]").forEach((btn) => btn.addEventListener("click", () => openUserDetailFromOrder(btn.getAttribute("data-order-user"))));
  document.querySelectorAll("[data-order-product]").forEach((btn) => btn.addEventListener("click", () => openProductDetailFromOrder(btn.getAttribute("data-order-product"))));
  document.querySelectorAll("[data-order-close]").forEach((btn) => btn.addEventListener("click", () => closeOrder(btn.getAttribute("data-order-close"))));
  document.querySelectorAll("[data-order-ship]").forEach((btn) => btn.addEventListener("click", () => shipOrder(btn.getAttribute("data-order-ship"))));
  document.querySelectorAll("[data-order-refund]").forEach((btn) => btn.addEventListener("click", () => refundOrder(btn.getAttribute("data-order-refund"))));
  $("orders-total").textContent = `共 ${total} 条`;
  updateOrderPager(total);
  if (state.selectedOrderId) loadOrderDetail(state.selectedOrderId);
  log(`订单列表已刷新 items=${items.length} total=${total}`);
}

function updateOrderPager(total) {
  const page = Number(orderFilters.page || 1);
  const pageSize = Number(orderFilters.page_size || 20);
  const totalPages = Math.max(1, Math.ceil(Number(total || 0) / pageSize));
  if (page > totalPages) {
    orderFilters.page = totalPages;
    loadOrders();
    return;
  }
  const pageLabel = $("orders-page");
  if (pageLabel) pageLabel.textContent = `第 ${page} / ${totalPages} 页`;
  const sizeSelect = $("orders-page-size");
  if (sizeSelect) sizeSelect.value = String(pageSize);
  const prev = $("orders-prev");
  if (prev) prev.disabled = page <= 1;
  const next = $("orders-next");
  if (next) next.disabled = page >= totalPages;
}

async function loadOrderDetail(orderId) {
  if (!orderId) return;
  state.selectedOrderId = orderId;
  const panel = $("order-detail-panel");
  panel.style.display = "";
  panel.innerHTML = '<div class="loading">详情加载中...</div>';
  const [detailResp, logsResp] = await Promise.all([
    authed(`/api/admin/orders/detail?order_id=${encodeURIComponent(orderId)}`),
    authed(`/api/admin/orders/status-logs?order_id=${encodeURIComponent(orderId)}`),
  ]);
  if (!detailResp.ok || detailResp.data.error) {
    panel.innerHTML = `<div class="error">详情加载失败: ${escapeHtml(detailResp.data.error || detailResp.status)}</div>`;
    return;
  }
  renderOrderDetail(detailResp.data, logsResp.ok ? (logsResp.data.items || []) : []);
}

function openProductDetailFromOrder(productId) {
  state.productFilters.productId = String(productId || "");
  state.productFilters.status = "";
  state.productFilters.keyword = "";
  state.productFilters.supplierId = "";
  state.productFilters.promotionStatus = "";
  state.productFilters.stockStatus = "";
  state.productFilters.page = 1;
  const productIdFilter = $("product-id-filter");
  if (productIdFilter) productIdFilter.value = state.productFilters.productId;
  const keywordFilter = $("product-keyword-filter");
  if (keywordFilter) keywordFilter.value = "";
  const statusFilter = $("product-status-filter");
  if (statusFilter) statusFilter.value = "";
  const supplierFilter = $("product-supplier-filter");
  if (supplierFilter) supplierFilter.value = "";
  const promotionFilter = $("product-promotion-filter");
  if (promotionFilter) promotionFilter.value = "";
  const stockFilter = $("product-stock-filter");
  if (stockFilter) stockFilter.value = "";
  switchTab("products");
  loadProductDetail(productId);
}

async function loadProductDetail(productId) {
  const panel = $("product-detail-panel");
  if (!panel || !productId) return;
  panel.style.display = "";
  panel.innerHTML = '<div class="loading">商品详情加载中...</div>';
  const resp = await authed(`/api/admin/products/detail?product_id=${encodeURIComponent(String(productId))}`);
  if (!resp.ok || resp.data.error) {
    panel.innerHTML = `<div class="error">商品详情加载失败: ${escapeHtml(productErrorMessage(resp.data.error) || resp.status)}</div>`;
    return;
  }
  renderProductDetail(resp.data);
}

function renderProductDetail(product) {
  const productId = Number(product.product_id || 0);
  const isActive = Number(product.status || 0) === 1;
  const currentPrice = Number(product.promotion_price_fen || 0) > 0 ? Number(product.promotion_price_fen || 0) : Number(product.sale_price_fen || 0);
  $("product-detail-panel").innerHTML = `
    <h3>商品详情 ${productId}</h3>
    <div class="detail-grid">
      <div class="detail-item"><span class="detail-label">名称</span><span class="detail-value">${escapeHtml(product.name || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">供应商</span><span class="detail-value">${supplierDisplayName(product.supplier_id, product.supplier_name)}</span></div>
      <div class="detail-item"><span class="detail-label">原价</span><span class="detail-value">¥${formatPriceFen(product.origin_price_fen)}</span></div>
      <div class="detail-item"><span class="detail-label">售价</span><span class="detail-value">¥${formatPriceFen(product.sale_price_fen)}</span></div>
      <div class="detail-item"><span class="detail-label">当前价</span><span class="detail-value">¥${formatPriceFen(currentPrice)}</span></div>
      <div class="detail-item"><span class="detail-label">库存</span><span class="detail-value">${Number(product.stock_available || 0)}</span></div>
      <div class="detail-item"><span class="detail-label">活动</span><span class="detail-value">${escapeHtml(product.promotion_tag || product.promotion_type || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">状态</span><span class="badge ${isActive ? "paid" : "closed"}">${isActive ? "上架" : "下架"}</span></div>
    </div>
    <div class="table-actions" style="margin-top:12px">
      ${isActive ? `<button class="button small danger" data-product-detail-down="${productId}">下架</button>` : `<button class="button small primary" data-product-detail-up="${productId}">上架</button>`}
      <button class="button small ghost" data-product-detail-stock-plus="${productId}">+10</button>
      <button class="button small ghost" data-product-detail-stock-minus="${productId}">-10</button>
      <button class="button small ghost" data-product-detail-promotions="${productId}">查看促销</button>
      <button class="button small ghost" data-product-detail-orders="${productId}">查看订单</button>
      <button class="button small ghost" data-product-detail-security="${productId}">安全日志</button>
    </div>`;
  document.querySelector("[data-product-detail-up]")?.addEventListener("click", () => setProductStatus(productId, 1));
  document.querySelector("[data-product-detail-down]")?.addEventListener("click", () => setProductStatus(productId, 2));
  document.querySelector("[data-product-detail-stock-plus]")?.addEventListener("click", () => adjustProductStock(productId, 10));
  document.querySelector("[data-product-detail-stock-minus]")?.addEventListener("click", () => adjustProductStock(productId, -10));
  document.querySelector("[data-product-detail-promotions]")?.addEventListener("click", () => openProductPromotions(productId));
  document.querySelector("[data-product-detail-orders]")?.addEventListener("click", () => openProductOrders(productId));
  document.querySelector("[data-product-detail-security]")?.addEventListener("click", () => openSecurityKeyword(`product:${productId}`));
  log(`商品详情已加载 product_id=${productId}`);
}


function renderOrderDetail(detail, logs) {
  const st = STATUS_MAP[detail.status] || { text: detail.status_text || "未知", cls: "" };
  const logHtml = logs.length === 0 ? '<div class="empty">暂无状态日志</div>' : logs.map((item) => `
    <div class="timeline-item">
      <div><strong>${escapeHtml(item.from_status_text)}</strong> → <strong>${escapeHtml(item.to_status_text)}</strong></div>
      <div>${escapeHtml(item.remark || "")}</div>
      <div class="timeline-meta">operator=${operatorLink(item.operator_id)} · ${escapeHtml(item.create_time || "")}</div>
    </div>
  `).join("");
  $("order-detail-panel").innerHTML = `
    <h3>订单详情 ${escapeHtml(detail.order_id)}</h3>
    <div class="detail-grid">
      <div class="detail-item"><span class="detail-label">用户ID</span><span class="detail-value">${Number(detail.user_id || 0) || "—"}</span></div>
      <div class="detail-item"><span class="detail-label">商品</span><span class="detail-value">${escapeHtml(detail.product_name || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">数量</span><span class="detail-value">${detail.amount || 0}</span></div>
      <div class="detail-item"><span class="detail-label">订单状态</span><span class="badge ${st.cls}">${st.text}</span></div>
      <div class="detail-item"><span class="detail-label">支付状态</span><span class="detail-value">${escapeHtml(detail.payment_status_text || detail.payment_status || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">支付单</span><span class="detail-value">${escapeHtml(detail.payment_order_id || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">原单价</span><span class="detail-value">¥${formatPriceFen(detail.origin_unit_price_fen)}</span></div>
      <div class="detail-item"><span class="detail-label">成交单价</span><span class="detail-value">¥${formatPriceFen(detail.sale_unit_price_fen)}</span></div>
      <div class="detail-item"><span class="detail-label">应付</span><span class="detail-value">¥${formatPriceFen(detail.payable_amount_fen)}</span></div>
      <div class="detail-item"><span class="detail-label">优惠</span><span class="detail-value">¥${formatPriceFen(detail.discount_amount_fen)}</span></div>
      <div class="detail-item"><span class="detail-label">促销</span><span class="detail-value">${escapeHtml(detail.promotion_tag || detail.promotion_type || "—")}</span></div>
    </div>
    <div class="table-actions" style="margin-top:12px">
      ${Number(detail.status || 0) === 0 ? `<button class="button small danger" data-order-detail-close="${escapeHtml(detail.order_id)}">关闭订单</button>` : ""}
      ${Number(detail.status || 0) === 1 ? `<button class="button small ship" data-order-detail-ship="${escapeHtml(detail.order_id)}">发货</button>` : ""}
      ${Number(detail.status || 0) === 1 || Number(detail.status || 0) === 3 ? `<button class="button small danger" data-order-detail-refund="${escapeHtml(detail.order_id)}">直接退款</button>` : ""}
      <button class="button small ghost" data-order-detail-user="${Number(detail.user_id || 0)}" ${Number(detail.user_id || 0) ? "" : "disabled"}>查看用户</button>
      <button class="button small ghost" data-order-detail-product="${Number(detail.product_id || 0)}" ${Number(detail.product_id || 0) ? "" : "disabled"}>查看商品</button>
      <button class="button small ghost" data-order-detail-security="${escapeHtml(detail.order_id)}">安全日志</button>
    </div>
    <h3>状态日志</h3>
    <div class="timeline">${logHtml}</div>
  `;
  document.querySelector("[data-order-detail-ship]")?.addEventListener("click", () => shipOrder(detail.order_id));
  document.querySelector("[data-order-detail-close]")?.addEventListener("click", () => closeOrder(detail.order_id));
  document.querySelector("[data-order-detail-refund]")?.addEventListener("click", () => refundOrder(detail.order_id));
  document.querySelector("[data-order-detail-user]")?.addEventListener("click", () => openUserDetailFromOrder(detail.user_id));
  document.querySelector("[data-order-detail-product]")?.addEventListener("click", () => openProductDetailFromOrder(detail.product_id));
  document.querySelector("[data-order-detail-security]")?.addEventListener("click", () => openSecurityKeyword(`order:${detail.order_id}`));
  document.querySelectorAll("[data-operator-user]").forEach((btn) => btn.addEventListener("click", () => openUserDetailFromOrder(btn.getAttribute("data-operator-user"))));
}

function operatorLink(operatorId) {
  const id = Number(operatorId || 0);
  if (!id) return "0";
  return `<button class="link-button" data-operator-user="${id}">${id}</button>`;
}

async function shipOrder(orderId) {
  const shipPath = state.workspace === "merchant" ? "/api/merchant/orders/ship" : "/api/admin/orders/ship";
  const resp = await authed(shipPath, { method: "POST", jsonBody: { order_id: orderId } });
  if (!resp.ok || resp.data.error) {
    log(`发货失败: ${orderErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`订单已发货 order_id=${orderId}`);
  loadOrders();
  if (state.selectedOrderId === orderId) loadOrderDetail(orderId);
}

async function closeOrder(orderId) {
  const values = await openAdminFormModal({
    title: "关闭订单",
    submitText: "确认关闭",
    fields: [
      { name: "reason", label: "关闭原因", type: "textarea", value: "admin close", required: true },
    ],
    validate: (data) => data.reason.trim() ? "" : "请输入关闭原因",
  });
  if (!values) return;
  const resp = await authed("/api/admin/orders/close", { method: "POST", jsonBody: { order_id: orderId, reason: values.reason.trim() } });
  if (!resp.ok || resp.data.error) {
    log(`关闭订单失败: ${orderErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`订单已关闭 order_id=${orderId}`);
  loadOrders();
  if (state.selectedOrderId === orderId) loadOrderDetail(orderId);
}

async function refundOrder(orderId) {
  const values = await openAdminFormModal({
    title: "直接退款",
    submitText: "确认直接退款",
    fields: [
      { name: "reason", label: "退款原因", type: "textarea", value: "admin refund", required: true },
    ],
    validate: (data) => data.reason.trim() ? "" : "请输入退款原因",
  });
  if (!values) return;
  const reason = values.reason.trim();
  const resp = await authed("/api/admin/orders/refund", { method: "POST", jsonBody: { order_id: orderId, reason } });
  if (!resp.ok || resp.data.error) {
    log(`退款失败: ${orderErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`订单已退款 order_id=${orderId}`);
  loadOrders();
  if (state.selectedOrderId === orderId) loadOrderDetail(orderId);
}

const REFUND_STATUS_MAP = {
  0: { text: "待审核", cls: "refund" },
  1: { text: "已审核", cls: "paid" },
  2: { text: "已退款", cls: "refunded" },
  3: { text: "已驳回", cls: "closed" },
  4: { text: "退款失败", cls: "danger" },
};

async function loadRefunds() {
  const tbody = $("refunds-tbody");
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="8" class="loading">加载中...</td></tr>';
  const params = new URLSearchParams({
    page: String(state.refundFilters.page || 1),
    page_size: String(state.refundFilters.page_size || 50),
  });
  if (state.refundFilters.status !== "") params.set("status", state.refundFilters.status);
  if (state.refundFilters.orderId) params.set("order_id", state.refundFilters.orderId);
  if (state.refundFilters.userId) params.set("user_id", state.refundFilters.userId);
  const refundListPath = state.workspace === "merchant" ? "/api/merchant/refunds" : "/api/admin/refunds";
  const resp = await authed(`${refundListPath}?${params}`);
  if (!resp.ok || resp.data.error) {
    tbody.innerHTML = `<tr><td colspan="8" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }
  const items = resp.data.items || [];
  tbody.innerHTML = items.length === 0 ? '<tr><td colspan="8" class="empty">暂无退款单</td></tr>' : items.map((item) => {
    const st = REFUND_STATUS_MAP[Number(item.status)] || { text: item.status_text || "未知", cls: "" };
    const pending = Number(item.status) === 0 || Number(item.status) === 1;
    return `<tr>
      <td>${escapeHtml(item.refund_id || "")}</td>
      <td><button class="link-button" data-refund-order="${escapeHtml(item.order_id || "")}">${escapeHtml(item.order_id || "")}</button></td>
      <td><button class="link-button" data-refund-user="${item.user_id || ""}">${item.user_id || "—"}</button></td>
      <td>¥${formatPriceFen(item.refund_amount_fen || 0)}</td>
      <td><span class="badge ${st.cls}">${escapeHtml(st.text)}</span></td>
      <td>${escapeHtml(item.reason || "—")}</td>
      <td>${escapeHtml(item.request_time || "")}</td>
      <td><div class="table-actions">
        ${pending ? `<button class="button small primary" data-refund-approve="${escapeHtml(item.refund_id)}">通过</button>` : ""}
        ${pending ? `<button class="button small danger" data-refund-reject="${escapeHtml(item.refund_id)}">驳回</button>` : ""}
      </div></td>
    </tr>`;
  }).join("");
  document.querySelectorAll("[data-refund-order]").forEach((btn) => btn.addEventListener("click", () => {
    switchTab("orders");
    loadOrderDetail(btn.getAttribute("data-refund-order"));
  }));
  document.querySelectorAll("[data-refund-user]").forEach((btn) => btn.addEventListener("click", () => openUserDetailFromOrder(btn.getAttribute("data-refund-user"))));
  document.querySelectorAll("[data-refund-approve]").forEach((btn) => btn.addEventListener("click", () => auditRefund(btn.getAttribute("data-refund-approve"), true)));
  document.querySelectorAll("[data-refund-reject]").forEach((btn) => btn.addEventListener("click", () => auditRefund(btn.getAttribute("data-refund-reject"), false)));
  $("refunds-total").textContent = `共 ${resp.data.total || 0} 条`;
  log(`退款列表已刷新 items=${items.length} total=${resp.data.total || 0}`);
}

async function auditRefund(refundId, approve) {
  const values = await openAdminFormModal({
    title: approve ? "通过退款" : "驳回退款",
    submitText: approve ? "确认退款完成" : "确认驳回",
    fields: [
      { name: "remark", label: approve ? "处理备注" : "驳回原因", type: "textarea", value: approve ? "refund approved" : "refund rejected", required: true },
    ],
    validate: (data) => data.remark.trim() ? "" : "请输入处理说明",
  });
  if (!values) return;
  const resp = await authed("/api/admin/refunds/audit", {
    method: "POST",
    jsonBody: { refund_id: refundId, approve, remark: values.remark.trim() },
  });
  if (!resp.ok || resp.data.error) {
    log(`退款审核失败: ${escapeHtml(resp.data.error || resp.status)}`);
    return;
  }
  log(`退款审核完成 refund_id=${refundId} status=${resp.data.status || ""}`);
  loadRefunds();
  loadDashboard();
}

function supplierDisplayName(supplierId, supplierName = "") {
  if (supplierName) return `${escapeHtml(supplierName)} (${supplierId || "—"})`;
  const supplier = state.supplierOptions.find((item) => Number(item.supplier_id) === Number(supplierId));
  return supplier ? `${escapeHtml(supplier.name)} (${supplier.supplier_id})` : escapeHtml(supplierId || "—");
}

function productStockDisplay(stock) {
  const value = Number(stock || 0);
  if (value <= 0) return '<span class="badge closed">缺货</span>';
  if (value <= 100) return `<span class="badge pending">${value}</span>`;
  return String(value);
}

function productImageCell(imageUrl) {
  if (!imageUrl) return '<span class="product-image-empty">无图</span>';
  return `<img class="product-image-thumb" src="${escapeHtml(imageUrl)}" alt="商品图" loading="lazy" />`;
}

async function uploadProductImage(file) {
  if (!file) return "";
  const form = new FormData();
  form.append("image", file);
  const resp = await authedForm("/api/admin/products/image", form);
  if (!resp.ok || resp.data.error) {
    log(`图片上传失败: ${resp.data.error || resp.status}`);
    return "";
  }
  return resp.data.image_url || "";
}

async function loadProductSupplierOptions(selectedSupplierId = 0) {
  const select = $("product-supplier");
  if (!select) return [];
  const targetSupplierId = Number(selectedSupplierId || select.value || 0);
  select.innerHTML = '<option value="">供应商加载中...</option>';
  const resp = await authed("/api/admin/suppliers?status=1&page=1&page_size=100");
  if (!resp.ok || resp.data.error) {
    select.innerHTML = '<option value="">供应商加载失败</option>';
    log(`供应商选项加载失败: ${resp.data.error || resp.status}`);
    return [];
  }
  const items = resp.data.items || [];
  state.supplierOptions = [...items];
  if (targetSupplierId > 0 && !state.supplierOptions.some((s) => Number(s.supplier_id) === Number(targetSupplierId))) {
    state.supplierOptions.push({ supplier_id: Number(targetSupplierId), name: `供应商 ${targetSupplierId}`, status: 1 });
  }
  select.innerHTML = '<option value="">请选择供应商</option>' + state.supplierOptions
    .map((s) => `<option value="${s.supplier_id}">${escapeHtml(s.name)} (${s.supplier_id})</option>`)
    .join("");
  if (targetSupplierId > 0) select.value = String(targetSupplierId);
  else if (items.length === 1) select.value = String(items[0].supplier_id);
  return state.supplierOptions;
}

async function loadProducts() {
  await loadProductSupplierOptions();
  $("products-tbody").innerHTML = '<tr><td colspan="11" class="loading">加载中...</td></tr>';
  const params = new URLSearchParams({ page: String(state.productFilters.page || 1), page_size: String(state.productFilters.page_size || 20) });
  if (state.productFilters.productId) params.set("product_id", state.productFilters.productId);
  if (state.productFilters.supplierId) params.set("supplier_id", state.productFilters.supplierId);
  if (state.productFilters.status) params.set("status", state.productFilters.status);
  if (state.productFilters.keyword) params.set("keyword", state.productFilters.keyword);
  if (state.productFilters.promotionStatus) params.set("promotion_status", state.productFilters.promotionStatus);
  if (state.productFilters.stockStatus) params.set("stock_status", state.productFilters.stockStatus);
  const productListPath = state.workspace === "merchant" ? "/api/merchant/products" : "/api/admin/products";
  const resp = await authed(`${productListPath}?${params}`);
  if (!resp.ok || resp.data.error) {
    $("products-tbody").innerHTML = `<tr><td colspan="11" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }

  const items = resp.data.items || [];
  const total = resp.data.total || 0;
  $("products-tbody").innerHTML = items.length === 0 ? '<tr><td colspan="11" class="empty">暂无商品</td></tr>' : items.map((p) => `<tr>
        <td>${p.product_id}</td>
        <td>${productImageCell(p.image_url)}</td>
        <td>${escapeHtml(p.name)}</td>
        <td>${supplierDisplayName(p.supplier_id, p.supplier_name)}</td>
        <td>¥${formatPriceFen(p.origin_price_fen)}</td>
        <td>¥${formatPriceFen(p.sale_price_fen)}</td>
        <td>¥${formatPriceFen(p.promotion_price_fen > 0 ? p.promotion_price_fen : p.sale_price_fen)}</td>
        <td>${p.promotion_tag ? `<span class="badge shipped">${escapeHtml(p.promotion_tag)}</span>` : "—"}</td>
        <td>${productStockDisplay(p.stock_available)}</td>
        <td>${p.status === 1 ? '<span class="badge paid">上架</span>' : '<span class="badge closed">下架</span>'}</td>
        <td>
          <div class="table-actions">
            <button class="button small ghost" data-product-detail="${p.product_id}">详情</button>
            <button class="button small ghost" data-product-edit="${p.product_id}">编辑</button>
            <button class="button small ghost" data-product-promotion="${p.product_id}">促销</button>
            <button class="button small ghost" data-product-orders="${p.product_id}">订单</button>
            <button class="button small ghost" data-product-security="${p.product_id}">安全</button>
            ${p.status === 1 ? `<button class="button small danger" data-product-down="${p.product_id}">下架</button>` : `<button class="button small primary" data-product-up="${p.product_id}">上架</button>`}
            <button class="button small ghost" data-stock-plus="${p.product_id}">+10</button>
            <button class="button small ghost" data-stock-minus="${p.product_id}">-10</button>
          </div>
        </td>
      </tr>`).join("");
  document.querySelectorAll("[data-product-detail]").forEach((btn) => btn.addEventListener("click", () => loadProductDetail(btn.getAttribute("data-product-detail"))));
  document.querySelectorAll("[data-product-edit]").forEach((btn) => btn.addEventListener("click", () => editProduct(items.find((p) => String(p.product_id) === btn.getAttribute("data-product-edit")))));
  document.querySelectorAll("[data-product-promotion]").forEach((btn) => btn.addEventListener("click", () => openProductPromotions(btn.getAttribute("data-product-promotion"))));
  document.querySelectorAll("[data-product-orders]").forEach((btn) => btn.addEventListener("click", () => openProductOrders(btn.getAttribute("data-product-orders"))));
  document.querySelectorAll("[data-product-security]").forEach((btn) => btn.addEventListener("click", () => openSecurityKeyword(`product:${btn.getAttribute("data-product-security")}`)));
  document.querySelectorAll("[data-product-up]").forEach((btn) => btn.addEventListener("click", () => setProductStatus(btn.getAttribute("data-product-up"), 1)));
  document.querySelectorAll("[data-product-down]").forEach((btn) => btn.addEventListener("click", () => setProductStatus(btn.getAttribute("data-product-down"), 2)));
  document.querySelectorAll("[data-stock-plus]").forEach((btn) => btn.addEventListener("click", () => adjustProductStock(btn.getAttribute("data-stock-plus"), 10)));
  document.querySelectorAll("[data-stock-minus]").forEach((btn) => btn.addEventListener("click", () => adjustProductStock(btn.getAttribute("data-stock-minus"), -10)));
  $("products-total").textContent = `共 ${total} 条`;
  updateProductPager(total);
  log(`商品列表已刷新 items=${items.length} total=${total}`);
}

function updateProductPager(total) {
  const page = Number(state.productFilters.page || 1);
  const pageSize = Number(state.productFilters.page_size || 20);
  const totalPages = Math.max(1, Math.ceil(Number(total || 0) / pageSize));
  if (page > totalPages) {
    state.productFilters.page = totalPages;
    loadProducts();
    return;
  }
  const pageLabel = $("products-page");
  if (pageLabel) pageLabel.textContent = `第 ${page} / ${totalPages} 页`;
  const sizeSelect = $("products-page-size");
  if (sizeSelect) sizeSelect.value = String(pageSize);
  const prev = $("products-prev");
  if (prev) prev.disabled = page <= 1;
  const next = $("products-next");
  if (next) next.disabled = page >= totalPages;
}

async function createProduct() {
  const supplierId = Number($("product-supplier").value || 0);
  if (supplierId <= 0) {
    log("新增商品失败: 请选择供应商");
    return;
  }
  const payload = {
    name: $("product-name").value.trim(),
    image_url: $("product-image-url")?.value.trim() || "",
    origin_price_fen: Number($("product-origin-price").value || 0),
    sale_price_fen: Number($("product-sale-price").value || 0),
    stock_available: Number($("product-stock").value || 0),
    supplier_id: supplierId,
    status: 1,
  };
  if (payload.sale_price_fen > payload.origin_price_fen) {
    log("新增商品失败: 现价不能高于原价");
    return;
  }
  const imageFile = $("product-image-file")?.files?.[0];
  if (imageFile) {
    const imageUrl = await uploadProductImage(imageFile);
    if (!imageUrl) return;
    payload.image_url = imageUrl;
  }
  const productCreatePath = state.workspace === "merchant" ? "/api/merchant/products/create" : "/api/admin/products/create";
  const resp = await authed(productCreatePath, { method: "POST", jsonBody: payload });
  if (!resp.ok || resp.data.error) {
    log(`新增商品失败: ${productErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  ["product-name", "product-image-url", "product-origin-price", "product-sale-price", "product-stock"].forEach((id) => { if ($(id)) $(id).value = ""; });
  if ($("product-image-file")) $("product-image-file").value = "";
  $("product-supplier").value = "";
  log(`新增商品成功 product_id=${resp.data.product_id}`);
  state.productFilters.page = 1;
  loadProducts();
}

async function editProduct(product) {
  if (!product) return;
  const suppliers = await loadProductSupplierOptions(product.supplier_id);
  const values = await openAdminFormModal({
    title: `编辑商品 ${product.product_id}`,
    submitText: "保存",
    fields: [
      { name: "name", label: "商品名称", value: product.name || "", required: true },
      { name: "imageUrl", label: "图片地址", value: product.image_url || "" },
      { name: "originPrice", label: "原价(分)", type: "number", min: 0, value: String(product.origin_price_fen || 0), required: true },
      { name: "salePrice", label: "售价(分)", type: "number", min: 0, value: String(product.sale_price_fen || 0), required: true },
      {
        name: "supplierId",
        label: "供应商",
        type: "select",
        value: String(product.supplier_id || ""),
        options: suppliers
          .filter((supplier) => Number(supplier.status || 1) === 1 || Number(supplier.supplier_id) === Number(product.supplier_id))
          .map((supplier) => ({ value: supplier.supplier_id, label: `${supplier.name} (${supplier.supplier_id})` })),
      },
    ],
    validate: (data) => {
      if (!data.name.trim()) return "请输入商品名称";
      const originPrice = Number(data.originPrice || 0);
      const salePrice = Number(data.salePrice || 0);
      if (originPrice < 0 || salePrice < 0) return "价格不能为负数";
      if (salePrice > originPrice) return "售价不能高于原价";
      if (Number(data.supplierId || 0) <= 0) return "请选择供应商";
      return "";
    },
  });
  if (!values) return;
  const payload = {
    product_id: Number(product.product_id),
    name: values.name.trim(),
    image_url: values.imageUrl.trim(),
    origin_price_fen: Number(values.originPrice || 0),
    sale_price_fen: Number(values.salePrice || 0),
    supplier_id: Number(values.supplierId || 0),
  };
  const resp = await authed("/api/admin/products/update", { method: "POST", jsonBody: payload });
  if (!resp.ok || resp.data.error) {
    log(`编辑商品失败: ${productErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`商品已更新 product_id=${product.product_id}`);
  loadProducts();
  if ($("product-detail-panel")?.style.display !== "none") loadProductDetail(product.product_id);
}

async function setProductStatus(productId, status) {
  const resp = await authed("/api/admin/products/update", {
    method: "POST",
    jsonBody: { product_id: Number(productId), status },
  });
  if (!resp.ok || resp.data.error) {
    log(`商品状态更新失败: ${productErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`商品状态已更新 product_id=${productId} status=${status}`);
  loadProducts();
  if ($("product-detail-panel")?.style.display !== "none") loadProductDetail(productId);
}

async function adjustProductStock(productId, delta) {
  const resp = await authed("/api/admin/products/stock-adjust", {
    method: "POST",
    jsonBody: { product_id: Number(productId), delta, bucket_idx: 0 },
  });
  if (!resp.ok || resp.data.error) {
    log(`库存调整失败: ${resp.data.error || resp.status}`);
    return;
  }
  log(`库存已调整 product_id=${productId} stock=${resp.data.stock_available}`);
  loadProducts();
  if ($("product-detail-panel")?.style.display !== "none") loadProductDetail(productId);
}

function openProductPromotions(productId) {
  state.promotionFilters.productId = String(productId || "");
  state.promotionFilters.status = "";
  state.promotionFilters.keyword = "";
  state.promotionFilters.effectStatus = "";
  state.promotionFilters.page = 1;
  const productFilter = $("promotion-product-filter");
  if (productFilter) productFilter.value = state.promotionFilters.productId;
  const statusFilter = $("promotion-status-filter");
  if (statusFilter) statusFilter.value = "";
  const keywordFilter = $("promotion-keyword-filter");
  if (keywordFilter) keywordFilter.value = "";
  const effectFilter = $("promotion-effect-filter");
  if (effectFilter) effectFilter.value = "";
  switchTab("promotions");
}

function openProductOrders(productId) {
  orderFilters = { status: "", order_id: "", user_id: "", product_id: String(productId || ""), product_name: "", created_from: "", created_to: "", page: 1, page_size: 20 };
  const statusFilter = $("order-status-filter");
  if (statusFilter) statusFilter.value = "";
  const orderFilter = $("order-id-filter");
  if (orderFilter) orderFilter.value = "";
  const userFilter = $("order-user-filter");
  if (userFilter) userFilter.value = "";
  const productFilter = $("order-product-filter");
  if (productFilter) productFilter.value = orderFilters.product_id;
  const productNameFilter = $("order-product-name-filter");
  if (productNameFilter) productNameFilter.value = "";
  const fromFilter = $("order-from-filter");
  if (fromFilter) fromFilter.value = "";
  const toFilter = $("order-to-filter");
  if (toFilter) toFilter.value = "";
  switchTab("orders");
}

function openUserOrders(userId) {
  orderFilters = { status: "", order_id: "", user_id: String(userId || ""), product_id: "", product_name: "", created_from: "", created_to: "", page: 1, page_size: 20 };
  const statusFilter = $("order-status-filter");
  if (statusFilter) statusFilter.value = "";
  const orderFilter = $("order-id-filter");
  if (orderFilter) orderFilter.value = "";
  const userFilter = $("order-user-filter");
  if (userFilter) userFilter.value = orderFilters.user_id;
  const productFilter = $("order-product-filter");
  if (productFilter) productFilter.value = "";
  const productNameFilter = $("order-product-name-filter");
  if (productNameFilter) productNameFilter.value = "";
  const fromFilter = $("order-from-filter");
  if (fromFilter) fromFilter.value = "";
  const toFilter = $("order-to-filter");
  if (toFilter) toFilter.value = "";
  switchTab("orders");
}

function openUserSecurity(userId) {
  const userFilter = $("security-user-filter");
  if (userFilter) userFilter.value = String(userId || "");
  const keywordFilter = $("security-keyword-filter");
  if (keywordFilter) keywordFilter.value = "";
  const eventFilter = $("security-event-filter");
  if (eventFilter) eventFilter.value = "";
  const resultFilter = $("security-result-filter");
  if (resultFilter) resultFilter.value = "";
  switchTab("security");
}

function openSecurityKeyword(keyword) {
  const userFilter = $("security-user-filter");
  if (userFilter) userFilter.value = "";
  const keywordFilter = $("security-keyword-filter");
  if (keywordFilter) keywordFilter.value = String(keyword || "");
  const eventFilter = $("security-event-filter");
  if (eventFilter) eventFilter.value = "";
  const resultFilter = $("security-result-filter");
  if (resultFilter) resultFilter.value = "";
  switchTab("security");
}

async function loadSuppliers() {
  $("suppliers-tbody").innerHTML = '<tr><td colspan="6" class="loading">加载中...</td></tr>';
  const params = new URLSearchParams({ page: String(state.supplierFilters.page || 1), page_size: String(state.supplierFilters.page_size || 20) });
  if (state.supplierFilters.status) params.set("status", state.supplierFilters.status);
  if (state.supplierFilters.keyword) params.set("keyword", state.supplierFilters.keyword);
  const resp = await authed(`/api/admin/suppliers?${params}`);
  if (!resp.ok || resp.data.error) {
    $("suppliers-tbody").innerHTML = `<tr><td colspan="6" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }

  const items = resp.data.items || [];
  const total = resp.data.total || 0;
  state.supplierOptions = items.filter((s) => Number(s.status) === 1);
  $("suppliers-tbody").innerHTML = items.length === 0 ? '<tr><td colspan="6" class="empty">暂无供应商</td></tr>' : items.map((s) => {
    const isActive = Number(s.status) === 1;
    const productCount = Number(s.product_count || 0);
    const activeProducts = Number(s.active_products || 0);
    return `<tr>
        <td>${s.supplier_id}</td>
        <td>${escapeHtml(s.name || "—")}</td>
        <td><button class="link-button" data-supplier-products="${s.supplier_id}">${productCount}</button></td>
        <td>${activeProducts > 0 ? `<span class="badge paid">${activeProducts}</span>` : `<span class="badge closed">0</span>`}</td>
        <td>${isActive ? '<span class="badge paid">启用</span>' : '<span class="badge closed">停用</span>'}</td>
        <td>
          <div class="table-actions">
            <button class="button small ghost" data-supplier-detail="${s.supplier_id}">详情</button>
            <button class="button small ghost" data-supplier-edit="${s.supplier_id}">编辑</button>
            <button class="button small ghost" data-supplier-products="${s.supplier_id}">商品</button>
            <button class="button small ghost" data-supplier-security="${s.supplier_id}">安全</button>
            <button class="button small ${isActive ? "danger" : "primary"}" data-supplier-status="${s.supplier_id}" data-supplier-next-status="${isActive ? 2 : 1}">
              ${isActive ? "停用" : "启用"}
            </button>
          </div>
        </td>
      </tr>`;
  }).join("");
  document.querySelectorAll("[data-supplier-detail]").forEach((btn) => btn.addEventListener("click", () => loadSupplierDetail(btn.getAttribute("data-supplier-detail"))));
  document.querySelectorAll("[data-supplier-edit]").forEach((btn) => btn.addEventListener("click", () => editSupplier(items.find((s) => String(s.supplier_id) === btn.getAttribute("data-supplier-edit")))));
  document.querySelectorAll("[data-supplier-products]").forEach((btn) => btn.addEventListener("click", () => openSupplierProducts(btn.getAttribute("data-supplier-products"))));
  document.querySelectorAll("[data-supplier-security]").forEach((btn) => btn.addEventListener("click", () => openSecurityKeyword(`supplier:${btn.getAttribute("data-supplier-security")}`)));
  document.querySelectorAll("[data-supplier-status]").forEach((btn) => btn.addEventListener("click", () => setSupplierStatus(btn.getAttribute("data-supplier-status"), Number(btn.getAttribute("data-supplier-next-status")))));
  $("suppliers-total").textContent = `共 ${total} 条`;
  updateSupplierPager(total);
  log(`供应商列表已刷新 items=${items.length} total=${total}`);
}

function updateSupplierPager(total) {
  const page = Number(state.supplierFilters.page || 1);
  const pageSize = Number(state.supplierFilters.page_size || 20);
  const totalPages = Math.max(1, Math.ceil(Number(total || 0) / pageSize));
  if (page > totalPages) {
    state.supplierFilters.page = totalPages;
    loadSuppliers();
    return;
  }
  const pageLabel = $("suppliers-page");
  if (pageLabel) pageLabel.textContent = `第 ${page} / ${totalPages} 页`;
  const sizeSelect = $("suppliers-page-size");
  if (sizeSelect) sizeSelect.value = String(pageSize);
  const prev = $("suppliers-prev");
  if (prev) prev.disabled = page <= 1;
  const next = $("suppliers-next");
  if (next) next.disabled = page >= totalPages;
}

async function createSupplier() {
  const name = $("supplier-name").value.trim();
  if (!name) {
    log("新增供应商失败: 请输入供应商名称");
    return;
  }
  const resp = await authed("/api/admin/suppliers/create", {
    method: "POST",
    jsonBody: { name, status: 1 },
  });
  if (!resp.ok || resp.data.error) {
    log(`新增供应商失败: ${supplierErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  $("supplier-name").value = "";
  log(`新增供应商成功 supplier_id=${resp.data.supplier_id}`);
  state.supplierFilters.page = 1;
  loadProductSupplierOptions();
  loadSuppliers();
}

async function editSupplier(supplier) {
  if (!supplier) return;
  const values = await openAdminFormModal({
    title: `编辑供应商 ${supplier.supplier_id}`,
    submitText: "保存",
    fields: [
      { name: "name", label: "供应商名称", value: supplier.name || "", required: true },
    ],
    validate: (data) => data.name.trim() ? "" : "请输入供应商名称",
  });
  if (!values) return;
  const resp = await authed("/api/admin/suppliers/update", {
    method: "POST",
    jsonBody: { supplier_id: Number(supplier.supplier_id), name: values.name.trim() },
  });
  if (!resp.ok || resp.data.error) {
    log(`编辑供应商失败: ${supplierErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`供应商已更新 supplier_id=${supplier.supplier_id}`);
  loadProductSupplierOptions();
  loadSuppliers();
  if ($("supplier-detail-panel")?.style.display !== "none") loadSupplierDetail(supplier.supplier_id);
}

async function setSupplierStatus(supplierId, status) {
  const resp = await authed("/api/admin/suppliers/update", {
    method: "POST",
    jsonBody: { supplier_id: Number(supplierId), status },
  });
  if (!resp.ok || resp.data.error) {
    log(`供应商状态更新失败: ${supplierErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`供应商状态已更新 supplier_id=${supplierId} status=${status}`);
  loadProductSupplierOptions();
  loadSuppliers();
  if ($("supplier-detail-panel")?.style.display !== "none") loadSupplierDetail(supplierId);
}

async function loadSupplierDetail(supplierId) {
  const panel = $("supplier-detail-panel");
  if (!panel || !supplierId) return;
  panel.style.display = "";
  panel.innerHTML = '<div class="loading">供应商详情加载中...</div>';
  const resp = await authed(`/api/admin/suppliers/detail?supplier_id=${encodeURIComponent(String(supplierId))}`);
  if (!resp.ok || resp.data.error) {
    panel.innerHTML = `<div class="error">供应商详情加载失败: ${escapeHtml(supplierErrorMessage(resp.data.error) || resp.status)}</div>`;
    return;
  }
  renderSupplierDetail(resp.data);
}

function renderSupplierDetail(supplier) {
  const supplierId = Number(supplier.supplier_id || 0);
  const isActive = Number(supplier.status || 0) === 1;
  $("supplier-detail-panel").innerHTML = `
    <h3>供应商详情 ${supplierId}</h3>
    <div class="detail-grid">
      <div class="detail-item"><span class="detail-label">名称</span><span class="detail-value">${escapeHtml(supplier.name || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">状态</span><span class="badge ${isActive ? "paid" : "closed"}">${isActive ? "启用" : "停用"}</span></div>
      <div class="detail-item"><span class="detail-label">商品数</span><span class="detail-value">${Number(supplier.product_count || 0)}</span></div>
      <div class="detail-item"><span class="detail-label">启用商品</span><span class="detail-value">${Number(supplier.active_products || 0)}</span></div>
    </div>
    <div class="table-actions" style="margin-top:12px">
      <button class="button small ghost" data-supplier-detail-edit="${supplierId}">编辑</button>
      <button class="button small ${isActive ? "danger" : "primary"}" data-supplier-detail-status="${supplierId}" data-supplier-detail-next-status="${isActive ? 2 : 1}">${isActive ? "停用" : "启用"}</button>
      <button class="button small ghost" data-supplier-detail-products="${supplierId}">查看商品</button>
      <button class="button small ghost" data-supplier-detail-security="${supplierId}">安全日志</button>
    </div>`;
  document.querySelector("[data-supplier-detail-edit]")?.addEventListener("click", () => editSupplier(supplier));
  document.querySelector("[data-supplier-detail-status]")?.addEventListener("click", () => setSupplierStatus(supplierId, Number(document.querySelector("[data-supplier-detail-status]")?.getAttribute("data-supplier-detail-next-status"))));
  document.querySelector("[data-supplier-detail-products]")?.addEventListener("click", () => openSupplierProducts(supplierId));
  document.querySelector("[data-supplier-detail-security]")?.addEventListener("click", () => openSecurityKeyword(`supplier:${supplierId}`));
  log(`供应商详情已加载 supplier_id=${supplierId}`);
}

function openSupplierProducts(supplierId) {
  state.productFilters = { status: "", keyword: "", productId: "", supplierId: String(supplierId || ""), promotionStatus: "", stockStatus: "", page: 1, page_size: 20 };
  const keywordFilter = $("product-keyword-filter");
  if (keywordFilter) keywordFilter.value = "";
  const productIdFilter = $("product-id-filter");
  if (productIdFilter) productIdFilter.value = "";
  const supplierFilter = $("product-supplier-filter");
  if (supplierFilter) supplierFilter.value = state.productFilters.supplierId;
  const statusFilter = $("product-status-filter");
  if (statusFilter) statusFilter.value = "";
  const promotionFilter = $("product-promotion-filter");
  if (promotionFilter) promotionFilter.value = "";
  const stockFilter = $("product-stock-filter");
  if (stockFilter) stockFilter.value = "";
  switchTab("products");
}

function promotionProductDisplay(productId, productName = "") {
  const product = state.productOptions.find((item) => Number(item.product_id) === Number(productId));
  const name = productName || product?.name || "商品";
  return `${escapeHtml(name)} (${productId || "—"})`;
}

function promotionErrorMessage(error) {
  if (error === "active limited price promotion already exists") return "该商品已有启用中的限时价规则，请先停用原规则";
  if (error === "active limited price promotion window overlaps") return "该商品已有时间重叠的启用限时价规则";
  if (error === "ends_at must be after starts_at") return "结束时间必须晚于开始时间";
  if (error === "discount_value must be <= product sale_price_fen") return "限时价不能高于商品现价";
  return error || "";
}

function promotionTypeText(type) {
  return type === "LIMITED_PRICE" || type === "limited_price" ? "限时价" : type || "—";
}

async function loadPromotionProductOptions(selectedProductId = 0) {
  const select = $("promotion-product");
  if (!select) return [];
  const targetProductId = Number(selectedProductId || select.value || 0);
  select.innerHTML = '<option value="">商品加载中...</option>';
  const resp = await authed("/api/admin/products?page=1&page_size=100");
  if (!resp.ok || resp.data.error) {
    select.innerHTML = '<option value="">商品加载失败</option>';
    log(`促销商品选项加载失败: ${resp.data.error || resp.status}`);
    return [];
  }
  const items = resp.data.items || [];
  state.productOptions = [...items];
  if (targetProductId > 0 && !state.productOptions.some((p) => Number(p.product_id) === Number(targetProductId))) {
    state.productOptions.push({ product_id: Number(targetProductId), name: `商品 ${targetProductId}`, supplier_name: "" });
  }
  select.innerHTML = '<option value="">请选择商品</option>' + state.productOptions
    .map((p) => `<option value="${p.product_id}">${escapeHtml(p.name)} (${p.product_id})</option>`)
    .join("");
  if (targetProductId > 0) select.value = String(targetProductId);
  else if (items.length === 1) select.value = String(items[0].product_id);
  return state.productOptions;
}

async function loadPromotions() {
  await loadPromotionProductOptions();
  $("promotions-tbody").innerHTML = '<tr><td colspan="9" class="loading">加载中...</td></tr>';
  const params = new URLSearchParams({ page: String(state.promotionFilters.page || 1), page_size: String(state.promotionFilters.page_size || 20) });
  if (state.promotionFilters.status) params.set("status", state.promotionFilters.status);
  if (state.promotionFilters.keyword) params.set("keyword", state.promotionFilters.keyword);
  if (state.promotionFilters.productId) params.set("product_id", state.promotionFilters.productId);
  if (state.promotionFilters.effectStatus) params.set("effect_status", state.promotionFilters.effectStatus);
  const resp = await authed(`/api/admin/promotions?${params}`);
  if (!resp.ok || resp.data.error) {
    $("promotions-tbody").innerHTML = `<tr><td colspan="9" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }
  const items = resp.data.items || [];
  const total = resp.data.total || 0;
  $("promotions-tbody").innerHTML = items.length === 0 ? '<tr><td colspan="9" class="empty">暂无促销</td></tr>' : items.map((p) => {
    const isActive = Number(p.status) === 1;
    return `<tr>
        <td>${p.promotion_id}</td>
        <td>${promotionProductDisplay(p.product_id, p.product_name)}</td>
        <td>${escapeHtml(promotionTypeText(p.type))}</td>
        <td>${p.sale_price_fen > 0 ? `¥${formatPriceFen(p.sale_price_fen)} → ¥${formatPriceFen(p.discount_value)}` : `¥${formatPriceFen(p.discount_value)}`}</td>
        <td>${escapeHtml(p.starts_at || "—")}</td>
        <td>${escapeHtml(p.ends_at || "—")}</td>
        <td><span class="badge shipped">${escapeHtml(p.effect_status_text || "—")}</span></td>
        <td>${isActive ? '<span class="badge paid">启用</span>' : '<span class="badge closed">停用</span>'}</td>
        <td>
          <div class="table-actions">
            <button class="button small ghost" data-promotion-detail="${p.promotion_id}">详情</button>
            <button class="button small ghost" data-promotion-edit="${p.promotion_id}">编辑</button>
            <button class="button small ghost" data-promotion-security="${p.promotion_id}">安全</button>
            <button class="button small ${isActive ? "danger" : "primary"}" data-promotion-status="${p.promotion_id}" data-promotion-next-status="${isActive ? 2 : 1}">
              ${isActive ? "停用" : "启用"}
            </button>
          </div>
        </td>
      </tr>`;
  }).join("");
  document.querySelectorAll("[data-promotion-detail]").forEach((btn) => btn.addEventListener("click", () => loadPromotionDetail(btn.getAttribute("data-promotion-detail"))));
  document.querySelectorAll("[data-promotion-edit]").forEach((btn) => btn.addEventListener("click", () => editPromotion(items.find((p) => String(p.promotion_id) === btn.getAttribute("data-promotion-edit")))));
  document.querySelectorAll("[data-promotion-security]").forEach((btn) => btn.addEventListener("click", () => openSecurityKeyword(`promotion:${btn.getAttribute("data-promotion-security")}`)));
  document.querySelectorAll("[data-promotion-status]").forEach((btn) => btn.addEventListener("click", () => setPromotionStatus(btn.getAttribute("data-promotion-status"), Number(btn.getAttribute("data-promotion-next-status")))));
  $("promotions-total").textContent = `共 ${total} 条`;
  updatePromotionPager(total);
  log(`促销列表已刷新 items=${items.length} total=${total}`);
}

function updatePromotionPager(total) {
  const page = Number(state.promotionFilters.page || 1);
  const pageSize = Number(state.promotionFilters.page_size || 20);
  const totalPages = Math.max(1, Math.ceil(Number(total || 0) / pageSize));
  if (page > totalPages) {
    state.promotionFilters.page = totalPages;
    loadPromotions();
    return;
  }
  const pageLabel = $("promotions-page");
  if (pageLabel) pageLabel.textContent = `第 ${page} / ${totalPages} 页`;
  const sizeSelect = $("promotions-page-size");
  if (sizeSelect) sizeSelect.value = String(pageSize);
  const prev = $("promotions-prev");
  if (prev) prev.disabled = page <= 1;
  const next = $("promotions-next");
  if (next) next.disabled = page >= totalPages;
}

async function createPromotion() {
  const productId = Number($("promotion-product").value || 0);
  const discountValue = Number($("promotion-price").value || 0);
  if (productId <= 0 || discountValue <= 0) {
    log("新增促销失败: 请选择商品并填写限时价");
    return;
  }
  const product = state.productOptions.find((item) => Number(item.product_id) === productId);
  if (product && Number(product.sale_price_fen || 0) > 0 && discountValue > Number(product.sale_price_fen)) {
    log("新增促销失败: 限时价不能高于商品现价");
    return;
  }
  const resp = await authed("/api/admin/promotions/create", {
    method: "POST",
    jsonBody: {
      product_id: productId,
      type: "LIMITED_PRICE",
      discount_value: discountValue,
      threshold_amount: 0,
      starts_at: $("promotion-starts").value.trim(),
      ends_at: $("promotion-ends").value.trim(),
      status: 1,
    },
  });
  if (!resp.ok || resp.data.error) {
    log(`新增促销失败: ${promotionErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  ["promotion-price", "promotion-starts", "promotion-ends"].forEach((id) => { $(id).value = ""; });
  $("promotion-product").value = "";
  log(`新增促销成功 promotion_id=${resp.data.promotion_id}`);
  state.promotionFilters.page = 1;
  loadPromotions();
}

async function editPromotion(promotion) {
  if (!promotion) return;
  const products = await loadPromotionProductOptions(promotion.product_id);
  const values = await openAdminFormModal({
    title: `编辑促销 ${promotion.promotion_id}`,
    submitText: "保存",
    fields: [
      {
        name: "productId",
        label: "商品",
        type: "select",
        value: String(promotion.product_id || ""),
        options: products.map((product) => ({ value: product.product_id, label: `${product.name} (${product.product_id})` })),
      },
      { name: "discountValue", label: "限时价(分)", type: "number", min: 1, value: String(promotion.discount_value || 0), required: true },
      { name: "startsAt", label: "开始时间，留空立即生效", value: promotion.starts_at || "" },
      { name: "endsAt", label: "结束时间，留空长期有效", value: promotion.ends_at || "" },
    ],
    validate: (data) => {
      const productId = Number(data.productId || 0);
      const discountValue = Number(data.discountValue || 0);
      if (productId <= 0) return "请选择商品";
      if (discountValue <= 0) return "请输入有效的限时价";
      const product = state.productOptions.find((item) => Number(item.product_id) === productId);
      if (product && Number(product.sale_price_fen || 0) > 0 && discountValue > Number(product.sale_price_fen)) {
        return `限时价不能高于商品现价 ¥${formatPriceFen(product.sale_price_fen)}`;
      }
      const startsAt = data.startsAt.trim();
      const endsAt = data.endsAt.trim();
      if (startsAt && endsAt && new Date(endsAt).getTime() < new Date(startsAt).getTime()) return "结束时间必须晚于开始时间";
      return "";
    },
  });
  if (!values) return;
  const productId = values.productId;
  const discountValue = values.discountValue;
  const resp = await authed("/api/admin/promotions/update", {
    method: "POST",
    jsonBody: {
      promotion_id: Number(promotion.promotion_id),
      product_id: Number(productId || 0),
      discount_value: Number(discountValue || 0),
      threshold_amount: Number(promotion.threshold_amount || 0),
      starts_at: values.startsAt.trim(),
      ends_at: values.endsAt.trim(),
    },
  });
  if (!resp.ok || resp.data.error) {
    log(`编辑促销失败: ${promotionErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`促销已更新 promotion_id=${promotion.promotion_id}`);
  loadPromotions();
  if ($("promotion-detail-panel")?.style.display !== "none") loadPromotionDetail(promotion.promotion_id);
}

async function setPromotionStatus(promotionId, status) {
  const resp = await authed("/api/admin/promotions/update", {
    method: "POST",
    jsonBody: { promotion_id: Number(promotionId), status },
  });
  if (!resp.ok || resp.data.error) {
    log(`促销状态更新失败: ${promotionErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`促销状态已更新 promotion_id=${promotionId} status=${status}`);
  loadPromotions();
  if ($("promotion-detail-panel")?.style.display !== "none") loadPromotionDetail(promotionId);
}

async function loadPromotionDetail(promotionId) {
  const panel = $("promotion-detail-panel");
  if (!panel || !promotionId) return;
  panel.style.display = "";
  panel.innerHTML = '<div class="loading">促销详情加载中...</div>';
  const resp = await authed(`/api/admin/promotions/detail?promotion_id=${encodeURIComponent(String(promotionId))}`);
  if (!resp.ok || resp.data.error) {
    panel.innerHTML = `<div class="error">促销详情加载失败: ${escapeHtml(promotionErrorMessage(resp.data.error) || resp.status)}</div>`;
    return;
  }
  renderPromotionDetail(resp.data);
}

function renderPromotionDetail(promotion) {
  const promotionId = Number(promotion.promotion_id || 0);
  const productId = Number(promotion.product_id || 0);
  const isActive = Number(promotion.status || 0) === 1;
  $("promotion-detail-panel").innerHTML = `
    <h3>促销详情 ${promotionId}</h3>
    <div class="detail-grid">
      <div class="detail-item"><span class="detail-label">商品</span><span class="detail-value">${escapeHtml(promotion.product_name || "商品")} (${productId})</span></div>
      <div class="detail-item"><span class="detail-label">类型</span><span class="detail-value">${escapeHtml(promotionTypeText(promotion.type))}</span></div>
      <div class="detail-item"><span class="detail-label">商品售价</span><span class="detail-value">¥${formatPriceFen(promotion.sale_price_fen)}</span></div>
      <div class="detail-item"><span class="detail-label">限时价</span><span class="detail-value">¥${formatPriceFen(promotion.discount_value)}</span></div>
      <div class="detail-item"><span class="detail-label">开始时间</span><span class="detail-value">${escapeHtml(promotion.starts_at || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">结束时间</span><span class="detail-value">${escapeHtml(promotion.ends_at || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">时间态</span><span class="detail-value">${escapeHtml(promotion.effect_status_text || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">状态</span><span class="badge ${isActive ? "paid" : "closed"}">${isActive ? "启用" : "停用"}</span></div>
    </div>
    <div class="table-actions" style="margin-top:12px">
      <button class="button small ghost" data-promotion-detail-edit="${promotionId}">编辑</button>
      <button class="button small ${isActive ? "danger" : "primary"}" data-promotion-detail-status="${promotionId}" data-promotion-detail-next-status="${isActive ? 2 : 1}">${isActive ? "停用" : "启用"}</button>
      <button class="button small ghost" data-promotion-detail-product="${productId}">查看商品</button>
      <button class="button small ghost" data-promotion-detail-orders="${productId}">查看订单</button>
      <button class="button small ghost" data-promotion-detail-security="${promotionId}">安全日志</button>
    </div>`;
  document.querySelector("[data-promotion-detail-edit]")?.addEventListener("click", () => editPromotion(promotion));
  document.querySelector("[data-promotion-detail-status]")?.addEventListener("click", () => setPromotionStatus(promotionId, Number(document.querySelector("[data-promotion-detail-status]")?.getAttribute("data-promotion-detail-next-status"))));
  document.querySelector("[data-promotion-detail-product]")?.addEventListener("click", () => openProductDetailFromOrder(productId));
  document.querySelector("[data-promotion-detail-orders]")?.addEventListener("click", () => openProductOrders(productId));
  document.querySelector("[data-promotion-detail-security]")?.addEventListener("click", () => openSecurityKeyword(`promotion:${promotionId}`));
  log(`促销详情已加载 promotion_id=${promotionId}`);
}

async function loadUsers() {
  $("users-tbody").innerHTML = '<tr><td colspan="7" class="loading">加载中...</td></tr>';
  const params = new URLSearchParams({ page: String(state.userFilters.page || 1), page_size: String(state.userFilters.page_size || 20) });
  if (state.userFilters.status) params.set("status", state.userFilters.status);
  if (state.userFilters.role) params.set("role", state.userFilters.role);
  if (state.userFilters.keyword) params.set("keyword", state.userFilters.keyword);
  const resp = await authed(`/api/admin/users?${params}`);
  if (!resp.ok || resp.data.error) {
    $("users-tbody").innerHTML = `<tr><td colspan="7" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }

  const items = resp.data.items || resp.data.users || [];
  const total = resp.data.total || 0;
  $("users-tbody").innerHTML = items.length === 0 ? '<tr><td colspan="7" class="empty">暂无用户数据</td></tr>' : items.map((u) => {
    const userId = Number(u.user_id || u.id || 0);
    const status = Number(u.status || 1);
    const isActive = status === 1;
    const disablingSelf = isActive && userId === currentUserId();
    return `<tr>
        <td>${userId || "—"}</td>
        <td>${escapeHtml(u.display_name || u.username || "—")}</td>
        <td>${escapeHtml(u.phone || "—")}</td>
        <td>${escapeHtml(u.role || "user")}</td>
        <td>${isActive ? '<span class="badge paid">启用</span>' : '<span class="badge closed">禁用</span>'}</td>
        <td>${escapeHtml(u.created_at || u.create_time || "—")}</td>
        <td>
          <div class="table-actions">
            <button class="button small ghost" data-user-detail="${userId}">详情</button>
            <button class="button small ghost" data-user-orders="${userId}">订单</button>
            <button class="button small ghost" data-user-security="${userId}">日志</button>
            <button class="button small ${isActive ? "danger" : "primary"}" data-user-status="${userId}" data-user-next-status="${isActive ? 2 : 1}" ${disablingSelf ? "disabled" : ""}>
              ${isActive ? "禁用" : "启用"}
            </button>
          </div>
        </td>
      </tr>`;
  }).join("");
  document.querySelectorAll("[data-user-detail]").forEach((btn) => btn.addEventListener("click", () => loadUserDetail(btn.getAttribute("data-user-detail"))));
  document.querySelectorAll("[data-user-orders]").forEach((btn) => btn.addEventListener("click", () => openUserOrders(btn.getAttribute("data-user-orders"))));
  document.querySelectorAll("[data-user-security]").forEach((btn) => btn.addEventListener("click", () => openUserSecurity(btn.getAttribute("data-user-security"))));
  document.querySelectorAll("[data-user-status]").forEach((btn) => btn.addEventListener("click", () => setUserStatus(btn.getAttribute("data-user-status"), Number(btn.getAttribute("data-user-next-status")))));
  $("users-total").textContent = `共 ${total} 条`;
  updateUserPager(total);
  log(`用户列表已刷新 items=${items.length} total=${total}`);
}

function updateUserPager(total) {
  const page = Number(state.userFilters.page || 1);
  const pageSize = Number(state.userFilters.page_size || 20);
  const totalPages = Math.max(1, Math.ceil(Number(total || 0) / pageSize));
  if (page > totalPages) {
    state.userFilters.page = totalPages;
    loadUsers();
    return;
  }
  const pageLabel = $("users-page");
  if (pageLabel) pageLabel.textContent = `第 ${page} / ${totalPages} 页`;
  const sizeSelect = $("users-page-size");
  if (sizeSelect) sizeSelect.value = String(pageSize);
  const prev = $("users-prev");
  if (prev) prev.disabled = page <= 1;
  const next = $("users-next");
  if (next) next.disabled = page >= totalPages;
}

function openUserDetailFromOrder(userId) {
  state.userFilters.keyword = String(userId || "");
  state.userFilters.status = "";
  state.userFilters.role = "";
  state.userFilters.page = 1;
  const keywordFilter = $("user-keyword-filter");
  if (keywordFilter) keywordFilter.value = state.userFilters.keyword;
  const statusFilter = $("user-status-filter");
  if (statusFilter) statusFilter.value = "";
  const roleFilter = $("user-role-filter");
  if (roleFilter) roleFilter.value = "";
  switchTab("users");
  loadUserDetail(userId);
}

async function loadUserDetail(userId) {
  const panel = $("user-detail-panel");
  if (!panel || !userId) return;
  panel.style.display = "";
  panel.innerHTML = '<div class="loading">用户详情加载中...</div>';
  const resp = await authed(`/api/admin/users/detail?user_id=${encodeURIComponent(String(userId))}`);
  if (!resp.ok || resp.data.error) {
    panel.innerHTML = `<div class="error">用户详情加载失败: ${escapeHtml(userErrorMessage(resp.data.error) || resp.status)}</div>`;
    return;
  }
  renderUserDetail(resp.data);
}

function renderUserDetail(user) {
  const status = Number(user.status || 1);
  const isActive = status === 1;
  const userId = Number(user.user_id || user.id || 0);
  $("user-detail-panel").innerHTML = `
    <h3>用户详情 ${userId}</h3>
    <div class="detail-grid">
      <div class="detail-item"><span class="detail-label">昵称</span><span class="detail-value">${escapeHtml(user.display_name || user.username || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">手机号</span><span class="detail-value">${escapeHtml(user.phone || "—")}</span></div>
      <div class="detail-item"><span class="detail-label">角色</span><span class="detail-value">${escapeHtml(user.role || "user")}</span></div>
      <div class="detail-item"><span class="detail-label">状态</span><span class="badge ${isActive ? "paid" : "closed"}">${isActive ? "启用" : "禁用"}</span></div>
      <div class="detail-item"><span class="detail-label">注册时间</span><span class="detail-value">${escapeHtml(user.created_at || user.create_time || "—")}</span></div>
    </div>
    <div class="table-actions" style="margin-top:12px">
      <button class="button small ${isActive ? "danger" : "primary"}" data-user-detail-status="${userId}" data-user-detail-next-status="${isActive ? 2 : 1}" ${isActive && userId === currentUserId() ? "disabled" : ""}>${isActive ? "禁用" : "启用"}</button>
      <button class="button small ghost" data-user-detail-orders="${userId}">查看订单</button>
      <button class="button small ghost" data-user-detail-security="${userId}">安全日志</button>
    </div>`;
  document.querySelector("[data-user-detail-status]")?.addEventListener("click", () => setUserStatus(userId, Number(document.querySelector("[data-user-detail-status]")?.getAttribute("data-user-detail-next-status"))));
  document.querySelector("[data-user-detail-orders]")?.addEventListener("click", () => openUserOrders(userId));
  document.querySelector("[data-user-detail-security]")?.addEventListener("click", () => openUserSecurity(userId));
  log(`用户详情已加载 user_id=${userId}`);
}

async function setUserStatus(userId, status) {
  if (Number(status) !== 1 && Number(userId) === currentUserId()) {
    log("不能禁用当前登录的管理员账号");
    return;
  }
  const resp = await authed("/api/admin/users/status", {
    method: "POST",
    jsonBody: { user_id: Number(userId), status },
  });
  if (!resp.ok || resp.data.error) {
    log(`用户状态更新失败: ${userErrorMessage(resp.data.error) || resp.status}`);
    return;
  }
  log(`用户状态已更新 user_id=${userId} status=${resp.data.status_text || status}`);
  loadUsers();
  if ($("user-detail-panel")?.style.display !== "none") loadUserDetail(userId);
}

const SECURITY_EVENT_TYPES = [
  "login_password_success",
  "login_password_fail",
  "login_code_success",
  "login_code_fail",
  "send_code_success",
  "send_code_blocked",
  "refresh_success",
  "logout_success",
  "logout_all_success",
  "reset_password_success",
  "reset_password_fail",
  "register_success",
  "register_fail",
  "admin_user_enabled",
  "admin_user_disabled",
  "admin_user_status_update_failed",
  "admin_order_shipped",
  "admin_order_refunded",
  "admin_order_closed",
  "admin_product_created",
  "admin_product_updated",
  "admin_product_enabled",
  "admin_product_disabled",
  "admin_product_stock_adjusted",
  "admin_supplier_created",
  "admin_supplier_updated",
  "admin_supplier_enabled",
  "admin_supplier_disabled",
  "admin_promotion_created",
  "admin_promotion_updated",
  "admin_promotion_enabled",
  "admin_promotion_disabled",
];

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
    admin_order_closed: "管理员关闭订单",
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
  return map[type] || type || "—";
}

function securityResultBadge(result) {
  if (result === "success") return '<span class="badge paid">成功</span>';
  if (result === "blocked") return '<span class="badge pending">拦截</span>';
  if (result === "fail" || result === "failed") return '<span class="badge closed">失败</span>';
  return `<span class="badge">${escapeHtml(result || "—")}</span>`;
}

function securityResultMatches(actual, expected) {
  actual = String(actual || "").toLowerCase();
  expected = String(expected || "").toLowerCase();
  return actual === expected || ((actual === "fail" || actual === "failed") && (expected === "fail" || expected === "failed"));
}

function formatSecurityTime(seconds) {
  const value = Number(seconds || 0);
  if (!value) return "—";
  return new Date(value * 1000).toLocaleString();
}

function populateSecurityEventOptions(items) {
  const select = $("security-event-filter");
  if (!select) return;
  const current = select.value;
  const types = [...new Set([...SECURITY_EVENT_TYPES, ...items.map((item) => item.event_type).filter(Boolean)])];
  select.innerHTML = '<option value="">全部事件</option>' + types.map((type) => `<option value="${escapeHtml(type)}">${escapeHtml(securityEventText(type))}</option>`).join("");
  select.value = types.includes(current) ? current : "";
}

function securitySubjectCell(subject) {
  const value = String(subject || "");
  if (!value) return "—";
  return `<button class="link-button" data-security-subject="${escapeHtml(value)}">${escapeHtml(value)}</button>`;
}

function securityReasonText(subject) {
  const reason = String(subject || "").match(/\breason:([A-Za-z0-9_-]+)/)?.[1] || "";
  const map = {
    not_found: "对象不存在",
    product_not_found: "商品不存在",
    invalid_status: "状态不允许",
    status_changed: "状态已变化",
    not_paid_status: "订单未支付",
    invalid_price: "价格不合法",
    invalid_discount: "折扣价不合法",
    invalid_window: "时间窗口不合法",
    window_conflict: "活动时间冲突",
    active_supplier_not_found: "启用供应商不存在",
    has_active_products: "仍有关联启用商品",
    insufficient_or_missing_bucket: "库存不足或分桶不存在",
    invalid_user_id: "用户ID不合法",
    self_disable_blocked: "禁止禁用当前管理员",
    store_failed: "存储更新失败",
  };
  return map[reason] || reason || "—";
}

function openSecuritySubject(subject) {
  const value = String(subject || "");
  const order = value.match(/\border:([A-Za-z0-9_-]+)/);
  if (order?.[1]) {
    orderFilters = { status: "", order_id: order[1], user_id: "", product_id: "", product_name: "", created_from: "", created_to: "", page: 1, page_size: 20 };
    const orderFilter = $("order-id-filter");
    if (orderFilter) orderFilter.value = order[1];
    switchTab("orders");
    loadOrderDetail(order[1]);
    return;
  }
  const promotion = value.match(/\bpromotion:(\d+)/);
  if (promotion?.[1]) {
    switchTab("promotions");
    loadPromotionDetail(promotion[1]);
    return;
  }
  const supplier = value.match(/\bsupplier:(\d+)/);
  if (supplier?.[1]) {
    switchTab("suppliers");
    loadSupplierDetail(supplier[1]);
    return;
  }
  const product = value.match(/\bproduct:(\d+)/);
  if (product?.[1]) {
    openProductDetailFromOrder(product[1]);
    return;
  }
  const operator = value.match(/\boperator:(\d+)/);
  if (operator?.[1]) {
    openUserDetailFromOrder(operator[1]);
    return;
  }
  const user = value.match(/\btarget_user:(\d+)/);
  if (user?.[1]) openUserDetailFromOrder(user[1]);
}

function renderSecurityEvents() {
  const tbody = $("security-tbody");
  if (!tbody) return;
  const userId = Number($("security-user-filter")?.value || 0);
  const eventType = $("security-event-filter")?.value || "";
  const result = $("security-result-filter")?.value || "";
  const keyword = ($("security-keyword-filter")?.value || "").trim().toLowerCase();
  const items = state.securityItems.filter((item) => {
    if (userId && Number(item.user_id || 0) !== userId) return false;
    if (eventType && item.event_type !== eventType) return false;
    if (result && !securityResultMatches(item.result, result)) return false;
    if (keyword) {
      const source = `${item.user_id || ""} ${item.subject || ""} ${item.ip || ""} ${item.user_agent || ""} ${item.event_type || ""} ${item.result || ""}`.toLowerCase();
      if (!source.includes(keyword)) return false;
    }
    return true;
  });
  tbody.innerHTML = items.length === 0 ? '<tr><td colspan="8" class="empty">暂无安全日志</td></tr>' : items.map((item) => {
    const userId = Number(item.user_id || 0);
    return `<tr>
      <td>${escapeHtml(formatSecurityTime(item.created_at))}</td>
      <td>${escapeHtml(securityEventText(item.event_type))}</td>
      <td>${securityResultBadge(item.result)}</td>
      <td>${userId ? `<button class="link-button" data-security-user="${userId}">${userId}</button>` : "—"}</td>
      <td>${securitySubjectCell(item.subject)}</td>
      <td>${escapeHtml(item.ip || "—")}</td>
      <td>${escapeHtml(securityReasonText(item.subject))}</td>
      <td>${escapeHtml(item.user_agent || "—")}</td>
    </tr>`;
  }).join("");
  document.querySelectorAll("[data-security-user]").forEach((btn) => btn.addEventListener("click", () => openUserDetailFromOrder(btn.getAttribute("data-security-user"))));
  document.querySelectorAll("[data-security-subject]").forEach((btn) => btn.addEventListener("click", () => openSecuritySubject(btn.getAttribute("data-security-subject"))));
  $("security-total").textContent = `共 ${items.length}/${state.securityItems.length} 条`;
}

async function loadReconciliationIssues() {
  const tbody = $("reconciliation-tbody");
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="8" class="loading">加载中...</td></tr>';
  const resp = await authed("/api/admin/reconciliation/issues?page=1&page_size=50");
  if (!resp.ok || resp.data.error) {
    tbody.innerHTML = `<tr><td colspan="8" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }
  const items = resp.data.items || [];
  tbody.innerHTML = items.length === 0 ? '<tr><td colspan="8" class="empty">暂无对账风险</td></tr>' : items.map((item) => `
    <tr>
      <td>${item.id || item.issue_id || ""}</td>
      <td>${escapeHtml(item.issue_type || "")}</td>
      <td>${escapeHtml(item.order_id || "")}</td>
      <td>${escapeHtml(item.payment_order_id || "")}</td>
      <td>¥${formatPriceFen(item.expected_amount_fen || 0)}</td>
      <td>¥${formatPriceFen(item.actual_amount_fen || 0)}</td>
      <td>${item.severity || ""}</td>
      <td>${item.status || 0}</td>
    </tr>
  `).join("");
}

async function loadAdminEvents() {
  const tbody = $("events-tbody");
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="7" class="loading">加载中...</td></tr>';
  const resp = await authed("/api/admin/events?page=1&page_size=50");
  if (!resp.ok || resp.data.error) {
    tbody.innerHTML = `<tr><td colspan="7" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }
  const items = resp.data.items || [];
  tbody.innerHTML = items.length === 0 ? '<tr><td colspan="7" class="empty">暂无待处理事件</td></tr>' : items.map((item) => `
    <tr>
      <td>${item.id || ""}</td>
      <td>${escapeHtml(item.event_type || "")}</td>
      <td>${escapeHtml(item.aggregate_id || "")}</td>
      <td>${item.status || 0}</td>
      <td>${item.retry_count || 0}</td>
      <td>${escapeHtml(item.last_error || "")}</td>
      <td>${escapeHtml(item.create_time || "")}</td>
    </tr>
  `).join("");
}

async function loadSecurityEvents() {
  const tbody = $("security-tbody");
  if (!tbody) return;
  tbody.innerHTML = '<tr><td colspan="8" class="loading">加载中...</td></tr>';
  const params = new URLSearchParams({ limit: "100" });
  const userId = $("security-user-filter")?.value || "";
  const eventType = $("security-event-filter")?.value || "";
  const result = $("security-result-filter")?.value || "";
  const keyword = $("security-keyword-filter")?.value.trim() || "";
  if (userId) params.set("user_id", userId);
  if (eventType) params.set("event_type", eventType);
  if (result) params.set("result", result);
  if (keyword) params.set("keyword", keyword);
  const resp = await authed(`/api/admin/security/events/recent?${params}`);
  if (!resp.ok || resp.data.error) {
    tbody.innerHTML = `<tr><td colspan="8" class="error">加载失败: ${escapeHtml(resp.data.error || resp.status)}</td></tr>`;
    return;
  }
  const items = resp.data.items || [];
  state.securityItems = items;
  populateSecurityEventOptions(items);
  renderSecurityEvents();
  log(`安全日志已刷新 items=${items.length}`);
}

document.querySelectorAll("[data-workspace]").forEach((btn) =>
  btn.addEventListener("click", () => setWorkspace(btn.getAttribute("data-workspace"), { switchDefault: true }))
);

document.querySelectorAll("[data-tab]").forEach((btn) =>
  btn.addEventListener("click", () => switchTab(btn.getAttribute("data-tab")))
);

setWorkspace(state.workspace);

$("order-filter-apply")?.addEventListener("click", () => {
  orderFilters.status = $("order-status-filter")?.value || "";
  orderFilters.order_id = $("order-id-filter")?.value.trim() || "";
  orderFilters.user_id = $("order-user-filter")?.value || "";
  orderFilters.product_id = $("order-product-filter")?.value || "";
  orderFilters.product_name = $("order-product-name-filter")?.value.trim() || "";
  orderFilters.created_from = $("order-from-filter")?.value || "";
  orderFilters.created_to = $("order-to-filter")?.value || "";
  orderFilters.page = 1;
  loadOrders();
});

$("orders-prev")?.addEventListener("click", () => {
  if (orderFilters.page <= 1) return;
  orderFilters.page -= 1;
  loadOrders();
});

$("orders-next")?.addEventListener("click", () => {
  orderFilters.page += 1;
  loadOrders();
});

$("orders-page-size")?.addEventListener("change", () => {
  orderFilters.page_size = Number($("orders-page-size")?.value || 20);
  orderFilters.page = 1;
  loadOrders();
});

$("refund-filter-apply")?.addEventListener("click", () => {
  state.refundFilters.status = $("refund-status-filter")?.value ?? "0";
  state.refundFilters.orderId = $("refund-order-filter")?.value.trim() || "";
  state.refundFilters.userId = $("refund-user-filter")?.value || "";
  state.refundFilters.page = 1;
  loadRefunds();
});

$("refund-refresh")?.addEventListener("click", () => loadRefunds());

$("product-filter-apply")?.addEventListener("click", () => {
  state.productFilters.status = $("product-status-filter")?.value || "";
  state.productFilters.keyword = $("product-keyword-filter")?.value.trim() || "";
  state.productFilters.productId = $("product-id-filter")?.value || "";
  state.productFilters.supplierId = $("product-supplier-filter")?.value || "";
  state.productFilters.promotionStatus = $("product-promotion-filter")?.value || "";
  state.productFilters.stockStatus = $("product-stock-filter")?.value || "";
  state.productFilters.page = 1;
  loadProducts();
});

$("products-prev")?.addEventListener("click", () => {
  if (state.productFilters.page <= 1) return;
  state.productFilters.page -= 1;
  loadProducts();
});

$("products-next")?.addEventListener("click", () => {
  state.productFilters.page += 1;
  loadProducts();
});

$("products-page-size")?.addEventListener("change", () => {
  state.productFilters.page_size = Number($("products-page-size")?.value || 20);
  state.productFilters.page = 1;
  loadProducts();
});

$("product-create")?.addEventListener("click", createProduct);

$("supplier-filter-apply")?.addEventListener("click", () => {
  state.supplierFilters.status = $("supplier-status-filter")?.value || "";
  state.supplierFilters.keyword = $("supplier-keyword-filter")?.value.trim() || "";
  state.supplierFilters.page = 1;
  loadSuppliers();
});

$("suppliers-prev")?.addEventListener("click", () => {
  if (state.supplierFilters.page <= 1) return;
  state.supplierFilters.page -= 1;
  loadSuppliers();
});

$("suppliers-next")?.addEventListener("click", () => {
  state.supplierFilters.page += 1;
  loadSuppliers();
});

$("suppliers-page-size")?.addEventListener("change", () => {
  state.supplierFilters.page_size = Number($("suppliers-page-size")?.value || 20);
  state.supplierFilters.page = 1;
  loadSuppliers();
});

$("supplier-create")?.addEventListener("click", createSupplier);

$("promotion-filter-apply")?.addEventListener("click", () => {
  state.promotionFilters.status = $("promotion-status-filter")?.value || "";
  state.promotionFilters.keyword = $("promotion-keyword-filter")?.value.trim() || "";
  state.promotionFilters.productId = $("promotion-product-filter")?.value || "";
  state.promotionFilters.effectStatus = $("promotion-effect-filter")?.value || "";
  state.promotionFilters.page = 1;
  loadPromotions();
});

$("promotions-prev")?.addEventListener("click", () => {
  if (state.promotionFilters.page <= 1) return;
  state.promotionFilters.page -= 1;
  loadPromotions();
});

$("promotions-next")?.addEventListener("click", () => {
  state.promotionFilters.page += 1;
  loadPromotions();
});

$("promotions-page-size")?.addEventListener("change", () => {
  state.promotionFilters.page_size = Number($("promotions-page-size")?.value || 20);
  state.promotionFilters.page = 1;
  loadPromotions();
});

$("promotion-create")?.addEventListener("click", createPromotion);

$("user-filter-apply")?.addEventListener("click", () => {
  state.userFilters.status = $("user-status-filter")?.value || "";
  state.userFilters.role = $("user-role-filter")?.value || "";
  state.userFilters.keyword = $("user-keyword-filter")?.value.trim() || "";
  state.userFilters.page = 1;
  loadUsers();
});

$("users-prev")?.addEventListener("click", () => {
  if (state.userFilters.page <= 1) return;
  state.userFilters.page -= 1;
  loadUsers();
});

$("users-next")?.addEventListener("click", () => {
  state.userFilters.page += 1;
  loadUsers();
});

$("users-page-size")?.addEventListener("change", () => {
  state.userFilters.page_size = Number($("users-page-size")?.value || 20);
  state.userFilters.page = 1;
  loadUsers();
});

$("security-refresh")?.addEventListener("click", loadSecurityEvents);
$("security-filter-apply")?.addEventListener("click", loadSecurityEvents);
$("reconciliation-refresh")?.addEventListener("click", loadReconciliationIssues);
$("events-refresh")?.addEventListener("click", loadAdminEvents);
$("merchant-applications-refresh")?.addEventListener("click", loadMerchantApplications);

$("console-toggle")?.addEventListener("click", () => {
  $("console-log")?.classList.toggle("hidden");
});

if (!state.token) {
  $("login-hint").style.display = "";
  $("admin-main").style.display = "none";
} else {
  $("login-hint").style.display = "none";
  $("admin-main").style.display = "";
  switchTab("dashboard");
}

log("后台管理已就绪");
