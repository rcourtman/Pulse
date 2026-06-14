import { createSignal } from 'solid-js';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  DEFAULT_LOCALE,
  getLocaleFallbackChain,
  normalizeLocale,
  resolveSupportedLocale,
  type SupportedLocale,
} from './locales';
import {
  EN_MESSAGES,
  I18N_RUNTIME_MESSAGE_LOADERS,
  type I18nCatalog,
  type I18nMessageKey,
} from './messages';

type MessageParams = Record<string, string | number | boolean | null | undefined>;
type LocalePreferenceStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;
type DeferredLocale = Exclude<SupportedLocale, typeof DEFAULT_LOCALE>;

const localeMessageOverrides: Partial<Record<DeferredLocale, Partial<I18nCatalog>>> = {};
const pendingLocaleLoads = new Map<DeferredLocale, Promise<void>>();
const [catalogVersion, setCatalogVersion] = createSignal(0);

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

function getDeferredLocale(locale: SupportedLocale): DeferredLocale | null {
  return locale === DEFAULT_LOCALE ? null : locale;
}

function handleLocaleCatalogLoadFailure(locale: SupportedLocale, error: unknown): void {
  if (typeof console === 'undefined') return;
  console.warn(`Unable to load ${locale} locale catalog; falling back to English.`, error);
}

function scheduleLocaleCatalogLoad(locale: SupportedLocale): void {
  void loadLocaleCatalog(locale).catch((error: unknown) => {
    handleLocaleCatalogLoadFailure(locale, error);
  });
}

function getLocaleMessage(locale: SupportedLocale, key: I18nMessageKey): string | undefined {
  const deferredLocale = getDeferredLocale(locale);
  if (!deferredLocale) return EN_MESSAGES[key];
  return localeMessageOverrides[deferredLocale]?.[key];
}

export function isLocaleCatalogLoaded(locale: string | null | undefined): boolean {
  const normalized = normalizeLocale(locale);
  const deferredLocale = getDeferredLocale(normalized);
  return !deferredLocale || Boolean(localeMessageOverrides[deferredLocale]);
}

export function loadLocaleCatalog(locale: string | null | undefined): Promise<SupportedLocale> {
  const normalized = normalizeLocale(locale);
  const deferredLocale = getDeferredLocale(normalized);
  if (!deferredLocale || localeMessageOverrides[deferredLocale]) {
    return Promise.resolve(normalized);
  }

  const pendingLoad = pendingLocaleLoads.get(deferredLocale);
  if (pendingLoad) return pendingLoad.then(() => normalized);

  const loadPromise = I18N_RUNTIME_MESSAGE_LOADERS[deferredLocale]()
    .then((messageOverrides) => {
      localeMessageOverrides[deferredLocale] = messageOverrides;
      setCatalogVersion((version) => version + 1);
    })
    .finally(() => {
      pendingLocaleLoads.delete(deferredLocale);
    });

  pendingLocaleLoads.set(deferredLocale, loadPromise);
  return loadPromise.then(() => normalized);
}

const initialLocalePreference = getInitialLocalePreference();
const [activeLocaleSignal, setActiveLocaleSignal] =
  createSignal<SupportedLocale>(initialLocalePreference);
scheduleLocaleCatalogLoad(initialLocalePreference);

export const activeLocale = activeLocaleSignal;

export function getActiveLocale(): SupportedLocale {
  return activeLocaleSignal();
}

export function setActiveLocale(locale: string | null | undefined): SupportedLocale {
  const normalized = normalizeLocale(locale);
  setActiveLocaleSignal(normalized);
  scheduleLocaleCatalogLoad(normalized);
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
  catalogVersion();
  const fallbackChain = getLocaleFallbackChain(locale);
  const template =
    fallbackChain.reduce<string | undefined>(
      (message, fallbackLocale) => message ?? getLocaleMessage(fallbackLocale, key),
      undefined,
    ) ?? EN_MESSAGES[key];
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (match, paramKey: string) => {
    const value = params[paramKey];
    return value === null || value === undefined ? match : String(value);
  });
}

export const t = translateMessage;

export type { I18nMessageKey } from './messages';
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
  ALERTS_OVERVIEW_ALLOWED_IDENTICAL_TRANSLATIONS,
  ALERTS_OVERVIEW_NON_TRANSLATABLE_TOKENS,
  FIRST_SESSION_MONITORING_ALLOWED_IDENTICAL_TRANSLATIONS,
  FIRST_SESSION_MONITORING_NON_TRANSLATABLE_TOKENS,
  LOCALIZATION_FOUNDATION,
  LOCALIZED_ALERTS_OVERVIEW_JOURNEY_KEYS,
  LOCALIZED_FIRST_SESSION_MONITORING_JOURNEY_KEYS,
  LOCALIZED_SETTINGS_GENERAL_JOURNEY_KEYS,
  NEVER_TRANSLATE_COPY_RULES,
  SETTINGS_GENERAL_ALLOWED_IDENTICAL_TRANSLATIONS,
  SETTINGS_GENERAL_NON_TRANSLATABLE_TOKENS,
} from './policy';
