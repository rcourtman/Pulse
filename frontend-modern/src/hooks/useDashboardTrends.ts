import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';
import {
  type ChartData,
  type HistoryTimeRange,
  type InfrastructureSummaryMetric,
  type StorageSummaryTrendResponse,
  type TimeRange,
} from '@/api/charts';
import { buildInfrastructureEmptyHistoryLabel } from '@/components/Infrastructure/infrastructureSummaryModel';
import type { DashboardOverviewSummary } from '@/hooks/useDashboardOverview';
import { eventBus } from '@/stores/events';
import { getOrgID } from '@/utils/apiClient';
import {
  fetchInfrastructureSummaryAndCache,
  readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';
import { normalizeOrgScope } from '@/utils/orgScope';
import {
  fetchStorageSummaryTrendAndCache,
  readStorageSummaryTrendCache,
} from '@/utils/storageSummaryTrendCache';

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

const INFRASTRUCTURE_RANGE: HistoryTimeRange = '1h';
const STORAGE_RANGE: TimeRange = '24h';
const DASHBOARD_INFRASTRUCTURE_METRICS: InfrastructureSummaryMetric[] = ['cpu', 'memory'];

function createEmptyTrendData(): TrendData {
  return {
    points: [],
    delta: null,
    currentValue: null,
  };
}

function createEmptyInfrastructureTrends(): DashboardTrends['infrastructure'] {
  return {
    cpu: new Map<string, TrendData>(),
    memory: new Map<string, TrendData>(),
    emptyMessage: null,
  };
}

function createEmptyStorageTrends(): DashboardTrends['storage'] {
  return {
    capacity: null,
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

type DashboardTrendResourceRef = DashboardOverviewSummary['infrastructure']['topCPU'][number];

function buildMetricTrendMap(
  resources: DashboardTrendResourceRef[],
  map: Map<string, ChartData>,
  metric: 'cpu' | 'memory',
): Map<string, TrendData> {
  const result = new Map<string, TrendData>();

  for (const resource of resources.slice(0, 5)) {
    const historyKey = resource.metricsTarget?.resourceId || resource.id;
    const history = map.get(historyKey);
    if (!history) {
      continue;
    }

    const points = metric === 'cpu' ? history.cpu ?? [] : history.memory ?? [];
    result.set(resource.id, extractTrendData(points));
  }

  return result;
}

function buildInfrastructureTrendSnapshot(
  overview: DashboardOverviewSummary,
  map: Map<string, ChartData>,
  oldestDataTimestamp: number | null,
): DashboardTrends['infrastructure'] {
  if (overview.infrastructure.total === 0) {
    return createEmptyInfrastructureTrends();
  }

  const cpu = buildMetricTrendMap(overview.infrastructure.topCPU, map, 'cpu');
  const memory = buildMetricTrendMap(overview.infrastructure.topMemory, map, 'memory');
  const hasRenderableHistory = [...cpu.values(), ...memory.values()].some(
    (trend) => trend.points.length >= 2,
  );

  return {
    cpu,
    memory,
    emptyMessage: hasRenderableHistory
      ? null
      : buildInfrastructureEmptyHistoryLabel(oldestDataTimestamp === null),
  };
}

export function buildStorageCapacityTrendPoints(
  pools: Record<
    string,
    {
      name?: string;
      usage?: TrendPoint[];
      used?: TrendPoint[];
      avail?: TrendPoint[];
    }
  >,
): TrendPoint[] {
  const buckets = new Map<
    number,
    { used: number; avail: number; hasUsed: boolean; hasAvail: boolean }
  >();

  for (const pool of Object.values(pools)) {
    for (const point of pool.used ?? []) {
      if (!Number.isFinite(point.timestamp) || !Number.isFinite(point.value)) {
        continue;
      }
      const bucket = buckets.get(point.timestamp) ?? {
        used: 0,
        avail: 0,
        hasUsed: false,
        hasAvail: false,
      };
      bucket.used += point.value;
      bucket.hasUsed = true;
      buckets.set(point.timestamp, bucket);
    }
    for (const point of pool.avail ?? []) {
      if (!Number.isFinite(point.timestamp) || !Number.isFinite(point.value)) {
        continue;
      }
      const bucket = buckets.get(point.timestamp) ?? {
        used: 0,
        avail: 0,
        hasUsed: false,
        hasAvail: false,
      };
      bucket.avail += point.value;
      bucket.hasAvail = true;
      buckets.set(point.timestamp, bucket);
    }
  }

  return Array.from(buckets.entries())
    .sort((a, b) => a[0] - b[0])
    .flatMap(([timestamp, bucket]) => {
      if (!bucket.hasUsed || !bucket.hasAvail) {
        return [];
      }
      const total = bucket.used + bucket.avail;
      if (!Number.isFinite(total) || total <= 0) {
        return [];
      }
      return [
        {
          timestamp,
          value: (bucket.used / total) * 100,
        },
      ];
    });
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

function buildStorageTrendSnapshot(
  storageSummary: StorageSummaryTrendResponse | null,
): DashboardTrends['storage'] {
  if (!storageSummary) {
    return createEmptyStorageTrends();
  }
  return {
    capacity: extractTrendData(storageSummary.capacity),
  };
}

export function useDashboardTrends(
  overview: Accessor<DashboardOverviewSummary>,
  infrastructureRange?: Accessor<HistoryTimeRange>,
): Accessor<DashboardTrends> {
  const [infrastructureError, setInfrastructureError] = createSignal<string | null>(null);
  const [storageError, setStorageError] = createSignal<string | null>(null);
  const [infrastructureCharts, setInfrastructureCharts] = createSignal<Map<string, ChartData>>(
    new Map(),
  );
  const [oldestDataTimestamp, setOldestDataTimestamp] = createSignal<number | null>(null);
  const [storageSummary, setStorageSummary] = createSignal<StorageSummaryTrendResponse | null>(
    null,
  );
  const [infrastructureLoading, setInfrastructureLoading] = createSignal(false);
  const [storageLoading, setStorageLoading] = createSignal(false);
  const [orgVersion, setOrgVersion] = createSignal(0);

  const unsubscribeOrgSwitch = eventBus.on('org_switched', () => {
    setOrgVersion((value) => value + 1);
  });

  onCleanup(() => {
    unsubscribeOrgSwitch();
  });

  const hasInfrastructureResources = createMemo(() =>
    overview().infrastructure.total > 0,
  );
  const hasStorageResources = createMemo(() => overview().storage.total > 0);

  const infrastructureScopeKey = createMemo(() => {
    const version = orgVersion();
    const orgScope = normalizeOrgScope(getOrgID());
    const range = infrastructureRange ? infrastructureRange() : INFRASTRUCTURE_RANGE;
    return JSON.stringify({
      orgScope,
      version,
      hasInfrastructure: hasInfrastructureResources(),
      infrastructureRange: range,
    });
  });

  const storageScopeKey = createMemo(() => {
    const version = orgVersion();
    const orgScope = normalizeOrgScope(getOrgID());
    return JSON.stringify({
      orgScope,
      version,
      hasStorage: hasStorageResources(),
      storageRange: STORAGE_RANGE,
    });
  });

  let latestInfrastructureRequestId = 0;
  let latestStorageRequestId = 0;

  createEffect(() => {
    infrastructureScopeKey();
    const range = infrastructureRange ? infrastructureRange() : INFRASTRUCTURE_RANGE;
    const summaryRange = toInfrastructureSummaryRange(range);

    if (!hasInfrastructureResources()) {
      setInfrastructureCharts(new Map());
      setOldestDataTimestamp(null);
      setInfrastructureError(null);
      setInfrastructureLoading(false);
      return;
    }

    const cached = readInfrastructureSummaryCache(
      summaryRange,
      undefined,
      undefined,
      DASHBOARD_INFRASTRUCTURE_METRICS,
    );
    if (cached) {
      setInfrastructureCharts(cached.map);
      setOldestDataTimestamp(cached.oldestDataTimestamp);
      setInfrastructureError(null);
      setInfrastructureLoading(false);
      return;
    }

    const requestId = ++latestInfrastructureRequestId;
    setInfrastructureLoading(true);

    fetchInfrastructureSummaryAndCache(summaryRange, {
      caller: 'useDashboardTrends',
      metrics: DASHBOARD_INFRASTRUCTURE_METRICS,
    })
      .then((result) => {
        if (requestId !== latestInfrastructureRequestId) return;
        setInfrastructureCharts(result.map);
        setOldestDataTimestamp(result.oldestDataTimestamp);
        setInfrastructureError(null);
      })
      .catch((err) => {
        if (requestId !== latestInfrastructureRequestId) return;
        if (!cached) {
          setInfrastructureCharts(new Map());
          setOldestDataTimestamp(null);
        }
        setInfrastructureError(toErrorMessage(err));
      })
      .finally(() => {
        if (requestId === latestInfrastructureRequestId) {
          setInfrastructureLoading(false);
        }
      });
  });

  createEffect(() => {
    storageScopeKey();

    if (!hasStorageResources()) {
      setStorageSummary(null);
      setStorageError(null);
      setStorageLoading(false);
      return;
    }

    const cached = readStorageSummaryTrendCache(STORAGE_RANGE);
    if (cached) {
      setStorageSummary(cached);
    }

    const requestId = ++latestStorageRequestId;
    setStorageLoading(true);

    fetchStorageSummaryTrendAndCache(STORAGE_RANGE, { caller: 'useDashboardTrends' })
      .then((result) => {
        if (requestId !== latestStorageRequestId) return;
        setStorageSummary(result);
        setStorageError(null);
      })
      .catch((err) => {
        if (requestId !== latestStorageRequestId) return;
        if (!cached) {
          setStorageSummary(null);
        }
        setStorageError(toErrorMessage(err));
      })
      .finally(() => {
        if (requestId === latestStorageRequestId) {
          setStorageLoading(false);
        }
      });
  });

  const infrastructureSnapshot = createMemo(() =>
    buildInfrastructureTrendSnapshot(overview(), infrastructureCharts(), oldestDataTimestamp()),
  );
  const storageSnapshot = createMemo(() => buildStorageTrendSnapshot(storageSummary()));

  return createMemo<DashboardTrends>(() => {
    return {
      infrastructure: infrastructureSnapshot(),
      storage: storageSnapshot(),
      loading: infrastructureLoading() || storageLoading(),
      error: infrastructureError() ?? storageError(),
    };
  });
}
