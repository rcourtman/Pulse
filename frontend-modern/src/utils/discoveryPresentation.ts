import type {
  AvailabilityProbeSuggestion,
  DiscoveryFact,
  ResourceDiscovery,
} from '@/types/discovery';
import {
  getInfrastructureSettingsLocationLabel,
  getInfrastructureSettingsTarget,
} from '@/utils/infrastructureSettingsPresentation';
import { SETTINGS_PULSE_INTELLIGENCE_ASSISTANT_PATH } from '@/routing/resourceLinks';

export interface DiscoveryIdentifiedSummary {
  serviceName: string;
  serviceType?: string;
  serviceVersion?: string;
  category?: string;
  confidence: number;
  confidencePercent: string;
  cliAccess?: string;
  portCount: number;
  configPathCount: number;
  dataPathCount: number;
  logPathCount: number;
  discoveredAt?: string;
  observedAt?: string;
  sourceLabel: string;
  suggestedUrl?: string;
  suggestedUrlReasonText?: string;
  suggestedUrlReasonTitle?: string;
  suggestedUrlDiagnostic?: string;
  hasEndpointCandidate: boolean;
  suggestedAvailabilityProbe?: AvailabilityProbeSuggestion;
}

const OBSERVED_SOURCE_LABEL = 'Observed by Discovery';
const DISCOVERY_PROVENANCE_LABEL = 'Discovery';
const DISCOVERY_PROVENANCE_TITLE =
  'Surfaced by opt-in Discovery from agent-observed workload context.';

const toSentence = (text?: string | null): string => {
  const trimmed = (text || '').trim();
  if (!trimmed) return '';
  const first = trimmed.charAt(0);
  return first.toUpperCase() + trimmed.slice(1);
};

const normalizeDiscoveryToken = (value?: string | null): string => {
  return (value || '').trim().toLowerCase().replace(/[_-]+/g, ' ').replace(/\s+/g, ' ');
};

const isMeaningfulDiscoveryText = (value?: string | null): boolean => {
  const normalized = normalizeDiscoveryToken(value);
  if (!normalized) return false;
  return ![
    'detected',
    'n a',
    'none',
    'app',
    'application',
    'container',
    'host',
    'linux',
    'lxc',
    'service',
    'system container',
    'unknown',
    'unknown app',
    'unknown application',
    'unknown container',
    'unknown host',
    'unknown service',
    'unknown system container',
    'unknown virtual machine',
    'unknown vm',
    'unknown workload',
    'virtual machine',
    'vm',
    'workload',
  ].includes(normalized);
};

const isMeaningfulDiscoveryFact = (fact: DiscoveryFact): boolean => {
  const key = normalizeDiscoveryToken(fact.key);
  const value = normalizeDiscoveryToken(fact.value);
  const source = normalizeDiscoveryToken(fact.source);
  if (!isMeaningfulDiscoveryText(fact.value)) return false;
  if (key === 'status' && source === 'metadata') return false;
  if (key.includes('availability') && value.includes('missing')) return false;
  if (key.startsWith('missing') || key.endsWith('missing')) return false;
  if (value.startsWith('missing ') || value.includes(' missing ')) return false;
  if (
    value.includes('does not exist') ||
    value.includes('not found') ||
    value.includes('failed') ||
    value.includes('error')
  ) {
    return false;
  }
  return ['config', 'dependency', 'network', 'port', 'security', 'service', 'version'].includes(
    normalizeDiscoveryToken(fact.category),
  );
};

