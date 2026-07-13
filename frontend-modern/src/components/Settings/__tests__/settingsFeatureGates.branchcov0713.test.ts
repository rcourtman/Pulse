/**
 * Branch-coverage tests for the still-uncovered surface of settingsFeatureGates:
 *   - isFeatureLocked  (exercised DIRECTLY; the sibling branchcov0712c suite only
 *                       reaches it indirectly through isTabLocked)
 *   - getTabLockReason (not covered by any sibling test)
 *   - isTabLocked      (only the predicate short-circuit / invocation-ordering
 *                       aspect, which branchcov0712c does not assert; the return-
 *                       value matrix over every tab is already pinned there and
 *                       is intentionally NOT duplicated here)
 *
 * Every guard arm, the `every()` true/false outcomes, guard ORDERING (runtime
 * before features), and the exact tier-specific reason strings are pinned with
 * concrete inputs and assertions. No tautologies: reason strings are checked
 * against a hand-written per-tab snapshot, not by re-calling the helper.
 */
import { describe, expect, it } from 'vitest';

import type { SettingsTab } from '../settingsNavigationModel';
import { getTabLockReason, isFeatureLocked, isTabLocked, tabFeatureRequirements } from '../settingsFeatureGates';

// ---- Helpers ---------------------------------------------------------------
// Mirrors the `hasFeatures` factory used by the sibling branchcov0712c test so
// the invocation pattern and predicate semantics stay consistent across suites.

const hasFeatures =
  (features: string[]) =>
  (feature: string): boolean =>
    features.includes(feature);

// The complete set of tabs that carry feature requirements in
// `tabFeatureRequirements`. Driving every entry guarantees the
// `tabFeatureRequirements[tab]` lookup -> defined-array arm is exercised for
// each key, not just a representative sample.
const allGatedTabs = Object.entries(tabFeatureRequirements) as Array<
  [SettingsTab, string[]]
>;

/**
 * Wraps `hasFeatures` with a call-sink so tests can assert guard ORDERING —
 * specifically that `hasFeature` is NOT invoked once an earlier guard
 * (empty/unloaded tab) has already resolved the result. This is the branch
 * behavior the sibling suite does not check.
 */
const trackingHas =
  (granted: string[], sink: { calls: string[] }) =>
  (feature: string): boolean => {
    sink.calls.push(feature);
    return granted.includes(feature);
  };

// Hand-written snapshot of the exact reason string each gated tab must emit
// when its required feature is missing. Derived from reading
// `getFeatureMinTierLabel` (relay -> 'Relay', multi_tenant -> 'MSP',
// audit_logging -> 'Pro' fallback); NOT computed by calling the helper, so a
// change to either the tier map or the message template fails this loudly.
const EXPECTED_LOCK_REASON: Partial<Record<SettingsTab, string>> = {
  'system-relay': 'This settings section requires Relay.',
  'security-webhooks': 'This settings section requires Pro.',
  'organization-overview': 'This settings section requires MSP.',
  'organization-access': 'This settings section requires MSP.',
  'organization-sharing': 'This settings section requires MSP.',
  'organization-billing': 'This settings section requires MSP.',
  'organization-billing-admin': 'This settings section requires MSP.',
};

// ---- isFeatureLocked (direct) ---------------------------------------------
// `isFeatureLocked` is the shared engine behind `isTabLocked`. The sibling
// suite only reaches it indirectly, so its three guards and the `every()`
// true/false outcomes are pinned here by calling it directly.

