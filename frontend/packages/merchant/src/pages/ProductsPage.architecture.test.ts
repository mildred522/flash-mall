import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('Merchant ProductsPage module boundary', () => {
  it('delegates columns and modal rendering to product components', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/pages/ProductsPage.tsx'), 'utf8');

    expect(source).toContain("from '../components/products/MerchantProductEditorModal'");
    expect(source).toContain("from '../components/products/StockAdjustModal'");
    expect(source).toContain("from '../components/products/productColumns'");
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(170);
  });
});
