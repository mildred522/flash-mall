import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

function round(value, digits = 2) {
  const scale = 10 ** digits;
  return Math.round((value + Number.EPSILON) * scale) / scale;
}

function median(values) {
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
}

function coefficientOfVariation(values) {
  const mean = values.reduce((sum, value) => sum + value, 0) / values.length;
  if (mean === 0 || values.length < 2) return 0;
  const variance = values.reduce((sum, value) => sum + (value - mean) ** 2, 0) / (values.length - 1);
  return Math.sqrt(variance) / mean;
}

function summarizeGroup(reports) {
  const qps = reports.map((report) => report.qps);
  const p95 = reports.map((report) => report.p95_ms);
  return {
    runs: reports.length,
    qps_median: round(median(qps)),
    qps_cv: round(coefficientOfVariation(qps), 4),
    p50_median_ms: round(median(reports.map((report) => report.p50_ms ?? 0))),
    p95_median_ms: round(median(p95)),
    p95_cv: round(coefficientOfVariation(p95), 4),
    p99_median_ms: round(median(reports.map((report) => report.p99_ms))),
    success_rate_min: Math.min(...reports.map((report) => report.success_rate)),
    response_bytes_median: median(reports.map((report) => report.response_bytes)),
  };
}

export function summarizeReports(reports) {
  const entryReports = reports.filter((report) => report.name.startsWith('entry-'));
  const hertzReports = reports.filter((report) => report.name.startsWith('hertz-'));
  if (entryReports.length === 0 || entryReports.length !== hertzReports.length) {
    throw new Error('entry and hertz reports must have the same non-zero run count');
  }
  const entry = summarizeGroup(entryReports);
  const hertz = summarizeGroup(hertzReports);
  const stabilityThreshold = 0.1;
  const coefficients = {
    'entry.qps_cv': entry.qps_cv,
    'entry.p95_cv': entry.p95_cv,
    'hertz.qps_cv': hertz.qps_cv,
    'hertz.p95_cv': hertz.p95_cv,
  };
  const exceeded = Object.entries(coefficients)
    .filter(([, value]) => value > stabilityThreshold)
    .map(([name]) => name);
  return {
    entry,
    hertz,
    stability: {
      threshold_cv: stabilityThreshold,
      status: exceeded.length === 0 ? 'stable' : 'unstable',
      exceeded,
    },
    comparison: {
      qps_change_pct: round((hertz.qps_median / entry.qps_median - 1) * 100),
      p95_change_pct: round((hertz.p95_median_ms / entry.p95_median_ms - 1) * 100),
    },
  };
}

function markdown(summary, metadata) {
  return `# Entry API 与 Hertz 性能对比

> 这是同一公开目录业务的架构对比，不是隔离 HTTP 框架的微基准。两条链路共享 Product RPC、Redis、MySQL 与 Etcd，但各自保留真实缓存和编排方式。

- baseline: \`${metadata.baseline_commit}\`（\`origin/main\`）
- candidate: \`${metadata.candidate_commit}\`（\`codex/arch-hertz-kitex\`）
- requests/run: ${metadata.requests}，concurrency: ${metadata.concurrency}，runs: ${summary.entry.runs}
- Entry 响应字节中位数: ${summary.entry.response_bytes_median}；Hertz: ${summary.hertz.response_bytes_median}
- 稳定性: ${summary.stability.status === 'stable' ? '通过' : `未通过（超限：${summary.stability.exceeded.join('、')}）`}；CV 门槛: ${summary.stability.threshold_cv}

| gateway | QPS 中位数 | QPS CV | p50 ms | p95 ms | p95 CV | p99 ms | 最低成功率 |
|---|---:|---:|---:|---:|---:|---:|---:|
| Go-zero Entry | ${summary.entry.qps_median} | ${summary.entry.qps_cv} | ${summary.entry.p50_median_ms} | ${summary.entry.p95_median_ms} | ${summary.entry.p95_cv} | ${summary.entry.p99_median_ms} | ${(summary.entry.success_rate_min * 100).toFixed(2)}% |
| Hertz | ${summary.hertz.qps_median} | ${summary.hertz.qps_cv} | ${summary.hertz.p50_median_ms} | ${summary.hertz.p95_median_ms} | ${summary.hertz.p95_cv} | ${summary.hertz.p99_median_ms} | ${(summary.hertz.success_rate_min * 100).toFixed(2)}% |

Hertz 相对 Entry：QPS ${summary.comparison.qps_change_pct}%；p95 ${summary.comparison.p95_change_pct}%。正数表示增加，负数表示降低。
`;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const [inputDir, summaryPath, reportPath, metadataPath] = process.argv.slice(2);
  if (!inputDir || !summaryPath || !reportPath || !metadataPath) {
    throw new Error('usage: summarize-comparison.mjs INPUT_DIR SUMMARY_JSON REPORT_MD METADATA_JSON');
  }
  const reports = readdirSync(inputDir).filter((name) => /^run-\d+-(entry|hertz)\.json$/.test(name))
    .map((name) => JSON.parse(readFileSync(`${inputDir}/${name}`, 'utf8')));
  const metadata = JSON.parse(readFileSync(metadataPath, 'utf8'));
  const summary = {...summarizeReports(reports), metadata};
  writeFileSync(summaryPath, `${JSON.stringify(summary, null, 2)}\n`);
  writeFileSync(reportPath, markdown(summary, metadata));
}
