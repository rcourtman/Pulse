import { Component, For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  InteractiveSparkline,
  type InteractiveSparklineSeries,
} from '@/components/shared/InteractiveSparkline';
import { DensityMap } from '@/components/shared/DensityMap';
import {
  ChartsAPI,
  type ChartData,
  type MetricPoint,
  type TimeRange,
  type WorkloadChartsResponse,
} from '@/api/charts';
import {
  SUMMARY_TIME_RANGES,
  SUMMARY_TIME_RANGE_LABEL,
} from '@/components/shared/summaryTimeRange';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';

interface WorkloadsSummaryProps {
  timeRange?: TimeRange;
  selectedNodeId?: string | null;
  visibleWorkloadIds?: string[];
  fallbackGuestCounts?: {
    total: number;
    running: number;
    stopped: number;
  };
  fallbackSnapshots?: WorkloadSummarySnapshot[];
  hoveredWorkloadId?: string | null;
  focusedWorkloadId?: string | null;
  onTimeRangeChange?: (range: TimeRange) => void;
}

export interface WorkloadSummarySnapshot {
  id: string;
  name: string;
  cpu: number;
  memory: number;
  disk: number;
  network: number;
}

interface WorkloadSeries {
  id: string;
  name: string;
  color: string;
  cpu: MetricPoint[];
  memory: MetricPoint[];
  disk: MetricPoint[];
  network: MetricPoint[];
}

const normalizeWorkloadId = (id: string): string => {
  const trimmed = id.trim();
  if (!trimmed) return '';
  if (trimmed.includes(':')) return trimmed;

  const parts = trimmed.split('-');
  const vmid = parts[parts.length - 1];

  // Standalone Proxmox guests use "node-vmid"; canonicalize to "node:node:vmid".
  if (parts.length === 2 && /^\d+$/.test(vmid)) {
    const node = parts[0];
    if (node) {
      return `${node}:${node}:${vmid}`;
    }
  }

  if (parts.length < 3) return trimmed;
  const node = parts[parts.length - 2];
  const instance = parts.slice(0, -2).join('-');

  if (!instance || !node || !/^\d+$/.test(vmid)) return trimmed;
  return `${instance}:${node}:${vmid}`;
};

const WORKLOAD_COLORS = [
  '#3b82f6',
  '#8b5cf6',
  '#10b981',
  '#f97316',
  '#ec4899',
  '#06b6d4',
  '#f59e0b',
  '#ef4444',
  '#14b8a6',
  '#84cc16',
  '#e11d48',
  '#6366f1',
];

const WORKLOADS_SUMMARY_CACHE_PREFIX = 'pulse.workloadsSummaryCharts.';
const DEFAULT_ORG_SCOPE = 'default';
const WORKLOADS_SUMMARY_CACHE_VERSION = 2;
const WORKLOADS_SUMMARY_CACHE_MAX_AGE_MS = 5 * 60_000;
const WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES = 360;
const WORKLOADS_SUMMARY_CACHE_MAX_CHARS = 900_000;
const WORKLOAD_CHART_DEFAULT_POINT_LIMIT = 180;
const WORKLOADS_IDLE_THRESHOLD_MS = 2 * 60_000;
const WORKLOADS_DEEP_IDLE_THRESHOLD_MS = 10 * 60_000;
let lastWorkloadsSummaryCharts: WorkloadChartsResponse | null = null;
let lastWorkloadsSummaryScopeKey: string | null = null;

type CachedChartData = Pick<ChartData, 'cpu' | 'memory' | 'disk' | 'netin' | 'netout'>;

interface CachedWorkloadsSummary {
  version: number;
  range: TimeRange;
  nodeScope: string;
  cachedAt: number;
  data: Record<string, CachedChartData>;
  dockerData: Record<string, CachedChartData>;
}

const normalizeOrgScope = (orgID?: string | null): string => {
  const normalized = (orgID || '').trim();
  return normalized || DEFAULT_ORG_SCOPE;
};

const cacheKeyFor = (range: TimeRange, nodeScope: string, orgScope: string) =>
  `${WORKLOADS_SUMMARY_CACHE_PREFIX}${encodeURIComponent(orgScope)}::${range}::${encodeURIComponent(nodeScope || '__all__')}`;

