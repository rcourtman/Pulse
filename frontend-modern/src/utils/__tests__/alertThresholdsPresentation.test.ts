import { describe, expect, it } from 'vitest';
import {
  AGENT_DISKS_EMPTY_STATE,
  AGENT_DISKS_FILTER_EMPTY_STATE,
  AGENT_THRESHOLDS_FILTER_EMPTY_STATE,
  ALERT_THRESHOLDS_BACKUP_IGNORE_VMIDS_PLACEHOLDER,
  ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_PLACEHOLDER,
  ALERT_THRESHOLDS_DOCKER_SERVICES_TITLE,
  ALERT_THRESHOLDS_DOCKER_SERVICES_DESCRIPTION,
  ALERT_THRESHOLDS_DOCKER_SERVICES_TOGGLE_LABEL,
  ALERT_THRESHOLDS_DOCKER_SERVICES_TOGGLE_DESCRIPTION,
  ALERT_THRESHOLDS_DOCKER_SERVICES_WARNING_GAP_LABEL,
  ALERT_THRESHOLDS_DOCKER_SERVICES_WARNING_GAP_DESCRIPTION,
  ALERT_THRESHOLDS_DOCKER_SERVICES_CRITICAL_GAP_LABEL,
  ALERT_THRESHOLDS_DOCKER_SERVICES_CRITICAL_GAP_DESCRIPTION,
  ALERT_THRESHOLDS_DOCKER_SERVICES_GAP_VALIDATION_MESSAGE,
  ALERT_THRESHOLDS_GUEST_IGNORED_PREFIXES_PLACEHOLDER,
  ALERT_THRESHOLDS_GUEST_TAG_BLACKLIST_PLACEHOLDER,
  ALERT_THRESHOLDS_GUEST_TAG_WHITELIST_PLACEHOLDER,
  ALERT_THRESHOLDS_HELP_DISMISS_LABEL,
  ALERT_THRESHOLDS_SEARCH_PLACEHOLDER,
  ALERT_THRESHOLDS_SECTION_TITLE_AGENT_DISKS,
  ALERT_THRESHOLDS_SECTION_TITLE_AGENTS,
  ALERT_THRESHOLDS_SECTION_TITLE_BACKUPS,
  ALERT_THRESHOLDS_SECTION_TITLE_DOCKER_CONTAINERS,
  ALERT_THRESHOLDS_SECTION_TITLE_DOCKER_HOSTS,
  ALERT_THRESHOLDS_SECTION_TITLE_GUESTS,
  ALERT_THRESHOLDS_SECTION_TITLE_GUEST_FILTERING,
  ALERT_THRESHOLDS_SECTION_TITLE_NODES,
  ALERT_THRESHOLDS_SECTION_TITLE_PBS,
  ALERT_THRESHOLDS_SECTION_TITLE_PMG,
  ALERT_THRESHOLDS_SECTION_TITLE_SNAPSHOTS,
  ALERT_THRESHOLDS_SECTION_TITLE_STORAGE,
  BACKUP_THRESHOLDS_EMPTY_STATE,
  CONTAINERS_FILTER_EMPTY_STATE,
  CONTAINER_RUNTIMES_FILTER_EMPTY_STATE,
  GUEST_THRESHOLDS_EMPTY_STATE,
  GUEST_THRESHOLDS_FILTER_EMPTY_STATE,
  GUEST_FILTERING_EMPTY_STATE,
  NODE_THRESHOLDS_FILTER_EMPTY_STATE,
  PBS_THRESHOLDS_EMPTY_STATE,
  PBS_THRESHOLDS_FILTER_EMPTY_STATE,
  PMG_THRESHOLDS_EMPTY_STATE,
  PMG_THRESHOLDS_FILTER_EMPTY_STATE,
  SNAPSHOT_THRESHOLDS_EMPTY_STATE,
  STORAGE_THRESHOLDS_EMPTY_STATE,
  STORAGE_THRESHOLDS_FILTER_EMPTY_STATE,
  getAlertThresholdsBackupOrphanedPresentation,
  getAlertThresholdsDockerIgnoredPrefixesPresentation,
  getAlertThresholdsDockerServicePresentation,
  getAlertThresholdsGuestFilterPresentation,
  getAlertThresholdsHelpBanner,
  getAlertThresholdsHelpDismissLabel,
  getAlertThresholdsSearchPlaceholder,
  getAlertThresholdsSectionTitles,
} from '../alertThresholdsPresentation';

