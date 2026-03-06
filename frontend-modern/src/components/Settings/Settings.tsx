import {
  Component,
  createSignal,
  onMount,
  Show,
  Suspense,
  createEffect,
  createMemo,
  onCleanup,
} from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { useNavigate, useLocation } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { logger } from '@/utils/logger';
import { ChangePasswordModal } from './ChangePasswordModal';
import { UnifiedAgents } from './UnifiedAgents';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { SSOProvidersPanel } from './SSOProvidersPanel';
import { AISettings } from './AISettings';
import { AICostDashboard } from '@/components/AI/AICostDashboard';
import { GeneralSettingsPanel } from './GeneralSettingsPanel';
import { UpdateConfirmationModal } from '@/components/UpdateConfirmationModal';
import { BackupTransferDialogs } from './BackupTransferDialogs';
import { ProLicensePanel } from './ProLicensePanel';
import { AgentLedgerPanel } from './AgentLedgerPanel';
import { ProxmoxSettingsPanel } from './ProxmoxSettingsPanel';
import { SettingsPageShell } from './SettingsPageShell';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { eventBus } from '@/stores/events';
import { SETTINGS_HEADER_META } from './settingsHeaderMeta';
import { createSettingsPanelRegistry } from './settingsPanelRegistry';
import {
  baseTabGroups,
  getSettingsNavItem,
  getSettingsTabSaveBehavior,
  shouldHideSettingsNavItem,
} from './settingsTabs';
import { useBackupTransferFlow } from './useBackupTransferFlow';
import { useInfrastructureSettingsState } from './useInfrastructureSettingsState';
import { useSystemSettingsState } from './useSystemSettingsState';
import { useSettingsNavigation } from './useSettingsNavigation';
import { DEFAULT_SETTINGS_TAB } from './settingsRouting';
import { tabFeatureRequirements } from './settingsFeatureGates';

