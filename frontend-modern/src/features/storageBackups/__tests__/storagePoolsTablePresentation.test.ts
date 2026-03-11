import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { StorageAlertRowState } from '@/features/storageBackups/storageAlertState';
import {
  buildStoragePoolsTableGroups,
  buildStoragePoolsTableRowModel,
} from '@/features/storageBackups/storagePoolsTablePresentation';
import type { StorageGroupedRecords } from '@/components/Storage/useStorageModel';

const baseRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-1',
    name: 'tank',
    source: {
      platform: 'truenas',
      type: 'storage',
      label: 'TrueNAS',
    },
    category: 'pool',
    health: 'healthy',
    statusLabel: 'Healthy',
    refs: {},
    capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60 },
    ...overrides,
  }) as StorageRecord;

const baseAlertState = (overrides: Partial<StorageAlertRowState> = {}): StorageAlertRowState => ({
  hasAlert: false,
  alertCount: 0,
  severity: null,
  hasUnacknowledgedAlert: false,
  unacknowledgedCount: 0,
  acknowledgedCount: 0,
  hasAcknowledgedOnlyAlert: false,
  ...overrides,
});

describe('storage pools table presentation', () => {
  it('builds group models with canonical expansion state', () => {
    const groups = buildStoragePoolsTableGroups(
      [
        {
          key: 'pve1',
          items: [baseRecord()],
          stats: {
            totalBytes: 100,
            usedBytes: 40,
            usagePercent: 40,
            byHealth: { healthy: 1, warning: 0, critical: 0, offline: 0, unknown: 0 },
          },
        } as StorageGroupedRecords,
      ],
      'host',
      new Set(['pve1']),
    );

    expect(groups).toEqual([
      expect.objectContaining({
        key: 'pve1',
        expanded: true,
        showHeader: true,
      }),
    ]);
  });

  it('builds row alert presentation from canonical storage inputs', () => {
    const record = baseRecord({
      id: 'storage-2',
      details: { node: 'tower' } as StorageRecord['details'],
    });

    const row = buildStoragePoolsTableRowModel(record, {
      expandedPoolId: 'storage-2',
      highlightedRecordId: null,
      nodeOnlineByLabel: new Map([['tower', false]]),
      getRecordAlertState: () =>
        baseAlertState({
          hasAlert: true,
          alertCount: 1,
          severity: 'critical',
          hasUnacknowledgedAlert: true,
          unacknowledgedCount: 1,
        }),
    });

    expect(row.expanded).toBe(true);
    expect(row.parentNodeOnline).toBe(false);
    expect(row.alertDataAttrs).toEqual({
      'data-row-id': 'storage-2',
      'data-alert-state': 'none',
      'data-alert-severity': 'critical',
      'data-resource-highlighted': 'false',
    });
    expect(row.rowClass).toContain('bg-surface-alt');
  });
});
