import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import { formatHistoryChartTimeLabel } from '@/components/shared/historyChartModel';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import type { GuestDrawerHistoryTarget } from '../guestDrawerModel';

vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(async () => undefined),
  maxHistoryDays: () => 7,
  isRangeLocked: (range: HistoryTimeRange) => ['14d', '30d', '90d'].includes(range),
}));

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

const time = 1_700_000_000_000;
const target: GuestDrawerHistoryTarget = { resourceType: 'agent', resourceId: 'pbs-host' };
const point = (offset: number, value: number) => ({
  timestamp: time + offset,
  value,
  min: value,
  max: value,
});
const response = (
  metrics: AllMetricsHistoryResponse['metrics'] = {
    cpu: [point(120_000, 43), point(0, 11), point(60_000, 22)],
    memory: [point(30_000, 30), point(120_000, 55)],
    disk: [point(45_000, 0)],
  },
): AllMetricsHistoryResponse => ({
  ...target,
  range: '24h',
  start: time,
  end: time + 120_000,
  source: 'store',
  metrics,
});
const utilization = () => screen.getAllByTestId('guest-history-group-chart')[0];
const hover = (chart: HTMLElement, offset: number, duration = 120_000) => {
  const plot = within(chart).getByTestId('guest-history-plot');
  vi.spyOn(plot, 'getBoundingClientRect').mockReturnValue({ left: 0, width: 360 } as DOMRect);
  fireEvent.mouseMove(plot, { clientX: 34 + (offset / duration) * 318 });
  return plot;
};

describe('GuestDrawerHistory common-time pointer inspection', () => {
  it('chooses the nearest actual observation across all series, not the first metric', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    hover(utilization(), 30_000);
    expect(within(utilization()).getByTestId('guest-history-hover-time')).toHaveTextContent(
      formatHistoryChartTimeLabel(time + 30_000, '24h'),
    );
    expect(utilization()).toHaveTextContent('CPU-');
    expect(utilization()).toHaveTextContent('Memory30.0%');
    expect(utilization().querySelectorAll('circle[r="3"]')).toHaveLength(1);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('does not borrow neighbouring or current readings for a missing metric at the chosen time', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => (
      <GuestDrawerHistory
        target={target}
        range="24h"
        currentMetrics={{ cpu: 99, memory: 98, disk: 88 }}
      />
    ));
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const plot = hover(utilization(), 0);
    expect(utilization()).toHaveTextContent('CPU11.0%');
    expect(utilization()).toHaveTextContent('Memory-');
    expect(utilization()).toHaveTextContent('Disk-');
    expect(utilization()).not.toHaveTextContent('98.0%');
    expect(
      within(utilization()).getByRole('img', { name: 'Utilization history' }),
    ).toHaveAccessibleDescription(
      `${new Date(time).toLocaleString()}. CPU 11.0%. Memory no observation. Disk no observation.`,
    );
    fireEvent.pointerLeave(plot);
    expect(utilization()).toHaveTextContent('CPU43.0%');
    expect(utilization()).toHaveTextContent('Memory55.0%');
    expect(within(utilization()).queryByTestId('guest-history-hover-time')).toBeNull();
  });

  it('can inspect a lone zero observation without drawing a synthetic trend', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    hover(utilization(), 45_000);
    expect(utilization()).toHaveTextContent('CPU-');
    expect(utilization()).toHaveTextContent('Memory-');
    expect(utilization()).toHaveTextContent('Disk0.0%');
    expect(utilization().querySelectorAll('circle[r="3"]')).toHaveLength(1);
    expect(utilization().querySelectorAll('path')).toHaveLength(2);
  });

  it('renders all metrics that really share a timestamp at the same cursor position', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    hover(utilization(), 120_000);
    expect(utilization()).toHaveTextContent('CPU43.0%');
    expect(utilization()).toHaveTextContent('Memory55.0%');
    expect(utilization()).toHaveTextContent('Disk-');
    const markers = [...utilization().querySelectorAll('circle[r="3"]')];
    expect(markers).toHaveLength(2);
    expect(markers[0].getAttribute('cx')).toBe(markers[1].getAttribute('cx'));
  });

  it('snaps between observations and resolves a tie to the earlier actual time', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    hover(utilization(), 37_500);
    expect(utilization()).toHaveTextContent('CPU-');
    expect(utilization()).toHaveTextContent('Memory30.0%');
    expect(utilization()).toHaveTextContent('Disk-');
  });

  it('keeps keyboard selection authoritative and restores pointer inspection after blur', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    const slider = await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    slider.focus();
    fireEvent.input(slider, { target: { value: '0' } });
    hover(utilization(), 30_000);
    expect(utilization()).toHaveTextContent('CPU11.0%');
    expect(utilization()).toHaveTextContent('Memory-');
    fireEvent.blur(slider);
    hover(utilization(), 30_000);
    expect(utilization()).toHaveTextContent('CPU-');
    expect(utilization()).toHaveTextContent('Memory30.0%');
  });

  it('reconciles a hovered time with a refreshed series instead of substituting every latest metric', async () => {
    vi.useFakeTimers();
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockResolvedValueOnce(response({ memory: [point(30_000, 31)], cpu: [point(120_000, 88)] }));
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await vi.advanceTimersByTimeAsync(0);
    hover(utilization(), 30_000);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(utilization()).toHaveTextContent('CPU-');
    expect(utilization()).toHaveTextContent('Memory31.0%');
    expect(utilization().querySelectorAll('path')).toHaveLength(0);
    expect(utilization().querySelectorAll('circle[r="3"]')).toHaveLength(1);
  });

  it.each(['target', 'range'] as const)(
    'clears pointer state on %s replacement',
    async (change) => {
      vi.spyOn(ChartsAPI, 'getMetricsHistory')
        .mockResolvedValueOnce(response())
        .mockImplementation(() => new Promise(() => {}));
      const [current, setTarget] = createSignal(target);
      const [range, setRange] = createSignal<HistoryTimeRange>('24h');
      render(() => <GuestDrawerHistory target={current()} range={range()} />);
      await screen.findByRole('slider', { name: 'Inspect Utilization history' });
      hover(utilization(), 30_000);
      if (change === 'target') setTarget({ ...target, resourceId: 'replacement' });
      else setRange('6h');
      await waitFor(() =>
        expect(within(utilization()).queryByTestId('guest-history-hover-time')).toBeNull(),
      );
      expect(utilization().querySelectorAll('circle[r="3"]')).toHaveLength(0);
    },
  );

  it('inspects real zero rates and temperatures while leaving missing sibling directions unavailable', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        netin: [point(0, 0), point(120_000, 2048)],
        netout: [point(30_000, 4096)],
        temperature: [point(0, 42), point(120_000, 57)],
      }),
    );
    render(() => (
      <GuestDrawerHistory target={target} range="24h" groups={HOST_METRICS_HISTORY_GROUPS} />
    ));
    await screen.findByRole('slider', { name: 'Inspect Network I/O history' });
    const groups = screen.getAllByTestId('guest-history-group-chart');
    const network = groups.find((group) => group.dataset.historyGroup === 'network')!;
    hover(network, 0);
    expect(network).toHaveTextContent('In0 B/s');
    expect(network).toHaveTextContent('Out-');
    hover(network, 30_000);
    expect(network).toHaveTextContent('In-');
    expect(network).toHaveTextContent('Out4.00 KB/s');
    const thermals = groups.find((group) => group.dataset.historyGroup === 'thermals')!;
    hover(thermals, 0);
    expect(thermals).toHaveTextContent('CPU42°C');
  });
});
