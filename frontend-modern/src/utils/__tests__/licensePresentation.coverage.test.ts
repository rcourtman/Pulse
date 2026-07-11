/**
 * Branch-coverage tests for licensePresentation.ts.
 *
 * Fills defensive / edge-case branches not already exercised by
 * `licensePresentation.test.ts`.  Each `it` targets a distinct branch
 * (happy path, null / undefined input, fallback, or early-return).
 */
import { describe, expect, it } from 'vitest';

import {
  getBillingAdminTrialStatus,
  getCommercialMigrationActionText,
  getCommercialMigrationNotice,
  getFeatureMinTierLabel,
  getGrandfatheredPriceContinuityNotice,
  getLicenseFeatureLabel,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
  getSelfHostedActivationSuccessPresentation,
  getSelfHostedCurrentPlanPresentation,
  getSelfHostedCurrentPlanStatusPresentation,
  getSelfHostedPlanComparisonPresentation,
  getSelfHostedPlanLabel,
  getSelfHostedPlanStatusPresentation,
  isDisplayableLicenseFeature,
  requiresPulseProRuntime,
} from '@/utils/licensePresentation';
import { PATROL_CONTROL_PATH } from '@/routing/resourceLinks';

/* ------------------------------------------------------------------ *
 * Label / feature lookup edge cases
 * ------------------------------------------------------------------ */

describe('getLicenseTierLabel - edge cases', () => {
  it('returns Unknown for empty, null, and undefined tier', () => {
    expect(getLicenseTierLabel('')).toBe('Unknown');
    expect(getLicenseTierLabel(null)).toBe('Unknown');
    expect(getLicenseTierLabel(undefined)).toBe('Unknown');
    expect(getLicenseTierLabel('   ')).toBe('Unknown');
  });

  it('returns mapped labels for every canonical tier key', () => {
    expect(getLicenseTierLabel('relay')).toBe('Relay');
    expect(getLicenseTierLabel('pro')).toBe('Pro');
    expect(getLicenseTierLabel('pro_annual')).toBe('Pro Annual');
    expect(getLicenseTierLabel('lifetime')).toBe('Lifetime');
    expect(getLicenseTierLabel('cloud')).toBe('Cloud');
    expect(getLicenseTierLabel('msp')).toBe('MSP');
  });

  it('title-cases unknown tiers as a fallback', () => {
    expect(getLicenseTierLabel('team_seats')).toBe('Team Seats');
  });
});

describe('getSelfHostedPlanLabel - edge cases', () => {
  it('returns Unknown for empty, null, and undefined', () => {
    expect(getSelfHostedPlanLabel('')).toBe('Unknown');
    expect(getSelfHostedPlanLabel(null)).toBe('Unknown');
    expect(getSelfHostedPlanLabel(undefined)).toBe('Unknown');
  });

  it('maps pro_annual to Pulse Pro Annual', () => {
    expect(getSelfHostedPlanLabel('pro_annual')).toBe('Pulse Pro Annual');
  });

  it('falls back to getLicenseTierLabel for tiers not in SELF_HOSTED_PLAN_LABELS', () => {
    expect(getSelfHostedPlanLabel('enterprise')).toBe('Enterprise');
    expect(getSelfHostedPlanLabel('custom_plan')).toBe('Custom Plan');
  });
});

describe('getLicenseFeatureLabel - edge cases', () => {
  it('returns Unknown for empty, null, and undefined', () => {
    expect(getLicenseFeatureLabel('')).toBe('Unknown');
    expect(getLicenseFeatureLabel(null)).toBe('Unknown');
    expect(getLicenseFeatureLabel(undefined)).toBe('Unknown');
  });
});

describe('isDisplayableLicenseFeature - edge cases', () => {
  it('returns false for empty, null, and undefined', () => {
    expect(isDisplayableLicenseFeature('')).toBe(false);
    expect(isDisplayableLicenseFeature(null)).toBe(false);
    expect(isDisplayableLicenseFeature(undefined)).toBe(false);
    expect(isDisplayableLicenseFeature('   ')).toBe(false);
  });

  it('normalises case and whitespace before checking the catalog', () => {
    expect(isDisplayableLicenseFeature('AI_PATROL')).toBe(true);
    expect(isDisplayableLicenseFeature('  Relay  ')).toBe(true);
  });
});

