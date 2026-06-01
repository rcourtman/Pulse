import { describe, expect, it } from 'vitest';
import { SELF_HOSTED_FEATURE_CATALOG } from '@/utils/selfHostedFeatureCatalog.generated';
import {
  getManagedHostingRequestUrl,
  getSelfHostedPurchaseStartUrl,
  getSelfHostedBillingHref,
  getSelfHostedBillingPlanDetail,
  getSelfHostedBillingPlanIntent,
  getSelfHostedBillingPurchaseArrival,
  getSelfHostedBillingUsageDetail,
  getPricingRouteDestination,
  getPublicPricingUrl,
  getUpgradeFallbackDestination,
  getInProductPricingDestination,
  isExternalPricingDestination,
  isRetiredPricingFeature,
  isSelfHostedPurchaseStartDestination,
  resolveCanonicalSelfHostedBillingHref,
  resolveSelfHostedBillingSection,
  resolveSelfHostedPurchaseStartDestination,
  scopeSelfHostedBillingDestination,
  SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
  SELF_HOSTED_PRO_BILLING_PLAN_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
  SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_UNAVAILABLE,
  SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
  SELF_HOSTED_PRO_BILLING_ROUTE,
  SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_HREF,
  SELF_HOSTED_PURCHASE_START_PATH,
} from '@/utils/pricingHandoff';

