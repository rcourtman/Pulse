import { Accessor, Component, Setter, createMemo } from 'solid-js';
import type { VersionInfo } from '@/api/updates';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { AISettings } from './AISettings';
import { AICostDashboard } from '@/components/AI/AICostDashboard';
import { ProLicensePanel } from './ProLicensePanel';
import { SSOProvidersPanel } from './SSOProvidersPanel';
import { createSettingsPanelRegistry } from './settingsPanelRegistry';
import type { ProxmoxSettingsPanelProps } from './ProxmoxSettingsPanel';
import type { SettingsSystemPanels } from './useSettingsSystemPanels';

interface UseSettingsPanelRegistryParams {
  securityStatus: Accessor<SecurityStatusInfo | null>;
  securityStatusLoading: Accessor<boolean>;
  organizationMonitoredSystemUsage: Accessor<number>;
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
  versionInfo: Accessor<VersionInfo | null>;
  getInfrastructurePanelProps: () => ProxmoxSettingsPanelProps;
  systemPanels: SettingsSystemPanels;
}

export function useSettingsPanelRegistry(params: UseSettingsPanelRegistryParams) {
  const settingsCapabilities = createMemo(
    () => params.securityStatus()?.settingsCapabilities ?? null,
  );

  const systemAiPanel: Component = () => (
    <div class="space-y-6">
      <AISettings />
      <AICostDashboard />
    </div>
  );

  const systemBillingPanel: Component = () => (
    <div class="space-y-6">
      <ProLicensePanel />
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
      getInfrastructurePanelProps: params.getInfrastructurePanelProps,
      systemGeneralPanel: params.systemPanels.systemGeneralPanel,
      systemAiPanel,
      systemBillingPanel,
      securitySsoPanel,
      getNetworkPanelProps: params.systemPanels.getNetworkPanelProps,
      getUpdatesPanelProps: params.systemPanels.getUpdatesPanelProps,
      getRecoveryPanelProps: params.systemPanels.getRecoveryPanelProps,
      getOrganizationOverviewPanelProps: () => ({}),
      getOrganizationAccessPanelProps: () => ({}),
      getOrganizationSharingPanelProps: () => ({}),
      getOrganizationBillingPanelProps: () => ({
        nodeUsage: params.organizationMonitoredSystemUsage(),
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
