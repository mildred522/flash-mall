import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const schemaModules = [
  'scripts/k8s/sql/00-bootstrap.sql',
  'scripts/k8s/sql/05-dtm.sql',
  'scripts/k8s/sql/10-order.sql',
  'scripts/k8s/sql/11-payment-provider-migrations.sql',
  'scripts/k8s/sql/20-product-schema.sql',
  'scripts/k8s/sql/22-data-repair.sql',
  'scripts/k8s/sql/30-auth-schema.sql',
  'scripts/k8s/sql/31-auth-migrations.sql',
];
const demoModules = [
  'scripts/k8s/sql/40-demo-order.sql',
  'scripts/k8s/sql/41-demo-product.sql',
  'scripts/k8s/sql/42-demo-auth.sql',
];

const generatedSchema = schemaModules.map((path) => readFileSync(resolve(root, path), 'utf8')).join('');
const generatedDemo = demoModules.map((path) => readFileSync(resolve(root, path), 'utf8')).join('');
const schema = readFileSync(resolve(root, 'scripts/k8s/schema.sql'), 'utf8');
const demo = readFileSync(resolve(root, 'scripts/k8s/demo-seed.sql'), 'utf8');
const demoApply = readFileSync(resolve(root, 'scripts/k8s/apply-demo-seed.sh'), 'utf8');
if (existsSync(resolve(root, 'scripts/k8s/init-db.sql'))) {
  throw new Error('legacy scripts/k8s/init-db.sql must not mix schema and demo data');
}

if (generatedSchema !== schema || generatedDemo !== demo) {
  throw new Error('generated SQL bundles are stale; run node scripts/k8s/build-sql-bundles.mjs');
}

for (const path of [...schemaModules, ...demoModules]) {
  const lines = readFileSync(resolve(root, path), 'utf8').split(/\r?\n/).length - 1;
  if (lines > 400) throw new Error(`${path} is still too large (${lines} lines)`);
}

for (const forbidden of ['13800000001', '山岚烘焙研究所', '桂花乌龙手工曲奇']) {
  if (schema.includes(forbidden)) {
    throw new Error(`schema bundle contains demo fixture: ${forbidden}`);
  }
}
if (!demo.includes('20260730_demo_fixture_v1')) {
  throw new Error('demo bundle must carry an explicit fixture version');
}
if (demo.lastIndexOf('INSERT INTO demo_fixture_state') < demo.lastIndexOf('INSERT INTO user_credentials')) {
  throw new Error('demo fixture version must be recorded only after all domain fixtures succeed');
}
for (const productID of [100, 101, 102, 103, 104, 201, 202, 211, 212]) {
  const snapshotPattern = new RegExp(
    `\\(${productID},\\s*\\d+,\\s*0,\\s*\\d+,\\s*'demo-seed',\\s*1\\)`,
    'i',
  );
  if (!snapshotPattern.test(demo)) {
    throw new Error(`demo bundle is missing product_stock_snapshot for product ${productID}`);
  }
}
for (const adoptionGuard of [
  'user_credentials',
  'user_identities',
  'merchant_user',
  'merchant_store_profile',
  'product_stock_snapshot',
  'product_stock_bucket',
]) {
  if (!demoApply.includes(adoptionGuard)) {
    throw new Error(`demo adoption must validate ${adoptionGuard} before recording the fixture version`);
  }
}

const dataRepair = readFileSync(resolve(root, 'scripts/k8s/sql/22-data-repair.sql'), 'utf8');
if (!dataRepair.includes('schema_migrations') || !dataRepair.includes('20260723_order_snapshot_utf8_asset_repair')) {
  throw new Error('22-data-repair.sql must be guarded by a versioned schema migration');
}

console.log(`SQL bundles verified: ${schemaModules.length} schema modules, ${demoModules.length} demo modules`);
