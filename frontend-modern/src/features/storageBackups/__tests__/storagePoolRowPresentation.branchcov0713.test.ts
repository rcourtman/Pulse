import { describe, expect, it } from 'vitest';
import type { StorageCapacityDeltaPresentation } from '@/features/storageBackups/storageCapacityDeltaPresentation';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildStoragePoolRowModel,
  STORAGE_POOL_ROW_PLACEHOLDER_CLASS,
} from '@/features/storageBackups/storagePoolRowPresentation';

// Branch-coverage extension for `buildStoragePoolRowModel`.
//
// The sibling `storagePoolRowPresentation.test.ts` already pins the happy path
// (truthy capacity bytes, non-null capacityDelta, delegate-field wiring). This
// file targets ONLY the local branches inside `buildStoragePoolRowModel` that
// the happy path leaves unexecuted:
//
//   totalBytes = record.capacity.totalBytes || 0      // `|| 0` falsy arm
//   usedBytes  = record.capacity.usedBytes  || 0      // `|| 0` falsy arm
//   freeBytes  = record.capacity.freeBytes
//                  ?? (totalBytes > 0                 // ternary true/false arms
//                        ? Math.max(totalBytes - usedBytes, 0)  // Math.max arms
//                        : 0)
//   capacityDeltaLabel:     capacityDelta?.label     ?? '—'
//   capacityDeltaTitle:     capacityDelta?.title     ?? 'No used-capacity change...'
//   capacityDeltaToneClass: capacityDelta?.toneClass ?? STORAGE_POOL_ROW_PLACEHOLDER_CLASS
//
// Minimal, fully-valid base record. The delegate helpers produce deterministic
// values for this base (zfsPool=null, hostLabel=location.label, topologyLabel=
// category, stateLabel='Available', etc.); those delegate branches are owned by
// rowPresentation / recordPresentation and are not re-tested here. We assert
// only the capacity-coercion and capacityDelta-defaulting outputs.

const baseRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-1',
    name: 'tank',
    source: {
      platform: 'proxmox-pbs',
      family: 'onprem',
      origin: 'resource',
      adapterId: 'pbs-1',
    },
    category: 'pool',
    health: 'healthy',
    location: { label: 'pbs01', scope: 'host' },
    capacity: { totalBytes: 1000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
    capabilities: [],
    observedAt: 0,
    refs: {},
    ...overrides,
  }) as StorageRecord;

// ---------------------------------------------------------------------------
// capacity bytes coercion: `record.capacity.totalBytes || 0` and `usedBytes || 0`
// ---------------------------------------------------------------------------

