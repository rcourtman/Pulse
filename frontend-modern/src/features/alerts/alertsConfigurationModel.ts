import type {
  AlertConfig,
  ActivationState,
  BackupAlertConfig,
  SnapshotAlertConfig,
} from '@/types/alerts';
import {
  FACTORY_AGENT_DEFAULTS,
  FACTORY_BACKUP_DEFAULTS,
  FACTORY_DOCKER_DEFAULTS,
  FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY,
  FACTORY_DOCKER_STATE_SEVERITY,
  FACTORY_GUEST_DEFAULTS,
  FACTORY_KUBERNETES_DEFAULTS,
  FACTORY_NODE_DEFAULTS,
  FACTORY_PBS_DEFAULTS,
  FACTORY_SNAPSHOT_DEFAULTS,
  FACTORY_STORAGE_DEFAULT,
  FACTORY_TRUENAS_DEFAULTS,
  FACTORY_TRUENAS_DISK_DEFAULTS,
  FACTORY_VMWARE_DEFAULTS,
} from '@/utils/alertThresholdDefaults';

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
import type {
  CooldownConfig,
  EscalationConfig,
  EscalationNotifyTarget,
  GroupingConfig,
  QuietHoursConfig,
} from './types';
import { GROUPING_WINDOW_DEFAULT_SECONDS, clampCooldownMinutes } from './types';

export {
  FACTORY_AGENT_DEFAULTS,
  FACTORY_BACKUP_DEFAULTS,
  FACTORY_DOCKER_DEFAULTS,
  FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY,
  FACTORY_DOCKER_STATE_SEVERITY,
  FACTORY_GUEST_DEFAULTS,
  FACTORY_KUBERNETES_DEFAULTS,
  FACTORY_NODE_DEFAULTS,
  FACTORY_PBS_DEFAULTS,
  FACTORY_SNAPSHOT_DEFAULTS,
  FACTORY_STORAGE_DEFAULT,
  FACTORY_TRUENAS_DEFAULTS,
  FACTORY_TRUENAS_DISK_DEFAULTS,
  FACTORY_VMWARE_DEFAULTS,
} from '@/utils/alertThresholdDefaults';

export interface AlertsConfigurationSnapshot {
  scheduleQuietHours: QuietHoursConfig;
  scheduleCooldown: CooldownConfig;
  scheduleGrouping: GroupingConfig;
  scheduleEscalation: EscalationConfig;
  notifyOnResolve: boolean;
  guestDefaults: Record<string, number | undefined>;
  guestDisableConnectivity: boolean;
  guestPoweredOffSeverity: 'warning' | 'critical';
  nodeDefaults: Record<string, number | undefined>;
  pbsDefaults: Record<string, number | undefined>;
  kubernetesDefaults: Record<string, number | undefined>;
  trueNASDefaults: Record<string, number | undefined>;
  trueNASDiskDefaults: Record<string, number | undefined>;
  vmwareDefaults: Record<string, number | undefined>;
  agentDefaults: Record<string, number | undefined>;
  dockerDefaults: typeof FACTORY_DOCKER_DEFAULTS;
  dockerDisableConnectivity: boolean;
  dockerPoweredOffSeverity: 'warning' | 'critical';
  dockerIgnoredPrefixes: string[];
  ignoredGuestPrefixes: string[];
  guestTagWhitelist: string[];
  guestTagBlacklist: string[];
  storageDefault: number;
  backupDefaults: BackupAlertConfig;
  timeThresholds: {
    guest: number;
    node: number;
    storage: number;
    pbs: number;
    agent: number;
    'k8s-cluster': number;
    'k8s-node': number;
    'k8s-deployment': number;
    'k8s-namespace': number;
    pod: number;
    'truenas-system': number;
    'truenas-pool': number;
    'truenas-dataset': number;
    'truenas-disk': number;
    'vmware-host': number;
    'vmware-vm': number;
    'vmware-datastore': number;
    'vmware-network': number;
  };
  metricTimeThresholds: Record<string, Record<string, number>>;
  snapshotDefaults: SnapshotAlertConfig;
  pmgThresholds: {
    queueTotalWarning: number;
    queueTotalCritical: number;
    oldestMessageWarnMins: number;
    oldestMessageCritMins: number;
    deferredQueueWarn: number;
    deferredQueueCritical: number;
    holdQueueWarn: number;
    holdQueueCritical: number;
    quarantineSpamWarn: number;
    quarantineSpamCritical: number;
    quarantineVirusWarn: number;
    quarantineVirusCritical: number;
    quarantineGrowthWarnPct: number;
    quarantineGrowthWarnMin: number;
    quarantineGrowthCritPct: number;
    quarantineGrowthCritMin: number;
  };
  disableAllNodes: boolean;
  disableAllGuests: boolean;
  disableAllAgents: boolean;
  disableAllStorage: boolean;
  disableAllPBS: boolean;
  disableAllPMG: boolean;
  disableAllDockerHosts: boolean;
  disableAllDockerServices: boolean;
  disableAllDockerContainers: boolean;
  disableAllKubernetes: boolean;
  disableAllTrueNAS: boolean;
  disableAllVMware: boolean;
  disableAllNodesOffline: boolean;
  disableAllGuestsOffline: boolean;
  disableAllAgentsOffline: boolean;
  disableAllPBSOffline: boolean;
  disableAllPMGOffline: boolean;
  disableAllDockerHostsOffline: boolean;
}

