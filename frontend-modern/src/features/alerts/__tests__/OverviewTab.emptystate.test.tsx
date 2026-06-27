import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {},
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

function defaultProps(overrides: Record<string, unknown> = {}) {
  return {
    overrides: [] as never[],
    activeAlerts: {} as Record<string, never>,
    updateAlert: vi.fn(),
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
