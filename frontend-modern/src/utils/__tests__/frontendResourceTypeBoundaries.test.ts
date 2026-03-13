import { describe, expect, it } from 'vitest';
import resourceTypeCompatSource from '@/utils/resourceTypeCompat.ts?raw';
import discoveryTypesSource from '@/types/discovery.ts?raw';
import resourceLinksSource from '@/routing/resourceLinks.ts?raw';
import reportingResourceTypesSource from '@/components/Settings/reportingResourceTypes.ts?raw';
import reportingResourceTypesUtilSource from '@/utils/reportingResourceTypes.ts?raw';
import chartsApiSource from '@/api/charts.ts?raw';
import investigateAlertButtonSource from '@/components/Alerts/InvestigateAlertButton.tsx?raw';
import alertTargetTypesSource from '@/utils/alertTargetTypes.ts?raw';
import resourceBadgesSource from '@/components/Infrastructure/resourceBadges.ts?raw';
import resourceBadgePresentationSource from '@/utils/resourceBadgePresentation.ts?raw';
import workloadTypeBadgesSource from '@/components/shared/workloadTypeBadges.ts?raw';
import emptyStateSource from '@/components/shared/EmptyState.tsx?raw';
import webInterfaceUrlFieldSource from '@/components/shared/WebInterfaceUrlField.tsx?raw';
import workloadTypePresentationSource from '@/utils/workloadTypePresentation.ts?raw';
import sourcePlatformsSource from '@/utils/sourcePlatforms.ts?raw';
import rbacPermissionsSource from '@/utils/rbacPermissions.ts?raw';
import systemSettingsPresentationSource from '@/utils/systemSettingsPresentation.ts?raw';
import updatesSettingsPanelSource from '@/components/Settings/UpdatesSettingsPanel.tsx?raw';
import updatesPresentationSource from '@/utils/updatesPresentation.ts?raw';
import environmentLockBadgeSource from '@/components/shared/EnvironmentLockBadge.tsx?raw';
import environmentLockPresentationSource from '@/utils/environmentLockPresentation.ts?raw';
import dockerRuntimeSettingsCardSource from '@/components/Settings/DockerRuntimeSettingsCard.tsx?raw';
import infrastructurePageSource from '@/pages/Infrastructure.tsx?raw';
import discoveryTargetSource from '@/utils/discoveryTarget.ts?raw';
import infrastructureEmptyStatePresentationSource from '@/utils/infrastructureEmptyStatePresentation.ts?raw';
import recoverySummarySource from '@/components/Recovery/RecoverySummary.tsx?raw';
import recoverySource from '@/components/Recovery/Recovery.tsx?raw';
import dashboardRecoverySource from '@/hooks/useDashboardRecovery.ts?raw';
import recoveryOutcomePresentationSource from '@/utils/recoveryOutcomePresentation.ts?raw';
import recoveryArtifactModePresentationSource from '@/utils/recoveryArtifactModePresentation.ts?raw';
import recoveryFilterChipPresentationSource from '@/utils/recoveryFilterChipPresentation.ts?raw';
import recoveryIssuePresentationSource from '@/utils/recoveryIssuePresentation.ts?raw';
import recoveryActionPresentationSource from '@/utils/recoveryActionPresentation.ts?raw';
import recoveryDatePresentationSource from '@/utils/recoveryDatePresentation.ts?raw';
import recoveryRecordPresentationSource from '@/utils/recoveryRecordPresentation.ts?raw';
import recoveryEmptyStatePresentationSource from '@/utils/recoveryEmptyStatePresentation.ts?raw';
import recoveryStatusPresentationSource from '@/utils/recoveryStatusPresentation.ts?raw';
import recoverySummaryPresentationSource from '@/utils/recoverySummaryPresentation.ts?raw';
import recoveryTablePresentationSource from '@/utils/recoveryTablePresentation.ts?raw';
import recoveryTimelineChartPresentationSource from '@/utils/recoveryTimelineChartPresentation.ts?raw';
import recoveryTimelinePresentationSource from '@/utils/recoveryTimelinePresentation.ts?raw';
import dashboardSource from '@/components/Dashboard/Dashboard.tsx?raw';
import dashboardRouteSource from '@/pages/Dashboard.tsx?raw';
import dashboardHelpersSource from '@/pages/DashboardPanels/dashboardHelpers.ts?raw';
import dashboardMetricPresentationSource from '@/utils/dashboardMetricPresentation.ts?raw';
import dashboardTrendPresentationSource from '@/utils/dashboardTrendPresentation.ts?raw';
import trendChartsSource from '@/pages/DashboardPanels/TrendCharts.tsx?raw';
import thresholdSliderSource from '@/components/Dashboard/ThresholdSlider.tsx?raw';
import compositionPanelSource from '@/pages/DashboardPanels/CompositionPanel.tsx?raw';
import dashboardCompositionPresentationSource from '@/utils/dashboardCompositionPresentation.ts?raw';
import problemResourcesTableSource from '@/pages/DashboardPanels/ProblemResourcesTable.tsx?raw';
import problemResourcePresentationSource from '@/utils/problemResourcePresentation.ts?raw';
import kpiStripSource from '@/pages/DashboardPanels/KPIStrip.tsx?raw';
import recentAlertsPanelSource from '@/pages/DashboardPanels/RecentAlertsPanel.tsx?raw';
import storagePanelSource from '@/pages/DashboardPanels/StoragePanel.tsx?raw';
import recoveryStatusPanelSource from '@/pages/DashboardPanels/RecoveryStatusPanel.tsx?raw';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import dashboardDiskListSource from '@/components/Dashboard/DiskList.tsx?raw';
import guestDrawerSource from '@/components/Dashboard/GuestDrawer.tsx?raw';
import dashboardGuestPresentationSource from '@/utils/dashboardGuestPresentation.ts?raw';
import dashboardAlertPresentationSource from '@/utils/dashboardAlertPresentation.ts?raw';
import dashboardStoragePresentationSource from '@/utils/dashboardStoragePresentation.ts?raw';
import dashboardRecoveryPresentationSource from '@/utils/dashboardRecoveryPresentation.ts?raw';
import dashboardKpiPresentationSource from '@/utils/dashboardKpiPresentation.ts?raw';
import dashboardEmptyStatePresentationSource from '@/utils/dashboardEmptyStatePresentation.ts?raw';
import diskListSource from '@/components/Storage/DiskList.tsx?raw';
import useDiskListModelSource from '@/components/Storage/useDiskListModel.ts?raw';
import diskDetailSource from '@/components/Storage/DiskDetail.tsx?raw';
import storageDetailKeyValueRowSource from '@/components/Storage/StorageDetailKeyValueRow.tsx?raw';
import storageDetailMetricCardSource from '@/components/Storage/StorageDetailMetricCard.tsx?raw';
import diskLiveMetricSource from '@/components/Storage/DiskLiveMetric.tsx?raw';
import useDiskLiveMetricModelSource from '@/components/Storage/useDiskLiveMetricModel.ts?raw';
import storageControlsSource from '@/components/Storage/StorageControls.tsx?raw';
import useStorageControlsModelSource from '@/components/Storage/useStorageControlsModel.ts?raw';
import storageFilterSource from '@/components/Storage/StorageFilter.tsx?raw';
import storagePageControlsSource from '@/components/Storage/StoragePageControls.tsx?raw';
import useStoragePageControlsModelSource from '@/components/Storage/useStoragePageControlsModel.ts?raw';
import storageCephSectionSource from '@/components/Storage/StorageCephSection.tsx?raw';
import storageContentCardSource from '@/components/Storage/StorageContentCard.tsx?raw';
import storagePageSource from '@/components/Storage/Storage.tsx?raw';
import storagePageBannerSource from '@/components/Storage/StoragePageBanner.tsx?raw';
import useStoragePageBannerModelSource from '@/components/Storage/useStoragePageBannerModel.ts?raw';
import storagePageBannersSource from '@/components/Storage/StoragePageBanners.tsx?raw';
import useStoragePageBannersModelSource from '@/components/Storage/useStoragePageBannersModel.ts?raw';
import storagePageSummarySource from '@/components/Storage/StoragePageSummary.tsx?raw';
import storageCephSummaryCardSource from '@/components/Storage/StorageCephSummaryCard.tsx?raw';
import useStorageCephSummaryCardModelSource from '@/components/Storage/useStorageCephSummaryCardModel.ts?raw';
import useStorageContentCardModelSource from '@/components/Storage/useStorageContentCardModel.ts?raw';
import useStorageCephSectionModelSource from '@/components/Storage/useStorageCephSectionModel.ts?raw';
import useZFSHealthMapModelSource from '@/components/Storage/useZFSHealthMapModel.ts?raw';
import storagePoolsTableSource from '@/components/Storage/StoragePoolsTable.tsx?raw';
import useStoragePoolsTableModelSource from '@/components/Storage/useStoragePoolsTableModel.ts?raw';
import storageGroupRowSource from '@/components/Storage/StorageGroupRow.tsx?raw';
import zfsHealthMapSource from '@/components/Storage/ZFSHealthMap.tsx?raw';
import enhancedStorageBarSource from '@/components/Storage/EnhancedStorageBar.tsx?raw';
import useEnhancedStorageBarModelSource from '@/components/Storage/useEnhancedStorageBarModel.ts?raw';
import storagePoolDetailSource from '@/components/Storage/StoragePoolDetail.tsx?raw';
import useStoragePoolDetailModelSource from '@/components/Storage/useStoragePoolDetailModel.ts?raw';
import useDiskDetailModelSource from '@/components/Storage/useDiskDetailModel.ts?raw';
import storagePoolRowSource from '@/components/Storage/StoragePoolRow.tsx?raw';
import storageExpansionStateSource from '@/components/Storage/useStorageExpansionState.ts?raw';
import storageFilterStateSource from '@/components/Storage/useStorageFilterState.ts?raw';
import useStorageFilterToolbarModelSource from '@/components/Storage/useStorageFilterToolbarModel.ts?raw';
import storagePageFiltersSource from '@/components/Storage/useStoragePageFilters.ts?raw';
import storagePageDataSource from '@/components/Storage/useStoragePageData.ts?raw';
import storagePageModelSource from '@/components/Storage/useStoragePageModel.ts?raw';
import storagePageResourcesSource from '@/components/Storage/useStoragePageResources.ts?raw';
import storagePageStatusHookSource from '@/components/Storage/useStoragePageStatus.ts?raw';
import storagePageSummaryHookSource from '@/components/Storage/useStoragePageSummary.ts?raw';
import storagePageStateSource from '@/components/Storage/storagePageState.ts?raw';
import storageResourceHighlightSource from '@/components/Storage/useStorageResourceHighlight.ts?raw';
import useStorageModelSource from '@/components/Storage/useStorageModel.ts?raw';
import useStorageCephModelSource from '@/components/Storage/useStorageCephModel.ts?raw';
import useStorageAlertStateSource from '@/components/Storage/useStorageAlertState.ts?raw';
import temperatureUtilSource from '@/utils/temperature.ts?raw';
import pmgInstanceDrawerSource from '@/components/PMG/PMGInstanceDrawer.tsx?raw';
import serviceHealthPresentationSource from '@/utils/serviceHealthPresentation.ts?raw';
import swarmServicesDrawerSource from '@/components/Docker/SwarmServicesDrawer.tsx?raw';
import swarmPresentationSource from '@/utils/swarmPresentation.ts?raw';
import k8sDeploymentsDrawerSource from '@/components/Kubernetes/K8sDeploymentsDrawer.tsx?raw';
import k8sDeploymentPresentationSource from '@/utils/k8sDeploymentPresentation.ts?raw';
import k8sNamespacesDrawerSource from '@/components/Kubernetes/K8sNamespacesDrawer.tsx?raw';
import k8sNamespacePresentationSource from '@/utils/k8sNamespacePresentation.ts?raw';
import k8sStatusPresentationSource from '@/utils/k8sStatusPresentation.ts?raw';
import raidCardSource from '@/components/shared/cards/RaidCard.tsx?raw';
import raidPresentationSource from '@/utils/raidPresentation.ts?raw';
import proLicensePanelSource from '@/components/Settings/ProLicensePanel.tsx?raw';
import securityPostureSummarySource from '@/components/Settings/SecurityPostureSummary.tsx?raw';
import securityAuthPanelSource from '@/components/Settings/SecurityAuthPanel.tsx?raw';
import securityWarningSource from '@/components/SecurityWarning.tsx?raw';
import licensePresentationSource from '@/utils/licensePresentation.ts?raw';
import securityScorePresentationSource from '@/utils/securityScorePresentation.ts?raw';
import securityAuthPresentationSource from '@/utils/securityAuthPresentation.ts?raw';
import resourceDetailDrawerSource from '@/components/Infrastructure/ResourceDetailDrawer.tsx?raw';
import resourceDetailMappersSource from '@/components/Infrastructure/resourceDetailMappers.ts?raw';
import unifiedResourceTableSource from '@/components/Infrastructure/UnifiedResourceTable.tsx?raw';
import findingsPanelSource from '@/components/AI/FindingsPanel.tsx?raw';
import exploreStatusBlockSource from '@/components/AI/Chat/ExploreStatusBlock.tsx?raw';
import aiExplorePresentationSource from '@/utils/aiExplorePresentation.ts?raw';
import aiFindingPresentationSource from '@/utils/aiFindingPresentation.ts?raw';
import discoveryTabSource from '@/components/Discovery/DiscoveryTab.tsx?raw';
import discoveryPresentationSource from '@/utils/discoveryPresentation.ts?raw';
import mailGatewaySource from '@/components/PMG/MailGateway.tsx?raw';
import pmgInstancePanelSource from '@/components/PMG/PMGInstancePanel.tsx?raw';
import pmgPresentationSource from '@/utils/pmgPresentation.ts?raw';
import pmgThreatPresentationSource from '@/utils/pmgThreatPresentation.ts?raw';
import pmgQueuePresentationSource from '@/utils/pmgQueuePresentation.ts?raw';
import pmgServiceHealthBadgeSource from '@/components/PMG/ServiceHealthBadge.tsx?raw';
import proxmoxSettingsPanelSource from '@/components/Settings/ProxmoxSettingsPanel.tsx?raw';
import proxmoxSettingsPresentationSource from '@/utils/proxmoxSettingsPresentation.ts?raw';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';
import nodeModalSource from '@/components/Settings/NodeModal.tsx?raw';
import nodeModalPresentationSource from '@/utils/nodeModalPresentation.ts?raw';
import cephPageSource from '@/pages/Ceph.tsx?raw';
import cephServiceIconSource from '@/components/Ceph/CephServiceIcon.tsx?raw';
import diskPresentationSource from '@/features/storageBackups/diskPresentation.ts?raw';
import diskLiveMetricPresentationSource from '@/features/storageBackups/diskLiveMetricPresentation.ts?raw';
import storageDetailPresentationSource from '@/features/storageBackups/detailPresentation.ts?raw';
import diskDetailPresentationSource from '@/features/storageBackups/diskDetailPresentation.ts?raw';
import storageRecordPresentationSource from '@/features/storageBackups/recordPresentation.ts?raw';
import storageModelCoreSource from '@/features/storageBackups/storageModelCore.ts?raw';
import resourceStorageMappingSource from '@/features/storageBackups/resourceStorageMapping.ts?raw';
import resourceStoragePresentationSource from '@/features/storageBackups/resourceStoragePresentation.ts?raw';
import storageAdapterCoreSource from '@/features/storageBackups/storageAdapterCore.ts?raw';
import storageAlertStateSource from '@/features/storageBackups/storageAlertState.ts?raw';
import cephRecordPresentationSource from '@/features/storageBackups/cephRecordPresentation.ts?raw';
import cephSummaryPresentationSource from '@/features/storageBackups/cephSummaryPresentation.ts?raw';
import cephSummaryCardPresentationSource from '@/features/storageBackups/cephSummaryCardPresentation.ts?raw';
import storageDomainSource from '@/features/storageBackups/storageDomain.ts?raw';
import storagePoolDetailPresentationSource from '@/features/storageBackups/storagePoolDetailPresentation.ts?raw';
import storageBarPresentationSource from '@/features/storageBackups/storageBarPresentation.ts?raw';
import storagePagePresentationSource from '@/features/storageBackups/storagePagePresentation.ts?raw';
import storagePageStatusSource from '@/features/storageBackups/storagePageStatus.ts?raw';
import storageRowPresentationSource from '@/features/storageBackups/rowPresentation.ts?raw';
import storagePoolRowPresentationSource from '@/features/storageBackups/storagePoolRowPresentation.ts?raw';
import storageGroupPresentationSource from '@/features/storageBackups/groupPresentation.ts?raw';
import storageFilterPresentationSource from '@/features/storageBackups/storageFilterPresentation.ts?raw';
import storageRowAlertPresentationSource from '@/features/storageBackups/storageRowAlertPresentation.ts?raw';
import storagePoolsTablePresentationSource from '@/features/storageBackups/storagePoolsTablePresentation.ts?raw';
import zfsPresentationSource from '@/features/storageBackups/zfsPresentation.ts?raw';
import zfsHealthMapPresentationSource from '@/features/storageBackups/zfsHealthMapPresentation.ts?raw';
import storageAdaptersSource from '@/features/storageBackups/storageAdapters.ts?raw';
import deployStatusBadgeSource from '@/components/Infrastructure/deploy/DeployStatusBadge.tsx?raw';
import deployCandidatesStepSource from '@/components/Infrastructure/deploy/CandidatesStep.tsx?raw';
import deployResultsStepSource from '@/components/Infrastructure/deploy/ResultsStep.tsx?raw';
import deployFlowPresentationSource from '@/utils/deployFlowPresentation.ts?raw';
import deployStatusPresentationSource from '@/utils/deployStatusPresentation.ts?raw';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import alertDestinationsPresentationSource from '@/utils/alertDestinationsPresentation.ts?raw';
import alertBulkEditPresentationSource from '@/utils/alertBulkEditPresentation.ts?raw';
import alertEmailPresentationSource from '@/utils/alertEmailPresentation.ts?raw';
import alertOverviewPresentationSource from '@/utils/alertOverviewPresentation.ts?raw';
import alertResourceTablePresentationSource from '@/utils/alertResourceTablePresentation.ts?raw';
import alertWebhookPresentationSource from '@/utils/alertWebhookPresentation.ts?raw';
import alertOverviewTabSource from '@/features/alerts/OverviewTab.tsx?raw';
import alertIncidentPresentationSource from '@/utils/alertIncidentPresentation.ts?raw';
import alertHistoryPresentationSource from '@/utils/alertHistoryPresentation.ts?raw';
import bulkEditDialogSource from '@/components/Alerts/BulkEditDialog.tsx?raw';
import alertResourceTableSource from '@/components/Alerts/ResourceTable.tsx?raw';
import emailProviderSelectSource from '@/components/Alerts/EmailProviderSelect.tsx?raw';
import webhookConfigSource from '@/components/Alerts/WebhookConfig.tsx?raw';
import alertActivationPresentationSource from '@/utils/alertActivationPresentation.ts?raw';
import alertFrequencyPresentationSource from '@/utils/alertFrequencyPresentation.ts?raw';
import alertSeverityPresentationSource from '@/utils/alertSeverityPresentation.ts?raw';
import alertTabsPresentationSource from '@/utils/alertTabsPresentation.ts?raw';
import alertGroupingPresentationSource from '@/utils/alertGroupingPresentation.ts?raw';
import alertSchedulePresentationSource from '@/utils/alertSchedulePresentation.ts?raw';
import configuredNodeTablesSource from '@/components/Settings/ConfiguredNodeTables.tsx?raw';
import configuredNodeCapabilityPresentationSource from '@/utils/configuredNodeCapabilityPresentation.ts?raw';
import configuredNodeStatusPresentationSource from '@/utils/configuredNodeStatusPresentation.ts?raw';
import rbacPresentationSource from '@/utils/rbacPresentation.ts?raw';
import upgradePresentationSource from '@/utils/upgradePresentation.ts?raw';
import agentProfilesPanelSource from '@/components/Settings/AgentProfilesPanel.tsx?raw';
import agentProfilesPresentationSource from '@/utils/agentProfilesPresentation.ts?raw';
import organizationAccessPanelSource from '@/components/Settings/OrganizationAccessPanel.tsx?raw';
import billingAdminPanelSource from '@/components/Settings/BillingAdminPanel.tsx?raw';
import organizationBillingPanelSource from '@/components/Settings/OrganizationBillingPanel.tsx?raw';
import organizationOverviewPanelSource from '@/components/Settings/OrganizationOverviewPanel.tsx?raw';
import organizationSharingPanelSource from '@/components/Settings/OrganizationSharingPanel.tsx?raw';
import organizationRolePresentationSource from '@/utils/organizationRolePresentation.ts?raw';
import organizationSettingsPresentationSource from '@/utils/organizationSettingsPresentation.ts?raw';
import rolesPanelSource from '@/components/Settings/RolesPanel.tsx?raw';
import auditWebhookPanelSource from '@/components/Settings/AuditWebhookPanel.tsx?raw';
import auditWebhookPresentationSource from '@/utils/auditWebhookPresentation.ts?raw';
import auditLogPanelSource from '@/components/Settings/AuditLogPanel.tsx?raw';
import auditLogPresentationSource from '@/utils/auditLogPresentation.ts?raw';
import ssoProvidersPanelSource from '@/components/Settings/SSOProvidersPanel.tsx?raw';
import ssoProviderPresentationSource from '@/utils/ssoProviderPresentation.ts?raw';
import userAssignmentsPanelSource from '@/components/Settings/UserAssignmentsPanel.tsx?raw';
import investigationMessagesSource from '@/components/patrol/InvestigationMessages.tsx?raw';
import investigationSectionSource from '@/components/patrol/InvestigationSection.tsx?raw';
import runHistoryPanelSource from '@/components/patrol/RunHistoryPanel.tsx?raw';
import runToolCallTraceSource from '@/components/patrol/RunToolCallTrace.tsx?raw';
import diagnosticsPanelSource from '@/components/Settings/DiagnosticsPanel.tsx?raw';
import diagnosticsPresentationSource from '@/utils/diagnosticsPresentation.ts?raw';
import aiSettingsSource from '@/components/Settings/AISettings.tsx?raw';
import aiIntelligenceSource from '@/pages/AIIntelligence.tsx?raw';
import aiQuickstartPresentationSource from '@/utils/aiQuickstartPresentation.ts?raw';
import aiCostPresentationSource from '@/utils/aiCostPresentation.ts?raw';
import thresholdSliderPresentationSource from '@/utils/thresholdSliderPresentation.ts?raw';
import emptyStatePresentationSource from '@/utils/emptyStatePresentation.ts?raw';
import reportingPanelSource from '@/components/Settings/ReportingPanel.tsx?raw';
import systemLogsPanelSource from '@/components/Settings/SystemLogsPanel.tsx?raw';
import systemLogsPresentationSource from '@/utils/systemLogsPresentation.ts?raw';
import patrolEmptyStatePresentationSource from '@/utils/patrolEmptyStatePresentation.ts?raw';
import patrolRunPresentationSource from '@/utils/patrolRunPresentation.ts?raw';
import patrolSummaryPresentationSource from '@/utils/patrolSummaryPresentation.ts?raw';
import aiCostDashboardSource from '@/components/AI/AICostDashboard.tsx?raw';
import modelSelectorSource from '@/components/AI/Chat/ModelSelector.tsx?raw';
import remediationStatusSource from '@/components/patrol/RemediationStatus.tsx?raw';
import remediationPresentationSource from '@/utils/remediationPresentation.ts?raw';
import aiChatPresentationSource from '@/utils/aiChatPresentation.ts?raw';
import infrastructureDetailsDrawerSource from '@/components/shared/InfrastructureDetailsDrawer.tsx?raw';

