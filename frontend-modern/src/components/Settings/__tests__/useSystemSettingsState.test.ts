import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SettingsTab } from '../settingsTypes';

type UseSystemSettingsStateModule = typeof import('../useSystemSettingsState');

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('useSystemSettingsState', () => {
  let useSystemSettingsState: UseSystemSettingsStateModule['useSystemSettingsState'];
  let getSystemSettingsMock: ReturnType<typeof vi.fn>;
  let getVersionMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();

    getSystemSettingsMock = vi.fn();
    getVersionMock = vi.fn().mockResolvedValue({
      version: '1.0.0',
      build: 'test',
      runtime: 'go1.22',
      channel: 'stable',
      isDocker: false,
      isSourceBuild: false,
      isDevelopment: false,
      deploymentType: 'systemd',
    });

    vi.doMock('@/api/settings', () => ({
      SettingsAPI: {
        getSystemSettings: getSystemSettingsMock,
        updateSystemSettings: vi.fn(),
      },
    }));

    vi.doMock('@/api/updates', () => ({
      UpdatesAPI: {
        getVersion: getVersionMock,
        getUpdatePlan: vi.fn(),
        applyUpdate: vi.fn(),
      },
    }));

    vi.doMock('@/stores/notifications', () => ({
      notificationStore: {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
      },
    }));

    vi.doMock('@/utils/logger', () => ({
      logger: {
        error: vi.fn(),
        warn: vi.fn(),
        info: vi.fn(),
        debug: vi.fn(),
      },
    }));

    vi.doMock('@/utils/apiClient', () => ({
      apiFetch: vi.fn(),
      apiFetchJSON: vi.fn(),
    }));

    vi.doMock('@/stores/updates', () => ({
      updateStore: {
        checkForUpdates: vi.fn().mockResolvedValue(undefined),
        updateInfo: vi.fn().mockReturnValue(null),
        isDismissed: vi.fn().mockReturnValue(false),
        clearDismissed: vi.fn(),
      },
    }));

    vi.doMock('@/stores/systemSettings', () => ({
      updateDockerUpdateActionsSetting: vi.fn(),
    }));

    ({ useSystemSettingsState } = await import('../useSystemSettingsState'));
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.resetModules();
  });

  const mountHook = () => {
    let dispose = () => {};
    let hookState: ReturnType<UseSystemSettingsStateModule['useSystemSettingsState']>;

    createRoot((d) => {
      dispose = d;
      const [_discoveryEnabled, setDiscoveryEnabled] = createSignal(false);

      hookState = useSystemSettingsState({
        activeTab: () => 'system-updates' as SettingsTab,
        currentTab: () => 'system-updates' as SettingsTab,
        loadSecurityStatus: async () => {},
        setDiscoveryEnabled,
        applySavedDiscoverySubnet: () => {},
      });
    });

    return { dispose, hookState: hookState! };
  };

  it('preserves explicit zero auto-update interval values from system settings', async () => {
    getSystemSettingsMock.mockResolvedValue({
      autoUpdateEnabled: true,
      autoUpdateCheckInterval: 0,
      autoUpdateTime: '04:00',
    });

    const { hookState, dispose } = mountHook();

    await hookState.initializeSystemSettingsState();
    await flushAsync();

    expect(hookState.autoUpdateCheckInterval()).toBe(0);
    dispose();
  });
});
