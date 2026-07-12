import { describe, expect, it } from 'vitest';
import type { SettingsTab } from '../settingsNavigationModel';
import { isTabLocked, tabFeatureRequirements } from '../settingsFeatureGates';

// ---- Helpers ---------------------------------------------------------------
// Mirrors the `hasFeatures` factory used by the sibling
// `settingsNavigation.integration.test.tsx` and `settingsRouting.test.ts` so the
// invocation pattern and predicate semantics stay consistent across the suite.

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

// A spread of real catalog tabs that are intentionally ABSENT from
// `tabFeatureRequirements`; each must resolve to `undefined` on lookup and
// therefore never lock, regardless of feature/runtime state.
const nonGatedTabs: SettingsTab[] = [
  'infrastructure-systems',
  'system-general',
  'system-network',
  'api',
  'support-diagnostics',
  'security-overview',
];

// ---- isTabLocked: tab NOT present in tabFeatureRequirements ---------------
// `tabFeatureRequirements[tab]` yields `undefined`, which is forwarded to
// `isFeatureLocked` and trips the `!features` arm of its first guard
// (`if (!features || features.length === 0) return false;`).

describe('isTabLocked - tab absent from tabFeatureRequirements (lookup -> undefined)', () => {
  it.each(nonGatedTabs)(
    'never locks %s even when runtime is loaded and no features are held',
    (tab) => {
      expect(isTabLocked(tab, hasFeatures([]), () => true)).toBe(false);
    },
  );

  it('never locks a non-gated tab while runtime capabilities are unloaded', () => {
    expect(isTabLocked('system-general', hasFeatures([]), () => false)).toBe(false);
  });

  it('never locks a non-gated tab even when every feature flag is held', () => {
    // Ensures the `undefined` short-circuit wins over a permissive hasFeature.
    expect(
      isTabLocked(
        'api',
        () => true,
        () => true,
      ),
    ).toBe(false);
  });
});

// ---- isTabLocked: tab present, runtime NOT loaded -------------------------
// `isFeatureLocked` second guard: `if (!runtimeCapabilitiesLoaded()) return false;`
// A tab with real requirements stays unlocked while license state is unresolved
// so the UI does not flash lock UI before capabilities are known.

describe('isTabLocked - gated tab while runtime capabilities are unloaded', () => {
  it.each(allGatedTabs)(
    'does not lock %s (requires %s) before runtime capabilities load, even with no features',
    (tab, _required) => {
      expect(isTabLocked(tab, hasFeatures([]), () => false)).toBe(false);
    },
  );

  it('does not lock a gated tab when the required feature is held but runtime is unloaded', () => {
    // runtime-not-loaded guard must fire BEFORE the feature check, so holding the
    // feature must not change the outcome here.
    expect(isTabLocked('system-relay', hasFeatures(['relay']), () => false)).toBe(false);
  });
});

// ---- isTabLocked: tab present, runtime loaded, features all present -------
// `isFeatureLocked` reaches `return !features.every(hasFeature)` with every() -> true,
// yielding `!true === false` (unlocked).

describe('isTabLocked - gated tab unlocked by holding every required feature', () => {
  it.each(allGatedTabs)(
    'does not lock %s when all required features (%s) are present and runtime is loaded',
    (tab, required) => {
      expect(isTabLocked(tab, hasFeatures(required), () => true)).toBe(false);
    },
  );

  it('stays unlocked when the hasFeature predicate grants features broadly', () => {
    // A permissive predicate still satisfies every() -> true -> unlocked.
    expect(
      isTabLocked(
        'organization-overview',
        () => true,
        () => true,
      ),
    ).toBe(false);
  });
});

// ---- isTabLocked: tab present, runtime loaded, a required feature missing --
// `isFeatureLocked` reaches `return !features.every(hasFeature)` with every() -> false,
// yielding `!false === true` (LOCKED). This is the only branch that returns true.

describe('isTabLocked - gated tab locks when a required feature is missing', () => {
  it.each(allGatedTabs)(
    'locks %s when none of the required features (%s) are held and runtime is loaded',
    (tab, _required) => {
      expect(isTabLocked(tab, hasFeatures([]), () => true)).toBe(true);
    },
  );

  it('locks even when an unrelated feature is held (every() -> false on partial match)', () => {
    // system-relay requires ['relay']; granting a different feature must NOT
    // satisfy the every() predicate -> locked.
    expect(isTabLocked('system-relay', hasFeatures(['multi_tenant']), () => true)).toBe(true);
  });

  it('locks when the held feature is a proper superset that omits the required key', () => {
    // security-webhooks requires ['audit_logging']; holding only 'relay' leaves
    // every() -> false -> locked. Pins the single-element every() false arm.
    expect(isTabLocked('security-webhooks', hasFeatures(['relay']), () => true)).toBe(true);
  });

  it('locks an organization tab when multi_tenant is missing despite other org features', () => {
    // organization-billing requires ['multi_tenant']; the presence of unrelated
    // features must not unlock it.
    expect(
      isTabLocked('organization-billing', hasFeatures(['audit_logging', 'relay']), () => true),
    ).toBe(true);
  });
});

// ---- isTabLocked: explicit per-key contract over the whole requirements map
// Pins the exact (tab -> required feature -> lock outcome) mapping so a future
// edit to `tabFeatureRequirements` cannot silently change lock behavior.

describe('isTabLocked - full tabFeatureRequirements contract', () => {
  // Source-of-truth snapshot of the requirements map; if this drifts the test
  // fails loudly, which is the point.
  const expectedMap: Partial<Record<SettingsTab, string[]>> = {
    'system-relay': ['relay'],
    'security-webhooks': ['audit_logging'],
    'organization-overview': ['multi_tenant'],
    'organization-access': ['multi_tenant'],
    'organization-sharing': ['multi_tenant'],
    'organization-billing': ['multi_tenant'],
    'organization-billing-admin': ['multi_tenant'],
  };

  it('tabFeatureRequirements matches the expected snapshot', () => {
    expect(tabFeatureRequirements).toStrictEqual(expectedMap);
  });

  it.each(allGatedTabs)(
    '%s is locked iff its single required feature is absent (runtime loaded)',
    (tab, required) => {
      // LOCKED arm: every() -> false.
      expect(isTabLocked(tab, hasFeatures([]), () => true)).toBe(true);
      // UNLOCKED arm: every() -> true.
      expect(isTabLocked(tab, hasFeatures(required), () => true)).toBe(false);
      // UNLOADED arm: runtime guard returns false regardless of features.
      expect(isTabLocked(tab, hasFeatures(required), () => false)).toBe(false);
    },
  );
});

// ---- isTabLocked: defensive input shape -----------------------------------
// `isTabLocked` accepts any SettingsTab; a deliberately-cast tab that is not in
// the map exercises the `undefined` lookup path through a non-literal input,
// confirming no throw and a concrete `false` result.

describe('isTabLocked - defensive behavior for out-of-map tabs', () => {
  it('returns false for a cast tab id absent from tabFeatureRequirements', () => {
    const unknownTab = 'totally-not-a-real-tab' as unknown as SettingsTab;
    expect(isTabLocked(unknownTab, hasFeatures([]), () => true)).toBe(false);
    // Also confirm the runtime-unloaded path is equally safe.
    expect(isTabLocked(unknownTab, () => true, () => false)).toBe(false);
  });
});
