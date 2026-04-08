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
  getLicenseFeatureLabel,
  getLicenseStatusLoadingState,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
  getMonitoredSystemContinuityNotice,
  getNoActiveProLicenseState,
  getOrganizationBillingLicenseStatusLabel,
  getInactiveProUpsellNotice,
  getPurchaseActivationNotice,
  getTrialEndedProLicenseNotice,
  getTrialActivationNotice,
  isDisplayableLicenseFeature,
  SELF_HOSTED_RECOVERY_PRESENTATION,
} from '@/utils/licensePresentation';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from '@/components/Settings/selfHostedBillingPresentation';

describe('licensePresentation', () => {
  it('returns canonical tier labels', () => {
    expect(getLicenseTierLabel('free')).toBe('Community');
    expect(getLicenseTierLabel('pro_plus')).toBe('Pro+');
    expect(getLicenseTierLabel('enterprise')).toBe('Enterprise');
    expect(getLicenseTierLabel('custom_tier')).toBe('Custom Tier');
  });

  it('returns canonical license loading and inactive copy', () => {
    expect(getLicenseStatusLoadingState()).toEqual({
      text: 'Loading license status...',
    });
    expect(getNoActiveProLicenseState()).toEqual({
      text: 'No Pro license is active.',
    });
    expect(getTrialEndedProLicenseNotice()).toEqual({
      tone: 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 text-red-900 dark:text-red-100',
      title: 'Your Pro trial has ended',
      body: 'Upgrade to keep Pro features.',
      actionLabel: 'View Pro plans',
    });
    expect(getInactiveProUpsellNotice()).toEqual({
      tone: 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200',
      title: 'Upgrade to Pro',
      body: 'Unlock Pulse Patrol, alert analysis, auto-fix, and more.',
      actionLabel: 'View Pro plans',
    });
    expect(SELF_HOSTED_RECOVERY_PRESENTATION).toMatchObject({
      disclosureLabel: 'Redeem existing key',
      fieldLabel: 'Pulse Pro Key',
      activateIdleLabel: 'Activate License',
      clearIdleLabel: 'Clear License',
      legacyNotice: {
        title: 'Legacy v5 license detected',
      },
    });
    expect(SELF_HOSTED_PRO_BILLING_PRESENTATION).toEqual({
      shellTitle: 'Pulse Pro',
      shellDescription:
        'Manage self-hosted billing, monitored-system limits, and Pulse Pro license status.',
      infrastructureRouteReferral: 'Billing and monitored-system limits live in Pulse Pro.',
      infrastructureWorkspaceReferral:
        'Billing, monitored-system limits, and Pulse Pro license status live in Pulse Pro, not here.',
      sectionSelectorAriaLabel: 'Pulse Pro billing section',
      refreshLabel: 'Refresh',
      planTabLabel: 'Plan',
      usageTabLabel: 'Usage',
      planSectionTitle: 'Plan',
      planSectionDescription:
        'Review your active plan, expiry, included limits, and paid capabilities.',
      usageSectionTitle: 'Usage',
      monitoredSystemUpgradeArrivalTitle: 'Need a higher monitored-system cap?',
      monitoredSystemUpgradeArrivalBody:
        'Open Pulse Account to compare self-hosted plans, complete checkout, and return here with Pulse Pro activated automatically.',
      monitoredSystemUpgradeArrivalActionLabel: 'Compare plans',
      trialStartTitle: 'Try Pro for free',
      trialStartBody: 'Start a 14-day Pro trial for this organization.',
      trialStartIdleActionLabel: 'Start 14-day Pro Trial',
      trialStartPendingActionLabel: 'Starting...',
      recoverySectionTitle: 'Recovery',
      recoverySectionDescription:
        'Use recovery tools only when you already have a Pulse Pro key or need to remove a local key from this instance.',
    });
  });

  it('returns canonical feature labels', () => {
    expect(getLicenseFeatureLabel('ai_patrol')).toBe('Pulse Patrol');
    expect(getLicenseFeatureLabel('mobile_app')).toBe('Mobile App Access');
    expect(getLicenseFeatureLabel('custom_feature')).toBe('Custom Feature');
  });

  it('hides non-operable capability labels from customer-facing plan surfaces', () => {
    expect(isDisplayableLicenseFeature('ai_patrol')).toBe(true);
    expect(isDisplayableLicenseFeature('multi_tenant')).toBe(true);
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
  });

  it('formats plan versions and commercial migration notices canonically', () => {
    expect(formatLicensePlanVersion('pro_plus')).toBe('Pro Plus');
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
    expect(
      getGrandfatheredPriceContinuityNotice('v5_pro_monthly_grandfathered', 'active'),
    ).toMatchObject({
      title: 'Grandfathered v5 pricing',
      tone: expect.stringContaining('green'),
    });
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
      ),
    ).toMatchObject({
      title: 'Migration continuity verification pending',
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
      ),
    ).toMatchObject({
      title: 'Grandfathered monitored-system floor',
      tone: expect.stringContaining('green'),
    });
  });

  it('returns canonical trial activation notices', () => {
    expect(getTrialActivationNotice('activated')).toMatchObject({
      title: 'Trial activated',
      tone: expect.stringContaining('green'),
    });
    expect(getTrialActivationNotice('replayed')).toMatchObject({
      title: 'Trial already activated',
      tone: expect.stringContaining('sky'),
    });
    expect(getTrialActivationNotice('invalid')).toMatchObject({
      title: 'Activation link invalid',
      body: expect.stringContaining('fresh secure trial handoff'),
    });
    expect(getTrialActivationNotice('unavailable')).toMatchObject({
      title: 'Activation unavailable',
      body: expect.stringContaining('Refresh the billing state below'),
    });
    expect(getTrialActivationNotice('ineligible')).toMatchObject({
      title: 'Trial not available',
      tone: expect.stringContaining('red'),
    });
    expect(getTrialActivationNotice('')).toBeNull();
  });

  it('returns canonical purchase activation notices', () => {
    expect(getPurchaseActivationNotice('activated')).toMatchObject({
      title: 'Pulse Pro activated',
      tone: expect.stringContaining('green'),
    });
    expect(getPurchaseActivationNotice('cancelled')).toMatchObject({
      title: 'Checkout cancelled',
      tone: expect.stringContaining('amber'),
    });
    expect(getPurchaseActivationNotice('expired')).toMatchObject({
      title: 'Upgrade return expired',
      body: expect.stringContaining('Start the upgrade again'),
    });
    expect(getPurchaseActivationNotice('failed')).toMatchObject({
      title: 'Activation needs attention',
      tone: expect.stringContaining('red'),
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
