/**
 * Metrics View Mode Store
 *
 * Global preference for displaying metrics as progress bars or sparklines.
 * This affects all tables: node summaries, guest tables, and container tables.
 */

import { createSignal } from 'solid-js';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { seedFromBackend } from './metricsHistory';

export type MetricsViewMode = 'bars' | 'sparklines';

// Read initial value from localStorage
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

// Create signal
const [metricsViewMode, setMetricsViewMode] = createSignal<MetricsViewMode>(
  getInitialViewMode()
);

/**
 * Get the current metrics view mode
 */
export function getMetricsViewMode(): MetricsViewMode {
  return metricsViewMode();
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
      console.warn('Failed to save metrics view mode preference', err);
    }
  }

  // When switching to sparklines, seed historical data from backend
  if (mode === 'sparklines') {
    // Fire and forget - don't block the UI
    seedFromBackend('1h').catch(() => {
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
  };
}
