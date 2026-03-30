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
    testConnection: vi.fn(),
  },
  isRedactedTrueNASSecret: (value: string | null | undefined) => (value || '').trim() === '********',
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
});
