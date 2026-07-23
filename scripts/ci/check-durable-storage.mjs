import { readFileSync } from 'node:fs';

const compose = readFileSync(new URL('../../deploy/docker-compose.yml', import.meta.url), 'utf8');

const requirements = [
  ['uploads use an explicit Docker volume', /flash-mall-uploads:\s*\n\s+name:\s*flash-mall-uploads/],
  ['gateway mounts the durable upload volume', /flash-mall-uploads:\/data\/flash-mall\/uploads/],
  ['uploads are not tied to a worktree runtime directory', /(?<!\S)\.\.\/\.runtime\/uploads/],
  ['MySQL server defaults to utf8mb4', /--character-set-server=utf8mb4/],
  ['MySQL server uses an utf8mb4 collation', /--collation-server=utf8mb4_unicode_ci/],
];

for (const [description, pattern] of requirements) {
  const matched = pattern.test(compose);
  const negativeRequirement = description.includes('not tied');
  if (negativeRequirement ? matched : !matched) {
    throw new Error(`durable storage check failed: ${description}`);
  }
}

console.log('durable storage configuration verified');
