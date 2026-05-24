import { useLocation } from '@solidjs/router';
import ShipWheelIcon from 'lucide-solid/icons/ship-wheel';
import { Show, createMemo, type Accessor } from 'solid-js';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  PlatformErrorState,
  PlatformSectionTabs,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
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
    const segment = location.pathname.split('/').filter(Boolean)[1];
    if (segment === 'workloads') return 'pods';
    return segment && VALID_TABS.has(segment as KubernetesPageTabId)
      ? (segment as KubernetesPageTabId)
      : 'overview';
  });
  const model = createMemo(() => buildKubernetesPageModel(resources()));
  const controllerResources = createMemo(() => getKubernetesControllerResources(model()));

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
              <KubernetesOverview model={model} controllers={controllerResources} />
            </Show>
            <Show when={activeTab() === 'nodes'}>
              <KubernetesNodesTable
                resources={model().nodes}
                emptyIcon={k8sIcon()}
                emptyTitle="No nodes reported"
                emptyDescription="Kubernetes nodes appear here as soon as the agent enumerates Node resources."
              />
            </Show>
            <Show when={activeTab() === 'pods'}>
              <KubernetesPodsTable
                resources={model().pods}
                emptyIcon={k8sIcon()}
                emptyTitle="No pods reported"
                emptyDescription="Pods appear here once the agent can read Pod resources."
              />
            </Show>
            <Show when={activeTab() === 'deployments'}>
              <KubernetesDeploymentsTable
                resources={model().deployments}
                emptyIcon={k8sIcon()}
                emptyTitle="No deployments reported"
                emptyDescription="Deployments appear here once the agent can read Deployment resources."
              />
            </Show>
            <Show when={activeTab() === 'controllers'}>
              <KubernetesControllersTable
                resources={controllerResources()}
                emptyIcon={k8sIcon()}
                emptyTitle="No workload controllers reported"
                emptyDescription="StatefulSets, DaemonSets, Jobs, and CronJobs appear here when the agent reports them."
              />
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
              <KubernetesPolicyTable
                resources={model().policy}
                emptyIcon={k8sIcon()}
                emptyTitle="No policy resources reported"
                emptyDescription="NetworkPolicies, PodDisruptionBudgets, ResourceQuotas, and LimitRanges appear here once the agent can read policy inventory."
              />
            </Show>
            <Show when={activeTab() === 'autoscaling'}>
              <KubernetesAutoscalingTable
                resources={model().autoscaling}
                emptyIcon={k8sIcon()}
                emptyTitle="No autoscaling resources reported"
                emptyDescription="HorizontalPodAutoscalers appear here once the agent can read autoscaling inventory."
              />
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
  controllers: Accessor<Resource[]>;
}

const getKubernetesControllerResources = (model: KubernetesPageModel): Resource[] => [
  ...model.statefulSets,
  ...model.daemonSets,
  ...model.jobs,
  ...model.cronJobs,
];

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
      <KubernetesNodesTable
        resources={props.model().nodes}
        emptyIcon={k8sIcon()}
        emptyTitle="No nodes reported"
        emptyDescription="Kubernetes nodes appear here as soon as the agent enumerates them."
        showToolbar={false}
      />
      <Show when={props.model().pods.length > 0}>
        <KubernetesPodsTable
          resources={props.model().pods}
          emptyIcon={k8sIcon()}
          emptyTitle="No pods reported"
          emptyDescription="Pods appear here once the agent can read Pod resources."
          showToolbar={false}
        />
      </Show>
      <Show when={props.model().deployments.length > 0}>
        <KubernetesDeploymentsTable
          resources={props.model().deployments}
          emptyIcon={k8sIcon()}
          emptyTitle="No deployments reported"
          emptyDescription="Deployments appear here once the cluster reports them."
          showToolbar={false}
        />
      </Show>
      <Show when={props.controllers().length > 0}>
        <KubernetesControllersTable
          resources={props.controllers()}
          emptyIcon={k8sIcon()}
          emptyTitle="No controllers reported"
          emptyDescription="StatefulSets, DaemonSets, Jobs, and CronJobs appear here when the agent reports them."
          showToolbar={false}
        />
      </Show>
    </div>
  );
}

export default KubernetesPageSurface;
