import { describe, expect, it } from 'vitest';
import {
  getSelfHostedPurchaseStartUrl,
  getSelfHostedBillingHref,
  getSelfHostedBillingPlanDetail,
  getSelfHostedBillingPlanIntent,
  getSelfHostedBillingPurchaseArrival,
  getSelfHostedBillingUsageDetail,
  getPricingRouteDestination,
  getPublicPricingUrl,
  getUpgradeFallbackDestination,
  isExternalPricingDestination,
  isSelfHostedPurchaseStartDestination,
  resolveCanonicalSelfHostedBillingHref,
  resolveSelfHostedBillingSection,
  resolveSelfHostedPurchaseStartDestination,
  scopeSelfHostedBillingDestination,
  SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
  SELF_HOSTED_PRO_BILLING_PLAN_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF,
  SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
  SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
  SELF_HOSTED_PRO_BILLING_ROUTE,
  SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_HREF,
  SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
  SELF_HOSTED_PURCHASE_START_PATH,
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

  it('routes self-hosted upgrades to Pulse Account first', () => {
    expect(getUpgradeFallbackDestination('relay')).toBe(getSelfHostedPurchaseStartUrl('relay'));
  });

  it('returns the canonical public pricing URL when no feature is provided', () => {
    expect(getPublicPricingUrl()).toBe(
      'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade',
    );
  });

  it('preserves extra query parameters when handing off the legacy pricing route', () => {
    expect(getPricingRouteDestination('?feature=relay&utm_content=legacy-bookmark')).toBe(
      getSelfHostedPurchaseStartUrl(
        'relay',
        new URLSearchParams('feature=relay&utm_content=legacy-bookmark'),
      ),
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
        purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
      }),
    ).toBe(
      `${SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF}&purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}`,
    );
    expect(
      getSelfHostedBillingHref('plan', {
        intent: SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
        detail: SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
      }),
    ).toBe(`${SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF}&details=recovery`);
  });

  it('canonicalizes legacy self-hosted billing aliases to route-owned states', () => {
    expect(
      resolveCanonicalSelfHostedBillingHref(SELF_HOSTED_PRO_BILLING_ROUTE, '', '#pulse-pro-usage'),
    ).toBe(SELF_HOSTED_PRO_BILLING_USAGE_HREF);
    expect(
      resolveCanonicalSelfHostedBillingHref(SELF_HOSTED_PRO_BILLING_ROUTE, '', '#pulse-pro-recovery'),
    ).toBe(SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF);
    expect(resolveCanonicalSelfHostedBillingHref(SELF_HOSTED_PRO_BILLING_ROUTE)).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
  });

  it('derives billing focus and arrival intent from canonical routes', () => {
    expect(resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_USAGE_HREF)).toBe('usage');
    expect(resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_PLAN_HREF)).toBe('plan');
    expect(
      getSelfHostedBillingUsageDetail('?details=' + SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL),
    ).toBe(SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL);
    expect(
      getSelfHostedBillingPlanIntent('?intent=' + SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT),
    ).toBe(SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT);
    expect(getSelfHostedBillingPlanDetail('?details=' + SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL)).toBe(
      SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
    );
    expect(
      getSelfHostedBillingPurchaseArrival(
        '?purchase=' + SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
      ),
    ).toBe(SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED);
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
        resolveSelfHostedPurchaseStartDestination('relay'),
        'usage',
        { detail: SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL },
      ),
    ).toEqual({
      href: getSelfHostedPurchaseStartUrl('relay'),
      external: false,
      hardNavigation: true,
      newTab: true,
      preserveOpener: true,
    });
  });

  it('detects external pricing destinations', () => {
    expect(isExternalPricingDestination(getPublicPricingUrl('relay'))).toBe(true);
    expect(isExternalPricingDestination(getSelfHostedPurchaseStartUrl('relay'))).toBe(false);
    expect(isExternalPricingDestination('/cloud')).toBe(false);
  });

  it('detects the internal self-hosted purchase start destination', () => {
    expect(isSelfHostedPurchaseStartDestination(getSelfHostedPurchaseStartUrl('relay'))).toBe(true);
    expect(isSelfHostedPurchaseStartDestination(SELF_HOSTED_PURCHASE_START_PATH)).toBe(true);
    expect(isSelfHostedPurchaseStartDestination(getPublicPricingUrl('relay'))).toBe(false);
  });
});
