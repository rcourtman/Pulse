import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import StorageSummary from '../StorageSummary';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';

const mockGetStorageSummaryCharts = vi.fn();

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getStorageSummaryCharts: (...args: unknown[]) => mockGetStorageSummaryCharts(...args),
    },
  };
});

vi.mock('@/utils/apiClient', async () => {
  const actual = await vi.importActual<typeof import('@/utils/apiClient')>('@/utils/apiClient');
  return {
    ...actual,
    getOrgID: () => 'default',
  };
});

vi.mock('@/components/shared/InteractiveSparkline', () => ({
  InteractiveSparkline: (props: {
    series?: Array<{ id?: string; name?: string; data?: Array<unknown> }>;
    highlightSeriesId?: string | null;
    interactionState?: string;
    activeSeriesDisplay?: string;
    hoverSourceKey?: string;
    hoverSync?: { seriesId: string } | null;
    onHoverSyncChange?: (value: {
      sourceKey: string;
      seriesId: string;
      timestamp: number;
    } | null) => void;
  }) => {
    const series = props.series ?? [];
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
          data-highlight-series-id={props.highlightSeriesId || ''}
          data-hover-source-key={props.hoverSourceKey || ''}
          data-hover-sync-series-id={props.hoverSync?.seriesId || ''}
          data-interaction-state={props.interactionState || 'default'}
          data-active-series-display={props.activeSeriesDisplay || ''}
        />
      </>
    );
  },
}));

describe('StorageSummary', () => {
  const now = Date.now();
  const twoPointSeries = [
    { timestamp: now - 60_000, value: 45 },
    { timestamp: now, value: 47 },
  ];

  beforeEach(() => {
    mockGetStorageSummaryCharts.mockReset();
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps storage summary series page-scoped when a focused resource is selected', async () => {
    mockGetStorageSummaryCharts.mockResolvedValueOnce({
      pools: {
        'pool:alpha': {
          name: 'Alpha Pool',
          usage: twoPointSeries,
          used: twoPointSeries,
          avail: twoPointSeries,
        },
        'pool:beta': {
          name: 'Beta Pool',
          usage: twoPointSeries,
          used: twoPointSeries,
          avail: twoPointSeries,
        },
      },
      disks: {},
      stats: {
        oldestDataTimestamp: now - 60_000,
      },
    });

    render(() => (
      <StorageSummary poolCount={2} diskCount={0} timeRange="1h" focusedResourceId="pool:alpha" />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(3);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-series-count')).toBe('2');
        expect(sparkline.getAttribute('data-highlight-series-id')).toBe('pool:alpha');
        expect(sparkline.getAttribute('data-active-series-display')).toBe('isolate');
      }
    });
  });

  it('treats chart hover as the shared active storage entity across cards', async () => {
    mockGetStorageSummaryCharts.mockResolvedValueOnce({
      pools: {
        'pool:alpha': {
          name: 'Alpha Pool',
          usage: twoPointSeries,
          used: twoPointSeries,
          avail: twoPointSeries,
        },
        'pool:beta': {
          name: 'Beta Pool',
          usage: twoPointSeries,
          used: twoPointSeries,
          avail: twoPointSeries,
        },
      },
      disks: {
        'disk:serial-1': {
          name: 'Disk 1',
          temperature: twoPointSeries,
        },
      },
      stats: {
        oldestDataTimestamp: now - 60_000,
      },
    });

    render(() => <StorageSummary poolCount={2} diskCount={1} timeRange="1h" />);

    await waitFor(() => {
      expect(screen.getAllByTestId('sparkline')).toHaveLength(4);
    });

    screen.getByTestId('chart-hover-pool-usage').click();

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      const poolCards = sparklines.filter((sparkline) =>
        ['pool-usage', 'used-capacity', 'available-space'].includes(
          sparkline.getAttribute('data-hover-source-key') || '',
        ),
      );
      const diskTempCard = sparklines.find(
        (sparkline) => sparkline.getAttribute('data-hover-source-key') === 'disk-temperature',
      );

      expect(poolCards).toHaveLength(3);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-highlight-series-id')).toBe('pool:alpha');
        expect(sparkline.getAttribute('data-hover-sync-series-id')).toBe('pool:alpha');
      }
      for (const sparkline of poolCards) {
        expect(sparkline.getAttribute('data-interaction-state')).toBe('active');
      }
      expect(diskTempCard?.getAttribute('data-interaction-state')).toBe('inactive');
    });
  });

  it('filters pool summary cards to the hovered storage group scope', async () => {
    const poolIds = ['pool:alpha', 'pool:beta', 'pool:gamma'];
    const hoveredGroupScope: SummarySeriesGroupScope = {
      id: 'storage:node:pve1',
      label: 'pve1 (2 pools)',
      seriesIds: poolIds.slice(0, 2),
    };
    mockGetStorageSummaryCharts.mockResolvedValueOnce({
      pools: {
        'pool:alpha': {
          name: 'Alpha Pool',
          usage: twoPointSeries,
          used: twoPointSeries,
          avail: twoPointSeries,
        },
        'pool:beta': {
          name: 'Beta Pool',
          usage: twoPointSeries,
          used: twoPointSeries,
          avail: twoPointSeries,
        },
        'pool:gamma': {
          name: 'Gamma Pool',
          usage: twoPointSeries,
          used: twoPointSeries,
          avail: twoPointSeries,
        },
      },
      disks: {},
      stats: {
        oldestDataTimestamp: now - 60_000,
      },
    });

    render(() => (
      <StorageSummary
        poolCount={3}
        diskCount={0}
        timeRange="1h"
        hoveredGroupScope={hoveredGroupScope}
        hoveredResourceId="pool:alpha"
      />
    ));

    await waitFor(() => {
      const sparklines = screen.getAllByTestId('sparkline');
      expect(sparklines).toHaveLength(3);
      for (const sparkline of sparklines) {
        expect(sparkline.getAttribute('data-series-count')).toBe('2');
        expect(sparkline.getAttribute('data-series-ids')).toBe('pool:alpha|pool:beta');
        expect(sparkline.getAttribute('data-highlight-series-id')).toBe('pool:alpha');
        expect(sparkline.getAttribute('data-interaction-state')).toBe('active');
      }
    });
  });

});
