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

    store.markContainerUpdating('agent-1', 'container-1', 'cmd-1');
    store.markContainerUpdateSuccess('agent-1', 'container-1');

    expect(store.getContainerUpdateState('agent-1', 'container-1')?.state).toBe('success');

    vi.advanceTimersByTime(5000);

    expect(store.getContainerUpdateState('agent-1', 'container-1')).toBeUndefined();
  });

  it('does not clear a newer update when an older success timer fires', async () => {
    const store = await loadStore();

    store.markContainerUpdating('agent-1', 'container-1', 'cmd-1');
    store.markContainerUpdateSuccess('agent-1', 'container-1');

    vi.advanceTimersByTime(1000);
    store.markContainerUpdating('agent-1', 'container-1', 'cmd-2');

    vi.advanceTimersByTime(4000);

    expect(store.getContainerUpdateState('agent-1', 'container-1')?.state).toBe('updating');
  });

  it('does not clear a newer queued update when an older error timer fires', async () => {
    const store = await loadStore();

    store.markContainerUpdating('agent-1', 'container-1', 'cmd-1');
    store.markContainerUpdateError('agent-1', 'container-1', 'failed');

    vi.advanceTimersByTime(1000);
    store.markContainerQueued('agent-1', 'container-1', 'cmd-2');

    vi.advanceTimersByTime(9000);

    expect(store.getContainerUpdateState('agent-1', 'container-1')?.state).toBe('queued');
  });

  it('does not turn a governed pending audit into success from a legacy command or elapsed time', async () => {
    const store = await loadStore();
    store.markContainerQueued('agent-1', 'container-1', 'action-1');
    store.syncWithAgentCommand('agent-1', {
      id: 'command:container-1',
      type: 'update_container',
      status: 'completed',
    } as never);
    vi.advanceTimersByTime(6 * 60_000);
    expect(store.getContainerUpdateState('agent-1', 'container-1')).toMatchObject({
      state: 'queued',
      actionId: 'action-1',
    });
  });

  it('keeps an operator-closed unknown outcome reviewable without presenting it as failed', async () => {
    const store = await loadStore();
    store.markContainerQueued('agent-1', 'container-1', 'action-1');
    store.markContainerUpdateInconclusive('agent-1', 'container-1', 'action-1');
    vi.advanceTimersByTime(6 * 60_000);
    expect(store.getContainerUpdateState('agent-1', 'container-1')).toMatchObject({
      state: 'inconclusive',
      actionId: 'action-1',
    });
  });
});
