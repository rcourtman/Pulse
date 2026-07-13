/**
 * Branch-coverage tests for settingsHeaderMeta.
 *
 * Target functions:
 *   - getSettingsHeaderMeta(locale?)                      (exported)
 *   - getInfrastructureReadOnlyHeaderDescription(locale?) (exported)
 *
 * The sibling `settingsLocalization.test.ts` suite already exercises:
 *   - getSettingsHeaderMeta('es') for: system-network, system-ai,
 *     system-ai-assistant, system-billing
 *   - getSettingsHeaderMeta('de') for: system-ai, system-ai-discovery,
 *     system-ai-assistant, system-billing
 *   - getSettingsHeaderMeta() (no-arg via active 'es') for security-auth.title
 *   - getInfrastructureReadOnlyHeaderDescription('es') via .toContain('solo lectura')
 *   - static SETTINGS_HEADER_META spot checks (system-billing, system-ai-discovery)
 *
 * This file pins the UNCOVERED branches of the two target functions:
 *   - English baseline (the getLocaleFallbackChain single-element [en] arm and
 *     the getLocaleMessage `!deferredLocale` EN_MESSAGES arm) for every SettingsTab
 *     the sibling suite leaves untouched.
 *   - locale argument variants: undefined (translateMessage default-parameter arm),
 *     explicit 'en', null, and an unsupported locale string (both normalized to
 *     DEFAULT_LOCALE via resolveSupportedLocale -> normalizeLocale).
 *   - Map parity: every SettingsTab key present with non-empty title+description,
 *     dynamic English output matches the static SETTINGS_HEADER_META (drift guard),
 *     and Object.fromEntries yields a fresh object per call.
 *   - getInfrastructureReadOnlyHeaderDescription across 'en' / 'de' / 'es' and the
 *     no-arg (active-locale) arm plus null / unsupported-string normalization.
 */
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { DEFAULT_LOCALE, setActiveLocale, type SupportedLocale } from '@/i18n';
import type { SettingsHeaderMetaMap } from '../settingsNavigationModel';
import {
  getInfrastructureReadOnlyHeaderDescription,
  getSettingsHeaderMeta,
  SETTINGS_HEADER_META,
} from '../settingsHeaderMeta';
import type { SettingsTab } from '../settingsNavigationModel';

// Every SettingsTab from settingsNavigationModel.ts. Used for key-parity checks.
const ALL_TABS: readonly SettingsTab[] = [
  'infrastructure-systems',
  'monitoring-availability',
  'system-general',
  'system-network',
  'system-updates',
  'system-recovery',
  'system-ai',
  'system-ai-patrol',
  'system-ai-assistant',
  'system-ai-discovery',
  'system-relay',
  'system-billing',
  'support-diagnostics',
  'support-reporting',
  'support-logs',
  'organization-overview',
  'organization-access',
  'organization-sharing',
  'organization-billing',
  'organization-billing-admin',
  'api',
  'security-overview',
  'security-data-handling',
  'security-auth',
  'security-sso',
  'security-roles',
  'security-users',
  'security-audit',
  'security-webhooks',
];