export function hasMeaningfulDiscoveryContext(
  discovery: ResourceDiscovery | null | undefined,
): boolean {
  if (!discovery) return false;
  const portCount = Array.isArray(discovery.ports) ? discovery.ports.length : 0;
  const configPathCount = Array.isArray(discovery.config_paths) ? discovery.config_paths.length : 0;
  const dataPathCount = Array.isArray(discovery.data_paths) ? discovery.data_paths.length : 0;
  const logPathCount = Array.isArray(discovery.log_paths) ? discovery.log_paths.length : 0;
  const hasFacts =
    Array.isArray(discovery.facts) &&
    discovery.facts.some((fact) => isMeaningfulDiscoveryFact(fact));
  const hasPaths = configPathCount + dataPathCount + logPathCount > 0;
  const hasSuggestedUrl = Boolean(normalizeDiscoverySuggestedUrl(discovery.suggested_url));
  const hasSuggestedProbe = Boolean(discovery.suggested_availability_probe);

  return Boolean(
    isMeaningfulDiscoveryText(discovery.service_name) ||
    isMeaningfulDiscoveryText(discovery.service_type) ||
    isMeaningfulDiscoveryText(discovery.service_version) ||
    isMeaningfulDiscoveryText(discovery.category) ||
    portCount > 0 ||
    hasFacts ||
    hasPaths ||
    hasSuggestedUrl ||
    hasSuggestedProbe,
  );
}

// getDiscoveryIdentifiedSummary returns a compact presentation object for
// surfaces outside the Discovery sub-tab (e.g. the workload drawer overview)
// to label a resource with its identified service and endpoint candidates.
// Returns null when the stored record has no meaningful observed context — the gate mirrors
// hasValidDiscovery in useDiscoveryTabState so the same record either
// renders in both surfaces or neither.
export function getDiscoveryIdentifiedSummary(
  discovery: ResourceDiscovery | null | undefined,
): DiscoveryIdentifiedSummary | null {
  if (!discovery) return null;
  if (!hasMeaningfulDiscoveryContext(discovery)) return null;
  const serviceName = (discovery.service_name || '').trim();
  const hasName = isMeaningfulDiscoveryText(serviceName);
  const confidence = typeof discovery.confidence === 'number' ? discovery.confidence : 0;
  const portCount = Array.isArray(discovery.ports) ? discovery.ports.length : 0;
  const configPathCount = Array.isArray(discovery.config_paths) ? discovery.config_paths.length : 0;
  const dataPathCount = Array.isArray(discovery.data_paths) ? discovery.data_paths.length : 0;
  const logPathCount = Array.isArray(discovery.log_paths) ? discovery.log_paths.length : 0;
  const hasCli = typeof discovery.cli_access === 'string' && discovery.cli_access.trim().length > 0;
  const suggestedUrl = normalizeDiscoverySuggestedUrl(discovery.suggested_url);
  const suggestedUrlReason = getDiscoverySuggestedURLReason(discovery);
  return {
    serviceName: hasName ? serviceName : 'Unidentified service',
    serviceType: isMeaningfulDiscoveryText(discovery.service_type)
      ? discovery.service_type?.trim()
      : undefined,
    serviceVersion: isMeaningfulDiscoveryText(discovery.service_version)
      ? discovery.service_version?.trim()
      : undefined,
    category: isMeaningfulDiscoveryText(discovery.category)
      ? discovery.category?.trim()
      : undefined,
    confidence,
    confidencePercent: `${Math.round(confidence * 100)}%`,
    cliAccess: hasCli ? discovery.cli_access?.trim() : undefined,
    portCount,
    configPathCount,
    dataPathCount,
    logPathCount,
    discoveredAt: discovery.discovered_at || undefined,
    observedAt: discovery.updated_at || discovery.discovered_at || undefined,
    sourceLabel: OBSERVED_SOURCE_LABEL,
    suggestedUrl,
    suggestedUrlReasonText: suggestedUrlReason.text || undefined,
    suggestedUrlReasonTitle: suggestedUrlReason.title || undefined,
    suggestedUrlDiagnostic: discovery.suggested_url_diagnostic?.trim() || undefined,
    hasEndpointCandidate: Boolean(suggestedUrl),
    suggestedAvailabilityProbe: discovery.suggested_availability_probe ?? undefined,
  };
}

export function getDiscoveryObservedSourceLabel(): string {
  return OBSERVED_SOURCE_LABEL;
}

export function getDiscoveryProvenanceLabel(): string {
  return DISCOVERY_PROVENANCE_LABEL;
}

