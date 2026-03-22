import { createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import type {
  RawOverrideConfig,
  PMGThresholdDefaults,
  SnapshotAlertConfig,
  BackupAlertConfig,
} from '@/types/alerts';
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
import { matchesAlertIdentifier } from '@/features/alerts/identity';
import {
  PMG_THRESHOLD_COLUMNS,
  DEFAULT_SNAPSHOT_WARNING,
  DEFAULT_SNAPSHOT_CRITICAL,
  DEFAULT_SNAPSHOT_WARNING_SIZE,
  DEFAULT_SNAPSHOT_CRITICAL_SIZE,
  DEFAULT_BACKUP_WARNING,
  DEFAULT_BACKUP_CRITICAL,
} from '@/features/alerts/thresholds/constants';
import { normalizeDockerIgnoredInput } from '@/features/alerts/thresholds/helpers';
import type {
  Override,
  OverrideType,
  OfflineState,
  ThresholdsTableProps,
} from '@/features/alerts/thresholds/types';
import type {
  Resource as TableResource,
  ThresholdsActiveTab,
} from '@/features/alerts/thresholds/tableTypes';
import { useCollapsedSections } from '@/components/Alerts/Thresholds/hooks/useCollapsedSections';
import { useThresholdsData } from './useThresholdsData';

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

  const getActiveTabFromRoute = (): ThresholdsActiveTab => {
    const path = location.pathname;
    if (path.includes('/thresholds/containers')) return 'docker';
    if (path.includes('/thresholds/agents')) return 'agents';
    if (path.includes('/thresholds/mail-gateway')) return 'pmg';
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
    }
  });

  const handleTabClick = (tab: ThresholdsActiveTab) => {
    const tabRoutes: Record<ThresholdsActiveTab, string> = {
      agents: '/alerts/thresholds/agents',
      docker: '/alerts/thresholds/containers',
      pmg: '/alerts/thresholds/mail-gateway',
      proxmox: '/alerts/thresholds/proxmox',
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
    snapshotFactoryConfig,
    sanitizeSnapshotConfig,
    backupFactoryConfig,
    sanitizeBackupConfig,
    snapshotDefaultsRecord,
    snapshotFactoryDefaultsRecord,
    backupDefaultsRecord,
    backupFactoryDefaultsRecord,
    snapshotOverridesCount,
    backupOverridesCount,
    guestsGroupedByNode,
    guestsFlat,
    guestGroupHeaderMeta,
    pbsServersWithOverrides,
    pmgGlobalDefaults,
    pmgServersWithOverrides,
    storageWithOverrides,
    storageGroupedByNode,
  } = useThresholdsData(props, editingId, searchTerm);

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
          label: 'Nodes',
          overrides: countOverrides(nodesWithOverrides()),
          tab: 'proxmox',
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
          label: 'Agents',
          overrides: countOverrides(agentsWithOverrides()),
          tab: 'agents',
          total: props.agents?.length ?? 0,
        },
        {
          key: 'agentDisks',
          label: 'Agent Disks',
          overrides: countOverrides(agentDisksWithOverrides()),
          tab: 'agents',
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
          tab: 'proxmox',
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

  const saveEdit = (resourceId: string) => {
    const allResources = [
      ...nodesWithOverrides(),
      ...agentsWithOverrides(),
      ...agentDisksWithOverrides(),
      ...dockerHostsWithOverrides(),
      ...guestsFlat(),
      ...dockerContainersFlat(),
      ...storageWithOverrides(),
      ...pbsServersWithOverrides(),
    ];
    const resource = allResources.find((entry) => entry.id === resourceId);
    if (!resource) return;

    const editedThresholdMap = editingThresholds();
    const trimmedNote = editingNote().trim();
    const noteForOverride = trimmedNote.length > 0 ? trimmedNote : undefined;

    if (resource.editScope === 'backup') {
      const currentBackupDefaults = props.backupDefaults();
      updateBackupDefaults({
        criticalDays:
          editedThresholdMap['critical days'] ??
          currentBackupDefaults.criticalDays ??
          DEFAULT_BACKUP_CRITICAL,
        enabled: currentBackupDefaults.enabled,
        warningDays:
          editedThresholdMap['warning days'] ??
          currentBackupDefaults.warningDays ??
          DEFAULT_BACKUP_WARNING,
      });
      cancelEdit();
      return;
    }

    if (resource.editScope === 'snapshot') {
      const currentSnapshotDefaults = props.snapshotDefaults();
      updateSnapshotDefaults({
        criticalDays:
          editedThresholdMap['critical days'] ??
          currentSnapshotDefaults.criticalDays ??
          DEFAULT_SNAPSHOT_CRITICAL,
        criticalSizeGiB:
          editedThresholdMap['critical size (gib)'] ??
          currentSnapshotDefaults.criticalSizeGiB ??
          DEFAULT_SNAPSHOT_CRITICAL_SIZE,
        enabled: currentSnapshotDefaults.enabled,
        warningDays:
          editedThresholdMap['warning days'] ??
          currentSnapshotDefaults.warningDays ??
          DEFAULT_SNAPSHOT_WARNING,
        warningSizeGiB:
          editedThresholdMap['warning size (gib)'] ??
          currentSnapshotDefaults.warningSizeGiB ??
          DEFAULT_SNAPSHOT_WARNING_SIZE,
      });
      cancelEdit();
      return;
    }

    const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;
    const overrideThresholds: Record<string, number> = {};

    Object.keys(editedThresholdMap).forEach((key) => {
      const editedValue = editedThresholdMap[key];
      const defaultValue = defaultThresholds[key];
      if (editedValue !== undefined && editedValue !== defaultValue) {
        overrideThresholds[key] = editedValue;
      }
    });

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const hasStateOnlyOverride = Boolean(
      resource.disabled ||
        resource.disableConnectivity ||
        resource.poweredOffSeverity !== undefined ||
        noteForOverride !== undefined ||
        existingOverride?.backup ||
        existingOverride?.snapshot,
    );

    if (Object.keys(overrideThresholds).length === 0 && !hasStateOnlyOverride) {
      if (resource.hasOverride) {
        props.setOverrides(props.overrides().filter((override) => override.id !== resourceId));
        const newRawConfig = { ...props.rawOverridesConfig() };
        delete newRawConfig[resourceId];
        props.setRawOverridesConfig(newRawConfig);
        props.setHasUnsavedChanges(true);
      }
      cancelEdit();
      return;
    }

    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type as OverrideType,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: resource.disabled,
      disableConnectivity: resource.disableConnectivity,
      poweredOffSeverity: resource.poweredOffSeverity,
      note: noteForOverride,
      backup: existingOverride?.backup,
      snapshot: existingOverride?.snapshot,
      thresholds: overrideThresholds,
    };

    const existingIndex = props.overrides().findIndex((entry) => entry.id === resourceId);
    if (existingIndex >= 0) {
      const nextOverrides = [...props.overrides()];
      nextOverrides[existingIndex] = override;
      props.setOverrides(nextOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
    const previousRaw = props.rawOverridesConfig()[resourceId];
    const hysteresisThresholds: RawOverrideConfig = {};

    if (previousRaw) {
      if (previousRaw.disabled !== undefined) hysteresisThresholds.disabled = previousRaw.disabled;
      if (previousRaw.disableConnectivity !== undefined) {
        hysteresisThresholds.disableConnectivity = previousRaw.disableConnectivity;
      }
      if (previousRaw.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = previousRaw.poweredOffSeverity;
      }
    }

    Object.entries(overrideThresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = {
          clear: Math.max(0, value - 5),
          trigger: value,
        };
      }
    });

    if (resource.disabled) {
      hysteresisThresholds.disabled = true;
    } else {
      delete hysteresisThresholds.disabled;
    }

    if (resource.disableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
      delete hysteresisThresholds.poweredOffSeverity;
    } else {
      if (
        (resource.type === 'guest' || resource.type === 'dockerContainer') &&
        props.guestDisableConnectivity()
      ) {
        hysteresisThresholds.disableConnectivity = false;
      } else {
        delete hysteresisThresholds.disableConnectivity;
      }
      if (resource.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = resource.poweredOffSeverity;
      } else {
        delete hysteresisThresholds.poweredOffSeverity;
      }
    }

    if (noteForOverride) {
      hysteresisThresholds.note = noteForOverride;
    } else {
      delete hysteresisThresholds.note;
    }

    if (previousRaw?.backup) hysteresisThresholds.backup = previousRaw.backup;
    if (previousRaw?.snapshot) hysteresisThresholds.snapshot = previousRaw.snapshot;

    newRawConfig[resourceId] = hysteresisThresholds;
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);
    cancelEdit();
  };

  const handleBulkEdit = (ids: string[], columns: string[]) => {
    setBulkEditIds(ids);
    setBulkEditColumns(columns);
    setIsBulkEditDialogOpen(true);
  };

  const handleSaveBulkEdit = (thresholds: Record<string, number | undefined>) => {
    setIsBulkEditDialogOpen(false);

    const nextOverrides = [...props.overrides()];
    const nextRawConfig = { ...props.rawOverridesConfig() };
    const allResources = [
      ...nodesWithOverrides(),
      ...agentsWithOverrides(),
      ...agentDisksWithOverrides(),
      ...dockerHostsWithOverrides(),
      ...pbsServersWithOverrides(),
      ...pmgServersWithOverrides(),
      ...storageWithOverrides(),
    ];

    for (const id of bulkEditIds()) {
      const resource = allResources.find((entry) => entry.id === id);
      if (!resource) continue;

      const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;
      const existingOverride = nextOverrides.find((override) => override.id === id);
      const previousRaw = nextRawConfig[id];
      const newThresholds: Record<string, number | undefined> = {
        ...(existingOverride?.thresholds ?? {}),
      };

      Object.keys(thresholds).forEach((key) => {
        if (thresholds[key] !== undefined) {
          const value = thresholds[key];
          if (value === defaultThresholds[key]) {
            delete newThresholds[key];
          } else {
            newThresholds[key] = value as number;
          }
        }
      });

      const hasStateOnlyOverride = Boolean(
        resource.disabled ||
          resource.disableConnectivity ||
          resource.poweredOffSeverity !== undefined ||
          existingOverride?.note !== undefined ||
          existingOverride?.backup ||
          existingOverride?.snapshot,
      );

      if (Object.keys(newThresholds).length === 0 && !hasStateOnlyOverride) {
        if (resource.hasOverride) {
          const existingIndex = nextOverrides.findIndex((override) => override.id === id);
          if (existingIndex !== -1) nextOverrides.splice(existingIndex, 1);
          delete nextRawConfig[id];
        }
        continue;
      }

      const override: Override = {
        id,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? resource.instance : undefined,
        disabled: resource.disabled,
        disableConnectivity: resource.disableConnectivity,
        poweredOffSeverity: resource.poweredOffSeverity,
        note: existingOverride?.note,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: newThresholds,
      };

      const existingIndex = nextOverrides.findIndex((entry) => entry.id === id);
      if (existingIndex >= 0) {
        nextOverrides[existingIndex] = override;
      } else {
        nextOverrides.push(override);
      }

      const hysteresisThresholds: RawOverrideConfig = {};
      if (previousRaw) {
        if (previousRaw.disabled !== undefined) hysteresisThresholds.disabled = previousRaw.disabled;
        if (previousRaw.disableConnectivity !== undefined) {
          hysteresisThresholds.disableConnectivity = previousRaw.disableConnectivity;
        }
        if (previousRaw.poweredOffSeverity !== undefined) {
          hysteresisThresholds.poweredOffSeverity = previousRaw.poweredOffSeverity;
        }
        if (previousRaw.note !== undefined) hysteresisThresholds.note = previousRaw.note;
        if (previousRaw.backup !== undefined) hysteresisThresholds.backup = previousRaw.backup;
        if (previousRaw.snapshot !== undefined) {
          hysteresisThresholds.snapshot = previousRaw.snapshot;
        }
      }

      Object.entries(newThresholds).forEach(([metric, value]) => {
        if (value !== undefined && value !== null) {
          hysteresisThresholds[metric] = {
            clear: Math.max(0, value - 5),
            trigger: value,
          };
        }
      });

      nextRawConfig[id] = hysteresisThresholds;
    }

    props.setOverrides(nextOverrides);
    props.setRawOverridesConfig(nextRawConfig);
    props.setHasUnsavedChanges(true);
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

  const removeOverride = (resourceId: string) => {
    props.setOverrides(props.overrides().filter((override) => override.id !== resourceId));
    const newRawConfig = { ...props.rawOverridesConfig() };
    delete newRawConfig[resourceId];
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);
  };

  const toggleBackup = (resourceId: string, forceState?: boolean) => {
    const resource = [...guestsFlat(), ...dockerContainersFlat()].find((entry) => entry.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const baseConfig = existingOverride?.backup || props.backupDefaults();
    const newBackup = {
      ...baseConfig,
      enabled: forceState !== undefined ? forceState : !baseConfig.enabled,
    };

    const override: Override = {
      ...(existingOverride || {
        id: resourceId,
        instance: 'instance' in resource ? resource.instance : undefined,
        name: resource.name,
        node: 'node' in resource ? resource.node : undefined,
        thresholds: {},
        type: resource.type as OverrideType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
      }),
      backup: newBackup,
    };

    const existingIndex = props.overrides().findIndex((entry) => entry.id === resourceId);
    if (existingIndex >= 0) {
      const nextOverrides = [...props.overrides()];
      nextOverrides[existingIndex] = override;
      props.setOverrides(nextOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig = { ...props.rawOverridesConfig() };
    newRawConfig[resourceId] = {
      ...(newRawConfig[resourceId] || {}),
      backup: newBackup,
    };
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);
  };

  const toggleSnapshot = (resourceId: string, forceState?: boolean) => {
    const resource = [...guestsFlat(), ...dockerContainersFlat()].find((entry) => entry.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const baseConfig = existingOverride?.snapshot || props.snapshotDefaults();
    const newSnapshot = {
      ...baseConfig,
      enabled: forceState !== undefined ? forceState : !baseConfig.enabled,
    };

    const override: Override = {
      ...(existingOverride || {
        id: resourceId,
        instance: 'instance' in resource ? resource.instance : undefined,
        name: resource.name,
        node: 'node' in resource ? resource.node : undefined,
        thresholds: {},
        type: resource.type as OverrideType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
      }),
      snapshot: newSnapshot,
    };

    const existingIndex = props.overrides().findIndex((entry) => entry.id === resourceId);
    if (existingIndex >= 0) {
      const nextOverrides = [...props.overrides()];
      nextOverrides[existingIndex] = override;
      props.setOverrides(nextOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig = { ...props.rawOverridesConfig() };
    newRawConfig[resourceId] = {
      ...(newRawConfig[resourceId] || {}),
      snapshot: newSnapshot,
    };
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);
  };

  const toggleDisabled = (resourceId: string, forceState?: boolean) => {
    const resource = [
      ...guestsFlat(),
      ...dockerContainersFlat(),
      ...storageWithOverrides(),
      ...pbsServersWithOverrides(),
      ...agentsWithOverrides(),
      ...agentDisksWithOverrides(),
    ].find((entry) => entry.id === resourceId);

    if (
      !resource ||
      (resource.type !== 'guest' &&
        resource.type !== 'storage' &&
        resource.type !== 'pbs' &&
        resource.type !== 'dockerContainer' &&
        resource.type !== 'agent' &&
        resource.type !== 'agentDisk')
    ) {
      return;
    }

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const currentDisabledState = resource.disabled;
    const newDisabledState = forceState !== undefined ? forceState : !currentDisabledState;
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;

    if (!newDisabledState && (!existingOverride || Object.keys(cleanThresholds).length === 0)) {
      props.setOverrides(props.overrides().filter((override) => override.id !== resourceId));
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? resource.instance : undefined,
        disabled: newDisabledState,
        disableConnectivity: existingOverride?.disableConnectivity,
        poweredOffSeverity: existingOverride?.poweredOffSeverity,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: cleanThresholds,
      };

      const existingIndex = props.overrides().findIndex((entry) => entry.id === resourceId);
      if (existingIndex >= 0) {
        const nextOverrides = [...props.overrides()];
        nextOverrides[existingIndex] = override;
        props.setOverrides(nextOverrides);
      } else {
        props.setOverrides([...props.overrides(), override]);
      }

      const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
      const hysteresisThresholds: RawOverrideConfig = {};

      Object.entries(override.thresholds).forEach(([metric, value]) => {
        if (typeof value === 'number') {
          hysteresisThresholds[metric] = {
            clear: Math.max(0, value - 5),
            trigger: value,
          };
        }
      });

      if (newDisabledState) hysteresisThresholds.disabled = true;
      if (override.backup) hysteresisThresholds.backup = override.backup;
      if (override.snapshot) hysteresisThresholds.snapshot = override.snapshot;
      if (override.disableConnectivity) hysteresisThresholds.disableConnectivity = true;
      if (override.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = override.poweredOffSeverity;
      }

      if (Object.keys(hysteresisThresholds).length === 0) {
        delete newRawConfig[resourceId];
      } else {
        newRawConfig[resourceId] = hysteresisThresholds;
      }
      props.setRawOverridesConfig(newRawConfig);
    }

    if (newDisabledState && props.removeAlerts) {
      if (resource.type === 'guest') {
        props.removeAlerts(
          (alert) => alert.resourceId === resourceId && alert.type === 'powered-off',
        );
      } else if (resource.type === 'pbs') {
        const offlineId = `pbs-offline-${resourceId}`;
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (matchesAlertIdentifier(alert, offlineId) || alert.type === 'offline'),
        );
      } else if (resource.type === 'dockerContainer') {
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.type === 'docker-container-state' || alert.type === 'docker-container-health'),
        );
      }
    }

    props.setHasUnsavedChanges(true);
  };

  const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {
    const resource = [
      ...nodesWithOverrides(),
      ...pbsServersWithOverrides(),
      ...guestsFlat(),
      ...agentsWithOverrides(),
      ...dockerHostsWithOverrides(),
    ].find((entry) => entry.id === resourceId);

    if (
      !resource ||
      (resource.type !== 'agent' &&
        resource.type !== 'pbs' &&
        resource.type !== 'guest' &&
        resource.type !== 'dockerHost')
    ) {
      return;
    }

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const currentDisableConnectivity = resource.disableConnectivity;
    const newDisableConnectivity =
      forceState !== undefined ? forceState : !currentDisableConnectivity;
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;
    delete (cleanThresholds as Record<string, unknown>).disableConnectivity;

    if (!newDisableConnectivity && Object.keys(cleanThresholds).length === 0) {
      props.setOverrides(props.overrides().filter((override) => override.id !== resourceId));
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        disableConnectivity: newDisableConnectivity,
        disabled: existingOverride?.disabled,
        poweredOffSeverity: existingOverride?.poweredOffSeverity,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: cleanThresholds,
      };

      const existingIndex = props.overrides().findIndex((entry) => entry.id === resourceId);
      if (existingIndex >= 0) {
        const nextOverrides = [...props.overrides()];
        nextOverrides[existingIndex] = override;
        props.setOverrides(nextOverrides);
      } else {
        props.setOverrides([...props.overrides(), override]);
      }

      const newRawConfig = { ...props.rawOverridesConfig() };
      const hysteresisThresholds: Record<string, unknown> = {};

      Object.entries(cleanThresholds).forEach(([metric, value]) => {
        if (value !== undefined && value !== null) {
          hysteresisThresholds[metric] = {
            clear: Math.max(0, value - 5),
            trigger: value,
          };
        }
      });

      if (newDisableConnectivity) {
        hysteresisThresholds.disableConnectivity = true;
      }
      if (override.backup) hysteresisThresholds.backup = override.backup;
      if (override.snapshot) hysteresisThresholds.snapshot = override.snapshot;
      if (override.disabled) hysteresisThresholds.disabled = true;
      if (override.poweredOffSeverity) {
        hysteresisThresholds.poweredOffSeverity = override.poweredOffSeverity;
      }

      if (Object.keys(hysteresisThresholds).length === 0) {
        delete newRawConfig[resourceId];
      } else {
        newRawConfig[resourceId] = hysteresisThresholds as RawOverrideConfig;
      }
      props.setRawOverridesConfig(newRawConfig);
    }

    props.setHasUnsavedChanges(true);

    if (props.removeAlerts && resource.type === 'dockerHost') {
      const offlineId = `docker-host-offline-${resourceId}`;
      const resourceKey = `docker:${resourceId}`;
      props.removeAlerts(
        (alert) => matchesAlertIdentifier(alert, offlineId) || alert.resourceId === resourceKey,
      );
    }
  };

  const setOfflineState = (resourceId: string, state: OfflineState) => {
    const resource = [...guestsFlat(), ...dockerContainersFlat()].find((entry) => entry.id === resourceId);
    if (!resource) return;

    const isDockerContainer = resource.type === 'dockerContainer';
    const defaultDisabled = isDockerContainer
      ? props.dockerDisableConnectivity()
      : props.guestDisableConnectivity();
    const defaultSeverity = isDockerContainer
      ? props.dockerPoweredOffSeverity()
      : props.guestPoweredOffSeverity();

    const existingOverride = props.overrides().find((override) => override.id === resourceId);
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;
    delete (cleanThresholds as Record<string, unknown>).disableConnectivity;
    delete (cleanThresholds as Record<string, unknown>).poweredOffSeverity;

    const newDisableConnectivity = state === 'off';
    const newSeverity: 'warning' | 'critical' | undefined =
      state === 'off' ? undefined : state === 'critical' ? 'critical' : 'warning';

    const overrideDisabled = existingOverride?.disabled || false;
    const hasThresholds = Object.keys(cleanThresholds).length > 0;
    const differsFromDefaults =
      newDisableConnectivity !== defaultDisabled ||
      (!newDisableConnectivity && newSeverity !== defaultSeverity);

    if (
      !differsFromDefaults &&
      !hasThresholds &&
      !overrideDisabled &&
      !existingOverride?.disableConnectivity
    ) {
      if (existingOverride) {
        props.setOverrides(props.overrides().filter((override) => override.id !== resourceId));
        const newRawConfig = { ...props.rawOverridesConfig() };
        delete newRawConfig[resourceId];
        props.setRawOverridesConfig(newRawConfig);
        props.setHasUnsavedChanges(true);
      }
      return;
    }

    const override: Override = {
      id: resourceId,
      name: resource.name,
      type: resource.type as OverrideType,
      resourceType: resource.resourceType,
      vmid: 'vmid' in resource ? resource.vmid : undefined,
      node: 'node' in resource ? resource.node : undefined,
      instance: 'instance' in resource ? resource.instance : undefined,
      disabled: overrideDisabled,
      disableConnectivity: newDisableConnectivity,
      poweredOffSeverity: newDisableConnectivity ? undefined : newSeverity,
      backup: existingOverride?.backup,
      snapshot: existingOverride?.snapshot,
      thresholds: cleanThresholds,
    };

    const existingIndex = props.overrides().findIndex((entry) => entry.id === resourceId);
    if (existingIndex >= 0) {
      const nextOverrides = [...props.overrides()];
      nextOverrides[existingIndex] = override;
      props.setOverrides(nextOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
    const hysteresisThresholds: RawOverrideConfig = {};

    Object.entries(cleanThresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = {
          clear: Math.max(0, value - 5),
          trigger: value,
        };
      }
    });

    if (overrideDisabled) hysteresisThresholds.disabled = true;
    if (newDisableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
    } else {
      if (defaultDisabled) hysteresisThresholds.disableConnectivity = false;
      if (newSeverity) hysteresisThresholds.poweredOffSeverity = newSeverity;
    }
    if (override.backup) hysteresisThresholds.backup = override.backup;
    if (override.snapshot) hysteresisThresholds.snapshot = override.snapshot;

    if (Object.keys(hysteresisThresholds).length > 0) {
      newRawConfig[resourceId] = hysteresisThresholds;
    } else {
      delete newRawConfig[resourceId];
    }

    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);

    if (props.removeAlerts && newDisableConnectivity) {
      if (resource.type === 'guest') {
        props.removeAlerts(
          (alert) => alert.resourceId === resourceId && alert.type === 'powered-off',
        );
      } else if (resource.type === 'dockerContainer') {
        props.removeAlerts(
          (alert) =>
            alert.resourceId === resourceId &&
            (alert.type === 'docker-container-state' || alert.type === 'docker-container-health'),
        );
      }
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
