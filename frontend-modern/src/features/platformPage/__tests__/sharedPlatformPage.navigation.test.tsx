import { cleanup, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it } from 'vitest';
import { PlatformSectionTabs, getPlatformSectionTabScrollLeft } from '../sharedPlatformPage';

afterEach(() => {
  cleanup();
  window.history.replaceState({}, '', '/');
});

describe('PlatformSectionTabs', () => {
  it('does not recenter an active tab that is already visible', () => {
    expect(
      getPlatformSectionTabScrollLeft({
        scrollLeft: 0,
        scrollWidth: 406,
        clientWidth: 346,
        tabOffsetLeft: 135,
        tabOffsetWidth: 75,
      }),
    ).toBe(0);
  });

  it('moves only far enough to reveal a tab clipped on either edge', () => {
    expect(
      getPlatformSectionTabScrollLeft({
        scrollLeft: 180,
        scrollWidth: 500,
        clientWidth: 200,
        tabOffsetLeft: 150,
        tabOffsetWidth: 70,
      }),
    ).toBe(142);
    expect(
      getPlatformSectionTabScrollLeft({
        scrollLeft: 0,
        scrollWidth: 500,
        clientWidth: 200,
        tabOffsetLeft: 350,
        tabOffsetWidth: 100,
      }),
    ).toBe(258);
  });

  it('clamps active-tab visibility scrolling to the rail bounds', () => {
    expect(
      getPlatformSectionTabScrollLeft({
        scrollLeft: 40,
        scrollWidth: 500,
        clientWidth: 200,
        tabOffsetLeft: 0,
        tabOffsetWidth: 70,
      }),
    ).toBe(0);
    expect(
      getPlatformSectionTabScrollLeft({
        scrollLeft: 250,
        scrollWidth: 500,
        clientWidth: 200,
        tabOffsetLeft: 470,
        tabOffsetWidth: 70,
      }),
    ).toBe(300);
  });

  it('hides overview-only section navigation', () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <PlatformSectionTabs
              tabs={[{ id: 'overview', label: 'Overview', path: '/example/overview' }] as const}
              active="overview"
              ariaLabel="Example sections"
            />
          )}
        />
      </Router>
    ));

    expect(screen.queryByRole('navigation', { name: 'Example sections' })).toBeNull();
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
    expect(navigation).toHaveClass('overflow-x-auto');
    expect(navigation).toHaveClass('scrollbar-hide');
    expect(navigation).not.toHaveClass('flex-wrap');
    expect(within(navigation).getByRole('link', { name: 'Overview' })).toHaveAttribute(
      'href',
      '/truenas/overview',
    );
    const activeLink = within(navigation).getByRole('link', { name: 'Storage' });
    expect(activeLink).toHaveAttribute('aria-current', 'page');
    expect(activeLink).toHaveClass('shrink-0');
    expect(activeLink).toHaveClass('whitespace-nowrap');
  });

  it('keeps the active destination visible when the tab rail narrows', async () => {
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <PlatformSectionTabs
              tabs={
                [
                  { id: 'overview', label: 'Overview', path: '/example/overview' },
                  { id: 'images', label: 'Images', path: '/example/images' },
                  { id: 'storage', label: 'Storage', path: '/example/storage' },
                  { id: 'networks', label: 'Networks', path: '/example/networks' },
                ] as const
              }
              active="networks"
              ariaLabel="Example sections"
            />
          )}
        />
      </Router>
    ));

    await new Promise((resolve) => window.setTimeout(resolve));

    const navigation = screen.getByRole('navigation', { name: 'Example sections' });
    const activeLink = within(navigation).getByRole('link', { name: 'Networks' });
    Object.defineProperties(navigation, {
      clientWidth: { configurable: true, value: 200 },
      scrollWidth: { configurable: true, value: 500 },
      scrollLeft: { configurable: true, writable: true, value: 0 },
    });
    Object.defineProperties(activeLink, {
      offsetLeft: { configurable: true, value: 350 },
      offsetWidth: { configurable: true, value: 100 },
    });

    window.dispatchEvent(new Event('resize'));

    await waitFor(() => expect(navigation.scrollLeft).toBe(258));
  });
});
