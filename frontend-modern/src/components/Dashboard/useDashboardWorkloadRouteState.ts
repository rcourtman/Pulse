import { createEffect, createSignal, type Accessor, type Setter } from 'solid-js';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { deserializeDashboardWorkloadViewMode } from './dashboardWorkloadRouteModel';
import { useDashboardWorkloadFilterOptions } from './useDashboardWorkloadFilterOptions';
import { useDashboardWorkloadUrlSync } from './useDashboardWorkloadUrlSync';

export interface DashboardWorkloadRouteStateOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  showFilters: Accessor<boolean>;
  setShowFilters: Setter<boolean>;
}

export function useDashboardWorkloadRouteState(options: DashboardWorkloadRouteStateOptions) {
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
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
      deserialize: (raw) => (typeof raw === 'string' ? raw : ''),
      serialize: (value) => value,
    },
  );
  const [workloadsRouteActive, setWorkloadsRouteActive] = createSignal(false);

  const handleNodeSelect = (nodeId: string | null, nodeType: 'pve' | 'pbs' | 'pmg' | null) => {
    if (nodeType === 'pve' || nodeType === null) {
      setSelectedHostHint(null);
      setSelectedNode(nodeId);
      if (nodeId && !options.showFilters()) {
        options.setShowFilters(true);
      }
    }
  };

  const resetWorkloadRouteFilters = () => {
    setSelectedNode(null);
    setSelectedHostHint(null);
    setSelectedKubernetesContext(null);
    setSelectedKubernetesNamespace(null);
    setContainerRuntime('');
    setViewMode('all');
  };

  const filterOptions = useDashboardWorkloadFilterOptions({
    allGuests: options.allGuests,
    isWorkloadsRoute: workloadsRouteActive,
    viewMode,
    containerRuntime,
    selectedNode,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    setContainerRuntime,
    setSelectedKubernetesContext,
    handleNodeSelect,
    setSelectedKubernetesNamespace,
  });

  const { isWorkloadsRoute } = useDashboardWorkloadUrlSync({
    containerRuntime,
    containerRuntimeOptions: filterOptions.containerRuntimeOptions,
    kubernetesNamespaceOptions: filterOptions.kubernetesNamespaceOptions,
    selectedHostHint,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setSelectedHostHint,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setSelectedNode,
    setShowFilters: options.setShowFilters,
    setViewMode,
    showFilters: options.showFilters,
    viewMode,
    workloadNodeOptions: filterOptions.workloadNodeOptions,
  });
  createEffect(() => {
    setWorkloadsRouteActive(isWorkloadsRoute());
  });

  const {
    containerRuntimeFilterConfig,
    containerRuntimeOptions,
    hostFilterConfig,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    namespaceFilterConfig,
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
    resetWorkloadRouteFilters,
    selectedHostHint,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setSelectedNode,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setViewMode,
    viewMode,
    workloadNodeOptions,
  } as const;
}
