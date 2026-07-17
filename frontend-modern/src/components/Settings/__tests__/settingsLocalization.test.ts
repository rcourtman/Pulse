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
      title: 'Plans & Billing',
      description: 'Plan, license, and Patrol mode for this instance.',
    });
    expect(SETTINGS_HEADER_META['system-ai-discovery']).toEqual({
      title: 'Service Context',
      description:
        'Configure the model-backed service context Assistant and Patrol use. Infrastructure discovery and onboarding stay under Infrastructure.',
    });
  });

  it('localizes settings navigation groups and items for German', () => {
    const groups = getSettingsNavGroups('de');
    expect(groups[0]?.label).toBe('Infrastruktur');
    expect(groups[0]?.items[0]?.label).toBe('Infrastruktur');
    expect(getSettingsNavItem('system-updates', 'de')?.label).toBe('Updates');
    expect(getSettingsNavItem('system-ai-assistant', 'de')?.label).toBe('Assistant');
    expect(getSettingsNavItem('system-ai-discovery', 'de')).toBeUndefined();
    expect(getSettingsNavItem('security-data-handling', 'de')?.label).toBe('Ressourcenschutz');
  });

  it('localizes settings navigation groups and items for Spanish', () => {
    const groups = getSettingsNavGroups('es');
    expect(groups.find((group) => group.id === 'system')?.label).toBe('Sistema');
    expect(getSettingsNavItem('system-ai', 'es')?.label).toBe('Proveedores y modelos');
    expect(getSettingsNavItem('system-ai-patrol', 'es')?.label).toBe('Patrol');
    expect(getSettingsNavItem('system-ai-assistant', 'es')?.label).toBe('Assistant');
    expect(getSettingsNavItem('system-ai-discovery', 'es')).toBeUndefined();
    expect(getSettingsNavItem('support-diagnostics', 'es')?.label).toBe('Diagnóstico y salud');
  });

  it('localizes the value-first Remote Access header in every catalog locale', () => {
    expect(SETTINGS_HEADER_META['system-relay']).toEqual({
      title: 'Remote Access',
      description:
        'Check on your systems and get alert push notifications anywhere with the Pulse Mobile app — no port forwarding or VPN required.',
    });
    expect(getSettingsHeaderMeta('es')['system-relay']).toEqual({
      title: 'Acceso remoto',
      description:
        'Consulta tus sistemas y recibe notificaciones push de alertas desde cualquier lugar con la aplicación Pulse Mobile — sin abrir puertos ni VPN.',
    });
    expect(getSettingsHeaderMeta('de')['system-relay']).toEqual({
      title: 'Remote-Zugriff',
      description:
        'Behalten Sie Ihre Systeme von ueberall im Blick und erhalten Sie Alarm-Push-Benachrichtigungen ueber die Pulse-Mobile-App — ohne Portfreigaben oder VPN.',
    });
  });

  it('localizes settings header metadata and read-only infrastructure copy', () => {
    const spanishMeta = getSettingsHeaderMeta('es');
    expect(spanishMeta['system-network']).toEqual({
      title: 'Red',
      description: 'Configura la URL pública, CORS, inserción y límites de red para webhooks.',
    });
    expect(spanishMeta['system-ai']).toEqual({
      title: 'Proveedores y modelos',
      description:
        'Configura proveedores, modelos predeterminados, salud de proveedores, presupuesto y uso para Pulse Intelligence.',
    });
    expect(spanishMeta['system-ai-assistant']).toEqual({
      title: 'Assistant',
      description:
        'Configura el comportamiento del chat, los permisos de acciones, las sesiones y los conectores de agentes externos (MCP) del Assistant.',
    });
    expect(spanishMeta['system-billing']).toEqual({
      title: 'Planes y facturacion',
      description: 'Plan, licencia y modo de Patrol para esta instancia.',
    });
    expect(getSettingsHeaderMeta('de')['system-ai']).toEqual({
      title: 'Anbieter & Modelle',
      description:
        'Konfigurieren Sie Anbieter, Standardmodelle, Anbieterzustand, Budget und Nutzung fuer Pulse Intelligence.',
    });
    expect(getSettingsHeaderMeta('de')['system-ai-discovery']).toEqual({
      title: 'Service-Kontext',
      description:
        'Konfigurieren Sie den KI-gestuetzten Service-Kontext fuer Assistant und Patrol. Infrastruktur-Erkennung und Onboarding bleiben unter Infrastruktur.',
    });
    expect(getSettingsHeaderMeta('de')['system-ai-assistant']).toEqual({
      title: 'Assistant',
      description:
        'Konfigurieren Sie Chatverhalten, Aktionsrechte, Sitzungen und externe Agent-Verbindungen (MCP) des Assistant.',
    });
    expect(getSettingsHeaderMeta('de')['system-billing']).toEqual({
      title: 'Plaene & Abrechnung',
      description: 'Plan, Lizenz und Patrol-Modus fuer diese Instanz.',
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
