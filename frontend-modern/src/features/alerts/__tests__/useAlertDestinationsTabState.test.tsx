import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { showErrorWithDetail } from '@/utils/toast';

import { useAlertDestinationsTabState } from '../useAlertDestinationsTabState';
import type { UIAppriseConfig, UIEmailConfig } from '../types';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    createWebhook: vi.fn(),
    deleteWebhook: vi.fn(),
    getDeliveryLog: vi.fn(),
    getHealth: vi.fn(),
    getWebhooks: vi.fn(),
    dismissTerminalFailures: vi.fn(),
    retryTerminalFailures: vi.fn(),
    testNotification: vi.fn(),
    testWebhook: vi.fn(),
    updateWebhook: vi.fn(),
  },
}));

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getEvents: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

vi.mock('@/utils/toast', () => ({
  showErrorWithDetail: vi.fn(),
}));

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

describe('useAlertDestinationsTabState', () => {
  beforeEach(() => {
    vi.mocked(AlertsAPI.getEvents).mockReset();
    vi.mocked(AlertsAPI.getEvents).mockResolvedValue([]);
    vi.mocked(NotificationsAPI.createWebhook).mockReset();
    vi.mocked(NotificationsAPI.deleteWebhook).mockReset();
    vi.mocked(NotificationsAPI.getDeliveryLog).mockReset();
    vi.mocked(NotificationsAPI.getHealth).mockReset();
    vi.mocked(NotificationsAPI.getWebhooks).mockReset();
    vi.mocked(NotificationsAPI.dismissTerminalFailures).mockReset();
    vi.mocked(NotificationsAPI.retryTerminalFailures).mockReset();
    vi.mocked(NotificationsAPI.testNotification).mockReset();
    vi.mocked(NotificationsAPI.testWebhook).mockReset();
    vi.mocked(NotificationsAPI.updateWebhook).mockReset();
    vi.mocked(notificationStore.error).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(notificationStore.warning).mockReset();
    vi.mocked(showErrorWithDetail).mockReset();
  });

  it('owns webhook runtime and destination test actions separately from config load/save state', async () => {
    const [emailConfig] = createSignal(buildEmailConfig());
    const [appriseConfig, setAppriseConfig] = createSignal(buildAppriseConfig());
    const [configLoadError] = createSignal<string | null>(null);
    const [isRetrying] = createSignal(false);
    const [isLoadingDestinations] = createSignal(false);
    const onRetryLoad = vi.fn();

    vi.mocked(NotificationsAPI.getWebhooks).mockResolvedValue([
      {
        enabled: true,
        headers: {},
        id: 'hook-1',
        method: 'POST',
        name: 'Ops',
        url: 'https://hooks.example.test/ops',
      },
    ] as never);
    vi.mocked(NotificationsAPI.getHealth).mockResolvedValue({
      overallHealthy: true,
      queue: {
        pending: 0,
        sending: 0,
        sent: 0,
        failed: 0,
        deadLetter: 0,
        healthy: true,
        status: 'healthy',
        attentionRequired: 0,
        reasonCodes: [],
        completedRetentionDays: 7,
        deadLetterRetentionDays: 30,
        countsAreRetentionBounded: true,
        retryAttemptsAffectHealth: false,
        terminalFailuresAffectHealth: true,
        failureClasses7d: {
          authentication: 0,
          rate_limited: 0,
          connectivity: 0,
          tls: 0,
          configuration: 0,
          rejected: 0,
          server_error: 0,
          unknown: 0,
        },
        failureClassesAvailable: true,
        failureClassWindowDays: 7,
      },
    });
    vi.mocked(NotificationsAPI.getDeliveryLog).mockResolvedValue({
      entries: [
        {
          notificationId: 'email-1',
          type: 'email',
          outcome: 'sent',
          alertIds: ['disk-critical-1'],
          alertCount: 1,
          attempts: 1,
          success: true,
          timestamp: '2026-08-20T12:00:00Z',
        },
      ],
      windowDays: 7,
    });
    vi.mocked(AlertsAPI.getEvents).mockResolvedValue([
      {
        id: 41,
        occurredAt: '2026-08-20T12:01:00Z',
        type: 'notification_suppressed',
        alertId: 'disk-critical-2',
        resourceName: 'backup-pool',
        alertType: 'usage',
        reason: 'notifications_inactive',
        message: 'Notification suppressed: alert delivery is not turned on.',
      },
    ]);
    vi.mocked(NotificationsAPI.testNotification).mockResolvedValue({ status: 'success' } as never);
    vi.mocked(NotificationsAPI.testWebhook).mockResolvedValue({ success: true } as never);

    const { result } = renderHook(() =>
      useAlertDestinationsTabState({
        appriseConfig,
        configLoadError,
        emailConfig,
        isLoadingDestinations,
        isRetrying,
        onRetryLoad,
        setAppriseConfig,
      }),
    );

    await waitFor(() => expect(NotificationsAPI.getWebhooks).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(NotificationsAPI.getHealth).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(NotificationsAPI.getDeliveryLog).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(AlertsAPI.getEvents).toHaveBeenCalledTimes(1));
    expect(AlertsAPI.getEvents).toHaveBeenCalledWith(
      expect.objectContaining({
        types: ['notification_suppressed', 'notification_deferred'],
        limit: 100,
      }),
    );
    expect(result.deliveryHealth()?.queue.status).toBe('healthy');
    expect(result.deliveryLog()?.entries).toHaveLength(1);
    expect(result.heldEvents()).toEqual([
      expect.objectContaining({ reason: 'notifications_inactive', resourceName: 'backup-pool' }),
    ]);
    expect(result.deliveryLogUnavailable()).toBe(false);
    expect(result.webhooks()).toEqual([
      expect.objectContaining({ id: 'hook-1', service: 'generic' }),
    ]);

    await result.testEmailConfig();
    expect(NotificationsAPI.testNotification).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'email' }),
    );

    await result.testApprise();
    expect(NotificationsAPI.testNotification).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'apprise',
        config: expect.objectContaining({
          mode: 'cli',
          targets: ['mailto://alerts@example.com'],
        }),
      }),
    );

    expect(result.webhooks()).toEqual([
      expect.objectContaining({ id: 'hook-1', service: 'generic' }),
    ]);

    result.updateApprise({ mode: 'http', serverUrl: 'https://apprise.internal' });
    expect(result.appriseState()).toEqual(
      expect.objectContaining({ mode: 'http', serverUrl: 'https://apprise.internal' }),
    );

    result.handleRetry();
    expect(onRetryLoad).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(NotificationsAPI.getWebhooks).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(NotificationsAPI.getHealth).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(NotificationsAPI.getDeliveryLog).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(AlertsAPI.getEvents).toHaveBeenCalledTimes(2));
    expect(notificationStore.success).toHaveBeenCalledTimes(2);
    expect(notificationStore.warning).not.toHaveBeenCalled();
    expect(showErrorWithDetail).not.toHaveBeenCalled();
  });

  it('warns instead of celebrating when a test send reports delivery is paused', async () => {
    const [emailConfig] = createSignal(buildEmailConfig());
    const [appriseConfig, setAppriseConfig] = createSignal(buildAppriseConfig());
    const [configLoadError] = createSignal<string | null>(null);
    const [isRetrying] = createSignal(false);
    const [isLoadingDestinations] = createSignal(false);

    vi.mocked(NotificationsAPI.getWebhooks).mockResolvedValue([]);
    vi.mocked(NotificationsAPI.getHealth).mockRejectedValue(new Error('offline'));
    vi.mocked(NotificationsAPI.getDeliveryLog).mockResolvedValue({ entries: [], windowDays: 7 });
    // The backend reports the test went out while the activation gate keeps
    // real alerts suppressed; plain success here is the postmortem trap.
    vi.mocked(NotificationsAPI.testNotification).mockResolvedValue({
      status: 'success',
      deliveryPaused: true,
    } as never);

    const { result } = renderHook(() =>
      useAlertDestinationsTabState({
        appriseConfig,
        configLoadError,
        emailConfig,
        isLoadingDestinations,
        isRetrying,
        onRetryLoad: vi.fn(),
        setAppriseConfig,
      }),
    );

    await result.testEmailConfig();
    expect(notificationStore.warning).toHaveBeenCalledWith(
      expect.stringContaining('delivery is paused'),
    );
    expect(notificationStore.success).not.toHaveBeenCalled();

    await result.testApprise();
    expect(notificationStore.warning).toHaveBeenCalledTimes(2);
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it('confirms and resolves retained terminal deliveries without deleting delivery history', async () => {
    const [emailConfig] = createSignal(buildEmailConfig());
    const [appriseConfig, setAppriseConfig] = createSignal(buildAppriseConfig());
    const [configLoadError] = createSignal<string | null>(null);
    const [isRetrying] = createSignal(false);
    const [isLoadingDestinations] = createSignal(false);
    const confirmSpy = vi.spyOn(globalThis, 'confirm').mockReturnValue(true);

    vi.mocked(NotificationsAPI.getWebhooks).mockResolvedValue([]);
    vi.mocked(NotificationsAPI.getHealth).mockResolvedValue({
      overallHealthy: false,
      queue: {
        attentionRequired: 3,
        completedRetentionDays: 7,
        countsAreRetentionBounded: true,
        deadLetter: 2,
        deadLetterRetentionDays: 30,
        failed: 1,
        failureClasses7d: {
          authentication: 3,
          configuration: 0,
          connectivity: 0,
          rate_limited: 0,
          rejected: 0,
          server_error: 0,
          tls: 0,
          unknown: 0,
        },
        failureClassesAvailable: true,
        failureClassWindowDays: 7,
        healthy: false,
        pending: 0,
        reasonCodes: ['retained_failed_deliveries', 'retained_dead_letter_deliveries'],
        retryAttemptsAffectHealth: false,
        sending: 0,
        sent: 0,
        status: 'degraded',
        terminalFailuresAffectHealth: true,
      },
    });
    vi.mocked(NotificationsAPI.getDeliveryLog).mockResolvedValue({ entries: [], windowDays: 7 });
    vi.mocked(NotificationsAPI.retryTerminalFailures).mockResolvedValue({
      affected: 3,
      success: true,
    });
    vi.mocked(NotificationsAPI.dismissTerminalFailures).mockResolvedValue({
      affected: 3,
      success: true,
    });

    const { result } = renderHook(() =>
      useAlertDestinationsTabState({
        appriseConfig,
        configLoadError,
        emailConfig,
        isLoadingDestinations,
        isRetrying,
        onRetryLoad: vi.fn(),
        setAppriseConfig,
      }),
    );

    await waitFor(() => expect(result.deliveryNeedsAttention()).toBe(true));
    await result.retryTerminalFailures();
    expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining('Retry 3 retained deliveries'));
    expect(NotificationsAPI.retryTerminalFailures).toHaveBeenCalledTimes(1);
    expect(notificationStore.success).toHaveBeenCalledWith(
      '3 retained deliveries queued for retry.',
    );

    await result.dismissTerminalFailures();
    expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining('Dismiss 3 retained failures'));
    expect(NotificationsAPI.dismissTerminalFailures).toHaveBeenCalledTimes(1);
    expect(notificationStore.success).toHaveBeenCalledWith('3 retained failures dismissed.');
    expect(NotificationsAPI.getDeliveryLog).toHaveBeenCalledTimes(3);

    confirmSpy.mockRestore();
  });
});
