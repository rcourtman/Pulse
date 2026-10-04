import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';

vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(async () => undefined),
  maxHistoryDays: () => 90,
  isRangeLocked: () => false,
}));
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
});

const start = Date.UTC(2026, 9, 4, 12);
const target = { resourceType: 'vm' as const, resourceId: 'fixture:pve:101' };
const point = (minute: number, value: number) => ({
  timestamp: start + minute * 60_000,
  value,
  min: value,
  max: value,
});
const response = (
  metrics: AllMetricsHistoryResponse['metrics'] = {
    cpu: [0, 1, 2, 3, 4].map((minute) => point(minute, 10)),
    memory: [point(0, 20), point(1, 25), point(4, 30)],
    disk: [point(0, 50), point(4, 50)],
  },
): AllMetricsHistoryResponse => ({
  ...target,
  range: '1h',
  start,
  end: start + 60 * 60_000,
  source: 'store',
  metrics,
});
const group = (id = 'utilization') =>
  screen.getAllByTestId('guest-history-group-chart').find((el) => el.dataset.historyGroup === id)!;
const paths = (colour: string, id = 'utilization') => [
  ...group(id).querySelectorAll(`path[stroke="${colour}"]`),
];
const marks = (metric: string, id = 'utilization') =>
  group(id).querySelectorAll(`[data-history-observation="${metric}"]`);
const note = (id = 'utilization') => group(id).querySelector('[data-history-gaps]');
const mount = () => render(() => <GuestDrawerHistory target={target} range="1h" />);
const refresh = () => fireEvent.click(screen.getByRole('button', { name: 'Refresh history' }));

