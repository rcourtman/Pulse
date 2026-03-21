import { Accessor, Component, Show } from 'solid-js';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { GeneralSettingsPanel, type GeneralSettingsPanelProps } from './GeneralSettingsPanel';
import type { NetworkSettingsPanelProps } from './networkSettingsModel';
import type { RecoverySettingsPanelProps } from './RecoverySettingsPanel';
import type { UpdatesSettingsPanelProps } from './UpdatesSettingsPanel';
import { getSettingsConfigurationLoadingState } from '@/utils/settingsShellPresentation';
import type { useBackupTransferFlow } from './useBackupTransferFlow';
import type { useDiscoverySettingsState } from './useDiscoverySettingsState';
import type { useSystemSettingsState } from './useSystemSettingsState';

interface DiscoverySettingsPanelState extends ReturnType<typeof useDiscoverySettingsState> {
  commitDiscoverySubnet: (rawValue: string) => Promise<boolean>;
  handleDiscoveryEnabledChange: (enabled: boolean) => Promise<boolean>;
  handleDiscoveryModeChange: (mode: 'auto' | 'custom') => Promise<void>;
}

interface UseSettingsSystemPanelsParams {
  darkMode: Accessor<boolean>;
  themePreference: Accessor<'light' | 'dark' | 'system'>;
  setThemePreference: (pref: 'light' | 'dark' | 'system') => void;
  initialLoadComplete: Accessor<boolean>;
  systemSettings: ReturnType<typeof useSystemSettingsState>;
  discoverySettings: DiscoverySettingsPanelState;
  backupTransferFlow: ReturnType<typeof useBackupTransferFlow>;
  securityStatus: Accessor<SecurityStatusInfo | null>;
}

export interface SettingsSystemPanels {
  systemGeneralPanel: Component;
  getNetworkPanelProps: () => NetworkSettingsPanelProps;
  getRecoveryPanelProps: () => RecoverySettingsPanelProps;
  getUpdatesPanelProps: () => UpdatesSettingsPanelProps;
}

