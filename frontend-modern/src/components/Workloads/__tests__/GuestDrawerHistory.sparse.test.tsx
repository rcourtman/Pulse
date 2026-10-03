import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';

vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(async () => undefined),
  maxHistoryDays: () => 7,
  isRangeLocked: (range: HistoryTimeRange) => ['14d', '30d', '90d'].includes(range),
}));

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
});

const start = Date.UTC(2026, 9, 1, 11);
const time = start + 30 * 60_000;
const end = start + 60 * 60_000;
const target = { resourceType: 'agent' as const, resourceId: 'pbs-host' };
const point = (timestamp = time, value = 0) => ({ timestamp, value, min: value, max: value });
const response = (
  metrics: AllMetricsHistoryResponse['metrics'] = {},
): AllMetricsHistoryResponse => ({
  ...target,
  range: '1h',
  start,
  end,
  source: 'store',
  metrics,
});
const mount = (currentMetrics: Record<string, number | undefined> = {}) =>
  render(() => (
    <GuestDrawerHistory
      target={target}
      range="1h"
      groups={HOST_METRICS_HISTORY_GROUPS}
      currentMetrics={currentMetrics}
    />
  ));
const group = (id = 'utilization') =>
  screen
    .getAllByTestId('guest-history-group-chart')
    .find((chart) => chart.dataset.historyGroup === id)!;
const dots = () => document.querySelectorAll('[data-history-observation]');

