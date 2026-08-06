/**
 * Branch-coverage tests for licensePresentation.ts (0725am batch).
 *
 * Each `it` targets a distinct uncovered branch arm reported by a V8
 * branch-coverage run scoped to this module.  Inputs that violate the
 * (required, non-optional) `tier` / `subscription_state` / `capabilities`
 * string fields are cast through `unknown` so TypeScript stays strict while
 * still exercising the module's `|| ''` / `|| []` defensive fallbacks.
 *
 * Does NOT modify the source module or any existing test file.
 */
import { describe, expect, it } from 'vitest';

import type { LicenseCommercialEntitlements } from '@/api/license';
import {
  formatLicensePlanVersion,
  getBillingAdminStateUpdateSuccessMessage,
  getBillingAdminTrialStatus,
  getOrganizationBillingLicenseStatusLabel,
  getSelfHostedActivationSuccessPresentation,
  getSelfHostedCurrentPlanPresentation,
  getSelfHostedPlanStatusPresentation,
  requiresPulseProRuntime,
} from '@/utils/licensePresentation';

/* ------------------------------------------------------------------ *
 * `(subscriptionState || '')` short-circuit '' arms
 *
 * A single current-plan call with an absent subscription_state drives the
 * `|| ''` fallback arm in four internal helpers at once:
 *   line 662  getSelfHostedCurrentPlanPresentation  (current.subscription_state)
 *   line 400  getPatrolControlAction               (subscriptionState arg)
 *   line 194  isActiveOrGraceSubscription          (subscriptionState arg)
 *   line 338  isActivePaidRuntimeState             (via requiresPulseProRuntime)
 *
 * The observable that DISTINGUISHES the '' arm: a grandfathered plan_version
 * is present, but because isActiveOrGraceSubscription('') === false the
 * 'Grandfathered price' badge is NOT pushed.  The existing grace+grandfathered
 * spec pushes that badge (truthy arm); this test proves the falsy arm skips it.
 * ------------------------------------------------------------------ */

describe('getSelfHostedCurrentPlanPresentation - absent subscription_state hits the || "" fallback arms', () => {
  it('does not push the grandfathered-price badge when subscription_state is absent even with a grandfathered plan_version', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: undefined,
        plan_version: 'v5_pro_monthly_grandfathered',
        capabilities: ['relay', 'mobile_app', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      } as unknown as LicenseCommercialEntitlements,
      displayableCapabilities: [
        'Pulse Relay (Remote Access)',
        'Patrol Applies Safe Fixes and Verifies the Result',
      ],
    });
    // isActiveOrGraceSubscription('') === false short-circuits the && so no
    // grandfathered badge is pushed despite the grandfathered plan_version.
    expect(result.supplementalBadges).not.toContain('Grandfathered price');
    expect(result.supplementalBadges).toEqual([]);
    // normalizedState '' is neither trial nor active/grace → final fallback.
    expect(result.body).toBe(
      'Review the plan details below to confirm what this key enables on this instance.',
    );
    // getPatrolControlAction returns undefined for the '' state.
    expect(result.patrolControlAction).toBeUndefined();
  });
});

/* ------------------------------------------------------------------ *
 * requiresPulseProRuntime - direct '' arms (lines 338 and 351)
 * ------------------------------------------------------------------ */

describe('requiresPulseProRuntime - absent subscription_state (line 338)', () => {
  it('returns false when a Pro-tier entitlement has no subscription_state', () => {
    // isActivePaidRuntimeState(undefined) takes the `|| ''` arm and returns
    // false, so requiresPulseProRuntime is false even for an active-style tier.
    expect(
      requiresPulseProRuntime({
        tier: 'pro',
        subscription_state: undefined,
      } as unknown as Parameters<typeof requiresPulseProRuntime>[0]),
    ).toBe(false);
  });
});

describe('requiresPulseProRuntime - absent tier (line 351)', () => {
  it('returns false when the entitlement has no tier', () => {
    // (entitlements.tier || '') takes the `|| ''` arm → normalizedTier '' is
    // not in PRO_RUNTIME_REQUIRED_TIERS → false.
    expect(
      requiresPulseProRuntime({
        subscription_state: 'active',
      } as unknown as Parameters<typeof requiresPulseProRuntime>[0]),
    ).toBe(false);
  });
});

