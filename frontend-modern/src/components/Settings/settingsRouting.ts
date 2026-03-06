export type SettingsTab =
  | 'proxmox'
  | 'agents'
  | 'system-general'
  | 'system-network'
  | 'system-updates'
  | 'system-recovery'
  | 'system-ai'
  | 'system-relay'
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
  | 'security-webhooks';

export type AgentKey = 'pve' | 'pbs' | 'pmg';

// Default landing tab for /settings when no deep-link tab is provided.
export const DEFAULT_SETTINGS_TAB: SettingsTab = 'agents';
const LEGACY_INFRASTRUCTURE_PREFIX = '/settings/infrastructure';
const LEGACY_AGENTS_PREFIX = '/settings/workloads';
const LEGACY_DOCKER_PREFIX = '/settings/workloads/docker';
const PROXMOX_PREFIX = '/settings/infrastructure/proxmox';
const LEGACY_PROXMOX_API_PREFIX = '/settings/infrastructure/api';
const LEGACY_INTEGRATIONS_API_PREFIX = '/settings/integrations/api';
const SECURITY_API_PREFIX = '/settings/security/api';

const normalizeSettingsPath = (path: string): string => {
  const trimmed = (path || '').trim();
  if (!trimmed) return '/settings';
  if (trimmed.length > 1 && trimmed.endsWith('/')) {
    return trimmed.replace(/\/+$/, '');
  }
  return trimmed;
};

export function resolveCanonicalSettingsPath(path: string): string | null {
  const normalizedPath = normalizeSettingsPath(path);
  if (!normalizedPath.startsWith('/settings')) return null;
  if (normalizedPath === LEGACY_AGENTS_PREFIX) {
    return settingsTabPath(DEFAULT_SETTINGS_TAB);
  }
  if (normalizedPath === LEGACY_INFRASTRUCTURE_PREFIX) {
    return settingsTabPath(DEFAULT_SETTINGS_TAB);
  }
  if (normalizedPath === LEGACY_DOCKER_PREFIX) {
    return settingsTabPath(DEFAULT_SETTINGS_TAB);
  }
  if (normalizedPath === LEGACY_PROXMOX_API_PREFIX) {
    return PROXMOX_PREFIX;
  }
  if (normalizedPath === `${LEGACY_INFRASTRUCTURE_PREFIX}/pve`) {
    return `${PROXMOX_PREFIX}/pve`;
  }
  if (normalizedPath === `${LEGACY_INFRASTRUCTURE_PREFIX}/pbs`) {
    return `${PROXMOX_PREFIX}/pbs`;
  }
  if (normalizedPath === `${LEGACY_INFRASTRUCTURE_PREFIX}/pmg`) {
    return `${PROXMOX_PREFIX}/pmg`;
  }
  if (normalizedPath === `${LEGACY_PROXMOX_API_PREFIX}/pve`) {
    return `${PROXMOX_PREFIX}/pve`;
  }
  if (normalizedPath === `${LEGACY_PROXMOX_API_PREFIX}/pbs`) {
    return `${PROXMOX_PREFIX}/pbs`;
  }
  if (normalizedPath === `${LEGACY_PROXMOX_API_PREFIX}/pmg`) {
    return `${PROXMOX_PREFIX}/pmg`;
  }
  if (normalizedPath === LEGACY_INTEGRATIONS_API_PREFIX) {
    return SECURITY_API_PREFIX;
  }
  return normalizedPath;
}

export function deriveTabFromPath(path: string): SettingsTab {
  const canonicalPath = resolveCanonicalSettingsPath(path) ?? normalizeSettingsPath(path);

  if (canonicalPath === '/settings') return 'agents';
  if (canonicalPath.includes(PROXMOX_PREFIX) || canonicalPath.includes(LEGACY_PROXMOX_API_PREFIX))
    return 'agents';

  if (canonicalPath.includes('/settings/system-general')) return 'system-general';
  if (canonicalPath.includes('/settings/system-network')) return 'system-network';
  if (canonicalPath.includes('/settings/system-updates')) return 'system-updates';
  if (canonicalPath.includes('/settings/system-recovery')) return 'system-recovery';
  if (canonicalPath.includes('/settings/system-ai')) return 'system-ai';
  if (canonicalPath.includes('/settings/system-relay')) return 'system-relay';
  if (canonicalPath.includes('/settings/system-pro')) return 'system-pro';
  if (canonicalPath.includes('/settings/organization/access')) return 'organization-access';
  if (canonicalPath.includes('/settings/organization/sharing')) return 'organization-sharing';
  if (canonicalPath.includes('/settings/organization/billing-admin'))
    return 'organization-billing-admin';
  if (canonicalPath.includes('/settings/organization/billing')) return 'organization-billing';
  if (canonicalPath.includes('/settings/organization')) return 'organization-overview';

  if (canonicalPath.includes(SECURITY_API_PREFIX)) return 'api';
  if (canonicalPath.includes(LEGACY_INTEGRATIONS_API_PREFIX)) return 'api';

  if (canonicalPath.includes('/settings/security-overview')) return 'security-overview';
  if (canonicalPath.includes('/settings/security-auth')) return 'security-auth';
  if (canonicalPath.includes('/settings/security-sso')) return 'security-sso';
  if (canonicalPath.includes('/settings/security-roles')) return 'security-roles';
  if (canonicalPath.includes('/settings/security-users')) return 'security-users';
  if (canonicalPath.includes('/settings/security-audit')) return 'security-audit';
  if (canonicalPath.includes('/settings/security-webhooks')) return 'security-webhooks';

  return DEFAULT_SETTINGS_TAB;
}

export function deriveAgentFromPath(path: string): AgentKey | null {
  const canonicalPath = resolveCanonicalSettingsPath(path) ?? normalizeSettingsPath(path);

  if (canonicalPath.includes(`${PROXMOX_PREFIX}/pve`)) return 'pve';
  if (canonicalPath.includes(`${PROXMOX_PREFIX}/pbs`)) return 'pbs';
  if (canonicalPath.includes(`${PROXMOX_PREFIX}/pmg`)) return 'pmg';
  return null;
}

export function deriveTabFromQuery(search: string): SettingsTab | null {
  const params = new URLSearchParams(search);
  const tab = params.get('tab')?.trim().toLowerCase();
  if (!tab) return null;

  switch (tab) {
    case 'infrastructure':
    case 'workloads':
    case 'agents':
      return 'agents';
    case 'proxmox':
      return 'proxmox';
    case 'system-recovery':
      return 'system-recovery';
    case 'system-updates':
      return 'system-updates';
    case 'system-network':
      return 'system-network';
    case 'system-general':
      return 'system-general';
    case 'system-ai':
      return 'system-ai';
    case 'system-relay':
      return 'system-relay';
    case 'system-pro':
      return 'system-pro';
    case 'api':
      return 'api';
    case 'docker':
      return 'agents';
    case 'organization-overview':
      return 'organization-overview';
    case 'organization-access':
      return 'organization-access';
    case 'organization-sharing':
      return 'organization-sharing';
    case 'organization-billing':
      return 'organization-billing';
    case 'organization-billing-admin':
      return 'organization-billing-admin';
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
    default:
      return null;
  }
}

export function settingsTabPath(tab: SettingsTab): string {
  switch (tab) {
    case 'proxmox':
      return PROXMOX_PREFIX;
    case 'agents':
      return '/settings';
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
      return SECURITY_API_PREFIX;
    case 'system-relay':
      return '/settings/system-relay';
    default:
      return `/settings/${tab}`;
  }
}
