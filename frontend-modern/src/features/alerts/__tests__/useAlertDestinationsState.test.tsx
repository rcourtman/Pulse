import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';
import { AlertsAPI } from '@/api/alerts';

import { useAlertDestinationsState } from '../useAlertDestinationsState';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getAppriseConfig: vi.fn(),
    getEmailConfig: vi.fn(),
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

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

describe('useAlertDestinationsState', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.getEmailConfig).mockReset();
    vi.mocked(NotificationsAPI.getAppriseConfig).mockReset();
    vi.mocked(NotificationsAPI.updateEmailConfig).mockReset();
    vi.mocked(NotificationsAPI.updateAppriseConfig).mockReset();
    vi.mocked(AlertsAPI.getDeadManConfig).mockReset();
    vi.mocked(AlertsAPI.updateDeadManConfig).mockReset();
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
    } as any);
    vi.mocked(NotificationsAPI.getAppriseConfig).mockResolvedValue({
      enabled: true,
      mode: 'cli',
      targets: ['mailto://ops@example.com'],
      cliPath: '/usr/local/bin/apprise',
      timeoutSeconds: 20,
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
    vi.mocked(AlertsAPI.getDeadManConfig).mockResolvedValue({
      pingUrl: '***REDACTED***',
      configured: true,
    });
    vi.mocked(AlertsAPI.updateDeadManConfig).mockResolvedValue({
      success: true,
      configured: true,
    });

    const { result } = renderHook(() => useAlertDestinationsState({ activeTab }));

    await result.loadDestinations();
    expect(NotificationsAPI.getEmailConfig).toHaveBeenCalledTimes(1);
    expect(NotificationsAPI.getAppriseConfig).toHaveBeenCalledTimes(1);
    expect(AlertsAPI.getDeadManConfig).toHaveBeenCalledTimes(1);
    expect(result.emailConfig().server).toBe('smtp.example.com');
    expect(result.appriseConfig().targetsText).toContain('mailto://ops@example.com');
    expect(result.deadManPingUrl()).toBe('***REDACTED***');

    setActiveTab('destinations');
    await Promise.resolve();
    await Promise.resolve();
    expect(NotificationsAPI.getEmailConfig).toHaveBeenCalledTimes(2);
    expect(NotificationsAPI.getAppriseConfig).toHaveBeenCalledTimes(2);
    expect(AlertsAPI.getDeadManConfig).toHaveBeenCalledTimes(2);

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
    expect(result.appriseConfig().mode).toBe('http');
    expect(result.appriseConfig().serverUrl).toBe('https://apprise.example.test');

    result.resetDestinations();
    expect(result.destConfigLoadError()).toBeNull();
    expect(result.emailConfig().enabled).toBe(false);
    expect(result.appriseConfig().enabled).toBe(false);
    expect(result.deadManPingUrl()).toBe('');
  });
});
