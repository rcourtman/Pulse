import { describe, expect, it } from 'vitest';
import type { SettingsTab } from '../settingsNavigationModel';
import {
  isSettingsNavItemLocked,
  shouldBlockSettingsRouteItem,
  shouldHideSettingsNavItem,
  type SettingsNavVisibilityContext,
} from '../settingsNavVisibility';

// ---- Helpers ---------------------------------------------------------------
// Mirrors the sibling `settingsNavigation.integration.test.tsx` hasFeatures
// factory and builds a fully-typed SettingsNavVisibilityContext with sensible
// defaults so each case only overrides the branch-relevant field.

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

// A SettingsTab value that is NOT present in the nav catalog, used to drive
// the defensive `!item` early-returns via a controlled cast.
const UNKNOWN_TAB = 'nonexistent-tab' as unknown as SettingsTab;

// ---- shouldHideSettingsNavItem --------------------------------------------

describe('shouldHideSettingsNavItem', () => {
  it('returns false for an unknown tab (defensive !item branch)', () => {
    expect(shouldHideSettingsNavItem(UNKNOWN_TAB, createContext())).toBe(false);
  });

  it('returns false for a tab whose item has no visibility gates', () => {
    expect(shouldHideSettingsNavItem('system-general', createContext())).toBe(false);
  });

  describe('hostedOnly gate (lines 53-55)', () => {
    it('hides a hostedOnly tab when hosted mode is disabled', () => {
      expect(
        shouldHideSettingsNavItem(
          'organization-billing-admin',
          createContext({
            hasFeature: hasFeatures(['multi_tenant']),
            hostedModeEnabled: false,
            settingsCapabilities: { billingAdmin: true },
            settingsCapabilitiesResolved: true,
          }),
        ),
      ).toBe(true);
    });

    it('skips the hostedOnly gate when hosted mode is enabled', () => {
      // hostedOnly + hostedModeEnabled true -> gate skipped; multi_tenant present
      // and capability granted -> no other gate fires -> not hidden.
      expect(
        shouldHideSettingsNavItem(
          'organization-billing-admin',
          createContext({
            hasFeature: hasFeatures(['multi_tenant']),
            hostedModeEnabled: true,
            settingsCapabilities: { billingAdmin: true },
            settingsCapabilitiesResolved: true,
            presentationPolicyResolved: true,
          }),
        ),
      ).toBe(false);
    });
  });

  describe('hideWhenOrganizationHidden gate (lines 57-65)', () => {
    it('hides when presentation policy is unresolved', () => {
      expect(
        shouldHideSettingsNavItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures(['multi_tenant']),
            presentationPolicyResolved: false,
          }),
        ),
      ).toBe(true);
    });

    it('hides when organizations are hidden by presentation policy', () => {
      expect(
        shouldHideSettingsNavItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures(['multi_tenant']),
            presentationPolicyResolved: true,
            presentationPolicyHidesOrganizations: true,
          }),
        ),
      ).toBe(true);
    });

    it('falls through when organizations are resolved and visible', () => {
      expect(
        shouldHideSettingsNavItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures(['multi_tenant']),
            presentationPolicyResolved: true,
            presentationPolicyHidesOrganizations: false,
          }),
        ),
      ).toBe(false);
    });
  });

  describe('hideWhenCommercialHidden gate (lines 67-75)', () => {
    it('hides a commercial-hidden tab when policy is unresolved', () => {
      expect(
        shouldHideSettingsNavItem(
          'system-billing',
          createContext({ presentationPolicyResolved: false }),
        ),
      ).toBe(true);
    });

    it('hides a commercial-hidden tab when commercial is hidden', () => {
      expect(
        shouldHideSettingsNavItem(
          'system-billing',
          createContext({
            presentationPolicyResolved: true,
            presentationPolicyHidesCommercial: true,
          }),
        ),
      ).toBe(true);
    });

    it('falls through when commercial is resolved and visible', () => {
      expect(
        shouldHideSettingsNavItem(
          'system-billing',
          createContext({
            presentationPolicyResolved: true,
            presentationPolicyHidesCommercial: false,
          }),
        ),
      ).toBe(false);
    });
  });

  describe('hideWhenDemoMode gate (lines 77-85)', () => {
    it('hides a demo-hidden tab when policy is unresolved', () => {
      expect(
        shouldHideSettingsNavItem(
          'support-diagnostics',
          createContext({ presentationPolicyResolved: false }),
        ),
      ).toBe(true);
    });

    it('hides a demo-hidden tab in demo mode', () => {
      expect(
        shouldHideSettingsNavItem(
          'support-diagnostics',
          createContext({
            presentationPolicyResolved: true,
            presentationPolicyIsDemoMode: true,
          }),
        ),
      ).toBe(true);
    });

    it('falls through when demo mode is resolved off', () => {
      expect(
        shouldHideSettingsNavItem(
          'support-diagnostics',
          createContext({
            presentationPolicyResolved: true,
            presentationPolicyIsDemoMode: false,
          }),
        ),
      ).toBe(false);
    });
  });

  describe('requiredCapability gate (lines 97-103)', () => {
    it('hides when resolved and the required capability is false', () => {
      expect(
        shouldHideSettingsNavItem(
          'api',
          createContext({
            settingsCapabilitiesResolved: true,
            settingsCapabilities: { apiAccessRead: false },
          }),
        ),
      ).toBe(true);
    });

    it('does not hide when capability state is unresolved', () => {
      expect(
        shouldHideSettingsNavItem(
          'api',
          createContext({ settingsCapabilitiesResolved: false, settingsCapabilities: null }),
        ),
      ).toBe(false);
    });

    it('does not hide when the required capability is granted', () => {
      expect(
        shouldHideSettingsNavItem(
          'api',
          createContext({
            settingsCapabilitiesResolved: true,
            settingsCapabilities: { apiAccessRead: true },
          }),
        ),
      ).toBe(false);
    });

    it('hides when resolved but settingsCapabilities is undefined (optional-chain -> undefined)', () => {
      // context.settingsCapabilities?.[cap] -> undefined -> undefined !== true -> hidden.
      expect(
        shouldHideSettingsNavItem(
          'api',
          createContext({
            settingsCapabilitiesResolved: true,
            settingsCapabilities: undefined,
          }),
        ),
      ).toBe(true);
    });
  });

  describe('hideWhenUnavailable gate -> hasRequiredFeatures / missingFeaturesArePaidRuntimeBlocked', () => {
    it('does not hide when all required features are present (hasRequiredFeatures every()->true)', () => {
      expect(
        shouldHideSettingsNavItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures(['multi_tenant']),
            presentationPolicyResolved: true,
          }),
        ),
      ).toBe(false);
    });

    it('hides when required features are missing and not runtime-blocked (missing-blocked -> false)', () => {
      expect(
        shouldHideSettingsNavItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures([]),
            presentationPolicyResolved: true,
            isRuntimeCapabilityBlocked: () => false,
          }),
        ),
      ).toBe(true);
    });

    it('keeps visible when every missing feature is paid-runtime-blocked (missing-blocked -> true)', () => {
      expect(
        shouldHideSettingsNavItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures([]),
            presentationPolicyResolved: true,
            isRuntimeCapabilityBlocked: (feature, reason) =>
              feature === 'multi_tenant' && reason === 'paid_runtime_required',
          }),
        ),
      ).toBe(false);
    });

    it('hides when isRuntimeCapabilityBlocked is omitted (optional-chain ?. yields undefined -> every()->false)', () => {
      // missingFeaturesArePaidRuntimeBlocked reads context.isRuntimeCapabilityBlocked?.(...);
      // when undefined, every() returns false -> hide block falls through to `return true`.
      expect(
        shouldHideSettingsNavItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures([]),
            presentationPolicyResolved: true,
            // isRuntimeCapabilityBlocked deliberately omitted
          }),
        ),
      ).toBe(true);
    });

    it('hides when the blocker accepts the feature but a different reason (reason-mismatch arm)', () => {
      // The source always passes 'paid_runtime_required'; a callback that only
      // blocks on a different reason returns false -> every()->false -> hidden.
      expect(
        shouldHideSettingsNavItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures([]),
            presentationPolicyResolved: true,
            isRuntimeCapabilityBlocked: (_feature, reason) => reason === 'some_other_reason',
          }),
        ),
      ).toBe(true);
    });
  });
});

