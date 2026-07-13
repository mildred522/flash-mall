import { spawnSync } from 'node:child_process';
import { readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join, resolve } from 'node:path';

const repositoryRoot = resolve(import.meta.dirname, '../..');
const files = process.argv.slice(2);
const targets = files.length > 0
  ? files.map((file) => resolve(file))
  : [
      resolve(repositoryRoot, 'app/entry/api/internal/handler/web/shop.html'),
      resolve(repositoryRoot, 'app/entry/api/internal/handler/web/admin.html'),
    ];

let failed = false;
for (const target of targets) {
  const html = readFileSync(target, 'utf8');
  const scripts = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/gi)]
    .filter((match) => !/\bsrc\s*=/.test(match[1]));

  if (scripts.length === 0) {
    console.error(`[FAIL] ${target}: no inline scripts found`);
    failed = true;
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
      }
    } finally {
      rmSync(temporary, { force: true });
    }
  }

  if (!failed) {
    console.log(`[OK] ${target}: ${scripts.length} inline script(s) parsed`);
  }
}

if (failed) {
  process.exitCode = 1;
}
