import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  asStorageAlertRecord,
  getStorageRecordAlertResourceIds,
} from '@/features/storageBackups/storageAlertState';

// ---------------------------------------------------------------------------
// Fixture builder — mirrors storageAlertState.test.ts so casts, import paths
// and the default record shape match the sibling suite. The defaults describe
// a healthy truenas pool; cases override `refs` / `details` / `id` / `name`
// to drive individual branches of getStorageRecordAlertResourceIds.
// ---------------------------------------------------------------------------

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01', scope: 'host' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: ['capacity'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

// A minimal well-formed Alert for the object/array pass-through assertions.
const makeAlert = (overrides: Partial<Alert> = {}): Alert => ({
  id: 'alert-1',
  type: 'disk.usage',
  level: 'critical',
  resourceId: 'storage-1',
  resourceName: 'tank',
  node: 'truenas01',
  instance: 'truenas01',
  message: 'pool almost full',
  value: 92,
  threshold: 90,
  startTime: '2024-01-01T00:00:00Z',
  acknowledged: false,
  ...overrides,
});

// ===========================================================================
// asStorageAlertRecord
// ===========================================================================

describe('asStorageAlertRecord branch coverage', () => {
  // ---- Branch L25 `if (!value) return {}` — every falsy arm ----------------
  it('returns {} for null (falsy arm)', () => {
    expect(asStorageAlertRecord(null)).toEqual({});
  });

  it('returns {} for undefined (falsy arm)', () => {
    expect(asStorageAlertRecord(undefined)).toEqual({});
  });

  it('returns {} for the empty string (falsy arm)', () => {
    expect(asStorageAlertRecord('')).toEqual({});
  });

  it('returns {} for 0 (falsy arm)', () => {
    expect(asStorageAlertRecord(0)).toEqual({});
  });

  it('returns {} for false (falsy arm)', () => {
    expect(asStorageAlertRecord(false)).toEqual({});
  });

  // ---- Branch L27 array path: L29 valid element (string id) is included ----
  it('indexes a valid array element by its string id', () => {
    const a = makeAlert({ id: 'a', level: 'warning' });
    const b = makeAlert({ id: 'b', level: 'critical' });
    expect(asStorageAlertRecord([a, b])).toStrictEqual({ a, b });
  });

  // ---- Branch L29 false arm: null element is skipped -----------------------
  it('skips null elements inside an array', () => {
    const a = makeAlert({ id: 'a' });
    expect(asStorageAlertRecord([a, null])).toStrictEqual({ a });
  });

  // ---- Branch L29 false arm: primitive element is skipped ------------------
  it('skips primitive (non-object) elements inside an array', () => {
    const a = makeAlert({ id: 'a' });
    // `alert && typeof alert === 'object'` is false for a string/number.
    expect(asStorageAlertRecord([a, 'not-an-alert', 42, true])).toStrictEqual({ a });
  });

  // ---- Branch L29 false arm: object with no `id` is skipped ----------------
  it('skips object elements that have no id field', () => {
    const a = makeAlert({ id: 'a' });
    expect(asStorageAlertRecord([a, { severity: 'warning' }])).toStrictEqual({ a });
  });

  // ---- Branch L29 false arm: object with a non-string id is skipped --------
  it('skips object elements whose id is not a string', () => {
    const a = makeAlert({ id: 'a' });
    // typeof id === 'string' is false for number 123 → dropped.
    expect(asStorageAlertRecord([a, { id: 123, level: 'warning' }])).toStrictEqual({ a });
  });

  // ---- Branch L29: later element overwrites earlier under the same id ------
  it('lets a later array element overwrite an earlier one sharing the same id', () => {
    const first = makeAlert({ id: 'a', level: 'warning', message: 'old' });
    const second = makeAlert({ id: 'a', level: 'critical', message: 'new' });
    expect(asStorageAlertRecord([first, second])).toStrictEqual({ a: second });
  });

  // ---- Branch L36 `typeof value !== 'object'` true → {} --------------------
  it('returns {} for a truthy non-object primitive (non-empty string)', () => {
    // passes the `!value` guard (truthy) and the Array.isArray guard, then
    // hits `typeof value !== 'object'` → {}.
    expect(asStorageAlertRecord('alerts')).toEqual({});
  });

  it('returns {} for a truthy non-object primitive (positive number)', () => {
    expect(asStorageAlertRecord(7)).toEqual({});
  });

  it('returns {} for the truthy boolean primitive `true`', () => {
    expect(asStorageAlertRecord(true)).toEqual({});
  });

  // ---- Branch L37 object path: returned by reference without validation ----
  it('returns a plain object by reference (no per-value validation on this arm)', () => {
    const input: Record<string, Alert> = { x: makeAlert({ id: 'x' }) };
    // L37 `return value as Record<string, Alert>` hands the same reference back.
    expect(asStorageAlertRecord(input)).toBe(input);
  });

  it('passes an empty object through as {} on the object arm', () => {
    expect(asStorageAlertRecord({})).toEqual({});
  });

  it('returns a multi-key alert object unchanged on the object arm', () => {
    const a = makeAlert({ id: 'a' });
    const b = makeAlert({ id: 'b' });
    const input = { a, b };
    expect(asStorageAlertRecord(input)).toStrictEqual(input);
  });
});

