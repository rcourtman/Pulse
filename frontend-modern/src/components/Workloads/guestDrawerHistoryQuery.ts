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

// Snapshot replacement often creates a new target object with unchanged
// coordinates. Only a real target/range/access change owns a new read.
export const createGuestDrawerHistorySource = (
  target: Accessor<GuestDrawerHistoryTarget | null>,
  range: Accessor<HistoryTimeRange>,
  enabled: Accessor<boolean>,
): Accessor<GuestDrawerHistoryQueryKey | null> =>
  createMemo<GuestDrawerHistoryQueryKey | null>(
    () => {
      const current = target();
      if (!current || !enabled()) return null;
      return {
        resourceType: current.resourceType,
        resourceId: current.resourceId,
        range: range(),
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
          previous.range === next.range,
        ),
    },
  );

// The drawer's Overview trends and its History tab read one resource's
// metrics-store history through this query. The shared cache key lets a 24h
// History view reuse the Overview's read instead of starting another.
export const createGuestDrawerHistoryQuery = (
  source: Accessor<GuestDrawerHistoryQueryKey | null>,
) =>
  createNonSuspendingQuery<AllMetricsHistoryResponse, GuestDrawerHistoryQueryKey>({
    source,
    fetcher: async (key, signal) =>
      normalizeHistoryResponse(
        await ChartsAPI.getMetricsHistory({
          resourceType: key.resourceType,
          resourceId: key.resourceId,
          range: key.range,
          maxPoints: GUEST_DRAWER_HISTORY_MAX_POINTS,
          signal,
        }),
      ),
    initialValue: EMPTY_HISTORY_RESPONSE,
    cacheKey: (key) => `guest-drawer-history:${key.resourceType}:${key.resourceId}:${key.range}`,
    // Former-host or former-range observations are not evidence for this
    // target. Matching cached reads and same-source polling remain retained.
    retainPreviousValueOnSourceChange: false,
    pollMs: GUEST_DRAWER_HISTORY_POLL_MS,
  });
