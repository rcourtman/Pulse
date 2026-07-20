import { describe, expect, it } from 'vitest';
import type { CephCluster } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildExplicitCephClusters,
  deriveCephClustersFromStorageRecords,
  resolveCephClusterForStorageRecord,
  summarizeCephClusters,
} from '@/features/storageBackups/cephSummaryPresentation';

// ---------------------------------------------------------------------------
// Fixture builders — mirror the shapes used by the sibling
// cephSummaryPresentation.test.ts so casts and import paths match.
// ---------------------------------------------------------------------------

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-1',
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
    observedAt: 1_700_000_000_000,
    details: {
      type: 'rbd',
      node: 'pve1',
      parentName: 'pve1',
    },
    refs: { platformEntityId: 'cluster-main' },
    ...overrides,
  }) as StorageRecord;

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'ceph-1',
    type: 'ceph',
    name: 'ceph-main',
    displayName: 'ceph-main',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    disk: { current: 40, total: 300, used: 120, free: 180 },
    platformData: {
      proxmox: { instance: 'cluster-main' },
      ceph: {
        fsid: 'fsid-1',
        healthStatus: 'HEALTH_OK',
        healthMessage: 'all good',
        numMons: 3,
        numMgrs: 2,
        numOsds: 6,
        numOsdsUp: 6,
        numOsdsIn: 6,
        numPGs: 128,
        pools: [
          {
            id: 11,
            name: 'rbd',
            storedBytes: 50,
            availableBytes: 50,
            objects: 10,
            percentUsed: 50,
          },
        ],
        services: [{ type: 'mon', running: 3, total: 3 }],
      },
    },
    ...overrides,
  }) as Resource;

const makeCluster = (overrides: Partial<CephCluster> = {}): CephCluster =>
  ({
    id: 'c-1',
    instance: 'cluster-main',
    name: 'cluster-main Ceph',
    health: 'HEALTH_OK',
    healthMessage: '',
    totalBytes: 300,
    usedBytes: 120,
    availableBytes: 180,
    usagePercent: 40,
    numMons: 3,
    numMgrs: 2,
    numOsds: 6,
    numOsdsUp: 6,
    numOsdsIn: 6,
    numPGs: 128,
    pools: [],
    services: [],
    lastUpdated: 1_700_000_000_000,
    ...overrides,
  }) as CephCluster;

// ===========================================================================
// buildExplicitCephClusters
// ===========================================================================

