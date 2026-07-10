import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { UpdatesAPI } from '@/api/updates';
import type { UpdateChannel } from '@/types/config';
import { apiFetchJSON } from '@/utils/apiClient';

describe('UpdatesAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('preserves the app version and agent update target from version payloads', async () => {
    const response = {
      version: '6.0.0-rc.6+git.174.g259476907.dirty',
      build: 'development',
      runtime: 'go',
      isDocker: false,
      isSourceBuild: false,
      isDevelopment: true,
      deploymentType: 'development',
      agentUpdateTargetVersion: '6.0.0-rc.6',
    };
    apiFetchJSONMock.mockResolvedValueOnce(response as any);

    await expect(UpdatesAPI.getVersion()).resolves.toEqual(response);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/version');
  });

  it('encodes optional update-check channel safely', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ available: false } as any);
    await UpdatesAPI.checkForUpdates('rc');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/updates/check?channel=rc');
  });

  it('omits blank channel for update checks', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ available: false } as any);
    await UpdatesAPI.checkForUpdates('   ' as UpdateChannel);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/updates/check');
  });

  it('rejects empty apply-update download URL before making a request', async () => {
    await expect(UpdatesAPI.applyUpdate('   ')).rejects.toThrow('Download URL is required');
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
  });

  it('trims download URL before apply-update request', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ status: 'started', message: 'ok' } as any);
    await UpdatesAPI.applyUpdate('  https://example.com/pulse.tar.gz  ');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/updates/apply', {
      method: 'POST',
      body: JSON.stringify({ downloadUrl: 'https://example.com/pulse.tar.gz' }),
    });
  });

  it('rejects empty rollback event ID before making a request', async () => {
    await expect(UpdatesAPI.rollbackUpdate('   ')).rejects.toThrow('Event ID is required');
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
  });

  it('trims event ID before rollback request', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ status: 'started', message: 'ok' } as any);
    await UpdatesAPI.rollbackUpdate('  01JZEXAMPLE  ');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/updates/rollback', {
      method: 'POST',
      body: JSON.stringify({ eventId: '01JZEXAMPLE' }),
    });
  });

  it('encodes the update-history limit', async () => {
    apiFetchJSONMock.mockResolvedValueOnce([] as any);
    await UpdatesAPI.listUpdateHistory(5);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/updates/history?limit=5');
  });

  it('encodes update-plan version and channel safely', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ canAutoUpdate: true } as any);
    await UpdatesAPI.getUpdatePlan('v1.2.3-rc.1+build', 'rc');

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/updates/plan?version=v1.2.3-rc.1%2Bbuild&channel=rc',
    );
  });

  it('preserves update-plan readiness payloads from the backend', async () => {
    const response = {
      canAutoUpdate: true,
      requiresRoot: true,
      rollbackSupport: true,
      readiness: {
        status: 'blocked',
        summary: 'Resolve 1 blocked upgrade check before installing this update.',
        checks: [
          {
            id: 'agent-token-scopes',
            status: 'blocked',
            title: 'Agent token scopes',
            summary: 'Registered agents exist, but no loaded API token grants agent reporting scope.',
          },
        ],
      },
    };
    apiFetchJSONMock.mockResolvedValueOnce(response as any);

    await expect(UpdatesAPI.getUpdatePlan('v6.0.0')).resolves.toEqual(response);
  });

  it('rejects blank update-plan version before making a request', async () => {
    await expect(UpdatesAPI.getUpdatePlan('   ', 'stable')).rejects.toThrow('Version is required');
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
  });
});
