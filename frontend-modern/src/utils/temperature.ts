import type { Temperature } from '@/types/api';
import { createSignal, createRoot } from 'solid-js';
import {
  getDefaultDisplayMetricThresholds,
  getMetricSeverity,
  type DisplayMetricType,
  type MetricDisplayThresholds,
} from './metricThresholds';
import { STORAGE_KEYS } from './localStorage';

export type TemperatureUnit = 'celsius' | 'fahrenheit';

// Reactive temperature unit store - created once and shared across the app
const createTemperatureStore = () => {
  // Load initial value from localStorage
  const stored = localStorage.getItem(STORAGE_KEYS.TEMPERATURE_UNIT);
  const initial: TemperatureUnit = stored === 'fahrenheit' ? 'fahrenheit' : 'celsius';

  const [unit, setUnitInternal] = createSignal<TemperatureUnit>(initial);

  const setUnit = (newUnit: TemperatureUnit) => {
    localStorage.setItem(STORAGE_KEYS.TEMPERATURE_UNIT, newUnit);
    setUnitInternal(newUnit);
  };

  return { unit, setUnit };
};

// Create the store at module level using createRoot to avoid context issues
export const temperatureStore = createRoot(createTemperatureStore);

/**
 * Convert Celsius to Fahrenheit
 */
export const celsiusToFahrenheit = (celsius: number): number => {
  return (celsius * 9) / 5 + 32;
};

/**
 * Format a temperature value with the appropriate unit symbol.
 * Uses the global temperature unit preference.
 * @param celsius - Temperature in Celsius
 * @param options - Formatting options
 * @returns Formatted temperature string (e.g., "72°C" or "162°F")
 */
export const formatTemperature = (
  celsius: number | null | undefined,
  options: { showUnit?: boolean; decimals?: number } = {},
): string => {
  const { showUnit = true, decimals = 0 } = options;

  if (celsius === null || celsius === undefined || !Number.isFinite(celsius)) {
    return '—';
  }

  const unit = temperatureStore.unit();
  const value = unit === 'fahrenheit' ? celsiusToFahrenheit(celsius) : celsius;
  const symbol = unit === 'fahrenheit' ? '°F' : '°C';

  const formatted = decimals > 0 ? value.toFixed(decimals) : Math.round(value).toString();

  return showUnit ? `${formatted}${symbol}` : formatted;
};

/**
 * Get the current temperature unit symbol
 */
export const getTemperatureSymbol = (): string => {
  return temperatureStore.unit() === 'fahrenheit' ? '°F' : '°C';
};

const isValidTemperature = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value);

export const getCpuTemperature = (temperature?: Temperature | null): number | null => {
  if (!temperature?.available) return null;

  const candidates: number[] = [];

  if (isValidTemperature(temperature.cpuPackage)) {
    candidates.push(temperature.cpuPackage);
  }
  if (isValidTemperature(temperature.cpuMax)) {
    candidates.push(temperature.cpuMax);
  }
  if (Array.isArray(temperature.cores)) {
    temperature.cores.forEach((core) => {
      if (isValidTemperature(core.temp)) {
        candidates.push(core.temp);
      }
    });
  }

  if (candidates.length === 0) {
    return null;
  }

  return Math.max(...candidates);
};

const TEMPERATURE_TEXT_CLASSES = {
  critical: 'text-red-600 dark:text-red-400',
  warning: 'text-amber-600 dark:text-amber-400',
  normal: 'text-green-600 dark:text-green-400',
} as const;

export type TemperatureDisplayMetric = Extract<
  DisplayMetricType,
  'temperature' | 'diskTemperature'
>;

export const getTemperatureTextClass = (
  celsius: number | null | undefined,
  thresholds?: MetricDisplayThresholds | null,
  metric: TemperatureDisplayMetric = 'temperature',
): string => {
  if (celsius === null || celsius === undefined || !Number.isFinite(celsius)) {
    return 'text-muted';
  }
  return TEMPERATURE_TEXT_CLASSES[
    getMetricSeverity(celsius, metric, thresholds ?? getDefaultDisplayMetricThresholds(metric))
  ];
};
