import { createMemo, type Accessor } from 'solid-js';

import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import { workloadMatchesPlatformScope } from '@/utils/workloads';
import type { WorkloadsToolbarFilterConfig } from './workloadsFilterModel';
import {
  buildWorkloadsContainerRuntimeOptions,
  buildWorkloadsKubernetesContextOptions,
  buildWorkloadsPlatformOptions,
  buildWorkloadsKubernetesNamespaceOptions,
  buildWorkloadNodeOptions,
} from './workloadRouteModel';
import {
  buildWorkloadsContainerRuntimeFilterConfig,
  buildWorkloadsHostFilterConfig,
  buildWorkloadsPlatformFilterConfig,
  buildWorkloadsNamespaceFilterConfig,
} from './workloadFilterConfigModel';

interface WorkloadsWorkloadFilterOptionsOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  isWorkloadsRoute: Accessor<boolean>;
  allowEmbeddedScopeFilters: Accessor<boolean>;
  viewMode: Accessor<ViewMode>;
  platformScope?: Accessor<string | null | undefined>;
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

export function useWorkloadFilterOptions(options: WorkloadsWorkloadFilterOptionsOptions) {
  const platformScopedGuests = createMemo(() => {
    const normalizedScope = normalizeSourcePlatformQueryValue(options.platformScope?.() || '');
    if (!normalizedScope || normalizedScope === 'all') {
      return options.allGuests();
    }
    return options
      .allGuests()
      .filter((guest) => workloadMatchesPlatformScope(guest, normalizedScope));
  });

  const workloadNodeOptions = createMemo(() => buildWorkloadNodeOptions(platformScopedGuests()));

  const kubernetesContextOptions = createMemo(() =>
    buildWorkloadsKubernetesContextOptions(platformScopedGuests()),
  );

  const kubernetesNamespaceOptions = createMemo(() =>
    buildWorkloadsKubernetesNamespaceOptions(
      platformScopedGuests(),
      options.selectedKubernetesContext(),
    ),
  );

  const containerRuntimeOptions = createMemo(() =>
    buildWorkloadsContainerRuntimeOptions(platformScopedGuests()),
  );

  const platformOptions = createMemo(() =>
    buildWorkloadsPlatformOptions(options.allGuests(), options.viewMode()),
  );

  const containerRuntimeFilterConfig = createMemo<WorkloadsToolbarFilterConfig | undefined>(() =>
    buildWorkloadsContainerRuntimeFilterConfig({
      isWorkloadsRoute: options.isWorkloadsRoute(),
      allowEmbeddedScopeFilters: options.allowEmbeddedScopeFilters(),
      viewMode: options.viewMode(),
      containerRuntime: options.containerRuntime(),
      runtimeOptions: containerRuntimeOptions(),
      onChange: (value) => options.setContainerRuntime(value),
    }),
  );

  const platformFilterConfig = createMemo<WorkloadsToolbarFilterConfig | undefined>(() =>
    buildWorkloadsPlatformFilterConfig({
      isWorkloadsRoute: options.isWorkloadsRoute(),
      allowEmbeddedScopeFilters: options.allowEmbeddedScopeFilters(),
      selectedPlatform: options.selectedPlatform(),
      platformOptions: platformOptions(),
      onChange: (value) => options.setSelectedPlatform(value || null),
    }),
  );

  const hostFilterConfig = createMemo<WorkloadsToolbarFilterConfig | undefined>(() =>
    buildWorkloadsHostFilterConfig({
      isWorkloadsRoute: options.isWorkloadsRoute(),
      allowEmbeddedScopeFilters: options.allowEmbeddedScopeFilters(),
      viewMode: options.viewMode(),
      selectedKubernetesContext: options.selectedKubernetesContext(),
      kubernetesContextOptions: kubernetesContextOptions(),
      selectedNode: options.selectedNode(),
      workloadNodeOptions: workloadNodeOptions(),
      onContextChange: (value) => options.setSelectedKubernetesContext(value || null),
      onNodeChange: (value) => options.handleNodeSelect(value || null, value ? 'pve' : null),
    }),
  );

  const namespaceFilterConfig = createMemo<WorkloadsToolbarFilterConfig | undefined>(() =>
    buildWorkloadsNamespaceFilterConfig({
      isWorkloadsRoute: options.isWorkloadsRoute(),
      allowEmbeddedScopeFilters: options.allowEmbeddedScopeFilters(),
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
