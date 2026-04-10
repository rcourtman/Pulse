import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';
import {
  type ChartData,
  type HistoryTimeRange,
  type StorageSummaryChartsResponse,
  type TimeRange,
} from '@/api/charts';
import {
  buildInfrastructureEmptyHistoryLabel,
  buildInfrastructureSummarySeries,
} from '@/components/Infrastructure/infrastructureSummaryModel';
import { eventBus } from '@/stores/events';
import { isAgentFacetInfrastructureResource } from '@/utils/agentResources';
import { getOrgID } from '@/utils/apiClient';
import {
  fetchInfrastructureSummaryAndCache,
  readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';
import { normalizeOrgScope } from '@/utils/orgScope';
import {
  fetchStorageSummaryAndCache,
  readStorageSummaryCache,
} from '@/utils/storageSummaryCache';
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

const INFRASTRUCTURE_RANGE: HistoryTimeRange = '1h';
const STORAGE_RANGE: TimeRange = '24h';

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

function buildInfrastructureTrendSnapshot(
  resources: Resource[],
  map: Map<string, ChartData>,
  oldestDataTimestamp: number | null,
): DashboardTrends['infrastructure'] {
  const infrastructureResources = resources.filter((resource) => isInfrastructure(resource));
  if (infrastructureResources.length === 0) {
    return createEmptyInfrastructureTrends();
  }

  const agentFacetResources = resources.filter((resource) =>
    isAgentFacetInfrastructureResource(resource),
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

export function buildStorageCapacityTrendPoints(
  pools: StorageSummaryChartsResponse['pools'],
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
  storageSummary: StorageSummaryChartsResponse | null,
): DashboardTrends['storage'] {
  if (!storageSummary) {
    return createEmptyStorageTrends();
  }
  return {
    capacity: extractTrendData(buildStorageCapacityTrendPoints(storageSummary.pools)),
  };
}

export function useDashboardTrends(
  resources: Accessor<Resource[]>,
  infrastructureRange?: Accessor<HistoryTimeRange>,
): Accessor<DashboardTrends> {
  const [infrastructureError, setInfrastructureError] = createSignal<string | null>(null);
  const [storageError, setStorageError] = createSignal<string | null>(null);
  const [infrastructureCharts, setInfrastructureCharts] = createSignal<Map<string, ChartData>>(
    new Map(),
  );
  const [oldestDataTimestamp, setOldestDataTimestamp] = createSignal<number | null>(null);
  const [storageSummary, setStorageSummary] = createSignal<StorageSummaryChartsResponse | null>(
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
    resources().some((resource) => isInfrastructure(resource)),
  );
  const hasStorageResources = createMemo(() => resources().some((resource) => isStorage(resource)));

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

    const cached = readInfrastructureSummaryCache(summaryRange);
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

    const cached = readStorageSummaryCache(STORAGE_RANGE);
    if (cached) {
      setStorageSummary(cached);
    }

    const requestId = ++latestStorageRequestId;
    setStorageLoading(true);

    fetchStorageSummaryAndCache(STORAGE_RANGE, { caller: 'useDashboardTrends' })
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
    buildInfrastructureTrendSnapshot(resources(), infrastructureCharts(), oldestDataTimestamp()),
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
