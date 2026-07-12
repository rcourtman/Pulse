import { describe, expect, it } from 'vitest';
import type { ZFSDevice } from '@/types/api';
import { getZfsDeviceBlockClass } from '@/features/storageBackups/zfsPresentation';

// ---------------------------------------------------------------------------
// Fixture builder — mirror zfsPresentation.test.ts: a minimal `ZFSDevice` cast.
// The only field getZfsDeviceBlockClass reads is `state`, so the factory only
// needs a state override. Each test still passes a fully-typed ZFSDevice so the
// pre-push tsconfig stays happy without `as any` on the call site.
// ---------------------------------------------------------------------------

const makeDevice = (overrides: Partial<ZFSDevice> = {}): ZFSDevice =>
  ({
    name: 'da0',
    type: 'disk',
    state: 'ONLINE',
    readErrors: 0,
    writeErrors: 0,
    checksumErrors: 0,
    ...overrides,
  }) as ZFSDevice;

// ===========================================================================
// getZfsDeviceBlockClass
//
// Source branches:
//   L4  `device.state?.toUpperCase()`   — optional-chain + normalization
//   L5  `state === 'ONLINE'`            — true / false
//   L6  `state === 'DEGRADED'`          — true / false
//   L7  `state === 'FAULTED' || state === 'UNAVAIL' || state === 'OFFLINE'`
//                                        — three OR arms
//   L10 default fall-through             — slate
// ===========================================================================

describe('getZfsDeviceBlockClass branch coverage', () => {
  // ---- L5 true arm: ONLINE → exact green block string ---------------------
  it('returns the exact ONLINE green block class with hover variant', () => {
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'ONLINE' }))).toBe(
      'bg-green-500 dark:bg-green-500 hover:bg-green-400',
    );
  });

  // ---- L5 false → L6 true arm: DEGRADED → exact yellow block string ------
  it('returns the exact DEGRADED yellow block class with hover variant', () => {
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'DEGRADED' }))).toBe(
      'bg-yellow-500 dark:bg-yellow-500 hover:bg-yellow-400',
    );
  });

  // ---- L7 first OR arm: FAULTED → exact red block string ------------------
  it('returns the exact red block class for FAULTED (first OR arm)', () => {
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'FAULTED' }))).toBe(
      'bg-red-500 dark:bg-red-500 hover:bg-red-400',
    );
  });

  // ---- L7 second OR arm: UNAVAIL → exact red block string -----------------
  it('returns the exact red block class for UNAVAIL (second OR arm)', () => {
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'UNAVAIL' }))).toBe(
      'bg-red-500 dark:bg-red-500 hover:bg-red-400',
    );
  });

  // ---- L7 third OR arm: OFFLINE → exact red block string ------------------
  it('returns the exact red block class for OFFLINE (third OR arm)', () => {
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'OFFLINE' }))).toBe(
      'bg-red-500 dark:bg-red-500 hover:bg-red-400',
    );
  });

  // ---- L10 default arm: unrecognized state → slate ------------------------
  it('falls through to the slate default for an unrecognized state', () => {
    // 'REMOVED' is listed in the ZFSDevice comment but NOT handled by the
    // function → every conditional is false → final return.
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'REMOVED' }))).toBe(
      'bg-slate-400 hover:bg-slate-300',
    );
    // Any other arbitrary string also falls through.
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'SUSPENDED' }))).toBe(
      'bg-slate-400 hover:bg-slate-300',
    );
  });

  // ---- L4 toUpperCase() normalization: lowercase/mixedcase states match ---
  it('normalizes state via toUpperCase so lowercase and mixed-case forms match', () => {
    // lowercase 'online' → 'ONLINE' → green (proves .toUpperCase() runs).
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'online' }))).toBe(
      'bg-green-500 dark:bg-green-500 hover:bg-green-400',
    );
    // mixed-case 'Degraded' → 'DEGRADED' → yellow.
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'Degraded' }))).toBe(
      'bg-yellow-500 dark:bg-yellow-500 hover:bg-yellow-400',
    );
    // lowercase 'faulted' → 'FAULTED' → red.
    expect(getZfsDeviceBlockClass(makeDevice({ state: 'faulted' }))).toBe(
      'bg-red-500 dark:bg-red-500 hover:bg-red-400',
    );
  });

  // ---- L4 optional-chain `state?.` arm: state undefined → slate default ---
  it('exercises the state?. optional-chain when state is undefined and falls to slate', () => {
    // ZFSDevice.state is typed as required string, so we must cast to bypass
    // strict null checks. `state?.toUpperCase()` short-circuits to undefined
    // → no conditional matches → default slate string.
    const noState = { ...makeDevice(), state: undefined } as unknown as ZFSDevice;
    expect(getZfsDeviceBlockClass(noState)).toBe('bg-slate-400 hover:bg-slate-300');
  });

  // ---- L4 defensive: state null → slate default ---------------------------
  it('exercises the state?. optional-chain when state is null and falls to slate', () => {
    // Same defensive optional-chain arm reached via null (the pre-push hook
    // runs the real tsconfig, so the null value is injected through an
    // `as unknown as` cast).
    const nullState = { ...makeDevice(), state: null } as unknown as ZFSDevice;
    expect(getZfsDeviceBlockClass(nullState)).toBe('bg-slate-400 hover:bg-slate-300');
  });
});