describe('pricingHandoff', () => {
  it('does not keep a special in-product monitored-system pricing route', () => {
    expect(getUpgradeFallbackDestination('max_monitored_systems')).toBe(
      getSelfHostedPurchaseStartUrl('max_monitored_systems'),
    );
  });

  it('routes cloud pricing links to request-assisted hosting instead of an in-product trial page', () => {
    expect(getUpgradeFallbackDestination('cloud')).toBe(getManagedHostingRequestUrl());
    expect(getInProductPricingDestination('cloud')).toBeUndefined();
  });

  it('routes paid self-hosted feature upgrades to the in-product billing plan page', () => {
    expect(getUpgradeFallbackDestination('relay')).toBe(SELF_HOSTED_PRO_BILLING_PLAN_HREF);
    expect(getUpgradeFallbackDestination('mobile_app')).toBe(SELF_HOSTED_PRO_BILLING_PLAN_HREF);
    expect(getUpgradeFallbackDestination('push_notifications')).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
    expect(getUpgradeFallbackDestination('ai_alerts')).toBe(SELF_HOSTED_PRO_BILLING_PLAN_HREF);
    expect(getUpgradeFallbackDestination('rbac')).toBe(SELF_HOSTED_PRO_BILLING_PLAN_HREF);
    expect(getUpgradeFallbackDestination('advanced_reporting')).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
  });

  it('keeps all displayable paid self-hosted catalog features on the owned Plans surface', () => {
    const paidCatalogFeatureKeys = SELF_HOSTED_FEATURE_CATALOG.filter(
      (entry) =>
        entry.displayableInSelfHostedPlan &&
        !entry.includedIn.community &&
        (entry.includedIn.relay || entry.includedIn.pro),
    ).map((entry) => entry.key);

    expect(paidCatalogFeatureKeys).toEqual([
      'relay',
      'mobile_app',
      'push_notifications',
      'ai_alerts',
      'ai_autofix',
      'long_term_metrics',
      'rbac',
      'audit_logging',
      'advanced_reporting',
      'agent_profiles',
    ]);
    for (const key of paidCatalogFeatureKeys) {
      expect(getUpgradeFallbackDestination(key)).toBe(SELF_HOSTED_PRO_BILLING_PLAN_HREF);
    }
  });

  it('keeps retired trial-expired out of upgrade fallbacks while preserving neutral legacy arrival', () => {
    expect(getPricingRouteDestination('?feature=trial_expired')).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
    expect(getInProductPricingDestination('trial_expired')).toBeUndefined();
    expect(getUpgradeFallbackDestination('trial_expired')).toBeUndefined();
    expect(isRetiredPricingFeature('trial_expired')).toBe(true);
    expect(
      isSelfHostedPurchaseStartDestination(getPricingRouteDestination('?feature=trial_expired')),
    ).toBe(false);
  });

  it('routes unknown feature upgrades to Pulse Account purchase start', () => {
    expect(getUpgradeFallbackDestination('unknown_pro_feature')).toBe(
      getSelfHostedPurchaseStartUrl('unknown_pro_feature'),
    );
  });

  it('returns the canonical public pricing URL when no feature is provided', () => {
    expect(getPublicPricingUrl()).toBe(
      'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade',
    );
  });

  it('preserves extra query parameters when handing off the legacy pricing route', () => {
    expect(
      getPricingRouteDestination('?feature=unknown_pro_feature&utm_content=legacy-bookmark'),
    ).toBe(
      getSelfHostedPurchaseStartUrl(
        'unknown_pro_feature',
        new URLSearchParams('feature=unknown_pro_feature&utm_content=legacy-bookmark'),
      ),
    );
  });

  it('treats legacy monitored-system pricing links as external purchase-start compatibility', () => {
    expect(getPricingRouteDestination('?feature=max_monitored_systems')).toBe(
      getSelfHostedPurchaseStartUrl('max_monitored_systems'),
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
        intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
        purchase: SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
      }),
    ).toBe(
      `${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF}&purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}`,
    );
    expect(
      getSelfHostedBillingHref('plan', {
        intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
        detail: SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
      }),
    ).toBe(`${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF}&details=recovery`);
  });

  it('canonicalizes legacy self-hosted billing aliases to route-owned states', () => {
    expect(
      resolveCanonicalSelfHostedBillingHref(SELF_HOSTED_PRO_BILLING_ROUTE, '', '#pulse-pro-usage'),
    ).toBe(SELF_HOSTED_PRO_BILLING_USAGE_HREF);
    expect(
      resolveCanonicalSelfHostedBillingHref(
        SELF_HOSTED_PRO_BILLING_ROUTE,
        '',
        '#pulse-pro-recovery',
      ),
    ).toBe(SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF);
    expect(resolveCanonicalSelfHostedBillingHref(SELF_HOSTED_PRO_BILLING_ROUTE)).toBe(
      SELF_HOSTED_PRO_BILLING_PLAN_HREF,
    );
    expect(
      resolveCanonicalSelfHostedBillingHref(
        SELF_HOSTED_PRO_BILLING_PLAN_HREF,
        '?intent=max_monitored_systems',
      ),
    ).toBe(SELF_HOSTED_PRO_BILLING_PLAN_HREF);
    expect(resolveCanonicalSelfHostedBillingHref(`${SELF_HOSTED_PRO_BILLING_ROUTE}/history`)).toBe(
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
      getSelfHostedBillingPlanIntent('?intent=' + SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT),
    ).toBe(SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT);
    expect(
      resolveSelfHostedBillingSection(
        SELF_HOSTED_PRO_BILLING_PLAN_HREF,
        '?intent=max_monitored_systems',
      ),
    ).toBe('plan');
    expect(getSelfHostedBillingPlanIntent('?intent=max_monitored_systems')).toBeNull();
    expect(
      getSelfHostedBillingPlanDetail('?details=' + SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL),
    ).toBe(SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL);
    expect(
      getSelfHostedBillingPurchaseArrival(
        '?purchase=' + SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
      ),
    ).toBe(SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED);
    expect(
      getSelfHostedBillingPurchaseArrival(
        '?purchase=' + SELF_HOSTED_PRO_BILLING_PURCHASE_UNAVAILABLE,
      ),
    ).toBe(SELF_HOSTED_PRO_BILLING_PURCHASE_UNAVAILABLE);
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
        { intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT },
      ),
    ).toEqual({
      href: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
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
    expect(isExternalPricingDestination(getManagedHostingRequestUrl())).toBe(true);
  });

  it('detects the internal self-hosted purchase start destination', () => {
    expect(isSelfHostedPurchaseStartDestination(getSelfHostedPurchaseStartUrl('relay'))).toBe(true);
    expect(isSelfHostedPurchaseStartDestination(SELF_HOSTED_PURCHASE_START_PATH)).toBe(true);
    expect(isSelfHostedPurchaseStartDestination(getPublicPricingUrl('relay'))).toBe(false);
  });
});
