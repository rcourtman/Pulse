import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import type { Alert } from '@/types/api';
import type { NotificationHealth } from '@/api/notifications';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

const getDeliveryDiagnoses = vi.fn<() => Promise<never[]>>();

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    get getDeliveryDiagnoses() {
      return getDeliveryDiagnoses;
    },
  },
}));

const getHealth = vi.fn<() => Promise<NotificationHealth>>();
const retryTerminalFailures = vi.fn();
const dismissTerminalFailures = vi.fn();

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    get getHealth() {
      return getHealth;
    },
    get retryTerminalFailures() {
      return retryTerminalFailures;
    },
    get dismissTerminalFailures() {
      return dismissTerminalFailures;
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

function degradedHealth(): NotificationHealth {
  return {
    overallHealthy: false,
    queue: {
      pending: 0,
      sending: 0,
      sent: 12,
      failed: 0,
      deadLetter: 85,
      healthy: false,
      status: 'degraded',
      attentionRequired: 85,
      reasonCodes: ['dead_letter_retained'],
      completedRetentionDays: 7,
      deadLetterRetentionDays: 30,
      countsAreRetentionBounded: true,
      retryAttemptsAffectHealth: false,
      terminalFailuresAffectHealth: true,
      failureClasses7d: {
        authentication: 0,
        rate_limited: 0,
        connectivity: 1,
        tls: 0,
        configuration: 0,
        rejected: 0,
        server_error: 0,
        unknown: 0,
      },
      failureClassesAvailable: true,
      failureClassWindowDays: 7,
    },
  };
}

function defaultProps() {
  return {
    overrides: [] as never[],
    activeAlerts: {} as Record<string, Alert>,
    updateAlert: vi.fn(),
    showQuickTip: () => false,
    dismissQuickTip: vi.fn(),
    showAcknowledged: () => true,
    setShowAcknowledged: vi.fn(),
    alertsDisabled: () => false,
  };
}

// The delivery warning shows up on the alerts overview, so the actions that
// clear it must be there too. A warning whose only remedy lives on another
// tab is the complaint in #1812.
describe('OverviewTab delivery health actions', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
    getDeliveryDiagnoses.mockReset();
    getDeliveryDiagnoses.mockResolvedValue([]);
    getHealth.mockReset();
  });

  afterEach(() => {
    cleanup();
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('offers retry and dismiss on the overview delivery warning', async () => {
    getHealth.mockResolvedValue(degradedHealth());

    render(() => <OverviewTab {...defaultProps()} />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
    expect(screen.getByRole('button', { name: 'Retry retained deliveries' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Dismiss retained failures' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Refresh delivery status' })).toBeNull();
    expect(screen.getByRole('alert')).toHaveTextContent('Most recent failures: connectivity (1).');
  });
});
