import { useLocation } from '@solidjs/router';
import ShipWheelIcon from 'lucide-solid/icons/ship-wheel';
import { Show, createMemo } from 'solid-js';
import { WorkloadsSurface } from '@/components/Workloads/WorkloadsSurface';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformResourceTable,
  PlatformSectionTabs,
  PlatformTableEmptyState,
} from '@/features/platformPage/sharedPlatformPage';
import {
  KUBERNETES_TAB_SPECS,
  buildKubernetesPageModel,
  type KubernetesPageTabId,
} from './kubernetesPageModel';

const KUBERNETES_RESOURCE_QUERY =
  'type=k8s-cluster,k8s-node,pod,k8s-deployment,k8s-service';
const KUBERNETES_PLATFORM_FILTER = 'kubernetes';
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
            <Show when={activeTab() === 'overview'}>
              <PlatformResourceTable
                resources={model().clusters}
                emptyIcon={k8sIcon()}
                emptyTitle="No clusters reported"
                emptyDescription="Kubernetes clusters appear here once at least one agent reports cluster context."
              />
            </Show>
            <Show when={activeTab() === 'nodes'}>
              <PlatformResourceTable
                resources={model().nodes}
                emptyIcon={k8sIcon()}
                emptyTitle="No nodes reported"
                emptyDescription="Kubernetes nodes appear here as soon as the agent enumerates them."
              />
            </Show>
            <Show when={activeTab() === 'pods'}>
              <WorkloadsSurface
                vms={[]}
                containers={[]}
                nodes={[]}
                useWorkloads
                embedded
                tableOnly
                forcedPlatform={KUBERNETES_PLATFORM_FILTER}
              />
            </Show>
            <Show when={activeTab() === 'deployments'}>
              <PlatformResourceTable
                resources={model().deployments}
                emptyIcon={k8sIcon()}
                emptyTitle="No deployments reported"
                emptyDescription="Deployments appear here once the cluster reports them."
              />
            </Show>
            <Show when={activeTab() === 'services'}>
              <PlatformResourceTable
                resources={model().services}
                emptyIcon={k8sIcon()}
                emptyTitle="No services reported"
                emptyDescription="Kubernetes services appear here once the cluster reports them."
              />
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default KubernetesPageSurface;
