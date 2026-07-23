import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('OrdersPage module boundary', () => {
  it('delegates columns and modal rendering to order components', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/pages/OrdersPage.tsx'), 'utf8');

    expect(source).toContain("from '../components/orders/OrderDetailModal'");
    expect(source).toContain("from '../components/orders/OrderStatusLogsModal'");
    expect(source).toContain("from '../components/orders/orderColumns'");
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(300);
  });
});
