const body = document.getElementById("health-body");
const overall = document.getElementById("overall");
const logBox = document.getElementById("debug-log");

function log(message) {
  logBox.textContent = `[${new Date().toLocaleTimeString()}] ${message}\n${logBox.textContent}`;
}

async function runHealth() {
  overall.textContent = "检查中";
  body.innerHTML = "";
  try {
    const response = await fetch("/api/system/health", { credentials: "same-origin" });
    const data = await response.json();
    overall.textContent = data.overall ? "整体正常" : "部分降级";
    overall.className = `pill ${data.overall ? "ok" : "bad"}`;
    (data.dependencies || []).forEach((dep) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `<td>${dep.name}</td><td class="${dep.ok ? "ok" : "bad"}">${dep.ok ? "OK" : "FAIL"}</td><td>${dep.detail}</td>`;
      body.appendChild(tr);
    });
    log(`健康检查完成 overall=${data.overall}`);
  } catch (error) {
    overall.textContent = "检查失败";
    overall.className = "pill bad";
    log(`健康检查失败 error=${error.message || "unknown"}`);
  }
}

document.getElementById("run-health").addEventListener("click", runHealth);
