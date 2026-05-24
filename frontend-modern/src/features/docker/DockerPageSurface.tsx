import { useLocation } from '@solidjs/router';
import ContainerIcon from 'lucide-solid/icons/container';
import { Show, createMemo } from 'solid-js';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
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
  DOCKER_TAB_SPECS,
  buildDockerPageModel,
  resolveDockerPageTabId,
  type DockerPageModel,
  type DockerPageTabId,
} from './dockerPageModel';

const DOCKER_RESOURCE_QUERY =
  'type=agent,docker-host,app-container,docker-service,docker-image,docker-volume,docker-network,docker-task,docker-swarm-node,docker-secret,docker-config';

const dockerIcon = () => <ContainerIcon class="h-6 w-6 text-slate-400" />;

export function DockerPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: DOCKER_RESOURCE_QUERY,
    cacheKey: 'docker-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const activeTab = createMemo<DockerPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1];
    return resolveDockerPageTabId(segment);
  });
  const model = createMemo(() => buildDockerPageModel(resources()));
  const showServicesSection = createMemo(() => model().services.length > 0);

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
                hosts={model().hosts}
                hostSourceCount={model().hosts.length}
                containers={model().containers}
                services={model().services}
                serviceSourceCount={model().services.length}
                showServicesSection={showServicesSection()}
              />
            </Show>
            <Show when={activeTab() === 'containers'}>
              <DockerContainersTable
                resources={model().containers}
                emptyIcon={dockerIcon()}
                emptyTitle="No Docker or Podman containers"
                emptyDescription="Containers appear here when a Docker or Podman host reports workload inventory."
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
  return (
    <div class="space-y-4">
      <DockerStorageUsageTable
        hosts={props.model.hosts}
        sourceCount={props.model.hosts.length}
        emptyIcon={dockerIcon()}
        emptyTitle="No Docker or Podman storage usage"
        emptyDescription="Engine disk-usage snapshots appear here when a Docker or Podman host reports them."
      />
      <DockerVolumesTable
        resources={props.model.volumes}
        emptyIcon={dockerIcon()}
        emptyTitle="No volumes"
        emptyDescription="Volumes appear here when the container runtime reports volume inventory."
      />
    </div>
  );
}

function DockerSwarm(props: { model: DockerPageModel }) {
  const hasSwarmInventory = createMemo(
    () =>
      props.model.services.length > 0 ||
      props.model.tasks.length > 0 ||
      props.model.nodes.length > 0 ||
      props.model.secrets.length > 0 ||
      props.model.configs.length > 0,
  );

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
  services: ReturnType<typeof buildDockerPageModel>['services'];
  serviceSourceCount: number;
  showServicesSection: boolean;
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
        showToolbar={false}
      />
      <Show when={props.showServicesSection}>
        <DockerServicesTable
          resources={props.services}
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
