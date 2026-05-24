import { describe, expect, it } from 'vitest';

import {
  buildCephPoolThresholdStorage,
  buildThresholdStorageResources,
  cephPoolStorageId,
} from '../cephThresholdStorage';
import type { CephCluster, Storage } from '@/types/api';

describe('ceph threshold storage helpers', () => {
  it('uses the same stable Ceph pool storage id shape as the backend', () => {
    expect(cephPoolStorageId('inst1', { id: 2, name: 'data_replication' })).toBe(
      'inst1-ceph-pool-data_replication',
    );
    expect(cephPoolStorageId('Main Cluster', { id: 3, name: 'pool/slash' })).toBe(
      'Main-Cluster-ceph-pool-pool-slash',
    );
  });

  it('projects Ceph pools into storage threshold resources', () => {
    const clusters: CephCluster[] = [
      {
        id: 'cluster-1',
        instance: 'inst1',
        name: 'Ceph',
        health: 'HEALTH_OK',
        totalBytes: 1000,
        usedBytes: 910,
        availableBytes: 90,
        usagePercent: 91,
        numMons: 3,
        numMgrs: 1,
        numOsds: 3,
        numOsdsUp: 3,
        numOsdsIn: 3,
        numPGs: 64,
        lastUpdated: 1,
        pools: [
          {
            id: 2,
            name: 'data_replication',
            storedBytes: 910,
            availableBytes: 90,
            objects: 42,
            percentUsed: 91,
          },
        ],
      },
    ];

    expect(buildCephPoolThresholdStorage(clusters)).toMatchObject([
      {
        id: 'inst1-ceph-pool-data_replication',
        name: 'data_replication',
        node: 'cluster',
        instance: 'inst1',
        type: 'ceph-pool',
        status: 'available',
        usage: 91,
        shared: true,
      },
    ]);
  });

  it('deduplicates Ceph pool resources against existing storage resources', () => {
    const storage: Storage[] = [
      {
        id: 'inst1-ceph-pool-data_replication',
        name: 'data_replication',
        node: 'cluster',
        instance: 'inst1',
        type: 'rbd',
        status: 'available',
        total: 1000,
        used: 910,
        free: 90,
        usage: 91,
        content: 'images',
        shared: true,
        enabled: true,
        active: true,
      },
    ];

    const clusters: CephCluster[] = [
      {
        id: 'cluster-1',
        instance: 'inst1',
        name: 'Ceph',
        health: 'HEALTH_OK',
        totalBytes: 1000,
        usedBytes: 910,
        availableBytes: 90,
        usagePercent: 91,
        numMons: 3,
        numMgrs: 1,
        numOsds: 3,
        numOsdsUp: 3,
        numOsdsIn: 3,
        numPGs: 64,
        lastUpdated: 1,
        pools: [
          {
            id: 2,
            name: 'data_replication',
            storedBytes: 1,
            availableBytes: 1,
            objects: 1,
            percentUsed: 50,
          },
        ],
      },
    ];

    expect(buildThresholdStorageResources(storage, clusters)).toHaveLength(1);
  });
});
