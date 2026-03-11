export const PBS_THRESHOLDS_EMPTY_STATE = 'No PBS servers configured.';
export const GUEST_THRESHOLDS_EMPTY_STATE = 'No VMs or containers found.';
export const NODE_THRESHOLDS_FILTER_EMPTY_STATE = 'No nodes match the current filters.';
export const PBS_THRESHOLDS_FILTER_EMPTY_STATE = 'No PBS servers match the current filters.';
export const GUEST_THRESHOLDS_FILTER_EMPTY_STATE = 'No VMs or containers match the current filters.';
export const GUEST_FILTERING_EMPTY_STATE = 'Configure guest filtering rules.';
export const BACKUP_THRESHOLDS_EMPTY_STATE = 'Configure recovery alert thresholds.';
export const SNAPSHOT_THRESHOLDS_EMPTY_STATE = 'Configure snapshot age thresholds.';
export const STORAGE_THRESHOLDS_EMPTY_STATE = 'No storage devices found.';
export const STORAGE_THRESHOLDS_FILTER_EMPTY_STATE = 'No storage devices match the current filters.';
export const PMG_THRESHOLDS_EMPTY_STATE =
  'No mail gateways configured yet. Add a PMG instance in Settings to manage thresholds.';
export const PMG_THRESHOLDS_FILTER_EMPTY_STATE = 'No mail gateways match the current filters.';
export const AGENT_THRESHOLDS_FILTER_EMPTY_STATE = 'No agents match the current filters.';
export const AGENT_DISKS_EMPTY_STATE =
  'No agent disks found. Agents with mounted filesystems will appear here.';
export const AGENT_DISKS_FILTER_EMPTY_STATE = 'No agent disks match the current filters.';
export const CONTAINER_RUNTIMES_FILTER_EMPTY_STATE =
  'No container runtimes match the current filters.';
export const CONTAINERS_FILTER_EMPTY_STATE = 'No containers match the current filters.';
export const ALERT_THRESHOLDS_SEARCH_PLACEHOLDER = 'Search resources...';
export const ALERT_THRESHOLDS_HELP_DISMISS_LABEL = 'Dismiss tips';
export const ALERT_THRESHOLDS_GUEST_IGNORED_PREFIXES_TITLE = 'Ignored Prefixes';
export const ALERT_THRESHOLDS_GUEST_IGNORED_PREFIXES_DESCRIPTION =
  'Skip metrics for guests starting with:';
export const ALERT_THRESHOLDS_GUEST_IGNORED_PREFIXES_PLACEHOLDER = 'dev-';
export const ALERT_THRESHOLDS_GUEST_TAG_WHITELIST_TITLE = 'Tag Whitelist';
export const ALERT_THRESHOLDS_GUEST_TAG_WHITELIST_DESCRIPTION =
  'Only monitor guests with at least one of these tags (leave empty to disable whitelist):';
export const ALERT_THRESHOLDS_GUEST_TAG_WHITELIST_PLACEHOLDER = 'production';
export const ALERT_THRESHOLDS_GUEST_TAG_BLACKLIST_TITLE = 'Tag Blacklist';
export const ALERT_THRESHOLDS_GUEST_TAG_BLACKLIST_DESCRIPTION =
  'Ignore guests with any of these tags:';
export const ALERT_THRESHOLDS_GUEST_TAG_BLACKLIST_PLACEHOLDER = 'maintenance';
export const ALERT_THRESHOLDS_BACKUP_ORPHANED_TITLE = 'Orphaned backups';
export const ALERT_THRESHOLDS_BACKUP_ORPHANED_DESCRIPTION =
  'Alert when backups exist for VMIDs that are no longer in inventory.';
export const ALERT_THRESHOLDS_BACKUP_ORPHANED_TOGGLE_LABEL = 'Alerts';
export const ALERT_THRESHOLDS_BACKUP_ORPHANED_TOGGLE_DESCRIPTION =
  'Toggle orphaned VM/Container backup alerts';
export const ALERT_THRESHOLDS_BACKUP_IGNORE_VMIDS_LABEL = 'Ignore VMIDs';
export const ALERT_THRESHOLDS_BACKUP_IGNORE_VMIDS_DESCRIPTION =
  'One per line. Use a trailing * to match a prefix (example: 10*).';
