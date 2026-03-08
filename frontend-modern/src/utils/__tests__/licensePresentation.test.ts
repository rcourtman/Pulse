import { describe, expect, it } from 'vitest';
import {
  getFeatureMinTierLabel,
  getLicenseFeatureLabel,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
} from '@/utils/licensePresentation';

describe('licensePresentation', () => {
  it('returns canonical tier labels', () => {
    expect(getLicenseTierLabel('free')).toBe('Community');
    expect(getLicenseTierLabel('pro_plus')).toBe('Pro+');
    expect(getLicenseTierLabel('enterprise')).toBe('Enterprise');
    expect(getLicenseTierLabel('custom_tier')).toBe('Custom Tier');
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
});