describe('alertThresholdsPresentation', () => {
  it('exports canonical thresholds empty-state copy', () => {
    expect(PBS_THRESHOLDS_EMPTY_STATE).toBe('No PBS servers configured.');
    expect(GUEST_THRESHOLDS_EMPTY_STATE).toBe('No VMs or containers found.');
    expect(NODE_THRESHOLDS_FILTER_EMPTY_STATE).toBe('No nodes match the current filters.');
    expect(PBS_THRESHOLDS_FILTER_EMPTY_STATE).toBe('No PBS servers match the current filters.');
    expect(GUEST_THRESHOLDS_FILTER_EMPTY_STATE).toBe('No VMs or containers match the current filters.');
    expect(GUEST_FILTERING_EMPTY_STATE).toBe('Configure guest filtering rules.');
    expect(BACKUP_THRESHOLDS_EMPTY_STATE).toBe('Configure recovery alert thresholds.');
    expect(SNAPSHOT_THRESHOLDS_EMPTY_STATE).toBe('Configure snapshot age thresholds.');
    expect(STORAGE_THRESHOLDS_EMPTY_STATE).toBe('No storage devices found.');
    expect(STORAGE_THRESHOLDS_FILTER_EMPTY_STATE).toBe('No storage devices match the current filters.');
    expect(PMG_THRESHOLDS_EMPTY_STATE).toContain('No mail gateways configured yet.');
    expect(PMG_THRESHOLDS_FILTER_EMPTY_STATE).toBe('No mail gateways match the current filters.');
    expect(AGENT_THRESHOLDS_FILTER_EMPTY_STATE).toBe('No agents match the current filters.');
    expect(AGENT_DISKS_EMPTY_STATE).toContain('Agents with mounted filesystems will appear here.');
    expect(AGENT_DISKS_FILTER_EMPTY_STATE).toBe('No agent disks match the current filters.');
    expect(CONTAINER_RUNTIMES_FILTER_EMPTY_STATE).toBe('No container runtimes match the current filters.');
    expect(CONTAINERS_FILTER_EMPTY_STATE).toBe('No containers match the current filters.');
  });

  it('exports canonical thresholds search and help-banner copy', () => {
    expect(ALERT_THRESHOLDS_SEARCH_PLACEHOLDER).toBe('Search resources...');
    expect(getAlertThresholdsSearchPlaceholder()).toBe('Search resources...');
    expect(ALERT_THRESHOLDS_HELP_DISMISS_LABEL).toBe('Dismiss tips');
    expect(getAlertThresholdsHelpDismissLabel()).toBe('Dismiss tips');
    expect(getAlertThresholdsHelpBanner()).toEqual({
      title: 'Quick tips:',
      disableValue: '0',
      reenableLabel: 'Off',
      customBadgeLabel: 'Custom',
      collapseHint: 'Click sections to collapse/expand.',
    });
  });

  it('exports canonical thresholds auxiliary filter and backup vocabulary', () => {
    expect(ALERT_THRESHOLDS_GUEST_IGNORED_PREFIXES_PLACEHOLDER).toBe('dev-');
    expect(ALERT_THRESHOLDS_GUEST_TAG_WHITELIST_PLACEHOLDER).toBe('production');
    expect(ALERT_THRESHOLDS_GUEST_TAG_BLACKLIST_PLACEHOLDER).toBe('maintenance');
    expect(ALERT_THRESHOLDS_BACKUP_IGNORE_VMIDS_PLACEHOLDER).toBe('100, 200, 10*');
    expect(ALERT_THRESHOLDS_DOCKER_IGNORED_PREFIXES_PLACEHOLDER).toBe('runner-');
    expect(getAlertThresholdsGuestFilterPresentation()).toEqual({
      ignoredPrefixes: {
        title: 'Ignored Prefixes',
        description: 'Skip metrics for guests starting with:',
        placeholder: 'dev-',
      },
      tagWhitelist: {
        title: 'Tag Whitelist',
        description:
          'Only monitor guests with at least one of these tags (leave empty to disable whitelist):',
        placeholder: 'production',
      },
      tagBlacklist: {
        title: 'Tag Blacklist',
        description: 'Ignore guests with any of these tags:',
        placeholder: 'maintenance',
      },
    });
    expect(getAlertThresholdsBackupOrphanedPresentation()).toEqual({
      title: 'Orphaned backups',
      description: 'Alert when backups exist for VMIDs that are no longer in inventory.',
      toggleLabel: 'Alerts',
      toggleDescription: 'Toggle orphaned VM/Container backup alerts',
      ignoreVmidsLabel: 'Ignore VMIDs',
      ignoreVmidsDescription: 'One per line. Use a trailing * to match a prefix (example: 10*).',
      ignoreVmidsPlaceholder: '100, 200, 10*',
    });
    expect(getAlertThresholdsDockerIgnoredPrefixesPresentation()).toEqual({
      title: 'Ignored container prefixes',
      description:
        'Containers whose name or ID starts with any prefix below are skipped for container alerts. Enter one prefix per line; matching is case-insensitive.',
      resetLabel: 'Reset',
      placeholder: 'runner-',
    });
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_TITLE).toBe('Swarm service alerts');
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_DESCRIPTION).toContain(
      'running replicas fall behind the desired count',
    );
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_TOGGLE_LABEL).toBe('Alerts');
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_TOGGLE_DESCRIPTION).toBe(
      'Toggle Swarm service replica monitoring',
    );
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_WARNING_GAP_LABEL).toBe('Warning gap %');
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_WARNING_GAP_DESCRIPTION).toBe(
      'Convert to warning when at least this percentage of replicas are missing.',
    );
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_CRITICAL_GAP_LABEL).toBe('Critical gap %');
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_CRITICAL_GAP_DESCRIPTION).toBe(
      'Raise a critical alert when the missing replica gap meets or exceeds this value.',
    );
    expect(ALERT_THRESHOLDS_DOCKER_SERVICES_GAP_VALIDATION_MESSAGE).toBe(
      'Critical gap must be greater than or equal to the warning gap when enabled.',
    );
    expect(getAlertThresholdsDockerServicePresentation()).toEqual({
      title: 'Swarm service alerts',
      description:
        'Pulse raises alerts when running replicas fall behind the desired count or a rollout gets stuck. Adjust the gap thresholds below or disable service alerts entirely.',
      toggleLabel: 'Alerts',
      toggleDescription: 'Toggle Swarm service replica monitoring',
      warningGapLabel: 'Warning gap %',
      warningGapDescription:
        'Convert to warning when at least this percentage of replicas are missing.',
      criticalGapLabel: 'Critical gap %',
      criticalGapDescription:
        'Raise a critical alert when the missing replica gap meets or exceeds this value.',
      gapValidationMessage:
        'Critical gap must be greater than or equal to the warning gap when enabled.',
    });
  });

  it('exports canonical thresholds section titles', () => {
    expect(ALERT_THRESHOLDS_SECTION_TITLE_NODES).toBe('Proxmox Nodes');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_PBS).toBe('PBS Servers');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_GUESTS).toBe('VMs & Containers');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_GUEST_FILTERING).toBe('Guest Filtering');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_BACKUPS).toBe('Recovery');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_SNAPSHOTS).toBe('Snapshot Age');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_STORAGE).toBe('Storage Devices');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_PMG).toBe('Mail Gateway Thresholds');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_AGENTS).toBe('Agents');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_AGENT_DISKS).toBe('Agent Disks');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_DOCKER_HOSTS).toBe('Container Runtimes');
    expect(ALERT_THRESHOLDS_SECTION_TITLE_DOCKER_CONTAINERS).toBe('Containers');
    expect(getAlertThresholdsSectionTitles()).toEqual({
      nodes: 'Proxmox Nodes',
      pbs: 'PBS Servers',
      guests: 'VMs & Containers',
      guestFiltering: 'Guest Filtering',
      backups: 'Recovery',
      snapshots: 'Snapshot Age',
      storage: 'Storage Devices',
      pmg: 'Mail Gateway Thresholds',
      agents: 'Agents',
      agentDisks: 'Agent Disks',
      dockerHosts: 'Container Runtimes',
      dockerContainers: 'Containers',
    });
  });
});
