import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import PricingHandoff from '@/pages/PricingHandoff';

const trackPaywallViewedMock = vi.fn();
const handoffToExternalPricingMock = vi.fn();

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: (...args: unknown[]) => trackPaywallViewedMock(...args),
}));

vi.mock('@/utils/pricingHandoff', async () => {
  const actual = await vi.importActual<typeof import('@/utils/pricingHandoff')>(
    '@/utils/pricingHandoff',
  );
  return {
    ...actual,
    handoffToExternalPricing: (...args: unknown[]) => handoffToExternalPricingMock(...args),
  };
});

describe('PricingHandoff', () => {
  beforeEach(() => {
    trackPaywallViewedMock.mockReset();
    handoffToExternalPricingMock.mockReset();
    window.scrollTo = vi.fn();
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  it('hands public pricing requests off to the website', async () => {
    window.history.replaceState({}, '', '/pricing?feature=relay');

    render(() => (
      <Router>
        <Route path="/pricing" component={PricingHandoff} />
      </Router>
    ));

    await waitFor(() => {
      expect(handoffToExternalPricingMock).toHaveBeenCalledWith(
        'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=relay',
      );
    });

    expect(screen.getByRole('heading', { name: 'Redirecting to pricing' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /continue to the public pricing site/i })).toHaveAttribute(
      'href',
      'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade&feature=relay',
    );
  });

  it('keeps monitored-system pricing handoffs inside the product', async () => {
    window.history.replaceState({}, '', '/pricing?feature=max_monitored_systems');

    render(() => (
      <Router>
        <Route path="/pricing" component={PricingHandoff} />
        <Route path="/settings/system/billing" component={() => <div>Billing destination</div>} />
      </Router>
    ));

    expect(await screen.findByText('Billing destination')).toBeInTheDocument();
    expect(handoffToExternalPricingMock).not.toHaveBeenCalled();
  });
});
