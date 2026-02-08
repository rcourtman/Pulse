import {
  Component,
  createSignal,
  onMount,
  For,
  Show,
  createEffect,
  createMemo,
} from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { useNavigate, useLocation } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { formatRelativeTime } from '@/utils/format';
import { NodeModal } from './NodeModal';
import { ChangePasswordModal } from './ChangePasswordModal';
import { UnifiedAgents } from './UnifiedAgents';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { OIDCPanel } from './OIDCPanel';
import { SSOProvidersPanel } from './SSOProvidersPanel';
import { AISettings } from './AISettings';
import { AICostDashboard } from '@/components/AI/AICostDashboard';
import { UpdateConfirmationModal } from '@/components/UpdateConfirmationModal';
import SettingsPanel from '@/components/shared/SettingsPanel';
import {
  PveNodesTable,
  PbsNodesTable,
  PmgNodesTable
} from './ConfiguredNodeTables';
import { SettingsSectionNav } from './SettingsSectionNav';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';
import Server from 'lucide-solid/icons/server';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Mail from 'lucide-solid/icons/mail';
import Loader from 'lucide-solid/icons/loader';
import Container from 'lucide-solid/icons/container';
import type { NodeConfigWithStatus } from '@/types/nodes';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { eventBus } from '@/stores/events';
import { hasFeature, isMultiTenantEnabled, licenseLoaded, loadLicenseStatus } from '@/stores/license';
import { SETTINGS_HEADER_META } from './settingsHeaderMeta';
import { baseTabGroups } from './settingsTabs';
import { isFeatureLocked, isTabLocked } from './settingsFeatureGates';
import type { SettingsNavGroup, SettingsTab } from './settingsTypes';
import { useSettingsNavigation } from './useSettingsNavigation';
import { useSystemSettingsState } from './useSystemSettingsState';
import { useInfrastructureSettingsState } from './useInfrastructureSettingsState';
import { useBackupTransferFlow } from './useBackupTransferFlow';
import {
  createSettingsPanelRegistry,
  type SettingsDispatchableTab,
} from './settingsPanelRegistry';

interface SettingsProps {
  darkMode: () => boolean;
  toggleDarkMode: () => void;
}

