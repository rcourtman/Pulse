import { describe, expect, it, vi } from 'vitest';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';
import {
  getInProductPricingDestination,
  getPricingRouteDestination,
  handoffToExternalPricing,
  isRetiredPricingFeature,
  isSelfHostedPurchaseStartDestination,
  LEGACY_SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
  LEGACY_SELF_HOSTED_PRO_BILLING_ROUTE,
  LEGACY_SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
  resolveCanonicalSelfHostedBillingHref,
  resolveSelfHostedBillingSection,
  scopeSelfHostedBillingDestination,
  SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
  SELF_HOSTED_PRO_BILLING_PLAN_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
  SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
  SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
  SELF_HOSTED_PRO_BILLING_ROUTE,
  SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
  SELF_HOSTED_PURCHASE_START_PATH,
} from '@/utils/pricingHandoff';

const PUBLIC_PRICING_URL =
  'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade';

describe('pricingHandoff (branch coverage)', () => {
  describe('normalizeSettingsLikePath (via resolveSelfHostedBillingSection)', () => {
    it('returns the original input and falls through when the path trims to empty', () => {
      // Empty/whitespace inputs hit the `if (!normalized) return pathname` early return.
      // The un-trimmed value is carried forward and, being a non-billing path, resolves to 'plan'.
      expect(resolveSelfHostedBillingSection('')).toBe('plan');
      expect(resolveSelfHostedBillingSection('   ')).toBe('plan');
    });

    it('strips a single trailing slash before matching routes', () => {
      expect(resolveSelfHostedBillingSection(`${SELF_HOSTED_PRO_BILLING_USAGE_ROUTE}/`)).toBe(
        'usage',
      );
      expect(resolveSelfHostedBillingSection(`${SELF_HOSTED_PRO_BILLING_PLAN_ROUTE}/`)).toBe(
        'plan',
      );
    });

    it('strips multiple trailing slashes via the /\\+$/ replace', () => {
      expect(resolveSelfHostedBillingSection(`${SELF_HOSTED_PRO_BILLING_USAGE_ROUTE}//`)).toBe(
        'usage',
      );
    });

    it('leaves a single-character root path unchanged (length > 1 guard is false)', () => {
      expect(resolveSelfHostedBillingSection('/')).toBe('plan');
    });

    it('passes a normal path straight through to the final return', () => {
      expect(resolveSelfHostedBillingSection('/totally/elsewhere')).toBe('plan');
    });
  });

  describe('resolveSelfHostedBillingSection', () => {
    it('resolves the legacy usage and plan sub-routes directly', () => {
      expect(resolveSelfHostedBillingSection(LEGACY_SELF_HOSTED_PRO_BILLING_USAGE_ROUTE)).toBe(
        'usage',
      );
      expect(resolveSelfHostedBillingSection(LEGACY_SELF_HOSTED_PRO_BILLING_PLAN_ROUTE)).toBe(
        'plan',
      );
    });

    it('treats any unrecognized non-billing path as the plan section', () => {
      expect(resolveSelfHostedBillingSection('/dashboard')).toBe('plan');
    });

    it('derives usage/plan from the section id hash at the canonical billing root', () => {
      expect(
        resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_ROUTE, '', '#pulse-pro-usage'),
      ).toBe('usage');
      expect(
        resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_ROUTE, '', '#pulse-pro-plan'),
      ).toBe('plan');
      expect(
        resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_ROUTE, '', '#pulse-pro-recovery'),
      ).toBe('plan');
    });

    it('accepts a hash without the leading "#" via normalizeHash', () => {
      expect(
        resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_ROUTE, '', 'pulse-pro-usage'),
      ).toBe('usage');
    });

    it('falls back to plan when the hash is blank/whitespace at the billing root', () => {
      expect(resolveSelfHostedBillingSection(SELF_HOSTED_PRO_BILLING_ROUTE, '', '   ')).toBe(
        'plan',
      );
    });

    it('derives usage/plan from query-string details at the billing root', () => {
      expect(
        resolveSelfHostedBillingSection(
          SELF_HOSTED_PRO_BILLING_ROUTE,
          `?details=${SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL}`,
        ),
      ).toBe('usage');
      expect(
        resolveSelfHostedBillingSection(
          SELF_HOSTED_PRO_BILLING_ROUTE,
          `?details=${SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL}`,
        ),
      ).toBe('plan');
    });

    it('defaults to plan at the legacy billing root with no disambiguating signal', () => {
      expect(resolveSelfHostedBillingSection(LEGACY_SELF_HOSTED_PRO_BILLING_ROUTE)).toBe('plan');
    });
  });

  describe('getInProductPricingDestination', () => {
    it('maps known in-product feature keys to their plan/selection hrefs', () => {
      expect(getInProductPricingDestination('relay')).toBe(SELF_HOSTED_PRO_BILLING_PLAN_HREF);
      expect(getInProductPricingDestination('mobile_app')).toBe(SELF_HOSTED_PRO_BILLING_PLAN_HREF);
      expect(getInProductPricingDestination('long_term_metrics')).toBe(
        SELF_HOSTED_PRO_BILLING_PLAN_HREF,
      );
      expect(getInProductPricingDestination('agent_profiles')).toBe(
        SELF_HOSTED_PRO_BILLING_PLAN_HREF,
      );
      expect(getInProductPricingDestination('self_hosted_plan')).toBe(
        SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
      );
      expect(getInProductPricingDestination('ai_autofix')).toBe(
        SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
      );
    });

    it('returns undefined when the feature is absent/blank (normalizeFeatureKey falsy branch)', () => {
      expect(getInProductPricingDestination(null)).toBeUndefined();
      expect(getInProductPricingDestination(undefined)).toBeUndefined();
      expect(getInProductPricingDestination('')).toBeUndefined();
      expect(getInProductPricingDestination('   ')).toBeUndefined();
    });

    it('returns undefined for a feature key that is not in the catalog map', () => {
      expect(getInProductPricingDestination('definitely_not_a_feature')).toBeUndefined();
    });
  });

  describe('isRetiredPricingFeature', () => {
    it('returns false for non-retired feature keys', () => {
      expect(isRetiredPricingFeature('relay')).toBe(false);
      expect(isRetiredPricingFeature('cloud')).toBe(false);
      expect(isRetiredPricingFeature('trial_expired_x')).toBe(false);
    });

    it('returns false (not the thrown/has branch) when the feature is absent/blank', () => {
      expect(isRetiredPricingFeature(null)).toBe(false);
      expect(isRetiredPricingFeature(undefined)).toBe(false);
      expect(isRetiredPricingFeature('')).toBe(false);
      expect(isRetiredPricingFeature('   ')).toBe(false);
    });

    it('returns true only for the retired trial_expired key', () => {
      expect(isRetiredPricingFeature('trial_expired')).toBe(true);
    });
  });

  describe('getPricingRouteDestination', () => {
    it('parses a search string without a leading "?" and still resolves in-product features', () => {
      expect(getPricingRouteDestination('feature=ai_autofix')).toBe(
        SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
      );
    });

    it('returns the canonical public pricing URL when there is no feature and no search', () => {
      expect(getPricingRouteDestination('')).toBe(PUBLIC_PRICING_URL);
    });

    it('merges non-feature query params onto the public pricing URL', () => {
      expect(getPricingRouteDestination('?utm_content=join')).toBe(
        `${PUBLIC_PRICING_URL}&utm_content=join`,
      );
    });

    it('drops a blank feature value and keeps non-empty params on the public URL', () => {
      // feature=&utm_content=keep -> feature is empty so it falls through to the public
      // URL branch; the empty value is skipped while utm_content is preserved.
      expect(getPricingRouteDestination('?feature=&utm_content=keep')).toBe(
        `${PUBLIC_PRICING_URL}&utm_content=keep`,
      );
    });
  });

  describe('resolveCanonicalSelfHostedBillingHref', () => {
    it('returns null for non-billing paths', () => {
      expect(resolveCanonicalSelfHostedBillingHref('/dashboard')).toBeNull();
      expect(resolveCanonicalSelfHostedBillingHref('')).toBeNull();
    });

    it('canonicalizes a counting-rules usage detail to the usage counting-rules href', () => {
      expect(
        resolveCanonicalSelfHostedBillingHref(
          SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
          `?details=${SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL}`,
        ),
      ).toBe(SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF);
    });

    it('canonicalizes a valid plan-selection intent to the plan selection href', () => {
      expect(
        resolveCanonicalSelfHostedBillingHref(
          SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
          `?intent=${SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT}`,
        ),
      ).toBe(SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF);
    });

    it('canonicalizes a recovery detail (from search) to the plan recovery href', () => {
      expect(
        resolveCanonicalSelfHostedBillingHref(
          SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
          `?details=${SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL}`,
        ),
      ).toBe(SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF);
    });

    it('carries a purchase arrival onto the canonical plan href', () => {
      expect(
        resolveCanonicalSelfHostedBillingHref(
          SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
          `?purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}`,
        ),
      ).toBe(
        `${SELF_HOSTED_PRO_BILLING_PLAN_HREF}?purchase=${SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED}`,
      );
    });

    it('defaults the bare canonical billing root to the plan href', () => {
      expect(resolveCanonicalSelfHostedBillingHref(SELF_HOSTED_PRO_BILLING_ROUTE)).toBe(
        SELF_HOSTED_PRO_BILLING_PLAN_HREF,
      );
    });

    it('ignores plan-only details on the usage section (usageDetail stays null)', () => {
      // recovery is a plan detail; on the usage section it must not be applied.
      expect(
        resolveCanonicalSelfHostedBillingHref(
          SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
          `?details=${SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL}`,
        ),
      ).toBe(SELF_HOSTED_PRO_BILLING_USAGE_HREF);
    });
  });

  describe('scopeSelfHostedBillingDestination', () => {
    it('returns an external destination unchanged (external early return)', () => {
      const destination: UpgradeDestination = {
        href: 'https://example.com/pricing',
        external: true,
      };
      expect(scopeSelfHostedBillingDestination(destination, 'plan')).toBe(destination);
      expect(scopeSelfHostedBillingDestination(destination, 'plan')).toStrictEqual(destination);
    });

    it('returns an internal non-billing destination unchanged (not-a-billing-path return)', () => {
      const destination: UpgradeDestination = { href: '/dashboard', external: false };
      expect(scopeSelfHostedBillingDestination(destination, 'plan')).toBe(destination);
      expect(scopeSelfHostedBillingDestination(destination, 'plan')).toStrictEqual({
        href: '/dashboard',
        external: false,
      });
    });

    it('rewrites a billing-root destination to the plain usage href', () => {
      const destination: UpgradeDestination = {
        href: SELF_HOSTED_PRO_BILLING_ROUTE,
        external: false,
      };
      expect(scopeSelfHostedBillingDestination(destination, 'usage')).toStrictEqual({
        href: SELF_HOSTED_PRO_BILLING_USAGE_HREF,
        external: false,
      });
    });

    it('preserves auxiliary destination fields when rewriting the href', () => {
      const destination: UpgradeDestination = {
        href: SELF_HOSTED_PRO_BILLING_ROUTE,
        external: false,
        hardNavigation: true,
        newTab: false,
        preserveOpener: true,
      };
      expect(
        scopeSelfHostedBillingDestination(destination, 'plan', {
          intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
        }),
      ).toStrictEqual({
        href: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
        external: false,
        hardNavigation: true,
        newTab: false,
        preserveOpener: true,
      });
    });
  });

  describe('isSelfHostedPurchaseStartDestination', () => {
    it('returns false and swallows the error for a malformed non-external URL (catch branch)', () => {
      // 'ftp://[' is not http(s)/mailto so it bypasses the external guard, but it is
      // an invalid special-scheme URL and throws inside new URL(...), hitting catch.
      expect(isSelfHostedPurchaseStartDestination('ftp://[')).toBe(false);
    });

    it('returns false for an internal path that is not the purchase-start path', () => {
      expect(isSelfHostedPurchaseStartDestination('/settings/elsewhere')).toBe(false);
    });

    it('returns true and ignores query string and hash on the purchase-start path', () => {
      expect(
        isSelfHostedPurchaseStartDestination(`${SELF_HOSTED_PURCHASE_START_PATH}?feature=relay`),
      ).toBe(true);
      expect(isSelfHostedPurchaseStartDestination(`${SELF_HOSTED_PURCHASE_START_PATH}#top`)).toBe(
        true,
      );
    });

    it('returns false for an external upgrade href (external guard)', () => {
      expect(isSelfHostedPurchaseStartDestination('https://pulserelay.pro/pricing')).toBe(false);
      expect(isSelfHostedPurchaseStartDestination('mailto:support@pulserelay.pro')).toBe(false);
    });
  });

  describe('handoffToExternalPricing', () => {
    it('replaces the window location with the destination href', () => {
      // jsdom defines `window.location.replace` as a non-configurable own
      // property, so vi.spyOn cannot intercept it. `window.location` itself is
      // a configurable accessor, so swap in a stub carrying a `replace` spy and
      // restore the original descriptor afterwards.
      const originalDescriptor = Object.getOwnPropertyDescriptor(window, 'location');
      const replaceSpy = vi.fn<(href: string) => void>();
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: { replace: replaceSpy },
      });
      try {
        handoffToExternalPricing('https://example.com/pricing');
        expect(replaceSpy).toHaveBeenCalledWith('https://example.com/pricing');
        expect(replaceSpy).toHaveBeenCalledTimes(1);
      } finally {
        if (originalDescriptor) {
          Object.defineProperty(window, 'location', originalDescriptor);
        }
      }
    });
  });
});
