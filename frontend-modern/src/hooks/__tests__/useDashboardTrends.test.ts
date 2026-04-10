import { createRoot, createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';

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

vi.mock('@/components/Infrastructure/infrastructureSummaryModel', () => ({
  buildInfrastructureEmptyHistoryLabel: vi.fn(() => null),
  buildInfrastructureSummarySeries: vi.fn((resources: Resource[]) =>
    resources
      .filter((resource) =>
        ['agent', 'docker-host', 'k8s-cluster', 'k8s-node'].includes(resource.type),
      )
      .map((resource) => ({
        id: resource.id,
        cpu: [
          { timestamp: 1_700_000_000_000, value: 20 },
          { timestamp: 1_700_000_060_000, value: 30 },
        ],
        memory: [
          { timestamp: 1_700_000_000_000, value: 40 },
          { timestamp: 1_700_000_060_000, value: 50 },
        ],
      })),
  ),
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

function createResource(partial: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource {
  return {
    ...partial,
    id: partial.id,
    type: partial.type,
    name: partial.name ?? partial.id,
    displayName: partial.displayName ?? partial.name ?? partial.id,
    platformId: partial.platformId ?? 'platform-1',
    platformType: partial.platformType ?? 'proxmox',
    sourceType: partial.sourceType ?? 'proxmox',
    status: partial.status ?? 'online',
    lastSeen: partial.lastSeen ?? 1_700_000_000_000,
  } as Resource;
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
      map: new Map(),
      oldestDataTimestamp: null,
    });
    vi.mocked(readStorageSummaryTrendCache).mockReturnValue(null);
    vi.mocked(fetchStorageSummaryTrendAndCache).mockResolvedValue({
      capacity: createPoints([40, 50]),
      timestamp: 1_700_000_060_000,
      stats: {} as never,
    });
  });

  it('does not refetch dashboard summary charts as paginated resources expand within the same scope', async () => {
    const infrastructureA = createResource({ id: 'infra-a', type: 'agent' });
    const infrastructureB = createResource({ id: 'infra-b', type: 'agent' });
    const storageA = createResource({ id: 'storage-a', type: 'storage' });
    const storageB = createResource({ id: 'storage-b', type: 'storage' });

    let dispose!: () => void;
    let trends!: ReturnType<typeof useDashboardTrends>;
    let setResources!: (value: Resource[]) => void;

    createRoot((d) => {
      dispose = d;
      const [resources, setResourcesSignal] = createSignal<Resource[]>([infrastructureA, storageA]);
      setResources = setResourcesSignal;
      const [range] = createSignal<'1h'>('1h');
      trends = useDashboardTrends(resources, range);
    });

    await vi.waitFor(() => {
      expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledTimes(1);
      expect(vi.mocked(fetchStorageSummaryTrendAndCache)).toHaveBeenCalledTimes(1);
      expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledWith('1h', {
        caller: 'useDashboardTrends',
        metrics: ['cpu', 'memory'],
      });
    });

    setResources([infrastructureA, infrastructureB, storageA, storageB]);

    await Promise.resolve();
    await Promise.resolve();

    expect(vi.mocked(fetchInfrastructureSummaryAndCache)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(fetchStorageSummaryTrendAndCache)).toHaveBeenCalledTimes(1);
    expect(Array.from(trends().infrastructure.cpu.keys())).toEqual(['infra-a', 'infra-b']);

    dispose();
  });

  it('refetches infrastructure charts when the dashboard trend range changes without refetching storage charts', async () => {
    const infrastructureA = createResource({ id: 'infra-a', type: 'agent' });
    const storageA = createResource({ id: 'storage-a', type: 'storage' });

    let dispose!: () => void;
    let setRange!: (value: '1h' | '12h') => void;

    createRoot((d) => {
      dispose = d;
      const [resources] = createSignal<Resource[]>([infrastructureA, storageA]);
      const [range, setRangeSignal] = createSignal<'1h' | '12h'>('1h');
      setRange = setRangeSignal;
      useDashboardTrends(resources, range);
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
    const infrastructureA = createResource({ id: 'infra-a', type: 'agent' });
    const storageA = createResource({ id: 'storage-a', type: 'storage' });

    vi.mocked(readInfrastructureSummaryCache).mockReturnValue({
      map: new Map(),
      oldestDataTimestamp: null,
      cachedAt: Date.now(),
    });

    let dispose!: () => void;

    createRoot((d) => {
      dispose = d;
      const [resources] = createSignal<Resource[]>([infrastructureA, storageA]);
      const [range] = createSignal<'1h'>('1h');
      useDashboardTrends(resources, range);
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
    expect(useDashboardTrendsSource).toContain("const DASHBOARD_INFRASTRUCTURE_METRICS");
    expect(useDashboardTrendsSource).toContain("metrics: DASHBOARD_INFRASTRUCTURE_METRICS");
    expect(useDashboardTrendsSource).toContain('const infrastructureScopeKey');
    expect(useDashboardTrendsSource).toContain('const hasInfrastructureResources');
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
