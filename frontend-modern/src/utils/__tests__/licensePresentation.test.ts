import { describe, expect, it } from 'vitest';
import {
  BILLING_ADMIN_EMPTY_STATE,
  formatLicensePlanVersion,
  getBillingAdminOrganizationBadges,
  getBillingAdminStateUpdateSuccessMessage,
  getBillingAdminTrialStatus,
  getCommercialMigrationNotice,
  getFeatureMinTierLabel,
  getGrandfatheredPriceContinuityNotice,
  getSelfHostedPlanStatusPresentation,
  getSelfHostedActivationSuccessPresentation,
  getSelfHostedPlanComparisonPresentation,
  getSelfHostedCurrentPlanPresentation,
  getSelfHostedCurrentPlanStatusPresentation,
  hasPulseProRuntimeMismatch,
  getLicenseFeatureLabel,
  getLicenseStatusLoadingState,
  getLicenseSubscriptionStatusPresentation,
  getSelfHostedPlanLabel,
  getLicenseTierLabel,
  getNoActiveSelfHostedActivationState,
  getOrganizationBillingLicenseStatusLabel,
  getPurchaseActivationNotice,
  isDisplayableLicenseFeature,
  isGrandfatheredRecurringV5PlanVersion,
  PATROL_CONTROL_STARTER_URL,
  SELF_HOSTED_RECOVERY_PRESENTATION,
} from '@/utils/licensePresentation';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from '@/components/Settings/selfHostedBillingPresentation';
import { getSelfHostedPlanEntitlementSummary } from '@/utils/selfHostedPlans';
import { PATROL_CONTROL_PATH } from '@/routing/resourceLinks';

