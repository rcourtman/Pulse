import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

const getConfigMock = vi.hoisted(() => vi.fn());
const activateMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/alerts', () => ({
  AlertsAPI: { getConfig: getConfigMock, activate: activateMock },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn() },
}));

vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

import { OverviewTab } from '../OverviewTab';
import { createAlertAcknowledgementRequests } from '@/stores/alertAcknowledgementRequests';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { eventBus } from '@/stores/events';
import { setOrgID } from '@/utils/apiClient';

function defaultProps(overrides: Record<string, unknown> = {}) {
  return {
    overrides: [] as never[],
    activeAlerts: {} as Record<string, never>,
    updateAlert: vi.fn(),
    acknowledgementRequests: createAlertAcknowledgementRequests(),
    showQuickTip: () => false,
    dismissQuickTip: vi.fn(),
    showAcknowledged: () => false,
    setShowAcknowledged: vi.fn(),
    alertsDisabled: () => false,
    ...overrides,
  };
}

describe('OverviewTab empty state', () => {
  afterEach(() => {
    cleanup();
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('shows "No active alerts" when alerts are enabled and none exist', () => {
    setActiveLocale(DEFAULT_LOCALE);
    render(() => <OverviewTab {...defaultProps()} />);

    expect(screen.getByText('No active alerts')).toBeInTheDocument();
    expect(
      screen.getByText('Alerts will appear here when thresholds are exceeded'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Alerting is paused')).not.toBeInTheDocument();
  });

  it('shows "Alerting is paused" when alerts are disabled', () => {
    setActiveLocale(DEFAULT_LOCALE);
    render(() => <OverviewTab {...defaultProps({ alertsDisabled: () => true })} />);

    expect(screen.getByText('Alerting is paused')).toBeInTheDocument();
    expect(
      screen.getByText('Toggle alerts on to resume monitoring and unlock configuration tabs'),
    ).toBeInTheDocument();
    expect(screen.queryByText('No active alerts')).not.toBeInTheDocument();
  });

  it('localizes the enabled and paused empty states through the active locale', () => {
    setActiveLocale('es');
    const { unmount } = render(() => <OverviewTab {...defaultProps()} />);

    expect(screen.getByText('No hay alertas activas')).toBeInTheDocument();
    expect(
      screen.getByText('Las alertas apareceran aqui cuando se superen los umbrales'),
    ).toBeInTheDocument();
    expect(screen.queryByText('No active alerts')).not.toBeInTheDocument();

    unmount();

    render(() => <OverviewTab {...defaultProps({ alertsDisabled: () => true })} />);

    expect(screen.getByText('Las alertas estan pausadas')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Activa las alertas para reanudar la monitorizacion y desbloquear las pestañas de configuracion',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Alerting is paused')).not.toBeInTheDocument();
  });
});

// Runs last: it loads the shared activation store, which the empty-state tests
// above rely on staying unloaded.
describe('OverviewTab delivery paused banner', () => {
  afterEach(() => {
    cleanup();
    setOrgID(null);
    eventBus.emit('org_switched');
  });

  it.each([
    ['a fresh install', { enabled: true, activationState: 'pending_review' }],
    [
      'an install whose delivery was switched off',
      { enabled: true, activationState: 'pending_review', activationTime: '2026-10-01T09:00:00Z' },
    ],
  ])('says only that delivery is off for %s', async (_case, config) => {
    getConfigMock.mockResolvedValue(config);
    await useAlertsActivation().refreshConfig();

    render(() => <OverviewTab {...defaultProps()} />);

    expect(screen.getByText(/Notification delivery is turned off\./)).toBeInTheDocument();
    expect(screen.queryByText(/for this install yet/)).not.toBeInTheDocument();
  });

  it('keeps the new workspace resume button usable while the old request is outstanding', async () => {
    setOrgID('west');
    getConfigMock.mockClear();
    getConfigMock.mockResolvedValue({ enabled: true, activationState: 'pending_review' });
    let finishOutgoing!: (result: { success: boolean }) => void;
    activateMock.mockReturnValueOnce(
      new Promise<{ success: boolean }>((resolve) => {
        finishOutgoing = resolve;
      }),
    );
    const store = useAlertsActivation();
    await store.refreshConfig();
    render(() => <OverviewTab {...defaultProps()} />);

    const resume = () => screen.getByRole('button', { name: 'Turn on delivery' });
    fireEvent.click(resume());
    expect(resume()).toBeDisabled();
    setOrgID('east');
    eventBus.emit('org_switched', 'east');
    await store.refreshConfig();
    expect(resume()).toBeEnabled();

    finishOutgoing({ success: true });
    await Promise.resolve();
    expect(getConfigMock).toHaveBeenCalledTimes(2);
    expect(resume()).toBeEnabled();
  });
});
