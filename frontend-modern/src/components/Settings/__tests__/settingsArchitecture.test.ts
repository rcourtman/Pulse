import { describe, expect, it } from 'vitest';
import settingsSource from '../Settings.tsx?raw';
import settingsDialogsSource from '../SettingsDialogs.tsx?raw';
import settingsShellSource from '../SettingsPageShell.tsx?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import infrastructureWorkspaceModelSource from '../infrastructureWorkspaceModel.ts?raw';
import infrastructureInstallPanelSource from '../InfrastructureInstallPanel.tsx?raw';
import infrastructureInstallerSectionSource from '../InfrastructureInstallerSection.tsx?raw';
import infrastructureOperationsControllerSource from '../InfrastructureOperationsController.tsx?raw';
import infrastructureOperationsModelSource from '../infrastructureOperationsModel.tsx?raw';
import infrastructureReportingPanelSource from '../InfrastructureReportingPanel.tsx?raw';
import infrastructureDirectConnectionsSummaryCardSource from '../InfrastructureDirectConnectionsSummaryCard.tsx?raw';
import infrastructureInventorySectionSource from '../InfrastructureInventorySection.tsx?raw';
import infrastructureActiveRowDetailsSource from '../InfrastructureActiveRowDetails.tsx?raw';
import infrastructureIgnoredRowDetailsSource from '../InfrastructureIgnoredRowDetails.tsx?raw';
import infrastructureStopMonitoringDialogSource from '../InfrastructureStopMonitoringDialog.tsx?raw';
import nodeModalAuthenticationSectionSource from '../NodeModalAuthenticationSection.tsx?raw';
import nodeModalBasicInfoSectionSource from '../NodeModalBasicInfoSection.tsx?raw';
import nodeModalModelSource from '../nodeModalModel.ts?raw';
import nodeModalMonitoringSectionSource from '../NodeModalMonitoringSection.tsx?raw';
import nodeModalSetupGuideSectionSource from '../NodeModalSetupGuideSection.tsx?raw';
import nodeModalStatusFooterSource from '../NodeModalStatusFooter.tsx?raw';
import nodeModalSource from '../NodeModal.tsx?raw';
import infrastructureInstallStateSource from '../useInfrastructureInstallState.tsx?raw';
import infrastructureOperationsStateSource from '../useInfrastructureOperationsState.tsx?raw';
import infrastructureReportingStateSource from '../useInfrastructureReportingState.tsx?raw';
import infrastructureSettingsStateSource from '../useInfrastructureSettingsState.ts?raw';
import infrastructureSettingsModelSource from '../infrastructureSettingsModel.ts?raw';
import infrastructureConfiguredNodesStateSource from '../useInfrastructureConfiguredNodesState.ts?raw';
import infrastructureDiscoveryRuntimeStateSource from '../useInfrastructureDiscoveryRuntimeState.ts?raw';
import settingsNavigationModelSource from '../settingsNavigationModel.ts?raw';
import settingsRoutingSource from '../settingsRouting.ts?raw';
import settingsTypesSource from '../settingsTypes.ts?raw';
import settingsNavCatalogSource from '../settingsNavCatalog.ts?raw';
import settingsNavVisibilitySource from '../settingsNavVisibility.ts?raw';
import settingsInfrastructurePanelPropsSource from '../useSettingsInfrastructurePanelProps.ts?raw';
import settingsNavigationHookSource from '../useSettingsNavigation.ts?raw';
import settingsShellStateSource from '../useSettingsShellState.ts?raw';
import nodeModalStateSource from '../useNodeModalState.ts?raw';
import settingsPanelRegistryContextSource from '../settingsPanelRegistryContext.tsx?raw';
import settingsPanelRegistryLoadersSource from '../settingsPanelRegistryLoaders.ts?raw';
import settingsTabSaveBehaviorSource from '../settingsTabSaveBehavior.ts?raw';
import settingsPanelRegistryHookSource from '../useSettingsPanelRegistry.tsx?raw';
import settingsSystemPanelsSource from '../useSettingsSystemPanels.tsx?raw';
import apiAccessPanelSource from '../APIAccessPanel.tsx?raw';
import apiTokenManagerSource from '../APITokenManager.tsx?raw';
import apiTokenManagerModelSource from '../apiTokenManagerModel.ts?raw';
import apiTokenManagerStateSource from '../useAPITokenManagerState.ts?raw';
import auditLogPanelSource from '../AuditLogPanel.tsx?raw';
import auditLogStateSource from '../useAuditLogPanelState.ts?raw';
import auditWebhookPanelSource from '../AuditWebhookPanel.tsx?raw';
import auditWebhookStateSource from '../useAuditWebhookPanelState.ts?raw';
import billingAdminOrganizationsTableSource from '../BillingAdminOrganizationsTable.tsx?raw';
import billingAdminPanelSource from '../BillingAdminPanel.tsx?raw';
import billingAdminPanelStateSource from '../useBillingAdminPanelState.ts?raw';
import generalSettingsPanelSource from '../GeneralSettingsPanel.tsx?raw';
import aiSettingsPanelSource from '../AISettings.tsx?raw';
import aiChatMaintenanceSectionSource from '../AIChatMaintenanceSection.tsx?raw';
import aiProviderConfigurationSectionSource from '../AIProviderConfigurationSection.tsx?raw';
import aiSettingsDialogsSource from '../AISettingsDialogs.tsx?raw';
import aiModelSelectionSectionSource from '../AIModelSelectionSection.tsx?raw';
import aiSettingsModelSource from '../aiSettingsModel.ts?raw';
import aiRuntimeControlsSectionSource from '../AIRuntimeControlsSection.tsx?raw';
import aiSettingsStatusAndActionsSource from '../AISettingsStatusAndActions.tsx?raw';
import aiSettingsStateSource from '../useAISettingsState.ts?raw';
import diagnosticsModelSource from '../diagnosticsModel.ts?raw';
import diagnosticsPanelSource from '../DiagnosticsPanel.tsx?raw';
import diagnosticsResultsPanelSource from '../DiagnosticsResultsPanel.tsx?raw';
import operationsPanelSource from '../OperationsPanel.tsx?raw';
import networkBoundarySettingsSectionSource from '../NetworkBoundarySettingsSection.tsx?raw';
import networkDiscoverySectionSource from '../NetworkDiscoverySection.tsx?raw';
import networkSettingsPanelSource from '../NetworkSettingsPanel.tsx?raw';
import networkSettingsModelSource from '../networkSettingsModel.ts?raw';
import copyCommandBlockSource from '../CopyCommandBlock.tsx?raw';
import updateInstallGuideSource from '../UpdateInstallGuide.tsx?raw';
import updatesSettingsModelSource from '../updatesSettingsModel.ts?raw';
import updatesSettingsPanelSource from '../UpdatesSettingsPanel.tsx?raw';
import recoverySettingsPanelSource from '../RecoverySettingsPanel.tsx?raw';
import systemSettingsStateSource from '../useSystemSettingsState.ts?raw';
import relaySettingsPanelSource from '../RelaySettingsPanel.tsx?raw';
import relayPairingSectionSource from '../RelayPairingSection.tsx?raw';
import monitoredSystemLedgerPanelSource from '../MonitoredSystemLedgerPanel.tsx?raw';
import proLicensePanelSource from '../ProLicensePanel.tsx?raw';
import monitoredSystemDefinitionDisclosureSource from '@/components/Commercial/MonitoredSystemDefinitionDisclosure.tsx?raw';
import proLicensePlanSectionSource from '../ProLicensePlanSection.tsx?raw';
import commercialBillingSectionsSource from '../CommercialBillingSections.tsx?raw';
import selfHostedCommercialActivationSectionSource from '../SelfHostedCommercialActivationSection.tsx?raw';
import commercialBillingModelSource from '@/utils/commercialBillingModel.ts?raw';
import monitoredSystemPresentationSource from '@/utils/monitoredSystemPresentation.ts?raw';
import relaySettingsPanelStateSource from '../useRelaySettingsPanelState.ts?raw';
import proLicensePanelStateSource from '../useProLicensePanelState.ts?raw';
import organizationOverviewPanelSource from '../OrganizationOverviewPanel.tsx?raw';
import organizationOverviewLoadingStateSource from '../OrganizationOverviewLoadingState.tsx?raw';
import organizationOverviewDetailsSectionSource from '../OrganizationOverviewDetailsSection.tsx?raw';
import organizationOverviewMembersSectionSource from '../OrganizationOverviewMembersSection.tsx?raw';
import organizationAccessPanelSource from '../OrganizationAccessPanel.tsx?raw';
import organizationAccessLoadingStateSource from '../OrganizationAccessLoadingState.tsx?raw';
import organizationAccessManagementSectionSource from '../OrganizationAccessManagementSection.tsx?raw';
import organizationAccessMembersSectionSource from '../OrganizationAccessMembersSection.tsx?raw';
import organizationSharingPanelSource from '../OrganizationSharingPanel.tsx?raw';
import organizationSharingCreateSectionSource from '../OrganizationSharingCreateSection.tsx?raw';
import organizationSharingLoadingStateSource from '../OrganizationSharingLoadingState.tsx?raw';
import organizationOutgoingSharesSectionSource from '../OrganizationOutgoingSharesSection.tsx?raw';
import organizationIncomingSharesSectionSource from '../OrganizationIncomingSharesSection.tsx?raw';
import organizationAccessStateSource from '../useOrganizationAccessPanelState.ts?raw';
import organizationOverviewStateSource from '../useOrganizationOverviewPanelState.ts?raw';
import organizationSharingStateSource from '../useOrganizationSharingPanelState.ts?raw';
import organizationBillingLoadingStateSource from '../OrganizationBillingLoadingState.tsx?raw';
import organizationBillingPanelSource from '../OrganizationBillingPanel.tsx?raw';
import organizationBillingStateSource from '../useOrganizationBillingPanelState.ts?raw';
import proxmoxDeleteNodeDialogSource from '../ProxmoxDeleteNodeDialog.tsx?raw';
import proxmoxConfiguredNodesTableSource from '../ProxmoxConfiguredNodesTable.tsx?raw';
import proxmoxDirectWorkspaceSource from '../ProxmoxDirectWorkspace.tsx?raw';
import proxmoxDirectConnectionsCardSource from '../ProxmoxDirectConnectionsCard.tsx?raw';
import proxmoxDiscoveryResultsCardSource from '../ProxmoxDiscoveryResultsCard.tsx?raw';
import proxmoxNodeModalStackSource from '../ProxmoxNodeModalStack.tsx?raw';
import proxmoxSettingsModelSource from '../proxmoxSettingsModel.ts?raw';
import proxmoxSettingsPanelSource from '../ProxmoxSettingsPanel.tsx?raw';
import proxmoxDirectWorkspaceStateSource from '../useProxmoxDirectWorkspaceState.ts?raw';
import securityOverviewPanelSource from '../SecurityOverviewPanel.tsx?raw';
import securityAuthPanelSource from '../SecurityAuthPanel.tsx?raw';
import ssoProvidersPanelSource from '../SSOProvidersPanel.tsx?raw';
import ssoProvidersStateSource from '../useSSOProvidersState.ts?raw';
import ssoProvidersModelSource from '../ssoProvidersModel.ts?raw';
import ssoProviderPresentationSource from '@/utils/ssoProviderPresentation.ts?raw';
import systemSettingsPresentationSource from '@/utils/systemSettingsPresentation.ts?raw';
import updatesPresentationSource from '@/utils/updatesPresentation.ts?raw';
import diagnosticsStateSource from '../useDiagnosticsPanelState.ts?raw';
import reportingCatalogModelSource from '../reportingCatalogModel.ts?raw';
import reportingPanelModelSource from '../reportingPanelModel.ts?raw';
import reportingInventoryExportModelSource from '../reportingInventoryExportModel.ts?raw';
import reportingPanelSource from '../ReportingPanel.tsx?raw';
import reportingPanelStateSource from '../useReportingPanelState.ts?raw';
import systemLogsPanelSource from '../SystemLogsPanel.tsx?raw';
import systemLogsPanelStateSource from '../useSystemLogsPanelState.ts?raw';
import rbacFeatureGateSectionSource from '../RBACFeatureGateSection.tsx?raw';
import rbacFeatureGateStateSource from '../useRBACFeatureGateState.ts?raw';
import rolesPanelSource from '../RolesPanel.tsx?raw';
import rolesEditorDialogSource from '../RolesEditorDialog.tsx?raw';
import rolesPanelStateSource from '../useRolesPanelState.ts?raw';
import userAssignmentsPanelSource from '../UserAssignmentsPanel.tsx?raw';
import userAssignmentsDialogSource from '../UserAssignmentsDialog.tsx?raw';
import userAssignmentsPanelStateSource from '../useUserAssignmentsPanelState.ts?raw';
import { SETTINGS_HEADER_META } from '../settingsHeaderMeta';

