import {
  Component,
  createSignal,
  onMount,
  Show,
  Suspense,
  createMemo,
} from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { useNavigate, useLocation } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { ProxmoxSettingsPanel } from './ProxmoxSettingsPanel';
import { SettingsDialogs } from './SettingsDialogs';
import { SettingsPageShell } from './SettingsPageShell';
import { eventBus } from '@/stores/events';
import { getSettingsTabSaveBehavior } from './settingsTabs';
import { useBackupTransferFlow } from './useBackupTransferFlow';
import { useDiscoverySettingsState } from './useDiscoverySettingsState';
import { useInfrastructureSettingsState } from './useInfrastructureSettingsState';
import { useSettingsAccess } from './useSettingsAccess';
import { useSettingsPanelRegistry } from './useSettingsPanelRegistry';
import { useSettingsShellState } from './useSettingsShellState';
import { useSystemSettingsState } from './useSystemSettingsState';
import { useSettingsNavigation } from './useSettingsNavigation';

import { getLimit, isPro, loadLicenseStatus } from '@/stores/license';
import {
  pbsInstanceFromResource,
  pmgInstanceFromResource,
} from '@/utils/resourceStateAdapters';

interface SettingsProps {
  darkMode: () => boolean;
  themePreference: () => 'light' | 'dark' | 'system';
  setThemePreference: (pref: 'light' | 'dark' | 'system') => void;
}

