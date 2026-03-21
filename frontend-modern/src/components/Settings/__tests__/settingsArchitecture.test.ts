import { describe, expect, it } from 'vitest';
import settingsSource from '../Settings.tsx?raw';
import settingsShellSource from '../SettingsPageShell.tsx?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import infrastructureWorkspaceModelSource from '../infrastructureWorkspaceModel.ts?raw';
import infrastructureInstallPanelSource from '../InfrastructureInstallPanel.tsx?raw';
import infrastructureOperationsControllerSource from '../InfrastructureOperationsController.tsx?raw';
import infrastructureOperationsModelSource from '../infrastructureOperationsModel.tsx?raw';
import infrastructureReportingPanelSource from '../InfrastructureReportingPanel.tsx?raw';
import infrastructureDirectConnectionsSummaryCardSource from '../InfrastructureDirectConnectionsSummaryCard.tsx?raw';
import nodeModalModelSource from '../nodeModalModel.ts?raw';
import nodeModalSource from '../NodeModal.tsx?raw';
import infrastructureOperationsStateSource from '../useInfrastructureOperationsState.tsx?raw';
import nodeModalStateSource from '../useNodeModalState.ts?raw';
import apiAccessPanelSource from '../APIAccessPanel.tsx?raw';
import apiTokenManagerSource from '../APITokenManager.tsx?raw';
import apiTokenManagerModelSource from '../apiTokenManagerModel.ts?raw';
import apiTokenManagerStateSource from '../useAPITokenManagerState.ts?raw';
import auditLogPanelSource from '../AuditLogPanel.tsx?raw';
import auditLogStateSource from '../useAuditLogPanelState.ts?raw';
import auditWebhookPanelSource from '../AuditWebhookPanel.tsx?raw';
import auditWebhookStateSource from '../useAuditWebhookPanelState.ts?raw';
import billingAdminPanelSource from '../BillingAdminPanel.tsx?raw';
import generalSettingsPanelSource from '../GeneralSettingsPanel.tsx?raw';
import aiSettingsPanelSource from '../AISettings.tsx?raw';
import aiProviderConfigurationSectionSource from '../AIProviderConfigurationSection.tsx?raw';
import aiSettingsDialogsSource from '../AISettingsDialogs.tsx?raw';
import aiSettingsModelSource from '../aiSettingsModel.ts?raw';
import aiSettingsStateSource from '../useAISettingsState.ts?raw';
import diagnosticsModelSource from '../diagnosticsModel.ts?raw';
import diagnosticsPanelSource from '../DiagnosticsPanel.tsx?raw';
import diagnosticsResultsPanelSource from '../DiagnosticsResultsPanel.tsx?raw';
import networkSettingsPanelSource from '../NetworkSettingsPanel.tsx?raw';
import copyCommandBlockSource from '../CopyCommandBlock.tsx?raw';
import updateInstallGuideSource from '../UpdateInstallGuide.tsx?raw';
import updatesSettingsModelSource from '../updatesSettingsModel.ts?raw';
import updatesSettingsPanelSource from '../UpdatesSettingsPanel.tsx?raw';
import recoverySettingsPanelSource from '../RecoverySettingsPanel.tsx?raw';
import relaySettingsPanelSource from '../RelaySettingsPanel.tsx?raw';
import proLicensePanelSource from '../ProLicensePanel.tsx?raw';
import commercialBillingSectionsSource from '../CommercialBillingSections.tsx?raw';
import selfHostedCommercialActivationSectionSource from '../SelfHostedCommercialActivationSection.tsx?raw';
import commercialBillingModelSource from '@/utils/commercialBillingModel.ts?raw';
import organizationOverviewPanelSource from '../OrganizationOverviewPanel.tsx?raw';
import organizationAccessPanelSource from '../OrganizationAccessPanel.tsx?raw';
import organizationSharingPanelSource from '../OrganizationSharingPanel.tsx?raw';
import organizationBillingPanelSource from '../OrganizationBillingPanel.tsx?raw';
import proxmoxDeleteNodeDialogSource from '../ProxmoxDeleteNodeDialog.tsx?raw';
import proxmoxDirectConnectionsCardSource from '../ProxmoxDirectConnectionsCard.tsx?raw';
import proxmoxDiscoveryResultsCardSource from '../ProxmoxDiscoveryResultsCard.tsx?raw';
import proxmoxSettingsPanelSource from '../ProxmoxSettingsPanel.tsx?raw';
import securityOverviewPanelSource from '../SecurityOverviewPanel.tsx?raw';
import securityAuthPanelSource from '../SecurityAuthPanel.tsx?raw';
import ssoProvidersPanelSource from '../SSOProvidersPanel.tsx?raw';
import ssoProvidersStateSource from '../useSSOProvidersState.ts?raw';
import ssoProvidersModelSource from '../ssoProvidersModel.ts?raw';
import diagnosticsStateSource from '../useDiagnosticsPanelState.ts?raw';
import reportingPanelModelSource from '../reportingPanelModel.ts?raw';
import reportingPanelSource from '../ReportingPanel.tsx?raw';
import reportingPanelStateSource from '../useReportingPanelState.ts?raw';
import rolesPanelSource from '../RolesPanel.tsx?raw';
import userAssignmentsPanelSource from '../UserAssignmentsPanel.tsx?raw';
import { SETTINGS_HEADER_META } from '../settingsHeaderMeta';

