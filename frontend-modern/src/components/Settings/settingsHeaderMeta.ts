import { t, type I18nMessageKey, type SupportedLocale } from '@/i18n';
import type { SettingsHeaderMetaMap, SettingsTab } from './settingsNavigationModel';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
import { RELAY_SETTINGS_DESCRIPTION } from '@/utils/relayPresentation';
import {
  AI_SETTINGS_PANEL_DESCRIPTION,
  AI_SETTINGS_PANEL_TITLE,
} from '@/utils/aiSettingsPresentation';

export const SETTINGS_HEADER_META: SettingsHeaderMetaMap = {
  'infrastructure-systems': {
    title: 'Infrastructure',
    description: 'Add, discover, and verify the infrastructure Pulse monitors.',
  },
  'monitoring-availability': {
    title: 'Availability checks',
    description: 'Monitor endpoint-only devices and services with ping, TCP, and HTTP probes.',
  },
  'system-general': {
    title: 'General',
    description: 'Manage appearance, layout, and default monitoring cadence.',
  },
  'system-network': {
    title: 'Network',
    description: 'Configure the public URL, CORS, embedding, and webhook network boundaries.',
  },
  'system-updates': {
    title: 'Updates',
    description: 'Manage version checks, update channels, and automatic update behavior.',
  },
  'system-recovery': {
    title: 'Recovery',
    description: 'Manage backup/snapshot polling plus configuration export and import workflows.',
  },
  'system-ai': {
    title: AI_SETTINGS_PANEL_TITLE,
    description: AI_SETTINGS_PANEL_DESCRIPTION,
  },
  'system-ai-patrol': {
    title: 'Patrol',
    description: 'Set when Patrol runs, what starts it, and which model it uses.',
  },
  'system-ai-assistant': {
    title: 'Assistant',
    description: 'Configure Assistant chat behavior, chat action permissions, and sessions.',
  },
  'system-ai-discovery': {
    title: 'Service Context',
    description:
      'Configure the model-backed service context Assistant and Patrol use. Infrastructure discovery and onboarding stay under Infrastructure.',
  },
  'system-relay': {
    title: 'Remote Access',
    description: RELAY_SETTINGS_DESCRIPTION,
  },
  'system-billing': {
    title: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellTitle,
    description: SELF_HOSTED_PRO_BILLING_PRESENTATION.shellDescription,
  },
  'support-diagnostics': {
    title: 'Diagnostics & Health',
    description: 'Run health checks, validate connectivity, and export troubleshooting snapshots.',
  },
  'support-reporting': {
    title: 'Data & Reports',
    description:
      'Export inventory data and generate performance reports from the canonical settings shell.',
  },
  'support-logs': {
    title: 'System Logs',
    description:
      'Inspect the live Pulse log stream and download the captured buffer for support work.',
  },
  'organization-overview': {
    title: 'Organization Overview',
    description: 'Review organization metadata, membership footprint, and ownership.',
  },
  'organization-access': {
    title: 'Organization Access',
    description: 'Manage organization invitations, member roles, and ownership transfers.',
  },
  'organization-sharing': {
    title: 'Organization Sharing',
    description: 'Share resources between organizations with scoped access.',
  },
  'organization-billing': {
    title: 'Billing & Usage',
    description:
      'Review your organization plan, applicable usage policies, and subscription status for paid access.',
  },
  'organization-billing-admin': {
    title: 'Billing Admin',
    description:
      'Review and manage tenant billing state across all organizations (hosted mode only).',
  },
  api: {
    title: 'API Access',
    description:
      'Generate and manage scoped Pulse tokens for agents, automation, and external integrations.',
  },
  'security-overview': {
    title: 'Security Overview',
    description: 'View your security posture at a glance and monitor authentication status.',
  },
  'security-data-handling': {
    title: 'Resource Privacy',
    description:
      'See which monitored resource details can be summarized, must stay local, or are redacted.',
  },
  'security-auth': {
    title: 'Authentication',
    description: 'Manage password-based authentication and credential rotation.',
  },
  'security-sso': {
    title: 'Single Sign-On Providers',
    description: 'Configure OIDC and SAML identity providers.',
  },
  'security-roles': {
    title: 'Roles',
    description: 'Define custom roles and manage granular permissions for users and tokens.',
  },
  'security-users': {
    title: 'User Access',
    description: 'Assign roles to users and view effective permissions across your infrastructure.',
  },
  'security-audit': {
    title: 'Audit Log',
    description: 'View security events, login attempts, and configuration changes.',
  },
  'security-webhooks': {
    title: 'Audit Webhooks',
    description: 'Configure real-time delivery of audit events to external systems.',
  },
};

