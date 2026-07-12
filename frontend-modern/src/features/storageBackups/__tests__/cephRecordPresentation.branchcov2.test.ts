import { describe, expect, it } from 'vitest';
import type { CephCluster } from '@/types/api';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import {
  consolidateCephClusterPoolRecords,
  getCephClusterKeyFromStorageRecord,
  getCephSummaryText,
  isCephClusterPoolStorageRecord,
} from '@/features/storageBackups/cephRecordPresentation';

// ---------------------------------------------------------------------------
// Fixture builders — mirror cephRecordPresentation.test.ts so casts and
// import paths match the sibling suite. The defaults describe a healthy PVE
// cephfs mount ("not" a cluster-internal pool row) so individual cases can
// opt into pool-row synthesis by overriding `details`.
// ---------------------------------------------------------------------------

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

// A cluster-internal Ceph pool row (models.StorageFromCephPool): type "ceph"
// homed on the "cluster" pseudo-node.
const makePoolRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  makeRecord({
    id: 'pool-row',
    name: 'cephfs-data',
    health: 'warning',
    details: { type: 'ceph', node: 'cluster', status: 'degraded' },
    ...overrides,
  });

// A PVE-side mount row that is NOT itself a cluster pool row.
const makeMountRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  makeRecord({
    id: 'mount-row',
    name: 'cephfs-data',
    health: 'healthy',
    details: { type: 'cephfs', node: 'shared', status: 'online' },
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

// ===========================================================================
// isCephClusterPoolStorageRecord
// ===========================================================================

describe('isCephClusterPoolStorageRecord branch coverage', () => {
  it('returns false on the early-return arm when the derived type is not "ceph"', () => {
    // type defaults to 'rbd' → getStorageRecordType !== 'ceph' → false.
    expect(isCephClusterPoolStorageRecord(makeRecord())).toBe(false);
    // category-derived type: no details.type, category 'pool' → 'pool' → false.
    expect(
      isCephClusterPoolStorageRecord(
        makeRecord({ details: undefined, category: 'pool' }) as StorageRecord,
      ),
    ).toBe(false);
  });

  it('returns true when type is "ceph" and details.node === "cluster"', () => {
    expect(
      isCephClusterPoolStorageRecord(
        makeRecord({ details: { type: 'ceph', node: 'cluster' } }),
      ),
    ).toBe(true);
  });

  it('returns false when type is "ceph" but details.node is not "cluster"', () => {
    // Exercises the `details.node === 'cluster'` false arm with a valid ceph type.
    expect(
      isCephClusterPoolStorageRecord(
        makeRecord({ details: { type: 'ceph', node: 'pve1' } }),
      ),
    ).toBe(false);
  });

  it('exercises the `(record.details || {})` defensive arm: details undefined with type "ceph"', () => {
    // details is undefined → `(record.details || {})` yields {} → details.node
    // is undefined → not 'cluster' → false. The type-only path proves the
    // `|| {}` branch is taken (no throw, undefined.node coerced via {}).
    expect(
      isCephClusterPoolStorageRecord(
        makeRecord({ details: { type: 'ceph' } }) as StorageRecord,
      ),
    ).toBe(false);
  });
});

// ===========================================================================
// consolidateCephClusterPoolRecords
// (also exercises the module-private storageHealthRank, getRecordPoolDetail
//  and getRecordStatusDetail through the public surface)
// ===========================================================================

describe('consolidateCephClusterPoolRecords branch coverage', () => {
  // ---- Branch: `poolRecords.length === 0` → returns the input reference ----
  it('short-circuits and returns the exact input reference when there are no pool rows', () => {
    const input: StorageRecord[] = [makeMountRecord(), makeMountRecord({ id: 'm2' })];
    // Early return at `if (poolRecords.length === 0) return records;` — same ref.
    expect(consolidateCephClusterPoolRecords(input)).toBe(input);
  });

  // ---- Branch: `consumedPoolIds.size === 0` → returns the input reference --
  it('returns the exact input reference when pool rows exist but none match a mount', () => {
    // A pool row plus a non-ceph record: the pool finds no ceph sibling, so
    // consumedPoolIds stays empty and the second early-return fires.
    const orphanPool = makePoolRecord({ id: 'orphan', name: 'unmatched-pool' });
    const nonCeph = makeRecord({
      id: 'local',
      name: 'local-zfs',
      details: { type: 'zfspool', node: 'pve1' },
      capabilities: ['capacity'],
    });
    const input: StorageRecord[] = [orphanPool, nonCeph];
    expect(consolidateCephClusterPoolRecords(input)).toBe(input);
  });

  // ---- Branch L83: `if (!pool) return record;` (record with no lifted pool) -
  it('passes unrelated records through unchanged (by reference) during consolidation', () => {
    const pool = makePoolRecord({ id: 'pool-1', name: 'cephfs-data', health: 'critical' });
    const mount = makeMountRecord({ id: 'mount-1', name: 'cephfs-data' });
    const unrelated = makeRecord({
      id: 'unrelated-1',
      name: 'local-zfs',
      details: { type: 'zfspool', node: 'pve1' },
      capabilities: ['capacity'],
    });
    const consolidated = consolidateCephClusterPoolRecords([pool, mount, unrelated]);
    // Pool consumed; mount lifted; unrelated has no entry in liftBySiblingId
    // → the `if (!pool) return record;` arm returns it by reference.
    expect(consolidated.map((r) => r.id)).toEqual(['mount-1', 'unrelated-1']);
    const unrelatedOut = consolidated.find((r) => r.id === 'unrelated-1');
    expect(unrelatedOut).toBe(unrelated);
  });

  // ---- Branch L72 (!existing → set) + L84 lift + L85 status-detail arm ----
  it('lifts the pool health onto a healthier mount and synthesizes a status from details.status', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'warning',
      details: { type: 'ceph', node: 'cluster', status: 'degraded' },
    });
    const mount = makeMountRecord({ id: 'mount-1', name: 'cephfs-data', health: 'healthy' });
    const [survivor] = consolidateCephClusterPoolRecords([pool, mount]);
    expect(survivor.id).toBe('mount-1');
    expect(survivor.health).toBe('warning');
    // getRecordStatusDetail string arm → 'degraded' feeds poolStatus.
    expect(survivor.statusLabel).toBe('degraded');
    expect(survivor.details?.status).toBe('degraded');
    // No issueSummary on the pool → synthesized fallback string.
    expect(survivor.issueSummary).toBe('Ceph reports pool cephfs-data degraded');
  });

  // ---- Branch L72 (existing present, new pool sicker → replace) -----------
  it('keeps the sicker of two pools that mount onto the same sibling', () => {
    const poolWarning = makePoolRecord({
      id: 'pool-w',
      name: 'cephfs-data',
      health: 'warning',
      details: { type: 'ceph', node: 'cluster', status: 'degraded' },
    });
    const poolCritical = makePoolRecord({
      id: 'pool-c',
      name: 'cephfs-data',
      health: 'critical',
      details: { type: 'ceph', node: 'cluster', status: 'unavailable' },
    });
    const mount = makeMountRecord({ id: 'mount-1', name: 'cephfs-data', health: 'healthy' });
    // storageHealthRank('critical')=4 > storageHealthRank('warning')=2 → replace.
    const consolidated = consolidateCephClusterPoolRecords([poolWarning, poolCritical, mount]);
    expect(consolidated.map((r) => r.id)).toEqual(['mount-1']);
    expect(consolidated[0].health).toBe('critical');
    expect(consolidated[0].statusLabel).toBe('unavailable');
  });

  // ---- Branch L72 (existing present, new pool NOT sicker → keep existing) -
  it('does not replace the existing lift when the second pool is not sicker', () => {
    const poolCritical = makePoolRecord({
      id: 'pool-c',
      name: 'cephfs-data',
      health: 'critical',
      issueSummary: 'first pool wins',
      details: { type: 'ceph', node: 'cluster', status: 'unavailable' },
    });
    const poolWarning = makePoolRecord({
      id: 'pool-w',
      name: 'cephfs-data',
      health: 'warning',
      issueSummary: 'second pool loses',
      details: { type: 'ceph', node: 'cluster', status: 'degraded' },
    });
    const mount = makeMountRecord({ id: 'mount-1', name: 'cephfs-data', health: 'healthy' });
    // storageHealthRank('warning')=2 > storageHealthRank('critical')=4 → false → keep critical.
    const [survivor] = consolidateCephClusterPoolRecords([poolCritical, poolWarning, mount]);
    expect(survivor.health).toBe('critical');
    // existing (critical) pool's issueSummary is preserved verbatim.
    expect(survivor.issueSummary).toBe('first pool wins');
  });

  // ---- Branch L84: pool not sicker than mount → record unchanged by ref ----
  it('leaves a mount untouched (by reference) when the pool is not sicker than it', () => {
    const healthyPool = makePoolRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'healthy',
      details: { type: 'ceph', node: 'cluster', status: 'available' },
    });
    const sickMount = makeMountRecord({
      id: 'mount-1',
      name: 'cephfs-data',
      health: 'critical',
      statusLabel: 'unavailable',
    });
    // storageHealthRank('healthy')=0 <= storageHealthRank('critical')=4 → return record.
    const consolidated = consolidateCephClusterPoolRecords([healthyPool, sickMount]);
    expect(consolidated).toEqual([sickMount]);
    expect(consolidated[0]).toBe(sickMount);
  });

  // ---- storageHealthRank `?? 0` arm: unrecognized record health → rank 0 --
  it('treats an unrecognized mount health as rank 0 so a sicker pool still lifts', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'critical',
      details: { type: 'ceph', node: 'cluster', status: 'unavailable' },
    });
    const mount = makeMountRecord({
      id: 'mount-1',
      name: 'cephfs-data',
      // 'quarantined' is not in STORAGE_HEALTH_SEVERITY_RANK → `?? 0`.
      health: 'quarantined' as unknown as NormalizedHealth,
    });
    const [survivor] = consolidateCephClusterPoolRecords([pool, mount]);
    // pool rank 4 > record rank 0 → lift happens; the bogus record health is
    // overwritten by the pool's 'critical'.
    expect(survivor.id).toBe('mount-1');
    expect(survivor.health).toBe('critical');
  });

  // ---- getRecordPoolDetail string arm + trim(): pool detail with whitespace
  it('matches a mount via the backing pool detail and trims whitespace before comparing', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'vm-pool',
      health: 'critical',
      details: { type: 'ceph', node: 'cluster', status: 'unavailable' },
    });
    const mount = makeMountRecord({
      id: 'mount-1',
      name: 'fast-rbd',
      // details.pool has surrounding whitespace; getRecordPoolDetail trims it
      // to 'vm-pool', which then equals pool.name.
      details: { type: 'rbd', node: 'shared', pool: '  vm-pool  ', status: 'online' },
    });
    const consolidated = consolidateCephClusterPoolRecords([pool, mount]);
    expect(consolidated.map((r) => r.id)).toEqual(['mount-1']);
    expect(consolidated[0].health).toBe('critical');
  });

  // ---- getRecordPoolDetail non-string arm: pool detail is a non-string ----
  it('does not treat a non-string pool detail as a backing-pool match', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'vm-pool',
      health: 'critical',
      details: { type: 'ceph', node: 'cluster', status: 'unavailable' },
    });
    const mount = makeMountRecord({
      id: 'mount-1',
      name: 'different-name',
      // details.pool is a number → getRecordPoolDetail returns '' (the
      // `typeof pool === 'string'` false arm). '' !== 'vm-pool' and the names
      // differ too, so this pool is orphaned and the mount is left as-is.
      details: { type: 'rbd', node: 'shared', pool: 999, status: 'online' } as Record<
        string,
        unknown
      >,
    });
    const input: StorageRecord[] = [pool, mount];
    // No sibling consumed → second early-return hands back the input reference.
    expect(consolidateCephClusterPoolRecords(input)).toBe(input);
  });

  // ---- getRecordStatusDetail non-string arm → falls through to statusLabel -
  it('falls back to pool.statusLabel when details.status is not a string', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'warning',
      statusLabel: 'down-label',
      details: {
        type: 'ceph',
        node: 'cluster',
        // status is a number → getRecordStatusDetail returns '' → the `||`
        // chain falls through to pool.statusLabel ('down-label').
        status: 42,
      } as Record<string, unknown>,
    });
    const mount = makeMountRecord({ id: 'mount-1', name: 'cephfs-data', health: 'healthy' });
    const [survivor] = consolidateCephClusterPoolRecords([pool, mount]);
    expect(survivor.health).toBe('warning');
    expect(survivor.statusLabel).toBe('down-label');
    expect(survivor.details?.status).toBe('down-label');
    expect(survivor.issueSummary).toBe('Ceph reports pool cephfs-data down-label');
  });

  // ---- getRecordStatusDetail empty → statusLabel empty → falls to health ---
  it('falls back to pool.health when neither details.status nor statusLabel resolve', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'critical',
      statusLabel: undefined,
      details: {
        type: 'ceph',
        node: 'cluster',
        // status is a boolean → getRecordStatusDetail returns '' (non-string).
        status: true,
      } as Record<string, unknown>,
    });
    const mount = makeMountRecord({ id: 'mount-1', name: 'cephfs-data', health: 'healthy' });
    const [survivor] = consolidateCephClusterPoolRecords([pool, mount]);
    // getRecordStatusDetail('') || statusLabel(undefined) || health('critical').
    expect(survivor.statusLabel).toBe('critical');
    expect(survivor.details?.status).toBe('critical');
    expect(survivor.issueSummary).toBe('Ceph reports pool cephfs-data critical');
  });

  // ---- L91 `pool.issueSummary?.trim() || ...` truthy arm ------------------
  it('preserves the pool issueSummary verbatim when it trims to a non-empty value', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'warning',
      issueSummary: 'Ceph cluster HEALTH_WARN',
      details: { type: 'ceph', node: 'cluster', status: 'degraded' },
    });
    const mount = makeMountRecord({ id: 'mount-1', name: 'cephfs-data', health: 'healthy' });
    const [survivor] = consolidateCephClusterPoolRecords([pool, mount]);
    expect(survivor.issueSummary).toBe('Ceph cluster HEALTH_WARN');
  });

  // ---- L91 `pool.issueSummary?.trim() || ...` whitespace-only arm ---------
  it('synthesizes the issue summary when the pool issueSummary is only whitespace', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'warning',
      issueSummary: '   ',
      details: { type: 'ceph', node: 'cluster', status: 'degraded' },
    });
    const mount = makeMountRecord({ id: 'mount-1', name: 'cephfs-data', health: 'healthy' });
    const [survivor] = consolidateCephClusterPoolRecords([pool, mount]);
    // '   '.trim() === '' → falsy → synthesized fallback string.
    expect(survivor.issueSummary).toBe('Ceph reports pool cephfs-data degraded');
  });

  // ---- L92 `{ ...(record.details || {}), status: poolStatus }` arm --------
  it('preserves the surviving mount details and overlays the lifted status', () => {
    const pool = makePoolRecord({
      id: 'pool-1',
      name: 'cephfs-data',
      health: 'warning',
      details: { type: 'ceph', node: 'cluster', status: 'degraded' },
    });
    const mount = makeMountRecord({
      id: 'mount-1',
      name: 'cephfs-data',
      health: 'healthy',
      details: { type: 'cephfs', node: 'shared', status: 'online', extra: 'kept' },
    });
    const [survivor] = consolidateCephClusterPoolRecords([pool, mount]);
    // record.details was truthy → spread keeps existing keys, status overridden.
    expect(survivor.details).toEqual({
      type: 'cephfs',
      node: 'shared',
      status: 'degraded',
      extra: 'kept',
    });
  });
});

