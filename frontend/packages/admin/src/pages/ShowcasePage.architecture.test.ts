import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('ShowcasePage module boundary', () => {
  it('delegates slots, candidates and draft rules to showcase components', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/pages/ShowcasePage.tsx'), 'utf8');

    expect(source).toContain("from '../components/showcase/ShowcaseSlots'");
    expect(source).toContain("from '../components/showcase/ShowcaseCandidates'");
    expect(source).toContain("from '../components/showcase/showcaseModel'");
    expect(source.split(/\r?\n/).length).toBeLessThanOrEqual(180);
  });
});
