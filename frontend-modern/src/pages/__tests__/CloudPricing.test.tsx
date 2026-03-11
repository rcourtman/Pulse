import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { Router, Route } from '@solidjs/router';

import CloudPricing from '@/pages/CloudPricing';

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: vi.fn(),
}));

describe('CloudPricing', () => {
  afterEach(() => {
    cleanup();
  });

  it('routes each cloud plan CTA to signup with the canonical tier key', async () => {
    render(() => (
      <Router>
        <Route path="/" component={CloudPricing} />
      </Router>
    ));

    expect(await screen.findByRole('link', { name: 'Claim Founding Rate' })).toHaveAttribute(
      'href',
      '/cloud/signup?tier=starter',
    );
    expect(screen.getByRole('link', { name: 'Get Power' })).toHaveAttribute(
      'href',
      '/cloud/signup?tier=power',
    );
    expect(screen.getByRole('link', { name: 'Get Max' })).toHaveAttribute(
      'href',
      '/cloud/signup?tier=max',
    );
  });
});
