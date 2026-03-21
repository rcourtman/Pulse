import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import type { VM, Container, Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { GUEST_COLUMNS, VIEW_MODE_COLUMNS } from './guestRowModel';
import { useWebSocket } from '@/App';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { getNodeDisplayName } from '@/utils/nodes';
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
} from '@/utils/workloads';
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import { parseWorkloadsLinkSearch } from '@/routing/resourceLinks';
import {
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
import { useDashboardWorkloadRouteState } from './useDashboardWorkloadRouteState';
import type { WorkloadSummarySnapshot } from '@/components/Workloads/WorkloadsSummary';

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
  const location = useLocation();
  const navigate = useNavigate();
  const ws = useWebSocket();
  const { connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = ws;
  const { isMobile } = useBreakpoint();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const [search, setSearch] = createSignal('');

  const kioskMode = useKioskMode();
  const dashboardInfrastructureEmptyState = createMemo(() => getDashboardInfrastructureEmptyState());
  const dashboardGuestsEmptyState = createMemo(() => getDashboardGuestsEmptyState(search()));
  const dashboardLoadingState = createMemo(() => getDashboardLoadingState(reconnecting()));
  const dashboardDisconnectedState = createMemo(() => getDashboardDisconnectedState(reconnecting()));

  const [isSearchLocked, setIsSearchLocked] = createSignal(false);
  const [selectedGuestId, setSelectedGuestIdRaw] = createSignal<string | null>(null);
  const [hoveredWorkloadId, setHoveredWorkloadId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);

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

  const {
    containerRuntime,
    containerRuntimeFilterConfig,
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
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setViewMode,
    viewMode,
    workloadNodeOptions,
    containerRuntimeOptions,
  } = useDashboardWorkloadRouteState({
    allGuests,
    showFilters,
    setShowFilters,
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
          resetWorkloadRouteFilters();
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
