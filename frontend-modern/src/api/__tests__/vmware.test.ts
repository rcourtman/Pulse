import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { VMwareAPI, isRedactedVMwareSecret } from '@/api/vmware';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('VMwareAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('normalizes listed connections from the API contract', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce([
      {
        id: ' conn-1 ',
        name: ' lab-vcenter ',
        host: ' vcsa.lab.local ',
        port: 443,
        username: ' administrator@vsphere.local ',
        password: ' ******** ',
        insecureSkipVerify: true,
        enabled: true,
        poll: {
          intervalSeconds: 60,
          lastAttemptAt: ' 2026-03-30T12:00:00Z ',
          lastSuccessAt: ' 2026-03-30T12:00:01Z ',
          consecutiveFailures: 0,
        },
        observed: {
          collectedAt: ' 2026-03-30T12:00:02Z ',
          hosts: 3,
          vms: 42,
          datastores: 6,
          viRelease: ' 8.0.3 ',
        },
      },
    ]);

    await expect(VMwareAPI.listConnections()).resolves.toEqual([
      {
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: true,
        enabled: true,
        poll: {
          intervalSeconds: 60,
          lastAttemptAt: '2026-03-30T12:00:00Z',
          lastSuccessAt: '2026-03-30T12:00:01Z',
          consecutiveFailures: 0,
          lastError: undefined,
        },
        observed: {
          collectedAt: '2026-03-30T12:00:02Z',
          hosts: 3,
          vms: 42,
          datastores: 6,
          viRelease: '8.0.3',
        },
      },
    ]);
  });

  it('creates, updates, deletes, and tests connections through canonical routes', async () => {
    vi.mocked(apiFetchJSON)
      .mockResolvedValueOnce({
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        insecureSkipVerify: false,
        enabled: true,
      })
      .mockResolvedValueOnce({
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        insecureSkipVerify: true,
        enabled: false,
      })
      .mockResolvedValueOnce({ success: true, id: 'conn-1' })
      .mockResolvedValueOnce({ success: true })
      .mockResolvedValueOnce({ success: true });

    await VMwareAPI.createConnection({
      name: 'lab-vcenter',
      host: 'vcsa.lab.local',
      port: 443,
      username: 'administrator@vsphere.local',
      password: 'secret',
      enabled: true,
    });
    await VMwareAPI.updateConnection('conn/1', {
      host: 'vcsa.lab.local',
      port: 8443,
      username: 'operator@vsphere.local',
      password: '********',
      insecureSkipVerify: true,
      enabled: false,
    });
    await VMwareAPI.deleteConnection('conn/1');
    await expect(
      VMwareAPI.testConnection({
        host: 'vcsa.lab.local',
        username: 'administrator@vsphere.local',
        password: 'secret',
      }),
    ).resolves.toEqual({ success: true });
    await expect(
      VMwareAPI.testSavedConnection('conn/1', {
        host: 'vcsa.lab.local',
        username: 'operator@vsphere.local',
        password: '********',
        insecureSkipVerify: true,
      }),
    ).resolves.toEqual({ success: true });

    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      1,
      '/api/vmware/connections',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          name: 'lab-vcenter',
          host: 'vcsa.lab.local',
          port: 443,
          username: 'administrator@vsphere.local',
          password: 'secret',
          enabled: true,
        }),
      }),
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      2,
      '/api/vmware/connections/conn%2F1',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          host: 'vcsa.lab.local',
          port: 8443,
          username: 'operator@vsphere.local',
          password: '********',
          insecureSkipVerify: true,
          enabled: false,
        }),
      }),
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      3,
      '/api/vmware/connections/conn%2F1',
      expect.objectContaining({ method: 'DELETE' }),
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      4,
      '/api/vmware/connections/test',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          host: 'vcsa.lab.local',
          username: 'administrator@vsphere.local',
          password: 'secret',
        }),
      }),
    );
    expect(apiFetchJSON).toHaveBeenNthCalledWith(
      5,
      '/api/vmware/connections/conn%2F1/test',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          host: 'vcsa.lab.local',
          username: 'operator@vsphere.local',
          password: '********',
          insecureSkipVerify: true,
        }),
      }),
    );
  });

  it('recognizes the masked-secret placeholder used for update preservation', () => {
    expect(isRedactedVMwareSecret('********')).toBe(true);
    expect(isRedactedVMwareSecret(' ******** ')).toBe(true);
    expect(isRedactedVMwareSecret('secret')).toBe(false);
    expect(isRedactedVMwareSecret(undefined)).toBe(false);
  });
});
