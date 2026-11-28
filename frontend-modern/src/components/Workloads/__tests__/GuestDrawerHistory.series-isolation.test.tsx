import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import {
  getEffectiveHiddenHistoryMetrics,
  toggleGuestDrawerHistorySeriesVisibility,
  type GuestDrawerHistoryTarget,
} from '../guestDrawerModel';

vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(async () => undefined),
}));

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
});

const start = Date.UTC(2026, 9, 12, 12);
const target: GuestDrawerHistoryTarget = { resourceType: 'vm', resourceId: 'fixture:pve:101' };

const point = (minute: number, value: number, min = value, max = value) => ({
  timestamp: start + minute * 60_000,
  value,
  min,
  max,
});

const response = (
  metrics: AllMetricsHistoryResponse['metrics'],
  overrides: Partial<AllMetricsHistoryResponse> = {},
): AllMetricsHistoryResponse => ({
  ...target,
  range: '24h',
  start,
  end: start + 10 * 60_000,
  source: 'store',
  metrics,
  ...overrides,
});

const group = (id = 'utilization') =>
  screen.getAllByTestId('guest-history-group-chart').find((el) => el.dataset.historyGroup === id)!;

const legendButton = (chart: HTMLElement, metric: string) =>
  chart.querySelector(`button[data-history-legend-series="${metric}"]`) as HTMLButtonElement;

const drawnSeries = (chart: HTMLElement) =>
  [...chart.querySelectorAll('path[data-history-series]')].map((el) =>
    el.getAttribute('data-history-series'),
  );

const drawnBands = (chart: HTMLElement) =>
  [...chart.querySelectorAll('path[data-history-band]')].map((el) =>
    el.getAttribute('data-history-band'),
  );

const drawnThresholds = (chart: HTMLElement) =>
  [...chart.querySelectorAll('line[data-history-threshold]')].map((el) =>
    el.getAttribute('data-history-threshold'),
  );

const axisLabels = (chart: HTMLElement) =>
  [...chart.querySelectorAll('[data-history-axis-label]')].map((el) => el.textContent?.trim());

const hover = (chart: HTMLElement, minute: number, durationMinutes = 10) => {
  const plot = within(chart).getByTestId('guest-history-plot');
  vi.spyOn(plot, 'getBoundingClientRect').mockReturnValue({ left: 0, width: 360 } as DOMRect);
  fireEvent.mouseMove(plot, { clientX: 34 + (minute / durationMinutes) * 318 });
  return plot;
};

describe('toggleGuestDrawerHistorySeriesVisibility', () => {
  const metrics = ['cpu', 'memory', 'disk'];

  it('isolates on regular click, switches isolation directly, and restores on second click', () => {
    const isolatedCpu = toggleGuestDrawerHistorySeriesVisibility(metrics, new Set(), 'cpu', false);
    expect([...isolatedCpu]).toEqual(['memory', 'disk']);

    const isolatedMemory = toggleGuestDrawerHistorySeriesVisibility(
      metrics,
      isolatedCpu,
      'memory',
      false,
    );
    expect([...isolatedMemory]).toEqual(['cpu', 'disk']);

    const restored = toggleGuestDrawerHistorySeriesVisibility(
      metrics,
      isolatedMemory,
      'memory',
      false,
    );
    expect([...restored]).toEqual([]);
  });

  it('toggles individual series with modifier click and restores all instead of hiding the last series', () => {
    const withoutDisk = toggleGuestDrawerHistorySeriesVisibility(metrics, new Set(), 'disk', true);
    expect([...withoutDisk]).toEqual(['disk']);

    const onlyCpu = toggleGuestDrawerHistorySeriesVisibility(metrics, withoutDisk, 'memory', true);
    expect([...onlyCpu]).toEqual(['disk', 'memory']);

    const restoredAll = toggleGuestDrawerHistorySeriesVisibility(metrics, onlyCpu, 'cpu', true);
    expect([...restoredAll]).toEqual([]);

    const addedBack = toggleGuestDrawerHistorySeriesVisibility(metrics, onlyCpu, 'memory', true);
    expect([...addedBack]).toEqual(['disk']);
  });

  it('normalizes stale or full hidden sets cleanly', () => {
    expect([
      ...getEffectiveHiddenHistoryMetrics(['temperature'], new Set(['temperature'])),
    ]).toEqual([]);
    expect([
      ...getEffectiveHiddenHistoryMetrics(['cpu', 'memory'], new Set(['cpu', 'memory', 'stale'])),
    ]).toEqual([]);
    expect([
      ...getEffectiveHiddenHistoryMetrics(['cpu', 'memory'], new Set(['memory', 'stale'])),
    ]).toEqual(['memory']);
  });
});

