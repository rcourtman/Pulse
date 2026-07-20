import { describe, expect, it } from 'vitest';

import type { RawOverrideConfig } from '@/types/alerts';

import type { Override } from '../types';
import {
  stripStateKeys,
  upsertOverride,
  withThresholdEntries,
} from '../thresholdsOverrideMutationModel';

const makeOverride = (id: string, thresholds: Override['thresholds'] = {}): Override =>
  ({
    id,
    name: id,
    type: 'guest',
    thresholds,
  }) as Override;

describe('thresholdsOverrideMutationModel', () => {
  describe('upsertOverride', () => {
    it('appends a new override when no matching id exists', () => {
      const existing = [makeOverride('a'), makeOverride('b')];
      const next = upsertOverride(existing, makeOverride('c'));

      expect(next.map((o) => o.id)).toEqual(['a', 'b', 'c']);
    });

    it('replaces an existing override with the same id in place', () => {
      const existing = [makeOverride('a'), makeOverride('b'), makeOverride('c')];
      const replacement = makeOverride('b', { cpu: 90 });
      const next = upsertOverride(existing, replacement);

      expect(next.map((o) => o.id)).toEqual(['a', 'b', 'c']);
      expect(next[1]).toBe(replacement);
    });

    it('replaces only the first matching id', () => {
      const dup = makeOverride('b', { cpu: 1 });
      const existing = [makeOverride('a'), dup, makeOverride('b', { cpu: 2 })];
      const replacement = makeOverride('b', { cpu: 90 });
      const next = upsertOverride(existing, replacement);

      expect(next[1]).toBe(replacement);
      expect(next[2]).toBe(existing[2]);
    });

    it('appends to an empty list', () => {
      const next = upsertOverride([], makeOverride('a'));
      expect(next.map((o) => o.id)).toEqual(['a']);
    });

    it('does not mutate the input array', () => {
      const existing = [makeOverride('a')];
      upsertOverride(existing, makeOverride('b'));
      expect(existing.map((o) => o.id)).toEqual(['a']);
    });

    it('preserves the order of unrelated overrides when replacing', () => {
      const existing = [makeOverride('a'), makeOverride('b'), makeOverride('c'), makeOverride('d')];
      const next = upsertOverride(existing, makeOverride('c', { cpu: 90 }));
      expect(next.map((o) => o.id)).toEqual(['a', 'b', 'c', 'd']);
      expect(next[2].thresholds).toEqual({ cpu: 90 });
    });
  });

  describe('withThresholdEntries', () => {
    it('adds hysteresis entries for each provided threshold', () => {
      const result = withThresholdEntries({}, { cpu: 90, memory: 80 });
      expect(result).toEqual({
        cpu: { trigger: 90, clear: 85 },
        memory: { trigger: 80, clear: 75 },
      });
    });

    it('clamps the clear value to zero when the trigger is below the margin', () => {
      expect(withThresholdEntries({}, { cpu: 3 })).toEqual({
        cpu: { trigger: 3, clear: 0 },
      });
    });

    it('clamps the clear value to zero when the trigger is exactly the margin', () => {
      expect(withThresholdEntries({}, { cpu: 5 })).toEqual({
        cpu: { trigger: 5, clear: 0 },
      });
    });

    it('keeps the clear value at zero for a zero trigger', () => {
      expect(withThresholdEntries({}, { cpu: 0 })).toEqual({
        cpu: { trigger: 0, clear: 0 },
      });
    });

    it('clamps the clear value to zero for a negative trigger (max(0, ...))', () => {
      expect(withThresholdEntries({}, { cpu: -10 })).toEqual({
        cpu: { trigger: -10, clear: 0 },
      });
    });

    it.each([
      ['undefined', undefined],
      ['null', null],
    ])('skips %s threshold values', (_label, value) => {
      expect(withThresholdEntries({}, { cpu: value as number | undefined, memory: 80 })).toEqual({
        memory: { trigger: 80, clear: 75 },
      });
    });

    it('preserves existing entries on the raw config and overwrites metrics by name', () => {
      const base = {
        cpu: { trigger: 50, clear: 45 },
        note: 'keep me',
      } as RawOverrideConfig;
      const result = withThresholdEntries(base, { cpu: 90, disk: 70 });

      expect(result).toEqual({
        cpu: { trigger: 90, clear: 85 },
        disk: { trigger: 70, clear: 65 },
        note: 'keep me',
      });
    });

    it('does not mutate the input raw config', () => {
      const base = { cpu: { trigger: 50, clear: 45 } } as RawOverrideConfig;
      const snapshot = { ...base };
      withThresholdEntries(base, { cpu: 90 });
      expect(base).toEqual(snapshot);
    });

    it('returns an empty config for an empty thresholds map', () => {
      expect(withThresholdEntries({}, {})).toEqual({});
    });
  });

  describe('stripStateKeys', () => {
    it('removes disabled, disableConnectivity, and poweredOffSeverity', () => {
      const input = {
        cpu: 90,
        memory: 80,
        disabled: true,
        disableConnectivity: true,
        poweredOffSeverity: 'critical',
      } as unknown as Record<string, number>;
      expect(stripStateKeys(input)).toEqual({ cpu: 90, memory: 80 });
    });

    it('is a no-op when no state keys are present', () => {
      const input = { cpu: 90, memory: 80 } as Record<string, number>;
      expect(stripStateKeys(input)).toEqual({ cpu: 90, memory: 80 });
    });

    it('returns a new object and does not mutate the input', () => {
      const input = { cpu: 90, disabled: 1 } as unknown as Record<string, number>;
      const result = stripStateKeys(input);
      expect(result).not.toBe(input);
      expect((input as Record<string, unknown>).disabled).toBe(1);
    });

    it('handles an empty object', () => {
      expect(stripStateKeys({})).toEqual({});
    });
  });
});
