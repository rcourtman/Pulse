import { createRoot, createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useResourceDetailDrawerDockerActionsState } from '@/components/Infrastructure/useResourceDetailDrawerDockerActionsState';

const monitoringMock = vi.hoisted(() => ({
  checkDockerUpdates: vi.fn(),
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: monitoringMock,
}));

describe('useResourceDetailDrawerDockerActionsState', () => {
  beforeEach(() => {
    monitoringMock.checkDockerUpdates.mockReset();
  });

  it('queues docker update checks through the dedicated action owner', async () => {
    monitoringMock.checkDockerUpdates.mockResolvedValue(undefined);

    const scope = createRoot((dispose) => {
      const [dockerHostSourceId] = createSignal<string | null>('host-1');
      const [dockerUpdatesAvailable] = createSignal(3);
      return {
        dispose,
        state: useResourceDetailDrawerDockerActionsState({
          dockerHostSourceId,
          dockerUpdatesAvailable,
        }),
      };
    });

    await scope.state.queueDockerUpdateCheck();

    expect(monitoringMock.checkDockerUpdates).toHaveBeenCalledWith('host-1');
    expect(scope.state.dockerActionNote()).toBe('Check queued.');
    expect(scope.state.confirmUpdateAll()).toBe(false);
    scope.dispose();
  });

  it('points update-all at the reviewed per-container action flow', async () => {
    const scope = createRoot((dispose) => {
      const [dockerHostSourceId] = createSignal<string | null>('host-1');
      const [dockerUpdatesAvailable] = createSignal(4);
      return {
        dispose,
        state: useResourceDetailDrawerDockerActionsState({
          dockerHostSourceId,
          dockerUpdatesAvailable,
        }),
      };
    });

    const queued = await scope.state.queueDockerUpdateAll();

    expect(queued).toBe(false);
    expect(scope.state.confirmUpdateAll()).toBe(false);
    expect(scope.state.dockerActionNote()).toContain('per-container actions');
    expect(scope.state.dockerActionError()).toBe('');
    scope.dispose();
  });
});
