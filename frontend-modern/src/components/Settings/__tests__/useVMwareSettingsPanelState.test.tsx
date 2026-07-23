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
    previewConnection: vi.fn(),
    previewSavedConnection: vi.fn(),
    testConnection: vi.fn(),
    testSavedConnection: vi.fn(),
  },
  isRedactedVMwareSecret: (value: string | null | undefined) => (value || '').trim() === '********',
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

const safeVMwarePreview = () => ({
  current_count: 1,
  projected_count: 1,
  additional_count: 0,
  effect: 'attaches_existing',
  current_systems: [],
  projected_systems: [
    {
      name: 'esxi-01',
      type: 'host',
      status: 'online',
      source: 'vmware',
      status_explanation: { summary: '', reasons: [] },
      latest_included_signal: { name: '', type: '', at: '' },
      explanation: { summary: '', reasons: [], surfaces: [] },
    },
  ],
  current_system: null,
  projected_system: null,
});

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
    vi.mocked(VMwareAPI.previewSavedConnection).mockResolvedValueOnce(safeVMwarePreview() as never);
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
    await result.previewCurrentForm();
    await result.saveCurrentForm();

    expect(VMwareAPI.previewSavedConnection).toHaveBeenCalledWith(
      'conn-1',
      expect.objectContaining({
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
      }),
    );
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

  it('reports optional VMware test failures as a degraded success with diagnostics', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'lab-vcenter',
        host: 'vcsa.lab.local',
        port: 443,
        username: 'administrator@vsphere.local',
        password: '********',
        insecureSkipVerify: true,
        enabled: true,
      },
    ] as never);
    vi.mocked(VMwareAPI.testSavedConnection).mockResolvedValueOnce({
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

    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await waitFor(() => expect(result.connections()).toHaveLength(1));

    result.openEditDialog(result.connections()[0]);
    const succeeded = await result.testCurrentForm();

    expect(succeeded).toBe(true);
    expect(notificationStore.warning).toHaveBeenCalledWith(
      'VMware HostSystem recent events request failed with HTTP 500',
    );
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(result.connectionFailure()).toBeNull();
    expect(result.connectionTestWarning()).toEqual({
      title: 'VMware connection has limited data access',
      message: 'VMware HostSystem recent events request failed with HTTP 500',
      guidance:
        'Core inventory, authentication, and API compatibility checks succeeded using VI JSON 8.0.2.0. Pulse will keep monitoring and report affected optional data as degraded.',
    });
  });

  it('saves a connection without requiring a monitored-system preview', async () => {
    vi.mocked(VMwareAPI.listConnections)
      .mockResolvedValueOnce([] as never)
      .mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.createConnection).mockResolvedValueOnce({} as never);

    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'vcsa.lab.local',
      username: 'administrator@vsphere.local',
      password: 'secret',
    });

    await result.saveCurrentForm();

    expect(VMwareAPI.createConnection).toHaveBeenCalledWith(
      expect.objectContaining({
        host: 'vcsa.lab.local',
        username: 'administrator@vsphere.local',
        password: 'secret',
      }),
    );
    expect(notificationStore.success).toHaveBeenCalledWith('VMware connection added');
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
        'Pulse supports vCenter 8.0U1 and newer. Upgrade vCenter, then retry this connection test.',
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

  it('surfaces structured saved-connection failures and refreshes runtime state', async () => {
    const authError = 'VMware authentication failed while creating the VI JSON API session';
    vi.mocked(VMwareAPI.listConnections)
      .mockResolvedValueOnce([
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
      ] as never)
      .mockResolvedValueOnce([
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
            lastAttemptAt: '2026-03-31T12:00:00Z',
            lastError: {
              at: '2026-03-31T12:00:00Z',
              category: 'auth',
              message: authError,
            },
          },
        },
      ] as never);
    vi.mocked(VMwareAPI.testSavedConnection).mockRejectedValueOnce(
      Object.assign(new Error('Failed to connect to VMware vCenter'), {
        status: 400,
        code: 'vmware_connection_failed',
        details: {
          category: 'auth',
          error: authError,
        },
      }),
    );

    const { result } = renderHook(() => useVMwareSettingsPanelState());

    await waitFor(() => expect(result.connections()).toHaveLength(1));

    await result.testSavedConnection(result.connections()[0]);

    expect(notificationStore.error).toHaveBeenCalledWith(authError);
    expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(2);
    expect(result.connections()[0].poll?.lastError?.category).toBe('auth');
    expect(result.connections()[0].poll?.lastError?.message).toBe(authError);
  });

  it('previews monitored-system impact through the canonical VMware preview path', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.previewConnection).mockResolvedValueOnce({
      current_count: 1,
      projected_count: 3,
      additional_count: 2,
      effect: 'creates_multiple',
      current_systems: [],
      projected_systems: [
        {
          name: 'esxi-01',
          type: 'host',
          status: 'online',
          source: 'vmware',
          status_explanation: { summary: '', reasons: [] },
          latest_included_signal: { name: '', type: '', at: '' },
          explanation: { summary: '', reasons: [], surfaces: [] },
        },
      ],
      current_system: null,
      projected_system: null,
    } as never);

    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'vcsa.lab.local',
      username: 'administrator@vsphere.local',
      password: 'secret',
    });
    const preview = await result.previewCurrentForm();

    expect(VMwareAPI.previewConnection).toHaveBeenCalledWith(
      expect.objectContaining({
        host: 'vcsa.lab.local',
        username: 'administrator@vsphere.local',
        password: 'secret',
      }),
    );
    expect(preview?.additional_count).toBe(2);
    expect(result.monitoredSystemPreview()?.projected_count).toBe(3);
  });

  it('allows save when monitored-system impact preview is temporarily unavailable', async () => {
    vi.mocked(VMwareAPI.listConnections)
      .mockResolvedValueOnce([] as never)
      .mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.createConnection).mockResolvedValueOnce({} as never);
    vi.mocked(VMwareAPI.previewConnection).mockRejectedValueOnce(
      Object.assign(new Error('Unable to verify monitored-system grouping right now'), {
        status: 503,
        code: 'monitored_system_usage_unavailable',
        details: {
          reason: 'supplemental_inventory_unsettled',
        },
      }),
    );

    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'vcsa.lab.local',
      username: 'administrator@vsphere.local',
      password: 'secret',
    });

    const preview = await result.previewCurrentForm();

    expect(preview).toBeNull();
    expect(result.connectionFailure()).toBeNull();
    expect(result.monitoredSystemPreview()).toBeNull();
    expect(result.monitoredSystemPreviewErrorTitle()).toBe(
      'Monitored-system verification is temporarily unavailable',
    );
    expect(result.monitoredSystemPreviewError()).toBe(
      'Pulse is still settling provider-owned inventory for this platform connection. You can still save the connection and review the impact after the first baseline finishes.',
    );

    await result.saveCurrentForm();

    expect(VMwareAPI.createConnection).toHaveBeenCalled();
    expect(notificationStore.success).toHaveBeenCalledWith('VMware connection added');
  });

  it('surfaces backend save errors without reopening retired cap-preview handling', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.previewConnection).mockResolvedValueOnce(safeVMwarePreview() as never);
    vi.mocked(VMwareAPI.createConnection).mockRejectedValueOnce(
      Object.assign(new Error('VMware connection save failed'), {
        status: 500,
      }),
    );

    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'vcsa.lab.local',
      username: 'administrator@vsphere.local',
      password: 'secret',
    });
    await result.previewCurrentForm();
    await result.saveCurrentForm();

    expect(notificationStore.error).toHaveBeenCalledWith('VMware connection save failed');
    expect(result.monitoredSystemPreview()).toMatchObject({
      projected_count: 1,
      effect: 'attaches_existing',
    });
    expect(result.dialogOpen()).toBe(true);
  });

  it('treats save-time monitored-system usage unavailability as an ordinary save error', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.previewConnection).mockResolvedValueOnce(safeVMwarePreview() as never);
    vi.mocked(VMwareAPI.createConnection).mockRejectedValueOnce(
      Object.assign(new Error('Unable to verify monitored-system grouping right now'), {
        status: 503,
        code: 'monitored_system_usage_unavailable',
        details: {
          reason: 'supplemental_inventory_rebuild_pending',
        },
      }),
    );

    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'vcsa.lab.local',
      username: 'administrator@vsphere.local',
      password: 'secret',
    });
    await result.previewCurrentForm();
    await result.saveCurrentForm();

    expect(result.connectionFailure()).toBeNull();
    expect(result.monitoredSystemPreview()).toMatchObject({
      projected_count: 1,
      effect: 'attaches_existing',
    });
    expect(result.monitoredSystemPreviewError()).toBeNull();
    expect(notificationStore.error).toHaveBeenCalledWith(
      'Unable to verify monitored-system grouping right now',
    );
  });
});
