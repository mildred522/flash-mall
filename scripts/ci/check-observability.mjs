import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const observabilityDir = resolve(root, 'deploy/observability');
const dashboardDir = resolve(observabilityDir, 'grafana/dashboards');

function read(relativePath) {
  return readFileSync(resolve(root, relativePath), 'utf8');
}

function requireText(content, pattern, message) {
  if (!pattern.test(content)) {
    throw new Error(message);
  }
}

const prometheus = read('deploy/observability/prometheus.yml');
requireText(prometheus, /^rule_files:\s*\n\s*-\s*alerts\.yml$/m, 'Prometheus must load alerts.yml');

const compose = read('deploy/docker-compose.yml');
requireText(
  compose,
  /\.\/observability\/alerts\.yml:\/etc\/prometheus\/alerts\.yml:ro/,
  'Compose must mount the Prometheus alert rules',
);
requireText(
  compose,
  /\$\{FLASH_MALL_GRAFANA_PORT:-3000\}:3000/,
  'Grafana host port must remain configurable',
);

const alertsPath = resolve(observabilityDir, 'alerts.yml');
if (!existsSync(alertsPath)) {
  throw new Error('deploy/observability/alerts.yml is missing');
}
const alerts = readFileSync(alertsPath, 'utf8');
for (const alertName of [
  'FlashMallServiceDown',
  'InventoryCommandErrorRateHigh',
  'InventoryDeadLettersPresent',
  'OutboxDeadLettersPresent',
  'OutboxBacklogHigh',
  'PaymentCallbackErrors',
  'HertzHTTPErrorRateHigh',
  'HertzHTTPTailLatencyHigh',
]) {
  requireText(alerts, new RegExp(`alert:\\s*${alertName}\\b`), `missing alert ${alertName}`);
}

const requiredDashboardQueries = new Map([
  ['overview.json', ['up{job=~', 'sql_client_in_use_connections']],
  ['inventory.json', ['inventory_kitex_commands_total', 'inventory_kitex_command_duration_seconds_bucket', 'inventory_reservations']],
  ['payment-outbox.json', ['flashmall_payment_state_transition_duration_seconds_bucket', 'flashmall_payment_callback_duration_seconds_bucket', 'flashmall_outbox_publish_duration_seconds_bucket', 'flashmall_outbox_events']],
  ['cache-rpc.json', ['flashmall_cache_requests_total', 'rpc_server_requests_duration_ms_bucket', 'rpc_server_requests_code_total']],
  ['capacity-slo.json', ['flashmall_http_requests_total', 'flashmall_http_request_duration_seconds_bucket', 'flashmall_http_in_flight_requests']],
]);

for (const [filename, queries] of requiredDashboardQueries) {
  const path = resolve(dashboardDir, filename);
  if (!existsSync(path)) {
    throw new Error(`missing Grafana dashboard ${filename}`);
  }
  const dashboard = JSON.parse(readFileSync(path, 'utf8'));
  const ids = dashboard.panels.map((panel) => panel.id);
  if (new Set(ids).size !== ids.length) {
    throw new Error(`${filename} contains duplicate panel IDs`);
  }
  const expressions = dashboard.panels
    .flatMap((panel) => panel.targets ?? [])
    .map((target) => target.expr ?? '')
    .join('\n');
  for (const query of queries) {
    if (!expressions.includes(query)) {
      throw new Error(`${filename} does not query ${query}`);
    }
  }
}

const dashboards = readdirSync(dashboardDir).filter((filename) => filename.endsWith('.json'));
console.log(`Observability configuration verified: ${dashboards.length} dashboard(s), 8 alert(s)`);
