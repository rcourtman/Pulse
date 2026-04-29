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
  getSelfHostedActivationSuccessPresentation,
  getSelfHostedPlanComparisonPresentation,
  getSelfHostedCurrentPlanPresentation,
  getSelfHostedCurrentPlanStatusPresentation,
  getLicenseFeatureLabel,
  getLicenseStatusLoadingState,
  getLicenseSubscriptionStatusPresentation,
  getSelfHostedPlanLabel,
  getLicenseTierLabel,
  getDisplayableMonitoredSystemContinuity,
  getMonitoredSystemContinuityNotice,
  getNoActiveSelfHostedActivationState,
  getOrganizationBillingLicenseStatusLabel,
  getPurchaseActivationNotice,
  isDisplayableLicenseFeature,
  isGrandfatheredRecurringV5PlanVersion,
  isUncappedGrandfatheredPlanVersion,
  SELF_HOSTED_RECOVERY_PRESENTATION,
} from '@/utils/licensePresentation';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from '@/components/Settings/selfHostedBillingPresentation';

describe('licensePresentation', () => {
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
      disclosureLabel: 'Use existing key',
      fieldLabel: 'License or Activation Key',
      activateIdleLabel: 'Activate Key',
      clearIdleLabel: 'Clear Key',
      legacyNotice: {
        title: 'Legacy v5 license detected',
      },
    });
    expect(SELF_HOSTED_PRO_BILLING_PRESENTATION).toEqual({
      navLabel: 'Plans',
      shellTitle: 'Self-hosted plan',
      shellDescription:
        'Review the plan this instance is using and the optional capabilities connected to it.',
      infrastructureRouteReferral: 'Billing and self-hosted plan changes live in Plans.',
      infrastructureWorkspaceReferral:
        'Self-hosted plan status, optional activation, and available capabilities live in Plans, not here.',
      sectionSelectorAriaLabel: 'Self-hosted plans section',
      refreshLabel: 'Refresh',
      planTabLabel: 'Plan',
      usageTabLabel: 'Usage',
      planSectionTitle: 'Current plan',
      planSectionDescription:
        'See which self-hosted tier this instance is using and which capabilities are available on this install.',
      planComparisonSectionTitle: 'Optional extras',
      planComparisonActionLabel: 'See all plans',
      usageSectionTitle: 'Usage',
      hiddenShellTitle: 'Demo mode',
      hiddenShellDescription: 'Commercial settings are hidden for this session.',
      hiddenStateTitle: 'License and billing details are hidden',
      hiddenStateBody:
        'This public demo uses sample infrastructure data, so Pulse hides license identity, billing state, monitored-system usage, and upgrade actions instead of creating a demo license.',
      policyLoadingTitle: 'Loading settings access',
      policyLoadingBody:
        'Pulse waits for the session presentation policy before showing license, billing, or usage details.',
      planSelectionPromptTitle: 'Compare self-hosted plans',
      planSelectionPromptBody:
        'Community includes core monitoring at no cost. Relay is optional for secure access from anywhere, and Pulse Pro adds root-cause analysis, safe remediation workflows, and 90-day history.',
      planSelectionPromptActionLabel: 'Compare plans',
      purchaseActivatedPlanActionLabel: 'Review plan',
      purchaseCancelledActionLabel: 'Compare plans',
      purchaseExpiredActionLabel: 'Compare plans',
      purchaseFailedActionLabel: 'Open recovery',
      purchaseUnavailableActionLabel: 'Try again',
      recoverySectionTitle: 'Existing purchases',
      recoverySectionDescription:
        'Add an activation key you already have, recover a previous self-hosted purchase, or clear a local key from this instance.',
    });
  });

  it('returns canonical feature labels', () => {
    expect(getLicenseFeatureLabel('ai_patrol')).toBe('Pulse Patrol');
    expect(getLicenseFeatureLabel('mobile_app')).toBe('Mobile App Access');
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

  it('returns grandfathered recurring price continuity notices only for active recurring v5 plans', () => {
    expect(isGrandfatheredRecurringV5PlanVersion('v5_pro_monthly_grandfathered')).toBe(true);
    expect(isGrandfatheredRecurringV5PlanVersion('v5_pro_annual_grandfathered')).toBe(true);
    expect(isGrandfatheredRecurringV5PlanVersion('v5_lifetime_grandfathered')).toBe(false);
    expect(isUncappedGrandfatheredPlanVersion('v5_pro_monthly_grandfathered', false)).toBe(true);
    expect(isUncappedGrandfatheredPlanVersion('v5_pro_annual_grandfathered', false)).toBe(true);
    expect(isUncappedGrandfatheredPlanVersion(undefined, true)).toBe(true);
    expect(isUncappedGrandfatheredPlanVersion('pro', false)).toBe(false);
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

  it('hides monitored-system continuity when self-hosted effective capacity is uncapped', () => {
    expect(
      getDisplayableMonitoredSystemContinuity({
        continuity: {
          plan_limit: 10,
          effective_limit: 0,
          capture_pending: true,
        },
        planVersion: 'legacy_migration_fallback',
        isLifetime: false,
        subscriptionState: 'active',
      }),
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
      body: 'Community is active on this instance. It includes self-hosted monitoring, 7-day metric history, Pulse Patrol (BYOK), and update alerts.',
      unlockedFeaturesLabel: 'Included on this instance',
      unlockedFeatures: [
        'Real-time monitoring',
        '7-day metric history',
        'Pulse Patrol (BYOK)',
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
          monitored_system_continuity: {
            plan_limit: 10,
            grandfathered_floor: 23,
            effective_limit: 23,
            capture_pending: false,
          },
        },
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Safe Remediation Workflows'],
      }),
    ).toMatchObject({
      title: 'Current plan: Pulse Pro',
      body: 'Pulse Pro is active on this instance. Root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras are available right now.',
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
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Mobile App Access',
          'Safe Remediation Workflows',
        ],
      }),
    ).toEqual({
      title: 'Current plan: Pulse Pro',
      body: 'Pulse Pro is active on this instance. Root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras are available right now.',
      unlockedFeaturesLabel: 'Primary capabilities',
      unlockedFeatures: [
        'Alert Root-Cause Analysis',
        'Safe Remediation Workflows',
        '90-day metric history',
      ],
      includedExtrasLabel: 'Included extras',
      includedExtras: [
        'Advanced SSO (SAML/Multi-Provider)',
        'Role-Based Access Control (RBAC)',
        'Audit Logging',
        'PDF/CSV Reporting',
        'Centralized Agent Profiles',
      ],
      supplementalBadges: [],
      supplementalSummary: '',
    });

    expect(
      getSelfHostedCurrentPlanPresentation({
        entitlements: {
          tier: 'pro_plus',
          subscription_state: 'active',
          capabilities: ['relay', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
        },
        displayableCapabilities: ['Pulse Relay (Remote Access)', 'Safe Remediation Workflows'],
      }),
    ).toMatchObject({
      title: 'Current plan: Legacy Pulse Pro+',
      body: 'Legacy Pulse Pro+ is active on this instance. Root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras are available right now.',
      unlockedFeaturesLabel: 'Primary capabilities',
      unlockedFeatures: [
        'Alert Root-Cause Analysis',
        'Safe Remediation Workflows',
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
          monitored_system_continuity: {
            plan_limit: 10,
            grandfathered_floor: 23,
            effective_limit: 23,
            capture_pending: false,
          },
        },
        displayableCapabilities: ['Pulse Relay (Remote Access)'],
      }),
    ).toEqual({
      title: 'Current plan: Pulse Pro',
      body: 'Pulse Pro is active on this instance. Root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras are available right now.',
      unlockedFeaturesLabel: 'Primary capabilities',
      unlockedFeatures: [
        'Alert Root-Cause Analysis',
        'Safe Remediation Workflows',
        '90-day metric history',
      ],
      includedExtrasLabel: 'Included extras',
      includedExtras: [
        'Advanced SSO (SAML/Multi-Provider)',
        'Role-Based Access Control (RBAC)',
        'Audit Logging',
        'PDF/CSV Reporting',
        'Centralized Agent Profiles',
      ],
      supplementalBadges: ['Grandfathered price'],
      supplementalSummary:
        'This migrated v5 subscription keeps its existing recurring price until cancellation. Self-hosted monitoring and child-resource volume are not metered under the current v6 policy.',
    });
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
          title: 'What Relay adds',
          body: 'Reach this Pulse instance securely from anywhere, check it from mobile, get push notifications, and keep 14 days of history.',
          highlights: [
            'Pulse Relay (Remote Access)',
            'Mobile App Access',
            'Push Notifications',
            '14-day metric history',
          ],
        },
        {
          title: 'What Pulse Pro adds',
          body: 'Add operations features on top of free monitoring: root-cause analysis, safe remediation workflows, 90-day history, SAML SSO, RBAC, audit logging, reporting, and agent profiles.',
          highlights: [
            'Alert Root-Cause Analysis',
            'Safe Remediation Workflows',
            '90-day metric history',
            'Advanced SSO (SAML/Multi-Provider)',
            'Role-Based Access Control (RBAC)',
            'Audit Logging',
            'PDF/CSV Reporting',
            'Centralized Agent Profiles',
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
          title: 'What Pulse Pro adds',
          body: 'Add operations features on top of free monitoring: root-cause analysis, safe remediation workflows, 90-day history, SAML SSO, RBAC, audit logging, reporting, and agent profiles.',
          highlights: [
            'Alert Root-Cause Analysis',
            'Safe Remediation Workflows',
            '90-day metric history',
            'Advanced SSO (SAML/Multi-Provider)',
            'Role-Based Access Control (RBAC)',
            'Audit Logging',
            'PDF/CSV Reporting',
            'Centralized Agent Profiles',
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

  it('builds activation-success summaries for purchase and pasted-key paths', () => {
    expect(
      getSelfHostedActivationSuccessPresentation({
        entitlements: {
          tier: 'pro',
          subscription_state: 'active',
          capabilities: ['relay', 'mobile_app', 'push_notifications', 'ai_autofix'],
          limits: [],
          upgrade_reasons: [],
        },
        displayableCapabilities: [
          'Pulse Relay (Remote Access)',
          'Mobile App Access',
          'Push Notifications',
          'Safe Remediation Workflows',
        ],
        source: 'purchase',
      }),
    ).toEqual({
      tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
      title: 'Pulse Pro is now active',
      body: 'Checkout completed and this instance is now running Pulse Pro.',
      highlightsLabel: 'Available now on this instance',
      highlights: [
        'Alert Root-Cause Analysis',
        'Safe Remediation Workflows',
        '90-day metric history',
        'Advanced SSO (SAML/Multi-Provider)',
        'Role-Based Access Control (RBAC)',
        'Audit Logging',
        'PDF/CSV Reporting',
        'Centralized Agent Profiles',
      ],
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
          'Mobile App Access',
          'Push Notifications',
        ],
        source: 'manual',
      }),
    ).toEqual({
      tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
      title: 'Relay is now active',
      body: 'The activation key was accepted and this instance is now running Relay.',
      highlightsLabel: 'Available now on this instance',
      highlights: [
        'Pulse Relay (Remote Access)',
        'Mobile App Access',
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
        displayableCapabilities: ['Pulse Patrol', 'Safe Remediation Workflows'],
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

  it('returns monitored-system continuity notices for pending verification and captured grandfathering', () => {
    expect(
      getMonitoredSystemContinuityNotice(
        {
          plan_limit: 10,
          effective_limit: 10,
          capture_pending: true,
        },
        {
          current_available: false,
          current_unavailable_reason: 'supplemental_inventory_unsettled',
        },
        {
          mode: 'usage_unavailable',
          urgency: 'ok',
          current: 0,
          limit: 10,
          current_available: false,
          current_unavailable_reason: 'supplemental_inventory_unsettled',
          available_slots: 0,
          overage: 0,
          blocks_new_systems: false,
          existing_monitoring_continues: false,
        },
      ),
    ).toMatchObject({
      title: 'Migration continuity verification pending',
      body: expect.stringContaining('grandfathered monitored-system floor'),
      tone: expect.stringContaining('amber'),
    });
    expect(
      getMonitoredSystemContinuityNotice(
        {
          plan_limit: 10,
          effective_limit: 10,
          capture_pending: true,
        },
        {
          current: 23,
          limit: 10,
          current_available: true,
          state: 'enforced',
        },
        {
          mode: 'over_limit_frozen',
          urgency: 'enforced',
          current: 23,
          limit: 10,
          current_available: true,
          available_slots: 0,
          overage: 13,
          reason: 'legacy_migration_capture_pending',
          blocks_new_systems: true,
          existing_monitoring_continues: true,
        },
      ),
    ).toMatchObject({
      title: 'Migration continuity verification pending',
      body: expect.stringContaining('already monitoring 23'),
      tone: expect.stringContaining('amber'),
    });
    expect(
      getMonitoredSystemContinuityNotice(
        {
          plan_limit: 10,
          grandfathered_floor: 23,
          effective_limit: 23,
          capture_pending: false,
          captured_at: 123,
        },
        {
          current_available: true,
        },
        {
          mode: 'at_limit_blocking_new',
          urgency: 'enforced',
          current: 23,
          limit: 23,
          current_available: true,
          available_slots: 0,
          overage: 0,
          reason: 'limit_reached',
          blocks_new_systems: true,
          existing_monitoring_continues: true,
        },
        {
          planVersion: 'v5_pro_monthly_grandfathered',
          isLifetime: false,
          subscriptionState: 'active',
        },
      ),
    ).toBeNull();
    expect(
      getMonitoredSystemContinuityNotice(
        {
          plan_limit: 10,
          grandfathered_floor: 23,
          effective_limit: 23,
          capture_pending: false,
          captured_at: 123,
        },
        {
          current_available: true,
        },
        {
          mode: 'at_limit_blocking_new',
          urgency: 'enforced',
          current: 23,
          limit: 23,
          current_available: true,
          available_slots: 0,
          overage: 0,
          reason: 'limit_reached',
          blocks_new_systems: true,
          existing_monitoring_continues: true,
        },
      ),
    ).toMatchObject({
      title: 'Grandfathered monitored-system floor',
      tone: expect.stringContaining('green'),
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
      title: 'Activation needs attention',
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
