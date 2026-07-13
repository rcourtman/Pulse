import { describe, expect, it } from 'vitest';

import {
  formatConfidenceLabel,
  formatConfidencePercentage,
} from '@/utils/confidencePresentation';

describe('confidencePresentation — branch coverage (branchcov0713)', () => {
  describe('formatConfidencePercentage — Number.isFinite guard arm', () => {
    it('returns empty string for NaN (non-finite)', () => {
      expect(formatConfidencePercentage(NaN)).toBe('');
    });

    it('returns empty string for positive Infinity (non-finite)', () => {
      expect(formatConfidencePercentage(Infinity)).toBe('');
    });

    it('returns empty string for negative Infinity (non-finite)', () => {
      expect(formatConfidencePercentage(-Infinity)).toBe('');
    });
  });

  describe('formatConfidencePercentage — rounding happy path & boundaries', () => {
    it('rounds a value whose product crosses .5 upward to the next integer', () => {
      expect(formatConfidencePercentage(0.875)).toBe('88%');
    });

    it('rounds 0.999 up to 100% (boundary that saturates the percent)', () => {
      expect(formatConfidencePercentage(0.999)).toBe('100%');
    });

    it('returns 100% for exactly 1', () => {
      expect(formatConfidencePercentage(1)).toBe('100%');
    });

    it('returns 0% for exactly 0', () => {
      expect(formatConfidencePercentage(0)).toBe('0%');
    });

    it('rounds a small positive value down to 0%', () => {
      expect(formatConfidencePercentage(0.001)).toBe('0%');
    });

    it('formats a negative ratio with a leading minus sign', () => {
      expect(formatConfidencePercentage(-0.5)).toBe('-50%');
    });

    it('formats a large value greater than 1 as an unscaled integer percent', () => {
      expect(formatConfidencePercentage(12.3456)).toBe('1235%');
    });
  });

  describe('formatConfidenceLabel — number arm', () => {
    it('delegates a finite number to formatConfidencePercentage', () => {
      expect(formatConfidenceLabel(0.999)).toBe('100%');
    });

    it('returns empty string for NaN via the number arm (non-finite propagation)', () => {
      expect(formatConfidenceLabel(NaN)).toBe('');
    });

    it('returns empty string for Infinity via the number arm (non-finite propagation)', () => {
      expect(formatConfidenceLabel(Infinity)).toBe('');
    });
  });

  describe('formatConfidenceLabel — string arm, trim-then-empty guard', () => {
    it('returns empty string for an empty string (falsy after trim)', () => {
      expect(formatConfidenceLabel('')).toBe('');
    });

    it('returns empty string for a whitespace-only string (falsy after trim)', () => {
      expect(formatConfidenceLabel('   ')).toBe('');
    });

    it('returns empty string for a tab/newline-only string (falsy after trim)', () => {
      expect(formatConfidenceLabel('\t\n  ')).toBe('');
    });
  });

  describe('formatConfidenceLabel — string arm, humanizeToken variants', () => {
    it('title-cases a single lowercase token', () => {
      expect(formatConfidenceLabel('low')).toBe('Low');
    });

    it('trims surrounding whitespace then title-cases the token', () => {
      expect(formatConfidenceLabel(' medium ')).toBe('Medium');
    });

    it('replaces underscores with spaces and title-cases each token', () => {
      expect(formatConfidenceLabel('very_high')).toBe('Very High');
    });

    it('title-cases a multi-token underscore string preserving inner casing', () => {
      expect(formatConfidenceLabel('abc_def_ghi')).toBe('Abc Def Ghi');
    });

    it('title-cases each word of an all-caps multi-word token via the underscore path', () => {
      expect(formatConfidenceLabel('UPPER_CASE_TOKEN')).toBe('UPPER CASE TOKEN');
    });

    it('leaves a 4-char all-caps token untouched because preserveShortAllCaps is not passed', () => {
      expect(formatConfidenceLabel('HIGH')).toBe('HIGH');
    });

    it('title-cases a numeric string (digit is an inert word char)', () => {
      expect(formatConfidenceLabel('42')).toBe('42');
    });
  });

  describe('formatConfidenceLabel — nullish / non-string-non-number fallback arm', () => {
    it('returns empty string for undefined', () => {
      expect(formatConfidenceLabel(undefined)).toBe('');
    });

    it('returns empty string for null', () => {
      expect(formatConfidenceLabel(null)).toBe('');
    });

    it('returns empty string for a boolean (typeof !== string|number, cast to satisfy declared type)', () => {
      const malformed = true as unknown as Parameters<typeof formatConfidenceLabel>[0];
      expect(formatConfidenceLabel(malformed)).toBe('');
    });

    it('returns empty string for a plain object (typeof object, cast to satisfy declared type)', () => {
      const malformed = { confidence: 0.9 } as unknown as Parameters<
        typeof formatConfidenceLabel
      >[0];
      expect(formatConfidenceLabel(malformed)).toBe('');
    });

    it('returns empty string for an array (typeof object, cast to satisfy declared type)', () => {
      const malformed = [0.5] as unknown as Parameters<typeof formatConfidenceLabel>[0];
      expect(formatConfidenceLabel(malformed)).toBe('');
    });
  });
});