const extractedModules = [
  '../settingsTypes.ts',
  '../settingsTabs.ts',
  '../DockerRuntimeSettingsCard.tsx',
  '../settingsHeaderMeta.ts',
  '../settingsFeatureGates.ts',
  '../BackupTransferDialogs.tsx',
  '../InfrastructureOperationsController.tsx',
  '../infrastructureOperationsModel.tsx',
  '../useInfrastructureOperationsState.tsx',
  '../apiTokenManagerModel.ts',
  '../useAPITokenManagerState.ts',
  '../useAuditLogPanelState.ts',
  '../useAuditWebhookPanelState.ts',
  '../NodeModal.tsx',
  '../nodeModalModel.ts',
  '../useNodeModalState.ts',
  '../InfrastructureWorkspace.tsx',
  '../infrastructureWorkspaceModel.ts',
  '../InfrastructureInstallPanel.tsx',
  '../InfrastructureReportingPanel.tsx',
  '../InfrastructureDirectConnectionsSummaryCard.tsx',
  '../AIProviderConfigurationSection.tsx',
  '../AISettingsDialogs.tsx',
  '../aiSettingsModel.ts',
  '../useAISettingsState.ts',
  '../diagnosticsModel.ts',
  '../DiagnosticsPanel.tsx',
  '../DiagnosticsResultsPanel.tsx',
  '../CopyCommandBlock.tsx',
  '../UpdateInstallGuide.tsx',
  '../ReportingPanel.tsx',
  '../reportingPanelModel.ts',
  '../updatesSettingsModel.ts',
  '../useDiagnosticsPanelState.ts',
  '../useReportingPanelState.ts',
  '../useSSOProvidersState.ts',
  '../ssoProvidersModel.ts',
  '../ProxmoxSettingsPanel.tsx',
  '../ProxmoxDeleteNodeDialog.tsx',
  '../ProxmoxDirectConnectionsCard.tsx',
  '../ProxmoxDiscoveryResultsCard.tsx',
  '../SettingsDialogs.tsx',
  '../SettingsPageShell.tsx',
  '../useDiscoverySettingsState.ts',
  '../useSettingsAccess.ts',
  '../useSettingsShellState.ts',
  '../useSettingsNavigation.ts',
  '../useSettingsPanelRegistry.tsx',
  '../useSystemSettingsState.ts',
  '../useInfrastructureSettingsState.ts',
  '../useBackupTransferFlow.ts',
  '../settingsPanelRegistry.ts',
] as const;

