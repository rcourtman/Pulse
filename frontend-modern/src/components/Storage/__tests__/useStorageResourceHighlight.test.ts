import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import { useStorageResourceHighlight } from '@/components/Storage/useStorageResourceHighlight';

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-ceph',
    name: 'ceph-store',
    category: 'pool',
    health: 'healthy',
    location: { label: 'cluster-main', scope: 'cluster' },
    capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
    capabilities: ['capacity'],
    source: {
      platform: 'proxmox-pve',
      family: 'virtualization',
      origin: 'resource',
      adapterId: 'resource-storage',
    },
    observedAt: Date.now(),
    refs: { platformEntityId: '' },
    ...overrides,
  }) as StorageRecord;

describe('useStorageResourceHighlight', () => {
  it('highlights matching deep-linked resources and expands ceph pools', () => {
    const [locationSearch] = createSignal('?resource=storage-ceph');
    const [records] = createSignal<StorageRecord[]>([makeRecord()]);
    const expandedIds: Array<string | null> = [];

    const { result } = renderHook(() =>
      useStorageResourceHighlight({
        locationSearch,
        records,
        isStorageRecordCeph: () => true,
        setExpandedPoolId: (value) => {
          if (typeof value === 'function') {
            expandedIds.push(value(expandedIds.at(-1) ?? null));
            return;
          }
          expandedIds.push(value);
        },
      }),
    );

    expect(result()).toBe('storage-ceph');
    expect(expandedIds).toEqual(['storage-ceph']);
  });

  it('does nothing when no resource match exists', () => {
    const [locationSearch] = createSignal('?resource=missing');
    const [records] = createSignal<StorageRecord[]>([makeRecord()]);
    const expandedIds: Array<string | null> = [];

    const { result } = renderHook(() =>
      useStorageResourceHighlight({
        locationSearch,
        records,
        isStorageRecordCeph: () => true,
        setExpandedPoolId: (value) => {
          if (typeof value === 'function') {
            expandedIds.push(value(expandedIds.at(-1) ?? null));
            return;
          }
          expandedIds.push(value);
        },
      }),
    );

    expect(result()).toBeNull();
    expect(expandedIds).toEqual([]);
  });
});
