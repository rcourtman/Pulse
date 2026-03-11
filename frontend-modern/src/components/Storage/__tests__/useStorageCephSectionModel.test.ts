import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import { useStorageCephSectionModel } from '@/components/Storage/useStorageCephSectionModel';

describe('useStorageCephSectionModel', () => {
  it('derives canonical Ceph summary visibility', () => {
    const [view] = createSignal<'pools' | 'disks'>('pools');
    const [summary] = createSignal({
      totalBytes: 100,
      usedBytes: 50,
      usagePercent: 50,
      clusters: [{ key: 'ceph-1', name: 'ceph-1', pools: 1, totalBytes: 100, usedBytes: 50, usagePercent: 50, health: 'healthy' }],
    });
    const [filteredRecords] = createSignal<StorageRecord[]>([
      {
        id: 'storage-ceph',
        name: 'ceph-store',
        category: 'pool',
        health: 'healthy',
        source: { platform: 'generic', family: 'onprem', origin: 'resource', adapterId: 'test' },
        location: { label: 'ceph-1', scope: 'cluster' },
        capacity: { totalBytes: 100, usedBytes: 50, freeBytes: 50, usagePercent: 50 },
        capabilities: [],
        observedAt: 0,
      } as StorageRecord,
    ]);

    const { result } = renderHook(() =>
      useStorageCephSectionModel({
        view,
        summary,
        filteredRecords,
        isCephRecord: () => true,
      }),
    );

    expect(result.showSummary()).toBe(true);
  });
});
