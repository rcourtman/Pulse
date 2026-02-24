import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';
import {
  ChartsAPI as ChartService,
  type AggregatedMetricPoint,
  type HistoryTimeRange,
  type ResourceType as HistoryResourceType,
} from '@/api/charts';
import type { DashboardOverview } from '@/hooks/useDashboardOverview';
import {
  isStorage,
  type Resource,
  type ResourceType as UnifiedResourceType,
} from '@/types/resource';

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
  /** The unified resource ID used as the map key (may differ from the API resource ID). */
  originalId: string;
}

interface DashboardTrendRequest {
  cpu: HistoryTarget[];
  memory: HistoryTarget[];
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

function buildHistoryTargets(
  ids: string[],
  unifiedTypeById: Map<string, UnifiedResourceType>,
  metricsTargetById: Map<string, { resourceType: string; resourceId: string }>,
): HistoryTarget[] {
  const uniqueIds = dedupeValues(ids);
  const targets: HistoryTarget[] = [];

  for (const id of uniqueIds) {
    const mt = metricsTargetById.get(id);
    if (mt) {
      const historyType = asHistoryResourceType(mt.resourceType);
      if (historyType) {
        targets.push({ id: mt.resourceId, resourceType: historyType, originalId: id });
        continue;
      }
    }
    // Fallback: derive from unified type
    const unifiedType = unifiedTypeById.get(id);
    if (!unifiedType) continue;
    const mappedType = mapUnifiedTypeToHistoryType(unifiedType);
    if (!mappedType) continue;
    const historyType = asHistoryResourceType(mappedType);
    if (!historyType) continue;
    targets.push({ id, resourceType: historyType, originalId: id });
  }

  return targets;
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

async function fetchDashboardTrendSnapshot(
  request: DashboardTrendRequest,
): Promise<DashboardTrendSnapshot> {
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
            request.infrastructureRange,
            SPARKLINE_POINTS,
          );
          return [target.originalId, extractTrendData(points)] as const;
        } catch (error) {
          captureError(error);
          return [target.originalId, createEmptyTrendData()] as const;
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
            request.infrastructureRange,
            SPARKLINE_POINTS,
          );
          return [target.originalId, extractTrendData(points)] as const;
        } catch (error) {
          captureError(error);
          return [target.originalId, createEmptyTrendData()] as const;
        }
      }),
    ),
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

export function useDashboardTrends(
  overview: Accessor<DashboardOverview>,
  resources: Accessor<Resource[]>,
  infrastructureRange?: Accessor<HistoryTimeRange>,
): Accessor<DashboardTrends> {
  const [error, setError] = createSignal<string | null>(null);

  const trendRequest = createMemo<DashboardTrendRequest>(() => {
    const currentOverview = overview();
    const currentResources = resources();
    const unifiedTypeById = new Map(
      currentResources.map((resource) => [resource.id, resource.type] as const),
    );
    const metricsTargetById = new Map<string, { resourceType: string; resourceId: string }>();
    for (const resource of currentResources) {
      if (resource.metricsTarget) {
        metricsTargetById.set(resource.id, resource.metricsTarget);
      }
    }

    const cpuTargets = buildHistoryTargets(
      currentOverview.infrastructure.topCPU.map((item) => item.id),
      unifiedTypeById,
      metricsTargetById,
    );
    const memoryTargets = buildHistoryTargets(
      currentOverview.infrastructure.topMemory.map((item) => item.id),
      unifiedTypeById,
      metricsTargetById,
    );
    const storageTargets = dedupeValues(
      currentResources
        .filter((resource) => isStorage(resource))
        .map((resource) => {
          const mt = resource.metricsTarget;
          return mt ? mt.resourceId : resource.id;
        }),
    ).sort();

    return {
      cpu: cpuTargets,
      memory: memoryTargets,
      storage: storageTargets,
      infrastructureRange: infrastructureRange ? infrastructureRange() : INFRASTRUCTURE_RANGE,
    };
  });

  // Stabilize the request key: sort targets by ID so that metric-value
  // fluctuations don't shuffle the ordering and trigger unnecessary refetches.
  const requestKey = createMemo(() => {
    const req = trendRequest();
    const stableReq = {
      cpu: [...req.cpu].sort((a, b) => a.id.localeCompare(b.id)),
      memory: [...req.memory].sort((a, b) => a.id.localeCompare(b.id)),
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

    // Skip empty requests (no targets yet).
    if (request.cpu.length === 0 && request.memory.length === 0 && request.storage.length === 0)
      return;

    const requestId = ++latestRequestId;
    setTrendLoading(true);

    fetchDashboardTrendSnapshot(request)
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
