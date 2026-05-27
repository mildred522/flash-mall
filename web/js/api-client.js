import { state } from "./state.js";

export async function api(path, options = {}) {
  const headers = Object.assign({}, options.headers || {});
  if (options.jsonBody !== undefined) headers["Content-Type"] = "application/json";
  if (options.auth && state.token) headers.Authorization = `Bearer ${state.token}`;
  const response = await fetch(path, {
    method: options.method || "GET",
    headers,
    body: options.jsonBody !== undefined ? JSON.stringify(options.jsonBody) : options.body,
    credentials: "same-origin",
  });
  const text = await response.text();
  let data = {};
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = { raw: text };
  }
  return { ok: response.ok, status: response.status, data };
}
