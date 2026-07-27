import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const serviceDockerfiles = [
  'build/docker/auth-api.Dockerfile',
  'build/docker/entry-api.Dockerfile',
  'build/docker/hertz-gateway.Dockerfile',
  'build/docker/inventory-kitex.Dockerfile',
  'build/docker/order-rpc.Dockerfile',
  'build/docker/product-rpc.Dockerfile',
];

for (const dockerfile of serviceDockerfiles) {
  const content = readFileSync(resolve(root, dockerfile), 'utf8');
  if (!/^COPY app\/common \.\/app\/common$/m.test(content)) {
    throw new Error(`${dockerfile} must copy app/common into the Go build context`);
  }
}

console.log(`Docker build contexts verified: ${serviceDockerfiles.length} service image(s)`);
