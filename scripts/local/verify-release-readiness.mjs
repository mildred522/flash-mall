#!/usr/bin/env node
import { fileURLToPath } from 'node:url';

const defaultAccounts = [
  { name: 'user', phone: '13800000001', password: 'flashmall123', userID: 1001 },
  { name: 'admin', phone: '13800000002', password: 'admin123', userID: 1002 },
  { name: 'merchant_1101', phone: '13800001101', password: 'flashmall123', userID: 1101, merchantID: 1101 },
  { name: 'merchant_1102', phone: '13800001102', password: 'flashmall123', userID: 1102, merchantID: 1102 },
];

export function assertLocalReadinessTarget(rawURL) {
  const target = new URL(rawURL);
  if (target.protocol !== 'http:' || target.username || target.password) {
    throw new Error(`readiness target must be credential-free local HTTP: ${rawURL}`);
  }
  const host = target.hostname.toLowerCase();
  if (host !== 'localhost' && host !== '127.0.0.1' && host !== '[::1]' && host !== '::1') {
    throw new Error(`readiness target must use a loopback host: ${rawURL}`);
  }
  return target;
}

function endpoint(base, path) {
  const prefix = base.pathname.replace(/\/$/, '');
  return `${base.origin}${prefix}${path}`;
}

function basicAuth(user, password) {
  return `Basic ${Buffer.from(`${user}:${password}`).toString('base64')}`;
}

function bearer(token) {
  return { authorization: `Bearer ${token}` };
}

async function expectResponse(fetchImpl, url, init, validate) {
  const response = await fetchImpl(url, init);
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  if (validate) await validate(response);
  return response;
}

async function expectJSON(fetchImpl, url, init, validate) {
  const response = await expectResponse(fetchImpl, url, init);
  const value = await response.json();
  if (validate) await validate(value);
  return value;
}

function expectEnvelope(value) {
  if (value?.code !== 'OK' || !value.data) throw new Error('API envelope is not successful');
  return value.data;
}

