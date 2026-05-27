export const state = {
  token: localStorage.getItem("fm_token") || "",
  user: JSON.parse(localStorage.getItem("fm_user") || "null"),
  consoleCollapsed: window.matchMedia("(max-width:960px)").matches,
  timerSeconds: 8108,
  products: [],
};

export const $ = (id) => document.getElementById(id);
export const authModal = $("auth-modal");
export const logBox = $("console-log");

export function log(message) {
  logBox.textContent = `[${new Date().toLocaleTimeString()}] ${message}\n${logBox.textContent}`;
}

export function err(message) {
  $("auth-error").textContent = message || "";
}

export function rid() {
  return `req-shop-${Date.now()}-${Math.floor(Math.random() * 100000)}`;
}
