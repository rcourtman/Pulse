import { createSignal } from 'solid-js';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  DEFAULT_LOCALE,
  getLocaleFallbackChain,
  normalizeLocale,
  resolveSupportedLocale,
  type SupportedLocale,
} from './locales';
import { EN_MESSAGES, I18N_MESSAGES, type I18nMessageKey } from './messages';

type MessageParams = Record<string, string | number | boolean | null | undefined>;
type LocalePreferenceStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

function getDefaultStorage(): LocalePreferenceStorage | null {
  if (typeof window === 'undefined') return null;
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

function safeGetLocalePreference(storage: LocalePreferenceStorage | null): string | null {
  if (!storage) return null;
  try {
    return storage.getItem(STORAGE_KEYS.LOCALE_PREFERENCE);
  } catch {
    return null;
  }
}

function safeSetLocalePreference(
  locale: SupportedLocale,
  storage: LocalePreferenceStorage | null,
): void {
  if (!storage) return;
  try {
    storage.setItem(STORAGE_KEYS.LOCALE_PREFERENCE, locale);
  } catch {
    // Locale selection remains active in-memory when storage is unavailable.
  }
}

function getNavigatorLocaleCandidates(): string[] {
  if (typeof navigator === 'undefined') return [];
  const candidates = Array.isArray(navigator.languages) ? [...navigator.languages] : [];
  if (navigator.language) {
    candidates.push(navigator.language);
  }
  return candidates;
}

export function getStoredLocalePreference(
  storage: LocalePreferenceStorage | null = getDefaultStorage(),
): SupportedLocale | null {
  return resolveSupportedLocale(safeGetLocalePreference(storage));
}

export function detectBrowserLocale(
  candidates: readonly string[] = getNavigatorLocaleCandidates(),
) {
  for (const candidate of candidates) {
    const locale = resolveSupportedLocale(candidate);
    if (locale) return locale;
  }
  return DEFAULT_LOCALE;
}

export function getInitialLocalePreference({
  storage = getDefaultStorage(),
  browserLocales,
}: {
  storage?: LocalePreferenceStorage | null;
  browserLocales?: readonly string[];
} = {}): SupportedLocale {
  return getStoredLocalePreference(storage) ?? detectBrowserLocale(browserLocales);
}

const [activeLocaleSignal, setActiveLocaleSignal] = createSignal<SupportedLocale>(
  getInitialLocalePreference(),
);

export const activeLocale = activeLocaleSignal;

export function getActiveLocale(): SupportedLocale {
  return activeLocaleSignal();
}

export function setActiveLocale(locale: string | null | undefined): SupportedLocale {
  const normalized = normalizeLocale(locale);
  setActiveLocaleSignal(normalized);
  return normalized;
}

export function setLocalePreference(
  locale: string | null | undefined,
  storage: LocalePreferenceStorage | null = getDefaultStorage(),
): SupportedLocale {
  const normalized = setActiveLocale(locale);
  safeSetLocalePreference(normalized, storage);
  return normalized;
}

export function translateMessage(
  key: I18nMessageKey,
  params: MessageParams = {},
  locale: string | null | undefined = activeLocaleSignal(),
): string {
  const fallbackChain = getLocaleFallbackChain(locale);
  const template =
    fallbackChain.reduce<string | undefined>(
      (message, fallbackLocale) => message ?? I18N_MESSAGES[fallbackLocale][key],
      undefined,
    ) ?? EN_MESSAGES[key];
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (match, paramKey: string) => {
    const value = params[paramKey];
    return value === null || value === undefined ? match : String(value);
  });
}

export const t = translateMessage;

export type { I18nMessageKey } from './messages';
export { I18N_MESSAGES } from './messages';
export {
  DEFAULT_LOCALE,
  FIRST_LOCALIZATION_LOCALES,
  getLocaleFallbackChain,
  NEXT_LOCALIZATION_LOCALES,
  SUPPORTED_LOCALE_REGISTRY,
  SUPPORTED_LOCALE_LABELS,
  SUPPORTED_LOCALES,
  isSupportedLocale,
  normalizeLocale,
  resolveSupportedLocale,
  type LocaleRolloutStage,
  type SupportedLocaleDefinition,
  type SupportedLocale,
} from './locales';
export {
  LOCALIZATION_FOUNDATION,
  LOCALIZED_SETTINGS_GENERAL_JOURNEY_KEYS,
  NEVER_TRANSLATE_COPY_RULES,
  SETTINGS_GENERAL_ALLOWED_IDENTICAL_TRANSLATIONS,
  SETTINGS_GENERAL_NON_TRANSLATABLE_TOKENS,
} from './policy';
