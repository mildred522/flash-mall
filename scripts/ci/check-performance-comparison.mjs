import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const scriptPath = resolve(root, 'scripts/perf/compare-entry-hertz.sh');
const dockerfilePath = resolve(root, 'build/docker/entry-api-baseline-perf.Dockerfile');
const baselineResultPath = resolve(root, 'benchmarks/results/entry-hertz-20260729.json');

if (!existsSync(scriptPath)) {
  throw new Error('scripts/perf/compare-entry-hertz.sh is missing');
}

const script = readFileSync(scriptPath, 'utf8');
for (const [name, pattern] of [
  ['origin/main tree guard', /origin\/main\^\{tree\}/],
  ['baseline worktree guard', /FLASH_MALL_BASELINE_ROOT/],
  ['read-only shared route', /\/api\/shop\/catalog/],
  ['response contract check', /validate_catalog/],
  ['alternating sample order', /entry hertz|hertz entry/],
  ['temporary baseline container', /entry-api-perf-baseline/],
  ['cleanup trap', /trap cleanup EXIT/],
  ['raw JSON reports', /tools\/httpbench/],
  ['summary report', /summarize-comparison\.mjs/],
]) {
  if (!pattern.test(script)) {
    throw new Error(`performance comparison lacks ${name}`);
  }
}

if (!existsSync(dockerfilePath)) {
  throw new Error('build/docker/entry-api-baseline-perf.Dockerfile is missing');
}

if (!existsSync(baselineResultPath)) {
  throw new Error('frozen Entry/Hertz comparison result is missing');
}
const baselineResult = JSON.parse(readFileSync(baselineResultPath, 'utf8'));
if (baselineResult.baseline?.branch !== 'origin/main' ||
    baselineResult.candidate?.branch !== 'codex/arch-hertz-kitex' ||
    baselineResult.experiment?.minimum_success_rate !== 1 ||
    baselineResult.assessment?.status !== 'directionally_repeatable_but_p95_cv_above_threshold') {
  throw new Error('frozen Entry/Hertz comparison result has lost its provenance or stability caveat');
}
const dockerfile = readFileSync(dockerfilePath, 'utf8');
if (!/id=flash-mall-go-mod,target=\/go\/pkg\/mod/.test(dockerfile) ||
    !/id=flash-mall-go-build,target=\/root\/\.cache\/go-build/.test(dockerfile)) {
  throw new Error('baseline performance image must reuse shared Go build caches');
}

console.log('Performance comparison contract verified: origin/main vs Hertz, read-only alternating samples');
