import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const loadStore = async () => import('@/stores/containerUpdates');

describe('containerUpdates store lifecycle', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetModules();
  });

  afterEach(async () => {
    const store = await loadStore();
    store.stopContainerUpdateCleanup({ clearStates: true });
    vi.clearAllTimers();
    vi.useRealTimers();
  });

  it('cancels stale auto-clear timers when an update restarts', async () => {
    const store = await loadStore();

    store.markContainerUpdateSuccess('host-1', 'ctr-1');
    expect(store.getContainerUpdateState('host-1', 'ctr-1')?.state).toBe('success');

    vi.advanceTimersByTime(3000);
    store.markContainerUpdating('host-1', 'ctr-1', 'cmd-2');
    vi.advanceTimersByTime(3000);

    expect(store.getContainerUpdateState('host-1', 'ctr-1')?.state).toBe('updating');
  });

  it('stops pending auto-clear timers when cleanup lifecycle is stopped', async () => {
    const store = await loadStore();

    store.markContainerUpdateError('host-2', 'ctr-2', 'boom');
    expect(store.getContainerUpdateState('host-2', 'ctr-2')?.state).toBe('error');

    store.stopContainerUpdateCleanup();
    vi.advanceTimersByTime(20000);

    expect(store.getContainerUpdateState('host-2', 'ctr-2')?.state).toBe('error');
  });
});
