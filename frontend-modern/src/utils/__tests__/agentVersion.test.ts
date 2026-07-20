import { describe, expect, it } from 'vitest';

import {
  compareAgentVersions,
  formatAgentVersionDisplay,
  parseAgentVersion,
} from '@/utils/agentVersion';

describe('agentVersion', () => {
  describe('parseAgentVersion', () => {
    it.each([
      ['null', null, null],
      ['undefined', undefined, null],
      ['empty string', '', null],
      ['whitespace only', '   ', null],
    ])('returns null for %s', (_label, input, expected) => {
      expect(parseAgentVersion(input)).toEqual(expected);
    });

    it.each([
      ['plain x.y.z', '1.2.3', { major: 1, minor: 2, patch: 3, prerelease: [] }],
      ['lowercase v prefix', 'v1.2.3', { major: 1, minor: 2, patch: 3, prerelease: [] }],
      ['uppercase V prefix', 'V1.2.3', { major: 1, minor: 2, patch: 3, prerelease: [] }],
      ['whitespace padded', '  1.2.3  ', { major: 1, minor: 2, patch: 3, prerelease: [] }],
      ['with prerelease', '1.2.3-rc.5', { major: 1, minor: 2, patch: 3, prerelease: ['rc', '5'] }],
      [
        'with prerelease + build',
        '1.2.3-rc.5+git.1',
        { major: 1, minor: 2, patch: 3, prerelease: ['rc', '5'] },
      ],
      [
        'build metadata only',
        '1.2.3+git.151.gabcdef',
        { major: 1, minor: 2, patch: 3, prerelease: [] },
      ],
      [
        'v + prerelease + build',
        'v1.2.3-rc.5+build.2',
        { major: 1, minor: 2, patch: 3, prerelease: ['rc', '5'] },
      ],
      ['leading zeros', '01.02.03', { major: 1, minor: 2, patch: 3, prerelease: [] }],
      [
        'extra dotted segments (first 3 used)',
        '1.2.3.4.5',
        { major: 1, minor: 2, patch: 3, prerelease: [] },
      ],
      [
        'trailing dash with empty prerelease',
        '1.2.3-',
        { major: 1, minor: 2, patch: 3, prerelease: [] },
      ],
    ])('parses %s', (_label, input, expected) => {
      expect(parseAgentVersion(input)).toEqual(expected);
    });

    it.each([
      ['two parts', '1.2'],
      ['one part', '1'],
      ['non-numeric major', 'a.b.c'],
      ['non-numeric patch', '1.2.x'],
      ['leading dash (negative)', '-1.2.3'],
      ['only dots', '...'],
    ])('returns null for malformed %s', (_label, input) => {
      expect(parseAgentVersion(input)).toBeNull();
    });

    it('is lenient about a numeric-looking prefix in a part', () => {
      // parseInt('1e2', 10) === 1; this documents current lenient behavior.
      expect(parseAgentVersion('1e2.0.0')).toEqual({
        major: 1,
        minor: 0,
        patch: 0,
        prerelease: [],
      });
    });
  });

  describe('compareAgentVersions', () => {
    it('returns null when either side is unparseable', () => {
      expect(compareAgentVersions(null, '1.0.0')).toBeNull();
      expect(compareAgentVersions('1.0.0', null)).toBeNull();
      expect(compareAgentVersions('garbage', '1.0.0')).toBeNull();
      expect(compareAgentVersions('1.0.0', '1.0')).toBeNull();
      expect(compareAgentVersions(undefined, undefined)).toBeNull();
    });

    it.each([
      ['equal stable', '1.0.0', '1.0.0', 0],
      ['major lower', '1.0.0', '2.0.0', -1],
      ['major higher', '2.0.0', '1.0.0', 1],
      ['minor lower', '1.1.0', '1.2.0', -1],
      ['minor higher', '1.2.0', '1.1.0', 1],
      ['patch lower', '1.0.1', '1.0.2', -1],
      ['patch higher', '1.0.2', '1.0.1', 1],
      ['v prefix ignored', 'v1.2.3', '1.2.3', 0],
    ])('compares core precedence: %s', (_label, a, b, expected) => {
      expect(compareAgentVersions(a, b)).toBe(expected);
    });

    it.each([
      ['stable > prerelease', '1.0.0', '1.0.0-rc.1', 1],
      ['prerelease < stable', '1.0.0-rc.1', '1.0.0', -1],
      ['numeric prerelease lower', '1.0.0-rc.1', '1.0.0-rc.2', -1],
      ['numeric prerelease higher', '1.0.0-rc.2', '1.0.0-rc.1', 1],
      ['equal prerelease', '1.0.0-rc.1', '1.0.0-rc.1', 0],
      ['shorter prerelease set lower', '1.0.0-alpha', '1.0.0-alpha.1', -1],
      ['longer prerelease set higher', '1.0.0-alpha.1', '1.0.0-alpha', 1],
      ['lexical alpha lower', '1.0.0-alpha', '1.0.0-beta', -1],
      ['lexical alpha higher', '1.0.0-beta', '1.0.0-alpha', 1],
      ['numeric identifier below alpha', '1.0.0-1', '1.0.0-alpha', -1],
      ['alpha above numeric identifier', '1.0.0-alpha', '1.0.0-1', 1],
      ['build metadata ignored', '1.0.0+a', '1.0.0+b', 0],
    ])('applies semver prerelease rules: %s', (_label, a, b, expected) => {
      expect(compareAgentVersions(a, b)).toBe(expected);
    });

    it('compares real-world agent version ranges in ascending order', () => {
      const sorted = ['6.0.0-rc.5', '6.0.0', '6.1.0', '6.1.1'].sort(
        (x, y) => compareAgentVersions(x, y) ?? 0,
      );
      expect(sorted).toEqual(['6.0.0-rc.5', '6.0.0', '6.1.0', '6.1.1']);
    });
  });

  describe('formatAgentVersionDisplay', () => {
    it.each([
      ['null', null, ''],
      ['undefined', undefined, ''],
      ['empty', '', ''],
      ['malformed (too few parts)', '1.2', ''],
      ['plain', '1.2.3', 'v1.2.3'],
      ['lowercase v prefix', 'v1.2.3', 'v1.2.3'],
      ['uppercase V prefix', 'V1.2.3', 'v1.2.3'],
      ['prerelease', '1.2.3-rc.5', 'v1.2.3-rc.5'],
      ['strips build metadata', '1.2.3+git.151', 'v1.2.3'],
      ['prerelease + build', '1.2.3-rc.5+build', 'v1.2.3-rc.5'],
      ['trims whitespace', '  1.2.3  ', 'v1.2.3'],
      ['normalizes extra segments', '1.2.3.4.5', 'v1.2.3'],
      ['normalizes leading zeros', '01.02.03', 'v1.2.3'],
    ])('formats %s', (_label, input, expected) => {
      expect(formatAgentVersionDisplay(input)).toBe(expected);
    });
  });
});
