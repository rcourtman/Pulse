import { describe, expect, it } from 'vitest';
import agentProfilesPanelSource from '../AgentProfilesPanel.tsx?raw';
import apiTokenManagerSource from '../APITokenManager.tsx?raw';
import unifiedAgentsSource from '../UnifiedAgents.tsx?raw';
import agentLedgerPanelSource from '../AgentLedgerPanel.tsx?raw';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import completeStepSource from '@/components/SetupWizard/steps/CompleteStep.tsx?raw';
import infrastructureSelectorSource from '@/components/shared/InfrastructureSelector.tsx?raw';
import aiChatSource from '@/components/AI/Chat/index.tsx?raw';
import aiChatPresentationSource from '@/utils/aiChatPresentation.ts?raw';
import diagnosticsPanelSource from '../DiagnosticsPanel.tsx?raw';
import auditLogPanelSource from '../AuditLogPanel.tsx?raw';
import auditWebhookPanelSource from '../AuditWebhookPanel.tsx?raw';
import systemSettingsStateSource from '../useSystemSettingsState.ts?raw';
import infrastructureSettingsStateSource from '../useInfrastructureSettingsState.ts?raw';
import settingsPanelRegistrySource from '../useSettingsPanelRegistry.tsx?raw';
import infrastructureSummarySource from '@/components/Infrastructure/InfrastructureSummary.tsx?raw';
import infrastructureSelectorComponentSource from '@/components/shared/InfrastructureSelector.tsx?raw';
import workloadsLinkSource from '@/components/Infrastructure/workloadsLink.ts?raw';
import unifiedResourceTableSource from '@/components/Infrastructure/UnifiedResourceTable.tsx?raw';
import thresholdsTableSource from '@/components/Alerts/ThresholdsTable.tsx?raw';
import collapsibleSectionSource from '@/components/Alerts/Thresholds/sections/CollapsibleSection.tsx?raw';
import alertThresholdsPresentationSource from '@/utils/alertThresholdsPresentation.ts?raw';
import alertThresholdsSectionPresentationSource from '@/utils/alertThresholdsSectionPresentation.ts?raw';
import loginSource from '@/components/Login.tsx?raw';
import settingsSource from '../Settings.tsx?raw';
import aiSettingsSource from '../AISettings.tsx?raw';
import reportingPanelSource from '../ReportingPanel.tsx?raw';
import suggestProfileModalSource from '../SuggestProfileModal.tsx?raw';
import aiIntelligenceSource from '@/pages/AIIntelligence.tsx?raw';
import aiPatrolSchedulePresentationSource from '@/utils/aiPatrolSchedulePresentation.ts?raw';
import patrolSummaryPresentationSource from '@/utils/patrolSummaryPresentation.ts?raw';
import aiCostDashboardSource from '@/components/AI/AICostDashboard.tsx?raw';
import aiCostPresentationSource from '@/utils/aiCostPresentation.ts?raw';
import aiControlLevelPresentationSource from '@/utils/aiControlLevelPresentation.ts?raw';
import agentProfilesPresentationSource from '@/utils/agentProfilesPresentation.ts?raw';
import agentProfileSuggestionPresentationSource from '@/utils/agentProfileSuggestionPresentation.ts?raw';
import reportingPresentationSource from '@/utils/reportingPresentation.ts?raw';
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
import organizationOverviewPanelSource from '../OrganizationOverviewPanel.tsx?raw';
import organizationSharingPanelSource from '../OrganizationSharingPanel.tsx?raw';
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
import resourceBadgesSource from '@/components/Infrastructure/resourceBadges.ts?raw';
import resourceBadgePresentationSource from '@/utils/resourceBadgePresentation.ts?raw';
import recoverySource from '@/components/Recovery/Recovery.tsx?raw';
import recoveryTablePresentationSource from '@/utils/recoveryTablePresentation.ts?raw';
import problemResourcesTableSource from '@/pages/DashboardPanels/ProblemResourcesTable.tsx?raw';
import workloadTypeBadgesSource from '@/components/shared/workloadTypeBadges.ts?raw';
import workloadTypePresentationSource from '@/utils/workloadTypePresentation.ts?raw';
import dashboardGuestPresentationSource from '@/utils/dashboardGuestPresentation.ts?raw';
import containerUpdatesSource from '@/stores/containerUpdates.ts?raw';
import websocketStoreSource from '@/stores/websocket.ts?raw';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import dashboardDiskListSource from '@/components/Dashboard/DiskList.tsx?raw';
import guestDrawerSource from '@/components/Dashboard/GuestDrawer.tsx?raw';
import resourcePickerSource from '../ResourcePicker.tsx?raw';
import reportableResourceTypesSource from '@/utils/reportableResourceTypes.ts?raw';
import sourcePlatformsSource from '@/utils/sourcePlatforms.ts?raw';
import agentCapabilityPresentationSource from '@/utils/agentCapabilityPresentation.ts?raw';
import statusUtilsSource from '@/utils/status.ts?raw';
import unifiedAgentStatusPresentationSource from '@/utils/unifiedAgentStatusPresentation.ts?raw';
import unifiedAgentInventoryPresentationSource from '@/utils/unifiedAgentInventoryPresentation.ts?raw';
import relayPresentationSource from '@/utils/relayPresentation.ts?raw';
import relaySettingsPanelSource from '../RelaySettingsPanel.tsx?raw';
import proxmoxSettingsPanelSource from '../ProxmoxSettingsPanel.tsx?raw';
import relayOnboardingCardSource from '@/components/Dashboard/RelayOnboardingCard.tsx?raw';
import generalSettingsPanelSource from '../GeneralSettingsPanel.tsx?raw';
import networkSettingsPanelSource from '../NetworkSettingsPanel.tsx?raw';
import recoverySettingsPanelSource from '../RecoverySettingsPanel.tsx?raw';
import rolesPanelSource from '../RolesPanel.tsx?raw';
import userAssignmentsPanelSource from '../UserAssignmentsPanel.tsx?raw';
import ssoProvidersPanelSource from '../SSOProvidersPanel.tsx?raw';
import rbacPermissionsSource from '@/utils/rbacPermissions.ts?raw';
import rbacPresentationSource from '@/utils/rbacPresentation.ts?raw';
import apiTokenPresentationSource from '@/utils/apiTokenPresentation.ts?raw';
import ssoProviderPresentationSource from '@/utils/ssoProviderPresentation.ts?raw';
import systemSettingsPresentationSource from '@/utils/systemSettingsPresentation.ts?raw';

