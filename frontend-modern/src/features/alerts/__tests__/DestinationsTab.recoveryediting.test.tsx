import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { NotificationsAPI, type NotificationHealth } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import { DestinationsTab } from '../tabs/DestinationsTab';
import type { UIAppriseConfig, UIEmailConfig } from '../types';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getHealth: vi.fn(),
    getDeliveryLog: vi.fn(),
    retryTerminalFailures: vi.fn(),
    dismissTerminalFailures: vi.fn(),
  },
}));
vi.mock('@/api/alerts', () => ({ AlertsAPI: { getEvents: vi.fn().mockResolvedValue([]) } }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));
vi.mock('@/stores/license', () => ({ hasFeature: () => false }));
vi.mock('@/stores/licenseCommercial', () => ({ getUpgradeActionDestination: () => null }));
vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => true,
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ config: () => null }),
}));
// Keep the actual tab, delivery hooks, health/log/feedback cards and SMTP editor.
// Unrelated destination transports and their independent network calls are excluded.
vi.mock('../AlertAppriseDestinationsSection', () => ({
  AlertAppriseDestinationsSection: () => null,
}));
vi.mock('../AlertWebhookDestinationsSection', () => ({
  AlertWebhookDestinationsSection: () => null,
}));
vi.mock('../AlertDeadManDestinationSection', () => ({
  AlertDeadManDestinationSection: () => null,
}));
vi.mock('../AlertPushDestinationsSection', () => ({ AlertPushDestinationsSection: () => null }));

const buildEmailConfig = (): UIEmailConfig => ({
  enabled: true,
  from: 'pulse@example.com',
  maxRetries: 3,
  password: '',
  port: 587,
  provider: 'smtp',
  rateLimit: 60,
  replyTo: '',
  retryDelay: 5,
  server: 'smtp.example.com',
  startTLS: true,
  tls: true,
  to: ['alerts@example.com'],
  username: 'ops@example.com',
});

const buildAppriseConfig = (): UIAppriseConfig => ({
  apiKey: '',
  apiKeyHeader: 'X-API-KEY',
  cliPath: '/usr/local/bin/apprise',
  configKey: '',
  enabled: true,
  hasApiKey: false,
  mode: 'cli',
  serverUrl: '',
  skipTlsVerify: false,
  targetsText: 'mailto://alerts@example.com',
  timeoutSeconds: 20,
});

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

