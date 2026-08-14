import { existsSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const targets = {
  read: { successRate: 0.999, p95: 100, p99: 250 },
  'order-cycle': { successRate: 0.99, p95: 1500, p99: 3000 },
  'payment-cycle': { successRate: 0.99, p95: 2000, p99: 4000 },
  idempotency: { successRate: 0.99, p95: 2000, p99: 4000 },
};

export function performanceStagePassed(report) {
  const target = targets[report.scenario];
  if (!target) return false;
  const attainment = report.target_rps > 0 ? report.qps / report.target_rps : 1;
  const minimumAttainment = report.target_rps <= 2 ? 0.85 : 0.95;
  return report.success_rate >= target.successRate &&
    report.p95_ms <= target.p95 && report.p99_ms <= target.p99 &&
    (report.dropped ?? 0) === 0 && attainment >= minimumAttainment;
}

function stagePassed(report, invariants) {
  return invariants.passed && performanceStagePassed(report);
}

function median(values) {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((left, right) => left - right);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
}

function compactStage(result, invariants) {
  const report = result.report;
  return {
    stage: result.stage,
    target_rps: report.target_rps,
    qps: report.qps,
    attainment: report.target_rps > 0 ? report.qps / report.target_rps : 1,
    success_rate: report.success_rate,
    attempts: report.attempts,
    failed: report.failed,
    dropped: report.dropped ?? 0,
    p50_ms: report.p50_ms,
    p95_ms: report.p95_ms,
    p99_ms: report.p99_ms,
    max_ms: report.max_ms,
    operations: report.operations ?? {},
    steps: report.steps ?? {},
    passed: stagePassed(report, invariants),
  };
}

function profileScenario(kind, stages) {
  const passed = stages.every((stage) => stage.passed);
  const result = { passed, stages };
  if (kind === 'baseline') {
    result.median_p95_ms = median(stages.map((stage) => stage.p95_ms));
    result.median_p99_ms = median(stages.map((stage) => stage.p99_ms));
    result.p95_spread_ms = Math.max(...stages.map((stage) => stage.p95_ms)) -
      Math.min(...stages.map((stage) => stage.p95_ms));
  }
  if (kind === 'stress') {
    const passing = stages.filter((stage) => stage.passed).map((stage) => stage.target_rps);
    const failed = stages.filter((stage) => !stage.passed).map((stage) => stage.target_rps);
    result.last_passed_target_rps = passing.at(-1) ?? 0;
    result.first_failed_target_rps = failed[0] ?? 0;
    result.boundary_observed = failed.length > 0;
    result.passed = stages.length > 0;
  }
  return result;
}

function dominantStep(stage) {
  return Object.entries(stage.steps ?? {})
    .sort((left, right) => (right[1].p95_ms ?? 0) - (left[1].p95_ms ?? 0))[0]?.[0] ?? '';
}

function bottleneckEvidence(profiles, resources) {
  const evidence = [];
  for (const [scenario, profile] of Object.entries(profiles.stress ?? {})) {
    const stage = profile.stages.find((item) => !item.passed);
    if (!stage) continue;
    const resource = resources[stage.stage] ?? {};
    const signals = [];
    for (const [name, values] of Object.entries(resource.containers ?? {})) {
      if ((values.max_cpu_percent ?? 0) >= 85) {
        signals.push(`${name} CPU reached ${values.max_cpu_percent.toFixed(1)}%`);
      }
    }
    if ((resource.mysql?.Threads_running?.max ?? 0) >= 4) {
      signals.push(`MySQL running threads reached ${resource.mysql.Threads_running.max}`);
    }
    if ((resource.redis?.blocked_clients?.max ?? 0) > 0) {
      signals.push(`Redis blocked clients reached ${resource.redis.blocked_clients.max}`);
    }
    if (stage.attainment < 0.95 && stage.p95_ms < targets[scenario].p95 * 0.25 && stage.dropped === 0) {
      signals.push(`load generator pacing may be limiting target attainment (${(100 * stage.attainment).toFixed(1)}%)`);
    }
    if (signals.length === 0) signals.push('no resource saturation signal; inspect trace and Go profiles');
    evidence.push({
      scenario,
      first_failed_target_rps: stage.target_rps,
      achieved_qps: stage.qps,
      p95_ms: stage.p95_ms,
      dominant_step: dominantStep(stage),
      signals,
    });
  }
  return evidence;
}

function stabilityResourceGate(resources) {
  const violations = [];
  const stages = Object.entries(resources).filter(([stage]) => stage.startsWith('stability'));
  for (const [stage, resource] of stages) {
    for (const [name, values] of Object.entries(resource.containers ?? {})) {
      if (name.includes('loadgen')) continue;
      const growth = values.memory_bytes?.delta ?? 0;
      if (growth > 128 * 1024 ** 2) {
        violations.push(`${stage}: ${name} memory grew ${(growth / 1024 ** 2).toFixed(1)} MiB`);
      }
    }
    if ((resource.redis?.blocked_clients?.max ?? 0) > 0) {
      violations.push(`${stage}: Redis reported blocked clients`);
    }
    if ((resource.host?.available_memory_kb?.delta ?? 0) < -1024 * 1024) {
      violations.push(`${stage}: host available memory fell by more than 1 GiB`);
    }
  }
  return { observed: stages.length > 0, passed: violations.length === 0, violations };
}

export function summarizePerformance({ results, invariants, metadata, resources }) {
  const grouped = {};
  for (const result of results) {
    if (!result.report || !targets[result.report.scenario] || !result.test_kind) continue;
    const kind = result.test_kind;
    const scenario = result.report.scenario;
    ((grouped[kind] ??= {})[scenario] ??= []).push(compactStage(result, invariants));
  }

  const profiles = {};
  for (const [kind, scenarios] of Object.entries(grouped)) {
    profiles[kind] = {};
    for (const [scenario, stages] of Object.entries(scenarios)) {
      stages.sort((left, right) => left.target_rps - right.target_rps || left.stage.localeCompare(right.stage));
      profiles[kind][scenario] = profileScenario(kind, stages);
    }
  }
  const requiredKinds = ['baseline', 'load', 'stability', 'recovery'];
  const requiredPassed = requiredKinds.every((kind) => profiles[kind] &&
    Object.values(profiles[kind]).length > 0 && Object.values(profiles[kind]).every((item) => item.passed));
  const stabilityResources = stabilityResourceGate(resources);
  return {
    schema_version: 2,
    metadata,
    invariants,
    profiles,
    resources,
    resource_gates: { stability: stabilityResources },
    bottlenecks: bottleneckEvidence(profiles, resources),
    overall_passed: requiredPassed && invariants.passed && (!stabilityResources.observed || stabilityResources.passed),
    violations: [...new Set([...(invariants.violations ?? []), ...stabilityResources.violations])],
  };
}

function numericSummary(values) {
  if (values.length === 0) return { samples: 0, min: 0, max: 0, first: 0, last: 0, delta: 0 };
  return {
    samples: values.length,
    min: Math.min(...values),
    max: Math.max(...values),
    first: values[0],
    last: values.at(-1),
    delta: values.at(-1) - values[0],
  };
}

function parsePercent(value) {
  const parsed = Number.parseFloat(String(value ?? '').replace('%', ''));
  return Number.isFinite(parsed) ? parsed : 0;
}

export function parseByteSize(value) {
  const match = String(value ?? '').trim().match(/^([0-9.]+)\s*([kmgt]?i?b)$/i);
  if (!match) return 0;
  const units = { b: 1, kb: 1e3, mb: 1e6, gb: 1e9, tb: 1e12,
    kib: 1024, mib: 1024 ** 2, gib: 1024 ** 3, tib: 1024 ** 4 };
  const multiplier = units[match[2].toLowerCase()];
  const parsed = Number.parseFloat(match[1]);
  return Number.isFinite(parsed) && multiplier ? parsed * multiplier : 0;
}

function readResources(inputDir) {
  const resources = {};
  const dockerPath = `${inputDir}/resources-docker.jsonl`;
  if (existsSync(dockerPath)) {
    const grouped = {};
    for (const line of readFileSync(dockerPath, 'utf8').split(/\r?\n/).filter(Boolean)) {
      const item = JSON.parse(line);
      const stage = item.stage;
      const name = item.Name ?? item.Container ?? 'unknown';
      const key = `${stage}\u0000${name}`;
      const group = (grouped[key] ??= { cpu: [], memory: [] });
      group.cpu.push(parsePercent(item.CPUPerc));
      group.memory.push(parseByteSize(String(item.MemUsage ?? '').split('/')[0]));
    }
    for (const [key, values] of Object.entries(grouped)) {
      const [stage, name] = key.split('\u0000');
      const cpu = numericSummary(values.cpu);
      const memory = numericSummary(values.memory);
      (((resources[stage] ??= {}).containers ??= {})[name]) = {
        samples: cpu.samples,
        average_cpu_percent: values.cpu.reduce((sum, value) => sum + value, 0) / values.cpu.length,
        max_cpu_percent: cpu.max,
        memory_bytes: memory,
      };
    }
  }

  const servicesPath = `${inputDir}/resources-services.tsv`;
  if (existsSync(servicesPath)) {
    const grouped = {};
    for (const line of readFileSync(servicesPath, 'utf8').split(/\r?\n/).filter(Boolean)) {
      const [, stage, service, metric, rawValue] = line.split('\t');
      const value = Number(rawValue);
      if (!stage || !service || !metric || !Number.isFinite(value)) continue;
      const key = `${stage}\u0000${service}\u0000${metric}`;
      (grouped[key] ??= []).push(value);
    }
    for (const [key, values] of Object.entries(grouped)) {
      const [stage, service, metric] = key.split('\u0000');
      ((resources[stage] ??= {})[service] ??= {})[metric] = numericSummary(values);
    }
  }

  const hostPath = `${inputDir}/resources-host.tsv`;
  if (existsSync(hostPath)) {
    const grouped = {};
    for (const line of readFileSync(hostPath, 'utf8').split(/\r?\n/).filter(Boolean)) {
      const [, stage, load1, load5, load15, availableKB] = line.split('\t');
      const values = { load1, load5, load15, available_memory_kb: availableKB };
      for (const [metric, rawValue] of Object.entries(values)) {
        const value = Number(rawValue);
        if (stage && Number.isFinite(value)) (grouped[`${stage}\u0000${metric}`] ??= []).push(value);
      }
    }
    for (const [key, values] of Object.entries(grouped)) {
      const [stage, metric] = key.split('\u0000');
      ((resources[stage] ??= {}).host ??= {})[metric] = numericSummary(values);
    }
  }
  return resources;
}

function formatNumber(value, digits = 2) {
  return Number.isFinite(value) ? value.toFixed(digits) : '-';
}

function markdown(summary) {
  const lines = [
    '# Flash Mall 完整性能测试报告',
    '',
    `- commit: \`${summary.metadata.commit ?? 'unknown'}\``,
    `- suite: ${summary.metadata.suite ?? 'unknown'}`,
    `- environment: ${summary.metadata.environment ?? 'unknown'}`,
    `- correctness invariants: ${summary.invariants.passed ? 'passed' : 'failed'}`,
    `- required profiles: ${summary.overall_passed ? 'passed' : 'failed'}`,
    '',
    '## 阶段结果',
    '',
    '| profile | scenario | stage | target RPS | achieved QPS | p95 ms | p99 ms | success | dropped | gate |',
    '|---|---|---|---:|---:|---:|---:|---:|---:|---|',
  ];
  for (const [kind, scenarios] of Object.entries(summary.profiles)) {
    for (const [scenario, profile] of Object.entries(scenarios)) {
      for (const stage of profile.stages) {
        lines.push(`| ${kind} | ${scenario} | ${stage.stage} | ${stage.target_rps} | ${formatNumber(stage.qps)} | ${formatNumber(stage.p95_ms, 3)} | ${formatNumber(stage.p99_ms, 3)} | ${formatNumber(100 * stage.success_rate)}% | ${stage.dropped} | ${stage.passed ? 'pass' : 'fail'} |`);
      }
    }
  }

  lines.push('', '## 容量边界与瓶颈证据', '');
  const stress = summary.profiles.stress ?? {};
  for (const [scenario, profile] of Object.entries(stress)) {
    lines.push(`- ${scenario}: last passed ${profile.last_passed_target_rps || '-'} RPS; first failed ${profile.first_failed_target_rps || 'not reached'} RPS.`);
  }
  if (summary.bottlenecks.length === 0) {
    lines.push('- No failed stress stage was observed within the configured ceiling.');
  }
  for (const item of summary.bottlenecks) {
    lines.push(`- ${item.scenario} at ${item.first_failed_target_rps} RPS: achieved ${formatNumber(item.achieved_qps)} QPS, p95 ${formatNumber(item.p95_ms, 3)} ms; dominant step ${item.dominant_step || 'unknown'}; ${item.signals.join('; ')}.`);
  }

  lines.push('', '## 基准重复性', '');
  for (const [scenario, profile] of Object.entries(summary.profiles.baseline ?? {})) {
    lines.push(`- ${scenario}: median p95 ${formatNumber(profile.median_p95_ms, 3)} ms, median p99 ${formatNumber(profile.median_p99_ms, 3)} ms, p95 spread ${formatNumber(profile.p95_spread_ms, 3)} ms.`);
  }
  const stability = summary.resource_gates?.stability;
  lines.push('', '## 稳定性资源门禁', '');
  lines.push(`- resource sampling: ${stability?.observed ? 'observed' : 'missing'}; gate: ${stability?.passed ? 'pass' : 'fail'}.`);
  for (const violation of stability?.violations ?? []) lines.push(`- ${violation}.`);
  if (summary.violations.length > 0) lines.push('', `Violations: ${summary.violations.join(', ')}`);
  lines.push('', '> 压力档失败用于确定容量边界，不会单独导致套件失败；基准、负载、稳定性、恢复或业务不变量失败才会阻断结果。', '');
  return lines.join('\n');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const [inputDir, summaryPath, reportPath] = process.argv.slice(2);
  if (!inputDir || !summaryPath || !reportPath) {
    throw new Error('usage: summarize-performance.mjs INPUT_DIR SUMMARY_JSON REPORT_MD');
  }
  const stageDir = `${inputDir}/stages`;
  const results = readdirSync(stageDir)
    .filter((name) => name.endsWith('.json'))
    .map((name) => JSON.parse(readFileSync(`${stageDir}/${name}`, 'utf8')));
  const invariants = JSON.parse(readFileSync(`${inputDir}/invariants.json`, 'utf8'));
  const metadata = JSON.parse(readFileSync(`${inputDir}/metadata.json`, 'utf8'));
  const summary = summarizePerformance({ results, invariants, metadata, resources: readResources(inputDir) });
  writeFileSync(summaryPath, `${JSON.stringify(summary, null, 2)}\n`);
  writeFileSync(reportPath, markdown(summary));
}
