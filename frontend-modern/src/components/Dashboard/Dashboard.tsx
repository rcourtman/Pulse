import { Show } from 'solid-js';
import { InfrastructureSelector } from '@/components/shared/InfrastructureSelector';
import { DashboardFilter } from './DashboardFilter';
import { WorkloadsSummary } from '@/components/Workloads/WorkloadsSummary';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';
import { StickySummarySection } from '@/components/shared/StickySummarySection';
import { DashboardStateCards } from './DashboardStateCards';
import { DashboardStatsStrip } from './DashboardStatsStrip';
import { DashboardWorkloadTable } from './DashboardWorkloadTable';
import { useDashboardState, type DashboardProps } from './useDashboardState';

export function Dashboard(props: DashboardProps) {
  const state = useDashboardState(props);

  return (
    <div
      ref={state.setClearSurfaceRootRef}
      class="space-y-3"
      data-testid="workloads-page"
    >
      <Show when={state.isWorkloadsRoute() && !state.workloadsSummaryCollapsed()}>
        <StickySummarySection>
          <WorkloadsSummary
            timeRange={state.workloadsSummaryRange()}
            onTimeRangeChange={state.setWorkloadsSummaryRange}
            selectedNodeId={state.selectedNode()}
            fallbackGuestCounts={state.workloadsSummaryFallbackCounts()}
            fallbackSnapshots={state.workloadsSummaryFallbackSnapshots()}
            visibleWorkloadIds={state.workloadsSummaryVisibleIds()}
            chartHoverSync={state.chartHoverSync()}
            hoveredGroupScope={state.hoveredSummaryWorkloadGroupScope()}
            focusedGroupScope={state.focusedSummaryWorkloadGroupScope()}
            hoveredWorkloadId={state.hoveredWorkloadId()}
            focusedWorkloadId={state.selectedGuestId()}
            onChartHoverSyncChange={state.setChartHoverSync}
            showJumpToActiveRow={state.shouldShowJumpToActiveWorkloadRow()}
            onJumpToActiveRow={state.jumpToActiveWorkloadRow}
          />
        </StickySummarySection>
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

      <div class="space-y-3" data-testid="workloads-interaction-surface">
        <Show
          when={
            !state.kioskMode() &&
            state.surfaceConnected() &&
            state.surfaceInitialDataReceived() &&
            state.allGuests().length > 0
          }
        >
          <div data-summary-clear-ignore>
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
              platformFilter={state.platformFilterConfig()}
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
          <DashboardWorkloadTable
            activeAlerts={state.activeAlerts}
            alertsEnabled={state.alertsEnabled}
            bottomSpacerHeight={state.bottomSpacerHeight}
            clearPinnedSummaryScope={state.clearPinnedSummaryScope}
            getGroupLabel={state.getGroupLabel}
            groupedGuests={state.groupedGuests}
            groupedWindowing={state.groupedWindowing}
            guestMetadata={state.guestMetadata}
            guestParentNodeMap={state.guestParentNodeMap}
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
            mobileVisibleColumnIds={state.mobileVisibleColumnIds}
            mobileVisibleColumns={state.mobileVisibleColumns}
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
          />
        </Show>
      </div>

      <DashboardStatsStrip
        connected={state.surfaceConnected}
        initialDataReceived={state.surfaceInitialDataReceived}
        totalStats={state.totalStats}
      />

      <ScrollToTopButton />
    </div>
  );
}
