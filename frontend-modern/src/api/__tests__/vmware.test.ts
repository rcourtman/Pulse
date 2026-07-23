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
          networks: 8,
          viRelease: ' 8.0.3 ',
          degraded: true,
          issueCount: 3,
          issues: [
            {
              stage: ' signals ',
              category: ' permission ',
              message: ' VMware permissions are insufficient for host overall status ',
              occurrences: 2,
            },
          ],
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
          networks: 8,
          viRelease: '8.0.3',
          degraded: true,
          issueCount: 3,
          issues: [
            {
              stage: 'signals',
              category: 'permission',
              message: 'VMware permissions are insufficient for host overall status',
              occurrences: 2,
            },
          ],
        },
        monitorVms: false,
        monitorHosts: false,
        monitorDatastores: false,
      },
    ]);
  });

  it('round-trips per-surface monitor* flags through list and update payloads', async () => {
    vi.mocked(apiFetchJSON)
      .mockResolvedValueOnce([
        {
          id: 'conn-1',
          name: 'lab-vcenter',
          host: 'vcsa.lab.local',
          insecureSkipVerify: false,
          enabled: true,
          monitorVms: true,
          monitorHosts: false,
          monitorDatastores: true,
        },
      ])
      .mockResolvedValueOnce({
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        insecureSkipVerify: false,
        enabled: true,
        monitorVms: false,
        monitorHosts: true,
        monitorDatastores: false,
      });

    const connections = await VMwareAPI.listConnections();
    expect(connections[0]).toMatchObject({
      monitorVms: true,
      monitorHosts: false,
      monitorDatastores: true,
    });

    await VMwareAPI.updateConnection('conn-1', {
      host: 'vcsa.lab.local',
      enabled: true,
      monitorVms: false,
      monitorHosts: true,
      monitorDatastores: false,
    });
    expect(apiFetchJSON).toHaveBeenLastCalledWith(
      '/api/vmware/connections/conn-1',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          host: 'vcsa.lab.local',
          enabled: true,
          monitorVms: false,
          monitorHosts: true,
          monitorDatastores: false,
        }),
      }),
    );
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
    ).resolves.toEqual({
      success: true,
      hosts: 0,
      vms: 0,
      datastores: 0,
      networks: 0,
      viRelease: undefined,
      degraded: false,
      issueCount: 0,
      issues: [],
    });
    await expect(
      VMwareAPI.testSavedConnection('conn/1', {
        host: 'vcsa.lab.local',
        username: 'operator@vsphere.local',
        password: '********',
        insecureSkipVerify: true,
      }),
    ).resolves.toEqual({
      success: true,
      hosts: 0,
      vms: 0,
      datastores: 0,
      networks: 0,
      viRelease: undefined,
      degraded: false,
      issueCount: 0,
      issues: [],
    });

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

  it('preserves degraded connection-test diagnostics', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      success: true,
      hosts: 2,
      vms: 14,
      datastores: 3,
      networks: 5,
      viRelease: ' 8.0.2.0 ',
      degraded: true,
      issueCount: 1,
      issues: [
        {
          stage: ' signals ',
          entity_type: ' host ',
          entity_id: ' host-101 ',
          category: ' endpoint ',
          message: ' VMware HostSystem recent events request failed with HTTP 500 ',
        },
      ],
    });

    await expect(
      VMwareAPI.testSavedConnection('conn-1', {
        host: 'vcsa.lab.local',
        username: 'administrator@vsphere.local',
        password: '********',
      }),
    ).resolves.toEqual({
      success: true,
      hosts: 2,
      vms: 14,
      datastores: 3,
      networks: 5,
      viRelease: '8.0.2.0',
      degraded: true,
      issueCount: 1,
      issues: [
        {
          stage: 'signals',
          entityType: 'host',
          entityId: 'host-101',
          category: 'endpoint',
          message: 'VMware HostSystem recent events request failed with HTTP 500',
        },
      ],
    });
  });

  it('recognizes the masked-secret placeholder used for update preservation', () => {
    expect(isRedactedVMwareSecret('********')).toBe(true);
    expect(isRedactedVMwareSecret(' ******** ')).toBe(true);
    expect(isRedactedVMwareSecret('secret')).toBe(false);
    expect(isRedactedVMwareSecret(undefined)).toBe(false);
  });
});
