import { describe, expect, it } from 'vitest';
import agentProfilesPanelSource from '../AgentProfilesPanel.tsx?raw';
import agentProfilesPanelStateSource from '../useAgentProfilesPanelState.ts?raw';
import apiTokenManagerSource from '../APITokenManager.tsx?raw';
import apiTokenManagerModelSource from '../apiTokenManagerModel.ts?raw';
import apiTokenManagerStateSource from '../useAPITokenManagerState.ts?raw';
import infrastructureActiveRowDetailsSource from '../InfrastructureActiveRowDetails.tsx?raw';
import infrastructureIgnoredRowDetailsSource from '../InfrastructureIgnoredRowDetails.tsx?raw';
import infrastructureInstallerSectionSource from '../InfrastructureInstallerSection.tsx?raw';
import infrastructureInventorySectionSource from '../InfrastructureInventorySection.tsx?raw';
import infrastructureInstallStateSource from '../useInfrastructureInstallState.tsx?raw';
import infrastructureOperationsStateSource from '../useInfrastructureOperationsState.tsx?raw';
import infrastructureReportingStateSource from '../useInfrastructureReportingState.tsx?raw';
import infrastructureSettingsModelSource from '../infrastructureSettingsModel.ts?raw';
import infrastructureConfiguredNodesStateSource from '../useInfrastructureConfiguredNodesState.ts?raw';
import infrastructureDiscoveryRuntimeStateSource from '../useInfrastructureDiscoveryRuntimeState.ts?raw';
import infrastructureStopMonitoringDialogSource from '../InfrastructureStopMonitoringDialog.tsx?raw';
import agentLedgerPanelSource from '../MonitoredSystemLedgerPanel.tsx?raw';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import setupCompletionPanelSource from '@/components/SetupWizard/SetupCompletionPanel.tsx?raw';
import infrastructureSelectorSource from '@/components/shared/InfrastructureSelector.tsx?raw';
import infrastructureSelectorModelSource from '@/components/shared/infrastructureSelectorModel.ts?raw';
import aiChatSource from '@/components/AI/Chat/index.tsx?raw';
import aiChatPresentationSource from '@/utils/aiChatPresentation.ts?raw';
import diagnosticsPanelSource from '../DiagnosticsPanel.tsx?raw';
import auditLogPanelSource from '../AuditLogPanel.tsx?raw';
import auditWebhookPanelSource from '../AuditWebhookPanel.tsx?raw';
import systemSettingsStateSource from '../useSystemSettingsState.ts?raw';
import infrastructureSettingsStateSource from '../useInfrastructureSettingsState.ts?raw';
import infrastructureWorkspaceSource from '../InfrastructureWorkspace.tsx?raw';
import infrastructureWorkspaceModelSource from '../infrastructureWorkspaceModel.ts?raw';
import settingsPanelRegistrySource from '../useSettingsPanelRegistry.tsx?raw';
import settingsSystemPanelsSource from '../useSettingsSystemPanels.tsx?raw';
import infrastructureSummarySource from '@/components/Infrastructure/InfrastructureSummary.tsx?raw';
import infrastructureSelectorComponentSource from '@/components/shared/InfrastructureSelector.tsx?raw';
import infrastructureSelectorStateSource from '@/components/shared/useInfrastructureSelectorState.ts?raw';
import workloadsLinkSource from '@/components/Infrastructure/workloadsLink.ts?raw';
import unifiedResourceHostTableCardSource from '@/components/Infrastructure/UnifiedResourceHostTableCard.tsx?raw';
import unifiedResourceTableSource from '@/components/Infrastructure/UnifiedResourceTable.tsx?raw';
import thresholdsTableSource from '@/components/Alerts/ThresholdsTable.tsx?raw';
import thresholdsTableAgentDisksSectionSource from '@/components/Alerts/ThresholdsTableAgentDisksSection.tsx?raw';
import thresholdsTableAgentsResourcesSectionSource from '@/components/Alerts/ThresholdsTableAgentsResourcesSection.tsx?raw';
import alertsConfigurationSurfaceSource from '@/features/alerts/AlertsConfigurationSurface.tsx?raw';
import thresholdsDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsData.ts?raw';
import thresholdsHostDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsHostData.ts?raw';
import thresholdsTableStateSource from '@/features/alerts/thresholds/hooks/useThresholdsTableState.ts?raw';
import collapsibleSectionSource from '@/components/Alerts/Thresholds/sections/CollapsibleSection.tsx?raw';
import alertThresholdsPresentationSource from '@/utils/alertThresholdsPresentation.ts?raw';
import alertThresholdsSectionPresentationSource from '@/utils/alertThresholdsSectionPresentation.ts?raw';
import monitoredSystemLedgerApiSource from '@/api/monitoredSystemLedger.ts?raw';
import monitoredSystemPresentationSource from '@/utils/monitoredSystemPresentation.ts?raw';
import loginSource from '@/components/Login.tsx?raw';
import settingsSource from '../Settings.tsx?raw';
import aiSettingsShellSource from '../AISettings.tsx?raw';
import aiChatMaintenanceSectionSource from '../AIChatMaintenanceSection.tsx?raw';
import aiProviderConfigurationSectionSource from '../AIProviderConfigurationSection.tsx?raw';
import aiSettingsDialogsSource from '../AISettingsDialogs.tsx?raw';
import aiModelSelectionSectionSource from '../AIModelSelectionSection.tsx?raw';
import aiSettingsModelSource from '../aiSettingsModel.ts?raw';
import aiRuntimeControlsSectionSource from '../AIRuntimeControlsSection.tsx?raw';
import aiSettingsStatusAndActionsSource from '../AISettingsStatusAndActions.tsx?raw';
import aiSettingsStateSource from '../useAISettingsState.ts?raw';
import reportingPanelSource from '../ReportingPanel.tsx?raw';
import reportingCatalogModelSource from '../reportingCatalogModel.ts?raw';
import reportingPanelModelSource from '../reportingPanelModel.ts?raw';
import reportingInventoryExportModelSource from '../reportingInventoryExportModel.ts?raw';
import updatesSettingsPanelSource from '../UpdatesSettingsPanel.tsx?raw';
import suggestProfileModalSource from '../SuggestProfileModal.tsx?raw';
import aiIntelligenceSource from '@/pages/AIIntelligence.tsx?raw';
import patrolIntelligenceBannersSource from '@/features/patrol/PatrolIntelligenceBanners.tsx?raw';
import patrolIntelligenceHeaderSource from '@/features/patrol/PatrolIntelligenceHeader.tsx?raw';
import patrolIntelligenceSummarySource from '@/features/patrol/PatrolIntelligenceSummary.tsx?raw';
import patrolIntelligenceSurfaceSource from '@/features/patrol/PatrolIntelligenceSurface.tsx?raw';
import patrolIntelligenceStateSource from '@/features/patrol/usePatrolIntelligenceState.ts?raw';
import patrolIntelligenceWorkspaceSource from '@/features/patrol/PatrolIntelligenceWorkspace.tsx?raw';
import aiPatrolSchedulePresentationSource from '@/utils/aiPatrolSchedulePresentation.ts?raw';
import patrolSummaryPresentationSource from '@/utils/patrolSummaryPresentation.ts?raw';
import aiCostDashboardSource from '@/components/AI/AICostDashboard.tsx?raw';
import aiCostPresentationSource from '@/utils/aiCostPresentation.ts?raw';
import aiControlLevelPresentationSource from '@/utils/aiControlLevelPresentation.ts?raw';
import agentProfilesPresentationSource from '@/utils/agentProfilesPresentation.ts?raw';
import agentProfileSuggestionPresentationSource from '@/utils/agentProfileSuggestionPresentation.ts?raw';
import reportingPresentationSource from '@/utils/reportingPresentation.ts?raw';
import reportingPanelStateSource from '../useReportingPanelState.ts?raw';
import aiSettingsPresentationSource from '@/utils/aiSettingsPresentation.ts?raw';
import aiSessionDiffPresentationSource from '@/utils/aiSessionDiffPresentation.ts?raw';
import settingsShellPresentationSource from '@/utils/settingsShellPresentation.ts?raw';
import upgradePresentationSource from '@/utils/upgradePresentation.ts?raw';
import appSource from '@/App.tsx?raw';
import apiTypesSource from '@/types/api.ts?raw';
import aiTypesSource from '@/types/ai.ts?raw';
import resourceTypesSource from '@/types/resource.ts?raw';
import unifiedResourcesHookSource from '@/hooks/useUnifiedResources.ts?raw';
import discoveryTypesSource from '@/types/discovery.ts?raw';
import discoveryTargetUtilsSource from '@/utils/discoveryTarget.ts?raw';
import mentionAutocompleteSource from '@/components/AI/Chat/MentionAutocomplete.tsx?raw';
import commandPaletteSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import organizationAccessPanelSource from '../OrganizationAccessPanel.tsx?raw';
import organizationAccessLoadingStateSource from '../OrganizationAccessLoadingState.tsx?raw';
import organizationAccessManagementSectionSource from '../OrganizationAccessManagementSection.tsx?raw';
import organizationAccessMembersSectionSource from '../OrganizationAccessMembersSection.tsx?raw';
import organizationOverviewPanelSource from '../OrganizationOverviewPanel.tsx?raw';
import organizationOverviewLoadingStateSource from '../OrganizationOverviewLoadingState.tsx?raw';
import organizationOverviewDetailsSectionSource from '../OrganizationOverviewDetailsSection.tsx?raw';
import organizationOverviewMembersSectionSource from '../OrganizationOverviewMembersSection.tsx?raw';
import organizationSharingPanelSource from '../OrganizationSharingPanel.tsx?raw';
import organizationSharingCreateSectionSource from '../OrganizationSharingCreateSection.tsx?raw';
import organizationSharingLoadingStateSource from '../OrganizationSharingLoadingState.tsx?raw';
import organizationOutgoingSharesSectionSource from '../OrganizationOutgoingSharesSection.tsx?raw';
import organizationIncomingSharesSectionSource from '../OrganizationIncomingSharesSection.tsx?raw';
import organizationAccessStateSource from '../useOrganizationAccessPanelState.ts?raw';
import organizationOverviewStateSource from '../useOrganizationOverviewPanelState.ts?raw';
import organizationSharingStateSource from '../useOrganizationSharingPanelState.ts?raw';
import canonicalResourceTypesSource from '@/utils/canonicalResourceTypes.ts?raw';
import organizationRolePresentationSource from '@/utils/organizationRolePresentation.ts?raw';
import organizationSettingsPresentationSource from '@/utils/organizationSettingsPresentation.ts?raw';
import resourceLinksSource from '@/routing/resourceLinks.ts?raw';
import alertsApiSource from '@/api/alerts.ts?raw';
import licenseApiSource from '@/api/license.ts?raw';
import monitoringApiSource from '@/api/monitoring.ts?raw';
import chartsApiSource from '@/api/charts.ts?raw';
import resourceStateAdaptersSource from '@/utils/resourceStateAdapters.ts?raw';
import resourceDetailMappersSource from '@/components/Infrastructure/resourceDetailMappers.ts?raw';
import resourceDetailDiscoveryModelSource from '@/components/Infrastructure/resourceDetailDiscoveryModel.ts?raw';
import resourceBadgesSource from '@/components/Infrastructure/resourceBadges.ts?raw';
import resourceBadgePresentationSource from '@/utils/resourceBadgePresentation.ts?raw';
import recoveryComponentSource from '@/components/Recovery/Recovery.tsx?raw';
import recoveryActivitySectionSource from '@/components/Recovery/RecoveryActivitySection.tsx?raw';
import recoveryHistorySectionSource from '@/components/Recovery/RecoveryHistorySection.tsx?raw';
import recoveryHistoryTableSource from '@/components/Recovery/RecoveryHistoryTable.tsx?raw';
import recoveryProtectedInventorySectionSource from '@/components/Recovery/RecoveryProtectedInventorySection.tsx?raw';
import recoveryHistorySectionStateSource from '@/components/Recovery/useRecoveryHistorySectionState.ts?raw';
import recoveryTablePresentationSource from '@/utils/recoveryTablePresentation.ts?raw';
import problemResourcesTableSource from '@/features/dashboardOverview/ProblemResourcesTable.tsx?raw';
import workloadTypeBadgesSource from '@/components/shared/workloadTypeBadges.ts?raw';
import workloadTypePresentationSource from '@/utils/workloadTypePresentation.ts?raw';

