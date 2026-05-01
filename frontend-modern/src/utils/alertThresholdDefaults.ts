import type { BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';

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
