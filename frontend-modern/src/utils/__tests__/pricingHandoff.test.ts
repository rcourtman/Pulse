import { describe, expect, it } from 'vitest';
import {
  getPricingRouteDestination,
  getPublicPricingUrl,
  getUpgradeFallbackDestination,
  isExternalPricingDestination,
} from '@/utils/pricingHandoff';

describe('pricingHandoff', () => {
  it('routes product-owned monitored-system pricing links to billing', () => {
    expect(getUpgradeFallbackDestination('max_monitored_systems')).toBe('/settings/system/billing');
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
      '/settings/system/billing',
    );
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
