import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
import {
  createNonSuspendingQuery,
  resetCreateNonSuspendingQueryCacheForTest,
} from '@/hooks/createNonSuspendingQuery';
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

const response = (
  resourceId: string,
  range = '24h',
  value = 88,
  resourceType = 'agent',
): AllMetricsHistoryResponse => ({
  resourceType,
  resourceId,
  range,
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

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

const paths = (container: HTMLElement) =>
  container.querySelectorAll('[data-testid="guest-history-plot"] path');

describe('GuestDrawerHistory source isolation', () => {
  it.each(['resource', 'resource type', 'range'] as const)(
    'clears old points while an uncached %s replacement is loading',
    async (change) => {
      const pending = deferred<AllMetricsHistoryResponse>();
      const fetch = vi
        .spyOn(ChartsAPI, 'getMetricsHistory')
        .mockResolvedValueOnce(response('host-old'))
        .mockReturnValueOnce(pending.promise);
      const [target, setTarget] = createSignal<GuestDrawerHistoryTarget>({
        resourceType: 'agent',
        resourceId: 'host-old',
      });
      const [range, setRange] = createSignal<HistoryTimeRange>('24h');
      const { container } = render(() => <GuestDrawerHistory target={target()} range={range()} />);
      await waitFor(() => expect(paths(container)).toHaveLength(1));

      if (change === 'resource') setTarget({ resourceType: 'agent', resourceId: 'pbs-service' });
      if (change === 'resource type') setTarget({ resourceType: 'vm', resourceId: 'host-old' });
      if (change === 'range') setRange('6h');
      await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));

      expect(paths(container)).toHaveLength(0);
      expect(screen.getAllByText('Loading history').length).toBeGreaterThan(0);
      pending.resolve(response(target().resourceId, range(), 12, target().resourceType));
      await waitFor(() => expect(paths(container)).toHaveLength(1));
      expect(screen.queryByText('Loading history')).not.toBeInTheDocument();
    },
  );

  it('does not cache former-host points under a failed replacement target', async () => {
    const pending = deferred<AllMetricsHistoryResponse>();
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response('host-old'))
      .mockReturnValueOnce(pending.promise)
      .mockImplementation(() => new Promise(() => {}));
    const [target, setTarget] = createSignal<GuestDrawerHistoryTarget>({
      resourceType: 'agent',
      resourceId: 'host-old',
    });
    const first = render(() => <GuestDrawerHistory target={target()} range="24h" />);
    await waitFor(() => expect(paths(first.container)).toHaveLength(1));
    setTarget({ resourceType: 'agent', resourceId: 'pbs-service' });
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    pending.reject(new Error('Replacement unavailable'));
    await screen.findByText('Failed to load history data');
    first.unmount();

    // Read the same public query/cache API as a remount. A cached failure may
    // be retained, but it must not contain former-host observations.
    const CacheProbe = () => {
      const query = createNonSuspendingQuery<AllMetricsHistoryResponse, string>({
        source: () => 'pbs-service',
        cacheKey: (id) => `guest-drawer-history:agent:${id}:24h`,
        fetcher: () => new Promise(() => {}),
        initialValue: { ...response(''), metrics: {} },
      });
      return <output data-testid="cached-resource">{query.value().resourceId}</output>;
    };
    render(() => <CacheProbe />);
    expect(screen.getByTestId('cached-resource')).toBeEmptyDOMElement();
  });

  it('aborts a superseded read and ignores late success and failure from earlier targets', async () => {
    const old = deferred<AllMetricsHistoryResponse>();
    const middle = deferred<AllMetricsHistoryResponse>();
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockReturnValueOnce(old.promise)
      .mockReturnValueOnce(middle.promise)
      .mockResolvedValueOnce(response('current-host', '24h', 12));
    const [target, setTarget] = createSignal<GuestDrawerHistoryTarget>({
      resourceType: 'agent',
      resourceId: 'old-host',
    });
    const { container } = render(() => <GuestDrawerHistory target={target()} range="24h" />);
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    const firstSignal = fetch.mock.calls[0][0].signal;
    expect(firstSignal).toBeInstanceOf(AbortSignal);
    setTarget({ resourceType: 'agent', resourceId: 'middle-host' });
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    expect(firstSignal!.aborted).toBe(true);
    setTarget({ resourceType: 'agent', resourceId: 'current-host' });
    await waitFor(() => expect(paths(container)).toHaveLength(1));
    const currentPath = paths(container)[0].getAttribute('d');
    old.resolve(response('old-host'));
    middle.reject(new Error('Late failure'));
    await old.promise;
    await middle.promise.catch(() => undefined);
    await Promise.resolve();

    expect(paths(container)[0]).toHaveAttribute('d', currentPath);
    expect(screen.queryByText('Failed to load history data')).not.toBeInTheDocument();
    expect(fetch.mock.calls[1][0].signal!.aborted).toBe(true);
  });

  it('keeps only matching cached history when revisiting a resource and range', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response('host-old'))
      .mockResolvedValueOnce(response('host-new', '24h', 12))
      .mockImplementation(() => new Promise(() => {}));
    const [target, setTarget] = createSignal<GuestDrawerHistoryTarget>({
      resourceType: 'agent',
      resourceId: 'host-old',
    });
    const { container } = render(() => <GuestDrawerHistory target={target()} range="24h" />);
    await waitFor(() => expect(paths(container)).toHaveLength(1));
    const oldPath = paths(container)[0].getAttribute('d');
    setTarget({ resourceType: 'agent', resourceId: 'host-new' });
    await waitFor(() => expect(paths(container)[0]).not.toHaveAttribute('d', oldPath));
    setTarget({ resourceType: 'agent', resourceId: 'host-old' });
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));

    expect(paths(container)[0]).toHaveAttribute('d', oldPath);
    expect(screen.queryByText('Loading history')).not.toBeInTheDocument();
  });

  it('retains same-source points during background polling without a loading flash', async () => {
    vi.useFakeTimers();
    const pending = deferred<AllMetricsHistoryResponse>();
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(response('host-old'))
      .mockReturnValueOnce(pending.promise);
    const target: GuestDrawerHistoryTarget = { resourceType: 'agent', resourceId: 'host-old' };
    const { container } = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await vi.advanceTimersByTimeAsync(0);
    expect(paths(container)).toHaveLength(1);
    const originalPath = paths(container)[0].getAttribute('d');
    await vi.advanceTimersByTimeAsync(30_000);
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(paths(container)[0]).toHaveAttribute('d', originalPath);
    expect(screen.queryByText('Loading history')).not.toBeInTheDocument();
    pending.resolve(response('host-old', '24h', 12));
    await vi.advanceTimersByTimeAsync(0);
    expect(paths(container)[0]).not.toHaveAttribute('d', originalPath);
  });

  it('aborts and rejects late points when the history target becomes unavailable', async () => {
    const pending = deferred<AllMetricsHistoryResponse>();
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockReturnValue(pending.promise);
    const [target, setTarget] = createSignal<GuestDrawerHistoryTarget | null>({
      resourceType: 'agent',
      resourceId: 'host-old',
    });
    const { container } = render(() => <GuestDrawerHistory target={target()} range="24h" />);
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    setTarget(null);
    await screen.findByText('History unavailable');
    expect(fetch.mock.calls[0][0].signal!.aborted).toBe(true);
    pending.resolve(response('host-old'));
    await pending.promise;
    await Promise.resolve();
    expect(paths(container)).toHaveLength(0);
    expect(screen.getByText('History unavailable')).toBeInTheDocument();
  });

  it('aborts a pending read when its range is locked or its drawer unmounts', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockImplementation(() => new Promise(() => {}));
    const [range, setRange] = createSignal<HistoryTimeRange>('24h');
    const target: GuestDrawerHistoryTarget = { resourceType: 'agent', resourceId: 'host-old' };
    const view = render(() => <GuestDrawerHistory target={target} range={range()} />);
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    setRange('14d');
    await screen.findByText(/14 days history requires a higher license plan/);
    expect(fetch.mock.calls[0][0].signal).toBeInstanceOf(AbortSignal);
    expect(fetch.mock.calls[0][0].signal!.aborted).toBe(true);
    expect(fetch).toHaveBeenCalledTimes(1);
    setRange('24h');
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    const active = fetch.mock.calls[1][0].signal;
    expect(active!.aborted).toBe(false);
    view.unmount();
    expect(active!.aborted).toBe(true);
  });
});
