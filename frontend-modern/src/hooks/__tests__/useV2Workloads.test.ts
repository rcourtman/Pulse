import { createEffect, createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type UseV2WorkloadsModule = typeof import('@/hooks/useV2Workloads');

const sampleResource = {
  id: 'cluster-a-pve1-101',
  type: 'vm',
  name: 'vm-101',
  status: 'running',
  lastSeen: '2026-02-06T12:00:00Z',
  vmid: 101,
  node: 'pve1',
  instance: 'cluster-a',
  metrics: {
    cpu: { percent: 25 },
    memory: { used: 2 * 1024, total: 4 * 1024, percent: 50 },
    disk: { used: 20 * 1024, total: 100 * 1024, percent: 20 },
  },
  sources: ['proxmox'],
};

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

const advanceAndFlush = async (ms: number) => {
  vi.advanceTimersByTime(ms);
  await flushAsync();
};

const deferred = <T,>() => {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

const waitForWorkloadCount = async (getCount: () => number, expectedMin = 1) => {
  for (let i = 0; i < 20; i += 1) {
    if (getCount() >= expectedMin) {
      return;
    }
    await flushAsync();
  }
  throw new Error(`Timed out waiting for at least ${expectedMin} workloads`);
};

describe('useV2Workloads', () => {
  let apiFetchJSONMock: ReturnType<typeof vi.fn>;
  let useV2Workloads: UseV2WorkloadsModule['useV2Workloads'];
  let resetV2WorkloadsCacheForTests: UseV2WorkloadsModule['__resetV2WorkloadsCacheForTests'];

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();

    apiFetchJSONMock = vi.fn().mockResolvedValue({
      data: [sampleResource],
      meta: { totalPages: 1 },
    });

    vi.doMock('@/utils/apiClient', () => ({
      apiFetchJSON: apiFetchJSONMock,
    }));

    ({
      useV2Workloads,
      __resetV2WorkloadsCacheForTests: resetV2WorkloadsCacheForTests,
    } = await import('@/hooks/useV2Workloads'));

    resetV2WorkloadsCacheForTests();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
    vi.resetModules();
  });

  it('reuses fresh cache on remount without an extra network fetch', async () => {
    let disposeFirst = () => {};
    createRoot((d) => {
      disposeFirst = d;
      const [enabled] = createSignal(true);
      useV2Workloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

    disposeFirst();

    let disposeSecond = () => {};
    createRoot((d) => {
      disposeSecond = d;
      const [enabled] = createSignal(true);
      useV2Workloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

    disposeSecond();
  });

  it('handles empty v2 responses without mutating into undefined state', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseV2WorkloadsModule['useV2Workloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useV2Workloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(result!.workloads()).toEqual([]);

    dispose();
  });

  it('keeps workload reference stable when polling returns identical payload', async () => {
    let dispose = () => {};
    let result: ReturnType<UseV2WorkloadsModule['useV2Workloads']> | undefined;
    let effectRuns = 0;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useV2Workloads(enabled);
      createEffect(() => {
        result!.workloads();
        effectRuns += 1;
      });
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    await waitForWorkloadCount(() => result!.workloads().length);
    const initialRef = result!.workloads();
    const initialEffectRuns = effectRuns;

    await advanceAndFlush(5_000);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    expect(result!.workloads()).toBe(initialRef);
    expect(effectRuns).toBe(initialEffectRuns);

    dispose();
  });

  it('maintains polling cadence under load without overlapping fetch churn', async () => {
    let dispose = () => {};
    let result: ReturnType<UseV2WorkloadsModule['useV2Workloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useV2Workloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    await waitForWorkloadCount(() => result!.workloads().length);

    const slowPoll = deferred<unknown>();
    apiFetchJSONMock.mockImplementationOnce(() => slowPoll.promise as Promise<any>);

    await advanceAndFlush(5_000);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);

    await advanceAndFlush(10_000);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);

    slowPoll.resolve({
      data: [sampleResource],
      meta: { totalPages: 1 },
    });
    await flushAsync();

    await advanceAndFlush(5_000);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(3);

    dispose();
  });
});
