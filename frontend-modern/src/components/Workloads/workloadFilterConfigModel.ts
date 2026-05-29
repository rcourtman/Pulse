import type { ViewMode } from '@/types/workloads';
import { getAllFilterOptionLabel } from '@/components/shared/filterOptionPresentation';
import { getSourcePlatformLabel, normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import { isContainerWorkloadViewMode } from '@/utils/workloads';
import type { WorkloadsStatusMode, WorkloadsToolbarFilterConfig } from './workloadsFilterModel';
import type { WorkloadNodeOption } from './workloadRouteModel';

export const WORKLOADS_KUBERNETES_CONTEXT_FILTER_LABEL = 'K8s cluster';
export const WORKLOADS_KUBERNETES_CONTEXT_ALL_OPTION_LABEL =
  getAllFilterOptionLabel('K8s clusters');
export const WORKLOADS_CONTAINER_RUNTIME_ALL_OPTION_LABEL = getAllFilterOptionLabel('runtimes');
export const WORKLOADS_PLATFORM_ALL_OPTION_LABEL = getAllFilterOptionLabel('platforms');
export const WORKLOADS_NODE_ALL_OPTION_LABEL = getAllFilterOptionLabel('nodes');
export const WORKLOADS_NAMESPACE_ALL_OPTION_LABEL = getAllFilterOptionLabel('namespaces');
export const WORKLOADS_CLUSTER_ALL_OPTION_LABEL = getAllFilterOptionLabel('clusters');
export const WORKLOAD_TYPE_OPTIONS: Array<{ value: ViewMode; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'vm', label: 'VMs' },
  { value: 'container', label: 'Containers' },
  { value: 'pod', label: 'Pods' },
];
export const WORKLOAD_STATUS_FILTER_OPTIONS: Array<{ value: WorkloadsStatusMode; label: string }> =
  [
    { value: 'all', label: 'All' },
    { value: 'running', label: 'Running' },
    { value: 'degraded', label: 'Degraded' },
    { value: 'stopped', label: 'Stopped' },
  ];

interface WorkloadsContainerRuntimeFilterConfigOptions {
  isWorkloadsRoute: boolean;
  allowEmbeddedScopeFilters?: boolean;
  viewMode: ViewMode;
  containerRuntime: string;
  runtimeOptions: string[];
  onChange: (value: string) => void;
}

export const buildWorkloadsContainerRuntimeFilterConfig = ({
  isWorkloadsRoute,
  allowEmbeddedScopeFilters,
  viewMode,
  containerRuntime,
  runtimeOptions,
  onChange,
}: WorkloadsContainerRuntimeFilterConfigOptions): WorkloadsToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute && !allowEmbeddedScopeFilters) return undefined;
  if (!isContainerWorkloadViewMode(viewMode)) return undefined;
  if (runtimeOptions.length < 2) return undefined;

  return {
    id: 'workloads-container-runtime-filter',
    label: 'Runtime',
    value: containerRuntime,
    options: [
      { value: '', label: WORKLOADS_CONTAINER_RUNTIME_ALL_OPTION_LABEL },
      ...runtimeOptions.map((value) => ({ value, label: value })),
    ],
    onChange,
  };
};

interface WorkloadsPlatformFilterConfigOptions {
  isWorkloadsRoute: boolean;
  allowEmbeddedScopeFilters?: boolean;
  selectedPlatform: string | null;
  platformOptions: WorkloadsToolbarFilterConfig['options'];
  onChange: (value: string) => void;
}

export const buildWorkloadsPlatformFilterOptions = (
  selectedPlatform: string | null,
  platformOptions: WorkloadsToolbarFilterConfig['options'],
): WorkloadsToolbarFilterConfig['options'] => {
  const allOption = { value: '', label: WORKLOADS_PLATFORM_ALL_OPTION_LABEL };
  const selectedValue = normalizeSourcePlatformQueryValue(selectedPlatform);
  if (!selectedValue || selectedValue === 'all') {
    return [allOption, ...platformOptions];
  }
  if (platformOptions.some((option) => option.value === selectedValue)) {
    return [allOption, ...platformOptions];
  }
  return [
    allOption,
    {
      value: selectedValue,
      label: getSourcePlatformLabel(selectedValue),
    },
    ...platformOptions,
  ];
};

