import { readFileSync } from 'node:fs';

import { performanceStagePassed } from './summarize-performance.mjs';

const [reportPath] = process.argv.slice(2);
if (!reportPath) throw new Error('usage: check-stage-performance.mjs STAGE_JSON');
const result = JSON.parse(readFileSync(reportPath, 'utf8'));
process.exit(performanceStagePassed(result.report) ? 0 : 1);
