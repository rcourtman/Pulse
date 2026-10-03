import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { cleanup, render } from '@solidjs/testing-library';
import { ChartsAPI } from '@/api/charts';
import type { HistoryChartProps } from '../historyChartModel';
import { useHistoryChartState, type HistoryChartState } from '../useHistoryChartState';

vi.mock('@/stores/license', () => ({
  isRangeLocked: (range: string) => range === '90d',
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 7,
}));
vi.mock('@/api/charts', () => ({ ChartsAPI: { getMetricsHistory: vi.fn() } }));
const request = vi.mocked(ChartsAPI.getMetricsHistory);
const points = (value: number) => [{ timestamp: 1000, value, min: value, max: value }];
const settle = async () => {
  await Promise.resolve();
  await Promise.resolve();
};
function deferred() {
  let resolve!: (result: Awaited<ReturnType<typeof ChartsAPI.getMetricsHistory>>) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<Awaited<ReturnType<typeof ChartsAPI.getMetricsHistory>>>(
    (yes, no) => {
      resolve = yes;
      reject = no;
    },
  );
  return { promise, resolve, reject };
}
function mount() {
  const [props, setProps] = createSignal<HistoryChartProps>({
    resourceId: 'a',
    resourceType: 'agent',
    metric: 'cpu',
    range: '1h',
  });
  let state!: HistoryChartState;
  const view = render(() => {
    state = useHistoryChartState(
      {
        get resourceId() {
          return props().resourceId;
        },
        get resourceType() {
          return props().resourceType;
        },
        get metric() {
          return props().metric;
        },
        get range() {
          return props().range;
        },
        get data() {
          return props().data;
        },
      },
      { getCanvas: () => undefined, getContainer: () => undefined },
    );
    return <div />;
  });
  return {
    state,
    unmount: view.unmount,
    change: (next: Partial<HistoryChartProps>) => setProps((p) => ({ ...p, ...next })),
  };
}

