import { ChartsAPI, type StorageSummaryTrendResponse, type TimeRange } from '@/api/charts';
import { eventBus } from '@/stores/events';
import { getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';

const STORAGE_SUMMARY_TREND_CACHE_VERSION = 1;

function cacheKeyFor(
  range: TimeRange,
  orgScope: string = normalizeOrgScope(getOrgID()),
): string {
  return `${STORAGE_SUMMARY_TREND_CACHE_VERSION}::${orgScope}::${range}`;
}

const inMemoryCache = new Map<string, StorageSummaryTrendResponse>();
const inFlightFetches = new Map<string, Promise<StorageSummaryTrendResponse>>();

export function readStorageSummaryTrendCache(range: TimeRange): StorageSummaryTrendResponse | null {
  return inMemoryCache.get(cacheKeyFor(range)) ?? null;
}

export function fetchStorageSummaryTrendAndCache(
  range: TimeRange,
  _options?: { caller?: string },
): Promise<StorageSummaryTrendResponse> {
  const inFlightKey = cacheKeyFor(range);
  const existing = inFlightFetches.get(inFlightKey);
  if (existing) {
    return existing;
  }

  const request = ChartsAPI.getStorageSummaryTrend(range)
    .then((response) => {
      inMemoryCache.set(inFlightKey, response);
      return response;
    })
    .finally(() => {
      inFlightFetches.delete(inFlightKey);
    });

  inFlightFetches.set(inFlightKey, request);
  return request;
}

export function __resetStorageSummaryTrendCacheForTests(): void {
  inMemoryCache.clear();
  inFlightFetches.clear();
}

const unsubscribeStorageOrgSwitch = eventBus.on('org_switched', () => {
  inMemoryCache.clear();
  inFlightFetches.clear();
});

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    unsubscribeStorageOrgSwitch();
  });
}
