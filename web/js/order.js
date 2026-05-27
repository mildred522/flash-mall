import { state, $, log, rid } from "./state.js";
import { authed, openAuth } from "./auth.js";

export { rid };

export function latest(orderId, requestId) {
  if (orderId) $("latest-order").textContent = orderId;
  if (requestId) $("latest-request").textContent = requestId;
}

export async function order(productId, amount, scene) {
  if (!state.token) {
    openAuth("password");
    log("下单被拦截：请先登录");
    return null;
  }
  const requestId = rid();
  const payload = { request_id: requestId, user_id: 999999, product_id: productId, amount };
  const t0 = performance.now();
  const response = await authed("/api/order/create", { method: "POST", jsonBody: payload });
  const cost = (performance.now() - t0).toFixed(1);
  latest(response.ok ? response.data.order_id : "失败", requestId);
  if (response.ok) {
    log(`下单成功 scene=${scene} product=${productId} qty=${amount} req=${requestId} order=${response.data.order_id} latency=${cost}ms`);
  } else {
    if (response.status === 401) openAuth("password");
    log(`下单失败 scene=${scene} status=${response.status} req=${requestId} body=${JSON.stringify(response.data)} latency=${cost}ms`);
  }
  return response;
}

export async function burst() {
  if (!state.token) {
    openAuth("password");
    return;
  }
  log("连续下单开始 rounds=3 qty=1");
  for (let i = 0; i < 3; i += 1) {
    await order(100, 1, `burst-${i + 1}/3`);
    if (i < 2) await new Promise((resolve) => setTimeout(resolve, 140));
  }
  log("连续下单结束 rounds=3");
}
