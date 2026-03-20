import { createSignal, createMemo, Show, For, createEffect } from 'solid-js';
import { useNavigate, useLocation } from '@solidjs/router';
import Toggle from '@/components/shared/Toggle';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { useCollapsedSections } from './Thresholds/hooks/useCollapsedSections';
import { TagInput } from '@/components/shared/TagInput';
import Server from 'lucide-solid/icons/server';
import Monitor from 'lucide-solid/icons/monitor';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Database from 'lucide-solid/icons/database';
import Archive from 'lucide-solid/icons/archive';
import Camera from 'lucide-solid/icons/camera';
import Mail from 'lucide-solid/icons/mail';
import Users from 'lucide-solid/icons/users';
import Boxes from 'lucide-solid/icons/boxes';

// Workaround for eslint false-positive when `For` is used only in JSX
const __ensureForUsage = For;
void __ensureForUsage;

import type {
  RawOverrideConfig,
  PMGThresholdDefaults,
  SnapshotAlertConfig,
  BackupAlertConfig,
} from '@/types/alerts';
import { ResourceTable } from './ResourceTable';
import { BulkEditDialog } from './BulkEditDialog';
import type { Resource as TableResource } from './ResourceTable';
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
import type {
  OverrideType,
  OfflineState,
  Override,
  ThresholdsTableProps,
} from '@/features/alerts/thresholds/types';
import { matchesAlertIdentifier } from '@/features/alerts/identity';
import {
  PMG_THRESHOLD_COLUMNS,
  DEFAULT_SNAPSHOT_WARNING,
  DEFAULT_SNAPSHOT_CRITICAL,
  DEFAULT_SNAPSHOT_WARNING_SIZE,
  DEFAULT_SNAPSHOT_CRITICAL_SIZE,
  DEFAULT_BACKUP_WARNING,
  DEFAULT_BACKUP_CRITICAL,
  DEFAULT_BACKUP_FRESH_HOURS,
  DEFAULT_BACKUP_STALE_HOURS,
} from '@/features/alerts/thresholds/constants';
import { normalizeDockerIgnoredInput, formatMetricValue } from '@/features/alerts/thresholds/helpers';
import { useThresholdsData } from '@/features/alerts/thresholds/hooks/useThresholdsData';

