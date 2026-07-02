import { afterEach, describe, expect, it } from 'vitest';
import {
  ALERTS_OVERVIEW_ALLOWED_IDENTICAL_TRANSLATIONS,
  ALERTS_OVERVIEW_NON_TRANSLATABLE_TOKENS,
  COMMERCIAL_PRICING_HANDOFF_ALLOWED_IDENTICAL_TRANSLATIONS,
  COMMERCIAL_PRICING_HANDOFF_NON_TRANSLATABLE_TOKENS,
  DEFAULT_LOCALE,
  FIRST_SESSION_MONITORING_ALLOWED_IDENTICAL_TRANSLATIONS,
  FIRST_SESSION_MONITORING_NON_TRANSLATABLE_TOKENS,
  FIRST_LOCALIZATION_LOCALES,
  LOCALIZATION_FOUNDATION,
  LOCALIZED_ALERTS_OVERVIEW_JOURNEY_KEYS,
  LOCALIZED_COMMERCIAL_PRICING_HANDOFF_KEYS,
  LOCALIZED_FIRST_SESSION_MONITORING_JOURNEY_KEYS,
  LOCALIZED_SETTINGS_GENERAL_JOURNEY_KEYS,
  NEVER_TRANSLATE_COPY_RULES,
  NEXT_LOCALIZATION_LOCALES,
  SETTINGS_GENERAL_ALLOWED_IDENTICAL_TRANSLATIONS,
  SETTINGS_GENERAL_NON_TRANSLATABLE_TOKENS,
  SUPPORTED_LOCALE_REGISTRY,
  SUPPORTED_LOCALES,
  detectBrowserLocale,
  getActiveLocale,
  getInitialLocalePreference,
  getLocaleFallbackChain,
  getStoredLocalePreference,
  normalizeLocale,
  setActiveLocale,
  setLocalePreference,
  t,
  type I18nMessageKey,
} from '@/i18n';
import { I18N_MESSAGES } from '@/i18n/catalogs';
import { STORAGE_KEYS } from '@/utils/localStorage';

