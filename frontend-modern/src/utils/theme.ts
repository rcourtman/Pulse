import { STORAGE_KEYS } from '@/utils/localStorage';

export type ThemePreference = 'light' | 'dark' | 'system';

const LEGACY_BOOTSTRAP_THEME_KEY = 'pulse_dark_mode';

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

function safeRemove(key: string): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.removeItem(key);
  } catch {
    // Ignore storage write failures (private browsing, quota, etc.).
  }
}

function normalizeLegacyBooleanTheme(value: string | null): ThemePreference | null {
  if (value === 'true') return 'dark';
  if (value === 'false') return 'light';
  return null;
}

export function normalizeThemePreference(value: string | null | undefined): ThemePreference {
  const normalized = value ?? null;
  return isThemePreference(normalized) ? normalized : 'system';
}

export function hasStoredThemePreference(): boolean {
  if (isThemePreference(safeGet(STORAGE_KEYS.THEME_PREFERENCE))) {
    return true;
  }
  if (normalizeLegacyBooleanTheme(safeGet(STORAGE_KEYS.DARK_MODE))) {
    return true;
  }
  return Boolean(normalizeLegacyBooleanTheme(safeGet(LEGACY_BOOTSTRAP_THEME_KEY)));
}

export function getStoredThemePreference(): ThemePreference {
  const explicitPreference = safeGet(STORAGE_KEYS.THEME_PREFERENCE);
  if (isThemePreference(explicitPreference)) {
    return explicitPreference;
  }

  const legacyTheme = normalizeLegacyBooleanTheme(safeGet(STORAGE_KEYS.DARK_MODE));
  if (legacyTheme) {
    // Migrate legacy key so every surface reads one canonical key.
    safeSet(STORAGE_KEYS.THEME_PREFERENCE, legacyTheme);
    return legacyTheme;
  }

  const legacyBootstrapTheme = normalizeLegacyBooleanTheme(safeGet(LEGACY_BOOTSTRAP_THEME_KEY));
  if (legacyBootstrapTheme) {
    // Migrate legacy bootstrap key to canonical key and compatibility key.
    safeSet(STORAGE_KEYS.THEME_PREFERENCE, legacyBootstrapTheme);
    safeSet(STORAGE_KEYS.DARK_MODE, String(legacyBootstrapTheme === 'dark'));
    return legacyBootstrapTheme;
  }

  return 'system';
}

export function persistThemePreference(preference: ThemePreference): void {
  safeSet(STORAGE_KEYS.THEME_PREFERENCE, preference);

  // Keep legacy key in sync for backward compatibility with older builds.
  if (preference === 'system') {
    safeRemove(STORAGE_KEYS.DARK_MODE);
  } else {
    safeSet(STORAGE_KEYS.DARK_MODE, String(preference === 'dark'));
  }

  // Remove stale bootstrap key once a canonical preference exists.
  safeRemove(LEGACY_BOOTSTRAP_THEME_KEY);
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
