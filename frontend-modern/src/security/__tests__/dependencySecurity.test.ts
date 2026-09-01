// @vitest-environment node

import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

interface PackageManifest {
  dependencies: Record<string, string>;
}

interface PackageLock {
  packages: Record<string, { version?: string }>;
}

const manifest = JSON.parse(
  readFileSync(new URL('../../../package.json', import.meta.url), 'utf8'),
) as PackageManifest;
const lock = JSON.parse(
  readFileSync(new URL('../../../package-lock.json', import.meta.url), 'utf8'),
) as PackageLock;

const parseVersion = (version: string): [number, number, number] => {
  const [major = 0, minor = 0, patch = 0] = version
    .split('-', 1)[0]
    .split('.')
    .map((part) => Number.parseInt(part, 10));
  return [major, minor, patch];
};

const atLeast = (version: string, floor: [number, number, number]): boolean => {
  const current = parseVersion(version);
  for (let index = 0; index < current.length; index += 1) {
    if (current[index] !== floor[index]) return current[index] > floor[index];
  }
  return true;
};

const lockedVersions = (packageName: string): string[] =>
  Object.entries(lock.packages)
    .filter(
      ([path]) =>
        path === `node_modules/${packageName}` || path.endsWith(`/node_modules/${packageName}`),
    )
    .map(([, entry]) => entry.version)
    .filter((version): version is string => Boolean(version));

const braceExpansionIsPatched = (version: string): boolean => {
  const [major] = parseVersion(version);
  if (major === 1) return atLeast(version, [1, 1, 18]);
  if (major === 2) return atLeast(version, [2, 1, 4]);
  if (major === 3) return atLeast(version, [3, 0, 6]);
  return major >= 5 && atLeast(version, [5, 0, 9]);
};

const nanoidIsPatched = (version: string): boolean => {
  const [major] = parseVersion(version);
  if (major === 3) return atLeast(version, [3, 3, 17]);
  return major >= 5 && atLeast(version, [5, 1, 6]);
};

describe('frontend dependency security floors', () => {
  it('keeps DOMPurify above the hook-detachment XSS floor', () => {
    expect(manifest.dependencies.dompurify).toBe('^3.4.13');
    const versions = lockedVersions('dompurify');
    expect(versions).not.toHaveLength(0);
    for (const version of versions) {
      expect(atLeast(version, [3, 4, 13]), `dompurify ${version} is vulnerable`).toBe(true);
    }
  });

  it('keeps every brace-expansion line above both denial-of-service advisory floors', () => {
    const versions = lockedVersions('brace-expansion');
    expect(versions).not.toHaveLength(0);
    for (const version of versions) {
      expect(braceExpansionIsPatched(version), `brace-expansion ${version} is vulnerable`).toBe(
        true,
      );
    }
  });

  it('keeps nanoid custom generators above the zero-size loop floor', () => {
    const versions = lockedVersions('nanoid');
    expect(versions).not.toHaveLength(0);
    for (const version of versions) {
      expect(nanoidIsPatched(version), `nanoid ${version} is vulnerable`).toBe(true);
    }
  });

  it('keeps browserslist above the unbounded-cache and custom-stats floors', () => {
    const versions = lockedVersions('browserslist');
    expect(versions).not.toHaveLength(0);
    for (const version of versions) {
      expect(atLeast(version, [4, 28, 7]), `browserslist ${version} is vulnerable`).toBe(true);
    }
  });
});
