import { Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { buildInfrastructureOnboardingPath } from '@/components/Settings/infrastructureWorkspaceModel';
import { EmptyState } from '@/components/shared/EmptyState';
import { Card } from '@/components/shared/Card';
import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';
import { ChartVisibilityToggleButton } from '@/components/shared/FilterToolbar';
import { GroupedTableModeSegmentedControl } from '@/components/shared/GroupedTableModeSegmentedControl';
import { PageHeader } from '@/components/shared/PageHeader';
import { StickySummarySection } from '@/components/shared/StickySummarySection';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import { InfrastructureSummary } from '@/components/Infrastructure/InfrastructureSummary';
import ServerIcon from 'lucide-solid/icons/server';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import SettingsIcon from 'lucide-solid/icons/settings';
import { ScrollToTopButton } from '@/components/shared/ScrollToTopButton';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { AgentDeployModal } from '@/components/Infrastructure/AgentDeployModal';
import {
  getInfrastructureEmptyState,
  getInfrastructureFilterEmptyState,
  getInfrastructureLoadFailureState,
} from '@/utils/infrastructureEmptyStatePresentation';
import { useInfrastructurePageState, type GroupingMode } from './useInfrastructurePageState';

export function InfrastructurePageSurface() {
  const navigate = useNavigate();
  const {
    loading,
    error,
    refetch,
    initialLoadComplete,
    showNoResources,
    selectedSource,
    setSelectedSource,
    selectedStatus,
    setSelectedStatus,
    searchQuery,
    setSearchQuery,
    infrastructureSummaryRange,
    setInfrastructureSummaryRange,
    summaryCollapsed,
    setSummaryCollapsed,
    groupingMode,
    setGroupingMode,
    activeSummaryResourceId,
    activeSummaryResourceGroupScope,
    focusedSummaryResourceGroupId,
    focusedSummaryResourceGroupScope,
    expandedResourceId,
    setExpandedResourceId,
    chartHoverSync,
    hoveredResourceId,
    hoveredSummaryResourceGroupScope,
    setFocusedResourceGroupId,
    setHoveredResourceGroupScope,
    setHoveredResourceId,
    highlightedResourceId,
    revealedResourceId,
    isMobile,
    jumpToActiveResourceRow,
    deployCluster,
    setDeployCluster,
    kioskMode,
    clearPinnedSummaryScope,
    sourceOptions,
    statusOptions,
    hasActiveFilters,
    clearFilters,
    filteredResources,
    hasFilteredResources,
    setChartHoverSync,
    setSummaryClearSurfaceRootRef,
    setSummaryTableRootRef,
    shouldShowJumpToActiveResourceRow,
  } = useInfrastructurePageState();
  const infrastructureEmptyState = () => getInfrastructureEmptyState();
  const infrastructureFilterEmptyState = () => getInfrastructureFilterEmptyState();
  const infrastructureLoadFailureState = () => getInfrastructureLoadFailureState();

  return (
    <div ref={setSummaryClearSurfaceRootRef} data-testid="infrastructure-page" class="space-y-4">
      <PageHeader
        title="Infrastructure"
        description="Inspect connected resources, filter by platform and status, and drill into live health and capacity."
      />

      <Show
        when={!loading() || initialLoadComplete()}
        fallback={
          <div
            data-testid="infrastructure-loading"
            class="space-y-3 animate-pulse pointer-events-none select-none"
          >
            <div class="hidden lg:block h-[124px] w-full bg-surface-alt rounded-md border border-border"></div>
            <Card padding="sm" class="h-[52px] bg-surface-alt"></Card>
            <Card padding="none" tone="card" class="h-[600px] overflow-hidden">
              <div class="h-8 border-b"></div>
              <div class="space-y-4 p-4">
                <div class="h-4 w-1/4 rounded bg-surface-hover"></div>
                <div class="h-4 w-1/2 rounded bg-surface-hover"></div>
                <div class="h-4 w-1/3 rounded bg-surface-hover"></div>
              </div>
            </Card>
          </div>
        }
      >
        <Show
          when={!error()}
          fallback={
            <Card class="p-6">
              <EmptyState
                icon={<ServerIcon class="w-6 h-6 text-slate-400" />}
                title={infrastructureLoadFailureState().title}
                description={infrastructureLoadFailureState().description}
                actions={
                  <button
                    type="button"
                    onClick={() => refetch()}
                    class="inline-flex items-center gap-2 rounded-md border px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:"
                  >
                    <RefreshCwIcon class="h-3.5 w-3.5" />
                    {infrastructureLoadFailureState().actionLabel}
                  </button>
                }
              />
            </Card>
          }
        >
          <Show
            when={!showNoResources()}
            fallback={
              <Card class="p-6">
                <EmptyState
                  icon={<ServerIcon class="w-6 h-6 text-slate-400" />}
                  title={infrastructureEmptyState().title}
                  description={infrastructureEmptyState().description}
                  actions={
                    <button
                      type="button"
                      onClick={() => navigate(buildInfrastructureOnboardingPath('pick'))}
                      class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
                    >
                      <SettingsIcon class="h-3.5 w-3.5" />
                      {infrastructureEmptyState().actionLabel}
                    </button>
                  }
                />
              </Card>
            }
          >
            <div class="space-y-3">
              <Show when={!summaryCollapsed()}>
                <StickySummarySection>
                  <InfrastructureSummary
                    resources={filteredResources()}
                    timeRange={infrastructureSummaryRange()}
                    onTimeRangeChange={setInfrastructureSummaryRange}
                    hoveredGroupScope={hoveredSummaryResourceGroupScope()}
                    focusedGroupScope={focusedSummaryResourceGroupScope()}
                    hoveredResourceId={hoveredResourceId()}
                    focusedResourceId={expandedResourceId()}
                    chartHoverSync={chartHoverSync()}
                    onChartHoverSyncChange={setChartHoverSync}
                    showJumpToActiveRow={shouldShowJumpToActiveResourceRow()}
                    onJumpToActiveRow={jumpToActiveResourceRow}
                  />
                </StickySummarySection>
              </Show>

              <div class="space-y-3" data-testid="infrastructure-interaction-surface">
                <Show when={!kioskMode()}>
                  <div data-summary-clear-ignore>
                    <FilterBar
                      isMobile={isMobile}
                      savedViewsKey="infrastructure"
                      search={{
                        value: searchQuery,
                        setValue: setSearchQuery,
                        placeholder: 'Search resources, IDs, IPs, or tags...',
                        historyKey: STORAGE_KEYS.RESOURCES_SEARCH_HISTORY,
                        emptyMessage: 'Recent infrastructure searches appear here.',
                      }}
                      filters={
                        [
                          {
                            id: 'platform',
                            label: 'Platform',
                            group: 'scope',
                            value: selectedSource,
                            setValue: setSelectedSource,
                            defaultValue: '',
                            options: () => [
                              { value: '', label: 'All' },
                              ...sourceOptions().map((source) => ({
                                value: source.key,
                                label: source.label,
                              })),
                            ],
                          },
                          {
                            id: 'status',
                            label: 'Status',
                            group: 'status',
                            value: selectedStatus,
                            setValue: setSelectedStatus,
                            defaultValue: '',
                            options: () => [
                              { value: '', label: 'All' },
                              ...statusOptions().map((status) => ({
                                value: status.key,
                                label: status.label,
                              })),
                            ],
                          },
                        ] as FilterDef[]
                      }
                      viewOptionsTrailing={
                        <>
                          <GroupedTableModeSegmentedControl
                            value={groupingMode()}
                            onChange={(value) => setGroupingMode(value as GroupingMode)}
                          />
                          <ChartVisibilityToggleButton
                            collapsed={summaryCollapsed()}
                            onToggle={() => setSummaryCollapsed((collapsed) => !collapsed)}
                          />
                        </>
                      }
                    />
                  </div>
                </Show>

                <Show
                  when={hasFilteredResources()}
                  fallback={
                    <Card class="p-6">
                      <EmptyState
                        icon={<ServerIcon class="w-6 h-6 text-slate-400" />}
                        title={infrastructureFilterEmptyState().title}
                        description={infrastructureFilterEmptyState().description}
                        actions={
                          <Show when={hasActiveFilters()}>
                            <button
                              type="button"
                              onClick={clearFilters}
                              class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
                            >
                              {infrastructureFilterEmptyState().actionLabel}
                            </button>
                          </Show>
                        }
                      />
                    </Card>
                  }
                >
                  <div class="space-y-3">
                    <UnifiedResourceTable
                      resources={filteredResources()}
                      expandedResourceId={expandedResourceId()}
                      clearPinnedSummaryScope={clearPinnedSummaryScope}
                      activeSummaryGroupScope={activeSummaryResourceGroupScope()}
                      hoveredSummaryGroupScope={hoveredSummaryResourceGroupScope()}
                      focusedSummaryGroupScope={focusedSummaryResourceGroupScope()}
                      focusedSummaryGroupId={focusedSummaryResourceGroupId()}
                      hoveredResourceId={activeSummaryResourceId()}
                      highlightedResourceId={highlightedResourceId()}
                      revealedResourceId={revealedResourceId()}
                      onExpandedResourceChange={setExpandedResourceId}
                      onGroupFocusChange={setFocusedResourceGroupId}
                      onGroupHoverChange={setHoveredResourceGroupScope}
                      onHoverChange={setHoveredResourceId}
                      groupingMode={groupingMode()}
                      onDeployCluster={(id, name) => setDeployCluster({ id, name })}
                      setTableRootRef={setSummaryTableRootRef}
                    />
                  </div>
                </Show>
              </div>
            </div>
          </Show>
        </Show>
      </Show>
      <Show when={deployCluster()}>
        {(cluster) => (
          <AgentDeployModal
            isOpen={true}
            clusterId={cluster().id}
            clusterName={cluster().name}
            onClose={() => setDeployCluster(null)}
          />
        )}
      </Show>
      <ScrollToTopButton />
    </div>
  );
}

export default InfrastructurePageSurface;
