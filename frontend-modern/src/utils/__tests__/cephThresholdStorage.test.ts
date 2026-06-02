import { describe, expect, it } from 'vitest';

import {
  buildCephPoolThresholdStorage,
  buildThresholdStorageResources,
  cephPoolStorageAliasIds,
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

  // #1341: in a cluster the API instance name (cluster name) differs from the
  // host-agent's node hostname, so the pool's source identities share no
  // prefix. The alias IDs must bridge them so a per-pool override resolves.
  it('builds cross-source alias ids from instance aliases', () => {
    const aliases = cephPoolStorageAliasIds(
      'prodcluster',
      { id: 2, name: 'data_replication' },
      ['agent:pve5'],
    );
    expect(aliases).toContain('agent:pve5-ceph-pool-data_replication');
    expect(aliases).toContain('pve5-ceph-pool-data_replication');
    expect(aliases).toContain('agent:prodcluster-ceph-pool-data_replication');
    // The primary id is never listed as its own alias.
    expect(aliases).not.toContain('prodcluster-ceph-pool-data_replication');
  });

  it('attaches alias ids to ceph pool threshold resources from instanceAliases', () => {
    const clusters: CephCluster[] = [
      {
        id: 'cluster-1',
        instance: 'prodcluster',
        instanceAliases: ['agent:pve5'],
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
          { id: 2, name: 'data_replication', storedBytes: 910, availableBytes: 90, objects: 1, percentUsed: 91 },
        ],
      },
    ];
    const [row] = buildCephPoolThresholdStorage(clusters);
    expect(row.id).toBe('prodcluster-ceph-pool-data_replication');
    expect(row.aliasIds).toContain('agent:pve5-ceph-pool-data_replication');
  });
});
