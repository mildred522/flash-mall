import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('StoreSettingsPage module boundary', () => {
  it('delegates form and preview rendering to store components', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/pages/StoreSettingsPage.tsx'), 'utf8');

    expect(source).toContain("from '../components/store/StoreProfileForm'");
    expect(source).toContain("from '../components/store/StorePreview'");
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(150);
  });
});
