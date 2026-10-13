import { cleanup, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import DrawerTrendCards, { type DrawerTrendCardConfig } from '../DrawerTrendCards';

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
});

const start = Date.UTC(2026, 9, 12, 12);
const target = { resourceType: 'vm' as const, resourceId: 'fixture:pve:281' };
const point = (hour: number, value: number) => ({
  timestamp: start + hour * 3_600_000,
  value,
  min: value,
  max: value,
});
const response = (metrics: AllMetricsHistoryResponse['metrics']): AllMetricsHistoryResponse => ({
  ...target,
  range: '24h',
  start,
  end: start + 24 * 3_600_000,
  source: 'store',
  metrics,
});
const memoryCard = (overrides: Partial<DrawerTrendCardConfig> = {}): DrawerTrendCardConfig => ({
  metric: 'memory',
  label: 'Memory',
  unit: '%',
  color: '#f59e0b',
  current: 85.4,
  detail: '3.42 GB of 4.00 GB',
  ...overrides,
});
const card = (metric = 'memory') => screen.getByTestId(`drawer-trend-card-${metric}`);

describe('DrawerTrendCards', () => {
  it('reads the 24h history once and shows the live value, the 24h range and the detail', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValue(response({ memory: [point(0, 84), point(12, 84.6), point(23, 85.6)] }));
    render(() => <DrawerTrendCards target={target} enabled cards={[memoryCard()]} />);

    await waitFor(() => expect(card()).toHaveTextContent('24h: 84.0%–85.6%'));
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toMatchObject({ ...target, range: '24h' });
    expect(within(card()).getByText('85.4%')).toBeInTheDocument();
    expect(card()).toHaveTextContent('3.42 GB of 4.00 GB');
    expect(within(card()).getByRole('img')).toHaveAccessibleName(
      'Memory 85.4%, 84.0% to 85.6% over 24 hours',
    );
    // A current reading is not drawn as a trend point; three stored points.
    expect(card().querySelector('path')?.getAttribute('d')?.match(/[ML]/g)).toHaveLength(3);
  });

  it("draws an active alert's own trigger and marks the card with its severity", async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({ memory: [point(0, 84), point(23, 85.6)] }),
    );
    render(() => (
      <DrawerTrendCards
        target={target}
        enabled
        cards={[memoryCard({ alertSeverity: 'warning', alertThreshold: 85 })]}
      />
    ));

    const threshold = await screen.findByTestId('drawer-trend-threshold');
    // 0-100% scale on a 48px plot: 85% sits at 7.2px.
    expect(Number(threshold.getAttribute('y1'))).toBeCloseTo(7.2, 1);
    expect(card()).toHaveClass('border-amber-400');
  });

  it('labels a deferred reading as last known and says why, instead of calling it current', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({ memory: [point(0, 60), point(23, 62)] }),
    );
    render(() => (
      <DrawerTrendCards
        target={target}
        enabled
        cards={[
          memoryCard({
            current: undefined,
            deferred: { lastKnownValue: 62, message: 'Guest reads are paused during a backup.' },
          }),
        ]}
      />
    ));

    expect(within(card()).getByText('last known')).toBeInTheDocument();
    expect(within(card()).getByText('62.0%')).toBeInTheDocument();
    expect(card()).toHaveTextContent('Guest reads are paused during a backup.');
    // The chart's accessible name carries the same qualifier as the visible value.
    expect(await within(card()).findByRole('img')).toHaveAccessibleName(
      'Memory last known 62.0%, 60.0% to 62.0% over 24 hours',
    );
  });

  it('says it is still collecting when no history has been stored', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(response({}));
    render(() => <DrawerTrendCards target={target} enabled cards={[memoryCard()]} />);
    await waitFor(() => expect(card()).toHaveTextContent('Collecting history'));
  });

  it('does not read history while its tab is hidden', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValue(response({ memory: [point(0, 84), point(23, 85)] }));
    const [enabled, setEnabled] = createSignal(false);
    render(() => <DrawerTrendCards target={target} enabled={enabled()} cards={[memoryCard()]} />);

    await Promise.resolve();
    expect(fetch).not.toHaveBeenCalled();
    setEnabled(true);
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  });

  it('states a spike inside an averaged bucket, and does not call one reading steady', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({
        memory: [
          { timestamp: start, value: 50, min: 48, max: 99 },
          { timestamp: start + 3_600_000, value: 51, min: 50, max: 52 },
        ],
        cpu: [point(0, 4)],
      }),
    );
    render(() => (
      <DrawerTrendCards
        target={target}
        enabled
        cards={[memoryCard(), memoryCard({ metric: 'cpu', label: 'CPU', current: 4 })]}
      />
    ));
    await waitFor(() => expect(card()).toHaveTextContent('24h: 48.0%–99.0%'));
    expect(card('cpu')).toHaveTextContent('24h: one reading so far');
  });

  it("widens a temperature scale to the alert's trigger instead of clamping the line", async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue(
      response({ temperature: [point(0, 45), point(12, 55), point(23, 50)] }),
    );
    render(() => (
      <DrawerTrendCards
        target={target}
        enabled
        cards={[
          {
            metric: 'temperature',
            label: 'CPU temperature',
            unit: 'C',
            color: '#ef4444',
            current: 50,
            alertSeverity: 'warning',
            alertThreshold: 85,
          },
        ]}
      />
    ));
    const threshold = await screen.findByTestId('drawer-trend-threshold');
    const thresholdY = Number(threshold.getAttribute('y1'));
    // Clamped to a 45-55°C scale the line would sit on the top edge (0).
    expect(thresholdY).toBeGreaterThan(0);
    const pathYs = (card('temperature').querySelector('path')?.getAttribute('d') ?? '')
      .split(/[ML]/)
      .filter(Boolean)
      .map((pair) => Number(pair.split(',')[1]));
    expect(Math.min(...pathYs)).toBeGreaterThan(thresholdY);
  });

  it('keeps an earlier trend after a failed refresh, and says so', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response({ memory: [point(0, 84), point(23, 85)] }))
      .mockRejectedValue(new Error('boom'));
    const [enabled, setEnabled] = createSignal(true);
    render(() => <DrawerTrendCards target={target} enabled={enabled()} cards={[memoryCard()]} />);
    await waitFor(() => expect(card()).toHaveTextContent('24h: 84.0%–85.0%'));
    setEnabled(false);
    setEnabled(true);
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(card()).toHaveTextContent('Refresh failed. Showing the last loaded trend.'),
    );
    expect(card()).toHaveTextContent('24h: 84.0%–85.0%');
  });

  it('stops polling while hidden and reads again when shown', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      const fetch = vi
        .spyOn(ChartsAPI, 'getMetricsHistory')
        .mockResolvedValue(response({ memory: [point(0, 84), point(23, 85)] }));
      const [enabled, setEnabled] = createSignal(true);
      render(() => <DrawerTrendCards target={target} enabled={enabled()} cards={[memoryCard()]} />);
      const settle = async () => {
        for (let i = 0; i < 5; i += 1) await Promise.resolve();
      };
      await settle();
      expect(fetch).toHaveBeenCalledTimes(1);
      vi.advanceTimersByTime(30_000);
      await settle();
      expect(fetch).toHaveBeenCalledTimes(2);
      setEnabled(false);
      vi.advanceTimersByTime(120_000);
      await settle();
      expect(fetch).toHaveBeenCalledTimes(2);
      setEnabled(true);
      await settle();
      expect(fetch).toHaveBeenCalledTimes(3);
    } finally {
      vi.useRealTimers();
    }
  });

  it('says the trend is unavailable when the read fails, keeping the live value', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory').mockRejectedValue(new Error('boom'));
    render(() => <DrawerTrendCards target={target} enabled cards={[memoryCard()]} />);

    await waitFor(() => expect(card()).toHaveTextContent('Trend unavailable'));
    expect(within(card()).getByText('85.4%')).toBeInTheDocument();
  });
});
