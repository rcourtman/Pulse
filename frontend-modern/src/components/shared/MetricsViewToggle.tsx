/**
 * Metrics View Mode Toggle
 *
 * Toggle button to switch between progress bars and sparkline views for metrics.
 * This is a global setting that affects all tables throughout the app.
 */

import { Component } from 'solid-js';
import { useMetricsViewMode } from '@/stores/metricsViewMode';

export const MetricsViewToggle: Component = () => {
  const { viewMode, setViewMode } = useMetricsViewMode();

  return (
    <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
      <button
        type="button"
        onClick={() => setViewMode('bars')}
        class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
          viewMode() === 'bars'
            ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
        }`}
        title="Bar view"
      >
        Bars
      </button>
      <button
        type="button"
        onClick={() => setViewMode('sparklines')}
        class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
          viewMode() === 'sparklines'
            ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
        }`}
        title="Sparkline view"
      >
        Trends
      </button>
    </div>
  );
};
