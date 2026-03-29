import { createSignal, type Accessor, type Setter } from 'solid-js';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { deserializeDashboardWorkloadViewMode } from './dashboardWorkloadRouteModel';
import {
  DASHBOARD_WORKLOAD_ROUTE_RESET_STATE,
  deserializeDashboardContainerRuntime,
  resolveDashboardWorkloadNodeSelection,
} from './dashboardWorkloadRouteStateModel';
import { useDashboardWorkloadFilterOptions } from './useDashboardWorkloadFilterOptions';
import { useDashboardWorkloadUrlSync } from './useDashboardWorkloadUrlSync';

export interface DashboardWorkloadRouteStateOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  showFilters: Accessor<boolean>;
  setShowFilters: Setter<boolean>;
}

export function useDashboardWorkloadRouteState(options: DashboardWorkloadRouteStateOptions) {
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [selectedPlatform, setSelectedPlatform] = createSignal<string | null>(null);
  const [selectedKubernetesContext, setSelectedKubernetesContext] = createSignal<string | null>(
    null,
  );
  const [selectedKubernetesNamespace, setSelectedKubernetesNamespace] = createSignal<
    string | null
  >(null);
  const [selectedHostHint, setSelectedHostHint] = createSignal<string | null>(null);

  const [viewMode, setViewMode] = usePersistentSignal<ViewMode>('dashboardViewMode', 'all', {
    deserialize: deserializeDashboardWorkloadViewMode,
  });

  const [containerRuntime, setContainerRuntime] = usePersistentSignal<string>(
    'dashboardContainerRuntime',
    '',
    {
      deserialize: deserializeDashboardContainerRuntime,
      serialize: (value) => value,
    },
  );

  const handleNodeSelect = (nodeId: string | null, nodeType: 'pve' | 'pbs' | 'pmg' | null) => {
    const selection = resolveDashboardWorkloadNodeSelection({
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
    setSelectedNode(DASHBOARD_WORKLOAD_ROUTE_RESET_STATE.selectedNode);
    setSelectedHostHint(DASHBOARD_WORKLOAD_ROUTE_RESET_STATE.selectedHostHint);
    setSelectedPlatform(DASHBOARD_WORKLOAD_ROUTE_RESET_STATE.selectedPlatform);
    setSelectedKubernetesContext(DASHBOARD_WORKLOAD_ROUTE_RESET_STATE.selectedKubernetesContext);
    setSelectedKubernetesNamespace(
      DASHBOARD_WORKLOAD_ROUTE_RESET_STATE.selectedKubernetesNamespace,
    );
    setContainerRuntime(DASHBOARD_WORKLOAD_ROUTE_RESET_STATE.containerRuntime);
    setViewMode(DASHBOARD_WORKLOAD_ROUTE_RESET_STATE.viewMode);
  };

  const { isWorkloadsRoute } = useDashboardWorkloadUrlSync({
    containerRuntime,
    containerRuntimeOptions: () => filterOptions.containerRuntimeOptions(),
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

  const filterOptions = useDashboardWorkloadFilterOptions({
    allGuests: options.allGuests,
    isWorkloadsRoute,
    viewMode,
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
