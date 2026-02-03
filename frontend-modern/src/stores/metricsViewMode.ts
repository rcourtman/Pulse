/**
 * Metrics View Mode Store
 *
 * Global preference for displaying metrics as progress bars or sparklines.
 * Also manages the time range for sparkline data.
 * This affects all tables: node summaries, guest tables, and container tables.
 */

import { createSignal } from 'solid-js';
import { logger } from '@/utils/logger';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { seedFromBackend, resetSeedingState } from './metricsHistory';
import type { TimeRange } from '@/api/charts';

export type MetricsViewMode = 'bars' | 'sparklines';

// Available time ranges for sparklines
export const TIME_RANGE_OPTIONS: { value: TimeRange; label: string }[] = [
  { value: '15m', label: '15m' },
  { value: '1h', label: '1h' },
  { value: '4h', label: '4h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
];

// Read initial view mode from localStorage
const getInitialViewMode = (): MetricsViewMode => {
  if (typeof window === 'undefined') return 'bars';

  try {
    const stored = localStorage.getItem(STORAGE_KEYS.METRICS_VIEW_MODE);
    if (stored === 'sparklines' || stored === 'bars') {
      return stored;
    }
  } catch (_err) {
    // Ignore localStorage errors
  }

  return 'bars'; // Default to bars (current behavior)
};

// Read initial time range from localStorage
const getInitialTimeRange = (): TimeRange => {
  if (typeof window === 'undefined') return '1h';

  try {
    const stored = localStorage.getItem(STORAGE_KEYS.METRICS_TIME_RANGE);
    if (stored && ['5m', '15m', '30m', '1h', '4h', '12h', '24h', '7d', '30d'].includes(stored)) {
      return stored as TimeRange;
    }
  } catch (_err) {
    // Ignore localStorage errors
  }

  return '1h'; // Default to 1 hour
};

// Create signals
const [metricsViewMode, setMetricsViewMode] = createSignal<MetricsViewMode>(
  getInitialViewMode()
);

const [metricsTimeRange, setMetricsTimeRange] = createSignal<TimeRange>(
  getInitialTimeRange()
);

/**
 * Get the current metrics view mode
 */
export function getMetricsViewMode(): MetricsViewMode {
  return metricsViewMode();
}

/**
 * Get the current metrics time range
 */
export function getMetricsTimeRange(): TimeRange {
  return metricsTimeRange();
}

/**
 * Set the metrics view mode and persist to localStorage
 */
export function setMetricsViewModePreference(mode: MetricsViewMode): void {
  setMetricsViewMode(mode);

  if (typeof window !== 'undefined') {
    try {
      localStorage.setItem(STORAGE_KEYS.METRICS_VIEW_MODE, mode);
    } catch (err) {
      // Ignore localStorage errors
      logger.warn('Failed to save metrics view mode preference', err);
    }
  }

  // When switching to sparklines, seed historical data from backend
  if (mode === 'sparklines') {
    // Fire and forget - don't block the UI
    seedFromBackend(metricsTimeRange()).catch(() => {
      // Errors are already logged in seedFromBackend
    });
  }
}

/**
 * Set the metrics time range and persist to localStorage
 */
export function setMetricsTimeRangePreference(range: TimeRange): void {
  const previousRange = metricsTimeRange();
  setMetricsTimeRange(range);

  if (typeof window !== 'undefined') {
    try {
      localStorage.setItem(STORAGE_KEYS.METRICS_TIME_RANGE, range);
    } catch (err) {
      // Ignore localStorage errors
      logger.warn('Failed to save metrics time range preference', err);
    }
  }

  // If we're in sparklines mode and range changed, re-seed from backend
  if (metricsViewMode() === 'sparklines' && range !== previousRange) {
    // Reset seeding state to force a fresh fetch
    resetSeedingState();
    // Fire and forget - don't block the UI
    seedFromBackend(range).catch(() => {
      // Errors are already logged in seedFromBackend
    });
  }
}

/**
 * Toggle between bars and sparklines
 */
export function toggleMetricsViewMode(): void {
  const current = metricsViewMode();
  const next: MetricsViewMode = current === 'bars' ? 'sparklines' : 'bars';
  setMetricsViewModePreference(next);
}

/**
 * Hook for components to use the view mode
 */
export function useMetricsViewMode() {
  return {
    viewMode: metricsViewMode,
    setViewMode: setMetricsViewModePreference,
    toggle: toggleMetricsViewMode,
    timeRange: metricsTimeRange,
    setTimeRange: setMetricsTimeRangePreference,
  };
}
