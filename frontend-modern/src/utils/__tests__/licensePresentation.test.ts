import { describe, expect, it } from 'vitest';
import {
  BILLING_ADMIN_EMPTY_STATE,
  formatLicensePlanVersion,
  getBillingAdminOrganizationBadges,
  getBillingAdminStateUpdateSuccessMessage,
  getBillingAdminTrialStatus,
  getCommercialMigrationNotice,
  getFeatureMinTierLabel,
  getLicenseFeatureLabel,
  getLicenseStatusLoadingState,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
  getNoActiveProLicenseState,
  getOrganizationBillingLicenseStatusLabel,
  getTrialActivationNotice,
} from '@/utils/licensePresentation';

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
  });

  it('returns canonical feature labels', () => {
    expect(getLicenseFeatureLabel('ai_patrol')).toBe('Pulse Patrol');
    expect(getLicenseFeatureLabel('mobile_app')).toBe('Mobile App Access');
    expect(getLicenseFeatureLabel('custom_feature')).toBe('Custom Feature');
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

  it('returns canonical trial activation notices', () => {
    expect(getTrialActivationNotice('activated')).toMatchObject({
      title: 'Trial activated',
      tone: expect.stringContaining('green'),
    });
    expect(getTrialActivationNotice('ineligible')).toMatchObject({
      title: 'Trial not available',
      tone: expect.stringContaining('red'),
    });
    expect(getTrialActivationNotice('')).toBeNull();
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
    expect(getBillingAdminOrganizationBadges({ soft_deleted: true, suspended: true } as never)).toMatchObject([
      { label: 'soft-deleted' },
    ]);
    expect(getBillingAdminOrganizationBadges({ soft_deleted: false, suspended: true } as never)).toMatchObject([
      { label: 'suspended' },
    ]);
    expect(getBillingAdminStateUpdateSuccessMessage('active')).toBe(
      'Organization billing activated',
    );
    expect(BILLING_ADMIN_EMPTY_STATE).toBe('No organizations found.');
  });
});
