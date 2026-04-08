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
      pollIntervalSeconds: 90,
      useHttps: true,
      insecureSkipVerify: false,
      enabled: true,
    } as never);
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([
      {
        id: 'conn-1',
        name: 'tower',
        host: 'truenas.local',
        apiKey: '********',
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
    await result.saveCurrentForm();

    expect(TrueNASAPI.updateConnection).toHaveBeenCalledWith(
      'conn-1',
      expect.objectContaining({
        host: 'truenas.local',
        apiKey: '********',
        pollIntervalSeconds: 90,
        username: '',
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

  it('previews monitored-system impact through the canonical TrueNAS preview path', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.previewConnection).mockResolvedValueOnce({
      current_count: 4,
      projected_count: 4,
      additional_count: 0,
      limit: 10,
      would_exceed_limit: false,
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

  it('blocks save when monitored-system usage is temporarily unavailable during preview', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.previewConnection).mockRejectedValueOnce(
      Object.assign(new Error('Unable to verify monitored-system capacity right now'), {
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
    expect(result.monitoredSystemAdmissionSaveBlocked()).toBe(true);
    expect(result.monitoredSystemPreviewErrorTitle()).toBe(
      'Monitored-system capacity is temporarily unavailable',
    );
    expect(result.monitoredSystemPreviewError()).toBe(
      'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    );

    await result.saveCurrentForm();

    expect(TrueNASAPI.createConnection).not.toHaveBeenCalled();
    expect(notificationStore.error).toHaveBeenLastCalledWith(
      'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    );
  });

  it('reuses the canonical monitored-system preview when a save is denied by the backend', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.createConnection).mockRejectedValueOnce(
      Object.assign(new Error('Monitored-system limit reached (10/9)'), {
        status: 402,
        feature: 'max_monitored_systems',
        monitored_system_preview: {
          current_count: 9,
          projected_count: 10,
          additional_count: 1,
          limit: 9,
          would_exceed_limit: true,
          effect: 'creates_new',
          current_systems: [],
          projected_systems: [
            {
              name: 'tower',
              type: 'truenas-system',
              status: 'online',
              source: 'truenas',
            },
          ],
          current_system: null,
          projected_system: null,
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
    await result.saveCurrentForm();

    expect(notificationStore.error).toHaveBeenCalledWith('Monitored-system limit reached (10/9)');
    expect(result.monitoredSystemPreview()).toMatchObject({
      would_exceed_limit: true,
      projected_count: 10,
      effect: 'creates_new',
    });
    expect(result.dialogOpen()).toBe(true);
  });

  it('surfaces monitored-system usage unavailability when save is rejected before preview settles', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValueOnce([] as never);
    vi.mocked(TrueNASAPI.createConnection).mockRejectedValueOnce(
      Object.assign(new Error('Unable to verify monitored-system capacity right now'), {
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
    await result.saveCurrentForm();

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
