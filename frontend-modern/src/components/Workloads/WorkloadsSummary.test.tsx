import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  WorkloadsSummary,
  __resetInMemoryWorkloadCacheForTests,
  type WorkloadSummarySnapshot,
} from './WorkloadsSummary';
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
  InteractiveSparkline: (props: {
    series?: Array<{ id?: string; name?: string; data?: Array<unknown> }>;
    highlightSeriesId?: string | null;
    interactionState?: string;
    activeSeriesDisplay?: string;
    hoverSourceKey?: string;
    hoverSync?: { seriesId: string; timestamp?: number } | null;
    onHoverSyncChange?: (
      value: {
        sourceKey: string;
        seriesId: string;
        timestamp: number;
      } | null,
    ) => void;
  }) => {
    const series = props.series ?? [];
    const maxPoints = series.reduce((max, current) => Math.max(max, current.data?.length ?? 0), 0);
    const triggerHover = () => {
      const seriesId = series[0]?.id;
      if (!props.onHoverSyncChange || !seriesId || !props.hoverSourceKey) {
        return;
      }
      props.onHoverSyncChange({
        sourceKey: props.hoverSourceKey,
        seriesId,
        timestamp: Date.now(),
      });
    };
    return (
      <>
        <button
          type="button"
          data-testid={`chart-hover-${props.hoverSourceKey || 'unknown'}`}
          onClick={triggerHover}
        />
        <div
          data-testid="sparkline"
          data-series-count={series.length}
          data-series-ids={series.map((current) => current.id || '').join('|')}
          data-series-names={series.map((current) => current.name || '').join('|')}
          data-max-points={maxPoints}
          data-highlight-series-id={props.highlightSeriesId || ''}
          data-hover-source-key={props.hoverSourceKey || ''}
          data-hover-sync-series-id={props.hoverSync?.seriesId || ''}
          data-hover-sync-timestamp={
            props.hoverSync?.timestamp ? String(props.hoverSync.timestamp) : ''
          }
          data-interaction-state={props.interactionState || 'default'}
          data-active-series-display={props.activeSeriesDisplay || ''}
        />
      </>
    );
  },
}));

vi.mock('@/components/shared/DensityMap', () => ({
  DensityMap: (props: {
    series?: Array<{ id?: string; name?: string; data?: Array<unknown> }>;
    highlightSeriesId?: string | null;
    interactionState?: string;
    hoverSourceKey?: string;
    hoverSync?: { seriesId: string; timestamp?: number } | null;
    onHoverSyncChange?: (
      value: {
        sourceKey: string;
        seriesId: string;
        timestamp: number;
      } | null,
    ) => void;
  }) => {
    const series = props.series ?? [];
    const maxPoints = series.reduce((max, current) => Math.max(max, current.data?.length ?? 0), 0);
    const triggerHover = () => {
      const seriesId = series[0]?.id;
      if (!props.onHoverSyncChange || !seriesId || !props.hoverSourceKey) {
        return;
      }
      props.onHoverSyncChange({
        sourceKey: props.hoverSourceKey,
        seriesId,
        timestamp: Date.now(),
      });
    };
    return (
      <>
        <button
          type="button"
          data-testid={`chart-hover-${props.hoverSourceKey || 'unknown'}`}
          onClick={triggerHover}
        />
        <div
          data-testid="sparkline"
          data-series-count={series.length}
          data-series-ids={series.map((current) => current.id || '').join('|')}
          data-series-names={series.map((current) => current.name || '').join('|')}
          data-max-points={maxPoints}
          data-highlight-series-id={props.highlightSeriesId || ''}
          data-hover-source-key={props.hoverSourceKey || ''}
          data-hover-sync-series-id={props.hoverSync?.seriesId || ''}
          data-hover-sync-timestamp={
            props.hoverSync?.timestamp ? String(props.hoverSync.timestamp) : ''
          }
          data-interaction-state={props.interactionState || 'default'}
        />
      </>
    );
  },
}));

const now = Date.now();
const singlePointSeries = [{ timestamp: now, value: 12 }];
const twoPointSeries = [
  { timestamp: now - 60_000, value: 12 },
  { timestamp: now, value: 18 },
];