import {
  getLimit,
  hasFeature,
  isHostedModeEnabled,
  isPro,
  licenseLoaded,
  loadLicenseStatus,
} from '@/stores/license';
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

  const headerMeta = () =>
    SETTINGS_HEADER_META[activeTab()] ?? {
      title: 'Settings',
      description: 'Manage Pulse configuration.',
    };

  // Sidebar always starts expanded for discoverability (issue #764)
  // Users can collapse during session but it resets on page reload
  const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = createSignal(
    typeof window !== 'undefined' ? window.innerWidth < 1024 : false,
  );
  const [showPasswordModal, setShowPasswordModal] = createSignal(false);
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

  const [discoveryEnabled, setDiscoveryEnabled] = createSignal(false);
  const [discoverySubnet, setDiscoverySubnet] = createSignal('auto');
  const [discoveryMode, setDiscoveryMode] = createSignal<'auto' | 'custom'>('auto');
  const [discoverySubnetDraft, setDiscoverySubnetDraft] = createSignal('');
  const [lastCustomSubnet, setLastCustomSubnet] = createSignal('');
  const [discoverySubnetError, setDiscoverySubnetError] = createSignal<string | undefined>();
  const [savingDiscoverySettings, setSavingDiscoverySettings] = createSignal(false);
  let discoverySubnetInputRef: HTMLInputElement | undefined;

  const parseSubnetList = (value: string) => {
    const seen = new Set<string>();
    return value
      .split(',')
      .map((token) => token.trim())
      .filter((token) => {
        if (!token || token.toLowerCase() === 'auto' || seen.has(token)) {
          return false;
        }
        seen.add(token);
        return true;
      });
  };

  const normalizeSubnetList = (value: string) => parseSubnetList(value).join(', ');

  const currentDraftSubnetValue = () => {
    if (discoveryMode() === 'custom') {
      return discoverySubnetDraft();
    }
    const draft = discoverySubnetDraft();
    if (draft.trim() !== '') {
      return draft;
    }
    const saved = discoverySubnet();
    return saved.toLowerCase() === 'auto' ? '' : saved;
  };

  const isValidCIDR = (value: string) => {
    const subnets = parseSubnetList(value);
    if (subnets.length === 0) {
      return false;
    }

    return subnets.every((token) => {
      const [network, prefix] = token.split('/');
      if (!network || typeof prefix === 'undefined') {
        return false;
      }

      const prefixNumber = Number(prefix);
      if (!Number.isInteger(prefixNumber) || prefixNumber < 0 || prefixNumber > 32) {
        return false;
      }

      const octets = network.split('.');
      if (octets.length !== 4) {
        return false;
      }

      return octets.every((octet) => {
        if (octet === '') return false;
        if (!/^\d+$/.test(octet)) return false;
        const valueNumber = Number(octet);
        return valueNumber >= 0 && valueNumber <= 255;
      });
    });
  };

  const applySavedDiscoverySubnet = (subnet?: string | null) => {
    const raw = typeof subnet === 'string' ? subnet.trim() : '';
    if (raw === '' || raw.toLowerCase() === 'auto') {
      setDiscoverySubnet('auto');
      setDiscoveryMode('auto');
      setDiscoverySubnetDraft('');
    } else {
      setDiscoveryMode('custom');
      const normalizedValue = normalizeSubnetList(raw);
      setDiscoverySubnet(normalizedValue);
      setDiscoverySubnetDraft(normalizedValue);
      setLastCustomSubnet(normalizedValue);
      setDiscoverySubnetError(undefined);
      return;
    }
    setDiscoverySubnetError(undefined);
  };
  // Connection timeout removed - backend-only setting

  // Security
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatusInfo | null>(null);
  const [securityStatusLoading, setSecurityStatusLoading] = createSignal(true);
  const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
  const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(false);
  const [searchQuery, setSearchQuery] = createSignal('');
  let searchInputRef: HTMLInputElement | undefined;
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
    getDiscoverySubnetInputRef: () => discoverySubnetInputRef,
    temperatureMonitoringEnabled,
    savingTemperatureSetting,
    setSavingTemperatureSetting,
    loadSecurityStatus,
    initializeSystemSettingsState,
  });

  const visibleTabGroups = createMemo(() => {
    const hostedModeEnabled = isHostedModeEnabled();
    const settingsCapabilities = securityStatus()?.settingsCapabilities ?? null;

    return baseTabGroups
      .map((group) => ({
        ...group,
        items: group.items.filter(
          (item) =>
            !shouldHideSettingsNavItem(item.id, {
              hasFeature,
              licenseLoaded,
              hostedModeEnabled,
              settingsCapabilities,
            }),
        ),
      }))
      .filter((group) => group.items.length > 0);
  });

  const settingsCapabilities = createMemo(() => securityStatus()?.settingsCapabilities ?? null);
  const activeTabSaveBehavior = createMemo(() => getSettingsTabSaveBehavior(activeTab()));
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
              <Show when={disableDockerUpdateActionsLocked()}>
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
              onClick={() => handleDisableDockerUpdateActionsChange(!disableDockerUpdateActions())}
              disabled={disableDockerUpdateActionsLocked() || savingDockerUpdateActions()}
              class={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${
                disableDockerUpdateActions() ? 'bg-blue-600' : 'bg-surface-alt'
              } ${disableDockerUpdateActionsLocked() ? 'opacity-50 cursor-not-allowed' : ''}`}
              role="switch"
              aria-checked={disableDockerUpdateActions()}
              title={
                disableDockerUpdateActionsLocked() ? 'Locked by environment variable' : undefined
              }
            >
              <span
                class={`inline-block h-4 w-4 transform rounded-full bg-white shadow-sm transition-transform ${
                  disableDockerUpdateActions() ? 'translate-x-6' : 'translate-x-1'
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
      <Show when={!initialLoadComplete()}>
        <div class="flex items-center justify-center rounded-md border border-dashed border-border bg-surface-alt py-12 text-sm text-muted">
          Loading configuration...
        </div>
      </Show>
      <Show when={initialLoadComplete()}>
        <GeneralSettingsPanel
          darkMode={props.darkMode}
          themePreference={props.themePreference}
          setThemePreference={props.setThemePreference}
          pvePollingInterval={pvePollingInterval}
          setPVEPollingInterval={setPVEPollingInterval}
          pvePollingSelection={pvePollingSelection}
          setPVEPollingSelection={setPVEPollingSelection}
          pvePollingCustomSeconds={pvePollingCustomSeconds}
          setPVEPollingCustomSeconds={setPVEPollingCustomSeconds}
          pvePollingEnvLocked={pvePollingEnvLocked}
          setHasUnsavedChanges={setHasUnsavedChanges}
          disableLocalUpgradeMetrics={disableLocalUpgradeMetrics}
          disableLocalUpgradeMetricsLocked={disableLocalUpgradeMetricsLocked}
          savingUpgradeMetrics={savingUpgradeMetrics}
          handleDisableLocalUpgradeMetricsChange={handleDisableLocalUpgradeMetricsChange}
          telemetryEnabled={telemetryEnabled}
          telemetryEnabledLocked={telemetryEnabledLocked}
          savingTelemetry={savingTelemetry}
          handleTelemetryEnabledChange={handleTelemetryEnabledChange}
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
        onConfigUpdated={loadSecurityStatus}
        canManage={settingsCapabilities()?.singleSignOnWrite === true}
      />
    </div>
  );
  const settingsPanelRegistry = createMemo(() =>
    createSettingsPanelRegistry({
      agentsPanel,
      dockerPanel,
      systemGeneralPanel,
      systemAiPanel,
      systemProPanel,
      securitySsoPanel,
      getNetworkPanelProps: () => ({
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
        setHasUnsavedChanges,
        parseSubnetList,
        normalizeSubnetList,
        isValidCIDR,
        currentDraftSubnetValue,
        discoverySubnetInputRef: (el: HTMLInputElement) => {
          discoverySubnetInputRef = el;
        },
      }),
      getUpdatesPanelProps: () => ({
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
        setHasUnsavedChanges,
        updatePlan,
        onInstallUpdate: handleInstallUpdate,
        isInstalling: isInstallingUpdate,
      }),
      getRecoveryPanelProps: () => ({
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
        setHasUnsavedChanges,
        showExportDialog,
        setShowExportDialog,
        showImportDialog,
        setShowImportDialog,
        setUseCustomPassphrase,
        securityStatus,
      }),
      getOrganizationOverviewPanelProps: () => ({}),
      getOrganizationAccessPanelProps: () => ({}),
      getOrganizationSharingPanelProps: () => ({}),
      getOrganizationBillingPanelProps: () => ({
        nodeUsage: organizationAgentUsage(),
        guestUsage: organizationGuestUsage(),
      }),
      getApiAccessPanelProps: () => ({
        currentTokenHint: securityStatus()?.apiTokenHint,
        onTokensChanged: () => {
          void loadSecurityStatus();
        },
        refreshing: securityStatusLoading(),
        canManage: settingsCapabilities()?.apiAccessWrite === true,
      }),
      getSecurityOverviewPanelProps: () => ({
        securityStatus,
        securityStatusLoading,
      }),
      getSecurityAuthPanelProps: () => ({
        securityStatus,
        securityStatusLoading,
        versionInfo,
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
        loadSecurityStatus,
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
  const activeSettingsPanelEntry = createMemo(() => {
    const currentTab = activeTab();
    if (currentTab === 'proxmox') {
      return null;
    }

    return settingsPanelRegistry()[currentTab];
  });

  const flatTabs = createMemo(() => visibleTabGroups().flatMap((group) => group.items));

  const filteredTabGroups = createMemo(() => {
    const q = searchQuery().trim().toLowerCase();
    const groups = visibleTabGroups();
    if (!q) return groups;

    return groups
      .map((group) => {
        const filteredItems = group.items.filter((item) => {
          const matchLabel = item.label.toLowerCase().includes(q);
          const description = SETTINGS_HEADER_META[item.id]?.description?.toLowerCase() || '';
          const matchDesc = description.includes(q);
          return matchLabel || matchDesc;
        });
        return { ...group, items: filteredItems };
      })
      .filter((group) => group.items.length > 0);
  });

  createEffect(() => {
    const currentTab = activeTab();
    const requiresFeatureResolution = Boolean(tabFeatureRequirements[currentTab]?.length);
    const requiresCapabilityResolution = Boolean(
      getSettingsNavItem(currentTab)?.requiredCapability,
    );
    if (
      (requiresFeatureResolution && !licenseLoaded()) ||
      (requiresCapabilityResolution && securityStatusLoading())
    ) {
      return;
    }

    if (!flatTabs().some((tab) => tab.id === currentTab)) {
      setActiveTab(DEFAULT_SETTINGS_TAB);
    }
  });

  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setSearchQuery('');
        searchInputRef?.blur();
        return;
      }

      if (
        document.activeElement?.tagName === 'INPUT' ||
        document.activeElement?.tagName === 'TEXTAREA'
      ) {
        return;
      }
      if (e.metaKey || e.ctrlKey || e.altKey || e.key.length > 1) {
        if (e.key !== 'Backspace') return;
      }

      if (searchInputRef) {
        e.preventDefault();
        searchInputRef.focus();
        if (e.key === 'Backspace') {
          setSearchQuery((prev) => prev.slice(0, -1));
        } else {
          setSearchQuery((prev) => prev + e.key);
        }
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    onCleanup(() => window.removeEventListener('keydown', handleKeyDown));
  });

  onMount(() => {
    loadLicenseStatus();
  });

  async function loadSecurityStatus() {
    setSecurityStatusLoading(true);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/status');
      if (response.ok) {
        const status = await response.json();
        logger.debug('Security status loaded', status);
        setSecurityStatus(status);
      } else {
        logger.error('Failed to fetch security status', { status: response.status });
      }
    } catch (err) {
      logger.error('Failed to fetch security status', err);
    } finally {
      setSecurityStatusLoading(false);
    }
  }

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
        assignSearchInputRef={(el) => {
          searchInputRef = el;
        }}
        filteredTabGroups={filteredTabGroups}
        flatTabs={flatTabs}
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        isPro={isPro}
      >
        <Show when={activeTab() === 'proxmox'}>
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
        <Show when={activeTab() !== 'proxmox' && activeSettingsPanelEntry()}>
          {(entry) => (
            <Suspense
              fallback={
                <div class="flex items-center justify-center rounded-md border border-dashed border-border bg-surface-alt py-12 text-sm text-muted">
                  Loading settings...
                </div>
              }
            >
              <Dynamic component={entry().component} {...(entry().getProps?.() ?? {})} />
            </Suspense>
          )}
        </Show>
      </SettingsPageShell>

      {/* Update Confirmation Modal */}
      <UpdateConfirmationModal
        isOpen={showUpdateConfirmation()}
        onClose={() => setShowUpdateConfirmation(false)}
        onConfirm={handleConfirmUpdate}
        currentVersion={versionInfo()?.version || 'Unknown'}
        latestVersion={updateInfo()?.latestVersion || ''}
        plan={
          updatePlan() || {
            canAutoUpdate: false,
            requiresRoot: false,
            rollbackSupport: false,
          }
        }
        isApplying={isInstallingUpdate()}
        isPrerelease={updateInfo()?.isPrerelease}
        isMajorUpgrade={updateInfo()?.isMajorUpgrade}
        warning={updateInfo()?.warning}
      />

      <BackupTransferDialogs
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
      />

      <ChangePasswordModal
        isOpen={showPasswordModal()}
        onClose={() => {
          setShowPasswordModal(false);
          // Refresh security status after password change
          loadSecurityStatus();
        }}
      />
    </>
  );
};

export default Settings;
