import { Accessor, Component, Setter, Show, createMemo } from 'solid-js';
import type { UpdateInfo, UpdatePlan, VersionInfo } from '@/api/updates';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { Card } from '@/components/shared/Card';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { AISettings } from './AISettings';
import { AICostDashboard } from '@/components/AI/AICostDashboard';
import { GeneralSettingsPanel } from './GeneralSettingsPanel';
import { ProLicensePanel } from './ProLicensePanel';
import { AgentLedgerPanel } from './AgentLedgerPanel';
import { SSOProvidersPanel } from './SSOProvidersPanel';
import { UnifiedAgents } from './UnifiedAgents';
import { createSettingsPanelRegistry } from './settingsPanelRegistry';

interface UseSettingsPanelRegistryParams {
  darkMode: Accessor<boolean>;
  themePreference: Accessor<'light' | 'dark' | 'system'>;
  setThemePreference: (pref: 'light' | 'dark' | 'system') => void;
  initialLoadComplete: Accessor<boolean>;
  pvePollingInterval: Accessor<number>;
  setPVEPollingInterval: Setter<number>;
  pvePollingSelection: Accessor<number | 'custom'>;
  setPVEPollingSelection: Setter<number | 'custom'>;
  pvePollingCustomSeconds: Accessor<number>;
  setPVEPollingCustomSeconds: Setter<number>;
  pvePollingEnvLocked: Accessor<boolean>;
  setHasUnsavedChanges: Setter<boolean>;
  disableLocalUpgradeMetrics: Accessor<boolean>;
  disableLocalUpgradeMetricsLocked: Accessor<boolean>;
  savingUpgradeMetrics: Accessor<boolean>;
  handleDisableLocalUpgradeMetricsChange: (disabled: boolean) => Promise<void>;
  telemetryEnabled: Accessor<boolean>;
  telemetryEnabledLocked: Accessor<boolean>;
  savingTelemetry: Accessor<boolean>;
  handleTelemetryEnabledChange: (enabled: boolean) => Promise<void>;
  disableDockerUpdateActions: Accessor<boolean>;
  disableDockerUpdateActionsLocked: Accessor<boolean>;
  savingDockerUpdateActions: Accessor<boolean>;
  handleDisableDockerUpdateActionsChange: (disabled: boolean) => Promise<void>;
  discoveryEnabled: Accessor<boolean>;
  discoveryMode: Accessor<'auto' | 'custom'>;
  discoverySubnetDraft: Accessor<string>;
  discoverySubnetError: Accessor<string | undefined>;
  savingDiscoverySettings: Accessor<boolean>;
  envOverrides: Accessor<Record<string, boolean>>;
  allowedOrigins: Accessor<string>;
  setAllowedOrigins: Setter<string>;
  allowEmbedding: Accessor<boolean>;
  setAllowEmbedding: Setter<boolean>;
  allowedEmbedOrigins: Accessor<string>;
  setAllowedEmbedOrigins: Setter<string>;
  webhookAllowedPrivateCIDRs: Accessor<string>;
  setWebhookAllowedPrivateCIDRs: Setter<string>;
  publicURL: Accessor<string>;
  setPublicURL: Setter<string>;
  handleDiscoveryEnabledChange: (enabled: boolean) => Promise<boolean>;
  handleDiscoveryModeChange: (mode: 'auto' | 'custom') => Promise<void>;
  setDiscoveryMode: Setter<'auto' | 'custom'>;
  setDiscoverySubnetDraft: Setter<string>;
  setDiscoverySubnetError: Setter<string | undefined>;
  setLastCustomSubnet: Setter<string>;
  commitDiscoverySubnet: (rawValue: string) => Promise<boolean>;
  parseSubnetList: (value: string) => string[];
  normalizeSubnetList: (value: string) => string;
  isValidCIDR: (value: string) => boolean;
  currentDraftSubnetValue: () => string;
  assignDiscoverySubnetInputRef: (el: HTMLInputElement) => void;
  versionInfo: Accessor<VersionInfo | null>;
  updateInfo: Accessor<UpdateInfo | null>;
  checkingForUpdates: Accessor<boolean>;
  updateChannel: Accessor<'stable' | 'rc'>;
  setUpdateChannel: Setter<'stable' | 'rc'>;
  autoUpdateEnabled: Accessor<boolean>;
  setAutoUpdateEnabled: Setter<boolean>;
  autoUpdateCheckInterval: Accessor<number>;
  setAutoUpdateCheckInterval: Setter<number>;
  autoUpdateTime: Accessor<string>;
  setAutoUpdateTime: Setter<string>;
  checkForUpdates: () => Promise<void>;
  updatePlan: Accessor<UpdatePlan | null>;
  handleInstallUpdate: () => Promise<void>;
  isInstallingUpdate: Accessor<boolean>;
  backupPollingEnabled: Accessor<boolean>;
  setBackupPollingEnabled: Setter<boolean>;
  backupPollingInterval: Accessor<number>;
  setBackupPollingInterval: Setter<number>;
  backupPollingCustomMinutes: Accessor<number>;
  setBackupPollingCustomMinutes: Setter<number>;
  backupPollingUseCustom: Accessor<boolean>;
  setBackupPollingUseCustom: Setter<boolean>;
  backupPollingEnvLocked: Accessor<boolean>;
  backupIntervalSelectValue: () => string;
  backupIntervalSummary: () => string;
  showExportDialog: Accessor<boolean>;
  setShowExportDialog: Setter<boolean>;
  showImportDialog: Accessor<boolean>;
  setShowImportDialog: Setter<boolean>;
  setUseCustomPassphrase: Setter<boolean>;
  securityStatus: Accessor<SecurityStatusInfo | null>;
  securityStatusLoading: Accessor<boolean>;
  organizationAgentUsage: Accessor<number>;
  organizationGuestUsage: Accessor<number>;
  loadSecurityStatus: () => Promise<void>;
  showQuickSecuritySetup: Accessor<boolean>;
  setShowQuickSecuritySetup: Setter<boolean>;
  showQuickSecurityWizard: Accessor<boolean>;
  setShowQuickSecurityWizard: Setter<boolean>;
  showPasswordModal: Accessor<boolean>;
  setShowPasswordModal: Setter<boolean>;
  hideLocalLogin: Accessor<boolean>;
  hideLocalLoginLocked: Accessor<boolean>;
  savingHideLocalLogin: Accessor<boolean>;
  handleHideLocalLoginChange: (enabled: boolean) => Promise<void>;
}

