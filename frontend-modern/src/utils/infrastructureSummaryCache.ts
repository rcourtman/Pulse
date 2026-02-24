import {
  ChartsAPI,
  type ChartData,
  type ChartsResponse,
  type InfrastructureChartsResponse,
  type TimeRange,
} from '@/api/charts';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';

export const INFRA_SUMMARY_CACHE_PREFIX = 'pulse.infrastructureSummaryCharts.';
export const INFRA_SUMMARY_CACHE_MAX_AGE_MS = 5 * 60_000;
const INFRA_SUMMARY_CACHE_VERSION = 1;
const INFRA_SUMMARY_CACHE_MAX_CHARS = 900_000;
const INFRA_SUMMARY_CACHE_MAX_POINTS_PER_SERIES = 360;
const DEFAULT_ORG_SCOPE = 'default';

const INFRA_SUMMARY_PERF_LOG_PREFIX = '[InfraSummaryPerf]';

// Opt-in perf logging to keep default test/dev output clean.
const infraSummaryPerfEnabled =
  import.meta.env.DEV && import.meta.env.VITE_INFRA_SUMMARY_PERF === '1';

function infraSummaryPerfNow(): number {
  if (typeof performance !== 'undefined' && typeof performance.now === 'function') {
    return performance.now();
  }
  return Date.now();
}

function infraSummaryPerfLog(message: string, data?: Record<string, unknown>): void {
  if (!infraSummaryPerfEnabled) return;
  if (data) {
    console.debug(`${INFRA_SUMMARY_PERF_LOG_PREFIX} ${message}`, data);
    return;
  }
  console.debug(`${INFRA_SUMMARY_PERF_LOG_PREFIX} ${message}`);
}

function countChartMapPoints(map: Map<string, ChartData>): number {
  let total = 0;
  for (const data of map.values()) {
    total += data.cpu?.length ?? 0;
    total += data.memory?.length ?? 0;
    total += data.disk?.length ?? 0;
    total += data.netin?.length ?? 0;
    total += data.netout?.length ?? 0;
  }
  return total;
}

function pickRicherSeries<T>(incoming?: T[], existing?: T[]): T[] | undefined {
  const incomingLen = Array.isArray(incoming) ? incoming.length : 0;
  const existingLen = Array.isArray(existing) ? existing.length : 0;
  if (incomingLen === 0 && existingLen === 0) return undefined;
  if (incomingLen >= existingLen) return incoming;
  return existing;
}

function mergeChartData(existing: ChartData | undefined, incoming: ChartData): ChartData {
  if (!existing) return incoming;

  return {
    cpu: pickRicherSeries(incoming.cpu, existing.cpu),
    memory: pickRicherSeries(incoming.memory, existing.memory),
    disk: pickRicherSeries(incoming.disk, existing.disk),
    diskread: pickRicherSeries(incoming.diskread, existing.diskread),
    diskwrite: pickRicherSeries(incoming.diskwrite, existing.diskwrite),
    netin: pickRicherSeries(incoming.netin, existing.netin),
    netout: pickRicherSeries(incoming.netout, existing.netout),
  };
}

type CachedChartData = Pick<ChartData, 'cpu' | 'memory' | 'disk' | 'netin' | 'netout'>;

interface CachedInfrastructureSummary {
  version: number;
  range: TimeRange;
  cachedAt: number;
  oldestDataTimestamp: number | null;
  charts: Record<string, CachedChartData>;
}

export interface InfrastructureSummaryCacheHit {
  map: Map<string, ChartData>;
  oldestDataTimestamp: number | null;
  cachedAt: number;
}

export interface InfrastructureSummaryFetchOptions {
  caller?: string;
}

export interface InfrastructureSummaryFetchResult {
  map: Map<string, ChartData>;
  oldestDataTimestamp: number | null;
}

