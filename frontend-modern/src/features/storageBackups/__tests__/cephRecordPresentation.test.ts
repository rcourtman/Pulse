import { describe, expect, it } from 'vitest';
import type { CephCluster } from '@/types/api';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  collectCephClusterNodes,
  getCephClusterKeyFromStorageRecord,
  getCephPoolsText,
  getCephSummaryText,
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
});
