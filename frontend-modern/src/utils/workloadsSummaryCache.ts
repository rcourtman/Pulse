import {
  ChartsAPI,
  type ChartData,
  type TimeRange,
  type WorkloadChartsResponse,
} from '@/api/charts';
import { getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';
import { eventBus } from '@/stores/events';

export const WORKLOADS_SUMMARY_CACHE_VERSION = 6;

const WORKLOADS_SUMMARY_CACHE_PREFIX = 'pulse.workloadsSummaryCharts.';
const WORKLOADS_SUMMARY_CACHE_MAX_AGE_MS = 5 * 60_000;
const WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES = 360;
const WORKLOADS_SUMMARY_CACHE_MAX_CHARS = 900_000;
const MAX_IN_MEMORY_WORKLOAD_ENTRIES = 20;

type CachedChartData = Pick<
  ChartData,
  'cpu' | 'memory' | 'disk' | 'diskread' | 'diskwrite' | 'netin' | 'netout'
>;

type CachedWorkloadsSummary = {
  version: number;
  range: TimeRange;
  nodeScope: string;
  cachedAt: number;
  data: Record<string, CachedChartData>;
  dockerData: Record<string, CachedChartData>;
};

type FetchWorkloadsSummaryAndCacheOptions = {
  caller?: string;
  maxPoints?: number | null;
  nodeId?: string | null;
  orgScope?: string | null;
  signal?: AbortSignal;
};

const inMemoryWorkloadCache = new Map<string, WorkloadChartsResponse>();
const inFlightWorkloadFetches = new Map<string, Promise<WorkloadChartsResponse>>();

const normalizeNodeScope = (nodeScope: string | null | undefined): string => nodeScope?.trim() ?? '';

const normalizeMaxPoints = (maxPoints: number | null | undefined): number | undefined => {
  if (typeof maxPoints !== 'number' || !Number.isFinite(maxPoints) || maxPoints <= 0) {
    return undefined;
  }
  return Math.round(maxPoints);
};

const resolveOrgScope = (orgScope?: string | null): string => normalizeOrgScope(orgScope ?? getOrgID());

const storageCacheKeyFor = (range: TimeRange, nodeScope: string, orgScope: string): string =>
  `${WORKLOADS_SUMMARY_CACHE_PREFIX}${encodeURIComponent(orgScope)}::${range}::${encodeURIComponent(nodeScope || '__all__')}`;

export function workloadsSummaryCacheScopeKey(
  range: TimeRange,
  nodeScope: string | null | undefined = '',
  orgScope?: string | null,
): string {
  return `${WORKLOADS_SUMMARY_CACHE_VERSION}::${resolveOrgScope(orgScope)}::${range}::${normalizeNodeScope(nodeScope)}`;
}

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
  diskread: trimPoints(data.diskread, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  diskwrite: trimPoints(data.diskwrite, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  netin: trimPoints(data.netin, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  netout: trimPoints(data.netout, WORKLOADS_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
});

const writeInMemoryWorkloadsSummaryCache = (
  range: TimeRange,
  nodeScope: string,
  orgScope: string,
  response: WorkloadChartsResponse,
): void => {
  const scopeKey = workloadsSummaryCacheScopeKey(range, nodeScope, orgScope);
  if (!inMemoryWorkloadCache.has(scopeKey) && inMemoryWorkloadCache.size >= MAX_IN_MEMORY_WORKLOAD_ENTRIES) {
    const oldest = inMemoryWorkloadCache.keys().next().value;
    if (oldest !== undefined) inMemoryWorkloadCache.delete(oldest);
  }
  inMemoryWorkloadCache.set(scopeKey, response);
};

export function readInMemoryWorkloadsSummaryCache(
  range: TimeRange,
  nodeScope: string | null | undefined = '',
  orgScope?: string | null,
): WorkloadChartsResponse | null {
  return inMemoryWorkloadCache.get(workloadsSummaryCacheScopeKey(range, nodeScope, orgScope)) ?? null;
}

export function persistWorkloadsSummaryCache(
  range: TimeRange,
  nodeScope: string | null | undefined,
  orgScope: string | null | undefined,
  response: WorkloadChartsResponse,
): void {
  const normalizedNodeScope = normalizeNodeScope(nodeScope);
  const normalizedOrgScope = resolveOrgScope(orgScope);

  writeInMemoryWorkloadsSummaryCache(range, normalizedNodeScope, normalizedOrgScope, response);

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
      nodeScope: normalizedNodeScope,
      cachedAt: Date.now(),
      data,
      dockerData,
    };
    const serialized = JSON.stringify(payload);
    const cacheKey = storageCacheKeyFor(range, normalizedNodeScope, normalizedOrgScope);
    if (serialized.length > WORKLOADS_SUMMARY_CACHE_MAX_CHARS) {
      window.localStorage.removeItem(cacheKey);
      return;
    }
    window.localStorage.setItem(cacheKey, serialized);
  } catch {
    // Ignore cache write failures.
  }
}

export function readWorkloadsSummaryCache(
  range: TimeRange,
  nodeScope: string | null | undefined = '',
  orgScope?: string | null,
): WorkloadChartsResponse | null {
  if (typeof window === 'undefined') return null;

  const normalizedNodeScope = normalizeNodeScope(nodeScope);
  const normalizedOrgScope = resolveOrgScope(orgScope);
  const cacheKey = storageCacheKeyFor(range, normalizedNodeScope, normalizedOrgScope);

  try {
    const raw = window.localStorage.getItem(cacheKey);
    if (!raw) return null;

    const parsed = JSON.parse(raw) as CachedWorkloadsSummary;
    if (
      parsed?.version !== WORKLOADS_SUMMARY_CACHE_VERSION ||
      parsed.range !== range ||
      parsed.nodeScope !== normalizedNodeScope ||
      typeof parsed.cachedAt !== 'number'
    ) {
      window.localStorage.removeItem(cacheKey);
      return null;
    }
    if (Date.now() - parsed.cachedAt > WORKLOADS_SUMMARY_CACHE_MAX_AGE_MS) {
      window.localStorage.removeItem(cacheKey);
      return null;
    }

    const response = {
      data: parsed.data || {},
      dockerData: parsed.dockerData || {},
      guestTypes: {},
      timestamp: parsed.cachedAt,
      stats: {
        oldestDataTimestamp: 0,
      },
    };
    writeInMemoryWorkloadsSummaryCache(range, normalizedNodeScope, normalizedOrgScope, response);
    return response;
  } catch {
    return null;
  }
}

export function hasFreshWorkloadsSummaryCache(
  range: TimeRange,
  nodeScope: string | null | undefined = '',
  orgScope?: string | null,
): boolean {
  return (
    readInMemoryWorkloadsSummaryCache(range, nodeScope, orgScope) !== null ||
    readWorkloadsSummaryCache(range, nodeScope, orgScope) !== null
  );
}

export function fetchWorkloadsSummaryAndCache(
  range: TimeRange,
  options: FetchWorkloadsSummaryAndCacheOptions = {},
): Promise<WorkloadChartsResponse> {
  const nodeScope = normalizeNodeScope(options.nodeId);
  const orgScope = resolveOrgScope(options.orgScope);
  const maxPoints = normalizeMaxPoints(options.maxPoints);
  const requestKey = `${workloadsSummaryCacheScopeKey(range, nodeScope, orgScope)}::${maxPoints ?? 'default'}`;

  if (!options.signal) {
    const inFlight = inFlightWorkloadFetches.get(requestKey);
    if (inFlight) return inFlight;
  }

  const promise = ChartsAPI.getWorkloadCharts(range, options.signal, {
    nodeId: nodeScope || undefined,
    maxPoints,
  }).then((response) => {
    persistWorkloadsSummaryCache(range, nodeScope, orgScope, response);
    return response;
  });

  if (!options.signal) {
    inFlightWorkloadFetches.set(requestKey, promise);
    void promise.finally(() => {
      inFlightWorkloadFetches.delete(requestKey);
    });
  }

  return promise;
}

export function __resetWorkloadsSummaryCacheForTests(): void {
  inMemoryWorkloadCache.clear();
  inFlightWorkloadFetches.clear();
}

const unsubscribeWorkloadOrgSwitch = eventBus.on('org_switched', () => {
  inMemoryWorkloadCache.clear();
  inFlightWorkloadFetches.clear();
});

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    unsubscribeWorkloadOrgSwitch();
  });
}
