import {
  isExternalUpgradeHref,
  type UpgradeDestination,
} from '@/utils/upgradeNavigation';

const DEFAULT_PUBLIC_PRICING_URL =
  'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade';

export const SELF_HOSTED_PRO_BILLING_ROUTE = '/settings/system/billing';
export const SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID = 'pulse-pro-plan';
export const SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID = 'pulse-pro-usage';
export const SELF_HOSTED_PRO_BILLING_PLAN_HREF = `${SELF_HOSTED_PRO_BILLING_ROUTE}#${SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID}`;
export const SELF_HOSTED_PRO_BILLING_USAGE_HREF = `${SELF_HOSTED_PRO_BILLING_ROUTE}#${SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID}`;

const IN_PRODUCT_PRICING_DESTINATIONS: Record<string, string> = {
  max_monitored_systems: SELF_HOSTED_PRO_BILLING_PLAN_HREF,
  cloud: '/cloud',
};

const INTERNAL_HREF_BASE = 'https://pulse.invalid';

function normalizeFeatureKey(feature: string | null | undefined): string | undefined {
  const normalized = feature?.trim();
  return normalized ? normalized : undefined;
}

export function getInProductPricingDestination(
  feature: string | null | undefined,
): string | undefined {
  const normalizedFeature = normalizeFeatureKey(feature);
  if (!normalizedFeature) return undefined;
  return IN_PRODUCT_PRICING_DESTINATIONS[normalizedFeature];
}

export function getPublicPricingUrl(feature?: string | null): string {
  const url = new URL(DEFAULT_PUBLIC_PRICING_URL);
  const normalizedFeature = normalizeFeatureKey(feature);
  if (normalizedFeature) {
    url.searchParams.set('feature', normalizedFeature);
  }
  return url.toString();
}

export function getUpgradeFallbackDestination(feature?: string | null): string {
  return getInProductPricingDestination(feature) || getPublicPricingUrl(feature);
}

export function anchorSelfHostedBillingDestination(
  destination: UpgradeDestination,
  sectionId: string,
): UpgradeDestination {
  if (destination.external) {
    return destination;
  }

  const normalizedSectionId = sectionId.trim();
  if (!normalizedSectionId) {
    return destination;
  }

  const url = new URL(destination.href, INTERNAL_HREF_BASE);
  if (url.pathname !== SELF_HOSTED_PRO_BILLING_ROUTE || url.hash) {
    return destination;
  }

  url.hash = normalizedSectionId;
  return {
    ...destination,
    href: `${url.pathname}${url.search}${url.hash}`,
  };
}

export function getPricingRouteDestination(search: string): string {
  const params = new URLSearchParams(search.startsWith('?') ? search.slice(1) : search);
  const feature = params.get('feature');
  const inProductDestination = getInProductPricingDestination(feature);
  if (inProductDestination) {
    return inProductDestination;
  }

  const url = new URL(DEFAULT_PUBLIC_PRICING_URL);
  for (const [key, value] of params.entries()) {
    const normalizedValue = value.trim();
    if (normalizedValue) {
      url.searchParams.set(key, normalizedValue);
    }
  }
  return url.toString();
}

export function isExternalPricingDestination(destination: string): boolean {
  return isExternalUpgradeHref(destination);
}

export function handoffToExternalPricing(destination: string): void {
  window.location.replace(destination);
}