const SETTINGS_HEADER_META_KEYS = {
  'infrastructure-systems': {
    title: 'settings.header.infrastructure.title',
    description: 'settings.header.infrastructure.description',
  },
  'monitoring-availability': {
    title: 'settings.header.monitoringAvailability.title',
    description: 'settings.header.monitoringAvailability.description',
  },
  'system-general': {
    title: 'settings.header.systemGeneral.title',
    description: 'settings.header.systemGeneral.description',
  },
  'system-network': {
    title: 'settings.header.systemNetwork.title',
    description: 'settings.header.systemNetwork.description',
  },
  'system-updates': {
    title: 'settings.header.systemUpdates.title',
    description: 'settings.header.systemUpdates.description',
  },
  'system-recovery': {
    title: 'settings.header.systemRecovery.title',
    description: 'settings.header.systemRecovery.description',
  },
  'system-ai': {
    title: 'settings.header.systemAi.title',
    description: 'settings.header.systemAi.description',
  },
  'system-ai-patrol': {
    title: 'settings.header.systemAiPatrol.title',
    description: 'settings.header.systemAiPatrol.description',
  },
  'system-ai-assistant': {
    title: 'settings.header.systemAiAssistant.title',
    description: 'settings.header.systemAiAssistant.description',
  },
  'system-ai-discovery': {
    title: 'settings.header.systemAiDiscovery.title',
    description: 'settings.header.systemAiDiscovery.description',
  },
  'system-relay': {
    title: 'settings.header.systemRelay.title',
    description: 'settings.header.systemRelay.description',
  },
  'system-billing': {
    title: 'settings.header.systemBilling.title',
    description: 'settings.header.systemBilling.description',
  },
  'support-diagnostics': {
    title: 'settings.header.supportDiagnostics.title',
    description: 'settings.header.supportDiagnostics.description',
  },
  'support-reporting': {
    title: 'settings.header.supportReporting.title',
    description: 'settings.header.supportReporting.description',
  },
  'support-logs': {
    title: 'settings.header.supportLogs.title',
    description: 'settings.header.supportLogs.description',
  },
  'organization-overview': {
    title: 'settings.header.organizationOverview.title',
    description: 'settings.header.organizationOverview.description',
  },
  'organization-access': {
    title: 'settings.header.organizationAccess.title',
    description: 'settings.header.organizationAccess.description',
  },
  'organization-sharing': {
    title: 'settings.header.organizationSharing.title',
    description: 'settings.header.organizationSharing.description',
  },
  'organization-billing': {
    title: 'settings.header.organizationBilling.title',
    description: 'settings.header.organizationBilling.description',
  },
  'organization-billing-admin': {
    title: 'settings.header.organizationBillingAdmin.title',
    description: 'settings.header.organizationBillingAdmin.description',
  },
  api: {
    title: 'settings.header.api.title',
    description: 'settings.header.api.description',
  },
  'security-overview': {
    title: 'settings.header.securityOverview.title',
    description: 'settings.header.securityOverview.description',
  },
  'security-data-handling': {
    title: 'settings.header.securityDataHandling.title',
    description: 'settings.header.securityDataHandling.description',
  },
  'security-auth': {
    title: 'settings.header.securityAuth.title',
    description: 'settings.header.securityAuth.description',
  },
  'security-sso': {
    title: 'settings.header.securitySso.title',
    description: 'settings.header.securitySso.description',
  },
  'security-roles': {
    title: 'settings.header.securityRoles.title',
    description: 'settings.header.securityRoles.description',
  },
  'security-users': {
    title: 'settings.header.securityUsers.title',
    description: 'settings.header.securityUsers.description',
  },
  'security-audit': {
    title: 'settings.header.securityAudit.title',
    description: 'settings.header.securityAudit.description',
  },
  'security-webhooks': {
    title: 'settings.header.securityWebhooks.title',
    description: 'settings.header.securityWebhooks.description',
  },
} as const satisfies Record<SettingsTab, { title: I18nMessageKey; description: I18nMessageKey }>;

export function getSettingsHeaderMeta(locale?: SupportedLocale): SettingsHeaderMetaMap {
  return Object.fromEntries(
    Object.entries(SETTINGS_HEADER_META_KEYS).map(([tab, keys]) => [
      tab,
      {
        title: t(keys.title, {}, locale),
        description: t(keys.description, {}, locale),
      },
    ]),
  ) as SettingsHeaderMetaMap;
}

export function getInfrastructureReadOnlyHeaderDescription(locale?: SupportedLocale): string {
  return t('settings.header.infrastructure.readOnlyDescription', {}, locale);
}