describe('getFeatureMinTierLabel - edge cases', () => {
  it('returns Pro for empty, null, and undefined', () => {
    expect(getFeatureMinTierLabel('')).toBe('Pro');
    expect(getFeatureMinTierLabel(null)).toBe('Pro');
    expect(getFeatureMinTierLabel(undefined)).toBe('Pro');
  });

  it('returns Relay for push_notifications and mobile_app', () => {
    expect(getFeatureMinTierLabel('push_notifications')).toBe('Relay');
    expect(getFeatureMinTierLabel('mobile_app')).toBe('Relay');
  });
});

/* ------------------------------------------------------------------ *
 * Subscription-status presentation
 * ------------------------------------------------------------------ */

describe('getLicenseSubscriptionStatusPresentation - uncovered branches', () => {
  it('returns Active and Suspended for their respective states', () => {
    expect(getLicenseSubscriptionStatusPresentation('active')).toEqual({
      label: 'Active',
      badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
    });
    expect(getLicenseSubscriptionStatusPresentation('suspended')).toEqual({
      label: 'Suspended',
      badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
    });
  });

  it('maps canceled to the same presentation as expired', () => {
    expect(getLicenseSubscriptionStatusPresentation('canceled')).toEqual({
      label: 'Expired',
      badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
    });
  });

  it('normalises mixed-case and whitespace before matching', () => {
    expect(getLicenseSubscriptionStatusPresentation('  TRIAL  ').label).toBe('Trial');
    expect(getLicenseSubscriptionStatusPresentation('Grace').label).toBe('Grace Period');
  });
});

describe('getSelfHostedCurrentPlanStatusPresentation - edge cases', () => {
  it('maps community tier to the Community badge', () => {
    expect(
      getSelfHostedCurrentPlanStatusPresentation({
        tier: 'community',
        subscription_state: 'active',
      }),
    ).toEqual({
      label: 'Community',
      badgeClass: 'bg-surface text-base-content border border-border',
    });
  });

  it('delegates to getLicenseSubscriptionStatusPresentation for null entitlements', () => {
    expect(getSelfHostedCurrentPlanStatusPresentation(null)).toEqual({
      label: 'Unknown',
      badgeClass: 'bg-surface-alt text-muted',
    });
    expect(getSelfHostedCurrentPlanStatusPresentation(undefined)).toEqual({
      label: 'Unknown',
      badgeClass: 'bg-surface-alt text-muted',
    });
  });

  it('delegates to subscription-state presentation for a paid tier', () => {
    expect(
      getSelfHostedCurrentPlanStatusPresentation({
        tier: 'pro',
        subscription_state: 'suspended',
      }),
    ).toEqual({
      label: 'Suspended',
      badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
    });
  });
});

/* ------------------------------------------------------------------ *
 * requiresPulseProRuntime
 * ------------------------------------------------------------------ */

describe('requiresPulseProRuntime - branch coverage', () => {
  it('returns false for null and undefined entitlements', () => {
    expect(requiresPulseProRuntime(null)).toBe(false);
    expect(requiresPulseProRuntime(undefined)).toBe(false);
  });

  it('returns false when hosted_mode is true', () => {
    expect(
      requiresPulseProRuntime({
        tier: 'pro',
        subscription_state: 'active',
        hosted_mode: true,
      }),
    ).toBe(false);
  });

  it('returns false for tiers outside the Pro runtime set', () => {
    expect(
      requiresPulseProRuntime({ tier: 'relay', subscription_state: 'active' }),
    ).toBe(false);
    expect(
      requiresPulseProRuntime({ tier: 'free', subscription_state: 'active' }),
    ).toBe(false);
  });

  it('returns false for a Pro-tier with non-paid subscription state', () => {
    expect(
      requiresPulseProRuntime({ tier: 'pro', subscription_state: 'expired' }),
    ).toBe(false);
  });

  it('returns true for active, grace, and trial Pro-tier entitlements', () => {
    expect(
      requiresPulseProRuntime({ tier: 'pro', subscription_state: 'active' }),
    ).toBe(true);
    expect(
      requiresPulseProRuntime({ tier: 'pro', subscription_state: 'grace' }),
    ).toBe(true);
    expect(
      requiresPulseProRuntime({ tier: 'pro', subscription_state: 'trial' }),
    ).toBe(true);
  });

  it('returns true for every tier in the Pro-runtime-required set', () => {
    for (const tier of ['pro', 'pro_annual', 'pro_plus', 'lifetime', 'enterprise']) {
      expect(
        requiresPulseProRuntime({ tier, subscription_state: 'active' }),
      ).toBe(true);
    }
  });
});

