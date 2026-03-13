import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { CephSummaryStats } from '@/features/storageBackups/cephSummaryPresentation';
import type { StorageRecord } from '@/features/storageBackups/models';
import { useStorageCephSectionModel } from '@/components/Storage/useStorageCephSectionModel';

describe('useStorageCephSectionModel', () => {
  it('derives canonical Ceph summary visibility', () => {
    const [view] = createSignal<'pools' | 'disks'>('pools');
    const [summary] = createSignal<CephSummaryStats | null>({
      totalBytes: 100,
      usedBytes: 50,
      availableBytes: 50,
      usagePercent: 50,
      clusters: [
        {
          id: 'ceph-1',
          instance: 'cluster-a',
          name: 'ceph-1',
          totalBytes: 100,
          usedBytes: 50,
          availableBytes: 50,
          usagePercent: 50,
          health: 'HEALTH_OK',
          healthMessage: '',
          numMons: 1,
          numMgrs: 1,
          numOsds: 3,
          numOsdsUp: 3,
          numOsdsIn: 3,
          numPGs: 64,
          pools: [],
          services: [],
          lastUpdated: 0,
        },
      ],
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
