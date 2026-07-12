import { describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/temperature', () => ({
  formatTemperature: (value: number) => `${value}°C`,
  getTemperatureSymbol: () => '°C',
}));

import { getThresholdSliderPosition } from '../thresholdSliderModel';

describe('getThresholdSliderPosition (branch coverage 2)', () => {
  describe('default bounds (min ?? 0 and max ?? 100 both fall through)', () => {
    it('returns the in-range percent using the default 0-100 bounds (45 -> 45)', () => {
      expect(getThresholdSliderPosition(45)).toBe(45);
    });

    it('returns 50 for the midpoint of the default bounds', () => {
      expect(getThresholdSliderPosition(50)).toBe(50);
    });

    it('returns 0 at the minimum boundary (value === min -> percent 0, Math.max passthrough)', () => {
      expect(getThresholdSliderPosition(0)).toBe(0);
    });

    it('returns 100 at the maximum boundary (value === max -> Math.min passthrough at 100)', () => {
      expect(getThresholdSliderPosition(100)).toBe(100);
    });
  });

  describe('custom bounds (both min and max provided, no ?? defaults used)', () => {
    it('maps a value to a percent across a custom [50,100] range (60 -> 20)', () => {
      expect(getThresholdSliderPosition(60, 50, 100)).toBe(20);
    });

    it('returns 0 when the value sits at the custom min', () => {
      expect(getThresholdSliderPosition(50, 50, 100)).toBe(0);
    });

    it('returns 100 when the value sits at the custom max', () => {
      expect(getThresholdSliderPosition(100, 50, 100)).toBe(100);
    });

    it('supports a negative min bound (0 over [-50,50] -> 50)', () => {
      expect(getThresholdSliderPosition(0, -50, 50)).toBe(50);
    });

    it('returns a fractional percent when the range does not divide evenly (25 over [0,200] -> 12.5)', () => {
      expect(getThresholdSliderPosition(25, 0, 200)).toBe(12.5);
    });
  });

  describe('mixed default/custom bounds (only one ?? arm falls through)', () => {
    it('uses the default min when min is undefined and a custom max is supplied (50 over [0,200] -> 25)', () => {
      expect(getThresholdSliderPosition(50, undefined, 200)).toBe(25);
    });

    it('uses the default max when max is undefined and a custom min is supplied (75 over [50,100] -> 50)', () => {
      expect(getThresholdSliderPosition(75, 50, undefined)).toBe(50);
    });
  });

  describe('clamping arms of Math.max(0, Math.min(100, percent))', () => {
    it('clamps a value above max down to 100 (Math.min(100, percent) -> 100)', () => {
      expect(getThresholdSliderPosition(150)).toBe(100);
    });

    it('clamps a value far above a custom max down to 100', () => {
      expect(getThresholdSliderPosition(999, 50, 100)).toBe(100);
    });

    it('clamps a value below min up to 0 (Math.max(0, percent) -> 0)', () => {
      expect(getThresholdSliderPosition(-20)).toBe(0);
    });

    it('clamps a value below a custom min up to 0', () => {
      expect(getThresholdSliderPosition(10, 50, 100)).toBe(0);
    });
  });

  describe('degenerate range guard (range <= 0 early return)', () => {
    it('returns 0 when min equals max (range === 0 boundary)', () => {
      expect(getThresholdSliderPosition(50, 50, 50)).toBe(0);
    });

    it('returns 0 when min exceeds max (range strictly negative)', () => {
      expect(getThresholdSliderPosition(50, 100, 50)).toBe(0);
    });

    it('returns 0 at the degenerate guard even when the value is below the min', () => {
      expect(getThresholdSliderPosition(-100, 100, 100)).toBe(0);
    });
  });

  describe('non-finite value (observed behavior)', () => {
    it('propagates NaN through the clamp instead of coercing to 0 (robustness gap)', () => {
      const result = getThresholdSliderPosition(Number.NaN, 0, 100);
      expect(result).toBeNaN();
    });
  });
});
