import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it } from 'vitest';
import { PlatformSectionTabs } from '../sharedPlatformPage';

afterEach(() => {
  cleanup();
  window.history.replaceState({}, '', '/');
});

describe('PlatformSectionTabs', () => {
  it('hides overview-only section navigation', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <PlatformSectionTabs
              tabs={
                [{ id: 'overview', label: 'Overview', path: '/standalone/overview' }] as const
              }
              active="overview"
              ariaLabel="Standalone sections"
            />
          )}
        />
      </Router>
    ));

    expect(screen.queryByRole('navigation', { name: 'Standalone sections' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Overview' })).toBeNull();
  });

  it('keeps section navigation visible when there are alternate destinations', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <PlatformSectionTabs
              tabs={
                [
                  { id: 'overview', label: 'Overview', path: '/truenas/overview' },
                  { id: 'storage', label: 'Storage', path: '/truenas/storage' },
                ] as const
              }
              active="storage"
              ariaLabel="TrueNAS sections"
            />
          )}
        />
      </Router>
    ));

    const navigation = screen.getByRole('navigation', { name: 'TrueNAS sections' });
    expect(within(navigation).getByRole('link', { name: 'Overview' })).toHaveAttribute(
      'href',
      '/truenas/overview',
    );
    expect(within(navigation).getByRole('link', { name: 'Storage' })).toHaveAttribute(
      'aria-current',
      'page',
    );
  });
});
