export const DEFAULT_LOCALE = 'en';

export const SUPPORTED_LOCALES = ['en', 'de', 'es'] as const;

export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];

export type LocaleRolloutStage = 'source' | 'first-wave';

export interface SupportedLocaleDefinition {
  label: string;
  englishLabel: string;
  fallbackLocale: typeof DEFAULT_LOCALE;
  rolloutStage: LocaleRolloutStage;
}

export const SUPPORTED_LOCALE_REGISTRY: Record<SupportedLocale, SupportedLocaleDefinition> = {
  en: {
    label: 'English',
    englishLabel: 'English',
    fallbackLocale: DEFAULT_LOCALE,
    rolloutStage: 'source',
  },
  de: {
    label: 'Deutsch',
    englishLabel: 'German',
    fallbackLocale: DEFAULT_LOCALE,
    rolloutStage: 'first-wave',
  },
  es: {
    label: 'Español',
    englishLabel: 'Spanish',
    fallbackLocale: DEFAULT_LOCALE,
    rolloutStage: 'first-wave',
  },
};

export const FIRST_LOCALIZATION_LOCALES = [
  'de',
  'es',
] as const satisfies readonly SupportedLocale[];

export const NEXT_LOCALIZATION_LOCALES = ['fr', 'pt-BR', 'ja', 'zh-Hans', 'ko'] as const;

export const SUPPORTED_LOCALE_LABELS: Record<SupportedLocale, string> = {
  en: SUPPORTED_LOCALE_REGISTRY.en.label,
  de: SUPPORTED_LOCALE_REGISTRY.de.label,
  es: SUPPORTED_LOCALE_REGISTRY.es.label,
};

const SUPPORTED_LOCALE_SET = new Set<string>(SUPPORTED_LOCALES);

const LOCALE_ALIASES: Record<string, SupportedLocale> = {
  'en-gb': 'en',
  'en-us': 'en',
  'de-at': 'de',
  'de-ch': 'de',
  'de-de': 'de',
  'es-419': 'es',
  'es-ar': 'es',
  'es-cl': 'es',
  'es-co': 'es',
  'es-es': 'es',
  'es-mx': 'es',
  'es-pe': 'es',
  'es-us': 'es',
};

export function isSupportedLocale(value: string): value is SupportedLocale {
  return SUPPORTED_LOCALE_SET.has(value);
}

export function resolveSupportedLocale(value: string | null | undefined): SupportedLocale | null {
  const normalized = value?.trim().replace(/_/g, '-').toLowerCase();
  if (!normalized) return null;
  if (isSupportedLocale(normalized)) return normalized;
  if (LOCALE_ALIASES[normalized]) return LOCALE_ALIASES[normalized];

  const baseLocale = normalized.split('-')[0] ?? '';
  return isSupportedLocale(baseLocale) ? baseLocale : null;
}

export function normalizeLocale(value: string | null | undefined): SupportedLocale {
  return resolveSupportedLocale(value) ?? DEFAULT_LOCALE;
}

export function getLocaleFallbackChain(
  value: string | null | undefined,
): readonly SupportedLocale[] {
  const locale = normalizeLocale(value);
  const fallbackLocale = SUPPORTED_LOCALE_REGISTRY[locale].fallbackLocale;
  return locale === fallbackLocale ? [locale] : [locale, fallbackLocale];
}
