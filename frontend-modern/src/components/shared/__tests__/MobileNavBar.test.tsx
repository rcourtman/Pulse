import { fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { type Component } from 'solid-js';
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

  it('renders one platform and the daily operations set with every other route in More', async () => {
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
      Array.from(fixedRail?.querySelectorAll('button[data-tab-id]') ?? []).map((button) =>
        button.getAttribute('data-tab-id'),
      ),
    ).toEqual(['proxmox', 'alerts', 'ai', 'actions', 'more']);

    const more = screen.getByRole('button', { name: 'More navigation' });
    expect(more).toHaveAttribute('aria-haspopup', 'menu');
    expect(more).toHaveAttribute('aria-expanded', 'false');
    expect(more).toHaveAttribute('aria-current', 'page');
    fireEvent.click(more);

    const menu = await screen.findByRole('menu', { name: 'More navigation destinations' });
    expect(more).toHaveAttribute('aria-expanded', 'true');
    expect(within(menu).getByRole('group', { name: 'Infrastructure' })).toBeInTheDocument();
    expect(within(menu).getByRole('group', { name: 'Pulse' })).toBeInTheDocument();
    expect(
      within(menu)
        .getAllByRole('menuitem')
        .map((item) => item.getAttribute('data-tab-id')),
    ).toEqual(['standalone', 'settings']);
    expect(within(menu).getByRole('menuitem', { name: 'Machines' })).toHaveAttribute(
      'aria-current',
      'page',
    );

    const fixedProxmox = fixedRail?.querySelector('[data-tab-id="proxmox"]');
    expect(fixedProxmox).toHaveClass('min-h-10', 'gap-0', 'py-0.5', 'text-[9px]');
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

    fireEvent.click(screen.getByRole('button', { name: 'Alerts' }));
    expect(onUtilityClick).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'alerts', route: '/alerts' }),
    );

    const more = screen.getByRole('button', { name: 'More navigation' });
    fireEvent.click(more);
    const menu = await screen.findByRole('menu');
    fireEvent.click(within(menu).getByRole('menuitem', { name: 'Machines' }));
    expect(onPrimaryClick).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'standalone', route: '/standalone/overview' }),
    );
    expect(screen.queryByRole('menu')).toBeNull();

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
      within(nav).getByRole('button', { name: 'Alerts: 2 critical, 3 warning' }),
    ).toHaveTextContent('23');
    expect(
      within(nav).getByRole('button', { name: 'Patrol: 2 open work items' }),
    ).toHaveTextContent('Patrol2');
    expect(within(nav).queryByRole('button', { name: 'Pulse Patrol Patrol' })).toBeNull();

    fireEvent.click(within(nav).getByRole('button', { name: 'More navigation' }));
    const menu = await screen.findByRole('menu');
    expect(within(menu).getByRole('menuitem', { name: /Machines/ })).toHaveTextContent(
      'MachinesSetup',
    );
    expect(within(menu).getByRole('menuitem', { name: 'Settings' })).toBeInTheDocument();
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

    const more = screen.getByRole('button', { name: 'More navigation' });
    more.focus();
    fireEvent.keyDown(more, { key: 'ArrowDown' });
    const menu = await screen.findByRole('menu');
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
    expect(more).toHaveFocus();

    fireEvent.click(more);
    await screen.findByRole('menu');
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
    screen.getAllByRole('button').forEach((button) => {
      expect(button).not.toHaveAttribute('aria-current');
    });
  });
});