describe('licensePresentation', () => {
  it('routes the Patrol mode CTA to the owned Patrol journey', () => {
    expect(PATROL_CONTROL_STARTER_URL).toBe(
      '/patrol?patrolControlStarter=patrol_control#patrol-control',
    );
  });

  it('returns canonical tier labels', () => {
    expect(getLicenseTierLabel('free')).toBe('Community');
    expect(getLicenseTierLabel('pro_plus')).toBe('Legacy Pro+');
    expect(getLicenseTierLabel('enterprise')).toBe('Enterprise');
    expect(getLicenseTierLabel('custom_tier')).toBe('Custom Tier');
    expect(getSelfHostedPlanLabel('pro')).toBe('Pulse Pro');
    expect(getSelfHostedPlanLabel('pro_plus')).toBe('Legacy Pulse Pro+');
    expect(getSelfHostedPlanLabel('lifetime')).toBe('Pulse Pro Lifetime');
    expect(getSelfHostedPlanLabel('relay')).toBe('Relay');
  });

  it('returns canonical license loading and inactive copy', () => {
    expect(getLicenseStatusLoadingState()).toEqual({
      text: 'Loading license status...',
    });
    expect(getNoActiveSelfHostedActivationState()).toEqual({
      text: 'Community is ready to use on this instance.',
    });
    expect(SELF_HOSTED_RECOVERY_PRESENTATION).toMatchObject({
      disclosureLabel: 'Manual key recovery',
      fieldLabel: 'License key',
      activateIdleLabel: 'Apply key',
      clearIdleLabel: 'Clear key',
      privateRuntimeNotice: {
        title: 'Paid Docker and Linux installs use a private runtime',
        actionLabel: 'Open Pulse Pro downloads',
        actionUrl: 'https://pulserelay.pro/download.html',
      },
      legacyNotice: {
        title: 'Legacy v5 license detected',
      },
    });
    expect(SELF_HOSTED_PRO_BILLING_PRESENTATION).toEqual({
      navLabel: 'Plans & Billing',
      shellTitle: 'Plans & Billing',
      shellDescription: 'Plan, license, and Patrol mode for this instance.',
      infrastructureRouteReferral: 'Billing and self-hosted plan changes live in Plans & Billing.',
      infrastructureWorkspaceReferral:
        'Self-hosted plan status, Patrol mode, and available capabilities live in Plans & Billing, not here.',
      sectionSelectorAriaLabel: 'Plans and billing section',
      refreshLabel: 'Refresh',
      planTabLabel: 'Plan',
      usageTabLabel: 'Usage',
      planSectionTitle: 'Current plan',
      planSectionDescription: 'Current tier and enabled capabilities.',
      planComparisonSectionTitle: 'Available plans',
      planComparisonActionLabel: 'View plans',
      planComparisonTrialActionLabel: 'Start 14-day free Pro trial',
      planComparisonTrialActionNote:
        'Card required. You will not be charged if you cancel during the trial.',
      usageSectionTitle: 'Usage',
      hiddenShellTitle: 'Demo mode',
      hiddenShellDescription: 'Commercial settings are hidden for this session.',
      hiddenStateTitle: 'License and billing details are hidden',
      hiddenStateBody:
        'This public demo uses sample infrastructure data, so Pulse hides license identity, billing state, monitored-system usage, and upgrade actions instead of creating a demo license.',
      policyLoadingTitle: 'Loading settings access',
      policyLoadingBody:
        'Pulse waits for the session presentation policy before showing license, billing, or usage details.',
      planSelectionPromptTitle: 'Select a plan',
      planSelectionPromptBody: 'Choose the plan for this install.',
      planSelectionPromptActionLabel: 'View plans',
      purchaseActivatedPlanActionLabel: 'Review plan',
      purchaseCancelledActionLabel: 'View plans',
      purchaseExpiredActionLabel: 'View plans',
      purchaseFailedActionLabel: 'Open recovery',
      purchaseUnavailableActionLabel: 'Try again',
      recoverySectionTitle: 'License recovery',
      recoverySectionDescription: 'Paste a license key or clear the license on this install.',
    });
  });

  it('returns canonical feature labels', () => {
    expect(getLicenseFeatureLabel('ai_patrol')).toBe('Pulse Patrol');
    expect(getLicenseFeatureLabel('mobile_app')).toBe('Pulse Mobile Pairing');
    expect(getLicenseFeatureLabel('update_alerts')).toBe('Update Alerts');
    expect(getLicenseFeatureLabel('relay')).toBe('Pulse Relay (Remote Access)');
    expect(getLicenseFeatureLabel('custom_feature')).toBe('Custom Feature');
  });

  it('hides non-operable capability labels from customer-facing plan surfaces', () => {
    expect(isDisplayableLicenseFeature('ai_patrol')).toBe(true);
    expect(isDisplayableLicenseFeature('multi_tenant')).toBe(false);
    expect(isDisplayableLicenseFeature('kubernetes_ai')).toBe(false);
    expect(isDisplayableLicenseFeature('multi_user')).toBe(false);
    expect(isDisplayableLicenseFeature('white_label')).toBe(false);
    expect(isDisplayableLicenseFeature('unlimited')).toBe(false);
  });

  it('returns minimum tier labels for gated features', () => {
    expect(getFeatureMinTierLabel('relay')).toBe('Relay');
    expect(getFeatureMinTierLabel('multi_tenant')).toBe('MSP');
    expect(getFeatureMinTierLabel('unknown_feature')).toBe('Pro');
  });

  it('returns canonical subscription-state labels and tones', () => {
    expect(getLicenseSubscriptionStatusPresentation('trial')).toMatchObject({
      label: 'Trial',
      badgeClass: expect.stringContaining('green'),
    });
    expect(getLicenseSubscriptionStatusPresentation('grace')).toMatchObject({
      label: 'Grace Period',
      badgeClass: expect.stringContaining('amber'),
    });
    expect(getLicenseSubscriptionStatusPresentation('expired')).toMatchObject({
      label: 'Expired',
      badgeClass: expect.stringContaining('red'),
    });
    expect(getLicenseSubscriptionStatusPresentation('mystery')).toMatchObject({
      label: 'Unknown',
      badgeClass: expect.stringContaining('bg-surface-alt'),
    });
    expect(
      getSelfHostedCurrentPlanStatusPresentation({
        tier: 'free',
        subscription_state: 'expired',
      }),
    ).toEqual({
      label: 'Community',
      badgeClass: 'bg-surface text-base-content border border-border',
    });
    expect(
      getSelfHostedCurrentPlanStatusPresentation({
        tier: 'pro',
        subscription_state: 'active',
      }),
    ).toMatchObject({
      label: 'Active',
      badgeClass: expect.stringContaining('green'),
    });
  });

  it('formats plan versions and commercial migration notices canonically', () => {
    expect(formatLicensePlanVersion('pro_plus')).toBe('Legacy Pro Plus');
    expect(formatLicensePlanVersion('cloud_founding')).toBe('Cloud Starter (Founding)');
    expect(formatLicensePlanVersion('msp_growth')).toBe('MSP Growth');
    expect(formatLicensePlanVersion('v5_pro_monthly_grandfathered')).toBe(
      'V5 Pro Monthly (Grandfathered)',
    );
    expect(formatLicensePlanVersion('v5_pro_annual_grandfathered')).toBe(
      'V5 Pro Annual (Grandfathered)',
    );
    expect(
      getCommercialMigrationNotice({
        state: 'pending',
        reason: 'exchange_rate_limited',
        recommended_action: 'retry_activation',
      } as never),
    ).toMatchObject({
      title: 'v5 license migration pending',
      tone: expect.stringContaining('amber'),
    });
  });

  it('renders the installation limit as terminal with slot guidance, never as a retryable handoff', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'exchange_installation_limit',
      recommended_action: 'free_installation_slot',
    } as never);
    expect(notice).toMatchObject({
      title: 'v5 license migration needs attention',
      tone: expect.stringContaining('red'),
    });
    expect(notice?.body).toContain('maximum number of v6 installations');
    expect(notice?.body).toContain('support@pulserelay.pro');
    expect(notice?.body).not.toContain('Retry activation');
    expect(notice?.body).not.toContain('still settling');
  });

  it('points superseded v5 keys at current-key retrieval instead of retrying', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'exchange_stale_key',
      recommended_action: 'retrieve_current_key',
    } as never);
    expect(notice).toMatchObject({
      title: 'v5 license migration needs attention',
      tone: expect.stringContaining('red'),
    });
    expect(notice?.body).toContain('superseded by a renewal');
    expect(notice?.body).toContain('pulserelay.pro/retrieve-license');
    expect(notice?.body).not.toContain('Retry with the original v5 Pro/Lifetime key');
  });

  it('keeps generic rejected-key copy distinct while offering safe license retrieval', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'exchange_invalid',
      recommended_action: 'enter_supported_v5_key',
    } as never);
    expect(notice).toMatchObject({
      title: 'v5 license migration needs attention',
      tone: expect.stringContaining('red'),
    });
    expect(notice?.body).toContain('rejected during v6 migration');
    expect(notice?.body).toContain('pulserelay.pro/retrieve-license');
    expect(notice?.body).toContain('Retry with the original v5 Pro/Lifetime key');
  });

  it('renders sustained transport failure as blocked-egress policy, not ordinary pending', () => {
    const notice = getCommercialMigrationNotice({
      state: 'pending',
      reason: 'exchange_connectivity_required',
      recommended_action: 'allow_license_egress',
    } as never);
    expect(notice).toMatchObject({
      title: 'v5 license migration pending',
      tone: expect.stringContaining('amber'),
    });
    expect(notice?.body).toContain('over a day');
    expect(notice?.body).toContain('Paid v6 features require periodic outbound HTTPS');
    expect(notice?.body).toContain('Core monitoring keeps running');
    expect(notice?.body).toContain('license.pulserelay.pro');
    expect(notice?.body).toContain('docs/UPGRADE_v6.md');
  });

  it('renders an unreadable persisted v5 license as terminal with re-enter-key guidance', () => {
    const notice = getCommercialMigrationNotice({
      state: 'failed',
      reason: 'persisted_license_unreadable',
      recommended_action: 'enter_supported_v5_key',
    } as never);
    expect(notice).toMatchObject({
      title: 'v5 license migration needs attention',
      tone: expect.stringContaining('red'),
    });
    expect(notice?.body).toContain('could not be read on this system');
    expect(notice?.body).toContain('Retry with the original v5 Pro/Lifetime key');
  });

  it('returns grandfathered recurring price continuity notices only for active recurring v5 plans', () => {
    expect(isGrandfatheredRecurringV5PlanVersion('v5_pro_monthly_grandfathered')).toBe(true);
    expect(isGrandfatheredRecurringV5PlanVersion('v5_pro_annual_grandfathered')).toBe(true);
    expect(isGrandfatheredRecurringV5PlanVersion('v5_lifetime_grandfathered')).toBe(false);
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_monthly_grandfathered', 'active'),
    ).toMatchObject({
      title: 'Grandfathered v5 pricing',
      tone: expect.stringContaining('green'),
      body: expect.stringContaining('keeps its existing recurring price until you cancel'),
    });
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_monthly_grandfathered', 'active')?.body,
    ).toContain('Self-hosted monitoring and child-resource volume are not metered');
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_monthly_grandfathered', 'active')?.body,
    ).not.toContain('guest capacity');
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_annual_grandfathered', 'grace'),
    ).toMatchObject({
      title: 'Grandfathered v5 pricing',
    });
    expect(getGrandfatheredPriceContinuityNotice('v5_lifetime_grandfathered', 'active')).toBeNull();
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_monthly_grandfathered', 'expired'),
    ).toBeNull();
  });

  it('builds entitlement-first current-plan presentation for community, paid, and grandfathered installs', () => {
    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'free',
          subscription_state: 'expired',
          capabilities: [],
          limits: [],
          upgrade_reasons: [],
        },
        displayableCapabilities: [],
      }),
    ).toEqual({
      title: 'Current plan: Community',
      body: 'Community is active on this instance. It includes self-hosted monitoring, 7-day metric history, watch-only Patrol, update alerts, and SSO.',
      unlockedFeaturesLabel: 'Included on this instance',
      unlockedFeatures: [
        'Real-time monitoring',
        '7-day metric history',
        'Watch-only Patrol',
        'Update alerts',
      ],
      includedExtras: [],
      supplementalBadges: [],
      supplementalSummary: '',
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          plan_version: 'legacy_migration_fallback',
          capabilities: ['relay', 'ai_autofix'],
          limits: [{ key: 'max_monitored_systems', limit: 10, current: 23, state: 'enforced' }],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Patrol Applies Safe Fixes and Verifies the Result'],
      }),
    ).toMatchObject({
      title: 'Current plan: Pulse Pro',
      body: 'Pulse Pro is active on this instance. It includes Patrol modes (Ask first, Safe auto-fix, Autopilot), 90-day metric history, RBAC, audit logging, reporting, and agent profiles.',
      supplementalBadges: [],
      supplementalSummary: '',
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
      }),
    ).toEqual({
      title: 'Current plan: Pulse Pro',
      body: 'Pulse Pro is active on this instance. It includes Patrol modes (Ask first, Safe auto-fix, Autopilot), 90-day metric history, RBAC, audit logging, reporting, and agent profiles.',
      unlockedFeaturesLabel: 'Primary capabilities',
      unlockedFeatures: [
        'Patrol Investigates Issues and Explains the Root Cause',
        'Patrol Applies Safe Fixes and Verifies the Result',
        '90-day metric history',
      ],
      includedExtrasLabel: 'Included extras',
      includedExtras: [
        'Role-Based Access Control (RBAC)',
        'Audit Logging',
        'PDF/CSV Reporting',
        'Centralized Agent Profiles',
      ],
      supplementalBadges: [],
      supplementalSummary: '',
      patrolControlAction: {
        actionLabel: 'Choose Patrol mode',
        actionUrl: PATROL_CONTROL_STARTER_URL,
        actionIntent: 'patrol_control',
      },
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        patrolOperatorStatus: {
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 0,
          patrolAutonomyResolvedOperationsLoopCount: 0,
          externalAgentReady: false,
        },
      }),
    ).toMatchObject({
      body: 'Pulse Pro is active on this instance. Open Patrol to continue current work.',
      patrolControlAction: {
        actionLabel: 'Open Patrol',
        actionUrl: PATROL_CONTROL_STARTER_URL,
        actionIntent: 'patrol_control',
      },
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        patrolOperatorStatus: {
          proActivationOperationsLoopStarterCount: 1,
          proActivationCompletedOperationsLoopCount: 0,
          proActivationResolvedOperationsLoopCount: 0,
          proActivationValueProofState: 'in_progress',
          externalAgentReady: false,
        },
      }).patrolControlAction,
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        patrolOperatorStatus: {
          patrolControlOperationsLoopStarterCount: 1,
          proActivationOperationsLoopStarterCount: 1,
          externalAgentReady: false,
        },
      }).patrolControlAction,
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        patrolOperatorStatus: {
          patrolControlOperationsLoopStarterCount: 2,
          proActivationOperationsLoopStarterCount: 1,
          externalAgentReady: false,
        },
      }).patrolControlAction,
    ).toMatchObject({
      actionLabel: 'Open Patrol',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        patrolOperatorStatus: {
          patrolControlOperationsLoopStarterCount: 1,
          patrolControlCompletedOperationsLoopCount: 1,
          patrolControlResolvedOperationsLoopCount: 1,
          patrolControlValueState: 'verified',
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 1,
          patrolAutonomyResolvedOperationsLoopCount: 0,
          patrolAutonomyValueState: 'governed_decision_recorded',
          externalAgentReady: true,
        },
      }).patrolControlAction,
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });
    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        patrolOperatorStatus: {
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 1,
          patrolAutonomyResolvedOperationsLoopCount: 1,
          patrolAutonomyValueState: 'verified',
          externalAgentReady: true,
        },
      }).patrolControlAction,
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        patrolOperatorStatus: {
          nextAction: 'complete',
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 1,
          patrolAutonomyResolvedOperationsLoopCount: 0,
          patrolAutonomyValueState: 'governed_decision_recorded',
          externalAgentReady: true,
        },
      }).patrolControlAction,
    ).toMatchObject({
      actionLabel: 'Review Patrol decision',
      actionUrl: PATROL_CONTROL_PATH,
      actionIntent: 'patrol_control',
    });
    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        patrolOperatorStatus: {
          nextAction: 'open_mcp',
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 1,
          patrolAutonomyResolvedOperationsLoopCount: 0,
          patrolAutonomyValueState: 'verified_needs_mcp',
          externalAgentReady: false,
        },
      }).patrolControlAction,
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro_plus',
          subscription_state: 'active',
          capabilities: ['relay', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Patrol Applies Safe Fixes and Verifies the Result'],
      }),
    ).toMatchObject({
      title: 'Current plan: Legacy Pulse Pro+',
      body: 'Legacy Pulse Pro+ is active on this instance. It includes Patrol modes (Ask first, Safe auto-fix, Autopilot), 90-day metric history, RBAC, audit logging, reporting, and agent profiles.',
      unlockedFeaturesLabel: 'Primary capabilities',
      unlockedFeatures: [
        'Patrol Investigates Issues and Explains the Root Cause',
        'Patrol Applies Safe Fixes and Verifies the Result',
        '90-day metric history',
      ],
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          plan_version: 'v5_pro_monthly_grandfathered',
          capabilities: ['relay'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: ['Pulse Relay (Remote Access)'],
      }),
    ).toEqual({
      title: 'Current plan: Pulse Pro',
      body: 'Pulse Pro is active on this instance. It includes Patrol modes (Ask first, Safe auto-fix, Autopilot), 90-day metric history, RBAC, audit logging, reporting, and agent profiles.',
      unlockedFeaturesLabel: 'Primary capabilities',
      unlockedFeatures: [
        'Patrol Investigates Issues and Explains the Root Cause',
        'Patrol Applies Safe Fixes and Verifies the Result',
        '90-day metric history',
      ],
      includedExtrasLabel: 'Included extras',
      includedExtras: [
        'Role-Based Access Control (RBAC)',
        'Audit Logging',
        'PDF/CSV Reporting',
        'Centralized Agent Profiles',
      ],
      supplementalBadges: ['Grandfathered price'],
      supplementalSummary:
        'This migrated v5 subscription keeps its existing recurring price until cancellation. Self-hosted monitoring and child-resource volume are not metered in current v6 self-hosted packaging.',
    });
  });

  it('surfaces active Pro licenses that are running on the community runtime', () => {
    const entitlements = {
      tier: 'pro',
      subscription_state: 'active',
      capabilities: ['relay', 'audit_logging', 'rbac', 'ai_autofix'],
      limits: [],
      upgrade_reasons: [],
      runtime: {
        build: 'community',
        label: 'Pulse Community runtime',
      },
      max_history_days: 90,
    };

    expect(hasPulseProRuntimeMismatch(entitlements)).toBe(true);
    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements,
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Audit Logging'],
      }),
    ).toMatchObject({
      title: 'Current plan: Pulse Pro',
      body: expect.stringContaining('private Pulse Pro runtime'),
      supplementalBadges: ['Pro runtime missing'],
      privateRuntimeAction: {
        actionLabel: 'Open Pulse Pro downloads',
        actionUrl: 'https://pulserelay.pro/download.html',
      },
    });
    expect(getSelfHostedPlanStatusPresentation(entitlements)?.items[0]).toMatchObject({
      label: 'Pulse Pro runtime',
      statusLabel: 'Needs attention',
      state: 'missing',
      detail: expect.stringContaining('public Docker image'),
    });
    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements,
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Audit Logging'],
        source: 'manual',
      }),
    ).toMatchObject({
      title: 'Pulse Pro license is active',
      tone: expect.stringContaining('amber'),
      body: expect.stringContaining('community runtime'),
      highlightsLabel: 'Licensed capabilities',
      actionLabel: 'Open Pulse Pro downloads',
      actionUrl: 'https://pulserelay.pro/download.html',
    });
  });

  it('surfaces active Pro licenses when runtime identity is missing', () => {
    const entitlements = {
      tier: 'pro',
      subscription_state: 'active',
      capabilities: ['relay', 'audit_logging', 'rbac', 'ai_autofix'],
      limits: [],
      upgrade_reasons: [],
      max_history_days: 90,
    };

    expect(hasPulseProRuntimeMismatch(entitlements)).toBe(true);
    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements,
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Audit Logging'],
      }),
    ).toMatchObject({
      title: 'Current plan: Pulse Pro',
      body: expect.stringContaining('not reporting the private Pulse Pro runtime'),
      supplementalBadges: ['Pro runtime missing'],
      privateRuntimeAction: {
        actionLabel: 'Open Pulse Pro downloads',
        actionUrl: 'https://pulserelay.pro/download.html',
      },
    });
    expect(getSelfHostedPlanStatusPresentation(entitlements)?.items[0]).toMatchObject({
      label: 'Pulse Pro runtime',
      statusLabel: 'Needs attention',
      state: 'missing',
      detail: expect.stringContaining('not reporting a Pulse Pro runtime identity'),
    });
    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements,
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Audit Logging'],
        source: 'manual',
      }),
    ).toMatchObject({
      title: 'Pulse Pro license is active',
      tone: expect.stringContaining('amber'),
      body: expect.stringContaining('not reporting the private Pulse Pro runtime'),
      highlightsLabel: 'Licensed capabilities',
      actionLabel: 'Open Pulse Pro downloads',
      actionUrl: 'https://pulserelay.pro/download.html',
    });
  });

  it('sources active self-hosted current-plan summaries from the shared plan contract', () => {
    expect(
      getSelfHostedCurrentPlanPresentation({
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
      }).body,
    ).toBe(getSelfHostedPlanEntitlementSummary('relay', 'Relay'));

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro_plus',
          subscription_state: 'active',
          capabilities: ['relay', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Patrol Applies Safe Fixes and Verifies the Result'],
      }).body,
    ).toBe(getSelfHostedPlanEntitlementSummary('pro', 'Legacy Pulse Pro+'));
  });

  it('keeps the Patrol mode entry point on active Pro current-plan summaries', () => {
    const patrolControlAction = {
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    };

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Patrol Applies Safe Fixes and Verifies the Result'],
      }).patrolControlAction,
    ).toEqual(patrolControlAction);

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'trial',
          capabilities: ['ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
      }).patrolControlAction,
    ).toEqual(patrolControlAction);

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'community', label: 'Pulse Community runtime' },
        },
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
      }).patrolControlAction,
    ).toBeUndefined();

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'relay',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Push Notifications',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
      }).patrolControlAction,
    ).toBeUndefined();
  });

  it('builds restrained higher-tier comparison cards for Community and Relay only', () => {
    expect(
      getSelfHostedPlanComparisonPresentation({
        entitlements: {
          tier: 'free',
          subscription_state: 'expired',
          capabilities: [],
          limits: [],
          upgrade_reasons: [],
        },
      }),
    ).toEqual({
      cards: [
        {
          title: 'Relay plan',
          body: 'Remote web access, Pulse Mobile pairing, push notifications, and 14-day metric history.',
          highlights: [
            'Everything in Community',
            'Remote web access via Relay',
            'Pulse Mobile pairing',
            'Push notifications',
            'No inbound ports required',
            '14-day metric history',
          ],
        },
        {
          title: 'Pulse Pro plan',
          body: 'Patrol investigates issues, applies safe fixes, and verifies the result, plus 90-day metric history and team controls.',
          highlights: [
            'Everything in Relay',
            'Patrol modes: Ask first, Safe auto-fix, or Autopilot',
            'Patrol investigates issues and explains the root cause',
            'Patrol applies safe fixes and verifies the result',
            '90-day metric history',
            'Team controls: RBAC, audit logging, reporting, and agent profiles',
          ],
        },
      ],
    });

    expect(
      getSelfHostedPlanComparisonPresentation({
        entitlements: {
          tier: 'relay',
          subscription_state: 'active',
          capabilities: ['relay'],
          limits: [],
          upgrade_reasons: [],
        },
      }),
    ).toEqual({
      cards: [
        {
          title: 'Pulse Pro plan',
          body: 'Patrol investigates issues, applies safe fixes, and verifies the result, plus 90-day metric history and team controls.',
          highlights: [
            'Everything in Relay',
            'Patrol modes: Ask first, Safe auto-fix, or Autopilot',
            'Patrol investigates issues and explains the root cause',
            'Patrol applies safe fixes and verifies the result',
            '90-day metric history',
            'Team controls: RBAC, audit logging, reporting, and agent profiles',
          ],
        },
      ],
    });

    expect(
      getSelfHostedPlanComparisonPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['ai_patrol'],
          limits: [],
          upgrade_reasons: [],
        },
      }),
    ).toEqual({ cards: [] });
  });

  it('builds entitlement-derived status for active Relay and Pro installs', () => {
    expect(
      getSelfHostedPlanStatusPresentation({
        tier: 'relay',
        subscription_state: 'active',
        capabilities: ['relay', 'mobile_app', 'push_notifications', 'long_term_metrics'],
        limits: [],
        upgrade_reasons: [],
        max_history_days: 14,
      }),
    ).toEqual({
      title: 'Relay status',
      body: 'These checks show the capabilities this instance can use right now, based on its entitlement and runtime payloads.',
      items: [
        {
          label: 'Remote access, pairing, and push',
          statusLabel: 'Active',
          state: 'active',
          detail:
            'Relay, Pulse Mobile pairing, and push notifications are available on this instance.',
        },
        {
          label: '14-day metric history',
          statusLabel: 'Active',
          state: 'active',
          detail: 'This instance has 14 days of metric history available.',
        },
      ],
    });

    expect(
      getSelfHostedPlanStatusPresentation({
        tier: 'pro',
        subscription_state: 'active',
        capabilities: [
          'relay',
          'mobile_app',
          'push_notifications',
          'long_term_metrics',
          'ai_alerts',
          'ai_autofix',
          'advanced_sso',
          'rbac',
          'audit_logging',
          'advanced_reporting',
          'agent_profiles',
        ],
        limits: [],
        upgrade_reasons: [],
        max_history_days: 90,
        runtime: { build: 'pro', label: 'Pulse Pro runtime' },
      }),
    ).toMatchObject({
      title: 'Capability details',
      body: 'Open this only when a Pro capability looks unavailable. Normal setup is choosing Patrol mode.',
      items: [
        { label: 'Pulse Pro runtime', statusLabel: 'Active' },
        { label: 'Remote access, pairing, and push', statusLabel: 'Active' },
        { label: '90-day metric history', statusLabel: 'Active' },
        { label: 'Patrol investigation and remediation', statusLabel: 'Active' },
        { label: 'Team controls', statusLabel: 'Active' },
      ],
    });

    const partialProStatus = getSelfHostedPlanStatusPresentation({
      tier: 'pro',
      subscription_state: 'active',
      capabilities: ['relay', 'ai_autofix'],
      limits: [],
      upgrade_reasons: [],
      max_history_days: 14,
      runtime: { build: 'pro', label: 'Pulse Pro runtime' },
    });
    expect(partialProStatus?.items.map((item) => [item.label, item.statusLabel])).toEqual([
      ['Pulse Pro runtime', 'Active'],
      ['Remote access, pairing, and push', 'Partial'],
      ['90-day metric history', 'Partial'],
      ['Patrol investigation and remediation', 'Partial'],
      ['Team controls', 'Needs attention'],
    ]);
    expect(partialProStatus?.items).toContainEqual({
      label: 'Patrol investigation and remediation',
      state: 'partial',
      statusLabel: 'Partial',
      detail:
        'Some Patrol capability is available, but investigation or remediation may not be fully enabled yet.',
    });
    expect(partialProStatus?.items).toContainEqual({
      label: 'Team controls',
      state: 'missing',
      statusLabel: 'Needs attention',
      detail:
        'Team controls are not available yet. Refresh the plan or open recovery before relying on this Pro install.',
    });
    expect(partialProStatus?.items.map((item) => item.detail).join(' ')).not.toContain(
      'admin tools',
    );

    expect(
      getSelfHostedPlanStatusPresentation({
        tier: 'free',
        subscription_state: 'expired',
        capabilities: [],
        limits: [],
        upgrade_reasons: [],
      }),
    ).toBeNull();
  });

  it('keeps Patrol work state out of plan capability details', () => {
    const proEntitlements = {
      tier: 'pro',
      subscription_state: 'active',
      capabilities: [
        'relay',
        'mobile_app',
        'push_notifications',
        'ai_alerts',
        'ai_autofix',
        'rbac',
        'audit_logging',
        'advanced_reporting',
        'agent_profiles',
      ],
      limits: [],
      upgrade_reasons: [],
      max_history_days: 90,
      runtime: { build: 'pro', label: 'Pulse Pro runtime' },
    };

    const expectedCapabilityItems = [
      ['Pulse Pro runtime', 'Active'],
      ['Remote access, pairing, and push', 'Active'],
      ['90-day metric history', 'Active'],
      ['Patrol investigation and remediation', 'Active'],
      ['Team controls', 'Active'],
    ];
    const expectOnlyCapabilityItems = (
      status?: ReturnType<typeof getSelfHostedPlanStatusPresentation>,
    ) => {
      expect(status?.title).toBe('Capability details');
      expect(status?.items.map((item) => [item.label, item.statusLabel])).toEqual(
        expectedCapabilityItems,
      );
      expect(status?.items.some((item) => item.label === 'Patrol work')).toBe(false);
    };

    expect(
      getSelfHostedPlanStatusPresentation(proEntitlements)?.items.some(
        (item) => item.label === 'Patrol work',
      ),
    ).toBe(false);

    expectOnlyCapabilityItems(getSelfHostedPlanStatusPresentation(proEntitlements));

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        patrolControlOperationsLoopStarterCount: 1,
        patrolControlCompletedOperationsLoopCount: 1,
        patrolControlResolvedOperationsLoopCount: 1,
        patrolControlValueState: 'verified',
        patrolAutonomyOperationsLoopStarterCount: 1,
        patrolAutonomyCompletedOperationsLoopCount: 1,
        patrolAutonomyResolvedOperationsLoopCount: 0,
        patrolAutonomyValueState: 'governed_decision_recorded',
        externalAgentReady: true,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        patrolAutonomyOperationsLoopStarterCount: 1,
        patrolAutonomyCompletedOperationsLoopCount: 1,
        patrolAutonomyResolvedOperationsLoopCount: 1,
        patrolAutonomyValueState: 'verified',
        externalAgentReady: true,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        nextAction: 'complete',
        progressLabel:
          'Patrol recorded a rejected change decision. Nothing was changed; approve a safer fix before marking the issue resolved.',
        patrolAutonomyOperationsLoopStarterCount: 1,
        patrolAutonomyCompletedOperationsLoopCount: 1,
        patrolAutonomyResolvedOperationsLoopCount: 0,
        patrolAutonomyValueState: 'governed_decision_recorded',
        externalAgentReady: true,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        nextAction: 'complete',
        patrolAutonomyOperationsLoopStarterCount: 1,
        patrolAutonomyCompletedOperationsLoopCount: 1,
        patrolAutonomyResolvedOperationsLoopCount: 0,
        patrolAutonomyValueState: 'governed_decision_recorded',
        externalAgentReady: false,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        nextAction: 'open_mcp',
        progressLabel: 'Legacy status asked for MCP readiness after the outcome was verified.',
        patrolAutonomyOperationsLoopStarterCount: 1,
        patrolAutonomyCompletedOperationsLoopCount: 0,
        patrolAutonomyResolvedOperationsLoopCount: 0,
        patrolAutonomyValueState: 'verified_needs_mcp',
        externalAgentReady: false,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        patrolAutonomyOperationsLoopStarterCount: 2,
        patrolAutonomyCompletedOperationsLoopCount: 0,
        patrolAutonomyResolvedOperationsLoopCount: 0,
        progressLabel: 'Investigation ready',
        externalAgentReady: false,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        proActivationOperationsLoopStarterCount: 1,
        proActivationCompletedOperationsLoopCount: 0,
        proActivationResolvedOperationsLoopCount: 0,
        proActivationValueProofState: 'in_progress',
        externalAgentReady: false,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        patrolControlOperationsLoopStarterCount: 1,
        proActivationOperationsLoopStarterCount: 1,
        progressLabel: 'Legacy activation entry recorded',
        externalAgentReady: false,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        patrolControlOperationsLoopStarterCount: 2,
        proActivationOperationsLoopStarterCount: 1,
        progressLabel: 'Patrol mode started',
        externalAgentReady: false,
      }),
    );

    expectOnlyCapabilityItems(
      getSelfHostedPlanStatusPresentation(proEntitlements, {
        patrolAutonomyOperationsLoopStarterCount: 0,
        patrolAutonomyCompletedOperationsLoopCount: 0,
        patrolAutonomyResolvedOperationsLoopCount: 0,
        externalAgentReady: false,
      }),
    );
  });

  it('builds activation-success summaries for purchase and pasted-key paths', () => {
    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
          runtime: { build: 'pro', label: 'Pulse Pro runtime' },
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Pulse Mobile Pairing',
          'Push Notifications',
          'Patrol Applies Safe Fixes and Verifies the Result',
        ],
        source: 'purchase',
      }),
    ).toEqual({
      tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
      title: 'Pulse Pro is now active',
      body: 'Checkout completed and Pulse Pro is active. Choose Patrol mode.',
      highlightsLabel: 'Available now on this instance',
      highlights: [
        'Patrol Investigates Issues and Explains the Root Cause',
        'Patrol Applies Safe Fixes and Verifies the Result',
        '90-day metric history',
        'Role-Based Access Control (RBAC)',
        'Audit Logging',
        'PDF/CSV Reporting',
        'Centralized Agent Profiles',
      ],
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedActivationSuccessPresentation({
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
        source: 'manual',
      }),
    ).toEqual({
      tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
      title: 'Relay is now active',
      body: 'The license key was accepted and this instance is now running Relay.',
      highlightsLabel: 'Available now on this instance',
      highlights: [
        'Pulse Relay (Remote Access)',
        'Pulse Mobile Pairing',
        'Push Notifications',
        '14-day metric history',
      ],
    });

    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'trial',
          capabilities: ['ai_patrol', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
        },
        displayableCapabilities: ['Pulse Patrol', 'Patrol Applies Safe Fixes and Verifies the Result'],
        source: 'purchase',
      }),
    ).toBeNull();

    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: {
          tier: 'free',
          subscription_state: 'expired',
          capabilities: [],
          limits: [],
          upgrade_reasons: [],
        },
        displayableCapabilities: [],
        source: 'purchase',
      }),
    ).toBeNull();
  });

  it('keeps activation-success actions aligned with actionable Patrol states', () => {
    const proEntitlements = {
      tier: 'pro',
      subscription_state: 'active',
      capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
      limits: [],
      upgrade_reasons: [],
      runtime: { build: 'pro', label: 'Pulse Pro runtime' },
    };

    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: proEntitlements,
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
        patrolOperatorStatus: {
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 0,
          patrolAutonomyResolvedOperationsLoopCount: 0,
          externalAgentReady: false,
        },
        source: 'manual',
      }),
    ).toMatchObject({
      actionLabel: 'Open Patrol',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: proEntitlements,
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
        patrolOperatorStatus: {
          proActivationOperationsLoopStarterCount: 1,
          proActivationCompletedOperationsLoopCount: 0,
          proActivationResolvedOperationsLoopCount: 0,
          proActivationValueProofState: 'in_progress',
          externalAgentReady: false,
        },
        source: 'manual',
      }),
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: proEntitlements,
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
        patrolOperatorStatus: {
          patrolControlOperationsLoopStarterCount: 1,
          proActivationOperationsLoopStarterCount: 1,
          externalAgentReady: false,
        },
        source: 'manual',
      }),
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: proEntitlements,
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
        patrolOperatorStatus: {
          nextAction: 'open_mcp',
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 0,
          patrolAutonomyResolvedOperationsLoopCount: 0,
          patrolAutonomyValueState: 'verified_needs_mcp',
          externalAgentReady: false,
        },
        source: 'manual',
      }),
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: proEntitlements,
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
        patrolOperatorStatus: {
          nextAction: 'complete',
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 1,
          patrolAutonomyResolvedOperationsLoopCount: 0,
          patrolAutonomyValueState: 'governed_decision_recorded',
          externalAgentReady: true,
        },
        source: 'manual',
      }),
    ).toMatchObject({
      actionLabel: 'Review Patrol decision',
      actionUrl: PATROL_CONTROL_PATH,
      actionIntent: 'patrol_control',
    });

    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: proEntitlements,
        displayableCapabilities: ['Patrol Applies Safe Fixes and Verifies the Result'],
        patrolOperatorStatus: {
          patrolAutonomyOperationsLoopStarterCount: 1,
          patrolAutonomyCompletedOperationsLoopCount: 1,
          patrolAutonomyResolvedOperationsLoopCount: 1,
          patrolAutonomyValueState: 'verified',
          externalAgentReady: true,
        },
        source: 'manual',
      }),
    ).toMatchObject({
      actionLabel: 'Choose Patrol mode',
      actionUrl: PATROL_CONTROL_STARTER_URL,
      actionIntent: 'patrol_control',
    });
  });

  it('returns canonical purchase activation notices', () => {
    expect(getPurchaseActivationNotice('activated')).toMatchObject({
      title: 'Plan activated',
      tone: expect.stringContaining('green'),
    });
    expect(getPurchaseActivationNotice('cancelled')).toMatchObject({
      title: 'Checkout cancelled',
      body: expect.stringContaining('start the upgrade again'),
      tone: expect.stringContaining('amber'),
    });
    expect(getPurchaseActivationNotice('expired')).toMatchObject({
      title: 'Upgrade return expired',
      body: expect.stringContaining('Start the upgrade again'),
    });
    expect(getPurchaseActivationNotice('failed')).toMatchObject({
      title: 'Plan needs attention',
      body: expect.stringContaining('open recovery'),
      tone: expect.stringContaining('red'),
    });
    expect(getPurchaseActivationNotice('unavailable')).toMatchObject({
      title: 'Pulse Account unavailable',
      body: expect.stringContaining('Retry from this instance'),
      tone: expect.stringContaining('amber'),
    });
    expect(getPurchaseActivationNotice('')).toBeNull();
  });

  it('returns canonical organization and billing-admin billing vocabulary', () => {
    expect(getOrganizationBillingLicenseStatusLabel({ valid: false, in_grace_period: false })).toBe(
      'No License',
    );
    expect(getOrganizationBillingLicenseStatusLabel({ valid: true, in_grace_period: true })).toBe(
      'Grace Period',
    );
    expect(
      getBillingAdminTrialStatus({
        subscription_state: 'trial',
        trial_ends_at: 1_700_000_000,
      } as never),
    ).toContain('Trial (ends');
    expect(getBillingAdminTrialStatus({ subscription_state: 'active' } as never)).toBe('No trial');
    expect(
      getBillingAdminOrganizationBadges({ soft_deleted: true, suspended: true } as never),
    ).toMatchObject([{ label: 'soft-deleted' }]);
    expect(
      getBillingAdminOrganizationBadges({ soft_deleted: false, suspended: true } as never),
    ).toMatchObject([{ label: 'suspended' }]);
    expect(getBillingAdminStateUpdateSuccessMessage('active')).toBe(
      'Organization billing activated',
    );
    expect(BILLING_ADMIN_EMPTY_STATE).toBe('No organizations found.');
  });
});
