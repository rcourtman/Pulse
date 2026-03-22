import { createSignal, onCleanup, onMount } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import type { ActivationState, BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';
import type { Resource, ResourceType } from '@/types/resource';
import {
  getAlertConfigDiscardedSuccess,
  getAlertConfigReloadFailure,
  getAlertConfigSaveSuccess,
} from '@/utils/alertConfigPresentation';
import { logger } from '@/utils/logger';

import {
  clampMaxAlertsPerHour,
  createDefaultCooldown,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  createDefaultResolveNotifications,
  DEFAULT_DELAY_SECONDS,
  fallbackMaxAlertsPerHour,
  getTriggerValue,
  normalizeMetricDelayMap,
} from './helpers';
import { useAlertDestinationsState } from './useAlertDestinationsState';
import { useAlertOverridesState } from './useAlertOverridesState';
import type {
  AlertTab,
  CooldownConfig,
  EscalationConfig,
  EscalationNotifyTarget,
  GroupingConfig,
  Override,
  QuietHoursConfig,
} from './types';
import { GROUPING_WINDOW_DEFAULT_SECONDS, clampCooldownMinutes } from './types';

export interface AlertsConfigurationSurfaceProps {
  activeTab: Accessor<AlertTab>;
  allResources: Accessor<Resource[]>;
  byType: (resourceType: ResourceType) => Resource[];
  children: (resourceId: string) => Resource[];
  activeAlerts: Record<string, Alert>;
  removeAlerts: (predicate: (alert: Alert) => boolean) => void;
  setOverviewOverrides: (value: Override[]) => void;
  hasUnsavedChanges: Accessor<boolean>;
  setHasUnsavedChanges: (value: boolean) => void;
  alertsActivationState: () => ActivationState | null;
  alertsActivationConfig: () => {
    enabled?: boolean;
    activationTime?: string | null;
    observationWindowHours?: number | null;
  } | null;
}

export const FACTORY_GUEST_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 90,
  diskRead: -1,
  diskWrite: -1,
  networkIn: -1,
  networkOut: -1,
};

export const FACTORY_NODE_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 90,
  temperature: 80,
};

export const FACTORY_PBS_DEFAULTS = {
  cpu: 80,
  memory: 85,
};

export const FACTORY_AGENT_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 90,
  diskTemperature: 55,
};

export const FACTORY_DOCKER_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 85,
  restartCount: 3,
  restartWindow: 300,
  memoryWarnPct: 90,
  memoryCriticalPct: 95,
  serviceWarnGapPercent: 10,
  serviceCriticalGapPercent: 50,
};

export const FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY = false;
export const FACTORY_DOCKER_STATE_SEVERITY: 'warning' | 'critical' = 'warning';
export const FACTORY_STORAGE_DEFAULT = 85;
export const FACTORY_SNAPSHOT_DEFAULTS: SnapshotAlertConfig = {
  enabled: false,
  warningDays: 30,
  criticalDays: 45,
};
export const FACTORY_BACKUP_DEFAULTS: BackupAlertConfig = {
  enabled: false,
  warningDays: 7,
  criticalDays: 14,
  freshHours: 24,
  staleHours: 72,
  alertOrphaned: true,
  ignoreVMIDs: [],
};

