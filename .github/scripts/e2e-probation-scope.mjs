import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

import { PROBATION_SPECS } from '../../tests/integration/e2e-tiering.mjs';

const CONFIGURATION_PATHS = new Set([
  '.github/workflows/test-e2e.yml',
  '.github/scripts/e2e-probation-scope.mjs',
  'scripts/tests/test_e2e_probation_scope.mjs',
  'tests/integration/e2e-tiering.mjs',
  'tests/integration/playwright.config.ts',
  'tests/integration/package.json',
  'tests/integration/package-lock.json',
  'tests/integration/docker-compose.test.yml',
  'tests/integration/scripts/check-e2e-tiering.mjs',
  'tests/integration/scripts/pretest.mjs',
  'tests/integration/scripts/run-playwright.mjs',
  'tests/integration/tests/helpers.ts',
]);
const SPEC_ROOT = 'tests/integration/tests/';

export function requiresProbationRun(changedPaths, probationSpecs = PROBATION_SPECS) {
  // The tier manifest names literal spec paths under a recursive prefix.
  // Reject unsupported patterns before they can silently omit coverage.
  const suffixes = probationSpecs.map((pattern) => {
    const suffix = pattern.slice(3);
    if (!pattern.startsWith('**/') || ['*', '?', '[', '{'].some((symbol) => suffix.includes(symbol))) {
      throw new Error(`Unsupported E2E probation pattern: ${pattern}`);
    }
    return suffix;
  });
  return changedPaths.some((file) => {
    if (CONFIGURATION_PATHS.has(file)) return true;
    if (!file.startsWith(SPEC_ROOT)) return false;
    const relative = file.slice(SPEC_ROOT.length);
    return suffixes.some((suffix) => relative === suffix || relative.endsWith(`/${suffix}`));
  });
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const changedPaths = readFileSync(0, 'utf8').split('\0').filter(Boolean);
  process.stdout.write(`${requiresProbationRun(changedPaths)}\n`);
}
