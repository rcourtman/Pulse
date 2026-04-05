const DEFAULT_PUBLIC_PRICING_URL =
  'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade';

const IN_PRODUCT_PRICING_DESTINATIONS: Record<string, string> = {
  max_monitored_systems: '/settings/system/billing',
  cloud: '/cloud',
};

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
  return /^https?:\/\//.test(destination);
}

export function handoffToExternalPricing(destination: string): void {
  window.location.replace(destination);
}
