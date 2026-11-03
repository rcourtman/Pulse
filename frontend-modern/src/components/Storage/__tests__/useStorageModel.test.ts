import { renderHook } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import { useStorageModel } from '@/components/Storage/useStorageModel';
import type { StorageRecord } from '@/features/storageBackups/models';

const record = (overrides: Partial<StorageRecord>): StorageRecord => ({
  id: 'storage-1',
  name: 'storage',
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
  observedAt: 0,
  details: { type: 'rbd' },
  refs: {},
  ...overrides,
});

const model = (records: StorageRecord[], search = '') =>
  renderHook(() =>
    useStorageModel({
      records: () => records,
      search: () => search,
      sourceFilter: () => 'all',
      healthFilter: () => 'all',
      selectedNodeId: () => 'all',
      nodeOptions: () => [],
      sortKey: () => 'name',
      sortDirection: () => 'asc',
      groupBy: () => 'none',
    }),
  ).result;

describe('useStorageModel', () => {
  // Ceph telemetry adds a cluster-internal pool row for each pool PVE also
  // mounts as storage. Showing both lists the same Ceph pool twice.
  const cephPool = record({
    id: 'pool-1',
    name: 'cephfs-data',
    source: {
      platform: 'ceph-telemetry',
      family: 'virtualization',
      origin: 'resource',
      adapterId: 'resource-storage',
    },
    details: { type: 'ceph', node: 'cluster' },
  });
  const cephMount = record({
    id: 'mount-1',
    name: 'cephfs-data',
    details: { type: 'cephfs', node: 'shared' },
  });
  const local = record({
    id: 'local-1',
    name: 'local-zfs',
    capabilities: ['capacity'],
    details: { type: 'zfspool', node: 'pve1' },
  });

  it('lists one row per Ceph pool, with filters and source options built from that list', () => {
    const result = model([cephPool, cephMount, local]);

    expect(result.filteredRecords().map((row) => row.id)).toEqual(['mount-1', 'local-1']);
    expect(result.sortedRecords().map((row) => row.id)).toEqual(['mount-1', 'local-1']);
    expect(result.groupedRecords().flatMap((group) => group.items.map((row) => row.id))).toEqual([
      'mount-1',
      'local-1',
    ]);
    expect(result.sourceOptions()).not.toContain('ceph-telemetry');
    expect(model([cephPool, cephMount, local], 'cephfs-data').filteredRecords()).toHaveLength(1);
  });
});
