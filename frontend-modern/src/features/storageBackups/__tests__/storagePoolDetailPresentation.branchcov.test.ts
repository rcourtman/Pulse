import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildStoragePoolDetailConfigRows,
  buildStoragePoolDetailTopologyRows,
  buildStoragePoolDetailZfsSummary,
  getStoragePoolLinkedDisks,
  type StoragePoolDetailLinkedDisk,
} from '@/features/storageBackups/storagePoolDetailPresentation';

const buildRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'host-1', scope: 'host' },
  capacity: { totalBytes: 1000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'health'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

const unraidSource = (adapterId = 'resource-storage') => ({
  platform: 'unraid',
  family: 'onprem' as const,
  origin: 'resource' as const,
  adapterId,
});

const rowValue = (rows: { label: string; value: string }[], label: string): string | undefined =>
  rows.find((row) => row.label === label)?.value;

// ---------------------------------------------------------------------------
// buildStoragePoolDetailConfigRows
// ---------------------------------------------------------------------------

describe('buildStoragePoolDetailConfigRows branch coverage', () => {
  it('splices a Content row at index 3 (before Status) when details.content is present', () => {
    const rows = buildStoragePoolDetailConfigRows(buildRecord({ details: { content: 'Media share' } }));
    expect(rows[3]).toEqual({ label: 'Content', value: 'Media share' });
    expect(rows[4]).toEqual({ label: 'Status', value: 'available' });
  });

  it('omits the Content row entirely when details.content is absent or blank', () => {
    const rows = buildStoragePoolDetailConfigRows(buildRecord({ details: {} }));
    expect(rows.some((row) => row.label === 'Content')).toBe(false);
  });

  it('renders Shared as "Yes" / "No" from a boolean details.shared and "-" when absent', () => {
    expect(rowValue(buildStoragePoolDetailConfigRows(buildRecord({ details: { shared: true } })), 'Shared')).toBe(
      'Yes',
    );
    expect(rowValue(buildStoragePoolDetailConfigRows(buildRecord({ details: { shared: false } })), 'Shared')).toBe(
      'No',
    );
    expect(rowValue(buildStoragePoolDetailConfigRows(buildRecord({ details: {} })), 'Shared')).toBe('-');
  });

  it('shows "n/a" for Used/Free/Total when totalBytes is null (falsy)', () => {
    const rows = buildStoragePoolDetailConfigRows(
      buildRecord({ capacity: { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: null } }),
    );
    expect(rowValue(rows, 'Used')).toBe('n/a');
    expect(rowValue(rows, 'Free')).toBe('n/a');
    expect(rowValue(rows, 'Total')).toBe('n/a');
  });

  it('computes freeBytes from total - used when freeBytes is null and totalBytes > 0', () => {
    const rows = buildStoragePoolDetailConfigRows(
      buildRecord({ capacity: { totalBytes: 2048, usedBytes: 1024, freeBytes: null, usagePercent: 50 } }),
    );
    expect(rowValue(rows, 'Used')).toBe('1.00 KB');
    expect(rowValue(rows, 'Free')).toBe('1.00 KB');
    expect(rowValue(rows, 'Total')).toBe('2.00 KB');
  });

  it('clamps the computed freeBytes to 0 when used exceeds total', () => {
    const rows = buildStoragePoolDetailConfigRows(
      buildRecord({ capacity: { totalBytes: 100, usedBytes: 200, freeBytes: null, usagePercent: 200 } }),
    );
    expect(rowValue(rows, 'Free')).toBe('0 B');
  });
});

// ---------------------------------------------------------------------------
// buildStoragePoolDetailTopologyRows
// (also exercises module-private pluralize + readRecordDetailNumber)
// ---------------------------------------------------------------------------

describe('buildStoragePoolDetailTopologyRows branch coverage', () => {
  it('returns an empty array for a non-UnRAID record', () => {
    expect(buildStoragePoolDetailTopologyRows(buildRecord(), [])).toEqual([]);
  });

  it('summarizes a cache pool with "Cache pool" kind, cache-device count, and optional state/errors', () => {
    const record = buildRecord({
      source: unraidSource(),
      details: { type: 'unraid-cache-pool', platform: 'unraid', arrayState: 'STARTED' },
    });
    const linkedDisks = [
      { role: 'cache', spunDown: false, errorCount: 0 },
      { role: 'cache', spunDown: false, errorCount: 0 },
      { role: 'cache', spunDown: true, errorCount: 5 },
    ] as unknown as StoragePoolDetailLinkedDisk[];

    const rows = buildStoragePoolDetailTopologyRows(record, linkedDisks);

    expect(rowValue(rows, 'Kind')).toBe('Cache pool');
    expect(rowValue(rows, 'State')).toBe('Started');
    expect(rowValue(rows, 'Devices')).toBe('3 disks'); // plural arm of pluralize
    expect(rowValue(rows, 'Spun down')).toBe('1 disk'); // singular arm of pluralize
    expect(rowValue(rows, 'Disk errors')).toBe('5');
    expect(rowValue(rows, 'Parity')).toBeUndefined();
    expect(rowValue(rows, 'Sync')).toBeUndefined();
  });

  it('falls back to linkedDisks.length for Devices when no disk carries the cache role', () => {
    const record = buildRecord({
      source: unraidSource(),
      details: { type: 'unraid-cache-pool', platform: 'unraid' },
    });
    const linkedDisks = [
      { role: '', spunDown: false, errorCount: 0 },
      { role: '', spunDown: false, errorCount: 0 },
    ] as unknown as StoragePoolDetailLinkedDisk[];

    const rows = buildStoragePoolDetailTopologyRows(record, linkedDisks);

    expect(rowValue(rows, 'Devices')).toBe('2 disks');
    expect(rowValue(rows, 'State')).toBeUndefined(); // arrayState absent -> no State row
  });

  it('reports "None configured" parity and pluralized data disks for a minimal array', () => {
    const record = buildRecord({
      source: unraidSource(),
      details: { type: 'unraid-array', platform: 'unraid', topology: 'array' },
    });
    const linkedDisks = [
      { role: 'data', spunDown: false, errorCount: 0 },
      { role: 'data', spunDown: false, errorCount: 0 },
    ] as unknown as StoragePoolDetailLinkedDisk[];

    const rows = buildStoragePoolDetailTopologyRows(record, linkedDisks);

    expect(rowValue(rows, 'Kind')).toBe('Array');
    expect(rowValue(rows, 'Parity')).toBe('None configured'); // parityDisks === 0 arm
    expect(rowValue(rows, 'Data disks')).toBe('2 disks'); // plural arm of pluralize
    expect(rowValue(rows, 'State')).toBeUndefined();
    expect(rowValue(rows, 'Spun down')).toBeUndefined();
    expect(rowValue(rows, 'Disk errors')).toBeUndefined();
    expect(rowValue(rows, 'Sync')).toBeUndefined();
  });

  it('renders the Sync action without a percentage when syncProgress is 0', () => {
    const record = buildRecord({
      source: unraidSource(),
      details: { type: 'unraid-array', platform: 'unraid', topology: 'array', syncAction: 'checking', syncProgress: 0 },
    });
    expect(rowValue(buildStoragePoolDetailTopologyRows(record, []), 'Sync')).toBe('checking');
  });

  it('renders the Sync action with a rounded percentage when syncProgress > 0', () => {
    const record = buildRecord({
      source: unraidSource(),
      details: {
        type: 'unraid-array',
        platform: 'unraid',
        topology: 'array',
        syncAction: 'rebuilding',
        syncProgress: 45.6,
      },
    });
    expect(rowValue(buildStoragePoolDetailTopologyRows(record, []), 'Sync')).toBe('rebuilding (46%)');
  });

  it.each([
    ['a string', 'not-a-number'],
    ['NaN', Number.NaN],
    ['Infinity', Number.POSITIVE_INFINITY],
    ['-Infinity', Number.NEGATIVE_INFINITY],
  ])('treats non-finite/non-numeric syncProgress (%s) as 0 so no percentage is shown', (_label, progress) => {
    const record = buildRecord({
      source: unraidSource(),
      details: {
        type: 'unraid-array',
        platform: 'unraid',
        topology: 'array',
        syncAction: 'checking',
        syncProgress: progress,
      },
    });
    expect(rowValue(buildStoragePoolDetailTopologyRows(record, []), 'Sync')).toBe('checking');
  });
});

// ---------------------------------------------------------------------------
// buildStoragePoolDetailZfsSummary
// ---------------------------------------------------------------------------

describe('buildStoragePoolDetailZfsSummary branch coverage', () => {
  it('returns null when the record has no valid zfs pool', () => {
    expect(buildStoragePoolDetailZfsSummary(buildRecord({ details: {} }))).toBeNull();
    // Malformed pool: toZfsPool rejects a candidate whose devices field is not an array.
    expect(
      buildStoragePoolDetailZfsSummary(buildRecord({ details: { zfsPool: { state: 'ONLINE' } } })),
    ).toBeNull();
  });

  it('produces an empty scan and a null errorSummary for a clean pool with scan "none"', () => {
    const record = buildRecord({
      details: {
        zfsPool: {
          state: 'ONLINE',
          scan: 'none',
          readErrors: 0,
          writeErrors: 0,
          checksumErrors: 0,
          devices: [],
        },
      },
    });
    expect(buildStoragePoolDetailZfsSummary(record)).toEqual({
      state: 'ONLINE',
      scan: '',
      errorSummary: null,
      devices: [],
    });
  });

  it('treats an empty-string scan the same as "none"', () => {
    const record = buildRecord({
      details: {
        zfsPool: { state: 'DEGRADED', scan: '', readErrors: 0, writeErrors: 0, checksumErrors: 0, devices: [] },
      },
    });
    expect(buildStoragePoolDetailZfsSummary(record)).toEqual({
      state: 'DEGRADED',
      scan: '',
      errorSummary: null,
      devices: [],
    });
  });

  it('defaults missing device fields and yields an empty per-device error summary when all errors are zero', () => {
    const record = buildRecord({
      details: {
        zfsPool: {
          state: 'ONLINE',
          scan: 'scrub ok',
          readErrors: 0,
          writeErrors: 0,
          checksumErrors: 0,
          devices: [{ name: 'sdc' }],
        },
      },
    });
    expect(buildStoragePoolDetailZfsSummary(record)).toEqual({
      state: 'ONLINE',
      scan: 'scrub ok',
      errorSummary: null,
      devices: [{ name: 'sdc', type: '', state: '', errorSummary: '', message: '' }],
    });
  });

  it('builds a non-null pool-level errorSummary when any pool error counter is positive', () => {
    const record = buildRecord({
      details: {
        zfsPool: {
          state: 'FAULTED',
          scan: 'none',
          readErrors: 1,
          writeErrors: 0,
          checksumErrors: 0,
          devices: [],
        },
      },
    });
    expect(buildStoragePoolDetailZfsSummary(record)).toEqual({
      state: 'FAULTED',
      scan: '',
      errorSummary: 'Errors: R:1 W:0 C:0',
      devices: [],
    });
  });
});

// ---------------------------------------------------------------------------
// getStoragePoolLinkedDisks
// (also exercises module-private diskRoleRank, readPhysicalDisk, readDiskDevPath,
//  and getUnraidStorageGroup)
// ---------------------------------------------------------------------------

describe('getStoragePoolLinkedDisks branch coverage', () => {
  it('links a disk whose parentId matches record.id (direct-parent arm)', () => {
    const record = buildRecord({ id: 'pool-1' });
    const disk = {
      id: 'd-1',
      name: 'd-1',
      parentId: 'pool-1',
      physicalDisk: { devPath: '/dev/sda', storageRole: 'data', storageGroup: 'other' },
    } as unknown as Resource;

    expect(getStoragePoolLinkedDisks(record, [disk]).map((d) => d.id)).toEqual(['d-1']);
  });

  it('links a disk whose parentId matches refs.resourceId (legacy ref arm)', () => {
    const record = buildRecord({ id: 'pool-1', refs: { resourceId: 'legacy-pool-1' } });
    const disk = {
      id: 'd-1',
      name: 'd-1',
      parentId: 'legacy-pool-1',
      physicalDisk: { devPath: '/dev/sda', storageRole: 'data', storageGroup: 'other' },
    } as unknown as Resource;

    expect(getStoragePoolLinkedDisks(record, [disk]).map((d) => d.id)).toEqual(['d-1']);
  });

  it('falls back to disk.name for devPath and "Unknown" model when physicalDisk is entirely absent', () => {
    const record = buildRecord({ id: 'pool-1' });
    const disk = { id: 'bare-1', name: 'bare-1', parentId: 'pool-1' } as unknown as Resource;

    const [linked] = getStoragePoolLinkedDisks(record, [disk]);

    expect(linked.devPath).toBe('bare-1'); // readDiskDevPath -> '' -> fallback to disk.name
    expect(linked.model).toBe('Unknown'); // readDiskModel default
    expect(linked.temperature).toBe(0);
    expect(linked.sizeLabel).toBe('');
    expect(linked.diskType).toBe('');
    expect(linked.hasIssue).toBe(false);
    expect(linked.errorCount).toBe(0);
    expect(linked.ioLabel).toBe('');
  });

  it('falls back to disk.name when physicalDisk exists but devPath is missing', () => {
    const record = buildRecord({ id: 'pool-1' });
    const disk = {
      id: 'nodev',
      name: 'NodevDisk',
      parentId: 'pool-1',
      physicalDisk: { storageRole: 'data' },
    } as unknown as Resource;

    expect(getStoragePoolLinkedDisks(record, [disk])[0].devPath).toBe('NodevDisk');
  });

  it('breaks role ties by devPath localeCompare', () => {
    const record = buildRecord({ id: 'pool-1' });
    const disks = [
      { id: 'b', name: 'b', parentId: 'pool-1', physicalDisk: { devPath: '/dev/sdd', storageRole: 'data' } },
      { id: 'a', name: 'a', parentId: 'pool-1', physicalDisk: { devPath: '/dev/sdc', storageRole: 'data' } },
    ] as unknown as Resource[];

    expect(getStoragePoolLinkedDisks(record, disks).map((d) => d.devPath)).toEqual(['/dev/sdc', '/dev/sdd']);
  });

  it('ranks roles parity(0) < data(1) < cache(2) < unknown(3)', () => {
    const record = buildRecord({ id: 'pool-1' });
    const disks = [
      { id: 'u', name: 'u', parentId: 'pool-1', physicalDisk: { devPath: '/dev/sdz', storageRole: 'spare' } },
      { id: 'c', name: 'c', parentId: 'pool-1', physicalDisk: { devPath: '/dev/sdy', storageRole: 'cache' } },
      { id: 'p', name: 'p', parentId: 'pool-1', physicalDisk: { devPath: '/dev/sdv', storageRole: 'parity' } },
      { id: 'd', name: 'd', parentId: 'pool-1', physicalDisk: { devPath: '/dev/sdx', storageRole: 'data' } },
    ] as unknown as Resource[];

    expect(getStoragePoolLinkedDisks(record, disks).map((d) => d.role)).toEqual([
      'parity',
      'data',
      'cache',
      'spare',
    ]);
  });

  it('matches an UnRAID cache-pool disk by the trimmed record name used as storage group', () => {
    const record = buildRecord({
      id: 'cache-pool-1',
      name: '  My Cache  ',
      source: unraidSource(),
      details: { type: 'unraid-cache-pool', platform: 'unraid' },
    });
    const disks = [
      { id: 'in', name: 'in', physicalDisk: { devPath: '/dev/sda', storageRole: 'cache', storageGroup: 'My Cache' } },
      { id: 'out', name: 'out', physicalDisk: { devPath: '/dev/sdb', storageRole: 'cache', storageGroup: 'other' } },
    ] as unknown as Resource[];

    expect(getStoragePoolLinkedDisks(record, disks).map((d) => d.id)).toEqual(['in']);
  });

  it('matches an UnRAID array disk via topology "array" even when type is generic', () => {
    const record = buildRecord({
      id: 'arr-1',
      name: 'Tower',
      source: unraidSource(),
      details: { type: 'pool', platform: 'unraid', topology: 'array' },
    });
    const disks = [
      { id: 'in', name: 'in', physicalDisk: { devPath: '/dev/sda', storageRole: 'data', storageGroup: 'unraid-array' } },
      { id: 'out', name: 'out', physicalDisk: { devPath: '/dev/sdb', storageRole: 'data', storageGroup: 'nope' } },
    ] as unknown as Resource[];

    expect(getStoragePoolLinkedDisks(record, disks).map((d) => d.id)).toEqual(['in']);
  });

  it('excludes disks when the UnRAID record resolves to an empty storage group', () => {
    const record = buildRecord({
      id: 'arr-2',
      name: 'Tower',
      source: unraidSource(),
      details: { type: 'pool', platform: 'unraid', topology: '' },
    });
    const disks = [
      { id: 'x', name: 'x', physicalDisk: { devPath: '/dev/sda', storageRole: 'data', storageGroup: 'unraid-array' } },
    ] as unknown as Resource[];

    expect(getStoragePoolLinkedDisks(record, disks)).toEqual([]);
  });
});
