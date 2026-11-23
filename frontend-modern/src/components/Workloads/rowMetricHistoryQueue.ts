import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';

import {
  ChartsAPI,
  type AllMetricsHistoryResponse,
  type ResourceType,
  type SingleMetricHistoryResponse,
} from '@/api/charts';

import {
  WORKLOAD_TABLE_HISTORY_MAX_POINTS,
  type WorkloadTableMetricHistoryRange,
} from './workloadMetricHistoryModel';

/** The metrics-history coordinates of one table row. */
export interface RowMetricHistoryTarget {
  /** Stable row identity, used to find the hovered row among the candidates. */
  rowId: string;
  resourceType: ResourceType;
  resourceId: string;
}

interface RowMetricHistoryQueryKey extends RowMetricHistoryTarget {
  range: WorkloadTableMetricHistoryRange;
}

export interface RowMetricHistoryQueueOptions<T> {
  range: Accessor<WorkloadTableMetricHistoryRange>;
  /** While false nothing is fetched and queued reads are cancelled. */
  onDemand: Accessor<boolean>;
  /** The row being previewed; its read and its neighbours' go first. */
  activeRow: Accessor<T | null>;
  /** Rows in display order, warmed from the top while no row is active. */
  candidates: Accessor<readonly T[]>;
  targetFor: (row: T) => RowMetricHistoryTarget | null;
}

export interface RowMetricHistoryQueue<T> {
  /** The cached history for a row, or null until its read lands. */
  getHistory: (row: T) => AllMetricsHistoryResponse | null;
}

const ROW_HISTORY_PREFETCH_CONCURRENCY = 4;
const ROW_HISTORY_INITIAL_PREFETCH_COUNT = 6;
const ROW_HISTORY_ADJACENT_PREFETCH_COUNT = 4;
const ROW_HISTORY_CACHE_MAX_ENTRIES = 18;

const normalizeRowHistoryResponse = (
  response: AllMetricsHistoryResponse | SingleMetricHistoryResponse,
): AllMetricsHistoryResponse =>
  'metrics' in response
    ? response
    : {
        resourceType: response.resourceType,
        resourceId: response.resourceId,
        range: response.range,
        start: response.start,
        end: response.end,
        metrics: { [response.metric]: response.points },
        source: response.source,
      };

const buildRowHistoryCacheKey = (key: RowMetricHistoryQueryKey): string =>
  `${key.resourceType}:${key.resourceId}:${key.range}:${WORKLOAD_TABLE_HISTORY_MAX_POINTS}`;

/**
 * On-demand per-row metric history for a table's hover lens: one bounded
 * queue of single-resource history reads with a small cache, shared by every
 * table that previews a row's own history. Reads start only while `onDemand`
 * holds, a range change drops the cache, and a failed read leaves the row on
 * its live bars.
 */
