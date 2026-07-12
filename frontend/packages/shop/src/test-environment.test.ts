import { describe, expect, it } from 'vitest';

describe('shop test environment', () => {
  it('provides a browser document', () => {
    expect(document.createElement('div')).toBeInstanceOf(HTMLDivElement);
  });
});
