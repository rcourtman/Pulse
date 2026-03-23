import { describe, expect, it } from 'vitest';
import {
  BACKUP_INTERVAL_MAX_MINUTES,
  BACKUP_INTERVAL_OPTIONS,
  COMMON_DISCOVERY_SUBNETS,
  getBackupIntervalSelectValue,
  getBackupIntervalSummary,
  getCheckForUpdatesErrorMessage,
  getDockerUpdateActionsUpdateErrorMessage,
  getHideLocalLoginUpdateErrorMessage,
  getLocalUpgradeMetricsUpdateErrorMessage,
  getReduceUpsellNoiseUpdateErrorMessage,
  getStartUpdateErrorMessage,
  getSystemSettingsSaveErrorMessage,
  getTelemetryUpdateErrorMessage,
  getTemperatureMonitoringUpdateErrorMessage,
  PVE_POLLING_MAX_SECONDS,
  PVE_POLLING_MIN_SECONDS,
  PVE_POLLING_PRESETS,
} from '@/utils/systemSettingsPresentation';

describe('systemSettingsPresentation', () => {
  it('exports canonical PVE polling bounds and presets', () => {
    expect(PVE_POLLING_MIN_SECONDS).toBe(10);
    expect(PVE_POLLING_MAX_SECONDS).toBe(3600);
    expect(PVE_POLLING_PRESETS).toEqual([
      { label: 'Realtime (10s)', value: 10 },
      { label: 'Balanced (30s)', value: 30 },
      { label: 'Low (60s)', value: 60 },
      { label: 'Very low (5m)', value: 300 },
    ]);
  });

  it('exports canonical backup interval options and max custom minutes', () => {
    expect(BACKUP_INTERVAL_OPTIONS).toEqual([
      { value: 0, label: 'Default (~90s)' },
      { value: 300, label: '5 minutes' },
      { value: 900, label: '15 minutes' },
      { value: 1800, label: '30 minutes' },
      { value: 3600, label: '1 hour' },
      { value: 21600, label: '6 hours' },
      { value: 86400, label: '24 hours' },
    ]);
    expect(BACKUP_INTERVAL_MAX_MINUTES).toBe(7 * 24 * 60);
  });

  it('exports canonical common discovery subnet suggestions', () => {
    expect(COMMON_DISCOVERY_SUBNETS).toEqual([
      '192.168.1.0/24',
      '192.168.0.0/24',
      '10.0.0.0/24',
      '172.16.0.0/24',
      '192.168.10.0/24',
    ]);
  });

  it('returns canonical backup interval select values', () => {
    expect(getBackupIntervalSelectValue(false, 0)).toBe('0');
    expect(getBackupIntervalSelectValue(false, 300)).toBe('300');
    expect(getBackupIntervalSelectValue(false, 123)).toBe('custom');
    expect(getBackupIntervalSelectValue(true, 300)).toBe('custom');
  });

  it('returns canonical backup interval summaries', () => {
    expect(getBackupIntervalSummary(false, 0)).toBe('Backup polling is disabled.');
    expect(getBackupIntervalSummary(true, 0)).toBe(
      'Pulse checks backups and snapshots at the default cadence (~every 90 seconds).',
    );
    expect(getBackupIntervalSummary(true, 3600)).toBe('Pulse checks backups every hour.');
    expect(getBackupIntervalSummary(true, 7200)).toBe('Pulse checks backups every 2 hours.');
    expect(getBackupIntervalSummary(true, 86400)).toBe('Pulse checks backups every day.');
    expect(getBackupIntervalSummary(true, 172800)).toBe('Pulse checks backups every 2 days.');
    expect(getBackupIntervalSummary(true, 900)).toBe('Pulse checks backups every 15 minutes.');
  });

  it('returns canonical system settings operational failure copy', () => {
    expect(getSystemSettingsSaveErrorMessage()).toBe('Unable to save settings.');
    expect(getSystemSettingsSaveErrorMessage('forbidden')).toBe('forbidden');
    expect(getHideLocalLoginUpdateErrorMessage()).toBe(
      'Unable to update local login visibility.',
    );
    expect(getDockerUpdateActionsUpdateErrorMessage()).toBe(
      'Unable to update container update actions.',
    );
    expect(getReduceUpsellNoiseUpdateErrorMessage()).toBe(
      'Unable to update upgrade guidance preferences.',
    );
    expect(getLocalUpgradeMetricsUpdateErrorMessage()).toBe(
      'Unable to update local upgrade metrics.',
    );
    expect(getTelemetryUpdateErrorMessage()).toBe('Unable to update anonymous telemetry.');
    expect(getTemperatureMonitoringUpdateErrorMessage()).toBe(
      'Unable to update temperature monitoring.',
    );
    expect(getCheckForUpdatesErrorMessage()).toBe('Unable to check for updates.');
    expect(getStartUpdateErrorMessage()).toBe('Unable to start the update. Please try again.');
  });
});