export function createRowMetricHistoryQueue<T>(
  options: RowMetricHistoryQueueOptions<T>,
): RowMetricHistoryQueue<T> {
  const [revision, setRevision] = createSignal(0);
  const cache = new Map<string, AllMetricsHistoryResponse>();
  const pending: RowMetricHistoryQueryKey[] = [];
  const inFlight = new Map<string, { controller: AbortController; generation: number }>();
  let generation = 0;
  let scheduledRange: WorkloadTableMetricHistoryRange | null = null;
  let warmingActive = false;
  let disposed = false;

  const keyFor = (row: T, range: WorkloadTableMetricHistoryRange) => {
    const target = options.targetFor(row);
    return target ? { ...target, range } : null;
  };
  const keysFor = (rows: readonly T[], range: WorkloadTableMetricHistoryRange) =>
    rows
      .map((row) => keyFor(row, range))
      .filter((key): key is RowMetricHistoryQueryKey => key !== null);

  const cancel = () => {
    generation += 1;
    pending.splice(0, pending.length);
    inFlight.forEach(({ controller }) => controller.abort());
    inFlight.clear();
  };

  const trimCache = () => {
    while (cache.size > ROW_HISTORY_CACHE_MAX_ENTRIES) {
      const oldestKey = cache.keys().next().value as string | undefined;
      if (!oldestKey) return;
      cache.delete(oldestKey);
    }
  };

  const pump = () => {
    while (!disposed && inFlight.size < ROW_HISTORY_PREFETCH_CONCURRENCY && pending.length > 0) {
      const key = pending.shift();
      if (!key) break;
      const cacheKey = buildRowHistoryCacheKey(key);
      if (cache.has(cacheKey) || inFlight.has(cacheKey)) continue;

      const controller = new AbortController();
      const readGeneration = generation;
      inFlight.set(cacheKey, { controller, generation: readGeneration });
      void ChartsAPI.getMetricsHistory({
        resourceType: key.resourceType,
        resourceId: key.resourceId,
        range: key.range,
        maxPoints: WORKLOAD_TABLE_HISTORY_MAX_POINTS,
        signal: controller.signal,
      })
        .then((response) => {
          if (disposed || readGeneration !== generation || controller.signal.aborted) return;
          cache.set(cacheKey, normalizeRowHistoryResponse(response));
          trimCache();
          setRevision((value) => value + 1);
        })
        .catch(() => {
          // Hover warming is opportunistic. The live bars remain available if
          // history is missing or the request is cancelled by a range change.
        })
        .finally(() => {
          const current = inFlight.get(cacheKey);
          if (current?.generation === readGeneration) {
            inFlight.delete(cacheKey);
          }
          pump();
        });
    }
  };

  const enqueue = (keys: RowMetricHistoryQueryKey[], prioritize: boolean) => {
    const uniqueKeys = keys.filter((key, index) => {
      const cacheKey = buildRowHistoryCacheKey(key);
      return (
        keys.findIndex((candidate) => buildRowHistoryCacheKey(candidate) === cacheKey) === index &&
        !cache.has(cacheKey) &&
        !inFlight.has(cacheKey)
      );
    });
    if (uniqueKeys.length === 0) return;

    const uniqueCacheKeys = new Set(uniqueKeys.map(buildRowHistoryCacheKey));
    for (let index = pending.length - 1; index >= 0; index -= 1) {
      if (uniqueCacheKeys.has(buildRowHistoryCacheKey(pending[index]))) {
        pending.splice(index, 1);
      }
    }
    if (prioritize) {
      pending.unshift(...uniqueKeys);
    } else {
      pending.push(...uniqueKeys);
    }
    pump();
  };

  createEffect(() => {
    const range = options.range();
    const onDemand = options.onDemand();
    const candidates = options.candidates();
    const activeRow = options.activeRow();

    if (scheduledRange !== range) {
      cancel();
      cache.clear();
      setRevision((value) => value + 1);
      scheduledRange = range;
    }

    if (!onDemand) {
      if (warmingActive) cancel();
      warmingActive = false;
      return;
    }
    warmingActive = true;

    if (!activeRow) {
      enqueue(keysFor(candidates.slice(0, ROW_HISTORY_INITIAL_PREFETCH_COUNT), range), false);
      return;
    }

    const activeId = options.targetFor(activeRow)?.rowId;
    const activeIndex =
      activeId === undefined
        ? -1
        : candidates.findIndex((row) => options.targetFor(row)?.rowId === activeId);
    const adjacentRows =
      activeIndex >= 0
        ? [
            activeRow,
            ...candidates.slice(
              activeIndex + 1,
              activeIndex + 1 + ROW_HISTORY_ADJACENT_PREFETCH_COUNT,
            ),
            ...(activeIndex > 0 ? [candidates[activeIndex - 1]] : []),
          ]
        : [activeRow];
    enqueue(keysFor(adjacentRows, range), true);
  });

  onCleanup(() => {
    disposed = true;
    cancel();
  });

  return {
    getHistory: (row) => {
      revision();
      const key = keyFor(row, options.range());
      return key ? (cache.get(buildRowHistoryCacheKey(key)) ?? null) : null;
    },
  };
}
