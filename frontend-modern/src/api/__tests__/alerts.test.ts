import { describe, expect, it, vi, beforeEach } from 'vitest';
import { AlertsAPI } from '../alerts';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('AlertsAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getActive', () => {
    it('fetches active alerts', async () => {
      const mockAlerts = [{ id: 'alert-1', severity: 'critical' }];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockAlerts);

      const result = await AlertsAPI.getActive();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/alerts/active');
      expect(result).toEqual(mockAlerts);
    });
  });

  describe('getHistory', () => {
    it('fetches history with no params', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce([]);

      await AlertsAPI.getHistory();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/alerts/history?');
    });

    it('fetches history with params', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce([]);

      await AlertsAPI.getHistory({ limit: 50, severity: 'critical' });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/alerts/history?limit=50&severity=critical'
      );
    });

    it('handles undefined params', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce([]);

      await AlertsAPI.getHistory({ limit: 10, offset: undefined });

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/alerts/history?limit=10');
    });
  });

  describe('getConfig', () => {
    it('fetches alert config', async () => {
      const mockConfig = { activationState: 'active' };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockConfig);

      const result = await AlertsAPI.getConfig();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/alerts/config');
      expect(result).toEqual(mockConfig);
    });
  });

  describe('updateConfig', () => {
    it('updates alert config', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      const config = {
        enabled: true,
        activationState: 'active' as const,
        guestDefaults: {},
        nodeDefaults: {},
        storageDefault: { trigger: 90, clear: 80 },
        overrides: {},
      };
      const result = await AlertsAPI.updateConfig(config);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/alerts/config',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify(config),
        })
      );
      expect(result).toEqual({ success: true });
    });
  });

  describe('activate', () => {
    it('activates alerts', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true, state: 'active' });

      const result = await AlertsAPI.activate();

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/alerts/activate',
        expect.objectContaining({ method: 'POST' })
      );
      expect(result).toEqual({ success: true, state: 'active' });
    });
  });

  describe('clearHistory', () => {
    it('clears alert history', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ success: true });

      const result = await AlertsAPI.clearHistory();

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/alerts/history',
        expect.objectContaining({ method: 'DELETE' })
      );
      expect(result).toEqual({ success: true });
    });
  });

  describe('bulkAcknowledge', () => {
    it('acknowledges multiple alerts', async () => {
      const results = {
        results: [
          { alertId: 'alert-1', success: true },
          { alertId: 'alert-2', success: true },
        ],
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(results);

      const result = await AlertsAPI.bulkAcknowledge(['alert-1', 'alert-2'], 'user1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/alerts/bulk/acknowledge',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ alertIds: ['alert-1', 'alert-2'], user: 'user1' }),
        })
      );
      expect(result).toEqual(results);
    });
  });
});
