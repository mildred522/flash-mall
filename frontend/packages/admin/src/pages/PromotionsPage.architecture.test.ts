import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('PromotionsPage module boundary', () => {
  it('delegates columns and modal rendering to promotion components', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/pages/PromotionsPage.tsx'), 'utf8');

    expect(source).toContain("from '../components/promotions/PromotionEditorModal'");
    expect(source).toContain("from '../components/promotions/PromotionDetailModal'");
    expect(source).toContain("from '../components/promotions/promotionColumns'");
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(320);
  });
});
