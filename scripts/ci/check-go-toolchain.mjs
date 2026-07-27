import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const goMod = readFileSync(resolve(root, 'go.mod'), 'utf8');
const toolchain = goMod.match(/^toolchain\s+(go\S+)$/m)?.[1];

if (!toolchain) {
  throw new Error('go.mod must pin the CI toolchain with a toolchain directive');
}

const workflowPaths = [
  '.github/workflows/ci.yml',
  '.github/workflows/full-ci.yml',
];

let setupSteps = 0;
for (const workflowPath of workflowPaths) {
  const workflow = readFileSync(resolve(root, workflowPath), 'utf8');
  const setupGoUses = [...workflow.matchAll(/uses:\s*actions\/setup-go@v(\d+)/g)];
  setupSteps += setupGoUses.length;

  for (const match of setupGoUses) {
    const major = Number(match[1]);
    if (major < 6) {
      throw new Error(
        `${workflowPath} uses setup-go@v${major}; setup-go@v6+ is required to honor ${toolchain}`,
      );
    }
  }

  const goVersionFiles = workflow.match(/go-version-file:\s*go\.mod/g)?.length ?? 0;
  const dependencyPaths = workflow.match(/cache-dependency-path:\s*go\.sum/g)?.length ?? 0;
  if (goVersionFiles !== setupGoUses.length || dependencyPaths !== setupGoUses.length) {
    throw new Error(
      `${workflowPath} must configure go-version-file: go.mod and cache-dependency-path: go.sum for every setup-go step`,
    );
  }
}

if (setupSteps === 0) {
  throw new Error('no setup-go steps found in CI workflows');
}

console.log(`Go CI toolchain verified: ${toolchain}, ${setupSteps} setup step(s)`);
