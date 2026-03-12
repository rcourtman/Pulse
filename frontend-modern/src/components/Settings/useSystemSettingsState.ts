import { Accessor, Setter, createSignal } from 'solid-js';
import { SettingsAPI } from '@/api/settings';
import { UpdatesAPI } from '@/api/updates';
import type { UpdateInfo, UpdatePlan, VersionInfo } from '@/api/updates';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { updateStore } from '@/stores/updates';
import {
  updateDisableLocalUpgradeMetricsSetting,
  updateDockerUpdateActionsSetting,
  updateReduceProUpsellNoiseSetting,
} from '@/stores/systemSettings';
import {
  BACKUP_INTERVAL_OPTIONS,
  getCheckForUpdatesErrorMessage,
  getBackupIntervalSelectValue,
  getBackupIntervalSummary,
  getDockerUpdateActionsUpdateErrorMessage,
  getHideLocalLoginUpdateErrorMessage,
  getLocalUpgradeMetricsUpdateErrorMessage,
  getReduceUpsellNoiseUpdateErrorMessage,
  getStartUpdateErrorMessage,
  getSystemSettingsSaveErrorMessage,
  getTelemetryUpdateErrorMessage,
  getTemperatureMonitoringUpdateErrorMessage,
  PVE_POLLING_MAX_SECONDS,
  PVE_POLLING_MIN_SECONDS,
  PVE_POLLING_PRESETS,
} from '@/utils/systemSettingsPresentation';
import { getSettingsTabSaveBehavior } from './settingsTabs';
import type { SettingsTab } from './settingsTypes';

interface UseSystemSettingsStateParams {
  activeTab: Accessor<SettingsTab>;
  loadSecurityStatus: () => Promise<void>;
  setDiscoveryEnabled: Setter<boolean>;
  applySavedDiscoverySubnet: (subnet?: string | null) => void;
}

