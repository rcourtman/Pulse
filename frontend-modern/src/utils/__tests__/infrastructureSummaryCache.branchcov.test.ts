import { beforeEach, describe, expect, it, vi } from 'vitest';
import type {
  ChartStats,
  InfrastructureChartsResponse,
  TimeRange,
} from '@/api/charts';
import { setOrgID } from '@/utils/apiClient';
import {
  __resetInfrastructureSummaryFetchesForTests,
  extractInfrastructureSummaryChartMapFromInfrastructureResponse,
  fetchInfrastructureSummaryAndCache,
  persistInfrastructureSummaryCache,
  readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';

// ChartsAPI is mocked so fetchInfrastructureSummaryAndCache is deterministic and
// offline. apiClient stays real (setOrgID) and orgScope/events stay real, exactly
// like the sibling infrastructureSummaryCache.test.ts.
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

const DEFAULT_METRICS_KEY = 'cpu,memory,disk,diskread,diskwrite,netin,netout';

const cacheKeyForRange = (
  range: TimeRange,
  orgScope = 'default',
  metrics = DEFAULT_METRICS_KEY,
) => `pulse.infrastructureSummaryCharts.${encodeURIComponent(orgScope)}::${range}::${metrics}`;

interface MetricPoint {
  timestamp: number;
  value: number;
}

const metricPoint = (timestamp: number, value: number): MetricPoint => ({ timestamp, value });

// Build a series of N points starting at `start`, stepping by `step` ms.
const makeSeries = (count: number, start = 0, step = 30_000): MetricPoint[] =>
  Array.from({ length: count }, (_, i) => metricPoint(start + i * step, i));

const now = () => Date.now();

// A minimal, type-correct InfrastructureChartsResponse fixture.
const makeInfraResponse = (
  overrides: Partial<InfrastructureChartsResponse> = {},
): InfrastructureChartsResponse => ({
  nodeData: {},
  timestamp: now(),
  stats: { oldestDataTimestamp: now() - 60_000 },
  ...overrides,
});

// Shape of the JSON payload persistInfrastructureSummaryCache writes. Used to
// hand-craft localStorage entries that exercise readInfrastructureSummaryCache's
// invalidation / coercion branches directly.
interface StoredPayload {
  version: number;
  range: TimeRange;
  cachedAt: number;
  metrics: string[];
  oldestDataTimestamp: number | string | null;
  charts: Record<string, Record<string, unknown>> | undefined;
}

const storePayload = (
  range: TimeRange,
  payload: StoredPayload,
  orgScope = 'default',
  metrics = DEFAULT_METRICS_KEY,
) => {
  window.localStorage.setItem(cacheKeyForRange(range, orgScope, metrics), JSON.stringify(payload));
};

const buildPayload = (overrides: Partial<StoredPayload> = {}): StoredPayload => ({
  version: 5,
  range: '1h',
  cachedAt: now(),
  metrics: ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'],
  oldestDataTimestamp: null,
  charts: {},
  ...overrides,
});

describe('extractInfrastructureSummaryChartMapFromInfrastructureResponse', () => {
  it('returns an empty map when every data field is absent', () => {
    const map = extractInfrastructureSummaryChartMapFromInfrastructureResponse(
      makeInfraResponse({ nodeData: undefined }),
    );
    expect(map.size).toBe(0);
  });

  it('defaults missing nodeData to an empty object while still indexing agentData', () => {
    const map = extractInfrastructureSummaryChartMapFromInfrastructureResponse(
      makeInfraResponse({
        nodeData: undefined,
        agentData: {
          'agent-1': { cpu: makeSeries(2) },
        },
      }),
    );
    expect(map.size).toBe(1);
    expect(map.get('agent-1')?.cpu?.length).toBe(2);
  });

  it('indexes node, dockerHost and agent entries as distinct keys', () => {
    const map = extractInfrastructureSummaryChartMapFromInfrastructureResponse(
      makeInfraResponse({
        nodeData: { 'node-1': { cpu: makeSeries(1) } },
        dockerHostData: { 'docker-1': { memory: makeSeries(1) } },
        agentData: { 'agent-1': { disk: makeSeries(1) } },
      }),
    );
    expect([...map.keys()].sort()).toEqual(['agent-1', 'docker-1', 'node-1']);
    expect(map.get('node-1')?.cpu?.length).toBe(1);
    expect(map.get('docker-1')?.memory?.length).toBe(1);
    expect(map.get('agent-1')?.disk?.length).toBe(1);
  });
});

describe('readInfrastructureSummaryCache invalidation', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('returns null on a cache miss (no raw entry)', () => {
    expect(readInfrastructureSummaryCache('1h')).toBeNull();
  });

  it('evicts and returns null when the stored version mismatches', () => {
    storePayload('1h', buildPayload({ version: 4 }));
    expect(readInfrastructureSummaryCache('1h')).toBeNull();
    expect(localStorage.getItem(cacheKeyForRange('1h'))).toBeNull();
  });

  it('evicts and returns null when the stored range mismatches', () => {
    storePayload('1h', buildPayload({ range: '24h' }));
    expect(readInfrastructureSummaryCache('1h')).toBeNull();
    expect(localStorage.getItem(cacheKeyForRange('1h'))).toBeNull();
  });

  it('evicts and returns null when the stored metrics mismatch', () => {
    storePayload('1h', buildPayload({ metrics: ['cpu'] }));
    expect(readInfrastructureSummaryCache('1h')).toBeNull();
    expect(localStorage.getItem(cacheKeyForRange('1h'))).toBeNull();
  });

  it('evicts and returns null when cachedAt is not a number', () => {
    storePayload('1h', buildPayload({ cachedAt: 'recent' as unknown as number }));
    expect(readInfrastructureSummaryCache('1h')).toBeNull();
    expect(localStorage.getItem(cacheKeyForRange('1h'))).toBeNull();
  });

  it('evicts and returns null when the entry is older than a custom maxAgeMs', () => {
    storePayload('1h', buildPayload({ cachedAt: now() - 100_000 }));
    // default TTL (5m) still fresh
    expect(readInfrastructureSummaryCache('1h')).not.toBeNull();
    localStorage.clear();
    storePayload('1h', buildPayload({ cachedAt: now() - 100_000 }));
    expect(readInfrastructureSummaryCache('1h', 50_000)).toBeNull();
    expect(localStorage.getItem(cacheKeyForRange('1h'))).toBeNull();
  });

  it('returns an empty map when charts is missing', () => {
    storePayload('1h', buildPayload({ charts: undefined }));
    const cached = readInfrastructureSummaryCache('1h');
    expect(cached).not.toBeNull();
    expect(cached?.map.size).toBe(0);
    expect(cached?.cachedAt).toBeTypeOf('number');
  });

  it('coerces non-array series fields to empty arrays', () => {
    storePayload(
      '1h',
      buildPayload({
        charts: {
          'node-1': {
            cpu: makeSeries(1),
            memory: 123,
            disk: 'not-an-array',
            diskread: null,
            diskwrite: undefined,
            netin: {},
            netout: [],
          },
        },
      }),
    );
    const cached = readInfrastructureSummaryCache('1h');
    const series = cached?.map.get('node-1');
    expect(series).toBeDefined();
    expect(series?.cpu?.length).toBe(1);
    expect(series?.memory).toEqual([]);
    expect(series?.disk).toEqual([]);
    expect(series?.diskread).toEqual([]);
    expect(series?.diskwrite).toEqual([]);
    expect(series?.netin).toEqual([]);
    expect(series?.netout).toEqual([]);
  });

  it('returns null oldestDataTimestamp when the stored value is not a finite number', () => {
    storePayload(
      '1h',
      buildPayload({ oldestDataTimestamp: 'not-a-number', charts: undefined }),
    );
    const cached = readInfrastructureSummaryCache('1h');
    expect(cached).not.toBeNull();
    expect(cached?.oldestDataTimestamp).toBeNull();
  });

  it('returns the numeric oldestDataTimestamp on the happy path', () => {
    const ts = now() - 5_000;
    storePayload(
      '1h',
      buildPayload({ oldestDataTimestamp: ts, charts: undefined }),
    );
    expect(readInfrastructureSummaryCache('1h')?.oldestDataTimestamp).toBe(ts);
  });

  it('swallows malformed JSON and returns null', () => {
    window.localStorage.setItem(cacheKeyForRange('1h'), '{not valid json');
    expect(readInfrastructureSummaryCache('1h')).toBeNull();
  });
});

