import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import calloutCardSource from '@/components/shared/CalloutCard.tsx?raw';
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
import infrastructureDetailsDrawerSource from '@/components/shared/InfrastructureDetailsDrawer.tsx?raw';
import infrastructureDetailsDrawerModelSource from '@/components/shared/infrastructureDetailsDrawerModel.ts?raw';
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
import subtabsSource from '@/components/shared/Subtabs.tsx?raw';
import toggleSource from '@/components/shared/Toggle.tsx?raw';
import toggleModelSource from '@/components/shared/toggleModel.ts?raw';
import searchTipsPopoverSource from '@/components/shared/SearchTipsPopover.tsx?raw';
import searchTipsPopoverModelSource from '@/components/shared/searchTipsPopoverModel.ts?raw';
import tooltipSource from '@/components/shared/Tooltip.tsx?raw';
import tooltipPortalSource from '@/components/shared/TooltipPortal.tsx?raw';
import tooltipModelSource from '@/components/shared/tooltipModel.ts?raw';
import trialBannerSource from '@/components/shared/TrialBanner.tsx?raw';
import trialBannerModelSource from '@/components/shared/trialBannerModel.ts?raw';
import interactiveSparklineSource from '@/components/shared/InteractiveSparkline.tsx?raw';
import interactiveSparklineModelSource from '@/components/shared/interactiveSparklineModel.ts?raw';
import contextualFocusSource from '@/components/shared/contextualFocus.ts?raw';
import summaryCardInteractionSource from '@/components/shared/summaryCardInteraction.ts?raw';
import summaryJumpToRowButtonSource from '@/components/shared/SummaryJumpToRowButton.tsx?raw';
import summaryRowActionButtonSource from '@/components/shared/SummaryRowActionButton.tsx?raw';
import summaryInteractionA11ySource from '@/components/shared/summaryInteractionA11y.ts?raw';
import summaryScopeBarSource from '@/components/shared/SummaryScopeBar.tsx?raw';
import summaryScopePresentationSource from '@/components/shared/summaryScopePresentation.ts?raw';
import summaryTableFocusSource from '@/components/shared/summaryTableFocus.ts?raw';
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
import commandPaletteStateSource from '@/components/shared/useCommandPaletteState.ts?raw';
import activeUseTrialNudgeStateSource from '@/components/shared/useActiveUseTrialNudgeState.ts?raw';
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
import trialBannerStateSource from '@/components/shared/useTrialBannerState.ts?raw';
import interactiveSparklineStateSource from '@/components/shared/useInteractiveSparklineState.ts?raw';
import monitoredSystemLimitWarningBannerStateSource from '@/components/shared/useMonitoredSystemLimitWarningBannerState.ts?raw';
import selectionCardGroupStateSource from '@/components/shared/useSelectionCardGroupState.ts?raw';
import webInterfaceUrlFieldSource from '@/components/shared/WebInterfaceUrlField.tsx?raw';
import webInterfaceUrlFieldModelSource from '@/components/shared/webInterfaceUrlFieldModel.ts?raw';
import webInterfaceUrlFieldStateSource from '@/components/shared/useWebInterfaceUrlFieldState.ts?raw';
import guestRowSource from '@/components/Dashboard/GuestRow.tsx?raw';
import workloadPanelSource from '@/components/Dashboard/WorkloadPanel.tsx?raw';
import guestRowStateSource from '@/components/Dashboard/useGuestRowState.ts?raw';
import dashboardSelectionStateSource from '@/components/Dashboard/useDashboardSelectionState.ts?raw';
import infrastructureSummarySource from '@/components/Infrastructure/InfrastructureSummary.tsx?raw';
import infrastructureSummaryStateSource from '@/components/Infrastructure/useInfrastructureSummaryState.ts?raw';
import unifiedResourceHostTableCardSource from '@/components/Infrastructure/UnifiedResourceHostTableCard.tsx?raw';
import unifiedResourcePBSTableSectionSource from '@/components/Infrastructure/UnifiedResourcePBSTableSection.tsx?raw';
import unifiedResourcePMGTableSectionSource from '@/components/Infrastructure/UnifiedResourcePMGTableSection.tsx?raw';
import nodeGroupHeaderSource from '@/components/shared/NodeGroupHeader.tsx?raw';
import storageGroupRowSource from '@/components/Storage/StorageGroupRow.tsx?raw';
import storagePoolRowSource from '@/components/Storage/StoragePoolRow.tsx?raw';
import diskListSource from '@/components/Storage/DiskList.tsx?raw';
import storageSummarySource from '@/components/Storage/StorageSummary.tsx?raw';
import workloadsSummarySource from '@/components/Workloads/WorkloadsSummary.tsx?raw';
import resourceDetailDrawerOverviewSource from '@/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx?raw';
import aiSettingsDialogsSource from '@/components/Settings/AISettingsDialogs.tsx?raw';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';
import proxmoxSettingsPanelSource from '@/components/Settings/ProxmoxSettingsPanel.tsx?raw';
import reportingPanelSource from '@/components/Settings/ReportingPanel.tsx?raw';
import updatesSettingsPanelSource from '@/components/Settings/UpdatesSettingsPanel.tsx?raw';

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
    expect(tooltipSource).toContain('bg-surface text-base-content border-border');
    expect(tooltipSource).not.toContain("'background-color': 'rgb(15, 23, 42)'");
    expect(tooltipPortalSource).toContain('bg-surface');
    expect(tooltipPortalSource).toContain('text-base-content');
    expect(tooltipPortalSource).toContain('border-border');
    expect(tooltipPortalSource).not.toContain("'background-color': 'rgb(15, 23, 42)'");
    expect(interactiveSparklineSource).toContain('bg-surface');
    expect(interactiveSparklineSource).toContain('text-base-content');
    expect(interactiveSparklineSource).toContain('border-border');
    expect(interactiveSparklineSource).not.toContain("'background-color': 'rgb(15, 23, 42)'");
  });

  it('keeps active use trial nudge on shell, runtime, and model owners', () => {
    expect(activeUseTrialNudgeSource).toContain('useActiveUseTrialNudgeState');
    expect(activeUseTrialNudgeSource).toContain('ACTIVE_USE_TRIAL_NUDGE_TITLE');
    expect(activeUseTrialNudgeSource).not.toContain('createSignal');
    expect(activeUseTrialNudgeSource).not.toContain('createMemo');
    expect(activeUseTrialNudgeSource).not.toContain('startProTrial');
    expect(activeUseTrialNudgeSource).not.toContain('localStorage');
    expect(activeUseTrialNudgeSource).not.toContain('setInterval');

    expect(activeUseTrialNudgeStateSource).toContain('export function useActiveUseTrialNudgeState');
    expect(activeUseTrialNudgeStateSource).toContain('createSignal');
    expect(activeUseTrialNudgeStateSource).toContain('createMemo');
    expect(activeUseTrialNudgeStateSource).toContain('window.localStorage');
    expect(activeUseTrialNudgeStateSource).toContain('setInterval');
    expect(activeUseTrialNudgeStateSource).toContain('runStartProTrialAction');
    expect(activeUseTrialNudgeStateSource).toContain('snoozeUpsell');

    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY');
    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY');
    expect(activeUseTrialNudgeModelSource).toContain('isActiveUseTrialNudgeEligible');
    expect(activeUseTrialNudgeModelSource).toContain('isActiveUseTrialNudgeOldEnough');
    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_TITLE');
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
    expect(summaryCardInteractionSource).toContain('resolveSummaryScopeState');
    expect(interactiveSparklineModelSource).toContain('onHoverSyncChange');
    expect(densityMapModelSource).toContain('onHoverSyncChange');

    expect(dashboardSelectionStateSource).toContain('preserveScrollableAncestorVerticalOffset');
    expect(dashboardSelectionStateSource).toContain('hoveredWorkloadGroupScope');
    expect(dashboardSelectionStateSource).toContain('activeSummaryWorkloadGroupScope');
    expect(dashboardSelectionStateSource).not.toContain('const scrollTop = scroller?.scrollTop');

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
    expect(summaryTableFocusSource).toContain('consumeNextFocusedRevealSkip');
    expect(summaryTableFocusSource).toContain('MutationObserver');
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

  it('keeps scope-bar presentation on one shared primitive and model', () => {
    expect(summaryScopePresentationSource).toContain('buildSummaryScopePresentation');
    expect(summaryScopePresentationSource).toContain("mode: 'all'");
    expect(summaryScopeBarSource).toContain('SummaryScopeBar');
    expect(summaryScopeBarSource).toContain('Previewing');
    expect(summaryScopeBarSource).toContain('Showing');
    expect(summaryScopeBarSource).toContain('Pinned to');
    expect(summaryScopeBarSource).toContain('Reset pinned scope');
    expect(summaryScopeBarSource).not.toContain('rounded-full');
    expect(summaryScopeBarSource).not.toContain('bg-surface-alt/60');
    expect(summaryScopeBarSource).not.toContain('useLocation(');
    expect(summaryScopeBarSource).not.toContain('useNavigate(');
  });

  it('keeps synchronized summary values on one shared card/readout contract', () => {
    expect(summaryMetricCardSource).toContain('headerValue?: JSX.Element');
    expect(summaryMetricCardSource).toContain("bodyLayout?: 'chart' | 'auto'");
    expect(summaryMetricCardSource).toContain('props.headerValue');
    expect(summaryMetricCardSource).not.toContain('data-summary-sync-readout');

    expect(summarySynchronizedReadoutSource).toContain(
      'export const SummarySynchronizedReadout',
    );
    expect(summarySynchronizedReadoutSource).toContain('data-summary-sync-readout="true"');
    expect(summarySynchronizedReadoutSource).toContain(
      'formatSummarySynchronizedReadoutTime',
    );
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

    expect(guestRowSource).toContain('data-summary-row-active');
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

  it('keeps trial banner on shell, runtime, and model owners', () => {
    expect(trialBannerSource).toContain('useTrialBannerState');
    expect(trialBannerSource).toContain('TRIAL_BANNER_TITLE');
    expect(trialBannerSource).not.toContain('createSignal');
    expect(trialBannerSource).not.toContain('createMemo');
    expect(trialBannerSource).not.toContain('loadLicenseStatus');
    expect(trialBannerSource).not.toContain('licenseStatus');
    expect(trialBannerSource).not.toContain('getUpgradeActionUrlOrFallback');

    expect(trialBannerStateSource).toContain('export function useTrialBannerState');
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
    expect(proxmoxSettingsPanelSource).toContain('CalloutCard');
    expect(proxmoxSettingsPanelSource).not.toContain(
      'rounded-md border border-blue-200 bg-blue-50 px-4 py-3',
    );
    expect(reportingPanelSource).toContain('CalloutCard');
    expect(reportingPanelSource).not.toContain('rounded-md border border-blue-200 bg-blue-50 p-6');
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
    expect(monitoredSystemLimitWarningBannerSource).toContain(
      'MONITORED_SYSTEM_LIMIT_LEARN_MORE_LABEL',
    );
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createEffect');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('createMemo');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('loadLicenseStatus');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('trackUpgradeMetricEvent');
    expect(monitoredSystemLimitWarningBannerSource).not.toContain('legacyConnections()');

    expect(monitoredSystemLimitWarningBannerStateSource).toContain(
      'export function useMonitoredSystemLimitWarningBannerState',
    );
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
      'getMonitoredSystemLimitUpgradeLabel',
    );
    expect(monitoredSystemLimitWarningBannerModelSource).toContain(
      'getMonitoredSystemLimitInstallCollectorsLabel',
    );
  });

  it('keeps shared tag badges in the shared primitive boundary', () => {
    expect(tagBadgesSource).toContain("from '@/components/shared/Tooltip'");
    expect(guestRowSource).toContain("from '@/components/shared/TagBadges'");
    expect(guestRowSource).not.toContain("from './TagBadges'");
    expect(resourceDetailDrawerOverviewSource).toContain("from '@/components/shared/TagBadges'");
    expect(resourceDetailDrawerOverviewSource).not.toContain(
      "from '@/components/Dashboard/TagBadges'",
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

    expect(infrastructureSelectorStateSource).toContain('useResources');
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
    expect(interactiveSparklineSource).toContain('x1={sparkline.activeHoverCursorX() ?? 0}');
    expect(interactiveSparklineSource).toContain('y1={0}');
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
    expect(interactiveSparklineModelSource).toContain('let tooltipY = clientY - 10;');
    expect(interactiveSparklineModelSource).not.toContain('let tooltipY = chartRect.top - 6;');
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
    expect(densityMapSource).toContain('whitespace-nowrap text-[11px] font-semibold text-emerald-400');
    expect(densityMapSource).toContain('whitespace-nowrap text-[11px] font-semibold text-base-content');
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
    expect(historyChartModelSource).toContain('HISTORY_CHART_RANGES');
    expect(historyChartModelSource).toContain('getHistoryChartScale');
    expect(historyChartModelSource).toContain('findHistoryChartClosestPoint');

    expect(historyChartHeaderSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartHeaderSource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartHeaderSource).not.toContain('setupCanvasDPR');

    expect(historyChartOverlaySource).toContain('Collecting data... History will appear here.');
    expect(historyChartOverlaySource).toContain('Unlock {props.chart.lockTierLabel()} Features');
    expect(historyChartOverlaySource).not.toContain('ChartsAPI.getMetricsHistory');
    expect(historyChartOverlaySource).not.toContain('setupCanvasDPR');

    expect(historyChartTooltipSource).toContain('formatHistoryChartTooltipValue');
    expect(historyChartTooltipSource).toContain('new Date(point().timestamp).toLocaleString()');
    expect(historyChartTooltipSource).not.toContain('ChartsAPI.getMetricsHistory');
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
    expect(pulseDataGridSource).toContain('isPulseDataGridInteractiveTarget');
    expect(pulseDataGridSource).not.toContain('useBreakpoint');
    expect(pulseDataGridSource).not.toContain('createStore');
    expect(pulseDataGridSource).not.toContain('target.closest(');

    expect(pulseDataGridStateSource).toContain('export function usePulseDataGridState');
    expect(pulseDataGridStateSource).toContain('useBreakpoint');
    expect(pulseDataGridStateSource).toContain('createStore');
    expect(pulseDataGridStateSource).toContain('reconcile(');

    expect(pulseDataGridModelSource).toContain('export const getPulseDataGridAlignClass');
    expect(pulseDataGridModelSource).toContain('export const isPulseDataGridInteractiveTarget');
    expect(pulseDataGridModelSource).toContain('target.closest(');
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
    expect(whatsNewModalSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalSource).not.toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalSource).not.toContain('createSignal');
    expect(whatsNewModalSource).not.toContain('WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalSource).not.toContain('Documentation');
    expect(whatsNewModalSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md',
    );

    expect(whatsNewModalStateSource).toContain('export function useWhatsNewModalState');
    expect(whatsNewModalStateSource).toContain('createLocalStorageBooleanSignal');
    expect(whatsNewModalStateSource).toContain('createSignal');
    expect(whatsNewModalStateSource).toContain('STORAGE_KEYS.WHATS_NEW_NAV_V2_SHOWN');
    expect(whatsNewModalStateSource).toContain('handleClose');

    expect(whatsNewModalModelSource).toContain('WHATS_NEW_FEATURE_CARDS');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_TELEMETRY_TITLE');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_PRIVACY_URL');
    expect(whatsNewModalModelSource).toContain('WHATS_NEW_DOCS_LABEL');
    expect(whatsNewModalModelSource).toContain("title: 'Infrastructure'");
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
    expect(summaryMetricCardSource).toContain("isCompact() ? 'mb-1 min-h-[20px]' : 'mb-1.5 min-h-[24px]'");
    expect(summaryMetricCardSource).toContain("isCompact() ? 'h-[108px] sm:h-[120px]' : 'h-[136px] sm:h-[150px]'");
    expect(summaryMetricCardSource).toContain('!p-1.5 sm:!p-2');
    expect(summaryMetricCardSource).not.toContain('Recovery Posture');
    expect(summaryMetricCardSource).not.toContain('Freshness');
  });

  it('keeps tooltip on shell, runtime, and model owners', () => {
    expect(tooltipSource).toContain('useTooltipState');
    expect(tooltipSource).toContain('createTooltipSystemState');
    expect(tooltipSource).not.toContain('createSignal');
    expect(tooltipSource).not.toContain('requestAnimationFrame');
    expect(tooltipSource).not.toContain('sanitizeTooltipContent');
    expect(tooltipSource).not.toContain('resolveTooltipPosition');

    expect(tooltipStateSource).toContain('export function useTooltipState');
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
});
