import { fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import type { Component } from 'solid-js';
import mobileNavBarSource from '@/components/shared/MobileNavBar.tsx?raw';
import mobileNavBarModelSource from '@/components/shared/mobileNavBarModel.ts?raw';
import mobileNavBarStateSource from '@/components/shared/useMobileNavBarState.ts?raw';
import { MobileNavBar } from '@/components/shared/MobileNavBar';

HTMLElement.prototype.scrollIntoView = vi.fn();
window.requestAnimationFrame = ((callback: FrameRequestCallback) => {
  callback(0);
  return 1;
}) as typeof window.requestAnimationFrame;

const StandaloneIcon: Component<{ class?: string }> = (props) => (
  <span class={props.class}>SA</span>
);
const ProxmoxIcon: Component<{ class?: string }> = (props) => <span class={props.class}>PX</span>;
const AlertsIcon: Component<{ class?: string }> = (props) => <span class={props.class}>AL</span>;
const SettingsIcon: Component<{ class?: string }> = (props) => <span class={props.class}>SE</span>;
const PatrolIcon: Component<{ class?: string }> = (props) => (
  <svg aria-label="Pulse Patrol" class={props.class} viewBox="0 0 24 24">
    <title>Pulse Patrol</title>
    <circle cx="12" cy="12" r="8" />
  </svg>
);

describe('MobileNavBar', () => {
  it('keeps the mobile nav on shell, runtime, and model owners', () => {
    expect(mobileNavBarSource).toContain('useMobileNavBarState');
    expect(mobileNavBarSource).toContain('getMobileNavTabButtonClass');
    expect(mobileNavBarSource).not.toContain('createSignal');
    expect(mobileNavBarSource).not.toContain('requestAnimationFrame');
    expect(mobileNavBarSource).not.toContain('new Set(priority)');

    expect(mobileNavBarStateSource).toContain('createSignal');
    expect(mobileNavBarStateSource).toContain('window.addEventListener');
    expect(mobileNavBarStateSource).toContain('requestAnimationFrame');
    expect(mobileNavBarStateSource).toContain('scrollIntoView');
    expect(mobileNavBarStateSource).toContain('export function useMobileNavBarState');

    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavPrimaryTabs');
    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavUtilityTabs');
    expect(mobileNavBarModelSource).toContain('getMobileNavAlertBadgeCounts');
    expect(mobileNavBarModelSource).toContain('getMobileNavFadeState');
    expect(mobileNavBarModelSource).toContain("'proxmox'");
    expect(mobileNavBarModelSource).toContain("'docker'");
    expect(mobileNavBarModelSource).toContain("'kubernetes'");
    expect(mobileNavBarModelSource).toContain("'truenas'");
    expect(mobileNavBarModelSource).toContain("'vmware'");
    expect(mobileNavBarModelSource).not.toContain("'workloads'");
    expect(mobileNavBarModelSource).not.toContain("'storage'");
    expect(mobileNavBarModelSource).not.toContain("'recovery'");
    expect(mobileNavBarModelSource).not.toContain("'infrastructure'");
  });

  it('keeps decorative icon labels out of mobile tab accessible names', () => {
    render(() => (
      <MobileNavBar
        activeTab={() => 'ai'}
        primaryTabs={() => []}
        utilityTabs={() => [
          {
            id: 'ai',
            label: 'Patrol',
            route: '/patrol',
            tooltip: 'Continuous verification',
            badge: null,
            count: undefined,
            breakdown: undefined,
            icon: PatrolIcon,
          },
        ]}
        onPrimaryClick={() => {}}
        onUtilityClick={() => {}}
      />
    ));

    const navList = screen.getByRole('tablist', { name: 'Mobile navigation' });
    const patrolButton = within(navList).getByRole('button', { name: 'Patrol' });

    expect(patrolButton).toHaveAttribute('data-tab-id', 'ai');
    expect(within(navList).queryByRole('button', { name: 'Pulse Patrol Patrol' })).toBeNull();
  });

  it('allows inactive platform tabs to render without an active mobile tab', () => {
    const { container } = render(() => (
      <MobileNavBar
        activeTab={() => null}
        primaryTabs={() => [
          {
            id: 'standalone',
            label: 'Standalone',
            route: '/standalone/overview',
            settingsRoute: '/settings/infrastructure',
            tooltip: 'Standalone',
            enabled: true,
            live: true,
            icon: StandaloneIcon,
            alwaysShow: true,
          },
        ]}
        utilityTabs={() => [
          {
            id: 'settings',
            label: 'Settings',
            route: '/settings',
            tooltip: 'Settings',
            badge: null,
            count: undefined,
            breakdown: undefined,
            icon: SettingsIcon,
          },
        ]}
        onPrimaryClick={() => {}}
        onUtilityClick={() => {}}
      />
    ));

    const buttons = container.querySelectorAll('button[data-tab-id]');
    expect(buttons).toHaveLength(2);
    buttons.forEach((button) => {
      expect(button).not.toHaveClass('bg-blue-50');
      expect(button).not.toHaveClass('text-blue-700');
    });
  });

  it('orders tabs, renders alert badges, and shows fades from scroll state', async () => {
    const onPrimaryClick = vi.fn();
    const onUtilityClick = vi.fn();

    const { container } = render(() => (
      <MobileNavBar
        activeTab={() => 'proxmox'}
        primaryTabs={() => [
          {
            id: 'standalone',
            label: 'Standalone',
            route: '/standalone/overview',
            settingsRoute: '/settings/infrastructure',
            tooltip: 'Standalone',
            enabled: true,
            live: true,
            icon: StandaloneIcon,
            alwaysShow: true,
          },
          {
            id: 'proxmox',
            label: 'Proxmox',
            route: '/proxmox/overview',
            settingsRoute: '/settings/infrastructure',
            tooltip: 'Proxmox',
            enabled: true,
            live: true,
            icon: ProxmoxIcon,
            alwaysShow: true,
          },
        ]}
        utilityTabs={() => [
          {
            id: 'settings',
            label: 'Settings',
            route: '/settings',
            tooltip: 'Settings',
            badge: 'pro',
            count: undefined,
            breakdown: undefined,
            icon: SettingsIcon,
          },
          {
            id: 'alerts',
            label: 'Alerts',
            route: '/alerts',
            tooltip: 'Alerts',
            badge: null,
            count: 5,
            breakdown: { critical: 2, warning: 3 },
            icon: AlertsIcon,
          },
        ]}
        onPrimaryClick={onPrimaryClick}
        onUtilityClick={onUtilityClick}
      />
    ));

    const navList = screen.getByRole('tablist', { name: 'Mobile navigation' });
    Object.defineProperty(navList, 'scrollWidth', { configurable: true, value: 400 });
    Object.defineProperty(navList, 'clientWidth', { configurable: true, value: 200 });
    Object.defineProperty(navList, 'scrollLeft', { configurable: true, value: 20, writable: true });

    fireEvent.scroll(navList);

    const buttons = container.querySelectorAll('button[data-tab-id]');
    expect(buttons[0]).toHaveAttribute('data-tab-id', 'proxmox');
    expect(buttons[1]).toHaveAttribute('data-tab-id', 'standalone');
    expect(buttons[2]).toHaveAttribute('data-tab-id', 'alerts');
    expect(buttons[3]).toHaveAttribute('data-tab-id', 'settings');

    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Pro')).toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Standalone'));
    expect(onPrimaryClick).toHaveBeenCalledWith(expect.objectContaining({ id: 'standalone' }));

    fireEvent.click(screen.getByTitle('Alerts'));
    expect(onUtilityClick).toHaveBeenCalledWith(expect.objectContaining({ id: 'alerts' }));

    await waitFor(() => {
      expect(container.querySelector('.bg-gradient-to-r')).toBeTruthy();
      expect(container.querySelector('.bg-gradient-to-l')).toBeTruthy();
    });
  });
});
