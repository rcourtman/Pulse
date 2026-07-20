import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getStorageCapabilitiesForResource,
  getStorageCategoryFromType,
  isCanonicalDatastoreStorageResource,
  readResourceStorageMeta,
  type ResourceStorageMeta,
  type StorageClassificationContext,
} from '@/features/storageBackups/resourceStorageMapping';

// ---------------------------------------------------------------------------
// Shared fixtures — mirror the conventions in resourceStorageMapping.test.ts:
// a `makeResource` factory cast as `Resource`, plus a `platformData` record.
// ---------------------------------------------------------------------------

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'storage-1',
    type: 'storage',
    name: 'tank',
    platformType: 'truenas',
    sourceType: 'api',
    ...overrides,
  }) as Resource;

// Sentinel used in platformData.storage so that, when resource.storage is
// rejected by normalizeStorageMeta (returns null), readResourceStorageMeta
// falls through and we can observe the fallback shape — proving the null arm.
const FALLBACK_PLATFORM_STORAGE = { type: 'fallback-marker' } as const;

// Drive the (private) normalizeStorageMeta through the public
// readResourceStorageMeta. resource.storage is the unit under test; the
// sentinel in platformData.storage lets us detect when normalizeStorageMeta
// returned null (fallthrough) vs. returned the normalized direct meta.
const normalizeOf = (input: unknown): ResourceStorageMeta | undefined =>
  readResourceStorageMeta({ ...makeResource(), storage: input } as unknown as Resource, {
    storage: FALLBACK_PLATFORM_STORAGE,
  });

// ===========================================================================
// normalizeStorageMeta  (module-private — exercised via readResourceStorageMeta)
// ===========================================================================

