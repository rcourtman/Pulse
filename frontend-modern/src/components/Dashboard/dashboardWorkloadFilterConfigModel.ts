import type { ViewMode } from '@/types/workloads';
import { getAllFilterOptionLabel } from '@/components/shared/filterOptionPresentation';
import { getSourcePlatformLabel, normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import type { DashboardToolbarFilterConfig } from './dashboardFilterModel';
import type { DashboardWorkloadNodeOption } from './dashboardWorkloadRouteModel';

export const DASHBOARD_KUBERNETES_CONTEXT_FILTER_LABEL = 'K8s cluster';
export const DASHBOARD_KUBERNETES_CONTEXT_ALL_OPTION_LABEL =
  getAllFilterOptionLabel('K8s clusters');
export const DASHBOARD_CONTAINER_RUNTIME_ALL_OPTION_LABEL =
  getAllFilterOptionLabel('runtimes');
export const DASHBOARD_PLATFORM_ALL_OPTION_LABEL = getAllFilterOptionLabel('platforms');
export const DASHBOARD_NODE_ALL_OPTION_LABEL = getAllFilterOptionLabel('nodes');
export const DASHBOARD_NAMESPACE_ALL_OPTION_LABEL = getAllFilterOptionLabel('namespaces');
export const DASHBOARD_WORKLOAD_TYPE_OPTIONS: Array<{ value: ViewMode; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'vm', label: 'VMs' },
  { value: 'system-container', label: 'System containers' },
  { value: 'app-container', label: 'App containers' },
  { value: 'pod', label: 'Pods' },
];

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
      { value: '', label: DASHBOARD_CONTAINER_RUNTIME_ALL_OPTION_LABEL },
      ...runtimeOptions.map((value) => ({ value, label: value })),
    ],
    onChange,
  };
};

interface DashboardPlatformFilterConfigOptions {
  isWorkloadsRoute: boolean;
  selectedPlatform: string | null;
  platformOptions: DashboardToolbarFilterConfig['options'];
  onChange: (value: string) => void;
}

export const buildDashboardPlatformFilterOptions = (
  selectedPlatform: string | null,
  platformOptions: DashboardToolbarFilterConfig['options'],
): DashboardToolbarFilterConfig['options'] => {
  const allOption = { value: '', label: DASHBOARD_PLATFORM_ALL_OPTION_LABEL };
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

export const buildDashboardPlatformFilterConfig = ({
  isWorkloadsRoute,
  selectedPlatform,
  platformOptions,
  onChange,
}: DashboardPlatformFilterConfigOptions): DashboardToolbarFilterConfig | undefined => {
  if (!isWorkloadsRoute) return undefined;
  if (platformOptions.length === 0) return undefined;
  if (platformOptions.length === 1 && !(selectedPlatform || '').trim()) return undefined;

  return {
    id: 'workloads-platform-filter',
    label: 'Platform',
    value: selectedPlatform ?? '',
    options: buildDashboardPlatformFilterOptions(selectedPlatform, platformOptions),
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
      label: DASHBOARD_KUBERNETES_CONTEXT_FILTER_LABEL,
      value: selectedKubernetesContext ?? '',
      options: [
        { value: '', label: DASHBOARD_KUBERNETES_CONTEXT_ALL_OPTION_LABEL },
        ...kubernetesContextOptions.map((context) => ({ value: context, label: context })),
      ],
      onChange: onContextChange,
    };
  }

  return {
    id: 'workloads-node-filter',
    label: 'Node',
    value: selectedNode ?? '',
    options: [{ value: '', label: DASHBOARD_NODE_ALL_OPTION_LABEL }, ...workloadNodeOptions],
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
      { value: '', label: DASHBOARD_NAMESPACE_ALL_OPTION_LABEL },
      ...namespaceOptions.map((value) => ({ value, label: value })),
    ],
    onChange,
  };
};
