import { createSignal } from 'solid-js';
import { DEFAULT_LOCALE, normalizeLocale, type SupportedLocale } from './locales';
import { EN_MESSAGES, I18N_MESSAGES, type I18nMessageKey } from './messages';

type MessageParams = Record<string, string | number | boolean | null | undefined>;

const [activeLocaleSignal, setActiveLocaleSignal] = createSignal<SupportedLocale>(DEFAULT_LOCALE);

export const activeLocale = activeLocaleSignal;

export function getActiveLocale(): SupportedLocale {
  return activeLocaleSignal();
}

export function setActiveLocale(locale: string | null | undefined): SupportedLocale {
  const normalized = normalizeLocale(locale);
  setActiveLocaleSignal(normalized);
  return normalized;
}

export function translateMessage(
  key: I18nMessageKey,
  params: MessageParams = {},
  locale: string | null | undefined = activeLocaleSignal(),
): string {
  const normalizedLocale = normalizeLocale(locale);
  const template = I18N_MESSAGES[normalizedLocale][key] ?? EN_MESSAGES[key];
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
  NEXT_LOCALIZATION_LOCALES,
  SUPPORTED_LOCALE_LABELS,
  SUPPORTED_LOCALES,
  isSupportedLocale,
  normalizeLocale,
  type SupportedLocale,
} from './locales';
