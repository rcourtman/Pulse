import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import mobileNavBarSource from '@/components/shared/MobileNavBar.tsx?raw';
import mobileNavBarModelSource from '@/components/shared/mobileNavBarModel.ts?raw';
import mobileNavBarStateSource from '@/components/shared/useMobileNavBarState.ts?raw';
import { MobileNavBar } from '@/components/shared/MobileNavBar';

HTMLElement.prototype.scrollIntoView = vi.fn();
window.requestAnimationFrame = ((callback: FrameRequestCallback) => {
  callback(0);
  return 1;
}) as typeof window.requestAnimationFrame;

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

    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavPlatformTabs');
    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavUtilityTabs');
    expect(mobileNavBarModelSource).toContain('getMobileNavAlertBadgeCounts');
    expect(mobileNavBarModelSource).toContain('getMobileNavFadeState');
  });

  it('orders tabs, renders alert badges, and shows fades from scroll state', async () => {
    const onPlatformClick = vi.fn();
    const onUtilityClick = vi.fn();

    const { container } = render(() => (
      <MobileNavBar
        activeTab={() => 'dashboard'}
        platformTabs={() => [
          {
            id: 'storage',
            label: 'Storage',
            route: '/storage',
            settingsRoute: '/settings/storage',
            tooltip: 'Storage',
            enabled: true,
            live: true,
            icon: <span>ST</span>,
            alwaysShow: true,
          },
          {
            id: 'dashboard',
            label: 'Dashboard',
            route: '/dashboard',
            settingsRoute: '/settings/dashboard',
            tooltip: 'Dashboard',
            enabled: true,
            live: true,
            icon: <span>DB</span>,
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
            icon: <span>SE</span>,
          },
          {
            id: 'alerts',
            label: 'Alerts',
            route: '/alerts',
            tooltip: 'Alerts',
            badge: null,
            count: 5,
            breakdown: { critical: 2, warning: 3 },
            icon: <span>AL</span>,
          },
        ]}
        onPlatformClick={onPlatformClick}
        onUtilityClick={onUtilityClick}
      />
    ));

    const navList = screen.getByRole('tablist', { name: 'Mobile navigation' });
    Object.defineProperty(navList, 'scrollWidth', { configurable: true, value: 400 });
    Object.defineProperty(navList, 'clientWidth', { configurable: true, value: 200 });
    Object.defineProperty(navList, 'scrollLeft', { configurable: true, value: 20, writable: true });

    fireEvent.scroll(navList);

    const buttons = container.querySelectorAll('button[data-tab-id]');
    expect(buttons[0]).toHaveAttribute('data-tab-id', 'dashboard');
    expect(buttons[1]).toHaveAttribute('data-tab-id', 'storage');

    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Pro')).toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Storage'));
    expect(onPlatformClick).toHaveBeenCalledWith(expect.objectContaining({ id: 'storage' }));

    fireEvent.click(screen.getByTitle('Alerts'));
    expect(onUtilityClick).toHaveBeenCalledWith(expect.objectContaining({ id: 'alerts' }));

    await waitFor(() => {
      expect(container.querySelector('.bg-gradient-to-r')).toBeTruthy();
      expect(container.querySelector('.bg-gradient-to-l')).toBeTruthy();
    });
  });
});
