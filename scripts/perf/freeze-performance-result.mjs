import { readFileSync, writeFileSync } from 'node:fs';

const [inputPath, outputPath] = process.argv.slice(2);
if (!inputPath || !outputPath) {
  throw new Error('usage: freeze-performance-result.mjs SUMMARY_JSON OUTPUT_JSON');
}

const summary = JSON.parse(readFileSync(inputPath, 'utf8'));
if (!summary.overall_passed) {
  throw new Error('refusing to freeze a performance suite that did not pass');
}

const stability = summary.resources?.['stability-mixed'] ?? {};
const selectedContainers = [
  'hertz-gateway', 'order-rpc', 'inventory-kitex', 'mysql', 'redis', 'rabbitmq', 'jaeger',
];
const containers = Object.fromEntries(selectedContainers
  .filter((name) => stability.containers?.[name])
  .map((name) => [name, stability.containers[name]]));

const frozen = {
  schema_version: 2,
  metadata: summary.metadata,
  invariants: summary.invariants,
  overall_passed: summary.overall_passed,
  resource_gates: summary.resource_gates,
  profiles: summary.profiles,
  bottlenecks: summary.bottlenecks,
  stability_resource_highlights: {
    containers,
    mysql: {
      threads_running: stability.mysql?.Threads_running,
      row_lock_current_waits: stability.mysql?.Innodb_row_lock_current_waits,
      row_lock_time: stability.mysql?.Innodb_row_lock_time,
    },
    redis: {
      blocked_clients: stability.redis?.blocked_clients,
      used_memory: stability.redis?.used_memory,
    },
    host: { available_memory_kb: stability.host?.available_memory_kb },
  },
};

writeFileSync(outputPath, `${JSON.stringify(frozen, null, 2)}\n`);
