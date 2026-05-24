import { useLocation } from '@solidjs/router';
import ContainerIcon from 'lucide-solid/icons/container';
import { Show, createMemo } from 'solid-js';
import { WorkloadsFilter } from '@/components/Workloads/WorkloadsFilter';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import type { WorkloadsStatusOption } from '@/components/Workloads/workloadsFilterModel';
import { useWorkloadsState } from '@/components/Workloads/useWorkloadsState';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import { APP_CONTAINER_COLUMN_LABEL_OVERRIDES } from '@/features/platformPage/appContainerColumns';
import { DockerHostsTable } from './DockerHostsTable';
import { DockerInventoryTable } from './DockerInventoryTable';
import { DockerServicesTable } from './DockerServicesTable';
import {
  DOCKER_TAB_SPECS,
  buildDockerPageModel,
  buildDockerContainerDefaultHiddenColumnIds,
  buildDockerWorkloadGroupLabelBadges,
  filterDockerHosts,
  filterDockerServices,
  type DockerPageTabId,
} from './dockerPageModel';

const DOCKER_RESOURCE_QUERY =
  'type=agent,docker-host,app-container,docker-service,docker-image,docker-volume,docker-network,docker-task';
const DOCKER_PLATFORM_FILTER = 'docker';
const DOCKER_WORKLOAD_FORCED_VIEW_MODE = 'app-container';
const DOCKER_WORKLOAD_DEFAULT_SORT_KEY = 'name';
const DOCKER_WORKLOAD_COLUMN_SCOPE = 'docker-runtime-containers';
const DOCKER_WORKLOAD_COLUMN_LABEL_OVERRIDES = {
  ...APP_CONTAINER_COLUMN_LABEL_OVERRIDES,
  context: 'Host',
} as const;
const DOCKER_WORKLOAD_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'degraded', label: 'Attention' },
  { value: 'stopped', label: 'Stopped' },
];
const VALID_TABS = new Set<DockerPageTabId>(DOCKER_TAB_SPECS.map((tab) => tab.id));

const dockerIcon = () => <ContainerIcon class="h-6 w-6 text-slate-400" />;

export function DockerPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: DOCKER_RESOURCE_QUERY,
    cacheKey: 'docker-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const activeTab = createMemo<DockerPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as DockerPageTabId | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
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
      <PlatformSectionTabs
        tabs={DOCKER_TAB_SPECS}
        active={activeTab()}
        ariaLabel="Container runtime sections"
      />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableLoadingState
            title="Loading container runtime resources"
            description="Pulse is loading the Docker / Podman resource snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load container runtime resources"
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
                description="Install the Pulse agent on a Docker or Podman host to populate this runtime lens."
              />
            }
          >
            <Show when={activeTab() === 'overview'}>
              <DockerOverview
                showSharedFilterToolbar={showSharedFilterToolbar()}
                workloadsState={workloadsState}
                hosts={filteredHosts()}
                hostSourceCount={model().hosts.length}
                filteredServices={filteredServices()}
                serviceSourceCount={model().services.length}
                showServicesSection={showServicesSection()}
              />
            </Show>
            <Show when={activeTab() === 'containers'}>
              <DockerContainers workloadsState={workloadsState} />
            </Show>
            <Show when={activeTab() === 'images'}>
              <DockerInventoryTable
                resources={model().images}
                variant="images"
                emptyIcon={dockerIcon()}
                emptyTitle="No images"
                emptyDescription="Images appear here when a Docker or Podman host reports local image inventory."
              />
            </Show>
            <Show when={activeTab() === 'volumes'}>
              <DockerInventoryTable
                resources={model().volumes}
                variant="volumes"
                emptyIcon={dockerIcon()}
                emptyTitle="No volumes"
                emptyDescription="Volumes appear here when the container runtime reports volume inventory."
              />
            </Show>
            <Show when={activeTab() === 'networks'}>
              <DockerInventoryTable
                resources={model().networks}
                variant="networks"
                emptyIcon={dockerIcon()}
                emptyTitle="No networks"
                emptyDescription="Networks appear here when the container runtime reports network inventory."
              />
            </Show>
            <Show when={activeTab() === 'services'}>
              <DockerServicesTable
                resources={model().services}
                sourceCount={model().services.length}
                emptyIcon={dockerIcon()}
                emptyTitle="No Swarm services"
                emptyDescription="Docker Swarm services appear here when a Swarm manager reports them."
              />
            </Show>
            <Show when={activeTab() === 'tasks'}>
              <DockerInventoryTable
                resources={model().tasks}
                variant="tasks"
                emptyIcon={dockerIcon()}
                emptyTitle="No Swarm tasks"
                emptyDescription="Swarm tasks appear here when a Swarm manager reports replica task inventory."
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default DockerPageSurface;

