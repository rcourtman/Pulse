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
} from '@/features/platformPage/sharedPlatformPage';
import { KubernetesClustersTable } from './KubernetesClustersTable';
import { KubernetesDeploymentsTable } from './KubernetesDeploymentsTable';
import { KubernetesNodesTable } from './KubernetesNodesTable';
import {
  KUBERNETES_TAB_SPECS,
  buildKubernetesPageModel,
  type KubernetesPageModel,
} from './kubernetesPageModel';

// Include `agent` rows so K8s nodes that the backend registry merged onto
// the linked agent host (sources=['agent','kubernetes']) still appear in the
// Nodes section of the Overview stack; the page model filters them down to
// those tagged kubernetes.
const KUBERNETES_RESOURCE_QUERY = 'type=k8s-cluster,k8s-node,pod,k8s-deployment,agent';
const KUBERNETES_PLATFORM_FILTER = 'kubernetes';
const KUBERNETES_WORKLOAD_FORCED_VIEW_MODE = 'pod';
const KUBERNETES_WORKLOAD_COLUMN_SCOPE = 'kubernetes-pods';
const KUBERNETES_POD_STATUS_OPTIONS: readonly WorkloadsStatusOption[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'degraded', label: 'Needs attention' },
  { value: 'stopped', label: 'Not running' },
];

const k8sIcon = () => <ShipWheelIcon class="h-6 w-6 text-slate-400" />;

export function KubernetesPageSurface() {
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: KUBERNETES_RESOURCE_QUERY,
    cacheKey: 'kubernetes-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });
  const model = createMemo(() => buildKubernetesPageModel(resources()));

  return (
    <div data-testid="kubernetes-page" class="space-y-3">
      <PlatformSectionTabs
        tabs={KUBERNETES_TAB_SPECS}
        active="overview"
        ariaLabel="Kubernetes sections"
      />

      <Show
        when={!loading() || model().resources.length > 0}
        fallback={
          <PlatformTableEmptyState
            icon={k8sIcon()}
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
            <KubernetesOverview model={model} />
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
    return props.model().deployments.filter((deployment) =>
      resourceMatchesSearch(deployment, searchTerm()),
    );
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
            hostFilter={workloadsState.hostFilterConfig()}
            namespaceFilter={workloadsState.namespaceFilterConfig()}
            platformFilter={undefined}
            suppressTypeFilter
            metricDisplayMode={workloadsState.workloadMetricDisplayMode}
            setMetricDisplayMode={workloadsState.setWorkloadMetricDisplayMode}
            metricHistoryRange={workloadsState.workloadMetricHistoryRange}
            setMetricHistoryRange={workloadsState.setWorkloadMetricHistoryRange}
            forcedPlatform={KUBERNETES_PLATFORM_FILTER}
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
      <Show when={props.model().deployments.length > 0}>
        <KubernetesDeploymentsTable
          resources={filteredDeployments()}
          emptyIcon={k8sIcon()}
          emptyTitle="No deployments reported"
          emptyDescription="Deployments appear here once the cluster reports them."
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
