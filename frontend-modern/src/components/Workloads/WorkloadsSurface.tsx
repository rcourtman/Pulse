import { Show } from 'solid-js';
import { EmptyState } from '@/components/shared/EmptyState';
import { TableCard } from '@/components/shared/TableCard';
import { WorkloadsFilter } from './WorkloadsFilter';
import {
  DEFAULT_WORKLOADS_VIEW_MODE,
  hasActiveWorkloadsFilters,
} from './workloadsFilterModel';
import { WorkloadsTable } from './WorkloadsTable';
import {
  useWorkloadsState,
  type WorkloadsState,
  type WorkloadsSurfaceProps,
} from './useWorkloadsState';
export type { WorkloadsSurfaceProps } from './useWorkloadsState';

interface WorkloadsSurfaceComponentProps extends WorkloadsSurfaceProps {
  emptyStateDescription?: string;
  emptyStateTitle?: string;
  state?: WorkloadsState;
}

export function WorkloadsSurface(props: WorkloadsSurfaceComponentProps) {
  const state = props.state ?? useWorkloadsState(props);
  const tableOnlyFiltersActive = () =>
    hasActiveWorkloadsFilters({
      search: state.search(),
      viewMode: props.forcedViewMode !== undefined ? DEFAULT_WORKLOADS_VIEW_MODE : state.viewMode(),
      statusMode: state.statusMode(),
      hostFilterValue: state.hostFilterConfig()?.value,
      platformFilterValue: state.platformFilterConfig()?.value,
      namespaceFilterValue: state.namespaceFilterConfig()?.value,
      containerRuntimeFilterValue: state.containerRuntimeFilterConfig()?.value,
    });
  const tableOnlyEmptyState = () => {
    if (tableOnlyFiltersActive()) {
      return state.workloadsGuestsEmptyState();
    }

    return {
      title: props.emptyStateTitle ?? 'No workloads',
      description:
        props.emptyStateDescription ?? 'Workloads appear here when inventory is available.',
    };
  };

  return (
    <div ref={state.setClearSurfaceRootRef} class="space-y-3" data-testid="workloads-page">
      <div class="space-y-3" data-testid="workloads-interaction-surface">
        <Show
          when={
            !props.suppressFilterToolbar &&
            !state.kioskMode() &&
            state.surfaceConnected() &&
            state.surfaceInitialDataReceived() &&
            state.allGuests().length > 0
          }
        >
          <div data-summary-clear-ignore>
            <WorkloadsFilter
              savedViewsKey={state.savedViewsKey()}
              search={state.search}
              setSearch={state.setSearch}
              viewMode={state.viewMode}
              setViewMode={state.setViewMode}
              statusMode={state.statusMode}
              setStatusMode={state.setStatusMode}
              groupingMode={state.groupingMode}
              setGroupingMode={state.setGroupingMode}
              defaultSortKey={props.defaultSortKey}
              setSortKey={state.setSortKey}
              setSortDirection={state.setSortDirection}
              onBeforeAutoFocus={state.handleBeforeAutoFocus}
              ariaLabel={props.filterAriaLabel}
              searchPlaceholder={props.filterSearchPlaceholder}
              searchEmptyMessage={props.filterSearchEmptyMessage}
              statusOptions={props.filterStatusOptions}
              columnVisibility={state.workloadsFilterColumnVisibility()}
              containerRuntimeFilter={state.containerRuntimeFilterConfig()}
              hostFilter={state.hostFilterConfig()}
              namespaceFilter={state.namespaceFilterConfig()}
              platformFilter={state.platformFilterConfig()}
              suppressTypeFilter={props.forcedViewMode !== undefined}
              metricDisplayMode={state.workloadMetricDisplayMode}
              setMetricDisplayMode={state.setWorkloadMetricDisplayMode}
              metricHistoryRange={state.workloadMetricHistoryRange}
              setMetricHistoryRange={state.setWorkloadMetricHistoryRange}
              forcedPlatform={props.forcedPlatform}
              pinnedSelectionActive={() =>
                Boolean(state.selectedGuestId() || state.focusedSummaryWorkloadGroupId())
              }
              onClearPinnedSelection={state.clearPinnedSummaryScope}
            />
          </div>
        </Show>

        <Show
          when={
            state.surfaceConnected() &&
            state.surfaceInitialDataReceived() &&
            state.filteredGuests().length > 0
          }
        >
          <WorkloadsTable
            activeAlerts={state.activeAlerts}
            alertsEnabled={state.alertsEnabled}
            bottomSpacerHeight={state.bottomSpacerHeight}
            compactGroupHeaders={state.compactGroupHeaders}
            getGroupLabel={state.getGroupLabel}
            groupedGuests={state.groupedGuests}
            groupedWindowing={state.groupedWindowing}
            groupLabelBadges={state.groupLabelBadges}
            guestMetadata={state.guestMetadata}
            guestParentNodeMap={state.guestParentNodeMap}
            groupNodeDrawerMode={state.groupNodeDrawerMode}
            groupingMode={state.groupingMode}
            handleCustomUrlUpdate={state.handleCustomUrlUpdate}
            handleSort={state.handleSort}
            handleTagClick={state.handleTagClick}
            activeSummaryWorkloadGroupScope={state.activeSummaryWorkloadGroupScope}
            activeSummaryWorkloadId={state.activeSummaryWorkloadId}
            focusedSummaryWorkloadGroupScope={state.focusedSummaryWorkloadGroupScope}
            focusedSummaryWorkloadGroupId={state.focusedSummaryWorkloadGroupId}
            hoveredSummaryWorkloadGroupScope={state.hoveredSummaryWorkloadGroupScope}
            isMobile={state.isMobile}
            nodeByInstance={state.nodeByInstance}
            search={state.search}
            selectedGuestId={state.selectedGuestId}
            setFocusedWorkloadGroupScope={state.setFocusedWorkloadGroupScope}
            setHoveredWorkloadGroupScope={state.setHoveredWorkloadGroupScope}
            setHoveredWorkloadId={state.setHoveredWorkloadId}
            setSelectedGuestId={state.setSelectedGuestId}
            setTableRootRef={state.setTableRootRef}
            setTableBodyRef={state.setTableBodyRef}
            setTableWrapperRef={state.setTableWrapperRef}
            sortDirection={state.sortDirection}
            sortKey={state.sortKey}
            topSpacerHeight={state.topSpacerHeight}
            totalColumns={state.totalColumns}
            visibleColumns={state.visibleColumns}
            visibleGroupKeys={state.visibleGroupKeys}
            windowedGroupedGuests={state.windowedGroupedGuests}
            workloadIOEmphasis={state.workloadIOEmphasis}
            workloadMetricDisplayMode={state.workloadMetricDisplayMode}
            workloadMetricHistory={state.workloadMetricHistory}
            workloadTableLayoutMode={state.workloadTableLayoutMode}
            workloadTableVisibleColumnIds={state.workloadTableVisibleColumnIds}
            workloadTableVisibleColumns={state.workloadTableVisibleColumns}
          />
        </Show>
        <Show
          when={
            state.surfaceConnected() &&
            state.surfaceInitialDataReceived() &&
            state.filteredGuests().length === 0
          }
        >
          <TableCard>
            <div class="p-6">
              <EmptyState
                title={tableOnlyEmptyState().title}
                description={tableOnlyEmptyState().description}
              />
            </div>
          </TableCard>
        </Show>
      </div>
    </div>
  );
}
