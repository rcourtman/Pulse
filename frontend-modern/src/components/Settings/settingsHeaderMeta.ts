import type { SettingsHeaderMetaMap } from './settingsTypes';

export const SETTINGS_HEADER_META: SettingsHeaderMetaMap = {
  proxmox: {
    title: 'Proxmox',
    description:
      'Add and manage Proxmox VE, Backup Server, and Mail Gateway connections when the unified agent is not available on the host.',
  },
  agents: {
    title: 'Infrastructure',
    description:
      'Install and manage unified agents, runtime behavior, and the recommended path for bringing infrastructure and workloads into Pulse.',
  },
  'system-general': {
    title: 'General',
    description: 'Manage appearance, layout, and default monitoring cadence.',
  },
  'system-network': {
    title: 'Network Settings',
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
    title: 'AI Services',
    description: 'Configure AI providers, models, Pulse Assistant, and Patrol.',
  },
  'system-relay': {
    title: 'Remote Access',
    description:
      'Configure Pulse relay connectivity for secure remote access (mobile rollout coming soon).',
  },
  'system-pro': {
    title: 'Pulse Pro',
    description: 'Activate your Pro license to unlock auto-fix, alert-triggered AI, and advanced features.',
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
    title: 'Billing & Plan',
    description: 'Review your current plan tier, usage against limits, and available upgrade paths.',
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
