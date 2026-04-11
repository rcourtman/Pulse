import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import PricingHandoff from '@/pages/PricingHandoff';
import pricingHandoffSource from '@/pages/PricingHandoff.tsx?raw';
import { getSelfHostedPurchaseStartUrl } from '@/utils/pricingHandoff';

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

  it('hands self-hosted upgrade requests off to Pulse Account', async () => {
    window.history.replaceState({}, '', '/pricing?feature=relay');
    const expectedDestination = getSelfHostedPurchaseStartUrl(
      'relay',
      new URLSearchParams('feature=relay'),
    );

    render(() => (
      <Router>
        <Route path="/pricing" component={PricingHandoff} />
      </Router>
    ));

    await waitFor(() => {
      expect(handoffToExternalPricingMock).toHaveBeenCalledWith(
        expectedDestination,
      );
    });

    expect(screen.getByRole('heading', { name: 'Redirecting to Pulse Account' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /continue to Pulse Account/i })).toHaveAttribute(
      'href',
      expectedDestination,
    );
  });

  it('keeps the pricing handoff on the shared page-header shell', () => {
    expect(pricingHandoffSource).toContain("import { PageHeader } from '@/components/shared/PageHeader';");
    expect(pricingHandoffSource).toContain('<PageHeader');
    expect(pricingHandoffSource).not.toContain('<h1');
  });

  it('keeps monitored-system pricing handoffs inside the product', async () => {
    window.history.replaceState({}, '', '/pricing?feature=max_monitored_systems');

    render(() => (
      <Router>
        <Route path="/pricing" component={PricingHandoff} />
        <Route
          path="/settings/system/billing/plan"
          component={() => <div>Billing destination</div>}
        />
      </Router>
    ));

    expect(await screen.findByText('Billing destination')).toBeInTheDocument();
    expect(handoffToExternalPricingMock).not.toHaveBeenCalled();
  });
});
