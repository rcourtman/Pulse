import { describe, expect, it } from 'vitest';

import { normalizeGuestDrawerTags } from '../guestDrawerModel';

// `normalizeGuestDrawerTags` narrows `Guest['tags']` (`string[] | string | null`).
// These tests deliberately drive both arms of every conditional in the body:
//   1. the `Array.isArray(tags)` branch (map -> trim, filter -> length > 0)
//   2. the `typeof tags === 'string'` branch (split -> trim, filter -> length > 0)
//   3. the terminal `return []` fallback (neither array nor string)
// A few inputs are deliberately outside the declared union (undefined / number)
// to exercise the defensive fallback; they are cast through `unknown` to satisfy
// strict null checks under the real tsconfig.
type TagsArg = Parameters<typeof normalizeGuestDrawerTags>[0];

describe('normalizeGuestDrawerTags', () => {
  describe('array input (Array.isArray branch)', () => {
    it('trims leading/trailing whitespace from each tag', () => {
      expect(normalizeGuestDrawerTags(['  hello  ', '\tworld\n'])).toStrictEqual([
        'hello',
        'world',
      ]);
    });

    it('preserves internal spacing because only the ends are trimmed', () => {
      expect(normalizeGuestDrawerTags(['web server', ' db primary '])).toStrictEqual([
        'web server',
        'db primary',
      ]);
    });

    it('keeps tags that are non-empty after trimming (filter length > 0 true arm)', () => {
      expect(normalizeGuestDrawerTags(['a', 'b', 'c'])).toStrictEqual(['a', 'b', 'c']);
    });

    it('drops empty-string tags (filter length > 0 false arm)', () => {
      expect(normalizeGuestDrawerTags(['a', '', 'b'])).toStrictEqual(['a', 'b']);
    });

    it('drops whitespace-only tags because trim yields an empty string', () => {
      expect(normalizeGuestDrawerTags(['a', '   ', '\t', 'b'])).toStrictEqual(['a', 'b']);
    });

    it('preserves the original order of the surviving tags', () => {
      expect(normalizeGuestDrawerTags([' c ', '', ' a ', ' b '])).toStrictEqual([
        'c',
        'a',
        'b',
      ]);
    });

    it('returns an empty array for an empty input array', () => {
      expect(normalizeGuestDrawerTags([])).toStrictEqual([]);
    });

    it('returns an empty array when every element is empty or whitespace', () => {
      expect(normalizeGuestDrawerTags(['', '   ', '\t'])).toStrictEqual([]);
    });

    it('returns a new array instance rather than the same reference', () => {
      const input: TagsArg = ['a', 'b'];
      const result = normalizeGuestDrawerTags(input);
      expect(result).not.toBe(input);
      expect(result).toStrictEqual(['a', 'b']);
    });
  });

  describe('string input (typeof === "string" branch)', () => {
    it('returns a single tag untouched when there is no comma', () => {
      expect(normalizeGuestDrawerTags('solo')).toStrictEqual(['solo']);
    });

    it('splits on commas and trims each segment', () => {
      expect(normalizeGuestDrawerTags(' a , b ,c')).toStrictEqual(['a', 'b', 'c']);
    });

    it('preserves internal spacing within a segment while trimming its ends', () => {
      expect(normalizeGuestDrawerTags('web server, db primary ')).toStrictEqual([
        'web server',
        'db primary',
      ]);
    });

    it('drops empty segments produced by consecutive commas (filter false arm)', () => {
      expect(normalizeGuestDrawerTags('a,,b')).toStrictEqual(['a', 'b']);
    });

    it('drops whitespace-only segments because trim yields an empty string', () => {
      expect(normalizeGuestDrawerTags('a,   ,\t,b')).toStrictEqual(['a', 'b']);
    });

    it('returns an empty array for the empty string', () => {
      expect(normalizeGuestDrawerTags('')).toStrictEqual([]);
    });

    it('returns an empty array for a string of only commas', () => {
      expect(normalizeGuestDrawerTags(',,,')).toStrictEqual([]);
    });

    it('returns an empty array for a string of only whitespace and commas', () => {
      expect(normalizeGuestDrawerTags('   , \t ,  ')).toStrictEqual([]);
    });
  });

  describe('fallback (neither array nor string)', () => {
    it('returns an empty array for null', () => {
      expect(normalizeGuestDrawerTags(null)).toStrictEqual([]);
    });

    it('returns an empty array for undefined', () => {
      expect(normalizeGuestDrawerTags(undefined as unknown as TagsArg)).toStrictEqual([]);
    });

    it('returns an empty array for an unexpected numeric value', () => {
      expect(normalizeGuestDrawerTags(123 as unknown as TagsArg)).toStrictEqual([]);
    });
  });
});
