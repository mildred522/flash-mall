import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const files = {
  runner: resolve(root, 'scripts/perf/run-performance-suite.sh'),
  collector: resolve(root, 'scripts/perf/collect-stage-resources.sh'),
  summarizer: resolve(root, 'scripts/perf/summarize-performance.mjs'),
  stageGate: resolve(root, 'scripts/perf/check-stage-performance.mjs'),
  tool: resolve(root, 'tools/capacitybench/main.go'),
  stats: resolve(root, 'tools/capacitybench/stats.go'),
  compose: resolve(root, 'deploy/docker-compose.yml'),
};
for (const [name, path] of Object.entries(files)) {
  if (!existsSync(path)) throw new Error(`performance ${name} is missing: ${path}`);
}

const runner = readFileSync(files.runner, 'utf8');
if (/\]\][^\n]*&&\s*break/.test(runner)) {
  throw new Error('stress ladder must not leak a false test status under set -e');
}
for (const [name, pattern] of [
  ['loopback mutation guard', /127\.0\.0\.1.*localhost|localhost.*127\.0\.0\.1/s],
  ['explicit mutation approval', /--allow-mutation/],
  ['explicit reset approval', /--confirm-reset/],
  ['exit restoration trap', /trap restore_demo/],
  ['baseline profile', /run_stage baseline/],
  ['expected load profile', /run_stage load/],
  ['stress ladder', /run_stress_ladder/],
  ['mixed stability profile', /run_stability_mix/],
  ['continuous resource sampling', /start_sampler/],
  ['CPU profile capture', /debug\/pprof\/profile/],
  ['Compose-network load generation', /allow-compose-target/],
  ['isolated load-generator cleanup', /flashmall\.performance\.run/],
  ['static load-generator build', /CGO_ENABLED=0/],
  ['RabbitMQ recovery', /docker pause rabbitmq/],
  ['business invariants', /verify_invariants/],
  ['fixture restoration', /reset-demo --confirm-reset/],
]) {
  if (!pattern.test(runner)) throw new Error(`performance suite lacks ${name}`);
}

const collector = readFileSync(files.collector, 'utf8');
for (const [name, pattern] of [
  ['container sampling', /docker stats --no-stream/],
  ['MySQL lock sampling', /Innodb_row_lock_current_waits/],
  ['Redis saturation sampling', /blocked_clients/],
  ['host load sampling', /\/proc\/loadavg/],
]) {
  if (!pattern.test(collector)) throw new Error(`resource collector lacks ${name}`);
}

const stats = readFileSync(files.stats, 'utf8');
if (!/Operations.*latencySummary/s.test(stats) || !/Steps.*latencySummary/s.test(stats)) {
  throw new Error('capacity reports must preserve operation and business-step latency');
}

const compose = readFileSync(files.compose, 'utf8');
for (const port of ['6061', '6062', '6063', '6064']) {
  if (!compose.includes(port)) throw new Error(`pprof port ${port} is not available to the local suite`);
}

console.log('Performance suite contract verified: baseline, load, stress, stability, profiles, resources, recovery');
