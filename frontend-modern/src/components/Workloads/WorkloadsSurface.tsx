import { Show } from 'solid-js';
import { InfrastructureSelector } from '@/components/shared/InfrastructureSelector';
import { EmptyState } from '@/components/shared/EmptyState';
import { PageHeader } from '@/components/shared/PageHeader';
import { TableCard } from '@/components/shared/TableCard';
import { WorkloadsFilter } from './WorkloadsFilter';
import { WorkloadsSummary } from '@/components/Workloads/WorkloadsSummary';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';
import { StickySummarySection } from '@/components/shared/StickySummarySection';
import { WorkloadsStateCards } from './WorkloadsStateCards';
import { WorkloadsStatsStrip } from './WorkloadsStatsStrip';
import { WorkloadsTable } from './WorkloadsTable';
import { useWorkloadsState, type WorkloadsSurfaceProps } from './useWorkloadsState';

export function WorkloadsSurface(props: WorkloadsSurfaceProps) {
  const state = useWorkloadsState(props);

  return (
    <div ref={state.setClearSurfaceRootRef} class="space-y-3" data-testid="workloads-page">
      <Show when={!props.embedded}>
        <PageHeader
          title="Workloads"
          description="Inspect live workloads, filter by platform and status, and drill into compute, memory, and I/O posture."
        />
      </Show>

      <Show
        when={!props.tableOnly && state.isWorkloadsRoute() && !state.workloadsSummaryCollapsed()}
      >
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

      <Show when={!props.embedded}>
        <InfrastructureSelector
          currentTab="workloads"
          globalTemperatureMonitoringEnabled={state.ws.state.temperatureMonitoringEnabled}
          onNodeSelect={state.handleNodeSelect}
          nodes={state.infrastructureNodes()}
          searchTerm={state.search()}
          showNodeSummary={!state.isWorkloadsRoute()}
        />
      </Show>

      <Show when={!props.tableOnly}>
        <WorkloadsStateCards
          allGuests={state.allGuests}
          connected={state.surfaceConnected}
          workloadsDisconnectedState={state.workloadsDisconnectedState}
          workloadsGuestsEmptyState={state.workloadsGuestsEmptyState}
          workloadsInfrastructureEmptyState={state.workloadsInfrastructureEmptyState}
          workloadsLoadingState={state.workloadsLoadingState}
          workloadsNoInventoryState={state.workloadsNoInventoryState}
          filteredGuests={state.filteredGuests}
          hasInfrastructureSources={state.hasInfrastructureSources}
          infrastructureSourceStateReady={state.infrastructureSourceStateReady}
          initialDataReceived={state.surfaceInitialDataReceived}
          kioskMode={state.kioskMode}
          navigate={state.navigate}
          reconnect={state.reconnectSurface}
          workloadInventoryIssues={state.workloadInventoryIssues}
          workloads={state.workloads}
        />
      </Show>

      <div class="space-y-3" data-testid="workloads-interaction-surface">
        <Show
          when={
            (props.showFilterToolbar || !props.tableOnly) &&
            !state.kioskMode() &&
            state.surfaceConnected() &&
            state.surfaceInitialDataReceived() &&
            state.allGuests().length > 0
          }
        >
          <div data-summary-clear-ignore>
            <WorkloadsFilter
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
              columnVisibility={state.workloadsFilterColumnVisibility()}
              chartsCollapsed={
                state.isWorkloadsRoute() ? state.workloadsSummaryCollapsed : undefined
              }
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
          <WorkloadsTable
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
            workloadMetricHistoryRange={state.workloadMetricHistoryRange}
            workloadMetricHistory={state.workloadMetricHistory}
            workloadTableLayoutMode={state.workloadTableLayoutMode}
            workloadTableVisibleColumnIds={state.workloadTableVisibleColumnIds}
            workloadTableVisibleColumns={state.workloadTableVisibleColumns}
            setWorkloadMetricDisplayMode={state.setWorkloadMetricDisplayMode}
            setWorkloadMetricHistoryRange={state.setWorkloadMetricHistoryRange}
          />
        </Show>
        <Show
          when={
            props.tableOnly &&
            state.surfaceConnected() &&
            state.surfaceInitialDataReceived() &&
            state.filteredGuests().length === 0
          }
        >
          <TableCard>
            <div class="p-6">
              <EmptyState
                title="No Proxmox workloads"
                description="Proxmox VMs and containers appear here when inventory is available."
              />
            </div>
          </TableCard>
        </Show>
      </div>

      <Show when={!props.embedded}>
        <WorkloadsStatsStrip
          connected={state.surfaceConnected}
          initialDataReceived={state.surfaceInitialDataReceived}
          totalStats={state.totalStats}
        />
      </Show>

      <Show when={!props.embedded}>
        <ScrollToTopButton />
      </Show>
    </div>
  );
}
