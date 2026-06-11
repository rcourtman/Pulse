import { describe, expect, it } from 'vitest';
import type { CephCluster } from '@/types/api';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  collectCephClusterNodes,
  consolidateCephClusterPoolRecords,
  getCephClusterKeyFromStorageRecord,
  getCephPoolsText,
  getCephSummaryText,
  isCephClusterPoolStorageRecord,
  isCephStorageRecord,
} from '@/features/storageBackups/cephRecordPresentation';

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'ceph-pool',
  category: 'pool',
  health: 'healthy',
  location: { label: 'pve1', scope: 'node' },
  capacity: { totalBytes: 1_000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'replication'],
  source: {
    platform: 'proxmox-pve',
    family: 'virtualization',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  details: { type: 'rbd', parentId: 'cluster-a' },
  refs: { platformEntityId: 'cluster-a' },
  ...overrides,
});

const makeCluster = (overrides: Partial<CephCluster> = {}): CephCluster =>
  ({
    id: 'ceph-1',
    instance: 'cluster-a',
    name: 'cluster-a Ceph',
    health: 'HEALTH_OK',
    healthMessage: '',
    totalBytes: 1_000,
    usedBytes: 400,
    availableBytes: 600,
    usagePercent: 40,
    numMons: 3,
    numMgrs: 2,
    numOsds: 6,
    numOsdsUp: 6,
    numOsdsIn: 6,
    numPGs: 256,
    pools: [],
    services: [],
    lastUpdated: Date.now(),
    ...overrides,
  }) as CephCluster;

describe('cephRecordPresentation', () => {
  it('derives canonical ceph record identity and cluster keys', () => {
    const record = makeRecord();
    expect(isCephStorageRecord(record)).toBe(true);
    expect(getCephClusterKeyFromStorageRecord(record)).toBe('cluster-a');
    expect(collectCephClusterNodes(new Set(['pve2']), record)).toEqual(new Set(['pve2', 'pve1']));
  });

  it('formats canonical ceph summary text from clusters or record capacity', () => {
    expect(getCephSummaryText(makeRecord(), makeCluster())).toContain('OSDs 6/6');
    expect(
      getCephSummaryText(
        makeRecord({ capacity: { totalBytes: 0, usedBytes: 0, freeBytes: 0, usagePercent: 0 } }),
        null,
      ),
    ).toBe('');
    expect(getCephSummaryText(makeRecord(), null)).toContain('40%');
  });

  it('formats canonical ceph pool text from cluster pool data or record usage', () => {
    expect(
      getCephPoolsText(
        makeRecord(),
        makeCluster({
          pools: [
            { id: 1, name: 'rbd', storedBytes: 75, availableBytes: 25, objects: 1, percentUsed: 75 },
          ],
        }),
      ),
    ).toBe('rbd: 75%');
    expect(getCephPoolsText(makeRecord({ name: 'ceph-a' }), null)).toBe('ceph-a: 40%');
  });

  it('classifies cluster-internal ceph pool rows by their synthesis signature', () => {
    const poolRecord = makeRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      details: { type: 'ceph', node: 'cluster' },
    });
    expect(isCephClusterPoolStorageRecord(poolRecord)).toBe(true);
    expect(isCephClusterPoolStorageRecord(makeRecord())).toBe(false);
    expect(
      isCephClusterPoolStorageRecord(makeRecord({ details: { type: 'cephfs', node: 'cluster' } })),
    ).toBe(false);
  });

  it('collapses cluster pool rows into the PVE storage rows that mount them', () => {
    const poolRecord = makeRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'warning',
      details: { type: 'ceph', node: 'cluster', status: 'degraded' },
    });
    const mountRecord = makeRecord({
      id: 'mount-1',
      name: 'cephfs-data',
      health: 'healthy',
      details: { type: 'cephfs', node: 'shared', status: 'online' },
    });
    const unrelated = makeRecord({
      id: 'other-1',
      name: 'local-zfs',
      details: { type: 'zfspool', node: 'pve1' },
      capabilities: ['capacity'],
      source: {
        platform: 'proxmox-pve',
        family: 'virtualization',
        origin: 'resource',
        adapterId: 'resource-storage',
      },
    });

    const consolidated = consolidateCephClusterPoolRecords([poolRecord, mountRecord, unrelated]);

    expect(consolidated.map((record) => record.id)).toEqual(['mount-1', 'other-1']);
    const survivor = consolidated[0];
    expect(survivor.health).toBe('warning');
    expect(survivor.statusLabel).toBe('degraded');
    expect(survivor.details?.status).toBe('degraded');
    expect(survivor.issueSummary).toBe('Ceph reports pool cephfs-data degraded');
  });

  it('matches pool rows to mounts via the backing pool detail', () => {
    const poolRecord = makeRecord({
      id: 'pool-1',
      name: 'vm-pool',
      health: 'critical',
      issueSummary: 'Ceph cluster HEALTH_ERR',
      details: { type: 'ceph', node: 'cluster', status: 'unavailable' },
    });
    const mountRecord = makeRecord({
      id: 'mount-1',
      name: 'fast-rbd',
      health: 'healthy',
      details: { type: 'rbd', node: 'shared', pool: 'vm-pool', status: 'online' },
    });

    const consolidated = consolidateCephClusterPoolRecords([poolRecord, mountRecord]);

    expect(consolidated.map((record) => record.id)).toEqual(['mount-1']);
    expect(consolidated[0].health).toBe('critical');
    expect(consolidated[0].issueSummary).toBe('Ceph cluster HEALTH_ERR');
  });

  it('keeps unmounted pool rows and never downgrades a sicker mount', () => {
    const orphanPool = makeRecord({
      id: 'pool-1',
      name: 'standalone-pool',
      details: { type: 'ceph', node: 'cluster' },
    });
    expect(consolidateCephClusterPoolRecords([orphanPool])).toEqual([orphanPool]);

    const healthyPool = makeRecord({
      id: 'pool-2',
      name: 'cephfs-data',
      health: 'healthy',
      details: { type: 'ceph', node: 'cluster', status: 'available' },
    });
    const sickMount = makeRecord({
      id: 'mount-2',
      name: 'cephfs-data',
      health: 'critical',
      statusLabel: 'unavailable',
      details: { type: 'cephfs', node: 'shared', status: 'unavailable' },
    });
    const consolidated = consolidateCephClusterPoolRecords([healthyPool, sickMount]);
    expect(consolidated).toEqual([sickMount]);
  });
});
