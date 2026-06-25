import { For, Show, createMemo } from 'solid-js';

import { buildInfrastructureWorkspacePath } from '@/components/Settings/infrastructureWorkspaceModel';
import { EmptyState } from '@/components/shared/EmptyState';
import { ButtonLink } from '@/components/shared/Button';
import { TableCard } from '@/components/shared/TableCard';
import { WorkloadsFilter } from './WorkloadsFilter';
import { DEFAULT_WORKLOADS_VIEW_MODE, hasActiveWorkloadsFilters } from './workloadsFilterModel';
import { WorkloadsTable } from './WorkloadsTable';
import type { WorkloadInventorySourceIssue } from './workloadInventorySourceIssues';
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

type TableOnlyEmptyState = {
  title: string;
  description: string;
  actionLabel?: string;
};

const PLATFORM_ISSUE_TYPES: Record<string, readonly WorkloadInventorySourceIssue['type'][]> = {
  docker: ['docker'],
  kubernetes: ['kubernetes'],
  'proxmox-pve': ['pve'],
  'vmware-vsphere': ['vmware'],
};

function WorkloadInventoryIssueList(props: { issues: readonly WorkloadInventorySourceIssue[] }) {
  return (
    <div
      class="w-full max-w-2xl border-t border-border pt-3 text-left"
      data-testid="workload-inventory-source-issues"
    >
      <ul class="divide-y divide-border">
        <For each={props.issues}>
          {(issue) => (
            <li class="py-3">
              <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div class="min-w-0 space-y-1">
                  <p class="text-sm font-semibold text-base-content">
                    {issue.stateLabel}: {issue.name}
                  </p>
                  <p class="text-sm leading-6 text-muted">{issue.description}</p>
                  <Show when={issue.detail}>
                    <p class="text-xs leading-5 text-muted">{issue.detail}</p>
                  </Show>
                </div>
                <span class="shrink-0 rounded bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-950 dark:text-amber-200">
                  {issue.coverageLabel}
                </span>
              </div>
            </li>
          )}
        </For>
      </ul>
    </div>
  );
}

export function WorkloadsSurface(props: WorkloadsSurfaceComponentProps) {
  const state = props.state ?? useWorkloadsState(props);
  const visibleInventoryIssues = createMemo(() => {
    const issues = state.workloadInventoryIssues?.() ?? [];
    const forcedPlatform = props.forcedPlatform?.trim().toLowerCase();
    const platformIssueTypes = forcedPlatform ? PLATFORM_ISSUE_TYPES[forcedPlatform] : undefined;
    if (!platformIssueTypes) return issues;
    return issues.filter((issue) => platformIssueTypes.includes(issue.type));
  });
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
  const tableOnlyEmptyState = createMemo<TableOnlyEmptyState>(() => {
    if (tableOnlyFiltersActive()) {
      return state.workloadsGuestsEmptyState();
    }

    if (visibleInventoryIssues().length > 0) {
      return state.workloadsNoInventoryState?.() ?? {
        title: 'No workload inventory available',
        description:
          'Pulse has infrastructure sources, but no VM, container, or pod inventory is available right now.',
        actionLabel: 'Review infrastructure sources',
      };
    }

    return {
      title: props.emptyStateTitle ?? 'No workloads',
      description:
        props.emptyStateDescription ?? 'Workloads appear here when inventory is available.',
    };
  });

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
            nestedWorkloadContextByGuestId={state.nestedWorkloadContextByGuestId}
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
                tone={visibleInventoryIssues().length > 0 ? 'warning' : 'default'}
                align={visibleInventoryIssues().length > 0 ? 'left' : 'center'}
                actions={
                  <>
                    <Show when={visibleInventoryIssues().length > 0}>
                      <WorkloadInventoryIssueList issues={visibleInventoryIssues()} />
                    </Show>
                    <Show when={tableOnlyEmptyState().actionLabel}>
                      <ButtonLink
                        href={buildInfrastructureWorkspacePath()}
                        variant="secondary"
                        size="sm"
                      >
                        {tableOnlyEmptyState().actionLabel}
                      </ButtonLink>
                    </Show>
                  </>
                }
              />
            </div>
          </TableCard>
        </Show>
      </div>
    </div>
  );
}
