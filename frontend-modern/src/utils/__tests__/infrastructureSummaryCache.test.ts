import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChartData, TimeRange } from '@/api/charts';
import { setOrgID } from '@/utils/apiClient';
import {
  __resetInfrastructureSummaryFetchesForTests,
  extractInfrastructureSummaryChartMapFromInfrastructureResponse,
  fetchInfrastructureSummaryAndCache,
  persistInfrastructureSummaryCache,
  readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';

const mockGetCharts = vi.fn();

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getInfrastructureSummaryCharts: (...args: unknown[]) => mockGetCharts(...args),
    },
  };
});

const makeResponse = () => ({
  nodeData: {
    'node-1': {
      cpu: [
        { timestamp: Date.now() - 30_000, value: 10 },
        { timestamp: Date.now(), value: 15 },
      ],
      memory: [
        { timestamp: Date.now() - 30_000, value: 30 },
        { timestamp: Date.now(), value: 35 },
      ],
      disk: [
        { timestamp: Date.now() - 30_000, value: 40 },
        { timestamp: Date.now(), value: 45 },
      ],
    },
  },
  dockerHostData: {},
  hostData: {},
  timestamp: Date.now(),
  stats: {
    oldestDataTimestamp: Date.now() - 30_000,
  },
});

const makeMetricSeries = (count: number, start = Date.now() - count * 30_000) =>
  Array.from({ length: count }, (_, i) => ({
    timestamp: start + i * 30_000,
    value: i % 100,
  }));

const cacheKeyForRange = (range: TimeRange, orgScope = 'default') =>
  `pulse.infrastructureSummaryCharts.${encodeURIComponent(orgScope)}::${range}`;

describe('infrastructureSummaryCache fetch dedupe', () => {
  beforeEach(() => {
    mockGetCharts.mockReset();
    __resetInfrastructureSummaryFetchesForTests();
    setOrgID('default');
    localStorage.clear();
  });

  it('deduplicates concurrent requests for the same range', async () => {
    let resolveFetch: ((value: ReturnType<typeof makeResponse>) => void) | undefined;
    mockGetCharts.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFetch = resolve as (value: ReturnType<typeof makeResponse>) => void;
        }),
    );

    const first = fetchInfrastructureSummaryAndCache('1h');
    const second = fetchInfrastructureSummaryAndCache('1h');

    expect(mockGetCharts).toHaveBeenCalledTimes(1);
    expect(mockGetCharts).toHaveBeenCalledWith('1h');

    resolveFetch?.(makeResponse());

    const [firstResult, secondResult] = await Promise.all([first, second]);
    expect(firstResult.map.size).toBe(1);
    expect(secondResult.map.size).toBe(1);
  });

  it('fetches separately per range and persists cache entries', async () => {
    mockGetCharts.mockImplementation((_range: TimeRange) => Promise.resolve(makeResponse()));

    await fetchInfrastructureSummaryAndCache('1h');
    await fetchInfrastructureSummaryAndCache('24h');

    expect(mockGetCharts).toHaveBeenCalledTimes(2);
    expect(readInfrastructureSummaryCache('1h')).not.toBeNull();
    expect(readInfrastructureSummaryCache('24h')).not.toBeNull();
  });

  it('merges overlapping hostData and dockerHostData keys without dropping richer network series', () => {
    const now = Date.now();
    const response = {
      nodeData: {},
      hostData: {
        'shared-host': {
          cpu: [
            { timestamp: now - 60_000, value: 10 },
            { timestamp: now, value: 20 },
          ],
          netin: [
            { timestamp: now - 60_000, value: 1000 },
            { timestamp: now, value: 2000 },
          ],
          netout: [
            { timestamp: now - 60_000, value: 500 },
            { timestamp: now, value: 1500 },
          ],
        },
      },
      dockerHostData: {
        'shared-host': {
          cpu: [
            { timestamp: now - 60_000, value: 30 },
            { timestamp: now, value: 40 },
          ],
          netin: [],
          netout: [],
        },
      },
      timestamp: now,
      stats: {
        oldestDataTimestamp: now - 60_000,
      },
    };

    const map = extractInfrastructureSummaryChartMapFromInfrastructureResponse(response);
    const merged = map.get('shared-host');
    expect(merged).toBeDefined();
    expect(merged?.cpu?.length).toBe(2);
    expect(merged?.netin?.length).toBe(2);
    expect(merged?.netout?.length).toBe(2);
  });
});

