import {
  createMemo,
  createSignal,
  type Accessor,
  type Setter,
} from 'solid-js';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { resolveWorkloadType } from '@/utils/workloads';
import type { DashboardToolbarFilterConfig } from './dashboardFilterModel';
import { getKubernetesContextKey, workloadNodeScopeId } from './workloadTopology';
import { useDashboardWorkloadUrlSync } from './useDashboardWorkloadUrlSync';

export interface DashboardWorkloadRouteStateOptions {
  allGuests: Accessor<WorkloadGuest[]>;
  showFilters: Accessor<boolean>;
  setShowFilters: Setter<boolean>;
}

export function useDashboardWorkloadRouteState(options: DashboardWorkloadRouteStateOptions) {
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [selectedKubernetesContext, setSelectedKubernetesContext] = createSignal<string | null>(
    null,
  );
  const [selectedKubernetesNamespace, setSelectedKubernetesNamespace] = createSignal<
    string | null
  >(null);
  const [selectedHostHint, setSelectedHostHint] = createSignal<string | null>(null);

  const [viewMode, setViewMode] = usePersistentSignal<ViewMode>('dashboardViewMode', 'all', {
    deserialize: (raw) => {
      if (typeof raw !== 'string') return 'all';
      const normalized = raw.trim().toLowerCase();
      if (
        normalized === 'all' ||
        normalized === 'vm' ||
        normalized === 'system-container' ||
        normalized === 'app-container' ||
        normalized === 'pod'
      ) {
        return normalized as ViewMode;
      }
      if (normalized === 'docker') return 'app-container';
      if (normalized === 'kubernetes') return 'pod';
      return 'all';
    },
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

  const { isWorkloadsRoute } = useDashboardWorkloadUrlSync({
    containerRuntime,
    containerRuntimeOptions,
    kubernetesNamespaceOptions,
    selectedHostHint,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setSelectedHostHint,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setSelectedNode,
    setShowFilters: options.setShowFilters,
    setViewMode,
    showFilters: options.showFilters,
    viewMode,
    workloadNodeOptions,
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
