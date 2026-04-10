import { ChartsAPI, type StorageSummaryChartsResponse, type TimeRange } from '@/api/charts';
import { eventBus } from '@/stores/events';
import { getOrgID } from '@/utils/apiClient';
import { normalizeOrgScope } from '@/utils/orgScope';

const STORAGE_SUMMARY_CACHE_VERSION = 1;

function cacheKeyFor(
  range: TimeRange,
  nodeId?: string,
  orgScope: string = normalizeOrgScope(getOrgID()),
): string {
  return `${STORAGE_SUMMARY_CACHE_VERSION}::${orgScope}::${range}::${nodeId || '__all__'}`;
}

const inMemoryCache = new Map<string, StorageSummaryChartsResponse>();
const inFlightFetches = new Map<string, Promise<StorageSummaryChartsResponse>>();

export function readStorageSummaryCache(
  range: TimeRange,
  nodeId?: string,
): StorageSummaryChartsResponse | null {
  return inMemoryCache.get(cacheKeyFor(range, nodeId)) ?? null;
}

export function fetchStorageSummaryAndCache(
  range: TimeRange,
  options?: { caller?: string; nodeId?: string },
): Promise<StorageSummaryChartsResponse> {
  const inFlightKey = cacheKeyFor(range, options?.nodeId);
  const existing = inFlightFetches.get(inFlightKey);
  if (existing) {
    return existing;
  }

  const request = ChartsAPI.getStorageSummaryCharts(range, undefined, {
    nodeId: options?.nodeId,
  })
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

export function __resetStorageSummaryCacheForTests(): void {
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
