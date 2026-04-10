import { createRoot, createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DashboardOverviewSummary } from '@/hooks/useDashboardOverview';

vi.mock('@/utils/apiClient', () => ({
  getOrgID: () => 'test-org',
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: vi.fn(() => () => {}),
  },
}));

vi.mock('@/utils/infrastructureSummaryCache', () => ({
  fetchInfrastructureSummaryAndCache: vi.fn(),
  readInfrastructureSummaryCache: vi.fn(),
}));

vi.mock('@/utils/storageSummaryTrendCache', () => ({
  fetchStorageSummaryTrendAndCache: vi.fn(),
  readStorageSummaryTrendCache: vi.fn(),
}));

import useDashboardTrendsSource from '@/hooks/useDashboardTrends.ts?raw';
import {
  buildStorageCapacityTrendPoints,
  computeTrendDelta,
  extractTrendData,
  useDashboardTrends,
  type TrendPoint,
} from '@/hooks/useDashboardTrends';
import {
  fetchInfrastructureSummaryAndCache,
  readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';
import {
  fetchStorageSummaryTrendAndCache,
  readStorageSummaryTrendCache,
} from '@/utils/storageSummaryTrendCache';

function createOverviewSummary(
  overrides: Partial<DashboardOverviewSummary> = {},
): DashboardOverviewSummary {
  return {
    health: {
      totalResources: 0,
      byStatus: {},
      ...overrides.health,
    },
    infrastructure: {
      total: 0,
      byStatus: {},
      byType: {},
      topCPU: [],
      topMemory: [],
      ...overrides.infrastructure,
    },
    workloads: {
      total: 0,
      running: 0,
      stopped: 0,
      byType: {},
      ...overrides.workloads,
    },
    storage: {
      total: 0,
      totalCapacity: 0,
      totalUsed: 0,
      warningCount: 0,
      criticalCount: 0,
      ...overrides.storage,
    },
    problemResources: [],
    ...overrides,
  };
}

function createPoints(values: number[]): TrendPoint[] {
  const start = 1_700_000_000_000;
  return values.map((value, index) => ({
    timestamp: start + index * 60_000,
    value,
  }));
}

describe('computeTrendDelta', () => {
  it('returns null for empty points', () => {
    expect(computeTrendDelta([])).toBeNull();
  });

  it('returns null for a single point', () => {
    expect(computeTrendDelta(createPoints([42]))).toBeNull();
  });

  it('returns positive delta for an increasing trend', () => {
    const delta = computeTrendDelta(createPoints([10, 12, 14, 16, 18, 20, 22, 24]));
    expect(delta).not.toBeNull();
    expect(delta ?? 0).toBeGreaterThan(0);
  });

  it('returns negative delta for a decreasing trend', () => {
    const delta = computeTrendDelta(createPoints([24, 22, 20, 18, 16, 14, 12, 10]));
    expect(delta).not.toBeNull();
    expect(delta ?? 0).toBeLessThan(0);
  });

  it('returns near-zero delta for a flat trend', () => {
    const delta = computeTrendDelta(createPoints([55, 55, 55, 55, 55, 55, 55, 55]));
    expect(delta).not.toBeNull();
    expect(delta ?? 0).toBeCloseTo(0, 10);
  });

  it('supports exactly two points', () => {
    const delta = computeTrendDelta(createPoints([10, 20]));
    expect(delta).not.toBeNull();
    expect(delta ?? 0).toBeCloseTo(100, 6);
  });
});

describe('extractTrendData', () => {
  it('returns empty trend data for empty input', () => {
    expect(extractTrendData([])).toEqual({
      points: [],
      delta: null,
      currentValue: null,
    });
  });

  it('returns empty trend data for a single point input', () => {
    expect(extractTrendData(createPoints([80]))).toEqual({
      points: [],
      delta: null,
      currentValue: null,
    });
  });

  it('normalizes, sorts, and computes trend fields for real-ish data', () => {
    const rawPoints = [
      { timestamp: 1_700_000_360_000, value: 65 },
      { timestamp: 1_700_000_000_000, value: 50 },
      { timestamp: 1_700_000_180_000, value: 55 },
      { timestamp: 1_700_000_540_000, value: 72 },
    ];

    const trend = extractTrendData(rawPoints);

    expect(trend.points.map((point) => point.timestamp)).toEqual([
      1_700_000_000_000, 1_700_000_180_000, 1_700_000_360_000, 1_700_000_540_000,
    ]);
    expect(trend.currentValue).toBe(72);
    expect(trend.delta).not.toBeNull();
    expect(trend.delta ?? 0).toBeGreaterThan(0);
  });
});

describe('buildStorageCapacityTrendPoints', () => {
  it('aggregates used and available bytes into total capacity percentages', () => {
    const points = buildStorageCapacityTrendPoints({
      'pool-a': {
        name: 'Pool A',
        usage: [],
        used: createPoints([400, 600]),
        avail: createPoints([600, 400]),
      },
      'pool-b': {
        name: 'Pool B',
        usage: [],
        used: createPoints([100, 300]),
        avail: createPoints([900, 700]),
      },
    });

    expect(points).toEqual([
      { timestamp: 1_700_000_000_000, value: 25 },
      { timestamp: 1_700_000_060_000, value: 45 },
    ]);
  });

  it('drops timestamps without both used and available capacity', () => {
    const points = buildStorageCapacityTrendPoints({
      'pool-a': {
        name: 'Pool A',
        usage: [],
        used: createPoints([400, 600]),
        avail: [{ timestamp: 1_700_000_000_000, value: 600 }],
      },
    });

    expect(points).toEqual([{ timestamp: 1_700_000_000_000, value: 40 }]);
  });
});

describe('useDashboardTrends fetch scoping', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(readInfrastructureSummaryCache).mockReturnValue(null);
    vi.mocked(fetchInfrastructureSummaryAndCache).mockResolvedValue({
      map: new Map([
        [
          'infra-a',
          {
            cpu: createPoints([20, 30]),
            memory: createPoints([40, 50]),
          },
        ],
        [
          'infra-b',
          {
            cpu: createPoints([45, 55]),
            memory: createPoints([65, 75]),
          },
        ],
      ]),
      oldestDataTimestamp: null,
    });
    vi.mocked(readStorageSummaryTrendCache).mockReturnValue(null);
    vi.mocked(fetchStorageSummaryTrendAndCache).mockResolvedValue({
      capacity: createPoints([40, 50]),
      timestamp: 1_700_000_060_000,
      stats: {} as never,
    });
  });

  it('does not refetch dashboard summary charts as the compact overview expands within the same scope', async () => {
    const initialOverview = createOverviewSummary({
      health: { totalResources: 2, byStatus: { online: 2 } },
      infrastructure: {
        total: 1,
        byStatus: { online: 1 },
        byType: { agent: 1 },
        topCPU: [{ id: 'infra-a', name: 'Infra A', percent: 55 }],
        topMemory: [{ id: 'infra-a', name: 'Infra A', percent: 75 }],
      },
      storage: {
        total: 1,
        totalCapacity: 1000,
        totalUsed: 500,
        warningCount: 0,
        criticalCount: 0,
      },
    });
    const expandedOverview = createOverviewSummary({
      ...initialOverview,
      health: { totalResources: 4, byStatus: { online: 4 } },
      infrastructure: {
        total: 2,
        byStatus: { online: 2 },
        byType: { agent: 2 },
        topCPU: [
          { id: 'infra-a', name: 'Infra A', percent: 55 },
          { id: 'infra-b', name: 'Infra B', percent: 45 },
        ],
        topMemory: [
          { id: 'infra-a', name: 'Infra A', percent: 75 },
          { id: 'infra-b', name: 'Infra B', percent: 65 },
        ],
      },
    });

    let dispose!: () => void;
    let trends!: ReturnType<typeof useDashboardTrends>;
    let setOverview!: (value: DashboardOverviewSummary) => void;

    createRoot((d) => {
      dispose = d;
      const [overview, setOverviewSignal] = createSignal(initialOverview);
      setOverview = setOverviewSignal;
      const [range] = createSignal<'1h'>('1h');
      trends = useDashboardTrends(overview, range);
    });

    await vi.waitFor(() => {
      expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledTimes(1);
      expect(vi.mocked(fetchStorageSummaryTrendAndCache)).toHaveBeenCalledTimes(1);
      expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledWith('1h', {
        caller: 'useDashboardTrends',
        metrics: ['cpu', 'memory'],
      });
    });

    setOverview(expandedOverview);

    await Promise.resolve();
    await Promise.resolve();

    expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(fetchStorageSummaryTrendAndCache)).toHaveBeenCalledTimes(1);
    expect(Array.from(trends().infrastructure.cpu.keys())).toEqual(['infra-a', 'infra-b']);

    dispose();
  });

  it('joins dashboard sparkline rankings through canonical metrics targets instead of unified resource ids', async () => {
    const overview = createOverviewSummary({
      health: { totalResources: 2, byStatus: { online: 2 } },
      infrastructure: {
        total: 1,
        byStatus: { online: 1 },
        byType: { agent: 1 },
        topCPU: [
          {
            id: 'agent-a',
            name: 'Infra A',
            percent: 55,
            metricsTarget: { resourceType: 'agent', resourceId: 'infra-a' },
          },
        ],
        topMemory: [
          {
            id: 'agent-a',
            name: 'Infra A',
            percent: 75,
            metricsTarget: { resourceType: 'agent', resourceId: 'infra-a' },
          },
        ],
      },
    });

    let dispose!: () => void;
    let trends!: ReturnType<typeof useDashboardTrends>;

    createRoot((d) => {
      dispose = d;
      const [summary] = createSignal(overview);
      const [range] = createSignal<'1h'>('1h');
      trends = useDashboardTrends(summary, range);
    });

    await vi.waitFor(() => {
      expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledTimes(1);
    });

    expect(Array.from(trends().infrastructure.cpu.keys())).toEqual(['agent-a']);
    expect(trends().infrastructure.cpu.get('agent-a')?.points).toEqual(createPoints([20, 30]));
    expect(Array.from(trends().infrastructure.memory.keys())).toEqual(['agent-a']);
    expect(trends().infrastructure.memory.get('agent-a')?.points).toEqual(createPoints([40, 50]));

    dispose();
  });

  it('refetches infrastructure charts when the dashboard trend range changes without refetching storage charts', async () => {
    const overview = createOverviewSummary({
      health: { totalResources: 2, byStatus: { online: 2 } },
      infrastructure: {
        total: 1,
        byStatus: { online: 1 },
        byType: { agent: 1 },
        topCPU: [{ id: 'infra-a', name: 'Infra A', percent: 55 }],
        topMemory: [{ id: 'infra-a', name: 'Infra A', percent: 75 }],
      },
      storage: {
        total: 1,
        totalCapacity: 1000,
        totalUsed: 500,
        warningCount: 0,
        criticalCount: 0,
      },
    });

    let dispose!: () => void;
    let setRange!: (value: '1h' | '12h') => void;

    createRoot((d) => {
      dispose = d;
      const [summary] = createSignal(overview);
      const [range, setRangeSignal] = createSignal<'1h' | '12h'>('1h');
      setRange = setRangeSignal;
      useDashboardTrends(summary, range);
    });

    await vi.waitFor(() => {
      expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledTimes(1);
      expect(vi.mocked(fetchStorageSummaryTrendAndCache)).toHaveBeenCalledTimes(1);
      expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledWith('1h', {
        caller: 'useDashboardTrends',
        metrics: ['cpu', 'memory'],
      });
    });

    setRange('12h');

    await vi.waitFor(() => {
      expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledTimes(2);
    });
    expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenLastCalledWith('12h', {
      caller: 'useDashboardTrends',
      metrics: ['cpu', 'memory'],
    });
    expect(vi.mocked(fetchStorageSummaryTrendAndCache)).toHaveBeenCalledTimes(1);

    dispose();
  });

  it('reuses a fresh infrastructure summary cache instead of immediately refetching the same scope', async () => {
    const overview = createOverviewSummary({
      health: { totalResources: 2, byStatus: { online: 2 } },
      infrastructure: {
        total: 1,
        byStatus: { online: 1 },
        byType: { agent: 1 },
        topCPU: [{ id: 'infra-a', name: 'Infra A', percent: 55 }],
        topMemory: [{ id: 'infra-a', name: 'Infra A', percent: 75 }],
      },
      storage: {
        total: 1,
        totalCapacity: 1000,
        totalUsed: 500,
        warningCount: 0,
        criticalCount: 0,
      },
    });

    vi.mocked(readInfrastructureSummaryCache).mockReturnValue({
      map: new Map(),
      oldestDataTimestamp: null,
      cachedAt: Date.now(),
    });

    let dispose!: () => void;

    createRoot((d) => {
      dispose = d;
      const [summary] = createSignal(overview);
      const [range] = createSignal<'1h'>('1h');
      useDashboardTrends(summary, range);
    });

    await Promise.resolve();
    await Promise.resolve();

    expect(vi.mocked(fetchInfrastructureSummaryAndCache)).not.toHaveBeenCalled();
    expect(vi.mocked(fetchStorageSummaryTrendAndCache)).toHaveBeenCalledTimes(1);

    dispose();
  });
});

