import { useLocation } from '@solidjs/router';
import ContainerIcon from 'lucide-solid/icons/container';
import { Show, createMemo } from 'solid-js';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
} from '@/features/platformPage/sharedPlatformPage';
import { DockerHostsTable } from './DockerHostsTable';
import { DockerServicesTable } from './DockerServicesTable';
import {
  DOCKER_TAB_SPECS,
  buildDockerPageModel,
  buildVisibleDockerTabSpecs,
  type DockerPageTabId,
} from './dockerPageModel';

const DOCKER_RESOURCE_QUERY = 'type=agent,docker-host,app-container,docker-service';
const DOCKER_PLATFORM_FILTER = 'docker';
const DOCKER_WORKLOAD_COLUMN_SCOPE = 'docker';
const DOCKER_WORKLOAD_DEFAULT_HIDDEN_COLUMNS = ['disk'];
const DOCKER_WORKLOAD_COLUMN_LABEL_OVERRIDES = {
  disk: 'Writable layer',
} as const;
const VALID_TABS = new Set<DockerPageTabId>(DOCKER_TAB_SPECS.map((tab) => tab.id));

const dockerIcon = () => <ContainerIcon class="h-6 w-6 text-slate-400" />;

export function DockerPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: DOCKER_RESOURCE_QUERY,
    cacheKey: 'docker-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const model = createMemo(() => buildDockerPageModel(resources()));
  const visibleTabs = createMemo(() => buildVisibleDockerTabSpecs(model()));
  const visibleTabIds = createMemo(
    () => new Set<DockerPageTabId>(visibleTabs().map((tab) => tab.id)),
  );
  const activeTab = createMemo<DockerPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as DockerPageTabId | undefined;
    if (!segment || !VALID_TABS.has(segment)) return 'overview';
    return visibleTabIds().has(segment) ? segment : 'overview';
  });

  return (
    <div data-testid="docker-page" class="space-y-3">
      <PlatformSectionTabs
        tabs={visibleTabs()}
        active={activeTab()}
        ariaLabel="Docker sections"
      />

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
            <Show when={activeTab() === 'overview'}>
              <div class="space-y-4">
                <DockerHostsTable
                  resources={model().hosts}
                  emptyIcon={dockerIcon()}
                  emptyTitle="No Docker hosts"
                  emptyDescription="Container hosts appear here once a Pulse agent registers them."
                  showToolbar={false}
                />
                <WorkloadsSurface
                  vms={[]}
                  containers={[]}
                  nodes={[]}
                  useWorkloads
                  embedded
                  tableOnly
                  showFilterToolbar
                  suppressPlatformFilter
                  forcedPlatform={DOCKER_PLATFORM_FILTER}
                  columnVisibilityStorageScope={DOCKER_WORKLOAD_COLUMN_SCOPE}
                  additionalDefaultHiddenColumnIds={DOCKER_WORKLOAD_DEFAULT_HIDDEN_COLUMNS}
                  columnLabelOverrides={DOCKER_WORKLOAD_COLUMN_LABEL_OVERRIDES}
                  compactGroupHeaders
                />
                <Show when={model().services.length > 0}>
                  <DockerServicesTable
                    resources={model().services}
                    emptyIcon={dockerIcon()}
                    emptyTitle="No Swarm services"
                    emptyDescription="Docker Swarm services appear here when a Swarm manager reports them."
                    showToolbar={false}
                  />
                </Show>
              </div>
            </Show>
            <Show when={activeTab() === 'containers'}>
              <WorkloadsSurface
                vms={[]}
                containers={[]}
                nodes={[]}
                useWorkloads
                embedded
                tableOnly
                showFilterToolbar
                suppressPlatformFilter
                forcedPlatform={DOCKER_PLATFORM_FILTER}
                columnVisibilityStorageScope={DOCKER_WORKLOAD_COLUMN_SCOPE}
                additionalDefaultHiddenColumnIds={DOCKER_WORKLOAD_DEFAULT_HIDDEN_COLUMNS}
                columnLabelOverrides={DOCKER_WORKLOAD_COLUMN_LABEL_OVERRIDES}
              />
            </Show>
            <Show when={activeTab() === 'services'}>
              <DockerServicesTable
                resources={model().services}
                emptyIcon={dockerIcon()}
                emptyTitle="No Swarm services"
                emptyDescription="Docker Swarm services appear here when a Swarm manager reports them."
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default DockerPageSurface;
