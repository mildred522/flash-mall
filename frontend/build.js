import { execSync } from 'child_process';
import { copyFileSync, mkdirSync, existsSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(__dirname, '../app/entry/api/internal/handler/web');

console.log('[build] Building shop...');
execSync('npm run build -w packages/shop', { cwd: __dirname, stdio: 'inherit' });

console.log('[build] Building admin...');
execSync('npm run build -w packages/admin', { cwd: __dirname, stdio: 'inherit' });

if (!existsSync(webDir)) mkdirSync(webDir, { recursive: true });

copyFileSync(resolve(__dirname, 'packages/shop/dist/index.html'), resolve(webDir, 'shop.html'));
console.log('[build] Copied shop.html');

copyFileSync(resolve(__dirname, 'packages/admin/dist/index.html'), resolve(webDir, 'admin.html'));
console.log('[build] Copied admin.html');

console.log('[build] Done!');