// ===========================================================================
// getCephClusterKeyFromStorageRecord
// ===========================================================================

describe('getCephClusterKeyFromStorageRecord branch coverage', () => {
  it('prefers refs.platformEntityId when it is a non-empty string', () => {
    expect(getCephClusterKeyFromStorageRecord(makeRecord())).toBe('cluster-a');
  });

  it('uses details.parentId when refs is undefined (optional-chain short-circuit)', () => {
    // refs undefined → `record.refs?.platformEntityId` is undefined → falls to
    // parent. Exercises the `record.refs?.` optional-chain falsy arm.
    const record = makeRecord({
      refs: undefined,
      details: { type: 'rbd', parentId: 'parent-key' },
    });
    expect(getCephClusterKeyFromStorageRecord(record)).toBe('parent-key');
  });

  it('uses details.parentId when refs.platformEntityId is an empty string', () => {
    const record = makeRecord({
      refs: { platformEntityId: '' },
      details: { type: 'rbd', parentId: 'parent-key' },
    });
    expect(getCephClusterKeyFromStorageRecord(record)).toBe('parent-key');
  });

  it('coerces a non-string parentId to "" (defensive ternary false arm) then uses location.label', () => {
    const record = makeRecord({
      refs: { platformEntityId: '' },
      details: { type: 'rbd', parentId: 999 } as Record<string, unknown>,
      location: { label: 'loc-key', scope: 'cluster' },
    });
    // typeof parentId === 'string' is false → parent '' → next fallback wins.
    expect(getCephClusterKeyFromStorageRecord(record)).toBe('loc-key');
  });

  it('falls back to location.label when refs and parentId are both absent', () => {
    const record = makeRecord({
      refs: undefined,
      details: { type: 'rbd' },
      location: { label: 'loc-key', scope: 'cluster' },
    });
    expect(getCephClusterKeyFromStorageRecord(record)).toBe('loc-key');
  });

  it('exercises the `(record.details || {})` arm then falls to source.platform', () => {
    // details undefined → {} → parent '' → refs/platformEntityId absent →
    // location.label '' → final fallback source.platform.
    const record = makeRecord({
      refs: undefined,
      details: undefined,
      location: { label: '', scope: 'cluster' },
      source: {
        platform: 'proxmox-pve',
        family: 'virtualization',
        origin: 'resource',
        adapterId: 'resource-storage',
      },
    });
    expect(getCephClusterKeyFromStorageRecord(record)).toBe('proxmox-pve');
  });

  it('reaches the source.platform final fallback when every higher-precedence key is empty', () => {
    const record = makeRecord({
      refs: { platformEntityId: '' },
      details: { type: 'rbd', parentId: '' },
      location: { label: '', scope: 'cluster' },
      source: {
        platform: 'truenas',
        family: 'virtualization',
        origin: 'resource',
        adapterId: 'resource-storage',
      },
    });
    expect(getCephClusterKeyFromStorageRecord(record)).toBe('truenas');
  });
});

