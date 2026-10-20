import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import type { Alert } from '@/types/api';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

const getConfigMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/alerts', () => ({
  AlertsAPI: { getConfig: getConfigMock, getDeliveryDiagnoses: vi.fn().mockResolvedValue([]) },
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
import { useAlertsActivation } from '@/stores/alertsActivation';

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

function groupedAlert(id: string, overrides: Partial<Alert> = {}): Alert {
  return {
    id,
    type: 'connection',
    level: 'warning',
    resourceId: `resource-${id}`,
    resourceName: `Resource ${id}`,
    instance: 'lab',
    node: '',
    message: `Current condition for ${id}`,
    value: 0,
    threshold: 0,
    startTime: '2026-10-20T12:00:00Z',
    acknowledged: false,
    ...overrides,
  };
}

describe('OverviewTab grouped alert disclosure', () => {
  afterEach(() => {
    cleanup();
    setActiveLocale(DEFAULT_LOCALE);
  });

  it.each(['shared-system', 'infrastructure-incident'] as const)(
    'exposes hidden severities without replacing the %s primary',
    (kind) => {
      const primary = groupedAlert('primary', {
        correlation: { key: 'incident:one', kind, role: 'primary', reason: 'Synthetic source' },
      });
      const related = (id: string, level: Alert['level'], acknowledged = false) =>
        groupedAlert(id, {
          level,
          acknowledged,
          correlation: {
            key: 'incident:one',
            kind,
            role: 'supporting',
            reason: 'Synthetic source',
          },
        });
      const critical = related('critical', 'critical');
      const warning = related('warning', 'warning');
      const info = related('info', 'info');
      const acknowledged = related('acknowledged', 'critical', true);
      render(() => (
        <OverviewTab
          {...defaultProps({
            activeAlerts: { primary, critical, warning, info, acknowledged },
            showAcknowledged: () => true,
          })}
        />
      ));

      expect(screen.getByText(primary.message)).toBeVisible();
      expect(screen.queryByText(critical.message)).not.toBeInTheDocument();
      const disclosure = screen.getByRole('button', {
        name: '+4 linked signals 1 Critical 1 Warning 1 Info',
      });
      expect(disclosure).toHaveAttribute('aria-expanded', 'false');
      expect(disclosure).not.toHaveAttribute('aria-controls');
      expect(disclosure).toHaveClass('min-h-11', 'sm:min-h-6');
      fireEvent.click(disclosure);
      expect(disclosure).toHaveAttribute('aria-expanded', 'true');
      const controlled = document.getElementById(disclosure.getAttribute('aria-controls')!);
      expect(controlled).not.toBeNull();
      expect(within(controlled!).getByText(critical.message)).toBeVisible();
      expect(within(controlled!).getByText(acknowledged.message)).toBeVisible();
      fireEvent.click(disclosure);
      expect(disclosure).toHaveAttribute('aria-expanded', 'false');
      expect(screen.queryByText(critical.message)).not.toBeInTheDocument();
    },
  );

  it('updates the summary on ordinary observation and acknowledgement changes', () => {
    const primary = groupedAlert('primary', { resourceId: 'same-resource' });
    const critical = groupedAlert('critical', {
      resourceId: 'same-resource',
      level: 'critical',
      acknowledged: true,
    });
    const warning = groupedAlert('warning', { resourceId: 'same-resource' });
    const [alerts, setAlerts] = createSignal({ primary, critical, warning });
    render(() => (
      <OverviewTab {...defaultProps({ showAcknowledged: () => true })} activeAlerts={alerts()} />
    ));
    expect(screen.getByRole('button', { name: '+2 related 1 Warning' })).toBeVisible();
    setAlerts({ primary, critical: { ...critical, level: 'warning' }, warning });
    expect(screen.getByRole('button', { name: '+2 related 1 Warning' })).toBeVisible();
    setAlerts({ primary, critical: { ...critical, acknowledged: false }, warning });
    // With no designated correlation primary, the critical card itself leads.
    expect(screen.getByText(critical.message)).toBeVisible();
    expect(screen.getByRole('button', { name: '+2 related 2 Warning' })).toBeVisible();
    setAlerts({ primary, critical: { ...critical, acknowledged: true }, warning });
    expect(screen.queryByText(critical.message)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '+2 related 1 Warning' })).toBeVisible();
  });

  it('keeps independent group disclosures uniquely bound', () => {
    const alerts = Object.fromEntries(
      ['one', 'two'].flatMap((group) =>
        ['primary', 'related'].map((role) => {
          const alert = groupedAlert(`${group}-${role}`, { resourceId: group });
          return [alert.id, alert];
        }),
      ),
    );
    render(() => <OverviewTab {...defaultProps({ activeAlerts: alerts })} />);
    const disclosures = screen.getAllByRole('button', { name: '+1 related 1 Warning' });
    disclosures.forEach((button) => fireEvent.click(button));
    const ids = disclosures.map((button) => button.getAttribute('aria-controls'));
    expect(new Set(ids).size).toBe(2);
    ids.forEach((id) => expect(document.getElementById(id!)).toBeInTheDocument());
    fireEvent.click(disclosures[0]);
    expect(document.getElementById(ids[0]!)).not.toBeInTheDocument();
    expect(document.getElementById(ids[1]!)).toBeInTheDocument();
  });
});

// Runs last: it loads the shared activation store, which the empty-state tests
// above rely on staying unloaded.
describe('OverviewTab delivery paused banner', () => {
  afterEach(() => {
    cleanup();
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
});
