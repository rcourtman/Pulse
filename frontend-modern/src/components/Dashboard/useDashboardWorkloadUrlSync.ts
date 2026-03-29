import {
  createMemo,
  createEffect,
  createSignal,
  onCleanup,
  untrack,
  type Accessor,
  type Setter,
} from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import type { ViewMode } from '@/types/workloads';
import {
  WORKLOADS_PATH,
} from '@/routing/resourceLinks';
import type { DashboardWorkloadNodeOption } from './dashboardWorkloadRouteModel';
import {
  parseDashboardWorkloadUrlParams,
  resolveDashboardManagedWorkloadsNavigateTarget,
  resolveDashboardWorkloadRuntimeParam,
  resolveDashboardWorkloadTypeParam,
} from './dashboardWorkloadUrlSyncModel';

export interface DashboardWorkloadUrlSyncOptions {
  containerRuntime: Accessor<string>;
  containerRuntimeOptions: Accessor<string[]>;
  kubernetesNamespaceOptions: Accessor<string[]>;
  selectedHostHint: Accessor<string | null>;
  selectedPlatform: Accessor<string | null>;
  selectedKubernetesContext: Accessor<string | null>;
  selectedKubernetesNamespace: Accessor<string | null>;
  selectedNode: Accessor<string | null>;
  setContainerRuntime: Setter<string>;
  setSelectedHostHint: Setter<string | null>;
  setSelectedPlatform: Setter<string | null>;
  setSelectedKubernetesContext: Setter<string | null>;
  setSelectedKubernetesNamespace: Setter<string | null>;
  setSelectedNode: Setter<string | null>;
  setShowFilters: Setter<boolean>;
  setViewMode: Setter<ViewMode>;
  showFilters: Accessor<boolean>;
  viewMode: Accessor<ViewMode>;
  workloadNodeOptions: Accessor<DashboardWorkloadNodeOption[]>;
}

