import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import type { StorageGroupedRecords } from '@/components/Storage/useStorageModel';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { StorageAlertRowState } from '@/features/storageBackups/storageAlertState';
import { useStoragePoolsTableModel } from '@/components/Storage/useStoragePoolsTableModel';

const baseRecord = (): StorageRecord =>
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
    location: { label: 'tower', scope: 'host' },
    capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
    capabilities: [],
    observedAt: 0,
    details: { node: 'tower' },
  }) as StorageRecord;

const baseAlertState = (): StorageAlertRowState => ({
  hasAlert: false,
  alertCount: 0,
  severity: null,
  hasUnacknowledgedAlert: false,
  unacknowledgedCount: 0,
  acknowledgedCount: 0,
  hasAcknowledgedOnlyAlert: false,
});

describe('useStoragePoolsTableModel', () => {
  it('builds canonical group/row state and pool toggles', () => {
    const [groupedRecords] = createSignal<StorageGroupedRecords[]>([
      {
        key: 'tower',
        items: [baseRecord()],
        stats: {
          totalBytes: 100,
          usedBytes: 40,
          usagePercent: 40,
          byHealth: { healthy: 1, warning: 0, critical: 0, offline: 0, unknown: 0 },
        },
      } as StorageGroupedRecords,
    ]);
    const [groupBy] = createSignal<'host'>('host');
    const [expandedGroups] = createSignal(new Set(['tower']));
    const [expandedPoolId] = createSignal<string | null>('storage-1');
    const [highlightedRecordId] = createSignal<string | null>(null);
    const [nodeOnlineByLabel] = createSignal(new Map([['tower', true]]));
    const setExpandedPoolId = vi.fn();

    const { result } = renderHook(() =>
      useStoragePoolsTableModel({
        groupedRecords,
        groupBy,
        expandedGroups,
        expandedPoolId,
        highlightedRecordId,
        nodeOnlineByLabel,
        getRecordAlertState: () => baseAlertState(),
        setExpandedPoolId,
      }),
    );

    expect(result.groups()).toHaveLength(1);
    const row = result.buildRowModel('storage-1', baseRecord());
    expect(row.expanded).toBe(true);

    result.togglePool('storage-1');
    expect(setExpandedPoolId).toHaveBeenCalledTimes(1);
  });
});
