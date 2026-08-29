import { useLocation, useSearchParams } from '@solidjs/router';
import { Show, createMemo, type Accessor } from 'solid-js';
import { ButtonLink } from '@/components/shared/Button';
import { buildInfrastructureAgentUpdatesPath } from '@/components/Settings/infrastructureWorkspaceModel';
import type { FilterDef } from '@/components/shared/FilterBar';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import { PlatformOutdatedAgentNotice } from '@/features/platformPage/PlatformOutdatedAgentNotice';
import {
  collectOutdatedAgentHosts,
  formatAgentVersionDisplay,
} from '@/features/platformPage/agentVersion';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { buildKubernetesPath, KUBERNETES_QUERY_PARAMS } from '@/routing/resourceLinks';
import { updateStore } from '@/stores/updates';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
  PlatformTableToolbar,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import { buildPlatformResourceSearchSuggestions } from '@/features/platformPage/platformSearchSuggestions';
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
  buildKubernetesOverviewPosture,
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
  'type=k8s-cluster,k8s-node,pod,k8s-deployment,k8s-replicaset,k8s-namespace,k8s-service,k8s-statefulset,k8s-daemonset,k8s-job,k8s-cronjob,k8s-ingress,k8s-endpoint-slice,k8s-network-policy,k8s-persistent-volume,k8s-persistent-volume-claim,k8s-storage-class,k8s-configmap,k8s-secret,k8s-serviceaccount,k8s-role,k8s-cluster-role,k8s-role-binding,k8s-cluster-role-binding,k8s-resource-quota,k8s-limit-range,k8s-pod-disruption-budget,k8s-horizontal-pod-autoscaler,k8s-event,agent&source=kubernetes';

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
  const agentUpdateTargetVersion = createMemo(
    () => updateStore.versionInfo()?.agentUpdateTargetVersion,
  );
  const outdatedAgentHosts = createMemo(() =>
    collectOutdatedAgentHosts(model().nodes, agentUpdateTargetVersion()),
  );
  const outdatedAgentUpdatePath = createMemo(() =>
    buildInfrastructureAgentUpdatesPath(outdatedAgentHosts().map((host) => host.agentId)),
  );
  const serverVersionDisplay = createMemo(() =>
    formatAgentVersionDisplay(agentUpdateTargetVersion()),
  );

  return (
    <div data-testid="kubernetes-page" class="pulse-wide-data-surface space-y-3">
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
              subjectSingular="node"
              subjectPlural="nodes"
            />
            <Show when={activeTab() === 'overview'}>
              <div class="space-y-4">
                <KubernetesOverview model={model} />
              </div>
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
  resetFilters: (extraParams?: Record<string, string | null>) => void;
};

