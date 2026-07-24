import { describe, expect, it } from 'vitest';
import type { ClusterEndpoint } from '@/types/nodes';
import {
  applyClusterNodeDisplayNamesLocally,
  buildClusterNodeDisplayNameOverridesPayload,
} from '../nodeModalModel';

// Same fixture-builder shape as the sibling nodeModalModel.test.ts suite, but
// parameterised over the two optional fields the display-name helpers branch on
// (`nodeIdentity` + `displayName`) so each uncovered arm can be driven in
// isolation. `omitIdentity: true` yields a member whose nodeIdentity is absent
// (undefined), exercising the `!nodeIdentity` guards.
const endpoint = (
  nodeName: string,
  fields: { nodeIdentity?: string; displayName?: string; omitIdentity?: boolean } = {},
): ClusterEndpoint => ({
  nodeId: `node/${nodeName}`,
  nodeIdentity: fields.omitIdentity ? undefined : (fields.nodeIdentity ?? `cluster-${nodeName}`),
  nodeName,
  host: `https://${nodeName}.local:8006`,
  ip: '10.0.0.1',
  online: true,
  lastSeen: '',
  ...(fields.displayName !== undefined ? { displayName: fields.displayName } : {}),
});

// ---- buildClusterNodeDisplayNameOverridesPayload ----------------------------
//
// The sibling suites only feed this helper non-empty endpoints whose identities
// are all present in the form map and whose values all differ from the saved
// names. The tests below drive the remaining arms:
//   * `if (!endpoints?.length) return undefined` — empty + undefined endpoints.
//   * `if (!nodeIdentity) return []` — a member with an absent identity.
//   * `if (value === undefined || value.trim() === (endpoint.displayName ?? ''))`
//     — both the `value === undefined` short-circuit (identity absent from the
//     form map) and the equality-true arm (form value trims to the saved name,
//     including the `(undefined ?? '')` fallback for a member with no saved
//     display name).
//   * `changed.length > 0 ? changed : undefined` — the falsy (`: undefined`)
//     arm reached when every member is filtered out.

describe('buildClusterNodeDisplayNameOverridesPayload', () => {
  it('returns undefined for empty or undefined endpoints (early return)', () => {
    expect(
      buildClusterNodeDisplayNameOverridesPayload([], { 'cluster-pve1': 'Compute' }),
    ).toBeUndefined();
    expect(
      buildClusterNodeDisplayNameOverridesPayload(undefined, { 'cluster-pve1': 'Compute' }),
    ).toBeUndefined();
  });

  it('drops members that have no nodeIdentity while still emitting changed siblings', () => {
    const endpoints = [
      endpoint('pve1', { omitIdentity: true, displayName: 'Lonely' }),
      endpoint('pve2', { displayName: 'Old' }),
    ];
    const payload = buildClusterNodeDisplayNameOverridesPayload(endpoints, {
      // Present but unreachable: pve1 has no identity, so this entry is never
      // matched against it.
      'cluster-pve1': 'Whatever',
      'cluster-pve2': 'New',
    });
    expect(payload).toEqual([{ nodeIdentity: 'cluster-pve2', displayName: 'New' }]);
  });

  it('skips members whose identity has no form entry (value === undefined short-circuit)', () => {
    const endpoints = [
      endpoint('pve1', { displayName: 'Alpha' }), // 'cluster-pve1' absent from the map
      endpoint('pve2', { displayName: 'Beta' }), // 'cluster-pve2' present + changed
    ];
    const payload = buildClusterNodeDisplayNameOverridesPayload(endpoints, {
      'cluster-pve2': 'Beta2',
    });
    expect(payload).toEqual([{ nodeIdentity: 'cluster-pve2', displayName: 'Beta2' }]);
  });

  it('treats a form value that trims to the saved display name as unchanged', () => {
    const payload = buildClusterNodeDisplayNameOverridesPayload(
      [endpoint('pve1', { displayName: 'Compute' })],
      { 'cluster-pve1': '  Compute  ' },
    );
    expect(payload).toBeUndefined();
  });

  it('treats an empty form value as unchanged when no display name is saved (?? "" fallback)', () => {
    // saved displayName is absent -> (undefined ?? '') === ''; form '' trims to
    // '' which equals it, so the member is filtered and the result collapses to
    // undefined via the `changed.length > 0 ? changed : undefined` falsy arm.
    const payload = buildClusterNodeDisplayNameOverridesPayload([endpoint('pve1')], {
      'cluster-pve1': '',
    });
    expect(payload).toBeUndefined();
  });
});