describe('useDashboardTrends infrastructure routing', () => {
  it('routes dashboard infrastructure sparklines through the infrastructure summary chart cache', () => {
    expect(useDashboardTrendsSource).toContain('readInfrastructureSummaryCache');
    expect(useDashboardTrendsSource).toContain('fetchInfrastructureSummaryAndCache');
    expect(useDashboardTrendsSource).toContain("caller: 'useDashboardTrends'");
    expect(useDashboardTrendsSource).toContain('const DASHBOARD_INFRASTRUCTURE_METRICS');
    expect(useDashboardTrendsSource).toContain('metrics: DASHBOARD_INFRASTRUCTURE_METRICS');
    expect(useDashboardTrendsSource).toContain('const infrastructureScopeKey');
    expect(useDashboardTrendsSource).toContain('const hasInfrastructureResources');
    expect(useDashboardTrendsSource).toContain('buildMetricTrendMap');
    expect(useDashboardTrendsSource).not.toContain('buildInfrastructureSummarySeries');
    expect(useDashboardTrendsSource).not.toContain('request.cpu.map(async');
    expect(useDashboardTrendsSource).not.toContain('request.memory.map(async');
  });

  it('routes storage trends through the storage summary charts endpoint', () => {
    expect(useDashboardTrendsSource).toContain('readStorageSummaryTrendCache');
    expect(useDashboardTrendsSource).toContain('fetchStorageSummaryTrendAndCache(STORAGE_RANGE');
    expect(useDashboardTrendsSource).toContain('const storageScopeKey');
    expect(useDashboardTrendsSource).toContain('extractTrendData(storageSummary.capacity)');
    expect(useDashboardTrendsSource).not.toContain('/api/storage-charts');
    expect(useDashboardTrendsSource).not.toContain('request.storage.map(async');
  });
});
