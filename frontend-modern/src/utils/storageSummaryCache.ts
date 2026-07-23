import { ChartsAPI, type StorageSummaryChartsResponse, type TimeRange } from '@/api/charts';
import { eventBus } from '@/stores/events';
import { getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';

const STORAGE_SUMMARY_CACHE_VERSION = 1;
const MAX_IN_MEMORY_STORAGE_SUMMARIES = 20;

function cacheKeyFor(
  range: TimeRange,
  nodeId?: string,
  orgScope: string = normalizeOrgScope(getOrgID()),
): string {
  return `${STORAGE_SUMMARY_CACHE_VERSION}::${orgScope}::${range}::${nodeId || '__all__'}`;
}

const inMemoryCache = new Map<string, StorageSummaryChartsResponse>();
const inFlightFetches = new Map<
  string,
  {
    controller: AbortController;
    promise: Promise<StorageSummaryChartsResponse>;
  }
>();
let storageSummaryCacheGeneration = 0;

const readInMemoryStorageSummary = (key: string): StorageSummaryChartsResponse | null => {
  const cached = inMemoryCache.get(key);
  if (!cached) {
    return null;
  }
  inMemoryCache.delete(key);
  inMemoryCache.set(key, cached);
  return cached;
};

const writeInMemoryStorageSummary = (key: string, response: StorageSummaryChartsResponse): void => {
  inMemoryCache.delete(key);
  inMemoryCache.set(key, response);
  while (inMemoryCache.size > MAX_IN_MEMORY_STORAGE_SUMMARIES) {
    const oldestKey = inMemoryCache.keys().next().value;
    if (oldestKey === undefined) {
      break;
    }
    inMemoryCache.delete(oldestKey);
  }
};

const clearStorageSummaryCache = () => {
  storageSummaryCacheGeneration += 1;
  inMemoryCache.clear();
  for (const entry of inFlightFetches.values()) {
    entry.controller.abort();
  }
  inFlightFetches.clear();
};

export function readStorageSummaryCache(
  range: TimeRange,
  nodeId?: string,
): StorageSummaryChartsResponse | null {
  return readInMemoryStorageSummary(cacheKeyFor(range, nodeId));
}

export function fetchStorageSummaryAndCache(
  range: TimeRange,
  options?: { caller?: string; nodeId?: string },
): Promise<StorageSummaryChartsResponse> {
  const inFlightKey = cacheKeyFor(range, options?.nodeId);
  const existing = inFlightFetches.get(inFlightKey);
  if (existing) {
    return existing.promise;
  }

  const controller = new AbortController();
  const cacheGeneration = storageSummaryCacheGeneration;
  const request = ChartsAPI.getStorageSummaryCharts(range, controller.signal, {
    nodeId: options?.nodeId,
  })
    .then((response) => {
      if (cacheGeneration === storageSummaryCacheGeneration) {
        writeInMemoryStorageSummary(inFlightKey, response);
      }
      return response;
    })
    .finally(() => {
      const inFlight = inFlightFetches.get(inFlightKey);
      if (inFlight?.controller === controller) {
        inFlightFetches.delete(inFlightKey);
      }
    });

  inFlightFetches.set(inFlightKey, { controller, promise: request });
  return request;
}

const unsubscribeStorageOrgSwitch = eventBus.on('org_switched', () => {
  clearStorageSummaryCache();
});

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    unsubscribeStorageOrgSwitch();
    clearStorageSummaryCache();
  });
}
