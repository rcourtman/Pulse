import { createMemo, type Accessor } from 'solid-js';

import {
  ChartsAPI,
  type AllMetricsHistoryResponse,
  type HistoryTimeRange,
  type ResourceType,
  type SingleMetricHistoryResponse,
} from '@/api/charts';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';

import {
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  type GuestDrawerHistoryTarget,
} from './guestDrawerModel';

export const GUEST_DRAWER_HISTORY_MAX_POINTS = 240;
export const GUEST_DRAWER_HISTORY_POLL_MS = 30_000;

export interface GuestDrawerHistoryQueryKey {
  resourceType: ResourceType;
  resourceId: string;
  range: HistoryTimeRange;
  liveReadingsAvailable: boolean;
}

const EMPTY_HISTORY_RESPONSE: AllMetricsHistoryResponse = {
  resourceType: '',
  resourceId: '',
  range: GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  start: 0,
  end: 0,
  metrics: {},
  source: 'store',
};

const normalizeHistoryResponse = (
  result: SingleMetricHistoryResponse | AllMetricsHistoryResponse,
): AllMetricsHistoryResponse => {
  if ('metrics' in result) return result;
  return {
    resourceType: result.resourceType,
    resourceId: result.resourceId,
    range: result.range,
    start: result.start,
    end: result.end,
    metrics: { [result.metric]: result.points ?? [] },
    source: result.source,
  };
};

// A cached live reply describes the source before it stopped reporting. Keep
// real history, but withdraw that live snapshot as soon as its authority ends.
const filterUnavailableLiveReading = (
  response: AllMetricsHistoryResponse,
  available: boolean,
): AllMetricsHistoryResponse =>
  !available && response.source === 'live'
    ? { ...response, metrics: {}, source: 'store' }
    : response;

// Snapshot replacement often creates a new target object with unchanged
// coordinates. A delivery transition also owns a new read, so an open drawer
// does not wait for its next poll after the source stops or resumes reporting.
export const createGuestDrawerHistorySource = (
  target: Accessor<GuestDrawerHistoryTarget | null>,
  range: Accessor<HistoryTimeRange>,
  enabled: Accessor<boolean>,
  liveReadingsAvailable: Accessor<boolean> = () => true,
): Accessor<GuestDrawerHistoryQueryKey | null> =>
  createMemo<GuestDrawerHistoryQueryKey | null>(
    () => {
      const current = target();
      if (!current || !enabled()) return null;
      return {
        resourceType: current.resourceType,
        resourceId: current.resourceId,
        range: range(),
        liveReadingsAvailable: liveReadingsAvailable(),
      };
    },
    null,
    {
      equals: (previous, next) =>
        previous === next ||
        Boolean(
          previous &&
          next &&
          previous.resourceType === next.resourceType &&
          previous.resourceId === next.resourceId &&
          previous.range === next.range &&
          previous.liveReadingsAvailable === next.liveReadingsAvailable,
        ),
    },
  );

// The drawer's Overview trends and its History tab read one resource's
// metrics-store history through this query. The shared cache key lets a 24h
// History view reuse the Overview's read instead of starting another.
export const createGuestDrawerHistoryQuery = (
  source: Accessor<GuestDrawerHistoryQueryKey | null>,
) => {
  const query = createNonSuspendingQuery<AllMetricsHistoryResponse, GuestDrawerHistoryQueryKey>({
    source,
    fetcher: async (key, signal) =>
      filterUnavailableLiveReading(
        normalizeHistoryResponse(
          await ChartsAPI.getMetricsHistory({
            resourceType: key.resourceType,
            resourceId: key.resourceId,
            range: key.range,
            maxPoints: GUEST_DRAWER_HISTORY_MAX_POINTS,
            signal,
          }),
        ),
        key.liveReadingsAvailable,
      ),
    initialValue: EMPTY_HISTORY_RESPONSE,
    cacheKey: (key) => `guest-drawer-history:${key.resourceType}:${key.resourceId}:${key.range}`,
    // Former-host or former-range observations are not evidence for this
    // target. Matching cached reads and same-source polling remain retained.
    retainPreviousValueOnSourceChange: false,
    pollMs: GUEST_DRAWER_HISTORY_POLL_MS,
  });
  return {
    ...query,
    // Matching cached history stays visible during the fresh read. A live
    // cache entry loses authority synchronously, before that read can settle.
    value: createMemo(() =>
      filterUnavailableLiveReading(query.value(), source()?.liveReadingsAvailable !== false),
    ),
  };
};
