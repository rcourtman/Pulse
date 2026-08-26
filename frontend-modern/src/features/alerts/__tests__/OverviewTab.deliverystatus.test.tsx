import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import type { Alert, AlertDeliveryDiagnosis } from '@/types/api';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

const getDeliveryDiagnoses = vi.fn<() => Promise<AlertDeliveryDiagnosis[]>>();

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    get getDeliveryDiagnoses() {
      return getDeliveryDiagnoses;
    },
  },
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

function makeAlert(id: string, ack = false): Alert {
  return {
    id,
    resourceId: `vm-${id}`,
    resourceName: `VM ${id}`,
    type: 'cpu',
    level: 'warning',
    message: `High CPU on VM ${id}`,
    startTime: new Date().toISOString(),
    acknowledged: ack,
    node: 'node1',
  } as Alert;
}

function makeDiagnosis(
  id: string,
  overrides: Partial<AlertDeliveryDiagnosis>,
): AlertDeliveryDiagnosis {
  return {
    alertIdentifier: id,
    alertId: id,
    trackingKey: `vm-${id}/cpu`,
    status: 'would_send',
    reason: 'ready',
    message: 'Alert delivery is currently eligible for notification delivery.',
    alertType: 'cpu',
    level: 'warning',
    notificationsEnabled: true,
    activationState: 'active',
    cooldownMinutes: 5,
    maxAlertsHour: 10,
    recentAlertsInHour: 0,
    flappingActive: false,
    flappingHistoryInWindow: 0,
    flappingThreshold: 5,
    flappingWindowSeconds: 300,
    ...overrides,
  };
}

function defaultProps(overrides: Record<string, unknown> = {}) {
  return {
    overrides: [] as never[],
    activeAlerts: {} as Record<string, Alert>,
    updateAlert: vi.fn(),
    showQuickTip: () => false,
    dismissQuickTip: vi.fn(),
    showAcknowledged: () => true,
    setShowAcknowledged: vi.fn(),
    alertsDisabled: () => false,
    ...overrides,
  };
}

describe('OverviewTab delivery status line', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
    getDeliveryDiagnoses.mockReset();
  });

  afterEach(() => {
    cleanup();
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('renders held-notification status from the bulk diagnosis endpoint', async () => {
    const activeAlerts: Record<string, Alert> = { a1: makeAlert('a1') };
    getDeliveryDiagnoses.mockResolvedValue([
      makeDiagnosis('a1', { status: 'suppressed', reason: 'notifications_disabled' }),
    ]);

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    await waitFor(() => {
      expect(screen.getByText('Notifications are turned off')).toBeTruthy();
    });
    expect(getDeliveryDiagnoses).toHaveBeenCalled();
  });

  it('renders no delivery line when the diagnosis fetch fails', async () => {
    const activeAlerts: Record<string, Alert> = { a1: makeAlert('a1') };
    getDeliveryDiagnoses.mockRejectedValue(new Error('boom'));

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    await waitFor(() => {
      expect(getDeliveryDiagnoses).toHaveBeenCalled();
    });
    expect(screen.queryByText('Notifications are turned off')).toBeNull();
    expect(screen.getByText('High CPU on VM a1')).toBeTruthy();
  });
});
