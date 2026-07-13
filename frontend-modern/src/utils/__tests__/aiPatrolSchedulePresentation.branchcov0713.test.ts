import { describe, expect, it } from 'vitest';
import {
  buildPatrolScheduleOptions,
  getPatrolScheduleLabel,
  PATROL_SCHEDULE_PRESETS,
} from '@/utils/aiPatrolSchedulePresentation';

// Branch-coverage additions for the two currently under-exercised functions in
// aiPatrolSchedulePresentation.ts. The sibling suites already cover every
// canonical preset value and several finite unmatched integers/floats for
// getPatrolScheduleLabel, plus the preset(360) and custom(17) arms of
// buildPatrolScheduleOptions. This file targets the remaining arms:
//   - getPatrolScheduleLabel: non-finite numbers and coerced non-number inputs
//     that still flow through the template-literal fallback.
//   - buildPatrolScheduleOptions: the Number.isFinite short-circuit arm, the
//     preset-match (B-false) arm beyond value 360, the comparator sort
//     positions (front / middle / end), reference freshness, and source-array
//     immutability.

describe('aiPatrolSchedulePresentation — branch coverage (batch 0713)', () => {
  describe('getPatrolScheduleLabel — non-finite / coerced fallback arm', () => {
    it('falls back to a templated label for NaN (no preset matches)', () => {
      // NaN is a valid `number` value; it matches no preset, so the
      // `${minutes} min` template literal arm fires and String-coerces NaN.
      expect(getPatrolScheduleLabel(NaN)).toBe('NaN min');
    });

    it('falls back to a templated label for +Infinity', () => {
      expect(getPatrolScheduleLabel(Infinity)).toBe('Infinity min');
    });

    it('falls back to a templated label (with leading minus) for -Infinity', () => {
      expect(getPatrolScheduleLabel(-Infinity)).toBe('-Infinity min');
    });

    it('String-coerces a null input to "null min" through the fallback arm', () => {
      // `null` violates the declared `number` type but is observable through
      // the exported API; it does not match any preset, so the template
      // literal interpolates `String(null)` === 'null'.
      const minutes = null as unknown as Parameters<typeof getPatrolScheduleLabel>[0];
      expect(getPatrolScheduleLabel(minutes)).toBe('null min');
    });

    it('String-coerces an undefined input to "undefined min" through the fallback arm', () => {
      const minutes = undefined as unknown as Parameters<typeof getPatrolScheduleLabel>[0];
      expect(getPatrolScheduleLabel(minutes)).toBe('undefined min');
    });

    it('String-coerces an empty-string input to " min" through the fallback arm', () => {
      // '' is non-nullish but matches no preset value, so the fallback runs.
      const minutes = '' as unknown as Parameters<typeof getPatrolScheduleLabel>[0];
      expect(getPatrolScheduleLabel(minutes)).toBe(' min');
    });
  });

  describe('buildPatrolScheduleOptions — Number.isFinite short-circuit (A-false) arm', () => {
    it('returns the canonical presets unchanged when current is NaN', () => {
      // `Number.isFinite(NaN)` is false → the && short-circuits and no custom
      // option is appended.
      const result = buildPatrolScheduleOptions(NaN);
      expect(result).toEqual([...PATROL_SCHEDULE_PRESETS]);
      expect(result).toHaveLength(PATROL_SCHEDULE_PRESETS.length);
    });

    it('returns the canonical presets unchanged when current is +Infinity', () => {
      const result = buildPatrolScheduleOptions(Infinity);
      expect(result).toEqual([...PATROL_SCHEDULE_PRESETS]);
      expect(result).toHaveLength(PATROL_SCHEDULE_PRESETS.length);
    });

    it('returns the canonical presets unchanged when current is -Infinity', () => {
      const result = buildPatrolScheduleOptions(-Infinity);
      expect(result).toEqual([...PATROL_SCHEDULE_PRESETS]);
      expect(result).toHaveLength(PATROL_SCHEDULE_PRESETS.length);
    });

    it('short-circuits for null coerced to number, returning presets unchanged', () => {
      // `Number.isFinite(null)` is false → short-circuit arm fires; no push,
      // no sort. Verifies defensive behavior against a type-violating input.
      const current = null as unknown as Parameters<typeof buildPatrolScheduleOptions>[0];
      const result = buildPatrolScheduleOptions(current);
      expect(result).toEqual([...PATROL_SCHEDULE_PRESETS]);
    });

    it('short-circuits for undefined coerced to number, returning presets unchanged', () => {
      const current = undefined as unknown as Parameters<typeof buildPatrolScheduleOptions>[0];
      const result = buildPatrolScheduleOptions(current);
      expect(result).toEqual([...PATROL_SCHEDULE_PRESETS]);
    });
  });

  describe('buildPatrolScheduleOptions — preset-match (B-false) arm', () => {
    it('returns presets unchanged for the 0 preset (boundary value, also falsy)', () => {
      // Guards against a regression where the some() predicate is replaced
      // with a truthy check that would mishandle the value 0.
      const result = buildPatrolScheduleOptions(0);
      expect(result).toEqual([...PATROL_SCHEDULE_PRESETS]);
      expect(result).toContainEqual({ value: 0, label: 'Off' });
    });

    it('returns presets unchanged for the max preset (1440)', () => {
      const result = buildPatrolScheduleOptions(1440);
      expect(result).toEqual([...PATROL_SCHEDULE_PRESETS]);
      expect(result).toContainEqual({ value: 1440, label: '24 hours' });
    });

    it('returns presets unchanged for the smallest non-zero preset (10)', () => {
      const result = buildPatrolScheduleOptions(10);
      expect(result).toEqual([...PATROL_SCHEDULE_PRESETS]);
    });
  });

  describe('buildPatrolScheduleOptions — custom value push + sort positions', () => {
    it('inserts a negative custom value at the front (sorted ascending)', () => {
      // Exercises the comparator `(a, b) => a.value - b.value` returning a
      // positive value, moving the new entry to index 0.
      const result = buildPatrolScheduleOptions(-5);
      expect(result).toHaveLength(PATROL_SCHEDULE_PRESETS.length + 1);
      expect(result[0]).toStrictEqual({ value: -5, label: '-5 min' });
      expect(result[1]).toStrictEqual({ value: 0, label: 'Off' });
      // Whole array remains sorted ascending by value.
      const values = result.map((o) => o.value);
      const sorted = [...values].sort((a, b) => a - b);
      expect(values).toEqual(sorted);
    });

    it('inserts a custom value that falls between two presets at the right middle index', () => {
      // 45 sits between preset 30 and preset 60 → must land at index 4.
      const result = buildPatrolScheduleOptions(45);
      expect(result).toHaveLength(PATROL_SCHEDULE_PRESETS.length + 1);
      expect(result[4]).toStrictEqual({ value: 45, label: '45 min' });
      expect(result[3]).toStrictEqual({ value: 30, label: '30 min' });
      expect(result[5]).toStrictEqual({ value: 60, label: '1 hour' });
    });

    it('inserts a custom value larger than the max preset at the end', () => {
      // Exercises the comparator returning a negative value for every existing
      // entry, leaving the new entry at the last index.
      const result = buildPatrolScheduleOptions(2000);
      expect(result).toHaveLength(PATROL_SCHEDULE_PRESETS.length + 1);
      expect(result[result.length - 1]).toStrictEqual({ value: 2000, label: '2000 min' });
      expect(result[result.length - 2]).toStrictEqual({ value: 1440, label: '24 hours' });
    });

    it('derives the custom entry label from getPatrolScheduleLabel (not a preset label)', () => {
      // 90 is not a preset, so the label is the templated fallback rather than
      // a canonical preset string. Confirms the push reuses the label helper.
      const result = buildPatrolScheduleOptions(90);
      const inserted = result.find((o) => o.value === 90);
      expect(inserted).toBeDefined();
      expect(inserted?.label).toBe(getPatrolScheduleLabel(90));
      expect(inserted?.label).toBe('90 min');
    });
  });

  describe('buildPatrolScheduleOptions — reference + source immutability', () => {
    it('returns a fresh array reference on every call (no shared mutation)', () => {
      const a = buildPatrolScheduleOptions(45);
      const b = buildPatrolScheduleOptions(45);
      expect(a).not.toBe(b);
      expect(a).toEqual(b);
    });

    it('does not mutate PATROL_SCHEDULE_PRESETS after inserting custom values', () => {
      // PATROL_SCHEDULE_PRESETS is declared `readonly`; calling the builder
      // with several custom values must leave the source array untouched.
      const before = [...PATROL_SCHEDULE_PRESETS];
      buildPatrolScheduleOptions(-5);
      buildPatrolScheduleOptions(45);
      buildPatrolScheduleOptions(2000);
      buildPatrolScheduleOptions(NaN);
      expect(PATROL_SCHEDULE_PRESETS).toEqual(before);
      expect(PATROL_SCHEDULE_PRESETS).toHaveLength(before.length);
    });

    it('does not retain prior custom values between independent calls', () => {
      // Each call rebuilds from the presets; a previous custom value must not
      // leak into a subsequent call's result.
      const withCustom = buildPatrolScheduleOptions(45);
      expect(withCustom).toContainEqual({ value: 45, label: '45 min' });
      const after = buildPatrolScheduleOptions(60);
      expect(after).not.toContainEqual({ value: 45, label: '45 min' });
      expect(after).toEqual([...PATROL_SCHEDULE_PRESETS]);
    });
  });
});
