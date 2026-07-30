import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const targets = {
  read: { successRate: 0.999, p95: 100, p99: 250 },
  'order-cycle': { successRate: 0.99, p95: 1500, p99: 3000 },
  'payment-cycle': { successRate: 0.99, p95: 2000, p99: 4000 },
  idempotency: { successRate: 0.99, p95: 2000, p99: 4000 },
};

function stagePassed(report, invariants) {
  const target = targets[report.scenario];
  if (!target) return false;
  if (report.scenario !== 'read' && !invariants.passed) return false;
  return report.success_rate >= target.successRate &&
    report.p95_ms <= target.p95 &&
    report.p99_ms <= target.p99 &&
    (report.dropped ?? 0) === 0;
}

export function summarizeCapacity({ results, invariants, metadata }) {
  const grouped = {};
  for (const result of results) {
    const report = result.report;
    if (!report || !targets[report.scenario]) continue;
    const stage = {
      target_rps: report.target_rps,
      qps: report.qps,
      success_rate: report.success_rate,
      p95_ms: report.p95_ms,
      p99_ms: report.p99_ms,
      attempts: report.attempts,
      failed: report.failed,
      dropped: report.dropped ?? 0,
      passed: stagePassed(report, invariants),
    };
    (grouped[report.scenario] ??= []).push(stage);
  }

  const scenarios = {};
  for (const [name, stages] of Object.entries(grouped)) {
    stages.sort((left, right) => left.target_rps - right.target_rps);
    const targetResults = new Map();
    for (const stage of stages) {
      const aggregate = targetResults.get(stage.target_rps) ?? { passed: true };
      aggregate.passed &&= stage.passed;
      targetResults.set(stage.target_rps, aggregate);
    }
    const passingTargets = [...targetResults.entries()]
      .filter(([, aggregate]) => aggregate.passed)
      .map(([targetRPS]) => targetRPS);
    const failedTargets = [...targetResults.entries()]
      .filter(([, aggregate]) => !aggregate.passed)
      .map(([targetRPS]) => targetRPS);
    scenarios[name] = {
      safe_target_rps: passingTargets.at(-1) ?? 0,
      first_failed_target_rps: failedTargets.at(0) ?? 0,
      stages,
    };
  }

  const violations = invariants.passed ? [] : [...new Set(invariants.violations ?? [])];
  return {
    schema_version: 1,
    metadata,
    invariants,
    scenarios,
    overall_passed: Object.keys(scenarios).length > 0 &&
      Object.values(scenarios).every((scenario) => scenario.safe_target_rps > 0) &&
      violations.length === 0,
    violations,
  };
}

function markdown(summary) {
  const lines = [
    '# Flash Mall 容量边界',
    '',
    `- commit: \`${summary.metadata.commit ?? 'unknown'}\``,
    `- environment: ${summary.metadata.environment ?? 'unknown'}`,
    `- correctness invariants: ${summary.invariants.passed ? 'passed' : 'failed'}`,
    '',
    '| scenario | safe target RPS | first failed RPS |',
    '|---|---:|---:|',
  ];
  for (const [name, scenario] of Object.entries(summary.scenarios)) {
    lines.push(`| ${name} | ${scenario.safe_target_rps} | ${scenario.first_failed_target_rps || '-'} |`);
  }
  if (summary.violations.length > 0) {
    lines.push('', `Violations: ${summary.violations.join(', ')}`);
  }
  lines.push('', '> 结果是当前开发机与固定 Compose 资源下的可重复参考容量，不代表公网生产容量。', '');
  return lines.join('\n');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const [inputDir, summaryPath, reportPath] = process.argv.slice(2);
  if (!inputDir || !summaryPath || !reportPath) {
    throw new Error('usage: summarize-capacity.mjs INPUT_DIR SUMMARY_JSON REPORT_MD');
  }
  const results = readdirSync(inputDir)
    .filter((name) => /^stage-.*\.json$/.test(name))
    .map((name) => JSON.parse(readFileSync(`${inputDir}/${name}`, 'utf8')));
  const invariants = JSON.parse(readFileSync(`${inputDir}/invariants.json`, 'utf8'));
  const metadata = JSON.parse(readFileSync(`${inputDir}/metadata.json`, 'utf8'));
  const summary = summarizeCapacity({ results, invariants, metadata });
  writeFileSync(summaryPath, `${JSON.stringify(summary, null, 2)}\n`);
  writeFileSync(reportPath, markdown(summary));
}
