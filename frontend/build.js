import { execSync } from 'child_process';
import { copyFileSync, cpSync, mkdirSync, existsSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(__dirname, '../app/entry/api/internal/handler/web');

console.log('[build] Building shop...');
execSync('npm run build -w packages/shop', { cwd: __dirname, stdio: 'inherit' });

console.log('[build] Building admin...');
execSync('npm run build -w packages/admin', { cwd: __dirname, stdio: 'inherit' });

console.log('[build] Building merchant...');
execSync('npm run build -w packages/merchant', { cwd: __dirname, stdio: 'inherit' });

if (!existsSync(webDir)) mkdirSync(webDir, { recursive: true });

copyFileSync(resolve(__dirname, 'packages/shop/dist/index.html'), resolve(webDir, 'shop.html'));
console.log('[build] Copied shop.html');

const productAssets = resolve(__dirname, 'packages/shop/dist/products');
if (existsSync(productAssets)) {
  cpSync(productAssets, resolve(webDir, 'products'), { recursive: true, force: true });
  console.log('[build] Copied bundled product images');
}

copyFileSync(resolve(__dirname, 'packages/admin/dist/index.html'), resolve(webDir, 'admin.html'));
console.log('[build] Copied admin.html');

copyFileSync(resolve(__dirname, 'packages/merchant/dist/index.html'), resolve(webDir, 'merchant.html'));
console.log('[build] Copied merchant.html');

console.log('[build] Done!');
