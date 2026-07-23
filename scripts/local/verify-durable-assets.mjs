#!/usr/bin/env node

import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { basename, extname, resolve } from 'node:path';

const baseURL = process.env.FLASH_MALL_BASE_URL || 'http://127.0.0.1:8889';
const imagePath = resolve(process.argv[2] || '');
const productID = Number(process.argv[3] || 106);

if (!process.argv[2] || !Number.isSafeInteger(productID) || productID <= 0) {
  throw new Error('usage: node scripts/local/verify-durable-assets.mjs IMAGE_PATH [PRODUCT_ID]');
}

async function jsonRequest(path, options = {}) {
  const headers = new Headers(options.headers || {});
  let body = options.body;
  if (options.json !== undefined) {
    headers.set('Content-Type', 'application/json');
    body = JSON.stringify(options.json);
  }
  const response = await fetch(baseURL + path, { ...options, headers, body });
  const text = await response.text();
  let payload;
  try {
    payload = JSON.parse(text);
  } catch {
    throw new Error(`${options.method || 'GET'} ${path} returned ${response.status}: ${text}`);
  }
  if (!response.ok || (payload.code && payload.code !== 'OK')) {
    throw new Error(`${options.method || 'GET'} ${path} failed: ${JSON.stringify(payload)}`);
  }
  return payload.data ?? payload;
}

async function login(phone, password) {
  const data = await jsonRequest('/api/auth/login', {
    method: 'POST',
    json: { phone, password },
  });
  if (!data.access_token) throw new Error(`login did not return a token for ${phone}`);
  return data.access_token;
}

function bearer(token) {
  return { Authorization: `Bearer ${token}` };
}

const image = await readFile(imagePath);
const extension = extname(imagePath).toLowerCase();
const mimeTypes = { '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.png': 'image/png', '.webp': 'image/webp', '.gif': 'image/gif' };
const mimeType = mimeTypes[extension];
if (!mimeType) throw new Error(`unsupported image extension: ${extension}`);
const expectedHash = createHash('sha256').update(image).digest('hex');

const adminToken = await login('13800000002', 'admin123');
const original = await jsonRequest(`/api/admin/products/detail?product_id=${productID}`, {
  headers: bearer(adminToken),
});

const form = new FormData();
form.set('image', new Blob([image], { type: mimeType }), basename(imagePath));
const uploadResponse = await jsonRequest('/api/admin/products/image', {
  method: 'POST',
  headers: bearer(adminToken),
  body: form,
});
const imageURL = uploadResponse.image_url;
if (!imageURL || !imageURL.includes(expectedHash)) {
  throw new Error(`content-addressed URL mismatch: expected ${expectedHash}, got ${imageURL}`);
}

const verificationName = `持久化验收商品🧥-${Date.now()}`;
const updateProduct = (overrides) => jsonRequest('/api/admin/products/update', {
  method: 'POST',
  headers: bearer(adminToken),
  json: {
    product_id: original.product_id,
    name: original.name,
    image_url: imageURL,
    origin_price_fen: original.origin_price_fen,
    sale_price_fen: original.sale_price_fen,
    supplier_id: original.supplier_id,
    status: original.status,
    ...overrides,
  },
});

let order;
try {
  await updateProduct({ name: verificationName, status: 1 });
  const userToken = await login('13800000001', 'flashmall123');
  order = await jsonRequest('/api/order/create', {
    method: 'POST',
    headers: bearer(userToken),
    json: {
      request_id: `durable-assets-${Date.now()}`,
      user_id: 0,
      product_id: productID,
      amount: 1,
    },
  });
  const detail = await jsonRequest(`/api/order/detail?order_id=${encodeURIComponent(order.order_id)}`, {
    headers: bearer(userToken),
  });
  if (detail.product_name !== verificationName) {
    throw new Error(`UTF-8 snapshot mismatch: ${JSON.stringify(detail.product_name)}`);
  }
  if (detail.image_url !== imageURL) {
    throw new Error(`order image snapshot mismatch: expected ${imageURL}, got ${detail.image_url}`);
  }
  const served = Buffer.from(await (await fetch(baseURL + imageURL)).arrayBuffer());
  const servedHash = createHash('sha256').update(served).digest('hex');
  if (servedHash !== expectedHash) {
    throw new Error(`served image hash mismatch: expected ${expectedHash}, got ${servedHash}`);
  }
} finally {
  await updateProduct({});
}

console.log(JSON.stringify({
  status: 'ok',
  product_id: productID,
  order_id: order.order_id,
  product_name: verificationName,
  image_url: imageURL,
  sha256: expectedHash,
}, null, 2));