// ---- applyClusterNodeDisplayNamesLocally ------------------------------------
//
// Remaining arms:
//   * `if (!endpoints?.length || !overrides?.length) return endpoints` — every
//     falsy operand (empty/undefined endpoints, empty/undefined overrides),
//     asserting the SAME reference is handed back untouched.
//   * `if (!endpoint.nodeIdentity) return endpoint` — a member with an absent
//     identity is returned by reference even when an override targets the
//     identity string it would have had.
//   * `const value = byIdentity.get(...); if (value === undefined) return endpoint`
//     — no override targets this member's identity.
//   * `displayName: value || undefined` — the falsy arm where an empty-string
//     override clears the saved display name.

describe('applyClusterNodeDisplayNamesLocally', () => {
  it('returns the input by reference when there is nothing to apply', () => {
    const endpoints = [endpoint('pve1')];
    // overrides empty or undefined -> endpoints handed back untouched
    expect(applyClusterNodeDisplayNamesLocally(endpoints, [])).toBe(endpoints);
    expect(applyClusterNodeDisplayNamesLocally(endpoints, undefined)).toBe(endpoints);
    // endpoints empty or undefined -> early return of that same value
    expect(
      applyClusterNodeDisplayNamesLocally([], [{ nodeIdentity: 'cluster-pve1', displayName: 'X' }]),
    ).toEqual([]);
    expect(
      applyClusterNodeDisplayNamesLocally(undefined, [
        { nodeIdentity: 'cluster-pve1', displayName: 'X' },
      ]),
    ).toBeUndefined();
  });

  it('leaves members without a nodeIdentity untouched (returns the same endpoint)', () => {
    const endpoints = [
      endpoint('pve1', { omitIdentity: true, displayName: 'Kept' }),
      endpoint('pve2', { displayName: 'Old' }),
    ];
    const result = applyClusterNodeDisplayNamesLocally(endpoints, [
      { nodeIdentity: 'cluster-pve1', displayName: 'ShouldNotApply' },
      { nodeIdentity: 'cluster-pve2', displayName: 'New' },
    ]);
    // pve1 short-circuits on `!nodeIdentity`: same object, display name intact.
    expect(result?.[0]).toBe(endpoints[0]);
    expect(result?.[0].displayName).toBe('Kept');
    // pve2 is matched and updated.
    expect(result?.[1].displayName).toBe('New');
  });

  it('leaves a member untouched when no override targets its identity', () => {
    const endpoints = [endpoint('pve1', { displayName: 'Original' })];
    const result = applyClusterNodeDisplayNamesLocally(endpoints, [
      { nodeIdentity: 'cluster-somebody-else', displayName: 'Ignored' },
    ]);
    expect(result?.[0]).toBe(endpoints[0]);
    expect(result?.[0].displayName).toBe('Original');
  });

  it('clears the display name when the override value is an empty string (value || undefined)', () => {
    const endpoints = [endpoint('pve1', { displayName: 'OldName' })];
    const result = applyClusterNodeDisplayNamesLocally(endpoints, [
      { nodeIdentity: 'cluster-pve1', displayName: '' },
    ]);
    expect(result?.[0].displayName).toBeUndefined();
    // a fresh object is produced (spread), not the original reference
    expect(result?.[0]).not.toBe(endpoints[0]);
    expect(result?.[0].nodeName).toBe('pve1');
  });
});
