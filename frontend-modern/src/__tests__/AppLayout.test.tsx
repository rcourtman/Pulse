import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router, useNavigate } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { State } from '@/types/api';
import { createSignal, type Accessor } from 'solid-js';
import type { AppConnectionStatus } from '@/useAppRuntimeState';
import type { Resource } from '@/types/resource';
import {
  AppLayout,
  resetPrimaryNavigationRouteMemory,
  sessionHasSettingsAccess,
} from '@/AppLayout';
import { isKioskMode, setKioskMode } from '@/utils/url';
import type { PlatformNavigationVisibility } from '@/features/platformNavigation/platformNavigationModel';
import { aiChatStore } from '@/stores/aiChat';
import { clearRuntimeBranding, updateRuntimeBrandingFromResponse } from '@/stores/systemSettings';

HTMLElement.prototype.scrollIntoView = vi.fn();
window.scrollTo = vi.fn();

function setViewportWidth(width: number) {
  Object.defineProperty(window, 'innerWidth', {
    value: width,
    writable: true,
    configurable: true,
  });
  window.dispatchEvent(new Event('resize'));
}

const patrolAttentionMockState = vi.hoisted(() => ({
  activeCount: 0,
}));
const actionInboxMockState = vi.hoisted(() => ({
  pendingActionCount: 0,
}));
const preloadRouteModuleMock = vi.hoisted(() => vi.fn(() => Promise.resolve()));

vi.mock('@/stores/patrolAttention', () => ({
  patrolAttentionStore: {
    summary: () => ({
      activeCount: patrolAttentionMockState.activeCount,
    }),
  },
}));

vi.mock('@/stores/actionInbox', () => ({
  actionInboxStore: {
    get pendingActionCount() {
      return actionInboxMockState.pendingActionCount;
    },
  },
}));

vi.mock('@/routing/routePreload', () => ({
  preloadRouteModule: preloadRouteModuleMock,
}));

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

const renderLayout = (
  resources: Resource[] = [],
  initialPath = '/settings/infrastructure',
  platformVisibility?: PlatformNavigationVisibility,
  tokenScopes: string[] = ['settings:read'],
  connectionStatus: Accessor<AppConnectionStatus> = () => ({
    kind: 'connected',
    label: 'Connected',
    detail: 'Backend and live data stream are connected.',
    tone: 'healthy',
  }),
) => {
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
      connectionStatus={connectionStatus}
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
      platformVisibility={platformVisibility ? () => platformVisibility : undefined}
      tokenScopes={() => tokenScopes}
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
      <Route path="/alerts" component={LayoutRoute} />
      <Route path="/actions" component={LayoutRoute} />
    </Router>
  ));
};

