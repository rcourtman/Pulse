import { Component, createSignal, onMount, Show, Suspense, createMemo } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { useNavigate, useLocation } from '@solidjs/router';
import { useWebSocket } from '@/App';
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
import { useSettingsSystemPanels } from './useSettingsSystemPanels';
import { useSystemSettingsState } from './useSystemSettingsState';
import { useSettingsNavigation } from './useSettingsNavigation';
import { getSettingsLoadingState } from '@/utils/settingsShellPresentation';

import { getLimit, isPro, loadLicenseStatus } from '@/stores/license';
import { pbsInstanceFromResource, pmgInstanceFromResource } from '@/utils/resourceStateAdapters';

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
  const organizationMonitoredSystemUsage = createMemo(
    () => getLimit('max_monitored_systems')?.current ?? 0,
  );
  const organizationGuestUsage = createMemo(() => getLimit('max_guests')?.current ?? 0);
  const discoverySettings = useDiscoverySettingsState();

  // Security
  const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
  const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(false);
  const { securityStatus, securityStatusLoading, flatTabs, filteredTabGroups, loadSecurityStatus } =
    useSettingsAccess({
      activeTab,
      setActiveTab,
      searchQuery,
    });
  const backupTransferFlow = useBackupTransferFlow({ securityStatus });
  const systemSettings = useSystemSettingsState({
    activeTab,
    loadSecurityStatus,
    setDiscoveryEnabled: discoverySettings.setDiscoveryEnabled,
    applySavedDiscoverySubnet: discoverySettings.applySavedDiscoverySubnet,
  });
  const infrastructureSettings = useInfrastructureSettingsState({
    eventBus,
    currentTab,
    discoveryEnabled: discoverySettings.discoveryEnabled,
    setDiscoveryEnabled: discoverySettings.setDiscoveryEnabled,
    discoverySubnet: discoverySettings.discoverySubnet,
    discoveryMode: discoverySettings.discoveryMode,
    setDiscoveryMode: discoverySettings.setDiscoveryMode,
    discoverySubnetDraft: discoverySettings.discoverySubnetDraft,
    setDiscoverySubnetDraft: discoverySettings.setDiscoverySubnetDraft,
    lastCustomSubnet: discoverySettings.lastCustomSubnet,
    setLastCustomSubnet: discoverySettings.setLastCustomSubnet,
    setDiscoverySubnetError: discoverySettings.setDiscoverySubnetError,
    savingDiscoverySettings: discoverySettings.savingDiscoverySettings,
    setSavingDiscoverySettings: discoverySettings.setSavingDiscoverySettings,
    envOverrides: systemSettings.envOverrides,
    normalizeSubnetList: discoverySettings.normalizeSubnetList,
    isValidCIDR: discoverySettings.isValidCIDR,
    applySavedDiscoverySubnet: discoverySettings.applySavedDiscoverySubnet,
    getDiscoverySubnetInputRef: discoverySettings.getDiscoverySubnetInputRef,
    temperatureMonitoringEnabled: systemSettings.temperatureMonitoringEnabled,
    savingTemperatureSetting: systemSettings.savingTemperatureSetting,
    setSavingTemperatureSetting: systemSettings.setSavingTemperatureSetting,
    loadSecurityStatus,
    initializeSystemSettingsState: systemSettings.initializeSystemSettingsState,
  });
  const systemPanels = useSettingsSystemPanels({
    darkMode: props.darkMode,
    themePreference: props.themePreference,
    setThemePreference: props.setThemePreference,
    initialLoadComplete: infrastructureSettings.initialLoadComplete,
    systemSettings,
    discoverySettings: {
      ...discoverySettings,
      handleDiscoveryEnabledChange: infrastructureSettings.handleDiscoveryEnabledChange,
      handleDiscoveryModeChange: infrastructureSettings.handleDiscoveryModeChange,
      commitDiscoverySubnet: infrastructureSettings.commitDiscoverySubnet,
    },
    backupTransferFlow,
    securityStatus,
  });

  const activeTabSaveBehavior = createMemo(() => getSettingsTabSaveBehavior(activeTab()));
  const settingsPanelRegistry = useSettingsPanelRegistry({
    securityStatus,
    securityStatusLoading,
    organizationMonitoredSystemUsage,
    organizationGuestUsage,
    loadSecurityStatus,
    showQuickSecuritySetup,
    setShowQuickSecuritySetup,
    showQuickSecurityWizard,
    setShowQuickSecurityWizard,
    showPasswordModal,
    setShowPasswordModal,
    hideLocalLogin: systemSettings.hideLocalLogin,
    hideLocalLoginLocked: systemSettings.hideLocalLoginLocked,
    savingHideLocalLogin: systemSettings.savingHideLocalLogin,
    handleHideLocalLoginChange: systemSettings.handleHideLocalLoginChange,
    versionInfo: systemSettings.versionInfo,
    systemPanels,
    getInfrastructurePanelProps: () => ({
      selectedAgent,
      onSelectAgent: handleSelectAgent,
      initialLoadComplete: infrastructureSettings.initialLoadComplete,
      discoveryEnabled: discoverySettings.discoveryEnabled,
      discoveryMode: discoverySettings.discoveryMode,
      discoveryScanStatus: infrastructureSettings.discoveryScanStatus,
      discoveredNodes: infrastructureSettings.discoveredNodes,
      savingDiscoverySettings: discoverySettings.savingDiscoverySettings,
      envOverrides: systemSettings.envOverrides,
      agentStateResources: () =>
        (state.resources ?? []).filter((resource) => resource.type === 'agent'),
      pbsInstances: pbsInstancesFromResources,
      pmgInstances: pmgInstancesFromResources,
      pveNodes: infrastructureSettings.pveNodes,
      pbsNodes: infrastructureSettings.pbsNodes,
      pmgNodes: infrastructureSettings.pmgNodes,
      temperatureMonitoringEnabled: systemSettings.temperatureMonitoringEnabled,
      triggerDiscoveryScan: infrastructureSettings.triggerDiscoveryScan,
      loadDiscoveredNodes: infrastructureSettings.loadDiscoveredNodes,
      handleDiscoveryEnabledChange: infrastructureSettings.handleDiscoveryEnabledChange,
      testNodeConnection: infrastructureSettings.testNodeConnection,
      requestDeleteNode: infrastructureSettings.requestDeleteNode,
      refreshClusterNodes: infrastructureSettings.refreshClusterNodes,
      setShowNodeModal: infrastructureSettings.setShowNodeModal,
      editingNode: infrastructureSettings.editingNode,
      setEditingNode: infrastructureSettings.setEditingNode,
      setCurrentNodeType: infrastructureSettings.setCurrentNodeType,
      modalResetKey: infrastructureSettings.modalResetKey,
      setModalResetKey: infrastructureSettings.setModalResetKey,
      isNodeModalVisible: infrastructureSettings.isNodeModalVisible,
      securityStatus,
      resolveTemperatureMonitoringEnabled:
        infrastructureSettings.resolveTemperatureMonitoringEnabled,
      temperatureMonitoringLocked: systemSettings.temperatureMonitoringLocked,
      savingTemperatureSetting: systemSettings.savingTemperatureSetting,
      handleTemperatureMonitoringChange: systemSettings.handleTemperatureMonitoringChange,
      handleNodeTemperatureMonitoringChange:
        infrastructureSettings.handleNodeTemperatureMonitoringChange,
      saveNode: infrastructureSettings.saveNode,
      showDeleteNodeModal: infrastructureSettings.showDeleteNodeModal,
      cancelDeleteNode: infrastructureSettings.cancelDeleteNode,
      deleteNode: infrastructureSettings.deleteNode,
      deleteNodeLoading: infrastructureSettings.deleteNodeLoading,
      nodePendingDeleteLabel: infrastructureSettings.nodePendingDeleteLabel,
      nodePendingDeleteHost: infrastructureSettings.nodePendingDeleteHost,
      nodePendingDeleteType: infrastructureSettings.nodePendingDeleteType,
      nodePendingDeleteTypeLabel: infrastructureSettings.nodePendingDeleteTypeLabel,
      disableDockerUpdateActions: systemSettings.disableDockerUpdateActions,
      disableDockerUpdateActionsLocked: systemSettings.disableDockerUpdateActionsLocked,
      savingDockerUpdateActions: systemSettings.savingDockerUpdateActions,
      handleDisableDockerUpdateActionsChange:
        systemSettings.handleDisableDockerUpdateActionsChange,
    }),
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
        hasUnsavedChanges={systemSettings.hasUnsavedChanges}
        activeTabSaveBehavior={activeTabSaveBehavior}
        saveSettings={systemSettings.saveSettings}
        discardChanges={() => window.location.reload()}
        isMobileMenuOpen={isMobileMenuOpen}
        setIsMobileMenuOpen={setIsMobileMenuOpen}
        sidebarCollapsed={sidebarCollapsed}
        setSidebarCollapsed={setSidebarCollapsed}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
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
                  {getSettingsLoadingState().text}
                </div>
              }
            >
              <Dynamic component={entry().component} {...(entry().getProps?.() ?? {})} />
            </Suspense>
          )}
        </Show>
      </SettingsPageShell>

      <SettingsDialogs
        showUpdateConfirmation={systemSettings.showUpdateConfirmation}
        closeUpdateConfirmation={() => systemSettings.setShowUpdateConfirmation(false)}
        handleConfirmUpdate={systemSettings.handleConfirmUpdate}
        versionInfo={systemSettings.versionInfo}
        updateInfo={systemSettings.updateInfo}
        updatePlan={systemSettings.updatePlan}
        isInstallingUpdate={systemSettings.isInstallingUpdate}
        securityStatus={securityStatus}
        exportPassphrase={backupTransferFlow.exportPassphrase}
        setExportPassphrase={backupTransferFlow.setExportPassphrase}
        useCustomPassphrase={backupTransferFlow.useCustomPassphrase}
        setUseCustomPassphrase={backupTransferFlow.setUseCustomPassphrase}
        importPassphrase={backupTransferFlow.importPassphrase}
        setImportPassphrase={backupTransferFlow.setImportPassphrase}
        importFile={backupTransferFlow.importFile}
        setImportFile={backupTransferFlow.setImportFile}
        showExportDialog={backupTransferFlow.showExportDialog}
        showImportDialog={backupTransferFlow.showImportDialog}
        showApiTokenModal={backupTransferFlow.showApiTokenModal}
        apiTokenInput={backupTransferFlow.apiTokenInput}
        setApiTokenInput={backupTransferFlow.setApiTokenInput}
        handleExport={backupTransferFlow.handleExport}
        handleImport={backupTransferFlow.handleImport}
        closeExportDialog={backupTransferFlow.closeExportDialog}
        closeImportDialog={backupTransferFlow.closeImportDialog}
        closeApiTokenModal={backupTransferFlow.closeApiTokenModal}
        handleApiTokenAuthenticate={backupTransferFlow.handleApiTokenAuthenticate}
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
