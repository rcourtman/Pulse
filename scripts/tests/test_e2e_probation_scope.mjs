import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

import { requiresProbationRun } from '../../.github/scripts/e2e-probation-scope.mjs';
import { PROBATION_SPECS } from '../../tests/integration/e2e-tiering.mjs';

const probation = ['**/probation.spec.ts', '**/journeys/serial.spec.ts'];

test('changed probation specs execute at both root and nested paths', () => {
  for (const file of ['probation.spec.ts', 'nested/probation.spec.ts', 'journeys/serial.spec.ts']) {
    assert.equal(requiresProbationRun([`tests/integration/tests/${file}`], probation), true);
  }
});

test('default selection consumes the current probation manifest', () => {
  for (const pattern of PROBATION_SPECS) {
    const file = pattern.slice(3);
    assert.equal(requiresProbationRun([`tests/integration/tests/${file}`]), true);
  }
  assert.equal(requiresProbationRun([]), false);
});

test('stable-only specs, product changes and unrelated matching filenames keep their existing scope', () => {
  for (const file of ['tests/integration/tests/stable.spec.ts', 'frontend-modern/src/App.tsx', 'other/probation.spec.ts']) {
    assert.equal(requiresProbationRun([file], probation), false);
  }
  assert.equal(requiresProbationRun([], probation), false);
});

test('tier, workflow and shared fixture changes require probation coverage', () => {
  for (const file of ['tests/integration/e2e-tiering.mjs', 'tests/integration/playwright.config.ts', '.github/workflows/test-e2e.yml', 'tests/integration/tests/helpers.ts']) {
    assert.equal(requiresProbationRun([file], probation), true);
  }
});

test('unsupported manifest patterns fail admission', () => {
  for (const pattern of ['*.spec.ts', '**/*.spec.ts', '**/choice?.spec.ts', '**/[ab].spec.ts']) {
    assert.throws(() => requiresProbationRun([], [pattern]), /Unsupported E2E probation pattern/);
  }
});

const cli = fileURLToPath(new URL('../../.github/scripts/e2e-probation-scope.mjs', import.meta.url));
for (const [name, input, expected] of [
  ['a configuration path among NUL-delimited files', 'README.md\0tests/integration/e2e-tiering.mjs\0', 'true\n'],
  ['a newline within one filename', 'README.md\n.github/workflows/test-e2e.yml\0', 'false\n'],
  ['an empty diff', '', 'false\n'],
]) {
  test(`CLI handles ${name}`, () => {
    const result = spawnSync(process.execPath, [cli], { input, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, expected);
  });
}
