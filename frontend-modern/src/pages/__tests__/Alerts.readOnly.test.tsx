import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { Alerts } from '@/pages/Alerts';
import { createAlertAcknowledgementRequests } from '@/stores/alertAcknowledgementRequests';
import { createSignal, onCleanup } from 'solid-js';
import type { SecurityStatus } from '@/types/config';
import { eventBus } from '@/stores/events';
import { setOrgID } from '@/utils/apiClient';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import alertHistoryFiltersCardSource from '@/features/alerts/AlertHistoryFiltersCard.tsx?raw';

const navigateSpy = vi.hoisted(() => vi.fn());
const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));
const activationStateMock = vi.hoisted(() => vi.fn(() => 'active'));
const locationState = vi.hoisted(() => ({
  pathname: '/alerts',
  hash: '',
  search: '',
  query: {},
}));
// The store's request owner, one per test, as the app keeps one per store.
const storeFixture = vi.hoisted(() => ({ alertAcknowledgementRequests: undefined as unknown }));
const overviewTabSpy = vi.hoisted(() =>
  vi.fn((props: { alertsDisabled: () => boolean; acknowledgementRequests?: unknown }) => (
    <div data-testid="overview-tab">{props.alertsDisabled() ? 'disabled' : 'enabled'}</div>
  )),
);
const historyTabSpy = vi.hoisted(() => vi.fn(() => <div data-testid="history-tab">History</div>));
const configurationSurfaceSpy = vi.hoisted(() =>
  vi.fn(() => <div data-testid="alerts-config-surface">Config</div>),
);

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => locationState,
    useNavigate: () => navigateSpy,
    useBeforeLeave: () => undefined,
  };
});

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    activeAlerts: {},
    activeAlertsHydrationStatus: () => 'ready',
    refreshActiveAlerts: vi.fn(async () => undefined),
    updateAlert: vi.fn(),
    removeAlerts: vi.fn(),
    alertAcknowledgementRequests: storeFixture.alertAcknowledgementRequests,
  }),
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    get: vi.fn(),
    resources: () => [],
    byType: () => [],
    children: () => [],
  }),
}));

vi.mock('@/stores/license', () => ({
  hasFeature: () => true,
  runtimeCapabilitiesLoaded: () => true,
  runtimeCapabilitiesLoading: () => false,
  loadRuntimeCapabilities: vi.fn(async () => undefined),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/stores/aiAvailability', () => ({
  aiAvailabilityStore: {
    enabled: false,
  },
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    activationState: () => activationStateMock(),
    detectionEnabled: () => true,
    notificationDeliveryEnabled: () => activationStateMock() === 'active',
    isLoading: () => false,
    activate: vi.fn(async () => true),
    deactivate: vi.fn(async () => true),
    refreshActiveAlerts: vi.fn(async () => undefined),
    refreshConfig: vi.fn(async () => undefined),
    config: () => null,
  }),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => presentationPolicyIsReadOnlyMock(),
}));

vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn() },
}));

vi.mock('@/features/alerts/OverviewTab', () => ({
  OverviewTab: overviewTabSpy,
}));

vi.mock('@/features/alerts/tabs/HistoryTab', () => ({
  HistoryTab: historyTabSpy,
}));

vi.mock('@/features/alerts/AlertsConfigurationSurface', () => ({
  AlertsConfigurationSurface: configurationSurfaceSpy,
}));

const admittedStatus = {
  detailLevel: 'privileged',
  hasAuthentication: true,
  requiresAuth: true,
  currentUsername: 'alice',
  apiTokenHint: 'hint-a',
  tokenScopes: ['settings:read', 'settings:write'],
} as SecurityStatus;