export interface BuildAlertsConfigurationPayloadArgs {
  snapshot: AlertsConfigurationSnapshot;
  rawOverridesConfig: AlertConfig['overrides'];
  alertsActivationState: ActivationState | null;
  alertsActivationConfig: {
    enabled?: boolean;
    activationTime?: string | null;
    observationWindowHours?: number | null;
  } | null;
}

export const ALERT_DOCKER_GAP_VALIDATION_ERROR =
  'Swarm service critical gap must be greater than or equal to the warning gap when enabled.';

const cloneDays = (days: QuietHoursConfig['days']): QuietHoursConfig['days'] => ({ ...days });

const cloneBackupDefaults = (backupDefaults: BackupAlertConfig): BackupAlertConfig => ({
  ...backupDefaults,
  ignoreVMIDs: [...(backupDefaults.ignoreVMIDs ?? [])],
});

const createHysteresisThreshold = (trigger: number | undefined, clearMargin = 5) => {
  const normalized = typeof trigger === 'number' ? trigger : 0;
  return {
    trigger: normalized,
    clear: Math.max(0, normalized - clearMargin),
  };
};

const normalizeGap = (value: unknown, fallback: number) => {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return fallback;
  }
  return Math.max(0, Math.min(100, numeric));
};

const normalizeWarningCriticalPair = (
  warning: number | undefined,
  critical: number | undefined,
): { warning: number; critical: number } => {
  const normalizedWarning = Math.max(0, warning ?? 0);
  const safeCritical = Math.max(0, critical ?? 0);
  const finalWarning =
    safeCritical > 0 && normalizedWarning > safeCritical ? safeCritical : normalizedWarning;
  const finalCritical = safeCritical > 0 ? Math.max(safeCritical, finalWarning) : finalWarning;
  return {
    warning: finalWarning,
    critical: finalCritical,
  };
};

const normalizeStringList = (values: string[] | undefined): string[] =>
  (values ?? []).map((value) => value.trim()).filter((value) => value.length > 0);

export function createDefaultAlertsConfigurationSnapshot(): AlertsConfigurationSnapshot {
  return {
    scheduleQuietHours: createDefaultQuietHours(),
    scheduleCooldown: createDefaultCooldown(),
    scheduleGrouping: createDefaultGrouping(),
    scheduleEscalation: createDefaultEscalation(),
    notifyOnResolve: createDefaultResolveNotifications(),
    guestDefaults: { ...FACTORY_GUEST_DEFAULTS },
    guestDisableConnectivity: false,
    guestPoweredOffSeverity: 'warning',
    nodeDefaults: { ...FACTORY_NODE_DEFAULTS },
    pbsDefaults: { ...FACTORY_PBS_DEFAULTS },
    kubernetesDefaults: { ...FACTORY_KUBERNETES_DEFAULTS },
    trueNASDefaults: { ...FACTORY_TRUENAS_DEFAULTS },
    trueNASDiskDefaults: { ...FACTORY_TRUENAS_DISK_DEFAULTS },
    vmwareDefaults: { ...FACTORY_VMWARE_DEFAULTS },
    agentDefaults: { ...FACTORY_AGENT_DEFAULTS },
    dockerDefaults: { ...FACTORY_DOCKER_DEFAULTS },
    dockerDisableConnectivity: FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY,
    dockerPoweredOffSeverity: FACTORY_DOCKER_STATE_SEVERITY,
    dockerIgnoredPrefixes: [],
    ignoredGuestPrefixes: [],
    guestTagWhitelist: [],
    guestTagBlacklist: [],
    storageDefault: FACTORY_STORAGE_DEFAULT,
    backupDefaults: cloneBackupDefaults(FACTORY_BACKUP_DEFAULTS),
    timeThresholds: {
      guest: DEFAULT_DELAY_SECONDS,
      node: DEFAULT_DELAY_SECONDS,
      storage: DEFAULT_DELAY_SECONDS,
      pbs: DEFAULT_DELAY_SECONDS,
      agent: DEFAULT_DELAY_SECONDS,
      'k8s-cluster': DEFAULT_DELAY_SECONDS,
      'k8s-node': DEFAULT_DELAY_SECONDS,
      'k8s-deployment': DEFAULT_DELAY_SECONDS,
      'k8s-namespace': DEFAULT_DELAY_SECONDS,
      pod: DEFAULT_DELAY_SECONDS,
      'truenas-system': DEFAULT_DELAY_SECONDS,
      'truenas-pool': DEFAULT_DELAY_SECONDS,
      'truenas-dataset': DEFAULT_DELAY_SECONDS,
      'truenas-disk': DEFAULT_DELAY_SECONDS,
      'vmware-host': DEFAULT_DELAY_SECONDS,
      'vmware-vm': DEFAULT_DELAY_SECONDS,
      'vmware-datastore': DEFAULT_DELAY_SECONDS,
      'vmware-network': DEFAULT_DELAY_SECONDS,
    },
    metricTimeThresholds: {},
    snapshotDefaults: { ...FACTORY_SNAPSHOT_DEFAULTS },
    pmgThresholds: {
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
    },
    disableAllNodes: false,
    disableAllGuests: false,
    disableAllAgents: false,
    disableAllStorage: false,
    disableAllPBS: false,
    disableAllPMG: false,
    disableAllDockerHosts: false,
    disableAllDockerServices: false,
    disableAllDockerContainers: false,
    disableAllKubernetes: false,
    disableAllTrueNAS: false,
    disableAllVMware: false,
    disableAllNodesOffline: false,
    disableAllGuestsOffline: false,
    disableAllAgentsOffline: false,
    disableAllPBSOffline: false,
    disableAllPMGOffline: false,
    disableAllDockerHostsOffline: false,
  };
}

