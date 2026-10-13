import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import {
  buildGuestDrawerHistoryBandPath,
  type GuestDrawerHistoryTarget,
} from '../guestDrawerModel';

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
    expect(utilization()).toHaveTextContent('Memoryno reading');
    expect(utilization()).not.toHaveTextContent('88.0%');
    choose(slider, 1);
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      `${new Date(time + 30_000).toLocaleString()}. CPU no observation. Memory 30.0%. Disk no observation.`,
    );
    expect(utilization()).toHaveTextContent('CPUno reading');
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
      const restoredTime = within(utilization()).getByTestId('guest-history-hover-time');
      expect(restoredTime).toHaveTextContent('Latest stored');
      expect(restoredTime).toHaveAttribute('datetime', new Date(time + 120_000).toISOString());
      expect(utilization().querySelectorAll('[data-history-hover-marker]')).toHaveLength(0);
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
    expect(utilization().querySelector('path[data-history-series]')).toBeNull();
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

// Min-max band: each stored point is a bucket average carrying its extremes.
const bandStart = Date.UTC(2026, 9, 12, 12);
const bandTarget = { resourceType: 'vm' as const, resourceId: 'fixture:pve:101' };
const bandPoint = (minute: number, value: number, min = value, max = value) => ({
  timestamp: bandStart + minute * 60_000,
  value,
  min,
  max,
});
const bandResponse = (
  metrics: AllMetricsHistoryResponse['metrics'],
): AllMetricsHistoryResponse => ({
  ...bandTarget,
  range: '1h',
  start: bandStart,
  end: bandStart + 3 * 60_000,
  source: 'store',
  metrics,
});
const bandUtilization = () =>
  screen
    .getAllByTestId('guest-history-group-chart')
    .find((el) => el.dataset.historyGroup === 'utilization')!;

// A 99% CPU burst inside a bucket that averages 20%.
const bandSpiky = {
  cpu: [bandPoint(0, 10), bandPoint(1, 20, 10, 99), bandPoint(2, 12)],
  memory: [bandPoint(0, 40), bandPoint(1, 40, 39.5, 41.5), bandPoint(2, 40)],
};

describe('drawer History min-max band', () => {
  it("draws a band up to a bucket's peak so a short spike is not averaged away", async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(bandResponse(bandSpiky));
    render(() => <GuestDrawerHistory target={bandTarget} range="1h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const band = bandUtilization().querySelector('path[data-history-band="cpu"]');
    expect(band).not.toBeNull();
    // 99% on the 0-100 scale sits at y = 8 + (1 - 0.99) * 66 = 8.66 of the viewBox.
    expect(band?.getAttribute('d')).toContain(',8.66');
    expect(band?.getAttribute('d')).toMatch(/ Z$/);
    // The average line itself still passes through 20%, not the peak.
    const line = bandUtilization().querySelector('path[data-history-series="cpu"]');
    expect(line?.getAttribute('d')).not.toContain(',8.66');
  });

  it('draws no band for buckets whose readings never spread', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      bandResponse({
        ...bandSpiky,
        memory: [bandPoint(0, 40), bandPoint(1, 40), bandPoint(2, 40)],
      }),
    );
    render(() => <GuestDrawerHistory target={bandTarget} range="1h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(bandUtilization().querySelector('path[data-history-band="memory"]')).toBeNull();
    expect(bandUtilization().querySelectorAll('path[data-history-band]')).toHaveLength(1);
  });

  it('names the peak only while a time is inspected, and to assistive technology', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(bandResponse(bandSpiky));
    render(() => <GuestDrawerHistory target={bandTarget} range="1h" />);
    const slider = await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    // The default readout is the latest stored time: one number per series.
    expect(bandUtilization().querySelector('[data-history-peak]')).toBeNull();

    fireEvent.focus(slider);
    fireEvent.input(slider, { target: { value: '1' } });
    expect(bandUtilization().querySelector('[data-history-peak="cpu"]')).toHaveTextContent(
      'peak 99.0%',
    );
    // Memory's bucket spread is too small to see, so it is not called a peak.
    expect(bandUtilization().querySelector('[data-history-peak="memory"]')).toBeNull();
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      expect.stringContaining(
        'CPU 20.0% average, 10.0% to 99.0%. Memory 40.0% average, 39.5% to 41.5%.',
      ),
    );

    fireEvent.blur(slider);
    expect(bandUtilization().querySelector('[data-history-peak]')).toBeNull();
    expect(
      within(bandUtilization()).getByRole('img', { name: 'Utilization history' }),
    ).toBeTruthy();
  });
});

describe('buildGuestDrawerHistoryBandPath', () => {
  const scale = { minValue: 0, maxValue: 100 };

  it('needs two points and some spread', () => {
    expect(
      buildGuestDrawerHistoryBandPath([bandPoint(0, 5, 1, 9)], scale, bandStart, bandStart + 1),
    ).toBe('');
    expect(
      buildGuestDrawerHistoryBandPath(
        [bandPoint(0, 5), bandPoint(1, 6)],
        scale,
        bandStart,
        bandStart + 60_000,
      ),
    ).toBe('');
  });

  it('runs forward along the highs and back along the lows', () => {
    const path = buildGuestDrawerHistoryBandPath(
      [bandPoint(0, 50, 0, 100), bandPoint(1, 50, 0, 100)],
      scale,
      bandStart,
      bandStart + 60_000,
    );
    expect(path).toBe('M34.00,8.00 L352.00,8.00 L352.00,74.00 L34.00,74.00 Z');
  });
});
