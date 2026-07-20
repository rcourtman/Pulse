import { describe, expect, it } from 'vitest';
import type { StorageCapability, StorageRecord } from '@/features/storageBackups/models';
import {
  findSelectedStorageNode,
  groupStorageRecords,
  matchesStorageRecordNode,
  matchesStorageRecordSearch,
  sortStorageRecords,
  type StorageNodeOption,
  type StorageSortContext,
} from '@/features/storageBackups/storageModelCore';

const makeNode = (overrides: Partial<StorageNodeOption> = {}): StorageNodeOption => ({
  id: 'node-1',
  label: 'pve1',
  aliases: ['cluster-main', 'pve1.local'],
  ...overrides,
});

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-1',
    name: 'tank',
    category: 'pool',
    health: 'healthy',
    location: { label: 'pve1', scope: 'node' },
    capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
    capabilities: ['capacity'],
    source: {
      platform: 'proxmox-pve',
      family: 'virtualization',
      origin: 'resource',
      adapterId: 'resource-storage',
    },
    observedAt: Date.now(),
    statusLabel: 'Healthy',
    details: { node: 'pve1', nodeHints: ['cluster-main'] },
    refs: { platformEntityId: '' },
    ...overrides,
  }) as StorageRecord;

const growthEntry = (deltaBytes: number | null) => ({
  deltaBytes,
  label: deltaBytes === null ? '—' : `${deltaBytes}`,
  title: 'growth',
  toneClass: 'text-muted',
});

// ---------------------------------------------------------------------------
// findSelectedStorageNode
// ---------------------------------------------------------------------------

describe('findSelectedStorageNode branch coverage', () => {
  it('returns null for the "all" sentinel (early return)', () => {
    expect(findSelectedStorageNode('all', [makeNode()])).toBeNull();
  });

  it('returns null when no node option matches (|| null arm)', () => {
    expect(findSelectedStorageNode('does-not-exist', [makeNode()])).toBeNull();
  });

  it('returns null when the options array is empty (find -> undefined -> || null)', () => {
    expect(findSelectedStorageNode('x', [])).toBeNull();
  });

  it('returns the exact matching node option', () => {
    const a = makeNode({ id: 'a', label: 'alpha' });
    const b = makeNode({ id: 'b', label: 'beta' });
    expect(findSelectedStorageNode('b', [a, b])).toStrictEqual(b);
  });
});

// ---------------------------------------------------------------------------
// matchesStorageRecordNode
// ---------------------------------------------------------------------------