describe('buildExplicitCephClusters branch coverage', () => {
  it('returns an empty array when resources is empty', () => {
    expect(buildExplicitCephClusters([])).toEqual([]);
  });

  it('maps a fully-populated resource including pools and services', () => {
    const [cluster] = buildExplicitCephClusters([makeResource()]);
    expect(cluster).toMatchObject({
      id: 'ceph-1',
      instance: 'cluster-main',
      name: 'ceph-main',
      fsid: 'fsid-1',
      health: 'HEALTH_OK',
      healthMessage: 'all good',
      totalBytes: 300,
      usedBytes: 120,
      availableBytes: 180,
      usagePercent: 40,
      numMons: 3,
      numMgrs: 2,
      numOsds: 6,
      numOsdsUp: 6,
      numOsdsIn: 6,
      numPGs: 128,
      lastUpdated: 1_700_000_000_000,
    });
    // pools are mapped to the cluster-internal shape (id is always 0 — see
    // suspected bug note in GLM_REPORT.md).
    expect(cluster.pools).toEqual([
      {
        id: 0,
        name: 'rbd',
        storedBytes: 50,
        availableBytes: 50,
        objects: 10,
        percentUsed: 50,
      },
    ]);
    expect(cluster.services).toEqual([{ type: 'mon', running: 3, total: 3 }]);
  });

  it('defaults everything when platformData is missing entirely', () => {
    const [cluster] = buildExplicitCephClusters([makeResource({ platformData: undefined })]);
    // platformData || {} -> cephMeta {} -> all ceph fields default.
    expect(cluster.health).toBe('HEALTH_UNKNOWN');
    expect(cluster.healthMessage).toBe('');
    expect(cluster.fsid).toBeUndefined();
    expect(cluster.numMons).toBe(0);
    expect(cluster.numOsds).toBe(0);
    expect(cluster.numPGs).toBe(0);
    expect(cluster.pools).toBeUndefined();
    expect(cluster.services).toBeUndefined();
    // proxmox?.instance is absent -> falls back to platformId.
    expect(cluster.instance).toBe('cluster-main');
  });

  it('defaults ceph counts to 0 and health to HEALTH_UNKNOWN when ceph meta exists but is sparse', () => {
    const [cluster] = buildExplicitCephClusters([
      makeResource({
        platformData: { proxmox: { instance: 'cluster-main' }, ceph: {} },
      }),
    ]);
    expect(cluster.health).toBe('HEALTH_UNKNOWN');
    expect(cluster.healthMessage).toBe('');
    expect(cluster.numMons).toBe(0);
    expect(cluster.numMgrs).toBe(0);
    expect(cluster.numOsds).toBe(0);
    expect(cluster.numOsdsUp).toBe(0);
    expect(cluster.numOsdsIn).toBe(0);
    expect(cluster.numPGs).toBe(0);
  });

  it('falls back to resource.platformId when proxmox.instance is absent', () => {
    const [cluster] = buildExplicitCephClusters([
      makeResource({
        platformId: 'fallback-pid',
        platformData: { proxmox: {}, ceph: {} },
      }),
    ]);
    expect(cluster.instance).toBe('fallback-pid');
  });

  it('falls back to empty string when neither proxmox.instance nor platformId exist', () => {
    const [cluster] = buildExplicitCephClusters([
      makeResource({
        platformId: '',
        platformData: { proxmox: {}, ceph: {} },
      }),
    ]);
    expect(cluster.instance).toBe('');
  });

  it('defaults all disk metrics to 0 when resource.disk is absent', () => {
    const [cluster] = buildExplicitCephClusters([makeResource({ disk: undefined })]);
    expect(cluster.totalBytes).toBe(0);
    expect(cluster.usedBytes).toBe(0);
    expect(cluster.availableBytes).toBe(0);
    expect(cluster.usagePercent).toBe(0);
  });

  it('maps pools with default values when individual pool fields are missing', () => {
    const [cluster] = buildExplicitCephClusters([
      makeResource({
        platformData: {
          proxmox: { instance: 'cluster-main' },
          ceph: {
            pools: [
              {}, // all fields absent
              { name: 'only-name' },
            ],
          },
        },
      }),
    ]);
    expect(cluster.pools).toEqual([
      { id: 0, name: '', storedBytes: 0, availableBytes: 0, objects: 0, percentUsed: 0 },
      { id: 0, name: 'only-name', storedBytes: 0, availableBytes: 0, objects: 0, percentUsed: 0 },
    ]);
  });

  it('maps services with default values when individual service fields are missing', () => {
    const [cluster] = buildExplicitCephClusters([
      makeResource({
        platformData: {
          proxmox: { instance: 'cluster-main' },
          ceph: {
            services: [{}, { type: 'mgr' }],
          },
        },
      }),
    ]);
    expect(cluster.services).toEqual([
      { type: '', running: 0, total: 0 },
      { type: 'mgr', running: 0, total: 0 },
    ]);
  });

  it('uses lastSeen verbatim when it is a positive finite number', () => {
    const [cluster] = buildExplicitCephClusters([makeResource({ lastSeen: 9_999_999 })]);
    expect(cluster.lastUpdated).toBe(9_999_999);
  });

  it('falls back to Date.now() for lastUpdated when lastSeen is 0 (falsy)', () => {
    const before = Date.now();
    const [cluster] = buildExplicitCephClusters([makeResource({ lastSeen: 0 })]);
    const after = Date.now();
    expect(cluster.lastUpdated).toBeGreaterThanOrEqual(before);
    expect(cluster.lastUpdated).toBeLessThanOrEqual(after);
  });
});