const toCachedChartData = (data: ChartData): CachedChartData => ({
  cpu: trimPoints(data.cpu ?? [], INFRA_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  memory: trimPoints(data.memory ?? [], INFRA_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  disk: trimPoints(data.disk ?? [], INFRA_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  netin: trimPoints(data.netin ?? [], INFRA_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
  netout: trimPoints(data.netout ?? [], INFRA_SUMMARY_CACHE_MAX_POINTS_PER_SERIES),
});

const normalizeOrgScope = (orgID: string | null | undefined): string => {
  const normalized = orgID?.trim();
  if (!normalized) {
    return DEFAULT_ORG_SCOPE;
  }
  return normalized;
};

const inFlightKeyFor = (range: TimeRange, orgScope: string) =>
  `${encodeURIComponent(orgScope)}::${range}`;

const cacheKeyForRange = (range: TimeRange, orgScope: string = normalizeOrgScope(getOrgID())) =>
  `${INFRA_SUMMARY_CACHE_PREFIX}${encodeURIComponent(orgScope)}::${range}`;

function trimPoints<T>(points: T[], max: number): T[] {
  if (points.length <= max) return points;
  if (max <= 1) return points.slice(points.length - 1);

  // Simple stride sampling. We bias toward keeping the newest points.
  const start = Math.max(0, points.length - max * 2);
  const sliced = points.slice(start);
  if (sliced.length <= max) return sliced;

  const step = Math.ceil(sliced.length / max);
  const result: T[] = [];
  for (let i = 0; i < sliced.length; i += step) {
    result.push(sliced[i]);
  }
  // Ensure the last point is present.
  if (result[result.length - 1] !== sliced[sliced.length - 1]) {
    result.push(sliced[sliced.length - 1]);
  }
  return result.length > max ? result.slice(result.length - max) : result;
}

export function extractInfrastructureSummaryChartMap(
  response: ChartsResponse,
): Map<string, ChartData> {
  const map = new Map<string, ChartData>();

  if (response.nodeData) {
    for (const [id, data] of Object.entries(response.nodeData)) {
      map.set(id, mergeChartData(map.get(id), data));
    }
  }
  if (response.hostData) {
    for (const [id, data] of Object.entries(response.hostData)) {
      map.set(id, mergeChartData(map.get(id), data));
    }
  }
  if (response.dockerHostData) {
    for (const [id, data] of Object.entries(response.dockerHostData)) {
      map.set(id, mergeChartData(map.get(id), data));
    }
  }

  return map;
}

export function extractInfrastructureSummaryChartMapFromInfrastructureResponse(
  response: InfrastructureChartsResponse,
): Map<string, ChartData> {
  // Reuse the same extraction shape by mapping fields to the ChartsResponse subset.
  return extractInfrastructureSummaryChartMap({
    data: {},
    nodeData: response.nodeData ?? {},
    storageData: {},
    dockerHostData: response.dockerHostData,
    hostData: response.hostData,
    dockerData: {},
    guestTypes: {},
    timestamp: response.timestamp,
    stats: response.stats,
  });
}

export function persistInfrastructureSummaryCache(
  range: TimeRange,
  map: Map<string, ChartData>,
  oldestDataTimestamp: number | null,
  orgScope?: string,
): void {
  if (typeof window === 'undefined') return;

  try {
    const scopedOrg = normalizeOrgScope(orgScope ?? getOrgID());
    const charts: Record<string, CachedChartData> = {};
    for (const [key, value] of map.entries()) {
      charts[key] = toCachedChartData(value);
    }
    const payload: CachedInfrastructureSummary = {
      version: INFRA_SUMMARY_CACHE_VERSION,
      range,
      cachedAt: Date.now(),
      oldestDataTimestamp,
      charts,
    };

    const serialized = JSON.stringify(payload);
    if (serialized.length > INFRA_SUMMARY_CACHE_MAX_CHARS) {
      // Avoid blowing past localStorage limits (and evicting unrelated app state).
      window.localStorage.removeItem(cacheKeyForRange(range, scopedOrg));
      return;
    }

    window.localStorage.setItem(cacheKeyForRange(range, scopedOrg), serialized);
  } catch {
    // Ignore storage write failures.
  }
}

export function readInfrastructureSummaryCache(
  range: TimeRange,
  maxAgeMs: number = INFRA_SUMMARY_CACHE_MAX_AGE_MS,
  orgScope?: string,
): InfrastructureSummaryCacheHit | null {
  if (typeof window === 'undefined') return null;

  try {
    const scopedOrg = normalizeOrgScope(orgScope ?? getOrgID());
    const cacheKey = cacheKeyForRange(range, scopedOrg);
    const raw = window.localStorage.getItem(cacheKey);
    if (!raw) return null;

    const parsed = JSON.parse(raw) as CachedInfrastructureSummary;
    if (
      parsed?.version !== INFRA_SUMMARY_CACHE_VERSION ||
      parsed.range !== range ||
      typeof parsed.cachedAt !== 'number'
    ) {
      return null;
    }

    if (Date.now() - parsed.cachedAt > maxAgeMs) {
      window.localStorage.removeItem(cacheKey);
      return null;
    }

    const chartEntries =
      parsed.charts && typeof parsed.charts === 'object' ? Object.entries(parsed.charts) : [];
    const map = new Map<string, ChartData>(
      chartEntries.map(([key, value]) => [
        key,
        {
          cpu: Array.isArray(value?.cpu) ? value.cpu : [],
          memory: Array.isArray(value?.memory) ? value.memory : [],
          disk: Array.isArray(value?.disk) ? value.disk : [],
          netin: Array.isArray(value?.netin) ? value.netin : [],
          netout: Array.isArray(value?.netout) ? value.netout : [],
        },
      ]),
    );

    return {
      map,
      cachedAt: parsed.cachedAt,
      oldestDataTimestamp:
        typeof parsed.oldestDataTimestamp === 'number' &&
        Number.isFinite(parsed.oldestDataTimestamp)
          ? parsed.oldestDataTimestamp
          : null,
    };
  } catch {
    return null;
  }
}

export function hasFreshInfrastructureSummaryCache(
  range: TimeRange,
  maxAgeMs: number = INFRA_SUMMARY_CACHE_MAX_AGE_MS,
): boolean {
  return readInfrastructureSummaryCache(range, maxAgeMs) !== null;
}

const inFlightFetches = new Map<string, Promise<InfrastructureSummaryFetchResult>>();
let infraSummaryFetchSeq = 0;

/**
 * Fetch infrastructure summary charts for a range and persist the resulting
 * cache payload. Concurrent requests for the same range are deduplicated.
 */
export function fetchInfrastructureSummaryAndCache(
  range: TimeRange,
  options?: InfrastructureSummaryFetchOptions,
): Promise<InfrastructureSummaryFetchResult> {
  const caller = options?.caller || 'unknown';
  const orgScope = normalizeOrgScope(getOrgID());
  const inFlightKey = inFlightKeyFor(range, orgScope);

  const existing = inFlightFetches.get(inFlightKey);
  if (existing) {
    infraSummaryPerfLog('fetch deduped', { caller, range, orgScope });
    return existing;
  }

  const requestId = ++infraSummaryFetchSeq;
  const startedAt = infraSummaryPerfNow();
  infraSummaryPerfLog('fetch start', { caller, range, orgScope, requestId });

  const request = ChartsAPI.getInfrastructureSummaryCharts(range)
    .then((response) => {
      const map = extractInfrastructureSummaryChartMapFromInfrastructureResponse(response);
      const oldestDataTimestamp =
        typeof response.stats?.oldestDataTimestamp === 'number' &&
        Number.isFinite(response.stats.oldestDataTimestamp)
          ? response.stats.oldestDataTimestamp
          : null;
      persistInfrastructureSummaryCache(range, map, oldestDataTimestamp, orgScope);

      infraSummaryPerfLog('fetch done', {
        caller,
        range,
        orgScope,
        requestId,
        ms: Math.round(infraSummaryPerfNow() - startedAt),
        series: map.size,
        points: countChartMapPoints(map),
        primarySourceHint: response.stats?.primarySourceHint,
        metricsStoreEnabled: response.stats?.metricsStoreEnabled,
        inMemoryThresholdSecs: response.stats?.inMemoryThresholdSecs,
        pointCounts: response.stats?.pointCounts,
      });

      return { map, oldestDataTimestamp };
    })
    .catch((error) => {
      infraSummaryPerfLog('fetch error', {
        caller,
        range,
        orgScope,
        requestId,
        ms: Math.round(infraSummaryPerfNow() - startedAt),
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    })
    .finally(() => {
      inFlightFetches.delete(inFlightKey);
    });

  inFlightFetches.set(inFlightKey, request);
  return request;
}

/**
 * Test-only helper to clear in-flight request state between unit tests.
 */
export function __resetInfrastructureSummaryFetchesForTests(): void {
  inFlightFetches.clear();
}

// Invalidate infrastructure summary cache on org switch.
const unsubscribeOrgSwitchCacheInvalidation = eventBus.on('org_switched', () => {
  if (typeof window === 'undefined') return;
  try {
    const keysToRemove: string[] = [];
    for (let i = 0; i < window.localStorage.length; i++) {
      const key = window.localStorage.key(i);
      if (key && key.startsWith(INFRA_SUMMARY_CACHE_PREFIX)) {
        keysToRemove.push(key);
      }
    }
    keysToRemove.forEach((key) => window.localStorage.removeItem(key));
  } catch {
    /* ignore storage errors */
  }
});

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    unsubscribeOrgSwitchCacheInvalidation();
  });
}