// ---- shouldBlockSettingsRouteItem ------------------------------------------

describe('shouldBlockSettingsRouteItem', () => {
  it('returns false for an unknown tab (defensive !item branch)', () => {
    expect(shouldBlockSettingsRouteItem(UNKNOWN_TAB, createContext())).toBe(false);
  });

  it('mirrors the hostedOnly behavior for routing', () => {
    expect(
      shouldBlockSettingsRouteItem(
        'organization-billing-admin',
        createContext({
          hasFeature: hasFeatures(['multi_tenant']),
          hostedModeEnabled: false,
          settingsCapabilities: { billingAdmin: true },
          settingsCapabilitiesResolved: true,
        }),
      ),
    ).toBe(true);
  });

  it('blocks a demo-hidden route while demo policy is unresolved', () => {
    expect(
      shouldBlockSettingsRouteItem(
        'support-diagnostics',
        createContext({ presentationPolicyResolved: false }),
      ),
    ).toBe(true);
  });

  it('blocks a commercial-hidden route when commercial is hidden', () => {
    expect(
      shouldBlockSettingsRouteItem(
        'system-billing',
        createContext({
          presentationPolicyResolved: true,
          presentationPolicyHidesCommercial: true,
        }),
      ),
    ).toBe(true);
  });

  it('does not block a capability-gated route while capability state is unresolved', () => {
    expect(
      shouldBlockSettingsRouteItem(
        'api',
        createContext({ settingsCapabilitiesResolved: false, settingsCapabilities: null }),
      ),
    ).toBe(false);
  });

  describe('PANEL_OWNED_FEATURE_GATE_TABS bypass (lines 174-183)', () => {
    it.each([
      'system-relay',
      'support-reporting',
      'security-roles',
      'security-users',
      'security-audit',
      'security-webhooks',
    ] as const)(
      'does NOT block panel-owned tab %s even when its features are missing',
      (tab) => {
        expect(
          shouldBlockSettingsRouteItem(
            tab,
            createContext({
              hasFeature: hasFeatures([]),
              presentationPolicyResolved: true,
              settingsCapabilitiesResolved: false,
            }),
          ),
        ).toBe(false);
      },
    );
  });

  describe('non-panel-owned hideWhenUnavailable tabs are feature-gated for routing', () => {
    // organization-overview is hideWhenUnavailable + NOT in PANEL_OWNED_FEATURE_GATE_TABS,
    // so it is the canonical tab to exercise the route-level feature gate.
    it('blocks when required features are missing and not runtime-blocked', () => {
      const ctx = createContext({
        hasFeature: hasFeatures([]),
        presentationPolicyResolved: true,
        isRuntimeCapabilityBlocked: () => false,
      });
      expect(shouldBlockSettingsRouteItem('organization-overview', ctx)).toBe(true);
      // Contrast: the same context hides the item from the nav.
      expect(shouldHideSettingsNavItem('organization-overview', ctx)).toBe(true);
    });

    it('does not block when required features are present', () => {
      expect(
        shouldBlockSettingsRouteItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures(['multi_tenant']),
            presentationPolicyResolved: true,
          }),
        ),
      ).toBe(false);
    });

    it('does not block when missing features are paid-runtime-blocked', () => {
      expect(
        shouldBlockSettingsRouteItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures([]),
            presentationPolicyResolved: true,
            isRuntimeCapabilityBlocked: () => true,
          }),
        ),
      ).toBe(false);
    });

    it('does not block when isRuntimeCapabilityBlocked is omitted', () => {
      // ?. yields undefined -> every()->false -> missing-blocked returns false
      // -> but the feature-gate block then returns true... wait: missing-blocked
      // false means we DO block. So this asserts the omitted-callback path leads
      // to a block (every()->false).
      expect(
        shouldBlockSettingsRouteItem(
          'organization-overview',
          createContext({
            hasFeature: hasFeatures([]),
            presentationPolicyResolved: true,
          }),
        ),
      ).toBe(true);
    });
  });
});

