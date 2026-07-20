import { describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/temperature', () => ({
  formatTemperature: (value: number | null | undefined) =>
    value === null || value === undefined || !Number.isFinite(value) ? '—' : `${value}°C`,
  getTemperatureSymbol: () => '°C',
}));

import type { ThresholdSliderMetricType } from '@/utils/thresholdSliderPresentation';
import {
  DEFAULT_THRESHOLD_SLIDER_MAX,
  DEFAULT_THRESHOLD_SLIDER_MIN,
  getThresholdSliderBounds,
  getThresholdSliderLabel,
  getThresholdSliderThumbTransform,
  getThresholdSliderTitle,
} from '../thresholdSliderModel';

// getThresholdSliderBounds declares both params as `number | undefined`. These
// aliases let us deliberately pass `null` (a nullish value the declared type
// excludes) through `as unknown as` so the `??` fallback arms are exercised
// without resorting to `any`.
type BoundsMin = Parameters<typeof getThresholdSliderBounds>[0];
type BoundsMax = Parameters<typeof getThresholdSliderBounds>[1];

const PERCENT_TYPES: readonly ThresholdSliderMetricType[] = ['cpu', 'memory', 'disk'];

describe('getThresholdSliderBounds (branch coverage 0713)', () => {
  describe('?? fallback arms', () => {
    it('falls back to both default constants when called with no arguments', () => {
      expect(getThresholdSliderBounds()).toEqual({
        min: DEFAULT_THRESHOLD_SLIDER_MIN,
        max: DEFAULT_THRESHOLD_SLIDER_MAX,
      });
    });

    it('uses the provided min and falls back to the default max', () => {
      expect(getThresholdSliderBounds(10)).toEqual({
        min: 10,
        max: DEFAULT_THRESHOLD_SLIDER_MAX,
      });
    });

    it('falls back to the default min and uses the provided max', () => {
      expect(getThresholdSliderBounds(undefined, 200)).toEqual({
        min: DEFAULT_THRESHOLD_SLIDER_MIN,
        max: 200,
      });
    });

    it('uses both provided bounds unchanged', () => {
      expect(getThresholdSliderBounds(10, 200)).toEqual({ min: 10, max: 200 });
    });

    it('returns exactly the {min, max} shape with no extra keys', () => {
      const bounds = getThresholdSliderBounds(5, 95);
      expect(Object.keys(bounds).sort()).toEqual(['max', 'min']);
    });
  });

  describe('null is treated as nullish by ?? (deliberate malformed input)', () => {
    it('falls back to both defaults when both bounds are null', () => {
      expect(
        getThresholdSliderBounds(null as unknown as BoundsMin, null as unknown as BoundsMax),
      ).toEqual({ min: DEFAULT_THRESHOLD_SLIDER_MIN, max: DEFAULT_THRESHOLD_SLIDER_MAX });
    });

    it('falls back to the default min when min is null but a real max is given', () => {
      expect(getThresholdSliderBounds(null as unknown as BoundsMin, 200)).toEqual({
        min: DEFAULT_THRESHOLD_SLIDER_MIN,
        max: 200,
      });
    });

    it('falls back to the default max when max is null but a real min is given', () => {
      expect(getThresholdSliderBounds(10, null as unknown as BoundsMax)).toEqual({
        min: 10,
        max: DEFAULT_THRESHOLD_SLIDER_MAX,
      });
    });
  });
});

describe('getThresholdSliderThumbTransform (branch coverage 0713)', () => {
  describe('position <= 1 arm -> translateX(0%)', () => {
    it.each([
      ['zero', 0],
      ['exactly 1 (low boundary, <=)', 1],
      ['negative', -5],
    ])('returns the left-edge transform for %s', (_label, position) => {
      expect(getThresholdSliderThumbTransform(position)).toBe('translateY(-50%) translateX(0%)');
    });
  });

  describe('middle arm (1 < position < 99) -> translateX(-50%)', () => {
    it.each([
      ['just above the low threshold', 2],
      ['the midpoint', 50],
      ['just below the high threshold', 98],
    ])('returns the centered transform for %s', (_label, position) => {
      expect(getThresholdSliderThumbTransform(position)).toBe('translateY(-50%) translateX(-50%)');
    });
  });

  describe('position >= 99 arm -> translateX(-100%)', () => {
    it.each([
      ['exactly 99 (high boundary, >=)', 99],
      ['100', 100],
      ['far above', 150],
    ])('returns the right-edge transform for %s', (_label, position) => {
      expect(getThresholdSliderThumbTransform(position)).toBe('translateY(-50%) translateX(-100%)');
    });
  });
});

describe('getThresholdSliderTitle (branch coverage 0713)', () => {
  describe("type === 'temperature' arm", () => {
    it('composes "Temperature: " + formatTemperature(value)', () => {
      expect(getThresholdSliderTitle('temperature', 72)).toBe('Temperature: 72°C');
    });

    it('delegates non-finite values to formatTemperature (NaN -> em-dash)', () => {
      expect(getThresholdSliderTitle('temperature', Number.NaN)).toBe('Temperature: —');
    });

    it('delegates null values to formatTemperature (null -> em-dash)', () => {
      expect(getThresholdSliderTitle('temperature', null as unknown as number)).toBe(
        'Temperature: —',
      );
    });
  });

  describe('else arm (type.toUpperCase() + value%)', () => {
    it('uppercases each non-temperature metric type and appends the value percent', () => {
      expect(PERCENT_TYPES.map((t) => getThresholdSliderTitle(t, 50))).toEqual([
        'CPU: 50%',
        'MEMORY: 50%',
        'DISK: 50%',
      ]);
    });

    it('renders 0% for a zero value', () => {
      expect(getThresholdSliderTitle('cpu', 0)).toBe('CPU: 0%');
    });

    it('does not guard NaN in the else arm (observed: "CPU: NaN%")', () => {
      expect(getThresholdSliderTitle('cpu', Number.NaN)).toBe('CPU: NaN%');
    });
  });
});

describe('getThresholdSliderLabel (branch coverage 0713)', () => {
  describe("type === 'temperature' arm", () => {
    it('interpolates the raw value + temperature symbol (does NOT call formatTemperature)', () => {
      expect(getThresholdSliderLabel('temperature', 72)).toBe('72°C');
    });

    it('renders 0°C for a zero value', () => {
      expect(getThresholdSliderLabel('temperature', 0)).toBe('0°C');
    });

    it('does not route NaN through formatTemperature (observed: "NaN°C", unlike the title)', () => {
      expect(getThresholdSliderLabel('temperature', Number.NaN)).toBe('NaN°C');
    });

    it('does not null-guard the raw interpolation (observed: "null°C")', () => {
      expect(getThresholdSliderLabel('temperature', null as unknown as number)).toBe('null°C');
    });
  });

  describe('else arm (value%)', () => {
    it('appends % for each non-temperature metric type', () => {
      expect(PERCENT_TYPES.map((t) => getThresholdSliderLabel(t, 50))).toEqual([
        '50%',
        '50%',
        '50%',
      ]);
    });

    it('renders 0% for a zero value', () => {
      expect(getThresholdSliderLabel('cpu', 0)).toBe('0%');
    });

    it('does not guard NaN in the else arm (observed: "NaN%")', () => {
      expect(getThresholdSliderLabel('cpu', Number.NaN)).toBe('NaN%');
    });
  });
});
