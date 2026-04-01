import { ComponentErrorBoundary } from '@/components/ErrorBoundary';
import { Card } from '@/components/shared/Card';
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
  | 'activeSummaryWorkloadId'
  | 'isMobile'
  | 'mobileVisibleColumnIds'
  | 'mobileVisibleColumns'
  | 'nodeByInstance'
  | 'search'
  | 'selectedGuestId'
  | 'setHoveredWorkloadId'
  | 'setSelectedGuestId'
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
  return (
    <ComponentErrorBoundary name="Guest Table">
      <Card padding="none" tone="card" class="mb-4 rounded-md">
        <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
          Workloads
        </div>
        <div class="overflow-x-auto">
          <Table
            wrapperRef={props.setTableWrapperRef}
            class="whitespace-nowrap min-w-[max-content]"
            style={{
              'table-layout': 'fixed',
              'min-width': props.isMobile() ? '100%' : 'max-content',
            }}
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
              activeSummaryWorkloadId={props.activeSummaryWorkloadId}
              mobileVisibleColumnIds={props.mobileVisibleColumnIds}
              nodeByInstance={props.nodeByInstance}
              search={props.search}
              selectedGuestId={props.selectedGuestId}
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
        </div>
      </Card>
    </ComponentErrorBoundary>
  );
}
