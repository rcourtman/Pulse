export const deserializeWorkloadsContainerRuntime = (raw: unknown): string =>
  typeof raw === 'string' ? raw : '';

export const WORKLOADS_WORKLOAD_ROUTE_RESET_STATE = {
  selectedNode: null,
  selectedHostHint: null,
  selectedPlatform: null,
  selectedKubernetesContext: null,
  selectedKubernetesNamespace: null,
  selectedCluster: null,
  containerRuntime: '',
  viewMode: 'all',
} as const;

interface WorkloadsWorkloadNodeSelectionOptions {
  nodeId: string | null;
  nodeType: 'pve' | 'pbs' | 'pmg' | null;
  showFilters: boolean;
}

export interface WorkloadsWorkloadNodeSelectionResult {
  selectedNode: string | null;
  selectedHostHint: null;
  shouldApply: boolean;
  shouldShowFilters: boolean;
}

export const resolveWorkloadsWorkloadNodeSelection = ({
  nodeId,
  nodeType,
  showFilters,
}: WorkloadsWorkloadNodeSelectionOptions): WorkloadsWorkloadNodeSelectionResult => {
  const shouldApply = nodeType === 'pve' || nodeType === null;
  return {
    selectedNode: nodeId,
    selectedHostHint: null,
    shouldApply,
    shouldShowFilters: Boolean(nodeId) && !showFilters,
  };
};
