import type { FilterStack } from '@/utils/searchQuery';

export interface HysteresisThreshold {
  trigger: number;
  clear: number;
}

export interface AlertThresholds {
  cpu?: HysteresisThreshold;
  memory?: HysteresisThreshold;
  disk?: HysteresisThreshold;
  diskRead?: HysteresisThreshold;
  diskWrite?: HysteresisThreshold;
  networkIn?: HysteresisThreshold;
  networkOut?: HysteresisThreshold;
  temperature?: HysteresisThreshold;
  diskTemperature?: HysteresisThreshold;
  disableConnectivity?: boolean; // Disable connectivity/powered-off alerts
  poweredOffSeverity?: 'warning' | 'critical';
  // Legacy support for backward compatibility
  cpuLegacy?: number;
  memoryLegacy?: number;
  diskLegacy?: number;
  diskReadLegacy?: number;
  diskWriteLegacy?: number;
  networkInLegacy?: number;
  networkOutLegacy?: number;
  // Allow indexing with string
  [key: string]: HysteresisThreshold | BackupAlertConfig | SnapshotAlertConfig | number | boolean | string | undefined;
}

export type RawOverrideConfig = AlertThresholds & {
  disabled?: boolean;
  disableConnectivity?: boolean;
  poweredOffSeverity?: 'warning' | 'critical';
  note?: string;
  backup?: BackupAlertConfig;
  snapshot?: SnapshotAlertConfig;
  // NOTE: To disable individual metrics, set threshold to -1
};

export interface CustomAlertRule {
  id: string;
  name: string;
  description?: string;
  filterConditions: FilterStack;
  thresholds: AlertThresholds;
  priority: number;
  enabled: boolean;
  notifications: {
    email?: {
      enabled: boolean;
      recipients: string[];
    };
    webhook?: {
      enabled: boolean;
      url: string;
    };
  };
  createdAt: string;
  updatedAt: string;
}

export interface DockerThresholdConfig {
  cpu?: HysteresisThreshold;
  memory?: HysteresisThreshold;
  disk?: HysteresisThreshold;
  restartCount?: number;
  restartWindow?: number;
  memoryWarnPct?: number;
  memoryCriticalPct?: number;
  serviceWarnGapPercent?: number;
  serviceCriticalGapPercent?: number;
  stateDisableConnectivity?: boolean;
  statePoweredOffSeverity?: 'warning' | 'critical';
}

export interface PMGThresholdDefaults {
  queueTotalWarning?: number;
  queueTotalCritical?: number;
  oldestMessageWarnMins?: number;
  oldestMessageCritMins?: number;
  deferredQueueWarn?: number;
  deferredQueueCritical?: number;
  holdQueueWarn?: number;
  holdQueueCritical?: number;
  quarantineSpamWarn?: number;
  quarantineSpamCritical?: number;
  quarantineVirusWarn?: number;
  quarantineVirusCritical?: number;
  quarantineGrowthWarnPct?: number;
  quarantineGrowthWarnMin?: number;
  quarantineGrowthCritPct?: number;
  quarantineGrowthCritMin?: number;
}

export interface SnapshotAlertConfig {
  enabled: boolean;
  warningDays: number;
  criticalDays: number;
  warningSizeGiB?: number;
  criticalSizeGiB?: number;
}

export interface BackupAlertConfig {
  enabled: boolean;
  warningDays: number;
  criticalDays: number;
  // Dashboard indicator thresholds (separate from alert thresholds)
  freshHours?: number; // Backups newer than this show as green (default: 24)
  staleHours?: number; // Backups older than freshHours but newer than this show as amber (default: 72)
}

export type ActivationState = 'pending_review' | 'active' | 'snoozed';

export interface AlertConfig {
  enabled: boolean;
  activationState?: ActivationState;
  observationWindowHours?: number;
  activationTime?: string;
  guestDefaults: AlertThresholds;
  nodeDefaults: AlertThresholds;
  hostDefaults?: AlertThresholds;
  storageDefault: HysteresisThreshold;
  dockerDefaults?: DockerThresholdConfig;
  dockerIgnoredContainerPrefixes?: string[];
  ignoredGuestPrefixes?: string[];
  guestTagWhitelist?: string[];
  guestTagBlacklist?: string[];
  pmgDefaults?: PMGThresholdDefaults;
  snapshotDefaults?: SnapshotAlertConfig;
  backupDefaults?: BackupAlertConfig;
  customRules?: CustomAlertRule[];
  overrides: Record<string, RawOverrideConfig>; // key: resource ID
  minimumDelta?: number;
  suppressionWindow?: number;
  hysteresisMargin?: number;
  timeThreshold?: number; // Legacy single global delay
  timeThresholds?: {
    guest?: number;
    node?: number;
    storage?: number;
    pbs?: number;
  };
  metricTimeThresholds?: Record<string, Record<string, number>>;
  aggregation?: {
    enabled: boolean;
    timeWindow: number;
    countThreshold: number;
    similarityWindow: number;
  };
  flapping?: {
    enabled: boolean;
    threshold: number;
    window: number;
    suppressionTime: number;
    minStability: number;
  };
  ioNormalization?: {
    enabled: boolean;
    vmDiskMax: number;
    containerDiskMax: number;
    networkMax: number;
  };
  notifications?: {
    email?: {
      server: string;
      port: number;
      username: string;
      password: string;
      from: string;
      tls: boolean;
    };
    webhooks?: Array<{
      id: string;
      name: string;
      url: string;
      enabled: boolean;
    }>;
  };
  schedule?: {
    quietHours?: {
      enabled: boolean;
      start: string;
      end: string;
      timezone?: string;
      days: number[] | Record<string, boolean>;
      suppress?: {
        performance?: boolean;
        storage?: boolean;
        offline?: boolean;
      };
    };
    cooldown?: number;
    groupingWindow?: number;
    maxAlertsHour?: number;
    notifyOnResolve?: boolean;
    grouping?: {
      enabled: boolean;
      window: number;
      byNode?: boolean;
      byGuest?: boolean;
    };
    escalation?: {
      enabled: boolean;
      levels?: Array<{ after: number; notify: string }>;
    };
  };
  disableAllNodes?: boolean;
  disableAllGuests?: boolean;
  disableAllStorage?: boolean;
  disableAllPBS?: boolean;
  disableAllPMG?: boolean;
  disableAllHosts?: boolean;
  disableAllDockerHosts?: boolean;
  disableAllDockerContainers?: boolean;
  disableAllDockerServices?: boolean;
  disableAllNodesOffline?: boolean;
  disableAllGuestsOffline?: boolean;
  disableAllPBSOffline?: boolean;
  disableAllPMGOffline?: boolean;
  disableAllHostsOffline?: boolean;
  disableAllDockerHostsOffline?: boolean;
}

// Priority levels:
// 0: Global defaults
// 1-99: Reserved for system rules
// 100+: Custom user rules
// 1000+: Guest-specific overrides