export function getDiscoveryProvenanceTitle(): string {
  return DISCOVERY_PROVENANCE_TITLE;
}

export function getDiscoveryProvenanceBadgeClass(): string {
  return 'inline-flex h-5 shrink-0 items-center gap-1 rounded border border-cyan-200 bg-cyan-50 px-1.5 text-[10px] font-medium leading-none text-cyan-700 dark:border-cyan-800 dark:bg-cyan-950 dark:text-cyan-200';
}

export function getDiscoveryProvenanceIconClass(): string {
  return 'inline-flex h-4 w-4 shrink-0 items-center justify-center rounded border border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-800 dark:bg-cyan-950 dark:text-cyan-200';
}

export function normalizeDiscoverySuggestedUrl(value?: string | null): string | undefined {
  const trimmed = (value || '').trim();
  return trimmed || undefined;
}

export function getDiscoveryURLSuggestionSourceLabel(code?: string | null): string {
  switch ((code || '').trim()) {
    case 'service_default_match':
      return 'Known service default';
    case 'service_default_variation_match':
      return 'Known service variant';
    case 'web_port_inference':
      return 'Detected web port';
    case 'host_management_profile_proxmox_node':
    case 'host_management_profile_linked_proxmox_node':
    case 'host_management_profile_pve':
      return 'Proxmox node profile';
    case 'host_management_profile_pbs':
      return 'Proxmox Backup profile';
    case 'host_management_profile_pmg':
      return 'Proxmox Mail Gateway profile';
    case 'host_management_profile_nas':
      return 'NAS node profile';
    default:
      return 'Discovery heuristic';
  }
}

export function getDiscoverySuggestedURLReason(
  discovery?: {
    suggested_url_source_code?: string | null;
    suggested_url_source_detail?: string | null;
  } | null,
) {
  if (!discovery) {
    return { text: '', title: '' } as const;
  }
  const detail = toSentence(discovery.suggested_url_source_detail);
  const label = getDiscoveryURLSuggestionSourceLabel(discovery.suggested_url_source_code);
  const title = discovery.suggested_url_source_detail
    ? `${label}: ${discovery.suggested_url_source_detail}`
    : label;
  return {
    text: detail || (discovery.suggested_url_source_code ? label : ''),
    title,
  } as const;
}

export function getDiscoveryAnalysisProviderBadgeClass(isLocal?: boolean | null): string {
  return isLocal
    ? 'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
    : 'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
}

export function getDiscoveryCategoryBadgeClass(): string {
  return 'inline-block rounded bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900 dark:text-blue-200';
}

export function getDiscoverySuggestedURLCardClass(): string {
  return 'rounded border border-blue-200 bg-blue-50 p-3 shadow-sm dark:border-blue-800 dark:bg-blue-900';
}

export function getDiscoverySuggestedURLHeadingClass(): string {
  return 'text-[11px] font-medium uppercase tracking-wide text-blue-800 dark:text-blue-200 mb-1';
}

export function getDiscoverySuggestedURLTextClass(): string {
  return 'text-blue-700 dark:text-blue-300';
}

export function getDiscoverySuggestedURLCodeClass(): string {
  return 'min-w-0 flex-1 rounded bg-blue-100 px-2 py-1.5 text-xs text-blue-800 dark:bg-blue-950 dark:text-blue-100 font-mono break-all';
}

export function getDiscoverySuggestedURLActionClass(): string {
  return 'inline-flex min-h-8 min-w-8 shrink-0 items-center justify-center rounded border border-blue-200 bg-blue-100 text-blue-700 transition-colors hover:bg-blue-200 dark:border-blue-700 dark:bg-blue-950 dark:text-blue-200 dark:hover:bg-blue-900';
}

export function getDiscoveryInitialEmptyState(loading: boolean) {
  return loading
    ? {
        title: 'Checking existing discovery data...',
        description: 'You can run a discovery scan if this takes too long.',
      }
    : {
        title: 'No discovery data yet',
        description: 'Run a discovery scan to identify services and configuration details.',
      };
}