type DockerWorkloadsState = ReturnType<typeof useWorkloadsState>;

function DockerWorkloadsFilter(props: { workloadsState: DockerWorkloadsState }) {
  return (
    <div data-summary-clear-ignore>
      <WorkloadsFilter
        search={props.workloadsState.search}
        setSearch={props.workloadsState.setSearch}
        viewMode={props.workloadsState.viewMode}
        setViewMode={props.workloadsState.setViewMode}
        statusMode={props.workloadsState.statusMode}
        setStatusMode={props.workloadsState.setStatusMode}
        groupingMode={props.workloadsState.groupingMode}
        setGroupingMode={props.workloadsState.setGroupingMode}
        defaultSortKey={DOCKER_WORKLOAD_DEFAULT_SORT_KEY}
        setSortKey={props.workloadsState.setSortKey}
        setSortDirection={props.workloadsState.setSortDirection}
        onBeforeAutoFocus={props.workloadsState.handleBeforeAutoFocus}
        ariaLabel="Container runtime workload filters"
        searchPlaceholder="Search containers by name, image, host, or runtime"
        searchEmptyMessage="Recent Docker container searches appear here."
        statusOptions={DOCKER_WORKLOAD_STATUS_OPTIONS}
        columnVisibility={props.workloadsState.workloadsFilterColumnVisibility()}
        containerRuntimeFilter={props.workloadsState.containerRuntimeFilterConfig()}
        hostFilter={undefined}
        platformFilter={undefined}
        namespaceFilter={undefined}
        suppressTypeFilter
        metricDisplayMode={props.workloadsState.workloadMetricDisplayMode}
        setMetricDisplayMode={props.workloadsState.setWorkloadMetricDisplayMode}
        metricHistoryRange={props.workloadsState.workloadMetricHistoryRange}
        setMetricHistoryRange={props.workloadsState.setWorkloadMetricHistoryRange}
        forcedPlatform={DOCKER_PLATFORM_FILTER}
        pinnedSelectionActive={() =>
          Boolean(
            props.workloadsState.selectedGuestId() ||
            props.workloadsState.focusedSummaryWorkloadGroupId(),
          )
        }
        onClearPinnedSelection={props.workloadsState.clearPinnedSummaryScope}
      />
    </div>
  );
}

function DockerContainers(props: { workloadsState: DockerWorkloadsState }) {
  return (
    <div class="space-y-4">
      <DockerWorkloadsFilter workloadsState={props.workloadsState} />
      <WorkloadsSurface
        state={props.workloadsState}
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
    </div>
  );
}

function DockerOverview(props: {
  showSharedFilterToolbar: boolean;
  workloadsState: DockerWorkloadsState;
  hosts: ReturnType<typeof buildDockerPageModel>['hosts'];
  hostSourceCount: number;
  filteredServices: ReturnType<typeof buildDockerPageModel>['services'];
  serviceSourceCount: number;
  showServicesSection: boolean;
}) {
  return (
    <div class="space-y-4">
      <Show when={props.showSharedFilterToolbar}>
        <DockerWorkloadsFilter workloadsState={props.workloadsState} />
      </Show>
      <DockerHostsTable
        resources={props.hosts}
        sourceCount={props.hostSourceCount}
        emptyIcon={dockerIcon()}
        emptyTitle="No Docker or Podman hosts"
        emptyDescription="Container hosts appear here once a Pulse agent registers them."
        showToolbar={false}
      />
      <WorkloadsSurface
        state={props.workloadsState}
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
      <Show when={props.showServicesSection}>
        <DockerServicesTable
          resources={props.filteredServices}
          sourceCount={props.serviceSourceCount}
          emptyIcon={dockerIcon()}
          emptyTitle="No Swarm services"
          emptyDescription="Docker Swarm services appear here when a Swarm manager reports them."
          showToolbar={false}
        />
      </Show>
    </div>
  );
}
