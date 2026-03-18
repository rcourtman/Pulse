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
  let updateSystemSettingsMock: ReturnType<typeof vi.fn>;
  let updateStoreVersionInfoMock: ReturnType<typeof vi.fn>;

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
    updateSystemSettingsMock = vi.fn().mockResolvedValue(undefined);
    updateStoreVersionInfoMock = vi.fn().mockReturnValue(null);

    vi.doMock('@/api/settings', () => ({
      SettingsAPI: {
        getSystemSettings: getSystemSettingsMock,
        updateSystemSettings: updateSystemSettingsMock,
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
        versionInfo: updateStoreVersionInfoMock,
        isDismissed: vi.fn().mockReturnValue(false),
        clearDismissed: vi.fn(),
      },
    }));

    vi.doMock('@/stores/systemSettings', () => ({
      updateDisableLocalUpgradeMetricsSetting: vi.fn(),
      updateDockerUpdateActionsSetting: vi.fn(),
      updateReduceProUpsellNoiseSetting: vi.fn(),
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
        loadSecurityStatus: async () => {},
        setDiscoveryEnabled,
        applySavedDiscoverySubnet: () => {},
      });
    });

    return { dispose, hookState: hookState! };
  };

  const mountHookWithTab = (tab: SettingsTab) => {
    let dispose = () => {};
    let hookState: ReturnType<UseSystemSettingsStateModule['useSystemSettingsState']>;

    createRoot((d) => {
      dispose = d;
      const [_discoveryEnabled, setDiscoveryEnabled] = createSignal(false);

      hookState = useSystemSettingsState({
        activeTab: () => tab,
        loadSecurityStatus: async () => {},
        setDiscoveryEnabled,
        applySavedDiscoverySubnet: () => {},
      });
    });

    return { dispose, hookState: hookState! };
  };

  it('shows clean success toast without port reference when saving from General tab', async () => {
    const { hookState, dispose } = mountHookWithTab('system-general');
    const { notificationStore } = await import('@/stores/notifications');

    await hookState.saveSettings();
    await flushAsync();

    expect(notificationStore.success).toHaveBeenCalledWith('Settings saved successfully.');
    dispose();
  });

  it('shows network changes toast when saving from Network tab', async () => {
    const { hookState, dispose } = mountHookWithTab('system-network');
    const { notificationStore } = await import('@/stores/notifications');

    await hookState.saveSettings();
    await flushAsync();

    expect(notificationStore.success).toHaveBeenCalledWith(
      'Settings saved successfully. Service restart may be required for network changes.',
    );
    dispose();
  });

  it('does not reload the page when saving from General tab', async () => {
    const savedLocation = window.location;
    vi.useFakeTimers();
    try {
      const reloadMock = vi.fn();
      Object.defineProperty(window, 'location', {
        value: { ...savedLocation, reload: reloadMock },
        writable: true,
        configurable: true,
      });

      const { hookState, dispose } = mountHookWithTab('system-general');

      await hookState.saveSettings();
      await flushAsync();

      vi.advanceTimersByTime(5000);
      expect(reloadMock).not.toHaveBeenCalled();

      dispose();
    } finally {
      Object.defineProperty(window, 'location', {
        value: savedLocation,
        writable: true,
        configurable: true,
      });
      vi.useRealTimers();
    }
  });

  it('reloads the page when saving from Network tab', async () => {
    const savedLocation = window.location;
    vi.useFakeTimers();
    try {
      const reloadMock = vi.fn();
      Object.defineProperty(window, 'location', {
        value: { ...savedLocation, reload: reloadMock },
        writable: true,
        configurable: true,
      });

      const { hookState, dispose } = mountHookWithTab('system-network');

      await hookState.saveSettings();
      await flushAsync();

      vi.advanceTimersByTime(3000);
      expect(reloadMock).toHaveBeenCalledOnce();

      dispose();
    } finally {
      Object.defineProperty(window, 'location', {
        value: savedLocation,
        writable: true,
        configurable: true,
      });
      vi.useRealTimers();
    }
  });

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

  it('normalizes persisted RC auto-updates off during initialization', async () => {
    getSystemSettingsMock.mockResolvedValue({
      updateChannel: 'rc',
      autoUpdateEnabled: true,
      autoUpdateCheckInterval: 24,
      autoUpdateTime: '04:00',
    });

    const { hookState, dispose } = mountHook();

    await hookState.initializeSystemSettingsState();
    await flushAsync();

    expect(hookState.updateChannel()).toBe('rc');
    expect(hookState.autoUpdateEnabled()).toBe(false);
    dispose();
  });

  it('reuses version info from the shared update store during initialization', async () => {
    updateStoreVersionInfoMock.mockReturnValue({
      version: '1.0.1',
      build: 'retry',
      runtime: 'go1.22',
      channel: 'stable',
      isDocker: false,
      isSourceBuild: false,
      isDevelopment: false,
      deploymentType: 'systemd',
    });

    const { hookState, dispose } = mountHook();

    await hookState.initializeSystemSettingsState();
    await flushAsync();

    expect(hookState.versionInfo()?.version).toBe('1.0.1');
    expect(getVersionMock).not.toHaveBeenCalled();
    dispose();
  });

  it('forces autoUpdateEnabled off when saving RC channel settings', async () => {
    const { hookState, dispose } = mountHook();

    hookState.setUpdateChannel('rc');
    hookState.setAutoUpdateEnabled(true);

    await hookState.saveSettings();
    await flushAsync();

    expect(updateSystemSettingsMock).toHaveBeenCalledWith(
      expect.objectContaining({
        updateChannel: 'rc',
        autoUpdateEnabled: false,
      }),
    );
    dispose();
  });
});