const aiSettingsSource = [
  aiSettingsShellSource,
  aiModelSelectionSectionSource,
  aiRuntimeControlsSectionSource,
  aiChatMaintenanceSectionSource,
  aiSettingsStatusAndActionsSource,
  aiProviderConfigurationSectionSource,
  aiSettingsDialogsSource,
  aiSettingsModelSource,
  aiSettingsStateSource,
].join('\n');
const infrastructureOperationsSource = [
  infrastructureInstallStateSource,
  infrastructureOperationsStateSource,
  infrastructureReportingStateSource,
  infrastructureInstallerSectionSource,
  infrastructureInventorySectionSource,
  infrastructureActiveRowDetailsSource,
  infrastructureIgnoredRowDetailsSource,
  infrastructureStopMonitoringDialogSource,
].join('\n');
import dashboardGuestPresentationSource from '@/utils/dashboardGuestPresentation.ts?raw';
import containerUpdatesSource from '@/stores/containerUpdates.ts?raw';
import websocketStoreSource from '@/stores/websocket.ts?raw';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import guestRowCellsSource from '@/components/Dashboard/GuestRowCells.tsx?raw';
import guestRowStateSource from '@/components/Dashboard/useGuestRowState.ts?raw';
import dashboardDiskListSource from '@/components/Dashboard/DiskList.tsx?raw';
import dashboardDiskListStateSource from '@/components/Dashboard/useDiskListState.ts?raw';
import guestDrawerSource from '@/components/Dashboard/GuestDrawer.tsx?raw';
import resourcePickerSource from '../ResourcePicker.tsx?raw';
import reportableResourceTypesSource from '@/utils/reportableResourceTypes.ts?raw';
import sourcePlatformsSource from '@/utils/sourcePlatforms.ts?raw';
import agentCapabilityPresentationSource from '@/utils/agentCapabilityPresentation.ts?raw';
import statusUtilsSource from '@/utils/status.ts?raw';
import unifiedAgentStatusPresentationSource from '@/utils/unifiedAgentStatusPresentation.ts?raw';
import unifiedAgentInventoryPresentationSource from '@/utils/unifiedAgentInventoryPresentation.ts?raw';
import relayPresentationSource from '@/utils/relayPresentation.ts?raw';
import relayPairingSectionSource from '../RelayPairingSection.tsx?raw';
import relaySettingsPanelSource from '../RelaySettingsPanel.tsx?raw';
import relaySettingsPanelStateSource from '../useRelaySettingsPanelState.ts?raw';
import proxmoxDeleteNodeDialogSource from '../ProxmoxDeleteNodeDialog.tsx?raw';
import proxmoxConfiguredNodesTableSource from '../ProxmoxConfiguredNodesTable.tsx?raw';
import proxmoxDirectWorkspaceSource from '../ProxmoxDirectWorkspace.tsx?raw';
import proxmoxDirectConnectionsCardSource from '../ProxmoxDirectConnectionsCard.tsx?raw';
import proxmoxDiscoveryResultsCardSource from '../ProxmoxDiscoveryResultsCard.tsx?raw';
import proxmoxNodeModalStackSource from '../ProxmoxNodeModalStack.tsx?raw';
import proxmoxSettingsPanelSource from '../ProxmoxSettingsPanel.tsx?raw';
import proxmoxSettingsModelSource from '../proxmoxSettingsModel.ts?raw';
import proxmoxDirectWorkspaceStateSource from '../useProxmoxDirectWorkspaceState.ts?raw';
import relayOnboardingCardSource from '@/components/Dashboard/RelayOnboardingCard.tsx?raw';
import relayOnboardingCardStateSource from '@/components/Dashboard/useRelayOnboardingCardState.ts?raw';
import generalSettingsPanelSource from '../GeneralSettingsPanel.tsx?raw';
import networkBoundarySettingsSectionSource from '../NetworkBoundarySettingsSection.tsx?raw';
import networkDiscoverySectionSource from '../NetworkDiscoverySection.tsx?raw';
import networkSettingsPanelSource from '../NetworkSettingsPanel.tsx?raw';
import recoverySettingsPanelSource from '../RecoverySettingsPanel.tsx?raw';
import rbacFeatureGateSectionSource from '../RBACFeatureGateSection.tsx?raw';
import rbacFeatureGateStateSource from '../useRBACFeatureGateState.ts?raw';
import rolesPanelSource from '../RolesPanel.tsx?raw';
import rolesEditorDialogSource from '../RolesEditorDialog.tsx?raw';
import rolesPanelStateSource from '../useRolesPanelState.ts?raw';
import userAssignmentsPanelSource from '../UserAssignmentsPanel.tsx?raw';
import userAssignmentsDialogSource from '../UserAssignmentsDialog.tsx?raw';
import userAssignmentsPanelStateSource from '../useUserAssignmentsPanelState.ts?raw';
import ssoProvidersPanelSource from '../SSOProvidersPanel.tsx?raw';
import ssoProvidersStateSource from '../useSSOProvidersState.ts?raw';
import ssoProvidersModelSource from '../ssoProvidersModel.ts?raw';
import rbacPermissionsSource from '@/utils/rbacPermissions.ts?raw';
import rbacPresentationSource from '@/utils/rbacPresentation.ts?raw';
import apiTokenPresentationSource from '@/utils/apiTokenPresentation.ts?raw';
import ssoProviderPresentationSource from '@/utils/ssoProviderPresentation.ts?raw';
import systemSettingsPresentationSource from '@/utils/systemSettingsPresentation.ts?raw';

const recoverySource = [
  recoveryComponentSource,
  recoveryProtectedInventorySectionSource,
  recoveryActivitySectionSource,
  recoveryHistorySectionSource,
  recoveryHistoryTableSource,
  recoveryHistorySectionStateSource,
].join('\n');

