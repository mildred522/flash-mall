import assert from 'node:assert/strict';
import test from 'node:test';

import { assertLocalMutationTarget } from './local-mutation-guard.mjs';

test('requires an explicit mutation flag', () => {
  assert.throws(
    () => assertLocalMutationTarget('http://127.0.0.1:8889', false),
    /--allow-mutation/,
  );
});

test('accepts loopback targets after explicit confirmation', () => {
  assert.equal(
    assertLocalMutationTarget('http://127.0.0.1:8889', true).origin,
    'http://127.0.0.1:8889',
  );
  assert.equal(
    assertLocalMutationTarget('http://localhost:8889', true).origin,
    'http://localhost:8889',
  );
});

test('rejects non-local and credentialed targets', () => {
  assert.throws(
    () => assertLocalMutationTarget('https://mall.example.com', true),
    /loopback/,
  );
  assert.throws(
    () => assertLocalMutationTarget('http://user:pass@127.0.0.1:8889', true),
    /credentials/,
  );
});
