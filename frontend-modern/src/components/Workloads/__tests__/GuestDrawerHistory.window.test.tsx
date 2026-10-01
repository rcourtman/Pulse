import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import {
  getGuestDrawerHistoryRangeBounds,
  normalizeGuestDrawerHistoryPoints,
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

const start = Date.UTC(2026, 8, 30, 12);
const hour = 3_600_000;
const end = start + 24 * hour;
const target = { resourceType: 'agent' as const, resourceId: 'pbs-host' };
const point = (timestamp: number, value: number) => ({ timestamp, value, min: value, max: value });
const response = (
  overrides: Partial<AllMetricsHistoryResponse> = {},
): AllMetricsHistoryResponse => ({
  ...target,
  range: '24h',
  start,
  end,
  source: 'store',
  metrics: {
    cpu: [point(end - 10 * 60_000, 12), point(end - 5 * 60_000, 25)],
    memory: [point(end - 5 * 60_000, 40)],
    netin: [point(end - hour, 0), point(end - 5 * 60_000, 2048)],
    diskwrite: [point(end - 5 * 60_000, 0), point(end, 0)],
    temperature: [point(end - 5 * 60_000, 50)],
  },
  ...overrides,
});
const mount = () =>
  render(() => (
    <GuestDrawerHistory target={target} range="24h" groups={HOST_METRICS_HISTORY_GROUPS} />
  ));
const group = (id: string) =>
  screen
    .getAllByTestId('guest-history-group-chart')
    .find((chart) => chart.dataset.historyGroup === id)!;
const pathXs = (chart: HTMLElement) =>
  [
    ...chart
      .querySelector('path')!
      .getAttribute('d')!
      .matchAll(/[ML](-?[\d.]+),/g),
  ].map((match) => Number(match[1]));
const expectedX = (timestamp: number, first = start, last = end) =>
  34 + ((timestamp - first) / Math.max(1, last - first)) * 318;
const axis = (chart: HTMLElement) => within(chart).getByTestId('guest-history-time-window');
const selectSharedTime = (chart: HTMLElement) => {
  const slider = within(chart).getByRole('slider');
  slider.focus();
  fireEvent.input(slider, { target: { value: '1' } });
  return chart.querySelector('circle[r="3"]')!;
};

describe('GuestDrawerHistory shared time window', () => {
  it('does not stretch five minutes of stored CPU across a 24-hour query', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const xs = pathXs(group('utilization'));
    expect(xs[0]).toBeCloseTo(expectedX(end - 10 * 60_000), 2);
    expect(xs[1]).toBeCloseTo(expectedX(end - 5 * 60_000), 2);
    expect(xs[1] - xs[0]).toBeLessThan(2);
  });

  it('places the same observation time at the same x in panels with different sample coverage', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    mount();
    await screen.findByRole('slider', { name: 'Inspect Network I/O history' });
    const utilizationX = selectSharedTime(group('utilization')).getAttribute('cx');
    const networkX = selectSharedTime(group('network')).getAttribute('cx');
    expect(networkX).toBe(utilizationX);
    expect(Number(networkX)).toBeCloseTo(expectedX(end - 5 * 60_000));
  });

  it('labels every panel with the full dated interval, including empty and lone-observation panels', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    for (const chart of screen.getAllByTestId('guest-history-group-chart')) {
      const times = axis(chart).querySelectorAll('time');
      expect(times).toHaveLength(2);
      expect(times[0]).toHaveAttribute('datetime', new Date(start).toISOString());
      expect(times[1]).toHaveAttribute('datetime', new Date(end).toISOString());
      expect(times[0]).toHaveAccessibleName(`Window start: ${new Date(start).toLocaleString()}`);
      expect(times[1]).toHaveAccessibleName(`Window end: ${new Date(end).toLocaleString()}`);
      expect(times[0].textContent).not.toBe(times[1].textContent);
    }
  });

  it('retains the last fulfilled window with its data after a failed refresh', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockRejectedValueOnce(new Error('Private fixture detail'));
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const before = group('utilization').querySelector('path')!.getAttribute('d');
    fireEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
    await screen.findByText('History refresh failed. Showing previously loaded history.');
    expect(group('utilization').querySelector('path')).toHaveAttribute('d', before);
    expect(axis(group('utilization')).querySelector('time')).toHaveAttribute(
      'datetime',
      new Date(start).toISOString(),
    );
    expect(document.body).not.toHaveTextContent('Private fixture detail');
  });

  it('moves the window on a successful poll without moving a selected observation to a different time', async () => {
    vi.useFakeTimers();
    const shift = 10 * 60_000;
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockResolvedValueOnce(response({ start: start + shift, end: end + shift }));
    mount();
    await vi.advanceTimersByTimeAsync(0);
    selectSharedTime(group('utilization'));
    const valueText = within(group('utilization'))
      .getByRole('slider')
      .getAttribute('aria-valuetext');
    await vi.advanceTimersByTimeAsync(30_000);
    expect(within(group('utilization')).getByRole('slider')).toHaveAttribute(
      'aria-valuetext',
      valueText,
    );
    expect(
      Number(group('utilization').querySelector('circle[r="3"]')!.getAttribute('cx')),
    ).toBeCloseTo(expectedX(end - 5 * 60_000, start + shift, end + shift));
    expect(axis(group('utilization')).querySelectorAll('time')[1]).toHaveAttribute(
      'datetime',
      new Date(end + shift).toISOString(),
    );
  });

  it('does not keep former-range endpoints while a replacement read is pending', async () => {
    let finish!: (value: AllMetricsHistoryResponse) => void;
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = resolve;
          }),
      );
    const [range, setRange] = createSignal<HistoryTimeRange>('24h');
    render(() => (
      <GuestDrawerHistory target={target} range={range()} groups={HOST_METRICS_HISTORY_GROUPS} />
    ));
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    setRange('1h');
    await waitFor(() =>
      expect(screen.queryAllByTestId('guest-history-time-window')).toHaveLength(0),
    );
    expect(group('utilization').querySelectorAll('path')).toHaveLength(0);
    finish(response({ range: '1h', start: end - hour }));
    await waitFor(() => expect(screen.getAllByTestId('guest-history-time-window')).toHaveLength(4));
    expect(pathXs(group('utilization'))[0]).toBeCloseTo(expectedX(end - 10 * 60_000, end - hour));
  });

  it('keeps an empty valid query window without inventing observations or a trend', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response({ metrics: {} }));
    mount();
    await waitFor(() =>
      expect(screen.getAllByText('No stored history in this range')).toHaveLength(4),
    );
    expect(screen.queryByRole('slider')).toBeNull();
    expect(document.querySelectorAll('path')).toHaveLength(0);
    expect(screen.getAllByTestId('guest-history-time-window')).toHaveLength(4);
  });

  it('falls back to one shared observed interval when the API window is invalid', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({ start: NaN, end: Infinity }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const xs = pathXs(group('utilization'));
    expect(xs[0]).toBeCloseTo(expectedX(end - 10 * 60_000, end - hour));
    expect(axis(group('utilization')).querySelector('time')).toHaveAttribute(
      'datetime',
      new Date(end - hour).toISOString(),
    );
    expect(selectSharedTime(group('utilization')).getAttribute('cx')).toBe(
      selectSharedTime(group('network')).getAttribute('cx'),
    );
  });

  it('encloses returned edge observations rather than clipping or discarding an aggregated bucket', async () => {
    const edge = start - 5 * 60_000;
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        metrics: { ...response().metrics, cpu: [point(edge, 10), point(end - 5 * 60_000, 25)] },
      }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(axis(group('network')).querySelector('time')).toHaveAttribute(
      'datetime',
      new Date(edge).toISOString(),
    );
    expect(pathXs(group('network'))[0]).toBeCloseTo(expectedX(end - hour, edge));
    expect(within(group('utilization')).getByRole('slider')).toHaveAttribute('max', '1');
  });

  it('rejects non-dates before geometry and labels can be poisoned', () => {
    expect(normalizeGuestDrawerHistoryPoints([point(9e15, 12), point(start, 0)], '%')).toEqual([
      point(start, 0),
    ]);
    expect(
      getGuestDrawerHistoryRangeBounds([{ points: [point(9e15, 12), point(start, 0)] }]),
    ).toEqual({ startTime: start, endTime: start });
  });

  it('does not let an unconfigured metric change the visible panels time window', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        metrics: { ...response().metrics, unrelated: [point(end + 90 * 24 * hour, 10)] },
      }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(axis(group('utilization')).querySelectorAll('time')[1]).toHaveAttribute(
      'datetime',
      new Date(end).toISOString(),
    );
    expect(pathXs(group('utilization'))[0]).toBeCloseTo(expectedX(end - 10 * 60_000), 2);
  });
});
