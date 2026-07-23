import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('UsersPage module boundary', () => {
  it('delegates columns, detail modal and presentation rules to user components', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/pages/UsersPage.tsx'), 'utf8');

    expect(source).toContain("from '../components/users/UserDetailModal'");
    expect(source).toContain("from '../components/users/userColumns'");
    expect(source).toContain("from '../components/users/userModel'");
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(170);
  });
});