export function useDashboardWorkloadUrlSync(options: DashboardWorkloadUrlSyncOptions) {
  const navigate = useNavigate();
  const location = useLocation();
  const isWorkloadsRoute = () => location.pathname === WORKLOADS_PATH;
  const workloadUrlParams = createMemo(() => parseDashboardWorkloadUrlParams(location.search));

  const [handledTypeParam, setHandledTypeParam] = createSignal('');
  const [handledRuntimeParam, setHandledRuntimeParam] = createSignal('');
  const [handledContextParam, setHandledContextParam] = createSignal('');
  const [handledNamespaceParam, setHandledNamespaceParam] = createSignal('');
  const [handledAgentParam, setHandledAgentParam] = createSignal('');
  const [handledPlatformParam, setHandledPlatformParam] = createSignal('');

  let pendingUrlSyncHandle: number | null = null;
  let pendingUrlSyncPath: string | null = null;
  const scheduleUrlSyncNavigate = (nextPath: string) => {
    pendingUrlSyncPath = nextPath;
    if (pendingUrlSyncHandle !== null) return;
    pendingUrlSyncHandle = window.setTimeout(() => {
      pendingUrlSyncHandle = null;
      const target = pendingUrlSyncPath;
      pendingUrlSyncPath = null;
      if (!target) return;
      const current = `${untrack(() => location.pathname)}${untrack(() => location.search)}`;
      if (current === target) return;
      navigate(target, { replace: true });
    }, 0);
  };
  onCleanup(() => {
    if (pendingUrlSyncHandle !== null) {
      window.clearTimeout(pendingUrlSyncHandle);
      pendingUrlSyncHandle = null;
      pendingUrlSyncPath = null;
    }
  });

  createEffect(() => {
    if (options.viewMode() === 'pod') return;
    const hostHint = options.selectedHostHint();
    if (!hostHint || options.selectedNode() !== null) return;
    const normalizedHint = hostHint.trim().toLowerCase();
    if (!normalizedHint) return;
    const workloadNode = options.workloadNodeOptions().find((candidate) => {
      const label = candidate.label.toLowerCase();
      const value = candidate.value.toLowerCase();
      return label === normalizedHint || value === normalizedHint || label.includes(normalizedHint);
    });
    if (!workloadNode) return;
    options.setSelectedNode(workloadNode.value);
    options.setSelectedHostHint(null);
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (options.viewMode() !== 'pod') return;
    const selected = (options.selectedKubernetesNamespace() || '').trim();
    if (!selected) return;
    const normalized = selected.toLowerCase();
    const exists = options
      .kubernetesNamespaceOptions()
      .some((value) => value.toLowerCase() === normalized);
    if (!exists) {
      options.setSelectedKubernetesNamespace(null);
    }
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (options.viewMode() !== 'app-container') return;
    const selected = options.containerRuntime().trim();
    if (!selected) return;
    const normalized = selected.toLowerCase();
    const exists = options
      .containerRuntimeOptions()
      .some((value) => value.toLowerCase() === normalized);
    if (!exists) {
      options.setContainerRuntime('');
    }
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (options.viewMode() === 'pod') {
      if (options.selectedNode() !== null) {
        options.setSelectedNode(null);
      }
      if (options.selectedHostHint() !== null) {
        options.setSelectedHostHint(null);
      }
      return;
    }
    if (options.selectedKubernetesContext() !== null) {
      options.setSelectedKubernetesContext(null);
    }
    if (options.selectedKubernetesNamespace() !== null) {
      options.setSelectedKubernetesNamespace(null);
    }
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (options.viewMode() !== 'app-container' && options.containerRuntime().trim() !== '') {
      options.setContainerRuntime('');
    }
  });

  createEffect(() => {
    const parsed = workloadUrlParams();
    const normalizedType = parsed.type;
    if (normalizedType === handledTypeParam()) return;

    if (!normalizedType) {
      setHandledTypeParam('');
      return;
    }

    const nextMode = resolveDashboardWorkloadTypeParam(parsed);
    if (!nextMode) {
      setHandledTypeParam(normalizedType);
      return;
    }

    options.setViewMode(nextMode);
    setHandledTypeParam(normalizedType);
  });

  createEffect(() => {
    const normalized = workloadUrlParams().platform;
    const currentSelected = options.selectedPlatform();

    if (normalized) {
      if (currentSelected !== normalized) {
        options.setSelectedPlatform(normalized);
      }
      if (!options.showFilters()) {
        options.setShowFilters(true);
      }
      if (handledPlatformParam() !== normalized) {
        setHandledPlatformParam(normalized);
      }
      return;
    }

    if (currentSelected !== null) {
      options.setSelectedPlatform(null);
    }
    if (handledPlatformParam() !== '') {
      setHandledPlatformParam('');
    }
  });

  createEffect(() => {
    const normalized = workloadUrlParams().context;
    if (normalized === handledContextParam()) return;

    if (normalized) {
      if (options.viewMode() !== 'pod') {
        options.setViewMode('pod');
      }
      options.setSelectedKubernetesContext(normalized);
      if (!options.showFilters()) {
        options.setShowFilters(true);
      }
      setHandledContextParam(normalized);
      return;
    }

    options.setSelectedKubernetesContext(null);
    setHandledContextParam('');
  });

  createEffect(() => {
    const normalized = workloadUrlParams().namespace;
    if (normalized === handledNamespaceParam()) return;

    if (normalized) {
      if (options.viewMode() !== 'pod') {
        options.setViewMode('pod');
      }
      options.setSelectedKubernetesNamespace(normalized);
      if (!options.showFilters()) {
        options.setShowFilters(true);
      }
      setHandledNamespaceParam(normalized);
      return;
    }

    options.setSelectedKubernetesNamespace(null);
    setHandledNamespaceParam('');
  });

  createEffect(() => {
    const normalized = workloadUrlParams().agent;
    if (normalized === handledAgentParam()) return;

    if (normalized) {
      options.setSelectedHostHint(normalized);
      if (!options.showFilters()) {
        options.setShowFilters(true);
      }
      setHandledAgentParam(normalized);
      return;
    }

    options.setSelectedHostHint(null);
    if (options.selectedNode() !== null) {
      options.setSelectedNode(null);
    }
    setHandledAgentParam('');
  });

  createEffect(() => {
    const parsed = workloadUrlParams();
    const urlRuntime = parsed.runtime;
    if (urlRuntime === handledRuntimeParam()) return;

    const resolution = resolveDashboardWorkloadRuntimeParam(parsed);
    if (!resolution.shouldApply) {
      setHandledRuntimeParam(urlRuntime);
      return;
    }

    if (resolution.forceViewMode && options.viewMode() !== resolution.forceViewMode) {
      options.setViewMode(resolution.forceViewMode);
    }
    options.setContainerRuntime(resolution.runtime);
    if (resolution.runtime && !options.showFilters()) {
      options.setShowFilters(true);
    }
    setHandledRuntimeParam(urlRuntime);
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;

    const parsed = workloadUrlParams();
    const urlType = parsed.type;
    const urlRuntime = parsed.runtime;
    const urlContext = parsed.context;
    const urlNamespace = parsed.namespace;
    const urlAgent = parsed.agent;
    const urlPlatform = parsed.platform;
    const urlResource = parsed.resource;

    if (handledTypeParam() !== urlType) return;
    if (handledPlatformParam() !== urlPlatform) return;
    if (handledRuntimeParam() !== urlRuntime) return;
    if (handledContextParam() !== urlContext) return;
    if (handledNamespaceParam() !== urlNamespace) return;
    if (handledAgentParam() !== urlAgent) return;
    if (urlResource) return;

    const nextPath = resolveDashboardManagedWorkloadsNavigateTarget({
      currentSearch: location.search,
      viewMode: options.viewMode(),
      containerRuntime: options.containerRuntime(),
      selectedKubernetesContext: options.selectedKubernetesContext(),
      selectedKubernetesNamespace: options.selectedKubernetesNamespace(),
      selectedNode: options.selectedNode(),
      selectedHostHint: options.selectedHostHint(),
      selectedPlatform: options.selectedPlatform(),
    });
    if (nextPath) {
      scheduleUrlSyncNavigate(nextPath);
    }
  });

  return {
    isWorkloadsRoute,
  } as const;
}