export const buildWorkloadsPlatformFilterConfig = ({
  isWorkloadsRoute,
  allowEmbeddedScopeFilters,
  selectedPlatform,
  platformOptions,
  onChange,
}: WorkloadsPlatformFilterConfigOptions): WorkloadsToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute && !allowEmbeddedScopeFilters) return undefined;
  if (platformOptions.length === 0) return undefined;
  if (platformOptions.length === 1 && !(selectedPlatform || '').trim()) return undefined;

  return {
    id: 'workloads-platform-filter',
    label: 'Platform',
    value: selectedPlatform ?? '',
    options: buildWorkloadsPlatformFilterOptions(selectedPlatform, platformOptions),
    onChange,
  };
};

interface WorkloadsHostFilterConfigOptions {
  isWorkloadsRoute: boolean;
  allowEmbeddedScopeFilters?: boolean;
  viewMode: ViewMode;
  selectedKubernetesContext: string | null;
  kubernetesContextOptions: string[];
  selectedNode: string | null;
  workloadNodeOptions: WorkloadNodeOption[];
  onContextChange: (value: string) => void;
  onNodeChange: (value: string) => void;
}

export const buildWorkloadsHostFilterConfig = ({
  isWorkloadsRoute,
  allowEmbeddedScopeFilters,
  viewMode,
  selectedKubernetesContext,
  kubernetesContextOptions,
  selectedNode,
  workloadNodeOptions,
  onContextChange,
  onNodeChange,
}: WorkloadsHostFilterConfigOptions): WorkloadsToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute && !allowEmbeddedScopeFilters) return undefined;

  if (viewMode === 'pod') {
    return {
      id: 'workloads-k8s-context-filter',
      label: WORKLOADS_KUBERNETES_CONTEXT_FILTER_LABEL,
      value: selectedKubernetesContext ?? '',
      options: [
        { value: '', label: WORKLOADS_KUBERNETES_CONTEXT_ALL_OPTION_LABEL },
        ...kubernetesContextOptions.map((context) => ({ value: context, label: context })),
      ],
      onChange: onContextChange,
    };
  }

  return {
    id: 'workloads-node-filter',
    label: 'Node',
    value: selectedNode ?? '',
    options: [{ value: '', label: WORKLOADS_NODE_ALL_OPTION_LABEL }, ...workloadNodeOptions],
    onChange: onNodeChange,
  };
};

interface WorkloadsNamespaceFilterConfigOptions {
  isWorkloadsRoute: boolean;
  allowEmbeddedScopeFilters?: boolean;
  viewMode: ViewMode;
  selectedNamespace: string | null;
  namespaceOptions: string[];
  onChange: (value: string) => void;
}

export const buildWorkloadsNamespaceFilterConfig = ({
  isWorkloadsRoute,
  allowEmbeddedScopeFilters,
  viewMode,
  selectedNamespace,
  namespaceOptions,
  onChange,
}: WorkloadsNamespaceFilterConfigOptions): WorkloadsToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute && !allowEmbeddedScopeFilters) return undefined;
  if (viewMode !== 'pod') return undefined;
  if (namespaceOptions.length === 0) return undefined;

  return {
    id: 'workloads-k8s-namespace-filter',
    label: 'Namespace',
    value: selectedNamespace ?? '',
    options: [
      { value: '', label: WORKLOADS_NAMESPACE_ALL_OPTION_LABEL },
      ...namespaceOptions.map((value) => ({ value, label: value })),
    ],
    onChange,
  };
};

interface WorkloadsClusterFilterConfigOptions {
  isWorkloadsRoute: boolean;
  allowEmbeddedScopeFilters?: boolean;
  viewMode: ViewMode;
  selectedCluster: string | null;
  clusterOptions: string[];
  onChange: (value: string) => void;
}

export const buildWorkloadsClusterFilterConfig = ({
  isWorkloadsRoute,
  allowEmbeddedScopeFilters,
  viewMode,
  selectedCluster,
  clusterOptions,
  onChange,
}: WorkloadsClusterFilterConfigOptions): WorkloadsToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute && !allowEmbeddedScopeFilters) return undefined;
  if (viewMode !== 'vm') return undefined;
  if (clusterOptions.length === 0) return undefined;

  return {
    id: 'workloads-vmware-cluster-filter',
    label: 'Cluster',
    value: selectedCluster ?? '',
    options: [
      { value: '', label: WORKLOADS_CLUSTER_ALL_OPTION_LABEL },
      ...clusterOptions.map((value) => ({ value, label: value })),
    ],
    onChange,
  };
};
