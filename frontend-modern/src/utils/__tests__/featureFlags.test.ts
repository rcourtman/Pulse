import { afterEach, describe, expect, it, vi } from 'vitest';
import { isStorageBackupsV2Enabled } from '@/utils/featureFlags';
import { STORAGE_KEYS } from '@/utils/localStorage';

describe('featureFlags', () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    localStorage.removeItem(STORAGE_KEYS.STORAGE_BACKUPS_V2_ENABLED);
  });

  it('enables storage/backups v2 from env flag', () => {
    vi.stubEnv('VITE_STORAGE_BACKUPS_V2', '1');
    expect(isStorageBackupsV2Enabled()).toBe(true);
  });

  it('enables storage/backups v2 from local storage override', () => {
    vi.stubEnv('VITE_STORAGE_BACKUPS_V2', '0');
    localStorage.setItem(STORAGE_KEYS.STORAGE_BACKUPS_V2_ENABLED, 'true');
    expect(isStorageBackupsV2Enabled()).toBe(true);
  });

  it('defaults storage/backups v2 to disabled', () => {
    vi.stubEnv('VITE_STORAGE_BACKUPS_V2', '0');
    expect(isStorageBackupsV2Enabled()).toBe(false);
  });
});

