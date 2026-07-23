import { renderHook, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TrueNASAPI } from '@/api/truenas';
import { notificationStore } from '@/stores/notifications';
import { useTrueNASSettingsPanelState } from '../useTrueNASSettingsPanelState';

vi.mock('@/api/truenas', () => ({
  TrueNASAPI: {
    listConnections: vi.fn(),
    createConnection: vi.fn(),
    updateConnection: vi.fn(),
    deleteConnection: vi.fn(),
    previewConnection: vi.fn(),
    previewSavedConnection: vi.fn(),
    testConnection: vi.fn(),
    testSavedConnection: vi.fn(),
  },
  isRedactedTrueNASSecret: (value: string | null | undefined) =>
    (value || '').trim() === '********',
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

const safeTrueNASPreview = () => ({
  current_count: 1,
  projected_count: 1,
  additional_count: 0,
  effect: 'attaches_existing',
  current_systems: [],
  projected_systems: [
    {
      name: 'tower',
      type: 'truenas-system',
      status: 'online',
      source: 'truenas',
      status_explanation: { summary: '', reasons: [] },
      latest_included_signal: { name: '', type: '', at: '' },
      explanation: { summary: '', reasons: [], surfaces: [] },
    },
  ],
  current_system: null,
  projected_system: null,
});

describe('useTrueNASSettingsPanelState', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('treats a 404 list response as a feature-disabled integration state', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockRejectedValueOnce({
      status: 404,
      message: 'TrueNAS integration has been explicitly disabled',
    });

    const { result } = renderHook(() => useTrueNASSettingsPanelState());

    await waitFor(() => expect(result.featureDisabled()).toBe(true));
    expect(result.featureDisabledMessage()).toBe(
      'TrueNAS integration has been explicitly disabled',
    );
    expect(result.connections()).toEqual([]);
  });

  it('preserves masked API keys when editing an existing connection without replacing the secret', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        apiKey: '********',
        username: 'pulse-readonly',
        pollIntervalSeconds: 90,
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
      },
    ] as never);
    vi.mocked(TrueNASAPI.updateConnection).mockResolvedValueOnce({
      id: 'conn-1',
      name: 'tower',
      host: 'truenas.local',
      apiKey: '********',
      username: 'pulse-readonly',
      pollIntervalSeconds: 90,
      useHttps: true,
      insecureSkipVerify: false,
      enabled: true,
    } as never);
    vi.mocked(TrueNASAPI.previewSavedConnection).mockResolvedValueOnce(
      safeTrueNASPreview() as never,
    );
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        apiKey: '********',
        username: 'pulse-readonly',
        pollIntervalSeconds: 90,
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
      },
    ] as never);

    const { result } = renderHook(() => useTrueNASSettingsPanelState());

    await waitFor(() => expect(result.connections()).toHaveLength(1));

    result.openEditDialog(result.connections()[0]);
    expect(result.dialogOpen()).toBe(true);
    expect(result.form().pollIntervalSeconds).toBe('90');
    await result.previewCurrentForm();
    await result.saveCurrentForm();

    expect(TrueNASAPI.previewSavedConnection).toHaveBeenCalledWith(
      'conn-1',
      expect.objectContaining({
        host: 'truenas.local',
        apiKey: '********',
        pollIntervalSeconds: 90,
        username: 'pulse-readonly',
      }),
    );
    expect(TrueNASAPI.updateConnection).toHaveBeenCalledWith(
      'conn-1',
      expect.objectContaining({
        host: 'truenas.local',
        apiKey: '********',
        pollIntervalSeconds: 90,
        username: 'pulse-readonly',
        password: '',
      }),
    );
    expect(result.dialogOpen()).toBe(false);
    expect(notificationStore.success).toHaveBeenCalledWith('TrueNAS connection updated');
  });

  it('tests saved connections through the canonical saved-connection API path', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        apiKey: '********',
        pollIntervalSeconds: 60,
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
      },
    ] as never);
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        apiKey: '********',
        pollIntervalSeconds: 60,
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
        poll: {
          intervalSeconds: 60,
          lastSuccessAt: '2026-03-30T10:00:00Z',
        },
      },
    ] as never);
    vi.mocked(TrueNASAPI.testSavedConnection).mockResolvedValueOnce({ success: true } as never);

    const { result } = renderHook(() => useTrueNASSettingsPanelState());

    await waitFor(() => expect(result.connections()).toHaveLength(1));

    await result.testSavedConnection(result.connections()[0]);

    expect(TrueNASAPI.testSavedConnection).toHaveBeenCalledWith('conn-1');
    expect(TrueNASAPI.testConnection).not.toHaveBeenCalled();
    expect(notificationStore.success).toHaveBeenCalledWith(
      'TrueNAS connection successful for tower',
    );
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(2);
    expect(result.connections()[0].poll?.lastSuccessAt).toBe('2026-03-30T10:00:00Z');
  });

  it('tests edited saved connections through the canonical saved-connection API path', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        apiKey: '********',
        pollIntervalSeconds: 60,
        useHttps: true,
        insecureSkipVerify: false,
        enabled: true,
      },
    ] as never);
    vi.mocked(TrueNASAPI.testSavedConnection).mockResolvedValueOnce({ success: true } as never);

    const { result } = renderHook(() => useTrueNASSettingsPanelState());

    await waitFor(() => expect(result.connections()).toHaveLength(1));

    result.openEditDialog(result.connections()[0]);
    result.updateForm({ host: 'tower-edited.local', pollIntervalSeconds: '120' });
    await result.testCurrentForm();

    expect(TrueNASAPI.testSavedConnection).toHaveBeenCalledWith(
      'conn-1',
      expect.objectContaining({
        host: 'tower-edited.local',
        apiKey: '********',
        pollIntervalSeconds: 120,
      }),
    );
    expect(TrueNASAPI.testConnection).not.toHaveBeenCalled();
    expect(notificationStore.success).toHaveBeenCalledWith('TrueNAS connection successful');
  });

  it('saves a connection without requiring a monitored-system preview', async () => {
    vi.mocked(TrueNASAPI.listConnections)
      .mockResolvedValueOnce([] as never)
      .mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.createConnection).mockResolvedValueOnce({} as never);

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'tower.local',
      apiKey: 'secret',
    });

    await result.saveCurrentForm();

    expect(TrueNASAPI.createConnection).toHaveBeenCalledWith(
      expect.objectContaining({
        host: 'tower.local',
        apiKey: 'secret',
      }),
    );
    expect(notificationStore.success).toHaveBeenCalledWith('TrueNAS connection added');
  });

  it('previews monitored-system impact through the canonical TrueNAS preview path', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.previewConnection).mockResolvedValueOnce({
      current_count: 4,
      projected_count: 4,
      additional_count: 0,
      effect: 'attaches_existing',
      current_systems: [
        {
          name: 'tower',
          type: 'truenas-system',
          status: 'online',
          source: 'agent',
          status_explanation: { summary: '', reasons: [] },
          latest_included_signal: { name: '', type: '', at: '' },
          explanation: { summary: '', reasons: [], surfaces: [] },
        },
      ],
      projected_systems: [
        {
          name: 'tower',
          type: 'truenas-system',
          status: 'online',
          source: 'multiple',
          status_explanation: { summary: '', reasons: [] },
          latest_included_signal: { name: '', type: '', at: '' },
          explanation: { summary: '', reasons: [], surfaces: [] },
        },
      ],
      current_system: null,
      projected_system: null,
    } as never);

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'tower.local',
      apiKey: 'secret',
    });
    const preview = await result.previewCurrentForm();

    expect(TrueNASAPI.previewConnection).toHaveBeenCalledWith(
      expect.objectContaining({
        host: 'tower.local',
        apiKey: 'secret',
      }),
    );
    expect(preview?.additional_count).toBe(0);
    expect(result.monitoredSystemPreview()?.effect).toBe('attaches_existing');
  });

  it('allows save when monitored-system impact preview is temporarily unavailable', async () => {
    vi.mocked(TrueNASAPI.listConnections)
      .mockResolvedValueOnce([] as never)
      .mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.createConnection).mockResolvedValueOnce({} as never);
    vi.mocked(TrueNASAPI.previewConnection).mockRejectedValueOnce(
      Object.assign(new Error('Unable to verify monitored-system grouping right now'), {
        status: 503,
        code: 'monitored_system_usage_unavailable',
        details: {
          reason: 'supplemental_inventory_unsettled',
        },
      }),
    );

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'tower.local',
      apiKey: 'secret',
    });

    const preview = await result.previewCurrentForm();

    expect(preview).toBeNull();
    expect(result.monitoredSystemPreview()).toBeNull();
    expect(result.monitoredSystemPreviewErrorTitle()).toBe(
      'Monitored-system verification is temporarily unavailable',
    );
    expect(result.monitoredSystemPreviewError()).toBe(
      'Pulse is still settling provider-owned inventory for this platform connection. You can still save the connection and review the impact after the first baseline finishes.',
    );

    await result.saveCurrentForm();

    expect(TrueNASAPI.createConnection).toHaveBeenCalled();
    expect(notificationStore.success).toHaveBeenCalledWith('TrueNAS connection added');
  });

  it('surfaces backend save errors without reopening retired cap-preview handling', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.previewConnection).mockResolvedValueOnce(safeTrueNASPreview() as never);
    vi.mocked(TrueNASAPI.createConnection).mockRejectedValueOnce(
      Object.assign(new Error('TrueNAS connection save failed'), {
        status: 500,
      }),
    );

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'tower.local',
      apiKey: 'secret',
    });
    await result.previewCurrentForm();
    await result.saveCurrentForm();

    expect(notificationStore.error).toHaveBeenCalledWith('TrueNAS connection save failed');
    expect(result.monitoredSystemPreview()).toMatchObject({
      projected_count: 1,
      effect: 'attaches_existing',
    });
    expect(result.dialogOpen()).toBe(true);
  });

  it('treats save-time monitored-system usage unavailability as an ordinary save error', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.previewConnection).mockResolvedValueOnce(safeTrueNASPreview() as never);
    vi.mocked(TrueNASAPI.createConnection).mockRejectedValueOnce(
      Object.assign(new Error('Unable to verify monitored-system grouping right now'), {
        status: 503,
        code: 'monitored_system_usage_unavailable',
        details: {
          reason: 'supplemental_inventory_rebuild_pending',
        },
      }),
    );

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({
      host: 'tower.local',
      apiKey: 'secret',
    });
    await result.previewCurrentForm();
    await result.saveCurrentForm();

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
