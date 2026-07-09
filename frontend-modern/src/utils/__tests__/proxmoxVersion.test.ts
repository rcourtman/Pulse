import { describe, expect, it } from 'vitest';

import { formatProxmoxVersion } from '@/utils/proxmoxVersion';

describe('proxmoxVersion', () => {
  describe('formatProxmoxVersion', () => {
    it.each([
      ['null', null, ''],
      ['undefined', undefined, ''],
      ['empty string', '', ''],
      ['whitespace only', '   ', ''],
      ['unknown (lowercase)', 'unknown', ''],
      ['UNKNOWN (uppercase)', 'UNKNOWN', ''],
      ['padded unknown', '  unknown  ', ''],
      ['Unknown (mixed case)', 'Unknown', ''],
    ])('returns empty for %s', (_label, input, expected) => {
      expect(formatProxmoxVersion(input)).toBe(expected);
    });

    describe('pve-manager extraction', () => {
      it.each([
        ['plain', 'pve-manager/8.0.4', '8.0.4'],
        ['uppercase prefix', 'PVE-Manager/8.0.4', '8.0.4'],
        ['stops at slash', 'pve-manager/8.0/9', '8.0'],
        ['stops at space', 'pve-manager/8.0.4 running', '8.0.4'],
        ['with suffix', 'pve-manager/8.0.4-1', '8.0.4-1'],
      ])('extracts version for %s', (_label, input, expected) => {
        expect(formatProxmoxVersion(input)).toBe(expected);
      });

      it('falls through when pve-manager has no capturable value', () => {
        expect(formatProxmoxVersion('pve-manager/')).toBe('pve-manager/');
      });
    });

    describe('numeric dotted-version fallback', () => {
      it.each([
        ['plain dotted', '8.0.4', '8.0.4'],
        ['embedded in text', 'version 8.0.4 here', '8.0.4'],
        ['two segments', '8.0', '8.0'],
        ['with hyphen suffix', '8.0.4-1', '8.0.4-1'],
        ['with plus suffix', '8.0.4+build', '8.0.4+build'],
      ])('extracts %s', (_label, input, expected) => {
        expect(formatProxmoxVersion(input)).toBe(expected);
      });
    });

    describe('last-resort passthrough', () => {
      it.each([
        ['bare single number has no dotted version', '8', '8'],
        ['garbage token', 'garbage', 'garbage'],
        ['preserves original case', 'Garbage-Text', 'Garbage-Text'],
        ['number with trailing text', '8x', '8x'],
      ])('returns input trimmed for %s', (_label, input, expected) => {
        expect(formatProxmoxVersion(input)).toBe(expected);
      });

      it('trims surrounding whitespace on passthrough', () => {
        expect(formatProxmoxVersion('  custom-build  ')).toBe('custom-build');
      });
    });

    describe('precedence', () => {
      it('prefers the pve-manager capture over the numeric fallback', () => {
        expect(formatProxmoxVersion('pve-manager/7.4.3 and 99.99.99')).toBe('7.4.3');
      });
    });
  });
});