// ===========================================================================
// deriveCephClustersFromStorageRecords
// ===========================================================================

describe('deriveCephClustersFromStorageRecords branch coverage', () => {
  it('returns an empty array when records is empty', () => {
    expect(deriveCephClustersFromStorageRecords([])).toEqual([]);
  });

  it('returns an empty array when no records are ceph storage records', () => {
    // type 'dir' is not ceph, capabilities lack 'replication'.
    const nonCeph = makeRecord({
      id: 'nc-1',
      details: { type: 'dir', node: 'pve1', parentName: 'pve1' },
      capabilities: ['capacity'],
      source: {
        platform: 'truenas-scale',
        family: 'virtualization',
        origin: 'resource',
        adapterId: 'a',
      },
    });
    expect(deriveCephClustersFromStorageRecords([nonCeph])).toEqual([]);
  });

  it('skips non-ceph records and processes only ceph records in a mixed set', () => {
    const nonCeph = makeRecord({
      id: 'nc-1',
      name: 'local',
      details: { type: 'dir', node: 'pve1', parentName: 'pve1' },
      capabilities: ['capacity'],
      source: {
        platform: 'truenas-scale',
        family: 'virtualization',
        origin: 'resource',
        adapterId: 'a',
      },
    });
    const ceph = makeRecord({
      id: 'ceph-1',
      details: { type: 'rbd', node: 'pve1', parentName: 'pve1' },
    });
    const clusters = deriveCephClustersFromStorageRecords([nonCeph, ceph]);
    expect(clusters).toHaveLength(1);
    expect(clusters[0].instance).toBe('cluster-main');
  });

  it('produces separate clusters when cluster keys differ', () => {
    const clusterA = makeRecord({
      id: 'r-a',
      details: { type: 'rbd', node: 'pve1', parentName: 'pve1' },
      refs: { platformEntityId: 'cluster-a' },
    });
    const clusterB = makeRecord({
      id: 'r-b',
      details: { type: 'rbd', node: 'pve2', parentName: 'pve2' },
      refs: { platformEntityId: 'cluster-b' },
    });
    const clusters = deriveCephClustersFromStorageRecords([clusterA, clusterB]);
    expect(clusters).toHaveLength(2);
    const instances = clusters.map((c) => c.instance).sort();
    expect(instances).toEqual(['cluster-a', 'cluster-b']);
  });

  it('clamps negative totalBytes and usedBytes to 0', () => {
    const [cluster] = deriveCephClustersFromStorageRecords([
      makeRecord({
        id: 'neg-1',
        capacity: {
          totalBytes: -100,
          usedBytes: -50,
          freeBytes: 0,
          usagePercent: null,
        },
      }),
    ]);
    expect(cluster.totalBytes).toBe(0);
    expect(cluster.usedBytes).toBe(0);
    expect(cluster.usagePercent).toBe(0);
  });

  it('computes freeBytes from total - used when freeBytes is null (the ?? arm)', () => {
    const [cluster] = deriveCephClustersFromStorageRecords([
      makeRecord({
        id: 'null-free',
        capacity: { totalBytes: 200, usedBytes: 80, freeBytes: null, usagePercent: null },
      }),
    ]);
    expect(cluster.availableBytes).toBe(120);
  });

  it('uses freeBytes verbatim when it is 0 (not nullish — the ?? short-circuit)', () => {
    // freeBytes 0 is NOT nullish, so ?? does NOT kick in. availableBytes is 0,
    // NOT total - used (which would be 60). This distinguishes ?? from ||.
    const [cluster] = deriveCephClustersFromStorageRecords([
      makeRecord({
        id: 'zero-free',
        capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 0, usagePercent: null },
      }),
    ]);
    expect(cluster.availableBytes).toBe(0);
  });

  it('clamps negative freeBytes to 0', () => {
    const [cluster] = deriveCephClustersFromStorageRecords([
      makeRecord({
        id: 'neg-free',
        capacity: { totalBytes: 100, usedBytes: 40, freeBytes: -30, usagePercent: null },
      }),
    ]);
    expect(cluster.availableBytes).toBe(0);
  });

  it('computes usagePercent from used/total when total is positive', () => {
    const [cluster] = deriveCephClustersFromStorageRecords([
      makeRecord({
        id: 'pct-1',
        capacity: { totalBytes: 400, usedBytes: 100, freeBytes: 300, usagePercent: null },
      }),
    ]);
    expect(cluster.usagePercent).toBe(25);
  });

  it('yields usagePercent 0 when total bytes for a cluster is 0', () => {
    const [cluster] = deriveCephClustersFromStorageRecords([
      makeRecord({
        id: 'zero-total',
        capacity: { totalBytes: 0, usedBytes: 0, freeBytes: 0, usagePercent: null },
      }),
    ]);
    expect(cluster.usagePercent).toBe(0);
  });

  it('sets numMons to 1 and numMgrs to 1 when the cluster has a single node', () => {
    const [cluster] = deriveCephClustersFromStorageRecords([makeRecord()]);
    expect(cluster.numMons).toBe(1);
    expect(cluster.numMgrs).toBe(1);
  });

  it('caps numMons at 3 and sets numMgrs to 2 when node count exceeds 3', () => {
    // Five records with distinct node labels but the same cluster key -> 5
    // unique nodes -> Math.min(3, Math.max(1, 5)) = 3 mons; numMgrs = 2.
    const records: StorageRecord[] = ['n1', 'n2', 'n3', 'n4', 'n5'].map((node) =>
      makeRecord({
        id: `r-${node}`,
        details: { type: 'rbd', node, parentName: node },
        capacity: { totalBytes: 100, usedBytes: 10, freeBytes: 90, usagePercent: 10 },
      }),
    );
    const [cluster] = deriveCephClustersFromStorageRecords(records);
    expect(cluster.numMons).toBe(3);
    expect(cluster.numMgrs).toBe(2);
  });

  it('sets numMons to 2 for exactly two distinct nodes', () => {
    const records: StorageRecord[] = ['n1', 'n2'].map((node) =>
      makeRecord({
        id: `r-${node}`,
        details: { type: 'rbd', node, parentName: node },
      }),
    );
    const [cluster] = deriveCephClustersFromStorageRecords(records);
    expect(cluster.numMons).toBe(2);
    expect(cluster.numMgrs).toBe(2);
  });

  it('derives numOsds and numPGs from record count', () => {
    // 3 records -> numOsds = max(1, 3*2) = 6; numPGs = max(128, 3*64) = 192.
    const records: StorageRecord[] = ['n1', 'n2', 'n3'].map((node) =>
      makeRecord({
        id: `r-${node}`,
        details: { type: 'rbd', node, parentName: node },
      }),
    );
    const [cluster] = deriveCephClustersFromStorageRecords(records);
    expect(cluster.numOsds).toBe(6);
    expect(cluster.numOsdsUp).toBe(6);
    expect(cluster.numOsdsIn).toBe(6);
    expect(cluster.numPGs).toBe(192);
  });

  it('uses the derived cluster shape with HEALTH_UNKNOWN and a diagnostic healthMessage', () => {
    const [cluster] = deriveCephClustersFromStorageRecords([makeRecord()]);
    expect(cluster.health).toBe('HEALTH_UNKNOWN');
    expect(cluster.healthMessage).toBe(
      'Derived from storage metrics - live Ceph telemetry unavailable.',
    );
    expect(cluster.pools).toBeUndefined();
    expect(cluster.services).toBeUndefined();
    expect(cluster.name).toBe('cluster-main Ceph');
  });
});