// ===========================================================================
// getStorageRecordAlertResourceIds
// ===========================================================================

describe('getStorageRecordAlertResourceIds branch coverage', () => {
  // ---- Branch L70 `record.refs || {}` right operand: refs undefined → {} ---
  it('does not throw and omits refs-derived ids when refs is undefined', () => {
    // refs undefined → `{}` → resourceId/platformEntityId undefined → filtered.
    // details undefined → `{}` → node undefined → detailNode '' → derived ''.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({ id: 'only-id', refs: undefined, details: undefined }),
      ),
    ).toStrictEqual(['only-id']);
  });

  // ---- Branch L71 `(record.details || {})` right operand: details null -----
  it('treats null details as {} and still returns the record id', () => {
    // `details: null` is an explicitly-malformed input to drive the
    // `(record.details || {})` right operand; cast through unknown for ts.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({ id: 'solo', details: null as unknown as Record<string, unknown> }),
      ),
    ).toStrictEqual(['solo']);
  });

  // ---- Branch L72 `typeof details.node === 'string'` false arm -------------
  it('yields an empty detailNode when details.node is a non-string', () => {
    // node is a number → typeof !== 'string' → detailNode ''. With no
    // detailNode the derivedLegacyId ternary falls to ''.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'solo',
          refs: { platformEntityId: 'cluster-a', resourceId: 'res-1' },
          details: { node: 999 } as Record<string, unknown>,
        }),
      ),
    ).toStrictEqual(['solo', 'res-1']);
  });

  // ---- Branch L72 true arm: details.node string is trimmed -----------------
  it('trims whitespace from details.node when building the derived legacy id', () => {
    // node '  pve1  ' → trim → 'pve1'; platformEntityId 'cluster-a' → 'cluster-a'.
    // derived = `cluster-a-pve1-tank`.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'storage-1',
          name: 'tank',
          refs: { platformEntityId: 'cluster-a' },
          details: { node: '  pve1  ' },
        }),
      ),
    ).toStrictEqual(['storage-1', 'cluster-a-pve1-tank']);
  });

  // ---- Branch L74 `typeof refs.platformEntityId === 'string'` false arm ----
  it('yields an empty detailInstance when refs.platformEntityId is a non-string', () => {
    // platformEntityId is a number → detailInstance '' → derived ''.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'solo',
          refs: { platformEntityId: 418 as unknown as string, resourceId: 'res-1' },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['solo', 'res-1']);
  });

  // ---- Branch L74 true arm: refs.platformEntityId string is trimmed --------
  it('trims whitespace from refs.platformEntityId when building the derived legacy id', () => {
    // platformEntityId '  cluster-a  ' → 'cluster-a'; node 'pve1' → 'pve1'.
    // derived = `cluster-a-pve1-tank`.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'storage-1',
          name: 'tank',
          refs: { platformEntityId: '  cluster-a  ' },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['storage-1', 'cluster-a-pve1-tank']);
  });

  // ---- Branch L76 ternary false arm: missing record.name → derived '' -----
  it('does not synthesize a legacy id when record.name is empty', () => {
    // detailInstance + detailNode truthy but record.name '' → falsy → derived ''.
    // name '' also is dropped by the post-trim length>0 filter downstream is N/A
    // here (name only feeds the template, not the candidate list directly).
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'storage-1',
          name: '',
          refs: { platformEntityId: 'cluster-a', resourceId: 'res-1' },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['storage-1', 'res-1']);
  });

  // ---- Branch L76 ternary false arm: missing detailNode → derived '' -------
  it('does not synthesize a legacy id when details.node is absent', () => {
    // detailInstance truthy, detailNode '' (no node key) → derived ''.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'storage-1',
          name: 'tank',
          refs: { platformEntityId: 'cluster-a', resourceId: 'res-1' },
          details: { other: 'x' },
        }),
      ),
    ).toStrictEqual(['storage-1', 'res-1']);
  });

  // ---- Branch L76 ternary false arm: missing detailInstance → derived '' ---
  it('does not synthesize a legacy id when refs.platformEntityId is absent', () => {
    // detailInstance '' (no platformEntityId) → derived ''.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'storage-1',
          name: 'tank',
          refs: { resourceId: 'res-1' },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['storage-1', 'res-1']);
  });

  // ---- Branch L76 ternary true arm: all three present → derived built ------
  it('synthesizes the legacy id when instance, node and name are all present', () => {
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'storage-1',
          name: 'tank',
          refs: { platformEntityId: 'cluster-a', resourceId: 'res-1' },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['storage-1', 'res-1', 'cluster-a-pve1-tank']);
  });

  // ---- Branch L83 `typeof value === 'string'` filter: non-string dropped ---
  it('drops a non-string refs.resourceId (e.g. number) from the candidate set', () => {
    // resourceId is a number → typeof !== 'string' → filtered out at L83.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'storage-1',
          name: 'tank',
          refs: { resourceId: 418 as unknown as string },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['storage-1']);
  });

  // ---- Branch L85 `value.length > 0` filter: whitespace-only id dropped ----
  it('drops an id that is only whitespace after trimming', () => {
    // id '   ' → trim '' → length 0 → dropped. resourceId 'res-1' survives.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: '   ',
          name: 'tank',
          refs: { resourceId: 'res-1' },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['res-1']);
  });

  // ---- Branch L84 `.map(trim)`: surrounding whitespace is stripped ---------
  it('trims surrounding whitespace from each surviving candidate id', () => {
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: '  storage-1  ',
          name: 'tank',
          refs: { resourceId: '  res-1  ' },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['storage-1', 'res-1']);
  });

  // ---- Branch L81 `new Set(...)`: duplicate ids are de-duplicated ----------
  it('de-duplicates identical candidate ids via the Set', () => {
    // id, resourceId and the derived id all collapse to the same literal so
    // the Set yields exactly one entry.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'dup',
          name: 'ignored',
          refs: { resourceId: 'dup' },
          details: {},
        }),
      ),
    ).toStrictEqual(['dup']);
  });

  // ---- Branch L81 `new Set(...)`: dedup after trim collapses equivalents ---
  it('de-duplicates ids that only differ by surrounding whitespace', () => {
    // 'dup' and '  dup  ' both trim to 'dup' → one Set entry.
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'dup',
          name: 'ignored',
          refs: { resourceId: '  dup  ' },
          details: {},
        }),
      ),
    ).toStrictEqual(['dup']);
  });

  // ---- Combined: empty record id + no refs/details → empty result ----------
  it('returns an empty array when every candidate is empty or non-string', () => {
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: '   ',
          name: '',
          refs: { resourceId: 0 as unknown as string },
          details: undefined,
        }),
      ),
    ).toStrictEqual([]);
  });

  // ---- Ordering preserved: id, resourceId, derivedLegacyId insertion order -
  it('preserves insertion order [id, resourceId, derived] when all are unique', () => {
    expect(
      getStorageRecordAlertResourceIds(
        makeRecord({
          id: 'storage-1',
          name: 'tank',
          refs: { platformEntityId: 'cluster-a', resourceId: 'legacy-storage-id' },
          details: { node: 'pve1' },
        }),
      ),
    ).toStrictEqual(['storage-1', 'legacy-storage-id', 'cluster-a-pve1-tank']);
  });
});
