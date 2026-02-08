import { STORAGE_KEYS } from '@/utils/localStorage';

const readLocalBoolean = (key: string): boolean => {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(key) === 'true';
  } catch {
    return false;
  }
};

export const isStorageBackupsV2Enabled = (): boolean => {
  if (import.meta.env.VITE_STORAGE_BACKUPS_V2 === '1') return true;
  return readLocalBoolean(STORAGE_KEYS.STORAGE_BACKUPS_V2_ENABLED);
};

export const isBackupsV2RolledBack = (): boolean =>
  readLocalBoolean(STORAGE_KEYS.BACKUPS_V2_ROLLBACK);

export const isStorageV2RolledBack = (): boolean =>
  readLocalBoolean(STORAGE_KEYS.STORAGE_V2_ROLLBACK);
