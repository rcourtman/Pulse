import { describe, expect, it } from 'vitest';
import {
  anchorSelfHostedBillingDestination,
  getPricingRouteDestination,
  getPublicPricingUrl,
  getUpgradeFallbackDestination,
  isExternalPricingDestination,
  SELF_HOSTED_PRO_BILLING_PLAN_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_HREF,
} from '@/utils/pricingHandoff';

describe('pricingHandoff', () => {
  it('routes product-owned monitored-system pricing links to billing', () => {
    expect(getUpgradeFallbackDestination('max_monitored_systems')).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
  });

  it('routes product-owned cloud pricing links to the in-product cloud page', () => {
    expect(getUpgradeFallbackDestination('cloud')).toBe('/cloud');
  });

  it('routes self-hosted upgrades to the public pricing site', () => {
    expect(getUpgradeFallbackDestination('relay')).toBe(
      'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=relay',
    );
  });

  it('returns the canonical public pricing URL when no feature is provided', () => {
    expect(getPublicPricingUrl()).toBe(
      'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade',
    );
  });

  it('preserves extra query parameters when handing off the legacy pricing route', () => {
    expect(getPricingRouteDestination('?feature=relay&utm_content=legacy-bookmark')).toBe(
      'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=relay&utm_content=legacy-bookmark',
    );
  });

  it('keeps internal route exceptions when handing off the legacy pricing route', () => {
    expect(getPricingRouteDestination('?feature=max_monitored_systems')).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
  });

  it('anchors in-product billing destinations to the requested section when no hash exists', () => {
    expect(
      anchorSelfHostedBillingDestination(
        { href: '/settings/system/billing', external: false },
        'pulse-pro-usage',
      ),
    ).toEqual({
      href: SELF_HOSTED_PRO_BILLING_USAGE_HREF,
      external: false,
    });
  });

  it('keeps anchored or external destinations unchanged when scoping billing sections', () => {
    expect(
      anchorSelfHostedBillingDestination(
        { href: SELF_HOSTED_PRO_BILLING_PLAN_HREF, external: false },
        'pulse-pro-usage',
      ),
    ).toEqual({
      href: SELF_HOSTED_PRO_BILLING_PLAN_HREF,
      external: false,
    });
    expect(
      anchorSelfHostedBillingDestination(
        { href: getPublicPricingUrl('relay'), external: true },
        'pulse-pro-usage',
      ),
    ).toEqual({
      href: getPublicPricingUrl('relay'),
      external: true,
    });
  });

  it('detects external pricing destinations', () => {
    expect(
      isExternalPricingDestination(
        'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade',
      ),
    ).toBe(true);
    expect(isExternalPricingDestination('/cloud')).toBe(false);
  });
});
