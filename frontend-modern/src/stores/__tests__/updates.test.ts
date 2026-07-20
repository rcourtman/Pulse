import { beforeEach, describe, expect, it, vi } from 'vitest';
import { STORAGE_KEYS } from '@/utils/localStorage';

const mockGetVersion = vi.fn();
const mockCheckForUpdates = vi.fn();
const mockApplyUpdate = vi.fn();
const mockRollbackUpdate = vi.fn();
const mockNotifySuccess = vi.fn();
const mockNotifyError = vi.fn();

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    getVersion: (...args: unknown[]) => mockGetVersion(...args),
    checkForUpdates: (...args: unknown[]) => mockCheckForUpdates(...args),
    applyUpdate: (...args: unknown[]) => mockApplyUpdate(...args),
    rollbackUpdate: (...args: unknown[]) => mockRollbackUpdate(...args),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => mockNotifySuccess(...args),
    error: (...args: unknown[]) => mockNotifyError(...args),
  },
}));

const loadUpdateStore = async () => {
  const { updateStore } = await import('@/stores/updates');
  return updateStore;
};

const baseVersionInfo = {
  version: 'v1.0.0',
  build: 'abc123',
  runtime: 'linux-amd64',
  channel: 'stable',
  isDocker: false,
  isSourceBuild: false,
  isDevelopment: false,
  deploymentType: 'systemd',
};

const baseUpdateInfo = {
  available: true,
  currentVersion: 'v1.0.0',
  latestVersion: 'v1.1.0',
  releaseNotes: 'Release notes',
  releaseDate: '2026-02-11T00:00:00Z',
  downloadUrl: 'https://example.com/pulse-v1.1.0.tar.gz',
  isPrerelease: false,
};

