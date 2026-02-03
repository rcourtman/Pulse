import type { Component } from 'solid-js';
import { For, Show, createMemo, createSignal, createEffect, onMount, Accessor } from 'solid-js';
import { AIAPI } from '@/api/ai';
import Sparkles from 'lucide-solid/icons/sparkles';
import ExternalLink from 'lucide-solid/icons/external-link';
import { LicenseAPI, type LicenseFeatureStatus } from '@/api/license';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { useColumnVisibility, type ColumnDef } from '@/hooks/useColumnVisibility';
import type {
  KubernetesCluster,
  KubernetesDeployment,
  KubernetesNode,
  KubernetesPod,
} from '@/types/api';
import type { AISettings } from '@/types/ai';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { ScrollableTable } from '@/components/shared/ScrollableTable';
import { StatusDot } from '@/components/shared/StatusDot';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import { formatRelativeTime, formatBytes } from '@/utils/format';
import { DEGRADED_HEALTH_STATUSES, OFFLINE_HEALTH_STATUSES, type StatusIndicator } from '@/utils/status';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { HistoryChart } from '@/components/shared/HistoryChart';
import type { HistoryTimeRange } from '@/api/charts';
import { GuestMetadataAPI, type GuestMetadata } from '@/api/guestMetadata';
import { logger } from '@/utils/logger';

type GuestMetadataRecord = Record<string, GuestMetadata>;

// Module-level cache for guest metadata to persist across component remounts
let cachedK8sGuestMetadata: GuestMetadataRecord | null = null;

const getK8sGuestMetadataCache = (): GuestMetadataRecord => {
  return cachedK8sGuestMetadata ?? {};
};

const setK8sGuestMetadataCache = (metadata: GuestMetadataRecord) => {
  cachedK8sGuestMetadata = metadata;
};

// Global state for expanded row
const [expandedRowId, setExpandedRowId] = createSignal<string | null>(null);

interface KubernetesClustersProps {
  clusters: KubernetesCluster[];
}

type ViewMode = 'clusters' | 'nodes' | 'pods' | 'deployments';
type StatusFilter = 'all' | 'healthy' | 'unhealthy';

const normalize = (value?: string | null): string => (value || '').trim().toLowerCase();

const getStatusIndicator = (status: string | undefined | null): StatusIndicator => {
  const normalized = normalize(status);
  if (!normalized) return { variant: 'muted', label: 'Unknown' };
  if (OFFLINE_HEALTH_STATUSES.has(normalized)) return { variant: 'danger', label: 'Offline' };
  if (DEGRADED_HEALTH_STATUSES.has(normalized)) return { variant: 'warning', label: 'Degraded' };
  if (normalized === 'online') return { variant: 'success', label: 'Online' };
  return { variant: 'muted', label: status ?? 'Unknown' };
};

const isPodHealthy = (pod: KubernetesPod): boolean => {
  const phase = normalize(pod.phase);
  if (!phase) return false;
  if (phase !== 'running') return false;

  const containers = pod.containers ?? [];
  if (containers.length === 0) return true;

  return containers.every((container) => {
    if (!container.ready) return false;
    const state = normalize(container.state);
    if (!state) return true;
    return state === 'running';
  });
};

const isDeploymentHealthy = (d: KubernetesDeployment): boolean => {
  const desired = d.desiredReplicas ?? 0;
  if (desired <= 0) return true;
  const available = d.availableReplicas ?? 0;
  const ready = d.readyReplicas ?? 0;
  const updated = d.updatedReplicas ?? 0;
  return available >= desired && ready >= desired && updated >= desired;
};

const getClusterDisplayName = (cluster: KubernetesCluster): string => {
  return cluster.customDisplayName || cluster.displayName || cluster.name || cluster.id;
};

const summarizeNodes = (nodes: KubernetesNode[] | undefined) => {
  const list = nodes ?? [];
  const notReady = list.filter((n) => !n.ready).length;
  const unschedulable = list.filter((n) => !!n.unschedulable).length;
  return { total: list.length, notReady, unschedulable };
};

const summarizePods = (pods: KubernetesPod[] | undefined) => {
  const list = pods ?? [];
  const unhealthy = list.filter((p) => !isPodHealthy(p)).length;
  return { total: list.length, unhealthy };
};

const summarizeDeployments = (deployments: KubernetesDeployment[] | undefined) => {
  const list = deployments ?? [];
  const unhealthy = list.filter((d) => !isDeploymentHealthy(d)).length;
  return { total: list.length, unhealthy };
};

