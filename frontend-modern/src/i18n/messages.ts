import type { SupportedLocale } from './locales';

export const EN_MESSAGES = {
  'settings.header.api.description':
    'Generate and manage scoped Pulse tokens for agents, automation, and external integrations.',
  'settings.header.api.title': 'API Access',
  'settings.header.infrastructure.description':
    'Add, discover, and verify the infrastructure Pulse monitors.',
  'settings.header.infrastructure.readOnlyDescription':
    'Review the current top-level monitored systems and reporting posture. Setup changes stay unavailable in this read-only session.',
  'settings.header.infrastructure.title': 'Infrastructure',
  'settings.header.monitoringAvailability.description':
    'Monitor endpoint-only devices and services with ping, TCP, and HTTP probes.',
  'settings.header.monitoringAvailability.title': 'Availability checks',
  'settings.header.organizationAccess.description':
    'Manage organization invitations, member roles, and ownership transfers.',
  'settings.header.organizationAccess.title': 'Organization Access',
  'settings.header.organizationBilling.description':
    'Review your organization plan, applicable usage policies, and subscription status for paid access.',
  'settings.header.organizationBilling.title': 'Billing & Usage',
  'settings.header.organizationBillingAdmin.description':
    'Review and manage tenant billing state across all organizations (hosted mode only).',
  'settings.header.organizationBillingAdmin.title': 'Billing Admin',
  'settings.header.organizationOverview.description':
    'Review organization metadata, membership footprint, and ownership.',
  'settings.header.organizationOverview.title': 'Organization Overview',
  'settings.header.organizationSharing.description':
    'Share resources between organizations with scoped access.',
  'settings.header.organizationSharing.title': 'Organization Sharing',
  'settings.header.securityAudit.description':
    'View security events, login attempts, and configuration changes.',
  'settings.header.securityAudit.title': 'Audit Log',
  'settings.header.securityAuth.description':
    'Manage password-based authentication and credential rotation.',
  'settings.header.securityAuth.title': 'Authentication',
  'settings.header.securityDataHandling.description':
    'See which monitored resource details can be summarized, must stay local, or are redacted.',
  'settings.header.securityDataHandling.title': 'Resource Privacy',
  'settings.header.securityOverview.description':
    'View your security posture at a glance and monitor authentication status.',
  'settings.header.securityOverview.title': 'Security Overview',
  'settings.header.securityRoles.description':
    'Define custom roles and manage granular permissions for users and tokens.',
  'settings.header.securityRoles.title': 'Roles',
  'settings.header.securitySso.description': 'Configure OIDC and SAML identity providers.',
  'settings.header.securitySso.title': 'Single Sign-On Providers',
  'settings.header.securityUsers.description':
    'Assign roles to users and view effective permissions across your infrastructure.',
  'settings.header.securityUsers.title': 'User Access',
  'settings.header.securityWebhooks.description':
    'Configure real-time delivery of audit events to external systems.',
  'settings.header.securityWebhooks.title': 'Audit Webhooks',
  'settings.header.systemAi.description':
    'Configure providers and models for Pulse Assistant and Patrol.',
  'settings.header.systemAi.title': 'Assistant & Patrol',
  'settings.header.systemBilling.description':
    'Review the plan this instance is using and the optional capabilities connected to it.',
  'settings.header.systemBilling.title': 'Self-hosted plan',
  'settings.header.systemGeneral.description':
    'Manage appearance, layout, and default monitoring cadence.',
  'settings.header.systemGeneral.title': 'General',
  'settings.header.systemNetwork.description':
    'Configure the public URL, CORS, embedding, and webhook network boundaries.',
  'settings.header.systemNetwork.title': 'Network',
  'settings.header.systemRecovery.description':
    'Manage backup/snapshot polling plus configuration export and import workflows.',
  'settings.header.systemRecovery.title': 'Recovery',
  'settings.header.systemRelay.description':
    'Configure Pulse relay connectivity for secure remote access and Pulse Mobile pairing.',
  'settings.header.systemRelay.title': 'Remote Access',
  'settings.header.systemUpdates.description':
    'Manage version checks, update channels, and automatic update behavior.',
  'settings.header.systemUpdates.title': 'Updates',
  'settings.header.supportDiagnostics.description':
    'Run health checks, validate connectivity, and export troubleshooting snapshots.',
  'settings.header.supportDiagnostics.title': 'Diagnostics & Health',
  'settings.header.supportLogs.description':
    'Inspect the live Pulse log stream and download the captured buffer for support work.',
  'settings.header.supportLogs.title': 'System Logs',
  'settings.header.supportReporting.description':
    'Export inventory data and generate performance reports from the canonical settings shell.',
  'settings.header.supportReporting.title': 'Data & Reports',
  'settings.nav.group.infrastructure': 'Infrastructure',
  'settings.nav.group.monitoring': 'Monitoring',
  'settings.nav.group.organization': 'Organization',
  'settings.nav.group.security': 'Security',
  'settings.nav.group.support': 'Support',
  'settings.nav.group.system': 'System',
  'settings.nav.item.apiAccess': 'API Access',
  'settings.nav.item.auditLog': 'Audit Log',
  'settings.nav.item.auditWebhooks': 'Audit Webhooks',
  'settings.nav.item.authentication': 'Authentication',
  'settings.nav.item.availabilityChecks': 'Availability checks',
  'settings.nav.item.billing': 'Billing',
  'settings.nav.item.billingAdmin': 'Billing Admin',
  'settings.nav.item.dataReports': 'Data & Reports',
  'settings.nav.item.diagnosticsHealth': 'Diagnostics & Health',
  'settings.nav.item.general': 'General',
  'settings.nav.item.infrastructure': 'Infrastructure',
  'settings.nav.item.network': 'Network',
  'settings.nav.item.organizationAccess': 'Access',
  'settings.nav.item.organizationOverview': 'Overview',
  'settings.nav.item.plans': 'Plans',
  'settings.nav.item.recovery': 'Recovery',
  'settings.nav.item.remoteAccess': 'Remote Access',
  'settings.nav.item.resourcePrivacy': 'Resource Privacy',
  'settings.nav.item.roles': 'Roles',
  'settings.nav.item.securityOverview': 'Security Overview',
  'settings.nav.item.sharing': 'Sharing',
  'settings.nav.item.singleSignOn': 'Single Sign-On',
  'settings.nav.item.systemLogs': 'System Logs',
  'settings.nav.item.updates': 'Updates',
  'settings.nav.item.users': 'Users',
  'settings.nav.item.assistantPatrol': 'Assistant & Patrol',
  'settings.shell.collapseSidebarLabel': 'Collapse settings navigation',
  'settings.shell.configurationLoading': 'Loading configuration...',
  'settings.shell.discardLabel': 'Discard',
  'settings.shell.expandSidebarLabel': 'Expand settings navigation',
  'settings.shell.loading': 'Loading settings...',
  'settings.shell.mobileBackLabel': 'Settings',
  'settings.shell.navigationAriaLabel': 'Settings navigation',
  'settings.shell.navigationTitle': 'Settings',
  'settings.shell.saveChangesLabel': 'Save Changes',
  'settings.shell.searchEmpty': 'No settings found for "{query}"',
  'settings.shell.searchPlaceholder': 'Search settings...',
  'settings.shell.unsavedDescription': 'Your changes will be lost if you navigate away.',
  'settings.shell.unsavedTitle': 'Unsaved changes',
} as const;

