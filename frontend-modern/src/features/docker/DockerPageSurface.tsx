import ContainerIcon from 'lucide-solid/icons/container';
import { Show, createMemo } from 'solid-js';
import { WorkloadsFilter } from '@/components/Workloads/WorkloadsFilter';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import type { WorkloadsStatusOption } from '@/components/Workloads/workloadsFilterModel';
import { useWorkloadsState } from '@/components/Workloads/useWorkloadsState';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformTableEmptyState,
} from '@/features/platformPage/sharedPlatformPage';
import { DockerHostsTable } from './DockerHostsTable';
import { DockerServicesTable } from './DockerServicesTable';
import {
  buildDockerPageModel,
  buildDockerContainerDefaultHiddenColumnIds,
  buildDockerWorkloadGroupLabelBadges,
  filterDockerHosts,
  filterDockerServices,
} from './dockerPageModel';

const DOCKER_RESOURCE_QUERY = 'type=agent,docker-host,app-container,docker-service';
const DOCKER_PLATFORM_FILTER = 'docker';
const DOCKER_WORKLOAD_FORCED_VIEW_MODE = 'app-container';
const DOCKER_WORKLOAD_DEFAULT_SORT_KEY = 'name';
const DOCKER_WORKLOAD_COLUMN_SCOPE = 'docker-runtime-containers';
const DOCKER_WORKLOAD_COLUMN_LABEL_OVERRIDES = {
  context: 'Host',
  disk: 'Writable layer',
} as const;
const DOCKER_WORKLOAD_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'degraded', label: 'Attention' },
  { value: 'stopped', label: 'Stopped' },
];

const dockerIcon = () => <ContainerIcon class="h-6 w-6 text-slate-400" />;

export function DockerPageSurface() {
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: DOCKER_RESOURCE_QUERY,
    cacheKey: 'docker-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const model = createMemo(() => buildDockerPageModel(resources()));
  const dockerWorkloadDefaultHiddenColumns = createMemo(() =>
    buildDockerContainerDefaultHiddenColumnIds(model().containers),
  );
  const dockerWorkloadGroupLabelBadges = createMemo(() =>
    buildDockerWorkloadGroupLabelBadges(model().hosts),
  );
  const workloadsState = useWorkloadsState({
    vms: [],
    containers: [],
    nodes: [],
    useWorkloads: true,
    embedded: true,
    tableOnly: true,
    forcedPlatform: DOCKER_PLATFORM_FILTER,
    forcedViewMode: DOCKER_WORKLOAD_FORCED_VIEW_MODE,
    defaultSortKey: DOCKER_WORKLOAD_DEFAULT_SORT_KEY,
    showFilterToolbar: true,
    suppressPlatformFilter: true,
    allowEmbeddedScopeFilters: true,
    columnVisibilityStorageScope: DOCKER_WORKLOAD_COLUMN_SCOPE,
    additionalDefaultHiddenColumnIds: dockerWorkloadDefaultHiddenColumns(),
    columnLabelOverrides: DOCKER_WORKLOAD_COLUMN_LABEL_OVERRIDES,
    groupLabelBadges: dockerWorkloadGroupLabelBadges(),
    compactGroupHeaders: true,
    groupNodeDrawerMode: 'disabled',
  });
  const pageFilters = createMemo(() => ({
    containerRuntime: workloadsState.containerRuntime().trim() || null,
    searchTerm: workloadsState.search().trim() || null,
    selectedHostScope: workloadsState.selectedNode(),
    statusMode: workloadsState.statusMode(),
  }));
  const filteredHosts = createMemo(() => filterDockerHosts(model().hosts, pageFilters()));
  const filteredServices = createMemo(() => filterDockerServices(model().services, pageFilters()));
  const showSharedFilterToolbar = createMemo(
    () =>
      workloadsState.surfaceConnected() &&
      workloadsState.surfaceInitialDataReceived() &&
      workloadsState.allGuests().length > 0,
  );
  const showServicesSection = createMemo(
    () => pageFilters().containerRuntime?.toLowerCase() !== 'podman' && model().services.length > 0,
  );

  return (
    <div data-testid="docker-page" class="space-y-3">
      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableEmptyState
            icon={dockerIcon()}
            title="Loading Docker resources"
            description="Pulse is loading the Docker / Podman resource snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load Docker resources"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show
            when={model().resources.length > 0}
            fallback={
              <PlatformTableEmptyState
                icon={dockerIcon()}
                title="No Docker or Podman hosts"
                description="Install the Pulse agent on a Docker or Podman host to populate this platform page."
              />
            }
          >
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
                    defaultSortKey={DOCKER_WORKLOAD_DEFAULT_SORT_KEY}
                    setSortKey={workloadsState.setSortKey}
                    setSortDirection={workloadsState.setSortDirection}
                    onBeforeAutoFocus={workloadsState.handleBeforeAutoFocus}
                    ariaLabel="Docker workload filters"
                    searchPlaceholder="Search containers by name, image, host, or runtime"
                    searchEmptyMessage="Recent Docker container searches appear here."
                    statusOptions={DOCKER_WORKLOAD_STATUS_OPTIONS}
                    columnVisibility={workloadsState.workloadsFilterColumnVisibility()}
                    containerRuntimeFilter={workloadsState.containerRuntimeFilterConfig()}
                    hostFilter={undefined}
                    platformFilter={undefined}
                    namespaceFilter={undefined}
                    suppressTypeFilter
                    metricDisplayMode={workloadsState.workloadMetricDisplayMode}
                    setMetricDisplayMode={workloadsState.setWorkloadMetricDisplayMode}
                    metricHistoryRange={workloadsState.workloadMetricHistoryRange}
                    setMetricHistoryRange={workloadsState.setWorkloadMetricHistoryRange}
                    forcedPlatform={DOCKER_PLATFORM_FILTER}
                    pinnedSelectionActive={() =>
                      Boolean(
                        workloadsState.selectedGuestId() ||
                        workloadsState.focusedSummaryWorkloadGroupId(),
                      )
                    }
                    onClearPinnedSelection={workloadsState.clearPinnedSummaryScope}
                  />
                </div>
              </Show>
              <DockerHostsTable
                resources={filteredHosts()}
                sourceCount={model().hosts.length}
                emptyIcon={dockerIcon()}
                emptyTitle="No Docker or Podman hosts"
                emptyDescription="Container hosts appear here once a Pulse agent registers them."
                showToolbar={false}
              />
              <WorkloadsSurface
                state={workloadsState}
                vms={[]}
                containers={[]}
                nodes={[]}
                useWorkloads
                embedded
                tableOnly
                forcedPlatform={DOCKER_PLATFORM_FILTER}
                forcedViewMode={DOCKER_WORKLOAD_FORCED_VIEW_MODE}
                groupNodeDrawerMode="disabled"
                emptyStateTitle="No Docker or Podman containers"
                emptyStateDescription="Containers appear here when a Docker or Podman host reports workload inventory."
              />
              <Show when={showServicesSection()}>
                <DockerServicesTable
                  resources={filteredServices()}
                  sourceCount={model().services.length}
                  emptyIcon={dockerIcon()}
                  emptyTitle="No Swarm services"
                  emptyDescription="Docker Swarm services appear here when a Swarm manager reports them."
                  showToolbar={false}
                />
              </Show>
            </div>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default DockerPageSurface;
