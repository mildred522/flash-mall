import { describe, expect, it } from 'vitest';
import { parseShopRoute } from './App';

describe('parseShopRoute', () => {
  it('recognizes the public shop routes', () => {
    expect(parseShopRoute('/')).toEqual({ page: 'shop' });
    expect(parseShopRoute('/shop')).toEqual({ page: 'shop' });
    expect(parseShopRoute('/orders')).toEqual({ page: 'orders' });
    expect(parseShopRoute('/product/100')).toEqual({ page: 'product', productId: 100 });
    expect(parseShopRoute('/store/1000')).toEqual({ page: 'store', merchantId: 1000 });
  });

  it('falls back to the shop for malformed paths', () => {
    expect(parseShopRoute('/product/not-a-number')).toEqual({ page: 'shop' });
    expect(parseShopRoute('/unknown')).toEqual({ page: 'shop' });
  });
});
