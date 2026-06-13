import { afterEach, describe, expect, it } from 'vitest';
import {
  DEFAULT_LOCALE,
  FIRST_LOCALIZATION_LOCALES,
  I18N_MESSAGES,
  NEXT_LOCALIZATION_LOCALES,
  SUPPORTED_LOCALES,
  detectBrowserLocale,
  getActiveLocale,
  getInitialLocalePreference,
  getStoredLocalePreference,
  normalizeLocale,
  setActiveLocale,
  setLocalePreference,
  t,
  type I18nMessageKey,
} from '@/i18n';
import { STORAGE_KEYS } from '@/utils/localStorage';

describe('i18n foundation', () => {
  afterEach(() => {
    localStorage.removeItem(STORAGE_KEYS.LOCALE_PREFERENCE);
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

  it('detects initial locale from stored preference before browser language', () => {
    localStorage.setItem(STORAGE_KEYS.LOCALE_PREFERENCE, 'es-MX');

    expect(getStoredLocalePreference(localStorage)).toBe('es');
    expect(
      getInitialLocalePreference({
        storage: localStorage,
        browserLocales: ['de-DE'],
      }),
    ).toBe('es');
  });

  it('uses the first supported browser locale when no preference is stored', () => {
    localStorage.removeItem(STORAGE_KEYS.LOCALE_PREFERENCE);

    expect(detectBrowserLocale(['fr-FR', 'es-ES', 'de-DE'])).toBe('es');
    expect(
      getInitialLocalePreference({
        storage: localStorage,
        browserLocales: ['fr-FR', 'de-AT'],
      }),
    ).toBe('de');
  });

  it('persists explicit locale preferences separately from in-memory locale changes', () => {
    expect(setLocalePreference('de-DE', localStorage)).toBe('de');
    expect(getActiveLocale()).toBe('de');
    expect(localStorage.getItem(STORAGE_KEYS.LOCALE_PREFERENCE)).toBe('de');

    expect(setActiveLocale('es')).toBe('es');
    expect(localStorage.getItem(STORAGE_KEYS.LOCALE_PREFERENCE)).toBe('de');
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
