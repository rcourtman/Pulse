import { describe, expect, it, vi } from 'vitest';
import type { SettingsNavItem, SettingsTab } from '../settingsNavigationModel';
import {
  isSettingsNavItemLocked,
  shouldBlockSettingsRouteItem,
  shouldHideSettingsNavItem,
  type SettingsNavVisibilityContext,
} from '../settingsNavVisibility';

// Branch-coverage complement to `settingsNavVisibility.branchcov2.test.ts`.
// That sibling file already covers every branch that is reachable through REAL
// catalog items for `shouldHideSettingsNavItem` and `isSettingsNavItemLocked`,
// plus the hostedOnly / commercial-hidden-hidden / demo-unresolved /
// requiredCapability / hideWhenUnavailable arms of `shouldBlockSettingsRouteItem`.
//
// This file targets ONLY the branches the sibling file leaves open:
//   1. `shouldBlockSettingsRouteItem` real-catalog arms that the sibling skipped:
//        - hideWhenOrganizationHidden {unresolved, hidesOrganizations} (L127-133)
//        - hideWhenCommercialHidden  {unresolved}                       (L137-139)
//        - hideWhenDemoMode          {isDemoMode}                       (L151-153)
//        - requiredCapability        {resolved + missing}               (L168-172)
//   2. The `hideWhenReadOnly` gate (L87-95 in shouldHide, L156-164 in shouldBlock)
//      which NO real catalog item declares, so it is only reachable by injecting
//      a synthetic item shape through the real catalog module the target imports.
//   3. The `return isTabLocked(...)` -> true outcome of `isSettingsNavItemLocked`
//      (unreachable for real items because every feature-gated catalog item also
//      sets hideWhenUnavailable, short-circuiting at L193).

// Synthetic tab ids. No real SettingsTab literal matches these; cast at the
// type boundary so strict TS stays clean (matches the sibling file's
// UNKNOWN_TAB convention: `'<x>' as unknown as SettingsTab`).
const { READONLY_TAB, LOCKED_TAB } = vi.hoisted(() => ({
  READONLY_TAB: '__branchcov0713_readonly__' as unknown as SettingsTab,
  LOCKED_TAB: '__branchcov0713_locked__' as unknown as SettingsTab,
}));

// Inject synthetic item shapes by wrapping the REAL catalog module the target
// imports. Real tabs delegate to the real implementation unchanged, so every
// real-catalog assertion below exercises the genuine production lookup; only
// the two synthetic ids are overlaid with the gate shape under test.
vi.mock('../settingsNavCatalog', async (importActual) => {
  const actual = await importActual<typeof import('../settingsNavCatalog')>();
  return {
    ...actual,
    getSettingsNavItem: (
      tab: Parameters<typeof actual.getSettingsNavItem>[0],
      locale?: Parameters<typeof actual.getSettingsNavItem>[1],
    ): SettingsNavItem | undefined => {
      if (tab === READONLY_TAB || tab === LOCKED_TAB) {
        // Clone a fully-shaped real item that carries no other visibility
        // gates (system-general) and overlay the synthetic id (+ the gate
        // under test). Cloning a real item keeps the full SettingsNavItem
        // shape (icon/label/etc.) without fabricating any field.
        const base = actual.getSettingsNavItem('system-general', locale);
        if (!base) return undefined;
        if (tab === READONLY_TAB) return { ...base, id: tab, hideWhenReadOnly: true };
        return { ...base, id: tab };
      }
      return actual.getSettingsNavItem(tab, locale);
    },
  };
});

// `isSettingsNavItemLocked` calls `isTabLocked` from the real feature-gates
// module. Real tabs never reach the `return isTabLocked(...)` -> true outcome
// (every feature-gated tab also sets hideWhenUnavailable). For LOCKED_TAB only
// we force isTabLocked -> true so that outcome is exercised; every other tab
// delegates to the real implementation.
vi.mock('../settingsFeatureGates', async (importActual) => {
  const actual = await importActual<typeof import('../settingsFeatureGates')>();
  return {
    ...actual,
    isTabLocked: (
      tab: Parameters<typeof actual.isTabLocked>[0],
      hasFeature: Parameters<typeof actual.isTabLocked>[1],
      runtimeCapabilitiesLoaded: Parameters<typeof actual.isTabLocked>[2],
    ): boolean =>
      tab === LOCKED_TAB ? true : actual.isTabLocked(tab, hasFeature, runtimeCapabilitiesLoaded),
  };
});

// ---- Helpers ---------------------------------------------------------------
// Mirrors the sibling file's `hasFeatures` + `createContext` factories so each
// case only overrides the branch-relevant context field.

const hasFeatures =
  (features: string[]) =>
  (feature: string): boolean =>
    features.includes(feature);

const createContext = (
  overrides: Partial<SettingsNavVisibilityContext> = {},
): SettingsNavVisibilityContext => ({
  hasFeature: hasFeatures([]),
  runtimeCapabilitiesLoaded: () => true,
  hostedModeEnabled: false,
  ...overrides,
});

// ---- shouldHideSettingsNavItem: hideWhenReadOnly gate (L87-95) -------------
// No real catalog item sets `hideWhenReadOnly`; this gate is reachable only via
// the synthetic READONLY_TAB injected above.

