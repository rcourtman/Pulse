import { useLocation, useSearchParams } from '@solidjs/router';
import { Show, createMemo, createSignal, type Accessor } from 'solid-js';
import { buildInfrastructureAgentUpdatesPath } from '@/components/Settings/infrastructureWorkspaceModel';
import type { FilterDef } from '@/components/shared/FilterBar';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { updateStore } from '@/stores/updates';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
  PlatformTableToolbar,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import { KubernetesAlertsTable } from './KubernetesAlertsTable';
import { KubernetesAutoscalingTable } from './KubernetesAutoscalingTable';
import { KubernetesClustersTable } from './KubernetesClustersTable';
import { KubernetesConfigTable } from './KubernetesConfigTable';
import { KubernetesControllersTable } from './KubernetesControllersTable';
import { KubernetesDeploymentsTable } from './KubernetesDeploymentsTable';
import { KubernetesEventsTable } from './KubernetesEventsTable';
import { KubernetesNetworkingTable } from './KubernetesNetworkingTable';
import { KubernetesNodesTable } from './KubernetesNodesTable';
import { KubernetesPodsTable } from './KubernetesPodsTable';
import { KubernetesPolicyTable } from './KubernetesPolicyTable';
import { KubernetesServicesTable } from './KubernetesServicesTable';
import { KubernetesStorageTable } from './KubernetesStorageTable';
import {
  buildKubernetesPageModel,
  filterKubernetesResources,
  getKubernetesPageTabSpecs,
  resolveKubernetesPageTabId,
  type KubernetesPageModel,
  type KubernetesPageTabId,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

// Include `agent` rows so K8s nodes that the backend registry merged onto
// the linked agent host (sources=['agent','kubernetes']) still appear in the
// Nodes section of the Overview stack; the page model filters them down to
// those tagged kubernetes.
const KUBERNETES_RESOURCE_QUERY =
  'type=k8s-cluster,k8s-node,pod,k8s-deployment,k8s-replicaset,k8s-namespace,k8s-service,k8s-statefulset,k8s-daemonset,k8s-job,k8s-cronjob,k8s-ingress,k8s-endpoint-slice,k8s-network-policy,k8s-persistent-volume,k8s-persistent-volume-claim,k8s-storage-class,k8s-configmap,k8s-secret,k8s-serviceaccount,k8s-role,k8s-cluster-role,k8s-role-binding,k8s-cluster-role-binding,k8s-resource-quota,k8s-limit-range,k8s-pod-disruption-budget,k8s-horizontal-pod-autoscaler,k8s-event,agent';

const KubernetesIcon = getPlatformIcon('kubernetes');
const k8sIcon = () => <KubernetesIcon class="h-6 w-6 text-slate-400" />;

export function KubernetesPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: KUBERNETES_RESOURCE_QUERY,
    cacheKey: 'kubernetes-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const requestedTab = createMemo<KubernetesPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1];
    return resolveKubernetesPageTabId(segment);
  });
  const model = createMemo(() => buildKubernetesPageModel(resources()));
  const tabs = createMemo(() => getKubernetesPageTabSpecs(model()));
  const activeTab = createMemo<KubernetesPageTabId>(() =>
    tabs().some((tab) => tab.id === requestedTab()) ? requestedTab() : 'overview',
  );
  const controllerResources = createMemo(() => getKubernetesControllerResources(model()));
  const outdatedAgentHosts = createMemo(() =>
    collectOutdatedAgentHosts(model().nodes, updateStore.versionInfo()?.version),
  );
  const outdatedAgentUpdatePath = createMemo(() =>
    buildInfrastructureAgentUpdatesPath(outdatedAgentHosts().map((host) => host.agentId)),
  );
  const serverVersionDisplay = createMemo(() =>
    formatAgentVersionDisplay(updateStore.versionInfo()?.version),
  );

  return (
    <div data-testid="kubernetes-page" class="space-y-3">
      <PlatformSectionTabs tabs={tabs()} active={activeTab()} ariaLabel="Kubernetes sections" />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableLoadingState
            title="Loading Kubernetes resources"
            description="Pulse is loading the Kubernetes resource snapshot."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load Kubernetes resources"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <Show
            when={model().resources.length > 0}
            fallback={
              <PlatformTableEmptyState
                icon={k8sIcon()}
                title="No Kubernetes clusters"
                description="Install the Pulse agent on a Kubernetes node to populate this platform page."
              />
            }
          >
            <PlatformOutdatedAgentNotice
              hosts={outdatedAgentHosts()}
              targetVersion={serverVersionDisplay()}
              missingLabel="Kubernetes nodes, workloads, services, storage, configuration, and events"
              actionHref={outdatedAgentUpdatePath()}
              actionLabel="Open agent upgrade commands"
            />
            <Show when={activeTab() === 'overview'}>
              <KubernetesOverview model={model} />
            </Show>
            <Show when={activeTab() === 'nodes'}>
              <KubernetesNodesTable
                resources={model().nodes}
                emptyIcon={k8sIcon()}
                emptyTitle="No nodes reported"
                emptyDescription="Kubernetes nodes appear here as soon as the agent enumerates Node resources."
              />
            </Show>
            <Show when={activeTab() === 'workloads'}>
              <KubernetesWorkloads model={model()} controllers={controllerResources()} />
            </Show>
            <Show when={activeTab() === 'services'}>
              <KubernetesServices model={model()} />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <KubernetesStorageTable
                resources={model().storage}
                emptyIcon={k8sIcon()}
                emptyTitle="No Kubernetes volume resources reported"
                emptyDescription="StorageClasses, persistent volumes, and claims appear here once the agent can read storage inventory."
              />
            </Show>
            <Show when={activeTab() === 'configuration'}>
              <KubernetesConfiguration model={model()} />
            </Show>
            <Show when={activeTab() === 'events'}>
              <KubernetesEventsTable
                resources={model().events}
                emptyIcon={k8sIcon()}
                emptyTitle="No events reported"
                emptyDescription="Events appear here when the agent can read the Kubernetes Events API."
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

interface KubernetesOverviewProps {
  model: Accessor<KubernetesPageModel>;
}

const getKubernetesControllerResources = (model: KubernetesPageModel): Resource[] => [
  ...model.replicaSets,
  ...model.statefulSets,
  ...model.daemonSets,
  ...model.jobs,
  ...model.cronJobs,
];

type SharedToolbarState = {
  search: Accessor<string>;
  setSearch: (value: string) => void;
  status: Accessor<KubernetesResourceStatusFilter>;
  setStatus: (value: KubernetesResourceStatusFilter) => void;
  hasActiveFilters: Accessor<boolean>;
  resetFilters: () => void;
};

function createKubernetesSharedToolbar(): SharedToolbarState {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<KubernetesResourceStatusFilter>('all');
  const hasActiveFilters = createMemo(() => search().trim().length > 0 || status() !== 'all');
  const resetFilters = () => {
    setSearch('');
    setStatus('all');
  };
  return { search, setSearch, status, setStatus, hasActiveFilters, resetFilters };
}

function countKubernetesVisible(
  sections: ReadonlyArray<Resource[]>,
  search: string,
  status: KubernetesResourceStatusFilter,
): number {
  let visible = 0;
  for (const section of sections) {
    visible += filterKubernetesResources(section, search, status).length;
  }
  return visible;
}

type KubernetesNamespaceScope = {
  scopedSections: Accessor<Resource[][]>;
  scopeFilters: Accessor<FilterDef[]>;
  hasActiveNamespace: Accessor<boolean>;
  clearNamespace: () => void;
};

// Namespace scope for a shared-toolbar Kubernetes tab. Namespace is the primary
// scoping axis and the shared toolbar drives several stacked tables at once, so
// the facet pre-filters every section's resources rather than living on any one
// table. URL-backed so it is shareable and captured by saved views; the facet
// only appears when more than one namespace is present.
function createKubernetesNamespaceScope(
  sections: Accessor<Resource[][]>,
): KubernetesNamespaceScope {
  const [searchParams, setSearchParams] = useSearchParams();
  const namespaceFilter = () =>
    typeof searchParams.namespace === 'string' ? searchParams.namespace : '';
  const namespaceOf = (resource: Resource) => (resource.kubernetes?.namespace ?? '').trim();
  const matchesNamespace = (resource: Resource) => {
    const namespace = namespaceFilter();
    return !namespace || namespaceOf(resource) === namespace;
  };
  const namespaceOptions = createMemo(() => {
    const seen = new Set<string>();
    for (const section of sections()) {
      for (const resource of section) {
        const namespace = namespaceOf(resource);
        if (namespace) seen.add(namespace);
      }
    }
    return [...seen].sort((a, b) => a.localeCompare(b));
  });
  const scopedSections = createMemo<Resource[][]>(() =>
    sections().map((rows) => rows.filter(matchesNamespace)),
  );
  const scopeFilters = createMemo<FilterDef[]>(() => {
    if (namespaceOptions().length <= 1) return [];
    return [
      {
        id: 'namespace',
        label: 'Namespace',
        group: 'scope',
        options: () => [
          { value: '', label: 'All namespaces' },
          ...namespaceOptions().map((namespace) => ({ value: namespace, label: namespace })),
        ],
        value: namespaceFilter,
        setValue: (value: string) => setSearchParams({ namespace: value || null }),
        defaultValue: '',
      },
    ];
  });
  return {
    scopedSections,
    scopeFilters,
    hasActiveNamespace: () => namespaceFilter() !== '',
    clearNamespace: () => setSearchParams({ namespace: null }),
  };
}

function KubernetesWorkloads(props: { model: KubernetesPageModel; controllers: Resource[] }) {
  const hasWorkloadInventory = createMemo(
    () => props.model.workloads.length > 0 || props.model.autoscaling.length > 0,
  );
  const toolbar = createKubernetesSharedToolbar();
  const sections = createMemo<Resource[][]>(() => [
    props.model.deployments,
    props.model.pods,
    props.controllers,
    props.model.autoscaling,
  ]);
  const totalRows = createMemo(() => sections().reduce((sum, rows) => sum + rows.length, 0));

  // Namespace scope applies across every workload section at once (see the
  // shared helper). Section order here defines the scoped accessors below.
  const scope = createKubernetesNamespaceScope(sections);
  const scopedDeployments = () => scope.scopedSections()[0];
  const scopedPods = () => scope.scopedSections()[1];
  const scopedControllers = () => scope.scopedSections()[2];
  const scopedAutoscaling = () => scope.scopedSections()[3];
  const scopeFilters = scope.scopeFilters;
  const visibleRows = createMemo(() =>
    countKubernetesVisible(scope.scopedSections(), toolbar.search(), toolbar.status()),
  );
  const hasActiveFilters = () => toolbar.hasActiveFilters() || scope.hasActiveNamespace();
  const resetFilters = () => {
    toolbar.resetFilters();
    scope.clearNamespace();
  };

  return (
    <Show
      when={hasWorkloadInventory()}
      fallback={
        <PlatformTableEmptyState
          icon={k8sIcon()}
          title="No workload resources reported"
          description="Pods, Deployments, workload controllers, and HorizontalPodAutoscalers appear here once the agent can read them."
        />
      }
    >
      <div class="space-y-4">
        <PlatformTableToolbar
          search={toolbar.search}
          onSearchChange={toolbar.setSearch}
          searchPlaceholder="Search workload inventory"
          status={toolbar.status()}
          onStatusChange={toolbar.setStatus}
          statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
          filters={scopeFilters()}
          savedViewsKey="kubernetes-workloads"
          visible={visibleRows()}
          total={totalRows()}
          rowNoun="rows"
          hasActiveFilters={hasActiveFilters()}
          onResetFilters={resetFilters}
        />
        <Show when={scopedDeployments().length > 0}>
          <KubernetesDeploymentsTable
            resources={scopedDeployments()}
            emptyIcon={k8sIcon()}
            emptyTitle="No deployments reported"
            emptyDescription="Deployments appear here once the agent can read Deployment resources."
            showToolbar={false}
            externalSearch={toolbar.search}
            externalStatus={toolbar.status}
          />
        </Show>
        <Show when={scopedPods().length > 0}>
          <KubernetesPodsTable
            resources={scopedPods()}
            emptyIcon={k8sIcon()}
            emptyTitle="No pods reported"
            emptyDescription="Pods appear here once the agent can read Pod resources."
            showToolbar={false}
            externalSearch={toolbar.search}
            externalStatus={toolbar.status}
          />
        </Show>
        <Show when={scopedControllers().length > 0}>
          <KubernetesControllersTable
            resources={scopedControllers()}
            emptyIcon={k8sIcon()}
            emptyTitle="No workload controllers reported"
            emptyDescription="ReplicaSets, StatefulSets, DaemonSets, Jobs, and CronJobs appear here when the agent reports them."
            showToolbar={false}
            externalSearch={toolbar.search}
            externalStatus={toolbar.status}
          />
        </Show>
        <Show when={scopedAutoscaling().length > 0}>
          <KubernetesAutoscalingTable
            resources={scopedAutoscaling()}
            emptyIcon={k8sIcon()}
            emptyTitle="No autoscaling resources reported"
            emptyDescription="HorizontalPodAutoscalers appear here once the agent can read autoscaling inventory."
            showToolbar={false}
            externalSearch={toolbar.search}
            externalStatus={toolbar.status}
          />
        </Show>
      </div>
    </Show>
  );
}

function KubernetesServices(props: { model: KubernetesPageModel }) {
  const hasServiceInventory = createMemo(
    () => props.model.services.length > 0 || props.model.serviceNetworking.length > 0,
  );
  const toolbar = createKubernetesSharedToolbar();
  const sections = createMemo<Resource[][]>(() => [
    props.model.services,
    props.model.serviceNetworking,
  ]);
  const totalRows = createMemo(() => sections().reduce((sum, rows) => sum + rows.length, 0));
  const scope = createKubernetesNamespaceScope(sections);
  const scopedServices = () => scope.scopedSections()[0];
  const scopedNetworking = () => scope.scopedSections()[1];
  const visibleRows = createMemo(() =>
    countKubernetesVisible(scope.scopedSections(), toolbar.search(), toolbar.status()),
  );
  const hasActiveFilters = () => toolbar.hasActiveFilters() || scope.hasActiveNamespace();
  const resetFilters = () => {
    toolbar.resetFilters();
    scope.clearNamespace();
  };

  return (
    <Show
      when={hasServiceInventory()}
      fallback={
        <PlatformTableEmptyState
          icon={k8sIcon()}
          title="No service or networking resources reported"
          description="Services, ingresses, and endpoint slices appear here once the agent can read cluster traffic resources."
        />
      }
    >
      <div class="space-y-4">
        <PlatformTableToolbar
          search={toolbar.search}
          onSearchChange={toolbar.setSearch}
          searchPlaceholder="Search services and networking"
          status={toolbar.status()}
          onStatusChange={toolbar.setStatus}
          statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
          filters={scope.scopeFilters()}
          savedViewsKey="kubernetes-services"
          visible={visibleRows()}
          total={totalRows()}
          rowNoun="rows"
          hasActiveFilters={hasActiveFilters()}
          onResetFilters={resetFilters}
        />
        <Show when={scopedServices().length > 0}>
          <KubernetesServicesTable
            resources={scopedServices()}
            emptyIcon={k8sIcon()}
            emptyTitle="No services reported"
            emptyDescription="Services appear here once the agent can read Service resources."
            showToolbar={false}
            externalSearch={toolbar.search}
            externalStatus={toolbar.status}
          />
        </Show>
        <Show when={scopedNetworking().length > 0}>
          <KubernetesNetworkingTable
            resources={scopedNetworking()}
            emptyIcon={k8sIcon()}
            emptyTitle="No ingress or endpoint resources reported"
            emptyDescription="Ingresses and endpoint slices appear here once the agent can read networking inventory."
            showToolbar={false}
            externalSearch={toolbar.search}
            externalStatus={toolbar.status}
          />
        </Show>
      </div>
    </Show>
  );
}

function KubernetesConfiguration(props: { model: KubernetesPageModel }) {
  const hasConfigurationInventory = createMemo(
    () => props.model.config.length > 0 || props.model.policy.length > 0,
  );
  const toolbar = createKubernetesSharedToolbar();
  const sections = createMemo<Resource[][]>(() => [props.model.config, props.model.policy]);
  const totalRows = createMemo(() => sections().reduce((sum, rows) => sum + rows.length, 0));
  const scope = createKubernetesNamespaceScope(sections);
  const scopedConfig = () => scope.scopedSections()[0];
  const scopedPolicy = () => scope.scopedSections()[1];
  const visibleRows = createMemo(() =>
    countKubernetesVisible(scope.scopedSections(), toolbar.search(), toolbar.status()),
  );
  const hasActiveFilters = () => toolbar.hasActiveFilters() || scope.hasActiveNamespace();
  const resetFilters = () => {
    toolbar.resetFilters();
    scope.clearNamespace();
  };

  return (
    <Show
      when={hasConfigurationInventory()}
      fallback={
        <PlatformTableEmptyState
          icon={k8sIcon()}
          title="No configuration or policy resources reported"
          description="Namespaces, ConfigMaps, Secrets, ServiceAccounts, RBAC, policies, quotas, and limits appear here once the agent can read them."
        />
      }
    >
      <div class="space-y-4">
        <PlatformTableToolbar
          search={toolbar.search}
          onSearchChange={toolbar.setSearch}
          searchPlaceholder="Search configuration and policy"
          status={toolbar.status()}
          onStatusChange={toolbar.setStatus}
          statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
          filters={scope.scopeFilters()}
          savedViewsKey="kubernetes-config"
          visible={visibleRows()}
          total={totalRows()}
          rowNoun="rows"
          hasActiveFilters={hasActiveFilters()}
          onResetFilters={resetFilters}
        />
        <Show when={scopedConfig().length > 0}>
          <KubernetesConfigTable
            resources={scopedConfig()}
            emptyIcon={k8sIcon()}
            emptyTitle="No config resources reported"
            emptyDescription="Namespaces, ConfigMaps, Secrets, and ServiceAccounts appear here once the agent can read cluster configuration inventory."
            showToolbar={false}
            externalSearch={toolbar.search}
            externalStatus={toolbar.status}
          />
        </Show>
        <Show when={scopedPolicy().length > 0}>
          <KubernetesPolicyTable
            resources={scopedPolicy()}
            emptyIcon={k8sIcon()}
            emptyTitle="No policy resources reported"
            emptyDescription="NetworkPolicies, PodDisruptionBudgets, ResourceQuotas, and LimitRanges appear here once the agent can read policy inventory."
            showToolbar={false}
            externalSearch={toolbar.search}
            externalStatus={toolbar.status}
          />
        </Show>
      </div>
    </Show>
  );
}

function KubernetesOverview(props: KubernetesOverviewProps) {
  return (
    <div class="space-y-4">
      <KubernetesClustersTable
        clusters={props.model().clusters}
        scope={props.model().resources}
        emptyIcon={k8sIcon()}
        emptyTitle="No clusters reported"
        emptyDescription="Kubernetes clusters appear here once at least one agent reports cluster context."
        showToolbar={false}
      />
      <Show when={props.model().incidents.length > 0}>
        <KubernetesAlertsTable
          incidents={props.model().incidents}
          emptyIcon={k8sIcon()}
          emptyTitle="No active Kubernetes alerts"
          emptyDescription="Kubernetes health alerts appear here when the Pulse alert engine reports active workload, node, or cluster incidents."
        />
      </Show>
    </div>
  );
}

export default KubernetesPageSurface;
