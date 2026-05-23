import { Show, createEffect, createMemo, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import ServerIcon from 'lucide-solid/icons/server';
import SettingsIcon from 'lucide-solid/icons/settings';
import type { TimeRange } from '@/api/charts';
import { buildInfrastructureOnboardingPath } from '@/components/Settings/infrastructureWorkspaceModel';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import { InfrastructureSummary } from '@/components/Infrastructure/InfrastructureSummary';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';
import { ChartVisibilityToggleButton } from '@/components/shared/FilterToolbar';
import { GroupedTableModeSegmentedControl } from '@/components/shared/GroupedTableModeSegmentedControl';
import { StickySummarySection } from '@/components/shared/StickySummarySection';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { SummaryChartHoverSync } from '@/components/shared/contextualFocus';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import { buildAgentsPageFilterModel, buildAgentsPageModel } from './agentsPageModel';

type AgentsGroupingMode = 'grouped' | 'flat';

const AGENTS_RESOURCE_QUERY = 'type=agent';
const AGENTS_TAB_SPECS = [{ id: 'overview', label: 'Overview', path: '/agents/overview' }] as const;

const agentsIcon = () => <ServerIcon class="h-6 w-6 text-slate-400" />;

export function AgentsPageSurface() {
  const navigate = useNavigate();
  const { isMobile } = useBreakpoint();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: AGENTS_RESOURCE_QUERY,
    cacheKey: 'agents-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });

  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);
  const [selectedStatus, setSelectedStatus] = createSignal('');
  const [searchQuery, setSearchQuery] = createSignal('');
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);
  const [hoveredResourceId, setHoveredResourceId] = createSignal<string | null>(null);
  const [chartHoverSync, setChartHoverSync] = createSignal<SummaryChartHoverSync | null>(null);
  const [summaryRange, setSummaryRange] = usePersistentSignal<TimeRange>(
    STORAGE_KEYS.AGENTS_SUMMARY_RANGE,
    '1h',
    {
      deserialize: (raw) => (isSummaryTimeRange(raw) ? raw : '1h'),
    },
  );
  const [summaryCollapsed, setSummaryCollapsed] = usePersistentSignal<boolean>(
    STORAGE_KEYS.AGENTS_SUMMARY_COLLAPSED,
    false,
    { deserialize: (raw) => raw === 'true' },
  );
  const [groupingMode, setGroupingMode] = usePersistentSignal<AgentsGroupingMode>(
    'agentsGroupingMode',
    'grouped',
    { deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped') },
  );

  const model = createMemo(() => buildAgentsPageModel(resources()));
  const filterModel = createMemo(() =>
    buildAgentsPageFilterModel(model().resources, selectedStatus(), searchQuery()),
  );
  const showLoading = createMemo(
    () => loading() && !initialLoadComplete() && model().resources.length === 0,
  );
  const showEmpty = createMemo(
    () => initialLoadComplete() && !error() && model().resources.length === 0,
  );

  const clearFilters = () => {
    setSelectedStatus('');
    setSearchQuery('');
  };

  createEffect(() => {
    if (!loading() && !initialLoadComplete()) {
      setInitialLoadComplete(true);
    }
  });

  return (
    <div data-testid="agents-page" class="space-y-3">
      <PlatformSectionTabs tabs={AGENTS_TAB_SPECS} active="overview" ariaLabel="Agents sections" />

      <Show
        when={!showLoading()}
        fallback={
          <PlatformTableLoadingState
            title="Loading Pulse Agent resources"
            description="Pulse is loading the agent-backed machine snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load Pulse Agent resources"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show
            when={!showEmpty()}
            fallback={
              <PlatformTableEmptyState
                icon={agentsIcon()}
                title="No Pulse Agent machines"
                description="Install the Pulse Agent on Linux, macOS, Windows, Unraid, or another host to populate this platform page."
                actions={
                  <button
                    type="button"
                    onClick={() => navigate(buildInfrastructureOnboardingPath('pick'))}
                    class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
                  >
                    <SettingsIcon class="h-3.5 w-3.5" />
                    Add agent
                  </button>
                }
              />
            }
          >
            <div class="space-y-4">
              <Show when={!summaryCollapsed()}>
                <StickySummarySection>
                  <InfrastructureSummary
                    resources={filterModel().filteredResources}
                    timeRange={summaryRange()}
                    onTimeRangeChange={setSummaryRange}
                    hoveredResourceId={hoveredResourceId()}
                    focusedResourceId={expandedResourceId()}
                    chartHoverSync={chartHoverSync()}
                    onChartHoverSyncChange={setChartHoverSync}
                  />
                </StickySummarySection>
              </Show>

              <FilterBar
                isMobile={isMobile}
                savedViewsKey="agents"
                search={{
                  value: searchQuery,
                  setValue: setSearchQuery,
                  placeholder: 'Search agents, hostnames, IPs, or tags...',
                  historyKey: STORAGE_KEYS.AGENTS_SEARCH_HISTORY,
                  emptyMessage: 'Recent agent searches appear here.',
                }}
                filters={
                  [
                    {
                      id: 'status',
                      label: 'Status',
                      group: 'status',
                      value: selectedStatus,
                      setValue: setSelectedStatus,
                      defaultValue: '',
                      options: () => [
                        { value: '', label: 'All' },
                        ...filterModel().statusOptions.map((status) => ({
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
                      onChange={(value) => setGroupingMode(value as AgentsGroupingMode)}
                    />
                    <ChartVisibilityToggleButton
                      collapsed={summaryCollapsed()}
                      onToggle={() => setSummaryCollapsed((collapsed) => !collapsed)}
                    />
                  </>
                }
              />

              <Show
                when={filterModel().hasFilteredResources}
                fallback={
                  <Card class="p-6">
                    <EmptyState
                      icon={agentsIcon()}
                      title="No agents match these filters"
                      description="Adjust the search or status filter to see more Pulse Agent machines."
                      actions={
                        <Show when={filterModel().hasActiveFilters}>
                          <button
                            type="button"
                            onClick={clearFilters}
                            class="inline-flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-sm hover:bg-slate-50"
                          >
                            Clear filters
                          </button>
                        </Show>
                      }
                    />
                  </Card>
                }
              >
                <UnifiedResourceTable
                  resources={filterModel().filteredResources}
                  expandedResourceId={expandedResourceId()}
                  onExpandedResourceChange={setExpandedResourceId}
                  hoveredResourceId={hoveredResourceId()}
                  onHoverChange={setHoveredResourceId}
                  groupingMode={groupingMode()}
                />
              </Show>
            </div>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default AgentsPageSurface;