const getPodStatusBadge = (pod: KubernetesPod) => {
  if (isPodHealthy(pod)) {
    return { class: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300', label: 'Running' };
  }
  const phase = normalize(pod.phase);
  if (phase === 'pending') {
    return { class: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300', label: 'Pending' };
  }
  if (phase === 'failed') {
    return { class: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300', label: 'Failed' };
  }
  if (phase === 'succeeded') {
    return { class: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300', label: 'Completed' };
  }
  // Check for CrashLoopBackOff or other container issues
  const containers = pod.containers ?? [];
  const crashingContainer = containers.find((c) => c.reason?.toLowerCase().includes('crash'));
  if (crashingContainer) {
    return { class: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300', label: 'CrashLoop' };
  }
  const waitingContainer = containers.find((c) => normalize(c.state) === 'waiting');
  if (waitingContainer) {
    return { class: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300', label: waitingContainer.reason || 'Waiting' };
  }
  return { class: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300', label: pod.phase || 'Unknown' };
};

// Get primary container image (first container)
const getPrimaryImage = (pod: KubernetesPod): string => {
  const containers = pod.containers ?? [];
  if (containers.length === 0) return '—';
  const image = containers[0].image ?? '';
  // Truncate long image names, show just the image:tag part
  const parts = image.split('/');
  return parts[parts.length - 1] || image || '—';
};

// Format age from timestamp
const formatAge = (timestamp?: number | string | null): string => {
  if (!timestamp) return '—';
  const ts = typeof timestamp === 'string' ? Date.parse(timestamp) : timestamp;
  if (isNaN(ts)) return '—';
  return formatRelativeTime(ts);
};

// Column definitions for the pods table
const POD_COLUMNS: ColumnDef[] = [
  { id: 'name', label: 'Pod', priority: 'essential', toggleable: false },
  { id: 'namespace', label: 'Namespace', priority: 'essential', toggleable: true },
  { id: 'cluster', label: 'Cluster', priority: 'primary', toggleable: true },
  { id: 'status', label: 'Status', priority: 'essential', toggleable: false },
  { id: 'ready', label: 'Ready', priority: 'primary', toggleable: true },
  { id: 'restarts', label: 'Restarts', priority: 'primary', toggleable: true },
  { id: 'image', label: 'Image', priority: 'secondary', toggleable: true },
  { id: 'age', label: 'Age', priority: 'primary', toggleable: true },
];

const PodRow: Component<{
  cluster: KubernetesCluster;
  pod: KubernetesPod;
  columns: { isColumnVisible: (id: string) => boolean };
  guestMetadata: Accessor<GuestMetadataRecord>;
  onCustomUrlChange: (guestId: string, url: string) => void;
}> = (props) => {
  const rowId = `${props.cluster.id}:${props.pod.uid}`;
  const isExpanded = createMemo(() => expandedRowId() === rowId);
  const [activeTab, setActiveTab] = createSignal<'overview' | 'discovery'>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>('1h');

  const toggle = (e: MouseEvent) => {
    if ((e.target as HTMLElement).closest('a, button, input')) return;
    setExpandedRowId(prev => prev === rowId ? null : rowId);
  };

  const statusBadge = () => getPodStatusBadge(props.pod);
  const containers = () => props.pod.containers ?? [];
  const readyContainers = () => containers().filter(c => c.ready).length;

  return (
    <>
      <tr
        class={`transition-colors cursor-pointer ${isExpanded() ? 'bg-gray-50 dark:bg-gray-800/40' : 'hover:bg-gray-50 dark:hover:bg-gray-900/20'}`}
        onClick={toggle}
      >
        <td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">
          <div class="font-medium truncate max-w-[200px]" title={props.pod.name}>{props.pod.name}</div>
          <div class="text-xs text-gray-500 dark:text-gray-400 truncate">
            {props.pod.nodeName || 'unscheduled'}
          </div>
        </td>
        <Show when={props.columns.isColumnVisible('namespace')}>
          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
            <span class="px-2 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-xs font-mono">
              {props.pod.namespace}
            </span>
          </td>
        </Show>
        <Show when={props.columns.isColumnVisible('cluster')}>
          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
            {getClusterDisplayName(props.cluster)}
          </td>
        </Show>
        <td class="px-4 py-3 text-sm">
          <span class={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${statusBadge().class}`}>
            {statusBadge().label}
          </span>
        </td>
        <Show when={props.columns.isColumnVisible('ready')}>
          <td class="px-4 py-3 text-sm">
            <span class={readyContainers() === containers().length ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'}>
              {readyContainers()}/{containers().length}
            </span>
          </td>
        </Show>
        <Show when={props.columns.isColumnVisible('restarts')}>
          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
            <Show when={(props.pod.restarts ?? 0) > 0} fallback={<span class="text-gray-400">0</span>}>
              <span class="text-amber-600 dark:text-amber-400 font-medium">{props.pod.restarts}</span>
            </Show>
          </td>
        </Show>
        <Show when={props.columns.isColumnVisible('image')}>
          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
            <span class="font-mono text-xs truncate max-w-[150px] block" title={(props.pod.containers ?? [])[0]?.image}>
              {getPrimaryImage(props.pod)}
            </span>
          </td>
        </Show>
        <Show when={props.columns.isColumnVisible('age')}>
          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 whitespace-nowrap">
            {formatAge(props.pod.createdAt)}
          </td>
        </Show>
      </tr>

      <Show when={isExpanded()}>
        <tr>
          <td colspan={8} class="p-0">
            <div class="w-0 min-w-full bg-gray-50 dark:bg-gray-900/60 px-4 py-3 overflow-hidden">
              {/* Tabs Header */}
              <div class="flex border-b border-gray-200 dark:border-gray-700/50 mb-4 bg-white/50 dark:bg-gray-800/50 backdrop-blur-sm sticky top-0 z-10 -mx-4 px-4 pt-2">
                <button
                  type="button"
                  class={`px-4 py-2 text-xs font-medium border-b-2 transition-colors ${activeTab() === 'overview' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'}`}
                  onClick={(e) => { e.stopPropagation(); setActiveTab('overview'); }}
                >
                  Overview
                </button>
                <button
                  type="button"
                  class={`px-4 py-2 text-xs font-medium border-b-2 transition-colors ${activeTab() === 'discovery' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'}`}
                  onClick={(e) => { e.stopPropagation(); setActiveTab('discovery'); }}
                >
                  Discovery
                </button>
              </div>

              <div class={activeTab() === 'overview' ? '' : 'hidden'}>
                {/* Overview Content */}
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <Card>
                    <div class="p-4 space-y-3">
                      <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Details</h3>
                      <div class="grid grid-cols-2 gap-2 text-xs">
                        <div class="text-gray-500 dark:text-gray-400">ID</div>
                        <div class="font-mono truncate" title={props.pod.uid}>{props.pod.uid}</div>
                        <div class="text-gray-500 dark:text-gray-400">QoS Class</div>
                        <div>{props.pod.qosClass || '—'}</div>
                      </div>
                    </div>
                  </Card>

                  <Card>
                    <div class="p-4 space-y-3">
                      <div class="flex justify-between items-center">
                        <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">Containers</h3>
                        <span class="text-xs text-gray-500">{containers().length}</span>
                      </div>
                      <div class="space-y-2 max-h-[300px] overflow-y-auto">
                        <For each={containers()}>
                          {(container) => (
                            <div class="border rounded p-2 text-xs bg-gray-50 dark:bg-gray-800">
                              <div class="flex justify-between font-medium">
                                <span>{container.name}</span>
                                <span class={container.ready ? 'text-green-600' : 'text-amber-600'}>{container.state}</span>
                              </div>
                              <div class="font-mono text-gray-500 truncate" title={container.image}>{container.image}</div>
                              <div class="mt-1 flex gap-2 text-[10px] text-gray-400">
                                <span>Restarts: {container.restartCount}</span>
                              </div>
                            </div>
                          )}
                        </For>
                      </div>
                    </div>
                  </Card>
                </div>
                <div class="mt-3 space-y-3">
                  <div class="flex items-center gap-2">
                    <svg class="w-3.5 h-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" />
                      <path stroke-linecap="round" d="M12 6v6l4 2" />
                    </svg>
                    <select
                      value={historyRange()}
                      onChange={(e) => setHistoryRange(e.currentTarget.value as HistoryTimeRange)}
                      class="text-[11px] font-medium pl-2 pr-6 py-1 rounded-md border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 cursor-pointer focus:ring-1 focus:ring-blue-500 focus:border-blue-500 appearance-none"
                      style={{ "background-image": "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%239ca3af' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E\")", "background-repeat": "no-repeat", "background-position": "right 6px center" }}
                    >
                      <option value="1h">Last 1 hour</option>
                      <option value="6h">Last 6 hours</option>
                      <option value="12h">Last 12 hours</option>
                      <option value="24h">Last 24 hours</option>
                      <option value="7d">Last 7 days</option>
                      <option value="30d">Last 30 days</option>
                      <option value="90d">Last 90 days</option>
                    </select>
                  </div>

                  <div class="relative">
                    <div class="space-y-3">
                      <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(50%-0.5rem)] [&>*]:min-w-[250px]">
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                          <HistoryChart
                            resourceType="k8s"
                            resourceId={props.pod.uid}
                            metric="cpu"
                            height={120}
                            color="#8b5cf6"
                            label="CPU"
                            unit="%"
                            range={historyRange()}
                            hideSelector={true}
                            compact={true}
                            hideLock={true}
                          />
                        </div>
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                          <HistoryChart
                            resourceType="k8s"
                            resourceId={props.pod.uid}
                            metric="memory"
                            height={120}
                            color="#f59e0b"
                            label="Memory"
                            unit="%"
                            range={historyRange()}
                            hideSelector={true}
                            compact={true}
                            hideLock={true}
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div class={activeTab() === 'discovery' ? '' : 'hidden'}>
                <DiscoveryTab
                  resourceType="k8s"
                  hostId={props.cluster.id}
                  resourceId={props.pod.uid}
                  guestId={props.pod.uid}
                  hostname={props.pod.name}
                  customUrl={props.guestMetadata()[props.pod.uid]?.customUrl}
                  onCustomUrlChange={(url) => props.onCustomUrlChange(props.pod.uid, url)}
                />
              </div>
            </div>
          </td>
        </tr>
      </Show>
    </>
  );
};

export const KubernetesClusters: Component<KubernetesClustersProps> = (props) => {
  const [search, setSearch] = createSignal('');
  // PERFORMANCE: Debounce search term to prevent jank during rapid typing
  const debouncedSearch = useDebouncedValue(() => search(), 200);
  const [viewMode, setViewMode] = createSignal<ViewMode>('clusters');
  const [statusFilter, setStatusFilter] = createSignal<StatusFilter>('all');
  const [showHidden, setShowHidden] = createSignal(false);
  const [namespaceFilter, setNamespaceFilter] = createSignal<string>('all');
  const [licenseFeatures, setLicenseFeatures] = createSignal<LicenseFeatureStatus | null>(null);
  const [licenseLoading, setLicenseLoading] = createSignal(true);
  const [aiSettings, setAiSettings] = createSignal<AISettings | null>(null);
  const [aiLoading, setAiLoading] = createSignal(true);
  const [analysisClusterId, setAnalysisClusterId] = createSignal('');
  const [analysisLoading, setAnalysisLoading] = createSignal(false);
  const [analysisResult, setAnalysisResult] = createSignal('');
  const [analysisError, setAnalysisError] = createSignal('');
  const [analysisMeta, setAnalysisMeta] = createSignal<{ model: string; inputTokens: number; outputTokens: number } | null>(null);

  // Column visibility for pods table
  const podColumns = useColumnVisibility('k8s-pod-columns', POD_COLUMNS);

  // Guest metadata for tracking custom URLs
  const [guestMetadata, setGuestMetadata] = createSignal<GuestMetadataRecord>(getK8sGuestMetadataCache());

  // Sorting state with persistence
  type SortKey = 'name' | 'status' | 'namespace' | 'cluster' | 'age' | 'restarts' | 'ready' | 'replicas';
  type SortDir = 'asc' | 'desc';
  const [sortKey, setSortKey] = usePersistentSignal<SortKey>('k8s-sort-key', 'name');
  const [sortDirection, setSortDirection] = usePersistentSignal<SortDir>('k8s-sort-dir', 'asc');

  const toggleSort = (key: SortKey) => {
    if (sortKey() === key) {
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setSortKey(key);
      setSortDirection('asc');
    }
  };

  const sortIndicator = (key: SortKey) => sortKey() === key ? (sortDirection() === 'asc' ? ' ▲' : ' ▼') : '';

  // Search input ref for keyboard focus
  let searchInputRef: HTMLInputElement | undefined;

  // Global keyboard handler - focus search on typing
  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isInputField =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.tagName === 'SELECT' ||
        target.contentEditable === 'true';

      // Escape clears search
      if (e.key === 'Escape' && searchInputRef) {
        setSearch('');
        searchInputRef.blur();
        return;
      }

      // Focus search on printable character (when not in input field)
      if (!isInputField && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        searchInputRef?.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  });

  const kubernetesAiEnabled = createMemo(() => licenseFeatures()?.features?.kubernetes_ai === true);
  const aiConfigured = createMemo(() => aiSettings()?.configured === true);
  const upgradeUrl = createMemo(() => licenseFeatures()?.upgrade_url || 'https://pulserelay.pro/');

  const clustersForAnalysis = createMemo(() => props.clusters ?? []);

  const getClusterOptionLabel = (cluster: KubernetesCluster): string => {
    const base = getClusterDisplayName(cluster);
    if (cluster.pendingUninstall) return `${base} (pending uninstall)`;
    if (cluster.hidden) return `${base} (hidden)`;
    return base;
  };

  const loadLicenseStatus = async () => {
    setLicenseLoading(true);
    try {
      const status = await LicenseAPI.getFeatures();
      setLicenseFeatures(status);
    } catch (_err) {
      setLicenseFeatures(null);
    } finally {
      setLicenseLoading(false);
    }
  };

  const loadAiSettings = async () => {
    setAiLoading(true);
    try {
      const settings = await AIAPI.getSettings();
      setAiSettings(settings);
    } catch (_err) {
      setAiSettings(null);
    } finally {
      setAiLoading(false);
    }
  };

  onMount(() => {
    void loadLicenseStatus();
    void loadAiSettings();

    // Load guest metadata
    GuestMetadataAPI.getAllMetadata().then((metadata) => {
      setGuestMetadata(metadata ?? {});
      setK8sGuestMetadataCache(metadata ?? {});
    }).catch((err) => {
      logger.debug('Failed to load guest metadata for K8s', err);
    });

    // Listen for metadata changes from other sources
    const handleMetadataChanged = async () => {
      try {
        const metadata = await GuestMetadataAPI.getAllMetadata();
        setGuestMetadata(metadata ?? {});
        setK8sGuestMetadataCache(metadata ?? {});
      } catch (err) {
        logger.debug('Failed to refresh guest metadata', err);
      }
    };

    window.addEventListener('pulse:metadata-changed', handleMetadataChanged);
    return () => window.removeEventListener('pulse:metadata-changed', handleMetadataChanged);
  });

  createEffect(() => {
    const clusters = clustersForAnalysis();
    if (clusters.length === 0) {
      setAnalysisClusterId('');
      return;
    }
    if (!clusters.some((cluster) => cluster.id === analysisClusterId())) {
      setAnalysisClusterId(clusters[0].id);
    }
  });

  createEffect(() => {
    analysisClusterId();
    setAnalysisError('');
    setAnalysisResult('');
    setAnalysisMeta(null);
  });

  const handleAnalyzeCluster = async () => {
    if (!analysisClusterId()) {
      setAnalysisError('Select a cluster to analyze.');
      return;
    }
    if (!aiConfigured()) {
      setAnalysisError('Pulse Assistant is not configured. Configure it in Settings → AI.');
      return;
    }
    if (!kubernetesAiEnabled()) {
      setAnalysisError('Pulse Pro is required for Kubernetes analysis.');
      return;
    }

    setAnalysisLoading(true);
    setAnalysisError('');
    setAnalysisResult('');
    setAnalysisMeta(null);
    try {
      const response = await AIAPI.analyzeKubernetesCluster(analysisClusterId());
      setAnalysisResult(response.content || '');
      setAnalysisMeta({
        model: response.model,
        inputTokens: response.input_tokens,
        outputTokens: response.output_tokens,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to analyze cluster';
      setAnalysisError(message);
    } finally {
      setAnalysisLoading(false);
    }
  };

  // Get all unique namespaces for the filter dropdown
  const allNamespaces = createMemo(() => {
    const namespaces = new Set<string>();
    for (const cluster of props.clusters ?? []) {
      if (!showHidden() && cluster.hidden) continue;
      for (const pod of cluster.pods ?? []) {
        if (pod.namespace) namespaces.add(pod.namespace);
      }
      for (const dep of cluster.deployments ?? []) {
        if (dep.namespace) namespaces.add(dep.namespace);
      }
    }
    return Array.from(namespaces).sort();
  });

  // Get all nodes flattened across clusters
  const allNodes = createMemo(() => {
    const clusters = props.clusters ?? [];
    const nodes: Array<{ cluster: KubernetesCluster; node: KubernetesNode }> = [];
    for (const cluster of clusters) {
      if (!showHidden() && cluster.hidden) continue;
      for (const node of cluster.nodes ?? []) {
        nodes.push({ cluster, node });
      }
    }
    return nodes;
  });

  // Get all pods flattened across clusters
  const allPods = createMemo(() => {
    const clusters = props.clusters ?? [];
    const pods: Array<{ cluster: KubernetesCluster; pod: KubernetesPod }> = [];
    for (const cluster of clusters) {
      if (!showHidden() && cluster.hidden) continue;
      for (const pod of cluster.pods ?? []) {
        pods.push({ cluster, pod });
      }
    }
    return pods;
  });

  // Get all deployments flattened across clusters
  const allDeployments = createMemo(() => {
    const clusters = props.clusters ?? [];
    const deps: Array<{ cluster: KubernetesCluster; deployment: KubernetesDeployment }> = [];
    for (const cluster of clusters) {
      if (!showHidden() && cluster.hidden) continue;
      for (const dep of cluster.deployments ?? []) {
        deps.push({ cluster, deployment: dep });
      }
    }
    return deps;
  });

  const visibleClusters = createMemo(() => {
    // PERFORMANCE: Use debounced search term
    const term = debouncedSearch().trim().toLowerCase();
    const clusters = props.clusters ?? [];
    const status = statusFilter();
    const key = sortKey();
    const dir = sortDirection();

    const filtered = clusters
      .filter((cluster) => showHidden() || !cluster.hidden)
      .filter((cluster) => {
        if (status === 'all') return true;
        const clusterStatus = normalize(cluster.status);
        const isHealthy = clusterStatus === 'online';
        if (status === 'healthy') return isHealthy;
        if (status === 'unhealthy') return !isHealthy;
        return true;
      })
      .filter((cluster) => {
        if (!term) return true;
        const haystack = [
          getClusterDisplayName(cluster),
          cluster.id,
          cluster.server ?? '',
          cluster.context ?? '',
          cluster.version ?? '',
        ]
          .join(' ')
          .toLowerCase();
        return haystack.includes(term);
      });

    // Sort clusters
    return filtered.sort((a, b) => {
      let cmp = 0;
      switch (key) {
        case 'name': cmp = getClusterDisplayName(a).localeCompare(getClusterDisplayName(b)); break;
        case 'status': cmp = (normalize(a.status) === 'online' ? 0 : 1) - (normalize(b.status) === 'online' ? 0 : 1); break;
        default: cmp = getClusterDisplayName(a).localeCompare(getClusterDisplayName(b));
      }
      return dir === 'desc' ? -cmp : cmp;
    });
  });

  const filteredNodes = createMemo(() => {
    const term = debouncedSearch().trim().toLowerCase();
    const status = statusFilter();
    const key = sortKey();
    const dir = sortDirection();

    const filtered = allNodes()
      .filter(({ node }) => {
        if (status === 'all') return true;
        const isHealthy = node.ready && !node.unschedulable;
        if (status === 'healthy') return isHealthy;
        if (status === 'unhealthy') return !isHealthy;
        return true;
      })
      .filter(({ cluster, node }) => {
        if (!term) return true;
        const haystack = [
          node.name,
          getClusterDisplayName(cluster),
          node.kubeletVersion ?? '',
          node.osImage ?? '',
          ...(node.roles ?? []),
        ]
          .join(' ')
          .toLowerCase();
        return haystack.includes(term);
      });

    // Sort nodes
    return filtered.sort((a, b) => {
      let cmp = 0;
      switch (key) {
        case 'name': cmp = (a.node.name ?? '').localeCompare(b.node.name ?? ''); break;
        case 'cluster': cmp = getClusterDisplayName(a.cluster).localeCompare(getClusterDisplayName(b.cluster)); break;
        case 'status': cmp = (a.node.ready ? 0 : 1) - (b.node.ready ? 0 : 1); break;
        default: cmp = (a.node.name ?? '').localeCompare(b.node.name ?? '');
      }
      return dir === 'desc' ? -cmp : cmp;
    });
  });

  const filteredPods = createMemo(() => {
    const term = debouncedSearch().trim().toLowerCase();
    const status = statusFilter();
    const ns = namespaceFilter();
    const key = sortKey();
    const dir = sortDirection();

    const filtered = allPods()
      .filter(({ pod }) => {
        if (ns !== 'all' && pod.namespace !== ns) return false;
        if (status === 'all') return true;
        const healthy = isPodHealthy(pod);
        if (status === 'healthy') return healthy;
        if (status === 'unhealthy') return !healthy;
        return true;
      })
      .filter(({ cluster, pod }) => {
        if (!term) return true;
        const haystack = [
          pod.name,
          pod.namespace,
          pod.nodeName ?? '',
          pod.phase ?? '',
          getClusterDisplayName(cluster),
          ...(pod.containers ?? []).map(c => c.image ?? ''),
        ]
          .join(' ')
          .toLowerCase();
        return haystack.includes(term);
      });

    // Sort
    return filtered.sort((a, b) => {
      let cmp = 0;
      switch (key) {
        case 'name': cmp = (a.pod.name ?? '').localeCompare(b.pod.name ?? ''); break;
        case 'namespace': cmp = (a.pod.namespace ?? '').localeCompare(b.pod.namespace ?? ''); break;
        case 'cluster': cmp = getClusterDisplayName(a.cluster).localeCompare(getClusterDisplayName(b.cluster)); break;
        case 'restarts': cmp = (a.pod.restarts ?? 0) - (b.pod.restarts ?? 0); break;
        case 'age': cmp = (a.pod.createdAt ?? 0) - (b.pod.createdAt ?? 0); break;
        case 'status': cmp = (isPodHealthy(a.pod) ? 0 : 1) - (isPodHealthy(b.pod) ? 0 : 1); break;
        default: cmp = (a.pod.name ?? '').localeCompare(b.pod.name ?? '');
      }
      return dir === 'desc' ? -cmp : cmp;
    });
  });

  const filteredDeployments = createMemo(() => {
    const term = debouncedSearch().trim().toLowerCase();
    const status = statusFilter();
    const ns = namespaceFilter();
    const key = sortKey();
    const dir = sortDirection();

    const filtered = allDeployments()
      .filter(({ deployment }) => {
        if (ns !== 'all' && deployment.namespace !== ns) return false;
        if (status === 'all') return true;
        const healthy = isDeploymentHealthy(deployment);
        if (status === 'healthy') return healthy;
        if (status === 'unhealthy') return !healthy;
        return true;
      })
      .filter(({ cluster, deployment }) => {
        if (!term) return true;
        const haystack = [
          deployment.name,
          deployment.namespace,
          getClusterDisplayName(cluster),
        ]
          .join(' ')
          .toLowerCase();
        return haystack.includes(term);
      });

    // Sort
    return filtered.sort((a, b) => {
      let cmp = 0;
      switch (key) {
        case 'name': cmp = (a.deployment.name ?? '').localeCompare(b.deployment.name ?? ''); break;
        case 'namespace': cmp = (a.deployment.namespace ?? '').localeCompare(b.deployment.namespace ?? ''); break;
        case 'cluster': cmp = getClusterDisplayName(a.cluster).localeCompare(getClusterDisplayName(b.cluster)); break;
        case 'replicas': cmp = (a.deployment.desiredReplicas ?? 0) - (b.deployment.desiredReplicas ?? 0); break;
        case 'ready': cmp = (a.deployment.readyReplicas ?? 0) - (b.deployment.readyReplicas ?? 0); break;
        case 'status': cmp = (isDeploymentHealthy(a.deployment) ? 0 : 1) - (isDeploymentHealthy(b.deployment) ? 0 : 1); break;
        default: cmp = (a.deployment.name ?? '').localeCompare(b.deployment.name ?? '');
      }
      return dir === 'desc' ? -cmp : cmp;
    });
  });

  const isEmpty = createMemo(() => (props.clusters?.length ?? 0) === 0);

  const hasActiveFilters = createMemo(
    () => search().trim() !== '' || statusFilter() !== 'all' || showHidden() || namespaceFilter() !== 'all',
  );

  const handleReset = () => {
    setSearch('');
    setStatusFilter('all');
    setShowHidden(false);
    setNamespaceFilter('all');
    setViewMode('clusters');
  };

  // Handle custom URL changes from discovery tab
  const handleK8sCustomUrlChange = (guestId: string, url: string) => {
    const trimmed = url.trim();
    setGuestMetadata((prev) => {
      const updated = { ...prev };
      if (trimmed) {
        updated[guestId] = { ...updated[guestId], id: guestId, customUrl: trimmed };
      } else if (updated[guestId]) {
        const { customUrl: _, ...rest } = updated[guestId];
        if (Object.keys(rest).length > 1) {
          updated[guestId] = rest as GuestMetadata;
        } else {
          delete updated[guestId];
        }
      }
      setK8sGuestMetadataCache(updated);
      return updated;
    });
  };

  return (
    <div class="space-y-4">
      <Show when={clustersForAnalysis().length > 0}>
        <Card padding="sm">
          <div class="flex flex-col gap-3">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="flex-1 min-w-[300px]">
                <div class="flex items-center gap-2">
                  <div class="text-sm font-bold text-gray-900 dark:text-gray-100 flex items-center gap-2">
                    Kubernetes Analysis
                    {/* Badge removed - feature soft-locked instead */}
                  </div>
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400 mt-1 leading-relaxed">
                  Generate deep health insights and actionable remediation for your clusters using Pulse's advanced analysis engine.
                </div>
              </div>
              <Show when={!licenseLoading() && !kubernetesAiEnabled()}>
                <a
                  href={upgradeUrl()}
                  target="_blank"
                  rel="noreferrer"
                  class="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-xl hover:bg-blue-700 transition-all shadow-md text-xs font-bold"
                >
                  Get Pulse Pro
                  <ExternalLink class="w-3 h-3" />
                </a>
              </Show>
            </div>

            <Show when={licenseLoading() || aiLoading()}>
              <div class="flex items-center gap-3 p-4 bg-blue-50/50 dark:bg-blue-900/20 rounded-xl border border-blue-100 dark:border-blue-800 animate-pulse">
                <div class="h-4 w-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
                <span class="text-xs font-medium text-blue-700 dark:text-blue-300">Synchronizing Pulse Assistant & License...</span>
              </div>
            </Show>

            <Show when={!licenseLoading() && !kubernetesAiEnabled()}>
              <div class="relative overflow-hidden p-6 rounded-2xl bg-indigo-50 dark:bg-indigo-950/40 border border-indigo-100 dark:border-indigo-800 shadow-xl group">
                <div class="absolute top-0 right-0 p-8 opacity-10 group-hover:scale-110 transition-transform duration-700">
                  <Sparkles class="w-32 h-32 text-indigo-600" />
                </div>
                <div class="relative flex flex-col items-center text-center max-w-lg mx-auto">
                  <div class="p-3 bg-white dark:bg-gray-800 rounded-2xl shadow-lg mb-4">
                    <Sparkles class="w-8 h-8 text-indigo-500" />
                  </div>
                  <h4 class="text-lg font-bold text-gray-900 dark:text-white mb-2">Power up your Kubernetes Fleet</h4>
                  <p class="text-sm text-gray-600 dark:text-gray-400 mb-6 leading-relaxed">
                    Pulse Pro brings advanced diagnostics to your Kubernetes clusters. Identify bottlenecks, security risks, and configuration drift in seconds.
                  </p>
                  <a
                    href={upgradeUrl()}
                    target="_blank"
                    rel="noreferrer"
                    class="inline-flex items-center gap-2.5 px-6 py-2.5 bg-indigo-600 text-white rounded-xl hover:bg-indigo-700 transform hover:scale-105 active:scale-95 transition-all shadow-lg font-bold text-sm"
                  >
                    Unlock Kubernetes Insights
                    <ExternalLink class="w-4 h-4" />
                  </a>
                </div>
              </div>
            </Show>

            <Show when={!licenseLoading() && kubernetesAiEnabled()}>
              <div class="flex flex-col gap-2">
                <div class="flex flex-wrap items-center gap-2">
                  <select
                    value={analysisClusterId()}
                    onChange={(e) => setAnalysisClusterId(e.currentTarget.value)}
                    class="px-2.5 py-1.5 text-xs font-medium rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                  >
                    <For each={clustersForAnalysis()}>
                      {(cluster) => (
                        <option value={cluster.id}>{getClusterOptionLabel(cluster)}</option>
                      )}
                    </For>
                  </select>
                  <button
                    type="button"
                    onClick={handleAnalyzeCluster}
                    disabled={analysisLoading() || !analysisClusterId() || !aiConfigured()}
                    class={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${analysisLoading() || !analysisClusterId() || !aiConfigured()
                      ? 'bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400 cursor-not-allowed'
                      : 'bg-blue-600 text-white hover:bg-blue-700'
                      }`}
                  >
                    {analysisLoading() ? 'Analyzing...' : 'Analyze'}
                  </button>
                  <Show when={analysisLoading()}>
                    <span class="text-xs text-gray-500 dark:text-gray-400">Running analysis...</span>
                  </Show>
                </div>

                <Show when={!aiLoading() && !aiConfigured()}>
                  <div class="text-xs text-amber-600 dark:text-amber-400">
                    Pulse Assistant is not configured. Configure it in Settings → AI.
                  </div>
                </Show>

                <Show when={analysisError()}>
                  <div class="text-xs text-red-600 dark:text-red-400">
                    {analysisError()}
                  </div>
                </Show>

                <Show when={analysisResult()}>
                  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2">
                    <Show when={analysisMeta()}>
                      <div class="text-[11px] text-gray-500 dark:text-gray-400 mb-2">
                        Model: {analysisMeta()!.model} · Tokens: {analysisMeta()!.inputTokens + analysisMeta()!.outputTokens}
                      </div>
                    </Show>
                    <div class="text-sm text-gray-700 dark:text-gray-200 whitespace-pre-wrap">
                      {analysisResult()}
                    </div>
                  </div>
                </Show>
              </div>
            </Show>
          </div>
        </Card>
      </Show>
      {/* Filter Bar */}
      <Card padding="sm">
        <div class="flex flex-col gap-3">
          {/* Search - full width on its own row */}
          <div class="relative">
            <input
              ref={searchInputRef}
              type="text"
              placeholder="Search clusters, nodes, pods..."
              value={search()}
              onInput={(e) => setSearch(e.currentTarget.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  setSearch('');
                  e.currentTarget.blur();
                }
              }}
              class="w-full pl-9 pr-8 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
            />
            <svg
              class="absolute left-3 top-2 h-4 w-4 text-gray-400 dark:text-gray-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <Show when={search()}>
              <button
                type="button"
                class="absolute right-2 top-1/2 -translate-y-1/2 transform text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                onClick={() => setSearch('')}
                aria-label="Clear search"
              >
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </Show>
          </div>

          {/* Filters - second row */}
          <div class="flex flex-wrap items-center gap-2">
            {/* View Mode Toggle */}
            <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <button
                type="button"
                onClick={() => setViewMode('clusters')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${viewMode() === 'clusters'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Clusters
              </button>
              <button
                type="button"
                onClick={() => setViewMode('nodes')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${viewMode() === 'nodes'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Nodes
              </button>
              <button
                type="button"
                onClick={() => setViewMode('pods')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${viewMode() === 'pods'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Pods
              </button>
              <button
                type="button"
                onClick={() => setViewMode('deployments')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${viewMode() === 'deployments'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                Deployments
              </button>
            </div>

            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block" />

            {/* Status Filter */}
            <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <button
                type="button"
                onClick={() => setStatusFilter('all')}
                class={`px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${statusFilter() === 'all'
                  ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                All
              </button>
              <button
                type="button"
                onClick={() => setStatusFilter(statusFilter() === 'healthy' ? 'all' : 'healthy')}
                class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${statusFilter() === 'healthy'
                  ? 'bg-white dark:bg-gray-800 text-green-600 dark:text-green-400 shadow-sm ring-1 ring-green-200 dark:ring-green-800'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                <span class={`w-2 h-2 rounded-full ${statusFilter() === 'healthy' ? 'bg-green-500' : 'bg-green-400/60'}`} />
                Healthy
              </button>
              <button
                type="button"
                onClick={() => setStatusFilter(statusFilter() === 'unhealthy' ? 'all' : 'unhealthy')}
                class={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95 ${statusFilter() === 'unhealthy'
                  ? 'bg-white dark:bg-gray-800 text-amber-600 dark:text-amber-400 shadow-sm ring-1 ring-amber-200 dark:ring-amber-800'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
              >
                <span class={`w-2 h-2 rounded-full ${statusFilter() === 'unhealthy' ? 'bg-amber-500' : 'bg-amber-400/60'}`} />
                Unhealthy
              </button>
            </div>

            {/* Namespace Filter - only show for pods/deployments */}
            <Show when={(viewMode() === 'pods' || viewMode() === 'deployments') && allNamespaces().length > 1}>
              <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block" />
              <select
                value={namespaceFilter()}
                onChange={(e) => setNamespaceFilter(e.currentTarget.value)}
                class="px-2.5 py-1 text-xs font-medium rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
              >
                <option value="all">All namespaces</option>
                <For each={allNamespaces()}>
                  {(ns) => <option value={ns}>{ns}</option>}
                </For>
              </select>
            </Show>

            <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block" />

            {/* Show Hidden Toggle */}
            <label class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={showHidden()}
                onChange={(e) => setShowHidden(e.currentTarget.checked)}
                class="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500/20"
              />
              Show hidden
            </label>

            {/* Column Picker - only show for pods view */}
            <Show when={viewMode() === 'pods'}>
              <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block" />
              <ColumnPicker
                columns={podColumns.availableToggles()}
                isHidden={podColumns.isHiddenByUser}
                onToggle={podColumns.toggle}
                onReset={podColumns.resetToDefaults}
              />
            </Show>

            {/* Reset Button */}
            <Show when={hasActiveFilters()}>
              <div class="h-5 w-px bg-gray-200 dark:bg-gray-600 hidden sm:block" />
              <button
                type="button"
                onClick={handleReset}
                class="flex items-center justify-center gap-1 px-2.5 py-1 text-xs font-medium rounded-lg text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900/50 hover:bg-blue-200 dark:hover:bg-blue-900/70 transition-colors"
                title="Reset filters"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
                  <path d="M21 3v5h-5" />
                  <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
                  <path d="M8 16H3v5" />
                </svg>
                <span class="hidden sm:inline">Reset</span>
              </button>
            </Show>
          </div>
        </div>
      </Card>

      {/* Main Table */}
      <Card padding="none" class="overflow-hidden">
        <Show
          when={!isEmpty()}
          fallback={
            <div class="p-10">
              <EmptyState
                title="No Kubernetes clusters reporting yet"
                description="Enable Kubernetes monitoring on a unified agent to start reporting cluster health."
              />
            </div>
          }
        >
          {/* Clusters View */}
          <Show when={viewMode() === 'clusters'}>
            <ScrollableTable minWidth="900px" persistKey="kubernetes-clusters-table">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-900/40">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('name')}>Cluster{sortIndicator('name')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('status')}>Status{sortIndicator('status')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Nodes</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Pods</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Deployments</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Version</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Last Seen</th>
                  </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={visibleClusters()} fallback={
                    <tr><td colSpan={7} class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">No clusters match the current filters.</td></tr>
                  }>
                    {(cluster) => {
                      const indicator = () => getStatusIndicator(cluster.status);
                      const nodes = () => summarizeNodes(cluster.nodes);
                      const pods = () => summarizePods(cluster.pods);
                      const deployments = () => summarizeDeployments(cluster.deployments);

                      return (
                        <tr class="hover:bg-gray-50 dark:hover:bg-gray-900/20">
                          <td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">
                            <div class="flex items-center gap-2">
                              <span class="font-medium">{getClusterDisplayName(cluster)}</span>
                              <Show when={cluster.pendingUninstall}>
                                <span class="text-[10px] px-2 py-0.5 rounded-full bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200">
                                  Pending uninstall
                                </span>
                              </Show>
                              <Show when={cluster.hidden}>
                                <span class="text-[10px] px-2 py-0.5 rounded-full bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200">
                                  Hidden
                                </span>
                              </Show>
                            </div>
                            <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate max-w-xs font-mono">
                              {cluster.server || '—'}
                            </div>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            <div class="flex items-center gap-2">
                              <StatusDot variant={indicator().variant} size="sm" />
                              <span>{indicator().label}</span>
                            </div>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            <span class={nodes().notReady > 0 ? 'text-amber-600 dark:text-amber-400' : ''}>{nodes().total - nodes().notReady}</span>
                            <span class="text-gray-400">/{nodes().total}</span>
                            <Show when={nodes().notReady > 0}>
                              <span class="ml-1 text-xs text-gray-400">ready</span>
                            </Show>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            <span class={pods().unhealthy > 0 ? 'text-amber-600 dark:text-amber-400' : ''}>{pods().total - pods().unhealthy}</span>
                            <span class="text-gray-400">/{pods().total}</span>
                            <Show when={pods().unhealthy > 0}>
                              <span class="ml-1 text-xs text-gray-400">healthy</span>
                            </Show>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            <span class={deployments().unhealthy > 0 ? 'text-amber-600 dark:text-amber-400' : ''}>{deployments().total - deployments().unhealthy}</span>
                            <span class="text-gray-400">/{deployments().total}</span>
                            <Show when={deployments().unhealthy > 0}>
                              <span class="ml-1 text-xs text-gray-400">ok</span>
                            </Show>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 font-mono">
                            {cluster.version || '—'}
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            {cluster.lastSeen ? formatRelativeTime(cluster.lastSeen) : '—'}
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </ScrollableTable>
          </Show>

          {/* Nodes View */}
          <Show when={viewMode() === 'nodes'}>
            <ScrollableTable minWidth="1000px" persistKey="kubernetes-nodes-table">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-900/40">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('name')}>Node{sortIndicator('name')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('cluster')}>Cluster{sortIndicator('cluster')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('status')}>Status{sortIndicator('status')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Roles</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">CPU</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Memory</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Pods</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Version</th>
                  </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={filteredNodes()} fallback={
                    <tr><td colSpan={8} class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">No nodes match the current filters.</td></tr>
                  }>
                    {({ cluster, node }) => {
                      const isHealthy = () => node.ready && !node.unschedulable;
                      const roles = () => (node.roles ?? []).join(', ') || 'worker';

                      return (
                        <tr class="hover:bg-gray-50 dark:hover:bg-gray-900/20">
                          <td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">
                            <div class="font-medium">{node.name}</div>
                            <div class="text-xs text-gray-500 dark:text-gray-400 truncate max-w-xs">
                              {node.osImage || '—'}
                            </div>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            {getClusterDisplayName(cluster)}
                          </td>
                          <td class="px-4 py-3 text-sm">
                            <Show when={isHealthy()} fallback={
                              <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                                <span class="w-1.5 h-1.5 rounded-full bg-amber-500" />
                                {!node.ready ? 'NotReady' : 'Unschedulable'}
                              </span>
                            }>
                              <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300">
                                <span class="w-1.5 h-1.5 rounded-full bg-green-500" />
                                Ready
                              </span>
                            </Show>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            <span class="px-2 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-xs">
                              {roles()}
                            </span>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 font-mono">
                            {node.allocatableCpuCores ?? node.capacityCpuCores ?? '—'} cores
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 font-mono">
                            {node.allocatableMemoryBytes ? formatBytes(node.allocatableMemoryBytes) : node.capacityMemoryBytes ? formatBytes(node.capacityMemoryBytes) : '—'}
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 font-mono">
                            {node.allocatablePods ?? node.capacityPods ?? '—'}
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 font-mono">
                            {node.kubeletVersion || '—'}
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </ScrollableTable>
          </Show>

          {/* Pods View */}
          <Show when={viewMode() === 'pods'}>
            <ScrollableTable minWidth="800px" persistKey="kubernetes-pods-table">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-900/40">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('name')}>Pod{sortIndicator('name')}</th>
                    <Show when={podColumns.isColumnVisible('namespace')}>
                      <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('namespace')}>Namespace{sortIndicator('namespace')}</th>
                    </Show>
                    <Show when={podColumns.isColumnVisible('cluster')}>
                      <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('cluster')}>Cluster{sortIndicator('cluster')}</th>
                    </Show>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('status')}>Status{sortIndicator('status')}</th>
                    <Show when={podColumns.isColumnVisible('ready')}>
                      <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('ready')}>Ready{sortIndicator('ready')}</th>
                    </Show>
                    <Show when={podColumns.isColumnVisible('restarts')}>
                      <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('restarts')}>Restarts{sortIndicator('restarts')}</th>
                    </Show>
                    <Show when={podColumns.isColumnVisible('image')}>
                      <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Image</th>
                    </Show>
                    <Show when={podColumns.isColumnVisible('age')}>
                      <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('age')}>Age{sortIndicator('age')}</th>
                    </Show>
                  </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={filteredPods()} fallback={
                    <tr><td colSpan={8} class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">No pods match the current filters.</td></tr>
                  }>
                    {({ cluster, pod }) => (
                      <PodRow
                        cluster={cluster}
                        pod={pod}
                        columns={podColumns}
                        guestMetadata={guestMetadata}
                        onCustomUrlChange={handleK8sCustomUrlChange}
                      />
                    )}
                  </For>
                </tbody>
              </table>
            </ScrollableTable>
          </Show>

          {/* Deployments View */}
          <Show when={viewMode() === 'deployments'}>
            <ScrollableTable minWidth="800px" persistKey="kubernetes-deployments-table">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-900/40">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('name')}>Deployment{sortIndicator('name')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('namespace')}>Namespace{sortIndicator('namespace')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('cluster')}>Cluster{sortIndicator('cluster')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('status')}>Status{sortIndicator('status')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('replicas')}>Replicas{sortIndicator('replicas')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-700 dark:hover:text-gray-200" onClick={() => toggleSort('ready')}>Ready{sortIndicator('ready')}</th>
                    <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Up-to-date</th>
                  </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={filteredDeployments()} fallback={
                    <tr><td colSpan={7} class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">No deployments match the current filters.</td></tr>
                  }>
                    {({ cluster, deployment }) => {
                      const healthy = () => isDeploymentHealthy(deployment);
                      const desired = () => deployment.desiredReplicas ?? 0;
                      const ready = () => deployment.readyReplicas ?? 0;
                      const updated = () => deployment.updatedReplicas ?? 0;

                      return (
                        <tr class="hover:bg-gray-50 dark:hover:bg-gray-900/20">
                          <td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-100">
                            <div class="font-medium">{deployment.name}</div>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            <span class="px-2 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-xs font-mono">
                              {deployment.namespace}
                            </span>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                            {getClusterDisplayName(cluster)}
                          </td>
                          <td class="px-4 py-3 text-sm">
                            <Show when={healthy()} fallback={
                              <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                                <span class="w-1.5 h-1.5 rounded-full bg-amber-500" />
                                Progressing
                              </span>
                            }>
                              <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300">
                                <span class="w-1.5 h-1.5 rounded-full bg-green-500" />
                                Available
                              </span>
                            </Show>
                          </td>
                          <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300 font-medium">
                            {desired()}
                          </td>
                          <td class="px-4 py-3 text-sm">
                            <span class={ready() >= desired() ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'}>
                              {ready()}/{desired()}
                            </span>
                          </td>
                          <td class="px-4 py-3 text-sm">
                            <span class={updated() >= desired() ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'}>
                              {updated()}/{desired()}
                            </span>
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </ScrollableTable>
          </Show>
        </Show>
      </Card>
    </div>
  );
};