const makeChartsResponse = (ids: string[]): WorkloadChartsResponse => ({
  data: Object.fromEntries(
    ids.map((id) => [
      id,
      {
        cpu: twoPointSeries,
        memory: twoPointSeries,
        disk: twoPointSeries,
        diskread: twoPointSeries,
        diskwrite: twoPointSeries,
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

describe('WorkloadsSummary performance behavior', () => {
  beforeEach(() => {
    mockGetWorkloadCharts.mockReset();
    __resetInMemoryWorkloadCacheForTests();
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('hydrates from cache immediately while live fetch is pending', async () => {
    const workloadId = 'cluster-a:pve1:101';

    // First render: fetch succeeds, populating the in-memory cache.
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse([workloadId]));
    const { unmount } = render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: 1, running: 1, stopped: 0 }}
        fallbackSnapshots={makeSnapshots([workloadId])}
      />
    ));
    await waitFor(() => {
      expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(1);
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
    });
    unmount();

    // Second render: fetch hangs, but in-memory cache provides instant data.
    mockGetWorkloadCharts.mockImplementationOnce(() => new Promise(() => {}));
    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: 1, running: 1, stopped: 0 }}
        fallbackSnapshots={makeSnapshots([workloadId])}
      />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      for (const sparkline of sparklines) {
        expect(Number(sparkline.getAttribute('data-series-count'))).toBeGreaterThan(0);
      }
    });
  });

  it('ignores and purges stale cache versions on failed live fetch', async () => {
    const staleWorkloadId = 'cluster-a:pve1:stale';
    const cacheKey = 'pulse.workloadsSummaryCharts.default::1h::__all__';
    localStorage.setItem(
      cacheKey,
      JSON.stringify({
        version: 0,
        range: '1h',
        nodeScope: '',
        cachedAt: Date.now(),
        data: makeChartsResponse([staleWorkloadId]).data,
        dockerData: {},
      }),
    );
    mockGetWorkloadCharts.mockRejectedValueOnce(new Error('network down'));

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: 1, running: 1, stopped: 0 }}
        fallbackSnapshots={makeSnapshots(['cluster-a:pve1:fresh'])}
      />
    ));

    await waitFor(() => {
      expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(screen.queryByTestId('sparkline')).not.toBeInTheDocument();
      expect(screen.getAllByText('Trend data unavailable')).toHaveLength(4);
      expect(localStorage.getItem(cacheKey)).toBeNull();
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

  it('falls back to the focused workload id for summary chart emphasis', async () => {
    const workloadId = 'cluster-a:pve1:101';
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse([workloadId]));

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: 1, running: 1, stopped: 0 }}
        fallbackSnapshots={makeSnapshots([workloadId])}
        focusedWorkloadId={workloadId}
      />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-highlight-series-id')).toBe(workloadId);
        expect(sparkline.getAttribute('data-interaction-state')).toBe('active');
      }
    });
  });

  it('promotes chart hover into shared summary focus across all workload cards', async () => {
    const workloadIds = ['cluster-a:pve1:101', 'cluster-a:pve1:102'];
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse(workloadIds));

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: workloadIds.length, running: workloadIds.length, stopped: 0 }}
        fallbackSnapshots={makeSnapshots(workloadIds)}
      />
    ));

    await waitFor(() => {
      expect(screen.getAllByTestId('sparkline')).toHaveLength(4);
    });

    screen.getByTestId('chart-hover-cpu').click();

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      const timestamps = new Set(
        sparklines.map((sparkline) => sparkline.getAttribute('data-hover-sync-timestamp')),
      );
      expect(timestamps.size).toBe(1);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-highlight-series-id')).toBe(workloadIds[0]);
        expect(sparkline.getAttribute('data-hover-sync-series-id')).toBe(workloadIds[0]);
        expect(sparkline.getAttribute('data-hover-sync-timestamp')).not.toBe('');
        expect(sparkline.getAttribute('data-interaction-state')).toBe('active');
      }
    });
  });

  it('ignores focused workload ids that do not have interactive history', async () => {
    const interactiveWorkloadId = 'cluster-a:pve1:101';
    const nonInteractiveWorkloadId = 'cluster-a:pve1:102';
    mockGetWorkloadCharts.mockResolvedValueOnce({
      data: {
        [interactiveWorkloadId]: {
          cpu: twoPointSeries,
          memory: twoPointSeries,
          disk: twoPointSeries,
          diskread: twoPointSeries,
          diskwrite: twoPointSeries,
          netin: twoPointSeries,
          netout: twoPointSeries,
        },
        [nonInteractiveWorkloadId]: {
          cpu: singlePointSeries,
          memory: singlePointSeries,
          disk: singlePointSeries,
          diskread: singlePointSeries,
          diskwrite: singlePointSeries,
          netin: singlePointSeries,
          netout: singlePointSeries,
        },
      },
      dockerData: {},
      guestTypes: {},
      timestamp: now,
      stats: {
        oldestDataTimestamp: now - 60_000,
      },
    });

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: 2, running: 2, stopped: 0 }}
        fallbackSnapshots={makeSnapshots([interactiveWorkloadId, nonInteractiveWorkloadId])}
        focusedWorkloadId={nonInteractiveWorkloadId}
      />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-highlight-series-id')).toBe('');
        expect(sparkline.getAttribute('data-interaction-state')).toBe('default');
      }
    });
  });

  it('keeps the page summary series page-scoped when a focused workload is selected', async () => {
    const workloadIds = ['cluster-a:pve1:101', 'cluster-a:pve1:102'];
    mockGetWorkloadCharts.mockResolvedValueOnce(makeChartsResponse(workloadIds));

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: workloadIds.length, running: workloadIds.length, stopped: 0 }}
        fallbackSnapshots={makeSnapshots(workloadIds)}
        focusedWorkloadId={workloadIds[0]}
      />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(4);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-series-count')).toBe('2');
        expect(sparkline.getAttribute('data-highlight-series-id')).toBe(workloadIds[0]);
      }
      const isolated = sparklines.filter(
        (sparkline) => sparkline.getAttribute('data-active-series-display') === 'isolate',
      );
      expect(isolated).toHaveLength(2);
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

  it('renders a deliberate jump affordance when the active workload row is off-screen', () => {
    const onJumpToActiveRow = vi.fn();

    render(() => (
      <WorkloadsSummary
        timeRange="1h"
        fallbackGuestCounts={{ total: 0, running: 0, stopped: 0 }}
        fallbackSnapshots={[]}
        showJumpToActiveRow
        onJumpToActiveRow={onJumpToActiveRow}
      />
    ));

    screen.getByRole('button', { name: 'Jump to row' }).click();

    expect(onJumpToActiveRow).toHaveBeenCalledTimes(1);
  });
});