describe('drawer History does not join missing stored observations', () => {
  it('breaks only the affected metric and keeps isolated recovery observations visible', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(paths('#8b5cf6')[0].getAttribute('d')?.match(/L/g)).toHaveLength(4);
    expect(paths('#f59e0b')).toHaveLength(1);
    expect(paths('#f59e0b')[0].getAttribute('d')?.match(/L/g)).toHaveLength(1);
    expect(marks('memory')).toHaveLength(1);
    expect(paths('#10b981')).toHaveLength(0);
    expect(marks('disk')).toHaveLength(2);
    expect(note()).toHaveTextContent('Missing observations: Memory, Disk.');
    expect(note()).toHaveTextContent(
      'Lines stop where a series has no reading at another stored time in this panel.',
    );
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('retains isolated measured zero, not an invented zero during a gap', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 0), point(1, 1), point(2, 2)],
        memory: [point(0, 0), point(2, 0)],
      }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(paths('#f59e0b')).toHaveLength(0);
    expect(marks('memory')).toHaveLength(2);
    const slider = within(group()).getByRole('slider');
    fireEvent.input(slider, { target: { value: '1' } });
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      expect.stringContaining('Memory no observation.'),
    );
    expect(group()).toHaveTextContent('Memory-');
    fireEvent.input(slider, { target: { value: '2' } });
    expect(group()).toHaveTextContent('Memory0.0%');
  });

  it('does not fabricate a trend for interleaved, independently sampled series', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 10), point(2, 20)],
        memory: [point(1, 30), point(3, 40)],
      }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(group().querySelectorAll('path')).toHaveLength(0);
    expect(marks('cpu')).toHaveLength(2);
    expect(marks('memory')).toHaveLength(2);
    expect(note()).toHaveTextContent('Missing observations: CPU, Memory.');
  });

  it('preserves multiple continuous segments without dropping either side of the gap', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [0, 1, 2, 3, 4].map((minute) => point(minute, 10)),
        memory: [0, 1, 3, 4].map((minute) => point(minute, 20)),
      }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(paths('#f59e0b')).toHaveLength(2);
    for (const path of paths('#f59e0b'))
      expect(path.getAttribute('d')?.match(/L/g)).toHaveLength(1);
    expect(marks('memory')).toHaveLength(0);
    expect(within(group()).getByRole('img')).toHaveAccessibleDescription(
      expect.stringContaining('Missing observations: Memory.'),
    );
  });

  it('applies the same rule to network and disk I/O, retaining valid zero rates', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        netin: [point(0, 0), point(1, 100), point(2, 200)],
        netout: [point(0, 0), point(2, 0)],
        diskread: [point(0, 0), point(2, 0)],
        diskwrite: [point(0, 0), point(1, 50), point(2, 100)],
      }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Network I/O history' });
    expect(paths('#10b981', 'network')).toHaveLength(1);
    expect(paths('#fb923c', 'network')).toHaveLength(0);
    expect(marks('netout', 'network')).toHaveLength(2);
    expect(note('network')).toHaveTextContent('Missing observations: Out.');
    expect(paths('#3b82f6', 'disk-io')).toHaveLength(0);
    expect(marks('diskread', 'disk-io')).toHaveLength(2);
    expect(paths('#f59e0b', 'disk-io')).toHaveLength(1);
    expect(note('disk-io')).toHaveTextContent('Missing observations: Read.');
  });

  it('does not guess missed polls from elapsed time or timestamps in other panels', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 10), point(59, 20)],
        memory: [point(0, 30), point(59, 40)],
        netin: [point(20, 100)],
        unconfigured: [point(30, 100)],
      }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(paths('#8b5cf6')).toHaveLength(1);
    expect(paths('#f59e0b')).toHaveLength(1);
    expect(note()).toBeNull();
  });

  it('does not create a gap from invalid samples or an entirely absent series', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(0, 10), point(1, NaN), point(2, 20)],
        memory: [point(0, 30), point(2, 40)],
      }),
    );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(paths('#8b5cf6')).toHaveLength(1);
    expect(paths('#f59e0b')).toHaveLength(1);
    expect(note()).toBeNull();
    expect(marks('disk')).toHaveLength(0);
  });

  it('cannot close a stored gap with live recovery or retained numeric carriers', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    const [live, setLive] = createSignal(false);
    render(() => (
      <GuestDrawerHistory
        target={target}
        range="1h"
        currentMetrics={live() ? { memory: 99, disk: 99 } : {}}
        deferredMetrics={live() ? {} : { memory: { lastKnownValue: 25, message: 'Last known.' } }}
      />
    ));
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = group();
    expect(marks('memory')).toHaveLength(1);
    setLive(true);
    expect(group()).toBe(chart);
    expect(marks('memory')).toHaveLength(1);
    expect(paths('#10b981')).toHaveLength(0);
    expect(note()).toHaveTextContent('Memory, Disk');
    expect(group()).not.toHaveTextContent('99.0%');
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it('closes a gap only with matching stored observations and preserves dated inspection', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockResolvedValueOnce(
        response({
          cpu: [0, 1, 2, 3, 4].map((minute) => point(minute, 10)),
          memory: [0, 1, 2, 3, 4].map((minute) => point(minute, 20)),
        }),
      );
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const chart = group();
    const slider = within(chart).getByRole('slider');
    fireEvent.input(slider, { target: { value: '2' } });
    expect(slider).toHaveAttribute(
      'aria-valuetext',
      expect.stringContaining('Memory no observation.'),
    );
    refresh();
    await waitFor(() => expect(note()).toBeNull());
    expect(group()).toBe(chart);
    expect(paths('#f59e0b')).toHaveLength(1);
    expect(paths('#f59e0b')[0].getAttribute('d')?.match(/L/g)).toHaveLength(4);
    expect(slider).toHaveValue('2');
    expect(slider).toHaveAttribute('aria-valuetext', expect.stringContaining('Memory 20.0%.'));
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it('retains gaps on a transient refresh failure, but withdraws them on access denial', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockRejectedValueOnce(Object.assign(new Error('private transient detail'), { status: 503 }))
      .mockRejectedValueOnce(Object.assign(new Error('private denial'), { status: 403 }));
    mount();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    refresh();
    await screen.findByRole('button', { name: 'Retry history' });
    expect(note()).toHaveTextContent('Memory, Disk');
    expect(paths('#10b981')).toHaveLength(0);
    fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
    await waitFor(() => expect(screen.queryByTestId('guest-history-group-chart')).toBeNull());
    expect(document.querySelector('[data-history-gaps]')).toBeNull();
    expect(document.body).not.toHaveTextContent(/private transient detail|private denial/);
  });
});
