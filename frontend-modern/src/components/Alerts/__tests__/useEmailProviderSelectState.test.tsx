import { renderHook, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsAPI } from '@/api/notifications';

import { useEmailProviderSelectState } from '../useEmailProviderSelectState';

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getEmailProviders: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

const gmailProvider = {
  name: 'Gmail',
  smtpHost: 'smtp.gmail.com',
  smtpPort: 587,
  tls: false,
  startTLS: true,
  authRequired: true,
  instructions: 'Use an App Password from your Google Account settings.',
};

const sendgridProvider = {
  name: 'SendGrid',
  smtpHost: 'smtp.sendgrid.net',
  smtpPort: 587,
  tls: false,
  startTLS: true,
  authRequired: true,
  instructions: 'Create an API key in the SendGrid dashboard.',
};

const buildConfig = (overrides: Record<string, unknown> = {}) => ({
  enabled: true,
  provider: '',
  server: '',
  port: 587,
  username: 'ops@example.com',
  password: '',
  from: 'pulse@example.com',
  to: ['alerts@example.com'],
  tls: false,
  startTLS: true,
  replyTo: '',
  maxRetries: 3,
  retryDelay: 5,
  rateLimit: 60,
  ...overrides,
});

describe('useEmailProviderSelectState', () => {
  beforeEach(() => {
    vi.mocked(NotificationsAPI.getEmailProviders).mockReset();
  });

  it('owns provider catalog loading and provider-default application separately from the render shell', async () => {
    vi.mocked(NotificationsAPI.getEmailProviders).mockResolvedValue([
      gmailProvider,
      sendgridProvider,
    ] as never);

    const onChange = vi.fn();
    const { result } = renderHook(() =>
      useEmailProviderSelectState({
        config: buildConfig({ provider: 'Gmail' }),
        onChange,
      }),
    );

    await waitFor(() => expect(NotificationsAPI.getEmailProviders).toHaveBeenCalledTimes(1));
    expect(result.providers()).toEqual([
      expect.objectContaining({ name: 'Gmail' }),
      expect.objectContaining({ name: 'SendGrid' }),
    ]);
    expect(result.currentProvider()).toEqual(expect.objectContaining({ name: 'Gmail' }));

    result.handleProviderChange('SendGrid');
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        provider: 'SendGrid',
        server: 'smtp.sendgrid.net',
        port: 587,
        username: 'apikey',
      }),
    );

    onChange.mockClear();
    result.handleProviderChange('');
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ provider: '' }));

    expect(result.showAdvanced()).toBe(false);
    result.toggleShowAdvanced();
    expect(result.showAdvanced()).toBe(true);

    expect(result.showInstructions()).toBe(false);
    result.toggleShowInstructions();
    expect(result.showInstructions()).toBe(true);
  });
});