const extractedModules = [
  '../settingsNavigationModel.ts',
  '../settingsRouting.ts',
  '../settingsNavCatalog.ts',
  '../settingsNavVisibility.ts',
  '../settingsTabSaveBehavior.ts',
  '../settingsTypes.ts',
  '../DockerRuntimeSettingsCard.tsx',
  '../settingsHeaderMeta.ts',
  '../settingsFeatureGates.ts',
  '../BackupTransferDialogs.tsx',
  '../InfrastructureOperationsController.tsx',
  '../infrastructureOperationsModel.tsx',
  '../useInfrastructureInstallState.tsx',
  '../useInfrastructureOperationsState.tsx',
  '../useInfrastructureReportingState.tsx',
  '../infrastructureSettingsModel.ts',
  '../useInfrastructureConfiguredNodesState.ts',
  '../useInfrastructureDiscoveryRuntimeState.ts',
  '../useSettingsInfrastructurePanelProps.ts',
  '../settingsPanelRegistryContext.tsx',
  '../settingsPanelRegistryLoaders.ts',
  '../apiTokenManagerModel.ts',
  '../useAPITokenManagerState.ts',
  '../useAuditLogPanelState.ts',
  '../useAuditWebhookPanelState.ts',
  '../BillingAdminOrganizationsTable.tsx',
  '../useBillingAdminPanelState.ts',
  '../NodeModalAuthenticationSection.tsx',
  '../NodeModalBasicInfoSection.tsx',
  '../NodeModalMonitoringSection.tsx',
  '../NodeModalSetupGuideSection.tsx',
  '../NodeModalStatusFooter.tsx',
  '../NodeModal.tsx',
  '../nodeModalModel.ts',
  '../useNodeModalState.ts',
  '../InfrastructureWorkspace.tsx',
  '../infrastructureWorkspaceModel.ts',
  '../InfrastructureInstallPanel.tsx',
  '../InfrastructureInstallerSection.tsx',
  '../InfrastructureReportingPanel.tsx',
  '../InfrastructureDirectConnectionsSummaryCard.tsx',
  '../InfrastructureInventorySection.tsx',
  '../InfrastructureActiveRowDetails.tsx',
  '../InfrastructureIgnoredRowDetails.tsx',
  '../InfrastructureStopMonitoringDialog.tsx',
  '../AIProviderConfigurationSection.tsx',
  '../AISettingsDialogs.tsx',
  '../AIChatMaintenanceSection.tsx',
  '../AIModelSelectionSection.tsx',
  '../AIRuntimeControlsSection.tsx',
  '../AISettingsStatusAndActions.tsx',
  '../aiSettingsModel.ts',
  '../useAISettingsState.ts',
  '../diagnosticsModel.ts',
  '../DiagnosticsPanel.tsx',
  '../DiagnosticsResultsPanel.tsx',
  '../OperationsPanel.tsx',
  '../NetworkBoundarySettingsSection.tsx',
  '../NetworkDiscoverySection.tsx',
  '../networkSettingsModel.ts',
  '../CopyCommandBlock.tsx',
  '../UpdateInstallGuide.tsx',
  '../ReportingPanel.tsx',
  '../reportingPanelModel.ts',
  '../reportingInventoryExportModel.ts',
  '../ProLicensePlanSection.tsx',
  '../RelayPairingSection.tsx',
  '../OrganizationOverviewLoadingState.tsx',
  '../OrganizationOverviewDetailsSection.tsx',
  '../OrganizationOverviewMembersSection.tsx',
  '../OrganizationAccessLoadingState.tsx',
  '../OrganizationAccessManagementSection.tsx',
  '../OrganizationAccessMembersSection.tsx',
  '../OrganizationSharingCreateSection.tsx',
  '../OrganizationSharingLoadingState.tsx',
  '../OrganizationOutgoingSharesSection.tsx',
  '../OrganizationIncomingSharesSection.tsx',
  '../OrganizationBillingLoadingState.tsx',
  '../useOrganizationAccessPanelState.ts',
  '../useOrganizationBillingPanelState.ts',
  '../useOrganizationOverviewPanelState.ts',
  '../useOrganizationSharingPanelState.ts',
  '../RBACFeatureGateSection.tsx',
  '../RolesEditorDialog.tsx',
  '../useRBACFeatureGateState.ts',
  '../useRolesPanelState.ts',
  '../UserAssignmentsDialog.tsx',
  '../useUserAssignmentsPanelState.ts',
  '../updatesSettingsModel.ts',
  '../useProLicensePanelState.ts',
  '../useRelaySettingsPanelState.ts',
  '../useDiagnosticsPanelState.ts',
  '../useReportingPanelState.ts',
  '../useSystemLogsPanelState.ts',
  '../useSSOProvidersState.ts',
  '../ssoProvidersModel.ts',
  '../ProxmoxSettingsPanel.tsx',
  '../ProxmoxDirectWorkspace.tsx',
  '../ProxmoxConfiguredNodesTable.tsx',
  '../ProxmoxDeleteNodeDialog.tsx',
  '../ProxmoxDirectConnectionsCard.tsx',
  '../ProxmoxDiscoveryResultsCard.tsx',
  '../ProxmoxNodeModalStack.tsx',
  '../proxmoxSettingsModel.ts',
  '../useProxmoxDirectWorkspaceState.ts',
  '../SettingsDialogs.tsx',
  '../SettingsPageShell.tsx',
  '../useDiscoverySettingsState.ts',
  '../useSettingsAccess.ts',
  '../useSettingsShellState.ts',
  '../useSettingsNavigation.ts',
  '../useSettingsPanelRegistry.tsx',
  '../useSettingsSystemPanels.tsx',
  '../useSystemSettingsState.ts',
  '../useInfrastructureSettingsState.ts',
  '../useBackupTransferFlow.ts',
  '../settingsPanelRegistry.ts',
] as const;

const requiredImportSources = [
  './settingsTabSaveBehavior',
  './SettingsDialogs',
  './SettingsPageShell',
  './useBackupTransferFlow',
  './useDiscoverySettingsState',
  './useInfrastructureSettingsState',
  './useSettingsInfrastructurePanelProps',
  './useSettingsAccess',
  './useSettingsPanelRegistry',
  './useSettingsShellState',
  './useSettingsSystemPanels',
  './useSystemSettingsState',
  './useSettingsNavigation',
] as const;

const topLevelSettingsPanelSources = {
  APIAccessPanel: apiAccessPanelSource,
  AuditLogPanel: auditLogPanelSource,
  AuditWebhookPanel: auditWebhookPanelSource,
  BillingAdminPanel: billingAdminPanelSource,
  GeneralSettingsPanel: generalSettingsPanelSource,
  AISettings: aiSettingsPanelSource,
  InfrastructureWorkspace: infrastructureWorkspaceSource,
  NetworkSettingsPanel: networkSettingsPanelSource,
  UpdatesSettingsPanel: updatesSettingsPanelSource,
  RecoverySettingsPanel: recoverySettingsPanelSource,
  RelaySettingsPanel: relaySettingsPanelSource,
  ProLicensePanel: proLicensePanelSource,
  OrganizationOverviewPanel: organizationOverviewPanelSource,
  OrganizationAccessPanel: organizationAccessPanelSource,
  OrganizationSharingPanel: organizationSharingPanelSource,
  OrganizationBillingPanel: organizationBillingPanelSource,
  SecurityOverviewPanel: securityOverviewPanelSource,
  SecurityAuthPanel: securityAuthPanelSource,
  SSOProvidersPanel: ssoProvidersPanelSource,
  RolesPanel: rolesPanelSource,
  UserAssignmentsPanel: userAssignmentsPanelSource,
} as const;

