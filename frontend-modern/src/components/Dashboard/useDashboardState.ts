import { createEffect, createMemo } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { VM, Container, Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { useWebSocket } from '@/App';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useWorkloads } from '@/hooks/useWorkloads';
import { useKioskMode } from '@/hooks/useKioskMode';
import {
  getDashboardDisconnectedState,
  getDashboardGuestsEmptyState,
  getDashboardInfrastructureEmptyState,
  getDashboardLoadingState,
} from '@/utils/dashboardEmptyStatePresentation';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import {
  createWorkloadSortComparator,
  filterWorkloads,
  type FilterWorkloadsParams,
} from './workloadSelectors';
import {
  type DashboardSortKey,
} from './dashboardFilterModel';
import { useDashboardControlsState } from './useDashboardControlsState';
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

export type WorkloadSortKey = DashboardSortKey;

export function useDashboardState(props: DashboardProps) {
  const navigate = useNavigate();
  const ws = useWebSocket();
  const { connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = ws;
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');

  const kioskMode = useKioskMode();

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

  const [showFilters, setShowFilters] = usePersistentSignal<boolean>(
    'dashboardShowFilters',
    false,
    {
      deserialize: (raw) => raw === 'true',
      serialize: (value) => String(value),
    },
  );

  const {
    containerRuntime,
    containerRuntimeFilterConfig,
    handleNodeSelect,
    hostFilterConfig,
    isWorkloadsRoute,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    namespaceFilterConfig,
    platformFilterConfig,
    platformOptions,
    resetWorkloadRouteFilters,
    selectedHostHint,
    selectedPlatform,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setSelectedNode,
    setSelectedPlatform,
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

  const {
    columnVisibility,
    dashboardFilterColumnVisibility,
    groupingMode,
    handleBeforeAutoFocus,
    handleSort,
    handleTagClick,
    isMobile,
    isSearchLocked,
    mobileVisibleColumnIds,
    mobileVisibleColumns,
    search,
    setGroupingMode,
    setSearch,
    setSortDirection,
    setSortKey,
    setStatusMode,
    sortDirection,
    sortKey,
    statusMode,
    totalColumns,
    visibleColumns,
    workloadsSummaryCollapsed,
    workloadsSummaryRange,
    setWorkloadsSummaryCollapsed,
    setWorkloadsSummaryRange,
  } = useDashboardControlsState({
    containerRuntime,
    resetWorkloadRouteFilters,
    selectedHostHint,
    selectedPlatform,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setShowFilters,
    showFilters,
    viewMode,
  });

  const dashboardInfrastructureEmptyState = createMemo(() => getDashboardInfrastructureEmptyState());
  const dashboardGuestsEmptyState = createMemo(() => getDashboardGuestsEmptyState(search()));
  const dashboardLoadingState = createMemo(() => getDashboardLoadingState(reconnecting()));
  const dashboardDisconnectedState = createMemo(() => getDashboardDisconnectedState(reconnecting()));
  const hasWorkloadsData = createMemo(() => allGuests().length > 0);
  const surfaceConnected = createMemo(() =>
    workloadsEnabled()
      ? workloads.loading() || hasWorkloadsData() || !workloads.error()
      : connected(),
  );
  const surfaceInitialDataReceived = createMemo(() =>
    workloadsEnabled()
      ? hasWorkloadsData() || !workloads.loading() || Boolean(workloads.error())
      : initialDataReceived(),
  );

  const reconnectSurface = () => {
    if (workloadsEnabled()) {
      void workloads.refetch();
    }
    reconnect();
  };

  let lastConnected = connected();
  createEffect(() => {
    const isConnected = connected();
    if (workloadsEnabled() && isConnected && !lastConnected) {
      void workloads.refetch();
    }
    lastConnected = isConnected;
  });

  const guestSortComparator = createMemo(() =>
    createWorkloadSortComparator(sortKey() || '', sortDirection()),
  );

  const filteredGuests = createMemo(() => {
    const params: FilterWorkloadsParams = {
      guests: allGuests(),
      viewMode: viewMode(),
      statusMode: statusMode(),
      searchTerm: search().trim(),
      selectedNode: selectedNode(),
      selectedHostHint: selectedHostHint(),
      selectedPlatform: selectedPlatform(),
      selectedKubernetesContext: selectedKubernetesContext(),
      selectedKubernetesNamespace: selectedKubernetesNamespace(),
      containerRuntime: containerRuntime().trim() || null,
    };
    return filterWorkloads(params);
  });

  const {
    activeSummaryWorkloadId,
    chartHoverSync,
    hoveredWorkloadId,
    jumpToActiveWorkloadRow,
    revealedGuestId,
    selectedGuestId,
    setChartHoverSync,
    setHoveredWorkloadId,
    setSelectedGuestId,
    setTableBodyRef,
    setTableWrapperRef,
    shouldShowJumpToActiveWorkloadRow,
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
    revealedGuestId,
    selectedGuestId,
    tableBodyRef,
  });

  return {
    activeAlerts,
    alertsEnabled,
    allGuests,
    activeSummaryWorkloadId,
    bottomSpacerHeight,
    chartHoverSync,
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
    jumpToActiveWorkloadRow,
    kioskMode,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    mobileVisibleColumnIds,
    mobileVisibleColumns,
    navigate,
    nodeByInstance,
    namespaceFilterConfig,
    platformFilterConfig,
    platformOptions,
    reconnect,
    reconnectSurface,
    search,
    selectedGuestId,
    selectedHostHint,
    selectedPlatform,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setChartHoverSync,
    setGroupingMode,
    setHoveredWorkloadId,
    setSearch,
    setSelectedGuestId,
    setSelectedPlatform,
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
    shouldShowJumpToActiveWorkloadRow,
    sortDirection,
    sortKey,
    statusMode,
    surfaceConnected,
    surfaceInitialDataReceived,
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