describe('AppLayout navigation icons', () => {
  beforeEach(() => {
    setViewportWidth(1440);
    window.history.replaceState({}, '', '/settings/infrastructure');
    resetPrimaryNavigationRouteMemory();
    patrolAttentionMockState.activeCount = 0;
    actionInboxMockState.pendingActionCount = 0;
    preloadRouteModuleMock.mockReset();
    preloadRouteModuleMock.mockResolvedValue(undefined);
    clearRuntimeBranding();
    aiChatStore.close();
    aiChatStore.setEnabled(true);
  });

  afterEach(() => {
    aiChatStore.close();
    aiChatStore.setEnabled(false);
    clearRuntimeBranding();
    cleanup();
  });

  const getInfrastructureLink = (name: string) => {
    const desktopNav = screen.getByRole('navigation', { name: 'Primary navigation' });
    const infrastructureGroup = desktopNav.querySelector('[aria-label="Infrastructure"]');
    expect(infrastructureGroup).toBeTruthy();
    return within(infrastructureGroup as HTMLElement).getByRole('link', { name });
  };

  const platformResources = () => [
    makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
    makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' }),
  ];

  it('renders fresh utility icons for both desktop and mobile navigation trees', () => {
    const { container } = renderLayout();

    expect(container.querySelector('.pulse-shell')).toHaveClass('pb-safe-or-14');
    expect(container.querySelector('.pulse-shell')).not.toHaveClass('pb-safe-or-16');
    expect(container.querySelector('.header')).toHaveClass('mb-1', 'sm:mb-3');
    const main = container.querySelector('main');
    expect(main).toHaveClass('mb-1', 'sm:mb-2');
    expect(main).toHaveAttribute('id', 'main');
    expect(main).toHaveAttribute('tabindex', '-1');
    expect(screen.getByText('Preview')).toHaveClass('bg-orange-700', 'text-white');
    expect(screen.getByText('Preview')).not.toHaveClass('bg-orange-500');
    expect(container.querySelector('footer')).toHaveClass('pulse-footer', 'px-2', 'sm:px-4');

    const desktopNav = screen.getByRole('navigation', { name: 'Primary navigation' });
    const systemGroup = desktopNav.querySelector('[aria-label="System"]');
    expect(systemGroup).toBeTruthy();

    const desktopTabs = within(systemGroup as HTMLElement).getAllByRole('link');
    expect(desktopTabs).toHaveLength(4);
    desktopTabs.forEach((tab) => {
      expect(tab.tagName).toBe('A');
      expect(tab.querySelector('svg')).toBeTruthy();
    });
    const desktopPatrolTab = within(systemGroup as HTMLElement).getByRole('link', {
      name: 'Patrol',
    });
    expect(desktopPatrolTab.querySelector('svg')).toBeTruthy();
    expect(within(systemGroup as HTMLElement).getByRole('link', { name: '1 Alerts' })).toBeTruthy();
    expect(
      within(systemGroup as HTMLElement).queryByRole('link', {
        name: 'Pulse Patrol Patrol',
      }),
    ).toBeNull();
    expect(within(systemGroup as HTMLElement).queryByRole('link', { name: 'Patrol P' })).toBeNull();

    const mobileNav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    ['alerts', 'ai', 'actions'].forEach((tabId) => {
      const button = mobileNav.querySelector<HTMLElement>(`[data-tab-id="${tabId}"]`);
      expect(button).toBeTruthy();
      expect(button?.querySelector('svg')).toBeTruthy();
    });
    const mobilePatrolTab = within(mobileNav).getByRole('link', {
      name: 'Patrol',
    });
    expect(mobilePatrolTab.querySelector('svg')).toBeTruthy();

    fireEvent.click(within(mobileNav).getByRole('button', { name: 'More navigation' }));
    const mobileOverflow = screen.getByRole('menu', { name: 'More navigation destinations' });
    const mobileSettingsTab = within(mobileOverflow).getByRole('menuitem', { name: 'Settings' });
    expect(mobileSettingsTab.querySelector('svg')).toBeTruthy();

    expect(container).toHaveTextContent('Infrastructure body');
  });

  it('retains navigation labels, icons and focus as backend health changes to sync reconnecting', async () => {
    // #1899: isolate the shell status transition from inventory changes. This
    // is not a reproduction of the reporter's socket/proxy or CSS environment.
    const [connectionStatus, setConnectionStatus] = createSignal<AppConnectionStatus>({
      kind: 'backend-healthy',
      label: 'Backend healthy',
      detail: 'Backend is healthy, but the live data stream is not connected.',
      tone: 'warning',
    });
    renderLayout(
      platformResources(),
      '/settings/infrastructure',
      undefined,
      ['settings:read'],
      connectionStatus,
    );
    const navigation = screen.getByRole('navigation', { name: 'Primary navigation' });
    const links = within(navigation).getAllByRole('link');
    const labels = links.map((link) => link.textContent);
    const proxmox = getInfrastructureLink('Proxmox');
    proxmox.focus();
    expect(proxmox).toHaveFocus();

    for (const status of [
      { kind: 'sync-reconnecting', label: 'Sync reconnecting', tone: 'warning' },
      { kind: 'connected', label: 'Connected', tone: 'healthy' },
      { kind: 'sync-reconnecting', label: 'Sync reconnecting', tone: 'warning' },
    ] as const) {
      setConnectionStatus({ ...status, detail: status.label });
      await waitFor(() => expect(screen.getByText(status.label)).toBeInTheDocument());
      expect(navigation).toBeInTheDocument();
      const currentLinks = within(navigation).getAllByRole('link');
      expect(currentLinks.map((link) => link.textContent)).toEqual(labels);
      currentLinks.forEach((link, index) => {
        expect(link).toBe(links[index]);
        expect(link.querySelector('svg')).toBeTruthy();
      });
      expect(proxmox).toHaveFocus();
    }
  });

  it('surfaces canonical Patrol attention as a count without renaming Patrol', () => {
    patrolAttentionMockState.activeCount = 2;
    renderLayout();

    const desktopNav = screen.getByRole('navigation', { name: 'Primary navigation' });
    const systemGroup = desktopNav.querySelector('[aria-label="System"]');
    expect(systemGroup).toBeTruthy();

    const desktopPatrolTab = within(systemGroup as HTMLElement).getByRole('link', {
      name: 'Patrol: 2 active attention items',
    });
    expect(desktopPatrolTab).toHaveTextContent('Patrol');
    expect(desktopPatrolTab).toHaveTextContent('2');
    expect(within(systemGroup as HTMLElement).queryByText('Needs Attention')).toBeNull();

    const mobileNav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    const mobilePatrolTab = within(mobileNav).getByRole('link', {
      name: 'Patrol: 2 active attention items',
    });
    expect(mobilePatrolTab).toHaveTextContent('Patrol');
    expect(mobilePatrolTab).toHaveTextContent('2');
    expect(within(mobileNav).queryByText('Needs Attention')).toBeNull();
  });

  it('gives Actions its own navigation state and approval count', () => {
    actionInboxMockState.pendingActionCount = 3;
    renderLayout([], '/actions');

    const desktopNav = screen.getByRole('navigation', { name: 'Primary navigation' });
    const systemGroup = desktopNav.querySelector('[aria-label="System"]');
    expect(systemGroup).toBeTruthy();
    const actionsTab = within(systemGroup as HTMLElement).getByRole('link', {
      name: 'Actions: 3 actions await approval',
    });
    expect(actionsTab.className).toContain('text-blue-600');
    expect(actionsTab).toHaveAttribute('href', '/actions');
    expect(actionsTab).toHaveAttribute('aria-current', 'page');
    expect(within(systemGroup as HTMLElement).getByRole('link', { name: 'Patrol' })).toBeTruthy();

    const mobileNav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    expect(
      within(mobileNav).getByRole('link', { name: 'Actions: 3 actions await approval' }),
    ).toHaveAttribute('aria-current', 'page');
    expect(document.title).toContain('Actions');
  });

  it('commits utility navigation without waiting for a cold route preload', async () => {
    setViewportWidth(390);
    preloadRouteModuleMock.mockReturnValueOnce(new Promise<void>(() => undefined));
    renderLayout([], '/alerts');

    const mobileNav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    fireEvent.click(within(mobileNav).getByRole('link', { name: 'Actions' }));

    await waitFor(() => expect(window.location.pathname).toBe('/actions'));
  });

  it('shows platform and runtime lens tabs with supported infrastructure evidence', () => {
    renderLayout([
      makeResource({ id: 'agent-1', type: 'agent', platformType: 'agent' }),
      makeResource({ id: 'pve-1', type: 'agent', platformType: 'proxmox-pve' }),
      makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' }),
      makeResource({ id: 'vcenter-1', type: 'vm', platformType: 'vmware-vsphere' }),
    ]);

    const desktopNav = screen.getByRole('navigation', { name: 'Primary navigation' });
    const infrastructureGroup = desktopNav.querySelector('[aria-label="Infrastructure"]');
    expect(infrastructureGroup).toBeTruthy();

    expect(
      within(infrastructureGroup as HTMLElement)
        .getAllByRole('link')
        .map((tab) => tab.getAttribute('aria-label')),
    ).toEqual(['Proxmox', 'Docker', 'vSphere', 'Machines']);
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('link', { name: 'Machines' }),
    ).toBeTruthy();
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('link', { name: 'Proxmox' }),
    ).toHaveAttribute('href', '/proxmox/overview');
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('link', { name: 'Docker' }),
    ).toBeTruthy();
    expect(
      within(infrastructureGroup as HTMLElement).queryByRole('link', { name: 'Kubernetes' }),
    ).toBeNull();
    expect(
      within(infrastructureGroup as HTMLElement).queryByRole('link', { name: 'TrueNAS' }),
    ).toBeNull();
    expect(
      within(infrastructureGroup as HTMLElement).getByRole('link', { name: 'vSphere' }),
    ).toBeTruthy();

    const mobileNav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    fireEvent.click(
      within(mobileNav).getByRole('button', { name: 'Switch platform, current Proxmox' }),
    );
    const platformMenu = screen.getByRole('menu', { name: 'Switch platform' });
    expect(
      within(platformMenu)
        .getAllByRole('menuitem')
        .map((item) => item.getAttribute('data-tab-id')),
    ).toEqual(['proxmox', 'docker', 'vmware', 'standalone']);
  });

  it('does not expose cached platform evidence before navigation admission resolves', () => {
    renderLayout(
      [makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' })],
      '/settings/infrastructure',
      {
        proxmox: false,
        docker: false,
        kubernetes: false,
        truenas: false,
        vmware: false,
        standalone: false,
      },
    );

    const desktopNav = screen.getByRole('navigation', { name: 'Primary navigation' });
    const infrastructureGroup = desktopNav.querySelector('[aria-label="Infrastructure"]');
    expect(infrastructureGroup).toBeTruthy();
    expect(within(infrastructureGroup as HTMLElement).queryByRole('link')).toBeNull();
  });

  it('restores the previous Proxmox route state when returning from another platform tab', async () => {
    renderLayout(platformResources(), '/proxmox/overview?status=running');

    const mobileNav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    fireEvent.click(
      within(mobileNav).getByRole('button', { name: 'Switch platform, current Proxmox' }),
    );
    const platformMenu = await screen.findByRole('menu', { name: 'Switch platform' });
    const rememberedMobileLink = within(platformMenu).getByRole('menuitem', { name: 'Proxmox' });
    expect(rememberedMobileLink.tagName).toBe('A');
    expect(rememberedMobileLink).toHaveAttribute('href', '/proxmox/overview?status=running');
    fireEvent.keyDown(rememberedMobileLink, { key: 'Escape' });

    await fireEvent.click(getInfrastructureLink('Docker'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/docker/overview');
      expect(window.location.search).toBe('');
    });

    await fireEvent.click(getInfrastructureLink('Proxmox'));
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

    await fireEvent.click(getInfrastructureLink('Docker'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/docker/overview');
      expect(window.location.search).toBe('');
    });

    await fireEvent.click(getInfrastructureLink('Proxmox'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/proxmox/overview');
      expect(window.location.search).toBe('?status=running');
    });
  });

  it('keeps remembered route state scoped to the platform tab that owns it', async () => {
    renderLayout(platformResources(), '/docker/overview?host=docker-1');

    await fireEvent.click(getInfrastructureLink('Proxmox'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/proxmox/overview');
      expect(window.location.search).toBe('');
    });

    await fireEvent.click(getInfrastructureLink('Docker'));
    await waitFor(() => {
      expect(window.location.pathname).toBe('/docker/overview');
      expect(window.location.search).toBe('?host=docker-1');
    });
  });

  it('uses the canonical platform root route when there is no remembered route state', async () => {
    renderLayout(platformResources(), '/settings/infrastructure');

    await fireEvent.click(getInfrastructureLink('Proxmox'));
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

  it('renders entitled custom branding and uses its name in the browser title', () => {
    updateRuntimeBrandingFromResponse({
      enabled: true,
      displayName: 'Acme Operations',
      logoDataUrl: 'data:image/png;base64,YWJj',
    });

    renderLayout();

    expect(screen.getByTestId('custom-brand-logo')).toHaveAttribute(
      'src',
      'data:image/png;base64,YWJj',
    );
    expect(screen.getByText('Acme Operations')).toBeInTheDocument();
    expect(screen.getByTestId('pulse-brand-lockup')).not.toHaveClass('animate-pulse-brand');
    expect(document.title).toBe('Settings · Acme Operations');
  });

  it('keeps the mobile Assistant launcher in header flow instead of covering page content', () => {
    setViewportWidth(390);
    renderLayout();

    const launcher = screen.getByRole('button', { name: 'Ask Pulse Assistant about Settings' });
    const launcherClass = launcher.getAttribute('class') ?? '';

    expect(launcherClass).toContain('h-11');
    expect(launcherClass).toContain('w-11');
    expect(launcherClass).toContain('rounded-full');
    expect(launcherClass).not.toContain('fixed');
    expect(launcher.closest('.header-controls')).toBeInTheDocument();
  });

  it('preserves the Assistant edge launcher on desktop', () => {
    setViewportWidth(1440);
    renderLayout();

    const launcher = screen.getByRole('button', { name: 'Ask Pulse Assistant about Settings' });
    expect(launcher).toHaveClass('fixed');
    expect(launcher).toHaveClass('right-0');
    expect(launcher).toHaveClass('top-1/2');
    expect(launcher).toHaveClass('-translate-y-1/2');
    expect(launcher.closest('.header-controls')).toBeNull();
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
    openAssistant.mockRestore();
  });

  it('returns focus to the Assistant launcher after chat closes', async () => {
    renderLayout();

    const launcher = screen.getByRole('button', {
      name: 'Ask Pulse Assistant about Settings',
    });
    launcher.focus();
    await fireEvent.click(launcher);

    await waitFor(() => {
      expect(
        screen.queryByRole('button', { name: 'Ask Pulse Assistant about Settings' }),
      ).not.toBeInTheDocument();
    });

    aiChatStore.close();

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: 'Ask Pulse Assistant about Settings' }),
      ).toHaveFocus();
    });
  });
});

