export type SettingsTab =
  | 'proxmox'
  | 'docker'
  | 'agents'
  | 'workspace' // Formerly system-general
  | 'integrations' // Formerly system-network
  | 'maintenance' // Formerly system-updates/system-backups
  | 'authentication' // Formerly security-auth/sso/overview
  | 'team' // Formerly security-users/roles
  | 'audit'; // Formerly security-audit/webhooks

export type AgentKey = 'pve' | 'pbs' | 'pmg';

// Default landing tab for /settings when no deep-link tab is provided.
export const DEFAULT_SETTINGS_TAB: SettingsTab = 'agents';

const normalizeSettingsPath = (path: string): string => {
  const trimmed = (path || '').trim();
  if (!trimmed) return '/settings';
  if (trimmed.length > 1 && trimmed.endsWith('/')) {
    return trimmed.replace(/\/+$/, '');
  }
  return trimmed;
};

const SETTINGS_PATH_ALIASES: Record<string, string> = {
  '/settings/proxmox': '/settings/infrastructure',
  '/settings/agent-hub': '/settings/infrastructure',
  '/settings/servers': '/settings/infrastructure',
  '/settings/pve': '/settings/infrastructure/pve',
  '/settings/pbs': '/settings/infrastructure/pbs',
  '/settings/pmg': '/settings/infrastructure/pmg',
  '/settings/storage': '/settings/infrastructure/pbs',
  '/settings/docker': '/settings/workloads/docker',
  '/settings/containers': '/settings/workloads/docker',
  '/settings/hosts': '/settings/workloads',
  '/settings/host-agents': '/settings/workloads',
  '/settings/linuxServers': '/settings/workloads',
  '/settings/windowsServers': '/settings/workloads',
  '/settings/macServers': '/settings/workloads',
  '/settings/agents': '/settings/workloads',
  '/settings/backups': '/settings/system-recovery',
  '/settings/recovery': '/settings/system-recovery',
  '/settings/system-backups': '/settings/system-recovery',
  '/settings/updates': '/settings/system-updates',
  '/settings/operations/updates': '/settings/system-updates',
  '/settings/integrations/relay': '/settings/system-relay',
  '/settings/security': '/settings/security-overview',
  '/settings/api': '/settings/integrations/api',
  '/settings/billing': '/settings/organization/billing',
  '/settings/plan': '/settings/organization/billing',
};

export function resolveCanonicalSettingsPath(path: string): string | null {
  const normalizedPath = normalizeSettingsPath(path);
  if (!normalizedPath.startsWith('/settings')) return null;
  return SETTINGS_PATH_ALIASES[normalizedPath] ?? normalizedPath;
}