/* ------------------------------------------------------------------ *
 * getGrandfatheredPriceContinuityNotice
 * ------------------------------------------------------------------ */

describe('getGrandfatheredPriceContinuityNotice - edge cases', () => {
  it('returns null for null / undefined plan version', () => {
    expect(getGrandfatheredPriceContinuityNotice(null, 'active')).toBeNull();
    expect(getGrandfatheredPriceContinuityNotice(undefined, 'active')).toBeNull();
  });

  it('returns null for a non-grandfathered plan version even when active', () => {
    expect(getGrandfatheredPriceContinuityNotice('pro', 'active')).toBeNull();
  });

  it('returns null for a grandfathered plan with a null subscription state', () => {
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_monthly_grandfathered', null),
    ).toBeNull();
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_annual_grandfathered', undefined),
    ).toBeNull();
  });

  it('returns null for a grandfathered plan with a non-active non-grace state', () => {
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_monthly_grandfathered', 'suspended'),
    ).toBeNull();
  });
});

/* ------------------------------------------------------------------ *
 * getCommercialMigrationActionText
 * ------------------------------------------------------------------ */

describe('getCommercialMigrationActionText - all switch branches', () => {
  it('returns retry guidance for retry_activation', () => {
    expect(getCommercialMigrationActionText('retry_activation')).toBe(
      'Retry from this instance.',
    );
  });

  it('returns current-key guidance for use_v6_activation_key', () => {
    expect(getCommercialMigrationActionText('use_v6_activation_key')).toBe(
      'Use the current v6 key for this purchase.',
    );
  });

  it('returns v5-key guidance for enter_supported_v5_key', () => {
    expect(getCommercialMigrationActionText('enter_supported_v5_key')).toBe(
      'Retry with the original v5 Pro/Lifetime key from this instance.',
    );
  });

  it('returns support-contact guidance for free_installation_slot', () => {
    expect(getCommercialMigrationActionText('free_installation_slot')).toContain(
      'Contact support@pulserelay.pro',
    );
  });

  it('returns retrieval guidance for retrieve_current_key', () => {
    expect(getCommercialMigrationActionText('retrieve_current_key')).toContain(
      'pulserelay.pro/retrieve-license',
    );
  });

  it('returns egress guidance for allow_license_egress', () => {
    expect(getCommercialMigrationActionText('allow_license_egress')).toContain(
      'Allow outbound HTTPS to license.pulserelay.pro',
    );
  });

  it('returns generic guidance for undefined and unknown actions', () => {
    expect(getCommercialMigrationActionText(undefined)).toBe(
      'Review the plan state from this instance before trying again.',
    );
    expect(getCommercialMigrationActionText('totally_unknown')).toBe(
      'Review the plan state from this instance before trying again.',
    );
  });
});

/* ------------------------------------------------------------------ *
 * getCommercialMigrationNotice
 * ------------------------------------------------------------------ */

