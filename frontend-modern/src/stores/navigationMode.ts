import { STORAGE_KEYS, createLocalStorageStringSignal } from '@/utils/localStorage';

export type NavigationMode = 'unified' | 'classic';

const DEFAULT_MODE: NavigationMode = 'unified';

const [navigationModeStored, setNavigationModeStored] = createLocalStorageStringSignal(
  STORAGE_KEYS.NAVIGATION_MODE,
  DEFAULT_MODE,
);

export function navigationMode(): NavigationMode {
  const raw = (navigationModeStored() || '').trim().toLowerCase();
  return raw === 'classic' ? 'classic' : DEFAULT_MODE;
}

export function setNavigationMode(mode: NavigationMode): void {
  setNavigationModeStored(mode);
}

