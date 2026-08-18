import { cleanup, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { getHorizontalRailScrollLeft } from '@/components/shared/horizontalRailVisibilityModel';
import { PlatformSectionTabs } from '../sharedPlatformPage';

afterEach(() => {
  cleanup();
  window.history.replaceState({}, '', '/');
});

describe('PlatformSectionTabs', () => {
  it('does not recenter an active tab that is already visible', () => {
    expect(
      getHorizontalRailScrollLeft({
        scrollLeft: 0,
        scrollWidth: 406,
        clientWidth: 346,
        itemOffsetLeft: 135,
        itemOffsetWidth: 75,
      }),
    ).toBe(0);
  });

  it('moves only far enough to reveal a tab clipped on either edge', () => {
    expect(
      getHorizontalRailScrollLeft({
        scrollLeft: 180,
        scrollWidth: 500,
        clientWidth: 200,
        itemOffsetLeft: 150,
        itemOffsetWidth: 70,
      }),
    ).toBe(142);
    expect(
      getHorizontalRailScrollLeft({
        scrollLeft: 0,
        scrollWidth: 500,
        clientWidth: 200,
        itemOffsetLeft: 350,
        itemOffsetWidth: 100,
      }),
    ).toBe(258);
  });

  it('clamps active-tab visibility scrolling to the rail bounds', () => {
    expect(
      getHorizontalRailScrollLeft({
        scrollLeft: 40,
        scrollWidth: 500,
        clientWidth: 200,
        itemOffsetLeft: 0,
        itemOffsetWidth: 70,
      }),
    ).toBe(0);
    expect(
      getHorizontalRailScrollLeft({
        scrollLeft: 250,
        scrollWidth: 500,
        clientWidth: 200,
        itemOffsetLeft: 470,
        itemOffsetWidth: 70,
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
    expect(navigation).toHaveClass('pl-0');
    expect(navigation).toHaveClass('pr-0');
    expect(navigation).not.toHaveClass('px-10');
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

  it('exposes operable phone-width controls when more sections are clipped', async () => {
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
              active="overview"
              ariaLabel="Example sections"
            />
          )}
        />
      </Router>
    ));

    const navigation = screen.getByRole('navigation', { name: 'Example sections' });
    const scrollBy = vi.fn();
    Object.defineProperties(navigation, {
      clientWidth: { configurable: true, value: 220 },
      scrollWidth: { configurable: true, value: 520 },
      scrollLeft: { configurable: true, writable: true, value: 0 },
      scrollBy: { configurable: true, value: scrollBy },
    });

    window.dispatchEvent(new Event('resize'));
    const scrollRight = await screen.findByRole('button', {
      name: 'Example sections: scroll right',
    });
    expect(navigation).toHaveClass('pl-0');
    expect(navigation).toHaveClass('pr-10');
    expect(
      screen.queryByRole('button', { name: 'Example sections: scroll left' }),
    ).not.toBeInTheDocument();

    scrollRight.click();
    expect(scrollBy).toHaveBeenCalledWith({ left: 160, behavior: 'smooth' });

    navigation.scrollLeft = 120;
    navigation.dispatchEvent(new Event('scroll'));
    expect(
      await screen.findByRole('button', { name: 'Example sections: scroll left' }),
    ).toBeInTheDocument();
    expect(navigation).toHaveClass('pl-0');
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
