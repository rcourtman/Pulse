import { createRoot, createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useResourceDetailDrawerDockerActionsState } from '@/components/Infrastructure/useResourceDetailDrawerDockerActionsState';

const monitoringMock = vi.hoisted(() => ({
  checkDockerUpdates: vi.fn(),
  updateAllDockerContainers: vi.fn(),
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: monitoringMock,
}));

describe('useResourceDetailDrawerDockerActionsState', () => {
  beforeEach(() => {
    monitoringMock.checkDockerUpdates.mockReset();
    monitoringMock.updateAllDockerContainers.mockReset();
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

  it('requires confirmation before queueing update-all mutations', async () => {
    monitoringMock.updateAllDockerContainers.mockResolvedValue(undefined);

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

    await scope.state.queueDockerUpdateAll();

    expect(monitoringMock.updateAllDockerContainers).not.toHaveBeenCalled();
    expect(scope.state.confirmUpdateAll()).toBe(true);
    expect(scope.state.dockerActionNote()).toContain('Click again to update 4 containers.');

    await scope.state.queueDockerUpdateAll();

    expect(monitoringMock.updateAllDockerContainers).toHaveBeenCalledWith('host-1');
    expect(scope.state.dockerActionNote()).toBe('Update queued.');
    expect(scope.state.confirmUpdateAll()).toBe(false);
    scope.dispose();
  });

  it('clears pending confirmation when docker actions are hidden', async () => {
    const scope = createRoot((dispose) => {
      const [dockerHostSourceId] = createSignal<string | null>('host-1');
      const [dockerUpdatesAvailable] = createSignal(2);
      return {
        dispose,
        state: useResourceDetailDrawerDockerActionsState({
          dockerHostSourceId,
          dockerUpdatesAvailable,
        }),
      };
    });

    scope.state.toggleDockerUpdateControls();
    await scope.state.queueDockerUpdateAll();
    expect(scope.state.confirmUpdateAll()).toBe(true);

    scope.state.toggleDockerUpdateControls();

    expect(scope.state.confirmUpdateAll()).toBe(false);
    scope.dispose();
  });
});