describe('getSettingsHeaderMeta', () => {
  afterEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  // ---- English baseline -------------------------------------------------
  // getLocaleFallbackChain('en') returns [en] (the locale === fallbackLocale
  // single-element arm) and getLocaleMessage returns EN_MESSAGES[key] via the
  // `!deferredLocale` arm. The sibling suite only pins these tabs in es/de, so
  // every tab here is a newly-covered English entry.
  describe('English baseline (single-element [en] fallback chain)', () => {
    // Computed inside beforeAll so the i18n runtime (catalogVersion signal,
    // active-locale default) is fully initialized at evaluation time.
    let en: SettingsHeaderMetaMap;
    beforeAll(() => {
      en = getSettingsHeaderMeta('en');
    });

    it('localizes infrastructure-systems to the English baseline', () => {
      expect(en['infrastructure-systems']).toEqual({
        title: 'Infrastructure',
        description: 'Add, discover, and verify the infrastructure Pulse monitors.',
      });
    });

    it('localizes monitoring-availability to the English baseline', () => {
      expect(en['monitoring-availability']).toEqual({
        title: 'Availability checks',
        description:
          'Monitor endpoint-only devices and services with ping, TCP, and HTTP probes.',
      });
    });

    it('localizes system-general to the English baseline', () => {
      expect(en['system-general']).toEqual({
        title: 'General',
        description: 'Manage appearance, layout, and default monitoring cadence.',
      });
    });

    it('localizes system-network to the English baseline (sibling only pinned es/de)', () => {
      expect(en['system-network']).toEqual({
        title: 'Network',
        description:
          'Configure the public URL, CORS, embedding, and webhook network boundaries.',
      });
    });

    it('localizes system-updates to the English baseline', () => {
      expect(en['system-updates']).toEqual({
        title: 'Updates',
        description: 'Manage version checks, update channels, and automatic update behavior.',
      });
    });

    it('localizes system-recovery to the English baseline', () => {
      expect(en['system-recovery']).toEqual({
        title: 'Recovery',
        description:
          'Manage backup/snapshot polling plus configuration export and import workflows.',
      });
    });

    it('localizes system-ai to the English baseline (sibling only pinned es/de)', () => {
      expect(en['system-ai']).toEqual({
        title: 'Provider & Models',
        description:
          'Configure providers, default models, provider health, budget, and usage for Pulse Intelligence.',
      });
    });

    it('localizes system-ai-patrol to the English baseline', () => {
      expect(en['system-ai-patrol']).toEqual({
        title: 'Patrol',
        description: 'Set when Patrol runs, what starts it, and which model it uses.',
      });
    });

    it('localizes system-ai-assistant to the English baseline (sibling only pinned es/de)', () => {
      expect(en['system-ai-assistant']).toEqual({
        title: 'Assistant',
        description: 'Configure Assistant chat behavior, chat action permissions, and sessions.',
      });
    });

    it('localizes system-ai-discovery to the English baseline (sibling only pinned de)', () => {
      expect(en['system-ai-discovery']).toEqual({
        title: 'Service Context',
        description:
          'Configure the model-backed service context Assistant and Patrol use. Infrastructure discovery and onboarding stay under Infrastructure.',
      });
    });

    it('localizes system-relay to the English baseline', () => {
      expect(en['system-relay']).toEqual({
        title: 'Remote Access',
        description:
          'Configure Pulse relay connectivity for secure remote access and Pulse Mobile pairing.',
      });
    });

    it('localizes system-billing to the English baseline (sibling only pinned es/de)', () => {
      expect(en['system-billing']).toEqual({
        title: 'Plans & Billing',
        description: 'Plan, license, and Patrol mode for this instance.',
      });
    });

    it('localizes support-diagnostics to the English baseline', () => {
      expect(en['support-diagnostics']).toEqual({
        title: 'Diagnostics & Health',
        description:
          'Run health checks, validate connectivity, and export troubleshooting snapshots.',
      });
    });

    it('localizes support-reporting to the English baseline', () => {
      expect(en['support-reporting']).toEqual({
        title: 'Data & Reports',
        description:
          'Export inventory data and generate performance reports from the canonical settings shell.',
      });
    });

    it('localizes support-logs to the English baseline', () => {
      expect(en['support-logs']).toEqual({
        title: 'System Logs',
        description:
          'Inspect the live Pulse log stream and download the captured buffer for support work.',
      });
    });

    it('localizes organization-overview to the English baseline', () => {
      expect(en['organization-overview']).toEqual({
        title: 'Organization Overview',
        description: 'Review organization metadata, membership footprint, and ownership.',
      });
    });

    it('localizes organization-access to the English baseline', () => {
      expect(en['organization-access']).toEqual({
        title: 'Organization Access',
        description: 'Manage organization invitations, member roles, and ownership transfers.',
      });
    });

    it('localizes organization-sharing to the English baseline', () => {
      expect(en['organization-sharing']).toEqual({
        title: 'Organization Sharing',
        description: 'Share resources between organizations with scoped access.',
      });
    });

    it('localizes organization-billing to the English baseline', () => {
      expect(en['organization-billing']).toEqual({
        title: 'Billing & Usage',
        description:
          'Review your organization plan, applicable usage policies, and subscription status for paid access.',
      });
    });

    it('localizes organization-billing-admin to the English baseline', () => {
      expect(en['organization-billing-admin']).toEqual({
        title: 'Billing Admin',
        description:
          'Review and manage tenant billing state across all organizations (hosted mode only).',
      });
    });

    it('localizes api to the English baseline', () => {
      expect(en['api']).toEqual({
        title: 'API Access',
        description:
          'Generate and manage scoped Pulse tokens for agents, automation, and external integrations.',
      });
    });

    it('localizes security-overview to the English baseline', () => {
      expect(en['security-overview']).toEqual({
        title: 'Security Overview',
        description: 'View your security posture at a glance and monitor authentication status.',
      });
    });

    it('localizes security-data-handling to the English baseline', () => {
      expect(en['security-data-handling']).toEqual({
        title: 'Resource Privacy',
        description:
          'See which monitored resource details can be summarized, must stay local, or are redacted.',
      });
    });

    it('localizes security-auth to the English baseline (sibling only pinned .title via es)', () => {
      expect(en['security-auth']).toEqual({
        title: 'Authentication',
        description: 'Manage password-based authentication and credential rotation.',
      });
    });

    it('localizes security-sso to the English baseline', () => {
      expect(en['security-sso']).toEqual({
        title: 'Single Sign-On Providers',
        description: 'Configure OIDC and SAML identity providers.',
      });
    });

    it('localizes security-roles to the English baseline', () => {
      expect(en['security-roles']).toEqual({
        title: 'Roles',
        description: 'Define custom roles and manage granular permissions for users and tokens.',
      });
    });

    it('localizes security-users to the English baseline', () => {
      expect(en['security-users']).toEqual({
        title: 'User Access',
        description:
          'Assign roles to users and view effective permissions across your infrastructure.',
      });
    });

    it('localizes security-audit to the English baseline', () => {
      expect(en['security-audit']).toEqual({
        title: 'Audit Log',
        description: 'View security events, login attempts, and configuration changes.',
      });
    });

    it('localizes security-webhooks to the English baseline', () => {
      expect(en['security-webhooks']).toEqual({
        title: 'Audit Webhooks',
        description: 'Configure real-time delivery of audit events to external systems.',
      });
    });
  });

  // ---- locale argument handling ----------------------------------------
  describe('locale argument handling', () => {
    it('uses the active locale when called with no argument (translateMessage default-parameter arm)', () => {
      // Default active locale here is DEFAULT_LOCALE ('en'); the no-arg call
      // must match an explicit 'en' call.
      expect(getSettingsHeaderMeta()).toEqual(getSettingsHeaderMeta('en'));
    });

    it('honors an active non-default locale when no argument is passed', () => {
      setActiveLocale('de');
      const noArg = getSettingsHeaderMeta();
      // Concrete value pin proves the active-locale path really localized.
      expect(noArg['security-auth'].title).toBe('Authentifizierung');
      expect(noArg).toEqual(getSettingsHeaderMeta('de'));
    });

    it('normalizes an explicit null locale to DEFAULT_LOCALE (resolveSupportedLocale ?? DEFAULT_LOCALE arm)', () => {
      const nullMeta = getSettingsHeaderMeta(null as unknown as SupportedLocale);
      expect(nullMeta).toEqual(getSettingsHeaderMeta('en'));
    });

    it('normalizes an unsupported locale string to DEFAULT_LOCALE (base-locale lookup misses -> DEFAULT_LOCALE arm)', () => {
      // 'fr' is listed in NEXT_LOCALIZATION_LOCALES but is not a SupportedLocale,
      // so resolveSupportedLocale returns null and normalizeLocale falls back.
      const unsupported = getSettingsHeaderMeta('fr' as unknown as SupportedLocale);
      expect(unsupported).toEqual(getSettingsHeaderMeta('en'));
    });
  });

  // ---- map shape and key parity ----------------------------------------
  describe('map shape and key parity', () => {
    it('returns exactly one entry per SettingsTab', () => {
      const meta = getSettingsHeaderMeta('en');
      expect(Object.keys(meta).sort()).toEqual([...ALL_TABS].sort());
    });

    it('every entry has a non-empty string title and description', () => {
      const meta = getSettingsHeaderMeta('en');
      for (const tab of ALL_TABS) {
        const entry = meta[tab];
        expect(typeof entry.title).toBe('string');
        expect(entry.title.length).toBeGreaterThan(0);
        expect(typeof entry.description).toBe('string');
        expect(entry.description.length).toBeGreaterThan(0);
      }
    });

    it('keeps the dynamic English output in sync with the static SETTINGS_HEADER_META (drift guard)', () => {
      // SETTINGS_HEADER_META is the hand-maintained static baseline; the dynamic
      // getter reads the English i18n catalog. The two MUST agree for every tab
      // or copy edits silently bypass localization. (See GLM_REPORT.md.)
      const dynamic = getSettingsHeaderMeta('en');
      for (const tab of ALL_TABS) {
        expect(dynamic[tab]).toEqual(SETTINGS_HEADER_META[tab]);
      }
    });

    it('returns a fresh object on each call (Object.fromEntries yields a new map)', () => {
      const a = getSettingsHeaderMeta('en');
      const b = getSettingsHeaderMeta('en');
      expect(a).not.toBe(b);
      expect(a).toEqual(b);
    });
  });

  // ---- localized catalogs: uncovered tabs ------------------------------
  describe('German catalog — tabs not pinned by the sibling suite', () => {
    // Sibling already covers system-ai, system-ai-discovery, system-ai-assistant,
    // and system-billing in 'de'. Every other tab is pinned here so the German
    // fallback chain ([de, en]) resolves to the de override for each key.
    // Computed in beforeAll so the deferred 'de' catalog (loaded asynchronously
    // by setup.ts) is present at evaluation time.
    let de: SettingsHeaderMetaMap;
    beforeAll(() => {
      de = getSettingsHeaderMeta('de');
    });

    it('localizes infrastructure-systems', () => {
      expect(de['infrastructure-systems']).toEqual({
        title: 'Infrastruktur',
        description:
          'Fuegen Sie Infrastruktur hinzu, finden Sie sie automatisch, und pruefen Sie, was Pulse ueberwacht.',
      });
    });

    it('localizes monitoring-availability', () => {
      expect(de['monitoring-availability']).toEqual({
        title: 'Verfuegbarkeitspruefungen',
        description:
          'Ueberwachen Sie Endgeraete und Dienste nur ueber Ping-, TCP- und HTTP-Pruefungen.',
      });
    });

    it('localizes system-general', () => {
      expect(de['system-general']).toEqual({
        title: 'Allgemein',
        description: 'Verwalten Sie Darstellung, Layout und Standard-Monitoring-Takt.',
      });
    });

    it('localizes system-network', () => {
      expect(de['system-network']).toEqual({
        title: 'Netzwerk',
        description:
          'Konfigurieren Sie oeffentliche URL, CORS, Einbettung und Netzwerkgrenzen fuer Webhooks.',
      });
    });

    it('localizes system-updates', () => {
      expect(de['system-updates']).toEqual({
        title: 'Updates',
        description:
          'Verwalten Sie Versionspruefungen, Update-Kanaele und automatisches Update-Verhalten.',
      });
    });

    it('localizes system-recovery', () => {
      expect(de['system-recovery']).toEqual({
        title: 'Wiederherstellung',
        description:
          'Verwalten Sie Backup-/Snapshot-Abfragen sowie Export- und Importablaeufe der Konfiguration.',
      });
    });

    it('localizes system-ai-patrol', () => {
      expect(de['system-ai-patrol']).toEqual({
        title: 'Patrol',
        description:
          'Legen Sie fest, wann Patrol laeuft, was Patrol startet und welches Modell verwendet wird.',
      });
    });

    it('localizes system-relay', () => {
      expect(de['system-relay']).toEqual({
        title: 'Remote-Zugriff',
        description:
          'Konfigurieren Sie Pulse-Relay-Konnektivitaet fuer sicheren Remote-Zugriff und Pulse-Mobile-Kopplung.',
      });
    });

    it('localizes support-diagnostics', () => {
      expect(de['support-diagnostics']).toEqual({
        title: 'Diagnose & Zustand',
        description:
          'Fuehren Sie Zustandspruefungen aus, validieren Sie Verbindungen und exportieren Sie Troubleshooting-Snapshots.',
      });
    });

    it('localizes support-reporting', () => {
      expect(de['support-reporting']).toEqual({
        title: 'Daten & Berichte',
        description:
          'Exportieren Sie Inventardaten und erstellen Sie Leistungsberichte aus der kanonischen Einstellungsoberflaeche.',
      });
    });

    it('localizes support-logs', () => {
      expect(de['support-logs']).toEqual({
        title: 'Systemprotokolle',
        description:
          'Pruefen Sie den Live-Pulse-Logstream und laden Sie den erfassten Puffer fuer Supportarbeit herunter.',
      });
    });

    it('localizes organization-overview', () => {
      expect(de['organization-overview']).toEqual({
        title: 'Organisationsuebersicht',
        description: 'Pruefen Sie Organisationsdaten, Mitgliederumfang und Eigentum.',
      });
    });

    it('localizes organization-access', () => {
      expect(de['organization-access']).toEqual({
        title: 'Organisationszugriff',
        description:
          'Verwalten Sie Einladungen, Mitgliederrollen und Eigentumsuebertragungen der Organisation.',
      });
    });

    it('localizes organization-sharing', () => {
      expect(de['organization-sharing']).toEqual({
        title: 'Organisationsfreigabe',
        description: 'Teilen Sie Ressourcen zwischen Organisationen mit begrenztem Zugriff.',
      });
    });

    it('localizes organization-billing', () => {
      expect(de['organization-billing']).toEqual({
        title: 'Abrechnung & Nutzung',
        description:
          'Pruefen Sie Organisationsplan, geltende Nutzungsrichtlinien und Abonnementstatus fuer bezahlten Zugriff.',
      });
    });

    it('localizes organization-billing-admin', () => {
      expect(de['organization-billing-admin']).toEqual({
        title: 'Abrechnungsadmin',
        description:
          'Pruefen und verwalten Sie den Abrechnungsstatus aller Mandanten (nur Hosted-Modus).',
      });
    });

    it('localizes api', () => {
      expect(de['api']).toEqual({
        title: 'API-Zugriff',
        description:
          'Erstellen und verwalten Sie begrenzte Pulse-Tokens fuer Agents, Automatisierung und externe Integrationen.',
      });
    });

    it('localizes security-overview', () => {
      expect(de['security-overview']).toEqual({
        title: 'Sicherheitsuebersicht',
        description:
          'Sehen Sie Ihre Sicherheitslage auf einen Blick und ueberwachen Sie den Authentifizierungsstatus.',
      });
    });

    it('localizes security-data-handling', () => {
      expect(de['security-data-handling']).toEqual({
        title: 'Ressourcenschutz',
        description:
          'Sehen Sie, welche ueberwachten Ressourcendetails zusammengefasst werden duerfen, lokal bleiben muessen oder redigiert sind.',
      });
    });

    it('localizes security-auth', () => {
      expect(de['security-auth']).toEqual({
        title: 'Authentifizierung',
        description:
          'Verwalten Sie passwortbasierte Authentifizierung und Zugangsdatenrotation.',
      });
    });

    it('localizes security-sso', () => {
      expect(de['security-sso']).toEqual({
        title: 'Single-Sign-On-Anbieter',
        description: 'Konfigurieren Sie OIDC- und SAML-Identity-Provider.',
      });
    });

    it('localizes security-roles', () => {
      expect(de['security-roles']).toEqual({
        title: 'Rollen',
        description:
          'Definieren Sie eigene Rollen und verwalten Sie granulare Berechtigungen fuer Benutzer und Tokens.',
      });
    });

    it('localizes security-users', () => {
      expect(de['security-users']).toEqual({
        title: 'Benutzerzugriff',
        description:
          'Weisen Sie Benutzern Rollen zu und sehen Sie effektive Berechtigungen in Ihrer Infrastruktur.',
      });
    });

    it('localizes security-audit', () => {
      expect(de['security-audit']).toEqual({
        title: 'Audit-Protokoll',
        description:
          'Sehen Sie Sicherheitsereignisse, Anmeldeversuche und Konfigurationsaenderungen.',
      });
    });

    it('localizes security-webhooks', () => {
      expect(de['security-webhooks']).toEqual({
        title: 'Audit-Webhooks',
        description:
          'Konfigurieren Sie die Echtzeit-Zustellung von Audit-Ereignissen an externe Systeme.',
      });
    });
  });

  describe('Spanish catalog — tabs not pinned by the sibling suite', () => {
    // Sibling already covers system-network, system-ai, system-ai-assistant,
    // and system-billing in 'es'. Every other tab is pinned here.
    // Computed in beforeAll so the deferred 'es' catalog is present.
    let es: SettingsHeaderMetaMap;
    beforeAll(() => {
      es = getSettingsHeaderMeta('es');
    });

    it('localizes infrastructure-systems', () => {
      expect(es['infrastructure-systems']).toEqual({
        title: 'Infraestructura',
        description: 'Agrega, descubre y verifica la infraestructura que Pulse supervisa.',
      });
    });

    it('localizes monitoring-availability', () => {
      expect(es['monitoring-availability']).toEqual({
        title: 'Comprobaciones de disponibilidad',
        description:
          'Supervisa dispositivos y servicios solo por endpoint con pruebas ping, TCP y HTTP.',
      });
    });

    it('localizes system-general', () => {
      expect(es['system-general']).toEqual({
        title: 'General',
        description: 'Administra apariencia, diseño y cadencia predeterminada de supervisión.',
      });
    });

    it('localizes system-updates', () => {
      expect(es['system-updates']).toEqual({
        title: 'Actualizaciones',
        description:
          'Administra comprobaciones de versión, canales de actualización y comportamiento de actualización automática.',
      });
    });

    it('localizes system-recovery', () => {
      expect(es['system-recovery']).toEqual({
        title: 'Recuperación',
        description:
          'Administra sondeos de copias/snapshots y flujos de exportación e importación de configuración.',
      });
    });

    it('localizes system-ai-patrol', () => {
      expect(es['system-ai-patrol']).toEqual({
        title: 'Patrol',
        description: 'Configura cuando se ejecuta Patrol, que lo inicia y que modelo usa.',
      });
    });

    it('localizes system-ai-discovery title (sibling only pinned de for this tab)', () => {
      expect(es['system-ai-discovery'].title).toBe('Contexto de servicio');
    });

    it('localizes system-relay', () => {
      expect(es['system-relay']).toEqual({
        title: 'Acceso remoto',
        description:
          'Configura la conectividad de relay de Pulse para acceso remoto seguro y vinculación con Pulse Mobile.',
      });
    });

    it('localizes support-diagnostics', () => {
      expect(es['support-diagnostics']).toEqual({
        title: 'Diagnóstico y salud',
        description:
          'Ejecuta comprobaciones de salud, valida conectividad y exporta snapshots de resolución de problemas.',
      });
    });

    it('localizes support-reporting', () => {
      expect(es['support-reporting']).toEqual({
        title: 'Datos e informes',
        description:
          'Exporta datos de inventario y genera informes de rendimiento desde la consola canónica de ajustes.',
      });
    });

    it('localizes support-logs', () => {
      expect(es['support-logs']).toEqual({
        title: 'Logs del sistema',
        description:
          'Inspecciona el flujo de logs de Pulse en vivo y descarga el búfer capturado para soporte.',
      });
    });

    it('localizes organization-overview', () => {
      expect(es['organization-overview']).toEqual({
        title: 'Resumen de organización',
        description: 'Revisa metadatos, alcance de miembros y propiedad de la organización.',
      });
    });

    it('localizes organization-access', () => {
      expect(es['organization-access']).toEqual({
        title: 'Acceso de organización',
        description:
          'Administra invitaciones, roles de miembros y transferencias de propiedad de la organización.',
      });
    });

    it('localizes organization-sharing', () => {
      expect(es['organization-sharing']).toEqual({
        title: 'Uso compartido de organización',
        description: 'Comparte recursos entre organizaciones con acceso delimitado.',
      });
    });

    it('localizes organization-billing', () => {
      expect(es['organization-billing']).toEqual({
        title: 'Facturación y uso',
        description:
          'Revisa el plan de la organización, las políticas de uso aplicables y el estado de suscripción para acceso de pago.',
      });
    });

    it('localizes organization-billing-admin', () => {
      expect(es['organization-billing-admin']).toEqual({
        title: 'Admin de facturación',
        description:
          'Revisa y administra el estado de facturación de todos los tenants (solo modo alojado).',
      });
    });

    it('localizes api', () => {
      expect(es['api']).toEqual({
        title: 'Acceso API',
        description:
          'Genera y administra tokens de Pulse con alcance limitado para agentes, automatización e integraciones externas.',
      });
    });

    it('localizes security-overview', () => {
      expect(es['security-overview']).toEqual({
        title: 'Resumen de seguridad',
        description:
          'Consulta tu postura de seguridad de un vistazo y supervisa el estado de autenticación.',
      });
    });

    it('localizes security-data-handling', () => {
      expect(es['security-data-handling']).toEqual({
        title: 'Privacidad de recursos',
        description:
          'Consulta qué detalles de recursos supervisados se pueden resumir, deben quedarse locales o están redactados.',
      });
    });

    it('localizes security-auth (full entry, sibling only checked .title via active locale)', () => {
      expect(es['security-auth']).toEqual({
        title: 'Autenticación',
        description: 'Administra autenticación con contraseña y rotación de credenciales.',
      });
    });

    it('localizes security-sso', () => {
      expect(es['security-sso']).toEqual({
        title: 'Proveedores de inicio de sesión único',
        description: 'Configura proveedores de identidad OIDC y SAML.',
      });
    });

    it('localizes security-roles', () => {
      expect(es['security-roles']).toEqual({
        title: 'Roles',
        description:
          'Define roles personalizados y administra permisos granulares para usuarios y tokens.',
      });
    });

    it('localizes security-users', () => {
      expect(es['security-users']).toEqual({
        title: 'Acceso de usuarios',
        description:
          'Asigna roles a usuarios y consulta permisos efectivos en tu infraestructura.',
      });
    });

    it('localizes security-audit', () => {
      expect(es['security-audit']).toEqual({
        title: 'Registro de auditoría',
        description:
          'Consulta eventos de seguridad, intentos de inicio de sesión y cambios de configuración.',
      });
    });

    it('localizes security-webhooks', () => {
      expect(es['security-webhooks']).toEqual({
        title: 'Webhooks de auditoría',
        description:
          'Configura la entrega en tiempo real de eventos de auditoría a sistemas externos.',
      });
    });
  });
});

