import type { SettingsHeaderMetaMap } from './settingsTypes';

export const SETTINGS_HEADER_META: SettingsHeaderMetaMap = {
  proxmox: {
    title: 'Infrastructure',
    description:
      'Manage infrastructure integrations for Proxmox VE, Backup Server, and Mail Gateway.',
  },
  docker: {
    title: 'Docker Workloads',
    description:
      'Configure Docker-specific workload controls and update behavior across monitored hosts.',
  },
  agents: {
    title: 'Unified Agents',
    description: 'Install and manage host, Docker, and Kubernetes agents from one workflow.',
  },
  'system-general': {
    title: 'General Settings',
    description: 'Configure appearance, layout, and default monitoring cadence.',
  },
  'system-network': {
    title: 'Network Settings',
    description: 'Configure discovery, CORS, embedding, and webhook network boundaries.',
  },
  'system-updates': {
    title: 'Updates',
    description: 'Manage version checks, update channels, and automatic update behavior.',
  },
  'system-backups': {
    title: 'Backups',
    description: 'Manage backup polling plus configuration export and import workflows.',
  },
  'system-ai': {
    title: 'AI Settings',
    description: 'Configure AI providers, model defaults, Pulse Assistant, and Patrol automation.',
  },
  'system-relay': {
    title: 'Remote Access',
    description: 'Configure Pulse relay connectivity for secure remote access.',
  },
  'system-pro': {
    title: 'Pro',
    description: 'Manage license activation and Pro feature access.',
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
    title: 'Organization Billing',
    description: 'Track plan tier, usage, and upgrade options for multi-tenant deployments.',
  },
  'organization-billing-admin': {
    title: 'Billing Admin',
    description: 'Review and manage tenant billing state across all organizations (hosted mode only).',
  },
  api: {
    title: 'API Access',
    description:
      'Generate and manage scoped tokens for agents, automation, and integrations.',
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
    title: 'Single Sign-On',
    description: 'Configure OIDC providers for team authentication.',
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
  diagnostics: {
    title: 'Diagnostics',
    description:
      'Inspect discovery scans, connection health, and runtime metrics for troubleshooting.',
  },
  reporting: {
    title: 'Reporting',
    description: 'Generate and export infrastructure reports in PDF and CSV formats.',
  },
  'system-logs': {
    title: 'System Logs',
    description: 'View real-time system logs and download support bundles.',
  },
};
