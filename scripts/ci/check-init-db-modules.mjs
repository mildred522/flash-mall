import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const modules = [
  'scripts/k8s/sql/00-bootstrap.sql',
  'scripts/k8s/sql/10-order.sql',
  'scripts/k8s/sql/20-product-schema.sql',
  'scripts/k8s/sql/21-product-seed.sql',
  'scripts/k8s/sql/30-auth-schema.sql',
  'scripts/k8s/sql/31-auth-seed.sql',
];

const generated = modules.map((path) => readFileSync(resolve(root, path), 'utf8')).join('');
const aggregate = readFileSync(resolve(root, 'scripts/k8s/init-db.sql'), 'utf8');

if (generated !== aggregate) {
  throw new Error('scripts/k8s/init-db.sql is stale; run node scripts/k8s/build-init-db.mjs');
}

for (const path of modules) {
  const lines = readFileSync(resolve(root, path), 'utf8').split(/\r?\n/).length - 1;
  if (lines > 400) throw new Error(`${path} is still too large (${lines} lines)`);
}

console.log(`init-db modules verified: ${modules.length} files`);
