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
  InteractiveSparkline: (props: { series?: Array<{ id?: string; name?: string; data?: Array<unknown> }> }) => {
    const series = props.series ?? [];
    const maxPoints = series.reduce((max, current) => Math.max(max, current.data?.length ?? 0), 0);
    return (
      <div
        data-testid="sparkline"
        data-series-count={series.length}
        data-series-ids={series.map((current) => current.id || '').join('|')}
        data-series-names={series.map((current) => current.name || '').join('|')}
        data-max-points={maxPoints}
      />
    );
  },
}));

vi.mock('@/components/shared/DensityMap', () => ({
  DensityMap: (props: { series?: Array<{ id?: string; name?: string; data?: Array<unknown> }> }) => {
    const series = props.series ?? [];
    const maxPoints = series.reduce((max, current) => Math.max(max, current.data?.length ?? 0), 0);
    return (
      <div
        data-testid="sparkline"
        data-series-count={series.length}
        data-series-ids={series.map((current) => current.id || '').join('|')}
        data-series-names={series.map((current) => current.name || '').join('|')}
        data-max-points={maxPoints}
      />
    );
  },
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

const makeCacheKey = (range: string, nodeScope: string, orgScope = 'default') =>
  `pulse.workloadsSummaryCharts.${encodeURIComponent(orgScope)}::${range}::${encodeURIComponent(nodeScope || '__all__')}`;

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

    mockGetWorkloadCharts.mockImplementationOnce(() => new Promise(() => { }));

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

  it.each([
    { workloadCount: 79, expectedMaxPoints: 180 },
    { workloadCount: 80, expectedMaxPoints: 150 },
    { workloadCount: 120, expectedMaxPoints: 120 },
    { workloadCount: 250, expectedMaxPoints: 80 },
    { workloadCount: 400, expectedMaxPoints: 64 },
    { workloadCount: 600, expectedMaxPoints: 48 },
  ])(
    'uses adaptive maxPoints=$expectedMaxPoints for workloadCount=$workloadCount',
    async ({ workloadCount, expectedMaxPoints }) => {
      mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse([]));
      const snapshots = makeSnapshots(
        Array.from({ length: workloadCount }, (_, i) => `cluster-a:pve1:${1000 + i}`),
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
      expect(call[2]).toMatchObject({ maxPoints: expectedMaxPoints });
    },
  );

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
        expect(sparkline.getAttribute('data-series-ids')).toBe(ids[1]);
      }
    });

    const call = mockGetWorkloadCharts.mock.calls[0];
    expect(call[2]).toMatchObject({ maxPoints: 180 });
  });

  it('uses fallback snapshots to scope selected-node workload series', async () => {
    const chartIds = ['cluster-a:pve1:101', 'cluster-a:pve1:102', 'cluster-a:pve1:103'];
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse(chartIds));
    const scopedSnapshots = makeSnapshots(['cluster-a:pve1:101', 'cluster-a:pve1:102']);

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        selectedNodeId="pve1"
        fallbackGuestCounts={{ total: chartIds.length, running: chartIds.length, stopped: 0 }}
        fallbackSnapshots={scopedSnapshots}
      />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-series-count')).toBe('2');
      }
    });
  });

  it('applies fallback snapshot names after workload id normalization', async () => {
    const rawChartId = 'pve1-101';
    const normalizedSnapshotId = 'pve1:pve1:101';
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse([rawChartId]));

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        selectedNodeId="pve1"
        fallbackGuestCounts={{ total: 1, running: 1, stopped: 0 }}
        fallbackSnapshots={[
          {
            id: normalizedSnapshotId,
            name: 'Primary DB',
            cpu: 25,
            memory: 40,
            disk: 55,
            network: 2_048,
          },
        ]}
      />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-series-count')).toBe('1');
        expect(sparkline.getAttribute('data-series-names')).toContain('Primary DB');
      }
    });
  });

  it('keeps chart point requests bounded for very large workload sets', async () => {
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse([]));
    const snapshots = makeSnapshots(
      Array.from({ length: 1200 }, (_, i) => `cluster-a:pve1:${2000 + i}`),
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
    expect(call[2]).toMatchObject({ maxPoints: 48 });
  });
});