/* ------------------------------------------------------------------ *
 * formatLicensePlanVersion (lines 452, 453, 459)
 * ------------------------------------------------------------------ */

describe('formatLicensePlanVersion - absent value (lines 452 & 453)', () => {
  it('returns null for null and undefined input via the `|| ""` then early-return arms', () => {
    // line 452: (value || '') takes the '' arm for null/undefined.
    // line 453: !normalized is true → return null.
    expect(formatLicensePlanVersion(null)).toBeNull();
    expect(formatLicensePlanVersion(undefined)).toBeNull();
  });
});

describe('formatLicensePlanVersion - non-canonical value (line 459)', () => {
  it('falls through the grandfathered/legacy/cloud lookups to the title-case fallback', () => {
    // 'internal_beta_channel' is absent from every label map, so the
    // `if (canonical) return canonical;` arm at line 459 is the FALSE branch
    // (existing specs only exercise the truthy canonical arm via cloud_founding).
    expect(formatLicensePlanVersion('internal_beta_channel')).toBe('Internal Beta Channel');
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedCurrentPlanPresentation - absent tier (line 663)
 * ------------------------------------------------------------------ */

describe('getSelfHostedCurrentPlanPresentation - absent tier (line 663)', () => {
  it('renders the Unknown plan label via the `|| ""` tier arm', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: undefined,
        subscription_state: 'active',
        capabilities: ['relay'],
        limits: [],
        upgrade_reasons: [],
      } as unknown as LicenseCommercialEntitlements,
      displayableCapabilities: ['Pulse Relay (Remote Access)'],
    });
    // (current.tier || '') → '' → getSelfHostedPlanLabel(undefined) === 'Unknown'.
    expect(result.title).toBe('Current plan: Unknown');
    // planDefinition null → getSelfHostedActivePlanSummary returns null → || fallback body.
    expect(result.body).toBe(
      'Unknown is active on this instance. These capabilities are available right now.',
    );
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedCurrentPlanPresentation - enterprise + community runtime
 * Line 758: includedExtrasLabel ternary `: undefined` arm.
 *
 * 'enterprise' is in PRO_RUNTIME_REQUIRED_TIERS (so requiresPulseProRuntime /
 * hasPulseProRuntimeMismatch fire) but has NO plan definition, so
 * getSelfHostedIncludedExtras returns [] → includedExtrasLabel falls to the
 * `undefined` arm.  Existing runtime-mismatch specs use tier 'pro', whose plan
 * definition carries non-empty includedExtras ('Included extras' arm).
 * ------------------------------------------------------------------ */

describe('getSelfHostedCurrentPlanPresentation - empty extras on a runtime-mismatched tier (line 758)', () => {
  it('omits includedExtrasLabel when a Pro-runtime tier has no plan definition extras', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'enterprise',
        subscription_state: 'active',
        capabilities: ['relay', 'audit_logging', 'rbac', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'community', label: 'Pulse Community runtime' },
      },
      displayableCapabilities: ['Pulse Relay (Remote Access)', 'Audit Logging'],
    });
    // includedExtras.length === 0 → the `: undefined` arm of the ternary.
    expect(result.includedExtrasLabel).toBeUndefined();
    expect(result.includedExtras).toEqual([]);
    // Confirms we are in the active+runtimeMismatch return block (line 753+).
    expect(result.supplementalBadges).toContain('Pro runtime missing');
    expect(result.privateRuntimeAction).toEqual({
      actionLabel: 'Open Pulse Pro downloads',
      actionUrl: 'https://pulserelay.pro/download.html',
    });
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedPlanStatusPresentation (lines 855 & 861)
 * ------------------------------------------------------------------ */

describe('getSelfHostedPlanStatusPresentation - absent subscription_state (line 855)', () => {
  it('returns null when subscription_state is absent so normalizedState is ""', () => {
    // (entitlements.subscription_state || '') takes the '' arm, then the
    // active/grace/trial guard at line 856 rejects "" → return null.
    expect(
      getSelfHostedPlanStatusPresentation({
        tier: 'relay',
        subscription_state: undefined,
        capabilities: ['relay'],
        limits: [],
        upgrade_reasons: [],
        max_history_days: 14,
      } as unknown as LicenseCommercialEntitlements),
    ).toBeNull();
  });
});

describe('getSelfHostedPlanStatusPresentation - absent capabilities (line 861)', () => {
  it('treats an absent capabilities array as empty so every required capability is missing', () => {
    const result = getSelfHostedPlanStatusPresentation({
      tier: 'relay',
      subscription_state: 'active',
      capabilities: undefined,
      limits: [],
      upgrade_reasons: [],
      max_history_days: 14,
    } as unknown as LicenseCommercialEntitlements);
    expect(result).not.toBeNull();
    // (entitlements.capabilities || []) takes the [] arm → empty capability Set
    // → the remote-access item is fully 'missing'.
    const remoteAccess = result?.items.find((i) => i.label.includes('Remote access'));
    expect(remoteAccess).toMatchObject({
      state: 'missing',
      statusLabel: 'Needs attention',
    });
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedActivationSuccessPresentation (lines 965 & 966)
 * ------------------------------------------------------------------ */

describe('getSelfHostedActivationSuccessPresentation - absent subscription_state (line 965)', () => {
  it('returns null when subscription_state is absent', () => {
    // (current.subscription_state || '') takes the '' arm; the active/grace
    // guard at line 971 then rejects "" → null.
    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: undefined,
          capabilities: ['relay', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        } as unknown as LicenseCommercialEntitlements,
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
        source: 'manual',
      }),
    ).toBeNull();
  });
});

describe('getSelfHostedActivationSuccessPresentation - absent tier (line 966)', () => {
  it('renders the Unknown plan label via the `|| ""` tier arm', () => {
    const result = getSelfHostedActivationSuccessPresentation({
      entitlements: {
        tier: undefined,
        subscription_state: 'active',
        capabilities: ['relay'],
        limits: [],
        upgrade_reasons: [],
      } as unknown as LicenseCommercialEntitlements,
      displayableCapabilities: ['Pulse Relay (Remote Access)'],
      source: 'manual',
    });
    expect(result).not.toBeNull();
    // (current.tier || '') → '' → getSelfHostedPlanLabel(undefined) === 'Unknown'.
    expect(result?.title).toBe('Unknown is now active');
    expect(result?.body).toBe(
      'The license key was accepted and this instance is now running Unknown.',
    );
  });
});

/* ------------------------------------------------------------------ *
 * Small switch / ternary arms (lines 1251, 1259, 1294)
 * ------------------------------------------------------------------ */

describe('getOrganizationBillingLicenseStatusLabel - valid license outside grace (line 1251)', () => {
  it('returns Active for a valid license that is not in its grace period', () => {
    // Existing specs cover the `: 'Active'` falsy arm only via in_grace_period
    // being absent on a {valid:false} payload (which returns 'No License').
    // Here valid:true & in_grace_period:false takes the `: 'Active'` arm.
    expect(getOrganizationBillingLicenseStatusLabel({ valid: true, in_grace_period: false })).toBe(
      'Active',
    );
  });
});

describe('getBillingAdminTrialStatus - absent subscription_state (line 1259)', () => {
  it('renders the started date as N/A when subscription_state is absent but a trial end exists', () => {
    // (state.subscription_state || '') takes the '' arm; with a trial_ends_at
    // present the 'No trial' guard is skipped and the started date renders N/A.
    const result = getBillingAdminTrialStatus({
      trial_ends_at: 1_700_000_000,
    } as never);
    expect(result).toContain('Trial (started N/A');
    expect(result).toContain('ends');
    expect(result).not.toContain('No trial');
  });
});

describe('getBillingAdminStateUpdateSuccessMessage - suspended (line 1294)', () => {
  it('returns the suspended message for the suspended arm of the ternary', () => {
    // Existing specs only exercise the `: 'Organization billing activated'`
    // arm (nextState 'active'); this hits the 'suspended' truthy arm.
    expect(getBillingAdminStateUpdateSuccessMessage('suspended')).toBe(
      'Organization billing suspended',
    );
  });
});