describe('getCommercialMigrationNotice - uncovered branches', () => {
  it('returns null for undefined migration', () => {
    expect(getCommercialMigrationNotice(undefined)).toBeNull();
  });

  it('returns null when migration has no state', () => {
    expect(getCommercialMigrationNotice({})).toBeNull();
    expect(getCommercialMigrationNotice({ reason: 'exchange_invalid' })).toBeNull();
  });

  it('renders pending conflict as settling handoff', () => {
    const notice = getCommercialMigrationNotice({
      state: 'pending',
      reason: 'exchange_conflict',
    });
    expect(notice).toMatchObject({
      title: 'v5 license migration pending',
      tone: expect.stringContaining('amber'),
    });
    expect(notice?.body).toContain('still settling');
  });

  it('uses the generic pending body for exchange_unavailable', () => {
    const notice = getCommercialMigrationNotice({
      state: 'pending',
      reason: 'exchange_unavailable',
    });
    expect(notice?.body).toContain('did not complete yet');
  });

  it('uses the generic pending body for an unrecognised reason', () => {
    const notice = getCommercialMigrationNotice({
      state: 'pending',
      reason: 'some_new_reason',
    });
    expect(notice?.body).toContain('did not complete yet');
  });

  it('renders malformed key as terminal', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'exchange_malformed',
    });
    expect(notice?.body).toContain('malformed and cannot be migrated');
  });

  it('renders revoked key as terminal', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'exchange_revoked',
    });
    expect(notice?.body).toContain('no longer eligible for automatic migration');
  });

  it('renders non-migratable key as terminal', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'exchange_non_migratable',
    });
    expect(notice?.body).toContain('not eligible for automatic v6 migration');
  });

  it('renders unsupported key as terminal', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'exchange_unsupported',
    });
    expect(notice?.body).toContain('not a supported v5 Pro/Lifetime migration input');
  });

  it('uses the generic failed body for an unrecognised reason', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'some_unknown_reason',
    });
    expect(notice?.body).toContain('could not be migrated automatically');
  });
});

/* ------------------------------------------------------------------ *
 * getBillingAdminTrialStatus (exercises internal formatUnixSeconds)
 * ------------------------------------------------------------------ */

