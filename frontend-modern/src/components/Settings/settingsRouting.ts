export type SettingsTab =
  | 'proxmox'
  | 'docker'
  | 'agents'
  | 'system-general'
  | 'system-network'
  | 'system-updates'
  | 'system-recovery'
  | 'system-ai'
  | 'system-relay'
  | 'system-logs'
  | 'system-pro'
  | 'organization-overview'
  | 'organization-access'
  | 'organization-billing'
  | 'organization-billing-admin'
  | 'organization-sharing'
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
  '/settings/system-logs': '/settings/operations/logs',
  '/settings/diagnostics': '/settings/operations/diagnostics',
  '/settings/reporting': '/settings/operations/reporting',
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

  if (canonicalPath.includes('/settings/system-general')) return 'system-general';
  if (canonicalPath.includes('/settings/system-network')) return 'system-network';
  if (canonicalPath.includes('/settings/system-updates')) return 'system-updates';
  if (canonicalPath.includes('/settings/backups')) return 'system-recovery';
  if (canonicalPath.includes('/settings/system-backups')) return 'system-recovery';
  if (canonicalPath.includes('/settings/recovery')) return 'system-recovery';
  if (canonicalPath.includes('/settings/system-recovery')) return 'system-recovery';
  if (canonicalPath.includes('/settings/system-ai')) return 'system-ai';
  if (canonicalPath.includes('/settings/integrations/relay')) return 'system-relay';
  if (canonicalPath.includes('/settings/system-relay')) return 'system-relay';
  if (canonicalPath.includes('/settings/system-pro')) return 'system-pro';
  if (canonicalPath.includes('/settings/organization/access')) return 'organization-access';
  if (canonicalPath.includes('/settings/organization/sharing')) return 'organization-sharing';
  if (canonicalPath.includes('/settings/organization/billing-admin')) return 'organization-billing-admin';
  if (canonicalPath.includes('/settings/billing')) return 'organization-billing';
  if (canonicalPath.includes('/settings/plan')) return 'organization-billing';
  if (canonicalPath.includes('/settings/organization/billing')) return 'organization-billing';
  if (canonicalPath.includes('/settings/organization')) return 'organization-overview';
  if (canonicalPath.includes('/settings/operations/logs')) return 'system-logs';
  if (canonicalPath.includes('/settings/system-logs')) return 'system-logs';

  if (canonicalPath.includes('/settings/integrations/api')) return 'api';
  if (canonicalPath.includes('/settings/api')) return 'api';

  if (canonicalPath.includes('/settings/security-overview')) return 'security-overview';
  if (canonicalPath.includes('/settings/security-auth')) return 'security-auth';
  if (canonicalPath.includes('/settings/security-sso')) return 'security-sso';
  if (canonicalPath.includes('/settings/security-roles')) return 'security-roles';
  if (canonicalPath.includes('/settings/security-users')) return 'security-users';
  if (canonicalPath.includes('/settings/security-audit')) return 'security-audit';
  if (canonicalPath.includes('/settings/security-webhooks')) return 'security-webhooks';
  if (canonicalPath.includes('/settings/security')) return 'security-overview';

  if (canonicalPath.includes('/settings/operations/updates')) return 'system-updates';
  if (canonicalPath.includes('/settings/updates')) return 'system-updates';
  if (canonicalPath.includes('/settings/operations/diagnostics')) return 'diagnostics';
  if (canonicalPath.includes('/settings/diagnostics')) return 'diagnostics';
  if (canonicalPath.includes('/settings/operations/reporting')) return 'reporting';
  if (canonicalPath.includes('/settings/reporting')) return 'reporting';

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
      return 'system-recovery';
    case 'recovery':
      return 'system-recovery';
    case 'updates':
      return 'system-updates';
    case 'network':
      return 'system-network';
    case 'general':
      return 'system-general';
    case 'api':
      return 'api';
    case 'organization':
    case 'org':
      return 'organization-overview';
    case 'organization-access':
    case 'org-access':
      return 'organization-access';
    case 'organization-sharing':
    case 'sharing':
      return 'organization-sharing';
    case 'billing':
    case 'plan':
      return 'organization-billing';
    case 'billing-admin':
      return 'organization-billing-admin';
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
    case 'system-recovery':
      return '/settings/system-recovery';
    case 'organization-overview':
      return '/settings/organization';
    case 'organization-access':
      return '/settings/organization/access';
    case 'organization-sharing':
      return '/settings/organization/sharing';
    case 'organization-billing':
      return '/settings/organization/billing';
    case 'organization-billing-admin':
      return '/settings/organization/billing-admin';
    case 'api':
      return '/settings/integrations/api';
    case 'system-relay':
      return '/settings/system-relay';
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
