import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const scriptPath = resolve(root, 'scripts/local/verify-failure-recovery.sh');

if (!existsSync(scriptPath)) {
  throw new Error('scripts/local/verify-failure-recovery.sh is missing');
}

const script = readFileSync(scriptPath, 'utf8');
const requirements = [
  ['explicit disruption opt-in', /--allow-disruption/],
  ['exit cleanup trap', /trap cleanup EXIT/],
  ['container ownership check', /com\.docker\.compose\.project/],
  ['pause injection', /docker pause/],
  ['unpause recovery', /docker unpause/],
  ['gateway readiness evidence', /api\/system\/health/],
  ['Prometheus target evidence', /api\/v1\/query/],
  ['isolated Outbox probe', /chaos\.probe/],
  ['Outbox cleanup', /DELETE FROM order_outbox/],
];

for (const [name, pattern] of requirements) {
  if (!pattern.test(script)) {
    throw new Error(`failure recovery verifier lacks ${name}`);
  }
}

console.log(`Failure recovery safety contract verified: ${requirements.length} invariant(s)`);