describe('buildStoragePoolRowModel branch coverage — capacity byte coercion', () => {
  it('coerces null capacity bytes to 0 via the `|| 0` falsy arms', () => {
    const record = baseRecord({
      capacity: { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: null },
    });
    const model = buildStoragePoolRowModel(record);
    expect(model.totalBytes).toBe(0);
    expect(model.usedBytes).toBe(0);
    // freeBytes null -> `??` right arm; totalBytes(coerced 0) > 0 is false ->
    // ternary false arm -> literal 0 (NOT Math.max(0-0, 0)).
    expect(model.freeBytes).toBe(0);
  });

  it('coerces zero capacity bytes to 0 via the `|| 0` falsy arms (0 is falsy)', () => {
    // totalBytes: 0 is falsy, so `0 || 0` -> 0. Same for usedBytes. This is a
    // distinct input from null but exercises the same `|| 0` branch; pinning it
    // documents that the boundary value 0 is treated identically to null.
    const record = baseRecord({
      capacity: { totalBytes: 0, usedBytes: 0, freeBytes: 0, usagePercent: 0 },
    });
    const model = buildStoragePoolRowModel(record);
    expect(model.totalBytes).toBe(0);
    expect(model.usedBytes).toBe(0);
    // freeBytes 0 is NOT null/undefined, so the `??` LEFT arm fires and returns
    // 0 verbatim (NOT the derived total-used path).
    expect(model.freeBytes).toBe(0);
  });

  it('preserves a truthy usedBytes while totalBytes is null (asymmetric coercion)', () => {
    const record = baseRecord({
      capacity: { totalBytes: null, usedBytes: 500, freeBytes: null, usagePercent: null },
    });
    const model = buildStoragePoolRowModel(record);
    expect(model.totalBytes).toBe(0); // null -> 0
    expect(model.usedBytes).toBe(500); // truthy, preserved
    // totalBytes(coerced 0) > 0 false -> ternary false arm -> 0.
    expect(model.freeBytes).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// freeBytes derivation: `freeBytes ?? (totalBytes > 0 ? Math.max(...) : 0)`
// ---------------------------------------------------------------------------

describe('buildStoragePoolRowModel branch coverage — freeBytes derivation', () => {
  it('uses the explicit freeBytes when present (`??` left arm), ignoring total-used', () => {
    const record = baseRecord({
      capacity: { totalBytes: 1000, usedBytes: 400, freeBytes: 500, usagePercent: 40 },
    });
    const model = buildStoragePoolRowModel(record);
    // total-used would be 600, but the explicit 500 wins via the `??` left arm.
    expect(model.freeBytes).toBe(500);
    expect(model.totalBytes).toBe(1000);
    expect(model.usedBytes).toBe(400);
  });

  it('derives freeBytes from total-used when freeBytes is null and the diff is positive (Math.max first-arg arm)', () => {
    const record = baseRecord({
      capacity: { totalBytes: 1000, usedBytes: 250, freeBytes: null, usagePercent: 25 },
    });
    const model = buildStoragePoolRowModel(record);
    // totalBytes 1000 > 0 -> ternary true arm -> Math.max(1000-250, 0) = 750.
    expect(model.freeBytes).toBe(750);
  });

  it('clamps freeBytes to 0 when usedBytes exceeds totalBytes (Math.max second-arg arm)', () => {
    const record = baseRecord({
      capacity: { totalBytes: 100, usedBytes: 250, freeBytes: null, usagePercent: null },
    });
    const model = buildStoragePoolRowModel(record);
    // totalBytes 100 > 0 -> ternary true arm -> Math.max(100-250, 0) = Math.max(-150, 0) = 0.
    expect(model.freeBytes).toBe(0);
    expect(model.totalBytes).toBe(100);
    expect(model.usedBytes).toBe(250);
  });

  it('returns 0 freeBytes at the diff===0 boundary (usedBytes === totalBytes, freeBytes null)', () => {
    const record = baseRecord({
      capacity: { totalBytes: 1000, usedBytes: 1000, freeBytes: null, usagePercent: 100 },
    });
    const model = buildStoragePoolRowModel(record);
    // Math.max(0, 0) = 0; both args equal, first-arg arm returns 0.
    expect(model.freeBytes).toBe(0);
  });

  it('short-circuits to 0 via the `totalBytes > 0` false arm when totalBytes coerces from null, even if usedBytes is huge', () => {
    // totalBytes null -> coerced 0 -> `0 > 0` false -> ternary false arm -> 0.
    // Pinning that the guard prevents entering the Math.max clamp path at all,
    // which matters because Math.max(0-10000, 0) would also yield 0 — the OUTPUT
    // is identical but the BRANCH is distinct.
    const record = baseRecord({
      capacity: { totalBytes: null, usedBytes: 10_000, freeBytes: null, usagePercent: null },
    });
    const model = buildStoragePoolRowModel(record);
    expect(model.freeBytes).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// capacityDelta defaulting: `capacityDelta?.X ?? <default>`
// ---------------------------------------------------------------------------

describe('buildStoragePoolRowModel branch coverage — capacityDelta default branches', () => {
  it('applies the default capacityDelta fields when capacityDelta is explicitly null', () => {
    const model = buildStoragePoolRowModel(baseRecord(), null);
    expect(model.capacityDeltaLabel).toBe('—');
    expect(model.capacityDeltaTitle).toBe('No used-capacity change history available.');
    expect(model.capacityDeltaToneClass).toBe(STORAGE_POOL_ROW_PLACEHOLDER_CLASS);
  });

  it('applies the default capacityDelta fields when capacityDelta is omitted (default parameter)', () => {
    const model = buildStoragePoolRowModel(baseRecord());
    expect(model.capacityDeltaLabel).toBe('—');
    expect(model.capacityDeltaTitle).toBe('No used-capacity change history available.');
    expect(model.capacityDeltaToneClass).toBe(STORAGE_POOL_ROW_PLACEHOLDER_CLASS);
  });

  it('treats an explicitly-passed undefined capacityDelta as the default null (default parameter fires)', () => {
    // The signature `capacityDelta: StorageCapacityDeltaPresentation | null = null`
    // makes the parameter optional, so `undefined` activates the `= null` default.
    const model = buildStoragePoolRowModel(baseRecord(), undefined);
    expect(model.capacityDeltaLabel).toBe('—');
    expect(model.capacityDeltaTitle).toBe('No used-capacity change history available.');
    expect(model.capacityDeltaToneClass).toBe(STORAGE_POOL_ROW_PLACEHOLDER_CLASS);
  });

  it('passes through a shrinking capacityDelta (negative deltaBytes) label/title/toneClass', () => {
    const delta: StorageCapacityDeltaPresentation = {
      deltaBytes: -20 * 1024 * 1024 * 1024,
      label: '-20.00 GB',
      title: 'Used capacity shrank by 20.00 GB over 24h.',
      toneClass: 'text-emerald-600 dark:text-emerald-300',
    };
    const model = buildStoragePoolRowModel(baseRecord(), delta);
    expect(model.capacityDeltaLabel).toBe('-20.00 GB');
    expect(model.capacityDeltaTitle).toBe('Used capacity shrank by 20.00 GB over 24h.');
    expect(model.capacityDeltaToneClass).toBe('text-emerald-600 dark:text-emerald-300');
  });

  it('returns empty-string label/title/toneClass verbatim (`??` only defaults on null/undefined, not on "")', () => {
    // StorageCapacityDeltaPresentation declares these fields as `string`, so ''
    // is a fully-typed value — no cast needed. The `??` operator must NOT
    // substitute the defaults for empty strings.
    const delta: StorageCapacityDeltaPresentation = {
      deltaBytes: 0,
      label: '',
      title: '',
      toneClass: '',
    };
    const model = buildStoragePoolRowModel(baseRecord(), delta);
    expect(model.capacityDeltaLabel).toBe('');
    expect(model.capacityDeltaTitle).toBe('');
    expect(model.capacityDeltaToneClass).toBe('');
  });
});