export function useSettingsSystemPanels(
  params: UseSettingsSystemPanelsParams,
): SettingsSystemPanels {
  const systemGeneralPanelProps = (): GeneralSettingsPanelProps => ({
    darkMode: params.darkMode,
    themePreference: params.themePreference,
    setThemePreference: params.setThemePreference,
    pvePollingInterval: params.systemSettings.pvePollingInterval,
    setPVEPollingInterval: params.systemSettings.setPVEPollingInterval,
    pvePollingSelection: params.systemSettings.pvePollingSelection,
    setPVEPollingSelection: params.systemSettings.setPVEPollingSelection,
    pvePollingCustomSeconds: params.systemSettings.pvePollingCustomSeconds,
    setPVEPollingCustomSeconds: params.systemSettings.setPVEPollingCustomSeconds,
    pvePollingEnvLocked: params.systemSettings.pvePollingEnvLocked,
    setHasUnsavedChanges: params.systemSettings.setHasUnsavedChanges,
    disableLocalUpgradeMetrics: params.systemSettings.disableLocalUpgradeMetrics,
    disableLocalUpgradeMetricsLocked: params.systemSettings.disableLocalUpgradeMetricsLocked,
    savingUpgradeMetrics: params.systemSettings.savingUpgradeMetrics,
    handleDisableLocalUpgradeMetricsChange:
      params.systemSettings.handleDisableLocalUpgradeMetricsChange,
    telemetryEnabled: params.systemSettings.telemetryEnabled,
    telemetryEnabledLocked: params.systemSettings.telemetryEnabledLocked,
    savingTelemetry: params.systemSettings.savingTelemetry,
    handleTelemetryEnabledChange: params.systemSettings.handleTelemetryEnabledChange,
    disableDockerUpdateActions: params.systemSettings.disableDockerUpdateActions,
    disableDockerUpdateActionsLocked: params.systemSettings.disableDockerUpdateActionsLocked,
    savingDockerUpdateActions: params.systemSettings.savingDockerUpdateActions,
    handleDisableDockerUpdateActionsChange:
      params.systemSettings.handleDisableDockerUpdateActionsChange,
  });

  const systemGeneralPanel: Component = () => (
    <>
      <Show when={!params.initialLoadComplete()}>
        <div class="flex items-center justify-center rounded-md border border-dashed border-border bg-surface-alt py-12 text-sm text-muted">
          {getSettingsConfigurationLoadingState().text}
        </div>
      </Show>
      <Show when={params.initialLoadComplete()}>
        <GeneralSettingsPanel {...systemGeneralPanelProps()} />
      </Show>
    </>
  );

  return {
    systemGeneralPanel,
    getNetworkPanelProps: () => ({
      discoveryEnabled: params.discoverySettings.discoveryEnabled,
      discoveryMode: params.discoverySettings.discoveryMode,
      discoverySubnetDraft: params.discoverySettings.discoverySubnetDraft,
      discoverySubnetError: params.discoverySettings.discoverySubnetError,
      savingDiscoverySettings: params.discoverySettings.savingDiscoverySettings,
      envOverrides: params.systemSettings.envOverrides,
      allowedOrigins: params.systemSettings.allowedOrigins,
      setAllowedOrigins: params.systemSettings.setAllowedOrigins,
      allowEmbedding: params.systemSettings.allowEmbedding,
      setAllowEmbedding: params.systemSettings.setAllowEmbedding,
      allowedEmbedOrigins: params.systemSettings.allowedEmbedOrigins,
      setAllowedEmbedOrigins: params.systemSettings.setAllowedEmbedOrigins,
      webhookAllowedPrivateCIDRs: params.systemSettings.webhookAllowedPrivateCIDRs,
      setWebhookAllowedPrivateCIDRs: params.systemSettings.setWebhookAllowedPrivateCIDRs,
      publicURL: params.systemSettings.publicURL,
      setPublicURL: params.systemSettings.setPublicURL,
      handleDiscoveryEnabledChange: params.discoverySettings.handleDiscoveryEnabledChange,
      handleDiscoveryModeChange: params.discoverySettings.handleDiscoveryModeChange,
      setDiscoveryMode: params.discoverySettings.setDiscoveryMode,
      setDiscoverySubnetDraft: params.discoverySettings.setDiscoverySubnetDraft,
      setDiscoverySubnetError: params.discoverySettings.setDiscoverySubnetError,
      setLastCustomSubnet: params.discoverySettings.setLastCustomSubnet,
      commitDiscoverySubnet: params.discoverySettings.commitDiscoverySubnet,
      setHasUnsavedChanges: params.systemSettings.setHasUnsavedChanges,
      parseSubnetList: params.discoverySettings.parseSubnetList,
      normalizeSubnetList: params.discoverySettings.normalizeSubnetList,
      isValidCIDR: params.discoverySettings.isValidCIDR,
      currentDraftSubnetValue: params.discoverySettings.currentDraftSubnetValue,
      discoverySubnetInputRef: params.discoverySettings.assignDiscoverySubnetInputRef,
    }),
    getUpdatesPanelProps: () => ({
      versionInfo: params.systemSettings.versionInfo,
      updateInfo: params.systemSettings.updateInfo,
      checkingForUpdates: params.systemSettings.checkingForUpdates,
      updateChannel: params.systemSettings.updateChannel,
      setUpdateChannel: params.systemSettings.setUpdateChannel,
      autoUpdateEnabled: params.systemSettings.autoUpdateEnabled,
      setAutoUpdateEnabled: params.systemSettings.setAutoUpdateEnabled,
      autoUpdateCheckInterval: params.systemSettings.autoUpdateCheckInterval,
      setAutoUpdateCheckInterval: params.systemSettings.setAutoUpdateCheckInterval,
      autoUpdateTime: params.systemSettings.autoUpdateTime,
      setAutoUpdateTime: params.systemSettings.setAutoUpdateTime,
      checkForUpdates: params.systemSettings.checkForUpdates,
      setHasUnsavedChanges: params.systemSettings.setHasUnsavedChanges,
      updatePlan: params.systemSettings.updatePlan,
      onInstallUpdate: params.systemSettings.handleInstallUpdate,
      isInstalling: params.systemSettings.isInstallingUpdate,
    }),
    getRecoveryPanelProps: () => ({
      backupPollingEnabled: params.systemSettings.backupPollingEnabled,
      setBackupPollingEnabled: params.systemSettings.setBackupPollingEnabled,
      backupPollingInterval: params.systemSettings.backupPollingInterval,
      setBackupPollingInterval: params.systemSettings.setBackupPollingInterval,
      backupPollingCustomMinutes: params.systemSettings.backupPollingCustomMinutes,
      setBackupPollingCustomMinutes: params.systemSettings.setBackupPollingCustomMinutes,
      backupPollingUseCustom: params.systemSettings.backupPollingUseCustom,
      setBackupPollingUseCustom: params.systemSettings.setBackupPollingUseCustom,
      backupPollingEnvLocked: params.systemSettings.backupPollingEnvLocked,
      backupIntervalSelectValue: params.systemSettings.backupIntervalSelectValue,
      backupIntervalSummary: params.systemSettings.backupIntervalSummary,
      setHasUnsavedChanges: params.systemSettings.setHasUnsavedChanges,
      showExportDialog: params.backupTransferFlow.showExportDialog,
      setShowExportDialog: params.backupTransferFlow.setShowExportDialog,
      showImportDialog: params.backupTransferFlow.showImportDialog,
      setShowImportDialog: params.backupTransferFlow.setShowImportDialog,
      setUseCustomPassphrase: params.backupTransferFlow.setUseCustomPassphrase,
      securityStatus: params.securityStatus,
    }),
  };
}
