#!/usr/bin/env node
// Keep the reviewed Vitest security floor and exact companion alignment without
// freezing future maintained 4.x patches to one historical manifest spelling.
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const frontend = new URL('../frontend-modern/', import.meta.url);
const floor = [4, 1, 11]; // GHSA-82fw-gwwq-j7x9: redirect-mock file reads.
const stableVersion = /^(4)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
const companions = ['vitest', '@vitest/mocker', '@vitest/coverage-v8'];

function versionParts(value) {
  const match = typeof value === 'string' && stableVersion.exec(value);
  const parts = match && match[0] === value ? match.slice(1).map(Number) : null;
  return parts?.every(Number.isSafeInteger) ? parts : null;
}

function atLeast(version, minimum) {
  for (let index = 0; index < minimum.length; index++) {
    if (version[index] !== minimum[index]) return version[index] > minimum[index];
  }
  return true;
}

export function vitestGraphErrors(manifest, lock) {
  const packages = lock?.packages;
  if (lock?.lockfileVersion !== 3 || !packages || typeof packages !== 'object' || Array.isArray(packages)) {
    return ['Expected an npm v3 lock with a complete packages map.'];
  }
  const errors = [];
  const runner = packages['node_modules/vitest']?.version;
  const runnerParts = versionParts(runner);
  for (const name of ['vitest', '@vitest/coverage-v8']) {
    const range = manifest?.devDependencies?.[name];
    const declared = typeof range === 'string' && range.startsWith('^') && versionParts(range.slice(1));
    if (!declared || !atLeast(declared, floor)) {
      errors.push(`${name} must declare a stable reviewed ^4.x floor at or above ^4.1.11.`);
    } else if (runnerParts && !atLeast(runnerParts, declared)) {
      errors.push(`The locked runner is below the declared ${name} floor.`);
    }
    if (packages['']?.devDependencies?.[name] !== range) {
      errors.push(`The root lock declaration for ${name} must match package.json.`);
    }
  }
  for (const name of companions) {
    const entries = Object.entries(packages).filter(([path]) =>
      path === `node_modules/${name}` || path.endsWith(`/node_modules/${name}`),
    );
    if (!entries.length || (name === 'vitest' &&
        (entries.length !== 1 || entries[0][0] !== 'node_modules/vitest'))) {
      errors.push(`${name} must be locked; Vitest must have one direct runner.`);
    }
    for (const [, entry] of entries) {
      const parts = versionParts(entry?.version);
      if (!parts || !atLeast(parts, floor)) {
        errors.push(`${name} must lock a stable reviewed 4.x version at or above 4.1.11.`);
      } else if (entry.version !== runner) {
        errors.push(`${name} must exactly match the locked runner.`);
      }
    }
  }
  return errors;
}

export function main() {
  let manifest, lock;
  try {
    manifest = JSON.parse(readFileSync(new URL('package.json', frontend), 'utf8'));
    lock = JSON.parse(readFileSync(new URL('package-lock.json', frontend), 'utf8'));
  } catch {
    console.error('Frontend Vitest graph: manifest/lock read or JSON parse failed.');
    return 2;
  }
  const errors = vitestGraphErrors(manifest, lock);
  for (const error of errors) console.error(`Frontend Vitest graph: ${error}`);
  if (errors.length) return 1;
  console.log('Frontend Vitest graph: reviewed 4.x floors and exact runner/mocker/coverage alignment retained.');
  return 0;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main();
}
