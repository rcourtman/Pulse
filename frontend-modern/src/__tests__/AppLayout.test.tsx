import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { AppLayout } from '@/AppLayout';
import { aiChatStore } from '@/stores/aiChat';

HTMLElement.prototype.scrollIntoView = vi.fn();

describe('AppLayout navigation icons', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/infrastructure');
    aiChatStore.close();
    aiChatStore.setEnabled(true);
  });

  afterEach(() => {
    aiChatStore.close();
    aiChatStore.setEnabled(false);
    cleanup();
  });

  const makeResource = (overrides: Partial<Resource>): Resource =>
    ({
      id: overrides.id ?? 'resource-1',
      name: overrides.name ?? overrides.id ?? 'resource-1',
      displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'resource-1',
      type: overrides.type ?? 'agent',
      platformId: overrides.platformId ?? 'platform-1',
      platformType: overrides.platformType ?? 'agent',
      sourceType: overrides.sourceType ?? 'api',
      status: overrides.status ?? 'online',
      lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
      ...overrides,
    }) as Resource;

  const renderLayout = (resources: Resource[] = []) =>
    render(() => (
      <Router>
        <Route
          path="/infrastructure"
          component={() => (
            <AppLayout
              connectionStatus={() => ({
                kind: 'connected',
                label: 'Connected',
                detail: 'Backend and live data stream are connected.',
                tone: 'healthy',
              })}
              lastUpdateText={() => ''}
              versionInfo={() =>
                ({
                  version: '6.0.0-rc.2',
                  channel: 'rc',
                  isDevelopment: false,
                  isDocker: false,
                }) as never
              }
              hasAuth={() => true}
              needsAuth={() => false}
              proxyAuthInfo={() => null}
              handleLogout={() => {}}
              state={() =>
                ({
                  activeAlerts: [{ id: 'alert-1', level: 'warning', acknowledged: false }],
                  resources,
                }) as unknown as State
              }
              tokenScopes={() => ['settings:read']}
              organizations={() => []}
              activeOrgID={() => 'default'}
              orgsLoading={() => false}
              showOrgSwitcher={() => false}
              onSwitchOrg={() => {}}
            >
              <div>Infrastructure body</div>
            </AppLayout>
          )}
        />
      </Router>
    ));

  it('renders fresh utility icons for both desktop and mobile navigation trees', () => {
    const { container } = renderLayout();

    const desktopNav = screen.getByRole('tablist', { name: 'Primary navigation' });
    const systemGroup = desktopNav.querySelector('[aria-label="System"]');
    expect(systemGroup).toBeTruthy();

    const desktopTabs = within(systemGroup as HTMLElement).getAllByRole('tab');
    expect(desktopTabs).toHaveLength(3);
    desktopTabs.forEach((tab) => {
      expect(tab.querySelector('svg')).toBeTruthy();
    });
    const desktopPatrolTab = within(systemGroup as HTMLElement).getByRole('tab', {
      name: 'Patrol',
    });
    expect(desktopPatrolTab.querySelector('svg')).toBeTruthy();
    expect(within(systemGroup as HTMLElement).getByRole('tab', { name: '1 Alerts' })).toBeTruthy();
    expect(
      within(systemGroup as HTMLElement).queryByRole('tab', { name: 'Pulse Patrol Patrol' }),
    ).toBeNull();
    expect(within(systemGroup as HTMLElement).queryByRole('tab', { name: 'Patrol P' })).toBeNull();

    const mobileTablist = screen.getByRole('tablist', { name: 'Mobile navigation' });
    ['alerts', 'ai', 'settings'].forEach((tabId) => {
      const button = mobileTablist.querySelector<HTMLElement>(`[data-tab-id="${tabId}"]`);
      expect(button).toBeTruthy();
      expect(button?.querySelector('svg')).toBeTruthy();
    });
    const mobilePatrolTab = within(mobileTablist).getByRole('button', { name: 'Patrol' });
    expect(mobilePatrolTab.querySelector('svg')).toBeTruthy();

    expect(container).toHaveTextContent('Infrastructure body');
  });

  it('shows platform and runtime lens tabs with supported infrastructure evidence', () => {
    renderLayout([
      makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
      makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' }),
      makeResource({ id: 'vcenter-1', type: 'vm', platformType: 'vmware-vsphere' }),
    ]);

    const desktopNav = screen.getByRole('tablist', { name: 'Primary navigation' });
    const infrastructureGroup = desktopNav.querySelector('[aria-label="Infrastructure"]');
    expect(infrastructureGroup).toBeTruthy();

    expect(
      within(infrastructureGroup as HTMLElement).getByRole('tab', { name: 'Proxmox' }),
    ).toBeTruthy();
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('tab', { name: 'Containers' }),
    ).toBeTruthy();
    expect(
      within(infrastructureGroup as HTMLElement).queryByRole('tab', { name: 'Kubernetes' }),
    ).toBeNull();
    expect(
      within(infrastructureGroup as HTMLElement).queryByRole('tab', { name: 'TrueNAS' }),
    ).toBeNull();
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('tab', { name: 'vSphere' }),
    ).toBeTruthy();
  });

  it('keeps connected brand motion on the logo while the wordmark stays static', () => {
    const { container } = renderLayout();

    const brandLockup = screen.getByTestId('pulse-brand-lockup');
    expect(brandLockup).toHaveClass('animate-pulse-brand');
    expect(brandLockup.querySelector('.pulse-brand-logo')).toBeTruthy();
    const wordmark = brandLockup.querySelector('.pulse-brand-wordmark');
    expect(wordmark).toHaveTextContent('Pulse');
    expect(wordmark).not.toHaveClass('animate-pulse-brand');
    expect(container.querySelector('.animate-pulse-logo')).toBeNull();
  });

  it('keeps the assistant launcher clear of the mobile navigation breakpoint', () => {
    renderLayout();

    const launcher = screen.getByRole('button', { name: 'Expand Pulse Assistant' });
    const launcherClass = launcher.getAttribute('class') ?? '';

    expect(launcherClass).toContain('right-4');
    expect(launcherClass).toContain('bottom-[calc(5rem+env(safe-area-inset-bottom,0px))]');
    expect(launcherClass).toContain('rounded-full');
    expect(launcherClass).toContain('lg:right-0');
    expect(launcherClass).toContain('lg:top-1/2');
    expect(launcherClass).toContain('lg:bottom-auto');
    expect(launcherClass).not.toContain('sm:top-1/2');
  });
});
