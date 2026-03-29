import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { TrueNASAPI, isRedactedTrueNASSecret } from '@/api/truenas';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('TrueNASAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('normalizes listed connections from the API contract', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce([
      {
        id: ' conn-1 ',
        name: ' tower ',
        host: ' truenas.local ',
        port: 443,
        apiKey: ' ******** ',
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
      },
    ]);

    await expect(TrueNASAPI.listConnections()).resolves.toEqual([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        port: 443,
        apiKey: '********',
        username: undefined,
        password: undefined,
        useHttps: true,
        insecureSkipVerify: false,
        fingerprint: undefined,
        enabled: true,
        pollIntervalSeconds: undefined,
      },
    ]);
  });

  it('creates, updates, deletes, and tests connections through canonical routes', async () => {
    vi.mocked(apiFetchJSON)
      .mockResolvedValueOnce({
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
      })
      .mockResolvedValueOnce({
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        useHttps: false,
        insecureSkipVerify: true,
        enabled: false,
      })
      .mockResolvedValueOnce({ success: true, id: 'conn-1' })
      .mockResolvedValueOnce({ success: true });

    await TrueNASAPI.createConnection({
      name: 'tower',
      host: 'truenas.local',
      apiKey: 'secret',
      useHttps: true,
      enabled: true,
    });
    await TrueNASAPI.updateConnection('conn/1', {
      host: 'truenas.local',
      username: 'admin',
      password: '********',
      useHttps: false,
      insecureSkipVerify: true,
      enabled: false,
    });
    await TrueNASAPI.deleteConnection('conn/1');
    await expect(
      TrueNASAPI.testConnection({
        host: 'truenas.local',
        apiKey: 'secret',
      }),
    ).resolves.toEqual({ success: true });

    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      1,
      '/api/truenas/connections',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          name: 'tower',
          host: 'truenas.local',
          apiKey: 'secret',
          useHttps: true,
          enabled: true,
        }),
      }),
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      2,
      '/api/truenas/connections/conn%2F1',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          host: 'truenas.local',
          username: 'admin',
          password: '********',
          useHttps: false,
          insecureSkipVerify: true,
          enabled: false,
        }),
      }),
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      3,
      '/api/truenas/connections/conn%2F1',
      expect.objectContaining({ method: 'DELETE' }),
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      4,
      '/api/truenas/connections/test',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ host: 'truenas.local', apiKey: 'secret' }),
      }),
    );
  });

  it('recognizes the masked-secret placeholder used for update preservation', () => {
    expect(isRedactedTrueNASSecret('********')).toBe(true);
    expect(isRedactedTrueNASSecret(' ******** ')).toBe(true);
    expect(isRedactedTrueNASSecret('secret')).toBe(false);
    expect(isRedactedTrueNASSecret(undefined)).toBe(false);
  });
});
