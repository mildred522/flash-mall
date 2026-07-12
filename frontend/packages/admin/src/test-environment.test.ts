import { describe, expect, it } from 'vitest';

describe('admin test environment', () => {
  it('provides a browser document', () => {
    expect(document.createElement('div')).toBeInstanceOf(HTMLDivElement);
  });
});
