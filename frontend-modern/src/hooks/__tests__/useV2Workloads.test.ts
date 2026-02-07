import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type UseV2WorkloadsModule = typeof import('@/hooks/useV2Workloads');

const sampleResource = {
  id: 'cluster-a-pve1-101',
  type: 'vm',
  name: 'vm-101',
  status: 'running',
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

  it('continues polling while enabled', async () => {
    let dispose = () => {};
    createRoot((d) => {
      dispose = d;
      const [enabled] = createSignal(true);
      useV2Workloads(enabled);
    });

    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(5_000);
    await flushAsync();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);

    dispose();
  });
});
