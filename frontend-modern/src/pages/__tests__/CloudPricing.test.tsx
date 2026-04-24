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

    expect(await screen.findByRole('link', { name: 'Start Starter Trial' })).toHaveAttribute(
      'href',
      '/cloud/signup?tier=starter',
    );
    expect(screen.getByText('Founding rate')).toBeInTheDocument();
    expect(screen.getByText('$19')).toBeInTheDocument();
    expect(screen.getByText('$29/month')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Start Power Trial' })).toHaveAttribute(
      'href',
      '/cloud/signup?tier=power',
    );
    expect(screen.getByRole('link', { name: 'Start Max Trial' })).toHaveAttribute(
      'href',
      '/cloud/signup?tier=max',
    );
    expect(screen.queryByText('Starter founding rate')).not.toBeInTheDocument();
    expect(screen.getAllByText('All Pro features')).toHaveLength(1);
    expect(screen.getByText('Managed hosting')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Choose a Cloud plan, add a payment method, and start the 14-day trial with no upfront charge in secure checkout.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('How it works')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Pulse Account' })).toHaveAttribute(
      'href',
      '/portal',
    );
    expect(screen.getByRole('link', { name: 'See self-hosted plans' })).toHaveAttribute(
      'href',
      'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade',
    );
    expect(
      screen.queryByText(/provisioned in under 60 seconds/i),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/no maintenance ever/i)).not.toBeInTheDocument();
  });
});
