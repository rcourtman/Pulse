import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
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

const target: GuestDrawerHistoryTarget = { resourceType: 'agent', resourceId: 'pbs-host' };
const response = (value = 42, resourceId = target.resourceId): AllMetricsHistoryResponse => ({
  resourceType: 'agent',
  resourceId,
  range: '24h',
  start: 1_700_000_000_000,
  end: 1_700_000_060_000,
  source: 'store',
  metrics: {
    cpu: [
      { timestamp: 1_700_000_000_000, value: value - 1, min: value - 1, max: value - 1 },
      { timestamp: 1_700_000_060_000, value, min: value, max: value },
    ],
  },
});
const paths = (container: HTMLElement) =>
  container.querySelectorAll('[data-testid="guest-history-plot"] path');

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

describe('GuestDrawerHistory refresh recovery', () => {
  it('offers one scoped retry after initial failure without claiming collection or exposing errors', async () => {
    const pending = deferred<AllMetricsHistoryResponse>();
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockRejectedValueOnce(new Error('private diagnostic detail'))
      .mockReturnValueOnce(pending.promise);
    const { container } = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByText('Failed to load history data');
    const retry = screen.getByRole('button', { name: 'Retry history' });
    expect(screen.queryByText('Collecting history')).not.toBeInTheDocument();
    expect(screen.queryByText(/previously loaded/)).not.toBeInTheDocument();
    expect(container).not.toHaveTextContent('private diagnostic detail');
    expect(paths(container)).toHaveLength(0);
    fireEvent.click(retry);
    fireEvent.click(retry);
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    expect(retry).toHaveAttribute('aria-disabled', 'true');
    expect(retry).toHaveAttribute('aria-busy', 'true');
    expect(fetch.mock.calls[1][0]).toMatchObject({ ...target, range: '24h' });
    pending.resolve(response());
    await waitFor(() => expect(paths(container)).toHaveLength(1));
    expect(retry).toHaveAttribute('aria-disabled', 'false');
    expect(retry).toHaveAccessibleName('Refresh history');
  });

  it('retains only the current history on a failed poll and updates its already-mounted status region', async () => {
    vi.useFakeTimers();
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockRejectedValueOnce(new Error('Unavailable'));
    const { container } = render(() => <GuestDrawerHistory target={target} range="24h" />);
    const status = screen.getByRole('status', { name: 'History refresh status' });
    expect(status).toBeEmptyDOMElement();
    await vi.advanceTimersByTimeAsync(0);
    const original = paths(container)[0].getAttribute('d');
    await vi.advanceTimersByTimeAsync(30_000);
    expect(paths(container)).toHaveLength(1);
    expect(paths(container)[0]).toHaveAttribute('d', original);
    expect(status).toHaveTextContent('History refresh failed. Showing previously loaded history.');
    expect(status).toHaveAttribute('aria-live', 'polite');
    expect(status).toHaveAttribute('aria-atomic', 'true');
    expect(screen.queryByText('Loading history')).not.toBeInTheDocument();
  });

  it('keeps retry keyboard focus through successful recovery without losing retained points', async () => {
    vi.useFakeTimers();
    const pending = deferred<AllMetricsHistoryResponse>();
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockRejectedValueOnce(new Error('Unavailable'))
      .mockReturnValueOnce(pending.promise);
    const { container } = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await vi.advanceTimersByTimeAsync(30_000);
    const original = paths(container)[0].getAttribute('d');
    const retry = screen.getByRole('button', { name: 'Retry history' });
    retry.focus();
    fireEvent.click(retry);
    expect(paths(container)[0]).toHaveAttribute('d', original);
    expect(retry).toHaveFocus();
    pending.resolve(response(12));
    await vi.advanceTimersByTimeAsync(0);
    expect(paths(container)[0]).not.toHaveAttribute('d', original);
    expect(screen.getByRole('button', { name: 'Refresh history' })).toBe(retry);
    expect(retry).toHaveFocus();
    expect(screen.getByRole('status', { name: 'History refresh status' })).toBeEmptyDOMElement();
  });

  it('settles retry loading when a background poll supersedes a slow manual refresh', async () => {
    vi.useFakeTimers();
    const pending = deferred<AllMetricsHistoryResponse>();
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response())
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValueOnce(response(12));
    const { container } = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await vi.advanceTimersByTimeAsync(0);
    const refresh = screen.getByRole('button', { name: 'Refresh history' });
    fireEvent.click(refresh);
    expect(refresh).toHaveAttribute('aria-disabled', 'true');
    await vi.advanceTimersByTimeAsync(30_000);
    expect(fetch).toHaveBeenCalledTimes(3);
    expect(fetch.mock.calls[1][0].signal!.aborted).toBe(true);
    expect(refresh).toHaveAttribute('aria-disabled', 'false');
    expect(refresh).toHaveAttribute('aria-busy', 'false');
    const latest = paths(container)[0].getAttribute('d');
    pending.resolve(response(99));
    await vi.advanceTimersByTimeAsync(0);
    expect(paths(container)[0]).toHaveAttribute('d', latest);
  });

  it('retains the failure after a failed retry and allows another attempt', async () => {
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockRejectedValueOnce(new Error('Unavailable'))
      .mockRejectedValueOnce(new Error('Still unavailable'))
      .mockResolvedValueOnce(response());
    const { container } = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByText('Failed to load history data');
    const retry = screen.getByRole('button', { name: 'Retry history' });
    fireEvent.click(retry);
    await waitFor(() => expect(retry).toHaveAttribute('aria-disabled', 'false'));
    expect(screen.getByText('Failed to load history data')).toBeInTheDocument();
    expect(paths(container)).toHaveLength(0);
    fireEvent.click(retry);
    await waitFor(() => expect(paths(container)).toHaveLength(1));
  });

  it.each(['resource', 'range'] as const)(
    'does not label a failed replacement %s as retained history',
    async (change) => {
      vi.useFakeTimers();
      const fetch = vi
        .spyOn(ChartsAPI, 'getMetricsHistory')
        .mockResolvedValueOnce(response())
        .mockRejectedValueOnce(new Error('Poll failed'))
        .mockRejectedValueOnce(new Error('Replacement failed'))
        .mockResolvedValueOnce(response(12, 'replacement'));
      const [currentTarget, setTarget] = createSignal<GuestDrawerHistoryTarget>(target);
      const [range, setRange] = createSignal<HistoryTimeRange>('24h');
      const { container } = render(() => (
        <GuestDrawerHistory target={currentTarget()} range={range()} />
      ));
      await vi.advanceTimersByTimeAsync(30_000);
      expect(paths(container)).toHaveLength(1);
      if (change === 'resource') setTarget({ ...target, resourceId: 'replacement' });
      else setRange('6h');
      await vi.advanceTimersByTimeAsync(0);
      expect(paths(container)).toHaveLength(0);
      expect(screen.queryByText(/previously loaded/)).not.toBeInTheDocument();
      expect(screen.getByText('Failed to load history data')).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: 'Retry history' }));
      await vi.advanceTimersByTimeAsync(0);
      expect(fetch.mock.calls[3][0]).toMatchObject({ ...currentTarget(), range: range() });
      expect(paths(container)).toHaveLength(1);
    },
  );

  it.each(['empty', 'invalid'] as const)(
    'does not present %s history or current readings as previously loaded observations',
    async (observation) => {
      vi.useFakeTimers();
      vi.spyOn(ChartsAPI, 'getMetricsHistory')
        .mockResolvedValueOnce({
          ...response(),
          metrics:
            observation === 'empty'
              ? {}
              : { cpu: [{ timestamp: NaN, value: NaN, min: NaN, max: NaN }] },
        })
        .mockRejectedValueOnce(new Error('Unavailable'));
      const { container } = render(() => (
        <GuestDrawerHistory target={target} range="24h" currentMetrics={{ cpu: 42 }} />
      ));
      await vi.advanceTimersByTimeAsync(30_000);
      expect(screen.getByText('Failed to load history data')).toBeInTheDocument();
      expect(screen.queryByText(/previously loaded/)).not.toBeInTheDocument();
      expect(screen.queryByText('Collecting history')).not.toBeInTheDocument();
      expect(paths(container)).toHaveLength(0);
    },
  );

  it('offers no refresh when the target is absent or the range is locked', async () => {
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory');
    const [currentTarget, setTarget] = createSignal<GuestDrawerHistoryTarget | null>(null);
    render(() => <GuestDrawerHistory target={currentTarget()} range="14d" />);
    expect(screen.queryByRole('button', { name: /history/i })).not.toBeInTheDocument();
    setTarget(target);
    await screen.findByText(/14 days history requires a higher license plan/);
    expect(screen.queryByRole('button', { name: /history/i })).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });
});
