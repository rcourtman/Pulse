import { useLocation } from '@solidjs/router';
import ShipWheelIcon from 'lucide-solid/icons/ship-wheel';
import { Show, createMemo, type Accessor } from 'solid-js';
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
import { KubernetesClustersTable } from './KubernetesClustersTable';
import { KubernetesConfigTable } from './KubernetesConfigTable';
import { KubernetesDeploymentsTable } from './KubernetesDeploymentsTable';
import { KubernetesInventoryTable } from './KubernetesInventoryTable';
import { KubernetesNetworkingTable } from './KubernetesNetworkingTable';
import { KubernetesNodesTable } from './KubernetesNodesTable';
import { KubernetesServicesTable } from './KubernetesServicesTable';
import { KubernetesStorageTable } from './KubernetesStorageTable';
import {
  KUBERNETES_TAB_SPECS,
  buildKubernetesPageModel,
  type KubernetesPageModel,
  type KubernetesPageTabId,
} from './kubernetesPageModel';

// Include `agent` rows so K8s nodes that the backend registry merged onto
// the linked agent host (sources=['agent','kubernetes']) still appear in the
// Nodes section of the Overview stack; the page model filters them down to
// those tagged kubernetes.
const KUBERNETES_RESOURCE_QUERY =
  'type=k8s-cluster,k8s-node,pod,k8s-deployment,k8s-replicaset,k8s-namespace,k8s-service,k8s-statefulset,k8s-daemonset,k8s-job,k8s-cronjob,k8s-ingress,k8s-endpoint-slice,k8s-network-policy,k8s-persistent-volume,k8s-persistent-volume-claim,k8s-storage-class,k8s-configmap,k8s-secret,k8s-serviceaccount,k8s-resource-quota,k8s-limit-range,k8s-pod-disruption-budget,k8s-horizontal-pod-autoscaler,k8s-event,agent';
const KUBERNETES_PLATFORM_FILTER = 'kubernetes';
const KUBERNETES_WORKLOAD_FORCED_VIEW_MODE = 'pod';
const KUBERNETES_WORKLOAD_COLUMN_SCOPE = 'kubernetes-pods';
const KUBERNETES_POD_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'degraded', label: 'Needs attention' },
  { value: 'stopped', label: 'Not running' },
];
const VALID_TABS = new Set<KubernetesPageTabId>(KUBERNETES_TAB_SPECS.map((tab) => tab.id));

const k8sIcon = () => <ShipWheelIcon class="h-6 w-6 text-slate-400" />;

