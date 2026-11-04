import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { parsePlaywrightTestIdentity } from './e2e-tier-identity.mjs';

const root = fileURLToPath(new URL('..', import.meta.url));
const playwright = path.join(root, 'node_modules', '.bin', 'playwright');

function list(shard) {
  const env = { ...process.env, PULSE_E2E_TIER: 'probation' };
  delete env.PULSE_E2E_PROBATION_SHARD;
  if (shard) env.PULSE_E2E_PROBATION_SHARD = shard;
  const result = spawnSync(playwright, ['test', '--list', '--pass-with-no-tests'], {
    cwd: root,
    env,
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
  });
  if (result.status !== 0) throw new Error(`Probation listing failed for ${shard || 'all'}: ${result.stderr || result.stdout}`);
  return parseListedTests(result.stdout, shard || 'all');
}

export function parseListedTests(output, label) {
  const keys = [];
  for (const line of output.split('\n')) {
    const identity = parsePlaywrightTestIdentity(line);
    if (identity) keys.push(identity.key);
  }
  const announced = output.match(/Total: (\d+) tests? in \d+ files?/);
  if (!announced || Number(announced[1]) !== keys.length) {
    throw new Error(`Incomplete Playwright identity parsing for ${label}`);
  }
  if (new Set(keys).size !== keys.length) throw new Error(`Duplicate listing in ${label}`);
  return keys;
}

export function validateProbationAllocation(expected, allocations) {
  const seen = new Set();
  for (const keys of allocations) {
    for (const key of keys) {
      if (!expected.has(key)) throw new Error(`Unexpected probation case: ${key}`);
      if (seen.has(key)) throw new Error(`Probation case allocated twice: ${key}`);
      seen.add(key);
    }
  }
  const missing = [...expected].filter((key) => !seen.has(key));
  if (missing.length) throw new Error(`Unallocated probation cases: ${missing.join(', ')}`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  const expected = new Set(list(null));
  const allocations = Array.from({ length: 8 }, (_, index) => {
    const keys = list(`${index + 1}/8`);
    console.log(`Probation shard ${index + 1}/8: ${keys.length} test/project instances`);
    return keys;
  });
  validateProbationAllocation(expected, allocations);
  console.log(`Probation allocation covers all ${expected.size} test/project instances exactly once`);
}
