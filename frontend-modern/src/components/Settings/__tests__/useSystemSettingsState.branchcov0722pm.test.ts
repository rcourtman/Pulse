import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SettingsTab } from '../settingsNavigationModel';
import { useSystemSettingsState } from '../useSystemSettingsState';

// The sibling test drives every dependency through vi.doMock + vi.resetModules +
// a dynamic re-import of the target. That re-imports a second instrumented copy
// of the hook and makes its coverage unmeasurable, so this file instead uses
// hoisted vi.mock factories and a single static import of the target. The
// createRoot/dispose invocation pattern and the mock object shapes are otherwise
// identical to the sibling test.
const mocks = vi.hoisted(() => ({
  getSystemSettingsMock: vi.fn(),
  updateSystemSettingsMock: vi.fn(),
  getTelemetryPreviewMock: vi.fn(),
  resetTelemetryInstallIDMock: vi.fn(),
  getVersionMock: vi.fn(),
  getUpdatePlanMock: vi.fn(),
  copyToClipboardMock: vi.fn(),
  notificationSuccessMock: vi.fn(),
  notificationErrorMock: vi.fn(),
  notificationInfoMock: vi.fn(),
  loggerErrorMock: vi.fn(),
  loggerWarnMock: vi.fn(),
  updateStoreCheckForUpdatesMock: vi.fn(),
  updateStoreApplyUpdateMock: vi.fn(),
  updateStoreUpdateInfoMock: vi.fn(),
  updateStoreVersionInfoMock: vi.fn(),
  updateStoreIsDismissedMock: vi.fn(),
  updateStoreClearDismissedMock: vi.fn(),
  updateDockerUpdateActionsSettingMock: vi.fn(),
}));

vi.mock('@/api/settings', () => ({
  SettingsAPI: {
    getSystemSettings: mocks.getSystemSettingsMock,
    getTelemetryPreview: mocks.getTelemetryPreviewMock,
    resetTelemetryInstallID: mocks.resetTelemetryInstallIDMock,
    updateSystemSettings: mocks.updateSystemSettingsMock,
  },
}));

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    getVersion: mocks.getVersionMock,
    getUpdatePlan: mocks.getUpdatePlanMock,
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: mocks.notificationSuccessMock,
    error: mocks.notificationErrorMock,
    info: mocks.notificationInfoMock,
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: mocks.loggerErrorMock,
    warn: mocks.loggerWarnMock,
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: mocks.copyToClipboardMock,
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(),
  apiFetchJSON: vi.fn(),
}));

vi.mock('@/stores/updates', () => ({
  updateStore: {
    checkForUpdates: mocks.updateStoreCheckForUpdatesMock,
    applyUpdate: mocks.updateStoreApplyUpdateMock,
    updateInfo: mocks.updateStoreUpdateInfoMock,
    versionInfo: mocks.updateStoreVersionInfoMock,
    isDismissed: mocks.updateStoreIsDismissedMock,
    clearDismissed: mocks.updateStoreClearDismissedMock,
  },
}));

vi.mock('@/stores/systemSettings', () => ({
  updateDockerUpdateActionsSetting: mocks.updateDockerUpdateActionsSettingMock,
}));