const requiredImportSources = [
  './settingsTabs',
  './SettingsDialogs',
  './SettingsPageShell',
  './useBackupTransferFlow',
  './useDiscoverySettingsState',
  './useInfrastructureSettingsState',
  './useSettingsAccess',
  './useSettingsPanelRegistry',
  './useSettingsShellState',
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
    expect(registrySource).toContain('InfrastructureWorkspace');
    expect(registrySource).toContain("'security-webhooks'");
    expect(accessHookSource).toContain('shouldHideSettingsNavItem');
    expect(accessHookSource).toContain('tabFeatureRequirements');
    expect(panelRegistryHookSource).toContain('createSettingsPanelRegistry');
    expect(shellHookSource).toContain('SETTINGS_HEADER_META');
    expect(settingsSource).toContain('useSettingsPanelRegistry');
    expect(settingsSource).toContain('useSettingsAccess');
    expect(settingsSource).toContain('useSettingsShellState');
    expect(settingsSource).toContain('activeSettingsPanelEntry');
    expect(settingsSource).toContain('<Dynamic component={entry().component}');
    expect(settingsSource).not.toContain('<ProxmoxSettingsPanel');
    expect(settingsSource).not.toContain("<Show when={activeTab() === 'system-general'}>");
    expect(settingsSource).not.toContain("<Show when={activeTab() === 'security-webhooks'}>");
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
    expect(proLicensePanelSource).toContain('buildSelfHostedCommercialPlanModel');
    expect(proLicensePanelSource).toContain('SelfHostedCommercialActivationSection');
    expect(proLicensePanelSource).toContain('MonitoredSystemLedgerPanel');
    expect(proLicensePanelSource).toContain('CommercialBillingShell');
    expect(proLicensePanelSource).toContain('CommercialSection');
    expect(proLicensePanelSource).toContain('CommercialStatGrid');
    expect(selfHostedCommercialActivationSectionSource).toContain('License / Activation Key');
    expect(selfHostedCommercialActivationSectionSource).toContain('Start 14-day Pro Trial');
    expect(organizationBillingPanelSource).toContain('./CommercialBillingSections');
    expect(organizationBillingPanelSource).toContain('buildHostedCommercialPlanModel');
    expect(organizationBillingPanelSource).toContain('buildHostedCommercialUsageModel');
    expect(organizationBillingPanelSource).toContain('CommercialBillingShell');
    expect(organizationBillingPanelSource).toContain('CommercialSection');
    expect(organizationBillingPanelSource).toContain('CommercialUsageMeters');
    expect(settingsSource).toContain('organizationMonitoredSystemUsage');
    expect(settingsSource).toContain("getLimit('max_monitored_systems')?.current ?? 0");
    expect(settingsSource).not.toContain('organizationAgentUsage');
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
    expect(proxmoxSettingsPanelSource).toContain('./ProxmoxDirectConnectionsCard');
    expect(proxmoxSettingsPanelSource).toContain('./ProxmoxDiscoveryResultsCard');
    expect(proxmoxSettingsPanelSource).toContain('./ProxmoxDeleteNodeDialog');
    expect(proxmoxSettingsPanelSource).not.toContain('No discovery matches for this Proxmox type yet.');
    expect(proxmoxSettingsPanelSource).not.toContain('What happens next');
    expect(proxmoxDirectConnectionsCardSource).toContain('getSettingsConfigurationLoadingState');
    expect(proxmoxDiscoveryResultsCardSource).toContain('Discovery issues:');
    expect(proxmoxDiscoveryResultsCardSource).toContain(
      'No discovery matches for this Proxmox type yet. You can still add a direct',
    );
    expect(proxmoxDeleteNodeDialogSource).toContain('What happens next');
  });

  it('uses lazy() imports for panel components in settingsPanelRegistry', async () => {
    const source = (await import('../settingsPanelRegistry.ts?raw')).default;

    expect(source).toContain('lazy(');

    const staticImports = Array.from(
      source.matchAll(
        /^import\s+(?!type\b)(?!{[^}]*}\s+from\s+'solid-js').*from\s+'\.\/\w+Panel'/gm,
      ),
    );
    expect(staticImports.length).toBe(0);
  });

  it('keeps page-level settings header chrome inside SettingsPageShell', () => {
    expect(settingsShellSource).toContain('<PageHeader');
    expect(settingsShellSource).toContain('getSettingsSearchEmptyState');
    expect(settingsShellSource).not.toContain('No settings found for "');
    expect(infrastructureWorkspaceSource).not.toContain('<PageHeader');
    expect(infrastructureWorkspaceSource).not.toMatch(/<h[12][^>]*>/);
    expect(infrastructureWorkspaceSource).not.toContain('Add and manage infrastructure');
    expect(infrastructureWorkspaceSource).not.toContain('tracking-[0.22em]');
    expect(infrastructureWorkspaceSource).toContain('./infrastructureWorkspaceModel');
    expect(infrastructureWorkspaceSource).not.toContain('createSignal<InfrastructureWorkspaceView>');
    expect(infrastructureWorkspaceSource).not.toContain('createEffect(() =>');
    expect(infrastructureWorkspaceSource).toContain('InfrastructureInstallPanel');
    expect(infrastructureWorkspaceSource).toContain('InfrastructureReportingPanel');
    expect(infrastructureInstallPanelSource).toContain('useInfrastructureOperationsState');
    expect(infrastructureReportingPanelSource).toContain('useInfrastructureOperationsState');
    expect(infrastructureReportingPanelSource).toContain(
      './InfrastructureDirectConnectionsSummaryCard',
    );
    expect(infrastructureReportingPanelSource).not.toContain('Direct Proxmox connections');
    expect(infrastructureReportingPanelSource).not.toContain('Manage direct connections');
    expect(infrastructureOperationsControllerSource).toContain('useInfrastructureOperationsState');
    expect(infrastructureOperationsStateSource).toContain(
      'export const useInfrastructureOperationsState',
    );
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
    expect(infrastructureDirectConnectionsSummaryCardSource).toContain(
      'Manage direct connections',
    );
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

  it('keeps the API token manager shell behind extracted state and model owners', () => {
    expect(apiTokenManagerSource).toContain('./useAPITokenManagerState');
    expect(apiTokenManagerSource).not.toContain('const [tokens, setTokens] = createSignal<APITokenRecord[]>([])');
    expect(apiTokenManagerSource).not.toContain('const loadTokens = async () =>');
    expect(apiTokenManagerSource).not.toContain('const hasAgentScopeResource = (resource: Resource)');
    expect(apiTokenManagerStateSource).toContain('export const useAPITokenManagerState =');
    expect(apiTokenManagerStateSource).toContain('const [tokens, setTokens] = createSignal<APITokenRecord[]>([])');
    expect(apiTokenManagerStateSource).toContain('const loadTokens = async () =>');
    expect(apiTokenManagerStateSource).toContain('SecurityAPI.listTokens()');
    expect(apiTokenManagerModelSource).toContain('export const hasAgentScopeResource =');
    expect(apiTokenManagerModelSource).toContain('export const buildDockerTokenUsage =');
    expect(apiTokenManagerModelSource).toContain('export const buildAgentTokenUsage =');
  });

  it('keeps the node setup modal shell behind extracted state and model owners', () => {
    expect(nodeModalSource).toContain('@/components/Settings/nodeModalModel');
    expect(nodeModalSource).toContain('@/components/Settings/useNodeModalState');
    expect(nodeModalSource).not.toContain('const deriveNameFromHost =');
    expect(nodeModalSource).not.toContain('const PVE_MANUAL_PERMISSION_COMMAND = `');
    expect(nodeModalSource).not.toContain('const [quickSetupBootstrap, setQuickSetupBootstrap] =');
    expect(nodeModalSource).not.toContain('const handleTestConnection = async () =>');
    expect(nodeModalModelSource).toContain('export interface NodeModalProps');
    expect(nodeModalModelSource).toContain('export const deriveNameFromHost =');
    expect(nodeModalModelSource).toContain('export const PVE_MANUAL_PERMISSION_COMMAND = `');
    expect(nodeModalStateSource).toContain('export const useNodeModalState =');
    expect(nodeModalStateSource).toContain('const [quickSetupBootstrap, setQuickSetupBootstrap] =');
    expect(nodeModalStateSource).toContain('const handleTestConnection = async () =>');
    expect(nodeModalStateSource).toContain("const PROXMOX_SETUP_HOST_REQUIRED_MESSAGE = 'Proxmox setup host is required';");
  });

  it('keeps AI settings sub-surfaces behind extracted runtime owners', () => {
    expect(aiSettingsPanelSource).toContain('@/components/Settings/AIProviderConfigurationSection');
    expect(aiSettingsPanelSource).toContain('@/components/Settings/AISettingsDialogs');
    expect(aiSettingsPanelSource).toContain('@/components/Settings/aiSettingsModel');
    expect(aiSettingsPanelSource).toContain('@/components/Settings/useAISettingsState');
    expect(aiSettingsPanelSource).not.toContain('const [loading, setLoading] = createSignal(false);');
    expect(aiSettingsPanelSource).not.toContain('const handleSave = async (event?: Event) =>');
    expect(aiSettingsPanelSource).not.toContain('AIAPI.getSettings()');
    expect(aiProviderConfigurationSectionSource).toContain('@/components/Settings/aiSettingsModel');
    expect(aiSettingsDialogsSource).toContain('@/components/Settings/aiSettingsModel');
    expect(aiSettingsModelSource).toContain('export const AI_PROVIDER_CONFIGS');
    expect(aiSettingsModelSource).toContain('export const AI_SETUP_PROVIDER_OPTIONS');
    expect(aiSettingsStateSource).toContain('export const useAISettingsState =');
    expect(aiSettingsStateSource).toContain('const [loading, setLoading] = createSignal(false);');
    expect(aiSettingsStateSource).toContain('const handleSave = async (event?: Event) =>');
    expect(aiSettingsStateSource).toContain('const handleEnabledToggle = async (newValue: boolean) =>');
    expect(aiSettingsStateSource).toContain('AIAPI.getSettings()');
  });

  it('keeps the updates settings shell behind extracted install-guide owners', () => {
    expect(updatesSettingsPanelSource).toContain('@/components/Settings/UpdateInstallGuide');
    expect(updatesSettingsPanelSource).toContain('@/components/Settings/updatesSettingsModel');
    expect(updatesSettingsPanelSource).not.toContain("navigator.clipboard.writeText('update')");
    expect(updatesSettingsPanelSource).not.toContain("value: 'stable'");
    expect(updatesSettingsPanelSource).not.toContain("value: 'rc'");
    expect(updateInstallGuideSource).toContain('@/components/Settings/CopyCommandBlock');
    expect(updateInstallGuideSource).toContain('buildUpdateInstallGuide');
    expect(copyCommandBlockSource).toContain('export function CopyCommandBlock');
    expect(copyCommandBlockSource).toContain("aria-label=\"Copy to clipboard\"");
    expect(updatesSettingsModelSource).toContain('export function getUpdateChannelCardOptions');
    expect(updatesSettingsModelSource).toContain('export function buildUpdateInstallGuide');
  });

  it('keeps the diagnostics shell behind extracted runtime and results owners', () => {
    expect(diagnosticsPanelSource).toContain('@/components/Settings/DiagnosticsResultsPanel');
    expect(diagnosticsPanelSource).toContain('@/components/Settings/useDiagnosticsPanelState');
    expect(diagnosticsPanelSource).toContain('formatUptime');
    expect(diagnosticsPanelSource).not.toContain("apiFetchJSON('/api/diagnostics')");
    expect(diagnosticsPanelSource).not.toContain('URL.createObjectURL');
    expect(diagnosticsPanelSource).not.toContain('sanitizeDiagnosticsData');
    expect(diagnosticsResultsPanelSource).toContain('DIAGNOSTICS_EMPTY_PBS_MESSAGE');
    expect(diagnosticsResultsPanelSource).toContain('getStatusIndicatorBadgeToneClasses(');
    expect(diagnosticsStateSource).toContain('export const useDiagnosticsPanelState =');
    expect(diagnosticsStateSource).toContain("apiFetchJSON('/api/diagnostics')");
    expect(diagnosticsStateSource).toContain('URL.createObjectURL');
    expect(diagnosticsModelSource).toContain('export function sanitizeDiagnosticsData');
    expect(diagnosticsModelSource).toContain('export function buildDiagnosticsExportFilename');
    expect(diagnosticsModelSource).toContain('export function formatUptime');
  });

  it('keeps the reporting shell behind extracted runtime and model owners', () => {
    expect(reportingPanelSource).toContain('@/components/Settings/useReportingPanelState');
    expect(reportingPanelSource).toContain('@/components/Settings/reportingPanelModel');
    expect(reportingPanelSource).not.toContain('loadLicenseStatus()');
    expect(reportingPanelSource).not.toContain('startProTrial()');
    expect(reportingPanelSource).not.toContain("apiFetch('/api/admin/reports/generate");
    expect(reportingPanelSource).not.toContain('window.URL.createObjectURL');
    expect(reportingPanelStateSource).toContain('export const useReportingPanelState =');
    expect(reportingPanelStateSource).toContain('loadLicenseStatus');
    expect(reportingPanelStateSource).toContain('startProTrial');
    expect(reportingPanelStateSource).toContain('buildReportingRequest');
    expect(reportingPanelStateSource).toContain('getReportingGenerateSuccessMessage');
    expect(reportingPanelModelSource).toContain('export function getReportingRangeStart');
    expect(reportingPanelModelSource).toContain('export function buildReportingRequest');
    expect(reportingPanelModelSource).toContain('export function buildReportingFilename');
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
  });

  it('keeps the SSO providers shell behind extracted runtime owners', () => {
    expect(ssoProvidersPanelSource).toContain('@/components/Settings/useSSOProvidersState');
    expect(ssoProvidersPanelSource).not.toContain('const loadProviders = async () =>');
    expect(ssoProvidersPanelSource).not.toContain('const handleSave = async (');
    expect(ssoProvidersPanelSource).not.toContain('const testConnection = async () =>');
    expect(ssoProvidersStateSource).toContain('@/components/Settings/ssoProvidersModel');
    expect(ssoProvidersStateSource).toContain('export const useSSOProvidersState =');
    expect(ssoProvidersStateSource).toContain('const loadProviders = async () =>');
    expect(ssoProvidersStateSource).toContain('const handleSave = async (event?: Event) =>');
    expect(ssoProvidersStateSource).toContain('const testConnection = async () =>');
    expect(ssoProvidersModelSource).toContain('export const createEmptyProviderForm =');
    expect(ssoProvidersModelSource).toContain('export const mapProviderDetailsToForm =');
    expect(ssoProvidersModelSource).toContain('export const buildProviderPayload =');
    expect(ssoProvidersModelSource).toContain('export const buildProviderTestPayload =');
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

  it('keeps shell titles aligned with the leading settings panel on key top-level surfaces', () => {
    for (const { tab, title, source } of canonicalShellTitleExpectations) {
      expect(SETTINGS_HEADER_META[tab].title, `${tab} should keep its shell title canonical`).toBe(
        title,
      );
      expect(
        source,
        `${tab} should keep its leading SettingsPanel title aligned with the shell title`,
      ).toContain(`title="${title}"`);
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
