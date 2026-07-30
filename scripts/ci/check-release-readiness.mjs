import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const verifierPath = resolve(root, 'scripts/local/verify-release-readiness.mjs');
const testPath = resolve(root, 'scripts/local/verify-release-readiness.test.mjs');
for (const path of [verifierPath, testPath]) {
  if (!existsSync(path)) throw new Error(`release readiness artifact is missing: ${path}`);
}

const verifier = readFileSync(verifierPath, 'utf8');
for (const [name, pattern] of [
  ['loopback-only target guard', /localhost.*127\.0\.0\.1|127\.0\.0\.1.*localhost/s],
  ['three frontend entries', /'\/shop'.*'\/admin'.*'\/merchant'/s],
  ['fixed user account', /13800000001/],
  ['fixed administrator account', /13800000002/],
  ['two isolated merchants', /1101.*1102/s],
  ['catalog asset verification', /content-type.*image\//s],
  ['Prometheus verification', /prometheus_targets/],
  ['Grafana verification', /grafana_dashboards/],
  ['Jaeger verification', /jaeger_services/],
  ['RabbitMQ verification', /rabbitmq_health/],
]) {
  if (!pattern.test(verifier)) throw new Error(`release readiness lacks ${name}`);
}
for (const forbidden of [
  '/api/order/create',
  '/api/order/pay',
  '/api/admin/products/update',
  '/api/merchant/products/update',
]) {
  if (verifier.includes(forbidden)) {
    throw new Error(`release readiness must not mutate business data: ${forbidden}`);
  }
}

for (const legacy of [
  'scripts/local/launcher.ps1',
  'scripts/local/start-all.ps1',
  'scripts/local/stop-all.ps1',
  'scripts/local/prepare-local-exes.ps1',
  'scripts/local/test-launcher.ps1',
]) {
  if (existsSync(resolve(root, legacy))) throw new Error(`legacy 8888 launcher still exists: ${legacy}`);
}

console.log('Release readiness contract verified: local Hertz entry, roles, assets, and observability');