describe('History request ownership', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    request.mockReset();
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it.each(['success', 'failure'])(
    'ignores a superseded %s even when cancellation is ignored',
    async (completion) => {
      const old = deferred(),
        current = deferred();
      request.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
      const { state, change } = mount();
      const signal = request.mock.calls[0][0].signal!;
      change({ resourceId: 'b' });
      expect(signal.aborted).toBe(true);
      current.resolve({ points: points(80), source: 'store' } as never);
      await settle();
      if (completion === 'success') old.resolve({ points: points(10), source: 'store' } as never);
      else old.reject(new Error('old error'));
      await settle();
      expect(state.data()).toEqual(points(80));
      expect(state.error()).toBeNull();
      expect(state.loading()).toBe(false);
    },
  );

  it.each([{ resourceId: 'b' }, { resourceType: 'node' }, { metric: 'memory' }, { range: '6h' }])(
    'clears old readings on selection change %j and exposes its failure',
    async (next) => {
      request.mockResolvedValueOnce({ points: points(10), source: 'store' } as never);
      const { state, change } = mount();
      await settle();
      const current = deferred();
      request.mockReturnValueOnce(current.promise);
      change(next as Partial<HistoryChartProps>);
      expect(state.data()).toEqual([]);
      expect(state.loading()).toBe(true);
      vi.spyOn(console, 'error').mockImplementation(() => {});
      current.reject(new Error('current failed'));
      await settle();
      expect(state.error()).toBe('Failed to load history data');
      expect(state.data()).toEqual([]);
    },
  );

  it.each([{ samples: points(10) }, { samples: [] }])(
    'retains the last result %j and exposes refresh failure until recovery',
    async ({ samples }) => {
      const initial = deferred();
      request.mockReturnValueOnce(initial.promise);
      const { state } = mount();
      vi.advanceTimersByTime(120_000);
      expect(request).toHaveBeenCalledTimes(1);
      initial.resolve({ points: samples, source: 'store' } as never);
      await settle();
      const refresh = deferred();
      request.mockReturnValueOnce(refresh.promise);
      vi.advanceTimersByTime(120_000);
      expect(request).toHaveBeenCalledTimes(2);
      vi.spyOn(console, 'error').mockImplementation(() => {});
      refresh.reject(new Error('refresh failed'));
      await settle();
      expect(state.data()).toEqual(samples);
      expect(state.error()).toBeNull();
      expect(state.refreshFailed()).toBe(true);
      expect(state.source()).toBe('store');
      expect(state.loading()).toBe(false);
      const recovery = deferred();
      request.mockReturnValueOnce(recovery.promise);
      vi.advanceTimersByTime(10_000);
      expect(state.refreshFailed()).toBe(true);
      recovery.resolve({ points: points(30), source: 'memory' } as never);
      await settle();
      expect(state.refreshFailed()).toBe(false);
      expect(state.data()).toEqual(points(30));
      expect(state.source()).toBe('memory');
    },
  );

  it.each([{ resourceId: 'b' }, { range: '6h' }, { data: [] }, { range: '90d' }])(
    'clears refresh failure on selection replacement %j',
    async (next) => {
      request.mockResolvedValueOnce({ points: points(10), source: 'store' } as never);
      const { state, change } = mount();
      await settle();
      vi.spyOn(console, 'error').mockImplementation(() => {});
      request.mockRejectedValueOnce(new Error('refresh failed'));
      await vi.advanceTimersByTimeAsync(10_000);
      expect(state.refreshFailed()).toBe(true);
      request.mockReturnValueOnce(deferred().promise);
      change(next as Partial<HistoryChartProps>);
      expect(state.refreshFailed()).toBe(false);
      expect(state.data()).toEqual([]);
    },
  );

  it('cancels fetched data when supplied data takes ownership, including empty samples', async () => {
    const old = deferred();
    request.mockReturnValueOnce(old.promise);
    const { state, change } = mount();
    change({ data: [] });
    expect(request.mock.calls[0][0].signal?.aborted).toBe(true);
    old.resolve({ points: points(10), source: 'store' } as never);
    await settle();
    vi.advanceTimersByTime(120_000);
    expect(request).toHaveBeenCalledTimes(1);
    expect(state.data()).toEqual([]);
    expect(state.source()).toBe('live');
    change({ data: points(80) });
    expect(state.data()).toEqual(points(80));
    request.mockResolvedValueOnce({ points: points(30), source: 'store' } as never);
    change({ data: undefined });
    await settle();
    expect(state.data()).toEqual(points(30));
  });

  it.each([{ range: '90d' }, { resourceId: '' }])(
    'invalidates in-flight work for unavailable selection %j',
    async (next) => {
      const old = deferred();
      request.mockReturnValueOnce(old.promise);
      const { state, change } = mount();
      change(next as Partial<HistoryChartProps>);
      old.resolve({ points: points(10), source: 'store' } as never);
      await settle();
      vi.advanceTimersByTime(120_000);
      expect(request).toHaveBeenCalledTimes(1);
      expect(state.data()).toEqual([]);
      expect(state.loading()).toBe(false);
    },
  );

  it('aborts and stops polling on unmount without consuming a late completion', async () => {
    const old = deferred();
    request.mockReturnValueOnce(old.promise);
    const { state, unmount } = mount();
    unmount();
    expect(request.mock.calls[0][0].signal?.aborted).toBe(true);
    old.resolve({ points: points(10), source: 'store' } as never);
    await settle();
    vi.advanceTimersByTime(120_000);
    expect(request).toHaveBeenCalledTimes(1);
    expect(state.data()).toEqual([]);
  });
});

describe('History access errors', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    request.mockReset();
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });
  it.each([
    [401, 'Sign in again or check your API token.'],
    [403, 'Access denied. Check your permissions and license plan.'],
  ])(
    'withdraws stored samples and source on status %s until a successful read',
    async (status, message) => {
      request.mockResolvedValueOnce({ points: points(42), source: 'store' } as never);
      const { state } = mount();
      await settle();
      request.mockRejectedValueOnce(
        Object.assign(new Error('private transport detail'), { status }),
      );
      await vi.advanceTimersByTimeAsync(10_000);
      expect(state.data()).toEqual([]);
      expect(state.source()).toBeNull();
      expect(state.refreshFailed()).toBe(false);
      expect(state.error()).toBe(message);
      const retry = deferred();
      request.mockReturnValueOnce(retry.promise);
      vi.advanceTimersByTime(10_000);
      expect(state.data()).toEqual([]);
      expect(state.error()).toBe(message);
      retry.resolve({ points: points(12), source: 'store' } as never);
      await settle();
      expect(state.data()).toEqual(points(12));
      expect(state.error()).toBeNull();
      expect(state.source()).toBe('store');
    },
  );
});