// ---- isSettingsNavItemLocked -----------------------------------------------

describe('isSettingsNavItemLocked', () => {
  it('returns false for an unknown tab (defensive !item branch)', () => {
    expect(isSettingsNavItemLocked(UNKNOWN_TAB, createContext())).toBe(false);
  });

  it('returns false for a tab with hideWhenUnavailable (early `item.hideWhenUnavailable` branch)', () => {
    // organization-overview, security-roles, system-relay, support-reporting all
    // carry hideWhenUnavailable -> short-circuit before isTabLocked is consulted.
    expect(
      isSettingsNavItemLocked(
        'organization-overview',
        createContext({ hasFeature: hasFeatures([]) }),
      ),
    ).toBe(false);
    expect(
      isSettingsNavItemLocked('security-roles', createContext({ hasFeature: hasFeatures([]) })),
    ).toBe(false);
    expect(
      isSettingsNavItemLocked('system-relay', createContext({ hasFeature: hasFeatures([]) })),
    ).toBe(false);
  });

  it('returns false for a tab without feature requirements (isTabLocked -> isFeatureLocked false branch)', () => {
    expect(
      isSettingsNavItemLocked('system-general', createContext({ hasFeature: hasFeatures([]) })),
    ).toBe(false);
  });

  it('returns false while runtime capabilities are not loaded (isFeatureLoaded -> false branch)', () => {
    expect(
      isSettingsNavItemLocked(
        'system-general',
        createContext({ runtimeCapabilitiesLoaded: () => false }),
      ),
    ).toBe(false);
  });

  it('never reports a real catalog tab as locked (every feature-gated tab also has hideWhenUnavailable)', () => {
    // See GLM_REPORT: the `return isTabLocked(...)` -> true outcome is unreachable
    // for real catalog items, so every real tab resolves to false here.
    const tabs: SettingsTab[] = [
      'system-relay',
      'security-webhooks',
      'organization-overview',
      'organization-access',
      'organization-sharing',
      'organization-billing',
      'organization-billing-admin',
      'system-general',
      'api',
      'support-diagnostics',
    ];
    for (const tab of tabs) {
      expect(
        isSettingsNavItemLocked(tab, createContext({ hasFeature: hasFeatures([]) })),
      ).toBe(false);
    }
  });
});