describe('isFeatureLocked - direct branch coverage', () => {
  // Guard 1, `!features` arm: `features` is `undefined`.
  it('returns false when features is undefined (first guard: !features)', () => {
    expect(isFeatureLocked(undefined, () => false, () => true)).toBe(false);
  });

  // Guard 1, `!features` arm: deliberately-malformed `null` input violates the
  // declared `string[] | undefined` type; cast through `unknown` to keep strict
  // mode clean. `!null` is true so the function must short-circuit to false.
  it('returns false when features is null (cast; !features covers nullish)', () => {
    const features = null as unknown as Parameters<typeof isFeatureLocked>[0];
    expect(isFeatureLocked(features, () => false, () => true)).toBe(false);
  });

  // Guard 1, `features.length === 0` arm.
  it('returns false when features is an empty array (length === 0 arm)', () => {
    expect(isFeatureLocked([], () => false, () => true)).toBe(false);
  });

  // Guard 2, `!runtimeCapabilitiesLoaded()` arm: populated features but the
  // license/capability state has not resolved yet -> must not lock.
  it('returns false when features are present but runtime is not loaded', () => {
    expect(isFeatureLocked(['relay'], () => false, () => false)).toBe(false);
  });

  // Guard 3, `every()` -> true arm: `!true === false` (unlocked).
  it('returns false when every required feature is held and runtime is loaded', () => {
    expect(
      isFeatureLocked(['relay', 'multi_tenant'], hasFeatures(['relay', 'multi_tenant']), () => true),
    ).toBe(false);
  });

  // Guard 3, `every()` -> false arm: `!false === true` (LOCKED). This is the
  // only branch that returns true.
  it('returns true when the first required feature is missing (every -> false)', () => {
    expect(isFeatureLocked(['relay', 'multi_tenant'], hasFeatures(['multi_tenant']), () => true)).toBe(
      true,
    );
  });

  it('returns true when only the first of several required features is held', () => {
    // every() -> false on a later element.
    expect(
      isFeatureLocked(
        ['relay', 'multi_tenant', 'audit_logging'],
        hasFeatures(['relay']),
        () => true,
      ),
    ).toBe(true);
  });

  it('returns true when a later required feature is missing while earlier ones are held', () => {
    // Pins that every() is not satisfied by a proper prefix.
    expect(
      isFeatureLocked(
        ['relay', 'multi_tenant', 'audit_logging'],
        hasFeatures(['relay', 'multi_tenant']),
        () => true,
      ),
    ).toBe(true);
  });

  // ---- short-circuit / guard ORDERING --------------------------------------
  it('does not invoke hasFeature when features is empty', () => {
    const sink = { calls: [] as string[] };
    isFeatureLocked([], trackingHas([], sink), () => true);
    expect(sink.calls).toEqual([]);
  });

  it('does not invoke hasFeature when runtime capabilities are unloaded (runtime guard before feature check)', () => {
    const sink = { calls: [] as string[] };
    const result = isFeatureLocked(['relay'], trackingHas(['relay'], sink), () => false);
    expect(result).toBe(false);
    expect(sink.calls).toEqual([]);
  });

  it('invokes hasFeature exactly once per required feature when runtime is loaded and all are held', () => {
    const sink = { calls: [] as string[] };
    isFeatureLocked(['a', 'b', 'c'], trackingHas(['a', 'b', 'c'], sink), () => true);
    expect(sink.calls).toEqual(['a', 'b', 'c']);
  });
});

// ---- isTabLocked - complementary short-circuit coverage --------------------
// branchcov0712c already pins the full (tab x feature x runtime) return-value
// matrix. This block adds the one aspect it does not assert: that `hasFeature`
// is bypassed entirely once an earlier guard resolves the result.

describe('isTabLocked - predicate short-circuit (complements branchcov0712c)', () => {
  it('does not invoke hasFeature for a non-gated tab (undefined lookup short-circuits before the feature check)', () => {
    const sink = { calls: [] as string[] };
    const result = isTabLocked('api', trackingHas([], sink), () => true);
    expect(result).toBe(false);
    expect(sink.calls).toEqual([]);
  });

  it('does not invoke hasFeature for a gated tab while runtime capabilities are unloaded', () => {
    const sink = { calls: [] as string[] };
    const result = isTabLocked('system-relay', trackingHas(['relay'], sink), () => false);
    expect(result).toBe(false);
    expect(sink.calls).toEqual([]);
  });

  it('invokes hasFeature for a gated tab exactly once its single required feature is reached (runtime loaded)', () => {
    const sink = { calls: [] as string[] };
    isTabLocked('system-relay', trackingHas(['relay'], sink), () => true);
    expect(sink.calls).toEqual(['relay']);
  });
});

// ---- getTabLockReason ------------------------------------------------------
// Not covered by any sibling test. Pins every guard, the `every()` unlocked
// arm, and the exact tier-specific reason string for the locked arm.

