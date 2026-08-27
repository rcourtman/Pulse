import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockedApiFetchJSON = vi.mocked(apiFetchJSON);

describe('AlertsAPI external watchdog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('uses the canonical masked configuration and operational status routes', async () => {
    mockedApiFetchJSON.mockResolvedValueOnce({
      pingUrl: '***REDACTED***',
      configured: true,
    });
    await expect(AlertsAPI.getDeadManConfig()).resolves.toEqual({
      pingUrl: '***REDACTED***',
      configured: true,
    });
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/alerts/deadman/config');

    mockedApiFetchJSON.mockResolvedValueOnce({
      configured: true,
      state: 'healthy',
      heartbeatIntervalSeconds: 60,
      recommendedGraceSeconds: 180,
      consecutiveFailures: 0,
    });
    await expect(AlertsAPI.getDeadManStatus()).resolves.toMatchObject({ state: 'healthy' });
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/alerts/deadman/status');

    mockedApiFetchJSON.mockResolvedValueOnce({ success: true, configured: true });
    await expect(
      AlertsAPI.updateDeadManConfig('https://watchdog.example.com/ping/replacement-token'),
    ).resolves.toEqual({ success: true, configured: true });
    expect(mockedApiFetchJSON).toHaveBeenLastCalledWith('/api/alerts/deadman/config', {
      method: 'PUT',
      body: JSON.stringify({
        pingUrl: 'https://watchdog.example.com/ping/replacement-token',
      }),
    });
  });
});
