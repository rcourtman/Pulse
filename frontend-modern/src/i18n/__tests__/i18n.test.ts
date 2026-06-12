import { afterEach, describe, expect, it } from 'vitest';
import {
  DEFAULT_LOCALE,
  FIRST_LOCALIZATION_LOCALES,
  I18N_MESSAGES,
  NEXT_LOCALIZATION_LOCALES,
  SUPPORTED_LOCALES,
  getActiveLocale,
  normalizeLocale,
  setActiveLocale,
  t,
  type I18nMessageKey,
} from '@/i18n';

describe('i18n foundation', () => {
  afterEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('normalizes supported locales and regional aliases', () => {
    expect(normalizeLocale('de-DE')).toBe('de');
    expect(normalizeLocale('es_MX')).toBe('es');
    expect(normalizeLocale('fr-FR')).toBe('en');
    expect(normalizeLocale(undefined)).toBe('en');
  });

  it('sets and reads the active locale through the normalized locale contract', () => {
    expect(setActiveLocale('es-ES')).toBe('es');
    expect(getActiveLocale()).toBe('es');
    expect(t('settings.shell.navigationTitle')).toBe('Ajustes');
  });

  it('falls back to the English catalog for unsupported locales and interpolates params', () => {
    expect(t('settings.shell.searchEmpty', { query: 'alerts' }, 'pt-BR')).toBe(
      'No settings found for "alerts"',
    );
  });

  it('keeps every supported locale complete against the English key set', () => {
    const englishKeys = Object.keys(I18N_MESSAGES.en).sort() as I18nMessageKey[];

    for (const locale of SUPPORTED_LOCALES) {
      expect(Object.keys(I18N_MESSAGES[locale]).sort()).toEqual(englishKeys);
    }
  });

  it('captures the first and next localization waves without enabling unsupported locales', () => {
    expect(FIRST_LOCALIZATION_LOCALES).toEqual(['de', 'es']);
    expect(NEXT_LOCALIZATION_LOCALES).toEqual(['fr', 'pt-BR', 'ja', 'zh-Hans', 'ko']);
    expect(SUPPORTED_LOCALES).toEqual(['en', 'de', 'es']);
  });
});
