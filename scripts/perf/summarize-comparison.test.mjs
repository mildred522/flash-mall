import assert from 'node:assert/strict';
import test from 'node:test';

import { summarizeReports } from './summarize-comparison.mjs';

test('summarizeReports uses medians and reports relative changes', () => {
  const reports = [
    { name: 'entry-run-1', qps: 100, p95_ms: 10, p99_ms: 15, success_rate: 1, response_bytes: 500 },
    { name: 'hertz-run-1', qps: 120, p95_ms: 8, p99_ms: 12, success_rate: 1, response_bytes: 520 },
    { name: 'hertz-run-2', qps: 110, p95_ms: 9, p99_ms: 13, success_rate: 1, response_bytes: 520 },
    { name: 'entry-run-2', qps: 90, p95_ms: 12, p99_ms: 18, success_rate: 1, response_bytes: 500 },
    { name: 'entry-run-3', qps: 110, p95_ms: 11, p99_ms: 16, success_rate: 1, response_bytes: 500 },
    { name: 'hertz-run-3', qps: 130, p95_ms: 7, p99_ms: 11, success_rate: 1, response_bytes: 520 },
  ];

  const got = summarizeReports(reports);
  assert.equal(got.entry.qps_median, 100);
  assert.equal(got.entry.p95_median_ms, 11);
  assert.equal(got.hertz.qps_median, 120);
  assert.equal(got.hertz.p95_median_ms, 8);
  assert.equal(got.comparison.qps_change_pct, 20);
  assert.equal(got.comparison.p95_change_pct, -27.27);
  assert.equal(got.stability.status, 'unstable');
  assert.deepEqual(got.stability.exceeded, ['hertz.p95_cv']);
});

test('summarizeReports marks samples stable only when every CV is at most ten percent', () => {
  const reports = [
    { name: 'entry-run-1', qps: 100, p95_ms: 10, p99_ms: 14, success_rate: 1, response_bytes: 500 },
    { name: 'entry-run-2', qps: 102, p95_ms: 10.2, p99_ms: 14.2, success_rate: 1, response_bytes: 500 },
    { name: 'entry-run-3', qps: 98, p95_ms: 9.8, p99_ms: 13.8, success_rate: 1, response_bytes: 500 },
    { name: 'hertz-run-1', qps: 120, p95_ms: 8, p99_ms: 12, success_rate: 1, response_bytes: 520 },
    { name: 'hertz-run-2', qps: 122, p95_ms: 8.2, p99_ms: 12.2, success_rate: 1, response_bytes: 520 },
    { name: 'hertz-run-3', qps: 118, p95_ms: 7.8, p99_ms: 11.8, success_rate: 1, response_bytes: 520 },
  ];

  assert.deepEqual(summarizeReports(reports).stability, {
    threshold_cv: 0.1,
    status: 'stable',
    exceeded: [],
  });
});