describe('monitored-system model guardrails', () => {
  it('keeps AgentProfilesPanel on unified resources (not host-only slices)', () => {
    expect(agentProfilesPanelSource).toContain('import { useAgentProfilesPanelState }');
    expect(agentProfilesPanelSource).toContain('useAgentProfilesPanelState()');
    expect(agentProfilesPanelSource).toContain('@/utils/agentProfilesPresentation');
    expect(agentProfilesPanelSource).toContain('getAgentProfilesEmptyState');
    expect(agentProfilesPanelSource).toContain('getAgentProfileAssignmentsEmptyState');
    expect(agentProfilesPanelSource).not.toContain('const loadData = async () =>');
    expect(agentProfilesPanelSource).not.toContain('const handleSave = async () =>');
    expect(agentProfilesPanelSource).not.toContain('AIAPI.getSettings()');
    expect(agentProfilesPanelSource).not.toContain("const hosts = byType('host')");
    expect(agentProfilesPanelSource).not.toContain('No profiles yet. Create one to get started.');
    expect(agentProfilesPanelSource).not.toContain(
      'No agents connected. Install an agent to assign profiles.',
    );
    expect(agentProfilesPanelStateSource).toContain('const { resources } = useResources()');
    expect(agentProfilesPanelStateSource).toContain('const loadData = async () =>');
    expect(agentProfilesPanelStateSource).toContain('const handleSave = async () =>');
    expect(agentProfilesPanelStateSource).toContain('AIAPI.getSettings()');
    expect(agentProfilesPanelStateSource).toContain('AgentProfilesAPI.listProfiles()');
    expect(agentProfilesPanelStateSource).toContain('AgentProfilesAPI.listAssignments()');
    expect(agentProfilesPresentationSource).toContain('export function getAgentProfilesEmptyState');
    expect(agentProfilesPresentationSource).toContain(
      'export function getAgentProfileAssignmentsEmptyState',
    );
    expect(agentLedgerPanelSource).toContain('getSimpleStatusIndicator');
    expect(agentLedgerPanelSource).toContain('getMonitoredSystemLedgerLoadingState');
    expect(agentLedgerPanelSource).toContain('getMonitoredSystemLedgerErrorState');
    expect(agentLedgerPanelSource).not.toContain('Loading monitored system ledger...');
    expect(agentLedgerPanelSource).not.toContain('Failed to load monitored system ledger.');
    expect(agentLedgerPanelSource).not.toContain('function statusVariant(');
    expect(monitoredSystemLedgerApiSource).toContain('@/utils/monitoredSystemPresentation');
    expect(monitoredSystemLedgerApiSource).toContain('getMonitoredSystemStatusFallbackSummary');
    expect(monitoredSystemLedgerApiSource).toContain(
      'getMonitoredSystemExplanationFallbackSummary',
    );
    expect(monitoredSystemLedgerApiSource).toContain('type MonitoredSystemLedgerRawEntry =');
    expect(monitoredSystemLedgerApiSource).toContain('systems?: MonitoredSystemLedgerRawEntry[];');
    expect(monitoredSystemLedgerApiSource).toContain('reported_at: string;');
    expect(monitoredSystemLedgerApiSource).not.toContain('last_seen: string;');
    expect(monitoredSystemLedgerApiSource).not.toContain('latest_included_signal_at?: string;');
    expect(monitoredSystemLedgerApiSource).not.toContain('latest_included_signal_source?: string;');
    expect(monitoredSystemLedgerApiSource).not.toContain('last_seen?: string;');
    expect(monitoredSystemLedgerApiSource).not.toContain(
      'All included top-level collection paths currently report online status.',
    );
    expect(monitoredSystemLedgerApiSource).not.toContain(
      'At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning.',
    );
    expect(monitoredSystemLedgerApiSource).not.toContain(
      'Pulse cannot determine a canonical runtime status for this monitored system yet.',
    );
    expect(monitoredSystemPresentationSource).toContain('statusSummaryByStatus');
    expect(agentLedgerPanelSource).not.toContain('getMonitoredSystemExplanationFallbackSummary');
    expect(agentLedgerPanelSource).not.toContain('getMonitoredSystemStatusFallbackSummary');
    expect(agentLedgerPanelSource).not.toContain('function systemExplanation(');
    expect(agentLedgerPanelSource).not.toContain('function systemStatusExplanation(');
  });

  it('keeps APITokenManager runtime usage mapped from unified resources', () => {
    expect(apiTokenManagerSource).toContain('import { useAPITokenManagerState }');
    expect(apiTokenManagerSource).toContain('useAPITokenManagerState(props)');
    expect(apiTokenManagerSource).not.toContain('const agentCapableResources = createMemo(() =>');
    expect(apiTokenManagerSource).not.toContain('@/utils/apiTokenPresentation');
    expect(apiTokenManagerStateSource).toContain('const agentCapableResources = createMemo(() =>');
    expect(apiTokenManagerStateSource).toContain('@/utils/apiTokenPresentation');
    expect(apiTokenManagerStateSource).toContain(
      "const dockerRuntimeResources = createMemo(() => byType('docker-host'))",
    );
    expect(apiTokenManagerModelSource).toContain('export const hasAgentScopeResource =');
    expect(apiTokenManagerModelSource).toContain("resource.type === 'agent'");
    expect(apiTokenManagerModelSource).toContain("resource.type === 'pbs'");
    expect(apiTokenManagerModelSource).toContain("resource.type === 'pmg'");
    expect(apiTokenManagerModelSource).toContain("resource.type === 'truenas'");
    expect(apiTokenManagerModelSource).toContain('resourceHasAgentFacet(resource)');
    expect(apiTokenManagerModelSource).toContain(
      'getActionableDockerRuntimeIdFromResource(resource)',
    );
    expect(apiTokenManagerStateSource).toContain('markDockerRuntimesTokenRevoked');
    expect(apiTokenManagerStateSource).not.toContain('markDockerHostsTokenRevoked');
    expect(apiTokenManagerModelSource).not.toContain("resource.type === 'host'");
    expect(apiTokenManagerStateSource).not.toContain('markHostsTokenRevoked');
    expect(apiTokenManagerStateSource).not.toContain(
      "const hostResources = createMemo(() => byType('host'))",
    );
    expect(apiTokenManagerModelSource).not.toContain('isAppContainerDiscoveryResourceType');
    expect(apiTokenManagerStateSource).not.toContain(
      "notificationStore.error('Failed to load API tokens')",
    );
    expect(apiTokenManagerStateSource).not.toContain(
      "notificationStore.error('Failed to generate API token')",
    );
    expect(apiTokenManagerStateSource).not.toContain(
      "notificationStore.error('Failed to revoke API token')",
    );
    expect(apiTokenPresentationSource).toContain('export function getAPITokensLoadErrorMessage');
  });

  it('keeps infrastructure operations free of v5 merge-workaround patterns', () => {
    expect(infrastructureOperationsStateSource).not.toContain('previousHostTypes');
    expect(infrastructureOperationsStateSource).not.toContain('const allHosts = createMemo(');
    expect(infrastructureOperationsSource).toContain('@/utils/unifiedAgentStatusPresentation');
    expect(infrastructureOperationsSource).not.toContain('const MONITORING_STOPPED_STATUS_LABEL =');
    expect(infrastructureOperationsSource).not.toContain('const ALLOW_RECONNECT_LABEL =');
    expect(infrastructureOperationsStateSource).toContain('withPrivilegeEscalation');
    expect(infrastructureOperationsStateSource).toContain('./useInfrastructureInstallState');
    expect(infrastructureOperationsStateSource).toContain('./useInfrastructureReportingState');
    expect(infrastructureOperationsSource).toContain('@/utils/agentCapabilityPresentation');
    expect(infrastructureOperationsSource).not.toContain('const getCapabilityLabel =');
    expect(infrastructureOperationsSource).not.toContain('const getCapabilityBadgeClass =');
    expect(agentCapabilityPresentationSource).toContain("export type AgentCapability = 'agent'");
    expect(agentCapabilityPresentationSource).toContain('export function getAgentCapabilityLabel');
    expect(agentCapabilityPresentationSource).toContain(
      'export function getAgentCapabilityBadgeClass',
    );
    expect(infrastructureReportingStateSource).not.toContain('isConnectedHealthStatus');
    expect(infrastructureReportingStateSource).not.toContain('const connectedFromStatus =');
    expect(agentProfilesPanelStateSource).toContain('isConnectedHealthStatus');
    expect(agentProfilesPanelSource).not.toContain('const connectedFromStatus =');
    expect(statusUtilsSource).toContain('export function isConnectedHealthStatus');
    expect(infrastructureOperationsSource).toContain('@/utils/unifiedAgentStatusPresentation');
    expect(infrastructureOperationsSource).not.toContain('const statusBadgeClass =');
    expect(infrastructureOperationsSource).not.toContain('const statusBadgeClasses =');
    expect(infrastructureOperationsSource).toContain('getUnifiedAgentLookupStatusPresentation');
    expect(unifiedAgentStatusPresentationSource).toContain(
      'export function getUnifiedAgentStatusPresentation',
    );
    expect(unifiedAgentStatusPresentationSource).toContain(
      'export function getUnifiedAgentLookupStatusPresentation',
    );
    expect(infrastructureOperationsSource).toContain('getInventorySubjectLabel');
    expect(infrastructureOperationsSource).toContain('getMonitoringStoppedEmptyState');
    expect(infrastructureOperationsSource).toContain('getRemovedUnifiedAgentItemLabel');
    expect(infrastructureOperationsSource).toContain('getUnifiedAgentLastSeenLabel');
    expect(infrastructureOperationsSource).not.toContain('const getInventorySubjectLabel =');
    expect(infrastructureOperationsSource).not.toContain('const getRemovedItemLabel =');
    expect(infrastructureOperationsSource).not.toContain('const lastSeenLabel = () => {');
    expect(infrastructureOperationsSource).not.toContain(
      'No monitoring-stopped items match the current filters.',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getInventorySubjectLabel',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getRemovedUnifiedAgentItemLabel',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getUnifiedAgentLastSeenLabel',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getMonitoringStoppedEmptyState',
    );
    expect(unifiedAgentInventoryPresentationSource).not.toContain(
      'export function getMonitoredSystemLedgerLoadingState',
    );
    expect(unifiedAgentInventoryPresentationSource).not.toContain(
      'export function getMonitoredSystemLedgerErrorState',
    );
    expect(monitoredSystemPresentationSource).toContain(
      'export function getMonitoredSystemLedgerLoadingState',
    );
    expect(monitoredSystemPresentationSource).toContain(
      'export function getMonitoredSystemLedgerErrorState',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getUnifiedAgentStopMonitoringSuccessMessage',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getUnifiedAgentAllowReconnectSuccessMessage',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getUnifiedAgentConfigUpdateSuccessMessage',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getUnifiedAgentClipboardCopySuccessMessage',
    );
    expect(infrastructureOperationsSource).not.toContain(
      'No host identifiers are available to stop monitoring.',
    );
    expect(infrastructureOperationsSource).not.toContain('Failed to update agent configuration');
    expect(infrastructureOperationsSource).not.toContain('Uninstall command copied');
    expect(infrastructureOperationsSource).not.toContain('Upgrade command copied');
    expect(infrastructureOperationsSource).not.toContain(
      "notificationStore.error('Failed to copy')",
    );
    expect(relaySettingsPanelSource).toContain('./useRelaySettingsPanelState');
    expect(relaySettingsPanelSource).toContain('./RelayPairingSection');
    expect(relaySettingsPanelSource).toContain('getSettingsConfigurationLoadingState');
    expect(relaySettingsPanelSource).toContain('@/utils/upgradePresentation');
    expect(relaySettingsPanelSource).toContain('UPGRADE_TRIAL_LABEL');
    expect(relaySettingsPanelSource).not.toContain('createSignal(');
    expect(relaySettingsPanelSource).not.toContain('Loading configuration...');
    expect(relaySettingsPanelSource).not.toContain('>Start free trial<');
    expect(relaySettingsPanelStateSource).toContain('getRelayConnectionPresentation');
    expect(relaySettingsPanelStateSource).toContain('trackPaywallViewed');
    expect(relayPairingSectionSource).toContain('getRelayDiagnosticClass');
    expect(proxmoxSettingsPanelSource).toContain('CalloutCard');
    expect(proxmoxSettingsPanelSource).not.toContain('Loading configuration...');
    expect(proxmoxSettingsPanelSource).toContain('./ProxmoxDirectWorkspace');
    expect(proxmoxDirectWorkspaceSource).toContain('./useProxmoxDirectWorkspaceState');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxConfiguredNodesTable');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxDirectConnectionsCard');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxDiscoveryResultsCard');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxDeleteNodeDialog');
    expect(proxmoxDirectWorkspaceSource).toContain('./ProxmoxNodeModalStack');
    expect(proxmoxSettingsPanelSource).not.toContain('setPrefillNode(');
    expect(proxmoxSettingsPanelSource).not.toContain('const renderConfiguredTable = () =>');
    expect(proxmoxSettingsPanelSource).not.toContain('const renderNodeModal = (type: NodeType)');
    expect(proxmoxSettingsPanelSource).not.toContain(
      'No discovery matches for this Proxmox type yet.',
    );
    expect(proxmoxSettingsPanelSource).not.toContain('What happens next');
    expect(proxmoxConfiguredNodesTableSource).toContain('PveNodesTable');
    expect(proxmoxDirectConnectionsCardSource).toContain('getSettingsConfigurationLoadingState');
    expect(proxmoxDiscoveryResultsCardSource).toContain('formatRelativeTime');
    expect(proxmoxDeleteNodeDialogSource).toContain('SectionHeader');
    expect(proxmoxNodeModalStackSource).toContain('<NodeModal');
    expect(proxmoxDirectWorkspaceStateSource).toContain('buildProxmoxDiscoveryPrefillNode');
    expect(proxmoxSettingsModelSource).toContain('./infrastructureSettingsModel');
    expect(infrastructureSettingsStateSource).toContain('./useInfrastructureConfiguredNodesState');
    expect(infrastructureSettingsStateSource).toContain('./useInfrastructureDiscoveryRuntimeState');
    expect(infrastructureSettingsStateSource).not.toContain('NodesAPI.getNodes');
    expect(infrastructureSettingsStateSource).not.toContain('SettingsAPI.updateSystemSettings');
    expect(infrastructureConfiguredNodesStateSource).toContain('NodesAPI.getNodes');
    expect(infrastructureConfiguredNodesStateSource).toContain('NodesAPI.deleteNode');
    expect(infrastructureConfiguredNodesStateSource).toContain('NodesAPI.refreshClusterNodes');
    expect(infrastructureConfiguredNodesStateSource).not.toContain("apiFetch('/api/discover'");
    expect(infrastructureDiscoveryRuntimeStateSource).toContain("apiFetch('/api/discover'");
    expect(infrastructureDiscoveryRuntimeStateSource).toContain('SettingsAPI.updateSystemSettings');
    expect(infrastructureDiscoveryRuntimeStateSource).toContain("eventBus.on('discovery_status'");
    expect(infrastructureDiscoveryRuntimeStateSource).not.toContain('NodesAPI.getNodes');
    expect(infrastructureSettingsModelSource).toContain('collectConfiguredInfrastructureHosts');
    expect(infrastructureSettingsModelSource).toContain('matchConfiguredNodeToResource');
    expect(relayOnboardingCardSource).toContain('@/utils/relayPresentation');
    expect(relayOnboardingCardSource).toContain('./useRelayOnboardingCardState');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_TITLE');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_DESCRIPTION');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_UPGRADE_LABEL');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_TRIAL_LABEL');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_TRIAL_STARTING_LABEL');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_SETUP_LABEL');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_DISCONNECTED_LABEL');
    expect(relayOnboardingCardSource).not.toContain('createSignal(');
    expect(relayOnboardingCardSource).not.toContain('loadLicenseStatus()');
    expect(relayOnboardingCardSource).not.toContain('RelayAPI.getStatus()');
    expect(relayOnboardingCardSource).not.toContain('startProTrial()');
    expect(relayOnboardingCardSource).not.toContain('Pair Your Mobile Device');
    expect(relayOnboardingCardSource).not.toContain('Relay is currently disconnected.');
    expect(relayOnboardingCardStateSource).toContain('loadLicenseStatus()');
    expect(relayOnboardingCardStateSource).toContain('RelayAPI.getStatus()');
    expect(relayOnboardingCardStateSource).toContain('trackPaywallViewed');
    expect(relayOnboardingCardStateSource).toContain('runStartProTrialAction({');
    expect(relayOnboardingCardStateSource).not.toContain('startProTrial()');
    expect(infrastructureInstallStateSource).toContain('STORAGE_KEYS.SETUP_HANDOFF');
    expect(infrastructureInstallerSectionSource).toContain(
      'Security configured. Save these first-run credentials now.',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'Generate a scoped install token below before copying agent commands.',
    );
    expect(setupCompletionPanelSource).toContain('@/utils/relayPresentation');
    expect(setupCompletionPanelSource).toContain('RELAY_ONBOARDING_SETUP_LABEL');
    expect(setupCompletionPanelSource).toContain('RELAY_ONBOARDING_TRIAL_STARTING_LABEL');
    expect(setupCompletionPanelSource).toContain('RELAY_ONBOARDING_SETUP_WIZARD_TRIAL_LABEL');
    expect(setupCompletionPanelSource).toContain('RELAY_ONBOARDING_TRIAL_HINT');
    expect(setupCompletionPanelSource).not.toContain('Start Free Trial & Set Up Mobile');
    expect(setupCompletionPanelSource).not.toContain('14-DAY PRO TRIAL');
    expect(relayPresentationSource).toContain('export function getRelayConnectionPresentation');
    expect(relayPresentationSource).toContain('export function getRelayDiagnosticClass');
    expect(relayPresentationSource).toContain('export const RELAY_ONBOARDING_TITLE');
    expect(relayPresentationSource).toContain('export const RELAY_ONBOARDING_DESCRIPTION');
    expect(relayPresentationSource).toContain('export const RELAY_ONBOARDING_UPGRADE_LABEL');
    expect(relayPresentationSource).toContain(
      'export const RELAY_ONBOARDING_SETUP_WIZARD_TRIAL_LABEL',
    );
    expect(relayPresentationSource).toContain('export const RELAY_ONBOARDING_TRIAL_HINT');
    expect(aiSettingsSource).toContain('@/utils/aiControlLevelPresentation');
    expect(aiSettingsSource).toContain('@/utils/aiSettingsPresentation');
    expect(aiSettingsSource).toContain('getAIControlLevelPanelClass');
    expect(aiSettingsSource).toContain('getAIControlLevelBadgeClass');
    expect(aiSettingsSource).toContain('getAIControlLevelDescription');
    expect(aiSettingsSource).toContain('getAISettingsReadinessPresentation');
    expect(aiSettingsSource).toContain('getAISettingsLoadingState');
    expect(aiSettingsSource).toContain('getAIChatSessionsLoadingState');
    expect(aiSettingsSource).toContain('getAIChatSessionsEmptyState');
    expect(aiSettingsSource).toContain('getAIOAuthErrorMessage');
    expect(aiSettingsSource).toContain('getAISessionSummarizeErrorMessage');
    expect(aiSettingsSource).toContain('getAISessionDiffErrorMessage');
    expect(aiSettingsSource).toContain('getAISessionRevertErrorMessage');
    expect(aiSettingsSource).toContain('getAISettingsSaveErrorMessage');
    expect(aiSettingsSource).toContain('getAICredentialsClearErrorMessage');
    expect(aiSettingsSource).toContain('getAISettingsToggleErrorMessage');
    expect(aiSettingsShellSource).toContain('@/components/Settings/useAISettingsState');
    expect(aiSettingsShellSource).toContain('@/components/Settings/AIModelSelectionSection');
    expect(aiSettingsShellSource).toContain('@/components/Settings/AIRuntimeControlsSection');
    expect(aiSettingsShellSource).toContain('@/components/Settings/AIChatMaintenanceSection');
    expect(aiSettingsShellSource).toContain('@/components/Settings/AISettingsStatusAndActions');
    expect(aiRuntimeControlsSectionSource).toContain('@/utils/upgradePresentation');
    expect(aiRuntimeControlsSectionSource).toContain('UPGRADE_ACTION_LABEL');
    expect(aiRuntimeControlsSectionSource).toContain('UPGRADE_TRIAL_LABEL');
    expect(aiSettingsSource).not.toContain('Loading Pulse Assistant settings...');
    expect(aiSettingsSource).not.toContain('Loading chat sessions...');
    expect(aiSettingsSource).not.toContain('No chat sessions yet. Start a chat to create one.');
    expect(aiSettingsSource).not.toContain("'Failed to summarize session.'");
    expect(aiSettingsSource).not.toContain("'Failed to get session diff.'");
    expect(aiSettingsSource).not.toContain("'Failed to revert session.'");
    expect(aiSettingsSource).not.toContain("'Failed to save Pulse Assistant settings'");
    expect(aiSettingsSource).not.toContain("'Failed to clear credentials'");
    expect(aiRuntimeControlsSectionSource).not.toContain('>Upgrade to Pro<');
    expect(aiRuntimeControlsSectionSource).not.toContain('>Start free trial<');
    expect(aiSettingsSource).not.toContain("'Failed to update Pulse Assistant setting'");
    expect(aiSettingsSource).not.toContain('const normalizeControlLevel =');
    expect(aiSettingsSource).not.toContain('const errorMessages: Record<string, string> =');
    expect(aiSettingsShellSource).not.toContain(
      'const [loading, setLoading] = createSignal(false);',
    );
    expect(aiSettingsShellSource).not.toContain('const handleSave = async (event?: Event) =>');
    expect(aiSettingsShellSource).not.toContain('Chat Session Maintenance');
    expect(aiSettingsShellSource).not.toContain('Discovery Settings');
    expect(aiSettingsShellSource).not.toContain('Pulse Permission Level');
    expect(aiSettingsStateSource).toContain('export const useAISettingsState =');
    expect(aiSettingsStateSource).toContain('export type AISettingsState =');
    expect(aiSettingsStateSource).toContain('const [loading, setLoading] = createSignal(false);');
    expect(aiSettingsStateSource).toContain('const handleSave = async (event?: Event) =>');
    expect(aiSettingsStateSource).toContain(
      'const handleEnabledToggle = async (newValue: boolean) =>',
    );
    expect(aiControlLevelPresentationSource).toContain('export function normalizeAIControlLevel');
    expect(aiControlLevelPresentationSource).toContain(
      'export function getAIControlLevelPanelClass',
    );
    expect(aiControlLevelPresentationSource).toContain(
      'export function getAIControlLevelBadgeClass',
    );
    expect(aiControlLevelPresentationSource).toContain(
      'export function getAIControlLevelDescription',
    );
    expect(aiControlLevelPresentationSource).toContain(
      'export function getAIChatControlLevelPresentation',
    );
    expect(aiSettingsPresentationSource).toContain(
      'export function getAISettingsReadinessPresentation',
    );
    expect(aiSettingsPresentationSource).toContain('export function getAISettingsLoadingState');
    expect(aiSettingsPresentationSource).toContain('export function getAISettingsLoadErrorMessage');
    expect(aiSettingsPresentationSource).toContain('export function getAISettingsRetryLabel');
    expect(aiSettingsDialogsSource).toContain('SelectionCardGroup');
    expect(aiSettingsDialogsSource).toContain('variant="compact"');
    expect(updatesSettingsPanelSource).toContain('SelectionCardGroup');
    expect(updatesSettingsPanelSource).toContain('variant="detail"');
    expect(aiSettingsPresentationSource).toContain('export function getAIChatSessionsLoadingState');
    expect(aiSettingsPresentationSource).toContain('export function getAIChatSessionsEmptyState');
    expect(aiSettingsPresentationSource).toContain('export function getAIModelsLoadErrorMessage');
    expect(aiSettingsPresentationSource).toContain(
      'export function getAIChatSessionsLoadErrorMessage',
    );
    expect(aiSettingsPresentationSource).toContain('export function getAIOAuthErrorMessage');
    expect(suggestProfileModalSource).toContain('@/utils/agentProfileSuggestionPresentation');
    expect(suggestProfileModalSource).toContain('AGENT_PROFILE_SUGGESTION_EXAMPLE_PROMPTS');
    expect(suggestProfileModalSource).toContain('getAgentProfileSuggestionLoadingState');
    expect(suggestProfileModalSource).toContain('getAgentProfileSuggestionKeyLabel');
    expect(suggestProfileModalSource).toContain('formatAgentProfileSuggestionValue');
    expect(suggestProfileModalSource).toContain('hasAgentProfileSuggestionValue');
    expect(suggestProfileModalSource).toContain('getAgentProfileSuggestionRiskHints');
    expect(suggestProfileModalSource).toContain('getAgentProfileSuggestionValueBadgeClass');
    expect(suggestProfileModalSource).not.toContain('const formatValueBadgeClass =');
    expect(suggestProfileModalSource).not.toContain('Generating suggestion...');
    expect(suggestProfileModalSource).not.toContain('const toTitleCase =');
    expect(suggestProfileModalSource).not.toContain('const formatDisplayValue =');
    expect(suggestProfileModalSource).not.toContain('const hasValue =');
    expect(suggestProfileModalSource).not.toContain('const examplePrompts =');
    expect(agentProfileSuggestionPresentationSource).toContain(
      'export function getAgentProfileSuggestionValueBadgeClass',
    );
    expect(agentProfileSuggestionPresentationSource).toContain(
      'export const AGENT_PROFILE_SUGGESTION_EXAMPLE_PROMPTS',
    );
    expect(agentProfileSuggestionPresentationSource).toContain(
      'export function getAgentProfileSuggestionKeyLabel',
    );
    expect(agentProfileSuggestionPresentationSource).toContain(
      'export function formatAgentProfileSuggestionValue',
    );
    expect(agentProfileSuggestionPresentationSource).toContain(
      'export function hasAgentProfileSuggestionValue',
    );
    expect(agentProfileSuggestionPresentationSource).toContain(
      'export function getAgentProfileSuggestionRiskHints',
    );
    expect(reportingPanelSource).toContain('FilterButtonGroup');
    expect(reportingPanelSource).toContain('CalloutCard');
    expect(reportingPanelSource).toContain('variant="prominent"');
    expect(reportingPanelSource).toContain('@/utils/upgradePresentation');
    expect(reportingPanelSource).toContain('@/components/Settings/useReportingPanelState');
    expect(reportingPanelSource).toContain('@/components/Settings/reportingCatalogModel');
    expect(reportingPanelSource).toContain('@/components/Settings/reportingPanelModel');
    expect(reportingPanelSource).toContain('reportingCatalog');
    expect(reportingPanelSource).toContain('getUpgradeActionButtonClass');
    expect(reportingPanelSource).toContain('UPGRADE_ACTION_LABEL');
    expect(reportingPanelSource).toContain('UPGRADE_TRIAL_LABEL');
    expect(reportingPanelSource).not.toContain('>Upgrade to Pro<');
    expect(reportingPanelSource).not.toContain('>Start free trial<');
    expect(reportingPanelSource).not.toContain("<For each={['24h', '7d', '30d']}>");
    expect(reportingPanelSource).not.toContain('window.URL.createObjectURL');
    expect(reportingPanelStateSource).toContain('buildReportingRequest');
    expect(reportingPanelStateSource).toContain('buildReportingCatalogRequest');
    expect(reportingPanelStateSource).toContain('parseReportingCatalog');
    expect(reportingPanelStateSource).toContain('getReportingGenerateSelectionRequiredMessage');
    expect(reportingPanelStateSource).toContain('getReportingGenerateSuccessMessage');
    expect(reportingPanelStateSource).toContain('getReportingGenerateErrorMessage');
    expect(reportingPanelStateSource).toContain('buildVMInventoryExportRequest');
    expect(reportingPanelStateSource).toContain('getReportingInventoryExportSuccessMessage');
    expect(reportingCatalogModelSource).toContain('export function buildReportingCatalogRequest');
    expect(reportingCatalogModelSource).toContain('export function parseReportingCatalog');
    expect(reportingPanelModelSource).toContain('export function getReportingRangeStart');
    expect(reportingPanelModelSource).toContain('export function buildReportingRequest');
    expect(reportingInventoryExportModelSource).toContain(
      'export function buildVMInventoryExportRequest',
    );
    expect(reportingPresentationSource).toContain(
      'export function getReportingGenerateSelectionRequiredMessage',
    );
    expect(reportingPresentationSource).toContain(
      'export function getReportingGenerateSuccessMessage',
    );
    expect(reportingPresentationSource).toContain(
      'export function getReportingGenerateErrorMessage',
    );
    expect(reportingPresentationSource).toContain(
      'export function getReportingInventoryExportSuccessMessage',
    );
    expect(reportingPresentationSource).not.toContain('getReportingToggleButtonClass');
    expect(aiIntelligenceSource).toContain(
      "import { PatrolIntelligenceSurface } from '@/features/patrol/PatrolIntelligenceSurface';",
    );
    expect(aiIntelligenceSource).not.toContain('buildPatrolScheduleOptions');
    expect(aiIntelligenceSource).not.toContain('PATROL_NO_ISSUES_LABEL');
    expect(patrolIntelligenceSurfaceSource).toContain('./usePatrolIntelligenceState');
    expect(patrolIntelligenceSurfaceSource).toContain('./PatrolIntelligenceHeader');
    expect(patrolIntelligenceSurfaceSource).toContain('./PatrolIntelligenceBanners');
    expect(patrolIntelligenceSurfaceSource).toContain('./PatrolIntelligenceSummary');
    expect(patrolIntelligenceSurfaceSource).toContain('./PatrolIntelligenceWorkspace');
    expect(patrolIntelligenceSurfaceSource).not.toContain('buildPatrolScheduleOptions');
    expect(patrolIntelligenceSurfaceSource).not.toContain('PATROL_NO_ISSUES_LABEL');
    expect(patrolIntelligenceStateSource).toContain('export function usePatrolIntelligenceState');
    expect(patrolIntelligenceStateSource).toContain('export type PatrolIntelligenceState =');
    expect(patrolIntelligenceStateSource).toContain('getPatrolStatus');
    expect(patrolIntelligenceStateSource).toContain('usePatrolStream');
    expect(patrolIntelligenceStateSource).toContain('updatePatrolAutonomySettings');
    expect(patrolIntelligenceHeaderSource).toContain('buildPatrolScheduleOptions');
    expect(patrolIntelligenceHeaderSource).toContain('getAIQuickstartCreditsPresentation');
    expect(patrolIntelligenceSummarySource).toContain('PATROL_NO_ISSUES_LABEL');
    expect(patrolIntelligenceSummarySource).toContain('getPatrolSummaryPresentation');
    expect(patrolIntelligenceWorkspaceSource).toContain('ApprovalBanner');
    expect(patrolIntelligenceWorkspaceSource).toContain('FindingsPanel');
    expect(patrolIntelligenceBannersSource).toContain('trackUpgradeClicked');
    expect(aiIntelligenceSource).not.toContain('No issues found');
    expect(patrolIntelligenceSurfaceSource).not.toContain('No issues found');
    expect(aiIntelligenceSource).not.toContain('const SCHEDULE_PRESETS =');
    expect(patrolIntelligenceSurfaceSource).not.toContain('const SCHEDULE_PRESETS =');
    expect(aiPatrolSchedulePresentationSource).toContain('export const PATROL_SCHEDULE_PRESETS');
    expect(aiPatrolSchedulePresentationSource).toContain(
      'export function buildPatrolScheduleOptions',
    );
    expect(patrolSummaryPresentationSource).toContain('export const PATROL_NO_ISSUES_LABEL');
    expect(aiCostDashboardSource).toContain('getAICostRangeButtonClass');
    expect(aiCostDashboardSource).toContain('getAICostLoadingState');
    expect(aiCostDashboardSource).toContain('AI_COST_EMPTY_STATE');
    expect(aiCostDashboardSource).toContain('AI_COST_DAILY_USD_EMPTY_STATE');
    expect(aiCostDashboardSource).toContain('AI_COST_DAILY_TOKEN_EMPTY_STATE');
    expect(aiCostDashboardSource).not.toContain('No usage data yet.');
    expect(aiCostDashboardSource).not.toContain('No daily USD trend yet.');
    expect(aiCostDashboardSource).not.toContain('No daily token trend yet.');
    expect(aiCostDashboardSource).not.toContain('Loading usage…');
    expect(aiCostDashboardSource).not.toContain('const rangeButtonClass =');
    expect(aiCostPresentationSource).toContain('export function getAICostRangeButtonClass');
    expect(aiCostPresentationSource).toContain('export function getAICostLoadingState');
    expect(aiCostPresentationSource).toContain('export const AI_COST_EMPTY_STATE');
    expect(aiCostPresentationSource).toContain('export const AI_COST_DAILY_USD_EMPTY_STATE');
    expect(aiCostPresentationSource).toContain('export const AI_COST_DAILY_TOKEN_EMPTY_STATE');
    expect(rbacFeatureGateSectionSource).toContain('@/utils/upgradePresentation');
    expect(agentProfilesPanelStateSource).toContain('@/utils/upgradePresentation');
    expect(auditLogPanelSource).toContain('@/utils/upgradePresentation');
    expect(ssoProvidersPanelSource).toContain('@/utils/upgradePresentation');
    expect(auditWebhookPanelSource).toContain('@/utils/upgradePresentation');
    expect(upgradePresentationSource).toContain('export const UPGRADE_ACTION_LABEL');
    expect(upgradePresentationSource).toContain('export const UPGRADE_TRIAL_LABEL');
    expect(upgradePresentationSource).toContain('export const UPGRADE_TRIAL_LINK_CLASS');
    expect(upgradePresentationSource).toContain('export function getUpgradeActionButtonClass');
  });

  it('keeps the infrastructure workspace on router-derived view state', () => {
    expect(infrastructureWorkspaceSource).toContain('./infrastructureWorkspaceModel');
    expect(infrastructureWorkspaceSource).not.toContain(
      'createSignal<InfrastructureWorkspaceView>',
    );
    expect(infrastructureWorkspaceSource).not.toContain('setActiveView(');
    expect(infrastructureWorkspaceSource).not.toContain(
      "navigate('/settings/infrastructure/proxmox')",
    );
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function getInfrastructureWorkspaceViewFromPath',
    );
    expect(infrastructureWorkspaceModelSource).toContain(
      "pathname.startsWith('/settings/infrastructure/proxmox')",
    );
    expect(infrastructureWorkspaceModelSource).toContain(
      "path: '/settings/infrastructure/operations'",
    );
  });

  it('keeps websocket State contract resource-first without legacy per-type arrays', () => {
    expect(apiTypesSource).toContain('export interface State {');
    expect(apiTypesSource).toContain('resources: Resource[]');
    expect(apiTypesSource).not.toContain('nodes: Node[]');
    expect(apiTypesSource).not.toContain('vms: VM[]');
    expect(apiTypesSource).not.toContain('containers: Container[]');
    expect(apiTypesSource).not.toContain('dockerHosts: DockerHost[]');
    expect(apiTypesSource).not.toContain('hosts: Host[]');
    expect(apiTypesSource).not.toContain('storage: Storage[]');
    expect(apiTypesSource).not.toContain('pbs: PBSInstance[]');
    expect(apiTypesSource).not.toContain('pmg: PMGInstance[]');
    expect(apiTypesSource).not.toContain('replicationJobs: ReplicationJob[]');
    expect(apiTypesSource).toContain('export interface DockerRuntime');
    expect(apiTypesSource).not.toContain('export interface DockerHost');
  });

  it('keeps discovery resource types on canonical v6 names', () => {
    expect(discoveryTypesSource).not.toContain("| 'lxc'");
    expect(discoveryTypesSource).not.toContain("| 'docker_lxc'");
    expect(discoveryTargetUtilsSource).toContain('discoveryTarget.agentId');
    expect(unifiedResourcesHookSource).toContain(
      'const discoveryAgentId = v2.discoveryTarget?.agentId;',
    );
    expect(apiTypesSource).toContain("export type GuestType = 'vm' | 'system-container';");
    expect(apiTypesSource).not.toContain("'qemu'");
    expect(apiTypesSource).not.toContain("'lxc'");
    expect(commandPaletteSource).not.toContain("'lxc'");
    expect(organizationSharingCreateSectionSource).toContain('@/utils/canonicalResourceTypes');
    expect(canonicalResourceTypesSource).toContain('export const CANONICAL_RESOURCE_TYPES =');
    expect(canonicalResourceTypesSource).toContain("'system-container'");
    expect(canonicalResourceTypesSource).toContain("'app-container'");
    expect(canonicalResourceTypesSource).not.toContain("'container',");
  });

  it('keeps organization role vocabulary on shared utilities', () => {
    expect(organizationAccessPanelSource).toContain('./useOrganizationAccessPanelState');
    expect(organizationAccessPanelSource).toContain('./OrganizationAccessLoadingState');
    expect(organizationAccessPanelSource).toContain('./OrganizationAccessManagementSection');
    expect(organizationAccessPanelSource).toContain('./OrganizationAccessMembersSection');
    expect(organizationAccessManagementSectionSource).toContain(
      '@/utils/organizationRolePresentation',
    );
    expect(organizationAccessMembersSectionSource).toContain(
      '@/utils/organizationRolePresentation',
    );
    expect(organizationAccessManagementSectionSource).toContain(
      '@/utils/organizationSettingsPresentation',
    );
    expect(organizationAccessMembersSectionSource).toContain(
      '@/utils/organizationSettingsPresentation',
    );
    expect(organizationAccessManagementSectionSource).toContain('ORGANIZATION_MEMBER_ROLE_OPTIONS');
    expect(organizationAccessMembersSectionSource).toContain('ORGANIZATION_MEMBER_ROLE_OPTIONS');
    expect(organizationAccessMembersSectionSource).toContain('roleBadgeClass');
    expect(organizationAccessMembersSectionSource).toContain('formatOrgDate');
    expect(organizationAccessMembersSectionSource).toContain('getOrganizationAccessEmptyState');
    expect(organizationAccessLoadingStateSource).toContain('animate-pulse');
    expect(organizationAccessStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationAccessStateSource).toContain('OrgsAPI.updateMemberRole');
    expect(organizationAccessStateSource).toContain('OrgsAPI.inviteMember');
    expect(organizationAccessStateSource).toContain('OrgsAPI.removeMember');
    expect(organizationAccessStateSource).toContain('getOrganizationAccessRoleUpdatedMessage');
    expect(organizationAccessStateSource).toContain('getOrganizationAccessMemberAddedMessage');
    expect(organizationAccessStateSource).toContain('getOrganizationAccessMemberRemovedMessage');
    expect(organizationAccessPanelSource).not.toContain("{ value: 'viewer', label: 'Viewer' }");
    expect(organizationAccessStateSource).not.toContain(
      "notificationStore.error('User ID is required')",
    );
    expect(organizationAccessStateSource).not.toContain(
      "notificationStore.error(error instanceof Error ? error.message : 'Failed to add member')",
    );
    expect(organizationAccessStateSource).not.toContain(
      "notificationStore.error(error instanceof Error ? error.message : 'Failed to remove member')",
    );
    expect(organizationAccessStateSource).not.toContain('notificationStore.success(`Updated ');
    expect(organizationAccessStateSource).not.toContain('notificationStore.success(`Added ');
    expect(organizationAccessStateSource).not.toContain('notificationStore.success(`Removed ');
    expect(organizationOverviewPanelSource).toContain('./useOrganizationOverviewPanelState');
    expect(organizationOverviewPanelSource).toContain('./OrganizationOverviewLoadingState');
    expect(organizationOverviewPanelSource).toContain('./OrganizationOverviewDetailsSection');
    expect(organizationOverviewPanelSource).toContain('./OrganizationOverviewMembersSection');
    expect(organizationOverviewDetailsSectionSource).toContain(
      '@/utils/organizationSettingsPresentation',
    );
    expect(organizationOverviewMembersSectionSource).toContain(
      '@/utils/organizationSettingsPresentation',
    );
    expect(organizationOverviewDetailsSectionSource).toContain('formatOrgDate');
    expect(organizationOverviewMembersSectionSource).toContain('roleBadgeClass');
    expect(organizationOverviewMembersSectionSource).toContain('formatOrgDate');
    expect(organizationOverviewMembersSectionSource).toContain(
      'getOrganizationOverviewMembersEmptyState',
    );
    expect(organizationOverviewLoadingStateSource).toContain('animate-pulse');
    expect(organizationOverviewStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationOverviewStateSource).toContain('OrgsAPI.get');
    expect(organizationOverviewStateSource).toContain('OrgsAPI.listMembers');
    expect(organizationOverviewStateSource).toContain('OrgsAPI.update');
    expect(organizationOverviewStateSource).toContain('getOrganizationDisplayNameUpdatedMessage');
    expect(organizationOverviewStateSource).not.toContain(
      "notificationStore.error('Display name is required')",
    );
    expect(organizationOverviewStateSource).not.toContain(
      "error instanceof Error ? error.message : 'Failed to update organization name'",
    );
    expect(organizationOverviewStateSource).not.toContain(
      "notificationStore.success('Organization name updated')",
    );
    expect(organizationSharingCreateSectionSource).toContain(
      '@/utils/organizationRolePresentation',
    );
    expect(organizationSharingStateSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationSharingCreateSectionSource).toContain('ORGANIZATION_SHARE_ROLE_OPTIONS');
    expect(organizationOutgoingSharesSectionSource).toContain('normalizeOrganizationShareRole');
    expect(organizationIncomingSharesSectionSource).toContain('normalizeOrganizationShareRole');
    expect(organizationSharingCreateSectionSource).not.toContain(
      'const accessRoleOptions: Array<{ value: ShareAccessRole; label: string }> = [',
    );
    expect(organizationOutgoingSharesSectionSource).not.toContain('const normalizeShareRole =');
    expect(organizationSharingStateSource).not.toContain(
      "notificationStore.error('Target organization is required')",
    );
    expect(organizationSharingStateSource).not.toContain(
      "notificationStore.error('Target organization must differ from the current organization')",
    );
    expect(organizationSharingStateSource).not.toContain(
      "notificationStore.error('Valid resource type and resource ID are required')",
    );
    expect(organizationSharingStateSource).not.toContain(
      "notificationStore.success('Resource shared successfully')",
    );
    expect(organizationSharingStateSource).not.toContain(
      "notificationStore.success('Share removed')",
    );
    expect(organizationSharingPanelSource).toContain('./OrganizationSharingLoadingState');
    expect(organizationSharingPanelSource).toContain('./OrganizationSharingCreateSection');
    expect(organizationSharingPanelSource).toContain('./OrganizationOutgoingSharesSection');
    expect(organizationSharingPanelSource).toContain('./OrganizationIncomingSharesSection');
    expect(organizationSharingLoadingStateSource).toContain('animate-pulse');
    expect(organizationRolePresentationSource).toContain(
      'export const ORGANIZATION_MEMBER_ROLE_OPTIONS',
    );
    expect(organizationRolePresentationSource).toContain(
      'export const ORGANIZATION_SHARE_ROLE_OPTIONS',
    );
    expect(organizationRolePresentationSource).toContain(
      'export function normalizeOrganizationShareRole',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationAccessManageRequiredMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationAccessRoleUpdatedMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationAccessMemberAddedMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationAccessMemberRemovedMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationMemberUserIdRequiredMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationMemberRoleUpdateErrorMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationDisplayNameRequiredMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationDisplayNameUpdatedMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationOverviewManageRequiredMessage',
    );
  });

  it('keeps alerts agent thresholds sourced from unified agent resources', () => {
    expect(alertsPageSource).toContain(
      "import { AlertsConfigurationSurface } from '@/features/alerts/AlertsConfigurationSurface';",
    );
    expect(alertsPageSource).not.toContain('const agentResources = createMemo(');
    expect(alertsPageSource).not.toContain("resourceType: 'Agent Disk'");
    expect(alertsConfigurationSurfaceSource).toContain('<ThresholdsTab');
    expect(thresholdsTableStateSource).toContain(
      "import { useThresholdsData } from './useThresholdsData';",
    );
    expect(thresholdsDataHookSource).toContain('useThresholdsHostData');
    expect(thresholdsHostDataHookSource).toContain("resourceType: 'Agent Disk'");
    expect(thresholdsHostDataHookSource).not.toContain("resourceType: 'Host Disk'");
    expect(thresholdsHostDataHookSource).toContain('props.agentDefaults');
    expect(thresholdsTableAgentsResourcesSectionSource).toContain('disableAllAgents');
    expect(thresholdsTableSource).not.toContain('hostDefaults');
    expect(thresholdsTableSource).not.toContain('disableAllHosts');
  });

  it('keeps setup and node summary fallbacks aware of v6 agent facets', () => {
    expect(setupCompletionPanelSource).toContain('const hasAgentFacet = (resource: Resource)');
    expect(setupCompletionPanelSource).toContain("resource.type === 'agent'");
    expect(setupCompletionPanelSource).toContain('const agentFacetResources = resources.filter(');
    expect(setupCompletionPanelSource).toContain('Open Infrastructure Install');
    expect(setupCompletionPanelSource).toContain(
      'The canonical install flow now lives in Infrastructure Operations.',
    );
    expect(setupCompletionPanelSource).not.toContain('state.nodes || []');
    expect(setupCompletionPanelSource).not.toContain('state.hosts || []');
    expect(setupCompletionPanelSource).not.toContain(
      '(state.hosts || []).length > 0\n            ? state.hosts\n            : resources',
    );
    expect(infrastructureSelectorSource).toContain('useInfrastructureSelectorState');
    expect(infrastructureSelectorSource).not.toContain(
      'const hasAgentFacet = (resource: Resource): boolean =>',
    );
    expect(infrastructureSelectorSource).not.toContain("resource.type === 'truenas'");
    expect(infrastructureSelectorStateSource).toContain('useResources');
    expect(infrastructureSelectorModelSource).toContain('hasInfrastructureSelectorAgentFacet');
    expect(infrastructureSelectorModelSource).toContain("resource.type === 'truenas'");
    expect(infrastructureSelectorModelSource).not.toContain('if (hostLikeResources.length === 0');
    expect(guestRowCellsSource).toContain('getDashboardGuestBackupStatusPresentation');
    expect(guestRowCellsSource).toContain('getDashboardGuestBackupTooltip');
    expect(guestRowCellsSource).toContain('getDashboardGuestNetworkEmptyState');
    expect(guestRowSource).toContain('getDashboardGuestDiskStatusMessage');
    expect(guestRowSource).not.toContain('const BACKUP_STATUS_CONFIG: Record<');
    expect(guestRowSource).not.toContain('No backup found');
    expect(guestRowSource).not.toContain('No IP assigned');
    expect(guestRowSource).not.toContain(
      'No filesystems found. VM may be booting or using a Live ISO.',
    );
    expect(dashboardDiskListSource).toContain('./useDiskListState');
    expect(dashboardDiskListStateSource).toContain('getDashboardGuestDiskStatusMessage');
    expect(dashboardDiskListSource).not.toContain(
      'No filesystems found. VM may be booting or using a Live ISO.',
    );
    expect(dashboardGuestPresentationSource).toContain(
      'export function getDashboardGuestBackupStatusPresentation',
    );
    expect(dashboardGuestPresentationSource).toContain(
      'export function getDashboardGuestBackupTooltip',
    );
    expect(dashboardGuestPresentationSource).toContain(
      'export function getDashboardGuestNetworkEmptyState',
    );
    expect(dashboardGuestPresentationSource).toContain(
      'export function getDashboardGuestDiskStatusMessage',
    );
  });

  it('keeps discovery mapping on canonical agentId only', () => {
    expect(resourceDetailDiscoveryModelSource).toContain('const explicitDiscoveryAgentId =');
    expect(resourceDetailDiscoveryModelSource).toContain('agentId: explicitDiscoveryAgentId');
    expect(resourceDetailDiscoveryModelSource).not.toContain('hostId: explicitDiscoveryAgentId');
    expect(resourceDetailMappersSource).toContain('getActionableAgentIdFromResource(resource)');
    expect(unifiedResourcesHookSource).toContain(
      'const discoveryAgentId = v2.discoveryTarget?.agentId;',
    );
  });

  it('keeps AI chat mention resources aware of agent facets beyond host type', () => {
    expect(aiChatSource).toContain('const hasAgentFacet = (resource: Resource): boolean =>');
    expect(aiChatSource).toContain("resource.type === 'truenas'");
    expect(aiChatSource).not.toContain("resource.type === 'host'");
    expect(aiChatSource).toContain("type: 'agent'");
    expect(aiChatSource).not.toContain("mention.type === 'agent'");
    expect(aiChatSource).toContain('@/utils/aiControlLevelPresentation');
    expect(aiChatSource).toContain('AI_CHAT_SESSION_EMPTY_STATE');
    expect(aiChatSource).not.toContain('No previous conversations');
    expect(aiChatSource).toContain('getAIChatControlLevelPresentation');
    expect(aiChatSource).toContain('normalizeAIControlLevel');
    expect(aiChatSource).not.toContain('const normalizeControlLevel =');
    expect(aiChatSource).not.toContain('const labelForControlLevel =');
    expect(aiChatSource).not.toContain('const controlTone =');
    expect(aiChatSource).toContain('const mentionsForAPI =');
    expect(aiChatSource).toContain('? mentions.map((mention) => ({');
    expect(aiChatSource).toContain('name: mention.label,');
    expect(aiChatSource).toContain('type: mention.type,');
    expect(aiChatSource).not.toContain(
      'const mentionsForAPI = mentions.length > 0 ? mentions : undefined;',
    );
    expect(aiChatPresentationSource).toContain('export const AI_CHAT_SESSION_EMPTY_STATE');
    expect(mentionAutocompleteSource).toContain("| 'agent'");
    expect(mentionAutocompleteSource).toContain('getSimpleStatusIndicator');
    expect(mentionAutocompleteSource).toContain('<StatusDot');
    expect(mentionAutocompleteSource).not.toContain('const getStatusColor =');
    expect(mentionAutocompleteSource).toContain("| 'system-container'");
    expect(mentionAutocompleteSource).not.toContain("| 'container'");
    expect(mentionAutocompleteSource).not.toContain("| 'storage'");
    expect(mentionAutocompleteSource).not.toContain("| 'host'");
  });

  it('keeps infrastructure selectors on v6 agent-family resources', () => {
    expect(infrastructureSummarySource).not.toContain("resource.type === 'host'");
    expect(infrastructureSelectorComponentSource).not.toContain("resource.type === 'host'");
    expect(workloadsLinkSource).not.toContain("resource.type === 'host'");
    expect(unifiedResourceTableSource).not.toContain('buildMetricKeyForUnifiedResource');
    expect(unifiedResourceTableSource).not.toContain("buildMetricKey('host', resource.id)");
    expect(unifiedResourceHostTableCardSource).toContain('buildMetricKeyForUnifiedResource');
    expect(unifiedResourceHostTableCardSource).not.toContain("buildMetricKey('host', resource.id)");
    expect(resourceBadgesSource).not.toContain("host: 'Agent'");
    expect(problemResourcesTableSource).not.toContain("host: 'Agent'");
  });

  it('keeps dashboard and recovery type presentation on shared canonical helpers', () => {
    expect(problemResourcesTableSource).toContain('getResourceTypeLabel(pr.resource.type)');
    expect(problemResourcesTableSource).not.toContain('function formatType(');
    expect(recoveryTablePresentationSource).toContain(
      'getResourceTypePresentation(normalizeRecoverySubjectTypeKey(raw))',
    );
    expect(recoverySource).not.toContain('SUBJECT_TYPE_LABELS');
    expect(resourceBadgesSource).toContain('@/utils/resourceBadgePresentation');
    expect(resourceBadgePresentationSource).toContain('getResourceTypePresentation');
    expect(workloadTypeBadgesSource).toContain('getWorkloadTypePresentation');
    expect(workloadTypeBadgesSource).not.toContain('canonicalizeFrontendResourceType');
    expect(workloadTypePresentationSource).toContain('canonicalizeFrontendResourceType');
    expect(recoverySource).toContain('normalizeSourcePlatformQueryValue');
    expect(recoverySource).not.toContain('const normalizeProviderFromQuery');
    expect(sourcePlatformsSource).toContain('export const normalizeSourcePlatformQueryValue');
    expect(aiSettingsSource).toContain('getAISessionDiffStatusPresentation');
    expect(aiSettingsSource).not.toContain('const formatDiffStatus =');
    expect(aiSettingsSource).not.toContain('const diffStatusClasses =');
    expect(aiSessionDiffPresentationSource).toContain(
      'export function getAISessionDiffStatusPresentation',
    );
  });

  it('keeps resource picker reportable type policy on shared utilities', () => {
    expect(resourcePickerSource).toContain('@/utils/reportableResourceTypes');
    expect(resourcePickerSource).toContain('RESOURCE_PICKER_TYPE_FILTERS');
    expect(resourcePickerSource).toContain('getResourcePickerTypeFilterLabel');
    expect(resourcePickerSource).toContain('getResourcePickerEmptyState');
    expect(resourcePickerSource).toContain('getSimpleStatusIndicator');
    expect(resourcePickerSource).toContain('<StatusDot');
    expect(resourcePickerSource).not.toContain('const REPORTABLE_RESOURCE_TYPES =');
    expect(resourcePickerSource).not.toContain('const typeFilterLabels =');
    expect(resourcePickerSource).not.toContain('function getStatusColor(');
    expect(resourcePickerSource).not.toContain('function normalizeType(');
    expect(resourcePickerSource).not.toContain('No resources available');
    expect(resourcePickerSource).not.toContain(
      'Resources appear as Pulse collects infrastructure and workload metrics',
    );
    expect(resourcePickerSource).not.toContain('No resources match your filters');
    expect(reportableResourceTypesSource).toContain('export const REPORTABLE_RESOURCE_TYPES =');
    expect(reportableResourceTypesSource).toContain('export const RESOURCE_PICKER_TYPE_FILTERS');
    expect(reportableResourceTypesSource).toContain(
      'export const getResourcePickerTypeFilterLabel',
    );
    expect(reportableResourceTypesSource).toContain('export const getResourcePickerEmptyState');
    expect(reportableResourceTypesSource).toContain(
      'export const matchesReportableResourceTypeFilter =',
    );
  });

  it('keeps diagnostics alerts contract free of removed legacy threshold flags', () => {
    expect(diagnosticsPanelSource).not.toContain('legacyThresholdsDetected');
    expect(diagnosticsPanelSource).not.toContain('legacyThresholdSources');
    expect(diagnosticsPanelSource).not.toContain('legacyScheduleSettings');
    expect(systemSettingsStateSource).toContain('export function useSystemSettingsState(');
    expect(systemSettingsStateSource).not.toContain('legacyThresholdsDetected');
    expect(systemSettingsStateSource).not.toContain('legacyThresholdSources');
    expect(systemSettingsStateSource).not.toContain('legacyScheduleSettings');
  });

  it('keeps system settings polling presets and backup interval contracts on shared utilities', () => {
    expect(systemSettingsStateSource).toContain('@/utils/systemSettingsPresentation');
    expect(systemSettingsStateSource).toContain('getBackupIntervalSelectValue');
    expect(systemSettingsStateSource).toContain('getBackupIntervalSummary');
    expect(systemSettingsStateSource).toContain('getSystemSettingsSaveErrorMessage');
    expect(systemSettingsStateSource).toContain('getCheckForUpdatesErrorMessage');
    expect(systemSettingsStateSource).toContain('getStartUpdateErrorMessage');
    expect(systemSettingsStateSource).toContain('PVE_POLLING_PRESETS');
    expect(systemSettingsStateSource).not.toContain('const BACKUP_INTERVAL_OPTIONS =');
    expect(systemSettingsStateSource).not.toContain('const PVE_POLLING_PRESETS =');
    expect(systemSettingsStateSource).not.toContain(
      "notificationStore.error(error instanceof Error ? error.message : 'Failed to save settings')",
    );
    expect(systemSettingsStateSource).not.toContain(
      "notificationStore.error('Failed to check for updates')",
    );
    expect(systemSettingsStateSource).not.toContain(
      "notificationStore.error('Failed to start update. Please try again.')",
    );
    expect(generalSettingsPanelSource).toContain('@/utils/systemSettingsPresentation');
    expect(generalSettingsPanelSource).toContain('FilterButtonGroup');
    expect(generalSettingsPanelSource).toContain('variant="prominent"');
    expect(generalSettingsPanelSource).not.toContain('const PVE_POLLING_PRESETS =');
    expect(recoverySettingsPanelSource).toContain('@/utils/systemSettingsPresentation');
    expect(recoverySettingsPanelSource).not.toContain('const BACKUP_INTERVAL_OPTIONS =');
    expect(networkSettingsPanelSource).toContain('./NetworkDiscoverySection');
    expect(networkSettingsPanelSource).toContain('./NetworkBoundarySettingsSection');
    expect(networkDiscoverySectionSource).toContain('@/utils/systemSettingsPresentation');
    expect(networkDiscoverySectionSource).not.toContain('const COMMON_DISCOVERY_SUBNETS =');
    expect(networkBoundarySettingsSectionSource).not.toContain('const COMMON_DISCOVERY_SUBNETS =');
    expect(systemSettingsPresentationSource).toContain('export const PVE_POLLING_PRESETS');
    expect(systemSettingsPresentationSource).toContain('export const BACKUP_INTERVAL_OPTIONS');
    expect(systemSettingsPresentationSource).toContain('export const COMMON_DISCOVERY_SUBNETS');
    expect(systemSettingsPresentationSource).toContain(
      'export function getBackupIntervalSelectValue',
    );
    expect(systemSettingsPresentationSource).toContain('export function getBackupIntervalSummary');
    expect(systemSettingsPresentationSource).toContain(
      'export function getSystemSettingsSaveErrorMessage',
    );
    expect(systemSettingsPresentationSource).toContain(
      'export function getCheckForUpdatesErrorMessage',
    );
    expect(systemSettingsPresentationSource).toContain(
      'export function getStartUpdateErrorMessage',
    );
  });

  it('keeps SSO provider presentation on shared utilities', () => {
    expect(ssoProvidersPanelSource).toContain('@/utils/ssoProviderPresentation');
    expect(ssoProvidersPanelSource).toContain('@/components/Settings/useSSOProvidersState');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderTypeLabel');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderSummary');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderCardClass');
    expect(ssoProvidersPanelSource).toContain('getSSOCertificatePresentation');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderAddButtonLabel');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderModalTitle');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderEmptyStateTitle');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderEmptyStateDescription');
    expect(ssoProvidersPanelSource).toContain('getSSOProvidersLoadingState');
    expect(ssoProvidersPanelSource).toContain('SSOProviderTypeIcon');
    expect(ssoProvidersPanelSource).not.toContain('No SSO providers configured');
    expect(ssoProvidersPanelSource).not.toContain('Loading SSO providers...');
    expect(ssoProvidersPanelSource).not.toContain(
      "notificationStore.error('Failed to load SSO providers')",
    );
    expect(ssoProvidersPanelSource).not.toContain(
      "notificationStore.error('Failed to load provider details')",
    );
    expect(ssoProvidersPanelSource).not.toContain("notificationStore.success('Provider deleted')");
    expect(ssoProvidersPanelSource).not.toContain(
      "notificationStore.error('Failed to delete provider')",
    );
    expect(ssoProvidersPanelSource).not.toContain(
      "notificationStore.success('Connection test successful')",
    );
    expect(ssoProvidersPanelSource).not.toContain(
      "notificationStore.error('Failed to test connection')",
    );
    expect(ssoProvidersPanelSource).not.toContain(
      "notificationStore.error('Please enter an IdP Metadata URL')",
    );
    expect(ssoProvidersPanelSource).not.toContain('const loadProviders = async () =>');
    expect(ssoProvidersPanelSource).not.toContain('const handleSave = async (');
    expect(ssoProvidersPanelSource).not.toContain('const testConnection = async () =>');
    expect(ssoProvidersPanelSource).not.toContain('provider.type.toUpperCase()');
    expect(ssoProvidersPanelSource).not.toContain("provider.type === 'oidc' ? (");
    expect(ssoProvidersPanelSource).not.toContain(
      "testResult()?.success\n                      ? 'bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800'",
    );
    expect(ssoProvidersPanelSource).not.toContain(
      "cert.isExpired ? 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300' : 'bg-surface-hover text-base-content'",
    );
    expect(ssoProviderPresentationSource).toContain('export function getSSOProviderTypeLabel');
    expect(ssoProviderPresentationSource).toContain('export function getSSOProviderSummary');
    expect(ssoProviderPresentationSource).toContain('export function getSSOProviderCardClass');
    expect(ssoProviderPresentationSource).toContain('export function getSSOProviderAddButtonLabel');
    expect(ssoProviderPresentationSource).toContain('export function getSSOProviderModalTitle');
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderEmptyStateTitle',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderEmptyStateDescription',
    );
    expect(ssoProviderPresentationSource).toContain('export function getSSOProvidersLoadingState');
    expect(ssoProviderPresentationSource).toContain('export function getSSOTestResultPresentation');
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOCertificatePresentation',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProvidersLoadErrorMessage',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderDetailsLoadErrorMessage',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderSaveSuccessMessage',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderDeleteSuccessMessage',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOConnectionTestSuccessMessage',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOMetadataUrlRequiredMessage',
    );
    expect(ssoProvidersStateSource).toContain('@/components/Settings/ssoProvidersModel');
    expect(ssoProvidersStateSource).toContain('getSSOTestResultPresentation');
    expect(ssoProvidersStateSource).toContain('getSSOProvidersLoadErrorMessage');
    expect(ssoProvidersStateSource).toContain('getSSOProviderDetailsLoadErrorMessage');
    expect(ssoProvidersStateSource).toContain('getSSOProviderSaveSuccessMessage');
    expect(ssoProvidersStateSource).toContain('getSSOProviderDeleteSuccessMessage');
    expect(ssoProvidersStateSource).toContain('getSSOConnectionTestSuccessMessage');
    expect(ssoProvidersModelSource).toContain('export const createEmptyProviderForm =');
    expect(ssoProvidersModelSource).toContain('export const mapProviderDetailsToForm =');
    expect(ssoProvidersModelSource).toContain('export const buildProviderPayload =');
    expect(ssoProvidersModelSource).toContain('export const buildProviderTestPayload =');
  });

  it('keeps RBAC permission vocabulary on shared utilities', () => {
    expect(rolesPanelSource).toContain('@/utils/rbacPresentation');
    expect(rolesPanelSource).toContain('./RBACFeatureGateSection');
    expect(rolesPanelSource).toContain('./RolesEditorDialog');
    expect(rolesPanelSource).toContain('./useRolesPanelState');
    expect(rbacFeatureGateStateSource).toContain('getRBACFeatureGateCopy');
    expect(rolesPanelSource).toContain('getRolesEmptyState');
    expect(rolesPanelSource).not.toContain(
      "const ACTIONS = ['read', 'write', 'delete', 'admin', '*']",
    );
    expect(rolesPanelSource).not.toContain(
      "const RESOURCES = ['settings', 'audit_logs', 'nodes', 'users', 'license', '*']",
    );
    expect(rolesPanelSource).not.toContain('Custom Roles (Pro)');
    expect(rolesPanelSource).not.toContain('No roles available.');
    expect(rbacPermissionsSource).toContain('export const RBAC_PERMISSION_ACTIONS');
    expect(rbacPermissionsSource).toContain('export const RBAC_PERMISSION_RESOURCES');
    expect(rbacPermissionsSource).toContain('export function createDefaultRBACPermission');
    expect(rolesPanelStateSource).toContain('@/utils/rbacPermissions');
    expect(rbacPresentationSource).toContain('export function getRBACFeatureGateCopy');
    expect(rbacPresentationSource).toContain('export function getRolesEmptyState');
    expect(rbacPresentationSource).toContain('export function getRolesLoadErrorMessage');
    expect(rbacPresentationSource).toContain('export function getRolesSaveErrorMessage');
    expect(rbacPresentationSource).toContain('export function getUserAssignmentsLoadErrorMessage');
    expect(rbacPresentationSource).toContain('export function getUserAssignmentsEmptyStateCopy');
    expect(userAssignmentsPanelSource).toContain('@/utils/rbacPresentation');
    expect(userAssignmentsPanelSource).toContain('./RBACFeatureGateSection');
    expect(userAssignmentsPanelSource).toContain('./UserAssignmentsDialog');
    expect(userAssignmentsPanelSource).toContain('./useUserAssignmentsPanelState');
    expect(rbacFeatureGateStateSource).toContain('getRBACFeatureGateCopy');
    expect(userAssignmentsPanelSource).toContain('getUserAssignmentsEmptyStateCopy');
    expect(userAssignmentsPanelStateSource).toContain('getUserAssignmentsLoadErrorMessage');
    expect(userAssignmentsPanelStateSource).toContain('getUserAssignmentsUpdateErrorMessage');
    expect(rolesPanelStateSource).toContain('getRolesLoadErrorMessage');
    expect(rolesPanelStateSource).toContain('getRolesRequiredFieldsMessage');
    expect(rolesEditorDialogSource).toContain('RBAC_PERMISSION_ACTIONS');
    expect(rolesEditorDialogSource).toContain('RBAC_PERMISSION_RESOURCES');
    expect(rbacFeatureGateSectionSource).toContain('trackUpgradeClicked');
    expect(rbacFeatureGateStateSource).toContain('trackPaywallViewed');
    expect(rbacFeatureGateStateSource).toContain('runStartProTrialAction({');
    expect(rbacFeatureGateStateSource).not.toContain('startProTrial()');
    expect(userAssignmentsDialogSource).toContain('Effective Permissions Preview');
    expect(userAssignmentsPanelStateSource).toContain('RBACAPI.getUsers');
    expect(userAssignmentsPanelStateSource).toContain('RBACAPI.updateUserRoles');
    expect(rolesPanelSource).not.toContain("notificationStore.error('Failed to load roles')");
    expect(rolesPanelSource).not.toContain("notificationStore.error('Failed to save role')");
    expect(userAssignmentsPanelSource).not.toContain(
      "notificationStore.error('Failed to load user assignments')",
    );
    expect(userAssignmentsPanelSource).not.toContain(
      "notificationStore.error('Failed to update user roles')",
    );
    expect(userAssignmentsPanelSource).not.toContain('Centralized Access Control (Pro)');
    expect(userAssignmentsPanelSource).not.toContain('No users yet');
    expect(userAssignmentsPanelSource).not.toContain('Configure SSO in Security settings');
    expect(userAssignmentsPanelSource).not.toContain('Users sync on first login');
  });

  it('keeps alert thresholds routes on v6 container path only', () => {
    expect(thresholdsTableStateSource).toContain(
      "if (path.includes('/thresholds/containers')) return 'docker';",
    );
    expect(thresholdsTableStateSource).toContain('/thresholds/agents');
    expect(thresholdsHostDataHookSource).toContain("resourceType: 'Agent Disk'");
    expect(thresholdsTableStateSource).toContain('getAlertThresholdsSectionTitles');
    expect(thresholdsTableAgentDisksSectionSource).toContain(
      'title={state.sectionTitles.agentDisks}',
    );
    expect(thresholdsHostDataHookSource).not.toContain("resourceType: 'Host Disk'");
    expect(thresholdsHostDataHookSource).toContain('props.agentDefaults');
    expect(thresholdsTableStateSource).not.toContain('props.hostDefaults');
    expect(thresholdsTableStateSource).not.toContain('timeThresholds().host');
    expect(thresholdsTableStateSource).not.toContain('/thresholds/docker');
    expect(thresholdsTableStateSource).toContain('PBS_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('GUEST_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('NODE_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('PBS_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('GUEST_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('GUEST_FILTERING_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('BACKUP_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('SNAPSHOT_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('STORAGE_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('STORAGE_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('PMG_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('PMG_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('AGENT_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('AGENT_DISKS_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('AGENT_DISKS_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('CONTAINER_RUNTIMES_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('CONTAINERS_FILTER_EMPTY_STATE');
    expect(thresholdsTableStateSource).toContain('getAlertThresholdsGuestFilterPresentation');
    expect(thresholdsTableStateSource).toContain('getAlertThresholdsBackupOrphanedPresentation');
    expect(thresholdsTableStateSource).toContain(
      'getAlertThresholdsDockerIgnoredPrefixesPresentation',
    );
    expect(thresholdsTableSource).not.toContain('No PBS servers configured.');
    expect(thresholdsTableSource).not.toContain('No VMs or containers found.');
    expect(thresholdsTableSource).not.toContain('No nodes match the current filters.');
    expect(thresholdsTableSource).not.toContain('No PBS servers match the current filters.');
    expect(thresholdsTableSource).not.toContain('No VMs or containers match the current filters.');
    expect(thresholdsTableSource).not.toContain('Configure guest filtering rules.');
    expect(thresholdsTableSource).not.toContain('Configure recovery alert thresholds.');
    expect(thresholdsTableSource).not.toContain('Configure snapshot age thresholds.');
    expect(thresholdsTableSource).not.toContain('No storage devices found.');
    expect(thresholdsTableSource).not.toContain('No storage devices match the current filters.');
    expect(thresholdsTableSource).not.toContain('No mail gateways match the current filters.');
    expect(thresholdsTableSource).not.toContain('No agents match the current filters.');
    expect(thresholdsTableSource).not.toContain(
      'No agent disks found. Agents with mounted filesystems will appear here.',
    );
    expect(thresholdsTableSource).not.toContain('No agent disks match the current filters.');
    expect(thresholdsTableSource).not.toContain('No container runtimes match the current filters.');
    expect(thresholdsTableSource).not.toContain('No containers match the current filters.');
    expect(thresholdsTableSource).not.toContain('Skip metrics for guests starting with:');
    expect(thresholdsTableSource).not.toContain(
      'Only monitor guests with at least one of these tags (leave empty to disable whitelist):',
    );
    expect(thresholdsTableSource).not.toContain('Ignore guests with any of these tags:');
    expect(thresholdsTableSource).not.toContain(
      'Alert when backups exist for VMIDs that are no longer in inventory.',
    );
    expect(thresholdsTableSource).not.toContain('Toggle orphaned VM/Container backup alerts');
    expect(thresholdsTableSource).not.toContain(
      'Containers whose name or ID starts with any prefix below are skipped for container alerts. Enter one prefix per line; matching is case-insensitive.',
    );
    expect(thresholdsTableSource).not.toContain('runner-');
    expect(thresholdsTableSource).not.toContain('100, 200, 10*');
    expect(thresholdsTableSource).not.toContain('Search resources...');
    expect(thresholdsTableSource).not.toContain('Dismiss tips');
    expect(thresholdsTableSource).not.toContain('Quick tips:');
    expect(thresholdsTableSource).not.toContain('Swarm service alerts');
    expect(thresholdsTableSource).not.toContain('Toggle Swarm service replica monitoring');
    expect(thresholdsTableSource).not.toContain('Warning gap %');
    expect(thresholdsTableSource).not.toContain(
      'Convert to warning when at least this percentage of replicas are missing.',
    );
    expect(thresholdsTableSource).not.toContain('Critical gap %');
    expect(thresholdsTableSource).not.toContain(
      'Raise a critical alert when the missing replica gap meets or exceeds this value.',
    );
    expect(thresholdsTableSource).not.toContain(
      'Critical gap must be greater than or equal to the warning gap when enabled.',
    );
    expect(thresholdsTableSource).not.toContain('title="Proxmox Nodes"');
    expect(thresholdsTableSource).not.toContain('title="PBS Servers"');
    expect(thresholdsTableSource).not.toContain('title="VMs & Containers"');
    expect(thresholdsTableSource).not.toContain('title="Guest Filtering"');
    expect(thresholdsTableSource).not.toContain('title="Recovery"');
    expect(thresholdsTableSource).not.toContain('title="Snapshot Age"');
    expect(thresholdsTableSource).not.toContain('title="Storage Devices"');
    expect(thresholdsTableSource).not.toContain('title="Mail Gateway Thresholds"');
    expect(thresholdsTableSource).not.toContain('title="Agents"');
    expect(thresholdsTableSource).not.toContain('title="Agent Disks"');
    expect(thresholdsTableSource).not.toContain('title="Container Runtimes"');
    expect(thresholdsTableSource).not.toContain('title="Containers"');
    expect(alertThresholdsPresentationSource).toContain('export const PBS_THRESHOLDS_EMPTY_STATE');
    expect(alertThresholdsPresentationSource).toContain(
      'export const GUEST_THRESHOLDS_EMPTY_STATE',
    );
    expect(alertThresholdsPresentationSource).toContain(
      'export const NODE_THRESHOLDS_FILTER_EMPTY_STATE',
    );
    expect(alertThresholdsPresentationSource).toContain('export const PMG_THRESHOLDS_EMPTY_STATE');
    expect(alertThresholdsPresentationSource).toContain(
      'export const ALERT_THRESHOLDS_SEARCH_PLACEHOLDER',
    );
    expect(alertThresholdsPresentationSource).toContain(
      'export function getAlertThresholdsHelpBanner',
    );
    expect(alertThresholdsPresentationSource).toContain(
      'export function getAlertThresholdsGuestFilterPresentation',
    );
    expect(alertThresholdsPresentationSource).toContain(
      'export function getAlertThresholdsBackupOrphanedPresentation',
    );
    expect(alertThresholdsPresentationSource).toContain(
      'export function getAlertThresholdsDockerIgnoredPrefixesPresentation',
    );
    expect(alertThresholdsPresentationSource).toContain(
      'export function getAlertThresholdsDockerServicePresentation',
    );
    expect(alertThresholdsPresentationSource).toContain(
      'export function getAlertThresholdsSectionTitles',
    );
    expect(collapsibleSectionSource).toContain('getAlertThresholdsSectionDisabledLabel');
    expect(collapsibleSectionSource).toContain('getAlertThresholdsSectionUnsavedChangesTitle');
    expect(collapsibleSectionSource).not.toContain('>Disabled<');
    expect(collapsibleSectionSource).not.toContain('title="Unsaved changes"');
    expect(alertThresholdsSectionPresentationSource).toContain(
      'export function getAlertThresholdsSectionDisabledLabel',
    );
    expect(alertThresholdsSectionPresentationSource).toContain(
      'export function getAlertThresholdsSectionUnsavedChangesTitle',
    );
  });

  it('keeps workloads routing on canonical v6 agent query param only', () => {
    expect(resourceLinksSource).toContain("agent: 'agent'");
    expect(resourceLinksSource).not.toContain('LEGACY_WORKLOADS_HOST_QUERY_PARAM');
    expect(resourceLinksSource).not.toContain('canonicalHost || legacyHost');
  });

  it('keeps alerts API adapter fully canonical v6', () => {
    expect(alertsApiSource).not.toContain('normalizeAlertConfigFromAPI');
    expect(alertsApiSource).not.toContain('serializeAlertConfigForAPI');
    expect(alertsApiSource).not.toContain('hostDefaults');
    expect(alertsApiSource).not.toContain('disableAllHosts');
    expect(alertsApiSource).not.toContain('timeThresholds.host');
    expect(alertsApiSource).toContain('body: JSON.stringify(config)');
  });

  it('keeps license API free of removed legacy exchange surface', () => {
    expect(licenseApiSource).not.toContain('exchangeLicense(');
    expect(licenseApiSource).not.toContain('/exchange');
    expect(licenseApiSource).not.toContain('existing_jwt');
  });

  it('keeps monitoring API container-runtime errors on v6 naming', () => {
    expect(monitoringApiSource).toContain('Container runtime not found');
    expect(monitoringApiSource).not.toContain('Docker host not found');
    expect(apiTypesSource).toContain('export interface DockerRuntimeCommand');
    expect(apiTypesSource).not.toContain('export interface DockerHostCommand');
    expect(monitoringApiSource).toContain('deleteDockerRuntime');
    expect(monitoringApiSource).toContain('allowDockerRuntimeReenroll');
    expect(monitoringApiSource).not.toContain('deleteDockerHost');
    expect(monitoringApiSource).not.toContain('allowDockerHostReenroll');
    expect(monitoringApiSource).toContain(
      'body: JSON.stringify({ agentId, containerId, containerName })',
    );
    expect(monitoringApiSource).not.toContain(
      'body: JSON.stringify({ hostId, containerId, containerName })',
    );
    expect(monitoringApiSource).toContain('agentId?: string;');
    expect(monitoringApiSource).not.toContain('hostId?: string;');
    expect(monitoringApiSource).toContain('/agents/docker/runtimes/');
    expect(monitoringApiSource).not.toContain('/agents/docker/hosts/');
  });

  it('keeps container update actions wired to agentId naming only', () => {
    expect(containerUpdatesSource).toContain('export function syncWithAgentCommand');
    expect(containerUpdatesSource).not.toContain('export const syncWithHostCommand');
    expect(websocketStoreSource).toContain('syncWithAgentCommand(agentId, command as any)');
    expect(websocketStoreSource).not.toContain('syncWithHostCommand(hostId, command as any)');
    expect(websocketStoreSource).toContain('markDockerRuntimesTokenRevoked');
    expect(websocketStoreSource).toContain("markTokenRevoked('dockerRuntimes', tokenId, agentIds)");
    expect(guestRowSource).toContain('agentId={dockerHostId()}');
    expect(guestRowStateSource).toContain('getWorkloadDockerHostId(props.guest)');
    expect(guestRowSource).not.toContain('hostId={dockerHostId()}');
    expect(guestRowStateSource).not.toContain('getWorkloadDockerServerId');
    expect(guestDrawerSource).toContain('agentId={discoveryAgentId()}');
    expect(guestDrawerSource).not.toContain('hostId={discoveryHostId()}');
  });

  it('keeps chart API contract on canonical agentData naming', () => {
    expect(chartsApiSource).toContain('agentData?: Record<string, ChartData>');
    expect(chartsApiSource).not.toContain('hostData?: Record<string, ChartData>');
    expect(chartsApiSource).toContain('agents?: number;');
    expect(chartsApiSource).not.toContain('hosts?: number;');
    expect(chartsApiSource).toContain("| 'system-container'");
    expect(chartsApiSource).toContain("| 'app-container'");
    expect(chartsApiSource).not.toContain("| 'container'");
    expect(chartsApiSource).toContain("| 'docker-host'");
    expect(chartsApiSource).not.toContain("| 'dockerHost'");
    expect(chartsApiSource).not.toContain("| 'guest'");
    expect(chartsApiSource).not.toContain("| 'docker'");
    expect(resourcePickerSource).toContain('@/utils/reportableResourceTypes');
    expect(reportableResourceTypesSource).toContain("return 'docker-host';");
    expect(reportableResourceTypesSource).toContain("return 'app-container';");
    expect(reportableResourceTypesSource).not.toContain("return 'dockerHost';");
    expect(reportableResourceTypesSource).not.toContain("return 'dockerContainer';");
    expect(reportableResourceTypesSource).not.toContain("return 'container';");
  });

  it('keeps login and SSO settings on provider-based flows only', () => {
    expect(loginSource).not.toContain('Legacy OIDC fallback');
    expect(loginSource).not.toContain('startOidcLogin');
    expect(settingsSource).not.toContain('<OIDCPanel');
    expect(settingsSource).toContain('getSettingsLoadingState');
    expect(settingsSource).not.toContain('Loading settings...');
    expect(settingsSystemPanelsSource).toContain('getSettingsConfigurationLoadingState');
    expect(settingsSystemPanelsSource).not.toContain('Loading configuration...');
    expect(settingsPanelRegistrySource).toContain('createSettingsPanelRegistry');
    expect(settingsPanelRegistrySource).toContain('buildSettingsPanelRegistryContext(params)');
    expect(settingsPanelRegistrySource).not.toContain('getSettingsConfigurationLoadingState');
    expect(settingsShellPresentationSource).toContain('export function getSettingsLoadingState');
    expect(settingsShellPresentationSource).toContain(
      'export function getSettingsConfigurationLoadingState',
    );
    expect(settingsSource).not.toContain(
      "stateHosts={(state.resources ?? []).filter((r) => r.type === 'host')}",
    );
    expect(infrastructureDiscoveryRuntimeStateSource).toContain(
      '@/utils/infrastructureSettingsPresentation',
    );
    expect(infrastructureDiscoveryRuntimeStateSource).toContain(
      'getDiscoveryScanStartErrorMessage',
    );
    expect(infrastructureDiscoveryRuntimeStateSource).toContain(
      'getDiscoverySettingUpdateErrorMessage',
    );
    expect(infrastructureDiscoveryRuntimeStateSource).toContain(
      'getDiscoverySubnetUpdateErrorMessage',
    );
    expect(infrastructureConfiguredNodesStateSource).toContain(
      'getNodeTemperatureMonitoringUpdateErrorMessage',
    );
    expect(infrastructureDiscoveryRuntimeStateSource).not.toContain(
      "notificationStore.error('Failed to start discovery scan')",
    );
    expect(infrastructureDiscoveryRuntimeStateSource).not.toContain(
      "notificationStore.error('Failed to update discovery setting')",
    );
    expect(infrastructureDiscoveryRuntimeStateSource).not.toContain(
      "notificationStore.error('Failed to update discovery subnet')",
    );
    expect(infrastructureConfiguredNodesStateSource).not.toContain(
      "notificationStore.error(error instanceof Error ? error.message : 'Failed to update temperature monitoring setting')",
    );
    expect(infrastructureConfiguredNodesStateSource).not.toContain(
      "notificationStore.error(error instanceof Error ? error.message : 'Failed to delete node')",
    );
    expect(infrastructureSettingsStateSource).not.toContain("byType('host')");
    expect(appSource).not.toContain('oidcEnabled');
    expect(appSource).not.toContain('oidcUsername');
  });

  it('keeps AI settings/types free of removed legacy autonomous and schedule fields', () => {
    expect(aiTypesSource).not.toContain('autonomous_mode');
    expect(aiTypesSource).not.toContain('patrol_schedule_preset');
    expect(aiSettingsSource).not.toContain('settings_ai_autonomous_mode');
  });

  it('keeps unified resource typing normalized to agent (no legacy host type)', () => {
    expect(resourceTypesSource).not.toContain("| 'host' // Standalone host (via agent)");
    expect(unifiedResourcesHookSource).not.toContain("case 'host':");
    expect(unifiedResourcesHookSource).toContain("case 'agent':");
    expect(unifiedResourcesHookSource).toContain("return 'agent';");
    expect(unifiedResourcesHookSource).not.toContain("return 'host';");
  });

  it('keeps node link IDs on canonical linkedAgentId only', () => {
    expect(apiTypesSource).toContain('linkedAgentId?: string');
    expect(apiTypesSource).not.toContain('linkedHostAgentId');
    expect(resourceStateAdaptersSource).toContain('const linkedAgentId =');
    expect(resourceStateAdaptersSource).toContain(
      'asString(platform?.linkedAgentId) || getActionableAgentIdFromResource(resource)',
    );
    expect(resourceStateAdaptersSource).not.toContain('linkedHostAgentId');
  });
});