describe('getInfrastructureReadOnlyHeaderDescription', () => {
  afterEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('returns the English baseline when locale is "en" (single-element fallback arm)', () => {
    expect(getInfrastructureReadOnlyHeaderDescription('en')).toBe(
      'Review the current top-level monitored systems and reporting posture. Setup changes stay unavailable in this read-only session.',
    );
  });

  it('returns the German translation when locale is "de"', () => {
    expect(getInfrastructureReadOnlyHeaderDescription('de')).toBe(
      'Pruefen Sie die aktuell ueberwachten Hauptsysteme und den Reporting-Status. Einrichtungsänderungen bleiben in dieser schreibgeschuetzten Sitzung nicht verfuegbar.',
    );
  });

  it('returns the full Spanish translation (sibling only asserted .toContain)', () => {
    expect(getInfrastructureReadOnlyHeaderDescription('es')).toBe(
      'Revisa los sistemas principales supervisados y el estado de informes. Los cambios de configuración no están disponibles en esta sesión de solo lectura.',
    );
  });

  it('uses the active locale when called with no argument (translateMessage default-parameter arm)', () => {
    // Default active locale is DEFAULT_LOCALE ('en').
    expect(getInfrastructureReadOnlyHeaderDescription()).toBe(
      getInfrastructureReadOnlyHeaderDescription('en'),
    );
  });

  it('honors an active non-default locale when no argument is passed', () => {
    setActiveLocale('de');
    expect(getInfrastructureReadOnlyHeaderDescription()).toBe(
      getInfrastructureReadOnlyHeaderDescription('de'),
    );
  });

  it('normalizes an explicit null locale to DEFAULT_LOCALE', () => {
    expect(
      getInfrastructureReadOnlyHeaderDescription(null as unknown as SupportedLocale),
    ).toBe(getInfrastructureReadOnlyHeaderDescription('en'));
  });

  it('normalizes an unsupported locale string to DEFAULT_LOCALE', () => {
    // 'fr' is not yet a SupportedLocale, so normalizeLocale falls back to en.
    expect(
      getInfrastructureReadOnlyHeaderDescription('fr' as unknown as SupportedLocale),
    ).toBe(getInfrastructureReadOnlyHeaderDescription('en'));
  });
});
