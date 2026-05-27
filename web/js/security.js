import { state, $ } from "./state.js";
import { authed } from "./auth.js";

export function summarizeSecurity(items, matcher) {
  const hit = items.find((item) => matcher(item));
  if (!hit) return "normal";
  return `${hit.event_type}:${hit.result}`;
}

export function renderSecurityEvents(items) {
  const lines = (items || []).map((item) => {
    const when = item.created_at ? new Date(item.created_at * 1000).toLocaleTimeString() : "--:--:--";
    const who = item.subject || `user:${item.user_id || 0}`;
    const ip = item.ip || "-";
    return `[${when}] ${item.event_type} ${item.result} ${who} ${ip}`;
  });
  $("security-events").textContent = lines.length ? lines.join("\n") : "No recent security events yet.";
  $("login-risk-summary").textContent = summarizeSecurity(items, (item) =>
    String(item.event_type || "").includes("login")
  );
  $("code-risk-summary").textContent = summarizeSecurity(items, (item) => {
    const eventType = String(item.event_type || "");
    return eventType.includes("code") || eventType.includes("password");
  });
}

export async function loadSecurityEvents() {
  if (!state.token) {
    renderSecurityEvents([]);
    return false;
  }
  const response = await authed("/api/auth/security/events/recent");
  if (!response.ok) {
    renderSecurityEvents([]);
    return false;
  }
  renderSecurityEvents(response.data.items || []);
  return true;
}
