import { describe, expect, it } from 'vitest';
import useDashboardTrendsSource from '@/hooks/useDashboardTrends.ts?raw';
import {
  buildStorageCapacityTrendPoints,
  computeTrendDelta,
  extractTrendData,
  type TrendPoint,
} from '@/hooks/useDashboardTrends';

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

describe('useDashboardTrends infrastructure routing', () => {
  it('routes dashboard infrastructure sparklines through the infrastructure summary chart cache', () => {
    expect(useDashboardTrendsSource).toContain('fetchInfrastructureSummaryAndCache');
    expect(useDashboardTrendsSource).toContain("caller: 'useDashboardTrends'");
    expect(useDashboardTrendsSource).not.toContain('request.cpu.map(async');
    expect(useDashboardTrendsSource).not.toContain('request.memory.map(async');
  });

  it('routes storage trends through the storage summary charts endpoint', () => {
    expect(useDashboardTrendsSource).toContain('ChartService.getStorageSummaryCharts(STORAGE_RANGE)');
    expect(useDashboardTrendsSource).toContain('buildStorageCapacityTrendPoints(storageSummary.pools)');
    expect(useDashboardTrendsSource).not.toContain('metrics-store/history');
    expect(useDashboardTrendsSource).not.toContain('request.storage.map(async');
  });
});
