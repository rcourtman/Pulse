import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import { GUEST_DRAWER_HISTORY_POLL_MS } from '../guestDrawerHistoryQuery';

vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(async () => undefined),
  maxHistoryDays: () => 90,
  isRangeLocked: () => false,
}));
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
});
afterEach(() => {
  vi.useRealTimers();
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
});

const start = Date.UTC(2026, 9, 12, 12);
const target = { resourceType: 'vm' as const, resourceId: 'fixture:pve:101' };
const point = (minute: number, value: number) => ({
  timestamp: start + minute * 60_000,
  value,
  min: value,
  max: value,
});
const response = (
  metrics: AllMetricsHistoryResponse['metrics'] = {
    cpu: [point(0, 12), point(2, 22)],
    memory: [point(1, 95)],
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
const readoutTime = (id = 'utilization') =>
  within(group(id)).getByTestId('guest-history-hover-time');
const refresh = () => vi.advanceTimersByTimeAsync(GUEST_DRAWER_HISTORY_POLL_MS);
const ready = () => screen.findByRole('slider', { name: 'Inspect Utilization history' });

describe('drawer History default readout belongs to one stored time', () => {
  it.each(['vm', 'system-container'] as const)(
    'does not combine differently dated %s readings or borrow live fallback values',
    async (resourceType) => {
      const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
      render(() => (
        <GuestDrawerHistory
          target={{ ...target, resourceType }}
          range="1h"
          currentMetrics={{ cpu: 99, memory: 98, disk: 88 }}
        />
      ));
      await ready();
      expect(group()).toHaveTextContent('CPU22.0%');
      expect(group()).toHaveTextContent('Memoryno reading');
      expect(group()).toHaveTextContent('Diskno reading');
      expect(group()).not.toHaveTextContent(/95.0%|98.0%|88.0%|99.0%/);
      expect(group().querySelector('[data-history-current]')).toBeNull();
      expect(readoutTime()).toHaveTextContent('Latest stored');
      expect(readoutTime()).toHaveAttribute('datetime', new Date(start + 120_000).toISOString());
      expect(within(group()).getByRole('img')).toHaveAccessibleDescription(
        expect.stringContaining('CPU 22.0%. Memory no observation. Disk no observation.'),
      );
      expect(fetch).toHaveBeenCalledTimes(1);
    },
  );

  it('keeps aligned measured zero and a single real observation without a trend', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({ cpu: [point(1, 0)], memory: [point(1, 0)], disk: [point(1, 0)] }),
    );
    render(() => <GuestDrawerHistory target={target} range="1h" />);
    await screen.findByText('Single observation. No trend yet.');
    expect(group()).toHaveTextContent('CPU0.0%');
    expect(group()).toHaveTextContent('Memory0.0%');
    expect(group()).toHaveTextContent('Disk0.0%');
    expect(readoutTime()).toHaveAttribute('datetime', new Date(start + 60_000).toISOString());
    expect(group().querySelector('path[data-history-series]')).toBeNull();
    expect(group().querySelectorAll('[data-history-observation]')).toHaveLength(3);
  });

  it('retains separately qualified fallback only when the panel has no stored observations', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response({}));
    render(() => (
      <GuestDrawerHistory
        target={target}
        range="1h"
        currentMetrics={{ cpu: 0 }}
        deferredMetrics={{
          memory: { lastKnownValue: 95, message: 'Original memory time retained.' },
          disk: { lastKnownValue: 50, valueLabel: 'freshness unknown', message: 'Time unknown.' },
        }}
      />
    ));
    await screen.findAllByText('No stored history in this range');
    expect(group().querySelector('[data-history-current="cpu"]')).toHaveTextContent(
      'CPU0.0%current',
    );
    expect(group().querySelector('[data-history-last-known="memory"]')).toHaveTextContent(
      'Memory95.0%last known',
    );
    expect(group().querySelector('[data-history-unknown="disk"]')).toHaveTextContent(
      'Disk50.0%freshness unknown',
    );
    expect(within(group()).queryByTestId('guest-history-hover-time')).toBeNull();
    fetch.mockResolvedValue(response({ cpu: [point(2, 22)] }));
    await refresh();
    await waitFor(() => expect(group()).toHaveTextContent('CPU22.0%'));
    expect(group()).toHaveTextContent('Memoryno reading');
    expect(group()).toHaveTextContent('Diskno reading');
    expect(group().querySelector('[data-history-last-known]')).toBeNull();
    expect(group().querySelector('[data-history-unknown]')).toBeNull();
  });

  it('returns to the newest stored time after inspection and never follows a live snapshot', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response());
    const [live, setLive] = createSignal(88);
    render(() => (
      <GuestDrawerHistory target={target} range="1h" currentMetrics={{ disk: live() }} />
    ));
    const slider = await ready();
    slider.focus();
    fireEvent.input(slider, { target: { value: '1' } });
    expect(group()).toHaveTextContent('Memory95.0%');
    expect(group()).toHaveTextContent('CPUno reading');
    fireEvent.blur(slider);
    expect(group()).toHaveTextContent('CPU22.0%');
    expect(group()).toHaveTextContent('Memoryno reading');
    expect(readoutTime()).toHaveTextContent('Latest stored');
    setLive(77);
    expect(group()).not.toHaveTextContent('77.0%');
    expect(fetch).toHaveBeenCalledTimes(1);
    fetch.mockResolvedValue(response({ cpu: [point(2, 22)], memory: [point(3, 25)] }));
    await refresh();
    await waitFor(() => expect(group()).toHaveTextContent('Memory25.0%'));
    expect(group()).toHaveTextContent('CPUno reading');
    expect(readoutTime()).toHaveAttribute('datetime', new Date(start + 180_000).toISOString());
  });

  it("uses each panel's own time and preserves real zero rates", async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        cpu: [point(5, 22)],
        netin: [point(0, 200)],
        netout: [point(2, 0)],
        diskread: [point(3, 0)],
        diskwrite: [point(1, 100)],
      }),
    );
    render(() => <GuestDrawerHistory target={target} range="1h" />);
    await screen.findByRole('slider', { name: 'Inspect Network I/O history' });
    expect(group('network')).toHaveTextContent('Inno reading');
    expect(group('network')).toHaveTextContent('Out0 B/s');
    expect(readoutTime('network')).toHaveAttribute(
      'datetime',
      new Date(start + 120_000).toISOString(),
    );
    expect(group('disk-io')).toHaveTextContent('Read0 B/s');
    expect(group('disk-io')).toHaveTextContent('Writeno reading');
    expect(readoutTime('disk-io')).toHaveAttribute(
      'datetime',
      new Date(start + 180_000).toISOString(),
    );
  });

  it('retains the dated readout on a transient failure and removes it when access is denied', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockRejectedValueOnce(Object.assign(new Error('private transient detail'), { status: 503 }))
      .mockRejectedValueOnce(Object.assign(new Error('private access detail'), { status: 403 }));
    render(() => <GuestDrawerHistory target={target} range="1h" />);
    await ready();
    await refresh();
    await screen.findByRole('button', { name: 'Retry history' });
    expect(readoutTime()).toHaveAttribute('datetime', new Date(start + 120_000).toISOString());
    expect(group()).toHaveTextContent('Memoryno reading');
    fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
    await waitFor(() => expect(screen.queryByTestId('guest-history-group-chart')).toBeNull());
    expect(screen.queryByTestId('guest-history-hover-time')).toBeNull();
    expect(document.body).not.toHaveTextContent(/private transient detail|private access detail/);
  });
});
