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
      limit: 5,
      would_exceed_limit: false,
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

  it('blocks save when monitored-system usage is temporarily unavailable during preview', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.previewConnection).mockRejectedValueOnce(
      Object.assign(new Error('Unable to verify monitored-system capacity right now'), {
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
    expect(result.monitoredSystemAdmissionSaveBlocked()).toBe(true);
    expect(result.monitoredSystemPreviewErrorTitle()).toBe(
      'Monitored-system capacity is temporarily unavailable',
    );
    expect(result.monitoredSystemPreviewError()).toBe(
      'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    );

    await result.saveCurrentForm();

    expect(VMwareAPI.createConnection).not.toHaveBeenCalled();
    expect(notificationStore.error).toHaveBeenLastCalledWith(
      'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    );
  });

  it('reuses the canonical monitored-system preview when a save is denied by the backend', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.createConnection).mockRejectedValueOnce(
      Object.assign(new Error('Monitored-system limit reached (7/6)'), {
        status: 402,
        feature: 'max_monitored_systems',
        monitored_system_preview: {
          current_count: 5,
          projected_count: 7,
          additional_count: 2,
          limit: 6,
          would_exceed_limit: true,
          effect: 'mixed_existing_and_new',
          current_systems: [
            {
              name: 'esxi-01',
              type: 'agent',
              status: 'online',
              source: 'agent',
            },
          ],
          projected_systems: [
            {
              name: 'esxi-01',
              type: 'agent',
              status: 'online',
              source: 'vmware',
            },
            {
              name: 'esxi-02',
              type: 'agent',
              status: 'online',
              source: 'vmware',
            },
          ],
          current_system: null,
          projected_system: null,
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
    await result.saveCurrentForm();

    expect(notificationStore.error).toHaveBeenCalledWith('Monitored-system limit reached (7/6)');
    expect(result.monitoredSystemPreview()).toMatchObject({
      would_exceed_limit: true,
      projected_count: 7,
      effect: 'mixed_existing_and_new',
    });
    expect(result.dialogOpen()).toBe(true);
  });

  it('surfaces monitored-system usage unavailability when save is rejected before preview settles', async () => {
    vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(VMwareAPI.createConnection).mockRejectedValueOnce(
      Object.assign(new Error('Unable to verify monitored-system capacity right now'), {
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
    await result.saveCurrentForm();

    expect(result.connectionFailure()).toBeNull();
    expect(result.monitoredSystemPreview()).toBeNull();
    expect(result.monitoredSystemAdmissionSaveBlocked()).toBe(true);
    expect(result.monitoredSystemPreviewError()).toBe(
      'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view, so this connection cannot be saved yet. Retry preview in a moment.',
    );
    expect(notificationStore.error).toHaveBeenCalledWith(
      'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view, so this connection cannot be saved yet. Retry preview in a moment.',
    );
  });
});
