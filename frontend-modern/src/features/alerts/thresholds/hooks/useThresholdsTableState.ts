import { createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import type { PMGThresholdDefaults, SnapshotAlertConfig, BackupAlertConfig } from '@/types/alerts';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { logger } from '@/utils/logger';
import {
  AGENT_DISKS_EMPTY_STATE,
  AGENT_DISKS_FILTER_EMPTY_STATE,
  AGENT_THRESHOLDS_FILTER_EMPTY_STATE,
  getAlertThresholdsHelpBanner,
  getAlertThresholdsGuestFilterPresentation,
  getAlertThresholdsHelpDismissLabel,
  getAlertThresholdsBackupOrphanedPresentation,
  getAlertThresholdsDockerServicePresentation,
  getAlertThresholdsDockerIgnoredPrefixesPresentation,
  getAlertThresholdsSearchPlaceholder,
  getAlertThresholdsSectionTitles,
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
} from '@/utils/alertThresholdsPresentation';
import { PMG_THRESHOLD_COLUMNS } from '@/features/alerts/thresholds/constants';
import { normalizeDockerIgnoredInput } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import type {
  Resource as TableResource,
  ThresholdsActiveTab,
} from '@/features/alerts/thresholds/tableTypes';
import { useCollapsedSections } from '@/components/Alerts/Thresholds/hooks/useCollapsedSections';
import { useThresholdsAvailabilityMutations } from './useThresholdsAvailabilityMutations';
import { useThresholdsData } from './useThresholdsData';
import { useThresholdsRecoveryDefaultsState } from './useThresholdsRecoveryDefaultsState';
import { useThresholdsOverrideMutations } from './useThresholdsOverrideMutations';

const HELP_BANNER_KEY = 'pulse-thresholds-help-dismissed';

interface ThresholdsSummaryItem {
  key:
    | 'agentDisks'
    | 'agents'
    | 'backups'
    | 'dockerContainers'
    | 'dockerHosts'
    | 'guests'
    | 'kubernetesClusters'
    | 'kubernetesDeployments'
    | 'kubernetesNamespaces'
    | 'kubernetesNodes'
    | 'kubernetesPods'
    | 'nodes'
    | 'pbs'
    | 'pmg'
    | 'snapshots'
    | 'storage'
    | 'trueNASDatasets'
    | 'trueNASDisks'
    | 'trueNASPools'
    | 'trueNASSystems'
    | 'vmwareDatastores'
    | 'vmwareHosts'
    | 'vmwareNetworks'
    | 'vmwareVMs';
  label: string;
  overrides: number;
  tab: ThresholdsActiveTab;
  total: number;
}

export function useThresholdsTableState(props: ThresholdsTableProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const sectionTitles = getAlertThresholdsSectionTitles();
  const guestFilterPresentation = getAlertThresholdsGuestFilterPresentation();
  const backupOrphanedPresentation = getAlertThresholdsBackupOrphanedPresentation();
  const dockerServicePresentation = getAlertThresholdsDockerServicePresentation();
  const dockerIgnoredPrefixesPresentation = getAlertThresholdsDockerIgnoredPrefixesPresentation();

  const { isCollapsed, toggleSection, setCollapsed, expandAll, collapseAll } =
    useCollapsedSections();

  const [helpBannerDismissed, setHelpBannerDismissed] = createSignal(
    typeof window !== 'undefined' && localStorage.getItem(HELP_BANNER_KEY) === 'true',
  );

  const dismissHelpBanner = () => {
    setHelpBannerDismissed(true);
    if (typeof window !== 'undefined') {
      localStorage.setItem(HELP_BANNER_KEY, 'true');
    }
  };

  const [searchTerm, setSearchTerm] = createSignal('');
  const [overrideFilter, setOverrideFilter] = createSignal('all');
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [editingThresholds, setEditingThresholds] = createSignal<
    Record<string, number | undefined>
  >({});
  const [editingNote, setEditingNote] = createSignal('');
  const [bulkEditIds, setBulkEditIds] = createSignal<string[]>([]);
  const [bulkEditColumns, setBulkEditColumns] = createSignal<string[]>([]);
  const [isBulkEditDialogOpen, setIsBulkEditDialogOpen] = createSignal(false);
  const [activeTab, setActiveTab] = createSignal<ThresholdsActiveTab>('proxmox');
  const [dockerIgnoredInput, setDockerIgnoredInput] = createSignal(
    props.dockerIgnoredPrefixes().join('\n'),
  );

  const serviceWarnInputId = 'docker-service-warn-gap';
  const serviceCriticalInputId = 'docker-service-critical-gap';

  createEffect(() => {
    const remote = props.dockerIgnoredPrefixes();
    const local = dockerIgnoredInput();
    const normalizedLocal = normalizeDockerIgnoredInput(local);
    const isSynced =
      remote.length === normalizedLocal.length &&
      remote.every((value, index) => value === normalizedLocal[index]);

    if (!isSynced) {
      setDockerIgnoredInput(remote.join('\n'));
    }
  });

  const serviceGapValidationMessage = createMemo(() => {
    const warn = Number(props.dockerDefaults.serviceWarnGapPercent ?? 0);
    const crit = Number(props.dockerDefaults.serviceCriticalGapPercent ?? 0);
    if (crit > 0 && warn > crit) {
      return dockerServicePresentation.gapValidationMessage;
    }
    return '';
  });

  const hasDockerSpecificControls = createMemo(() => (props.dockerHosts?.length ?? 0) > 0);

  const getActiveTabFromRoute = (): ThresholdsActiveTab => {
    const path = location.pathname;
    if (path.includes('/thresholds/containers') || path.includes('/thresholds/docker'))
      return 'docker';
    if (path.includes('/thresholds/kubernetes')) return 'kubernetes';
    if (path.includes('/thresholds/truenas')) return 'truenas';
    if (path.includes('/thresholds/vmware') || path.includes('/thresholds/vsphere'))
      return 'vmware';
    if (path.includes('/thresholds/systems') || path.includes('/thresholds/agents'))
      return 'systems';
    // PBS and PMG are Proxmox sections, not peer tabs. Legacy section
    // deep-links resolve to the Proxmox tab (the redirect effect rewrites
    // them to `/proxmox#pbs` / `/proxmox#pmg` so the section scrolls into view).
    return 'proxmox';
  };

  createEffect(() => {
    const tabFromRoute = getActiveTabFromRoute();
    if (activeTab() !== tabFromRoute) {
      setActiveTab(tabFromRoute);
    }
  });

  createEffect(() => {
    if (location.pathname === '/alerts/thresholds') {
      navigate('/alerts/thresholds/proxmox', { replace: true });
      return;
    }

    if (location.pathname === '/alerts/thresholds/infrastructure') {
      navigate('/alerts/thresholds/proxmox', { replace: true });
      return;
    }

    if (location.pathname === '/alerts/thresholds/containers') {
      navigate('/alerts/thresholds/docker', { replace: true });
      return;
    }

    if (
      location.pathname === '/alerts/thresholds/mail-gateway' ||
      location.pathname === '/alerts/thresholds/pmg'
    ) {
      navigate('/alerts/thresholds/proxmox#pmg', { replace: true });
      return;
    }

    if (location.pathname === '/alerts/thresholds/pbs') {
      navigate('/alerts/thresholds/proxmox#pbs', { replace: true });
      return;
    }

    if (location.pathname === '/alerts/thresholds/agents') {
      navigate('/alerts/thresholds/systems', { replace: true });
    }
  });

  const handleTabClick = (tab: ThresholdsActiveTab) => {
    const tabRoutes: Record<ThresholdsActiveTab, string> = {
      proxmox: '/alerts/thresholds/proxmox',
      docker: '/alerts/thresholds/docker',
      kubernetes: '/alerts/thresholds/kubernetes',
      truenas: '/alerts/thresholds/truenas',
      vmware: '/alerts/thresholds/vmware',
      systems: '/alerts/thresholds/systems',
    };
    navigate(tabRoutes[tab]);
  };

  const handleDockerIgnoredChange = (value: string) => {
    setDockerIgnoredInput(value);
    props.setDockerIgnoredPrefixes(normalizeDockerIgnoredInput(value));
    props.setHasUnsavedChanges(true);
  };

  const handleResetDockerIgnored = () => {
    if (props.resetDockerIgnoredPrefixes) {
      props.resetDockerIgnoredPrefixes();
    } else {
      props.setDockerIgnoredPrefixes([]);
    }
    setDockerIgnoredInput('');
    props.setHasUnsavedChanges(true);
  };

  const hasActiveAlert = (resourceId: string, metric: string): boolean => {
    if (!alertsEnabled()) return false;
    if (!props.activeAlerts) return false;
    return `${resourceId}-${metric}` in props.activeAlerts;
  };

  const {
    nodesWithOverrides: rawNodesWithOverrides,
    agentsWithOverrides: rawAgentsWithOverrides,
    agentDisksWithOverrides: rawAgentDisksWithOverrides,
    agentDisksGroupedByAgent: rawAgentDisksGroupedByAgent,
    agentGroupHeaderMeta,
    dockerHostsWithOverrides: rawDockerHostsWithOverrides,
    dockerContainersGroupedByHost: rawDockerContainersGroupedByHost,
    dockerContainersFlat: rawDockerContainersFlat,
    totalDockerContainers,
    dockerHostGroupMeta,
    guestsGroupedByNode: rawGuestsGroupedByNode,
    guestsFlat: rawGuestsFlat,
    guestGroupHeaderMeta,
    kubernetesClustersWithOverrides: rawKubernetesClustersWithOverrides = () => [],
    kubernetesDeploymentsWithOverrides: rawKubernetesDeploymentsWithOverrides = () => [],
    kubernetesNamespacesWithOverrides: rawKubernetesNamespacesWithOverrides = () => [],
    kubernetesNodesWithOverrides: rawKubernetesNodesWithOverrides = () => [],
    kubernetesPodsWithOverrides: rawKubernetesPodsWithOverrides = () => [],
    pbsServersWithOverrides: rawPbsServersWithOverrides,
    pmgGlobalDefaults,
    pmgServersWithOverrides: rawPmgServersWithOverrides,
    storageWithOverrides: rawStorageWithOverrides,
    storageGroupedByNode: rawStorageGroupedByNode,
    trueNASDatasetsWithOverrides: rawTrueNASDatasetsWithOverrides = () => [],
    trueNASDisksWithOverrides: rawTrueNASDisksWithOverrides = () => [],
    trueNASPoolsWithOverrides: rawTrueNASPoolsWithOverrides = () => [],
    trueNASSystemsWithOverrides: rawTrueNASSystemsWithOverrides = () => [],
    vmwareDatastoresWithOverrides: rawVmwareDatastoresWithOverrides = () => [],
    vmwareHostsWithOverrides: rawVmwareHostsWithOverrides = () => [],
    vmwareNetworksWithOverrides: rawVmwareNetworksWithOverrides = () => [],
    vmwareVMsWithOverrides: rawVmwareVMsWithOverrides = () => [],
  } = useThresholdsData(props, editingId, searchTerm);

  const resourceMatchesOverrideFilter = (resource: TableResource): boolean => {
    const filter = overrideFilter();
    if (filter === 'custom') {
      return Boolean(
        resource.hasOverride || resource.disableConnectivity || resource.poweredOffSeverity,
      );
    }
    if (filter === 'disabled') {
      return Boolean(resource.disabled || resource.disableConnectivity);
    }
    return true;
  };

  const filterResources = (resources: TableResource[]): TableResource[] =>
    resources.filter(resourceMatchesOverrideFilter);

  const filterGroupedResources = (
    groups: Record<string, TableResource[]>,
  ): Record<string, TableResource[]> => {
    const filtered: Record<string, TableResource[]> = {};
    Object.entries(groups).forEach(([groupKey, resources]) => {
      const visibleResources = filterResources(resources);
      if (visibleResources.length > 0) {
        filtered[groupKey] = visibleResources;
      }
    });
    return filtered;
  };

  const nodesWithOverrides = createMemo(() => filterResources(rawNodesWithOverrides()));
  const agentsWithOverrides = createMemo(() => filterResources(rawAgentsWithOverrides()));
  const agentDisksWithOverrides = createMemo(() => filterResources(rawAgentDisksWithOverrides()));
  const agentDisksGroupedByAgent = createMemo(() =>
    filterGroupedResources(rawAgentDisksGroupedByAgent()),
  );
  const dockerHostsWithOverrides = createMemo(() => filterResources(rawDockerHostsWithOverrides()));
  const dockerContainersGroupedByHost = createMemo(() =>
    filterGroupedResources(rawDockerContainersGroupedByHost()),
  );
  const dockerContainersFlat = createMemo(() => filterResources(rawDockerContainersFlat()));
  const guestsGroupedByNode = createMemo(() => filterGroupedResources(rawGuestsGroupedByNode()));
  const guestsFlat = createMemo(() => filterResources(rawGuestsFlat()));
  const kubernetesClustersWithOverrides = createMemo(() =>
    filterResources(rawKubernetesClustersWithOverrides()),
  );
  const kubernetesDeploymentsWithOverrides = createMemo(() =>
    filterResources(rawKubernetesDeploymentsWithOverrides()),
  );
  const kubernetesNamespacesWithOverrides = createMemo(() =>
    filterResources(rawKubernetesNamespacesWithOverrides()),
  );
  const kubernetesNodesWithOverrides = createMemo(() =>
    filterResources(rawKubernetesNodesWithOverrides()),
  );
  const kubernetesPodsWithOverrides = createMemo(() =>
    filterResources(rawKubernetesPodsWithOverrides()),
  );
  const pbsServersWithOverrides = createMemo(() => filterResources(rawPbsServersWithOverrides()));
  const pmgServersWithOverrides = createMemo(() => filterResources(rawPmgServersWithOverrides()));
  const storageWithOverrides = createMemo(() => filterResources(rawStorageWithOverrides()));
  const storageGroupedByNode = createMemo(() => filterGroupedResources(rawStorageGroupedByNode()));
  const trueNASDatasetsWithOverrides = createMemo(() =>
    filterResources(rawTrueNASDatasetsWithOverrides()),
  );
  const trueNASDisksWithOverrides = createMemo(() =>
    filterResources(rawTrueNASDisksWithOverrides()),
  );
  const trueNASPoolsWithOverrides = createMemo(() =>
    filterResources(rawTrueNASPoolsWithOverrides()),
  );
  const trueNASSystemsWithOverrides = createMemo(() =>
    filterResources(rawTrueNASSystemsWithOverrides()),
  );
  const vmwareDatastoresWithOverrides = createMemo(() =>
    filterResources(rawVmwareDatastoresWithOverrides()),
  );
  const vmwareHostsWithOverrides = createMemo(() => filterResources(rawVmwareHostsWithOverrides()));
  const vmwareNetworksWithOverrides = createMemo(() =>
    filterResources(rawVmwareNetworksWithOverrides()),
  );
  const vmwareVMsWithOverrides = createMemo(() => filterResources(rawVmwareVMsWithOverrides()));

  // Deep-links to a Proxmox sub-section (PBS / PMG) arrive as `#pbs` / `#pmg`.
  // Expand the target section and scroll it into view. The section's anchor only
  // mounts once its table rows resolve, so the effect keys off the registered
  // section element (set by the section's `ref`): it re-runs the moment the
  // anchor mounts, whether the data was warm at navigation or arrived later. We
  // scroll the element directly rather than via requestAnimationFrame, which is
  // suspended while the tab is hidden.
  const [sectionElements, setSectionElements] = createSignal<Record<string, HTMLElement | null>>(
    {},
  );
  const SECTION_HASH_IDS = new Set(['pbs', 'pmg']);
  const [lastSectionHashScrolled, setLastSectionHashScrolled] = createSignal<string | null>(null);

  createEffect(() => {
    const hash = location.hash ?? '';
    const sectionId = hash.startsWith('#') ? hash.slice(1) : '';
    const target = sectionElements()[sectionId];

    if (activeTab() !== 'proxmox' || !SECTION_HASH_IDS.has(sectionId)) {
      if (!SECTION_HASH_IDS.has(sectionId)) {
        setLastSectionHashScrolled(null);
      }
      return;
    }

    if (hash === lastSectionHashScrolled()) {
      return;
    }

    setCollapsed(sectionId, false);

    if (!target) {
      return;
    }
    target.scrollIntoView({ block: 'start' });
    setLastSectionHashScrolled(hash);
  });

  const {
    backupDefaultsRecord,
    backupFactoryConfig,
    backupFactoryDefaultsRecord,
    backupOverridesCount,
    sanitizeBackupConfig,
    sanitizeSnapshotConfig,
    snapshotDefaultsRecord,
    snapshotFactoryConfig,
    snapshotFactoryDefaultsRecord,
    snapshotOverridesCount,
  } = useThresholdsRecoveryDefaultsState(props);

  const countOverrides = (resources: TableResource[] | undefined) =>
    resources?.filter(
      (resource) => resource.hasOverride || resource.disabled || resource.disableConnectivity,
    ).length ?? 0;

  const registerSection = (key: string) => (element: HTMLDivElement | null) => {
    setSectionElements((prev) => {
      if (prev[key] === element) {
        return prev;
      }
      return { ...prev, [key]: element };
    });
  };

  const updateSnapshotDefaults = (
    updater: SnapshotAlertConfig | ((prev: SnapshotAlertConfig) => SnapshotAlertConfig),
  ) => {
    props.setSnapshotDefaults((prev) => {
      const next =
        typeof updater === 'function'
          ? (updater as (prev: SnapshotAlertConfig) => SnapshotAlertConfig)(prev)
          : { ...prev, ...updater };
      return sanitizeSnapshotConfig(next);
    });
    props.setHasUnsavedChanges(true);
  };

  const updateBackupDefaults = (
    updater: BackupAlertConfig | ((prev: BackupAlertConfig) => BackupAlertConfig),
  ) => {
    props.setBackupDefaults((prev) => {
      const next =
        typeof updater === 'function'
          ? (updater as (prev: BackupAlertConfig) => BackupAlertConfig)(prev)
          : { ...prev, ...updater };
      return sanitizeBackupConfig(next);
    });
    props.setHasUnsavedChanges(true);
  };

  const setPMGGlobalDefaults = (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => {
    const current = pmgGlobalDefaults();
    const nextRecord =
      typeof value === 'function' ? value({ ...current }) : { ...current, ...value };

    let changed = false;
    props.setPMGThresholds((prev: PMGThresholdDefaults) => {
      const updated: PMGThresholdDefaults = { ...prev };
      PMG_THRESHOLD_COLUMNS.forEach(({ key, normalized }) => {
        const raw = nextRecord[normalized];
        if (typeof raw === 'number' && !Number.isNaN(raw)) {
          const sanitized = Math.max(0, Math.round(raw));
          if (updated[key] !== sanitized) {
            updated[key] = sanitized;
            changed = true;
          }
        }
      });
      return updated;
    });

    if (changed) {
      props.setHasUnsavedChanges(true);
    }
  };

  const summaryItems = createMemo<ThresholdsSummaryItem[]>(() => {
    try {
      const items: ThresholdsSummaryItem[] = [
        {
          key: 'nodes',
          label: 'Virtualization Hosts',
          overrides: countOverrides(nodesWithOverrides()),
          tab: 'proxmox',
          total: props.nodes?.length ?? 0,
        },
        {
          key: 'dockerHosts',
          label: 'Container Runtimes',
          overrides: countOverrides(dockerHostsWithOverrides()),
          tab: 'docker',
          total: props.containerRuntimes?.length ?? 0,
        },
        {
          key: 'agents',
          label: 'Machines',
          overrides: countOverrides(agentsWithOverrides()),
          tab: 'systems',
          total: props.agents?.length ?? 0,
        },
        {
          key: 'agentDisks',
          label: 'Machine Disks',
          overrides: countOverrides(agentDisksWithOverrides()),
          tab: 'systems',
          total: agentDisksWithOverrides().length,
        },
        {
          key: 'storage',
          label: 'Storage',
          overrides: countOverrides(storageWithOverrides()),
          tab: 'proxmox',
          total: props.storage?.length ?? 0,
        },
        {
          key: 'backups',
          label: 'Recovery',
          overrides: backupOverridesCount(),
          tab: 'proxmox',
          total: 1,
        },
        {
          key: 'snapshots',
          label: 'Snapshot Age',
          overrides: snapshotOverridesCount(),
          tab: 'proxmox',
          total: 1,
        },
        {
          key: 'pbs',
          label: 'PBS Servers',
          overrides: countOverrides(pbsServersWithOverrides()),
          tab: 'proxmox',
          total: props.pbsInstances?.length ?? 0,
        },
        {
          key: 'pmg',
          label: 'Mail Gateways',
          overrides: countOverrides(pmgServersWithOverrides()),
          tab: 'proxmox',
          total: props.pmgInstances?.length ?? 0,
        },
        {
          key: 'dockerContainers',
          label: 'Containers',
          overrides: countOverrides(dockerContainersFlat()),
          tab: 'docker',
          total: totalDockerContainers() ?? 0,
        },
        {
          key: 'guests',
          label: 'VMs & Containers',
          overrides: countOverrides(guestsFlat()),
          tab: 'proxmox',
          total: props.allGuests?.()?.length ?? 0,
        },
        {
          key: 'kubernetesClusters',
          label: 'Kubernetes Clusters',
          overrides: countOverrides(kubernetesClustersWithOverrides()),
          tab: 'kubernetes',
          total: kubernetesClustersWithOverrides().length,
        },
        {
          key: 'kubernetesNodes',
          label: 'Kubernetes Nodes',
          overrides: countOverrides(kubernetesNodesWithOverrides()),
          tab: 'kubernetes',
          total: kubernetesNodesWithOverrides().length,
        },
        {
          key: 'kubernetesNamespaces',
          label: 'Namespaces',
          overrides: countOverrides(kubernetesNamespacesWithOverrides()),
          tab: 'kubernetes',
          total: kubernetesNamespacesWithOverrides().length,
        },
        {
          key: 'kubernetesDeployments',
          label: 'Deployments',
          overrides: countOverrides(kubernetesDeploymentsWithOverrides()),
          tab: 'kubernetes',
          total: kubernetesDeploymentsWithOverrides().length,
        },
        {
          key: 'kubernetesPods',
          label: 'Pods',
          overrides: countOverrides(kubernetesPodsWithOverrides()),
          tab: 'kubernetes',
          total: kubernetesPodsWithOverrides().length,
        },
        {
          key: 'trueNASSystems',
          label: 'TrueNAS Systems',
          overrides: countOverrides(trueNASSystemsWithOverrides()),
          tab: 'truenas',
          total: trueNASSystemsWithOverrides().length,
        },
        {
          key: 'trueNASPools',
          label: 'Pools',
          overrides: countOverrides(trueNASPoolsWithOverrides()),
          tab: 'truenas',
          total: trueNASPoolsWithOverrides().length,
        },
        {
          key: 'trueNASDatasets',
          label: 'Datasets',
          overrides: countOverrides(trueNASDatasetsWithOverrides()),
          tab: 'truenas',
          total: trueNASDatasetsWithOverrides().length,
        },
        {
          key: 'trueNASDisks',
          label: 'Disks',
          overrides: countOverrides(trueNASDisksWithOverrides()),
          tab: 'truenas',
          total: trueNASDisksWithOverrides().length,
        },
        {
          key: 'vmwareHosts',
          label: 'vSphere Hosts',
          overrides: countOverrides(vmwareHostsWithOverrides()),
          tab: 'vmware',
          total: vmwareHostsWithOverrides().length,
        },
        {
          key: 'vmwareVMs',
          label: 'vSphere VMs',
          overrides: countOverrides(vmwareVMsWithOverrides()),
          tab: 'vmware',
          total: vmwareVMsWithOverrides().length,
        },
        {
          key: 'vmwareDatastores',
          label: 'Datastores',
          overrides: countOverrides(vmwareDatastoresWithOverrides()),
          tab: 'vmware',
          total: vmwareDatastoresWithOverrides().length,
        },
        {
          key: 'vmwareNetworks',
          label: 'Networks',
          overrides: countOverrides(vmwareNetworksWithOverrides()),
          tab: 'vmware',
          total: vmwareNetworksWithOverrides().length,
        },
      ];

      return items
        .filter((item) => item.total > 0 || item.overrides > 0)
        .filter((item) => item.tab === activeTab());
    } catch (error) {
      logger.error('Error in summaryItems memo:', error);
      return [];
    }
  });

  const hasSection = (key: string) => summaryItems().some((item) => item.key === key);

  const startEditing = (
    resourceId: string,
    currentThresholds: Record<string, number | undefined>,
    defaults: Record<string, number | undefined>,
    note?: string,
  ) => {
    setEditingId(resourceId);
    setEditingThresholds({ ...defaults, ...currentThresholds });
    setEditingNote(note ?? '');
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditingThresholds({});
    setEditingNote('');
  };

  const {
    handleSaveBulkEdit: persistBulkEdit,
    removeOverride,
    saveEdit,
    toggleBackup,
    toggleSnapshot,
  } = useThresholdsOverrideMutations({
    props,
    resources: {
      nodesWithOverrides,
      agentsWithOverrides,
      agentDisksWithOverrides,
      dockerHostsWithOverrides,
      guestsFlat,
      dockerContainersFlat,
      pbsServersWithOverrides,
      pmgServersWithOverrides,
      storageWithOverrides,
      kubernetesClustersWithOverrides,
      kubernetesNodesWithOverrides,
      kubernetesNamespacesWithOverrides,
      kubernetesDeploymentsWithOverrides,
      kubernetesPodsWithOverrides,
      trueNASSystemsWithOverrides,
      trueNASPoolsWithOverrides,
      trueNASDatasetsWithOverrides,
      trueNASDisksWithOverrides,
      vmwareHostsWithOverrides,
      vmwareVMsWithOverrides,
      vmwareDatastoresWithOverrides,
      vmwareNetworksWithOverrides,
    },
    editingThresholds,
    editingNote,
    bulkEditIds,
    cancelEdit,
    updateBackupDefaults,
    updateSnapshotDefaults,
  });

  const { setOfflineState, toggleDisabled, toggleNodeConnectivity } =
    useThresholdsAvailabilityMutations({
      props,
      resources: {
        nodesWithOverrides,
        agentsWithOverrides,
        agentDisksWithOverrides,
        dockerHostsWithOverrides,
        guestsFlat,
        dockerContainersFlat,
        pbsServersWithOverrides,
        storageWithOverrides,
        kubernetesClustersWithOverrides,
        kubernetesNodesWithOverrides,
        kubernetesNamespacesWithOverrides,
        kubernetesDeploymentsWithOverrides,
        kubernetesPodsWithOverrides,
        trueNASSystemsWithOverrides,
        trueNASPoolsWithOverrides,
        trueNASDatasetsWithOverrides,
        trueNASDisksWithOverrides,
        vmwareHostsWithOverrides,
        vmwareVMsWithOverrides,
        vmwareDatastoresWithOverrides,
        vmwareNetworksWithOverrides,
      },
      removeOverride,
    });

  const handleBulkEdit = (ids: string[], columns: string[]) => {
    setBulkEditIds(ids);
    setBulkEditColumns(columns);
    setIsBulkEditDialogOpen(true);
  };

  const handleSaveBulkEdit = (thresholds: Record<string, number | undefined>) => {
    setIsBulkEditDialogOpen(false);
    persistBulkEdit(thresholds);
    setBulkEditIds([]);
    setBulkEditColumns([]);
  };

  const updateMetricDelay = (
    typeKey:
      | 'guest'
      | 'node'
      | 'storage'
      | 'pbs'
      | 'agent'
      | 'k8s-cluster'
      | 'k8s-node'
      | 'k8s-deployment'
      | 'k8s-namespace'
      | 'pod'
      | 'truenas-system'
      | 'truenas-pool'
      | 'truenas-dataset'
      | 'truenas-disk'
      | 'vmware-host'
      | 'vmware-vm'
      | 'vmware-datastore'
      | 'vmware-network',
    metricKey: string,
    value: number | null,
  ) => {
    const normalizedMetric = metricKey.trim().toLowerCase();
    if (!normalizedMetric) return;

    let changed = false;
    props.setMetricTimeThresholds((prev) => {
      const current = prev ? { ...prev } : {};
      const existing = prev?.[typeKey];
      const typeOverrides = existing ? { ...existing } : {};

      if (value === null) {
        if (typeOverrides[normalizedMetric] === undefined) {
          return prev;
        }
        delete typeOverrides[normalizedMetric];
        changed = true;
      } else {
        const sanitized = Math.max(0, Math.round(value));
        if (typeOverrides[normalizedMetric] === sanitized) {
          return prev;
        }
        typeOverrides[normalizedMetric] = sanitized;
        changed = true;
      }

      if (!changed) {
        return prev;
      }

      if (Object.keys(typeOverrides).length === 0) {
        delete current[typeKey];
      } else {
        current[typeKey] = typeOverrides;
      }

      return current;
    });

    if (changed) {
      props.setHasUnsavedChanges(true);
    }
  };

  return {
    activeTab,
    agentDisksGroupedByAgent,
    agentGroupHeaderMeta,
    agentDisksWithOverrides,
    agentsWithOverrides,
    alertsEnabled,
    backupDefaultsRecord,
    backupFactoryConfig,
    backupFactoryDefaultsRecord,
    backupOrphanedPresentation,
    backupOverridesCount,
    BACKUP_THRESHOLDS_EMPTY_STATE,
    bulkEditColumns,
    bulkEditIds,
    cancelEdit,
    collapseAll,
    CONTAINERS_FILTER_EMPTY_STATE,
    CONTAINER_RUNTIMES_FILTER_EMPTY_STATE,
    dismissHelpBanner,
    dockerContainersFlat,
    dockerContainersGroupedByHost,
    dockerHostGroupMeta,
    dockerHostsWithOverrides,
    dockerIgnoredInput,
    dockerIgnoredPrefixesPresentation,
    dockerServicePresentation,
    editingId,
    editingNote,
    editingThresholds,
    expandAll,
    GUEST_FILTERING_EMPTY_STATE,
    GUEST_THRESHOLDS_EMPTY_STATE,
    GUEST_THRESHOLDS_FILTER_EMPTY_STATE,
    guestFilterPresentation,
    guestGroupHeaderMeta,
    guestsFlat,
    guestsGroupedByNode,
    handleBulkEdit,
    handleDockerIgnoredChange,
    handleResetDockerIgnored,
    handleSaveBulkEdit,
    handleTabClick,
    hasActiveAlert,
    hasSection,
    helpBannerDismissed,
    hasDockerSpecificControls,
    isBulkEditDialogOpen,
    isCollapsed,
    kubernetesClustersWithOverrides,
    kubernetesDeploymentsWithOverrides,
    kubernetesNamespacesWithOverrides,
    kubernetesNodesWithOverrides,
    kubernetesPodsWithOverrides,
    nodesWithOverrides,
    NODE_THRESHOLDS_FILTER_EMPTY_STATE,
    pbsServersWithOverrides,
    PBS_THRESHOLDS_EMPTY_STATE,
    PBS_THRESHOLDS_FILTER_EMPTY_STATE,
    pmgGlobalDefaults,
    PMG_THRESHOLDS_EMPTY_STATE,
    PMG_THRESHOLDS_FILTER_EMPTY_STATE,
    pmgServersWithOverrides,
    registerSection,
    removeOverride,
    overrideFilter,
    saveEdit,
    searchTerm,
    sectionTitles,
    serviceCriticalInputId,
    serviceGapValidationMessage,
    serviceWarnInputId,
    setActiveTab,
    setBulkEditColumns,
    setBulkEditIds,
    setDockerIgnoredInput,
    setEditingId,
    setEditingNote,
    setEditingThresholds,
    setHasUnsavedChanges: props.setHasUnsavedChanges,
    setIsBulkEditDialogOpen,
    setOverrideFilter,
    setOfflineState,
    setPMGGlobalDefaults,
    SNAPSHOT_THRESHOLDS_EMPTY_STATE,
    snapshotDefaultsRecord,
    snapshotFactoryConfig,
    snapshotFactoryDefaultsRecord,
    snapshotOverridesCount,
    startEditing,
    storageGroupedByNode,
    storageWithOverrides,
    STORAGE_THRESHOLDS_EMPTY_STATE,
    STORAGE_THRESHOLDS_FILTER_EMPTY_STATE,
    summaryItems,
    trueNASDatasetsWithOverrides,
    trueNASDisksWithOverrides,
    trueNASPoolsWithOverrides,
    trueNASSystemsWithOverrides,
    vmwareDatastoresWithOverrides,
    vmwareHostsWithOverrides,
    vmwareNetworksWithOverrides,
    vmwareVMsWithOverrides,
    toggleBackup,
    toggleDisabled,
    toggleNodeConnectivity,
    toggleSection,
    toggleSnapshot,
    totalDockerContainers,
    updateBackupDefaults,
    updateMetricDelay,
    updateSnapshotDefaults,
    setSearchTerm,
    AGENT_DISKS_EMPTY_STATE,
    AGENT_DISKS_FILTER_EMPTY_STATE,
    AGENT_THRESHOLDS_FILTER_EMPTY_STATE,
    getAlertThresholdsHelpBanner,
    getAlertThresholdsHelpDismissLabel,
    getAlertThresholdsSearchPlaceholder,
  } as const;
}

export type ThresholdsTableState = ReturnType<typeof useThresholdsTableState>;
