export const DEFAULT_LOCALE = 'en';

export const SUPPORTED_LOCALES = ['en', 'de', 'es'] as const;

export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];

export const FIRST_LOCALIZATION_LOCALES = [
  'de',
  'es',
] as const satisfies readonly SupportedLocale[];

export const NEXT_LOCALIZATION_LOCALES = ['fr', 'pt-BR', 'ja', 'zh-Hans', 'ko'] as const;

export const SUPPORTED_LOCALE_LABELS: Record<SupportedLocale, string> = {
  en: 'English',
  de: 'German',
  es: 'Spanish',
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

export function normalizeLocale(value: string | null | undefined): SupportedLocale {
  const normalized = value?.trim().replace('_', '-').toLowerCase();
  if (!normalized) return DEFAULT_LOCALE;
  if (isSupportedLocale(normalized)) return normalized;
  if (LOCALE_ALIASES[normalized]) return LOCALE_ALIASES[normalized];

  const baseLocale = normalized.split('-')[0] ?? '';
  return isSupportedLocale(baseLocale) ? baseLocale : DEFAULT_LOCALE;
}
