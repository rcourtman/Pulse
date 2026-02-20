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
  'workspace': {
    title: 'Workspace Settings',
    description: 'Configure appearance preferences, AI services, and Pulse Pro licensing.',
  },
  'integrations': {
    title: 'Network & Integrations',
    description: 'Manage network discovery, endpoints, API tokens, and webhook integrations.',
  },
  'maintenance': {
    title: 'System Maintenance',
    description: 'Check for updates, manage update channels, and configure backup polling.',
  },
  'authentication': {
    title: 'Authentication',
    description: 'View security posture and configure password or Single Sign-On authentication.',
  },
  'team': {
    title: 'Team & Roles',
    description: 'Manage users, assign functional roles, and view effective permissions across your infrastructure.',
  },
  'audit': {
    title: 'Audit Logs',
    description: 'View security events, login attempts, and configuration changes.',
  },
};
