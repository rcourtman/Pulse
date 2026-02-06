export type SettingsTab =
  | 'proxmox'
  | 'docker'
  | 'agents'
  | 'system-general'
  | 'system-network'
  | 'system-updates'
  | 'system-backups'
  | 'system-ai'
  | 'system-relay'
  | 'system-logs'
  | 'system-pro'
  | 'api'
  | 'security-overview'
  | 'security-auth'
  | 'security-sso'
  | 'security-roles'
  | 'security-users'
  | 'security-audit'
  | 'security-webhooks'
  | 'diagnostics'
  | 'reporting';

export type AgentKey = 'pve' | 'pbs' | 'pmg';

export function deriveTabFromPath(path: string): SettingsTab {
  if (path.includes('/settings/workloads/docker')) return 'docker';
  if (path.includes('/settings/infrastructure')) return 'proxmox';
  if (path.includes('/settings/storage')) return 'proxmox';
  if (path.includes('/settings/workloads')) return 'agents';

  if (path.includes('/settings/proxmox')) return 'proxmox';
  if (path.includes('/settings/agent-hub')) return 'proxmox';
  if (path.includes('/settings/docker')) return 'docker';

  if (
    path.includes('/settings/hosts') ||
    path.includes('/settings/host-agents') ||
    path.includes('/settings/servers') ||
    path.includes('/settings/linuxServers') ||
    path.includes('/settings/windowsServers') ||
    path.includes('/settings/macServers') ||
    path.includes('/settings/agents')
  ) {
    return 'agents';
  }

  if (path.includes('/settings/system-general')) return 'system-general';
  if (path.includes('/settings/system-network')) return 'system-network';
  if (path.includes('/settings/system-updates')) return 'system-updates';
  if (path.includes('/settings/backups')) return 'system-backups';
  if (path.includes('/settings/system-backups')) return 'system-backups';
  if (path.includes('/settings/system-ai')) return 'system-ai';
  if (path.includes('/settings/integrations/relay')) return 'system-relay';
  if (path.includes('/settings/system-relay')) return 'system-relay';
  if (path.includes('/settings/system-pro')) return 'system-pro';
  if (path.includes('/settings/operations/logs')) return 'system-logs';
  if (path.includes('/settings/system-logs')) return 'system-logs';

  if (path.includes('/settings/integrations/api')) return 'api';
  if (path.includes('/settings/api')) return 'api';

  if (path.includes('/settings/security-overview')) return 'security-overview';
  if (path.includes('/settings/security-auth')) return 'security-auth';
  if (path.includes('/settings/security-sso')) return 'security-sso';
  if (path.includes('/settings/security-roles')) return 'security-roles';
  if (path.includes('/settings/security-users')) return 'security-users';
  if (path.includes('/settings/security-audit')) return 'security-audit';
  if (path.includes('/settings/security-webhooks')) return 'security-webhooks';
  if (path.includes('/settings/security')) return 'security-overview';

  if (path.includes('/settings/operations/updates')) return 'system-updates';
  if (path.includes('/settings/updates')) return 'system-updates';
  if (path.includes('/settings/operations/diagnostics')) return 'diagnostics';
  if (path.includes('/settings/diagnostics')) return 'diagnostics';
  if (path.includes('/settings/operations/reporting')) return 'reporting';
  if (path.includes('/settings/reporting')) return 'reporting';

  // Legacy platform paths map to Proxmox connections.
  if (
    path.includes('/settings/pve') ||
    path.includes('/settings/pbs') ||
    path.includes('/settings/pmg') ||
    path.includes('/settings/containers') ||
    path.includes('/settings/linuxServers') ||
    path.includes('/settings/windowsServers') ||
    path.includes('/settings/macServers')
  ) {
    return 'proxmox';
  }

  return 'proxmox';
}

export function deriveAgentFromPath(path: string): AgentKey | null {
  if (path.includes('/settings/infrastructure/pve')) return 'pve';
  if (path.includes('/settings/infrastructure/pbs')) return 'pbs';
  if (path.includes('/settings/infrastructure/pmg')) return 'pmg';

  if (path.includes('/settings/pve')) return 'pve';
  if (path.includes('/settings/pbs')) return 'pbs';
  if (path.includes('/settings/pmg')) return 'pmg';

  if (path.includes('/settings/storage')) return 'pbs';
  return null;
}

export function deriveTabFromQuery(search: string): SettingsTab | null {
  const params = new URLSearchParams(search);
  const tab = params.get('tab')?.trim().toLowerCase();
  if (!tab) return null;

  switch (tab) {
    case 'infrastructure':
    case 'proxmox':
      return 'proxmox';
    case 'workloads':
    case 'agents':
      return 'agents';
    case 'docker':
      return 'docker';
    case 'backups':
      return 'system-backups';
    case 'updates':
      return 'system-updates';
    case 'network':
      return 'system-network';
    case 'general':
      return 'system-general';
    case 'api':
      return 'api';
    case 'security':
    case 'security-overview':
      return 'security-overview';
    case 'security-auth':
      return 'security-auth';
    case 'security-sso':
      return 'security-sso';
    case 'security-roles':
      return 'security-roles';
    case 'security-users':
      return 'security-users';
    case 'security-audit':
      return 'security-audit';
    case 'security-webhooks':
      return 'security-webhooks';
    case 'diagnostics':
      return 'diagnostics';
    case 'reporting':
      return 'reporting';
    default:
      return null;
  }
}

export function settingsTabPath(tab: SettingsTab): string {
  switch (tab) {
    case 'proxmox':
      return '/settings/infrastructure';
    case 'agents':
      return '/settings/workloads';
    case 'docker':
      return '/settings/workloads/docker';
    case 'system-backups':
      return '/settings/backups';
    case 'api':
      return '/settings/integrations/api';
    case 'system-relay':
      return '/settings/integrations/relay';
    case 'diagnostics':
      return '/settings/operations/diagnostics';
    case 'reporting':
      return '/settings/operations/reporting';
    case 'system-logs':
      return '/settings/operations/logs';
    default:
      return `/settings/${tab}`;
  }
}
