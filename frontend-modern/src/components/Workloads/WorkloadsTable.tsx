import { For } from 'solid-js';

import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
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
  | 'getGroupLabel'
  | 'groupedGuests'
  | 'groupedWindowing'
  | 'guestMetadata'
  | 'guestParentNodeMap'
  | 'groupingMode'
  | 'handleCustomUrlUpdate'
  | 'handleSort'
  | 'handleTagClick'
  | 'activeSummaryWorkloadGroupScope'
  | 'activeSummaryWorkloadId'
  | 'clearPinnedSummaryScope'
  | 'focusedSummaryWorkloadGroupScope'
  | 'focusedSummaryWorkloadGroupId'
  | 'hoveredSummaryWorkloadGroupScope'
  | 'isMobile'
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
  | 'workloadTableLayoutMode'
  | 'workloadTableVisibleColumnIds'
  | 'workloadTableVisibleColumns'
>;

export function WorkloadsTable(props: WorkloadsTableProps) {
  const showClearSelection = () =>
    Boolean(props.selectedGuestId() || props.focusedSummaryWorkloadGroupId());

  return (
    <ComponentErrorBoundary name="Guest Table">
      <TableCard
        ref={props.setTableRootRef}
        class="mb-4 rounded-md"
        data-summary-clear-surface
        data-testid="workloads-table-surface"
      >
        <TableCardHeader
          title="Workloads"
          showClearAction={showClearSelection()}
          onClear={props.clearPinnedSummaryScope}
        />
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
            getGroupLabel={props.getGroupLabel}
            groupedGuests={props.groupedGuests}
            groupedWindowing={props.groupedWindowing}
            guestMetadata={props.guestMetadata}
            guestParentNodeMap={props.guestParentNodeMap}
            groupingMode={props.groupingMode}
            handleCustomUrlUpdate={props.handleCustomUrlUpdate}
            handleTagClick={props.handleTagClick}
            activeSummaryWorkloadGroupScope={props.activeSummaryWorkloadGroupScope}
            activeSummaryWorkloadId={props.activeSummaryWorkloadId}
            focusedSummaryWorkloadGroupScope={props.focusedSummaryWorkloadGroupScope}
            focusedSummaryWorkloadGroupId={props.focusedSummaryWorkloadGroupId}
            hoveredSummaryWorkloadGroupScope={props.hoveredSummaryWorkloadGroupScope}
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
            workloadTableLayoutMode={props.workloadTableLayoutMode}
            workloadTableVisibleColumnIds={props.workloadTableVisibleColumnIds}
          />
        </Table>
      </TableCard>
    </ComponentErrorBoundary>
  );
}
