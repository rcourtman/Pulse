import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { NotificationsAPI } from '@/api/notifications';
import { apiFetchJSON } from '@/utils/apiClient';

describe('NotificationsAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('preserves valid zero values when mapping email config', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      enabled: false,
      provider: 'smtp',
      server: 'smtp.internal',
      port: 0,
      username: 'ops',
      password: '',
      from: 'pulse@internal',
      to: ['alerts@internal', 12],
      tls: false,
      startTLS: false,
      rateLimit: 0,
    } as any);

    const config = await NotificationsAPI.getEmailConfig();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/notifications/email');
    expect(config).toEqual({
      enabled: false,
      provider: 'smtp',
      server: 'smtp.internal',
      port: 0,
      username: 'ops',
      password: '',
      from: 'pulse@internal',
      to: ['alerts@internal'],
      tls: false,
      startTLS: false,
      rateLimit: 0,
    });
  });

  it('falls back to safe defaults for invalid backend field types', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      enabled: 'true',
      provider: 99,
      server: null,
      port: '587',
      username: undefined,
      password: false,
      from: {},
      to: 'alerts@internal',
      tls: 'false',
      startTLS: 1,
      rateLimit: '0',
    } as any);

    const config = await NotificationsAPI.getEmailConfig();

    expect(config).toEqual({
      enabled: false,
      provider: '',
      server: '',
      port: 587,
      username: '',
      password: '',
      from: '',
      to: [],
      tls: false,
      startTLS: false,
      rateLimit: undefined,
    });
  });
});
