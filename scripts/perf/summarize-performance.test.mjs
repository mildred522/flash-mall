import assert from 'node:assert/strict';
import test from 'node:test';

import { performanceStagePassed, summarizePerformance } from './summarize-performance.mjs';

function result(kind, scenario, stage, rps, successRate, p95, options = {}) {
  return {
    test_kind: kind,
    stage,
    report: {
      scenario,
      target_rps: rps,
      qps: options.qps ?? rps,
      success_rate: successRate,
      p95_ms: p95,
      p99_ms: options.p99 ?? p95 * 1.5,
      attempts: 100,
      failed: successRate === 1 ? 0 : 1,
      dropped: options.dropped ?? 0,
      operations: options.operations ?? {},
      steps: options.steps ?? {},
    },
  };
}

test('stress failure establishes a boundary without failing the required suite', () => {
  const summary = summarizePerformance({
    results: [
      result('baseline', 'read', 'baseline-read-1', 100, 1, 2),
      result('load', 'read', 'load-read', 600, 1, 5),
      result('stress', 'read', 'stress-read-800', 800, 1, 10),
      result('stress', 'read', 'stress-read-1200', 1200, 0.98, 150),
      result('stability', 'read', 'stability-read', 500, 1, 8),
      result('recovery', 'payment-cycle', 'recovery-payment', 2, 1, 100),
    ],
    invariants: { passed: true, violations: [] },
    metadata: { commit: 'abc' },
    resources: {},
  });

  assert.equal(summary.profiles.stress.read.last_passed_target_rps, 800);
  assert.equal(summary.profiles.stress.read.first_failed_target_rps, 1200);
  assert.equal(summary.overall_passed, true);
});

test('load or stability SLO failure fails the suite', () => {
  const summary = summarizePerformance({
    results: [
      result('baseline', 'read', 'baseline-read', 100, 1, 2),
      result('load', 'read', 'load-read', 600, 0.9, 200),
      result('stability', 'read', 'stability-read', 500, 1, 8),
      result('recovery', 'payment-cycle', 'recovery-payment', 2, 1, 100),
    ],
    invariants: { passed: true, violations: [] }, metadata: {}, resources: {},
  });

  assert.equal(summary.profiles.load.read.passed, false);
  assert.equal(summary.overall_passed, false);
});

test('failed stress stage reports dominant phase and CPU candidate', () => {
  const summary = summarizePerformance({
    results: [result('stress', 'order-cycle', 'stress-order', 40, 0.9, 1800, {
      steps: {
        create_order: { samples: 100, p95_ms: 1600 },
        cancel_order: { samples: 90, p95_ms: 100 },
      },
    })],
    invariants: { passed: true, violations: [] },
    metadata: { cpu_count: 4 },
    resources: {
      'stress-order': { containers: { 'order-rpc': { max_cpu_percent: 96 } } },
    },
  });

  assert.equal(summary.bottlenecks[0].dominant_step, 'create_order');
  assert.match(summary.bottlenecks[0].signals.join(','), /order-rpc CPU/);
});

test('performance-only stage gate ignores pending external invariants', () => {
  const report = result('stress', 'order-cycle', 'stress-order', 25, 1, 300).report;
  report.invariants = { passed: false, violations: ['external_verification_required'] };
  report.p99_ms = 500;
  assert.equal(performanceStagePassed(report), true);
});