const Settings: Component<SettingsProps> = (props) => {
  const { state, connected: _connected } = useWebSocket();
  const navigate = useNavigate();
  const location = useLocation();

  const { currentTab, activeTab, selectedAgent, setActiveTab, handleSelectAgent } =
    useSettingsNavigation({ navigate, location });

  const headerMeta = () =>
    SETTINGS_HEADER_META[activeTab()] ?? {
      title: 'Settings',
      description: 'Manage Pulse configuration.',
    };

  // Sidebar always starts expanded for discoverability (issue #764)
  // Users can collapse during session but it resets on page reload
  const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);
  const [showPasswordModal, setShowPasswordModal] = createSignal(false);

  // Discovery is infrastructure-coupled and remains in Settings.tsx for Packet 05.
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
    currentTab,
    loadSecurityStatus: () => loadSecurityStatus(),
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
    orgNodeUsage,
    orgGuestUsage,
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
    state,
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
    loadSecurityStatus: () => loadSecurityStatus(),
    initializeSystemSettingsState,
  });

  // Security
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatusInfo | null>(null);
  const [securityStatusLoading, setSecurityStatusLoading] = createSignal(true);
  const currentSettingsUser = createMemo(
    () => securityStatus()?.proxyAuthUsername || securityStatus()?.authUsername || undefined,
  );
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
  const [showQuickSecuritySetup, setShowQuickSecuritySetup] = createSignal(false);
  const authDisabledByEnv = createMemo(() => Boolean(securityStatus()?.deprecatedDisableAuth));
  const [showQuickSecurityWizard, setShowQuickSecurityWizard] = createSignal(false);
  const isFeatureLockedForLicense = (features?: string[]): boolean =>
    isFeatureLocked(features, hasFeature, licenseLoaded);

  const isTabLockedForLicense = (tab: SettingsTab): boolean =>
    isTabLocked(tab, hasFeature, licenseLoaded);

  const tabGroups = createMemo<SettingsNavGroup[]>(() =>
    baseTabGroups
      .map((group) => {
        const items = group.items
          .filter((item) => !(item.features?.includes('multi_tenant') && !isMultiTenantEnabled()))
          .map((item) => {
            const lockedByFeature = isFeatureLockedForLicense(item.features);
            return {
              ...item,
              disabled: item.disabled || lockedByFeature,
              badge: lockedByFeature ? 'Pro' : item.badge,
            };
          });

        return {
          ...group,
          items,
        };
      })
      .filter((group) => group.items.length > 0),
  );

  const flatTabs = createMemo(() => tabGroups().flatMap((group) => group.items));

  createEffect(() => {
    if (!licenseLoaded()) {
      return;
    }
    const tab = activeTab();
    if (tab.startsWith('organization-') && !isMultiTenantEnabled()) {
      setActiveTab('proxmox');
      return;
    }
    if (!isTabLockedForLicense(tab)) {
      return;
    }
    if (tab !== 'system-pro') {
      notificationStore.info('This settings section requires Pulse Pro.');
      setActiveTab('system-pro');
    }
  });

  onMount(() => {
    loadLicenseStatus();
  });

  const loadSecurityStatus = async () => {
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
  };

  createEffect(() => {
    if (authDisabledByEnv() && showQuickSecuritySetup()) {
      setShowQuickSecuritySetup(false);
    }
  });

  const AgentsPanel: Component = () => (
    <>
      <UnifiedAgents />
      <AgentProfilesPanel />
    </>
  );

  const DockerPanel: Component = () => (
    <SettingsPanel
      title="Docker Workload Controls"
      description="Configure server-wide Docker workload behavior."
      icon={<Container class="w-5 h-5" strokeWidth={2} />}
    >
      <div class="flex items-start justify-between gap-4 p-4 rounded-lg border border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-gray-800/50">
        <div class="flex-1 space-y-1">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-900 dark:text-gray-100">
              Hide Docker Update Buttons
            </span>
            <Show when={disableDockerUpdateActionsLocked()}>
              <span
                class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300"
                title="Locked by environment variable PULSE_DISABLE_DOCKER_UPDATE_ACTIONS"
              >
                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
                ENV
              </span>
            </Show>
          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            When enabled, the "Update" button on Docker containers is hidden across all views.
            Update detection still runs, so available updates remain visible.
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
            Environment variable override:{' '}
            <code class="px-1 py-0.5 rounded bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
              PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=true
            </code>
          </p>
        </div>
        <div class="flex-shrink-0">
          <button
            type="button"
            onClick={() => handleDisableDockerUpdateActionsChange(!disableDockerUpdateActions())}
            disabled={disableDockerUpdateActionsLocked() || savingDockerUpdateActions()}
            class={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900 ${disableDockerUpdateActions()
              ? 'bg-blue-600'
              : 'bg-gray-300 dark:bg-gray-600'
              } ${disableDockerUpdateActionsLocked() ? 'opacity-50 cursor-not-allowed' : ''}`}
            role="switch"
            aria-checked={disableDockerUpdateActions()}
            title={disableDockerUpdateActionsLocked() ? 'Locked by environment variable' : undefined}
          >
            <span
              class={`inline-block h-4 w-4 transform rounded-full bg-white shadow-sm transition-transform ${disableDockerUpdateActions() ? 'translate-x-6' : 'translate-x-1'
                }`}
            />
          </button>
        </div>
      </div>
    </SettingsPanel>
  );

  const SystemAiPanel: Component = () => (
    <div class="space-y-6">
      <AISettings />
      <AICostDashboard />
    </div>
  );

  const SecuritySsoPanel: Component = () => (
    <div class="space-y-6">
      <SSOProvidersPanel onConfigUpdated={loadSecurityStatus} />
      {/* Legacy OIDC panel for backward compatibility */}
      <OIDCPanel onConfigUpdated={loadSecurityStatus} />
    </div>
  );

  const panelRegistry = createSettingsPanelRegistry({
    agentsPanel: AgentsPanel,
    dockerPanel: DockerPanel,
    systemAiPanel: SystemAiPanel,
    securitySsoPanel: SecuritySsoPanel,
    getGeneralPanelProps: () => ({
      darkMode: props.darkMode,
      toggleDarkMode: props.toggleDarkMode,
      pvePollingInterval,
      setPVEPollingInterval,
      pvePollingSelection,
      setPVEPollingSelection,
      pvePollingCustomSeconds,
      setPVEPollingCustomSeconds,
      pvePollingEnvLocked,
      setHasUnsavedChanges,
    }),
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
    getBackupsPanelProps: () => ({
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
    getOrganizationOverviewPanelProps: () => ({
      currentUser: currentSettingsUser(),
    }),
    getOrganizationAccessPanelProps: () => ({
      currentUser: currentSettingsUser(),
    }),
    getOrganizationSharingPanelProps: () => ({
      currentUser: currentSettingsUser(),
    }),
    getOrganizationBillingPanelProps: () => ({
      nodeUsage: orgNodeUsage(),
      guestUsage: orgGuestUsage(),
    }),
    getApiAccessPanelProps: () => ({
      currentTokenHint: securityStatus()?.apiTokenHint,
      onTokensChanged: () => {
        void loadSecurityStatus();
      },
      refreshing: securityStatusLoading(),
    }),
    getSecurityOverviewPanelProps: () => ({
      securityStatus,
      securityStatusLoading,
    }),
    getSecurityAuthPanelProps: () => ({
      securityStatus,
      securityStatusLoading,
      versionInfo,
      authDisabledByEnv,
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
    }),
  });

  const isDispatchableTab = (tab: SettingsTab): tab is SettingsDispatchableTab => tab !== 'proxmox';

  const activePanelRender = createMemo<{
    component: Component<any>;
    props: object;
  } | null>(() => {
    const tab = activeTab();
    if (!isDispatchableTab(tab)) {
      return null;
    }

    const entry = panelRegistry[tab];
    return {
      component: entry.component,
      props: entry.getProps?.() ?? {},
    };
  });

  return (
    <>
      <div class="space-y-6">
        {/* Page header - no card wrapper for cleaner hierarchy */}
        <div class="px-1">
          <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-2">
            {headerMeta().title}
          </h1>
          <p class="text-base text-gray-600 dark:text-gray-400">{headerMeta().description}</p>
        </div>

        {/* Save notification bar - only show when there are unsaved changes */}
        <Show
          when={
            hasUnsavedChanges() &&
            (activeTab() === 'proxmox' ||
              activeTab() === 'system-general' ||
              activeTab() === 'system-network' ||
              activeTab() === 'system-updates' ||
              activeTab() === 'system-backups')
          }
        >
          <div class="bg-amber-50/70 dark:bg-amber-950/20 border border-amber-200 dark:border-amber-800/60 rounded-lg shadow-sm p-4">
            <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
              <div class="flex items-start gap-3">
                <svg
                  class="w-5 h-5 text-amber-500 dark:text-amber-300 flex-shrink-0 mt-0.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <div>
                  <p class="font-semibold text-gray-900 dark:text-gray-100">Unsaved changes</p>
                  <p class="text-sm text-gray-600 dark:text-gray-300 mt-0.5">
                    Your changes will be lost if you navigate away
                  </p>
                </div>
              </div>
              <div class="flex w-full sm:w-auto gap-3">
                <button
                  type="button"
                  class="flex-1 sm:flex-initial px-5 py-2.5 text-sm font-medium border border-amber-300 dark:border-amber-700 bg-amber-100 dark:bg-amber-900/40 text-amber-800 dark:text-amber-100 rounded-lg hover:bg-amber-200 dark:hover:bg-amber-900/60 shadow-sm transition-colors"
                  onClick={saveSettings}
                >
                  Save Changes
                </button>
                <button
                  type="button"
                  class="px-4 py-2.5 text-sm font-medium text-gray-600 dark:text-gray-300 hover:underline transition-colors"
                  onClick={() => {
                    window.location.reload();
                  }}
                >
                  Discard
                </button>
              </div>
            </div>
          </div>
        </Show>

        <Card padding="none" class="relative lg:flex overflow-hidden">
          <div
            class={`hidden lg:flex lg:flex-col ${sidebarCollapsed() ? 'w-16' : 'w-72'} ${sidebarCollapsed() ? 'lg:min-w-[4rem] lg:max-w-[4rem] lg:basis-[4rem]' : 'lg:min-w-[18rem] lg:max-w-[18rem] lg:basis-[18rem]'} relative border-b border-gray-200 dark:border-gray-700 lg:border-b-0 lg:border-r lg:border-gray-200 dark:lg:border-gray-700 lg:align-top flex-shrink-0 transition-all duration-200`}
            aria-label="Settings navigation"
            aria-expanded={!sidebarCollapsed()}
          >
            <div
              class={`sticky top-0 ${sidebarCollapsed() ? 'px-2' : 'px-4'} py-5 space-y-5 transition-all duration-200`}
            >
              <Show when={!sidebarCollapsed()}>
                <div class="flex items-center justify-between pb-2 border-b border-gray-200 dark:border-gray-700">
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Settings</h2>
                  <button
                    type="button"
                    onClick={() => setSidebarCollapsed(true)}
                    class="p-1 rounded-md text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                    aria-label="Collapse sidebar"
                  >
                    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M11 19l-7-7 7-7m8 14l-7-7 7-7"
                      />
                    </svg>
                  </button>
                </div>
              </Show>
              <Show when={sidebarCollapsed()}>
                <button
                  type="button"
                  onClick={() => setSidebarCollapsed(false)}
                  class="w-full p-2 rounded-md text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                  aria-label="Expand sidebar"
                >
                  <svg
                    class="w-5 h-5 mx-auto"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M13 5l7 7-7 7M5 5l7 7-7 7"
                    />
                  </svg>
                </button>
              </Show>
              <div id="settings-sidebar-menu" class="space-y-5">
                <For each={tabGroups()}>
                  {(group) => (
                    <div class="space-y-2">
                      <Show when={!sidebarCollapsed()}>
                        <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {group.label}
                        </p>
                      </Show>
                      <div class="space-y-1.5">
                        <For each={group.items}>
                          {(item) => {
                            const isActive = () => activeTab() === item.id;
                            return (
                              <button
                                type="button"
                                aria-current={isActive() ? 'page' : undefined}
                                disabled={item.disabled}
                                class={`flex w-full items-center ${sidebarCollapsed() ? 'justify-center' : 'gap-2.5'} rounded-md ${sidebarCollapsed() ? 'px-2 py-2.5' : 'px-3 py-2'
                                  } text-sm font-medium transition-colors ${item.disabled
                                    ? 'opacity-60 cursor-not-allowed text-gray-400 dark:text-gray-600'
                                    : isActive()
                                      ? 'bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-200'
                                      : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700/60 dark:hover:text-gray-100'
                                  }`}
                                onClick={() => {
                                  if (item.disabled) return;
                                  setActiveTab(item.id);
                                }}
                                title={
                                  sidebarCollapsed()
                                    ? `${item.label}${item.badge ? ` (${item.badge})` : ''}`
                                    : undefined
                                }
                              >
                                <item.icon class="w-4 h-4" {...(item.iconProps || {})} />
                                <Show when={!sidebarCollapsed()}>
                                  <span class="truncate">{item.label}</span>
                                  <Show when={item.badge}>
                                    <span class="ml-auto px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider bg-gray-500 text-white rounded-md shadow-sm">
                                      {item.badge}
                                    </span>
                                  </Show>
                                </Show>
                              </button>
                            );
                          }}
                        </For>
                      </div>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </div>

          <div class="flex-1 overflow-hidden">
            <Show when={flatTabs().length > 0}>
              <div class="lg:hidden border-b border-gray-200 dark:border-gray-700">
                <div
                  class="flex gap-1 px-2 py-1 w-full overflow-x-auto"
                  style="-webkit-overflow-scrolling: touch;"
                >
                  <For each={flatTabs()}>
                    {(tab) => {
                      const isActive = () => activeTab() === tab.id;
                      const disabled = tab.disabled;
                      return (
                        <button
                          type="button"
                          disabled={disabled}
                          class={`px-3 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${disabled
                            ? 'opacity-60 cursor-not-allowed text-gray-400 dark:text-gray-600 border-transparent'
                            : isActive()
                              ? 'text-blue-600 dark:text-blue-300 border-blue-500 dark:border-blue-400'
                              : 'text-gray-600 dark:text-gray-400 border-transparent hover:text-blue-500 dark:hover:text-blue-300 hover:border-blue-300/70 dark:hover:border-blue-500/50'
                            }`}
                          onClick={() => {
                            if (disabled) return;
                            setActiveTab(tab.id);
                          }}
                        >
                          {tab.label}
                        </button>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>

            <div class="p-6 lg:p-8">
              <Show when={activeTab() === 'proxmox'}>
                <SettingsSectionNav
                  current={selectedAgent()}
                  onSelect={handleSelectAgent}
                  class="mb-6"
                />
              </Show>

              {/* Recommendation banner for Proxmox tab */}
              <Show when={activeTab() === 'proxmox'}>
                <div class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 mb-6 dark:border-blue-800 dark:bg-blue-900/20">
                  <div class="flex items-start gap-3">
                    <svg
                      class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                    <div class="flex-1">
                      <p class="text-sm text-blue-800 dark:text-blue-200">
                        <strong>Recommended:</strong> Install the Pulse agent on your Proxmox nodes for extra capabilities like temperature monitoring and Pulse Patrol automation (it also auto-creates the API token and links the node).
                      </p>
                      <p class="mt-1 text-xs text-blue-700 dark:text-blue-300">
                        Prefer not to run an agent on PVE? Use the manual API token setup below.
                      </p>
                      <button
                        type="button"
                        onClick={() => navigate('/settings/workloads')}
                        class="mt-2 text-sm font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200 underline"
                      >
                        Install agent →
                      </button>
                    </div>
                  </div>
                </div>
              </Show>

              {/* PVE Nodes Tab */}
              <Show when={activeTab() === 'proxmox' && selectedAgent() === 'pve'}>
                <div class="space-y-6 mt-6">
                  <div class="space-y-4">
                    <Show when={!initialLoadComplete()}>
                      <div class="flex items-center justify-center rounded-lg border border-dashed border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40 py-12 text-sm text-gray-500 dark:text-gray-400">
                        Loading configuration...
                      </div>
                    </Show>
                    <Show when={initialLoadComplete()}>
                      <Card padding="none" tone="glass">
                        <div class="px-3 py-4 sm:px-6 sm:py-6 space-y-4">
                          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                            <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                              Proxmox VE nodes
                            </h4>
                            <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                              {/* Discovery toggle */}
                              <div
                                class="flex items-center gap-2 sm:gap-3"
                                title="Enable automatic discovery of Proxmox servers on your network"
                              >
                                <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">
                                  Discovery
                                </span>
                                <Toggle
                                  checked={discoveryEnabled()}
                                  onChange={async (e: ToggleChangeEvent) => {
                                    if (
                                      envOverrides().discoveryEnabled ||
                                      savingDiscoverySettings()
                                    ) {
                                      e.preventDefault();
                                      return;
                                    }
                                    const success = await handleDiscoveryEnabledChange(
                                      e.currentTarget.checked,
                                    );
                                    if (!success) {
                                      e.currentTarget.checked = discoveryEnabled();
                                    }
                                  }}
                                  disabled={
                                    envOverrides().discoveryEnabled || savingDiscoverySettings()
                                  }
                                  containerClass="gap-2"
                                  label={
                                    <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                      {discoveryEnabled() ? 'On' : 'Off'}
                                    </span>
                                  }
                                />
                              </div>

                              <Show when={discoveryEnabled()}>
                                <button
                                  type="button"
                                  onClick={async () => {
                                    notificationStore.info('Refreshing discovery...', 2000);
                                    try {
                                      await triggerDiscoveryScan({ quiet: true });
                                    } finally {
                                      await loadDiscoveredNodes();
                                    }
                                  }}
                                  class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
                                  title="Refresh discovered servers"
                                >
                                  <svg
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <polyline points="23 4 23 10 17 10"></polyline>
                                    <polyline points="1 20 1 14 7 14"></polyline>
                                    <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                                  </svg>
                                  <span class="hidden sm:inline">Refresh</span>
                                </button>
                              </Show>

                              <button
                                type="button"
                                onClick={() => {
                                  setEditingNode(null);
                                  setCurrentNodeType('pve');
                                  setModalResetKey((prev) => prev + 1);
                                  setShowNodeModal(true);
                                }}
                                class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
                              >
                                <svg
                                  width="16"
                                  height="16"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2"
                                >
                                  <line x1="12" y1="5" x2="12" y2="19"></line>
                                  <line x1="5" y1="12" x2="19" y2="12"></line>
                                </svg>
                                <span class="sm:hidden">Add</span>
                                <span class="hidden sm:inline">Add PVE Node</span>
                              </button>
                            </div>
                          </div>

                          <Show when={pveNodes().length > 0}>
                            <PveNodesTable
                              nodes={pveNodes()}
                              stateNodes={state.nodes ?? []}
                              stateHosts={state.hosts ?? []}
                              globalTemperatureMonitoringEnabled={temperatureMonitoringEnabled()}
                              onTestConnection={testNodeConnection}
                              onEdit={(node) => {
                                setEditingNode(node);
                                setCurrentNodeType('pve');
                                setShowNodeModal(true);
                              }}
                              onDelete={requestDeleteNode}
                              onRefreshCluster={refreshClusterNodes}
                            />
                          </Show>

                          <Show
                            when={
                              pveNodes().length === 0 &&
                              discoveredNodes().filter((n) => n.type === 'pve').length === 0
                            }
                          >
                            <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                              <div class="rounded-full bg-gray-100 dark:bg-gray-800 p-4 mb-4">
                                <Server class="h-8 w-8 text-gray-400 dark:text-gray-500" />
                              </div>
                              <p class="text-base font-medium text-gray-900 dark:text-gray-100 mb-1">
                                No PVE nodes configured
                              </p>
                              <p class="text-sm text-gray-500 dark:text-gray-400">
                                Add a Proxmox VE node to start monitoring your infrastructure
                              </p>
                            </div>
                          </Show>
                        </div>
                      </Card>
                    </Show>

                    {/* Discovered PVE nodes - only show when discovery is enabled */}
                    <Show when={discoveryEnabled()}>
                      <div class="space-y-3">
                        <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                          <Show when={discoveryScanStatus().scanning}>
                            <span class="flex items-center gap-2">
                              <Loader class="h-4 w-4 animate-spin" />
                              <span>Scanning your network for Proxmox VE servers…</span>
                            </span>
                          </Show>
                          <Show
                            when={
                              !discoveryScanStatus().scanning &&
                              (discoveryScanStatus().lastResultAt ||
                                discoveryScanStatus().lastScanStartedAt)
                            }
                          >
                            <span>
                              Last scan{' '}
                              {formatRelativeTime(
                                discoveryScanStatus().lastResultAt ??
                                discoveryScanStatus().lastScanStartedAt,
                                { compact: true },
                              )}
                            </span>
                          </Show>
                        </div>
                        <Show
                          when={
                            discoveryScanStatus().errors && discoveryScanStatus().errors!.length
                          }
                        >
                          <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                            <span class="font-medium">Discovery issues:</span>
                            <ul class="list-disc ml-4 mt-1 space-y-0.5">
                              <For each={discoveryScanStatus().errors || []}>
                                {(err) => <li>{err}</li>}
                              </For>
                            </ul>
                            <Show
                              when={
                                discoveryMode() === 'auto' &&
                                (discoveryScanStatus().errors || []).some((err) =>
                                  /timed out|timeout/i.test(err),
                                )
                              }
                            >
                              <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                                Large networks can time out in auto mode. Switch to a custom subnet
                                for faster, targeted scans.
                              </p>
                            </Show>
                          </div>
                        </Show>
                        <Show
                          when={
                            discoveryScanStatus().scanning &&
                            discoveredNodes().filter((n) => n.type === 'pve').length === 0
                          }
                        >
                          <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                            <Loader class="h-4 w-4 animate-spin" />
                            <span>
                              Waiting for responses… this can take up to a minute depending on your
                              network size.
                            </span>
                          </div>
                        </Show>
                        <For each={discoveredNodes().filter((n) => n.type === 'pve')}>
                          {(server) => (
                            <div
                              class="bg-gray-50/50 dark:bg-gray-700/30 rounded-lg p-4 border border-gray-200/50 dark:border-gray-600/50 opacity-75 hover:opacity-100 transition-opacity cursor-pointer"
                              onClick={() => {
                                // Pre-fill the modal with discovered server info
                                setEditingNode({
                                  id: '',
                                  type: 'pve',
                                  name: server.hostname || `pve-${server.ip}`,
                                  host: `https://${server.ip}:${server.port}`,
                                  user: '',
                                  tokenName: '',
                                  tokenValue: '',
                                  verifySSL: false,
                                  monitorVMs: true,
                                  monitorContainers: true,
                                  monitorStorage: true,
                                  monitorBackups: true,
                                  monitorPhysicalDisks: false,
                                  status: 'pending',
                                } as NodeConfigWithStatus);
                                setCurrentNodeType('pve');
                                setShowNodeModal(true);
                              }}
                            >
                              <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                                <div class="flex-1 min-w-0">
                                  <div class="flex items-start gap-3">
                                    <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                                    <div class="flex-1 min-w-0">
                                      <h4 class="font-medium text-gray-700 dark:text-gray-300">
                                        {server.hostname || `Proxmox VE at ${server.ip}`}
                                      </h4>
                                      <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                                        {server.ip}:{server.port}
                                      </p>
                                      <div class="flex items-center gap-2 mt-2">
                                        <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                          Discovered
                                        </span>
                                        <span class="text-xs text-gray-500 dark:text-gray-400">
                                          Click to configure
                                        </span>
                                      </div>
                                    </div>
                                  </div>
                                </div>
                                <svg
                                  width="20"
                                  height="20"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  class="text-gray-400 mt-1"
                                >
                                  <path
                                    d="M12 5v14m-7-7h14"
                                    stroke="currentColor"
                                    stroke-width="2"
                                    stroke-linecap="round"
                                  />
                                </svg>
                              </div>
                            </div>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </div>
              </Show>

              {/* PBS Nodes Tab */}
              <Show when={activeTab() === 'proxmox' && selectedAgent() === 'pbs'}>
                <div class="space-y-6 mt-6">
                  <div class="space-y-4">
                    <Show when={!initialLoadComplete()}>
                      <div class="flex items-center justify-center rounded-lg border border-dashed border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40 py-12 text-sm text-gray-500 dark:text-gray-400">
                        Loading configuration...
                      </div>
                    </Show>
                    <Show when={initialLoadComplete()}>
                      <Card padding="none" tone="glass">
                        <div class="px-3 py-4 sm:px-6 sm:py-6 space-y-4">
                          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                            <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                              Proxmox Backup Server nodes
                            </h4>
                            <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                              {/* Discovery toggle */}
                              <div
                                class="flex items-center gap-2 sm:gap-3"
                                title="Enable automatic discovery of PBS servers on your network"
                              >
                                <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">
                                  Discovery
                                </span>
                                <Toggle
                                  checked={discoveryEnabled()}
                                  onChange={async (e: ToggleChangeEvent) => {
                                    if (
                                      envOverrides().discoveryEnabled ||
                                      savingDiscoverySettings()
                                    ) {
                                      e.preventDefault();
                                      return;
                                    }
                                    const success = await handleDiscoveryEnabledChange(
                                      e.currentTarget.checked,
                                    );
                                    if (!success) {
                                      e.currentTarget.checked = discoveryEnabled();
                                    }
                                  }}
                                  disabled={
                                    envOverrides().discoveryEnabled || savingDiscoverySettings()
                                  }
                                  containerClass="gap-2"
                                  label={
                                    <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                      {discoveryEnabled() ? 'On' : 'Off'}
                                    </span>
                                  }
                                />
                              </div>

                              <Show when={discoveryEnabled()}>
                                <button
                                  type="button"
                                  onClick={async () => {
                                    notificationStore.info('Refreshing discovery...', 2000);
                                    try {
                                      await triggerDiscoveryScan({ quiet: true });
                                    } finally {
                                      await loadDiscoveredNodes();
                                    }
                                  }}
                                  class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
                                  title="Refresh discovered servers"
                                >
                                  <svg
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <polyline points="23 4 23 10 17 10"></polyline>
                                    <polyline points="1 20 1 14 7 14"></polyline>
                                    <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                                  </svg>
                                  <span class="hidden sm:inline">Refresh</span>
                                </button>
                              </Show>

                              <button
                                type="button"
                                onClick={() => {
                                  setEditingNode(null);
                                  setCurrentNodeType('pbs');
                                  setModalResetKey((prev) => prev + 1);
                                  setShowNodeModal(true);
                                }}
                                class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
                              >
                                <svg
                                  width="16"
                                  height="16"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2"
                                >
                                  <line x1="12" y1="5" x2="12" y2="19"></line>
                                  <line x1="5" y1="12" x2="19" y2="12"></line>
                                </svg>
                                <span class="sm:hidden">Add</span>
                                <span class="hidden sm:inline">Add PBS Node</span>
                              </button>
                            </div>
                          </div>

                          <Show when={pbsNodes().length > 0}>
                            <PbsNodesTable
                              nodes={pbsNodes()}
                              statePbs={state.pbs ?? []}
                              globalTemperatureMonitoringEnabled={temperatureMonitoringEnabled()}
                              onTestConnection={testNodeConnection}
                              onEdit={(node) => {
                                setEditingNode(node);
                                setCurrentNodeType('pbs');
                                setShowNodeModal(true);
                              }}
                              onDelete={requestDeleteNode}
                            />
                          </Show>

                          <Show
                            when={
                              pbsNodes().length === 0 &&
                              discoveredNodes().filter((n) => n.type === 'pbs').length === 0
                            }
                          >
                            <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                              <div class="rounded-full bg-gray-100 dark:bg-gray-800 p-4 mb-4">
                                <HardDrive class="h-8 w-8 text-gray-400 dark:text-gray-500" />
                              </div>
                              <p class="text-base font-medium text-gray-900 dark:text-gray-100 mb-1">
                                No PBS nodes configured
                              </p>
                              <p class="text-sm text-gray-500 dark:text-gray-400">
                                Add a Proxmox Backup Server to monitor your backups
                              </p>
                            </div>
                          </Show>
                        </div>
                      </Card>
                    </Show>

                    {/* Discovered PBS nodes - only show when discovery is enabled */}
                    <Show when={discoveryEnabled()}>
                      <div class="space-y-3">
                        <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                          <Show when={discoveryScanStatus().scanning}>
                            <span class="flex items-center gap-2">
                              <Loader class="h-4 w-4 animate-spin" />
                              <span>Scanning your network for Proxmox Backup Servers…</span>
                            </span>
                          </Show>
                          <Show
                            when={
                              !discoveryScanStatus().scanning &&
                              (discoveryScanStatus().lastResultAt ||
                                discoveryScanStatus().lastScanStartedAt)
                            }
                          >
                            <span>
                              Last scan{' '}
                              {formatRelativeTime(
                                discoveryScanStatus().lastResultAt ??
                                discoveryScanStatus().lastScanStartedAt,
                                { compact: true },
                              )}
                            </span>
                          </Show>
                        </div>
                        <Show
                          when={
                            discoveryScanStatus().errors && discoveryScanStatus().errors!.length
                          }
                        >
                          <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                            <span class="font-medium">Discovery issues:</span>
                            <ul class="list-disc ml-4 mt-1 space-y-0.5">
                              <For each={discoveryScanStatus().errors || []}>
                                {(err) => <li>{err}</li>}
                              </For>
                            </ul>
                            <Show
                              when={
                                discoveryMode() === 'auto' &&
                                (discoveryScanStatus().errors || []).some((err) =>
                                  /timed out|timeout/i.test(err),
                                )
                              }
                            >
                              <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                                Large networks can time out in auto mode. Switch to a custom subnet
                                for faster, targeted scans.
                              </p>
                            </Show>
                          </div>
                        </Show>
                        <Show
                          when={
                            discoveryScanStatus().scanning &&
                            discoveredNodes().filter((n) => n.type === 'pbs').length === 0
                          }
                        >
                          <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                            <Loader class="h-4 w-4 animate-spin" />
                            <span>
                              Waiting for responses… this can take up to a minute depending on your
                              network size.
                            </span>
                          </div>
                        </Show>
                        <For each={discoveredNodes().filter((n) => n.type === 'pbs')}>
                          {(server) => (
                            <div
                              class="bg-gray-50/50 dark:bg-gray-700/30 rounded-lg p-4 border border-gray-200/50 dark:border-gray-600/50 opacity-75 hover:opacity-100 transition-opacity cursor-pointer"
                              onClick={() => {
                                // Pre-fill the modal with discovered server info
                                setEditingNode({
                                  id: '',
                                  type: 'pbs',
                                  name: server.hostname || `pbs-${server.ip}`,
                                  host: `https://${server.ip}:${server.port}`,
                                  user: '',
                                  tokenName: '',
                                  tokenValue: '',
                                  verifySSL: false,
                                  monitorDatastores: true,
                                  monitorSyncJobs: true,
                                  monitorVerifyJobs: true,
                                  monitorPruneJobs: true,
                                  monitorGarbageJobs: true,
                                  status: 'pending',
                                } as NodeConfigWithStatus);
                                setCurrentNodeType('pbs');
                                setShowNodeModal(true);
                              }}
                            >
                              <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                                <div class="flex-1 min-w-0">
                                  <div class="flex items-start gap-3">
                                    <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                                    <div class="flex-1 min-w-0">
                                      <h4 class="font-medium text-gray-700 dark:text-gray-300">
                                        {server.hostname || `Backup Server at ${server.ip}`}
                                      </h4>
                                      <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                                        {server.ip}:{server.port}
                                      </p>
                                      <div class="flex items-center gap-2 mt-2">
                                        <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                          Discovered
                                        </span>
                                        <span class="text-xs text-gray-500 dark:text-gray-400">
                                          Click to configure
                                        </span>
                                      </div>
                                    </div>
                                  </div>
                                </div>
                                <svg
                                  width="20"
                                  height="20"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  class="text-gray-400 mt-1"
                                >
                                  <path
                                    d="M12 5v14m-7-7h14"
                                    stroke="currentColor"
                                    stroke-width="2"
                                    stroke-linecap="round"
                                  />
                                </svg>
                              </div>
                            </div>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </div>
              </Show>
              {/* PMG Nodes Tab */}
              <Show when={activeTab() === 'proxmox' && selectedAgent() === 'pmg'}>
                <div class="space-y-6 mt-6">
                  <div class="space-y-4">
                    <Show when={!initialLoadComplete()}>
                      <div class="flex items-center justify-center rounded-lg border border-dashed border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40 py-12 text-sm text-gray-500 dark:text-gray-400">
                        Loading configuration...
                      </div>
                    </Show>

                    <Show when={initialLoadComplete()}>
                      <Card padding="none" tone="glass">
                        <div class="px-3 py-4 sm:px-6 sm:py-6 space-y-4">
                          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                            <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                              Proxmox Mail Gateway nodes
                            </h4>
                            <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                              {/* Discovery toggle */}
                              <div
                                class="flex items-center gap-2 sm:gap-3"
                                title="Enable automatic discovery of PMG servers on your network"
                              >
                                <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">
                                  Discovery
                                </span>
                                <Toggle
                                  checked={discoveryEnabled()}
                                  onChange={async (e: ToggleChangeEvent) => {
                                    if (
                                      envOverrides().discoveryEnabled ||
                                      savingDiscoverySettings()
                                    ) {
                                      e.preventDefault();
                                      return;
                                    }
                                    const success = await handleDiscoveryEnabledChange(
                                      e.currentTarget.checked,
                                    );
                                    if (!success) {
                                      e.currentTarget.checked = discoveryEnabled();
                                    }
                                  }}
                                  disabled={
                                    envOverrides().discoveryEnabled || savingDiscoverySettings()
                                  }
                                  containerClass="gap-2"
                                  label={
                                    <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                                      {discoveryEnabled() ? 'On' : 'Off'}
                                    </span>
                                  }
                                />
                              </div>

                              <Show when={discoveryEnabled()}>
                                <button
                                  type="button"
                                  onClick={async () => {
                                    notificationStore.info('Refreshing discovery...', 2000);
                                    try {
                                      await triggerDiscoveryScan({ quiet: true });
                                    } finally {
                                      await loadDiscoveredNodes();
                                    }
                                  }}
                                  class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
                                  title="Refresh discovered servers"
                                >
                                  <svg
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <polyline points="23 4 23 10 17 10"></polyline>
                                    <polyline points="1 20 1 14 7 14"></polyline>
                                    <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
                                  </svg>
                                  <span class="hidden sm:inline">Refresh</span>
                                </button>
                              </Show>

                              <button
                                type="button"
                                onClick={() => {
                                  setEditingNode(null);
                                  setCurrentNodeType('pmg');
                                  setModalResetKey((prev) => prev + 1);
                                  setShowNodeModal(true);
                                }}
                                class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
                              >
                                <svg
                                  width="16"
                                  height="16"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2"
                                >
                                  <line x1="12" y1="5" x2="12" y2="19"></line>
                                  <line x1="5" y1="12" x2="19" y2="12"></line>
                                </svg>
                                <span class="sm:hidden">Add</span>
                                <span class="hidden sm:inline">Add PMG Node</span>
                              </button>
                            </div>
                          </div>

                          <Show when={pmgNodes().length > 0}>
                            <PmgNodesTable
                              nodes={pmgNodes()}
                              statePmg={state.pmg ?? []}
                              globalTemperatureMonitoringEnabled={temperatureMonitoringEnabled()}
                              onTestConnection={testNodeConnection}
                              onEdit={(node) => {
                                setEditingNode(node);
                                setCurrentNodeType('pmg');
                                setModalResetKey((prev) => prev + 1);
                                setShowNodeModal(true);
                              }}
                              onDelete={requestDeleteNode}
                            />
                          </Show>

                          <Show when={pmgNodes().length === 0}>
                            <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                              <div class="rounded-full bg-gray-100 dark:bg-gray-800 p-4 mb-4">
                                <Mail class="h-8 w-8 text-gray-400 dark:text-gray-500" />
                              </div>
                              <p class="text-base font-medium text-gray-900 dark:text-gray-100 mb-1">
                                No PMG nodes configured
                              </p>
                              <p class="text-sm text-gray-500 dark:text-gray-400">
                                Add a Proxmox Mail Gateway node to start monitoring
                              </p>
                            </div>
                          </Show>
                        </div>
                      </Card>
                    </Show>

                    {/* Discovered PMG nodes - only show when discovery is enabled */}
                    <Show when={discoveryEnabled()}>
                      <div class="space-y-3">
                        <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
                          <Show when={discoveryScanStatus().scanning}>
                            <span class="flex items-center gap-2">
                              <Loader class="h-4 w-4 animate-spin" />
                              Scanning network...
                            </span>
                          </Show>
                          <Show
                            when={
                              !discoveryScanStatus().scanning &&
                              (discoveryScanStatus().lastResultAt ||
                                discoveryScanStatus().lastScanStartedAt)
                            }
                          >
                            <span>
                              Last scan{' '}
                              {formatRelativeTime(
                                discoveryScanStatus().lastResultAt ??
                                discoveryScanStatus().lastScanStartedAt,
                                { compact: true },
                              )}
                            </span>
                          </Show>
                        </div>
                        <Show
                          when={
                            discoveryScanStatus().errors && discoveryScanStatus().errors!.length
                          }
                        >
                          <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                            <span class="font-medium">Discovery issues:</span>
                            <ul class="list-disc ml-4 mt-1 space-y-0.5">
                              <For each={discoveryScanStatus().errors || []}>
                                {(err) => <li>{err}</li>}
                              </For>
                            </ul>
                            <Show
                              when={
                                discoveryMode() === 'auto' &&
                                (discoveryScanStatus().errors || []).some((err) =>
                                  /timed out|timeout/i.test(err),
                                )
                              }
                            >
                              <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                                Large networks can time out in auto mode. Switch to a custom subnet
                                for faster, targeted scans.
                              </p>
                            </Show>
                          </div>
                        </Show>
                        <Show
                          when={
                            discoveryScanStatus().scanning &&
                            discoveredNodes().filter((n) => n.type === 'pmg').length === 0
                          }
                        >
                          <div class="text-center py-6 text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800/50 rounded-lg border-2 border-dashed border-gray-300 dark:border-gray-600">
                            <svg
                              class="h-8 w-8 mx-auto mb-2 animate-pulse text-gray-400"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                            >
                              <circle cx="11" cy="11" r="8" />
                              <path d="m21 21-4.35-4.35" />
                            </svg>
                            <p class="text-sm">Scanning for PMG servers...</p>
                          </div>
                        </Show>
                        <For each={discoveredNodes().filter((n) => n.type === 'pmg')}>
                          {(server) => (
                            <div
                              class="bg-gray-50 dark:bg-gray-800/50 border-l-4 border-gray-400 rounded-lg p-4 cursor-pointer hover:shadow-md transition-all"
                              onClick={() => {
                                setEditingNode(null);
                                setCurrentNodeType('pmg');
                                setModalResetKey((prev) => prev + 1);
                                setShowNodeModal(true);
                                setTimeout(() => {
                                  const hostInput = document.querySelector(
                                    'input[placeholder*="192.168"]',
                                  ) as HTMLInputElement;
                                  if (hostInput) {
                                    hostInput.value = server.ip;
                                    hostInput.dispatchEvent(new Event('input', { bubbles: true }));
                                  }
                                }, 50);
                              }}
                            >
                              <div class="flex items-start justify-between">
                                <div class="flex items-start gap-3 flex-1 min-w-0">
                                  <svg
                                    width="24"
                                    height="24"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                    class="text-gray-500 flex-shrink-0 mt-0.5"
                                  >
                                    <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
                                    <polyline points="22,6 12,13 2,6"></polyline>
                                  </svg>
                                  <div class="flex-1 min-w-0">
                                    <h4 class="font-medium text-gray-900 dark:text-gray-100 truncate">
                                      {server.hostname || `PMG at ${server.ip}`}
                                    </h4>
                                    <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                                      {server.ip}:{server.port}
                                    </p>
                                    <div class="flex items-center gap-2 mt-2">
                                      <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                        Discovered
                                      </span>
                                      <span class="text-xs text-gray-500 dark:text-gray-400">
                                        Click to configure
                                      </span>
                                    </div>
                                  </div>
                                </div>
                                <svg
                                  width="20"
                                  height="20"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  class="text-gray-400 mt-1"
                                >
                                  <path
                                    d="M12 5v14m-7-7h14"
                                    stroke="currentColor"
                                    stroke-width="2"
                                    stroke-linecap="round"
                                  />
                                </svg>
                              </div>
                            </div>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </div>
              </Show>
              <Show when={activePanelRender()}>
                {(panel) => {
                  const resolved = panel();
                  return <Dynamic component={resolved.component} {...resolved.props} />;
                }}
              </Show>
            </div>
          </div >
        </Card >
      </div >

      {/* Delete Node Modal */}
      < Show when={showDeleteNodeModal()} >
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <Card padding="lg" class="w-full max-w-lg space-y-5">
            <SectionHeader title={`Remove ${nodePendingDeleteLabel()}`} size="md" class="mb-1" />
            <div class="space-y-3 text-sm text-gray-600 dark:text-gray-300">
              <p>
                Removing this {nodePendingDeleteTypeLabel().toLowerCase()} also scrubs the Pulse
                footprint on the host — the proxy service, SSH key, API token, and bind mount are
                all cleaned up automatically.
              </p>
              <div class="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm leading-relaxed dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-100">
                <p class="font-medium text-blue-900 dark:text-blue-100">What happens next</p>
                <ul class="mt-2 list-disc space-y-1 pl-4 text-blue-800 dark:text-blue-200 text-sm">
                  <li>Pulse removes the node entry and clears related alerts.</li>
                  <li>
                    {nodePendingDeleteHost() ? (
                      <>
                        The host <span class="font-semibold">{nodePendingDeleteHost()}</span> loses
                        the proxy service, SSH key, and API token.
                      </>
                    ) : (
                      'The host loses the proxy service, SSH key, and API token.'
                    )}
                  </li>
                  <li>
                    If the host comes back later, rerunning the setup script reinstalls everything
                    with a fresh key.
                  </li>
                  <Show when={nodePendingDeleteType() === 'pbs'}>
                    <li>
                      Backup user tokens on the PBS are removed, so jobs referencing them will no
                      longer authenticate until the node is re-added.
                    </li>
                  </Show>
                  <Show when={nodePendingDeleteType() === 'pmg'}>
                    <li>
                      Mail gateway tokens are removed as part of the cleanup; re-enroll to restore
                      outbound telemetry.
                    </li>
                  </Show>
                </ul>
              </div>
            </div>

            <div class="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={cancelDeleteNode}
                class="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                disabled={deleteNodeLoading()}
              >
                Keep node
              </button>
              <button
                type="button"
                onClick={deleteNode}
                disabled={deleteNodeLoading()}
                class="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-red-500 dark:hover:bg-red-400"
              >
                {deleteNodeLoading() ? 'Removing…' : 'Remove node'}
              </button>
            </div>
          </Card>
        </div>
      </Show >

      {/* Node Modal - Use separate modals for PVE and PBS to ensure clean state */}
      < Show when={isNodeModalVisible('pve')} >
        <NodeModal
          isOpen={true}
          resetKey={modalResetKey()}
          onClose={() => {
            setShowNodeModal(false);
            setEditingNode(null);
            // Increment resetKey to force form reset on next open
            setModalResetKey((prev) => prev + 1);
          }}
          nodeType="pve"
          editingNode={editingNode()?.type === 'pve' ? (editingNode() ?? undefined) : undefined}
          securityStatus={securityStatus() ?? undefined}
          temperatureMonitoringEnabled={resolveTemperatureMonitoringEnabled(
            editingNode()?.type === 'pve' ? editingNode() : null,
          )}
          temperatureMonitoringLocked={temperatureMonitoringLocked()}
          savingTemperatureSetting={savingTemperatureSetting()}
          onToggleTemperatureMonitoring={
            editingNode()?.id
              ? (enabled: boolean) => handleNodeTemperatureMonitoringChange(editingNode()!.id, enabled)
              : handleTemperatureMonitoringChange
          }
          onSave={saveNode}
        />
      </Show >

      {/* PBS Node Modal - Separate instance to prevent contamination */}
      < Show when={isNodeModalVisible('pbs')} >
        <NodeModal
          isOpen={true}
          resetKey={modalResetKey()}
          onClose={() => {
            setShowNodeModal(false);
            setEditingNode(null);
            // Increment resetKey to force form reset on next open
            setModalResetKey((prev) => prev + 1);
          }}
          nodeType="pbs"
          editingNode={editingNode()?.type === 'pbs' ? (editingNode() ?? undefined) : undefined}
          securityStatus={securityStatus() ?? undefined}
          temperatureMonitoringEnabled={resolveTemperatureMonitoringEnabled(
            editingNode()?.type === 'pbs' ? editingNode() : null,
          )}
          temperatureMonitoringLocked={temperatureMonitoringLocked()}
          savingTemperatureSetting={savingTemperatureSetting()}
          onToggleTemperatureMonitoring={
            editingNode()?.id
              ? (enabled: boolean) => handleNodeTemperatureMonitoringChange(editingNode()!.id, enabled)
              : handleTemperatureMonitoringChange
          }
          onSave={saveNode}
        />
      </Show >

      {/* PMG Node Modal */}
      < Show when={isNodeModalVisible('pmg')} >
        <NodeModal
          isOpen={true}
          resetKey={modalResetKey()}
          onClose={() => {
            setShowNodeModal(false);
            setEditingNode(null);
            setModalResetKey((prev) => prev + 1);
          }}
          nodeType="pmg"
          editingNode={editingNode()?.type === 'pmg' ? (editingNode() ?? undefined) : undefined}
          securityStatus={securityStatus() ?? undefined}
          temperatureMonitoringEnabled={resolveTemperatureMonitoringEnabled(
            editingNode()?.type === 'pmg' ? editingNode() : null,
          )}
          temperatureMonitoringLocked={temperatureMonitoringLocked()}
          savingTemperatureSetting={savingTemperatureSetting()}
          onToggleTemperatureMonitoring={
            editingNode()?.id
              ? (enabled: boolean) => handleNodeTemperatureMonitoringChange(editingNode()!.id, enabled)
              : handleTemperatureMonitoringChange
          }
          onSave={saveNode}
        />
      </Show >

      {/* Update Confirmation Modal */}
      < UpdateConfirmationModal
        isOpen={showUpdateConfirmation()}
        onClose={() => setShowUpdateConfirmation(false)}
        onConfirm={handleConfirmUpdate}
        currentVersion={versionInfo()?.version || 'Unknown'}
        latestVersion={updateInfo()?.latestVersion || ''}
        plan={updatePlan() || {
          canAutoUpdate: false,
          requiresRoot: false,
          rollbackSupport: false,
        }}
        isApplying={isInstallingUpdate()}
      />

      {/* Export Dialog */}
      < Show when={showExportDialog()} >
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="Export configuration" size="md" class="mb-4" />

            <div class="space-y-4">
              {/* Password Choice Section - Only show if auth is enabled */}
              <Show when={securityStatus()?.hasAuthentication}>
                <div class="bg-gray-50 dark:bg-gray-900/50 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
                  <div class="space-y-3">
                    <label class="flex items-start gap-3 cursor-pointer">
                      <input
                        type="radio"
                        checked={!useCustomPassphrase()}
                        onChange={() => {
                          setUseCustomPassphrase(false);
                          setExportPassphrase('');
                        }}
                        class="mt-1 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="flex-1">
                        <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Use your login password
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                          Use the same password you use to log into Pulse (recommended)
                        </div>
                      </div>
                    </label>

                    <label class="flex items-start gap-3 cursor-pointer">
                      <input
                        type="radio"
                        checked={useCustomPassphrase()}
                        onChange={() => setUseCustomPassphrase(true)}
                        class="mt-1 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="flex-1">
                        <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Use a custom passphrase
                        </div>
                        <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                          Create a different passphrase for this backup
                        </div>
                      </div>
                    </label>
                  </div>
                </div>
              </Show>

              {/* Show password input based on selection */}
              <div class={formField}>
                <label class={labelClass()}>
                  {securityStatus()?.hasAuthentication
                    ? useCustomPassphrase()
                      ? 'Custom Passphrase'
                      : 'Enter Your Login Password'
                    : 'Encryption Passphrase'}
                </label>
                <input
                  type="password"
                  value={exportPassphrase()}
                  onInput={(e) => setExportPassphrase(e.currentTarget.value)}
                  placeholder={
                    securityStatus()?.hasAuthentication
                      ? useCustomPassphrase()
                        ? 'Enter a strong passphrase'
                        : 'Enter your Pulse login password'
                      : 'Enter a strong passphrase for encryption'
                  }
                  class={controlClass()}
                />
                <Show when={!securityStatus()?.hasAuthentication || useCustomPassphrase()}>
                  <p class={`${formHelpText} mt-1`}>
                    You'll need this passphrase to restore the backup.
                  </p>
                </Show>
                <Show when={securityStatus()?.hasAuthentication && !useCustomPassphrase()}>
                  <p class={`${formHelpText} mt-1`}>
                    You'll use this same password when restoring the backup
                  </p>
                </Show>
              </div>

              <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
                <div class="flex gap-2">
                  <svg
                    class="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                  <div class="text-xs text-amber-700 dark:text-amber-300">
                    <strong>Important:</strong> The backup contains node credentials but NOT
                    authentication settings. Each Pulse instance should configure its own login
                    credentials for security. Remember your{' '}
                    {useCustomPassphrase() || !securityStatus()?.hasAuthentication
                      ? 'passphrase'
                      : 'password'}{' '}
                    for restoring.
                  </div>
                </div>
              </div>

              <div class="flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={closeExportDialog}
                  class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleExport}
                  disabled={
                    !exportPassphrase() || (useCustomPassphrase() && exportPassphrase().length < 12)
                  }
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Export
                </button>
              </div>
            </div>
          </Card>
        </div>
      </Show >

      {/* API Token Modal */}
      < Show when={showApiTokenModal()} >
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="API token required" size="md" class="mb-4" />

            <div class="space-y-4">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                This Pulse instance requires an API token for export/import operations. Please enter
                the API token configured on the server.
              </p>

              <div class={formField}>
                <label class={labelClass()}>API Token</label>
                <input
                  type="password"
                  value={apiTokenInput()}
                  onInput={(e) => setApiTokenInput(e.currentTarget.value)}
                  placeholder="Enter API token"
                  class={controlClass()}
                />
              </div>

              <div class="text-xs text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 rounded p-2">
                <p class="font-semibold mb-1">The API token is set as an environment variable:</p>
                <code class="block">API_TOKENS=token-for-export,token-for-automation</code>
              </div>
            </div>

            <div class="flex justify-end space-x-2 mt-6">
              <button
                type="button"
                onClick={closeApiTokenModal}
                class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleApiTokenAuthenticate}
                disabled={!apiTokenInput()}
                class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Authenticate
              </button>
            </div>
          </Card>
        </div>
      </Show >

      {/* Import Dialog */}
      < Show when={showImportDialog()} >
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="Import configuration" size="md" class="mb-4" />

            <div class="space-y-4">
              <div class={formField}>
                <label class={labelClass()}>Configuration File</label>
                <input
                  type="file"
                  accept=".json"
                  onChange={(e) => {
                    const file = e.currentTarget.files?.[0];
                    if (file) setImportFile(file);
                  }}
                  class={controlClass('cursor-pointer')}
                />
              </div>

              <div class={formField}>
                <label class={labelClass()}>Backup Password</label>
                <input
                  type="password"
                  value={importPassphrase()}
                  onInput={(e) => setImportPassphrase(e.currentTarget.value)}
                  placeholder="Enter the password used when creating this backup"
                  class={controlClass()}
                />
                <p class={`${formHelpText} mt-1`}>
                  This is usually your Pulse login password, unless you used a custom passphrase
                </p>
              </div>

              <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded p-3">
                <p class="text-xs text-yellow-700 dark:text-yellow-300">
                  <strong>Warning:</strong> Importing will replace all current configuration. This
                  action cannot be undone.
                </p>
              </div>

              <div class="flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={closeImportDialog}
                  class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleImport}
                  disabled={!importPassphrase() || !importFile()}
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Import
                </button>
              </div>
            </div>
          </Card>
        </div>
      </Show >

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
