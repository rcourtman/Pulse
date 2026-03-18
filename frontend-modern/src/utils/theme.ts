import { STORAGE_KEYS } from '@/utils/localStorage';

export type ThemePreference = 'light' | 'dark' | 'system';

function isThemePreference(value: string | null): value is ThemePreference {
  return value === 'light' || value === 'dark' || value === 'system';
}

function safeGet(key: string): string | null {
  if (typeof window === 'undefined') return null;
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function safeSet(key: string, value: string): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Ignore storage write failures (private browsing, quota, etc.).
  }
}

export function normalizeThemePreference(value: string | null | undefined): ThemePreference {
  const normalized = value ?? null;
  return isThemePreference(normalized) ? normalized : 'system';
}

export function hasStoredThemePreference(): boolean {
  return isThemePreference(safeGet(STORAGE_KEYS.THEME_PREFERENCE));
}

export function getStoredThemePreference(): ThemePreference {
  const explicitPreference = safeGet(STORAGE_KEYS.THEME_PREFERENCE);
  if (isThemePreference(explicitPreference)) {
    return explicitPreference;
  }
  return 'system';
}

export function persistThemePreference(preference: ThemePreference): void {
  safeSet(STORAGE_KEYS.THEME_PREFERENCE, preference);
}

export function computeIsDark(preference: ThemePreference): boolean {
  if (preference === 'dark') return true;
  if (preference === 'light') return false;
  if (typeof window === 'undefined') return false;
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export function applyThemeClass(isDark: boolean): void {
  if (typeof window === 'undefined') return;
  document.documentElement.classList.toggle('dark', isDark);
}