describe('AppLayout Issue1650 kiosk scope containment', () => {
  const KIOSK_TOKEN_SCOPES = ['monitoring:read'];
  const DOCKER_ONLY_VISIBILITY: PlatformNavigationVisibility = {
    proxmox: false,
    docker: true,
    kubernetes: false,
    truenas: false,
    vmware: false,
    standalone: false,
  };

  const header = (container: HTMLElement) => container.querySelector('.header') as HTMLElement;
  const headerIsRevealed = (container: HTMLElement) =>
    header(container).style.transform === 'translateY(0)';

  beforeEach(() => {
    window.history.replaceState({}, '', '/settings/infrastructure');
    resetPrimaryNavigationRouteMemory();
    patrolAttentionMockState.activeCount = 0;
    aiChatStore.close();
    aiChatStore.setEnabled(false);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    setKioskMode(false);
    window.sessionStorage.clear();
  });

  it('treats a monitoring-only token as having no settings access', () => {
    expect(sessionHasSettingsAccess(KIOSK_TOKEN_SCOPES)).toBe(false);
    expect(sessionHasSettingsAccess(['monitoring:read', 'settings:read'])).toBe(true);
    expect(sessionHasSettingsAccess(['*'])).toBe(true);
    expect(sessionHasSettingsAccess([])).toBe(true);
    expect(sessionHasSettingsAccess(undefined)).toBe(true);
  });

  it('keeps kiosk on and peeks the header when a kiosk token presses Escape', () => {
    vi.useFakeTimers();
    setKioskMode(true);
    const { container } = renderLayout(
      [],
      '/docker/overview',
      DOCKER_ONLY_VISIBILITY,
      KIOSK_TOKEN_SCOPES,
    );

    // Kiosk shows the header briefly on entry, then hides it.
    vi.advanceTimersByTime(2000);
    expect(headerIsRevealed(container)).toBe(false);

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));

    expect(isKioskMode()).toBe(true);
    expect(screen.queryByRole('navigation', { name: 'Primary navigation' })).toBeNull();
    expect(headerIsRevealed(container)).toBe(true);

    // The peek is temporary, exactly like the hover and touch affordances.
    vi.advanceTimersByTime(3500);
    expect(headerIsRevealed(container)).toBe(false);
    expect(isKioskMode()).toBe(true);
  });

  it('still exits kiosk on Escape for a session that can reach settings', () => {
    setKioskMode(true);
    renderLayout([], '/docker/overview', DOCKER_ONLY_VISIBILITY, ['settings:read']);

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));

    expect(isKioskMode()).toBe(false);
    expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeTruthy();
  });

  it('redirects a scope-limited session away from settings even when kiosk is off', async () => {
    setKioskMode(false);
    renderLayout([], '/settings/infrastructure', DOCKER_ONLY_VISIBILITY, KIOSK_TOKEN_SCOPES);

    expect(isKioskMode()).toBe(false);
    await waitFor(() => {
      expect(window.location.pathname).toBe('/docker/overview');
    });
    const systemGroup = screen
      .getByRole('navigation', { name: 'Primary navigation' })
      .querySelector('[aria-label="System"]');
    expect(within(systemGroup as HTMLElement).queryByRole('link', { name: 'Settings' })).toBeNull();
  });

  it('leaves settings reachable with kiosk off for a session that has settings access', async () => {
    setKioskMode(false);
    renderLayout([], '/settings/infrastructure', DOCKER_ONLY_VISIBILITY, ['settings:read']);

    await waitFor(() => {
      expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeTruthy();
    });
    expect(window.location.pathname).toBe('/settings/infrastructure');
  });
});
