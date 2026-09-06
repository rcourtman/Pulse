import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
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

  it.each([
    { status: 'would_send', reason: 'ready' },
    { status: 'suppressed', reason: 'cooldown', nextEligibleAt: '2026-08-26T10:20:00Z' },
  ] as const)('does not turn dispatch evidence into receipt for $reason', async (state) => {
    getDeliveryDiagnoses.mockResolvedValue([
      makeDiagnosis('a1', { ...state, lastNotified: '2026-08-26T10:15:00Z' }),
    ]);
    render(() => <OverviewTab {...defaultProps({ activeAlerts: { a1: makeAlert('a1') } })} />);
    await waitFor(() => expect(screen.getByText(/^Dispatch requested /)).toBeTruthy());
    expect(screen.queryByText(/^Notified /)).toBeNull();
    if (state.reason === 'cooldown') expect(screen.getByText(/next eligible/)).toBeTruthy();
  });

  it('ignores an older diagnosis response after the active alert set changes', async () => {
    let finishOlder!: (value: AlertDeliveryDiagnosis[]) => void;
    getDeliveryDiagnoses.mockReturnValueOnce(
      new Promise((resolve) => {
        finishOlder = resolve;
      }),
    );
    getDeliveryDiagnoses.mockResolvedValueOnce([
      makeDiagnosis('a1', { status: 'suppressed', reason: 'notifications_disabled' }),
    ]);
    const [alerts, setAlerts] = createSignal<Record<string, Alert>>({ a1: makeAlert('a1') });
    render(() => <OverviewTab {...defaultProps()} activeAlerts={alerts()} />);
    await waitFor(() => expect(getDeliveryDiagnoses).toHaveBeenCalledTimes(1));
    setAlerts({ a1: makeAlert('a1'), a2: makeAlert('a2') });
    await waitFor(() => expect(screen.getByText('Notifications are turned off')).toBeTruthy());
    finishOlder([makeDiagnosis('a1', { lastNotified: '2026-08-26T10:15:00Z' })]);
    await Promise.resolve();
    expect(screen.queryByText(/^Dispatch requested /)).toBeNull();
    expect(screen.getByText('Notifications are turned off')).toBeTruthy();
  });

  it.each(['success', 'failure'] as const)('does not apply a resolved incident %s to its recurrence on the same resource', async (outcome) => {
    let finishOlder!: (value: AlertDeliveryDiagnosis[]) => void;
    let rejectOlder!: (reason: Error) => void;
    getDeliveryDiagnoses.mockReturnValueOnce(
      new Promise((resolve, reject) => {
        finishOlder = resolve;
        rejectOlder = reject;
      }),
    );
    getDeliveryDiagnoses.mockResolvedValueOnce([
      makeDiagnosis('a2', { status: 'suppressed', reason: 'notifications_disabled' }),
    ]);
    const original = makeAlert('a1');
    const recurrence = {
      ...makeAlert('a2'),
      resourceId: original.resourceId,
      resourceName: original.resourceName,
      startTime: '2026-09-06T19:00:00Z',
    };
    const [alerts, setAlerts] = createSignal<Record<string, Alert>>({ a1: original });
    render(() => <OverviewTab {...defaultProps()} activeAlerts={alerts()} />);
    await waitFor(() => expect(getDeliveryDiagnoses).toHaveBeenCalledTimes(1));
    setAlerts({ a2: recurrence });
    await waitFor(() => expect(screen.getByText('Notifications are turned off')).toBeTruthy());
    expect(getDeliveryDiagnoses).toHaveBeenCalledTimes(2);
    if (outcome === 'success') {
      finishOlder([makeDiagnosis('a1', { lastNotified: '2026-08-26T10:15:00Z' })]);
    } else {
      rejectOlder(new Error('Previous incident diagnosis request failed'));
    }
    await Promise.resolve();
    expect(screen.getByText('High CPU on VM a2')).toBeTruthy();
    expect(screen.queryByText('High CPU on VM a1')).toBeNull();
    expect(screen.queryByText(/^Dispatch requested /)).toBeNull();
    expect(screen.getByText('Notifications are turned off')).toBeTruthy();
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
