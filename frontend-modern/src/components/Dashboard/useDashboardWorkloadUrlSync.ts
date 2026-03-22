import {
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
  buildWorkloadsPath,
  parseWorkloadsLinkSearch,
  WORKLOADS_PATH,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import { areSearchParamsEquivalent } from '@/utils/searchParams';
import { normalizeWorkloadViewModeParam } from '@/utils/workloads';
import type { DashboardWorkloadNodeOption } from './dashboardWorkloadRouteModel';

export interface DashboardWorkloadUrlSyncOptions {
  containerRuntime: Accessor<string>;
  containerRuntimeOptions: Accessor<string[]>;
  kubernetesNamespaceOptions: Accessor<string[]>;
  selectedHostHint: Accessor<string | null>;
  selectedKubernetesContext: Accessor<string | null>;
  selectedKubernetesNamespace: Accessor<string | null>;
  selectedNode: Accessor<string | null>;
  setContainerRuntime: Setter<string>;
  setSelectedHostHint: Setter<string | null>;
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

  const [handledTypeParam, setHandledTypeParam] = createSignal('');
  const [handledRuntimeParam, setHandledRuntimeParam] = createSignal('');
  const [handledContextParam, setHandledContextParam] = createSignal('');
  const [handledNamespaceParam, setHandledNamespaceParam] = createSignal('');
  const [handledAgentParam, setHandledAgentParam] = createSignal('');

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
    const parsed = parseWorkloadsLinkSearch(location.search);
    const typeParam = parsed.type;
    const normalizedType = typeParam ?? '';
    if (normalizedType === handledTypeParam()) return;

    if (!normalizedType) {
      setHandledTypeParam('');
      return;
    }

    const hasK8sScope =
      Boolean((parsed.context ?? '').trim()) || Boolean((parsed.namespace ?? '').trim());
    const nextMode = normalizeWorkloadViewModeParam(normalizedType);
    if (!nextMode) {
      setHandledTypeParam(normalizedType);
      return;
    }
    if (hasK8sScope && nextMode !== 'pod') {
      setHandledTypeParam(normalizedType);
      return;
    }

    options.setViewMode(nextMode);
    setHandledTypeParam(normalizedType);
  });

  createEffect(() => {
    const { context: contextParam } = parseWorkloadsLinkSearch(location.search);
    const normalized = contextParam ?? '';
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
    const { namespace: namespaceParam } = parseWorkloadsLinkSearch(location.search);
    const normalized = namespaceParam ?? '';
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
    const { agent: agentParam } = parseWorkloadsLinkSearch(location.search);
    const normalized = agentParam ?? '';
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
    const parsed = parseWorkloadsLinkSearch(location.search);
    const urlRuntime = parsed.runtime ?? '';
    if (urlRuntime === handledRuntimeParam()) return;

    const urlContext = parsed.context ?? '';
    const hasContext = Boolean(urlContext.trim());
    const hasNamespace = Boolean((parsed.namespace ?? '').trim());
    const urlType = parsed.type ?? '';
    const nextMode = normalizeWorkloadViewModeParam(urlType);
    const runtimeRelevant =
      !hasContext && !hasNamespace && (nextMode === 'app-container' || !urlType.trim());

    if (!runtimeRelevant) {
      setHandledRuntimeParam(urlRuntime);
      return;
    }

    if (!urlRuntime.trim()) {
      options.setContainerRuntime('');
      setHandledRuntimeParam('');
      return;
    }

    if (options.viewMode() !== 'app-container') {
      options.setViewMode('app-container');
    }
    options.setContainerRuntime(urlRuntime);
    if (!options.showFilters()) {
      options.setShowFilters(true);
    }
    setHandledRuntimeParam(urlRuntime);
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;

    const parsed = parseWorkloadsLinkSearch(location.search);
    const urlType = parsed.type ?? '';
    const urlRuntime = parsed.runtime ?? '';
    const urlContext = parsed.context ?? '';
    const urlNamespace = parsed.namespace ?? '';
    const urlAgent = parsed.agent ?? '';
    const urlResource = parsed.resource ?? '';

    if (handledTypeParam() !== urlType) return;
    if (handledRuntimeParam() !== urlRuntime) return;
    if (handledContextParam() !== urlContext) return;
    if (handledNamespaceParam() !== urlNamespace) return;
    if (handledAgentParam() !== urlAgent) return;
    if (urlResource) return;

    const currentParams = new URLSearchParams(location.search);
    const nextParams = new URLSearchParams(location.search);
    const nextType = options.viewMode() === 'all' ? '' : options.viewMode();
    const nextRuntime =
      options.viewMode() === 'app-container' ? options.containerRuntime().trim() : '';
    const nextContext =
      options.viewMode() === 'pod' ? (options.selectedKubernetesContext() ?? '') : '';
    const nextNamespace =
      options.viewMode() === 'pod' ? (options.selectedKubernetesNamespace() ?? '') : '';
    const nextAgent =
      options.viewMode() === 'pod'
        ? ''
        : (options.selectedNode() ?? options.selectedHostHint() ?? '');

    const managedPath = buildWorkloadsPath({
      type: nextType || null,
      runtime: nextRuntime || null,
      context: nextContext || null,
      namespace: nextNamespace || null,
      agent: nextAgent || null,
    });
    const managedUrl = new URL(managedPath, 'http://pulse.local');
    nextParams.delete(WORKLOADS_QUERY_PARAMS.type);
    nextParams.delete(WORKLOADS_QUERY_PARAMS.runtime);
    nextParams.delete(WORKLOADS_QUERY_PARAMS.context);
    nextParams.delete(WORKLOADS_QUERY_PARAMS.namespace);
    nextParams.delete(WORKLOADS_QUERY_PARAMS.agent);
    managedUrl.searchParams.forEach((value, key) => {
      nextParams.set(key, value);
    });

    if (!areSearchParamsEquivalent(currentParams, nextParams)) {
      const nextSearch = nextParams.toString();
      const nextPath = nextSearch ? `${WORKLOADS_PATH}?${nextSearch}` : WORKLOADS_PATH;
      scheduleUrlSyncNavigate(nextPath);
    }
  });

  return {
    isWorkloadsRoute,
  } as const;
}
