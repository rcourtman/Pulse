import { fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { createSignal, type Component } from 'solid-js';
import { createStableTabList, utilityNavTabEquals } from '@/components/shared/stableNavTabs';
import mobileNavBarSource from '@/components/shared/MobileNavBar.tsx?raw';
import mobileNavBarModelSource from '@/components/shared/mobileNavBarModel.ts?raw';
import mobileNavBarStateSource from '@/components/shared/useMobileNavBarState.ts?raw';
import {
  MobileNavBar,
  type MobileNavBarPrimaryTab,
  type MobileNavBarUtilityTab,
} from '@/components/shared/MobileNavBar';

const TextIcon: Component<{ class?: string }> = (props) => <span class={props.class}>Icon</span>;
const PatrolIcon: Component<{ class?: string }> = (props) => (
  <svg aria-label="Pulse Patrol" class={props.class} viewBox="0 0 24 24">
    <title>Pulse Patrol</title>
    <circle cx="12" cy="12" r="8" />
  </svg>
);

function makePrimary(
  id: string,
  label: string,
  overrides: Partial<MobileNavBarPrimaryTab> = {},
): MobileNavBarPrimaryTab {
  return {
    id,
    label,
    route: `/${id}/overview`,
    settingsRoute: '/settings/infrastructure',
    tooltip: label,
    enabled: true,
    live: true,
    icon: TextIcon,
    alwaysShow: true,
    ...overrides,
  };
}

function makeUtility(
  id: MobileNavBarUtilityTab['id'],
  label: string,
  overrides: Partial<MobileNavBarUtilityTab> = {},
): MobileNavBarUtilityTab {
  return {
    id,
    label,
    route: id === 'ai' ? '/patrol' : `/${id}`,
    tooltip: label,
    badge: null,
    count: undefined,
    breakdown: undefined,
    icon: id === 'ai' ? PatrolIcon : TextIcon,
    ...overrides,
  };
}

describe('MobileNavBar', () => {
  it('keeps the mobile nav on shell, runtime, and model owners', () => {
    expect(mobileNavBarSource).toContain('useMobileNavBarState');
    expect(mobileNavBarSource).toContain('getMobileNavTabButtonClass');
    expect(mobileNavBarSource).toContain('role="menu"');
    expect(mobileNavBarSource).toContain('<a');
    expect(mobileNavBarSource).toContain('pb-safe xl:hidden');
    expect(mobileNavBarSource).not.toContain('createSignal');
    expect(mobileNavBarSource).not.toContain('requestAnimationFrame');
    expect(mobileNavBarSource).not.toContain('overflow-x-auto');

    expect(mobileNavBarStateSource).toContain('createSignal');
    expect(mobileNavBarStateSource).toContain("document.addEventListener('pointerdown'");
    expect(mobileNavBarStateSource).toContain("event.key === 'Escape'");
    expect(mobileNavBarStateSource).toContain('export function useMobileNavBarState');
    expect(mobileNavBarStateSource).not.toContain('scrollIntoView');

    expect(mobileNavBarModelSource).toContain('buildMobileNavBarLayout');
    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavPrimaryTabs');
    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavUtilityTabs');
    expect(mobileNavBarModelSource).toContain('getMobileNavAlertBadgeCounts');
    expect(mobileNavBarModelSource).toContain("'proxmox'");
    expect(mobileNavBarModelSource).toContain("'docker'");
    expect(mobileNavBarModelSource).toContain("'kubernetes'");
    expect(mobileNavBarModelSource).toContain("'truenas'");
    expect(mobileNavBarModelSource).toContain("'vmware'");
    expect(mobileNavBarModelSource).not.toContain("'workloads'");
    expect(mobileNavBarModelSource).not.toContain("'storage'");
    expect(mobileNavBarModelSource).not.toContain("'recovery'");
  });

  it('renders a dedicated platform switcher and keeps More for secondary utilities', async () => {
    const { container } = render(() => (
      <MobileNavBar
        activeTab={() => 'standalone'}
        primaryTabs={() => [
          makePrimary('standalone', 'Machines'),
          makePrimary('proxmox', 'Proxmox'),
        ]}
        utilityTabs={() => [
          makeUtility('settings', 'Settings'),
          makeUtility('actions', 'Actions'),
          makeUtility('ai', 'Patrol'),
          makeUtility('alerts', 'Alerts'),
        ]}
        onPrimaryClick={() => {}}
        onUtilityClick={() => {}}
      />
    ));

    const fixedRail = container.querySelector('[data-mobile-nav-rail="fixed"]');
    expect(fixedRail).toBeTruthy();
    expect(fixedRail).toHaveClass('px-0.5', 'py-0.5');
    expect(fixedRail).not.toHaveClass('py-1');
    expect(
      Array.from(fixedRail?.querySelectorAll('[data-tab-id]') ?? []).map((item) =>
        item.getAttribute('data-tab-id'),
      ),
    ).toEqual(['platform-switcher', 'alerts', 'ai', 'actions', 'more']);

    const platformSwitcher = screen.getByRole('button', {
      name: 'Switch platform, current Machines',
    });
    expect(platformSwitcher).toHaveAttribute('aria-haspopup', 'menu');
    expect(platformSwitcher).toHaveAttribute('aria-expanded', 'false');
    expect(platformSwitcher).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Alerts' })).toHaveAttribute('href', '/alerts');
    expect(screen.getByRole('link', { name: 'Patrol' })).toHaveAttribute('href', '/patrol');
    expect(screen.getByRole('link', { name: 'Actions' })).toHaveAttribute('href', '/actions');
    fireEvent.click(platformSwitcher);

    const platformMenu = await screen.findByRole('menu', { name: 'Switch platform' });
    expect(platformSwitcher).toHaveAttribute('aria-expanded', 'true');
    expect(
      within(platformMenu)
        .getAllByRole('menuitem')
        .map((item) => item.getAttribute('data-tab-id')),
    ).toEqual(['proxmox', 'standalone']);
    expect(within(platformMenu).getByRole('menuitem', { name: 'Machines' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(within(platformMenu).getByRole('menuitem', { name: 'Machines' }).tagName).toBe('A');
    expect(within(platformMenu).getByRole('menuitem', { name: 'Machines' })).toHaveAttribute(
      'href',
      '/standalone/overview',
    );

    const more = screen.getByRole('button', { name: 'More navigation' });
    expect(more).toHaveAttribute('aria-haspopup', 'menu');
    expect(more).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(more);

    const menu = await screen.findByRole('menu', { name: 'More navigation destinations' });
    expect(more).toHaveAttribute('aria-expanded', 'true');
    expect(within(menu).queryByRole('group', { name: 'Infrastructure' })).toBeNull();
    expect(within(menu).getByRole('group', { name: 'Pulse' })).toBeInTheDocument();
    expect(
      within(menu)
        .getAllByRole('menuitem')
        .map((item) => item.getAttribute('data-tab-id')),
    ).toEqual(['settings']);
    expect(within(menu).getByRole('menuitem', { name: 'Settings' })).toHaveAttribute(
      'href',
      '/settings',
    );

    expect(platformSwitcher).toHaveClass('min-h-10', 'gap-0', 'py-0.5', 'text-[9px]');
  });

  it('preserves route callbacks for fixed and overflow destinations', async () => {
    const onPrimaryClick = vi.fn();
    const onUtilityClick = vi.fn();
    render(() => (
      <MobileNavBar
        activeTab={() => 'proxmox'}
        primaryTabs={() => [
          makePrimary('proxmox', 'Proxmox'),
          makePrimary('standalone', 'Machines'),
        ]}
        utilityTabs={() => [makeUtility('alerts', 'Alerts'), makeUtility('settings', 'Settings')]}
        onPrimaryClick={onPrimaryClick}
        onUtilityClick={onUtilityClick}
      />
    ));

    fireEvent.click(screen.getByRole('link', { name: 'Alerts' }));
    expect(onUtilityClick).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'alerts', route: '/alerts' }),
    );

    const platformSwitcher = screen.getByRole('button', {
      name: 'Switch platform, current Proxmox',
    });
    fireEvent.click(platformSwitcher);
    const platformMenu = await screen.findByRole('menu', { name: 'Switch platform' });
    fireEvent.click(within(platformMenu).getByRole('menuitem', { name: 'Machines' }));
    expect(onPrimaryClick).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'standalone', route: '/standalone/overview' }),
    );
    expect(screen.queryByRole('menu')).toBeNull();

    const more = screen.getByRole('button', { name: 'More navigation' });
    fireEvent.click(more);
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Settings' }));
    expect(onUtilityClick).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'settings', route: '/settings' }),
    );
  });

  it('preserves fixed and overflow badges and keeps icon labels decorative', async () => {
    render(() => (
      <MobileNavBar
        activeTab={() => 'ai'}
        primaryTabs={() => [
          makePrimary('proxmox', 'Proxmox'),
          makePrimary('standalone', 'Machines', { enabled: false }),
        ]}
        utilityTabs={() => [
          makeUtility('alerts', 'Alerts', {
            count: 5,
            breakdown: { critical: 2, warning: 3 },
          }),
          makeUtility('ai', 'Patrol', { count: 2, countLabel: '2 open work items' }),
          makeUtility('settings', 'Settings', { badge: 'update' }),
        ]}
        onPrimaryClick={() => {}}
        onUtilityClick={() => {}}
      />
    ));

    const nav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    expect(
      within(nav).getByRole('link', { name: 'Alerts: 2 critical, 3 warning' }),
    ).toHaveTextContent('23');
    expect(within(nav).getByRole('link', { name: 'Patrol: 2 open work items' })).toHaveTextContent(
      'Patrol2',
    );
    expect(within(nav).queryByRole('link', { name: 'Pulse Patrol Patrol' })).toBeNull();

    fireEvent.click(within(nav).getByRole('button', { name: 'Switch platform, current Proxmox' }));
    const menu = await screen.findByRole('menu', { name: 'Switch platform' });
    expect(within(menu).getByRole('menuitem', { name: /Machines/ })).toHaveTextContent(
      'MachinesSetup',
    );
    fireEvent.click(within(nav).getByRole('button', { name: 'More navigation' }));
    const utilityMenu = await screen.findByRole('menu', {
      name: 'More navigation destinations',
    });
    expect(within(utilityMenu).getByRole('menuitem', { name: 'Settings' })).toBeInTheDocument();
  });

  it('supports menu arrow keys, Escape focus return, and outside dismissal', async () => {
    render(() => (
      <MobileNavBar
        activeTab={() => 'proxmox'}
        primaryTabs={() => [
          makePrimary('proxmox', 'Proxmox'),
          makePrimary('standalone', 'Machines'),
        ]}
        utilityTabs={() => [makeUtility('alerts', 'Alerts'), makeUtility('settings', 'Settings')]}
        onPrimaryClick={() => {}}
        onUtilityClick={() => {}}
      />
    ));

    const platformSwitcher = screen.getByRole('button', {
      name: 'Switch platform, current Proxmox',
    });
    platformSwitcher.focus();
    fireEvent.keyDown(platformSwitcher, { key: 'ArrowDown' });
    const menu = await screen.findByRole('menu', { name: 'Switch platform' });
    const items = within(menu).getAllByRole('menuitem');
    await waitFor(() => expect(items[0]).toHaveFocus());

    fireEvent.keyDown(items[0], { key: 'ArrowDown' });
    expect(items[1]).toHaveFocus();
    fireEvent.keyDown(items[1], { key: 'Home' });
    expect(items[0]).toHaveFocus();
    fireEvent.keyDown(items[0], { key: 'End' });
    expect(items[1]).toHaveFocus();
    fireEvent.keyDown(items[1], { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull());
    expect(platformSwitcher).toHaveFocus();

    const more = screen.getByRole('button', { name: 'More navigation' });
    fireEvent.click(more);
    await screen.findByRole('menu', { name: 'More navigation destinations' });
    fireEvent.pointerDown(document.body);
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull());
  });

  it('omits More when every supplied destination fits the fixed set', () => {
    render(() => (
      <MobileNavBar
        activeTab={() => null}
        primaryTabs={() => [makePrimary('proxmox', 'Proxmox')]}
        utilityTabs={() => [makeUtility('alerts', 'Alerts'), makeUtility('ai', 'Patrol')]}
        onPrimaryClick={() => {}}
        onUtilityClick={() => {}}
      />
    ));

    expect(screen.queryByRole('button', { name: 'More navigation' })).toBeNull();
    expect(screen.getByRole('link', { name: 'Proxmox' })).toHaveAttribute(
      'href',
      '/proxmox/overview',
    );
    screen.getAllByRole('link').forEach((link) => {
      expect(link).not.toHaveAttribute('aria-current');
    });
  });

  it('keeps destination DOM identity when tab arrays are rebuilt with equal content', async () => {
    const buildTabs = () => [
      makeUtility('alerts', 'Alerts', { count: 3, breakdown: { warning: 2, critical: 1 } }),
      makeUtility('ai', 'Patrol'),
      makeUtility('actions', 'Actions'),
    ];
    const [rawTabs, setRawTabs] = createSignal(buildTabs(), { equals: false });
    const utilityTabs = createStableTabList(rawTabs, utilityNavTabEquals);
    const { container } = render(() => (
      <MobileNavBar
        activeTab={() => null}
        primaryTabs={() => [makePrimary('proxmox', 'Proxmox')]}
        utilityTabs={() => [...utilityTabs()]}
        onPrimaryClick={() => {}}
        onUtilityClick={() => {}}
      />
    ));

    const destinationsBefore = [
      ...container.querySelectorAll<HTMLElement>('[data-mobile-nav-destination]'),
    ];
    expect(destinationsBefore.length).toBeGreaterThan(0);

    // A state frame carrying an unchanged alerts array rebuilds the tab
    // objects; identical content must not recreate the rendered destinations.
    setRawTabs(buildTabs());
    await waitFor(() => {
      const destinationsAfter = [
        ...container.querySelectorAll<HTMLElement>('[data-mobile-nav-destination]'),
      ];
      expect(destinationsAfter).toHaveLength(destinationsBefore.length);
      destinationsAfter.forEach((destination, index) => {
        expect(destination).toBe(destinationsBefore[index]);
      });
      destinationsBefore.forEach((destination) => {
        expect(destination.isConnected).toBe(true);
      });
    });
  });
});

