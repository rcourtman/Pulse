import { describe, expect, it } from 'vitest';
import { getPatrolScheduleLabel } from '@/utils/aiPatrolSchedulePresentation';

describe('getPatrolScheduleLabel (branch coverage)', () => {
  describe('preset-found arm: if (preset) return preset.label', () => {
    it('returns the "Off" label when minutes is 0 even though 0 is a falsy value', () => {
      // Guards against a regression where the guard is refactored to
      // `if (minutes)` (which would wrongly fall through for the 0 preset).
      expect(getPatrolScheduleLabel(0)).toBe('Off');
    });

    it('returns the canonical label for every remaining preset value not covered by the sibling suite', () => {
      // Sibling suite already covers 60 -> '1 hour'; here we exercise the
      // find() predicate true-arm for the rest of PATROL_SCHEDULE_PRESETS.
      expect(getPatrolScheduleLabel(10)).toBe('10 min');
      expect(getPatrolScheduleLabel(15)).toBe('15 min');
      expect(getPatrolScheduleLabel(30)).toBe('30 min');
      expect(getPatrolScheduleLabel(180)).toBe('3 hours');
      expect(getPatrolScheduleLabel(360)).toBe('6 hours');
      expect(getPatrolScheduleLabel(720)).toBe('12 hours');
      expect(getPatrolScheduleLabel(1440)).toBe('24 hours');
    });
  });

  describe('preset-not-found arm: return `${minutes} min`', () => {
    it('falls back to a templated label for an unmatched positive integer', () => {
      // Sibling suite already covers 17; use a different value here.
      expect(getPatrolScheduleLabel(5)).toBe('5 min');
    });

    it('falls back to a templated label for a large unmatched value', () => {
      expect(getPatrolScheduleLabel(9999)).toBe('9999 min');
    });

    it('falls back to a templated label (including the minus sign) for a negative value', () => {
      expect(getPatrolScheduleLabel(-5)).toBe('-5 min');
    });

    it('falls back to a templated label that preserves fractional minutes', () => {
      // Exercises String interpolation of a non-integer through the template
      // literal in the fallback branch.
      expect(getPatrolScheduleLabel(1.5)).toBe('1.5 min');
    });
  });
});
