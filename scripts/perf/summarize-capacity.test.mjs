import assert from 'node:assert/strict';
import test from 'node:test';

import { summarizeCapacity } from './summarize-capacity.mjs';

function result(scenario, rps, successRate, p95, p99) {
  return {
    report: {
      scenario,
      target_rps: rps,
      success_rate: successRate,
      p95_ms: p95,
      p99_ms: p99,
      qps: rps,
      attempts: 100,
      success: Math.round(100 * successRate),
      failed: Math.round(100 * (1 - successRate)),
      dropped: 0,
    },
  };
}

test('summarizeCapacity selects the last passing load stage', () => {
  const summary = summarizeCapacity({
    results: [
      result('read', 100, 1, 20, 40),
      result('read', 200, 0.999, 80, 200),
      result('read', 400, 0.98, 120, 400),
    ],
    invariants: { passed: true, violations: [] },
    metadata: { commit: 'abc' },
  });

  assert.equal(summary.scenarios.read.safe_target_rps, 200);
  assert.equal(summary.scenarios.read.first_failed_target_rps, 400);
  assert.equal(summary.overall_passed, true);
});

test('summarizeCapacity fails write stages when invariants fail', () => {
  const summary = summarizeCapacity({
    results: [result('order-cycle', 10, 1, 100, 200)],
    invariants: { passed: false, violations: ['reserved_reservations'] },
    metadata: {},
  });

  assert.equal(summary.scenarios['order-cycle'].safe_target_rps, 0);
  assert.equal(summary.overall_passed, false);
  assert.deepEqual(summary.violations, ['reserved_reservations']);
});

test('summarizeCapacity treats a repeated target as failed when any sample fails', () => {
  const summary = summarizeCapacity({
    results: [
      result('payment-cycle', 2, 0.6, 100, 120),
      result('payment-cycle', 2, 1, 90, 110),
    ],
    invariants: { passed: true, violations: [] },
    metadata: {},
  });

  assert.equal(summary.scenarios['payment-cycle'].safe_target_rps, 0);
  assert.equal(summary.scenarios['payment-cycle'].first_failed_target_rps, 2);
  assert.equal(summary.overall_passed, false);
});
