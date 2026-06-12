import { afterEach, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import {
  getInfrastructureReadOnlyHeaderDescription,
  getSettingsHeaderMeta,
  SETTINGS_HEADER_META,
} from '../settingsHeaderMeta';
import {
  getSettingsNavGroups,
  getSettingsNavItem,
  SETTINGS_NAV_GROUPS,
} from '../settingsNavCatalog';

describe('settings localization catalog', () => {
  afterEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('keeps the canonical English settings catalog as the static baseline', () => {
    expect(SETTINGS_NAV_GROUPS[0]?.label).toBe('Infrastructure');
    expect(SETTINGS_HEADER_META['system-billing']).toEqual({
      title: 'Self-hosted plan',
      description:
        'Review the plan this instance is using and the optional capabilities connected to it.',
    });
  });

  it('localizes settings navigation groups and items for German', () => {
    const groups = getSettingsNavGroups('de');
    expect(groups[0]?.label).toBe('Infrastruktur');
    expect(groups[0]?.items[0]?.label).toBe('Infrastruktur');
    expect(getSettingsNavItem('system-updates', 'de')?.label).toBe('Updates');
    expect(getSettingsNavItem('security-data-handling', 'de')?.label).toBe('Ressourcenschutz');
  });

  it('localizes settings navigation groups and items for Spanish', () => {
    const groups = getSettingsNavGroups('es');
    expect(groups.find((group) => group.id === 'system')?.label).toBe('Sistema');
    expect(getSettingsNavItem('system-ai', 'es')?.label).toBe('Assistant y Patrol');
    expect(getSettingsNavItem('support-diagnostics', 'es')?.label).toBe('Diagnóstico y salud');
  });

  it('localizes settings header metadata and read-only infrastructure copy', () => {
    const spanishMeta = getSettingsHeaderMeta('es');
    expect(spanishMeta['system-network']).toEqual({
      title: 'Red',
      description: 'Configura la URL pública, CORS, inserción y límites de red para webhooks.',
    });
    expect(getInfrastructureReadOnlyHeaderDescription('es')).toContain('solo lectura');
  });

  it('uses the active locale when no explicit locale is passed', () => {
    setActiveLocale('es');
    expect(getSettingsNavGroups().find((group) => group.id === 'security')?.label).toBe(
      'Seguridad',
    );
    expect(getSettingsHeaderMeta()['security-auth'].title).toBe('Autenticación');
  });
});
