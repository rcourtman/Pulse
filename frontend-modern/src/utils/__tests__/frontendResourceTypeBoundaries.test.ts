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
import workloadTypePresentationSource from '@/utils/workloadTypePresentation.ts?raw';
import sourcePlatformsSource from '@/utils/sourcePlatforms.ts?raw';
import discoveryTargetSource from '@/utils/discoveryTarget.ts?raw';
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
import recoveryStatusPresentationSource from '@/utils/recoveryStatusPresentation.ts?raw';
import recoverySummaryPresentationSource from '@/utils/recoverySummaryPresentation.ts?raw';
import recoveryTablePresentationSource from '@/utils/recoveryTablePresentation.ts?raw';
import recoveryTimelineChartPresentationSource from '@/utils/recoveryTimelineChartPresentation.ts?raw';
import recoveryTimelinePresentationSource from '@/utils/recoveryTimelinePresentation.ts?raw';
import dashboardHelpersSource from '@/pages/DashboardPanels/dashboardHelpers.ts?raw';
import dashboardMetricPresentationSource from '@/utils/dashboardMetricPresentation.ts?raw';
import trendChartsSource from '@/pages/DashboardPanels/TrendCharts.tsx?raw';
import compositionPanelSource from '@/pages/DashboardPanels/CompositionPanel.tsx?raw';
import dashboardCompositionPresentationSource from '@/utils/dashboardCompositionPresentation.ts?raw';
import problemResourcesTableSource from '@/pages/DashboardPanels/ProblemResourcesTable.tsx?raw';
import problemResourcePresentationSource from '@/utils/problemResourcePresentation.ts?raw';
import kpiStripSource from '@/pages/DashboardPanels/KPIStrip.tsx?raw';
import recentAlertsPanelSource from '@/pages/DashboardPanels/RecentAlertsPanel.tsx?raw';
import dashboardAlertPresentationSource from '@/utils/dashboardAlertPresentation.ts?raw';
import diskListSource from '@/components/Storage/DiskList.tsx?raw';
import diskDetailSource from '@/components/Storage/DiskDetail.tsx?raw';
import storageControlsSource from '@/components/Storage/StorageControls.tsx?raw';
import storagePageControlsSource from '@/components/Storage/StoragePageControls.tsx?raw';
import storageCephSectionSource from '@/components/Storage/StorageCephSection.tsx?raw';
import storageContentCardSource from '@/components/Storage/StorageContentCard.tsx?raw';
import storagePageSource from '@/components/Storage/Storage.tsx?raw';
import storagePageBannerSource from '@/components/Storage/StoragePageBanner.tsx?raw';
import storagePageBannersSource from '@/components/Storage/StoragePageBanners.tsx?raw';
import storageCephSummaryCardSource from '@/components/Storage/StorageCephSummaryCard.tsx?raw';
import storagePoolsTableSource from '@/components/Storage/StoragePoolsTable.tsx?raw';
import storageGroupRowSource from '@/components/Storage/StorageGroupRow.tsx?raw';
import storagePoolDetailSource from '@/components/Storage/StoragePoolDetail.tsx?raw';
import storagePoolRowSource from '@/components/Storage/StoragePoolRow.tsx?raw';
import storageExpansionStateSource from '@/components/Storage/useStorageExpansionState.ts?raw';
import storageFilterStateSource from '@/components/Storage/useStorageFilterState.ts?raw';
import storagePageFiltersSource from '@/components/Storage/useStoragePageFilters.ts?raw';
import storagePageDataSource from '@/components/Storage/useStoragePageData.ts?raw';
import storagePageStatusHookSource from '@/components/Storage/useStoragePageStatus.ts?raw';
import storagePageStateSource from '@/components/Storage/storagePageState.ts?raw';
import storageResourceHighlightSource from '@/components/Storage/useStorageResourceHighlight.ts?raw';
import useStorageModelSource from '@/components/Storage/useStorageModel.ts?raw';
import useStorageCephModelSource from '@/components/Storage/useStorageCephModel.ts?raw';
import useStorageAlertStateSource from '@/components/Storage/useStorageAlertState.ts?raw';
import temperatureUtilSource from '@/utils/temperature.ts?raw';
import pmgInstanceDrawerSource from '@/components/PMG/PMGInstanceDrawer.tsx?raw';
import serviceHealthPresentationSource from '@/utils/serviceHealthPresentation.ts?raw';
import swarmServicesDrawerSource from '@/components/Docker/SwarmServicesDrawer.tsx?raw';
import k8sDeploymentsDrawerSource from '@/components/Kubernetes/K8sDeploymentsDrawer.tsx?raw';
import k8sNamespacesDrawerSource from '@/components/Kubernetes/K8sNamespacesDrawer.tsx?raw';
import k8sStatusPresentationSource from '@/utils/k8sStatusPresentation.ts?raw';
import raidCardSource from '@/components/shared/cards/RaidCard.tsx?raw';
import raidPresentationSource from '@/utils/raidPresentation.ts?raw';
import proLicensePanelSource from '@/components/Settings/ProLicensePanel.tsx?raw';
import securityPostureSummarySource from '@/components/Settings/SecurityPostureSummary.tsx?raw';
import securityWarningSource from '@/components/SecurityWarning.tsx?raw';
import licensePresentationSource from '@/utils/licensePresentation.ts?raw';
import securityScorePresentationSource from '@/utils/securityScorePresentation.ts?raw';
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
import pmgThreatPresentationSource from '@/utils/pmgThreatPresentation.ts?raw';
import pmgQueuePresentationSource from '@/utils/pmgQueuePresentation.ts?raw';
import pmgServiceHealthBadgeSource from '@/components/PMG/ServiceHealthBadge.tsx?raw';
import cephPageSource from '@/pages/Ceph.tsx?raw';
import diskPresentationSource from '@/features/storageBackups/diskPresentation.ts?raw';
import diskDetailPresentationSource from '@/features/storageBackups/diskDetailPresentation.ts?raw';
import storageRecordPresentationSource from '@/features/storageBackups/recordPresentation.ts?raw';
import storageModelCoreSource from '@/features/storageBackups/storageModelCore.ts?raw';
import resourceStorageMappingSource from '@/features/storageBackups/resourceStorageMapping.ts?raw';
import resourceStoragePresentationSource from '@/features/storageBackups/resourceStoragePresentation.ts?raw';
import storageAdapterCoreSource from '@/features/storageBackups/storageAdapterCore.ts?raw';
import storageAlertStateSource from '@/features/storageBackups/storageAlertState.ts?raw';
import cephRecordPresentationSource from '@/features/storageBackups/cephRecordPresentation.ts?raw';
import cephSummaryPresentationSource from '@/features/storageBackups/cephSummaryPresentation.ts?raw';
import storageDomainSource from '@/features/storageBackups/storageDomain.ts?raw';
import storagePoolDetailPresentationSource from '@/features/storageBackups/storagePoolDetailPresentation.ts?raw';
import storagePagePresentationSource from '@/features/storageBackups/storagePagePresentation.ts?raw';
import storagePageStatusSource from '@/features/storageBackups/storagePageStatus.ts?raw';
import storageRowPresentationSource from '@/features/storageBackups/rowPresentation.ts?raw';
import storageGroupPresentationSource from '@/features/storageBackups/groupPresentation.ts?raw';
import storageRowAlertPresentationSource from '@/features/storageBackups/storageRowAlertPresentation.ts?raw';
import storageAdaptersSource from '@/features/storageBackups/storageAdapters.ts?raw';
import deployStatusBadgeSource from '@/components/Infrastructure/deploy/DeployStatusBadge.tsx?raw';
import deployStatusPresentationSource from '@/utils/deployStatusPresentation.ts?raw';
import alertsPageSource from '@/pages/Alerts.tsx?raw';
import alertIncidentPresentationSource from '@/utils/alertIncidentPresentation.ts?raw';
import alertHistoryPresentationSource from '@/utils/alertHistoryPresentation.ts?raw';
import alertActivationPresentationSource from '@/utils/alertActivationPresentation.ts?raw';
import alertFrequencyPresentationSource from '@/utils/alertFrequencyPresentation.ts?raw';
import alertSeverityPresentationSource from '@/utils/alertSeverityPresentation.ts?raw';
import alertTabsPresentationSource from '@/utils/alertTabsPresentation.ts?raw';
import alertGroupingPresentationSource from '@/utils/alertGroupingPresentation.ts?raw';
import alertSchedulePresentationSource from '@/utils/alertSchedulePresentation.ts?raw';
import configuredNodeTablesSource from '@/components/Settings/ConfiguredNodeTables.tsx?raw';
import configuredNodeCapabilityPresentationSource from '@/utils/configuredNodeCapabilityPresentation.ts?raw';
import auditLogPanelSource from '@/components/Settings/AuditLogPanel.tsx?raw';
import auditLogPresentationSource from '@/utils/auditLogPresentation.ts?raw';
import diagnosticsPanelSource from '@/components/Settings/DiagnosticsPanel.tsx?raw';
import aiSettingsSource from '@/components/Settings/AISettings.tsx?raw';
import aiIntelligenceSource from '@/pages/AIIntelligence.tsx?raw';
import aiQuickstartPresentationSource from '@/utils/aiQuickstartPresentation.ts?raw';
import emptyStatePresentationSource from '@/utils/emptyStatePresentation.ts?raw';
import systemLogsPanelSource from '@/components/Settings/SystemLogsPanel.tsx?raw';
import systemLogsPresentationSource from '@/utils/systemLogsPresentation.ts?raw';
import patrolSummaryPresentationSource from '@/utils/patrolSummaryPresentation.ts?raw';
import aiCostDashboardSource from '@/components/AI/AICostDashboard.tsx?raw';
import remediationStatusSource from '@/components/patrol/RemediationStatus.tsx?raw';
import remediationPresentationSource from '@/utils/remediationPresentation.ts?raw';

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
    expect(recoveryTimelinePresentationSource).toContain(
      'export function getRecoveryTimelineColumnButtonClass',
    );
    expect(recoverySummaryPresentationSource).toContain(
      'export const RECOVERY_SUMMARY_TIME_RANGES',
    );
    expect(recoverySummaryPresentationSource).toContain(
      'export const RECOVERY_FRESHNESS_BUCKETS',
    );
    expect(dashboardHelpersSource).toContain("from '@/utils/dashboardMetricPresentation'");
    expect(dashboardHelpersSource).not.toContain('export function statusBadgeClass');
    expect(dashboardHelpersSource).not.toContain('export function priorityBadgeClass');
    expect(dashboardMetricPresentationSource).toContain(
      'export function getDashboardStatusBadgeClass',
    );
    expect(dashboardMetricPresentationSource).toContain(
      'export function getDashboardPriorityBadgeClass',
    );
    expect(trendChartsSource).toContain("segmentedButtonClass(active(), false, 'accent')");
    expect(trendChartsSource).not.toContain(
      "'px-2 py-0.5 rounded bg-blue-600 text-white text-[11px] font-medium'",
    );
    expect(compositionPanelSource).toContain('getDashboardCompositionIcon');
    expect(compositionPanelSource).not.toContain('const TYPE_ICONS: Record<string, any> =');
    expect(dashboardCompositionPresentationSource).toContain(
      'export const getDashboardCompositionIcon',
    );
    expect(problemResourcesTableSource).toContain('getProblemResourceStatusVariant');
    expect(problemResourcesTableSource).not.toContain(
      'function statusVariant(pr: ProblemResource)',
    );
    expect(problemResourcePresentationSource).toContain(
      'export function getProblemResourceStatusVariant',
    );
    expect(kpiStripSource).toContain('getDashboardAlertTone');
    expect(kpiStripSource).not.toContain('const alertsTone =');
    expect(recentAlertsPanelSource).toContain('getAlertSeverityTextClass');
    expect(dashboardAlertPresentationSource).toContain('export function getDashboardAlertTone');
    expect(diskListSource).toContain('getTemperatureTextClass');
    expect(diskListSource).not.toContain('const getTemperatureTone =');
    expect(diskListSource).toContain('getPhysicalDiskHealthStatus');
    expect(diskListSource).toContain('getPhysicalDiskRoleLabel');
    expect(diskListSource).toContain('getPhysicalDiskParentLabel');
    expect(diskListSource).toContain('getPhysicalDiskPlatformLabel');
    expect(diskListSource).toContain('hasPhysicalDiskSmartWarning');
    expect(diskListSource).not.toContain('const titleize =');
    expect(diskListSource).not.toContain('const platformLabel =');
    expect(diskListSource).not.toContain('function hasSmartWarning(');
    expect(diskListSource).not.toContain('const getDiskHealthStatus =');
    expect(diskListSource).not.toContain('const getDiskRoleLabel =');
    expect(diskListSource).not.toContain('const getDiskParentLabel =');
    expect(storagePoolDetailSource).toContain('getLinkedDiskHealthDotClass');
    expect(storagePoolDetailSource).toContain('getLinkedDiskTemperatureTextClass');
    expect(storagePoolDetailSource).toContain('getZfsScanTextClass');
    expect(storagePoolDetailSource).toContain('getZfsErrorTextClass');
    expect(storagePoolDetailSource).toContain('getZfsErrorSummary');
    expect(storagePoolDetailSource).not.toContain("'bg-yellow-500'");
    expect(storagePoolDetailSource).not.toContain("'bg-green-500'");
    expect(storagePoolDetailSource).not.toContain("'text-red-500'");
    expect(storagePoolDetailSource).not.toContain("'text-yellow-500'");
    expect(storagePoolDetailSource).not.toContain("'text-yellow-600 dark:text-yellow-400 italic'");
    expect(storagePoolDetailSource).not.toContain("'text-red-600 dark:text-red-400 font-medium'");
    expect(diskDetailSource).toContain('getDiskAttributeValueTextClass');
    expect(diskDetailSource).not.toContain('function attrColor(');
    expect(storagePoolRowSource).toContain('getStoragePoolProtectionTextClass');
    expect(storagePoolRowSource).toContain('getStoragePoolIssueTextClass');
    expect(storagePoolRowSource).toContain('getCompactStoragePoolProtectionLabel');
    expect(storagePoolRowSource).toContain('getCompactStoragePoolImpactLabel');
    expect(storagePoolRowSource).toContain('getCompactStoragePoolIssueLabel');
    expect(storagePoolRowSource).toContain('getCompactStoragePoolIssueSummary');
    expect(storagePoolRowSource).toContain('getCompactStoragePoolProtectionTitle');
    expect(storagePoolRowSource).not.toContain('const protectionTextClass =');
    expect(storagePoolRowSource).not.toContain('const issueTextClass =');
    expect(storagePoolRowSource).not.toContain('const compactProtection = createMemo(() => {');
    expect(storagePoolRowSource).not.toContain('const compactImpact = createMemo(() => {');
    expect(storagePoolRowSource).not.toContain('const compactIssue = createMemo(() => {');
    expect(storagePoolRowSource).not.toContain('const compactIssueSummary = createMemo(() => {');
    expect(storagePageSource).toContain("from './storagePageState'");
    expect(storagePageSource).toContain('useStorageExpansionState');
    expect(storagePageSource).toContain('useStorageFilterState');
    expect(storagePageSource).toContain('useStoragePageFilters');
    expect(storagePageSource).toContain('useStoragePageData');
    expect(storagePageSource).toContain('countVisiblePhysicalDisksForNode');
    expect(storagePageSource).toContain('isStorageRecordCeph');
    expect(storagePageSource).toContain('StorageCephSection');
    expect(storagePageSource).toContain('StoragePageControls');
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
    expect(useStorageCephModelSource).not.toContain('export const isCephRecord =');
    expect(useStorageCephModelSource).not.toContain('export const getCephClusterKeyFromRecord =');
    expect(cephSummaryPresentationSource).toContain(
      'export const deriveCephClustersFromStorageRecords',
    );
    expect(cephSummaryPresentationSource).toContain('export const summarizeCephClusters');
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
    expect(storageGroupRowSource).toContain('getStorageGroupHealthCountPresentation');
    expect(storageGroupRowSource).toContain('getStorageGroupPoolCountLabel');
    expect(storageGroupRowSource).toContain('getStorageGroupUsagePercentLabel');
    expect(storageGroupPresentationSource).toContain(
      'export const getStorageGroupHealthCountPresentation',
    );
    expect(storageGroupPresentationSource).toContain(
      'export const getStorageGroupPoolCountLabel',
    );
    expect(storageGroupPresentationSource).toContain(
      'export const getStorageGroupUsagePercentLabel',
    );
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
    expect(storagePageFiltersSource).toContain('export const useStoragePageFilters');
    expect(storagePageFiltersSource).toContain('useStorageRouteState');
    expect(storagePageFiltersSource).toContain('buildStorageRouteFields');
    expect(storagePageStateSource).toContain('export const buildStorageRouteFields');
    expect(storageFilterStateSource).toContain('export const useStorageFilterState');
    expect(storageFilterStateSource).toContain('getStorageFilterGroupBy');
    expect(storageFilterStateSource).toContain('getStorageStatusFilterValue');
    expect(storageFilterStateSource).toContain('toStorageHealthFilterValue');
    expect(storagePageSource).toContain('useStoragePageStatus');
    expect(storagePageSource).toContain('StorageContentCard');
    expect(storagePageSource).toContain('StoragePageBanners');
    expect(storagePageSource).toContain('useStorageFilterState');
    expect(storagePageSource).toContain('useStorageResourceHighlight');
    expect(storagePageSource).toContain('useStorageExpansionState');
    expect(storageControlsSource).toContain('export const StorageControls');
    expect(storageControlsSource).toContain('StorageFilter');
    expect(storageControlsSource).toContain('Subtabs');
    expect(storageControlsSource).toContain('STORAGE_VIEW_OPTIONS');
    expect(storageControlsSource).toContain('DEFAULT_STORAGE_SORT_OPTIONS');
    expect(storagePageControlsSource).toContain('export const StoragePageControls');
    expect(storagePageControlsSource).toContain('StorageControls');
    expect(storagePageControlsSource).toContain('normalizeStorageSortKey');
    expect(storageCephSectionSource).toContain('export const StorageCephSection');
    expect(storageCephSectionSource).toContain('shouldShowCephSummaryCard');
    expect(storageCephSectionSource).toContain('StorageCephSummaryCard');
    expect(storagePageBannerSource).toContain('getStoragePageBannerMessage');
    expect(storagePageBannerSource).toContain('STORAGE_BANNER_ACTION_BUTTON_CLASS');
    expect(storagePageBannersSource).toContain('export const StoragePageBanners');
    expect(storagePageBannersSource).toContain('StoragePageBanner');
    expect(storageCephSummaryCardSource).toContain('getCephSummaryHeading');
    expect(storageCephSummaryCardSource).toContain('getCephHealthStyles');
    expect(storageContentCardSource).toContain('export const StorageContentCard');
    expect(storageContentCardSource).toContain('getStorageTableHeading');
    expect(storageContentCardSource).toContain('DiskList');
    expect(storageContentCardSource).toContain('StoragePoolsTable');
    expect(storagePoolsTableSource).toContain('export const StoragePoolsTable');
    expect(storagePoolsTableSource).toContain('STORAGE_POOL_TABLE_COLUMNS');
    expect(storagePoolsTableSource).toContain('getStorageLoadingMessage');
    expect(storagePoolsTableSource).toContain('getStorageRowAlertPresentation');
    expect(storagePoolsTableSource).toContain('StoragePoolRow');
    expect(storagePoolsTableSource).toContain('StorageGroupRow');
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
    expect(storagePageSource).not.toContain("buildPath: buildStoragePath");
    expect(storagePageSource).not.toContain('const storageFilterGroupBy =');
    expect(storagePageSource).not.toContain('const storageFilterStatus =');
    expect(storagePageSource).not.toContain('const setStorageFilterStatus =');
    expect(storagePageSource).not.toContain('const records = createMemo(() => buildStorageRecords');
    expect(storagePageSource).not.toContain('fields: {');
    expect(storagePageSource).not.toContain('rounded border border-amber-300 bg-amber-100 px-2 py-1');
    expect(storagePageSource).not.toContain('flex flex-wrap items-center justify-between gap-3');
    expect(storagePageSource).not.toContain('<Table class="w-full text-xs">');
    expect(storagePageSource).not.toContain('<StoragePageBanner kind=');
    expect(storagePageSource).not.toContain('<StorageCephSummaryCard summary=');
    expect(storageRowAlertPresentationSource).toContain(
      'export const getStorageRowAlertPresentation',
    );
    expect(diskPresentationSource).toContain('export function getPhysicalDiskHealthStatus');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskRoleLabel');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskParentLabel');
    expect(diskPresentationSource).toContain('export function getPhysicalDiskPlatformLabel');
    expect(diskPresentationSource).toContain('export function hasPhysicalDiskSmartWarning');
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
    expect(swarmServicesDrawerSource).not.toContain('const statusTone =');
    expect(k8sDeploymentsDrawerSource).toContain('getSimpleStatusIndicator');
    expect(k8sDeploymentsDrawerSource).toContain('<StatusDot');
    expect(k8sDeploymentsDrawerSource).not.toContain('const statusTone =');
    expect(k8sNamespacesDrawerSource).toContain('getNamespaceCountsIndicator');
    expect(k8sNamespacesDrawerSource).toContain('<StatusDot');
    expect(k8sNamespacesDrawerSource).not.toContain('const statusTone =');
    expect(k8sStatusPresentationSource).toContain('export function getNamespaceCountsIndicator');
    expect(raidCardSource).toContain('getRaidStateVariant');
    expect(raidCardSource).toContain('getRaidStateTextClass');
    expect(raidCardSource).toContain('getRaidDeviceBadgeClass');
    expect(raidCardSource).not.toContain('const raidStateVariant =');
    expect(raidCardSource).not.toContain('const deviceToneClass =');
    expect(raidPresentationSource).toContain('export function getRaidStateVariant');
    expect(raidPresentationSource).toContain('export function getRaidDeviceBadgeClass');
    expect(proLicensePanelSource).toContain('getLicenseSubscriptionStatusPresentation');
    expect(proLicensePanelSource).not.toContain('const statusLabel =');
    expect(proLicensePanelSource).not.toContain('const statusTone =');
    expect(licensePresentationSource).toContain(
      'export const getLicenseSubscriptionStatusPresentation',
    );
    expect(securityPostureSummarySource).toContain('getSecurityScorePresentation');
    expect(securityPostureSummarySource).not.toContain('const scoreTone =');
    expect(securityWarningSource).toContain('getSecurityWarningPresentation');
    expect(securityWarningSource).toContain('getSecurityScorePresentation');
    expect(securityWarningSource).toContain('warningPresentation().background');
    expect(securityWarningSource).toContain('warningPresentation().border');
    expect(securityWarningSource).not.toContain('bg-yellow-50 dark:bg-yellow-900');
    expect(findingsPanelSource).toContain('getFindingStatusBadgeClasses');
    expect(findingsPanelSource).toContain('getFindingStatusLabel');
    expect(findingsPanelSource).toContain("segmentedButtonClass(filter() === 'active')");
    expect(findingsPanelSource).toContain(
      "segmentedButtonClass(filter() === 'attention', false, 'warning')",
    );
    expect(findingsPanelSource).not.toContain("finding.status === 'resolved' ? 'Resolved'");
    expect(findingsPanelSource).not.toContain(
      "filter() === 'attention'\n                    ? 'bg-amber-50 dark:bg-amber-900 text-amber-700 dark:text-amber-300 border-amber-300 dark:border-amber-700 shadow-sm'",
    );
    expect(aiFindingPresentationSource).toContain(
      'export const getFindingStatusBadgeClasses',
    );
    expect(aiFindingPresentationSource).toContain('export const getFindingStatusLabel');
    expect(securityWarningSource).not.toContain('bg-red-50 dark:bg-red-900');
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityScorePresentation',
    );
    expect(securityScorePresentationSource).toContain(
      'export function getSecurityWarningPresentation',
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
    expect(discoveryTabSource).not.toContain('const getURLSuggestionSourceLabel =');
    expect(discoveryPresentationSource).toContain(
      'export function getDiscoveryURLSuggestionSourceLabel',
    );
    expect(discoveryTabSource).not.toContain('bg-blue-100 text-blue-700');
    expect(discoveryTabSource).not.toContain('bg-green-100 text-green-700');
    expect(discoveryPresentationSource).toContain(
      'export function getDiscoveryAnalysisProviderBadgeClass',
    );
    expect(discoveryPresentationSource).toContain('export function getDiscoveryCategoryBadgeClass');
    expect(mailGatewaySource).toContain('getPMGThreatPresentation');
    expect(mailGatewaySource).not.toContain('const barColor =');
    expect(mailGatewaySource).not.toContain('const textColor =');
    expect(mailGatewaySource).toContain('getPMGQueueTextClass');
    expect(mailGatewaySource).toContain('getPMGOldestAgeTextClass');
    expect(mailGatewaySource).not.toContain('const queueSeverity =');
    expect(mailGatewaySource).toContain('ServiceHealthBadge');
    expect(mailGatewaySource).not.toContain('const StatusBadge: Component');
    expect(pmgInstancePanelSource).toContain('getPMGThreatPresentation');
    expect(pmgInstancePanelSource).not.toContain('const barColor =');
    expect(pmgInstancePanelSource).not.toContain('const textColor =');
    expect(pmgInstancePanelSource).toContain('getPMGQueueTextClass');
    expect(pmgInstancePanelSource).toContain('getPMGOldestAgeTextClass');
    expect(pmgInstancePanelSource).not.toContain('const queueSeverity =');
    expect(pmgInstancePanelSource).toContain('ServiceHealthBadge');
    expect(pmgInstancePanelSource).not.toContain('const StatusBadge: Component');
    expect(pmgThreatPresentationSource).toContain('export function getPMGThreatPresentation');
    expect(pmgQueuePresentationSource).toContain('export function getPMGQueueTextClass');
    expect(pmgServiceHealthBadgeSource).toContain('getServiceHealthPresentation');
    expect(cephPageSource).toContain('getCephServiceStatusPresentation');
    expect(cephPageSource).not.toContain('const getServiceStatus =');
    expect(storageDomainSource).toContain('export const getCephServiceStatusPresentation');
    expect(deployStatusBadgeSource).toContain('getDeployStatusPresentation');
    expect(deployStatusBadgeSource).not.toContain('const statusConfig: Record<DeployTargetStatus');
    expect(deployStatusPresentationSource).toContain('export const getDeployStatusPresentation');
    expect(alertsPageSource).toContain('getAlertIncidentStatusPresentation');
    expect(alertsPageSource).toContain('getAlertIncidentLevelBadgeClass');
    expect(alertsPageSource).toContain('getAlertHistoryStatusPresentation');
    expect(alertsPageSource).toContain('getAlertHistorySourcePresentation');
    expect(alertsPageSource).toContain('getAlertHistoryResourceTypeBadgeClass');
    expect(alertsPageSource).toContain('getAlertActivationPresentation');
    expect(alertsPageSource).toContain('getAlertFrequencySelectionPresentation');
    expect(alertsPageSource).toContain('getAlertFrequencyClearFilterButtonClass');
    expect(alertsPageSource).toContain('getAlertSeverityDotClass');
    expect(alertsPageSource).toContain('getAlertsSidebarTabClass');
    expect(alertsPageSource).toContain('getAlertsMobileTabClass');
    expect(alertsPageSource).toContain('getAlertsTabTitle');
    expect(alertsPageSource).toContain('getAlertGroupingCardClass');
    expect(alertsPageSource).toContain('getAlertGroupingCheckboxClass');
    expect(alertsPageSource).toContain('getAlertQuietDayButtonClass');
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
    expect(alertTabsPresentationSource).toContain('export function getAlertsSidebarTabClass');
    expect(alertTabsPresentationSource).toContain('export function getAlertsMobileTabClass');
    expect(alertTabsPresentationSource).toContain('export function getAlertsTabTitle');
    expect(alertGroupingPresentationSource).toContain(
      'export function getAlertGroupingCardClass',
    );
    expect(alertGroupingPresentationSource).toContain(
      'export function getAlertGroupingCheckboxClass',
    );
    expect(alertSchedulePresentationSource).toContain(
      'export function getAlertQuietDayButtonClass',
    );
    expect(configuredNodeTablesSource).toContain('getConfiguredNodeCapabilityBadges');
    expect(configuredNodeTablesSource).not.toContain("'monitorVMs' in node");
    expect(configuredNodeTablesSource).not.toContain("'monitorDatastores' in node");
    expect(configuredNodeCapabilityPresentationSource).toContain(
      'export function getConfiguredNodeCapabilityBadges',
    );
    expect(auditLogPanelSource).toContain('getAuditEventTypeBadgeClass');
    expect(auditLogPanelSource).toContain('getAuditVerificationBadgePresentation');
    expect(auditLogPanelSource).not.toContain('const getEventTypeBadge =');
    expect(auditLogPanelSource).not.toContain('const getVerificationBadge =');
    expect(auditLogPresentationSource).toContain('export function getAuditEventTypeBadgeClass');
    expect(auditLogPresentationSource).toContain(
      'export function getAuditVerificationBadgePresentation',
    );
    expect(diagnosticsPanelSource).toContain('getStatusIndicatorBadgeToneClasses(');
    expect(diagnosticsPanelSource).not.toContain(
      "'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'",
    );
    expect(diagnosticsPanelSource).not.toContain(
      "'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'",
    );
    expect(aiSettingsSource).toContain('getAIProviderTestResultTextClass');
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
    expect(aiCostDashboardSource).toContain('segmentedButtonClass');
    expect(aiCostDashboardSource).not.toContain(
      "'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700'",
    );
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
});
