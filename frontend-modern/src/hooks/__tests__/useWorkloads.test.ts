import { createEffect, createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type UseWorkloadsModule = typeof import('@/hooks/useWorkloads');

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

describe('useWorkloads', () => {
  let apiFetchJSONMock: ReturnType<typeof vi.fn>;
  let useWorkloads: UseWorkloadsModule['useWorkloads'];
  let resetWorkloadsCacheForTests: UseWorkloadsModule['__resetWorkloadsCacheForTests'];
  let eventBus: (typeof import('@/stores/events'))['eventBus'];

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();

    apiFetchJSONMock = vi.fn().mockResolvedValue({
      data: [sampleResource],
      meta: { totalPages: 1 },
    });

    vi.doMock('@/utils/apiClient', () => ({
      apiFetchJSON: apiFetchJSONMock,
      getOrgID: () => 'default',
    }));

    ({
      useWorkloads,
      __resetWorkloadsCacheForTests: resetWorkloadsCacheForTests,
    } = await import('@/hooks/useWorkloads'));
    ({ eventBus } = await import('@/stores/events'));

    resetWorkloadsCacheForTests();
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
      useWorkloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

    disposeFirst();

    let disposeSecond = () => {};
    createRoot((d) => {
      disposeSecond = d;
      const [enabled] = createSignal(true);
      useWorkloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

    disposeSecond();
  });

  it('handles empty responses without mutating into undefined state', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      data: [],
      meta: { totalPages: 1 },
    });

    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(result!.workloads()).toEqual([]);

    dispose();
  });

  it('keeps workload reference stable when polling returns identical payload', async () => {
    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    let effectRuns = 0;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
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
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
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

  it('scopes shared cache by org and restores cached data when switching back', async () => {
    let dispose = () => {};
    let result: ReturnType<UseWorkloadsModule['useWorkloads']> | undefined;
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      result = useWorkloads(enabled);
    });

    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length);
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(result!.workloads()[0]?.id).toBe('cluster-a:pve1:101');

    apiFetchJSONMock.mockResolvedValueOnce({
      data: [
        {
          ...sampleResource,
          id: 'cluster-b-pve2-202',
          name: 'vm-202',
          vmid: 202,
          node: 'pve2',
          instance: 'cluster-b',
        },
      ],
      meta: { totalPages: 1 },
    });

    eventBus.emit('org_switched', 'tenant-b');
    await flushAsync();
    await waitForWorkloadCount(() => result!.workloads().length);

    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    expect(result!.workloads()[0]?.id).toBe('cluster-b:pve2:202');

    eventBus.emit('org_switched', 'default');
    await flushAsync();

    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    expect(result!.workloads()[0]?.id).toBe('cluster-a:pve1:101');

    dispose();
  });
});