describe('i18n foundation', () => {
  afterEach(() => {
    localStorage.removeItem(STORAGE_KEYS.LOCALE_PREFERENCE);
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('normalizes supported locales and regional aliases', () => {
    expect(normalizeLocale('de-DE')).toBe('de');
    expect(normalizeLocale('es_MX')).toBe('es');
    expect(normalizeLocale('es_MX_traditional')).toBe('es');
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
    expect(getLocaleFallbackChain('de-DE')).toEqual(['de', 'en']);
    expect(getLocaleFallbackChain('fr-FR')).toEqual(['en']);
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
    expect(SUPPORTED_LOCALE_REGISTRY.de).toMatchObject({
      label: 'Deutsch',
      englishLabel: 'German',
      fallbackLocale: 'en',
      rolloutStage: 'first-wave',
    });
  });

  it('documents the frontend localization architecture and non-translation boundaries', () => {
    expect(LOCALIZATION_FOUNDATION).toMatchObject({
      ownerLayer: 'frontend-modern/src/i18n',
      defaultLocale: 'en',
      firstWaveLocales: ['de', 'es'],
    });
    expect(NEVER_TRANSLATE_COPY_RULES.join(' ')).toContain('environment variable names');
    expect(NEVER_TRANSLATE_COPY_RULES.join(' ')).toContain('Resource names');
    expect(SETTINGS_GENERAL_NON_TRANSLATABLE_TOKENS).toEqual(
      expect.arrayContaining([
        'Pulse',
        'Proxmox VE',
        'Docker / Podman',
        'API',
        'CPU',
        'IP',
        'JSON',
        'PULSE_TELEMETRY',
        'PVE_POLLING_INTERVAL',
        'PULSE_DISABLE_DOCKER_UPDATE_ACTIONS',
        '"Update"',
      ]),
    );
    expect(FIRST_SESSION_MONITORING_NON_TRANSLATABLE_TOKENS).toEqual(
      expect.arrayContaining([
        'Pulse',
        'Pulse Agent',
        'API',
        'URL',
        'Docker',
        'Kubernetes',
        'Proxmox',
        'TrueNAS',
        'VMware',
        'PBS',
        'PMG',
        '.bootstrap_token',
        'sudo pulse bootstrap-token',
        'docker exec',
        'pct exec',
      ]),
    );
    expect(ALERTS_OVERVIEW_NON_TRANSLATABLE_TOKENS).toEqual(
      expect.arrayContaining([
        'Pulse',
        'Pulse Assistant',
        'Pulse Alerts',
        'Pro',
        'API',
        'alertIdentifier',
        'alertType',
        'resourceId',
        'resourceName',
        'node',
        'systemctl',
      ]),
    );
    expect(COMMERCIAL_PRICING_HANDOFF_NON_TRANSLATABLE_TOKENS).toEqual(
      expect.arrayContaining(['Pulse', 'Pulse Account']),
    );
  });

  it('requires explicit first-wave translations for the migrated settings general journey', () => {
    for (const locale of FIRST_LOCALIZATION_LOCALES) {
      const allowedIdenticalKeys: ReadonlySet<I18nMessageKey> = new Set(
        (SETTINGS_GENERAL_ALLOWED_IDENTICAL_TRANSLATIONS[locale] ??
          []) as readonly I18nMessageKey[],
      );
      for (const key of LOCALIZED_SETTINGS_GENERAL_JOURNEY_KEYS) {
        expect(I18N_MESSAGES[locale][key], `${locale}:${key}`).toBeTruthy();
        if (!allowedIdenticalKeys.has(key)) {
          expect(I18N_MESSAGES[locale][key], `${locale}:${key}`).not.toBe(I18N_MESSAGES.en[key]);
        }
      }
    }
  });

  it('keeps machine-facing identifiers unchanged in first-wave settings general catalog copy', () => {
    for (const locale of FIRST_LOCALIZATION_LOCALES) {
      const telemetryDescription = I18N_MESSAGES[locale]['settings.general.telemetry.description'];

      expect(I18N_MESSAGES[locale]['settings.general.language.description']).toContain('API');
      expect(telemetryDescription).toContain('Pulse');
      expect(telemetryDescription).toContain('IP');
      expect(telemetryDescription).toContain('90');
      expect(telemetryDescription).not.toMatch(/anonymous/i);
      expect(I18N_MESSAGES[locale]['settings.general.telemetry.copyJson']).toContain('JSON');
      expect(
        t(
          'settings.general.monitoringCadence.envLocked',
          {
            envVar: 'PVE_POLLING_INTERVAL',
          },
          locale,
        ),
      ).toContain('PVE_POLLING_INTERVAL');
    }
  });

  it('requires explicit first-wave translations for the migrated first-session monitoring journey', () => {
    for (const locale of FIRST_LOCALIZATION_LOCALES) {
      const allowedIdenticalKeys: ReadonlySet<I18nMessageKey> = new Set([
        ...((SETTINGS_GENERAL_ALLOWED_IDENTICAL_TRANSLATIONS[locale] ??
          []) as readonly I18nMessageKey[]),
        ...((FIRST_SESSION_MONITORING_ALLOWED_IDENTICAL_TRANSLATIONS[locale] ??
          []) as readonly I18nMessageKey[]),
      ]);
      for (const key of LOCALIZED_FIRST_SESSION_MONITORING_JOURNEY_KEYS) {
        expect(I18N_MESSAGES[locale][key], `${locale}:${key}`).toBeTruthy();
        if (!allowedIdenticalKeys.has(key)) {
          expect(I18N_MESSAGES[locale][key], `${locale}:${key}`).not.toBe(I18N_MESSAGES.en[key]);
        }
      }
    }
  });

  it('keeps machine-facing identifiers unchanged in first-session monitoring catalog copy', () => {
    for (const locale of FIRST_LOCALIZATION_LOCALES) {
      const telemetryNotice = I18N_MESSAGES[locale]['setup.welcome.telemetryNotice.description'];

      expect(telemetryNotice).toContain('Pulse');
      expect(telemetryNotice).toContain('PULSE_TELEMETRY=false');
      expect(telemetryNotice).not.toMatch(/anonymous/i);
      expect(I18N_MESSAGES[locale]['setup.welcome.deploymentHint.dockerUnnamed']).toContain(
        '<pulse-container>',
      );
      expect(I18N_MESSAGES[locale]['setup.welcome.deploymentHint.lxc']).toContain('Proxmox');
      expect(I18N_MESSAGES[locale]['setup.welcome.error.snapshotPaste']).toContain(
        '.bootstrap_token',
      );
      expect(I18N_MESSAGES[locale]['setup.completion.download.content']).toContain('URL:');
      expect(
        I18N_MESSAGES[locale]['setup.completion.sourceOptions.platformApi.description'],
      ).toContain('Proxmox');
      expect(
        I18N_MESSAGES[locale]['setup.completion.sourceOptions.platformApi.description'],
      ).toContain('TrueNAS');
      expect(I18N_MESSAGES[locale]['setup.completion.sourceOptions.agent.description']).toContain(
        'Docker',
      );
      expect(I18N_MESSAGES[locale]['setup.security.nextScreen.itemApiToken']).toContain('API');
    }
  });

  it('requires explicit first-wave translations for the migrated alerts overview journey', () => {
    for (const locale of FIRST_LOCALIZATION_LOCALES) {
      const allowedIdenticalKeys: ReadonlySet<I18nMessageKey> = new Set([
        ...((SETTINGS_GENERAL_ALLOWED_IDENTICAL_TRANSLATIONS[locale] ??
          []) as readonly I18nMessageKey[]),
        ...((FIRST_SESSION_MONITORING_ALLOWED_IDENTICAL_TRANSLATIONS[locale] ??
          []) as readonly I18nMessageKey[]),
        ...((ALERTS_OVERVIEW_ALLOWED_IDENTICAL_TRANSLATIONS[locale] ??
          []) as readonly I18nMessageKey[]),
      ]);
      for (const key of LOCALIZED_ALERTS_OVERVIEW_JOURNEY_KEYS) {
        expect(I18N_MESSAGES[locale][key], `${locale}:${key}`).toBeTruthy();
        if (!allowedIdenticalKeys.has(key)) {
          expect(I18N_MESSAGES[locale][key], `${locale}:${key}`).not.toBe(I18N_MESSAGES.en[key]);
        }
      }
    }
  });

  it('keeps machine-facing identifiers unchanged in alerts overview catalog copy', () => {
    for (const locale of FIRST_LOCALIZATION_LOCALES) {
      expect(I18N_MESSAGES[locale]['alerts.assistant.button.full']).toContain('Pulse Assistant');
      expect(I18N_MESSAGES[locale]['alerts.assistant.sourceLabel']).toBe('Pulse Alerts');
      expect(I18N_MESSAGES[locale]['alerts.assistant.locked.proRequired']).toContain('Pro');
      expect(I18N_MESSAGES[locale]['alerts.assistant.patrol.menuLabel']).toBeTruthy();
      expect(I18N_MESSAGES[locale]['alerts.assistant.patrol.menuLabel']).toContain('Patrol');
      expect(I18N_MESSAGES[locale]['alerts.assistant.patrol.title']).toContain('Patrol');
      expect(I18N_MESSAGES[locale]['alerts.assistant.explain.menuLabel']).toContain('Assistant');
      expect(
        t(
          'alerts.assistant.action.investigate',
          {
            alertIdentifier: 'alert:vm-101:cpu',
          },
          locale,
        ),
      ).toContain('alert:vm-101:cpu');
      expect(
        t(
          'alerts.assistant.subject',
          {
            level: 'warning',
            alertType: 'cpu',
            resourceName: 'db-vm-01',
          },
          locale,
        ),
      ).toContain('db-vm-01');
      expect(
        t(
          'alerts.assistant.subject',
          {
            level: 'warning',
            alertType: 'cpu',
            resourceName: 'db-vm-01',
          },
          locale,
        ),
      ).toContain('cpu');
    }
  });

  it('requires explicit first-wave translations for the migrated commercial pricing handoff', () => {
    for (const locale of FIRST_LOCALIZATION_LOCALES) {
      const allowedIdenticalKeys: ReadonlySet<I18nMessageKey> = new Set([
        ...((COMMERCIAL_PRICING_HANDOFF_ALLOWED_IDENTICAL_TRANSLATIONS[locale] ??
          []) as readonly I18nMessageKey[]),
      ]);
      for (const key of LOCALIZED_COMMERCIAL_PRICING_HANDOFF_KEYS) {
        expect(I18N_MESSAGES[locale][key], `${locale}:${key}`).toBeTruthy();
        if (!allowedIdenticalKeys.has(key)) {
          expect(I18N_MESSAGES[locale][key], `${locale}:${key}`).not.toBe(I18N_MESSAGES.en[key]);
        }
      }
    }
  });

  it('keeps Pulse Account untranslated in commercial pricing handoff copy', () => {
    for (const locale of FIRST_LOCALIZATION_LOCALES) {
      expect(I18N_MESSAGES[locale]['pricing.handoff.title.pulseAccount']).toContain(
        'Pulse Account',
      );
      expect(I18N_MESSAGES[locale]['pricing.handoff.link.pulseAccount']).toContain('Pulse Account');
    }
  });
});
