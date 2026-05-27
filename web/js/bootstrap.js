import { state, $, authModal, log } from "./state.js";
import { api } from "./api-client.js";
import {
  saveAuth, clearAuth, updateUI, openAuth, closeAuth, tab,
  passwordLogin, register, codeLogin, resetPassword,
  sendCode, logout, logoutAll, me, authed,
} from "./auth.js";
import { loadCatalog } from "./catalog.js";
import { burst, order } from "./order.js";
import { loadSecurityEvents } from "./security.js";
import { health } from "./health.js";
import { timerTick } from "./timer.js";
import { collapse } from "./console.js";

async function bootstrap() {
  updateUI();
  await loadCatalog();
  if (state.token && (await me())) {
    await loadSecurityEvents();
    log(`当前用户已恢复 ${state.user.display_name}`);
    return;
  }
  if (await (async () => {
    const response = await api("/api/auth/refresh", { method: "POST" });
    if (!response.ok) return false;
    saveAuth(response.data);
    log("access token 已刷新");
    return true;
  })()) {
    await me();
    await loadSecurityEvents();
  }
}

// Wire up DOM event listeners
document.querySelectorAll("[data-auth-tab]").forEach((button) =>
  button.addEventListener("click", () => tab(button.getAttribute("data-auth-tab")))
);
document.querySelectorAll("[data-open-auth]").forEach((button) =>
  button.addEventListener("click", () => openAuth(button.getAttribute("data-open-auth") || "password"))
);
$("open-auth").addEventListener("click", () => openAuth("password"));
$("console-login-action").addEventListener("click", () => openAuth("password"));
$("close-auth").addEventListener("click", closeAuth);
authModal.addEventListener("click", (event) => {
  if (event.target === authModal) closeAuth();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeAuth();
});
$("password-login-action").addEventListener("click", passwordLogin);
$("register-action").addEventListener("click", register);
$("code-login-action").addEventListener("click", codeLogin);
$("send-code-action").addEventListener("click", () => sendCode("register", $("register-phone").value.trim()));
$("send-login-code-action").addEventListener("click", () => sendCode("login", $("code-login-phone").value.trim()));
$("reset-password-action").addEventListener("click", () => sendCode("reset-password", $("reset-phone").value.trim()));
$("submit-reset-password").addEventListener("click", resetPassword);
$("console-logout-action").addEventListener("click", logout);
$("console-logout-all-action").addEventListener("click", logoutAll);
$("console-health-action").addEventListener("click", health);
$("console-buy-action").addEventListener("click", () => order(100, 1, "console-buy"));
$("console-burst-action").addEventListener("click", burst);
$("console-toggle").addEventListener("click", () => collapse(!state.consoleCollapsed));

// Initialize
import { renderProducts } from "./catalog.js";
renderProducts([]);
updateUI();
collapse(state.consoleCollapsed);
timerTick();
setInterval(timerTick, 1000);
bootstrap();
log("商城首页已就绪");
