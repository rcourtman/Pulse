import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router, useNavigate } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { AppLayout, resetPrimaryNavigationRouteMemory } from '@/AppLayout';
import { aiChatStore } from '@/stores/aiChat';

HTMLElement.prototype.scrollIntoView = vi.fn();
window.scrollTo = vi.fn();

const aiIntelligenceMockState = vi.hoisted(() => ({
  patrolOpenWorkCount: 0,
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get patrolOpenWorkCount() {
      return aiIntelligenceMockState.patrolOpenWorkCount;
    },
  },
}));

vi.mock('@/routing/routePreload', () => ({
  preloadRouteModule: vi.fn(() => Promise.resolve()),
}));

describe('AppLayout navigation icons', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/settings/infrastructure');
    resetPrimaryNavigationRouteMemory();
    aiIntelligenceMockState.patrolOpenWorkCount = 0;
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

  const renderLayout = (resources: Resource[] = [], initialPath = '/settings/infrastructure') => {
    window.history.replaceState({}, '', initialPath);
    const RouteStateProbe = () => {
      const navigate = useNavigate();
      return (
        <button
          type="button"
          onClick={() => navigate('/proxmox/overview?status=running', { replace: true })}
        >
          Set Proxmox running filter
        </button>
      );
    };
    const LayoutRoute = () => (
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
        <RouteStateProbe />
      </AppLayout>
    );
    return render(() => (
      <Router>
        <Route path="/settings/infrastructure" component={LayoutRoute} />
        <Route path="/proxmox/overview" component={LayoutRoute} />
        <Route path="/docker/overview" component={LayoutRoute} />
      </Router>
    ));
  };

  const getInfrastructureTab = (name: string) => {
    const desktopNav = screen.getByRole('tablist', { name: 'Primary navigation' });
    const infrastructureGroup = desktopNav.querySelector('[aria-label="Infrastructure"]');
    expect(infrastructureGroup).toBeTruthy();
    return within(infrastructureGroup as HTMLElement).getByRole('tab', { name });
  };

  const platformResources = () => [
    makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
    makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' }),
  ];

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
      within(systemGroup as HTMLElement).queryByRole('tab', {
        name: 'Pulse Patrol Patrol',
      }),
    ).toBeNull();
    expect(within(systemGroup as HTMLElement).queryByRole('tab', { name: 'Patrol P' })).toBeNull();

    const mobileTablist = screen.getByRole('tablist', { name: 'Mobile navigation' });
    ['alerts', 'ai', 'settings'].forEach((tabId) => {
      const button = mobileTablist.querySelector<HTMLElement>(`[data-tab-id="${tabId}"]`);
      expect(button).toBeTruthy();
      expect(button?.querySelector('svg')).toBeTruthy();
    });
    const mobilePatrolTab = within(mobileTablist).getByRole('button', {
      name: 'Patrol',
    });
    expect(mobilePatrolTab.querySelector('svg')).toBeTruthy();

    expect(container).toHaveTextContent('Infrastructure body');
  });

  it('surfaces Patrol open work as a count without renaming Patrol', () => {
    aiIntelligenceMockState.patrolOpenWorkCount = 2;
    renderLayout();

    const desktopNav = screen.getByRole('tablist', { name: 'Primary navigation' });
    const systemGroup = desktopNav.querySelector('[aria-label="System"]');
    expect(systemGroup).toBeTruthy();

    const desktopPatrolTab = within(systemGroup as HTMLElement).getByRole('tab', {
      name: 'Patrol: 2 open work items',
    });
    expect(desktopPatrolTab).toHaveTextContent('Patrol');
    expect(desktopPatrolTab).toHaveTextContent('2');
    expect(within(systemGroup as HTMLElement).queryByText('Needs Attention')).toBeNull();

    const mobileTablist = screen.getByRole('tablist', { name: 'Mobile navigation' });
    const mobilePatrolTab = within(mobileTablist).getByRole('button', {
      name: 'Patrol: 2 open work items',
    });
    expect(mobilePatrolTab).toHaveTextContent('Patrol');
    expect(mobilePatrolTab).toHaveTextContent('2');
    expect(within(mobileTablist).queryByText('Needs Attention')).toBeNull();
  });

  it('shows platform and runtime lens tabs with supported infrastructure evidence', () => {
    renderLayout([
      makeResource({ id: 'agent-1', type: 'agent', platformType: 'agent' }),
      makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
      makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' }),
      makeResource({ id: 'vcenter-1', type: 'vm', platformType: 'vmware-vsphere' }),
    ]);

    const desktopNav = screen.getByRole('tablist', { name: 'Primary navigation' });
    const infrastructureGroup = desktopNav.querySelector('[aria-label="Infrastructure"]');
    expect(infrastructureGroup).toBeTruthy();

    expect(
      within(infrastructureGroup as HTMLElement)
        .getAllByRole('tab')
        .map((tab) => tab.getAttribute('aria-label')),
    ).toEqual(['Proxmox', 'Docker', 'vSphere', 'Machines']);
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('tab', { name: 'Machines' }),
    ).toBeTruthy();
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('tab', { name: 'Proxmox' }),
    ).toBeTruthy();
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('tab', { name: 'Docker' }),
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

  it('restores the previous Proxmox route state when returning from another platform tab', async () => {
    renderLayout(platformResources(), '/proxmox/overview?status=running');

    await fireEvent.click(getInfrastructureTab('Docker'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/docker/overview');
      expect(window.location.search).toBe('');
    });

    await fireEvent.click(getInfrastructureTab('Proxmox'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/proxmox/overview');
      expect(window.location.search).toBe('?status=running');
    });
  });

  it('restores Proxmox route state changed after the page has loaded', async () => {
    renderLayout(platformResources(), '/proxmox/overview');

    await fireEvent.click(screen.getByRole('button', { name: 'Set Proxmox running filter' }));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/proxmox/overview');
      expect(window.location.search).toBe('?status=running');
    });

    await fireEvent.click(getInfrastructureTab('Docker'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/docker/overview');
      expect(window.location.search).toBe('');
    });

    await fireEvent.click(getInfrastructureTab('Proxmox'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/proxmox/overview');
      expect(window.location.search).toBe('?status=running');
    });
  });

  it('keeps remembered route state scoped to the platform tab that owns it', async () => {
    renderLayout(platformResources(), '/docker/overview?host=docker-1');

    await fireEvent.click(getInfrastructureTab('Proxmox'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/proxmox/overview');
      expect(window.location.search).toBe('');
    });

    await fireEvent.click(getInfrastructureTab('Docker'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/docker/overview');
      expect(window.location.search).toBe('?host=docker-1');
    });
  });

  it('uses the canonical platform root route when there is no remembered route state', async () => {
    renderLayout(platformResources(), '/settings/infrastructure');

    await fireEvent.click(getInfrastructureTab('Proxmox'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/proxmox/overview');
      expect(window.location.search).toBe('');
    });
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

    const launcher = screen.getByRole('button', { name: 'Ask Pulse Assistant about Settings' });
    const launcherClass = launcher.getAttribute('class') ?? '';

    expect(launcherClass).toContain('right-4');
    expect(launcherClass).toContain('bottom-[calc(5rem+env(safe-area-inset-bottom,0px))]');
    expect(launcherClass).toContain('rounded-full');
    expect(launcherClass).toContain('lg:right-0');
    expect(launcherClass).toContain('lg:top-1/2');
    expect(launcherClass).toContain('lg:bottom-auto');
    expect(launcherClass).not.toContain('sm:top-1/2');
  });

  it('opens Assistant with the current route attached', async () => {
    const openAssistant = vi.spyOn(aiChatStore, 'open').mockImplementation(() => {});
    renderLayout();

    await fireEvent.click(
      screen.getByRole('button', { name: 'Ask Pulse Assistant about Settings' }),
    );

    await waitFor(() => {
      expect(openAssistant).toHaveBeenCalledWith(
        expect.objectContaining({
          targetType: 'pulse-view',
          targetId: '/settings/infrastructure',
          context: expect.objectContaining({
            name: 'Settings',
            route: '/settings/infrastructure',
            surface: 'settings',
          }),
          briefing: expect.objectContaining({
            sourceLabel: 'Current view',
            title: 'Settings attached',
            statusLabel: 'Context only',
          }),
        }),
      );
    });
  });
});