// ===========================================================================
// summarizeCephClusters
// ===========================================================================

describe('summarizeCephClusters branch coverage', () => {
  it('returns all-zero totals and usagePercent 0 for an empty clusters array', () => {
    expect(summarizeCephClusters([])).toEqual({
      clusters: [],
      totalBytes: 0,
      usedBytes: 0,
      availableBytes: 0,
      usagePercent: 0,
    });
  });

  it('clamps negative byte values to 0 in the totals', () => {
    const result = summarizeCephClusters([
      makeCluster({ totalBytes: -100, usedBytes: -50, availableBytes: -25 }),
    ]);
    expect(result.totalBytes).toBe(0);
    expect(result.usedBytes).toBe(0);
    expect(result.availableBytes).toBe(0);
    expect(result.usagePercent).toBe(0);
  });

  it('treats null byte fields as 0 via the || fallback', () => {
    const result = summarizeCephClusters([
      {
        ...makeCluster(),
        totalBytes: null as unknown as number,
        usedBytes: null as unknown as number,
        availableBytes: null as unknown as number,
      },
    ]);
    expect(result.totalBytes).toBe(0);
    expect(result.usedBytes).toBe(0);
    expect(result.availableBytes).toBe(0);
  });

  it('sums totals across multiple clusters and computes aggregate usagePercent', () => {
    const result = summarizeCephClusters([
      makeCluster({ totalBytes: 400, usedBytes: 100, availableBytes: 300 }),
      makeCluster({
        id: 'c-2',
        instance: 'c2',
        name: 'c2 Ceph',
        totalBytes: 600,
        usedBytes: 300,
        availableBytes: 300,
      }),
    ]);
    expect(result.clusters).toHaveLength(2);
    expect(result.totalBytes).toBe(1000);
    expect(result.usedBytes).toBe(400);
    expect(result.availableBytes).toBe(600);
    expect(result.usagePercent).toBe(40);
  });

  it('yields usagePercent 0 when the summed total is 0', () => {
    const result = summarizeCephClusters([
      makeCluster({ totalBytes: 0, usedBytes: 50, availableBytes: 0 }),
    ]);
    expect(result.usagePercent).toBe(0);
  });
});

