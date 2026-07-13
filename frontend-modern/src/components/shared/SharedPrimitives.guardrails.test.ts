import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import calloutCardSource from '@/components/shared/CalloutCard.tsx?raw';
import emptyStateSource from '@/components/shared/EmptyState.tsx?raw';
import inlineNoticeSource from '@/components/shared/InlineNotice.tsx?raw';
import demoBannerSource from '@/components/DemoBanner.tsx?raw';
import commercialMigrationBannerSource from '@/components/CommercialMigrationBanner.tsx?raw';
import gitHubStarBannerSource from '@/components/GitHubStarBanner.tsx?raw';
import assistantCommandHelpDialogSource from '@/components/AI/Chat/AssistantCommandHelpDialog.tsx?raw';
import chatMessagesSource from '@/components/AI/Chat/ChatMessages.tsx?raw';
import aiChatSource from '@/components/AI/Chat/index.tsx?raw';
import messageItemSource from '@/components/AI/Chat/MessageItem.tsx?raw';
import toolExecutionBlockSource from '@/components/AI/Chat/ToolExecutionBlock.tsx?raw';
import findingsPanelSource from '@/components/AI/FindingsPanel.tsx?raw';
import aiModelPickerSource from '@/components/shared/AIModelPicker.tsx?raw';
import buttonSource from '@/components/shared/Button.tsx?raw';
import buttonModelSource from '@/components/shared/buttonModel.ts?raw';
import copyableCodeRowSource from '@/components/shared/CopyableCodeRow.tsx?raw';
import detailSectionTableSource from '@/components/shared/DetailSectionTable.tsx?raw';
import detailSectionModelSource from '@/components/shared/detailSectionModel.ts?raw';
import externalTextLinkSource from '@/components/shared/ExternalTextLink.tsx?raw';
import commandPaletteModalSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import commandPaletteModelSource from '@/components/shared/commandPaletteModel.ts?raw';
import columnPickerSource from '@/components/shared/ColumnPicker.tsx?raw';
import columnPickerModelSource from '@/components/shared/columnPickerModel.ts?raw';
import tagInputSource from '@/components/shared/TagInput.tsx?raw';
import tagInputModelSource from '@/components/shared/tagInputModel.ts?raw';
import containerUpdateBadgeSource from '@/components/shared/ContainerUpdateBadge.tsx?raw';
import containerUpdateBadgeModelSource from '@/components/shared/containerUpdateBadgeModel.ts?raw';
import dialogSource from '@/components/shared/Dialog.tsx?raw';
import dialogModelSource from '@/components/shared/dialogModel.ts?raw';
import filterButtonGroupSource from '@/components/shared/FilterButtonGroup.tsx?raw';
import filterButtonGroupModelSource from '@/components/shared/filterButtonGroupModel.ts?raw';
import selectablePillButtonSource from '@/components/shared/SelectablePillButton.tsx?raw';
import selectablePillModelSource from '@/components/shared/selectablePillModel.ts?raw';
import filterToolbarSource from '@/components/shared/FilterToolbar.tsx?raw';
import filterOptionPresentationSource from '@/components/shared/filterOptionPresentation.ts?raw';
import formSelectSource from '@/components/shared/FormSelect.tsx?raw';
import formTextareaSource from '@/components/shared/FormTextarea.tsx?raw';
import helpIconSource from '@/components/shared/HelpIcon.tsx?raw';
import helpIconModelSource from '@/components/shared/helpIconModel.ts?raw';
import historyChartHeaderSource from '@/components/shared/HistoryChartHeader.tsx?raw';
import historyChartOverlaySource from '@/components/shared/HistoryChartOverlay.tsx?raw';
import historyChartSource from '@/components/shared/HistoryChart.tsx?raw';
import historyChartModelSource from '@/components/shared/historyChartModel.ts?raw';
import historyChartTooltipSource from '@/components/shared/HistoryChartTooltip.tsx?raw';
import mobileNavBarSource from '@/components/shared/MobileNavBar.tsx?raw';
import mobileNavBarModelSource from '@/components/shared/mobileNavBarModel.ts?raw';
import pulseDataGridSource from '@/components/shared/PulseDataGrid.tsx?raw';
import pulseDataGridModelSource from '@/components/shared/pulseDataGridModel.ts?raw';
import progressBarSource from '@/components/shared/ProgressBar.tsx?raw';
import searchFieldSource from '@/components/shared/SearchField.tsx?raw';
import searchFieldModelSource from '@/components/shared/searchFieldModel.ts?raw';
import searchInputSource from '@/components/shared/SearchInput.tsx?raw';
import searchInputEnhancementsSource from '@/components/shared/SearchInputEnhancements.tsx?raw';
import searchInputEnhancementsModelSource from '@/components/shared/searchInputEnhancementsModel.ts?raw';
import searchInputModelSource from '@/components/shared/searchInputModel.ts?raw';
import statusDotSource from '@/components/shared/StatusDot.tsx?raw';
import loadingSpinnerSource from '@/components/shared/LoadingSpinner.tsx?raw';
import discoveryLoadingFallbackSource from '@/components/shared/DiscoveryLoadingFallback.tsx?raw';
import settingsLoadingSkeletonSource from '@/components/shared/SettingsLoadingSkeleton.tsx?raw';
import statusBadgeSource from '@/components/shared/StatusBadge.tsx?raw';
import statusBadgeModelSource from '@/components/shared/statusBadgeModel.ts?raw';
import statusIndicatorBadgeSource from '@/components/shared/StatusIndicatorBadge.tsx?raw';
import alertSeverityBadgeSource from '@/components/shared/AlertSeverityBadge.tsx?raw';
import metadataBadgeSource from '@/components/shared/MetadataBadge.tsx?raw';
import organizationBadgesSource from '@/components/shared/OrganizationBadges.tsx?raw';
import discoveryReadinessBadgeSource from '@/components/shared/DiscoveryReadinessBadge.tsx?raw';
import subtabsSource from '@/components/shared/Subtabs.tsx?raw';
import actionsSource from '@/pages/Actions.tsx?raw';
import toggleSource from '@/components/shared/Toggle.tsx?raw';
import toggleModelSource from '@/components/shared/toggleModel.ts?raw';
import searchTipsPopoverSource from '@/components/shared/SearchTipsPopover.tsx?raw';
import searchTipsPopoverModelSource from '@/components/shared/searchTipsPopoverModel.ts?raw';
import tooltipSource from '@/components/shared/Tooltip.tsx?raw';
import tooltipPortalSource from '@/components/shared/TooltipPortal.tsx?raw';
import tooltipModelSource from '@/components/shared/tooltipModel.ts?raw';
import upgradeLinkSource from '@/components/shared/UpgradeLink.tsx?raw';
import contextualFocusSource from '@/components/shared/contextualFocus.ts?raw';
import summaryCardInteractionSource from '@/components/shared/summaryCardInteraction.ts?raw';
import summaryRowActionButtonSource from '@/components/shared/SummaryRowActionButton.tsx?raw';
import summaryInteractionA11ySource from '@/components/shared/summaryInteractionA11y.ts?raw';
import tableSource from '@/components/shared/Table.tsx?raw';
import tableCardHeaderSource from '@/components/shared/TableCardHeader.tsx?raw';
import summaryTableFocusSource from '@/components/shared/summaryTableFocus.ts?raw';
import tableCardSource from '@/components/shared/TableCard.tsx?raw';
import groupedTableModeSegmentedControlSource from '@/components/shared/GroupedTableModeSegmentedControl.tsx?raw';
import groupedTableRowPresentationSource from '@/components/shared/groupedTableRowPresentation.ts?raw';
import animatedNumberSource from '@/components/shared/AnimatedNumber.tsx?raw';
import animatedNumberModelSource from '@/components/shared/animatedNumberModel.ts?raw';
import animatedNumberStateSource from '@/components/shared/useAnimatedNumberState.ts?raw';
import workloadsFilterSource from '@/components/Workloads/WorkloadsFilter.tsx?raw';
import infrastructureSourceManagerSource from '@/components/Settings/InfrastructureSourceManager.tsx?raw';
import selectionCardGroupSource from '@/components/shared/SelectionCardGroup.tsx?raw';
import selectionCardGroupModelSource from '@/components/shared/selectionCardGroupModel.ts?raw';
import summaryChartLayoutSource from '@/components/shared/summaryChartLayout.ts?raw';
import tagBadgesSource from '@/components/shared/TagBadges.tsx?raw';
import tlsVerificationWarningBannerSource from '@/components/shared/TlsVerificationWarningBanner.tsx?raw';
import commandPaletteStateSource from '@/components/shared/useCommandPaletteState.ts?raw';
import columnPickerStateSource from '@/components/shared/useColumnPickerState.ts?raw';
import tagInputStateSource from '@/components/shared/useTagInputState.ts?raw';
import containerUpdateButtonStateSource from '@/components/shared/useContainerUpdateButtonState.ts?raw';
import dialogStateSource from '@/components/shared/useDialogState.ts?raw';
import filterButtonGroupStateSource from '@/components/shared/useFilterButtonGroupState.ts?raw';
import helpIconStateSource from '@/components/shared/useHelpIconState.ts?raw';
import historyChartStateSource from '@/components/shared/useHistoryChartState.ts?raw';
import mobileNavBarStateSource from '@/components/shared/useMobileNavBarState.ts?raw';
import pulseDataGridStateSource from '@/components/shared/usePulseDataGridState.ts?raw';
import searchFieldStateSource from '@/components/shared/useSearchFieldState.ts?raw';
import searchInputStateSource from '@/components/shared/useSearchInputState.ts?raw';
import searchInputEnhancementsStateSource from '@/components/shared/useSearchInputEnhancements.ts?raw';
import statusBadgeStateSource from '@/components/shared/useStatusBadgeState.ts?raw';
import toggleStateSource from '@/components/shared/useToggleState.ts?raw';
import searchTipsPopoverStateSource from '@/components/shared/useSearchTipsPopoverState.ts?raw';
import tooltipStateSource from '@/components/shared/useTooltipState.ts?raw';
import upgradeNavigationHookSource from '@/components/shared/useUpgradeNavigation.ts?raw';
import selectionCardGroupStateSource from '@/components/shared/useSelectionCardGroupState.ts?raw';
import webInterfaceUrlFieldSource from '@/components/shared/WebInterfaceUrlField.tsx?raw';
import webInterfaceUrlFieldModelSource from '@/components/shared/webInterfaceUrlFieldModel.ts?raw';
import webInterfaceUrlFieldStateSource from '@/components/shared/useWebInterfaceUrlFieldState.ts?raw';
import webInterfaceNameLinkSource from '@/components/shared/WebInterfaceNameLink.tsx?raw';
import inlineDetailTableRowSource from '@/components/shared/InlineDetailTableRow.tsx?raw';
import toastSource from '@/components/Toast/Toast.tsx?raw';
import sharedTemplateRegistrySource from '../../../scripts/shared-template-registry.json?raw';
import discoveryTabSource from '@/components/Discovery/DiscoveryTab.tsx?raw';
import emailProviderSelectSource from '@/components/Alerts/EmailProviderSelect.tsx?raw';
import errorBoundarySource from '@/components/ErrorBoundary.tsx?raw';
import updateConfirmationModalSource from '@/components/UpdateConfirmationModal.tsx?raw';
import updateProgressModalSource from '@/components/UpdateProgressModal.tsx?raw';
import incidentTimelinePanelSource from '@/components/Alerts/IncidentTimelinePanel.tsx?raw';
import alertDetailPresentationSource from '@/utils/alertDetailPresentation.ts?raw';
import alertSeverityPresentationSource from '@/utils/alertSeverityPresentation.ts?raw';
import thresholdsTableDockerIgnoredPrefixesSectionSource from '@/components/Alerts/ThresholdsTableDockerIgnoredPrefixesSection.tsx?raw';
import webhookConfigFormSource from '@/components/Alerts/WebhookConfigForm.tsx?raw';
import reportMergeModalSource from '@/components/Infrastructure/ReportMergeModal.tsx?raw';
import availabilitySettingsPanelSource from '@/components/Settings/AvailabilitySettingsPanel.tsx?raw';
import billingAdminOrganizationsTableSource from '@/components/Settings/BillingAdminOrganizationsTable.tsx?raw';
import addressProbeStepSource from '@/components/Settings/ConnectionEditor/AddressProbeStep.tsx?raw';
import connectionEditorSource from '@/components/Settings/ConnectionEditor/ConnectionEditor.tsx?raw';
import availabilityTargetSlotSource from '@/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx?raw';
import trueNASCredentialSlotSource from '@/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx?raw';
import vmwareCredentialSlotSource from '@/components/Settings/ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx?raw';
import copyCommandBlockSource from '@/components/Settings/CopyCommandBlock.tsx?raw';
import infrastructureAgentUpdatesDialogSource from '@/components/Settings/InfrastructureAgentUpdatesDialog.tsx?raw';
import infrastructureDiscoverySettingsDialogSource from '@/components/Settings/InfrastructureDiscoverySettingsDialog.tsx?raw';
import selfHostedCommercialRecoverySectionSource from '@/components/Settings/SelfHostedCommercialRecoverySection.tsx?raw';
import suggestProfileModalSource from '@/components/Settings/SuggestProfileModal.tsx?raw';
import alertAppriseDestinationsSectionSource from '@/features/alerts/AlertAppriseDestinationsSection.tsx?raw';
import dockerPageSurfaceSource from '@/features/docker/DockerPageSurface.tsx?raw';
import dockerHostDrawerSource from '@/features/docker/DockerHostDrawer.tsx?raw';
import dockerServicesTableSource from '@/features/docker/DockerServicesTable.tsx?raw';
import kubernetesPageSurfaceSource from '@/features/kubernetes/KubernetesPageSurface.tsx?raw';
import proxmoxPageSurfaceSource from '@/features/proxmox/ProxmoxPageSurface.tsx?raw';
import standalonePageSurfaceSource from '@/features/standalone/StandalonePageSurface.tsx?raw';
import sharedPlatformPageSource from '@/features/platformPage/sharedPlatformPage.tsx?raw';
import platformAlertSeverityFilterOptionsSource from '@/features/platformPage/platformAlertSeverityFilterOptions.tsx?raw';
import platformResourceDetailTableRowSource from '@/features/platformPage/PlatformResourceDetailTableRow.tsx?raw';
import platformOutdatedAgentNoticeSource from '@/features/platformPage/PlatformOutdatedAgentNotice.tsx?raw';
import platformOutdatedSensorSetupNoticeSource from '@/features/platformPage/PlatformOutdatedSensorSetupNotice.tsx?raw';
import truenasPageSurfaceSource from '@/features/truenas/TrueNASPageSurface.tsx?raw';
import truenasProtectionTableSource from '@/features/truenas/TrueNASProtectionTable.tsx?raw';
import vmwarePageSurfaceSource from '@/features/vmware/VmwarePageSurface.tsx?raw';
import upgradeNavigationSource from '@/utils/upgradeNavigation.ts?raw';
import guestRowSource from '@/components/Workloads/GuestRow.tsx?raw';
import guestDrawerSource from '@/components/Workloads/GuestDrawer.tsx?raw';
import guestDrawerHistorySource from '@/components/Workloads/GuestDrawerHistory.tsx?raw';
import nodeDrawerSource from '@/components/Workloads/NodeDrawer.tsx?raw';
import workloadsSurfaceSource from '@/components/Workloads/WorkloadsSurface.tsx?raw';
import workloadsTableSource from '@/components/Workloads/WorkloadsTable.tsx?raw';
import workloadPanelSource from '@/components/Workloads/WorkloadPanel.tsx?raw';
import guestRowStateSource from '@/components/Workloads/useGuestRowState.ts?raw';
import workloadSelectionStateSource from '@/components/Workloads/useWorkloadSelectionState.ts?raw';
import dockerContainerTableModelSource from '@/features/docker/dockerContainerTableModel.ts?raw';
import dockerContainersTableSource from '@/features/docker/DockerContainersTable.tsx?raw';
import dockerHostsTableSource from '@/features/docker/DockerHostsTable.tsx?raw';
import dockerNativeTableSharedSource from '@/features/docker/DockerNativeTableShared.tsx?raw';
import dockerStorageUsageTableSource from '@/features/docker/DockerStorageUsageTable.tsx?raw';
import dockerVolumesTableSource from '@/features/docker/DockerVolumesTable.tsx?raw';
import kubernetesAutoscalingTableSource from '@/features/kubernetes/KubernetesAutoscalingTable.tsx?raw';
import kubernetesClustersTableSource from '@/features/kubernetes/KubernetesClustersTable.tsx?raw';
import kubernetesConfigTableSource from '@/features/kubernetes/KubernetesConfigTable.tsx?raw';
import kubernetesControllersTableSource from '@/features/kubernetes/KubernetesControllersTable.tsx?raw';
import kubernetesDeploymentsTableSource from '@/features/kubernetes/KubernetesDeploymentsTable.tsx?raw';
import kubernetesEventsTableSource from '@/features/kubernetes/KubernetesEventsTable.tsx?raw';
import kubernetesNetworkingTableSource from '@/features/kubernetes/KubernetesNetworkingTable.tsx?raw';
import kubernetesNodesTableSource from '@/features/kubernetes/KubernetesNodesTable.tsx?raw';
import kubernetesPodsTableSource from '@/features/kubernetes/KubernetesPodsTable.tsx?raw';
import kubernetesPolicyTableSource from '@/features/kubernetes/KubernetesPolicyTable.tsx?raw';
import kubernetesServicesTableSource from '@/features/kubernetes/KubernetesServicesTable.tsx?raw';
import kubernetesStorageTableSource from '@/features/kubernetes/KubernetesStorageTable.tsx?raw';
import proxmoxBackupServersTableSource from '@/features/proxmox/ProxmoxBackupServersTable.tsx?raw';
import proxmoxBackupsTableSource from '@/features/proxmox/ProxmoxBackupsTable.tsx?raw';
import proxmoxCephClusterDrawerSource from '@/features/proxmox/ProxmoxCephClusterDrawer.tsx?raw';
import proxmoxCephTableSource from '@/features/proxmox/ProxmoxCephTable.tsx?raw';
import proxmoxCoverageTableSource from '@/features/proxmox/ProxmoxCoverageTable.tsx?raw';
import proxmoxMailGatewayTableSource from '@/features/proxmox/ProxmoxMailGatewayTable.tsx?raw';
import proxmoxNodesTableSource from '@/features/proxmox/ProxmoxNodesTable.tsx?raw';
import proxmoxHostTableModelSource from '@/features/proxmox/proxmoxHostTableModel.ts?raw';
import proxmoxRecoverableTableSource from '@/features/proxmox/ProxmoxRecoverableTable.tsx?raw';
import proxmoxReplicationTableSource from '@/features/proxmox/ProxmoxReplicationTable.tsx?raw';
import vsphereHostsTableSource from '@/features/vmware/VsphereHostsTable.tsx?raw';
import availabilityChecksTableSource from '@/features/standalone/AvailabilityChecksTable.tsx?raw';
import agentsMachinesTableSource from '@/features/standalone/AgentsMachinesTable.tsx?raw';
import agentMachineTableModelSource from '@/features/standalone/agentMachineTableModel.ts?raw';
import unifiedResourceHostTableCardSource from '@/components/Infrastructure/UnifiedResourceHostTableCard.tsx?raw';
import unifiedResourceServiceInfrastructureCardSource from '@/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx?raw';
import unifiedResourcePBSTableSectionSource from '@/components/Infrastructure/UnifiedResourcePBSTableSection.tsx?raw';
import unifiedResourcePMGTableSectionSource from '@/components/Infrastructure/UnifiedResourcePMGTableSection.tsx?raw';
import proxmoxMailGatewayDrawerSource from '@/features/proxmox/ProxmoxMailGatewayDrawer.tsx?raw';
import swarmServicesDrawerSource from '@/components/Docker/SwarmServicesDrawer.tsx?raw';
import k8sDeploymentsDrawerSource from '@/components/Kubernetes/K8sDeploymentsDrawer.tsx?raw';
import k8sNamespacesDrawerSource from '@/components/Kubernetes/K8sNamespacesDrawer.tsx?raw';
import nodeGroupHeaderSource from '@/components/shared/NodeGroupHeader.tsx?raw';
import proxmoxVersionSource from '@/utils/proxmoxVersion.ts?raw';
import storageGroupRowSource from '@/components/Storage/StorageGroupRow.tsx?raw';
import storageGroupPresentationSource from '@/features/storageBackups/groupPresentation.ts?raw';
import storagePoolRowSource from '@/components/Storage/StoragePoolRow.tsx?raw';
import storageContentCardSource from '@/components/Storage/StorageContentCard.tsx?raw';
import storagePoolsTableSource from '@/components/Storage/StoragePoolsTable.tsx?raw';
import storagePoolDetailSource from '@/components/Storage/StoragePoolDetail.tsx?raw';
import diskListSource from '@/components/Storage/DiskList.tsx?raw';
import alertOverviewStatsCardsSource from '@/features/alerts/AlertOverviewStatsCards.tsx?raw';
import alertHistoryTableSectionSource from '@/features/alerts/AlertHistoryTableSection.tsx?raw';
import alertHistoryTableGroupRowSource from '@/features/alerts/AlertHistoryTableGroupRow.tsx?raw';
import alertResourceTableSource from '@/components/Alerts/ResourceTable.tsx?raw';
import alertResourceTableDesktopSource from '@/components/Alerts/AlertResourceTableDesktop.tsx?raw';
import alertResourceTableMobileSource from '@/components/Alerts/AlertResourceTableMobile.tsx?raw';
import alertResourceGroupHeaderSource from '@/components/Alerts/AlertResourceGroupHeader.tsx?raw';
import alertResourceTableRowSource from '@/components/Alerts/AlertResourceTableRow.tsx?raw';
import aiCostDashboardSource from '@/components/AI/AICostDashboard.tsx?raw';
import resourceDetailSummarySource from '@/components/Infrastructure/ResourceDetailSummary.tsx?raw';
import resourceDetailDrawerSource from '@/components/Infrastructure/ResourceDetailDrawer.tsx?raw';
import resourceDetailDrawerDebugTabSource from '@/components/Infrastructure/ResourceDetailDrawerDebugTab.tsx?raw';
import resourceDetailDrawerKubernetesModelSource from '@/components/Infrastructure/resourceDetailDrawerKubernetesModel.ts?raw';
import resourceDetailDrawerTrueNASModelSource from '@/components/Infrastructure/resourceDetailDrawerTrueNASModel.ts?raw';
import resourceDetailDrawerVmwareModelSource from '@/components/Infrastructure/resourceDetailDrawerVmwareModel.ts?raw';
import aiSettingsDialogsSource from '@/components/Settings/AISettingsDialogs.tsx?raw';
import aiProviderConfigurationSectionSource from '@/components/Settings/AIProviderConfigurationSection.tsx?raw';
import aiRuntimeControlsSectionSource from '@/components/Settings/AIRuntimeControlsSection.tsx?raw';
import agentIntegrationsPanelSource from '@/components/Settings/AgentIntegrationsPanel.tsx?raw';
import apiAccessPanelSource from '@/components/Settings/APIAccessPanel.tsx?raw';
import agentProfilesPanelSource from '@/components/Settings/AgentProfilesPanel.tsx?raw';
import apiTokenManagerSource from '@/components/Settings/APITokenManager.tsx?raw';
import auditWebhookPanelSource from '@/components/Settings/AuditWebhookPanel.tsx?raw';
import auditLogPanelSource from '@/components/Settings/AuditLogPanel.tsx?raw';
import billingAdminPanelSource from '@/components/Settings/BillingAdminPanel.tsx?raw';
import diagnosticsResultsPanelSource from '@/components/Settings/DiagnosticsResultsPanel.tsx?raw';
import discoverySettingsFormSource from '@/components/Settings/DiscoverySettingsForm.tsx?raw';
import dockerRuntimeSettingsCardSource from '@/components/Settings/DockerRuntimeSettingsCard.tsx?raw';
import dataHandlingPanelSource from '@/components/Settings/DataHandlingPanel.tsx?raw';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';
import infrastructureInstallerSectionSource from '@/components/Settings/InfrastructureInstallerSection.tsx?raw';
import infrastructureWorkspaceSource from '@/components/Settings/InfrastructureWorkspace.tsx?raw';
import monitoredSystemImpactPreviewSource from '@/components/Settings/MonitoredSystemImpactPreview.tsx?raw';
import organizationAccessManagementSectionSource from '@/components/Settings/OrganizationAccessManagementSection.tsx?raw';
import organizationAccessLoadingStateSource from '@/components/Settings/OrganizationAccessLoadingState.tsx?raw';
import organizationBillingLoadingStateSource from '@/components/Settings/OrganizationBillingLoadingState.tsx?raw';
import organizationOverviewDetailsSectionSource from '@/components/Settings/OrganizationOverviewDetailsSection.tsx?raw';
import organizationOverviewLoadingStateSource from '@/components/Settings/OrganizationOverviewLoadingState.tsx?raw';
import organizationSharingLoadingStateSource from '@/components/Settings/OrganizationSharingLoadingState.tsx?raw';
import organizationAccessInvitationsSectionSource from '@/components/Settings/OrganizationAccessInvitationsSection.tsx?raw';
import organizationAccessMembersSectionSource from '@/components/Settings/OrganizationAccessMembersSection.tsx?raw';
import organizationIncomingSharesSectionSource from '@/components/Settings/OrganizationIncomingSharesSection.tsx?raw';
import organizationOutgoingSharesSectionSource from '@/components/Settings/OrganizationOutgoingSharesSection.tsx?raw';
import organizationOverviewMembersSectionSource from '@/components/Settings/OrganizationOverviewMembersSection.tsx?raw';
import organizationSharingCreateSectionSource from '@/components/Settings/OrganizationSharingCreateSection.tsx?raw';
import proLicensePanelSource from '@/components/Settings/ProLicensePanel.tsx?raw';
import proLicensePlanSectionSource from '@/components/Settings/ProLicensePlanSection.tsx?raw';
import reportingPanelSource from '@/components/Settings/ReportingPanel.tsx?raw';
import rolesPanelSource from '@/components/Settings/RolesPanel.tsx?raw';
import securityAuthPanelSource from '@/components/Settings/SecurityAuthPanel.tsx?raw';
import securityOverviewPanelSource from '@/components/Settings/SecurityOverviewPanel.tsx?raw';
import ssoProvidersPanelSource from '@/components/Settings/SSOProvidersPanel.tsx?raw';
import updatesSettingsPanelSource from '@/components/Settings/UpdatesSettingsPanel.tsx?raw';
import userAssignmentsPanelSource from '@/components/Settings/UserAssignmentsPanel.tsx?raw';
import infrastructureSourcePickerSource from '@/components/Settings/InfrastructureSourcePicker.tsx?raw';
import resourcePickerSource from '@/components/Settings/ResourcePicker.tsx?raw';
import settingsPageShellSource from '@/components/Settings/SettingsPageShell.tsx?raw';
import loginSource from '@/components/Login.tsx?raw';
import patrolIntelligenceHeaderSource from '@/features/patrol/PatrolIntelligenceHeader.tsx?raw';
import patrolIntelligenceWorkspaceSource from '@/features/patrol/PatrolIntelligenceWorkspace.tsx?raw';
import approvalBannerSource from '@/components/patrol/ApprovalBanner.tsx?raw';
import approvalSectionSource from '@/components/patrol/ApprovalSection.tsx?raw';
import investigationMessagesSource from '@/components/patrol/InvestigationMessages.tsx?raw';
import investigationSectionSource from '@/components/patrol/InvestigationSection.tsx?raw';
import runHistoryPanelSource from '@/components/patrol/RunHistoryPanel.tsx?raw';
import runHistoryEntrySource from '@/components/patrol/RunHistoryEntry.tsx?raw';
import runToolCallTraceSource from '@/components/patrol/RunToolCallTrace.tsx?raw';
import filterBarSource from '@/components/shared/FilterBar/FilterBar.tsx?raw';
import filterChipSource from '@/components/shared/FilterBar/FilterChip.tsx?raw';
import featureGateSectionSource from '@/components/shared/FeatureGateSection.tsx?raw';
import addFilterMenuSource from '@/components/shared/FilterBar/AddFilterMenu.tsx?raw';
import filterCatalogSource from '@/components/shared/FilterBar/filterCatalog.ts?raw';
import filterBarOptionPresentationSource from '@/components/shared/FilterBar/filterOptionPresentation.tsx?raw';
import filterBarIndexSource from '@/components/shared/FilterBar/index.ts?raw';
import savedViewsMenuSource from '@/components/shared/FilterBar/SavedViewsMenu.tsx?raw';
import useSavedViewsSource from '@/components/shared/FilterBar/useSavedViews.ts?raw';
import storagePageControlsSource from '@/components/Storage/StoragePageControls.tsx?raw';
import orgSwitcherSource from '@/components/OrgSwitcher.tsx?raw';
import resourceDetailDrawerOverviewTabSource from '@/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx?raw';
import dockerAlertsTableSource from '@/features/docker/DockerAlertsTable.tsx?raw';
import kubernetesAlertsTableSource from '@/features/kubernetes/KubernetesAlertsTable.tsx?raw';
import proxmoxBackupsTableSharedSource from '@/features/proxmox/proxmoxBackupsTableShared.tsx?raw';
import truenasAlertsTableSource from '@/features/truenas/TrueNASAlertsTable.tsx?raw';
import truenasAppsTableSource from '@/features/truenas/TrueNASAppsTable.tsx?raw';
import truenasNetworkSharesTableSource from '@/features/truenas/TrueNASNetworkSharesTable.tsx?raw';
import truenasServicesTableSource from '@/features/truenas/TrueNASServicesTable.tsx?raw';
import truenasStorageTopologyTableSource from '@/features/truenas/TrueNASStorageTopologyTable.tsx?raw';
import truenasSystemsTableSource from '@/features/truenas/TrueNASSystemsTable.tsx?raw';
import truenasVirtualMachinesTableSource from '@/features/truenas/TrueNASVirtualMachinesTable.tsx?raw';
import vsphereActivityTableSource from '@/features/vmware/VsphereActivityTable.tsx?raw';
import vsphereAlertsTableSource from '@/features/vmware/VsphereAlertsTable.tsx?raw';
import vsphereDatastoresTableSource from '@/features/vmware/VsphereDatastoresTable.tsx?raw';
import vsphereNetworksTableSource from '@/features/vmware/VsphereNetworksTable.tsx?raw';

const sharedSources = import.meta.glob(['./*.tsx', './cards/*.tsx', './responsive/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;
const frontendIndexCssSource = readFileSync(join(process.cwd(), 'src/index.css'), 'utf8');
const readFrontendSource = (path: string) => readFileSync(join(process.cwd(), path), 'utf8');

describe('shared primitive guardrails', () => {
  it('limits raw Table composition inside shared primitives to the canonical allowlist', () => {
    const sharedRuntimeEntries = Object.entries(sharedSources).filter(
      ([path]) => !path.endsWith('.test.tsx') && !path.endsWith('.guardrails.test.ts'),
    );
    const tableImportPattern = /from\s*['"]@\/components\/shared\/Table['"]/;

    const rawTableUsers = sharedRuntimeEntries
      .filter(([, source]) => tableImportPattern.test(source))
      .map(([path]) => path)
      .sort();

    expect(rawTableUsers).toEqual(['./PulseDataGrid.tsx']);
  });

  it('keeps toast notification chrome on shared action and icon primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'toast-notification-shell');

    expect(registeredRule?.canonical?.path).toBe('src/components/Toast/Toast.tsx');
    expect(registeredRule?.canonical?.export).toBe('Toast');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Toast/Toast.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/Toast/Toast.tsx',
        patterns: [
          '<svg',
          'const icons = {',
          'flex-shrink-0 text-muted hover:text-base-content hover:bg-surface rounded-md p-1.5 transition-all duration-200',
        ],
      },
    ]);
    expect(toastSource).toContain('ActionIconButton');
    expect(toastSource).toContain('lucide-solid/icons/check-circle');
    expect(toastSource).toContain('lucide-solid/icons/circle-alert');
    expect(toastSource).toContain('lucide-solid/icons/alert-triangle');
    expect(toastSource).toContain('lucide-solid/icons/info');
    expect(toastSource).toContain('lucide-solid/icons/x');
    expect(toastSource).not.toContain('<svg');
    expect(toastSource).not.toContain('const icons = {');
    expect(toastSource).not.toContain(
      'flex-shrink-0 text-muted hover:text-base-content hover:bg-surface rounded-md p-1.5 transition-all duration-200',
    );
  });

  it('routes canonical settings and feature segmented selectors through FilterButtonGroup', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'filter-button-group-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'filter-button-group-local-segmented-control-styles',
    );
    const registeredFeatureGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'filter-button-group-local-feature-view-toggle-styles',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/FilterButtonGroup.tsx');
    expect(registeredRule?.canonical?.export).toBe('FilterButtonGroup');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/GeneralSettingsPanel.tsx',
      'src/components/Settings/ResourcePicker.tsx',
      'src/components/Settings/ReportingPanel.tsx',
      'src/features/patrol/PatrolIntelligenceHeader.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/components/Settings/ResourcePicker.tsx',
          patterns: expect.arrayContaining([
            'min-h-10 sm:min-h-9 min-w-10 px-3 py-2 rounded-md text-sm font-medium transition-all',
            '<For each={RESOURCE_PICKER_TYPE_FILTERS}>',
          ]),
        }),
        expect.objectContaining({
          path: 'src/features/proxmox/ProxmoxBackupsTable.tsx',
          patterns: expect.arrayContaining([
            'const viewButtonClass',
            'inline-flex items-center gap-1 rounded-md border border-border bg-surface p-1',
            'inline-flex min-h-8 items-center gap-1.5 rounded-sm px-3 text-xs font-medium transition-colors',
          ]),
        }),
      ]),
    );
    expect(registeredGuard?.canonical?.path).toBe(
      'src/components/shared/filterButtonGroupModel.ts',
    );
    expect(registeredGuard?.canonical?.export).toBe('getFilterButtonGroupButtonClass');
    expect(registeredGuard?.allPatterns).toEqual([
      'flex items-center bg-base rounded-md p-1 border shadow-inner',
      'flex-1 py-1.5 px-2 text-xs font-semibold rounded-md transition-all duration-200',
    ]);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components/Settings', 'src/features', 'src/pages']),
    );
    expect(registeredFeatureGuard?.canonical?.path).toBe(
      'src/components/shared/filterButtonGroupModel.ts',
    );
    expect(registeredFeatureGuard?.canonical?.export).toBe('getFilterButtonGroupButtonClass');
    expect(registeredFeatureGuard?.allPatterns).toEqual([
      'inline-flex items-center gap-1 rounded-md border border-border bg-surface p-1',
      'inline-flex min-h-8 items-center gap-1.5 rounded-sm px-3 text-xs font-medium transition-colors',
    ]);
    expect(registeredFeatureGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredFeatureGuard?.ignoredPaths).toEqual([
      'src/features/proxmox/__tests__/ProxmoxBackupsTable.test.tsx',
    ]);

    expect(filterButtonGroupSource).toContain('useFilterButtonGroupState');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupClass');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupButtonClass');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupCompactLabel');
    expect(filterButtonGroupSource).not.toContain('-webkit-overflow-scrolling: touch;');
    expect(filterButtonGroupSource).not.toContain("label.split(' ').pop()");
    expect(filterButtonGroupSource).not.toContain('props.onChange(option.value)');
    expect(filterButtonGroupStateSource).toContain('export function useFilterButtonGroupState');
    expect(filterButtonGroupStateSource).toContain('createMemo');
    expect(filterButtonGroupStateSource).toContain('props.disabled || option.disabled');
    expect(filterButtonGroupStateSource).toContain('props.onChange(option.value)');
    expect(filterButtonGroupModelSource).toContain("prominent: 'grid grid-cols-1 gap-2'");
    expect(filterButtonGroupModelSource).toContain('segmented:');
    expect(filterButtonGroupModelSource).toContain('touch-scroll');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupButtonClass');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupCompactLabel');
    expect(generalSettingsPanelSource).toContain('FilterButtonGroup');
    expect(generalSettingsPanelSource.match(/<FilterButtonGroup/g) ?? []).toHaveLength(4);
    expect(generalSettingsPanelSource).toContain('getLocalePreferenceOptions');
    expect(generalSettingsPanelSource).toContain(
      "ariaLabel={t('settings.general.language.ariaLabel')}",
    );
    expect(generalSettingsPanelSource).toContain('variant="prominent"');
    expect(generalSettingsPanelSource).not.toContain("props.themePreference() === 'light'");
    expect(generalSettingsPanelSource).not.toContain("temperatureStore.unit() === 'celsius'");
    expect(generalSettingsPanelSource).not.toContain(
      'props.pvePollingSelection() === option.value',
    );
    expect(resourcePickerSource).toContain('FilterButtonGroup');
    expect(resourcePickerSource).not.toContain('<For each={RESOURCE_PICKER_TYPE_FILTERS}>');
    expect(resourcePickerSource).not.toContain(
      'min-h-10 sm:min-h-9 min-w-10 px-3 py-2 rounded-md text-sm font-medium transition-all',
    );
    expect(reportingPanelSource.match(/<FilterButtonGroup/g) ?? []).toHaveLength(2);
    expect(reportingPanelSource).toContain('variant="prominent"');
    expect(reportingPanelSource).not.toContain('getReportingToggleButtonClass');
    expect(reportingPanelSource).not.toContain('<For each={REPORTING_RANGE_OPTIONS}>');
    expect(patrolIntelligenceHeaderSource).toContain('FilterButtonGroup');
    expect(patrolIntelligenceHeaderSource).toContain("variant={options.variant ?? 'segmented'}");
    expect(patrolIntelligenceHeaderSource).not.toContain("variant: 'prominent'");
    expect(patrolIntelligenceHeaderSource).toContain('selectedAutonomyPolicy');
    expect(patrolIntelligenceHeaderSource).not.toContain(
      'flex items-center bg-base rounded-md p-1 border shadow-inner',
    );
    expect(patrolIntelligenceHeaderSource).not.toContain(
      'flex-1 py-1.5 px-2 text-xs font-semibold rounded-md transition-all duration-200',
    );
    expect(proxmoxBackupsTableSource).toContain('FilterSegmentedControl');
    expect(proxmoxBackupsTableSource).not.toContain('FilterButtonGroup');
    expect(proxmoxBackupsTableSource).not.toContain('const viewButtonClass');
    expect(proxmoxBackupsTableSource).not.toContain(
      'inline-flex items-center gap-1 rounded-md border border-border bg-surface p-1',
    );
    expect(proxmoxBackupsTableSource).not.toContain(
      'inline-flex min-h-8 items-center gap-1.5 rounded-sm px-3 text-xs font-medium transition-colors',
    );
  });

  it('routes selectable scope pill buttons through SelectablePillButton', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'selectable-pill-button-shell',
    );
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'selectable-pill-button-local-scope-selector-styles',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/SelectablePillButton.tsx');
    expect(registeredRule?.canonical?.export).toBe('SelectablePillButton');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/APITokenManager.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/Settings/APITokenManager.tsx',
        patterns: [
          'inline-flex min-h-10 sm:min-h-10 items-center rounded-full border px-3 py-2 text-sm font-semibold transition',
          'min-h-10 sm:min-h-10 rounded-full border px-3 py-2 text-sm font-semibold transition',
          'border-blue-500 bg-blue-600 text-white shadow-sm',
          'hover:border-blue-400 hover:text-blue-600 dark:hover:border-blue-400 dark:hover:text-blue-200',
        ],
      },
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/selectablePillModel.ts');
    expect(registeredGuard?.canonical?.export).toBe('getSelectablePillButtonClass');
    expect(registeredGuard?.allPatterns).toEqual([
      'rounded-full border px-3 py-2 text-sm font-semibold transition',
      'border-blue-500 bg-blue-600 text-white shadow-sm',
      'hover:border-blue-400 hover:text-blue-600',
    ]);
    expect(registeredGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/Settings/__tests__/APITokenManager.test.tsx',
      'src/components/shared/SelectablePillButton.test.tsx',
    ]);

    expect(selectablePillButtonSource).toContain('getSelectablePillButtonClass');
    expect(selectablePillButtonSource).toContain("aria-pressed={local.active ? 'true' : 'false'}");
    expect(selectablePillButtonSource).not.toContain('border-blue-500 bg-blue-600');
    expect(selectablePillModelSource).toContain('SELECTABLE_PILL_BUTTON_BASE_CLASS');
    expect(selectablePillModelSource).toContain('SELECTABLE_PILL_BUTTON_ACTIVE_CLASS');
    expect(selectablePillModelSource).toContain('SELECTABLE_PILL_BUTTON_INACTIVE_CLASS');
    expect(selectablePillModelSource).toContain('getSelectablePillButtonClass');
    expect(apiTokenManagerSource).toContain('SelectablePillButton');
    expect(apiTokenManagerSource.match(/<SelectablePillButton/g) ?? []).toHaveLength(3);
    expect(apiTokenManagerSource).not.toContain(
      'inline-flex min-h-10 sm:min-h-10 items-center rounded-full border px-3 py-2 text-sm font-semibold transition',
    );
    expect(apiTokenManagerSource).not.toContain(
      'min-h-10 sm:min-h-10 rounded-full border px-3 py-2 text-sm font-semibold transition',
    );
    expect(apiTokenManagerSource).not.toContain('border-blue-500 bg-blue-600 text-white shadow-sm');
    expect(apiTokenManagerSource).not.toContain('hover:border-blue-400 hover:text-blue-600');
  });

  it('keeps AI model picker labels route-aware for gateway providers', () => {
    expect(aiModelPickerSource).toContain('formatAIModelRouteLabel(match)');
    expect(aiModelPickerSource).toContain('formatAIModelRouteLabel(model)');
    expect(aiModelPickerSource).toContain('isPulseOwnedLocalModelRoute');
    expect(aiModelPickerSource).toContain('const secondaryModelId = () =>');
    expect(aiModelPickerSource).toContain('!isPulseOwnedLocalModelRoute(model.id)');
    expect(aiModelPickerSource).not.toContain("model.id.split(':').pop()");
  });

  it('keeps native form selects on the shared labelled primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'form-select-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'form-select-local-native-select',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/FormSelect.tsx');
    expect(registeredRule?.canonical?.export).toBe('FormSelect');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/AI/FindingsPanel.tsx',
      'src/components/Alerts/EmailProviderSelect.tsx',
      'src/components/Alerts/WebhookConfigForm.tsx',
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
      'src/components/Kubernetes/K8sDeploymentsDrawer.tsx',
      'src/components/OrgSwitcher.tsx',
      'src/components/Settings/AIChatMaintenanceSection.tsx',
      'src/components/Settings/AIModelSelectionSection.tsx',
      'src/components/Settings/AIRuntimeControlsSection.tsx',
      'src/components/Settings/AgentProfilesPanel.tsx',
      'src/components/Settings/AuditLogPanel.tsx',
      'src/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx',
      'src/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx',
      'src/components/Settings/InfrastructureInstallerSection.tsx',
      'src/components/Settings/NodeModalMonitoringSection.tsx',
      'src/components/Settings/OrganizationAccessManagementSection.tsx',
      'src/components/Settings/OrganizationAccessMembersSection.tsx',
      'src/components/Settings/OrganizationSharingCreateSection.tsx',
      'src/components/Settings/RecoverySettingsPanel.tsx',
      'src/components/Settings/RolesEditorDialog.tsx',
      'src/components/Settings/SystemLogsPanel.tsx',
      'src/components/Settings/UpdatesSettingsPanel.tsx',
      'src/components/Storage/DiskDetail.tsx',
      'src/components/Storage/StoragePageControls.tsx',
      'src/components/Storage/StoragePoolDetail.tsx',
      'src/components/Workloads/GuestDrawerHistory.tsx',
      'src/components/shared/FilterBar/AddFilterMenu.tsx',
      'src/components/shared/FilterToolbar.tsx',
      'src/features/alerts/AlertAppriseDestinationsSection.tsx',
      'src/features/alerts/AlertEscalationSection.tsx',
      'src/features/alerts/AlertQuietHoursSection.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/FormSelect.tsx');
    expect(registeredGuard?.canonical?.export).toBe('FormSelect');
    expect(registeredGuard?.allPatterns).toEqual(['<select']);
    expect(registeredGuard?.scopes).toEqual(['src/components', 'src/features']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/Infrastructure/__tests__/ResourceDetailDrawer.docker-container.test.tsx',
      'src/components/Infrastructure/__tests__/ResourceDetailDrawer.machine-history.test.tsx',
      'src/components/shared/FilterToolbar.test.tsx',
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);

    expect(formSelectSource).toContain("from '@/components/shared/Form'");
    expect(formSelectSource).toContain('createUniqueId');
    expect(formSelectSource).toContain('splitProps');
    expect(formSelectSource).toContain('interface FormSelectProps');
    expect(formSelectSource).toContain('label: JSX.Element');
    expect(formSelectSource).toContain('<label for={selectId()}');
    expect(formSelectSource).toContain('<select');
    expect(formSelectSource).toContain('id={selectId()}');
    expect(formSelectSource).toContain('aria-describedby={describedBy()}');
    expect(formSelectSource).toContain('createEffect');
    expect(formSelectSource).toContain('MutationObserver');
    expect(formSelectSource).toContain("'value'");
    expect(formSelectSource).toContain('selectElement.value = resolvedValue');
    expect(formSelectSource).toContain('local.selectBaseClass ?? formSelect');
    expect(formSelectSource).toContain('local.fieldBaseClass ?? formField');

    for (const source of [
      addFilterMenuSource,
      filterToolbarSource,
      guestDrawerHistorySource,
      k8sDeploymentsDrawerSource,
      orgSwitcherSource,
      resourceDetailDrawerOverviewTabSource,
      storagePageControlsSource,
    ]) {
      expect(source).toContain('FormSelect');
      expect(source).not.toContain('<select');
    }

    expect(guestDrawerHistorySource).toContain('id="guest-history-range"');
    expect(guestDrawerHistorySource).toContain('data-testid="guest-history-range-control"');
  });

  it('keeps native form textareas on the shared labelled primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'form-textarea-shell');
    const alertGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'form-textarea-local-alert-fields',
    );
    const settingsGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'form-textarea-local-settings-fields',
    );
    const infrastructureGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'form-textarea-local-infrastructure-fields',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/FormTextarea.tsx');
    expect(registeredRule?.canonical?.export).toBe('FormTextarea');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Alerts/EmailProviderSelect.tsx',
      'src/components/Alerts/IncidentTimelinePanel.tsx',
      'src/components/Alerts/ThresholdsTableDockerIgnoredPrefixesSection.tsx',
      'src/components/Alerts/WebhookConfigForm.tsx',
      'src/components/Infrastructure/ReportMergeModal.tsx',
      'src/components/Settings/AgentProfilesPanel.tsx',
      'src/components/Settings/SelfHostedCommercialRecoverySection.tsx',
      'src/components/Settings/SuggestProfileModal.tsx',
      'src/features/alerts/AlertAppriseDestinationsSection.tsx',
    ]);
    expect(alertGuard?.canonical?.path).toBe('src/components/shared/FormTextarea.tsx');
    expect(alertGuard?.canonical?.export).toBe('FormTextarea');
    expect(alertGuard?.allPatterns).toEqual(['<textarea']);
    expect(alertGuard?.scopes).toEqual(['src/components/Alerts', 'src/features/alerts']);
    expect(alertGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsGuard?.canonical?.path).toBe('src/components/shared/FormTextarea.tsx');
    expect(settingsGuard?.canonical?.export).toBe('FormTextarea');
    expect(settingsGuard?.allPatterns).toEqual(['<textarea']);
    expect(settingsGuard?.scopes).toEqual(['src/components/Settings']);
    expect(settingsGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(infrastructureGuard?.canonical?.path).toBe('src/components/shared/FormTextarea.tsx');
    expect(infrastructureGuard?.canonical?.export).toBe('FormTextarea');
    expect(infrastructureGuard?.allPatterns).toEqual(['<textarea']);
    expect(infrastructureGuard?.scopes).toEqual(['src/components/Infrastructure']);
    expect(infrastructureGuard?.allowedPaths ?? []).toHaveLength(0);

    expect(formTextareaSource).toContain("from '@/components/shared/Form'");
    expect(formTextareaSource).toContain('createUniqueId');
    expect(formTextareaSource).toContain('splitProps');
    expect(formTextareaSource).toContain('interface FormTextareaProps');
    expect(formTextareaSource).toContain('label: JSX.Element');
    expect(formTextareaSource).toContain('<label for={textareaId()}');
    expect(formTextareaSource).toContain('<textarea');
    expect(formTextareaSource).toContain('id={textareaId()}');
    expect(formTextareaSource).toContain('aria-describedby={describedBy()}');
    expect(formTextareaSource).toContain('createEffect');
    expect(formTextareaSource).toContain("'value'");
    expect(formTextareaSource).toContain('textareaElement.value = nextValue');
    expect(formTextareaSource).toContain('local.textareaBaseClass ?? formTextarea');
    expect(formTextareaSource).toContain('local.fieldBaseClass ?? formField');

    const migratedConsumers = [
      emailProviderSelectSource,
      incidentTimelinePanelSource,
      thresholdsTableDockerIgnoredPrefixesSectionSource,
      webhookConfigFormSource,
      reportMergeModalSource,
      agentProfilesPanelSource,
      alertResourceTableMobileSource,
      alertResourceTableRowSource,
      selfHostedCommercialRecoverySectionSource,
      suggestProfileModalSource,
      ssoProvidersPanelSource,
      alertAppriseDestinationsSectionSource,
    ];
    for (const source of migratedConsumers) {
      expect(source).toContain('FormTextarea');
      expect(source).not.toContain('<textarea');
    }
  });

  it('routes selectable settings cards through SelectionCardGroup', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'selection-card-group-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'selection-card-group-local-active-card-styles',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/SelectionCardGroup.tsx');
    expect(registeredRule?.canonical?.export).toBe('SelectionCardGroup');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/AISettingsDialogs.tsx',
      'src/components/Settings/UpdatesSettingsPanel.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe(
      'src/components/shared/selectionCardGroupModel.ts',
    );
    expect(registeredGuard?.canonical?.export).toBe('getSelectionCardButtonClass');
    expect(registeredGuard?.allPatterns).toEqual([
      'p-4 rounded-md border-2 transition-all text-left',
      'p-3 rounded-md border-2 transition-all text-center',
    ]);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components/Settings', 'src/features', 'src/pages']),
    );

    expect(selectionCardGroupSource).toContain('useSelectionCardGroupState');
    expect(selectionCardGroupSource).toContain('getSelectionCardGroupClass');
    expect(selectionCardGroupSource).toContain('getSelectionCardButtonClass');
    expect(selectionCardGroupSource).toContain('getSelectionCardTitleClass');
    expect(selectionCardGroupSource).toContain('aria-pressed');
    expect(selectionCardGroupSource).toContain('disabled={selectionCardGroup.isOptionDisabled');
    expect(selectionCardGroupSource).not.toContain('resolveSelectionCardTone');
    expect(selectionCardGroupSource).not.toContain('props.onChange(option.value)');
    expect(selectionCardGroupStateSource).toContain('export function useSelectionCardGroupState');
    expect(selectionCardGroupStateSource).toContain('createMemo');
    expect(selectionCardGroupStateSource).toContain('resolveSelectionCardTone');
    expect(selectionCardGroupStateSource).toContain('props.disabled || option.disabled');
    expect(selectionCardGroupStateSource).toContain('props.onChange(option.value)');
    expect(selectionCardGroupModelSource).toContain('resolveSelectionCardGroupVariant');
    expect(selectionCardGroupModelSource).toContain('resolveSelectionCardTone');
    expect(selectionCardGroupModelSource).toContain('getSelectionCardButtonClass');
    expect(selectionCardGroupModelSource).toContain("compact: 'grid grid-cols-2 gap-2'");
    expect(selectionCardGroupModelSource).toContain("detail: 'grid grid-cols-1 gap-3'");
    expect(aiSettingsDialogsSource).toContain('SelectionCardGroup');
    expect(aiSettingsDialogsSource).toContain('variant="compact"');
    expect(aiSettingsDialogsSource).not.toContain(
      'class={`p-3 rounded-md border-2 transition-all text-center',
    );
    expect(updatesSettingsPanelSource).toContain('SelectionCardGroup');
    expect(updatesSettingsPanelSource).toContain('variant="detail"');
    expect(updatesSettingsPanelSource).not.toContain(
      'class={`p-4 rounded-md border-2 transition-all text-left',
    );
  });

  it('keeps AI model picking on the shared searchable notable-first primitive', () => {
    expect(aiModelPickerSource).toContain(
      "import { SearchField } from '@/components/shared/SearchField';",
    );
    expect(aiModelPickerSource).toContain(
      "import { AI_CHAT_MODEL_SELECTOR_EMPTY_STATE } from '@/utils/aiChatPresentation';",
    );
    expect(aiModelPickerSource).toContain('const notableModels = createMemo');
    expect(aiModelPickerSource).toContain('(model) => !model.notable');
    expect(aiModelPickerSource).toContain('Show ${hiddenModelCount()} older models');
    expect(aiModelPickerSource).toContain('const MODEL_ROUTE_PROVIDER_RE');
    expect(aiModelPickerSource).toContain('const selectedBadge = createMemo');
    expect(aiModelPickerSource).toContain('const selectedButtonLabel = createMemo');
    expect(aiModelPickerSource).toContain('aria-label={selectedButtonLabel()}');
    expect(aiModelPickerSource).toContain("·{' '}");
    expect(aiModelPickerSource).toContain("const CURRENT_SELECTION_LABEL = 'Current'");
    expect(aiModelPickerSource).toContain('const CurrentSelectionBadge');
    expect(aiModelPickerSource).toContain('const optionAriaLabel');
    expect(aiModelPickerSource).toContain('const displayedOptionKeys = createMemo');
    expect(aiModelPickerSource).toContain('const currentOptionKey = createMemo');
    expect(aiModelPickerSource).toContain('const focusInitialOption = () =>');
    expect(aiModelPickerSource).toContain('const handleSearchKeyDown = (event: KeyboardEvent)');
    expect(aiModelPickerSource).toContain('const handleOptionKeyDown = (');
    expect(aiModelPickerSource).toContain('role="dialog"');
    expect(aiModelPickerSource).toContain('role="listbox"');
    expect(aiModelPickerSource).toContain(
      'aria-controls={isOpen() ? `${pickerId}-listbox` : undefined}',
    );
    expect(aiModelPickerSource).toContain('onKeyDown={(event) => handleOptionKeyDown');
    expect(aiModelPickerSource).toContain('aria-selected={isSelectedRoute(model.id)}');
    expect(aiModelPickerSource).toContain('aria-label={optionAriaLabel(');
    expect(aiModelPickerSource).toContain('role="option"');
    expect(aiModelPickerSource).toContain("candidate.includes('://')");
    expect(aiModelPickerSource).toContain('separator <= 0 || separator === candidate.length - 1');
    expect(aiModelPickerSource).toContain('MOBILE_BOTTOM_CLEARANCE');
    expect(aiModelPickerSource).toContain('TOP_CLEARANCE');
    expect(aiModelPickerSource).toContain("placement: 'bottom' as 'bottom' | 'top'");
    expect(aiModelPickerSource).toContain("position.placement === 'top'");
    expect(aiModelPickerSource).toContain("availableHeight = placement === 'top'");
    expect(aiModelPickerSource).toContain(
      "style={{ 'max-height': `${dropdownPosition().listMaxHeight}px` }}",
    );
    expect(aiModelPickerSource).not.toContain('No matching models.');
    expect(aiModelPickerSource).not.toContain('<select');
  });

  it('keeps command palette on shell, runtime, and model owners', () => {
    expect(commandPaletteModalSource).toContain('useCommandPaletteState');
    expect(commandPaletteModalSource).not.toContain('useNavigate');
    expect(commandPaletteModalSource).not.toContain('createSignal');
    expect(commandPaletteModalSource).not.toContain('buildInfrastructurePath');

    expect(commandPaletteStateSource).toContain('useNavigate');
    expect(commandPaletteStateSource).toContain('createSignal');
    expect(commandPaletteStateSource).toContain('buildProxmoxPath');
    expect(commandPaletteStateSource).toContain('export function useCommandPaletteState');
    // Infrastructure and aggregate workspace URLs are not command-palette
    // destinations; platform/runtime pages own those workflows.
    expect(commandPaletteStateSource).not.toContain('buildInfrastructurePath');
    expect(commandPaletteStateSource).not.toContain('buildWorkloadsPath');
    expect(commandPaletteStateSource).not.toContain('buildStoragePath');
    expect(commandPaletteStateSource).not.toContain('buildRecoveryPath');

    expect(commandPaletteModelSource).toContain('buildCommandPaletteCommands');
    expect(commandPaletteModelSource).toContain('normalizeCommandPaletteQuery');
    expect(commandPaletteModelSource).toContain('filterCommandPaletteCommands');
  });

  it('keeps shared tooltip shells on semantic theme tokens', () => {
    expect(tooltipSource).toContain('border border-border bg-surface');
    expect(tooltipSource).toContain('text-base-content');
    expect(tooltipSource).not.toContain("'background-color': 'rgb(15, 23, 42)'");
    expect(tooltipPortalSource).toContain('bg-surface');
    expect(tooltipPortalSource).toContain('text-base-content');
    expect(tooltipPortalSource).toContain('border-border');
    expect(tooltipPortalSource).not.toContain("'background-color': 'rgb(15, 23, 42)'");
    expect(historyChartTooltipSource).toContain('border border-border bg-surface');
    expect(historyChartTooltipSource).toContain('text-base-content');
    expect(historyChartTooltipSource).toContain('text-muted');
    expect(historyChartTooltipSource).not.toContain('border-slate-600');
    expect(historyChartTooltipSource).not.toContain('bg-slate-900');
    expect(historyChartTooltipSource).not.toContain(['text', 'slate', '50'].join('-'));
  });

  it('keeps shared table card chrome on one canonical header owner', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'table-card-header-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'table-card-header-local-summary-header',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/TableCardHeader.tsx');
    expect(registeredRule?.canonical?.export).toBe('TableCardHeader');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Infrastructure/UnifiedResourceHostTableCard.tsx',
      'src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx',
      'src/components/Storage/StorageContentCard.tsx',
      'src/features/platformPage/sharedPlatformPage.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/TableCardHeader.tsx');
    expect(registeredGuard?.canonical?.export).toBe('TableCardHeader');
    expect(registeredGuard?.allPatterns).toEqual(['SummaryTableCardHeader']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components', 'src/features', 'src/pages']),
    );

    expect(tableCardHeaderSource).toContain('Clear selection');
    expect(tableCardHeaderSource).toContain("props.clearLabel ?? 'Clear'");
    expect(tableCardHeaderSource).toContain('props.clearAriaLabel ??');
    expect(tableCardHeaderSource).toContain('event.stopPropagation()');
    expect(tableCardHeaderSource).toContain('TABLE_CARD_HEADER_CLASS');
    expect(tableCardHeaderSource).not.toContain('Pinned to');
    expect(tableCardHeaderSource).not.toContain('Scoped to');
    for (const source of [
      unifiedResourceHostTableCardSource,
      unifiedResourceServiceInfrastructureCardSource,
      storageContentCardSource,
    ]) {
      expect(source).toContain('TableCardHeader');
      expect(source).not.toContain('SummaryTableCardHeader');
    }
    expect(storagePoolRowSource).not.toContain('Clear selection');
  });

  it('keeps framed product table surfaces on the shared TableCard owner', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'table-card-frame-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'table-card-frame-local-wrapper',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/TableCard.tsx');
    expect(registeredRule?.canonical?.export).toBe('TableCard');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Infrastructure/UnifiedResourceHostTableCard.tsx',
      'src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx',
      'src/components/Storage/StorageContentCard.tsx',
      'src/components/Workloads/WorkloadsSurface.tsx',
      'src/components/Workloads/WorkloadsTable.tsx',
      'src/features/alerts/AlertHistoryTableSection.tsx',
      'src/features/platformPage/sharedPlatformPage.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/TableCard.tsx');
    expect(registeredGuard?.canonical?.export).toBe('TableCard');
    expect(registeredGuard?.allPatterns).toEqual([
      'overflow-hidden border-border-subtle bg-surface',
    ]);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components', 'src/features', 'src/pages']),
    );

    expect(tableCardSource).toContain("export const TABLE_CARD_FRAME_CLASS = 'overflow-hidden'");
    expect(tableCardSource).toContain("Omit<CardProps, 'border' | 'padding' | 'tone'>");
    expect(tableCardSource).toContain('border={true}');
    expect(tableCardSource).toContain('padding="none"');
    expect(tableCardSource).toContain('tone="card"');
    expect(tableCardSource).toContain('<Card');

    for (const source of [
      workloadsTableSource,
      workloadsSurfaceSource,
      alertHistoryTableSectionSource,
      sharedPlatformPageSource,
      unifiedResourceHostTableCardSource,
      unifiedResourceServiceInfrastructureCardSource,
      storageContentCardSource,
    ]) {
      expect(source).toContain('TableCard');
      expect(source).not.toContain('overflow-hidden border-border-subtle bg-surface');
    }
  });

  it('keeps compact information card frames on the shared InfoCardFrame owner', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        extensions?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const expectedConsumers = [
      'src/components/Discovery/DiscoveryTab.tsx',
      'src/components/Infrastructure/ResourceActionHistory.tsx',
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
      'src/components/shared/WebInterfaceUrlField.tsx',
      'src/components/shared/cards/DisksCard.tsx',
      'src/components/shared/cards/HardwareCard.tsx',
      'src/components/shared/cards/NetworkInterfacesCard.tsx',
      'src/components/shared/cards/RaidCard.tsx',
      'src/components/shared/cards/RootDiskCard.tsx',
      'src/components/shared/cards/SystemInfoCard.tsx',
      'src/components/shared/cards/TemperaturesCard.tsx',
      'src/components/Workloads/DrawerDiskListCard.tsx',
      'src/components/Workloads/GuestDrawerOverview.tsx',
      'src/components/Workloads/NodeDrawerOverview.tsx',
      'src/features/docker/DockerHostDrawerOverview.tsx',
      'src/features/storageBackups/detailPresentation.ts',
    ];
    const retiredFrameClass = 'rounded border border-border bg-surface p-3 shadow-sm';
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'compact-info-card-frame-shell',
    );
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'compact-info-card-frame-local-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/InfoCardFrame.tsx');
    expect(registeredRule?.canonical?.export).toBe('InfoCardFrame');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      expectedConsumers,
    );
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/InfoCardFrame.tsx');
    expect(registeredGuard?.canonical?.export).toBe('InfoCardFrame');
    expect(registeredGuard?.allPatterns).toEqual([retiredFrameClass]);
    expect(registeredGuard?.extensions).toEqual(['.ts', '.tsx']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components', 'src/features', 'src/pages']),
    );

    const infoCardFrameSource = readFrontendSource('src/components/shared/InfoCardFrame.tsx');
    expect(infoCardFrameSource).toContain('INFO_CARD_FRAME_CLASS');
    expect(infoCardFrameSource).toContain(retiredFrameClass);
    expect(infoCardFrameSource).toContain('getInfoCardFrameClass');

    for (const consumerPath of expectedConsumers) {
      const source = readFrontendSource(consumerPath);
      expect(source).toContain('InfoCardFrame');
      expect(source).not.toContain(retiredFrameClass);
    }

    expect(readFrontendSource('src/components/shared/WebInterfaceUrlField.tsx')).toContain(
      'getInfoCardFrameClass',
    );
    expect(readFrontendSource('src/features/storageBackups/detailPresentation.ts')).toContain(
      'INFO_CARD_FRAME_CLASS',
    );
  });

  it('keeps shared subtabs as one primitive and leaves shell styling to owning surfaces', () => {
    expect(subtabsSource).not.toContain("variant?: 'default' | 'control'");
    expect(subtabsSource).not.toContain('subtabsControlShellClass');
    expect(subtabsSource).toContain('subtabsShellClass');
    expect(subtabsSource).toContain('subtabsListClass');
    expect(subtabsSource).toContain('subtabButtonClass');
  });

  it('keeps the Actions inbox on the canonical in-page subtab primitive', () => {
    expect(actionsSource).toContain('<Subtabs');
    expect(actionsSource).not.toContain('role="tablist"');
    expect(actionsSource).not.toContain('max-w-6xl');
  });

  it('keeps contextual row focus on one shared helper across summary consumers', () => {
    expect(contextualFocusSource).toContain(
      'export const preserveScrollableAncestorVerticalOffset',
    );
    expect(contextualFocusSource).toContain('export const findInlineDetailElement');
    expect(contextualFocusSource).toContain('export const revealInlineDetailInViewport');
    expect(contextualFocusSource).toContain('export function useSummaryContextualFocusState');
    expect(contextualFocusSource).toContain('chartHoveredSeriesId');
    expect(contextualFocusSource).toContain('hoveredGroupScope');
    expect(contextualFocusSource).toContain('filterSeriesForActiveScope');
    expect(contextualFocusSource).toContain('markRouteStateDeliberateScroll');
    expect(contextualFocusSource).toContain('data-inline-detail-for');
    expect(summaryCardInteractionSource).toContain('chartHoveredSeriesId');
    expect(summaryCardInteractionSource).toContain('SummarySeriesGroupScope');
    expect(summaryCardInteractionSource).toContain('resolveSummaryGroupScope');
    expect(summaryCardInteractionSource).toContain('resolveSummaryGroupMemberInteractionState');
    expect(summaryCardInteractionSource).toContain('resolveSummaryScopeState');

    expect(workloadSelectionStateSource).toContain('preserveScrollableAncestorVerticalOffset');
    expect(workloadSelectionStateSource).toContain('hoveredWorkloadGroupScope');
    expect(workloadSelectionStateSource).toContain('activeSummaryWorkloadGroupScope');
    expect(workloadSelectionStateSource).not.toContain('const scrollTop = scroller?.scrollTop');

    expect(summaryTableFocusSource).toContain('export function useSummaryTableFocusBridge');
    expect(summaryTableFocusSource).toContain('export function useSummaryPageInteractionState');
    expect(summaryTableFocusSource).toContain('resolveSummaryActiveSeriesId');
    expect(summaryTableFocusSource).toContain('activeScopeState');
    expect(summaryTableFocusSource).toContain('focusedSeriesId');
    expect(summaryTableFocusSource).toContain('findInlineDetailElement');
    expect(summaryTableFocusSource).toContain('revealInlineDetailInViewport');
    expect(summaryTableFocusSource).toContain('MutationObserver');
    expect(summaryTableFocusSource).toContain('clearPinnedScope?: () => void;');
    expect(summaryTableFocusSource).toContain('onEscapeClear?: () => void;');
    expect(summaryTableFocusSource).toContain('setClearSurfaceRootRef');
    expect(summaryTableFocusSource).toContain('[data-summary-clear-ignore]');
    expect(summaryTableFocusSource).toContain("event.key !== 'Escape'");
    expect(summaryTableFocusSource).toContain('querySelector<HTMLElement>(');
    expect(summaryTableFocusSource).toContain(
      "row.scrollIntoView({ behavior: 'smooth', block: 'center' })",
    );
    expect(summaryTableFocusSource).not.toContain('useNavigate(');
  });

  it('keeps synchronized summary values on one shared card/readout contract', () => {});

  it('keeps summary-linked table row emphasis on the shared active-row presentation contract', () => {
    expect(frontendIndexCssSource).toContain("tr[data-summary-row-active='true'] > td");
    expect(frontendIndexCssSource).toContain('--color-summary-row-bg');
    expect(frontendIndexCssSource).toContain('--color-summary-row-accent');
    expect(frontendIndexCssSource).toContain("tr[data-summary-group-member-active='preview'] > td");
    expect(frontendIndexCssSource).toContain("tr[data-summary-group-member-active='pinned'] > td");
    expect(frontendIndexCssSource).toContain('--color-summary-group-member-pinned-accent');
    expect(frontendIndexCssSource).toContain('tr.grouped-table-row > td');
    expect(frontendIndexCssSource).toContain('--color-grouped-table-row-bg');
    expect(frontendIndexCssSource).toContain(
      '--color-grouped-table-row-bg: rgba(226, 232, 240, 0.72);',
    );
    expect(frontendIndexCssSource).toContain(
      '--color-grouped-table-row-bg: rgba(51, 65, 85, 0.58);',
    );
    expect(frontendIndexCssSource).not.toContain('--color-grouped-table-row-bg: theme(');
    expect(groupedTableRowPresentationSource).toContain('GROUPED_TABLE_ROW_CLASS');
    expect(groupedTableRowPresentationSource).toContain('GROUPED_TABLE_ROW_CELL_CLASS');
    expect(groupedTableRowPresentationSource).toContain('getGroupedTableRowCellClass');
    expect(groupedTableRowPresentationSource).not.toContain('GROUPED_TABLE_ROW_DIVIDER_CLASS');

    expect(guestRowSource).toContain('data-summary-row-active');
    expect(guestRowSource).toContain('data-summary-group-member-active');
    expect(guestRowStateSource).not.toContain('bg-sky-50/70');
    expect(guestRowStateSource).not.toContain('ring-sky-400/25');

    for (const source of [
      storagePoolRowSource,
      diskListSource,
      unifiedResourceHostTableCardSource,
      unifiedResourcePBSTableSectionSource,
      unifiedResourcePMGTableSectionSource,
    ]) {
      expect(source).toContain('data-summary-row-active');
      expect(source).not.toContain('bg-sky-50/70');
      expect(source).not.toContain('ring-sky-400/25');
      expect(source).not.toContain('bg-blue-100 dark:bg-blue-800');
      expect(source).not.toContain('ring-blue-300 dark:ring-blue-600');
    }

    expect(storagePoolRowSource).toContain('data-summary-group-member-active');
    expect(storageGroupRowSource).toContain('STORAGE_GROUP_ROW_CLASS');
    expect(storageGroupPresentationSource).toContain('getInteractiveGroupedTableRowClass');
    expect(storageGroupPresentationSource).toContain('getGroupedTableRowCellClass');
    expect(nodeGroupHeaderSource).toContain('getGroupedTableRowClass');
    expect(nodeGroupHeaderSource).toContain('getGroupedTableRowCellClass');
    expect(workloadPanelSource).toContain('getInteractiveGroupedTableRowClass');
    expect(workloadPanelSource).toContain('getGroupedTableRowCellClass');
    expect(unifiedResourceHostTableCardSource).toContain('getInteractiveGroupedTableRowClass');
    expect(unifiedResourceHostTableCardSource).toContain('getGroupedTableRowCellClass');
    expect(alertHistoryTableGroupRowSource).toContain('getGroupedTableRowClass');
    expect(alertHistoryTableGroupRowSource).toContain('getGroupedTableRowCellClass');
    expect(alertHistoryTableGroupRowSource).not.toContain('class="bg-surface-alt"');
    expect(alertResourceTableDesktopSource).toContain('getGroupedTableRowClass');
    expect(alertResourceTableDesktopSource).toContain('getGroupedTableRowCellClass');
    expect(infrastructureSourceManagerSource).toContain('getGroupedTableRowClass');
    expect(infrastructureSourceManagerSource).toContain('getGroupedTableRowCellClass');
    expect(infrastructureSourceManagerSource).not.toContain('bg-base hover:bg-base');
    expect(unifiedResourceHostTableCardSource).toContain('data-summary-group-member-active');
  });

  it('routes Proxmox node version presentation through the shared formatter', () => {
    expect(nodeGroupHeaderSource).toContain("from '@/utils/proxmoxVersion'");
    expect(nodeGroupHeaderSource).toContain('formatProxmoxVersion(props.node.pveVersion)');
    expect(nodeGroupHeaderSource).not.toContain('pve-manager\\/');

    expect(proxmoxVersionSource).toContain('formatProxmoxVersion');
    expect(proxmoxVersionSource).toContain('pve-manager\\/');
    expect(proxmoxVersionSource).toContain("version.toLowerCase() === 'unknown'");
  });

  it('keeps product table scroll frames on the shared table shell', () => {
    expect(tableSource).toContain('wrapperClass');
    expect(tableSource).toContain('w-full overflow-x-auto touch-scroll');
    expect(tableSource).toContain('w-full border-collapse text-left whitespace-nowrap');
    expect(tableCardSource).toContain('TABLE_CARD_FRAME_CLASS');
    expect(tableCardSource).toContain('overflow-hidden');
    expect(tableCardHeaderSource).toContain('TABLE_CARD_HEADER_CLASS');

    for (const source of [
      unifiedResourceHostTableCardSource,
      unifiedResourcePBSTableSectionSource,
      unifiedResourcePMGTableSectionSource,
      alertHistoryTableSectionSource,
      workloadsTableSource,
      storagePoolsTableSource,
      diskListSource,
      infrastructureSourceManagerSource,
      aiCostDashboardSource,
      proxmoxMailGatewayDrawerSource,
      pulseDataGridSource,
      swarmServicesDrawerSource,
      k8sDeploymentsDrawerSource,
      k8sNamespacesDrawerSource,
    ]) {
      expect(source).toContain('<Table');
      expect(source).not.toContain('<div class="overflow-x-auto">');
      expect(source).not.toContain('<div class="overflow-x-auto bg-surface">');
      expect(source).not.toContain('<div class="overflow-auto max-h-[600px]">');
    }

    for (const source of [
      unifiedResourceHostTableCardSource,
      alertHistoryTableSectionSource,
      workloadsTableSource,
      storageContentCardSource,
    ]) {
      expect(source).toContain('<TableCard');
    }

    for (const source of [unifiedResourceHostTableCardSource, storageContentCardSource]) {
      expect(source).toContain('TableCardHeader');
    }

    expect(alertHistoryTableSectionSource).not.toContain(
      'overflow-hidden rounded border border-border',
    );
    expect(storagePoolsTableSource).not.toContain('STORAGE_POOLS_SCROLL_WRAP_CLASS');
    expect(diskListSource).not.toContain("from '@/components/shared/Card'");
    expect(diskListSource).not.toContain('PHYSICAL_DISK_TABLE_SCROLL_CLASS');
    expect(storageContentCardSource).toContain('<StoragePoolsTable');
    expect(pulseDataGridSource).toContain('wrapperClass="scrollbar-hide"');
    expect(pulseDataGridSource).toContain('getPulseDataGridFrameClass');
    expect(pulseDataGridSource).not.toContain(
      '<div class="overflow-x-auto touch-scroll scrollbar-hide">',
    );

    for (const source of [
      agentProfilesPanelSource,
      apiTokenManagerSource,
      organizationAccessMembersSectionSource,
      organizationIncomingSharesSectionSource,
      organizationOutgoingSharesSectionSource,
      organizationOverviewMembersSectionSource,
      rolesPanelSource,
      userAssignmentsPanelSource,
    ]) {
      expect(source).toContain('<PulseDataGrid');
      expect(source).not.toContain('overflow-x-auto');
      expect(source).not.toContain('-mx-4');
      expect(source).not.toContain('border-x-0 sm:border-x');
    }

    expect(agentProfilesPanelSource.match(/frame="flush"/g) ?? []).toHaveLength(2);
    expect(apiTokenManagerSource.match(/frame="flush"/g) ?? []).toHaveLength(1);
    expect(rolesPanelSource.match(/frame="flush"/g) ?? []).toHaveLength(1);
    expect(userAssignmentsPanelSource.match(/frame="flush"/g) ?? []).toHaveLength(1);
  });

  it('keeps chart visibility display actions on the shared toolbar toggle', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'chart-visibility-toggle-button',
    );
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'chart-visibility-local-toggle-labels',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/FilterToolbar.tsx');
    expect(registeredRule?.canonical?.export).toBe('ChartVisibilityToggleButton');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Workloads/WorkloadsFilter.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/FilterToolbar.tsx');
    expect(registeredGuard?.canonical?.export).toBe('ChartVisibilityToggleButton');
    expect(registeredGuard?.allPatterns).toEqual(['Show charts', 'Hide charts']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/Workloads/__tests__/WorkloadsFilter.test.tsx',
    ]);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components/Workloads', 'src/features', 'src/pages']),
    );

    expect(filterToolbarSource).toContain('export const ChartVisibilityToggleButton');
    expect(filterToolbarSource).toContain("local.collapsed ? 'Show charts' : 'Hide charts'");
    expect(filterToolbarSource).toContain('active={!local.collapsed}');
    expect(filterToolbarSource).toContain('aria-pressed={!local.collapsed}');
    expect(filterToolbarSource).toContain('title={label()}');

    expect(workloadsFilterSource).toContain('ChartVisibilityToggleButton');
    expect(workloadsFilterSource).not.toContain('Show charts');
    expect(workloadsFilterSource).not.toContain('Hide charts');
  });

  it('keeps grouped/list table-mode controls on one shared presentation contract', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'grouped-table-mode-segmented-control',
    );
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'grouped-table-mode-local-segmented-control',
    );

    expect(registeredRule?.canonical?.path).toBe(
      'src/components/shared/GroupedTableModeSegmentedControl.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('GroupedTableModeSegmentedControl');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Workloads/WorkloadsFilter.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe(
      'src/components/shared/GroupedTableModeSegmentedControl.tsx',
    );
    expect(registeredGuard?.canonical?.export).toBe('GroupedTableModeSegmentedControl');
    expect(registeredGuard?.allPatterns).toEqual(['Grouped table view', 'Flat list view']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components', 'src/features']),
    );

    expect(groupedTableModeSegmentedControlSource).toContain('GroupedTableModeSegmentedControl');
    expect(groupedTableModeSegmentedControlSource).toContain('GROUPED_TABLE_MODE_ARIA_LABEL');
    expect(groupedTableModeSegmentedControlSource).toContain(
      "GROUPED_TABLE_MODE_GROUPED_TITLE = 'Grouped table view'",
    );
    expect(groupedTableModeSegmentedControlSource).toContain(
      "GROUPED_TABLE_MODE_FLAT_TITLE = 'Flat list view'",
    );
    expect(groupedTableModeSegmentedControlSource).toContain(
      "import FolderTreeIcon from 'lucide-solid/icons/folder-tree'",
    );
    expect(groupedTableModeSegmentedControlSource).toContain(
      "import ListIcon from 'lucide-solid/icons/list'",
    );

    for (const source of [workloadsFilterSource]) {
      expect(source).toContain('GroupedTableModeSegmentedControl');
      expect(source).not.toContain("title: 'Group by node'");
      expect(source).not.toContain("title: 'Grouped table view'");
      expect(source).not.toContain("title: 'Flat list view'");
    }
  });

  it('keeps shared all-option filter labels on one presentation helper', () => {
    expect(filterOptionPresentationSource).toContain('FILTER_OPTION_ALL_LABEL');
    expect(filterOptionPresentationSource).toContain('getAllFilterOptionLabel');
    expect(filterOptionPresentationSource).toContain('collapseWhitespace');
    expect(filterOptionPresentationSource).not.toContain('toUpperCase');
    expect(filterOptionPresentationSource).not.toContain('toLowerCase');
  });

  it('keeps summary-linked row input semantics on the shared interaction helper', () => {
    expect(summaryInteractionA11ySource).toContain('createSummaryInteractiveRowPreviewHandlers');
    expect(summaryInteractionA11ySource).toContain('createSummaryInteractiveActionKeydownHandler');
    expect(summaryInteractionA11ySource).toContain('buildSummaryDisclosureControlsId');
    expect(summaryInteractionA11ySource).toContain("event.key === 'Enter'");
    expect(summaryInteractionA11ySource).toContain("event.code === 'Space'");
    expect(summaryInteractionA11ySource).toContain("event.key !== 'Escape'");
    expect(summaryInteractionA11ySource).toContain("window.matchMedia('(pointer: fine)')");
    expect(summaryRowActionButtonSource).toContain('SummaryRowActionButton');
    expect(summaryRowActionButtonSource).toContain("kind: 'disclosure'");
    expect(summaryRowActionButtonSource).toContain("kind: 'scope'");
    expect(summaryRowActionButtonSource).toContain('aria-controls');
    expect(summaryRowActionButtonSource).toContain('aria-expanded');
    expect(summaryRowActionButtonSource).toContain('aria-pressed');
    expect(summaryRowActionButtonSource).toContain('data-row-action="true"');
    expect(platformResourceDetailTableRowSource).toContain('PlatformResourceDetailToggleButton');
    expect(platformResourceDetailTableRowSource).toContain('SummaryRowActionButton');

    for (const source of [
      guestRowSource,
      storageGroupRowSource,
      storagePoolRowSource,
      diskListSource,
      unifiedResourceHostTableCardSource,
      unifiedResourcePBSTableSectionSource,
      unifiedResourcePMGTableSectionSource,
    ]) {
      expect(source).toContain('createSummaryInteractiveRowPreviewHandlers');
      expect(source).toContain('SummaryRowActionButton');
    }

    expect(workloadPanelSource).toContain('createSummaryInteractiveRowPreviewHandlers');
    expect(workloadPanelSource).not.toContain('kind="scope"');
    expect(storageGroupRowSource).not.toContain('kind="scope"');
    expect(unifiedResourceHostTableCardSource).not.toContain('kind="scope"');

    expect(nodeGroupHeaderSource).toContain('WebInterfaceNameLink');
    expect(webInterfaceNameLinkSource).toContain('event.stopPropagation()');
  });

  it('keeps upgrade navigation split across shell, runtime, and utility owners', () => {
    expect(upgradeLinkSource).toContain('destination.external');
    expect(upgradeLinkSource).toContain("return target ?? '_blank';");
    expect(upgradeLinkSource).toContain('target={target()}');
    expect(upgradeLinkSource).toContain('export const UpgradeButtonLink');
    expect(upgradeLinkSource).toContain('ButtonLink');
    expect(upgradeLinkSource).toContain('preserveOpener={local.destination.preserveOpener}');
    expect(upgradeLinkSource).not.toContain('window.open');
    expect(upgradeLinkSource).not.toContain('useNavigate(');

    expect(upgradeNavigationHookSource).toContain('export function useUpgradeNavigation');
    expect(upgradeNavigationHookSource).toContain('useNavigate()');
    expect(upgradeNavigationHookSource).toContain('navigateToUpgradeDestination');
    expect(upgradeNavigationHookSource).not.toContain('window.open');

    expect(upgradeNavigationSource).toContain('export interface UpgradeDestination');
    expect(upgradeNavigationSource).toContain('isExternalUpgradeHref');
    expect(upgradeNavigationSource).toContain('resolveUpgradeDestination');
    expect(upgradeNavigationSource).toContain('navigateToUpgradeDestination');
    expect(upgradeNavigationSource).toContain("window.open(href, '_blank', 'noopener,noreferrer')");
  });

  it('keeps upgrade CTAs on the shared upgrade button primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'upgrade-action-button-link');
    const helperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'upgrade-action-local-class-helper',
    );
    const literalShellGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'upgrade-action-local-button-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/UpgradeLink.tsx');
    expect(registeredRule?.canonical?.export).toBe('UpgradeButtonLink');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/AgentProfilesPanel.tsx',
      'src/components/Settings/AuditLogPanel.tsx',
      'src/components/Settings/ProLicensePlanSection.tsx',
      'src/components/shared/FeatureGateSection.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/utils/upgradePresentation.ts',
          patterns: expect.arrayContaining(['getUpgradeActionButtonClass']),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ProLicensePlanSection.tsx',
          patterns: expect.arrayContaining([
            'inline-flex items-center gap-1 mt-3 min-h-10 sm:min-h-9 rounded-md border border-current/20 px-3 py-2 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/5',
          ]),
        }),
      ]),
    );
    expect(helperGuard?.canonical?.path).toBe('src/components/shared/UpgradeLink.tsx');
    expect(helperGuard?.canonical?.export).toBe('UpgradeButtonLink');
    expect(helperGuard?.allPatterns).toEqual(['getUpgradeActionButtonClass']);
    expect(helperGuard?.scopes).toEqual(['src/components', 'src/features', 'src/pages']);
    expect(helperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(helperGuard?.ignoredPaths).toEqual([
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);
    expect(literalShellGuard?.canonical?.path).toBe('src/components/shared/UpgradeLink.tsx');
    expect(literalShellGuard?.canonical?.export).toBe('UpgradeButtonLink');
    expect(literalShellGuard?.allPatterns).toEqual([
      '<UpgradeLink',
      'min-h-10 sm:min-h-9 rounded-md border',
      'text-xs font-medium',
    ]);
    expect(literalShellGuard?.scopes).toEqual(['src/components', 'src/features', 'src/pages']);
    expect(literalShellGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(literalShellGuard?.ignoredPaths).toEqual([
      'src/components/Settings/AIRuntimeControlsSection.tsx',
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
      'src/components/shared/__tests__/UpgradeLink.test.tsx',
    ]);

    for (const source of [
      agentProfilesPanelSource,
      auditLogPanelSource,
      proLicensePlanSectionSource,
      featureGateSectionSource,
    ]) {
      expect(source).toContain('UpgradeButtonLink');
      expect(source).not.toContain('getUpgradeActionButtonClass');
    }
    expect(proLicensePlanSectionSource).toContain('ButtonLink');
    expect(proLicensePlanSectionSource).not.toContain(
      'inline-flex items-center gap-1 mt-3 min-h-10 sm:min-h-9 rounded-md border border-current/20 px-3 py-2 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/5',
    );
    expect(proLicensePlanSectionSource).not.toContain(
      'inline-flex items-center gap-1 mt-4 min-h-10 sm:min-h-9 rounded-md border border-border px-3 py-2 text-xs font-medium text-base-content hover:bg-surface-hover',
    );
  });

  it('routes settings external documentation text links through ExternalTextLink', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        scopes?: string[];
        allPatterns?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'settings-external-text-link-shell',
    );
    const rawAnchorGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-external-text-link-local-anchor',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/ExternalTextLink.tsx');
    expect(registeredRule?.canonical?.export).toBe('ExternalTextLink');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/AIProviderConfigurationSection.tsx',
      'src/components/Settings/AIRuntimeControlsSection.tsx',
      'src/components/Settings/AISettingsDialogs.tsx',
      'src/components/Settings/APITokenManager.tsx',
      'src/components/Settings/AgentIntegrationsPanel.tsx',
      'src/components/Settings/GeneralSettingsPanel.tsx',
      'src/components/Settings/SecurityOverviewPanel.tsx',
      'src/components/Settings/SelfHostedCommercialRecoverySection.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/components/Settings/AIProviderConfigurationSection.tsx',
          patterns: expect.arrayContaining(['target="_blank"', 'rel="noopener']),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/APITokenManager.tsx',
          patterns: expect.arrayContaining(['target="_blank"', 'rel="noreferrer"']),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/SelfHostedCommercialRecoverySection.tsx',
          patterns: expect.arrayContaining(['target="_blank"', 'rel="noopener noreferrer"']),
        }),
      ]),
    );
    expect(rawAnchorGuard?.canonical?.path).toBe('src/components/shared/ExternalTextLink.tsx');
    expect(rawAnchorGuard?.canonical?.export).toBe('ExternalTextLink');
    expect(rawAnchorGuard?.scopes).toEqual(['src/components/Settings']);
    expect(rawAnchorGuard?.allPatterns).toEqual(['<a', 'target="_blank"']);
    expect(rawAnchorGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(rawAnchorGuard?.ignoredPaths).toEqual([
      'src/components/Settings/__tests__/ProLicensePanel.test.tsx',
      'src/components/Settings/__tests__/settingsArchitecture.test.ts',
    ]);
    expect(externalTextLinkSource).toContain('EXTERNAL_TEXT_LINK_REL');
    expect(externalTextLinkSource).toContain('target="_blank"');
    expect(externalTextLinkSource).toContain('rel={getExternalTextLinkRel');
    for (const source of [
      aiProviderConfigurationSectionSource,
      aiRuntimeControlsSectionSource,
      aiSettingsDialogsSource,
      apiTokenManagerSource,
      agentIntegrationsPanelSource,
      generalSettingsPanelSource,
      securityOverviewPanelSource,
      selfHostedCommercialRecoverySectionSource,
    ]) {
      expect(source).toContain('ExternalTextLink');
      expect(source).not.toContain('rel="noopener');
      expect(source).not.toContain('rel="noreferrer"');
    }
    expect(apiAccessPanelSource).toContain('ButtonLink');
    expect(apiAccessPanelSource).toContain('variant="info"');
    expect(apiAccessPanelSource).not.toContain('<a');
  });

  it('keeps column picker on shell, runtime, and model owners', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'column-picker-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'column-picker-local-column-chooser',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/ColumnPicker.tsx');
    expect(registeredRule?.canonical?.export).toBe('ColumnPicker');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Workloads/WorkloadsFilter.tsx',
      'src/features/standalone/AgentsMachinesTable.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/ColumnPicker.tsx');
    expect(registeredGuard?.canonical?.export).toBe('ColumnPicker');
    expect(registeredGuard?.allPatterns).toEqual([
      'Choose which columns to display',
      'Show Columns',
    ]);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual(['src/components/shared/ColumnPicker.test.tsx']);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components', 'src/features', 'src/pages']),
    );

    expect(columnPickerSource).toContain('useColumnPickerState');
    expect(columnPickerSource).toContain('COLUMN_PICKER_PANEL_TITLE');
    expect(columnPickerSource).toContain('COLUMN_PICKER_BUTTON_TITLE');
    expect(columnPickerSource).toContain('filterUtilityBadgeClass');
    expect(columnPickerSource).not.toContain('createSignal');
    expect(columnPickerSource).not.toContain('createEffect');
    expect(columnPickerSource).not.toContain('document.addEventListener');
    expect(columnPickerSource).not.toContain('getHiddenColumnCount');

    expect(columnPickerStateSource).toContain('export function useColumnPickerState');
    expect(columnPickerStateSource).toContain('createSignal');
    expect(columnPickerStateSource).toContain('createEffect');
    expect(columnPickerStateSource).toContain('document.addEventListener');
    expect(columnPickerStateSource).toContain('handleClickOutside');
    expect(columnPickerStateSource).toContain('hiddenCount');

    expect(columnPickerModelSource).toContain('COLUMN_PICKER_BUTTON_LABEL');
    expect(columnPickerModelSource).toContain('COLUMN_PICKER_BUTTON_TITLE');
    expect(columnPickerModelSource).toContain('COLUMN_PICKER_PANEL_TITLE');
    expect(columnPickerModelSource).toContain('COLUMN_PICKER_RESET_LABEL');
    expect(columnPickerModelSource).toContain('COLUMN_PICKER_EMPTY_LABEL');
    expect(columnPickerModelSource).toContain('getHiddenColumnCount');
    expect(columnPickerModelSource).toContain('shouldShowColumnPickerReset');
    expect(columnPickerModelSource).toContain('getColumnPickerOptionTextClass');

    for (const source of [workloadsFilterSource, agentsMachinesTableSource]) {
      expect(source).toContain('ColumnPicker');
      expect(source).not.toContain('Choose which columns to display');
      expect(source).not.toContain('Show Columns');
    }
  });

  it('keeps tag input on shell, runtime, and model owners', () => {
    expect(tagInputSource).toContain('useTagInputState');
    expect(tagInputSource).toContain('getTagInputPlaceholder');
    expect(tagInputSource).not.toContain('createSignal');
    expect(tagInputSource).not.toContain('querySelector');
    expect(tagInputSource).not.toContain('Backspace');
    expect(tagInputSource).not.toContain('addTag');

    expect(tagInputStateSource).toContain('export function useTagInputState');
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
  });

  it('keeps toggle on shell, runtime, and model owners', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'toggle-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'toggle-local-role-switch',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/Toggle.tsx');
    expect(registeredRule?.canonical?.export).toBe('Toggle');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Alerts/AlertResourceTableDesktop.tsx',
      'src/components/Alerts/AlertResourceTableMobile.tsx',
      'src/components/Alerts/AlertResourceTableRow.tsx',
      'src/components/Alerts/ThresholdsTableDockerServiceGapSection.tsx',
      'src/components/Alerts/ThresholdsTableProxmoxBackupsSection.tsx',
      'src/components/Infrastructure/ResourceOperatorStateSection.tsx',
      'src/components/Settings/AIRuntimeControlsSection.tsx',
      'src/components/Settings/AISettings.tsx',
      'src/components/Settings/AuditLogPanel.tsx',
      'src/components/Settings/DiscoverySettingsForm.tsx',
      'src/components/Settings/DockerRuntimeSettingsCard.tsx',
      'src/components/Settings/GeneralSettingsPanel.tsx',
      'src/components/Settings/NodeModalMonitoringSection.tsx',
      'src/components/Settings/RelaySettingsPanel.tsx',
      'src/components/Settings/SSOProvidersPanel.tsx',
      'src/components/Settings/SecurityAuthPanel.tsx',
      'src/features/alerts/AlertAppriseDestinationsSection.tsx',
      'src/features/alerts/AlertCooldownSection.tsx',
      'src/features/alerts/AlertEmailDestinationsSection.tsx',
      'src/features/alerts/AlertEscalationSection.tsx',
      'src/features/alerts/AlertGroupingSection.tsx',
      'src/features/alerts/AlertQuietHoursSection.tsx',
      'src/features/alerts/AlertRecoverySection.tsx',
      'src/features/patrol/PatrolIntelligenceHeader.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/Toggle.tsx');
    expect(registeredGuard?.canonical?.export).toBe('Toggle');
    expect(registeredGuard?.allPatterns).toEqual(['role="switch"', 'aria-checked']);
    expect(registeredGuard?.scopes).toEqual(['src/components', 'src/features']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/Alerts/ResourceTable.test.tsx',
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
      'src/components/shared/Toggle.test.tsx',
    ]);

    expect(toggleSource).toContain('useToggleState');
    expect(toggleSource).toContain('getToggleTrackClass');
    expect(toggleSource).toContain('getToggleKnobClass');
    expect(toggleSource).not.toContain('defaultPrevented');
    expect(toggleSource).not.toContain('toggleSizeConfig');
    expect(toggleSource).not.toContain('handleClick =');

    expect(toggleStateSource).toContain('export function useToggleState');
    expect(toggleStateSource).toContain('defaultPrevented');
    expect(toggleStateSource).toContain('currentTarget: { checked: next }');
    expect(toggleStateSource).toContain('props.onChange?.(event)');
    expect(toggleStateSource).toContain('props.onToggle?.()');

    expect(toggleModelSource).toContain('toggleSizeConfig');
    expect(toggleModelSource).toContain('resolveToggleSize');
    expect(toggleModelSource).toContain('getToggleTrackClass');
    expect(toggleModelSource).toContain('getToggleKnobClass');
    expect(toggleModelSource).toContain('ToggleChangeEvent');

    expect(dockerRuntimeSettingsCardSource).toContain('TogglePrimitive');
    expect(dockerRuntimeSettingsCardSource).toContain('ariaLabelledBy');
    expect(dockerRuntimeSettingsCardSource).toContain('ariaDescribedBy');
    expect(dockerRuntimeSettingsCardSource).not.toContain('role="switch"');
    expect(dockerRuntimeSettingsCardSource).not.toContain('aria-checked');
  });

  it('keeps status badge on shell, runtime, and model owners', () => {
    expect(statusBadgeSource).toContain('useStatusBadgeState');
    expect(statusBadgeSource).toContain('getStatusBadgeClass');
    expect(statusBadgeSource).toContain('getStatusBadgeLabel');
    expect(statusBadgeSource).toContain('getStatusBadgeTitle');
    expect(statusBadgeSource).not.toContain('cursor-not-allowed');
    expect(statusBadgeSource).not.toContain('props.onToggle?.()');
    expect(statusBadgeSource).not.toContain('labelEnabled ??');

    expect(statusBadgeStateSource).toContain('export function useStatusBadgeState');
    expect(statusBadgeStateSource).toContain('Boolean(props.disabled)');
    expect(statusBadgeStateSource).toContain('props.onToggle?.()');
    expect(statusBadgeStateSource).toContain('if (isDisabled())');

    expect(statusBadgeModelSource).toContain('STATUS_BADGE_PADDING_BY_SIZE');
    expect(statusBadgeModelSource).toContain('getStatusBadgeClass');
    expect(statusBadgeModelSource).toContain('getStatusBadgeLabel');
    expect(statusBadgeModelSource).toContain('getStatusBadgeTitle');
    expect(statusBadgeModelSource).toContain("labelEnabled ?? 'Enabled'");
  });

  it('keeps read-only status indicator badges on the shared shell template', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'status-indicator-badge-shell',
    );
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'status-indicator-badge-local-tone-helper',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/StatusIndicatorBadge.tsx');
    expect(registeredRule?.canonical?.export).toBe('StatusIndicatorBadge');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/AgentProfilesPanel.tsx',
      'src/components/Settings/DiagnosticsResultsPanel.tsx',
      'src/components/patrol/RunHistoryEntry.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/StatusIndicatorBadge.tsx');
    expect(registeredGuard?.canonical?.export).toBe('StatusIndicatorBadge');
    expect(registeredGuard?.allPatterns).toEqual(['getStatusIndicatorBadgeToneClasses(']);
    expect(registeredGuard?.scopes).toEqual(['src/components', 'src/features']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);

    expect(statusIndicatorBadgeSource).toContain('getSimpleStatusIndicator');
    expect(statusIndicatorBadgeSource).toContain('getStatusIndicatorBadgeToneClasses');
    expect(statusIndicatorBadgeSource).toContain('StatusDot');
    expect(statusIndicatorBadgeSource).toContain('STATUS_INDICATOR_BADGE_SHAPE_CLASSES');
    expect(agentProfilesPanelSource).toContain('StatusIndicatorBadge');
    expect(agentProfilesPanelSource).not.toContain('getStatusIndicatorBadgeToneClasses');
    expect(diagnosticsResultsPanelSource).toContain('StatusIndicatorBadge');
    expect(diagnosticsResultsPanelSource).not.toContain('const StatusBadge');
    expect(diagnosticsResultsPanelSource).not.toContain('getStatusIndicatorBadgeToneClasses');
    expect(runHistoryEntrySource).toContain('StatusIndicatorBadge');
    expect(runHistoryEntrySource).not.toContain('runStatus.badgeClass');
    expect(patrolIntelligenceWorkspaceSource).not.toContain('runtimeShellPresentation()');
  });

  it('keeps platform alert severity indicators and filters on shared alert severity primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
      requiredPatternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        triggerPatterns?: string[];
        requiredPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-alert-severity-indicator-shell',
    );
    const filterOptionsRule = registry.rules?.find(
      (rule) => rule.id === 'platform-alert-severity-filter-options',
    );
    const detailToneRule = registry.rules?.find(
      (rule) => rule.id === 'platform-alert-severity-detail-tone',
    );
    const detailFormatterRule = registry.rules?.find(
      (rule) => rule.id === 'platform-alert-detail-formatters',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-alert-severity-local-helper',
    );
    const localDetailToneGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-alert-severity-local-detail-tone-helper',
    );
    const localCodeFormatterGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-alert-local-code-formatter-helper',
    );
    const localResourceTypeFormatterGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-alert-local-resource-type-formatter-helper',
    );
    const localStartedAtFormatterGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-alert-local-started-at-formatter-helper',
    );
    const localDetailDateFormatterGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-alert-local-detail-date-formatter-helper',
    );
    const localEntityTypeFormatterGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-alert-local-entity-type-formatter-helper',
    );
    const requiredGuard = registry.requiredPatternGuards?.find(
      (guard) => guard.id === 'platform-alert-severity-indicator-required',
    );
    const requiredFilterOptionsGuard = registry.requiredPatternGuards?.find(
      (guard) => guard.id === 'platform-alert-severity-filter-options-required',
    );
    const requiredDetailFormatterGuard = registry.requiredPatternGuards?.find(
      (guard) => guard.id === 'platform-alert-detail-formatters-required',
    );
    const alertTableConsumerPaths = [
      'src/features/docker/DockerAlertsTable.tsx',
      'src/features/kubernetes/KubernetesAlertsTable.tsx',
      'src/features/truenas/TrueNASAlertsTable.tsx',
      'src/features/vmware/VsphereAlertsTable.tsx',
    ];

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/AlertSeverityBadge.tsx');
    expect(registeredRule?.canonical?.export).toBe('AlertSeverityBadge');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      alertTableConsumerPaths,
    );
    expect(filterOptionsRule?.canonical?.path).toBe(
      'src/features/platformPage/platformAlertSeverityFilterOptions.tsx',
    );
    expect(filterOptionsRule?.canonical?.export).toBe('getPlatformAlertSeverityFilterOptions');
    expect(filterOptionsRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      alertTableConsumerPaths,
    );
    expect(detailToneRule?.canonical?.path).toBe('src/utils/alertSeverityPresentation.ts');
    expect(detailToneRule?.canonical?.export).toBe('getAlertSeverityDetailTone');
    expect(detailToneRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      alertTableConsumerPaths,
    );
    expect(detailFormatterRule?.canonical?.path).toBe('src/utils/alertDetailPresentation.ts');
    expect(detailFormatterRule?.canonical?.export).toBe('formatPlatformAlertCode');
    expect(detailFormatterRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      alertTableConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual(
      alertTableConsumerPaths.map((path) => ({
        path,
        patterns: ['severityVariant', 'severityTextClass'],
      })),
    );
    expect(filterOptionsRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/docker/DockerAlertsTable.tsx',
        patterns: [
          'filterChipStatusDot(',
          "value: 'critical'",
          "value: 'warning'",
          "value: 'info'",
        ],
      },
      {
        path: 'src/features/kubernetes/KubernetesAlertsTable.tsx',
        patterns: [
          'filterChipStatusDot(',
          "value: 'critical'",
          "value: 'warning'",
          "value: 'info'",
        ],
      },
      {
        path: 'src/features/truenas/TrueNASAlertsTable.tsx',
        patterns: ["value: 'critical'", "value: 'warning'", "value: 'info'"],
      },
      {
        path: 'src/features/vmware/VsphereAlertsTable.tsx',
        patterns: [
          'filterChipStatusDot(',
          "value: 'critical'",
          "value: 'warning'",
          "value: 'info'",
        ],
      },
    ]);
    expect(detailToneRule?.forbiddenPatterns).toEqual(
      alertTableConsumerPaths.map((path) => ({
        path,
        patterns: ['const alertTone', 'type AlertDetailTone'],
      })),
    );
    expect(detailFormatterRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/docker/DockerAlertsTable.tsx',
        patterns: [
          'const formatResourceType',
          'const formatCode',
          'const formatStartedAt',
          'const detailDateTime',
        ],
      },
      {
        path: 'src/features/kubernetes/KubernetesAlertsTable.tsx',
        patterns: [
          'const formatResourceType',
          'const formatCode',
          'const formatStartedAt',
          'const detailDateTime',
        ],
      },
      {
        path: 'src/features/truenas/TrueNASAlertsTable.tsx',
        patterns: [
          'const formatResourceType',
          'const formatCode',
          'const formatStartedAt',
          'const detailDateTime',
        ],
      },
      {
        path: 'src/features/vmware/VsphereAlertsTable.tsx',
        patterns: [
          'const formatResourceType',
          'const formatEntityType',
          'const formatCode',
          'const formatStartedAt',
          'const detailDateTime',
        ],
      },
    ]);
    expect(localHelperGuard?.canonical?.path).toBe('src/components/shared/AlertSeverityBadge.tsx');
    expect(localHelperGuard?.canonical?.export).toBe('AlertSeverityBadge');
    expect(localHelperGuard?.allPatterns).toEqual(['severityVariant', 'severityTextClass']);
    expect(localHelperGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localDetailToneGuard?.canonical?.path).toBe('src/utils/alertSeverityPresentation.ts');
    expect(localDetailToneGuard?.canonical?.export).toBe('getAlertSeverityDetailTone');
    expect(localDetailToneGuard?.allPatterns).toEqual(['const alertTone', 'severityBucket']);
    expect(localDetailToneGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localDetailToneGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localDetailToneGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localCodeFormatterGuard?.canonical?.path).toBe('src/utils/alertDetailPresentation.ts');
    expect(localCodeFormatterGuard?.canonical?.export).toBe('formatPlatformAlertCode');
    expect(localCodeFormatterGuard?.allPatterns).toEqual(['const formatCode', 'IncidentRow']);
    expect(localCodeFormatterGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localCodeFormatterGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localCodeFormatterGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localResourceTypeFormatterGuard?.canonical?.path).toBe(
      'src/utils/alertDetailPresentation.ts',
    );
    expect(localResourceTypeFormatterGuard?.canonical?.export).toBe(
      'formatPlatformAlertResourceType',
    );
    expect(localResourceTypeFormatterGuard?.allPatterns).toEqual([
      'const formatResourceType',
      'IncidentRow',
    ]);
    expect(localResourceTypeFormatterGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localResourceTypeFormatterGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localResourceTypeFormatterGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localStartedAtFormatterGuard?.canonical?.path).toBe(
      'src/utils/alertDetailPresentation.ts',
    );
    expect(localStartedAtFormatterGuard?.canonical?.export).toBe('formatPlatformAlertStartedAt');
    expect(localStartedAtFormatterGuard?.allPatterns).toEqual([
      'const formatStartedAt',
      'IncidentRow',
    ]);
    expect(localStartedAtFormatterGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localStartedAtFormatterGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localStartedAtFormatterGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localDetailDateFormatterGuard?.canonical?.path).toBe(
      'src/utils/alertDetailPresentation.ts',
    );
    expect(localDetailDateFormatterGuard?.canonical?.export).toBe(
      'formatPlatformAlertDetailDateTime',
    );
    expect(localDetailDateFormatterGuard?.allPatterns).toEqual([
      'const detailDateTime',
      'IncidentRow',
    ]);
    expect(localDetailDateFormatterGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localDetailDateFormatterGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localDetailDateFormatterGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localEntityTypeFormatterGuard?.canonical?.path).toBe(
      'src/utils/alertDetailPresentation.ts',
    );
    expect(localEntityTypeFormatterGuard?.canonical?.export).toBe('formatPlatformAlertEntityType');
    expect(localEntityTypeFormatterGuard?.allPatterns).toEqual([
      'const formatEntityType',
      'VmwareIncidentRow',
    ]);
    expect(localEntityTypeFormatterGuard?.scopes).toEqual(['src/features/vmware']);
    expect(localEntityTypeFormatterGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localEntityTypeFormatterGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(requiredGuard?.canonical?.path).toBe('src/components/shared/AlertSeverityBadge.tsx');
    expect(requiredGuard?.canonical?.export).toBe('AlertSeverityBadge');
    expect(requiredGuard?.triggerPatterns).toEqual([
      'severityBucket',
      'IncidentRow',
      "getPlatformTableCellClassForKind('badge')",
    ]);
    expect(requiredGuard?.requiredPatterns).toEqual(['AlertSeverityBadge', 'AlertSeverityDot']);
    expect(requiredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(requiredGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(requiredFilterOptionsGuard?.canonical?.path).toBe(
      'src/features/platformPage/platformAlertSeverityFilterOptions.tsx',
    );
    expect(requiredFilterOptionsGuard?.canonical?.export).toBe(
      'getPlatformAlertSeverityFilterOptions',
    );
    expect(requiredFilterOptionsGuard?.triggerPatterns).toEqual([
      'severityBucket',
      'PlatformTableToolbar',
      'statusOptions',
    ]);
    expect(requiredFilterOptionsGuard?.requiredPatterns).toEqual([
      'getPlatformAlertSeverityFilterOptions',
    ]);
    expect(requiredFilterOptionsGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(requiredFilterOptionsGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(requiredDetailFormatterGuard?.canonical?.path).toBe(
      'src/utils/alertDetailPresentation.ts',
    );
    expect(requiredDetailFormatterGuard?.canonical?.export).toBe('formatPlatformAlertCode');
    expect(requiredDetailFormatterGuard?.triggerPatterns).toEqual([
      'IncidentRow',
      'incident.code',
      'incident.startedAt',
    ]);
    expect(requiredDetailFormatterGuard?.requiredPatterns).toEqual([
      'formatPlatformAlertCode',
      'formatPlatformAlertStartedAt',
      'formatPlatformAlertDetailDateTime',
    ]);
    expect(requiredDetailFormatterGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(requiredDetailFormatterGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(alertSeverityBadgeSource).toContain('StatusIndicatorBadge');
    expect(alertSeverityBadgeSource).toContain('StatusDot');
    expect(alertSeverityBadgeSource).toContain('getAlertSeverityIndicator');
    expect(platformAlertSeverityFilterOptionsSource).toContain(
      'getPlatformAlertSeverityFilterOptions',
    );
    expect(platformAlertSeverityFilterOptionsSource).toContain('filterChipStatusDot');
    expect(alertSeverityPresentationSource).toContain('getAlertSeverityIndicatorVariant');
    expect(alertSeverityPresentationSource).toContain('getAlertSeverityDetailTone');
    expect(alertSeverityPresentationSource).toContain('formatAlertSeverityLabel');
    expect(alertDetailPresentationSource).toContain('formatPlatformAlertCode');
    expect(alertDetailPresentationSource).toContain('formatPlatformAlertResourceType');
    expect(alertDetailPresentationSource).toContain('formatPlatformAlertEntityType');
    expect(alertDetailPresentationSource).toContain('formatPlatformAlertStartedAt');
    expect(alertDetailPresentationSource).toContain('formatPlatformAlertDetailDateTime');

    for (const source of [
      dockerAlertsTableSource,
      kubernetesAlertsTableSource,
      truenasAlertsTableSource,
      vsphereAlertsTableSource,
    ]) {
      expect(source).toContain('AlertSeverityBadge');
      expect(source).toContain('AlertSeverityDot');
      expect(source).toContain('formatAlertSeverityLabel');
      expect(source).toContain('getAlertSeverityDetailTone');
      expect(source).toContain('getPlatformAlertSeverityFilterOptions');
      expect(source).toContain('formatPlatformAlertCode');
      expect(source).toContain('formatPlatformAlertResourceType');
      expect(source).toContain('formatPlatformAlertStartedAt');
      expect(source).toContain('formatPlatformAlertDetailDateTime');
      expect(source).not.toContain('severityVariant');
      expect(source).not.toContain('severityTextClass');
      expect(source).not.toContain('const alertTone');
      expect(source).not.toContain('type AlertDetailTone');
      expect(source).not.toContain('const formatResourceType');
      expect(source).not.toContain('const formatCode');
      expect(source).not.toContain('const formatStartedAt');
      expect(source).not.toContain('const detailDateTime');
      expect(source).not.toContain('filterChipStatusDot(');
      expect(source).not.toContain("value: 'critical'");
      expect(source).not.toContain("value: 'warning'");
      expect(source).not.toContain("value: 'info'");
      expect(source).not.toContain('text-red-700 dark:text-red-300');
      expect(source).not.toContain('text-amber-700 dark:text-amber-300');
    }
  });

  it('keeps metadata badges on shared badge primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const metadataRule = registry.rules?.find((rule) => rule.id === 'metadata-badge-shell');
    const roleRule = registry.rules?.find((rule) => rule.id === 'organization-role-badge-shell');
    const shareStatusRule = registry.rules?.find(
      (rule) => rule.id === 'organization-share-status-badge-shell',
    );
    const localRoleGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'organization-role-badge-local-shell',
    );
    const localShareStatusGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'organization-share-status-badge-local-shell',
    );
    const patrolRunMetadataGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'patrol-run-metadata-badge-local-shell',
    );
    const patrolInvestigationMetadataGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'patrol-investigation-metadata-badge-local-shell',
    );
    const patrolFindingMetadataGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'patrol-finding-metadata-badge-local-shell',
    );
    const patrolApprovalToolCallMetadataGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'patrol-approval-tool-call-metadata-badge-local-shell',
    );
    const proxmoxBackupMetadataGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'proxmox-backup-metadata-badge-local-shell',
    );

    expect(metadataRule?.canonical?.path).toBe('src/components/shared/MetadataBadge.tsx');
    expect(metadataRule?.canonical?.export).toBe('MetadataBadge');
    expect(metadataRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/shared/OrganizationBadges.tsx',
      'src/components/AI/FindingsPanel.tsx',
      'src/components/patrol/ApprovalBanner.tsx',
      'src/components/patrol/ApprovalSection.tsx',
      'src/components/patrol/InvestigationSection.tsx',
      'src/components/patrol/RunHistoryEntry.tsx',
      'src/components/patrol/RunToolCallTrace.tsx',
      'src/features/proxmox/proxmoxBackupsTableShared.tsx',
      'src/features/patrol/PatrolIntelligenceWorkspace.tsx',
    ]);
    expect(roleRule?.canonical?.path).toBe('src/components/shared/OrganizationBadges.tsx');
    expect(roleRule?.canonical?.export).toBe('OrganizationRoleBadge');
    expect(roleRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/OrganizationAccessInvitationsSection.tsx',
      'src/components/Settings/OrganizationAccessMembersSection.tsx',
      'src/components/Settings/OrganizationIncomingSharesSection.tsx',
      'src/components/Settings/OrganizationOutgoingSharesSection.tsx',
      'src/components/Settings/OrganizationOverviewMembersSection.tsx',
    ]);
    expect(shareStatusRule?.canonical?.path).toBe('src/components/shared/OrganizationBadges.tsx');
    expect(shareStatusRule?.canonical?.export).toBe('OrganizationShareStatusBadge');
    expect(shareStatusRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/OrganizationIncomingSharesSection.tsx',
      'src/components/Settings/OrganizationOutgoingSharesSection.tsx',
    ]);
    expect(localRoleGuard?.allPatterns).toEqual(['roleBadgeClass(']);
    expect(localRoleGuard?.scopes).toEqual(['src/components/Settings']);
    expect(localShareStatusGuard?.allPatterns).toEqual(['statusBadgeClass(']);
    expect(localShareStatusGuard?.scopes).toEqual(['src/components/Settings']);
    expect(patrolRunMetadataGuard?.canonical?.path).toBe('src/components/shared/MetadataBadge.tsx');
    expect(patrolRunMetadataGuard?.canonical?.export).toBe('MetadataBadge');
    expect(patrolRunMetadataGuard?.allPatterns).toEqual([
      'inline-flex items-center gap-1',
      'rounded-full text-xs font-medium',
    ]);
    expect(patrolRunMetadataGuard?.scopes).toEqual(['src/components/patrol']);
    expect(patrolFindingMetadataGuard?.canonical?.path).toBe(
      'src/components/shared/MetadataBadge.tsx',
    );
    expect(patrolFindingMetadataGuard?.canonical?.export).toBe('MetadataBadge');
    expect(patrolFindingMetadataGuard?.allPatterns).toEqual([
      'px-1.5 py-0.5 border text-[10px] font-medium rounded',
    ]);
    expect(patrolFindingMetadataGuard?.scopes).toEqual(['src/components/AI']);
    expect(patrolInvestigationMetadataGuard?.canonical?.path).toBe(
      'src/components/shared/MetadataBadge.tsx',
    );
    expect(patrolInvestigationMetadataGuard?.canonical?.export).toBe('MetadataBadge');
    expect(patrolInvestigationMetadataGuard?.allPatterns).toEqual([
      'px-1.5 py-0.5 border text-[10px] font-medium rounded',
    ]);
    expect(patrolInvestigationMetadataGuard?.scopes).toEqual(['src/components/patrol']);
    expect(patrolApprovalToolCallMetadataGuard?.canonical?.path).toBe(
      'src/components/shared/MetadataBadge.tsx',
    );
    expect(patrolApprovalToolCallMetadataGuard?.canonical?.export).toBe('MetadataBadge');
    expect(patrolApprovalToolCallMetadataGuard?.allPatterns).toEqual([
      'px-1.5 py-0.5',
      'text-[10px] font-medium rounded',
    ]);
    expect(patrolApprovalToolCallMetadataGuard?.scopes).toEqual(['src/components/patrol']);
    expect(proxmoxBackupMetadataGuard?.canonical?.path).toBe(
      'src/components/shared/MetadataBadge.tsx',
    );
    expect(proxmoxBackupMetadataGuard?.canonical?.export).toBe('MetadataBadge');
    expect(proxmoxBackupMetadataGuard?.allPatterns).toEqual([
      'rounded-sm px-1.5 py-0.5 text-[10px] font-semibold',
    ]);
    expect(proxmoxBackupMetadataGuard?.scopes).toEqual(['src/features/proxmox']);

    expect(metadataBadgeSource).toContain('METADATA_BADGE_TONE_CLASSES');
    expect(metadataBadgeSource).toContain('MetadataBadgeAppearance');
    expect(metadataBadgeSource).toContain('getMetadataBadgeClass');
    expect(metadataBadgeSource).toContain("muted: 'bg-surface-alt text-muted'");
    expect(metadataBadgeSource).toContain("warning: 'bg-amber-100 text-amber-800");
    expect(metadataBadgeSource).not.toContain(["muted: 'bg", 'slate', '100'].join('-'));
    expect(organizationBadgesSource).toContain('MetadataBadge');
    expect(organizationBadgesSource).toContain('getOrganizationRoleBadgeTone');
    expect(organizationBadgesSource).toContain('getOrganizationShareStatusBadgeTone');

    for (const source of [
      organizationAccessInvitationsSectionSource,
      organizationAccessMembersSectionSource,
      organizationIncomingSharesSectionSource,
      organizationOutgoingSharesSectionSource,
      organizationOverviewMembersSectionSource,
    ]) {
      expect(source).toContain('OrganizationRoleBadge');
      expect(source).not.toContain('roleBadgeClass');
      expect(source).not.toContain('inline-flex rounded-full px-2 py-0.5 text-xs font-medium');
    }

    for (const source of [
      organizationIncomingSharesSectionSource,
      organizationOutgoingSharesSectionSource,
    ]) {
      expect(source).toContain('OrganizationShareStatusBadge');
      expect(source).not.toContain('statusBadgeClass');
      expect(source).not.toContain(
        'inline-flex w-fit rounded-full px-2 py-0.5 text-xs font-medium',
      );
    }

    expect(runHistoryEntrySource).toContain('MetadataBadge');
    expect(runHistoryEntrySource).not.toContain(
      'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium',
    );
    expect(runHistoryEntrySource).not.toContain(
      'inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-blue-50',
    );
    expect(patrolIntelligenceWorkspaceSource).toContain('MetadataBadge');
    expect(patrolIntelligenceWorkspaceSource).toContain('tone={group.tone}');
    expect(patrolIntelligenceWorkspaceSource).not.toContain(
      'findingsBadgePresentation().toneClasses',
    );
    expect(patrolIntelligenceWorkspaceSource).not.toContain(
      'ml-1.5 px-1.5 py-0.5 text-xs rounded-full',
    );
    expect(approvalBannerSource).toContain('MetadataBadge');
    expect(approvalBannerSource).toContain('APPROVAL_BANNER_BADGE_PROPS');
    expect(approvalBannerSource).not.toContain('firstApprovalRisk()!.badgeClass');
    expect(approvalSectionSource).toContain('MetadataBadge');
    expect(approvalSectionSource).toContain('BADGE_PROPS');
    expect(approvalSectionSource).not.toContain('approvalRisk.badgeClass');
    expect(approvalSectionSource).not.toContain('fixRisk.badgeClass');
    expect(findingsPanelSource).toContain('MetadataBadge');
    expect(findingsPanelSource).toContain('FINDING_ROW_BADGE_PROPS');
    expect(findingsPanelSource).not.toContain(
      'px-1.5 py-0.5 border text-[10px] font-medium rounded',
    );
    expect(findingsPanelSource).not.toContain('getFindingStatusBadgeClasses');
    expect(findingsPanelSource).not.toContain('getFindingSourceBadgeClasses');
    expect(findingsPanelSource).not.toContain('getFindingLoopStateBadgeClasses');
    expect(findingsPanelSource).not.toContain('getInvestigationStatusBadgeClasses');
    expect(findingsPanelSource).not.toContain('getInvestigationOutcomeBadgeClasses');
    expect(findingsPanelSource).not.toContain('getInvestigationConfidenceBadgeClasses');
    expect(investigationSectionSource).toContain('MetadataBadge');
    expect(investigationSectionSource).toContain('INVESTIGATION_BADGE_PROPS');
    expect(investigationSectionSource).not.toContain(
      'px-1.5 py-0.5 border text-[10px] font-medium rounded',
    );
    expect(investigationSectionSource).not.toContain('getInvestigationStatusBadgeClasses');
    expect(investigationSectionSource).not.toContain('getInvestigationOutcomeBadgeClasses');
    expect(runToolCallTraceSource).toContain('MetadataBadge');
    expect(runToolCallTraceSource).toContain('RUN_TOOL_CALL_BADGE_PROPS');
    expect(runToolCallTraceSource).not.toContain('getToolCallResultBadgeClass');
    expect(proxmoxBackupsTableSharedSource).toContain('MetadataBadge');
    expect(proxmoxBackupsTableSharedSource).toContain('PROXMOX_BACKUP_METADATA_BADGE_PROPS');
    expect(proxmoxBackupsTableSharedSource).toContain('presentation().badgeTone');
    expect(proxmoxBackupsTableSharedSource).not.toContain('presentation().badgeClassName');
    expect(proxmoxBackupsTableSharedSource).not.toContain(
      'inline-flex items-center rounded-sm px-1.5 py-0.5 text-[10px] font-semibold',
    );
  });

  it('keeps resource status dots on the shared StatusDot primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'status-dot-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'status-dot-local-linked-disk-health-dot',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/StatusDot.tsx');
    expect(registeredRule?.canonical?.export).toBe('StatusDot');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/shared/StatusIndicatorBadge.tsx',
      'src/components/Settings/ResourcePicker.tsx',
      'src/components/Storage/StoragePoolDetail.tsx',
      'src/components/shared/NodeGroupHeader.tsx',
      'src/features/alerts/AlertOverviewStatsCards.tsx',
      'src/features/docker/DockerHostsTable.tsx',
      'src/features/kubernetes/KubernetesNodesTable.tsx',
      'src/features/truenas/TrueNASSystemsTable.tsx',
      'src/features/vmware/VsphereHostsTable.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/StatusDot.tsx');
    expect(registeredGuard?.canonical?.export).toBe('StatusDot');
    expect(registeredGuard?.allPatterns).toEqual([
      'h-2 w-2 rounded-full bg-yellow-500',
      'h-2 w-2 rounded-full bg-green-500',
    ]);
    expect(registeredGuard?.scopes).toEqual([
      'src/components/Storage',
      'src/features/storageBackups',
    ]);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);

    expect(statusDotSource).toContain('const VARIANT_CLASSES');
    expect(statusDotSource).toContain("warning: 'bg-amber-500 dark:bg-amber-400'");
    expect(statusDotSource).toContain('aria-hidden={ariaHidden()}');
    expect(statusIndicatorBadgeSource).toContain('StatusDot');
    expect(storagePoolDetailSource).toContain('StatusDot');
    expect(storagePoolDetailSource).toContain('getLinkedDiskHealthDotVariant');
    expect(storagePoolDetailSource).not.toContain('getLinkedDiskHealthDotClass');
    expect(storagePoolDetailSource).not.toContain('h-2 w-2 rounded-full bg-yellow-500');
    expect(storagePoolDetailSource).not.toContain('h-2 w-2 rounded-full bg-green-500');

    for (const source of [
      statusIndicatorBadgeSource,
      resourcePickerSource,
      storagePoolDetailSource,
      nodeGroupHeaderSource,
      alertOverviewStatsCardsSource,
      dockerHostsTableSource,
      kubernetesNodesTableSource,
      truenasSystemsTableSource,
      vsphereHostsTableSource,
    ]) {
      expect(source).toContain('StatusDot');
    }
  });

  it('keeps shared, discovery drawer, Login, Settings, Patrol, and AI loading spinners on shared loading primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'loading-spinner-shell');
    const discoveryFallbackRule = registry.rules?.find(
      (rule) => rule.id === 'discovery-loading-fallback',
    );
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'patrol-ai-local-loading-spinner-shell',
    );
    const settingsBorderTopGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-local-border-loading-spinner-shell',
    );
    const settingsBorderBottomGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-local-border-bottom-loading-spinner-shell',
    );
    const loginBorderGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'login-local-border-loading-spinner-shell',
    );
    const sharedComponentSpinnerGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'shared-component-local-loading-spinner-shell',
    );
    const drawerDiscoveryFallbackGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'drawer-discovery-local-loading-fallback',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/LoadingSpinner.tsx');
    expect(registeredRule?.canonical?.export).toBe('LoadingSpinner');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/AI/FindingsPanel.tsx',
      'src/components/Login.tsx',
      'src/components/Settings/AISettings.tsx',
      'src/components/Settings/AISettingsDialogs.tsx',
      'src/components/Settings/APITokenManager.tsx',
      'src/components/Settings/AgentProfilesPanel.tsx',
      'src/components/Settings/RolesPanel.tsx',
      'src/components/Settings/SSOProvidersPanel.tsx',
      'src/components/Settings/UpdateInstallGuide.tsx',
      'src/components/Settings/UpdatesSettingsPanel.tsx',
      'src/components/Settings/UserAssignmentsDialog.tsx',
      'src/components/Settings/UserAssignmentsPanel.tsx',
      'src/components/UpdateProgressModal.tsx',
      'src/components/shared/Button.tsx',
      'src/components/shared/DiscoveryLoadingFallback.tsx',
      'src/components/shared/HistoryChartOverlay.tsx',
      'src/components/shared/PulseDataGrid.tsx',
      'src/components/patrol/ApprovalBanner.tsx',
      'src/components/patrol/ApprovalSection.tsx',
      'src/components/patrol/InvestigationMessages.tsx',
      'src/components/patrol/InvestigationSection.tsx',
      'src/components/patrol/RunToolCallTrace.tsx',
    ]);
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/LoadingSpinner.tsx');
    expect(registeredGuard?.canonical?.export).toBe('LoadingSpinner');
    expect(registeredGuard?.allPatterns).toEqual(['border-t-transparent', 'animate-spin']);
    expect(registeredGuard?.scopes).toEqual([
      'src/components/patrol',
      'src/features/patrol',
      'src/components/AI/FindingsPanel.tsx',
    ]);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsBorderTopGuard?.canonical?.path).toBe(
      'src/components/shared/LoadingSpinner.tsx',
    );
    expect(settingsBorderTopGuard?.canonical?.export).toBe('LoadingSpinner');
    expect(settingsBorderTopGuard?.allPatterns).toEqual(['border-t-transparent', 'animate-spin']);
    expect(settingsBorderTopGuard?.scopes).toEqual(['src/components/Settings']);
    expect(settingsBorderTopGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsBorderBottomGuard?.canonical?.path).toBe(
      'src/components/shared/LoadingSpinner.tsx',
    );
    expect(settingsBorderBottomGuard?.canonical?.export).toBe('LoadingSpinner');
    expect(settingsBorderBottomGuard?.allPatterns).toEqual(['border-b-2', 'animate-spin']);
    expect(settingsBorderBottomGuard?.scopes).toEqual(['src/components/Settings']);
    expect(settingsBorderBottomGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(loginBorderGuard?.canonical?.path).toBe('src/components/shared/LoadingSpinner.tsx');
    expect(loginBorderGuard?.canonical?.export).toBe('LoadingSpinner');
    expect(loginBorderGuard?.allPatterns).toEqual(['border-t-transparent', 'animate-spin']);
    expect(loginBorderGuard?.scopes).toEqual(['src/components/Login.tsx']);
    expect(loginBorderGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(sharedComponentSpinnerGuard?.canonical?.path).toBe(
      'src/components/shared/LoadingSpinner.tsx',
    );
    expect(sharedComponentSpinnerGuard?.canonical?.export).toBe('LoadingSpinner');
    expect(sharedComponentSpinnerGuard?.allPatterns).toEqual(['animate-spin']);
    expect(sharedComponentSpinnerGuard?.scopes).toEqual([
      'src/components/shared/Button.tsx',
      'src/components/shared/HistoryChartOverlay.tsx',
      'src/components/shared/PulseDataGrid.tsx',
    ]);
    expect(sharedComponentSpinnerGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(discoveryFallbackRule?.canonical?.path).toBe(
      'src/components/shared/DiscoveryLoadingFallback.tsx',
    );
    expect(discoveryFallbackRule?.canonical?.export).toBe('DiscoveryLoadingFallback');
    expect(discoveryFallbackRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Infrastructure/ResourceDetailDrawer.tsx',
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
      'src/components/Workloads/GuestDrawer.tsx',
      'src/components/Workloads/NodeDrawer.tsx',
      'src/features/docker/DockerHostDrawer.tsx',
    ]);
    expect(drawerDiscoveryFallbackGuard?.canonical?.path).toBe(
      'src/components/shared/DiscoveryLoadingFallback.tsx',
    );
    expect(drawerDiscoveryFallbackGuard?.canonical?.export).toBe('DiscoveryLoadingFallback');
    expect(drawerDiscoveryFallbackGuard?.allPatterns).toEqual([
      'border-t-transparent',
      'animate-spin',
    ]);
    expect(drawerDiscoveryFallbackGuard?.scopes).toEqual([
      'src/components/Infrastructure/ResourceDetailDrawer.tsx',
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
      'src/components/Workloads/GuestDrawer.tsx',
      'src/components/Workloads/NodeDrawer.tsx',
      'src/features/docker/DockerHostDrawer.tsx',
    ]);
    expect(drawerDiscoveryFallbackGuard?.allowedPaths ?? []).toHaveLength(0);

    expect(loadingSpinnerSource).toContain('getLoadingSpinnerClass');
    expect(loadingSpinnerSource).toContain('aria-hidden={ariaHidden()}');
    expect(loadingSpinnerSource).toContain("button: 'h-5 w-5 border-2'");
    expect(discoveryLoadingFallbackSource).toContain('LoadingSpinner');
    expect(discoveryLoadingFallbackSource).toContain('getDiscoveryLoadingState');
    expect(discoveryLoadingFallbackSource).toContain('role="status"');
    expect(loginSource).toContain('LoadingSpinner');
    expect(loginSource).toContain('size="button"');
    expect(loginSource).not.toContain(
      'animate-spin h-12 w-12 border-4 border-blue-500 border-t-transparent rounded-full mx-auto mb-4',
    );
    expect(loginSource).not.toContain('class="animate-spin -ml-1 mr-3 h-5 w-5 text-white"');

    for (const source of [
      buttonSource,
      discoveryLoadingFallbackSource,
      historyChartOverlaySource,
      pulseDataGridSource,
      updateProgressModalSource,
    ]) {
      expect(source).toContain('LoadingSpinner');
      expect(source).not.toContain('animate-spin');
    }

    const discoveryFallbackConsumers: Array<[string, string]> = [
      ['src/components/Infrastructure/ResourceDetailDrawer.tsx', resourceDetailDrawerSource],
      [
        'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
        resourceDetailDrawerOverviewTabSource,
      ],
      ['src/components/Workloads/GuestDrawer.tsx', guestDrawerSource],
      ['src/components/Workloads/NodeDrawer.tsx', nodeDrawerSource],
      ['src/features/docker/DockerHostDrawer.tsx', dockerHostDrawerSource],
    ];
    const discoveryFallbackForbiddenByPath = new Map(
      discoveryFallbackRule?.forbiddenPatterns?.map((entry) => [
        entry.path,
        entry.patterns ?? [],
      ]) ?? [],
    );

    for (const [path, source] of discoveryFallbackConsumers) {
      expect(source).toContain('DiscoveryLoadingFallback');
      for (const retiredPattern of discoveryFallbackForbiddenByPath.get(path) ?? []) {
        expect(source).not.toContain(retiredPattern);
      }
      expect(source).not.toContain('border-t-transparent');
    }

    for (const source of [
      findingsPanelSource,
      approvalSectionSource,
      investigationMessagesSource,
      investigationSectionSource,
      runToolCallTraceSource,
    ]) {
      expect(source).toContain('LoadingSpinner');
      expect(source).not.toMatch(/border(?:-\d)?[^\n]*border-t-transparent[^\n]*animate-spin/);
    }

    const settingsSpinnerConsumers = [
      'src/components/Settings/AISettings.tsx',
      'src/components/Settings/AISettingsDialogs.tsx',
      'src/components/Settings/APITokenManager.tsx',
      'src/components/Settings/AgentProfilesPanel.tsx',
      'src/components/Settings/RolesPanel.tsx',
      'src/components/Settings/SSOProvidersPanel.tsx',
      'src/components/Settings/UpdateInstallGuide.tsx',
      'src/components/Settings/UpdatesSettingsPanel.tsx',
      'src/components/Settings/UserAssignmentsDialog.tsx',
      'src/components/Settings/UserAssignmentsPanel.tsx',
    ];
    const forbiddenPatternsByPath = new Map(
      registeredRule?.forbiddenPatterns?.map((entry) => [entry.path, entry.patterns ?? []]) ?? [],
    );

    for (const path of settingsSpinnerConsumers) {
      const source = readFrontendSource(path);
      expect(source).toContain('LoadingSpinner');
      for (const retiredPattern of forbiddenPatternsByPath.get(path) ?? []) {
        expect(source).not.toContain(retiredPattern);
      }
      expect(source).not.toContain('border-t-transparent');
      expect(source).not.toContain('border-b-2 border-blue-500');
    }
  });

  it('keeps organization Settings loading skeletons on the shared skeleton primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
      requiredPatternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        pathIncludes?: string[];
        requiredPatterns?: string[];
        scopes?: string[];
        triggerPatterns?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'settings-loading-skeleton-shell',
    );
    const localSkeletonGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-local-loading-skeleton-block-shell',
    );
    const requiredSkeletonGuard = registry.requiredPatternGuards?.find(
      (guard) => guard.id === 'settings-loading-state-shared-skeleton-required',
    );
    const organizationLoadingStateConsumers: Array<[string, string]> = [
      [
        'src/components/Settings/OrganizationAccessLoadingState.tsx',
        organizationAccessLoadingStateSource,
      ],
      [
        'src/components/Settings/OrganizationBillingLoadingState.tsx',
        organizationBillingLoadingStateSource,
      ],
      [
        'src/components/Settings/OrganizationOverviewLoadingState.tsx',
        organizationOverviewLoadingStateSource,
      ],
      [
        'src/components/Settings/OrganizationSharingLoadingState.tsx',
        organizationSharingLoadingStateSource,
      ],
    ];
    const settingsLoadingSkeletonConsumers: Array<[string, string]> = [
      ['src/components/Settings/DataHandlingPanel.tsx', dataHandlingPanelSource],
      ...organizationLoadingStateConsumers,
      ['src/components/Settings/SecurityOverviewPanel.tsx', securityOverviewPanelSource],
    ];

    expect(registeredRule?.canonical?.path).toBe(
      'src/components/shared/SettingsLoadingSkeleton.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('SettingsLoadingSkeleton');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      settingsLoadingSkeletonConsumers.map(([path]) => path),
    );
    expect(localSkeletonGuard?.canonical?.path).toBe(
      'src/components/shared/SettingsLoadingSkeleton.tsx',
    );
    expect(localSkeletonGuard?.canonical?.export).toBe('SettingsLoadingSkeleton');
    expect(localSkeletonGuard?.allPatterns).toEqual(['animate-pulse', 'bg-surface-hover']);
    expect(localSkeletonGuard?.scopes).toEqual(['src/components/Settings']);
    expect(localSkeletonGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(requiredSkeletonGuard?.canonical?.path).toBe(
      'src/components/shared/SettingsLoadingSkeleton.tsx',
    );
    expect(requiredSkeletonGuard?.pathIncludes).toEqual(['LoadingState']);
    expect(requiredSkeletonGuard?.triggerPatterns).toEqual(['LoadingState']);
    expect(requiredSkeletonGuard?.requiredPatterns).toEqual(['SettingsLoadingSkeleton']);

    expect(settingsLoadingSkeletonSource).toContain('export function SettingsLoadingSkeleton');
    expect(settingsLoadingSkeletonSource).toContain('export function SettingsSkeletonMetricGrid');
    expect(settingsLoadingSkeletonSource).toContain('export function SettingsSkeletonTable');
    expect(settingsLoadingSkeletonSource).toContain('animate-pulse');
    expect(settingsLoadingSkeletonSource).toContain('bg-surface-hover');
    expect(settingsLoadingSkeletonSource).toContain('bg-surface-alt');

    for (const [path, source] of settingsLoadingSkeletonConsumers) {
      expect(source).toContain('SettingsLoadingSkeleton');
      expect(source).not.toContain('animate-pulse');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }

    for (const [, source] of organizationLoadingStateConsumers) {
      expect(source).not.toContain('bg-surface-hover');
      expect(source).not.toContain('bg-surface-alt');
    }
  });

  it('keeps standard command buttons on the shared Button primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'button-command-shell');
    const actionIconRule = registry.rules?.find((rule) => rule.id === 'action-icon-button-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-secondary-command-local-shell',
    );
    const alertResourceActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'alert-resource-action-local-svg-button-shell',
    );
    const standaloneAgentMachineActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'standalone-agent-machine-action-icon-local-shell',
    );
    const aiChatActionIconHeaderGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'ai-chat-action-icon-header-local-shell',
    );
    const aiChatActionIconAccentGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'ai-chat-action-icon-accent-local-shell',
    );
    const aiChatActionIconPrimarySendGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'ai-chat-action-icon-primary-send-local-shell',
    );
    const aiChatActionIconWarningGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'ai-chat-action-icon-warning-local-shell',
    );
    const aiChatActionIconFooterGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'ai-chat-action-icon-footer-local-shell',
    );
    const aiChatMessageQueuedActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'ai-chat-message-queued-action-icon-local-shell',
    );
    const aiChatMessageCopyValueGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'ai-chat-message-copy-value-local-shell',
    );
    const aiChatToolCopyValueGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'ai-chat-tool-copy-value-local-shell',
    );
    const commandCopyGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-command-copy-local-shell',
    );
    const compactSettingsActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-compact-settings-action-local-shell',
    );
    const settingsActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-secondary-settings-action-local-shell',
    );
    const settingsOutlineActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-outline-settings-action-local-shell',
    );
    const settingsPrimaryActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-primary-settings-action-local-shell',
    );
    const settingsInfoActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-info-settings-action-local-shell',
    );
    const settingsSuccessActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-success-settings-action-local-shell',
    );
    const patrolSuccessApprovalActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-success-patrol-approval-action-local-shell',
    );
    const patrolWarningApprovalActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-warning-solid-patrol-approval-action-local-shell',
    );
    const patrolPrimaryApprovalActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-primary-patrol-approval-action-local-shell',
    );
    const patrolNeutralApprovalActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-neutral-patrol-approval-action-local-shell',
    );
    const settingsSuccessOutlineActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-success-outline-settings-action-local-shell',
    );
    const settingsSuccessGhostActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-success-ghost-settings-action-local-shell',
    );
    const settingsSuccessGhostRowActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-success-ghost-settings-row-action-local-shell',
    );
    const largeReportingActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-large-reporting-action-local-shell',
    );
    const settingsDangerActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-danger-settings-action-local-shell',
    );
    const settingsDangerOutlineActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-danger-outline-settings-action-local-shell',
    );
    const settingsDangerGhostRowActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-danger-ghost-settings-row-action-local-shell',
    );
    const settingsRowActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-outline-settings-row-action-local-shell',
    );
    const settingsDialogCloseGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-outline-settings-dialog-close-local-shell',
    );
    const billingAdminOrganizationRowActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'billing-admin-organization-row-action-local-shell',
    );
    const billingAdminOrganizationReloadActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'billing-admin-organization-reload-action-local-shell',
    );
    const ssoProviderPrimaryActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'sso-provider-primary-action-local-shell',
    );
    const ssoProviderSecondaryActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'sso-provider-secondary-action-local-shell',
    );
    const ssoProviderInlineFormActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'sso-provider-inline-form-action-local-shell',
    );
    const ssoProviderActionIconGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'sso-provider-action-icon-local-shell',
    );
    const ssoProviderCopyLinkGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'sso-provider-copy-link-local-shell',
    );
    const drawerHeaderActionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-drawer-header-action-local-shell',
    );
    const drawerHeaderIconGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'button-drawer-header-icon-local-shell',
    );
    const copyValueRule = registry.rules?.find((rule) => rule.id === 'copy-value-action-shell');
    const copyableCodeRowRule = registry.rules?.find(
      (rule) => rule.id === 'copyable-code-row-shell',
    );
    const ssoProviderActionRule = registry.rules?.find(
      (rule) => rule.id === 'sso-provider-settings-action-shell',
    );
    const copyValueNeutralGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'copy-value-neutral-local-button-shell',
    );
    const copyValueMutedGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'copy-value-muted-local-button-shell',
    );
    const copyValueAccentGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'copy-value-accent-local-button-shell',
    );
    const copyableCodeRowGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'copyable-code-row-local-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(registeredRule?.canonical?.export).toBe('Button');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/AI/Chat/ChatMessages.tsx',
      'src/components/patrol/ApprovalBanner.tsx',
      'src/components/patrol/ApprovalSection.tsx',
      'src/components/ErrorBoundary.tsx',
      'src/components/Infrastructure/ResourceDetailDrawer.tsx',
      'src/components/Infrastructure/ResourceDetailDrawerDebugTab.tsx',
      'src/components/Settings/AgentProfilesPanel.tsx',
      'src/components/Settings/APIAccessPanel.tsx',
      'src/components/Settings/AvailabilitySettingsPanel.tsx',
      'src/components/Settings/BillingAdminPanel.tsx',
      'src/components/Settings/BillingAdminOrganizationsTable.tsx',
      'src/components/Settings/ConnectionEditor/AddressProbeStep.tsx',
      'src/components/Settings/ConnectionEditor/ConnectionEditor.tsx',
      'src/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx',
      'src/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx',
      'src/components/Settings/ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx',
      'src/components/Settings/CopyCommandBlock.tsx',
      'src/components/Settings/DataHandlingPanel.tsx',
      'src/components/Settings/GeneralSettingsPanel.tsx',
      'src/components/Settings/InfrastructureAgentUpdatesDialog.tsx',
      'src/components/Settings/InfrastructureDiscoverySettingsDialog.tsx',
      'src/components/Settings/InfrastructureInstallerSection.tsx',
      'src/components/Settings/InfrastructureSourceManager.tsx',
      'src/components/Settings/InfrastructureWorkspace.tsx',
      'src/components/Settings/OrganizationAccessInvitationsSection.tsx',
      'src/components/Settings/OrganizationAccessManagementSection.tsx',
      'src/components/Settings/OrganizationAccessMembersSection.tsx',
      'src/components/Settings/OrganizationIncomingSharesSection.tsx',
      'src/components/Settings/OrganizationOutgoingSharesSection.tsx',
      'src/components/Settings/OrganizationOverviewDetailsSection.tsx',
      'src/components/Settings/OrganizationSharingCreateSection.tsx',
      'src/components/Settings/ProLicensePanel.tsx',
      'src/components/Settings/ProLicensePlanSection.tsx',
      'src/components/Settings/ReportingPanel.tsx',
      'src/components/Settings/ResourcePicker.tsx',
      'src/components/Settings/RolesPanel.tsx',
      'src/components/Settings/SelfHostedCommercialRecoverySection.tsx',
      'src/components/Settings/SecurityAuthPanel.tsx',
      'src/components/Settings/SSOProvidersPanel.tsx',
      'src/components/Settings/UserAssignmentsPanel.tsx',
      'src/components/UpdateConfirmationModal.tsx',
      'src/components/UpdateProgressModal.tsx',
      'src/components/Workloads/GuestDrawer.tsx',
      'src/features/patrol/PatrolIntelligenceWorkspace.tsx',
      'src/features/standalone/StandalonePageSurface.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/components/patrol/ApprovalBanner.tsx',
          patterns: expect.arrayContaining([
            'px-3 py-1.5 bg-green-600 hover:bg-green-700',
            'px-3 py-1.5 bg-surface-alt hover:bg-surface-hover',
            'px-3 py-1.5 bg-amber-600 hover:bg-amber-700',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/patrol/ApprovalSection.tsx',
          patterns: expect.arrayContaining([
            'px-3 py-1.5 bg-green-600 hover:bg-green-700',
            'px-3 py-1.5 bg-amber-600 hover:bg-amber-700',
            'px-3 py-1.5 bg-blue-600 hover:bg-blue-700',
            'px-3 py-1.5 hover:bg-surface-hover disabled:opacity-50 text-muted',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/GeneralSettingsPanel.tsx',
          patterns: expect.arrayContaining([
            'inline-flex items-center rounded-md border border-border bg-surface px-3 py-2 text-xs font-medium text-base-content transition hover:bg-surface-hover',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/BillingAdminPanel.tsx',
          patterns: expect.arrayContaining([
            'w-full sm:w-auto px-3 py-1.5 text-xs font-medium rounded-md border border-border bg-surface hover:bg-surface-hover disabled:opacity-50',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/BillingAdminOrganizationsTable.tsx',
          patterns: expect.arrayContaining([
            'px-2.5 py-1.5 text-xs font-medium rounded-md border border-border bg-surface hover:bg-surface-hover disabled:opacity-50',
            'px-2 py-1 text-xs rounded-md border border-border bg-surface hover:bg-surface-hover',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/APIAccessPanel.tsx',
          patterns: expect.arrayContaining([
            'inline-flex min-h-10 sm:min-h-10 w-fit items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ResourcePicker.tsx',
          patterns: expect.arrayContaining([
            'hover:border-red-500 hover:text-red-400',
            'w-full sm:w-auto min-h-10 sm:min-h-9 flex items-center justify-center gap-1.5 px-3 py-2.5 text-sm rounded-md border border-border',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/RolesPanel.tsx',
          patterns: expect.arrayContaining([
            'inline-flex w-full sm:w-auto min-h-10 sm:min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-blue-700',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/OrganizationAccessInvitationsSection.tsx',
          patterns: expect.arrayContaining([
            'inline-flex items-center justify-center rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60',
            'inline-flex items-center justify-center rounded-md border border-border bg-surface px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:border-red-300 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/OrganizationAccessManagementSection.tsx',
          patterns: expect.arrayContaining([
            'inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/OrganizationAccessMembersSection.tsx',
          patterns: expect.arrayContaining([
            'inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/OrganizationIncomingSharesSection.tsx',
          patterns: expect.arrayContaining([
            'inline-flex items-center rounded-md px-2 py-1 text-xs font-medium text-emerald-700 hover:bg-emerald-50 dark:text-emerald-300 dark:hover:bg-emerald-900 disabled:cursor-not-allowed disabled:opacity-60',
            'inline-flex items-center rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/OrganizationOutgoingSharesSection.tsx',
          patterns: expect.arrayContaining([
            'inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/OrganizationOverviewDetailsSection.tsx',
          patterns: expect.arrayContaining([
            'inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/OrganizationSharingCreateSection.tsx',
          patterns: expect.arrayContaining([
            'text-xs font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200',
            'inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ReportingPanel.tsx',
          patterns: expect.arrayContaining([
            'rounded-md border border-base-300 bg-base-100 px-4 py-2 text-sm font-medium text-base-content',
            'flex w-full items-center justify-center gap-2 rounded-md px-6 py-3 font-semibold transition-all sm:w-auto',
            'bg-blue-600 text-white hover:bg-blue-700',
            'bg-emerald-600 text-white hover:bg-emerald-700',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ProLicensePanel.tsx',
          patterns: expect.arrayContaining([
            'inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ProLicensePlanSection.tsx',
          patterns: expect.arrayContaining([
            'mt-2 inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-xs font-medium rounded-md border border-amber-300 dark:border-amber-700 text-amber-800 dark:text-amber-200 hover:bg-amber-100 dark:hover:bg-amber-800 transition-colors disabled:opacity-60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/SelfHostedCommercialRecoverySection.tsx',
          patterns: expect.arrayContaining([
            'min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed',
            'min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60 disabled:cursor-not-allowed',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/SecurityAuthPanel.tsx',
          patterns: expect.arrayContaining([
            'w-full sm:w-auto px-3 py-2 text-xs font-medium rounded-md border border-amber-300 text-amber-800 bg-amber-100 hover:bg-amber-200 transition-colors dark:border-amber-700 dark:text-amber-200 dark:bg-amber-900 dark:hover:bg-amber-800',
            'w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors',
            'w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/SSOProvidersPanel.tsx',
          patterns: expect.arrayContaining([
            'min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors flex items-center gap-1.5',
            'min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors flex items-center gap-1.5',
            'px-3 py-2 text-sm font-medium bg-surface-hover text-base-content rounded-md hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap',
            'px-4 py-2 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50',
            'px-4 py-2 text-sm font-medium bg-red-600 text-white rounded-md hover:bg-red-700',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/UserAssignmentsPanel.tsx',
          patterns: expect.arrayContaining([
            'inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium text-base-content hover:bg-surface-hover transition-colors',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/InfrastructureInstallerSection.tsx',
          patterns: expect.arrayContaining([
            'inline-flex items-center justify-center rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-700',
            'inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 transition-colors hover:bg-emerald-100',
            'inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100',
            'inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/ErrorBoundary.tsx',
          patterns: expect.arrayContaining([
            'px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700',
            'px-4 py-2 bg-slate-600 text-white rounded hover:bg-slate-700',
            'text-xs px-2 py-1 bg-red-600 text-white rounded hover:bg-red-700',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/UpdateConfirmationModal.tsx',
          patterns: expect.arrayContaining([
            'px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover rounded-md transition-colors',
            'px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/UpdateProgressModal.tsx',
          patterns: expect.arrayContaining([
            'mt-2 px-3 py-1.5 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 rounded',
            'px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors',
          ]),
        }),
      ]),
    );
    expect(errorBoundarySource).toContain("import { Button } from '@/components/shared/Button';");
    expect(errorBoundarySource).not.toContain(
      'px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700',
    );
    expect(errorBoundarySource).not.toContain(
      'px-4 py-2 bg-slate-600 text-white rounded hover:bg-slate-700',
    );
    expect(errorBoundarySource).not.toContain(
      'text-xs px-2 py-1 bg-red-600 text-white rounded hover:bg-red-700',
    );
    expect(updateConfirmationModalSource).toContain('ActionIconButton');
    expect(updateConfirmationModalSource).not.toContain(
      'px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover rounded-md transition-colors',
    );
    expect(updateConfirmationModalSource).not.toContain(
      'px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors',
    );
    expect(updateProgressModalSource).toContain('ActionIconButton');
    expect(updateProgressModalSource).not.toContain(
      'mt-2 px-3 py-1.5 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 rounded',
    );
    expect(updateProgressModalSource).not.toContain(
      'px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors',
    );
    expect(billingAdminOrganizationsTableSource).toContain('@/components/shared/Button');
    expect(billingAdminOrganizationsTableSource).toContain('<Button');
    expect(billingAdminOrganizationsTableSource).toContain('variant="secondary"');
    expect(billingAdminOrganizationsTableSource).toContain('size="sm"');
    expect(billingAdminOrganizationsTableSource).toContain('size="xs"');
    expect(billingAdminOrganizationsTableSource).not.toContain(
      'px-2.5 py-1.5 text-xs font-medium rounded-md border border-border bg-surface hover:bg-surface-hover disabled:opacity-50',
    );
    expect(billingAdminOrganizationsTableSource).not.toContain(
      'px-2 py-1 text-xs rounded-md border border-border bg-surface hover:bg-surface-hover',
    );
    expect(approvalBannerSource).toContain('@/components/shared/Button');
    expect(approvalBannerSource).toContain('<ButtonLink');
    expect(approvalBannerSource).toContain('variant="warningSolid"');
    expect(approvalBannerSource).not.toContain('px-3 py-1.5 bg-green-600 hover:bg-green-700');
    expect(approvalBannerSource).not.toContain('px-3 py-1.5 bg-surface-alt hover:bg-surface-hover');
    expect(approvalBannerSource).not.toContain('px-3 py-1.5 bg-amber-600 hover:bg-amber-700');
    expect(approvalSectionSource).toContain('@/components/shared/Button');
    expect(approvalSectionSource).toContain('<Button');
    expect(approvalSectionSource).toContain('<ButtonLink');
    expect(approvalSectionSource).toContain('variant="primary"');
    expect(approvalSectionSource).toContain('variant="ghost"');
    expect(approvalSectionSource).not.toContain('px-3 py-1.5 bg-green-600 hover:bg-green-700');
    expect(approvalSectionSource).not.toContain('px-3 py-1.5 bg-amber-600 hover:bg-amber-700');
    expect(approvalSectionSource).not.toContain('px-3 py-1.5 bg-blue-600 hover:bg-blue-700');
    expect(approvalSectionSource).not.toContain(
      'px-3 py-1.5 hover:bg-surface-hover disabled:opacity-50 text-muted',
    );
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/buttonModel.ts');
    expect(registeredGuard?.canonical?.export).toBe('getButtonClass');
    expect(registeredGuard?.allPatterns).toEqual([
      'rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content',
    ]);
    expect(registeredGuard?.scopes).toEqual(['src/components', 'src/features', 'src/pages']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual(['src/components/shared/Button.test.tsx']);
    expect(commandCopyGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(commandCopyGuard?.canonical?.export).toBe('CommandCopyButton');
    expect(commandCopyGuard?.allPatterns).toEqual([
      'absolute right-2 top-2',
      'items-center justify-center',
      'bg-surface-hover p-2',
    ]);
    expect(commandCopyGuard?.scopes).toEqual(['src/components', 'src/features', 'src/pages']);
    expect(commandCopyGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(commandCopyGuard?.ignoredPaths).toEqual(['src/components/shared/Button.test.tsx']);
    expect(compactSettingsActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(compactSettingsActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(compactSettingsActionGuard?.allPatterns).toEqual([
      'rounded-md border border-border bg-surface px-3 py-2 text-xs font-medium text-base-content',
    ]);
    expect(compactSettingsActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(compactSettingsActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(compactSettingsActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsActionGuard?.canonical?.path).toBe('src/components/shared/buttonModel.ts');
    expect(settingsActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsActionGuard?.allPatterns).toEqual([
      'rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content',
    ]);
    expect(settingsActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsActionGuard?.ignoredPaths).toEqual(['src/components/shared/Button.test.tsx']);
    expect(settingsOutlineActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(settingsOutlineActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsOutlineActionGuard?.allPatterns).toEqual([
      'rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content',
    ]);
    expect(settingsOutlineActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsOutlineActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsOutlineActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsPrimaryActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(settingsPrimaryActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsPrimaryActionGuard?.allPatterns).toEqual([
      'rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white',
    ]);
    expect(settingsPrimaryActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsPrimaryActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsPrimaryActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsInfoActionGuard?.canonical?.path).toBe('src/components/shared/buttonModel.ts');
    expect(settingsInfoActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsInfoActionGuard?.allPatterns).toEqual([
      'rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700',
    ]);
    expect(settingsInfoActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsInfoActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsInfoActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsSuccessActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(settingsSuccessActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsSuccessActionGuard?.allPatterns).toEqual([
      'rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white',
    ]);
    expect(settingsSuccessActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsSuccessActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsSuccessActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(patrolSuccessApprovalActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(patrolSuccessApprovalActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(patrolSuccessApprovalActionGuard?.allPatterns).toEqual([
      'px-3 py-1.5 bg-green-600 hover:bg-green-700',
      'text-white text-xs font-medium rounded',
    ]);
    expect(patrolSuccessApprovalActionGuard?.scopes).toEqual([
      'src/components/patrol',
      'src/features/patrol',
    ]);
    expect(patrolSuccessApprovalActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(patrolSuccessApprovalActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(patrolWarningApprovalActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(patrolWarningApprovalActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(patrolWarningApprovalActionGuard?.allPatterns).toEqual([
      'px-3 py-1.5 bg-amber-600 hover:bg-amber-700',
      'text-white text-xs font-medium rounded',
    ]);
    expect(patrolWarningApprovalActionGuard?.scopes).toEqual([
      'src/components/patrol',
      'src/features/patrol',
    ]);
    expect(patrolWarningApprovalActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(patrolWarningApprovalActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(patrolPrimaryApprovalActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(patrolPrimaryApprovalActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(patrolPrimaryApprovalActionGuard?.allPatterns).toEqual([
      'px-3 py-1.5 bg-blue-600 hover:bg-blue-700',
      'text-white text-xs font-medium rounded',
    ]);
    expect(patrolPrimaryApprovalActionGuard?.scopes).toEqual([
      'src/components/patrol',
      'src/features/patrol',
    ]);
    expect(patrolPrimaryApprovalActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(patrolPrimaryApprovalActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(patrolNeutralApprovalActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(patrolNeutralApprovalActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(patrolNeutralApprovalActionGuard?.allPatterns).toEqual([
      'px-3 py-1.5 bg-surface-alt hover:bg-surface-hover',
      'text-base-content text-xs font-medium rounded-md',
    ]);
    expect(patrolNeutralApprovalActionGuard?.scopes).toEqual([
      'src/components/patrol',
      'src/features/patrol',
    ]);
    expect(patrolNeutralApprovalActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(patrolNeutralApprovalActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsSuccessOutlineActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(settingsSuccessOutlineActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsSuccessOutlineActionGuard?.allPatterns).toEqual([
      'rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900',
    ]);
    expect(settingsSuccessOutlineActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsSuccessOutlineActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsSuccessOutlineActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsSuccessGhostActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(settingsSuccessGhostActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsSuccessGhostActionGuard?.allPatterns).toEqual([
      'rounded-md px-3 py-2 text-sm font-medium text-emerald-900',
    ]);
    expect(settingsSuccessGhostActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsSuccessGhostActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsSuccessGhostActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsSuccessGhostRowActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(settingsSuccessGhostRowActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsSuccessGhostRowActionGuard?.allPatterns).toEqual([
      'rounded-md px-2 py-1 text-xs font-medium text-emerald-700',
      'hover:bg-emerald-50 dark:text-emerald-300 dark:hover:bg-emerald-900',
    ]);
    expect(settingsSuccessGhostRowActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsSuccessGhostRowActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsSuccessGhostRowActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(largeReportingActionGuard?.canonical?.path).toBe('src/components/shared/buttonModel.ts');
    expect(largeReportingActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(largeReportingActionGuard?.allPatterns).toEqual([
      'flex w-full items-center justify-center gap-2 rounded-md px-6 py-3 font-semibold transition-all sm:w-auto',
    ]);
    expect(largeReportingActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(largeReportingActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(largeReportingActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsDangerActionGuard?.canonical?.path).toBe('src/components/shared/buttonModel.ts');
    expect(settingsDangerActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsDangerActionGuard?.allPatterns).toEqual([
      'rounded-md bg-rose-600 px-3 py-2 text-sm font-medium text-white',
    ]);
    expect(settingsDangerActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsDangerActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsDangerActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsDangerOutlineActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(settingsDangerOutlineActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsDangerOutlineActionGuard?.allPatterns).toEqual([
      'rounded-md border border-rose-300 px-3 py-2 text-sm font-medium text-rose-700',
    ]);
    expect(settingsDangerOutlineActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsDangerOutlineActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsDangerOutlineActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsDangerGhostRowActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(settingsDangerGhostRowActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsDangerGhostRowActionGuard?.allPatterns).toEqual([
      'rounded-md px-2 py-1 text-xs font-medium text-red-600',
      'hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900',
    ]);
    expect(settingsDangerGhostRowActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsDangerGhostRowActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsDangerGhostRowActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(settingsRowActionGuard?.canonical?.path).toBe('src/components/shared/buttonModel.ts');
    expect(settingsRowActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsRowActionGuard?.allPatterns).toEqual([
      'rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content',
    ]);
    expect(settingsRowActionGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsRowActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsRowActionGuard?.ignoredPaths).toEqual(['src/components/shared/Button.test.tsx']);
    expect(settingsDialogCloseGuard?.canonical?.path).toBe('src/components/shared/buttonModel.ts');
    expect(settingsDialogCloseGuard?.canonical?.export).toBe('getButtonClass');
    expect(settingsDialogCloseGuard?.allPatterns).toEqual([
      'h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover',
    ]);
    expect(settingsDialogCloseGuard?.scopes).toEqual([
      'src/components/Settings',
      'src/features',
      'src/pages',
    ]);
    expect(settingsDialogCloseGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(settingsDialogCloseGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(billingAdminOrganizationRowActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(billingAdminOrganizationRowActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(billingAdminOrganizationRowActionGuard?.allPatterns).toEqual([
      'px-2.5 py-1.5 text-xs font-medium rounded-md border border-border bg-surface hover:bg-surface-hover disabled:opacity-50',
    ]);
    expect(billingAdminOrganizationRowActionGuard?.scopes).toEqual([
      'src/components/Settings/BillingAdminOrganizationsTable.tsx',
    ]);
    expect(billingAdminOrganizationRowActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(billingAdminOrganizationRowActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(billingAdminOrganizationReloadActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(billingAdminOrganizationReloadActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(billingAdminOrganizationReloadActionGuard?.allPatterns).toEqual([
      'px-2 py-1 text-xs rounded-md border border-border bg-surface hover:bg-surface-hover',
    ]);
    expect(billingAdminOrganizationReloadActionGuard?.scopes).toEqual([
      'src/components/Settings/BillingAdminOrganizationsTable.tsx',
    ]);
    expect(billingAdminOrganizationReloadActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(billingAdminOrganizationReloadActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(ssoProviderPrimaryActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(ssoProviderPrimaryActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(ssoProviderPrimaryActionGuard?.allPatterns).toEqual([
      'min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md',
      'hover:bg-blue-700 transition-colors',
    ]);
    expect(ssoProviderPrimaryActionGuard?.scopes).toEqual([
      'src/components/Settings/SSOProvidersPanel.tsx',
    ]);
    expect(ssoProviderPrimaryActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(ssoProviderPrimaryActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(ssoProviderSecondaryActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(ssoProviderSecondaryActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(ssoProviderSecondaryActionGuard?.allPatterns).toEqual([
      'text-sm font-medium border border-border text-base-content rounded-md',
      'hover:bg-surface-hover transition-colors',
    ]);
    expect(ssoProviderSecondaryActionGuard?.scopes).toEqual([
      'src/components/Settings/SSOProvidersPanel.tsx',
    ]);
    expect(ssoProviderSecondaryActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(ssoProviderSecondaryActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(ssoProviderInlineFormActionGuard?.canonical?.path).toBe(
      'src/components/shared/buttonModel.ts',
    );
    expect(ssoProviderInlineFormActionGuard?.canonical?.export).toBe('getButtonClass');
    expect(ssoProviderInlineFormActionGuard?.allPatterns).toEqual([
      'px-3 py-2 text-sm font-medium bg-surface-hover text-base-content rounded-md',
      'disabled:cursor-not-allowed whitespace-nowrap',
    ]);
    expect(ssoProviderInlineFormActionGuard?.scopes).toEqual([
      'src/components/Settings/SSOProvidersPanel.tsx',
    ]);
    expect(ssoProviderInlineFormActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(ssoProviderInlineFormActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(ssoProviderActionIconGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(ssoProviderActionIconGuard?.canonical?.export).toBe('ActionIconButton');
    expect(ssoProviderActionIconGuard?.allPatterns).toEqual([
      'p-2 text-slate-500',
      'rounded-md transition-colors',
    ]);
    expect(ssoProviderActionIconGuard?.scopes).toEqual([
      'src/components/Settings/SSOProvidersPanel.tsx',
    ]);
    expect(ssoProviderActionIconGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(ssoProviderActionIconGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(ssoProviderCopyLinkGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(ssoProviderCopyLinkGuard?.canonical?.export).toBe('CopyValueButton');
    expect(ssoProviderCopyLinkGuard?.allPatterns).toEqual([
      'text-blue-600 hover:underline flex items-center gap-1',
    ]);
    expect(ssoProviderCopyLinkGuard?.scopes).toEqual([
      'src/components/Settings/SSOProvidersPanel.tsx',
    ]);
    expect(ssoProviderCopyLinkGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(ssoProviderCopyLinkGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(drawerHeaderActionGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(drawerHeaderActionGuard?.canonical?.export).toBe('DrawerHeaderActionButton');
    expect(drawerHeaderActionGuard?.allPatterns).toEqual([
      'inline-flex h-8 items-center gap-1.5 rounded border border-border bg-surface px-2 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
    ]);
    expect(drawerHeaderActionGuard?.scopes).toEqual([
      'src/components',
      'src/features',
      'src/pages',
    ]);
    expect(drawerHeaderActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(drawerHeaderActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(drawerHeaderIconGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(drawerHeaderIconGuard?.canonical?.export).toBe('DrawerHeaderIconButton');
    expect(drawerHeaderIconGuard?.allPatterns).toEqual([
      'inline-flex h-8 w-8 items-center justify-center rounded-md hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
    ]);
    expect(drawerHeaderIconGuard?.scopes).toEqual(['src/components', 'src/features', 'src/pages']);
    expect(drawerHeaderIconGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(drawerHeaderIconGuard?.ignoredPaths).toEqual(['src/components/shared/Button.test.tsx']);
    expect(actionIconRule?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(actionIconRule?.canonical?.export).toBe('ActionIconButton');
    expect(actionIconRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Alerts/AlertResourceTableDesktop.tsx',
      'src/components/Alerts/AlertResourceTableMobile.tsx',
      'src/components/Alerts/AlertResourceTableRow.tsx',
      'src/components/Alerts/ResourceTable.tsx',
      'src/components/AI/Chat/index.tsx',
      'src/components/AI/Chat/MessageItem.tsx',
      'src/components/Settings/RolesPanel.tsx',
      'src/components/Settings/SSOProvidersPanel.tsx',
      'src/features/standalone/AgentsMachinesTable.tsx',
    ]);
    expect(actionIconRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/components/Alerts/AlertResourceTableDesktop.tsx',
          patterns: expect.arrayContaining([
            '<svg',
            'class="p-1 hover:text-muted',
            'class="p-1 text-red-600',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Alerts/AlertResourceTableMobile.tsx',
          patterns: expect.arrayContaining([
            '<svg',
            'class="p-1.5 bg-blue-50',
            'class="p-1.5 bg-surface-hover',
            'class="p-1.5 bg-green-50',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Alerts/AlertResourceTableRow.tsx',
          patterns: expect.arrayContaining([
            '<svg',
            'class="p-1 hover:text-muted',
            'class="p-1 text-blue-600',
            'class="p-1 hover:text-base-content',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Alerts/ResourceTable.tsx',
          patterns: expect.arrayContaining([
            '<svg',
            'text-slate-400 hover:text-white bg-surface hover:bg-slate-700 rounded-full p-1.5',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/AI/Chat/index.tsx',
          patterns: expect.arrayContaining([
            'flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted',
            'flex-shrink-0 p-2 hover:text-base-content rounded-md hover:bg-surface-hover transition-colors',
            'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 disabled:cursor-wait disabled:opacity-70 dark:text-blue-200 dark:hover:bg-blue-900/60',
            'rounded p-1 text-muted opacity-0 transition-opacity hover:bg-blue-100 hover:text-blue-600 focus:opacity-100 group-hover:opacity-100 group-focus-within:opacity-100 dark:hover:bg-blue-900 dark:hover:text-blue-300',
            'order-2 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md hover:text-base-content hover:bg-surface-hover transition-colors sm:order-none',
            'flex h-7 w-7 items-center justify-center rounded-md border border-amber-200 bg-surface text-amber-700 transition-colors hover:bg-amber-100 hover:text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-200 dark:hover:bg-amber-900',
            'flex h-7 w-7 items-center justify-center rounded-md text-amber-700 transition-colors hover:bg-amber-100 hover:text-amber-900 dark:text-amber-200 dark:hover:bg-amber-900',
            'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 dark:text-blue-200 dark:hover:bg-blue-900/60',
            'flex h-9 w-9 items-center justify-center rounded-md bg-blue-600 text-white shadow-sm transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-45',
            'flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/AI/Chat/MessageItem.tsx',
          patterns: expect.arrayContaining([
            'inline-flex h-5 w-5 items-center justify-center rounded text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 focus:bg-blue-100 focus:outline-none focus:ring-2 focus:ring-blue-500/30',
          ]),
        }),
        expect.objectContaining({
          path: 'src/features/standalone/AgentsMachinesTable.tsx',
          patterns: expect.arrayContaining([
            'inline-flex h-7 w-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/SSOProvidersPanel.tsx',
          patterns: expect.arrayContaining([
            'p-2 text-slate-500 hover:text-blue-600 hover:bg-surface-hover rounded-md transition-colors',
            'p-2 text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900 rounded-md transition-colors',
            'class="text-slate-400 hover:text-base-content"',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/RolesPanel.tsx',
          patterns: expect.arrayContaining([
            'p-1.5 rounded-md text-slate-500 hover:text-blue-600 hover:bg-surface-hover dark:hover:text-blue-300',
            'p-1.5 rounded-md text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:text-red-400 dark:hover:bg-red-900',
          ]),
        }),
      ]),
    );
    expect(alertResourceActionGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(alertResourceActionGuard?.canonical?.export).toBe('ActionIconButton');
    expect(alertResourceActionGuard?.allPatterns).toEqual(['<button', '<svg', 'Edit thresholds']);
    expect(alertResourceActionGuard?.scopes).toEqual(['src/components/Alerts']);
    expect(alertResourceActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(alertResourceActionGuard?.ignoredPaths).toEqual([
      'src/components/Alerts/ResourceTable.test.tsx',
      'src/components/shared/Button.test.tsx',
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);
    expect(standaloneAgentMachineActionGuard?.canonical?.path).toBe(
      'src/components/shared/Button.tsx',
    );
    expect(standaloneAgentMachineActionGuard?.canonical?.export).toBe('ActionIconButton');
    expect(standaloneAgentMachineActionGuard?.allPatterns).toEqual([
      'inline-flex h-7 w-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60',
    ]);
    expect(standaloneAgentMachineActionGuard?.scopes).toEqual([
      'src/features/standalone/AgentsMachinesTable.tsx',
    ]);
    expect(standaloneAgentMachineActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(standaloneAgentMachineActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    expect(aiChatMessageQueuedActionGuard?.canonical?.path).toBe(
      'src/components/shared/Button.tsx',
    );
    expect(aiChatMessageQueuedActionGuard?.canonical?.export).toBe('ActionIconButton');
    expect(aiChatMessageQueuedActionGuard?.allPatterns).toEqual([
      'inline-flex h-5 w-5 items-center justify-center rounded text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 focus:bg-blue-100 focus:outline-none focus:ring-2 focus:ring-blue-500/30',
    ]);
    expect(aiChatMessageQueuedActionGuard?.scopes).toEqual([
      'src/components/AI/Chat/MessageItem.tsx',
    ]);
    expect(aiChatMessageQueuedActionGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(aiChatMessageQueuedActionGuard?.ignoredPaths).toEqual([
      'src/components/shared/Button.test.tsx',
    ]);
    for (const guard of [
      aiChatActionIconHeaderGuard,
      aiChatActionIconAccentGuard,
      aiChatActionIconPrimarySendGuard,
      aiChatActionIconWarningGuard,
      aiChatActionIconFooterGuard,
    ]) {
      expect(guard?.canonical?.path).toBe('src/components/shared/Button.tsx');
      expect(guard?.canonical?.export).toBe('ActionIconButton');
      expect(guard?.scopes).toEqual(['src/components/AI/Chat']);
      expect(guard?.allowedPaths ?? []).toHaveLength(0);
      expect(guard?.ignoredPaths).toEqual([
        'src/components/AI/Chat/__tests__/AIChat.test.tsx',
        'src/components/shared/Button.test.tsx',
      ]);
    }
    expect(aiChatActionIconHeaderGuard?.allPatterns).toEqual([
      'flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content',
    ]);
    expect(aiChatActionIconAccentGuard?.allPatterns).toEqual([
      'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950',
    ]);
    expect(aiChatActionIconPrimarySendGuard?.allPatterns).toEqual([
      'flex h-9 w-9 items-center justify-center rounded-md bg-blue-600 text-white shadow-sm transition-colors hover:bg-blue-700',
    ]);
    expect(aiChatActionIconWarningGuard?.allPatterns).toEqual([
      'flex h-7 w-7 items-center justify-center rounded-md border border-amber-200 bg-surface text-amber-700 transition-colors hover:bg-amber-100',
    ]);
    expect(aiChatActionIconFooterGuard?.allPatterns).toEqual([
      'flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content',
    ]);
    expect(copyValueRule?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(copyValueRule?.canonical?.export).toBe('CopyValueButton');
    expect(copyValueRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/AI/Chat/MessageItem.tsx',
      'src/components/AI/Chat/ToolExecutionBlock.tsx',
      'src/components/Discovery/DiscoveryTab.tsx',
      'src/components/Settings/SSOProvidersPanel.tsx',
      'src/components/shared/WebInterfaceUrlField.tsx',
    ]);
    expect(copyValueRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/components/AI/Chat/MessageItem.tsx',
          patterns: expect.arrayContaining([
            'lucide-solid/icons/copy',
            "lucide-solid/icons/check';",
            'mt-1 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border-subtle bg-surface text-muted opacity-0 shadow-sm transition-opacity hover:text-base-content',
            'ml-auto inline-flex h-7 w-7 items-center justify-center rounded-md border border-border-subtle bg-surface text-muted opacity-0 shadow-sm transition-opacity hover:text-base-content',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/AI/Chat/ToolExecutionBlock.tsx',
          patterns: expect.arrayContaining([
            'lucide-solid/icons/copy',
            "lucide-solid/icons/check';",
            'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-border-subtle bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30',
          ]),
        }),
      ]),
    );
    for (const guard of [aiChatMessageCopyValueGuard, aiChatToolCopyValueGuard]) {
      expect(guard?.canonical?.path).toBe('src/components/shared/Button.tsx');
      expect(guard?.canonical?.export).toBe('CopyValueButton');
      expect(guard?.allowedPaths ?? []).toHaveLength(0);
      expect(guard?.ignoredPaths).toEqual(['src/components/shared/Button.test.tsx']);
    }
    expect(aiChatMessageCopyValueGuard?.scopes).toEqual(['src/components/AI/Chat/MessageItem.tsx']);
    expect(aiChatMessageCopyValueGuard?.allPatterns).toEqual([
      'inline-flex h-7 w-7',
      'border border-border-subtle bg-surface text-muted opacity-0 shadow-sm transition-opacity hover:text-base-content',
      'CopyIcon class="h-3.5 w-3.5"',
    ]);
    expect(aiChatToolCopyValueGuard?.scopes).toEqual([
      'src/components/AI/Chat/ToolExecutionBlock.tsx',
    ]);
    expect(aiChatToolCopyValueGuard?.allPatterns).toEqual([
      'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-border-subtle bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30',
      'CopyIcon class="h-3 w-3"',
    ]);
    expect(copyableCodeRowRule?.canonical?.path).toBe('src/components/shared/CopyableCodeRow.tsx');
    expect(copyableCodeRowRule?.canonical?.export).toBe('CopyableCodeRow');
    expect(copyableCodeRowRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Discovery/DiscoveryTab.tsx',
    ]);
    expect(ssoProviderActionRule?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(ssoProviderActionRule?.canonical?.export).toBe('Button');
    expect(ssoProviderActionRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/SSOProvidersPanel.tsx',
    ]);
    expect(ssoProviderActionRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/Settings/SSOProvidersPanel.tsx',
        patterns: [
          '<button',
          'lucide-solid/icons/copy',
          'min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium bg-blue-600',
          'p-2 text-slate-500 hover:text-blue-600',
          'text-blue-600 hover:underline flex items-center gap-1',
        ],
      },
    ]);
    expect(copyValueNeutralGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(copyValueNeutralGuard?.canonical?.export).toBe('CopyValueButton');
    expect(copyValueNeutralGuard?.allPatterns).toEqual([
      'inline-flex min-h-7 min-w-7 shrink-0 items-center justify-center rounded border border-border bg-surface px-2 text-muted transition-colors hover:bg-surface-hover hover:text-base-content',
      'fallback={<CopyIcon class="h-3.5 w-3.5" />}',
    ]);
    expect(copyValueMutedGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(copyValueMutedGuard?.canonical?.export).toBe('CopyValueButton');
    expect(copyValueMutedGuard?.allPatterns).toEqual([
      'inline-flex min-h-8 min-w-8 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content',
      'fallback={<CopyIcon class="h-3.5 w-3.5" />}',
    ]);
    expect(copyValueAccentGuard?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(copyValueAccentGuard?.canonical?.export).toBe('CopyValueButton');
    expect(copyValueAccentGuard?.allPatterns).toEqual([
      'inline-flex min-h-7 min-w-7 shrink-0 items-center justify-center rounded text-blue-700 transition-colors hover:bg-blue-100 dark:text-blue-200 dark:hover:bg-blue-950',
      'fallback={<CopyIcon class="h-3.5 w-3.5" />}',
    ]);
    expect(copyableCodeRowGuard?.canonical?.path).toBe('src/components/shared/CopyableCodeRow.tsx');
    expect(copyableCodeRowGuard?.canonical?.export).toBe('CopyableCodeRow');
    expect(copyableCodeRowGuard?.allPatterns).toEqual([
      'flex items-start gap-2 rounded bg-surface-alt px-2 py-1.5',
      'break-all font-mono text-xs text-base-content',
    ]);

    expect(buttonSource).toContain('export function Button');
    expect(buttonSource).toContain('export function CommandCopyButton');
    expect(buttonSource).toContain('export function CopyValueButton');
    expect(buttonSource).toContain('export function ActionIconButton');
    expect(buttonSource).toContain('export function DrawerHeaderActionButton');
    expect(buttonSource).toContain('export function DrawerHeaderActionGroup');
    expect(buttonSource).toContain('export function DrawerHeaderIconButton');
    expect(buttonSource).toContain('export function ButtonLink');
    expect(buttonSource).toContain('getButtonClass');
    expect(buttonSource).toContain('getCopyValueButtonClass');
    expect(buttonSource).toContain('getActionIconButtonClass');
    expect(copyableCodeRowSource).toContain('CopyValueButton');
    expect(buttonModelSource).toContain('BUTTON_VARIANT_CLASSES');
    expect(buttonModelSource).toContain('BUTTON_SIZE_CLASSES');
    expect(buttonModelSource).toContain('primaryFlat:');
    expect(buttonModelSource).toContain('warning:');
    expect(buttonModelSource).toContain('info:');
    expect(buttonModelSource).toContain('settingsActionXs:');
    expect(buttonModelSource).toContain('success:');
    expect(buttonModelSource).toContain('successOutline:');
    expect(buttonModelSource).toContain('successGhost:');
    expect(buttonModelSource).toContain('dangerGhost:');
    expect(buttonModelSource).toContain('COPY_VALUE_BUTTON_VARIANT_CLASSES');
    expect(buttonModelSource).toContain('COPY_VALUE_BUTTON_SIZE_CLASSES');
    expect(buttonModelSource).toContain('ACTION_ICON_BUTTON_TONE_CLASSES');
    expect(buttonModelSource).toContain('ACTION_ICON_BUTTON_SIZE_CLASSES');
    expect(buttonModelSource).toContain('outline:');
    expect(buttonModelSource).toContain('outlineSelected:');
    expect(buttonModelSource).toContain('primary:');
    expect(buttonModelSource).toContain('accentGhost:');
    expect(buttonModelSource).toContain('warningGhost:');
    expect(buttonModelSource).toContain('warningOutline:');
    expect(buttonModelSource).toContain('infoGhost:');
    expect(buttonModelSource).toContain(["'2xs'", ": 'h-5 w-5'"].join(''));
    expect(buttonModelSource).toContain(['lg', ": 'h-9 w-9'"].join(''));
    expect(buttonModelSource).toContain('dangerOutline:');
    expect(buttonModelSource).toContain('settingsAction:');
    expect(buttonModelSource).toContain('getCopyValueButtonClass');
    expect(buttonModelSource).toContain('getActionIconButtonClass');
    expect(buttonModelSource).toContain('getDrawerHeaderActionButtonClass');
    expect(buttonModelSource).toContain('getDrawerHeaderIconButtonClass');
    for (const source of [
      alertResourceTableSource,
      alertResourceTableDesktopSource,
      alertResourceTableMobileSource,
      alertResourceTableRowSource,
    ]) {
      expect(source).toContain('ActionIconButton');
      expect(source).not.toContain(['<', 'svg'].join(''));
    }
    for (const drawerSource of [guestDrawerSource, resourceDetailDrawerSource]) {
      expect(drawerSource).toContain('@/components/shared/Button');
      expect(drawerSource).toContain('DrawerHeaderActionGroup');
      expect(drawerSource).toContain('DrawerHeaderActionButton');
      expect(drawerSource).toContain('DrawerHeaderIconButton');
      expect(drawerSource).not.toContain(
        'inline-flex h-8 items-center gap-1.5 rounded border border-border bg-surface px-2 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
      );
      expect(drawerSource).not.toContain(
        'inline-flex h-8 w-8 items-center justify-center rounded-md hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
      );
    }
    expect(chatMessagesSource).toContain('@/components/shared/Button');
    expect(chatMessagesSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content',
    );
    expect(aiChatSource).toContain('@/components/shared/Button');
    expect(aiChatSource).toContain('ActionIconButton');
    for (const retiredActionIconShell of [
      'flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted',
      'flex-shrink-0 p-2 hover:text-base-content rounded-md hover:bg-surface-hover transition-colors',
      'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 disabled:cursor-wait disabled:opacity-70 dark:text-blue-200 dark:hover:bg-blue-900/60',
      'rounded p-1 text-muted opacity-0 transition-opacity hover:bg-blue-100 hover:text-blue-600 focus:opacity-100 group-hover:opacity-100 group-focus-within:opacity-100 dark:hover:bg-blue-900 dark:hover:text-blue-300',
      'order-2 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md hover:text-base-content hover:bg-surface-hover transition-colors sm:order-none',
      'flex h-7 w-7 items-center justify-center rounded-md border border-amber-200 bg-surface text-amber-700 transition-colors hover:bg-amber-100 hover:text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-200 dark:hover:bg-amber-900',
      'flex h-7 w-7 items-center justify-center rounded-md text-amber-700 transition-colors hover:bg-amber-100 hover:text-amber-900 dark:text-amber-200 dark:hover:bg-amber-900',
      'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 dark:text-blue-200 dark:hover:bg-blue-900/60',
      'flex h-9 w-9 items-center justify-center rounded-md bg-blue-600 text-white shadow-sm transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-45',
      'flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30',
    ]) {
      expect(aiChatSource).not.toContain(retiredActionIconShell);
    }
    expect(agentsMachinesTableSource).toContain('@/components/shared/Button');
    expect(agentsMachinesTableSource).toContain('ActionIconButton');
    expect(agentsMachinesTableSource).not.toContain(
      'inline-flex h-7 w-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60',
    );
    expect(messageItemSource).toContain('@/components/shared/Button');
    expect(messageItemSource).toContain('ActionIconButton');
    expect(messageItemSource).toContain('CopyValueButton');
    expect(messageItemSource).not.toContain('lucide-solid/icons/copy');
    expect(messageItemSource).not.toContain("lucide-solid/icons/check';");
    expect(messageItemSource).not.toContain(
      'mt-1 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border-subtle bg-surface text-muted opacity-0 shadow-sm transition-opacity hover:text-base-content',
    );
    expect(messageItemSource).not.toContain(
      'ml-auto inline-flex h-7 w-7 items-center justify-center rounded-md border border-border-subtle bg-surface text-muted opacity-0 shadow-sm transition-opacity hover:text-base-content',
    );
    expect(messageItemSource).not.toContain(
      'inline-flex h-5 w-5 items-center justify-center rounded text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 focus:bg-blue-100 focus:outline-none focus:ring-2 focus:ring-blue-500/30',
    );
    expect(toolExecutionBlockSource).toContain('@/components/shared/Button');
    expect(toolExecutionBlockSource).toContain('CopyValueButton');
    expect(toolExecutionBlockSource).not.toContain('lucide-solid/icons/copy');
    expect(toolExecutionBlockSource).not.toContain("lucide-solid/icons/check';");
    expect(toolExecutionBlockSource).not.toContain(
      'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-border-subtle bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30',
    );
    expect(resourceDetailDrawerDebugTabSource).toContain('@/components/shared/Button');
    expect(resourceDetailDrawerDebugTabSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content',
    );
    expect(agentProfilesPanelSource).toContain('@/components/shared/Button');
    expect(agentProfilesPanelSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content',
    );
    expect(ssoProvidersPanelSource).toContain('@/components/shared/Button');
    expect(ssoProvidersPanelSource).toContain('ActionIconButton');
    expect(ssoProvidersPanelSource).toContain('CopyValueButton');
    expect(ssoProvidersPanelSource).not.toContain('<button');
    expect(ssoProvidersPanelSource).not.toContain('lucide-solid/icons/copy');
    expect(ssoProvidersPanelSource).not.toContain(
      'min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium bg-blue-600',
    );
    expect(ssoProvidersPanelSource).not.toContain('p-2 text-slate-500 hover:text-blue-600');
    expect(ssoProvidersPanelSource).not.toContain(
      'text-blue-600 hover:underline flex items-center gap-1',
    );
    expect(apiAccessPanelSource).toContain('@/components/shared/Button');
    expect(apiAccessPanelSource).toContain('ButtonLink');
    expect(apiAccessPanelSource).toContain('variant="info"');
    expect(apiAccessPanelSource).not.toContain(
      'inline-flex min-h-10 sm:min-h-10 w-fit items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700',
    );
    expect(availabilitySettingsPanelSource).toContain('@/components/shared/Button');
    expect(availabilitySettingsPanelSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content',
    );
    expect(availabilitySettingsPanelSource).not.toContain(
      'rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content',
    );
    expect(availabilitySettingsPanelSource).not.toContain(
      'h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover',
    );
    expect(addressProbeStepSource).toContain('@/components/shared/Button');
    expect(addressProbeStepSource).toContain('size="settingsAction"');
    expect(addressProbeStepSource).not.toContain(
      'rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white',
    );
    expect(connectionEditorSource).toContain('@/components/shared/Button');
    expect(connectionEditorSource).toContain('size="settingsAction"');
    expect(connectionEditorSource).not.toContain(
      'rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content',
    );
    expect(connectionEditorSource).not.toContain(
      'rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content',
    );
    for (const credentialSlotSource of [
      availabilityTargetSlotSource,
      trueNASCredentialSlotSource,
      vmwareCredentialSlotSource,
    ]) {
      expect(credentialSlotSource).toContain('@/components/shared/Button');
      expect(credentialSlotSource).toContain('size="settingsAction"');
      expect(credentialSlotSource).not.toContain(
        'rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content',
      );
      expect(credentialSlotSource).not.toContain(
        'rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white',
      );
      expect(credentialSlotSource).not.toContain(
        'rounded-md bg-rose-600 px-3 py-2 text-sm font-medium text-white',
      );
      expect(credentialSlotSource).not.toContain(
        'rounded-md border border-rose-300 px-3 py-2 text-sm font-medium text-rose-700',
      );
    }
    expect(copyCommandBlockSource).toContain('@/components/shared/Button');
    expect(copyCommandBlockSource).toContain('CommandCopyButton');
    expect(copyCommandBlockSource).not.toContain('absolute right-2 top-2');
    expect(copyCommandBlockSource).not.toContain('bg-surface-hover p-2');
    expect(dataHandlingPanelSource).toContain('@/components/shared/Button');
    expect(dataHandlingPanelSource).toContain('ButtonLink');
    expect(dataHandlingPanelSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content',
    );
    expect(generalSettingsPanelSource).toContain('@/components/shared/Button');
    expect(generalSettingsPanelSource).toContain('size="settingsActionXs"');
    expect(generalSettingsPanelSource).not.toContain(
      'inline-flex items-center rounded-md border border-border bg-surface px-3 py-2 text-xs font-medium text-base-content transition hover:bg-surface-hover',
    );
    expect(billingAdminPanelSource).toContain('@/components/shared/Button');
    expect(billingAdminPanelSource).toContain('size="sm"');
    expect(billingAdminPanelSource).not.toContain(
      'w-full sm:w-auto px-3 py-1.5 text-xs font-medium rounded-md border border-border bg-surface hover:bg-surface-hover disabled:opacity-50',
    );
    expect(infrastructureInstallerSectionSource).toContain('@/components/shared/Button');
    expect(infrastructureInstallerSectionSource).toContain('CommandCopyButton');
    expect(infrastructureInstallerSectionSource).toContain('variant="success"');
    expect(infrastructureInstallerSectionSource).toContain('variant="successOutline"');
    expect(infrastructureInstallerSectionSource).toContain('variant="successGhost"');
    expect(infrastructureInstallerSectionSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content',
    );
    expect(infrastructureInstallerSectionSource).not.toContain(
      'inline-flex items-center justify-center rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-700',
    );
    expect(infrastructureInstallerSectionSource).not.toContain(
      'inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 transition-colors hover:bg-emerald-100',
    );
    expect(infrastructureInstallerSectionSource).not.toContain(
      'inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100',
    );
    expect(infrastructureInstallerSectionSource).not.toContain(
      'inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100',
    );
    expect(infrastructureInstallerSectionSource).not.toContain('absolute right-2 top-2');
    expect(infrastructureInstallerSectionSource).not.toContain('bg-surface-hover p-2');
    expect(infrastructureAgentUpdatesDialogSource).toContain('@/components/shared/Button');
    expect(infrastructureAgentUpdatesDialogSource).toContain('CommandCopyButton');
    expect(infrastructureAgentUpdatesDialogSource).not.toContain('absolute right-2 top-2');
    expect(infrastructureAgentUpdatesDialogSource).not.toContain('bg-surface-hover p-2');
    expect(infrastructureAgentUpdatesDialogSource).not.toContain(
      'h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover',
    );
    expect(infrastructureDiscoverySettingsDialogSource).toContain('@/components/shared/Button');
    expect(infrastructureDiscoverySettingsDialogSource).not.toContain(
      'h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover',
    );
    expect(infrastructureSourceManagerSource).toContain('@/components/shared/Button');
    expect(infrastructureSourceManagerSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content',
    );
    expect(infrastructureSourceManagerSource).not.toContain(
      'rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content',
    );
    expect(infrastructureWorkspaceSource).toContain('@/components/shared/Button');
    expect(infrastructureWorkspaceSource).toContain('size="settingsAction"');
    expect(infrastructureWorkspaceSource).not.toContain(
      'rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content',
    );
    expect(infrastructureWorkspaceSource).not.toContain(
      'rounded-md bg-rose-600 px-3 py-2 text-sm font-medium text-white',
    );
    expect(infrastructureWorkspaceSource).not.toContain(
      'rounded-md border border-rose-300 px-3 py-2 text-sm font-medium text-rose-700',
    );
    expect(infrastructureWorkspaceSource).not.toContain(
      'rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content',
    );
    expect(infrastructureWorkspaceSource).not.toContain(
      'h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover',
    );
    expect(resourcePickerSource).toContain('@/components/shared/Button');
    expect(resourcePickerSource).toContain('<Button');
    expect(resourcePickerSource).not.toContain('hover:border-red-500 hover:text-red-400');
    expect(resourcePickerSource).not.toContain(
      'w-full sm:w-auto min-h-10 sm:min-h-9 flex items-center justify-center gap-1.5 px-3 py-2.5 text-sm rounded-md border border-border',
    );
    expect(rolesPanelSource).toContain('@/components/shared/Button');
    expect(rolesPanelSource).toContain('<Button');
    expect(rolesPanelSource).toContain('ActionIconButton');
    expect(rolesPanelSource).toContain('variant="primary"');
    expect(rolesPanelSource).toContain('size="settingsAction"');
    expect(rolesPanelSource).not.toContain(
      'inline-flex w-full sm:w-auto min-h-10 sm:min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-blue-700',
    );
    expect(rolesPanelSource).not.toContain(
      'p-1.5 rounded-md text-slate-500 hover:text-blue-600 hover:bg-surface-hover dark:hover:text-blue-300',
    );
    expect(rolesPanelSource).not.toContain(
      'p-1.5 rounded-md text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:text-red-400 dark:hover:bg-red-900',
    );
    for (const organizationActionSource of [
      organizationAccessInvitationsSectionSource,
      organizationAccessManagementSectionSource,
      organizationAccessMembersSectionSource,
      organizationIncomingSharesSectionSource,
      organizationOutgoingSharesSectionSource,
      organizationOverviewDetailsSectionSource,
      organizationSharingCreateSectionSource,
    ]) {
      expect(organizationActionSource).toContain('@/components/shared/Button');
      expect(organizationActionSource).toContain('<Button');
    }
    for (const organizationPrimarySource of [
      organizationAccessInvitationsSectionSource,
      organizationAccessManagementSectionSource,
      organizationOverviewDetailsSectionSource,
      organizationSharingCreateSectionSource,
    ]) {
      expect(organizationPrimarySource).toContain('variant="primary"');
    }
    for (const organizationDangerGhostSource of [
      organizationAccessMembersSectionSource,
      organizationIncomingSharesSectionSource,
      organizationOutgoingSharesSectionSource,
    ]) {
      expect(organizationDangerGhostSource).toContain('variant="dangerGhost"');
      expect(organizationDangerGhostSource).toContain('size="xs"');
    }
    expect(organizationIncomingSharesSectionSource).toContain('variant="successGhost"');
    expect(organizationAccessInvitationsSectionSource).toContain('variant="dangerOutline"');
    expect(organizationSharingCreateSectionSource).toContain('variant="ghost"');
    for (const retiredOrganizationActionPattern of [
      'inline-flex items-center justify-center rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60',
      'inline-flex items-center justify-center rounded-md border border-border bg-surface px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:border-red-300 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-60',
      'inline-flex w-full sm:w-auto items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60',
      'inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60',
      'inline-flex items-center rounded-md px-2 py-1 text-xs font-medium text-emerald-700 hover:bg-emerald-50 dark:text-emerald-300 dark:hover:bg-emerald-900 disabled:cursor-not-allowed disabled:opacity-60',
      'inline-flex items-center rounded-md px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900 disabled:cursor-not-allowed disabled:opacity-60',
      'text-xs font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200',
    ]) {
      for (const organizationActionSource of [
        organizationAccessInvitationsSectionSource,
        organizationAccessManagementSectionSource,
        organizationAccessMembersSectionSource,
        organizationIncomingSharesSectionSource,
        organizationOutgoingSharesSectionSource,
        organizationOverviewDetailsSectionSource,
        organizationSharingCreateSectionSource,
      ]) {
        expect(organizationActionSource).not.toContain(retiredOrganizationActionPattern);
      }
    }
    expect(reportingPanelSource).toContain('@/components/shared/Button');
    expect(reportingPanelSource).toContain('<Button');
    expect(reportingPanelSource).toContain('variant="success"');
    expect(reportingPanelSource).not.toContain(
      'rounded-md border border-base-300 bg-base-100 px-4 py-2 text-sm font-medium text-base-content',
    );
    expect(reportingPanelSource).not.toContain(
      'flex w-full items-center justify-center gap-2 rounded-md px-6 py-3 font-semibold transition-all sm:w-auto',
    );
    expect(reportingPanelSource).not.toContain('bg-blue-600 text-white hover:bg-blue-700');
    expect(reportingPanelSource).not.toContain('bg-emerald-600 text-white hover:bg-emerald-700');
    expect(proLicensePanelSource).toContain('@/components/shared/Button');
    expect(proLicensePanelSource).toContain('<Button');
    expect(proLicensePanelSource).toContain('variant="outline"');
    expect(proLicensePanelSource).toContain('size="settingsAction"');
    expect(proLicensePanelSource).not.toContain(
      'inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60',
    );
    expect(proLicensePlanSectionSource).toContain('@/components/shared/Button');
    expect(proLicensePlanSectionSource).toContain('<Button');
    expect(proLicensePlanSectionSource).toContain('variant="warning"');
    expect(proLicensePlanSectionSource).toContain('size="settingsActionXs"');
    expect(proLicensePlanSectionSource).not.toContain(
      'mt-2 inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-xs font-medium rounded-md border border-amber-300 dark:border-amber-700 text-amber-800 dark:text-amber-200 hover:bg-amber-100 dark:hover:bg-amber-800 transition-colors disabled:opacity-60',
    );
    expect(selfHostedCommercialRecoverySectionSource).toContain('@/components/shared/Button');
    expect(selfHostedCommercialRecoverySectionSource).toContain('variant="primary"');
    expect(selfHostedCommercialRecoverySectionSource).toContain('variant="outline"');
    expect(selfHostedCommercialRecoverySectionSource).toContain('size="settingsAction"');
    expect(selfHostedCommercialRecoverySectionSource).not.toContain(
      'min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed',
    );
    expect(ssoProvidersPanelSource).toContain('@/components/shared/Button');
    expect(ssoProvidersPanelSource).toContain('<Button');
    expect(ssoProvidersPanelSource).toContain('ActionIconButton');
    expect(ssoProvidersPanelSource).toContain('CopyValueButton');
    expect(ssoProvidersPanelSource).toContain('variant="primary"');
    expect(ssoProvidersPanelSource).toContain('size="settingsAction"');
    for (const retiredPattern of [
      'min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors flex items-center gap-1.5',
      'min-h-10 sm:min-h-9 px-3 py-2.5 text-sm font-medium border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors flex items-center gap-1.5',
      'p-2 text-slate-500 hover:text-blue-600 hover:bg-surface-hover rounded-md transition-colors',
      'p-2 text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900 rounded-md transition-colors',
      'text-blue-600 hover:underline flex items-center gap-1',
      'px-3 py-2 text-sm font-medium bg-surface-hover text-base-content rounded-md hover:bg-surface-hover disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap',
      'px-4 py-2 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50',
      'px-4 py-2 text-sm font-medium bg-red-600 text-white rounded-md hover:bg-red-700',
    ]) {
      expect(ssoProvidersPanelSource).not.toContain(retiredPattern);
    }
    expect(selfHostedCommercialRecoverySectionSource).not.toContain(
      'min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60 disabled:cursor-not-allowed',
    );
    expect(securityAuthPanelSource).toContain('@/components/shared/Button');
    expect(securityAuthPanelSource).toContain('<Button');
    expect(securityAuthPanelSource).toContain('variant="warning"');
    expect(securityAuthPanelSource).toContain('variant="primary"');
    expect(securityAuthPanelSource).toContain('size="settingsAction"');
    expect(securityAuthPanelSource).toContain('size="settingsActionXs"');
    expect(securityAuthPanelSource).not.toContain(
      'w-full sm:w-auto px-3 py-2 text-xs font-medium rounded-md border border-amber-300 text-amber-800 bg-amber-100 hover:bg-amber-200 transition-colors dark:border-amber-700 dark:text-amber-200 dark:bg-amber-900 dark:hover:bg-amber-800',
    );
    expect(securityAuthPanelSource).not.toContain(
      'w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors',
    );
    expect(securityAuthPanelSource).not.toContain(
      'w-full sm:w-auto min-h-10 sm:min-h-10 px-4 py-2.5 text-sm font-medium border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors',
    );
    expect(userAssignmentsPanelSource).toContain('@/components/shared/Button');
    expect(userAssignmentsPanelSource).toContain('<Button');
    expect(userAssignmentsPanelSource).toContain('variant="ghost"');
    expect(userAssignmentsPanelSource).toContain('size="settingsAction"');
    expect(userAssignmentsPanelSource).not.toContain(
      'inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium text-base-content hover:bg-surface-hover transition-colors',
    );
    expect(discoveryTabSource).toContain('@/components/shared/Button');
    expect(discoveryTabSource).toContain('@/components/shared/CopyableCodeRow');
    expect(discoveryTabSource).toContain('CopyValueButton');
    expect(discoveryTabSource).toContain('CopyableCodeRow');
    expect(discoveryTabSource).not.toContain('interface CopyValueButtonProps');
    expect(discoveryTabSource).not.toContain('const CopyValueButton');
    expect(discoveryTabSource).not.toContain('const CopyableCodeRow');
    expect(discoveryTabSource).not.toContain(
      'inline-flex min-h-7 min-w-7 shrink-0 items-center justify-center rounded border border-border bg-surface px-2 text-muted transition-colors hover:bg-surface-hover hover:text-base-content',
    );
    expect(discoveryTabSource).not.toContain(
      'flex items-start gap-2 rounded bg-surface-alt px-2 py-1.5',
    );
    expect(webInterfaceUrlFieldSource).toContain('CopyValueButton');
    expect(webInterfaceUrlFieldSource).not.toContain('CopyIcon class="h-3.5 w-3.5"');
    expect(webInterfaceUrlFieldSource).not.toContain(
      'inline-flex min-h-8 min-w-8 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content',
    );
    expect(patrolIntelligenceWorkspaceSource).toContain('@/components/shared/Button');
    expect(patrolIntelligenceWorkspaceSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content',
    );
    expect(standalonePageSurfaceSource).toContain('ButtonLink');
    expect(standalonePageSurfaceSource).not.toContain(
      'rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content',
    );
  });

  it('keeps discovery readiness badges presentation-only and accessible', () => {
    expect(discoveryReadinessBadgeSource).toContain(
      "import type { DiscoveryReadinessPresentation } from '@/utils/resourceDiscoveryReadiness'",
    );
    expect(discoveryReadinessBadgeSource).toContain('aria-label={presentation()?.title');
    expect(discoveryReadinessBadgeSource).toContain('title={presentation()?.title}');
    expect(discoveryReadinessBadgeSource).toContain('aria-hidden="true"');
    expect(discoveryReadinessBadgeSource).not.toContain('fetch(');
    expect(discoveryReadinessBadgeSource).not.toContain('localStorage');
    expect(discoveryReadinessBadgeSource).not.toContain('innerHTML');
  });

  it('routes settings info callouts through CalloutCard', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        scopes?: string[];
        allPatterns?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'settings-callout-card-shell',
    );
    const registeredWarningGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-connection-editor-local-warning-callout-shell',
    );
    const registeredDangerGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-connection-editor-local-danger-callout-shell',
    );
    const registeredRedGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-connection-editor-local-red-callout-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/CalloutCard.tsx');
    expect(registeredRule?.canonical?.export).toBe('CalloutCard');
    for (const guard of [registeredWarningGuard, registeredDangerGuard, registeredRedGuard]) {
      expect(guard?.canonical?.path).toBe('src/components/shared/CalloutCard.tsx');
      expect(guard?.canonical?.export).toBe('CalloutCard');
      expect(guard?.scopes).toEqual(['src/components/Settings/ConnectionEditor']);
      expect(guard?.allPatterns).toContain('rounded-md border');
    }
    expect(registeredWarningGuard?.allPatterns).toContain('bg-amber-50');
    expect(registeredDangerGuard?.allPatterns).toContain('bg-rose-50');
    expect(registeredRedGuard?.allPatterns).toContain('bg-red-50');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Settings/AIProviderConfigurationSection.tsx',
      'src/components/Settings/ConnectionEditor/AddressProbeStep.tsx',
      'src/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx',
      'src/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx',
      'src/components/Settings/ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx',
      'src/components/Settings/DiagnosticsResultsPanel.tsx',
      'src/components/Settings/DiscoverySettingsForm.tsx',
      'src/components/Settings/MonitoredSystemImpactPreview.tsx',
      'src/components/Settings/ReportingPanel.tsx',
      'src/components/Settings/SecurityAuthPanel.tsx',
      'src/components/Settings/SecurityOverviewPanel.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/components/Settings/AIProviderConfigurationSection.tsx',
          patterns: expect.arrayContaining(['rounded border border-red-200']),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ConnectionEditor/AddressProbeStep.tsx',
          patterns: expect.arrayContaining([
            'rounded-md border border-red-300 bg-red-50',
            'rounded-md border border-amber-300 bg-amber-50',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx',
          patterns: expect.arrayContaining([
            'border-green-300 bg-green-50',
            'rounded-md border border-rose-300 bg-rose-50',
            'testToneClass',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot.tsx',
          patterns: expect.arrayContaining([
            'rounded-md border border-amber-300 bg-amber-50',
            'rounded-md border border-rose-300 bg-rose-50',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/ConnectionEditor/CredentialSlots/VMwareCredentialSlot.tsx',
          patterns: expect.arrayContaining([
            'rounded-md border border-amber-300 bg-amber-50',
            'rounded-md border border-rose-300 bg-rose-50',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/DiagnosticsResultsPanel.tsx',
          patterns: expect.arrayContaining(['rounded-md border border-amber-200 bg-amber-50']),
        }),
        expect.objectContaining({
          path: 'src/components/Settings/DiscoverySettingsForm.tsx',
          patterns: expect.arrayContaining(['rounded-md border border-amber-200 bg-amber-50/80']),
        }),
      ]),
    );
    expect(calloutCardSource).toContain(
      "type CalloutTone = 'danger' | 'info' | 'success' | 'warning'",
    );
    expect(calloutCardSource).toContain("type CalloutScale = 'default' | 'compact'");
    expect(reportingPanelSource).toContain('CalloutCard');
    expect(reportingPanelSource).not.toContain('rounded-md border border-blue-200 bg-blue-50 p-6');
    for (const source of [
      aiProviderConfigurationSectionSource,
      diagnosticsResultsPanelSource,
      discoverySettingsFormSource,
      monitoredSystemImpactPreviewSource,
      reportingPanelSource,
      securityAuthPanelSource,
      securityOverviewPanelSource,
      addressProbeStepSource,
      availabilityTargetSlotSource,
      trueNASCredentialSlotSource,
      vmwareCredentialSlotSource,
    ]) {
      expect(source).toContain('CalloutCard');
    }
    for (const source of [
      addressProbeStepSource,
      availabilityTargetSlotSource,
      trueNASCredentialSlotSource,
      vmwareCredentialSlotSource,
    ]) {
      expect(source).toContain('scale="compact"');
      expect(source).toContain('padding="sm"');
    }
    expect(aiProviderConfigurationSectionSource).not.toContain('rounded border border-red-200');
    expect(addressProbeStepSource).not.toContain('rounded-md border border-red-300 bg-red-50');
    expect(addressProbeStepSource).not.toContain('rounded-md border border-amber-300 bg-amber-50');
    expect(availabilityTargetSlotSource).not.toContain('testToneClass');
    expect(availabilityTargetSlotSource).not.toContain('border-green-300 bg-green-50');
    expect(availabilityTargetSlotSource).not.toContain(
      'rounded-md border border-rose-300 bg-rose-50',
    );
    expect(trueNASCredentialSlotSource).not.toContain(
      'rounded-md border border-amber-300 bg-amber-50',
    );
    expect(trueNASCredentialSlotSource).not.toContain(
      'rounded-md border border-rose-300 bg-rose-50',
    );
    expect(vmwareCredentialSlotSource).not.toContain(
      'rounded-md border border-amber-300 bg-amber-50',
    );
    expect(vmwareCredentialSlotSource).not.toContain(
      'rounded-md border border-rose-300 bg-rose-50',
    );
    expect(diagnosticsResultsPanelSource).not.toContain(
      'rounded-md border border-amber-200 bg-amber-50',
    );
    expect(discoverySettingsFormSource).not.toContain(
      'rounded-md border border-amber-200 bg-amber-50/80',
    );
  });

  it('routes shared error-boundary fallbacks through shared callout and button primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'error-boundary-callout-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/CalloutCard.tsx');
    expect(registeredRule?.canonical?.export).toBe('CalloutCard');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/ErrorBoundary.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/ErrorBoundary.tsx',
        patterns: [
          '<svg',
          'bg-red-50 dark:bg-red-900 border border-red-200',
          'p-4 bg-red-50 dark:bg-red-900 border border-red-200',
        ],
      },
    ]);
    expect(errorBoundarySource).toContain('CalloutCard');
    expect(errorBoundarySource).toContain("import { Button } from '@/components/shared/Button';");
    expect(errorBoundarySource).toContain('lucide-solid/icons/alert-triangle');
    expect(errorBoundarySource).not.toContain('<svg');
    expect(errorBoundarySource).not.toContain('bg-red-50 dark:bg-red-900 border border-red-200');
    expect(errorBoundarySource).not.toContain(
      'p-4 bg-red-50 dark:bg-red-900 border border-red-200',
    );
  });

  it('routes update modal callouts, actions, and status indicators through shared primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'update-modal-action-callout-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/CalloutCard.tsx');
    expect(registeredRule?.canonical?.export).toBe('CalloutCard');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/UpdateConfirmationModal.tsx',
      'src/components/UpdateProgressModal.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/UpdateConfirmationModal.tsx',
        patterns: [
          '<svg',
          'bg-blue-50 dark:bg-blue-900 border border-blue-200',
          'bg-yellow-50 dark:bg-yellow-900 border border-yellow-200',
          'rounded-md p-4 border',
          'px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors',
        ],
      },
      {
        path: 'src/components/UpdateProgressModal.tsx',
        patterns: [
          '<svg',
          'bg-red-50 dark:bg-red-900 border border-red-200',
          'bg-blue-50 dark:bg-blue-900 border border-blue-200',
          'bg-yellow-50 dark:bg-yellow-900 border border-yellow-200',
          'mt-2 px-3 py-1.5 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 rounded',
        ],
      },
    ]);

    expect(updateConfirmationModalSource).toContain('CalloutCard');
    expect(updateConfirmationModalSource).toContain('ActionIconButton');
    expect(updateConfirmationModalSource).toContain('lucide-solid/icons/arrow-right');
    expect(updateProgressModalSource).toContain('CalloutCard');
    expect(updateProgressModalSource).toContain('ActionIconButton');
    expect(updateProgressModalSource).toContain('LoadingSpinner');

    for (const source of [updateConfirmationModalSource, updateProgressModalSource]) {
      expect(source).not.toContain('<svg');
      expect(source).not.toContain('bg-blue-50 dark:bg-blue-900 border border-blue-200');
      expect(source).not.toContain('bg-yellow-50 dark:bg-yellow-900 border border-yellow-200');
      expect(source).not.toContain(
        'px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors',
      );
    }
    expect(updateConfirmationModalSource).not.toContain('rounded-md p-4 border');
    expect(updateProgressModalSource).not.toContain(
      'bg-red-50 dark:bg-red-900 border border-red-200',
    );
    expect(updateProgressModalSource).not.toContain(
      'mt-2 px-3 py-1.5 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 rounded',
    );
  });

  it('routes platform inline notices through InlineNotice', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        scopes?: string[];
        allPatterns?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-inline-notice-shell',
    );
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-inline-notice-local-amber-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/InlineNotice.tsx');
    expect(registeredRule?.canonical?.export).toBe('InlineNotice');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/features/platformPage/PlatformOutdatedAgentNotice.tsx',
      'src/features/platformPage/PlatformOutdatedSensorSetupNotice.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/features/platformPage/PlatformOutdatedAgentNotice.tsx',
          patterns: expect.arrayContaining(['rounded-lg border border-amber-300 bg-amber-50']),
        }),
        expect.objectContaining({
          path: 'src/features/platformPage/PlatformOutdatedSensorSetupNotice.tsx',
          patterns: expect.arrayContaining(['rounded-lg border border-amber-300 bg-amber-50']),
        }),
      ]),
    );
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/InlineNotice.tsx');
    expect(registeredGuard?.canonical?.export).toBe('InlineNotice');
    expect(registeredGuard?.scopes).toEqual(['src/features/platformPage']);
    expect(registeredGuard?.allPatterns).toEqual(
      expect.arrayContaining(['rounded-lg border', 'border-amber-300 bg-amber-50']),
    );
    expect(inlineNoticeSource).toContain('INLINE_NOTICE_TONE_CLASSES');
    expect(inlineNoticeSource).toContain('INLINE_NOTICE_ACTION_TONE_CLASSES');
    for (const source of [
      platformOutdatedAgentNoticeSource,
      platformOutdatedSensorSetupNoticeSource,
    ]) {
      expect(source).toContain('InlineNotice');
      expect(source).not.toContain('rounded-lg border border-amber-300 bg-amber-50');
      expect(source).not.toContain('text-amber-900 underline-offset-2');
    }
    expect(platformOutdatedAgentNoticeSource).toContain(
      "import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';",
    );
    expect(platformOutdatedAgentNoticeSource).toContain(
      'const visible = createMemo(() => count() > 0 && !presentationPolicyIsReadOnly());',
    );
  });

  it('routes dismissible demo notices through InlineNotice banner primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'demo-banner-notice-shell');

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/InlineNotice.tsx');
    expect(registeredRule?.canonical?.export).toBe('InlineNotice');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/DemoBanner.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/DemoBanner.tsx',
        patterns: [
          '<svg',
          '<button',
          'bg-blue-50 dark:bg-blue-900 border-b border-blue-200 dark:border-blue-800',
          'p-1 hover:bg-blue-100 dark:hover:bg-blue-800 rounded text-blue-600',
        ],
      },
    ]);
    expect(inlineNoticeSource).toContain('INLINE_NOTICE_LAYOUT_CLASSES');
    expect(inlineNoticeSource).toContain('ActionIconButton');
    expect(demoBannerSource).toContain('InlineNotice');
    expect(demoBannerSource).toContain('layout="banner"');
    expect(demoBannerSource).toContain('lucide-solid/icons/info');
    expect(demoBannerSource).not.toContain('<svg');
    expect(demoBannerSource).not.toContain('<button');
    expect(demoBannerSource).not.toContain(
      'bg-blue-50 dark:bg-blue-900 border-b border-blue-200 dark:border-blue-800',
    );
    expect(demoBannerSource).not.toContain(
      'p-1 hover:bg-blue-100 dark:hover:bg-blue-800 rounded text-blue-600',
    );
  });

  it('routes commercial migration notices through InlineNotice banner primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'commercial-migration-banner-notice-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/InlineNotice.tsx');
    expect(registeredRule?.canonical?.export).toBe('InlineNotice');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/CommercialMigrationBanner.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/CommercialMigrationBanner.tsx',
        patterns: [
          '<svg',
          '<button',
          'toneClasses',
          'buttonClasses',
          'bg-amber-50 dark:bg-amber-900 border-b border-amber-200',
          'bg-red-50 dark:bg-red-900 border-b border-red-200',
          'p-1 rounded transition-colors opacity-70 hover:opacity-100',
        ],
      },
    ]);
    expect(inlineNoticeSource).toContain('actionOnClick');
    expect(commercialMigrationBannerSource).toContain('InlineNotice');
    expect(commercialMigrationBannerSource).toContain('layout="banner"');
    expect(commercialMigrationBannerSource).toContain('lucide-solid/icons/alert-triangle');
    expect(commercialMigrationBannerSource).toContain('actionOnClick');
    expect(commercialMigrationBannerSource).not.toContain('<svg');
    expect(commercialMigrationBannerSource).not.toContain('<button');
    expect(commercialMigrationBannerSource).not.toContain('toneClasses');
    expect(commercialMigrationBannerSource).not.toContain('buttonClasses');
    expect(commercialMigrationBannerSource).not.toContain(
      'bg-amber-50 dark:bg-amber-900 border-b border-amber-200',
    );
    expect(commercialMigrationBannerSource).not.toContain(
      'bg-red-50 dark:bg-red-900 border-b border-red-200',
    );
    expect(commercialMigrationBannerSource).not.toContain(
      'p-1 rounded transition-colors opacity-70 hover:opacity-100',
    );
  });

  it('routes GitHub star prompt actions through Button primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'github-star-banner-action-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/Button.tsx');
    expect(registeredRule?.canonical?.export).toBe('Button');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/GitHubStarBanner.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/GitHubStarBanner.tsx',
        patterns: [
          'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content',
          'inline-flex min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700',
          'inline-flex min-h-9 items-center justify-center rounded-md px-3 py-2 text-sm text-muted transition-colors hover:bg-surface-hover hover:text-base-content',
        ],
      },
    ]);
    expect(gitHubStarBannerSource).toContain('@/components/shared/Button');
    expect(gitHubStarBannerSource).toContain('<ActionIconButton');
    expect(gitHubStarBannerSource).toContain('<Button');
    expect(gitHubStarBannerSource).not.toContain(
      'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content',
    );
    expect(gitHubStarBannerSource).not.toContain(
      'inline-flex min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700',
    );
    expect(gitHubStarBannerSource).not.toContain(
      'inline-flex min-h-9 items-center justify-center rounded-md px-3 py-2 text-sm text-muted transition-colors hover:bg-surface-hover hover:text-base-content',
    );
  });

  it('keeps TLS verification warnings in the shared primitive boundary', () => {
    expect(tlsVerificationWarningBannerSource).toContain('role="alert"');
    expect(tlsVerificationWarningBannerSource).toContain('TLS verification disabled.');
    expect(tlsVerificationWarningBannerSource).toContain('controlled lab environments');
    expect(tlsVerificationWarningBannerSource).toContain('Install a trusted certificate');
    expect(tlsVerificationWarningBannerSource).not.toContain('CalloutCard');
  });

  it('keeps shared tag badges in the shared primitive boundary', () => {
    expect(tagBadgesSource).toContain("from '@/components/shared/Tooltip'");
    expect(guestRowSource).toContain("from '@/components/shared/TagBadges'");
    expect(guestRowSource).not.toContain("from './TagBadges'");
    expect(resourceDetailSummarySource).toContain("from '@/components/shared/TagBadges'");
    expect(resourceDetailSummarySource).not.toContain("from '@/components/Workloads/TagBadges'");
  });

  it('keeps interactive sparkline on shell, runtime, and model owners', () => {});

  it('keeps dialog on shell, runtime, and model owners', () => {
    expect(dialogSource).toContain('useDialogState');
    expect(dialogSource).toContain('getDialogViewportClass');
    expect(dialogSource).toContain('getDialogAlignmentClass');
    expect(dialogSource).toContain('getDialogPanelClass');
    expect(dialogSource).not.toContain('createEffect');
    expect(dialogSource).not.toContain('onCleanup');
    expect(dialogSource).not.toContain('FOCUSABLE_SELECTOR');
    expect(dialogSource).not.toContain('document.body.style.overflow');
    expect(dialogSource).not.toContain('querySelectorAll<HTMLElement>');

    expect(dialogStateSource).toContain('export function useDialogState');
    expect(dialogStateSource).toContain('createEffect');
    expect(dialogStateSource).toContain('onCleanup');
    expect(dialogStateSource).toContain('document.body.style.overflow');
    expect(dialogStateSource).toContain('openDialogCount');
    expect(dialogStateSource).toContain('getDialogFocusableElements');

    expect(dialogModelSource).toContain('export function getDialogLayout');
    expect(dialogModelSource).toContain('export function getDialogFocusableElements');
    expect(dialogModelSource).toContain('export function getDialogViewportClass');
    expect(dialogModelSource).toContain('export function getDialogAlignmentClass');
    expect(dialogModelSource).toContain('export function getDialogPanelClass');
    expect(dialogModelSource).toContain('FOCUSABLE_SELECTOR');
  });

  it('keeps history chart on shell, runtime, and model owners', () => {
    expect(historyChartSource).toContain('useHistoryChartState');
    expect(historyChartSource).toContain('HistoryChartHeader');
    expect(historyChartSource).toContain('HistoryChartOverlay');
    expect(historyChartSource).toContain('HistoryChartTooltip');
    expect(historyChartSource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartSource).not.toContain('calculateOptimalPoints');
    expect(historyChartSource).not.toContain('setupCanvasDPR');
    expect(historyChartSource).not.toContain('createSignal');
    expect(historyChartSource).not.toContain('Collecting data... History will appear here.');
    expect(historyChartSource).not.toContain('Unlock {chart.lockTierLabel()} Features');

    expect(historyChartStateSource).toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartStateSource).toContain('calculateOptimalPoints');
    expect(historyChartStateSource).toContain('setupCanvasDPR');
    expect(historyChartStateSource).toContain('export function useHistoryChartState');
    expect(historyChartStateSource).toContain('HISTORY_CHART_RANGES');

    expect(historyChartModelSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartModelSource).toContain('getHistoryChartTooltipLayout');
    expect(historyChartModelSource).toContain('HISTORY_CHART_RANGES');
    expect(historyChartModelSource).toContain('getHistoryChartScale');
    expect(historyChartModelSource).toContain('findHistoryChartClosestPoint');

    expect(historyChartHeaderSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartHeaderSource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartHeaderSource).not.toContain('setupCanvasDPR');

    expect(historyChartOverlaySource).toContain('Collecting data... History will appear here.');
    expect(historyChartOverlaySource).toContain(
      'Historical data beyond {props.chart.lockDays()} days is not enabled on this instance.',
    );
    expect(historyChartOverlaySource).not.toContain(
      'Unlock {props.chart.lockTierLabel()} Features',
    );
    expect(historyChartOverlaySource).not.toContain('free 14-day trial');
    expect(historyChartOverlaySource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartOverlaySource).not.toContain('setupCanvasDPR');

    expect(historyChartTooltipSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartTooltipSource).toContain('getHistoryChartTooltipLayout');
    expect(historyChartTooltipSource).toContain('foreignObject');
    expect(historyChartTooltipSource).toContain('width={props.chartWidth}');
    expect(historyChartTooltipSource).toContain('height={props.chartHeight}');
    expect(historyChartTooltipSource).toContain('new Date(point().timestamp).toLocaleString()');
    expect(historyChartTooltipSource).not.toContain('<Portal>');
    expect(historyChartTooltipSource).not.toContain('absolute inset-0 h-full w-full');
    expect(historyChartTooltipSource).not.toContain('preserveAspectRatio="none"');
    expect(historyChartTooltipSource).not.toContain('style={');
    expect(historyChartTooltipSource).not.toContain('ChartsAPI.getMetricsHistory');
  });

  it('keeps tag badges CSP-safe', () => {
    expect(tagBadgesSource).toContain('data-tag-dot="true"');
    expect(tagBadgesSource).toContain('data-tag-dot-fill="true"');
    expect(tagBadgesSource).toContain('data-tag-dot-ring="true"');
    expect(tagBadgesSource).not.toContain('style={');
    expect(tagBadgesSource).not.toContain('box-shadow');
    expect(tagBadgesSource).not.toContain('background-color');
  });

  it('keeps container update badge on shell, runtime, and model owners', () => {
    expect(containerUpdateBadgeSource).toContain('useContainerUpdateButtonState');
    expect(containerUpdateBadgeSource).toContain('getUpdateButtonClass');
    expect(containerUpdateBadgeSource).not.toContain('MonitoringAPI.updateDockerContainer');
    expect(containerUpdateBadgeSource).not.toContain('markContainerQueued');
    expect(containerUpdateBadgeSource).not.toContain('createSignal');

    expect(containerUpdateButtonStateSource).toContain('MonitoringAPI.updateDockerContainer');
    expect(containerUpdateButtonStateSource).toContain('markContainerQueued');
    expect(containerUpdateButtonStateSource).toContain('createSignal');
    expect(containerUpdateButtonStateSource).toContain(
      'export function useContainerUpdateButtonState',
    );

    expect(containerUpdateBadgeModelSource).toContain('getUpdateButtonClass');
    expect(containerUpdateBadgeModelSource).toContain('getUpdateButtonTooltip');
    expect(containerUpdateBadgeModelSource).toContain('hasContainerUpdate');
  });

  it('keeps web interface URL field on shell, runtime, and model owners', () => {
    expect(webInterfaceUrlFieldSource).toContain('useWebInterfaceUrlFieldState');
    expect(webInterfaceUrlFieldSource).not.toContain('GuestMetadataAPI.getMetadata');
    expect(webInterfaceUrlFieldSource).not.toContain('AgentMetadataAPI.updateMetadata');
    expect(webInterfaceUrlFieldSource).not.toContain('validateWebInterfaceCustomUrl');
    expect(webInterfaceUrlFieldSource).not.toContain('createSignal');

    expect(webInterfaceUrlFieldStateSource).toContain('GuestMetadataAPI.getMetadata');
    expect(webInterfaceUrlFieldStateSource).toContain('AgentMetadataAPI.updateMetadata');
    expect(webInterfaceUrlFieldStateSource).toContain('createSignal');
    expect(webInterfaceUrlFieldStateSource).toContain(
      'export function useWebInterfaceUrlFieldState',
    );

    expect(webInterfaceUrlFieldModelSource).toContain('validateWebInterfaceCustomUrl');
    expect(webInterfaceUrlFieldModelSource).toContain('getWebInterfaceSuggestedUrlFallback');
    expect(webInterfaceUrlFieldModelSource).toContain('shouldShowWebInterfaceSuggestedUrl');
  });

  it('keeps runtime web-interface launch on the shared resource-name template', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
    };
    const registeredRule = registry.rules.find(
      (rule) => rule.id === 'runtime-web-interface-name-link',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/WebInterfaceNameLink.tsx');
    expect(registeredRule?.canonical?.export).toBe('WebInterfaceNameLink');
    expect(webInterfaceNameLinkSource).toContain('export const WebInterfaceNameLink');
    expect(webInterfaceNameLinkSource).toContain('target="_blank"');
    expect(webInterfaceNameLinkSource).toContain('rel="noopener noreferrer"');
    expect(webInterfaceNameLinkSource).toContain('event.stopPropagation()');
    expect(webInterfaceNameLinkSource).toContain('Open web interface for');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      expect.arrayContaining([
        'src/components/shared/NodeGroupHeader.tsx',
        'src/components/Workloads/GuestRow.tsx',
        'src/features/standalone/AgentsMachinesTable.tsx',
        'src/features/proxmox/ProxmoxNodesTable.tsx',
        'src/components/Alerts/AlertResourceGroupHeader.tsx',
        'src/components/Alerts/AlertResourceTableRow.tsx',
      ]),
    );

    expect(nodeGroupHeaderSource).toContain('WebInterfaceNameLink');
    expect(guestRowSource).toContain('WebInterfaceNameLink');
    expect(agentsMachinesTableSource).toContain('WebInterfaceNameLink');
    expect(proxmoxNodesTableSource).toContain('WebInterfaceNameLink');
    expect(alertResourceGroupHeaderSource).toContain('WebInterfaceNameLink');
    expect(alertResourceTableRowSource).toContain('WebInterfaceNameLink');
    expect(nodeGroupHeaderSource).not.toContain('target="_blank"');
    expect(guestRowSource).not.toContain('target="_blank"');
    expect(agentsMachinesTableSource).not.toContain('target="_blank"');
    expect(proxmoxNodesTableSource).not.toContain('target="_blank"');
    expect(alertResourceGroupHeaderSource).not.toContain('target="_blank"');
    expect(alertResourceTableRowSource).not.toContain('target="_blank"');
    expect(nodeGroupHeaderSource).not.toContain('rel="noopener noreferrer"');
    expect(alertResourceGroupHeaderSource).not.toContain('rel="noopener noreferrer"');
    expect(alertResourceTableRowSource).not.toContain('rel="noopener noreferrer"');
    expect(agentsMachinesTableSource).not.toContain('AgentMachineWebLinkCell');
    expect(agentsMachinesTableSource).not.toContain('data-agent-machine-web-link');
    expect(proxmoxNodesTableSource).not.toContain('data-proxmox-host-web-link');
    expect(agentMachineTableModelSource).not.toContain("id: 'web'");
    expect(agentMachineTableModelSource).not.toContain("label: 'Web'");
    expect(proxmoxHostTableModelSource).not.toContain("id: 'web'");
    expect(proxmoxHostTableModelSource).not.toContain("label: 'Web'");
  });

  it('keeps platform table frames on the shared shell template', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      patternGuards: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allowedPaths?: string[];
      }>;
    };
    const registeredGuard = registry.patternGuards.find(
      (guard) => guard.id === 'platform-table-shell-local-frame',
    );

    expect(registeredGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredGuard?.canonical?.export).toBe('PlatformTableShell');
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(sharedPlatformPageSource).toContain('export function PlatformTableShell');
    expect(sharedPlatformPageSource).toContain('TableCard class={props.cardClass');
    expect(sharedPlatformPageSource).toContain('TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}');
    expect(sharedPlatformPageSource).toContain('TableBody class={PLATFORM_TABLE_BODY_CLASS}');
    expect(sharedPlatformPageSource).toContain(
      "export const PLATFORM_TABLE_DEFAULT_RESPONSIVE_MIN_WIDTH_CLASS = 'min-w-[48rem]'",
    );
    expect(sharedPlatformPageSource).toContain(
      'getPlatformTableResponsiveMinWidthClass(props.tableClass)',
    );

    for (const [path, source] of [
      ['src/features/docker/DockerHostsTable.tsx', dockerHostsTableSource],
      ['src/features/kubernetes/KubernetesNodesTable.tsx', kubernetesNodesTableSource],
      ['src/features/proxmox/ProxmoxNodesTable.tsx', proxmoxNodesTableSource],
      ['src/features/vmware/VsphereHostsTable.tsx', vsphereHostsTableSource],
    ] as const) {
      expect(registeredGuard?.allowedPaths ?? []).not.toContain(path);
      expect(source).toContain('PlatformTableShell');
      expect(source).not.toContain('TableCard class={PLATFORM_TABLE_CARD_CLASS}');
      expect(source).not.toContain('TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}');
      expect(source).not.toContain('TableBody class={PLATFORM_TABLE_BODY_CLASS}');
    }
  });

  it('keeps platform count grammar and route status normalization shared', () => {
    expect(sharedPlatformPageSource).toContain('export const getPlatformResourceCountNoun');
    expect(sharedPlatformPageSource).toContain(
      'export const normalizePlatformResourceStatusFilter',
    );
  });

  it('keeps platform table empty states on the shared shell template', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      patternGuards: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
      }>;
    };
    const registeredGuard = registry.patternGuards.find(
      (guard) => guard.id === 'platform-table-empty-state-local-empty-state',
    );

    expect(registeredGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredGuard?.canonical?.export).toBe('PlatformTableEmptyState');
    expect(registeredGuard?.allPatterns).toEqual(['@/components/shared/EmptyState']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining([
        'src/features/docker',
        'src/features/kubernetes',
        'src/features/proxmox',
        'src/features/standalone',
        'src/features/truenas',
        'src/features/vmware',
      ]),
    );
    expect(sharedPlatformPageSource).toContain('export function PlatformTableEmptyState');
    expect(sharedPlatformPageSource).toContain('icon?: JSX.Element');
    expect(sharedPlatformPageSource).toContain("from '@/components/shared/EmptyState'");

    for (const source of [
      dockerHostsTableSource,
      kubernetesNodesTableSource,
      proxmoxCephTableSource,
      proxmoxCoverageTableSource,
      proxmoxMailGatewayTableSource,
      proxmoxRecoverableTableSource,
      proxmoxReplicationTableSource,
      proxmoxNodesTableSource,
      vsphereHostsTableSource,
    ]) {
      expect(source).toContain('PlatformTableEmptyState');
      expect(source).not.toContain('@/components/shared/EmptyState');
      expect(source).not.toContain('<EmptyState');
      expect(source).not.toContain('<Card padding="lg">');
    }
  });

  it('keeps embedded panel empty states on the shared EmptyState primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        extensions?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'panel-empty-state-shell');
    const centeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-panel-local-centered-empty-state',
    );
    const actionGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-panel-local-action-empty-state',
    );
    const dashedGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'settings-panel-local-dashed-empty-state',
    );
    const consumers: Array<[string, string]> = [
      ['src/components/Settings/AgentProfilesPanel.tsx', agentProfilesPanelSource],
      ['src/components/Settings/AuditWebhookPanel.tsx', auditWebhookPanelSource],
      ['src/components/Settings/AuditLogPanel.tsx', auditLogPanelSource],
      ['src/components/Settings/AvailabilitySettingsPanel.tsx', availabilitySettingsPanelSource],
      ['src/components/Settings/DiagnosticsResultsPanel.tsx', diagnosticsResultsPanelSource],
      ['src/components/patrol/RunHistoryPanel.tsx', runHistoryPanelSource],
      ['src/components/Settings/SSOProvidersPanel.tsx', ssoProvidersPanelSource],
    ];
    const consumerPaths = consumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/EmptyState.tsx');
    expect(registeredRule?.canonical?.export).toBe('EmptyState');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      consumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/Settings/AgentProfilesPanel.tsx',
        patterns: ['text-center py-8 text-muted'],
      },
      {
        path: 'src/components/Settings/AuditWebhookPanel.tsx',
        patterns: [
          'py-10 flex flex-col items-center justify-center text-muted border-2 border-dashed border-border rounded-md',
        ],
      },
      {
        path: 'src/components/Settings/AuditLogPanel.tsx',
        patterns: [
          'text-center py-12 px-4 bg-surface-alt rounded-md border border-dashed border-border',
        ],
      },
      {
        path: 'src/components/Settings/AvailabilitySettingsPanel.tsx',
        patterns: ['flex flex-col items-center justify-center gap-3 px-4 py-12 text-center'],
      },
      {
        path: 'src/components/Settings/DiagnosticsResultsPanel.tsx',
        patterns: [
          'Activity class="mx-auto mb-4 h-12 w-12 text-muted"',
          'inline-flex min-h-10 items-center gap-2 rounded-md bg-blue-600',
        ],
      },
      {
        path: 'src/components/patrol/RunHistoryPanel.tsx',
        patterns: ['text-center py-8'],
      },
      {
        path: 'src/components/Settings/SSOProvidersPanel.tsx',
        patterns: ['text-center py-8 text-muted'],
      },
    ]);
    expect(centeredGuard?.canonical?.export).toBe('EmptyState');
    expect(centeredGuard?.scopes).toEqual([
      'src/components/Settings/AgentProfilesPanel.tsx',
      'src/components/Settings/SSOProvidersPanel.tsx',
    ]);
    expect(centeredGuard?.extensions).toEqual(['.tsx']);
    expect(centeredGuard?.allPatterns).toEqual(['text-center py-8 text-muted']);
    expect(actionGuard?.canonical?.export).toBe('EmptyState');
    expect(actionGuard?.scopes).toEqual(['src/components/Settings/AvailabilitySettingsPanel.tsx']);
    expect(actionGuard?.allPatterns).toEqual([
      'flex flex-col items-center justify-center gap-3 px-4 py-12 text-center',
    ]);
    expect(dashedGuard?.canonical?.export).toBe('EmptyState');
    expect(dashedGuard?.scopes).toEqual(['src/components/Settings/AuditLogPanel.tsx']);
    expect(dashedGuard?.allPatterns).toEqual([
      'text-center py-12 px-4 bg-surface-alt rounded-md border border-dashed border-border',
    ]);
    for (const guard of [centeredGuard, actionGuard, dashedGuard]) {
      expect(guard?.allowedPaths ?? []).toHaveLength(0);
      expect(guard?.ignoredPaths ?? []).toHaveLength(0);
    }

    expect(emptyStateSource).toContain("variant: 'framed'");
    expect(emptyStateSource).toContain("variant === 'framed'");
    expect(emptyStateSource).toContain("'px-4 py-8'");

    for (const [path, source] of consumers) {
      expect(source).toContain('@/components/shared/EmptyState');
      expect(source).toContain('<EmptyState');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
    expect(availabilitySettingsPanelSource).toContain('variant="panel"');
    expect(agentProfilesPanelSource.match(/variant="panel"/g) ?? []).toHaveLength(2);
    expect(auditWebhookPanelSource).toContain('variant="panel"');
    expect(diagnosticsResultsPanelSource).toContain('variant="panel"');
    expect(runHistoryPanelSource).toContain('variant="panel"');
    expect(ssoProvidersPanelSource).toContain('variant="panel"');
    expect(auditLogPanelSource).toContain('<EmptyState');
    expect(auditLogPanelSource).toContain('variant="panel"');
    expect(auditLogPanelSource).toContain('tone={activeFilterCount() > 0 ?');
    expect(auditLogPanelSource).not.toContain('Clear all filters</button>');
  });

  it('keeps platform table loading states on the shared status-row template', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-loading-state',
    );
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-loading-state-local-status-row',
    );
    const requiredConsumerPaths = [
      'src/features/docker/DockerPageSurface.tsx',
      'src/features/kubernetes/KubernetesPageSurface.tsx',
      'src/features/proxmox/ProxmoxBackupsTable.tsx',
      'src/features/proxmox/ProxmoxPageSurface.tsx',
      'src/features/proxmox/ProxmoxReplicationTable.tsx',
      'src/features/standalone/StandalonePageSurface.tsx',
      'src/features/truenas/TrueNASPageSurface.tsx',
      'src/features/truenas/TrueNASProtectionTable.tsx',
      'src/features/vmware/VmwarePageSurface.tsx',
    ];

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformTableLoadingState');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      expect.arrayContaining(requiredConsumerPaths),
    );
    expect(registeredGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredGuard?.canonical?.export).toBe('PlatformTableLoadingState');
    expect(registeredGuard?.allPatterns).toEqual(['role="status"', 'px-3 py-2 text-xs text-muted']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining([
        'src/features/docker',
        'src/features/kubernetes',
        'src/features/proxmox',
        'src/features/standalone',
        'src/features/truenas',
        'src/features/vmware',
      ]),
    );

    expect(sharedPlatformPageSource).toContain('export function PlatformTableLoadingState');
    expect(sharedPlatformPageSource).toContain('role="status"');
    expect(sharedPlatformPageSource).toContain('px-3 py-2 text-xs text-muted');

    for (const source of [
      dockerPageSurfaceSource,
      kubernetesPageSurfaceSource,
      proxmoxBackupsTableSource,
      proxmoxPageSurfaceSource,
      proxmoxReplicationTableSource,
      standalonePageSurfaceSource,
      truenasPageSurfaceSource,
      truenasProtectionTableSource,
      vmwarePageSurfaceSource,
    ]) {
      expect(source).toContain('PlatformTableLoadingState');
      expect(source).not.toContain('<div class="px-3 py-2 text-xs text-muted" role="status">');
    }
  });

  it('keeps Kubernetes platform table text fallbacks on the shared helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-text-value-fallback',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'kubernetes-platform-table-local-text-value-helper',
    );
    const kubernetesTextValueConsumers: Array<[string, string]> = [
      ['src/features/kubernetes/KubernetesAutoscalingTable.tsx', kubernetesAutoscalingTableSource],
      ['src/features/kubernetes/KubernetesClustersTable.tsx', kubernetesClustersTableSource],
      ['src/features/kubernetes/KubernetesConfigTable.tsx', kubernetesConfigTableSource],
      ['src/features/kubernetes/KubernetesControllersTable.tsx', kubernetesControllersTableSource],
      ['src/features/kubernetes/KubernetesDeploymentsTable.tsx', kubernetesDeploymentsTableSource],
      ['src/features/kubernetes/KubernetesEventsTable.tsx', kubernetesEventsTableSource],
      ['src/features/kubernetes/KubernetesNetworkingTable.tsx', kubernetesNetworkingTableSource],
      ['src/features/kubernetes/KubernetesNodesTable.tsx', kubernetesNodesTableSource],
      ['src/features/kubernetes/KubernetesPodsTable.tsx', kubernetesPodsTableSource],
      ['src/features/kubernetes/KubernetesPolicyTable.tsx', kubernetesPolicyTableSource],
      ['src/features/kubernetes/KubernetesServicesTable.tsx', kubernetesServicesTableSource],
      ['src/features/kubernetes/KubernetesStorageTable.tsx', kubernetesStorageTableSource],
    ];
    const kubernetesTextValueConsumerPaths = kubernetesTextValueConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('formatPlatformTableTextValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      kubernetesTextValueConsumerPaths,
    );
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('formatPlatformTableTextValue');
    expect(localHelperGuard?.allPatterns).toEqual([
      'const textValue',
      "asTrimmedString(value) || '—'",
    ]);
    expect(localHelperGuard?.scopes).toEqual(['src/features/kubernetes']);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export const formatPlatformTableTextValue');
    expect(sharedPlatformPageSource).toContain('asTrimmedString(value) || emptyText');

    for (const [path, source] of kubernetesTextValueConsumers) {
      expect(source).toContain('formatPlatformTableTextValue');
      expect(source).not.toContain('const textValue');
      expect(source).not.toContain("asTrimmedString(value) || '—'");
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
  });

  it('keeps TrueNAS platform table title-case fallbacks on the shared helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-title-case-value-fallback',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'truenas-platform-table-local-title-case-helper',
    );
    const truenasTitleCaseConsumers: Array<[string, string]> = [
      ['src/features/truenas/TrueNASAppsTable.tsx', truenasAppsTableSource],
      ['src/features/truenas/TrueNASNetworkSharesTable.tsx', truenasNetworkSharesTableSource],
      ['src/features/truenas/TrueNASServicesTable.tsx', truenasServicesTableSource],
      ['src/features/truenas/TrueNASStorageTopologyTable.tsx', truenasStorageTopologyTableSource],
      ['src/features/truenas/TrueNASVirtualMachinesTable.tsx', truenasVirtualMachinesTableSource],
    ];
    const truenasTitleCaseConsumerPaths = truenasTitleCaseConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('formatPlatformTableTitleCaseValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      truenasTitleCaseConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual(
      truenasTitleCaseConsumerPaths.map((path) => ({
        path,
        patterns: ['const titleCase'],
      })),
    );
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('formatPlatformTableTitleCaseValue');
    expect(localHelperGuard?.allPatterns).toEqual([
      'const titleCase',
      'asTrimmedString(value)',
      "return 'Unknown'",
    ]);
    expect(localHelperGuard?.scopes).toEqual(['src/features/truenas']);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export const formatPlatformTableTitleCaseValue');
    expect(sharedPlatformPageSource).toContain('if (!normalized) return emptyText');

    for (const [, source] of truenasTitleCaseConsumers) {
      expect(source).toContain('formatPlatformTableTitleCaseValue');
      expect(source).not.toContain('const titleCase');
    }
  });

  it('keeps platform table uptime fallbacks on the shared helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-uptime-value-fallback',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-uptime-helper',
    );
    const functionHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-uptime-function-helper',
    );
    const directFormatGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-format-uptime-import',
    );
    const platformUptimeConsumers: Array<[string, string]> = [
      ['src/features/docker/DockerHostsTable.tsx', dockerHostsTableSource],
      ['src/features/kubernetes/KubernetesNodesTable.tsx', kubernetesNodesTableSource],
      ['src/features/proxmox/ProxmoxBackupServersTable.tsx', proxmoxBackupServersTableSource],
      ['src/features/proxmox/ProxmoxMailGatewayDrawer.tsx', proxmoxMailGatewayDrawerSource],
      ['src/features/proxmox/ProxmoxMailGatewayTable.tsx', proxmoxMailGatewayTableSource],
      ['src/features/proxmox/ProxmoxNodesTable.tsx', proxmoxNodesTableSource],
      ['src/features/standalone/AgentsMachinesTable.tsx', agentsMachinesTableSource],
      ['src/features/truenas/TrueNASSystemsTable.tsx', truenasSystemsTableSource],
      ['src/features/vmware/VsphereHostsTable.tsx', vsphereHostsTableSource],
    ];
    const platformUptimeConsumerPaths = platformUptimeConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('formatPlatformTableUptimeValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformUptimeConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      { path: 'src/features/docker/DockerHostsTable.tsx', patterns: ['const formatUptime'] },
      {
        path: 'src/features/kubernetes/KubernetesNodesTable.tsx',
        patterns: ['const formatUptime'],
      },
      {
        path: 'src/features/proxmox/ProxmoxBackupServersTable.tsx',
        patterns: ['formatUptime(row.uptimeSeconds ?? 0)'],
      },
      {
        path: 'src/features/proxmox/ProxmoxMailGatewayDrawer.tsx',
        patterns: [
          'function formatUptime',
          'const days = Math.floor(seconds / 86_400);',
          'return `${mins}m`',
        ],
      },
      {
        path: 'src/features/proxmox/ProxmoxMailGatewayTable.tsx',
        patterns: ['const formatUptime'],
      },
      {
        path: 'src/features/proxmox/ProxmoxNodesTable.tsx',
        patterns: ['formatUptime(seconds)'],
      },
      { path: 'src/features/standalone/AgentsMachinesTable.tsx', patterns: ['const formatUptime'] },
      { path: 'src/features/truenas/TrueNASSystemsTable.tsx', patterns: ['const formatUptime'] },
      {
        path: 'src/features/vmware/VsphereHostsTable.tsx',
        patterns: ['formatUptime(host.uptime'],
      },
    ]);
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('formatPlatformTableUptimeValue');
    expect(localHelperGuard?.allPatterns).toEqual([
      'const formatUptime',
      'seconds / 86_400',
      'return `${mins}m`',
    ]);
    expect(localHelperGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/proxmox',
      'src/features/standalone',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(functionHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(functionHelperGuard?.canonical?.export).toBe('formatPlatformTableUptimeValue');
    expect(functionHelperGuard?.allPatterns).toEqual([
      'function formatUptime',
      'const days = Math.floor(seconds / 86_400);',
      'return `${mins}m`',
    ]);
    expect(functionHelperGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/proxmox',
      'src/features/standalone',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(functionHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(functionHelperGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(directFormatGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(directFormatGuard?.canonical?.export).toBe('formatPlatformTableUptimeValue');
    expect(directFormatGuard?.allPatterns).toEqual(['formatUptime(']);
    expect(directFormatGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/proxmox',
      'src/features/standalone',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(directFormatGuard?.pathIncludes).toEqual(['Table']);
    expect(directFormatGuard?.pathExcludes).toEqual(['__tests__']);
    expect(directFormatGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(directFormatGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export const formatPlatformTableUptimeValue');
    expect(sharedPlatformPageSource).toContain('formatUptime(seconds, options.compact)');

    for (const [, source] of platformUptimeConsumers) {
      expect(source).toContain('formatPlatformTableUptimeValue');
      expect(source).not.toContain('const formatUptime');
      expect(source).not.toContain('function formatUptime');
      expect(source).not.toContain('formatUptime(');
      expect(source).not.toContain('const days = Math.floor(seconds / 86_400);');
      expect(source).not.toContain('return `${mins}m`');
    }
  });

  it('keeps platform table byte-size fallbacks on the shared helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-byte-value-fallback',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-bytes-helper',
    );
    const localFormatBytesImportGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-format-bytes-import',
    );
    const platformByteValueConsumers: Array<[string, string]> = [
      ['src/features/docker/DockerNativeTableShared.tsx', dockerNativeTableSharedSource],
      ['src/features/docker/DockerStorageUsageTable.tsx', dockerStorageUsageTableSource],
      ['src/features/kubernetes/KubernetesNodesTable.tsx', kubernetesNodesTableSource],
      ['src/features/kubernetes/KubernetesStorageTable.tsx', kubernetesStorageTableSource],
      ['src/features/proxmox/ProxmoxBackupServersTable.tsx', proxmoxBackupServersTableSource],
      ['src/features/proxmox/ProxmoxCephTable.tsx', proxmoxCephTableSource],
      ['src/features/proxmox/ProxmoxCoverageTable.tsx', proxmoxCoverageTableSource],
      ['src/features/proxmox/ProxmoxRecoverableTable.tsx', proxmoxRecoverableTableSource],
      ['src/features/truenas/TrueNASProtectionTable.tsx', truenasProtectionTableSource],
      ['src/features/truenas/TrueNASStorageTopologyTable.tsx', truenasStorageTopologyTableSource],
      ['src/features/truenas/TrueNASSystemsTable.tsx', truenasSystemsTableSource],
      ['src/features/truenas/TrueNASVirtualMachinesTable.tsx', truenasVirtualMachinesTableSource],
    ];
    const platformByteValueConsumerPaths = platformByteValueConsumers.map(([path]) => path);
    const localFormatBytesHelperPatterns = [
      'const formatBytes',
      'value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)',
    ];

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('formatPlatformTableBytesValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformByteValueConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/docker/DockerNativeTableShared.tsx',
        patterns: ['formatBytes(value)'],
      },
      {
        path: 'src/features/docker/DockerStorageUsageTable.tsx',
        patterns: ['formatBytes('],
      },
      {
        path: 'src/features/kubernetes/KubernetesNodesTable.tsx',
        patterns: localFormatBytesHelperPatterns,
      },
      {
        path: 'src/features/kubernetes/KubernetesStorageTable.tsx',
        patterns: ['formatBytes('],
      },
      {
        path: 'src/features/proxmox/ProxmoxBackupServersTable.tsx',
        patterns: ['formatBytes('],
      },
      {
        path: 'src/features/proxmox/ProxmoxCephTable.tsx',
        patterns: ['formatBytes('],
      },
      {
        path: 'src/features/proxmox/ProxmoxCoverageTable.tsx',
        patterns: ['formatBytes('],
      },
      {
        path: 'src/features/proxmox/ProxmoxRecoverableTable.tsx',
        patterns: ['formatBytes('],
      },
      {
        path: 'src/features/truenas/TrueNASProtectionTable.tsx',
        patterns: ['formatBytes(point.sizeBytes)'],
      },
      {
        path: 'src/features/truenas/TrueNASStorageTopologyTable.tsx',
        patterns: [
          'formatBytes(size)',
          'formatBytes(row.resource.disk.used)',
          'formatBytes(row.resource.disk.total)',
        ],
      },
      {
        path: 'src/features/truenas/TrueNASSystemsTable.tsx',
        patterns: localFormatBytesHelperPatterns,
      },
      {
        path: 'src/features/truenas/TrueNASVirtualMachinesTable.tsx',
        patterns: localFormatBytesHelperPatterns,
      },
    ]);
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('formatPlatformTableBytesValue');
    expect(localHelperGuard?.allPatterns).toEqual(localFormatBytesHelperPatterns);
    expect(localHelperGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/proxmox',
      'src/features/truenas',
    ]);
    expect(localHelperGuard?.pathIncludes).toEqual(['Table']);
    expect(localHelperGuard?.pathExcludes).toEqual(['__tests__']);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localFormatBytesImportGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localFormatBytesImportGuard?.canonical?.export).toBe('formatPlatformTableBytesValue');
    expect(localFormatBytesImportGuard?.allPatterns).toEqual([
      "import { formatBytes } from '@/utils/format';",
      'formatBytes(',
    ]);
    expect(localFormatBytesImportGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/proxmox',
      'src/features/truenas',
    ]);
    expect(localFormatBytesImportGuard?.pathIncludes).toEqual(['Table']);
    expect(localFormatBytesImportGuard?.pathExcludes).toEqual(['__tests__']);
    expect(localFormatBytesImportGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localFormatBytesImportGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export const formatPlatformTableBytesValue');
    expect(sharedPlatformPageSource).toContain('formatBytes(bytes)');

    for (const [path, source] of platformByteValueConsumers) {
      expect(source).toContain('formatPlatformTableBytesValue');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
    expect(truenasVirtualMachinesTableSource).toContain(
      "formatPlatformTableBytesValue(vm()?.memoryBytes, '-')",
    );
  });

  it('keeps platform table date-time values on the shared primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-date-time-value',
    );
    const truenasDateGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-truenas-date-time-helper',
    );
    const vsphereDateGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-vsphere-activity-date-helper',
    );
    const platformDateTimeConsumers: Array<[string, string]> = [
      ['src/features/truenas/TrueNASProtectionTable.tsx', truenasProtectionTableSource],
      ['src/features/vmware/VsphereActivityTable.tsx', vsphereActivityTableSource],
    ];
    const platformDateTimeConsumerPaths = platformDateTimeConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformTableDateTimeValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformDateTimeConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/truenas/TrueNASProtectionTable.tsx',
        patterns: ['const formatPointTime', 'formatPointTime(point)'],
      },
      {
        path: 'src/features/vmware/VsphereActivityTable.tsx',
        patterns: [
          'const formatActivityDate',
          'formatActivityDate(activity.occurredAt || activity.observedAt)',
        ],
      },
    ]);
    expect(truenasDateGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(truenasDateGuard?.canonical?.export).toBe('PlatformTableDateTimeValue');
    expect(truenasDateGuard?.allPatterns).toEqual([
      'const formatPointTime',
      'toLocaleString(undefined, {',
      "minute: '2-digit'",
    ]);
    expect(truenasDateGuard?.scopes).toEqual(['src/features/truenas/TrueNASProtectionTable.tsx']);
    expect(truenasDateGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(truenasDateGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(vsphereDateGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(vsphereDateGuard?.canonical?.export).toBe('PlatformTableDateTimeValue');
    expect(vsphereDateGuard?.allPatterns).toEqual([
      'const formatActivityDate',
      'toLocaleString(undefined, {',
      "minute: '2-digit'",
    ]);
    expect(vsphereDateGuard?.scopes).toEqual(['src/features/vmware/VsphereActivityTable.tsx']);
    expect(vsphereDateGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(vsphereDateGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export function PlatformTableDateTimeValue');
    expect(sharedPlatformPageSource).toContain('formatPlatformTableDateTimeValue');
    expect(sharedPlatformPageSource).toContain('PLATFORM_TABLE_COMPACT_DATE_TIME_FORMAT');

    for (const [path, source] of platformDateTimeConsumers) {
      expect(source).toContain('PlatformTableDateTimeValue');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }

    // The When header renders through the shared sortable head; the
    // numeric-value alignment intent now lives on its kind prop.
    expect(vsphereActivityTableSource).toContain('kind="numeric-value"');
    expect(vsphereActivityTableSource).toContain('minYear={2000}');
  });

  it('keeps platform table relative timestamps on the shared primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        pathIncludes?: string[];
        pathExcludes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-relative-time-value',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-relative-time-helper',
    );
    const platformRelativeTimeConsumers: Array<[string, string]> = [
      ['src/features/docker/DockerVolumesTable.tsx', dockerVolumesTableSource],
      ['src/features/kubernetes/KubernetesDeploymentsTable.tsx', kubernetesDeploymentsTableSource],
      ['src/features/kubernetes/KubernetesEventsTable.tsx', kubernetesEventsTableSource],
      ['src/features/proxmox/proxmoxBackupsTableShared.tsx', proxmoxBackupsTableSharedSource],
      ['src/features/proxmox/ProxmoxReplicationTable.tsx', proxmoxReplicationTableSource],
      ['src/features/standalone/AvailabilityChecksTable.tsx', availabilityChecksTableSource],
      ['src/features/standalone/AgentsMachinesTable.tsx', agentsMachinesTableSource],
    ];
    const platformRelativeTimeConsumerPaths = platformRelativeTimeConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformTableRelativeTimeValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformRelativeTimeConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/docker/DockerVolumesTable.tsx',
        patterns: ['formatRelativeTime(parsed)'],
      },
      {
        path: 'src/features/kubernetes/KubernetesDeploymentsTable.tsx',
        patterns: ["formatRelativeTime(createdAt, { compact: true, emptyText: '—' })"],
      },
      {
        path: 'src/features/kubernetes/KubernetesEventsTable.tsx',
        patterns: [
          "formatRelativeTime(observedTimestamp(resource), { compact: true, emptyText: '—' })",
        ],
      },
      {
        path: 'src/features/proxmox/proxmoxBackupsTableShared.tsx',
        patterns: ['formatRelativeTime(props.artifact.createdAt, { compact: true })'],
      },
      {
        path: 'src/features/proxmox/ProxmoxReplicationTable.tsx',
        patterns: [
          'formatRelativeTime(job.lastSyncUnix * 1000, { compact: true })',
          'formatRelativeTime(job.lastSyncTime, { compact: true })',
        ],
      },
      {
        path: 'src/features/standalone/AgentsMachinesTable.tsx',
        patterns: ['const formatLastSeen', 'Math.floor((Date.now() - timestampMillis) / 1000)'],
      },
      {
        path: 'src/features/standalone/AvailabilityChecksTable.tsx',
        patterns: ['formatRelativeTime(availability?.lastChecked', 'const formatChecked'],
      },
    ]);
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('PlatformTableRelativeTimeValue');
    expect(localHelperGuard?.allPatterns).toEqual(['formatRelativeTime(']);
    expect(localHelperGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/proxmox',
      'src/features/standalone',
    ]);
    expect(localHelperGuard?.pathIncludes).toEqual(['Table']);
    expect(localHelperGuard?.pathExcludes).toEqual(['__tests__']);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export function PlatformTableRelativeTimeValue');
    expect(sharedPlatformPageSource).toContain('formatPlatformTableRelativeTimeValue');
    expect(sharedPlatformPageSource).toContain('formatRelativeTime(value');

    for (const [path, source] of platformRelativeTimeConsumers) {
      expect(source).toContain('PlatformTableRelativeTimeValue');
      expect(source).not.toContain('formatRelativeTime(');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
  });

  it('keeps platform table durations and intervals on the shared primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-duration-value',
    );
    const localDurationGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-duration-helper',
    );
    const localIntervalGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-interval-helper',
    );
    const platformDurationConsumers: Array<[string, string]> = [
      ['src/features/proxmox/ProxmoxReplicationTable.tsx', proxmoxReplicationTableSource],
      ['src/features/standalone/AvailabilityChecksTable.tsx', availabilityChecksTableSource],
    ];
    const platformDurationConsumerPaths = platformDurationConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformTableDurationValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformDurationConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/proxmox/ProxmoxReplicationTable.tsx',
        patterns: ['function formatDuration', 'seconds < 3600'],
      },
      {
        path: 'src/features/standalone/AvailabilityChecksTable.tsx',
        patterns: ['const formatInterval', 'toFixed(1)}m'],
      },
    ]);
    expect(localDurationGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localDurationGuard?.canonical?.export).toBe('PlatformTableDurationValue');
    expect(localDurationGuard?.allPatterns).toEqual([
      'function formatDuration',
      'seconds < 3600',
      'return `${h}h ${m}m`',
    ]);
    expect(localDurationGuard?.scopes).toEqual([
      'src/features/proxmox/ProxmoxReplicationTable.tsx',
    ]);
    expect(localDurationGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localDurationGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localIntervalGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localIntervalGuard?.canonical?.export).toBe('PlatformTableDurationValue');
    expect(localIntervalGuard?.allPatterns).toEqual([
      'const formatInterval',
      'pollIntervalSeconds',
      'toFixed(1)}m',
    ]);
    expect(localIntervalGuard?.scopes).toEqual([
      'src/features/standalone/AvailabilityChecksTable.tsx',
    ]);
    expect(localIntervalGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localIntervalGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export function PlatformTableDurationValue');
    expect(sharedPlatformPageSource).toContain('formatPlatformTableDurationValue');

    for (const [path, source] of platformDurationConsumers) {
      expect(source).toContain('PlatformTableDurationValue');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
  });

  it('keeps platform table integer count formatting on the shared helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-integer-value-format',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-integer-format-helper',
    );
    const localToLocaleGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-integer-count-tolocalestring',
    );
    const platformIntegerConsumers: Array<[string, string]> = [
      ['src/components/Kubernetes/K8sNamespacesDrawer.tsx', k8sNamespacesDrawerSource],
      ['src/features/proxmox/ProxmoxBackupServersTable.tsx', proxmoxBackupServersTableSource],
      ['src/features/proxmox/ProxmoxCephClusterDrawer.tsx', proxmoxCephClusterDrawerSource],
      ['src/features/proxmox/ProxmoxMailGatewayDrawer.tsx', proxmoxMailGatewayDrawerSource],
      ['src/features/proxmox/ProxmoxMailGatewayTable.tsx', proxmoxMailGatewayTableSource],
    ];
    const platformIntegerConsumerPaths = platformIntegerConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('formatPlatformTableIntegerValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformIntegerConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/Kubernetes/K8sNamespacesDrawer.tsx',
        patterns: ['const formatInteger', 'Math.round(n).toLocaleString()'],
      },
      {
        path: 'src/features/proxmox/ProxmoxBackupServersTable.tsx',
        patterns: ['row.backupCount.toLocaleString()'],
      },
      {
        path: 'src/features/proxmox/ProxmoxCephClusterDrawer.tsx',
        patterns: ['pool.objects.toLocaleString()'],
      },
      {
        path: 'src/features/proxmox/ProxmoxMailGatewayDrawer.tsx',
        patterns: ['function formatNumber', 'Math.round(value).toLocaleString()'],
      },
      {
        path: 'src/features/proxmox/ProxmoxMailGatewayTable.tsx',
        patterns: ['const formatLocaleCount', 'value.toLocaleString()'],
      },
    ]);
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('formatPlatformTableIntegerValue');
    expect(localHelperGuard?.allPatterns).toEqual(['Math.round(', '.toLocaleString()']);
    expect(localHelperGuard?.scopes).toEqual(['src/components/Kubernetes', 'src/features/proxmox']);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localToLocaleGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localToLocaleGuard?.canonical?.export).toBe('formatPlatformTableIntegerValue');
    expect(localToLocaleGuard?.allPatterns).toEqual(['toLocaleString()']);
    expect(localToLocaleGuard?.scopes).toEqual([
      'src/components/Kubernetes',
      'src/features/proxmox',
    ]);
    expect(localToLocaleGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localToLocaleGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export const formatPlatformTableIntegerValue');
    expect(sharedPlatformPageSource).toContain('maximumFractionDigits: 0');

    for (const [path, source] of platformIntegerConsumers) {
      expect(source).toContain('formatPlatformTableIntegerValue');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
    for (const source of [
      proxmoxBackupServersTableSource,
      proxmoxCephClusterDrawerSource,
      proxmoxMailGatewayDrawerSource,
      proxmoxMailGatewayTableSource,
    ]) {
      expect(source).toContain('PlatformTableNumberValue');
    }
  });

  it('keeps responsive platform table column widths on the shared weighted helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-weighted-column-width-style',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-weighted-width-helper',
    );
    const platformWidthConsumers: Array<[string, string]> = [
      ['src/features/docker/dockerContainerTableModel.ts', dockerContainerTableModelSource],
      ['src/features/proxmox/proxmoxHostTableModel.ts', proxmoxHostTableModelSource],
    ];
    const platformWidthConsumerPaths = platformWidthConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('getPlatformTableWeightedColumnWidthStyle');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformWidthConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/docker/dockerContainerTableModel.ts',
        patterns: ['const formatPercentage', 'value.toFixed(4)'],
      },
      {
        path: 'src/features/proxmox/proxmoxHostTableModel.ts',
        patterns: ['const formatPercentage', 'value.toFixed(4)'],
      },
    ]);
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('getPlatformTableWeightedColumnWidthStyle');
    expect(localHelperGuard?.allPatterns).toEqual([
      'const formatPercentage',
      'value.toFixed(4)',
      'return { width: formatPercentage(width) }',
    ]);
    expect(localHelperGuard?.scopes).toEqual([
      'src/features/docker/dockerContainerTableModel.ts',
      'src/features/proxmox/proxmoxHostTableModel.ts',
    ]);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain(
      'export const getPlatformTableWeightedColumnWidthStyle',
    );

    for (const [path, source] of platformWidthConsumers) {
      expect(source).toContain('getPlatformTableWeightedColumnWidthStyle');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
  });

  it('keeps platform table number-value fallbacks on the shared primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-number-value-fallback',
    );
    const localNumberGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-number-value-helper',
    );
    const localCountGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-count-cell-helper',
    );
    const localReplicaCountGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-replica-count-helper',
    );
    const localSwarmCountGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-swarm-count-spans',
    );
    const localTrueNASCountGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-truenas-count-cell-class',
    );
    const platformNumberValueConsumers: Array<[string, string]> = [
      ['src/features/docker/DockerNativeTableShared.tsx', dockerNativeTableSharedSource],
      ['src/features/docker/DockerServicesTable.tsx', dockerServicesTableSource],
      ['src/components/Docker/SwarmServicesDrawer.tsx', swarmServicesDrawerSource],
      ['src/features/kubernetes/KubernetesAutoscalingTable.tsx', kubernetesAutoscalingTableSource],
      ['src/features/kubernetes/KubernetesControllersTable.tsx', kubernetesControllersTableSource],
      ['src/features/kubernetes/KubernetesDeploymentsTable.tsx', kubernetesDeploymentsTableSource],
      ['src/features/kubernetes/KubernetesEventsTable.tsx', kubernetesEventsTableSource],
      ['src/features/kubernetes/KubernetesPodsTable.tsx', kubernetesPodsTableSource],
      ['src/features/proxmox/ProxmoxMailGatewayTable.tsx', proxmoxMailGatewayTableSource],
      ['src/features/truenas/TrueNASStorageTopologyTable.tsx', truenasStorageTopologyTableSource],
      ['src/features/truenas/TrueNASSystemsTable.tsx', truenasSystemsTableSource],
    ];
    const platformNumberValueConsumerPaths = platformNumberValueConsumers.map(([path]) => path);
    const optionalNumberMarkupPattern =
      'typeof value === \'number\' ? <span class="tabular-nums">{value}</span> : <span>—</span>';

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformTableNumberValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformNumberValueConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/docker/DockerNativeTableShared.tsx',
        patterns: [optionalNumberMarkupPattern],
      },
      {
        path: 'src/features/docker/DockerServicesTable.tsx',
        patterns: ['const replicaCount', '<span class="tabular-nums">{value ?? 0}</span>'],
      },
      {
        path: 'src/components/Docker/SwarmServicesDrawer.tsx',
        patterns: [
          '<span class="tabular-nums">{desired()}</span>',
          '<span class="tabular-nums">{running()}</span>',
        ],
      },
      {
        path: 'src/features/kubernetes/KubernetesAutoscalingTable.tsx',
        patterns: ['const numberValue', optionalNumberMarkupPattern],
      },
      {
        path: 'src/features/kubernetes/KubernetesControllersTable.tsx',
        patterns: ['const numberValue', optionalNumberMarkupPattern],
      },
      {
        path: 'src/features/kubernetes/KubernetesDeploymentsTable.tsx',
        patterns: ['const replicaCount', '<span class="tabular-nums">{value ?? 0}</span>'],
      },
      {
        path: 'src/features/kubernetes/KubernetesEventsTable.tsx',
        patterns: ['const numberValue', optionalNumberMarkupPattern],
      },
      {
        path: 'src/features/kubernetes/KubernetesPodsTable.tsx',
        patterns: ['const numericValue', optionalNumberMarkupPattern],
      },
      {
        path: 'src/features/proxmox/ProxmoxMailGatewayTable.tsx',
        patterns: ['const countCell', "value.toLocaleString() : '—'"],
      },
      {
        path: 'src/features/truenas/TrueNASSystemsTable.tsx',
        patterns: ['hidden text-base-content tabular-nums lg:table-cell'],
      },
      {
        path: 'src/features/truenas/TrueNASStorageTopologyTable.tsx',
        patterns: ['const diskCountLabel'],
      },
    ]);
    expect(localNumberGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localNumberGuard?.canonical?.export).toBe('PlatformTableNumberValue');
    expect(localNumberGuard?.allPatterns).toEqual([
      '<span class="tabular-nums">{value}</span>',
      '<span>—</span>',
    ]);
    expect(localNumberGuard?.scopes).toEqual(['src/features/docker', 'src/features/kubernetes']);
    expect(localNumberGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localNumberGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localCountGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localCountGuard?.canonical?.export).toBe('PlatformTableNumberValue');
    expect(localCountGuard?.allPatterns).toEqual([
      'const countCell',
      "value.toLocaleString() : '—'",
    ]);
    expect(localCountGuard?.scopes).toEqual(['src/features/proxmox']);
    expect(localCountGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localCountGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localReplicaCountGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localReplicaCountGuard?.canonical?.export).toBe('PlatformTableNumberValue');
    expect(localReplicaCountGuard?.allPatterns).toEqual([
      'const replicaCount',
      '<span class="tabular-nums">{value ?? 0}</span>',
    ]);
    expect(localReplicaCountGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
    ]);
    expect(localReplicaCountGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localReplicaCountGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localSwarmCountGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localSwarmCountGuard?.canonical?.export).toBe('PlatformTableNumberValue');
    expect(localSwarmCountGuard?.allPatterns).toEqual([
      'desiredTasks',
      'runningTasks',
      'class="tabular-nums"',
    ]);
    expect(localSwarmCountGuard?.scopes).toEqual(['src/components/Docker']);
    expect(localSwarmCountGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localSwarmCountGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localTrueNASCountGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localTrueNASCountGuard?.canonical?.export).toBe('PlatformTableNumberValue');
    expect(localTrueNASCountGuard?.allPatterns).toEqual([
      'TrueNAS',
      'hidden text-base-content tabular-nums lg:table-cell',
    ]);
    expect(localTrueNASCountGuard?.scopes).toEqual(['src/features/truenas']);
    expect(localTrueNASCountGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localTrueNASCountGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export function PlatformTableNumberValue');
    expect(sharedPlatformPageSource).toContain('Number.isFinite(value)');
    expect(sharedPlatformPageSource).toContain('class="tabular-nums"');

    for (const [path, source] of platformNumberValueConsumers) {
      expect(source).toContain('PlatformTableNumberValue');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
    expect(proxmoxMailGatewayTableSource).toContain('format={formatPlatformTableIntegerValue}');
  });

  it('keeps platform table count ratios on the shared primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-count-ratio-value',
    );
    const formatterRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-count-ratio-label',
    );
    const localCountRatioGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-count-ratio-helper',
    );
    const localReadyCountRatioGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-ready-count-ratio-label',
    );
    const platformCountRatioConsumers: Array<[string, string]> = [
      ['src/features/kubernetes/KubernetesClustersTable.tsx', kubernetesClustersTableSource],
    ];
    const platformCountRatioFormatterConsumers: Array<[string, string]> = [
      ['src/features/kubernetes/KubernetesNetworkingTable.tsx', kubernetesNetworkingTableSource],
    ];
    const platformCountRatioConsumerPaths = platformCountRatioConsumers.map(([path]) => path);
    const platformCountRatioFormatterConsumerPaths = platformCountRatioFormatterConsumers.map(
      ([path]) => path,
    );

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformTableCountRatioValue');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformCountRatioConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/kubernetes/KubernetesClustersTable.tsx',
        patterns: ['const childCountCell', '<span class="text-muted">/{count.total}</span>'],
      },
    ]);
    expect(formatterRule?.canonical?.path).toBe('src/features/platformPage/sharedPlatformPage.tsx');
    expect(formatterRule?.canonical?.export).toBe('formatPlatformTableCountRatioValue');
    expect(formatterRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformCountRatioFormatterConsumerPaths,
    );
    expect(formatterRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/kubernetes/KubernetesNetworkingTable.tsx',
        patterns: [
          '`${readyValue}/${totalValue} ready`',
          '`${ready ?? 0}/${total ?? ready ?? 0} ready`',
        ],
      },
    ]);
    expect(localCountRatioGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localCountRatioGuard?.canonical?.export).toBe('PlatformTableCountRatioValue');
    expect(localCountRatioGuard?.allPatterns).toEqual([
      'const childCountCell',
      '<span class="text-muted">/{count.total}</span>',
    ]);
    expect(localCountRatioGuard?.scopes).toEqual(['src/features/kubernetes']);
    expect(localCountRatioGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localCountRatioGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localReadyCountRatioGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localReadyCountRatioGuard?.canonical?.export).toBe('formatPlatformTableCountRatioValue');
    expect(localReadyCountRatioGuard?.allPatterns).toEqual([
      'readyEndpointCount',
      '/${',
      ' ready`',
    ]);
    expect(localReadyCountRatioGuard?.scopes).toEqual([
      'src/features/kubernetes/KubernetesNetworkingTable.tsx',
    ]);
    expect(localReadyCountRatioGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localReadyCountRatioGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export function PlatformTableCountRatioValue');
    expect(sharedPlatformPageSource).toContain(
      'export function formatPlatformTableCountRatioValue',
    );

    for (const [path, source] of platformCountRatioConsumers) {
      expect(source).toContain('PlatformTableCountRatioValue');
      const forbiddenPatterns =
        registeredRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }

    for (const [path, source] of platformCountRatioFormatterConsumers) {
      expect(source).toContain('formatPlatformTableCountRatioValue');
      const forbiddenPatterns =
        formatterRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
  });

  it('keeps platform table scalar unit values on shared primitives', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const percentRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-percent-value-fallback',
    );
    const temperatureRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-temperature-value-fallback',
    );
    const localPercentGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-percent-value-helper',
    );
    const localProxmoxPercentToFixedGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-proxmox-percent-tofixed-label',
    );
    const localProxmoxPercentLabelGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-proxmox-percent-label-helper',
    );
    const localTemperatureGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-temperature-value-helper',
    );
    const localTemperatureLabelGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-temperature-label-helper',
    );
    const percentConsumers: Array<[string, string]> = [
      ['src/features/proxmox/ProxmoxBackupServersTable.tsx', proxmoxBackupServersTableSource],
      ['src/features/proxmox/ProxmoxCephClusterDrawer.tsx', proxmoxCephClusterDrawerSource],
      ['src/features/proxmox/ProxmoxCephTable.tsx', proxmoxCephTableSource],
      ['src/features/proxmox/ProxmoxMailGatewayDrawer.tsx', proxmoxMailGatewayDrawerSource],
      ['src/features/proxmox/ProxmoxNodesTable.tsx', proxmoxNodesTableSource],
      ['src/features/truenas/TrueNASSystemsTable.tsx', truenasSystemsTableSource],
    ];
    const temperatureConsumers: Array<[string, string]> = [
      ['src/features/docker/DockerHostsTable.tsx', dockerHostsTableSource],
      ['src/features/truenas/TrueNASStorageTopologyTable.tsx', truenasStorageTopologyTableSource],
      ['src/features/truenas/TrueNASSystemsTable.tsx', truenasSystemsTableSource],
    ];
    const localPercentPatterns = [
      'const formatPercent',
      '<span class="tabular-nums">{percent.toFixed(1)}%</span>',
    ];
    const localTemperaturePatterns = [
      'const formatTemperature',
      '<span class="tabular-nums">{celsius.toFixed(1)}°C</span>',
    ];

    expect(percentRule?.canonical?.path).toBe('src/features/platformPage/sharedPlatformPage.tsx');
    expect(percentRule?.canonical?.export).toBe('PlatformTablePercentValue');
    expect(percentRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      percentConsumers.map(([path]) => path),
    );
    expect(percentRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/proxmox/ProxmoxBackupServersTable.tsx',
        patterns: [
          'Math.round(row.cpuPercent ?? 0)}%',
          'Math.round(row.memoryPercent ?? 0)}%',
          'Math.round(pct() ?? 0)}%',
        ],
      },
      {
        path: 'src/features/proxmox/ProxmoxCephClusterDrawer.tsx',
        patterns: ['clamped.toFixed(1)}%'],
      },
      {
        path: 'src/features/proxmox/ProxmoxCephTable.tsx',
        patterns: ['pct.toFixed(1)}%'],
      },
      {
        path: 'src/features/proxmox/ProxmoxMailGatewayDrawer.tsx',
        patterns: ['share.toFixed(1)}%'],
      },
      {
        path: 'src/features/proxmox/ProxmoxNodesTable.tsx',
        patterns: ['const formatPercentLabel', '`${Math.round(Math.max(0, normalized))}%`'],
      },
      {
        path: 'src/features/truenas/TrueNASSystemsTable.tsx',
        patterns: localPercentPatterns,
      },
    ]);

    expect(temperatureRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(temperatureRule?.canonical?.export).toBe('PlatformTableTemperatureValue');
    expect(temperatureRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      temperatureConsumers.map(([path]) => path),
    );
    expect(temperatureRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/docker/DockerHostsTable.tsx',
        patterns: localTemperaturePatterns,
      },
      {
        path: 'src/features/truenas/TrueNASStorageTopologyTable.tsx',
        patterns: ['const temperatureLabel', '${Math.round(value)}C'],
      },
      {
        path: 'src/features/truenas/TrueNASSystemsTable.tsx',
        patterns: localTemperaturePatterns,
      },
    ]);

    expect(localPercentGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localPercentGuard?.canonical?.export).toBe('PlatformTablePercentValue');
    expect(localPercentGuard?.allPatterns).toEqual(localPercentPatterns);
    expect(localPercentGuard?.scopes).toEqual(['src/features/truenas']);
    expect(localPercentGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localPercentGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localProxmoxPercentToFixedGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localProxmoxPercentToFixedGuard?.canonical?.export).toBe(
      'formatPlatformTablePercentValue',
    );
    expect(localProxmoxPercentToFixedGuard?.allPatterns).toEqual(['toFixed(1)}%']);
    expect(localProxmoxPercentToFixedGuard?.scopes).toEqual(['src/features/proxmox']);
    expect(localProxmoxPercentToFixedGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localProxmoxPercentToFixedGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localProxmoxPercentLabelGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localProxmoxPercentLabelGuard?.canonical?.export).toBe(
      'formatPlatformTablePercentValue',
    );
    expect(localProxmoxPercentLabelGuard?.allPatterns).toEqual(['formatPercentLabel']);
    expect(localProxmoxPercentLabelGuard?.scopes).toEqual(['src/features/proxmox']);
    expect(localProxmoxPercentLabelGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localProxmoxPercentLabelGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localTemperatureGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localTemperatureGuard?.canonical?.export).toBe('PlatformTableTemperatureValue');
    expect(localTemperatureGuard?.allPatterns).toEqual(localTemperaturePatterns);
    expect(localTemperatureGuard?.scopes).toEqual(['src/features/docker', 'src/features/truenas']);
    expect(localTemperatureGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localTemperatureGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localTemperatureLabelGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localTemperatureLabelGuard?.canonical?.export).toBe('PlatformTableTemperatureValue');
    expect(localTemperatureLabelGuard?.allPatterns).toEqual([
      'const temperatureLabel',
      '${Math.round(value)}C',
    ]);
    expect(localTemperatureLabelGuard?.scopes).toEqual(['src/features/truenas']);
    expect(localTemperatureLabelGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localTemperatureLabelGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export function PlatformTablePercentValue');
    expect(sharedPlatformPageSource).toContain('export const formatPlatformTablePercentValue');
    expect(sharedPlatformPageSource).toContain('export function PlatformTableTemperatureValue');
    expect(sharedPlatformPageSource).toContain('formatOneDecimalCelsius');

    for (const [path, source] of percentConsumers) {
      expect(source).toMatch(/PlatformTablePercentValue|formatPlatformTablePercentValue/);
      const forbiddenPatterns =
        percentRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }

    for (const [path, source] of temperatureConsumers) {
      expect(source).toContain('PlatformTableTemperatureValue');
      const forbiddenPatterns =
        temperatureRule?.forbiddenPatterns?.find((entry) => entry.path === path)?.patterns ?? [];
      for (const pattern of forbiddenPatterns) {
        expect(source).not.toContain(pattern);
      }
    }
  });

  it('keeps platform table compact value summaries on the shared helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-value-summary',
    );
    const localCompactListGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-compact-list-helper',
    );
    const localSummaryGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-value-summary-helper',
    );
    const platformValueSummaryConsumers: Array<[string, string]> = [
      ['src/features/kubernetes/KubernetesAutoscalingTable.tsx', kubernetesAutoscalingTableSource],
      ['src/features/kubernetes/KubernetesConfigTable.tsx', kubernetesConfigTableSource],
      ['src/features/kubernetes/KubernetesNetworkingTable.tsx', kubernetesNetworkingTableSource],
      ['src/features/kubernetes/KubernetesPolicyTable.tsx', kubernetesPolicyTableSource],
      ['src/features/kubernetes/KubernetesServicesTable.tsx', kubernetesServicesTableSource],
      ['src/features/truenas/TrueNASNetworkSharesTable.tsx', truenasNetworkSharesTableSource],
      ['src/features/vmware/VsphereDatastoresTable.tsx', vsphereDatastoresTableSource],
      ['src/features/vmware/VsphereNetworksTable.tsx', vsphereNetworksTableSource],
    ];
    const platformValueSummaryConsumerPaths = platformValueSummaryConsumers.map(([path]) => path);

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('summarizePlatformTableValues');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformValueSummaryConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual(
      platformValueSummaryConsumerPaths.map((path) => ({
        path,
        patterns: ['const compactList', 'const summarizeValues'],
      })),
    );
    expect(localCompactListGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localCompactListGuard?.canonical?.export).toBe('summarizePlatformTableValues');
    expect(localCompactListGuard?.allPatterns).toEqual(['const compactList']);
    expect(localCompactListGuard?.scopes).toEqual([
      'src/features/kubernetes',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localCompactListGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localCompactListGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localSummaryGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localSummaryGuard?.canonical?.export).toBe('summarizePlatformTableValues');
    expect(localSummaryGuard?.allPatterns).toEqual(['const summarizeValues']);
    expect(localSummaryGuard?.scopes).toEqual([
      'src/features/kubernetes',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localSummaryGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localSummaryGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export const summarizePlatformTableValues');
    expect(sharedPlatformPageSource).toContain('export type PlatformTableValueSummary');

    for (const [, source] of platformValueSummaryConsumers) {
      expect(source).toContain('summarizePlatformTableValues');
      expect(source).not.toContain('const compactList');
      expect(source).not.toContain('const summarizeValues');
    }
  });

  it('keeps platform table metric fallbacks on the shared helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-metric-fallback',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-metric-fallback-helper',
    );
    const localMarkerGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-local-metric-empty-marker',
    );
    const platformMetricConsumers: Array<[string, string]> = [
      ['src/features/docker/DockerContainersTable.tsx', dockerContainersTableSource],
      ['src/features/docker/DockerHostsTable.tsx', dockerHostsTableSource],
      ['src/features/kubernetes/KubernetesClustersTable.tsx', kubernetesClustersTableSource],
      ['src/features/kubernetes/KubernetesNodesTable.tsx', kubernetesNodesTableSource],
      ['src/features/proxmox/ProxmoxNodesTable.tsx', proxmoxNodesTableSource],
      ['src/features/standalone/AgentsMachinesTable.tsx', agentsMachinesTableSource],
      ['src/features/truenas/TrueNASAppsTable.tsx', truenasAppsTableSource],
      ['src/features/truenas/TrueNASSystemsTable.tsx', truenasSystemsTableSource],
      ['src/features/vmware/VsphereDatastoresTable.tsx', vsphereDatastoresTableSource],
      ['src/features/vmware/VsphereHostsTable.tsx', vsphereHostsTableSource],
    ];
    const platformMetricConsumerPaths = platformMetricConsumers.map(([path]) => path);
    const inlineMetricMarkerPatterns = [
      'flex justify-center',
      'text-xs text-muted',
      'aria-hidden="true"',
    ];

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformTableMetricFallback');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      platformMetricConsumerPaths,
    );
    expect(registeredRule?.forbiddenPatterns).toEqual(
      platformMetricConsumerPaths.map((path) => ({
        path,
        patterns: ['const finiteMetric', 'const metricFallback'],
      })),
    );
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('PlatformTableMetricFallback');
    expect(localHelperGuard?.allPatterns).toEqual([
      'const finiteMetric',
      'const metricFallback',
      'Number.isFinite(value)',
    ]);
    expect(localHelperGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/proxmox',
      'src/features/standalone',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localMarkerGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localMarkerGuard?.canonical?.export).toBe('PlatformTableMetricFallback');
    expect(localMarkerGuard?.allPatterns).toEqual(inlineMetricMarkerPatterns);
    expect(localMarkerGuard?.scopes).toEqual([
      'src/features/docker',
      'src/features/kubernetes',
      'src/features/proxmox',
      'src/features/standalone',
      'src/features/truenas',
      'src/features/vmware',
    ]);
    expect(localMarkerGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localMarkerGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export function PlatformTableMetricFallback');
    expect(sharedPlatformPageSource).toContain('export const getPlatformTableFiniteMetric');
    expect(sharedPlatformPageSource).toContain('Number.isFinite(value)');

    for (const [, source] of platformMetricConsumers) {
      expect(source).toContain('PlatformTableMetricFallback');
      expect(source).toContain('getPlatformTableFiniteMetric');
      expect(source).not.toContain('const finiteMetric');
      expect(source).not.toContain('const metricFallback');
      expect(source).not.toContain('Number.isFinite(value)');
      expect(inlineMetricMarkerPatterns.every((pattern) => source.includes(pattern))).toBe(false);
    }
  });

  it('keeps platform table model metric normalization on the shared helper', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        extensions?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'platform-table-finite-metric-normalization',
    );
    const localHelperGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-table-model-local-finite-metric-helper',
    );

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('getPlatformTableFiniteMetric');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/features/standalone/agentMachineTableModel.ts',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/features/standalone/agentMachineTableModel.ts',
        patterns: ['const finiteMetric'],
      },
    ]);
    expect(localHelperGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(localHelperGuard?.canonical?.export).toBe('getPlatformTableFiniteMetric');
    expect(localHelperGuard?.allPatterns).toEqual(['const finiteMetric']);
    expect(localHelperGuard?.scopes).toEqual(['src/features/standalone/agentMachineTableModel.ts']);
    expect(localHelperGuard?.extensions).toEqual(['.ts']);
    expect(localHelperGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localHelperGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(sharedPlatformPageSource).toContain('export const getPlatformTableFiniteMetric');
    expect(agentMachineTableModelSource).toContain('getPlatformTableFiniteMetric');
    expect(agentMachineTableModelSource).not.toContain('const finiteMetric');
    expect(agentMachineTableModelSource).not.toContain(
      "typeof value === 'number' && Number.isFinite(value) ? value : undefined",
    );
  });

  it('keeps platform section navigation on the shared tabs template', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'platform-section-tabs');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'platform-section-tabs-local-nav',
    );
    const requiredConsumerPaths = [
      'src/features/docker/DockerPageSurface.tsx',
      'src/features/kubernetes/KubernetesPageSurface.tsx',
      'src/features/proxmox/ProxmoxPageSurface.tsx',
      'src/features/standalone/StandalonePageSurface.tsx',
      'src/features/truenas/TrueNASPageSurface.tsx',
      'src/features/vmware/VmwarePageSurface.tsx',
    ];
    const platformPageSurfaceSources = [
      dockerPageSurfaceSource,
      kubernetesPageSurfaceSource,
      proxmoxPageSurfaceSource,
      standalonePageSurfaceSource,
      truenasPageSurfaceSource,
      vmwarePageSurfaceSource,
    ];

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformSectionTabs');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      expect.arrayContaining(requiredConsumerPaths),
    );
    expect(registeredGuard?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredGuard?.canonical?.export).toBe('PlatformSectionTabs');
    expect(registeredGuard?.allPatterns).toEqual([
      'aria-current={',
      'border-b-2',
      'border-b border-border',
    ]);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining([
        'src/features/docker',
        'src/features/kubernetes',
        'src/features/proxmox',
        'src/features/standalone',
        'src/features/truenas',
        'src/features/vmware',
      ]),
    );

    expect(sharedPlatformPageSource).toContain('export function PlatformSectionTabs');
    expect(sharedPlatformPageSource).toContain('props.tabs.length > 1');
    expect(sharedPlatformPageSource).toContain('href={tab.path}');
    expect(sharedPlatformPageSource).toContain('border-b-2');
    expect(sharedPlatformPageSource).toContain(
      "aria-current={props.active === tab.id ? 'page' : undefined}",
    );

    for (const source of platformPageSurfaceSources) {
      expect(source).toContain('PlatformSectionTabs');
      expect(source).not.toContain('aria-current={');
      expect(source).not.toContain('border-b-2');
      expect(source).not.toContain('border-b border-border');
    }
  });

  it('keeps platform load-failure states on the shared error template', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      requiredPatternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        triggerPatterns?: string[];
        requiredPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'platform-error-state');
    const requiredGuard = registry.requiredPatternGuards?.find(
      (guard) => guard.id === 'platform-error-state-required',
    );
    const requiredConsumerPaths = [
      'src/features/docker/DockerPageSurface.tsx',
      'src/features/kubernetes/KubernetesPageSurface.tsx',
      'src/features/proxmox/ProxmoxBackupsTable.tsx',
      'src/features/proxmox/ProxmoxPageSurface.tsx',
      'src/features/proxmox/ProxmoxReplicationTable.tsx',
      'src/features/standalone/StandalonePageSurface.tsx',
      'src/features/truenas/TrueNASPageSurface.tsx',
      'src/features/truenas/TrueNASProtectionTable.tsx',
      'src/features/vmware/VmwarePageSurface.tsx',
    ];
    const localRefreshButtonClass =
      'inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover';

    expect(registeredRule?.canonical?.path).toBe(
      'src/features/platformPage/sharedPlatformPage.tsx',
    );
    expect(registeredRule?.canonical?.export).toBe('PlatformErrorState');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      expect.arrayContaining(requiredConsumerPaths),
    );
    expect(requiredGuard?.canonical?.path).toBe('src/features/platformPage/sharedPlatformPage.tsx');
    expect(requiredGuard?.canonical?.export).toBe('PlatformErrorState');
    expect(requiredGuard?.triggerPatterns).toEqual(['title="Could not load']);
    expect(requiredGuard?.requiredPatterns).toEqual(['PlatformErrorState']);
    expect(requiredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(requiredGuard?.scopes).toEqual(
      expect.arrayContaining([
        'src/features/docker',
        'src/features/kubernetes',
        'src/features/proxmox',
        'src/features/standalone',
        'src/features/truenas',
        'src/features/vmware',
      ]),
    );

    expect(sharedPlatformPageSource).toContain('export function PlatformErrorState');
    expect(sharedPlatformPageSource).toContain('TriangleAlertIcon');
    expect(sharedPlatformPageSource).toContain('onClick={props.onRefresh}');
    expect(sharedPlatformPageSource).toContain('Refresh');

    for (const source of [
      dockerPageSurfaceSource,
      kubernetesPageSurfaceSource,
      proxmoxBackupsTableSource,
      proxmoxPageSurfaceSource,
      proxmoxReplicationTableSource,
      standalonePageSurfaceSource,
      truenasPageSurfaceSource,
      truenasProtectionTableSource,
      vmwarePageSurfaceSource,
    ]) {
      expect(source).toContain('PlatformErrorState');
    }

    for (const source of [proxmoxBackupsTableSource, proxmoxReplicationTableSource]) {
      expect(source).not.toContain(localRefreshButtonClass);
    }
  });

  it('keeps platform row-detail disclosure controls on the shared toggle primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      requiredPatternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        triggerPatterns?: string[];
        requiredPatterns?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const requiredGuards = registry.requiredPatternGuards ?? [];
    const sharedStateGuard = requiredGuards.find(
      (guard) => guard.id === 'platform-row-detail-toggle-shared-state',
    );
    const inlineDetailGuard = requiredGuards.find(
      (guard) => guard.id === 'platform-row-detail-toggle-inline-detail',
    );

    for (const guard of [sharedStateGuard, inlineDetailGuard]) {
      expect(guard?.canonical?.path).toBe(
        'src/features/platformPage/PlatformResourceDetailTableRow.tsx',
      );
      expect(guard?.canonical?.export).toBe('PlatformResourceDetailToggleButton');
      expect(guard?.requiredPatterns).toContain('PlatformResourceDetailToggleButton');
      expect(guard?.allowedPaths ?? []).toHaveLength(0);
    }
    expect(sharedStateGuard?.triggerPatterns).toContain('createPlatformResourceDetailState');
    expect(inlineDetailGuard?.triggerPatterns).toEqual(['data-inline', 'aria-expanded']);

    expect(platformResourceDetailTableRowSource).toContain(
      'export const PlatformResourceDetailToggleButton',
    );
    expect(platformResourceDetailTableRowSource).toContain('subjectLabel={`details for');
    expect(platformResourceDetailTableRowSource).toContain('SummaryRowActionButton');
    expect(platformResourceDetailTableRowSource).toContain(
      'onResourceActionSettled?: () => void | Promise<void>',
    );
    expect(platformResourceDetailTableRowSource).toContain(
      'onResourceActionSettled={props.onResourceActionSettled}',
    );
    expect(platformResourceDetailTableRowSource).not.toContain('ResourceActionsAPI');

    for (const source of [
      dockerHostsTableSource,
      kubernetesNodesTableSource,
      proxmoxCoverageTableSource,
      proxmoxNodesTableSource,
      vsphereHostsTableSource,
      agentsMachinesTableSource,
    ]) {
      expect(source).toContain('PlatformResourceDetailToggleButton');
    }
    expect(agentsMachinesTableSource).not.toContain('data-agent-machine-expand-icon');
  });

  it('keeps inline detail table rows on the shared row shell primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      requiredPatternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        triggerPatterns?: string[];
        requiredPatterns?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'inline-detail-table-row-shell',
    );
    const requiredGuards = registry.requiredPatternGuards ?? [];
    const inlineGuard = requiredGuards.find(
      (guard) => guard.id === 'inline-detail-table-row-shared-shell',
    );
    const namedGuard = requiredGuards.find(
      (guard) => guard.id === 'named-detail-table-row-shared-shell',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/InlineDetailTableRow.tsx');
    expect(registeredRule?.canonical?.export).toBe('InlineDetailTableRow');
    expect(inlineDetailTableRowSource).toContain('INLINE_DETAIL_TABLE_CELL_CLASS');
    expect(inlineDetailTableRowSource).toContain('INLINE_DETAIL_TABLE_CONTENT_CLASS');
    expect(inlineDetailTableRowSource).toContain('event.stopPropagation()');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      expect.arrayContaining([
        'src/components/Infrastructure/UnifiedResourceHostTableCard.tsx',
        'src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx',
        'src/components/Infrastructure/UnifiedResourcePMGTableSection.tsx',
        'src/components/Workloads/WorkloadPanel.tsx',
        'src/features/docker/DockerHostsTable.tsx',
        'src/features/docker/DockerNetworksTable.tsx',
        'src/features/platformPage/PlatformResourceDetailTableRow.tsx',
        'src/features/proxmox/ProxmoxCoverageTable.tsx',
        'src/features/proxmox/ProxmoxNodesTable.tsx',
      ]),
    );

    for (const guard of [inlineGuard, namedGuard]) {
      expect(guard?.canonical?.path).toBe('src/components/shared/InlineDetailTableRow.tsx');
      expect(guard?.canonical?.export).toBe('InlineDetailTableRow');
      expect(guard?.requiredPatterns).toEqual(['InlineDetailTableRow']);
      expect(guard?.allowedPaths ?? []).toHaveLength(0);
    }
    expect(inlineGuard?.triggerPatterns).toEqual(['data-inline', 'TableRow']);
    expect(inlineGuard?.ignoredPaths).toEqual([
      'src/components/Workloads/__tests__/WorkloadsSurface.performance.contract.test.tsx',
    ]);
    expect(namedGuard?.triggerPatterns).toEqual(['detail-row', 'TableRow']);
    expect(namedGuard?.ignoredPaths ?? []).toHaveLength(0);

    for (const source of [
      platformResourceDetailTableRowSource,
      dockerHostsTableSource,
      proxmoxCoverageTableSource,
      proxmoxNodesTableSource,
      unifiedResourceHostTableCardSource,
      unifiedResourcePBSTableSectionSource,
      unifiedResourcePMGTableSectionSource,
      workloadPanelSource,
    ]) {
      expect(source).toContain('InlineDetailTableRow');
      expect(source).not.toContain('p-0 border-b border-border bg-surface-alt');
      expect(source).not.toContain('border-b border-border bg-surface-alt p-0');
      expect(source).not.toContain(
        'bg-surface-alt px-4 py-4 border-b border-border-subtle shadow-inner',
      );
    }
  });

  it('keeps inline detail section content on the shared detail primitive', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
        scopes?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find(
      (rule) => rule.id === 'inline-detail-section-panel-shell',
    );
    const fieldGridGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'inline-detail-local-field-grid-shell',
    );
    const providerNamedGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'inline-detail-provider-named-primitive-import',
    );
    const providerRowModelGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'resource-detail-local-provider-row-model',
    );
    const vmwareDetailShellGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'resource-detail-local-vmware-detail-shell',
    );
    const numericFormatRule = registry.rules?.find(
      (rule) => rule.id === 'resource-detail-numeric-value-formatting',
    );
    const localByteFormatGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'resource-detail-local-byte-format-helper',
    );
    const localCapacityByteFormatGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'resource-detail-local-capacity-byte-format-helper',
    );
    const localCountFormatGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'resource-detail-local-count-format-helper',
    );
    const localIntegerFormatGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'resource-detail-local-integer-format-helper',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/DetailSectionTable.tsx');
    expect(registeredRule?.canonical?.export).toBe('DetailSection');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      expect.arrayContaining([
        'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
        'src/components/Infrastructure/resourceDetailDrawerKubernetesModel.ts',
        'src/components/Infrastructure/resourceDetailDrawerTrueNASModel.ts',
        'src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts',
        'src/features/docker/DockerAlertsTable.tsx',
        'src/features/kubernetes/KubernetesAlertsTable.tsx',
        'src/features/truenas/TrueNASAlertsTable.tsx',
        'src/features/truenas/TrueNASProtectionTable.tsx',
        'src/features/truenas/TrueNASServicesTable.tsx',
        'src/features/vmware/VsphereActivityTable.tsx',
        'src/features/vmware/VsphereAlertsTable.tsx',
      ]),
    );
    expect(registeredRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/components/Infrastructure/resourceDetailDrawerKubernetesModel.ts',
          patterns: expect.arrayContaining(['makeTrueNASDetailRow', 'trueNASDetailTableModel']),
        }),
        expect.objectContaining({
          path: 'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
          patterns: expect.arrayContaining([
            'vmwareRowToneClass',
            '<For each={drawer.vmwareDetailSections()}',
          ]),
        }),
        expect.objectContaining({
          path: 'src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts',
          patterns: expect.arrayContaining([
            'ResourceDetailDrawerVMwareRow',
            'ResourceDetailDrawerVMwareSection',
            'ResourceDetailDrawerVMwareRowTone',
            'filterNonEmptyRows',
          ]),
        }),
      ]),
    );

    expect(fieldGridGuard?.canonical?.path).toBe('src/components/shared/DetailSectionTable.tsx');
    expect(fieldGridGuard?.canonical?.export).toBe('InlineDetailPanel');
    expect(fieldGridGuard?.allPatterns).toEqual([
      'const DetailField',
      'font-semibold uppercase tracking-wide text-muted',
      'grid gap-3 sm:grid-cols-2 lg:grid-cols-3',
    ]);
    expect(fieldGridGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(providerNamedGuard?.canonical?.path).toBe(
      'src/components/shared/DetailSectionTable.tsx',
    );
    expect(providerNamedGuard?.canonical?.export).toBe('DetailSectionTable');
    expect(providerNamedGuard?.allPatterns).toEqual(['TrueNASDetailTable']);
    expect(providerNamedGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(providerRowModelGuard?.canonical?.path).toBe(
      'src/components/shared/detailSectionModel.ts',
    );
    expect(providerRowModelGuard?.canonical?.export).toBe('DetailSection');
    expect(providerRowModelGuard?.allPatterns).toEqual([
      'ResourceDetailDrawerVMwareRow',
      'ResourceDetailDrawerVMwareSection',
    ]);
    expect(providerRowModelGuard?.scopes).toEqual([
      'src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts',
    ]);
    expect(providerRowModelGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(providerRowModelGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(vmwareDetailShellGuard?.canonical?.path).toBe(
      'src/components/shared/DetailSectionTable.tsx',
    );
    expect(vmwareDetailShellGuard?.canonical?.export).toBe('DetailSectionTable');
    expect(vmwareDetailShellGuard?.allPatterns).toEqual([
      'vmwareRowToneClass',
      '<For each={drawer.vmwareDetailSections()}',
    ]);
    expect(vmwareDetailShellGuard?.scopes).toEqual([
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
    ]);
    expect(vmwareDetailShellGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(vmwareDetailShellGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(numericFormatRule?.canonical?.path).toBe('src/components/shared/detailSectionModel.ts');
    expect(numericFormatRule?.canonical?.export).toBe('formatDetailBytesValue');
    expect(numericFormatRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Infrastructure/resourceDetailDrawerKubernetesModel.ts',
      'src/components/Infrastructure/resourceDetailDrawerTrueNASModel.ts',
      'src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts',
    ]);
    expect(numericFormatRule?.forbiddenPatterns).toEqual([
      {
        path: 'src/components/Infrastructure/resourceDetailDrawerKubernetesModel.ts',
        patterns: ['const formatBytes', "const units = ['B', 'KB', 'MB', 'GB', 'TB']"],
      },
      {
        path: 'src/components/Infrastructure/resourceDetailDrawerTrueNASModel.ts',
        patterns: [
          'const formatInteger',
          'const formatBytes',
          'const formatCount',
          'new Intl.NumberFormat().format',
        ],
      },
      {
        path: 'src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts',
        patterns: [
          'const formatCount',
          'const formatCapacityBytes',
          "const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']",
        ],
      },
    ]);
    expect(localByteFormatGuard?.canonical?.export).toBe('formatDetailBytesValue');
    expect(localByteFormatGuard?.scopes).toEqual([
      'src/components/Infrastructure/resourceDetailDrawerKubernetesModel.ts',
      'src/components/Infrastructure/resourceDetailDrawerTrueNASModel.ts',
    ]);
    expect(localByteFormatGuard?.allPatterns).toEqual(['const formatBytes', "const units = ['B'"]);
    expect(localByteFormatGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localByteFormatGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localCapacityByteFormatGuard?.canonical?.export).toBe('formatDetailBytesValue');
    expect(localCapacityByteFormatGuard?.scopes).toEqual([
      'src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts',
    ]);
    expect(localCapacityByteFormatGuard?.allPatterns).toEqual([
      'const formatCapacityBytes',
      "const units = ['B'",
    ]);
    expect(localCapacityByteFormatGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localCapacityByteFormatGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localCountFormatGuard?.canonical?.export).toBe('formatDetailCountValue');
    expect(localCountFormatGuard?.scopes).toEqual([
      'src/components/Infrastructure/resourceDetailDrawerTrueNASModel.ts',
      'src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts',
    ]);
    expect(localCountFormatGuard?.allPatterns).toEqual(['const formatCount']);
    expect(localCountFormatGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localCountFormatGuard?.ignoredPaths ?? []).toHaveLength(0);
    expect(localIntegerFormatGuard?.canonical?.export).toBe('formatDetailIntegerValue');
    expect(localIntegerFormatGuard?.scopes).toEqual([
      'src/components/Infrastructure/resourceDetailDrawerTrueNASModel.ts',
    ]);
    expect(localIntegerFormatGuard?.allPatterns).toEqual([
      'const formatInteger',
      'new Intl.NumberFormat().format',
    ]);
    expect(localIntegerFormatGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(localIntegerFormatGuard?.ignoredPaths ?? []).toHaveLength(0);

    expect(detailSectionModelSource).toContain('export type DetailSection');
    expect(detailSectionModelSource).toContain('makeDetailRow');
    expect(detailSectionModelSource).toContain('formatDetailBytesValue');
    expect(detailSectionModelSource).toContain('formatDetailCountValue');
    expect(detailSectionModelSource).toContain('formatDetailIntegerValue');
    expect(detailSectionTableSource).toContain('DetailSectionTable');
    expect(detailSectionTableSource).toContain('InlineDetailPanel');
    expect(detailSectionTableSource).toContain('variant="outline"');

    for (const source of [
      resourceDetailDrawerKubernetesModelSource,
      resourceDetailDrawerTrueNASModelSource,
      resourceDetailDrawerVmwareModelSource,
    ]) {
      expect(source).toContain('@/components/shared/detailSectionModel');
      expect(source).toContain('DetailSection');
      expect(source).not.toContain('trueNASDetailTableModel');
      expect(source).not.toContain('makeTrueNASDetailRow');
    }
    expect(resourceDetailDrawerVmwareModelSource).toContain('makeDetailRow');
    expect(resourceDetailDrawerVmwareModelSource).toContain('compactDetailRows');
    expect(resourceDetailDrawerVmwareModelSource).toContain('compactDetailSections');
    expect(resourceDetailDrawerVmwareModelSource).not.toContain('ResourceDetailDrawerVMwareRow');
    expect(resourceDetailDrawerVmwareModelSource).not.toContain(
      'ResourceDetailDrawerVMwareSection',
    );
    expect(resourceDetailDrawerVmwareModelSource).not.toContain(
      'ResourceDetailDrawerVMwareRowTone',
    );
    expect(resourceDetailDrawerVmwareModelSource).not.toContain('filterNonEmptyRows');
    for (const source of [
      resourceDetailDrawerKubernetesModelSource,
      resourceDetailDrawerTrueNASModelSource,
      resourceDetailDrawerVmwareModelSource,
    ]) {
      expect(source).toContain('formatDetailBytesValue');
      expect(source).not.toContain('const formatBytes');
      expect(source).not.toContain('const formatCapacityBytes');
    }
    expect(resourceDetailDrawerTrueNASModelSource).toContain('formatDetailIntegerValue');
    expect(resourceDetailDrawerTrueNASModelSource).toContain('formatDetailCountValue');
    expect(resourceDetailDrawerVmwareModelSource).toContain('formatDetailCountValue');
    expect(resourceDetailDrawerTrueNASModelSource).not.toContain('const formatInteger');
    expect(resourceDetailDrawerTrueNASModelSource).not.toContain('const formatCount');
    expect(resourceDetailDrawerVmwareModelSource).not.toContain('const formatCount');

    expect(resourceDetailDrawerOverviewTabSource).toContain('DetailSectionTable');
    expect(resourceDetailDrawerOverviewTabSource).toContain(
      '<DetailSectionTable sections={props.drawer.vmwareDetailSections()} />',
    );
    expect(resourceDetailDrawerOverviewTabSource).not.toContain('TrueNASDetailSectionTable');
    expect(resourceDetailDrawerOverviewTabSource).not.toContain('vmwareRowToneClass');
    expect(resourceDetailDrawerOverviewTabSource).not.toContain(
      '<For each={drawer.vmwareDetailSections()}',
    );

    for (const source of [
      dockerAlertsTableSource,
      kubernetesAlertsTableSource,
      truenasAlertsTableSource,
      truenasProtectionTableSource,
      truenasServicesTableSource,
      vsphereActivityTableSource,
      vsphereAlertsTableSource,
    ]) {
      expect(source).toContain('InlineDetailPanel');
      expect(source).toContain('build');
      expect(source).toContain('DetailSections');
      expect(source).not.toContain('const DetailField');
      expect(source).not.toContain('grid gap-3 sm:grid-cols-2 lg:grid-cols-3');
      expect(source).not.toContain('TrueNASInlineDetailTable');
      expect(source).not.toContain('TrueNASDetailTable');
      expect(source).not.toContain('XIcon class="h-4 w-4"');
    }
  });

  it('keeps help icon on shell, runtime, and model owners', () => {
    expect(helpIconSource).toContain('useHelpIconState');
    expect(helpIconSource).not.toContain('getHelpContent(');
    expect(helpIconSource).not.toContain('requestAnimationFrame');
    expect(helpIconSource).not.toContain('createSignal');

    expect(helpIconStateSource).toContain('requestAnimationFrame');
    expect(helpIconStateSource).toContain('document.addEventListener');
    expect(helpIconStateSource).toContain('export function useHelpIconState');
    expect(helpIconStateSource).toContain('createSignal');

    expect(helpIconModelSource).toContain('resolveHelpContent');
    expect(helpIconModelSource).toContain('calculateHelpPopoverPosition');
    expect(helpIconModelSource).toContain('helpIconSizeClasses');
  });

  it('keeps mobile nav on shell, runtime, and model owners', () => {
    expect(mobileNavBarSource).toContain('useMobileNavBarState');
    expect(mobileNavBarSource).toContain('getMobileNavTabButtonClass');
    expect(mobileNavBarSource).not.toContain('createSignal');
    expect(mobileNavBarSource).not.toContain('requestAnimationFrame');
    expect(mobileNavBarSource).not.toContain('new Set(priority)');

    expect(mobileNavBarStateSource).toContain('createSignal');
    expect(mobileNavBarStateSource).toContain('window.addEventListener');
    expect(mobileNavBarStateSource).toContain('requestAnimationFrame');
    expect(mobileNavBarStateSource).toContain('scrollIntoView');
    expect(mobileNavBarStateSource).toContain('export function useMobileNavBarState');

    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavPrimaryTabs');
    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavUtilityTabs');
    expect(mobileNavBarModelSource).toContain('getMobileNavAlertBadgeCounts');
    expect(mobileNavBarModelSource).toContain('getMobileNavFadeState');
  });

  it('keeps pulse data grid on shell, runtime, and model owners', () => {
    expect(pulseDataGridSource).toContain('usePulseDataGridState');
    expect(pulseDataGridSource).toContain('getPulseDataGridAlignClass');
    expect(pulseDataGridSource).toContain('getPulseDataGridWidthAttr');
    expect(pulseDataGridSource).toContain('isPulseDataGridInteractiveTarget');
    expect(pulseDataGridSource).toContain('scrollbar-hide');
    expect(pulseDataGridSource).not.toContain('style={{');
    expect(pulseDataGridSource).not.toContain('style={');
    expect(pulseDataGridSource).not.toContain('useBreakpoint');
    expect(pulseDataGridSource).not.toContain('createStore');
    expect(pulseDataGridSource).not.toContain('target.closest(');

    expect(pulseDataGridStateSource).toContain('export function usePulseDataGridState');
    expect(pulseDataGridStateSource).toContain('useBreakpoint');
    expect(pulseDataGridStateSource).toContain('createStore');
    expect(pulseDataGridStateSource).toContain('reconcile(');

    expect(pulseDataGridModelSource).toContain('export const getPulseDataGridAlignClass');
    expect(pulseDataGridModelSource).toContain('export const getPulseDataGridWidthAttr');
    expect(pulseDataGridModelSource).toContain('export const isPulseDataGridInteractiveTarget');
    expect(pulseDataGridModelSource).toContain('target.closest(');
  });

  it('keeps progress bars CSP-safe in the shared primitive owner', () => {
    expect(progressBarSource).toContain('data-progress-fill');
    expect(progressBarSource).toContain('foreignObject');
    expect(progressBarSource).toContain('progress-fill-frame');
    expect(progressBarSource).not.toContain('style={{');
    expect(progressBarSource).not.toContain('style={');
    expect(frontendIndexCssSource).toContain('.progress-fill-frame');
    expect(frontendIndexCssSource).toContain('@media (prefers-reduced-motion: reduce)');
  });

  it('keeps animated numeric readouts on the shared reduced-motion primitive', () => {
    expect(animatedNumberSource).toContain('useAnimatedNumberState');
    expect(animatedNumberSource).toContain('data-animated-number');
    expect(animatedNumberSource).not.toContain('requestAnimationFrame');
    expect(animatedNumberSource).not.toContain('createSignal');
    expect(animatedNumberStateSource).toContain('window.requestAnimationFrame');
    expect(animatedNumberStateSource).toContain('window.cancelAnimationFrame');
    expect(animatedNumberStateSource).toContain('REDUCED_MOTION_QUERY');
    expect(animatedNumberStateSource).toContain('activeFrameEntries');
    expect(animatedNumberModelSource).toContain('DEFAULT_ANIMATED_NUMBER_DURATION_MS');
    expect(animatedNumberModelSource).toContain('prefers-reduced-motion');
    expect(animatedNumberModelSource).toContain('easeAnimatedNumberProgress');
    expect(frontendIndexCssSource).toContain('.animated-number');
    expect(frontendIndexCssSource).toContain('font-variant-numeric: tabular-nums');
  });

  it('keeps search field on shell, runtime, and model owners', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
        forbiddenPatterns?: Array<{ path?: string; patterns?: string[] }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        scopes?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'search-field-shell');
    const registeredGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'search-control-native-search-input',
    );

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/SearchField.tsx');
    expect(registeredRule?.canonical?.export).toBe('SearchField');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/AI/Chat/AssistantCommandHelpDialog.tsx',
      'src/components/AI/Chat/index.tsx',
      'src/components/Settings/InfrastructureSourcePicker.tsx',
      'src/components/Settings/ResourcePicker.tsx',
      'src/components/Settings/UserAssignmentsPanel.tsx',
      'src/components/shared/AIModelPicker.tsx',
      'src/components/shared/CommandPaletteModal.tsx',
      'src/components/shared/SearchInput.tsx',
    ]);
    expect(registeredRule?.forbiddenPatterns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: 'src/components/Settings/ResourcePicker.tsx',
          patterns: ['type="text"', 'onInput={(e) => setTagFilter'],
        }),
      ]),
    );
    expect(registeredGuard?.canonical?.path).toBe('src/components/shared/SearchField.tsx');
    expect(registeredGuard?.canonical?.export).toBe('SearchField');
    expect(registeredGuard?.allPatterns).toEqual(['type="search"']);
    expect(registeredGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(registeredGuard?.ignoredPaths).toEqual([
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);
    expect(registeredGuard?.scopes).toEqual(
      expect.arrayContaining(['src/components', 'src/features', 'src/pages']),
    );

    expect(searchFieldSource).toContain('useSearchFieldState');
    expect(searchFieldSource).not.toContain('let inputEl: HTMLInputElement');
    expect(searchFieldSource).not.toContain(
      "if (props.hasTrailingControls) return 'pr-14 sm:pr-20'",
    );
    expect(searchFieldSource).not.toContain("if (e.key === 'Escape'");

    expect(searchFieldStateSource).toContain('export function useSearchFieldState');
    expect(searchFieldStateSource).toContain('let inputEl: HTMLInputElement');
    expect(searchFieldStateSource).toContain("if (event.key === 'Escape'");
    expect(searchFieldStateSource).toContain('inputEl?.blur()');
    expect(searchFieldStateSource).toContain('getSearchFieldInputPaddingRightClass');

    expect(searchFieldModelSource).toContain('shouldShowSearchFieldShortcutHint');
    expect(searchFieldModelSource).toContain('shouldShowSearchFieldClearButton');
    expect(searchFieldModelSource).toContain('getSearchFieldInputPaddingRightClass');
    expect(searchFieldModelSource).toContain("return 'pr-14 sm:pr-20'");
    expect(resourcePickerSource.match(/<SearchField/g) ?? []).toHaveLength(2);
    expect(resourcePickerSource).toContain('placeholder="Filter by tag..."');
    expect(resourcePickerSource).not.toContain('type="text"');
    expect(resourcePickerSource).not.toContain('onInput={(e) => setTagFilter');

    for (const source of [
      assistantCommandHelpDialogSource,
      aiChatSource,
      infrastructureSourcePickerSource,
      resourcePickerSource,
      userAssignmentsPanelSource,
      aiModelPickerSource,
      commandPaletteModalSource,
      searchInputSource,
    ]) {
      expect(source).toContain('SearchField');
      expect(source).not.toContain('type="search"');
    }
  });

  it('keeps search input on shell, runtime, and model owners', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
    };
    const registeredRule = registry.rules?.find((rule) => rule.id === 'search-input-shell');

    expect(registeredRule?.canonical?.path).toBe('src/components/shared/SearchInput.tsx');
    expect(registeredRule?.canonical?.export).toBe('SearchInput');
    expect(registeredRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual([
      'src/components/Docker/SwarmServicesDrawer.tsx',
      'src/components/Kubernetes/K8sDeploymentsDrawer.tsx',
      'src/components/Kubernetes/K8sNamespacesDrawer.tsx',
      'src/components/Settings/SettingsPageShell.tsx',
      'src/components/shared/FilterBar/FilterBar.tsx',
    ]);

    expect(searchInputSource).toContain('useSearchInputState');
    expect(searchInputSource).not.toContain('let searchInputEl: HTMLInputElement');
    expect(searchInputSource).not.toContain('useTypeToSearch');
    expect(searchInputSource).not.toContain('useSearchInputEnhancements');
    expect(searchInputSource).not.toContain(
      'enhancements.isSimple() ? props.shortcutHint : undefined',
    );

    expect(searchInputStateSource).toContain('export function useSearchInputState');
    expect(searchInputStateSource).toContain('let searchInputEl: HTMLInputElement');
    expect(searchInputStateSource).toContain('useTypeToSearch');
    expect(searchInputStateSource).toContain('useSearchInputEnhancements');
    expect(searchInputStateSource).toContain('getSearchInputShortcutHint');
    expect(searchInputStateSource).toContain('shouldSearchInputShowTrailingControls');

    expect(searchInputModelSource).toContain('getSearchInputShortcutHint');
    expect(searchInputModelSource).toContain('shouldSearchInputShowTrailingControls');
    expect(searchInputModelSource).toContain('export interface SearchInputProps');

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

    for (const source of [
      swarmServicesDrawerSource,
      k8sDeploymentsDrawerSource,
      k8sNamespacesDrawerSource,
      settingsPageShellSource,
      filterBarSource,
    ]) {
      expect(source).toContain('SearchInput');
    }
  });

  it('keeps search tips popover on shell, runtime, and model owners', () => {
    expect(searchTipsPopoverSource).toContain('useSearchTipsPopoverState');
    expect(searchTipsPopoverSource).toContain('getSearchTipsPopoverTriggerClass');
    expect(searchTipsPopoverSource).not.toContain('createSignal');
    expect(searchTipsPopoverSource).not.toContain('createEffect');
    expect(searchTipsPopoverSource).not.toContain('window.addEventListener');
    expect(searchTipsPopoverSource).not.toContain('triggerVariant ===');

    expect(searchTipsPopoverStateSource).toContain('export function useSearchTipsPopoverState');
    expect(searchTipsPopoverStateSource).toContain('createSignal');
    expect(searchTipsPopoverStateSource).toContain('createEffect');
    expect(searchTipsPopoverStateSource).toContain('window.addEventListener');
    expect(searchTipsPopoverStateSource).toContain('pointerInside');

    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverTriggerClass');
    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverPositionClass');
    expect(searchTipsPopoverModelSource).toContain('getSearchTipsPopoverTriggerVariant');
    expect(searchTipsPopoverModelSource).toContain('shouldSearchTipsPopoverOpenOnHover');
  });

  it('keeps dialog stack visibility in the shared dialog runtime', () => {
    expect(dialogStateSource).toContain('export function dialogStackHasBlockingDialog');
    expect(dialogStateSource).toContain('createSignal');
    expect(dialogStateSource).toContain('openDialogCount');
    expect(dialogStateSource).toContain('document.body.style.overflow');
  });

  it('keeps summary density control inside the shared summary primitives', () => {
    expect(summaryChartLayoutSource).toContain(
      "export const SUMMARY_CHART_SLOT_CLASS = 'h-[136px] sm:h-[150px]'",
    );
    expect(summaryChartLayoutSource).toContain(
      "export const SUMMARY_CHART_SLOT_COMPACT_CLASS = 'h-[108px] sm:h-[120px]'",
    );
    expect(summaryChartLayoutSource).toContain(
      "export const SUMMARY_CHART_PLOT_AREA_CLASS = 'h-[120px] sm:h-[134px]'",
    );
  });

  it('keeps tooltip on shell, runtime, and model owners', () => {
    expect(tooltipSource).toContain('useTooltipState');
    expect(tooltipSource).toContain('createTooltipSystemState');
    expect(tooltipSource).toContain('foreignObject');
    expect(tooltipSource).not.toContain('createSignal');
    expect(tooltipSource).not.toContain('requestAnimationFrame');
    expect(tooltipSource).not.toContain('sanitizeTooltipContent');
    expect(tooltipSource).not.toContain('resolveTooltipPosition');
    expect(tooltipSource).not.toContain('style={');

    expect(tooltipPortalSource).toContain('useTooltipPortalState');
    expect(tooltipPortalSource).toContain('foreignObject');
    expect(tooltipPortalSource).not.toContain('createSignal');
    expect(tooltipPortalSource).not.toContain('resolveTooltipPosition');
    expect(tooltipPortalSource).not.toContain('style={');

    expect(tooltipStateSource).toContain('export function useTooltipState');
    expect(tooltipStateSource).toContain('export function useTooltipPortalState');
    expect(tooltipStateSource).toContain('export function createTooltipSystemState');
    expect(tooltipStateSource).toContain('createSignal');
    expect(tooltipStateSource).toContain('requestAnimationFrame');
    expect(tooltipStateSource).toContain('tooltipInstance');
    expect(tooltipStateSource).toContain('resolveTooltipPosition');
    expect(tooltipStateSource).toContain('sanitizeTooltipContent');

    expect(tooltipModelSource).toContain('export function sanitizeTooltipContent');
    expect(tooltipModelSource).toContain('export function resolveTooltipPosition');
    expect(tooltipModelSource).toContain("export type TooltipAlignment = 'left' | 'center'");
    expect(tooltipModelSource).toContain("export type TooltipDirection = 'up' | 'down'");
  });

  it('keeps the chip-based FilterBar on a catalog descriptor with shell, chip, and add-menu owners', () => {
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        requiredConsumers?: Array<{ path?: string }>;
      }>;
      patternGuards?: Array<{
        id: string;
        canonical?: { path?: string; export?: string };
        allPatterns?: string[];
        allowedPaths?: string[];
        ignoredPaths?: string[];
      }>;
    };
    const filterBarRule = registry.rules?.find((rule) => rule.id === 'filter-bar-catalog-shell');
    const legacyPageControlsGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'filter-bar-consumer-legacy-page-controls',
    );
    const chipDotGuard = registry.patternGuards?.find(
      (guard) => guard.id === 'filter-chip-status-dot-local-factory',
    );

    expect(filterBarRule?.canonical?.path).toBe('src/components/shared/FilterBar/FilterBar.tsx');
    expect(filterBarRule?.canonical?.export).toBe('FilterBar');
    expect(filterBarRule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(
      expect.arrayContaining([
        'src/components/Alerts/ThresholdsTable.tsx',
        'src/components/Settings/AuditLogPanel.tsx',
        'src/components/Storage/StoragePageControls.tsx',
        'src/components/Workloads/WorkloadsFilter.tsx',
        'src/features/alerts/AlertHistoryFiltersCard.tsx',
        'src/features/platformPage/sharedPlatformPage.tsx',
        'src/features/proxmox/ProxmoxBackupsTable.tsx',
      ]),
    );
    expect(legacyPageControlsGuard?.canonical?.path).toBe(
      'src/components/shared/FilterBar/FilterBar.tsx',
    );
    expect(legacyPageControlsGuard?.canonical?.export).toBe('FilterBar');
    expect(legacyPageControlsGuard?.allPatterns).toEqual(['@/components/shared/PageControls']);
    expect(legacyPageControlsGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(chipDotGuard?.canonical?.path).toBe(
      'src/components/shared/FilterBar/filterOptionPresentation.tsx',
    );
    expect(chipDotGuard?.canonical?.export).toBe('filterChipStatusDot');
    expect(chipDotGuard?.allPatterns).toEqual(['h-2 w-2 rounded-full ' + '${className}']);
    expect(chipDotGuard?.allowedPaths ?? []).toHaveLength(0);
    expect(chipDotGuard?.ignoredPaths).toEqual([
      'src/components/shared/SharedPrimitives.guardrails.test.ts',
    ]);

    expect(filterBarIndexSource).toContain("export { FilterBar } from './FilterBar';");
    expect(filterBarIndexSource).toContain("export { FilterChip } from './FilterChip';");
    expect(filterBarIndexSource).toContain("export { AddFilterMenu } from './AddFilterMenu';");
    expect(filterBarIndexSource).toContain("export { SavedViewsMenu } from './SavedViewsMenu';");
    expect(filterBarIndexSource).toContain("export { useSavedViews } from './useSavedViews';");
    expect(filterBarIndexSource).toContain(
      "export { filterChipStatusDot } from './filterOptionPresentation';",
    );
    expect(filterBarIndexSource).toContain('isFilterSet');
    expect(filterBarIndexSource).toContain('clearFilter');
    expect(filterBarIndexSource).toContain('formatFilterChipValue');
    expect(filterBarOptionPresentationSource).toContain('export const filterChipStatusDot');
    expect(filterBarOptionPresentationSource).toContain('h-2 w-2 rounded-full');
    expect(filterBarOptionPresentationSource).toContain('aria-hidden="true"');

    expect(filterCatalogSource).toContain('export interface FilterDef');
    expect(filterCatalogSource).toContain('defaultValue: string');
    expect(filterCatalogSource).toContain(
      'export const isFilterSet = (filter: FilterDef): boolean =>',
    );
    expect(filterCatalogSource).toContain(
      'export const clearFilter = (filter: FilterDef): void =>',
    );
    expect(filterCatalogSource).toContain('filter.value() !== filter.defaultValue');

    expect(filterBarSource).toContain("import { Card } from '@/components/shared/Card';");
    expect(filterBarSource).toContain(
      "import { SearchInput } from '@/components/shared/SearchInput';",
    );
    expect(filterBarSource).toContain(
      "import { FilterActionButton, FilterMobileToggleButton } from '@/components/shared/FilterToolbar';",
    );
    expect(filterBarSource).toContain(
      "import { FilterButtonGroup } from '@/components/shared/FilterButtonGroup';",
    );
    expect(filterBarSource).toContain("import { AddFilterMenu } from './AddFilterMenu';");
    expect(filterBarSource).toContain("import { FilterChip } from './FilterChip';");
    expect(filterBarSource).toContain('props.filters.filter(isFilterSet)');
    expect(filterBarSource).toContain('activeCount() > 0');
    expect(filterBarSource).not.toContain('import { PageControls }');
    expect(filterBarSource).not.toContain('LabeledFilterSelect');
    expect(filterBarSource).not.toContain('LabeledFilterToggleGroup');
    expect(filterBarSource).not.toContain('filterControlsVariant');

    expect(filterChipSource).toContain('clearFilter,');
    expect(filterChipSource).toContain('formatFilterChipValue,');
    expect(filterChipSource).toContain("from './filterCatalog';");
    expect(filterChipSource).toContain('aria-haspopup="listbox"');
    expect(filterChipSource).toContain('aria-label={`Remove ${props.filter.label} filter`}');
    expect(filterChipSource).toContain('onClick={() => clearFilter(props.filter)}');
    expect(filterChipSource).toContain("event.key === 'Escape'");
    expect(filterChipSource).not.toContain('MutationObserver');
    // Chip popovers keep their own value search for already-active filters,
    // with activeIndex seeded on the currently-selected value so Enter without
    // typing is a no-op.
    expect(filterChipSource).toContain('aria-label={`Filter ${props.filter.label} values`}');
    expect(filterChipSource).toContain('filteredOptions');
    expect(filterChipSource).toContain('handleSearchKeyDown');
    expect(filterChipSource).toContain("event.key === 'ArrowDown'");
    expect(filterChipSource).toContain("event.key === 'ArrowUp'");
    expect(filterChipSource).toContain("event.key === 'Enter'");
    expect(filterChipSource).toContain('commitActive');
    expect(filterChipSource).toContain('queueMicrotask(() => searchInputRef?.focus())');

    expect(addFilterMenuSource).toContain('isFilterSet,');
    expect(addFilterMenuSource).toContain('type FilterDef,');
    expect(addFilterMenuSource).toContain('type FilterSelectOption,');
    expect(addFilterMenuSource).toContain("from './filterCatalog';");
    expect(addFilterMenuSource).toContain('availableFilters');
    expect(addFilterMenuSource).toContain('option.value !== filter.defaultValue');
    expect(addFilterMenuSource).toContain('GROUP_ORDER');
    expect(addFilterMenuSource).toContain('filterGroupClass');
    expect(addFilterMenuSource).toContain('filterSelectClass');
    expect(addFilterMenuSource).toContain('aria-label="Filter"');
    expect(addFilterMenuSource).not.toContain('LabeledFilterSelect');
    // Add-filter selection is intentionally direct: the shared primitive
    // exposes a single native select grouped by filter category instead of a
    // nested searchable filter picker, while active chips still own value
    // refinement after a filter has been applied.
    expect(addFilterMenuSource).toContain('selectableGroups');
    expect(addFilterMenuSource).toContain('selectableByToken');
    expect(addFilterMenuSource).toContain('<optgroup label={GROUP_LABELS[group.key]}>');
    expect(addFilterMenuSource).toContain('{filter.filter.label}: {option.option.label}');
    expect(addFilterMenuSource).toContain('selected.filter.setValue(selected.option.value)');
    expect(addFilterMenuSource).not.toContain('Search filters');
    expect(addFilterMenuSource).not.toContain('Search values');

    // Saved views: per-page named filter combos persist to
    // localStorage under `pulse:filterbar:saved-views:<key>`. The hook owns
    // storage IO + URL navigation; the menu owns the dropdown chrome.
    expect(useSavedViewsSource).toContain(
      "import { useLocation, useNavigate } from '@solidjs/router';",
    );
    expect(useSavedViewsSource).toContain("'pulse:filterbar:saved-views:'");
    expect(useSavedViewsSource).toContain('export interface SavedView');
    expect(useSavedViewsSource).toContain('saveCurrent');
    expect(useSavedViewsSource).toContain('removeView');
    expect(useSavedViewsSource).toContain('applyView');
    expect(useSavedViewsSource).toContain('navigate(path,');
    expect(useSavedViewsSource).toContain('writeStored');
    expect(useSavedViewsSource).toContain('readStored');
    // Hook is robust to malformed localStorage and SSR.
    expect(useSavedViewsSource).toContain("typeof window === 'undefined'");
    expect(useSavedViewsSource).toContain('JSON.parse');
    // saveCurrent must snapshot the full URL search string so saved views
    // capture every URL-synced state (active filter chips AND the page's
    // search query, which every page URL-syncs through `?q=...`). A future
    // refactor that narrows the snapshot to a curated subset would silently
    // strip search from saved views.
    expect(useSavedViewsSource).toContain("window.location.search.replace(/^\\?/, '')");

    expect(savedViewsMenuSource).toContain("from './useSavedViews';");
    expect(savedViewsMenuSource).toContain('aria-label="Saved views"');
    expect(savedViewsMenuSource).toContain('Save current view as...');
    expect(savedViewsMenuSource).toContain("event.key === 'Enter'");
    expect(savedViewsMenuSource).toContain('queueMicrotask(() => nameInputRef?.focus())');

    // FilterBar consumers — each migrated page should declare a catalog of
    // FilterDef entries rather than rendering the labelled-select row from
    // PageControls. Guards against regression to the old layout-per-page
    // pattern.

    // Recovery events surface — every advanced filter folds into the catalog
    // (scope, method, verification, cluster, node, namespace) so the
    // dedicated "Filter" popover panel and FilterToolbarPanel-based advanced
    // panel are no longer rendered.

    // WorkloadsFilter — Type/Status stay in the shared FilterBar catalog but
    // render as inline primary controls; Node/Platform/Namespace/Runtime
    // remain selector-backed catalog filters. The xl-breakpoint
    // segmented↔select swap retires here.
    expect(workloadsFilterSource).toContain("from '@/components/shared/FilterBar';");
    expect(workloadsFilterSource).toContain('FilterBar,');
    expect(workloadsFilterSource).toContain('filterChipStatusDot,');
    expect(workloadsFilterSource).toContain('type FilterDef,');
    expect(workloadsFilterSource).toContain('type FilterSelectOption,');
    expect(workloadsFilterSource).toContain('<FilterBar');
    expect(workloadsFilterSource).toContain("ariaLabel={props.ariaLabel ?? 'Workloads filters'}");
    expect(workloadsFilterSource).toContain("id: 'workloads-type'");
    expect(workloadsFilterSource).toContain("id: 'workloads-status'");
    expect(workloadsFilterSource).toContain('inline: true');
    expect(workloadsFilterSource).toContain('viewOptionsTrailing={');
    expect(workloadsFilterSource).toContain('GroupedTableModeSegmentedControl');
    expect(workloadsFilterSource).toContain('ChartVisibilityToggleButton');
    expect(workloadsFilterSource).toContain('<ColumnPicker');
    expect(workloadsFilterSource).toContain('onClearAll={handleClearAll}');
    expect(workloadsFilterSource).toContain('showClearAll={showClearAll}');
    expect(workloadsFilterSource).not.toContain('PageControls');
    expect(workloadsFilterSource).not.toContain('LabeledFilterToggleGroup');
    expect(workloadsFilterSource).not.toContain('LabeledFilterSelect');
    expect(workloadsFilterSource).not.toContain('useWorkloadsFilterState');
    expect(workloadsFilterSource).not.toContain('const statusDot = (className: string)');

    // StoragePageControls — Subtabs sit above the FilterBar; sort key/sort
    // direction live in viewOptionsTrailing as raw view-options (not chips).
    // Per-view catalog filters (inline Group by and Status plus menu-backed
    // Source on Pools; Role/Group on Physical Disks) flow through the FilterBar
    // catalog. The legacy
    // 3-layer indirection (StoragePageControls → StorageControls →
    // StorageFilter) collapses to one component.
    expect(storagePageControlsSource).toContain("from '@/components/shared/FilterBar';");
    expect(storagePageControlsSource).toContain('FilterBar,');
    expect(storagePageControlsSource).toContain('filterChipStatusDot,');
    expect(storagePageControlsSource).toContain('type FilterDef');
    expect(storagePageControlsSource).toContain('<FilterBar');
    expect(storagePageControlsSource).toContain('filterChipStatusDot');
    expect(storagePageControlsSource).toContain('<Subtabs');
    expect(storagePageControlsSource).toContain(
      "ariaLabel={props.filterAriaLabel ?? 'Storage filters'}",
    );
    expect(storagePageControlsSource).toContain("id: 'storage-node'");
    expect(storagePageControlsSource).toContain("id: 'storage-group-by'");
    expect(storagePageControlsSource).toContain("id: 'storage-source'");
    expect(storagePageControlsSource).toContain("id: 'storage-status'");
    expect(storagePageControlsSource).toContain("id: 'storage-disk-role'");
    expect(storagePageControlsSource).toContain("id: 'storage-disk-group'");
    expect(storagePageControlsSource).toContain('viewOptionsTrailing={');
    expect(storagePageControlsSource).toContain('aria-label="Sort by"');
    expect(storagePageControlsSource).toContain('aria-label="Sort direction"');
    expect(storagePageControlsSource).toContain('onClearAll={handleClearAll}');
    expect(storagePageControlsSource).toContain('showClearAll={showClearAll}');
    // The component name is StoragePageControls but it must not import or
    // render the legacy PageControls primitive, the StorageControls /
    // StorageFilter intermediates, or any LabeledFilterSelect /
    // FilterSegmentedControl forks.
    expect(storagePageControlsSource).not.toContain("from '@/components/shared/PageControls'");
    expect(storagePageControlsSource).not.toContain('<PageControls');
    expect(storagePageControlsSource).not.toContain('<StorageControls');
    expect(storagePageControlsSource).not.toContain('<StorageFilter');
    expect(storagePageControlsSource).not.toContain('LabeledFilterSelect');
    expect(storagePageControlsSource).not.toContain('FilterSegmentedControl');
    expect(storagePageControlsSource).not.toContain('storageStatusDot');
    // Storage's three-layer indirection retired — StoragePageControls no
    // longer imports the deleted StorageFilter / StorageControls modules,
    // and reads the canonical Storage types directly from storagePageState
    // and useStorageModel rather than re-exporting them from the deleted
    // shell.
    expect(storagePageControlsSource).not.toContain("from './StorageFilter'");
    expect(storagePageControlsSource).not.toContain("from './StorageControls'");
    expect(storagePageControlsSource).not.toContain('useStoragePageControlsModel');
    expect(storagePageControlsSource).not.toContain('useStorageControlsModel');
    expect(storagePageControlsSource).toContain('type StorageStatusFilterValue,');
    expect(storagePageControlsSource).toContain(
      "import type { StorageGroupKey, StorageSortKey } from './useStorageModel';",
    );
    expect(sharedPlatformPageSource).toContain('filterChipStatusDot');
    expect(sharedPlatformPageSource).not.toContain('platformChipStatusDot');
    for (const source of [
      proxmoxBackupsTableSharedSource,
      proxmoxCephTableSource,
      proxmoxReplicationTableSource,
    ]) {
      expect(source).toContain('filterChipStatusDot');
      expect(source).not.toContain('const statusDot = (className: string)');
    }
  });

  it('keeps shared navigation and row actions usable at phone widths', () => {
    expect(subtabsSource).toContain('overflow-x-auto');
    expect(subtabsSource).toContain('whitespace-nowrap');
    expect(subtabsSource).toContain('min-h-10');
    expect(summaryRowActionButtonSource).toContain('h-10 w-10');
    expect(searchFieldSource).toContain('min-h-10');
    expect(filterToolbarSource).toContain('min-h-10');
    expect(inlineDetailTableRowSource).toContain('max-w-[calc(100vw-3.5rem)]');
  });
});