export function KubernetesPageSurface() {
  const location = useLocation();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: KUBERNETES_RESOURCE_QUERY,
    cacheKey: 'kubernetes-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const activeTab = createMemo<KubernetesPageTabId>(() => {
    const segment = location.pathname.split('/').filter(Boolean)[1] as
      | KubernetesPageTabId
      | undefined;
    return segment && VALID_TABS.has(segment) ? segment : 'overview';
  });
  const model = createMemo(() => buildKubernetesPageModel(resources()));

  return (
    <div data-testid="kubernetes-page" class="space-y-3">
      <PlatformSectionTabs
        tabs={KUBERNETES_TAB_SPECS}
        active={activeTab()}
        ariaLabel="Kubernetes sections"
      />

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
              <KubernetesWorkloads model={model} />
            </Show>
            <Show when={activeTab() === 'services'}>
              <KubernetesServicesTable
                resources={model().services}
                emptyIcon={k8sIcon()}
                emptyTitle="No services reported"
                emptyDescription="Services appear here once the agent can read Service resources."
              />
            </Show>
            <Show when={activeTab() === 'storage'}>
              <KubernetesStorageTable
                resources={model().storage}
                emptyIcon={k8sIcon()}
                emptyTitle="No Kubernetes volume resources reported"
                emptyDescription="StorageClasses, persistent volumes, and claims appear here once the agent can read storage inventory."
              />
            </Show>
            <Show when={activeTab() === 'networking'}>
              <KubernetesNetworkingTable
                resources={model().networking}
                emptyIcon={k8sIcon()}
                emptyTitle="No networking resources reported"
                emptyDescription="Services, ingresses, and endpoint slices appear here once the agent can read networking inventory."
              />
            </Show>
            <Show when={activeTab() === 'config'}>
              <KubernetesConfigTable
                resources={model().config}
                emptyIcon={k8sIcon()}
                emptyTitle="No config resources reported"
                emptyDescription="Namespaces, ConfigMaps, Secrets, and ServiceAccounts appear here once the agent can read cluster configuration inventory."
              />
            </Show>
            <Show when={activeTab() === 'policy'}>
              <KubernetesInventoryTable
                resources={model().policy}
                variant="policy"
                emptyIcon={k8sIcon()}
                emptyTitle="No policy resources reported"
                emptyDescription="NetworkPolicies, PodDisruptionBudgets, ResourceQuotas, and LimitRanges appear here once the agent can read policy inventory."
              />
            </Show>
            <Show when={activeTab() === 'autoscaling'}>
              <KubernetesInventoryTable
                resources={model().autoscaling}
                variant="autoscaling"
                emptyIcon={k8sIcon()}
                emptyTitle="No autoscaling resources reported"
                emptyDescription="HorizontalPodAutoscalers appear here once the agent can read autoscaling inventory."
              />
            </Show>
            <Show when={activeTab() === 'events'}>
              <KubernetesInventoryTable
                resources={model().events}
                variant="events"
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

function KubernetesOverview(props: KubernetesOverviewProps) {
  return <KubernetesWorkloadStack model={props.model} mode="overview" />;
}

function KubernetesWorkloads(props: KubernetesOverviewProps) {
  return <KubernetesWorkloadStack model={props.model} mode="workloads" />;
}

function KubernetesWorkloadStack(
  props: KubernetesOverviewProps & { mode: 'overview' | 'workloads' },
) {
  const workloadsState = useWorkloadsState({
    vms: [],
    containers: [],
    nodes: [],
    useWorkloads: true,
    embedded: true,
    tableOnly: true,
    forcedPlatform: KUBERNETES_PLATFORM_FILTER,
    forcedViewMode: KUBERNETES_WORKLOAD_FORCED_VIEW_MODE,
    showFilterToolbar: true,
    suppressPlatformFilter: true,
    allowEmbeddedScopeFilters: true,
    columnVisibilityStorageScope: KUBERNETES_WORKLOAD_COLUMN_SCOPE,
    // Backup column is driven exclusively by Proxmox vzdump / PBS data
    // (`resource.proxmox.lastBackup` in useWorkloads); Kubernetes
    // workloads have no equivalent source at this integration layer, so
    // the column would always render blank. Hide by default.
    additionalDefaultHiddenColumnIds: ['backup'],
    compactGroupHeaders: true,
  });
  const showSharedFilterToolbar = createMemo(
    () =>
      workloadsState.surfaceConnected() &&
      workloadsState.surfaceInitialDataReceived() &&
      workloadsState.allGuests().length > 0,
  );
  const searchTerm = createMemo(() => workloadsState.search().trim());
  const filteredClusters = createMemo(() => {
    if (!searchTerm()) return props.model().clusters;
    return props.model().clusters.filter((cluster) => resourceMatchesSearch(cluster, searchTerm()));
  });
  const filteredNodes = createMemo(() => {
    if (!searchTerm()) return props.model().nodes;
    return props.model().nodes.filter((node) => resourceMatchesSearch(node, searchTerm()));
  });
  const filteredDeployments = createMemo(() => {
    if (!searchTerm()) return props.model().deployments;
    return props
      .model()
      .deployments.filter((deployment) => resourceMatchesSearch(deployment, searchTerm()));
  });
  const filteredControllers = createMemo(() => {
    const controllers = [
      ...props.model().statefulSets,
      ...props.model().daemonSets,
      ...props.model().jobs,
      ...props.model().cronJobs,
    ];
    if (!searchTerm()) return controllers;
    return controllers.filter((controller) => resourceMatchesSearch(controller, searchTerm()));
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
            ariaLabel="Kubernetes pod filters"
            searchPlaceholder="Search pods by name, namespace, image, cluster, or node"
            searchEmptyMessage="Recent Kubernetes pod searches appear here."
            statusOptions={KUBERNETES_POD_STATUS_OPTIONS}
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
            forcedPlatform={KUBERNETES_PLATFORM_FILTER}
            pinnedSelectionActive={() =>
              Boolean(
                workloadsState.selectedGuestId() || workloadsState.focusedSummaryWorkloadGroupId(),
              )
            }
            onClearPinnedSelection={workloadsState.clearPinnedSummaryScope}
          />
        </div>
      </Show>
      <Show when={props.mode === 'overview'}>
        <KubernetesClustersTable
          clusters={filteredClusters()}
          scope={props.model().resources}
          emptyIcon={k8sIcon()}
          emptyTitle="No clusters reported"
          emptyDescription="Kubernetes clusters appear here once at least one agent reports cluster context."
          showToolbar={false}
        />
        <KubernetesNodesTable
          resources={filteredNodes()}
          emptyIcon={k8sIcon()}
          emptyTitle="No nodes reported"
          emptyDescription="Kubernetes nodes appear here as soon as the agent enumerates them."
          showToolbar={false}
        />
      </Show>
      <Show when={props.model().deployments.length > 0}>
        <KubernetesDeploymentsTable
          resources={filteredDeployments()}
          emptyIcon={k8sIcon()}
          emptyTitle="No deployments reported"
          emptyDescription="Deployments appear here once the cluster reports them."
          showToolbar={false}
        />
      </Show>
      <Show when={filteredControllers().length > 0}>
        <KubernetesInventoryTable
          resources={filteredControllers()}
          variant="controllers"
          emptyIcon={k8sIcon()}
          emptyTitle="No controllers reported"
          emptyDescription="StatefulSets, DaemonSets, Jobs, and CronJobs appear here when the agent reports them."
          showToolbar={false}
        />
      </Show>
      <WorkloadsSurface
        state={workloadsState}
        vms={[]}
        containers={[]}
        nodes={[]}
        useWorkloads
        embedded
        tableOnly
        forcedPlatform={KUBERNETES_PLATFORM_FILTER}
        forcedViewMode={KUBERNETES_WORKLOAD_FORCED_VIEW_MODE}
        compactGroupHeaders
      />
    </div>
  );
}

export default KubernetesPageSurface;
