import type { Component } from 'solid-js';
import type { SecurityStatusSettingsCapabilities } from '@/types/config';
import { normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';
import type { PlatformType } from '@/types/resource';
import {
  deriveAddStepFromSearch,
  INFRASTRUCTURE_ADD_QUERY_PARAM,
  INFRASTRUCTURE_AGENT_DOCTOR_PATH,
} from './infrastructureWorkspaceModel';
import {
  AVAILABILITY_ADD_QUERY_PARAM,
  shouldOpenAvailabilityTargetAddDialog,
} from './availabilitySettingsModel';
import {
  LEGACY_SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
  LEGACY_SELF_HOSTED_PRO_BILLING_ROUTE,
  LEGACY_SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
  SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
  SELF_HOSTED_PRO_BILLING_ROUTE,
  SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
} from '@/utils/pricingHandoff';
import { SETTINGS_API_ACCESS_PATH } from '@/routing/resourceLinks';

export type SettingsTab =
  | 'infrastructure-systems'
  | 'monitoring-availability'
  | 'system-general'
  | 'system-network'
  | 'system-updates'
  | 'system-recovery'
  | 'system-ai'
  | 'system-ai-patrol'
  | 'system-ai-assistant'
  | 'system-ai-discovery'
  | 'system-relay'
  | 'system-billing'
  | 'support-diagnostics'
  | 'support-reporting'
  | 'support-logs'
  | 'organization-overview'
  | 'organization-access'
  | 'organization-billing'
  | 'organization-billing-admin'
  | 'organization-sharing'
  | 'api'
  | 'security-overview'
  | 'security-data-handling'
  | 'security-auth'
  | 'security-sso'
  | 'security-roles'
  | 'security-users'
  | 'security-audit'
  | 'security-webhooks';

export type InfrastructureSettingsTab = Extract<SettingsTab, 'infrastructure-systems'>;

export type AgentKey = 'pve' | 'pbs' | 'pmg';
export type ProxmoxPlatformType = Extract<
  PlatformType,
  'proxmox-pve' | 'proxmox-pbs' | 'proxmox-pmg'
>;

export type SettingsNavGroupId =
  | 'infrastructure'
  | 'monitoring'
  | 'pulse-intelligence'
  | 'organization'
  | 'system'
  | 'support'
  | 'security';

export interface SettingsNavItem {
  id: SettingsTab;
  label: string;
  // Search-only aliases (never rendered). Lets sidebar search find a page by
  // terms its label/description can't carry, e.g. "mcp" or "claude" for the
  // Assistant page's external-agent connector setup.
  keywords?: string[];
  icon: Component<{ class?: string; strokeWidth?: number }>;
  iconProps?: { strokeWidth?: number };
  hideFromSidebar?: boolean;
  saveBehavior?: 'system';
  disabled?: boolean;
  locked?: boolean;
  hideWhenUnavailable?: boolean;
  hostedOnly?: boolean;
  hideWhenCommercialHidden?: boolean;
  hideWhenOrganizationHidden?: boolean;
  hideWhenDemoMode?: boolean;
  hideWhenReadOnly?: boolean;
  requiredCapability?: keyof SecurityStatusSettingsCapabilities;
  badge?: string;
  features?: string[];
  permissions?: string[];
}

export interface SettingsNavGroup {
  id: SettingsNavGroupId;
  label: string;
  items: SettingsNavItem[];
}

export interface SettingsHeaderMeta {
  title: string;
  description: string;
}

export type SettingsHeaderMetaMap = Record<SettingsTab, SettingsHeaderMeta>;

// Default landing tab for /settings when no deep-link tab is provided.
export const DEFAULT_SETTINGS_TAB: SettingsTab = 'infrastructure-systems';
const INFRASTRUCTURE_SYSTEMS_PREFIX = '/settings/infrastructure';
const MONITORING_PREFIX = '/settings/monitoring';
const MONITORING_AVAILABILITY_PREFIX = `${MONITORING_PREFIX}/availability`;
export const SETTINGS_PULSE_INTELLIGENCE_PATH = '/settings/pulse-intelligence';
export const SETTINGS_PROVIDER_MODELS_PATH = `${SETTINGS_PULSE_INTELLIGENCE_PATH}/provider`;
const PULSE_INTELLIGENCE_PREFIX = SETTINGS_PULSE_INTELLIGENCE_PATH;
const PULSE_INTELLIGENCE_PROVIDER_PREFIX = SETTINGS_PROVIDER_MODELS_PATH;
const PULSE_INTELLIGENCE_PATROL_PREFIX = `${PULSE_INTELLIGENCE_PREFIX}/patrol`;
const PULSE_INTELLIGENCE_ASSISTANT_PREFIX = `${PULSE_INTELLIGENCE_PREFIX}/assistant`;
const PULSE_INTELLIGENCE_DISCOVERY_PREFIX = `${PULSE_INTELLIGENCE_PREFIX}/discovery`;
const LEGACY_SYSTEM_AI_PREFIX = '/settings/system-ai';
const RETIRED_SETTINGS_WORKLOADS_PREFIX = '/settings/workloads';
const RETIRED_SETTINGS_OPERATIONS_PREFIX = '/settings/operations';
const RETIRED_SETTINGS_INTEGRATIONS_API_PREFIX = '/settings/integrations/api';
const RETIRED_SETTINGS_SYSTEM_PRO_PREFIX = '/settings/system-pro';
const SUPPORT_PREFIX = '/settings/support';
const SUPPORT_DIAGNOSTICS_PREFIX = `${SUPPORT_PREFIX}/diagnostics`;
const SUPPORT_REPORTING_PREFIX = `${SUPPORT_PREFIX}/reporting`;
const SUPPORT_LOGS_PREFIX = `${SUPPORT_PREFIX}/logs`;
const SECURITY_API_PREFIX = SETTINGS_API_ACCESS_PATH;
const SELF_HOSTED_BILLING_PREFIX = SELF_HOSTED_PRO_BILLING_ROUTE;
const LEGACY_SYSTEM_BILLING_PREFIX = LEGACY_SELF_HOSTED_PRO_BILLING_ROUTE;

const PROXMOX_AGENT_META: Record<
  AgentKey,
  {
    platformType: ProxmoxPlatformType;
    label: string;
    nodeLabel: string;
  }
> = {
  pve: {
    platformType: 'proxmox-pve',
    label: 'Proxmox VE',
    nodeLabel: 'Proxmox VE node',
  },
  pbs: {
    platformType: 'proxmox-pbs',
    label: 'Proxmox Backup Server',
    nodeLabel: 'Proxmox Backup Server',
  },
  pmg: {
    platformType: 'proxmox-pmg',
    label: 'Proxmox Mail Gateway',
    nodeLabel: 'Proxmox Mail Gateway',
  },
};

const normalizeSettingsPath = (path: string): string => {
  const trimmed = (path || '').trim();
  if (!trimmed) return '/settings';
  if (trimmed.length > 1 && trimmed.endsWith('/')) {
    return trimmed.replace(/\/+$/, '');
  }
  return trimmed;
};

export function isRetiredSettingsCompatibilityPath(path: string): boolean {
  const normalizedPath = normalizeSettingsPath(path);
  return (
    normalizedPath === RETIRED_SETTINGS_WORKLOADS_PREFIX ||
    normalizedPath.startsWith(`${RETIRED_SETTINGS_WORKLOADS_PREFIX}/`) ||
    normalizedPath === RETIRED_SETTINGS_OPERATIONS_PREFIX ||
    normalizedPath.startsWith(`${RETIRED_SETTINGS_OPERATIONS_PREFIX}/`) ||
    normalizedPath === RETIRED_SETTINGS_INTEGRATIONS_API_PREFIX ||
    normalizedPath.startsWith(`${RETIRED_SETTINGS_INTEGRATIONS_API_PREFIX}/`) ||
    normalizedPath === RETIRED_SETTINGS_SYSTEM_PRO_PREFIX ||
    normalizedPath.startsWith(`${RETIRED_SETTINGS_SYSTEM_PRO_PREFIX}/`) ||
    // Infrastructure subpaths are retired v5-era deep links, except Agent
    // Doctor, which is a live routed page under the infrastructure workspace.
    (normalizedPath.startsWith(`${INFRASTRUCTURE_SYSTEMS_PREFIX}/`) &&
      normalizedPath !== INFRASTRUCTURE_AGENT_DOCTOR_PATH)
  );
}

export function resolveCanonicalSettingsPath(path: string): string | null {
  const normalizedPath = normalizeSettingsPath(path);
  if (normalizedPath !== '/settings' && !normalizedPath.startsWith('/settings/')) return null;
  if (isRetiredSettingsCompatibilityPath(normalizedPath)) return null;
  if (normalizedPath === '/settings') {
    return settingsTabPath(DEFAULT_SETTINGS_TAB);
  }
  if (normalizedPath === INFRASTRUCTURE_SYSTEMS_PREFIX) {
    return settingsTabPath(DEFAULT_SETTINGS_TAB);
  }
  if (normalizedPath === MONITORING_PREFIX) {
    return MONITORING_AVAILABILITY_PREFIX;
  }
  if (
    normalizedPath === PULSE_INTELLIGENCE_PREFIX ||
    normalizedPath === PULSE_INTELLIGENCE_PROVIDER_PREFIX ||
    normalizedPath === LEGACY_SYSTEM_AI_PREFIX
  ) {
    return settingsTabPath('system-ai');
  }
  if (normalizedPath === PULSE_INTELLIGENCE_DISCOVERY_PREFIX) {
    return settingsTabPath('system-ai-assistant');
  }
  if (normalizedPath === SUPPORT_PREFIX) {
    return SUPPORT_DIAGNOSTICS_PREFIX;
  }
  if (
    normalizedPath === SELF_HOSTED_BILLING_PREFIX ||
    normalizedPath === LEGACY_SYSTEM_BILLING_PREFIX ||
    normalizedPath === LEGACY_SELF_HOSTED_PRO_BILLING_PLAN_ROUTE
  ) {
    return SELF_HOSTED_PRO_BILLING_PLAN_ROUTE;
  }
  if (normalizedPath === LEGACY_SELF_HOSTED_PRO_BILLING_USAGE_ROUTE) {
    return SELF_HOSTED_PRO_BILLING_USAGE_ROUTE;
  }
  return normalizedPath;
}

export function deriveTabFromPath(path: string): SettingsTab {
  const canonicalPath = resolveCanonicalSettingsPath(path) ?? normalizeSettingsPath(path);

  if (canonicalPath === '/settings') return DEFAULT_SETTINGS_TAB;
  if (
    canonicalPath === INFRASTRUCTURE_SYSTEMS_PREFIX ||
    canonicalPath === INFRASTRUCTURE_AGENT_DOCTOR_PATH
  ) {
    return 'infrastructure-systems';
  }
  if (canonicalPath.startsWith(MONITORING_AVAILABILITY_PREFIX)) {
    return 'monitoring-availability';
  }
  if (canonicalPath.startsWith(PULSE_INTELLIGENCE_PATROL_PREFIX)) {
    return 'system-ai-patrol';
  }
  if (canonicalPath.startsWith(PULSE_INTELLIGENCE_ASSISTANT_PREFIX)) {
    return 'system-ai-assistant';
  }
  if (canonicalPath.startsWith(PULSE_INTELLIGENCE_DISCOVERY_PREFIX)) {
    return 'system-ai-discovery';
  }
  if (canonicalPath.startsWith(PULSE_INTELLIGENCE_PROVIDER_PREFIX)) {
    return 'system-ai';
  }

  if (canonicalPath.includes('/settings/system-general')) return 'system-general';
  if (canonicalPath.includes('/settings/system-network')) return 'system-network';
  if (canonicalPath.includes('/settings/system-updates')) return 'system-updates';
  if (canonicalPath.includes('/settings/system-recovery')) return 'system-recovery';
  if (canonicalPath.includes('/settings/system-ai')) return 'system-ai';
  if (canonicalPath.includes('/settings/system-relay')) return 'system-relay';
  if (canonicalPath.includes(SELF_HOSTED_BILLING_PREFIX)) return 'system-billing';
  if (canonicalPath.startsWith(SUPPORT_LOGS_PREFIX)) return 'support-logs';
  if (canonicalPath.startsWith(SUPPORT_REPORTING_PREFIX)) return 'support-reporting';
  if (canonicalPath.startsWith(SUPPORT_DIAGNOSTICS_PREFIX) || canonicalPath === SUPPORT_PREFIX)
    return 'support-diagnostics';
  if (canonicalPath.includes('/settings/organization/access')) return 'organization-access';
  if (canonicalPath.includes('/settings/organization/sharing')) return 'organization-sharing';
  if (canonicalPath.includes('/settings/organization/billing-admin'))
    return 'organization-billing-admin';
  if (canonicalPath.includes('/settings/organization/billing')) return 'organization-billing';
  if (canonicalPath.includes('/settings/organization')) return 'organization-overview';

  if (canonicalPath.includes(SECURITY_API_PREFIX)) return 'api';

  if (canonicalPath.includes('/settings/security-overview')) return 'security-overview';
  if (canonicalPath.includes('/settings/security-data-handling')) return 'security-data-handling';
  if (canonicalPath.includes('/settings/security-auth')) return 'security-auth';
  if (canonicalPath.includes('/settings/security-sso')) return 'security-sso';
  if (canonicalPath.includes('/settings/security-roles')) return 'security-roles';
  if (canonicalPath.includes('/settings/security-users')) return 'security-users';
  if (canonicalPath.includes('/settings/security-audit')) return 'security-audit';
  if (canonicalPath.includes('/settings/security-webhooks')) return 'security-webhooks';

  return DEFAULT_SETTINGS_TAB;
}

export function settingsAgentPlatformType(agent: AgentKey): ProxmoxPlatformType {
  return PROXMOX_AGENT_META[agent].platformType;
}

export function settingsAgentLabel(agent: AgentKey): string {
  return PROXMOX_AGENT_META[agent].label;
}

export function settingsAgentNodeLabel(agent: AgentKey): string {
  return PROXMOX_AGENT_META[agent].nodeLabel;
}

export function agentKeyFromPlatformType(value: string | null | undefined): AgentKey | null {
  const normalized = normalizeSourcePlatformKey(value);
  switch (normalized) {
    case 'proxmox-pve':
      return 'pve';
    case 'proxmox-pbs':
      return 'pbs';
    case 'proxmox-pmg':
      return 'pmg';
    default:
      return null;
  }
}

export function deriveTabFromQuery(search: string): SettingsTab | null {
  const params = new URLSearchParams(search);
  if (isAISettingsOAuthCallbackQuery(search)) return 'system-ai';

  const tab = params.get('tab')?.trim().toLowerCase();
  if (!tab) return null;

  switch (tab) {
    case 'infrastructure':
      return 'infrastructure-systems';
    case 'availability':
    case 'monitoring-availability':
      return 'monitoring-availability';
    case 'system-recovery':
      return 'system-recovery';
    case 'system-updates':
      return 'system-updates';
    case 'system-network':
      return 'system-network';
    case 'system-general':
      return 'system-general';
    case 'system-ai':
    case 'pulse-intelligence':
    case 'provider-models':
    case 'provider':
      return 'system-ai';
    case 'system-ai-patrol':
    case 'patrol':
      return 'system-ai-patrol';
    case 'system-ai-assistant':
    case 'assistant':
      return 'system-ai-assistant';
    case 'system-ai-discovery':
    case 'discovery':
      return 'system-ai-assistant';
    case 'system-relay':
      return 'system-relay';
    case 'system-billing':
      return 'system-billing';
    case 'diagnostics':
    case 'support-diagnostics':
      return 'support-diagnostics';
    case 'reporting':
    case 'support-reporting':
      return 'support-reporting';
    case 'logs':
    case 'support-logs':
      return 'support-logs';
    case 'api':
      return 'api';
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
    case 'security-data-handling':
    case 'data-handling':
    case 'resource-privacy':
      return 'security-data-handling';
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

export function isAISettingsOAuthCallbackQuery(search: string): boolean {
  const params = new URLSearchParams(search);
  return params.has('ai_oauth_success') || params.has('ai_oauth_error');
}

export function settingsTabPath(tab: SettingsTab): string {
  switch (tab) {
    case 'infrastructure-systems':
      return INFRASTRUCTURE_SYSTEMS_PREFIX;
    case 'monitoring-availability':
      return MONITORING_AVAILABILITY_PREFIX;
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
    case 'system-ai':
      return PULSE_INTELLIGENCE_PROVIDER_PREFIX;
    case 'system-ai-patrol':
      return PULSE_INTELLIGENCE_PATROL_PREFIX;
    case 'system-ai-assistant':
      return PULSE_INTELLIGENCE_ASSISTANT_PREFIX;
    case 'system-ai-discovery':
      return PULSE_INTELLIGENCE_DISCOVERY_PREFIX;
    case 'system-relay':
      return '/settings/system-relay';
    case 'system-billing':
      return SELF_HOSTED_PRO_BILLING_PLAN_ROUTE;
    case 'support-diagnostics':
      return SUPPORT_DIAGNOSTICS_PREFIX;
    case 'support-reporting':
      return SUPPORT_REPORTING_PREFIX;
    case 'support-logs':
      return SUPPORT_LOGS_PREFIX;
    default:
      return `/settings/${tab}`;
  }
}

const ROUTEABLE_SETTINGS_PATHS = new Set<string>([
  settingsTabPath('infrastructure-systems'),
  INFRASTRUCTURE_AGENT_DOCTOR_PATH,
  settingsTabPath('monitoring-availability'),
  settingsTabPath('system-general'),
  settingsTabPath('system-network'),
  settingsTabPath('system-updates'),
  settingsTabPath('system-recovery'),
  settingsTabPath('system-ai'),
  settingsTabPath('system-ai-patrol'),
  settingsTabPath('system-ai-assistant'),
  settingsTabPath('system-ai-discovery'),
  settingsTabPath('system-relay'),
  settingsTabPath('system-billing'),
  SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
  settingsTabPath('support-diagnostics'),
  settingsTabPath('support-reporting'),
  settingsTabPath('support-logs'),
  settingsTabPath('organization-overview'),
  settingsTabPath('organization-access'),
  settingsTabPath('organization-billing'),
  settingsTabPath('organization-billing-admin'),
  settingsTabPath('organization-sharing'),
  settingsTabPath('api'),
  settingsTabPath('security-overview'),
  settingsTabPath('security-data-handling'),
  settingsTabPath('security-auth'),
  settingsTabPath('security-sso'),
  settingsTabPath('security-roles'),
  settingsTabPath('security-users'),
  settingsTabPath('security-audit'),
  settingsTabPath('security-webhooks'),
]);

export function isRouteableSettingsPath(path: string): boolean {
  const normalizedPath = normalizeSettingsPath(path);
  if (normalizedPath === '/settings') return true;
  if (isRetiredSettingsCompatibilityPath(normalizedPath)) return false;

  const canonicalPath = resolveCanonicalSettingsPath(normalizedPath);
  return canonicalPath ? ROUTEABLE_SETTINGS_PATHS.has(canonicalPath) : false;
}

export function isRouteableSettingsLocation(path: string, search = ''): boolean {
  if (!isRouteableSettingsPath(path)) return false;

  const normalizedPath = normalizeSettingsPath(path);
  const params = new URLSearchParams(search);
  if (
    normalizedPath === INFRASTRUCTURE_SYSTEMS_PREFIX &&
    params.has(INFRASTRUCTURE_ADD_QUERY_PARAM) &&
    deriveAddStepFromSearch(search) === null
  ) {
    return false;
  }

  if (
    normalizedPath === MONITORING_AVAILABILITY_PREFIX &&
    params.has(AVAILABILITY_ADD_QUERY_PARAM) &&
    !shouldOpenAvailabilityTargetAddDialog(normalizedPath, search)
  ) {
    return false;
  }

  return true;
}

export function isInfrastructureSettingsTab(tab: SettingsTab): tab is InfrastructureSettingsTab {
  return tab === 'infrastructure-systems';
}
