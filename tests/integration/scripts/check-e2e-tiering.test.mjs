import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import {
  listPlaywrightTests,
  validateE2ETiering,
} from './check-e2e-tiering-lib.mjs';

const canonicalLedger = path.resolve(
  fileURLToPath(new URL('../e2e-tiering.mjs', import.meta.url)),
);

const cliSource = `
import fs from 'node:fs';

const mode = process.env.PULSE_TIER_SMOKE_MODE;
if (mode === 'nonzero') {
  console.error('fixture Playwright failure');
  process.exit(23);
}
if (mode === 'signal') {
  console.error('fixture Playwright termination');
  process.kill(process.pid, 'SIGTERM');
}

const marker = process.env.PULSE_TIER_SMOKE_MARKER;
if (marker) {
  fs.appendFileSync(marker, JSON.stringify({ argv: process.argv, execPath: process.execPath }) + '\\n');
}

const tier = process.env.PULSE_E2E_TIER;
const files = fs.readdirSync('tests').filter((name) => name.endsWith('.spec.ts')).sort();
const selected = tier === 'probation' ? [] : files;
if (mode === 'omit-stable' && tier === 'stable') selected.pop();
for (const file of selected) console.log('[chromium] › ' + file + ':1:1 › fixture test');
`;

async function createFixture({
  fileCount = 10,
  ledger = 'export const PROBATION_SPECS = [];\nexport const QUARANTINED_SPECS = [];\n',
} = {}) {
  const integrationRoot = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pulse tier checker space-'),
  );
  await fs.mkdir(path.join(integrationRoot, 'tests'), { recursive: true });
  await fs.mkdir(path.join(integrationRoot, 'node_modules', 'playwright'), {
    recursive: true,
  });
  await fs.writeFile(path.join(integrationRoot, 'e2e-tiering.mjs'), ledger);
  await fs.writeFile(
    path.join(integrationRoot, 'node_modules', 'playwright', 'cli.js'),
    cliSource,
  );
  await Promise.all(
    Array.from({ length: fileCount }, (_, index) =>
      fs.writeFile(
        path.join(
          integrationRoot,
          'tests',
          `fixture-${String(index).padStart(2, '0')}.spec.ts`,
        ),
        '',
      ),
    ),
  );
  return integrationRoot;
}

async function removeFixture(integrationRoot) {
  await fs.rm(integrationRoot, { recursive: true, force: true });
}

test('uses Node to launch the pinned CLI from an integration path containing spaces', async (t) => {
  const integrationRoot = await createFixture();
  const marker = path.join(integrationRoot, 'launcher.jsonl');
  const previousMarker = process.env.PULSE_TIER_SMOKE_MARKER;
  process.env.PULSE_TIER_SMOKE_MARKER = marker;
  t.after(async () => {
    if (previousMarker === undefined)
      delete process.env.PULSE_TIER_SMOKE_MARKER;
    else process.env.PULSE_TIER_SMOKE_MARKER = previousMarker;
    await removeFixture(integrationRoot);
  });

  const result = await validateE2ETiering({ integrationRoot });
  assert.deepEqual(result.errors, []);
  assert.equal(result.allFiles.size, 10);
  assert.equal(result.stableTests.size, 10);

  const launches = (await fs.readFile(marker, 'utf8'))
    .trim()
    .split('\n')
    .map((line) => JSON.parse(line));
  assert.equal(launches.length, 3);
  for (const launch of launches) {
    assert.equal(launch.execPath, process.execPath);
    assert.deepEqual(launch.argv.slice(1), [
      path.join(integrationRoot, 'node_modules', 'playwright', 'cli.js'),
      'test',
      '--list',
    ]);
  }
});

test('reports malformed fixture ledgers without changing the canonical ledger', async (t) => {
  const integrationRoot = await createFixture({
    ledger:
      "export const PROBATION_SPECS = ['not-a-glob'];\nexport const QUARANTINED_SPECS = [];\n",
  });
  const before = await fs.readFile(canonicalLedger, 'utf8');
  t.after(async () => removeFixture(integrationRoot));

  const result = await validateE2ETiering({ integrationRoot });
  assert.match(
    result.errors.join('\n'),
    /PROBATION_SPECS has invalid entries: not-a-glob/,
  );
  assert.equal(await fs.readFile(canonicalLedger, 'utf8'), before);
});

