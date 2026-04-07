import { describe, expect, it } from 'vitest';
import {
  getSelfHostedBillingHref,
  getSelfHostedBillingPlanIntent,
  getSelfHostedBillingUsageDetail,
  getPricingRouteDestination,
  getPublicPricingUrl,
  getUpgradeFallbackDestination,
  isExternalPricingDestination,
  resolveCanonicalSelfHostedBillingHref,
  resolveSelfHostedBillingSection,
  scopeSelfHostedBillingDestination,
  SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
  SELF_HOSTED_PRO_BILLING_PLAN_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF,
  SELF_HOSTED_PRO_BILLING_ROUTE,
  SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_HREF,
  SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
} from '@/utils/pricingHandoff';

describe('pricingHandoff', () => {
  it('routes product-owned monitored-system pricing links to billing', () => {
    expect(getUpgradeFallbackDestination('max_monitored_systems')).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF,
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
      SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF,
    );
  });

  it('resolves canonical billing hrefs for owned plan and usage states', () => {
    expect(
      getSelfHostedBillingHref('usage', {
        detail: SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
      }),
    ).toBe(SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF);
    expect(
      getSelfHostedBillingHref('plan', {
        intent: SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
      }),
    ).toBe(SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF);
  });

  it('canonicalizes legacy self-hosted billing aliases to route-owned states', () => {
    expect(
      resolveCanonicalSelfHostedBillingHref(
        SELF_HOSTED_PRO_BILLING_ROUTE,
        '',
        '#pulse-pro-usage',
      ),
    ).toBe(SELF_HOSTED_PRO_BILLING_USAGE_HREF);
    expect(resolveCanonicalSelfHostedBillingHref(SELF_HOSTED_PRO_BILLING_ROUTE)).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
  });

  it('derives billing focus and arrival intent from canonical routes', () => {
    expect(resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_USAGE_HREF)).toBe('usage');
    expect(resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_PLAN_HREF)).toBe('plan');
    expect(
      getSelfHostedBillingUsageDetail(
        '?details=' + SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
      ),
    ).toBe(SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL);
    expect(
      getSelfHostedBillingPlanIntent(
        '?intent=' + SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
      ),
    ).toBe(SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT);
  });

  it('scopes in-product billing destinations to canonical routes and leaves external ones alone', () => {
    expect(
      scopeSelfHostedBillingDestination(
        { href: SELF_HOSTED_PRO_BILLING_ROUTE, external: false },
        'usage',
        { detail: SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL },
      ),
    ).toEqual({
      href: SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF,
      external: false,
    });
    expect(
      scopeSelfHostedBillingDestination(
        { href: SELF_HOSTED_PRO_BILLING_PLAN_HREF, external: false },
        'plan',
        { intent: SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT },
      ),
    ).toEqual({
      href: SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF,
      external: false,
    });
    expect(
      scopeSelfHostedBillingDestination(
        { href: getPublicPricingUrl('relay'), external: true },
        'usage',
        { detail: SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL },
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
