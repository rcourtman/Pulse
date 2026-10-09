import { describe, expect, it } from 'vitest';
import { parseRecoveryDateKey } from '@/utils/recoveryDatePresentation';

describe('recoveryDatePresentation branch coverage (part 2)', () => {
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
});
