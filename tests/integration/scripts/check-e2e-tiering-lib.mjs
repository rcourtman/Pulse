import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

const MIN_STABLE_SPEC_FILES = 10;
const MAX_OUTPUT_BYTES = 64 * 1024 * 1024;

function listSpecFiles(testsRoot) {
  const specs = [];
  function walk(directory, relativeDirectory = '') {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory()) {
        walk(
          path.join(directory, entry.name),
          `${relativeDirectory}${entry.name}/`,
        );
      } else if (entry.name.endsWith('.spec.ts')) {
        specs.push(`${relativeDirectory}${entry.name}`);
      }
    }
  }
  walk(testsRoot);
  return specs.sort();
}

function duplicates(values) {
  const seen = new Set();
  return values.filter((value) => {
    if (seen.has(value)) return true;
    seen.add(value);
    return false;
  });
}

function difference(left, right) {
  return new Set([...left].filter((value) => !right.has(value)));
}

function sameSet(actual, expected, label, errors) {
  const missing = difference(expected, actual);
  const extra = difference(actual, expected);
  if (missing.size)
    errors.push(`${label} is missing: ${[...missing].join(', ')}`);
  if (extra.size)
    errors.push(`${label} has unexpected entries: ${[...extra].join(', ')}`);
}

function ledgerFiles(values, label, allFiles, errors) {
  const badFormat = values.filter(
    (value) => !/^\*\*\/.+\.spec\.ts$/.test(value),
  );
  if (badFormat.length)
    errors.push(`${label} has invalid entries: ${badFormat.join(', ')}`);

  const duplicateValues = duplicates(values);
  if (duplicateValues.length)
    errors.push(`${label} has duplicates: ${duplicateValues.join(', ')}`);

  const sortedValues = [...values].sort();
  if (values.some((value, index) => value !== sortedValues[index])) {
    errors.push(`${label} must be sorted`);
  }

  const files = new Set(values.map((value) => value.replace(/^\*\*\//, '')));
  const unknown = [...files].filter((value) => !allFiles.has(value));
  if (unknown.length)
    errors.push(`${label} has unknown specs: ${unknown.join(', ')}`);
  return files;
}

export function listPlaywrightTests({
  integrationRoot,
  tier,
  spawn = spawnSync,
}) {
  const playwrightCli = path.join(
    integrationRoot,
    'node_modules',
    'playwright',
    'cli.js',
  );
  if (!fs.existsSync(playwrightCli)) {
    throw new Error(
      'Pinned Playwright CLI is not installed; run npm ci before this check',
    );
  }
  const env = { ...process.env };
  if (tier) env.PULSE_E2E_TIER = tier;
  else delete env.PULSE_E2E_TIER;

  const result = spawn(process.execPath, [playwrightCli, 'test', '--list'], {
    cwd: integrationRoot,
    env,
    encoding: 'utf8',
    maxBuffer: MAX_OUTPUT_BYTES,
    shell: false,
  });
  const label = tier || 'all';
  if (result.error) {
    throw new Error(
      `Could not launch pinned Playwright CLI for ${label} tier: ${result.error.message}`,
    );
  }
  if (result.signal) {
    throw new Error(
      `Pinned Playwright CLI was signaled for ${label} tier: ${result.signal}`,
    );
  }
  if (result.status !== 0) {
    throw new Error(
      `Pinned Playwright CLI --list failed for ${label} tier:\n${result.stderr || result.stdout}`,
    );
  }

  const tests = new Map();
  for (const rawLine of result.stdout.split('\n')) {
    const line = rawLine.replace(/\x1b\[[0-9;]*m/g, '');
    const match = line.match(
      /^\s*\[([^\]]+)\] › ((.+?\.spec\.ts):\d+:\d+ › .+)$/,
    );
    if (!match) continue;
    const testPath = match[3].replaceAll('\\', '/');
    const key = `[${match[1]}] › ${testPath}${match[2].slice(match[3].length)}`;
    if (tests.has(key))
      throw new Error(`Playwright listed a duplicate test: ${key}`);
    tests.set(key, testPath);
  }
  return tests;
}

export async function validateE2ETiering({ integrationRoot }) {
  const ledger = await import(
    `${pathToFileURL(path.join(integrationRoot, 'e2e-tiering.mjs')).href}?${Date.now()}`
  );
  const errors = [];
  const allFiles = new Set(listSpecFiles(path.join(integrationRoot, 'tests')));
  const probationFiles = ledgerFiles(
    ledger.PROBATION_SPECS,
    'PROBATION_SPECS',
    allFiles,
    errors,
  );
  const quarantineFiles = ledgerFiles(
    ledger.QUARANTINED_SPECS,
    'QUARANTINED_SPECS',
    allFiles,
    errors,
  );
  const overlap = [...probationFiles].filter((value) =>
    quarantineFiles.has(value),
  );
  if (overlap.length)
    errors.push(`Probation/quarantine overlap: ${overlap.join(', ')}`);

  const allTests = listPlaywrightTests({ integrationRoot, tier: null });
  const stableTests = listPlaywrightTests({ integrationRoot, tier: 'stable' });
  const probationTests = listPlaywrightTests({
    integrationRoot,
    tier: 'probation',
  });
  const allKeys = new Set(allTests.keys());
  const stableKeys = new Set(stableTests.keys());
  const probationKeys = new Set(probationTests.keys());

  const instanceOverlap = [...stableKeys].filter((key) =>
    probationKeys.has(key),
  );
  if (instanceOverlap.length)
    errors.push(`Stable/probation test overlap: ${instanceOverlap.join(', ')}`);
  sameSet(
    new Set([...stableKeys, ...probationKeys]),
    allKeys,
    'Stable + probation tests',
    errors,
  );

  const listedAllFiles = new Set(allTests.values());
  const listedStableFiles = new Set(stableTests.values());
  const listedProbationFiles = new Set(probationTests.values());
  const expectedAllFiles = difference(allFiles, quarantineFiles);
  const expectedStableFiles = difference(expectedAllFiles, probationFiles);
  sameSet(listedAllFiles, expectedAllFiles, 'All-tier spec files', errors);
  sameSet(
    listedStableFiles,
    expectedStableFiles,
    'Stable-tier spec files',
    errors,
  );
  sameSet(
    listedProbationFiles,
    probationFiles,
    'Probation-tier spec files',
    errors,
  );

  if (listedStableFiles.size < MIN_STABLE_SPEC_FILES) {
    errors.push(
      `Stable tier has ${listedStableFiles.size} spec files; the signal floor is ${MIN_STABLE_SPEC_FILES}`,
    );
  }

  return {
    allFiles,
    allTests,
    errors,
    listedProbationFiles,
    listedStableFiles,
    probationTests,
    quarantineFiles,
    stableTests,
  };
}