export function useSystemSettingsState({
  activeTab,
  loadSecurityStatus,
  setDiscoveryEnabled,
  applySavedDiscoverySubnet,
}: UseSystemSettingsStateParams) {
  const normalizeAutoUpdateEnabled = (channel: 'stable' | 'rc', enabled: boolean) =>
    channel === 'stable' && enabled;
  const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
  const [pvePollingInterval, setPVEPollingInterval] = createSignal<number>(PVE_POLLING_MIN_SECONDS);
  const [pvePollingSelection, setPVEPollingSelection] = createSignal<number | 'custom'>(
    PVE_POLLING_MIN_SECONDS,
  );
  const [pvePollingCustomSeconds, setPVEPollingCustomSeconds] = createSignal(30);
  const [allowedOrigins, setAllowedOrigins] = createSignal('');
  const [allowEmbedding, setAllowEmbedding] = createSignal(false);
  const [allowedEmbedOrigins, setAllowedEmbedOrigins] = createSignal('');
  const [webhookAllowedPrivateCIDRs, setWebhookAllowedPrivateCIDRs] = createSignal('');
  const [publicURL, setPublicURL] = createSignal('');
  const [envOverrides, setEnvOverrides] = createSignal<Record<string, boolean>>({});
  const [temperatureMonitoringEnabled, setTemperatureMonitoringEnabled] = createSignal(true);
  const [savingTemperatureSetting, setSavingTemperatureSetting] = createSignal(false);
  const [hideLocalLogin, setHideLocalLogin] = createSignal(false);
  const [savingHideLocalLogin, setSavingHideLocalLogin] = createSignal(false);
  const [disableDockerUpdateActions, setDisableDockerUpdateActions] = createSignal(false);
  const [savingDockerUpdateActions, setSavingDockerUpdateActions] = createSignal(false);
  const [reduceProUpsellNoise, setReduceProUpsellNoise] = createSignal(false);
  const [savingReduceUpsells, setSavingReduceUpsells] = createSignal(false);
  const [disableLocalUpgradeMetrics, setDisableLocalUpgradeMetrics] = createSignal(false);
  const [savingUpgradeMetrics, setSavingUpgradeMetrics] = createSignal(false);
  const [telemetryEnabled, setTelemetryEnabled] = createSignal(true);
  const [savingTelemetry, setSavingTelemetry] = createSignal(false);
  const [versionInfo, setVersionInfo] = createSignal<VersionInfo | null>(null);
  const [updateInfo, setUpdateInfo] = createSignal<UpdateInfo | null>(null);
  const [checkingForUpdates, setCheckingForUpdates] = createSignal(false);
  const [updateChannel, setUpdateChannel] = createSignal<'stable' | 'rc'>('stable');
  const [autoUpdateEnabled, setAutoUpdateEnabled] = createSignal(false);
  const [autoUpdateCheckInterval, setAutoUpdateCheckInterval] = createSignal(24);
  const [autoUpdateTime, setAutoUpdateTime] = createSignal('03:00');
  const [updatePlan, setUpdatePlan] = createSignal<UpdatePlan | null>(null);
  const [isInstallingUpdate, setIsInstallingUpdate] = createSignal(false);
  const [showUpdateConfirmation, setShowUpdateConfirmation] = createSignal(false);
  const [backupPollingEnabled, setBackupPollingEnabled] = createSignal(true);
  const [backupPollingInterval, setBackupPollingInterval] = createSignal(0);
  const [backupPollingCustomMinutes, setBackupPollingCustomMinutes] = createSignal(60);
  const [backupPollingUseCustom, setBackupPollingUseCustom] = createSignal(false);

  const temperatureMonitoringLocked = () =>
    Boolean(
      envOverrides().temperatureMonitoringEnabled ||
      envOverrides()['ENABLE_TEMPERATURE_MONITORING'],
    );
  const hideLocalLoginLocked = () =>
    Boolean(envOverrides().hideLocalLogin || envOverrides()['PULSE_AUTH_HIDE_LOCAL_LOGIN']);
  const disableDockerUpdateActionsLocked = () =>
    Boolean(
      envOverrides().disableDockerUpdateActions ||
      envOverrides()['PULSE_DISABLE_DOCKER_UPDATE_ACTIONS'],
    );
  const disableLocalUpgradeMetricsLocked = () =>
    Boolean(
      envOverrides().disableLocalUpgradeMetrics ||
      envOverrides()['PULSE_DISABLE_LOCAL_UPGRADE_METRICS'],
    );
  const telemetryEnabledLocked = () =>
    Boolean(envOverrides().telemetryEnabled || envOverrides()['PULSE_TELEMETRY']);
  const pvePollingEnvLocked = () =>
    Boolean(envOverrides().pvePollingInterval || envOverrides().PVE_POLLING_INTERVAL);
  const backupPollingEnvLocked = () =>
    Boolean(envOverrides()['ENABLE_BACKUP_POLLING'] || envOverrides()['BACKUP_POLLING_INTERVAL']);

  const backupIntervalSelectValue = () =>
    getBackupIntervalSelectValue(backupPollingUseCustom(), backupPollingInterval());

  const backupIntervalSummary = () =>
    getBackupIntervalSummary(backupPollingEnabled(), backupPollingInterval());

  const initializeSystemSettingsState = async () => {
    try {
      const systemSettings = await SettingsAPI.getSystemSettings();
      const rawPVESecs =
        typeof systemSettings.pvePollingInterval === 'number'
          ? Math.round(systemSettings.pvePollingInterval)
          : PVE_POLLING_MIN_SECONDS;
      const clampedPVESecs = Math.min(
        PVE_POLLING_MAX_SECONDS,
        Math.max(PVE_POLLING_MIN_SECONDS, rawPVESecs),
      );

      setPVEPollingInterval(clampedPVESecs);
      const presetMatch = PVE_POLLING_PRESETS.find((opt) => opt.value === clampedPVESecs);
      if (presetMatch) {
        setPVEPollingSelection(presetMatch.value);
      } else {
        setPVEPollingSelection('custom');
        setPVEPollingCustomSeconds(clampedPVESecs);
      }

      setAllowedOrigins(systemSettings.allowedOrigins ?? '');
      setDiscoveryEnabled(systemSettings.discoveryEnabled ?? false);
      applySavedDiscoverySubnet(systemSettings.discoverySubnet);
      setAllowEmbedding(systemSettings.allowEmbedding ?? false);
      setAllowedEmbedOrigins(systemSettings.allowedEmbedOrigins || '');
      setWebhookAllowedPrivateCIDRs(systemSettings.webhookAllowedPrivateCIDRs || '');
      setPublicURL(systemSettings.publicURL || '');
      setTemperatureMonitoringEnabled(
        typeof systemSettings.temperatureMonitoringEnabled === 'boolean'
          ? systemSettings.temperatureMonitoringEnabled
          : true,
      );
      setHideLocalLogin(systemSettings.hideLocalLogin ?? false);
      setDisableDockerUpdateActions(systemSettings.disableDockerUpdateActions ?? false);
      setReduceProUpsellNoise(systemSettings.reduceProUpsellNoise ?? false);
      setDisableLocalUpgradeMetrics(systemSettings.disableLocalUpgradeMetrics ?? false);
      setTelemetryEnabled(systemSettings.telemetryEnabled ?? true);

      if (typeof systemSettings.backupPollingEnabled === 'boolean') {
        setBackupPollingEnabled(systemSettings.backupPollingEnabled);
      } else {
        setBackupPollingEnabled(true);
      }

      const intervalSeconds =
        typeof systemSettings.backupPollingInterval === 'number'
          ? Math.max(0, Math.floor(systemSettings.backupPollingInterval))
          : 0;
      setBackupPollingInterval(intervalSeconds);
      if (intervalSeconds > 0) {
        setBackupPollingCustomMinutes(Math.max(1, Math.round(intervalSeconds / 60)));
      }

      const isPresetInterval = BACKUP_INTERVAL_OPTIONS.some((opt) => opt.value === intervalSeconds);
      setBackupPollingUseCustom(!isPresetInterval && intervalSeconds > 0);
      const savedUpdateChannel =
        systemSettings.updateChannel === 'rc' ? ('rc' as const) : ('stable' as const);
      setUpdateChannel(savedUpdateChannel);
      setAutoUpdateEnabled(
        normalizeAutoUpdateEnabled(savedUpdateChannel, systemSettings.autoUpdateEnabled || false),
      );
      setAutoUpdateCheckInterval(systemSettings.autoUpdateCheckInterval ?? 24);
      setAutoUpdateTime(systemSettings.autoUpdateTime || '03:00');

      if (systemSettings.envOverrides) {
        setEnvOverrides(systemSettings.envOverrides);
      }
    } catch (error) {
      logger.error('Failed to load settings', error);
    }

    try {
      const cachedVersion = updateStore.versionInfo();
      if (cachedVersion) {
        setVersionInfo(cachedVersion);
      }

      await updateStore.checkForUpdates();
      const version = updateStore.versionInfo();
      if (version) {
        setVersionInfo(version);
      }

      const storeInfo = updateStore.updateInfo();
      if (storeInfo) {
        setUpdateInfo(storeInfo);
        if (storeInfo.available && storeInfo.latestVersion) {
          try {
            const plan = await UpdatesAPI.getUpdatePlan(storeInfo.latestVersion);
            setUpdatePlan(plan);
          } catch (planError) {
            logger.warn('Failed to fetch update plan on load', planError);
          }
        }
      }

      if (version?.channel && !updateChannel()) {
        setUpdateChannel(version.channel as 'stable' | 'rc');
      }
    } catch (error) {
      logger.error('Failed to load version', error);
    }
  };

  const saveSettings = async () => {
    const initiatingTab = activeTab();
    try {
      if (getSettingsTabSaveBehavior(initiatingTab) === 'system') {
        await SettingsAPI.updateSystemSettings({
          pvePollingInterval: pvePollingInterval(),
          allowedOrigins: allowedOrigins(),
          updateChannel: updateChannel(),
          autoUpdateEnabled: normalizeAutoUpdateEnabled(updateChannel(), autoUpdateEnabled()),
          autoUpdateCheckInterval: autoUpdateCheckInterval(),
          autoUpdateTime: autoUpdateTime(),
          backupPollingEnabled: backupPollingEnabled(),
          backupPollingInterval: backupPollingInterval(),
          allowEmbedding: allowEmbedding(),
          allowedEmbedOrigins: allowedEmbedOrigins(),
          webhookAllowedPrivateCIDRs: webhookAllowedPrivateCIDRs(),
          publicURL: publicURL(),
        });
      }

      const isNetworkTab = initiatingTab === 'system-network';
      notificationStore.success(
        isNetworkTab
          ? 'Settings saved successfully. Service restart may be required for network changes.'
          : 'Settings saved successfully.',
      );
      setHasUnsavedChanges(false);

      if (isNetworkTab) {
        setTimeout(() => {
          window.location.reload();
        }, 3000);
      }
    } catch (error) {
      notificationStore.error(
        getSystemSettingsSaveErrorMessage(error instanceof Error ? error.message : undefined),
      );
    }
  };

  const handleHideLocalLoginChange = async (enabled: boolean): Promise<void> => {
    if (hideLocalLoginLocked() || savingHideLocalLogin()) {
      return;
    }

    const previous = hideLocalLogin();
    setHideLocalLogin(enabled);
    setSavingHideLocalLogin(true);

    try {
      await SettingsAPI.updateSystemSettings({ hideLocalLogin: enabled });
      if (enabled) {
        notificationStore.success('Local login hidden', 2000);
      } else {
        notificationStore.info('Local login visible', 2000);
      }
      await loadSecurityStatus();
    } catch (error) {
      logger.error('Failed to update hide local login setting', error);
      notificationStore.error(
        getHideLocalLoginUpdateErrorMessage(error instanceof Error ? error.message : undefined),
      );
      setHideLocalLogin(previous);
    } finally {
      setSavingHideLocalLogin(false);
    }
  };

  const handleDisableDockerUpdateActionsChange = async (disabled: boolean): Promise<void> => {
    if (disableDockerUpdateActionsLocked() || savingDockerUpdateActions()) {
      return;
    }

    const previous = disableDockerUpdateActions();
    setDisableDockerUpdateActions(disabled);
    setSavingDockerUpdateActions(true);

    try {
      await SettingsAPI.updateSystemSettings({ disableDockerUpdateActions: disabled });
      updateDockerUpdateActionsSetting(disabled);

      if (disabled) {
        notificationStore.success('Docker update buttons hidden', 2000);
      } else {
        notificationStore.info('Docker update buttons visible', 2000);
      }
    } catch (error) {
      logger.error('Failed to update Docker update actions setting', error);
      notificationStore.error(
        getDockerUpdateActionsUpdateErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
      setDisableDockerUpdateActions(previous);
    } finally {
      setSavingDockerUpdateActions(false);
    }
  };

  const handleReduceProUpsellNoiseChange = async (enabled: boolean): Promise<void> => {
    if (savingReduceUpsells()) {
      return;
    }

    const previous = reduceProUpsellNoise();
    setReduceProUpsellNoise(enabled);
    setSavingReduceUpsells(true);

    try {
      await SettingsAPI.updateSystemSettings({ reduceProUpsellNoise: enabled });
      updateReduceProUpsellNoiseSetting(enabled);
      notificationStore.success(enabled ? 'Pro prompts reduced' : 'Pro prompts restored', 2000);
    } catch (error) {
      logger.error('Failed to update reduce upsell noise setting', error);
      notificationStore.error(
        getReduceUpsellNoiseUpdateErrorMessage(error instanceof Error ? error.message : undefined),
      );
      setReduceProUpsellNoise(previous);
    } finally {
      setSavingReduceUpsells(false);
    }
  };

  const handleDisableLocalUpgradeMetricsChange = async (disabled: boolean): Promise<void> => {
    if (disableLocalUpgradeMetricsLocked() || savingUpgradeMetrics()) {
      return;
    }

    const previous = disableLocalUpgradeMetrics();
    setDisableLocalUpgradeMetrics(disabled);
    setSavingUpgradeMetrics(true);

    try {
      await SettingsAPI.updateSystemSettings({ disableLocalUpgradeMetrics: disabled });
      updateDisableLocalUpgradeMetricsSetting(disabled);
      notificationStore.success(
        disabled ? 'Local upgrade metrics disabled' : 'Local upgrade metrics enabled',
        2000,
      );
    } catch (error) {
      logger.error('Failed to update local upgrade metrics setting', error);
      notificationStore.error(
        getLocalUpgradeMetricsUpdateErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
      setDisableLocalUpgradeMetrics(previous);
    } finally {
      setSavingUpgradeMetrics(false);
    }
  };

  const handleTelemetryEnabledChange = async (enabled: boolean): Promise<void> => {
    if (telemetryEnabledLocked() || savingTelemetry()) {
      return;
    }

    const previous = telemetryEnabled();
    setTelemetryEnabled(enabled);
    setSavingTelemetry(true);

    try {
      await SettingsAPI.updateSystemSettings({ telemetryEnabled: enabled });
      notificationStore.success(
        enabled ? 'Anonymous telemetry enabled' : 'Anonymous telemetry disabled',
        3000,
      );
    } catch (error) {
      logger.error('Failed to update telemetry setting', error);
      notificationStore.error(
        getTelemetryUpdateErrorMessage(error instanceof Error ? error.message : undefined),
      );
      setTelemetryEnabled(previous);
    } finally {
      setSavingTelemetry(false);
    }
  };

  const handleTemperatureMonitoringChange = async (enabled: boolean): Promise<void> => {
    if (temperatureMonitoringLocked() || savingTemperatureSetting()) {
      return;
    }

    const previous = temperatureMonitoringEnabled();
    setTemperatureMonitoringEnabled(enabled);
    setSavingTemperatureSetting(true);

    try {
      await SettingsAPI.updateSystemSettings({ temperatureMonitoringEnabled: enabled });
      if (enabled) {
        notificationStore.success('Temperature monitoring enabled', 2000);
      } else {
        notificationStore.info('Temperature monitoring disabled', 2000);
      }
    } catch (error) {
      logger.error('Failed to update temperature monitoring setting', error);
      notificationStore.error(
        getTemperatureMonitoringUpdateErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
      setTemperatureMonitoringEnabled(previous);
    } finally {
      setSavingTemperatureSetting(false);
    }
  };

  const checkForUpdates = async () => {
    setCheckingForUpdates(true);
    try {
      await updateStore.checkForUpdates(true);
      const info = updateStore.updateInfo();
      setUpdateInfo(info);

      if (info?.available && info.latestVersion) {
        try {
          const plan = await UpdatesAPI.getUpdatePlan(info.latestVersion);
          setUpdatePlan(plan);
        } catch (planError) {
          logger.warn('Failed to fetch update plan', planError);
          setUpdatePlan(null);
        }
      } else {
        setUpdatePlan(null);
      }

      if (info?.available && updateStore.isDismissed()) {
        updateStore.clearDismissed();
      }

      if (!info?.available) {
        notificationStore.success('You are running the latest version');
      }
    } catch (error) {
      notificationStore.error(getCheckForUpdatesErrorMessage());
      logger.error('Update check error', error);
    } finally {
      setCheckingForUpdates(false);
    }
  };

  const handleInstallUpdate = () => {
    setShowUpdateConfirmation(true);
  };

  const handleConfirmUpdate = async () => {
    const info = updateInfo();
    if (!info?.downloadUrl) {
      return;
    }

    setIsInstallingUpdate(true);
    try {
      await UpdatesAPI.applyUpdate(info.downloadUrl);
      setShowUpdateConfirmation(false);
    } catch (error) {
      logger.error('Failed to start update', error);
      notificationStore.error(getStartUpdateErrorMessage());
    } finally {
      setIsInstallingUpdate(false);
    }
  };

  return {
    hasUnsavedChanges,
    setHasUnsavedChanges,
    pvePollingInterval,
    setPVEPollingInterval,
    pvePollingSelection,
    setPVEPollingSelection,
    pvePollingCustomSeconds,
    setPVEPollingCustomSeconds,
    pvePollingEnvLocked,
    allowedOrigins,
    setAllowedOrigins,
    allowEmbedding,
    setAllowEmbedding,
    allowedEmbedOrigins,
    setAllowedEmbedOrigins,
    webhookAllowedPrivateCIDRs,
    setWebhookAllowedPrivateCIDRs,
    publicURL,
    setPublicURL,
    envOverrides,
    temperatureMonitoringEnabled,
    setTemperatureMonitoringEnabled,
    temperatureMonitoringLocked,
    savingTemperatureSetting,
    setSavingTemperatureSetting,
    handleTemperatureMonitoringChange,
    hideLocalLogin,
    hideLocalLoginLocked,
    savingHideLocalLogin,
    handleHideLocalLoginChange,
    disableDockerUpdateActions,
    disableDockerUpdateActionsLocked,
    savingDockerUpdateActions,
    handleDisableDockerUpdateActionsChange,
    reduceProUpsellNoise,
    savingReduceUpsells,
    handleReduceProUpsellNoiseChange,
    disableLocalUpgradeMetrics,
    disableLocalUpgradeMetricsLocked,
    savingUpgradeMetrics,
    handleDisableLocalUpgradeMetricsChange,
    telemetryEnabled,
    telemetryEnabledLocked,
    savingTelemetry,
    handleTelemetryEnabledChange,
    versionInfo,
    updateInfo,
    checkingForUpdates,
    updateChannel,
    setUpdateChannel,
    autoUpdateEnabled,
    setAutoUpdateEnabled,
    autoUpdateCheckInterval,
    setAutoUpdateCheckInterval,
    autoUpdateTime,
    setAutoUpdateTime,
    updatePlan,
    isInstallingUpdate,
    showUpdateConfirmation,
    setShowUpdateConfirmation,
    backupPollingEnabled,
    setBackupPollingEnabled,
    backupPollingInterval,
    setBackupPollingInterval,
    backupPollingCustomMinutes,
    setBackupPollingCustomMinutes,
    backupPollingUseCustom,
    setBackupPollingUseCustom,
    backupPollingEnvLocked,
    backupIntervalSelectValue,
    backupIntervalSummary,
    initializeSystemSettingsState,
    saveSettings,
    checkForUpdates,
    handleInstallUpdate,
    handleConfirmUpdate,
  };
}
