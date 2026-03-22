import { createEffect, createMemo, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { VM, Container, Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { GUEST_COLUMNS, VIEW_MODE_COLUMNS } from './guestRowModel';
import { useWebSocket } from '@/App';
import { useAlertsActivation } from '@/stores/alertsActivation';
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
import { isSummaryTimeRange } from '@/components/shared/summaryTimeRange';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import {
  createWorkloadSortComparator,
  filterWorkloads,
  type FilterWorkloadsParams,
} from './workloadSelectors';
import { useDashboardGuestMetadataState } from './useDashboardGuestMetadataState';
import { useDashboardSelectionState } from './useDashboardSelectionState';
import { useDashboardWorkloadDerivedState } from './useDashboardWorkloadDerivedState';
import { useDashboardWorkloadRouteState } from './useDashboardWorkloadRouteState';

export interface DashboardProps {
  vms: VM[];
  containers: Container[];
  nodes: Node[];
  useWorkloads?: boolean;
}

type StatusMode = 'all' | 'running' | 'degraded' | 'stopped';
type GroupingMode = 'grouped' | 'flat';
export type WorkloadSortKey = keyof WorkloadGuest | 'diskIo' | 'netIo';

export function useDashboardState(props: DashboardProps) {
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
    setSelectedNode,
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

  const {
    hoveredWorkloadId,
    selectedGuestId,
    setHoveredWorkloadId,
    setSelectedGuestId,
    setTableBodyRef,
    setTableWrapperRef,
    tableBodyRef,
  } = useDashboardSelectionState({
    filteredGuests,
    setSelectedNode,
  });

  const {
    bottomSpacerHeight,
    getGroupLabel,
    groupedGuests,
    groupedWindowing,
    guestParentNodeMap,
    nodeByInstance,
    topSpacerHeight,
    totalStats,
    visibleGroupKeys,
    windowedGroupedGuests,
    workloadIOEmphasis,
    workloadsSummaryFallbackCounts,
    workloadsSummaryFallbackSnapshots,
    workloadsSummaryVisibleIds,
  } = useDashboardWorkloadDerivedState({
    allGuests,
    filteredGuests,
    groupingMode,
    guestSortComparator,
    nodes: () => props.nodes,
    selectedGuestId,
    tableBodyRef,
  });

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

export type DashboardState = ReturnType<typeof useDashboardState>;