describe('agent model guardrails', () => {
  it('keeps AgentProfilesPanel on unified resources (not host-only slices)', () => {
    expect(agentProfilesPanelSource).toContain('const { resources } = useResources()');
    expect(agentProfilesPanelSource).toContain('@/utils/agentProfilesPresentation');
    expect(agentProfilesPanelSource).toContain('getAgentProfilesEmptyState');
    expect(agentProfilesPanelSource).toContain('getAgentProfileAssignmentsEmptyState');
    expect(agentProfilesPanelSource).not.toContain("const hosts = byType('host')");
    expect(agentProfilesPanelSource).not.toContain('No profiles yet. Create one to get started.');
    expect(agentProfilesPanelSource).not.toContain(
      'No agents connected. Install an agent to assign profiles.',
    );
    expect(agentProfilesPresentationSource).toContain(
      'export function getAgentProfilesEmptyState',
    );
    expect(agentProfilesPresentationSource).toContain(
      'export function getAgentProfileAssignmentsEmptyState',
    );
    expect(agentLedgerPanelSource).toContain('getSimpleStatusIndicator');
    expect(agentLedgerPanelSource).toContain('getAgentLedgerLoadingState');
    expect(agentLedgerPanelSource).toContain('getAgentLedgerErrorState');
    expect(agentLedgerPanelSource).not.toContain('Loading agent ledger...');
    expect(agentLedgerPanelSource).not.toContain('Failed to load agent ledger.');
    expect(agentLedgerPanelSource).not.toContain('function statusVariant(');
  });

  it('keeps APITokenManager runtime/agent usage mapped from unified resources', () => {
    expect(apiTokenManagerSource).toContain('const agentCapableResources = createMemo(() =>');
    expect(apiTokenManagerSource).toContain('@/utils/apiTokenPresentation');
    expect(apiTokenManagerSource).toContain(
      "const dockerRuntimeResources = createMemo(() => byType('docker-host'))",
    );
    expect(apiTokenManagerSource).toContain('const hasAgentScopeResource = (resource: Resource)');
    expect(apiTokenManagerSource).toContain("resource.type === 'agent'");
    expect(apiTokenManagerSource).toContain("resource.type === 'pbs'");
    expect(apiTokenManagerSource).toContain("resource.type === 'pmg'");
    expect(apiTokenManagerSource).toContain("resource.type === 'truenas'");
    expect(apiTokenManagerSource).toContain('resource.agent != null');
    expect(apiTokenManagerSource).toContain('markDockerRuntimesTokenRevoked');
    expect(apiTokenManagerSource).not.toContain('markDockerHostsTokenRevoked');
    expect(apiTokenManagerSource).not.toContain("resource.type === 'host'");
    expect(apiTokenManagerSource).not.toContain('markHostsTokenRevoked');
    expect(apiTokenManagerSource).not.toContain(
      "const hostResources = createMemo(() => byType('host'))",
    );
    expect(apiTokenManagerSource).not.toContain("notificationStore.error('Failed to load API tokens')");
    expect(apiTokenManagerSource).not.toContain(
      "notificationStore.error('Failed to generate API token')",
    );
    expect(apiTokenManagerSource).not.toContain(
      "notificationStore.error('Failed to revoke API token')",
    );
    expect(apiTokenPresentationSource).toContain('export function getAPITokensLoadErrorMessage');
  });

  it('keeps UnifiedAgents free of v5 merge-workaround patterns', () => {
    expect(unifiedAgentsSource).not.toContain('previousHostTypes');
    expect(unifiedAgentsSource).not.toContain('const allHosts = createMemo(');
    expect(unifiedAgentsSource).toContain('@/utils/unifiedAgentStatusPresentation');
    expect(unifiedAgentsSource).not.toContain('const MONITORING_STOPPED_STATUS_LABEL =');
    expect(unifiedAgentsSource).not.toContain('const ALLOW_RECONNECT_LABEL =');
    expect(unifiedAgentsSource).toContain('withPrivilegeEscalation');
    expect(unifiedAgentsSource).toContain('@/utils/agentCapabilityPresentation');
    expect(unifiedAgentsSource).not.toContain('const getCapabilityLabel =');
    expect(unifiedAgentsSource).not.toContain('const getCapabilityBadgeClass =');
    expect(agentCapabilityPresentationSource).toContain("export type AgentCapability = 'agent'");
    expect(agentCapabilityPresentationSource).toContain('export function getAgentCapabilityLabel');
    expect(agentCapabilityPresentationSource).toContain(
      'export function getAgentCapabilityBadgeClass',
    );
    expect(unifiedAgentsSource).not.toContain('isConnectedHealthStatus');
    expect(unifiedAgentsSource).not.toContain('const connectedFromStatus =');
    expect(agentProfilesPanelSource).toContain('isConnectedHealthStatus');
    expect(agentProfilesPanelSource).not.toContain('const connectedFromStatus =');
    expect(statusUtilsSource).toContain('export function isConnectedHealthStatus');
    expect(unifiedAgentsSource).toContain('@/utils/unifiedAgentStatusPresentation');
    expect(unifiedAgentsSource).not.toContain('const statusBadgeClass =');
    expect(unifiedAgentsSource).not.toContain('const statusBadgeClasses =');
    expect(unifiedAgentsSource).toContain('getUnifiedAgentLookupStatusPresentation');
    expect(unifiedAgentStatusPresentationSource).toContain(
      'export function getUnifiedAgentStatusPresentation',
    );
    expect(unifiedAgentStatusPresentationSource).toContain(
      'export function getUnifiedAgentLookupStatusPresentation',
    );
    expect(unifiedAgentsSource).toContain('getInventorySubjectLabel');
    expect(unifiedAgentsSource).toContain('getMonitoringStoppedEmptyState');
    expect(unifiedAgentsSource).toContain('getRemovedUnifiedAgentItemLabel');
    expect(unifiedAgentsSource).toContain('getUnifiedAgentLastSeenLabel');
    expect(unifiedAgentsSource).not.toContain('const getInventorySubjectLabel =');
    expect(unifiedAgentsSource).not.toContain('const getRemovedItemLabel =');
    expect(unifiedAgentsSource).not.toContain('const lastSeenLabel = () => {');
    expect(unifiedAgentsSource).not.toContain(
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
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getAgentLedgerLoadingState',
    );
    expect(unifiedAgentInventoryPresentationSource).toContain(
      'export function getAgentLedgerErrorState',
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
    expect(unifiedAgentsSource).not.toContain('No host identifiers are available to stop monitoring.');
    expect(unifiedAgentsSource).not.toContain('Failed to update agent configuration');
    expect(unifiedAgentsSource).not.toContain('Uninstall command copied');
    expect(unifiedAgentsSource).not.toContain('Upgrade command copied');
    expect(unifiedAgentsSource).not.toContain("notificationStore.error('Failed to copy')");
    expect(relaySettingsPanelSource).toContain('getRelayConnectionPresentation');
    expect(relaySettingsPanelSource).toContain('getRelayDiagnosticClass');
    expect(relaySettingsPanelSource).toContain('getSettingsConfigurationLoadingState');
    expect(relaySettingsPanelSource).not.toContain('const connectionStatusVariant =');
    expect(relaySettingsPanelSource).not.toContain('const connectionStatusText =');
    expect(relaySettingsPanelSource).not.toContain('Loading configuration...');
    expect(proxmoxSettingsPanelSource).toContain('getSettingsConfigurationLoadingState');
    expect(proxmoxSettingsPanelSource).not.toContain('Loading configuration...');
    expect(relayOnboardingCardSource).toContain('@/utils/relayPresentation');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_TITLE');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_DESCRIPTION');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_UPGRADE_LABEL');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_TRIAL_LABEL');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_TRIAL_STARTING_LABEL');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_SETUP_LABEL');
    expect(relayOnboardingCardSource).toContain('RELAY_ONBOARDING_DISCONNECTED_LABEL');
    expect(relayOnboardingCardSource).not.toContain('Pair Your Mobile Device');
    expect(relayOnboardingCardSource).not.toContain('Relay is currently disconnected.');
    expect(completeStepSource).toContain('@/utils/relayPresentation');
    expect(completeStepSource).toContain('RELAY_ONBOARDING_SETUP_LABEL');
    expect(completeStepSource).toContain('RELAY_ONBOARDING_TRIAL_STARTING_LABEL');
    expect(completeStepSource).toContain('RELAY_ONBOARDING_SETUP_WIZARD_TRIAL_LABEL');
    expect(completeStepSource).toContain('RELAY_ONBOARDING_TRIAL_HINT');
    expect(completeStepSource).not.toContain('Start Free Trial & Set Up Mobile');
    expect(completeStepSource).not.toContain('14-DAY PRO TRIAL');
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
    expect(aiSettingsSource).not.toContain('Loading Pulse Assistant settings...');
    expect(aiSettingsSource).not.toContain('Loading chat sessions...');
    expect(aiSettingsSource).not.toContain('No chat sessions yet. Start a chat to create one.');
    expect(aiSettingsSource).not.toContain("'Failed to summarize session.'");
    expect(aiSettingsSource).not.toContain("'Failed to get session diff.'");
    expect(aiSettingsSource).not.toContain("'Failed to revert session.'");
    expect(aiSettingsSource).not.toContain("'Failed to save Pulse Assistant settings'");
    expect(aiSettingsSource).not.toContain("'Failed to clear credentials'");
    expect(aiSettingsSource).not.toContain("'Failed to update Pulse Assistant setting'");
    expect(aiSettingsSource).not.toContain('const normalizeControlLevel =');
    expect(aiSettingsSource).not.toContain('const errorMessages: Record<string, string> =');
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
    expect(aiSettingsPresentationSource).toContain(
      'export function getAISettingsLoadingState',
    );
    expect(aiSettingsPresentationSource).toContain(
      'export function getAISettingsLoadErrorMessage',
    );
    expect(aiSettingsPresentationSource).toContain(
      'export function getAISettingsRetryLabel',
    );
    expect(aiSettingsPresentationSource).toContain(
      'export function getAIChatSessionsLoadingState',
    );
    expect(aiSettingsPresentationSource).toContain(
      'export function getAIChatSessionsEmptyState',
    );
    expect(aiSettingsPresentationSource).toContain(
      'export function getAIModelsLoadErrorMessage',
    );
    expect(aiSettingsPresentationSource).toContain(
      'export function getAIChatSessionsLoadErrorMessage',
    );
    expect(aiSettingsPresentationSource).toContain(
      'export function getAIOAuthErrorMessage',
    );
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
    expect(reportingPanelSource).toContain('REPORTING_RANGE_OPTIONS');
    expect(reportingPanelSource).toContain('getReportingToggleButtonClass');
    expect(reportingPanelSource).toContain('@/utils/upgradePresentation');
    expect(reportingPanelSource).toContain('getUpgradeActionButtonClass');
    expect(reportingPanelSource).toContain('UPGRADE_ACTION_LABEL');
    expect(reportingPanelSource).toContain('UPGRADE_TRIAL_LABEL');
    expect(reportingPanelSource).not.toContain('>Upgrade to Pro<');
    expect(reportingPanelSource).not.toContain('>Start free trial<');
    expect(reportingPanelSource).not.toContain("<For each={['24h', '7d', '30d']}>");
    expect(reportingPresentationSource).toContain('export const REPORTING_RANGE_OPTIONS');
    expect(reportingPresentationSource).toContain(
      'export function getReportingToggleButtonClass',
    );
    expect(aiIntelligenceSource).toContain('buildPatrolScheduleOptions');
    expect(aiIntelligenceSource).toContain('PATROL_NO_ISSUES_LABEL');
    expect(aiIntelligenceSource).not.toContain('No issues found');
    expect(aiIntelligenceSource).not.toContain('const SCHEDULE_PRESETS =');
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
    expect(rolesPanelSource).toContain('@/utils/upgradePresentation');
    expect(userAssignmentsPanelSource).toContain('@/utils/upgradePresentation');
    expect(agentProfilesPanelSource).toContain('@/utils/upgradePresentation');
    expect(auditLogPanelSource).toContain('@/utils/upgradePresentation');
    expect(ssoProvidersPanelSource).toContain('@/utils/upgradePresentation');
    expect(auditWebhookPanelSource).toContain('@/utils/upgradePresentation');
    expect(upgradePresentationSource).toContain('export const UPGRADE_ACTION_LABEL');
    expect(upgradePresentationSource).toContain('export const UPGRADE_TRIAL_LABEL');
    expect(upgradePresentationSource).toContain('export const UPGRADE_TRIAL_LINK_CLASS');
    expect(upgradePresentationSource).toContain('export function getUpgradeActionButtonClass');
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
    expect(organizationSharingPanelSource).toContain('@/utils/canonicalResourceTypes');
    expect(canonicalResourceTypesSource).toContain('export const CANONICAL_RESOURCE_TYPES =');
    expect(canonicalResourceTypesSource).toContain("'system-container'");
    expect(canonicalResourceTypesSource).toContain("'app-container'");
    expect(canonicalResourceTypesSource).not.toContain("'container',");
  });

  it('keeps organization role vocabulary on shared utilities', () => {
    expect(organizationAccessPanelSource).toContain('@/utils/organizationRolePresentation');
    expect(organizationAccessPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationOverviewPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationAccessPanelSource).toContain('ORGANIZATION_MEMBER_ROLE_OPTIONS');
    expect(organizationAccessPanelSource).not.toContain("{ value: 'viewer', label: 'Viewer' }");
    expect(organizationAccessPanelSource).not.toContain("notificationStore.error('User ID is required')");
    expect(organizationAccessPanelSource).not.toContain(
      "notificationStore.error(error instanceof Error ? error.message : 'Failed to add member')",
    );
    expect(organizationAccessPanelSource).not.toContain(
      "notificationStore.error(error instanceof Error ? error.message : 'Failed to remove member')",
    );
    expect(organizationOverviewPanelSource).not.toContain(
      "notificationStore.error('Display name is required')",
    );
    expect(organizationOverviewPanelSource).not.toContain(
      "error instanceof Error ? error.message : 'Failed to update organization name'",
    );
    expect(organizationSharingPanelSource).toContain('@/utils/organizationRolePresentation');
    expect(organizationSharingPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationSharingPanelSource).toContain('ORGANIZATION_SHARE_ROLE_OPTIONS');
    expect(organizationSharingPanelSource).toContain('normalizeOrganizationShareRole');
    expect(organizationSharingPanelSource).not.toContain(
      "const accessRoleOptions: Array<{ value: ShareAccessRole; label: string }> = [",
    );
    expect(organizationSharingPanelSource).not.toContain('const normalizeShareRole =');
    expect(organizationSharingPanelSource).not.toContain(
      "notificationStore.error('Target organization is required')",
    );
    expect(organizationSharingPanelSource).not.toContain(
      "notificationStore.error('Target organization must differ from the current organization')",
    );
    expect(organizationSharingPanelSource).not.toContain(
      "notificationStore.error('Valid resource type and resource ID are required')",
    );
    expect(organizationSharingPanelSource).not.toContain(
      "notificationStore.success('Resource shared successfully')",
    );
    expect(organizationSharingPanelSource).not.toContain(
      "notificationStore.success('Share removed')",
    );
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
      'export function getOrganizationMemberUserIdRequiredMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationDisplayNameRequiredMessage',
    );
  });

  it('keeps alerts agent thresholds sourced from unified agent resources', () => {
    expect(alertsPageSource).toContain('const agentResources = createMemo(');
    expect(alertsPageSource).toContain('agents={agentResources()}');
    expect(alertsPageSource).toContain("resourceType: 'Agent Disk'");
    expect(alertsPageSource).not.toContain("resourceType: 'Host Disk'");
    expect(alertsPageSource).toContain('agentDefaults');
    expect(alertsPageSource).toContain('disableAllAgents');
    expect(alertsPageSource).not.toContain('hostDefaults');
    expect(alertsPageSource).not.toContain('disableAllHosts');
  });

  it('keeps setup and node summary fallbacks aware of v6 agent facets', () => {
    expect(completeStepSource).toContain('const hasAgentFacet = (resource: Resource)');
    expect(completeStepSource).toContain("resource.type === 'agent'");
    expect(completeStepSource).toContain('command -v sudo');
    expect(completeStepSource).toContain('const agentFacetResources = resources.filter(');
    expect(completeStepSource).not.toContain('state.nodes || []');
    expect(completeStepSource).not.toContain('state.hosts || []');
    expect(completeStepSource).not.toContain(
      '(state.hosts || []).length > 0\n            ? state.hosts\n            : resources',
    );
    expect(infrastructureSelectorSource).toContain(
      'const hasAgentFacet = (resource: Resource): boolean =>',
    );
    expect(infrastructureSelectorSource).toContain("resource.type === 'truenas'");
    expect(infrastructureSelectorSource).not.toContain('if (hostLikeResources.length === 0');
    expect(guestRowSource).toContain('getDashboardGuestBackupStatusPresentation');
    expect(guestRowSource).toContain('getDashboardGuestBackupTooltip');
    expect(guestRowSource).toContain('getDashboardGuestNetworkEmptyState');
    expect(guestRowSource).toContain('getDashboardGuestDiskStatusMessage');
    expect(guestRowSource).not.toContain('const BACKUP_STATUS_CONFIG: Record<');
    expect(guestRowSource).not.toContain('No backup found');
    expect(guestRowSource).not.toContain('No IP assigned');
    expect(guestRowSource).not.toContain(
      'No filesystems found. VM may be booting or using a Live ISO.',
    );
    expect(dashboardDiskListSource).toContain('getDashboardGuestDiskStatusMessage');
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
    expect(resourceDetailMappersSource).toContain('agentId: explicitDiscoveryAgentId');
    expect(resourceDetailMappersSource).not.toContain('hostId: explicitDiscoveryAgentId');
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
    expect(aiChatSource).toContain(
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
    expect(unifiedResourceTableSource).toContain('buildMetricKeyForUnifiedResource');
    expect(unifiedResourceTableSource).not.toContain("buildMetricKey('host', resource.id)");
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
    expect(reportableResourceTypesSource).toContain(
      'export const RESOURCE_PICKER_TYPE_FILTERS',
    );
    expect(reportableResourceTypesSource).toContain(
      'export const getResourcePickerTypeFilterLabel',
    );
    expect(reportableResourceTypesSource).toContain(
      'export const getResourcePickerEmptyState',
    );
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
    expect(generalSettingsPanelSource).not.toContain('const PVE_POLLING_PRESETS =');
    expect(recoverySettingsPanelSource).toContain('@/utils/systemSettingsPresentation');
    expect(recoverySettingsPanelSource).not.toContain('const BACKUP_INTERVAL_OPTIONS =');
    expect(networkSettingsPanelSource).toContain('@/utils/systemSettingsPresentation');
    expect(networkSettingsPanelSource).not.toContain('const COMMON_DISCOVERY_SUBNETS =');
    expect(systemSettingsPresentationSource).toContain('export const PVE_POLLING_PRESETS');
    expect(systemSettingsPresentationSource).toContain('export const BACKUP_INTERVAL_OPTIONS');
    expect(systemSettingsPresentationSource).toContain('export const COMMON_DISCOVERY_SUBNETS');
    expect(systemSettingsPresentationSource).toContain(
      'export function getBackupIntervalSelectValue',
    );
    expect(systemSettingsPresentationSource).toContain(
      'export function getBackupIntervalSummary',
    );
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
    expect(ssoProvidersPanelSource).toContain('getSSOProviderTypeLabel');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderSummary');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderCardClass');
    expect(ssoProvidersPanelSource).toContain('getSSOTestResultPresentation');
    expect(ssoProvidersPanelSource).toContain('getSSOCertificatePresentation');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderAddButtonLabel');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderModalTitle');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderEmptyStateTitle');
    expect(ssoProvidersPanelSource).toContain('getSSOProviderEmptyStateDescription');
    expect(ssoProvidersPanelSource).toContain('getSSOProvidersLoadingState');
    expect(ssoProvidersPanelSource).toContain('SSOProviderTypeIcon');
    expect(ssoProvidersPanelSource).not.toContain('No SSO providers configured');
    expect(ssoProvidersPanelSource).not.toContain('Loading SSO providers...');
    expect(ssoProvidersPanelSource).not.toContain("notificationStore.error('Failed to load SSO providers')");
    expect(ssoProvidersPanelSource).not.toContain(
      "notificationStore.error('Failed to load provider details')",
    );
    expect(ssoProvidersPanelSource).not.toContain("notificationStore.success('Provider deleted')");
    expect(ssoProvidersPanelSource).not.toContain("notificationStore.error('Failed to delete provider')");
    expect(ssoProvidersPanelSource).not.toContain("notificationStore.success('Connection test successful')");
    expect(ssoProvidersPanelSource).not.toContain("notificationStore.error('Failed to test connection')");
    expect(ssoProvidersPanelSource).not.toContain("notificationStore.error('Please enter an IdP Metadata URL')");
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
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderAddButtonLabel',
    );
    expect(ssoProviderPresentationSource).toContain('export function getSSOProviderModalTitle');
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderEmptyStateTitle',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProviderEmptyStateDescription',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOProvidersLoadingState',
    );
    expect(ssoProviderPresentationSource).toContain(
      'export function getSSOTestResultPresentation',
    );
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
  });

  it('keeps RBAC permission vocabulary on shared utilities', () => {
    expect(rolesPanelSource).toContain('@/utils/rbacPermissions');
    expect(rolesPanelSource).toContain('@/utils/rbacPresentation');
    expect(rolesPanelSource).toContain('RBAC_PERMISSION_ACTIONS');
    expect(rolesPanelSource).toContain('RBAC_PERMISSION_RESOURCES');
    expect(rolesPanelSource).toContain('createDefaultRBACPermission');
    expect(rolesPanelSource).toContain('getRBACFeatureGateCopy');
    expect(rolesPanelSource).toContain('getRolesEmptyState');
    expect(rolesPanelSource).not.toContain("const ACTIONS = ['read', 'write', 'delete', 'admin', '*']");
    expect(rolesPanelSource).not.toContain("const RESOURCES = ['settings', 'audit_logs', 'nodes', 'users', 'license', '*']");
    expect(rolesPanelSource).not.toContain('Custom Roles (Pro)');
    expect(rolesPanelSource).not.toContain('No roles available.');
    expect(rbacPermissionsSource).toContain('export const RBAC_PERMISSION_ACTIONS');
    expect(rbacPermissionsSource).toContain('export const RBAC_PERMISSION_RESOURCES');
    expect(rbacPermissionsSource).toContain('export function createDefaultRBACPermission');
    expect(rbacPresentationSource).toContain('export function getRBACFeatureGateCopy');
    expect(rbacPresentationSource).toContain('export function getRolesEmptyState');
    expect(rbacPresentationSource).toContain('export function getRolesLoadErrorMessage');
    expect(rbacPresentationSource).toContain('export function getRolesSaveErrorMessage');
    expect(rbacPresentationSource).toContain(
      'export function getUserAssignmentsLoadErrorMessage',
    );
    expect(rbacPresentationSource).toContain(
      'export function getUserAssignmentsEmptyStateCopy',
    );
    expect(userAssignmentsPanelSource).toContain('@/utils/rbacPresentation');
    expect(userAssignmentsPanelSource).toContain('getRBACFeatureGateCopy');
    expect(userAssignmentsPanelSource).toContain('getUserAssignmentsEmptyStateCopy');
    expect(userAssignmentsPanelSource).toContain('getUserAssignmentsLoadErrorMessage');
    expect(userAssignmentsPanelSource).toContain('getUserAssignmentsUpdateErrorMessage');
    expect(rolesPanelSource).toContain('getRolesLoadErrorMessage');
    expect(rolesPanelSource).toContain('getRolesRequiredFieldsMessage');
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
    expect(thresholdsTableSource).toContain(
      "if (path.includes('/thresholds/containers')) return 'docker';",
    );
    expect(thresholdsTableSource).toContain('/thresholds/agents');
    expect(thresholdsTableSource).toContain("resourceType: 'Agent Disk'");
    expect(thresholdsTableSource).toContain('getAlertThresholdsSectionTitles');
    expect(thresholdsTableSource).toContain('title={sectionTitles.agentDisks}');
    expect(thresholdsTableSource).not.toContain("resourceType: 'Host Disk'");
    expect(thresholdsTableSource).toContain('props.agentDefaults');
    expect(thresholdsTableSource).not.toContain('props.hostDefaults');
    expect(thresholdsTableSource).not.toContain('timeThresholds().host');
    expect(thresholdsTableSource).not.toContain('/thresholds/docker');
    expect(thresholdsTableSource).toContain('PBS_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('GUEST_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('NODE_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('PBS_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('GUEST_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('GUEST_FILTERING_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('BACKUP_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('SNAPSHOT_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('STORAGE_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('STORAGE_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('PMG_THRESHOLDS_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('PMG_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('AGENT_THRESHOLDS_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('AGENT_DISKS_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('AGENT_DISKS_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('CONTAINER_RUNTIMES_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('CONTAINERS_FILTER_EMPTY_STATE');
    expect(thresholdsTableSource).toContain('getAlertThresholdsGuestFilterPresentation');
    expect(thresholdsTableSource).toContain('getAlertThresholdsBackupOrphanedPresentation');
    expect(thresholdsTableSource).toContain(
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
    expect(alertThresholdsPresentationSource).toContain(
      'export const PMG_THRESHOLDS_EMPTY_STATE',
    );
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
    expect(guestRowSource).toContain('agentId={getWorkloadDockerHostId(props.guest)}');
    expect(guestRowSource).not.toContain('hostId={getWorkloadDockerHostId(props.guest)}');
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
    expect(settingsPanelRegistrySource).toContain('getSettingsConfigurationLoadingState');
    expect(settingsPanelRegistrySource).not.toContain('Loading configuration...');
    expect(settingsShellPresentationSource).toContain('export function getSettingsLoadingState');
    expect(settingsShellPresentationSource).toContain(
      'export function getSettingsConfigurationLoadingState',
    );
    expect(settingsSource).not.toContain(
      "stateHosts={(state.resources ?? []).filter((r) => r.type === 'host')}",
    );
    expect(infrastructureSettingsStateSource).toContain('@/utils/infrastructureSettingsPresentation');
    expect(infrastructureSettingsStateSource).toContain('getDiscoveryScanStartErrorMessage');
    expect(infrastructureSettingsStateSource).toContain('getDiscoverySettingUpdateErrorMessage');
    expect(infrastructureSettingsStateSource).toContain('getDiscoverySubnetUpdateErrorMessage');
    expect(infrastructureSettingsStateSource).toContain(
      'getNodeTemperatureMonitoringUpdateErrorMessage',
    );
    expect(infrastructureSettingsStateSource).not.toContain(
      "notificationStore.error('Failed to start discovery scan')",
    );
    expect(infrastructureSettingsStateSource).not.toContain(
      "notificationStore.error('Failed to update discovery setting')",
    );
    expect(infrastructureSettingsStateSource).not.toContain(
      "notificationStore.error('Failed to update discovery subnet')",
    );
    expect(infrastructureSettingsStateSource).not.toContain(
      "notificationStore.error(error instanceof Error ? error.message : 'Failed to update temperature monitoring setting')",
    );
    expect(infrastructureSettingsStateSource).not.toContain(
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
