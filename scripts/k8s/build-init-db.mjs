import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const modules = [
  'scripts/k8s/sql/00-bootstrap.sql',
  'scripts/k8s/sql/10-order.sql',
  'scripts/k8s/sql/20-product-schema.sql',
  'scripts/k8s/sql/21-product-seed.sql',
  'scripts/k8s/sql/22-data-repair.sql',
  'scripts/k8s/sql/30-auth-schema.sql',
  'scripts/k8s/sql/31-auth-seed.sql',
];

const output = modules.map((path) => readFileSync(resolve(root, path), 'utf8')).join('');
writeFileSync(resolve(root, 'scripts/k8s/init-db.sql'), output, 'utf8');
console.log(`generated scripts/k8s/init-db.sql from ${modules.length} modules`);