describe('DestinationsTab recovery while editing SMTP', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    setActiveLocale(DEFAULT_LOCALE);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    vi.mocked(NotificationsAPI.getHealth).mockResolvedValue(degradedHealth());
    vi.mocked(NotificationsAPI.getDeliveryLog).mockResolvedValue({
      entries: [
        {
          notificationId: 'retained-attempt',
          type: 'email',
          destinationId: 'email:fixture',
          outcome: 'failed',
          success: false,
          alertIds: ['retained-alert'],
          alertCount: 1,
          attempts: 3,
          timestamp: '2026-09-07T12:00:00Z',
          errorMessage: 'SMTP fixture rejected',
        },
      ],
      windowDays: 7,
      completedRetentionDays: 7,
      deadLetterRetentionDays: 30,
    });
  });
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it.each(
    (
      [
        ['retryTerminalFailures', 'Retry retained deliveries', 'retry'],
        ['dismissTerminalFailures', 'Dismiss retained failures', 'dismiss'],
      ] as const
    ).flatMap(([action, label, verb]) =>
      (['available', 'unavailable'] as const).map((refresh) => ({ action, label, verb, refresh })),
    ),
  )(
    'preserves unfinished edits through $action with $refresh refresh',
    async ({ action, label, verb, refresh }) => {
      const [emailConfig, setEmailConfig] = createSignal(buildEmailConfig());
      const [appriseConfig, setAppriseConfig] = createSignal(buildAppriseConfig());
      const dirty = vi.fn();
      render(() => (
        <DestinationsTab
          emailConfig={emailConfig}
          setEmailConfig={setEmailConfig}
          appriseConfig={appriseConfig}
          setAppriseConfig={setAppriseConfig}
          setHasUnsavedChanges={dirty}
          configLoadError={() => null}
          isRetrying={() => false}
          isLoadingDestinations={() => false}
          onRetryLoad={vi.fn()}
          webhooks={() => []}
          deadManPingUrl={() => ''}
          setDeadManPingUrl={vi.fn()}
          pushMinimumSeverity={() => 'all'}
          setPushMinimumSeverity={vi.fn()}
        />
      ));
      await screen.findByRole('button', { name: label });
      await screen.findByText('SMTP fixture rejected');
      const editor = screen.getByRole('textbox', { name: 'SMTP server' });
      fireEvent.input(editor, { target: { value: 'unfinished.smtp.example' } });
      expect(dirty).toHaveBeenCalledWith(true);
      dirty.mockClear();
      const status = screen.getByRole('status');

      let reject!: (error: Error) => void;
      vi.mocked(NotificationsAPI[action]).mockReturnValueOnce(
        new Promise((_, fail) => {
          reject = fail;
        }),
      );
      fireEvent.click(screen.getByRole('button', { name: label }));
      editor.focus();
      expect(screen.getByRole('button', { name: label })).toBeDisabled();
      reject(new Error('fixture rejection'));
      await waitFor(() => expect(status).toHaveTextContent('Unable to ' + verb));
      expect(editor).toHaveFocus();
      expect(editor).toHaveValue('unfinished.smtp.example');
      expect(emailConfig().server).toBe('unfinished.smtp.example');
      expect(dirty).not.toHaveBeenCalledWith(false);
      expect(screen.getByText('SMTP fixture rejected')).toBeInTheDocument();
      expect(NotificationsAPI.getDeliveryLog).toHaveBeenCalledTimes(1);

      // Cancelling after a failure must not erase that evidence or start work.
      vi.mocked(window.confirm).mockReturnValueOnce(false);
      fireEvent.click(screen.getByRole('button', { name: label }));
      expect(NotificationsAPI[action]).toHaveBeenCalledTimes(1);
      expect(NotificationsAPI.getHealth).toHaveBeenCalledTimes(1);
      expect(NotificationsAPI.getDeliveryLog).toHaveBeenCalledTimes(1);
      expect(screen.getByRole('button', { name: label })).toBeEnabled();
      expect(screen.getByRole('status')).toBe(status);
      expect(status).toHaveTextContent('Unable to ' + verb);
      expect(editor).toHaveFocus();
      expect(editor).toHaveValue('unfinished.smtp.example');
      expect(dirty).not.toHaveBeenCalledWith(false);
      expect(screen.getByText('SMTP fixture rejected')).toBeInTheDocument();

      let accept!: (result: { success: boolean; affected: number }) => void;
      vi.mocked(NotificationsAPI[action]).mockReturnValueOnce(
        new Promise((resolve) => {
          accept = resolve;
        }),
      );
      const healthy = degradedHealth();
      healthy.queue = {
        ...healthy.queue,
        status: 'healthy',
        healthy: true,
        attentionRequired: 0,
        failed: 0,
        deadLetter: 0,
      };
      if (refresh === 'unavailable') {
        vi.mocked(NotificationsAPI.getHealth).mockRejectedValueOnce(
          new Error('health unavailable'),
        );
        vi.mocked(NotificationsAPI.getDeliveryLog).mockRejectedValueOnce(
          new Error('history unavailable'),
        );
      } else {
        vi.mocked(NotificationsAPI.getHealth).mockResolvedValueOnce(healthy);
      }
      vi.mocked(notificationStore.error).mockClear();
      fireEvent.click(screen.getByRole('button', { name: label }));
      editor.focus();
      accept({ success: true, affected: 85 });
      if (refresh === 'available') {
        await waitFor(() => expect(screen.queryByRole('button', { name: label })).toBeNull());
      } else {
        await screen.findByText('Notification delivery status is unavailable');
        await waitFor(() => expect(screen.getByRole('button', { name: label })).toBeEnabled());
      }
      await waitFor(() => expect(NotificationsAPI.getDeliveryLog).toHaveBeenCalledTimes(2));
      expect(screen.getByRole('textbox', { name: 'SMTP server' })).toBe(editor);
      expect(editor).toHaveFocus();
      expect(editor).toHaveValue('unfinished.smtp.example');
      expect(emailConfig().server).toBe('unfinished.smtp.example');
      expect(dirty).not.toHaveBeenCalledWith(false);
      expect(screen.getByRole('status')).toBe(status);
      expect(status).toBeEmptyDOMElement();
      if (refresh === 'unavailable') {
        expect(
          await screen.findByText('Notification delivery status is unavailable'),
        ).toBeInTheDocument();
        expect(
          await screen.findByText(
            'Pulse could not read the delivery log, so recent delivery activity cannot be shown.',
          ),
        ).toBeInTheDocument();
        expect(screen.queryByText('SMTP fixture rejected')).not.toBeInTheDocument();
      } else {
        expect(screen.getByText('SMTP fixture rejected')).toBeInTheDocument();
      }
      // A failed read must not relabel an accepted mutation as rejected.
      expect(notificationStore.success).toHaveBeenCalledTimes(1);
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(NotificationsAPI.getHealth).toHaveBeenCalledTimes(2);
      expect(NotificationsAPI[action]).toHaveBeenCalledTimes(2);
    },
  );
});
