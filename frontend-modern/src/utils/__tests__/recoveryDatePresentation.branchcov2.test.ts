import { describe, expect, it } from 'vitest';
import {
  normalizeRecoveryDateSearchText,
  parseRecoveryDateKey,
  resolveRecoveryDateSearchKey,
} from '@/utils/recoveryDatePresentation';

describe('recoveryDatePresentation branch coverage (part 2)', () => {
  describe('normalizeRecoveryDateSearchText', () => {
    it('returns an empty string for a falsy value (value || "" branch)', () => {
      expect(normalizeRecoveryDateSearchText('')).toBe('');
    });

    it('strips 1st/2nd/3rd/4th and 21st-style ordinals before punctuation collapse', () => {
      expect(normalizeRecoveryDateSearchText('The 1st 2nd 3rd 4th and 21st')).toBe(
        'the 1 2 3 4 and 21',
      );
    });

    it('collapses mixed punctuation, tabs, and repeated whitespace into single spaces', () => {
      expect(normalizeRecoveryDateSearchText('  Friday,   Feb\t14th  ')).toBe('friday feb 14');
    });

    it('lowercases and trims alphabetic-only input without ordinals', () => {
      expect(normalizeRecoveryDateSearchText('  FEBRUARY  ')).toBe('february');
    });
  });

  describe('parseRecoveryDateKey', () => {
    it('falls back to new Date(key) when the year component is missing/NaN', () => {
      const date = parseRecoveryDateKey('');
      expect(Number.isNaN(date.getTime())).toBe(true);
    });

    it('falls back to new Date(key) when the month component is zero (!month branch)', () => {
      const date = parseRecoveryDateKey('2026-0-05');
      expect(Number.isNaN(date.getTime())).toBe(true);
    });

    it('falls back to new Date(key) when the day component is zero (!day branch)', () => {
      const date = parseRecoveryDateKey('2026-03-0');
      expect(Number.isNaN(date.getTime())).toBe(true);
    });

    it('parses a well-formed key into local Y/M/D components', () => {
      const date = parseRecoveryDateKey('2026-07-12');
      expect(date.getFullYear()).toBe(2026);
      expect(date.getMonth()).toBe(6);
      expect(date.getDate()).toBe(12);
    });
  });

  describe('resolveRecoveryDateSearchKey', () => {
    describe('length guard', () => {
      it('returns null when normalized text is shorter than 5 characters', () => {
        expect(resolveRecoveryDateSearchKey('Feb', [])).toBeNull();
      });

      it('proceeds when normalized text is exactly 5 characters (boundary)', () => {
        expect(resolveRecoveryDateSearchKey('vm 12', [])).toBeNull();
      });
    });

    describe('ISO-ish branch (yyyy m d)', () => {
      it('resolves a normalized ISO-ish date to a key', () => {
        expect(resolveRecoveryDateSearchKey('2026-02-14', [])).toBe('2026-02-14');
      });

      it('returns null when the ISO-ish month overflows (rollover guard in getRecoveryDateSearchKey)', () => {
        expect(resolveRecoveryDateSearchKey('2026 13 01', [])).toBeNull();
      });

      it('returns null when the ISO-ish day is impossible for the month (Feb 30 rollover)', () => {
        expect(resolveRecoveryDateSearchKey('2026 2 30', [])).toBeNull();
      });
    });

    describe('weekday prefix stripping', () => {
      it('strips a long weekday name before parsing the remainder', () => {
        expect(resolveRecoveryDateSearchKey('Monday Feb 14 2026', [])).toBe('2026-02-14');
      });

      it('strips a short weekday alias before parsing the remainder', () => {
        expect(resolveRecoveryDateSearchKey('Tue Feb 14 2026', [])).toBe('2026-02-14');
      });
    });

    describe('token count guard', () => {
      it('returns null when only a single non-weekday token remains (length < 2)', () => {
        expect(resolveRecoveryDateSearchKey('hello', [])).toBeNull();
      });

      it('returns null when a weekday is the only token (length 0 after shift)', () => {
        expect(resolveRecoveryDateSearchKey('monday', [])).toBeNull();
      });

      it('returns null when more than 3 tokens remain (length > 3)', () => {
        expect(resolveRecoveryDateSearchKey('feb 14 2026 foo', [])).toBeNull();
      });
    });

    describe('month/day validation', () => {
      it('returns null when the month token is not a recognized month alias (monthIndex null)', () => {
        expect(resolveRecoveryDateSearchKey('vm 14', [])).toBeNull();
      });

      it('returns null when the day token is non-numeric (Number.isInteger(day) false)', () => {
        expect(resolveRecoveryDateSearchKey('Feb ab', [])).toBeNull();
      });
    });

    describe('explicit-year (tokens[2]) branch', () => {
      it('returns null when tokens[2] is not a 4-digit year', () => {
        expect(resolveRecoveryDateSearchKey('Feb 14 26', [])).toBeNull();
      });

      it('returns null when the explicit year + month + day roll over (getRecoveryDateSearchKey guard)', () => {
        expect(resolveRecoveryDateSearchKey('Feb 30 2026', [])).toBeNull();
      });
    });

    describe('candidate resolution branch', () => {
      it('returns the single candidate key matching month/day', () => {
        expect(resolveRecoveryDateSearchKey('Feb 14', ['2026-02-13', '2026-02-14'])).toBe(
          '2026-02-14',
        );
      });

      it('returns null when multiple distinct candidate keys match the month/day', () => {
        expect(resolveRecoveryDateSearchKey('Feb 14', ['2025-02-14', '2026-02-14'])).toBeNull();
      });

      it('ignores falsy/blank candidate entries via the String(key || "") defensive mapping', () => {
        const keys = [null, undefined, '', '2026-02-14'] as unknown as readonly string[];
        expect(resolveRecoveryDateSearchKey('Feb 14', keys)).toBe('2026-02-14');
      });

      it('de-duplicates repeated matching candidate keys before the count check', () => {
        expect(
          resolveRecoveryDateSearchKey('Feb 14', ['2026-02-14', '2026-02-14', '2026-02-14']),
        ).toBe('2026-02-14');
      });

      it('falls back to the current year when candidates are present but none match', () => {
        expect(resolveRecoveryDateSearchKey('Feb 14', ['2026-03-09'], new Date(2026, 0, 1))).toBe(
          '2026-02-14',
        );
      });

      it('falls back to the provided now year when there are no candidates', () => {
        expect(resolveRecoveryDateSearchKey('Feb 14', [], new Date(2026, 4, 14))).toBe(
          '2026-02-14',
        );
      });

      it('returns null when the fallback date rolls over for the current year (Feb 30)', () => {
        expect(resolveRecoveryDateSearchKey('Feb 30', [], new Date(2026, 0, 1))).toBeNull();
      });
    });
  });
});
