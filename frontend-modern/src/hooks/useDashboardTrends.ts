import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';
import {
  ChartsAPI as ChartService,
  type AggregatedMetricPoint,
  type HistoryTimeRange,
  type ResourceType as HistoryResourceType,
  type TimeRange,
} from '@/api/charts';
import {
  buildInfrastructureEmptyHistoryLabel,
  buildInfrastructureSummarySeries,
} from '@/components/Infrastructure/infrastructureSummaryModel';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import { hasAgentFacet } from '@/utils/agentResources';
import { fetchInfrastructureSummaryAndCache } from '@/utils/infrastructureSummaryCache';
import { isInfrastructure, isStorage, type Resource } from '@/types/resource';

export type TrendPoint = {
  timestamp: number;
  value: number;
};

export type TrendData = {
  points: TrendPoint[];
  delta: number | null;
  currentValue: number | null;
};

export interface DashboardTrends {
  infrastructure: {
    cpu: Map<string, TrendData>;
    memory: Map<string, TrendData>;
    emptyMessage: string | null;
  };
  storage: {
    capacity: TrendData | null;
  };
  loading: boolean;
  error: string | null;
}

interface DashboardTrendRequest {
  infrastructure: string[];
  storage: string[];
  infrastructureRange: HistoryTimeRange;
}

interface DashboardTrendSnapshot {
  infrastructure: DashboardTrends['infrastructure'];
  storage: DashboardTrends['storage'];
  error: string | null;
}

const SPARKLINE_POINTS = 30;
const INFRASTRUCTURE_RANGE: HistoryTimeRange = '1h';
const STORAGE_RANGE: HistoryTimeRange = '24h';

function createEmptyTrendData(): TrendData {
  return {
    points: [],
    delta: null,
    currentValue: null,
  };
}

function createEmptyTrendSnapshot(): DashboardTrendSnapshot {
  return {
    infrastructure: {
      cpu: new Map<string, TrendData>(),
      memory: new Map<string, TrendData>(),
      emptyMessage: null,
    },
    storage: {
      capacity: null,
    },
    error: null,
  };
}

function toErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return 'Failed to load dashboard trends';
}

function normalizeTrendPoints(points: Array<{ timestamp: number; value: number }>): TrendPoint[] {
  const normalized = points.filter(
    (point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value),
  );
  normalized.sort((a, b) => a.timestamp - b.timestamp);
  return normalized;
}

function averagePoints(points: TrendPoint[]): number {
  if (points.length === 0) return 0;
  return points.reduce((sum, point) => sum + point.value, 0) / points.length;
}

function dedupeValues(values: string[]): string[] {
  return Array.from(new Set(values));
}

function toInfrastructureSummaryRange(range: HistoryTimeRange): TimeRange {
  switch (range) {
    case '1h':
      return '1h';
    case '6h':
    case '12h':
      return '12h';
    case '24h':
      return '24h';
    case '7d':
      return '7d';
    case '30d':
    case '90d':
      return '30d';
    default:
      return '1h';
  }
}

async function fetchInfrastructureTrendSnapshot(
  resources: Resource[],
  range: HistoryTimeRange,
): Promise<DashboardTrends['infrastructure']> {
  const infrastructureResources = resources.filter((resource) => isInfrastructure(resource));
  if (infrastructureResources.length === 0) {
    return {
      cpu: new Map<string, TrendData>(),
      memory: new Map<string, TrendData>(),
      emptyMessage: null,
    };
  }

  const agentFacetResources = resources.filter(
    (resource) =>
      (resource.type === 'agent' ||
        resource.type === 'pbs' ||
        resource.type === 'pmg' ||
        resource.type === 'truenas') &&
      hasAgentFacet(resource),
  );

  const { map, oldestDataTimestamp } = await fetchInfrastructureSummaryAndCache(
    toInfrastructureSummaryRange(range),
    { caller: 'useDashboardTrends' },
  );
  const summarySeries = buildInfrastructureSummarySeries(
    infrastructureResources,
    map,
    agentFacetResources,
  );

  const cpu = new Map<string, TrendData>();
  const memory = new Map<string, TrendData>();
  let hasRenderableHistory = false;

  for (const series of summarySeries) {
    const cpuTrend = extractTrendData(series.cpu);
    const memoryTrend = extractTrendData(series.memory);
    if (cpuTrend.points.length >= 2 || memoryTrend.points.length >= 2) {
      hasRenderableHistory = true;
    }
    cpu.set(series.id, cpuTrend);
    memory.set(series.id, memoryTrend);
  }

  return {
    cpu,
    memory,
    emptyMessage: hasRenderableHistory
      ? null
      : buildInfrastructureEmptyHistoryLabel(oldestDataTimestamp === null),
  };
}

export function aggregateStoragePoints(allSeries: TrendPoint[][]): TrendPoint[] {
  const buckets = new Map<number, { sum: number; count: number }>();

  for (const series of allSeries) {
    for (const point of series) {
      const bucket = buckets.get(point.timestamp) ?? { sum: 0, count: 0 };
      bucket.sum += point.value;
      bucket.count += 1;
      buckets.set(point.timestamp, bucket);
    }
  }

  return Array.from(buckets.entries())
    .sort((a, b) => a[0] - b[0])
    .map(([timestamp, bucket]) => ({
      timestamp,
      value: bucket.sum / bucket.count,
    }));
}

