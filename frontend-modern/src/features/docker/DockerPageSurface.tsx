import { useLocation } from '@solidjs/router';
import { Show, createMemo, createSignal } from 'solid-js';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
  PlatformTableToolbar,
} from '@/features/platformPage/sharedPlatformPage';
import { DockerAlertsTable } from './DockerAlertsTable';
import { DockerConfigsTable } from './DockerConfigsTable';
import { DockerContainersTable } from './DockerContainersTable';
import { DockerHostsTable } from './DockerHostsTable';
import { DockerImagesTable } from './DockerImagesTable';
import { DockerNetworksTable } from './DockerNetworksTable';
import { DockerSecretsTable } from './DockerSecretsTable';
import { DockerServicesTable } from './DockerServicesTable';
import { DockerStorageUsageTable } from './DockerStorageUsageTable';
import { DockerSwarmNodesTable } from './DockerSwarmNodesTable';
import { DockerTasksTable } from './DockerTasksTable';
import { DockerVolumesTable } from './DockerVolumesTable';
import {
  buildDockerPageModel,
  filterDockerResources,
  getDockerPageTabSpecs,
  hasDockerEngineStorageUsage,
  hasDockerStorageInventory,
  hasDockerSwarmInventory,
  resolveDockerPageTabId,
  type DockerPageModel,
  type DockerPageTabId,
  type DockerResourceStatusFilter,
} from './dockerPageModel';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import { updateStore } from '@/stores/updates';

const DOCKER_RESOURCE_QUERY =
  'type=agent,docker-host,app-container,docker-service,docker-image,docker-volume,docker-network,docker-task,docker-swarm-node,docker-secret,docker-config';

const DockerIcon = getPlatformIcon('docker');
const dockerIcon = () => <DockerIcon class="h-6 w-6 text-slate-400" />;