describe('getBillingAdminTrialStatus - branch coverage', () => {
  it('returns Loading for null and undefined state', () => {
    expect(getBillingAdminTrialStatus(null)).toBe('Loading...');
    expect(getBillingAdminTrialStatus(undefined)).toBe('Loading...');
  });

  it('returns No trial for a non-trial state with no trial timestamps', () => {
    expect(
      getBillingAdminTrialStatus({ subscription_state: 'active' } as never),
    ).toBe('No trial');
  });

  it('formats a trial state with only trial_ends_at', () => {
    const result = getBillingAdminTrialStatus({
      subscription_state: 'trial',
      trial_ends_at: 1_700_000_000,
    } as never);
    expect(result).toContain('Trial (ends');
    expect(result).not.toContain('started');
  });

  it('formats a non-trial state that has trial timestamps', () => {
    const result = getBillingAdminTrialStatus({
      subscription_state: 'active',
      trial_started_at: 1_690_000_000,
      trial_ends_at: 1_700_000_000,
    } as never);
    expect(result).toContain('Trial (started');
    expect(result).toContain('ends');
  });

  it('returns N/A for a zero or negative timestamp via formatUnixSeconds', () => {
    const result = getBillingAdminTrialStatus({
      subscription_state: 'active',
      trial_started_at: 0,
      trial_ends_at: 1_700_000_000,
    } as never);
    expect(result).toContain('started N/A');
  });

  it('returns the raw string for a non-finite timestamp via formatUnixSeconds', () => {
    const result = getBillingAdminTrialStatus({
      subscription_state: 'active',
      trial_started_at: Infinity,
    } as never);
    expect(result).toContain('started Infinity');
    expect(result).toContain('ends N/A');
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedPlanComparisonPresentation
 * ------------------------------------------------------------------ */

describe('getSelfHostedPlanComparisonPresentation - edge cases', () => {
  it('shows Relay and Pro cards for null / undefined entitlements', () => {
    const result = getSelfHostedPlanComparisonPresentation({ entitlements: null });
    expect(result.cards).toHaveLength(2);
    expect(result.cards[0].title).toBe('Relay plan');
    expect(result.cards[1].title).toBe('Pulse Pro plan');
  });

  it('shows Relay and Pro cards for the community tier', () => {
    const result = getSelfHostedPlanComparisonPresentation({
      entitlements: {
        tier: 'community',
        subscription_state: 'active',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
      },
    });
    expect(result.cards.map((c) => c.title)).toEqual(['Relay plan', 'Pulse Pro plan']);
  });

  it('returns no cards for an unrecognized tier', () => {
    const result = getSelfHostedPlanComparisonPresentation({
      entitlements: {
        tier: 'msp',
        subscription_state: 'active',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
      },
    });
    expect(result.cards).toEqual([]);
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedCurrentPlanPresentation
 * (exercises internal: isActiveOrGraceSubscription,
 *  getPatrolControlAction, getSelfHostedUnlockedFeatures,
 *  getSelfHostedIncludedExtras, hasGovernedDecisionOnlyPatrolOperatorOutcome)
 * ------------------------------------------------------------------ */

describe('getSelfHostedCurrentPlanPresentation - null entitlements branch', () => {
  it('returns the loading state when entitlements are null', () => {
    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: null,
        displayableCapabilities: [],
      }),
    ).toEqual({
      title: 'Current plan: Unknown',
      body: 'Pulse is still loading the current self-hosted plan state for this instance.',
      unlockedFeaturesLabel: 'Available on this instance',
      unlockedFeatures: [],
      includedExtras: [],
      supplementalBadges: [],
    });
  });
});

describe('getSelfHostedCurrentPlanPresentation - trial branch', () => {
  it('shows runtime-mismatch badge and private-runtime action for a Pro trial on community runtime', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'trial',
        capabilities: ['ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'community', label: 'Pulse Community runtime' },
      },
      displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
    });
    expect(result.title).toBe('Current plan: Pulse Pro Trial');
    expect(result.body).toContain('trial entitlement is active');
    expect(result.body).toContain('running the community runtime');
    expect(result.supplementalBadges).toContain('Pro runtime missing');
    expect(result.privateRuntimeAction).toEqual({
      actionLabel: 'Open Pulse Pro downloads',
      actionUrl: 'https://pulserelay.pro/download.html',
    });
    expect(result.patrolControlAction).toBeUndefined();
  });

  it('shows active-trial body when unlocked features are present', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'relay',
        subscription_state: 'trial',
        capabilities: ['relay', 'mobile_app', 'push_notifications'],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: [
        'Pulse Relay (Remote Access)',
        'Pulse Mobile Pairing',
        'Push Notifications',
      ],
    });
    expect(result.title).toBe('Current plan: Relay Trial');
    expect(result.body).toBe('Relay trial capabilities are active on this instance right now.');
  });

  it('shows being-confirmed body when no unlocked features are available', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'msp',
        subscription_state: 'trial',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: [],
    });
    expect(result.title).toBe('Current plan: MSP Trial');
    expect(result.body).toBe('MSP trial entitlement is being confirmed for this instance.');
    expect(result.unlockedFeatures).toEqual([]);
  });
});

describe('getSelfHostedCurrentPlanPresentation - is_lifetime branch', () => {
  it('pushes the grandfathered-lifetime badge for an active lifetime install', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        is_lifetime: true,
        capabilities: ['relay', 'mobile_app', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: ['Pulse Relay (Remote Access)', 'Patrol Applies Safe Fixes and Verifies the Result'],
    });
    expect(result.supplementalBadges).toContain('Grandfathered lifetime');
    expect(result.supplementalSummary).toContain(
      'migrated lifetime install remains valid permanently',
    );
  });
});

describe('getSelfHostedCurrentPlanPresentation - grace + grandfathered branch', () => {
  it('shows the grandfathered-price badge for a grace-state recurring v5 plan', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'grace',
        plan_version: 'v5_pro_annual_grandfathered',
        capabilities: ['relay', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: ['Pulse Relay (Remote Access)', 'Patrol Applies Safe Fixes and Verifies the Result'],
    });
    expect(result.supplementalBadges).toContain('Grandfathered price');
  });
});

describe('getSelfHostedCurrentPlanPresentation - fallback branch', () => {
  it('returns the review-details fallback for a suspended pro plan', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'suspended',
        capabilities: ['relay', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: ['Pulse Relay (Remote Access)'],
    });
    expect(result.title).toBe('Current plan: Pulse Pro');
    expect(result.body).toBe(
      'Review the plan details below to confirm what this key enables on this instance.',
    );
    expect(result.unlockedFeaturesLabel).toBe('Available on this instance');
    expect(result.patrolControlAction).toBeUndefined();
  });
});

