import ShipWheelIcon from 'lucide-solid/icons/ship-wheel';
import { Show, createMemo } from 'solid-js';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
} from '@/features/platformPage/sharedPlatformPage';
import { KubernetesClustersTable } from './KubernetesClustersTable';
import { KubernetesDeploymentsTable } from './KubernetesDeploymentsTable';
import { KubernetesNodesTable } from './KubernetesNodesTable';
import { KUBERNETES_TAB_SPECS, buildKubernetesPageModel } from './kubernetesPageModel';

// Include `agent` rows so K8s nodes that the backend registry merged onto
// the linked agent host (sources=['agent','kubernetes']) still appear in the
// Nodes section of the Overview stack; the page model filters them down to
// those tagged kubernetes.
const KUBERNETES_RESOURCE_QUERY = 'type=k8s-cluster,k8s-node,pod,k8s-deployment,agent';
const KUBERNETES_PLATFORM_FILTER = 'kubernetes';
const KUBERNETES_WORKLOAD_FORCED_VIEW_MODE = 'pod';
const KUBERNETES_WORKLOAD_COLUMN_SCOPE = 'kubernetes-pods';

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
            <div class="space-y-4">
              <KubernetesClustersTable
                clusters={model().clusters}
                scope={model().resources}
                emptyIcon={k8sIcon()}
                emptyTitle="No clusters reported"
                emptyDescription="Kubernetes clusters appear here once at least one agent reports cluster context."
                showToolbar={false}
              />
              <KubernetesNodesTable
                resources={model().nodes}
                emptyIcon={k8sIcon()}
                emptyTitle="No nodes reported"
                emptyDescription="Kubernetes nodes appear here as soon as the agent enumerates them."
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
                forcedPlatform={KUBERNETES_PLATFORM_FILTER}
                forcedViewMode={KUBERNETES_WORKLOAD_FORCED_VIEW_MODE}
                columnVisibilityStorageScope={KUBERNETES_WORKLOAD_COLUMN_SCOPE}
                compactGroupHeaders
              />
              <Show when={model().deployments.length > 0}>
                <KubernetesDeploymentsTable
                  resources={model().deployments}
                  emptyIcon={k8sIcon()}
                  emptyTitle="No deployments reported"
                  emptyDescription="Deployments appear here once the cluster reports them."
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

export default KubernetesPageSurface;
