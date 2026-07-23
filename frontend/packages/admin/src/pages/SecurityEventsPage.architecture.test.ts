import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('SecurityEventsPage module boundary', () => {
  it('delegates event modeling, filters and columns to security components', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/pages/SecurityEventsPage.tsx'), 'utf8');

    expect(source).toContain("from '../components/security/SecurityEventFilters'");
    expect(source).toContain("from '../components/security/securityEventColumns'");
    expect(source).toContain("from '../components/security/securityEventModel'");
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(170);
  });
});