export function useSettingsPanelRegistry(params: UseSettingsPanelRegistryParams) {
  const settingsCapabilities = createMemo(() => params.securityStatus()?.settingsCapabilities ?? null);

  const agentsPanel: Component = () => (
    <>
      <UnifiedAgents />
      <AgentProfilesPanel />
    </>
  );

  const dockerPanel: Component = () => (
    <Card padding="lg" class="mb-6">
      <div class="space-y-4">
        <div class="space-y-1">
          <h3 class="text-base font-semibold text-base-content">Docker Settings</h3>
          <p class="text-sm text-muted">Server-wide settings for Docker container management.</p>
        </div>

        <div class="flex items-start justify-between gap-4 p-4 rounded-md border border-border bg-surface-hover">
          <div class="flex-1 space-y-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-base-content">Hide Docker Update Buttons</span>
              <Show when={params.disableDockerUpdateActionsLocked()}>
                <span
                  class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300"
                  title="Locked by environment variable PULSE_DISABLE_DOCKER_UPDATE_ACTIONS"
                >
                  <svg
                    class="w-3 h-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                    />
                  </svg>
                  ENV
                </span>
              </Show>
            </div>
            <p class="text-xs text-muted">
              When enabled, the "Update" button on Docker containers will be hidden across all
              views. Update detection will still work, allowing you to see which containers have
              updates available. Use this in production environments where you prefer Pulse to be
              read-only.
            </p>
            <p class="text-xs text-muted mt-1">
              Can also be set via environment variable:{' '}
              <code class="px-1 py-0.5 rounded bg-surface-hover text-base-content">
                PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=true
              </code>
            </p>
          </div>
          <div class="flex-shrink-0">
            <button
              type="button"
              onClick={() =>
                params.handleDisableDockerUpdateActionsChange(!params.disableDockerUpdateActions())
              }
              disabled={
                params.disableDockerUpdateActionsLocked() || params.savingDockerUpdateActions()
              }
              class={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${
                params.disableDockerUpdateActions() ? 'bg-blue-600' : 'bg-surface-alt'
              } ${params.disableDockerUpdateActionsLocked() ? 'opacity-50 cursor-not-allowed' : ''}`}
              role="switch"
              aria-checked={params.disableDockerUpdateActions()}
              title={
                params.disableDockerUpdateActionsLocked()
                  ? 'Locked by environment variable'
                  : undefined
              }
            >
              <span
                class={`inline-block h-4 w-4 transform rounded-full bg-white shadow-sm transition-transform ${
                  params.disableDockerUpdateActions() ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
          </div>
        </div>
      </div>
    </Card>
  );

  const systemAiPanel: Component = () => (
    <div class="space-y-6">
      <AISettings />
      <AICostDashboard />
    </div>
  );

  const systemGeneralPanel: Component = () => (
    <>
      <Show when={!params.initialLoadComplete()}>
        <div class="flex items-center justify-center rounded-md border border-dashed border-border bg-surface-alt py-12 text-sm text-muted">
          Loading configuration...
        </div>
      </Show>
      <Show when={params.initialLoadComplete()}>
        <GeneralSettingsPanel
          darkMode={params.darkMode}
          themePreference={params.themePreference}
          setThemePreference={params.setThemePreference}
          pvePollingInterval={params.pvePollingInterval}
          setPVEPollingInterval={params.setPVEPollingInterval}
          pvePollingSelection={params.pvePollingSelection}
          setPVEPollingSelection={params.setPVEPollingSelection}
          pvePollingCustomSeconds={params.pvePollingCustomSeconds}
          setPVEPollingCustomSeconds={params.setPVEPollingCustomSeconds}
          pvePollingEnvLocked={params.pvePollingEnvLocked}
          setHasUnsavedChanges={params.setHasUnsavedChanges}
          disableLocalUpgradeMetrics={params.disableLocalUpgradeMetrics}
          disableLocalUpgradeMetricsLocked={params.disableLocalUpgradeMetricsLocked}
          savingUpgradeMetrics={params.savingUpgradeMetrics}
          handleDisableLocalUpgradeMetricsChange={params.handleDisableLocalUpgradeMetricsChange}
          telemetryEnabled={params.telemetryEnabled}
          telemetryEnabledLocked={params.telemetryEnabledLocked}
          savingTelemetry={params.savingTelemetry}
          handleTelemetryEnabledChange={params.handleTelemetryEnabledChange}
        />
      </Show>
    </>
  );

  const systemProPanel: Component = () => (
    <div class="space-y-6">
      <ProLicensePanel />
      <AgentLedgerPanel />
    </div>
  );

  const securitySsoPanel: Component = () => (
    <div class="space-y-6">
      <SSOProvidersPanel
        onConfigUpdated={params.loadSecurityStatus}
        canManage={settingsCapabilities()?.singleSignOnWrite === true}
      />
    </div>
  );

  return createMemo(() =>
    createSettingsPanelRegistry({
      agentsPanel,
      dockerPanel,
      systemGeneralPanel,
      systemAiPanel,
      systemProPanel,
      securitySsoPanel,
      getNetworkPanelProps: () => ({
        discoveryEnabled: params.discoveryEnabled,
        discoveryMode: params.discoveryMode,
        discoverySubnetDraft: params.discoverySubnetDraft,
        discoverySubnetError: params.discoverySubnetError,
        savingDiscoverySettings: params.savingDiscoverySettings,
        envOverrides: params.envOverrides,
        allowedOrigins: params.allowedOrigins,
        setAllowedOrigins: params.setAllowedOrigins,
        allowEmbedding: params.allowEmbedding,
        setAllowEmbedding: params.setAllowEmbedding,
        allowedEmbedOrigins: params.allowedEmbedOrigins,
        setAllowedEmbedOrigins: params.setAllowedEmbedOrigins,
        webhookAllowedPrivateCIDRs: params.webhookAllowedPrivateCIDRs,
        setWebhookAllowedPrivateCIDRs: params.setWebhookAllowedPrivateCIDRs,
        publicURL: params.publicURL,
        setPublicURL: params.setPublicURL,
        handleDiscoveryEnabledChange: params.handleDiscoveryEnabledChange,
        handleDiscoveryModeChange: params.handleDiscoveryModeChange,
        setDiscoveryMode: params.setDiscoveryMode,
        setDiscoverySubnetDraft: params.setDiscoverySubnetDraft,
        setDiscoverySubnetError: params.setDiscoverySubnetError,
        setLastCustomSubnet: params.setLastCustomSubnet,
        commitDiscoverySubnet: params.commitDiscoverySubnet,
        setHasUnsavedChanges: params.setHasUnsavedChanges,
        parseSubnetList: params.parseSubnetList,
        normalizeSubnetList: params.normalizeSubnetList,
        isValidCIDR: params.isValidCIDR,
        currentDraftSubnetValue: params.currentDraftSubnetValue,
        discoverySubnetInputRef: params.assignDiscoverySubnetInputRef,
      }),
      getUpdatesPanelProps: () => ({
        versionInfo: params.versionInfo,
        updateInfo: params.updateInfo,
        checkingForUpdates: params.checkingForUpdates,
        updateChannel: params.updateChannel,
        setUpdateChannel: params.setUpdateChannel,
        autoUpdateEnabled: params.autoUpdateEnabled,
        setAutoUpdateEnabled: params.setAutoUpdateEnabled,
        autoUpdateCheckInterval: params.autoUpdateCheckInterval,
        setAutoUpdateCheckInterval: params.setAutoUpdateCheckInterval,
        autoUpdateTime: params.autoUpdateTime,
        setAutoUpdateTime: params.setAutoUpdateTime,
        checkForUpdates: params.checkForUpdates,
        setHasUnsavedChanges: params.setHasUnsavedChanges,
        updatePlan: params.updatePlan,
        onInstallUpdate: params.handleInstallUpdate,
        isInstalling: params.isInstallingUpdate,
      }),
      getRecoveryPanelProps: () => ({
        backupPollingEnabled: params.backupPollingEnabled,
        setBackupPollingEnabled: params.setBackupPollingEnabled,
        backupPollingInterval: params.backupPollingInterval,
        setBackupPollingInterval: params.setBackupPollingInterval,
        backupPollingCustomMinutes: params.backupPollingCustomMinutes,
        setBackupPollingCustomMinutes: params.setBackupPollingCustomMinutes,
        backupPollingUseCustom: params.backupPollingUseCustom,
        setBackupPollingUseCustom: params.setBackupPollingUseCustom,
        backupPollingEnvLocked: params.backupPollingEnvLocked,
        backupIntervalSelectValue: params.backupIntervalSelectValue,
        backupIntervalSummary: params.backupIntervalSummary,
        setHasUnsavedChanges: params.setHasUnsavedChanges,
        showExportDialog: params.showExportDialog,
        setShowExportDialog: params.setShowExportDialog,
        showImportDialog: params.showImportDialog,
        setShowImportDialog: params.setShowImportDialog,
        setUseCustomPassphrase: params.setUseCustomPassphrase,
        securityStatus: params.securityStatus,
      }),
      getOrganizationOverviewPanelProps: () => ({}),
      getOrganizationAccessPanelProps: () => ({}),
      getOrganizationSharingPanelProps: () => ({}),
      getOrganizationBillingPanelProps: () => ({
        nodeUsage: params.organizationAgentUsage(),
        guestUsage: params.organizationGuestUsage(),
      }),
      getApiAccessPanelProps: () => ({
        currentTokenHint: params.securityStatus()?.apiTokenHint,
        onTokensChanged: () => {
          void params.loadSecurityStatus();
        },
        refreshing: params.securityStatusLoading(),
        canManage: settingsCapabilities()?.apiAccessWrite === true,
      }),
      getSecurityOverviewPanelProps: () => ({
        securityStatus: params.securityStatus,
        securityStatusLoading: params.securityStatusLoading,
      }),
      getSecurityAuthPanelProps: () => ({
        securityStatus: params.securityStatus,
        securityStatusLoading: params.securityStatusLoading,
        versionInfo: params.versionInfo,
        showQuickSecuritySetup: params.showQuickSecuritySetup,
        setShowQuickSecuritySetup: params.setShowQuickSecuritySetup,
        showQuickSecurityWizard: params.showQuickSecurityWizard,
        setShowQuickSecurityWizard: params.setShowQuickSecurityWizard,
        showPasswordModal: params.showPasswordModal,
        setShowPasswordModal: params.setShowPasswordModal,
        hideLocalLogin: params.hideLocalLogin,
        hideLocalLoginLocked: params.hideLocalLoginLocked,
        savingHideLocalLogin: params.savingHideLocalLogin,
        handleHideLocalLoginChange: params.handleHideLocalLoginChange,
        loadSecurityStatus: params.loadSecurityStatus,
        canManage: settingsCapabilities()?.authenticationWrite === true,
      }),
      getRelayPanelProps: () => ({
        canManage: settingsCapabilities()?.relayWrite === true,
      }),
      getAuditWebhookPanelProps: () => ({
        canManage: settingsCapabilities()?.auditWebhooksWrite === true,
      }),
    }),
  );
}