describe('matchesStorageRecordNode branch coverage', () => {
  it('returns true when node is null (!node early return)', () => {
    expect(matchesStorageRecordNode(makeRecord(), null)).toBe(true);
  });

  it('matches via the node id against a record hint', () => {
    const node = makeNode({ id: 'pve1', label: 'other', aliases: [] });
    expect(matchesStorageRecordNode(makeRecord(), node)).toBe(true);
  });

  it('matches via an alias when id and label do not match any hint', () => {
    const node = makeNode({ id: 'unrelated', label: 'unrelated', aliases: ['cluster-main'] });
    expect(matchesStorageRecordNode(makeRecord(), node)).toBe(true);
  });

  it('uses [] when node.aliases is undefined (|| [] arm)', () => {
    const node: StorageNodeOption = { id: 'pve1', label: 'pve1' };
    expect(matchesStorageRecordNode(makeRecord(), node)).toBe(true);
  });

  it('normalizes falsy id/label to "" and drops them ((value || "") + filter(Boolean))', () => {
    const node = makeNode({ id: '', label: '', aliases: ['pve1'] });
    expect(matchesStorageRecordNode(makeRecord(), node)).toBe(true);
  });

  it('returns false when no alias matches any hint', () => {
    const node = makeNode({ id: 'zzz', label: 'yyy', aliases: ['nope'] });
    expect(matchesStorageRecordNode(makeRecord(), node)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// groupStorageRecords
// ---------------------------------------------------------------------------

describe('groupStorageRecords branch coverage', () => {
  it('returns [] for empty input (records.length === 0)', () => {
    expect(groupStorageRecords([], 'node')).toEqual([]);
    expect(groupStorageRecords([], 'type')).toEqual([]);
  });

  it('groups everything under "All" with merged stats when groupBy is "none"', () => {
    const a = makeRecord({ id: 'a' });
    const b = makeRecord({ id: 'b' });
    const result = groupStorageRecords([a, b], 'none');
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe('All');
    expect(result[0].items).toStrictEqual([a, b]);
    expect(result[0].stats).toStrictEqual({
      totalBytes: 200,
      usedBytes: 80,
      usagePercent: 40,
      byHealth: { healthy: 2, warning: 0, critical: 0, offline: 0, unknown: 0 },
    });
  });

  it('groups by type and sorts the group keys ascending', () => {
    const zfs = makeRecord({ id: 'z', name: 'zfs-pool', details: { type: 'zfs' } });
    const dir = makeRecord({ id: 'd', name: 'dir-pool', details: { type: 'dir' } });
    const result = groupStorageRecords([zfs, dir], 'type');
    expect(result.map((group) => group.key)).toEqual(['dir', 'zfs']);
    expect(result[0].items).toStrictEqual([dir]);
    expect(result[1].items).toStrictEqual([zfs]);
  });

  it('groups by node via getStorageRecordNodeLabel (else arm) and buckets repeats', () => {
    const a = makeRecord({ id: 'a', details: { node: 'pve1' } });
    const b = makeRecord({ id: 'b', details: { node: 'pve1' } });
    const c = makeRecord({ id: 'c', details: { node: 'pve2' } });
    const result = groupStorageRecords([a, b, c], 'node');
    expect(result.map((group) => group.key)).toEqual(['pve1', 'pve2']);
    expect(result[0].items).toStrictEqual([a, b]);
    expect(result[1].items).toStrictEqual([c]);
  });

  it('groups by status via getStorageRecordStatus and sorts keys', () => {
    const ok = makeRecord({ id: 'a', details: { status: 'available' } });
    const bad = makeRecord({ id: 'b', details: { status: 'degraded' } });
    const result = groupStorageRecords([ok, bad], 'status');
    expect(result.map((group) => group.key)).toEqual(['available', 'degraded']);
  });

  it('collapses a whitespace-only type key to "unknown" (key.trim() || "unknown")', () => {
    const rec = makeRecord({ id: 'x', name: 'X', details: { type: '   ' } });
    const result = groupStorageRecords([rec], 'type');
    expect(result.map((group) => group.key)).toEqual(['unknown']);
  });

  it('reuses an existing group bucket (if (!groups.has) branch, false arm)', () => {
    const a = makeRecord({ id: 'a', details: { type: 'zfs' } });
    const b = makeRecord({ id: 'b', details: { type: 'zfs' } });
    const result = groupStorageRecords([a, b], 'type');
    expect(result).toHaveLength(1);
    expect(result[0].key).toBe('zfs');
    expect(result[0].items).toStrictEqual([a, b]);
    expect(result[0].stats).toStrictEqual({
      totalBytes: 200,
      usedBytes: 80,
      usagePercent: 40,
      byHealth: { healthy: 2, warning: 0, critical: 0, offline: 0, unknown: 0 },
    });
  });
});

// ---------------------------------------------------------------------------
// matchesStorageRecordSearch
// ---------------------------------------------------------------------------

describe('matchesStorageRecordSearch branch coverage', () => {
  it('returns true for an empty query (!query arm)', () => {
    expect(matchesStorageRecordSearch(makeRecord(), '')).toBe(true);
  });

  it('returns true when only node terms match and there are no free terms', () => {
    expect(matchesStorageRecordSearch(makeRecord(), 'node:pve1')).toBe(true);
  });

  it('returns false when a node term fails (matchesStorageNodeTerms false branch)', () => {
    expect(matchesStorageRecordSearch(makeRecord(), 'node:nope')).toBe(false);
  });

  it('matches a free term found in capabilities', () => {
    const rec = makeRecord({ capabilities: ['replication', 'snapshots'] });
    expect(matchesStorageRecordSearch(rec, 'replication')).toBe(true);
  });

  it('spreads [] when capabilities is undefined (... || [] arm) and drops the capability token', () => {
    const rec = makeRecord({
      capabilities: undefined as unknown as StorageCapability[],
    });
    expect(matchesStorageRecordSearch(rec, 'tank')).toBe(true);
    // 'capacity' was the default capability; now absent from the haystack.
    expect(matchesStorageRecordSearch(rec, 'capacity')).toBe(false);
  });

  it('matches a free term present in an optional summary field (actionSummary)', () => {
    const rec = makeRecord({ actionSummary: 'Scrub running' });
    expect(matchesStorageRecordSearch(rec, 'scrub')).toBe(true);
  });

  it('returns false when one free term matches but another does not (every() false arm)', () => {
    expect(matchesStorageRecordSearch(makeRecord(), 'tank missing')).toBe(false);
  });

  it('returns false when no free term matches at all', () => {
    expect(matchesStorageRecordSearch(makeRecord(), 'zzzzzz')).toBe(false);
  });

  it('returns true when a node term and a free term both match', () => {
    expect(matchesStorageRecordSearch(makeRecord(), 'node:cluster-main tank')).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// sortStorageRecords (and the inner numericCompare closure)
// ---------------------------------------------------------------------------

describe('sortStorageRecords branch coverage', () => {
  it('sorts by usage asc and desc (numericCompare < and > arms)', () => {
    const low = makeRecord({
      id: 'low',
      name: 'low',
      capacity: { totalBytes: 100, usedBytes: 10, freeBytes: 90, usagePercent: 10 },
    });
    const high = makeRecord({
      id: 'high',
      name: 'high',
      capacity: { totalBytes: 100, usedBytes: 90, freeBytes: 10, usagePercent: 90 },
    });
    expect(sortStorageRecords([high, low], 'usage', 'asc').map((r) => r.id)).toEqual([
      'low',
      'high',
    ]);
    expect(sortStorageRecords([low, high], 'usage', 'desc').map((r) => r.id)).toEqual([
      'high',
      'low',
    ]);
  });

  it('numericCompare === arm: equal usage falls through to the name tie-break', () => {
    const a = makeRecord({
      id: 'a',
      name: 'Bravo',
      capacity: { totalBytes: 100, usedBytes: 50, freeBytes: 50, usagePercent: 50 },
    });
    const b = makeRecord({
      id: 'b',
      name: 'Alpha',
      capacity: { totalBytes: 100, usedBytes: 50, freeBytes: 50, usagePercent: 50 },
    });
    expect(sortStorageRecords([a, b], 'usage', 'asc').map((r) => r.name)).toEqual([
      'Alpha',
      'Bravo',
    ]);
    expect(sortStorageRecords([a, b], 'usage', 'desc').map((r) => r.name)).toEqual([
      'Bravo',
      'Alpha',
    ]);
  });

  it('sorts by name through the default else arm', () => {
    const a = makeRecord({ id: 'a', name: 'Zeta' });
    const b = makeRecord({ id: 'b', name: 'Alpha' });
    expect(sortStorageRecords([a, b], 'name', 'asc').map((r) => r.name)).toEqual(['Alpha', 'Zeta']);
    expect(sortStorageRecords([a, b], 'name', 'desc').map((r) => r.name)).toEqual([
      'Zeta',
      'Alpha',
    ]);
  });

  it('sorts by type via textCompare', () => {
    const zfs = makeRecord({ id: 'a', name: 'a', details: { type: 'zfs' } });
    const dir = makeRecord({ id: 'b', name: 'b', details: { type: 'dir' } });
    expect(sortStorageRecords([zfs, dir], 'type', 'asc').map((r) => r.id)).toEqual(['b', 'a']);
  });

  it('sorts by priority, defaulting a missing incidentPriority to 0 (|| 0 arm)', () => {
    const withPrio = makeRecord({ id: 'p', name: 'p', incidentPriority: 5 });
    const noPrio = makeRecord({ id: 'n', name: 'n' });
    expect(sortStorageRecords([withPrio, noPrio], 'priority', 'asc').map((r) => r.id)).toEqual([
      'n',
      'p',
    ]);
    expect(sortStorageRecords([withPrio, noPrio], 'priority', 'desc').map((r) => r.id)).toEqual([
      'p',
      'n',
    ]);
  });

  it('growth: left null, right non-null returns 1 (null sorts after in asc)', () => {
    const withGrowth = makeRecord({ id: 'g', name: 'g' });
    const noGrowth = makeRecord({ id: 'n', name: 'n' });
    const ctx: StorageSortContext = {
      growthBySeriesId: new Map([['g', growthEntry(100)]]),
    };
    expect(
      sortStorageRecords([noGrowth, withGrowth], 'growth', 'asc', ctx).map((r) => r.id),
    ).toEqual(['g', 'n']);
  });

  it('growth: left non-null, right null returns -1 (early return bypasses direction flip)', () => {
    const withGrowth = makeRecord({ id: 'g', name: 'g' });
    const noGrowth = makeRecord({ id: 'n', name: 'n' });
    const ctx: StorageSortContext = {
      growthBySeriesId: new Map([['g', growthEntry(100)]]),
    };
    // Even in desc the null-growth record stays last because of the early `return -1`.
    expect(
      sortStorageRecords([withGrowth, noGrowth], 'growth', 'desc', ctx).map((r) => r.id),
    ).toEqual(['g', 'n']);
  });

  it('growth: both null -> comparison 0 -> name tie-break', () => {
    const a = makeRecord({ id: 'a', name: 'Bravo' });
    const b = makeRecord({ id: 'b', name: 'Alpha' });
    const ctx: StorageSortContext = { growthBySeriesId: new Map() };
    expect(sortStorageRecords([a, b], 'growth', 'asc', ctx).map((r) => r.name)).toEqual([
      'Alpha',
      'Bravo',
    ]);
  });

  it('growth: no context -> growthBySeriesId undefined -> ?? null for all -> name tie-break', () => {
    const a = makeRecord({ id: 'a', name: 'Bravo' });
    const b = makeRecord({ id: 'b', name: 'Alpha' });
    expect(sortStorageRecords([a, b], 'growth', 'asc').map((r) => r.name)).toEqual([
      'Alpha',
      'Bravo',
    ]);
  });

  it('growth: both non-null -> numericCompare (asc and desc)', () => {
    const small = makeRecord({ id: 's', name: 's' });
    const big = makeRecord({ id: 'b', name: 'b' });
    const ctx: StorageSortContext = {
      growthBySeriesId: new Map([
        ['s', growthEntry(10)],
        ['b', growthEntry(999)],
      ]),
    };
    expect(sortStorageRecords([big, small], 'growth', 'asc', ctx).map((r) => r.id)).toEqual([
      's',
      'b',
    ]);
    expect(sortStorageRecords([small, big], 'growth', 'desc', ctx).map((r) => r.id)).toEqual([
      'b',
      's',
    ]);
  });

  it('growth: deltaBytes === 0 is NOT collapsed to null by ?? (compared as 0)', () => {
    const zero = makeRecord({ id: 'z', name: 'z' });
    const big = makeRecord({ id: 'b', name: 'b' });
    const ctx: StorageSortContext = {
      growthBySeriesId: new Map([
        ['z', growthEntry(0)],
        ['b', growthEntry(999)],
      ]),
    };
    expect(sortStorageRecords([big, zero], 'growth', 'asc', ctx).map((r) => r.id)).toEqual([
      'z',
      'b',
    ]);
  });

  it('growth: entry present but deltaBytes null -> treated as null', () => {
    const nullDelta = makeRecord({ id: 'nd', name: 'nd' });
    const val = makeRecord({ id: 'v', name: 'v' });
    const ctx: StorageSortContext = {
      growthBySeriesId: new Map([
        ['nd', growthEntry(null)],
        ['v', growthEntry(50)],
      ]),
    };
    expect(sortStorageRecords([nullDelta, val], 'growth', 'asc', ctx).map((r) => r.id)).toEqual([
      'v',
      'nd',
    ]);
  });

  it('returns a new sorted array and does not mutate the input ([...records] copy)', () => {
    const a = makeRecord({ id: 'a', name: 'a' });
    const b = makeRecord({ id: 'b', name: 'b' });
    const input = [b, a];
    const sorted = sortStorageRecords(input, 'name', 'asc');
    expect(sorted).not.toBe(input);
    expect(input.map((r) => r.id)).toEqual(['b', 'a']);
    expect(sorted.map((r) => r.id)).toEqual(['a', 'b']);
  });
});
