import { renderHook, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { VMwareAPI } from '@/api/vmware';
import { notificationStore } from '@/stores/notifications';
import { useVMwareSettingsPanelState } from '../useVMwareSettingsPanelState';

vi.mock('@/api/vmware', () => ({
  VMwareAPI: {
    listConnections: vi.fn(),
    createConnection: vi.fn(),
    updateConnection: vi.fn(),
    deleteConnection: vi.fn(),
    testConnection: vi.fn(),
    testSavedConnection: vi.fn(),
  },
  isRedactedVMwareSecret: (value: string | null | undefined) => (value || '').trim() === '********',
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

describe('useVMwareSettingsPanelState', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('treats a 404 list response as a feature-disabled integration state', async () => {
    vi.mocked(VMwareAPI.listConnections).mockRejectedValueOnce({
      status: 404,
      message: 'VMware integration has been explicitly disabled',
    });

    const { result } = renderHook(() => useVMwareSettingsPanelState());

    await waitFor(() => expect(result.featureDisabled()).toBe(true));
    expect(result.featureDisabledMessage()).toBe('VMware integration has been explicitly disabled');
    expect(result.connections()).toEqual([]);
  });

  it('preserves the masked password when editing an existing connection without replacing the secret', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: false,
        enabled: true,
      },
    ] as never);
    vi.mocked(VMwareAPI.updateConnection).mockResolvedValueOnce({
      id: 'conn-1',
      name: 'lab-vcenter',
      host: 'vcsa.lab.local',
      port: 443,
      username: 'administrator@vsphere.local',
      password: '********',
      insecureSkipVerify: false,
      enabled: true,
    } as never);
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: false,
        enabled: true,
      },
    ] as never);

    const { result } = renderHook(() => useVMwareSettingsPanelState());

    await waitFor(() => expect(result.connections()).toHaveLength(1));

    result.openEditDialog(result.connections()[0]);
    expect(result.dialogOpen()).toBe(true);
    await result.saveCurrentForm();

    expect(VMwareAPI.updateConnection).toHaveBeenCalledWith(
      'conn-1',
      expect.objectContaining({
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: false,
        enabled: true,
      }),
    );
    expect(result.dialogOpen()).toBe(false);
    expect(notificationStore.success).toHaveBeenCalledWith('VMware connection updated');
  });

  it('tests saved connections through the canonical saved-connection API path', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: false,
        enabled: true,
      },
    ] as never);
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: false,
        enabled: true,
        poll: {
          lastSuccessAt: '2026-03-30T10:00:00Z',
        },
      },
    ] as never);
    vi.mocked(VMwareAPI.testSavedConnection).mockResolvedValueOnce({ success: true } as never);

    const { result } = renderHook(() => useVMwareSettingsPanelState());

    await waitFor(() => expect(result.connections()).toHaveLength(1));

    await result.testSavedConnection(result.connections()[0]);

    expect(VMwareAPI.testSavedConnection).toHaveBeenCalledWith('conn-1');
    expect(VMwareAPI.testConnection).not.toHaveBeenCalled();
    expect(notificationStore.success).toHaveBeenCalledWith(
      'VMware connection successful for lab-vcenter',
    );
    expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(2);
    expect(result.connections()[0].poll?.lastSuccessAt).toBe('2026-03-30T10:00:00Z');
  });

  it('tests edited saved connections through the canonical saved-connection API path', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: false,
        enabled: true,
      },
    ] as never);
    vi.mocked(VMwareAPI.testSavedConnection).mockResolvedValueOnce({ success: true } as never);

    const { result } = renderHook(() => useVMwareSettingsPanelState());

    await waitFor(() => expect(result.connections()).toHaveLength(1));

    result.openEditDialog(result.connections()[0]);
    result.updateForm({ host: 'edited.lab.local', port: '8443', insecureSkipVerify: true });
    await result.testCurrentForm();

    expect(VMwareAPI.testSavedConnection).toHaveBeenCalledWith(
      'conn-1',
      expect.objectContaining({
        host: 'edited.lab.local',
        port: 8443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: true,
      }),
    );
    expect(VMwareAPI.testConnection).not.toHaveBeenCalled();
    expect(notificationStore.success).toHaveBeenCalledWith('VMware connection successful');
  });

  it('surfaces categorized draft test guidance from structured backend failures', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.testConnection).mockRejectedValueOnce(
      Object.assign(new Error('Failed to connect to VMware vCenter'), {
        status: 400,
        code: 'vmware_connection_failed',
        details: {
          category: 'unsupported_version',
          error: 'VMware vCenter 6.7 is below the supported VI JSON release floor',
        },
      }),
    );

    const { result } = renderHook(() => useVMwareSettingsPanelState());

    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'legacy.lab.local',
      username: 'administrator@vsphere.local',
      password: 'secret',
    });
    const succeeded = await result.testCurrentForm();

    expect(succeeded).toBe(false);
    expect(notificationStore.error).toHaveBeenCalledWith(
      'VMware vCenter 6.7 is below the supported VI JSON release floor',
    );
    expect(result.connectionFailure()).toEqual({
      category: 'unsupported_version',
      code: 'vmware_connection_failed',
      guidance:
        'Use a supported vCenter release within the current VI JSON phase-1 floor, then retry this connection test.',
      message: 'VMware vCenter 6.7 is below the supported VI JSON release floor',
      title: 'Unsupported vCenter version',
      tone: 'warning',
    });
  });

  it.each([
    {
      category: 'auth',
      error: 'VMware authentication failed while creating the VI JSON API session',
      expected: {
        guidance: 'Verify the username, password, and account scope in vCenter before retrying.',
        title: 'Authentication failed',
        tone: 'danger',
      },
    },
    {
      category: 'tls',
      error: 'VMware TLS validation failed during Automation API session bootstrap',
      expected: {
        guidance:
          'Install a trusted certificate for vCenter, or enable Skip TLS verification only for controlled lab environments.',
        title: 'TLS validation failed',
        tone: 'warning',
      },
    },
    {
      category: 'network',
      error: 'VMware network error during VI JSON login',
      expected: {
        guidance:
          'Confirm DNS, reachability, port 443, and any firewall rules from the Pulse server to vCenter.',
        title: 'Pulse could not reach vCenter',
        tone: 'danger',
      },
    },
  ])(
    'maps $category failures onto shared draft onboarding guidance',
    async ({ category, error, expected }) => {
      vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([] as never);
      vi.mocked(VMwareAPI.testConnection).mockRejectedValueOnce(
        Object.assign(new Error('Failed to connect to VMware vCenter'), {
          status: 400,
          code: 'vmware_connection_failed',
          details: {
            category,
            error,
          },
        }),
      );

      const { result } = renderHook(() => useVMwareSettingsPanelState());

      await waitFor(() => expect(result.loading()).toBe(false));

      result.openCreateDialog();
      result.updateForm({
        host: `${category}.lab.local`,
        username: 'administrator@vsphere.local',
        password: 'secret',
      });
      const succeeded = await result.testCurrentForm();

      expect(succeeded).toBe(false);
      expect(notificationStore.error).toHaveBeenCalledWith(error);
      expect(result.connectionFailure()).toEqual({
        category,
        code: 'vmware_connection_failed',
        guidance: expected.guidance,
        message: error,
        title: expected.title,
        tone: expected.tone,
      });
    },
  );
});
