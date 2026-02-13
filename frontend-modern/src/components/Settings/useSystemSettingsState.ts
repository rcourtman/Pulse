import { Accessor, Setter, createEffect, createSignal, onCleanup } from 'solid-js';
import { SettingsAPI } from '@/api/settings';
import { UpdatesAPI } from '@/api/updates';
import type { UpdateInfo, UpdatePlan, VersionInfo } from '@/api/updates';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { apiFetch } from '@/utils/apiClient';
import { updateStore } from '@/stores/updates';
import { updateDockerUpdateActionsSetting } from '@/stores/systemSettings';
import type { SettingsTab } from './settingsTypes';

const BACKUP_INTERVAL_OPTIONS = [
  { label: 'Default (~90 seconds)', value: 0 },
  { label: '15 minutes', value: 15 * 60 },
  { label: '30 minutes', value: 30 * 60 },
  { label: '1 hour', value: 60 * 60 },
  { label: '6 hours', value: 6 * 60 * 60 },
  { label: '12 hours', value: 12 * 60 * 60 },
  { label: '24 hours', value: 24 * 60 * 60 },
];

const PVE_POLLING_MIN_SECONDS = 10;
const PVE_POLLING_MAX_SECONDS = 3600;
const PVE_POLLING_PRESETS = [
  { label: '10 seconds (default)', value: 10 },
  { label: '15 seconds', value: 15 },
  { label: '30 seconds', value: 30 },
  { label: '60 seconds', value: 60 },
  { label: '2 minutes', value: 120 },
  { label: '5 minutes', value: 300 },
];

interface DiagnosticsNode {
  id: string;
  name: string;
  host: string;
  type: string;
  authMethod: string;
  connected: boolean;
  error?: string;
  details?: Record<string, unknown>;
  lastPoll?: string;
  clusterInfo?: Record<string, unknown>;
}

interface DiagnosticsPBS {
  id: string;
  name: string;
  host: string;
  connected: boolean;
  error?: string;
  details?: Record<string, unknown>;
}

interface SystemDiagnostic {
  os: string;
  arch: string;
  goVersion: string;
  numCPU: number;
  numGoroutine: number;
  memoryMB: number;
}

interface APITokenSummary {
  id: string;
  name: string;
  hint?: string;
  createdAt?: string;
  lastUsedAt?: string;
  source?: string;
}

interface APITokenUsage {
  tokenId: string;
  hostCount: number;
  hosts?: string[];
}

interface APITokenDiagnostic {
  enabled: boolean;
  tokenCount: number;
  hasEnvTokens: boolean;
  hasLegacyToken: boolean;
  recommendTokenSetup: boolean;
  recommendTokenRotation: boolean;
  legacyDockerHostCount?: number;
  unusedTokenCount?: number;
  notes?: string[];
  tokens?: APITokenSummary[];
  usage?: APITokenUsage[];
}

interface DockerAgentAttention {
  hostId: string;
  name: string;
  status: string;
  agentVersion?: string;
  tokenHint?: string;
  lastSeen?: string;
  issues: string[];
}

interface DockerAgentDiagnostic {
  hostsTotal: number;
  hostsOnline: number;
  hostsReportingVersion: number;
  hostsWithTokenBinding: number;
  hostsWithoutTokenBinding: number;
  hostsWithoutVersion?: number;
  hostsOutdatedVersion?: number;
  hostsWithStaleCommand?: number;
  hostsPendingUninstall?: number;
  hostsNeedingAttention: number;
  recommendedAgentVersion?: string;
  attention?: DockerAgentAttention[];
  notes?: string[];
}

interface DiscoveryDiagnostic {
  enabled: boolean;
  configuredSubnet?: string;
  activeSubnet?: string;
  environmentOverride?: string;
  subnetAllowlist?: string[];
  subnetBlocklist?: string[];
  scanning?: boolean;
  scanInterval?: string;
  lastScanStartedAt?: string;
  lastResultTimestamp?: string;
  lastResultServers?: number;
  lastResultErrors?: number;
}

interface AlertsDiagnostic {
  legacyThresholdsDetected: boolean;
  legacyThresholdSources?: string[];
  legacyScheduleSettings?: string[];
  missingCooldown: boolean;
  missingGroupingWindow: boolean;
  notes?: string[];
}

interface DiagnosticsData {
  version: string;
  runtime: string;
  uptime: number;
  nodes: DiagnosticsNode[];
  pbs: DiagnosticsPBS[];
  system: SystemDiagnostic;
  apiTokens?: APITokenDiagnostic | null;
  dockerAgents?: DockerAgentDiagnostic | null;
  alerts?: AlertsDiagnostic | null;
  discovery?: DiscoveryDiagnostic | null;
  errors: string[];
}

interface UseSystemSettingsStateParams {
  activeTab: Accessor<SettingsTab>;
  currentTab: Accessor<SettingsTab>;
  loadSecurityStatus: () => Promise<void>;
  setDiscoveryEnabled: Setter<boolean>;
  applySavedDiscoverySubnet: (subnet?: string | null) => void;
}

