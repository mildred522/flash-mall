import { spawnSync } from 'node:child_process';
import { existsSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join, resolve } from 'node:path';

const repositoryRoot = resolve(import.meta.dirname, '../..');
const files = process.argv.slice(2);
const targets = files.length > 0
  ? files.map((file) => resolve(file))
  : [
      resolve(repositoryRoot, 'artifacts/web/shop.html'),
      resolve(repositoryRoot, 'artifacts/web/admin.html'),
      resolve(repositoryRoot, 'artifacts/web/merchant.html'),
    ];

let failed = false;
const autocompleteContracts = new Map([
  ['frontend/packages/admin/src/pages/LoginPage.tsx', ['autoComplete="username"', 'autoComplete="current-password"']],
  ['frontend/packages/merchant/src/pages/LoginPage.tsx', ['autoComplete="username"', 'autoComplete="current-password"', 'autoComplete="one-time-code"', 'autoComplete="new-password"']],
  ['frontend/packages/shop/src/components/AuthModal.tsx', ['autoComplete="username"', 'autoComplete="current-password"', 'autoComplete="one-time-code"', 'autoComplete="new-password"']],
]);
for (const [path, expectedTokens] of autocompleteContracts) {
  const source = readFileSync(resolve(repositoryRoot, path), 'utf8');
  for (const token of expectedTokens) {
    if (!source.includes(token)) {
      console.error(`[FAIL] ${path}: missing ${token}`);
      failed = true;
    }
  }
}

for (const legacyPath of [
  resolve(repositoryRoot, 'web/package.json'),
  resolve(repositoryRoot, 'app/entry/api/internal/handler/web/admin.html'),
]) {
  if (existsSync(legacyPath)) {
    console.error(`[FAIL] legacy frontend path still exists: ${legacyPath}`);
    failed = true;
  }
}

for (const target of targets) {
  let html;
  try {
    html = readFileSync(target, 'utf8');
  } catch (error) {
    console.error(`[FAIL] ${target}: ${error.code === 'ENOENT' ? 'artifact is missing' : error.message}`);
    failed = true;
    continue;
  }

  let targetFailed = false;
  if (!/<link\b[^>]*\brel=["']icon["'][^>]*>/i.test(html)) {
    console.error(`[FAIL] ${target}: favicon declaration is missing`);
    failed = true;
    targetFailed = true;
  }
  const scripts = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/gi)]
    .filter((match) => !/\bsrc\s*=/.test(match[1]));

  if (scripts.length === 0) {
    console.error(`[FAIL] ${target}: no inline scripts found`);
    failed = true;
    targetFailed = true;
    continue;
  }

  for (const [index, match] of scripts.entries()) {
    const temporary = join(
      tmpdir(),
      `flash-mall-${process.pid}-${basename(target)}-${index}.mjs`,
    );
    try {
      writeFileSync(temporary, match[2], 'utf8');
      const result = spawnSync(process.execPath, ['--check', temporary], {
        encoding: 'utf8',
      });
      if (result.status !== 0) {
        console.error(`[FAIL] ${target}: inline script ${index} is invalid`);
        console.error((result.stderr || result.stdout).trim());
        failed = true;
        targetFailed = true;
      }
    } finally {
      rmSync(temporary, { force: true });
    }
  }

  if (!targetFailed) {
    console.log(`[OK] ${target}: ${scripts.length} inline script(s) parsed`);
  }
}

if (failed) {
  process.exitCode = 1;
}