export const ALERT_THRESHOLDS_BACKUP_IGNORE_VMIDS_PLACEHOLDER = '100, 200, 10*';
export const ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_TITLE = 'Ignored container prefixes';
export const ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_DESCRIPTION =
  'Containers whose name or ID starts with any prefix below are skipped for container alerts. Enter one prefix per line; matching is case-insensitive.';
export const ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_RESET_LABEL = 'Reset';
export const ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_PLACEHOLDER = 'runner-';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_TITLE = 'Swarm service alerts';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_DESCRIPTION =
  'Pulse raises alerts when running replicas fall behind the desired count or a rollout gets stuck. Adjust the gap thresholds below or disable service alerts entirely.';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_TOGGLE_LABEL = 'Alerts';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_TOGGLE_DESCRIPTION =
  'Toggle Swarm service replica monitoring';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_WARNING_GAP_LABEL = 'Warning gap %';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_WARNING_GAP_DESCRIPTION =
  'Convert to warning when at least this percentage of replicas are missing.';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_CRITICAL_GAP_LABEL = 'Critical gap %';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_CRITICAL_GAP_DESCRIPTION =
  'Raise a critical alert when the missing replica gap meets or exceeds this value.';
export const ALERT_THRESHOLDS_DOCKER_SERVICES_GAP_VALIDATION_MESSAGE =
  'Critical gap must be greater than or equal to the warning gap when enabled.';
export const ALERT_THRESHOLDS_SECTION_TITLE_NODES = 'Proxmox Nodes';
export const ALERT_THRESHOLDS_SECTION_TITLE_PBS = 'PBS Servers';
export const ALERT_THRESHOLDS_SECTION_TITLE_GUESTS = 'VMs & Containers';
export const ALERT_THRESHOLDS_SECTION_TITLE_GUEST_FILTERING = 'Guest Filtering';
export const ALERT_THRESHOLDS_SECTION_TITLE_BACKUPS = 'Recovery';
export const ALERT_THRESHOLDS_SECTION_TITLE_SNAPSHOTS = 'Snapshot Age';
export const ALERT_THRESHOLDS_SECTION_TITLE_STORAGE = 'Storage Devices';
export const ALERT_THRESHOLDS_SECTION_TITLE_PMG = 'Mail Gateway Thresholds';
export const ALERT_THRESHOLDS_SECTION_TITLE_AGENTS = 'Agents';
export const ALERT_THRESHOLDS_SECTION_TITLE_AGENT_DISKS = 'Agent Disks';
export const ALERT_THRESHOLDS_SECTION_TITLE_DOCKER_HOSTS = 'Container Runtimes';
export const ALERT_THRESHOLDS_SECTION_TITLE_DOCKER_CONTAINERS = 'Containers';

export function getAlertThresholdsSearchPlaceholder() {
  return ALERT_THRESHOLDS_SEARCH_PLACEHOLDER;
}

export function getAlertThresholdsHelpDismissLabel() {
  return ALERT_THRESHOLDS_HELP_DISMISS_LABEL;
}

export function getAlertThresholdsHelpBanner() {
  return {
    title: 'Quick tips:',
    disableValue: '0',
    reenableLabel: 'Off',
    customBadgeLabel: 'Custom',
    collapseHint: 'Click sections to collapse/expand.',
  } as const;
}

export function getAlertThresholdsGuestFilterPresentation() {
  return {
    ignoredPrefixes: {
      title: ALERT_THRESHOLDS_GUEST_IGNORED_PREFIXES_TITLE,
      description: ALERT_THRESHOLDS_GUEST_IGNORED_PREFIXES_DESCRIPTION,
      placeholder: ALERT_THRESHOLDS_GUEST_IGNORED_PREFIXES_PLACEHOLDER,
    },
    tagWhitelist: {
      title: ALERT_THRESHOLDS_GUEST_TAG_WHITELIST_TITLE,
      description: ALERT_THRESHOLDS_GUEST_TAG_WHITELIST_DESCRIPTION,
      placeholder: ALERT_THRESHOLDS_GUEST_TAG_WHITELIST_PLACEHOLDER,
    },
    tagBlacklist: {
      title: ALERT_THRESHOLDS_GUEST_TAG_BLACKLIST_TITLE,
      description: ALERT_THRESHOLDS_GUEST_TAG_BLACKLIST_DESCRIPTION,
      placeholder: ALERT_THRESHOLDS_GUEST_TAG_BLACKLIST_PLACEHOLDER,
    },
  } as const;
}

