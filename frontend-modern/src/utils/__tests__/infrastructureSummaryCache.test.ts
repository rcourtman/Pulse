import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { TimeRange } from '@/api/charts';
import {
  __resetInfrastructureSummaryFetchesForTests,
  extractInfrastructureSummaryChartMapFromInfrastructureResponse,
  fetchInfrastructureSummaryAndCache,
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

describe('infrastructureSummaryCache fetch dedupe', () => {
  beforeEach(() => {
    mockGetCharts.mockReset();
    __resetInfrastructureSummaryFetchesForTests();
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