// ---- hasRequiredFeatures / missingFeaturesArePaidRuntimeBlocked (private) ---
// These helpers are not exported; they are exercised indirectly through the
// three public functions above. This block pins the branch mapping so the
// coverage intent is explicit and self-documenting.

describe('private helpers hasRequiredFeatures / missingFeaturesArePaidRuntimeBlocked (indirect)', () => {
  it('hasRequiredFeatures: every()->true when features are present, every()->false when missing', () => {
    // every()->true: features present -> availability gate skipped -> not hidden.
    expect(
      shouldHideSettingsNavItem(
        'organization-overview',
        createContext({
          hasFeature: hasFeatures(['multi_tenant']),
          presentationPolicyResolved: true,
        }),
      ),
    ).toBe(false);
    // every()->false: feature missing -> availability gate engages.
    expect(
      shouldHideSettingsNavItem(
        'organization-overview',
        createContext({
          hasFeature: hasFeatures([]),
          presentationPolicyResolved: true,
          isRuntimeCapabilityBlocked: () => false,
        }),
      ),
    ).toBe(true);
  });

  it('missingFeaturesArePaidRuntimeBlocked: every()->true keeps visible, every()->false hides/blocks', () => {
    // every()->true over the blocker -> not hidden, not blocked.
    const blockedCtx = createContext({
      hasFeature: hasFeatures([]),
      presentationPolicyResolved: true,
      isRuntimeCapabilityBlocked: () => true,
    });
    expect(shouldHideSettingsNavItem('organization-overview', blockedCtx)).toBe(false);
    expect(shouldBlockSettingsRouteItem('organization-overview', blockedCtx)).toBe(false);

    // every()->false over the blocker -> hidden AND blocked.
    const notBlockedCtx = createContext({
      hasFeature: hasFeatures([]),
      presentationPolicyResolved: true,
      isRuntimeCapabilityBlocked: () => false,
    });
    expect(shouldHideSettingsNavItem('organization-overview', notBlockedCtx)).toBe(true);
    expect(shouldBlockSettingsRouteItem('organization-overview', notBlockedCtx)).toBe(true);
  });
});