export type I18nMessageKey = keyof typeof EN_MESSAGES;

export const I18N_MESSAGES: Record<SupportedLocale, Record<I18nMessageKey, string>> = {
  en: EN_MESSAGES,
  de: {
    ...EN_MESSAGES,
    'settings.header.api.description':
      'Erstellen und verwalten Sie begrenzte Pulse-Tokens fuer Agents, Automatisierung und externe Integrationen.',
    'settings.header.api.title': 'API-Zugriff',
    'settings.header.infrastructure.description':
      'Fuegen Sie Infrastruktur hinzu, finden Sie sie automatisch, und pruefen Sie, was Pulse ueberwacht.',
    'settings.header.infrastructure.readOnlyDescription':
      'Pruefen Sie die aktuell ueberwachten Hauptsysteme und den Reporting-Status. Einrichtungsänderungen bleiben in dieser schreibgeschuetzten Sitzung nicht verfuegbar.',
    'settings.header.infrastructure.title': 'Infrastruktur',
    'settings.header.monitoringAvailability.description':
      'Ueberwachen Sie Endgeraete und Dienste nur ueber Ping-, TCP- und HTTP-Pruefungen.',
    'settings.header.monitoringAvailability.title': 'Verfuegbarkeitspruefungen',
    'settings.header.organizationAccess.description':
      'Verwalten Sie Einladungen, Mitgliederrollen und Eigentumsuebertragungen der Organisation.',
    'settings.header.organizationAccess.title': 'Organisationszugriff',
    'settings.header.organizationBilling.description':
      'Pruefen Sie Organisationsplan, geltende Nutzungsrichtlinien und Abonnementstatus fuer bezahlten Zugriff.',
    'settings.header.organizationBilling.title': 'Abrechnung & Nutzung',
    'settings.header.organizationBillingAdmin.description':
      'Pruefen und verwalten Sie den Abrechnungsstatus aller Mandanten (nur Hosted-Modus).',
    'settings.header.organizationBillingAdmin.title': 'Abrechnungsadmin',
    'settings.header.organizationOverview.description':
      'Pruefen Sie Organisationsdaten, Mitgliederumfang und Eigentum.',
    'settings.header.organizationOverview.title': 'Organisationsuebersicht',
    'settings.header.organizationSharing.description':
      'Teilen Sie Ressourcen zwischen Organisationen mit begrenztem Zugriff.',
    'settings.header.organizationSharing.title': 'Organisationsfreigabe',
    'settings.header.securityAudit.description':
      'Sehen Sie Sicherheitsereignisse, Anmeldeversuche und Konfigurationsaenderungen.',
    'settings.header.securityAudit.title': 'Audit-Protokoll',
    'settings.header.securityAuth.description':
      'Verwalten Sie passwortbasierte Authentifizierung und Zugangsdatenrotation.',
    'settings.header.securityAuth.title': 'Authentifizierung',
    'settings.header.securityDataHandling.description':
      'Sehen Sie, welche ueberwachten Ressourcendetails zusammengefasst werden duerfen, lokal bleiben muessen oder redigiert sind.',
    'settings.header.securityDataHandling.title': 'Ressourcenschutz',
    'settings.header.securityOverview.description':
      'Sehen Sie Ihre Sicherheitslage auf einen Blick und ueberwachen Sie den Authentifizierungsstatus.',
    'settings.header.securityOverview.title': 'Sicherheitsuebersicht',
    'settings.header.securityRoles.description':
      'Definieren Sie eigene Rollen und verwalten Sie granulare Berechtigungen fuer Benutzer und Tokens.',
    'settings.header.securityRoles.title': 'Rollen',
    'settings.header.securitySso.description':
      'Konfigurieren Sie OIDC- und SAML-Identity-Provider.',
    'settings.header.securitySso.title': 'Single-Sign-On-Anbieter',
    'settings.header.securityUsers.description':
      'Weisen Sie Benutzern Rollen zu und sehen Sie effektive Berechtigungen in Ihrer Infrastruktur.',
    'settings.header.securityUsers.title': 'Benutzerzugriff',
    'settings.header.securityWebhooks.description':
      'Konfigurieren Sie die Echtzeit-Zustellung von Audit-Ereignissen an externe Systeme.',
    'settings.header.securityWebhooks.title': 'Audit-Webhooks',
    'settings.header.systemAi.description':
      'Konfigurieren Sie Anbieter und Modelle fuer Pulse Assistant und Patrol.',
    'settings.header.systemAi.title': 'Assistant & Patrol',
    'settings.header.systemBilling.description':
      'Pruefen Sie, welchen Plan diese Instanz nutzt und welche optionalen Funktionen damit verbunden sind.',
    'settings.header.systemBilling.title': 'Self-hosted-Plan',
    'settings.header.systemGeneral.description':
      'Verwalten Sie Darstellung, Layout und Standard-Monitoring-Takt.',
    'settings.header.systemGeneral.title': 'Allgemein',
    'settings.header.systemNetwork.description':
      'Konfigurieren Sie oeffentliche URL, CORS, Einbettung und Netzwerkgrenzen fuer Webhooks.',
    'settings.header.systemNetwork.title': 'Netzwerk',
    'settings.header.systemRecovery.description':
      'Verwalten Sie Backup-/Snapshot-Abfragen sowie Export- und Importablaeufe der Konfiguration.',
    'settings.header.systemRecovery.title': 'Wiederherstellung',
    'settings.header.systemRelay.description':
      'Konfigurieren Sie Pulse-Relay-Konnektivitaet fuer sicheren Remote-Zugriff und Pulse-Mobile-Kopplung.',
    'settings.header.systemRelay.title': 'Remote-Zugriff',
    'settings.header.systemUpdates.description':
      'Verwalten Sie Versionspruefungen, Update-Kanaele und automatisches Update-Verhalten.',
    'settings.header.systemUpdates.title': 'Updates',
    'settings.header.supportDiagnostics.description':
      'Fuehren Sie Zustandspruefungen aus, validieren Sie Verbindungen und exportieren Sie Troubleshooting-Snapshots.',
    'settings.header.supportDiagnostics.title': 'Diagnose & Zustand',
    'settings.header.supportLogs.description':
      'Pruefen Sie den Live-Pulse-Logstream und laden Sie den erfassten Puffer fuer Supportarbeit herunter.',
    'settings.header.supportLogs.title': 'Systemprotokolle',
    'settings.header.supportReporting.description':
      'Exportieren Sie Inventardaten und erstellen Sie Leistungsberichte aus der kanonischen Einstellungsoberflaeche.',
    'settings.header.supportReporting.title': 'Daten & Berichte',
    'settings.nav.group.infrastructure': 'Infrastruktur',
    'settings.nav.group.monitoring': 'Monitoring',
    'settings.nav.group.organization': 'Organisation',
    'settings.nav.group.security': 'Sicherheit',
    'settings.nav.group.support': 'Support',
    'settings.nav.group.system': 'System',
    'settings.nav.item.apiAccess': 'API-Zugriff',
    'settings.nav.item.auditLog': 'Audit-Protokoll',
    'settings.nav.item.auditWebhooks': 'Audit-Webhooks',
    'settings.nav.item.authentication': 'Authentifizierung',
    'settings.nav.item.availabilityChecks': 'Verfuegbarkeitspruefungen',
    'settings.nav.item.billing': 'Abrechnung',
    'settings.nav.item.billingAdmin': 'Abrechnungsadmin',
    'settings.nav.item.dataReports': 'Daten & Berichte',
    'settings.nav.item.diagnosticsHealth': 'Diagnose & Zustand',
    'settings.nav.item.general': 'Allgemein',
    'settings.nav.item.infrastructure': 'Infrastruktur',
    'settings.nav.item.network': 'Netzwerk',
    'settings.nav.item.organizationAccess': 'Zugriff',
    'settings.nav.item.organizationOverview': 'Uebersicht',
    'settings.nav.item.plans': 'Plaene',
    'settings.nav.item.recovery': 'Wiederherstellung',
    'settings.nav.item.remoteAccess': 'Remote-Zugriff',
    'settings.nav.item.resourcePrivacy': 'Ressourcenschutz',
    'settings.nav.item.roles': 'Rollen',
    'settings.nav.item.securityOverview': 'Sicherheitsuebersicht',
    'settings.nav.item.sharing': 'Freigabe',
    'settings.nav.item.singleSignOn': 'Single Sign-On',
    'settings.nav.item.systemLogs': 'Systemprotokolle',
    'settings.nav.item.updates': 'Updates',
    'settings.nav.item.users': 'Benutzer',
    'settings.nav.item.assistantPatrol': 'Assistant & Patrol',
    'settings.shell.collapseSidebarLabel': 'Einstellungsnavigation einklappen',
    'settings.shell.configurationLoading': 'Konfiguration wird geladen...',
    'settings.shell.discardLabel': 'Verwerfen',
    'settings.shell.expandSidebarLabel': 'Einstellungsnavigation ausklappen',
    'settings.shell.loading': 'Einstellungen werden geladen...',
    'settings.shell.mobileBackLabel': 'Einstellungen',
    'settings.shell.navigationAriaLabel': 'Einstellungsnavigation',
    'settings.shell.navigationTitle': 'Einstellungen',
    'settings.shell.saveChangesLabel': 'Aenderungen speichern',
    'settings.shell.searchEmpty': 'Keine Einstellungen fuer "{query}" gefunden',
    'settings.shell.searchPlaceholder': 'Einstellungen suchen...',
    'settings.shell.unsavedDescription':
      'Ihre Aenderungen gehen verloren, wenn Sie diese Seite verlassen.',
    'settings.shell.unsavedTitle': 'Nicht gespeicherte Aenderungen',
  },
  es: {
    ...EN_MESSAGES,
    'settings.header.api.description':
      'Genera y administra tokens de Pulse con alcance limitado para agentes, automatización e integraciones externas.',
    'settings.header.api.title': 'Acceso API',
    'settings.header.infrastructure.description':
      'Agrega, descubre y verifica la infraestructura que Pulse supervisa.',
    'settings.header.infrastructure.readOnlyDescription':
      'Revisa los sistemas principales supervisados y el estado de informes. Los cambios de configuración no están disponibles en esta sesión de solo lectura.',
    'settings.header.infrastructure.title': 'Infraestructura',
    'settings.header.monitoringAvailability.description':
      'Supervisa dispositivos y servicios solo por endpoint con pruebas ping, TCP y HTTP.',
    'settings.header.monitoringAvailability.title': 'Comprobaciones de disponibilidad',
    'settings.header.organizationAccess.description':
      'Administra invitaciones, roles de miembros y transferencias de propiedad de la organización.',
    'settings.header.organizationAccess.title': 'Acceso de organización',
    'settings.header.organizationBilling.description':
      'Revisa el plan de la organización, las políticas de uso aplicables y el estado de suscripción para acceso de pago.',
    'settings.header.organizationBilling.title': 'Facturación y uso',
    'settings.header.organizationBillingAdmin.description':
      'Revisa y administra el estado de facturación de todos los tenants (solo modo alojado).',
    'settings.header.organizationBillingAdmin.title': 'Admin de facturación',
    'settings.header.organizationOverview.description':
      'Revisa metadatos, alcance de miembros y propiedad de la organización.',
    'settings.header.organizationOverview.title': 'Resumen de organización',
    'settings.header.organizationSharing.description':
      'Comparte recursos entre organizaciones con acceso delimitado.',
    'settings.header.organizationSharing.title': 'Uso compartido de organización',
    'settings.header.securityAudit.description':
      'Consulta eventos de seguridad, intentos de inicio de sesión y cambios de configuración.',
    'settings.header.securityAudit.title': 'Registro de auditoría',
    'settings.header.securityAuth.description':
      'Administra autenticación con contraseña y rotación de credenciales.',
    'settings.header.securityAuth.title': 'Autenticación',
    'settings.header.securityDataHandling.description':
      'Consulta qué detalles de recursos supervisados se pueden resumir, deben quedarse locales o están redactados.',
    'settings.header.securityDataHandling.title': 'Privacidad de recursos',
    'settings.header.securityOverview.description':
      'Consulta tu postura de seguridad de un vistazo y supervisa el estado de autenticación.',
    'settings.header.securityOverview.title': 'Resumen de seguridad',
    'settings.header.securityRoles.description':
      'Define roles personalizados y administra permisos granulares para usuarios y tokens.',
    'settings.header.securityRoles.title': 'Roles',
    'settings.header.securitySso.description': 'Configura proveedores de identidad OIDC y SAML.',
    'settings.header.securitySso.title': 'Proveedores de inicio de sesión único',
    'settings.header.securityUsers.description':
      'Asigna roles a usuarios y consulta permisos efectivos en tu infraestructura.',
    'settings.header.securityUsers.title': 'Acceso de usuarios',
    'settings.header.securityWebhooks.description':
      'Configura la entrega en tiempo real de eventos de auditoría a sistemas externos.',
    'settings.header.securityWebhooks.title': 'Webhooks de auditoría',
    'settings.header.systemAi.description':
      'Configura proveedores y modelos para Pulse Assistant y Patrol.',
    'settings.header.systemAi.title': 'Assistant y Patrol',
    'settings.header.systemBilling.description':
      'Revisa el plan que usa esta instancia y las capacidades opcionales conectadas.',
    'settings.header.systemBilling.title': 'Plan autohospedado',
    'settings.header.systemGeneral.description':
      'Administra apariencia, diseño y cadencia predeterminada de supervisión.',
    'settings.header.systemGeneral.title': 'General',
    'settings.header.systemNetwork.description':
      'Configura la URL pública, CORS, inserción y límites de red para webhooks.',
    'settings.header.systemNetwork.title': 'Red',
    'settings.header.systemRecovery.description':
      'Administra sondeos de copias/snapshots y flujos de exportación e importación de configuración.',
    'settings.header.systemRecovery.title': 'Recuperación',
    'settings.header.systemRelay.description':
      'Configura la conectividad de relay de Pulse para acceso remoto seguro y vinculación con Pulse Mobile.',
    'settings.header.systemRelay.title': 'Acceso remoto',
    'settings.header.systemUpdates.description':
      'Administra comprobaciones de versión, canales de actualización y comportamiento de actualización automática.',
    'settings.header.systemUpdates.title': 'Actualizaciones',
    'settings.header.supportDiagnostics.description':
      'Ejecuta comprobaciones de salud, valida conectividad y exporta snapshots de resolución de problemas.',
    'settings.header.supportDiagnostics.title': 'Diagnóstico y salud',
    'settings.header.supportLogs.description':
      'Inspecciona el flujo de logs de Pulse en vivo y descarga el búfer capturado para soporte.',
    'settings.header.supportLogs.title': 'Logs del sistema',
    'settings.header.supportReporting.description':
      'Exporta datos de inventario y genera informes de rendimiento desde la consola canónica de ajustes.',
    'settings.header.supportReporting.title': 'Datos e informes',
    'settings.nav.group.infrastructure': 'Infraestructura',
    'settings.nav.group.monitoring': 'Supervisión',
    'settings.nav.group.organization': 'Organización',
    'settings.nav.group.security': 'Seguridad',
    'settings.nav.group.support': 'Soporte',
    'settings.nav.group.system': 'Sistema',
    'settings.nav.item.apiAccess': 'Acceso API',
    'settings.nav.item.auditLog': 'Registro de auditoría',
    'settings.nav.item.auditWebhooks': 'Webhooks de auditoría',
    'settings.nav.item.authentication': 'Autenticación',
    'settings.nav.item.availabilityChecks': 'Comprobaciones de disponibilidad',
    'settings.nav.item.billing': 'Facturación',
    'settings.nav.item.billingAdmin': 'Admin de facturación',
    'settings.nav.item.dataReports': 'Datos e informes',
    'settings.nav.item.diagnosticsHealth': 'Diagnóstico y salud',
    'settings.nav.item.general': 'General',
    'settings.nav.item.infrastructure': 'Infraestructura',
    'settings.nav.item.network': 'Red',
    'settings.nav.item.organizationAccess': 'Acceso',
    'settings.nav.item.organizationOverview': 'Resumen',
    'settings.nav.item.plans': 'Planes',
    'settings.nav.item.recovery': 'Recuperación',
    'settings.nav.item.remoteAccess': 'Acceso remoto',
    'settings.nav.item.resourcePrivacy': 'Privacidad de recursos',
    'settings.nav.item.roles': 'Roles',
    'settings.nav.item.securityOverview': 'Resumen de seguridad',
    'settings.nav.item.sharing': 'Uso compartido',
    'settings.nav.item.singleSignOn': 'Inicio de sesión único',
    'settings.nav.item.systemLogs': 'Logs del sistema',
    'settings.nav.item.updates': 'Actualizaciones',
    'settings.nav.item.users': 'Usuarios',
    'settings.nav.item.assistantPatrol': 'Assistant y Patrol',
    'settings.shell.collapseSidebarLabel': 'Contraer navegación de ajustes',
    'settings.shell.configurationLoading': 'Cargando configuración...',
    'settings.shell.discardLabel': 'Descartar',
    'settings.shell.expandSidebarLabel': 'Expandir navegación de ajustes',
    'settings.shell.loading': 'Cargando ajustes...',
    'settings.shell.mobileBackLabel': 'Ajustes',
    'settings.shell.navigationAriaLabel': 'Navegación de ajustes',
    'settings.shell.navigationTitle': 'Ajustes',
    'settings.shell.saveChangesLabel': 'Guardar cambios',
    'settings.shell.searchEmpty': 'No se encontraron ajustes para "{query}"',
    'settings.shell.searchPlaceholder': 'Buscar ajustes...',
    'settings.shell.unsavedDescription': 'Tus cambios se perderán si sales.',
    'settings.shell.unsavedTitle': 'Cambios sin guardar',
  },
};
