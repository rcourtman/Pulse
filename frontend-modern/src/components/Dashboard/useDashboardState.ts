import { createEffect, createMemo, createSignal, onCleanup, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import type { VM, Container, Node } from '@/types/api';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { GUEST_COLUMNS, VIEW_MODE_COLUMNS } from './guestRowModel';
import { useWebSocket } from '@/App';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { getNodeDisplayName } from '@/utils/nodes';
import { logger } from '@/utils/logger';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { blurFocusedTypeToSearch } from '@/hooks/useTypeToSearch';
import { useWorkloads } from '@/hooks/useWorkloads';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { aiChatStore } from '@/stores/aiChat';
import { useKioskMode } from '@/hooks/useKioskMode';
import {
  getDashboardDisconnectedState,
  getDashboardGuestsEmptyState,
  getDashboardInfrastructureEmptyState,
  getDashboardLoadingState,
} from '@/utils/dashboardEmptyStatePresentation';
import {
  getCanonicalWorkloadId,
  normalizeWorkloadViewModeParam,
  resolveWorkloadType,
} from '@/utils/workloads';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import {
  buildWorkloadsPath,
  parseWorkloadsLinkSearch,
  WORKLOADS_PATH,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import { areSearchParamsEquivalent } from '@/utils/searchParams';
import {
  workloadNodeScopeId,
  getKubernetesContextKey,
  filterWorkloads,
  getDiskUsagePercent,
  createWorkloadSortComparator,
  getWorkloadGroupKey,
  groupWorkloads,
  computeWorkloadStats,
  computeWorkloadIOEmphasis,
  buildNodeByInstance,
  buildGuestParentNodeMap,
  type FilterWorkloadsParams,
  type WorkloadStats,
} from './workloadSelectors';
import { useGroupedTableWindowing } from './useGroupedTableWindowing';
import { useDashboardGuestMetadataState } from './useDashboardGuestMetadataState';
import type { WorkloadSummarySnapshot } from '@/components/Workloads/WorkloadsSummary';
import type { DashboardToolbarFilterConfig } from './dashboardFilterModel';

export interface DashboardProps {
  vms: VM[];
  containers: Container[];
  nodes: Node[];
  useWorkloads?: boolean;
}

type StatusMode = 'all' | 'running' | 'degraded' | 'stopped';
type GroupingMode = 'grouped' | 'flat';
export type WorkloadSortKey = keyof WorkloadGuest | 'diskIo' | 'netIo';

const DASHBOARD_TABLE_ESTIMATED_ROW_HEIGHT = 32;

const workloadMetricPercent = (value: number | null | undefined): number => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0;
  if (value <= 1) return Math.max(0, value * 100);
  return Math.max(0, value);
};

export function useDashboardState(props: DashboardProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const ws = useWebSocket();
  const { connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = ws;
  const { isMobile } = useBreakpoint();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const isWorkloadsRoute = () => location.pathname === WORKLOADS_PATH;
  const [search, setSearch] = createSignal('');

  const kioskMode = useKioskMode();
  const dashboardInfrastructureEmptyState = createMemo(() => getDashboardInfrastructureEmptyState());
  const dashboardGuestsEmptyState = createMemo(() => getDashboardGuestsEmptyState(search()));
  const dashboardLoadingState = createMemo(() => getDashboardLoadingState(reconnecting()));
  const dashboardDisconnectedState = createMemo(() => getDashboardDisconnectedState(reconnecting()));

  const [isSearchLocked, setIsSearchLocked] = createSignal(false);
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const [selectedKubernetesContext, setSelectedKubernetesContext] = createSignal<string | null>(
    null,
  );
  const [selectedKubernetesNamespace, setSelectedKubernetesNamespace] = createSignal<
    string | null
  >(null);
  const [selectedGuestId, setSelectedGuestIdRaw] = createSignal<string | null>(null);
  const [hoveredWorkloadId, setHoveredWorkloadId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  const [handledTypeParam, setHandledTypeParam] = createSignal<string>('');
  const [handledRuntimeParam, setHandledRuntimeParam] = createSignal<string>('');
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

  let tableRef: HTMLDivElement | undefined;
  const [tableBodyRef, setTableBodyRef] = createSignal<HTMLTableSectionElement | null>(null);
  const setTableWrapperRef = (element: HTMLDivElement | undefined) => {
    tableRef = element;
  };

  const setSelectedGuestId = (id: string | null) => {
    let scroller: HTMLElement | null = tableRef ?? null;
    while (scroller) {
      const { overflowY } = getComputedStyle(scroller);
      if (
        (overflowY === 'auto' || overflowY === 'scroll') &&
        scroller.scrollHeight > scroller.clientHeight
      ) {
        break;
      }
      scroller = scroller.parentElement;
    }
    const scrollTop = scroller?.scrollTop ?? 0;
    setSelectedGuestIdRaw(id);
    if (scroller) scroller.scrollTop = scrollTop;
    requestAnimationFrame(() => {
      if (scroller) scroller.scrollTop = scrollTop;
    });
  };

  createEffect(() => {
    const { resource: resourceId } = parseWorkloadsLinkSearch(location.search);
    if (!resourceId || resourceId === handledResourceId()) return;
    setSelectedGuestId(resourceId);
    const [instance, node, vmid] = resourceId.split(':');
    if (instance && node && vmid) {
      setSelectedNode(`${instance}-${node}`);
    }
    setHandledResourceId(resourceId);
  });

  const { guestMetadata, handleCustomUrlUpdate } = useDashboardGuestMetadataState();

  const workloadsEnabled = createMemo(() => props.useWorkloads === true);
  const workloads = useWorkloads(workloadsEnabled);

  const dedupeGuests = (guests: WorkloadGuest[]): WorkloadGuest[] => {
    const seen = new Set<string>();
    const deduped: WorkloadGuest[] = [];
    for (const guest of guests) {
      const canonicalId = getCanonicalWorkloadId(guest);
      if (seen.has(canonicalId)) continue;
      seen.add(canonicalId);
      deduped.push(guest);
    }
    return deduped;
  };

  const allGuests = createMemo<WorkloadGuest[]>(() =>
    workloadsEnabled() ? dedupeGuests(workloads.workloads()) : [],
  );

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

  const [statusMode, setStatusMode] = usePersistentSignal<StatusMode>(
    'dashboardStatusMode',
    'all',
    {
      deserialize: (raw) =>
        raw === 'all' || raw === 'running' || raw === 'degraded' || raw === 'stopped'
          ? (raw as StatusMode)
          : 'all',
    },
  );

  const [groupingMode, setGroupingMode] = usePersistentSignal<GroupingMode>(
    'dashboardGroupingMode',
    'grouped',
    {
      deserialize: (raw) => (raw === 'grouped' || raw === 'flat' ? raw : 'grouped'),
    },
  );

  const [showFilters, setShowFilters] = usePersistentSignal<boolean>(
    'dashboardShowFilters',
    false,
    {
      deserialize: (raw) => raw === 'true',
      serialize: (value) => String(value),
    },
  );
  const [workloadsSummaryRange, setWorkloadsSummaryRange] = usePersistentSignal(
    STORAGE_KEYS.WORKLOADS_SUMMARY_RANGE,
    '1h',
    {
      deserialize: (raw) => (isSummaryTimeRange(raw) ? raw : '1h'),
    },
  );
  const [workloadsSummaryCollapsed, setWorkloadsSummaryCollapsed] = usePersistentSignal<boolean>(
    STORAGE_KEYS.WORKLOADS_SUMMARY_COLLAPSED,
    false,
    { deserialize: (raw) => raw === 'true' },
  );

  const [sortKey, setSortKey] = createSignal<WorkloadSortKey | null>('type');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  const workloadNodeOptions = createMemo(() => {
    const labelsByScope = new Map<string, string>();
    const nodeNameCounts = new Map<string, number>();

    for (const guest of allGuests()) {
      const type = resolveWorkloadType(guest);
      if (type === 'pod') continue;
      const scope = workloadNodeScopeId(guest);
      if (!scope || scope === '-') continue;
      const nodeName = (guest.node || '').trim();
      if (!nodeName) continue;
      nodeNameCounts.set(nodeName, (nodeNameCounts.get(nodeName) || 0) + 1);
    }

    for (const guest of allGuests()) {
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
    for (const guest of allGuests()) {
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
    for (const guest of allGuests()) {
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
    for (const guest of allGuests()) {
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
      if (!showFilters()) {
        setShowFilters(true);
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
      if (!showFilters()) {
        setShowFilters(true);
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
      if (!showFilters()) {
        setShowFilters(true);
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
    if (!showFilters()) {
      setShowFilters(true);
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
    if (urlResource && handledResourceId() !== urlResource) return;

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

  const relevantColumns = createMemo(() => {
    const base = VIEW_MODE_COLUMNS[viewMode()];
    if (!base) return null;
    if (groupingMode() === 'grouped' && base.has('node')) {
      const filtered = new Set(base);
      filtered.delete('node');
      return filtered;
    }
    return base;
  });
  const columnVisibility = useColumnVisibility(
    STORAGE_KEYS.DASHBOARD_HIDDEN_COLUMNS,
    GUEST_COLUMNS,
    ['os', 'ip'],
    relevantColumns,
  );
  const visibleColumns = columnVisibility.visibleColumns;
  const visibleColumnIds = createMemo(() => visibleColumns().map((column) => column.id));
  const mobileEssentialColumns = new Set(['name', 'cpu', 'memory', 'disk', 'link']);
  const mobileVisibleColumns = createMemo(() =>
    isMobile() ? visibleColumns().filter((column) => mobileEssentialColumns.has(column.id)) : visibleColumns(),
  );
  const mobileVisibleColumnIds = createMemo(() =>
    isMobile() ? mobileVisibleColumns().map((column) => column.id) : visibleColumnIds(),
  );
  const totalColumns = createMemo(() => mobileVisibleColumns().length);

  let lastConnected = connected();
  createEffect(() => {
    const isConnected = connected();
    if (workloadsEnabled() && isConnected && !lastConnected) {
      void workloads.refetch();
    }
    lastConnected = isConnected;
  });

  const nodeByInstance = createMemo(() => buildNodeByInstance(props.nodes));
  const guestParentNodeMap = createMemo(() =>
    buildGuestParentNodeMap(allGuests(), nodeByInstance()),
  );

  const handleSort = (key: WorkloadSortKey) => {
    if (sortKey() === key) {
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setSortKey(key);
      if (
        key === 'cpu' ||
        key === 'memory' ||
        key === 'disk' ||
        key === 'diskIo' ||
        key === 'netIo' ||
        key === 'uptime'
      ) {
        setSortDirection('desc');
      } else {
        setSortDirection('asc');
      }
    }
  };

  const guestSortComparator = createMemo(() =>
    createWorkloadSortComparator(sortKey() || '', sortDirection()),
  );

  createEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        const hasActiveFilters =
          search().trim() ||
          sortKey() !== 'type' ||
          sortDirection() !== 'asc' ||
          selectedNode() !== null ||
          selectedHostHint() !== null ||
          selectedKubernetesContext() !== null ||
          selectedKubernetesNamespace() !== null ||
          containerRuntime().trim() !== '' ||
          viewMode() !== 'all' ||
          statusMode() !== 'all';

        if (hasActiveFilters) {
          setSearch('');
          setIsSearchLocked(false);
          setSortKey('type');
          setSortDirection('asc');
          setSelectedNode(null);
          setSelectedHostHint(null);
          setSelectedKubernetesContext(null);
          setSelectedKubernetesNamespace(null);
          setContainerRuntime('');
          setViewMode('all');
          setStatusMode('all');

          blurFocusedTypeToSearch();
        } else {
          setShowFilters(!showFilters());
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  });

  const filteredGuests = createMemo(() => {
    const params: FilterWorkloadsParams = {
      guests: allGuests(),
      viewMode: viewMode(),
      statusMode: statusMode(),
      searchTerm: search().trim(),
      selectedNode: selectedNode(),
      selectedHostHint: selectedHostHint(),
      selectedKubernetesContext: selectedKubernetesContext(),
      selectedKubernetesNamespace: selectedKubernetesNamespace(),
      containerRuntime: containerRuntime().trim() || null,
    };
    return filterWorkloads(params);
  });

  const workloadsSummaryVisibleIds = createMemo<string[]>(() =>
    filteredGuests().map((guest) => getCanonicalWorkloadId(guest)),
  );

  const workloadsSummaryFallbackCounts = createMemo(() => {
    const guests = filteredGuests();
    const running = guests.filter(
      (guest) => guest.status === 'running' || guest.status === 'online',
    ).length;
    return {
      total: guests.length,
      running,
      stopped: Math.max(0, guests.length - running),
    };
  });

  const workloadsSummaryFallbackSnapshots = createMemo<WorkloadSummarySnapshot[]>(() =>
    filteredGuests().map((guest) => {
      const guestId = getCanonicalWorkloadId(guest);
      const memoryUsage = workloadMetricPercent(guest.memory?.usage);
      let diskUsage = workloadMetricPercent(guest.disk?.usage);
      if (
        (!diskUsage || diskUsage <= 0) &&
        typeof guest.disk?.used === 'number' &&
        typeof guest.disk?.total === 'number' &&
        Number.isFinite(guest.disk.used) &&
        Number.isFinite(guest.disk.total) &&
        guest.disk.total > 0
      ) {
        const selectorDiskUsage = getDiskUsagePercent(guest);
        const rawDiskUsage = (guest.disk.used / guest.disk.total) * 100;
        diskUsage = rawDiskUsage > 100 ? rawDiskUsage : (selectorDiskUsage ?? rawDiskUsage);
      }

      return {
        id: guestId,
        name: guest.name || guestId,
        cpu: workloadMetricPercent(guest.cpu),
        memory: memoryUsage,
        disk: Math.max(0, diskUsage),
        network: Math.max(0, guest.networkIn || 0) + Math.max(0, guest.networkOut || 0),
      };
    }),
  );

  createEffect(() => {
    const hoveredId = hoveredWorkloadId();
    if (!hoveredId) return;
    const exists = filteredGuests().some((guest) => getCanonicalWorkloadId(guest) === hoveredId);
    if (!exists) {
      setHoveredWorkloadId(null);
    }
  });

  const getGroupLabel = (
    groupKey: string,
    guests: WorkloadGuest[],
  ): { type: string; name: string } => {
    const node = nodeByInstance()[groupKey];
    if (node) return { type: '', name: getNodeDisplayName(node) };
    const normalizedGroupKey = guests.length > 0 ? getWorkloadGroupKey(guests[0]) : groupKey;
    const [prefix, ...rest] = normalizedGroupKey.split(':');
    const context = rest.length > 0 ? rest.join(':') : normalizedGroupKey;
    if (prefix === 'app-container') return { type: 'App Containers', name: context };
    if (prefix === 'pod') return { type: 'Pods', name: context };
    if (prefix === 'vm') return { type: 'VM', name: context };
    if (prefix === 'system-container') return { type: 'Container', name: context };
    const first = guests[0];
    if (first) {
      const cluster = (first.clusterName || '').trim();
      const nodeName = (first.node || '').trim();
      if (nodeName && cluster) return { type: cluster, name: nodeName };
      if (nodeName) return { type: '', name: nodeName };
    }
    return { type: '', name: context };
  };

  const groupedGuests = createMemo(() =>
    groupWorkloads(filteredGuests(), groupingMode(), guestSortComparator()),
  );

  const sortedGroupKeys = createMemo(() => {
    const groups = groupedGuests();
    const nodes = nodeByInstance();
    return Object.keys(groups).sort((a, b) => {
      const nodeA = nodes[a];
      const nodeB = nodes[b];
      const labelA = nodeA ? getNodeDisplayName(nodeA) : getGroupLabel(a, groups[a]).name;
      const labelB = nodeB ? getNodeDisplayName(nodeB) : getGroupLabel(b, groups[b]).name;
      return labelA.localeCompare(labelB) || a.localeCompare(b);
    });
  });

  const guestGlobalIndexById = createMemo(() => {
    const indexById = new Map<string, number>();
    const groups = groupedGuests();
    let globalIndex = 0;

    for (const groupKey of sortedGroupKeys()) {
      const guests = groups[groupKey] || [];
      for (const guest of guests) {
        indexById.set(getCanonicalWorkloadId(guest), globalIndex);
        globalIndex += 1;
      }
    }

    return indexById;
  });

  const revealGuestIndex = createMemo<number | null>(() => {
    const selectedId = selectedGuestId();
    if (!selectedId) return null;
    return guestGlobalIndexById().get(selectedId) ?? null;
  });

  const groupedWindowing = useGroupedTableWindowing({
    totalRowCount: () => filteredGuests().length,
    revealIndex: revealGuestIndex,
  });

  const groupStartIndexByKey = createMemo(() => {
    const starts = new Map<string, number>();
    const groups = groupedGuests();
    let globalIndex = 0;

    for (const groupKey of sortedGroupKeys()) {
      starts.set(groupKey, globalIndex);
      globalIndex += (groups[groupKey] || []).length;
    }

    return starts;
  });

  const windowedGroupedGuests = createMemo<Record<string, WorkloadGuest[]>>(() => {
    const groups = groupedGuests();
    if (!groupedWindowing.isWindowed()) {
      return groups;
    }

    const starts = groupStartIndexByKey();
    const result: Record<string, WorkloadGuest[]> = {};
    for (const groupKey of sortedGroupKeys()) {
      const guests = groups[groupKey] || [];
      const groupStart = starts.get(groupKey) ?? 0;
      const visible = groupedWindowing.getVisibleSlice(groupKey, guests, groupStart);
      if (visible.length > 0) {
        result[groupKey] = visible;
      }
    }

    return result;
  });

  const visibleGroupKeys = createMemo(() => {
    const keys = sortedGroupKeys();
    if (!groupedWindowing.isWindowed()) return keys;
    const groups = windowedGroupedGuests();
    return keys.filter((groupKey) => (groups[groupKey] || []).length > 0);
  });

  const topSpacerHeight = createMemo(() =>
    groupedWindowing.isWindowed()
      ? groupedWindowing.startIndex() * DASHBOARD_TABLE_ESTIMATED_ROW_HEIGHT
      : 0,
  );

  const bottomSpacerHeight = createMemo(() =>
    groupedWindowing.isWindowed()
      ? Math.max(
          0,
          (filteredGuests().length - groupedWindowing.endIndex()) *
            DASHBOARD_TABLE_ESTIMATED_ROW_HEIGHT,
        )
      : 0,
  );

  const syncGuestWindowToViewport = () => {
    if (!groupedWindowing.isWindowed() || typeof window === 'undefined') return;
    const body = tableBodyRef();
    if (!body) return;
    const rect = body.getBoundingClientRect();
    const scrollTop = Math.max(0, -rect.top);
    groupedWindowing.onScroll(scrollTop, window.innerHeight, DASHBOARD_TABLE_ESTIMATED_ROW_HEIGHT);
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    filteredGuests().length;
    if (!groupedWindowing.isWindowed()) return;
    if (!tableBodyRef()) return;

    const handleViewportChange = () => {
      syncGuestWindowToViewport();
    };

    handleViewportChange();
    window.addEventListener('scroll', handleViewportChange, { passive: true });
    window.addEventListener('resize', handleViewportChange);
    onCleanup(() => {
      window.removeEventListener('scroll', handleViewportChange);
      window.removeEventListener('resize', handleViewportChange);
    });
  });

  const totalStats = createMemo<WorkloadStats>(() => computeWorkloadStats(filteredGuests()));
  const workloadIOEmphasis = createMemo(() => computeWorkloadIOEmphasis(filteredGuests()));

  const handleNodeSelect = (nodeId: string | null, nodeType: 'pve' | 'pbs' | 'pmg' | null) => {
    logger.debug('handleNodeSelect called', { nodeId, nodeType });

    if (nodeType === 'pve' || nodeType === null) {
      setSelectedHostHint(null);
      setSelectedNode(nodeId);
      logger.debug('Set selected node', { nodeId });
      if (nodeId && !showFilters()) {
        setShowFilters(true);
      }
    }
  };

  const handleBeforeAutoFocus = () => {
    if (aiChatStore.focusInput()) return true;
    if (!showFilters()) setShowFilters(true);
    return false;
  };

  const handleTagClick = (tag: string) => {
    const currentSearch = search().trim();
    const tagFilter = `tags:${tag}`;

    if (currentSearch.includes(tagFilter)) {
      let newSearch = currentSearch;

      if (currentSearch === tagFilter) {
        newSearch = '';
      } else if (currentSearch.startsWith(tagFilter + ',')) {
        newSearch = currentSearch.replace(tagFilter + ',', '').trim();
      } else if (currentSearch.endsWith(', ' + tagFilter)) {
        newSearch = currentSearch.replace(', ' + tagFilter, '').trim();
      } else if (currentSearch.includes(', ' + tagFilter + ',')) {
        newSearch = currentSearch.replace(', ' + tagFilter + ',', ',').trim();
      } else if (currentSearch.includes(tagFilter + ', ')) {
        newSearch = currentSearch.replace(tagFilter + ', ', '').trim();
      }

      setSearch(newSearch);
      if (!newSearch) {
        setIsSearchLocked(false);
      }
    } else {
      if (!currentSearch || isSearchLocked()) {
        setSearch(tagFilter);
        setIsSearchLocked(false);
      } else {
        setSearch(`${currentSearch}, ${tagFilter}`);
      }

      if (!showFilters()) {
        setShowFilters(true);
      }
    }
  };

  const dashboardFilterColumnVisibility = createMemo(() => ({
    availableColumns: columnVisibility.availableToggles(),
    isColumnHidden: columnVisibility.isHiddenByUser,
    onColumnToggle: columnVisibility.toggle,
    onColumnReset: columnVisibility.resetToDefaults,
  }));

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
    activeAlerts,
    alertsEnabled,
    allGuests,
    bottomSpacerHeight,
    columnVisibility,
    connected,
    containerRuntime,
    containerRuntimeFilterConfig,
    containerRuntimeOptions,
    dashboardFilterColumnVisibility,
    dashboardDisconnectedState,
    dashboardGuestsEmptyState,
    dashboardInfrastructureEmptyState,
    dashboardLoadingState,
    filteredGuests,
    getGroupLabel,
    groupedGuests,
    groupedWindowing,
    guestMetadata,
    guestParentNodeMap,
    handleBeforeAutoFocus,
    handleCustomUrlUpdate,
    handleNodeSelect,
    handleSort,
    handleTagClick,
    hostFilterConfig,
    hoveredWorkloadId,
    initialDataReceived,
    isMobile,
    isSearchLocked,
    isWorkloadsRoute,
    kioskMode,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    mobileVisibleColumnIds,
    mobileVisibleColumns,
    navigate,
    nodeByInstance,
    namespaceFilterConfig,
    reconnect,
    search,
    selectedGuestId,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setGroupingMode,
    setHoveredWorkloadId,
    setSearch,
    setSelectedGuestId,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setSortDirection,
    setSortKey,
    setStatusMode,
    setTableBodyRef,
    setTableWrapperRef,
    setViewMode,
    setWorkloadsSummaryCollapsed,
    setWorkloadsSummaryRange,
    sortDirection,
    sortKey,
    statusMode,
    topSpacerHeight,
    totalColumns,
    totalStats,
    viewMode,
    visibleColumns,
    visibleGroupKeys,
    windowedGroupedGuests,
    workloadIOEmphasis,
    workloadNodeOptions,
    workloads,
    workloadsSummaryCollapsed,
    workloadsSummaryFallbackCounts,
    workloadsSummaryFallbackSnapshots,
    workloadsSummaryRange,
    workloadsSummaryVisibleIds,
    ws,
    groupingMode,
  } as const;
}