export function getDiscoveryLoadingState() {
  return {
    text: 'Loading discovery data...',
  } as const;
}

export function getDiscoverySuggestedURLFallback(diagnostic?: string | null) {
  return {
    title: 'No suggested URL available',
    description: diagnostic || '',
  } as const;
}

export function getDiscoveryNotesEmptyState() {
  return {
    text: 'No discovery notes yet. Add notes to capture important context.',
  } as const;
}

export function getDiscoveryCommandSettingsTarget() {
  return getInfrastructureSettingsTarget();
}

export function getDiscoveryApiAccessSettingsTarget() {
  return {
    href: '/settings/security/api',
    label: 'Settings → API Access',
  } as const;
}

export function getDiscoveryServiceContextSettingsTarget() {
  return {
    href: SETTINGS_PULSE_INTELLIGENCE_ASSISTANT_PATH,
    label: 'Open Assistant settings',
  } as const;
}

export function getDiscoveryNoConnectedAgentMessage(commandsEnabled?: boolean): string {
  const infrastructureSettings = getInfrastructureSettingsLocationLabel();

  if (commandsEnabled === false) {
    return `Commands not enabled. Enable Pulse commands from ${infrastructureSettings} for this agent.`;
  }

  if (commandsEnabled === true) {
    return 'Agent not connected for command execution. The API token may be missing the "agent:exec" scope. Check Settings → API Access.';
  }

  return `No agent available for command execution. Enable Pulse commands from ${infrastructureSettings} and make sure the API token has "agent:exec" scope in Settings → API Access.`;
}

export function getNetworkDiscoveryPriorityNotice() {
  return {
    title: 'Network scan safety',
    items: [
      'Environment variables still override these settings.',
      'Changes made here are saved to system.json immediately.',
      'Automatic mode can scan every detected interface, including bridge or shared networks; use custom subnets when scope matters.',
    ],
  } as const;
}

export function getNetworkDiscoverySectionPresentation(discoveryEnabled: boolean) {
  return {
    headerTitle: 'Network discovery',
    headerDescription: 'Control how Pulse scans your network for Proxmox services.',
    toggleTitle: 'Automatic scanning',
    toggleDescription:
      'Enable discovery to surface Proxmox VE, Proxmox Backup Server, and Proxmox Mail Gateway endpoints automatically.',
    toggleStateLabel: discoveryEnabled ? 'Enabled' : 'Disabled',
    scanScopeLabel: 'Scan scope',
    commonNetworksLabel: 'Common networks',
    environmentOverrideMessage:
      'Discovery settings are locked by environment variables. Update the service configuration and restart Pulse to change them here.',
  } as const;
}

export function getNetworkDiscoveryModePresentation(mode: 'auto' | 'custom') {
  if (mode === 'auto') {
    return {
      label: 'Automatic scan (full network scope)',
      description:
        'Scan every network interface on this host, including container bridges, local subnets, and gateways. Use custom subnets on large or shared networks to reduce scan time.',
    } as const;
  }

  return {
    label: 'Custom subnets (targeted)',
    description:
      'Limit discovery to one or more CIDR ranges for faster, more targeted scans on large networks.',
  } as const;
}

export function getNetworkDiscoverySubnetPresentation(mode: 'auto' | 'custom') {
  return {
    label: 'Discovery subnets',
    helpTooltip:
      'Use CIDR notation, for example 192.168.1.0/24 or 10.0.0.0/24. Smaller ranges finish more quickly.',
    placeholder:
      mode === 'auto' ? 'automatic (scan every detected network)' : '192.168.1.0/24, 10.0.0.0/24',
    guidance:
      mode === 'auto'
        ? 'Automatic mode scans all host network interfaces, which can include shared or corporate networks. Switch to custom subnets for a faster, more targeted scan.'
        : 'Example: 192.168.1.0/24, 10.0.0.0/24. Smaller ranges finish faster and reduce timeout risk.',
  } as const;
}