describe('normalizeStorageMeta (via readResourceStorageMeta)', () => {
  // ---- Branch: `!value || typeof value !== 'object'` → return null -------
  it('returns null for null/undefined/non-object inputs (observed via platformData fallthrough)', () => {
    // null  → null
    expect(normalizeOf(null)).toEqual({ type: 'fallback-marker' });
    // undefined → null
    expect(normalizeOf(undefined)).toEqual({ type: 'fallback-marker' });
    // primitive number → null
    expect(normalizeOf(42)).toEqual({ type: 'fallback-marker' });
    // primitive string → null
    expect(normalizeOf('not-an-object')).toEqual({ type: 'fallback-marker' });
    // boolean → null
    expect(normalizeOf(true)).toEqual({ type: 'fallback-marker' });
  });

  it('returns undefined when both direct and platform storage normalize to null', () => {
    // Both inputs non-object → both normalize to null → readResourceStorageMeta
    // resolves to `null || undefined` → undefined (the `nestedMeta || undefined`
    // arm of readResourceStorageMeta, exercised through normalizeStorageMeta).
    expect(
      readResourceStorageMeta({ ...makeResource(), storage: null } as unknown as Resource, {
        storage: 'not-an-object-either',
      }),
    ).toBeUndefined();
  });

  // ---- Branch: each `typeof candidate.X === 'string' ? X : undefined` ----
  it('preserves every string-valued field and returns the full normalized shape', () => {
    const normalized = normalizeOf({
      type: 'rbd',
      platform: 'proxmox-pve',
      topology: 'pool',
      content: 'images',
      protection: 'protected',
      arrayState: 'online',
      syncAction: 'syncing',
      path: '/dev/sda',
    });

    expect(normalized).toEqual({
      type: 'rbd',
      platform: 'proxmox-pve',
      topology: 'pool',
      content: 'images',
      protection: 'protected',
      arrayState: 'online',
      syncAction: 'syncing',
      path: '/dev/sda',
      contentTypes: undefined,
      shared: undefined,
      syncProgress: undefined,
      numProtected: undefined,
      numDisabled: undefined,
      numInvalid: undefined,
      numMissing: undefined,
      isCeph: undefined,
      isZfs: undefined,
      zfsPool: undefined,
    });
  });

  it('coerces wrong-typed string fields to undefined (defensive ternary false arm)', () => {
    const normalized = normalizeOf({
      type: 123,
      platform: 456,
      topology: true,
      content: { x: 1 },
      protection: ['no'],
      arrayState: 7,
      syncAction: 8,
      path: 9,
    });

    // Each defensive `typeof X === 'string'` is false → undefined.
    expect(normalized?.type).toBeUndefined();
    expect(normalized?.platform).toBeUndefined();
    expect(normalized?.topology).toBeUndefined();
    expect(normalized?.content).toBeUndefined();
    expect(normalized?.protection).toBeUndefined();
    expect(normalized?.arrayState).toBeUndefined();
    expect(normalized?.syncAction).toBeUndefined();
    expect(normalized?.path).toBeUndefined();
  });

  // ---- Branch: each `typeof candidate.X === 'boolean' ? X : undefined` ---
  it('preserves boolean fields and coerces non-booleans to undefined', () => {
    expect(normalizeOf({ shared: true, isCeph: false, isZfs: true })?.shared).toBe(true);
    expect(normalizeOf({ shared: true, isCeph: false, isZfs: true })?.isCeph).toBe(false);
    expect(normalizeOf({ shared: true, isCeph: false, isZfs: true })?.isZfs).toBe(true);

    const wrongBool = normalizeOf({ shared: 'yes', isCeph: 1, isZfs: null });
    expect(wrongBool?.shared).toBeUndefined();
    expect(wrongBool?.isCeph).toBeUndefined();
    expect(wrongBool?.isZfs).toBeUndefined();
  });

  // ---- Branch: each `typeof candidate.X === 'number' ? X : undefined` ----
  it('preserves numeric fields and coerces non-numbers to undefined', () => {
    const ok = normalizeOf({
      syncProgress: 50,
      numProtected: 3,
      numDisabled: 1,
      numInvalid: 2,
      numMissing: 0,
    });
    expect(ok?.syncProgress).toBe(50);
    expect(ok?.numProtected).toBe(3);
    expect(ok?.numDisabled).toBe(1);
    expect(ok?.numInvalid).toBe(2);
    expect(ok?.numMissing).toBe(0);

    const wrong = normalizeOf({
      syncProgress: '50',
      numProtected: true,
      numDisabled: '1',
      numInvalid: null,
      numMissing: undefined,
    });
    expect(wrong?.syncProgress).toBeUndefined();
    expect(wrong?.numProtected).toBeUndefined();
    expect(wrong?.numDisabled).toBeUndefined();
    expect(wrong?.numInvalid).toBeUndefined();
    expect(wrong?.numMissing).toBeUndefined();
  });

  // ---- Branch: `Array.isArray(candidate.contentTypes) ? filter : undefined`
  it('returns contentTypes undefined when the value is not an array', () => {
    expect(normalizeOf({ contentTypes: 'not-array' })?.contentTypes).toBeUndefined();
    expect(normalizeOf({ contentTypes: { 0: 'a' } })?.contentTypes).toBeUndefined();
    expect(normalizeOf({ contentTypes: 42 })?.contentTypes).toBeUndefined();
    expect(normalizeOf({})?.contentTypes).toBeUndefined();
  });

  it('keeps only non-empty trimmed strings from contentTypes and drops everything else', () => {
    // The filter keeps `typeof item === 'string' && item.trim().length > 0`.
    // Mixed: valid string, empty string, whitespace-only, numbers, null,
    // booleans, object — only the genuinely-populated strings survive.
    expect(
      normalizeOf({ contentTypes: ['images', '', '   ', 42, null, false, {}, 'rootdir'] })
        ?.contentTypes,
    ).toEqual(['images', 'rootdir']);

    // An array of only rejectable items collapses to an empty array
    // (NOT undefined — the array branch was taken).
    expect(normalizeOf({ contentTypes: ['', '  ', 1, null] })?.contentTypes).toEqual([]);
  });

  // ---- Branch: `candidate.zfsPool && typeof ... === 'object' ? cast : undef`
  it('returns zfsPool as-is when it is an object, undefined otherwise', () => {
    const pool = { name: 'tank', state: 'ONLINE' };
    expect(normalizeOf({ zfsPool: pool })?.zfsPool).toEqual(pool);

    // null → falsy short-circuit → undefined
    expect(normalizeOf({ zfsPool: null })?.zfsPool).toBeUndefined();
    // primitive → typeof !== 'object' → undefined
    expect(normalizeOf({ zfsPool: 'tank' })?.zfsPool).toBeUndefined();
    expect(normalizeOf({ zfsPool: 42 })?.zfsPool).toBeUndefined();
  });
});

