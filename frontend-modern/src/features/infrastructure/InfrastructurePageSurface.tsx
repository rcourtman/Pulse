import { For, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { buildInfrastructureWorkspacePath } from '@/components/Settings/infrastructureWorkspaceModel';
import { EmptyState } from '@/components/shared/EmptyState';
import { Card } from '@/components/shared/Card';
import { FilterSegmentedControl, LabeledFilterSelect } from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
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
    filtersOpen,
    setFiltersOpen,
    activeFilterCount,
    kioskMode,
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
    <div data-testid="infrastructure-page" class="space-y-4">
      <Show
        when={!loading() || initialLoadComplete()}
        fallback={
          <div class="space-y-3 animate-pulse pointer-events-none select-none">
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
                      onClick={() => navigate(buildInfrastructureWorkspacePath('install'))}
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

              <div
                ref={setSummaryClearSurfaceRootRef}
                class="space-y-3"
                data-testid="infrastructure-interaction-surface"
              >
              <Show when={!kioskMode()}>
                <div data-summary-clear-ignore>
                <Card padding="sm" class="mb-4">
                  <PageControls
                    search={
                      <SearchInput
                        value={searchQuery}
                        onChange={setSearchQuery}
                        placeholder="Search resources, IDs, IPs, or tags..."
                        class="w-full"
                        typeToSearch
                        history={{
                          storageKey: STORAGE_KEYS.RESOURCES_SEARCH_HISTORY,
                          emptyMessage: 'Recent infrastructure searches appear here.',
                        }}
                      />
                    }
                    mobileFilters={{
                      enabled: isMobile(),
                      onToggle: () => setFiltersOpen((o) => !o),
                      count: activeFilterCount(),
                    }}
                    resetAction={{
                      show: hasActiveFilters(),
                      onClick: clearFilters,
                      label: 'Clear',
                      class: 'ml-auto text-base-content',
                    }}
                    showFilters={!isMobile() || filtersOpen()}
                    toolbarClass="lg:flex-nowrap"
                  >
                    <LabeledFilterSelect
                      id="infra-source-filter"
                      label="Source"
                      value={selectedSource()}
                      onChange={(e) => setSelectedSource(e.currentTarget.value)}
                      selectClass="min-w-[8rem]"
                    >
                      <option value="">All</option>
                      <For each={sourceOptions()}>
                        {(source) => <option value={source.key}>{source.label}</option>}
                      </For>
                    </LabeledFilterSelect>

                    <LabeledFilterSelect
                      id="infra-status-filter"
                      label="Status"
                      value={selectedStatus()}
                      onChange={(e) => setSelectedStatus(e.currentTarget.value)}
                      selectClass="min-w-[7rem]"
                    >
                      <option value="">All</option>
                      <For each={statusOptions()}>
                        {(status) => <option value={status.key}>{status.label}</option>}
                      </For>
                    </LabeledFilterSelect>

                    <FilterSegmentedControl
                      value={groupingMode()}
                      onChange={(value) => setGroupingMode(value as GroupingMode)}
                      aria-label="Group By"
                      options={[
                        {
                          value: 'grouped',
                          title: 'Group by cluster',
                          label: (
                            <>
                              <svg
                                class="w-3 h-3"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                              >
                                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11z" />
                              </svg>
                              Grouped
                            </>
                          ),
                        },
                        {
                          value: 'flat',
                          title: 'Flat list view',
                          label: (
                            <>
                              <svg
                                class="w-3 h-3"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                              >
                                <line x1="8" y1="6" x2="21" y2="6" />
                                <line x1="8" y1="12" x2="21" y2="12" />
                                <line x1="8" y1="18" x2="21" y2="18" />
                                <line x1="3" y1="6" x2="3.01" y2="6" />
                                <line x1="3" y1="12" x2="3.01" y2="12" />
                                <line x1="3" y1="18" x2="3.01" y2="18" />
                              </svg>
                              List
                            </>
                          ),
                        },
                      ]}
                    />

                    <FilterSegmentedControl
                      class="hidden lg:inline-flex"
                      value={summaryCollapsed() ? 'hidden' : 'shown'}
                      onChange={() => setSummaryCollapsed((c) => !c)}
                      aria-label="Charts"
                      options={[
                        {
                          value: 'shown',
                          title: summaryCollapsed() ? 'Show charts' : 'Hide charts',
                          label: (
                            <>
                              <svg
                                class="w-3 h-3"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                              >
                                <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
                              </svg>
                              Charts
                            </>
                          ),
                        },
                      ]}
                    />
                  </PageControls>
                </Card>
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
