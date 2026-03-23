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
import commandPaletteModalSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import commandPaletteModelSource from '@/components/shared/commandPaletteModel.ts?raw';
import activeUseTrialNudgeSource from '@/components/shared/ActiveUseTrialNudge.tsx?raw';
import activeUseTrialNudgeModelSource from '@/components/shared/activeUseTrialNudgeModel.ts?raw';
import columnPickerSource from '@/components/shared/ColumnPicker.tsx?raw';
import columnPickerModelSource from '@/components/shared/columnPickerModel.ts?raw';
import tagInputSource from '@/components/shared/TagInput.tsx?raw';
import tagInputModelSource from '@/components/shared/tagInputModel.ts?raw';
import collapsibleSearchInputSource from '@/components/shared/CollapsibleSearchInput.tsx?raw';
import collapsibleSearchInputModelSource from '@/components/shared/collapsibleSearchInputModel.ts?raw';
import containerUpdateBadgeSource from '@/components/shared/ContainerUpdateBadge.tsx?raw';
import containerUpdateBadgeModelSource from '@/components/shared/containerUpdateBadgeModel.ts?raw';
import densityMapSource from '@/components/shared/DensityMap.tsx?raw';
import densityMapModelSource from '@/components/shared/densityMapModel.ts?raw';
import dialogSource from '@/components/shared/Dialog.tsx?raw';
import dialogModelSource from '@/components/shared/dialogModel.ts?raw';
import filterButtonGroupSource from '@/components/shared/FilterButtonGroup.tsx?raw';
import filterButtonGroupModelSource from '@/components/shared/filterButtonGroupModel.ts?raw';
import helpIconSource from '@/components/shared/HelpIcon.tsx?raw';
import helpIconModelSource from '@/components/shared/helpIconModel.ts?raw';
import historyChartHeaderSource from '@/components/shared/HistoryChartHeader.tsx?raw';
import historyChartOverlaySource from '@/components/shared/HistoryChartOverlay.tsx?raw';
import historyChartSource from '@/components/shared/HistoryChart.tsx?raw';
import historyChartModelSource from '@/components/shared/historyChartModel.ts?raw';
import historyChartTooltipSource from '@/components/shared/HistoryChartTooltip.tsx?raw';
import mobileNavBarSource from '@/components/shared/MobileNavBar.tsx?raw';
import mobileNavBarModelSource from '@/components/shared/mobileNavBarModel.ts?raw';
import infrastructureSelectorSource from '@/components/shared/InfrastructureSelector.tsx?raw';
import pulseDataGridSource from '@/components/shared/PulseDataGrid.tsx?raw';
import pulseDataGridModelSource from '@/components/shared/pulseDataGridModel.ts?raw';
import whatsNewModalSource from '@/components/shared/WhatsNewModal.tsx?raw';
import whatsNewModalModelSource from '@/components/shared/whatsNewModalModel.ts?raw';
import searchFieldSource from '@/components/shared/SearchField.tsx?raw';
import searchFieldModelSource from '@/components/shared/searchFieldModel.ts?raw';
import searchInputSource from '@/components/shared/SearchInput.tsx?raw';
import searchInputEnhancementsSource from '@/components/shared/SearchInputEnhancements.tsx?raw';
import searchInputEnhancementsModelSource from '@/components/shared/searchInputEnhancementsModel.ts?raw';
import searchInputModelSource from '@/components/shared/searchInputModel.ts?raw';
import scrollToTopButtonSource from '@/components/shared/ScrollToTopButton.tsx?raw';
import scrollToTopButtonModelSource from '@/components/shared/scrollToTopButtonModel.ts?raw';
import statusBadgeSource from '@/components/shared/StatusBadge.tsx?raw';
import statusBadgeModelSource from '@/components/shared/statusBadgeModel.ts?raw';
import toggleSource from '@/components/shared/Toggle.tsx?raw';
import toggleModelSource from '@/components/shared/toggleModel.ts?raw';
import searchTipsPopoverSource from '@/components/shared/SearchTipsPopover.tsx?raw';
import searchTipsPopoverModelSource from '@/components/shared/searchTipsPopoverModel.ts?raw';
import tooltipSource from '@/components/shared/Tooltip.tsx?raw';
import tooltipModelSource from '@/components/shared/tooltipModel.ts?raw';
import trialBannerSource from '@/components/shared/TrialBanner.tsx?raw';
import trialBannerModelSource from '@/components/shared/trialBannerModel.ts?raw';
import monitoredSystemLimitWarningBannerSource from '@/components/shared/MonitoredSystemLimitWarningBanner.tsx?raw';
import monitoredSystemLimitWarningBannerModelSource from '@/components/shared/monitoredSystemLimitWarningBannerModel.ts?raw';
import infrastructureSummaryTableSource from '@/components/shared/InfrastructureSummaryTable.tsx?raw';
import infrastructureSummaryTableRowSource from '@/components/shared/InfrastructureSummaryTableRow.tsx?raw';
import interactiveSparklineSource from '@/components/shared/InteractiveSparkline.tsx?raw';
import interactiveSparklineModelSource from '@/components/shared/interactiveSparklineModel.ts?raw';
import infrastructureSelectorModelSource from '@/components/shared/infrastructureSelectorModel.ts?raw';
import selectionCardGroupSource from '@/components/shared/SelectionCardGroup.tsx?raw';
import selectionCardGroupModelSource from '@/components/shared/selectionCardGroupModel.ts?raw';
import sharedInfrastructureSummaryTableModelSource from '@/components/shared/infrastructureSummaryTableModel.ts?raw';
import commandPaletteStateSource from '@/components/shared/useCommandPaletteState.ts?raw';
import activeUseTrialNudgeStateSource from '@/components/shared/useActiveUseTrialNudgeState.ts?raw';
import columnPickerStateSource from '@/components/shared/useColumnPickerState.ts?raw';
import tagInputStateSource from '@/components/shared/useTagInputState.ts?raw';
import collapsibleSearchInputStateSource from '@/components/shared/useCollapsibleSearchInputState.ts?raw';
import containerUpdateButtonStateSource from '@/components/shared/useContainerUpdateButtonState.ts?raw';
import dialogStateSource from '@/components/shared/useDialogState.ts?raw';
import filterButtonGroupStateSource from '@/components/shared/useFilterButtonGroupState.ts?raw';
import helpIconStateSource from '@/components/shared/useHelpIconState.ts?raw';
import historyChartStateSource from '@/components/shared/useHistoryChartState.ts?raw';
import mobileNavBarStateSource from '@/components/shared/useMobileNavBarState.ts?raw';
import infrastructureSelectorStateSource from '@/components/shared/useInfrastructureSelectorState.ts?raw';
import pulseDataGridStateSource from '@/components/shared/usePulseDataGridState.ts?raw';
import whatsNewModalStateSource from '@/components/shared/useWhatsNewModalState.ts?raw';
import searchFieldStateSource from '@/components/shared/useSearchFieldState.ts?raw';
import searchInputStateSource from '@/components/shared/useSearchInputState.ts?raw';
import searchInputEnhancementsStateSource from '@/components/shared/useSearchInputEnhancements.ts?raw';
import scrollToTopButtonStateSource from '@/components/shared/useScrollToTopButtonState.ts?raw';
import statusBadgeStateSource from '@/components/shared/useStatusBadgeState.ts?raw';
import toggleStateSource from '@/components/shared/useToggleState.ts?raw';
import searchTipsPopoverStateSource from '@/components/shared/useSearchTipsPopoverState.ts?raw';
import tooltipStateSource from '@/components/shared/useTooltipState.ts?raw';
import trialBannerStateSource from '@/components/shared/useTrialBannerState.ts?raw';
import interactiveSparklineStateSource from '@/components/shared/useInteractiveSparklineState.ts?raw';
import monitoredSystemLimitWarningBannerStateSource from '@/components/shared/useMonitoredSystemLimitWarningBannerState.ts?raw';
import infrastructureSummaryTableStateSource from '@/components/shared/useInfrastructureSummaryTableState.ts?raw';
import selectionCardGroupStateSource from '@/components/shared/useSelectionCardGroupState.ts?raw';
import resourceBadgePresentationSource from '@/utils/resourceBadgePresentation.ts?raw';
import workloadTypeBadgesSource from '@/components/shared/workloadTypeBadges.ts?raw';
import tagBadgesSource from '@/components/shared/TagBadges.tsx?raw';
import emptyStateSource from '@/components/shared/EmptyState.tsx?raw';
import densityMapStateSource from '@/components/shared/useDensityMapState.ts?raw';
import webInterfaceUrlFieldSource from '@/components/shared/WebInterfaceUrlField.tsx?raw';
import webInterfaceUrlFieldModelSource from '@/components/shared/webInterfaceUrlFieldModel.ts?raw';
import webInterfaceUrlFieldStateSource from '@/components/shared/useWebInterfaceUrlFieldState.ts?raw';
import workloadTypePresentationSource from '@/utils/workloadTypePresentation.ts?raw';
import sourcePlatformsSource from '@/utils/sourcePlatforms.ts?raw';
import storageSourcesSource from '@/utils/storageSources.ts?raw';
import rbacPermissionsSource from '@/utils/rbacPermissions.ts?raw';
import systemSettingsPresentationSource from '@/utils/systemSettingsPresentation.ts?raw';
import updatesSettingsPanelSource from '@/components/Settings/UpdatesSettingsPanel.tsx?raw';
import copyCommandBlockSource from '@/components/Settings/CopyCommandBlock.tsx?raw';
import updateInstallGuideSource from '@/components/Settings/UpdateInstallGuide.tsx?raw';
import reportingPanelModelSource from '@/components/Settings/reportingPanelModel.ts?raw';
import reportingPanelStateSource from '@/components/Settings/useReportingPanelState.ts?raw';
import updatesSettingsModelSource from '@/components/Settings/updatesSettingsModel.ts?raw';
import diagnosticsModelSource from '@/components/Settings/diagnosticsModel.ts?raw';
import diagnosticsResultsPanelSource from '@/components/Settings/DiagnosticsResultsPanel.tsx?raw';
import diagnosticsStateSource from '@/components/Settings/useDiagnosticsPanelState.ts?raw';
import settingsShellSource from '@/components/Settings/Settings.tsx?raw';
import settingsDialogsSource from '@/components/Settings/SettingsDialogs.tsx?raw';
import settingsPanelRegistrySource from '@/components/Settings/useSettingsPanelRegistry.tsx?raw';
import settingsPanelRegistryContextSource from '@/components/Settings/settingsPanelRegistryContext.tsx?raw';
import settingsPanelRegistryLoadersSource from '@/components/Settings/settingsPanelRegistryLoaders.ts?raw';
import settingsNavigationModelSource from '@/components/Settings/settingsNavigationModel.ts?raw';
import settingsRoutingSource from '@/components/Settings/settingsRouting.ts?raw';
import settingsTypesSource from '@/components/Settings/settingsTypes.ts?raw';
import settingsNavigationHookSource from '@/components/Settings/useSettingsNavigation.ts?raw';
import settingsShellStateSource from '@/components/Settings/useSettingsShellState.ts?raw';
import settingsNavCatalogSource from '@/components/Settings/settingsNavCatalog.ts?raw';
import settingsNavVisibilitySource from '@/components/Settings/settingsNavVisibility.ts?raw';
import settingsTabSaveBehaviorSource from '@/components/Settings/settingsTabSaveBehavior.ts?raw';
import settingsSystemPanelsSource from '@/components/Settings/useSettingsSystemPanels.tsx?raw';
import settingsInfrastructurePanelPropsSource from '@/components/Settings/useSettingsInfrastructurePanelProps.ts?raw';
import discoverySettingsStateSource from '@/components/Settings/useDiscoverySettingsState.ts?raw';
import networkBoundarySettingsSectionSource from '@/components/Settings/NetworkBoundarySettingsSection.tsx?raw';
import networkDiscoverySectionSource from '@/components/Settings/NetworkDiscoverySection.tsx?raw';
import networkSettingsPanelSource from '@/components/Settings/NetworkSettingsPanel.tsx?raw';
import networkSettingsModelSource from '@/components/Settings/networkSettingsModel.ts?raw';
import reportingPresentationSource from '@/utils/reportingPresentation.ts?raw';
import updatesPresentationSource from '@/utils/updatesPresentation.ts?raw';
import environmentLockBadgeSource from '@/components/shared/EnvironmentLockBadge.tsx?raw';
import environmentLockPresentationSource from '@/utils/environmentLockPresentation.ts?raw';
import dockerRuntimeSettingsCardSource from '@/components/Settings/DockerRuntimeSettingsCard.tsx?raw';
import infrastructurePageShellSource from '@/pages/Infrastructure.tsx?raw';
import operationsPageRouteSource from '@/pages/Operations.tsx?raw';
import discoveryTargetSource from '@/utils/discoveryTarget.ts?raw';
import infrastructureEmptyStatePresentationSource from '@/utils/infrastructureEmptyStatePresentation.ts?raw';
import recoverySummarySource from '@/components/Recovery/RecoverySummary.tsx?raw';
import recoveryComponentSource from '@/components/Recovery/Recovery.tsx?raw';
import recoveryActivitySectionSource from '@/components/Recovery/RecoveryActivitySection.tsx?raw';
import recoveryHistorySectionSource from '@/components/Recovery/RecoveryHistorySection.tsx?raw';
import recoveryHistoryTableSource from '@/components/Recovery/RecoveryHistoryTable.tsx?raw';
import recoveryProtectedInventorySectionSource from '@/components/Recovery/RecoveryProtectedInventorySection.tsx?raw';
import recoveryHistorySectionStateSource from '@/components/Recovery/useRecoveryHistorySectionState.ts?raw';
import recoverySurfaceStateSource from '@/features/recovery/useRecoverySurfaceState.ts?raw';
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
import dashboardStateCardsSource from '@/components/Dashboard/DashboardStateCards.tsx?raw';
import dashboardStatsStripSource from '@/components/Dashboard/DashboardStatsStrip.tsx?raw';
import dashboardFilterSource from '@/components/Dashboard/DashboardFilter.tsx?raw';
import dashboardWorkloadTableSource from '@/components/Dashboard/DashboardWorkloadTable.tsx?raw';
import workloadPanelSource from '@/components/Dashboard/WorkloadPanel.tsx?raw';
import workloadTableHeaderSource from '@/components/Dashboard/WorkloadTableHeader.tsx?raw';
import dashboardFilterModelSource from '@/components/Dashboard/dashboardFilterModel.ts?raw';
import dashboardControlsStateSource from '@/components/Dashboard/useDashboardControlsState.ts?raw';
import dashboardGuestMetadataStateSource from '@/components/Dashboard/useDashboardGuestMetadataState.ts?raw';
import dashboardSelectionModelSource from '@/components/Dashboard/dashboardSelectionModel.ts?raw';
import dashboardSelectionStateSource from '@/components/Dashboard/useDashboardSelectionState.ts?raw';
import dashboardWorkloadDerivedStateSource from '@/components/Dashboard/useDashboardWorkloadDerivedState.ts?raw';
import dashboardWorkloadViewportSyncSource from '@/components/Dashboard/useDashboardWorkloadViewportSync.ts?raw';
import dashboardWorkloadFilterOptionsSource from '@/components/Dashboard/useDashboardWorkloadFilterOptions.ts?raw';
import dashboardWorkloadFilterConfigModelSource from '@/components/Dashboard/dashboardWorkloadFilterConfigModel.ts?raw';
import dashboardWorkloadRouteModelSource from '@/components/Dashboard/dashboardWorkloadRouteModel.ts?raw';
import dashboardWorkloadRouteStateModelSource from '@/components/Dashboard/dashboardWorkloadRouteStateModel.ts?raw';
import dashboardWorkloadUrlSyncModelSource from '@/components/Dashboard/dashboardWorkloadUrlSyncModel.ts?raw';
import dashboardWorkloadRouteStateSource from '@/components/Dashboard/useDashboardWorkloadRouteState.ts?raw';
import dashboardWorkloadUrlSyncSource from '@/components/Dashboard/useDashboardWorkloadUrlSync.ts?raw';
import dashboardStateSource from '@/components/Dashboard/useDashboardState.ts?raw';
import dashboardFilterStateSource from '@/components/Dashboard/useDashboardFilterState.ts?raw';
import workloadTopologySource from '@/components/Dashboard/workloadTopology.ts?raw';
import groupedTableWindowingSource from '@/components/Dashboard/useGroupedTableWindowing.ts?raw';
import thresholdSliderModelSource from '@/components/Dashboard/thresholdSliderModel.ts?raw';
import thresholdSliderStateSource from '@/components/Dashboard/useThresholdSliderState.ts?raw';
import enhancedCpuBarSource from '@/components/Dashboard/EnhancedCPUBar.tsx?raw';
import enhancedCpuBarModelSource from '@/components/Dashboard/enhancedCpuBarModel.ts?raw';
import enhancedCpuBarStateSource from '@/components/Dashboard/useEnhancedCPUBarState.ts?raw';
import stackedDiskBarSource from '@/components/Dashboard/StackedDiskBar.tsx?raw';
import stackedDiskBarModelSource from '@/components/Dashboard/stackedDiskBarModel.ts?raw';
import stackedDiskBarStateSource from '@/components/Dashboard/useStackedDiskBarState.ts?raw';
import stackedMemoryBarSource from '@/components/Dashboard/StackedMemoryBar.tsx?raw';
import stackedMemoryBarModelSource from '@/components/Dashboard/stackedMemoryBarModel.ts?raw';
import stackedMemoryBarStateSource from '@/components/Dashboard/useStackedMemoryBarState.ts?raw';
import workloadsSummarySource from '@/components/Workloads/WorkloadsSummary.tsx?raw';
import dashboardRouteSource from '@/pages/Dashboard.tsx?raw';
import dashboardMetricPresentationSource from '@/utils/dashboardMetricPresentation.ts?raw';
import dashboardTrendPresentationSource from '@/utils/dashboardTrendPresentation.ts?raw';
import trendChartsSource from '@/features/dashboardOverview/TrendCharts.tsx?raw';
import thresholdSliderSource from '@/components/Dashboard/ThresholdSlider.tsx?raw';
import problemResourcesTableSource from '@/features/dashboardOverview/ProblemResourcesTable.tsx?raw';
import problemResourcePresentationSource from '@/utils/problemResourcePresentation.ts?raw';
import kpiStripSource from '@/features/dashboardOverview/KPIStrip.tsx?raw';
import recentAlertsPanelSource from '@/components/Alerts/RecentAlertsPanel.tsx?raw';
import storagePanelSource from '@/components/Storage/DashboardStoragePanel.tsx?raw';
import recoveryStatusPanelSource from '@/components/Recovery/DashboardRecoveryStatusPanel.tsx?raw';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import guestRowCellsSource from '@/components/Dashboard/GuestRowCells.tsx?raw';
import guestRowModelSource from '@/components/Dashboard/guestRowModel.tsx?raw';
import guestRowStateSource from '@/components/Dashboard/useGuestRowState.ts?raw';
import dashboardDiskListSource from '@/components/Dashboard/DiskList.tsx?raw';
import dashboardDiskListModelSource from '@/components/Dashboard/diskListModel.ts?raw';
import dashboardDiskListStateSource from '@/components/Dashboard/useDiskListState.ts?raw';
import metricBarSource from '@/components/Dashboard/MetricBar.tsx?raw';
import metricBarModelSource from '@/components/Dashboard/metricBarModel.ts?raw';
import metricBarStateSource from '@/components/Dashboard/useMetricBarState.ts?raw';
import guestDrawerSource from '@/components/Dashboard/GuestDrawer.tsx?raw';
import guestDrawerOverviewSource from '@/components/Dashboard/GuestDrawerOverview.tsx?raw';
import guestDrawerModelSource from '@/components/Dashboard/guestDrawerModel.ts?raw';
import guestDrawerStateSource from '@/components/Dashboard/useGuestDrawerState.ts?raw';
import dashboardGuestPresentationSource from '@/utils/dashboardGuestPresentation.ts?raw';
import orgScopeSource from '@/utils/orgScope.ts?raw';
import workloadsSource from '@/utils/workloads.ts?raw';
import infrastructureSummaryCacheSource from '@/utils/infrastructureSummaryCache.ts?raw';
import dashboardStoragePresentationSource from '@/utils/dashboardStoragePresentation.ts?raw';
import dashboardRecoveryPresentationSource from '@/utils/dashboardRecoveryPresentation.ts?raw';
import dashboardKpiPresentationSource from '@/utils/dashboardKpiPresentation.ts?raw';
import dashboardEmptyStatePresentationSource from '@/utils/dashboardEmptyStatePresentation.ts?raw';
import throughputPresentationSource from '@/utils/throughputPresentation.ts?raw';
import resourceChangeSummarySource from '@/components/Infrastructure/ResourceChangeSummary.tsx?raw';
import resourceChangePresentationSource from '@/utils/resourceChangePresentation.ts?raw';
import resourceCorrelationPresentationSource from '@/utils/resourceCorrelationPresentation.ts?raw';
import confidencePresentationSource from '@/utils/confidencePresentation.ts?raw';
import approvalPresentationSource from '@/utils/approvalPresentation.ts?raw';
import textPresentationSource from '@/utils/textPresentation.ts?raw';
import messageItemSource from '@/components/AI/Chat/MessageItem.tsx?raw';
import toolExecutionBlockSource from '@/components/AI/Chat/ToolExecutionBlock.tsx?raw';
import aiChatSource from '@/components/AI/Chat/index.tsx?raw';
import useChatSource from '@/components/AI/Chat/hooks/useChat.ts?raw';
import patrolStatusBarSource from '@/components/patrol/PatrolStatusBar.tsx?raw';
import patrolFormatSource from '@/utils/patrolFormat.ts?raw';
import aiFindingPresentationSource from '@/utils/aiFindingPresentation.ts?raw';
import operationsPageSurfaceSource from '@/features/operations/OperationsPageSurface.tsx?raw';
import operationsPageModelSource from '@/features/operations/operationsPageModel.ts?raw';
import chatIdentifiersSource from '@/utils/chatIdentifiers.ts?raw';
import resourceIdentitySource from '@/utils/resourceIdentity.ts?raw';
import stringUtilsSource from '@/utils/stringUtils.ts?raw';
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
import storageSummarySource from '@/components/Storage/StorageSummary.tsx?raw';
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
import alertIncidentEventFiltersSource from '@/components/Alerts/IncidentEventFilters.tsx?raw';
import alertIncidentTimelinePanelSource from '@/components/Alerts/IncidentTimelinePanel.tsx?raw';
import alertIncidentTimelineEventCardSource from '@/components/Alerts/IncidentTimelineEventCard.tsx?raw';
import swarmServicesDrawerSource from '@/components/Docker/SwarmServicesDrawer.tsx?raw';
import swarmPresentationSource from '@/utils/swarmPresentation.ts?raw';
import k8sDeploymentsDrawerSource from '@/components/Kubernetes/K8sDeploymentsDrawer.tsx?raw';
import k8sDeploymentPresentationSource from '@/utils/k8sDeploymentPresentation.ts?raw';
import k8sNamespacesDrawerSource from '@/components/Kubernetes/K8sNamespacesDrawer.tsx?raw';
import k8sNamespacePresentationSource from '@/utils/k8sNamespacePresentation.ts?raw';
import k8sStatusPresentationSource from '@/utils/k8sStatusPresentation.ts?raw';
import raidCardSource from '@/components/shared/cards/RaidCard.tsx?raw';
import raidPresentationSource from '@/utils/raidPresentation.ts?raw';
import relayOnboardingCardSource from '@/components/Dashboard/RelayOnboardingCard.tsx?raw';
import proLicensePanelSource from '@/components/Settings/ProLicensePanel.tsx?raw';
import proLicensePlanSectionSource from '@/components/Settings/ProLicensePlanSection.tsx?raw';
import securityPostureSummarySource from '@/components/Settings/SecurityPostureSummary.tsx?raw';
import securityAuthPanelSource from '@/components/Settings/SecurityAuthPanel.tsx?raw';
import securityWarningSource from '@/components/SecurityWarning.tsx?raw';
import licensePresentationSource from '@/utils/licensePresentation.ts?raw';
import securityScorePresentationSource from '@/utils/securityScorePresentation.ts?raw';
import securityAuthPresentationSource from '@/utils/securityAuthPresentation.ts?raw';
import resourceDetailDrawerShellSource from '@/components/Infrastructure/ResourceDetailDrawer.tsx?raw';
import resourceDetailDrawerOverviewSource from '@/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx?raw';
import resourceDetailDrawerDebugSource from '@/components/Infrastructure/ResourceDetailDrawerDebugTab.tsx?raw';
import infrastructureSummarySource from '@/components/Infrastructure/InfrastructureSummary.tsx?raw';
import infrastructureSummaryStateSource from '@/components/Infrastructure/useInfrastructureSummaryState.ts?raw';
import infrastructureSummaryModelSource from '@/components/Infrastructure/infrastructureSummaryModel.ts?raw';
import resourceDetailDrawerDiscoveryModelSource from '@/components/Infrastructure/resourceDetailDiscoveryModel.ts?raw';
import resourceDetailMappersSource from '@/components/Infrastructure/resourceDetailMappers.ts?raw';
import resourceDetailDrawerHistoryStateSource from '@/components/Infrastructure/useResourceDetailDrawerHistoryState.ts?raw';
import resourceDetailDrawerDerivedStateSource from '@/components/Infrastructure/useResourceDetailDrawerDerivedState.ts?raw';
import resourceDetailDrawerOperationalModelSource from '@/components/Infrastructure/resourceDetailDrawerOperationalModel.ts?raw';
import resourceDetailDrawerServiceModelSource from '@/components/Infrastructure/resourceDetailDrawerServiceModel.ts?raw';
import resourceDetailDrawerDockerActionsStateSource from '@/components/Infrastructure/useResourceDetailDrawerDockerActionsState.ts?raw';
import resourceDetailDrawerStateSource from '@/components/Infrastructure/useResourceDetailDrawerState.ts?raw';
import unifiedResourceTableSource from '@/components/Infrastructure/UnifiedResourceTable.tsx?raw';
import unifiedResourceTableStateSource from '@/components/Infrastructure/useUnifiedResourceTableState.ts?raw';
import unifiedResourceTableViewportSyncSource from '@/components/Infrastructure/useUnifiedResourceTableViewportSync.ts?raw';
import unifiedResourceTableStateModelSource from '@/components/Infrastructure/unifiedResourceTableStateModel.ts?raw';
import unifiedResourceTableModelSource from '@/components/Infrastructure/unifiedResourceTableModel.ts?raw';
import useUnifiedResourcesSource from '@/hooks/useUnifiedResources.ts?raw';
import useWorkloadsSource from '@/hooks/useWorkloads.ts?raw';
import findingsPanelSource from '@/components/AI/FindingsPanel.tsx?raw';
import exploreStatusBlockSource from '@/components/AI/Chat/ExploreStatusBlock.tsx?raw';
import aiExplorePresentationSource from '@/utils/aiExplorePresentation.ts?raw';
import discoveryTabSource from '@/components/Discovery/DiscoveryTab.tsx?raw';
import discoveryPresentationSource from '@/utils/discoveryPresentation.ts?raw';
import mailGatewaySource from '@/components/PMG/MailGateway.tsx?raw';
import pmgInstancePanelSource from '@/components/PMG/PMGInstancePanel.tsx?raw';
import pmgPresentationSource from '@/utils/pmgPresentation.ts?raw';
import pmgThreatPresentationSource from '@/utils/pmgThreatPresentation.ts?raw';
import pmgQueuePresentationSource from '@/utils/pmgQueuePresentation.ts?raw';
import pmgServiceHealthBadgeSource from '@/components/PMG/ServiceHealthBadge.tsx?raw';
import proxmoxSettingsPanelSource from '@/components/Settings/ProxmoxSettingsPanel.tsx?raw';
import proxmoxConfiguredNodesTableSource from '@/components/Settings/ProxmoxConfiguredNodesTable.tsx?raw';
import proxmoxDirectWorkspaceSource from '@/components/Settings/ProxmoxDirectWorkspace.tsx?raw';
import proxmoxNodeModalStackSource from '@/components/Settings/ProxmoxNodeModalStack.tsx?raw';
import proxmoxSettingsModelSource from '@/components/Settings/proxmoxSettingsModel.ts?raw';
import proxmoxDirectWorkspaceStateSource from '@/components/Settings/useProxmoxDirectWorkspaceState.ts?raw';
import infrastructureWorkspaceSource from '@/components/Settings/InfrastructureWorkspace.tsx?raw';
import infrastructureWorkspaceModelSource from '@/components/Settings/infrastructureWorkspaceModel.ts?raw';
import proxmoxSettingsPresentationSource from '@/utils/proxmoxSettingsPresentation.ts?raw';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';
import nodeModalAuthenticationSectionSource from '@/components/Settings/NodeModalAuthenticationSection.tsx?raw';
import nodeModalBasicInfoSectionSource from '@/components/Settings/NodeModalBasicInfoSection.tsx?raw';
import nodeModalModelSource from '@/components/Settings/nodeModalModel.ts?raw';
import nodeModalMonitoringSectionSource from '@/components/Settings/NodeModalMonitoringSection.tsx?raw';
import nodeModalSetupGuideSectionSource from '@/components/Settings/NodeModalSetupGuideSection.tsx?raw';
import nodeModalStatusFooterSource from '@/components/Settings/NodeModalStatusFooter.tsx?raw';
import nodeModalSource from '@/components/Settings/NodeModal.tsx?raw';
import nodeModalStateSource from '@/components/Settings/useNodeModalState.ts?raw';
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
import relayOnboardingCardStateSource from '@/components/Dashboard/useRelayOnboardingCardState.ts?raw';
import proLicensePanelStateSource from '@/components/Settings/useProLicensePanelState.ts?raw';
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
import alertsConfigurationSurfaceSource from '@/features/alerts/AlertsConfigurationSurface.tsx?raw';
import alertsConfigurationStateSource from '@/features/alerts/useAlertsConfigurationState.ts?raw';
import alertsConfigurationSnapshotStateSource from '@/features/alerts/useAlertsConfigurationSnapshotState.ts?raw';
import alertsConfigurationModelSource from '@/features/alerts/alertsConfigurationModel.ts?raw';
import alertOverridesModelSource from '@/features/alerts/alertOverridesModel.ts?raw';
import alertOverridesStateSource from '@/features/alerts/useAlertOverridesState.ts?raw';
import alertDestinationsModelSource from '@/features/alerts/alertDestinationsModel.ts?raw';
import alertDestinationsStateSource from '@/features/alerts/useAlertDestinationsState.ts?raw';
import alertDestinationsTabStateSource from '@/features/alerts/useAlertDestinationsTabState.ts?raw';
import alertWebhookDestinationsStateSource from '@/features/alerts/useAlertWebhookDestinationsState.ts?raw';
import alertAcknowledgementStateSource from '@/features/alerts/useAlertAcknowledgementState.ts?raw';
import alertHistoryAdministrationCardSource from '@/features/alerts/AlertHistoryAdministrationCard.tsx?raw';
import alertHistoryFiltersCardSource from '@/features/alerts/AlertHistoryFiltersCard.tsx?raw';
import alertHistoryFrequencyCardSource from '@/features/alerts/AlertHistoryFrequencyCard.tsx?raw';
import alertHistoryTableAlertRowSource from '@/features/alerts/AlertHistoryTableAlertRow.tsx?raw';
import alertHistoryTableGroupRowSource from '@/features/alerts/AlertHistoryTableGroupRow.tsx?raw';
import alertHistoryTableSectionSource from '@/features/alerts/AlertHistoryTableSection.tsx?raw';
import alertResourceIncidentsPanelSource from '@/features/alerts/AlertResourceIncidentsPanel.tsx?raw';
import alertHistoryStateSource from '@/features/alerts/useAlertHistoryState.ts?raw';
import alertResourceIncidentsStateSource from '@/features/alerts/useAlertResourceIncidentsState.ts?raw';
import alertHistoryModelSource from '@/features/alerts/alertHistoryModel.ts?raw';
import alertIncidentTimelineStateSource from '@/features/alerts/useAlertIncidentTimelineState.ts?raw';
import alertOverviewActiveAlertsSectionSource from '@/features/alerts/AlertOverviewActiveAlertsSection.tsx?raw';
import alertOverviewAlertCardSource from '@/features/alerts/AlertOverviewAlertCard.tsx?raw';
import alertOverviewStatsCardsSource from '@/features/alerts/AlertOverviewStatsCards.tsx?raw';
import alertOverviewStateSource from '@/features/alerts/useAlertOverviewState.ts?raw';
import alertScheduleStateSource from '@/features/alerts/useAlertScheduleState.ts?raw';
import alertDestinationsTabSource from '@/features/alerts/tabs/DestinationsTab.tsx?raw';
import alertHistoryTabSource from '@/features/alerts/tabs/HistoryTab.tsx?raw';
import alertScheduleTabSource from '@/features/alerts/tabs/ScheduleTab.tsx?raw';
import alertThresholdsTabSource from '@/features/alerts/tabs/ThresholdsTab.tsx?raw';
import thresholdsTabModelSource from '@/features/alerts/thresholds/thresholdsTabModel.ts?raw';
import thresholdsTableSource from '@/components/Alerts/ThresholdsTable.tsx?raw';
import thresholdsTableAgentDisksSectionSource from '@/components/Alerts/ThresholdsTableAgentDisksSection.tsx?raw';
import thresholdsTableAgentsTabSource from '@/components/Alerts/ThresholdsTableAgentsTab.tsx?raw';
import thresholdsTableAgentsResourcesSectionSource from '@/components/Alerts/ThresholdsTableAgentsResourcesSection.tsx?raw';
import thresholdsTableDockerContainersSectionSource from '@/components/Alerts/ThresholdsTableDockerContainersSection.tsx?raw';
import thresholdsTableDockerHostsSectionSource from '@/components/Alerts/ThresholdsTableDockerHostsSection.tsx?raw';
import thresholdsTableDockerIgnoredPrefixesSectionSource from '@/components/Alerts/ThresholdsTableDockerIgnoredPrefixesSection.tsx?raw';
import thresholdsTableDockerServiceGapSectionSource from '@/components/Alerts/ThresholdsTableDockerServiceGapSection.tsx?raw';
import thresholdsTableDockerTabSource from '@/components/Alerts/ThresholdsTableDockerTab.tsx?raw';
import thresholdsTablePMGTabSource from '@/components/Alerts/ThresholdsTablePMGTab.tsx?raw';
import thresholdsTableProxmoxBackupsSectionSource from '@/components/Alerts/ThresholdsTableProxmoxBackupsSection.tsx?raw';
import thresholdsTableProxmoxGuestFilteringSectionSource from '@/components/Alerts/ThresholdsTableProxmoxGuestFilteringSection.tsx?raw';
import thresholdsTableProxmoxGuestsSectionSource from '@/components/Alerts/ThresholdsTableProxmoxGuestsSection.tsx?raw';
import thresholdsTableProxmoxNodesSectionSource from '@/components/Alerts/ThresholdsTableProxmoxNodesSection.tsx?raw';
import thresholdsTableProxmoxPBSSectionSource from '@/components/Alerts/ThresholdsTableProxmoxPBSSection.tsx?raw';
import thresholdsTableProxmoxSnapshotsSectionSource from '@/components/Alerts/ThresholdsTableProxmoxSnapshotsSection.tsx?raw';
import thresholdsTableProxmoxStorageSectionSource from '@/components/Alerts/ThresholdsTableProxmoxStorageSection.tsx?raw';
import thresholdsTableProxmoxTabSource from '@/components/Alerts/ThresholdsTableProxmoxTab.tsx?raw';
import thresholdsDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsData.ts?raw';
import thresholdsHostDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsHostData.ts?raw';
import thresholdsDockerDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsDockerData.ts?raw';
import thresholdsGuestDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsGuestData.ts?raw';
import thresholdsInfrastructureDataHookSource from '@/features/alerts/thresholds/hooks/useThresholdsInfrastructureData.ts?raw';
import thresholdsRecoveryDefaultsStateHookSource from '@/features/alerts/thresholds/hooks/useThresholdsRecoveryDefaultsState.ts?raw';
import thresholdsTableStateHookSource from '@/features/alerts/thresholds/hooks/useThresholdsTableState.ts?raw';
import thresholdsAvailabilityMutationsHookSource from '@/features/alerts/thresholds/hooks/useThresholdsAvailabilityMutations.ts?raw';
import thresholdsOverrideMutationsHookSource from '@/features/alerts/thresholds/hooks/useThresholdsOverrideMutations.ts?raw';
import thresholdsOverrideMutationModelSource from '@/features/alerts/thresholds/thresholdsOverrideMutationModel.ts?raw';
import thresholdsResourceModelSource from '@/features/alerts/thresholds/thresholdsResourceModel.ts?raw';
import thresholdsTableSectionPropsSource from '@/features/alerts/thresholds/thresholdsTableSectionProps.ts?raw';
import alertIncidentPresentationSource from '@/utils/alertIncidentPresentation.ts?raw';
import alertHistoryPresentationSource from '@/utils/alertHistoryPresentation.ts?raw';
import bulkEditDialogSource from '@/components/Alerts/BulkEditDialog.tsx?raw';
import alertResourceTableModelSource from '@/components/Alerts/alertResourceTableModel.ts?raw';
import alertResourceGroupHeaderSource from '@/components/Alerts/AlertResourceGroupHeader.tsx?raw';
import alertResourceTableDesktopSource from '@/components/Alerts/AlertResourceTableDesktop.tsx?raw';
import alertResourceTableMobileSource from '@/components/Alerts/AlertResourceTableMobile.tsx?raw';
import alertResourceTableRowSource from '@/components/Alerts/AlertResourceTableRow.tsx?raw';
import alertResourceTableSource from '@/components/Alerts/ResourceTable.tsx?raw';
import alertResourceTableStateSource from '@/components/Alerts/useAlertResourceTableState.ts?raw';
import emailProviderSelectSource from '@/components/Alerts/EmailProviderSelect.tsx?raw';
import emailProviderSelectStateSource from '@/components/Alerts/useEmailProviderSelectState.ts?raw';
import webhookConfigSource from '@/components/Alerts/WebhookConfig.tsx?raw';
import webhookConfigFormSource from '@/components/Alerts/WebhookConfigForm.tsx?raw';
import webhookConfigListSource from '@/components/Alerts/WebhookConfigList.tsx?raw';
import webhookConfigStateSource from '@/components/Alerts/useWebhookConfigState.ts?raw';
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
import agentProfilesPanelStateSource from '@/components/Settings/useAgentProfilesPanelState.ts?raw';
import agentProfilesPresentationSource from '@/utils/agentProfilesPresentation.ts?raw';
import agentProfileSuggestionPresentationSource from '@/utils/agentProfileSuggestionPresentation.ts?raw';
import organizationAccessPanelSource from '@/components/Settings/OrganizationAccessPanel.tsx?raw';
import organizationAccessLoadingStateSource from '@/components/Settings/OrganizationAccessLoadingState.tsx?raw';
import organizationAccessManagementSectionSource from '@/components/Settings/OrganizationAccessManagementSection.tsx?raw';
import organizationAccessMembersSectionSource from '@/components/Settings/OrganizationAccessMembersSection.tsx?raw';
import billingAdminOrganizationsTableSource from '@/components/Settings/BillingAdminOrganizationsTable.tsx?raw';
import billingAdminPanelSource from '@/components/Settings/BillingAdminPanel.tsx?raw';
import billingAdminPanelStateSource from '@/components/Settings/useBillingAdminPanelState.ts?raw';
import organizationBillingLoadingStateSource from '@/components/Settings/OrganizationBillingLoadingState.tsx?raw';
import organizationBillingPanelSource from '@/components/Settings/OrganizationBillingPanel.tsx?raw';
import organizationOverviewLoadingStateSource from '@/components/Settings/OrganizationOverviewLoadingState.tsx?raw';
import organizationOverviewMembersSectionSource from '@/components/Settings/OrganizationOverviewMembersSection.tsx?raw';
import organizationSharingCreateSectionSource from '@/components/Settings/OrganizationSharingCreateSection.tsx?raw';
import organizationOutgoingSharesSectionSource from '@/components/Settings/OrganizationOutgoingSharesSection.tsx?raw';
import organizationIncomingSharesSectionSource from '@/components/Settings/OrganizationIncomingSharesSection.tsx?raw';
import organizationAccessStateSource from '@/components/Settings/useOrganizationAccessPanelState.ts?raw';
import organizationBillingStateSource from '@/components/Settings/useOrganizationBillingPanelState.ts?raw';
import organizationOverviewStateSource from '@/components/Settings/useOrganizationOverviewPanelState.ts?raw';
import organizationSharingStateSource from '@/components/Settings/useOrganizationSharingPanelState.ts?raw';
import organizationRolePresentationSource from '@/utils/organizationRolePresentation.ts?raw';
import organizationSettingsPresentationSource from '@/utils/organizationSettingsPresentation.ts?raw';
import rbacFeatureGateSectionSource from '@/components/Settings/RBACFeatureGateSection.tsx?raw';
import rbacFeatureGateStateSource from '@/components/Settings/useRBACFeatureGateState.ts?raw';