export function DockerPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: DOCKER_RESOURCE_QUERY,
    cacheKey: 'docker-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const requestedTab = createMemo<DockerPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1];
    return resolveDockerPageTabId(segment);
  });
  const model = createMemo(() => buildDockerPageModel(resources()));
  const tabs = createMemo(() => getDockerPageTabSpecs(model()));
  const activeTab = createMemo<DockerPageTabId>(() =>
    tabs().some((tab) => tab.id === requestedTab()) ? requestedTab() : 'overview',
  );
  const outdatedAgentHosts = createMemo(() =>
    collectOutdatedAgentHosts(model().hosts, updateStore.versionInfo()?.version),
  );
  const serverVersionDisplay = createMemo(() =>
    formatAgentVersionDisplay(updateStore.versionInfo()?.version),
  );

  return (
    <div data-testid="docker-page" class="space-y-3">
      <PlatformSectionTabs
        tabs={tabs()}
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
            <PlatformOutdatedAgentNotice
              hosts={outdatedAgentHosts()}
              targetVersion={serverVersionDisplay()}
              missingLabel="images, networks, storage, and Swarm details"
            />
            <Show when={activeTab() === 'overview'}>
              <DockerOverview
                hosts={model().hosts}
                hostSourceCount={model().hosts.length}
                containers={model().containers}
                incidents={model().incidents}
              />
            </Show>
            <Show when={activeTab() === 'images'}>
              <DockerImagesTable
                resources={model().images}
                emptyIcon={dockerIcon()}
                emptyTitle="No images"
                emptyDescription="Images appear here when a Docker or Podman host reports local image inventory."
              />
            </Show>
            <Show when={activeTab() === 'networks'}>
              <DockerNetworksTable
                resources={model().networks}
                emptyIcon={dockerIcon()}
                emptyTitle="No networks"
                emptyDescription="Networks appear here when the container runtime reports network inventory."
              />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <DockerStorage model={model()} />
            </Show>
            <Show when={activeTab() === 'swarm'}>
              <DockerSwarm model={model()} />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default DockerPageSurface;

function DockerStorage(props: { model: DockerPageModel }) {
  const hasEngineUsage = createMemo(() => props.model.hosts.some(hasDockerEngineStorageUsage));
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<DockerResourceStatusFilter>('all');
  const storageHosts = createMemo(() => props.model.hosts.filter(hasDockerEngineStorageUsage));
  const totalRows = createMemo(() => storageHosts().length + props.model.volumes.length);
  const visibleRows = createMemo(
    () =>
      filterDockerResources(storageHosts(), search(), status()).length +
      filterDockerResources(props.model.volumes, search(), status()).length,
  );
  const hasActiveFilters = createMemo(() => search().trim().length > 0 || status() !== 'all');
  const resetFilters = () => {
    setSearch('');
    setStatus('all');
  };

  return (
    <Show
      when={hasDockerStorageInventory(props.model)}
      fallback={
        <PlatformTableEmptyState
          icon={dockerIcon()}
          title="No Docker or Podman storage inventory"
          description="Engine disk-usage snapshots and volumes appear here when Docker or Podman hosts report storage inventory."
        />
      }
    >
      <div class="space-y-4">
        <PlatformTableToolbar
          search={search}
          onSearchChange={setSearch}
          searchPlaceholder="Search storage usage and volumes"
          status={status()}
          onStatusChange={setStatus}
          statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
          visible={visibleRows()}
          total={totalRows()}
          rowNoun="rows"
          hasActiveFilters={hasActiveFilters()}
          onResetFilters={resetFilters}
        />
        <Show when={hasEngineUsage()}>
          <DockerStorageUsageTable
            hosts={props.model.hosts}
            sourceCount={props.model.hosts.length}
            emptyIcon={dockerIcon()}
            emptyTitle="No Docker or Podman storage usage"
            emptyDescription="Engine disk-usage snapshots appear here when a Docker or Podman host reports them."
            showToolbar={false}
            externalSearch={search}
            externalStatus={status}
          />
        </Show>
        <Show when={props.model.volumes.length > 0}>
          <DockerVolumesTable
            resources={props.model.volumes}
            emptyIcon={dockerIcon()}
            emptyTitle="No volumes"
            emptyDescription="Volumes appear here when the container runtime reports volume inventory."
            showToolbar={false}
            externalSearch={search}
            externalStatus={status}
          />
        </Show>
      </div>
    </Show>
  );
}

function DockerSwarm(props: { model: DockerPageModel }) {
  const hasSwarmInventory = createMemo(() => hasDockerSwarmInventory(props.model));

  return (
    <Show
      when={hasSwarmInventory()}
      fallback={
        <PlatformTableEmptyState
          icon={dockerIcon()}
          title="No Swarm inventory"
          description="Docker Swarm services, tasks, nodes, secrets, and configs appear here when a Swarm manager reports them."
        />
      }
    >
      <div class="space-y-4">
        <Show when={props.model.services.length > 0}>
          <DockerServicesTable
            resources={props.model.services}
            sourceCount={props.model.services.length}
            emptyIcon={dockerIcon()}
            emptyTitle="No Swarm services"
            emptyDescription="Docker Swarm services appear here when a Swarm manager reports them."
          />
        </Show>
        <Show when={props.model.tasks.length > 0}>
          <DockerTasksTable
            resources={props.model.tasks}
            emptyIcon={dockerIcon()}
            emptyTitle="No Swarm tasks"
            emptyDescription="Docker Swarm tasks appear here when a Swarm manager reports task state."
          />
        </Show>
        <Show when={props.model.nodes.length > 0}>
          <DockerSwarmNodesTable
            resources={props.model.nodes}
            emptyIcon={dockerIcon()}
            emptyTitle="No Swarm nodes"
            emptyDescription="Swarm nodes appear here when a Swarm manager reports cluster membership."
          />
        </Show>
        <Show when={props.model.secrets.length > 0}>
          <DockerSecretsTable
            resources={props.model.secrets}
            emptyIcon={dockerIcon()}
            emptyTitle="No Swarm secrets"
            emptyDescription="Docker Swarm secrets appear here when a Swarm manager reports secret metadata."
          />
        </Show>
        <Show when={props.model.configs.length > 0}>
          <DockerConfigsTable
            resources={props.model.configs}
            emptyIcon={dockerIcon()}
            emptyTitle="No Swarm configs"
            emptyDescription="Docker Swarm configs appear here when a Swarm manager reports config metadata."
          />
        </Show>
      </div>
    </Show>
  );
}

function DockerOverview(props: {
  hosts: ReturnType<typeof buildDockerPageModel>['hosts'];
  hostSourceCount: number;
  containers: ReturnType<typeof buildDockerPageModel>['containers'];
  incidents: ReturnType<typeof buildDockerPageModel>['incidents'];
}) {
  return (
    <div class="space-y-4">
      <DockerHostsTable
        resources={props.hosts}
        sourceCount={props.hostSourceCount}
        emptyIcon={dockerIcon()}
        emptyTitle="No Docker or Podman hosts"
        emptyDescription="Container hosts appear here once a Pulse agent registers them."
        showToolbar={false}
      />
      <DockerContainersTable
        resources={props.containers}
        emptyIcon={dockerIcon()}
        emptyTitle="No Docker or Podman containers"
        emptyDescription="Containers appear here when a Docker or Podman host reports workload inventory."
      />
      <Show when={props.incidents.length > 0}>
        <DockerAlertsTable
          incidents={props.incidents}
          emptyIcon={dockerIcon()}
          emptyTitle="No active Docker alerts"
          emptyDescription="Docker health alerts appear here when the Pulse alert engine reports active container, host, or Swarm incidents."
        />
      </Show>
    </div>
  );
}
