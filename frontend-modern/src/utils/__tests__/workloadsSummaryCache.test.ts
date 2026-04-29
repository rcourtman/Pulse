import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { WorkloadChartsResponse } from '@/api/charts';
import {
  __resetWorkloadsSummaryCacheForTests,
  fetchWorkloadsSummaryAndCache,
  hasFreshWorkloadsSummaryCache,
  readInMemoryWorkloadsSummaryCache,
  readWorkloadsSummaryCache,
} from '@/utils/workloadsSummaryCache';

const mockGetWorkloadCharts = vi.fn();

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getWorkloadCharts: (...args: unknown[]) => mockGetWorkloadCharts(...args),
    },
  };
});

vi.mock('@/utils/apiClient', () => ({
  getOrgID: () => 'default',
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: vi.fn(() => () => {}),
  },
}));

const makeResponse = (): WorkloadChartsResponse => ({
  data: {
    'cluster-a:pve1:101': {
      cpu: [
        { timestamp: 1, value: 10 },
        { timestamp: 2, value: 20 },
      ],
      memory: [
        { timestamp: 1, value: 30 },
        { timestamp: 2, value: 40 },
      ],
      disk: [
        { timestamp: 1, value: 50 },
        { timestamp: 2, value: 60 },
      ],
      diskread: [
        { timestamp: 1, value: 70 },
        { timestamp: 2, value: 80 },
      ],
      diskwrite: [
        { timestamp: 1, value: 90 },
        { timestamp: 2, value: 100 },
      ],
      netin: [
        { timestamp: 1, value: 110 },
        { timestamp: 2, value: 120 },
      ],
      netout: [
        { timestamp: 1, value: 130 },
        { timestamp: 2, value: 140 },
      ],
    },
  },
  dockerData: {},
  guestTypes: {},
  timestamp: Date.now(),
  stats: {
    oldestDataTimestamp: 1,
  },
});

describe('workloadsSummaryCache', () => {
  beforeEach(() => {
    mockGetWorkloadCharts.mockReset();
    __resetWorkloadsSummaryCacheForTests();
    localStorage.clear();
  });

  it('fetches workload charts once and warms memory plus local cache', async () => {
    const response = makeResponse();
    mockGetWorkloadCharts.mockResolvedValueOnce(response);

    await fetchWorkloadsSummaryAndCache('1h', {
      maxPoints: 180,
    });

    expect(mockGetWorkloadCharts).toHaveBeenCalledWith('1h', undefined, {
      nodeId: undefined,
      maxPoints: 180,
    });
    expect(readInMemoryWorkloadsSummaryCache('1h')).toBe(response);
    expect(readWorkloadsSummaryCache('1h')).toEqual(
      expect.objectContaining({
        data: expect.objectContaining({
          'cluster-a:pve1:101': expect.any(Object),
        }),
      }),
    );
    expect(hasFreshWorkloadsSummaryCache('1h')).toBe(true);
  });

  it('deduplicates concurrent app-shell prewarm requests', async () => {
    const response = makeResponse();
    mockGetWorkloadCharts.mockResolvedValueOnce(response);

    const first = fetchWorkloadsSummaryAndCache('1h');
    const second = fetchWorkloadsSummaryAndCache('1h');

    await expect(Promise.all([first, second])).resolves.toEqual([response, response]);
    expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(1);
  });
});
