import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import PricingHandoff from '@/pages/PricingHandoff';
import pricingHandoffSource from '@/pages/PricingHandoff.tsx?raw';
import { getSelfHostedPurchaseStartUrl } from '@/utils/pricingHandoff';

const handoffToExternalPricingMock = vi.fn();

vi.mock('@/utils/pricingHandoff', async () => {
  const actual =
    await vi.importActual<typeof import('@/utils/pricingHandoff')>('@/utils/pricingHandoff');
  return {
    ...actual,
    handoffToExternalPricing: (...args: unknown[]) => handoffToExternalPricingMock(...args),
  };
});

describe('PricingHandoff', () => {
  beforeEach(() => {
    handoffToExternalPricingMock.mockReset();
    window.scrollTo = vi.fn();
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  it('hands unknown feature requests off to Pulse Account', async () => {
    window.history.replaceState({}, '', '/pricing?feature=unknown_pro_feature');
    const expectedDestination = getSelfHostedPurchaseStartUrl(
      'unknown_pro_feature',
      new URLSearchParams('feature=unknown_pro_feature'),
    );

    render(() => (
      <Router>
        <Route path="/pricing" component={PricingHandoff} />
      </Router>
    ));

    await waitFor(() => {
      expect(handoffToExternalPricingMock).toHaveBeenCalledWith(expectedDestination);
    });

    expect(
      screen.getByRole('heading', { name: 'Redirecting to Pulse Account' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /continue to Pulse Account/i })).toHaveAttribute(
      'href',
      expectedDestination,
    );
  });

  it('uses PageHeader without hiding the manual-redirect link on mobile', () => {
    expect(pricingHandoffSource).toContain(
      "import { PageHeader } from '@/components/shared/PageHeader';",
    );
    expect(pricingHandoffSource).toContain('<PageHeader');
    expect(pricingHandoffSource).toContain('descriptionVisibility="always"');
    expect(pricingHandoffSource).toContain('updateDocumentTitle={false}');
    expect(pricingHandoffSource).not.toContain('<h1');
  });

  // The earlier 'retired monitored-system pricing handoffs' case has been
  // removed: max_monitored_systems is no longer in
  // RETIRED_TRIAL_PRICING_FEATURES (the set was narrowed to trial features
  // only). It is now a live paid-feature key that routes to the
  // self-hosted purchase-start flow rather than the neutral Plans surface.
  // The trial_expired case below still exercises the retired-feature path.

  it('keeps retired trial pricing handoffs on the neutral Plans surface', async () => {
    window.history.replaceState({}, '', '/pricing?feature=trial_expired');

    render(() => (
      <Router>
        <Route path="/pricing" component={PricingHandoff} />
        <Route
          path="/settings/system/billing/plan"
          component={() => <div>Plans destination</div>}
        />
      </Router>
    ));

    expect(await screen.findByText('Plans destination')).toBeInTheDocument();
    expect(handoffToExternalPricingMock).not.toHaveBeenCalled();
  });
});
