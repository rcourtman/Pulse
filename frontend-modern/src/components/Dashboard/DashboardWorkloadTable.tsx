import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { Card } from '@/components/shared/Card';
import { SummaryTableCardHeader } from '@/components/shared/SummaryTableCardHeader';
import { Table } from '@/components/shared/Table';

import type { DashboardState } from './useDashboardState';
import { WorkloadPanel } from './WorkloadPanel';
import { WorkloadTableHeader } from './WorkloadTableHeader';

type DashboardWorkloadTableProps = Pick<
  DashboardState,
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
  | 'mobileVisibleColumnIds'
  | 'mobileVisibleColumns'
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
>;

export function DashboardWorkloadTable(props: DashboardWorkloadTableProps) {
  const showClearSelection = () =>
    Boolean(props.selectedGuestId() || props.focusedSummaryWorkloadGroupId());

  return (
    <ComponentErrorBoundary name="Guest Table">
      <Card
        ref={props.setTableRootRef}
        padding="none"
        tone="card"
        class="mb-4 rounded-md"
        data-summary-clear-surface
        data-testid="workloads-table-surface"
      >
        <SummaryTableCardHeader
          title="Workloads"
          showClearAction={showClearSelection()}
          onClear={props.clearPinnedSummaryScope}
        />
        <Table
          wrapperRef={props.setTableWrapperRef}
          class={`workload-table ${props.isMobile() ? 'workload-table--mobile' : 'workload-table--desktop'}`}
        >
          <WorkloadTableHeader
            handleSort={props.handleSort}
            isMobile={props.isMobile}
            mobileVisibleColumns={props.mobileVisibleColumns}
            sortDirection={props.sortDirection}
            sortKey={props.sortKey}
            visibleColumns={props.visibleColumns}
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
            mobileVisibleColumnIds={props.mobileVisibleColumnIds}
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
          />
        </Table>
      </Card>
    </ComponentErrorBoundary>
  );
}
