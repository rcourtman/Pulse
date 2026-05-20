import { useLocation } from '@solidjs/router';
import DatabaseIcon from 'lucide-solid/icons/database';
import { Show, createMemo, type Accessor } from 'solid-js';
import StorageSurface from '@/components/Storage/Storage';
import { WorkloadsFilter } from '@/components/Workloads/WorkloadsFilter';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { useWorkloadsState } from '@/components/Workloads/useWorkloadsState';
import type { WorkloadsStatusOption } from '@/components/Workloads/workloadsFilterModel';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { resourceMatchesSearch } from '@/utils/resourceSearchMatch';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { TrueNASSystemsTable } from './TrueNASSystemsTable';
import {
  TRUENAS_TAB_SPECS,
  buildTrueNASPageModel,
  type TrueNASPageModel,
  type TrueNASPageTabId,
} from './truenasPageModel';

// `pool` and `dataset` collapse into `storage` at the API boundary
// (with `storage.topology` differentiating them) — they are not
// first-class type tokens and including them triggers a 400 from
// `/api/resources`. The page model still buckets by topology
// client-side.
const TRUENAS_RESOURCE_QUERY = 'type=agent,app-container,storage,physical_disk';
const TRUENAS_PLATFORM_FILTER = 'truenas';
const VALID_TABS = new Set<TrueNASPageTabId>(TRUENAS_TAB_SPECS.map((tab) => tab.id));
const TRUENAS_APP_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'degraded', label: 'Attention' },
  { value: 'stopped', label: 'Stopped' },
];

const truenasIcon = () => <DatabaseIcon class="h-6 w-6 text-slate-400" />;

export function TrueNASPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: TRUENAS_RESOURCE_QUERY,
    cacheKey: 'truenas-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const activeTab = createMemo<TrueNASPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as TrueNASPageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const model = createMemo(() => buildTrueNASPageModel(resources()));

  return (
    <div data-testid="truenas-page" class="space-y-3">
      <PlatformSectionTabs
        tabs={TRUENAS_TAB_SPECS}
        active={activeTab()}
        ariaLabel="TrueNAS sections"
      />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableLoadingState
            title="Loading TrueNAS resources"
            description="Pulse is loading the TrueNAS resource snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load TrueNAS resources"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show
            when={model().resources.length > 0}
            fallback={
              <PlatformTableEmptyState
                icon={truenasIcon()}
                title="No TrueNAS systems"
                description="Add a TrueNAS connection in Settings or install the Pulse agent on a TrueNAS host."
              />
            }
          >
            <Show when={activeTab() === 'overview'}>
              <TrueNASOverview model={model} />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <StorageSurface
                embedded
                tableOnly
                showFilterToolbar
                forcedSourceFilter={TRUENAS_PLATFORM_FILTER}
                filterAriaLabel="TrueNAS storage filters"
                filterSearchPlaceholder="Search TrueNAS pools, datasets, disks, or nodes"
                filterSearchEmptyMessage="Recent TrueNAS storage searches appear here."
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

interface TrueNASOverviewProps {
  model: Accessor<TrueNASPageModel>;
}

function TrueNASOverview(props: TrueNASOverviewProps) {
  const workloadsState = useWorkloadsState({
    vms: [],
    containers: [],
    nodes: [],
    useWorkloads: true,
    embedded: true,
    tableOnly: true,
    forcedPlatform: TRUENAS_PLATFORM_FILTER,
    forcedViewMode: 'app-container',
    showFilterToolbar: true,
    suppressPlatformFilter: true,
    allowEmbeddedScopeFilters: true,
    compactGroupHeaders: true,
  });
  const showSharedFilterToolbar = createMemo(
    () =>
      props.model().apps.length > 0 &&
      workloadsState.surfaceConnected() &&
      workloadsState.surfaceInitialDataReceived() &&
      workloadsState.allGuests().length > 0,
  );
  const filteredSystems = createMemo(() => {
    const term = workloadsState.search().trim();
    if (!term) return props.model().systems;
    return props.model().systems.filter((system) => resourceMatchesSearch(system, term));
  });

  return (
    <div class="space-y-4">
      <Show when={showSharedFilterToolbar()}>
        <div data-summary-clear-ignore>
          <WorkloadsFilter
            search={workloadsState.search}
            setSearch={workloadsState.setSearch}
            viewMode={workloadsState.viewMode}
            setViewMode={workloadsState.setViewMode}
            statusMode={workloadsState.statusMode}
            setStatusMode={workloadsState.setStatusMode}
            groupingMode={workloadsState.groupingMode}
            setGroupingMode={workloadsState.setGroupingMode}
            setSortKey={workloadsState.setSortKey}
            setSortDirection={workloadsState.setSortDirection}
            onBeforeAutoFocus={workloadsState.handleBeforeAutoFocus}
            ariaLabel="TrueNAS app filters"
            searchPlaceholder="Search TrueNAS apps by name, image, namespace, or system"
            searchEmptyMessage="Recent TrueNAS app searches appear here."
            statusOptions={TRUENAS_APP_STATUS_OPTIONS}
            columnVisibility={workloadsState.workloadsFilterColumnVisibility()}
            containerRuntimeFilter={workloadsState.containerRuntimeFilterConfig()}
            hostFilter={undefined}
            namespaceFilter={undefined}
            platformFilter={undefined}
            suppressTypeFilter
            metricDisplayMode={workloadsState.workloadMetricDisplayMode}
            setMetricDisplayMode={workloadsState.setWorkloadMetricDisplayMode}
            metricHistoryRange={workloadsState.workloadMetricHistoryRange}
            setMetricHistoryRange={workloadsState.setWorkloadMetricHistoryRange}
            forcedPlatform={TRUENAS_PLATFORM_FILTER}
            pinnedSelectionActive={() =>
              Boolean(
                workloadsState.selectedGuestId() || workloadsState.focusedSummaryWorkloadGroupId(),
              )
            }
            onClearPinnedSelection={workloadsState.clearPinnedSummaryScope}
          />
        </div>
      </Show>
      <TrueNASSystemsTable
        systems={filteredSystems()}
        scope={props.model().resources}
        emptyIcon={truenasIcon()}
        emptyTitle="No TrueNAS systems"
        emptyDescription="TrueNAS systems appear here once a TrueNAS connection reports its top-level appliance."
        showToolbar={false}
      />
      <Show when={props.model().apps.length > 0}>
        <WorkloadsSurface
          state={workloadsState}
          vms={[]}
          containers={[]}
          nodes={[]}
          useWorkloads
          embedded
          tableOnly
          forcedPlatform={TRUENAS_PLATFORM_FILTER}
          forcedViewMode="app-container"
          compactGroupHeaders
        />
      </Show>
    </div>
  );
}

export default TrueNASPageSurface;
