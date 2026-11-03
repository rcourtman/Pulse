import { traverse, transformSync, types as t } from '@babel/core';
// @ts-expect-error babel-preset-solid ships no type declarations.
import solidPreset from 'babel-preset-solid';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import aiModelPickerSource from '@/components/shared/AIModelPicker.tsx?raw';
import detailSectionTableSource from '@/components/shared/DetailSectionTable.tsx?raw';
import detailSectionModelSource from '@/components/shared/detailSectionModel.ts?raw';
import drawerAttentionSectionSource from '@/components/shared/DrawerAttentionSection.tsx?raw';
import filterToolbarSource from '@/components/shared/FilterToolbar.tsx?raw';
import filterOptionPresentationSource from '@/components/shared/filterOptionPresentation.ts?raw';
import historyChartTooltipSource from '@/components/shared/HistoryChartTooltip.tsx?raw';
import {
  GPU_METRICS_HISTORY_GROUPS,
  HOST_METRICS_HISTORY_GROUPS,
} from '@/components/shared/hostMetricsHistoryModel';
import pulseDataGridSource from '@/components/shared/PulseDataGrid.tsx?raw';
import progressBarSource from '@/components/shared/ProgressBar.tsx?raw';
import searchFieldSource from '@/components/shared/SearchField.tsx?raw';
import subtabsSource from '@/components/shared/Subtabs.tsx?raw';
import actionsSource from '@/pages/Actions.tsx?raw';
import tooltipSource from '@/components/shared/Tooltip.tsx?raw';
import tooltipPortalSource from '@/components/shared/TooltipPortal.tsx?raw';
import contextualFocusSource from '@/components/shared/contextualFocus.ts?raw';
import summaryCardInteractionSource from '@/components/shared/summaryCardInteraction.ts?raw';
import summaryRowActionButtonSource from '@/components/shared/SummaryRowActionButton.tsx?raw';
import summaryInteractionA11ySource from '@/components/shared/summaryInteractionA11y.ts?raw';
import windowedPageScrollSource from '@/components/shared/windowedPageScroll.ts?raw';
import tableSource from '@/components/shared/Table.tsx?raw';
import tableSortPresentationSource from '@/components/shared/tableSortPresentation.ts?raw';
import tableCardHeaderSource from '@/components/shared/TableCardHeader.tsx?raw';
import summaryTableFocusSource from '@/components/shared/summaryTableFocus.ts?raw';
import tableCardSource from '@/components/shared/TableCard.tsx?raw';
import groupedTableRowPresentationSource from '@/components/shared/groupedTableRowPresentation.ts?raw';
import storagePoolsTableWindowingSource from '@/components/Storage/useStoragePoolsTableWindowing.ts?raw';
import workloadViewportSyncSource from '@/components/Workloads/useWorkloadViewportSync.ts?raw';
import platformWindowedItemsSource from '@/features/platformPage/usePlatformWindowedItems.ts?raw';
import animatedNumberSource from '@/components/shared/AnimatedNumber.tsx?raw';
import animatedNumberModelSource from '@/components/shared/animatedNumberModel.ts?raw';
import animatedNumberStateSource from '@/components/shared/useAnimatedNumberState.ts?raw';
import workloadsFilterSource from '@/components/Workloads/WorkloadsFilter.tsx?raw';
import infrastructureSourceManagerSource from '@/components/Settings/InfrastructureSourceManager.tsx?raw';
import summaryChartLayoutSource from '@/components/shared/summaryChartLayout.ts?raw';
import tagBadgesSource from '@/components/shared/TagBadges.tsx?raw';
import tlsVerificationWarningBannerSource from '@/components/shared/TlsVerificationWarningBanner.tsx?raw';
import dialogStateSource from '@/components/shared/useDialogState.ts?raw';
import webInterfaceLinkSource from '@/components/shared/WebInterfaceLink.tsx?raw';
import inlineDetailTableRowSource from '@/components/shared/InlineDetailTableRow.tsx?raw';
import sharedTemplateRegistrySource from '../../../scripts/shared-template-registry.json?raw';
import dockerHostDrawerSource from '@/features/docker/DockerHostDrawer.tsx?raw';
import kubernetesPageSurfaceSource from '@/features/kubernetes/KubernetesPageSurface.tsx?raw';
import sharedPlatformPageSource from '@/features/platformPage/sharedPlatformPage.tsx?raw';
import platformResourceDetailTableRowSource from '@/features/platformPage/PlatformResourceDetailTableRow.tsx?raw';
import truenasProtectionTableSource from '@/features/truenas/TrueNASProtectionTable.tsx?raw';
import guestRowSource from '@/components/Workloads/GuestRow.tsx?raw';
import guestDrawerSource from '@/components/Workloads/GuestDrawer.tsx?raw';
import guestDrawerHistorySource from '@/components/Workloads/GuestDrawerHistory.tsx?raw';
import nodeDrawerSource from '@/components/Workloads/NodeDrawer.tsx?raw';
import workloadsTableSource from '@/components/Workloads/WorkloadsTable.tsx?raw';
import workloadTableHeaderSource from '@/components/Workloads/WorkloadTableHeader.tsx?raw';
import workloadPanelSource from '@/components/Workloads/WorkloadPanel.tsx?raw';
import guestRowStateSource from '@/components/Workloads/useGuestRowState.ts?raw';
import workloadSelectionStateSource from '@/components/Workloads/useWorkloadSelectionState.ts?raw';
import proxmoxBackupServersTableSource from '@/features/proxmox/ProxmoxBackupServersTable.tsx?raw';
import proxmoxCephTableSource from '@/features/proxmox/ProxmoxCephTable.tsx?raw';
import proxmoxNodesTableSource from '@/features/proxmox/ProxmoxNodesTable.tsx?raw';
import proxmoxPageModelSource from '@/features/proxmox/proxmoxPageModel.ts?raw';
import proxmoxRecoverableTableSource from '@/features/proxmox/ProxmoxRecoverableTable.tsx?raw';
import agentsMachinesTableSource from '@/features/standalone/AgentsMachinesTable.tsx?raw';
import proxmoxMailGatewayDrawerSource from '@/features/proxmox/ProxmoxMailGatewayDrawer.tsx?raw';
import k8sNamespacesDrawerSource from '@/components/Kubernetes/K8sNamespacesDrawer.tsx?raw';
import nodeGroupHeaderSource from '@/components/shared/NodeGroupHeader.tsx?raw';
import proxmoxVersionSource from '@/utils/proxmoxVersion.ts?raw';
import storageGroupRowSource from '@/components/Storage/StorageGroupRow.tsx?raw';
import storageGroupPresentationSource from '@/features/storageBackups/groupPresentation.ts?raw';
import storagePoolRowSource from '@/components/Storage/StoragePoolRow.tsx?raw';
import storageContentCardSource from '@/components/Storage/StorageContentCard.tsx?raw';
import storagePoolsTableSource from '@/components/Storage/StoragePoolsTable.tsx?raw';
import storagePoolDetailSource from '@/components/Storage/StoragePoolDetail.tsx?raw';
import diskDrawerOverviewSource from '@/components/Storage/DiskDrawerOverview.tsx?raw';
import diskListSource from '@/components/Storage/DiskList.tsx?raw';
import alertHistoryTableSectionSource from '@/features/alerts/AlertHistoryTableSection.tsx?raw';
import alertHistoryTableGroupRowSource from '@/features/alerts/AlertHistoryTableGroupRow.tsx?raw';
import alertResourceTableDesktopSource from '@/components/Alerts/AlertResourceTableDesktop.tsx?raw';
import aiCostDashboardSource from '@/components/AI/AICostDashboard.tsx?raw';
import resourceDetailSummarySource from '@/components/Infrastructure/ResourceDetailSummary.tsx?raw';
import resourceDetailDrawerSource from '@/components/Infrastructure/ResourceDetailDrawer.tsx?raw';
import agentProfilesPanelSource from '@/components/Settings/AgentProfilesPanel.tsx?raw';
import apiTokenManagerSource from '@/components/Settings/APITokenManager.tsx?raw';
import organizationAccessMembersSectionSource from '@/components/Settings/OrganizationAccessMembersSection.tsx?raw';
import organizationIncomingSharesSectionSource from '@/components/Settings/OrganizationIncomingSharesSection.tsx?raw';
import organizationOutgoingSharesSectionSource from '@/components/Settings/OrganizationOutgoingSharesSection.tsx?raw';
import organizationOverviewMembersSectionSource from '@/components/Settings/OrganizationOverviewMembersSection.tsx?raw';
import rolesPanelSource from '@/components/Settings/RolesPanel.tsx?raw';
import userAssignmentsPanelSource from '@/components/Settings/UserAssignmentsPanel.tsx?raw';
import featureGateSectionSource from '@/components/shared/FeatureGateSection.tsx?raw';
import settingsPanelSource from '@/components/shared/SettingsPanel.tsx?raw';
import resourceDetailDrawerOverviewTabSource from '@/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx?raw';
import dockerAlertsTableSource from '@/features/docker/DockerAlertsTable.tsx?raw';
import kubernetesAlertsTableSource from '@/features/kubernetes/KubernetesAlertsTable.tsx?raw';
import proxmoxBackupsTableSharedSource from '@/features/proxmox/proxmoxBackupsTableShared.tsx?raw';
import truenasAlertsTableSource from '@/features/truenas/TrueNASAlertsTable.tsx?raw';
import truenasAppsTableSource from '@/features/truenas/TrueNASAppsTable.tsx?raw';
import truenasNetworkSharesTableSource from '@/features/truenas/TrueNASNetworkSharesTable.tsx?raw';
import truenasStorageTopologyTableSource from '@/features/truenas/TrueNASStorageTopologyTable.tsx?raw';
import truenasSystemsTableSource from '@/features/truenas/TrueNASSystemsTable.tsx?raw';
import truenasVirtualMachinesTableSource from '@/features/truenas/TrueNASVirtualMachinesTable.tsx?raw';
import vsphereActivityTableSource from '@/features/vmware/VsphereActivityTable.tsx?raw';
import vsphereAlertsTableSource from '@/features/vmware/VsphereAlertsTable.tsx?raw';
import dockerHostDrawerOverviewSource from '@/features/docker/DockerHostDrawerOverview.tsx?raw';