describe('persistInfrastructureSummaryCache write-failure path', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('swallows a localStorage.setItem failure without throwing', () => {
    const spy = vi
      .spyOn(Storage.prototype, 'setItem')
      .mockImplementation(() => {
        throw new Error('QuotaExceededError');
      });
    try {
      expect(() =>
        persistInfrastructureSummaryCache(
          '1h',
          new Map([['node-1', { cpu: makeSeries(2) }]]),
          now() - 60_000,
        ),
      ).not.toThrow();
      expect(spy).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
    }
  });
});

describe('toCachedChartData / trimPoints coverage via persist', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('fills absent series with empty arrays (?? [] arm) and keeps present series', () => {
    persistInfrastructureSummaryCache(
      '1h',
      new Map([
        // cpu present, every other series absent -> exercises the `data.X ?? []`
        // fallback arm in toCachedChartData for all optional fields.
        ['node-1', { cpu: makeSeries(3) }],
      ]),
      null,
    );
    const series = readInfrastructureSummaryCache('1h')?.map.get('node-1');
    expect(series).toBeDefined();
    expect(series?.cpu?.length).toBe(3);
    expect(series?.memory).toEqual([]);
    expect(series?.disk).toEqual([]);
    expect(series?.diskread).toEqual([]);
    expect(series?.diskwrite).toEqual([]);
    expect(series?.netin).toEqual([]);
    expect(series?.netout).toEqual([]);
  });

  it('returns the trimmed series as-is when it fits within the budget (result.length <= max arm)', () => {
    // 361 points (> 360 max) exercises the stride-sampling path but produces a
    // result shorter than max, so the final `result.length > max ? slice : result`
    // ternary takes its `result` arm rather than re-slicing.
    const series = makeSeries(361, now() - 361 * 30_000);
    persistInfrastructureSummaryCache(
      '1h',
      new Map([['node-1', { cpu: series }]]),
      null,
    );
    const cached = readInfrastructureSummaryCache('1h')?.map.get('node-1');
    expect(cached?.cpu).toBeDefined();
    // Stride of 2 over 361 points keeps the newest point and yields 181 samples.
    const cachedCpu = cached?.cpu ?? [];
    expect(cachedCpu.length).toBe(181);
    expect(cachedCpu[cachedCpu.length - 1]?.timestamp).toBe(
      series[series.length - 1]?.timestamp,
    );
  });
});

