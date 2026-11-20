import { createMemo, createSignal, type Accessor } from 'solid-js';

import {
  ChartsAPI,
  type AllMetricsHistoryResponse,
  type HistoryTimeRange,
  type ResourceType,
  type SingleMetricHistoryResponse,
} from '@/api/charts';
import { createNonSuspendingQuery, STALLED_QUERY_READ_MS } from '@/hooks/createNonSuspendingQuery';

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
  pollAfterRead: boolean;
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

const readHistory = async (key: GuestDrawerHistoryQueryKey, signal: AbortSignal) =>
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
  );

const readHistoryBeforeDeadline = async (key: GuestDrawerHistoryQueryKey, owner: AbortSignal) => {
  if (owner.aborted) throw owner.reason;
  const controller = new AbortController();
  const abort = () => controller.abort(owner.reason);
  owner.addEventListener('abort', abort, { once: true });
  const deadline = setTimeout(
    () => controller.abort(new Error('The request timed out.')),
    STALLED_QUERY_READ_MS,
  );
  // Settle the chart state even if a transport fails to honour the abort.
  const interrupted = new Promise<never>((_, reject) => {
    controller.signal.addEventListener('abort', () => reject(controller.signal.reason), {
      once: true,
    });
  });
  try {
    return await Promise.race([interrupted, readHistory(key, controller.signal)]);
  } finally {
    clearTimeout(deadline);
    owner.removeEventListener('abort', abort);
  }
};

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
        pollAfterRead: current.pollAfterRead === true,
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
          previous.liveReadingsAvailable === next.liveReadingsAvailable &&
          previous.pollAfterRead === next.pollAfterRead,
        ),
    },
  );

// The drawer's Overview trends and its History tab read one resource's
// metrics-store history through this query. The shared cache key lets a 24h
// History view reuse the Overview's read instead of starting another.
export const createGuestDrawerHistoryQuery = (
  source: Accessor<GuestDrawerHistoryQueryKey | null>,
) => {
  const [pollBlocker, setPollBlocker] = createSignal<AbortSignal | null>(null);
  const query = createNonSuspendingQuery<AllMetricsHistoryResponse, GuestDrawerHistoryQueryKey>({
    source,
    fetcher: async (key, signal) => {
      if (!key.pollAfterRead) return readHistory(key, signal);
      setPollBlocker(signal);
      try {
        return await readHistoryBeforeDeadline(key, signal);
      } finally {
        // A range/host change can start a new read before this one unwinds.
        setPollBlocker((active) => (active === signal ? null : active));
      }
    },
    initialValue: EMPTY_HISTORY_RESPONSE,
    cacheKey: (key) => `guest-drawer-history:${key.resourceType}:${key.resourceId}:${key.range}`,
    // Former-host or former-range observations are not evidence for this
    // target. Matching cached reads and same-source polling remain retained.
    retainPreviousValueOnSourceChange: false,
    // For opt-in targets the bounded read owns settlement, not the poll.
    // Resume the 30s cadence when it finishes, without a catch-up request.
    pollMs: () => (pollBlocker() === null ? GUEST_DRAWER_HISTORY_POLL_MS : undefined),
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