const trimPoints = <T,>(points: T[] | undefined, max: number): T[] => {
  if (!points || points.length === 0) return [];
  if (points.length <= max) return points;
  if (max <= 1) return points.slice(points.length - 1);

  const start = Math.max(0, points.length - max * 2);
  const sliced = points.slice(start);
  if (sliced.length <= max) return sliced;

  const step = Math.ceil(sliced.length / max);
  const result: T[] = [];
  for (let i = 0; i < sliced.length; i += step) {
    result.push(sliced[i]);
  }
  if (result[result.length - 1] !== sliced[sliced.length - 1]) {
    result.push(sliced[sliced.length - 1]);
  }
  return result.length > max ? result.slice(result.length - max) : result;
};

const toCachedChartData = (data: ChartData): CachedChartData => ({
  cpu: trimPoints(data.cpu, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  memory: trimPoints(data.memory, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  disk: trimPoints(data.disk, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  netin: trimPoints(data.netin, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  netout: trimPoints(data.netout, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
});

const persistWorkloadsSummaryCache = (
  range: TimeRange,
  nodeScope: string,
  orgScope: string,
  response: WorkloadChartsResponse,
): void => {
  if (typeof window === 'undefined') return;
  try {
    const data: Record<string, CachedChartData> = {};
    const dockerData: Record<string, CachedChartData> = {};
    for (const [id, chartData] of Object.entries(response.data || {})) {
      data[id] = toCachedChartData(chartData);
    }
    for (const [id, chartData] of Object.entries(response.dockerData || {})) {
      dockerData[id] = toCachedChartData(chartData);
    }

    const payload: CachedWorkloadsSummary = {
      version: WORKLOADS_SUMMARY_CACHE_VERSION,
      range,
      nodeScope,
      cachedAt: Date.now(),
      data,
      dockerData,
    };
    const serialized = JSON.stringify(payload);
    if (serialized.length > WORKLOADS_SUMMARY_CACHE_MAX_CHARS) {
      window.localStorage.removeItem(cacheKeyFor(range, nodeScope, orgScope));
      return;
    }
    window.localStorage.setItem(cacheKeyFor(range, nodeScope, orgScope), serialized);
  } catch {
    // Ignore cache write failures.
  }
};

const readWorkloadsSummaryCache = (
  range: TimeRange,
  nodeScope: string,
  orgScope: string,
): WorkloadChartsResponse | null => {
  if (typeof window === 'undefined') return null;
  try {
    const raw = window.localStorage.getItem(cacheKeyFor(range, nodeScope, orgScope));
    if (!raw) return null;

    const parsed = JSON.parse(raw) as CachedWorkloadsSummary;
    if (
      parsed?.version !== WORKLOADS_SUMMARY_CACHE_VERSION ||
      parsed.range !== range ||
      parsed.nodeScope !== nodeScope ||
      typeof parsed.cachedAt !== 'number'
    ) {
      return null;
    }
    if (Date.now() - parsed.cachedAt > WORKLOADS_SUMMARY_CACHE_MAX_AGE_MS) {
      window.localStorage.removeItem(cacheKeyFor(range, nodeScope, orgScope));
      return null;
    }

    return {
      data: parsed.data || {},
      dockerData: parsed.dockerData || {},
      guestTypes: {},
      timestamp: parsed.cachedAt,
      stats: {
        oldestDataTimestamp: 0,
      },
    };
  } catch {
    return null;
  }
};

const refreshIntervalForRange = (range: TimeRange): number => {
  switch (range) {
    case '5m':
    case '15m':
    case '1h':
      return 10_000;
    case '4h':
    case '12h':
    case '24h':
      return 30_000;
    case '7d':
    case '30d':
      return 120_000;
    default:
      return 30_000;
  }
};

const adaptiveRefreshIntervalForRange = (
  range: TimeRange,
  lastInteractionAt: number,
): number => {
  const base = refreshIntervalForRange(range);
  if (typeof document !== 'undefined' && document.visibilityState === 'hidden') {
    return Math.max(base * 6, 120_000);
  }

  const idleMs = Date.now() - lastInteractionAt;
  if (idleMs >= WORKLOADS_DEEP_IDLE_THRESHOLD_MS) {
    return Math.max(base * 4, 60_000);
  }
  if (idleMs >= WORKLOADS_IDLE_THRESHOLD_MS) {
    return Math.max(base * 2, 30_000);
  }
  return base;
};

const clampPercent = (value: number): number => {
  if (!Number.isFinite(value)) return 0;
  if (value < 0) return 0;
  if (value > 100) return 100;
  return value;
};

const clampNonNegative = (value: number): number => {
  if (!Number.isFinite(value)) return 0;
  if (value < 0) return 0;
  return value;
};

const formatRate = (bytesPerSec: number): string => {
  if (bytesPerSec >= 1e9) return `${(bytesPerSec / 1e9).toFixed(1)} GB/s`;
  if (bytesPerSec >= 1e6) return `${(bytesPerSec / 1e6).toFixed(1)} MB/s`;
  if (bytesPerSec >= 1e3) return `${(bytesPerSec / 1e3).toFixed(0)} KB/s`;
  return `${Math.round(bytesPerSec)} B/s`;
};

const workloadPointLimitForCount = (count: number): number => {
  if (count >= 600) return 48;
  if (count >= 400) return 64;
  if (count >= 250) return 80;
  if (count >= 120) return 120;
  if (count >= 80) return 150;
  return WORKLOAD_CHART_DEFAULT_POINT_LIMIT;
};

const hashString = (value: string): number => {
  let hash = 0;
  for (let i = 0; i < value.length; i++) {
    hash = (hash * 31 + value.charCodeAt(i)) >>> 0;
  }
  return hash;
};

const colorForWorkload = (id: string): string => {
  if (!id) return WORKLOAD_COLORS[0];
  return WORKLOAD_COLORS[hashString(id) % WORKLOAD_COLORS.length];
};

const sanitizeMetricPoints = (
  points: MetricPoint[] | undefined,
  clamp: (value: number) => number,
): MetricPoint[] => {
  if (!points || points.length === 0) return [];
  return points
    .filter((point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value))
    .map((point) => ({ timestamp: point.timestamp, value: clamp(point.value) }))
    .sort((a, b) => a.timestamp - b.timestamp);
};

const ensureRenderableSeries = (points: MetricPoint[]): MetricPoint[] => {
  // Need at least 2 real data points to render a meaningful line.
  // Single-point series (e.g., stopped resources with a fallback value)
  // create artifacts at the chart edge when padded.
  if (points.length < 2) return [];
  return points;
};

const mergeNetworkPoints = (
  netIn: MetricPoint[] | undefined,
  netOut: MetricPoint[] | undefined,
): MetricPoint[] => {
  const totals = new Map<number, number>();
  const append = (points: MetricPoint[] | undefined) => {
    if (!points) return;
    for (const point of points) {
      if (!Number.isFinite(point.timestamp)) continue;
      totals.set(point.timestamp, (totals.get(point.timestamp) || 0) + clampNonNegative(point.value));
    }
  };
  append(netIn);
  append(netOut);
  return Array.from(totals.entries())
    .sort((a, b) => a[0] - b[0])
    .map(([timestamp, value]) => ({ timestamp, value }));
};

const buildLiveSeries = (
  id: string,
  chartData: ChartData,
  fallbackName?: string,
): WorkloadSeries => ({
  id,
  name: fallbackName || id,
  color: colorForWorkload(id),
  cpu: ensureRenderableSeries(sanitizeMetricPoints(chartData.cpu, clampPercent)),
  memory: ensureRenderableSeries(sanitizeMetricPoints(chartData.memory, clampPercent)),
  disk: ensureRenderableSeries(sanitizeMetricPoints(chartData.disk, clampPercent)),
  network: ensureRenderableSeries(
    sanitizeMetricPoints(mergeNetworkPoints(chartData.netin, chartData.netout), clampNonNegative),
  ),
});

const buildWorkloadSeriesFromCharts = (
  response: WorkloadChartsResponse,
  namesById: Map<string, string>,
): Map<string, WorkloadSeries> => {
  const seriesById = new Map<string, WorkloadSeries>();
  for (const [id, chartData] of Object.entries(response.data || {})) {
    if (!id || !chartData) continue;
    const normalizedId = normalizeWorkloadId(id);
    const fallbackName = namesById.get(normalizedId) || namesById.get(id);
    seriesById.set(normalizedId, buildLiveSeries(normalizedId, chartData, fallbackName));
  }

  for (const [id, chartData] of Object.entries(response.dockerData || {})) {
    if (!id || !chartData) continue;
    const normalizedId = normalizeWorkloadId(id);
    const fallbackName = namesById.get(normalizedId) || namesById.get(id);
    seriesById.set(normalizedId, buildLiveSeries(normalizedId, chartData, fallbackName));
  }

  return seriesById;
};

export const WorkloadsSummary: Component<WorkloadsSummaryProps> = (props) => {
  const [charts, setCharts] = createSignal<WorkloadChartsResponse | null>(null);
  const [loadedScopeKey, setLoadedScopeKey] = createSignal<string | null>(null);
  const [fetchFailed, setFetchFailed] = createSignal(false);
  const [orgScope, setOrgScope] = createSignal(normalizeOrgScope(getOrgID()));
  const selectedRange = createMemo<TimeRange>(() => props.timeRange || '1h');
  const selectedNodeScope = createMemo(() => props.selectedNodeId?.trim() || '');
  const activeScopeKey = createMemo(() => `${orgScope()}::${selectedRange()}::${selectedNodeScope()}`);
  const hasCurrentRangeData = createMemo(() => loadedScopeKey() === activeScopeKey());

  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let activeFetchController: AbortController | null = null;
  let pollingToken = 0;
  let lastInteractionAt = Date.now();

  const clearRefreshTimer = () => {
    if (!refreshTimer) return;
    clearTimeout(refreshTimer);
    refreshTimer = undefined;
  };

  const scheduleNextFetch = (token: number) => {
    clearRefreshTimer();
    if (token !== pollingToken) return;
    const delay = adaptiveRefreshIntervalForRange(selectedRange(), lastInteractionAt);
    refreshTimer = setTimeout(() => {
      if (token !== pollingToken) return;
      void fetchCharts().finally(() => scheduleNextFetch(token));
    }, delay);
  };

  const fetchCharts = async (options?: { prioritize?: boolean }) => {
    const prioritize = options?.prioritize === true;
    if (activeFetchController && !prioritize) return;
    if (activeFetchController && prioritize) {
      activeFetchController.abort();
    }

    const range = selectedRange();
    const nodeId = selectedNodeScope() || undefined;
    const currentOrgScope = orgScope();
    const scopeKey = activeScopeKey();
    const controller = new AbortController();
    activeFetchController = controller;

    try {
      const response = await ChartsAPI.getWorkloadCharts(range, controller.signal, {
        nodeId,
        maxPoints: workloadPointLimit(),
      });
      if (activeFetchController !== controller) return;
      persistWorkloadsSummaryCache(range, selectedNodeScope(), currentOrgScope, response);
      setCharts(response);
      setLoadedScopeKey(scopeKey);
      lastWorkloadsSummaryCharts = response;
      lastWorkloadsSummaryScopeKey = scopeKey;
      setFetchFailed(false);
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        return;
      }
      if (activeFetchController === controller) {
        setLoadedScopeKey(scopeKey);
        setFetchFailed(true);
      }
    } finally {
      if (activeFetchController === controller) {
        activeFetchController = null;
      }
    }
  };

  createEffect(() => {
    const scopeKey = activeScopeKey();
    const range = selectedRange();
    const nodeScope = selectedNodeScope();
    const currentOrgScope = orgScope();
    const token = ++pollingToken;

    clearRefreshTimer();
    const cached = readWorkloadsSummaryCache(range, nodeScope, currentOrgScope);
    if (cached) {
      setCharts(cached);
      setLoadedScopeKey(scopeKey);
      lastWorkloadsSummaryCharts = cached;
      lastWorkloadsSummaryScopeKey = scopeKey;
    } else if (lastWorkloadsSummaryCharts && lastWorkloadsSummaryScopeKey === scopeKey) {
      setCharts(lastWorkloadsSummaryCharts);
      setLoadedScopeKey(lastWorkloadsSummaryScopeKey);
    } else {
      setCharts(null);
      setLoadedScopeKey(null);
    }
    setFetchFailed(false);
    void fetchCharts({ prioritize: true }).finally(() => scheduleNextFetch(token));
  });

  const unsubscribeOrgSwitch = eventBus.on('org_switched', (nextOrgID) => {
    setOrgScope(normalizeOrgScope(nextOrgID));
  });

  createEffect(() => {
    if (typeof window === 'undefined' || typeof document === 'undefined') return;

    const markInteraction = () => {
      lastInteractionAt = Date.now();
    };
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        lastInteractionAt = Date.now();
        void fetchCharts({ prioritize: true });
      }
      scheduleNextFetch(pollingToken);
    };

    const activityEvents: Array<keyof WindowEventMap> = ['pointerdown', 'keydown', 'touchstart', 'wheel'];
    for (const eventName of activityEvents) {
      window.addEventListener(eventName, markInteraction, { passive: true });
    }
    document.addEventListener('visibilitychange', handleVisibilityChange);

    onCleanup(() => {
      for (const eventName of activityEvents) {
        window.removeEventListener(eventName, markInteraction);
      }
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    });
  });

  onCleanup(() => {
    clearRefreshTimer();
    if (activeFetchController) activeFetchController.abort();
    unsubscribeOrgSwitch();
  });

  const guestCounts = createMemo(() => {
    if (props.fallbackGuestCounts) return props.fallbackGuestCounts;
    return { total: 0, running: 0, stopped: 0 };
  });

  const selectedNodeWorkloadIds = createMemo<Set<string> | null>(() => {
    if (!selectedNodeScope()) return null;
    const snapshots = props.fallbackSnapshots || [];
    return new Set(snapshots.map((snapshot) => snapshot.id).filter(Boolean));
  });

  const visibleWorkloadIdSet = createMemo<Set<string> | null>(() => {
    const ids = props.visibleWorkloadIds;
    if (!ids) return null;
    return new Set(ids.filter(Boolean));
  });

  const estimatedWorkloadCount = createMemo<number>(() => {
    const visible = visibleWorkloadIdSet();
    if (visible && visible.size > 0) return visible.size;
    if (props.fallbackSnapshots && props.fallbackSnapshots.length > 0) {
      return props.fallbackSnapshots.length;
    }
    const chartResponse = charts();
    if (!chartResponse) return 0;
    return Object.keys(chartResponse.data || {}).length + Object.keys(chartResponse.dockerData || {}).length;
  });

  const workloadPointLimit = createMemo<number>(() =>
    workloadPointLimitForCount(estimatedWorkloadCount()),
  );

  const workloadSeries = createMemo<WorkloadSeries[]>(() => {
    const snapshots = props.fallbackSnapshots || [];
    const namesById = new Map<string, string>();

    for (const snapshot of snapshots) {
      if (!snapshot.id) continue;
      const normalizedId = normalizeWorkloadId(snapshot.id);
      namesById.set(snapshot.id, snapshot.name || snapshot.id);
      namesById.set(normalizedId, snapshot.name || snapshot.id);
    }

    const liveSeriesById = charts()
      ? buildWorkloadSeriesFromCharts(charts()!, namesById)
      : new Map<string, WorkloadSeries>();
    const nodeFilterIdsRaw = selectedNodeWorkloadIds();
    const visibleIdsRaw = visibleWorkloadIdSet();
    const nodeFilterIds =
      nodeFilterIdsRaw
        ? new Set(Array.from(nodeFilterIdsRaw.values()).map((id) => normalizeWorkloadId(id)))
        : null;
    const visibleIds =
      visibleIdsRaw
        ? new Set(Array.from(visibleIdsRaw.values()).map((id) => normalizeWorkloadId(id)))
        : null;
    let candidateIds = new Set<string>();

    if (nodeFilterIds && nodeFilterIds.size > 0) {
      for (const id of nodeFilterIds) candidateIds.add(id);
    } else {
      for (const id of liveSeriesById.keys()) candidateIds.add(id);
    }

    if (visibleIds) {
      const filteredIds = new Set<string>();
      for (const id of candidateIds) {
        if (visibleIds.has(id)) {
          filteredIds.add(id);
        }
      }
      candidateIds = filteredIds;
    }

    const merged: WorkloadSeries[] = [];
    for (const id of candidateIds) {
      const live = liveSeriesById.get(id);
      if (!live) continue;

      merged.push({
        id,
        name: live.name || namesById.get(id) || id,
        color: live.color || colorForWorkload(id),
        cpu: live.cpu,
        memory: live.memory,
        disk: live.disk,
        network: live.network,
      });
    }

    return merged.sort((a, b) => a.name.localeCompare(b.name));
  });

  const displaySeries = createMemo(() => {
    const focused = props.focusedWorkloadId;
    const all = workloadSeries();
    if (!focused) return all;
    const match = all.find((s) => s.id === focused);
    return match ? [match] : all;
  });

  const focusedWorkloadName = createMemo(() => {
    const focused = props.focusedWorkloadId;
    if (!focused) return null;
    const match = workloadSeries().find((s) => s.id === focused);
    return match?.name || null;
  });

  const cpuSeries = createMemo<InteractiveSparklineSeries[]>(() =>
    displaySeries().map((series) => ({
      id: series.id,
      data: series.cpu,
      color: series.color,
      name: series.name,
    })),
  );
  const memorySeries = createMemo<InteractiveSparklineSeries[]>(() =>
    displaySeries().map((series) => ({
      id: series.id,
      data: series.memory,
      color: series.color,
      name: series.name,
    })),
  );
  const diskSeries = createMemo<InteractiveSparklineSeries[]>(() =>
    displaySeries().map((series) => ({
      id: series.id,
      data: series.disk,
      color: series.color,
      name: series.name,
    })),
  );
  const networkSeries = createMemo<InteractiveSparklineSeries[]>(() =>
    displaySeries().map((series) => ({
      id: series.id,
      data: series.network,
      color: series.color,
      name: series.name,
    })),
  );

  const hasCpuData = createMemo(() => cpuSeries().some((series) => series.data.length >= 2));
  const hasMemoryData = createMemo(() => memorySeries().some((series) => series.data.length >= 2));
  const hasDiskData = createMemo(() => diskSeries().some((series) => series.data.length >= 2));
  const hasNetworkData = createMemo(() => networkSeries().some((series) => series.data.length >= 2));

  const fallbackTrendMessage = () => {
    if (guestCounts().total === 0) return 'No workloads';
    if (!hasCurrentRangeData()) return '';
    if (fetchFailed()) return 'Trend data unavailable';
    return 'No history yet';
  };

  return (
    <div
      data-testid="workloads-summary"
      class="overflow-hidden rounded-md border border-slate-200 bg-white p-2 shadow-sm dark:border-slate-700 dark:bg-slate-800 sm:p-3"
    >
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 px-1 pb-2 text-[11px] text-slate-500 dark:border-slate-700 dark:text-slate-400">
        <div class="flex items-center gap-3">
          <span class="font-medium text-slate-700 dark:text-slate-200">
            {guestCounts().total} workloads
          </span>
          <Show when={guestCounts().running > 0}>
            <span class="text-emerald-600 dark:text-emerald-400">{guestCounts().running} running</span>
          </Show>
          <Show when={guestCounts().stopped > 0}>
            <span class="text-slate-400 dark:text-slate-500">{guestCounts().stopped} stopped</span>
          </Show>
        </div>
        <Show when={props.onTimeRangeChange}>
          <div class="inline-flex shrink-0 rounded border border-slate-300 bg-white p-0.5 text-xs dark:border-slate-700 dark:bg-slate-900">
            <For each={SUMMARY_TIME_RANGES}>
              {(range) => (
                <button
                  type="button"
                  onClick={() => props.onTimeRangeChange?.(range)}
                  class={`rounded px-2 py-1 ${selectedRange() === range
                    ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                    : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-700'
                    }`}
                >
                  {SUMMARY_TIME_RANGE_LABEL[range]}
                </button>
              )}
            </For>
          </div>
        </Show>
      </div>

      <div class="grid gap-2 sm:gap-3 grid-cols-2 lg:grid-cols-4">
        <Card padding="sm" class="h-full">
          <div class="flex flex-col h-full">
            <div class="flex items-center justify-between mb-1.5">
              <div class="flex items-center min-w-0">
                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide shrink-0">CPU</span>
                <Show when={focusedWorkloadName()}>
                  <span class="text-xs text-slate-400 dark:text-slate-500 ml-1.5 truncate">&mdash; {focusedWorkloadName()}</span>
                </Show>
              </div>
            </div>
            <Show
              when={hasCpuData()}
              fallback={
                <div class="flex h-[56px] items-center text-sm text-slate-400 dark:text-slate-500">
                  {fallbackTrendMessage()}
                </div>
              }
            >
              <div class="flex-1 min-h-0">
                <InteractiveSparkline
                  series={cpuSeries()}
                  rangeLabel={selectedRange()}
                  timeRange={selectedRange()}
                  yMode="percent"
                  sortTooltipByValue
                  maxTooltipRows={8}
                  highlightNearestSeriesOnHover
                  highlightSeriesId={props.hoveredWorkloadId}
                />
              </div>
            </Show>
          </div>
        </Card>

        <Card padding="sm" class="h-full">
          <div class="flex flex-col h-full">
            <div class="flex items-center justify-between mb-1.5">
              <div class="flex items-center min-w-0">
                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide shrink-0">Memory</span>
                <Show when={focusedWorkloadName()}>
                  <span class="text-xs text-slate-400 dark:text-slate-500 ml-1.5 truncate">&mdash; {focusedWorkloadName()}</span>
                </Show>
              </div>
            </div>
            <Show
              when={hasMemoryData()}
              fallback={
                <div class="flex h-[56px] items-center text-sm text-slate-400 dark:text-slate-500">
                  {fallbackTrendMessage()}
                </div>
              }
            >
              <div class="flex-1 min-h-0">
                <InteractiveSparkline
                  series={memorySeries()}
                  rangeLabel={selectedRange()}
                  timeRange={selectedRange()}
                  yMode="percent"
                  sortTooltipByValue
                  maxTooltipRows={8}
                  highlightNearestSeriesOnHover
                  highlightSeriesId={props.hoveredWorkloadId}
                />
              </div>
            </Show>
          </div>
        </Card>

        <Card padding="sm" class="h-full">
          <div class="flex flex-col h-full">
            <div class="flex items-center justify-between mb-1.5">
              <div class="flex items-center min-w-0">
                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide shrink-0">Storage</span>
                <Show when={focusedWorkloadName()}>
                  <span class="text-xs text-slate-400 dark:text-slate-500 ml-1.5 truncate">&mdash; {focusedWorkloadName()}</span>
                </Show>
              </div>
            </div>
            <Show
              when={hasDiskData()}
              fallback={
                <div class="flex h-[56px] items-center text-sm text-slate-400 dark:text-slate-500">
                  {fallbackTrendMessage()}
                </div>
              }
            >
              <div class="flex-1 min-h-0">
                <InteractiveSparkline
                  series={diskSeries()}
                  rangeLabel={selectedRange()}
                  timeRange={selectedRange()}
                  yMode="percent"
                  sortTooltipByValue
                  maxTooltipRows={8}
                  highlightNearestSeriesOnHover
                  highlightSeriesId={props.hoveredWorkloadId}
                />
              </div>
            </Show>
          </div>
        </Card>

        <Card padding="sm" class="h-full">
          <div class="flex flex-col h-full">
            <div class="flex items-center justify-between mb-1.5">
              <div class="flex items-center min-w-0">
                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide shrink-0">Network</span>
                <Show when={focusedWorkloadName()}>
                  <span class="text-xs text-slate-400 dark:text-slate-500 ml-1.5 truncate">&mdash; {focusedWorkloadName()}</span>
                </Show>
              </div>
            </div>
            <Show
              when={hasNetworkData()}
              fallback={
                <div class="flex h-[56px] items-center text-sm text-slate-400 dark:text-slate-500">
                  {fallbackTrendMessage()}
                </div>
              }
            >
              <div class="flex-1 min-h-0">
                <DensityMap
                  series={networkSeries()}
                  rangeLabel={selectedRange()}
                  timeRange={selectedRange()}
                  formatValue={formatRate}
                />
              </div>
            </Show>
          </div>
        </Card>
      </div>
    </div>
  );
};

export default WorkloadsSummary;
