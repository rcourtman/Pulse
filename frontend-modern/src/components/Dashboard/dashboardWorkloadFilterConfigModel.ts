import type { ViewMode } from '@/types/workloads';
import type { DashboardToolbarFilterConfig } from './dashboardFilterModel';
import type { DashboardWorkloadNodeOption } from './dashboardWorkloadRouteModel';

interface DashboardContainerRuntimeFilterConfigOptions {
  isWorkloadsRoute: boolean;
  viewMode: ViewMode;
  containerRuntime: string;
  runtimeOptions: string[];
  onChange: (value: string) => void;
}

export const buildDashboardContainerRuntimeFilterConfig = ({
  isWorkloadsRoute,
  viewMode,
  containerRuntime,
  runtimeOptions,
  onChange,
}: DashboardContainerRuntimeFilterConfigOptions): DashboardToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute) return undefined;
  if (viewMode !== 'app-container') return undefined;
  if (runtimeOptions.length === 0) return undefined;

  return {
    id: 'workloads-container-runtime-filter',
    label: 'Runtime',
    value: containerRuntime,
    options: [
      { value: '', label: 'All runtimes' },
      ...runtimeOptions.map((value) => ({ value, label: value })),
    ],
    onChange,
  };
};

interface DashboardHostFilterConfigOptions {
  isWorkloadsRoute: boolean;
  viewMode: ViewMode;
  selectedKubernetesContext: string | null;
  kubernetesContextOptions: string[];
  selectedNode: string | null;
  workloadNodeOptions: DashboardWorkloadNodeOption[];
  onContextChange: (value: string) => void;
  onNodeChange: (value: string) => void;
}

export const buildDashboardHostFilterConfig = ({
  isWorkloadsRoute,
  viewMode,
  selectedKubernetesContext,
  kubernetesContextOptions,
  selectedNode,
  workloadNodeOptions,
  onContextChange,
  onNodeChange,
}: DashboardHostFilterConfigOptions): DashboardToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute) return undefined;

  if (viewMode === 'pod') {
    return {
      id: 'workloads-k8s-context-filter',
      label: 'Cluster',
      value: selectedKubernetesContext ?? '',
      options: [
        { value: '', label: 'All clusters' },
        ...kubernetesContextOptions.map((context) => ({ value: context, label: context })),
      ],
      onChange: onContextChange,
    };
  }

  return {
    id: 'workloads-node-filter',
    label: 'Node',
    value: selectedNode ?? '',
    options: [{ value: '', label: 'All nodes' }, ...workloadNodeOptions],
    onChange: onNodeChange,
  };
};

interface DashboardNamespaceFilterConfigOptions {
  isWorkloadsRoute: boolean;
  viewMode: ViewMode;
  selectedNamespace: string | null;
  namespaceOptions: string[];
  onChange: (value: string) => void;
}

export const buildDashboardNamespaceFilterConfig = ({
  isWorkloadsRoute,
  viewMode,
  selectedNamespace,
  namespaceOptions,
  onChange,
}: DashboardNamespaceFilterConfigOptions): DashboardToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute) return undefined;
  if (viewMode !== 'pod') return undefined;
  if (namespaceOptions.length === 0) return undefined;

  return {
    id: 'workloads-k8s-namespace-filter',
    label: 'Namespace',
    value: selectedNamespace ?? '',
    options: [
      { value: '', label: 'All namespaces' },
      ...namespaceOptions.map((value) => ({ value, label: value })),
    ],
    onChange,
  };
};