export function ThresholdsTable(props: ThresholdsTableProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const sectionTitles = getAlertThresholdsSectionTitles();

  // Collapsible section state management
  const { isCollapsed, toggleSection, expandAll, collapseAll } = useCollapsedSections();

  // Help banner dismiss state (persisted to localStorage)
  const HELP_BANNER_KEY = 'pulse-thresholds-help-dismissed';
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

  const [activeTab, setActiveTab] = createSignal<'proxmox' | 'pmg' | 'agents' | 'docker'>(
    'proxmox',
  );
  const guestFilterPresentation = getAlertThresholdsGuestFilterPresentation();
  const backupOrphanedPresentation = getAlertThresholdsBackupOrphanedPresentation();
  const dockerServicePresentation = getAlertThresholdsDockerServicePresentation();
  const dockerIgnoredPrefixesPresentation = getAlertThresholdsDockerIgnoredPrefixesPresentation();
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
      remote.every((val, i) => val === normalizedLocal[i]);

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

  // Determine active tab from URL
  const getActiveTabFromRoute = (): 'proxmox' | 'pmg' | 'agents' | 'docker' => {
    const path = location.pathname;
    if (path.includes('/thresholds/containers')) return 'docker';
    if (path.includes('/thresholds/agents')) return 'agents';
    if (path.includes('/thresholds/mail-gateway')) return 'pmg';
    return 'proxmox'; // default
  };

  // Sync active tab with route on mount and route changes
  createEffect(() => {
    const tabFromRoute = getActiveTabFromRoute();
    if (activeTab() !== tabFromRoute) {
      setActiveTab(tabFromRoute);
    }
  });

  // Handle default redirect - if at /alerts/thresholds exactly, redirect to /alerts/thresholds/proxmox
  createEffect(() => {
    if (location.pathname === '/alerts/thresholds') {
      navigate('/alerts/thresholds/proxmox', { replace: true });
    }
  });

  const handleTabClick = (tab: 'proxmox' | 'pmg' | 'agents' | 'docker') => {
    const tabRoutes = {
      proxmox: '/alerts/thresholds/proxmox',
      pmg: '/alerts/thresholds/mail-gateway',
      agents: '/alerts/thresholds/agents',
      docker: '/alerts/thresholds/containers',
    };
    navigate(tabRoutes[tab]);
  };

  const handleDockerIgnoredChange = (value: string) => {
    setDockerIgnoredInput(value);
    const normalized = normalizeDockerIgnoredInput(value);
    props.setDockerIgnoredPrefixes(normalized);
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

  // Check if there's an active alert for a resource/metric
  const hasActiveAlert = (resourceId: string, metric: string): boolean => {
    if (!alertsEnabled()) return false;
    if (!props.activeAlerts) return false;
    const alertKey = `${resourceId}-${metric}`;
    return alertKey in props.activeAlerts;
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

  const registerSection = (_key: string) => (_el: HTMLDivElement | null) => {
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

  const summaryItems = createMemo(() => {
    try {
      const items = [
        {
          key: 'nodes' as const,
          label: 'Nodes',
          total: props.nodes?.length ?? 0,
          overrides: countOverrides(nodesWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'dockerHosts' as const,
          label: 'Container Runtimes',
          total: props.dockerHosts?.length ?? 0,
          overrides: countOverrides(dockerHostsWithOverrides()),
          tab: 'docker' as const,
        },
        {
          key: 'agents' as const,
          label: 'Agents',
          total: props.agents?.length ?? 0,
          overrides: countOverrides(agentsWithOverrides()),
          tab: 'agents' as const,
        },
        {
          key: 'agentDisks' as const,
          label: 'Agent Disks',
          total: agentDisksWithOverrides().length,
          overrides: countOverrides(agentDisksWithOverrides()),
          tab: 'agents' as const,
        },
        {
          key: 'storage' as const,
          label: 'Storage',
          total: props.storage?.length ?? 0,
          overrides: countOverrides(storageWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'backups' as const,
          label: 'Recovery',
          total: 1,
          overrides: backupOverridesCount(),
          tab: 'proxmox' as const,
        },
        {
          key: 'snapshots' as const,
          label: 'Snapshot Age',
          total: 1,
          overrides: snapshotOverridesCount(),
          tab: 'proxmox' as const,
        },
        {
          key: 'pbs' as const,
          label: 'PBS Servers',
          total: props.pbsInstances?.length ?? 0,
          overrides: countOverrides(pbsServersWithOverrides()),
          tab: 'proxmox' as const,
        },
        {
          key: 'pmg' as const,
          label: 'Mail Gateways',
          total: props.pmgInstances?.length ?? 0,
          overrides: countOverrides(pmgServersWithOverrides()),
          tab: 'pmg' as const,
        },
        {
          key: 'dockerContainers' as const,
          label: 'Containers',
          total: totalDockerContainers() ?? 0,
          overrides: countOverrides(dockerContainersFlat()),
          tab: 'docker' as const,
        },
        {
          key: 'guests' as const,
          label: 'VMs & Containers',
          total: props.allGuests?.()?.length ?? 0,
          overrides: countOverrides(guestsFlat()),
          tab: 'proxmox' as const,
        },
      ];

      const filtered = items.filter((item) => item.total > 0 || item.overrides > 0);
      return filtered.filter((item) => item.tab === activeTab());
    } catch (err) {
      logger.error('Error in summaryItems memo:', err);
      return [];
    }
  });

  const hasSection = (key: string) => summaryItems()?.some((item) => item.key === key) ?? false;

  const startEditing = (
    resourceId: string,
    currentThresholds: Record<string, number | undefined>,
    defaults: Record<string, number | undefined>,
    note?: string,
  ) => {
    setEditingId(resourceId);
    // Merge defaults with overrides for editing
    const mergedThresholds = { ...defaults, ...currentThresholds };
    setEditingThresholds(mergedThresholds);
    setEditingNote(note ?? '');
  };

  const saveEdit = (resourceId: string) => {
    // Flatten grouped guests to find the resource
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const allResources = [
      ...nodesWithOverrides(),
      ...agentsWithOverrides(),
      ...agentDisksWithOverrides(),
      ...dockerHostsWithOverrides(),
      ...allGuests,
      ...allDockerContainers,
      ...storageWithOverrides(),
      ...pbsServersWithOverrides(),
    ];
    const resource = allResources.find((r) => r.id === resourceId);
    if (!resource) return;

    const editedThresholds = editingThresholds();
    const trimmedNote = editingNote().trim();
    const noteForOverride = trimmedNote.length > 0 ? trimmedNote : undefined;

    if (resource.editScope === 'backup') {
      const currentBackupDefaults = props.backupDefaults();
      const nextWarning =
        editedThresholds['warning days'] ??
        currentBackupDefaults.warningDays ??
        DEFAULT_BACKUP_WARNING;
      const nextCritical =
        editedThresholds['critical days'] ??
        currentBackupDefaults.criticalDays ??
        DEFAULT_BACKUP_CRITICAL;

      updateBackupDefaults({
        enabled: currentBackupDefaults.enabled,
        warningDays: nextWarning,
        criticalDays: nextCritical,
      });

      cancelEdit();
      return;
    }

    if (resource.editScope === 'snapshot') {
      const currentSnapshotDefaults = props.snapshotDefaults();
      const nextWarning =
        editedThresholds['warning days'] ??
        currentSnapshotDefaults.warningDays ??
        DEFAULT_SNAPSHOT_WARNING;
      const nextCritical =
        editedThresholds['critical days'] ??
        currentSnapshotDefaults.criticalDays ??
        DEFAULT_SNAPSHOT_CRITICAL;
      const nextWarningSize =
        editedThresholds['warning size (gib)'] ??
        currentSnapshotDefaults.warningSizeGiB ??
        DEFAULT_SNAPSHOT_WARNING_SIZE;
      const nextCriticalSize =
        editedThresholds['critical size (gib)'] ??
        currentSnapshotDefaults.criticalSizeGiB ??
        DEFAULT_SNAPSHOT_CRITICAL_SIZE;

      updateSnapshotDefaults({
        enabled: currentSnapshotDefaults.enabled,
        warningDays: nextWarning,
        criticalDays: nextCritical,
        warningSizeGiB: nextWarningSize,
        criticalSizeGiB: nextCriticalSize,
      });

      cancelEdit();
      return;
    }

    const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;

    // Only include values that differ from defaults
    const overrideThresholds: Record<string, number> = {};
    Object.keys(editedThresholds).forEach((key) => {
      const editedValue = editedThresholds[key];
      const defaultValue = defaultThresholds[key as keyof typeof defaultThresholds];
      if (editedValue !== undefined && editedValue !== defaultValue) {
        overrideThresholds[key] = editedValue;
      }
    });

    // Find existing override to check for backup/snapshot fields
    const existingOverrideCheck = props.overrides().find((o) => o.id === resourceId);

    const hasStateOnlyOverride = Boolean(
      resource.disabled ||
      resource.disableConnectivity ||
      resource.poweredOffSeverity !== undefined ||
      noteForOverride !== undefined ||
      existingOverrideCheck?.backup ||
      existingOverrideCheck?.snapshot,
    );

    // If no threshold overrides or state flags remain, remove the override entirely
    if (Object.keys(overrideThresholds).length === 0 && !hasStateOnlyOverride) {
      // If there was an existing override, remove it
      if (resource.hasOverride) {
        const newOverrides = props.overrides().filter((o) => o.id !== resourceId);
        props.setOverrides(newOverrides);

        // Also remove from raw config
        const newRawConfig = { ...props.rawOverridesConfig() };
        delete newRawConfig[resourceId];
        props.setRawOverridesConfig(newRawConfig);
        props.setHasUnsavedChanges(true);
      }
      cancelEdit();
      return;
    }

    // Create or update override
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
      backup: existingOverrideCheck?.backup,
      snapshot: existingOverrideCheck?.snapshot,
      thresholds: overrideThresholds,
    };

    // Update overrides list
    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    // Update raw config
    const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
    const previousRaw = props.rawOverridesConfig()[resourceId];
    const hysteresisThresholds: RawOverrideConfig = {};
    if (previousRaw) {
      if (previousRaw.disabled !== undefined) {
        hysteresisThresholds.disabled = previousRaw.disabled;
      }
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
          trigger: value,
          clear: Math.max(0, value - 5),
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
    if (previousRaw?.backup) {
      hysteresisThresholds.backup = previousRaw.backup;
    }
    if (previousRaw?.snapshot) {
      hysteresisThresholds.snapshot = previousRaw.snapshot;
    }
    newRawConfig[resourceId] = hysteresisThresholds;
    props.setRawOverridesConfig(newRawConfig);

    props.setHasUnsavedChanges(true);
    setEditingId(null);
    setEditingThresholds({});
    setEditingNote('');
  };

  const handleBulkEdit = (ids: string[], columns: string[]) => {
    setBulkEditIds(ids);
    setBulkEditColumns(columns);
    setIsBulkEditDialogOpen(true);
  };

  const handleSaveBulkEdit = (thresholds: Record<string, number | undefined>) => {
    setIsBulkEditDialogOpen(false);

    const newOverrides = [...props.overrides()];
    const newRawConfig = { ...props.rawOverridesConfig() };
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
      const resource = allResources.find((r) => r.id === id);
      if (!resource) continue;

      const defaultThresholds = (resource.defaults ?? {}) as Record<string, number | undefined>;
      const existingOverrideCheck = newOverrides.find((o) => o.id === id);
      const previousRaw = newRawConfig[id];

      // Merge current thresholds explicitly checking what differs from defaults
      const currentOverrides = existingOverrideCheck?.thresholds ?? {};
      const newThresholds: Record<string, number | undefined> = { ...currentOverrides };

      // Update with new bulk thresholds
      Object.keys(thresholds).forEach((key) => {
        if (thresholds[key] !== undefined) {
          const val = thresholds[key];
          if (val === defaultThresholds[key as keyof typeof defaultThresholds]) {
            delete newThresholds[key];
          } else {
            newThresholds[key] = val as number;
          }
        }
      });

      const hasStateOnlyOverride = Boolean(
        resource.disabled ||
        resource.disableConnectivity ||
        resource.poweredOffSeverity !== undefined ||
        existingOverrideCheck?.note !== undefined ||
        existingOverrideCheck?.backup ||
        existingOverrideCheck?.snapshot,
      );

      // If no override fields remain, remove entirely
      if (Object.keys(newThresholds).length === 0 && !hasStateOnlyOverride) {
        if (resource.hasOverride) {
          const idx = newOverrides.findIndex((o) => o.id === id);
          if (idx !== -1) newOverrides.splice(idx, 1);
          delete newRawConfig[id];
        }
        continue;
      }

      // Create new override
      const override: Override = {
        id: id,
        name: resource.name,
        type: resource.type as OverrideType,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? resource.instance : undefined,
        disabled: resource.disabled,
        disableConnectivity: resource.disableConnectivity,
        poweredOffSeverity: resource.poweredOffSeverity,
        note: existingOverrideCheck?.note,
        backup: existingOverrideCheck?.backup,
        snapshot: existingOverrideCheck?.snapshot,
        thresholds: newThresholds,
      };

      // Update overrides
      const existingIndex = newOverrides.findIndex((o) => o.id === id);
      if (existingIndex >= 0) {
        newOverrides[existingIndex] = override;
      } else {
        newOverrides.push(override);
      }

      // Update raw config
      const hysteresisThresholds: RawOverrideConfig = {};
      if (previousRaw) {
        if (previousRaw.disabled !== undefined)
          hysteresisThresholds.disabled = previousRaw.disabled;
        if (previousRaw.disableConnectivity !== undefined)
          hysteresisThresholds.disableConnectivity = previousRaw.disableConnectivity;
        if (previousRaw.poweredOffSeverity !== undefined)
          hysteresisThresholds.poweredOffSeverity = previousRaw.poweredOffSeverity;
        if (previousRaw.note !== undefined) hysteresisThresholds.note = previousRaw.note;
        if (previousRaw.backup !== undefined) hysteresisThresholds.backup = previousRaw.backup;
        if (previousRaw.snapshot !== undefined)
          hysteresisThresholds.snapshot = previousRaw.snapshot;
      }

      Object.entries(newThresholds).forEach(([metric, value]) => {
        if (value !== undefined && value !== null) {
          hysteresisThresholds[metric] = {
            trigger: value,
            clear: Math.max(0, value - 5),
          };
        }
      });

      newRawConfig[id] = hysteresisThresholds;
    }

    props.setOverrides(newOverrides);
    props.setRawOverridesConfig(newRawConfig);
    props.setHasUnsavedChanges(true);

    // Clear bulk edit state
    setBulkEditIds([]);
    setBulkEditColumns([]);
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditingThresholds({});
    setEditingNote('');
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
    props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

    const newRawConfig = { ...props.rawOverridesConfig() };
    delete newRawConfig[resourceId];
    props.setRawOverridesConfig(newRawConfig);

    props.setHasUnsavedChanges(true);
  };

  const toggleBackup = (resourceId: string, forceState?: boolean) => {
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const resource = [...allGuests, ...allDockerContainers].find((r) => r.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const existingOverride = props.overrides().find((o) => o.id === resourceId);
    const baseConfig = existingOverride?.backup || props.backupDefaults();
    const newEnabled = forceState !== undefined ? forceState : !baseConfig.enabled;
    const newBackup = { ...baseConfig, enabled: newEnabled };

    const override: Override = {
      ...(existingOverride || {
        id: resourceId,
        name: resource.name,
        type: resource.type as any,
        vmid: 'vmid' in resource ? (resource as any).vmid : undefined,
        node: 'node' in resource ? (resource as any).node : undefined,
        instance: 'instance' in resource ? (resource as any).instance : undefined,
        thresholds: {},
      }),
      backup: newBackup,
    };

    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
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
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const resource = [...allGuests, ...allDockerContainers].find((r) => r.id === resourceId);
    if (!resource || (resource.type !== 'guest' && resource.type !== 'dockerContainer')) return;

    const existingOverride = props.overrides().find((o) => o.id === resourceId);
    const baseConfig = existingOverride?.snapshot || props.snapshotDefaults();
    const newEnabled = forceState !== undefined ? forceState : !baseConfig.enabled;
    const newSnapshot = { ...baseConfig, enabled: newEnabled };

    const override: Override = {
      ...(existingOverride || {
        id: resourceId,
        name: resource.name,
        type: resource.type as any,
        vmid: 'vmid' in resource ? (resource as any).vmid : undefined,
        node: 'node' in resource ? (resource as any).node : undefined,
        instance: 'instance' in resource ? (resource as any).instance : undefined,
        thresholds: {},
      }),
      snapshot: newSnapshot,
    };

    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
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
    // Flatten grouped guests to find the resource
    const allGuests = guestsFlat();
    const allDockerContainers = dockerContainersFlat();
    const allResources = [
      ...allGuests,
      ...allDockerContainers,
      ...storageWithOverrides(),
      ...pbsServersWithOverrides(),
      ...agentsWithOverrides(),
      ...agentDisksWithOverrides(),
    ];
    const resource = allResources.find((r) => r.id === resourceId);
    if (
      !resource ||
      (resource.type !== 'guest' &&
        resource.type !== 'storage' &&
        resource.type !== 'pbs' &&
        resource.type !== 'dockerContainer' &&
        resource.type !== 'agent' &&
        resource.type !== 'agentDisk')
    )
      return;

    // Get existing override if it exists
    const existingOverride = props.overrides().find((o) => o.id === resourceId);

    // Determine the current disabled state - check the resource's current state, not the override
    const currentDisabledState = resource.disabled;
    const newDisabledState = forceState !== undefined ? forceState : !currentDisabledState;

    // Clean the thresholds to exclude 'disabled' if it got in there
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;

    // If enabling (disabled = false) and no custom thresholds exist, remove the override entirely
    if (!newDisabledState && (!existingOverride || Object.keys(cleanThresholds).length === 0)) {
      // Remove the override completely
      props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

      // Remove from raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      const override: Override = {
        id: resourceId,
        name: resource.name,
        type: resource.type,
        resourceType: resource.resourceType,
        vmid: 'vmid' in resource ? resource.vmid : undefined,
        node: 'node' in resource ? resource.node : undefined,
        instance: 'instance' in resource ? (resource as any).instance : undefined,
        disabled: newDisabledState,
        disableConnectivity: existingOverride?.disableConnectivity,
        poweredOffSeverity: existingOverride?.poweredOffSeverity,
        backup: existingOverride?.backup,
        snapshot: existingOverride?.snapshot,
        thresholds: cleanThresholds, // Only keep actual threshold overrides
      };

      const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
      if (existingIndex >= 0) {
        const newOverrides = [...props.overrides()];
        newOverrides[existingIndex] = override;
        props.setOverrides(newOverrides);
      } else {
        props.setOverrides([...props.overrides(), override]);
      }

      // Update raw config
      const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
      const hysteresisThresholds: RawOverrideConfig = {};

      // Only add threshold overrides that differ from defaults
      Object.entries(override.thresholds).forEach(([metric, value]) => {
        if (typeof value === 'number') {
          hysteresisThresholds[metric] = {
            trigger: value,
            clear: Math.max(0, value - 5),
          };
        }
      });

      if (newDisabledState) {
        hysteresisThresholds.disabled = true;
      } else {
        delete hysteresisThresholds.disabled;
      }

      if (override.backup) {
        hysteresisThresholds.backup = override.backup;
      }
      if (override.snapshot) {
        hysteresisThresholds.snapshot = override.snapshot;
      }
      if (override.disableConnectivity) {
        hysteresisThresholds.disableConnectivity = true;
      }
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
    // Find the resource - could be a node, PBS server, or guest
    const nodes = nodesWithOverrides();
    const pbsServers = pbsServersWithOverrides();
    const guests = guestsFlat();
    const agents = agentsWithOverrides();
    const dockerHosts = dockerHostsWithOverrides();
    const resource = [...nodes, ...pbsServers, ...guests, ...agents, ...dockerHosts].find(
      (r) => r.id === resourceId,
    );
    if (
      !resource ||
      (resource.type !== 'agent' &&
        resource.type !== 'pbs' &&
        resource.type !== 'guest' &&
        resource.type !== 'agent' &&
        resource.type !== 'dockerHost')
    )
      return;

    // Get existing override if it exists
    const existingOverride = props.overrides().find((o) => o.id === resourceId);

    // Determine the current state - use the resource's computed state, not just the override
    const currentDisableConnectivity = resource.disableConnectivity;
    const newDisableConnectivity =
      forceState !== undefined ? forceState : !currentDisableConnectivity;

    // Clean the thresholds to exclude any unwanted fields
    const cleanThresholds: Record<string, number> = { ...(existingOverride?.thresholds || {}) };
    delete (cleanThresholds as Record<string, unknown>).disabled;
    delete (cleanThresholds as Record<string, unknown>).disableConnectivity;

    // If enabling connectivity alerts (disableConnectivity = false) and no custom thresholds exist, remove the override entirely
    if (!newDisableConnectivity && Object.keys(cleanThresholds).length === 0) {
      // Remove the override completely
      props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));

      // Remove from raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      delete newRawConfig[resourceId];
      props.setRawOverridesConfig(newRawConfig);
    } else {
      // Update or create the override
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

      // Update overrides list
      const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
      if (existingIndex >= 0) {
        const newOverrides = [...props.overrides()];
        newOverrides[existingIndex] = override;
        props.setOverrides(newOverrides);
      } else {
        props.setOverrides([...props.overrides(), override]);
      }

      // Update raw config
      const newRawConfig = { ...props.rawOverridesConfig() };
      const hysteresisThresholds: Record<string, any> = {};

      // Add threshold configs
      Object.entries(cleanThresholds).forEach(([metric, value]) => {
        if (value !== undefined && value !== null) {
          hysteresisThresholds[metric] = {
            trigger: value,
            clear: Math.max(0, (value as number) - 5),
          };
        }
      });

      if (newDisableConnectivity) {
        hysteresisThresholds.disableConnectivity = true;
      } else {
        delete hysteresisThresholds.disableConnectivity;
      }

      if (override.backup) {
        hysteresisThresholds.backup = override.backup;
      }
      if (override.snapshot) {
        hysteresisThresholds.snapshot = override.snapshot;
      }
      if (override.disabled) {
        hysteresisThresholds.disabled = true;
      }
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
    const guests = guestsFlat();
    const dockerContainers = dockerContainersFlat();
    const resource = [...guests, ...dockerContainers].find((r) => r.id === resourceId);
    if (!resource) return;

    const isDockerContainer = resource.type === 'dockerContainer';
    const defaultDisabled = isDockerContainer
      ? props.dockerDisableConnectivity()
      : props.guestDisableConnectivity();
    const defaultSeverity = isDockerContainer
      ? props.dockerPoweredOffSeverity()
      : props.guestPoweredOffSeverity();

    const existingOverride = props.overrides().find((o) => o.id === resourceId);
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
      // Remove override entirely
      if (existingOverride) {
        props.setOverrides(props.overrides().filter((o) => o.id !== resourceId));
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

    const existingIndex = props.overrides().findIndex((o) => o.id === resourceId);
    if (existingIndex >= 0) {
      const newOverrides = [...props.overrides()];
      newOverrides[existingIndex] = override;
      props.setOverrides(newOverrides);
    } else {
      props.setOverrides([...props.overrides(), override]);
    }

    const newRawConfig: Record<string, RawOverrideConfig> = { ...props.rawOverridesConfig() };
    const hysteresisThresholds: RawOverrideConfig = {};

    Object.entries(cleanThresholds).forEach(([metric, value]) => {
      if (value !== undefined && value !== null) {
        hysteresisThresholds[metric] = {
          trigger: value,
          clear: Math.max(0, value - 5),
        };
      }
    });

    if (overrideDisabled) {
      hysteresisThresholds.disabled = true;
    }

    if (newDisableConnectivity) {
      hysteresisThresholds.disableConnectivity = true;
    } else {
      if (defaultDisabled) {
        hysteresisThresholds.disableConnectivity = false;
      }
      if (newSeverity) {
        hysteresisThresholds.poweredOffSeverity = newSeverity;
      }
    }

    if (override.backup) {
      hysteresisThresholds.backup = override.backup;
    }
    if (override.snapshot) {
      hysteresisThresholds.snapshot = override.snapshot;
    }

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

  return (
    <div class="space-y-4">
      {/* Search Bar */}
      <div class="relative">
        <SearchInput
          value={searchTerm}
          onChange={setSearchTerm}
          placeholder={getAlertThresholdsSearchPlaceholder()}
          class="w-full"
          onBeforeAutoFocus={() => Boolean(editingId())}
          focusOnShortcut
          clearOnEscape
          shortcutHint="Ctrl+F"
        />
      </div>

      {/* Help Banner - Dismissible */}
      <Show when={!helpBannerDismissed()}>
        <div class="rounded-md border border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-900 p-3 relative group">
          <button
            type="button"
            onClick={dismissHelpBanner}
            class="absolute top-2 right-2 p-1 rounded-md text-blue-400 hover:text-blue-600 dark:text-blue-500 dark:hover:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-900 opacity-0 group-hover:opacity-100 transition-opacity"
            title={getAlertThresholdsHelpDismissLabel()}
            aria-label={getAlertThresholdsHelpDismissLabel()}
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
          <div class="flex items-start gap-2 pr-6">
            <svg
              class="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <div class="text-sm text-blue-900 dark:text-blue-100">
              <span class="font-medium">{getAlertThresholdsHelpBanner().title}</span> Set any
              threshold to{' '}
              <code class="px-1 py-0.5 bg-blue-100 dark:bg-blue-900 rounded text-xs font-mono">
                {getAlertThresholdsHelpBanner().disableValue}
              </code>{' '}
              to disable alerts for that metric. Click on disabled thresholds showing{' '}
              <span class="italic">{getAlertThresholdsHelpBanner().reenableLabel}</span> to
              re-enable them. Resources with custom settings show a{' '}
              <span class="inline-flex items-center px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded text-xs">
                {getAlertThresholdsHelpBanner().customBadgeLabel}
              </span>{' '}
              badge.{' '}
              <span class="text-blue-600 dark:text-blue-400">
                {getAlertThresholdsHelpBanner().collapseHint}
              </span>
            </div>
          </div>
        </div>
      </Show>

      {/* Tab Navigation */}
      <div class="border-b border-border">
        <nav class="-mb-px flex gap-4 sm:gap-6" aria-label="Tabs">
          <button
            type="button"
            onClick={() => handleTabClick('proxmox')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'proxmox' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Server class="w-4 h-4" />
            <span class="hidden sm:inline">Proxmox / PBS</span>
            <span class="sm:hidden">Proxmox</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('pmg')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'pmg' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Mail class="w-4 h-4" />
            <span class="hidden sm:inline">Mail Gateway</span>
            <span class="sm:hidden">Mail</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('agents')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'agents' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Users class="w-4 h-4" />
            <span>Agents</span>
          </button>
          <button
            type="button"
            onClick={() => handleTabClick('docker')}
            class={`py-3 px-1 border-b-2 font-medium text-sm transition-colors cursor-pointer flex items-center gap-1.5 ${activeTab() === 'docker' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-base-content hover:border-slate-300'}`}
          >
            <Boxes class="w-4 h-4" />
            <span>Containers</span>
          </button>
        </nav>
      </div>

      {/* Section Controls - Only show on Proxmox tab which has multiple sections */}
      <Show when={activeTab() === 'proxmox'}>
        <div class="flex justify-end gap-2">
          <button
            type="button"
            onClick={expandAll}
            class="text-xs px-2 py-1 hover:text-muted hover:bg-surface-hover rounded transition-colors"
          >
            Expand all
          </button>
          <span class="text-muted">|</span>
          <button
            type="button"
            onClick={collapseAll}
            class="text-xs px-2 py-1 hover:text-muted hover:bg-surface-hover rounded transition-colors"
          >
            Collapse all
          </button>
        </div>
      </Show>

      <div class="space-y-6">
        <Show when={activeTab() === 'proxmox'}>
          <Show when={hasSection('nodes')}>
            <CollapsibleSection
              id="nodes"
              title={sectionTitles.nodes}
              resourceCount={nodesWithOverrides().length}
              collapsed={isCollapsed('nodes')}
              onToggle={() => toggleSection('nodes')}
              icon={<Server class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllNodes()}
              emptyMessage={NODE_THRESHOLDS_FILTER_EMPTY_STATE}
            >
              <div ref={registerSection('nodes')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={nodesWithOverrides()}
                  columns={['CPU %', 'Memory %', 'Disk %', 'Temp °C']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={NODE_THRESHOLDS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) =>
                    handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])
                  }
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.nodeDefaults}
                  setGlobalDefaults={props.setNodeDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllNodes}
                  onToggleGlobalDisable={() => props.setDisableAllNodes(!props.disableAllNodes())}
                  globalDisableOfflineFlag={props.disableAllNodesOffline}
                  onToggleGlobalDisableOffline={() =>
                    props.setDisableAllNodesOffline(!props.disableAllNodesOffline())
                  }
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().node}
                  metricDelaySeconds={props.metricTimeThresholds().node ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('node', metric, value)}
                  factoryDefaults={props.factoryNodeDefaults}
                  onResetDefaults={props.resetNodeDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('pbs')}>
            <CollapsibleSection
              id="pbs"
              title={sectionTitles.pbs}
              resourceCount={pbsServersWithOverrides().length}
              collapsed={isCollapsed('pbs')}
              onToggle={() => toggleSection('pbs')}
              icon={<Database class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllPBS()}
              emptyMessage={PBS_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('pbs')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={pbsServersWithOverrides()}
                  columns={['CPU %', 'Memory %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={PBS_THRESHOLDS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) =>
                    handleBulkEdit(ids, [
                      'CPU %',
                      'Memory %',
                      'Disk R MB/s',
                      'Disk W MB/s',
                      'Net In MB/s',
                      'Net Out MB/s',
                    ])
                  }
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.pbsDefaults ?? { cpu: 80, memory: 85 }}
                  setGlobalDefaults={props.setPBSDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllPBS}
                  onToggleGlobalDisable={() => props.setDisableAllPBS(!props.disableAllPBS())}
                  globalDisableOfflineFlag={props.disableAllPBSOffline}
                  onToggleGlobalDisableOffline={() =>
                    props.setDisableAllPBSOffline(!props.disableAllPBSOffline())
                  }
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().pbs}
                  metricDelaySeconds={props.metricTimeThresholds().pbs ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('pbs', metric, value)}
                  factoryDefaults={props.factoryPBSDefaults}
                  onResetDefaults={props.resetPBSDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('guests')}>
            <CollapsibleSection
              id="guests"
              title={sectionTitles.guests}
              resourceCount={props.allGuests().length}
              collapsed={isCollapsed('guests')}
              onToggle={() => toggleSection('guests')}
              icon={<Monitor class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllGuests()}
              emptyMessage={GUEST_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('guests')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={guestsGroupedByNode()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
                  columns={[
                    'CPU %',
                    'Memory %',
                    'Disk %',
                    'Backup',
                    'Snapshot',
                    'Disk R MB/s',
                    'Disk W MB/s',
                    'Net In MB/s',
                    'Net Out MB/s',
                  ]}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={GUEST_THRESHOLDS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  onToggleNodeConnectivity={toggleNodeConnectivity}
                  onToggleBackup={toggleBackup}
                  onToggleSnapshot={toggleSnapshot}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) =>
                    handleBulkEdit(ids, [
                      'CPU %',
                      'Memory %',
                      'Disk %',
                      'Disk R MB/s',
                      'Disk W MB/s',
                      'Net In MB/s',
                      'Net Out MB/s',
                    ])
                  }
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={props.guestDefaults}
                  setGlobalDefaults={props.setGuestDefaults}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllGuests}
                  onToggleGlobalDisable={() => props.setDisableAllGuests(!props.disableAllGuests())}
                  globalDisableOfflineFlag={() => props.guestDisableConnectivity()}
                  onToggleGlobalDisableOffline={() =>
                    props.setGuestDisableConnectivity(!props.guestDisableConnectivity())
                  }
                  globalOfflineSeverity={props.guestPoweredOffSeverity()}
                  onSetGlobalOfflineState={(state) => {
                    if (state === 'off') {
                      props.setGuestDisableConnectivity(true);
                    } else {
                      props.setGuestDisableConnectivity(false);
                      props.setGuestPoweredOffSeverity(
                        state === 'critical' ? 'critical' : 'warning',
                      );
                    }
                    props.setHasUnsavedChanges(true);
                  }}
                  onSetOfflineState={setOfflineState}
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().guest}
                  metricDelaySeconds={props.metricTimeThresholds().guest ?? {}}
                  onMetricDelayChange={(metric, value) => updateMetricDelay('guest', metric, value)}
                  factoryDefaults={props.factoryGuestDefaults}
                  onResetDefaults={props.resetGuestDefaults}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={activeTab() === 'proxmox'}>
            <CollapsibleSection
              id="guest-filtering"
              title={sectionTitles.guestFiltering}
              collapsed={isCollapsed('guest-filtering')}
              onToggle={() => toggleSection('guest-filtering')}
              icon={<Monitor class="w-5 h-5" />}
              emptyMessage={GUEST_FILTERING_EMPTY_STATE}
            >
              <div class="grid grid-cols-1 gap-6 p-4 xl:grid-cols-3">
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-base-content">
                      {guestFilterPresentation.ignoredPrefixes.title}
                    </h3>
                    <p class="text-xs text-muted">
                      {guestFilterPresentation.ignoredPrefixes.description}
                    </p>
                  </div>
                  <TagInput
                    tags={props.ignoredGuestPrefixes()}
                    onChange={(tags) => {
                      props.setIgnoredGuestPrefixes(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder={guestFilterPresentation.ignoredPrefixes.placeholder}
                  />
                </Card>
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-base-content">
                      {guestFilterPresentation.tagWhitelist.title}
                    </h3>
                    <p class="text-xs text-muted">
                      {guestFilterPresentation.tagWhitelist.description}
                    </p>
                  </div>
                  <TagInput
                    tags={props.guestTagWhitelist()}
                    onChange={(tags) => {
                      props.setGuestTagWhitelist(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder={guestFilterPresentation.tagWhitelist.placeholder}
                  />
                </Card>
                <Card padding="md" tone="card">
                  <div class="mb-2">
                    <h3 class="text-sm font-semibold text-base-content">
                      {guestFilterPresentation.tagBlacklist.title}
                    </h3>
                    <p class="text-xs text-muted">
                      {guestFilterPresentation.tagBlacklist.description}
                    </p>
                  </div>
                  <TagInput
                    tags={props.guestTagBlacklist()}
                    onChange={(tags) => {
                      props.setGuestTagBlacklist(tags);
                      props.setHasUnsavedChanges(true);
                    }}
                    placeholder={guestFilterPresentation.tagBlacklist.placeholder}
                  />
                </Card>
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('backups')}>
            <CollapsibleSection
              id="backups"
              title={sectionTitles.backups}
              collapsed={isCollapsed('backups')}
              onToggle={() => toggleSection('backups')}
              icon={<Archive class="w-5 h-5" />}
              isGloballyDisabled={!props.backupDefaults().enabled}
              emptyMessage={BACKUP_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('backups')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={[
                    {
                      id: 'backups-defaults',
                      name: 'Global Defaults',
                      thresholds: backupDefaultsRecord(),
                      defaults: backupDefaultsRecord(),
                      editable: true,
                      editScope: 'backup',
                    },
                  ]}
                  columns={[
                    'Fresh Hours',
                    'Stale Hours',
                    'Warning Days',
                    'Critical Days',
                    'Warning Size (GiB)',
                    'Critical Size (GiB)',
                  ]}
                  activeAlerts={props.activeAlerts}
                  emptyMessage=""
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={backupDefaultsRecord()}
                  setGlobalDefaults={(value) => {
                    updateBackupDefaults((prev) => {
                      const currentRecord = {
                        'fresh hours': prev.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
                        'stale hours': prev.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
                        'warning days': prev.warningDays ?? 0,
                        'critical days': prev.criticalDays ?? 0,
                      };
                      const nextRecord =
                        typeof value === 'function'
                          ? value(currentRecord)
                          : { ...currentRecord, ...value };
                      return {
                        ...prev,
                        freshHours:
                          typeof nextRecord['fresh hours'] === 'number'
                            ? nextRecord['fresh hours']
                            : prev.freshHours,
                        staleHours:
                          typeof nextRecord['stale hours'] === 'number'
                            ? nextRecord['stale hours']
                            : prev.staleHours,
                        warningDays:
                          typeof nextRecord['warning days'] === 'number'
                            ? nextRecord['warning days']
                            : prev.warningDays,
                        criticalDays:
                          typeof nextRecord['critical days'] === 'number'
                            ? nextRecord['critical days']
                            : prev.criticalDays,
                      };
                    });
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={() => !props.backupDefaults().enabled}
                  onToggleGlobalDisable={() =>
                    updateBackupDefaults((prev) => ({
                      ...prev,
                      enabled: !prev.enabled,
                    }))
                  }
                  factoryDefaults={backupFactoryDefaultsRecord()}
                  onResetDefaults={() => {
                    if (props.resetBackupDefaults) {
                      props.resetBackupDefaults();
                      props.setHasUnsavedChanges(true);
                    } else {
                      updateBackupDefaults(backupFactoryConfig());
                    }
                  }}
                />
                <Card padding="md" tone="card" class="mt-6">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <h3 class="text-sm font-semibold text-base-content">
                        {backupOrphanedPresentation.title}
                      </h3>
                      <p class="mt-1 text-xs text-muted">
                        {backupOrphanedPresentation.description}
                      </p>
                    </div>
                    <Toggle
                      checked={props.backupDefaults().alertOrphaned ?? true}
                      onToggle={() =>
                        updateBackupDefaults((prev) => ({
                          ...prev,
                          alertOrphaned: !(prev.alertOrphaned ?? true),
                        }))
                      }
                      label={
                        <span class="text-sm font-medium text-base-content">
                          {backupOrphanedPresentation.toggleLabel}
                        </span>
                      }
                      description={
                        <span class="text-xs text-muted">
                          {backupOrphanedPresentation.toggleDescription}
                        </span>
                      }
                      size="sm"
                    />
                  </div>
                  <div class="mt-4">
                    <label class="text-xs font-medium uppercase tracking-wide text-muted">
                      {backupOrphanedPresentation.ignoreVmidsLabel}
                    </label>
                    <p class="mt-1 text-xs text-muted">
                      {backupOrphanedPresentation.ignoreVmidsDescription}
                    </p>
                    <TagInput
                      tags={props.backupDefaults().ignoreVMIDs ?? []}
                      onChange={(tags) => {
                        updateBackupDefaults((prev) => ({ ...prev, ignoreVMIDs: tags }));
                        props.setHasUnsavedChanges(true);
                      }}
                      placeholder={backupOrphanedPresentation.ignoreVmidsPlaceholder}
                    />
                  </div>
                </Card>
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('snapshots')}>
            <CollapsibleSection
              id="snapshots"
              title={sectionTitles.snapshots}
              collapsed={isCollapsed('snapshots')}
              onToggle={() => toggleSection('snapshots')}
              icon={<Camera class="w-5 h-5" />}
              isGloballyDisabled={!props.snapshotDefaults().enabled}
              emptyMessage={SNAPSHOT_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('snapshots')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  resources={[
                    {
                      id: 'snapshots-defaults',
                      name: 'Global Defaults',
                      thresholds: snapshotDefaultsRecord(),
                      defaults: snapshotDefaultsRecord(),
                      editable: true,
                      editScope: 'snapshot',
                    },
                  ]}
                  columns={['Warning Days', 'Critical Days']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage=""
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  showOfflineAlertsColumn={true}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %', 'Temperature °C'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={snapshotDefaultsRecord()}
                  setGlobalDefaults={(value) => {
                    updateSnapshotDefaults((prev) => {
                      const currentRecord = {
                        'warning days': prev.warningDays ?? 0,
                        'critical days': prev.criticalDays ?? 0,
                        'warning size (gib)': prev.warningSizeGiB ?? 0,
                        'critical size (gib)': prev.criticalSizeGiB ?? 0,
                      };
                      const nextRecord =
                        typeof value === 'function'
                          ? value(currentRecord)
                          : { ...currentRecord, ...value };
                      return {
                        ...prev,
                        warningDays:
                          typeof nextRecord['warning days'] === 'number'
                            ? nextRecord['warning days']
                            : prev.warningDays,
                        criticalDays:
                          typeof nextRecord['critical days'] === 'number'
                            ? nextRecord['critical days']
                            : prev.criticalDays,
                        warningSizeGiB:
                          typeof nextRecord['warning size (gib)'] === 'number'
                            ? nextRecord['warning size (gib)']
                            : prev.warningSizeGiB,
                        criticalSizeGiB:
                          typeof nextRecord['critical size (gib)'] === 'number'
                            ? nextRecord['critical size (gib)']
                            : prev.criticalSizeGiB,
                      };
                    });
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={() => !props.snapshotDefaults().enabled}
                  onToggleGlobalDisable={() =>
                    updateSnapshotDefaults((prev) => ({
                      ...prev,
                      enabled: !prev.enabled,
                    }))
                  }
                  factoryDefaults={snapshotFactoryDefaultsRecord()}
                  onResetDefaults={() => {
                    if (props.resetSnapshotDefaults) {
                      props.resetSnapshotDefaults();
                      props.setHasUnsavedChanges(true);
                    } else {
                      updateSnapshotDefaults(snapshotFactoryConfig());
                    }
                  }}
                />
              </div>
            </CollapsibleSection>
          </Show>

          <Show when={hasSection('storage')}>
            <CollapsibleSection
              id="storage"
              title={sectionTitles.storage}
              resourceCount={props.storage.length}
              collapsed={isCollapsed('storage')}
              onToggle={() => toggleSection('storage')}
              icon={<HardDrive class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllStorage()}
              emptyMessage={STORAGE_THRESHOLDS_EMPTY_STATE}
            >
              <div ref={registerSection('storage')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={storageGroupedByNode()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
                  columns={['Usage %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={STORAGE_THRESHOLDS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  showOfflineAlertsColumn={false}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) => handleBulkEdit(ids, ['Usage %'])}
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={{ usage: props.storageDefault() }}
                  setGlobalDefaults={(value) => {
                    if (typeof value === 'function') {
                      const newValue = value({ usage: props.storageDefault() });
                      props.setStorageDefault(newValue.usage ?? 85);
                    } else {
                      props.setStorageDefault(value.usage ?? 85);
                    }
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                  globalDisableFlag={props.disableAllStorage}
                  onToggleGlobalDisable={() =>
                    props.setDisableAllStorage(!props.disableAllStorage())
                  }
                  showDelayColumn={true}
                  globalDelaySeconds={props.timeThresholds().storage}
                  metricDelaySeconds={props.metricTimeThresholds().storage ?? {}}
                  onMetricDelayChange={(metric, value) =>
                    updateMetricDelay('storage', metric, value)
                  }
                  factoryDefaults={
                    props.factoryStorageDefault !== undefined
                      ? { usage: props.factoryStorageDefault }
                      : undefined
                  }
                  onResetDefaults={props.resetStorageDefault}
                />
              </div>
            </CollapsibleSection>
          </Show>
        </Show>

        <Show when={activeTab() === 'pmg'}>
          <Show
            when={pmgServersWithOverrides().length > 0}
            fallback={
              <div class="rounded-md border border-border bg-surface p-6 text-sm text-muted">
                {PMG_THRESHOLDS_EMPTY_STATE}
              </div>
            }
          >
            <div ref={registerSection('pmg')} class="scroll-mt-24">
              <ResourceTable
                title={sectionTitles.pmg}
                resources={pmgServersWithOverrides()}
                columns={[
                  'Queue Warn',
                  'Queue Crit',
                  'Deferred Warn',
                  'Deferred Crit',
                  'Hold Warn',
                  'Hold Crit',
                  'Oldest Warn (min)',
                  'Oldest Crit (min)',
                  'Spam Warn',
                  'Spam Crit',
                  'Virus Warn',
                  'Virus Crit',
                  'Growth Warn %',
                  'Growth Warn Min',
                  'Growth Crit %',
                  'Growth Crit Min',
                ]}
                activeAlerts={props.activeAlerts}
                emptyMessage={PMG_THRESHOLDS_FILTER_EMPTY_STATE}
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) => handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %'])}
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={pmgGlobalDefaults()}
                setGlobalDefaults={setPMGGlobalDefaults}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllPMG}
                onToggleGlobalDisable={() => props.setDisableAllPMG(!props.disableAllPMG())}
                globalDisableOfflineFlag={props.disableAllPMGOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllPMGOffline(!props.disableAllPMGOffline())
                }
              />
            </div>
          </Show>
        </Show>

        <Show when={activeTab() === 'agents'}>
          <Show when={hasSection('agents')}>
            <div ref={registerSection('agents')} class="scroll-mt-24">
              <ResourceTable
                title={sectionTitles.agents}
                resources={agentsWithOverrides()}
                columns={['CPU %', 'Memory %', 'Disk %', 'Disk Temp °C']}
                activeAlerts={props.activeAlerts}
                emptyMessage={AGENT_THRESHOLDS_FILTER_EMPTY_STATE}
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) =>
                  handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])
                }
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={props.agentDefaults}
                setGlobalDefaults={props.setAgentDefaults}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllAgents}
                onToggleGlobalDisable={() => props.setDisableAllAgents(!props.disableAllAgents())}
                globalDisableOfflineFlag={props.disableAllAgentsOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllAgentsOffline(!props.disableAllAgentsOffline())
                }
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().agent}
                metricDelaySeconds={props.metricTimeThresholds().agent ?? {}}
                onMetricDelayChange={(metric, value) => updateMetricDelay('agent', metric, value)}
                factoryDefaults={props.factoryAgentDefaults}
                onResetDefaults={props.resetAgentDefaults}
              />
            </div>
          </Show>

          <Show when={hasSection('agentDisks')}>
            <CollapsibleSection
              id="agentDisks"
              title={sectionTitles.agentDisks}
              resourceCount={agentDisksWithOverrides().length}
              collapsed={isCollapsed('agentDisks')}
              onToggle={() => toggleSection('agentDisks')}
              icon={<HardDrive class="w-5 h-5" />}
              isGloballyDisabled={props.disableAllAgents()}
              emptyMessage={AGENT_DISKS_EMPTY_STATE}
            >
              <div ref={registerSection('agentDisks')} class="scroll-mt-24">
                <ResourceTable
                  title=""
                  groupedResources={agentDisksGroupedByAgent()}
                  groupHeaderMeta={guestGroupHeaderMeta()}
                  columns={['Disk %']}
                  activeAlerts={props.activeAlerts}
                  emptyMessage={AGENT_DISKS_FILTER_EMPTY_STATE}
                  onEdit={startEditing}
                  onSaveEdit={saveEdit}
                  onCancelEdit={cancelEdit}
                  onRemoveOverride={removeOverride}
                  onToggleDisabled={toggleDisabled}
                  showOfflineAlertsColumn={false}
                  editingId={editingId}
                  editingThresholds={editingThresholds}
                  setEditingThresholds={setEditingThresholds}
                  editingNote={editingNote}
                  setEditingNote={setEditingNote}
                  onBulkEdit={(ids) =>
                    handleBulkEdit(ids, [
                      'CPU %',
                      'Memory %',
                      'Disk R MB/s',
                      'Disk W MB/s',
                      'Net In MB/s',
                      'Net Out MB/s',
                    ])
                  }
                  formatMetricValue={formatMetricValue}
                  hasActiveAlert={hasActiveAlert}
                  globalDefaults={{ disk: props.agentDefaults.disk }}
                  setGlobalDefaults={(value) => {
                    if (typeof value === 'function') {
                      const newValue = value({ disk: props.agentDefaults.disk });
                      props.setAgentDefaults((prev) => ({ ...prev, disk: newValue.disk }));
                    } else {
                      props.setAgentDefaults((prev) => ({ ...prev, disk: value.disk }));
                    }
                  }}
                  setHasUnsavedChanges={props.setHasUnsavedChanges}
                />
              </div>
            </CollapsibleSection>
          </Show>
        </Show>

        <Show when={activeTab() === 'docker'}>
          <Card padding="md" tone="card" class="mb-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-base-content">
                  {dockerIgnoredPrefixesPresentation.title}
                </h3>
                <p class="mt-1 text-xs text-muted">
                  {dockerIgnoredPrefixesPresentation.description}
                </p>
              </div>
              <Show when={(props.dockerIgnoredPrefixes().length ?? 0) > 0}>
                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-md border border-transparent px-3 py-1 text-xs font-medium transition hover:bg-surface-alt"
                  onClick={handleResetDockerIgnored}
                >
                  {dockerIgnoredPrefixesPresentation.resetLabel}
                </button>
              </Show>
            </div>
            <textarea
              value={dockerIgnoredInput()}
              onInput={(event) => handleDockerIgnoredChange(event.currentTarget.value)}
              onKeyDown={(event) => {
                // Ensure Enter key works in textarea for creating new lines
                if (event.key === 'Enter') {
                  // Don't prevent default - allow the newline to be inserted
                  event.stopPropagation();
                }
              }}
              placeholder={dockerIgnoredPrefixesPresentation.placeholder}
              rows={4}
              class="mt-4 w-full rounded-md border border-border bg-surface p-3 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
            />
          </Card>

          <Card padding="md" tone="card" class="mb-6">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-base-content">
                  {dockerServicePresentation.title}
                </h3>
                <p class="mt-1 text-xs text-muted">{dockerServicePresentation.description}</p>
              </div>
              <Toggle
                checked={!props.disableAllDockerServices()}
                onToggle={() => {
                  props.setDisableAllDockerServices(!props.disableAllDockerServices());
                  props.setHasUnsavedChanges(true);
                }}
                label={
                  <span class="text-sm font-medium text-base-content">
                    {dockerServicePresentation.toggleLabel}
                  </span>
                }
                description={
                  <span class="text-xs text-muted">
                    {dockerServicePresentation.toggleDescription}
                  </span>
                }
                size="sm"
              />
            </div>

            <div class="mt-4 grid gap-4 sm:grid-cols-2">
              <div>
                <label
                  for={serviceWarnInputId}
                  class="text-xs font-medium uppercase tracking-wide text-muted"
                >
                  {dockerServicePresentation.warningGapLabel}
                </label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  id={serviceWarnInputId}
                  value={props.dockerDefaults.serviceWarnGapPercent}
                  onInput={(event) => {
                    const value = Number(event.currentTarget.value);
                    const normalized = Number.isFinite(value)
                      ? Math.max(0, Math.min(100, value))
                      : 0;
                    props.setDockerDefaults((prev) => ({
                      ...prev,
                      serviceWarnGapPercent: normalized,
                    }));
                    props.setHasUnsavedChanges(true);
                  }}
                  class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
                />
                <p class="mt-1 text-xs text-muted">
                  {dockerServicePresentation.warningGapDescription}
                </p>
              </div>
              <div>
                <label
                  for={serviceCriticalInputId}
                  class="text-xs font-medium uppercase tracking-wide text-muted"
                >
                  {dockerServicePresentation.criticalGapLabel}
                </label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  id={serviceCriticalInputId}
                  value={props.dockerDefaults.serviceCriticalGapPercent}
                  onInput={(event) => {
                    const value = Number(event.currentTarget.value);
                    const normalized = Number.isFinite(value)
                      ? Math.max(0, Math.min(100, value))
                      : 0;
                    props.setDockerDefaults((prev) => ({
                      ...prev,
                      serviceCriticalGapPercent: normalized,
                    }));
                    props.setHasUnsavedChanges(true);
                  }}
                  class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
                />
                <p class="mt-1 text-xs text-muted">
                  {dockerServicePresentation.criticalGapDescription}
                </p>
              </div>
            </div>
            {serviceGapValidationMessage() && (
              <p class="mt-1.5 text-xs font-medium text-red-600 dark:text-red-400">
                {serviceGapValidationMessage()}
              </p>
            )}
          </Card>

          <Show when={hasSection('dockerHosts')}>
            <div ref={registerSection('dockerHosts')} class="scroll-mt-24">
              <ResourceTable
                title={sectionTitles.dockerHosts}
                resources={dockerHostsWithOverrides()}
                columns={[]}
                activeAlerts={props.activeAlerts}
                emptyMessage={CONTAINER_RUNTIMES_FILTER_EMPTY_STATE}
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                onToggleNodeConnectivity={toggleNodeConnectivity}
                showOfflineAlertsColumn={true}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) =>
                  handleBulkEdit(ids, [
                    'CPU %',
                    'Memory %',
                    'Disk %',
                    'Disk R MB/s',
                    'Disk W MB/s',
                    'Net In MB/s',
                    'Net Out MB/s',
                    'Restart Count',
                    'Restart Window (s)',
                  ])
                }
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDisableFlag={props.disableAllDockerHosts}
                onToggleGlobalDisable={() =>
                  props.setDisableAllDockerHosts(!props.disableAllDockerHosts())
                }
                globalDisableOfflineFlag={props.disableAllDockerHostsOffline}
                onToggleGlobalDisableOffline={() =>
                  props.setDisableAllDockerHostsOffline(!props.disableAllDockerHostsOffline())
                }
              />
            </div>
          </Show>

          <Show when={hasSection('dockerContainers')}>
            <div ref={registerSection('dockerContainers')} class="scroll-mt-24">
              <ResourceTable
                title={sectionTitles.dockerContainers}
                groupedResources={dockerContainersGroupedByHost()}
                groupHeaderMeta={dockerHostGroupMeta()}
                columns={[
                  'CPU %',
                  'Memory %',
                  'Disk %',
                  'Restart Count',
                  'Restart Window (s)',
                  'Memory Warn %',
                  'Memory Critical %',
                ]}
                activeAlerts={props.activeAlerts}
                emptyMessage={CONTAINERS_FILTER_EMPTY_STATE}
                onEdit={startEditing}
                onSaveEdit={saveEdit}
                onCancelEdit={cancelEdit}
                onRemoveOverride={removeOverride}
                onToggleDisabled={toggleDisabled}
                showOfflineAlertsColumn={false}
                editingId={editingId}
                editingThresholds={editingThresholds}
                setEditingThresholds={setEditingThresholds}
                editingNote={editingNote}
                setEditingNote={setEditingNote}
                onBulkEdit={(ids) =>
                  handleBulkEdit(ids, ['CPU %', 'Memory %', 'Disk %', 'Temp °C'])
                }
                formatMetricValue={formatMetricValue}
                hasActiveAlert={hasActiveAlert}
                globalDefaults={{
                  cpu: props.dockerDefaults.cpu,
                  memory: props.dockerDefaults.memory,
                  disk: props.dockerDefaults.disk,
                  restartCount: props.dockerDefaults.restartCount,
                  restartWindow: props.dockerDefaults.restartWindow,
                  memoryWarnPct: props.dockerDefaults.memoryWarnPct,
                  memoryCriticalPct: props.dockerDefaults.memoryCriticalPct,
                }}
                setGlobalDefaults={(value) => {
                  const current = {
                    cpu: props.dockerDefaults.cpu,
                    memory: props.dockerDefaults.memory,
                    disk: props.dockerDefaults.disk,
                    restartCount: props.dockerDefaults.restartCount,
                    restartWindow: props.dockerDefaults.restartWindow,
                    memoryWarnPct: props.dockerDefaults.memoryWarnPct,
                    memoryCriticalPct: props.dockerDefaults.memoryCriticalPct,
                  };
                  const next =
                    typeof value === 'function' ? value(current) : { ...current, ...value };

                  props.setDockerDefaults((prev) => ({
                    ...prev,
                    cpu: next.cpu ?? prev.cpu,
                    memory: next.memory ?? prev.memory,
                    disk: next.disk ?? prev.disk,
                    restartCount: next.restartCount ?? prev.restartCount,
                    restartWindow: next.restartWindow ?? prev.restartWindow,
                    memoryWarnPct: next.memoryWarnPct ?? prev.memoryWarnPct,
                    memoryCriticalPct: next.memoryCriticalPct ?? prev.memoryCriticalPct,
                  }));
                }}
                setHasUnsavedChanges={props.setHasUnsavedChanges}
                globalDisableFlag={props.disableAllDockerContainers}
                onToggleGlobalDisable={() =>
                  props.setDisableAllDockerContainers(!props.disableAllDockerContainers())
                }
                globalDisableOfflineFlag={() => props.dockerDisableConnectivity()}
                onToggleGlobalDisableOffline={() =>
                  props.setDockerDisableConnectivity(!props.dockerDisableConnectivity())
                }
                showDelayColumn={true}
                globalDelaySeconds={props.timeThresholds().guest}
                metricDelaySeconds={props.metricTimeThresholds().guest ?? {}}
                onMetricDelayChange={(metric, value) => updateMetricDelay('guest', metric, value)}
                globalOfflineSeverity={props.dockerPoweredOffSeverity()}
                onSetGlobalOfflineState={(state) => {
                  if (state === 'off') {
                    props.setDockerDisableConnectivity(true);
                  } else {
                    props.setDockerDisableConnectivity(false);
                    props.setDockerPoweredOffSeverity(
                      state === 'critical' ? 'critical' : 'warning',
                    );
                  }
                  props.setHasUnsavedChanges(true);
                }}
                onSetOfflineState={setOfflineState}
                factoryDefaults={props.factoryDockerDefaults}
                onResetDefaults={props.resetDockerDefaults}
              />
            </div>
          </Show>
        </Show>
      </div>

      <BulkEditDialog
        isOpen={isBulkEditDialogOpen()}
        onClose={() => setIsBulkEditDialogOpen(false)}
        selectedIds={bulkEditIds()}
        columns={bulkEditColumns()}
        onSave={handleSaveBulkEdit}
      />
    </div>
  );
}
