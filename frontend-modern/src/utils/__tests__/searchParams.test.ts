import { describe, expect, it } from 'vitest';
import { areSearchParamsEquivalent } from '@/utils/searchParams';

describe('searchParams', () => {
  describe('areSearchParamsEquivalent', () => {
    it('returns true for identical params', () => {
      const a = new URLSearchParams('foo=bar&baz=qux');
      const b = new URLSearchParams('foo=bar&baz=qux');
      expect(areSearchParamsEquivalent(a, b)).toBe(true);
    });

    it('returns true for params with different order', () => {
      const a = new URLSearchParams('foo=bar&baz=qux');
      const b = new URLSearchParams('baz=qux&foo=bar');
      expect(areSearchParamsEquivalent(a, b)).toBe(true);
    });

    it('returns true for empty params', () => {
      const a = new URLSearchParams('');
      const b = new URLSearchParams('');
      expect(areSearchParamsEquivalent(a, b)).toBe(true);
    });

    it('returns false for different param counts', () => {
      const a = new URLSearchParams('foo=bar');
      const b = new URLSearchParams('foo=bar&baz=qux');
      expect(areSearchParamsEquivalent(a, b)).toBe(false);
    });

    it('returns false for different keys', () => {
      const a = new URLSearchParams('foo=bar');
      const b = new URLSearchParams('baz=qux');
      expect(areSearchParamsEquivalent(a, b)).toBe(false);
    });

    it('returns false for same keys but different values', () => {
      const a = new URLSearchParams('foo=bar');
      const b = new URLSearchParams('foo=baz');
      expect(areSearchParamsEquivalent(a, b)).toBe(false);
    });

    it('handles duplicate keys', () => {
      const a = new URLSearchParams('foo=bar&foo=baz');
      const b = new URLSearchParams('foo=baz&foo=bar');
      expect(areSearchParamsEquivalent(a, b)).toBe(true);
    });

    it('handles URL-encoded values correctly', () => {
      // URLSearchParams automatically decodes values, so these are equivalent
      const a = new URLSearchParams('foo=hello%20world');
      const b = new URLSearchParams('foo=hello world');
      // They are equivalent because URLSearchParams decodes them to the same value
      expect(areSearchParamsEquivalent(a, b)).toBe(true);
    });

    it('handles single param vs empty', () => {
      const a = new URLSearchParams('foo=bar');
      const b = new URLSearchParams('');
      expect(areSearchParamsEquivalent(a, b)).toBe(false);
    });

    it('handles multiple params with same value', () => {
      const a = new URLSearchParams('a=1&b=2');
      const b = new URLSearchParams('a=1&b=2');
      expect(areSearchParamsEquivalent(a, b)).toBe(true);
    });

    it('handles complex ordering with multiple params', () => {
      const a = new URLSearchParams('a=1&b=2&c=3');
      const b = new URLSearchParams('c=3&a=1&b=2');
      expect(areSearchParamsEquivalent(a, b)).toBe(true);
    });
  });
});
