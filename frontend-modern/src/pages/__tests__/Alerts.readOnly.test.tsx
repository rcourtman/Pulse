import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { Alerts } from '@/pages/Alerts';
import alertsPageSource from '@/pages/Alerts.tsx?raw';

const navigateSpy = vi.hoisted(() => vi.fn());
const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));
const activationStateMock = vi.hoisted(() => vi.fn(() => 'active'));
const locationState = vi.hoisted(() => ({
  pathname: '/alerts',
  hash: '',
  search: '',
  query: {},
}));
const overviewTabSpy = vi.hoisted(() =>
  vi.fn((props: { alertsDisabled: () => boolean }) => (
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
    updateAlert: vi.fn(),
    removeAlerts: vi.fn(),
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

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: { enabled: false },
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    activationState: () => activationStateMock(),
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

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: vi.fn(),
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

describe('Alerts read-only presentation', () => {
  beforeEach(() => {
    cleanup();
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
  });

  afterEach(() => cleanup());

  it('keeps the mobile tab shell on shared scroll classes instead of inline styles', () => {
    expect(alertsPageSource).toContain('touch-scroll');
    expect(alertsPageSource).toContain('scrollbar-hide');
    expect(alertsPageSource).not.toContain('style="-webkit-overflow-scrolling: touch;"');
  });

  it('hides alerts management affordances in read-only sessions', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    activationStateMock.mockReturnValue('pending_review');

    render(() => <Alerts />);

    await waitFor(() => {
      expect(screen.getByTestId('overview-tab')).toBeInTheDocument();
    });

    expect(screen.queryByLabelText('Toggle alerts')).not.toBeInTheDocument();
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

    render(() => <Alerts />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/alerts/overview', { replace: true });
    });
    expect(screen.queryByTestId('alerts-config-surface')).not.toBeInTheDocument();
  });

  it('keeps configuration surfaces available in writable sessions', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    activationStateMock.mockReturnValue('active');
    locationState.pathname = '/alerts/thresholds';

    render(() => <Alerts />);

    await waitFor(() => {
      expect(screen.getByTestId('alerts-config-surface')).toBeInTheDocument();
    });
    expect(screen.queryByLabelText('Toggle alerts')).not.toBeInTheDocument();
    expect(screen.getAllByText('Thresholds').length).toBeGreaterThan(0);
    expect(navigateSpy).not.toHaveBeenCalledWith('/alerts/overview', { replace: true });
  });
});
