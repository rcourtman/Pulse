import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  celsiusToFahrenheit,
  formatTemperature,
  getTemperatureSymbol,
  getCpuTemperature,
  getTemperatureTextClass,
  temperatureStore,
} from '../temperature';

describe('temperature', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  describe('celsiusToFahrenheit', () => {
    it('converts correctly', () => {
      expect(celsiusToFahrenheit(0)).toBe(32);
      expect(celsiusToFahrenheit(100)).toBe(212);
      expect(celsiusToFahrenheit(-40)).toBe(-40);
      expect(celsiusToFahrenheit(37)).toBeCloseTo(98.6);
    });
  });

  describe('formatTemperature', () => {
    it('formats Celsius correctly by default', () => {
      temperatureStore.setUnit('celsius');
      expect(formatTemperature(25)).toBe('25°C');
      expect(formatTemperature(25.6, { decimals: 1 })).toBe('25.6°C');
      expect(formatTemperature(25.6, { decimals: 0 })).toBe('26°C');
    });

    it('formats Fahrenheit correctly', () => {
      temperatureStore.setUnit('fahrenheit');
      expect(formatTemperature(0)).toBe('32°F');
      expect(formatTemperature(100)).toBe('212°F');
    });

    it('handles null/undefined/NaN', () => {
      expect(formatTemperature(null)).toBe('—');
      expect(formatTemperature(undefined)).toBe('—');
      expect(formatTemperature(NaN)).toBe('—');
    });

    it('respects showUnit option', () => {
      temperatureStore.setUnit('celsius');
      expect(formatTemperature(25, { showUnit: false })).toBe('25');
    });
  });

  describe('getTemperatureSymbol', () => {
    it('returns correct symbol', () => {
      temperatureStore.setUnit('celsius');
      expect(getTemperatureSymbol()).toBe('°C');
      temperatureStore.setUnit('fahrenheit');
      expect(getTemperatureSymbol()).toBe('°F');
    });
  });

  describe('getCpuTemperature', () => {
    it('returns null if not available', () => {
      expect(getCpuTemperature(null)).toBeNull();
      expect(getCpuTemperature({ available: false, lastUpdate: '' } as any)).toBeNull();
    });

    it('picks the maximum value from various sources', () => {
      const temp = {
        available: true,
        lastUpdate: '',
        cpuPackage: 50,
        cpuMax: 55,
        cores: [
          { name: 'Core 0', temp: 52 },
          { name: 'Core 1', temp: 58 },
        ],
      };
      // Should pick 58
      expect(getCpuTemperature(temp as any)).toBe(58);
    });

    it('handles missing fields gracefully', () => {
      const temp = {
        available: true,
        lastUpdate: '',
        cpuPackage: 60,
        // others missing
      };
      expect(getCpuTemperature(temp as any)).toBe(60);
    });

    it('filters invalid temperatures', () => {
      const temp = {
        available: true,
        lastUpdate: '',
        cpuPackage: NaN,
        cpuMax: Infinity,
        cores: [{ name: 'C1', temp: 45 }],
      };
      expect(getCpuTemperature(temp as any)).toBe(45);
    });

    it('returns null if no valid temperatures found', () => {
      const temp = {
        available: true,
        lastUpdate: '',
        cpuPackage: NaN,
        cores: [],
      };
      expect(getCpuTemperature(temp as any)).toBeNull();
    });
  });

  describe('getTemperatureTextClass', () => {
    it('returns muted tone for nullish or invalid values', () => {
      expect(getTemperatureTextClass(null)).toBe('text-muted');
      expect(getTemperatureTextClass(undefined)).toBe('text-muted');
      expect(getTemperatureTextClass(NaN)).toBe('text-muted');
    });

    it('returns green for safe temperatures', () => {
      expect(getTemperatureTextClass(45)).toContain('text-green-600');
    });

    it('returns amber for elevated temperatures', () => {
      expect(getTemperatureTextClass(60)).toContain('text-amber-600');
    });

    it('returns red for high temperatures', () => {
      expect(getTemperatureTextClass(70)).toContain('text-red-600');
    });
  });

  describe('temperatureStore persistence', () => {
    it('saves to localStorage', () => {
      const spy = vi.spyOn(Storage.prototype, 'setItem');
      temperatureStore.setUnit('fahrenheit');
      expect(spy).toHaveBeenCalledWith('temperatureUnit', 'fahrenheit');
    });
  });
});