export function getAlertThresholdsBackupOrphanedPresentation() {
  return {
    title: ALERT_THRESHOLDS_BACKUP_ORPHANED_TITLE,
    description: ALERT_THRESHOLDS_BACKUP_ORPHANED_DESCRIPTION,
    toggleLabel: ALERT_THRESHOLDS_BACKUP_ORPHANED_TOGGLE_LABEL,
    toggleDescription: ALERT_THRESHOLDS_BACKUP_ORPHANED_TOGGLE_DESCRIPTION,
    ignoreVmidsLabel: ALERT_THRESHOLDS_BACKUP_IGNORE_VMIDS_LABEL,
    ignoreVmidsDescription: ALERT_THRESHOLDS_BACKUP_IGNORE_VMIDS_DESCRIPTION,
    ignoreVmidsPlaceholder: ALERT_THRESHOLDS_BACKUP_IGNORE_VMIDS_PLACEHOLDER,
  } as const;
}

export function getAlertThresholdsDockerIgnoredPrefixesPresentation() {
  return {
    title: ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_TITLE,
    description: ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_DESCRIPTION,
    resetLabel: ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_RESET_LABEL,
    placeholder: ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_PLACEHOLDER,
  } as const;
}

export function getAlertThresholdsDockerServicePresentation() {
  return {
    title: ALERT_THRESHOLDS_DOCKER_SERVICES_TITLE,
    description: ALERT_THRESHOLDS_DOCKER_SERVICES_DESCRIPTION,
    toggleLabel: ALERT_THRESHOLDS_DOCKER_SERVICES_TOGGLE_LABEL,
    toggleDescription: ALERT_THRESHOLDS_DOCKER_SERVICES_TOGGLE_DESCRIPTION,
    warningGapLabel: ALERT_THRESHOLDS_DOCKER_SERVICES_WARNING_GAP_LABEL,
    warningGapDescription: ALERT_THRESHOLDS_DOCKER_SERVICES_WARNING_GAP_DESCRIPTION,
    criticalGapLabel: ALERT_THRESHOLDS_DOCKER_SERVICES_CRITICAL_GAP_LABEL,
    criticalGapDescription: ALERT_THRESHOLDS_DOCKER_SERVICES_CRITICAL_GAP_DESCRIPTION,
    gapValidationMessage: ALERT_THRESHOLDS_DOCKER_SERVICES_GAP_VALIDATION_MESSAGE,
  } as const;
}

export function getAlertThresholdsSectionTitles() {
  return {
    nodes: ALERT_THRESHOLDS_SECTION_TITLE_NODES,
    pbs: ALERT_THRESHOLDS_SECTION_TITLE_PBS,
    guests: ALERT_THRESHOLDS_SECTION_TITLE_GUESTS,
    guestFiltering: ALERT_THRESHOLDS_SECTION_TITLE_GUEST_FILTERING,
    backups: ALERT_THRESHOLDS_SECTION_TITLE_BACKUPS,
    snapshots: ALERT_THRESHOLDS_SECTION_TITLE_SNAPSHOTS,
    storage: ALERT_THRESHOLDS_SECTION_TITLE_STORAGE,
    pmg: ALERT_THRESHOLDS_SECTION_TITLE_PMG,
    agents: ALERT_THRESHOLDS_SECTION_TITLE_AGENTS,
    agentDisks: ALERT_THRESHOLDS_SECTION_TITLE_AGENT_DISKS,
    dockerHosts: ALERT_THRESHOLDS_SECTION_TITLE_DOCKER_HOSTS,
    dockerContainers: ALERT_THRESHOLDS_SECTION_TITLE_DOCKER_CONTAINERS,
  } as const;
}