// Search and status live in the URL, like the namespace scope below, so the
// whole filter state is shareable and bookmarkable. That is what makes -term
// exclusions portable: search `-name`, bookmark the resulting URL, and the
// noisy rows stay hidden every time you open it. URL writes replace the
// history entry so typing does not pile up back-button states.
function createKubernetesSharedToolbar(): SharedToolbarState {
  const [searchParams, setSearchParams] = useSearchParams();
  const search = () => {
    const value = searchParams[KUBERNETES_QUERY_PARAMS.query];
    return typeof value === 'string' ? value : '';
  };
  const setSearch = (value: string) =>
    setSearchParams({ [KUBERNETES_QUERY_PARAMS.query]: value || null }, { replace: true });
  const status = (): KubernetesResourceStatusFilter => {
    const value = searchParams[KUBERNETES_QUERY_PARAMS.status];
    return value === 'online' || value === 'degraded' || value === 'offline' ? value : 'all';
  };
  const setStatus = (value: KubernetesResourceStatusFilter) =>
    setSearchParams(
      { [KUBERNETES_QUERY_PARAMS.status]: value === 'all' ? null : value },
      { replace: true },
    );
  const hasActiveFilters = createMemo(() => search().trim().length > 0 || status() !== 'all');
  // Reset must clear every URL param in ONE navigation. Consecutive
  // setSearchParams calls each merge against the pre-navigation URL (the
  // router commits inside an async transition), so a second call would
  // resurrect the params the first one just cleared. Callers pass the params
  // they own (e.g. namespace) instead of issuing their own follow-up write.
  const resetFilters = (extraParams?: Record<string, string | null>) =>
    setSearchParams(
      {
        [KUBERNETES_QUERY_PARAMS.query]: null,
        [KUBERNETES_QUERY_PARAMS.status]: null,
        ...extraParams,
      },
      { replace: true },
    );
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

type KubernetesInventoryScope = {
  scopedSections: Accessor<Resource[][]>;
  scopeFilters: Accessor<FilterDef[]>;
  hasActiveScope: Accessor<boolean>;
};

// Cluster and namespace scope for a shared-toolbar Kubernetes tab. The shared
// toolbar drives several stacked tables at once, so both facets pre-filter every
// section rather than living on any one table. URL-backed scope is shareable and
// bookmarkable, and lets the Overview cluster table drive the workload
// inventory beneath it.
function createKubernetesInventoryScope(
  sections: Accessor<Resource[][]>,
): KubernetesInventoryScope {
  const [searchParams, setSearchParams] = useSearchParams();
  const clusterFilter = () => {
    const value = searchParams[KUBERNETES_QUERY_PARAMS.cluster];
    return typeof value === 'string' ? value : '';
  };
  const namespaceFilter = () => {
    const value = searchParams[KUBERNETES_QUERY_PARAMS.namespace];
    return typeof value === 'string' ? value : '';
  };
  const clusterOf = (resource: Resource) =>
    (resource.kubernetes?.clusterId ?? resource.kubernetes?.clusterName ?? '').trim();
  const namespaceOf = (resource: Resource) => (resource.kubernetes?.namespace ?? '').trim();
  const matchesCluster = (resource: Resource) => {
    const cluster = clusterFilter();
    return !cluster || clusterOf(resource) === cluster;
  };
  const matchesNamespace = (resource: Resource) => {
    const namespace = namespaceFilter();
    return !namespace || namespaceOf(resource) === namespace;
  };
  const clusterOptions = createMemo(() => {
    const clusters = new Map<string, string>();
    for (const section of sections()) {
      for (const resource of section) {
        const value = clusterOf(resource);
        if (!value) continue;
        const label = (resource.kubernetes?.clusterName ?? value).trim() || value;
        if (!clusters.has(value)) clusters.set(value, label);
      }
    }
    return [...clusters.entries()].sort((left, right) => left[1].localeCompare(right[1]));
  });
  const namespaceOptions = createMemo(() => {
    const seen = new Set<string>();
    for (const section of sections()) {
      for (const resource of section) {
        if (!matchesCluster(resource)) continue;
        const namespace = namespaceOf(resource);
        if (namespace) seen.add(namespace);
      }
    }
    return [...seen].sort((a, b) => a.localeCompare(b));
  });
  const scopedSections = createMemo<Resource[][]>(() =>
    sections().map((rows) =>
      rows.filter((resource) => matchesCluster(resource) && matchesNamespace(resource)),
    ),
  );
  const scopeFilters = createMemo<FilterDef[]>(() => {
    const filters: FilterDef[] = [];
    if (clusterOptions().length > 1) {
      filters.push({
        id: 'cluster',
        label: 'Cluster',
        group: 'scope',
        options: () => [
          { value: '', label: 'All clusters' },
          ...clusterOptions().map(([value, label]) => ({ value, label })),
        ],
        value: clusterFilter,
        setValue: (value: string) =>
          setSearchParams({
            [KUBERNETES_QUERY_PARAMS.cluster]: value || null,
            [KUBERNETES_QUERY_PARAMS.namespace]: null,
          }),
        defaultValue: '',
      });
    }
    if (namespaceOptions().length > 1) {
      filters.push({
        id: 'namespace',
        label: 'Namespace',
        group: 'scope',
        options: () => [
          { value: '', label: 'All namespaces' },
          ...namespaceOptions().map((namespace) => ({ value: namespace, label: namespace })),
        ],
        value: namespaceFilter,
        setValue: (value: string) =>
          setSearchParams({ [KUBERNETES_QUERY_PARAMS.namespace]: value || null }),
        defaultValue: '',
      });
    }
    return filters;
  });
  return {
    scopedSections,
    scopeFilters,
    hasActiveScope: () => clusterFilter() !== '' || namespaceFilter() !== '',
  };
}

function KubernetesWorkloads(props: {
  model: KubernetesPageModel;
  controllers: Resource[];
  attentionCount?: number;
}) {
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
  const searchSuggestions = createMemo(() =>
    buildPlatformResourceSearchSuggestions(sections().flat()),
  );
  const totalRows = createMemo(() => sections().reduce((sum, rows) => sum + rows.length, 0));

  // Cluster and namespace scope apply across every workload section at once
  // (see the shared helper). Section order defines the scoped accessors below.
  const scope = createKubernetesInventoryScope(sections);
  const scopedDeployments = () => scope.scopedSections()[0];
  const scopedPods = () => scope.scopedSections()[1];
  const scopedControllers = () => scope.scopedSections()[2];
  const scopedAutoscaling = () => scope.scopedSections()[3];
  const scopeFilters = scope.scopeFilters;
  const countForStatus = (value: KubernetesResourceStatusFilter): number =>
    countKubernetesVisible(scope.scopedSections(), toolbar.search(), value);
  const visibleRows = createMemo(() => countForStatus(toolbar.status()));
  const hasActiveFilters = () => toolbar.hasActiveFilters() || scope.hasActiveScope();
  const resetFilters = () =>
    toolbar.resetFilters({
      [KUBERNETES_QUERY_PARAMS.cluster]: null,
      [KUBERNETES_QUERY_PARAMS.namespace]: null,
    });

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
          searchSuggestions={searchSuggestions}
          status={toolbar.status()}
          onStatusChange={toolbar.setStatus}
          statusOptions={withPlatformStatusCounts(PLATFORM_HEALTH_FILTER_OPTIONS, countForStatus)}
          filters={scopeFilters()}
          leadingControls={
            <Show when={(props.attentionCount ?? 0) > 0}>
              <div
                class="inline-flex min-w-0 items-center gap-1.5 text-xs font-medium text-amber-700 dark:text-amber-300"
                role="status"
              >
                <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-amber-500" aria-hidden="true" />
                <span class="whitespace-nowrap">
                  {props.attentionCount} workload{props.attentionCount === 1 ? '' : 's'}{' '}
                  {props.attentionCount === 1 ? 'needs' : 'need'} attention
                </span>
                <ButtonLink href={buildKubernetesPath('workloads')} variant="warning" size="xs">
                  Review
                </ButtonLink>
              </div>
            </Show>
          }
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
  const searchSuggestions = createMemo(() =>
    buildPlatformResourceSearchSuggestions(sections().flat()),
  );
  const totalRows = createMemo(() => sections().reduce((sum, rows) => sum + rows.length, 0));
  const scope = createKubernetesInventoryScope(sections);
  const scopedServices = () => scope.scopedSections()[0];
  const scopedNetworking = () => scope.scopedSections()[1];
  const countForStatus = (value: KubernetesResourceStatusFilter): number =>
    countKubernetesVisible(scope.scopedSections(), toolbar.search(), value);
  const visibleRows = createMemo(() => countForStatus(toolbar.status()));
  const hasActiveFilters = () => toolbar.hasActiveFilters() || scope.hasActiveScope();
  const resetFilters = () =>
    toolbar.resetFilters({
      [KUBERNETES_QUERY_PARAMS.cluster]: null,
      [KUBERNETES_QUERY_PARAMS.namespace]: null,
    });

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
          searchSuggestions={searchSuggestions}
          status={toolbar.status()}
          onStatusChange={toolbar.setStatus}
          statusOptions={withPlatformStatusCounts(PLATFORM_HEALTH_FILTER_OPTIONS, countForStatus)}
          filters={scope.scopeFilters()}
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
  const searchSuggestions = createMemo(() =>
    buildPlatformResourceSearchSuggestions(sections().flat()),
  );
  const totalRows = createMemo(() => sections().reduce((sum, rows) => sum + rows.length, 0));
  const scope = createKubernetesInventoryScope(sections);
  const scopedConfig = () => scope.scopedSections()[0];
  const scopedPolicy = () => scope.scopedSections()[1];
  const countForStatus = (value: KubernetesResourceStatusFilter): number =>
    countKubernetesVisible(scope.scopedSections(), toolbar.search(), value);
  const visibleRows = createMemo(() => countForStatus(toolbar.status()));
  const hasActiveFilters = () => toolbar.hasActiveFilters() || scope.hasActiveScope();
  const resetFilters = () =>
    toolbar.resetFilters({
      [KUBERNETES_QUERY_PARAMS.cluster]: null,
      [KUBERNETES_QUERY_PARAMS.namespace]: null,
    });

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
          searchSuggestions={searchSuggestions}
          status={toolbar.status()}
          onStatusChange={toolbar.setStatus}
          statusOptions={withPlatformStatusCounts(PLATFORM_HEALTH_FILTER_OPTIONS, countForStatus)}
          filters={scope.scopeFilters()}
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
  const [searchParams, setSearchParams] = useSearchParams();
  const workloadAttention = createMemo(() => {
    const posture = buildKubernetesOverviewPosture(props.model());
    return posture.podAttention + posture.deploymentAttention;
  });

  return (
    <div class="space-y-4">
      <KubernetesClustersTable
        clusters={props.model().clusters}
        scope={props.model().resources}
        emptyIcon={k8sIcon()}
        emptyTitle="No clusters reported"
        emptyDescription="Kubernetes clusters appear here once at least one agent reports cluster context."
        showToolbar={false}
        selectedClusterId={
          typeof searchParams[KUBERNETES_QUERY_PARAMS.cluster] === 'string'
            ? searchParams[KUBERNETES_QUERY_PARAMS.cluster]
            : ''
        }
        onSelectCluster={(cluster) => {
          const clusterId =
            cluster.kubernetes?.clusterId?.trim() || cluster.kubernetes?.clusterName?.trim() || '';
          setSearchParams({
            [KUBERNETES_QUERY_PARAMS.cluster]:
              searchParams[KUBERNETES_QUERY_PARAMS.cluster] === clusterId ? null : clusterId,
            [KUBERNETES_QUERY_PARAMS.namespace]: null,
          });
        }}
      />
      <KubernetesWorkloads
        model={props.model()}
        controllers={getKubernetesControllerResources(props.model())}
        attentionCount={workloadAttention()}
      />
      <Show when={props.model().incidents.length > 0}>
        <div id="kubernetes-health-signals">
          <KubernetesAlertsTable
            incidents={props.model().incidents}
            emptyIcon={k8sIcon()}
            emptyTitle="No active Kubernetes alerts"
            emptyDescription="Kubernetes health alerts appear here when the Pulse alert engine reports active workload, node, or cluster incidents."
          />
        </div>
      </Show>
    </div>
  );
}

export default KubernetesPageSurface;
