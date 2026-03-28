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

  describe('getTelemetryPreview', () => {
    it('fetches the telemetry preview payload', async () => {
      const mockPreview = {
        enabled: true,
        payload: {
          install_id: 'preview-install-id',
          version: '6.0.0',
          platform: 'docker',
          os: 'linux',
          arch: 'amd64',
          event: 'heartbeat',
          pve_nodes: 1,
          pbs_instances: 0,
          pmg_instances: 0,
          vms: 2,
          containers: 3,
          docker_hosts: 0,
          kubernetes_clusters: 0,
          ai_enabled: false,
          active_alerts: 0,
          relay_enabled: false,
          sso_enabled: false,
          multi_tenant: false,
          paid_license: false,
          has_api_tokens: true,
        },
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockPreview);

      const result = await SettingsAPI.getTelemetryPreview();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/system/settings/telemetry-preview');
      expect(result).toEqual(mockPreview);
    });
  });

  describe('resetTelemetryInstallID', () => {
    it('posts the telemetry install-id reset action', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        enabled: true,
        payload: {
          install_id: 'rotated-install-id',
        },
      });

      await SettingsAPI.resetTelemetryInstallID();

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/system/settings/telemetry-reset-id',
        expect.objectContaining({
          method: 'POST',
        }),
      );
    });
  });
});
