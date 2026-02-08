import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  isStorageBackupsV2Enabled,
  isBackupsV2RolledBack,
  isStorageV2RolledBack,
} from '@/utils/featureFlags';
import { STORAGE_KEYS } from '@/utils/localStorage';

describe('featureFlags', () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    localStorage.removeItem(STORAGE_KEYS.STORAGE_BACKUPS_V2_ENABLED);
    localStorage.removeItem(STORAGE_KEYS.STORAGE_V2_ROLLBACK);
    localStorage.removeItem(STORAGE_KEYS.BACKUPS_V2_ROLLBACK);
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

  it('backups v2 rollback defaults to false', () => {
    expect(isBackupsV2RolledBack()).toBe(false);
  });

  it('backups v2 rollback reads from localStorage', () => {
    localStorage.setItem(STORAGE_KEYS.BACKUPS_V2_ROLLBACK, 'true');
    expect(isBackupsV2RolledBack()).toBe(true);
  });

  it('storage v2 rollback defaults to false', () => {
    expect(isStorageV2RolledBack()).toBe(false);
  });

  it('storage v2 rollback reads from localStorage', () => {
    localStorage.setItem(STORAGE_KEYS.STORAGE_V2_ROLLBACK, 'true');
    expect(isStorageV2RolledBack()).toBe(true);
  });
});
