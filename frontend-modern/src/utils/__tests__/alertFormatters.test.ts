import { describe, expect, it } from 'vitest';
import { getAlertUnit, formatAlertValue, formatAlertThreshold } from '@/utils/alertFormatters';

describe('alertFormatters', () => {
  describe('getAlertUnit', () => {
    it('returns % for undefined metric type', () => {
      expect(getAlertUnit(undefined)).toBe('%');
    });

    it('returns % for empty string', () => {
      expect(getAlertUnit('')).toBe('%');
    });

    it('returns % for cpu', () => {
      expect(getAlertUnit('cpu')).toBe('%');
    });

    it('returns % for memory', () => {
      expect(getAlertUnit('memory')).toBe('%');
    });

    it('returns % for disk', () => {
      expect(getAlertUnit('disk')).toBe('%');
    });

    it('returns temperature symbol for temperature metric', () => {
      const unit = getAlertUnit('temperature');
      expect(unit).toMatch(/°[CF]/);
    });

    it('returns temperature symbol for temp metric', () => {
      const unit = getAlertUnit('temp');
      expect(unit).toMatch(/°[CF]/);
    });

    it('is case-insensitive for temperature', () => {
      expect(getAlertUnit('TEMPERATURE')).toMatch(/°[CF]/);
      expect(getAlertUnit('Temperature')).toMatch(/°[CF]/);
    });
  });

  describe('formatAlertValue', () => {
    it('returns N/A for undefined value', () => {
      expect(formatAlertValue(undefined)).toBe('N/A');
    });

    it('returns N/A for NaN', () => {
      expect(formatAlertValue(NaN)).toBe('N/A');
    });

    it('returns N/A for non-finite values', () => {
      expect(formatAlertValue(Infinity)).toBe('N/A');
      expect(formatAlertValue(-Infinity)).toBe('N/A');
    });

    it('formats percentage values with default decimals', () => {
      expect(formatAlertValue(82.5)).toBe('82.5%');
      expect(formatAlertValue(100)).toBe('100.0%');
    });

    it('formats percentage values with custom decimals', () => {
      expect(formatAlertValue(82.567, undefined, 2)).toBe('82.57%');
      expect(formatAlertValue(82.5, undefined, 0)).toBe('83%');
    });

    it('formats temperature values with symbol', () => {
      const unit = getAlertUnit('temperature');
      expect(formatAlertValue(74, 'temperature')).toBe(`74.0${unit}`);
    });
  });

  describe('formatAlertThreshold', () => {
    it('returns Not configured for undefined', () => {
      expect(formatAlertThreshold(undefined)).toBe('Not configured');
    });

    it('returns Not configured for NaN', () => {
      expect(formatAlertThreshold(NaN)).toBe('Not configured');
    });

    it('returns Disabled for 0', () => {
      expect(formatAlertThreshold(0)).toBe('Disabled');
    });

    it('returns Disabled for negative values', () => {
      expect(formatAlertThreshold(-1)).toBe('Disabled');
    });

    it('formats positive percentage values', () => {
      expect(formatAlertThreshold(80)).toBe('80%');
      expect(formatAlertThreshold(90.5)).toBe('90.5%');
    });

    it('formats temperature thresholds', () => {
      const unit = getAlertUnit('temperature');
      expect(formatAlertThreshold(70, 'temperature')).toBe(`70${unit}`);
    });
  });
});
