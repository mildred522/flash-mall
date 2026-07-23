import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('shared type module boundaries', () => {
  it('keeps the public types entrypoint as a domain barrel', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/types.ts'), 'utf8');

    for (const domain of ['common', 'auth', 'catalog', 'order', 'merchant', 'admin-order', 'admin-product', 'admin-catalog']) {
      expect(source).toContain(`export * from './types/${domain}';`);
    }
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(12);
  });
});
