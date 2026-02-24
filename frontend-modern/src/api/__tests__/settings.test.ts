import { describe, expect, it, vi, beforeEach } from 'vitest';
import { SettingsAPI, type SystemSettingsResponse } from '../settings';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('SettingsAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('updateSystemSettings', () => {
    it('updates system settings', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await SettingsAPI.updateSystemSettings({ fullWidthMode: true });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/system/settings/update',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ fullWidthMode: true }),
        }),
      );
    });
  });

  describe('getSystemSettings', () => {
    it('fetches system settings', async () => {
      const mockSettings: SystemSettingsResponse = {
        autoUpdateEnabled: true,
        fullWidthMode: true,
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockSettings);

      const result = await SettingsAPI.getSystemSettings();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/system/settings');
      expect(result).toEqual(mockSettings);
    });
  });
});
