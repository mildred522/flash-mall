import { describe, expect, it } from 'vitest';
import { resolveProductImage } from '@flash-mall/shared';

describe('resolveProductImage', () => {
  it('prefers an explicit product image URL', () => {
    expect(resolveProductImage(100, ' /uploads/products/custom.webp ')).toBe('/uploads/products/custom.webp');
  });

  it('falls back to the bundled demo image', () => {
    expect(resolveProductImage(100, '')).toBe('/products/100.svg');
  });

  it('returns an empty URL for an unknown product', () => {
    expect(resolveProductImage(999, '')).toBe('');
  });
});
