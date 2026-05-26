import { createSignal, type Accessor, type Setter } from 'solid-js';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { deserializeWorkloadViewMode } from './workloadRouteModel';
import {
  WORKLOADS_WORKLOAD_ROUTE_RESET_STATE,
  deserializeWorkloadsContainerRuntime,
  resolveWorkloadsWorkloadNodeSelection,
} from './workloadRouteStateModel';
import { useWorkloadFilterOptions } from './useWorkloadFilterOptions';
import { useWorkloadUrlSync } from './useWorkloadUrlSync';

export interface WorkloadRouteStateOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  forcedPlatform?: string;
  forcedViewMode?: ViewMode;
  showFilters: Accessor<boolean>;
  setShowFilters: Setter<boolean>;
}

export function useWorkloadRouteState(options: WorkloadRouteStateOptions) {
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [selectedPlatform, setSelectedPlatform] = createSignal<string | null>(null);
  const [selectedKubernetesContext, setSelectedKubernetesContext] = createSignal<string | null>(
    null,
  );
  const [selectedKubernetesNamespace, setSelectedKubernetesNamespace] = createSignal<string | null>(
    null,
  );
  const [selectedHostHint, setSelectedHostHint] = createSignal<string | null>(null);

  const [viewMode, setViewMode] = usePersistentSignal<ViewMode>('workloadsViewMode', 'all', {
    deserialize: deserializeWorkloadViewMode,
  });

  const [containerRuntime, setContainerRuntime] = usePersistentSignal<string>(
    'workloadsContainerRuntime',
    '',
    {
      deserialize: deserializeWorkloadsContainerRuntime,
      serialize: (value) => value,
    },
  );
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
    setContainerRuntime(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.containerRuntime);
    setViewMode(WORKLOADS_WORKLOAD_ROUTE_RESET_STATE.viewMode);
  };

  const { isWorkloadsRoute } = useWorkloadUrlSync({
    containerRuntime,
    containerRuntimeOptions: () => filterOptions.containerRuntimeOptions(),
    routeStateEnabled: () => true,
    kubernetesNamespaceOptions: () => filterOptions.kubernetesNamespaceOptions(),
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
    setContainerRuntime,
    setSelectedPlatform,
    setSelectedKubernetesContext,
    handleNodeSelect,
    setSelectedKubernetesNamespace,
  });

  const {
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
    selectedHostHint,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    selectedPlatform,
    setContainerRuntime,
    setSelectedNode,
    setSelectedPlatform,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setViewMode,
    viewMode,
    workloadNodeOptions,
  } as const;
}
