import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  readStringValue,
  readBooleanValue,
  readNumberValue,
  readStringArrayValue,
  parseAppriseTargets,
  formatAppriseTargets,
  guessNumericId,
  platformData,
} from '@/features/alerts/helpers';

describe('alerts helpers — coercion coverage', () => {
  describe('readStringValue', () => {
    it('returns the string when given a string', () => {
      expect(readStringValue('apprise')).toBe('apprise');
    });

    it('treats the empty string as a valid string', () => {
      expect(readStringValue('')).toBe('');
    });

    it('returns the default fallback for non-string inputs', () => {
      expect(readStringValue(undefined)).toBe('');
      expect(readStringValue(null)).toBe('');
      expect(readStringValue(42)).toBe('');
      expect(readStringValue(true)).toBe('');
      expect(readStringValue({ server: 'x' })).toBe('');
      expect(readStringValue(['a'])).toBe('');
    });

    it('honors a caller-supplied fallback', () => {
      expect(readStringValue(undefined, 'fallback')).toBe('fallback');
      expect(readStringValue(0, 'fallback')).toBe('fallback');
      expect(readStringValue('real', 'fallback')).toBe('real');
    });
  });

  describe('readBooleanValue', () => {
    it('returns the boolean when given a boolean', () => {
      expect(readBooleanValue(true)).toBe(true);
      expect(readBooleanValue(false)).toBe(false);
    });

    it('returns the default fallback for non-boolean inputs', () => {
      expect(readBooleanValue(undefined)).toBe(false);
      expect(readBooleanValue(null)).toBe(false);
      expect(readBooleanValue('true')).toBe(false);
      expect(readBooleanValue(0)).toBe(false);
      expect(readBooleanValue(1)).toBe(false);
    });

    it('honors a caller-supplied fallback and still preserves an explicit false', () => {
      expect(readBooleanValue(undefined, true)).toBe(true);
      expect(readBooleanValue('x', true)).toBe(true);
      // An explicit false must not be overwritten by the fallback.
      expect(readBooleanValue(false, true)).toBe(false);
    });
  });

  describe('readNumberValue', () => {
    it('returns the number when given a finite number', () => {
      expect(readNumberValue(587, 0)).toBe(587);
      expect(readNumberValue(0, 99)).toBe(0);
      expect(readNumberValue(-5, 99)).toBe(-5);
    });

    it('returns the fallback for non-finite numbers', () => {
      expect(readNumberValue(Number.NaN, 25)).toBe(25);
      expect(readNumberValue(Number.POSITIVE_INFINITY, 25)).toBe(25);
      expect(readNumberValue(Number.NEGATIVE_INFINITY, 25)).toBe(25);
    });

    it('returns the fallback for non-number inputs', () => {
      expect(readNumberValue(undefined, 25)).toBe(25);
      expect(readNumberValue(null, 25)).toBe(25);
      expect(readNumberValue('587', 25)).toBe(25);
      expect(readNumberValue(true, 25)).toBe(25);
      expect(readNumberValue({}, 25)).toBe(25);
    });
  });

  describe('readStringArrayValue', () => {
    it('returns the array when every entry is a string', () => {
      expect(readStringArrayValue(['a', 'b', 'c'])).toEqual(['a', 'b', 'c']);
    });

    it('filters out non-string entries while preserving order', () => {
      expect(readStringArrayValue(['a', 1, null, 'b', undefined, true, 'c'])).toEqual([
        'a',
        'b',
        'c',
      ]);
    });

    it('returns an empty array for non-array inputs', () => {
      expect(readStringArrayValue(undefined)).toEqual([]);
      expect(readStringArrayValue(null)).toEqual([]);
      expect(readStringArrayValue('not-an-array')).toEqual([]);
      expect(readStringArrayValue({ length: 1 })).toEqual([]);
    });

    it('returns an empty array for an empty array input', () => {
      expect(readStringArrayValue([])).toEqual([]);
    });
  });

  describe('parseAppriseTargets', () => {
    it('splits newline-separated targets', () => {
      expect(parseAppriseTargets('mailto:a@x\nmailto:b@x')).toEqual(['mailto:a@x', 'mailto:b@x']);
    });

    it('splits comma-separated targets', () => {
      expect(parseAppriseTargets('mailto:a@x,mailto:b@x')).toEqual(['mailto:a@x', 'mailto:b@x']);
    });

    it('splits a mix of newlines and commas', () => {
      expect(parseAppriseTargets('mailto:a@x\nmailto:b@x,mailto:c@x')).toEqual([
        'mailto:a@x',
        'mailto:b@x',
        'mailto:c@x',
      ]);
    });

    it('normalizes CRLF line endings', () => {
      expect(parseAppriseTargets('mailto:a@x\r\nmailto:b@x')).toEqual(['mailto:a@x', 'mailto:b@x']);
    });

    it('trims whitespace around each target', () => {
      expect(parseAppriseTargets('  mailto:a@x  , \tmailto:b@x \n')).toEqual([
        'mailto:a@x',
        'mailto:b@x',
      ]);
    });

    it('drops empty entries produced by trailing separators and blanks', () => {
      expect(parseAppriseTargets('mailto:a@x,\n, ,\nmailto:b@x\n')).toEqual([
        'mailto:a@x',
        'mailto:b@x',
      ]);
    });

    it('deduplicates targets keeping first occurrence order', () => {
      expect(parseAppriseTargets('mailto:a@x\nmailto:b@x\nmailto:a@x')).toEqual([
        'mailto:a@x',
        'mailto:b@x',
      ]);
    });

    it('returns an empty array for an empty string', () => {
      expect(parseAppriseTargets('')).toEqual([]);
    });
  });

  describe('formatAppriseTargets', () => {
    it('joins targets with newlines', () => {
      expect(formatAppriseTargets(['mailto:a@x', 'mailto:b@x'])).toBe('mailto:a@x\nmailto:b@x');
    });

    it('returns a single target without a trailing newline', () => {
      expect(formatAppriseTargets(['mailto:a@x'])).toBe('mailto:a@x');
    });

    it('returns an empty string for an empty array', () => {
      expect(formatAppriseTargets([])).toBe('');
    });

    it('returns an empty string for nullish inputs', () => {
      expect(formatAppriseTargets(undefined)).toBe('');
      expect(formatAppriseTargets(null)).toBe('');
    });

    it('preserves caller-provided order', () => {
      expect(formatAppriseTargets(['c', 'a', 'b'])).toBe('c\na\nb');
    });
  });

  describe('parseAppriseTargets / formatAppriseTargets round-trip', () => {
    it('parse(format(targets)) is identity for a clean list', () => {
      const targets = ['mailto:a@x', 'mailto:b@x', 'mailto:c@x'];
      expect(parseAppriseTargets(formatAppriseTargets(targets))).toEqual(targets);
    });

    it('format(parse(text)) normalizes messy input to newline form', () => {
      const messy = '  mailto:a@x , mailto:b@x\n\r\nmailto:a@x\n ';
      expect(formatAppriseTargets(parseAppriseTargets(messy))).toBe('mailto:a@x\nmailto:b@x');
    });

    it('an empty list round-trips through the empty string', () => {
      expect(formatAppriseTargets(parseAppriseTargets(''))).toBe('');
      expect(parseAppriseTargets(formatAppriseTargets([]))).toEqual([]);
    });
  });

  describe('guessNumericId', () => {
    it('extracts trailing digits', () => {
      expect(guessNumericId('node-100')).toBe(100);
      expect(guessNumericId('vm/42')).toBe(42);
      expect(guessNumericId('100')).toBe(100);
    });

    it('tolerates trailing whitespace after the digits', () => {
      expect(guessNumericId('guest 7 ')).toBe(7);
      expect(guessNumericId('guest 7\t')).toBe(7);
    });

    it('returns 0 when no digits are present', () => {
      expect(guessNumericId('no-id-here')).toBe(0);
      expect(guessNumericId('')).toBe(0);
    });

    it('returns 0 when digits appear only before a non-digit suffix', () => {
      expect(guessNumericId('100abc')).toBe(0);
      expect(guessNumericId('node-5-extra')).toBe(0);
    });

    it('parses leading zeros with radix 10', () => {
      expect(guessNumericId('ct-007')).toBe(7);
    });

    it('parses large ids', () => {
      expect(guessNumericId('storage-100200300')).toBe(100200300);
    });
  });

  describe('platformData', () => {
    const makeResource = (overrides: Partial<Resource>): Resource =>
      ({ id: 'r1', name: 'r1', type: 'vm', ...overrides }) as Resource;

    it('returns undefined when platformData is absent', () => {
      expect(platformData(makeResource({}))).toBeUndefined();
    });

    it('returns undefined when platformData is null', () => {
      expect(
        platformData(makeResource({ platformData: null as unknown as undefined })),
      ).toBeUndefined();
    });

    it('returns the unwrapped payload when present', () => {
      const payload = { proxmox: { vmid: 100, node: 'n1' } };
      const result = platformData(makeResource({ platformData: payload }));
      expect(result).toEqual(payload);
    });

    it('returns a (truthy) empty object when platformData is an empty object', () => {
      const result = platformData(makeResource({ platformData: {} }));
      expect(result).toEqual({});
    });
  });
});
