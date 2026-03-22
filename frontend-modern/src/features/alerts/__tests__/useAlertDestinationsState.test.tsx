import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';

import { useAlertDestinationsState } from '../useAlertDestinationsState';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getAppriseConfig: vi.fn(),
    getEmailConfig: vi.fn(),
    updateAppriseConfig: vi.fn(),
    updateEmailConfig: vi.fn(),
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

    const { result } = renderHook(() => useAlertDestinationsState({ activeTab }));

    await result.loadDestinations();
    expect(NotificationsAPI.getEmailConfig).toHaveBeenCalledTimes(1);
    expect(NotificationsAPI.getAppriseConfig).toHaveBeenCalledTimes(1);
    expect(result.emailConfig().server).toBe('smtp.example.com');
    expect(result.appriseConfig().targetsText).toContain('mailto://ops@example.com');

    setActiveTab('destinations');
    await Promise.resolve();
    await Promise.resolve();
    expect(NotificationsAPI.getEmailConfig).toHaveBeenCalledTimes(2);
    expect(NotificationsAPI.getAppriseConfig).toHaveBeenCalledTimes(2);

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
    expect(result.appriseConfig().mode).toBe('http');
    expect(result.appriseConfig().serverUrl).toBe('https://apprise.example.test');

    result.resetDestinations();
    expect(result.destConfigLoadError()).toBeNull();
    expect(result.emailConfig().enabled).toBe(false);
    expect(result.appriseConfig().enabled).toBe(false);
  });
});