export function deriveTabFromPath(path: string): SettingsTab {
  const canonicalPath = resolveCanonicalSettingsPath(path) ?? normalizeSettingsPath(path);

  if (canonicalPath.includes('/settings/workloads/docker')) return 'docker';
  if (canonicalPath.includes('/settings/infrastructure')) return 'proxmox';
  if (canonicalPath.includes('/settings/storage')) return 'proxmox';
  if (canonicalPath.includes('/settings/workloads')) return 'agents';

  if (canonicalPath.includes('/settings/proxmox')) return 'proxmox';
  if (canonicalPath.includes('/settings/agent-hub')) return 'proxmox';
  if (canonicalPath.includes('/settings/docker')) return 'docker';

  if (
    canonicalPath.includes('/settings/hosts') ||
    canonicalPath.includes('/settings/host-agents') ||
    canonicalPath.includes('/settings/servers') ||
    canonicalPath.includes('/settings/linuxServers') ||
    canonicalPath.includes('/settings/windowsServers') ||
    canonicalPath.includes('/settings/macServers') ||
    canonicalPath.includes('/settings/agents')
  ) {
    return 'agents';
  }

  if (canonicalPath.includes('/settings/workspace')) return 'workspace';
  if (canonicalPath.includes('/settings/system-general')) return 'workspace';
  if (canonicalPath.includes('/settings/system-ai')) return 'workspace';
  if (canonicalPath.includes('/settings/system-pro')) return 'workspace';
  
  if (canonicalPath.includes('/settings/integrations')) return 'integrations';
  if (canonicalPath.includes('/settings/system-network')) return 'integrations';
  if (canonicalPath.includes('/settings/api')) return 'integrations';
  if (canonicalPath.includes('/settings/integrations/api')) return 'integrations';
  
  if (canonicalPath.includes('/settings/maintenance')) return 'maintenance';
  if (canonicalPath.includes('/settings/system-updates')) return 'maintenance';
  if (canonicalPath.includes('/settings/updates')) return 'maintenance';
  if (canonicalPath.includes('/settings/operations/updates')) return 'maintenance';
  if (canonicalPath.includes('/settings/system-recovery')) return 'maintenance';
  if (canonicalPath.includes('/settings/backups')) return 'maintenance';
  if (canonicalPath.includes('/settings/recovery')) return 'maintenance';

  if (canonicalPath.includes('/settings/authentication')) return 'authentication';
  if (canonicalPath.includes('/settings/security-overview')) return 'authentication';
  if (canonicalPath.includes('/settings/security-auth')) return 'authentication';
  if (canonicalPath.includes('/settings/security-sso')) return 'authentication';
  if (canonicalPath.includes('/settings/security')) return 'authentication';

  if (canonicalPath.includes('/settings/team')) return 'team';
  if (canonicalPath.includes('/settings/security-roles')) return 'team';
  if (canonicalPath.includes('/settings/security-users')) return 'team';
  if (canonicalPath.includes('/settings/organization')) return 'team';
  if (canonicalPath.includes('/settings/organization/access')) return 'team';

  if (canonicalPath.includes('/settings/audit')) return 'audit';
  if (canonicalPath.includes('/settings/security-audit')) return 'audit';
  if (canonicalPath.includes('/settings/security-webhooks')) return 'audit';

  // Legacy platform paths map to Proxmox connections.
  if (
    canonicalPath.includes('/settings/pve') ||
    canonicalPath.includes('/settings/pbs') ||
    canonicalPath.includes('/settings/pmg') ||
    canonicalPath.includes('/settings/containers') ||
    canonicalPath.includes('/settings/linuxServers') ||
    canonicalPath.includes('/settings/windowsServers') ||
    canonicalPath.includes('/settings/macServers')
  ) {
    return 'proxmox';
  }

  return 'proxmox';
}

export function deriveAgentFromPath(path: string): AgentKey | null {
  const canonicalPath = resolveCanonicalSettingsPath(path) ?? normalizeSettingsPath(path);

  if (canonicalPath.includes('/settings/infrastructure/pve')) return 'pve';
  if (canonicalPath.includes('/settings/infrastructure/pbs')) return 'pbs';
  if (canonicalPath.includes('/settings/infrastructure/pmg')) return 'pmg';

  if (canonicalPath.includes('/settings/pve')) return 'pve';
  if (canonicalPath.includes('/settings/pbs')) return 'pbs';
  if (canonicalPath.includes('/settings/pmg')) return 'pmg';

  if (canonicalPath.includes('/settings/storage')) return 'pbs';
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
      return 'maintenance';
    case 'recovery':
      return 'maintenance';
    case 'updates':
    case 'maintenance':
      return 'maintenance';
    case 'network':
    case 'integrations':
      return 'integrations';
    case 'general':
    case 'workspace':
      return 'workspace';
    
    case 'organization':
    case 'org':
      return 'team';
    case 'team':
    case 'org-access':
      return 'team';
    case 'team':
    case 'sharing':
      return 'team';
    case 'billing':
    case 'plan':
      return 'team';
    case 'billing-admin':
      return 'team';
    case 'security':
    case 'security-overview':
      return 'authentication';
    case 'security-auth':
    case 'authentication':
      return 'authentication';
    case 'security-sso':
      return 'authentication';
    case 'security-roles':
      return 'team';
    case 'security-users':
    case 'team':
      return 'team';
    case 'security-audit':
    case 'audit':
      return 'audit';
    case 'security-webhooks':
      return 'audit';
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
    case 'maintenance':
      return '/settings/system-recovery';
    case 'team':
      return '/settings/organization';
    case 'team':
      return '/settings/organization/access';
    case 'team':
      return '/settings/organization/sharing';
    case 'team':
      return '/settings/organization/billing';
    case 'team':
      return '/settings/organization/billing-admin';
    case 'integrations':
      return '/settings/integrations/api';
    case 'integrations':
      return '/settings/system-relay';
    default:
      return `/settings/${tab}`;
  }
}
