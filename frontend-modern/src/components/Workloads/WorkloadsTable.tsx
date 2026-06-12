import { For } from 'solid-js';

import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { Table } from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';

import { getGuestColumnWidthStyle } from './guestRowModel';
import type { WorkloadsState } from './useWorkloadsState';
import { WorkloadPanel } from './WorkloadPanel';
import { WorkloadTableHeader } from './WorkloadTableHeader';

type WorkloadsTableProps = Pick<
  WorkloadsState,
  | 'activeAlerts'
  | 'alertsEnabled'
  | 'bottomSpacerHeight'
  | 'compactGroupHeaders'
  | 'getGroupLabel'
  | 'groupedGuests'
  | 'groupedWindowing'
  | 'groupLabelBadges'
  | 'guestMetadata'
  | 'guestParentNodeMap'
  | 'groupNodeDrawerMode'
  | 'groupingMode'
  | 'handleCustomUrlUpdate'
  | 'handleSort'
  | 'handleTagClick'
  | 'activeSummaryWorkloadGroupScope'
  | 'activeSummaryWorkloadId'
  | 'focusedSummaryWorkloadGroupScope'
  | 'focusedSummaryWorkloadGroupId'
  | 'hoveredSummaryWorkloadGroupScope'
  | 'isMobile'
  | 'nestedWorkloadContextByGuestId'
  | 'nodeByInstance'
  | 'search'
  | 'selectedGuestId'
  | 'setFocusedWorkloadGroupScope'
  | 'setHoveredWorkloadGroupScope'
  | 'setHoveredWorkloadId'
  | 'setSelectedGuestId'
  | 'setTableRootRef'
  | 'setTableBodyRef'
  | 'setTableWrapperRef'
  | 'sortDirection'
  | 'sortKey'
  | 'topSpacerHeight'
  | 'totalColumns'
  | 'visibleColumns'
  | 'visibleGroupKeys'
  | 'windowedGroupedGuests'
  | 'workloadIOEmphasis'
  | 'workloadMetricDisplayMode'
  | 'workloadMetricHistory'
  | 'workloadTableLayoutMode'
  | 'workloadTableVisibleColumnIds'
  | 'workloadTableVisibleColumns'
>;

export function WorkloadsTable(props: WorkloadsTableProps) {
  return (
    <ComponentErrorBoundary name="Guest Table">
      <TableCard
        ref={props.setTableRootRef}
        class="mb-4 rounded-md"
        data-summary-clear-surface
        data-testid="workloads-table-surface"
      >
        <Table
          wrapperRef={props.setTableWrapperRef}
          class={`workload-table min-w-full table-fixed ${props.isMobile() ? 'workload-table--mobile' : 'workload-table--desktop'}`}
        >
          <colgroup>
            <For each={props.workloadTableVisibleColumns()}>
              {(column) => (
                <col
                  data-workload-col={column.id}
                  style={getGuestColumnWidthStyle(
                    column.id,
                    props.isMobile(),
                    props.workloadTableLayoutMode(),
                    props.workloadTableVisibleColumnIds(),
                  )}
                />
              )}
            </For>
          </colgroup>
          <WorkloadTableHeader
            handleSort={props.handleSort}
            isMobile={props.isMobile}
            sortDirection={props.sortDirection}
            sortKey={props.sortKey}
            visibleColumns={props.visibleColumns}
            workloadTableLayoutMode={props.workloadTableLayoutMode}
            workloadTableVisibleColumnIds={props.workloadTableVisibleColumnIds}
            workloadTableVisibleColumns={props.workloadTableVisibleColumns}
          />
          <WorkloadPanel
            activeAlerts={props.activeAlerts}
            alertsEnabled={props.alertsEnabled}
            bottomSpacerHeight={props.bottomSpacerHeight}
            compactGroupHeaders={props.compactGroupHeaders}
            getGroupLabel={props.getGroupLabel}
            groupedGuests={props.groupedGuests}
            groupedWindowing={props.groupedWindowing}
            groupLabelBadges={props.groupLabelBadges}
            guestMetadata={props.guestMetadata}
            guestParentNodeMap={props.guestParentNodeMap}
            groupNodeDrawerMode={props.groupNodeDrawerMode}
            groupingMode={props.groupingMode}
            handleCustomUrlUpdate={props.handleCustomUrlUpdate}
            handleTagClick={props.handleTagClick}
            activeSummaryWorkloadGroupScope={props.activeSummaryWorkloadGroupScope}
            activeSummaryWorkloadId={props.activeSummaryWorkloadId}
            focusedSummaryWorkloadGroupScope={props.focusedSummaryWorkloadGroupScope}
            focusedSummaryWorkloadGroupId={props.focusedSummaryWorkloadGroupId}
            hoveredSummaryWorkloadGroupScope={props.hoveredSummaryWorkloadGroupScope}
            isMobile={props.isMobile}
            nestedWorkloadContextByGuestId={props.nestedWorkloadContextByGuestId}
            nodeByInstance={props.nodeByInstance}
            search={props.search}
            selectedGuestId={props.selectedGuestId}
            setFocusedWorkloadGroupScope={props.setFocusedWorkloadGroupScope}
            setHoveredWorkloadGroupScope={props.setHoveredWorkloadGroupScope}
            setHoveredWorkloadId={props.setHoveredWorkloadId}
            setSelectedGuestId={props.setSelectedGuestId}
            setTableBodyRef={props.setTableBodyRef}
            topSpacerHeight={props.topSpacerHeight}
            totalColumns={props.totalColumns}
            visibleGroupKeys={props.visibleGroupKeys}
            windowedGroupedGuests={props.windowedGroupedGuests}
            workloadIOEmphasis={props.workloadIOEmphasis}
            workloadMetricDisplayMode={props.workloadMetricDisplayMode}
            workloadMetricHistory={props.workloadMetricHistory}
            workloadTableLayoutMode={props.workloadTableLayoutMode}
            workloadTableVisibleColumnIds={props.workloadTableVisibleColumnIds}
            workloadTableVisibleColumns={props.workloadTableVisibleColumns}
          />
        </Table>
      </TableCard>
    </ComponentErrorBoundary>
  );
}
