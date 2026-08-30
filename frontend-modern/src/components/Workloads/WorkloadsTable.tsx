import { For, type JSX } from 'solid-js';

import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { Table } from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';

import { getGuestColumnWidthStyle } from './guestRowModel';
import type { WorkloadsState } from './useWorkloadsState';
import { WorkloadPanel } from './WorkloadPanel';
import { WorkloadTableHeader } from './WorkloadTableHeader';

export const WORKLOAD_TABLE_MOBILE_MIN_WIDTH_CLASS = 'min-w-[0px]';

type WorkloadsTableProps = Pick<
  WorkloadsState,
  | 'activeAlerts'
  | 'alertsEnabled'
  | 'bottomSpacerHeight'
  | 'compactGroupHeaders'
  | 'getGroupLabel'
  | 'getNodeTemperatureThresholds'
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
  | 'workloadMetricHoverMode'
  | 'workloadMemoryDisplayBasis'
  | 'workloadMetricHistory'
  | 'workloadTableLayoutMode'
  | 'workloadTableVisibleColumnIds'
  | 'workloadTableVisibleColumns'
  | 'workloadColumnWidths'
  | 'workloadManualColumnSizing'
  | 'workloadManualColumnSizingSupported'
  | 'workloadTableManualWidth'
  | 'beginWorkloadColumnResize'
  | 'previewWorkloadColumnWidth'
  | 'commitWorkloadColumnResize'
  | 'cancelWorkloadColumnResize'
  | 'clearWorkloadColumnWidth'
> & {
  title?: JSX.Element;
};

export function WorkloadsTable(props: WorkloadsTableProps) {
  return (
    <ComponentErrorBoundary name="Guest Table">
      <TableCard
        ref={props.setTableRootRef}
        class="mb-4 rounded-md"
        data-summary-clear-surface
        data-testid="workloads-table-surface"
      >
        <TableCardHeader title={props.title} />
        <Table
          wrapperRef={props.setTableWrapperRef}
          class={`platform-table workload-table table-fixed ${props.isMobile() ? `workload-table--mobile ${WORKLOAD_TABLE_MOBILE_MIN_WIDTH_CLASS}` : 'workload-table--desktop min-w-full'}${props.workloadManualColumnSizing() ? ' workload-table--manual-widths' : ''}`}
          style={
            props.workloadTableManualWidth() === null
              ? undefined
              : { width: `${props.workloadTableManualWidth()}px` }
          }
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
                    props.workloadColumnWidths(),
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
            workloadMemoryDisplayBasis={props.workloadMemoryDisplayBasis}
            workloadTableLayoutMode={props.workloadTableLayoutMode}
            workloadTableVisibleColumnIds={props.workloadTableVisibleColumnIds}
            workloadTableVisibleColumns={props.workloadTableVisibleColumns}
            workloadColumnWidths={props.workloadColumnWidths}
            workloadManualColumnSizingSupported={props.workloadManualColumnSizingSupported}
            beginWorkloadColumnResize={props.beginWorkloadColumnResize}
            previewWorkloadColumnWidth={props.previewWorkloadColumnWidth}
            commitWorkloadColumnResize={props.commitWorkloadColumnResize}
            cancelWorkloadColumnResize={props.cancelWorkloadColumnResize}
            clearWorkloadColumnWidth={props.clearWorkloadColumnWidth}
          />
          <WorkloadPanel
            activeAlerts={props.activeAlerts}
            alertsEnabled={props.alertsEnabled}
            bottomSpacerHeight={props.bottomSpacerHeight}
            compactGroupHeaders={props.compactGroupHeaders}
            getGroupLabel={props.getGroupLabel}
            getNodeTemperatureThresholds={props.getNodeTemperatureThresholds}
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
            workloadMetricHoverMode={props.workloadMetricHoverMode}
            workloadMemoryDisplayBasis={props.workloadMemoryDisplayBasis}
            workloadMetricHistory={props.workloadMetricHistory}
            workloadTableLayoutMode={props.workloadTableLayoutMode}
            workloadTableVisibleColumnIds={props.workloadTableVisibleColumnIds}
            workloadTableVisibleColumns={props.workloadTableVisibleColumns}
            workloadColumnWidths={props.workloadColumnWidths}
          />
        </Table>
      </TableCard>
    </ComponentErrorBoundary>
  );
}
