import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const profilePath = resolve(root, 'scripts/perf/run-capacity-profile.sh');
const summarizerPath = resolve(root, 'scripts/perf/summarize-capacity.mjs');
const toolPath = resolve(root, 'tools/capacitybench/main.go');

for (const path of [profilePath, summarizerPath, toolPath]) {
  if (!existsSync(path)) throw new Error(`capacity artifact is missing: ${path}`);
}

const profile = readFileSync(profilePath, 'utf8');
for (const [name, pattern] of [
  ['loopback-only write guard', /127\.0\.0\.1.*localhost|localhost.*127\.0\.0\.1/s],
  ['explicit mutation approval', /--allow-mutation/],
  ['explicit reset approval', /--confirm-reset/],
  ['demo verification', /(?:flash-mall-control\.sh|\$control)"? verify-demo/],
  ['recoverable pre-reset verification', /if ! "\$control" verify-demo/],
  ['pre-run backup and reset', /(?:flash-mall-control\.sh|\$control)"? reset-demo/],
  ['exit restoration trap', /trap ['"]?restore_demo/],
  ['upload volume preservation', /flash-mall-uploads/],
  ['RabbitMQ backlog scenario', /docker pause rabbitmq/],
  ['invariant verification', /inventory_reservation.*RESERVED/s],
  ['negative stock verification', /stock < 0|stock<0/],
  ['Outbox drain verification', /order_outbox.*status IN \(0,2,3\)/s],
  ['runtime resource snapshot', /docker stats/],
  ['observability network recreation', /--force-recreate prometheus grafana/],
  ['failed capacity exit gate', /overall_passed.*exit 1/s],
]) {
  if (!pattern.test(profile)) throw new Error(`capacity profile lacks ${name}`);
}

for (const stale of [
  'scripts/k8s/perf-collect.ps1',
  'scripts/k8s/perf-matrix.ps1',
  'scripts/k8s/perf-reliable.ps1',
  'scripts/k8s/run-benchmark-incluster.ps1',
  'scripts/k8s/run-benchmark.ps1',
  'app/entry/api/scripts/benchmark/benchmark_tool.go',
]) {
  if (existsSync(resolve(root, stale))) {
    throw new Error(`stale pre-Hertz benchmark entry still exists: ${stale}`);
  }
}

console.log('Capacity profile contract verified: current Hertz flows, safe reset, invariants, and recovery');
