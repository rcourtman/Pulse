import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { UpdateInfo } from '@/api/updates';

type UpdatesStoreModule = typeof import('@/stores/updates');

describe('updateStore', () => {
  let updateStore: UpdatesStoreModule['updateStore'];
  let getVersionMock: ReturnType<typeof vi.fn>;
  let checkForUpdatesMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();
    localStorage.clear();

    getVersionMock = vi.fn().mockResolvedValue({
      version: 'v1.2.3',
      build: 'test',
      runtime: 'go1.23',
      isDocker: false,
      isSourceBuild: false,
      isDevelopment: false,
    });

    checkForUpdatesMock = vi.fn().mockResolvedValue({
      available: true,
      currentVersion: 'v1.2.3',
      latestVersion: 'v1.2.4',
      releaseNotes: 'Bug fixes',
      releaseDate: '2026-02-12T00:00:00.000Z',
      downloadUrl: '/download',
      isPrerelease: false,
    } satisfies UpdateInfo);

    vi.doMock('@/api/updates', () => ({
      UpdatesAPI: {
        getVersion: getVersionMock,
        checkForUpdates: checkForUpdatesMock,
      },
    }));

    vi.doMock('@/utils/logger', () => ({
      logger: {
        error: vi.fn(),
        warn: vi.fn(),
        info: vi.fn(),
        debug: vi.fn(),
      },
    }));

    ({ updateStore } = await import('@/stores/updates'));
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.resetModules();
    localStorage.clear();
  });

  it('ignores malformed cached update payloads and refreshes from API', async () => {
    localStorage.setItem(
      STORAGE_KEYS.UPDATES,
      JSON.stringify({
        lastCheck: Date.now(),
        dismissedVersion: 'v1.2.4',
        updateInfo: {
          available: 'true',
          currentVersion: 'v1.2.3',
          latestVersion: 124,
          releaseNotes: 'stale',
          releaseDate: '2026-02-01T00:00:00.000Z',
          downloadUrl: '/stale',
          isPrerelease: false,
        },
      }),
    );

    await updateStore.checkForUpdates();

    expect(getVersionMock).toHaveBeenCalledTimes(1);
    expect(checkForUpdatesMock).toHaveBeenCalledTimes(1);
    expect(updateStore.updateAvailable()).toBe(true);
    expect(updateStore.updateInfo()).toEqual({
      available: true,
      currentVersion: 'v1.2.3',
      latestVersion: 'v1.2.4',
      releaseNotes: 'Bug fixes',
      releaseDate: '2026-02-12T00:00:00.000Z',
      downloadUrl: '/download',
      isPrerelease: false,
    });

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEYS.UPDATES) || '{}');
    expect(typeof stored.updateInfo.available).toBe('boolean');
    expect(typeof stored.updateInfo.latestVersion).toBe('string');
  });
});
