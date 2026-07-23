import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('SuppliersPage module boundary', () => {
  it('delegates columns and modal rendering to supplier components', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/pages/SuppliersPage.tsx'), 'utf8');

    expect(source).toContain("from '../components/suppliers/SupplierEditorModal'");
    expect(source).toContain("from '../components/suppliers/SupplierDetailModal'");
    expect(source).toContain("from '../components/suppliers/supplierColumns'");
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(280);
  });
});