const Settings: Component<SettingsProps> = (props) => {
  const { state, connected: _connected } = useWebSocket();
  const navigate = useNavigate();
  const location = useLocation();
  const { currentTab, activeTab, selectedAgent, setActiveTab, handleSelectAgent } =
    useSettingsNavigation({
    navigate,
    location,
    });
  const {
    headerMeta,
    sidebarCollapsed,
    setSidebarCollapsed,
    isMobileMenuOpen,
    setIsMobileMenuOpen,
    showPasswordModal,
    setShowPasswordModal,
    searchQuery,
    setSearchQuery,
    assignSearchInputRef,
  } = useSettingsShellState({ activeTab });
  const pbsInstancesFromResources = createMemo(() =>
    (state.resources || [])
      .filter((resource) => resource.type === 'pbs')
      .map(pbsInstanceFromResource)
      .filter((instance): instance is NonNullable<typeof instance> => Boolean(instance)),
  );
  const pmgInstancesFromResources = createMemo(() =>
    (state.resources || [])
      .filter((resource) => resource.type === 'pmg')
      .map(pmgInstanceFromResource)
      .filter((instance): instance is NonNullable<typeof instance> => Boolean(instance)),
  );
  const organizationAgentUsage = createMemo(() => getLimit('max_agents')?.current ?? 0);
  const organizationGuestUsage = createMemo(() => getLimit('max_guests')?.current ?? 0);
  const {
    discoveryEnabled,
    setDiscoveryEnabled,
    discoverySubnet,
    discoveryMode,
    setDiscoveryMode,
    discoverySubnetDraft,
    setDiscoverySubnetDraft,
    lastCustomSubnet,
    setLastCustomSubnet,
    discoverySubnetError,
    setDiscoverySubnetError,
    savingDiscoverySettings,
    setSavingDiscoverySettings,
    parseSubnetList,
    normalizeSubnetList,
    currentDraftSubnetValue,
    isValidCIDR,
    applySavedDiscoverySubnet,
    assignDiscoverySubnetInputRef,
    getDiscoverySubnetInputRef,
  } = useDiscoverySettingsState();

  // Security
  const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
  const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(false);
  const {
    securityStatus,
    securityStatusLoading,
    flatTabs,
    filteredTabGroups,
    loadSecurityStatus,
  } = useSettingsAccess({
    activeTab,
    setActiveTab,
    searchQuery,
  });
  const {
    exportPassphrase,
    setExportPassphrase,
    useCustomPassphrase,
    setUseCustomPassphrase,
    importPassphrase,
    setImportPassphrase,
    importFile,
    setImportFile,
    showExportDialog,
    setShowExportDialog,
    showImportDialog,
    setShowImportDialog,
    showApiTokenModal,
    apiTokenInput,
    setApiTokenInput,
    handleExport,
    handleImport,
    closeExportDialog,
    closeImportDialog,
    closeApiTokenModal,
    handleApiTokenAuthenticate,
  } = useBackupTransferFlow({ securityStatus });
  const {
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
  } = useSystemSettingsState({
    activeTab,
    loadSecurityStatus,
    setDiscoveryEnabled,
    applySavedDiscoverySubnet,
  });
  const {
    discoveredNodes,
    setShowNodeModal,
    editingNode,
    setEditingNode,
    setCurrentNodeType,
    modalResetKey,
    setModalResetKey,
    initialLoadComplete,
    discoveryScanStatus,
    showDeleteNodeModal,
    deleteNodeLoading,
    pveNodes,
    pbsNodes,
    pmgNodes,
    isNodeModalVisible,
    resolveTemperatureMonitoringEnabled,
    loadDiscoveredNodes,
    triggerDiscoveryScan,
    handleDiscoveryEnabledChange,
    commitDiscoverySubnet,
    handleDiscoveryModeChange,
    handleNodeTemperatureMonitoringChange,
    requestDeleteNode,
    cancelDeleteNode,
    deleteNode,
    testNodeConnection,
    refreshClusterNodes,
    nodePendingDeleteLabel,
    nodePendingDeleteHost,
    nodePendingDeleteType,
    nodePendingDeleteTypeLabel,
    saveNode,
  } = useInfrastructureSettingsState({
    eventBus,
    currentTab,
    discoveryEnabled,
    setDiscoveryEnabled,
    discoverySubnet,
    discoveryMode,
    setDiscoveryMode,
    discoverySubnetDraft,
    setDiscoverySubnetDraft,
    lastCustomSubnet,
    setLastCustomSubnet,
    setDiscoverySubnetError,
    savingDiscoverySettings,
    setSavingDiscoverySettings,
    envOverrides,
    normalizeSubnetList,
    isValidCIDR,
    applySavedDiscoverySubnet,
    getDiscoverySubnetInputRef,
    temperatureMonitoringEnabled,
    savingTemperatureSetting,
    setSavingTemperatureSetting,
    loadSecurityStatus,
    initializeSystemSettingsState,
  });

  const activeTabSaveBehavior = createMemo(() => getSettingsTabSaveBehavior(activeTab()));
  const settingsPanelRegistry = useSettingsPanelRegistry({
    darkMode: props.darkMode,
    themePreference: props.themePreference,
    setThemePreference: props.setThemePreference,
    initialLoadComplete,
    pvePollingInterval,
    setPVEPollingInterval,
    pvePollingSelection,
    setPVEPollingSelection,
    pvePollingCustomSeconds,
    setPVEPollingCustomSeconds,
    pvePollingEnvLocked,
    setHasUnsavedChanges,
    disableLocalUpgradeMetrics,
    disableLocalUpgradeMetricsLocked,
    savingUpgradeMetrics,
    handleDisableLocalUpgradeMetricsChange,
    telemetryEnabled,
    telemetryEnabledLocked,
    savingTelemetry,
    handleTelemetryEnabledChange,
    disableDockerUpdateActions,
    disableDockerUpdateActionsLocked,
    savingDockerUpdateActions,
    handleDisableDockerUpdateActionsChange,
    discoveryEnabled,
    discoveryMode,
    discoverySubnetDraft,
    discoverySubnetError,
    savingDiscoverySettings,
    envOverrides,
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
    handleDiscoveryEnabledChange,
    handleDiscoveryModeChange,
    setDiscoveryMode,
    setDiscoverySubnetDraft,
    setDiscoverySubnetError,
    setLastCustomSubnet,
    commitDiscoverySubnet,
    parseSubnetList,
    normalizeSubnetList,
    isValidCIDR,
    currentDraftSubnetValue,
    assignDiscoverySubnetInputRef,
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
    checkForUpdates,
    updatePlan,
    handleInstallUpdate,
    isInstallingUpdate,
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
    showExportDialog,
    setShowExportDialog,
    showImportDialog,
    setShowImportDialog,
    setUseCustomPassphrase,
    securityStatus,
    securityStatusLoading,
    organizationAgentUsage,
    organizationGuestUsage,
    loadSecurityStatus,
    showQuickSecuritySetup,
    setShowQuickSecuritySetup,
    showQuickSecurityWizard,
    setShowQuickSecurityWizard,
    showPasswordModal,
    setShowPasswordModal,
    hideLocalLogin,
    hideLocalLoginLocked,
    savingHideLocalLogin,
    handleHideLocalLoginChange,
  });
  const activeSettingsPanelEntry = createMemo(() => {
    const currentTab = activeTab();
    return settingsPanelRegistry()[currentTab];
  });

  onMount(() => {
    loadLicenseStatus();
  });

  return (
    <>
      <SettingsPageShell
        headerMeta={headerMeta}
        hasUnsavedChanges={hasUnsavedChanges}
        activeTabSaveBehavior={activeTabSaveBehavior}
        saveSettings={saveSettings}
        discardChanges={() => window.location.reload()}
        isMobileMenuOpen={isMobileMenuOpen}
        setIsMobileMenuOpen={setIsMobileMenuOpen}
        sidebarCollapsed={sidebarCollapsed}
        setSidebarCollapsed={setSidebarCollapsed}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        assignSearchInputRef={assignSearchInputRef}
        filteredTabGroups={filteredTabGroups}
        flatTabs={flatTabs}
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        isPro={isPro}
      >
        <Show when={activeSettingsPanelEntry()}>
          {(entry) => (
            <Suspense
              fallback={
                <div class="flex items-center justify-center rounded-md border border-dashed border-border bg-surface-alt py-12 text-sm text-muted">
                  Loading settings...
                </div>
              }
            >
              <div class="space-y-8">
                <Dynamic component={entry().component} {...(entry().getProps?.() ?? {})} />
                <Show when={activeTab() === 'agents'}>
                  <ProxmoxSettingsPanel
                    selectedAgent={selectedAgent}
                    onSelectAgent={handleSelectAgent}
                    initialLoadComplete={initialLoadComplete}
                    discoveryEnabled={discoveryEnabled}
                    discoveryMode={discoveryMode}
                    discoveryScanStatus={discoveryScanStatus}
                    discoveredNodes={discoveredNodes}
                    savingDiscoverySettings={savingDiscoverySettings}
                    envOverrides={envOverrides}
                    agentStateResources={() =>
                      (state.resources ?? []).filter((resource) => resource.type === 'agent')
                    }
                    pbsInstances={pbsInstancesFromResources}
                    pmgInstances={pmgInstancesFromResources}
                    pveNodes={pveNodes}
                    pbsNodes={pbsNodes}
                    pmgNodes={pmgNodes}
                    temperatureMonitoringEnabled={temperatureMonitoringEnabled}
                    triggerDiscoveryScan={triggerDiscoveryScan}
                    loadDiscoveredNodes={loadDiscoveredNodes}
                    handleDiscoveryEnabledChange={handleDiscoveryEnabledChange}
                    testNodeConnection={testNodeConnection}
                    requestDeleteNode={requestDeleteNode}
                    refreshClusterNodes={refreshClusterNodes}
                    setShowNodeModal={setShowNodeModal}
                    editingNode={editingNode}
                    setEditingNode={setEditingNode}
                    setCurrentNodeType={setCurrentNodeType}
                    modalResetKey={modalResetKey}
                    setModalResetKey={setModalResetKey}
                    isNodeModalVisible={isNodeModalVisible}
                    securityStatus={securityStatus}
                    resolveTemperatureMonitoringEnabled={resolveTemperatureMonitoringEnabled}
                    temperatureMonitoringLocked={temperatureMonitoringLocked}
                    savingTemperatureSetting={savingTemperatureSetting}
                    handleTemperatureMonitoringChange={handleTemperatureMonitoringChange}
                    handleNodeTemperatureMonitoringChange={handleNodeTemperatureMonitoringChange}
                    saveNode={saveNode}
                    showDeleteNodeModal={showDeleteNodeModal}
                    cancelDeleteNode={cancelDeleteNode}
                    deleteNode={deleteNode}
                    deleteNodeLoading={deleteNodeLoading}
                    nodePendingDeleteLabel={nodePendingDeleteLabel}
                    nodePendingDeleteHost={nodePendingDeleteHost}
                    nodePendingDeleteType={nodePendingDeleteType}
                    nodePendingDeleteTypeLabel={nodePendingDeleteTypeLabel}
                  />
                </Show>
              </div>
            </Suspense>
          )}
        </Show>
      </SettingsPageShell>

      <SettingsDialogs
        showUpdateConfirmation={showUpdateConfirmation}
        closeUpdateConfirmation={() => setShowUpdateConfirmation(false)}
        handleConfirmUpdate={handleConfirmUpdate}
        versionInfo={versionInfo}
        updateInfo={updateInfo}
        updatePlan={updatePlan}
        isInstallingUpdate={isInstallingUpdate}
        securityStatus={securityStatus}
        exportPassphrase={exportPassphrase}
        setExportPassphrase={setExportPassphrase}
        useCustomPassphrase={useCustomPassphrase}
        setUseCustomPassphrase={setUseCustomPassphrase}
        importPassphrase={importPassphrase}
        setImportPassphrase={setImportPassphrase}
        importFile={importFile}
        setImportFile={setImportFile}
        showExportDialog={showExportDialog}
        showImportDialog={showImportDialog}
        showApiTokenModal={showApiTokenModal}
        apiTokenInput={apiTokenInput}
        setApiTokenInput={setApiTokenInput}
        handleExport={handleExport}
        handleImport={handleImport}
        closeExportDialog={closeExportDialog}
        closeImportDialog={closeImportDialog}
        closeApiTokenModal={closeApiTokenModal}
        handleApiTokenAuthenticate={handleApiTokenAuthenticate}
        showPasswordModal={showPasswordModal}
        closePasswordModal={() => {
          setShowPasswordModal(false);
          void loadSecurityStatus();
        }}
      />
    </>
  );
};

export default Settings;
