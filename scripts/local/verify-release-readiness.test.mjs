import assert from 'node:assert/strict';
import test from 'node:test';

import {
  assertLocalReadinessTarget,
  verifyReleaseReadiness,
} from './verify-release-readiness.mjs';

const accounts = {
  '13800000001': { user_id: 1001, token: 'user-token' },
  '13800000002': { user_id: 1002, token: 'admin-token' },
  '13800001101': { user_id: 1101, token: 'merchant-1101-token' },
  '13800001102': { user_id: 1102, token: 'merchant-1102-token' },
};

function json(value, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

function fixtureFetch({ brokenImage = false, jaegerServices } = {}) {
  return async (input, init = {}) => {
    const url = new URL(input);
    const path = `${url.pathname}${url.search}`;
    if (['/shop', '/admin', '/merchant'].includes(url.pathname)) {
      return new Response('<!doctype html><title>Flash Mall</title>', {
        headers: { 'content-type': 'text/html; charset=utf-8' },
      });
    }
    if (url.pathname === '/live') return new Response('ok');
    if (url.pathname === '/ready' || url.pathname === '/api/system/health') {
      return json({ code: 'OK', data: { status: 'ok' } });
    }
    if (url.pathname === '/api/shop/catalog') {
      return json({ code: 'OK', data: { items: [
        { product_id: 201, image_url: '/products/a.webp', merchant_id: 1101 },
        { product_id: 211, image_url: '/products/b.webp', merchant_id: 1102 },
      ] } });
    }
    if (url.pathname === '/products/a.webp' || url.pathname === '/products/b.webp') {
      if (brokenImage && url.pathname === '/products/b.webp') return new Response('', { status: 404 });
      return new Response('image', { headers: { 'content-type': 'image/webp' } });
    }
    if (url.pathname === '/api/shop/stores/detail') {
      const merchantID = Number(url.searchParams.get('merchant_id'));
      return json({ code: 'OK', data: {
        merchant_id: merchantID,
        status: 1,
        logo_url: `/products/${merchantID}-logo.svg`,
        banner_url: `/products/${merchantID}-banner.webp`,
      } });
    }
    if (/^\/products\/110[12]-(logo\.svg|banner\.webp)$/.test(url.pathname)) {
      return new Response('image', { headers: { 'content-type': 'image/svg+xml' } });
    }
    if (url.pathname === '/api/auth/login' && init.method === 'POST') {
      const account = accounts[JSON.parse(init.body).phone];
      return account ? json({ ...account, access_token: account.token }) : json({}, 401);
    }
    const authorization = new Headers(init.headers).get('authorization');
    if (url.pathname === '/api/orders' && authorization === 'Bearer user-token') {
      return json({ code: 'OK', data: { items: [] } });
    }
    if (url.pathname === '/api/admin/dashboard/stats' && authorization === 'Bearer admin-token') {
      return json({ code: 'OK', data: { total_orders: 0 } });
    }
    if (url.pathname === '/api/merchant/me') {
      const merchantID = authorization?.includes('1101') ? 1101 : 1102;
      return json({ code: 'OK', data: { items: [{ merchant_id: merchantID }] } });
    }
    if (url.pathname === '/api/merchant/products' && authorization?.startsWith('Bearer merchant-')) {
      return json({ code: 'OK', data: { items: [] } });
    }
    if (path === '/prom/api/v1/targets') {
      return json({ status: 'success', data: { activeTargets: [
        'hertz-gateway', 'order-rpc', 'product-rpc', 'inventory-kitex',
      ].map((job) => ({ labels: { job }, health: 'up' })) } });
    }
    if (path === '/grafana/api/health') return json({ database: 'ok' });
    if (path === '/grafana/api/search?type=dash-db') {
      return json(Array.from({ length: 5 }, (_, index) => ({ uid: `dashboard-${index}` })));
    }
    if (path === '/jaeger/api/services') {
      return json({ data: jaegerServices ?? ['hertz-gateway', 'order-rpc', 'inventory-kitex'] });
    }
    if (path === '/rabbit/api/overview') return json({ rabbitmq_version: '3.13' });
    return new Response('', { status: 404 });
  };
}

test('assertLocalReadinessTarget rejects remote or credential-bearing URLs', () => {
  for (const value of ['https://mall.example.com', 'http://user:pass@127.0.0.1:8889']) {
    assert.throws(() => assertLocalReadinessTarget(value));
  }
  assert.equal(assertLocalReadinessTarget('http://127.0.0.1:8889').origin, 'http://127.0.0.1:8889');
});

test('verifyReleaseReadiness covers business roles, assets, and observability', async () => {
  const report = await verifyReleaseReadiness({
    baseURL: 'http://127.0.0.1:8889',
    prometheusURL: 'http://127.0.0.1:8889/prom',
    grafanaURL: 'http://127.0.0.1:8889/grafana',
    jaegerURL: 'http://127.0.0.1:8889/jaeger',
    rabbitmqURL: 'http://127.0.0.1:8889/rabbit',
    fetchImpl: fixtureFetch(),
  });

  assert.equal(report.passed, true, JSON.stringify(report, null, 2));
  assert.equal(report.failed, 0);
  assert.equal(JSON.stringify(report).includes('user-token'), false);
  assert.equal(JSON.stringify(report).includes('admin-token'), false);
  assert.ok(report.checks.some((check) => check.name === 'merchant_scope_1102'));
  assert.ok(report.checks.some((check) => check.name === 'grafana_dashboards'));
});

test('verifyReleaseReadiness reports a broken catalog image', async () => {
  const report = await verifyReleaseReadiness({
    baseURL: 'http://127.0.0.1:8889',
    prometheusURL: 'http://127.0.0.1:8889/prom',
    grafanaURL: 'http://127.0.0.1:8889/grafana',
    jaegerURL: 'http://127.0.0.1:8889/jaeger',
    rabbitmqURL: 'http://127.0.0.1:8889/rabbit',
    fetchImpl: fixtureFetch({ brokenImage: true }),
  });

  assert.equal(report.passed, false);
  assert.ok(report.checks.some((check) => check.name === 'asset_/products/b.webp' && !check.passed));
});

test('verifyReleaseReadiness rejects Jaeger without a Flash Mall service', async () => {
  const report = await verifyReleaseReadiness({
    baseURL: 'http://127.0.0.1:8889',
    prometheusURL: 'http://127.0.0.1:8889/prom',
    grafanaURL: 'http://127.0.0.1:8889/grafana',
    jaegerURL: 'http://127.0.0.1:8889/jaeger',
    rabbitmqURL: 'http://127.0.0.1:8889/rabbit',
    fetchImpl: fixtureFetch({ jaegerServices: ['jaeger-all-in-one'] }),
  });

  assert.equal(report.passed, false);
  assert.ok(report.checks.some((check) => check.name === 'jaeger_services' && !check.passed));
});