// ===========================================================================
// getCephSummaryText
// ===========================================================================

describe('getCephSummaryText branch coverage', () => {
  // ---- Cluster arm: capacity + OSDs + PGs (all three parts) ---------------
  it('formats capacity, OSDs and PGs parts when the cluster provides all three', () => {
    // totalBytes 1000, usedBytes 400 → 40% → "400 B / 1000 B (40%)".
    // numOsds/numOsdsUp finite (6/6) and numPGs > 0 (256) → all parts pushed.
    expect(getCephSummaryText(makeRecord(), makeCluster())).toBe(
      '400 B / 1000 B (40%) • OSDs 6/6 • PGs 256',
    );
  });

  // ---- Cluster arm: percent===0 when totalBytes is 0 (no division) -------
  it('renders a zero-capacity cluster as "0 B / 0 B (0%)" without dividing by zero', () => {
    expect(
      getCephSummaryText(
        makeRecord(),
        makeCluster({ totalBytes: 0, usedBytes: 0, availableBytes: 0, numOsds: NaN, numPGs: 0 }),
      ),
    ).toBe('0 B / 0 B (0%)');
  });

  // ---- Cluster arm: OSDs omitted when numOsds/numOsdsUp not finite --------
  it('omits the OSDs part when numOsds is NaN', () => {
    expect(
      getCephSummaryText(
        makeRecord(),
        makeCluster({ numOsds: NaN, numOsdsUp: 6, numPGs: 0 }),
      ),
    ).toBe('400 B / 1000 B (40%)');
  });

  it('omits the OSDs part when numOsdsUp is NaN even if numOsds is finite', () => {
    expect(
      getCephSummaryText(
        makeRecord(),
        makeCluster({ numOsds: 6, numOsdsUp: NaN, numPGs: 0 }),
      ),
    ).toBe('400 B / 1000 B (40%)');
  });

  // ---- Cluster arm: PGs omitted when numPGs is 0 or not finite ------------
  it('keeps OSDs but omits PGs when numPGs is 0', () => {
    expect(
      getCephSummaryText(makeRecord(), makeCluster({ numPGs: 0 })),
    ).toBe('400 B / 1000 B (40%) • OSDs 6/6');
  });

  it('omits PGs when numPGs is NaN (Number.isFinite false)', () => {
    expect(
      getCephSummaryText(makeRecord(), makeCluster({ numPGs: NaN })),
    ).toBe('400 B / 1000 B (40%) • OSDs 6/6');
  });

  // ---- Cluster arm: large numPGs is rendered via toLocaleString ----------
  it('renders a large numPGs count with toLocaleString grouping', () => {
    expect(
      getCephSummaryText(makeRecord(), makeCluster({ numPGs: 1_500 })),
    ).toBe('400 B / 1000 B (40%) • OSDs 6/6 • PGs 1,500');
  });

  // ---- Cluster `|| 0` clamps: negative bytes treated as 0 ----------------
  it('clamps negative cluster total/used bytes to 0 before formatting', () => {
    // Math.max(0, totalBytes || 0): totalBytes -1000 is truthy → max(0,-1000)=0.
    // usedBytes -400 → 0. percent total>0? no → 0.
    expect(
      getCephSummaryText(
        makeRecord(),
        makeCluster({
          totalBytes: -1_000,
          usedBytes: -400,
          availableBytes: 0,
          numOsds: NaN,
          numPGs: 0,
        }),
      ),
    ).toBe('0 B / 0 B (0%)');
  });

  // ---- Cluster guard false: totalBytes NaN → else (record capacity) branch
  it('falls back to record capacity when cluster.totalBytes is not finite', () => {
    // Number.isFinite(NaN) is false → the whole cluster arm is skipped.
    expect(
      getCephSummaryText(
        makeRecord({ capacity: { totalBytes: 1_000, usedBytes: 400, freeBytes: 600, usagePercent: 40 } }),
        makeCluster({ totalBytes: NaN }),
      ),
    ).toBe('400 B / 1000 B (40%)');
  });

  // ---- Cluster guard false: cluster null → else (record capacity) branch --
  it('formats the record capacity summary when cluster is null', () => {
    expect(
      getCephSummaryText(
        makeRecord({ capacity: { totalBytes: 1_000, usedBytes: 400, freeBytes: 600, usagePercent: 40 } }),
        null,
      ),
    ).toBe('400 B / 1000 B (40%)');
  });

  // ---- Else branch: record total <= 0 → '' -------------------------------
  it('returns the empty string when the record capacity total is <= 0 and there is no cluster', () => {
    expect(
      getCephSummaryText(
        makeRecord({
          capacity: { totalBytes: 0, usedBytes: 0, freeBytes: 0, usagePercent: 0 },
        }),
        null,
      ),
    ).toBe('');
  });

  // ---- Else branch: record `|| 0` clamps nullish bytes -------------------
  it('treats null total/used bytes as 0 in the record fallback and returns ""', () => {
    // totalBytes null → `record.capacity.totalBytes || 0` → 0 → total <= 0 → ''.
    expect(
      getCephSummaryText(
        makeRecord({
          capacity: {
            totalBytes: null,
            usedBytes: null,
            freeBytes: null,
            usagePercent: null,
          },
        }),
        null,
      ),
    ).toBe('');
  });
});
