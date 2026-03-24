import { Show } from 'solid-js';
import { InfrastructureSelector } from '@/components/shared/InfrastructureSelector';
import { DashboardFilter } from './DashboardFilter';
import { WorkloadsSummary } from '@/components/Workloads/WorkloadsSummary';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';
import { DashboardStateCards } from './DashboardStateCards';
import { DashboardStatsStrip } from './DashboardStatsStrip';
import { DashboardWorkloadTable } from './DashboardWorkloadTable';
import { useDashboardState, type DashboardProps } from './useDashboardState';

export function Dashboard(props: DashboardProps) {
  const state = useDashboardState(props);

  return (
    <div class="space-y-3">
      <Show when={state.isWorkloadsRoute() && !state.workloadsSummaryCollapsed()}>
        <div class="hidden lg:block sticky-shield sticky top-0 z-20 bg-surface">
          <WorkloadsSummary
            timeRange={state.workloadsSummaryRange()}
            onTimeRangeChange={state.setWorkloadsSummaryRange}
            selectedNodeId={state.selectedNode()}
            fallbackGuestCounts={state.workloadsSummaryFallbackCounts()}
            fallbackSnapshots={state.workloadsSummaryFallbackSnapshots()}
            visibleWorkloadIds={state.workloadsSummaryVisibleIds()}
            hoveredWorkloadId={state.hoveredWorkloadId()}
            focusedWorkloadId={state.selectedGuestId()}
          />
        </div>
      </Show>

      <InfrastructureSelector
        currentTab="dashboard"
        globalTemperatureMonitoringEnabled={state.ws.state.temperatureMonitoringEnabled}
        onNodeSelect={state.handleNodeSelect}
        nodes={props.nodes}
        searchTerm={state.search()}
        showNodeSummary={!state.isWorkloadsRoute()}
      />

      <DashboardStateCards
        allGuests={state.allGuests}
        connected={state.surfaceConnected}
        dashboardDisconnectedState={state.dashboardDisconnectedState}
        dashboardGuestsEmptyState={state.dashboardGuestsEmptyState}
        dashboardInfrastructureEmptyState={state.dashboardInfrastructureEmptyState}
        dashboardLoadingState={state.dashboardLoadingState}
        filteredGuests={state.filteredGuests}
        initialDataReceived={state.surfaceInitialDataReceived}
        kioskMode={state.kioskMode}
        navigate={state.navigate}
        nodeCount={props.nodes.length}
        reconnect={state.reconnectSurface}
        workloads={state.workloads}
      />

      <Show
        when={
          !state.kioskMode() &&
          state.surfaceConnected() &&
          state.surfaceInitialDataReceived() &&
          state.allGuests().length > 0
        }
      >
        <DashboardFilter
          search={state.search}
          setSearch={state.setSearch}
          viewMode={state.viewMode}
          setViewMode={state.setViewMode}
          statusMode={state.statusMode}
          setStatusMode={state.setStatusMode}
          groupingMode={state.groupingMode}
          setGroupingMode={state.setGroupingMode}
          setSortKey={state.setSortKey}
          setSortDirection={state.setSortDirection}
          onBeforeAutoFocus={state.handleBeforeAutoFocus}
          columnVisibility={state.dashboardFilterColumnVisibility()}
          chartsCollapsed={state.isWorkloadsRoute() ? state.workloadsSummaryCollapsed : undefined}
          onChartsToggle={
            state.isWorkloadsRoute()
              ? () => state.setWorkloadsSummaryCollapsed((collapsed) => !collapsed)
              : undefined
          }
          containerRuntimeFilter={state.containerRuntimeFilterConfig()}
          hostFilter={state.hostFilterConfig()}
          namespaceFilter={state.namespaceFilterConfig()}
        />
      </Show>

      <Show
        when={
          state.surfaceConnected() &&
          state.surfaceInitialDataReceived() &&
          state.filteredGuests().length > 0
        }
      >
        <DashboardWorkloadTable
          activeAlerts={state.activeAlerts}
          alertsEnabled={state.alertsEnabled}
          bottomSpacerHeight={state.bottomSpacerHeight}
          getGroupLabel={state.getGroupLabel}
          groupedGuests={state.groupedGuests}
          groupedWindowing={state.groupedWindowing}
          guestMetadata={state.guestMetadata}
          guestParentNodeMap={state.guestParentNodeMap}
          groupingMode={state.groupingMode}
          handleCustomUrlUpdate={state.handleCustomUrlUpdate}
          handleSort={state.handleSort}
          handleTagClick={state.handleTagClick}
          isMobile={state.isMobile}
          mobileVisibleColumnIds={state.mobileVisibleColumnIds}
          mobileVisibleColumns={state.mobileVisibleColumns}
          nodeByInstance={state.nodeByInstance}
          search={state.search}
          selectedGuestId={state.selectedGuestId}
          setHoveredWorkloadId={state.setHoveredWorkloadId}
          setSelectedGuestId={state.setSelectedGuestId}
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
        />
      </Show>

      <DashboardStatsStrip
        connected={state.surfaceConnected}
        initialDataReceived={state.surfaceInitialDataReceived}
        totalStats={state.totalStats}
      />

      <ScrollToTopButton />
    </div>
  );
}
