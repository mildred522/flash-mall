import { $, log } from "./state.js";

export async function health() {
  $("health-summary").textContent = "检查中";
  $("health-detail").textContent = "正在探测依赖";
  $("health-dot").className = "dot";
  try {
    const response = await fetch("/api/system/health", { credentials: "same-origin" });
    const data = await response.json();
    const ok = !!data.overall;
    const failed = (data.dependencies || []).filter((dep) => !dep.ok).map((dep) => dep.name);
    $("health-summary").textContent = ok ? "整体正常" : "部分降级";
    $("health-detail").textContent = failed.length ? failed.join(", ") : "核心依赖全部可达";
    $("health-dot").className = `dot ${ok ? "ok" : "bad"}`;
    log(`健康检查完成 overall=${ok} detail=${$("health-detail").textContent}`);
  } catch (error) {
    $("health-summary").textContent = "检查失败";
    $("health-detail").textContent = error.message || "未知错误";
    $("health-dot").className = "dot bad";
    log(`健康检查失败 error=${$("health-detail").textContent}`);
  }
}
