import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import calloutCardSource from '@/components/shared/CalloutCard.tsx?raw';
import aiModelPickerSource from '@/components/shared/AIModelPicker.tsx?raw';
import commandPaletteModalSource from '@/components/shared/CommandPaletteModal.tsx?raw';
import commandPaletteModelSource from '@/components/shared/commandPaletteModel.ts?raw';
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
import filterOptionPresentationSource from '@/components/shared/filterOptionPresentation.ts?raw';
import formSelectSource from '@/components/shared/FormSelect.tsx?raw';
import helpIconSource from '@/components/shared/HelpIcon.tsx?raw';
import helpIconModelSource from '@/components/shared/helpIconModel.ts?raw';
import historyChartHeaderSource from '@/components/shared/HistoryChartHeader.tsx?raw';
import historyChartOverlaySource from '@/components/shared/HistoryChartOverlay.tsx?raw';
import historyChartSource from '@/components/shared/HistoryChart.tsx?raw';
import historyChartModelSource from '@/components/shared/historyChartModel.ts?raw';
import historyChartTooltipSource from '@/components/shared/HistoryChartTooltip.tsx?raw';
import infrastructureDetailsDrawerSource from '@/components/shared/InfrastructureDetailsDrawer.tsx?raw';
import infrastructureDetailsDrawerModelSource from '@/components/shared/infrastructureDetailsDrawerModel.ts?raw';
import mobileNavBarSource from '@/components/shared/MobileNavBar.tsx?raw';
import mobileNavBarModelSource from '@/components/shared/mobileNavBarModel.ts?raw';
import infrastructureSelectorSource from '@/components/shared/InfrastructureSelector.tsx?raw';
import pulseDataGridSource from '@/components/shared/PulseDataGrid.tsx?raw';
import pulseDataGridModelSource from '@/components/shared/pulseDataGridModel.ts?raw';
import progressBarSource from '@/components/shared/ProgressBar.tsx?raw';
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
import subtabsSource from '@/components/shared/Subtabs.tsx?raw';
import toggleSource from '@/components/shared/Toggle.tsx?raw';
import toggleModelSource from '@/components/shared/toggleModel.ts?raw';
import searchTipsPopoverSource from '@/components/shared/SearchTipsPopover.tsx?raw';
import searchTipsPopoverModelSource from '@/components/shared/searchTipsPopoverModel.ts?raw';
import tooltipSource from '@/components/shared/Tooltip.tsx?raw';
import tooltipPortalSource from '@/components/shared/TooltipPortal.tsx?raw';
import tooltipModelSource from '@/components/shared/tooltipModel.ts?raw';
import upgradeLinkSource from '@/components/shared/UpgradeLink.tsx?raw';
import interactiveSparklineSource from '@/components/shared/InteractiveSparkline.tsx?raw';
import interactiveSparklineModelSource from '@/components/shared/interactiveSparklineModel.ts?raw';
import contextualFocusSource from '@/components/shared/contextualFocus.ts?raw';
import summaryCardInteractionSource from '@/components/shared/summaryCardInteraction.ts?raw';
import summaryJumpToRowButtonSource from '@/components/shared/SummaryJumpToRowButton.tsx?raw';
import summaryRowActionButtonSource from '@/components/shared/SummaryRowActionButton.tsx?raw';
import summaryInteractionA11ySource from '@/components/shared/summaryInteractionA11y.ts?raw';
import tableSource from '@/components/shared/Table.tsx?raw';
import tableCardHeaderSource from '@/components/shared/TableCardHeader.tsx?raw';
import summaryTableCardHeaderSource from '@/components/shared/SummaryTableCardHeader.tsx?raw';
import summaryTableFocusSource from '@/components/shared/summaryTableFocus.ts?raw';
import tableCardSource from '@/components/shared/TableCard.tsx?raw';
import groupedTableModeSegmentedControlSource from '@/components/shared/GroupedTableModeSegmentedControl.tsx?raw';
import groupedTableRowPresentationSource from '@/components/shared/groupedTableRowPresentation.ts?raw';
import workloadsFilterSource from '@/components/Workloads/WorkloadsFilter.tsx?raw';
import infrastructurePageSurfaceSource from '@/features/infrastructure/InfrastructurePageSurface.tsx?raw';
import infrastructureSourceManagerSource from '@/components/Settings/InfrastructureSourceManager.tsx?raw';
import configuredNodeTablesSource from '@/components/Settings/ConfiguredNodeTables.tsx?raw';
import infrastructureSummaryTableSource from '@/components/shared/InfrastructureSummaryTable.tsx?raw';
import infrastructureSummaryTableRowSource from '@/components/shared/InfrastructureSummaryTableRow.tsx?raw';
import infrastructureSelectorModelSource from '@/components/shared/infrastructureSelectorModel.ts?raw';
import infrastructureSummaryTableModelSource from '@/components/shared/infrastructureSummaryTableModel.ts?raw';
import infrastructureSummaryTableStateSource from '@/components/shared/useInfrastructureSummaryTableState.ts?raw';
import monitoredSystemLimitWarningBannerSource from '@/components/shared/MonitoredSystemLimitWarningBanner.tsx?raw';
import monitoredSystemLimitWarningBannerModelSource from '@/components/shared/monitoredSystemLimitWarningBannerModel.ts?raw';
import selectionCardGroupSource from '@/components/shared/SelectionCardGroup.tsx?raw';
import selectionCardGroupModelSource from '@/components/shared/selectionCardGroupModel.ts?raw';
import summaryMetricCardSource from '@/components/shared/SummaryMetricCard.tsx?raw';
import summaryPanelSource from '@/components/shared/SummaryPanel.tsx?raw';
import summarySynchronizedReadoutSource from '@/components/shared/SummarySynchronizedReadout.tsx?raw';
import tagBadgesSource from '@/components/shared/TagBadges.tsx?raw';
import tlsVerificationWarningBannerSource from '@/components/shared/TlsVerificationWarningBanner.tsx?raw';
import commandPaletteStateSource from '@/components/shared/useCommandPaletteState.ts?raw';
import columnPickerStateSource from '@/components/shared/useColumnPickerState.ts?raw';
import tagInputStateSource from '@/components/shared/useTagInputState.ts?raw';
import collapsibleSearchInputStateSource from '@/components/shared/useCollapsibleSearchInputState.ts?raw';
import containerUpdateButtonStateSource from '@/components/shared/useContainerUpdateButtonState.ts?raw';
import densityMapStateSource from '@/components/shared/useDensityMapState.ts?raw';
import dialogStateSource from '@/components/shared/useDialogState.ts?raw';
import filterButtonGroupStateSource from '@/components/shared/useFilterButtonGroupState.ts?raw';
import helpIconStateSource from '@/components/shared/useHelpIconState.ts?raw';
import historyChartStateSource from '@/components/shared/useHistoryChartState.ts?raw';
import infrastructureDetailsDrawerStateSource from '@/components/shared/useInfrastructureDetailsDrawerState.ts?raw';
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
import upgradeNavigationHookSource from '@/components/shared/useUpgradeNavigation.ts?raw';
import interactiveSparklineStateSource from '@/components/shared/useInteractiveSparklineState.ts?raw';
import monitoredSystemLimitWarningBannerStateSource from '@/components/shared/useMonitoredSystemLimitWarningBannerState.ts?raw';
import selectionCardGroupStateSource from '@/components/shared/useSelectionCardGroupState.ts?raw';
import webInterfaceUrlFieldSource from '@/components/shared/WebInterfaceUrlField.tsx?raw';
import webInterfaceUrlFieldModelSource from '@/components/shared/webInterfaceUrlFieldModel.ts?raw';
import webInterfaceUrlFieldStateSource from '@/components/shared/useWebInterfaceUrlFieldState.ts?raw';
import upgradeNavigationSource from '@/utils/upgradeNavigation.ts?raw';
import guestRowSource from '@/components/Workloads/GuestRow.tsx?raw';
import workloadsTableSource from '@/components/Workloads/WorkloadsTable.tsx?raw';
import workloadPanelSource from '@/components/Workloads/WorkloadPanel.tsx?raw';
import guestRowStateSource from '@/components/Workloads/useGuestRowState.ts?raw';
import workloadSelectionStateSource from '@/components/Workloads/useWorkloadSelectionState.ts?raw';
import infrastructureSummarySource from '@/components/Infrastructure/InfrastructureSummary.tsx?raw';
import infrastructureSummaryStateSource from '@/components/Infrastructure/useInfrastructureSummaryState.ts?raw';
import unifiedResourceHostTableCardSource from '@/components/Infrastructure/UnifiedResourceHostTableCard.tsx?raw';
import unifiedResourceServiceInfrastructureCardSource from '@/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx?raw';
import unifiedResourcePBSTableSectionSource from '@/components/Infrastructure/UnifiedResourcePBSTableSection.tsx?raw';
import unifiedResourcePMGTableSectionSource from '@/components/Infrastructure/UnifiedResourcePMGTableSection.tsx?raw';
import pmgInstanceDrawerSource from '@/components/PMG/PMGInstanceDrawer.tsx?raw';
import swarmServicesDrawerSource from '@/components/Docker/SwarmServicesDrawer.tsx?raw';
import k8sDeploymentsDrawerSource from '@/components/Kubernetes/K8sDeploymentsDrawer.tsx?raw';
import k8sNamespacesDrawerSource from '@/components/Kubernetes/K8sNamespacesDrawer.tsx?raw';
import nodeGroupHeaderSource from '@/components/shared/NodeGroupHeader.tsx?raw';
import storageGroupRowSource from '@/components/Storage/StorageGroupRow.tsx?raw';
import storageGroupPresentationSource from '@/features/storageBackups/groupPresentation.ts?raw';
import storagePoolRowSource from '@/components/Storage/StoragePoolRow.tsx?raw';
import storageContentCardSource from '@/components/Storage/StorageContentCard.tsx?raw';
import storagePoolsTableSource from '@/components/Storage/StoragePoolsTable.tsx?raw';
import diskListSource from '@/components/Storage/DiskList.tsx?raw';
import storageSummarySource from '@/components/Storage/StorageSummary.tsx?raw';
import workloadsSummarySource from '@/components/Workloads/WorkloadsSummary.tsx?raw';
import recoveryComponentSource from '@/components/Recovery/Recovery.tsx?raw';
import recoveryHistorySectionSource from '@/components/Recovery/RecoveryHistorySection.tsx?raw';
import recoveryHistoryTableSource from '@/components/Recovery/RecoveryHistoryTable.tsx?raw';
import recoveryProtectedInventorySectionSource from '@/components/Recovery/RecoveryProtectedInventorySection.tsx?raw';
import recoveryTablePresentationSource from '@/utils/recoveryTablePresentation.ts?raw';
import alertHistoryTableSectionSource from '@/features/alerts/AlertHistoryTableSection.tsx?raw';
import alertHistoryTableGroupRowSource from '@/features/alerts/AlertHistoryTableGroupRow.tsx?raw';
import alertResourceTableDesktopSource from '@/components/Alerts/AlertResourceTableDesktop.tsx?raw';
import aiCostDashboardSource from '@/components/AI/AICostDashboard.tsx?raw';
import deployCandidatesStepSource from '@/components/Infrastructure/deploy/CandidatesStep.tsx?raw';
import deployConfirmStepSource from '@/components/Infrastructure/deploy/ConfirmStep.tsx?raw';
import deployDeployingStepSource from '@/components/Infrastructure/deploy/DeployingStep.tsx?raw';
import deployPreflightStepSource from '@/components/Infrastructure/deploy/PreflightStep.tsx?raw';
import deployResultsStepSource from '@/components/Infrastructure/deploy/ResultsStep.tsx?raw';
import cephPageSource from '@/pages/Ceph.tsx?raw';
import pmgMailGatewaySource from '@/components/PMG/MailGateway.tsx?raw';
import pmgInstancePanelSource from '@/components/PMG/PMGInstancePanel.tsx?raw';
import resourceDetailDrawerOverviewSource from '@/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx?raw';
import aiSettingsDialogsSource from '@/components/Settings/AISettingsDialogs.tsx?raw';
import agentProfilesPanelSource from '@/components/Settings/AgentProfilesPanel.tsx?raw';
import apiTokenManagerSource from '@/components/Settings/APITokenManager.tsx?raw';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';
import organizationAccessMembersSectionSource from '@/components/Settings/OrganizationAccessMembersSection.tsx?raw';
import organizationIncomingSharesSectionSource from '@/components/Settings/OrganizationIncomingSharesSection.tsx?raw';
import organizationOutgoingSharesSectionSource from '@/components/Settings/OrganizationOutgoingSharesSection.tsx?raw';
import organizationOverviewMembersSectionSource from '@/components/Settings/OrganizationOverviewMembersSection.tsx?raw';
import reportingPanelSource from '@/components/Settings/ReportingPanel.tsx?raw';
import rolesPanelSource from '@/components/Settings/RolesPanel.tsx?raw';
import updatesSettingsPanelSource from '@/components/Settings/UpdatesSettingsPanel.tsx?raw';
import userAssignmentsPanelSource from '@/components/Settings/UserAssignmentsPanel.tsx?raw';
import filterBarSource from '@/components/shared/FilterBar/FilterBar.tsx?raw';
import filterChipSource from '@/components/shared/FilterBar/FilterChip.tsx?raw';
import addFilterMenuSource from '@/components/shared/FilterBar/AddFilterMenu.tsx?raw';
import filterCatalogSource from '@/components/shared/FilterBar/filterCatalog.ts?raw';
import filterBarIndexSource from '@/components/shared/FilterBar/index.ts?raw';
import savedViewsMenuSource from '@/components/shared/FilterBar/SavedViewsMenu.tsx?raw';
import useSavedViewsSource from '@/components/shared/FilterBar/useSavedViews.ts?raw';
import storagePageControlsSource from '@/components/Storage/StoragePageControls.tsx?raw';