describe('getTabLockReason - branch coverage', () => {
  // Guard 1: tab NOT present in tabFeatureRequirements -> `undefined` -> null.
  it.each(
    [
      'infrastructure-systems',
      'system-general',
      'system-network',
      'api',
      'support-diagnostics',
      'security-overview',
      'security-audit',
    ] as SettingsTab[],
  )('returns null for non-gated tab %s (lookup -> undefined)', (tab) => {
    expect(getTabLockReason(tab, hasFeatures([]), () => true)).toBeNull();
  });

  // Guard 1 via a deliberately-cast out-of-map id (no throw, concrete null).
  it('returns null for a cast tab id absent from tabFeatureRequirements', () => {
    const unknownTab = 'totally-not-a-real-tab' as unknown as SettingsTab;
    expect(getTabLockReason(unknownTab, hasFeatures([]), () => true)).toBeNull();
  });

  // Guard 2: `!runtimeCapabilitiesLoaded()` -> null for every gated tab.
  it.each(allGatedTabs)(
    'returns null for gated tab %s while runtime capabilities are unloaded',
    (tab) => {
      expect(getTabLockReason(tab, hasFeatures([]), () => false)).toBeNull();
    },
  );

  // Guard 3: `requiredFeatures.every(hasFeature)` -> true -> null (unlocked).
  it.each(allGatedTabs)(
    'returns null for gated tab %s when every required feature is held',
    (tab, required) => {
      expect(getTabLockReason(tab, hasFeatures(required), () => true)).toBeNull();
    },
  );

  // Locked arm: emits the tier-specific reason string. The expected snapshot is
  // hand-written (see EXPECTED_LOCK_REASON) so this fails if either the tier
  // label mapping or the message template drifts.
  it.each(allGatedTabs)(
    'emits the exact tier-specific reason for %s when its required feature is missing',
    (tab) => {
      expect(getTabLockReason(tab, hasFeatures([]), () => true)).toBe(EXPECTED_LOCK_REASON[tab]);
    },
  );

  // ---- short-circuit / guard ORDERING --------------------------------------
  it('does not invoke hasFeature for a non-gated tab', () => {
    const sink = { calls: [] as string[] };
    getTabLockReason('api', trackingHas([], sink), () => true);
    expect(sink.calls).toEqual([]);
  });

  it('does not invoke hasFeature for a gated tab while runtime is unloaded (runtime guard before feature check)', () => {
    const sink = { calls: [] as string[] };
    getTabLockReason('system-relay', trackingHas([], sink), () => false);
    expect(sink.calls).toEqual([]);
  });

  it('invokes hasFeature exactly once for a single-requirement gated tab when runtime is loaded and the feature is held', () => {
    const sink = { calls: [] as string[] };
    getTabLockReason('system-relay', trackingHas(['relay'], sink), () => true);
    expect(sink.calls).toEqual(['relay']);
  });
});

// ---- cross-function consistency --------------------------------------------
// Pins that `getTabLockReason` and `isTabLocked` agree on every gated tab and
// every (runtime x feature) combination: a reason exists iff the tab is locked.

describe('getTabLockReason agrees with isTabLocked for every gated tab', () => {
  it.each(allGatedTabs)(
    '%s: reason is non-null iff isTabLocked is true across loaded/held x loaded/missing x unloaded',
    (tab, required) => {
      // loaded + missing -> LOCKED, reason present.
      expect(isTabLocked(tab, hasFeatures([]), () => true)).toBe(true);
      expect(getTabLockReason(tab, hasFeatures([]), () => true)).not.toBeNull();

      // loaded + held -> UNLOCKED, no reason.
      expect(isTabLocked(tab, hasFeatures(required), () => true)).toBe(false);
      expect(getTabLockReason(tab, hasFeatures(required), () => true)).toBeNull();

      // unloaded -> UNLOCKED regardless of features, no reason.
      expect(isTabLocked(tab, hasFeatures([]), () => false)).toBe(false);
      expect(getTabLockReason(tab, hasFeatures([]), () => false)).toBeNull();
    },
  );
});