const recoverySource = [
  recoveryComponentSource,
  recoveryProtectedInventorySectionSource,
  recoveryActivitySectionSource,
  recoveryHistorySectionSource,
  recoveryHistoryTableSource,
  recoveryHistorySectionStateSource,
].join('\n');
import rolesPanelSource from '@/components/Settings/RolesPanel.tsx?raw';
import rolesPanelStateSource from '@/components/Settings/useRolesPanelState.ts?raw';
import auditWebhookPanelSource from '@/components/Settings/AuditWebhookPanel.tsx?raw';
import auditWebhookStateSource from '@/components/Settings/useAuditWebhookPanelState.ts?raw';
import auditWebhookPresentationSource from '@/utils/auditWebhookPresentation.ts?raw';
import auditLogPanelSource from '@/components/Settings/AuditLogPanel.tsx?raw';
import auditLogStateSource from '@/components/Settings/useAuditLogPanelState.ts?raw';
import auditLogPresentationSource from '@/utils/auditLogPresentation.ts?raw';
import ssoProvidersPanelSource from '@/components/Settings/SSOProvidersPanel.tsx?raw';
import ssoProvidersStateSource from '@/components/Settings/useSSOProvidersState.ts?raw';
import ssoProvidersModelSource from '@/components/Settings/ssoProvidersModel.ts?raw';
import ssoProviderPresentationSource from '@/utils/ssoProviderPresentation.ts?raw';
import userAssignmentsPanelSource from '@/components/Settings/UserAssignmentsPanel.tsx?raw';
import userAssignmentsPanelStateSource from '@/components/Settings/useUserAssignmentsPanelState.ts?raw';
import investigationMessagesSource from '@/components/patrol/InvestigationMessages.tsx?raw';
import investigationSectionSource from '@/components/patrol/InvestigationSection.tsx?raw';
import runHistoryPanelSource from '@/components/patrol/RunHistoryPanel.tsx?raw';
import runToolCallTraceSource from '@/components/patrol/RunToolCallTrace.tsx?raw';
import diagnosticsPanelSource from '@/components/Settings/DiagnosticsPanel.tsx?raw';
import diagnosticsPresentationSource from '@/utils/diagnosticsPresentation.ts?raw';
import aiSettingsShellSource from '@/components/Settings/AISettings.tsx?raw';
import aiChatMaintenanceSectionSource from '@/components/Settings/AIChatMaintenanceSection.tsx?raw';
import aiProviderConfigurationSectionSource from '@/components/Settings/AIProviderConfigurationSection.tsx?raw';
import aiSettingsDialogsSource from '@/components/Settings/AISettingsDialogs.tsx?raw';
import aiModelSelectionSectionSource from '@/components/Settings/AIModelSelectionSection.tsx?raw';
import aiSettingsModelSource from '@/components/Settings/aiSettingsModel.ts?raw';
import aiRuntimeControlsSectionSource from '@/components/Settings/AIRuntimeControlsSection.tsx?raw';
import aiSettingsStatusAndActionsSource from '@/components/Settings/AISettingsStatusAndActions.tsx?raw';
import aiSettingsStateSource from '@/components/Settings/useAISettingsState.ts?raw';
import aiIntelligenceSource from '@/pages/AIIntelligence.tsx?raw';
import patrolIntelligenceBannersSource from '@/features/patrol/PatrolIntelligenceBanners.tsx?raw';
import patrolIntelligenceHeaderSource from '@/features/patrol/PatrolIntelligenceHeader.tsx?raw';
import patrolIntelligenceSummarySource from '@/features/patrol/PatrolIntelligenceSummary.tsx?raw';
import patrolIntelligenceSurfaceSource from '@/features/patrol/PatrolIntelligenceSurface.tsx?raw';
import patrolIntelligenceStateSource from '@/features/patrol/usePatrolIntelligenceState.ts?raw';
import patrolIntelligenceWorkspaceSource from '@/features/patrol/PatrolIntelligenceWorkspace.tsx?raw';
import aiQuickstartPresentationSource from '@/utils/aiQuickstartPresentation.ts?raw';
import aiCostPresentationSource from '@/utils/aiCostPresentation.ts?raw';
import thresholdSliderPresentationSource from '@/utils/thresholdSliderPresentation.ts?raw';
import emptyStatePresentationSource from '@/utils/emptyStatePresentation.ts?raw';
import reportingPanelSource from '@/components/Settings/ReportingPanel.tsx?raw';
import systemLogsPanelSource from '@/components/Settings/SystemLogsPanel.tsx?raw';
import systemLogsPanelStateSource from '@/components/Settings/useSystemLogsPanelState.ts?raw';
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
import infrastructureDetailsDrawerModelSource from '@/components/shared/infrastructureDetailsDrawerModel.ts?raw';
import infrastructurePageSurfaceSource from '@/features/infrastructure/InfrastructurePageSurface.tsx?raw';
import infrastructurePageStateSource from '@/features/infrastructure/useInfrastructurePageState.ts?raw';
import infrastructurePageRouteStateSource from '@/features/infrastructure/useInfrastructurePageRouteState.ts?raw';
import infrastructureDetailsDrawerStateSource from '@/components/shared/useInfrastructureDetailsDrawerState.ts?raw';

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

const resourceDetailDrawerSource = [
  resourceDetailDrawerShellSource,
  resourceDetailDrawerOverviewSource,
  resourceDetailDrawerDebugSource,
  resourceDetailDrawerHistoryStateSource,
  resourceDetailDrawerDerivedStateSource,
  resourceDetailDrawerStateSource,
].join('\n');

