import { readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const workflowDir = resolve(root, '.github/workflows');
const minimumMajors = new Map([
  ['actions/checkout', 6],
  ['actions/setup-node', 5],
  ['actions/setup-dotnet', 5],
  ['dorny/paths-filter', 4],
]);

let checked = 0;
for (const filename of readdirSync(workflowDir).filter((name) => /\.ya?ml$/.test(name))) {
  const workflowPath = `.github/workflows/${filename}`;
  const workflow = readFileSync(resolve(workflowDir, filename), 'utf8');

  for (const match of workflow.matchAll(/uses:\s*([^@\s]+)@v(\d+)/g)) {
    const action = match[1];
    const minimumMajor = minimumMajors.get(action);
    if (!minimumMajor) continue;

    checked += 1;
    const actualMajor = Number(match[2]);
    if (actualMajor < minimumMajor) {
      throw new Error(
        `${workflowPath} uses ${action}@v${actualMajor}; v${minimumMajor}+ is required for the Node 24 action runtime`,
      );
    }
  }
}

if (checked === 0) {
  throw new Error('no guarded GitHub Actions were found');
}

console.log(`GitHub Action runtimes verified: ${checked} guarded use(s)`);
