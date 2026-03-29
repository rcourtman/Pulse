export const deserializeDashboardContainerRuntime = (raw: unknown): string =>
  typeof raw === 'string' ? raw : '';

export const DASHBOARD_WORKLOAD_ROUTE_RESET_STATE = {
  selectedNode: null,
  selectedHostHint: null,
  selectedPlatform: null,
  selectedKubernetesContext: null,
  selectedKubernetesNamespace: null,
  containerRuntime: '',
  viewMode: 'all',
} as const;

interface DashboardWorkloadNodeSelectionOptions {
  nodeId: string | null;
  nodeType: 'pve' | 'pbs' | 'pmg' | null;
  showFilters: boolean;
}

export interface DashboardWorkloadNodeSelectionResult {
  selectedNode: string | null;
  selectedHostHint: null;
  shouldApply: boolean;
  shouldShowFilters: boolean;
}

export const resolveDashboardWorkloadNodeSelection = ({
  nodeId,
  nodeType,
  showFilters,
}: DashboardWorkloadNodeSelectionOptions): DashboardWorkloadNodeSelectionResult => {
  const shouldApply = nodeType === 'pve' || nodeType === null;
  return {
    selectedNode: nodeId,
    selectedHostHint: null,
    shouldApply,
    shouldShowFilters: Boolean(nodeId) && !showFilters,
  };
};