export async function verifyReleaseReadiness(options = {}) {
  const fetchImpl = options.fetchImpl ?? fetch;
  const bases = {
    app: assertLocalReadinessTarget(options.baseURL ?? 'http://127.0.0.1:8889'),
    prometheus: assertLocalReadinessTarget(options.prometheusURL ?? 'http://127.0.0.1:9099'),
    grafana: assertLocalReadinessTarget(options.grafanaURL ?? 'http://127.0.0.1:3000'),
    jaeger: assertLocalReadinessTarget(options.jaegerURL ?? 'http://127.0.0.1:16686'),
    rabbitmq: assertLocalReadinessTarget(options.rabbitmqURL ?? 'http://127.0.0.1:15672'),
  };
  const checks = [];
  const run = async (name, operation, describe) => {
    try {
      const value = await operation();
      const detail = describe
        ? describe(value)
        : value instanceof Response
          ? `HTTP ${value.status}`
          : typeof value === 'string' || typeof value === 'number'
            ? value
            : 'ok';
      checks.push({
        name,
        passed: true,
        detail,
      });
      return value;
    } catch (error) {
      checks.push({ name, passed: false, detail: error instanceof Error ? error.message : String(error) });
      return undefined;
    }
  };

  await run('liveness', () => expectResponse(fetchImpl, endpoint(bases.app, '/live')));
  await run('readiness', async () => expectEnvelope(await expectJSON(
    fetchImpl, endpoint(bases.app, '/ready'),
  )));
  await run('system_health', async () => {
    const data = expectEnvelope(await expectJSON(fetchImpl, endpoint(bases.app, '/api/system/health')));
    if (data.status !== 'ok') throw new Error(`status=${data.status ?? 'missing'}`);
    return data.name ?? data.service;
  });
  for (const page of ['/shop', '/admin', '/merchant']) {
    await run(`page_${page.slice(1)}`, () => expectResponse(
      fetchImpl,
      endpoint(bases.app, page),
      undefined,
      async (response) => {
        if (!response.headers.get('content-type')?.includes('text/html')) {
          throw new Error('not an HTML page');
        }
      },
    ));
  }

  const catalog = await run('catalog', async () => {
    const data = expectEnvelope(await expectJSON(fetchImpl, endpoint(bases.app, '/api/shop/catalog')));
    if (!Array.isArray(data.items) || data.items.length === 0) throw new Error('catalog is empty');
    for (const merchantID of [1101, 1102]) {
      if (!data.items.some((item) => item.merchant_id === merchantID)) {
        throw new Error(`merchant ${merchantID} is absent from showcase`);
      }
    }
    return data.items;
  });

  const assetPaths = new Set((catalog ?? []).map((item) => item.image_url).filter(Boolean));
  for (const merchantID of [1101, 1102]) {
    const store = await run(`store_${merchantID}`, async () => {
      const data = expectEnvelope(await expectJSON(
        fetchImpl,
        endpoint(bases.app, `/api/shop/stores/detail?merchant_id=${merchantID}`),
      ));
      if (data.merchant_id !== merchantID || data.status !== 1) {
        throw new Error(`unexpected store identity or status for ${merchantID}`);
      }
      return data;
    });
    if (store?.logo_url) assetPaths.add(store.logo_url);
    if (store?.banner_url) assetPaths.add(store.banner_url);
  }
  for (const path of assetPaths) {
    await run(`asset_${path}`, () => expectResponse(
      fetchImpl,
      endpoint(bases.app, path),
      undefined,
      async (response) => {
        if (!response.headers.get('content-type')?.startsWith('image/')) {
          throw new Error('asset content type is not image/*');
        }
      },
    ));
  }

  const sessions = new Map();
  for (const account of options.accounts ?? defaultAccounts) {
    const login = await run(`login_${account.name}`, async () => {
      const value = await expectJSON(fetchImpl, endpoint(bases.app, '/api/auth/login'), {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          phone: account.phone,
          password: account.password,
          device_type: 'release-readiness',
        }),
      });
      if (!value.access_token || value.user_id !== account.userID) {
        throw new Error(`unexpected identity for ${account.name}`);
      }
      return value.access_token;
    }, () => `user_id=${account.userID}`);
    if (login) sessions.set(account.name, login);
  }
  await run('user_orders', async () => expectEnvelope(await expectJSON(
    fetchImpl,
    endpoint(bases.app, '/api/orders?page=1&page_size=1'),
    { headers: bearer(sessions.get('user')) },
  )));
  await run('admin_dashboard', async () => expectEnvelope(await expectJSON(
    fetchImpl,
    endpoint(bases.app, '/api/admin/dashboard/stats'),
    { headers: bearer(sessions.get('admin')) },
  )));
  for (const merchantID of [1101, 1102]) {
    const token = sessions.get(`merchant_${merchantID}`);
    await run(`merchant_scope_${merchantID}`, async () => {
      const data = expectEnvelope(await expectJSON(
        fetchImpl,
        endpoint(bases.app, '/api/merchant/me'),
        { headers: bearer(token) },
      ));
      if (data.items?.length !== 1 || data.items[0].merchant_id !== merchantID) {
        throw new Error(`merchant scope is not isolated to ${merchantID}`);
      }
      return data.items[0].name;
    });
    await run(`merchant_products_${merchantID}`, async () => expectEnvelope(await expectJSON(
      fetchImpl,
      endpoint(bases.app, '/api/merchant/products?page=1&page_size=1'),
      { headers: bearer(token) },
    )));
  }

  await run('prometheus_targets', async () => {
    const value = await expectJSON(fetchImpl, endpoint(bases.prometheus, '/api/v1/targets'));
    const required = ['hertz-gateway', 'order-rpc', 'product-rpc', 'inventory-kitex'];
    for (const job of required) {
      const target = value.data?.activeTargets?.find((item) => item.labels?.job === job);
      if (target?.health !== 'up') throw new Error(`${job} is not up`);
    }
    return `${required.length} targets up`;
  });
  await run('grafana_health', async () => {
    const value = await expectJSON(fetchImpl, endpoint(bases.grafana, '/api/health'));
    if (value.database !== 'ok') throw new Error('Grafana database is not ok');
  });
  await run('grafana_dashboards', async () => {
    const value = await expectJSON(
      fetchImpl,
      endpoint(bases.grafana, '/api/search?type=dash-db'),
      { headers: { authorization: basicAuth(
        options.grafanaUser ?? process.env.FLASH_MALL_GRAFANA_USER ?? 'admin',
        options.grafanaPassword ?? process.env.FLASH_MALL_GRAFANA_PASSWORD ?? 'flashmall',
      ) } },
    );
    if (!Array.isArray(value) || value.length < 5) throw new Error(`dashboard count=${value?.length ?? 0}`);
    return `${value.length} dashboards`;
  });
  await run('jaeger_services', async () => {
    const value = await expectJSON(fetchImpl, endpoint(bases.jaeger, '/api/services'));
    if (!Array.isArray(value.data) || value.data.length === 0) throw new Error('Jaeger has no traced services');
    const businessServices = value.data.filter((name) => name !== 'jaeger-all-in-one');
    if (!businessServices.includes('hertz-gateway')) {
      throw new Error(`Flash Mall gateway trace is absent: ${businessServices.join(', ') || 'none'}`);
    }
    return `${businessServices.length} Flash Mall traced services`;
  });
  await run('rabbitmq_health', async () => {
    const value = await expectJSON(
      fetchImpl,
      endpoint(bases.rabbitmq, '/api/overview'),
      { headers: { authorization: basicAuth(
        options.rabbitmqUser ?? process.env.FLASH_MALL_RABBITMQ_USER ?? 'flashmall',
        options.rabbitmqPassword ?? process.env.FLASH_MALL_RABBITMQ_PASSWORD ?? 'flashmall-local',
      ) } },
    );
    if (!value.rabbitmq_version) throw new Error('RabbitMQ overview is incomplete');
    return `RabbitMQ ${value.rabbitmq_version}`;
  });

  const failed = checks.filter((check) => !check.passed).length;
  return {
    schema_version: 1,
    recorded_at: new Date().toISOString(),
    base_url: bases.app.origin,
    passed: failed === 0,
    total: checks.length,
    failed,
    checks,
  };
}