describe('MobileNavBar clearance publishing', () => {
  const NAV_HEIGHT_VARIABLE = '--pulse-mobile-nav-height';

  const renderBar = () =>
    render(() => (
      <MobileNavBar
        activeTab={() => 'proxmox'}
        primaryTabs={() => [makePrimary('proxmox', 'Proxmox')]}
        utilityTabs={() => [makeUtility('alerts', 'Alerts')]}
        onPrimaryClick={() => {}}
        onUtilityClick={() => {}}
      />
    ));

  it('publishes its measured height so overlays can clear it', () => {
    const { unmount } = renderBar();

    // Anything sitting on top of this bar reads the published value rather
    // than hardcoding one. Five call sites once hardcoded 5rem against a bar
    // that measures ~45px, leaving a band below the Assistant backdrop that
    // was neither dimmed nor click-blocked.
    expect(document.documentElement.style.getPropertyValue(NAV_HEIGHT_VARIABLE)).toMatch(
      /^\d+(\.\d+)?px$/,
    );

    unmount();
  });

  it('stops publishing once the bar is gone', () => {
    const { unmount } = renderBar();
    expect(document.documentElement.style.getPropertyValue(NAV_HEIGHT_VARIABLE)).not.toBe('');

    unmount();

    // A stale height would keep reserving space for a bar that is no longer
    // rendered, so consumers must fall back to the declared default.
    expect(document.documentElement.style.getPropertyValue(NAV_HEIGHT_VARIABLE)).toBe('');
  });

  it('measures after mount rather than from the ref', () => {
    // Solid runs refs before the node is in the document, where offsetHeight
    // is 0, so publishing from the ref reports a bar of no height.
    expect(mobileNavBarSource).toContain('onMount(');
    expect(mobileNavBarSource).not.toMatch(/ref=\{setNavRef\}/);
  });
});
