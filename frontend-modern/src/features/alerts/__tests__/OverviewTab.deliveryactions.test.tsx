import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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

import { notificationStore } from '@/stores/notifications';

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
    retryTerminalFailures.mockReset();
    dismissTerminalFailures.mockReset();
    vi.mocked(notificationStore.success).mockClear();
    vi.mocked(notificationStore.error).mockClear();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
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
  for (const action of [
    { name: 'Retry retained deliveries', api: retryTerminalFailures },
    { name: 'Dismiss retained failures', api: dismissTerminalFailures },
  ]) {
    it(`does not mutate or refresh when ${action.name} is cancelled`, async () => {
      getHealth.mockResolvedValue(degradedHealth());
      const confirmation = vi.spyOn(window, 'confirm').mockReturnValue(false);
      render(() => <OverviewTab {...defaultProps()} />);
      fireEvent.click(await screen.findByRole('button', { name: action.name }));

      expect(confirmation).toHaveBeenCalledOnce();
      expect(action.api).not.toHaveBeenCalled();
      expect(getHealth).toHaveBeenCalledTimes(1);
      expect(screen.getByRole('alert')).toBeTruthy();
    });

    it(`retains attention and enables another attempt when ${action.name} fails`, async () => {
      getHealth.mockResolvedValue(degradedHealth());
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      action.api.mockRejectedValue(new Error('queue action unavailable'));
      render(() => <OverviewTab {...defaultProps()} />);
      fireEvent.click(await screen.findByRole('button', { name: action.name }));

      await waitFor(() => expect(notificationStore.error).toHaveBeenCalledOnce());
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(getHealth).toHaveBeenCalledTimes(1);
      expect(screen.getByRole('alert')).toHaveTextContent('connectivity (1)');
      expect(screen.getByRole('button', { name: action.name })).not.toBeDisabled();
    });

    it(`keeps health visibly unknown after ${action.name} succeeds but refresh fails`, async () => {
      getHealth
        .mockResolvedValueOnce(degradedHealth())
        .mockRejectedValueOnce(new Error('health unavailable'));
      action.api.mockResolvedValue({ affected: 85 });
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      render(() => <OverviewTab {...defaultProps()} />);
      fireEvent.click(await screen.findByRole('button', { name: action.name }));

      const refresh = await screen.findByRole('button', { name: 'Refresh delivery status' });
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(notificationStore.success).toHaveBeenCalledOnce();
      expect(notificationStore.error).not.toHaveBeenCalled();

      const healthy = degradedHealth();
      healthy.overallHealthy = true;
      healthy.queue = {
        ...healthy.queue,
        status: 'healthy',
        healthy: true,
        attentionRequired: 0,
        deadLetter: 0,
      };
      getHealth.mockResolvedValueOnce(healthy);
      await waitFor(() => expect(refresh).not.toBeDisabled());
      fireEvent.click(refresh);
      await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
      expect(getHealth).toHaveBeenCalledTimes(3);
      expect(action.api).toHaveBeenCalledOnce();
    });

    it(`keeps both actions disabled until ${action.name} and its health refresh finish`, async () => {
      const healthy = degradedHealth();
      healthy.overallHealthy = true;
      healthy.queue = {
        ...healthy.queue,
        status: 'healthy',
        healthy: true,
        attentionRequired: 0,
        deadLetter: 0,
      };
      let completeAction!: (value: { affected: number }) => void;
      let completeHealth!: (value: NotificationHealth) => void;
      action.api.mockReturnValue(
        new Promise((resolve) => {
          completeAction = resolve;
        }),
      );
      getHealth.mockResolvedValueOnce(degradedHealth()).mockReturnValueOnce(
        new Promise((resolve) => {
          completeHealth = resolve;
        }),
      );
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      render(() => <OverviewTab {...defaultProps()} />);
      fireEvent.click(await screen.findByRole('button', { name: action.name }));

      const expectActionsDisabled = () => {
        const buttons = screen.getByRole('alert').querySelectorAll('button');
        expect(buttons.length).toBe(2);
        for (const button of buttons) expect(button).toBeDisabled();
      };
      expectActionsDisabled();
      expect(getHealth).toHaveBeenCalledTimes(1);
      completeAction({ affected: 85 });
      await waitFor(() => expect(getHealth).toHaveBeenCalledTimes(2));
      expectActionsDisabled();
      expect(screen.getByRole('alert')).toBeTruthy();

      completeHealth(healthy);
      await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
      expect(action.api).toHaveBeenCalledOnce();
      expect(notificationStore.success).toHaveBeenCalledOnce();
      expect(notificationStore.error).not.toHaveBeenCalled();
    });
  }
});
