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

  it('auto-clears success status after delay', async () => {
    const store = await loadStore();

    store.markContainerUpdating('host-1', 'container-1', 'cmd-1');
    store.markContainerUpdateSuccess('host-1', 'container-1');

    expect(store.getContainerUpdateState('host-1', 'container-1')?.state).toBe('success');

    vi.advanceTimersByTime(5000);

    expect(store.getContainerUpdateState('host-1', 'container-1')).toBeUndefined();
  });

  it('does not clear a newer update when an older success timer fires', async () => {
    const store = await loadStore();

    store.markContainerUpdating('host-1', 'container-1', 'cmd-1');
    store.markContainerUpdateSuccess('host-1', 'container-1');

    vi.advanceTimersByTime(1000);
    store.markContainerUpdating('host-1', 'container-1', 'cmd-2');

    vi.advanceTimersByTime(4000);

    expect(store.getContainerUpdateState('host-1', 'container-1')?.state).toBe('updating');
  });

  it('does not clear a newer queued update when an older error timer fires', async () => {
    const store = await loadStore();

    store.markContainerUpdating('host-1', 'container-1', 'cmd-1');
    store.markContainerUpdateError('host-1', 'container-1', 'failed');

    vi.advanceTimersByTime(1000);
    store.markContainerQueued('host-1', 'container-1', 'cmd-2');

    vi.advanceTimersByTime(9000);

    expect(store.getContainerUpdateState('host-1', 'container-1')?.state).toBe('queued');
  });
});