describe('frontend resource type boundaries', () => {
  it('keeps the shared compatibility adapter narrow and explicit', () => {
    expect(resourceTypeCompatSource).toContain('export const canonicalizeFrontendResourceType');
    expect(resourceTypeCompatSource).toContain("case 'host'");
    expect(resourceTypeCompatSource).toContain("case 'docker'");
    expect(resourceTypeCompatSource).toContain("case 'docker_host'");
    expect(resourceTypeCompatSource).toContain("case 'k8s'");
    expect(resourceTypeCompatSource).toContain("case 'kubernetes_cluster'");
    expect(resourceTypeCompatSource).not.toContain("case 'qemu'");
    expect(resourceTypeCompatSource).not.toContain("case 'lxc'");
    expect(resourceTypeCompatSource).not.toContain("case 'container'");
  });

  it('keeps canonical frontend discovery types separate from backend API aliases', () => {
    expect(discoveryTypesSource).toContain(
      "export type ResourceType = 'vm' | 'system-container' | 'app-container' | 'pod' | 'agent';",
    );
    expect(discoveryTypesSource).toContain('export type APIResourceType =');
    expect(discoveryTypesSource).toContain("| 'k8s'");
  });

  it('keeps compatibility handling centralized in shared adapters and edge translators', () => {
    expect(resourceLinksSource).toContain('canonicalizeWorkloadFilterType');
    expect(resourceLinksSource).toContain('normalizeSourcePlatformQueryValue');
    expect(resourceLinksSource).not.toContain("normalized === 'docker'");
    expect(resourceLinksSource).not.toContain("normalized === 'k8s'");
    expect(sourcePlatformsSource).toContain('export const normalizeSourcePlatformQueryValue');

    expect(reportingResourceTypesSource).toContain('@/utils/reportingResourceTypes');
    expect(reportingResourceTypesUtilSource).toContain('export function toReportingResourceType');
    expect(reportingResourceTypesUtilSource).toContain("case 'k8s-cluster'");
    expect(reportingResourceTypesUtilSource).toContain("return 'k8s';");
    expect(reportingResourceTypesSource).not.toContain("case 'host'");

    expect(chartsApiSource).toContain('export function toMetricsHistoryAPIResourceType');
    expect(chartsApiSource).toContain('export function asMetricsHistoryResourceType');
    expect(chartsApiSource).toContain('export function mapUnifiedTypeToHistoryResourceType');
    expect(chartsApiSource).toContain('export function canonicalizeMetricsHistoryTargetType');
    expect(chartsApiSource).toContain("| 'k8s-cluster'");
    expect(chartsApiSource).toContain("| 'k8s-node'");
    expect(chartsApiSource).toContain("| 'pod'");
    expect(chartsApiSource).toContain("case 'k8s-cluster'");
    expect(chartsApiSource).toContain("return 'k8s';");
    expect(chartsApiSource).toContain(
      "guestTypes?: Record<string, 'vm' | 'system-container' | 'k8s'>",
    );

    expect(investigateAlertButtonSource).toContain('resolveAlertTargetType');
    expect(investigateAlertButtonSource).not.toContain('canonicalizeFrontendResourceType');
    expect(alertTargetTypesSource).toContain('canonicalizeFrontendResourceType');
    expect(resourceBadgesSource).toContain('@/utils/resourceBadgePresentation');
    expect(resourceBadgePresentationSource).toContain('getResourceTypePresentation');
    expect(resourceBadgesSource).not.toContain('function formatType(');
    expect(workloadTypePresentationSource).toContain('canonicalizeFrontendResourceType');
    expect(workloadTypeBadgesSource).not.toContain('canonicalizeFrontendResourceType');
    expect(workloadTypeBadgesSource).toContain('getWorkloadTypePresentation');
    expect(emptyStateSource).toContain('getEmptyStatePresentation');
    expect(emptyStateSource).not.toContain('const iconBgClass: Record<EmptyStateTone, string> =');
    expect(emptyStateSource).not.toContain(
      'const titleToneClass: Record<EmptyStateTone, string> =',
    );
    expect(emptyStateSource).not.toContain(
      'const descriptionToneClass: Record<EmptyStateTone, string> =',
    );
    expect(emptyStatePresentationSource).toContain('export function getEmptyStatePresentation');
    expect(discoveryTargetSource).toContain('canonicalizeFrontendResourceType');
    expect(recoveryOutcomePresentationSource).toContain('import type { RecoveryOutcome }');
    expect(recoverySummarySource).toContain('buildRecoveryPostureSegments');
    expect(recoverySummarySource).toContain('RECOVERY_SUMMARY_TIME_RANGES');
    expect(recoverySummarySource).toContain('buildRecoveryFreshnessBuckets');
    expect(recoverySummarySource).not.toContain("const RECOVERY_TIME_RANGES: readonly string[] = ['7d', '30d', '90d']");
    expect(recoverySummarySource).not.toContain('const RECOVERY_TIME_RANGE_LABELS: Record<string, string>');
    expect(recoverySummarySource).not.toContain("const FRESHNESS_LABELS:");
    expect(recoverySource).toContain('getRecoveryArtifactModePresentation');
    expect(recoverySource).not.toContain('const MODE_LABELS: Record<ArtifactMode, string>');
    expect(recoverySource).not.toContain('const MODE_BADGE_CLASS: Record<ArtifactMode, string>');
    expect(recoverySource).not.toContain(
      'const CHART_SEGMENT_CLASS: Record<ArtifactMode, string>',
    );
    expect(recoverySource).toContain('getRecoveryIssueRailClass');
    expect(recoverySource).not.toContain(
      "const ISSUE_RAIL_CLASS: Record<Exclude<IssueTone, 'none'>, string>",
    );
    expect(recoverySummarySource).not.toContain('const normalizeOutcome =');
    expect(dashboardRecoverySource).toContain('normalizeRecoveryOutcome');
    expect(dashboardRecoverySource).not.toContain('const normalizeOutcome =');
    expect(recoveryArtifactModePresentationSource).toContain(
      'export function getRecoveryArtifactModePresentation',
    );
    expect(recoveryIssuePresentationSource).toContain('export function getRecoveryIssueRailClass');
    expect(recoverySource).toContain('getRecoveryFilterChipPresentation');
    expect(recoverySource).toContain("segmentedButtonClass(chartRangeDays() === range, false, 'accent')");
    expect(recoverySource).not.toContain('border-blue-200 bg-blue-50 px-2 py-0.5');
    expect(recoverySource).not.toContain('border-cyan-200 bg-cyan-50 px-2 py-0.5');
    expect(recoverySource).not.toContain('border-emerald-200 bg-emerald-50 px-2 py-0.5');
    expect(recoverySource).not.toContain('border-violet-200 bg-violet-50 px-2 py-0.5');
    expect(recoverySource).not.toContain(
      "chartRangeDays() === range\n                            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'",
    );
    expect(recoverySource).toContain('getRecoveryTimelineColumnButtonClass');
    expect(recoverySource).toContain('getRecoveryArtifactColumnHeaderClass');
    expect(recoverySource).toContain('getRecoveryArtifactRowClass');
    expect(recoverySource).toContain('recoveryDateKeyFromTimestamp');
    expect(recoverySource).toContain('parseRecoveryDateKey');
    expect(recoverySource).toContain('getRecoveryPrettyDateLabel');
    expect(recoverySource).toContain('getRecoveryFullDateLabel');
    expect(recoverySource).toContain('getRecoveryCompactAxisLabel');
    expect(recoverySource).toContain('formatRecoveryTimeOnly');
    expect(recoverySource).toContain('getRecoveryNiceAxisMax');
    expect(recoverySource).toContain('getRecoveryProtectedToggleClass');
    expect(recoverySource).toContain('getRecoveryRollupStatusPillClass');
    expect(recoverySource).toContain('getRecoveryRollupStatusPillLabel');
    expect(recoverySource).toContain('getRecoverySpecialOutcomeTextClass');
    expect(recoverySource).toContain('getRecoveryBreadcrumbLinkClass');
    expect(recoverySource).toContain('getRecoveryFilterPanelClearClass');
    expect(recoverySource).toContain('getRecoveryEmptyStateActionClass');
    expect(recoverySource).toContain('getRecoveryProtectedItemsLoadingState');
    expect(recoverySource).toContain('getRecoveryProtectedItemsFailureState');
    expect(recoverySource).toContain('getRecoveryActivityLoadingState');
    expect(recoverySource).toContain('getRecoveryActivityEmptyState');
    expect(recoverySource).toContain('getRecoveryPointsLoadingState');
    expect(recoverySource).toContain('getRecoveryPointsFailureState');
    expect(recoverySource).toContain('getRecoveryProtectedItemsEmptyState');
    expect(recoverySource).toContain('getRecoveryHistoryEmptyState');
    expect(recoverySource).toContain('getRecoveryDrawerCloseButtonClass');
    expect(recoverySource).toContain('getRecoveryRollupSubjectLabel');
    expect(recoverySource).toContain('getRecoveryPointSubjectLabel');
    expect(recoverySource).toContain('getRecoveryPointRepositoryLabel');
    expect(recoverySource).toContain('getRecoveryPointDetailsSummary');
    expect(recoverySource).toContain('getRecoveryPointTimestampMs');
    expect(recoverySource).toContain('normalizeRecoveryModeQueryValue');
    expect(recoverySource).toContain('getRecoveryTimelineAxisLabelClass');
    expect(recoverySource).toContain('getRecoveryTimelineBarMinWidthClass');
    expect(recoverySource).toContain('getRecoveryTimelineLabelEvery');
    expect(recoverySource).toContain('getRecoveryGroupNoTimestampLabel');
    expect(recoverySource).toContain('getRecoveryProtectedSearchPlaceholder');
    expect(recoverySource).toContain('getRecoveryHistorySearchPlaceholder');
    expect(recoverySource).toContain('getRecoverySearchHistoryEmptyMessage');
    expect(recoverySource).not.toContain('Search protected items...');
    expect(recoverySource).not.toContain('Search recovery history...');
    expect(recoverySource).not.toContain('Recent searches appear here.');
    expect(recoverySource).toContain('RECOVERY_TIMELINE_LEGEND_ITEM_CLASS');
    expect(recoverySource).toContain('RECOVERY_TIMELINE_RANGE_GROUP_CLASS');
    expect(recoverySource).toContain('getRecoveryEventTimeTextClass');
    expect(recoverySource).toContain('getRecoverySubjectTypeBadgeClass');
    expect(recoverySource).toContain('getRecoverySubjectTypeLabel');
    expect(recoverySource).toContain('getRecoveryRollupIssueTone');
    expect(recoverySource).toContain('getRecoveryRollupAgeTextClass');
    expect(recoverySource).toContain('RECOVERY_ADVANCED_FILTER_LABEL_CLASS');
    expect(recoverySource).toContain('RECOVERY_ADVANCED_FILTER_FIELD_CLASS');
    expect(recoverySource).toContain('RECOVERY_GROUP_HEADER_ROW_CLASS');
    expect(recoverySource).toContain('RECOVERY_GROUP_HEADER_TEXT_CLASS');
    expect(recoverySource).not.toContain(
      "isSelected\n                                    ? 'bg-blue-100 dark:bg-blue-900'\n                                    : 'hover:bg-surface-hover'",
    );
    expect(recoverySource).not.toContain('const groupHeaderRowClass =');
    expect(recoverySource).not.toContain('const groupHeaderTextClass =');
    expect(recoverySource).not.toContain('const dateKeyFromTimestamp =');
    expect(recoverySource).not.toContain('const parseDateKey =');
    expect(recoverySource).not.toContain('const prettyDateLabel =');
    expect(recoverySource).not.toContain('const fullDateLabel =');
    expect(recoverySource).not.toContain('const compactAxisLabel =');
    expect(recoverySource).not.toContain('const formatTimeOnly =');
    expect(recoverySource).not.toContain('const niceAxisMax =');
    expect(recoverySource).not.toContain(
      "protectedStaleOnly() ? 'border-amber-300 bg-amber-50 text-amber-800",
    );
    expect(recoverySource).not.toContain('rounded-full bg-blue-100/80 px-1.5 py-px');
    expect(recoverySource).not.toContain(
      'text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors',
    );
    expect(recoverySource).not.toContain(
      'inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content hover:bg-surface-hover',
    );
    expect(recoverySource).not.toContain('rounded-md p-1 hover:text-base-content hover:bg-surface-hover');
    expect(recoverySource).not.toContain("const labelEvery = dayCount <= 7 ? 1 : dayCount <= 30 ? 3 : 10");
    expect(recoverySource).not.toContain("class=\"flex items-center gap-1\"");
    expect(recoverySource).not.toContain("class=\"inline-flex rounded border border-border bg-surface p-0.5 text-xs\"");
    expect(recoverySource).not.toContain("'font-semibold text-blue-700 dark:text-blue-300'");
    expect(recoverySource).not.toContain('const rollupSubjectLabel =');
    expect(recoverySource).not.toContain('const pointTimestampMs =');
    expect(recoverySource).not.toContain('const buildSubjectLabelForPoint =');
    expect(recoverySource).not.toContain('const buildRepositoryLabelForPoint =');
    expect(recoverySource).not.toContain('const buildDetailsSummaryForPoint =');
    expect(recoverySource).not.toContain('const normalizeModeFromQuery =');
    expect(recoverySource).not.toContain('No protected items yet');
    expect(recoverySource).not.toContain('Loading protected items...');
    expect(recoverySource).not.toContain('Loading recovery activity...');
    expect(recoverySource).not.toContain('No recovery activity in the selected window.');
    expect(recoverySource).not.toContain('Loading recovery points...');
    expect(recoverySource).not.toContain('Failed to load protected items');
    expect(recoverySource).not.toContain('Failed to load recovery points');
    expect(recoverySource).not.toContain('Pulse hasn’t observed any protected items for this org yet.');
    expect(recoverySource).not.toContain('No recovery history matches your filters');
    expect(recoverySource).not.toContain(
      'Adjust your search, provider, method, status, or verification filters.',
    );
    expect(recoverySource).not.toContain('const eventTimeTextClass =');
    expect(recoverySource).not.toContain('const subjectTypeBadgeClass =');
    expect(recoverySource).not.toContain('const artifactColumnHeaderClass =');
    expect(recoverySource).not.toContain('const artifactRowClass =');
    expect(recoverySource).not.toContain('const advancedFilterLabelClass =');
    expect(recoverySource).not.toContain('const advancedFilterFieldClass =');
    expect(recoverySource).not.toContain('const deriveRollupIssueTone =');
    expect(recoverySource).not.toContain('const rollupAgeTextClass =');
    expect(recoveryFilterChipPresentationSource).toContain(
      'export function getRecoveryFilterChipPresentation',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoveryArtifactColumnHeaderClass',
    );
    expect(recoveryDatePresentationSource).toContain(
      'export function recoveryDateKeyFromTimestamp',
    );
    expect(recoveryDatePresentationSource).toContain('export function parseRecoveryDateKey');
    expect(recoveryDatePresentationSource).toContain(
      'export function getRecoveryPrettyDateLabel',
    );
    expect(recoveryDatePresentationSource).toContain(
      'export function getRecoveryFullDateLabel',
    );
    expect(recoveryDatePresentationSource).toContain(
      'export function getRecoveryCompactAxisLabel',
    );
    expect(recoveryDatePresentationSource).toContain(
      'export function formatRecoveryTimeOnly',
    );
    expect(recoveryDatePresentationSource).toContain(
      'export function getRecoveryNiceAxisMax',
    );
    expect(recoveryActionPresentationSource).toContain(
      'export function getRecoveryBreadcrumbLinkClass',
    );
    expect(recoveryActionPresentationSource).toContain(
      'export function getRecoveryFilterPanelClearClass',
    );
    expect(recoveryActionPresentationSource).toContain(
      'export function getRecoveryEmptyStateActionClass',
    );
    expect(recoveryActionPresentationSource).toContain(
      'export function getRecoveryDrawerCloseButtonClass',
    );
    expect(recoveryRecordPresentationSource).toContain(
      'export function getRecoveryRollupSubjectLabel',
    );
    expect(recoveryRecordPresentationSource).toContain(
      'export function getRecoveryPointSubjectLabel',
    );
    expect(recoveryRecordPresentationSource).toContain(
      'export function getRecoveryPointRepositoryLabel',
    );
    expect(recoveryRecordPresentationSource).toContain(
      'export function getRecoveryPointDetailsSummary',
    );
    expect(recoveryEmptyStatePresentationSource).toContain(
      'export function getRecoveryProtectedItemsLoadingState',
    );
    expect(recoveryEmptyStatePresentationSource).toContain(
      'export function getRecoveryProtectedItemsFailureState',
    );
    expect(recoveryEmptyStatePresentationSource).toContain(
      'export function getRecoveryActivityLoadingState',
    );
    expect(recoveryEmptyStatePresentationSource).toContain(
      'export function getRecoveryActivityEmptyState',
    );
    expect(recoveryEmptyStatePresentationSource).toContain(
      'export function getRecoveryProtectedItemsEmptyState',
    );
    expect(recoveryEmptyStatePresentationSource).toContain(
      'export function getRecoveryHistoryEmptyState',
    );
    expect(recoveryEmptyStatePresentationSource).toContain(
      'export function getRecoveryPointsLoadingState',
    );
    expect(recoveryEmptyStatePresentationSource).toContain(
      'export function getRecoveryPointsFailureState',
    );
    expect(recoveryRecordPresentationSource).toContain(
      'export function getRecoveryPointTimestampMs',
    );
    expect(recoveryRecordPresentationSource).toContain(
      'export function normalizeRecoveryModeQueryValue',
    );
    expect(recoveryTimelineChartPresentationSource).toContain(
      'export function getRecoveryTimelineAxisLabelClass',
    );
    expect(recoveryTimelineChartPresentationSource).toContain(
      'export function getRecoveryTimelineBarMinWidthClass',
    );
    expect(recoveryTimelineChartPresentationSource).toContain(
      'export function getRecoveryTimelineLabelEvery',
    );
    expect(recoveryStatusPresentationSource).toContain(
      'export function getRecoveryProtectedToggleClass',
    );
    expect(recoveryStatusPresentationSource).toContain(
      'export function getRecoveryRollupStatusPillClass',
    );
    expect(recoveryStatusPresentationSource).toContain(
      'export function getRecoveryRollupStatusPillLabel',
    );
    expect(recoveryStatusPresentationSource).toContain(
      'export function getRecoverySpecialOutcomeTextClass',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoveryArtifactRowClass',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoveryEventTimeTextClass',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoverySubjectTypeBadgeClass',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoverySubjectTypeLabel',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoveryRollupIssueTone',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoveryRollupAgeTextClass',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoveryGroupNoTimestampLabel',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoveryProtectedSearchPlaceholder',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoveryHistorySearchPlaceholder',
    );
    expect(recoveryTablePresentationSource).toContain(
      'export function getRecoverySearchHistoryEmptyMessage',
    );
    expect(recoveryTimelinePresentationSource).toContain(
      'export function getRecoveryTimelineColumnButtonClass',
    );
    expect(recoverySummaryPresentationSource).toContain(
      'export const RECOVERY_SUMMARY_TIME_RANGES',
    );
    expect(recoverySummaryPresentationSource).toContain(
      'export const RECOVERY_FRESHNESS_BUCKETS',
    );
    expect(recoverySummaryPresentationSource).toContain(
      'export function getRecoveryAttentionChipClass',
    );
    expect(recoverySummaryPresentationSource).toContain(
      'export function getRecoveryAttentionDotClass',
    );
    expect(recoverySummarySource).toContain('getRecoveryAttentionChipClass');
    expect(recoverySummarySource).toContain('getRecoveryAttentionDotClass');
    expect(recoverySummarySource).not.toContain("function getAttentionChipClass(");
    expect(recoverySummarySource).not.toContain("function getAttentionDotClass(");
    expect(dashboardHelpersSource).toContain("from '@/utils/dashboardMetricPresentation'");
    expect(dashboardHelpersSource).not.toContain('export function statusBadgeClass');
    expect(dashboardHelpersSource).not.toContain('export function priorityBadgeClass');
    expect(dashboardMetricPresentationSource).toContain(
      'export function getDashboardStatusBadgeClass',
    );
    expect(dashboardMetricPresentationSource).toContain(
      'export function getDashboardPriorityBadgeClass',
    );
    expect(trendChartsSource).toContain('getDashboardTrendColor');
    expect(trendChartsSource).toContain('getDashboardTrendErrorState');
    expect(trendChartsSource).not.toContain('RESOURCE_COLORS');
    expect(trendChartsSource).not.toContain('Unable to load trends');
    expect(dashboardTrendPresentationSource).toContain(
      'export function getDashboardTrendColor',
    );
    expect(dashboardTrendPresentationSource).toContain(
      'export function getDashboardTrendErrorState',
    );
    expect(systemSettingsPresentationSource).toContain('export const PVE_POLLING_PRESETS');
    expect(systemSettingsPresentationSource).toContain('export const BACKUP_INTERVAL_OPTIONS');
    expect(systemSettingsPresentationSource).toContain(
      'export function getBackupIntervalSelectValue',
    );
    expect(systemSettingsPresentationSource).toContain(
      'export function getBackupIntervalSummary',
    );
    expect(systemSettingsPresentationSource).toContain('export const COMMON_DISCOVERY_SUBNETS');
    expect(reportingPanelSource).toContain('@/utils/upgradePresentation');
    expect(reportingPanelSource).toContain('getUpgradeActionButtonClass');
    expect(reportingPanelSource).toContain('UPGRADE_ACTION_LABEL');
    expect(reportingPanelSource).toContain('UPGRADE_TRIAL_LABEL');
    expect(reportingPanelSource).not.toContain('>Upgrade to Pro<');
    expect(reportingPanelSource).not.toContain('>Start free trial<');
    expect(rolesPanelSource).toContain('@/utils/upgradePresentation');
    expect(userAssignmentsPanelSource).toContain('@/utils/upgradePresentation');
    expect(agentProfilesPanelSource).toContain('@/utils/upgradePresentation');
    expect(agentProfilesPanelSource).toContain('@/utils/agentProfilesPresentation');
    expect(agentProfilesPanelSource).toContain('getAgentProfilesEmptyState');
    expect(agentProfilesPanelSource).toContain('getAgentProfileAssignmentsEmptyState');
    expect(agentProfilesPanelSource).not.toContain('No profiles yet. Create one to get started.');
    expect(agentProfilesPanelSource).not.toContain(
      'No agents connected. Install an agent to assign profiles.',
    );
    expect(auditLogPanelSource).toContain('@/utils/upgradePresentation');
    expect(auditWebhookPanelSource).toContain('@/utils/upgradePresentation');
    expect(ssoProvidersPanelSource).toContain('@/utils/upgradePresentation');
    expect(upgradePresentationSource).toContain('export const UPGRADE_ACTION_LABEL');
    expect(upgradePresentationSource).toContain('export const UPGRADE_TRIAL_LABEL');
    expect(upgradePresentationSource).toContain('export const UPGRADE_TRIAL_LINK_CLASS');
    expect(upgradePresentationSource).toContain('export function getUpgradeActionButtonClass');
    expect(organizationAccessPanelSource).toContain('@/utils/organizationRolePresentation');
    expect(organizationAccessPanelSource).toContain('ORGANIZATION_MEMBER_ROLE_OPTIONS');
    expect(organizationAccessPanelSource).not.toContain(
      "const roleOptions: Array<{ value: OrganizationRole; label: string }> = [",
    );
    expect(organizationSharingPanelSource).toContain('@/utils/organizationRolePresentation');
    expect(organizationSharingPanelSource).toContain('ORGANIZATION_SHARE_ROLE_OPTIONS');
    expect(organizationSharingPanelSource).toContain('normalizeOrganizationShareRole');
    expect(organizationSharingPanelSource).not.toContain(
      "const accessRoleOptions: Array<{ value: ShareAccessRole; label: string }> = [",
    );
    expect(organizationSharingPanelSource).not.toContain('const normalizeShareRole =');
    expect(organizationRolePresentationSource).toContain(
      'export const ORGANIZATION_MEMBER_ROLE_OPTIONS',
    );
    expect(organizationRolePresentationSource).toContain(
      'export const ORGANIZATION_SHARE_ROLE_OPTIONS',
    );
    expect(organizationRolePresentationSource).toContain(
      'export function normalizeOrganizationShareRole',
    );
    expect(organizationAccessPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(billingAdminPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(billingAdminPanelSource).toContain('@/utils/licensePresentation');
    expect(organizationOverviewPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationSharingPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationBillingPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationBillingPanelSource).toContain('@/utils/licensePresentation');
    expect(organizationBillingPanelSource).not.toContain('Grace Period');
    expect(organizationBillingPanelSource).not.toContain('No License');
    expect(organizationAccessPanelSource).not.toContain(
      'Multi-tenant requires an Enterprise license',
    );
    expect(organizationOverviewPanelSource).not.toContain(
      'Multi-tenant is not enabled on this server',
    );
    expect(organizationSharingPanelSource).not.toContain(
      'Failed to load organization sharing details',
    );
    expect(organizationAccessPanelSource).toContain('getOrganizationAccessEmptyState');
    expect(organizationOverviewPanelSource).toContain('getOrganizationOverviewMembersEmptyState');
    expect(organizationSharingPanelSource).toContain('getOrganizationOutgoingSharesEmptyState');
    expect(organizationSharingPanelSource).toContain('getOrganizationIncomingSharesEmptyState');
    expect(organizationSharingPanelSource).toContain(
      'getOrganizationShareTargetOrgRequiredMessage',
    );
    expect(organizationSharingPanelSource).toContain(
      'getOrganizationShareCreateSuccessMessage',
    );
    expect(organizationAccessPanelSource).not.toContain('No organization members found.');
    expect(organizationOverviewPanelSource).not.toContain('No members found.');
    expect(organizationSharingPanelSource).not.toContain('No outgoing shares configured.');
    expect(organizationSharingPanelSource).not.toContain(
      'No incoming shares from other organizations.',
    );
    expect(organizationSharingPanelSource).not.toContain(
      "notificationStore.error('Target organization is required')",
    );
    expect(organizationSharingPanelSource).not.toContain(
      "notificationStore.success('Resource shared successfully')",
    );
    expect(organizationSharingPanelSource).not.toContain(
      "notificationStore.success('Share removed')",
    );
    expect(organizationBillingPanelSource).not.toContain('This feature is not available.');
    expect(billingAdminPanelSource).not.toContain('This feature is not available.');
    expect(billingAdminPanelSource).not.toContain('Failed to list organizations');
    expect(billingAdminPanelSource).not.toContain('No trial');
    expect(billingAdminPanelSource).not.toContain('soft-deleted');
    expect(billingAdminPanelSource).not.toContain('Organization billing suspended');
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationSettingsLoadErrorMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      "export type OrganizationSettingsLoadContext =",
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export const ORGANIZATION_SETTINGS_UNAVAILABLE_MESSAGE',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export const ORGANIZATION_SETTINGS_UNAVAILABLE_CLASS',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationAccessEmptyState',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationOverviewMembersEmptyState',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationOutgoingSharesEmptyState',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationIncomingSharesEmptyState',
    );
    expect(rbacPermissionsSource).toContain('export const RBAC_PERMISSION_ACTIONS');
    expect(rbacPermissionsSource).toContain('export const RBAC_PERMISSION_RESOURCES');
    expect(rbacPermissionsSource).toContain('export function createDefaultRBACPermission');
    expect(rolesPanelSource).toContain('@/utils/rbacPresentation');
    expect(rolesPanelSource).toContain('getRBACFeatureGateCopy');
    expect(rolesPanelSource).toContain('getRolesEmptyState');
    expect(rolesPanelSource).not.toContain('Custom Roles (Pro)');
    expect(rolesPanelSource).not.toContain('No roles available.');
    expect(userAssignmentsPanelSource).toContain('@/utils/rbacPresentation');
    expect(userAssignmentsPanelSource).toContain('getRBACFeatureGateCopy');
    expect(userAssignmentsPanelSource).toContain('getUserAssignmentsEmptyStateCopy');
    expect(userAssignmentsPanelSource).not.toContain('Centralized Access Control (Pro)');
    expect(userAssignmentsPanelSource).not.toContain('No users yet');
    expect(userAssignmentsPanelSource).not.toContain('Configure SSO in Security settings');
    expect(userAssignmentsPanelSource).not.toContain('Users sync on first login');
    expect(rbacPresentationSource).toContain('export function getRBACFeatureGateCopy');
    expect(rbacPresentationSource).toContain('export function getRolesEmptyState');
    expect(rbacPresentationSource).toContain(
      'export function getUserAssignmentsEmptyStateCopy',
    );
    expect(agentProfilesPresentationSource).toContain(
      'export function getAgentProfilesEmptyState',
    );
    expect(agentProfilesPresentationSource).toContain(
      'export function getAgentProfileAssignmentsEmptyState',
    );
    expect(trendChartsSource).toContain("segmentedButtonClass(active(), false, 'accent')");
    expect(trendChartsSource).not.toContain(
      "'px-2 py-0.5 rounded bg-blue-600 text-white text-[11px] font-medium'",
    );
    expect(compositionPanelSource).toContain('getDashboardCompositionIcon');
    expect(compositionPanelSource).toContain('DASHBOARD_COMPOSITION_EMPTY_STATE');
    expect(compositionPanelSource).not.toContain('No resources detected');
    expect(compositionPanelSource).not.toContain('const TYPE_ICONS: Record<string, any> =');
    expect(dashboardCompositionPresentationSource).toContain(
      'export const getDashboardCompositionIcon',
    );
    expect(dashboardCompositionPresentationSource).toContain(
      'export const DASHBOARD_COMPOSITION_EMPTY_STATE',
    );
    expect(problemResourcesTableSource).toContain('getProblemResourceStatusVariant');
    expect(problemResourcesTableSource).not.toContain(
      'function statusVariant(pr: ProblemResource)',
    );
    expect(problemResourcePresentationSource).toContain(
      'export function getProblemResourceStatusVariant',
    );
    expect(kpiStripSource).toContain('getDashboardAlertTone');
    expect(kpiStripSource).toContain('getDashboardKpiPresentation');
    expect(kpiStripSource).not.toContain('const alertsTone =');
    expect(kpiStripSource).not.toContain('border-l-[3px] border-l-blue-500');
    expect(dashboardKpiPresentationSource).toContain('export function getDashboardKpiPresentation');
    expect(recentAlertsPanelSource).toContain('getAlertSeverityTextClass');
    expect(recentAlertsPanelSource).toContain('getAlertSeverityCompactLabel');
    expect(recentAlertsPanelSource).toContain('@/utils/dashboardAlertPresentation');
    expect(recentAlertsPanelSource).not.toContain("alert.level === 'critical' ? 'CRIT' : 'WARN'");
    expect(recentAlertsPanelSource).not.toContain('No active alerts');
    expect(dashboardAlertPresentationSource).toContain('export function getDashboardAlertTone');
    expect(dashboardAlertPresentationSource).toContain(
      'export function getDashboardAlertSummaryText',
    );
    expect(dashboardAlertPresentationSource).toContain(
      'export const DASHBOARD_ALERTS_EMPTY_STATE',
    );
    expect(storagePanelSource).toContain('@/utils/dashboardStoragePresentation');
    expect(storagePanelSource).not.toContain('No storage resources');
    expect(recoveryStatusPanelSource).toContain('@/utils/dashboardRecoveryPresentation');
    expect(recoveryStatusPanelSource).not.toContain('No recovery data available');
    expect(recoveryStatusPanelSource).not.toContain('Last recovery point over 24 hours ago');
    expect(dashboardStoragePresentationSource).toContain(
      'export function computeDashboardStorageCapacityPercent',
    );
    expect(dashboardStoragePresentationSource).toContain(
      'export function getDashboardStorageIssueBadges',
    );
    expect(dashboardRecoveryPresentationSource).toContain(
      'export const DASHBOARD_RECOVERY_EMPTY_STATE',
    );
    expect(diskListSource).toContain('getTemperatureTextClass');
    expect(diskListSource).not.toContain('const getTemperatureTone =');
    expect(diskListSource).toContain('getPhysicalDiskHealthStatus');
    expect(diskListSource).toContain('getPhysicalDiskHealthSummary');
    expect(diskListSource).toContain('getPhysicalDiskHostLabel');
    expect(diskListSource).toContain('getPhysicalDiskEmptyStatePresentation');
    expect(diskListSource).toContain('getPhysicalDiskRoleLabel');
    expect(diskListSource).toContain('getPhysicalDiskParentLabel');
    expect(diskListSource).toContain('getPhysicalDiskSourceBadgePresentation');
    expect(diskListSource).not.toContain('const titleize =');
    expect(diskListSource).not.toContain('const platformLabel =');
    expect(diskListSource).not.toContain('function hasSmartWarning(');
    expect(diskListSource).not.toContain('const getDiskHealthStatus =');
    expect(diskListSource).not.toContain('const getDiskRoleLabel =');
    expect(diskListSource).not.toContain('const getDiskParentLabel =');
    expect(diskListSource).not.toContain('const healthSummary = () => {');
    expect(diskListSource).not.toContain('const hostLabel = () =>');
    expect(diskListSource).not.toContain('No physical disks found');
    expect(diskListSource).not.toContain('Physical disk monitoring requirements:');
    expect(diskListSource).toContain('useDiskListModel');
    expect(diskListSource).toContain('PHYSICAL_DISK_TABLE_CLASS');
    expect(diskListSource).not.toContain('cursor-pointer transition-colors');
    expect(diskListSource).not.toContain('function extractDiskData(');
    expect(diskListSource).not.toContain('const filteredDisks = createMemo(() => {');
    expect(diskListSource).not.toContain('const diskDataById = createMemo(() => {');
    expect(storagePoolDetailSource).toContain('getLinkedDiskHealthDotClass');
    expect(storagePoolDetailSource).toContain('getLinkedDiskTemperatureTextClass');
    expect(storagePoolDetailSource).toContain('getZfsScanTextClass');
    expect(storagePoolDetailSource).toContain('getZfsErrorTextClass');
    expect(storagePoolDetailSource).toContain('StorageDetailKeyValueRow');
    expect(storagePoolDetailSource).toContain('STORAGE_DETAIL_CARD_CLASS');
    expect(storagePoolDetailSource).toContain('useStoragePoolDetailModel');
    expect(storagePoolDetailSource).toContain('STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS');
    expect(storagePoolDetailSource).not.toContain('const ConfigRow: Component');
    expect(storagePoolDetailSource).not.toContain('const chartResourceType = createMemo');
    expect(storagePoolDetailSource).not.toContain('const chartResourceId = createMemo');
    expect(storagePoolDetailSource).not.toContain('const poolDisks = createMemo(() => {');
    expect(storagePoolDetailSource).not.toContain("'bg-yellow-500'");
    expect(storagePoolDetailSource).not.toContain("'bg-green-500'");
    expect(storagePoolDetailSource).not.toContain("'text-red-500'");
    expect(storagePoolDetailSource).not.toContain("'text-yellow-500'");
    expect(storagePoolDetailSource).not.toContain("'text-yellow-600 dark:text-yellow-400 italic'");
    expect(storagePoolDetailSource).not.toContain("'text-red-600 dark:text-red-400 font-medium'");
    expect(diskDetailSource).toContain('getDiskAttributeValueTextClass');
    expect(diskDetailSource).toContain('getDiskDetailAttributeCards');
    expect(diskDetailSource).toContain('getDiskDetailHistoryCharts');
    expect(diskDetailSource).toContain('getDiskDetailHistoryFallbackMessage');
    expect(diskDetailSource).toContain('getDiskDetailLiveBadgeLabel');
    expect(diskDetailSource).toContain('DISK_DETAIL_HISTORY_RANGE_OPTIONS');
    expect(diskDetailSource).toContain('DISK_DETAIL_LIVE_CHARTS');
    expect(diskDetailSource).toContain('StorageDetailMetricCard');
    expect(diskDetailSource).toContain('STORAGE_DETAIL_CARD_CLASS');
    expect(diskDetailSource).toContain('STORAGE_DISK_DETAIL_HEADER_CLASS');
    expect(diskDetailSource).toContain('useDiskDetailModel');
    expect(diskDetailSource).not.toContain('const AttrCard: Component');
    expect(diskDetailSource).not.toContain('function extractDiskData(');
    expect(diskDetailSource).not.toContain('const getMetricResourceId = () => {');
    expect(diskDetailSource).not.toContain('function attrColor(');
    expect(diskDetailSource).not.toContain('flex flex-wrap items-end justify-between gap-3 border-b border-border-subtle pb-3');
    expect(diskDetailSource).not.toContain('class="relative"');
    expect(useDiskDetailModelSource).toContain('extractPhysicalDiskPresentationData');
    expect(useDiskDetailModelSource).toContain('resolvePhysicalDiskMetricResourceId');
    expect(diskLiveMetricSource).toContain('useDiskLiveMetricModel');
    expect(diskLiveMetricSource).not.toContain('const latestMetric = createMemo(() => {');
    expect(diskLiveMetricSource).not.toContain('const formatted = createMemo(() => {');
    expect(diskLiveMetricSource).not.toContain("if (v > 90) return 'text-red-600 dark:text-red-400 font-bold'");
    expect(useDiskLiveMetricModelSource).toContain('getDiskLiveMetricFormattedValue');
    expect(useDiskLiveMetricModelSource).toContain('getDiskLiveMetricTextClass');
    expect(storagePoolRowSource).toContain('getStoragePoolProtectionTextClass');
    expect(storagePoolRowSource).toContain('getStoragePoolIssueTextClass');
    expect(storagePoolRowSource).toContain('buildStoragePoolRowModel');
    expect(storagePoolRowSource).toContain('STORAGE_POOL_ROW_CLASS');
    expect(storagePoolRowSource).not.toContain('getSourcePlatformBadge');
    expect(storagePoolRowSource).not.toContain('const totalBytes = createMemo(');
    expect(storagePoolRowSource).not.toContain('const usedBytes = createMemo(');
    expect(storagePoolRowSource).not.toContain('const freeBytes = createMemo(');
    expect(storagePoolRowSource).not.toContain('const platformLabel = createMemo(');
    expect(storagePoolRowSource).not.toContain('const hostLabel = createMemo(');
    expect(storagePoolRowSource).not.toContain('const topologyLabel = createMemo(');
    expect(storagePoolRowPresentationSource).toContain(
      'export const buildStoragePoolRowModel',
    );
    expect(storagePoolRowPresentationSource).toContain(
      'getSourcePlatformPresentation',
    );
    expect(storagePoolRowPresentationSource).toContain(
      'getCompactStoragePoolProtectionLabel',
    );
    expect(storagePoolRowPresentationSource).toContain(
      'getCompactStoragePoolImpactLabel',
    );
    expect(storagePoolRowPresentationSource).toContain(
      'getCompactStoragePoolIssueLabel',
    );
    expect(storagePoolRowPresentationSource).toContain(
      'getCompactStoragePoolIssueSummary',
    );
    expect(storagePoolRowPresentationSource).toContain(
      'getCompactStoragePoolProtectionTitle',
    );
    expect(storagePoolRowSource).not.toContain('const protectionTextClass =');
    expect(storagePoolRowSource).not.toContain('const issueTextClass =');
    expect(storagePoolRowSource).not.toContain('const compactProtection = createMemo(() => {');
    expect(storagePoolRowSource).not.toContain('const compactImpact = createMemo(() => {');
    expect(storagePoolRowSource).not.toContain('const compactIssue = createMemo(() => {');
    expect(storagePoolRowSource).not.toContain('const compactIssueSummary = createMemo(() => {');
    expect(storagePageSource).toContain("from './storagePageState'");
    expect(storagePageSource).toContain('useStoragePageModel');
    expect(storagePageSource).toContain('isStorageRecordCeph');
    expect(storagePageSource).toContain('StorageCephSection');
    expect(storagePageSource).toContain('StoragePageControls');
    expect(storagePageSource).toContain('StoragePageSummary');
    expect(storagePageSource).not.toContain('const normalizeHealthFilter =');
    expect(storagePageSource).not.toContain('const normalizeSortKey =');
    expect(storagePageSource).not.toContain('const normalizeGroupKey =');
    expect(storagePageSource).not.toContain('const normalizeView =');
    expect(storagePageSource).not.toContain('const normalizeSortDirection =');
    expect(storagePageSource).not.toContain('const getStorageMetaBoolean =');
    expect(storagePageSource).not.toContain('const isRecordCeph =');
    expect(storagePageSource).not.toContain('Reconnecting to backend data stream…');
    expect(storagePageSource).not.toContain('Storage data stream disconnected. Data may be stale.');
    expect(storagePageSource).not.toContain('Waiting for storage data from connected platforms.');
    expect(useStorageCephModelSource).toContain(
      "from '@/features/storageBackups/cephRecordPresentation'",
    );
    expect(useStorageCephModelSource).toContain('getCephClusterKeyFromStorageRecord');
    expect(useStorageCephModelSource).toContain('getCephSummaryText');
    expect(useStorageCephModelSource).toContain('getCephPoolsText');
    expect(useStorageCephModelSource).toContain(
      "from '@/features/storageBackups/cephSummaryPresentation'",
    );
    expect(useStorageCephModelSource).toContain('deriveCephClustersFromStorageRecords');
    expect(useStorageCephModelSource).toContain('summarizeCephClusters');
    expect(useStorageCephModelSource).toContain('buildExplicitCephClusters');
    expect(useStorageCephModelSource).toContain('buildCephClusterLookup');
    expect(useStorageCephModelSource).toContain('resolveCephClusterForStorageRecord');
    expect(useStorageCephModelSource).not.toContain('export const isCephRecord =');
    expect(useStorageCephModelSource).not.toContain('export const getCephClusterKeyFromRecord =');
    expect(useStorageCephModelSource).not.toContain('return resources.map((r) => {');
    expect(cephSummaryPresentationSource).toContain(
      'export const deriveCephClustersFromStorageRecords',
    );
    expect(cephSummaryPresentationSource).toContain('export const summarizeCephClusters');
    expect(cephSummaryPresentationSource).toContain('export const buildExplicitCephClusters');
    expect(cephSummaryPresentationSource).toContain('export const buildCephClusterLookup');
    expect(cephSummaryPresentationSource).toContain(
      'export const resolveCephClusterForStorageRecord',
    );
    expect(storageRowPresentationSource).toContain(
      'export function getStoragePoolProtectionTextClass',
    );
    expect(storageRowPresentationSource).toContain(
      'export function getStoragePoolIssueTextClass',
    );
    expect(storageRowPresentationSource).toContain(
      'export function getCompactStoragePoolProtectionLabel',
    );
    expect(storageRowPresentationSource).toContain(
      'export function getCompactStoragePoolImpactLabel',
    );
    expect(storageRowPresentationSource).toContain(
      'export function getCompactStoragePoolIssueLabel',
    );
    expect(storageRowPresentationSource).toContain(
      'export function getCompactStoragePoolIssueSummary',
    );
    expect(storageRowPresentationSource).toContain(
      'export function getCompactStoragePoolProtectionTitle',
    );
    expect(storageGroupRowSource).toContain("from '@/features/storageBackups/groupPresentation'");
    expect(storageGroupRowSource).toContain('buildStorageGroupRowPresentation');
    expect(storageGroupRowSource).toContain('STORAGE_GROUP_ROW_CLASS');
    expect(storageGroupRowSource).not.toContain('cursor-pointer select-none bg-surface-alt');
    expect(storageGroupPresentationSource).toContain(
      'export const getStorageGroupHealthCountPresentation',
    );
    expect(storageGroupPresentationSource).toContain(
      'export const getStorageGroupPoolCountLabel',
    );
    expect(storageGroupPresentationSource).toContain(
      'export const getStorageGroupUsagePercentLabel',
    );
    expect(storageGroupPresentationSource).toContain(
      'export const buildStorageGroupRowPresentation',
    );
    expect(zfsHealthMapSource).toContain('getZfsDeviceBlockClass');
    expect(zfsHealthMapSource).toContain('getZfsDeviceStateTextClass');
    expect(zfsHealthMapSource).toContain('getZfsHealthMapDeviceClass');
    expect(zfsHealthMapSource).toContain('getZfsHealthMapErrorSummaryClass');
    expect(zfsHealthMapSource).toContain('getZfsHealthMapMessageClass');
    expect(zfsHealthMapSource).toContain('useZFSHealthMapModel');
    expect(zfsHealthMapSource).not.toContain("const getDeviceColor = (device: ZFSDevice) => {");
    expect(zfsHealthMapSource).not.toContain('const isResilvering = (device: ZFSDevice) => {');
    expect(enhancedStorageBarSource).toContain('getZfsPoolStateTextClass');
    expect(enhancedStorageBarSource).toContain('getZfsPoolErrorOverlayClass');
    expect(enhancedStorageBarSource).toContain('getZfsScanTextClass');
    expect(enhancedStorageBarSource).toContain('getZfsErrorTextClass');
    expect(enhancedStorageBarSource).toContain('getStorageBarTooltipRowClass');
    expect(enhancedStorageBarSource).toContain('getStorageBarZfsHeadingLabel');
    expect(enhancedStorageBarSource).toContain('useEnhancedStorageBarModel');
    expect(enhancedStorageBarSource).toContain('STORAGE_BAR_ROOT_CLASS');
    expect(enhancedStorageBarSource).not.toContain('const usagePercent = createMemo(() => {');
    expect(enhancedStorageBarSource).not.toContain('const isScrubbing = createMemo(() => {');
    expect(enhancedStorageBarSource).not.toContain('const isResilvering = createMemo(() => {');
    expect(enhancedStorageBarSource).not.toContain('const hasErrors = createMemo(() => {');
    expect(enhancedStorageBarSource).not.toContain('metric-text w-full h-5 flex items-center min-w-0');
    expect(useEnhancedStorageBarModelSource).toContain('getStorageBarUsagePercent');
    expect(useEnhancedStorageBarModelSource).toContain('getStorageBarLabel');
    expect(useEnhancedStorageBarModelSource).toContain('getStorageBarTooltipRows');
    expect(useEnhancedStorageBarModelSource).toContain('getStorageBarZfsSummary');
    expect(storagePagePresentationSource).toContain('export const shouldShowCephSummaryCard');
    expect(storagePagePresentationSource).toContain(
      'export const getStoragePageBannerMessage',
    );
    expect(storagePagePresentationSource).toContain('export const STORAGE_VIEW_OPTIONS');
    expect(storagePagePresentationSource).toContain('export const STORAGE_POOL_TABLE_COLUMNS');
    expect(storagePagePresentationSource).toContain(
      'export const STORAGE_BANNER_ACTION_BUTTON_CLASS',
    );
    expect(storagePagePresentationSource).toContain('export const getStorageTableHeading');
    expect(storagePagePresentationSource).toContain('export const getStorageLoadingMessage');
    expect(storagePageStatusSource).toContain('export const getStoragePageBannerKind');
    expect(storagePageStatusSource).toContain('export const isStoragePoolLoading');
    expect(storagePageStatusHookSource).toContain('export const useStoragePageStatus');
    expect(storagePageStatusHookSource).toContain('getStoragePageBannerKind');
    expect(storagePageStatusHookSource).toContain('isStoragePoolLoading');
    expect(storagePageDataSource).toContain('export const useStoragePageData');
    expect(storagePageDataSource).toContain('buildStorageRecords');
    expect(storagePageDataSource).toContain('useStorageAlertState');
    expect(storagePageDataSource).toContain('useStorageCephModel');
    expect(storagePageDataSource).toContain('useStorageModel');
    expect(storagePageResourcesSource).toContain('export const useStoragePageResources');
    expect(storagePageResourcesSource).toContain('useWebSocket');
    expect(storagePageResourcesSource).toContain('useResources');
    expect(storagePageResourcesSource).toContain('useStorageRecoveryResources');
    expect(storagePageResourcesSource).toContain('useAlertsActivation');
    expect(storagePageModelSource).toContain('export const useStoragePageModel');
    expect(storagePageModelSource).toContain('useStoragePageResources');
    expect(storagePageModelSource).toContain('useStoragePageFilters');
    expect(storagePageModelSource).toContain('useStoragePageData');
    expect(storagePageModelSource).toContain('useStorageExpansionState');
    expect(storagePageModelSource).toContain('useStorageFilterState');
    expect(storagePageModelSource).toContain('useStoragePageStatus');
    expect(storagePageModelSource).toContain('useStorageResourceHighlight');
    expect(storagePageSummaryHookSource).toContain('export const useStoragePageSummary');
    expect(storagePageSummaryHookSource).toContain('countVisiblePhysicalDisksForNode');
    expect(storagePageFiltersSource).toContain('export const useStoragePageFilters');
    expect(storagePageFiltersSource).toContain('useStorageRouteState');
    expect(storagePageFiltersSource).toContain('buildStorageRouteFields');
    expect(storagePageStateSource).toContain('export const buildStorageRouteFields');
    expect(storageFilterStateSource).toContain('export const useStorageFilterState');
    expect(storageFilterStateSource).toContain('getStorageFilterGroupBy');
    expect(storageFilterStateSource).toContain('getStorageStatusFilterValue');
    expect(storageFilterStateSource).toContain('toStorageHealthFilterValue');
    expect(storagePageSource).toContain('StorageContentCard');
    expect(storagePageSource).toContain('StoragePageBanners');
    expect(storagePageSource).toContain('StoragePageSummary');
    expect(storageControlsSource).toContain('export const StorageControls');
    expect(storageControlsSource).toContain('StorageFilter');
    expect(storageControlsSource).toContain('Subtabs');
    expect(storageControlsSource).toContain('useStorageControlsModel');
    expect(storageControlsSource).toContain('STORAGE_CONTROLS_NODE_SELECT_CLASS');
    expect(storageControlsSource).toContain('STORAGE_CONTROLS_NODE_DIVIDER_CLASS');
    expect(storageControlsSource).toContain('DEFAULT_STORAGE_SORT_OPTIONS');
    expect(storageControlsSource).not.toContain('STORAGE_VIEW_OPTIONS');
    expect(storageControlsSource).not.toContain("props.setSelectedNodeId(event.currentTarget.value)");
    expect(storageControlsSource).not.toContain('focus:ring-blue-500');
    expect(useStorageControlsModelSource).toContain('STORAGE_VIEW_OPTIONS');
    expect(useStorageControlsModelSource).toContain('handleNodeFilterChange');
    expect(useStorageControlsModelSource).toContain('handleViewChange');
    expect(storageFilterSource).toContain('useStorageFilterToolbarModel');
    expect(storageFilterSource).toContain('STORAGE_FILTER_SORT_SELECT_CLASS');
    expect(storageFilterSource).toContain('STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS');
    expect(storageFilterSource).not.toContain('const activeFilterCount = createMemo(() => {');
    expect(storageFilterSource).not.toContain('const sourceOptions = (): StorageSourceOption[] =>');
    expect(storageFilterSource).not.toContain('props.setSortKey(DEFAULT_STORAGE_SORT_KEY)');
    expect(storageFilterSource).not.toContain("props.sortDirection() === 'asc' ? 'desc' : 'asc'");
    expect(storageFilterSource).not.toContain("props.sortDirection() === 'asc' ? 'rotate-180' : ''");
    expect(storageFilterSource).not.toContain('focus:ring-blue-500');
    expect(useStorageFilterToolbarModelSource).toContain('countActiveStorageFilters');
    expect(useStorageFilterToolbarModelSource).toContain('hasActiveStorageFilters');
    expect(useStorageFilterToolbarModelSource).toContain('DEFAULT_STORAGE_SORT_KEY');
    expect(useStorageFilterToolbarModelSource).toContain('DEFAULT_STORAGE_SOURCE_FILTER');
    expect(useStorageFilterToolbarModelSource).toContain('getStorageSortDirectionTitle');
    expect(useStorageFilterToolbarModelSource).toContain('getStorageSortDirectionIconClass');
    expect(useStorageFilterToolbarModelSource).toContain('getNextStorageSortDirection');
    expect(storageFilterPresentationSource).toContain(
      'export const getStorageSortDirectionTitle',
    );
    expect(storageFilterPresentationSource).toContain(
      'export const getStorageSortDirectionIconClass',
    );
    expect(storageFilterPresentationSource).toContain(
      'export const STORAGE_FILTER_SORT_SELECT_CLASS',
    );
    expect(storageFilterPresentationSource).toContain(
      'export const STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS',
    );
    expect(storagePageControlsSource).toContain('export const StoragePageControls');
    expect(storagePageControlsSource).toContain('StorageControls');
    expect(storagePageControlsSource).toContain('useStoragePageControlsModel');
    expect(storagePageControlsSource).not.toContain('normalizeStorageSortKey');
    expect(storagePageControlsSource).not.toContain("props.view() === 'pools' ? props.storageFilterGroupBy : undefined");
    expect(storagePageControlsSource).not.toContain("props.view() !== 'pools'");
    expect(useStoragePageControlsModelSource).toContain('normalizeStorageSortKey');
    expect(useStoragePageControlsModelSource).toContain("options.view() === 'pools'");
    expect(storageCephSectionSource).toContain('export const StorageCephSection');
    expect(storageCephSectionSource).toContain('useStorageCephSectionModel');
    expect(storageCephSectionSource).toContain('StorageCephSummaryCard');
    expect(storagePageBannerSource).toContain('useStoragePageBannerModel');
    expect(storagePageBannerSource).toContain('STORAGE_BANNER_ACTION_BUTTON_CLASS');
    expect(storagePageBannerSource).toContain('STORAGE_PAGE_BANNER_ROW_CLASS');
    expect(storagePageBannerSource).not.toContain('const actionLabel = () =>');
    expect(storagePageBannerSource).not.toContain('text-xs text-amber-800 dark:text-amber-200');
    expect(useStoragePageBannerModelSource).toContain('getStoragePageBannerMessage');
    expect(useStoragePageBannerModelSource).toContain('getStoragePageBannerActionLabel');
    expect(storagePageBannersSource).toContain('export const StoragePageBanners');
    expect(storagePageBannersSource).toContain('StoragePageBanner');
    expect(storagePageBannersSource).toContain('useStoragePageBannersModel');
    expect(storagePageBannersSource).not.toContain("props.kind() === 'reconnecting'");
    expect(useStoragePageBannersModelSource).toContain("kind === 'reconnecting' || kind === 'disconnected'");
    expect(storagePageSummarySource).toContain('export const StoragePageSummary');
    expect(storagePageSummarySource).toContain('useStoragePageSummary');
    expect(storagePageSummarySource).toContain('StorageSummary');
    expect(storageCephSummaryCardSource).toContain('useStorageCephSummaryCardModel');
    expect(storageCephSummaryCardSource).toContain('CEPH_SUMMARY_CARD_GRID_CLASS');
    expect(storageCephSummaryCardSource).toContain('CEPH_SUMMARY_CARD_HEALTH_BADGE_CLASS');
    expect(storageCephSummaryCardSource).not.toContain('const summary = () => props.summary');
    expect(storageCephSummaryCardSource).not.toContain('rounded-md border border-border bg-surface p-3');
    expect(storageCephSummaryCardSource).not.toContain('text-[11px] text-muted truncate max-w-[240px]');
    expect(storageCephSummaryCardSource).not.toContain('flex flex-wrap items-center justify-between gap-3');
    expect(useStorageCephSummaryCardModelSource).toContain('getCephSummaryHeaderPresentation');
    expect(useStorageCephSummaryCardModelSource).toContain('getCephSummaryClusterCards');
    expect(storageContentCardSource).toContain('export const StorageContentCard');
    expect(storageContentCardSource).toContain('useStorageContentCardModel');
    expect(storageContentCardSource).toContain('DiskList');
    expect(storageContentCardSource).toContain('StoragePoolsTable');
    expect(storageContentCardSource).toContain('STORAGE_CONTENT_CARD_HEADER_CLASS');
    expect(storageContentCardSource).not.toContain("props.selectedNodeId() === 'all' ? null : props.selectedNodeId()");
    expect(storageContentCardSource).not.toContain('border-b border-border bg-surface-hover px-3 py-2');
    expect(useStorageContentCardModelSource).toContain('getStorageTableHeading');
    expect(useStorageContentCardModelSource).toContain("options.view() === 'disks'");
    expect(useStorageCephSectionModelSource).toContain('shouldShowCephSummaryCard');
    expect(storagePoolsTableSource).toContain('export const StoragePoolsTable');
    expect(storagePoolsTableSource).toContain('STORAGE_POOL_TABLE_COLUMNS');
    expect(storagePoolsTableSource).toContain('getStorageLoadingMessage');
    expect(storagePoolsTableSource).toContain('useStoragePoolsTableModel');
    expect(storagePoolsTableSource).toContain('StoragePoolRow');
    expect(storagePoolsTableSource).toContain('StorageGroupRow');
    expect(storagePoolsTableSource).toContain('STORAGE_POOLS_EMPTY_STATE_CLASS');
    expect(storagePoolsTableSource).not.toContain('const group = createMemo(');
    expect(storagePoolsTableSource).not.toContain('const groupItems = createMemo(');
    expect(storagePoolsTableSource).not.toContain('const rowAlertPresentation = createMemo(');
    expect(storagePoolsTableSource).not.toContain('getStorageRecordNodeLabel');
    expect(storagePoolsTablePresentationSource).toContain(
      'export const buildStoragePoolsTableGroups',
    );
    expect(storagePoolsTablePresentationSource).toContain(
      'export const buildStoragePoolsTableRowModel',
    );
    expect(storagePoolsTablePresentationSource).toContain(
      'getStorageRowAlertPresentation',
    );
    expect(useStoragePoolsTableModelSource).toContain('buildStoragePoolsTableGroups');
    expect(useStoragePoolsTableModelSource).toContain('buildStoragePoolsTableRowModel');
    expect(useStoragePoolsTableModelSource).toContain('togglePool');
    expect(storageResourceHighlightSource).toContain('export const useStorageResourceHighlight');
    expect(storageResourceHighlightSource).toContain('findHighlightedStorageRecord');
    expect(storageExpansionStateSource).toContain('export const useStorageExpansionState');
    expect(storageExpansionStateSource).toContain('syncExpandedStorageGroups');
    expect(storageExpansionStateSource).toContain('toggleExpandedStorageGroup');
    expect(storagePageSource).not.toContain('const rowClass = createMemo(() => {');
    expect(storagePageSource).not.toContain('const rowStyle = createMemo(() => {');
    expect(storagePageSource).not.toContain('const isWaitingForData = createMemo(');
    expect(storagePageSource).not.toContain('const isDisconnectedAfterLoad = createMemo(');
    expect(storagePageSource).not.toContain('const [handledResourceId, setHandledResourceId]');
    expect(storagePageSource).not.toContain('const [expandedGroups, setExpandedGroups]');
    expect(storagePageSource).not.toContain('const activeAlertsAccessor = () => {');
    expect(storagePageSource).not.toContain('const adapterResources = createMemo(() => {');
    expect(storagePageSource).not.toContain('const [summaryTimeRange, setSummaryTimeRange]');
    expect(storagePageSource).not.toContain("buildPath: buildStoragePath");
    expect(storagePageSource).not.toContain('const storageFilterGroupBy =');
    expect(storagePageSource).not.toContain('const storageFilterStatus =');
    expect(storagePageSource).not.toContain('const setStorageFilterStatus =');
    expect(storagePageSource).not.toContain('const records = createMemo(() => buildStorageRecords');
    expect(storagePageSource).not.toContain('const { byType } = useResources()');
    expect(storagePageSource).not.toContain('const storageRecoveryResources = useStorageRecoveryResources()');
    expect(storagePageSource).not.toContain('useStoragePageFilters({');
    expect(storagePageSource).not.toContain('useStoragePageData({');
    expect(storagePageSource).not.toContain('useStoragePageResources()');
    expect(storagePageSource).not.toContain('fields: {');
    expect(storagePageSource).not.toContain('rounded border border-amber-300 bg-amber-100 px-2 py-1');
    expect(storagePageSource).not.toContain('flex flex-wrap items-center justify-between gap-3');
    expect(storagePageSource).not.toContain('<Table class="w-full text-xs">');
    expect(storagePageSource).not.toContain('<StoragePageBanner kind=');
    expect(storagePageSource).not.toContain('<StorageCephSummaryCard summary=');
    expect(storageRowAlertPresentationSource).toContain(
      'export const getStorageRowAlertPresentation',
    );
    expect(diskLiveMetricPresentationSource).toContain(
      'export const getDiskLiveMetricTextClass',
    );
    expect(diskLiveMetricPresentationSource).toContain(
      'export const getDiskLiveMetricFormattedValue',
    );
    expect(diskPresentationSource).toContain(
      'export function extractPhysicalDiskPresentationData',
    );
    expect(diskPresentationSource).toContain('export function matchesPhysicalDiskSearch');
    expect(diskPresentationSource).toContain('export function comparePhysicalDiskPresentation');
    expect(diskPresentationSource).toContain(
      'export function buildPhysicalDiskPresentationDataMap',
    );
    expect(diskPresentationSource).toContain('export function filterAndSortPhysicalDisks');
    expect(diskDetailPresentationSource).toContain('export function getDiskDetailAttributeCards');
    expect(diskDetailPresentationSource).toContain('export function getDiskDetailHistoryCharts');
    expect(diskDetailPresentationSource).toContain('export const DISK_DETAIL_HISTORY_RANGE_OPTIONS');
    expect(diskDetailPresentationSource).toContain('export const DISK_DETAIL_LIVE_CHARTS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_CARD_CLASS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_SELECT_CLASS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_EMPTY_CLASS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_ROW_CLASS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DISK_DETAIL_HEADER_CLASS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_FULL_WIDTH_ROW_CLASS');
    expect(storageDetailKeyValueRowSource).toContain('STORAGE_DETAIL_KEY_VALUE_ROW_CLASS');
    expect(storageDetailMetricCardSource).toContain('STORAGE_DETAIL_CARD_CLASS');
    expect(storagePoolDetailSource).toContain('STORAGE_DETAIL_ROW_CLASS');
    expect(storagePoolDetailSource).not.toContain('border-t border-border');
    expect(storagePoolDetailSource).not.toContain('col-span-2');
    expect(storagePoolDetailSource).not.toContain('text-base-content truncate flex-1');
    expect(zfsPresentationSource).toContain('export const getZfsDeviceBlockClass');
    expect(zfsPresentationSource).toContain('export const getZfsDeviceStateTextClass');
    expect(zfsHealthMapPresentationSource).toContain('export const getZfsHealthMapDevices');
    expect(zfsHealthMapPresentationSource).toContain(
      'export const isZfsHealthMapDeviceResilvering',
    );
    expect(zfsHealthMapPresentationSource).toContain(
      'export const getZfsHealthMapTooltipPresentation',
    );
    expect(zfsHealthMapPresentationSource).toContain(
      'export const getZfsHealthMapDeviceClass',
    );
    expect(zfsHealthMapPresentationSource).toContain(
      'export const getZfsHealthMapErrorSummaryClass',
    );
    expect(zfsHealthMapPresentationSource).toContain(
      'export const getZfsHealthMapMessageClass',
    );
    expect(zfsHealthMapPresentationSource).toContain(
      'export const ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS',
    );
    expect(useZFSHealthMapModelSource).toContain('getZfsHealthMapDevices');
    expect(useZFSHealthMapModelSource).toContain('isZfsHealthMapDeviceResilvering');
    expect(useZFSHealthMapModelSource).toContain('getZfsHealthMapTooltipPresentation');
    expect(zfsHealthMapSource).toContain('ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS');
    expect(zfsHealthMapSource).not.toContain('fixed z-[9999] pointer-events-none');
    expect(zfsHealthMapSource).not.toContain('bg-surface text-base-content text-[10px] rounded-md shadow-sm px-2 py-1.5 min-w-[120px] border border-border');
    expect(cephSummaryCardPresentationSource).toContain(
      'export const getCephSummaryHeaderPresentation',
    );
    expect(cephSummaryCardPresentationSource).toContain(
      'export const getCephSummaryClusterCards',
    );
    expect(cephSummaryCardPresentationSource).toContain(
      'export const CEPH_SUMMARY_CARD_GRID_CLASS',
    );
    expect(zfsPresentationSource).toContain('export const getZfsPoolStateTextClass');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskHealthStatus');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskHealthSummary');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskHostLabel');
    expect(diskPresentationSource).toContain(
      'export function getPhysicalDiskEmptyStatePresentation',
    );
    expect(diskPresentationSource).toContain('export const PHYSICAL_DISK_TABLE_CLASS');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskRoleLabel');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskParentLabel');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskPlatformLabel');
    expect(diskPresentationSource).toContain('export function hasPhysicalDiskSmartWarning');
    expect(useDiskListModelSource).toContain('buildPhysicalDiskPresentationDataMap');
    expect(useDiskListModelSource).toContain('filterAndSortPhysicalDisks');
    expect(useDiskListModelSource).toContain('matchesPhysicalDiskNode');
    expect(storageAdaptersSource).toContain(
      "from './resourceStoragePresentation'",
    );
    expect(storageAdaptersSource).toContain("from './resourceStorageMapping'");
    expect(storageAdaptersSource).toContain("from './storageAdapterCore'");
    expect(storageAdaptersSource).toContain('asNumberOrNull');
    expect(storageAdaptersSource).toContain('buildStorageCapacity');
    expect(storageAdaptersSource).toContain('buildStorageSource');
    expect(storageAdaptersSource).toContain('canonicalStorageIdentityKey');
    expect(storageAdaptersSource).toContain('dedupe');
    expect(storageAdaptersSource).toContain('getStringArray');
    expect(storageAdaptersSource).toContain('metricsTargetForStorageResource');
    expect(storageAdaptersSource).toContain('normalizeStorageResourceHealth');
    expect(storageAdaptersSource).toContain('getCanonicalStoragePlatformKey');
    expect(storageAdaptersSource).toContain('readResourceStorageMeta');
    expect(storageAdaptersSource).toContain('resolveResourceStorageContent');
    expect(storageAdaptersSource).toContain('getStorageCapabilitiesForResource');
    expect(storageAdaptersSource).toContain('getStorageCategoryFromType');
    expect(storageAdaptersSource).toContain('getResourceStoragePlatformLabel');
    expect(storageAdaptersSource).toContain('getResourceStorageTopologyLabel');
    expect(storageAdaptersSource).toContain('getResourceStorageIssueLabel');
    expect(storageAdaptersSource).toContain('getResourceStorageIssueSummary');
    expect(storageAdaptersSource).toContain('getResourceStorageImpactSummary');
    expect(storageAdaptersSource).toContain('getResourceStorageActionSummary');
    expect(storageAdaptersSource).toContain('getResourceStorageProtectionLabel');
    expect(storageAdaptersSource).not.toContain('const platformLabelForResource =');
    expect(storageAdaptersSource).not.toContain('const topologyLabelForResource =');
    expect(storageAdaptersSource).not.toContain('const issueLabelForResource =');
    expect(storageAdaptersSource).not.toContain('const issueSummaryForResource =');
    expect(storageAdaptersSource).not.toContain('const impactSummaryForResource =');
    expect(storageAdaptersSource).not.toContain('const actionSummaryForResource =');
    expect(storageAdaptersSource).not.toContain('const protectionLabelForResource =');
    expect(storageAdaptersSource).not.toContain('type ResourceStorageMeta =');
    expect(storageAdaptersSource).not.toContain('const normalizeStorageMeta =');
    expect(storageAdaptersSource).not.toContain('const readResourceStorageMeta =');
    expect(storageAdaptersSource).not.toContain('const resolveStorageContent =');
    expect(storageAdaptersSource).not.toContain('const capabilitiesForStorage =');
    expect(storageAdaptersSource).not.toContain('const categoryFromStorageType =');
    expect(storageAdaptersSource).not.toContain('const asNumberOrNull =');
    expect(storageAdaptersSource).not.toContain('const dedupe =');
    expect(storageAdaptersSource).not.toContain('const normalizeIdentityPart =');
    expect(storageAdaptersSource).not.toContain('const getStringArray =');
    expect(storageAdaptersSource).not.toContain('const canonicalStorageIdentityKey =');
    expect(storageAdaptersSource).not.toContain('const resolvePlatformFamily =');
    expect(storageAdaptersSource).not.toContain('const fromSource =');
    expect(storageAdaptersSource).not.toContain('const capacity =');
    expect(storageAdaptersSource).not.toContain('const metricsTargetForResource =');
    expect(storageAdaptersSource).not.toContain('const extractHealthTag =');
    expect(storageAdaptersSource).not.toContain('const normalizeHealthValue =');
    expect(storageAdaptersSource).not.toContain('const normalizeResourceHealth =');
    expect(useStorageModelSource).toContain('@/features/storageBackups/storageModelCore');
    expect(useStorageModelSource).toContain('buildStorageSourceOptions');
    expect(useStorageModelSource).toContain('filterStorageRecords');
    expect(useStorageModelSource).toContain('sortStorageRecords');
    expect(useStorageModelSource).toContain('groupStorageRecords');
    expect(useStorageModelSource).toContain('summarizeStorageRecords');
    expect(useStorageModelSource).not.toContain('const getRecordDetails =');
    expect(useStorageModelSource).not.toContain('const getRecordStringDetail =');
    expect(useStorageModelSource).not.toContain('const getRecordStringArrayDetail =');
    expect(useStorageModelSource).not.toContain('export const getRecordNodeHints =');
    expect(useStorageModelSource).not.toContain('export const getRecordType =');
    expect(useStorageModelSource).not.toContain('export const getRecordContent =');
    expect(useStorageModelSource).not.toContain('export const getRecordStatus =');
    expect(useStorageModelSource).not.toContain('export const getRecordPlatformLabel =');
    expect(useStorageModelSource).not.toContain('export const getRecordHostLabel =');
    expect(useStorageModelSource).not.toContain('export const getRecordTopologyLabel =');
    expect(useStorageModelSource).not.toContain('export const getRecordProtectionLabel =');
    expect(useStorageModelSource).not.toContain('export const getRecordIssueLabel =');
    expect(useStorageModelSource).not.toContain('export const getRecordIssueSummary =');
    expect(useStorageModelSource).not.toContain('export const getRecordImpactSummary =');
    expect(useStorageModelSource).not.toContain('export const getRecordActionSummary =');
    expect(useStorageModelSource).not.toContain('export const getRecordShared =');
    expect(useStorageModelSource).not.toContain('export const getRecordNodeLabel =');
    expect(useStorageModelSource).not.toContain('export const getRecordUsagePercent =');
    expect(useStorageModelSource).not.toContain('export const getRecordZfsPool =');
    expect(useStorageModelSource).not.toContain('const computeGroupStats =');
    expect(useStorageModelSource).not.toContain('const matchesSelectedNode =');
    expect(useStorageModelSource).not.toContain('const sourceOptions = createMemo(() => {');
    expect(storageModelCoreSource).toContain('export const findSelectedStorageNode');
    expect(storageModelCoreSource).toContain('export const matchesStorageRecordNode');
    expect(storageModelCoreSource).toContain('export const buildStorageSourceOptions');
    expect(storageModelCoreSource).toContain('export const matchesStorageRecordSearch');
    expect(storageModelCoreSource).toContain('export const filterStorageRecords');
    expect(storageModelCoreSource).toContain('export const sortStorageRecords');
    expect(storageModelCoreSource).toContain('export const groupStorageRecords');
    expect(storageModelCoreSource).toContain('export const summarizeStorageRecords');
    expect(useStorageAlertStateSource).toContain('@/features/storageBackups/storageAlertState');
    expect(useStorageAlertStateSource).toContain('asStorageAlertRecord');
    expect(useStorageAlertStateSource).toContain('mergeStorageAlertRowState');
    expect(useStorageAlertStateSource).toContain('getStorageRecordAlertResourceIds');
    expect(useStorageAlertStateSource).toContain('EMPTY_STORAGE_ALERT_STATE');
    expect(useStorageAlertStateSource).not.toContain('const EMPTY_ALERT_STATE =');
    expect(useStorageAlertStateSource).not.toContain('const asAlertRecord =');
    expect(useStorageAlertStateSource).not.toContain('const severityWeight =');
    expect(useStorageAlertStateSource).not.toContain('const mergeAlertState =');
    expect(useStorageAlertStateSource).not.toContain('const getRecordAlertResourceIds =');
    expect(diskDetailPresentationSource).toContain(
      'export function getDiskAttributeValueTextClass',
    );
    expect(diskDetailPresentationSource).toContain(
      'export function getLinkedDiskHealthDotClass',
    );
    expect(diskDetailPresentationSource).toContain(
      'export function getLinkedDiskTemperatureTextClass',
    );
    expect(storagePoolDetailPresentationSource).toContain(
      'export function getZfsScanTextClass',
    );
    expect(storagePoolDetailPresentationSource).toContain(
      'export function getZfsErrorTextClass',
    );
    expect(storagePoolDetailPresentationSource).toContain(
      'export function getZfsErrorSummary',
    );
    expect(storagePoolDetailPresentationSource).toContain(
      'export const STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS',
    );
    expect(storagePoolDetailPresentationSource).toContain(
      'export function resolveStoragePoolDetailChartTarget',
    );
    expect(storagePoolDetailPresentationSource).toContain(
      'export function buildStoragePoolDetailConfigRows',
    );
    expect(storagePoolDetailPresentationSource).toContain(
      'export function buildStoragePoolDetailZfsSummary',
    );
    expect(storagePoolDetailPresentationSource).toContain(
      'export function getStoragePoolLinkedDisks',
    );
    expect(storageBarPresentationSource).toContain('export const getStorageBarUsagePercent');
    expect(storageBarPresentationSource).toContain('export const getStorageBarLabel');
    expect(storageBarPresentationSource).toContain('export const getStorageBarTooltipRows');
    expect(storageBarPresentationSource).toContain('export const getStorageBarTooltipRowClass');
    expect(storageBarPresentationSource).toContain('export const getStorageBarZfsHeadingLabel');
    expect(storageBarPresentationSource).toContain('export const getStorageBarZfsSummary');
    expect(storageBarPresentationSource).toContain('export const STORAGE_BAR_ROOT_CLASS');
    expect(useStoragePoolDetailModelSource).toContain('resolveStoragePoolDetailChartTarget');
    expect(useStoragePoolDetailModelSource).toContain('buildStoragePoolDetailConfigRows');
    expect(useStoragePoolDetailModelSource).toContain('buildStoragePoolDetailZfsSummary');
    expect(useStoragePoolDetailModelSource).toContain('getStoragePoolLinkedDisks');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordNodeHints');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordType');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordContent');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordStatus');
    expect(storageRecordPresentationSource).toContain(
      'export const getStorageRecordPlatformLabel',
    );
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordHostLabel');
    expect(storageRecordPresentationSource).toContain(
      'export const getStorageRecordTopologyLabel',
    );
    expect(storageRecordPresentationSource).toContain(
      'export const getStorageRecordProtectionLabel',
    );
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordIssueLabel');
    expect(storageRecordPresentationSource).toContain(
      'export const getStorageRecordIssueSummary',
    );
    expect(storageRecordPresentationSource).toContain(
      'export const getStorageRecordImpactSummary',
    );
    expect(storageRecordPresentationSource).toContain(
      'export const getStorageRecordActionSummary',
    );
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordShared');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordNodeLabel');
    expect(storageRecordPresentationSource).toContain(
      'export const getStorageRecordUsagePercent',
    );
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordZfsPool');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordStats');
    expect(resourceStoragePresentationSource).toContain(
      'export const getCanonicalStoragePlatformKey',
    );
    expect(resourceStoragePresentationSource).toContain(
      'export const getResourceStoragePlatformLabel',
    );
    expect(resourceStoragePresentationSource).toContain(
      'export const getResourceStorageTopologyLabel',
    );
    expect(resourceStoragePresentationSource).toContain(
      'export const getResourceStorageIssueLabel',
    );
    expect(resourceStoragePresentationSource).toContain(
      'export const getResourceStorageIssueSummary',
    );
    expect(resourceStoragePresentationSource).toContain(
      'export const getResourceStorageImpactSummary',
    );
    expect(resourceStoragePresentationSource).toContain(
      'export const getResourceStorageActionSummary',
    );
    expect(resourceStoragePresentationSource).toContain(
      'export const getResourceStorageProtectionLabel',
    );
    expect(resourceStorageMappingSource).toContain('export type ResourceStorageMeta');
    expect(resourceStorageMappingSource).toContain('export const readResourceStorageMeta');
    expect(resourceStorageMappingSource).toContain(
      'export const resolveResourceStorageContent',
    );
    expect(resourceStorageMappingSource).toContain(
      'export const getStorageCapabilitiesForResource',
    );
    expect(resourceStorageMappingSource).toContain(
      'export const getStorageCategoryFromType',
    );
    expect(storageAdapterCoreSource).toContain('export const asNumberOrNull');
    expect(storageAdapterCoreSource).toContain('export const dedupe');
    expect(storageAdapterCoreSource).toContain('export const getStringArray');
    expect(storageAdapterCoreSource).toContain('export const canonicalStorageIdentityKey');
    expect(storageAdapterCoreSource).toContain('export const buildStorageSource');
    expect(storageAdapterCoreSource).toContain('export const buildStorageCapacity');
    expect(storageAdapterCoreSource).toContain(
      'export const metricsTargetForStorageResource',
    );
    expect(storageAdapterCoreSource).toContain(
      'export const normalizeStorageResourceHealth',
    );
    expect(storageAlertStateSource).toContain('export type StorageAlertRowState');
    expect(storageAlertStateSource).toContain('export const EMPTY_STORAGE_ALERT_STATE');
    expect(storageAlertStateSource).toContain('export const asStorageAlertRecord');
    expect(storageAlertStateSource).toContain('export const mergeStorageAlertRowState');
    expect(storageAlertStateSource).toContain('export const getStorageRecordAlertResourceIds');
    expect(storagePageStateSource).toContain('export const normalizeStorageHealthFilter');
    expect(storagePageStateSource).toContain('export const normalizeStorageSortKey');
    expect(storagePageStateSource).toContain('export const normalizeStorageGroupKey');
    expect(storagePageStateSource).toContain('export const normalizeStorageView');
    expect(storagePageStateSource).toContain('export const normalizeStorageSortDirection');
    expect(storagePageStateSource).toContain('export const getStorageFilterGroupBy');
    expect(storagePageStateSource).toContain('export const getStorageStatusFilterValue');
    expect(storagePageStateSource).toContain('export const toStorageHealthFilterValue');
    expect(storagePageStateSource).toContain('export const isStorageRecordCeph');
    expect(storagePageStateSource).toContain('export const buildStorageNodeOptions');
    expect(storagePageStateSource).toContain('export const filterStorageDiskNodeOptions');
    expect(storagePageStateSource).toContain('export const buildStorageNodeOnlineByLabel');
    expect(storagePageStateSource).toContain('export const syncExpandedStorageGroups');
    expect(storagePageStateSource).toContain('export const toggleExpandedStorageGroup');
    expect(storagePageStateSource).toContain('export const countVisiblePhysicalDisksForNode');
    expect(storagePageStateSource).toContain('export const DEFAULT_STORAGE_SORT_OPTIONS');
    expect(storagePageStateSource).toContain('export const STORAGE_STATUS_FILTER_OPTIONS');
    expect(storagePageStateSource).toContain('export const STORAGE_GROUP_BY_OPTIONS');
    expect(storagePageStateSource).toContain('export const countActiveStorageFilters');
    expect(storagePageStateSource).toContain('export const hasActiveStorageFilters');
    expect(storagePageStateSource).toContain('export const getStorageNodeFilterLabel');
    expect(storagePageStateSource).toContain('export const readStorageRouteValue');
    expect(storagePageStateSource).toContain('export const writeStorageRouteValue');
    expect(storagePageStateSource).toContain('export const coerceSelectedStorageNodeId');
    expect(storagePageStateSource).toContain('export const buildStorageNodeFilterOptions');
    expect(storagePageStateSource).toContain('export const DEFAULT_STORAGE_VIEW');
    expect(storagePageStateSource).toContain('export const DEFAULT_STORAGE_SOURCE_FILTER');
    expect(storagePageStateSource).toContain('export const DEFAULT_STORAGE_SORT_KEY');
    expect(storagePageStateSource).toContain('export const DEFAULT_STORAGE_SORT_DIRECTION');
    expect(storagePageStateSource).toContain('export const DEFAULT_STORAGE_GROUP_KEY');
    expect(storagePageStateSource).toContain('export const DEFAULT_STORAGE_STATUS_FILTER');
    expect(cephRecordPresentationSource).toContain('export const isCephStorageRecord');
    expect(cephRecordPresentationSource).toContain(
      'export const getCephClusterKeyFromStorageRecord',
    );
    expect(cephRecordPresentationSource).toContain('export const getCephSummaryText');
    expect(cephRecordPresentationSource).toContain('export const getCephPoolsText');
    expect(cephRecordPresentationSource).toContain('export const collectCephClusterNodes');
    expect(temperatureUtilSource).toContain('export const getTemperatureTextClass');
    expect(pmgInstanceDrawerSource).toContain('getServiceHealthPresentation');
    expect(pmgInstanceDrawerSource).not.toContain('const statusTone =');
    expect(serviceHealthPresentationSource).toContain(
      'export function getServiceHealthPresentation',
    );
    expect(swarmServicesDrawerSource).toContain('getSimpleStatusIndicator');
    expect(swarmServicesDrawerSource).toContain('<StatusDot');
    expect(swarmServicesDrawerSource).toContain('getSwarmDrawerPresentation');
    expect(swarmServicesDrawerSource).toContain('getSwarmServicesEmptyState');
    expect(swarmServicesDrawerSource).toContain('getSwarmServicesLoadingState');
    expect(swarmServicesDrawerSource).not.toContain('const statusTone =');
    expect(swarmServicesDrawerSource).not.toContain('No Swarm cluster detected');
    expect(swarmServicesDrawerSource).not.toContain('Search services...');
    expect(swarmServicesDrawerSource).not.toContain('Loading Swarm services...');
    expect(swarmServicesDrawerSource).not.toContain('No Swarm services found');
    expect(swarmServicesDrawerSource).not.toContain('No services match your filters');
    expect(swarmPresentationSource).toContain('export function getSwarmDrawerPresentation');
    expect(swarmPresentationSource).toContain('export function getSwarmServicesEmptyState');
    expect(swarmPresentationSource).toContain('export function getSwarmServicesLoadingState');
    expect(k8sDeploymentsDrawerSource).toContain('getSimpleStatusIndicator');
    expect(k8sDeploymentsDrawerSource).toContain('<StatusDot');
    expect(k8sDeploymentsDrawerSource).toContain('getK8sDeploymentsDrawerPresentation');
    expect(k8sDeploymentsDrawerSource).toContain('getK8sDeploymentsEmptyState');
    expect(k8sDeploymentsDrawerSource).toContain('getK8sDeploymentsLoadingState');
    expect(k8sDeploymentsDrawerSource).not.toContain('const statusTone =');
    expect(k8sDeploymentsDrawerSource).not.toContain('Desired state controllers (not Pods)');
    expect(k8sDeploymentsDrawerSource).not.toContain('Search deployments...');
    expect(k8sDeploymentsDrawerSource).not.toContain("label=\"Namespace\"");
    expect(k8sDeploymentsDrawerSource).not.toContain('All namespaces');
    expect(k8sDeploymentsDrawerSource).not.toContain('Open Pods');
    expect(k8sDeploymentsDrawerSource).not.toContain('Loading deployments...');
    expect(k8sDeploymentsDrawerSource).not.toContain('No deployments match your filters');
    expect(k8sDeploymentsDrawerSource).not.toContain(
      'Try clearing the search or namespace filter.',
    );
    expect(k8sDeploymentPresentationSource).toContain(
      'export function getK8sDeploymentsDrawerPresentation',
    );
    expect(k8sDeploymentPresentationSource).toContain(
      'export function getK8sDeploymentsEmptyState',
    );
    expect(k8sDeploymentPresentationSource).toContain(
      'export function getK8sDeploymentsLoadingState',
    );
    expect(k8sNamespacesDrawerSource).toContain('getNamespaceCountsIndicator');
    expect(k8sNamespacesDrawerSource).toContain('getK8sNamespacesDrawerPresentation');
    expect(k8sNamespacesDrawerSource).toContain('getK8sNamespacesEmptyState');
    expect(k8sNamespacesDrawerSource).toContain('getK8sNamespacesLoadingState');
    expect(k8sNamespacesDrawerSource).toContain('getK8sNamespacesFailureState');
    expect(k8sNamespacesDrawerSource).toContain('<StatusDot');
    expect(k8sNamespacesDrawerSource).not.toContain('const statusTone =');
    expect(k8sNamespacesDrawerSource).not.toContain('Scope Pods and Deployments by namespace');
    expect(k8sNamespacesDrawerSource).not.toContain('Search namespaces...');
    expect(k8sNamespacesDrawerSource).not.toContain('Open All Pods');
    expect(k8sNamespacesDrawerSource).not.toContain('View Deployments');
    expect(k8sNamespacesDrawerSource).not.toContain('Loading namespaces...');
    expect(k8sNamespacesDrawerSource).not.toContain('Failed to load namespaces');
    expect(k8sNamespacesDrawerSource).not.toContain('Aggregating Kubernetes namespaces.');
    expect(k8sNamespacesDrawerSource).not.toContain('No namespaces match your filters');
    expect(k8sNamespacesDrawerSource).not.toContain('Try clearing your search.');
    expect(k8sNamespacePresentationSource).toContain(
      'export function getK8sNamespacesDrawerPresentation',
    );
    expect(k8sNamespacePresentationSource).toContain(
      'export function getK8sNamespacesEmptyState',
    );
    expect(k8sNamespacePresentationSource).toContain(
      'export function getK8sNamespacesLoadingState',
    );
    expect(k8sNamespacePresentationSource).toContain(
      'export function getK8sNamespacesFailureState',
    );
    expect(k8sStatusPresentationSource).toContain('export function getNamespaceCountsIndicator');
    expect(raidCardSource).toContain('getRaidStateVariant');
    expect(raidCardSource).toContain('getRaidStateTextClass');
    expect(raidCardSource).toContain('getRaidDeviceBadgeClass');
    expect(raidCardSource).not.toContain('const raidStateVariant =');
    expect(raidCardSource).not.toContain('const deviceToneClass =');
    expect(raidPresentationSource).toContain('export function getRaidStateVariant');
    expect(raidPresentationSource).toContain('export function getRaidDeviceBadgeClass');
    expect(proLicensePanelSource).toContain('getLicenseSubscriptionStatusPresentation');
    expect(proLicensePanelSource).toContain('getLicenseStatusLoadingState');
    expect(proLicensePanelSource).toContain('getNoActiveProLicenseState');
    expect(proLicensePanelSource).toContain('formatLicensePlanVersion');
    expect(proLicensePanelSource).toContain('getTrialActivationNotice');
    expect(proLicensePanelSource).toContain('getCommercialMigrationNotice');
    expect(proLicensePanelSource).not.toContain('Loading license status...');
    expect(proLicensePanelSource).not.toContain('No Pro license is active.');
    expect(proLicensePanelSource).not.toContain('const statusLabel =');
    expect(proLicensePanelSource).not.toContain('const statusTone =');
    expect(proLicensePanelSource).not.toContain('const formatTitleCase =');
    expect(proLicensePanelSource).not.toContain('const commercialMigrationActionText =');
    expect(proLicensePanelSource).not.toContain('const commercialMigrationNoticeFor =');
    expect(licensePresentationSource).toContain(
      'export const getLicenseSubscriptionStatusPresentation',
    );
    expect(licensePresentationSource).toContain('export const getLicenseStatusLoadingState');
    expect(licensePresentationSource).toContain('export const getNoActiveProLicenseState');
    expect(licensePresentationSource).toContain(
      'export const getOrganizationBillingLicenseStatusLabel',
    );
    expect(licensePresentationSource).toContain('export const getBillingAdminTrialStatus');
    expect(licensePresentationSource).toContain(
      'export const getBillingAdminOrganizationBadges',
    );
    expect(licensePresentationSource).toContain(
      'export const getBillingAdminStateUpdateSuccessMessage',
    );
    expect(licensePresentationSource).toContain('export const BILLING_ADMIN_EMPTY_STATE');
    expect(licensePresentationSource).toContain('export const formatLicensePlanVersion');
    expect(licensePresentationSource).toContain('export const getCommercialMigrationNotice');
    expect(licensePresentationSource).toContain('export const getTrialActivationNotice');
    expect(securityPostureSummarySource).toContain('getSecurityScorePresentation');
    expect(securityPostureSummarySource).toContain('getSecurityScoreIconComponent');
    expect(securityPostureSummarySource).toContain('getSecurityFeatureCardPresentation');
    expect(securityPostureSummarySource).not.toContain('const scoreTone =');
    expect(securityPostureSummarySource).not.toContain('const ScoreIcon =');
    expect(securityPostureSummarySource).not.toContain(
      "item.enabled\n                    ? 'border-emerald-200 dark:border-emerald-800 bg-emerald-50 dark:bg-emerald-950'",
    );
    expect(securityWarningSource).toContain('getSecurityWarningPresentation');
    expect(securityWarningSource).toContain('getSecurityFeatureStatePresentation');
    expect(securityWarningSource).toContain('getSecurityScorePresentation');
    expect(securityWarningSource).toContain('warningPresentation().background');
    expect(securityWarningSource).toContain('warningPresentation().border');
    expect(securityWarningSource).not.toContain('bg-yellow-50 dark:bg-yellow-900');
    expect(securityWarningSource).not.toContain("status()!.credentialsEncrypted ? 'text-green-600' : 'text-red-600'");
    expect(findingsPanelSource).toContain('getFindingStatusBadgeClasses');
    expect(findingsPanelSource).toContain('getFindingStatusLabel');
    expect(findingsPanelSource).toContain('getFindingSeveritySortOrder');
    expect(findingsPanelSource).toContain('getInvestigationOutcomeSortOrder');
    expect(findingsPanelSource).toContain('getInvestigationOutcomeBadgeClasses');
    expect(findingsPanelSource).toContain('getInvestigationOutcomeLabel');
    expect(findingsPanelSource).toContain('getInvestigationStatusLabel');
    expect(findingsPanelSource).toContain('hasFindingInvestigationDetails');
    expect(findingsPanelSource).toContain('getFindingResolutionReason');
    expect(findingsPanelSource).toContain('buildFindingFilterOptions');
    expect(findingsPanelSource).toContain('getFindingEmptyStateCopy');
    expect(findingsPanelSource).not.toContain('finding.investigationSessionId');
    expect(findingsPanelSource).not.toContain("finding.status === 'resolved' ? 'Resolved'");
    expect(findingsPanelSource).not.toContain('const severityOrder');
    expect(findingsPanelSource).not.toContain('const outcomeOrder');
    expect(findingsPanelSource).not.toContain('switch (alertType) {');
    expect(findingsPanelSource).not.toContain('switch (finding.investigationOutcome) {');
    expect(findingsPanelSource).not.toContain(
      "filter() === 'attention'\n                    ? 'bg-amber-50 dark:bg-amber-900 text-amber-700 dark:text-amber-300 border-amber-300 dark:border-amber-700 shadow-sm'",
    );
    expect(findingsPanelSource).not.toContain('No active findings');
    expect(findingsPanelSource).not.toContain('No pending approvals.');
    expect(aiFindingPresentationSource).toContain(
      'export const getFindingStatusBadgeClasses',
    );
    expect(aiFindingPresentationSource).toContain('export const getFindingStatusLabel');
    expect(aiFindingPresentationSource).toContain(
      'export const getFindingSeveritySortOrder',
    );
    expect(aiFindingPresentationSource).toContain(
      'export const getFindingSeverityCompactLabel',
    );
    expect(aiFindingPresentationSource).toContain(
      'export const getInvestigationOutcomeBadgeClasses',
    );
    expect(aiFindingPresentationSource).toContain(
      'export const getInvestigationOutcomeLabel',
    );
    expect(aiFindingPresentationSource).toContain(
      'export const getInvestigationStatusLabel',
    );
    expect(aiFindingPresentationSource).toContain(
      'export const getInvestigationOutcomeSortOrder',
    );
    expect(aiFindingPresentationSource).toContain(
      'export const hasFindingInvestigationDetails',
    );
    expect(aiFindingPresentationSource).toContain('export const getFindingResolutionReason');
    expect(aiFindingPresentationSource).toContain('export const buildFindingFilterOptions');
    expect(aiFindingPresentationSource).toContain('export const getFindingEmptyStateCopy');
    expect(investigationSectionSource).toContain('getInvestigationOutcomeBadgeClasses');
    expect(investigationSectionSource).toContain('getInvestigationOutcomeLabel');
    expect(investigationSectionSource).toContain('getInvestigationStatusLabel');
    expect(investigationSectionSource).not.toContain('investigationStatusLabels');
    expect(investigationSectionSource).not.toContain('investigationOutcomeLabels');
    expect(investigationSectionSource).not.toContain('investigationOutcomeColors');
    expect(securityWarningSource).not.toContain('bg-red-50 dark:bg-red-900');
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityScorePresentation',
    );
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityWarningPresentation',
    );
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityFeatureStatePresentation',
    );
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityScoreIconComponent',
    );
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityFeatureCardPresentation',
    );
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityPostureItems',
    );
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityNetworkAccessSubtitle',
    );
    expect(securityPostureSummarySource).toContain('getSecurityPostureItems');
    expect(securityPostureSummarySource).toContain('getSecurityNetworkAccessSubtitle');
    expect(securityPostureSummarySource).not.toContain("label: 'Password login'");
    expect(securityPostureSummarySource).not.toContain('Public network access detected');
    expect(securityAuthPanelSource).toContain('@/utils/securityAuthPresentation');
    expect(securityAuthPanelSource).not.toContain('Authentication disabled');
    expect(securityAuthPanelSource).not.toContain(
      'Authentication is currently disabled. Set up password authentication to protect your Pulse instance.',
    );
    expect(securityAuthPanelSource).not.toContain(
      'Security Configured - Restart Required',
    );
    expect(securityAuthPresentationSource).toContain(
      'export const SECURITY_AUTH_DISABLED_PANEL_TITLE',
    );
    expect(securityAuthPresentationSource).toContain(
      'export function getSecurityAuthRestartInstruction',
    );
    expect(generalSettingsPanelSource).toContain('EnvironmentLockBadge');
    expect(generalSettingsPanelSource).not.toContain(
      'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    );
    expect(dockerRuntimeSettingsCardSource).toContain('EnvironmentLockBadge');
    expect(dockerRuntimeSettingsCardSource).toContain('ENVIRONMENT_LOCK_BUTTON_TITLE');
    expect(dockerRuntimeSettingsCardSource).not.toContain('>ENV<');
    expect(dockerRuntimeSettingsCardSource).not.toContain(
      'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    );
    expect(environmentLockBadgeSource).toContain('getEnvironmentLockTitle');
    expect(environmentLockPresentationSource).toContain(
      'export const ENVIRONMENT_LOCK_BADGE_CLASS',
    );
    expect(environmentLockPresentationSource).toContain(
      'export function getEnvironmentLockTitle',
    );
    expect(environmentLockPresentationSource).toContain(
      'export const ENVIRONMENT_LOCK_BUTTON_TITLE',
    );
    expect(resourceDetailDrawerSource).toContain('getServiceHealthPresentation');
    expect(resourceDetailDrawerSource).not.toContain('healthToneClass(');
    expect(resourceDetailDrawerSource).not.toContain('normalizeHealthLabel(');
    expect(resourceDetailMappersSource).not.toContain('export const normalizeHealthLabel');
    expect(resourceDetailMappersSource).not.toContain('export const healthToneClass');
    expect(unifiedResourceTableSource).toContain('getServiceHealthSummaryPresentation');
    expect(unifiedResourceTableSource).not.toContain('const summarizeServiceHealthTone =');
    expect(exploreStatusBlockSource).toContain('getAIExploreStatusPresentation');
    expect(exploreStatusBlockSource).not.toContain('const phaseLabel =');
    expect(exploreStatusBlockSource).not.toContain('const phaseClasses =');
    expect(aiExplorePresentationSource).toContain('export function getAIExploreStatusPresentation');
    expect(discoveryTabSource).toContain('getDiscoveryURLSuggestionSourceLabel');
    expect(discoveryTabSource).toContain('getDiscoveryAnalysisProviderBadgeClass');
    expect(discoveryTabSource).toContain('getDiscoveryCategoryBadgeClass');
    expect(discoveryTabSource).toContain('getDiscoveryInitialEmptyState');
    expect(discoveryTabSource).toContain('getDiscoveryLoadingState');
    expect(discoveryTabSource).toContain('getDiscoverySuggestedURLFallback');
    expect(discoveryTabSource).toContain('getDiscoveryNotesEmptyState');
    expect(guestDrawerSource).toContain('getDiscoveryLoadingState');
    expect(infrastructureDetailsDrawerSource).toContain('getDiscoveryLoadingState');
    expect(resourceDetailDrawerSource).toContain('getDiscoveryLoadingState');
    expect(discoveryTabSource).not.toContain('const getURLSuggestionSourceLabel =');
    expect(discoveryPresentationSource).toContain(
      'export function getDiscoveryURLSuggestionSourceLabel',
    );
    expect(discoveryTabSource).not.toContain('bg-blue-100 text-blue-700');
    expect(discoveryTabSource).not.toContain('bg-green-100 text-green-700');
    expect(discoveryTabSource).not.toContain('No discovery data yet');
    expect(discoveryTabSource).not.toContain('Loading discovery...');
    expect(guestDrawerSource).not.toContain('Loading discovery...');
    expect(infrastructureDetailsDrawerSource).not.toContain('Loading discovery...');
    expect(resourceDetailDrawerSource).not.toContain('Loading discovery...');
    expect(discoveryTabSource).not.toContain('No suggested URL found');
    expect(discoveryTabSource).not.toContain(
      'No notes yet. Add notes to document important information.',
    );
    expect(webInterfaceUrlFieldSource).toContain('getDiscoverySuggestedURLFallback');
    expect(webInterfaceUrlFieldSource).not.toContain('No suggested URL found');
    expect(discoveryPresentationSource).toContain(
      'export function getDiscoveryAnalysisProviderBadgeClass',
    );
    expect(discoveryPresentationSource).toContain('export function getDiscoveryCategoryBadgeClass');
    expect(discoveryPresentationSource).toContain('export function getDiscoveryInitialEmptyState');
    expect(discoveryPresentationSource).toContain('export function getDiscoveryLoadingState');
    expect(discoveryPresentationSource).toContain(
      'export function getDiscoverySuggestedURLFallback',
    );
    expect(discoveryPresentationSource).toContain('export function getDiscoveryNotesEmptyState');
    expect(mailGatewaySource).toContain('getPMGThreatPresentation');
    expect(mailGatewaySource).not.toContain('const barColor =');
    expect(mailGatewaySource).not.toContain('const textColor =');
    expect(mailGatewaySource).toContain('getPMGQueueTextClass');
    expect(mailGatewaySource).toContain('getPMGOldestAgeTextClass');
    expect(mailGatewaySource).not.toContain('const queueSeverity =');
    expect(mailGatewaySource).toContain('ServiceHealthBadge');
    expect(mailGatewaySource).toContain('PMG_EMPTY_STATE_TITLE');
    expect(mailGatewaySource).toContain('PMG_EMPTY_STATE_DESCRIPTION');
    expect(mailGatewaySource).toContain('PMG_LOADING_STATE_TITLE');
    expect(mailGatewaySource).toContain('PMG_LOADING_STATE_DESCRIPTION');
    expect(mailGatewaySource).toContain('getPMGDisconnectedState');
    expect(mailGatewaySource).toContain('getPMGSearchEmptyState');
    expect(mailGatewaySource).toContain('PMG_SEARCH_PLACEHOLDER');
    expect(mailGatewaySource).not.toContain('No Mail Gateways configured');
    expect(mailGatewaySource).not.toContain('Loading mail gateway data...');
    expect(mailGatewaySource).not.toContain('Connection lost');
    expect(mailGatewaySource).not.toContain('Attempting to reconnect…');
    expect(mailGatewaySource).not.toContain('Unable to connect to the backend server');
    expect(mailGatewaySource).not.toContain('Search gateways...');
    expect(mailGatewaySource).not.toContain('No gateways match "');
    expect(mailGatewaySource).not.toContain('Reconnect now');
    expect(mailGatewaySource).not.toContain('Clear search');
    expect(mailGatewaySource).not.toContain('const StatusBadge: Component');
    expect(pmgInstanceDrawerSource).toContain('getPMGDetailsDrawerPresentation');
    expect(pmgInstanceDrawerSource).toContain('PMG_DETAILS_EMPTY_STATE_TITLE');
    expect(pmgInstanceDrawerSource).toContain('PMG_DETAILS_EMPTY_STATE_DESCRIPTION');
    expect(pmgInstanceDrawerSource).toContain('PMG_DETAILS_LOADING_STATE_TITLE');
    expect(pmgInstanceDrawerSource).toContain('PMG_DETAILS_LOADING_STATE_DESCRIPTION');
    expect(pmgInstanceDrawerSource).toContain('PMG_DETAILS_FAILURE_STATE_TITLE');
    expect(pmgInstanceDrawerSource).not.toContain('Search domains...');
    expect(pmgInstanceDrawerSource).not.toContain('Unknown host');
    expect(pmgInstanceDrawerSource).not.toContain('Spam Distribution');
    expect(pmgInstanceDrawerSource).not.toContain('No PMG details for this resource yet');
    expect(pmgInstanceDrawerSource).not.toContain('Loading mail gateway details...');
    expect(pmgInstanceDrawerSource).not.toContain('Failed to load PMG details');
    expect(pmgInstancePanelSource).toContain('getPMGThreatPresentation');
    expect(pmgInstancePanelSource).not.toContain('const barColor =');
    expect(pmgInstancePanelSource).not.toContain('const textColor =');
    expect(pmgInstancePanelSource).toContain('getPMGQueueTextClass');
    expect(pmgInstancePanelSource).toContain('getPMGOldestAgeTextClass');
    expect(pmgInstancePanelSource).not.toContain('const queueSeverity =');
    expect(pmgInstancePanelSource).toContain('ServiceHealthBadge');
    expect(pmgInstancePanelSource).not.toContain('const StatusBadge: Component');
    expect(pmgPresentationSource).toContain('export const PMG_EMPTY_STATE_TITLE');
    expect(pmgPresentationSource).toContain('export const PMG_EMPTY_STATE_DESCRIPTION');
    expect(pmgPresentationSource).toContain('export const PMG_LOADING_STATE_TITLE');
    expect(pmgPresentationSource).toContain('export const PMG_LOADING_STATE_DESCRIPTION');
    expect(pmgPresentationSource).toContain('export const PMG_DISCONNECTED_STATE_TITLE');
    expect(pmgPresentationSource).toContain('export const PMG_SEARCH_PLACEHOLDER');
    expect(pmgPresentationSource).toContain('export const PMG_DETAILS_EMPTY_STATE_TITLE');
    expect(pmgPresentationSource).toContain('export const PMG_DETAILS_EMPTY_STATE_DESCRIPTION');
    expect(pmgPresentationSource).toContain('export const PMG_DETAILS_LOADING_STATE_TITLE');
    expect(pmgPresentationSource).toContain('export const PMG_DETAILS_LOADING_STATE_DESCRIPTION');
    expect(pmgPresentationSource).toContain('export const PMG_DETAILS_FAILURE_STATE_TITLE');
    expect(pmgPresentationSource).toContain('export function getPMGDetailsDrawerPresentation');
    expect(pmgPresentationSource).toContain('export function getPMGDisconnectedState');
    expect(pmgPresentationSource).toContain('export function getPMGSearchEmptyState');
    expect(pmgThreatPresentationSource).toContain('export function getPMGThreatPresentation');
    expect(pmgQueuePresentationSource).toContain('export function getPMGQueueTextClass');
    expect(pmgServiceHealthBadgeSource).toContain('getServiceHealthPresentation');
    expect(proxmoxSettingsPanelSource).toContain('getProxmoxVariantPresentation');
    expect(proxmoxSettingsPanelSource).toContain('buildProxmoxDiscoveryPrefillNode');
    expect(proxmoxSettingsPanelSource).not.toContain('const VARIANT_CONFIG: Record<NodeType');
    expect(proxmoxSettingsPanelSource).not.toContain('const buildDiscoveryPrefillNode =');
    expect(proxmoxSettingsPresentationSource).toContain(
      'export const PROXMOX_VARIANT_PRESENTATION',
    );
    expect(proxmoxSettingsPresentationSource).toContain(
      'export function getProxmoxVariantPresentation',
    );
    expect(proxmoxSettingsPresentationSource).toContain(
      'export function buildProxmoxDiscoveryPrefillNode',
    );
    expect(nodeModalSource).toContain('getNodeModalDefaultFormData');
    expect(nodeModalSource).toContain('getNodeProductName');
    expect(nodeModalSource).toContain('getNodeEndpointPlaceholder');
    expect(nodeModalSource).toContain('getNodeGuestUrlPlaceholder');
    expect(nodeModalSource).toContain('getNodeUsernamePlaceholder');
    expect(nodeModalSource).toContain('getNodeUsernameHelp');
    expect(nodeModalSource).toContain('getNodeTokenIdPlaceholder');
    expect(nodeModalSource).toContain('getNodeMonitoringCoverageCopy');
    expect(nodeModalSource).toContain('getTemperatureMonitoringLockedCopy');
    expect(nodeModalSource).toContain('getNodeModalTestResultPresentation');
    expect(nodeModalSource).toContain('buildNodeModalMonitoringPayload');
    expect(nodeModalSource).not.toContain('const getCleanFormData =');
    expect(nodeModalSource).not.toContain('const nodeProductName =');
    expect(nodeModalSource).not.toContain("testResult()?.status === 'success'");
    expect(nodeModalSource).not.toContain("testResult()?.status === 'warning'");
    expect(nodeModalPresentationSource).toContain('export function getNodeModalDefaultFormData');
    expect(nodeModalPresentationSource).toContain('export function getNodeProductName');
    expect(nodeModalPresentationSource).toContain('export function getNodeEndpointPlaceholder');
    expect(nodeModalPresentationSource).toContain('export function getNodeGuestUrlPlaceholder');
    expect(nodeModalPresentationSource).toContain('export function getNodeUsernamePlaceholder');
    expect(nodeModalPresentationSource).toContain('export function getNodeTokenIdPlaceholder');
    expect(nodeModalPresentationSource).toContain(
      'export function getNodeMonitoringCoverageCopy',
    );
    expect(nodeModalPresentationSource).toContain(
      'export function getTemperatureMonitoringLockedCopy',
    );
    expect(nodeModalPresentationSource).toContain(
      'export function getNodeModalTestResultPresentation',
    );
    expect(nodeModalPresentationSource).toContain(
      'export function buildNodeModalMonitoringPayload',
    );
    expect(cephPageSource).toContain('getCephServiceStatusPresentation');
    expect(cephPageSource).toContain('getCephLoadingStatePresentation');
    expect(cephPageSource).toContain('getCephDisconnectedStatePresentation');
    expect(cephPageSource).toContain('getCephNoClustersStatePresentation');
    expect(cephPageSource).toContain('getCephPoolsSearchEmptyStatePresentation');
    expect(cephPageSource).toContain('CephServiceIcon');
    expect(cephPageSource).not.toContain('const getServiceStatus =');
    expect(cephPageSource).not.toContain('const ServiceIcon: Component');
    expect(cephPageSource).not.toContain('Loading Ceph data...');
    expect(cephPageSource).not.toContain('No Ceph Clusters Detected');
    expect(cephPageSource).not.toContain('No pools match "');
    expect(storageDomainSource).toContain('export const getCephServiceStatusPresentation');
    expect(storageDomainSource).toContain('export const getCephLoadingStatePresentation');
    expect(storageDomainSource).toContain(
      'export const getCephDisconnectedStatePresentation',
    );
    expect(storageDomainSource).toContain('export const getCephNoClustersStatePresentation');
    expect(storageDomainSource).toContain(
      'export const getCephPoolsSearchEmptyStatePresentation',
    );
    expect(cephServiceIconSource).toContain('export const CephServiceIcon');
    expect(deployStatusBadgeSource).toContain('getDeployStatusPresentation');
    expect(deployStatusBadgeSource).not.toContain('const statusConfig: Record<DeployTargetStatus');
    expect(deployCandidatesStepSource).toContain('getDeployCandidatesLoadingState');
    expect(deployCandidatesStepSource).toContain('getDeployNoSourceAgentsState');
    expect(deployCandidatesStepSource).toContain('getDeployNoCandidatesState');
    expect(deployCandidatesStepSource).not.toContain('Loading cluster nodes...');
    expect(deployCandidatesStepSource).not.toContain('No nodes found in this cluster.');
    expect(deployCandidatesStepSource).not.toContain('No online source agents found.');
    expect(deployResultsStepSource).toContain('getDeployInstallCommandLoadingState');
    expect(deployResultsStepSource).not.toContain('Loading install command...');
    expect(deployFlowPresentationSource).toContain(
      'export function getDeployCandidatesLoadingState',
    );
    expect(deployFlowPresentationSource).toContain(
      'export function getDeployNoSourceAgentsState',
    );
    expect(deployFlowPresentationSource).toContain(
      'export function getDeployNoCandidatesState',
    );
    expect(deployFlowPresentationSource).toContain(
      'export function getDeployInstallCommandLoadingState',
    );
    expect(deployStatusPresentationSource).toContain('export const getDeployStatusPresentation');
    expect(alertsPageSource).toContain('getAlertIncidentStatusPresentation');
    expect(alertsPageSource).toContain('getAlertIncidentLevelBadgeClass');
    expect(alertsPageSource).toContain('getAlertDestinationsConfigLoadError');
    expect(alertsPageSource).toContain('getAlertDestinationsWebhookLoadError');
    expect(alertsPageSource).toContain('getAlertDestinationsLoadErrorBanner');
    expect(alertsPageSource).toContain('getAlertDestinationsAppriseTargetsHelp');
    expect(alertsPageSource).toContain('getAlertDestinationsAppriseTestLabel');
    expect(alertsPageSource).toContain('getAlertDestinationsAppriseTestError');
    expect(alertsPageSource).toContain('getAlertDestinationsAppriseValidationError');
    expect(alertsPageSource).toContain('getAlertDestinationsEmailTestSuccess');
    expect(alertsPageSource).toContain('getAlertDestinationsEmailTestFailure');
    expect(alertsPageSource).toContain('getAlertDestinationsAppriseTestSuccess');
    expect(alertsPageSource).toContain('getAlertDestinationsAppriseTestFailure');
    expect(alertsPageSource).toContain('getAlertDestinationsRetryLabel');
    expect(alertsPageSource).toContain('getAlertDestinationsStatusLabel');
    expect(alertsPageSource).toContain('getAlertWebhookTestSuccess');
    expect(alertsPageSource).toContain('getAlertWebhookTestFailure');
    expect(alertsPageSource).toContain('getAlertHistoryStatusPresentation');
    expect(alertsPageSource).toContain('getAlertHistorySourcePresentation');
    expect(alertsPageSource).toContain('getAlertHistoryResourceTypeBadgeClass');
    expect(alertsPageSource).toContain('getAlertResourceIncidentPanelTitle');
    expect(alertsPageSource).toContain('getAlertResourceIncidentCountLabel');
    expect(alertsPageSource).toContain('getAlertResourceIncidentLoadingState');
    expect(alertsPageSource).toContain('getAlertResourceIncidentEmptyState');
    expect(alertsPageSource).toContain('getAlertResourceIncidentRefreshLabel');
    expect(alertsPageSource).toContain('getAlertResourceIncidentAcknowledgedByLabel');
    expect(alertsPageSource).toContain('getAlertResourceIncidentToggleLabel');
    expect(alertsPageSource).toContain('getAlertResourceIncidentFilteredEventsEmptyState');
    expect(alertsPageSource).toContain('getAlertResourceIncidentRecentEventsSummary');
    expect(alertsPageSource).toContain('getAlertResourceIncidentNotePlaceholder');
    expect(alertsPageSource).toContain('getAlertResourceIncidentSaveNoteLabel');
    expect(alertsPageSource).toContain('getAlertIncidentEventFilterContainerClass');
    expect(alertsPageSource).toContain('getAlertIncidentEventFilterActionButtonClass');
    expect(alertsPageSource).toContain('getAlertIncidentEventFilterChipClass');
    expect(alertsPageSource).toContain('getAlertIncidentAcknowledgedBadgeClass');
    expect(alertsPageSource).toContain('getAlertIncidentTimelineEventCardClass');
    expect(alertsPageSource).toContain('getAlertIncidentNoteTextareaClass');
    expect(alertsPageSource).toContain('getAlertIncidentNoteSaveButtonClass');
    expect(alertsPageSource).toContain('getAlertIncidentTimelineMetaRowClass');
    expect(alertsPageSource).toContain('getAlertIncidentTimelineHeadingClass');
    expect(alertsPageSource).toContain('getAlertIncidentTimelineDetailClass');
    expect(alertsPageSource).toContain('getAlertIncidentTimelineCommandClass');
    expect(alertsPageSource).toContain('getAlertIncidentTimelineOutputClass');
    expect(alertsPageSource).toContain('getAlertResourceIncidentCardClass');
    expect(alertsPageSource).toContain('getAlertResourceIncidentSummaryRowClass');
    expect(alertsPageSource).toContain('getAlertResourceIncidentToggleButtonClass');
    expect(alertsPageSource).toContain('getAlertResourceIncidentTruncatedEventsLabel');
    expect(alertsPageSource).toContain('getAlertBucketCountLabel');
    expect(alertsPageSource).toContain('getAlertHistorySearchPlaceholder');
    expect(alertsPageSource).toContain('getAlertsPageHeaderMeta');
    expect(alertsPageSource).toContain('getAlertHistoryEmptyState');
    expect(alertsPageSource).toContain('getAlertHistoryLoadingState');
    expect(alertsPageSource).toContain('getAlertAdministrationSectionTitle');
    expect(alertsPageSource).toContain('getAlertAdministrationSectionDescription');
    expect(alertsPageSource).toContain('getAlertAdministrationClearHistoryLabel');
    expect(alertsPageSource).toContain('getAlertAdministrationClearHistoryError');
    expect(alertsPageSource).toContain('getAlertAdministrationClearHistoryConfirmation');
    expect(alertsPageSource).toContain('getAlertActivationPresentation');
    expect(alertsPageSource).toContain('getAlertActivationSuccess');
    expect(alertsPageSource).toContain('getAlertActivationFailure');
    expect(alertsPageSource).toContain('getAlertDeactivationSuccess');
    expect(alertsPageSource).toContain('getAlertDeactivationFailure');
    expect(alertsPageSource).toContain('getAlertFrequencySelectionPresentation');
    expect(alertsPageSource).toContain('getAlertFrequencyClearFilterButtonClass');
    expect(alertsPageSource).toContain('getAlertSeverityDotClass');
    expect(alertsPageSource).toContain('getAlertsSidebarTabClass');
    expect(alertsPageSource).toContain('getAlertsMobileTabClass');
    expect(alertsPageSource).toContain('getAlertsTabTitle');
    expect(alertsPageSource).toContain('getAlertsTabGroups');
    expect(alertsPageSource).toContain('getAlertGroupingCardClass');
    expect(alertsPageSource).toContain('getAlertGroupingCheckboxClass');
    expect(alertsPageSource).toContain('getAlertQuietDayButtonClass');
    expect(alertsPageSource).toContain('getAlertQuietSuppressCardClass');
    expect(alertsPageSource).toContain('getAlertQuietSuppressCheckboxClass');
    expect(alertsPageSource).toContain('getAlertConfigUnsavedChangesLabel');
    expect(alertsPageSource).toContain('getAlertConfigSaveChangesLabel');
    expect(alertsPageSource).toContain('getAlertConfigResetDefaultsLabel');
    expect(alertsPageSource).toContain('getAlertConfigResetDefaultsTitle');
    expect(alertsPageSource).toContain('getAlertConfigDiscardedSuccess');
    expect(alertsPageSource).toContain('getAlertConfigReloadFailure');
    expect(alertsPageSource).toContain('getAlertConfigDiscardLabel');
    expect(alertsPageSource).toContain('getAlertConfigToggleStatusLabel');
    expect(alertsPageSource).toContain('getAlertConfigLeaveConfirmation');
    expect(alertsPageSource).toContain('getAlertConfigSummaryQuietHours');
    expect(alertsPageSource).toContain('getAlertConfigSummarySuppressing');
    expect(alertsPageSource).toContain('getAlertConfigSummaryCooldown');
    expect(alertsPageSource).toContain('getAlertConfigSummaryGrouping');
    expect(alertsPageSource).toContain('getAlertConfigSummaryRecoveryEnabled');
    expect(alertsPageSource).toContain('getAlertConfigSummaryEscalation');
    expect(alertsPageSource).toContain('getAlertConfigQuietHourSuppressOptions');
    expect(alertsPageSource).toContain('ALERT_CONFIG_COOLDOWN_PERIOD_LABEL');
    expect(alertsPageSource).toContain('ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL');
    expect(alertsPageSource).toContain('ALERT_CONFIG_GROUPING_WINDOW_LABEL');
    expect(alertsPageSource).toContain('ALERT_CONFIG_GROUPING_STRATEGY_LABEL');
    expect(alertsPageSource).not.toContain('const statusClasses =');
    expect(alertsPageSource).not.toContain('const levelClasses =');
    expect(alertsPageSource).not.toContain("alert.source === 'ai' ? 'Patrol' : 'Alert'");
    expect(alertsPageSource).not.toContain("alert.resourceType === 'VM'");
    expect(alertsPageSource).not.toContain(
      "isAlertsActive() ? 'text-green-600 dark:text-green-400' : 'text-muted'",
    );
    expect(alertsPageSource).not.toContain(
      "isAlertsActive() ? 'bg-blue-600' : 'bg-surface-hover'",
    );
    expect(alertsPageSource).not.toContain(
      'Failed to load notification configuration. Your existing settings could not be retrieved.',
    );
    expect(alertsPageSource).not.toContain('Failed to load webhook configuration.');
    expect(alertsPageSource).not.toContain(
      'Saving now may overwrite your existing settings with defaults.',
    );
    expect(alertsPageSource).not.toContain('Configure SMTP delivery for alert emails.');
    expect(alertsPageSource).not.toContain('Relay grouped alerts through Apprise via CLI or remote API.');
    expect(alertsPageSource).not.toContain('Choose how Pulse should execute Apprise notifications.');
    expect(alertsPageSource).not.toContain('Enter one Apprise URL per line. Commas are also supported.');
    expect(alertsPageSource).not.toContain('Optional: override the URLs defined on your Apprise API instance. Leave blank to use the server defaults.');
    expect(alertsPageSource).not.toContain('Enable Apprise notifications before sending a test.');
    expect(alertsPageSource).not.toContain('Add at least one Apprise target to test CLI delivery.');
    expect(alertsPageSource).not.toContain('Enter an Apprise API server URL to test API delivery.');
    expect(alertsPageSource).not.toContain('Test email sent successfully! Check your inbox.');
    expect(alertsPageSource).not.toContain('Failed to send test email');
    expect(alertsPageSource).not.toContain('Test Apprise notification sent successfully!');
    expect(alertsPageSource).not.toContain('Failed to send test notification');
    expect(alertsPageSource).not.toContain('Test webhook sent successfully!');
    expect(alertsPageSource).not.toContain('Failed to send test webhook');
    expect(alertsPageSource).not.toContain('Enable only when the Apprise API uses a self-signed certificate.');
    expect(alertsPageSource).not.toContain(
      "isAlertsActive() ? 'translate-x-5' : 'translate-x-0'",
    );
    expect(alertsPageSource).not.toContain(
      'inline-flex items-center gap-2 rounded-full border border-blue-200',
    );
    expect(alertsPageSource).not.toContain('class="px-2 py-0.5 text-xs bg-blue-100');
    expect(alertsPageSource).not.toContain('class="h-2 w-2 rounded-full bg-yellow-500"');
    expect(alertsPageSource).not.toContain('class="h-2 w-2 rounded-full bg-red-500"');
    expect(alertsPageSource).not.toContain(
      "areAlertsDisabled()\n                                  ? 'cursor-not-allowed text-muted bg-surface-alt'",
    );
    expect(alertsPageSource).not.toContain(
      "activeTab() === item.id\n                                    ? 'bg-blue-50 text-blue-600 dark:bg-blue-900 dark:text-blue-200'",
    );
    expect(alertsPageSource).not.toContain(
      "activeTab() === tab.id\n                                ? 'bg-surface text-base-content shadow-sm'",
    );
    expect(alertsPageSource).not.toContain(
      "grouping().byNode\n                        ? 'border-blue-500 bg-blue-50 shadow-sm dark:bg-blue-900'",
    );
    expect(alertsPageSource).not.toContain(
      "grouping().byGuest\n                        ? 'border-blue-500 bg-blue-50 shadow-sm dark:bg-blue-900'",
    );
    expect(alertsPageSource).not.toContain(
      "grouping().byNode ? 'border-blue-500 bg-blue-500' : 'border-border'",
    );
    expect(alertsPageSource).not.toContain(
      "grouping().byGuest ? 'border-blue-500 bg-blue-500' : 'border-border'",
    );
    expect(alertsPageSource).not.toContain(
      "quietHours().days[day.id] ? 'rounded-md bg-blue-500 text-white shadow-sm' : 'rounded-md text-muted hover:bg-surface-hover '",
    );
    expect(alertsPageSource).not.toContain(
      "quietHours().suppress[option.key]\n                            ? 'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-500'",
    );
    expect(alertsPageSource).not.toContain(
      "quietHours().suppress[option.key]\n                              ? 'border-blue-500 bg-blue-500'",
    );
    expect(alertsPageSource).not.toContain('Search alerts...');
    expect(alertsPageSource).not.toContain('No alerts found');
    expect(alertsPageSource).not.toContain('No alerts');
    expect(alertsPageSource).not.toContain('Try adjusting your filters or check back later');
    expect(alertsPageSource).not.toContain('Resource incidents');
    expect(alertsPageSource).not.toContain('Loading incidents...');
    expect(alertsPageSource).not.toContain('No incidents recorded for this resource yet.');
    expect(alertsPageSource).not.toContain('Refreshing...');
    expect(alertsPageSource).not.toContain('Acknowledged by ');
    expect(alertsPageSource).not.toContain('No events match the selected filters.');
    expect(alertsPageSource).not.toContain('Showing last ');
    expect(alertsPageSource).not.toContain(
      'You have unsaved changes that will be lost. Discard changes and leave?',
    );
    expect(alertsPageSource).not.toContain(
      "Alerts activated! You'll now receive alerts when issues are detected.",
    );
    expect(alertsPageSource).not.toContain('Unable to activate alerts. Please try again.');
    expect(alertsPageSource).not.toContain(
      'Alerts deactivated. Nothing will be sent until you activate them again.',
    );
    expect(alertsPageSource).not.toContain('Unable to deactivate alerts. Please try again.');
    expect(alertsPageSource).not.toContain('Changes discarded');
    expect(alertsPageSource).not.toContain('Failed to reload configuration');
    expect(alertsPageSource).not.toContain('• Quiet hours active from ');
    expect(alertsPageSource).not.toContain('• Suppressing ');
    expect(alertsPageSource).not.toContain('• Recovery notifications enabled when alerts clear');
    expect(alertsPageSource).not.toContain(
      'CPU, memory, disk, and network thresholds stay quiet.',
    );
    expect(alertsPageSource).not.toContain(
      'Silence storage usage, disk health, and ZFS events.',
    );
    expect(alertsPageSource).not.toContain(
      'Skip connectivity and powered-off alerts during backups.',
    );
    expect(alertsPageSource).not.toContain('Minimum time between alerts for the same issue');
    expect(alertsPageSource).not.toContain('Per guest/metric combination');
    expect(alertsPageSource).not.toContain(
      'Alerts within this window are grouped together. Set to 0 to send immediately.',
    );
    expect(alertsPageSource).not.toContain('Pause non-critical alerts during specific times.');
    expect(alertsPageSource).not.toContain('Start time');
    expect(alertsPageSource).not.toContain('End time');
    expect(alertsPageSource).not.toContain('Timezone');
    expect(alertsPageSource).not.toContain('Discarding...');
    expect(alertsPageSource).not.toContain('Add a note for this incident...');
    expect(alertsPageSource).not.toContain('Save Note');
    expect(alertsPageSource).not.toContain('Loading alert history...');
    expect(alertsPageSource).not.toContain("title: 'Alerts Overview'");
    expect(alertsPageSource).not.toContain("title: 'Alert Thresholds'");
    expect(alertsPageSource).not.toContain("title: 'Notification Destinations'");
    expect(alertsPageSource).not.toContain("title: 'Maintenance Schedule'");
    expect(alertsPageSource).not.toContain("title: 'Alert History'");
    expect(alertsPageSource).not.toContain("title: 'Alerts'");
    expect(alertsPageSource).not.toContain("description: 'Manage alerting configuration.'");
    expect(alertsPageSource).not.toContain('Administrative Actions');
    expect(alertsPageSource).not.toContain("label: 'Status'");
    expect(alertsPageSource).not.toContain("label: 'Configuration'");
    expect(alertsPageSource).not.toContain("label: 'Overview'");
    expect(alertsPageSource).not.toContain("label: 'History'");
    expect(alertsPageSource).not.toContain("label: 'Thresholds'");
    expect(alertsPageSource).not.toContain("label: 'Notifications'");
    expect(alertsPageSource).not.toContain("label: 'Schedule'");
    expect(alertsPageSource).not.toContain(
      'Permanently clear all alert history. Use with caution - this action cannot be undone.',
    );
    expect(alertsPageSource).not.toContain(
      'Error clearing alert history: Please check your connection and try again.',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentStatusPresentation',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentLevelBadgeClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertHistoryStatusPresentation',
    );
    expect(alertHistoryPresentationSource).toContain(
      'export function getAlertHistorySourcePresentation',
    );
    expect(alertHistoryPresentationSource).toContain(
      'export function getAlertHistoryResourceTypeBadgeClass',
    );
    expect(alertActivationPresentationSource).toContain(
      'export function getAlertActivationPresentation',
    );
    expect(alertFrequencyPresentationSource).toContain(
      'export function getAlertFrequencySelectionPresentation',
    );
    expect(alertFrequencyPresentationSource).toContain(
      'export function getAlertFrequencyClearFilterButtonClass',
    );
    expect(alertSeverityPresentationSource).toContain('export function getAlertSeverityDotClass');
    expect(alertSeverityPresentationSource).toContain(
      'export function getAlertSeverityCompactLabel',
    );
    expect(alertTabsPresentationSource).toContain('export function getAlertsSidebarTabClass');
    expect(alertTabsPresentationSource).toContain('export function getAlertsMobileTabClass');
    expect(alertTabsPresentationSource).toContain('export function getAlertsTabTitle');
    expect(alertTabsPresentationSource).toContain('export function getAlertsTabGroups');
    expect(alertGroupingPresentationSource).toContain(
      'export function getAlertGroupingCardClass',
    );
    expect(alertGroupingPresentationSource).toContain(
      'export function getAlertGroupingCheckboxClass',
    );
    expect(alertSchedulePresentationSource).toContain(
      'export function getAlertQuietDayButtonClass',
    );
    expect(alertSchedulePresentationSource).toContain(
      'export function getAlertQuietSuppressCardClass',
    );
    expect(alertSchedulePresentationSource).toContain(
      'export function getAlertQuietSuppressCheckboxClass',
    );
    expect(configuredNodeTablesSource).toContain('getConfiguredNodeCapabilityBadges');
    expect(configuredNodeTablesSource).toContain('resolveConfiguredPveNodeStatusIndicator');
    expect(configuredNodeTablesSource).toContain('resolveConfiguredInstanceStatusIndicator');
    expect(configuredNodeTablesSource).not.toContain("'monitorVMs' in node");
    expect(configuredNodeTablesSource).not.toContain("'monitorDatastores' in node");
    expect(configuredNodeTablesSource).not.toContain('const resolveConfiguredNodeStatusIndicator =');
    expect(configuredNodeTablesSource).not.toContain('const resolvePbsStatusIndicator =');
    expect(configuredNodeTablesSource).not.toContain('const resolvePmgStatusIndicator =');
    expect(configuredNodeCapabilityPresentationSource).toContain(
      'export function getConfiguredNodeCapabilityBadges',
    );
    expect(configuredNodeStatusPresentationSource).toContain(
      'export function resolveConfiguredNodeStatusIndicator',
    );
    expect(configuredNodeStatusPresentationSource).toContain(
      'export function resolveConfiguredPveNodeStatusIndicator',
    );
    expect(configuredNodeStatusPresentationSource).toContain(
      'export function resolveConfiguredInstanceStatusIndicator',
    );
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
    expect(auditWebhookPanelSource).toContain('@/utils/auditWebhookPresentation');
    expect(auditWebhookPanelSource).toContain('getAuditWebhookFeatureGateCopy');
    expect(auditWebhookPanelSource).toContain('getAuditWebhookEmptyStateCopy');
    expect(auditWebhookPanelSource).toContain('getAuditWebhookLoadingState');
    expect(auditWebhookPanelSource).toContain('AUDIT_WEBHOOK_READONLY_NOTICE_CLASS');
    expect(auditWebhookPanelSource).toContain('AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS');
    expect(auditWebhookPanelSource).toContain('AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS');
    expect(auditWebhookPanelSource).not.toContain('No audit webhooks configured yet.');
    expect(auditWebhookPanelSource).not.toContain('Loading audit webhooks…');
    expect(auditWebhookPanelSource).not.toContain('Audit Webhooks (Pro)');
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookFeatureGateCopy',
    );
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookEmptyStateCopy',
    );
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookLoadingState',
    );
    expect(auditLogPanelSource).toContain('getAuditLogLoadingState');
    expect(auditLogPanelSource).toContain('getAuditLogEmptyState');
    expect(auditLogPanelSource).not.toContain('No audit events found');
    expect(auditLogPresentationSource).toContain('export function getAuditLogLoadingState');
    expect(auditLogPresentationSource).toContain('export function getAuditLogEmptyState');
    expect(auditLogPanelSource).toContain('getAuditEventTypeBadgeClass');
    expect(auditLogPanelSource).toContain('getAuditVerificationBadgePresentation');
    expect(auditLogPanelSource).toContain('getAuditEventStatusPresentation');
    expect(auditLogPanelSource).toContain('AUDIT_REFRESH_BUTTON_CLASS');
    expect(auditLogPanelSource).toContain('AUDIT_VERIFY_ALL_BUTTON_CLASS');
    expect(auditLogPanelSource).toContain('AUDIT_VERIFY_ROW_BUTTON_CLASS');
    expect(auditLogPanelSource).not.toContain('const getEventTypeBadge =');
    expect(auditLogPanelSource).not.toContain('const getVerificationBadge =');
    expect(auditLogPanelSource).not.toContain('const getEventIcon =');
    expect(auditLogPresentationSource).toContain('export function getAuditEventTypeBadgeClass');
    expect(auditLogPresentationSource).toContain(
      'export function getAuditVerificationBadgePresentation',
    );
    expect(auditLogPresentationSource).toContain(
      'export function getAuditEventStatusPresentation',
    );
    expect(auditLogPresentationSource).toContain('export const AUDIT_REFRESH_BUTTON_CLASS');
    expect(auditLogPresentationSource).toContain('export const AUDIT_VERIFY_ALL_BUTTON_CLASS');
    expect(auditLogPresentationSource).toContain('export const AUDIT_VERIFY_ROW_BUTTON_CLASS');
    expect(diagnosticsPanelSource).toContain('getStatusIndicatorBadgeToneClasses(');
    expect(diagnosticsPanelSource).toContain('DIAGNOSTICS_EMPTY_PBS_MESSAGE');
    expect(diagnosticsPanelSource).not.toContain('No PBS configured');
    expect(diagnosticsPanelSource).not.toContain(
      "'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'",
    );
    expect(diagnosticsPanelSource).not.toContain(
      "'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'",
    );
    expect(diagnosticsPresentationSource).toContain('export const DIAGNOSTICS_EMPTY_PBS_MESSAGE');
    expect(updatesSettingsPanelSource).toContain('getUpdateBuildBadges');
    expect(updatesSettingsPanelSource).toContain('getUpdateAvailabilityHeading');
    expect(updatesSettingsPanelSource).toContain('getUpdatePrimaryStatusLabel');
    expect(updatesSettingsPanelSource).toContain('getUpdateCheckModeLabel');
    expect(updatesSettingsPanelSource).not.toContain('Auto-check enabled');
    expect(updatesSettingsPanelSource).not.toContain('Manual checks only');
    expect(updatesSettingsPanelSource).not.toContain('Update Ready');
    expect(updatesSettingsPanelSource).not.toContain('Up to date');
    expect(updatesPresentationSource).toContain('export function getUpdateBuildBadges');
    expect(updatesPresentationSource).toContain('export function getUpdateAvailabilityHeading');
    expect(updatesPresentationSource).toContain('export function getUpdatePrimaryStatusLabel');
    expect(updatesPresentationSource).toContain('export function getUpdateCheckModeLabel');
    expect(aiSettingsSource).toContain('getAIProviderTestResultTextClass');
    expect(aiSettingsSource).toContain('getAISettingsLoadingState');
    expect(aiSettingsSource).toContain('getAISettingsLoadErrorMessage');
    expect(aiSettingsSource).toContain('getAISettingsRetryLabel');
    expect(aiSettingsSource).toContain('getAIChatSessionsLoadingState');
    expect(aiSettingsSource).toContain('getAIChatSessionsEmptyState');
    expect(aiSettingsSource).toContain('getAIModelsLoadErrorMessage');
    expect(aiSettingsSource).toContain('getAIChatSessionsLoadErrorMessage');
    expect(aiSettingsSource).toContain('getAISessionSummarizeErrorMessage');
    expect(aiSettingsSource).toContain('getAISessionDiffErrorMessage');
    expect(aiSettingsSource).toContain('getAISessionRevertErrorMessage');
    expect(aiSettingsSource).toContain('getAISettingsSaveErrorMessage');
    expect(aiSettingsSource).toContain('getAICredentialsClearErrorMessage');
    expect(aiSettingsSource).toContain('getAISettingsToggleErrorMessage');
    expect(aiSettingsSource).not.toContain('Loading Pulse Assistant settings...');
    expect(aiSettingsSource).not.toContain('Loading chat sessions...');
    expect(aiSettingsSource).not.toContain('No chat sessions yet. Start a chat to create one.');
    expect(aiSettingsSource).not.toContain(
      'Failed to load Pulse Assistant settings. Your configuration could not be retrieved.',
    );
    expect(aiSettingsSource).not.toContain("'Failed to load models'");
    expect(aiSettingsSource).not.toContain("'Failed to load chat sessions.'");
    expect(aiSettingsSource).not.toContain("'Failed to summarize session.'");
    expect(aiSettingsSource).not.toContain("'Failed to get session diff.'");
    expect(aiSettingsSource).not.toContain("'Failed to revert session.'");
    expect(aiSettingsSource).not.toContain("'Failed to save Pulse Assistant settings'");
    expect(aiSettingsSource).not.toContain("'Failed to clear credentials'");
    expect(aiSettingsSource).not.toContain("'Failed to update Pulse Assistant setting'");
    expect(aiSettingsSource).not.toContain(
      "providerTestResult()?.success ? 'text-green-600' : 'text-red-600'",
    );
    expect(aiIntelligenceSource).toContain('getPatrolSummaryPresentation');
    expect(aiIntelligenceSource).toContain('getAIQuickstartCreditsPresentation');
    expect(aiIntelligenceSource).not.toContain(
      "summaryStats().criticalFindings > 0\n                        ? 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800'",
    );
    expect(aiIntelligenceSource).not.toContain(
      "summaryStats().warningFindings > 0\n                        ? 'bg-amber-50 dark:bg-amber-900 border-amber-200 dark:border-amber-800'",
    );
    expect(aiIntelligenceSource).not.toContain(
      "summaryStats().fixedCount > 0\n                        ? 'bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800'",
    );
    expect(patrolSummaryPresentationSource).toContain(
      'export function getPatrolSummaryPresentation',
    );
    expect(aiIntelligenceSource).not.toContain(
      "(patrolStatus()?.quickstart_credits_remaining ?? 0) > 0\n                  ? 'bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300'",
    );
    expect(aiQuickstartPresentationSource).toContain(
      'export function getAIQuickstartCreditsPresentation',
    );
    expect(investigationMessagesSource).toContain('getInvestigationMessagesState');
    expect(investigationMessagesSource).not.toContain('Loading messages...');
    expect(investigationMessagesSource).not.toContain('No investigation messages available.');
    expect(investigationSectionSource).toContain('getInvestigationSectionState');
    expect(investigationSectionSource).not.toContain('Loading investigation...');
    expect(investigationSectionSource).not.toContain(
      'No investigation data available. Enable patrol autonomy to investigate findings.',
    );
    expect(runHistoryPanelSource).toContain('getRunHistoryEmptyState');
    expect(runHistoryPanelSource).toContain('getRunHistoryLoadingState');
    expect(runHistoryPanelSource).not.toContain(
      'No patrol runs yet. Trigger a run to populate history.',
    );
    expect(runHistoryPanelSource).not.toContain('Loading run history…');
    expect(runToolCallTraceSource).toContain('getToolCallsLoadingState');
    expect(runToolCallTraceSource).toContain('getToolCallsUnavailableState');
    expect(runToolCallTraceSource).not.toContain('Loading tool calls...');
    expect(runToolCallTraceSource).not.toContain(
      'Tool call details not available for this run.',
    );
    expect(patrolEmptyStatePresentationSource).toContain(
      'export function getInvestigationMessagesState',
    );
    expect(patrolEmptyStatePresentationSource).toContain(
      'export function getInvestigationSectionState',
    );
    expect(patrolEmptyStatePresentationSource).toContain(
      'export function getRunHistoryEmptyState',
    );
    expect(patrolRunPresentationSource).toContain('export function getRunHistoryLoadingState');
    expect(patrolRunPresentationSource).toContain('export function getToolCallsLoadingState');
    expect(patrolRunPresentationSource).toContain(
      'export function getToolCallsUnavailableState',
    );
    expect(systemLogsPanelSource).toContain('getSystemLogLineClass');
    expect(systemLogsPanelSource).toContain('getSystemLogStreamPresentation');
    expect(systemLogsPanelSource).not.toContain("log.includes('ERR')");
    expect(systemLogsPanelSource).not.toContain("isPaused() ? 'Stream Paused' : 'Live'");
    expect(systemLogsPanelSource).not.toContain(
      "'bg-amber-100 text-amber-600 dark:bg-amber-900 dark:text-amber-400'",
    );
    expect(systemLogsPresentationSource).toContain('export function getSystemLogLineClass');
    expect(systemLogsPresentationSource).toContain(
      'export function getSystemLogStreamPresentation',
    );
    expect(thresholdSliderSource).toContain('getThresholdSliderTextClass');
    expect(thresholdSliderSource).toContain('getThresholdSliderFillClass');
    expect(thresholdSliderSource).not.toContain('const colorMap');
    expect(thresholdSliderPresentationSource).toContain(
      'export function getThresholdSliderTextClass',
    );
    expect(thresholdSliderPresentationSource).toContain(
      'export function getThresholdSliderFillClass',
    );
    expect(aiCostDashboardSource).toContain('getAICostRangeButtonClass');
    expect(aiCostDashboardSource).toContain('getAICostLoadingState');
    expect(aiCostDashboardSource).toContain('AI_COST_EMPTY_STATE');
    expect(aiCostDashboardSource).toContain('AI_COST_DAILY_USD_EMPTY_STATE');
    expect(aiCostDashboardSource).toContain('AI_COST_DAILY_TOKEN_EMPTY_STATE');
    expect(aiCostDashboardSource).not.toContain('No usage data yet.');
    expect(aiCostDashboardSource).not.toContain('No daily USD trend yet.');
    expect(aiCostDashboardSource).not.toContain('No daily token trend yet.');
    expect(aiCostDashboardSource).not.toContain('Loading usage…');
    expect(aiCostDashboardSource).not.toContain(
      "'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'",
    );
    expect(aiCostPresentationSource).toContain('export const AI_COST_EMPTY_STATE');
    expect(aiCostPresentationSource).toContain('export function getAICostLoadingState');
    expect(aiCostPresentationSource).toContain('export const AI_COST_DAILY_USD_EMPTY_STATE');
    expect(aiCostPresentationSource).toContain('export const AI_COST_DAILY_TOKEN_EMPTY_STATE');
    expect(modelSelectorSource).toContain('AI_CHAT_MODEL_SELECTOR_EMPTY_STATE');
    expect(modelSelectorSource).not.toContain('No matching models.');
    expect(aiChatPresentationSource).toContain('export const AI_CHAT_MODEL_SELECTOR_EMPTY_STATE');
    expect(aiChatPresentationSource).toContain('export const AI_CHAT_SESSION_EMPTY_STATE');
    expect(remediationStatusSource).toContain('getRemediationPresentation');
    expect(remediationStatusSource).not.toContain(
      "'bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800'",
    );
    expect(remediationStatusSource).not.toContain(
      "'bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800'",
    );
    expect(remediationStatusSource).not.toContain("'Fix executed successfully'");
    expect(remediationStatusSource).not.toContain("'Fix failed'");
    expect(remediationPresentationSource).toContain('export function getRemediationPresentation');
  });

  it('keeps dashboard empty-state copy in a shared presentation utility', () => {
    expect(dashboardSource).toContain('getDashboardInfrastructureEmptyState');
    expect(dashboardSource).toContain('getDashboardGuestsEmptyState');
    expect(dashboardSource).toContain('getDashboardLoadingState');
    expect(dashboardSource).toContain('getDashboardDisconnectedState');
    expect(dashboardSource).not.toContain('Loading dashboard data...');
    expect(dashboardSource).not.toContain('Connection lost');
    expect(dashboardSource).not.toContain('Attempting to reconnect…');
    expect(dashboardSource).not.toContain('Unable to connect to the backend server');
    expect(dashboardSource).not.toContain('Reconnect now');
    expect(dashboardRouteSource).toContain('getDashboardDisconnectedBannerState');
    expect(dashboardRouteSource).toContain('getDashboardUnavailableState');
    expect(dashboardRouteSource).toContain('getDashboardNoResourcesState');
    expect(dashboardRouteSource).not.toContain(
      'Real-time data is currently unavailable. Showing last-known state.',
    );
    expect(dashboardRouteSource).not.toContain(
      'Real-time data is reconnecting. Showing last-known state.',
    );
    expect(dashboardRouteSource).not.toContain('Dashboard unavailable');
    expect(dashboardRouteSource).not.toContain(
      'Real-time dashboard data is currently unavailable. Reconnect to try again.',
    );
    expect(dashboardRouteSource).not.toContain('No resources yet');
    expect(dashboardRouteSource).not.toContain(
      'Once connected platforms report resources, your dashboard overview will appear here.',
    );
    expect(dashboardEmptyStatePresentationSource).toContain(
      'export function getDashboardInfrastructureEmptyState',
    );
    expect(dashboardEmptyStatePresentationSource).toContain(
      'export function getDashboardGuestsEmptyState',
    );
    expect(dashboardEmptyStatePresentationSource).toContain(
      'export function getDashboardLoadingState',
    );
    expect(dashboardEmptyStatePresentationSource).toContain(
      'export function getDashboardDisconnectedState',
    );
    expect(dashboardEmptyStatePresentationSource).toContain(
      'export function getDashboardDisconnectedBannerState',
    );
    expect(dashboardEmptyStatePresentationSource).toContain(
      'export function getDashboardUnavailableState',
    );
    expect(dashboardEmptyStatePresentationSource).toContain(
      'export function getDashboardNoResourcesState',
    );
  });

  it('keeps dashboard guest fallback copy in a shared presentation utility', () => {
    expect(guestRowSource).toContain('getDashboardGuestBackupStatusPresentation');
    expect(guestRowSource).toContain('getDashboardGuestBackupTooltip');
    expect(guestRowSource).toContain('getDashboardGuestNetworkEmptyState');
    expect(guestRowSource).toContain('getDashboardGuestDiskStatusMessage');
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

  it('keeps infrastructure page empty-state copy in a shared presentation utility', () => {
    expect(infrastructurePageSource).toContain('getInfrastructureEmptyState');
    expect(infrastructurePageSource).toContain('getInfrastructureFilterEmptyState');
    expect(infrastructurePageSource).toContain('getInfrastructureLoadFailureState');
    expect(infrastructurePageSource).not.toContain('No infrastructure resources yet');
    expect(infrastructurePageSource).not.toContain('No resources match filters');
    expect(infrastructurePageSource).not.toContain(
      'Add Proxmox VE nodes or install the Pulse agent on your infrastructure to start monitoring.',
    );
    expect(infrastructurePageSource).not.toContain(
      'Try adjusting the search, source, or status filters.',
    );
    expect(infrastructurePageSource).not.toContain('Unable to load infrastructure');
    expect(infrastructurePageSource).not.toContain(
      'We couldn’t fetch unified resources. Check connectivity or retry.',
    );
    expect(infrastructurePageSource).not.toContain('Retry');
    expect(infrastructureEmptyStatePresentationSource).toContain(
      'export function getInfrastructureEmptyState',
    );
    expect(infrastructureEmptyStatePresentationSource).toContain(
      'export function getInfrastructureFilterEmptyState',
    );
    expect(infrastructureEmptyStatePresentationSource).toContain(
      'export function getInfrastructureLoadFailureState',
    );
  });

  it('keeps alert incident timeline state copy in a shared presentation utility', () => {
    expect(alertOverviewTabSource).toContain('getAlertTimelineLoadingState');
    expect(alertOverviewTabSource).toContain('getAlertTimelineFilterEmptyState');
    expect(alertOverviewTabSource).toContain('getAlertTimelineEmptyState');
    expect(alertOverviewTabSource).toContain('getAlertTimelineUnavailableState');
    expect(alertOverviewTabSource).toContain('getAlertTimelineFailureState');
    expect(alertOverviewTabSource).toContain('getAlertIncidentEventFilterContainerClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentEventFilterChipClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentAcknowledgedBadgeClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentTimelineEventCardClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentNoteTextareaClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentNoteSaveButtonClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentTimelineMetaRowClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentTimelineHeadingClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentTimelineDetailClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentTimelineCommandClass');
    expect(alertOverviewTabSource).toContain('getAlertIncidentTimelineOutputClass');
    expect(alertOverviewTabSource).toContain('getAlertOverviewCardPresentation');
    expect(alertOverviewTabSource).toContain('getAlertOverviewAcknowledgedBadgeClass');
    expect(alertOverviewTabSource).toContain('getAlertOverviewStartedAtClass');
    expect(alertOverviewTabSource).toContain('getAlertOverviewPrimaryActionClass');
    expect(alertOverviewTabSource).toContain('getAlertOverviewSecondaryActionClass');
    expect(alertOverviewTabSource).not.toContain('Loading timeline...');
    expect(alertOverviewTabSource).not.toContain(
      'No timeline events match the selected filters.',
    );
    expect(alertOverviewTabSource).not.toContain('No timeline events yet.');
    expect(alertOverviewTabSource).not.toContain('No incident timeline available.');
    expect(alertOverviewTabSource).not.toContain('Failed to load timeline.');
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertTimelineLoadingState',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertTimelineFilterEmptyState',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertTimelineEmptyState',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertTimelineUnavailableState',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertTimelineFailureState',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertHistorySearchPlaceholder',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertOverviewCardPresentation',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertOverviewAcknowledgedBadgeClass',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertOverviewStartedAtClass',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertOverviewPrimaryActionClass',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertOverviewSecondaryActionClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentEventFilterContainerClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentEventFilterActionButtonClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentEventFilterChipClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentAcknowledgedBadgeClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentTimelineEventCardClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentNoteTextareaClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentNoteSaveButtonClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentTimelineMetaRowClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentTimelineHeadingClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentTimelineDetailClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentTimelineCommandClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertIncidentTimelineOutputClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentCardClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentSummaryRowClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentToggleButtonClass',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentTruncatedEventsLabel',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertHistoryEmptyState',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertsPageHeaderMeta',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertBucketCountLabel',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentLoadingState',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentEmptyState',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentPanelTitle',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentRefreshLabel',
    );
    expect(alertIncidentPresentationSource).toContain(
      'export function getAlertResourceIncidentToggleLabel',
    );
    expect(alertDestinationsPresentationSource).toContain(
      'export function getAlertDestinationsConfigLoadError',
    );
    expect(alertDestinationsPresentationSource).toContain(
      'export function getAlertDestinationsLoadErrorBanner',
    );
    expect(alertDestinationsPresentationSource).toContain(
      'export function getAlertDestinationsRetryLabel',
    );
    expect(alertDestinationsPresentationSource).toContain(
      'export function getAlertDestinationsAppriseTargetsHelp',
    );
    expect(alertDestinationsPresentationSource).toContain(
      'export function getAlertDestinationsAppriseTestLabel',
    );
    expect(alertDestinationsPresentationSource).toContain(
      'export function getAlertDestinationsAppriseTestError',
    );
    expect(alertDestinationsPresentationSource).toContain(
      'export function getAlertDestinationsStatusLabel',
    );
  });

  it('keeps alert resource table vocabulary in a shared presentation utility', () => {
    expect(alertResourceTableSource).toContain('getAlertResourceTableEmptyState');
    expect(alertResourceTableSource).toContain('getAlertResourceTableNoResultsState');
    expect(alertResourceTableSource).toContain('getAlertResourceTableCustomBadgeLabel');
    expect(alertResourceTableSource).toContain('getAlertResourceTableMetricPlaceholder');
    expect(alertResourceTableSource).toContain('getAlertResourceTableOverrideNotePlaceholder');
    expect(alertResourceTableSource).toContain(
      'getAlertResourceTableResetFactoryDefaultsLabel',
    );
    expect(alertResourceTableSource).toContain('getAlertResourceTableAlertDelayLabel');
    expect(alertResourceTableSource).toContain('getAlertResourceTableMetricInputTitle');
    expect(alertResourceTableSource).toContain('getAlertResourceTableEditMetricTitle');
    expect(alertResourceTableSource).not.toContain('No resources available.');
    expect(alertResourceTableSource).not.toContain('No {props.title.toLowerCase()} found');
    expect(alertResourceTableSource).not.toContain('Add a note about this override (optional)');
    expect(alertResourceTableSource).not.toContain('Add a note...');
    expect(alertResourceTableSource).not.toContain('Reset to factory defaults');
    expect(alertResourceTableSource).not.toContain('Alert Delay (s)');
    expect(alertResourceTableSource).not.toContain('Click to edit this metric');
    expect(alertResourceTableSource).not.toContain('Set to -1 to disable alerts for this metric');
    expect(alertResourceTablePresentationSource).toContain(
      'export const ALERT_RESOURCE_TABLE_EMPTY_STATE',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableEmptyState',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableNoResultsState',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableCustomBadgeLabel',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableMetricPlaceholder',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableOverrideNotePlaceholder',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableResetFactoryDefaultsLabel',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableAlertDelayLabel',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableMetricInputTitle',
    );
    expect(alertResourceTablePresentationSource).toContain(
      'export function getAlertResourceTableEditMetricTitle',
    );
  });

  it('keeps alert email provider vocabulary and placeholders in a shared presentation utility', () => {
    expect(emailProviderSelectSource).toContain('getAlertEmailProviderOptionLabel');
    expect(emailProviderSelectSource).toContain('getAlertEmailSetupInstructionsToggleLabel');
    expect(emailProviderSelectSource).toContain('getAlertEmailRecipientsPlaceholder');
    expect(emailProviderSelectSource).toContain('getAlertEmailAdvancedToggleLabel');
    expect(emailProviderSelectSource).toContain('getAlertEmailTestButtonLabel');
    expect(emailProviderSelectSource).not.toContain('Manual configuration');
    expect(emailProviderSelectSource).not.toContain('smtp.example.com');
    expect(emailProviderSelectSource).not.toContain('noreply@example.com');
    expect(emailProviderSelectSource).not.toContain('admin@example.com');
    expect(emailProviderSelectSource).not.toContain('Send test email');
    expect(emailProviderSelectSource).not.toContain('Sending test email…');
    expect(alertEmailPresentationSource).toContain(
      'export function getAlertEmailProviderOptionLabel',
    );
    expect(alertEmailPresentationSource).toContain(
      'export function getAlertEmailRecipientsPlaceholder',
    );
    expect(alertEmailPresentationSource).toContain(
      'export function getAlertEmailTestButtonLabel',
    );
  });

  it('keeps alert bulk-edit dialog labels in a shared presentation utility', () => {
    expect(bulkEditDialogSource).toContain('getAlertBulkEditDescription');
    expect(bulkEditDialogSource).toContain('getAlertBulkEditApplyLabel');
    expect(bulkEditDialogSource).toContain('getAlertBulkEditOpenLabel');
    expect(alertResourceTableSource).toContain('getAlertBulkEditOpenLabel');
    expect(bulkEditDialogSource).not.toContain('Bulk Edit Settings');
    expect(bulkEditDialogSource).not.toContain('Unchanged');
    expect(bulkEditDialogSource).not.toContain('Clear');
    expect(bulkEditDialogSource).not.toContain('Apply to ');
    expect(alertBulkEditPresentationSource).toContain(
      'export const ALERT_BULK_EDIT_DIALOG_TITLE',
    );
    expect(alertBulkEditPresentationSource).toContain(
      'export function getAlertBulkEditDescription',
    );
    expect(alertBulkEditPresentationSource).toContain(
      'export function getAlertBulkEditApplyLabel',
    );
    expect(alertBulkEditPresentationSource).toContain(
      'export function getAlertBulkEditOpenLabel',
    );
  });

  it('keeps alert webhook service vocabulary and action copy in a shared presentation utility', () => {
    expect(webhookConfigSource).toContain('getAlertWebhookServices');
    expect(webhookConfigSource).toContain('getAlertWebhookServiceLabel');
    expect(webhookConfigSource).toContain('getAlertWebhookSummaryLabel');
    expect(webhookConfigSource).toContain('getAlertWebhookToggleAllLabel');
    expect(webhookConfigSource).toContain('getAlertWebhookToggleLabel');
    expect(webhookConfigSource).toContain('getAlertWebhookNamePlaceholder');
    expect(webhookConfigSource).toContain('getAlertWebhookUrlPlaceholder');
    expect(webhookConfigSource).toContain('getAlertWebhookMentionPlaceholder');
    expect(webhookConfigSource).toContain('getAlertWebhookMentionHelp');
    expect(webhookConfigSource).toContain('getAlertWebhookTestLabel');
    expect(webhookConfigSource).toContain('getAlertWebhookSubmitLabel');
    expect(webhookConfigSource).not.toContain('Custom webhook endpoint');
    expect(webhookConfigSource).not.toContain('Discord server webhook');
    expect(webhookConfigSource).not.toContain('My Webhook');
    expect(webhookConfigSource).not.toContain('https://example.com/webhook');
    expect(webhookConfigSource).not.toContain('Optional — tag users or groups');
    expect(webhookConfigSource).not.toContain('Enable All');
    expect(webhookConfigSource).not.toContain('Disable All');
    expect(webhookConfigSource).not.toContain('Enable this webhook');
    expect(alertWebhookPresentationSource).toContain('export const ALERT_WEBHOOK_SERVICES');
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookServiceLabel',
    );
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookServices',
    );
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookTestLabel',
    );
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookSubmitLabel',
    );
  });
});