describe('Alerts read-only presentation', () => {
  beforeEach(() => {
    cleanup();
    setOrgID('default');
    navigateSpy.mockReset();
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    activationStateMock.mockReset();
    activationStateMock.mockReturnValue('active');
    overviewTabSpy.mockClear();
    historyTabSpy.mockClear();
    configurationSurfaceSpy.mockClear();
    locationState.pathname = '/alerts';
    locationState.hash = '';
    locationState.search = '';
    storeFixture.alertAcknowledgementRequests = createAlertAcknowledgementRequests();
  });

  afterEach(() => cleanup());

  it('withholds configuration until the current security status is known', () => {
    locationState.pathname = '/alerts/destinations';
    render(() => <Alerts securityStatus={null} />);
    expect(screen.queryByTestId('alerts-config-surface')).not.toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('Alert settings access is unavailable');
    expect(configurationSurfaceSpy).not.toHaveBeenCalled();
  });

  for (const detailLevel of ['authenticated', 'privileged'] as const)
    it(`admits a ${detailLevel} reader on an install that requires login`, () => {
      render(() => <Alerts securityStatus={{ ...admittedStatus, detailLevel }} />);
      expect(screen.getByTestId('alerts-config-surface')).toBeInTheDocument();
    });

  it('preserves configuration access on an auth-free install', () => {
    render(() => (
      <Alerts
        securityStatus={{ detailLevel: 'public', requiresAuth: false, hasAuthentication: false }}
      />
    ));
    expect(screen.getByTestId('alerts-config-surface')).toBeInTheDocument();
  });

  for (const detailLevel of ['public', undefined] as const)
    it(`withholds configuration from ${detailLevel ?? 'unresolved'} login discovery`, () => {
      locationState.pathname = '/alerts/thresholds';
      render(() => (
        <Alerts securityStatus={{ requiresAuth: true, hasAuthentication: true, detailLevel }} />
      ));
      expect(screen.queryByTestId('alerts-config-surface')).not.toBeInTheDocument();
      expect(screen.getByRole('status')).toHaveTextContent('Alert settings access is unavailable');
    });

  it('retires a signed-in editor when status becomes public and admits only a fresh authenticated editor', async () => {
    const retired = vi.fn();
    configurationSurfaceSpy.mockImplementation(() => {
      onCleanup(retired);
      return <div data-testid="alerts-config-surface">Config</div>;
    });
    const [status, setStatus] = createSignal<SecurityStatus>(admittedStatus);
    render(() => <Alerts securityStatus={status()} />);
    expect(screen.getByTestId('alerts-config-surface')).toBeInTheDocument();
    setStatus({ requiresAuth: true, hasAuthentication: true, detailLevel: 'public' });
    await waitFor(() => expect(retired).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId('alerts-config-surface')).not.toBeInTheDocument();
    setStatus({ ...admittedStatus, detailLevel: 'authenticated' });
    await screen.findByTestId('alerts-config-surface');
    expect(configurationSurfaceSpy).toHaveBeenCalledTimes(2);
  });

  it('keeps equivalent reader facts mounted, but retires user, token, scope and workspace lifetimes', async () => {
    const retired = vi.fn();
    configurationSurfaceSpy.mockImplementation(() => {
      onCleanup(retired);
      return <div data-testid="alerts-config-surface">Config</div>;
    });
    const [status, setStatus] = createSignal<SecurityStatus | null>(admittedStatus);
    render(() => <Alerts securityStatus={status()} />);
    await screen.findByTestId('alerts-config-surface');
    expect(configurationSurfaceSpy).toHaveBeenCalledTimes(1);
    setStatus({ ...admittedStatus, tokenScopes: admittedStatus.tokenScopes!.slice().reverse() });
    await Promise.resolve();
    expect(retired).not.toHaveBeenCalled();
    expect(configurationSurfaceSpy).toHaveBeenCalledTimes(1);
    for (const delta of [
      { currentUsername: 'bob' },
      { apiTokenHint: 'hint-b' },
      { tokenScopes: ['settings:read'] },
    ]) {
      const previous = retired.mock.calls.length;
      setStatus({ ...status()!, ...delta });
      await waitFor(() => expect(retired).toHaveBeenCalledTimes(previous + 1));
    }
    const previous = retired.mock.calls.length;
    setOrgID('tenant-b');
    eventBus.emit('org_switched', 'tenant-b');
    await waitFor(() => expect(retired).toHaveBeenCalledTimes(previous + 1));
    setStatus(null);
    await waitFor(() =>
      expect(screen.queryByTestId('alerts-config-surface')).not.toBeInTheDocument(),
    );
  });

  it('hands the store acknowledgement requests to the overview', async () => {
    render(() => <Alerts securityStatus={admittedStatus} />);

    expect(await screen.findByTestId('overview-tab')).toBeInTheDocument();
    expect(overviewTabSpy.mock.calls.at(-1)?.[0].acknowledgementRequests).toBe(
      storeFixture.alertAcknowledgementRequests,
    );
  });

  it('keeps alert history narrowing URL-owned with no saved-views affordance', () => {
    // Saved views stored the page's URL query string in localStorage. Bookmarks
    // already do that, and better, so the control was removed; history filters
    // stay URL-owned and the filtered page remains a shareable link.
    expect(alertHistoryFiltersCardSource).not.toContain('savedViewsKey');
    expect(alertHistoryFiltersCardSource).not.toContain('SavedViews');
  });

  it('keeps the mobile tab shell on shared scroll classes instead of inline styles', () => {
    expect(alertsPageSource).toContain('touch-scroll');
    expect(alertsPageSource).toContain('scrollbar-hide');
    expect(alertsPageSource).not.toContain('style="-webkit-overflow-scrolling: touch;"');
  });

  it('keeps the active mobile destination visible when the tab rail narrows', async () => {
    locationState.pathname = '/alerts/schedule';

    render(() => <Alerts securityStatus={admittedStatus} />);

    const navigation = document.querySelector<HTMLElement>('nav.touch-scroll');
    expect(navigation).not.toBeNull();
    const scheduleTab = Array.from(navigation?.querySelectorAll('button') ?? []).find(
      (button) => button.textContent?.trim() === 'Schedule',
    );
    expect(scheduleTab).toHaveAttribute('aria-current', 'page');
    if (!navigation || !scheduleTab) throw new Error('Expected the active Schedule mobile tab');

    Object.defineProperties(navigation, {
      clientWidth: { configurable: true, value: 336 },
      scrollWidth: { configurable: true, value: 391 },
      scrollLeft: { configurable: true, writable: true, value: 0 },
    });
    Object.defineProperties(scheduleTab, {
      offsetLeft: { configurable: true, value: 315 },
      offsetWidth: { configurable: true, value: 74 },
    });

    window.dispatchEvent(new Event('resize'));

    await waitFor(() => expect(navigation?.scrollLeft).toBe(55));
  });

  it('hides alerts management affordances in read-only sessions', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    activationStateMock.mockReturnValue('pending_review');

    render(() => <Alerts securityStatus={admittedStatus} />);

    await waitFor(() => {
      expect(screen.getByTestId('overview-tab')).toBeInTheDocument();
    });

    expect(screen.queryByLabelText('Send notifications')).not.toBeInTheDocument();
    expect(screen.getAllByText('Overview').length).toBeGreaterThan(0);
    expect(screen.getAllByText('History').length).toBeGreaterThan(0);
    expect(screen.queryByText('Thresholds')).not.toBeInTheDocument();
    expect(screen.queryByText('Notifications')).not.toBeInTheDocument();
    expect(screen.queryByText('Schedule')).not.toBeInTheDocument();
    expect(screen.queryByTestId('alerts-config-surface')).not.toBeInTheDocument();
    expect(overviewTabSpy).toHaveBeenCalled();
    expect(overviewTabSpy.mock.calls.at(-1)?.[0].alertsDisabled()).toBe(false);
  });

  it('redirects read-only configuration routes back to overview', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    activationStateMock.mockReturnValue('pending_review');
    locationState.pathname = '/alerts/thresholds';

    render(() => <Alerts securityStatus={admittedStatus} />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/alerts/overview', { replace: true });
    });
    expect(screen.queryByTestId('alerts-config-surface')).not.toBeInTheDocument();
  });

  it('keeps configuration surfaces available in writable sessions', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    activationStateMock.mockReturnValue('active');
    locationState.pathname = '/alerts/thresholds';

    render(() => <Alerts securityStatus={admittedStatus} />);

    await waitFor(() => {
      expect(screen.getByTestId('alerts-config-surface')).toBeInTheDocument();
    });
    expect(screen.queryByLabelText('Send notifications')).not.toBeInTheDocument();
    expect(screen.getAllByText('Thresholds').length).toBeGreaterThan(0);
    expect(navigateSpy).not.toHaveBeenCalledWith('/alerts/overview', { replace: true });
  });

  it('keeps alert truth and configuration visible while external notifications are paused', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    activationStateMock.mockReturnValue('pending_review');

    render(() => <Alerts securityStatus={admittedStatus} />);

    expect(await screen.findByTestId('overview-tab')).toBeInTheDocument();
    expect(screen.getByTestId('alerts-config-surface')).toBeInTheDocument();
    // The switch keeps one name; its position says delivery is off. A label
    // reading "Notifications paused" beside an off switch read as "not paused".
    expect(screen.getByRole('checkbox', { name: 'Send notifications' })).not.toBeChecked();
    expect(screen.queryByText('Notifications paused')).not.toBeInTheDocument();
    expect(screen.getAllByText('Thresholds').length).toBeGreaterThan(0);
    expect(screen.queryByText('Alerts are paused')).not.toBeInTheDocument();
    expect(overviewTabSpy.mock.calls.at(-1)?.[0].alertsDisabled()).toBe(false);
  });

  it('names the delivery switch the same way whether it is on or off', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    activationStateMock.mockReturnValue('active');

    render(() => <Alerts securityStatus={admittedStatus} />);

    expect(await screen.findByTestId('overview-tab')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Send notifications' })).toBeChecked();
    expect(screen.queryByText('Notifications enabled')).not.toBeInTheDocument();
  });
});