describe('getSelfHostedCurrentPlanPresentation - getPatrolControlAction branches', () => {
  it('omits patrolControlAction for an active Pro plan without the ai_autofix capability', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: ['Pulse Relay (Remote Access)'],
    });
    expect(result.patrolControlAction).toBeUndefined();
  });

  it('uses patrol-control decision stage when governed decision has no value state', () => {
    const result = getSelfHostedCurrentPlanPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay', 'mobile_app', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: ['Pulse Relay (Remote Access)', 'Patrol Applies Safe Fixes and Verifies the Result'],
      patrolOperatorStatus: {
        patrolControlOperationsLoopStarterCount: 1,
        patrolControlCompletedOperationsLoopCount: 1,
        patrolControlResolvedOperationsLoopCount: 0,
        externalAgentReady: false,
      },
    });
    expect(result.patrolControlAction).toMatchObject({
      actionLabel: 'Review Patrol decision',
      actionUrl: PATROL_CONTROL_PATH,
      actionIntent: 'patrol_control',
    });
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedPlanStatusPresentation
 * ------------------------------------------------------------------ */

describe('getSelfHostedPlanStatusPresentation - null-return branches', () => {
  it('returns null for null and undefined entitlements', () => {
    expect(getSelfHostedPlanStatusPresentation(null)).toBeNull();
    expect(getSelfHostedPlanStatusPresentation(undefined)).toBeNull();
  });

  it('returns null for a tier with no plan definition', () => {
    expect(
      getSelfHostedPlanStatusPresentation({
        tier: 'enterprise',
        subscription_state: 'active',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
        max_history_days: 90,
      }),
    ).toBeNull();
  });

  it('returns null when max_history_days is missing', () => {
    expect(
      getSelfHostedPlanStatusPresentation({
        tier: 'relay',
        subscription_state: 'active',
        capabilities: ['relay'],
        limits: [],
        upgrade_reasons: [],
      }),
    ).toBeNull();
  });

  it('returns null for an expired subscription state', () => {
    expect(
      getSelfHostedPlanStatusPresentation({
        tier: 'relay',
        subscription_state: 'expired',
        capabilities: ['relay'],
        limits: [],
        upgrade_reasons: [],
        max_history_days: 14,
      }),
    ).toBeNull();
  });
});

describe('getSelfHostedPlanStatusPresentation - trial and missing-history states', () => {
  it('builds status for a trial Relay plan', () => {
    const result = getSelfHostedPlanStatusPresentation({
      tier: 'relay',
      subscription_state: 'trial',
      capabilities: ['relay', 'mobile_app', 'push_notifications'],
      limits: [],
      upgrade_reasons: [],
      max_history_days: 14,
    });
    expect(result?.title).toBe('Relay status');
    expect(result?.items).toHaveLength(2);
    expect(result?.items.every((i) => i.state === 'active')).toBe(true);
  });

  it('reports missing metric history when max_history_days is 0', () => {
    const result = getSelfHostedPlanStatusPresentation({
      tier: 'relay',
      subscription_state: 'active',
      capabilities: ['relay', 'mobile_app', 'push_notifications'],
      limits: [],
      upgrade_reasons: [],
      max_history_days: 0,
    });
    const historyItem = result?.items.find((i) => i.label.includes('metric history'));
    expect(historyItem).toMatchObject({
      state: 'missing',
      statusLabel: 'Needs attention',
    });
    expect(historyItem?.detail).toContain('does not have metric-history access yet');
  });

  it('reports partial metric history when days are below the required amount', () => {
    const result = getSelfHostedPlanStatusPresentation({
      tier: 'relay',
      subscription_state: 'active',
      capabilities: ['relay', 'mobile_app', 'push_notifications'],
      limits: [],
      upgrade_reasons: [],
      max_history_days: 7,
    });
    const historyItem = result?.items.find((i) => i.label.includes('metric history'));
    expect(historyItem?.state).toBe('partial');
    expect(historyItem?.detail).toContain('below the expected');
  });
});

