// @vitest-environment node

import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const manifest = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
const lock = JSON.parse(readFileSync(new URL('../package-lock.json', import.meta.url), 'utf8')) as {
  packages: Record<string, { version?: string }>;
};

// The same mock registration and redirect fixes apply to the portal runner,
// not just frontend-modern: GHSA-qxg6-89mg-rqv8 and GHSA-xp26-3454-mjgp.
const patchedVitest4 = (version: string): boolean => {
  const match = /^4\.(\d+)\.(\d+)$/.exec(version);
  return Boolean(match && (Number(match[1]) > 1 || (Number(match[1]) === 1 && Number(match[2]) >= 12)));
};

describe('portal Vitest dependency security', () => {
  it.each([
    ['4.1.11', false],
    ['4.0.20', false],
    ['3.2.12', false],
    ['4.1.12-beta.1', false],
    ['', false],
    ['4.1.12', true],
    ['4.1.13', true],
    ['4.2.0', true],
  ])('classifies %s without accepting an old or unreviewed runner', (version, expected) => {
    expect(patchedVitest4(version as string)).toBe(expected);
  });

  it('declares the maintained 4.x runner above the shared mocker floor', () => {
    expect(manifest.devDependencies.vitest).toMatch(/^\^4\.\d+\.\d+$/);
    expect(patchedVitest4(manifest.devDependencies.vitest.slice(1))).toBe(true);
  });

  it.each([
    'vitest', '@vitest/mocker', '@vitest/expect', '@vitest/pretty-format',
    '@vitest/runner', '@vitest/snapshot', '@vitest/spy', '@vitest/utils',
  ])('keeps every %s copy patched and aligned with the runner', name => {
    const runner = lock.packages['node_modules/vitest']?.version;
    const versions = Object.entries(lock.packages)
      .filter(([path]) => path === `node_modules/${name}` || path.endsWith(`/node_modules/${name}`))
      .map(([, entry]) => entry.version ?? '');
    expect(versions.length, `${name} must be locked`).toBeGreaterThan(0);
    for (const version of versions) {
      expect(patchedVitest4(version), `${name} ${version} is below the reviewed floor`).toBe(true);
      expect(version, `${name} must match the runner`).toBe(runner);
    }
  });
});
