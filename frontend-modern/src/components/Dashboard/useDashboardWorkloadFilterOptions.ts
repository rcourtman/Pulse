import { createMemo, type Accessor } from 'solid-js';

import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import type { DashboardToolbarFilterConfig } from './dashboardFilterModel';
import {
  buildDashboardContainerRuntimeOptions,
  buildDashboardKubernetesContextOptions,
  buildDashboardPlatformOptions,
  buildDashboardKubernetesNamespaceOptions,
  buildDashboardWorkloadNodeOptions,
} from './dashboardWorkloadRouteModel';
import {
  buildDashboardContainerRuntimeFilterConfig,
  buildDashboardHostFilterConfig,
  buildDashboardPlatformFilterConfig,
  buildDashboardNamespaceFilterConfig,
} from './dashboardWorkloadFilterConfigModel';

interface DashboardWorkloadFilterOptionsOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  isWorkloadsRoute: Accessor<boolean>;
  viewMode: Accessor<ViewMode>;
  containerRuntime: Accessor<string>;
  selectedPlatform: Accessor<string | null>;
  selectedNode: Accessor<string | null>;
  selectedKubernetesContext: Accessor<string | null>;
  selectedKubernetesNamespace: Accessor<string | null>;
  setContainerRuntime: (value: string) => void;
  setSelectedPlatform: (value: string | null) => void;
  setSelectedKubernetesContext: (value: string | null) => void;
  handleNodeSelect: (nodeId: string | null, nodeType: 'pve' | 'pbs' | 'pmg' | null) => void;
  setSelectedKubernetesNamespace: (value: string | null) => void;
}

export function useDashboardWorkloadFilterOptions(
  options: DashboardWorkloadFilterOptionsOptions,
) {
  const workloadNodeOptions = createMemo(() =>
    buildDashboardWorkloadNodeOptions(options.allGuests()),
  );

  const kubernetesContextOptions = createMemo(() =>
    buildDashboardKubernetesContextOptions(options.allGuests()),
  );

  const kubernetesNamespaceOptions = createMemo(() =>
    buildDashboardKubernetesNamespaceOptions(
      options.allGuests(),
      options.selectedKubernetesContext(),
    ),
  );

  const containerRuntimeOptions = createMemo(() =>
    buildDashboardContainerRuntimeOptions(options.allGuests()),
  );

  const platformOptions = createMemo(() =>
    buildDashboardPlatformOptions(options.allGuests(), options.viewMode()),
  );

  const containerRuntimeFilterConfig = createMemo<DashboardToolbarFilterConfig | undefined>(() =>
    buildDashboardContainerRuntimeFilterConfig({
      isWorkloadsRoute: options.isWorkloadsRoute(),
      viewMode: options.viewMode(),
      containerRuntime: options.containerRuntime(),
      runtimeOptions: containerRuntimeOptions(),
      onChange: (value) => options.setContainerRuntime(value),
    }),
  );

  const platformFilterConfig = createMemo<DashboardToolbarFilterConfig | undefined>(() =>
    buildDashboardPlatformFilterConfig({
      isWorkloadsRoute: options.isWorkloadsRoute(),
      selectedPlatform: options.selectedPlatform(),
      platformOptions: platformOptions(),
      onChange: (value) => options.setSelectedPlatform(value || null),
    }),
  );

  const hostFilterConfig = createMemo<DashboardToolbarFilterConfig | undefined>(() =>
    buildDashboardHostFilterConfig({
      isWorkloadsRoute: options.isWorkloadsRoute(),
      viewMode: options.viewMode(),
      selectedKubernetesContext: options.selectedKubernetesContext(),
      kubernetesContextOptions: kubernetesContextOptions(),
      selectedNode: options.selectedNode(),
      workloadNodeOptions: workloadNodeOptions(),
      onContextChange: (value) => options.setSelectedKubernetesContext(value || null),
      onNodeChange: (value) => options.handleNodeSelect(value || null, value ? 'pve' : null),
    }),
  );

  const namespaceFilterConfig = createMemo<DashboardToolbarFilterConfig | undefined>(() =>
    buildDashboardNamespaceFilterConfig({
      isWorkloadsRoute: options.isWorkloadsRoute(),
      viewMode: options.viewMode(),
      selectedNamespace: options.selectedKubernetesNamespace(),
      namespaceOptions: kubernetesNamespaceOptions(),
      onChange: (value) => options.setSelectedKubernetesNamespace(value || null),
    }),
  );

  return {
    containerRuntimeFilterConfig,
    containerRuntimeOptions,
    hostFilterConfig,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    namespaceFilterConfig,
    platformFilterConfig,
    platformOptions,
    workloadNodeOptions,
  } as const;
}