// ===========================================================================
// isCanonicalDatastoreStorageResource
// ===========================================================================

describe('isCanonicalDatastoreStorageResource', () => {
  // ---- Branch: isBackupRepositoryStorageResource(...) === true → false ----
  it('returns false when the resource is classified as a backup repository', () => {
    expect(isCanonicalDatastoreStorageResource('pbs')).toBe(false);
    // via context.resourceType
    expect(
      isCanonicalDatastoreStorageResource('whatever', undefined, { resourceType: 'pbs' }),
    ).toBe(false);
    // via context.platform (substring match)
    expect(
      isCanonicalDatastoreStorageResource('whatever', undefined, { platform: 'proxmox-pbs' }),
    ).toBe(false);
    // via context.topology exact match
    expect(
      isCanonicalDatastoreStorageResource('whatever', undefined, { topology: 'backup-target' }),
    ).toBe(false);
  });

  // ---- Branch: resourceType === 'datastore' → true ----------------------
  it('returns true when context.resourceType is datastore', () => {
    expect(
      isCanonicalDatastoreStorageResource('mystery', undefined, { resourceType: 'datastore' }),
    ).toBe(true);
  });

  // ---- Branch: topology === 'datastore' (context OR storageMeta) --------
  it('returns true when topology is datastore from context or storageMeta', () => {
    expect(
      isCanonicalDatastoreStorageResource('mystery', undefined, { topology: 'datastore' }),
    ).toBe(true);
    expect(isCanonicalDatastoreStorageResource('mystery', { topology: 'datastore' })).toBe(true);
  });

  // ---- Branch: entityType === 'datastore' → true -----------------------
  it('returns true when context.entityType is datastore', () => {
    expect(
      isCanonicalDatastoreStorageResource('mystery', undefined, { entityType: 'datastore' }),
    ).toBe(true);
  });

  // ---- Branch: (platform.includes('vmware') && value.length > 0) -------
  it('returns true for vmware platform with a non-empty type, false when type is empty', () => {
    // context.platform path
    expect(
      isCanonicalDatastoreStorageResource('vmfs', undefined, { platform: 'vmware-vsphere' }),
    ).toBe(true);
    // storageMeta.platform path
    expect(isCanonicalDatastoreStorageResource('vmfs', { platform: 'vmware-vsphere' })).toBe(true);
    // value.length === 0 → false even with vmware platform
    expect(isCanonicalDatastoreStorageResource('', undefined, { platform: 'vmware-vsphere' })).toBe(
      false,
    );
    expect(isCanonicalDatastoreStorageResource('', { platform: 'vmware-vsphere' })).toBe(false);
  });

  // ---- Branch: `context?.platform || storageMeta?.platform` fallback ----
  it('uses context.platform when set, falling back to storageMeta.platform only when context is absent', () => {
    // context.platform truthy + non-vmware wins; storageMeta.platform ignored.
    expect(
      isCanonicalDatastoreStorageResource(
        'vmfs',
        { platform: 'vmware-vsphere' },
        { platform: 'proxmox-pve' },
      ),
    ).toBe(false);
    // context.platform undefined → fallback to storageMeta.platform.
    expect(isCanonicalDatastoreStorageResource('vmfs', { platform: 'vmware-vsphere' }, {})).toBe(
      true,
    );
  });

  // ---- Branch: fall-through → false ------------------------------------
  it('returns false when nothing matches', () => {
    expect(isCanonicalDatastoreStorageResource('mystery')).toBe(false);
    expect(isCanonicalDatastoreStorageResource(undefined)).toBe(false);
    expect(isCanonicalDatastoreStorageResource('mystery', undefined, {})).toBe(false);
  });
});