const canonicalShellTitleExpectations = [
  {
    tab: 'api',
    title: 'API Access',
    source: apiAccessPanelSource,
  },
  {
    tab: 'system-general',
    title: 'General',
    source: generalSettingsPanelSource,
  },
  {
    tab: 'system-network',
    title: 'Network',
    source: networkSettingsPanelSource,
  },
  {
    tab: 'system-updates',
    title: 'Updates',
    source: updatesSettingsPanelSource,
  },
  {
    tab: 'system-recovery',
    title: 'Recovery',
    source: recoverySettingsPanelSource,
  },
  {
    tab: 'system-ai',
    title: 'AI Services',
    source: aiSettingsPanelSource,
  },
  {
    tab: 'system-billing',
    title: 'Pulse Pro',
    source: proLicensePanelSource,
  },
  {
    tab: 'organization-billing',
    title: 'Billing & Usage',
    source: organizationBillingPanelSource,
  },
  {
    tab: 'security-overview',
    title: 'Security Overview',
    source: securityOverviewPanelSource,
  },
  {
    tab: 'security-auth',
    title: 'Authentication',
    source: securityAuthPanelSource,
  },
  {
    tab: 'security-sso',
    title: 'Single Sign-On Providers',
    source: ssoProvidersPanelSource,
  },
  {
    tab: 'security-audit',
    title: 'Audit Log',
    source: auditLogPanelSource,
  },
  {
    tab: 'security-webhooks',
    title: 'Audit Webhooks',
    source: auditWebhookPanelSource,
  },
] as const;