export function useAlertsConfigurationState(props: AlertsConfigurationSurfaceProps) {
  const [isReloadingConfig, setIsReloadingConfig] = createSignal(false);
  const [scheduleQuietHours, setScheduleQuietHours] =
    createSignal<QuietHoursConfig>(createDefaultQuietHours());
  const [scheduleCooldown, setScheduleCooldown] =
    createSignal<CooldownConfig>(createDefaultCooldown());
  const [scheduleGrouping, setScheduleGrouping] =
    createSignal<GroupingConfig>(createDefaultGrouping());
  const [scheduleEscalation, setScheduleEscalation] =
    createSignal<EscalationConfig>(createDefaultEscalation());
  const [notifyOnResolve, setNotifyOnResolve] = createSignal<boolean>(
    createDefaultResolveNotifications(),
  );
  const [guestDefaults, setGuestDefaults] = createSignal<Record<string, number | undefined>>({
    ...FACTORY_GUEST_DEFAULTS,
  });
  const [guestDisableConnectivity, setGuestDisableConnectivity] = createSignal(false);
  const [guestPoweredOffSeverity, setGuestPoweredOffSeverity] = createSignal<
    'warning' | 'critical'
  >('warning');
  const [nodeDefaults, setNodeDefaults] = createSignal<Record<string, number | undefined>>({
    ...FACTORY_NODE_DEFAULTS,
  });
  const [pbsDefaults, setPBSDefaults] = createSignal<Record<string, number | undefined>>({
    ...FACTORY_PBS_DEFAULTS,
  });
  const [agentDefaults, setAgentDefaults] = createSignal<Record<string, number | undefined>>({
    ...FACTORY_AGENT_DEFAULTS,
  });
  const [dockerDefaults, setDockerDefaults] = createSignal({ ...FACTORY_DOCKER_DEFAULTS });
  const [dockerDisableConnectivity, setDockerDisableConnectivity] = createSignal(
    FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY,
  );
  const [dockerPoweredOffSeverity, setDockerPoweredOffSeverity] = createSignal<
    'warning' | 'critical'
  >(FACTORY_DOCKER_STATE_SEVERITY);
  const [dockerIgnoredPrefixes, setDockerIgnoredPrefixes] = createSignal<string[]>([]);
  const [ignoredGuestPrefixes, setIgnoredGuestPrefixes] = createSignal<string[]>([]);
  const [guestTagWhitelist, setGuestTagWhitelist] = createSignal<string[]>([]);
  const [guestTagBlacklist, setGuestTagBlacklist] = createSignal<string[]>([]);
  const [storageDefault, setStorageDefault] = createSignal(FACTORY_STORAGE_DEFAULT);
  const [backupDefaults, setBackupDefaults] = createSignal<BackupAlertConfig>({
    ...FACTORY_BACKUP_DEFAULTS,
  });
  const [timeThresholds, setTimeThresholds] = createSignal({
    guest: DEFAULT_DELAY_SECONDS,
    node: DEFAULT_DELAY_SECONDS,
    storage: DEFAULT_DELAY_SECONDS,
    pbs: DEFAULT_DELAY_SECONDS,
    agent: DEFAULT_DELAY_SECONDS,
  });
  const [metricTimeThresholds, setMetricTimeThresholds] = createSignal<
    Record<string, Record<string, number>>
  >({});
  const [snapshotDefaults, setSnapshotDefaults] = createSignal<SnapshotAlertConfig>({
    ...FACTORY_SNAPSHOT_DEFAULTS,
  });
  const [pmgThresholds, setPMGThresholds] = createSignal({
    queueTotalWarning: 500,
    queueTotalCritical: 1000,
    oldestMessageWarnMins: 30,
    oldestMessageCritMins: 60,
    deferredQueueWarn: 200,
    deferredQueueCritical: 500,
    holdQueueWarn: 100,
    holdQueueCritical: 300,
    quarantineSpamWarn: 2000,
    quarantineSpamCritical: 5000,
    quarantineVirusWarn: 2000,
    quarantineVirusCritical: 5000,
    quarantineGrowthWarnPct: 25,
    quarantineGrowthWarnMin: 250,
    quarantineGrowthCritPct: 50,
    quarantineGrowthCritMin: 500,
  });
  const [disableAllNodes, setDisableAllNodes] = createSignal(false);
  const [disableAllGuests, setDisableAllGuests] = createSignal(false);
  const [disableAllAgents, setDisableAllAgents] = createSignal(false);
  const [disableAllStorage, setDisableAllStorage] = createSignal(false);
  const [disableAllPBS, setDisableAllPBS] = createSignal(false);
  const [disableAllPMG, setDisableAllPMG] = createSignal(false);
  const [disableAllDockerHosts, setDisableAllDockerHosts] = createSignal(false);
  const [disableAllDockerServices, setDisableAllDockerServices] = createSignal(false);
  const [disableAllDockerContainers, setDisableAllDockerContainers] = createSignal(false);
  const [disableAllNodesOffline, setDisableAllNodesOffline] = createSignal(false);
  const [disableAllGuestsOffline, setDisableAllGuestsOffline] = createSignal(false);
  const [disableAllAgentsOffline, setDisableAllAgentsOffline] = createSignal(false);
  const [disableAllPBSOffline, setDisableAllPBSOffline] = createSignal(false);
  const [disableAllPMGOffline, setDisableAllPMGOffline] = createSignal(false);
  const [disableAllDockerHostsOffline, setDisableAllDockerHostsOffline] = createSignal(false);
  const destinationsState = useAlertDestinationsState({ activeTab: props.activeTab });
  const overridesState = useAlertOverridesState({
    allResources: props.allResources,
    byType: props.byType,
    children: props.children,
    hasUnsavedChanges: props.hasUnsavedChanges,
    setOverviewOverrides: props.setOverviewOverrides,
  });

  const resetGuestDefaults = () => {
    setGuestDefaults({ ...FACTORY_GUEST_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetNodeDefaults = () => {
    setNodeDefaults({ ...FACTORY_NODE_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetPBSDefaults = () => {
    setPBSDefaults({ ...FACTORY_PBS_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetAgentDefaults = () => {
    setAgentDefaults({ ...FACTORY_AGENT_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetDockerDefaults = () => {
    setDockerDefaults({ ...FACTORY_DOCKER_DEFAULTS });
    setDockerDisableConnectivity(FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY);
    setDockerPoweredOffSeverity(FACTORY_DOCKER_STATE_SEVERITY);
    props.setHasUnsavedChanges(true);
  };
  const resetDockerIgnoredPrefixes = () => {
    setDockerIgnoredPrefixes([]);
    props.setHasUnsavedChanges(true);
  };
  const resetStorageDefault = () => {
    setStorageDefault(FACTORY_STORAGE_DEFAULT);
    props.setHasUnsavedChanges(true);
  };
  const resetBackupDefaults = () => {
    setBackupDefaults({ ...FACTORY_BACKUP_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetSnapshotDefaults = () => {
    setSnapshotDefaults({ ...FACTORY_SNAPSHOT_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };

  const loadAlertConfiguration = async (options: { notify?: boolean } = {}) => {
    setIsReloadingConfig(true);
    props.setHasUnsavedChanges(false);
    destinationsState.resetDestinations();

    setGuestDefaults({ ...FACTORY_GUEST_DEFAULTS });
    setGuestDisableConnectivity(false);
    setGuestPoweredOffSeverity('warning');
    setNodeDefaults({ ...FACTORY_NODE_DEFAULTS });
    setPBSDefaults({ ...FACTORY_PBS_DEFAULTS });
    setAgentDefaults({ ...FACTORY_AGENT_DEFAULTS });
    setDockerDefaults({ ...FACTORY_DOCKER_DEFAULTS });
    setDockerDisableConnectivity(FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY);
    setDockerPoweredOffSeverity(FACTORY_DOCKER_STATE_SEVERITY);
    setDockerIgnoredPrefixes([]);
    setIgnoredGuestPrefixes([]);
    setGuestTagWhitelist([]);
    setGuestTagBlacklist([]);
    setStorageDefault(FACTORY_STORAGE_DEFAULT);
    setTimeThresholds({
      guest: DEFAULT_DELAY_SECONDS,
      node: DEFAULT_DELAY_SECONDS,
      storage: DEFAULT_DELAY_SECONDS,
      pbs: DEFAULT_DELAY_SECONDS,
      agent: DEFAULT_DELAY_SECONDS,
    });
    setMetricTimeThresholds({});
    setScheduleQuietHours(createDefaultQuietHours());
    setScheduleCooldown(createDefaultCooldown());
    setScheduleGrouping(createDefaultGrouping());
    setScheduleEscalation(createDefaultEscalation());
    setNotifyOnResolve(createDefaultResolveNotifications());
    setBackupDefaults({ ...FACTORY_BACKUP_DEFAULTS });
    setSnapshotDefaults({ ...FACTORY_SNAPSHOT_DEFAULTS });

    try {
      const config = await AlertsAPI.getConfig();

      if (config.guestDefaults) {
        setGuestDefaults({
          cpu: getTriggerValue(config.guestDefaults.cpu) ?? FACTORY_GUEST_DEFAULTS.cpu,
          memory: getTriggerValue(config.guestDefaults.memory) ?? FACTORY_GUEST_DEFAULTS.memory,
          disk: getTriggerValue(config.guestDefaults.disk) ?? FACTORY_GUEST_DEFAULTS.disk,
          diskRead:
            getTriggerValue(config.guestDefaults.diskRead) ?? FACTORY_GUEST_DEFAULTS.diskRead,
          diskWrite:
            getTriggerValue(config.guestDefaults.diskWrite) ?? FACTORY_GUEST_DEFAULTS.diskWrite,
          networkIn:
            getTriggerValue(config.guestDefaults.networkIn) ?? FACTORY_GUEST_DEFAULTS.networkIn,
          networkOut:
            getTriggerValue(config.guestDefaults.networkOut) ?? FACTORY_GUEST_DEFAULTS.networkOut,
        });
        setGuestDisableConnectivity(Boolean(config.guestDefaults.disableConnectivity));
        setGuestPoweredOffSeverity(
          config.guestDefaults.poweredOffSeverity === 'critical' ? 'critical' : 'warning',
        );
      }

      if (config.nodeDefaults) {
        setNodeDefaults({
          cpu: getTriggerValue(config.nodeDefaults.cpu) ?? FACTORY_NODE_DEFAULTS.cpu,
          memory: getTriggerValue(config.nodeDefaults.memory) ?? FACTORY_NODE_DEFAULTS.memory,
          disk: getTriggerValue(config.nodeDefaults.disk) ?? FACTORY_NODE_DEFAULTS.disk,
          temperature:
            getTriggerValue(config.nodeDefaults.temperature) ?? FACTORY_NODE_DEFAULTS.temperature,
        });
      }

      if (config.pbsDefaults) {
        setPBSDefaults({
          cpu: getTriggerValue(config.pbsDefaults.cpu) ?? FACTORY_PBS_DEFAULTS.cpu,
          memory: getTriggerValue(config.pbsDefaults.memory) ?? FACTORY_PBS_DEFAULTS.memory,
        });
      }

      if (config.agentDefaults) {
        setAgentDefaults({
          cpu: getTriggerValue(config.agentDefaults.cpu) ?? FACTORY_AGENT_DEFAULTS.cpu,
          memory: getTriggerValue(config.agentDefaults.memory) ?? FACTORY_AGENT_DEFAULTS.memory,
          disk: getTriggerValue(config.agentDefaults.disk) ?? FACTORY_AGENT_DEFAULTS.disk,
          diskTemperature:
            getTriggerValue(config.agentDefaults.diskTemperature) ??
            FACTORY_AGENT_DEFAULTS.diskTemperature,
        });
      }

      if (config.dockerDefaults) {
        const normalizeGap = (value: unknown, fallback: number) => {
          const numeric = Number(value);
          if (!Number.isFinite(numeric)) {
            return fallback;
          }
          return Math.max(0, Math.min(100, numeric));
        };

        const serviceWarnGap = normalizeGap(
          config.dockerDefaults.serviceWarnGapPercent,
          FACTORY_DOCKER_DEFAULTS.serviceWarnGapPercent,
        );
        let serviceCriticalGap = normalizeGap(
          config.dockerDefaults.serviceCriticalGapPercent,
          FACTORY_DOCKER_DEFAULTS.serviceCriticalGapPercent,
        );
        if (serviceCriticalGap > 0 && serviceWarnGap > serviceCriticalGap) {
          serviceCriticalGap = serviceWarnGap;
        }

        setDockerDefaults({
          cpu: getTriggerValue(config.dockerDefaults.cpu) ?? FACTORY_DOCKER_DEFAULTS.cpu,
          memory: getTriggerValue(config.dockerDefaults.memory) ?? FACTORY_DOCKER_DEFAULTS.memory,
          disk: getTriggerValue(config.dockerDefaults.disk) ?? FACTORY_DOCKER_DEFAULTS.disk,
          restartCount:
            config.dockerDefaults.restartCount ?? FACTORY_DOCKER_DEFAULTS.restartCount,
          restartWindow:
            config.dockerDefaults.restartWindow ?? FACTORY_DOCKER_DEFAULTS.restartWindow,
          memoryWarnPct:
            config.dockerDefaults.memoryWarnPct ?? FACTORY_DOCKER_DEFAULTS.memoryWarnPct,
          memoryCriticalPct:
            config.dockerDefaults.memoryCriticalPct ??
            FACTORY_DOCKER_DEFAULTS.memoryCriticalPct,
          serviceWarnGapPercent: serviceWarnGap,
          serviceCriticalGapPercent: serviceCriticalGap,
        });
        setDockerDisableConnectivity(Boolean(config.dockerDefaults.stateDisableConnectivity));
        setDockerPoweredOffSeverity(
          config.dockerDefaults.statePoweredOffSeverity === 'critical' ? 'critical' : 'warning',
        );
      }

      setDockerIgnoredPrefixes(config.dockerIgnoredContainerPrefixes ?? []);
      setIgnoredGuestPrefixes(config.ignoredGuestPrefixes ?? []);
      setGuestTagWhitelist(config.guestTagWhitelist ?? []);
      setGuestTagBlacklist(config.guestTagBlacklist ?? []);

      if (config.storageDefault) {
        setStorageDefault(getTriggerValue(config.storageDefault) ?? FACTORY_STORAGE_DEFAULT);
      }
      if (config.timeThresholds) {
        setTimeThresholds({
          guest: config.timeThresholds.guest ?? DEFAULT_DELAY_SECONDS,
          node: config.timeThresholds.node ?? DEFAULT_DELAY_SECONDS,
          storage: config.timeThresholds.storage ?? DEFAULT_DELAY_SECONDS,
          pbs: config.timeThresholds.pbs ?? DEFAULT_DELAY_SECONDS,
          agent: config.timeThresholds.agent ?? DEFAULT_DELAY_SECONDS,
        });
      }
      if (config.metricTimeThresholds) {
        setMetricTimeThresholds(normalizeMetricDelayMap(config.metricTimeThresholds));
      }

      if (config.backupDefaults) {
        const enabled = Boolean(config.backupDefaults.enabled);
        const rawWarning =
          config.backupDefaults.warningDays ?? FACTORY_BACKUP_DEFAULTS.warningDays;
        const rawCritical =
          config.backupDefaults.criticalDays ?? FACTORY_BACKUP_DEFAULTS.criticalDays;
        const safeCritical = Math.max(0, rawCritical);
        const normalizedWarning = Math.max(0, rawWarning);
        const warningDays =
          safeCritical > 0 && normalizedWarning > safeCritical ? safeCritical : normalizedWarning;
        const criticalDays = Math.max(safeCritical, warningDays);
        const freshHours =
          config.backupDefaults.freshHours ?? FACTORY_BACKUP_DEFAULTS.freshHours;
        const staleHours =
          config.backupDefaults.staleHours ?? FACTORY_BACKUP_DEFAULTS.staleHours;
        const alertOrphaned =
          config.backupDefaults.alertOrphaned ?? FACTORY_BACKUP_DEFAULTS.alertOrphaned ?? true;
        const ignoreVMIDs = Array.from(
          new Set(
            (config.backupDefaults.ignoreVMIDs ?? FACTORY_BACKUP_DEFAULTS.ignoreVMIDs ?? [])
              .map((value) => value.trim())
              .filter((value) => value.length > 0),
          ),
        );
        setBackupDefaults({
          enabled,
          warningDays,
          criticalDays,
          freshHours,
          staleHours,
          alertOrphaned,
          ignoreVMIDs,
        });
      }

      if (config.snapshotDefaults) {
        const enabled = Boolean(config.snapshotDefaults.enabled);
        const rawWarning =
          config.snapshotDefaults.warningDays ?? FACTORY_SNAPSHOT_DEFAULTS.warningDays;
        const rawCritical =
          config.snapshotDefaults.criticalDays ?? FACTORY_SNAPSHOT_DEFAULTS.criticalDays;
        const safeCritical = Math.max(0, rawCritical);
        const normalizedWarning = Math.max(0, rawWarning);
        const warningDays =
          safeCritical > 0 && normalizedWarning > safeCritical ? safeCritical : normalizedWarning;
        const criticalDays = Math.max(safeCritical, warningDays);
        setSnapshotDefaults({ enabled, warningDays, criticalDays });
      }

      if (config.pmgDefaults) {
        setPMGThresholds({
          queueTotalWarning: config.pmgDefaults.queueTotalWarning ?? 500,
          queueTotalCritical: config.pmgDefaults.queueTotalCritical ?? 1000,
          oldestMessageWarnMins: config.pmgDefaults.oldestMessageWarnMins ?? 30,
          oldestMessageCritMins: config.pmgDefaults.oldestMessageCritMins ?? 60,
          deferredQueueWarn: config.pmgDefaults.deferredQueueWarn ?? 200,
          deferredQueueCritical: config.pmgDefaults.deferredQueueCritical ?? 500,
          holdQueueWarn: config.pmgDefaults.holdQueueWarn ?? 100,
          holdQueueCritical: config.pmgDefaults.holdQueueCritical ?? 300,
          quarantineSpamWarn: config.pmgDefaults.quarantineSpamWarn ?? 2000,
          quarantineSpamCritical: config.pmgDefaults.quarantineSpamCritical ?? 5000,
          quarantineVirusWarn: config.pmgDefaults.quarantineVirusWarn ?? 2000,
          quarantineVirusCritical: config.pmgDefaults.quarantineVirusCritical ?? 5000,
          quarantineGrowthWarnPct: config.pmgDefaults.quarantineGrowthWarnPct ?? 25,
          quarantineGrowthWarnMin: config.pmgDefaults.quarantineGrowthWarnMin ?? 250,
          quarantineGrowthCritPct: config.pmgDefaults.quarantineGrowthCritPct ?? 50,
          quarantineGrowthCritMin: config.pmgDefaults.quarantineGrowthCritMin ?? 500,
        });
      }

      setDisableAllNodes(config.disableAllNodes ?? false);
      setDisableAllGuests(config.disableAllGuests ?? false);
      setDisableAllAgents(config.disableAllAgents ?? false);
      setDisableAllStorage(config.disableAllStorage ?? false);
      setDisableAllPBS(config.disableAllPBS ?? false);
      setDisableAllPMG(config.disableAllPMG ?? false);
      setDisableAllDockerHosts(config.disableAllDockerHosts ?? false);
      setDisableAllDockerServices(config.disableAllDockerServices ?? false);
      setDisableAllDockerContainers(config.disableAllDockerContainers ?? false);
      setDisableAllNodesOffline(config.disableAllNodesOffline ?? false);
      setDisableAllGuestsOffline(config.disableAllGuestsOffline ?? false);
      setDisableAllAgentsOffline(config.disableAllAgentsOffline ?? false);
      setDisableAllPBSOffline(config.disableAllPBSOffline ?? false);
      setDisableAllPMGOffline(config.disableAllPMGOffline ?? false);
      setDisableAllDockerHostsOffline(config.disableAllDockerHostsOffline ?? false);

      overridesState.replaceRawOverridesConfig(config.overrides || {});

      if (config.schedule) {
        if (config.schedule.quietHours) {
          const quietHours = config.schedule.quietHours;
          const days = Array.isArray(quietHours.days)
            ? {
                sunday: quietHours.days.includes(0),
                monday: quietHours.days.includes(1),
                tuesday: quietHours.days.includes(2),
                wednesday: quietHours.days.includes(3),
                thursday: quietHours.days.includes(4),
                friday: quietHours.days.includes(5),
                saturday: quietHours.days.includes(6),
              }
            : ((quietHours.days as Record<string, boolean>) || createDefaultQuietHours().days);
          setScheduleQuietHours({
            enabled: quietHours.enabled || false,
            start: quietHours.start || '22:00',
            end: quietHours.end || '08:00',
            timezone:
              quietHours.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
            days,
            suppress: {
              performance: quietHours.suppress?.performance ?? false,
              storage: quietHours.suppress?.storage ?? false,
              offline: quietHours.suppress?.offline ?? false,
            },
          });
        }

        if (config.schedule.cooldown !== undefined) {
          const rawCooldown = config.schedule.cooldown;
          const cooldownEnabled = rawCooldown > 0;
          setScheduleCooldown({
            enabled: cooldownEnabled,
            minutes: cooldownEnabled ? clampCooldownMinutes(rawCooldown) : 0,
            maxAlerts: fallbackMaxAlertsPerHour(config.schedule.maxAlertsHour),
          });
        }

        if (config.schedule.grouping) {
          const groupingConfig = config.schedule.grouping;
          const rawGroupingWindowSeconds =
            typeof groupingConfig.window === 'number'
              ? groupingConfig.window
              : GROUPING_WINDOW_DEFAULT_SECONDS;
          const normalizedGroupingWindowSeconds = Math.max(0, rawGroupingWindowSeconds);
          setScheduleGrouping({
            enabled:
              groupingConfig.enabled !== undefined
                ? Boolean(groupingConfig.enabled)
                : normalizedGroupingWindowSeconds > 0,
            window: Math.round(normalizedGroupingWindowSeconds / 60),
            byNode: groupingConfig.byNode !== undefined ? groupingConfig.byNode : true,
            byGuest: groupingConfig.byGuest !== undefined ? groupingConfig.byGuest : false,
          });
        }

        if (config.schedule.notifyOnResolve !== undefined) {
          setNotifyOnResolve(Boolean(config.schedule.notifyOnResolve));
        }

        if (config.schedule.escalation) {
          const levels = (config.schedule.escalation.levels || []).map((level) => ({
            after: typeof level.after === 'number' ? level.after : 15,
            notify: (level.notify as EscalationNotifyTarget) || 'all',
          }));
          setScheduleEscalation({
            enabled: Boolean(config.schedule.escalation.enabled),
            levels,
          });
        }
      }

      await destinationsState.loadDestinations();

      if (options.notify) {
        notificationStore.success(getAlertConfigDiscardedSuccess());
      }
    } catch (error) {
      logger.error('Failed to load alert configuration:', error);
      if (options.notify) {
        notificationStore.error(getAlertConfigReloadFailure());
      }
    } finally {
      setIsReloadingConfig(false);
    }
  };

  const saveAlertConfiguration = async () => {
    const createHysteresisThreshold = (trigger: number | undefined, clearMargin = 5) => {
      const normalized = typeof trigger === 'number' ? trigger : 0;
      return {
        trigger: normalized,
        clear: Math.max(0, normalized - clearMargin),
      };
    };

    const snapshotConfig = snapshotDefaults();
    const normalizedSnapshotWarning = Math.max(0, snapshotConfig.warningDays ?? 0);
    const normalizedSnapshotCritical = Math.max(0, snapshotConfig.criticalDays ?? 0);
    const finalSnapshotCritical =
      normalizedSnapshotCritical > 0
        ? Math.max(normalizedSnapshotCritical, normalizedSnapshotWarning)
        : normalizedSnapshotWarning;

    const backupConfig = backupDefaults();
    const normalizedBackupWarning = Math.max(0, backupConfig.warningDays ?? 0);
    const normalizedBackupCritical = Math.max(0, backupConfig.criticalDays ?? 0);
    const finalBackupCritical =
      normalizedBackupCritical > 0
        ? Math.max(normalizedBackupCritical, normalizedBackupWarning)
        : normalizedBackupWarning;

    const dockerDefaultsValue = dockerDefaults();
    if (
      dockerDefaultsValue.serviceCriticalGapPercent > 0 &&
      dockerDefaultsValue.serviceWarnGapPercent > dockerDefaultsValue.serviceCriticalGapPercent
    ) {
      notificationStore.error(
        'Swarm service critical gap must be greater than or equal to the warning gap when enabled.',
      );
      return;
    }

    const normalizedCooldownMinutes = scheduleCooldown().enabled
      ? clampCooldownMinutes(scheduleCooldown().minutes)
      : 0;
    const normalizedMaxAlertsHour = clampMaxAlertsPerHour(scheduleCooldown().maxAlerts);
    const groupingState = scheduleGrouping();
    const groupingWindowSeconds =
      groupingState.enabled && groupingState.window >= 0 ? groupingState.window * 60 : 0;
    const groupingEnabled = groupingState.enabled && groupingWindowSeconds > 0;
    const activationConfig = props.alertsActivationConfig();

    const alertConfig = {
      enabled: activationConfig?.enabled ?? true,
      activationState: props.alertsActivationState() ?? undefined,
      activationTime: activationConfig?.activationTime ?? undefined,
      observationWindowHours: activationConfig?.observationWindowHours ?? undefined,
      disableAllNodes: disableAllNodes(),
      disableAllGuests: disableAllGuests(),
      disableAllAgents: disableAllAgents(),
      disableAllStorage: disableAllStorage(),
      disableAllPBS: disableAllPBS(),
      disableAllPMG: disableAllPMG(),
      disableAllDockerHosts: disableAllDockerHosts(),
      disableAllDockerContainers: disableAllDockerContainers(),
      disableAllDockerServices: disableAllDockerServices(),
      disableAllNodesOffline: disableAllNodesOffline(),
      disableAllGuestsOffline: disableAllGuestsOffline(),
      disableAllPBSOffline: disableAllPBSOffline(),
      disableAllAgentsOffline: disableAllAgentsOffline(),
      disableAllPMGOffline: disableAllPMGOffline(),
      disableAllDockerHostsOffline: disableAllDockerHostsOffline(),
      guestDefaults: {
        cpu: createHysteresisThreshold(guestDefaults().cpu),
        memory: createHysteresisThreshold(guestDefaults().memory),
        disk: createHysteresisThreshold(guestDefaults().disk),
        diskRead: createHysteresisThreshold(guestDefaults().diskRead),
        diskWrite: createHysteresisThreshold(guestDefaults().diskWrite),
        networkIn: createHysteresisThreshold(guestDefaults().networkIn),
        networkOut: createHysteresisThreshold(guestDefaults().networkOut),
        disableConnectivity: guestDisableConnectivity(),
        poweredOffSeverity: guestPoweredOffSeverity(),
      },
      nodeDefaults: {
        cpu: createHysteresisThreshold(nodeDefaults().cpu),
        memory: createHysteresisThreshold(nodeDefaults().memory),
        disk: createHysteresisThreshold(nodeDefaults().disk),
        temperature: createHysteresisThreshold(nodeDefaults().temperature),
      },
      agentDefaults: {
        cpu: createHysteresisThreshold(agentDefaults().cpu),
        memory: createHysteresisThreshold(agentDefaults().memory),
        disk: createHysteresisThreshold(agentDefaults().disk),
        diskTemperature: createHysteresisThreshold(agentDefaults().diskTemperature),
      },
      pbsDefaults: {
        cpu: createHysteresisThreshold(pbsDefaults().cpu),
        memory: createHysteresisThreshold(pbsDefaults().memory),
      },
      dockerDefaults: {
        cpu: createHysteresisThreshold(dockerDefaultsValue.cpu),
        memory: createHysteresisThreshold(dockerDefaultsValue.memory),
        disk: createHysteresisThreshold(dockerDefaultsValue.disk),
        restartCount: dockerDefaultsValue.restartCount,
        restartWindow: dockerDefaultsValue.restartWindow,
        memoryWarnPct: dockerDefaultsValue.memoryWarnPct,
        memoryCriticalPct: dockerDefaultsValue.memoryCriticalPct,
        serviceWarnGapPercent: dockerDefaultsValue.serviceWarnGapPercent,
        serviceCriticalGapPercent: dockerDefaultsValue.serviceCriticalGapPercent,
        stateDisableConnectivity: dockerDisableConnectivity(),
        statePoweredOffSeverity: dockerPoweredOffSeverity(),
      },
      dockerIgnoredContainerPrefixes: dockerIgnoredPrefixes()
        .map((prefix) => prefix.trim())
        .filter((prefix) => prefix.length > 0),
      ignoredGuestPrefixes: ignoredGuestPrefixes()
        .map((prefix) => prefix.trim())
        .filter((prefix) => prefix.length > 0),
      guestTagWhitelist: guestTagWhitelist()
        .map((tag) => tag.trim())
        .filter((tag) => tag.length > 0),
      guestTagBlacklist: guestTagBlacklist()
        .map((tag) => tag.trim())
        .filter((tag) => tag.length > 0),
      storageDefault: createHysteresisThreshold(storageDefault()),
      minimumDelta: 2.0,
      suppressionWindow: 5,
      hysteresisMargin: 5.0,
      timeThresholds: timeThresholds(),
      metricTimeThresholds: normalizeMetricDelayMap(metricTimeThresholds()),
      snapshotDefaults: {
        enabled: snapshotConfig.enabled,
        warningDays: normalizedSnapshotWarning,
        criticalDays: finalSnapshotCritical,
      },
      backupDefaults: {
        enabled: backupConfig.enabled,
        warningDays: normalizedBackupWarning,
        criticalDays: finalBackupCritical,
        freshHours: backupConfig.freshHours ?? 24,
        staleHours: backupConfig.staleHours ?? 72,
        alertOrphaned: backupConfig.alertOrphaned ?? true,
        ignoreVMIDs: (backupConfig.ignoreVMIDs ?? [])
          .map((value) => value.trim())
          .filter((value) => value.length > 0),
      },
      pmgDefaults: pmgThresholds(),
      overrides: overridesState.rawOverridesConfig(),
      schedule: {
        quietHours: scheduleQuietHours(),
        cooldown: normalizedCooldownMinutes,
        notifyOnResolve: notifyOnResolve(),
        maxAlertsHour: normalizedMaxAlertsHour,
        escalation: scheduleEscalation(),
        grouping: {
          enabled: groupingEnabled,
          window: groupingWindowSeconds,
          byNode: groupingState.byNode,
          byGuest: groupingState.byGuest,
        },
      },
      aggregation: {
        enabled: true,
        timeWindow: 10,
        countThreshold: 3,
        similarityWindow: 5.0,
      },
      flapping: {
        enabled: true,
        threshold: 5,
        window: 10,
        suppressionTime: 30,
        minStability: 0.8,
      },
      ioNormalization: {
        enabled: true,
        vmDiskMax: 500.0,
        containerDiskMax: 300.0,
        networkMax: 1000.0,
      },
    };

    await AlertsAPI.updateConfig(alertConfig);

    await destinationsState.saveDestinations();
    props.setHasUnsavedChanges(false);
    notificationStore.success(getAlertConfigSaveSuccess());
  };

  onMount(() => {
    void loadAlertConfiguration();
    const unsubscribeOrgSwitched = eventBus.on('org_switched', () => {
      void loadAlertConfiguration();
    });
    onCleanup(() => {
      unsubscribeOrgSwitched();
    });
  });

  return {
    isReloadingConfig,
    isLoadingDestinations: destinationsState.isLoadingDestinations,
    destConfigLoadError: destinationsState.destConfigLoadError,
    overrides: overridesState.overrides,
    setOverrides: overridesState.setOverrides,
    rawOverridesConfig: overridesState.rawOverridesConfig,
    setRawOverridesConfig: overridesState.setRawOverridesConfig,
    emailConfig: destinationsState.emailConfig,
    setEmailConfig: destinationsState.setEmailConfig,
    appriseConfig: destinationsState.appriseConfig,
    setAppriseConfig: destinationsState.setAppriseConfig,
    scheduleQuietHours,
    setScheduleQuietHours,
    scheduleCooldown,
    setScheduleCooldown,
    scheduleGrouping,
    setScheduleGrouping,
    scheduleEscalation,
    setScheduleEscalation,
    notifyOnResolve,
    setNotifyOnResolve,
    guestDefaults,
    setGuestDefaults,
    guestDisableConnectivity,
    setGuestDisableConnectivity,
    guestPoweredOffSeverity,
    setGuestPoweredOffSeverity,
    nodeDefaults,
    setNodeDefaults,
    pbsDefaults,
    setPBSDefaults,
    agentDefaults,
    setAgentDefaults,
    dockerDefaults,
    setDockerDefaults,
    dockerDisableConnectivity,
    setDockerDisableConnectivity,
    dockerPoweredOffSeverity,
    setDockerPoweredOffSeverity,
    dockerIgnoredPrefixes,
    setDockerIgnoredPrefixes,
    ignoredGuestPrefixes,
    setIgnoredGuestPrefixes,
    guestTagWhitelist,
    setGuestTagWhitelist,
    guestTagBlacklist,
    setGuestTagBlacklist,
    storageDefault,
    setStorageDefault,
    backupDefaults,
    setBackupDefaults,
    timeThresholds,
    setTimeThresholds,
    metricTimeThresholds,
    setMetricTimeThresholds,
    snapshotDefaults,
    setSnapshotDefaults,
    pmgThresholds,
    setPMGThresholds,
    disableAllNodes,
    setDisableAllNodes,
    disableAllGuests,
    setDisableAllGuests,
    disableAllAgents,
    setDisableAllAgents,
    disableAllStorage,
    setDisableAllStorage,
    disableAllPBS,
    setDisableAllPBS,
    disableAllPMG,
    setDisableAllPMG,
    disableAllDockerHosts,
    setDisableAllDockerHosts,
    disableAllDockerServices,
    setDisableAllDockerServices,
    disableAllDockerContainers,
    setDisableAllDockerContainers,
    disableAllNodesOffline,
    setDisableAllNodesOffline,
    disableAllGuestsOffline,
    setDisableAllGuestsOffline,
    disableAllAgentsOffline,
    setDisableAllAgentsOffline,
    disableAllPBSOffline,
    setDisableAllPBSOffline,
    disableAllPMGOffline,
    setDisableAllPMGOffline,
    disableAllDockerHostsOffline,
    setDisableAllDockerHostsOffline,
    allGuests: overridesState.allGuests,
    agentResources: overridesState.agentResources,
    pbsInstances: overridesState.pbsInstances,
    pmgInstances: overridesState.pmgInstances,
    resetGuestDefaults,
    resetNodeDefaults,
    resetPBSDefaults,
    resetAgentDefaults,
    resetDockerDefaults,
    resetDockerIgnoredPrefixes,
    resetStorageDefault,
    resetBackupDefaults,
    resetSnapshotDefaults,
    loadAlertConfiguration,
    saveAlertConfiguration,
    factoryGuestDefaults: FACTORY_GUEST_DEFAULTS,
    factoryNodeDefaults: FACTORY_NODE_DEFAULTS,
    factoryPBSDefaults: FACTORY_PBS_DEFAULTS,
    factoryAgentDefaults: FACTORY_AGENT_DEFAULTS,
    factoryDockerDefaults: FACTORY_DOCKER_DEFAULTS,
    factoryStorageDefault: FACTORY_STORAGE_DEFAULT,
    snapshotFactoryDefaults: FACTORY_SNAPSHOT_DEFAULTS,
    backupFactoryDefaults: FACTORY_BACKUP_DEFAULTS,
  };
}