// ===========================================================================
// getStorageCategoryFromType
// ===========================================================================

describe('getStorageCategoryFromType', () => {
  // ---- Branch: `!value` → 'other' --------------------------------------
  it('returns "other" for empty or undefined type', () => {
    expect(getStorageCategoryFromType(undefined)).toBe('other');
    expect(getStorageCategoryFromType('')).toBe('other');
  });

  // ---- Branch: isBackupRepositoryStorageResource(type, undefined, ctx) --
  it('returns "backup-repository" via any backup signal in context', () => {
    // value.includes('pbs')
    expect(getStorageCategoryFromType('pbs')).toBe('backup-repository');
    // context.resourceType === 'pbs'
    expect(getStorageCategoryFromType('mystery', { resourceType: 'pbs' })).toBe(
      'backup-repository',
    );
    // context.platform includes 'pbs'
    expect(getStorageCategoryFromType('mystery', { platform: 'proxmox-pbs' })).toBe(
      'backup-repository',
    );
    // context.topology === 'backup-target'
    expect(getStorageCategoryFromType('mystery', { topology: 'backup-target' })).toBe(
      'backup-repository',
    );
  });

  // ---- Branch: isCanonicalDatastoreStorageResource(type, undefined, ctx)
  it('returns "datastore" via each canonical signal in context', () => {
    expect(getStorageCategoryFromType('mystery', { resourceType: 'datastore' })).toBe('datastore');
    expect(getStorageCategoryFromType('mystery', { topology: 'datastore' })).toBe('datastore');
    expect(getStorageCategoryFromType('mystery', { entityType: 'datastore' })).toBe('datastore');
    // platform.includes('vmware') && value.length > 0
    expect(getStorageCategoryFromType('vmfs', { platform: 'vmware-vsphere' })).toBe('datastore');
  });

  // ---- Branch: pool substring detection (zfs | lvm | ceph | pool) ------
  it('returns "pool" for each pool-indicating substring', () => {
    expect(getStorageCategoryFromType('myzfs')).toBe('pool');
    expect(getStorageCategoryFromType('mylvm')).toBe('pool');
    expect(getStorageCategoryFromType('myceph')).toBe('pool');
    expect(getStorageCategoryFromType('mypool')).toBe('pool');
  });

  // ---- Branch: dataset / share / filesystem substrings -----------------
  it('returns the right category for dataset, share, and filesystem substrings', () => {
    expect(getStorageCategoryFromType('mydataset')).toBe('dataset');
    expect(getStorageCategoryFromType('mynfs')).toBe('share');
    expect(getStorageCategoryFromType('mycifs')).toBe('share');
    expect(getStorageCategoryFromType('mysmb')).toBe('share');
    expect(getStorageCategoryFromType('mydir')).toBe('filesystem');
    expect(getStorageCategoryFromType('myfilesystem')).toBe('filesystem');
  });

  // ---- Branch: ordering — pool beats dataset when both substrings match
  it('prefers "pool" over "dataset" when the type contains both substrings', () => {
    // 'pooldataset' triggers the pool branch first.
    expect(getStorageCategoryFromType('pooldataset')).toBe('pool');
  });

  // ---- Branch: fallback → 'other' --------------------------------------
  it('returns "other" for an unrecognized type with no matching context', () => {
    expect(getStorageCategoryFromType('mystery')).toBe('other');
    expect(getStorageCategoryFromType('mystery', {} as StorageClassificationContext)).toBe('other');
  });
});

// ===========================================================================
// getStorageCapabilitiesForResource
// ===========================================================================

