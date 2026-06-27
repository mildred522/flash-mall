import { state, $, authModal, log, err } from "./state.js";
import { api } from "./api-client.js";
import { switchCartOwner } from "./cart.js";

export function setUser(payload) {
  state.user = {
    user_id: payload.user_id,
    display_name: payload.display_name || `用户 ${payload.user_id}`,
    phone: payload.phone || "",
  };
  localStorage.setItem("fm_user", JSON.stringify(state.user));
  switchCartOwner();
}

export function saveAuth(payload) {
  state.token = payload.access_token || "";
  if (payload.user_id) setUser(payload);
  localStorage.setItem("fm_token", state.token);
  updateUI();
}

export function clearAuth() {
  state.token = "";
  state.user = null;
  localStorage.removeItem("fm_token");
  localStorage.removeItem("fm_user");
  switchCartOwner();
  updateUI();
  renderSecurityEvents([]);
}

export function updateUI() {
  const authed = !!state.token && !!state.user;
  $("auth-chip").textContent = authed ? `${state.user.display_name} 已登录` : "游客模式";
  $("console-auth-text").textContent = authed ? state.user.display_name : "游客模式";
  $("console-token").textContent = authed ? `${state.token.slice(0, 24)}...` : "暂无令牌";
  $("current-user-name").textContent = authed ? `${state.user.display_name} / ${state.user.phone}` : "暂无";
  $("auth-dot").className = `dot ${authed ? "ok" : "bad"}`;
  $("open-auth").textContent = authed ? "切换账号" : "登录 / 注册";
}

export function tab(name) {
  document.querySelectorAll("[data-auth-tab]").forEach((button) =>
    button.classList.toggle("active", button.getAttribute("data-auth-tab") === name)
  );
  document.querySelectorAll(".panel").forEach((panel) =>
    panel.classList.toggle("active", panel.id === `auth-panel-${name}`)
  );
  const copy = {
    password: ["登录", "登录后下单、领券、查看当前状态都更顺手。"],
    register: ["注册", "先注册，再慢慢挑今天想买的。"],
    code: ["快捷登录", "验证码登录更适合直接演示完整链路。"],
    reset: ["找回密码", "先验证手机号，再设置一个新的登录密码。"],
  };
  $("auth-title").textContent = copy[name][0];
  $("auth-subtitle").textContent = copy[name][1];
  err("");
}

export function openAuth(name = "password") {
  tab(name);
  authModal.classList.add("open");
  authModal.setAttribute("aria-hidden", "false");
}

export function closeAuth() {
  authModal.classList.remove("open");
  authModal.setAttribute("aria-hidden", "true");
  err("");
}

export async function refresh() {
  const response = await api("/api/auth/refresh", { method: "POST" });
  if (!response.ok) {
    clearAuth();
    return false;
  }
  saveAuth(response.data);
  log("access token 已刷新");
  return true;
}

export async function authed(path, options = {}) {
  let response = await api(path, Object.assign({}, options, { auth: true }));
  if (response.status === 401 && (await refresh())) {
    response = await api(path, Object.assign({}, options, { auth: true }));
  }
  return response;
}

export async function me() {
  if (!state.token) return false;
  const response = await authed("/api/auth/me");
  if (!response.ok) {
    clearAuth();
    return false;
  }
  setUser(response.data);
  updateUI();
  return true;
}

export async function sendCode(scene, phone) {
  const path = scene === "reset-password" ? "/api/auth/password/forgot" : "/api/auth/code/send";
  const payload = scene === "reset-password" ? { phone } : { phone, scene };
  const response = await api(path, { method: "POST", jsonBody: payload });
  if (!response.ok) {
    const message = response.data.message || response.data.msg || `发送验证码失败（${response.status}）`;
    err(message);
    log(`发码失败 scene=${scene} phone=${phone} body=${JSON.stringify(response.data)}`);
    return false;
  }
  err("");
  log(`验证码已发送 scene=${scene} phone=${phone} debug_code=${response.data.debug_code || "-"}`);
  return true;
}

export async function passwordLogin() {
  const phone = $("login-phone").value.trim();
  const password = $("login-password").value;
  const response = await api("/api/auth/login", { method: "POST", jsonBody: { phone, password, device_type: "web" } });
  if (!response.ok) {
    const message = response.data.message || response.data.msg || `登录失败（${response.status}）`;
    err(message);
    log(`密码登录失败 phone=${phone} body=${JSON.stringify(response.data)}`);
    return;
  }
  saveAuth(response.data);
  closeAuth();
  await loadSecurityEvents();
  log(`密码登录成功 user_id=${response.data.user_id}`);
}

export async function register() {
  const payload = {
    display_name: $("register-name").value.trim(),
    phone: $("register-phone").value.trim(),
    password: $("register-password").value,
    code: $("register-code").value.trim(),
    device_type: "web",
  };
  const response = await api("/api/auth/register", { method: "POST", jsonBody: payload });
  if (!response.ok) {
    const message = response.data.message || response.data.msg || `注册失败（${response.status}）`;
    err(message);
    log(`注册失败 phone=${payload.phone} body=${JSON.stringify(response.data)}`);
    return;
  }
  saveAuth(response.data);
  closeAuth();
  await loadSecurityEvents();
  log(`注册成功 user_id=${response.data.user_id} phone=${payload.phone}`);
}

export async function codeLogin() {
  const payload = {
    phone: $("code-login-phone").value.trim(),
    code: $("code-login-code").value.trim(),
    device_type: "web",
  };
  const response = await api("/api/auth/login/code", { method: "POST", jsonBody: payload });
  if (!response.ok) {
    const message = response.data.message || response.data.msg || `验证码登录失败（${response.status}）`;
    err(message);
    log(`验证码登录失败 phone=${payload.phone} body=${JSON.stringify(response.data)}`);
    return;
  }
  saveAuth(response.data);
  closeAuth();
  await loadSecurityEvents();
  log(`验证码登录成功 user_id=${response.data.user_id}`);
}

export async function resetPassword() {
  const payload = {
    phone: $("reset-phone").value.trim(),
    code: $("reset-code").value.trim(),
    new_password: $("reset-password").value,
  };
  const response = await api("/api/auth/password/reset", { method: "POST", jsonBody: payload });
  if (!response.ok) {
    const message = response.data.message || response.data.msg || `重置密码失败（${response.status}）`;
    err(message);
    log(`重置密码失败 phone=${payload.phone} body=${JSON.stringify(response.data)}`);
    return;
  }
  clearAuth();
  tab("password");
  $("login-phone").value = payload.phone;
  $("login-password").value = payload.new_password;
  log(`重置密码成功 phone=${payload.phone}`);
}

export async function logout() {
  if (!state.token) {
    clearAuth();
    return;
  }
  const response = await api("/api/auth/logout", { method: "POST", auth: true });
  if (!response.ok) log(`退出登录失败 body=${JSON.stringify(response.data)}`);
  clearAuth();
  log("退出登录完成");
}

export async function logoutAll() {
  if (!state.token) {
    clearAuth();
    return;
  }
  const response = await api("/api/auth/logout-all", { method: "POST", auth: true });
  if (!response.ok) log(`全部下线失败 body=${JSON.stringify(response.data)}`);
  clearAuth();
  log("全部下线完成");
}

// Import loadSecurityEvents to avoid circular dep at call site
import { loadSecurityEvents, renderSecurityEvents } from "./security.js";