describe('GuestDrawerHistory legend series isolation', () => {
  it('isolates a clicked metric, switches directly to another, and restores all on second click or Show all', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 12, 8, 28), point(5, 18, 10, 42), point(10, 24, 15, 36)],
        memory: [point(0, 62, 60, 64), point(5, 66, 63, 72), point(10, 70, 68, 74)],
        disk: [point(0, 40), point(5, 42), point(10, 44)],
      }),
    );

    render(() => (
      <GuestDrawerHistory
        target={target}
        range="24h"
        alerts={{
          cpu: { severity: 'warning', value: 80 },
          memory: { severity: 'critical', value: 90 },
        }}
      />
    ));

    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = group('utilization');

    expect(drawnSeries(chart)).toEqual(['cpu', 'memory', 'disk']);
    expect(drawnThresholds(chart)).toEqual(['cpu', 'memory']);
    expect(within(chart).queryByTestId('guest-history-show-all')).toBeNull();

    // Click CPU -> isolate CPU only, keep fixed 0-100% Y-axis, hide Memory alert line.
    fireEvent.click(legendButton(chart, 'cpu'));

    expect(drawnSeries(chart)).toEqual(['cpu']);
    expect(drawnBands(chart)).toEqual(['cpu']);
    expect(drawnThresholds(chart)).toEqual(['cpu']);
    expect(axisLabels(chart)).toEqual(['100%', '50%', '0%']);
    expect(legendButton(chart, 'cpu')).toHaveAttribute('aria-pressed', 'true');
    expect(legendButton(chart, 'cpu')).toHaveAttribute('data-history-series-state', 'isolated');
    expect(legendButton(chart, 'memory')).toHaveAttribute('aria-pressed', 'false');
    expect(legendButton(chart, 'memory')).toHaveAttribute('data-history-series-state', 'hidden');
    expect(legendButton(chart, 'disk')).toHaveAttribute('aria-pressed', 'false');
    expect(within(chart).getByTestId('guest-history-series-status')).toHaveTextContent(
      'Utilization: showing CPU only.',
    );
    expect(
      within(chart).getByRole('img', { name: 'Utilization history' }),
    ).toHaveAccessibleDescription(expect.stringContaining('Showing CPU only.'));
    expect(
      within(chart).getByRole('img', { name: 'Utilization history' }),
    ).toHaveAccessibleDescription(expect.not.stringContaining('Memory critical alert'));

    // Hovering plots markers and peak labels only for the visible series.
    hover(chart, 5);
    expect(
      [...chart.querySelectorAll('[data-history-hover-marker]')].map((el) =>
        el.getAttribute('data-history-hover-marker'),
      ),
    ).toEqual(['cpu']);
    expect(
      [...chart.querySelectorAll('[data-history-peak]')].map((el) =>
        el.getAttribute('data-history-peak'),
      ),
    ).toEqual(['cpu']);

    // Click Memory -> directly switches isolation to Memory.
    fireEvent.click(legendButton(chart, 'memory'));
    expect(drawnSeries(chart)).toEqual(['memory']);
    expect(drawnThresholds(chart)).toEqual(['memory']);
    expect(legendButton(chart, 'memory')).toHaveAttribute('aria-pressed', 'true');
    expect(legendButton(chart, 'cpu')).toHaveAttribute('aria-pressed', 'false');

    // Click Memory again -> restores all series.
    fireEvent.click(legendButton(chart, 'memory'));
    expect(drawnSeries(chart)).toEqual(['cpu', 'memory', 'disk']);
    expect(drawnThresholds(chart)).toEqual(['cpu', 'memory']);
    expect(within(chart).queryByTestId('guest-history-show-all')).toBeNull();

    // Isolate Disk, then restore via Show all and verify focus moves to the first legend button.
    fireEvent.click(legendButton(chart, 'disk'));
    expect(drawnSeries(chart)).toEqual(['disk']);
    const showAll = within(chart).getByTestId('guest-history-show-all');
    showAll.focus();
    fireEvent.click(showAll);
    expect(drawnSeries(chart)).toEqual(['cpu', 'memory', 'disk']);
    expect(legendButton(chart, 'cpu')).toHaveFocus();
  });

  it('supports Ctrl/Cmd/Shift multi-toggle and restores all instead of hiding the final visible series', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 12), point(10, 24)],
        memory: [point(0, 62), point(10, 70)],
        disk: [point(0, 40), point(10, 44)],
      }),
    );

    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = group('utilization');

    // Cmd+click Disk hides only Disk (CPU + Memory remain visible).
    fireEvent.click(legendButton(chart, 'disk'), { metaKey: true });
    expect(drawnSeries(chart)).toEqual(['cpu', 'memory']);
    expect(legendButton(chart, 'cpu')).toHaveAttribute('data-history-series-state', 'visible');
    expect(legendButton(chart, 'memory')).toHaveAttribute('data-history-series-state', 'visible');
    expect(legendButton(chart, 'disk')).toHaveAttribute('data-history-series-state', 'hidden');

    // Ctrl+click Memory leaves only CPU.
    fireEvent.click(legendButton(chart, 'memory'), { ctrlKey: true });
    expect(drawnSeries(chart)).toEqual(['cpu']);
    expect(legendButton(chart, 'cpu')).toHaveAttribute('data-history-series-state', 'isolated');

    // Shift+click CPU (last remaining series) restores all series instead of leaving a blank chart.
    fireEvent.click(legendButton(chart, 'cpu'), { shiftKey: true });
    expect(drawnSeries(chart)).toEqual(['cpu', 'memory', 'disk']);
  });

  it('rescales auto-scaled I/O Y-axis when isolating a quiet series beside a large spike', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        netin: [point(0, 80 * 1024 * 1024), point(10, 100 * 1024 * 1024)],
        netout: [point(0, 512 * 1024), point(10, 1024 * 1024)],
      }),
    );

    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Network I/O history' });
    const network = group('network');

    // Combined scale is dominated by 100 MB/s In.
    expect(axisLabels(network)[0]).toBe('115M/s');

    // Isolating Out (max 1 MB/s) rescales the Y-axis down from 115M/s to ~1.2M/s.
    fireEvent.click(legendButton(network, 'netout'));
    expect(drawnSeries(network)).toEqual(['netout']);
    expect(axisLabels(network)[0]).toMatch(/^1\.[12]M\/s$/);

    // Clicking Out again restores both series and the full scale.
    fireEvent.click(legendButton(network, 'netout'));
    expect(drawnSeries(network)).toEqual(['netin', 'netout']);
    expect(axisLabels(network)[0]).toBe('115M/s');
  });

  it('keeps single-series panels non-interactive and isolates state per chart panel', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 12), point(10, 24)],
        memory: [point(0, 62), point(10, 70)],
        netin: [point(0, 1024), point(10, 2048)],
        netout: [point(0, 512), point(10, 1024)],
        temperature: [point(0, 45), point(10, 48)],
      }),
    );

    render(() => (
      <GuestDrawerHistory target={target} range="24h" groups={HOST_METRICS_HISTORY_GROUPS} />
    ));

    await screen.findByRole('slider', { name: 'Inspect Thermals history' });
    const thermals = group('thermals');
    const utilization = group('utilization');
    const network = group('network');

    // Single-series Thermals panel has no legend toggle buttons.
    expect(thermals.querySelectorAll('button[data-history-legend-series]')).toHaveLength(0);

    // Isolating CPU in Utilization does not affect Network I/O.
    fireEvent.click(legendButton(utilization, 'cpu'));
    expect(drawnSeries(utilization)).toEqual(['cpu']);
    expect(drawnSeries(network)).toEqual(['netin', 'netout']);
  });

  it('preserves gaps when hiding a sibling series and stays honest when isolating an empty series', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 12), point(10, 24)],
        memory: [point(0, 50), point(5, 55), point(10, 60)],
      }),
    );

    render(() => (
      <GuestDrawerHistory
        target={target}
        range="24h"
        currentMetrics={{ cpu: 99, memory: 98, disk: 77 }}
      />
    ));

    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = group('utilization');

    // Isolating CPU keeps its honest missing-observation break across minute 5.
    fireEvent.click(legendButton(chart, 'cpu'));
    expect(drawnSeries(chart)).toEqual([]);
    expect(chart.querySelectorAll('[data-history-observation="cpu"]')).toHaveLength(2);
    expect(chart.querySelector('[data-history-gaps]')).toHaveTextContent(
      'Missing observations: CPU.',
    );

    // Isolating Disk (0 stored points on a stored-history panel) shows an explicit
    // empty message and never substitutes live fallback current readings.
    fireEvent.click(legendButton(chart, 'disk'));
    expect(chart).toHaveTextContent('No stored history for Disk in this range');
    expect(chart.querySelector('[data-history-current]')).toBeNull();
    expect(chart).not.toHaveTextContent('77.0%');
    expect(within(chart).queryByTestId('guest-history-hover-time')).toBeNull();
  });

  it('persists isolation across range changes on the same resource and resets when switching resources', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockImplementation(async (params) =>
      response(
        {
          cpu: [point(0, 15), point(10, 25)],
          memory: [point(0, 50), point(10, 60)],
          disk: [point(0, 30), point(10, 35)],
        },
        { resourceId: params.resourceId, range: params.range ?? '24h' },
      ),
    );

    const [currentTarget, setCurrentTarget] = createSignal(target);
    const [range, setRange] = createSignal<HistoryTimeRange>('24h');

    render(() => <GuestDrawerHistory target={currentTarget()} range={range()} />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = group('utilization');

    fireEvent.click(legendButton(chart, 'cpu'));
    expect(drawnSeries(chart)).toEqual(['cpu']);

    // Changing time range on the same resource keeps CPU isolated.
    setRange('7d');
    await waitFor(() => expect(ChartsAPI.getMetricsHistory).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(drawnSeries(chart)).toEqual(['cpu']));
    expect(legendButton(chart, 'cpu')).toHaveAttribute('data-history-series-state', 'isolated');

    // Switching to another resource resets isolation back to all series.
    setCurrentTarget({ resourceType: 'vm', resourceId: 'fixture:pve:102' });
    await waitFor(() => expect(ChartsAPI.getMetricsHistory).toHaveBeenCalledTimes(3));
    await waitFor(() => expect(drawnSeries(chart)).toEqual(['cpu', 'memory', 'disk']));
    expect(within(chart).queryByTestId('guest-history-show-all')).toBeNull();
  });

  it('keeps the exact focused legend button mounted when toggling isolation and restoring', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 12), point(10, 24)],
        memory: [point(0, 62), point(10, 70)],
        disk: [point(0, 40), point(10, 44)],
      }),
    );

    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = group('utilization');

    const memoryButton = legendButton(chart, 'memory');
    memoryButton.focus();
    expect(document.activeElement).toBe(memoryButton);

    // Isolate Memory via button activation; the exact DOM node stays mounted and focused.
    fireEvent.keyDown(memoryButton, { key: ' ' });
    fireEvent.keyUp(memoryButton, { key: ' ' });
    fireEvent.click(memoryButton);
    expect(drawnSeries(chart)).toEqual(['memory']);
    expect(legendButton(chart, 'memory')).toBe(memoryButton);
    expect(document.activeElement).toBe(memoryButton);

    // Activate Memory again to restore all series; focus remains on the same button.
    fireEvent.keyDown(memoryButton, { key: ' ' });
    fireEvent.keyUp(memoryButton, { key: ' ' });
    fireEvent.click(memoryButton);
    expect(drawnSeries(chart)).toEqual(['cpu', 'memory', 'disk']);
    expect(legendButton(chart, 'memory')).toBe(memoryButton);
    expect(document.activeElement).toBe(memoryButton);
  });

  it('hides deferred freshness prose visually for hidden series while keeping accessible descriptions', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 12), point(10, 24)],
        memory: [point(0, 62), point(10, 70)],
      }),
    );

    render(() => (
      <GuestDrawerHistory
        target={target}
        range="24h"
        deferredMetrics={{
          memory: {
            lastKnownValue: 70,
            valueLabel: 'freshness unknown',
            message: 'Freshness unknown. Not a current measurement.',
          },
        }}
      />
    ));

    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = group('utilization');
    const deferredNote = chart.querySelector('[data-history-deferred="memory"]') as HTMLElement;
    const memoryButton = legendButton(chart, 'memory');

    expect(deferredNote).not.toHaveClass('sr-only');
    expect(memoryButton).toHaveAttribute('aria-describedby', deferredNote.id);

    // Isolating CPU hides Memory's freshness note visually while preserving aria-describedby.
    fireEvent.click(legendButton(chart, 'cpu'));
    expect(deferredNote).toHaveClass('sr-only');
    expect(memoryButton).toHaveAttribute('aria-describedby', deferredNote.id);

    // Restoring all series brings the inline note back.
    fireEvent.click(legendButton(chart, 'cpu'));
    expect(deferredNote).not.toHaveClass('sr-only');
  });

  it('resolves mixed-source readout labels from only the visible series', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response(
        {
          diskread: [point(0, 1024), point(10, 2048)],
          diskwrite: [point(0, 512), point(10, 1024)],
        },
        {
          source: undefined,
          metricSources: {
            diskread: 'store',
            diskwrite: 'memory',
          },
        },
      ),
    );

    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Disk I/O history' });
    const diskIo = group('disk-io');
    const timeReadout = () => within(diskIo).getByTestId('guest-history-hover-time');

    expect(timeReadout()).toHaveTextContent('Latest recorded');

    // Isolating the stored Read series updates the readout origin to Latest stored.
    fireEvent.click(legendButton(diskIo, 'diskread'));
    expect(timeReadout()).toHaveTextContent('Latest stored');

    // Isolating the memory-backed Write series uses Latest recorded.
    fireEvent.click(legendButton(diskIo, 'diskwrite'));
    expect(timeReadout()).toHaveTextContent('Latest recorded');

    // Restoring both returns to Latest recorded.
    fireEvent.click(legendButton(diskIo, 'diskwrite'));
    expect(timeReadout()).toHaveTextContent('Latest recorded');
  });

  it('rescales both Celsius temperature axis bounds when isolating a cooler series', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu_temp: [point(0, 68), point(10, 74)],
        drive_temp: [point(0, 34), point(10, 38)],
      }),
    );

    render(() => (
      <GuestDrawerHistory
        target={target}
        range="24h"
        groups={[
          {
            id: 'thermals-multi',
            label: 'Thermals',
            unit: 'C',
            series: [
              { metric: 'cpu_temp', label: 'CPU', unit: 'C', color: '#ef4444' },
              { metric: 'drive_temp', label: 'Drive', unit: 'C', color: '#3b82f6' },
            ],
          },
        ]}
      />
    ));

    await screen.findByRole('slider', { name: 'Inspect Thermals history' });
    const thermals = group('thermals-multi');
    const combinedAxis = axisLabels(thermals);
    expect(combinedAxis).toEqual(['80°C', '54°C', '28°C']);

    // Isolating Drive moves both the upper and lower Y-axis bounds.
    fireEvent.click(legendButton(thermals, 'drive_temp'));
    const isolatedAxis = axisLabels(thermals);
    expect(isolatedAxis).toEqual(['40°C', '36°C', '32°C']);
    expect(isolatedAxis[0]).not.toBe(combinedAxis[0]);
    expect(isolatedAxis[2]).not.toBe(combinedAxis[2]);

    // Restoring all recovers both original bounds.
    fireEvent.click(legendButton(thermals, 'drive_temp'));
    expect(axisLabels(thermals)).toEqual(combinedAxis);
  });

  it('preserves series isolation across a failed range read and subsequent recovery', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(
        response(
          {
            cpu: [point(0, 15), point(10, 25)],
            memory: [point(0, 50), point(10, 60)],
            disk: [point(0, 30), point(10, 35)],
          },
          { range: '24h' },
        ),
      )
      .mockRejectedValueOnce(Object.assign(new Error('temporary outage'), { status: 503 }))
      .mockResolvedValueOnce(
        response(
          {
            cpu: [point(0, 18), point(10, 28)],
            memory: [point(0, 52), point(10, 62)],
            disk: [point(0, 31), point(10, 36)],
          },
          { range: '7d' },
        ),
      );

    const [range, setRange] = createSignal<HistoryTimeRange>('24h');
    render(() => <GuestDrawerHistory target={target} range={range()} />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });

    // Isolate CPU at 24h.
    fireEvent.click(legendButton(group('utilization'), 'cpu'));
    expect(drawnSeries(group('utilization'))).toEqual(['cpu']);

    // Switch to an uncached 7d range that fails and temporarily unmounts the charts.
    setRange('7d');
    const retry = await screen.findByRole('button', { name: 'Retry history' });
    expect(screen.queryByTestId('guest-history-group-chart')).toBeNull();

    // Retrying succeeds and keeps CPU isolated without losing selection state.
    fireEvent.click(retry);
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
    await waitFor(() => expect(drawnSeries(group('utilization'))).toEqual(['cpu']));
    expect(legendButton(group('utilization'), 'cpu')).toHaveAttribute(
      'data-history-series-state',
      'isolated',
    );
  });

  it('lets tap-only users select two of three series with Compare mode without modifier keys', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 12), point(10, 24)],
        memory: [point(0, 62), point(10, 70)],
        disk: [point(0, 40), point(10, 44)],
        netin: [point(0, 1024), point(10, 2048)],
        netout: [point(0, 512), point(10, 1024)],
      }),
    );

    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const utilization = group('utilization');
    const network = group('network');

    // Compare is absent in default state and never shown on 2-series panels.
    expect(within(utilization).queryByTestId('guest-history-compare-toggle')).toBeNull();
    fireEvent.click(legendButton(network, 'netout'));
    expect(within(network).queryByTestId('guest-history-compare-toggle')).toBeNull();

    // Tap CPU -> single-tap isolation still works as default.
    fireEvent.click(legendButton(utilization, 'cpu'));
    expect(drawnSeries(utilization)).toEqual(['cpu']);
    const compareBtn = within(utilization).getByTestId('guest-history-compare-toggle');
    expect(compareBtn).toHaveAttribute('aria-pressed', 'false');

    // Tap Compare, then tap Memory (no modifier keys) -> CPU + Memory with Disk hidden.
    fireEvent.click(compareBtn);
    expect(compareBtn).toHaveAttribute('aria-pressed', 'true');
    expect(legendButton(utilization, 'cpu')).toHaveAttribute('title', 'Click to show all series.');
    fireEvent.click(legendButton(utilization, 'memory'));
    expect(legendButton(utilization, 'cpu')).toHaveAttribute('title', 'Click to hide CPU.');
    expect(drawnSeries(utilization)).toEqual(['cpu', 'memory']);
    expect(legendButton(utilization, 'disk')).toHaveAttribute('aria-pressed', 'false');

    // Tapping Disk adds it back and restores all 3 series, exiting Compare mode cleanly.
    fireEvent.click(legendButton(utilization, 'disk'));
    expect(drawnSeries(utilization)).toEqual(['cpu', 'memory', 'disk']);
    expect(within(utilization).queryByTestId('guest-history-compare-toggle')).toBeNull();
  });
});