export function readAlertsConfigurationSnapshot(config: AlertConfig): AlertsConfigurationSnapshot {
  const snapshot = createDefaultAlertsConfigurationSnapshot();

  if (config.guestDefaults) {
    snapshot.guestDefaults = {
      cpu: getTriggerValue(config.guestDefaults.cpu) ?? FACTORY_GUEST_DEFAULTS.cpu,
      memory: getTriggerValue(config.guestDefaults.memory) ?? FACTORY_GUEST_DEFAULTS.memory,
      disk: getTriggerValue(config.guestDefaults.disk) ?? FACTORY_GUEST_DEFAULTS.disk,
      diskRead: getTriggerValue(config.guestDefaults.diskRead) ?? FACTORY_GUEST_DEFAULTS.diskRead,
      diskWrite:
        getTriggerValue(config.guestDefaults.diskWrite) ?? FACTORY_GUEST_DEFAULTS.diskWrite,
      networkIn:
        getTriggerValue(config.guestDefaults.networkIn) ?? FACTORY_GUEST_DEFAULTS.networkIn,
      networkOut:
        getTriggerValue(config.guestDefaults.networkOut) ?? FACTORY_GUEST_DEFAULTS.networkOut,
    };
    snapshot.guestDisableConnectivity = Boolean(config.guestDefaults.disableConnectivity);
    snapshot.guestPoweredOffSeverity =
      config.guestDefaults.poweredOffSeverity === 'critical' ? 'critical' : 'warning';
  }

  if (config.nodeDefaults) {
    snapshot.nodeDefaults = {
      cpu: getTriggerValue(config.nodeDefaults.cpu) ?? FACTORY_NODE_DEFAULTS.cpu,
      memory: getTriggerValue(config.nodeDefaults.memory) ?? FACTORY_NODE_DEFAULTS.memory,
      disk: getTriggerValue(config.nodeDefaults.disk) ?? FACTORY_NODE_DEFAULTS.disk,
      temperature:
        getTriggerValue(config.nodeDefaults.temperature) ?? FACTORY_NODE_DEFAULTS.temperature,
    };
  }

  if (config.pbsDefaults) {
    snapshot.pbsDefaults = {
      cpu: getTriggerValue(config.pbsDefaults.cpu) ?? FACTORY_PBS_DEFAULTS.cpu,
      memory: getTriggerValue(config.pbsDefaults.memory) ?? FACTORY_PBS_DEFAULTS.memory,
    };
  }

  if (config.kubernetesDefaults) {
    snapshot.kubernetesDefaults = {
      cpu: getTriggerValue(config.kubernetesDefaults.cpu) ?? FACTORY_KUBERNETES_DEFAULTS.cpu,
      memory:
        getTriggerValue(config.kubernetesDefaults.memory) ?? FACTORY_KUBERNETES_DEFAULTS.memory,
      disk: getTriggerValue(config.kubernetesDefaults.disk) ?? FACTORY_KUBERNETES_DEFAULTS.disk,
      diskRead:
        getTriggerValue(config.kubernetesDefaults.diskRead) ?? FACTORY_KUBERNETES_DEFAULTS.diskRead,
      diskWrite:
        getTriggerValue(config.kubernetesDefaults.diskWrite) ??
        FACTORY_KUBERNETES_DEFAULTS.diskWrite,
      networkIn:
        getTriggerValue(config.kubernetesDefaults.networkIn) ??
        FACTORY_KUBERNETES_DEFAULTS.networkIn,
      networkOut:
        getTriggerValue(config.kubernetesDefaults.networkOut) ??
        FACTORY_KUBERNETES_DEFAULTS.networkOut,
    };
  }

  if (config.truenasDefaults) {
    snapshot.trueNASDefaults = {
      cpu: getTriggerValue(config.truenasDefaults.cpu) ?? FACTORY_TRUENAS_DEFAULTS.cpu,
      memory: getTriggerValue(config.truenasDefaults.memory) ?? FACTORY_TRUENAS_DEFAULTS.memory,
      disk: getTriggerValue(config.truenasDefaults.disk) ?? FACTORY_TRUENAS_DEFAULTS.disk,
      usage: getTriggerValue(config.truenasDefaults.usage) ?? FACTORY_TRUENAS_DEFAULTS.usage,
      temperature:
        getTriggerValue(config.truenasDefaults.temperature) ?? FACTORY_TRUENAS_DEFAULTS.temperature,
      diskRead:
        getTriggerValue(config.truenasDefaults.diskRead) ?? FACTORY_TRUENAS_DEFAULTS.diskRead,
      diskWrite:
        getTriggerValue(config.truenasDefaults.diskWrite) ?? FACTORY_TRUENAS_DEFAULTS.diskWrite,
      networkIn:
        getTriggerValue(config.truenasDefaults.networkIn) ?? FACTORY_TRUENAS_DEFAULTS.networkIn,
      networkOut:
        getTriggerValue(config.truenasDefaults.networkOut) ?? FACTORY_TRUENAS_DEFAULTS.networkOut,
    };
  }

  if (config.truenasDiskDefaults) {
    snapshot.trueNASDiskDefaults = {
      temperature:
        getTriggerValue(config.truenasDiskDefaults.temperature) ??
        FACTORY_TRUENAS_DISK_DEFAULTS.temperature,
    };
  }

  if (config.vmwareDefaults) {
    snapshot.vmwareDefaults = {
      cpu: getTriggerValue(config.vmwareDefaults.cpu) ?? FACTORY_VMWARE_DEFAULTS.cpu,
      memory: getTriggerValue(config.vmwareDefaults.memory) ?? FACTORY_VMWARE_DEFAULTS.memory,
      disk: getTriggerValue(config.vmwareDefaults.disk) ?? FACTORY_VMWARE_DEFAULTS.disk,
      usage: getTriggerValue(config.vmwareDefaults.usage) ?? FACTORY_VMWARE_DEFAULTS.usage,
      diskRead: getTriggerValue(config.vmwareDefaults.diskRead) ?? FACTORY_VMWARE_DEFAULTS.diskRead,
      diskWrite:
        getTriggerValue(config.vmwareDefaults.diskWrite) ?? FACTORY_VMWARE_DEFAULTS.diskWrite,
      networkIn:
        getTriggerValue(config.vmwareDefaults.networkIn) ?? FACTORY_VMWARE_DEFAULTS.networkIn,
      networkOut:
        getTriggerValue(config.vmwareDefaults.networkOut) ?? FACTORY_VMWARE_DEFAULTS.networkOut,
    };
  }

  if (config.agentDefaults) {
    snapshot.agentDefaults = {
      cpu: getTriggerValue(config.agentDefaults.cpu) ?? FACTORY_AGENT_DEFAULTS.cpu,
      memory: getTriggerValue(config.agentDefaults.memory) ?? FACTORY_AGENT_DEFAULTS.memory,
      disk: getTriggerValue(config.agentDefaults.disk) ?? FACTORY_AGENT_DEFAULTS.disk,
      diskTemperature:
        getTriggerValue(config.agentDefaults.diskTemperature) ??
        FACTORY_AGENT_DEFAULTS.diskTemperature,
    };
  }

  if (config.dockerDefaults) {
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

    snapshot.dockerDefaults = {
      cpu: getTriggerValue(config.dockerDefaults.cpu) ?? FACTORY_DOCKER_DEFAULTS.cpu,
      memory: getTriggerValue(config.dockerDefaults.memory) ?? FACTORY_DOCKER_DEFAULTS.memory,
      disk: getTriggerValue(config.dockerDefaults.disk) ?? FACTORY_DOCKER_DEFAULTS.disk,
      restartCount: config.dockerDefaults.restartCount ?? FACTORY_DOCKER_DEFAULTS.restartCount,
      restartWindow: config.dockerDefaults.restartWindow ?? FACTORY_DOCKER_DEFAULTS.restartWindow,
      memoryWarnPct: config.dockerDefaults.memoryWarnPct ?? FACTORY_DOCKER_DEFAULTS.memoryWarnPct,
      memoryCriticalPct:
        config.dockerDefaults.memoryCriticalPct ?? FACTORY_DOCKER_DEFAULTS.memoryCriticalPct,
      serviceWarnGapPercent: serviceWarnGap,
      serviceCriticalGapPercent: serviceCriticalGap,
    };
    snapshot.dockerDisableConnectivity = Boolean(config.dockerDefaults.stateDisableConnectivity);
    snapshot.dockerPoweredOffSeverity =
      config.dockerDefaults.statePoweredOffSeverity === 'critical' ? 'critical' : 'warning';
  }

  snapshot.dockerIgnoredPrefixes = [...(config.dockerIgnoredContainerPrefixes ?? [])];
  snapshot.ignoredGuestPrefixes = [...(config.ignoredGuestPrefixes ?? [])];
  snapshot.guestTagWhitelist = [...(config.guestTagWhitelist ?? [])];
  snapshot.guestTagBlacklist = [...(config.guestTagBlacklist ?? [])];

  if (config.storageDefault) {
    snapshot.storageDefault = getTriggerValue(config.storageDefault) ?? FACTORY_STORAGE_DEFAULT;
  }
  if (config.timeThresholds) {
    snapshot.timeThresholds = {
      guest: config.timeThresholds.guest ?? DEFAULT_DELAY_SECONDS,
      node: config.timeThresholds.node ?? DEFAULT_DELAY_SECONDS,
      storage: config.timeThresholds.storage ?? DEFAULT_DELAY_SECONDS,
      pbs: config.timeThresholds.pbs ?? DEFAULT_DELAY_SECONDS,
      agent: config.timeThresholds.agent ?? DEFAULT_DELAY_SECONDS,
      'k8s-cluster': config.timeThresholds['k8s-cluster'] ?? DEFAULT_DELAY_SECONDS,
      'k8s-node': config.timeThresholds['k8s-node'] ?? DEFAULT_DELAY_SECONDS,
      'k8s-deployment': config.timeThresholds['k8s-deployment'] ?? DEFAULT_DELAY_SECONDS,
      'k8s-namespace': config.timeThresholds['k8s-namespace'] ?? DEFAULT_DELAY_SECONDS,
      pod: config.timeThresholds.pod ?? DEFAULT_DELAY_SECONDS,
      'truenas-system': config.timeThresholds['truenas-system'] ?? DEFAULT_DELAY_SECONDS,
      'truenas-pool': config.timeThresholds['truenas-pool'] ?? DEFAULT_DELAY_SECONDS,
      'truenas-dataset': config.timeThresholds['truenas-dataset'] ?? DEFAULT_DELAY_SECONDS,
      'truenas-disk': config.timeThresholds['truenas-disk'] ?? DEFAULT_DELAY_SECONDS,
      'vmware-host': config.timeThresholds['vmware-host'] ?? DEFAULT_DELAY_SECONDS,
      'vmware-vm': config.timeThresholds['vmware-vm'] ?? DEFAULT_DELAY_SECONDS,
      'vmware-datastore': config.timeThresholds['vmware-datastore'] ?? DEFAULT_DELAY_SECONDS,
      'vmware-network': config.timeThresholds['vmware-network'] ?? DEFAULT_DELAY_SECONDS,
    };
  }
  if (config.metricTimeThresholds) {
    snapshot.metricTimeThresholds = normalizeMetricDelayMap(config.metricTimeThresholds);
  }

  if (config.backupDefaults) {
    const normalizedPair = normalizeWarningCriticalPair(
      config.backupDefaults.warningDays,
      config.backupDefaults.criticalDays,
    );
    snapshot.backupDefaults = {
      enabled: Boolean(config.backupDefaults.enabled),
      warningDays: normalizedPair.warning,
      criticalDays: normalizedPair.critical,
      freshHours: config.backupDefaults.freshHours ?? FACTORY_BACKUP_DEFAULTS.freshHours,
      staleHours: config.backupDefaults.staleHours ?? FACTORY_BACKUP_DEFAULTS.staleHours,
      alertOrphaned:
        config.backupDefaults.alertOrphaned ?? FACTORY_BACKUP_DEFAULTS.alertOrphaned ?? true,
      ignoreVMIDs: Array.from(
        new Set(
          normalizeStringList(
            config.backupDefaults.ignoreVMIDs ?? FACTORY_BACKUP_DEFAULTS.ignoreVMIDs,
          ),
        ),
      ),
    };
  }

  if (config.snapshotDefaults) {
    const normalizedPair = normalizeWarningCriticalPair(
      config.snapshotDefaults.warningDays,
      config.snapshotDefaults.criticalDays,
    );
    snapshot.snapshotDefaults = {
      enabled: Boolean(config.snapshotDefaults.enabled),
      warningDays: normalizedPair.warning,
      criticalDays: normalizedPair.critical,
    };
  }

  if (config.pmgDefaults) {
    snapshot.pmgThresholds = {
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
    };
  }

  snapshot.disableAllNodes = config.disableAllNodes ?? false;
  snapshot.disableAllGuests = config.disableAllGuests ?? false;
  snapshot.disableAllAgents = config.disableAllAgents ?? false;
  snapshot.disableAllStorage = config.disableAllStorage ?? false;
  snapshot.disableAllPBS = config.disableAllPBS ?? false;
  snapshot.disableAllPMG = config.disableAllPMG ?? false;
  snapshot.disableAllDockerHosts = config.disableAllDockerHosts ?? false;
  snapshot.disableAllDockerServices = config.disableAllDockerServices ?? false;
  snapshot.disableAllDockerContainers = config.disableAllDockerContainers ?? false;
  snapshot.disableAllKubernetes = config.disableAllKubernetes ?? false;
  snapshot.disableAllTrueNAS = config.disableAllTrueNAS ?? false;
  snapshot.disableAllVMware = config.disableAllVMware ?? false;
  snapshot.disableAllNodesOffline = config.disableAllNodesOffline ?? false;
  snapshot.disableAllGuestsOffline = config.disableAllGuestsOffline ?? false;
  snapshot.disableAllAgentsOffline = config.disableAllAgentsOffline ?? false;
  snapshot.disableAllPBSOffline = config.disableAllPBSOffline ?? false;
  snapshot.disableAllPMGOffline = config.disableAllPMGOffline ?? false;
  snapshot.disableAllDockerHostsOffline = config.disableAllDockerHostsOffline ?? false;

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
        : (quietHours.days as Record<string, boolean>) || createDefaultQuietHours().days;
      snapshot.scheduleQuietHours = {
        enabled: quietHours.enabled || false,
        start: quietHours.start || '22:00',
        end: quietHours.end || '08:00',
        timezone: quietHours.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
        days,
        suppress: {
          performance: quietHours.suppress?.performance ?? false,
          storage: quietHours.suppress?.storage ?? false,
          offline: quietHours.suppress?.offline ?? false,
        },
      };
    }

    if (config.schedule.cooldown !== undefined) {
      const rawCooldown = config.schedule.cooldown;
      const cooldownEnabled = rawCooldown > 0;
      snapshot.scheduleCooldown = {
        enabled: cooldownEnabled,
        minutes: cooldownEnabled ? clampCooldownMinutes(rawCooldown) : 0,
        maxAlerts: fallbackMaxAlertsPerHour(config.schedule.maxAlertsHour),
      };
    }

    if (config.schedule.grouping) {
      const groupingConfig = config.schedule.grouping;
      const rawGroupingWindowSeconds =
        typeof groupingConfig.window === 'number'
          ? groupingConfig.window
          : GROUPING_WINDOW_DEFAULT_SECONDS;
      const normalizedGroupingWindowSeconds = Math.max(0, rawGroupingWindowSeconds);
      snapshot.scheduleGrouping = {
        enabled:
          groupingConfig.enabled !== undefined
            ? Boolean(groupingConfig.enabled)
            : normalizedGroupingWindowSeconds > 0,
        window: Math.round(normalizedGroupingWindowSeconds / 60),
        byNode: groupingConfig.byNode !== undefined ? groupingConfig.byNode : true,
        byGuest: groupingConfig.byGuest !== undefined ? groupingConfig.byGuest : false,
      };
    }

    if (config.schedule.notifyOnResolve !== undefined) {
      snapshot.notifyOnResolve = Boolean(config.schedule.notifyOnResolve);
    }

    if (config.schedule.escalation) {
      snapshot.scheduleEscalation = {
        enabled: Boolean(config.schedule.escalation.enabled),
        levels: (config.schedule.escalation.levels || []).map((level) => ({
          after: typeof level.after === 'number' ? level.after : 15,
          notify: (level.notify as EscalationNotifyTarget) || 'all',
        })),
      };
    }
  }

  return snapshot;
}

