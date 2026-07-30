import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const bundles = {
  'schema.sql': [
  'scripts/k8s/sql/00-bootstrap.sql',
  'scripts/k8s/sql/10-order.sql',
  'scripts/k8s/sql/11-payment-provider-migrations.sql',
  'scripts/k8s/sql/20-product-schema.sql',
  'scripts/k8s/sql/22-data-repair.sql',
  'scripts/k8s/sql/30-auth-schema.sql',
  'scripts/k8s/sql/31-auth-migrations.sql',
  ],
  'demo-seed.sql': [
    'scripts/k8s/sql/40-demo-order.sql',
    'scripts/k8s/sql/41-demo-product.sql',
    'scripts/k8s/sql/42-demo-auth.sql',
  ],
};

for (const [file, modules] of Object.entries(bundles)) {
  const output = modules.map((path) => readFileSync(resolve(root, path), 'utf8')).join('');
  writeFileSync(resolve(root, `scripts/k8s/${file}`), output, 'utf8');
  console.log(`generated scripts/k8s/${file} from ${modules.length} modules`);
}