describe('getStorageCapabilitiesForResource', () => {
  // ---- Branch: base caps always present --------------------------------
  it('returns only the base caps when no signal matches', () => {
    expect(getStorageCapabilitiesForResource('mystery')).toEqual(['capacity', 'health']);
    expect(getStorageCapabilitiesForResource(undefined)).toEqual(['capacity', 'health']);
  });

  // ---- Branch: isBackupRepositoryStorageResource(...) === true ----------
  it('adds backup-repository caps for a PBS-classified resource', () => {
    expect(getStorageCapabilitiesForResource('pbs')).toEqual([
      'capacity',
      'health',
      'backup-repository',
      'deduplication',
      'namespaces',
    ]);
    // Same caps via context signal rather than the type substring.
    expect(
      getStorageCapabilitiesForResource('mystery', undefined, { resourceType: 'pbs' }),
    ).toEqual(['capacity', 'health', 'backup-repository', 'deduplication', 'namespaces']);
  });

  // ---- Branch: `storageMeta?.isZfs || value.includes('zfs')` ------------
  it('adds snapshots + compression when isZfs flag is set OR the type contains zfs', () => {
    // value.includes('zfs') arm
    expect(getStorageCapabilitiesForResource('zfsmirror')).toEqual([
      'capacity',
      'health',
      'snapshots',
      'compression',
    ]);
    // storageMeta.isZfs arm
    expect(getStorageCapabilitiesForResource('mystery', { isZfs: true })).toEqual([
      'capacity',
      'health',
      'snapshots',
      'compression',
    ]);
  });

  // ---- Branch: `storageMeta?.isCeph || value.includes('ceph')` ----------
  it('adds replication + multi-node when isCeph flag is set OR the type contains ceph', () => {
    // value.includes('ceph') arm
    expect(getStorageCapabilitiesForResource('cephrbd')).toEqual([
      'capacity',
      'health',
      'replication',
      'multi-node',
    ]);
    // storageMeta.isCeph arm
    expect(getStorageCapabilitiesForResource('mystery', { isCeph: true })).toEqual([
      'capacity',
      'health',
      'replication',
      'multi-node',
    ]);
  });

  // ---- Branch: `(context?.shared ?? storageMeta?.shared) === true` ------
  it('adds multi-node only when shared resolves to true via context or storageMeta', () => {
    // context.shared true
    expect(getStorageCapabilitiesForResource('mystery', undefined, { shared: true })).toEqual([
      'capacity',
      'health',
      'multi-node',
    ]);
    // context.shared undefined → ?? fallback to storageMeta.shared true
    expect(getStorageCapabilitiesForResource('mystery', { shared: true }, {})).toEqual([
      'capacity',
      'health',
      'multi-node',
    ]);
    // storageMeta only
    expect(getStorageCapabilitiesForResource('mystery', { shared: true })).toEqual([
      'capacity',
      'health',
      'multi-node',
    ]);
    // context.shared === false is NOT nullish, so ?? does NOT fall back to
    // storageMeta.shared — multi-node must NOT be added.
    expect(
      getStorageCapabilitiesForResource('mystery', { shared: true }, { shared: false }),
    ).toEqual(['capacity', 'health']);
    // shared === false explicitly → no multi-node
    expect(getStorageCapabilitiesForResource('mystery', undefined, { shared: false })).toEqual([
      'capacity',
      'health',
    ]);
  });

  // ---- Branch: dedupe collapses duplicate multi-node -------------------
  it('deduplicates multi-node when both ceph and shared branches add it', () => {
    const caps = getStorageCapabilitiesForResource('ceph', undefined, { shared: true });
    // ceph branch pushes 'multi-node'; shared branch pushes it again; the
    // Set-based dedupe must leave exactly one occurrence.
    expect(caps).toEqual(['capacity', 'health', 'replication', 'multi-node']);
    expect(caps.filter((c) => c === 'multi-node')).toHaveLength(1);
  });

  it('combines backup-repository caps with shared multi-node without duplicates', () => {
    expect(getStorageCapabilitiesForResource('pbs', undefined, { shared: true })).toEqual([
      'capacity',
      'health',
      'backup-repository',
      'deduplication',
      'namespaces',
      'multi-node',
    ]);
  });
});
