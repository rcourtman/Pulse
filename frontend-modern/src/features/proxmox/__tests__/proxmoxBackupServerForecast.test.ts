import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  backupServerDatastoreAlertIds,
  getDatastoreFillForecast,
  isBackupServerDatastoreAlert,
} from '../proxmoxBackupServerForecast';

const alert = (patch: Partial<Alert> = {}): Alert =>
  ({
    id: 'storage-usage',
    type: 'usage',
    level: 'warning',
    resourceId: 'pbs-main-tank',
    resourceName: 'tank',
    node: 'main',
    instance: 'pbs-main',
    message: 'Storage projected to fill in 5 days',
    value: 82,
    threshold: 90,
    startTime: '2026-11-02T08:00:00Z',
    acknowledged: false,
    ...patch,
  }) as Alert;

describe('backupServerDatastoreAlertIds', () => {
  it('names the poller storage id and its alias from the instance id, not the hostname', () => {
    const server = {
      id: 'pbs-f611797ee465f279',
      name: 'backup-vault',
      pbs: { instanceId: 'pbs-main' },
    } as Resource;
    expect(backupServerDatastoreAlertIds(server, 'tank')).toEqual([
      'pbs-main-tank',
      'pbs-main/tank',
    ]);
    expect(backupServerDatastoreAlertIds({ ...server, pbs: {} } as Resource, 'tank')).toEqual([]);
  });
});

describe('isBackupServerDatastoreAlert', () => {
  const pbs = (instanceId: string) =>
    ({ id: instanceId, name: instanceId, pbs: { instanceId } }) as Resource;

  it('tells apart two datastores that share a dashed storage id', () => {
    // Connection "main" store "daily-tank" and connection "main-daily" store
    // "tank" are both storage "pbs-main-daily-tank".
    const forDailyTank = alert({
      resourceId: 'pbs-main-daily-tank',
      metadata: { forecastDaysToFull: 3, storagePolicyAliases: ['pbs-main/daily-tank'] },
    });
    expect(backupServerDatastoreAlertIds(pbs('pbs-main'), 'daily-tank')).toContain(
      'pbs-main-daily-tank',
    );
    expect(backupServerDatastoreAlertIds(pbs('pbs-main-daily'), 'tank')).toContain(
      'pbs-main-daily-tank',
    );
    expect(isBackupServerDatastoreAlert(forDailyTank, pbs('pbs-main'), 'daily-tank')).toBe(true);
    expect(isBackupServerDatastoreAlert(forDailyTank, pbs('pbs-main-daily'), 'tank')).toBe(false);
  });

  it('falls back to the alert instance and store name without policy aliases', () => {
    const plain = alert({
      resourceId: 'pbs-main-daily-tank',
      instance: 'pbs-main',
      resourceName: 'daily-tank',
    });
    expect(isBackupServerDatastoreAlert(plain, pbs('pbs-main'), 'daily-tank')).toBe(true);
    expect(isBackupServerDatastoreAlert(plain, pbs('pbs-main-daily'), 'tank')).toBe(false);
  });
});

describe('getDatastoreFillForecast', () => {
  it('reads the soonest confident forecast and its growth', () => {
    const forecast = getDatastoreFillForecast([
      alert({ id: 'later', metadata: { forecastDaysToFull: 6.4 } }),
      alert({
        id: 'sooner',
        level: 'critical',
        acknowledged: true,
        metadata: { forecastDaysToFull: 0.6, forecastDailyChangePct: 4.25 },
      }),
    ]);
    expect(forecast).toMatchObject({
      level: 'critical',
      label: 'Full within a day',
      shortLabel: 'Full <1d',
      detail: 'Full within a day, growing 4.3% a day',
    });
  });

  it('rounds the runway the way the Storage page does', () => {
    expect(
      getDatastoreFillForecast([alert({ metadata: { forecastDaysToFull: 4.6 } })]),
    ).toMatchObject({
      level: 'warning',
      label: 'Full in ~5 days',
      shortLabel: 'Full in ~5d',
      detail: 'Full in ~5 days',
    });
  });

  it('shows nothing without a forecast: a usage alert is not a runway', () => {
    expect(getDatastoreFillForecast([alert()])).toBeNull();
    expect(getDatastoreFillForecast([alert({ metadata: { forecastDaysToFull: -1 } })])).toBeNull();
    expect(getDatastoreFillForecast([])).toBeNull();
  });
});