describe('GuestDrawerHistory sparse observations and current provenance', () => {
  it.each([
    ['cpu', 'utilization', 'CPU 0.0%'],
    ['netin', 'network', 'In 0 B/s'],
    ['temperature', 'thermals', 'CPU 0°C'],
  ])(
    'plots a lone measured zero for %s, without a synthetic trend or collecting claim',
    async (metric, id, label) => {
      const fetch = vi
        .spyOn(ChartsAPI, 'getMetricsHistory')
        .mockResolvedValue(response({ [metric]: [point()] }));
      mount({ [metric]: 99 });
      await waitFor(() => expect(dots()).toHaveLength(1));
      const chart = group(id);
      const dot = chart.querySelector(`[data-history-observation="${metric}"]`)!;
      expect(Number(dot.getAttribute('cx'))).toBeCloseTo(193);
      expect(Number(dot.getAttribute('cy'))).toBeCloseTo(74);
      expect(chart.querySelector('path')).toBeNull();
      expect(chart.querySelector('[data-history-current]')).toBeNull();
      expect(chart).not.toHaveTextContent('No stored history in this range');
      expect(chart).toHaveTextContent('Single observation. No trend yet.');
      expect(chart.querySelector('time.block')).toHaveAttribute(
        'datetime',
        new Date(time).toISOString(),
      );
      expect(within(chart).getByRole('img')).toHaveAccessibleDescription(
        new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')),
      );
      expect(screen.queryByRole('slider')).not.toBeInTheDocument();
      expect(screen.queryByText('Collecting history')).not.toBeInTheDocument();
      expect(fetch).toHaveBeenCalledTimes(1);
    },
  );

  it('draws a lone series alongside a trend and labels only the live fallback', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(start, 10), point(end, 20)],
        memory: [point(time, 0)],
      }),
    );
    mount({ cpu: 99, memory: 98, disk: 88 });
    await waitFor(() => expect(dots()).toHaveLength(1));
    expect(group().querySelectorAll('path')).toHaveLength(1);
    expect(group().querySelector('[data-history-observation]')).toHaveAttribute(
      'data-history-observation',
      'memory',
    );
    expect(group().querySelectorAll('[data-history-current]')).toHaveLength(1);
    expect(group().querySelector('[data-history-current="disk"]')).toHaveTextContent(
      'Disk88.0%current',
    );
    expect(group()).not.toHaveTextContent('Single observation. No trend yet.');
    const slider = within(group()).getByRole('slider');
    slider.focus();
    fireEvent.input(slider, { target: { value: '1' } });
    expect(group()).toHaveTextContent('CPU-');
    expect(group()).toHaveTextContent('Memory0.0%');
    expect(group()).toHaveTextContent('Disk-');
    expect(group().querySelector('[data-history-current]')).toBeNull();
    expect(dots()).toHaveLength(1);
  });

  it('does not connect single observations from different series or borrow their times', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(start, 10)],
        memory: [point(end, 20)],
      }),
    );
    mount();
    await waitFor(() => expect(dots()).toHaveLength(2));
    expect(group().querySelector('path')).toBeNull();
    expect(group().querySelector('[data-history-observation="cpu"]')).toHaveAttribute('cx', '34');
    expect(group().querySelector('[data-history-observation="memory"]')).toHaveAttribute(
      'cx',
      '352',
    );
    const slider = within(group()).getByRole('slider');
    slider.focus();
    fireEvent.input(slider, { target: { value: '0' } });
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      `${new Date(start).toLocaleString()}. CPU 10.0%. Memory no observation. Disk no observation.`,
    );
  });

  it.each(['empty', 'invalid'] as const)(
    'reports a successful %s read honestly and never plots current readings',
    async (kind) => {
      vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
        response(
          kind === 'empty'
            ? {}
            : {
                cpu: [point(NaN, 12)],
                memory: [point(time, Infinity)],
              },
        ),
      );
      mount({ cpu: 0, memory: Infinity, disk: undefined });
      await waitFor(() =>
        expect(screen.getAllByText('No stored history in this range')).toHaveLength(4),
      );
      expect(group().querySelector('[data-history-current="cpu"]')).toHaveTextContent(
        'CPU0.0%current',
      );
      expect(document.querySelectorAll('[data-history-current]')).toHaveLength(1);
      expect(dots()).toHaveLength(0);
      expect(document.querySelectorAll('path')).toHaveLength(0);
      expect(screen.queryByRole('slider')).not.toBeInTheDocument();
      expect(screen.queryByText('Single observation. No trend yet.')).not.toBeInTheDocument();
    },
  );

  it('retains a lone observation after failed refresh and removes its marker and caption when a trend arrives', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response({ cpu: [point(time, 0)] }))
      .mockRejectedValueOnce(new Error('private diagnostic'))
      .mockResolvedValueOnce(response({ cpu: [point(time, 0), point(end, 5)] }));
    mount();
    await waitFor(() => expect(dots()).toHaveLength(1));
    fireEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
    await screen.findByText('History refresh failed. Showing previously loaded history.');
    expect(dots()).toHaveLength(1);
    expect(group()).toHaveTextContent('Single observation. No trend yet.');
    expect(document.body).not.toHaveTextContent('private diagnostic');
    fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
    await screen.findByRole('slider');
    expect(group().querySelectorAll('path')).toHaveLength(1);
    expect(dots()).toHaveLength(0);
    expect(group()).not.toHaveTextContent('Single observation. No trend yet.');
    expect(fetch).toHaveBeenCalledTimes(3);
  });

  it.each(['target', 'range'] as const)(
    'clears former observations and captions during an uncached %s replacement',
    async (replacement) => {
      vi.spyOn(ChartsAPI, 'getMetricsHistory')
        .mockResolvedValueOnce(response({ cpu: [point(time, 0)] }))
        .mockImplementationOnce(() => new Promise(() => {}));
      const [current, setTarget] = createSignal(target);
      const [range, setRange] = createSignal<HistoryTimeRange>('1h');
      render(() => (
        <GuestDrawerHistory
          target={current()}
          range={range()}
          groups={HOST_METRICS_HISTORY_GROUPS}
        />
      ));
      await waitFor(() => expect(dots()).toHaveLength(1));
      if (replacement === 'target') setTarget({ ...target, resourceId: 'other-host' });
      else setRange('6h');
      await waitFor(() => expect(screen.getAllByText('Loading history')).toHaveLength(4));
      expect(dots()).toHaveLength(0);
      expect(screen.queryByText('Single observation. No trend yet.')).not.toBeInTheDocument();
      expect(document.querySelectorAll('time')).toHaveLength(0);
    },
  );
});