const infrastructurePageSource = [
  infrastructurePageShellSource,
  infrastructurePageSurfaceSource,
  infrastructurePageRouteStateSource,
  infrastructurePageStateSource,
].join('\n');

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
    expect(resourceLinksSource).toContain('export const buildInfrastructureHrefForWorkload');
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
    expect(workloadTypePresentationSource).toContain('titleCaseDelimitedLabel');
    expect(workloadTypeBadgesSource).not.toContain('canonicalizeFrontendResourceType');
    expect(workloadTypeBadgesSource).toContain('getWorkloadTypePresentation');
    expect(sourcePlatformsSource).toContain('titleCaseDelimitedLabel');
    expect(sourcePlatformsSource).not.toContain('const titleize =');
    expect(storageSourcesSource).toContain('titleCaseDelimitedLabel');
    expect(storageSourcesSource).not.toContain('const titleCaseLabel =');
    expect(workloadsSource).toContain('export const normalizeWorkloadViewModeParam');
    expect(orgScopeSource).toContain("export const DEFAULT_ORG_SCOPE = 'default'");
    expect(orgScopeSource).toContain('export const normalizeOrgScope');
    expect(dashboardSource).toContain('useDashboardState');
    expect(dashboardSource).toContain('DashboardStateCards');
    expect(dashboardSource).toContain('DashboardStatsStrip');
    expect(dashboardSource).toContain('DashboardWorkloadTable');
    expect(dashboardSource).not.toContain('const [search, setSearch] = createSignal(');
    expect(dashboardStateSource).toContain('useDashboardControlsState');
    expect(dashboardStateSource).toContain('useDashboardGuestMetadataState');
    expect(dashboardStateSource).toContain('useDashboardSelectionState');
    expect(dashboardStateSource).toContain('useDashboardWorkloadDerivedState');
    expect(dashboardStateSource).toContain('useDashboardWorkloadRouteState');
    expect(dashboardStateSource).toContain('filterWorkloads(params)');
    expect(dashboardStateSource).not.toContain('useBreakpoint');
    expect(dashboardStateSource).not.toContain('useColumnVisibility');
    expect(dashboardStateSource).not.toContain('blurFocusedTypeToSearch');
    expect(dashboardWorkloadRouteStateSource).toContain('useDashboardWorkloadUrlSync');
    expect(dashboardWorkloadRouteStateSource).not.toContain('normalizeWorkloadViewModeParam');
    expect(dashboardSource).not.toContain('function normalizeViewModeParam');
    expect(dashboardSource).not.toContain('workloadSummaryGuestId');
    expect(dashboardSource).not.toContain('createMemo(() => getCanonicalWorkloadId(guest()))');
    expect(workloadPanelSource).toContain(
      'createMemo(() => getCanonicalWorkloadId(guest()))',
    );
    expect(dashboardWorkloadTableSource).not.toContain(
      'createMemo(() => getCanonicalWorkloadId(guest()))',
    );
    expect(dashboardGuestMetadataStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(dashboardStateSource).not.toContain("const DEFAULT_ORG_SCOPE = 'default'");
    expect(dashboardStateSource).not.toContain('const normalizeOrgScope =');
    expect(dashboardStateSource).not.toContain('GuestMetadataAPI.getAllMetadata()');
    expect(dashboardGuestMetadataStateSource).toContain('GuestMetadataAPI.getAllMetadata()');
    expect(dashboardGuestMetadataStateSource).toContain("eventBus.on('org_switched'");
    expect(dashboardStateSource).not.toContain('buildWorkloadsPath({');
    expect(dashboardStateSource).not.toContain('parseWorkloadsLinkSearch');
    expect(dashboardStateSource).not.toContain('const [selectedGuestId, setSelectedGuestIdRaw]');
    expect(dashboardStateSource).not.toContain('const [hoveredWorkloadId, setHoveredWorkloadId]');
    expect(dashboardStateSource).not.toContain('groupWorkloads(');
    expect(dashboardStateSource).not.toContain('computeWorkloadStats(');
    expect(dashboardStateSource).not.toContain('computeWorkloadIOEmphasis(');
    expect(dashboardStateSource).not.toContain('buildNodeByInstance(');
    expect(dashboardStateSource).not.toContain('buildGuestParentNodeMap(');
    expect(dashboardWorkloadRouteStateSource).not.toContain('buildWorkloadsPath({');
    expect(dashboardWorkloadRouteStateSource).not.toContain('const workloadNodeOptions = createMemo');
    expect(dashboardWorkloadRouteStateSource).not.toContain(
      'const containerRuntimeFilterConfig = createMemo',
    );
    expect(dashboardWorkloadRouteStateSource).not.toContain('const [handledTypeParam, setHandledTypeParam]');
    expect(dashboardWorkloadRouteStateSource).not.toContain(
      "const [workloadsRouteActive, setWorkloadsRouteActive] = createSignal(false)",
    );
    expect(dashboardWorkloadRouteStateSource).toContain(
      "from './dashboardWorkloadRouteStateModel'",
    );
    expect(dashboardWorkloadRouteStateSource).toContain(
      'resolveDashboardWorkloadNodeSelection({',
    );
    expect(dashboardWorkloadRouteStateSource).toContain('DASHBOARD_WORKLOAD_ROUTE_RESET_STATE');
    expect(dashboardWorkloadUrlSyncSource).not.toContain('buildWorkloadsPath({');
    expect(dashboardWorkloadUrlSyncSource).not.toContain('normalizeWorkloadViewModeParam');
    expect(dashboardWorkloadUrlSyncSource).not.toContain('parseWorkloadsLinkSearch');
    expect(dashboardWorkloadUrlSyncSource).toContain("from './dashboardWorkloadRouteModel'");
    expect(dashboardWorkloadUrlSyncSource).toContain("from './dashboardWorkloadUrlSyncModel'");
    expect(dashboardWorkloadUrlSyncSource).toContain(
      'const [handledTypeParam, setHandledTypeParam]',
    );
    expect(dashboardWorkloadUrlSyncSource).toContain('parseDashboardWorkloadUrlParams');
    expect(dashboardWorkloadUrlSyncSource).toContain(
      'resolveDashboardManagedWorkloadsNavigateTarget({',
    );
    expect(dashboardWorkloadUrlSyncModelSource).toContain('parseWorkloadsLinkSearch(search)');
    expect(dashboardWorkloadUrlSyncModelSource).toContain('buildWorkloadsPath({');
    expect(dashboardWorkloadUrlSyncModelSource).toContain(
      'resolveDashboardManagedWorkloadsNavigateTarget',
    );
    expect(dashboardWorkloadUrlSyncModelSource).toContain(
      'resolveDashboardWorkloadRuntimeParam',
    );
    expect(dashboardWorkloadUrlSyncModelSource).toContain(
      'normalizeWorkloadViewModeParam(params.type)',
    );
    expect(dashboardControlsStateSource).toContain('useBreakpoint');
    expect(dashboardControlsStateSource).toContain('useColumnVisibility');
    expect(dashboardControlsStateSource).toContain('usePersistentSignal');
    expect(dashboardControlsStateSource).toContain('blurFocusedTypeToSearch');
    expect(dashboardControlsStateSource).toContain('DEFAULT_DASHBOARD_SORT_KEY');
    expect(dashboardWorkloadRouteStateSource).toContain('useDashboardWorkloadFilterOptions');
    expect(dashboardWorkloadFilterOptionsSource).toContain(
      "from './dashboardWorkloadFilterConfigModel'",
    );
    expect(dashboardWorkloadFilterOptionsSource).toContain(
      'buildDashboardWorkloadNodeOptions(options.allGuests())',
    );
    expect(dashboardWorkloadFilterOptionsSource).not.toContain(
      'const onContextChange = (value: string) =>',
    );
    expect(dashboardWorkloadFilterOptionsSource).toContain(
      'buildDashboardContainerRuntimeFilterConfig({',
    );
    expect(dashboardWorkloadFilterOptionsSource).toContain('buildDashboardHostFilterConfig({');
    expect(dashboardWorkloadFilterOptionsSource).toContain(
      'buildDashboardNamespaceFilterConfig({',
    );
    expect(dashboardWorkloadFilterConfigModelSource).toContain(
      'export const buildDashboardContainerRuntimeFilterConfig',
    );
    expect(dashboardWorkloadFilterConfigModelSource).toContain(
      'export const buildDashboardHostFilterConfig',
    );
    expect(dashboardWorkloadFilterConfigModelSource).toContain(
      'export const buildDashboardNamespaceFilterConfig',
    );
    expect(dashboardWorkloadRouteModelSource).toContain(
      'export const deserializeDashboardWorkloadViewMode',
    );
    expect(dashboardWorkloadRouteModelSource).toContain(
      "normalizeWorkloadViewModeParam(raw) ?? 'all'",
    );
    expect(dashboardWorkloadRouteModelSource).not.toContain(
      'export const buildDashboardContainerRuntimeFilterConfig',
    );
    expect(dashboardWorkloadRouteModelSource).not.toContain(
      'export const buildDashboardHostFilterConfig',
    );
    expect(dashboardWorkloadRouteModelSource).not.toContain(
      'export const buildDashboardNamespaceFilterConfig',
    );
    expect(dashboardWorkloadRouteStateModelSource).toContain(
      'export const DASHBOARD_WORKLOAD_ROUTE_RESET_STATE',
    );
    expect(dashboardWorkloadRouteStateModelSource).toContain(
      'export const resolveDashboardWorkloadNodeSelection',
    );
    expect(dashboardWorkloadRouteStateModelSource).toContain(
      'export const deserializeDashboardContainerRuntime',
    );
    expect(dashboardWorkloadDerivedStateSource).toContain('groupWorkloads(');
    expect(dashboardWorkloadDerivedStateSource).toContain('computeWorkloadStats(');
    expect(dashboardWorkloadDerivedStateSource).toContain('computeWorkloadIOEmphasis(');
    expect(dashboardWorkloadDerivedStateSource).toContain("from './workloadTopology'");
    expect(dashboardWorkloadDerivedStateSource).toContain('buildNodeByInstance(');
    expect(dashboardWorkloadDerivedStateSource).toContain('buildGuestParentNodeMap(');
    expect(dashboardWorkloadRouteStateSource).not.toContain("from './workloadTopology'");
    expect(dashboardWorkloadRouteModelSource).toContain("from './workloadTopology'");
    expect(dashboardWorkloadRouteModelSource).toContain('workloadNodeScopeId');
    expect(dashboardWorkloadRouteModelSource).toContain('getKubernetesContextKey');
    expect(dashboardWorkloadRouteStateSource).toContain('isWorkloadsRoute,');
    expect(dashboardSelectionStateSource).toContain('const [selectedGuestId, setSelectedGuestIdRaw]');
    expect(dashboardSelectionStateSource).toContain('const [hoveredWorkloadId, setHoveredWorkloadId]');
    expect(dashboardSelectionStateSource).toContain('setHandledResourceId(null)');
    expect(dashboardSelectionStateSource).toContain("from './dashboardSelectionModel'");
    expect(dashboardSelectionStateSource).not.toContain('parseWorkloadsLinkSearch');
    expect(dashboardSelectionStateSource).not.toContain('getCanonicalWorkloadId');
    expect(dashboardSelectionModelSource).toContain('parseWorkloadsLinkSearch(search)');
    expect(dashboardSelectionModelSource).toContain('getCanonicalWorkloadId');
    expect(dashboardSelectionModelSource).toContain('resolveDashboardResourceSelection');
    expect(dashboardSelectionModelSource).toContain('dashboardHasHoveredWorkload');
    expect(dashboardStateSource).not.toContain('const guestId = () => {');
    expect(dashboardFilterSource).toContain('useDashboardFilterState');
    expect(dashboardFilterSource).not.toContain('const [filtersOpen, setFiltersOpen] =');
    expect(dashboardFilterSource).not.toContain('useBreakpoint');
    expect(dashboardFilterSource).not.toContain("props.setSortKey('name')");
    expect(dashboardFilterStateSource).toContain('countActiveDashboardFilters');
    expect(dashboardFilterStateSource).not.toContain('props.containerRuntimeFilter?.onChange');
    expect(dashboardFilterStateSource).toContain('useBreakpoint');
    expect(dashboardFilterStateSource).toContain('DEFAULT_DASHBOARD_SORT_KEY');
    expect(dashboardFilterModelSource).toContain('export const countActiveDashboardFilters');
    expect(dashboardFilterModelSource).toContain('export const hasActiveDashboardFilters');
    expect(dashboardFilterModelSource).toContain("DEFAULT_DASHBOARD_SORT_KEY: DashboardSortKey = 'type'");
    expect(dashboardStateSource).not.toContain('const containerRuntimeFilterConfig = createMemo');
    expect(dashboardStateSource).not.toContain('useGroupedTableWindowing');
    expect(dashboardWorkloadRouteStateSource).toContain('useDashboardWorkloadUrlSync');
    expect(dashboardWorkloadRouteStateSource).toContain('containerRuntimeFilterConfig');
    expect(dashboardWorkloadRouteStateSource).toContain('hostFilterConfig');
    expect(dashboardWorkloadRouteStateSource).toContain('namespaceFilterConfig');
    expect(dashboardWorkloadDerivedStateSource).toContain('useGroupedTableWindowing');
    expect(dashboardWorkloadDerivedStateSource).toContain('useDashboardWorkloadViewportSync');
    expect(dashboardWorkloadDerivedStateSource).not.toContain('window.addEventListener');
    expect(dashboardWorkloadDerivedStateSource).not.toContain('getBoundingClientRect');
    expect(dashboardStateSource).not.toContain('const DEFAULT_WINDOW_SIZE =');
    expect(dashboardStateSource).not.toContain('const DEFAULT_ENABLE_THRESHOLD =');
    expect(dashboardStateSource).not.toContain('const DEFAULT_OVERSCAN_ROWS =');
    expect(groupedTableWindowingSource).toContain('const DEFAULT_WINDOW_SIZE');
    expect(groupedTableWindowingSource).toContain('const DEFAULT_ENABLE_THRESHOLD');
    expect(groupedTableWindowingSource).toContain('const DEFAULT_OVERSCAN_ROWS');
    expect(groupedTableWindowingSource).toContain('getVisibleSlice');
    expect(groupedTableWindowingSource).toContain('onScroll');
    expect(groupedTableWindowingSource).toContain('revealIndex');
    expect(dashboardWorkloadViewportSyncSource).toContain('window.addEventListener');
    expect(dashboardWorkloadViewportSyncSource).toContain('window.removeEventListener');
    expect(dashboardWorkloadViewportSyncSource).toContain('getBoundingClientRect');
    expect(dashboardWorkloadViewportSyncSource).toContain('groupedWindowing.onScroll');
    expect(workloadTopologySource).toContain('export const workloadNodeScopeId');
    expect(workloadTopologySource).toContain('export const getKubernetesContextKey');
    expect(workloadTopologySource).toContain('export const getWorkloadDockerHostId');
    expect(workloadTopologySource).toContain('export const getDiscoveryHostIdForWorkload');
    expect(workloadTopologySource).toContain('export const getDiscoveryResourceIdForWorkload');
    expect(workloadTopologySource).toContain('export const buildNodeByInstance');
    expect(workloadTopologySource).toContain('export const buildGuestParentNodeMap');
    expect(thresholdSliderSource).toContain('useThresholdSliderState');
    expect(thresholdSliderSource).not.toContain('const [thumbPosition, setThumbPosition] =');
    expect(thresholdSliderSource).not.toContain('const handleMouseDown = () => {');
    expect(thresholdSliderSource).not.toContain('formatTemperature(');
    expect(thresholdSliderStateSource).toContain('window.addEventListener');
    expect(thresholdSliderStateSource).toContain('document.addEventListener');
    expect(thresholdSliderStateSource).toContain('onCleanup');
    expect(thresholdSliderModelSource).toContain('export function getThresholdSliderPosition');
    expect(thresholdSliderModelSource).toContain('export function getThresholdSliderThumbTransform');
    expect(thresholdSliderModelSource).toContain('export function getThresholdSliderTitle');
    expect(thresholdSliderModelSource).toContain('export function getThresholdSliderLabel');
    expect(stackedDiskBarSource).toContain('useStackedDiskBarState');
    expect(stackedDiskBarSource).not.toContain('const [containerWidth, setContainerWidth] =');
    expect(stackedDiskBarSource).not.toContain('const tooltipContent = createMemo(() => {');
    expect(stackedDiskBarSource).not.toContain('const SEGMENT_COLORS =');
    expect(stackedDiskBarStateSource).toContain('new ResizeObserver');
    expect(stackedDiskBarStateSource).toContain('useTooltip');
    expect(stackedDiskBarModelSource).toContain('export function buildStackedDiskBarPresentation');
    expect(stackedDiskBarModelSource).toContain('const SEGMENT_COLORS');
    expect(stackedMemoryBarSource).toContain('useStackedMemoryBarState');
    expect(stackedMemoryBarSource).not.toContain('const [containerWidth, setContainerWidth] =');
    expect(stackedMemoryBarSource).not.toContain('const segments = createMemo(() => {');
    expect(stackedMemoryBarSource).not.toContain('const MEMORY_COLORS =');
    expect(stackedMemoryBarStateSource).toContain('new ResizeObserver');
    expect(stackedMemoryBarStateSource).toContain('useTooltip');
    expect(stackedMemoryBarModelSource).toContain(
      'export function buildStackedMemoryBarPresentation',
    );
    expect(stackedMemoryBarModelSource).toContain('const MEMORY_COLORS');
    expect(metricBarSource).toContain('useMetricBarState');
    expect(metricBarSource).not.toContain('const [containerWidth, setContainerWidth] =');
    expect(metricBarSource).not.toContain('const progressColorClass = createMemo(() => {');
    expect(metricBarSource).not.toContain('const showSublabel = createMemo(() => {');
    expect(metricBarStateSource).toContain('new ResizeObserver');
    expect(metricBarModelSource).toContain('export function buildMetricBarPresentation');
    expect(metricBarModelSource).toContain('estimateTextWidth');
    expect(enhancedCpuBarSource).toContain('useEnhancedCPUBarState');
    expect(enhancedCpuBarSource).not.toContain('const tip = useTooltip()');
    expect(enhancedCpuBarSource).not.toContain('const barColor = createMemo(() =>');
    expect(enhancedCpuBarSource).not.toContain('const anomalyRatio = createMemo(() =>');
    expect(enhancedCpuBarStateSource).toContain('useTooltip');
    expect(enhancedCpuBarModelSource).toContain('export function buildEnhancedCPUBarPresentation');
    expect(enhancedCpuBarModelSource).toContain('tooltipUsageClass');
    expect(workloadsSummarySource).toContain('normalizeOrgScope(getOrgID())');
    expect(workloadsSummarySource).not.toContain("const DEFAULT_ORG_SCOPE = 'default'");
    expect(workloadsSummarySource).not.toContain('const normalizeOrgScope =');
    expect(infrastructureSummaryCacheSource).toContain('normalizeOrgScope(getOrgID())');
    expect(infrastructureSummaryCacheSource).not.toContain('const normalizeOrgScope =');
    expect(useWorkloadsSource).toContain('normalizeOrgScope(getOrgID())');
    expect(useWorkloadsSource).not.toContain('const normalizeOrgScope =');
    expect(useUnifiedResourcesSource).toContain('normalizeOrgScope(getOrgID())');
    expect(useUnifiedResourcesSource).not.toContain('const normalizeOrgScope =');
    expect(infrastructureSummarySource).toContain('useInfrastructureSummaryState');
    expect(infrastructureSummarySource).not.toContain('fetchInfrastructureSummaryAndCache');
    expect(infrastructureSummarySource).not.toContain('setInterval(');
    expect(infrastructureSummarySource).not.toContain('AbortController');
    expect(infrastructureSummaryStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(infrastructureSummaryStateSource).not.toContain("getOrgID() || 'default'");
    expect(storageSummarySource).toContain('normalizeOrgScope(getOrgID())');
    expect(storageSummarySource).not.toContain("getOrgID() || 'default'");
    expect(guestRowSource).toContain('useGuestRowState');
    expect(guestRowSource).toContain("from './GuestRowCells'");
    expect(guestRowSource).toContain("from '@/components/shared/TagBadges'");
    expect(guestRowSource).not.toContain("from './TagBadges'");
    expect(guestRowSource).not.toContain('const guestId = createMemo(');
    expect(guestRowSource).not.toContain('function NetworkInfoCell(');
    expect(guestRowSource).not.toContain('function OSInfoCell(');
    expect(guestRowStateSource).toContain('getCanonicalWorkloadId');
    expect(guestRowStateSource).toContain("from '@/routing/resourceLinks'");
    expect(guestRowStateSource).not.toContain("./infrastructureLink");
    expect(guestRowModelSource).toContain('export const GUEST_COLUMNS');
    expect(guestRowCellsSource).toContain('function NetworkInfoCell(');
    expect(guestRowCellsSource).toContain('function OSInfoCell(');
    expect(guestRowCellsSource).toContain('useTooltip');
    expect(guestRowSource).not.toContain('buildGuestId');
    expect(tagBadgesSource).toContain("from '@/components/shared/Tooltip'");
    expect(resourceDetailDrawerOverviewSource).toContain("from '@/components/shared/TagBadges'");
    expect(resourceDetailDrawerOverviewSource).not.toContain(
      "from '@/components/Dashboard/TagBadges'",
    );
    expect(resourceDetailDrawerShellSource).toContain(
      "from './ResourceDetailDrawerOverviewTab'",
    );
    expect(resourceDetailDrawerShellSource).toContain("from './ResourceDetailDrawerDebugTab'");
    expect(resourceDetailDrawerStateSource).toContain("from './useResourceDetailDrawerHistoryState'");
    expect(resourceDetailDrawerStateSource).toContain("from './useResourceDetailDrawerDerivedState'");
    expect(resourceDetailDrawerStateSource).toContain(
      "from './useResourceDetailDrawerDockerActionsState'",
    );
    expect(resourceDetailDrawerStateSource).not.toContain('createResource(');
    expect(resourceDetailDrawerStateSource).not.toContain('MonitoringAPI.');
    expect(resourceDetailDrawerHistoryStateSource).toContain('createResource(');
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from '@/components/Infrastructure/resourceDetailDiscoveryModel'",
    );
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from './resourceDetailDrawerOperationalModel'",
    );
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from './resourceDetailDrawerServiceModel'",
    );
    expect(resourceDetailDrawerDiscoveryModelSource).toContain('export const toDiscoveryConfig');
    expect(resourceDetailDrawerDerivedStateSource).not.toContain('buildWorkloadsHref');
    expect(resourceDetailDrawerDerivedStateSource).not.toContain('buildServiceDetailLinks');
    expect(resourceDetailDrawerDerivedStateSource).not.toContain('const supportedBadge =');
    expect(resourceDetailDrawerDerivedStateSource).not.toContain(
      'const links: Array<{ href: string;',
    );
    expect(resourceDetailDrawerOperationalModelSource).toContain('buildKubernetesCapabilityBadges');
    expect(resourceDetailDrawerOperationalModelSource).toContain('buildHostDetailCards');
    expect(resourceDetailDrawerOperationalModelSource).toContain('buildHostDetailSummary');
    expect(resourceDetailDrawerOperationalModelSource).toContain('buildRelatedLinks');
    expect(resourceDetailDrawerServiceModelSource).toContain('getServiceDetailsSummary');
    expect(resourceDetailDrawerOverviewSource).not.toContain('MonitoringAPI.');
    expect(resourceDetailDrawerOverviewSource).toContain('drawer.queueDockerUpdateCheck');
    expect(resourceDetailDrawerOverviewSource).toContain('drawer.queueDockerUpdateAll');
    expect(resourceDetailDrawerDockerActionsStateSource).toContain('MonitoringAPI.checkDockerUpdates');
    expect(resourceDetailDrawerDockerActionsStateSource).toContain(
      'MonitoringAPI.updateAllDockerContainers',
    );
    expect(guestDrawerSource).toContain('useGuestDrawerState');
    expect(guestDrawerSource).toContain('GuestDrawerOverview');
    expect(guestDrawerStateSource).toContain('getCanonicalWorkloadId');
    expect(guestDrawerStateSource).toContain("from '@/routing/resourceLinks'");
    expect(guestDrawerStateSource).not.toContain("./infrastructureLink");
    expect(guestDrawerStateSource).toContain('guestOsSummary');
    expect(guestDrawerSource).not.toContain('const guestId = () => {');
    expect(guestDrawerSource).not.toContain('WebInterfaceUrlField');
    expect(guestDrawerOverviewSource).toContain('WebInterfaceUrlField');
    expect(guestDrawerOverviewSource).toContain('DiskList');
    expect(dashboardStateCardsSource).toContain('dashboardDisconnectedState().actionLabel');
    expect(dashboardWorkloadTableSource).toContain('WorkloadTableHeader');
    expect(dashboardWorkloadTableSource).toContain('WorkloadPanel');
    expect(dashboardWorkloadTableSource).not.toContain('<TableHead');
    expect(dashboardWorkloadTableSource).not.toContain('NodeGroupHeader');
    expect(dashboardWorkloadTableSource).not.toContain('GuestDrawer');
    expect(workloadTableHeaderSource).toContain('TableHead');
    expect(workloadTableHeaderSource).toContain("col.sortKey as WorkloadSortKey");
    expect(workloadTableHeaderSource).not.toContain('NodeGroupHeader');
    expect(workloadPanelSource).toContain('NodeGroupHeader');
    expect(workloadPanelSource).toContain('GuestDrawer');
    expect(workloadPanelSource).toContain('createMemo(() => getCanonicalWorkloadId(guest()))');
    expect(workloadPanelSource).not.toContain('TableHead');
    expect(dashboardStatsStripSource).toContain('totalStats().running');
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
    expect(recoverySummarySource).not.toContain(
      "const RECOVERY_TIME_RANGES: readonly string[] = ['7d', '30d', '90d']",
    );
    expect(recoverySummarySource).not.toContain(
      'const RECOVERY_TIME_RANGE_LABELS: Record<string, string>',
    );
    expect(recoverySummarySource).not.toContain('const FRESHNESS_LABELS:');
    expect(recoverySource).toContain('getRecoveryArtifactModePresentation');
    expect(recoverySource).not.toContain('const MODE_LABELS: Record<ArtifactMode, string>');
    expect(recoverySource).not.toContain('const MODE_BADGE_CLASS: Record<ArtifactMode, string>');
    expect(recoverySource).not.toContain('const CHART_SEGMENT_CLASS: Record<ArtifactMode, string>');
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
    expect(recoverySource).toContain(
      "import { useRecoverySurfaceState } from '@/features/recovery/useRecoverySurfaceState';",
    );
    expect(recoveryComponentSource).toContain('useRecoverySurfaceState');
    expect(recoveryHistorySectionSource).toContain('useRecoveryHistorySectionState');
    expect(recoveryHistorySectionSource).toContain('RecoveryHistoryTable');
    expect(recoveryHistorySectionStateSource).toContain(
      'export function useRecoveryHistorySectionState',
    );
    expect(recoverySource).not.toContain('parseRecoveryLinkSearch');
    expect(recoverySource).not.toContain('buildRecoveryPath');
    expect(recoverySource).not.toContain('useRecoveryRollups');
    expect(recoverySource).not.toContain('useRecoveryPointsFacets');
    expect(recoverySurfaceStateSource).toContain('export function useRecoverySurfaceState');
    expect(recoverySurfaceStateSource).toContain('parseRecoveryLinkSearch');
    expect(recoverySurfaceStateSource).toContain('buildRecoveryPath');
    expect(recoverySurfaceStateSource).toContain('useRecoveryRollups');
    expect(recoverySurfaceStateSource).toContain('useRecoveryPoints');
    expect(recoverySurfaceStateSource).toContain('useRecoveryPointsFacets');
    expect(recoverySurfaceStateSource).toContain('useRecoveryPointsSeries');
    expect(recoveryComponentSource.indexOf('<RecoveryActivitySection')).toBeGreaterThan(-1);
    expect(recoveryComponentSource.indexOf('<RecoveryActivitySection')).toBeLessThan(
      recoveryComponentSource.indexOf('<Show when={!recoveryPoints.response.error}>'),
    );
    expect(recoverySource).toContain('getRecoveryFilterChipPresentation');
    expect(recoverySource).not.toContain('const titleize =');
    expect(recoverySource).toContain(
      "segmentedButtonClass(props.chartRangeDays() === range, false, 'accent')",
    );
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
    expect(recoverySource).not.toContain(
      'rounded-md p-1 hover:text-base-content hover:bg-surface-hover',
    );
    expect(recoverySource).not.toContain(
      'const labelEvery = dayCount <= 7 ? 1 : dayCount <= 30 ? 3 : 10',
    );
    expect(recoverySource).not.toContain('class="flex items-center gap-1"');
    expect(recoverySource).not.toContain(
      'class="inline-flex rounded border border-border bg-surface p-0.5 text-xs"',
    );
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
    expect(recoverySource).not.toContain(
      'Pulse hasn’t observed any protected items for this org yet.',
    );
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
    expect(recoveryTablePresentationSource).toContain('titleCaseDelimitedLabel');
    expect(recoveryTablePresentationSource).not.toContain('const titleize =');
    expect(recoveryDatePresentationSource).toContain(
      'export function recoveryDateKeyFromTimestamp',
    );
    expect(recoveryDatePresentationSource).toContain('export function parseRecoveryDateKey');
    expect(recoveryDatePresentationSource).toContain('export function getRecoveryPrettyDateLabel');
    expect(recoveryDatePresentationSource).toContain('export function getRecoveryFullDateLabel');
    expect(recoveryDatePresentationSource).toContain('export function getRecoveryCompactAxisLabel');
    expect(recoveryDatePresentationSource).toContain('export function formatRecoveryTimeOnly');
    expect(recoveryDatePresentationSource).toContain('export function getRecoveryNiceAxisMax');
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
    expect(recoveryTablePresentationSource).toContain('export function getRecoveryRollupIssueTone');
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
    expect(recoverySummaryPresentationSource).toContain('export const RECOVERY_FRESHNESS_BUCKETS');
    expect(recoverySummaryPresentationSource).toContain(
      'export function getRecoveryAttentionChipClass',
    );
    expect(recoverySummaryPresentationSource).toContain(
      'export function getRecoveryAttentionDotClass',
    );
    expect(recoverySummarySource).toContain('getRecoveryAttentionChipClass');
    expect(recoverySummarySource).toContain('getRecoveryAttentionDotClass');
    expect(recoverySummarySource).not.toContain('function getAttentionChipClass(');
    expect(recoverySummarySource).not.toContain('function getAttentionDotClass(');
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
    expect(dashboardTrendPresentationSource).toContain('export function getDashboardTrendColor');
    expect(dashboardTrendPresentationSource).toContain(
      'export function getDashboardTrendErrorState',
    );
    expect(systemSettingsPresentationSource).toContain('export const PVE_POLLING_PRESETS');
    expect(systemSettingsPresentationSource).toContain('export const BACKUP_INTERVAL_OPTIONS');
    expect(systemSettingsPresentationSource).toContain(
      'export function getBackupIntervalSelectValue',
    );
    expect(systemSettingsPresentationSource).toContain('export function getBackupIntervalSummary');
    expect(systemSettingsPresentationSource).toContain('export const COMMON_DISCOVERY_SUBNETS');
    expect(networkSettingsPanelSource).toContain('./NetworkDiscoverySection');
    expect(networkSettingsPanelSource).toContain('./NetworkBoundarySettingsSection');
    expect(networkDiscoverySectionSource).toContain('COMMON_DISCOVERY_SUBNETS');
    expect(networkBoundarySettingsSectionSource).toContain(
      'Allowed Private IP Ranges for Webhooks',
    );
    expect(networkSettingsModelSource).toContain('export type NetworkDiscoverySectionProps');
    expect(networkSettingsModelSource).toContain('export type NetworkBoundarySettingsSectionProps');
    expect(settingsShellSource).toContain('./useDiscoverySettingsState');
    expect(settingsShellSource).toContain('./useSettingsInfrastructurePanelProps');
    expect(settingsShellSource).toContain('./useSettingsPanelRegistry');
    expect(settingsShellSource).toContain('./settingsTabSaveBehavior');
    expect(settingsShellSource).toContain('./useSettingsSystemPanels');
    expect(settingsShellSource).toContain('const discoverySettings = useDiscoverySettingsState()');
    expect(settingsShellSource).toContain('const systemPanels = useSettingsSystemPanels({');
    expect(settingsShellSource).toContain(
      'const infrastructurePanelProps = useSettingsInfrastructurePanelProps({',
    );
    expect(settingsShellSource).toContain(
      'const settingsPanelRegistry = useSettingsPanelRegistry({',
    );
    expect(settingsPanelRegistrySource).toContain('buildSettingsPanelRegistryContext');
    expect(settingsShellSource).not.toContain('getInfrastructurePanelProps: () => ({');
    expect(settingsPanelRegistryContextSource).toContain('systemPanels: SettingsSystemPanels');
    expect(settingsPanelRegistryLoadersSource).toContain(
      'export const SETTINGS_PANEL_REGISTRY_LOADERS',
    );
    expect(settingsPanelRegistryLoadersSource).toContain("import('./InfrastructureWorkspace')");
    expect(settingsPanelRegistryContextSource).toContain(
      'getNetworkPanelProps: params.systemPanels.getNetworkPanelProps',
    );
    expect(settingsPanelRegistryContextSource).toContain('const systemBillingPanel: Component');
    expect(settingsPanelRegistryContextSource).toContain('getSecurityAuthPanelProps');
    expect(settingsNavigationModelSource).toContain('export type SettingsTab =');
    expect(settingsNavigationModelSource).toContain('export function resolveCanonicalSettingsPath');
    expect(settingsNavigationModelSource).toContain('export function settingsTabPath');
    expect(settingsNavigationHookSource).toContain('deriveTabFromPath');
    expect(settingsNavigationHookSource).toContain('resolveCanonicalSettingsPath');
    expect(settingsNavigationHookSource).toContain('settingsTabPath');
    expect(settingsRoutingSource).toContain("from './settingsNavigationModel'");
    expect(settingsTypesSource).toContain("from './settingsNavigationModel'");
    expect(settingsDialogsSource).toContain('UpdateConfirmationModal');
    expect(settingsDialogsSource).toContain('BackupTransferDialogs');
    expect(settingsShellStateSource).toContain('SETTINGS_HEADER_META');
    expect(settingsShellStateSource).toContain('setSidebarCollapsed');
    expect(settingsNavCatalogSource).toContain('export const SETTINGS_NAV_GROUPS');
    expect(settingsNavVisibilitySource).toContain('shouldHideSettingsNavItem');
    expect(settingsTabSaveBehaviorSource).toContain('getSettingsNavItem(tab)?.saveBehavior');
    expect(settingsPanelRegistrySource).not.toContain('allowedOrigins: params.');
    expect(settingsPanelRegistrySource).not.toContain('backupPollingEnabled: params.');
    expect(settingsSystemPanelsSource).toContain('GeneralSettingsPanel');
    expect(settingsSystemPanelsSource).toContain('getSettingsConfigurationLoadingState');
    expect(settingsSystemPanelsSource).toContain('allowedOrigins:');
    expect(settingsSystemPanelsSource).toContain('backupPollingEnabled:');
    expect(settingsInfrastructurePanelPropsSource).toContain('pbsInstanceFromResource');
    expect(settingsInfrastructurePanelPropsSource).toContain('pmgInstanceFromResource');
    expect(settingsInfrastructurePanelPropsSource).toContain('const agentStateResources = createMemo');
    expect(settingsInfrastructurePanelPropsSource).toContain('getInfrastructurePanelProps');
    expect(discoverySettingsStateSource).toContain('export function useDiscoverySettingsState');
    expect(discoverySettingsStateSource).toContain('normalizeSubnetList');
    expect(discoverySettingsStateSource).toContain('isValidCIDR');
    expect(reportingPanelSource).toContain('@/utils/upgradePresentation');
    expect(reportingPanelSource).toContain('@/components/Settings/useReportingPanelState');
    expect(reportingPanelSource).toContain('@/components/Settings/reportingPanelModel');
    expect(reportingPanelSource).toContain('getUpgradeActionButtonClass');
    expect(reportingPanelSource).toContain('UPGRADE_ACTION_LABEL');
    expect(reportingPanelSource).toContain('UPGRADE_TRIAL_LABEL');
    expect(reportingPanelSource).not.toContain('>Upgrade to Pro<');
    expect(reportingPanelSource).not.toContain('>Start free trial<');
    expect(reportingPanelSource).not.toContain('window.URL.createObjectURL');
    expect(rolesPanelSource).toContain('./RBACFeatureGateSection');
    expect(rolesPanelSource).toContain('./useRolesPanelState');
    expect(userAssignmentsPanelSource).toContain('./RBACFeatureGateSection');
    expect(userAssignmentsPanelSource).toContain('./useUserAssignmentsPanelState');
    expect(rbacFeatureGateSectionSource).toContain('@/utils/upgradePresentation');
    expect(rbacFeatureGateSectionSource).toContain('getUpgradeActionButtonClass');
    expect(rolesPanelStateSource).toContain('getRolesLoadErrorMessage');
    expect(userAssignmentsPanelStateSource).toContain('getUserAssignmentsLoadErrorMessage');
    expect(agentProfilesPanelSource).toContain('./useAgentProfilesPanelState');
    expect(agentProfilesPanelStateSource).toContain('@/utils/upgradePresentation');
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
    expect(organizationAccessManagementSectionSource).toContain(
      '@/utils/organizationRolePresentation',
    );
    expect(organizationAccessMembersSectionSource).toContain(
      '@/utils/organizationRolePresentation',
    );
    expect(organizationAccessManagementSectionSource).toContain(
      'ORGANIZATION_MEMBER_ROLE_OPTIONS',
    );
    expect(organizationAccessPanelSource).not.toContain(
      'const roleOptions: Array<{ value: OrganizationRole; label: string }> = [',
    );
    expect(organizationSharingCreateSectionSource).toContain('@/utils/organizationRolePresentation');
    expect(organizationSharingCreateSectionSource).toContain('ORGANIZATION_SHARE_ROLE_OPTIONS');
    expect(organizationOutgoingSharesSectionSource).toContain('normalizeOrganizationShareRole');
    expect(organizationIncomingSharesSectionSource).toContain('normalizeOrganizationShareRole');
    expect(organizationSharingCreateSectionSource).not.toContain(
      'const accessRoleOptions: Array<{ value: ShareAccessRole; label: string }> = [',
    );
    expect(organizationOutgoingSharesSectionSource).not.toContain('const normalizeShareRole =');
    expect(organizationRolePresentationSource).toContain(
      'export const ORGANIZATION_MEMBER_ROLE_OPTIONS',
    );
    expect(organizationRolePresentationSource).toContain(
      'export const ORGANIZATION_SHARE_ROLE_OPTIONS',
    );
    expect(organizationRolePresentationSource).toContain(
      'export function normalizeOrganizationShareRole',
    );
    expect(organizationAccessStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationOverviewStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationSharingStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationBillingPanelSource).toContain('./useOrganizationBillingPanelState');
    expect(organizationBillingPanelSource).toContain('./OrganizationBillingLoadingState');
    expect(organizationBillingStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationAccessStateSource).toContain('@/utils/organizationSettingsPresentation');
    expect(billingAdminPanelStateSource).toContain('@/utils/organizationSettingsPresentation');
    expect(billingAdminOrganizationsTableSource).toContain('@/utils/licensePresentation');
    expect(organizationOverviewStateSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationSharingStateSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationBillingPanelSource).toContain('@/utils/organizationSettingsPresentation');
    expect(organizationBillingStateSource).toContain('@/utils/licensePresentation');
    expect(organizationBillingStateSource).toContain('@/utils/organizationSettingsPresentation');
    expect(billingAdminPanelSource).toContain('./useBillingAdminPanelState');
    expect(billingAdminPanelSource).toContain('./BillingAdminOrganizationsTable');
    expect(relayOnboardingCardSource).toContain('./useRelayOnboardingCardState');
    expect(relayOnboardingCardSource).not.toContain('createSignal(');
    expect(relayOnboardingCardSource).not.toContain('RelayAPI.getStatus()');
    expect(relayOnboardingCardStateSource).toContain('RelayAPI.getStatus()');
    expect(relayOnboardingCardStateSource).toContain('loadLicenseStatus()');
    expect(relayOnboardingCardStateSource).toContain('startProTrial()');
    expect(organizationBillingPanelSource).not.toContain('normalizeOrgScope(getOrgID())');
    expect(organizationBillingPanelSource).not.toContain('createSignal(');
    expect(billingAdminPanelSource).not.toContain('createSignal(');
    expect(organizationBillingPanelSource).not.toContain('Grace Period');
    expect(organizationBillingPanelSource).not.toContain('No License');
    expect(organizationAccessPanelSource).not.toContain(
      'Multi-tenant requires an Enterprise license',
    );
    expect(organizationOverviewStateSource).not.toContain(
      'Multi-tenant is not enabled on this server',
    );
    expect(organizationSharingStateSource).not.toContain(
      'Failed to load organization sharing details',
    );
    expect(organizationAccessMembersSectionSource).toContain('getOrganizationAccessEmptyState');
    expect(organizationAccessLoadingStateSource).toContain('animate-pulse');
    expect(organizationOverviewMembersSectionSource).toContain(
      'getOrganizationOverviewMembersEmptyState',
    );
    expect(organizationOverviewLoadingStateSource).toContain('animate-pulse');
    expect(organizationBillingLoadingStateSource).toContain('animate-pulse');
    expect(organizationOutgoingSharesSectionSource).toContain(
      'getOrganizationOutgoingSharesEmptyState',
    );
    expect(organizationIncomingSharesSectionSource).toContain(
      'getOrganizationIncomingSharesEmptyState',
    );
    expect(organizationSharingStateSource).toContain(
      'getOrganizationShareTargetOrgRequiredMessage',
    );
    expect(organizationSharingStateSource).toContain('getOrganizationShareCreateSuccessMessage');
    expect(organizationAccessMembersSectionSource).not.toContain('No organization members found.');
    expect(organizationOverviewMembersSectionSource).not.toContain('No members found.');
    expect(organizationOutgoingSharesSectionSource).not.toContain('No outgoing shares configured.');
    expect(organizationIncomingSharesSectionSource).not.toContain(
      'No incoming shares from other organizations.',
    );
    expect(organizationSharingStateSource).not.toContain(
      "notificationStore.error('Target organization is required')",
    );
    expect(organizationSharingStateSource).not.toContain(
      "notificationStore.success('Resource shared successfully')",
    );
    expect(organizationSharingStateSource).not.toContain(
      "notificationStore.success('Share removed')",
    );
    expect(organizationBillingPanelSource).not.toContain('This feature is not available.');
    expect(billingAdminPanelSource).not.toContain('This feature is not available.');
    expect(billingAdminPanelSource).not.toContain('Failed to list organizations');
    expect(billingAdminPanelSource).not.toContain('No trial');
    expect(billingAdminPanelSource).not.toContain('soft-deleted');
    expect(billingAdminPanelSource).not.toContain('Organization billing suspended');
    expect(billingAdminPanelStateSource).toContain('getBillingAdminStateUpdateSuccessMessage');
    expect(billingAdminPanelStateSource).toContain('getOrganizationSettingsPanelLoadErrorMessage');
    expect(billingAdminOrganizationsTableSource).toContain('getBillingAdminTrialStatus');
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationSettingsLoadErrorMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export type OrganizationSettingsLoadContext =',
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
      'export function getOrganizationAccessManageRequiredMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationAccessRoleUpdatedMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationDisplayNameUpdatedMessage',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationOverviewMembersEmptyState',
    );
    expect(organizationSettingsPresentationSource).toContain(
      'export function getOrganizationOverviewManageRequiredMessage',
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
    expect(rolesPanelSource).toContain('getRolesEmptyState');
    expect(rolesPanelSource).not.toContain('Custom Roles (Pro)');
    expect(rolesPanelSource).not.toContain('No roles available.');
    expect(rbacFeatureGateStateSource).toContain('getRBACFeatureGateCopy');
    expect(userAssignmentsPanelSource).toContain('@/utils/rbacPresentation');
    expect(userAssignmentsPanelSource).toContain('getUserAssignmentsEmptyStateCopy');
    expect(userAssignmentsPanelSource).not.toContain('Centralized Access Control (Pro)');
    expect(userAssignmentsPanelSource).not.toContain('No users yet');
    expect(userAssignmentsPanelSource).not.toContain('Configure SSO in Security settings');
    expect(userAssignmentsPanelSource).not.toContain('Users sync on first login');
    expect(rbacFeatureGateStateSource).toContain('getRBACFeatureGateCopy');
    expect(rbacPresentationSource).toContain('export function getRBACFeatureGateCopy');
    expect(rbacPresentationSource).toContain('export function getRolesEmptyState');
    expect(rbacPresentationSource).toContain('export function getUserAssignmentsEmptyStateCopy');
    expect(agentProfilesPresentationSource).toContain('export function getAgentProfilesEmptyState');
    expect(agentProfilesPresentationSource).toContain(
      'export function getAgentProfileAssignmentsEmptyState',
    );
    expect(agentProfileSuggestionPresentationSource).toContain('titleCaseDelimitedLabel');
    expect(agentProfileSuggestionPresentationSource).not.toContain('const titleize =');
    expect(trendChartsSource).toContain("segmentedButtonClass(active(), false, 'accent')");
    expect(trendChartsSource).not.toContain(
      "'px-2 py-0.5 rounded bg-blue-600 text-white text-[11px] font-medium'",
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
    expect(recentAlertsPanelSource).toContain('getAlertSeverityCompactLabel');
    expect(recentAlertsPanelSource).toContain('@/utils/alertOverviewPresentation');
    expect(recentAlertsPanelSource).not.toContain('getAlertSeverityTextClass');
    expect(recentAlertsPanelSource).not.toContain("alert.level === 'critical' ? 'CRIT' : 'WARN'");
    expect(recentAlertsPanelSource).not.toContain('No active alerts');
    expect(dashboardRouteSource).toContain("from '@/features/dashboardOverview'");
    expect(alertOverviewPresentationSource).toContain('export function getDashboardAlertTone');
    expect(alertOverviewPresentationSource).toContain(
      'export function getDashboardAlertSummaryText',
    );
    expect(storagePanelSource).toContain('@/utils/dashboardStoragePresentation');
    expect(storagePanelSource).toContain('@/utils/dashboardMetricPresentation');
    expect(storagePanelSource).not.toContain('./dashboardHelpers');
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
    expect(diskDetailSource).toContain('attributeCards()');
    expect(diskDetailSource).toContain('historyCharts()');
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
    expect(diskDetailSource).not.toContain(
      'flex flex-wrap items-end justify-between gap-3 border-b border-border-subtle pb-3',
    );
    expect(diskDetailSource).not.toContain('class="relative"');
    expect(useDiskDetailModelSource).toContain('extractPhysicalDiskPresentationData');
    expect(useDiskDetailModelSource).toContain('resolvePhysicalDiskMetricResourceId');
    expect(diskLiveMetricSource).toContain('useDiskLiveMetricModel');
    expect(diskLiveMetricSource).not.toContain('const latestMetric = createMemo(() => {');
    expect(diskLiveMetricSource).not.toContain('const formatted = createMemo(() => {');
    expect(diskLiveMetricSource).not.toContain(
      "if (v > 90) return 'text-red-600 dark:text-red-400 font-bold'",
    );
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
    expect(storagePoolRowPresentationSource).toContain('export const buildStoragePoolRowModel');
    expect(storagePoolRowPresentationSource).toContain('getSourcePlatformPresentation');
    expect(storagePoolRowPresentationSource).toContain('getCompactStoragePoolProtectionLabel');
    expect(storagePoolRowPresentationSource).toContain('getCompactStoragePoolImpactLabel');
    expect(storagePoolRowPresentationSource).toContain('getCompactStoragePoolIssueLabel');
    expect(storagePoolRowPresentationSource).toContain('getCompactStoragePoolIssueSummary');
    expect(storagePoolRowPresentationSource).toContain('getCompactStoragePoolProtectionTitle');
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
    expect(useStorageCephModelSource).toContain('resolveCephClusterForStorageRecord');
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
    expect(storageRowPresentationSource).toContain('export function getStoragePoolIssueTextClass');
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
    expect(storageGroupPresentationSource).toContain('export const getStorageGroupPoolCountLabel');
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
    expect(zfsHealthMapSource).not.toContain('const getDeviceColor = (device: ZFSDevice) => {');
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
    expect(enhancedStorageBarSource).not.toContain(
      'metric-text w-full h-5 flex items-center min-w-0',
    );
    expect(useEnhancedStorageBarModelSource).toContain('getStorageBarUsagePercent');
    expect(useEnhancedStorageBarModelSource).toContain('getStorageBarLabel');
    expect(useEnhancedStorageBarModelSource).toContain('getStorageBarTooltipRows');
    expect(useEnhancedStorageBarModelSource).toContain('getStorageBarZfsSummary');
    expect(storagePagePresentationSource).toContain('export const shouldShowCephSummaryCard');
    expect(storagePagePresentationSource).toContain('export const getStoragePageBannerMessage');
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
    expect(storageControlsSource).not.toContain(
      'props.setSelectedNodeId(event.currentTarget.value)',
    );
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
    expect(storageFilterSource).not.toContain(
      "props.sortDirection() === 'asc' ? 'rotate-180' : ''",
    );
    expect(storageFilterSource).not.toContain('focus:ring-blue-500');
    expect(useStorageFilterToolbarModelSource).toContain('countActiveStorageFilters');
    expect(useStorageFilterToolbarModelSource).toContain('hasActiveStorageFilters');
    expect(useStorageFilterToolbarModelSource).toContain('DEFAULT_STORAGE_SORT_KEY');
    expect(useStorageFilterToolbarModelSource).toContain('DEFAULT_STORAGE_SOURCE_FILTER');
    expect(useStorageFilterToolbarModelSource).toContain('getStorageSortDirectionTitle');
    expect(useStorageFilterToolbarModelSource).toContain('getStorageSortDirectionIconClass');
    expect(useStorageFilterToolbarModelSource).toContain('getNextStorageSortDirection');
    expect(storageFilterPresentationSource).toContain('export const getStorageSortDirectionTitle');
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
    expect(storagePageControlsSource).not.toContain(
      "props.view() === 'pools' ? props.storageFilterGroupBy : undefined",
    );
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
    expect(useStoragePageBannersModelSource).toContain(
      "kind === 'reconnecting' || kind === 'disconnected'",
    );
    expect(storagePageSummarySource).toContain('export const StoragePageSummary');
    expect(storagePageSummarySource).toContain('useStoragePageSummary');
    expect(storagePageSummarySource).toContain('StorageSummary');
    expect(storageCephSummaryCardSource).toContain('useStorageCephSummaryCardModel');
    expect(storageCephSummaryCardSource).toContain('CEPH_SUMMARY_CARD_GRID_CLASS');
    expect(storageCephSummaryCardSource).toContain('CEPH_SUMMARY_CARD_HEALTH_BADGE_CLASS');
    expect(storageCephSummaryCardSource).not.toContain('const summary = () => props.summary');
    expect(storageCephSummaryCardSource).not.toContain(
      'rounded-md border border-border bg-surface p-3',
    );
    expect(storageCephSummaryCardSource).not.toContain(
      'text-[11px] text-muted truncate max-w-[240px]',
    );
    expect(storageCephSummaryCardSource).not.toContain(
      'flex flex-wrap items-center justify-between gap-3',
    );
    expect(useStorageCephSummaryCardModelSource).toContain('getCephSummaryHeaderPresentation');
    expect(useStorageCephSummaryCardModelSource).toContain('getCephSummaryClusterCards');
    expect(storageContentCardSource).toContain('export const StorageContentCard');
    expect(storageContentCardSource).toContain('useStorageContentCardModel');
    expect(storageContentCardSource).toContain('DiskList');
    expect(storageContentCardSource).toContain('StoragePoolsTable');
    expect(storageContentCardSource).toContain('STORAGE_CONTENT_CARD_HEADER_CLASS');
    expect(storageContentCardSource).not.toContain(
      "props.selectedNodeId() === 'all' ? null : props.selectedNodeId()",
    );
    expect(storageContentCardSource).not.toContain(
      'border-b border-border bg-surface-hover px-3 py-2',
    );
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
    expect(storagePoolsTablePresentationSource).toContain('getStorageRowAlertPresentation');
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
    expect(storagePageSource).not.toContain('buildPath: buildStoragePath');
    expect(storagePageSource).not.toContain('const storageFilterGroupBy =');
    expect(storagePageSource).not.toContain('const storageFilterStatus =');
    expect(storagePageSource).not.toContain('const setStorageFilterStatus =');
    expect(storagePageSource).not.toContain('const records = createMemo(() => buildStorageRecords');
    expect(storagePageSource).not.toContain('const { byType } = useResources()');
    expect(storagePageSource).not.toContain(
      'const storageRecoveryResources = useStorageRecoveryResources()',
    );
    expect(storagePageSource).not.toContain('useStoragePageFilters({');
    expect(storagePageSource).not.toContain('useStoragePageData({');
    expect(storagePageSource).not.toContain('useStoragePageResources()');
    expect(storagePageSource).not.toContain('fields: {');
    expect(storagePageSource).not.toContain(
      'rounded border border-amber-300 bg-amber-100 px-2 py-1',
    );
    expect(storagePageSource).not.toContain('flex flex-wrap items-center justify-between gap-3');
    expect(storagePageSource).not.toContain('<Table class="w-full text-xs">');
    expect(storagePageSource).not.toContain('<StoragePageBanner kind=');
    expect(storagePageSource).not.toContain('<StorageCephSummaryCard summary=');
    expect(storageRowAlertPresentationSource).toContain(
      'export const getStorageRowAlertPresentation',
    );
    expect(diskLiveMetricPresentationSource).toContain('export const getDiskLiveMetricTextClass');
    expect(diskLiveMetricPresentationSource).toContain(
      'export const getDiskLiveMetricFormattedValue',
    );
    expect(diskPresentationSource).toContain('export function extractPhysicalDiskPresentationData');
    expect(diskPresentationSource).toContain('export function matchesPhysicalDiskSearch');
    expect(diskPresentationSource).toContain('export function comparePhysicalDiskPresentation');
    expect(diskPresentationSource).toContain(
      'export function buildPhysicalDiskPresentationDataMap',
    );
    expect(diskPresentationSource).toContain('export function filterAndSortPhysicalDisks');
    expect(diskDetailPresentationSource).toContain('export function getDiskDetailAttributeCards');
    expect(diskDetailPresentationSource).toContain('export function getDiskDetailHistoryCharts');
    expect(diskDetailPresentationSource).toContain(
      'export const DISK_DETAIL_HISTORY_RANGE_OPTIONS',
    );
    expect(diskDetailPresentationSource).toContain('export const DISK_DETAIL_LIVE_CHARTS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_CARD_CLASS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_SELECT_CLASS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_EMPTY_CLASS');
    expect(storageDetailPresentationSource).toContain('export const STORAGE_DETAIL_ROW_CLASS');
    expect(storageDetailPresentationSource).toContain(
      'export const STORAGE_DISK_DETAIL_HEADER_CLASS',
    );
    expect(storageDetailPresentationSource).toContain(
      'export const STORAGE_DETAIL_FULL_WIDTH_ROW_CLASS',
    );
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
    expect(zfsHealthMapPresentationSource).toContain('export const getZfsHealthMapDeviceClass');
    expect(zfsHealthMapPresentationSource).toContain(
      'export const getZfsHealthMapErrorSummaryClass',
    );
    expect(zfsHealthMapPresentationSource).toContain('export const getZfsHealthMapMessageClass');
    expect(zfsHealthMapPresentationSource).toContain(
      'export const ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS',
    );
    expect(useZFSHealthMapModelSource).toContain('getZfsHealthMapDevices');
    expect(useZFSHealthMapModelSource).toContain('isZfsHealthMapDeviceResilvering');
    expect(useZFSHealthMapModelSource).toContain('getZfsHealthMapTooltipPresentation');
    expect(zfsHealthMapSource).toContain('ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS');
    expect(zfsHealthMapSource).not.toContain('fixed z-[9999] pointer-events-none');
    expect(zfsHealthMapSource).not.toContain(
      'bg-surface text-base-content text-[10px] rounded-md shadow-sm px-2 py-1.5 min-w-[120px] border border-border',
    );
    expect(cephSummaryCardPresentationSource).toContain(
      'export const getCephSummaryHeaderPresentation',
    );
    expect(cephSummaryCardPresentationSource).toContain('export const getCephSummaryClusterCards');
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
    expect(storageAdaptersSource).toContain("from './resourceStoragePresentation'");
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
    expect(diskDetailPresentationSource).toContain('export function getLinkedDiskHealthDotClass');
    expect(diskDetailPresentationSource).toContain(
      'export function getLinkedDiskTemperatureTextClass',
    );
    expect(storagePoolDetailPresentationSource).toContain('export function getZfsScanTextClass');
    expect(storagePoolDetailPresentationSource).toContain('export function getZfsErrorTextClass');
    expect(storagePoolDetailPresentationSource).toContain('export function getZfsErrorSummary');
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
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordPlatformLabel');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordHostLabel');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordTopologyLabel');
    expect(storageRecordPresentationSource).toContain(
      'export const getStorageRecordProtectionLabel',
    );
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordIssueLabel');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordIssueSummary');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordImpactSummary');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordActionSummary');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordShared');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordNodeLabel');
    expect(storageRecordPresentationSource).toContain('export const getStorageRecordUsagePercent');
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
    expect(resourceStorageMappingSource).toContain('export const resolveResourceStorageContent');
    expect(resourceStorageMappingSource).toContain(
      'export const getStorageCapabilitiesForResource',
    );
    expect(resourceStorageMappingSource).toContain('export const getStorageCategoryFromType');
    expect(storageAdapterCoreSource).toContain('export const asNumberOrNull');
    expect(storageAdapterCoreSource).toContain('export const dedupe');
    expect(storageAdapterCoreSource).toContain('export const getStringArray');
    expect(storageAdapterCoreSource).toContain('export const canonicalStorageIdentityKey');
    expect(storageAdapterCoreSource).toContain('export const buildStorageSource');
    expect(storageAdapterCoreSource).toContain('export const buildStorageCapacity');
    expect(storageAdapterCoreSource).toContain('export const metricsTargetForStorageResource');
    expect(storageAdapterCoreSource).toContain('export const normalizeStorageResourceHealth');
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
    expect(k8sDeploymentsDrawerSource).not.toContain('label="Namespace"');
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
    expect(k8sNamespacePresentationSource).toContain('export function getK8sNamespacesEmptyState');
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
    expect(proLicensePanelSource).toContain('./useProLicensePanelState');
    expect(proLicensePanelSource).toContain('./ProLicensePlanSection');
    expect(proLicensePanelStateSource).toContain('getLicenseSubscriptionStatusPresentation');
    expect(proLicensePanelStateSource).toContain('formatLicensePlanVersion');
    expect(proLicensePanelStateSource).toContain('getTrialActivationNotice');
    expect(proLicensePanelStateSource).toContain('getCommercialMigrationNotice');
    expect(proLicensePlanSectionSource).toContain('getLicenseStatusLoadingState');
    expect(proLicensePlanSectionSource).toContain('getNoActiveProLicenseState');
    expect(proLicensePanelSource).not.toContain('Loading license status...');
    expect(proLicensePanelSource).not.toContain('No Pro license is active.');
    expect(proLicensePanelSource).not.toContain('const statusLabel =');
    expect(proLicensePanelStateSource).not.toContain('const statusTone =');
    expect(proLicensePanelStateSource).not.toContain('const formatTitleCase =');
    expect(proLicensePanelStateSource).not.toContain('const commercialMigrationActionText =');
    expect(proLicensePanelStateSource).not.toContain('const commercialMigrationNoticeFor =');
    expect(licensePresentationSource).toContain(
      'export const getLicenseSubscriptionStatusPresentation',
    );
    expect(licensePresentationSource).toContain('export const getLicenseStatusLoadingState');
    expect(licensePresentationSource).toContain('export const getNoActiveProLicenseState');
    expect(licensePresentationSource).toContain(
      'export const getOrganizationBillingLicenseStatusLabel',
    );
    expect(licensePresentationSource).toContain('export const getBillingAdminTrialStatus');
    expect(licensePresentationSource).toContain('export const getBillingAdminOrganizationBadges');
    expect(licensePresentationSource).toContain(
      'export const getBillingAdminStateUpdateSuccessMessage',
    );
    expect(licensePresentationSource).toContain('export const BILLING_ADMIN_EMPTY_STATE');
    expect(licensePresentationSource).toContain('export const formatLicensePlanVersion');
    expect(licensePresentationSource).toContain('titleCaseDelimitedLabel');
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
    expect(securityWarningSource).not.toContain(
      "status()!.credentialsEncrypted ? 'text-green-600' : 'text-red-600'",
    );
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
    expect(aiFindingPresentationSource).toContain('export const getFindingStatusBadgeClasses');
    expect(aiFindingPresentationSource).toContain('export const getFindingStatusLabel');
    expect(aiFindingPresentationSource).toContain('export const getFindingSeveritySortOrder');
    expect(aiFindingPresentationSource).toContain('export const getFindingSeverityCompactLabel');
    expect(aiFindingPresentationSource).toContain(
      'export const getInvestigationOutcomeBadgeClasses',
    );
    expect(aiFindingPresentationSource).toContain('export const getInvestigationOutcomeLabel');
    expect(aiFindingPresentationSource).toContain('export const getInvestigationStatusLabel');
    expect(aiFindingPresentationSource).toContain('export const getInvestigationOutcomeSortOrder');
    expect(aiFindingPresentationSource).toContain('export const hasFindingInvestigationDetails');
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
    expect(securityScorePresentationSource).toContain('export function getSecurityPostureItems');
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
    expect(securityAuthPanelSource).not.toContain('Security Configured - Restart Required');
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
    expect(environmentLockPresentationSource).toContain('export function getEnvironmentLockTitle');
    expect(environmentLockPresentationSource).toContain(
      'export const ENVIRONMENT_LOCK_BUTTON_TITLE',
    );
    expect(resourceDetailDrawerSource).toContain('getServiceHealthPresentation');
    expect(resourceDetailDrawerHistoryStateSource).toContain('ResourceAPI.getFacetBundle');
    expect(resourceDetailDrawerSource).toContain('History');
    expect(resourceDetailDrawerSource).toContain('RESOURCE_CHANGE_KIND_ORDER');
    expect(resourceDetailDrawerSource).toContain('RESOURCE_CHANGE_SOURCE_TYPE_ORDER');
    expect(resourceDetailDrawerSource).toContain('RESOURCE_CHANGE_SOURCE_ADAPTER_ORDER');
    expect(resourceDetailDrawerSource).toContain('getResourceChangeKindPresentation');
    expect(resourceDetailDrawerSource).toContain('getResourceChangeSourceTypePresentation');
    expect(resourceDetailDrawerSource).toContain('getResourceChangeSourceAdapterPresentation');
    expect(resourceChangeSummarySource).toContain('getResourceChangeKindPresentation');
    expect(resourceChangeSummarySource).toContain('getResourceChangeSourceTypePresentation');
    expect(resourceChangeSummarySource).toContain('getResourceChangeSourceAdapterPresentation');
    expect(resourceDetailDrawerSource).not.toContain('healthToneClass(');
    expect(resourceDetailDrawerSource).not.toContain('normalizeHealthLabel(');
    expect(resourceChangePresentationSource).toContain('getResourceChangeKindPresentation');
    expect(resourceChangePresentationSource).toContain('getResourceChangeSourceTypePresentation');
    expect(resourceChangePresentationSource).toContain(
      'getResourceChangeSourceAdapterPresentation',
    );
    expect(resourceCorrelationPresentationSource).toContain('formatConfidencePercentage');
    expect(resourceCorrelationPresentationSource).toContain('humanizeArrowDelimitedLabel');
    expect(resourceCorrelationPresentationSource).toContain('asTrimmedString');
    expect(confidencePresentationSource).toContain('formatConfidencePercentage');
    expect(confidencePresentationSource).toContain('formatConfidenceLabel');
    expect(approvalPresentationSource).toContain('getResourceApprovalLevelLabel');
    expect(throughputPresentationSource).toContain('formatThroughputRate');
    expect(resourceDetailDrawerSource).toContain('formatIdentifierLabel');
    expect(resourceDetailDrawerDerivedStateSource).toContain('formatIdentifierLabel');
    expect(resourceDetailDrawerServiceModelSource).toContain('buildPbsVisibleJobBreakdown');
    expect(resourceDetailDrawerServiceModelSource).toContain('buildPmgVisibleQueueBreakdown');
    expect(resourceDetailDrawerServiceModelSource).toContain('buildPmgVisibleMailBreakdown');
    expect(resourceChangePresentationSource).toContain('humanizeToken');
    expect(textPresentationSource).toContain('humanizeArrowDelimitedLabel');
    expect(resourceCorrelationPresentationSource).not.toContain(
      'formatResourceCorrelationEndpointLabel',
    );
    expect(resourceCorrelationPresentationSource).not.toContain("replace(/\\s*->\\s*/g, ' → ')");
    expect(textPresentationSource).toContain('humanizeToken');
    expect(textPresentationSource).toContain('formatIdentifierLabel');
    expect(resourceCorrelationPresentationSource).not.toContain('formatTrimmedLabel');
    expect(swarmServicesDrawerSource).toContain('asTrimmedString');
    expect(swarmServicesDrawerSource).not.toContain(
      "const normalize = (value?: string | null) => (value || '').trim();",
    );
    expect(pmgInstanceDrawerSource).toContain('asTrimmedString');
    expect(pmgInstanceDrawerSource).not.toContain(
      "const normalize = (value?: string | null) => (value || '').trim();",
    );
    expect(k8sNamespacesDrawerSource).toContain('asTrimmedString');
    expect(k8sNamespacesDrawerSource).not.toContain(
      "const normalize = (value?: string | null) => (value || '').trim();",
    );
    expect(k8sDeploymentsDrawerSource).toContain('asTrimmedString');
    expect(k8sDeploymentsDrawerSource).not.toContain(
      "const normalize = (value?: string | null) => (value || '').trim();",
    );
    expect(messageItemSource).toContain('formatIdentifierLabel');
    expect(toolExecutionBlockSource).toContain('formatIdentifierLabel');
    expect(aiChatSource).toContain('formatIdentifierLabel');
    expect(patrolStatusBarSource).toContain('formatTriggerReason');
    expect(findingsPanelSource).toContain('formatIdentifierLabel');
    expect(patrolFormatSource).toContain('formatIdentifierLabel');
    expect(aiFindingPresentationSource).toContain('formatIdentifierLabel');
    expect(messageItemSource).not.toContain("replace(/^pulse_/, '').replace(/_/g, ' ')");
    expect(toolExecutionBlockSource).not.toContain("replace(/^pulse_/, '').replace(/_/g, ' ')");
    expect(aiChatSource).not.toContain("replace(/^pulse_/, '').replace(/_/g, ' ')");
    expect(findingsPanelSource).not.toContain("replace(/_/g, ' ')");
    expect(patrolStatusBarSource).not.toContain("replace(/_/g, ' ') : ''");
    expect(patrolFormatSource).not.toContain("replace(/_/g, ' ') : 'Unknown'");
    expect(aiFindingPresentationSource).not.toContain("replace(/_/g, ' ')");
    expect(patrolRunPresentationSource).toContain('formatIdentifierLabel');
    expect(patrolRunPresentationSource).not.toContain("normalized.replace(/_/g, ' ')");
    expect(patrolRunPresentationSource).not.toContain(
      "normalized ? normalized.replace(/_/g, ' ') : 'unknown'",
    );
    expect(useChatSource).toContain('normalizeChatToolName');
    expect(useChatSource).not.toContain("replace(/^(pulse_)+/, '')");
    expect(aiChatSource).toContain('normalizeChatMentionKeyPart');
    expect(aiChatSource).not.toContain('const normalizeMentionKeyPart =');
    expect(chatIdentifiersSource).toContain('normalizeChatMentionKeyPart');
    expect(chatIdentifiersSource).toContain('normalizeChatToolName');
    expect(infrastructureSummaryModelSource).toContain('getNormalizedIdentityLookupVariants');
    expect(sharedInfrastructureSummaryTableModelSource).toContain(
      'getNormalizedIdentityLookupVariants',
    );
    expect(resourceIdentitySource).toContain('getNormalizedIdentityLookupVariants');
    expect(stringUtilsSource).toContain('export const asTrimmedString');
    expect(resourceIdentitySource).not.toContain(
      'const asTrimmedString = (value: unknown): string | undefined => {',
    );
    expect(commandPaletteModalSource).toContain('useCommandPaletteState');
    expect(commandPaletteModalSource).not.toContain('useNavigate');
    expect(commandPaletteModalSource).not.toContain('createSignal');
    expect(commandPaletteModalSource).not.toContain('buildInfrastructurePath');
    expect(commandPaletteStateSource).toContain('useNavigate');
    expect(commandPaletteStateSource).toContain('createSignal');
    expect(commandPaletteStateSource).toContain('buildInfrastructurePath');
    expect(commandPaletteModelSource).toContain('buildCommandPaletteCommands');
    expect(commandPaletteModelSource).toContain('normalizeCommandPaletteQuery');
    expect(commandPaletteModelSource).toContain('filterCommandPaletteCommands');
    expect(pulseDataGridSource).toContain('usePulseDataGridState');
    expect(pulseDataGridSource).toContain('getPulseDataGridAlignClass');
    expect(pulseDataGridSource).toContain('isPulseDataGridInteractiveTarget');
    expect(pulseDataGridSource).not.toContain('useBreakpoint');
    expect(pulseDataGridSource).not.toContain('createStore');
    expect(pulseDataGridSource).not.toContain('target.closest(');
    expect(pulseDataGridStateSource).toContain('useBreakpoint');
    expect(pulseDataGridStateSource).toContain('createStore');
    expect(pulseDataGridStateSource).toContain('reconcile(');
    expect(pulseDataGridModelSource).toContain('getPulseDataGridAlignClass');
    expect(pulseDataGridModelSource).toContain('isPulseDataGridInteractiveTarget');
    expect(pulseDataGridModelSource).toContain('target.closest(');
    expect(searchFieldSource).toContain('useSearchFieldState');
    expect(searchFieldSource).not.toContain('let inputEl: HTMLInputElement');
    expect(searchFieldSource).not.toContain("if (props.hasTrailingControls) return 'pr-14 sm:pr-20'");
    expect(searchFieldSource).not.toContain("if (e.key === 'Escape'");
    expect(searchFieldStateSource).toContain('let inputEl: HTMLInputElement');
    expect(searchFieldStateSource).toContain("if (event.key === 'Escape'");
    expect(searchFieldStateSource).toContain('inputEl?.blur()');
    expect(searchFieldModelSource).toContain('shouldShowSearchFieldShortcutHint');
    expect(searchFieldModelSource).toContain('shouldShowSearchFieldClearButton');
    expect(searchFieldModelSource).toContain('getSearchFieldInputPaddingRightClass');
    expect(searchInputSource).toContain('useSearchInputState');
    expect(searchInputSource).not.toContain('let searchInputEl: HTMLInputElement');
    expect(searchInputSource).not.toContain('useTypeToSearch');
    expect(searchInputSource).not.toContain('useSearchInputEnhancements');
    expect(searchInputStateSource).toContain('let searchInputEl: HTMLInputElement');
    expect(searchInputStateSource).toContain('useTypeToSearch');
    expect(searchInputStateSource).toContain('useSearchInputEnhancements');
    expect(searchInputStateSource).toContain('getSearchInputShortcutHint');
    expect(searchInputModelSource).toContain('getSearchInputShortcutHint');
    expect(searchInputModelSource).toContain('shouldSearchInputShowTrailingControls');
    expect(searchInputEnhancementsSource).toContain('getSearchHistoryToggleButtonClass');
    expect(searchInputEnhancementsSource).toContain('getSearchHistoryToggleTitle');
    expect(searchInputEnhancementsSource).toContain('SEARCH_HISTORY_CLEAR_LABEL');
    expect(searchInputEnhancementsSource).not.toContain('Show recent searches');
    expect(searchInputEnhancementsSource).not.toContain('No recent searches yet');
    expect(searchInputEnhancementsSource).not.toContain('Clear history');
    expect(searchInputEnhancementsSource).not.toContain('hover:bg-blue-50');
    expect(searchInputEnhancementsStateSource).toContain('createSearchHistoryManager');
    expect(searchInputEnhancementsStateSource).not.toContain('Show recent searches');
    expect(searchInputEnhancementsStateSource).toContain(
      "options.history?.emptyMessage ?? 'Searches you run will appear here.'",
    );
    expect(searchInputEnhancementsModelSource).toContain('getSearchHistoryToggleButtonClass');
    expect(searchInputEnhancementsModelSource).toContain('getSearchHistoryToggleTitle');
    expect(searchInputEnhancementsModelSource).toContain('SEARCH_HISTORY_CLEAR_LABEL');
    expect(searchInputEnhancementsModelSource).toContain('SEARCH_HISTORY_MENU_CLASS');
    expect(searchTipsPopoverSource).toContain('useSearchTipsPopoverState');
    expect(searchTipsPopoverSource).toContain('getSearchTipsPopoverTriggerClass');
    expect(searchTipsPopoverSource).not.toContain('createSignal');
    expect(searchTipsPopoverSource).not.toContain('createEffect');
    expect(searchTipsPopoverSource).not.toContain('window.addEventListener');
    expect(searchTipsPopoverStateSource).toContain('createSignal');
    expect(searchTipsPopoverStateSource).toContain('createEffect');
    expect(searchTipsPopoverStateSource).toContain('window.addEventListener');
    expect(searchTipsPopoverStateSource).toContain('pointerInside');
    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverTriggerClass');
    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverPositionClass');
    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverTriggerVariant');
    expect(searchTipsPopoverModelSource).toContain('shouldSearchTipsPopoverOpenOnHover');
    expect(activeUseTrialNudgeSource).toContain('useActiveUseTrialNudgeState');
    expect(activeUseTrialNudgeSource).toContain('ACTIVE_USE_TRIAL_NUDGE_TITLE');
    expect(activeUseTrialNudgeSource).not.toContain('createSignal');
    expect(activeUseTrialNudgeSource).not.toContain('createMemo');
    expect(activeUseTrialNudgeSource).not.toContain('startProTrial');
    expect(activeUseTrialNudgeSource).not.toContain('localStorage');
    expect(activeUseTrialNudgeSource).not.toContain('setInterval');
    expect(activeUseTrialNudgeStateSource).toContain('createSignal');
    expect(activeUseTrialNudgeStateSource).toContain('createMemo');
    expect(activeUseTrialNudgeStateSource).toContain('window.localStorage');
    expect(activeUseTrialNudgeStateSource).toContain('setInterval');
    expect(activeUseTrialNudgeStateSource).toContain('startProTrial');
    expect(activeUseTrialNudgeStateSource).toContain('snoozeUpsell');
    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY');
    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY');
    expect(activeUseTrialNudgeModelSource).toContain('isActiveUseTrialNudgeEligible');
    expect(activeUseTrialNudgeModelSource).toContain('isActiveUseTrialNudgeOldEnough');
    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_TITLE');
    expect(trialBannerSource).toContain('useTrialBannerState');
    expect(trialBannerSource).toContain('TRIAL_BANNER_TITLE');
    expect(trialBannerSource).not.toContain('createSignal');
    expect(trialBannerSource).not.toContain('createMemo');
    expect(trialBannerSource).not.toContain('loadLicenseStatus');
    expect(trialBannerSource).not.toContain('licenseStatus');
    expect(trialBannerSource).not.toContain('getUpgradeActionUrlOrFallback');
    expect(trialBannerStateSource).toContain('createSignal');
    expect(trialBannerStateSource).toContain('createMemo');
    expect(trialBannerStateSource).toContain('loadLicenseStatus');
    expect(trialBannerStateSource).toContain('licenseStatus');
    expect(trialBannerStateSource).toContain('getUpgradeActionUrlOrFallback');
    expect(trialBannerStateSource).toContain('snoozeUpsell');
    expect(trialBannerModelSource).toContain('TRIAL_BANNER_SNOOZE_KEY');
    expect(trialBannerModelSource).toContain('normalizeTrialBannerDaysRemaining');
    expect(trialBannerModelSource).toContain('getTrialBannerToneClass');
    expect(trialBannerModelSource).toContain('getTrialBannerStatusLabel');
    expect(trialBannerModelSource).toContain('TRIAL_BANNER_UPGRADE_LABEL');
    expect(columnPickerSource).toContain('useColumnPickerState');
    expect(columnPickerSource).toContain('COLUMN_PICKER_PANEL_TITLE');
    expect(columnPickerSource).not.toContain('createSignal');
    expect(columnPickerSource).not.toContain('createEffect');
    expect(columnPickerSource).not.toContain('document.addEventListener');
    expect(columnPickerSource).not.toContain('getHiddenColumnCount');
    expect(columnPickerStateSource).toContain('createSignal');
    expect(columnPickerStateSource).toContain('createEffect');
    expect(columnPickerStateSource).toContain('document.addEventListener');
    expect(columnPickerStateSource).toContain('handleClickOutside');
    expect(columnPickerStateSource).toContain('hiddenCount');
    expect(columnPickerModelSource).toContain('COLUMN_PICKER_BUTTON_LABEL');
    expect(columnPickerModelSource).toContain('COLUMN_PICKER_PANEL_TITLE');
    expect(columnPickerModelSource).toContain('getHiddenColumnCount');
    expect(columnPickerModelSource).toContain('shouldShowColumnPickerReset');
    expect(columnPickerModelSource).toContain('getColumnPickerOptionTextClass');
    expect(tagInputSource).toContain('useTagInputState');
    expect(tagInputSource).toContain('getTagInputPlaceholder');
    expect(tagInputSource).not.toContain('createSignal');
    expect(tagInputSource).not.toContain('querySelector');
    expect(tagInputSource).not.toContain('Backspace');
    expect(tagInputSource).not.toContain('addTag');
    expect(tagInputStateSource).toContain('createSignal');
    expect(tagInputStateSource).toContain('createMemo');
    expect(tagInputStateSource).toContain('inputRef?.focus');
    expect(tagInputStateSource).toContain("event.key === 'Backspace'");
    expect(tagInputStateSource).toContain('commitTag');
    expect(tagInputModelSource).toContain('TAG_INPUT_DELIMITER_KEYS');
    expect(tagInputModelSource).toContain('isTagInputCommitKey');
    expect(tagInputModelSource).toContain('getTagInputPlaceholder');
    expect(tagInputModelSource).toContain('getNextTagsAfterRemove');
    expect(tagInputModelSource).toContain('getTagInputRemoveTitle');
    expect(scrollToTopButtonSource).toContain('useScrollToTopButtonState');
    expect(scrollToTopButtonSource).toContain('SCROLL_TO_TOP_BUTTON_ARIA_LABEL');
    expect(scrollToTopButtonSource).toContain('getScrollToTopButtonClass');
    expect(scrollToTopButtonSource).not.toContain('createSignal');
    expect(scrollToTopButtonSource).not.toContain('onMount');
    expect(scrollToTopButtonSource).not.toContain('scrollHeight');
    expect(scrollToTopButtonSource).not.toContain('SCROLL_THRESHOLD');
    expect(scrollToTopButtonStateSource).toContain('createSignal');
    expect(scrollToTopButtonStateSource).toContain('onMount');
    expect(scrollToTopButtonStateSource).toContain('addEventListener');
    expect(scrollToTopButtonStateSource).toContain("scrollTo({ top: 0, behavior: 'smooth' })");
    expect(scrollToTopButtonStateSource).toContain('findNearestScrollableAncestor');
    expect(scrollToTopButtonModelSource).toContain('SCROLL_TO_TOP_BUTTON_THRESHOLD');
    expect(scrollToTopButtonModelSource).toContain('SCROLL_TO_TOP_BUTTON_ARIA_LABEL');
    expect(scrollToTopButtonModelSource).toContain('findNearestScrollableAncestor');
    expect(scrollToTopButtonModelSource).toContain('isScrollToTopButtonVisible');
    expect(scrollToTopButtonModelSource).toContain('getScrollToTopButtonClass');
    expect(filterButtonGroupSource).toContain('useFilterButtonGroupState');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupClass');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupButtonClass');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupCompactLabel');
    expect(filterButtonGroupSource).not.toContain("label.split(' ').pop()");
    expect(filterButtonGroupSource).not.toContain('props.onChange(option.value)');
    expect(filterButtonGroupStateSource).toContain('createMemo');
    expect(filterButtonGroupStateSource).toContain('props.disabled || option.disabled');
    expect(filterButtonGroupStateSource).toContain('props.onChange(option.value)');
    expect(filterButtonGroupModelSource).toContain('resolveFilterButtonGroupVariant');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupClass');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupButtonClass');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupCompactLabel');
    expect(filterButtonGroupModelSource).toContain("prominent: 'grid grid-cols-1 gap-2'");
    expect(toggleSource).toContain('useToggleState');
    expect(toggleSource).toContain('getToggleTrackClass');
    expect(toggleSource).toContain('getToggleKnobClass');
    expect(toggleSource).not.toContain('defaultPrevented');
    expect(toggleSource).not.toContain('toggleSizeConfig');
    expect(toggleSource).not.toContain('handleClick =');
    expect(toggleStateSource).toContain('defaultPrevented');
    expect(toggleStateSource).toContain('currentTarget: { checked: next }');
    expect(toggleStateSource).toContain('props.onChange?.(event)');
    expect(toggleStateSource).toContain('props.onToggle?.()');
    expect(toggleModelSource).toContain('toggleSizeConfig');
    expect(toggleModelSource).toContain('resolveToggleSize');
    expect(toggleModelSource).toContain('getToggleTrackClass');
    expect(toggleModelSource).toContain('getToggleKnobClass');
    expect(toggleModelSource).toContain('ToggleChangeEvent');
    expect(statusBadgeSource).toContain('useStatusBadgeState');
    expect(statusBadgeSource).toContain('getStatusBadgeClass');
    expect(statusBadgeSource).toContain('getStatusBadgeLabel');
    expect(statusBadgeSource).toContain('getStatusBadgeTitle');
    expect(statusBadgeSource).not.toContain('cursor-not-allowed');
    expect(statusBadgeSource).not.toContain('props.onToggle?.()');
    expect(statusBadgeSource).not.toContain('labelEnabled ??');
    expect(statusBadgeStateSource).toContain('Boolean(props.disabled)');
    expect(statusBadgeStateSource).toContain('props.onToggle?.()');
    expect(statusBadgeStateSource).toContain('if (isDisabled())');
    expect(statusBadgeModelSource).toContain('STATUS_BADGE_PADDING_BY_SIZE');
    expect(statusBadgeModelSource).toContain('getStatusBadgeClass');
    expect(statusBadgeModelSource).toContain('getStatusBadgeLabel');
    expect(statusBadgeModelSource).toContain('getStatusBadgeTitle');
    expect(statusBadgeModelSource).toContain("labelEnabled ?? 'Enabled'");
    expect(selectionCardGroupSource).toContain('useSelectionCardGroupState');
    expect(selectionCardGroupSource).toContain('getSelectionCardGroupClass');
    expect(selectionCardGroupSource).toContain('getSelectionCardButtonClass');
    expect(selectionCardGroupSource).toContain('getSelectionCardTitleClass');
    expect(selectionCardGroupSource).not.toContain('resolveSelectionCardTone');
    expect(selectionCardGroupSource).not.toContain('props.onChange(option.value)');
    expect(selectionCardGroupStateSource).toContain('createMemo');
    expect(selectionCardGroupStateSource).toContain('resolveSelectionCardTone');
    expect(selectionCardGroupStateSource).toContain('props.disabled || option.disabled');
    expect(selectionCardGroupStateSource).toContain('props.onChange(option.value)');
    expect(selectionCardGroupModelSource).toContain('resolveSelectionCardGroupVariant');
    expect(selectionCardGroupModelSource).toContain('resolveSelectionCardTone');
    expect(selectionCardGroupModelSource).toContain('getSelectionCardButtonClass');
    expect(selectionCardGroupModelSource).toContain("compact: 'grid grid-cols-2 gap-2'");
    expect(monitoredSystemLimitWarningBannerSource).toContain(
      'useMonitoredSystemLimitWarningBannerState',
    );
    expect(monitoredSystemLimitWarningBannerSource).toContain(
      'MONITORED_SYSTEM_LIMIT_LEARN_MORE_LABEL',
    );
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('loadLicenseStatus');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('legacyConnections()');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('loadLicenseStatus');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('legacyConnections');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('handleUpgradeClick');
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemMigrationMessage',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemBannerToneClass',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'MONITORED_SYSTEM_LIMIT_UPGRADE_LABEL',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_LABEL',
    );
    expect(whatsNewModalSource).toContain('useWhatsNewModalState');
    expect(whatsNewModalSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalSource).not.toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalSource).not.toContain('createSignal');
    expect(whatsNewModalSource).not.toContain('WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalSource).not.toContain('Documentation');
    expect(whatsNewModalSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md',
    );
    expect(whatsNewModalStateSource).toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalStateSource).toContain('createSignal');
    expect(whatsNewModalStateSource).toContain('STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalStateSource).toContain('handleClose');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_TELEMETRY_TITLE');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_PRIVACY_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_LABEL');
    expect(tooltipSource).toContain('useTooltipState');
    expect(tooltipSource).toContain('createTooltipSystemState');
    expect(tooltipSource).not.toContain('createSignal');
    expect(tooltipSource).not.toContain('requestAnimationFrame');
    expect(tooltipSource).not.toContain('sanitizeTooltipContent');
    expect(tooltipSource).not.toContain('resolveTooltipPosition');
    expect(tooltipStateSource).toContain('createSignal');
    expect(tooltipStateSource).toContain('requestAnimationFrame');
    expect(tooltipStateSource).toContain('tooltipInstance');
    expect(tooltipStateSource).toContain('resolveTooltipPosition');
    expect(tooltipModelSource).toContain('sanitizeTooltipContent');
    expect(tooltipModelSource).toContain('resolveTooltipPosition');
    expect(dialogSource).toContain('useDialogState');
    expect(dialogSource).toContain('getDialogViewportClass');
    expect(dialogSource).toContain('getDialogAlignmentClass');
    expect(dialogSource).toContain('getDialogPanelClass');
    expect(dialogSource).not.toContain('createEffect');
    expect(dialogSource).not.toContain('document.body.style.overflow');
    expect(dialogSource).not.toContain('FOCUSABLE_SELECTOR');
    expect(dialogStateSource).toContain('createEffect');
    expect(dialogStateSource).toContain('onCleanup');
    expect(dialogStateSource).toContain('document.body.style.overflow');
    expect(dialogStateSource).toContain('getDialogFocusableElements');
    expect(dialogModelSource).toContain('getDialogFocusableElements');
    expect(dialogModelSource).toContain('getDialogViewportClass');
    expect(dialogModelSource).toContain('getDialogAlignmentClass');
    expect(dialogModelSource).toContain('getDialogPanelClass');
    expect(collapsibleSearchInputSource).toContain('useCollapsibleSearchInputState');
    expect(collapsibleSearchInputSource).not.toContain('createSignal');
    expect(collapsibleSearchInputSource).not.toContain('useTypeToSearch');
    expect(collapsibleSearchInputSource).not.toContain(
      "const triggerLabel = () => props.triggerLabel ?? 'Search'",
    );
    expect(collapsibleSearchInputStateSource).toContain('createSignal');
    expect(collapsibleSearchInputStateSource).toContain('useTypeToSearch');
    expect(collapsibleSearchInputStateSource).toContain('queueMicrotask');
    expect(collapsibleSearchInputModelSource).toContain('getCollapsibleSearchTriggerLabel');
    expect(collapsibleSearchInputModelSource).toContain(
      'shouldShowCollapsibleSearchExpanded',
    );
    expect(collapsibleSearchInputModelSource).toContain('getCollapsibleSearchRootClass');
    expect(infrastructureSummaryModelSource).not.toContain(
      'const asTrimmedString = (value: unknown): string | null => {',
    );
    expect(sharedInfrastructureSummaryTableModelSource).not.toContain(
      'const asTrimmedString = (value: unknown): string | undefined => {',
    );
    expect(infrastructureSummaryTableSource).toContain('useInfrastructureSummaryTableState');
    expect(infrastructureSummaryTableSource).toContain('InfrastructureSummaryTableRow');
    expect(infrastructureSummaryTableSource).not.toContain('useWebSocket');
    expect(infrastructureSummaryTableSource).not.toContain('useAlertsActivation');
    expect(infrastructureSummaryTableSource).not.toContain('createSignal');
    expect(infrastructureSummaryTableSource).not.toContain('getAgentLikeIdentityAliases');
    expect(infrastructureSummaryTableStateSource).toContain('useWebSocket');
    expect(infrastructureSummaryTableStateSource).toContain('useAlertsActivation');
    expect(infrastructureSummaryTableStateSource).toContain(
      'export function useInfrastructureSummaryTableState',
    );
    expect(infrastructureSummaryTableRowSource).toContain('InfrastructureDetailsDrawer');
    expect(infrastructureSummaryTableRowSource).toContain('getAlertStyles');
    expect(sharedInfrastructureSummaryTableModelSource).toContain(
      'resolveInfrastructureSummaryLinkedAgent',
    );
    expect(infrastructureSelectorSource).toContain('useInfrastructureSelectorState');
    expect(infrastructureSelectorSource).toContain('InfrastructureSummaryTable');
    expect(infrastructureSelectorSource).not.toContain('useResources');
    expect(infrastructureSelectorSource).not.toContain('createSignal');
    expect(infrastructureSelectorSource).not.toContain("resource.type === 'truenas'");
    expect(infrastructureSelectorStateSource).toContain('useResources');
    expect(infrastructureSelectorStateSource).toContain('useRecoveryRollups');
    expect(infrastructureSelectorStateSource).toContain('createSignal');
    expect(infrastructureSelectorStateSource).toContain('document.addEventListener');
    expect(infrastructureSelectorModelSource).toContain('buildInfrastructureSelectorAgents');
    expect(infrastructureSelectorModelSource).toContain(
      'buildInfrastructureSelectorBackupCounts',
    );
    expect(infrastructureSelectorModelSource).toContain(
      'buildInfrastructureSelectorUnifiedNodes',
    );
    expect(infrastructureSelectorModelSource).toContain("resource.type === 'truenas'");
    expect(interactiveSparklineSource).toContain('useInteractiveSparklineState');
    expect(interactiveSparklineSource).not.toContain('scheduleSparkline');
    expect(interactiveSparklineSource).not.toContain('downsampleLTTB');
    expect(interactiveSparklineSource).not.toContain('createSignal');
    expect(interactiveSparklineStateSource).toContain('scheduleSparkline');
    expect(interactiveSparklineStateSource).toContain('createSignal');
    expect(interactiveSparklineModelSource).toContain('buildInteractiveSparklineChartData');
    expect(interactiveSparklineModelSource).toContain(
      'computeInteractiveSparklineHoverState',
    );
    expect(densityMapSource).toContain('useDensityMapState');
    expect(densityMapSource).not.toContain('timeRangeToMs');
    expect(densityMapSource).not.toContain('createSignal');
    expect(densityMapSource).not.toContain('ctx.fillRect');
    expect(densityMapStateSource).toContain('createSignal');
    expect(densityMapStateSource).toContain('canvas.getContext');
    expect(densityMapStateSource).toContain('window.addEventListener');
    expect(densityMapModelSource).toContain('buildDensityMapChartData');
    expect(densityMapModelSource).toContain('buildDensityMapHoveredState');
    expect(densityMapModelSource).toContain('formatDensityMapHoverTime');
    expect(densityMapModelSource).toContain('getDensityMapCellOpacity');
    expect(historyChartSource).toContain('useHistoryChartState');
    expect(historyChartSource).toContain('HistoryChartHeader');
    expect(historyChartSource).toContain('HistoryChartOverlay');
    expect(historyChartSource).toContain('HistoryChartTooltip');
    expect(historyChartSource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartSource).not.toContain('calculateOptimalPoints');
    expect(historyChartSource).not.toContain('setupCanvasDPR');
    expect(historyChartSource).not.toContain('Collecting data... History will appear here.');
    expect(historyChartSource).not.toContain('Unlock {chart.lockTierLabel()} Features');
    expect(historyChartStateSource).toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartStateSource).toContain('calculateOptimalPoints');
    expect(historyChartStateSource).toContain('setupCanvasDPR');
    expect(historyChartStateSource).toContain('HISTORY_CHART_RANGES');
    expect(historyChartModelSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartModelSource).toContain('HISTORY_CHART_RANGES');
    expect(historyChartModelSource).toContain('getHistoryChartScale');
    expect(historyChartHeaderSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartOverlaySource).toContain('Collecting data... History will appear here.');
    expect(historyChartOverlaySource).toContain('Unlock {props.chart.lockTierLabel()} Features');
    expect(historyChartTooltipSource).toContain('formatHistoryChartTooltipValue');
    expect(containerUpdateBadgeSource).toContain('useContainerUpdateButtonState');
    expect(containerUpdateBadgeSource).toContain('getUpdateButtonClass');
    expect(containerUpdateBadgeSource).not.toContain('MonitoringAPI.updateDockerContainer');
    expect(containerUpdateBadgeSource).not.toContain('markContainerQueued');
    expect(containerUpdateButtonStateSource).toContain('MonitoringAPI.updateDockerContainer');
    expect(containerUpdateButtonStateSource).toContain('markContainerQueued');
    expect(containerUpdateBadgeModelSource).toContain('getUpdateButtonClass');
    expect(containerUpdateBadgeModelSource).toContain('getUpdateButtonTooltip');
    expect(helpIconSource).toContain('useHelpIconState');
    expect(helpIconSource).not.toContain('getHelpContent(');
    expect(helpIconSource).not.toContain('requestAnimationFrame');
    expect(helpIconStateSource).toContain('requestAnimationFrame');
    expect(helpIconStateSource).toContain('document.addEventListener');
    expect(helpIconModelSource).toContain('resolveHelpContent');
    expect(helpIconModelSource).toContain('calculateHelpPopoverPosition');
    expect(mobileNavBarSource).toContain('useMobileNavBarState');
    expect(mobileNavBarSource).toContain('getMobileNavTabButtonClass');
    expect(mobileNavBarSource).not.toContain('createSignal');
    expect(mobileNavBarSource).not.toContain('requestAnimationFrame');
    expect(mobileNavBarSource).not.toContain('new Set(priority)');
    expect(mobileNavBarStateSource).toContain('createSignal');
    expect(mobileNavBarStateSource).toContain('window.addEventListener');
    expect(mobileNavBarStateSource).toContain('requestAnimationFrame');
    expect(mobileNavBarStateSource).toContain('scrollIntoView');
    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavPlatformTabs');
    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavUtilityTabs');
    expect(mobileNavBarModelSource).toContain('getMobileNavAlertBadgeCounts');
    expect(mobileNavBarModelSource).toContain('getMobileNavFadeState');
    expect(infrastructureDetailsDrawerSource).toContain('useInfrastructureDetailsDrawerState');
    expect(infrastructureDetailsDrawerSource).toContain(
      'resolveInfrastructureDetailsDrawerMetadataId',
    );
    expect(infrastructureDetailsDrawerSource).toContain(
      'resolveInfrastructureDetailsDrawerDiscoveryHostname',
    );
    expect(infrastructureDetailsDrawerSource).not.toContain('createSignal');
    expect(infrastructureDetailsDrawerSource).not.toContain('getInfrastructureMetadataId');
    expect(infrastructureDetailsDrawerSource).not.toContain(
      'getInfrastructureDiscoveryHostname',
    );
    expect(infrastructureDetailsDrawerStateSource).toContain('createSignal');
    expect(infrastructureDetailsDrawerStateSource).toContain(
      "type InfrastructureDetailsDrawerTab = 'overview' | 'discovery'",
    );
    expect(infrastructureDetailsDrawerModelSource).toContain(
      'resolveInfrastructureDetailsDrawerMetadataId',
    );
    expect(infrastructureDetailsDrawerModelSource).toContain(
      'resolveInfrastructureDetailsDrawerDiscoveryHostname',
    );
    expect(infrastructureDetailsDrawerModelSource).toContain('getInfrastructureMetadataId');
    expect(infrastructureDetailsDrawerModelSource).toContain(
      'getInfrastructureDiscoveryHostname',
    );
    expect(useUnifiedResourcesSource).not.toContain('normalizeResourcePolicyAISafeSummary(');
    expect(useUnifiedResourcesSource).not.toContain('normalizeResourcePolicy(');
    expect(useUnifiedResourcesSource).not.toContain('const resolvePolicySensitivity =');
    expect(useUnifiedResourcesSource).not.toContain('const resolvePolicyRoutingScope =');
    expect(useUnifiedResourcesSource).not.toContain('const resolvePolicyRedactionHints =');
    expect(useUnifiedResourcesSource).not.toContain('const resolvePolicy =');
    expect(resourceDetailMappersSource).toContain('titleCaseDelimitedLabel');
    expect(resourceDetailMappersSource).not.toContain('export const toDiscoveryConfig');
    expect(resourceDetailMappersSource).not.toContain('export const normalizeHealthLabel');
    expect(resourceDetailMappersSource).not.toContain('export const healthToneClass');
    expect(unifiedResourceTableSource).toContain('useUnifiedResourceTableState');
    expect(unifiedResourceTableSource).toContain('UnifiedResourceHostTableCard');
    expect(unifiedResourceTableSource).toContain('UnifiedResourceServiceInfrastructureCard');
    expect(unifiedResourceTableSource).not.toContain('const split = createMemo(() =>');
    expect(unifiedResourceTableSource).not.toContain('const sortedPBSResources = createMemo(() =>');
    expect(unifiedResourceTableSource).not.toContain('const resourceColumnStyle = createMemo(() =>');
    expect(unifiedResourceTableSource).not.toContain('getServiceHealthSummaryPresentation');
    expect(unifiedResourceTableSource).not.toContain('const getOutlierEmphasis =');
    expect(unifiedResourceTableSource).not.toContain('const summarizeServiceHealthTone =');
    expect(unifiedResourceTableStateSource).toContain('export function useUnifiedResourceTableState');
    expect(unifiedResourceTableStateSource).toContain("from './unifiedResourceTableStateModel'");
    expect(unifiedResourceTableStateSource).toContain('buildHostTableItems');
    expect(unifiedResourceTableStateSource).toContain('getUnifiedResourceTableColumnStyles');
    expect(unifiedResourceTableStateSource).toContain('useTableWindowing');
    expect(unifiedResourceTableStateSource).toContain('useUnifiedResourceTableViewportSync');
    expect(unifiedResourceTableStateSource).not.toContain('const resourceColumnStyle = createMemo(() =>');
    expect(unifiedResourceTableStateSource).not.toContain("const showGroupHeaders = props.groupingMode === 'grouped'");
    expect(unifiedResourceTableStateSource).not.toContain('const items: HostTableItem[] = [];');
    expect(unifiedResourceTableStateSource).not.toContain('window.addEventListener');
    expect(unifiedResourceTableStateSource).not.toContain('getBoundingClientRect');
    expect(unifiedResourceTableStateModelSource).toContain('export const buildHostTableItems');
    expect(unifiedResourceTableStateModelSource).toContain(
      'export const getUnifiedResourceTableColumnStyles',
    );
    expect(unifiedResourceTableStateModelSource).toContain(
      'export const getNextUnifiedResourceTableSortState',
    );
    expect(unifiedResourceTableModelSource).toContain('getServiceHealthSummaryPresentation');
    expect(unifiedResourceTableModelSource).toContain('export const getOutlierEmphasis');
    expect(unifiedResourceTableViewportSyncSource).toContain('window.addEventListener');
    expect(unifiedResourceTableViewportSyncSource).toContain('getBoundingClientRect');
    expect(unifiedResourceTableViewportSyncSource).toContain('scrollIntoView');
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
    expect(guestDrawerSource).toContain('useGuestDrawerState');
    expect(guestDrawerStateSource).toContain('getDiscoveryResourceTypeForWorkload');
    expect(guestDrawerStateSource).toContain('getWebInterfaceTargetLabelForWorkload');
    expect(guestDrawerStateSource).toContain('getDiscoveryLoadingState');
    expect(guestDrawerSource).not.toContain('const discoveryResourceType = () =>');
    expect(guestDrawerSource).not.toContain('const urlTargetLabel = () =>');
    expect(guestDrawerModelSource).toContain('export const getGuestDrawerBackupPresentation');
    expect(workloadsSource).toContain('export const getDiscoveryResourceTypeForWorkload');
    expect(workloadsSource).toContain('export const getWebInterfaceTargetLabelForWorkload');
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
    expect(webInterfaceUrlFieldSource).toContain('useWebInterfaceUrlFieldState');
    expect(webInterfaceUrlFieldSource).not.toContain('No suggested URL found');
    expect(webInterfaceUrlFieldSource).not.toContain('GuestMetadataAPI.getMetadata');
    expect(webInterfaceUrlFieldStateSource).toContain('GuestMetadataAPI.getMetadata');
    expect(webInterfaceUrlFieldStateSource).toContain('AgentMetadataAPI.updateMetadata');
    expect(webInterfaceUrlFieldModelSource).toContain('getWebInterfaceSuggestedUrlFallback');
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
    expect(proxmoxSettingsPanelSource).toContain('./ProxmoxDirectWorkspace');
    expect(proxmoxSettingsPanelSource).not.toContain('const VARIANT_CONFIG: Record<NodeType');
    expect(proxmoxSettingsPanelSource).not.toContain('const buildDiscoveryPrefillNode =');
    expect(proxmoxDirectWorkspaceSource).toContain('./useProxmoxDirectWorkspaceState');
    expect(proxmoxSettingsPanelSource).not.toContain('const openCreateNode = (type: NodeType) =>');
    expect(proxmoxSettingsPanelSource).not.toContain('const openDiscoveredNode = (server: DiscoveredServer) =>');
    expect(proxmoxConfiguredNodesTableSource).toContain('PveNodesTable');
    expect(proxmoxConfiguredNodesTableSource).toContain('PbsNodesTable');
    expect(proxmoxConfiguredNodesTableSource).toContain('PmgNodesTable');
    expect(proxmoxNodeModalStackSource).toContain('PROXMOX_NODE_TYPES');
    expect(proxmoxSettingsModelSource).toContain('export interface ProxmoxSettingsPanelProps');
    expect(proxmoxDirectWorkspaceStateSource).toContain(
      'export function useProxmoxDirectWorkspaceState',
    );
    expect(proxmoxDirectWorkspaceStateSource).toContain('getProxmoxVariantPresentation');
    expect(proxmoxDirectWorkspaceStateSource).toContain('buildProxmoxDiscoveryPrefillNode');
    expect(proxmoxSettingsPresentationSource).toContain(
      'export const PROXMOX_VARIANT_PRESENTATION',
    );
    expect(proxmoxSettingsPresentationSource).toContain(
      'export function getProxmoxVariantPresentation',
    );
    expect(proxmoxSettingsPresentationSource).toContain(
      'export function buildProxmoxDiscoveryPrefillNode',
    );
    expect(infrastructureWorkspaceSource).toContain(
      '@/components/Settings/infrastructureWorkspaceModel'.replace('@/components/Settings/', './'),
    );
    expect(infrastructureWorkspaceSource).not.toContain('inferViewFromPath');
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function getInfrastructureWorkspaceViewFromPath',
    );
    expect(infrastructureWorkspaceModelSource).toContain(
      'export function buildInfrastructureWorkspacePath',
    );
    expect(nodeModalSource).toContain('getNodeProductName');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalBasicInfoSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalAuthenticationSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalMonitoringSection');
    expect(nodeModalSource).toContain('@/components/Settings/NodeModalStatusFooter');
    expect(nodeModalSource).toContain('@/components/Settings/nodeModalModel');
    expect(nodeModalSource).toContain('@/components/Settings/useNodeModalState');
    expect(nodeModalSource).not.toContain('title="Basic information"');
    expect(nodeModalSource).not.toContain('title="Authentication"');
    expect(nodeModalSource).not.toContain('title="Monitoring coverage"');
    expect(nodeModalSource).not.toContain('const getCleanFormData =');
    expect(nodeModalSource).not.toContain('const nodeProductName =');
    expect(nodeModalSource).not.toContain('const deriveNameFromHost =');
    expect(nodeModalSource).not.toContain('const PVE_MANUAL_PERMISSION_COMMAND = `');
    expect(nodeModalSource).not.toContain('const [quickSetupBootstrap, setQuickSetupBootstrap] =');
    expect(nodeModalSource).not.toContain('const handleTestConnection = async () =>');
    expect(nodeModalSource).not.toContain("testResult()?.status === 'success'");
    expect(nodeModalSource).not.toContain("testResult()?.status === 'warning'");
    expect(nodeModalModelSource).toContain('export const deriveNameFromHost =');
    expect(nodeModalModelSource).toContain('export const PVE_MANUAL_PERMISSION_COMMAND = `');
    expect(nodeModalStateSource).toContain('export const useNodeModalState =');
    expect(nodeModalStateSource).toContain(
      'export type NodeModalState = ReturnType<typeof useNodeModalState>;',
    );
    expect(nodeModalStateSource).toContain('getNodeModalDefaultFormData');
    expect(nodeModalStateSource).toContain('getNodeModalTestResultPresentation');
    expect(nodeModalStateSource).toContain('buildNodeModalMonitoringPayload');
    expect(nodeModalBasicInfoSectionSource).toContain('title="Basic information"');
    expect(nodeModalBasicInfoSectionSource).toContain('getNodeEndpointPlaceholder');
    expect(nodeModalBasicInfoSectionSource).toContain('getNodeGuestUrlPlaceholder');
    expect(nodeModalAuthenticationSectionSource).toContain(
      '@/components/Settings/NodeModalSetupGuideSection',
    );
    expect(nodeModalAuthenticationSectionSource).toContain('getNodeUsernamePlaceholder');
    expect(nodeModalAuthenticationSectionSource).toContain('getNodeUsernameHelp');
    expect(nodeModalAuthenticationSectionSource).toContain('getNodeTokenIdPlaceholder');
    expect(nodeModalSetupGuideSectionSource).toContain('Connection Setup');
    expect(nodeModalMonitoringSectionSource).toContain('title="Monitoring coverage"');
    expect(nodeModalMonitoringSectionSource).toContain('getNodeMonitoringCoverageCopy');
    expect(nodeModalMonitoringSectionSource).toContain('getTemperatureMonitoringLockedCopy');
    expect(nodeModalStatusFooterSource).toContain('Start your free 14-day trial');
    expect(nodeModalPresentationSource).toContain('export function getNodeModalDefaultFormData');
    expect(nodeModalPresentationSource).toContain('export function getNodeProductName');
    expect(nodeModalPresentationSource).toContain('export function getNodeEndpointPlaceholder');
    expect(nodeModalPresentationSource).toContain('export function getNodeGuestUrlPlaceholder');
    expect(nodeModalPresentationSource).toContain('export function getNodeUsernamePlaceholder');
    expect(nodeModalPresentationSource).toContain('export function getNodeTokenIdPlaceholder');
    expect(nodeModalPresentationSource).toContain('export function getNodeMonitoringCoverageCopy');
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
    expect(storageDomainSource).toContain('export const getCephDisconnectedStatePresentation');
    expect(storageDomainSource).toContain('export const getCephNoClustersStatePresentation');
    expect(storageDomainSource).toContain('export const getCephPoolsSearchEmptyStatePresentation');
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
    expect(deployFlowPresentationSource).toContain('export function getDeployNoSourceAgentsState');
    expect(deployFlowPresentationSource).toContain('export function getDeployNoCandidatesState');
    expect(deployFlowPresentationSource).toContain(
      'export function getDeployInstallCommandLoadingState',
    );
    expect(deployStatusPresentationSource).toContain('export const getDeployStatusPresentation');
    expect(alertHistoryTableSectionSource).toContain('AlertHistoryTableGroupRow');
    expect(alertHistoryTableSectionSource).toContain('AlertHistoryTableAlertRow');
    expect(alertHistoryTableAlertRowSource).toContain('getAlertHistoryStatusPresentation');
    expect(alertHistoryTableAlertRowSource).toContain('getAlertIncidentLevelBadgeClass');
    expect(alertsPageSource).toContain(
      "import { AlertsConfigurationSurface } from '@/features/alerts/AlertsConfigurationSurface';",
    );
    expect(alertsPageSource).not.toContain('getAlertDestinationsConfigLoadError');
    expect(alertsConfigurationSurfaceSource).toContain('useAlertsConfigurationState');
    expect(alertsConfigurationSurfaceSource).not.toContain('AlertsAPI.getConfig');
    expect(alertsConfigurationStateSource).toContain('AlertsAPI.getConfig');
    expect(alertsConfigurationStateSource).toContain('createDefaultAlertsConfigurationSnapshot');
    expect(alertsConfigurationStateSource).toContain('readAlertsConfigurationSnapshot');
    expect(alertsConfigurationStateSource).toContain('buildAlertsConfigurationPayload');
    expect(alertsConfigurationStateSource).toContain('useAlertDestinationsState');
    expect(alertsConfigurationStateSource).toContain('useAlertsConfigurationSnapshotState');
    expect(alertsConfigurationStateSource).toContain('useAlertOverridesState');
    expect(alertsConfigurationStateSource).not.toContain('NotificationsAPI.getEmailConfig');
    expect(alertsConfigurationStateSource).not.toContain('NotificationsAPI.updateEmailConfig');
    expect(alertsConfigurationStateSource).toContain("eventBus.on('org_switched'");
    expect(alertsConfigurationStateSource).not.toContain('const createHysteresisThreshold =');
    expect(alertsConfigurationStateSource).not.toContain('const normalizeGap =');
    expect(alertsConfigurationStateSource).not.toContain(
      'const [rawOverridesConfig, setRawOverridesConfig] = createSignal',
    );
    expect(alertsConfigurationStateSource).not.toContain(
      'const [scheduleQuietHours, setScheduleQuietHours] = createSignal',
    );
    expect(alertsConfigurationStateSource).not.toContain('const hostOverrideIdCandidates =');
    expect(alertsConfigurationStateSource).not.toContain('const allGuests = createMemo');
    expect(alertsConfigurationSnapshotStateSource).toContain(
      'export function useAlertsConfigurationSnapshotState',
    );
    expect(alertsConfigurationSnapshotStateSource).toContain(
      'const [scheduleQuietHours, setScheduleQuietHours] = createSignal',
    );
    expect(alertsConfigurationSnapshotStateSource).toContain('applyConfigurationSnapshot');
    expect(alertsConfigurationSnapshotStateSource).toContain('captureConfigurationSnapshot');
    expect(alertsConfigurationSnapshotStateSource).toContain('resetGuestDefaults');
    expect(alertsConfigurationSnapshotStateSource).toContain('FACTORY_GUEST_DEFAULTS');
    expect(alertsConfigurationModelSource).toContain(
      'export function createDefaultAlertsConfigurationSnapshot',
    );
    expect(alertsConfigurationModelSource).toContain(
      'export function readAlertsConfigurationSnapshot',
    );
    expect(alertsConfigurationModelSource).toContain(
      'export function buildAlertsConfigurationPayload',
    );
    expect(alertsConfigurationModelSource).toContain('const createHysteresisThreshold =');
    expect(alertsConfigurationModelSource).toContain('const normalizeGap =');
    expect(alertOverridesStateSource).toContain('export function useAlertOverridesState');
    expect(alertOverridesStateSource).toContain('pbsInstanceFromResource');
    expect(alertOverridesStateSource).toContain('buildProjectedOverrides');
    expect(alertOverridesStateSource).not.toContain('getActionableAgentIdFromResource');
    expect(alertOverridesStateSource).not.toContain('const hostOverrideIdCandidates =');
    expect(alertOverridesStateSource).toContain('props.setOverviewOverrides(overrides())');
    expect(alertOverridesModelSource).toContain('export const normalizeRawOverridesConfig =');
    expect(alertOverridesModelSource).toContain('export const hostOverrideIdCandidates =');
    expect(alertOverridesModelSource).toContain('export const dockerHostOverrideIdCandidates =');
    expect(alertOverridesModelSource).toContain('export const buildProjectedOverrides =');
    expect(alertOverridesModelSource).toContain('getActionableAgentIdFromResource');
    expect(alertDestinationsStateSource).toContain('getAlertDestinationsConfigLoadError');
    expect(alertDestinationsStateSource).toContain('NotificationsAPI.getEmailConfig');
    expect(alertDestinationsStateSource).toContain('NotificationsAPI.updateEmailConfig');
    expect(alertDestinationsStateSource).toContain('buildEmailConfigPayload');
    expect(alertDestinationsStateSource).toContain('buildAppriseConfigPayload');
    expect(alertDestinationsStateSource).toContain('normalizeAppriseConfig');
    expect(alertDestinationsStateSource).not.toContain('formatAppriseTargets');
    expect(alertDestinationsStateSource).not.toContain('parseAppriseTargets');
    expect(alertDestinationsModelSource).toContain('export function normalizeAppriseConfig');
    expect(alertDestinationsModelSource).toContain('export function buildEmailConfigPayload');
    expect(alertDestinationsModelSource).toContain('export function buildAppriseConfigPayload');
    expect(alertDestinationsModelSource).toContain('formatAppriseTargets');
    expect(alertDestinationsModelSource).toContain('parseAppriseTargets');
    expect(alertDestinationsTabStateSource).toContain('export function useAlertDestinationsTabState');
    expect(alertDestinationsTabStateSource).toContain('NotificationsAPI.testNotification');
    expect(alertDestinationsTabStateSource).toContain('useAlertWebhookDestinationsState');
    expect(alertDestinationsTabStateSource).not.toContain('NotificationsAPI.getWebhooks');
    expect(alertDestinationsTabStateSource).not.toContain('NotificationsAPI.createWebhook');
    expect(alertWebhookDestinationsStateSource).toContain(
      'export function useAlertWebhookDestinationsState',
    );
    expect(alertWebhookDestinationsStateSource).toContain('NotificationsAPI.getWebhooks');
    expect(alertWebhookDestinationsStateSource).toContain('NotificationsAPI.createWebhook');
    expect(alertWebhookDestinationsStateSource).toContain(
      'getAlertDestinationsWebhookLoadError',
    );
    expect(alertDestinationsTabSource).toContain('useAlertDestinationsTabState');
    expect(alertDestinationsTabSource).toContain('AlertDestinationsLoadingState');
    expect(alertDestinationsTabSource).toContain('AlertDestinationsLoadErrorCard');
    expect(alertDestinationsTabSource).toContain('AlertEmailDestinationsSection');
    expect(alertDestinationsTabSource).toContain('AlertAppriseDestinationsSection');
    expect(alertDestinationsTabSource).toContain('AlertWebhookDestinationsSection');
    expect(alertDestinationsTabSource).not.toContain('NotificationsAPI.getWebhooks');
    expect(alertDestinationsTabSource).not.toContain('NotificationsAPI.testNotification');
    expect(alertDestinationsTabSource).not.toContain('NotificationsAPI.createWebhook');
    expect(alertDestinationsTabSource).not.toContain('ALERT_DESTINATIONS_EMAIL_PANEL_TITLE');
    expect(alertDestinationsTabSource).not.toContain('ALERT_DESTINATIONS_APPRISE_PANEL_TITLE');
    expect(alertDestinationsTabSource).not.toContain('getAlertWebhooksSectionTitle');
    expect(alertsPageSource).toContain(
      "import { HistoryTab } from '@/features/alerts/tabs/HistoryTab';",
    );
    expect(alertsPageSource).not.toContain('function HistoryTab(');
    expect(alertOverviewTabSource).toContain('AlertOverviewStatsCards');
    expect(alertOverviewTabSource).toContain('AlertOverviewActiveAlertsSection');
    expect(alertOverviewTabSource).toContain('useAlertIncidentTimelineState');
    expect(alertHistoryTabSource).toContain('useAlertHistoryState');
    expect(alertHistoryTabSource).toContain('AlertHistoryFrequencyCard');
    expect(alertHistoryTabSource).toContain('AlertHistoryFiltersCard');
    expect(alertHistoryTabSource).toContain('AlertResourceIncidentsPanel');
    expect(alertHistoryTabSource).toContain('AlertHistoryTableSection');
    expect(alertHistoryTabSource).toContain('AlertHistoryAdministrationCard');
    expect(alertsPageSource).toContain('getAlertsPageHeaderMeta');
    expect(alertHistoryTabSource).not.toContain('useAlertIncidentTimelineState');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.getHistory');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.getIncidentsForResource');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.clearHistory');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.getIncidentTimeline');
    expect(alertHistoryTabSource).not.toContain('AlertsAPI.addIncidentNote');
    expect(alertHistoryTabSource).not.toContain('usePersistentSignal(');
    expect(alertsPageSource).toContain('getAlertActivationPresentation');
    expect(alertsPageSource).toContain('getAlertActivationSuccess');
    expect(alertsPageSource).toContain('getAlertActivationFailure');
    expect(alertsPageSource).toContain('getAlertDeactivationSuccess');
    expect(alertsPageSource).toContain('getAlertDeactivationFailure');
    expect(alertHistoryFrequencyCardSource).toContain('getAlertFrequencySelectionPresentation');
    expect(alertHistoryFrequencyCardSource).toContain('getAlertFrequencyClearFilterButtonClass');
    expect(alertHistoryFrequencyCardSource).toContain('getAlertSeverityDotClass');
    expect(alertHistoryFrequencyCardSource).toContain('getAlertBucketCountLabel');
    expect(alertHistoryFiltersCardSource).toContain('getAlertHistorySearchPlaceholder');
    expect(alertResourceIncidentsPanelSource).toContain('IncidentEventFilters');
    expect(alertResourceIncidentsPanelSource).toContain('IncidentTimelineEventCard');
    expect(alertResourceIncidentsPanelSource).toContain('getAlertResourceIncidentCardClass');
    expect(alertResourceIncidentsPanelSource).toContain('getAlertResourceIncidentSummaryRowClass');
    expect(alertResourceIncidentsPanelSource).toContain('getAlertResourceIncidentToggleButtonClass');
    expect(alertResourceIncidentsPanelSource).toContain('getAlertResourceIncidentTruncatedEventsLabel');
    expect(alertResourceIncidentsPanelSource).not.toContain('getAlertIncidentTimelineEventCardClass');
    expect(alertResourceIncidentsPanelSource).not.toContain('getAlertIncidentTimelineDetailClass');
    expect(alertResourceIncidentsPanelSource).not.toContain('getAlertIncidentTimelineCommandClass');
    expect(alertHistoryTableSectionSource).toContain('AlertHistoryTableGroupRow');
    expect(alertHistoryTableSectionSource).toContain('AlertHistoryTableAlertRow');
    expect(alertHistoryTableSectionSource).toContain('getAlertHistoryEmptyState');
    expect(alertHistoryTableSectionSource).toContain('getAlertHistoryLoadingState');
    expect(alertHistoryTableSectionSource).not.toContain('IncidentTimelinePanel');
    expect(alertHistoryTableSectionSource).not.toContain('InvestigateAlertButton');
    expect(alertHistoryTableGroupRowSource).toContain('getGroupSummaryLabel');
    expect(alertHistoryTableAlertRowSource).toContain('getAlertHistoryStatusPresentation');
    expect(alertHistoryTableAlertRowSource).toContain('getAlertHistorySourcePresentation');
    expect(alertHistoryTableAlertRowSource).toContain('getAlertHistoryResourceTypeBadgeClass');
    expect(alertHistoryTableAlertRowSource).toContain('IncidentTimelinePanel');
    expect(alertHistoryTableAlertRowSource).toContain('InvestigateAlertButton');
    expect(alertHistoryAdministrationCardSource).toContain('getAlertAdministrationSectionTitle');
    expect(alertHistoryAdministrationCardSource).toContain(
      'getAlertAdministrationSectionDescription',
    );
    expect(alertHistoryAdministrationCardSource).toContain(
      'getAlertAdministrationClearHistoryLabel',
    );
    expect(alertHistoryStateSource).toContain('export function useAlertHistoryState');
    expect(alertHistoryStateSource).toContain('export type AlertHistoryState');
    expect(alertHistoryStateSource).toContain('AlertsAPI.getHistory');
    expect(alertHistoryStateSource).toContain('AlertsAPI.clearHistory');
    expect(alertHistoryStateSource).toContain('useAlertResourceIncidentsState');
    expect(alertHistoryStateSource).toContain('useAlertIncidentTimelineState');
    expect(alertHistoryStateSource).not.toContain('AlertsAPI.getIncidentsForResource');
    expect(alertHistoryStateSource).toContain('buildAlertHistoryItems');
    expect(alertHistoryStateSource).toContain('buildAlertTrends');
    expect(alertHistoryStateSource).toContain('groupAlertHistoryItems');
    expect(alertHistoryStateSource).not.toContain('const formatDuration =');
    expect(alertHistoryStateSource).not.toContain('const formatBucketRange =');
    expect(alertHistoryStateSource).not.toContain('const formatAxisTickLabel =');
    expect(alertHistoryStateSource).not.toContain('const monthNames = [');
    expect(alertHistoryModelSource).toContain('export function buildAlertHistoryItems');
    expect(alertHistoryModelSource).toContain('export function buildAlertTrends');
    expect(alertHistoryModelSource).toContain('export function groupAlertHistoryItems');
    expect(alertHistoryModelSource).toContain('export const MS_PER_HOUR');
    expect(alertResourceIncidentsStateSource).toContain(
      'export function useAlertResourceIncidentsState',
    );
    expect(alertResourceIncidentsStateSource).toContain('AlertsAPI.getIncidentsForResource');
    expect(alertsPageSource).toContain('getAlertsSidebarTabClass');
    expect(alertsPageSource).toContain('getAlertsMobileTabClass');
    expect(alertsPageSource).toContain('getAlertsTabTitle');
    expect(alertsPageSource).toContain('getAlertsTabGroups');
    expect(alertsConfigurationSurfaceSource).toContain(
      "import { ThresholdsTab } from './tabs/ThresholdsTab';",
    );
    expect(alertsPageSource).not.toContain(
      "import { ThresholdsTab } from '@/features/alerts/tabs/ThresholdsTab';",
    );
    expect(alertsPageSource).not.toContain(
      "import { ThresholdsTable } from '@/components/Alerts/ThresholdsTable';",
    );
    expect(alertsPageSource).not.toContain('function ThresholdsTab(');
    expect(alertThresholdsTabSource).toContain(
      "import { ThresholdsTable } from '@/components/Alerts/ThresholdsTable';",
    );
    expect(alertThresholdsTabSource).toContain('buildThresholdsTableProps');
    expect(alertThresholdsTabSource).not.toContain('pmgThresholds={props.pmgThresholds}');
    expect(thresholdsTabModelSource).toContain('export interface ThresholdsTabProps');
    expect(thresholdsTabModelSource).toContain('export function buildThresholdsTableProps');
    expect(thresholdsTabModelSource).toContain('guestDefaults: props.guestDefaults()');
    expect(thresholdsTabModelSource).not.toContain('hasUnsavedChanges');
    expect(thresholdsTableSource).toContain(
      "import { useThresholdsTableState } from '@/features/alerts/thresholds/hooks/useThresholdsTableState';",
    );
    expect(thresholdsTableSource).toContain("import { ThresholdsTableProxmoxTab } from './ThresholdsTableProxmoxTab';");
    expect(thresholdsTableSource).toContain("import { ThresholdsTablePMGTab } from './ThresholdsTablePMGTab';");
    expect(thresholdsTableSource).toContain("import { ThresholdsTableAgentsTab } from './ThresholdsTableAgentsTab';");
    expect(thresholdsTableSource).toContain("import { ThresholdsTableDockerTab } from './ThresholdsTableDockerTab';");
    expect(thresholdsTableSource).not.toContain('const [searchTerm, setSearchTerm] = createSignal');
    expect(thresholdsTableSource).not.toContain('const handleTabClick =');
    expect(thresholdsTableSource).not.toContain("groupedResources={state.guestsGroupedByNode()}");
    expect(thresholdsTableSource).not.toContain('dockerIgnoredPrefixesPresentation.title');
    expect(thresholdsTableProxmoxTabSource).toContain('export function ThresholdsTableProxmoxTab');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxNodesSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxPBSSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxGuestsSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxGuestFilteringSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxBackupsSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxSnapshotsSection');
    expect(thresholdsTableProxmoxTabSource).toContain('ThresholdsTableProxmoxStorageSection');
    expect(thresholdsTableProxmoxTabSource).not.toContain('backupOrphanedPresentation');
    expect(thresholdsTablePMGTabSource).toContain('export function ThresholdsTablePMGTab');
    expect(thresholdsTablePMGTabSource).toContain('pmgGlobalDefaults()');
    expect(thresholdsTableAgentsTabSource).toContain('export function ThresholdsTableAgentsTab');
    expect(thresholdsTableAgentsTabSource).toContain('ThresholdsTableAgentsResourcesSection');
    expect(thresholdsTableAgentsTabSource).toContain('ThresholdsTableAgentDisksSection');
    expect(thresholdsTableAgentsTabSource).not.toContain('agentDisksGroupedByAgent()');
    expect(thresholdsTableAgentsResourcesSectionSource).toContain(
      'export function ThresholdsTableAgentsResourcesSection',
    );
    expect(thresholdsTableAgentDisksSectionSource).toContain(
      'export function ThresholdsTableAgentDisksSection',
    );
    expect(thresholdsTableAgentDisksSectionSource).toContain('agentDisksGroupedByAgent()');
    expect(thresholdsTableDockerTabSource).toContain('export function ThresholdsTableDockerTab');
    expect(thresholdsTableDockerTabSource).toContain('ThresholdsTableDockerIgnoredPrefixesSection');
    expect(thresholdsTableDockerTabSource).toContain('ThresholdsTableDockerServiceGapSection');
    expect(thresholdsTableDockerTabSource).toContain('ThresholdsTableDockerHostsSection');
    expect(thresholdsTableDockerTabSource).toContain('ThresholdsTableDockerContainersSection');
    expect(thresholdsTableDockerTabSource).not.toContain('dockerIgnoredPrefixesPresentation.title');
    expect(thresholdsTableDockerTabSource).not.toContain('serviceGapValidationMessage()');
    expect(thresholdsTableDockerIgnoredPrefixesSectionSource).toContain(
      'export function ThresholdsTableDockerIgnoredPrefixesSection',
    );
    expect(thresholdsTableDockerIgnoredPrefixesSectionSource).toContain(
      'dockerIgnoredPrefixesPresentation.title',
    );
    expect(thresholdsTableDockerServiceGapSectionSource).toContain(
      'export function ThresholdsTableDockerServiceGapSection',
    );
    expect(thresholdsTableDockerServiceGapSectionSource).toContain('serviceGapValidationMessage()');
    expect(thresholdsTableDockerHostsSectionSource).toContain(
      'export function ThresholdsTableDockerHostsSection',
    );
    expect(thresholdsTableDockerContainersSectionSource).toContain(
      'export function ThresholdsTableDockerContainersSection',
    );
    expect(thresholdsTableSectionPropsSource).toContain('export interface ThresholdsTableSectionProps');
    expect(thresholdsTableProxmoxNodesSectionSource).toContain(
      'export function ThresholdsTableProxmoxNodesSection',
    );
    expect(thresholdsTableProxmoxPBSSectionSource).toContain(
      'export function ThresholdsTableProxmoxPBSSection',
    );
    expect(thresholdsTableProxmoxGuestsSectionSource).toContain(
      'export function ThresholdsTableProxmoxGuestsSection',
    );
    expect(thresholdsTableProxmoxGuestFilteringSectionSource).toContain(
      'export function ThresholdsTableProxmoxGuestFilteringSection',
    );
    expect(thresholdsTableProxmoxBackupsSectionSource).toContain(
      'export function ThresholdsTableProxmoxBackupsSection',
    );
    expect(thresholdsTableProxmoxBackupsSectionSource).toContain('backupOrphanedPresentation');
    expect(thresholdsTableProxmoxSnapshotsSectionSource).toContain(
      'export function ThresholdsTableProxmoxSnapshotsSection',
    );
    expect(thresholdsTableProxmoxStorageSectionSource).toContain(
      'export function ThresholdsTableProxmoxStorageSection',
    );
    expect(thresholdsDataHookSource).toContain('export function useThresholdsData');
    expect(thresholdsDataHookSource).toContain('useThresholdsHostData(inputs)');
    expect(thresholdsDataHookSource).toContain('useThresholdsDockerData(inputs)');
    expect(thresholdsDataHookSource).toContain('useThresholdsGuestData(inputs)');
    expect(thresholdsDataHookSource).toContain('useThresholdsInfrastructureData(inputs)');
    expect(thresholdsDataHookSource).not.toContain('const hostOverrideIdCandidates =');
    expect(thresholdsDataHookSource).not.toContain('const dockerContainersGroupedByHost = createMemo');
    expect(thresholdsHostDataHookSource).toContain('export function useThresholdsHostData');
    expect(thresholdsDockerDataHookSource).toContain('export function useThresholdsDockerData');
    expect(thresholdsGuestDataHookSource).toContain('export function useThresholdsGuestData');
    expect(thresholdsInfrastructureDataHookSource).toContain(
      'export function useThresholdsInfrastructureData',
    );
    expect(thresholdsResourceModelSource).toContain('export function hostOverrideIdCandidates');
    expect(thresholdsResourceModelSource).toContain('export function buildNodeHeaderMeta');
    expect(thresholdsResourceModelSource).toContain('export const normalizeStorageStatus');
    expect(thresholdsTableStateHookSource).toContain('export function useThresholdsTableState');
    expect(thresholdsTableStateHookSource).toContain('useCollapsedSections()');
    expect(thresholdsTableStateHookSource).toContain('useThresholdsData(props, editingId, searchTerm)');
    expect(thresholdsTableStateHookSource).toContain('useThresholdsRecoveryDefaultsState(props)');
    expect(thresholdsTableStateHookSource).toContain('useThresholdsOverrideMutations');
    expect(thresholdsTableStateHookSource).toContain('useThresholdsAvailabilityMutations');
    expect(thresholdsTableStateHookSource).not.toContain('const saveEdit = (resourceId: string) => {');
    expect(thresholdsTableStateHookSource).not.toContain(
      'const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {',
    );
    expect(thresholdsDataHookSource).not.toContain('const sanitizeSnapshotConfig =');
    expect(thresholdsDataHookSource).not.toContain('const sanitizeBackupConfig =');
    expect(thresholdsRecoveryDefaultsStateHookSource).toContain(
      'export function useThresholdsRecoveryDefaultsState',
    );
    expect(thresholdsRecoveryDefaultsStateHookSource).toContain('const sanitizeSnapshotConfig =');
    expect(thresholdsRecoveryDefaultsStateHookSource).toContain('const sanitizeBackupConfig =');
    expect(thresholdsOverrideMutationsHookSource).toContain(
      'export function useThresholdsOverrideMutations',
    );
    expect(thresholdsOverrideMutationsHookSource).toContain('const saveEdit = (resourceId: string) => {');
    expect(thresholdsOverrideMutationsHookSource).toContain(
      'const handleSaveBulkEdit = (thresholds: Record<string, number | undefined>) => {',
    );
    expect(thresholdsOverrideMutationsHookSource).not.toContain('matchesAlertIdentifier');
    expect(thresholdsOverrideMutationsHookSource).not.toContain(
      'const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {',
    );
    expect(thresholdsAvailabilityMutationsHookSource).toContain(
      'export function useThresholdsAvailabilityMutations',
    );
    expect(thresholdsAvailabilityMutationsHookSource).toContain('matchesAlertIdentifier');
    expect(thresholdsAvailabilityMutationsHookSource).toContain(
      'const toggleNodeConnectivity = (resourceId: string, forceState?: boolean) => {',
    );
    expect(thresholdsAvailabilityMutationsHookSource).toContain(
      'const setOfflineState = (resourceId: string, state: OfflineState) => {',
    );
    expect(thresholdsOverrideMutationModelSource).toContain('export const upsertOverride =');
    expect(thresholdsOverrideMutationModelSource).toContain(
      'export const withThresholdEntries =',
    );
    expect(thresholdsOverrideMutationModelSource).toContain('export const stripStateKeys =');
    expect(alertsPageSource).not.toContain('getAlertConfigUnsavedChangesLabel');
    expect(alertsConfigurationSurfaceSource).toContain('getAlertConfigUnsavedChangesLabel');
    expect(alertsConfigurationSurfaceSource).toContain('getAlertConfigSaveChangesLabel');
    expect(alertsConfigurationSurfaceSource).toContain('getAlertConfigDiscardLabel');
    expect(alertsConfigurationStateSource).toContain('getAlertConfigDiscardedSuccess');
    expect(alertsConfigurationStateSource).toContain('getAlertConfigReloadFailure');
    expect(alertsPageSource).toContain('getAlertConfigLeaveConfirmation');
    expect(alertScheduleTabSource).toContain('getAlertConfigResetDefaultsLabel');
    expect(alertScheduleTabSource).toContain('getAlertConfigResetDefaultsTitle');
    expect(alertScheduleTabSource).toContain('getAlertConfigQuietHourSuppressOptions');
    expect(alertScheduleTabSource).toContain('AlertQuietHoursSection');
    expect(alertScheduleTabSource).toContain('AlertCooldownSection');
    expect(alertScheduleTabSource).toContain('AlertGroupingSection');
    expect(alertScheduleTabSource).toContain('AlertRecoverySection');
    expect(alertScheduleTabSource).toContain('AlertEscalationSection');
    expect(alertScheduleTabSource).toContain('AlertScheduleSummarySection');
    expect(alertScheduleTabSource).not.toContain('ALERT_CONFIG_COOLDOWN_PERIOD_LABEL');
    expect(alertScheduleTabSource).not.toContain('ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL');
    expect(alertScheduleTabSource).not.toContain('ALERT_CONFIG_GROUPING_WINDOW_LABEL');
    expect(alertScheduleTabSource).not.toContain('ALERT_CONFIG_GROUPING_STRATEGY_LABEL');
    expect(alertsPageSource).not.toContain('const statusClasses =');
    expect(alertsPageSource).not.toContain('const levelClasses =');
    expect(alertsPageSource).not.toContain("alert.source === 'ai' ? 'Patrol' : 'Alert'");
    expect(alertsPageSource).not.toContain("alert.resourceType === 'VM'");
    expect(alertsPageSource).not.toContain(
      "isAlertsActive() ? 'text-green-600 dark:text-green-400' : 'text-muted'",
    );
    expect(alertsPageSource).not.toContain("isAlertsActive() ? 'bg-blue-600' : 'bg-surface-hover'");
    expect(alertsPageSource).not.toContain(
      'Failed to load notification configuration. Your existing settings could not be retrieved.',
    );
    expect(alertsPageSource).not.toContain('Failed to load webhook configuration.');
    expect(alertsPageSource).not.toContain(
      'Saving now may overwrite your existing settings with defaults.',
    );
    expect(alertsPageSource).not.toContain('Configure SMTP delivery for alert emails.');
    expect(alertsPageSource).not.toContain(
      'Relay grouped alerts through Apprise via CLI or remote API.',
    );
    expect(alertsPageSource).not.toContain(
      'Choose how Pulse should execute Apprise notifications.',
    );
    expect(alertsPageSource).not.toContain(
      'Enter one Apprise URL per line. Commas are also supported.',
    );
    expect(alertsPageSource).not.toContain(
      'Optional: override the URLs defined on your Apprise API instance. Leave blank to use the server defaults.',
    );
    expect(alertsPageSource).not.toContain('Enable Apprise notifications before sending a test.');
    expect(alertsPageSource).not.toContain('Add at least one Apprise target to test CLI delivery.');
    expect(alertsPageSource).not.toContain('Enter an Apprise API server URL to test API delivery.');
    expect(alertsPageSource).not.toContain('Test email sent successfully! Check your inbox.');
    expect(alertsPageSource).not.toContain('Failed to send test email');
    expect(alertsPageSource).not.toContain('Test Apprise notification sent successfully!');
    expect(alertsPageSource).not.toContain('Failed to send test notification');
    expect(alertsPageSource).not.toContain('Test webhook sent successfully!');
    expect(alertsPageSource).not.toContain('Failed to send test webhook');
    expect(alertsPageSource).not.toContain(
      'Enable only when the Apprise API uses a self-signed certificate.',
    );
    expect(alertsPageSource).not.toContain("isAlertsActive() ? 'translate-x-5' : 'translate-x-0'");
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
    expect(alertsPageSource).not.toContain('CPU, memory, disk, and network thresholds stay quiet.');
    expect(alertsPageSource).not.toContain('Silence storage usage, disk health, and ZFS events.');
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
    expect(alertGroupingPresentationSource).toContain('export function getAlertGroupingCardClass');
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
    expect(configuredNodeTablesSource).not.toContain(
      'const resolveConfiguredNodeStatusIndicator =',
    );
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
    expect(auditWebhookPanelSource).toContain('@/utils/auditWebhookPresentation');
    expect(auditWebhookPanelSource).toContain('@/components/Settings/useAuditWebhookPanelState');
    expect(auditWebhookPanelSource).toContain('getAuditWebhookFeatureGateCopy');
    expect(auditWebhookPanelSource).toContain('getAuditWebhookEmptyStateCopy');
    expect(auditWebhookPanelSource).toContain('getAuditWebhookLoadingState');
    expect(auditWebhookPanelSource).toContain('AUDIT_WEBHOOK_READONLY_NOTICE_CLASS');
    expect(auditWebhookPanelSource).toContain('AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS');
    expect(auditWebhookPanelSource).toContain('AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS');
    expect(auditWebhookPanelSource).not.toContain('No audit webhooks configured yet.');
    expect(auditWebhookPanelSource).not.toContain('Loading audit webhooks…');
    expect(auditWebhookPanelSource).not.toContain('Audit Webhooks (Pro)');
    expect(auditWebhookPanelSource).not.toContain('loadLicenseStatus();');
    expect(auditWebhookPanelSource).not.toContain('const fetchWebhooks = async () =>');
    expect(auditWebhookPanelSource).not.toContain('const saveWebhooks = async (urls: string[]) =>');
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookFeatureGateCopy',
    );
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookEmptyStateCopy',
    );
    expect(auditWebhookPresentationSource).toContain('export function getAuditWebhookLoadingState');
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookInvalidUrlMessage',
    );
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookDuplicateUrlMessage',
    );
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookSaveSuccessMessage',
    );
    expect(auditWebhookPresentationSource).toContain(
      'export function getAuditWebhookSaveErrorMessage',
    );
    expect(auditWebhookPresentationSource).toContain('AUDIT_WEBHOOK_SECURITY_NOTE_TITLE');
    expect(auditWebhookPresentationSource).toContain('AUDIT_WEBHOOK_SECURITY_NOTE_BODY');
    expect(auditWebhookStateSource).toContain('export const useAuditWebhookPanelState =');
    expect(auditWebhookStateSource).toContain('loadLicenseStatus();');
    expect(auditWebhookStateSource).toContain('trackPaywallViewed');
    expect(auditWebhookStateSource).toContain('const fetchWebhooks = async () =>');
    expect(auditWebhookStateSource).toContain('const saveWebhooks = async (urls: string[]) =>');
    expect(auditLogPanelSource).toContain('getAuditLogLoadingState');
    expect(auditLogPanelSource).toContain('getAuditLogEmptyState');
    expect(auditLogPanelSource).toContain('@/components/Settings/useAuditLogPanelState');
    expect(auditLogPanelSource).not.toContain('No audit events found');
    expect(auditLogPanelSource).not.toContain('createLocalStorageStringSignal');
    expect(auditLogPanelSource).not.toContain('const fetchAuditEvents = async (');
    expect(auditLogPanelSource).not.toContain('const verifyAllEvents = async (');
    expect(auditLogPanelSource).not.toContain('trackPaywallViewed');
    expect(auditLogPresentationSource).toContain('export function getAuditLogLoadingState');
    expect(auditLogPresentationSource).toContain('export function getAuditLogEmptyState');
    expect(auditLogStateSource).toContain('export const useAuditLogPanelState =');
    expect(auditLogStateSource).toContain('createLocalStorageStringSignal');
    expect(auditLogStateSource).toContain('const fetchAuditEvents = async (');
    expect(auditLogStateSource).toContain('const verifyAllEvents = async (');
    expect(auditLogStateSource).toContain('trackPaywallViewed');
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
    expect(auditLogPresentationSource).toContain('export function getAuditEventStatusPresentation');
    expect(auditLogPresentationSource).toContain('export const AUDIT_REFRESH_BUTTON_CLASS');
    expect(auditLogPresentationSource).toContain('export const AUDIT_VERIFY_ALL_BUTTON_CLASS');
    expect(auditLogPresentationSource).toContain('export const AUDIT_VERIFY_ROW_BUTTON_CLASS');
    expect(diagnosticsPanelSource).toContain('@/components/Settings/DiagnosticsResultsPanel');
    expect(diagnosticsPanelSource).toContain('@/components/Settings/useDiagnosticsPanelState');
    expect(diagnosticsPanelSource).toContain('formatUptime');
    expect(diagnosticsPanelSource).not.toContain('No PBS configured');
    expect(diagnosticsPanelSource).not.toContain("apiFetchJSON('/api/diagnostics')");
    expect(diagnosticsPanelSource).not.toContain('URL.createObjectURL');
    expect(diagnosticsPanelSource).not.toContain(
      "'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'",
    );
    expect(diagnosticsPanelSource).not.toContain(
      "'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'",
    );
    expect(diagnosticsResultsPanelSource).toContain('getStatusIndicatorBadgeToneClasses(');
    expect(diagnosticsResultsPanelSource).toContain('DIAGNOSTICS_EMPTY_PBS_MESSAGE');
    expect(diagnosticsStateSource).toContain('export const useDiagnosticsPanelState =');
    expect(diagnosticsStateSource).toContain("apiFetchJSON('/api/diagnostics')");
    expect(diagnosticsStateSource).toContain('URL.createObjectURL');
    expect(diagnosticsModelSource).toContain('export function sanitizeDiagnosticsData');
    expect(diagnosticsPresentationSource).toContain('export const DIAGNOSTICS_EMPTY_PBS_MESSAGE');
    expect(updatesSettingsPanelSource).toContain('getUpdateBuildBadges');
    expect(updatesSettingsPanelSource).toContain('getUpdateAvailabilityHeading');
    expect(updatesSettingsPanelSource).toContain('getUpdatePrimaryStatusLabel');
    expect(updatesSettingsPanelSource).toContain('getUpdateCheckModeLabel');
    expect(updatesSettingsPanelSource).toContain('@/components/Settings/UpdateInstallGuide');
    expect(updatesSettingsPanelSource).toContain('@/components/Settings/updatesSettingsModel');
    expect(updatesSettingsPanelSource).not.toContain('Auto-check enabled');
    expect(updatesSettingsPanelSource).not.toContain('Manual checks only');
    expect(updatesSettingsPanelSource).not.toContain('Update Ready');
    expect(updatesSettingsPanelSource).not.toContain('Up to date');
    expect(updatesSettingsPanelSource).not.toContain("navigator.clipboard.writeText('update')");
    expect(updateInstallGuideSource).toContain('@/components/Settings/CopyCommandBlock');
    expect(updateInstallGuideSource).toContain('buildUpdateInstallGuide');
    expect(copyCommandBlockSource).toContain('export function CopyCommandBlock');
    expect(copyCommandBlockSource).toContain("aria-label=\"Copy to clipboard\"");
    expect(updatesSettingsModelSource).toContain('export function getUpdateChannelCardOptions');
    expect(updatesSettingsModelSource).toContain('export function buildUpdateInstallGuide');
    expect(reportingPanelStateSource).toContain('buildReportingRequest');
    expect(reportingPanelStateSource).toContain('getReportingGenerateSuccessMessage');
    expect(reportingPanelStateSource).toContain('getReportingGenerateErrorMessage');
    expect(reportingPanelModelSource).toContain('export function getReportingRangeStart');
    expect(reportingPanelModelSource).toContain('export function buildReportingRequest');
    expect(reportingPresentationSource).toContain(
      'export function getReportingGenerateSelectionRequiredMessage',
    );
    expect(reportingPresentationSource).toContain(
      'export function getReportingGenerateSuccessMessage',
    );
    expect(reportingPresentationSource).toContain(
      'export function getReportingGenerateErrorMessage',
    );
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
    expect(aiSettingsShellSource).toContain('@/components/Settings/useAISettingsState');
    expect(aiSettingsShellSource).toContain('@/components/Settings/AIModelSelectionSection');
    expect(aiSettingsShellSource).toContain('@/components/Settings/AIRuntimeControlsSection');
    expect(aiSettingsShellSource).toContain('@/components/Settings/AIChatMaintenanceSection');
    expect(aiSettingsShellSource).toContain('@/components/Settings/AISettingsStatusAndActions');
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
    expect(aiSettingsShellSource).not.toContain('const [loading, setLoading] = createSignal(false);');
    expect(aiSettingsShellSource).not.toContain('const handleSave = async (event?: Event) =>');
    expect(aiSettingsShellSource).not.toContain('Chat Session Maintenance');
    expect(aiSettingsShellSource).not.toContain('Discovery Settings');
    expect(aiSettingsShellSource).not.toContain('Pulse Permission Level');
    expect(aiSettingsSource).not.toContain(
      "providerTestResult()?.success ? 'text-green-600' : 'text-red-600'",
    );
    expect(aiSettingsStateSource).toContain('export const useAISettingsState =');
    expect(aiSettingsStateSource).toContain('export type AISettingsState =');
    expect(aiSettingsStateSource).toContain('const [loading, setLoading] = createSignal(false);');
    expect(aiSettingsStateSource).toContain('const handleSave = async (event?: Event) =>');
    expect(aiSettingsStateSource).toContain('const handleEnabledToggle = async (newValue: boolean) =>');
    expect(aiIntelligenceSource).toContain(
      "import { PatrolIntelligenceSurface } from '@/features/patrol/PatrolIntelligenceSurface';",
    );
    expect(aiIntelligenceSource).not.toContain('getPatrolSummaryPresentation');
    expect(aiIntelligenceSource).not.toContain('getAIQuickstartCreditsPresentation');
    expect(patrolIntelligenceSurfaceSource).toContain("./usePatrolIntelligenceState");
    expect(patrolIntelligenceSurfaceSource).toContain('./PatrolIntelligenceHeader');
    expect(patrolIntelligenceSurfaceSource).toContain('./PatrolIntelligenceBanners');
    expect(patrolIntelligenceSurfaceSource).toContain('./PatrolIntelligenceSummary');
    expect(patrolIntelligenceSurfaceSource).toContain('./PatrolIntelligenceWorkspace');
    expect(operationsPageRouteSource).toContain(
      "import { OperationsPageSurface } from '@/features/operations/OperationsPageSurface';",
    );
    expect(operationsPageRouteSource).toContain('<OperationsPageSurface />');
    expect(operationsPageRouteSource).not.toContain('useLocation');
    expect(operationsPageRouteSource).not.toContain('useNavigate');
    expect(operationsPageSurfaceSource).toContain('@/components/shared/Subtabs');
    expect(operationsPageSurfaceSource).toContain('getOperationsTabFromPath');
    expect(operationsPageSurfaceSource).toContain('buildOperationsPath');
    expect(operationsPageSurfaceSource).toContain('<DiagnosticsPanel />');
    expect(operationsPageSurfaceSource).toContain('<ReportingPanel />');
    expect(operationsPageSurfaceSource).toContain('<SystemLogsPanel />');
    expect(operationsPageSurfaceSource).not.toContain('-webkit-overflow-scrolling');
    expect(operationsPageModelSource).toContain('export const OPERATIONS_TABS');
    expect(operationsPageModelSource).toContain('export function getOperationsTabFromPath');
    expect(operationsPageModelSource).toContain('export function buildOperationsPath');
    expect(reportingPanelSource).toContain('OperationsPanel');
    expect(systemLogsPanelSource).toContain('OperationsPanel');
    expect(systemLogsPanelSource).toContain('./useSystemLogsPanelState');
    expect(patrolIntelligenceSurfaceSource).not.toContain('getPatrolSummaryPresentation');
    expect(patrolIntelligenceSurfaceSource).not.toContain('getAIQuickstartCreditsPresentation');
    expect(patrolIntelligenceStateSource).toContain('export function usePatrolIntelligenceState');
    expect(patrolIntelligenceStateSource).toContain('export type PatrolIntelligenceState =');
    expect(patrolIntelligenceStateSource).toContain('getPatrolStatus');
    expect(patrolIntelligenceStateSource).toContain('usePatrolStream');
    expect(patrolIntelligenceStateSource).toContain('trackPaywallViewed');
    expect(patrolIntelligenceHeaderSource).toContain('buildPatrolScheduleOptions');
    expect(patrolIntelligenceHeaderSource).toContain('getAIQuickstartCreditsPresentation');
    expect(patrolIntelligenceSummarySource).toContain('getPatrolSummaryPresentation');
    expect(patrolIntelligenceSummarySource).toContain('PATROL_NO_ISSUES_LABEL');
    expect(patrolIntelligenceWorkspaceSource).toContain('ApprovalBanner');
    expect(patrolIntelligenceWorkspaceSource).toContain('FindingsPanel');
    expect(patrolIntelligenceBannersSource).toContain('trackUpgradeClicked');
    expect(patrolIntelligenceSurfaceSource).not.toContain(
      "summaryStats().criticalFindings > 0\n                        ? 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800'",
    );
    expect(patrolIntelligenceSurfaceSource).not.toContain(
      "summaryStats().warningFindings > 0\n                        ? 'bg-amber-50 dark:bg-amber-900 border-amber-200 dark:border-amber-800'",
    );
    expect(patrolIntelligenceSurfaceSource).not.toContain(
      "summaryStats().fixedCount > 0\n                        ? 'bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-800'",
    );
    expect(patrolSummaryPresentationSource).toContain(
      'export function getPatrolSummaryPresentation',
    );
    expect(patrolIntelligenceSurfaceSource).not.toContain(
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
    expect(runToolCallTraceSource).not.toContain('Tool call details not available for this run.');
    expect(patrolEmptyStatePresentationSource).toContain(
      'export function getInvestigationMessagesState',
    );
    expect(patrolEmptyStatePresentationSource).toContain(
      'export function getInvestigationSectionState',
    );
    expect(patrolEmptyStatePresentationSource).toContain('export function getRunHistoryEmptyState');
    expect(patrolRunPresentationSource).toContain('export function getRunHistoryLoadingState');
    expect(patrolRunPresentationSource).toContain('export function getToolCallsLoadingState');
    expect(patrolRunPresentationSource).toContain('export function getToolCallsUnavailableState');
    expect(systemLogsPanelSource).toContain('getSystemLogLineClass');
    expect(systemLogsPanelSource).toContain('getSystemLogStreamPresentation');
    expect(systemLogsPanelSource).not.toContain('notificationStore.success');
    expect(systemLogsPanelSource).not.toContain('new EventSource(');
    expect(systemLogsPanelSource).not.toContain("log.includes('ERR')");
    expect(systemLogsPanelSource).not.toContain("isPaused() ? 'Stream Paused' : 'Live'");
    expect(systemLogsPanelSource).not.toContain(
      "'bg-amber-100 text-amber-600 dark:bg-amber-900 dark:text-amber-400'",
    );
    expect(systemLogsPanelStateSource).toContain("window.location.href = '/api/logs/download'");
    expect(systemLogsPanelStateSource).toContain('notificationStore.success');
    expect(systemLogsPanelStateSource).toContain('new EventSource');
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
    expect(dashboardStateSource).toContain('getDashboardInfrastructureEmptyState');
    expect(dashboardStateSource).toContain('getDashboardGuestsEmptyState');
    expect(dashboardStateSource).toContain('getDashboardLoadingState');
    expect(dashboardStateSource).toContain('getDashboardDisconnectedState');
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
    expect(guestRowCellsSource).toContain('getDashboardGuestBackupStatusPresentation');
    expect(guestRowCellsSource).toContain('getDashboardGuestBackupTooltip');
    expect(guestRowCellsSource).toContain('getDashboardGuestNetworkEmptyState');
    expect(guestRowSource).toContain('getDashboardGuestDiskStatusMessage');
    expect(guestRowSource).not.toContain('No backup found');
    expect(guestRowSource).not.toContain('No IP assigned');
    expect(guestRowSource).not.toContain(
      'No filesystems found. VM may be booting or using a Live ISO.',
    );
    expect(dashboardDiskListSource).toContain('useDiskListState');
    expect(dashboardDiskListSource).not.toContain('getDashboardGuestDiskStatusMessage');
    expect(dashboardDiskListSource).not.toContain('const getUsagePercent =');
    expect(dashboardDiskListStateSource).toContain('getDashboardGuestDiskStatusMessage');
    expect(dashboardDiskListModelSource).toContain('export const buildDashboardDiskPresentation');
    expect(dashboardDiskListModelSource).toContain('export const getDashboardDiskUsagePercent');
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
    expect(infrastructurePageSurfaceSource).toContain('useInfrastructurePageState');
    expect(infrastructurePageSurfaceSource).toContain('useNavigate');
    expect(infrastructurePageSurfaceSource).not.toContain('useLocation(');
    expect(infrastructurePageSurfaceSource).not.toContain('buildInfrastructurePath(');
    expect(infrastructurePageStateSource).toContain('useInfrastructurePageRouteState');
    expect(infrastructurePageStateSource).not.toContain('useLocation(');
    expect(infrastructurePageStateSource).not.toContain('useNavigate(');
    expect(infrastructurePageStateSource).not.toContain('parseInfrastructureLinkSearch(');
    expect(infrastructurePageStateSource).not.toContain('buildInfrastructurePath(');
    expect(infrastructurePageStateSource).not.toContain('areSearchParamsEquivalent(');
    expect(infrastructurePageRouteStateSource).toContain('useLocation');
    expect(infrastructurePageRouteStateSource).toContain('useNavigate');
    expect(infrastructurePageRouteStateSource).toContain('parseInfrastructureLinkSearch');
    expect(infrastructurePageRouteStateSource).toContain('buildInfrastructurePath');
    expect(infrastructurePageRouteStateSource).toContain('areSearchParamsEquivalent');
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
    expect(alertOverviewTabSource).toContain('useAlertIncidentTimelineState');
    expect(alertOverviewTabSource).toContain('useAlertOverviewState');
    expect(alertIncidentEventFiltersSource).toContain('getAlertIncidentEventFilterContainerClass');
    expect(alertIncidentEventFiltersSource).toContain('getAlertIncidentEventFilterChipClass');
    expect(alertIncidentEventFiltersSource).toContain(
      'getAlertIncidentEventFilterActionButtonClass',
    );
    expect(alertIncidentTimelineStateSource).toContain(
      'export function useAlertIncidentTimelineState',
    );
    expect(alertIncidentTimelineStateSource).toContain('AlertsAPI.getIncidentTimeline');
    expect(alertIncidentTimelineStateSource).toContain('AlertsAPI.addIncidentNote');
    expect(alertIncidentTimelineStateSource).toContain('getAlertResourceIncidentTimelineFailure');
    expect(alertIncidentTimelineStateSource).toContain('getAlertResourceIncidentNoteSavedLabel');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertTimelineLoadingState');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertTimelineFilterEmptyState');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertTimelineEmptyState');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertTimelineUnavailableState');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertTimelineFailureState');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertIncidentAcknowledgedBadgeClass');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertIncidentNoteTextareaClass');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertIncidentNoteSaveButtonClass');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertIncidentTimelineMetaRowClass');
    expect(alertIncidentTimelinePanelSource).toContain('getAlertIncidentTimelineHeadingClass');
    expect(alertIncidentTimelinePanelSource).toContain('IncidentTimelineEventCard');
    expect(alertOverviewTabSource).not.toContain('getAlertIncidentTimelineEventCardClass');
    expect(alertOverviewTabSource).not.toContain('getAlertIncidentTimelineDetailClass');
    expect(alertOverviewTabSource).not.toContain('getAlertIncidentTimelineCommandClass');
    expect(alertOverviewTabSource).not.toContain('getAlertIncidentTimelineOutputClass');
    expect(alertOverviewTabSource).not.toContain('IncidentTimelinePanel');
    expect(alertOverviewTabSource).not.toContain('getAlertOverviewCardPresentation');
    expect(alertOverviewTabSource).not.toContain('getAlertOverviewAcknowledgedBadgeClass');
    expect(alertOverviewTabSource).not.toContain('getAlertOverviewStartedAtClass');
    expect(alertOverviewTabSource).not.toContain('getAlertOverviewPrimaryActionClass');
    expect(alertOverviewTabSource).not.toContain('getAlertOverviewSecondaryActionClass');
    expect(alertOverviewTabSource).not.toContain('AlertsAPI.bulkAcknowledge');
    expect(alertOverviewTabSource).not.toContain('AlertsAPI.acknowledge');
    expect(alertOverviewTabSource).not.toContain('AlertsAPI.unacknowledge');
    expect(alertOverviewTabSource).not.toContain('AlertsAPI.getIncidentTimeline');
    expect(alertOverviewTabSource).not.toContain('AlertsAPI.addIncidentNote');
    expect(alertOverviewTabSource).not.toContain('Loading timeline...');
    expect(alertOverviewTabSource).not.toContain('No timeline events match the selected filters.');
    expect(alertOverviewTabSource).not.toContain('No timeline events yet.');
    expect(alertOverviewTabSource).not.toContain('No incident timeline available.');
    expect(alertOverviewTabSource).not.toContain('Failed to load timeline.');
    expect(alertOverviewStateSource).toContain('export function useAlertOverviewState');
    expect(alertOverviewStateSource).toContain('export type AlertOverviewState');
    expect(alertOverviewStateSource).toContain('useAlertAcknowledgementState');
    expect(alertOverviewStateSource).not.toContain('AlertsAPI.bulkAcknowledge');
    expect(alertOverviewStateSource).not.toContain('AlertsAPI.acknowledge');
    expect(alertOverviewStateSource).not.toContain('AlertsAPI.unacknowledge');
    expect(alertOverviewStatsCardsSource).toContain('props.state.alertStats().acknowledged');
    expect(alertOverviewStatsCardsSource).toContain('props.state.alertStats().total24h');
    expect(alertOverviewStatsCardsSource).toContain('props.state.alertStats().overrides');
    expect(alertOverviewActiveAlertsSectionSource).toContain('AlertOverviewAlertCard');
    expect(alertOverviewActiveAlertsSectionSource).toContain('getAlertListEmptyState');
    expect(alertOverviewAlertCardSource).toContain('IncidentTimelinePanel');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewCardPresentation');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewAcknowledgedBadgeClass');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewStartedAtClass');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewPrimaryActionClass');
    expect(alertOverviewAlertCardSource).toContain('getAlertOverviewSecondaryActionClass');
    expect(alertAcknowledgementStateSource).toContain('export function useAlertAcknowledgementState');
    expect(alertAcknowledgementStateSource).toContain('AlertsAPI.bulkAcknowledge');
    expect(alertAcknowledgementStateSource).toContain('AlertsAPI.acknowledge');
    expect(alertAcknowledgementStateSource).toContain('AlertsAPI.unacknowledge');
    expect(recentAlertsPanelSource).toContain('useAlertAcknowledgementState');
    expect(recentAlertsPanelSource).not.toContain('AlertsAPI.bulkAcknowledge');
    expect(recentAlertsPanelSource).not.toContain('AlertsAPI.acknowledge');
    expect(alertScheduleTabSource).toContain('useAlertScheduleState');
    expect(alertScheduleTabSource).not.toContain('createDefaultQuietHours');
    expect(alertScheduleTabSource).not.toContain('createDefaultCooldown');
    expect(alertScheduleTabSource).not.toContain('createDefaultGrouping');
    expect(alertScheduleTabSource).not.toContain('createDefaultEscalation');
    expect(alertScheduleTabSource).not.toContain('const timezones = [');
    expect(alertScheduleTabSource).not.toContain('const days = [');
    expect(alertScheduleStateSource).toContain('export function useAlertScheduleState');
    expect(alertScheduleStateSource).toContain('createDefaultQuietHours');
    expect(alertScheduleStateSource).toContain('createDefaultCooldown');
    expect(alertScheduleStateSource).toContain('createDefaultGrouping');
    expect(alertScheduleStateSource).toContain('createDefaultEscalation');
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertTimelineLoadingState',
    );
    expect(alertOverviewPresentationSource).toContain(
      'export function getAlertTimelineFilterEmptyState',
    );
    expect(alertOverviewPresentationSource).toContain('export function getAlertTimelineEmptyState');
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
    expect(alertIncidentTimelineEventCardSource).toContain(
      'export function IncidentTimelineEventCard',
    );
    expect(alertIncidentEventFiltersSource).toContain('export function IncidentEventFilters');
    expect(alertIncidentTimelinePanelSource).toContain('export function IncidentTimelinePanel');
    expect(alertIncidentTimelineEventCardSource).toContain(
      'getAlertIncidentTimelineEventCardClass',
    );
    expect(alertIncidentTimelineEventCardSource).toContain('getAlertIncidentTimelineMetaRowClass');
    expect(alertIncidentTimelineEventCardSource).toContain('getAlertIncidentTimelineHeadingClass');
    expect(alertIncidentTimelineEventCardSource).toContain('getAlertIncidentTimelineDetailClass');
    expect(alertIncidentTimelineEventCardSource).toContain('getAlertIncidentTimelineCommandClass');
    expect(alertIncidentTimelineEventCardSource).toContain('getAlertIncidentTimelineOutputClass');
    expect(alertOverviewPresentationSource).toContain('export function getAlertHistoryEmptyState');
    expect(alertOverviewPresentationSource).toContain('export function getAlertsPageHeaderMeta');
    expect(alertOverviewPresentationSource).toContain('export function getAlertBucketCountLabel');
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

  it('keeps alerts configuration tabs feature-owned instead of page-local', () => {
    expect(alertsPageSource).toContain(
      "import { AlertsConfigurationSurface } from '@/features/alerts/AlertsConfigurationSurface';",
    );
    expect(alertsConfigurationSurfaceSource).toContain(
      "import { DestinationsTab } from './tabs/DestinationsTab';",
    );
    expect(alertsConfigurationSurfaceSource).toContain(
      "import { ScheduleTab } from './tabs/ScheduleTab';",
    );
    expect(alertsConfigurationSurfaceSource).toContain('useAlertsConfigurationState');
    expect(alertsConfigurationStateSource).toContain('buildAlertsConfigurationPayload');
    expect(alertsPageSource).not.toContain('function DestinationsTab(');
    expect(alertsPageSource).not.toContain('function ScheduleTab(');
    expect(alertDestinationsTabSource).toContain('useAlertDestinationsTabState');
    expect(alertDestinationsTabSource).not.toContain('NotificationsAPI.getWebhooks');
    expect(alertDestinationsTabSource).toContain('AlertEmailDestinationsSection');
    expect(alertDestinationsTabSource).toContain('AlertWebhookDestinationsSection');
    expect(alertScheduleTabSource).toContain('getAlertConfigQuietHourSuppressOptions');
    expect(alertScheduleTabSource).toContain('AlertGroupingSection');
    expect(alertScheduleTabSource).toContain('AlertQuietHoursSection');
  });

  it('keeps alert resource table vocabulary in a shared presentation utility', () => {
    expect(alertResourceTableSource).toContain('useAlertResourceTableState');
    expect(alertResourceTableSource).toContain('AlertResourceTableDesktop');
    expect(alertResourceTableSource).toContain('AlertResourceTableMobile');
    expect(alertResourceTableSource).not.toContain('const flattenResources = (): Resource[] => {');
    expect(alertResourceTableSource).not.toContain(
      'const normalizeMetricKey = (column: string): string => {',
    );
    expect(alertResourceTableSource).not.toContain(
      'const metricBounds = (metric: string): { min: number; max: number } => {',
    );
    expect(alertResourceTableSource).not.toContain('No resources available.');
    expect(alertResourceTableSource).not.toContain('No {props.title.toLowerCase()} found');
    expect(alertResourceTableSource).not.toContain('Add a note about this override (optional)');
    expect(alertResourceTableSource).not.toContain('Add a note...');
    expect(alertResourceTableSource).not.toContain('Reset to factory defaults');
    expect(alertResourceTableSource).not.toContain('Alert Delay (s)');
    expect(alertResourceTableSource).not.toContain('Click to edit this metric');
    expect(alertResourceTableSource).not.toContain('Set to -1 to disable alerts for this metric');
    expect(alertResourceTableSource).not.toContain(
      "if (resource.type === 'agent' && ['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(",
    );
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
    expect(alertResourceTableStateSource).toContain('export function useAlertResourceTableState');
    expect(alertResourceTableStateSource).toContain('toggleAll');
    expect(alertResourceTableStateSource).toContain('clearSelectedIds');
    expect(alertResourceGroupHeaderSource).toContain(
      'export function AlertResourceGroupHeader',
    );
    expect(alertResourceGroupHeaderSource).toContain('meta?.clusterName');
    expect(alertResourceTableDesktopSource).toContain(
      'export function AlertResourceTableDesktop',
    );
    expect(alertResourceTableDesktopSource).toContain('AlertResourceTableRow');
    expect(alertResourceTableDesktopSource).toContain('AlertResourceGroupHeader');
    expect(alertResourceTableDesktopSource).toContain('getAlertResourceTableCustomBadgeLabel');
    expect(alertResourceTableDesktopSource).toContain('getAlertResourceTableResetFactoryDefaultsLabel');
    expect(alertResourceTableDesktopSource).toContain('getAlertResourceTableAlertDelayLabel');
    expect(alertResourceTableDesktopSource).toContain('getAlertResourceTableMetricInputTitle');
    expect(alertResourceTableDesktopSource).toContain('getAlertResourceTableEmptyState');
    expect(alertResourceTableDesktopSource).toContain('getAlertResourceTableNoResultsState');
    expect(alertResourceTableMobileSource).toContain(
      'export function AlertResourceTableMobile',
    );
    expect(alertResourceTableMobileSource).toContain('AlertResourceGroupHeader');
    expect(alertResourceTableMobileSource).toContain('buildAlertResourceEditPayload');
    expect(alertResourceTableMobileSource).toContain('getAlertResourceTableCustomBadgeLabel');
    expect(alertResourceTableMobileSource).toContain('getAlertResourceTableMetricPlaceholder');
    expect(alertResourceTableMobileSource).toContain('getAlertResourceTableEmptyState');
    expect(alertResourceTableRowSource).toContain('export function AlertResourceTableRow');
    expect(alertResourceTableRowSource).toContain('alertResourceSupportsMetric');
    expect(alertResourceTableRowSource).toContain('getAlertResourceTableOverrideNotePlaceholder');
    expect(alertResourceTableRowSource).toContain('getAlertResourceTableMetricInputTitle');
    expect(alertResourceTableRowSource).toContain('getAlertResourceTableEditMetricTitle');
    expect(alertResourceTableModelSource).toContain(
      'export function normalizeAlertResourceMetricKey',
    );
    expect(alertResourceTableModelSource).toContain(
      'export function getAlertResourceMetricDisplayValue',
    );
    expect(alertResourceTableModelSource).toContain(
      'export function alertResourceSupportsMetric',
    );
    expect(alertResourceTableModelSource).toContain(
      'export function buildAlertResourceEditPayload',
    );
  });

  it('keeps alert email provider vocabulary and placeholders in a shared presentation utility', () => {
    expect(emailProviderSelectSource).toContain('useEmailProviderSelectState');
    expect(emailProviderSelectSource).toContain('getAlertEmailProviderOptionLabel');
    expect(emailProviderSelectSource).toContain('getAlertEmailSetupInstructionsToggleLabel');
    expect(emailProviderSelectSource).toContain('getAlertEmailRecipientsPlaceholder');
    expect(emailProviderSelectSource).toContain('getAlertEmailAdvancedToggleLabel');
    expect(emailProviderSelectSource).toContain('getAlertEmailTestButtonLabel');
    expect(emailProviderSelectSource).not.toContain('NotificationsAPI.getEmailProviders');
    expect(emailProviderSelectSource).not.toContain('interface EmailProvider {');
    expect(emailProviderSelectSource).not.toContain('interface EmailConfig {');
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
    expect(alertEmailPresentationSource).toContain('export function getAlertEmailTestButtonLabel');
    expect(emailProviderSelectStateSource).toContain('export function useEmailProviderSelectState');
    expect(emailProviderSelectStateSource).toContain('NotificationsAPI.getEmailProviders');
    expect(emailProviderSelectStateSource).toContain("provider.name === 'SendGrid'");
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
    expect(alertBulkEditPresentationSource).toContain('export const ALERT_BULK_EDIT_DIALOG_TITLE');
    expect(alertBulkEditPresentationSource).toContain(
      'export function getAlertBulkEditDescription',
    );
    expect(alertBulkEditPresentationSource).toContain('export function getAlertBulkEditApplyLabel');
    expect(alertBulkEditPresentationSource).toContain('export function getAlertBulkEditOpenLabel');
  });

  it('keeps alert webhook service vocabulary and action copy in a shared presentation utility', () => {
    expect(webhookConfigSource).toContain('useWebhookConfigState');
    expect(webhookConfigSource).toContain('WebhookConfigList');
    expect(webhookConfigSource).toContain('WebhookConfigForm');
    expect(webhookConfigSource).not.toContain('NotificationsAPI.getWebhookTemplates');
    expect(webhookConfigSource).not.toContain('const saveWebhook = () => {');
    expect(webhookConfigSource).not.toContain('const editWebhook = (webhook: Webhook) => {');
    expect(webhookConfigSource).not.toContain(
      'const toggleAllWebhooks = (enabled: boolean) => {',
    );
    expect(webhookConfigSource).not.toContain('Custom webhook endpoint');
    expect(webhookConfigSource).not.toContain('Discord server webhook');
    expect(webhookConfigSource).not.toContain('My Webhook');
    expect(webhookConfigSource).not.toContain('https://example.com/webhook');
    expect(webhookConfigSource).not.toContain('Optional — tag users or groups');
    expect(webhookConfigSource).not.toContain('getAlertWebhookMentionPlaceholder(');
    expect(webhookConfigSource).not.toContain('getAlertWebhookMentionHelp(');
    expect(webhookConfigSource).not.toContain('interface WebhookTemplate');
    expect(webhookConfigSource).not.toContain('Your Pushover application token');
    expect(webhookConfigSource).not.toContain('Primary user key or group key');
    expect(webhookConfigSource).not.toContain('app_token');
    expect(webhookConfigSource).not.toContain('user_token');
    expect(webhookConfigSource).not.toContain('createCustomFieldInputs');
    expect(webhookConfigSource).not.toContain('ensurePresetCustomFields');
    expect(webhookConfigSource).not.toContain('Enable All');
    expect(webhookConfigSource).not.toContain('Disable All');
    expect(webhookConfigSource).not.toContain('Enable this webhook');
    expect(webhookConfigListSource).toContain('getAlertWebhookServiceLabelFromTemplates');
    expect(webhookConfigListSource).toContain('getAlertWebhookSummaryLabel');
    expect(webhookConfigListSource).toContain('getAlertWebhookToggleAllLabel');
    expect(webhookConfigListSource).toContain('getAlertWebhookToggleLabel');
    expect(webhookConfigListSource).toContain('getAlertWebhookTestLabel');
    expect(webhookConfigFormSource).toContain('getAlertWebhookServices');
    expect(webhookConfigFormSource).toContain('getAlertWebhookNamePlaceholder');
    expect(webhookConfigFormSource).toContain('getAlertWebhookUrlPlaceholder');
    expect(webhookConfigFormSource).toContain(
      'getAlertWebhookMentionPlaceholderFromTemplates',
    );
    expect(webhookConfigFormSource).toContain('getAlertWebhookMentionHelpFromTemplates');
    expect(webhookConfigFormSource).toContain('hasAlertWebhookMentionSupportFromTemplates');
    expect(webhookConfigFormSource).toContain('getAlertWebhookTestLabel');
    expect(webhookConfigFormSource).toContain('getAlertWebhookSubmitLabel');
    expect(webhookConfigStateSource).toContain('NotificationsAPI.getWebhookTemplates');
    expect(webhookConfigStateSource).toContain('getAlertWebhookCustomFieldInputs');
    expect(webhookConfigStateSource).toContain('normalizeAlertWebhookCustomFields');
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookServiceLabelFromTemplates',
    );
    expect(alertWebhookPresentationSource).toContain('export function getAlertWebhookServices');
    expect(alertWebhookPresentationSource).not.toContain('ALERT_WEBHOOK_SERVICES');
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookMentionPlaceholderFromTemplates',
    );
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookMentionHelpFromTemplates',
    );
    expect(alertWebhookPresentationSource).toContain(
      'export function hasAlertWebhookMentionSupportFromTemplates',
    );
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookCustomFieldPresets',
    );
    expect(alertWebhookPresentationSource).toContain(
      'export function getAlertWebhookCustomFieldInputs',
    );
    expect(alertWebhookPresentationSource).toContain(
      'export function normalizeAlertWebhookCustomFields',
    );
    expect(alertWebhookPresentationSource).toContain('export function getAlertWebhookTestLabel');
    expect(alertWebhookPresentationSource).toContain('export function getAlertWebhookSubmitLabel');
  });
});