const sharedSources = import.meta.glob(['./*.tsx', './cards/*.tsx', './responsive/*.tsx'], {
  query: '?raw',
  eager: true,
  import: 'default',
}) as Record<string, string>;
const frontendIndexCssSource = readFileSync(join(process.cwd(), 'src/index.css'), 'utf8');
const readFrontendSource = (path: string) => readFileSync(join(process.cwd(), path), 'utf8');

describe('shared primitive guardrails', () => {
  it('enforces every shared template registry rule and pattern guard', () => {
    // The registry is the single source for shared-template adoption rules;
    // restating its entries here only doubled every edit.
    const audit = spawnSync(process.execPath, ['scripts/shared-template-audit.mjs'], {
      cwd: process.cwd(),
      encoding: 'utf8',
    });
    expect(audit.stderr).toBe('');
    expect(audit.status).toBe(0);
  });

  it('converts normalized disk usage ratios to percentages at the aggregate bar boundary', () => {
    const source = readFrontendSource('src/components/shared/cards/DisksCard.tsx');
    expect(source).toContain('aggregateDisk={{ ...aggregate(), usage: aggregateUsagePercent() }}');
    expect(source).toContain('(aggregate.used / aggregate.total) * 100');
    expect(source).not.toContain('aggregateDisk={aggregate()}');
  });

  it('keeps disk mounts in the parent scrolling flow rather than a hidden nested list', () => {
    const source = readFrontendSource('src/components/shared/cards/DisksCard.tsx');
    expect(source).toContain('data-testid="disks-card-mounts"');
    expect(source).not.toMatch(/max-h-|overflow-y-|custom-scrollbar/);
    expect(source).toContain('<For each={props.disks}>');
    expect(source).toContain('<StackedDiskBar');
  });

  it('keeps one canonical agent-host metric history group catalog', () => {
    expect(HOST_METRICS_HISTORY_GROUPS.map((group) => group.id)).toEqual([
      'utilization',
      'network',
      'disk-io',
      'thermals',
    ]);
    expect(HOST_METRICS_HISTORY_GROUPS.at(-1)).toMatchObject({
      label: 'Thermals',
      unit: 'C',
      series: [{ metric: 'temperature', label: 'CPU', unit: 'C' }],
    });
    expect(GPU_METRICS_HISTORY_GROUPS.map((group) => group.id)).toEqual([
      'gpu-utilization',
      'gpu-thermal',
    ]);
  });

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

  it('keeps AI model picker labels route-aware for gateway providers', () => {
    expect(aiModelPickerSource).toContain('formatAIModelRouteLabel(match)');
    expect(aiModelPickerSource).toContain('formatAIModelRouteLabel(model)');
    expect(aiModelPickerSource).toContain('isPulseOwnedLocalModelRoute');
    expect(aiModelPickerSource).toContain('const secondaryModelId = () =>');
    expect(aiModelPickerSource).toContain('!isPulseOwnedLocalModelRoute(model.id)');
    expect(aiModelPickerSource).not.toContain("model.id.split(':').pop()");
  });

  it('keeps AI model picking on the shared searchable notable-first primitive', () => {
    expect(aiModelPickerSource).toContain(
      "import { SearchField } from '@/components/shared/SearchField';",
    );
    expect(aiModelPickerSource).toContain(
      "import { AI_MODEL_PICKER_EMPTY_STATE } from '@/utils/aiSettingsPresentation';",
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

  it('keeps shared subtabs as one primitive and leaves shell styling to owning surfaces', () => {
    expect(subtabsSource).not.toContain("variant?: 'default' | 'control'");
    expect(subtabsSource).not.toContain('subtabsControlShellClass');
    expect(subtabsSource).toContain('subtabsShellClass');
    expect(subtabsSource).toContain('subtabsListClass');
    expect(subtabsSource).toContain('subtabButtonClass');
    expect(subtabsSource).toContain('canScrollLeft');
    expect(subtabsSource).toContain('canScrollRight');
    expect(subtabsSource).toContain('sm:hidden');
  });

  it('charts object detail history in the detail itself, not behind a History tab', () => {
    // Storage disk and pool details had only Overview and History, so with the
    // charts in the one view they need no tab strip at all.
    for (const source of [diskDrawerOverviewSource, storagePoolDetailSource]) {
      expect(source).not.toContain('@/components/shared/Subtabs');
      expect(source).not.toContain("value: 'history', label: 'History'");
    }
    // Guest, node, Docker host and resource-detail drawers chart their history
    // on Overview itself, with the range picker beside the charts. A History
    // tab there repeated Overview's charts, or hid them a tab away.
    for (const source of [guestDrawerSource, nodeDrawerSource, dockerHostDrawerSource]) {
      expect(source).toContain('@/components/shared/Subtabs');
      expect(source).toContain("value: 'overview', label: 'Overview'");
      expect(source).not.toContain("value: 'history', label: 'History'");
    }
    expect(resourceDetailDrawerSource).not.toContain("activeTab() === 'history'");
    // Drawer charts draw triggers from the resource's own alerts; Needs
    // attention may list nested resources, the charts never do.
    expect(resourceDetailDrawerOverviewTabSource).toContain('getOwnAlertsForUnifiedResource(');
    expect(dockerHostDrawerOverviewSource).toContain('getOwnAlertsForUnifiedResource(');
    expect(dockerHostDrawerOverviewSource).toContain('getAlertAttentionCopy(alert, now)');
    expect(resourceDetailDrawerOverviewTabSource).toContain('getAlertAttentionCopy(alert, now)');
    // Range pickers name their options from one table, never a local list.
    expect(guestDrawerHistorySource).toContain('formatHistoryRangeOptionLabel(option)');
    expect(guestDrawerHistorySource).not.toContain('const formatRangeLabel');

    expect(resourceDetailDrawerSource).toContain('@/components/shared/Subtabs');
    expect(resourceDetailDrawerSource).toContain('tabs={drawer.tabs().map');

    // Pool and disk drawers chart with the shared drawer history, whose range
    // picker sits in its toolbar; neither keeps a storage-local picker.
    expect(storagePoolDetailSource).toContain('GuestDrawerHistory');
    expect(diskDrawerOverviewSource).toContain('GuestDrawerHistory');
    for (const storageDetailSource of [diskDrawerOverviewSource, storagePoolDetailSource]) {
      expect(storageDetailSource).not.toContain('activeTab()');
      expect(storageDetailSource).not.toContain('STORAGE_DETAIL_SELECT_CLASS');
      expect(storageDetailSource).not.toContain('STORAGE_DETAIL_HEADER_SELECT_CLASS');
    }

    expect(resourceDetailDrawerSource).not.toContain('listClass="gap-2! sm:gap-6!"');
    expect(resourceDetailDrawerSource).not.toContain(
      'tabClass="px-0.5! text-xs! sm:px-1! sm:text-sm!"',
    );
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
    expect(contextualFocusSource).toContain('markRouteStateDeliberateScroll');
    expect(contextualFocusSource).toContain('data-inline-detail-for');
    expect(summaryCardInteractionSource).toContain('resolveSummaryScopeState');
    // No table pins or previews a group scope since the Storage group pin went.
    expect(summaryCardInteractionSource).not.toContain('SummarySeriesGroupScope');

    expect(workloadSelectionStateSource).toContain('preserveScrollableAncestorVerticalOffset');
    expect(workloadSelectionStateSource).not.toContain('const scrollTop = scroller?.scrollTop');

    expect(summaryTableFocusSource).toContain('export function useSummaryTableFocusBridge');
    expect(summaryTableFocusSource).toContain('export function useSummaryPageInteractionState');
    expect(summaryTableFocusSource).toContain('resolveSummaryActiveSeriesId');
    expect(summaryTableFocusSource).toContain('focusedSeriesId');
    expect(summaryTableFocusSource).toContain('findInlineDetailElement');
    expect(summaryTableFocusSource).toContain('revealInlineDetailInViewport');
    expect(summaryTableFocusSource).toContain('MutationObserver');
    expect(summaryTableFocusSource).toContain(
      'untrack(() => revealFocusedSeries(root, focusedId))',
    );
    expect(summaryTableFocusSource).toContain('clearPinnedScope?: () => void;');
    expect(summaryTableFocusSource).toContain('onEscapeClear?: () => void;');
    expect(summaryTableFocusSource).toContain('setClearSurfaceRootRef');
    expect(summaryTableFocusSource).toContain('[data-summary-clear-ignore]');
    expect(summaryTableFocusSource).toContain("event.key !== 'Escape'");
    expect(summaryTableFocusSource).toContain('querySelector<HTMLElement>(');
    expect(summaryTableFocusSource).not.toContain('focusedGroupId');
    expect(summaryTableFocusSource).not.toContain('activeGroupScope');
    expect(summaryTableFocusSource).not.toContain("window.addEventListener('scroll'");
    expect(summaryTableFocusSource).not.toContain('useNavigate(');
  });

  it('keeps summary-linked table row emphasis on the shared active-row presentation contract', () => {
    expect(frontendIndexCssSource).toContain("tr[data-summary-row-active='true'] > td");
    expect(frontendIndexCssSource).toContain('--color-summary-row-bg');
    expect(frontendIndexCssSource).toContain('--color-summary-row-accent');
    expect(frontendIndexCssSource).not.toContain('data-summary-group-member-active');
    expect(frontendIndexCssSource).not.toContain('--color-summary-group-member-');
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
    expect(guestRowStateSource).not.toContain('bg-sky-50/70');
    expect(guestRowStateSource).not.toContain('ring-sky-400/25');

    for (const source of [storagePoolRowSource, diskListSource]) {
      expect(source).toContain('data-summary-row-active');
      expect(source).not.toContain('bg-sky-50/70');
      expect(source).not.toContain('ring-sky-400/25');
      expect(source).not.toContain('bg-blue-100 dark:bg-blue-800');
      expect(source).not.toContain('ring-blue-300 dark:ring-blue-600');
    }

    expect(storagePoolRowSource).not.toContain('data-summary-group-member-active');
    expect(storageGroupRowSource).toContain('STORAGE_GROUP_ROW_CLASS');
    expect(storageGroupPresentationSource).toContain('getInteractiveGroupedTableRowClass');
    expect(storageGroupPresentationSource).toContain('getGroupedTableRowCellClass');
    expect(nodeGroupHeaderSource).toContain('getGroupedTableRowClass');
    expect(nodeGroupHeaderSource).toContain('getGroupedTableRowCellClass');
    expect(workloadPanelSource).toContain('class={getGroupedTableRowClass()}');
    expect(workloadPanelSource).toContain('getGroupedTableRowCellClass');
    expect(alertHistoryTableGroupRowSource).toContain('getGroupedTableRowClass');
    expect(alertHistoryTableGroupRowSource).toContain('getGroupedTableRowCellClass');
    expect(alertHistoryTableGroupRowSource).not.toContain('class="bg-surface-alt"');
    expect(alertResourceTableDesktopSource).toContain('getGroupedTableRowClass');
    expect(alertResourceTableDesktopSource).toContain('getGroupedTableRowCellClass');
    expect(infrastructureSourceManagerSource).toContain('getGroupedTableRowClass');
    expect(infrastructureSourceManagerSource).toContain('getGroupedTableRowCellClass');
    expect(infrastructureSourceManagerSource).not.toContain('bg-base hover:bg-base');
  });

  it('routes Proxmox node version presentation through the shared formatter', () => {
    expect(proxmoxPageModelSource).toContain("from '@/utils/proxmoxVersion'");
    expect(proxmoxPageModelSource).toContain('formatProxmoxVersion(resource.proxmox?.pveVersion)');
    expect(proxmoxPageModelSource).not.toContain('pve-manager\\/');

    expect(proxmoxVersionSource).toContain('formatProxmoxVersion');
    expect(proxmoxVersionSource).toContain('pve-manager\\/');
    expect(proxmoxVersionSource).toContain("version.toLowerCase() === 'unknown'");
  });

  it('keeps product table scroll frames on the shared table shell', () => {
    expect(tableSource).toContain('wrapperClass');
    expect(tableSource).toContain(
      'table-scroll-shell w-full min-w-0 max-w-full overflow-x-auto touch-scroll',
    );
    expect(frontendIndexCssSource).toContain('.table-scroll-shell');
    expect(frontendIndexCssSource).toContain('container-type: inline-size');
    expect(frontendIndexCssSource).toContain('contain: paint');
    expect(frontendIndexCssSource).toContain('overflow-y: hidden');
    expect(frontendIndexCssSource).toContain('overscroll-behavior-x: contain');
    expect(frontendIndexCssSource).toContain('overscroll-behavior-y: chain');
    expect(frontendIndexCssSource).toContain('.table-scroll-shell.table-scroll-shell-phone-page');
    expect(frontendIndexCssSource).toContain('overflow: clip');
    expect(tableSource).toContain("phoneVerticalScrollOwner?: 'page' | 'table'");
    expect(tableSource).toContain('table-scroll-shell-phone-page');
    expect(windowedPageScrollSource).toContain(
      "addEventListener('scroll', options.onScroll, { passive: true })",
    );
    expect(windowedPageScrollSource).toContain(
      "addEventListener('wheel', options.onWheel, { passive: true })",
    );
    expect(windowedPageScrollSource).not.toContain("addEventListener('touch");
    for (const source of [
      platformWindowedItemsSource,
      storagePoolsTableWindowingSource,
      workloadViewportSyncSource,
    ]) {
      expect(source).toContain('bindWindowedPageScrollEvents');
      expect(source).not.toContain("addEventListener('touch");
    }
    expect(frontendIndexCssSource).toContain(
      '.table-scroll-shell > .table-fixed.platform-table th.platform-table-name-column',
    );
    expect(frontendIndexCssSource).toContain('@container (max-width: 22.499rem)');
    expect(frontendIndexCssSource).toContain('platform-table-narrow-hidden');
    expect(frontendIndexCssSource).not.toContain('white-space: normal;\n    -webkit-box-orient');
    expect(frontendIndexCssSource).toContain('th.truenas-app-name-column');
    expect(frontendIndexCssSource).toContain('th.truenas-app-containers-column');
    expect(frontendIndexCssSource).toContain('th.truenas-app-updates-column');
    expect(frontendIndexCssSource).not.toContain('th.kubernetes-service-name-column');
    expect(frontendIndexCssSource).not.toContain('th.kubernetes-network-name-column');
    expect(tableSource).toContain('w-full border-collapse text-left whitespace-nowrap');
    expect(tableCardSource).toContain('TABLE_CARD_FRAME_CLASS');
    expect(tableCardSource).toContain('overflow-hidden');
    expect(tableCardHeaderSource).toContain('TABLE_CARD_HEADER_CLASS');

    for (const source of [
      alertHistoryTableSectionSource,
      workloadsTableSource,
      storagePoolsTableSource,
      diskListSource,
      infrastructureSourceManagerSource,
      aiCostDashboardSource,
      proxmoxMailGatewayDrawerSource,
      pulseDataGridSource,
      k8sNamespacesDrawerSource,
    ]) {
      expect(source).toContain('<Table');
      expect(source).not.toContain('<div class="overflow-x-auto">');
      expect(source).not.toContain('<div class="overflow-x-auto bg-surface">');
      expect(source).not.toContain('<div class="overflow-auto max-h-[600px]">');
    }

    for (const source of [
      alertHistoryTableSectionSource,
      workloadsTableSource,
      storageContentCardSource,
    ]) {
      expect(source).toContain('<TableCard');
    }

    expect(storageContentCardSource).toContain('TableCardHeader');

    expect(alertHistoryTableSectionSource).not.toContain(
      'overflow-hidden rounded-sm border border-border',
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

  it('keeps sortable table direction markers active-only and shared', () => {
    expect(tableSortPresentationSource).toContain('export function getTableSortIndicator');
    expect(tableSortPresentationSource).toContain('if (!active) return null');

    for (const source of [
      sharedPlatformPageSource,
      workloadTableHeaderSource,
      proxmoxBackupsTableSharedSource,
    ]) {
      expect(source).toContain('getTableSortIndicator');
    }

    expect(proxmoxBackupsTableSharedSource).not.toContain('ArrowUpDownIcon');
    expect(proxmoxBackupsTableSharedSource).not.toContain('SORT_ICON_CLASS');
    expect(storagePoolsTableSource).not.toContain('getTableSortIndicator');
    expect(storagePoolsTableSource).not.toContain('aria-sort');
    expect(storagePoolsTableSource).not.toContain('HEADER_SORT_BUTTON');
  });

  it('keeps platform sort activation native and responsive labels stable at the shared boundary', () => {
    const heading = sharedPlatformPageSource
      .split('export function PlatformSortableTableHead')[1]!
      .split('export const formatPlatformTableTextValue')[0]!;
    expect(heading).toContain('scope="col"');
    expect(heading).toContain('children(() => props.children)');
    expect(heading).toContain('<button');
    expect(heading).toContain('type="button"');
    expect(heading).toContain('focus-visible:ring-inset');
    expect(heading).toContain('[text-align:inherit] [text-transform:inherit]');
    expect(heading).toContain('<span aria-hidden="true">');
    // Native keyboard clicks bubble to the one cell handler. A second key
    // handler would double-advance Enter or Space through the sort cycle.
    expect(heading).not.toContain('onKeyDown=');
    expect(heading.match(/onClick=/g)).toHaveLength(1);
  });

  it('keeps the retired summary chart-visibility toggle out of the shared toolbar', () => {
    // The workload summary chart section this toggle controlled is deleted and
    // no other page exposes a chart show/hide control. A returning summary
    // section is a governed product decision that must register its own
    // primitive.
    const registry = JSON.parse(sharedTemplateRegistrySource) as {
      rules?: Array<{ id: string; canonical?: { export?: string } }>;
      patternGuards?: Array<{ id: string; canonical?: { export?: string } }>;
    };
    const retiredIds = ['chart-visibility-toggle-button', 'chart-visibility-local-toggle-labels'];
    for (const entry of [...(registry.rules ?? []), ...(registry.patternGuards ?? [])]) {
      expect(retiredIds).not.toContain(entry.id);
      expect(entry.canonical?.export).not.toBe('ChartVisibilityToggleButton');
    }

    expect(filterToolbarSource).not.toContain('ChartVisibilityToggleButton');
    expect(workloadsFilterSource).not.toContain('ChartVisibilityToggleButton');
    expect(workloadsFilterSource).not.toContain('onChartsToggle');
    expect(workloadsFilterSource).not.toContain('chartsCollapsed');
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
    expect(summaryRowActionButtonSource).toContain('hideWhenRowTappableOnMobile: boolean');
    expect(summaryRowActionButtonSource).toContain('sm:not-sr-only');
    expect(platformResourceDetailTableRowSource).toContain('PlatformResourceDetailToggleButton');
    expect(platformResourceDetailTableRowSource).toContain('SummaryRowActionButton');
    expect(platformResourceDetailTableRowSource).toContain('hideWhenRowTappableOnMobile');
    // A collapsed resource inspector uses the canonical drawer disclosure.
    expect(platformResourceDetailTableRowSource).toContain('ResourceDetailDrawerSupportDisclosure');

    for (const source of [guestRowSource, storagePoolRowSource, diskListSource]) {
      expect(source).toContain('createSummaryInteractiveRowPreviewHandlers');
      expect(source).toContain('SummaryRowActionButton');
      expect(source).toContain('hideWhenRowTappableOnMobile');
    }

    // Storage group headers are disclosure rows: no hover preview, no group pin.
    expect(storageGroupRowSource).not.toContain('createSummaryInteractiveRowPreviewHandlers');
    expect(storageGroupRowSource).toContain('SummaryRowActionButton');
    expect(storageGroupRowSource).toContain('hideWhenRowTappableOnMobile');
    expect(storageGroupRowSource).not.toContain('hideWhenRowTappableOnMobile={false}');
    expect(storageGroupRowSource).toContain('onClick={() => props.onToggle()}');
    expect(storageGroupRowSource).not.toContain('onFocusChange');

    expect(workloadPanelSource).not.toContain('kind="scope"');
    expect(storageGroupRowSource).not.toContain('kind="scope"');

    expect(nodeGroupHeaderSource).toContain('ResourceNameWithWebInterfaceLink');
    expect(webInterfaceLinkSource).toContain('event.stopPropagation()');
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

  it('keeps tag badges CSP-safe', () => {
    expect(tagBadgesSource).toContain('data-tag-dot="true"');
    expect(tagBadgesSource).toContain('data-tag-dot-fill="true"');
    expect(tagBadgesSource).toContain('data-tag-dot-ring="true"');
    expect(tagBadgesSource).not.toContain('style={');
    expect(tagBadgesSource).not.toContain('box-shadow');
    expect(tagBadgesSource).not.toContain('background-color');
  });

  it('keeps provider summary rows on the canonical single-line density contract', () => {
    expect(frontendIndexCssSource).toContain(
      '.table-fixed.platform-table > tbody > tr:not([data-inline-detail-for])',
    );
    expect(frontendIndexCssSource).toContain('height: 2rem;');
    expect(frontendIndexCssSource).not.toContain(
      '.pulse-wide-data-surface .host-row,\n    .pulse-wide-data-surface .workload-row',
    );

    for (const source of [
      dockerAlertsTableSource,
      kubernetesAlertsTableSource,
      proxmoxBackupServersTableSource,
      proxmoxCephTableSource,
      proxmoxNodesTableSource,
      proxmoxRecoverableTableSource,
      agentsMachinesTableSource,
      truenasAlertsTableSource,
      truenasAppsTableSource,
      truenasNetworkSharesTableSource,
      truenasProtectionTableSource,
      truenasStorageTopologyTableSource,
      truenasSystemsTableSource,
      truenasVirtualMachinesTableSource,
      vsphereActivityTableSource,
      vsphereAlertsTableSource,
    ]) {
      expect(source).not.toContain('block truncate text-[10px] text-muted');
      expect(source).not.toContain('flex min-w-0 flex-1 flex-col');
    }

    expect(agentsMachinesTableSource).not.toContain('grid-rows-2');
  });

  it('keeps platform count grammar and route status normalization shared', () => {
    expect(sharedPlatformPageSource).toContain('export const getPlatformResourceCountNoun');
    expect(sharedPlatformPageSource).toContain(
      'export const normalizePlatformResourceStatusFilter',
    );
  });

  it('keeps table-local attention counts on the shared platform filter template', () => {
    expect(sharedPlatformPageSource).toContain('export function withPlatformStatusCounts');
    expect(sharedPlatformPageSource).toContain(
      'count: inventoryCountsVisible() ? option.count : undefined',
    );

    for (const source of [truenasProtectionTableSource, vsphereAlertsTableSource]) {
      expect(source).toContain('withPlatformStatusCounts');
      expect(source).not.toContain('PlatformAttentionSummary');
      expect(source).not.toContain('<MetadataBadge');
    }

    expect(kubernetesPageSurfaceSource).not.toContain('PlatformAttentionSummary');
    expect(kubernetesPageSurfaceSource).toContain('attentionCount={workloadAttention()}');
  });

  it('keeps progress bars CSP-safe in the shared primitive owner', () => {
    expect(progressBarSource).toContain('data-progress-fill');
    expect(progressBarSource).toContain('foreignObject');
    expect(progressBarSource).toContain('progress-fill-frame');
    expect(progressBarSource).not.toContain('style={{');
    expect(progressBarSource).not.toContain('style={');
    expect(frontendIndexCssSource).toContain('.progress-fill-frame');
    expect(frontendIndexCssSource).toContain('@media (prefers-reduced-motion: reduce)');
    expect(detailSectionModelSource).toContain('export type DetailRowProgress');
    expect(detailSectionTableSource).toContain('row.progress');
    expect(detailSectionTableSource).toContain('<ProgressBar');
    expect(detailSectionTableSource).not.toContain('style={{');
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

  it('disables login entrance animations when reduced motion is requested', () => {
    expect(frontendIndexCssSource).toMatch(
      /@media \(prefers-reduced-motion: reduce\) \{[\s\S]*?\.animate-fade-in,[\s\S]*?\.animate-slide-up \{[\s\S]*?animation: none;/,
    );
  });

  it('bounds all product motion when reduced motion is requested', () => {
    expect(frontendIndexCssSource).toMatch(
      /@media \(prefers-reduced-motion: reduce\) \{[\s\S]*?\*,[\s\S]*?\*::before,[\s\S]*?\*::after \{[\s\S]*?animation-duration: 0\.01ms !important;[\s\S]*?animation-iteration-count: 1 !important;[\s\S]*?transition-duration: 0\.01ms !important;/,
    );
    expect(frontendIndexCssSource).toMatch(/html:focus-within \{[\s\S]*?scroll-behavior: auto;/);
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

  it('keeps shared navigation and row actions usable at phone widths', () => {
    expect(subtabsSource).toContain('overflow-x-auto');
    expect(subtabsSource).toContain('whitespace-nowrap');
    expect(subtabsSource).toContain('min-h-9');
    expect(subtabsSource).toContain('sm:min-h-10');
    expect(summaryRowActionButtonSource).toContain('h-9 w-9');
    // A visible phone chevron keeps its 36px target without setting the
    // row height: rows with one stay as tall as the 32px rows beside them.
    expect(summaryRowActionButtonSource).toContain('-mx-1 -my-2.5');
    expect(summaryRowActionButtonSource).toContain('sm:m-0');
    expect(searchFieldSource).toContain('min-h-11');
    expect(filterToolbarSource).toContain('min-h-11');
    // Phone and tablet drawers fill the table's visible width (the scroll
    // shell is a size container), not a viewport guess at the page chrome.
    expect(inlineDetailTableRowSource).toContain('max-w-[100cqi]');
    expect(inlineDetailTableRowSource).not.toContain('100vw-3.5rem');
  });

  it('keeps shared settings panels dense without forking their desktop framing', () => {
    expect(settingsPanelSource).toContain('data-settings-panel');
    expect(settingsPanelSource).toContain('px-2.5 py-2 sm:px-6 sm:py-4');
    expect(settingsPanelSource).toContain("'p-2.5 sm:p-6'");
    expect(settingsPanelSource).toContain("'space-y-3 sm:space-y-6'");
    expect(featureGateSectionSource).toContain('flex items-center gap-2.5 sm:gap-4');
    expect(featureGateSectionSource).toContain('line-clamp-2 text-[11px]');
  });

  it('keeps object drawers operator-first across platform implementations', () => {
    expect(drawerAttentionSectionSource).toContain('item.subject');
    expect(drawerAttentionSectionSource).toContain('item.metric');
    expect(drawerAttentionSectionSource).toContain('formatAlertSeverityLabel');
    expect(drawerAttentionSectionSource).toContain('getAlertSeverityBadgeClass');
    expect(drawerAttentionSectionSource).toContain('Show fewer alerts');
    expect(drawerAttentionSectionSource).toContain('aria-expanded={expanded()}');
    expect(drawerAttentionSectionSource).not.toContain('DetailSectionTable');
    // A threshold alert held under its trigger leads with the live reading and
    // a second line saying what clears it, never only its stale breach text.
    expect(drawerAttentionSectionSource).toContain('item.detail');
    expect(drawerAttentionSectionSource).toContain('item.title ?? item.message');
    for (const liveAlertConsumer of [
      'src/components/Workloads/NodeDrawerOverview.tsx',
      'src/features/proxmox/ProxmoxCephClusterDrawer.tsx',
    ]) {
      const source = readFrontendSource(liveAlertConsumer);
      expect(source).toContain('...getAlertAttentionCopy(alert, now)');
      expect(source).not.toContain('message: alert.message');
    }
    expect(readFrontendSource('src/components/Workloads/guestDrawerModel.ts')).toContain(
      'getAlertAttentionCopy(alert, now)',
    );

    const overviewConsumers = [
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
      'src/components/Workloads/GuestDrawerOverview.tsx',
      'src/components/Workloads/NodeDrawerOverview.tsx',
      'src/features/docker/DockerHostDrawerOverview.tsx',
    ];
    const manageConsumers = [
      'src/components/Infrastructure/ResourceDetailDrawer.tsx',
      'src/components/Workloads/GuestDrawer.tsx',
      'src/components/Workloads/NodeDrawer.tsx',
      'src/features/docker/DockerHostDrawer.tsx',
    ];
    const manageContentOwners = [
      'src/components/Infrastructure/ResourceDetailDrawer.tsx',
      'src/components/Workloads/GuestDrawerManage.tsx',
      'src/components/Workloads/NodeDrawer.tsx',
      'src/features/docker/DockerHostDrawer.tsx',
    ];

    for (const consumerPath of overviewConsumers) {
      const source = readFrontendSource(consumerPath);
      expect(source).toContain('DrawerAttentionSection');
      // Warning rows lift in front; the rest waits under one Details disclosure.
      expect(source).toContain('DrawerExceptionList');
      expect(source).toContain('<TechnicalDetailsDisclosure');
      expect(source).not.toContain('ResourceOperatorStateSection');
    }

    const nodeOverviewSource = readFrontendSource(
      'src/components/Workloads/NodeDrawerOverview.tsx',
    );
    expect(nodeOverviewSource).toContain('subject:');
    expect(nodeOverviewSource).toContain('metric: alertTypeDisplayLabel(alert.type)');

    for (const consumerPath of manageConsumers) {
      const source = readFrontendSource(consumerPath);
      expect(source).toContain("'manage'");
    }

    for (const ownerPath of manageContentOwners) {
      const source = readFrontendSource(ownerPath);
      expect(source).toContain('ResourceOperatorStateSection');
    }

    for (const headerPath of [
      'src/components/Infrastructure/ResourceDetailDrawer.tsx',
      'src/components/Workloads/GuestDrawer.tsx',
    ]) {
      const source = readFrontendSource(headerPath);
      expect(source).not.toContain('Ask Pulse Assistant about');
      expect(source).not.toContain('Copy Pulse context for');
      expect(source).not.toContain('copyAgentContext');
    }

    for (const statePath of [
      'src/components/Infrastructure/useResourceDetailDrawerState.ts',
      'src/components/Workloads/useGuestDrawerState.ts',
    ]) {
      const source = readFrontendSource(statePath);
      expect(source).not.toContain('AgentContextAPI');
      expect(source).not.toContain('formatAgentResourceContextForClipboard');
    }

    const disclosureSource = readFrontendSource(
      'src/components/shared/TechnicalDetailsDisclosure.tsx',
    );
    expect(disclosureSource).toContain('onToggle');
    // Lazily mounted by default; keepMounted is an explicit opt-in for a
    // disclosure that holds a surface's complete record (drawer Details).
    expect(disclosureSource).toContain('<Show when={props.keepMounted || expanded()}>');
    expect(disclosureSource).toContain('keepMounted?: boolean;');
    // The drawer's compact Worth checking list reuses the detail-row tone
    // mapping rather than keeping a local palette that could drift.
    expect(readFrontendSource('src/components/shared/DetailSectionTable.tsx')).toContain(
      'export const detailValueToneClass',
    );
    const exceptionListSource = readFrontendSource(
      'src/components/Workloads/DrawerExceptionList.tsx',
    );
    expect(exceptionListSource).toContain('detailValueToneClass(row.tone)');
    expect(exceptionListSource).not.toMatch(/text-(amber|rose|red)-700/);
    expect(disclosureSource).toContain('sections?: DetailSection[]');
    expect(disclosureSource).toContain('<DetailSectionTable');
    expect(disclosureSource).toContain('export const TechnicalDetailsSection');

    for (const overviewPath of [
      'src/components/Workloads/GuestDrawerOverview.tsx',
      'src/components/Workloads/NodeDrawerOverview.tsx',
      'src/features/docker/DockerHostDrawerOverview.tsx',
    ]) {
      const source = readFrontendSource(overviewPath);
      expect(source).toMatch(/<TechnicalDetails(Section|Disclosure)\b/);
      expect(source).toContain('...technicalSections()');
      expect(source).not.toContain('const DetailCard');
    }

    const resourceOverviewSource = readFrontendSource(
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
    );
    expect(resourceOverviewSource).toContain("content: 'all'");

    const registry = JSON.parse(readFrontendSource('scripts/shared-template-registry.json')) as {
      rules?: Array<{ id: string; requiredConsumers?: Array<{ path: string }> }>;
    };
    for (const ruleId of ['operator-first-drawer-attention', 'operator-first-technical-section']) {
      const rule = registry.rules?.find((candidate) => candidate.id === ruleId);
      expect(rule?.requiredConsumers?.map((consumer) => consumer.path)).toEqual(overviewConsumers);
    }
  });
});

// The production server sends `style-src 'self' 'nonce-...'` without
// 'unsafe-inline' (internal/api/security.go), so the browser reports every
// style attribute that arrives as markup. Solid compiles static style values
// (`style="..."`, or any literal entry of a `style={{ ... }}` object) into the
// element's template HTML, which is such markup: each render logs a CSP
// violation, and whether the style still applies is up to the browser. The Vite
// dev server allows inline styles, so this only shows in a production build.
// Dynamic style values are applied through CSSOM at runtime and are fine.
const componentRuntimeSources = import.meta.glob(
  ['../../**/*.tsx', '!../../**/__tests__/**', '!../../**/*.test.tsx'],
  { query: '?raw', eager: true, import: 'default' },
) as Record<string, string>;

const compileSolidTemplates = (source: string, filename: string): string[] => {
  // Same Solid preset options vite-plugin-solid uses for the client build.
  const result = transformSync(source, {
    filename,
    babelrc: false,
    configFile: false,
    ast: true,
    code: false,
    parserOpts: { plugins: ['jsx', 'typescript'] },
    presets: [[solidPreset, { generate: 'dom', hydratable: false }]],
  });
  const templates: string[] = [];
  if (!result?.ast) return templates;

  const templateHelpers = new Set<string>();
  traverse(result.ast, {
    ImportSpecifier(path) {
      const imported = path.node.imported;
      const importedName = t.isIdentifier(imported) ? imported.name : imported.value;
      const parent = path.parentPath.node;
      if (
        importedName === 'template' &&
        t.isImportDeclaration(parent) &&
        parent.source.value === 'solid-js/web'
      ) {
        templateHelpers.add(path.node.local.name);
      }
    },
  });
  if (templateHelpers.size === 0) return templates;

  traverse(result.ast, {
    CallExpression(path) {
      const { callee, arguments: args } = path.node;
      if (!t.isIdentifier(callee) || !templateHelpers.has(callee.name)) return;
      const [html] = args;
      if (t.isTemplateLiteral(html)) {
        templates.push(html.quasis.map((quasi) => quasi.value.cooked ?? '').join(''));
      } else if (t.isStringLiteral(html)) {
        templates.push(html.value);
      }
    },
  });
  return templates;
};

// Style attributes and <style> elements, including those inside nested
// <template> content, which querySelectorAll does not enter.
const collectInlineStyleMarkup = (root: ParentNode): string[] =>
  Array.from(root.querySelectorAll('[style], style, template')).flatMap((element) => {
    const found: string[] = [];
    if (element.hasAttribute('style')) {
      found.push(`<${element.localName} style="${element.getAttribute('style')}">`);
    }
    if (element.localName === 'style') found.push('<style>');
    if (element instanceof HTMLTemplateElement) {
      found.push(...collectInlineStyleMarkup(element.content));
    }
    return found;
  });

const findTemplateStyleAttributes = (source: string, filename: string): string[] =>
  compileSolidTemplates(source, filename).flatMap((html) => {
    const template = document.createElement('template');
    template.innerHTML = html;
    return collectInlineStyleMarkup(template.content);
  });

describe('production CSP inline style guardrails', () => {
  it('detects the static style forms Solid compiles into template markup', () => {
    const fixture = `
      export const Fixture = (props: { width: number; shown: boolean }) => (
        <section>
          <div style={{ 'overflow-anchor': 'none' }} />
          <span style="animation-delay: 120ms" />
          <p style={{ 'min-width': '0', width: \`\${props.width}px\` }} />
          <b style={{ width: \`\${props.width}px\` }} />
          <i style={props.shown ? 'opacity: 1' : undefined} />
          <em class="[overflow-anchor:none]" />
          <u STYLE="color: blue" />
          <template>
            <s style="color: red" />
          </template>
          <style>{'.fixture { color: green; }'}</style>
        </section>
      );
    `;

    expect(findTemplateStyleAttributes(fixture, 'Fixture.tsx')).toEqual([
      '<div style="overflow-anchor:none">',
      '<span style="animation-delay:120ms">',
      '<p style="min-width:0">',
      '<u style="color:blue">',
      '<s style="color:red">',
      '<style>',
    ]);
  });

  // Use a class for static styling instead: a Tailwind utility, or an
  // arbitrary property such as `[overflow-anchor:none]`. Only files that
  // mention style can emit one, and compiling those takes a few seconds, or
  // tens of seconds on a loaded machine.
  it('keeps static style attributes out of every runtime component template', () => {
    const paths = Object.keys(componentRuntimeSources);
    expect(paths.length).toBeGreaterThan(100);

    const offenders = paths
      .filter((path) => /\bstyle\b/i.test(componentRuntimeSources[path]))
      .flatMap((path) =>
        findTemplateStyleAttributes(componentRuntimeSources[path], path).map(
          (attribute) => `${path}: ${attribute}`,
        ),
      )
      .sort();

    expect(offenders).toEqual([]);
  }, 120_000);
});
