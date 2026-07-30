import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const profilePath = resolve(root, 'scripts/perf/run-capacity-profile.sh');
const summarizerPath = resolve(root, 'scripts/perf/summarize-capacity.mjs');
const toolPath = resolve(root, 'tools/capacitybench/main.go');
const resultPath = resolve(root, 'benchmarks/results/capacity-20260730.json');

for (const path of [profilePath, summarizerPath, toolPath, resultPath]) {
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
  ['relative output-safe summary gate', /readFileSync\(process\.argv\[1\].*summary\.overall_passed/s],
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

const result = JSON.parse(readFileSync(resultPath, 'utf8'));
const minimumTargets = {
  read: 600,
  'order-cycle': 10,
  'payment-cycle': 2,
  idempotency: 20,
};
if (!result.overall_passed ||
    !result.invariants?.passed ||
    result.violations?.length !== 0 ||
    !/^[0-9a-f]{40}$/.test(result.metadata?.commit ?? '')) {
  throw new Error('frozen capacity result has lost its provenance or correctness gate');
}
for (const [scenario, minimumTarget] of Object.entries(minimumTargets)) {
  const value = result.scenarios?.[scenario];
  if (!value ||
      value.safe_target_rps < minimumTarget ||
      value.stages.some((stage) => !stage.passed || stage.failed !== 0 || stage.dropped !== 0)) {
    throw new Error(`frozen capacity result is invalid for ${scenario}`);
  }
}

console.log('Capacity profile contract verified: current Hertz flows, safe reset, invariants, and recovery');
