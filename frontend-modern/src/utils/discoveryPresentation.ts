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
        description: 'You can run discovery now if this takes too long.',
      }
    : {
        title: 'No discovery data yet',
        description: 'Run a discovery scan to identify services and configurations',
      };
}

export function getDiscoveryLoadingState() {
  return {
    text: 'Loading discovery...',
  } as const;
}

export function getDiscoverySuggestedURLFallback(diagnostic?: string | null) {
  return {
    title: 'No suggested URL found',
    description: diagnostic || '',
  } as const;
}

export function getDiscoveryNotesEmptyState() {
  return {
    text: 'No notes yet. Add notes to document important information.',
  } as const;
}
