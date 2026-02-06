/**
 * Sparkline Data Store
 *
 * Fetches chart data from the backend and holds it in a reactive signal.
 * Replaces the old ring-buffer + sampler architecture with a simple
 * fetch-store-timer pattern.
 */

import { createSignal } from 'solid-js';
import { ChartsAPI, type ChartData, type ChartsResponse, type MetricPoint, type TimeRange } from '@/api/charts';
import { buildMetricKey } from '@/utils/metricsKeys';
import { logger } from '@/utils/logger';

/** Per-resource chart data (cpu/memory/disk point arrays) */
export interface ResourceChartData {
  cpu: MetricPoint[];
  memory: MetricPoint[];
  disk: MetricPoint[];
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

const [sparklineStore, setSparklineStore] = createSignal<Map<string, ResourceChartData>>(new Map());
const [sparklineLoading, setSparklineLoading] = createSignal(false);

let refreshTimer: ReturnType<typeof setInterval> | null = null;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Choose a refresh interval (ms) based on the selected range. */
function refreshIntervalForRange(range: TimeRange): number {
  switch (range) {
    case '5m':
    case '15m':
    case '1h':
      return 10_000;
    case '4h':
      return 30_000;
    case '24h':
      return 60_000;
    case '30m':
    case '12h':
      return 30_000;
    case '7d':
    case '30d':
      return 120_000;
    default:
      return 30_000;
  }
}

/** Convert a ChartsResponse into our flat Map<metricKey, ResourceChartData>. */
function processChartsResponse(response: ChartsResponse): Map<string, ResourceChartData> {
  const map = new Map<string, ResourceChartData>();

  const store = (key: string, chartData: ChartData) => {
    map.set(key, {
      cpu: chartData.cpu ?? [],
      memory: chartData.memory ?? [],
      disk: chartData.disk ?? [],
    });
  };

  // Guests (VMs / containers)
  if (response.data) {
    // Prefer backend-provided guest types; fall back to 'vm' when unavailable.
    const guestTypes = response.guestTypes ?? {};
    for (const [id, chartData] of Object.entries(response.data)) {
      const kind = guestTypes[id] ?? 'vm';
      store(buildMetricKey(kind, id), chartData);
    }
  }

  // Nodes
  if (response.nodeData) {
    for (const [id, chartData] of Object.entries(response.nodeData)) {
      store(buildMetricKey('node', id), chartData);
    }
  }

  // Docker containers
  if (response.dockerData) {
    for (const [id, chartData] of Object.entries(response.dockerData)) {
      store(buildMetricKey('dockerContainer', id), chartData);
    }
  }

  // Docker hosts
  if (response.dockerHostData) {
    for (const [id, chartData] of Object.entries(response.dockerHostData)) {
      store(buildMetricKey('dockerHost', id), chartData);
    }
  }

  // Unified host agents
  if (response.hostData) {
    for (const [id, chartData] of Object.entries(response.hostData)) {
      store(buildMetricKey('host', id), chartData);
    }
  }

  return map;
}

// ---------------------------------------------------------------------------
// Fetch
// ---------------------------------------------------------------------------

async function fetchAndStore(range: TimeRange, isInitial: boolean): Promise<void> {
  if (isInitial) setSparklineLoading(true);

  try {
    const response = await ChartsAPI.getCharts(range);
    const data = processChartsResponse(response);
    setSparklineStore(data);

    logger.debug('[SparklineData] Fetched', { range, resources: data.size });
  } catch (error) {
    // On background refresh failure, keep stale data visible.
    logger.error('[SparklineData] Fetch failed', { error, range });
  } finally {
    if (isInitial) setSparklineLoading(false);
  }
}

// ---------------------------------------------------------------------------
// Timer management
// ---------------------------------------------------------------------------

function stopTimer() {
  if (refreshTimer !== null) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
}

function startTimer(range: TimeRange) {
  stopTimer();
  const interval = refreshIntervalForRange(range);
  refreshTimer = setInterval(() => {
    void fetchAndStore(range, false);
  }, interval);
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Begin fetching sparkline data for the given range.
 * Starts a periodic refresh timer.
 */
export function activateSparklines(range: TimeRange): void {
  void fetchAndStore(range, true);
  startTimer(range);
}

/**
 * Switch to a new time range. Re-fetches immediately and restarts the
 * refresh timer with an interval appropriate for the new range.
 */
export function changeSparklineRange(range: TimeRange): void {
  void fetchAndStore(range, true);
  startTimer(range);
}

/**
 * Stop the refresh timer. Keeps the last fetched data in the store
 * so re-activation is instant (stale data shown while fresh fetch runs).
 */
export function deactivateSparklines(): void {
  stopTimer();
}

/**
 * Return MetricPoint[] for one resource+metric.
 * This is a plain function (not reactive) — callers should depend on
 * getSparklineStore() for reactivity.
 */
export function getSparklineData(
  resourceKey: string,
  metric: 'cpu' | 'memory' | 'disk',
): MetricPoint[] {
  const entry = sparklineStore().get(resourceKey);
  if (!entry) return [];
  return entry[metric];
}

/** Reactive signal accessor — use in createMemo for dependency tracking. */
export function getSparklineStore(): Map<string, ResourceChartData> {
  return sparklineStore();
}

/** True only during the initial fetch (not background refreshes). */
export function isSparklineLoading(): boolean {
  return sparklineLoading();
}

// ---------------------------------------------------------------------------
// Cleanup orphaned localStorage from the old ring-buffer store
// ---------------------------------------------------------------------------

if (typeof window !== 'undefined') {
  try {
    localStorage.removeItem('pulse_metrics_history');
  } catch {
    // Ignore
  }
}
