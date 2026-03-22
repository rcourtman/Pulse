import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';
import { notificationStore } from '@/stores/notifications';
import { showErrorWithDetail } from '@/utils/toast';

import { useAlertDestinationsTabState } from '../useAlertDestinationsTabState';
import type { UIAppriseConfig, UIEmailConfig } from '../types';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    createWebhook: vi.fn(),
    deleteWebhook: vi.fn(),
    getWebhooks: vi.fn(),
    testNotification: vi.fn(),
    testWebhook: vi.fn(),
    updateWebhook: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: vi.fn(),
    success: vi.fn(),
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
  mode: 'cli',
  serverUrl: '',
  skipTlsVerify: false,
  targetsText: 'mailto://alerts@example.com',
  timeoutSeconds: 20,
});

describe('useAlertDestinationsTabState', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.createWebhook).mockReset();
    vi.mocked(NotificationsAPI.deleteWebhook).mockReset();
    vi.mocked(NotificationsAPI.getWebhooks).mockReset();
    vi.mocked(NotificationsAPI.testNotification).mockReset();
    vi.mocked(NotificationsAPI.testWebhook).mockReset();
    vi.mocked(NotificationsAPI.updateWebhook).mockReset();
    vi.mocked(notificationStore.error).mockReset();
    vi.mocked(notificationStore.success).mockReset();
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
    vi.mocked(NotificationsAPI.testNotification).mockResolvedValue({ success: true } as never);
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
    expect(notificationStore.success).toHaveBeenCalledTimes(2);
    expect(showErrorWithDetail).not.toHaveBeenCalled();
  });
});