describe('infrastructureSummaryCache persistence', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('returns cache miss before persist and cache hit after persist', () => {
    expect(readInfrastructureSummaryCache('1h')).toBeNull();

    const now = Date.now();
    persistInfrastructureSummaryCache(
      '1h',
      new Map([
        [
          'node-1',
          {
            cpu: makeMetricSeries(2, now - 60_000),
            memory: makeMetricSeries(2, now - 60_000),
            disk: makeMetricSeries(2, now - 60_000),
            netin: makeMetricSeries(2, now - 60_000),
            netout: makeMetricSeries(2, now - 60_000),
          },
        ],
      ]),
      now - 60_000,
    );

    const cached = readInfrastructureSummaryCache('1h');
    expect(cached).not.toBeNull();
    expect(cached?.map.size).toBe(1);
    expect(cached?.map.get('node-1')?.cpu?.length).toBe(2);
  });

  it('evicts stale cache entries when ttl has expired', () => {
    vi.useFakeTimers();
    try {
      const start = new Date('2026-01-01T00:00:00.000Z');
      vi.setSystemTime(start);

      persistInfrastructureSummaryCache(
        '1h',
        new Map([
          [
            'node-1',
            {
              cpu: makeMetricSeries(2, start.getTime() - 60_000),
              memory: makeMetricSeries(2, start.getTime() - 60_000),
              disk: makeMetricSeries(2, start.getTime() - 60_000),
              netin: makeMetricSeries(2, start.getTime() - 60_000),
              netout: makeMetricSeries(2, start.getTime() - 60_000),
            },
          ],
        ]),
        start.getTime() - 60_000,
      );

      vi.setSystemTime(new Date('2026-01-01T00:06:01.000Z'));
      expect(readInfrastructureSummaryCache('1h')).toBeNull();
      expect(localStorage.getItem(cacheKeyForRange('1h'))).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('trims cached metric series to bounded point counts', () => {
    const sourceSeries = makeMetricSeries(1200);
    persistInfrastructureSummaryCache(
      '1h',
      new Map([
        [
          'node-1',
          {
            cpu: sourceSeries,
            memory: sourceSeries,
            disk: sourceSeries,
            netin: sourceSeries,
            netout: sourceSeries,
          },
        ],
      ]),
      null,
    );

    const cached = readInfrastructureSummaryCache('1h');
    const series = cached?.map.get('node-1');
    expect(series).toBeDefined();
    expect(series?.cpu?.length).toBeLessThanOrEqual(360);
    expect(series?.memory?.length).toBeLessThanOrEqual(360);
    expect(series?.disk?.length).toBeLessThanOrEqual(360);
    expect(series?.netin?.length).toBeLessThanOrEqual(360);
    expect(series?.netout?.length).toBeLessThanOrEqual(360);
    const cachedCpu = series?.cpu ?? [];
    expect(cachedCpu[cachedCpu.length - 1]?.timestamp).toBe(sourceSeries[sourceSeries.length - 1]?.timestamp);
  });

  it('drops oversized cache payloads to keep storage bounded', () => {
    const denseSeries = makeMetricSeries(360);
    const oversizedMap = new Map<string, ChartData>();
    for (let i = 0; i < 40; i++) {
      oversizedMap.set(`node-${i}`, {
        cpu: denseSeries,
        memory: denseSeries,
        disk: denseSeries,
        netin: denseSeries,
        netout: denseSeries,
      });
    }

    persistInfrastructureSummaryCache('1h', oversizedMap, null);
    expect(localStorage.getItem(cacheKeyForRange('1h'))).toBeNull();
  });

  it('keeps cache entries isolated per org scope', () => {
    const now = Date.now();

    setOrgID('org-a');
    persistInfrastructureSummaryCache(
      '1h',
      new Map([
        [
          'node-a',
          {
            cpu: makeMetricSeries(2, now - 60_000),
            memory: makeMetricSeries(2, now - 60_000),
            disk: makeMetricSeries(2, now - 60_000),
            netin: makeMetricSeries(2, now - 60_000),
            netout: makeMetricSeries(2, now - 60_000),
          },
        ],
      ]),
      now - 60_000,
    );

    setOrgID('org-b');
    persistInfrastructureSummaryCache(
      '1h',
      new Map([
        [
          'node-b',
          {
            cpu: makeMetricSeries(2, now - 60_000),
            memory: makeMetricSeries(2, now - 60_000),
            disk: makeMetricSeries(2, now - 60_000),
            netin: makeMetricSeries(2, now - 60_000),
            netout: makeMetricSeries(2, now - 60_000),
          },
        ],
      ]),
      now - 60_000,
    );

    expect(readInfrastructureSummaryCache('1h')?.map.get('node-b')).toBeDefined();
    expect(readInfrastructureSummaryCache('1h')?.map.get('node-a')).toBeUndefined();

    expect(readInfrastructureSummaryCache('1h', undefined, 'org-a')?.map.get('node-a')).toBeDefined();
    expect(readInfrastructureSummaryCache('1h', undefined, 'org-a')?.map.get('node-b')).toBeUndefined();
    expect(localStorage.getItem(cacheKeyForRange('1h', 'org-a'))).not.toBeNull();
    expect(localStorage.getItem(cacheKeyForRange('1h', 'org-b'))).not.toBeNull();
  });
});