describe('updateStore', () => {
  beforeEach(() => {
    vi.resetModules();
    localStorage.clear();
    mockGetVersion.mockReset();
    mockCheckForUpdates.mockReset();
    mockApplyUpdate.mockReset();
    mockRollbackUpdate.mockReset();
    mockNotifySuccess.mockReset();
    mockNotifyError.mockReset();
  });

  it('retries transient update-check failures before succeeding', async () => {
    mockGetVersion.mockResolvedValue(baseVersionInfo);
    mockCheckForUpdates
      .mockRejectedValueOnce(new Error('Request failed with status 503'))
      .mockResolvedValueOnce(baseUpdateInfo);

    const updateStore = await loadUpdateStore();
    await updateStore.checkForUpdates(true);

    expect(mockCheckForUpdates).toHaveBeenCalledTimes(2);
    expect(updateStore.updateAvailable()).toBe(true);
    expect(updateStore.updateInfo()?.latestVersion).toBe('v1.1.0');
    expect(updateStore.lastError()).toBeNull();
  });

  it('keeps cached update visibility when live checks fail', async () => {
    localStorage.setItem(
      STORAGE_KEYS.UPDATES,
      JSON.stringify({
        lastCheck: Date.now(),
        dismissedVersion: undefined,
        updateInfo: baseUpdateInfo,
      }),
    );
    mockGetVersion.mockRejectedValue(new Error('Failed to fetch'));

    const updateStore = await loadUpdateStore();
    await updateStore.checkForUpdates();

    expect(updateStore.updateAvailable()).toBe(true);
    expect(updateStore.updateInfo()?.latestVersion).toBe(baseUpdateInfo.latestVersion);
    expect(updateStore.lastError()).toBe('Failed to fetch');
    expect(mockCheckForUpdates).not.toHaveBeenCalled();
  });

  it('clears dismissed state when cached latest version changes', async () => {
    // Mock getVersion to match the cached currentVersion so the cache path is used
    mockGetVersion.mockResolvedValue({ ...baseVersionInfo, version: 'v1.0.0' });

    const updateStore = await loadUpdateStore();
    updateStore.simulateUpdate('v1.2.0');
    updateStore.dismissUpdate();
    expect(updateStore.isDismissed()).toBe(true);

    // Set cached data with a DIFFERENT latestVersion than what was dismissed
    localStorage.setItem(
      STORAGE_KEYS.UPDATES,
      JSON.stringify({
        lastCheck: Date.now(),
        dismissedVersion: 'v1.2.0',
        updateInfo: {
          ...baseUpdateInfo,
          latestVersion: 'v1.3.0',
        },
      }),
    );

    await updateStore.checkForUpdates();

    // Cache path calls getVersion once to verify current version matches
    expect(mockGetVersion).toHaveBeenCalledTimes(1);
    // Since cache is valid (version matches), checkForUpdates API is NOT called
    expect(mockCheckForUpdates).not.toHaveBeenCalled();
    // Dismissed version (v1.2.0) !== latest version (v1.3.0), so dismissed is cleared
    expect(updateStore.isDismissed()).toBe(false);
    expect(updateStore.updateAvailable()).toBe(true);
  });

  describe('applyUpdate', () => {
    it('posts the apply and records the pending-apply marker before it', async () => {
      mockApplyUpdate.mockResolvedValue({ status: 'ok', message: '' });

      const updateStore = await loadUpdateStore();
      updateStore.simulateUpdate('v1.1.0');

      const started = await updateStore.applyUpdate();

      expect(started).toBe(true);
      expect(mockApplyUpdate).toHaveBeenCalledWith('#');
      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEYS.UPDATES) ?? '{}');
      expect(persisted.pendingApply).toMatchObject({
        fromVersion: 'v6.0.0',
        toVersion: 'v1.1.0',
      });
      expect(mockNotifyError).not.toHaveBeenCalled();
    });

    it('clears the marker and toasts the shared error message when the apply fails', async () => {
      mockApplyUpdate.mockRejectedValue(new Error('boom'));

      const updateStore = await loadUpdateStore();
      updateStore.simulateUpdate('v1.1.0');

      const started = await updateStore.applyUpdate();

      expect(started).toBe(false);
      expect(mockNotifyError).toHaveBeenCalledWith('Unable to start the update. Please try again.');
      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEYS.UPDATES) ?? '{}');
      expect(persisted.pendingApply).toBeUndefined();
    });

    it('does nothing without a download URL', async () => {
      const updateStore = await loadUpdateStore();

      const started = await updateStore.applyUpdate();

      expect(started).toBe(false);
      expect(mockApplyUpdate).not.toHaveBeenCalled();
    });
  });

  describe('rollbackUpdate', () => {
    const rollbackParams = {
      eventId: '01JZEXAMPLE',
      fromVersion: 'v1.1.0',
      toVersion: 'v1.0.0',
    };

    it('posts the rollback and records a rollback pending marker before it', async () => {
      mockRollbackUpdate.mockResolvedValue({ status: 'started', message: '' });

      const updateStore = await loadUpdateStore();
      const started = await updateStore.rollbackUpdate(rollbackParams);

      expect(started).toBe(true);
      expect(mockRollbackUpdate).toHaveBeenCalledWith('01JZEXAMPLE');
      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEYS.UPDATES) ?? '{}');
      expect(persisted.pendingApply).toMatchObject({
        fromVersion: 'v1.1.0',
        toVersion: 'v1.0.0',
        action: 'rollback',
      });
      expect(mockNotifyError).not.toHaveBeenCalled();
    });

    it('clears the marker and toasts the backend error when the rollback is rejected', async () => {
      mockRollbackUpdate.mockRejectedValue(
        new Error(
          'no retained backup for this update; it may have been pruned by backup retention',
        ),
      );

      const updateStore = await loadUpdateStore();
      const started = await updateStore.rollbackUpdate(rollbackParams);

      expect(started).toBe(false);
      expect(mockNotifyError).toHaveBeenCalledWith(
        'no retained backup for this update; it may have been pruned by backup retention',
      );
      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEYS.UPDATES) ?? '{}');
      expect(persisted.pendingApply).toBeUndefined();
    });
  });

  describe('post-update confirmation', () => {
    const seedPendingApply = (fromVersion: string, toVersion: string) => {
      localStorage.setItem(
        STORAGE_KEYS.UPDATES,
        JSON.stringify({
          lastCheck: 0,
          pendingApply: { fromVersion, toVersion, startedAt: Date.now() },
        }),
      );
    };

    it('toasts once when the running version moved off the pre-update version', async () => {
      seedPendingApply('v1.0.0', 'v1.1.0');
      mockGetVersion.mockResolvedValue({ ...baseVersionInfo, version: 'v1.1.0' });
      mockCheckForUpdates.mockResolvedValue({
        ...baseUpdateInfo,
        available: false,
        currentVersion: 'v1.1.0',
      });

      const updateStore = await loadUpdateStore();
      await updateStore.checkForUpdates(true);

      expect(mockNotifySuccess).toHaveBeenCalledWith('Updated to v1.1.0');
      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEYS.UPDATES) ?? '{}');
      expect(persisted.pendingApply).toBeUndefined();

      // The marker is consumed: a later check must not toast again.
      mockNotifySuccess.mockClear();
      await updateStore.checkForUpdates(true);
      expect(mockNotifySuccess).not.toHaveBeenCalled();
    });

    it('confirms even for dev builds that skip the update check itself', async () => {
      seedPendingApply('v1.0.0', 'v1.1.0');
      mockGetVersion.mockResolvedValue({
        ...baseVersionInfo,
        version: 'v1.1.0',
        isDevelopment: true,
      });

      const updateStore = await loadUpdateStore();
      await updateStore.checkForUpdates(true);

      expect(mockCheckForUpdates).not.toHaveBeenCalled();
      expect(mockNotifySuccess).toHaveBeenCalledWith('Updated to v1.1.0');
    });

    it('uses rollback wording when a rollback marker confirms', async () => {
      localStorage.setItem(
        STORAGE_KEYS.UPDATES,
        JSON.stringify({
          lastCheck: 0,
          pendingApply: {
            fromVersion: 'v1.1.0',
            toVersion: 'v1.0.0',
            startedAt: Date.now(),
            action: 'rollback',
          },
        }),
      );
      mockGetVersion.mockResolvedValue({ ...baseVersionInfo, version: 'v1.0.0' });
      mockCheckForUpdates.mockResolvedValue({
        ...baseUpdateInfo,
        available: false,
        currentVersion: 'v1.0.0',
      });

      const updateStore = await loadUpdateStore();
      await updateStore.checkForUpdates(true);

      expect(mockNotifySuccess).toHaveBeenCalledWith('Rolled back to v1.0.0');
    });

    it('clears the marker silently when the version did not change', async () => {
      seedPendingApply('v1.0.0', 'v1.1.0');
      mockGetVersion.mockResolvedValue({ ...baseVersionInfo, version: 'v1.0.0' });
      mockCheckForUpdates.mockResolvedValue({ ...baseUpdateInfo, available: false });

      const updateStore = await loadUpdateStore();
      await updateStore.checkForUpdates(true);

      expect(mockNotifySuccess).not.toHaveBeenCalled();
      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEYS.UPDATES) ?? '{}');
      expect(persisted.pendingApply).toBeUndefined();
    });

    it('does not resurrect a consumed marker when the check later saves state', async () => {
      // Fresh-path save at the end of checkForUpdates spreads the in-memory
      // state; a stale copy would re-persist the consumed marker.
      seedPendingApply('v1.0.0', 'v1.1.0');
      mockGetVersion.mockResolvedValue({ ...baseVersionInfo, version: 'v1.1.0' });
      mockCheckForUpdates.mockResolvedValue({
        ...baseUpdateInfo,
        currentVersion: 'v1.1.0',
        latestVersion: 'v1.2.0',
      });

      const updateStore = await loadUpdateStore();
      await updateStore.checkForUpdates(true);

      const persisted = JSON.parse(localStorage.getItem(STORAGE_KEYS.UPDATES) ?? '{}');
      expect(persisted.updateInfo?.latestVersion).toBe('v1.2.0');
      expect(persisted.pendingApply).toBeUndefined();
    });
  });
});