function parseArguments(argv) {
  const values = {
    baseURL: process.env.FLASH_MALL_BASE_URL ?? 'http://127.0.0.1:8889',
    prometheusURL: process.env.FLASH_MALL_PROMETHEUS_URL ?? 'http://127.0.0.1:9099',
    grafanaURL: process.env.FLASH_MALL_GRAFANA_URL ?? 'http://127.0.0.1:3000',
    jaegerURL: process.env.FLASH_MALL_JAEGER_URL ?? 'http://127.0.0.1:16686',
    rabbitmqURL: process.env.FLASH_MALL_RABBITMQ_URL ?? 'http://127.0.0.1:15672',
    json: false,
  };
  const names = new Map([
    ['--base-url', 'baseURL'],
    ['--prometheus-url', 'prometheusURL'],
    ['--grafana-url', 'grafanaURL'],
    ['--jaeger-url', 'jaegerURL'],
    ['--rabbitmq-url', 'rabbitmqURL'],
  ]);
  for (let index = 0; index < argv.length; index += 1) {
    const name = argv[index];
    if (name === '--json') {
      values.json = true;
      continue;
    }
    const key = names.get(name);
    if (!key || !argv[index + 1]) throw new Error(`unknown or incomplete argument: ${name}`);
    values[key] = argv[index + 1];
    index += 1;
  }
  return values;
}

async function main() {
  const options = parseArguments(process.argv.slice(2));
  const report = await verifyReleaseReadiness(options);
  if (options.json) {
    process.stdout.write(`${JSON.stringify(report, null, 2)}\n`);
  } else {
    for (const check of report.checks) {
      process.stdout.write(`[${check.passed ? 'OK' : 'FAIL'}] ${check.name}: ${check.detail}\n`);
    }
    process.stdout.write(`Release readiness: ${report.total - report.failed}/${report.total} checks passed\n`);
  }
  process.exitCode = report.passed ? 0 : 1;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    process.stderr.write(`Release readiness failed: ${error.message}\n`);
    process.exitCode = 1;
  });
}