type HookState = ReturnType<typeof useSystemSettingsState>;

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('useSystemSettingsState branch coverage', () => {
  beforeEach(() => {
    mocks.getSystemSettingsMock.mockResolvedValue({});
    mocks.updateSystemSettingsMock.mockResolvedValue(undefined);
    mocks.getUpdatePlanMock.mockResolvedValue({
      canAutoUpdate: false,
      requiresRoot: false,
      rollbackSupport: false,
    });
    mocks.updateStoreCheckForUpdatesMock.mockResolvedValue(undefined);
    mocks.updateStoreApplyUpdateMock.mockResolvedValue(true);
    mocks.updateStoreUpdateInfoMock.mockReturnValue(null);
    mocks.updateStoreVersionInfoMock.mockReturnValue(null);
    mocks.updateStoreIsDismissedMock.mockReturnValue(false);
  });

  afterEach(() => {
    // resetAllMocks (not just clearAllMocks) so any one-shot value queues from
    // mockRejectedValueOnce/mockResolvedValueOnce cannot leak across tests; the
    // single static import of the target is unaffected.
    vi.resetAllMocks();
  });

  const mountHook = (options?: {
    activeTab?: SettingsTab;
    loadSecurityStatus?: () => Promise<void>;
  }): { dispose: () => void; hookState: HookState } => {
    let dispose = () => {};
    let hookState: HookState;

    createRoot((d) => {
      dispose = d;
      const [, setDiscoveryEnabled] = createSignal(false);
      hookState = useSystemSettingsState({
        activeTab: () => options?.activeTab ?? ('system-updates' as SettingsTab),
        loadSecurityStatus: options?.loadSecurityStatus ?? (async () => {}),
        setDiscoveryEnabled,
        applySavedDiscoverySubnet: () => {},
      });
    });

    return { dispose, hookState: hookState! };
  };

  // envOverrides is an internal signal with no exposed setter, so the only way
  // to drive the *Locked accessors is to seed it through initializeSystemSettingsState.
  const mountWithEnvOverrides = async (
    envOverrides: Record<string, boolean>,
  ): Promise<{ dispose: () => void; hookState: HookState }> => {
    mocks.getSystemSettingsMock.mockResolvedValue({ envOverrides });
    const mounted = mountHook();
    await mounted.hookState.initializeSystemSettingsState();
    await flushAsync();
    return mounted;
  };

  describe.each([
    {
      name: 'temperatureMonitoringLocked',
      accessor: (h: HookState) => h.temperatureMonitoringLocked(),
      firstKey: 'temperatureMonitoringEnabled',
      secondKey: 'ENABLE_TEMPERATURE_MONITORING',
    },
    {
      name: 'hideLocalLoginLocked',
      accessor: (h: HookState) => h.hideLocalLoginLocked(),
      firstKey: 'hideLocalLogin',
      secondKey: 'PULSE_AUTH_HIDE_LOCAL_LOGIN',
    },
    {
      name: 'disableDockerUpdateActionsLocked',
      accessor: (h: HookState) => h.disableDockerUpdateActionsLocked(),
      firstKey: 'disableDockerUpdateActions',
      secondKey: 'PULSE_DISABLE_DOCKER_UPDATE_ACTIONS',
    },
    {
      name: 'telemetryEnabledLocked',
      accessor: (h: HookState) => h.telemetryEnabledLocked(),
      firstKey: 'telemetryEnabled',
      secondKey: 'PULSE_TELEMETRY',
    },
    {
      name: 'pvePollingEnvLocked',
      accessor: (h: HookState) => h.pvePollingEnvLocked(),
      firstKey: 'pvePollingInterval',
      secondKey: 'PVE_POLLING_INTERVAL',
    },
    {
      name: 'backupPollingEnvLocked',
      accessor: (h: HookState) => h.backupPollingEnvLocked(),
      firstKey: 'ENABLE_BACKUP_POLLING',
      secondKey: 'BACKUP_POLLING_INTERVAL',
    },
  ])('$name', ({ accessor, firstKey, secondKey }) => {
    it('reports unlocked when no env overrides are present', () => {
      const { hookState, dispose } = mountHook();
      expect(accessor(hookState)).toBe(false);
      dispose();
    });

    it('locks when the primary override key is set', async () => {
      const { hookState, dispose } = await mountWithEnvOverrides({ [firstKey]: true });
      expect(accessor(hookState)).toBe(true);
      dispose();
    });

    it('locks when the secondary ENV override key is set', async () => {
      const { hookState, dispose } = await mountWithEnvOverrides({ [secondKey]: true });
      expect(accessor(hookState)).toBe(true);
      dispose();
    });
  });

  describe('backupIntervalSelectValue', () => {
    it('returns "custom" when custom polling is selected', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingUseCustom(true);
      hookState.setBackupPollingInterval(0);
      expect(hookState.backupIntervalSelectValue()).toBe('custom');
      dispose();
    });

    it('returns the numeric preset value for a known preset interval', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingUseCustom(false);
      hookState.setBackupPollingInterval(900);
      expect(hookState.backupIntervalSelectValue()).toBe('900');
      dispose();
    });

    it('returns "0" for the default preset interval', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingUseCustom(false);
      hookState.setBackupPollingInterval(0);
      expect(hookState.backupIntervalSelectValue()).toBe('0');
      dispose();
    });

    it('falls back to "custom" for an out-of-range interval', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingUseCustom(false);
      hookState.setBackupPollingInterval(12345);
      expect(hookState.backupIntervalSelectValue()).toBe('custom');
      dispose();
    });
  });

  describe('backupIntervalSummary', () => {
    it('reports disabled when backup polling is off', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingEnabled(false);
      hookState.setBackupPollingInterval(3600);
      expect(hookState.backupIntervalSummary()).toBe('Backup polling is disabled.');
      dispose();
    });

    it('reports the default cadence when the interval is zero', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingEnabled(true);
      hookState.setBackupPollingInterval(0);
      expect(hookState.backupIntervalSummary()).toBe(
        'Pulse checks backups and snapshots at the default cadence (~every 90 seconds).',
      );
      dispose();
    });

    it('renders a single day cadence', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingEnabled(true);
      hookState.setBackupPollingInterval(86400);
      expect(hookState.backupIntervalSummary()).toBe('Pulse checks backups every day.');
      dispose();
    });

    it('renders a multi-day cadence', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingEnabled(true);
      hookState.setBackupPollingInterval(172800);
      expect(hookState.backupIntervalSummary()).toBe('Pulse checks backups every 2 days.');
      dispose();
    });

    it('renders a single hour cadence', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingEnabled(true);
      hookState.setBackupPollingInterval(3600);
      expect(hookState.backupIntervalSummary()).toBe('Pulse checks backups every hour.');
      dispose();
    });

    it('renders a multi-hour cadence', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingEnabled(true);
      hookState.setBackupPollingInterval(7200);
      expect(hookState.backupIntervalSummary()).toBe('Pulse checks backups every 2 hours.');
      dispose();
    });

    it('renders a single minute cadence', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingEnabled(true);
      hookState.setBackupPollingInterval(60);
      expect(hookState.backupIntervalSummary()).toBe('Pulse checks backups every minute.');
      dispose();
    });

    it('renders a multi-minute cadence', () => {
      const { hookState, dispose } = mountHook();
      hookState.setBackupPollingEnabled(true);
      hookState.setBackupPollingInterval(300);
      expect(hookState.backupIntervalSummary()).toBe('Pulse checks backups every 5 minutes.');
      dispose();
    });
  });

  describe('handleHideLocalLoginChange', () => {
    it('is a no-op when the setting is locked by an env override', async () => {
      const { hookState, dispose } = await mountWithEnvOverrides({
        PULSE_AUTH_HIDE_LOCAL_LOGIN: true,
      });
      await hookState.handleHideLocalLoginChange(true);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).not.toHaveBeenCalled();
      expect(hookState.hideLocalLogin()).toBe(false);
      expect(hookState.savingHideLocalLogin()).toBe(false);
      dispose();
    });

    it('hides local login, toasts success, and refreshes security status', async () => {
      const loadSecurityStatus = vi.fn().mockResolvedValue(undefined);
      const { hookState, dispose } = mountHook({ loadSecurityStatus });
      await hookState.handleHideLocalLoginChange(true);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).toHaveBeenCalledWith({ hideLocalLogin: true });
      expect(hookState.hideLocalLogin()).toBe(true);
      expect(mocks.notificationSuccessMock).toHaveBeenCalledWith('Local login hidden', 2000);
      expect(loadSecurityStatus).toHaveBeenCalledTimes(1);
      expect(hookState.savingHideLocalLogin()).toBe(false);
      dispose();
    });

    it('uses the info toast and posts false when unhiding local login', async () => {
      const loadSecurityStatus = vi.fn().mockResolvedValue(undefined);
      const { hookState, dispose } = mountHook({ loadSecurityStatus });
      await hookState.handleHideLocalLoginChange(false);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).toHaveBeenCalledWith({ hideLocalLogin: false });
      expect(mocks.notificationInfoMock).toHaveBeenCalledWith('Local login visible', 2000);
      expect(hookState.hideLocalLogin()).toBe(false);
      expect(loadSecurityStatus).toHaveBeenCalledTimes(1);
      dispose();
    });

    it('forwards the backend error message and reverts the toggle on API failure', async () => {
      const loadSecurityStatus = vi.fn().mockResolvedValue(undefined);
      mocks.updateSystemSettingsMock.mockRejectedValueOnce(new Error('hide failed'));
      const { hookState, dispose } = mountHook({ loadSecurityStatus });
      await hookState.handleHideLocalLoginChange(true);
      await flushAsync();
      expect(hookState.hideLocalLogin()).toBe(false);
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith('hide failed');
      expect(loadSecurityStatus).not.toHaveBeenCalled();
      expect(hookState.savingHideLocalLogin()).toBe(false);
      dispose();
    });

    it('falls back to the default message for non-Error failures', async () => {
      mocks.updateSystemSettingsMock.mockRejectedValueOnce('unknown fault' as unknown as Error);
      const { hookState, dispose } = mountHook();
      await hookState.handleHideLocalLoginChange(true);
      await flushAsync();
      expect(hookState.hideLocalLogin()).toBe(false);
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith(
        'Unable to update local login visibility.',
      );
      dispose();
    });
  });

  describe('handleDisableDockerUpdateActionsChange', () => {
    it('is a no-op when the setting is locked by an env override', async () => {
      const { hookState, dispose } = await mountWithEnvOverrides({
        PULSE_DISABLE_DOCKER_UPDATE_ACTIONS: true,
      });
      await hookState.handleDisableDockerUpdateActionsChange(true);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).not.toHaveBeenCalled();
      expect(mocks.updateDockerUpdateActionsSettingMock).not.toHaveBeenCalled();
      expect(hookState.disableDockerUpdateActions()).toBe(false);
      expect(hookState.savingDockerUpdateActions()).toBe(false);
      dispose();
    });

    it('hides docker update buttons and mirrors the value into the shared store', async () => {
      const { hookState, dispose } = mountHook();
      await hookState.handleDisableDockerUpdateActionsChange(true);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).toHaveBeenCalledWith({
        disableDockerUpdateActions: true,
      });
      expect(mocks.updateDockerUpdateActionsSettingMock).toHaveBeenCalledWith(true);
      expect(mocks.notificationSuccessMock).toHaveBeenCalledWith(
        'Docker update buttons hidden',
        2000,
      );
      expect(hookState.disableDockerUpdateActions()).toBe(true);
      expect(hookState.savingDockerUpdateActions()).toBe(false);
      dispose();
    });

    it('restores docker update buttons when disabling the hide flag', async () => {
      const { hookState, dispose } = mountHook();
      await hookState.handleDisableDockerUpdateActionsChange(false);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).toHaveBeenCalledWith({
        disableDockerUpdateActions: false,
      });
      expect(mocks.updateDockerUpdateActionsSettingMock).toHaveBeenCalledWith(false);
      expect(mocks.notificationInfoMock).toHaveBeenCalledWith(
        'Docker update buttons visible',
        2000,
      );
      expect(hookState.disableDockerUpdateActions()).toBe(false);
      dispose();
    });

    it('forwards the backend error message and reverts the toggle on API failure', async () => {
      mocks.updateSystemSettingsMock.mockRejectedValueOnce(new Error('docker failed'));
      const { hookState, dispose } = mountHook();
      await hookState.handleDisableDockerUpdateActionsChange(true);
      await flushAsync();
      expect(hookState.disableDockerUpdateActions()).toBe(false);
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith('docker failed');
      expect(mocks.updateDockerUpdateActionsSettingMock).not.toHaveBeenCalled();
      expect(hookState.savingDockerUpdateActions()).toBe(false);
      dispose();
    });

    it('falls back to the default message for non-Error failures', async () => {
      mocks.updateSystemSettingsMock.mockRejectedValueOnce('unknown fault' as unknown as Error);
      const { hookState, dispose } = mountHook();
      await hookState.handleDisableDockerUpdateActionsChange(true);
      await flushAsync();
      expect(hookState.disableDockerUpdateActions()).toBe(false);
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith(
        expect.stringMatching(/^Unable to update .+ update actions\.$/),
      );
      dispose();
    });
  });

  describe('handleTelemetryEnabledChange', () => {
    it('is a no-op when the setting is locked by an env override', async () => {
      const { hookState, dispose } = await mountWithEnvOverrides({ PULSE_TELEMETRY: true });
      await hookState.handleTelemetryEnabledChange(true);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).not.toHaveBeenCalled();
      expect(hookState.telemetryEnabled()).toBe(true);
      expect(hookState.savingTelemetry()).toBe(false);
      dispose();
    });

    it('enables telemetry and toasts the enabled message', async () => {
      const { hookState, dispose } = mountHook();
      await hookState.handleTelemetryEnabledChange(true);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).toHaveBeenCalledWith({ telemetryEnabled: true });
      expect(hookState.telemetryEnabled()).toBe(true);
      expect(mocks.notificationSuccessMock).toHaveBeenCalledWith(
        'Outbound usage telemetry enabled',
        3000,
      );
      expect(hookState.savingTelemetry()).toBe(false);
      dispose();
    });

    it('disables telemetry and toasts the disabled message', async () => {
      const { hookState, dispose } = mountHook();
      await hookState.handleTelemetryEnabledChange(false);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).toHaveBeenCalledWith({ telemetryEnabled: false });
      expect(hookState.telemetryEnabled()).toBe(false);
      expect(mocks.notificationSuccessMock).toHaveBeenCalledWith(
        'Outbound usage telemetry disabled',
        3000,
      );
      dispose();
    });

    it('forwards the backend error message and reverts the toggle on API failure', async () => {
      mocks.updateSystemSettingsMock.mockRejectedValueOnce(new Error('telemetry failed'));
      const { hookState, dispose } = mountHook();
      await hookState.handleTelemetryEnabledChange(false);
      await flushAsync();
      expect(hookState.telemetryEnabled()).toBe(true);
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith('telemetry failed');
      expect(hookState.savingTelemetry()).toBe(false);
      dispose();
    });

    it('falls back to the default message for non-Error failures', async () => {
      mocks.updateSystemSettingsMock.mockRejectedValueOnce('unknown fault' as unknown as Error);
      const { hookState, dispose } = mountHook();
      await hookState.handleTelemetryEnabledChange(false);
      await flushAsync();
      expect(hookState.telemetryEnabled()).toBe(true);
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith(
        'Unable to update outbound usage telemetry.',
      );
      dispose();
    });
  });

  describe('handleTemperatureMonitoringChange', () => {
    it('is a no-op when the setting is locked by an env override', async () => {
      const { hookState, dispose } = await mountWithEnvOverrides({
        ENABLE_TEMPERATURE_MONITORING: true,
      });
      await hookState.handleTemperatureMonitoringChange(false);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).not.toHaveBeenCalled();
      expect(hookState.temperatureMonitoringEnabled()).toBe(true);
      expect(hookState.savingTemperatureSetting()).toBe(false);
      dispose();
    });

    it('is a no-op while a save is already in flight', async () => {
      const { hookState, dispose } = mountHook();
      hookState.setSavingTemperatureSetting(true);
      await hookState.handleTemperatureMonitoringChange(false);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).not.toHaveBeenCalled();
      expect(hookState.temperatureMonitoringEnabled()).toBe(true);
      dispose();
    });

    it('enables temperature monitoring and toasts success', async () => {
      const { hookState, dispose } = mountHook();
      hookState.setTemperatureMonitoringEnabled(false);
      await hookState.handleTemperatureMonitoringChange(true);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).toHaveBeenCalledWith({
        temperatureMonitoringEnabled: true,
      });
      expect(hookState.temperatureMonitoringEnabled()).toBe(true);
      expect(mocks.notificationSuccessMock).toHaveBeenCalledWith(
        'Temperature monitoring enabled',
        2000,
      );
      expect(hookState.savingTemperatureSetting()).toBe(false);
      dispose();
    });

    it('disables temperature monitoring and toasts an info message', async () => {
      const { hookState, dispose } = mountHook();
      await hookState.handleTemperatureMonitoringChange(false);
      await flushAsync();
      expect(mocks.updateSystemSettingsMock).toHaveBeenCalledWith({
        temperatureMonitoringEnabled: false,
      });
      expect(hookState.temperatureMonitoringEnabled()).toBe(false);
      expect(mocks.notificationInfoMock).toHaveBeenCalledWith(
        'Temperature monitoring disabled',
        2000,
      );
      dispose();
    });

    it('forwards the backend error message and reverts the toggle on API failure', async () => {
      mocks.updateSystemSettingsMock.mockRejectedValueOnce(new Error('temp failed'));
      const { hookState, dispose } = mountHook();
      await hookState.handleTemperatureMonitoringChange(false);
      await flushAsync();
      expect(hookState.temperatureMonitoringEnabled()).toBe(true);
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith('temp failed');
      expect(hookState.savingTemperatureSetting()).toBe(false);
      dispose();
    });

    it('falls back to the default message for non-Error failures', async () => {
      mocks.updateSystemSettingsMock.mockRejectedValueOnce('unknown fault' as unknown as Error);
      const { hookState, dispose } = mountHook();
      await hookState.handleTemperatureMonitoringChange(false);
      await flushAsync();
      expect(hookState.temperatureMonitoringEnabled()).toBe(true);
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith(
        'Unable to update temperature monitoring.',
      );
      dispose();
    });
  });

  describe('checkForUpdates', () => {
    it('fetches the plan and clears a dismissed update when one is available', async () => {
      const plan = { canAutoUpdate: true, requiresRoot: false, rollbackSupport: true };
      mocks.getUpdatePlanMock.mockResolvedValue(plan);
      mocks.updateStoreUpdateInfoMock.mockReturnValue({
        available: true,
        latestVersion: '2.0.0',
      });
      mocks.updateStoreIsDismissedMock.mockReturnValue(true);
      const { hookState, dispose } = mountHook();
      await hookState.checkForUpdates();
      await flushAsync();
      expect(mocks.updateStoreCheckForUpdatesMock).toHaveBeenCalledWith(true);
      expect(mocks.getUpdatePlanMock).toHaveBeenCalledWith('2.0.0');
      expect(hookState.updatePlan()).toEqual(plan);
      expect(hookState.updateInfo()).toEqual({ available: true, latestVersion: '2.0.0' });
      expect(mocks.updateStoreClearDismissedMock).toHaveBeenCalledTimes(1);
      expect(mocks.notificationSuccessMock).not.toHaveBeenCalledWith(
        'You are running the latest version',
      );
      expect(hookState.checkingForUpdates()).toBe(false);
      dispose();
    });

    it('toast the latest-version message and clears the plan when no update is available', async () => {
      mocks.updateStoreUpdateInfoMock.mockReturnValue(null);
      const { hookState, dispose } = mountHook();
      await hookState.checkForUpdates();
      await flushAsync();
      expect(mocks.getUpdatePlanMock).not.toHaveBeenCalled();
      expect(hookState.updatePlan()).toBeNull();
      expect(mocks.notificationSuccessMock).toHaveBeenCalledWith(
        'You are running the latest version',
      );
      expect(mocks.updateStoreClearDismissedMock).not.toHaveBeenCalled();
      dispose();
    });

    it('clears the plan and warns when the plan fetch fails for an available update', async () => {
      mocks.updateStoreUpdateInfoMock.mockReturnValue({
        available: true,
        latestVersion: '2.0.0',
      });
      mocks.getUpdatePlanMock.mockRejectedValue(new Error('plan down'));
      const { hookState, dispose } = mountHook();
      await hookState.checkForUpdates();
      await flushAsync();
      expect(hookState.updatePlan()).toBeNull();
      expect(mocks.loggerWarnMock).toHaveBeenCalledWith(
        'Failed to fetch update plan',
        expect.any(Error),
      );
      expect(mocks.updateStoreClearDismissedMock).not.toHaveBeenCalled();
      dispose();
    });

    it('toast an error and logs when checking for updates throws', async () => {
      mocks.updateStoreCheckForUpdatesMock.mockRejectedValue(new Error('network down'));
      const { hookState, dispose } = mountHook();
      await hookState.checkForUpdates();
      await flushAsync();
      expect(mocks.notificationErrorMock).toHaveBeenCalledWith('Unable to check for updates.');
      expect(mocks.loggerErrorMock).toHaveBeenCalledWith('Update check error', expect.any(Error));
      expect(hookState.checkingForUpdates()).toBe(false);
      dispose();
    });

    it('skips the plan fetch when an update is reported without a latest version', async () => {
      mocks.updateStoreUpdateInfoMock.mockReturnValue({ available: true, latestVersion: '' });
      const { hookState, dispose } = mountHook();
      await hookState.checkForUpdates();
      await flushAsync();
      expect(mocks.getUpdatePlanMock).not.toHaveBeenCalled();
      expect(hookState.updatePlan()).toBeNull();
      expect(mocks.notificationSuccessMock).not.toHaveBeenCalledWith(
        'You are running the latest version',
      );
      expect(mocks.updateStoreClearDismissedMock).not.toHaveBeenCalled();
      dispose();
    });
  });

  describe('handleInstallUpdate', () => {
    it('opens the update confirmation dialog', async () => {
      const { hookState, dispose } = mountHook();
      expect(hookState.showUpdateConfirmation()).toBe(false);
      await hookState.handleInstallUpdate();
      expect(hookState.showUpdateConfirmation()).toBe(true);
      dispose();
    });
  });

  describe('handleConfirmUpdate', () => {
    it('applies the update and closes the confirmation when the store starts it', async () => {
      const { hookState, dispose } = mountHook();
      hookState.setShowUpdateConfirmation(true);
      mocks.updateStoreApplyUpdateMock.mockResolvedValue(true);
      await hookState.handleConfirmUpdate();
      await flushAsync();
      expect(mocks.updateStoreApplyUpdateMock).toHaveBeenCalledTimes(1);
      expect(hookState.showUpdateConfirmation()).toBe(false);
      expect(hookState.isInstallingUpdate()).toBe(false);
      dispose();
    });

    it('keeps the confirmation open when the store declines to start', async () => {
      const { hookState, dispose } = mountHook();
      hookState.setShowUpdateConfirmation(true);
      mocks.updateStoreApplyUpdateMock.mockResolvedValue(false);
      await hookState.handleConfirmUpdate();
      await flushAsync();
      expect(hookState.showUpdateConfirmation()).toBe(true);
      expect(hookState.isInstallingUpdate()).toBe(false);
      dispose();
    });

    it('still clears the installing flag (finally) when the store throws', async () => {
      const { hookState, dispose } = mountHook();
      hookState.setShowUpdateConfirmation(true);
      mocks.updateStoreApplyUpdateMock.mockRejectedValue(new Error('apply failed'));
      await expect(hookState.handleConfirmUpdate()).rejects.toThrow('apply failed');
      expect(hookState.isInstallingUpdate()).toBe(false);
      expect(hookState.showUpdateConfirmation()).toBe(true);
      dispose();
    });
  });
});
