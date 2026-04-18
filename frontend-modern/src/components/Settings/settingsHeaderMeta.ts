import type { SettingsHeaderMetaMap } from './settingsNavigationModel';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
import { RELAY_SETTINGS_DESCRIPTION } from '@/utils/relayPresentation';
import {
  AI_SETTINGS_PANEL_DESCRIPTION,
  AI_SETTINGS_PANEL_TITLE,
} from '@/utils/aiSettingsPresentation';

export const SETTINGS_HEADER_META: SettingsHeaderMetaMap = {
  'infrastructure-systems': {
    title: 'Infrastructure',
    description:
      `Review monitored systems in one ledger, then use Add infrastructure when you need platform setup or agent install commands. ${SELF_HOSTED_PRO_BILLING_PRESENTATION.infrastructureRouteReferral}`,
  },
  'infrastructure-connections': {
    title: 'Infrastructure',
    description:
      `Review monitored systems in one ledger, then use Add infrastructure when you need platform setup or agent install commands. ${SELF_HOSTED_PRO_BILLING_PRESENTATION.infrastructureRouteReferral}`,
  },
  'infrastructure-install': {
    title: 'Infrastructure',
    description:
      `Review monitored systems in one ledger, then use Add infrastructure when you need platform setup or agent install commands. ${SELF_HOSTED_PRO_BILLING_PRESENTATION.infrastructureRouteReferral}`,
  },
  'system-general': {
    title: 'General',
    description: 'Manage appearance, layout, and default monitoring cadence.',
  },
  'system-network': {
    title: 'Network',
    description: 'Configure discovery, CORS, embedding, and webhook network boundaries.',
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
    description: 'Export inventory data and generate performance reports from the canonical settings shell.',
  },
  'support-logs': {
    title: 'System Logs',
    description: 'Inspect the live Pulse log stream and download the captured buffer for support work.',
  },
  'organization-overview': {
    title: 'Organization Overview',
    description: 'Review organization metadata, membership footprint, and ownership.',
  },
  'organization-access': {
    title: 'Organization Access',
    description: 'Manage organization members, roles, and ownership transfers.',
  },
  'organization-sharing': {
    title: 'Organization Sharing',
    description: 'Share resources between organizations with scoped access.',
  },
  'organization-billing': {
    title: 'Billing & Usage',
    description:
      'Review your organization plan, usage against plan limits, and subscription status for paid access.',
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
