import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { TrueNASAPI, type TrueNASConnection } from '@/api/truenas';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('TrueNASAPI branch coverage (0724pm)', () => {
  const mock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    mock.mockReset();
  });

  describe('normalizeTrueNASConnection (response-shape fallbacks)', () => {
    it('defaults a blank name to "", zeroes every absent observed count, keeps legacy-rest mode and surfaces a structured poll error', async () => {
      mock.mockResolvedValueOnce([
        {
          id: 'c1',
          host: 'truenas.local',
          useHttps: true,
          insecureSkipVerify: false,
          enabled: true,
          poll: {
            lastError: {
              at: ' 2026-01-01T00:00:00Z ',
              message: ' boom ',
              category: ' net ',
            },
          },
          observed: {},
          transport: {
            mode: 'legacy-rest',
            tls: false,
            connected: false,
          },
        },
      ]);

      const [connection] = await TrueNASAPI.listConnections();

      expect(connection.name).toBe('');
      expect(connection.poll?.lastError).toEqual({
        at: '2026-01-01T00:00:00Z',
        message: 'boom',
        category: 'net',
      });
      expect(connection.observed).toEqual({
        host: undefined,
        resourceId: undefined,
        collectedAt: undefined,
        systems: 0,
        storagePools: 0,
        datasets: 0,
        apps: 0,
        vms: 0,
        shares: 0,
        disks: 0,
        recoveryArtifacts: 0,
      });
      expect(connection.transport).toEqual({
        mode: 'legacy-rest',
        endpoint: undefined,
        tls: false,
        connected: false,
        authMechanism: undefined,
        applianceVersion: undefined,
        legacyReason: undefined,
        reconnects: undefined,
        lastError: undefined,
        lastConnectedAt: undefined,
      });
    });

    it('drops a non-object poll.lastError and collapses an unknown transport mode to negotiating', async () => {
      mock.mockResolvedValueOnce([
        {
          id: 'c2',
          host: 'truenas.local',
          useHttps: true,
          insecureSkipVerify: false,
          enabled: true,
          poll: { lastError: 'boom' },
          transport: { mode: 'totally-bogus' },
        },
      ] as unknown as TrueNASConnection[]);

      const [connection] = await TrueNASAPI.listConnections();

      expect(connection.poll?.lastError).toBeUndefined();
      expect(connection.transport?.mode).toBe('negotiating');
      expect(connection.transport?.tls).toBe(false);
      expect(connection.transport?.connected).toBe(false);
    });

    it('treats an absent transport mode as negotiating and drops a null poll.lastError', async () => {
      mock.mockResolvedValueOnce([
        {
          id: 'c3',
          host: 'truenas.local',
          useHttps: true,
          insecureSkipVerify: false,
          enabled: true,
          poll: { lastError: null },
          transport: {},
        },
      ]);

      const [connection] = await TrueNASAPI.listConnections();

      expect(connection.transport?.mode).toBe('negotiating');
      expect(connection.poll?.lastError).toBeUndefined();
    });
  });

  describe('listConnections (non-array response)', () => {
    it('returns an empty list when the backend answers with null', async () => {
      mock.mockResolvedValueOnce(null);

      await expect(TrueNASAPI.listConnections()).resolves.toEqual([]);
    });
  });

  describe('testSavedConnection (no input override)', () => {
    it('encodes the id and sends a bodyless POST when no input is supplied', async () => {
      mock.mockResolvedValueOnce({ success: true });

      await expect(TrueNASAPI.testSavedConnection('conn/with slash')).resolves.toEqual({
        success: true,
      });

      expect(mock).toHaveBeenLastCalledWith('/api/truenas/connections/conn%2Fwith%20slash/test', {
        method: 'POST',
      });
    });
  });
});
