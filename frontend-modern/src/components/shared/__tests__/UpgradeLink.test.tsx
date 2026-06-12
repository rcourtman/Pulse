import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it } from 'vitest';
import { UpgradeButtonLink, UpgradeLink } from '@/components/shared/UpgradeLink';

describe('UpgradeLink', () => {
  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  it('renders internal upgrade destinations as in-app links', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <UpgradeLink destination={{ href: '/settings/system/billing', external: false }}>
              Billing
            </UpgradeLink>
          )}
        />
      </Router>
    ));

    const link = screen.getByRole('link', { name: 'Billing' });
    expect(link).toHaveAttribute('href', '/settings/system/billing');
    expect(link).not.toHaveAttribute('target');
  });

  it('renders external upgrade destinations as safe new-tab links', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <UpgradeLink
              destination={{
                href: 'https://pulserelay.pro/pricing?feature=relay',
                external: true,
              }}
            >
              Upgrade
            </UpgradeLink>
          )}
        />
      </Router>
    ));

    const link = screen.getByRole('link', { name: 'Upgrade' });
    expect(link).toHaveAttribute('href', 'https://pulserelay.pro/pricing?feature=relay');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('preserves opener access for self-hosted purchase-start links', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <UpgradeLink
              destination={{
                href: '/auth/license-purchase-start?feature=relay',
                external: false,
                hardNavigation: true,
                newTab: true,
                preserveOpener: true,
              }}
            >
              Compare plans
            </UpgradeLink>
          )}
        />
      </Router>
    ));

    const link = screen.getByRole('link', { name: 'Compare plans' });
    expect(link).toHaveAttribute('href', '/auth/license-purchase-start?feature=relay');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).not.toHaveAttribute('rel');
  });

  it('renders upgrade actions through the shared button link shell', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <UpgradeButtonLink
              destination={{ href: '/settings/system/billing', external: false }}
              tone="warning"
              mobileFullWidth={false}
            >
              View plans
            </UpgradeButtonLink>
          )}
        />
      </Router>
    ));

    const link = screen.getByRole('link', { name: 'View plans' });
    expect(link).toHaveAttribute('href', '/settings/system/billing');
    expect(link).not.toHaveAttribute('target');
    expect(link).toHaveClass('inline-flex');
    expect(link).toHaveClass('bg-amber-100');
    expect(link).toHaveClass('w-auto');
  });

  it('keeps opener access available for upgrade button purchase-start links', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <UpgradeButtonLink
              destination={{
                href: '/auth/license-purchase-start?feature=relay',
                external: false,
                hardNavigation: true,
                newTab: true,
                preserveOpener: true,
              }}
            >
              Compare plans
            </UpgradeButtonLink>
          )}
        />
      </Router>
    ));

    const link = screen.getByRole('link', { name: 'Compare plans' });
    expect(link).toHaveAttribute('href', '/auth/license-purchase-start?feature=relay');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).not.toHaveAttribute('rel');
  });
});
