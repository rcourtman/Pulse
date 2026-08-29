#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

import {
  PROBATION_SPECS,
  QUARANTINED_SPECS,
} from '../e2e-tiering.mjs';
import { parsePlaywrightTestIdentity } from './e2e-tier-identity.mjs';

const integrationRoot = path.resolve(fileURLToPath(new URL('..', import.meta.url)));
const testsRoot = path.join(integrationRoot, 'tests');
const playwrightBin = path.join(integrationRoot, 'node_modules', '.bin', 'playwright');
const MIN_STABLE_SPEC_FILES = 10;

function listSpecFiles() {
  const specs = [];
  function walk(directory, relativeDirectory = '') {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory()) {
        walk(path.join(directory, entry.name), `${relativeDirectory}${entry.name}/`);
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

function ledgerFiles(values, label, allFiles, errors) {
  const badFormat = values.filter((value) => !/^\*\*\/.+\.spec\.ts$/.test(value));
  if (badFormat.length) errors.push(`${label} has invalid entries: ${badFormat.join(', ')}`);

  const duplicateValues = duplicates(values);
  if (duplicateValues.length) errors.push(`${label} has duplicates: ${duplicateValues.join(', ')}`);

  const sortedValues = [...values].sort();
  if (values.some((value, index) => value !== sortedValues[index])) {
    errors.push(`${label} must be sorted`);
  }

  const files = new Set(values.map((value) => value.replace(/^\*\*\//, '')));
  const unknown = [...files].filter((value) => !allFiles.has(value));
  if (unknown.length) errors.push(`${label} has unknown specs: ${unknown.join(', ')}`);
  return files;
}

function listPlaywrightTests(tier) {
  if (!fs.existsSync(playwrightBin)) {
    throw new Error('Playwright is not installed; run npm ci before this check');
  }
  const env = { ...process.env };
  if (tier) env.PULSE_E2E_TIER = tier;
  else delete env.PULSE_E2E_TIER;

  const result = spawnSync(playwrightBin, ['test', '--list'], {
    cwd: integrationRoot,
    env,
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
  });
  if (result.status !== 0) {
    throw new Error(
      `Playwright --list failed for ${tier || 'all'} tier:\n${result.stderr || result.stdout}`,
    );
  }

  const tests = new Map();
  for (const rawLine of result.stdout.split('\n')) {
    const identity = parsePlaywrightTestIdentity(rawLine);
    if (!identity) continue;
    if (tests.has(identity.key)) {
      throw new Error(`Playwright listed a duplicate test: ${identity.key}`);
    }
    tests.set(identity.key, identity.specFile);
  }
  return tests;
}

function difference(left, right) {
  return new Set([...left].filter((value) => !right.has(value)));
}

function sameSet(actual, expected, label, errors) {
  const missing = difference(expected, actual);
  const extra = difference(actual, expected);
  if (missing.size) errors.push(`${label} is missing: ${[...missing].join(', ')}`);
  if (extra.size) errors.push(`${label} has unexpected entries: ${[...extra].join(', ')}`);
}

const errors = [];
const allFiles = new Set(listSpecFiles());
const probationFiles = ledgerFiles(PROBATION_SPECS, 'PROBATION_SPECS', allFiles, errors);
const quarantineFiles = ledgerFiles(QUARANTINED_SPECS, 'QUARANTINED_SPECS', allFiles, errors);
const overlap = [...probationFiles].filter((value) => quarantineFiles.has(value));
if (overlap.length) errors.push(`Probation/quarantine overlap: ${overlap.join(', ')}`);

const allTests = listPlaywrightTests(null);
const stableTests = listPlaywrightTests('stable');
const probationTests = listPlaywrightTests('probation');
const allKeys = new Set(allTests.keys());
const stableKeys = new Set(stableTests.keys());
const probationKeys = new Set(probationTests.keys());

const instanceOverlap = [...stableKeys].filter((key) => probationKeys.has(key));
if (instanceOverlap.length) errors.push(`Stable/probation test overlap: ${instanceOverlap.join(', ')}`);
sameSet(new Set([...stableKeys, ...probationKeys]), allKeys, 'Stable + probation tests', errors);

const listedAllFiles = new Set(allTests.values());
const listedStableFiles = new Set(stableTests.values());
const listedProbationFiles = new Set(probationTests.values());
const expectedAllFiles = difference(allFiles, quarantineFiles);
const expectedStableFiles = difference(expectedAllFiles, probationFiles);
sameSet(listedAllFiles, expectedAllFiles, 'All-tier spec files', errors);
sameSet(listedStableFiles, expectedStableFiles, 'Stable-tier spec files', errors);
sameSet(listedProbationFiles, probationFiles, 'Probation-tier spec files', errors);

if (listedStableFiles.size < MIN_STABLE_SPEC_FILES) {
  errors.push(
    `Stable tier has ${listedStableFiles.size} spec files; the signal floor is ${MIN_STABLE_SPEC_FILES}`,
  );
}

if (errors.length) {
  console.error('E2E tier selection validation failed:');
  for (const error of errors) console.error(`- ${error}`);
  process.exit(1);
}

console.log('E2E tier selection validation passed');
console.log(`  spec_files: ${allFiles.size}`);
console.log(`  stable_spec_files: ${listedStableFiles.size}`);
console.log(`  probation_spec_files: ${listedProbationFiles.size}`);
console.log(`  quarantined_spec_files: ${quarantineFiles.size}`);
console.log(`  test_project_instances: ${allTests.size}`);
console.log(`  stable_test_project_instances: ${stableTests.size}`);
console.log(`  probation_test_project_instances: ${probationTests.size}`);
