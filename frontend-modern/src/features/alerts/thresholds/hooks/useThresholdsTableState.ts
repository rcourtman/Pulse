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
    | 'nodes'
    | 'pbs'
    | 'pmg'
    | 'snapshots'
    | 'storage';
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

  const { isCollapsed, toggleSection, expandAll, collapseAll } = useCollapsedSections();

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
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [editingThresholds, setEditingThresholds] = createSignal<
    Record<string, number | undefined>
  >({});
  const [editingNote, setEditingNote] = createSignal('');
  const [bulkEditIds, setBulkEditIds] = createSignal<string[]>([]);
  const [bulkEditColumns, setBulkEditColumns] = createSignal<string[]>([]);
  const [isBulkEditDialogOpen, setIsBulkEditDialogOpen] = createSignal(false);
  const [activeTab, setActiveTab] = createSignal<ThresholdsActiveTab>('infrastructure');
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

  const getActiveTabFromRoute = (): ThresholdsActiveTab => {
    const path = location.pathname;
    if (path.includes('/thresholds/containers')) return 'docker';
    if (path.includes('/thresholds/systems') || path.includes('/thresholds/agents')) return 'systems';
    if (path.includes('/thresholds/mail-gateway')) return 'pmg';
    return 'infrastructure';
  };

  createEffect(() => {
    const tabFromRoute = getActiveTabFromRoute();
    if (activeTab() !== tabFromRoute) {
      setActiveTab(tabFromRoute);
    }
  });

  createEffect(() => {
    if (location.pathname === '/alerts/thresholds') {
      navigate('/alerts/thresholds/infrastructure', { replace: true });
      return;
    }

    if (location.pathname === '/alerts/thresholds/proxmox') {
      navigate('/alerts/thresholds/infrastructure', { replace: true });
      return;
    }

    if (location.pathname === '/alerts/thresholds/agents') {
      navigate('/alerts/thresholds/systems', { replace: true });
    }
  });

  const handleTabClick = (tab: ThresholdsActiveTab) => {
    const tabRoutes: Record<ThresholdsActiveTab, string> = {
      infrastructure: '/alerts/thresholds/infrastructure',
      docker: '/alerts/thresholds/containers',
      pmg: '/alerts/thresholds/mail-gateway',
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
    nodesWithOverrides,
    agentsWithOverrides,
    agentDisksWithOverrides,
    agentDisksGroupedByAgent,
    dockerHostsWithOverrides,
    dockerContainersGroupedByHost,
    dockerContainersFlat,
    totalDockerContainers,
    dockerHostGroupMeta,
    guestsGroupedByNode,
    guestsFlat,
    guestGroupHeaderMeta,
    pbsServersWithOverrides,
    pmgGlobalDefaults,
    pmgServersWithOverrides,
    storageWithOverrides,
    storageGroupedByNode,
  } = useThresholdsData(props, editingId, searchTerm);

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

  const registerSection = (_key: string) => (_element: HTMLDivElement | null) => {
    /* no-op placeholder for future scroll restoration */
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
          tab: 'infrastructure',
          total: props.nodes?.length ?? 0,
        },
        {
          key: 'dockerHosts',
          label: 'Container Runtimes',
          overrides: countOverrides(dockerHostsWithOverrides()),
          tab: 'docker',
          total: props.dockerHosts?.length ?? 0,
        },
        {
          key: 'agents',
          label: 'Systems',
          overrides: countOverrides(agentsWithOverrides()),
          tab: 'systems',
          total: props.agents?.length ?? 0,
        },
        {
          key: 'agentDisks',
          label: 'System Disks',
          overrides: countOverrides(agentDisksWithOverrides()),
          tab: 'systems',
          total: agentDisksWithOverrides().length,
        },
        {
          key: 'storage',
          label: 'Storage',
          overrides: countOverrides(storageWithOverrides()),
          tab: 'infrastructure',
          total: props.storage?.length ?? 0,
        },
        {
          key: 'backups',
          label: 'Recovery',
          overrides: backupOverridesCount(),
          tab: 'infrastructure',
          total: 1,
        },
        {
          key: 'snapshots',
          label: 'Snapshot Age',
          overrides: snapshotOverridesCount(),
          tab: 'infrastructure',
          total: 1,
        },
        {
          key: 'pbs',
          label: 'PBS Servers',
          overrides: countOverrides(pbsServersWithOverrides()),
          tab: 'infrastructure',
          total: props.pbsInstances?.length ?? 0,
        },
        {
          key: 'pmg',
          label: 'Mail Gateways',
          overrides: countOverrides(pmgServersWithOverrides()),
          tab: 'pmg',
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
          tab: 'infrastructure',
          total: props.allGuests?.()?.length ?? 0,
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
    typeKey: 'guest' | 'node' | 'storage' | 'pbs' | 'agent',
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
    isBulkEditDialogOpen,
    isCollapsed,
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
