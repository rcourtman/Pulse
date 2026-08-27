import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';
import { AlertsAPI } from '@/api/alerts';
import { RelayAPI } from '@/api/relay';
import { hasFeature } from '@/stores/license';

import { useAlertDestinationsState } from '../useAlertDestinationsState';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getAppriseConfig: vi.fn(),
    getEmailConfig: vi.fn(),
    getWebhooks: vi.fn(),
    updateAppriseConfig: vi.fn(),
    updateEmailConfig: vi.fn(),
  },
}));

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getDeadManConfig: vi.fn(),
    updateDeadManConfig: vi.fn(),
  },
}));

vi.mock('@/api/relay', () => ({
  RelayAPI: {
    getConfig: vi.fn(),
    updateConfig: vi.fn(),
  },
}));

vi.mock('@/stores/license', () => ({
  hasFeature: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

describe('useAlertDestinationsState', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.getEmailConfig).mockReset();
    vi.mocked(NotificationsAPI.getAppriseConfig).mockReset();
    vi.mocked(NotificationsAPI.getWebhooks).mockReset();
    vi.mocked(NotificationsAPI.updateEmailConfig).mockReset();
    vi.mocked(NotificationsAPI.updateAppriseConfig).mockReset();
    vi.mocked(AlertsAPI.getDeadManConfig).mockReset();
    vi.mocked(AlertsAPI.updateDeadManConfig).mockReset();
    vi.mocked(RelayAPI.getConfig).mockReset();
    vi.mocked(RelayAPI.updateConfig).mockReset();
    vi.mocked(hasFeature).mockReset();
    vi.mocked(hasFeature).mockReturnValue(true);
  });

  it('owns alert destinations reload and save behavior separately from alert policy config', async () => {
    const [activeTab, setActiveTab] = createSignal<'overview' | 'destinations'>('overview');

    vi.mocked(NotificationsAPI.getEmailConfig).mockResolvedValue({
      enabled: true,
      provider: 'smtp',
      server: 'smtp.example.com',
      port: 587,
      username: 'ops@example.com',
      password: '',
      from: 'pulse@example.com',
      to: ['alerts@example.com'],
      tls: true,
      startTLS: true,
      minimumSeverity: 'critical',
    } as any);
    vi.mocked(NotificationsAPI.getAppriseConfig).mockResolvedValue({
      enabled: true,
      mode: 'cli',
      targets: ['mailto://ops@example.com'],
      cliPath: '/usr/local/bin/apprise',
      timeoutSeconds: 20,
      minimumSeverity: 'critical',
    } as any);
    vi.mocked(NotificationsAPI.updateEmailConfig).mockResolvedValue(undefined as any);
    vi.mocked(NotificationsAPI.updateAppriseConfig).mockResolvedValue({
      enabled: true,
      mode: 'http',
      targets: ['https://notify.example.test'],
      serverUrl: 'https://apprise.example.test',
      configKey: 'prod',
      apiKey: 'masked',
      apiKeyHeader: 'X-API-KEY',
      timeoutSeconds: 30,
      skipTlsVerify: false,
    } as any);
    vi.mocked(NotificationsAPI.getWebhooks).mockResolvedValue([
      {
        id: 'pager',
        name: 'Pager',
        url: 'https://pager.example.test',
        method: 'POST',
        headers: {},
        enabled: true,
      },
    ]);
    vi.mocked(AlertsAPI.getDeadManConfig).mockResolvedValue({
      pingUrl: '***REDACTED***',
      configured: true,
    });
    vi.mocked(AlertsAPI.updateDeadManConfig).mockResolvedValue({
      success: true,
      configured: true,
    });
    vi.mocked(RelayAPI.getConfig).mockResolvedValue({
      enabled: true,
      server_url: 'wss://relay.example.test',
      alert_minimum_severity: 'critical',
    });
    vi.mocked(RelayAPI.updateConfig).mockResolvedValue(undefined);

    const { result } = renderHook(() => useAlertDestinationsState({ activeTab }));

    await result.loadDestinations();
    expect(NotificationsAPI.getEmailConfig).toHaveBeenCalledTimes(1);
    expect(NotificationsAPI.getAppriseConfig).toHaveBeenCalledTimes(1);
    expect(NotificationsAPI.getWebhooks).toHaveBeenCalledTimes(1);
    expect(AlertsAPI.getDeadManConfig).toHaveBeenCalledTimes(1);
    expect(RelayAPI.getConfig).toHaveBeenCalledTimes(1);
    expect(result.emailConfig().server).toBe('smtp.example.com');
    expect(result.emailConfig().minimumSeverity).toBe('critical');
    expect(result.appriseConfig().targetsText).toContain('mailto://ops@example.com');
    expect(result.appriseConfig().minimumSeverity).toBe('critical');
    expect(result.deadManPingUrl()).toBe('***REDACTED***');
    expect(result.pushMinimumSeverity()).toBe('critical');
    expect(result.webhooks()).toEqual([
      expect.objectContaining({ id: 'pager', service: 'generic' }),
    ]);

    setActiveTab('destinations');
    await Promise.resolve();
    await Promise.resolve();
    expect(NotificationsAPI.getEmailConfig).toHaveBeenCalledTimes(2);
    expect(NotificationsAPI.getAppriseConfig).toHaveBeenCalledTimes(2);
    expect(NotificationsAPI.getWebhooks).toHaveBeenCalledTimes(2);
    expect(AlertsAPI.getDeadManConfig).toHaveBeenCalledTimes(2);
    expect(RelayAPI.getConfig).toHaveBeenCalledTimes(2);

    setActiveTab('overview');
    await Promise.resolve();

    result.setEmailConfig({
      ...result.emailConfig(),
      server: 'smtp.internal',
    });
    result.setAppriseConfig({
      ...result.appriseConfig(),
      mode: 'http',
      serverUrl: 'https://apprise.internal',
      targetsText: 'https://notify.internal',
    });
    result.setDeadManPingUrl('https://watchdog.example.test/ping/replacement-token');
    result.setPushMinimumSeverity('all');

    await result.saveDestinations();

    expect(NotificationsAPI.updateEmailConfig).toHaveBeenCalledWith(
      expect.objectContaining({ server: 'smtp.internal' }),
    );
    expect(NotificationsAPI.updateAppriseConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        mode: 'http',
        serverUrl: 'https://apprise.internal',
        targets: ['https://notify.internal'],
      }),
    );
    expect(AlertsAPI.updateDeadManConfig).toHaveBeenCalledWith(
      'https://watchdog.example.test/ping/replacement-token',
    );
    expect(RelayAPI.updateConfig).toHaveBeenCalledWith({ alert_minimum_severity: 'all' });
    expect(result.appriseConfig().mode).toBe('http');
    expect(result.appriseConfig().serverUrl).toBe('https://apprise.example.test');

    result.resetDestinations();
    expect(result.destConfigLoadError()).toBeNull();
    expect(result.emailConfig().enabled).toBe(false);
    expect(result.appriseConfig().enabled).toBe(false);
    expect(result.deadManPingUrl()).toBe('');
    expect(result.pushMinimumSeverity()).toBe('all');
  });
});
