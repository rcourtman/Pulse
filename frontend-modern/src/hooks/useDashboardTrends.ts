import { createEffect, createMemo, createResource, createSignal, type Accessor } from 'solid-js';
import { useWebSocket } from '@/App';
import {
  ChartsAPI as ChartService,
  type AggregatedMetricPoint,
  type HistoryTimeRange,
  type ResourceType as HistoryResourceType,
} from '@/api/charts';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import { isStorage, type ResourceType as UnifiedResourceType } from '@/types/resource';

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
  };
  storage: {
    capacity: TrendData | null;
  };
  loading: boolean;
  error: string | null;
}

interface HistoryTarget {
  id: string;
  resourceType: HistoryResourceType;
}

interface DashboardTrendRequest {
  cpu: HistoryTarget[];
  memory: HistoryTarget[];
  storage: string[];
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
    (point) =>
      Number.isFinite(point.timestamp) &&
      Number.isFinite(point.value),
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

function asHistoryResourceType(type: string): HistoryResourceType | null {
  const historyTypes: HistoryResourceType[] = [
    'node',
    'guest',
    'vm',
    'container',
    'storage',
    'docker',
    'dockerHost',
    'k8s',
    'host',
    'disk',
  ];
  return historyTypes.includes(type as HistoryResourceType) ? (type as HistoryResourceType) : null;
}

function buildHistoryTargets(ids: string[], unifiedTypeById: Map<string, UnifiedResourceType>): HistoryTarget[] {
  const uniqueIds = dedupeValues(ids);
  const targets: HistoryTarget[] = [];

  for (const id of uniqueIds) {
    const unifiedType = unifiedTypeById.get(id);
    if (!unifiedType) continue;
    const mappedType = mapUnifiedTypeToHistoryType(unifiedType);
    if (!mappedType) continue;
    const historyType = asHistoryResourceType(mappedType);
    if (!historyType) continue;
    targets.push({ id, resourceType: historyType });
  }

  return targets;
}

function aggregateStoragePoints(allSeries: TrendPoint[][]): TrendPoint[] {
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

export function mapUnifiedTypeToHistoryType(type: string): string | null {
  switch (type) {
    case 'node':
      return 'node';
    case 'host':
      return 'host';
    case 'docker-host':
      return 'dockerHost';
    case 'k8s-node':
    case 'k8s-cluster':
      return 'k8s';
    case 'truenas':
      return 'node';
    case 'vm':
    case 'container':
      return 'guest';
    case 'docker-container':
      return 'docker';
    case 'pod':
      return 'k8s';
    default:
      return null;
  }
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

async function fetchDashboardTrendSnapshot(request: DashboardTrendRequest): Promise<DashboardTrendSnapshot> {
  let firstError: string | null = null;
  const captureError = (error: unknown) => {
    if (firstError === null) {
      firstError = toErrorMessage(error);
    }
  };

  const [cpuEntries, memoryEntries, storageSeries] = await Promise.all([
    Promise.all(
      request.cpu.map(async (target) => {
        try {
          const points = await fetchMetricPoints(
            target.resourceType,
            target.id,
            'cpu',
            INFRASTRUCTURE_RANGE,
            SPARKLINE_POINTS,
          );
          return [target.id, extractTrendData(points)] as const;
        } catch (error) {
          captureError(error);
          return [target.id, createEmptyTrendData()] as const;
        }
      }),
    ),
    Promise.all(
      request.memory.map(async (target) => {
        try {
          const points = await fetchMetricPoints(
            target.resourceType,
            target.id,
            'memory',
            INFRASTRUCTURE_RANGE,
            SPARKLINE_POINTS,
          );
          return [target.id, extractTrendData(points)] as const;
        } catch (error) {
          captureError(error);
          return [target.id, createEmptyTrendData()] as const;
        }
      }),
    ),
    Promise.all(
      request.storage.map(async (resourceId) => {
        try {
          return await fetchMetricPoints('storage', resourceId, 'disk', STORAGE_RANGE, SPARKLINE_POINTS);
        } catch (error) {
          captureError(error);
          return [];
        }
      }),
    ),
  ]);

  const storageCapacity =
    request.storage.length > 0
      ? extractTrendData(aggregateStoragePoints(storageSeries))
      : null;

  return {
    infrastructure: {
      cpu: new Map<string, TrendData>(cpuEntries),
      memory: new Map<string, TrendData>(memoryEntries),
    },
    storage: {
      capacity: storageCapacity,
    },
    error: firstError,
  };
}

export function useDashboardTrends(overview: Accessor<DashboardOverview>): Accessor<DashboardTrends> {
  const { state } = useWebSocket();
  const [error, setError] = createSignal<string | null>(null);

  const trendRequest = createMemo<DashboardTrendRequest>(() => {
    const currentOverview = overview();
    const resources = Array.isArray(state.resources) ? state.resources : [];
    const unifiedTypeById = new Map(resources.map((resource) => [resource.id, resource.type] as const));

    const cpuTargets = buildHistoryTargets(
      currentOverview.infrastructure.topCPU.map((item) => item.id),
      unifiedTypeById,
    );
    const memoryTargets = buildHistoryTargets(
      currentOverview.infrastructure.topMemory.map((item) => item.id),
      unifiedTypeById,
    );
    const storageTargets = dedupeValues(
      resources.filter((resource) => isStorage(resource)).map((resource) => resource.id),
    ).sort();

    return {
      cpu: cpuTargets,
      memory: memoryTargets,
      storage: storageTargets,
    };
  });

  const requestKey = createMemo(() => JSON.stringify(trendRequest()));
  const initialSnapshot = createEmptyTrendSnapshot();

  const [trendResource] = createResource(
    requestKey,
    async () => fetchDashboardTrendSnapshot(trendRequest()),
    { initialValue: initialSnapshot },
  );

  createEffect(() => {
    const snapshot = trendResource();
    const resourceError = trendResource.error;
    if (snapshot?.error) {
      setError(snapshot.error);
      return;
    }
    if (resourceError) {
      setError(toErrorMessage(resourceError));
      return;
    }
    setError(null);
  });

  return createMemo<DashboardTrends>(() => {
    const snapshot = trendResource() ?? initialSnapshot;
    return {
      infrastructure: snapshot.infrastructure,
      storage: snapshot.storage,
      loading: trendResource.loading,
      error: error(),
    };
  });
}
