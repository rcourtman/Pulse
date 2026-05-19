import ContainerIcon from 'lucide-solid/icons/container';
import { Show, createMemo } from 'solid-js';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
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
                forcedViewMode={DOCKER_WORKLOAD_FORCED_VIEW_MODE}
                defaultSortKey={DOCKER_WORKLOAD_DEFAULT_SORT_KEY}
                columnVisibilityStorageScope={DOCKER_WORKLOAD_COLUMN_SCOPE}
                additionalDefaultHiddenColumnIds={dockerWorkloadDefaultHiddenColumns()}
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
        </Show>
      </Show>
    </div>
  );
}

export default DockerPageSurface;