export function buildAlertsConfigurationPayload({
  snapshot,
  rawOverridesConfig,
  alertsActivationState,
  alertsActivationConfig,
}: BuildAlertsConfigurationPayloadArgs): {
  alertConfig?: AlertConfig;
  dockerValidationError?: string;
} {
  if (
    snapshot.dockerDefaults.serviceCriticalGapPercent > 0 &&
    snapshot.dockerDefaults.serviceWarnGapPercent >
      snapshot.dockerDefaults.serviceCriticalGapPercent
  ) {
    return {
      dockerValidationError: ALERT_DOCKER_GAP_VALIDATION_ERROR,
    };
  }

  const normalizedSnapshotPair = normalizeWarningCriticalPair(
    snapshot.snapshotDefaults.warningDays,
    snapshot.snapshotDefaults.criticalDays,
  );
  const normalizedBackupPair = normalizeWarningCriticalPair(
    snapshot.backupDefaults.warningDays,
    snapshot.backupDefaults.criticalDays,
  );
  const normalizedCooldownMinutes = snapshot.scheduleCooldown.enabled
    ? clampCooldownMinutes(snapshot.scheduleCooldown.minutes)
    : 0;
  const normalizedMaxAlertsHour = clampMaxAlertsPerHour(snapshot.scheduleCooldown.maxAlerts);
  const groupingWindowSeconds =
    snapshot.scheduleGrouping.enabled && snapshot.scheduleGrouping.window >= 0
      ? snapshot.scheduleGrouping.window * 60
      : 0;
  const groupingEnabled = snapshot.scheduleGrouping.enabled && groupingWindowSeconds > 0;

  return {
    alertConfig: {
      enabled: alertsActivationConfig?.enabled ?? true,
      activationState: alertsActivationState ?? undefined,
      activationTime: alertsActivationConfig?.activationTime ?? undefined,
      observationWindowHours: alertsActivationConfig?.observationWindowHours ?? undefined,
      disableAllNodes: snapshot.disableAllNodes,
      disableAllGuests: snapshot.disableAllGuests,
      disableAllAgents: snapshot.disableAllAgents,
      disableAllStorage: snapshot.disableAllStorage,
      disableAllPBS: snapshot.disableAllPBS,
      disableAllPMG: snapshot.disableAllPMG,
      disableAllDockerHosts: snapshot.disableAllDockerHosts,
      disableAllDockerContainers: snapshot.disableAllDockerContainers,
      disableAllDockerServices: snapshot.disableAllDockerServices,
      disableAllKubernetes: snapshot.disableAllKubernetes,
      disableAllTrueNAS: snapshot.disableAllTrueNAS,
      disableAllVMware: snapshot.disableAllVMware,
      disableAllNodesOffline: snapshot.disableAllNodesOffline,
      disableAllGuestsOffline: snapshot.disableAllGuestsOffline,
      disableAllPBSOffline: snapshot.disableAllPBSOffline,
      disableAllAgentsOffline: snapshot.disableAllAgentsOffline,
      disableAllPMGOffline: snapshot.disableAllPMGOffline,
      disableAllDockerHostsOffline: snapshot.disableAllDockerHostsOffline,
      guestDefaults: {
        cpu: createHysteresisThreshold(snapshot.guestDefaults.cpu),
        memory: createHysteresisThreshold(snapshot.guestDefaults.memory),
        disk: createHysteresisThreshold(snapshot.guestDefaults.disk),
        diskRead: createHysteresisThreshold(snapshot.guestDefaults.diskRead),
        diskWrite: createHysteresisThreshold(snapshot.guestDefaults.diskWrite),
        networkIn: createHysteresisThreshold(snapshot.guestDefaults.networkIn),
        networkOut: createHysteresisThreshold(snapshot.guestDefaults.networkOut),
        disableConnectivity: snapshot.guestDisableConnectivity,
        poweredOffSeverity: snapshot.guestPoweredOffSeverity,
      },
      nodeDefaults: {
        cpu: createHysteresisThreshold(snapshot.nodeDefaults.cpu),
        memory: createHysteresisThreshold(snapshot.nodeDefaults.memory),
        disk: createHysteresisThreshold(snapshot.nodeDefaults.disk),
        temperature: createHysteresisThreshold(snapshot.nodeDefaults.temperature),
      },
      agentDefaults: {
        cpu: createHysteresisThreshold(snapshot.agentDefaults.cpu),
        memory: createHysteresisThreshold(snapshot.agentDefaults.memory),
        disk: createHysteresisThreshold(snapshot.agentDefaults.disk),
        diskTemperature: createHysteresisThreshold(snapshot.agentDefaults.diskTemperature),
      },
      pbsDefaults: {
        cpu: createHysteresisThreshold(snapshot.pbsDefaults.cpu),
        memory: createHysteresisThreshold(snapshot.pbsDefaults.memory),
      },
      kubernetesDefaults: {
        cpu: createHysteresisThreshold(snapshot.kubernetesDefaults.cpu),
        memory: createHysteresisThreshold(snapshot.kubernetesDefaults.memory),
        disk: createHysteresisThreshold(snapshot.kubernetesDefaults.disk),
        diskRead: createHysteresisThreshold(snapshot.kubernetesDefaults.diskRead),
        diskWrite: createHysteresisThreshold(snapshot.kubernetesDefaults.diskWrite),
        networkIn: createHysteresisThreshold(snapshot.kubernetesDefaults.networkIn),
        networkOut: createHysteresisThreshold(snapshot.kubernetesDefaults.networkOut),
      },
      truenasDefaults: {
        cpu: createHysteresisThreshold(snapshot.trueNASDefaults.cpu),
        memory: createHysteresisThreshold(snapshot.trueNASDefaults.memory),
        disk: createHysteresisThreshold(snapshot.trueNASDefaults.disk),
        usage: createHysteresisThreshold(snapshot.trueNASDefaults.usage),
        temperature: createHysteresisThreshold(snapshot.trueNASDefaults.temperature),
        diskRead: createHysteresisThreshold(snapshot.trueNASDefaults.diskRead),
        diskWrite: createHysteresisThreshold(snapshot.trueNASDefaults.diskWrite),
        networkIn: createHysteresisThreshold(snapshot.trueNASDefaults.networkIn),
        networkOut: createHysteresisThreshold(snapshot.trueNASDefaults.networkOut),
      },
      truenasDiskDefaults: {
        temperature: createHysteresisThreshold(snapshot.trueNASDiskDefaults.temperature),
      },
      vmwareDefaults: {
        cpu: createHysteresisThreshold(snapshot.vmwareDefaults.cpu),
        memory: createHysteresisThreshold(snapshot.vmwareDefaults.memory),
        disk: createHysteresisThreshold(snapshot.vmwareDefaults.disk),
        usage: createHysteresisThreshold(snapshot.vmwareDefaults.usage),
        diskRead: createHysteresisThreshold(snapshot.vmwareDefaults.diskRead),
        diskWrite: createHysteresisThreshold(snapshot.vmwareDefaults.diskWrite),
        networkIn: createHysteresisThreshold(snapshot.vmwareDefaults.networkIn),
        networkOut: createHysteresisThreshold(snapshot.vmwareDefaults.networkOut),
      },
      dockerDefaults: {
        cpu: createHysteresisThreshold(snapshot.dockerDefaults.cpu),
        memory: createHysteresisThreshold(snapshot.dockerDefaults.memory),
        disk: createHysteresisThreshold(snapshot.dockerDefaults.disk),
        restartCount: snapshot.dockerDefaults.restartCount,
        restartWindow: snapshot.dockerDefaults.restartWindow,
        memoryWarnPct: snapshot.dockerDefaults.memoryWarnPct,
        memoryCriticalPct: snapshot.dockerDefaults.memoryCriticalPct,
        serviceWarnGapPercent: snapshot.dockerDefaults.serviceWarnGapPercent,
        serviceCriticalGapPercent: snapshot.dockerDefaults.serviceCriticalGapPercent,
        stateDisableConnectivity: snapshot.dockerDisableConnectivity,
        statePoweredOffSeverity: snapshot.dockerPoweredOffSeverity,
      },
      dockerIgnoredContainerPrefixes: normalizeStringList(snapshot.dockerIgnoredPrefixes),
      ignoredGuestPrefixes: normalizeStringList(snapshot.ignoredGuestPrefixes),
      guestTagWhitelist: normalizeStringList(snapshot.guestTagWhitelist),
      guestTagBlacklist: normalizeStringList(snapshot.guestTagBlacklist),
      storageDefault: createHysteresisThreshold(snapshot.storageDefault),
      minimumDelta: 2.0,
      suppressionWindow: 5,
      hysteresisMargin: 5.0,
      timeThresholds: { ...snapshot.timeThresholds },
      metricTimeThresholds: normalizeMetricDelayMap(snapshot.metricTimeThresholds),
      snapshotDefaults: {
        enabled: snapshot.snapshotDefaults.enabled,
        warningDays: normalizedSnapshotPair.warning,
        criticalDays: normalizedSnapshotPair.critical,
      },
      backupDefaults: {
        enabled: snapshot.backupDefaults.enabled,
        warningDays: normalizedBackupPair.warning,
        criticalDays: normalizedBackupPair.critical,
        freshHours: snapshot.backupDefaults.freshHours ?? 24,
        staleHours: snapshot.backupDefaults.staleHours ?? 72,
        alertOrphaned: snapshot.backupDefaults.alertOrphaned ?? true,
        ignoreVMIDs: normalizeStringList(snapshot.backupDefaults.ignoreVMIDs),
      },
      pmgDefaults: { ...snapshot.pmgThresholds },
      overrides: rawOverridesConfig,
      schedule: {
        quietHours: {
          ...snapshot.scheduleQuietHours,
          days: cloneDays(snapshot.scheduleQuietHours.days),
        },
        cooldown: normalizedCooldownMinutes,
        notifyOnResolve: snapshot.notifyOnResolve,
        maxAlertsHour: normalizedMaxAlertsHour,
        escalation: {
          enabled: snapshot.scheduleEscalation.enabled,
          levels: snapshot.scheduleEscalation.levels.map((level) => ({ ...level })),
        },
        grouping: {
          enabled: groupingEnabled,
          window: groupingWindowSeconds,
          byNode: snapshot.scheduleGrouping.byNode,
          byGuest: snapshot.scheduleGrouping.byGuest,
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
    },
  };
}
