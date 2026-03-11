import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  asStorageAlertRecord,
  EMPTY_STORAGE_ALERT_STATE,
  getStorageRecordAlertResourceIds,
  mergeStorageAlertRowState,
} from '@/features/storageBackups/storageAlertState';

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01', scope: 'host' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: ['capacity'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

describe('storageAlertState', () => {
  it('normalizes alert collections into canonical alert records', () => {
    expect(
      asStorageAlertRecord([
        { id: 'a', severity: 'critical' },
        { id: 'b', severity: 'warning' },
      ]),
    ).toEqual({
      a: { id: 'a', severity: 'critical' },
      b: { id: 'b', severity: 'warning' },
    });
    expect(asStorageAlertRecord(null)).toEqual({});
  });

  it('merges alert row state without losing severity or acknowledgement semantics', () => {
    expect(
      mergeStorageAlertRowState(
        {
          ...EMPTY_STORAGE_ALERT_STATE,
          hasAlert: true,
          alertCount: 1,
          severity: 'warning',
          acknowledgedCount: 1,
          hasAcknowledgedOnlyAlert: true,
        },
        {
          ...EMPTY_STORAGE_ALERT_STATE,
          hasAlert: true,
          alertCount: 2,
          severity: 'critical',
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 2,
        },
      ),
    ).toEqual({
      hasAlert: true,
      alertCount: 3,
      severity: 'critical',
      hasUnacknowledgedAlert: true,
      unacknowledgedCount: 2,
      acknowledgedCount: 1,
      hasAcknowledgedOnlyAlert: false,
    });
  });

  it('derives storage alert candidate ids canonically', () => {
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          refs: { resourceId: 'legacy-storage-id', platformEntityId: 'cluster-a' },
          details: { node: 'pve1' },
        }),
      ),
    ).toEqual(['storage-1', 'legacy-storage-id', 'cluster-a-pve1-tank']);
  });
});
