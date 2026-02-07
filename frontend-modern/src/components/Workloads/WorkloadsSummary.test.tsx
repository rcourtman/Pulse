import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { WorkloadsSummary, type WorkloadSummarySnapshot } from './WorkloadsSummary';
import type { WorkloadChartsResponse } from '@/api/charts';

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

vi.mock('@/components/shared/InteractiveSparkline', () => ({
  InteractiveSparkline: (props: { series?: Array<unknown> }) => (
    <div data-testid="sparkline" data-series-count={props.series?.length ?? 0} />
  ),
}));

const now = Date.now();
const twoPointSeries = [
  { timestamp: now - 60_000, value: 12 },
  { timestamp: now, value: 18 },
];

const makeChartsResponse = (
  ids: string[],
): WorkloadChartsResponse => ({
  data: Object.fromEntries(
    ids.map((id) => [
      id,
      {
        cpu: twoPointSeries,
        memory: twoPointSeries,
        disk: twoPointSeries,
        netin: twoPointSeries,
        netout: twoPointSeries,
      },
    ]),
  ),
  dockerData: {},
  guestTypes: {},
  timestamp: now,
  stats: {
    oldestDataTimestamp: now - 60_000,
  },
});

const makeSnapshots = (ids: string[]): WorkloadSummarySnapshot[] =>
  ids.map((id) => ({
    id,
    name: id,
    cpu: 25,
    memory: 50,
    disk: 40,
    network: 1_000,
  }));

const makeCacheKey = (range: string, nodeScope: string) =>
  `pulse.workloadsSummaryCharts.${range}::${encodeURIComponent(nodeScope || '__all__')}`;

describe('WorkloadsSummary performance behavior', () => {
  beforeEach(() => {
    mockGetWorkloadCharts.mockReset();
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('hydrates from cache immediately while live fetch is pending', async () => {
    const workloadId = 'cluster-a:pve1:101';
    const cachePayload = {
      version: 2,
      range: '1h',
      nodeScope: '',
      cachedAt: now,
      data: {
        [workloadId]: {
          cpu: twoPointSeries,
          memory: twoPointSeries,
          disk: twoPointSeries,
          netin: twoPointSeries,
          netout: twoPointSeries,
        },
      },
      dockerData: {},
    };
    localStorage.setItem(makeCacheKey('1h', ''), JSON.stringify(cachePayload));

    mockGetWorkloadCharts.mockImplementationOnce(() => new Promise(() => {}));

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: 1, running: 1, stopped: 0 }}
        fallbackSnapshots={makeSnapshots([workloadId])}
      />
    ));

    await waitFor(() => {
      expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      for (const sparkline of sparklines) {
        expect(Number(sparkline.getAttribute('data-series-count'))).toBeGreaterThan(0);
      }
    });
  });

  it('requests fewer chart points for large workload sets', async () => {
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse([]));

    const snapshots = makeSnapshots(
      Array.from({ length: 450 }, (_, i) => `cluster-a:pve1:${1000 + i}`),
    );

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: snapshots.length, running: snapshots.length, stopped: 0 }}
        fallbackSnapshots={snapshots}
      />
    ));

    await waitFor(() => {
      expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(1);
    });

    const call = mockGetWorkloadCharts.mock.calls[0];
    expect(call[0]).toBe('1h');
    expect(call[2]).toMatchObject({ maxPoints: 64 });
  });

  it('renders only visible workload series to reduce sparkline work', async () => {
    const ids = ['cluster-a:pve1:101', 'cluster-a:pve1:102', 'cluster-a:pve1:103'];
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse(ids));

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: ids.length, running: ids.length, stopped: 0 }}
        fallbackSnapshots={makeSnapshots(ids)}
        visibleWorkloadIds={[ids[1]]}
      />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-series-count')).toBe('1');
      }
    });
  });
});
