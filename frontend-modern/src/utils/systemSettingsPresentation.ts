export const PVE_POLLING_MIN_SECONDS = 10;
export const PVE_POLLING_MAX_SECONDS = 3600;

export const PVE_POLLING_PRESETS = [
  { label: 'Realtime (10s)', value: 10 },
  { label: 'Balanced (30s)', value: 30 },
  { label: 'Low (60s)', value: 60 },
  { label: 'Very low (5m)', value: 300 },
] as const;

export const BACKUP_INTERVAL_OPTIONS = [
  { value: 0, label: 'Default (~90s)' },
  { value: 300, label: '5 minutes' },
  { value: 900, label: '15 minutes' },
  { value: 1800, label: '30 minutes' },
  { value: 3600, label: '1 hour' },
  { value: 21600, label: '6 hours' },
  { value: 86400, label: '24 hours' },
] as const;

export const BACKUP_INTERVAL_MAX_MINUTES = 7 * 24 * 60;

export const COMMON_DISCOVERY_SUBNETS = [
  '192.168.1.0/24',
  '192.168.0.0/24',
  '10.0.0.0/24',
  '172.16.0.0/24',
  '192.168.10.0/24',
] as const;

export function getBackupIntervalSelectValue(
  backupPollingUseCustom: boolean,
  backupPollingInterval: number,
): string {
  if (backupPollingUseCustom) {
    return 'custom';
  }
  return BACKUP_INTERVAL_OPTIONS.some((option) => option.value === backupPollingInterval)
    ? String(backupPollingInterval)
    : 'custom';
}

export function getBackupIntervalSummary(
  backupPollingEnabled: boolean,
  backupPollingInterval: number,
): string {
  if (!backupPollingEnabled) {
    return 'Backup polling is disabled.';
  }

  if (backupPollingInterval <= 0) {
    return 'Pulse checks backups and snapshots at the default cadence (~every 90 seconds).';
  }
  if (backupPollingInterval % 86400 === 0) {
    const days = backupPollingInterval / 86400;
    return `Pulse checks backups every ${days === 1 ? 'day' : `${days} days`}.`;
  }
  if (backupPollingInterval % 3600 === 0) {
    const hours = backupPollingInterval / 3600;
    return `Pulse checks backups every ${hours === 1 ? 'hour' : `${hours} hours`}.`;
  }

  const minutes = Math.max(1, Math.round(backupPollingInterval / 60));
  return `Pulse checks backups every ${minutes === 1 ? 'minute' : `${minutes} minutes`}.`;
}

export function getSystemSettingsSaveErrorMessage(message?: string): string {
  return message || 'Failed to save settings';
}

export function getHideLocalLoginUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update hide local login setting';
}

export function getDockerUpdateActionsUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update Docker update actions setting';
}

export function getReduceUpsellNoiseUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update reduce upsell noise setting';
}

export function getLocalUpgradeMetricsUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update local upgrade metrics setting';
}

export function getTelemetryUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update telemetry setting';
}

export function getTemperatureMonitoringUpdateErrorMessage(message?: string): string {
  return message || 'Failed to update temperature monitoring setting';
}

export function getCheckForUpdatesErrorMessage(): string {
  return 'Failed to check for updates';
}

export function getStartUpdateErrorMessage(): string {
  return 'Failed to start update. Please try again.';
}
