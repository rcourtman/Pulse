import {
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  untrack,
  type Accessor,
  type Setter,
} from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import {
  buildWorkloadsPath,
  parseWorkloadsLinkSearch,
  WORKLOADS_PATH,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import { areSearchParamsEquivalent } from '@/utils/searchParams';
import { normalizeWorkloadViewModeParam, resolveWorkloadType } from '@/utils/workloads';
import { getKubernetesContextKey, workloadNodeScopeId } from './workloadSelectors';
import type { DashboardToolbarFilterConfig } from './dashboardFilterModel';

export interface DashboardWorkloadRouteStateOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  showFilters: Accessor<boolean>;
  setShowFilters: Setter<boolean>;
}

export function useDashboardWorkloadRouteState(options: DashboardWorkloadRouteStateOptions) {
  const navigate = useNavigate();
  const location = useLocation();
  const isWorkloadsRoute = () => location.pathname === WORKLOADS_PATH;

  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [selectedKubernetesContext, setSelectedKubernetesContext] = createSignal<string | null>(
    null,
  );
  const [selectedKubernetesNamespace, setSelectedKubernetesNamespace] = createSignal<
    string | null
  >(null);
  const [handledTypeParam, setHandledTypeParam] = createSignal('');
  const [handledRuntimeParam, setHandledRuntimeParam] = createSignal('');
  const [handledContextParam, setHandledContextParam] = createSignal('');
  const [handledNamespaceParam, setHandledNamespaceParam] = createSignal('');
  const [handledAgentParam, setHandledAgentParam] = createSignal('');
  const [selectedHostHint, setSelectedHostHint] = createSignal<string | null>(null);

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

  const [viewMode, setViewMode] = usePersistentSignal<ViewMode>('dashboardViewMode', 'all', {
    deserialize: (raw) => normalizeWorkloadViewModeParam(raw) ?? 'all',
  });

  const [containerRuntime, setContainerRuntime] = usePersistentSignal<string>(
    'dashboardContainerRuntime',
    '',
    {
      deserialize: (raw) => (typeof raw === 'string' ? raw : ''),
      serialize: (value) => value,
    },
  );

  const workloadNodeOptions = createMemo(() => {
    const labelsByScope = new Map<string, string>();
    const nodeNameCounts = new Map<string, number>();

    for (const guest of options.allGuests()) {
      const type = resolveWorkloadType(guest);
      if (type === 'pod') continue;
      const scope = workloadNodeScopeId(guest);
      if (!scope || scope === '-') continue;
      const nodeName = (guest.node || '').trim();
      if (!nodeName) continue;
      nodeNameCounts.set(nodeName, (nodeNameCounts.get(nodeName) || 0) + 1);
    }

    for (const guest of options.allGuests()) {
      const type = resolveWorkloadType(guest);
      if (type === 'pod') continue;
      const scope = workloadNodeScopeId(guest);
      if (!scope || scope === '-' || labelsByScope.has(scope)) continue;
      const nodeName = (guest.node || '').trim();
      const instance = (guest.instance || '').trim();
      if (!nodeName) continue;
      const hasDuplicateNodeName = (nodeNameCounts.get(nodeName) || 0) > 1;
      const label = hasDuplicateNodeName && instance ? `${nodeName} (${instance})` : nodeName;
      labelsByScope.set(scope, label);
    }

    return Array.from(labelsByScope.entries())
      .map(([value, label]) => ({ value, label }))
      .sort((a, b) => a.label.localeCompare(b.label));
  });

  createEffect(() => {
    if (viewMode() === 'pod') return;
    const hostHint = selectedHostHint();
    if (!hostHint || selectedNode() !== null) return;
    const normalizedHint = hostHint.trim().toLowerCase();
    if (!normalizedHint) return;
    const option = workloadNodeOptions().find((candidate) => {
      const label = candidate.label.toLowerCase();
      const value = candidate.value.toLowerCase();
      return label === normalizedHint || value === normalizedHint || label.includes(normalizedHint);
    });
    if (!option) return;
    setSelectedNode(option.value);
    setSelectedHostHint(null);
  });

  const kubernetesContextOptions = createMemo(() => {
    const contexts = new Set<string>();
    for (const guest of options.allGuests()) {
      if (resolveWorkloadType(guest) !== 'pod') continue;
      const context = getKubernetesContextKey(guest);
      if (context) {
        contexts.add(context);
      }
    }
    return Array.from(contexts).sort((a, b) => a.localeCompare(b));
  });

  const kubernetesNamespaceOptions = createMemo(() => {
    const namespaces = new Set<string>();
    const contextFilter = (selectedKubernetesContext() || '').trim();
    for (const guest of options.allGuests()) {
      if (resolveWorkloadType(guest) !== 'pod') continue;
      if (contextFilter && getKubernetesContextKey(guest) !== contextFilter) continue;
      const ns = (guest.namespace || '').trim();
      if (ns) namespaces.add(ns);
    }
    return Array.from(namespaces).sort((a, b) => a.localeCompare(b));
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (viewMode() !== 'pod') return;
    const selected = (selectedKubernetesNamespace() || '').trim();
    if (!selected) return;
    const normalized = selected.toLowerCase();
    const exists = kubernetesNamespaceOptions().some((value) => value.toLowerCase() === normalized);
    if (!exists) {
      setSelectedKubernetesNamespace(null);
    }
  });

  const containerRuntimeOptions = createMemo(() => {
    const runtimes = new Set<string>();
    for (const guest of options.allGuests()) {
      if (resolveWorkloadType(guest) !== 'app-container') continue;
      const runtime = (guest.containerRuntime || '').trim();
      if (runtime) {
        runtimes.add(runtime);
      }
    }
    return Array.from(runtimes).sort((a, b) => a.localeCompare(b));
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (viewMode() !== 'app-container') return;
    const selected = containerRuntime().trim();
    if (!selected) return;
    const normalized = selected.toLowerCase();
    const exists = containerRuntimeOptions().some((value) => value.toLowerCase() === normalized);
    if (!exists) {
      setContainerRuntime('');
    }
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (viewMode() === 'pod') {
      if (selectedNode() !== null) {
        setSelectedNode(null);
      }
      if (selectedHostHint() !== null) {
        setSelectedHostHint(null);
      }
      return;
    }
    if (selectedKubernetesContext() !== null) {
      setSelectedKubernetesContext(null);
    }
    if (selectedKubernetesNamespace() !== null) {
      setSelectedKubernetesNamespace(null);
    }
  });

  createEffect(() => {
    if (!isWorkloadsRoute()) return;
    if (viewMode() !== 'app-container' && containerRuntime().trim() !== '') {
      setContainerRuntime('');
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

    setViewMode(nextMode);
    setHandledTypeParam(normalizedType);
  });

  createEffect(() => {
    const { context: contextParam } = parseWorkloadsLinkSearch(location.search);
    const normalized = contextParam ?? '';
    if (normalized === handledContextParam()) return;

    if (normalized) {
      if (viewMode() !== 'pod') {
        setViewMode('pod');
      }
      setSelectedKubernetesContext(normalized);
      if (!options.showFilters()) {
        options.setShowFilters(true);
      }
      setHandledContextParam(normalized);
      return;
    }

    setSelectedKubernetesContext(null);
    setHandledContextParam('');
  });

  createEffect(() => {
    const { namespace: namespaceParam } = parseWorkloadsLinkSearch(location.search);
    const normalized = namespaceParam ?? '';
    if (normalized === handledNamespaceParam()) return;

    if (normalized) {
      if (viewMode() !== 'pod') {
        setViewMode('pod');
      }
      setSelectedKubernetesNamespace(normalized);
      if (!options.showFilters()) {
        options.setShowFilters(true);
      }
      setHandledNamespaceParam(normalized);
      return;
    }

    setSelectedKubernetesNamespace(null);
    setHandledNamespaceParam('');
  });

  createEffect(() => {
    const { agent: agentParam } = parseWorkloadsLinkSearch(location.search);
    const normalized = agentParam ?? '';
    if (normalized === handledAgentParam()) return;

    if (normalized) {
      setSelectedHostHint(normalized);
      if (!options.showFilters()) {
        options.setShowFilters(true);
      }
      setHandledAgentParam(normalized);
      return;
    }

    setSelectedHostHint(null);
    if (selectedNode() !== null) {
      setSelectedNode(null);
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
      setContainerRuntime('');
      setHandledRuntimeParam('');
      return;
    }

    if (viewMode() !== 'app-container') {
      setViewMode('app-container');
    }
    setContainerRuntime(urlRuntime);
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
    const nextType = viewMode() === 'all' ? '' : viewMode();
    const nextRuntime = viewMode() === 'app-container' ? containerRuntime().trim() : '';
    const nextContext = viewMode() === 'pod' ? (selectedKubernetesContext() ?? '') : '';
    const nextNamespace = viewMode() === 'pod' ? (selectedKubernetesNamespace() ?? '') : '';
    const nextAgent = viewMode() === 'pod' ? '' : (selectedNode() ?? selectedHostHint() ?? '');

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

  const containerRuntimeFilterConfig = createMemo<DashboardToolbarFilterConfig | undefined>(() => {
    if (!isWorkloadsRoute()) return undefined;
    if (viewMode() !== 'app-container') return undefined;

    const options = containerRuntimeOptions();
    if (options.length === 0) return undefined;

    return {
      id: 'workloads-container-runtime-filter',
      label: 'Runtime',
      value: containerRuntime(),
      options: [
        { value: '', label: 'All runtimes' },
        ...options.map((value) => ({ value, label: value })),
      ],
      onChange: (value: string) => setContainerRuntime(value),
    };
  });

  const hostFilterConfig = createMemo<DashboardToolbarFilterConfig | undefined>(() => {
    if (!isWorkloadsRoute()) return undefined;

    if (viewMode() === 'pod') {
      return {
        id: 'workloads-k8s-context-filter',
        label: 'Cluster',
        value: selectedKubernetesContext() ?? '',
        options: [
          { value: '', label: 'All clusters' },
          ...kubernetesContextOptions().map((context) => ({
            value: context,
            label: context,
          })),
        ],
        onChange: (value: string) => setSelectedKubernetesContext(value || null),
      };
    }

    return {
      id: 'workloads-node-filter',
      label: 'Node',
      value: selectedNode() ?? '',
      options: [{ value: '', label: 'All nodes' }, ...workloadNodeOptions()],
      onChange: (value: string) => {
        handleNodeSelect(value || null, value ? 'pve' : null);
      },
    };
  });

  const namespaceFilterConfig = createMemo<DashboardToolbarFilterConfig | undefined>(() => {
    if (!isWorkloadsRoute()) return undefined;
    if (viewMode() !== 'pod') return undefined;

    const options = kubernetesNamespaceOptions();
    if (options.length === 0) return undefined;

    return {
      id: 'workloads-k8s-namespace-filter',
      label: 'Namespace',
      value: selectedKubernetesNamespace() ?? '',
      options: [
        { value: '', label: 'All namespaces' },
        ...options.map((value) => ({ value, label: value })),
      ],
      onChange: (value: string) => setSelectedKubernetesNamespace(value || null),
    };
  });

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
