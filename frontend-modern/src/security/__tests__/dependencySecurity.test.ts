// @vitest-environment node

import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

interface PackageManifest {
  dependencies: Record<string, string>;
  devDependencies: Record<string, string>;
  overrides: Record<string, string>;
}

interface PackageLock {
  packages: Record<
    string,
    {
      version?: string;
      integrity?: string;
      dependencies?: Record<string, string>;
      hasInstallScript?: boolean;
    }
  >;
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

// Accept maintained 3.x patch/minor upgrades without pinning the manifest to
// one historical fix. Reject broader ranges, prereleases and unreviewed majors.
const dompurifyRangeIsPatched = (range: string): boolean =>
  /^\^3\.\d+\.\d+$/.test(range) && atLeast(range.slice(1), [3, 4, 13]);

describe('frontend dependency security floors', () => {
  it('locks the documentation heading helper without install scripts or transitive dependencies', () => {
    const slugger = lock.packages['node_modules/github-slugger'];
    expect(slugger).toBeDefined();
    expect(manifest.dependencies['github-slugger']).toBe(slugger.version);
    expect(slugger.integrity).toMatch(/^sha512-/);
    expect(slugger.dependencies ?? {}).toEqual({});
    expect(slugger.hasInstallScript).not.toBe(true);
  });

  it('keeps Vitest and its mocker above the redirect-mock file-read floor', () => {
    // GHSA-82fw-gwwq-j7x9: the maintained 4.x fix starts at 4.1.11.
    expect(manifest.devDependencies.vitest).toBe('^4.1.11');
    expect(manifest.devDependencies['@vitest/coverage-v8']).toBe('^4.1.11');
    const runner = lockedVersions('vitest');
    expect(runner).toHaveLength(1);
    for (const name of ['vitest', '@vitest/mocker', '@vitest/coverage-v8']) {
      const versions = lockedVersions(name);
      expect(versions).not.toHaveLength(0);
      for (const version of versions) {
        expect(version).not.toContain('-');
        expect(atLeast(version, [4, 1, 11]), `${name} ${version} is vulnerable`).toBe(true);
        expect(version, `${name} must match the runner`).toBe(runner[0]);
      }
    }
  });

  it('keeps every js-yaml copy above the empty-merge CPU exhaustion floor', () => {
    // GHSA-2883-xcg3-v3hh: our override keeps every transitive copy on patched 4.x.
    expect(manifest.overrides['js-yaml']).toBe('^4.3.2');
    const versions = lockedVersions('js-yaml');
    expect(versions).not.toHaveLength(0);
    for (const version of versions) {
      expect(version).not.toContain('-');
      expect(atLeast(version, [4, 3, 2]), `js-yaml ${version} is vulnerable`).toBe(true);
    }
  });

  it.each(['^3.4.13', '^3.4.14', '^3.5.0'])('accepts patched DOMPurify range %s', (range) => {
    expect(dompurifyRangeIsPatched(range)).toBe(true);
  });

  it.each(['^3.4.12', '^3.3.99', '^4.0.0', '^3.4.13-beta.1', '*', '>=3.4.13', '^3.4.13 || ^2.0.0'])(
    'rejects unsafe or unreviewed DOMPurify range %s',
    (range) => {
      expect(dompurifyRangeIsPatched(range)).toBe(false);
    },
  );

  it('keeps DOMPurify above the hook-detachment XSS floor', () => {
    expect(dompurifyRangeIsPatched(manifest.dependencies.dompurify)).toBe(true);
    const versions = lockedVersions('dompurify');
    expect(versions).not.toHaveLength(0);
    for (const version of versions) {
      expect(version).not.toContain('-');
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
