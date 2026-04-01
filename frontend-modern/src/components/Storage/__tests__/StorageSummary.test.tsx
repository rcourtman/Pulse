import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import StorageSummary from '../StorageSummary';

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
  }) => {
    const series = props.series ?? [];
    return (
      <div
        data-testid="sparkline"
        data-series-count={series.length}
        data-series-ids={series.map((current) => current.id || '').join('|')}
        data-highlight-series-id={props.highlightSeriesId || ''}
        data-interaction-state={props.interactionState || 'default'}
        data-active-series-display={props.activeSeriesDisplay || ''}
      />
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
});