export function useSystemSettingsState({
  activeTab,
  currentTab,
  loadSecurityStatus,
  setDiscoveryEnabled,
  applySavedDiscoverySubnet,
}: UseSystemSettingsStateParams) {
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
  const [_diagnosticsData, setDiagnosticsData] = createSignal<DiagnosticsData | null>(null);
  const [_runningDiagnostics, setRunningDiagnostics] = createSignal(false);

  const temperatureMonitoringLocked = () =>
    Boolean(
      envOverrides().temperatureMonitoringEnabled || envOverrides()['ENABLE_TEMPERATURE_MONITORING'],
    );
  const hideLocalLoginLocked = () =>
    Boolean(envOverrides().hideLocalLogin || envOverrides()['PULSE_AUTH_HIDE_LOCAL_LOGIN']);
  const disableDockerUpdateActionsLocked = () =>
    Boolean(envOverrides().disableDockerUpdateActions || envOverrides()['PULSE_DISABLE_DOCKER_UPDATE_ACTIONS']);
  const pvePollingEnvLocked = () =>
    Boolean(envOverrides().pvePollingInterval || envOverrides().PVE_POLLING_INTERVAL);
  const backupPollingEnvLocked = () =>
    Boolean(envOverrides()['ENABLE_BACKUP_POLLING'] || envOverrides()['BACKUP_POLLING_INTERVAL']);

  const backupIntervalSelectValue = () => {
    if (backupPollingUseCustom()) {
      return 'custom';
    }
    const seconds = backupPollingInterval();
    return BACKUP_INTERVAL_OPTIONS.some((option) => option.value === seconds)
      ? String(seconds)
      : 'custom';
  };

  const backupIntervalSummary = () => {
    if (!backupPollingEnabled()) {
      return 'Backup polling is disabled.';
    }

    const seconds = backupPollingInterval();
    if (seconds <= 0) {
      return 'Pulse checks backups and snapshots at the default cadence (~every 90 seconds).';
    }
    if (seconds % 86400 === 0) {
      const days = seconds / 86400;
      return `Pulse checks backups every ${days === 1 ? 'day' : `${days} days`}.`;
    }
    if (seconds % 3600 === 0) {
      const hours = seconds / 3600;
      return `Pulse checks backups every ${hours === 1 ? 'hour' : `${hours} hours`}.`;
    }

    const minutes = Math.max(1, Math.round(seconds / 60));
    return `Pulse checks backups every ${minutes === 1 ? 'minute' : `${minutes} minutes`}.`;
  };

  const runDiagnostics = async () => {
    setRunningDiagnostics(true);
    try {
      const response = await apiFetch('/api/diagnostics');
      const diag = await response.json();
      setDiagnosticsData(diag);
    } catch (err) {
      logger.error('Failed to fetch diagnostics', err);
      notificationStore.error('Failed to run diagnostics');
    } finally {
      setRunningDiagnostics(false);
    }
  };

  createEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    if (currentTab() !== 'proxmox') {
      return;
    }

    void runDiagnostics();
    const intervalId = window.setInterval(() => {
      void runDiagnostics();
    }, 60000);

    onCleanup(() => {
      window.clearInterval(intervalId);
    });
  });

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
      setAutoUpdateEnabled(systemSettings.autoUpdateEnabled || false);
      setAutoUpdateCheckInterval(systemSettings.autoUpdateCheckInterval ?? 24);
      setAutoUpdateTime(systemSettings.autoUpdateTime || '03:00');

      if (systemSettings.updateChannel) {
        setUpdateChannel(systemSettings.updateChannel as 'stable' | 'rc');
      }

      if (systemSettings.envOverrides) {
        setEnvOverrides(systemSettings.envOverrides);
      }
    } catch (error) {
      logger.error('Failed to load settings', error);
    }

    try {
      const version = await UpdatesAPI.getVersion();
      setVersionInfo(version);
      await updateStore.checkForUpdates();

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

      if (version.channel && !updateChannel()) {
        setUpdateChannel(version.channel as 'stable' | 'rc');
      }
    } catch (error) {
      logger.error('Failed to load version', error);
    }
  };

  const saveSettings = async () => {
    try {
      if (
        activeTab() === 'system-general' ||
        activeTab() === 'system-network' ||
        activeTab() === 'system-updates' ||
        activeTab() === 'system-backups'
      ) {
        await SettingsAPI.updateSystemSettings({
          pvePollingInterval: pvePollingInterval(),
          allowedOrigins: allowedOrigins(),
          updateChannel: updateChannel(),
          autoUpdateEnabled: autoUpdateEnabled(),
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

      notificationStore.success(
        'Settings saved successfully. Service restart may be required for port changes.',
      );
      setHasUnsavedChanges(false);

      setTimeout(() => {
        window.location.reload();
      }, 3000);
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : 'Failed to save settings');
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
        error instanceof Error ? error.message : 'Failed to update hide local login setting',
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
        error instanceof Error ? error.message : 'Failed to update Docker update actions setting',
      );
      setDisableDockerUpdateActions(previous);
    } finally {
      setSavingDockerUpdateActions(false);
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
        error instanceof Error ? error.message : 'Failed to update temperature monitoring setting',
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
      notificationStore.error('Failed to check for updates');
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
      notificationStore.error('Failed to start update. Please try again.');
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
    _diagnosticsData,
    _runningDiagnostics,
    initializeSystemSettingsState,
    saveSettings,
    checkForUpdates,
    handleInstallUpdate,
    handleConfirmUpdate,
  };
}
