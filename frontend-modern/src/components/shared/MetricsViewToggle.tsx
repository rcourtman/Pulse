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
        class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${viewMode() === 'bars'
            ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
          }`}
        title="Bar view"
      >
        <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="3" y="12" width="4" height="9" rx="1" />
          <rect x="10" y="6" width="4" height="15" rx="1" />
          <rect x="17" y="3" width="4" height="18" rx="1" />
        </svg>
        Bars
      </button>
      <button
        type="button"
        onClick={() => setViewMode('sparklines')}
        class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${viewMode() === 'sparklines'
            ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50'
          }`}
        title="Sparkline view"
      >
        <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="3 17 9 11 13 15 21 7" />
          <polyline points="17 7 21 7 21 11" />
        </svg>
        Trends
      </button>
    </div>
  );
};
