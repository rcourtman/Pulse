import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SettingsTab } from '../settingsNavigationModel';

type UseSystemSettingsStateModule = typeof import('../useSystemSettingsState');

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('useSystemSettingsState', () => {
  let useSystemSettingsState: UseSystemSettingsStateModule['useSystemSettingsState'];
  let getSystemSettingsMock: ReturnType<typeof vi.fn>;
  let getTelemetryPreviewMock: ReturnType<typeof vi.fn>;
  let resetTelemetryInstallIDMock: ReturnType<typeof vi.fn>;
  let getVersionMock: ReturnType<typeof vi.fn>;
  let updateSystemSettingsMock: ReturnType<typeof vi.fn>;
  let updateStoreVersionInfoMock: ReturnType<typeof vi.fn>;
  let copyToClipboardMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();

    getSystemSettingsMock = vi.fn();
    getTelemetryPreviewMock = vi.fn();
    resetTelemetryInstallIDMock = vi.fn();
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
    copyToClipboardMock = vi.fn().mockResolvedValue(true);

    vi.doMock('@/api/settings', () => ({
      SettingsAPI: {
        getSystemSettings: getSystemSettingsMock,
        getTelemetryPreview: getTelemetryPreviewMock,
        resetTelemetryInstallID: resetTelemetryInstallIDMock,
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

    vi.doMock('@/utils/clipboard', () => ({
      copyToClipboard: copyToClipboardMock,
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

  it('loads the telemetry preview payload on demand', async () => {
    getTelemetryPreviewMock.mockResolvedValue({
      enabled: true,
      payload: {
        install_id: 'preview-install-id',
        version: '6.0.0',
        platform: 'docker',
        os: 'linux',
        arch: 'amd64',
        event: 'heartbeat',
        pve_nodes: 1,
        pbs_instances: 0,
        pmg_instances: 0,
        vms: 2,
        containers: 3,
        docker_hosts: 0,
        kubernetes_clusters: 0,
        ai_enabled: false,
        active_alerts: 0,
        relay_enabled: false,
        sso_enabled: false,
        multi_tenant: false,
        paid_license: false,
        has_api_tokens: true,
      },
    });

    const { hookState, dispose } = mountHookWithTab('system-general');

    await hookState.handleLoadTelemetryPreview();
    await flushAsync();

    expect(getTelemetryPreviewMock).toHaveBeenCalledOnce();
    expect(hookState.telemetryPreviewPayload()).toContain('"install_id": "preview-install-id"');
    expect(hookState.telemetryPreviewEnabled()).toBe(true);
    dispose();
  });

  it('copies the loaded telemetry preview payload', async () => {
    getTelemetryPreviewMock.mockResolvedValue({
      enabled: false,
      payload: {
        install_id: 'preview-install-id',
        version: '6.0.0',
        platform: 'docker',
        os: 'linux',
        arch: 'amd64',
        event: 'heartbeat',
        pve_nodes: 1,
        pbs_instances: 0,
        pmg_instances: 0,
        vms: 2,
        containers: 3,
        docker_hosts: 0,
        kubernetes_clusters: 0,
        ai_enabled: false,
        active_alerts: 0,
        relay_enabled: false,
        sso_enabled: false,
        multi_tenant: false,
        paid_license: false,
        has_api_tokens: true,
      },
    });

    const { hookState, dispose } = mountHookWithTab('system-general');
    const { notificationStore } = await import('@/stores/notifications');

    await hookState.handleLoadTelemetryPreview();
    await hookState.handleCopyTelemetryPreview();
    await flushAsync();

    expect(copyToClipboardMock).toHaveBeenCalledWith(expect.stringContaining('"event": "heartbeat"'));
    expect(notificationStore.success).toHaveBeenCalledWith(
      'Telemetry payload copied to clipboard',
      2000,
    );
    dispose();
  });

  it('resets the telemetry install ID after confirmation', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    resetTelemetryInstallIDMock.mockResolvedValue({
      enabled: true,
      payload: {
        install_id: 'rotated-install-id',
        version: '6.0.0',
        platform: 'binary',
        os: 'linux',
        arch: 'amd64',
        event: 'heartbeat',
        pve_nodes: 0,
        pbs_instances: 0,
        pmg_instances: 0,
        vms: 0,
        containers: 0,
        docker_hosts: 0,
        kubernetes_clusters: 0,
        ai_enabled: false,
        active_alerts: 0,
        relay_enabled: false,
        sso_enabled: false,
        multi_tenant: false,
        paid_license: false,
        has_api_tokens: true,
      },
    });

    const { hookState, dispose } = mountHookWithTab('system-general');
    const { notificationStore } = await import('@/stores/notifications');

    await hookState.handleResetTelemetryInstallID();
    await flushAsync();

    expect(confirmSpy).toHaveBeenCalled();
    expect(resetTelemetryInstallIDMock).toHaveBeenCalledOnce();
    expect(hookState.telemetryPreviewPayload()).toContain('"install_id": "rotated-install-id"');
    expect(notificationStore.success).toHaveBeenCalledWith('Telemetry install ID reset', 3000);

    confirmSpy.mockRestore();
    dispose();
  });
});