// ===========================================================================
// resolveCephClusterForStorageRecord
// ===========================================================================

describe('resolveCephClusterForStorageRecord branch coverage', () => {
  it('returns null when the lookup is empty', () => {
    expect(resolveCephClusterForStorageRecord(makeRecord(), {})).toBeNull();
  });

  it('returns null when the resolved key is not present in the lookup', () => {
    const cluster = makeCluster({ instance: 'other-cluster' });
    expect(
      resolveCephClusterForStorageRecord(makeRecord(), { 'other-cluster': cluster }),
    ).toBeNull();
  });

  it('resolves via refs.platformEntityId (first-precedence key source)', () => {
    const cluster = makeCluster();
    const record = makeRecord({ refs: { platformEntityId: 'pe-key' } });
    expect(resolveCephClusterForStorageRecord(record, { 'pe-key': cluster })).toBe(cluster);
  });

  it('resolves via details.parentId when platformEntityId is empty', () => {
    const cluster = makeCluster();
    const record = makeRecord({
      refs: { platformEntityId: '' },
      details: { type: 'rbd', parentId: 'par-key', node: 'pve1', parentName: 'pve1' },
    });
    expect(resolveCephClusterForStorageRecord(record, { 'par-key': cluster })).toBe(cluster);
  });

  it('resolves via location.label when platformEntityId and parentId are absent', () => {
    const cluster = makeCluster();
    const record = makeRecord({
      refs: { platformEntityId: '' },
      details: { type: 'rbd', node: 'pve1', parentName: 'pve1' },
      location: { label: 'loc-key', scope: 'cluster' },
    });
    expect(resolveCephClusterForStorageRecord(record, { 'loc-key': cluster })).toBe(cluster);
  });

  it('resolves via source.platform when all higher-precedence identity fields are empty', () => {
    const cluster = makeCluster();
    const record = makeRecord({
      refs: { platformEntityId: '' },
      details: { type: 'rbd', node: 'pve1', parentName: 'pve1' },
      location: { label: '', scope: 'cluster' },
      source: {
        platform: 'plat-key',
        family: 'virtualization',
        origin: 'resource',
        adapterId: 'a',
      },
    });
    expect(resolveCephClusterForStorageRecord(record, { 'plat-key': cluster })).toBe(cluster);
  });
});
