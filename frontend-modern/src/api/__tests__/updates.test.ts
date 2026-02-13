import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { UpdatesAPI } from '@/api/updates';
import { apiFetchJSON } from '@/utils/apiClient';

describe('UpdatesAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('encodes optional update-check channel safely', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ available: false } as any);
    await UpdatesAPI.checkForUpdates('beta channel/rc+1');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/updates/check?channel=beta+channel%2Frc%2B1');
  });

  it('omits blank channel for update checks', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ available: false } as any);
    await UpdatesAPI.checkForUpdates('   ');

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

  it('encodes update-plan version and channel safely', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ canAutoUpdate: true } as any);
    await UpdatesAPI.getUpdatePlan('v1.2.3-rc.1+build', 'edge/canary');

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/updates/plan?version=v1.2.3-rc.1%2Bbuild&channel=edge%2Fcanary',
    );
  });

  it('rejects blank update-plan version before making a request', async () => {
    await expect(UpdatesAPI.getUpdatePlan('   ', 'stable')).rejects.toThrow('Version is required');
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
  });
});