/* ------------------------------------------------------------------ *
 * getSelfHostedActivationSuccessPresentation
 * (exercises internal: getSelfHostedActivationHighlights)
 * ------------------------------------------------------------------ */

describe('getSelfHostedActivationSuccessPresentation - null-return branches', () => {
  it('returns null when source is null', () => {
    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: [],
          limits: [],
          upgrade_reasons: [],
        },
        displayableCapabilities: [],
        source: null,
      }),
    ).toBeNull();
  });

  it('returns null when entitlements are null', () => {
    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: null,
        displayableCapabilities: [],
        source: 'manual',
      }),
    ).toBeNull();
  });
});

describe('getSelfHostedActivationSuccessPresentation - purchase + runtime mismatch', () => {
  it('renders amber tone and download action for a purchase with community runtime', () => {
    const result = getSelfHostedActivationSuccessPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'community', label: 'Pulse Community runtime' },
      },
      displayableCapabilities: ['Pulse Relay (Remote Access)', 'Patrol Applies Safe Fixes and Verifies the Result'],
      source: 'purchase',
    });
    expect(result).not.toBeNull();
    expect(result?.tone).toContain('amber');
    expect(result?.title).toBe('Pulse Pro license is active');
    expect(result?.body).toContain('Checkout completed and the license is active');
    expect(result?.body).toContain('running the community runtime');
    expect(result?.highlightsLabel).toBe('Licensed capabilities');
    expect(result?.actionLabel).toBe('Open Pulse Pro downloads');
    expect(result?.actionUrl).toBe('https://pulserelay.pro/download.html');
  });
});

describe('getSelfHostedActivationSuccessPresentation - purchase without patrol action', () => {
  it('renders the now-running body for a Relay purchase with no patrol capability', () => {
    const result = getSelfHostedActivationSuccessPresentation({
      entitlements: {
        tier: 'relay',
        subscription_state: 'active',
        capabilities: ['relay', 'mobile_app', 'push_notifications'],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: [
        'Pulse Relay (Remote Access)',
        'Pulse Mobile Pairing',
        'Push Notifications',
      ],
      source: 'purchase',
    });
    expect(result).not.toBeNull();
    expect(result?.title).toBe('Relay is now active');
    expect(result?.body).toBe('Checkout completed and this instance is now running Relay.');
    expect(result?.tone).toContain('green');
    expect(result?.highlightsLabel).toBe('Available now on this instance');
    expect(result?.actionLabel).toBeUndefined();
    expect(result?.actionUrl).toBeUndefined();
  });
});

describe('getSelfHostedActivationSuccessPresentation - manual + runtime mismatch', () => {
  it('renders the key-accepted body for a manual activation with missing runtime identity', () => {
    const result = getSelfHostedActivationSuccessPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay', 'audit_logging', 'rbac', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
      },
      displayableCapabilities: ['Pulse Relay (Remote Access)', 'Audit Logging'],
      source: 'manual',
    });
    expect(result).not.toBeNull();
    expect(result?.body).toContain('license key was accepted');
    expect(result?.body).toContain('not reporting the private Pulse Pro runtime');
  });
});

describe('getSelfHostedActivationSuccessPresentation - highlights dedup', () => {
  it('deduplicates overlapping plan highlights and displayable capabilities', () => {
    const result = getSelfHostedActivationSuccessPresentation({
      entitlements: {
        tier: 'pro',
        subscription_state: 'active',
        capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
        limits: [],
        upgrade_reasons: [],
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      },
      displayableCapabilities: [
        'Patrol Investigates Issues and Explains the Root Cause',
        'Patrol Applies Safe Fixes and Verifies the Result',
        '90-day metric history',
      ],
      source: 'manual',
    });
    expect(result).not.toBeNull();
    const highlights = result?.highlights ?? [];
    expect(highlights.length).toBe(new Set(highlights).size);
    expect(highlights).toContain('Patrol Investigates Issues and Explains the Root Cause');
    expect(highlights).toContain('Role-Based Access Control (RBAC)');
  });
});