describe('countChartMapPoints coverage via fetch', () => {
  beforeEach(() => {
    mockGetCharts.mockReset();
    __resetInfrastructureSummaryFetchesForTests();
    setOrgID('default');
    localStorage.clear();
  });

  it('counts points across mixed partial-series entries without dropping undefined fields', async () => {
    // node-1 has only `cpu` (truthy length) and every other field undefined, so
    // countChartMapPoints' `data.X?.length ?? 0` expression evaluates both the
    // length arm (cpu) and the `?? 0` fallback (memory/disk/...). The perf-log
    // data object is built eagerly on every successful fetch, so this exercises
    // countChartMapPoints even though perf logging is disabled in tests.
    const response = makeInfraResponse({
      nodeData: { 'node-1': { cpu: makeSeries(4) } },
    });
    mockGetCharts.mockResolvedValueOnce(response);

    const result = await fetchInfrastructureSummaryAndCache('1h', { caller: 'branchcov' });

    expect(result.map.get('node-1')?.cpu?.length).toBe(4);
    expect(mockGetCharts).toHaveBeenCalledTimes(1);
  });
});

describe('fetchInfrastructureSummaryAndCache error + caller + stats branches', () => {
  beforeEach(() => {
    mockGetCharts.mockReset();
    __resetInfrastructureSummaryFetchesForTests();
    setOrgID('default');
    localStorage.clear();
  });

  it('rethrows when ChartsAPI rejects (covers .catch perf-log + rethrow)', async () => {
    mockGetCharts.mockRejectedValueOnce(new Error('boom'));
    await expect(fetchInfrastructureSummaryAndCache('1h')).rejects.toThrow('boom');
    // in-flight entry is cleared by .finally, so a second call issues a new request
    mockGetCharts.mockRejectedValueOnce(new Error('boom2'));
    await expect(fetchInfrastructureSummaryAndCache('1h')).rejects.toThrow('boom2');
    expect(mockGetCharts).toHaveBeenCalledTimes(2);
  });

  it('falls back to null oldestDataTimestamp when stats.oldestDataTimestamp is missing', async () => {
    const stats = {} as ChartStats;
    mockGetCharts.mockResolvedValueOnce(
      makeInfraResponse({ nodeData: { 'node-1': { cpu: makeSeries(1) } }, stats }),
    );
    const result = await fetchInfrastructureSummaryAndCache('1h');
    expect(result.oldestDataTimestamp).toBeNull();
    expect(result.map.get('node-1')?.cpu?.length).toBe(1);
  });

  it('falls back to null oldestDataTimestamp when stats.oldestDataTimestamp is non-numeric', async () => {
    const stats = { oldestDataTimestamp: 'nope' } as unknown as ChartStats;
    mockGetCharts.mockResolvedValueOnce(
      makeInfraResponse({ nodeData: { 'node-1': { cpu: makeSeries(1) } }, stats }),
    );
    const result = await fetchInfrastructureSummaryAndCache('1h');
    expect(result.oldestDataTimestamp).toBeNull();
  });

  it('defaults caller to unknown when options are omitted', async () => {
    mockGetCharts.mockResolvedValueOnce(makeInfraResponse());
    // No caller -> the `options?.caller || 'unknown'` expression takes its
    // 'unknown' arm; the call still resolves and persists a cache entry.
    await fetchInfrastructureSummaryAndCache('1h');
    expect(readInfrastructureSummaryCache('1h')).not.toBeNull();
  });
});