const sharedSources = import.meta.glob(['./*.tsx', './cards/*.tsx', './responsive/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;
const frontendIndexCssSource = readFileSync(join(process.cwd(), 'src/index.css'), 'utf8');

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

    expect(rawTableUsers).toEqual([
      './InfrastructureSummaryTable.tsx',
      './InfrastructureSummaryTableRow.tsx',
      './PulseDataGrid.tsx',
    ]);
  });

  it('routes canonical settings segmented selectors through FilterButtonGroup', () => {
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
    expect(filterButtonGroupModelSource).toContain('touch-scroll');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupButtonClass');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupCompactLabel');
    expect(generalSettingsPanelSource).toContain('FilterButtonGroup');
    expect(generalSettingsPanelSource.match(/<FilterButtonGroup/g) ?? []).toHaveLength(3);
    expect(generalSettingsPanelSource).toContain('variant="prominent"');
    expect(generalSettingsPanelSource).not.toContain("props.themePreference() === 'light'");
    expect(generalSettingsPanelSource).not.toContain("temperatureStore.unit() === 'celsius'");
    expect(generalSettingsPanelSource).not.toContain(
      'props.pvePollingSelection() === option.value',
    );
    expect(reportingPanelSource.match(/<FilterButtonGroup/g) ?? []).toHaveLength(2);
    expect(reportingPanelSource).toContain('variant="prominent"');
    expect(reportingPanelSource).not.toContain('getReportingToggleButtonClass');
    expect(reportingPanelSource).not.toContain('<For each={REPORTING_RANGE_OPTIONS}>');
  });

  it('keeps native form selects on the shared labelled primitive', () => {
    expect(formSelectSource).toContain("from '@/components/shared/Form'");
    expect(formSelectSource).toContain('createUniqueId');
    expect(formSelectSource).toContain('splitProps');
    expect(formSelectSource).toContain('interface FormSelectProps');
    expect(formSelectSource).toContain('label: JSX.Element');
    expect(formSelectSource).toContain('<label for={selectId()}');
    expect(formSelectSource).toContain('<select');
    expect(formSelectSource).toContain('id={selectId()}');
    expect(formSelectSource).toContain('aria-describedby={describedBy()}');
    expect(formSelectSource).toContain('local.selectBaseClass ?? formSelect');
    expect(formSelectSource).toContain('local.fieldBaseClass ?? formField');
  });

  it('routes selectable settings cards through SelectionCardGroup', () => {
    expect(selectionCardGroupSource).toContain('useSelectionCardGroupState');
    expect(selectionCardGroupSource).toContain('getSelectionCardGroupClass');
    expect(selectionCardGroupSource).toContain('getSelectionCardButtonClass');
    expect(selectionCardGroupSource).toContain('getSelectionCardTitleClass');
    expect(selectionCardGroupSource).not.toContain('resolveSelectionCardTone');
    expect(selectionCardGroupSource).not.toContain('props.onChange(option.value)');
    expect(selectionCardGroupStateSource).toContain('export function useSelectionCardGroupState');
    expect(selectionCardGroupStateSource).toContain('createMemo');
    expect(selectionCardGroupStateSource).toContain('resolveSelectionCardTone');
    expect(selectionCardGroupStateSource).toContain('props.onChange(option.value)');
    expect(selectionCardGroupModelSource).toContain('resolveSelectionCardGroupVariant');
    expect(selectionCardGroupModelSource).toContain('resolveSelectionCardTone');
    expect(selectionCardGroupModelSource).toContain('getSelectionCardButtonClass');
    expect(selectionCardGroupModelSource).toContain("compact: 'grid grid-cols-2 gap-2'");
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
    expect(aiModelPickerSource).toContain('return props.models.filter((model) => !model.notable');
    expect(aiModelPickerSource).toContain('Show ${hiddenModelCount()} older models');
    expect(aiModelPickerSource).toContain("if (!candidate.includes(':'))");
    expect(aiModelPickerSource).toContain('MOBILE_BOTTOM_CLEARANCE');
    expect(aiModelPickerSource).toContain('availableHeight = window.innerHeight');
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
    expect(commandPaletteStateSource).toContain('buildInfrastructurePath');
    expect(commandPaletteStateSource).toContain('export function useCommandPaletteState');

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
    expect(interactiveSparklineSource).toContain('TooltipPortal');
    expect(interactiveSparklineSource).toContain('text-base-content');
    expect(interactiveSparklineSource).not.toContain("'background-color': 'rgb(15, 23, 42)'");
  });

  it('keeps shared table card chrome on one canonical header owner', () => {
    expect(tableCardHeaderSource).toContain('Clear selection');
    expect(tableCardHeaderSource).toContain("props.clearLabel ?? 'Clear'");
    expect(tableCardHeaderSource).toContain('props.clearAriaLabel ??');
    expect(tableCardHeaderSource).toContain('event.stopPropagation()');
    expect(tableCardHeaderSource).toContain('TABLE_CARD_HEADER_CLASS');
    expect(tableCardHeaderSource).not.toContain('Pinned to');
    expect(tableCardHeaderSource).not.toContain('Scoped to');
    expect(summaryTableCardHeaderSource).toContain("from './TableCardHeader'");
    expect(summaryTableCardHeaderSource).not.toContain('border-b border-border bg-surface-hover');
    for (const source of [
      workloadsTableSource,
      unifiedResourceHostTableCardSource,
      unifiedResourceServiceInfrastructureCardSource,
      storageContentCardSource,
      recoveryHistorySectionSource,
      recoveryProtectedInventorySectionSource,
    ]) {
      expect(source).toContain('TableCardHeader');
      expect(source).not.toContain('SummaryTableCardHeader');
    }
    expect(storagePoolRowSource).not.toContain('Clear selection');
  });

  it('keeps framed product table surfaces on the shared TableCard owner', () => {
    expect(tableCardSource).toContain("export const TABLE_CARD_FRAME_CLASS = 'overflow-hidden'");
    expect(tableCardSource).toContain("Omit<CardProps, 'border' | 'padding' | 'tone'>");
    expect(tableCardSource).toContain('border={true}');
    expect(tableCardSource).toContain('padding="none"');
    expect(tableCardSource).toContain('tone="card"');
    expect(tableCardSource).toContain('<Card');

    for (const source of [
      workloadsTableSource,
      unifiedResourceHostTableCardSource,
      unifiedResourceServiceInfrastructureCardSource,
      storageContentCardSource,
      recoveryComponentSource,
      recoveryHistorySectionSource,
      recoveryProtectedInventorySectionSource,
    ]) {
      expect(source).toContain('TableCard');
      expect(source).not.toContain('overflow-hidden border-border-subtle bg-surface');
    }
  });

  it('keeps shared subtabs as one primitive and leaves shell styling to owning surfaces', () => {
    expect(subtabsSource).not.toContain("variant?: 'default' | 'control'");
    expect(subtabsSource).not.toContain('subtabsControlShellClass');
    expect(subtabsSource).toContain('subtabsShellClass');
    expect(subtabsSource).toContain('subtabsListClass');
    expect(subtabsSource).toContain('subtabButtonClass');
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
    expect(interactiveSparklineModelSource).toContain('onHoverSyncChange');
    expect(densityMapModelSource).toContain('onHoverSyncChange');

    expect(workloadSelectionStateSource).toContain('preserveScrollableAncestorVerticalOffset');
    expect(workloadSelectionStateSource).toContain('hoveredWorkloadGroupScope');
    expect(workloadSelectionStateSource).toContain('activeSummaryWorkloadGroupScope');
    expect(workloadSelectionStateSource).not.toContain('const scrollTop = scroller?.scrollTop');

    expect(summaryJumpToRowButtonSource).toContain('<span>Jump to row</span>');
    expect(summaryJumpToRowButtonSource).toContain('props.onClick');
    expect(summaryJumpToRowButtonSource).not.toContain('querySelector');
    expect(summaryJumpToRowButtonSource).not.toContain('scrollIntoView');

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

    expect(infrastructureSummaryStateSource).toContain('useSummaryContextualFocusState');
    expect(infrastructureSummaryStateSource).toContain('chartHoverSync');
    expect(infrastructureSummaryStateSource).toContain('hoveredGroupScope');
    expect(infrastructureSummaryStateSource).toContain('filterSeriesForActiveScope');
    expect(infrastructureSummaryStateSource).not.toContain(
      'const interactiveResourceIds = createMemo',
    );

    expect(storageSummarySource).toContain('useSummaryContextualFocusState');
    expect(storageSummarySource).toContain('chartHoverSync');
    expect(storageSummarySource).not.toContain('resolveSummaryActiveSeriesId');

    expect(workloadsSummarySource).toContain('useSummaryContextualFocusState');
    expect(workloadsSummarySource).toContain('chartHoverSync');
    expect(workloadsSummarySource).toContain('hoveredGroupScope');
    expect(workloadsSummarySource).toContain('filterSeriesForActiveScope');
    expect(workloadsSummarySource).not.toContain('const interactiveWorkloadIds = createMemo');
  });

  it('keeps synchronized summary values on one shared card/readout contract', () => {
    expect(summaryMetricCardSource).toContain('headerValue?: JSX.Element');
    expect(summaryMetricCardSource).toContain("bodyLayout?: 'chart' | 'auto'");
    expect(summaryMetricCardSource).toContain('props.headerValue');
    expect(summaryMetricCardSource).not.toContain('data-summary-sync-readout');

    expect(summarySynchronizedReadoutSource).toContain('export const SummarySynchronizedReadout');
    expect(summarySynchronizedReadoutSource).toContain('data-summary-sync-readout="true"');
    expect(summarySynchronizedReadoutSource).toContain('formatSummarySynchronizedReadoutTime');
    expect(summarySynchronizedReadoutSource).not.toContain('Portal');

    expect(interactiveSparklineModelSource).toContain(
      'buildInteractiveSparklineSynchronizedReadout',
    );
    expect(densityMapModelSource).toContain('buildDensityMapSynchronizedReadout');

    expect(infrastructureSummarySource).toContain('SummarySynchronizedReadout');
    expect(infrastructureSummarySource).toContain('headerValue={');
    expect(workloadsSummarySource).toContain('SummarySynchronizedReadout');
    expect(workloadsSummarySource).toContain('headerValue={');
    expect(storageSummarySource).toContain('SummarySynchronizedReadout');
    expect(storageSummarySource).toContain('headerValue={');
  });

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
    expect(recoveryTablePresentationSource).toContain('getGroupedTableRowCellClass');
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
      recoveryHistoryTableSource,
      recoveryProtectedInventorySectionSource,
      alertHistoryTableSectionSource,
      workloadsTableSource,
      storagePoolsTableSource,
      diskListSource,
      infrastructureSourceManagerSource,
      configuredNodeTablesSource,
      aiCostDashboardSource,
      cephPageSource,
      deployCandidatesStepSource,
      deployConfirmStepSource,
      deployDeployingStepSource,
      deployPreflightStepSource,
      deployResultsStepSource,
      pmgInstanceDrawerSource,
      pmgInstancePanelSource,
      pmgMailGatewaySource,
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
      recoveryHistorySectionSource,
      recoveryProtectedInventorySectionSource,
      alertHistoryTableSectionSource,
      workloadsTableSource,
      storageContentCardSource,
    ]) {
      expect(source).toContain('<TableCard');
    }

    for (const source of [
      unifiedResourceHostTableCardSource,
      recoveryHistorySectionSource,
      recoveryProtectedInventorySectionSource,
      workloadsTableSource,
      storageContentCardSource,
    ]) {
      expect(source).toContain('TableCardHeader');
    }

    expect(alertHistoryTableSectionSource).not.toContain(
      'overflow-hidden rounded border border-border',
    );
    expect(storagePoolsTableSource).not.toContain('STORAGE_POOLS_SCROLL_WRAP_CLASS');
    expect(diskListSource).not.toContain("from '@/components/shared/Card'");
    expect(diskListSource).not.toContain('PHYSICAL_DISK_TABLE_SCROLL_CLASS');
    expect(recoveryProtectedInventorySectionSource).toContain('wrapperClass="bg-surface"');
    expect(configuredNodeTablesSource).toContain('wrapperClass="max-h-[600px] overflow-y-auto"');
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

    for (const source of [
      deployCandidatesStepSource,
      deployConfirmStepSource,
      deployDeployingStepSource,
      deployPreflightStepSource,
      deployResultsStepSource,
    ]) {
      expect(source).toContain("from '@/components/shared/Table'");
      expect(source).not.toContain('<table');
      expect(source).not.toContain('<thead');
      expect(source).not.toContain('<tbody');
      expect(source).not.toContain('<tr ');
      expect(source).not.toContain('<td ');
      expect(source).not.toContain('<th ');
    }
  });

  it('keeps grouped/list table-mode controls on one shared presentation contract', () => {
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

    for (const source of [workloadsFilterSource, infrastructurePageSurfaceSource]) {
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

    expect(nodeGroupHeaderSource).toContain('event.stopPropagation()');
  });

  it('keeps upgrade navigation split across shell, runtime, and utility owners', () => {
    expect(upgradeLinkSource).toContain('destination.external');
    expect(upgradeLinkSource).toContain("return local.target ?? '_blank';");
    expect(upgradeLinkSource).toContain('target={target()}');
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

  it('keeps column picker on shell, runtime, and model owners', () => {
    expect(columnPickerSource).toContain('useColumnPickerState');
    expect(columnPickerSource).toContain('COLUMN_PICKER_PANEL_TITLE');
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
    expect(columnPickerModelSource).toContain('COLUMN_PICKER_PANEL_TITLE');
    expect(columnPickerModelSource).toContain('getHiddenColumnCount');
    expect(columnPickerModelSource).toContain('shouldShowColumnPickerReset');
    expect(columnPickerModelSource).toContain('getColumnPickerOptionTextClass');
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

  it('keeps scroll-to-top button on shell, runtime, and model owners', () => {
    expect(scrollToTopButtonSource).toContain('useScrollToTopButtonState');
    expect(scrollToTopButtonSource).toContain('SCROLL_TO_TOP_BUTTON_ARIA_LABEL');
    expect(scrollToTopButtonSource).toContain('getScrollToTopButtonClass');
    expect(scrollToTopButtonSource).not.toContain('createSignal');
    expect(scrollToTopButtonSource).not.toContain('onMount');
    expect(scrollToTopButtonSource).not.toContain('scrollHeight');
    expect(scrollToTopButtonSource).not.toContain('SCROLL_THRESHOLD');

    expect(scrollToTopButtonStateSource).toContain('export function useScrollToTopButtonState');
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
  });

  it('keeps toggle on shell, runtime, and model owners', () => {
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

  it('routes settings info callouts through CalloutCard', () => {
    expect(calloutCardSource).toContain(
      "type CalloutTone = 'danger' | 'info' | 'success' | 'warning'",
    );
    expect(reportingPanelSource).toContain('CalloutCard');
    expect(reportingPanelSource).not.toContain('rounded-md border border-blue-200 bg-blue-50 p-6');
  });

  it('keeps TLS verification warnings in the shared primitive boundary', () => {
    expect(tlsVerificationWarningBannerSource).toContain('role="alert"');
    expect(tlsVerificationWarningBannerSource).toContain('TLS verification disabled.');
    expect(tlsVerificationWarningBannerSource).toContain('controlled lab environments');
    expect(tlsVerificationWarningBannerSource).toContain('Install a trusted certificate');
    expect(tlsVerificationWarningBannerSource).not.toContain('CalloutCard');
  });

  it('keeps shared fleet limit banner copy on the monitored-system commercial term', () => {
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      '@/utils/monitoredSystemPresentation',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'formatMonitoredSystemLimitSummary',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'formatMonitoredSystemMigrationMessage',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemLimitInstallCollectorsLabel',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('Monitored systems:');
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('monitored-system cap');
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('Install v6 collectors');
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('v6 Unified Agents:');
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain(
      'do not count toward Unified Agents.',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('Install v6 Unified Agents');
  });

  it('keeps monitored system limit warning banner on shell, runtime, and model owners', () => {
    expect(monitoredSystemLimitWarningBannerSource).toContain(
      'useMonitoredSystemLimitWarningBannerState',
    );
    expect(monitoredSystemLimitWarningBannerSource).toContain('UpgradeLink');
    expect(monitoredSystemLimitWarningBannerSource).toContain(
      'MONITORED_SYSTEM_LIMIT_REVIEW_POLICY_LABEL',
    );
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('loadRuntimeCapabilities');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('legacyConnections()');

    expect(monitoredSystemLimitWarningBannerStateSource).toContain(
      'export function useMonitoredSystemLimitWarningBannerState',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('loadRuntimeCapabilities');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain(
      'getRuntimeMonitoredSystemCapacity',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('isHostedModeEnabled');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain(
      'presentationPolicyHidesCommercialSurfaces',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).toContain(
      'presentationPolicyHidesUpgradePrompts',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('hasMigrationGap');
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain(
      'scopeSelfHostedBillingDestination',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain(
      'SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain(
      'SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT',
    );
    expect(monitoredSystemLimitWarningBannerStateSource).toContain('reviewPolicyDestination');
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain('handleUpgradeClick');
    expect(monitoredSystemLimitWarningBannerStateSource).not.toContain("fetch('/api/health'");

    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemBannerToneClass',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain(
      'getMonitoredSystemLimitUpgradeLabel',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemLimitInstallCollectorsLabel',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'type MonitoredSystemCapacityStatus',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'isMonitoredSystemLimitUsageAvailable',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'formatMonitoredSystemLimitSummary',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain(
      'current_available !== false',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('current / limit');
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain('0 remaining');
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'SELF_HOSTED_PRO_BILLING_USAGE_HREF',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).not.toContain(
      'SELF_HOSTED_PRO_BILLING_PLAN_HREF',
    );
  });

  it('keeps shared tag badges in the shared primitive boundary', () => {
    expect(tagBadgesSource).toContain("from '@/components/shared/Tooltip'");
    expect(guestRowSource).toContain("from '@/components/shared/TagBadges'");
    expect(guestRowSource).not.toContain("from './TagBadges'");
    expect(resourceDetailDrawerOverviewSource).toContain("from '@/components/shared/TagBadges'");
    expect(resourceDetailDrawerOverviewSource).not.toContain(
      "from '@/components/Workloads/TagBadges'",
    );
  });

  it('keeps infrastructure summary table on shell, runtime, and model owners', () => {
    expect(infrastructureSummaryTableSource).toContain('useInfrastructureSummaryTableState');
    expect(infrastructureSummaryTableSource).toContain('InfrastructureSummaryTableRow');
    expect(infrastructureSummaryTableSource).not.toContain('useWebSocket');
    expect(infrastructureSummaryTableSource).not.toContain('useAlertsActivation');
    expect(infrastructureSummaryTableSource).not.toContain('createSignal');
    expect(infrastructureSummaryTableSource).not.toContain('getNormalizedIdentityLookupVariants');
    expect(infrastructureSummaryTableSource).not.toContain('getAgentLikeIdentityAliases');

    expect(infrastructureSummaryTableStateSource).toContain('useWebSocket');
    expect(infrastructureSummaryTableStateSource).toContain('useAlertsActivation');
    expect(infrastructureSummaryTableStateSource).toContain(
      'export function useInfrastructureSummaryTableState',
    );
    expect(infrastructureSummaryTableStateSource).toContain('createSignal');

    expect(infrastructureSummaryTableModelSource).toContain('getNormalizedIdentityLookupVariants');
    expect(infrastructureSummaryTableModelSource).toContain('getAgentLikeIdentityAliases');
    expect(infrastructureSummaryTableModelSource).toContain(
      'export const resolveInfrastructureSummaryLinkedAgent',
    );
    expect(infrastructureSummaryTableModelSource).toContain(
      'export const sortInfrastructureSummaryItems',
    );

    expect(infrastructureSummaryTableRowSource).toContain('InfrastructureDetailsDrawer');
    expect(infrastructureSummaryTableRowSource).toContain('getAlertStyles');
    expect(infrastructureSummaryTableRowSource).not.toContain('useWebSocket');
    expect(infrastructureSummaryTableRowSource).not.toContain('useAlertsActivation');
  });

  it('keeps infrastructure selector on shell, runtime, and model owners', () => {
    expect(infrastructureSelectorSource).toContain('useInfrastructureSelectorState');
    expect(infrastructureSelectorSource).toContain('InfrastructureSummaryTable');
    expect(infrastructureSelectorSource).not.toContain('useResources');
    expect(infrastructureSelectorSource).not.toContain('createSignal');
    expect(infrastructureSelectorSource).not.toContain("resource.type === 'truenas'");

    expect(infrastructureSelectorStateSource).toContain('useUnifiedResources');
    expect(infrastructureSelectorStateSource).toContain('enabled: showNodeSummary');
    expect(infrastructureSelectorStateSource).toContain('useRecoveryRollups');
    expect(infrastructureSelectorStateSource).toContain('createSignal');
    expect(infrastructureSelectorStateSource).toContain('document.addEventListener');
    expect(infrastructureSelectorStateSource).toContain(
      'export function useInfrastructureSelectorState',
    );

    expect(infrastructureSelectorModelSource).toContain('buildInfrastructureSelectorAgents');
    expect(infrastructureSelectorModelSource).toContain('buildInfrastructureSelectorBackupCounts');
    expect(infrastructureSelectorModelSource).toContain('buildInfrastructureSelectorUnifiedNodes');
    expect(infrastructureSelectorModelSource).toContain('isAgentFacetInfrastructureResource');
  });

  it('keeps interactive sparkline on shell, runtime, and model owners', () => {
    expect(interactiveSparklineSource).toContain('useInteractiveSparklineState');
    expect(interactiveSparklineSource).toContain('data-active-series-display');
    expect(interactiveSparklineSource).toContain('data-active-hover-cursor-x');
    expect(interactiveSparklineSource).toContain('data-sparkline-tooltip="true"');
    expect(interactiveSparklineSource).toContain('TooltipPortal');
    expect(interactiveSparklineSource).toContain('data-sparkline-y-axis="true"');
    expect(interactiveSparklineSource).toContain('data-sparkline-x-axis="true"');
    expect(interactiveSparklineSource).toContain('axisPositionPercent(tick.y, sparkline.vbH)');
    expect(interactiveSparklineSource).toContain('axisPositionPercent(tick.x, sparkline.vbW)');
    expect(interactiveSparklineSource).toContain('x1={sparkline.activeHoverCursorX() ?? 0}');
    expect(interactiveSparklineSource).toContain('y1={0}');
    expect(interactiveSparklineSource).not.toContain('viewBox={`0 0 28 ${sparkline.vbH}`}');
    expect(interactiveSparklineSource).not.toContain(
      'viewBox={`0 0 ${sparkline.vbW} ${sparkline.xAxisBandPx}`}',
    );
    expect(interactiveSparklineSource).not.toContain('style={{');
    expect(interactiveSparklineSource).not.toContain('style={');
    expect(interactiveSparklineSource).not.toContain('{(cursorX) => (');
    expect(interactiveSparklineSource).toContain('data-rendered-series-count');
    expect(interactiveSparklineSource).not.toContain('createEffect');
    expect(interactiveSparklineSource).not.toContain('createSignal');
    expect(interactiveSparklineSource).not.toContain('scheduleSparkline');
    expect(interactiveSparklineSource).not.toContain('downsampleLTTB');

    expect(interactiveSparklineStateSource).toContain(
      'export function useInteractiveSparklineState',
    );
    expect(interactiveSparklineStateSource).toContain('activeSeriesDisplay');
    expect(interactiveSparklineStateSource).toContain('shouldRenderSeries');
    expect(interactiveSparklineStateSource).toContain('renderedSeriesCount');
    expect(interactiveSparklineStateSource).toContain('createSignal');
    expect(interactiveSparklineStateSource).toContain('scheduleSparkline');
    expect(interactiveSparklineStateSource).toContain('computeInteractiveSparklineHoverState');
    expect(interactiveSparklineStateSource).toContain('const localHover = hoveredState();');
    expect(interactiveSparklineStateSource).toContain('return localHover.x;');
    expect(interactiveSparklineStateSource).toContain('timestamp: synchronizedHoverTimestamp()');
    expect(interactiveSparklineStateSource).not.toContain('timestamp: activeHoverTimestamp()');

    expect(interactiveSparklineModelSource).toContain('buildInteractiveSparklineChartData');
    expect(interactiveSparklineModelSource).toContain('computeInteractiveSparklineHoverState');
    expect(interactiveSparklineModelSource).toContain('getInteractiveSparklineCursorXForTimestamp');
    expect(interactiveSparklineModelSource).toContain('const tooltipX = chartRect.left + mouseX;');
    expect(interactiveSparklineModelSource).toContain(
      'const tooltipY = chartRect.top + mouseY - 6;',
    );
    expect(interactiveSparklineModelSource).toContain('downsampleLTTB');
    expect(interactiveSparklineModelSource).toContain('findNearestMetricPoint');
  });

  it('keeps density map on shell, runtime, and model owners', () => {
    expect(densityMapSource).toContain('useDensityMapState');
    expect(densityMapSource).not.toContain('Latest');
    expect(densityMapSource).not.toContain('Top activity overview');
    expect(densityMapSource).not.toContain('detail().currentValue');
    expect(densityMapSource).toContain('data-density-map-tooltip="true"');
    expect(densityMapSource).not.toContain('data-density-map-tooltip-sparkline="true"');
    expect(densityMapSource).toContain('grid-cols-[auto_minmax(0,1fr)_auto]');
    expect(densityMapSource).not.toContain('max-w-[94px]');
    expect(densityMapSource).toContain(
      'whitespace-nowrap text-[11px] font-semibold text-emerald-400',
    );
    expect(densityMapSource).toContain(
      'whitespace-nowrap text-[11px] font-semibold text-base-content',
    );
    expect(densityMapSource).not.toContain('timeRangeToMs');
    expect(densityMapSource).not.toContain('createSignal');
    expect(densityMapSource).not.toContain('ctx.fillRect');

    expect(densityMapStateSource).toContain('export function useDensityMapState');
    expect(densityMapStateSource).toContain('createSignal');
    expect(densityMapStateSource).toContain('canvas.getContext');
    expect(densityMapStateSource).toContain('window.addEventListener');

    expect(densityMapModelSource).toContain('buildDensityMapChartData');
    expect(densityMapModelSource).toContain('buildDensityMapFocusDetail');
    expect(densityMapModelSource).toContain('buildDensityMapHoveredState');
    expect(densityMapModelSource).toContain('formatDensityMapHoverTime');
    expect(densityMapModelSource).toContain('getDensityMapColumnIndexForTimestamp');
    expect(densityMapModelSource).toContain('getDensityMapCellOpacity');
    expect(densityMapModelSource).not.toContain('buildDensityMapFocusSparklinePath');
    expect(densityMapModelSource).not.toContain('sparklinePath: string | null;');
    expect(densityMapModelSource).not.toContain('currentValue:');
    expect(densityMapModelSource).not.toContain('hoveredValue:');
  });

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
    expect(historyChartTooltipSource).toContain('new Date(point().timestamp).toLocaleString()');
    expect(historyChartTooltipSource).not.toContain('<Portal>');
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

  it('keeps infrastructure details drawer on shell, runtime, and model owners', () => {
    expect(infrastructureDetailsDrawerSource).toContain('useInfrastructureDetailsDrawerState');
    expect(infrastructureDetailsDrawerSource).toContain(
      'resolveInfrastructureDetailsDrawerMetadataId',
    );
    expect(infrastructureDetailsDrawerSource).toContain(
      'resolveInfrastructureDetailsDrawerDiscoveryHostname',
    );
    expect(infrastructureDetailsDrawerSource).not.toContain('createSignal');
    expect(infrastructureDetailsDrawerSource).not.toContain('getInfrastructureMetadataId');
    expect(infrastructureDetailsDrawerSource).not.toContain('getInfrastructureDiscoveryHostname');

    expect(infrastructureDetailsDrawerStateSource).toContain(
      'export function useInfrastructureDetailsDrawerState',
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
    expect(infrastructureDetailsDrawerModelSource).toContain('getInfrastructureDiscoveryHostname');
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

    expect(mobileNavBarModelSource).toContain('buildOrderedMobileNavPlatformTabs');
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
    expect(progressBarSource).not.toContain('style={{');
    expect(progressBarSource).not.toContain('style={');
  });

  it('keeps search field on shell, runtime, and model owners', () => {
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
  });

  it('keeps search input on shell, runtime, and model owners', () => {
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

  it('keeps whats new modal on shell, runtime, and model owners', () => {
    expect(whatsNewModalSource).toContain('useWhatsNewModalState');
    expect(whatsNewModalSource).toContain('useDialogState');
    expect(whatsNewModalSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalSource).toContain('Portal');
    expect(whatsNewModalSource).not.toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalSource).not.toContain('createSignal');
    expect(whatsNewModalSource).not.toContain('WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalSource).not.toContain('Migration guide');
    expect(whatsNewModalSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md',
    );

    expect(whatsNewModalStateSource).toContain('export function useWhatsNewModalState');
    expect(whatsNewModalStateSource).toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalStateSource).toContain('createSignal');
    expect(whatsNewModalStateSource).toContain('createMemo');
    expect(whatsNewModalStateSource).toContain('STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalStateSource).toContain('sessionPresentationPolicyResolved');
    expect(whatsNewModalStateSource).toContain('presentationPolicyIsDemoMode');
    expect(whatsNewModalStateSource).toContain('handleClose');
    expect(whatsNewModalStateSource).toContain('handleNext');
    expect(whatsNewModalStateSource).toContain('spotlightStyle');

    expect(whatsNewModalModelSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_PRIVACY_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_LABEL');
    expect(whatsNewModalModelSource).toContain('MIGRATION_GUIDE_DOC_URL');
    expect(whatsNewModalModelSource).toContain('Telemetry details');
    expect(whatsNewModalModelSource).toContain("title: 'Infrastructure'");
  });

  it('keeps dialog stack visibility in the shared dialog runtime', () => {
    expect(dialogStateSource).toContain('export function dialogStackHasBlockingDialog');
    expect(dialogStateSource).toContain('createSignal');
    expect(dialogStateSource).toContain('openDialogCount');
    expect(dialogStateSource).toContain('document.body.style.overflow');
  });

  it('keeps summary density control inside the shared summary primitives', () => {
    expect(summaryPanelSource).toContain("density?: 'default' | 'compact'");
    expect(summaryPanelSource).toContain("props.density === 'compact'");
    expect(summaryPanelSource).toContain('grid-cols-2 lg:grid-cols-4');
    expect(summaryPanelSource).not.toContain('Recovery Posture');
    expect(summaryPanelSource).not.toContain('stale items');

    expect(summaryMetricCardSource).toContain("density?: 'default' | 'compact'");
    expect(summaryMetricCardSource).toContain("props.density === 'compact'");
    expect(summaryMetricCardSource).toContain("props.bodyLayout ?? 'chart'");
    expect(summaryMetricCardSource).toContain(
      "isCompact() ? 'mb-1 min-h-[20px]' : 'mb-1.5 min-h-[24px]'",
    );
    expect(summaryMetricCardSource).toContain(
      "isCompact() ? 'h-[108px] sm:h-[120px]' : 'h-[136px] sm:h-[150px]'",
    );
    expect(summaryMetricCardSource).toContain('!p-1.5 sm:!p-2');
    expect(summaryMetricCardSource).not.toContain('Recovery Posture');
    expect(summaryMetricCardSource).not.toContain('Freshness');
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

  it('keeps collapsible search input on shell, runtime, and model owners', () => {
    expect(collapsibleSearchInputSource).toContain('useCollapsibleSearchInputState');
    expect(collapsibleSearchInputSource).not.toContain('createSignal');
    expect(collapsibleSearchInputSource).not.toContain('useTypeToSearch');
    expect(collapsibleSearchInputSource).not.toContain(
      "const triggerLabel = () => props.triggerLabel ?? 'Search'",
    );
    expect(collapsibleSearchInputSource).not.toContain(
      "const layoutClass = showExpanded() ? 'order-last basis-full w-full' : 'shrink-0 md:ml-auto'",
    );

    expect(collapsibleSearchInputStateSource).toContain(
      'export function useCollapsibleSearchInputState',
    );
    expect(collapsibleSearchInputStateSource).toContain('createSignal');
    expect(collapsibleSearchInputStateSource).toContain('useTypeToSearch');
    expect(collapsibleSearchInputStateSource).toContain('queueMicrotask');
    expect(collapsibleSearchInputStateSource).toContain('setIsExpanded(true)');

    expect(collapsibleSearchInputModelSource).toContain('getCollapsibleSearchTriggerLabel');
    expect(collapsibleSearchInputModelSource).toContain('shouldShowCollapsibleSearchExpanded');
    expect(collapsibleSearchInputModelSource).toContain('getCollapsibleSearchRootClass');
    expect(collapsibleSearchInputModelSource).toContain('order-last basis-full w-full');
  });

  it('keeps the chip-based FilterBar on a catalog descriptor with shell, chip, and add-menu owners', () => {
    expect(filterBarIndexSource).toContain("export { FilterBar } from './FilterBar';");
    expect(filterBarIndexSource).toContain("export { FilterChip } from './FilterChip';");
    expect(filterBarIndexSource).toContain("export { AddFilterMenu } from './AddFilterMenu';");
    expect(filterBarIndexSource).toContain("export { SavedViewsMenu } from './SavedViewsMenu';");
    expect(filterBarIndexSource).toContain("export { useSavedViews } from './useSavedViews';");
    expect(filterBarIndexSource).toContain('isFilterSet');
    expect(filterBarIndexSource).toContain('clearFilter');
    expect(filterBarIndexSource).toContain('formatFilterChipValue');

    expect(filterCatalogSource).toContain('export interface FilterDef');
    expect(filterCatalogSource).toContain('defaultValue: string');
    expect(filterCatalogSource).toContain(
      'export const isFilterSet = (filter: FilterDef): boolean =>',
    );
    expect(filterCatalogSource).toContain('export const clearFilter = (filter: FilterDef): void =>');
    expect(filterCatalogSource).toContain("filter.value() !== filter.defaultValue");

    expect(filterBarSource).toContain(
      "import { Card } from '@/components/shared/Card';",
    );
    expect(filterBarSource).toContain(
      "import { SearchInput } from '@/components/shared/SearchInput';",
    );
    expect(filterBarSource).toContain(
      "import { FilterMobileToggleButton } from '@/components/shared/FilterToolbar';",
    );
    expect(filterBarSource).toContain("import { AddFilterMenu } from './AddFilterMenu';");
    expect(filterBarSource).toContain("import { FilterChip } from './FilterChip';");
    expect(filterBarSource).toContain('props.filters.filter(isFilterSet)');
    expect(filterBarSource).toContain('activeCount() > 0');
    expect(filterBarSource).not.toContain("import { PageControls }");
    expect(filterBarSource).not.toContain('LabeledFilterSelect');
    expect(filterBarSource).not.toContain('LabeledFilterToggleGroup');
    expect(filterBarSource).not.toContain('filterControlsVariant');

    expect(filterChipSource).toContain("clearFilter,");
    expect(filterChipSource).toContain("formatFilterChipValue,");
    expect(filterChipSource).toContain("from './filterCatalog';");
    expect(filterChipSource).toContain('aria-haspopup="listbox"');
    expect(filterChipSource).toContain('aria-label={`Remove ${props.filter.label} filter`}');
    expect(filterChipSource).toContain('onClick={() => clearFilter(props.filter)}');
    expect(filterChipSource).toContain("event.key === 'Escape'");
    expect(filterChipSource).not.toContain('MutationObserver');
    // Type-ahead parity with AddFilterMenu: chip popover has its own search
    // input + arrow nav + Enter-to-commit, with activeIndex seeded on the
    // currently-selected value so Enter without typing is a no-op.
    expect(filterChipSource).toContain('aria-label={`Filter ${props.filter.label} values`}');
    expect(filterChipSource).toContain('filteredOptions');
    expect(filterChipSource).toContain('handleSearchKeyDown');
    expect(filterChipSource).toContain("event.key === 'ArrowDown'");
    expect(filterChipSource).toContain("event.key === 'ArrowUp'");
    expect(filterChipSource).toContain("event.key === 'Enter'");
    expect(filterChipSource).toContain('commitActive');
    expect(filterChipSource).toContain('queueMicrotask(() => searchInputRef?.focus())');

    expect(addFilterMenuSource).toContain("isFilterSet,");
    expect(addFilterMenuSource).toContain('type FilterDef,');
    expect(addFilterMenuSource).toContain("from './filterCatalog';");
    expect(addFilterMenuSource).toContain('availableFilters');
    expect(addFilterMenuSource).toContain('option.value !== filter.defaultValue');
    expect(addFilterMenuSource).toContain('GROUP_ORDER');
    expect(addFilterMenuSource).toContain('aria-haspopup="menu"');
    expect(addFilterMenuSource).not.toContain('LabeledFilterSelect');
    // Type-ahead: a search input narrows the visible filter (or value) list
    // and Enter commits the active item, so power users can pick a filter
    // with one click + a few keystrokes instead of three clicks.
    expect(addFilterMenuSource).toContain("aria-label={activeFilter() ? 'Filter values' : 'Filter filters'}");
    expect(addFilterMenuSource).toContain('filteredGroupedAvailable');
    expect(addFilterMenuSource).toContain('flatVisibleFilters');
    expect(addFilterMenuSource).toContain('filteredOptions');
    expect(addFilterMenuSource).toContain('handleSearchKeyDown');
    expect(addFilterMenuSource).toContain("event.key === 'ArrowDown'");
    expect(addFilterMenuSource).toContain("event.key === 'ArrowUp'");
    expect(addFilterMenuSource).toContain("event.key === 'Enter'");
    expect(addFilterMenuSource).toContain('commitActive');
    expect(addFilterMenuSource).toContain('queueMicrotask(() => searchInputRef?.focus())');

    // Saved views: per-page named filter combos persist to
    // localStorage under `pulse:filterbar:saved-views:<key>`. The hook owns
    // storage IO + URL navigation; the menu owns the dropdown chrome.
    expect(useSavedViewsSource).toContain("import { useLocation, useNavigate } from '@solidjs/router';");
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
    expect(recoveryProtectedInventorySectionSource).toContain(
      "import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';",
    );
    expect(recoveryProtectedInventorySectionSource).toContain('<FilterBar');
    expect(recoveryProtectedInventorySectionSource).toContain("id: 'item-type'");
    expect(recoveryProtectedInventorySectionSource).toContain("id: 'platform'");
    expect(recoveryProtectedInventorySectionSource).toContain("id: 'protected-state'");
    expect(recoveryProtectedInventorySectionSource).toContain('role="group"');
    expect(recoveryProtectedInventorySectionSource).toContain(
      'ariaLabel="Protected items controls"',
    );
    expect(recoveryProtectedInventorySectionSource).not.toContain('PageControls');
    expect(recoveryProtectedInventorySectionSource).not.toContain('LabeledFilterSelect');
    expect(recoveryProtectedInventorySectionSource).not.toContain('protectedFiltersOpen');

    // Recovery events sub-tab — every advanced filter folds into the catalog
    // (scope, method, verification, cluster, node, namespace) so the
    // dedicated "Filter" popover panel and FilterToolbarPanel-based advanced
    // panel are no longer rendered.
    expect(recoveryHistorySectionSource).toContain(
      "import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';",
    );
    expect(recoveryHistorySectionSource).toContain('<FilterBar');
    expect(recoveryHistorySectionSource).toContain(
      'ariaLabel="Recovery events controls"',
    );
    expect(recoveryHistorySectionSource).toContain("id: 'item-type'");
    expect(recoveryHistorySectionSource).toContain("id: 'platform'");
    expect(recoveryHistorySectionSource).toContain("id: 'outcome'");
    expect(recoveryHistorySectionSource).toContain("id: 'scope'");
    expect(recoveryHistorySectionSource).toContain("id: 'method'");
    expect(recoveryHistorySectionSource).toContain("id: 'verification'");
    expect(recoveryHistorySectionSource).toContain("id: 'cluster'");
    expect(recoveryHistorySectionSource).toContain("id: 'node'");
    expect(recoveryHistorySectionSource).toContain("id: 'namespace'");
    expect(recoveryHistorySectionSource).toContain('searchTrailing={');
    expect(recoveryHistorySectionSource).toContain('<RecoveryHistoryItemFilter');
    expect(recoveryHistorySectionSource).toContain('<ColumnPicker');
    expect(recoveryHistorySectionSource).toContain('onClearAll={');
    expect(recoveryHistorySectionSource).toContain(
      'showClearAll={props.hasActiveArtifactFilters}',
    );
    expect(recoveryHistorySectionSource).not.toContain('PageControls');
    expect(recoveryHistorySectionSource).not.toContain('FilterActionButton');
    expect(recoveryHistorySectionSource).not.toContain('FilterToolbarPanel');
    expect(recoveryHistorySectionSource).not.toContain('moreFiltersOpen');
    expect(recoveryHistorySectionSource).not.toContain('historyFiltersOpen');

    // WorkloadsFilter — Type/Status/Node/Platform/Namespace/Runtime are all
    // catalog chips. The xl-breakpoint segmented↔select swap retires here.
    expect(workloadsFilterSource).toContain(
      "import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';",
    );
    expect(workloadsFilterSource).toContain('<FilterBar');
    expect(workloadsFilterSource).toContain('ariaLabel="Workloads filters"');
    expect(workloadsFilterSource).toContain("id: 'workloads-type'");
    expect(workloadsFilterSource).toContain("id: 'workloads-status'");
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

    // StoragePageControls — Subtabs sit above the FilterBar; sort key/sort
    // direction live in viewOptionsTrailing as raw view-options (not chips).
    // Per-view catalog filters (Group by, Source, Status on Pools; Role/Group
    // on Physical Disks) flow through the FilterBar catalog. The legacy
    // 3-layer indirection (StoragePageControls → StorageControls →
    // StorageFilter) collapses to one component.
    expect(storagePageControlsSource).toContain(
      "import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';",
    );
    expect(storagePageControlsSource).toContain('<FilterBar');
    expect(storagePageControlsSource).toContain('<Subtabs');
    expect(storagePageControlsSource).toContain('ariaLabel="Storage filters"');
    expect(storagePageControlsSource).toContain("id: 'storage-node'");
    expect(storagePageControlsSource).toContain("id: 'storage-group-by'");
    expect(storagePageControlsSource).toContain("id: 'storage-source'");
    expect(storagePageControlsSource).toContain("id: 'storage-status'");
    expect(storagePageControlsSource).toContain("id: 'storage-disk-role'");
    expect(storagePageControlsSource).toContain("id: 'storage-disk-group'");
    expect(storagePageControlsSource).toContain('viewOptionsTrailing={');
    expect(storagePageControlsSource).toContain('aria-label="Sort by"');
    expect(storagePageControlsSource).toContain('aria-label="Sort direction"');
    expect(storagePageControlsSource).toContain('ChartVisibilityToggleButton');
    expect(storagePageControlsSource).toContain('onClearAll={handleClearAll}');
    expect(storagePageControlsSource).toContain('showClearAll={showClearAll}');
    // The component name is StoragePageControls but it must not import or
    // render the legacy PageControls primitive, the StorageControls /
    // StorageFilter intermediates, or any LabeledFilterSelect /
    // FilterSegmentedControl forks.
    expect(storagePageControlsSource).not.toContain(
      "from '@/components/shared/PageControls'",
    );
    expect(storagePageControlsSource).not.toContain('<PageControls');
    expect(storagePageControlsSource).not.toContain('<StorageControls');
    expect(storagePageControlsSource).not.toContain('<StorageFilter');
    expect(storagePageControlsSource).not.toContain('LabeledFilterSelect');
    expect(storagePageControlsSource).not.toContain('FilterSegmentedControl');
    // Storage's three-layer indirection retired — StoragePageControls no
    // longer imports the deleted StorageFilter / StorageControls modules,
    // and reads the canonical Storage types directly from storagePageState
    // and useStorageModel rather than re-exporting them from the deleted
    // shell.
    expect(storagePageControlsSource).not.toContain("from './StorageFilter'");
    expect(storagePageControlsSource).not.toContain("from './StorageControls'");
    expect(storagePageControlsSource).not.toContain('useStoragePageControlsModel');
    expect(storagePageControlsSource).not.toContain('useStorageControlsModel');
    expect(storagePageControlsSource).toContain("type StorageStatusFilterValue,");
    expect(storagePageControlsSource).toContain(
      "import type { StorageGroupKey, StorageSortKey } from './useStorageModel';",
    );
  });
});
