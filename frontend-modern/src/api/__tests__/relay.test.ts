import { describe, expect, it, vi, beforeEach } from 'vitest';
import { RelayAPI, type RelayConfig, type RelayStatus } from '../relay';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('RelayAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getConfig', () => {
    it('fetches relay config', async () => {
      const mockConfig: RelayConfig = { enabled: true, server_url: 'https://relay.example.com' };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockConfig);

      const result = await RelayAPI.getConfig();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/settings/relay');
      expect(result).toEqual(mockConfig);
    });
  });

  describe('updateConfig', () => {
    it('updates relay config', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await RelayAPI.updateConfig({ enabled: false });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/settings/relay',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ enabled: false }),
        })
      );
    });

    it('updates server URL', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await RelayAPI.updateConfig({ server_url: 'https://new-server.com' });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/settings/relay',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ server_url: 'https://new-server.com' }),
        })
      );
    });
  });

  describe('getStatus', () => {
    it('fetches relay status', async () => {
      const mockStatus: RelayStatus = { connected: true, active_channels: 5 };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockStatus);

      const result = await RelayAPI.getStatus();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/settings/relay/status');
      expect(result).toEqual(mockStatus);
    });
  });
});