describe('shouldHideSettingsNavItem - hideWhenReadOnly gate (L87-95)', () => {
  it('hides a read-only-gated item while presentation policy is unresolved', () => {
    expect(
      shouldHideSettingsNavItem(READONLY_TAB, createContext({ presentationPolicyResolved: false })),
    ).toBe(true);
  });

  it('hides a read-only-gated item when read-only presentation policy is active', () => {
    expect(
      shouldHideSettingsNavItem(
        READONLY_TAB,
        createContext({ presentationPolicyResolved: true, presentationPolicyIsReadOnly: true }),
      ),
    ).toBe(true);
  });

  it('falls through the read-only gate when policy is resolved and not read-only', () => {
    // Resolved + not read-only -> neither inner arm fires -> execution falls
    // past the gate, and with no other gates on the item the final `return
    // false` is reached.
    expect(
      shouldHideSettingsNavItem(
        READONLY_TAB,
        createContext({ presentationPolicyResolved: true, presentationPolicyIsReadOnly: false }),
      ),
    ).toBe(false);
  });
});

// ---- shouldBlockSettingsRouteItem ------------------------------------------
// Real-catalog arms the sibling file did not exercise, then the synthetic
// hideWhenReadOnly gate.

describe('shouldBlockSettingsRouteItem - hideWhenOrganizationHidden gate (L126-134)', () => {
  it('blocks an org-hidden route while presentation policy is unresolved', () => {
    expect(
      shouldBlockSettingsRouteItem(
        'organization-overview',
        createContext({ presentationPolicyResolved: false }),
      ),
    ).toBe(true);
  });

  it('blocks an org-hidden route when organizations are hidden by presentation policy', () => {
    expect(
      shouldBlockSettingsRouteItem(
        'organization-overview',
        createContext({
          presentationPolicyResolved: true,
          presentationPolicyHidesOrganizations: true,
        }),
      ),
    ).toBe(true);
  });
});

describe('shouldBlockSettingsRouteItem - hideWhenCommercialHidden unresolved arm (L136-139)', () => {
  it('blocks a commercial-hidden route while presentation policy is unresolved', () => {
    // system-billing carries only hideWhenCommercialHidden, so an unresolved
    // policy engages this gate before any feature/capability gate is consulted.
    expect(
      shouldBlockSettingsRouteItem(
        'system-billing',
        createContext({ presentationPolicyResolved: false }),
      ),
    ).toBe(true);
  });
});

describe('shouldBlockSettingsRouteItem - hideWhenDemoMode isDemoMode arm (L146-153)', () => {
  it('blocks a demo-hidden route when demo mode is active', () => {
    // The sibling file only covered the unresolved arm for routing; this drives
    // the `presentationPolicyIsDemoMode` inner arm.
    expect(
      shouldBlockSettingsRouteItem(
        'support-diagnostics',
        createContext({ presentationPolicyResolved: true, presentationPolicyIsDemoMode: true }),
      ),
    ).toBe(true);
  });
});

describe('shouldBlockSettingsRouteItem - requiredCapability resolved+missing (L166-172)', () => {
  it('blocks a capability-gated route when resolved and the capability is false', () => {
    expect(
      shouldBlockSettingsRouteItem(
        'api',
        createContext({
          settingsCapabilitiesResolved: true,
          settingsCapabilities: { apiAccessRead: false },
        }),
      ),
    ).toBe(true);
  });

  it('blocks a capability-gated route when resolved but capabilities object is undefined', () => {
    // Optional-chain: context.settingsCapabilities?.[cap] -> undefined ->
    // undefined !== true -> block.
    expect(
      shouldBlockSettingsRouteItem(
        'api',
        createContext({ settingsCapabilitiesResolved: true, settingsCapabilities: undefined }),
      ),
    ).toBe(true);
  });
});

describe('shouldBlockSettingsRouteItem - hideWhenReadOnly gate (L156-164)', () => {
  it('blocks a read-only-gated route while presentation policy is unresolved', () => {
    expect(
      shouldBlockSettingsRouteItem(
        READONLY_TAB,
        createContext({ presentationPolicyResolved: false }),
      ),
    ).toBe(true);
  });

  it('blocks a read-only-gated route when read-only presentation policy is active', () => {
    expect(
      shouldBlockSettingsRouteItem(
        READONLY_TAB,
        createContext({ presentationPolicyResolved: true, presentationPolicyIsReadOnly: true }),
      ),
    ).toBe(true);
  });

  it('does not block a read-only-gated route when resolved and not read-only', () => {
    expect(
      shouldBlockSettingsRouteItem(
        READONLY_TAB,
        createContext({ presentationPolicyResolved: true, presentationPolicyIsReadOnly: false }),
      ),
    ).toBe(false);
  });
});

// ---- isSettingsNavItemLocked ----------------------------------------------
// branchcov2 already covers every reachable branch (unknown tab, hideWhenUnavailable
// early return, no-features fall-through, runtime-capabilities-not-loaded). The
// only remaining outcome is `return isTabLocked(...)` -> true, which is
// unreachable for real catalog items. LOCKED_TAB supplies a non-hideWhenUnavailable
// item while the mocked isTabLocked forces the true result.

describe('isSettingsNavItemLocked - isTabLocked true outcome (L196)', () => {
  it('reports a non-hideWhenUnavailable tab as locked when isTabLocked returns true', () => {
    expect(
      isSettingsNavItemLocked(LOCKED_TAB, createContext({ hasFeature: hasFeatures([]) })),
    ).toBe(true);
  });

  it('does not lock when runtime capabilities are not loaded (isTabLocked short-circuits to false)', () => {
    // Contrast case: real delegation path. LOCKED_TAB forces isTabLocked -> true,
    // but we keep a real-tab sanity check that a non-gated tab stays unlocked
    // regardless of runtime state, confirming the gate is tab-specific.
    expect(
      isSettingsNavItemLocked(
        'system-general',
        createContext({ runtimeCapabilitiesLoaded: () => false }),
      ),
    ).toBe(false);
  });
});