describe('Settings architecture guardrails', () => {
  it('keeps extracted settings modules present on disk', () => {
    const settingsModuleFiles = {
      ...import.meta.glob('../*.ts'),
      ...import.meta.glob('../*.tsx'),
    };

    for (const modulePath of extractedModules) {
      expect(
        Object.prototype.hasOwnProperty.call(settingsModuleFiles, modulePath),
        `${modulePath} should exist and remain externalized`,
      ).toBe(true);
    }
  });

  it('imports extracted architecture modules from Settings.tsx', () => {
    const importSources = Array.from(
      settingsSource.matchAll(/import[\s\S]*?from\s+['"]([^'"]+)['"];?/g),
      (match) => match[1],
    );

    for (const source of requiredImportSources) {
      expect(importSources, `Settings.tsx should import ${source}`).toContain(source);
    }
  });

  it('routes dispatchable settings tabs through the extracted panel registry', async () => {
    const registrySource = (await import('../settingsPanelRegistry.ts?raw')).default;
    const accessHookSource = (await import('../useSettingsAccess.ts?raw')).default;
    const panelRegistryHookSource = (await import('../useSettingsPanelRegistry.tsx?raw')).default;
    const shellHookSource = (await import('../useSettingsShellState.ts?raw')).default;

    expect(registrySource).toContain('createSettingsPanelRegistry');
    expect(registrySource).toContain('SETTINGS_PANEL_REGISTRY_LOADERS');
    expect(registrySource).toContain("'security-webhooks'");
    expect(accessHookSource).toContain('shouldHideSettingsNavItem');
    expect(accessHookSource).toContain('SETTINGS_NAV_GROUPS');
    expect(accessHookSource).toContain('./settingsNavCatalog');
    expect(accessHookSource).toContain('./settingsNavVisibility');
    expect(accessHookSource).toContain('tabFeatureRequirements');
    expect(panelRegistryHookSource).toContain('createSettingsPanelRegistry');
    expect(panelRegistryHookSource).toContain('buildSettingsPanelRegistryContext');
    expect(settingsPanelRegistryLoadersSource).toContain(
      'export const SETTINGS_PANEL_REGISTRY_LOADERS',
    );
    expect(settingsPanelRegistryLoadersSource).toContain("import('./InfrastructureWorkspace')");
    expect(shellHookSource).toContain('SETTINGS_HEADER_META');
    expect(settingsSource).toContain('useSettingsPanelRegistry');
    expect(settingsSource).toContain('useSettingsAccess');
    expect(settingsSource).toContain('useSettingsShellState');
    expect(settingsSource).toContain('getSettingsTabSaveBehavior');
    expect(settingsSource).toContain('activeSettingsPanelEntry');
    expect(settingsSource).toContain('<Dynamic component={entry().component}');
    expect(settingsSource).not.toContain('<ProxmoxSettingsPanel');
    expect(settingsSource).not.toContain("<Show when={activeTab() === 'system-general'}>");
    expect(settingsSource).not.toContain("<Show when={activeTab() === 'security-webhooks'}>");
  });

  it('keeps settings shell panel assembly split between dedicated system and registry owners', () => {
    expect(settingsSource).toContain('useSettingsSystemPanels');
    expect(settingsSource).toContain('useSettingsInfrastructurePanelProps');
    expect(settingsSource).toContain('./settingsTabSaveBehavior');
    expect(settingsSource).toContain('const systemPanels = useSettingsSystemPanels({');
    expect(settingsSource).toContain(
      'const infrastructurePanelProps = useSettingsInfrastructurePanelProps({',
    );
    expect(settingsSource).toContain('const settingsPanelRegistry = useSettingsPanelRegistry({');
    expect(settingsSource).toContain('systemPanels,');
    expect(settingsSource).toContain('const discoverySettings = useDiscoverySettingsState()');
    expect(settingsSource).not.toContain('getInfrastructurePanelProps: () => ({');
    expect(settingsSystemPanelsSource).toContain('GeneralSettingsPanel');
    expect(settingsSystemPanelsSource).toContain('getSettingsConfigurationLoadingState');
    expect(settingsSystemPanelsSource).toContain('getNetworkPanelProps');
    expect(settingsSystemPanelsSource).toContain('getUpdatesPanelProps');
    expect(settingsSystemPanelsSource).toContain('getRecoveryPanelProps');
    expect(settingsSystemPanelsSource).toContain('pvePollingInterval:');
    expect(settingsSystemPanelsSource).toContain('allowedOrigins:');
    expect(settingsSystemPanelsSource).toContain('backupPollingEnabled:');
    expect(settingsSystemPanelsSource).toContain('handleDiscoveryEnabledChange:');
    expect(settingsPanelRegistryHookSource).toContain('buildSettingsPanelRegistryContext');
    expect(settingsPanelRegistryContextSource).toContain('systemPanels: SettingsSystemPanels');
    expect(settingsPanelRegistryContextSource).toContain(
      'systemGeneralPanel: params.systemPanels.systemGeneralPanel',
    );
    expect(settingsPanelRegistryContextSource).toContain(
      'getNetworkPanelProps: params.systemPanels.getNetworkPanelProps',
    );
    expect(settingsPanelRegistryContextSource).toContain(
      'getUpdatesPanelProps: params.systemPanels.getUpdatesPanelProps',
    );
    expect(settingsPanelRegistryContextSource).toContain(
      'getRecoveryPanelProps: params.systemPanels.getRecoveryPanelProps',
    );
    expect(settingsPanelRegistryContextSource).toContain('const systemAiPanel: Component');
    expect(settingsPanelRegistryContextSource).toContain('const securitySsoPanel: Component');
    expect(settingsPanelRegistryHookSource).not.toContain('pvePollingInterval: params.');
    expect(settingsPanelRegistryHookSource).not.toContain('allowedOrigins: params.');
    expect(settingsPanelRegistryHookSource).not.toContain('backupPollingEnabled: params.');
    expect(settingsInfrastructurePanelPropsSource).toContain('pbsInstanceFromResource');
    expect(settingsInfrastructurePanelPropsSource).toContain('pmgInstanceFromResource');
    expect(settingsInfrastructurePanelPropsSource).toContain(
      'const agentStateResources = createMemo',
    );
    expect(settingsInfrastructurePanelPropsSource).toContain('getInfrastructurePanelProps');
    expect(settingsNavigationModelSource).toContain('export type SettingsTab =');
    expect(settingsNavigationModelSource).toContain('export const DEFAULT_SETTINGS_TAB');
    expect(settingsNavigationModelSource).toContain('export function resolveCanonicalSettingsPath');
    expect(settingsNavigationModelSource).toContain('export function settingsTabPath');
    expect(settingsNavigationHookSource).toContain('deriveTabFromPath');
    expect(settingsNavigationHookSource).toContain('resolveCanonicalSettingsPath');
    expect(settingsNavigationHookSource).toContain('settingsTabPath');
    expect(settingsRoutingSource).toContain("from './settingsNavigationModel'");
    expect(settingsTypesSource).toContain("from './settingsNavigationModel'");
    expect(settingsDialogsSource).toContain('UpdateConfirmationModal');
    expect(settingsDialogsSource).toContain('BackupTransferDialogs');
    expect(settingsDialogsSource).toContain('ChangePasswordModal');
    expect(settingsShellStateSource).toContain('SETTINGS_HEADER_META');
    expect(settingsShellStateSource).toContain('sidebarCollapsed');
    expect(settingsShellStateSource).toContain('showPasswordModal');
    expect(settingsNavCatalogSource).toContain('export const SETTINGS_NAV_GROUPS');
    expect(settingsNavCatalogSource).toContain('export function getSettingsNavItem');
    expect(settingsNavVisibilitySource).toContain('export function shouldHideSettingsNavItem');
    expect(settingsNavVisibilitySource).toContain('export function isSettingsNavItemLocked');
    expect(settingsTabSaveBehaviorSource).toContain('getSettingsNavItem(tab)?.saveBehavior');
  });

  it('keeps commercial plan and usage sections on a shared billing owner', () => {
    expect(commercialBillingSectionsSource).toContain('export const CommercialSection');
    expect(commercialBillingSectionsSource).toContain('export const CommercialStatGrid');
    expect(commercialBillingSectionsSource).toContain('export const CommercialUsageMeters');
    expect(commercialBillingSectionsSource).toContain('export const CommercialBillingShell');
    expect(commercialBillingModelSource).toContain('buildSelfHostedCommercialPlanModel');
    expect(commercialBillingModelSource).toContain('buildHostedCommercialPlanModel');
    expect(commercialBillingModelSource).toContain('buildHostedCommercialUsageModel');
    expect(proLicensePanelSource).toContain('./CommercialBillingSections');
    expect(proLicensePanelSource).toContain('./useProLicensePanelState');
    expect(proLicensePanelSource).toContain('./ProLicensePlanSection');
    expect(proLicensePanelSource).toContain('SelfHostedCommercialActivationSection');
    expect(proLicensePanelSource).toContain('MonitoredSystemLedgerPanel');
    expect(proLicensePanelSource).toContain('CommercialBillingShell');
    expect(proLicensePanelSource).toContain('CommercialSection');
    expect(proLicensePanelSource).not.toContain('createSignal(');
    expect(proLicensePanelSource).not.toContain('useLocation()');
    expect(monitoredSystemLedgerPanelSource).toContain('@/utils/monitoredSystemPresentation');
    expect(monitoredSystemLedgerPanelSource).toContain('getMonitoredSystemLedgerPresentation');
    expect(monitoredSystemLedgerPanelSource).toContain(
      'getMonitoredSystemCountingDetailsToggleLabel',
    );
    expect(monitoredSystemLedgerPanelSource).not.toContain('No monitored systems counted.');
    expect(monitoredSystemLedgerPanelSource).not.toContain('Current status');
    expect(monitoredSystemLedgerPanelSource).toContain('getMonitoredSystemLedgerDescription');
    expect(proLicensePanelSource).toContain('@/utils/monitoredSystemPresentation');
    expect(proLicensePanelSource).toContain('getMonitoredSystemBriefSummary');
    expect(monitoredSystemDefinitionDisclosureSource).toContain(
      '@/utils/monitoredSystemPresentation',
    );
    expect(monitoredSystemDefinitionDisclosureSource).toContain('getMonitoredSystemBriefSummary');
    expect(monitoredSystemDefinitionDisclosureSource).toContain(
      'getMonitoredSystemDisclosureToggleLabel',
    );
    expect(monitoredSystemDefinitionDisclosureSource).not.toContain('summary?: string');
    expect(monitoredSystemDefinitionDisclosureSource).not.toContain('{props.summary}');
    expect(proLicensePanelStateSource).toContain('buildSelfHostedCommercialPlanModel');
    expect(proLicensePanelStateSource).toContain('loadLicenseStatus(true)');
    expect(proLicensePanelStateSource).toContain('runStartProTrialAction({');
    expect(proLicensePanelStateSource).not.toContain('startProTrial()');
    expect(proLicensePlanSectionSource).toContain('CommercialStatGrid');
    expect(proLicensePlanSectionSource).toContain('getLicenseStatusLoadingState');
    expect(monitoredSystemPresentationSource).toContain(
      'export function getMonitoredSystemLedgerPresentation',
    );
    expect(monitoredSystemPresentationSource).toContain(
      'export function getMonitoredSystemCountingDetailsToggleLabel',
    );
    expect(monitoredSystemPresentationSource).toContain(
      'export function getMonitoredSystemBriefSummary',
    );
    expect(monitoredSystemPresentationSource).toContain(
      'export function getMonitoredSystemDisclosureToggleLabel',
    );
    expect(selfHostedCommercialActivationSectionSource).toContain('License / Activation Key');
    expect(selfHostedCommercialActivationSectionSource).toContain('Start 14-day Pro Trial');
    expect(organizationBillingPanelSource).toContain('./CommercialBillingSections');
    expect(organizationBillingPanelSource).toContain('./OrganizationBillingLoadingState');
    expect(organizationBillingPanelSource).toContain('./useOrganizationBillingPanelState');
    expect(organizationBillingPanelSource).toContain('CommercialBillingShell');
    expect(organizationBillingPanelSource).toContain('CommercialSection');
    expect(organizationBillingPanelSource).toContain('CommercialUsageMeters');
    expect(organizationBillingPanelSource).not.toContain('createSignal(');
    expect(organizationBillingPanelSource).not.toContain('onMount(() =>');
    expect(organizationBillingStateSource).toContain('buildHostedCommercialPlanModel');
    expect(organizationBillingStateSource).toContain('buildHostedCommercialUsageModel');
    expect(organizationBillingStateSource).toContain("eventBus.on('org_switched'");
    expect(organizationBillingLoadingStateSource).toContain('animate-pulse');
    expect(settingsSource).toContain('organizationMonitoredSystemUsage');
    expect(settingsSource).toContain("getLimit('max_monitored_systems')?.current ?? 0");
    expect(settingsSource).not.toContain('organizationAgentUsage');
  });

  it('keeps relay settings split into shell, runtime, and pairing owners', () => {
    expect(relaySettingsPanelSource).toContain('./useRelaySettingsPanelState');
    expect(relaySettingsPanelSource).toContain('./RelayPairingSection');
    expect(relaySettingsPanelSource).not.toContain('createSignal(');
    expect(relaySettingsPanelSource).not.toContain('QRCode.toDataURL(');
    expect(relaySettingsPanelStateSource).toContain(
      "trackPaywallViewed('relay', 'settings_relay_panel')",
    );
    expect(relaySettingsPanelStateSource).toContain('setInterval(() => void loadStatus(), 5000)');
    expect(relaySettingsPanelStateSource).toContain('QRCode.toDataURL(payload.deep_link');
    expect(relaySettingsPanelStateSource).toContain('runStartProTrialAction({');
    expect(relaySettingsPanelStateSource).not.toContain('startProTrial()');
    expect(relayPairingSectionSource).toContain('getRelayDiagnosticClass');
    expect(relayPairingSectionSource).toContain('Pair New Device');
  });

  it('keeps hosted billing admin split into shell, runtime, and table owners', () => {
    expect(billingAdminPanelSource).toContain('./useBillingAdminPanelState');
    expect(billingAdminPanelSource).toContain('./BillingAdminOrganizationsTable');
    expect(billingAdminPanelSource).not.toContain('createSignal(');
    expect(billingAdminPanelSource).not.toContain('BillingAdminAPI.listOrganizations');
    expect(billingAdminPanelStateSource).toContain('BillingAdminAPI.listOrganizations');
    expect(billingAdminPanelStateSource).toContain('BillingAdminAPI.putBillingState');
    expect(billingAdminPanelStateSource).toContain('promisePool');
    expect(billingAdminOrganizationsTableSource).toContain('PulseDataGrid');
    expect(billingAdminOrganizationsTableSource).toContain('Billing state JSON');
  });

  it('keeps system logs split into shell and runtime owners', () => {
    expect(systemLogsPanelSource).toContain('@/components/Settings/OperationsPanel');
    expect(systemLogsPanelSource).toContain('./useSystemLogsPanelState');
    expect(systemLogsPanelSource).not.toContain('createSignal(');
    expect(systemLogsPanelSource).not.toContain('new EventSource(');
    expect(systemLogsPanelSource).not.toContain("apiFetchJSON('/api/logs/level'");
    expect(systemLogsPanelStateSource).toContain('new EventSource');
    expect(systemLogsPanelStateSource).toContain("apiFetchJSON('/api/logs/level'");
    expect(systemLogsPanelStateSource).toContain('notificationStore.success');
  });

  it('keeps network settings split into shell, section, and model owners', () => {
    expect(networkSettingsPanelSource).toContain('./NetworkDiscoverySection');
    expect(networkSettingsPanelSource).toContain('./NetworkBoundarySettingsSection');
    expect(networkSettingsPanelSource).toContain('./networkSettingsModel');
    expect(networkSettingsPanelSource).not.toContain('COMMON_DISCOVERY_SUBNETS');
    expect(networkSettingsPanelSource).not.toContain('Dashboard URL for Notifications');
    expect(networkSettingsPanelSource).not.toContain('Allowed Private IP Ranges for Webhooks');
    expect(networkDiscoverySectionSource).toContain('COMMON_DISCOVERY_SUBNETS');
    expect(networkDiscoverySectionSource).toContain('@/utils/discoveryPresentation');
    expect(networkDiscoverySectionSource).toContain('getNetworkDiscoveryPriorityNotice');
    expect(networkDiscoverySectionSource).toContain('getNetworkDiscoverySectionPresentation');
    expect(networkDiscoverySectionSource).toContain('getNetworkDiscoveryModePresentation');
    expect(networkDiscoverySectionSource).toContain('getNetworkDiscoverySubnetPresentation');
    expect(networkDiscoverySectionSource).not.toContain('Configuration priority');
    expect(networkDiscoverySectionSource).not.toContain('Automatic scanning');
    expect(networkDiscoverySectionSource).not.toContain(
      'Discovery settings are locked by environment variables.',
    );
    expect(networkBoundarySettingsSectionSource).toContain('Dashboard URL for Notifications');
    expect(networkBoundarySettingsSectionSource).toContain(
      'Allowed Private IP Ranges for Webhooks',
    );
    expect(networkBoundarySettingsSectionSource).toContain('EnvironmentOverrideAlert');
    expect(networkSettingsModelSource).toContain('export interface NetworkSettingsPanelProps');
    expect(networkSettingsModelSource).toContain('export type NetworkDiscoverySectionProps');
    expect(networkSettingsModelSource).toContain('export type NetworkBoundarySettingsSectionProps');
  });

  it('does not re-inline extracted tab and header metadata definitions', () => {
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+baseTabGroups\s*=/);
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+SETTINGS_HEADER_META\s*=/);
    expect(settingsSource).not.toMatch(/\b(?:const|let|var)\s+tabFeatureRequirements\s*=/);
  });

  it('keeps Settings.tsx below the monolith guardrail ceiling', () => {
    // If this test fails, the change should be decomposed into the appropriate
    // extracted module rather than increasing the limit. Exceptions require
    // explicit discussion.
    const maxSettingsLines = 4500;
    const settingsLineCount = settingsSource.trimEnd().split(/\r?\n/).length;

    expect(settingsLineCount).toBeLessThanOrEqual(maxSettingsLines);
  });

  it('keeps the direct Proxmox settings workspace split into section owners', () => {
    expect(proxmoxSettingsPanelSource).toContain('./ProxmoxDirectWorkspace');
    expect(proxmoxSettingsPanelSource).toContain('./proxmoxSettingsModel');
    expect(proxmoxSettingsPanelSource).toContain('CalloutCard');
    expect(proxmoxSettingsPanelSource).toContain('SettingsSectionNav');
    expect(proxmoxSettingsPanelSource).not.toContain('./useProxmoxSettingsPanelState');
    expect(proxmoxSettingsPanelSource).not.toContain('./useProxmoxDirectWorkspaceState');
    expect(proxmoxDirectWorkspaceSource).toContain('./useProxmoxDirectWorkspaceState');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxConfiguredNodesTable');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxDirectConnectionsCard');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxDiscoveryResultsCard');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxDeleteNodeDialog');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxNodeModalStack');
    expect(proxmoxSettingsPanelSource).not.toContain('const renderConfiguredTable = () =>');
    expect(proxmoxSettingsPanelSource).not.toContain('const renderNodeModal = (type: NodeType)');
    expect(proxmoxSettingsPanelSource).not.toContain(
      'No discovery matches for this Proxmox type yet.',
    );
    expect(proxmoxSettingsPanelSource).not.toContain('What happens next');
    expect(proxmoxConfiguredNodesTableSource).toContain('PveNodesTable');
    expect(proxmoxConfiguredNodesTableSource).toContain('PbsNodesTable');
    expect(proxmoxConfiguredNodesTableSource).toContain('PmgNodesTable');
    expect(proxmoxDirectConnectionsCardSource).toContain('getSettingsConfigurationLoadingState');
    expect(proxmoxDiscoveryResultsCardSource).toContain('Discovery issues:');
    expect(proxmoxDiscoveryResultsCardSource).toContain(
      'No discovery matches for this Proxmox type yet. You can still add a direct',
    );
    expect(proxmoxDeleteNodeDialogSource).toContain('What happens next');
    expect(proxmoxNodeModalStackSource).toContain('PROXMOX_NODE_TYPES');
    expect(proxmoxNodeModalStackSource).toContain('<NodeModal');
    expect(proxmoxSettingsModelSource).toContain('export interface ProxmoxSettingsPanelProps');
    expect(proxmoxSettingsModelSource).toContain('./infrastructureSettingsModel');
    expect(proxmoxDirectWorkspaceStateSource).toContain(
      'export function useProxmoxDirectWorkspaceState',
    );
    expect(proxmoxDirectWorkspaceStateSource).toContain(
      "notificationStore.info('Refreshing discovery...'",
    );
  });

  it('keeps organization sharing split into shell, state, and section owners', () => {
    expect(organizationSharingPanelSource).toContain('./useOrganizationSharingPanelState');
    expect(organizationSharingPanelSource).toContain('./OrganizationSharingLoadingState');
    expect(organizationSharingPanelSource).toContain('./OrganizationSharingCreateSection');
    expect(organizationSharingPanelSource).toContain('./OrganizationOutgoingSharesSection');
    expect(organizationSharingPanelSource).toContain('./OrganizationIncomingSharesSection');
    expect(organizationSharingPanelSource).not.toContain('const loadSharingData = async');
    expect(organizationSharingPanelSource).not.toContain('const createShare = async');
    expect(organizationSharingStateSource).toContain('OrgsAPI.createShare');
    expect(organizationSharingStateSource).toContain('OrgsAPI.deleteShare');
    expect(organizationSharingStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationSharingCreateSectionSource).toContain('CANONICAL_RESOURCE_TYPES');
    expect(organizationSharingCreateSectionSource).toContain('ORGANIZATION_SHARE_ROLE_OPTIONS');
    expect(organizationOutgoingSharesSectionSource).toContain(
      'getOrganizationOutgoingSharesEmptyState',
    );
    expect(organizationIncomingSharesSectionSource).toContain(
      'getOrganizationIncomingSharesEmptyState',
    );
    expect(organizationSharingLoadingStateSource).toContain('animate-pulse');
  });

  it('keeps organization access split into shell, state, and section owners', () => {
    expect(organizationAccessPanelSource).toContain('./useOrganizationAccessPanelState');
    expect(organizationAccessPanelSource).toContain('./OrganizationAccessLoadingState');
    expect(organizationAccessPanelSource).toContain('./OrganizationAccessManagementSection');
    expect(organizationAccessPanelSource).toContain('./OrganizationAccessMembersSection');
    expect(organizationAccessPanelSource).not.toContain('const loadOrganizationAccess = async');
    expect(organizationAccessPanelSource).not.toContain('const inviteMember = async');
    expect(organizationAccessStateSource).toContain('OrgsAPI.updateMemberRole');
    expect(organizationAccessStateSource).toContain('OrgsAPI.inviteMember');
    expect(organizationAccessStateSource).toContain('OrgsAPI.removeMember');
    expect(organizationAccessStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationAccessManagementSectionSource).toContain(
      'getOrganizationAccessManageRequiredMessage',
    );
    expect(organizationAccessManagementSectionSource).toContain('ORGANIZATION_MEMBER_ROLE_OPTIONS');
    expect(organizationAccessMembersSectionSource).toContain('getOrganizationAccessEmptyState');
    expect(organizationAccessMembersSectionSource).toContain('formatOrgDate');
    expect(organizationAccessLoadingStateSource).toContain('animate-pulse');
  });

  it('keeps organization overview split into shell, state, and section owners', () => {
    expect(organizationOverviewPanelSource).toContain('./useOrganizationOverviewPanelState');
    expect(organizationOverviewPanelSource).toContain('./OrganizationOverviewLoadingState');
    expect(organizationOverviewPanelSource).toContain('./OrganizationOverviewDetailsSection');
    expect(organizationOverviewPanelSource).toContain('./OrganizationOverviewMembersSection');
    expect(organizationOverviewPanelSource).not.toContain('const loadOrganization = async');
    expect(organizationOverviewPanelSource).not.toContain('const saveDisplayName = async');
    expect(organizationOverviewStateSource).toContain('OrgsAPI.get');
    expect(organizationOverviewStateSource).toContain('OrgsAPI.listMembers');
    expect(organizationOverviewStateSource).toContain('OrgsAPI.update');
    expect(organizationOverviewStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationOverviewDetailsSectionSource).toContain(
      'getOrganizationOverviewManageRequiredMessage',
    );
    expect(organizationOverviewDetailsSectionSource).toContain('formatOrgDate');
    expect(organizationOverviewMembersSectionSource).toContain(
      'getOrganizationOverviewMembersEmptyState',
    );
    expect(organizationOverviewLoadingStateSource).toContain('animate-pulse');
  });

  it('keeps RBAC settings panels split into gate, state, and dialog owners', () => {
    expect(rolesPanelSource).toContain('./RBACFeatureGateSection');
    expect(rolesPanelSource).toContain('./RolesEditorDialog');
    expect(rolesPanelSource).toContain('./useRolesPanelState');
    expect(rolesPanelSource).not.toContain('const handleStartTrial = async');
    expect(rolesPanelSource).not.toContain('const loadRoles = async');
    expect(userAssignmentsPanelSource).toContain('./RBACFeatureGateSection');
    expect(userAssignmentsPanelSource).toContain('./UserAssignmentsDialog');
    expect(userAssignmentsPanelSource).toContain('./useUserAssignmentsPanelState');
    expect(userAssignmentsPanelSource).not.toContain('const handleStartTrial = async');
    expect(userAssignmentsPanelSource).not.toContain('const loadData = async');
    expect(rbacFeatureGateSectionSource).toContain('trackUpgradeClicked');
    expect(rbacFeatureGateStateSource).toContain('trackPaywallViewed');
    expect(rbacFeatureGateStateSource).toContain('runStartProTrialAction({');
    expect(rbacFeatureGateStateSource).not.toContain('startProTrial()');
    expect(rolesEditorDialogSource).toContain('RBAC_PERMISSION_ACTIONS');
    expect(rolesPanelStateSource).toContain('RBACAPI.getRoles');
    expect(rolesPanelStateSource).toContain('RBACAPI.saveRole');
    expect(userAssignmentsDialogSource).toContain('Effective Permissions Preview');
    expect(userAssignmentsPanelStateSource).toContain('RBACAPI.getUsers');
    expect(userAssignmentsPanelStateSource).toContain('RBACAPI.updateUserRoles');
  });

  it('uses lazy() imports for panel components in settingsPanelRegistry', async () => {
    const source = (await import('../settingsPanelRegistry.ts?raw')).default;
    const loadersSource = (await import('../settingsPanelRegistryLoaders.ts?raw')).default;

    expect(source).toContain('SETTINGS_PANEL_REGISTRY_LOADERS');
    expect(loadersSource).toContain('lazy(');

    const staticImports = Array.from(
      loadersSource.matchAll(
        /^import\s+(?!type\b)(?!{[^}]*}\s+from\s+'solid-js').*from\s+'\.\/\w+Panel'/gm,
      ),
    );
    expect(staticImports.length).toBe(0);
  });

  it('keeps page-level settings header chrome inside SettingsPageShell', () => {
    expect(settingsShellSource).toContain('<PageHeader');
    expect(settingsShellSource).toContain('getSettingsSearchEmptyState');
    expect(settingsShellSource).toContain('getSettingsUnsavedChangesBanner');
    expect(settingsShellSource).toContain('SETTINGS_SHELL_COPY');
    expect(settingsShellSource).not.toContain('No settings found for "');
    expect(settingsShellSource).not.toContain('Unsaved changes');
    expect(settingsShellSource).not.toContain('Search settings...');
    expect(settingsShellSource).not.toContain('Collapse sidebar');
    expect(settingsShellSource).not.toContain('Expand sidebar');
    expect(infrastructureWorkspaceSource).not.toContain('<PageHeader');
    expect(infrastructureWorkspaceSource).not.toMatch(/<h[12][^>]*>/);
    expect(infrastructureWorkspaceSource).not.toContain('Add and manage infrastructure');
    expect(infrastructureWorkspaceSource).not.toContain('tracking-[0.22em]');
    expect(infrastructureWorkspaceSource).toContain('./infrastructureWorkspaceModel');
    expect(infrastructureWorkspaceSource).not.toContain(
      'createSignal<InfrastructureWorkspaceView>',
    );
    expect(infrastructureWorkspaceSource).not.toContain('createEffect(() =>');
    expect(infrastructureWorkspaceSource).toContain('InfrastructureInstallPanel');
    expect(infrastructureWorkspaceSource).toContain('InfrastructureReportingPanel');
    expect(infrastructureInstallPanelSource).toContain('InfrastructureOperationsStateProvider');
    expect(infrastructureInstallPanelSource).toContain('InfrastructureInstallerSection');
    expect(infrastructureReportingPanelSource).toContain('InfrastructureOperationsStateProvider');
    expect(infrastructureReportingPanelSource).toContain('InfrastructureInventorySection');
    expect(infrastructureReportingPanelSource).toContain('InfrastructureStopMonitoringDialog');
    expect(infrastructureReportingPanelSource).toContain(
      './InfrastructureDirectConnectionsSummaryCard',
    );
    expect(infrastructureReportingPanelSource).not.toContain('Direct Proxmox connections');
    expect(infrastructureReportingPanelSource).not.toContain('Manage direct connections');
    expect(infrastructureOperationsControllerSource).toContain(
      'InfrastructureOperationsStateProvider',
    );
    expect(infrastructureOperationsControllerSource).toContain('InfrastructureInstallerSection');
    expect(infrastructureOperationsControllerSource).toContain('InfrastructureInventorySection');
    expect(infrastructureOperationsControllerSource).toContain(
      'InfrastructureStopMonitoringDialog',
    );
    expect(infrastructureOperationsStateSource).toContain(
      'export const useInfrastructureOperationsState',
    );
    expect(infrastructureOperationsStateSource).toContain('./useInfrastructureInstallState');
    expect(infrastructureOperationsStateSource).toContain('./useInfrastructureReportingState');
    expect(infrastructureOperationsStateSource).toContain(
      'export const InfrastructureOperationsStateProvider',
    );
    expect(infrastructureOperationsStateSource).toContain(
      'export const useInfrastructureOperationsContext',
    );
    expect(infrastructureOperationsStateSource).not.toContain('const renderInstallerSection =');
    expect(infrastructureOperationsStateSource).not.toContain('const renderInventorySection =');
    expect(infrastructureOperationsStateSource).not.toContain('const renderStopMonitoringDialog =');
    expect(infrastructureInstallStateSource).toContain(
      'export const useInfrastructureInstallState',
    );
    expect(infrastructureReportingStateSource).toContain(
      'export const useInfrastructureReportingState',
    );
    expect(infrastructureInstallerSectionSource).toContain('useInfrastructureOperationsContext');
    expect(infrastructureInventorySectionSource).toContain('useInfrastructureOperationsContext');
    expect(infrastructureStopMonitoringDialogSource).toContain(
      'useInfrastructureOperationsContext',
    );
    expect(infrastructureActiveRowDetailsSource).toContain('useInfrastructureOperationsContext');
    expect(infrastructureIgnoredRowDetailsSource).toContain('useInfrastructureOperationsContext');
    expect(infrastructureWorkspaceModelSource).toContain(
      'export const INFRASTRUCTURE_WORKSPACE_TABS',
    );
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function getInfrastructureWorkspaceViewFromPath',
    );
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function buildInfrastructureWorkspacePath',
    );
    expect(infrastructureDirectConnectionsSummaryCardSource).toContain(
      'Direct Proxmox connections',
    );
    expect(infrastructureDirectConnectionsSummaryCardSource).toContain('Manage direct connections');
    expect(infrastructureInstallPanelSource).not.toContain('<PageHeader');
    expect(infrastructureReportingPanelSource).not.toContain('<PageHeader');
  });

  it('keeps the infrastructure operations model extracted from the reactive hook owner', () => {
    expect(infrastructureOperationsStateSource).toContain('./infrastructureOperationsModel');
    expect(infrastructureOperationsStateSource).not.toContain('const INSTALL_PROFILE_OPTIONS');
    expect(infrastructureOperationsStateSource).not.toContain('const buildCommandsByPlatform =');
    expect(infrastructureOperationsStateSource).not.toContain(
      'const rowFromConnectedInfrastructureItem =',
    );
    expect(infrastructureOperationsModelSource).toContain('export const INSTALL_PROFILE_OPTIONS');
    expect(infrastructureOperationsModelSource).toContain(
      'export const rowFromConnectedInfrastructureItem',
    );
    expect(infrastructureOperationsModelSource).toContain('export const buildCommandsByPlatform');
  });

  it('keeps infrastructure settings split into composition, node, discovery, and model owners', () => {
    expect(infrastructureSettingsStateSource).toContain('./useInfrastructureConfiguredNodesState');
    expect(infrastructureSettingsStateSource).toContain('./useInfrastructureDiscoveryRuntimeState');
    expect(infrastructureSettingsStateSource).toContain("from './infrastructureSettingsModel'");
    expect(infrastructureSettingsStateSource).not.toContain('NodesAPI.getNodes');
    expect(infrastructureSettingsStateSource).not.toContain('SettingsAPI.updateSystemSettings');
    expect(infrastructureSettingsStateSource).not.toContain(
      'const [nodes, setNodes] = createSignal',
    );
    expect(infrastructureSettingsStateSource).not.toContain(
      'const [discoveredNodes, setDiscoveredNodes] = createSignal',
    );
    expect(infrastructureConfiguredNodesStateSource).toContain(
      'export const useInfrastructureConfiguredNodesState',
    );
    expect(infrastructureConfiguredNodesStateSource).toContain('NodesAPI.getNodes');
    expect(infrastructureConfiguredNodesStateSource).toContain('NodesAPI.updateNode');
    expect(infrastructureConfiguredNodesStateSource).toContain('NodesAPI.deleteNode');
    expect(infrastructureConfiguredNodesStateSource).toContain('NodesAPI.refreshClusterNodes');
    expect(infrastructureConfiguredNodesStateSource).not.toContain("apiFetch('/api/discover'");
    expect(infrastructureConfiguredNodesStateSource).not.toContain(
      'SettingsAPI.updateSystemSettings',
    );
    expect(infrastructureDiscoveryRuntimeStateSource).toContain(
      'export const useInfrastructureDiscoveryRuntimeState',
    );
    expect(infrastructureDiscoveryRuntimeStateSource).toContain("apiFetch('/api/discover'");
    expect(infrastructureDiscoveryRuntimeStateSource).toContain('SettingsAPI.updateSystemSettings');
    expect(infrastructureDiscoveryRuntimeStateSource).toContain("eventBus.on('discovery_updated'");
    expect(infrastructureDiscoveryRuntimeStateSource).not.toContain('NodesAPI.getNodes');
    expect(infrastructureSettingsModelSource).toContain('export interface DiscoveredServer');
    expect(infrastructureSettingsModelSource).toContain('export type NodeType =');
    expect(infrastructureSettingsModelSource).toContain(
      'export const matchConfiguredNodeToResource =',
    );
  });

  it('keeps the API token manager shell behind extracted state and model owners', () => {
    expect(apiTokenManagerSource).toContain('./useAPITokenManagerState');
    expect(apiTokenManagerSource).not.toContain(
      'const [tokens, setTokens] = createSignal<APITokenRecord[]>([])',
    );
    expect(apiTokenManagerSource).not.toContain('const loadTokens = async () =>');
    expect(apiTokenManagerSource).not.toContain(
      'const hasAgentScopeResource = (resource: Resource)',
    );
    expect(apiTokenManagerStateSource).toContain('export const useAPITokenManagerState =');
    expect(apiTokenManagerStateSource).toContain(
      'const [tokens, setTokens] = createSignal<APITokenRecord[]>([])',
    );
    expect(apiTokenManagerStateSource).toContain('const loadTokens = async () =>');
    expect(apiTokenManagerStateSource).toContain('SecurityAPI.listTokens()');
    expect(apiTokenManagerStateSource).toContain('const scopePresets = getAPITokenScopePresets();');
    expect(apiTokenManagerStateSource).not.toContain(
      'const scopePresets = API_TOKEN_SCOPE_PRESETS;',
    );
    expect(apiTokenManagerModelSource).toContain('export const hasAgentScopeResource =');
    expect(apiTokenManagerModelSource).toContain('export const buildDockerTokenUsage =');
    expect(apiTokenManagerModelSource).toContain('export const buildAgentTokenUsage =');
    expect(apiTokenManagerModelSource).toContain(
      'export const getAPITokenScopePresets = (): APITokenPreset[] => [',
    );
    expect(apiTokenManagerModelSource).not.toContain('export const API_TOKEN_SCOPE_PRESETS = [');
  });

  it('keeps the node setup modal shell behind extracted state and model owners', () => {
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalBasicInfoSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalAuthenticationSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalMonitoringSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalStatusFooter');
    expect(nodeModalSource).toContain('@/components/Settings/nodeModalModel');
    expect(nodeModalSource).toContain('@/components/Settings/useNodeModalState');
    expect(nodeModalSource).not.toContain('title="Basic information"');
    expect(nodeModalSource).not.toContain('title="Authentication"');
    expect(nodeModalSource).not.toContain('title="Monitoring coverage"');
    expect(nodeModalSource).not.toContain('const deriveNameFromHost =');
    expect(nodeModalSource).not.toContain('const PVE_MANUAL_PERMISSION_COMMAND = `');
    expect(nodeModalSource).not.toContain('const [quickSetupBootstrap, setQuickSetupBootstrap] =');
    expect(nodeModalSource).not.toContain('const handleTestConnection = async () =>');
    expect(nodeModalBasicInfoSectionSource).toContain('title="Basic information"');
    expect(nodeModalAuthenticationSectionSource).toContain(
      '@/components/Settings/NodeModalSetupGuideSection',
    );
    expect(nodeModalSetupGuideSectionSource).toContain('Connection Setup');
    expect(nodeModalMonitoringSectionSource).toContain('title="Monitoring coverage"');
    expect(nodeModalStatusFooterSource).toContain('Start your free 14-day trial');
    expect(nodeModalModelSource).toContain('export interface NodeModalProps');
    expect(nodeModalModelSource).toContain('export const deriveNameFromHost =');
    expect(nodeModalModelSource).toContain('export const PVE_MANUAL_PERMISSION_COMMAND = `');
    expect(nodeModalStateSource).toContain('export const useNodeModalState =');
    expect(nodeModalStateSource).toContain(
      'export type NodeModalState = ReturnType<typeof useNodeModalState>;',
    );
    expect(nodeModalStateSource).toContain('const [quickSetupBootstrap, setQuickSetupBootstrap] =');
    expect(nodeModalStateSource).toContain('const handleTestConnection = async () =>');
    expect(nodeModalStateSource).toContain(
      "const PROXMOX_SETUP_HOST_REQUIRED_MESSAGE = 'Proxmox setup host is required';",
    );
    expect(nodeModalStateSource).toContain('runStartProTrialAction({');
    expect(nodeModalStateSource).not.toContain('startProTrial()');
  });

  it('keeps AI settings sub-surfaces behind extracted runtime owners', () => {
    expect(aiSettingsPanelSource).toContain('@/components/Settings/AIModelSelectionSection');
    expect(aiSettingsPanelSource).toContain('@/components/Settings/AIRuntimeControlsSection');
    expect(aiSettingsPanelSource).toContain('@/components/Settings/AIChatMaintenanceSection');
    expect(aiSettingsPanelSource).toContain('@/components/Settings/AISettingsStatusAndActions');
    expect(aiSettingsPanelSource).toContain('@/components/Settings/AISettingsDialogs');
    expect(aiSettingsPanelSource).toContain('@/components/Settings/useAISettingsState');
    expect(aiSettingsPanelSource).not.toContain(
      'const [loading, setLoading] = createSignal(false);',
    );
    expect(aiSettingsPanelSource).not.toContain('const handleSave = async (event?: Event) =>');
    expect(aiSettingsPanelSource).not.toContain('AIAPI.getSettings()');
    expect(aiSettingsPanelSource).not.toContain('Chat Session Maintenance');
    expect(aiSettingsPanelSource).not.toContain('Discovery Settings');
    expect(aiSettingsPanelSource).not.toContain('Pulse Permission Level');
    expect(aiModelSelectionSectionSource).toContain(
      '@/components/Settings/AIProviderConfigurationSection',
    );
    expect(aiModelSelectionSectionSource).toContain('@/components/Settings/aiSettingsModel');
    expect(aiModelSelectionSectionSource).toContain('Advanced Model Selection');
    expect(aiRuntimeControlsSectionSource).toContain('Discovery Settings');
    expect(aiRuntimeControlsSectionSource).toContain('Pulse Permission Level');
    expect(aiChatMaintenanceSectionSource).toContain('Chat Session Maintenance');
    expect(aiSettingsStatusAndActionsSource).toContain('Save changes');
    expect(aiSettingsStatusAndActionsSource).toContain('Test Connection');
    expect(aiProviderConfigurationSectionSource).toContain('@/components/Settings/aiSettingsModel');
    expect(aiSettingsDialogsSource).toContain('@/components/Settings/aiSettingsModel');
    expect(aiSettingsModelSource).toContain('export const AI_PROVIDER_CONFIGS');
    expect(aiSettingsModelSource).toContain('export const AI_SETUP_PROVIDER_OPTIONS');
    expect(aiSettingsStateSource).toContain('export const useAISettingsState =');
    expect(aiSettingsStateSource).toContain('const [loading, setLoading] = createSignal(false);');
    expect(aiSettingsStateSource).toContain('const handleSave = async (event?: Event) =>');
    expect(aiSettingsStateSource).toContain(
      'const handleEnabledToggle = async (newValue: boolean) =>',
    );
    expect(aiSettingsStateSource).toContain('AIAPI.getSettings()');
    expect(aiSettingsStateSource).toContain('runStartProTrialAction({');
    expect(aiSettingsStateSource).not.toContain('startProTrial()');
    expect(aiSettingsStateSource).not.toContain('getTrialAlreadyUsedMessage()');
  });

  it('keeps the updates settings shell behind extracted install-guide owners', () => {
    expect(updatesSettingsPanelSource).toContain('@/components/Settings/UpdateInstallGuide');
    expect(updatesSettingsPanelSource).toContain('@/components/Settings/updatesSettingsModel');
    expect(updatesSettingsPanelSource).toContain('UPDATES_PANEL_COPY');
    expect(updatesSettingsPanelSource).toContain('getUpdateCheckActionLabel');
    expect(updatesSettingsPanelSource).not.toContain("navigator.clipboard.writeText('update')");
    expect(updatesSettingsPanelSource).not.toContain("value: 'stable'");
    expect(updatesSettingsPanelSource).not.toContain("value: 'rc'");
    expect(updatesSettingsPanelSource).not.toContain('Check Now');
    expect(updatesSettingsPanelSource).not.toContain('Checking...');
    expect(updatesSettingsPanelSource).not.toContain('Update Preferences');
    expect(updateInstallGuideSource).toContain('@/components/Settings/CopyCommandBlock');
    expect(updateInstallGuideSource).toContain('buildUpdateInstallGuide');
    expect(copyCommandBlockSource).toContain('export function CopyCommandBlock');
    expect(copyCommandBlockSource).toContain('aria-label="Copy to clipboard"');
    expect(updatesSettingsModelSource).toContain('export function getUpdateChannelCardOptions');
    expect(updatesSettingsModelSource).toContain('export function buildUpdateInstallGuide');
    expect(updatesSettingsModelSource).toContain("title: 'Pre-release'");
    expect(updatesSettingsModelSource).not.toContain('Release Candidate');
    expect(updatesPresentationSource).toContain(
      'Pre-release builds stay on a manual preview channel.',
    );
    expect(updatesPresentationSource).not.toContain('RC is a manual preview channel.');
  });

  it('keeps the diagnostics shell behind extracted runtime and results owners', () => {
    expect(diagnosticsPanelSource).toContain('@/components/Settings/OperationsPanel');
    expect(diagnosticsPanelSource).toContain('@/components/Settings/DiagnosticsResultsPanel');
    expect(diagnosticsPanelSource).toContain('@/components/Settings/useDiagnosticsPanelState');
    expect(diagnosticsPanelSource).toContain('DIAGNOSTICS_PANEL_COPY');
    expect(diagnosticsPanelSource).toContain('formatUptime');
    expect(diagnosticsPanelSource).not.toContain("apiFetchJSON('/api/diagnostics')");
    expect(diagnosticsPanelSource).not.toContain('URL.createObjectURL');
    expect(diagnosticsPanelSource).not.toContain('sanitizeDiagnosticsData');
    expect(diagnosticsPanelSource).not.toContain('System Diagnostics');
    expect(diagnosticsResultsPanelSource).toContain('DIAGNOSTICS_EMPTY_PBS_MESSAGE');
    expect(diagnosticsResultsPanelSource).toContain('DIAGNOSTICS_EMPTY_STATE_COPY');
    expect(diagnosticsResultsPanelSource).toContain('getStatusIndicatorBadgeToneClasses(');
    expect(diagnosticsStateSource).toContain('export const useDiagnosticsPanelState =');
    expect(diagnosticsStateSource).toContain("apiFetchJSON('/api/diagnostics')");
    expect(diagnosticsStateSource).toContain('URL.createObjectURL');
    expect(diagnosticsModelSource).toContain('export function sanitizeDiagnosticsData');
    expect(diagnosticsModelSource).toContain('export function buildDiagnosticsExportFilename');
    expect(diagnosticsModelSource).toContain('export function formatUptime');
  });

  it('keeps the reporting shell behind extracted runtime and model owners', () => {
    expect(reportingPanelSource).toContain('@/components/Settings/OperationsPanel');
    expect(reportingPanelSource).toContain('@/components/Settings/useReportingPanelState');
    expect(reportingPanelSource).toContain('@/components/Settings/reportingCatalogModel');
    expect(reportingPanelSource).toContain('@/components/Settings/reportingPanelModel');
    expect(reportingPanelSource).toContain('reportingCatalog');
    expect(reportingPanelSource).toContain('title="Reporting"');
    expect(reportingPanelSource).not.toContain('loadLicenseStatus()');
    expect(reportingPanelSource).not.toContain('startProTrial()');
    expect(reportingPanelSource).not.toContain("apiFetch('/api/admin/reports/generate");
    expect(reportingPanelSource).not.toContain('window.URL.createObjectURL');
    expect(reportingPanelStateSource).toContain('export const useReportingPanelState =');
    expect(reportingPanelStateSource).toContain('loadLicenseStatus');
    expect(reportingPanelStateSource).toContain('runStartProTrialAction({');
    expect(reportingPanelStateSource).not.toContain('startProTrial()');
    expect(reportingPanelStateSource).toContain('buildReportingRequest');
    expect(reportingPanelStateSource).toContain('buildReportingCatalogRequest');
    expect(reportingPanelStateSource).toContain('parseReportingCatalog');
    expect(reportingPanelStateSource).toContain('reportingFeatureId');
    expect(reportingPanelStateSource).not.toContain('!licenseLoaded()');
    expect(reportingPanelStateSource).not.toContain('!isReportingEnabled()');
    expect(reportingPanelStateSource).toContain('buildVMInventoryExportRequest');
    expect(reportingPanelStateSource).toContain('getReportingGenerateSuccessMessage');
    expect(reportingPanelStateSource).toContain('resolveReportingDownloadFilename');
    expect(reportingPanelStateSource).not.toContain("'advanced_reporting'");
    expect(reportingPanelStateSource).not.toContain('getTrialAlreadyUsedMessage()');
    expect(reportingCatalogModelSource).toContain('export function buildReportingCatalogRequest');
    expect(reportingCatalogModelSource).toContain('export function parseReportingCatalog');
    expect(reportingCatalogModelSource).toContain('interface ReportingLockedStateDefinition');
    expect(reportingCatalogModelSource).toContain('interface ReportingGuidanceDefinition');
    expect(reportingPanelSource).not.toContain('Advanced Reporting (Pro)');
    expect(reportingPanelSource).not.toContain('Advanced Insights');
    expect(reportingPanelSource).not.toContain('Detailed Reporting');
    expect(reportingPanelSource).not.toContain(
      'Performance reports come from the historical metrics store',
    );
    expect(reportingPanelModelSource).toContain('export function getReportingRangeStart');
    expect(reportingPanelModelSource).toContain('export function buildReportingRequest');
    expect(reportingPanelModelSource).toContain('export function buildReportingFilename');
    expect(reportingInventoryExportModelSource).toContain(
      'export function buildVMInventoryExportFilename',
    );
    expect(reportingInventoryExportModelSource).toContain(
      'export function parseVMInventoryExportDefinition',
    );
  });

  it('keeps the shared operations wrapper rooted at SettingsPanel', () => {
    expect(operationsPanelSource).toContain('@/components/shared/SettingsPanel');
    expect(operationsPanelSource).toContain('noPadding');
    expect(operationsPanelSource).toContain('bodyClass="divide-y divide-border"');
    expect(operationsPanelSource).not.toContain('createSignal(');
    expect(operationsPanelSource).not.toContain('apiFetch');
  });

  it('keeps the audit log shell behind an extracted runtime owner', () => {
    expect(auditLogPanelSource).toContain('@/components/Settings/useAuditLogPanelState');
    expect(auditLogPanelSource).not.toContain('createLocalStorageStringSignal');
    expect(auditLogPanelSource).not.toContain('const fetchAuditEvents = async (');
    expect(auditLogPanelSource).not.toContain('const verifyAllEvents = async (');
    expect(auditLogPanelSource).not.toContain('trackPaywallViewed');
    expect(auditLogStateSource).toContain('export const useAuditLogPanelState =');
    expect(auditLogStateSource).toContain('createLocalStorageStringSignal');
    expect(auditLogStateSource).toContain('const fetchAuditEvents = async (');
    expect(auditLogStateSource).toContain('const verifyAllEvents = async (');
    expect(auditLogStateSource).toContain('trackPaywallViewed');
    expect(auditLogStateSource).toContain('runStartProTrialAction({');
    expect(auditLogStateSource).not.toContain('startProTrial()');
    expect(auditLogStateSource).not.toContain('getTrialAlreadyUsedMessage()');
  });

  it('keeps the audit webhook shell behind an extracted runtime owner', () => {
    expect(auditWebhookPanelSource).toContain('@/components/Settings/useAuditWebhookPanelState');
    expect(auditWebhookPanelSource).not.toContain('loadLicenseStatus();');
    expect(auditWebhookPanelSource).not.toContain('const fetchWebhooks = async () =>');
    expect(auditWebhookPanelSource).not.toContain('const saveWebhooks = async (urls: string[]) =>');
    expect(auditWebhookPanelSource).not.toContain('trackPaywallViewed');
    expect(auditWebhookStateSource).toContain('export const useAuditWebhookPanelState =');
    expect(auditWebhookStateSource).toContain('loadLicenseStatus();');
    expect(auditWebhookStateSource).toContain('const fetchWebhooks = async () =>');
    expect(auditWebhookStateSource).toContain('const saveWebhooks = async (urls: string[]) =>');
    expect(auditWebhookStateSource).toContain('trackPaywallViewed');
    expect(auditWebhookStateSource).toContain('runStartProTrialAction({');
    expect(auditWebhookStateSource).not.toContain('startProTrial()');
    expect(auditWebhookStateSource).not.toContain('getTrialAlreadyUsedMessage()');
  });

  it('keeps the SSO providers shell behind extracted runtime owners', () => {
    expect(ssoProvidersPanelSource).toContain('@/components/Settings/useSSOProvidersState');
    expect(ssoProvidersPanelSource).toContain('@/utils/ssoProviderPresentation');
    expect(ssoProvidersPanelSource).not.toContain('const loadProviders = async () =>');
    expect(ssoProvidersPanelSource).not.toContain('const handleSave = async (');
    expect(ssoProvidersPanelSource).not.toContain('const testConnection = async () =>');
    expect(ssoProvidersStateSource).toContain('@/components/Settings/ssoProvidersModel');
    expect(ssoProvidersStateSource).toContain('@/utils/ssoProviderPresentation');
    expect(ssoProvidersStateSource).toContain('export const useSSOProvidersState =');
    expect(ssoProvidersStateSource).toContain('const loadProviders = async () =>');
    expect(ssoProvidersStateSource).toContain('const handleSave = async (event?: Event) =>');
    expect(ssoProvidersStateSource).toContain('const testConnection = async () =>');
    expect(ssoProvidersStateSource).toContain('runStartProTrialAction({');
    expect(ssoProvidersStateSource).not.toContain('startProTrial()');
    expect(ssoProvidersStateSource).not.toContain('getTrialAlreadyUsedMessage()');
    expect(ssoProvidersModelSource).toContain('export const createEmptyProviderForm =');
    expect(ssoProvidersModelSource).toContain('export const mapProviderDetailsToForm =');
    expect(ssoProvidersModelSource).toContain('export const buildProviderPayload =');
    expect(ssoProvidersModelSource).toContain('export const buildProviderTestPayload =');
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderEmptyStateTitle',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProvidersLoadErrorMessage',
    );
  });

  it('keeps shared system settings presentation extracted from panel and state owners', () => {
    expect(generalSettingsPanelSource).toContain('@/utils/systemSettingsPresentation');
    expect(recoverySettingsPanelSource).toContain('@/utils/systemSettingsPresentation');
    expect(networkDiscoverySectionSource).toContain('@/utils/systemSettingsPresentation');
    expect(systemSettingsStateSource).toContain('@/utils/systemSettingsPresentation');
    expect(systemSettingsPresentationSource).toContain('export const PVE_POLLING_PRESETS');
    expect(systemSettingsPresentationSource).toContain(
      'export function getSystemSettingsSaveErrorMessage',
    );
    expect(systemSettingsPresentationSource).toContain(
      'export function getStartUpdateErrorMessage',
    );
  });

  it('routes every top-level settings surface through the canonical panel shell framing', () => {
    for (const [panelName, source] of Object.entries(topLevelSettingsPanelSources)) {
      const usesCanonicalShell =
        source.includes('SettingsPanel') || source.includes('CommercialBillingShell');
      expect(usesCanonicalShell, `${panelName} should use a canonical settings panel shell`).toBe(
        true,
      );
      expect(source, `${panelName} should not introduce page-level header chrome`).not.toContain(
        '<PageHeader',
      );
    }
  });

  it('keeps infrastructure shell framing focused on operations, not billing', () => {
    expect(SETTINGS_HEADER_META['infrastructure-operations'].title).toBe(
      'Infrastructure Operations',
    );
    expect(SETTINGS_HEADER_META['infrastructure-operations'].description).toContain(
      'actively reporting',
    );
    expect(SETTINGS_HEADER_META['infrastructure-operations'].description).toContain('Pulse Pro');
  });

  it('keeps billing-related shell framing on monitored-system commercial terms', () => {
    expect(SETTINGS_HEADER_META['infrastructure-operations'].description).toContain(
      'monitored-system limits',
    );
    expect(SETTINGS_HEADER_META['infrastructure-operations'].description).not.toContain(
      'installed-agent',
    );
    expect(SETTINGS_HEADER_META['system-billing'].description).toContain('license status');
    expect(SETTINGS_HEADER_META['system-billing'].description).not.toContain('allocation');
    expect(SETTINGS_HEADER_META['organization-billing'].description).toContain(
      'subscription status',
    );
    expect(SETTINGS_HEADER_META['organization-billing'].description).toContain('plan limits');
  });

  it('keeps shell titles aligned with the leading settings panel on key top-level surfaces', () => {
    for (const { tab, title, source } of canonicalShellTitleExpectations) {
      expect(SETTINGS_HEADER_META[tab].title, `${tab} should keep its shell title canonical`).toBe(
        title,
      );
      const allowedTitleExpression =
        tab === 'system-updates' ? `title={UPDATES_PANEL_COPY.title}` : `title="${title}"`;
      expect(
        source,
        `${tab} should keep its leading SettingsPanel title aligned with the shell title`,
      ).toContain(allowedTitleExpression);
    }
  });

  it('keeps single-surface settings pages rooted directly at SettingsPanel', () => {
    expect(
      auditLogPanelSource,
      'AuditLogPanel should not wrap its single shell in an extra page spacer',
    ).not.toContain('<div class="space-y-6">');
    expect(
      updatesSettingsPanelSource,
      'UpdatesSettingsPanel should not wrap its single shell in an extra page spacer',
    ).not.toContain('<div class="space-y-6">');
    expect(
      recoverySettingsPanelSource,
      'RecoverySettingsPanel should not wrap its single shell in an extra page spacer',
    ).not.toContain('<div class="space-y-6">');
  });
});
