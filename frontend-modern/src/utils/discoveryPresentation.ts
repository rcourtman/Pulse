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

export function getDiscoveryAnalysisProviderBadgeClass(isLocal?: boolean | null): string {
  return isLocal
    ? 'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
    : 'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
}

export function getDiscoveryCategoryBadgeClass(): string {
  return 'inline-block rounded bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900 dark:text-blue-200';
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

export function getNetworkDiscoveryPriorityNotice() {
  return {
    title: 'Configuration precedence',
    items: [
      'Environment variables still override these settings.',
      'Changes made here are saved to system.json immediately.',
      'These settings remain in effect until an environment override replaces them.',
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
      mode === 'auto'
        ? 'automatic (scan every detected network)'
        : '192.168.1.0/24, 10.0.0.0/24',
    guidance:
      mode === 'auto'
        ? 'Automatic mode scans all host network interfaces, which can include shared or corporate networks. Switch to custom subnets for a faster, more targeted scan.'
        : 'Example: 192.168.1.0/24, 10.0.0.0/24. Smaller ranges finish faster and reduce timeout risk.',
  } as const;
}
