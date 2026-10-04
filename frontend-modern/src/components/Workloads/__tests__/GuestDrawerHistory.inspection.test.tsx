import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
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
  },
): AllMetricsHistoryResponse => ({
  ...target,
  range: '24h',
  start: time,
  end: time + 120_000,
  source: 'store',
  metrics,
});
const inspect = () => screen.getByRole('slider', { name: 'Inspect Utilization history' });
const utilization = () => screen.getAllByTestId('guest-history-group-chart')[0];
const choose = (slider: HTMLElement, index: number) =>
  fireEvent.input(slider, { target: { value: String(index) } });

// Native keyboard/touch stepping is exercised in the offline browser; jsdom
// tests the renderer's input/focus contract rather than simulating browser defaults.
describe('GuestDrawerHistory stored-observation inspection', () => {
  it('exposes an ordered native control, dated values and chart description without another read', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => <GuestDrawerHistory target={target} range="24h" currentMetrics={{ disk: 88 }} />);
    await waitFor(() => expect(inspect()).toHaveAttribute('max', '3'));
    const slider = inspect();
    expect(slider).toHaveAttribute('type', 'range');
    expect(slider).toHaveAttribute('min', '0');
    expect(slider).toHaveAttribute('step', '1');
    expect(slider).toHaveValue('3');
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      `${new Date(time + 120_000).toLocaleString()}. CPU 43.0%. Memory 55.0%. Disk no observation.`,
    );
    expect(
      within(utilization()).getByRole('img', { name: 'Utilization history' }),
    ).toHaveAccessibleDescription(/4 stored observation times/);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('inspects exact stored times and does not borrow nearby or current values for missing series', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => (
      <GuestDrawerHistory
        target={target}
        range="24h"
        currentMetrics={{ cpu: 99, memory: 98, disk: 88 }}
      />
    ));
    await waitFor(() => expect(inspect()).toHaveValue('3'));
    const slider = inspect();
    slider.focus();
    choose(slider, 0);
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      `${new Date(time).toLocaleString()}. CPU 11.0%. Memory no observation. Disk no observation.`,
    );
    expect(utilization()).toHaveTextContent('CPU11.0%');
    expect(utilization()).toHaveTextContent('Memory-');
    expect(utilization()).not.toHaveTextContent('88.0%');
    choose(slider, 1);
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      `${new Date(time + 30_000).toLocaleString()}. CPU no observation. Memory 30.0%. Disk no observation.`,
    );
    expect(utilization()).toHaveTextContent('CPU-');
    expect(utilization()).toHaveTextContent('Memory30.0%');
    expect(slider).toHaveFocus();
  });

  it('keeps focused selection independent of plot pointer movement, then restores normal hover on blur', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await waitFor(() => expect(inspect()).toHaveValue('3'));
    const slider = inspect();
    const plot = within(utilization()).getByTestId('guest-history-plot');
    vi.spyOn(plot, 'getBoundingClientRect').mockReturnValue({ left: 0, width: 360 } as DOMRect);
    slider.focus();
    choose(slider, 0);
    fireEvent.pointerMove(plot, { clientX: 352 });
    fireEvent.pointerLeave(plot);
    expect(utilization()).toHaveTextContent('CPU11.0%');
    fireEvent.blur(slider);
    expect(utilization()).toHaveTextContent('CPU43.0%');
    fireEvent.mouseMove(plot, { clientX: 34 });
    expect(utilization()).toHaveTextContent('CPU11.0%');
  });

  it('preserves the selected stored timestamp, not its index, when a poll adds observations', async () => {
    vi.useFakeTimers();
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockResolvedValueOnce(
        response({
          cpu: [
            point(-60_000, 8),
            point(0, 11),
            point(60_000, 22),
            point(120_000, 43),
            point(180_000, 66),
          ],
          memory: [point(30_000, 30), point(120_000, 55)],
        }),
      );
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await vi.advanceTimersByTimeAsync(0);
    const slider = inspect();
    slider.focus();
    choose(slider, 2);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(inspect()).toBe(slider);
    expect(slider).toHaveFocus();
    expect(slider).toHaveValue('3');
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      `${new Date(time + 60_000).toLocaleString()}. CPU 22.0%. Memory no observation. Disk no observation.`,
    );
    expect(utilization()).toHaveTextContent('CPU22.0%');
  });

  it('snaps an expired inspected timestamp to an actual remaining observation on refresh', async () => {
    vi.useFakeTimers();
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockResolvedValueOnce(response({ cpu: [point(120_000, 43), point(180_000, 66)] }));
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await vi.advanceTimersByTimeAsync(0);
    const slider = inspect();
    slider.focus();
    choose(slider, 0);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(slider).toHaveValue('0');
    expect(slider.getAttribute('aria-valuetext')).toContain(
      new Date(time + 120_000).toLocaleString(),
    );
    expect(utilization()).toHaveTextContent('CPU43.0%');
  });

  it.each(['target', 'resource type', 'range'] as const)(
    'clears selection on a cached %s replacement even when its timestamps coincide',
    async (change) => {
      vi.spyOn(ChartsAPI, 'getMetricsHistory')
        .mockResolvedValueOnce(response())
        .mockResolvedValueOnce(response({ cpu: [point(0, 77), point(120_000, 88)] }))
        .mockImplementation(() => new Promise(() => {}));
      const [current, setTarget] = createSignal(target);
      const [range, setRange] = createSignal<HistoryTimeRange>('24h');
      render(() => <GuestDrawerHistory target={current()} range={range()} />);
      await waitFor(() => expect(inspect()).toHaveValue('3'));
      if (change === 'target') setTarget({ ...target, resourceId: 'other-pbs' });
      else if (change === 'resource type') setTarget({ ...target, resourceType: 'vm' });
      else setRange('6h');
      await waitFor(() => expect(inspect()).toHaveValue('1'));
      inspect().focus();
      choose(inspect(), 0);
      expect(utilization()).toHaveTextContent('CPU77.0%');
      if (change !== 'range') setTarget(target);
      else setRange('24h');
      await waitFor(() => expect(inspect()).toHaveValue('3'));
      expect(utilization()).toHaveTextContent('CPU43.0%');
      expect(
        within(utilization()).queryByTestId('guest-history-hover-time'),
      ).not.toBeInTheDocument();
    },
  );

  it('inspects zero throughput and temperature using the existing host metric units', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        netin: [point(0, 0), point(60_000, 2048)],
        temperature: [point(0, 42), point(60_000, 57)],
      }),
    );
    render(() => (
      <GuestDrawerHistory target={target} range="24h" groups={HOST_METRICS_HISTORY_GROUPS} />
    ));
    const network = await screen.findByRole('slider', { name: 'Inspect Network I/O history' });
    network.focus();
    choose(network, 0);
    expect(network.getAttribute('aria-valuetext')).toContain('In 0 B/s. Out no observation.');
    const thermal = screen.getByRole('slider', { name: 'Inspect Thermals history' });
    thermal.focus();
    choose(thermal, 1);
    expect(thermal.getAttribute('aria-valuetext')).toContain('CPU 57°C.');
  });

  it('keeps stored observations inspectable after a failed poll without exposing the error', async () => {
    vi.useFakeTimers();
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockRejectedValueOnce(new Error('private backend detail'));
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(screen.getByRole('status', { name: 'History refresh status' })).toHaveTextContent(
      'Showing previously loaded history.',
    );
    inspect().focus();
    choose(inspect(), 0);
    expect(utilization()).toHaveTextContent('CPU11.0%');
    expect(utilization()).not.toHaveTextContent('private backend detail');
  });

  it('describes a lone stored observation without manufacturing a trend or pointless slider', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response({ cpu: [point(0, 11)] }));
    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await waitFor(() =>
      expect(within(utilization()).getByRole('img')).toHaveAccessibleDescription(
        /1 stored observation time/,
      ),
    );
    expect(within(utilization()).getByRole('img')).toHaveAccessibleDescription(
      `1 stored observation time. ${new Date(time).toLocaleString()}. CPU 11.0%. Memory no observation. Disk no observation.`,
    );
    expect(screen.queryByRole('slider')).not.toBeInTheDocument();
    expect(utilization().querySelector('path')).toBeNull();
  });

  it.each(['empty', 'invalid'] as const)(
    'does not offer inspection for %s stored history or live readings',
    async (kind) => {
      vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
        response(kind === 'empty' ? {} : { cpu: [point(0, NaN)] }),
      );
      render(() => <GuestDrawerHistory target={target} range="24h" currentMetrics={{ cpu: 42 }} />);
      await waitFor(() => expect(screen.queryByText('Loading history')).not.toBeInTheDocument());
      expect(screen.queryByRole('slider')).not.toBeInTheDocument();
      expect(within(utilization()).getByRole('img')).toHaveAccessibleDescription(
        'No stored history observations.',
      );
    },
  );

  it('offers no inspection or reads for an absent target or locked range', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory');
    const [current, setTarget] = createSignal<GuestDrawerHistoryTarget | null>(null);
    render(() => <GuestDrawerHistory target={current()} range="14d" />);
    expect(screen.queryByRole('slider')).not.toBeInTheDocument();
    setTarget(target);
    await screen.findByText(/14 days history requires/);
    expect(screen.queryByRole('slider')).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });
});
