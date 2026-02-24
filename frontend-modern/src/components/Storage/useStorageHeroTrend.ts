import { type Accessor, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';

import { ChartsAPI as ChartService, type HistoryTimeRange } from '@/api/charts';
import {
  aggregateStoragePoints,
  extractTrendData,
  type TrendData,
} from '@/hooks/useDashboardTrends';
import type { StorageRecord } from '@/features/storageBackups/models';

type TrendPoint = { timestamp: number; value: number };

const STORAGE_RANGE: HistoryTimeRange = '7d';
const MAX_POINTS = 30;
const REFRESH_INTERVAL_MS = 5 * 60 * 1000;

function normalizeTrendPoints(points: Array<{ timestamp: number; value: number }>): TrendPoint[] {
  const normalized = points.filter(
    (point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value),
  );
  normalized.sort((a, b) => a.timestamp - b.timestamp);
  return normalized;
}

export function useStorageHeroTrend(records: Accessor<StorageRecord[]>): {
  trend: Accessor<TrendData | null>;
  loading: Accessor<boolean>;
} {
  // Stabilize the request key so record reordering doesn't refetch.
  const resourceIdsKey = createMemo(() => {
    const ids = records()
      .map((r) => r.refs?.resourceId || r.id)
      .filter((id): id is string => typeof id === 'string' && id.trim().length > 0);

    const uniqueSortedIds = Array.from(new Set(ids)).sort();
    return JSON.stringify(uniqueSortedIds);
  });

  const [trend, setTrend] = createSignal<TrendData | null>(null);
  const [loading, setLoading] = createSignal(false);

  // Track the latest request to discard stale responses.
  let latestRequestId = 0;

  const fetchTrend = async () => {
    const key = resourceIdsKey();
    const resourceIds = JSON.parse(key) as string[];

    if (resourceIds.length === 0) {
      setTrend(null);
      setLoading(false);
      return;
    }

    const requestId = ++latestRequestId;
    setLoading(true);

    try {
      const results = await Promise.all(
        resourceIds.map((resourceId) =>
          ChartService.getMetricsHistory({
            resourceType: 'storage',
            resourceId,
            metric: 'disk',
            range: STORAGE_RANGE,
            maxPoints: MAX_POINTS,
          }),
        ),
      );

      if (requestId !== latestRequestId) return; // stale

      const allSeries: TrendPoint[][] = results.map((result) => {
        if (!('points' in result) || !Array.isArray(result.points)) return [];
        return normalizeTrendPoints(result.points as Array<{ timestamp: number; value: number }>);
      });

      const aggregatedPoints = aggregateStoragePoints(allSeries);
      const computed = extractTrendData(aggregatedPoints);
      setTrend(computed);
    } catch {
      if (requestId !== latestRequestId) return;
      setTrend(null);
    } finally {
      if (requestId === latestRequestId) {
        setLoading(false);
      }
    }
  };

  createEffect(() => {
    resourceIdsKey(); // track dependency — re-run effect when key changes

    fetchTrend();
    const timer = setInterval(fetchTrend, REFRESH_INTERVAL_MS);
    onCleanup(() => clearInterval(timer));
  });

  return { trend, loading };
}
