import { createSignal, onMount, type Accessor, type Setter } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { deserializeWorkloadViewMode } from './workloadRouteModel';
import {
  WORKLOADS_WORKLOAD_ROUTE_RESET_STATE,
  deserializeWorkloadsContainerRuntime,
  resolveWorkloadsWorkloadNodeSelection,
} from './workloadRouteStateModel';
import { useWorkloadFilterOptions } from './useWorkloadFilterOptions';
import { useWorkloadUrlSync } from './useWorkloadUrlSync';
import { WORKLOADS_QUERY_PARAMS } from '@/routing/resourceLinks';

export interface WorkloadRouteStateOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  forcedPlatform?: string;
  forcedViewMode?: ViewMode;
  showFilters: Accessor<boolean>;
  setShowFilters: Setter<boolean>;
}

export function useWorkloadRouteState(options: WorkloadRouteStateOptions) {
  const location = useLocation();
  const navigate = useNavigate();
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [selectedPlatform, setSelectedPlatform] = createSignal<string | null>(null);
  const [selectedKubernetesContext, setSelectedKubernetesContext] = createSignal<string | null>(
    null,
  );
  const [selectedKubernetesNamespace, setSelectedKubernetesNamespace] = createSignal<string | null>(
    null,
  );
  const [selectedCluster, setSelectedCluster] = createSignal<string | null>(null);
  const [selectedHostHint, setSelectedHostHint] = createSignal<string | null>(null);
  const [viewMode, setViewMode] = createSignal<ViewMode>('all');
  const [containerRuntime, setContainerRuntime] = createSignal<string>('');

  onMount(() => {
    if (typeof window === 'undefined') return;
    const params = new URLSearchParams(location.search);
    let mutated = false;

    if (!params.has(WORKLOADS_QUERY_PARAMS.type)) {
      const legacyView = deserializeWorkloadViewMode(
        window.localStorage.getItem('workloadsViewMode'),
      );
      if (legacyView !== 'all') {
        params.set(WORKLOADS_QUERY_PARAMS.type, legacyView);
        mutated = true;
      }
    }

    if (!params.has(WORKLOADS_QUERY_PARAMS.runtime)) {
      const legacyRuntime = deserializeWorkloadsContainerRuntime(
        window.localStorage.getItem('workloadsContainerRuntime'),
      );
      if (legacyRuntime !== '') {
        params.set(WORKLOADS_QUERY_PARAMS.runtime, legacyRuntime);
        mutated = true;
      }
    }

    if (mutated) {
      navigate(`${location.pathname}?${params.toString()}`, { replace: true });
    }
  });
  const filterViewMode = () => options.forcedViewMode ?? viewMode();
  const filterPlatformScope = () => options.forcedPlatform?.trim() || selectedPlatform();

  const handleNodeSelect = (nodeId: string | null, nodeType: 'pve' | 'pbs' | 'pmg' | null) => {
    const selection = resolveWorkloadsWorkloadNodeSelection({
      nodeId,
      nodeType,
      showFilters: options.showFilters(),
    });
    if (!selection.shouldApply) return;
    setSelectedHostHint(selection.selectedHostHint);
    setSelectedNode(selection.selectedNode);
    if (selection.shouldShowFilters) {
      options.setShowFilters(true);
    }
  };

  const resetWorkloadRouteFilters = () => {
    setSelectedNode(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.selectedNode);
    setSelectedHostHint(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.selectedHostHint);
    setSelectedPlatform(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.selectedPlatform);
    setSelectedKubernetesContext(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.selectedKubernetesContext);
    setSelectedKubernetesNamespace(
      WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.selectedKubernetesNamespace,
    );
    setSelectedCluster(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.selectedCluster);
    setContainerRuntime(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.containerRuntime);
    setViewMode(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.viewMode);
  };

  const { isWorkloadsRoute } = useWorkloadUrlSync({
    containerRuntime,
    containerRuntimeOptions: () => filterOptions.containerRuntimeOptions(),
    routeStateEnabled: () => true,
    kubernetesNamespaceOptions: () => filterOptions.kubernetesNamespaceOptions(),
    selectedCluster,
    setSelectedCluster,
    clusterOptions: () => filterOptions.clusterOptions(),
    selectedHostHint,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    selectedPlatform,
    setContainerRuntime,
    setSelectedHostHint,
    setSelectedPlatform,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setSelectedNode,
    setShowFilters: options.setShowFilters,
    setViewMode,
    showFilters: options.showFilters,
    viewMode,
    effectiveViewMode: filterViewMode,
    workloadNodeOptions: () => filterOptions.workloadNodeOptions(),
  });

  const filterOptions = useWorkloadFilterOptions({
    allGuests: options.allGuests,
    isWorkloadsRoute,
    allowEmbeddedScopeFilters: () => true,
    viewMode: filterViewMode,
    platformScope: filterPlatformScope,
    containerRuntime,
    selectedPlatform,
    selectedNode,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedCluster,
    setContainerRuntime,
    setSelectedPlatform,
    setSelectedKubernetesContext,
    handleNodeSelect,
    setSelectedKubernetesNamespace,
    setSelectedCluster,
  });

  const {
    clusterFilterConfig,
    clusterOptions,
    containerRuntimeFilterConfig,
    containerRuntimeOptions,
    hostFilterConfig,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    namespaceFilterConfig,
    platformFilterConfig,
    platformOptions,
    workloadNodeOptions,
  } = filterOptions;

  return {
    clusterFilterConfig,
    clusterOptions,
    containerRuntime,
    containerRuntimeFilterConfig,
    containerRuntimeOptions,
    handleNodeSelect,
    hostFilterConfig,
    isWorkloadsRoute,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    namespaceFilterConfig,
    platformFilterConfig,
    platformOptions,
    resetWorkloadRouteFilters,
    selectedCluster,
    selectedHostHint,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    selectedPlatform,
    setContainerRuntime,
    setSelectedCluster,
    setSelectedNode,
    setSelectedPlatform,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setViewMode,
    viewMode,
    workloadNodeOptions,
  } as const;
}