test('retains duplicate, overlap, unknown, sorting, split, and stable-floor ledger rejection', async (t) => {
  const cases = [
    {
      ledger:
        "export const PROBATION_SPECS = ['**/fixture-00.spec.ts', '**/fixture-00.spec.ts'];\nexport const QUARANTINED_SPECS = [];\n",
      expected: /PROBATION_SPECS has duplicates: \*\*\/fixture-00.spec.ts/,
      name: 'duplicate',
    },
    {
      ledger:
        "export const PROBATION_SPECS = ['**/fixture-00.spec.ts'];\nexport const QUARANTINED_SPECS = ['**/fixture-00.spec.ts'];\n",
      expected: /Probation\/quarantine overlap: fixture-00.spec.ts/,
      name: 'overlap',
    },
    {
      ledger:
        "export const PROBATION_SPECS = ['**/missing.spec.ts'];\nexport const QUARANTINED_SPECS = [];\n",
      expected: /PROBATION_SPECS has unknown specs: missing.spec.ts/,
      name: 'unknown',
    },
    {
      ledger:
        "export const PROBATION_SPECS = ['**/fixture-09.spec.ts', '**/fixture-00.spec.ts'];\nexport const QUARANTINED_SPECS = [];\n",
      expected: /PROBATION_SPECS must be sorted/,
      name: 'unsorted',
    },
  ];
  const canonicalBefore = await fs.readFile(canonicalLedger, 'utf8');
  const roots = [];
  t.after(async () => {
    await Promise.all(roots.map(removeFixture));
    assert.equal(await fs.readFile(canonicalLedger, 'utf8'), canonicalBefore);
  });

  for (const fixtureCase of cases) {
    const integrationRoot = await createFixture({ ledger: fixtureCase.ledger });
    roots.push(integrationRoot);
    const result = await validateE2ETiering({ integrationRoot });
    assert.match(
      result.errors.join('\n'),
      fixtureCase.expected,
      fixtureCase.name,
    );
  }

  const previousMode = process.env.PULSE_TIER_SMOKE_MODE;
  process.env.PULSE_TIER_SMOKE_MODE = 'omit-stable';
  const splitRoot = await createFixture();
  roots.push(splitRoot);
  const splitResult = await validateE2ETiering({ integrationRoot: splitRoot });
  assert.match(
    splitResult.errors.join('\n'),
    /Stable \+ probation tests is missing/,
  );
  if (previousMode === undefined) delete process.env.PULSE_TIER_SMOKE_MODE;
  else process.env.PULSE_TIER_SMOKE_MODE = previousMode;

  const floorRoot = await createFixture({ fileCount: 9 });
  roots.push(floorRoot);
  const floorResult = await validateE2ETiering({ integrationRoot: floorRoot });
  assert.match(
    floorResult.errors.join('\n'),
    /Stable tier has 9 spec files; the signal floor is 10/,
  );
});

test('fails closed for a missing CLI and unsuccessful child processes', async (t) => {
  const integrationRoot = await createFixture();
  t.after(async () => removeFixture(integrationRoot));

  await fs.rm(
    path.join(integrationRoot, 'node_modules', 'playwright', 'cli.js'),
  );
  assert.throws(
    () => listPlaywrightTests({ integrationRoot, tier: null }),
    /Pinned Playwright CLI is not installed/,
  );

  await fs.writeFile(
    path.join(integrationRoot, 'node_modules', 'playwright', 'cli.js'),
    cliSource,
  );
  const calls = [];
  assert.throws(
    () =>
      listPlaywrightTests({
        integrationRoot,
        tier: 'stable',
        spawn: (command, args, options) => {
          calls.push({ args, command, options });
          return { error: new Error('launch blocked') };
        },
      }),
    /Could not launch pinned Playwright CLI for stable tier: launch blocked/,
  );
  assert.equal(calls[0].command, process.execPath);
  assert.deepEqual(calls[0].args.slice(1), ['test', '--list']);
  assert.equal(calls[0].options.shell, false);

  const windowsListedTests = listPlaywrightTests({
    integrationRoot,
    tier: null,
    spawn: () => ({
      status: 0,
      stdout:
        '  [chromium] › journeys\\fixture.spec.ts:1:1 › fixture \\ title\n',
    }),
  });
  assert.deepEqual(
    [...windowsListedTests.values()],
    ['journeys/fixture.spec.ts'],
  );
  assert.deepEqual(
    [...windowsListedTests.keys()],
    ['[chromium] › journeys/fixture.spec.ts:1:1 › fixture \\ title'],
  );

  assert.throws(
    () =>
      listPlaywrightTests({
        integrationRoot,
        tier: null,
        spawn: () => ({ signal: 'SIGTERM' }),
      }),
    /Pinned Playwright CLI was signaled for all tier: SIGTERM/,
  );
  assert.throws(
    () =>
      listPlaywrightTests({
        integrationRoot,
        tier: null,
        spawn: () => ({ status: 23, stderr: 'fixture Playwright failure' }),
      }),
    /Pinned Playwright CLI --list failed for all tier:\nfixture Playwright failure/,
  );
});

test('fails closed when the pinned CLI exits nonzero', async (t) => {
  const integrationRoot = await createFixture();
  const previousMode = process.env.PULSE_TIER_SMOKE_MODE;
  process.env.PULSE_TIER_SMOKE_MODE = 'nonzero';
  t.after(async () => {
    if (previousMode === undefined) delete process.env.PULSE_TIER_SMOKE_MODE;
    else process.env.PULSE_TIER_SMOKE_MODE = previousMode;
    await removeFixture(integrationRoot);
  });

  assert.throws(
    () => listPlaywrightTests({ integrationRoot, tier: null }),
    /Pinned Playwright CLI --list failed for all tier:\nfixture Playwright failure/,
  );
});

test('fails closed when the pinned CLI is signaled', async (t) => {
  const integrationRoot = await createFixture();
  const previousMode = process.env.PULSE_TIER_SMOKE_MODE;
  process.env.PULSE_TIER_SMOKE_MODE = 'signal';
  t.after(async () => {
    if (previousMode === undefined) delete process.env.PULSE_TIER_SMOKE_MODE;
    else process.env.PULSE_TIER_SMOKE_MODE = previousMode;
    await removeFixture(integrationRoot);
  });

  const expected =
    process.platform === 'win32'
      ? /Pinned Playwright CLI --list failed for all tier:\nfixture Playwright termination/
      : /Pinned Playwright CLI was signaled for all tier: SIGTERM/;
  assert.throws(
    () => listPlaywrightTests({ integrationRoot, tier: null }),
    expected,
  );
});