async function fetchMetricPoints(
  resourceType: HistoryResourceType,
  resourceId: string,
  metric: 'cpu' | 'memory' | 'disk',
  range: HistoryTimeRange,
  maxPoints: number,
): Promise<TrendPoint[]> {
  const result = await ChartService.getMetricsHistory({
    resourceType,
    resourceId,
    metric,
    range,
    maxPoints,
  });

  if (!('points' in result) || !Array.isArray(result.points)) {
    return [];
  }

  return normalizeTrendPoints(result.points as AggregatedMetricPoint[]);
}

export function computeTrendDelta(points: TrendPoint[]): number | null {
  if (points.length < 2) return null;

  const quarterSize = Math.max(1, Math.floor(points.length * 0.25));
  const firstQuarter = points.slice(0, quarterSize);
  const lastQuarter = points.slice(points.length - quarterSize);

  const firstAverage = averagePoints(firstQuarter);
  const lastAverage = averagePoints(lastQuarter);

  if (firstAverage === 0) {
    return lastAverage === 0 ? 0 : null;
  }

  const delta = ((lastAverage - firstAverage) / firstAverage) * 100;
  return Number.isFinite(delta) ? delta : null;
}

export function extractTrendData(points: Array<{ timestamp: number; value: number }>): TrendData {
  const normalized = normalizeTrendPoints(points);
  if (normalized.length < 2) {
    return createEmptyTrendData();
  }

  return {
    points: normalized,
    delta: computeTrendDelta(normalized),
    currentValue: normalized[normalized.length - 1]?.value ?? null,
  };
}

async function fetchDashboardTrendSnapshot(
  request: DashboardTrendRequest,
  resources: Resource[],
): Promise<DashboardTrendSnapshot> {
  let firstError: string | null = null;
  const captureError = (error: unknown) => {
    if (firstError === null) {
      firstError = toErrorMessage(error);
    }
  };

  const [infrastructure, storageSeries] = await Promise.all([
    (async () => {
      try {
        return await fetchInfrastructureTrendSnapshot(resources, request.infrastructureRange);
      } catch (error) {
        captureError(error);
        return createEmptyTrendSnapshot().infrastructure;
      }
    })(),
    Promise.all(
      request.storage.map(async (resourceId) => {
        try {
          return await fetchMetricPoints(
            'storage',
            resourceId,
            'disk',
            STORAGE_RANGE,
            SPARKLINE_POINTS,
          );
        } catch (error) {
          captureError(error);
          return [];
        }
      }),
    ),
  ]);

  const storageCapacity =
    request.storage.length > 0 ? extractTrendData(aggregateStoragePoints(storageSeries)) : null;

  return {
    infrastructure,
    storage: {
      capacity: storageCapacity,
    },
    error: firstError,
  };
}

export function useDashboardTrends(
  overview: Accessor<DashboardOverview>,
  resources: Accessor<Resource[]>,
  infrastructureRange?: Accessor<HistoryTimeRange>,
): Accessor<DashboardTrends> {
  const [error, setError] = createSignal<string | null>(null);

  const trendRequest = createMemo<DashboardTrendRequest>(() => {
    const currentOverview = overview();
    const currentResources = resources();
    const infrastructureTargets = dedupeValues([
      ...currentOverview.infrastructure.topCPU.map((item) => item.id),
      ...currentOverview.infrastructure.topMemory.map((item) => item.id),
    ]).sort();
    const storageTargets = dedupeValues(
      currentResources
        .filter((resource) => isStorage(resource))
        .map((resource) => {
          const mt = resource.metricsTarget;
          return mt ? mt.resourceId : resource.id;
        }),
    ).sort();

    return {
      infrastructure: infrastructureTargets,
      storage: storageTargets,
      infrastructureRange: infrastructureRange ? infrastructureRange() : INFRASTRUCTURE_RANGE,
    };
  });

  // Stabilize the request key: sort targets by ID so that metric-value
  // fluctuations don't shuffle the ordering and trigger unnecessary refetches.
  const requestKey = createMemo(() => {
    const req = trendRequest();
    const stableReq = {
      infrastructure: [...req.infrastructure].sort(),
      storage: [...req.storage].sort(),
      infrastructureRange: req.infrastructureRange,
    };
    return JSON.stringify(stableReq);
  });

  const initialSnapshot = createEmptyTrendSnapshot();
  const [snapshot, setSnapshot] = createSignal<DashboardTrendSnapshot>(initialSnapshot);
  const [trendLoading, setTrendLoading] = createSignal(false);

  // Track the latest request to discard stale responses.
  let latestRequestId = 0;

  // Use manual async fetching instead of createResource so that refetches
  // never trigger the app-level <Suspense> boundary (which would unmount
  // the entire page and reset scroll position).
  createEffect(() => {
    requestKey(); // track dependency — re-run effect when key changes
    const request = trendRequest();
    const currentResources = resources();

    // Skip empty requests (no targets yet).
    if (request.infrastructure.length === 0 && request.storage.length === 0) return;

    const requestId = ++latestRequestId;
    setTrendLoading(true);

    fetchDashboardTrendSnapshot(request, currentResources)
      .then((result) => {
        if (requestId !== latestRequestId) return; // stale
        setSnapshot(result);
        if (result.error) {
          setError(result.error);
        } else {
          setError(null);
        }
      })
      .catch((err) => {
        if (requestId !== latestRequestId) return;
        setError(toErrorMessage(err));
      })
      .finally(() => {
        if (requestId === latestRequestId) {
          setTrendLoading(false);
        }
      });
  });

  return createMemo<DashboardTrends>(() => {
    const current = snapshot();
    return {
      infrastructure: current.infrastructure,
      storage: current.storage,
      loading: trendLoading(),
      error: error(),
    };
  });
}
