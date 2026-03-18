import { beforeEach, describe, expect, it, vi } from 'vitest';
import { STORAGE_KEYS } from '@/utils/localStorage';

const mockGetVersion = vi.fn();
const mockCheckForUpdates = vi.fn();

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    getVersion: (...args: unknown[]) => mockGetVersion(...args),
    checkForUpdates: (...args: unknown[]) => mockCheckForUpdates(...args),
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
});
