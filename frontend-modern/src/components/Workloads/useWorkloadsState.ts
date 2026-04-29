import { createEffect, createMemo } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { VM, Container, Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { useWebSocket } from '@/contexts/appRuntime';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { useWorkloads } from '@/hooks/useWorkloads';
import { useKioskMode } from '@/hooks/useKioskMode';
import {
  getWorkloadsDisconnectedState,
  getWorkloadsGuestsEmptyState,
  getWorkloadsInfrastructureEmptyState,
  getWorkloadsLoadingState,
} from '@/utils/workloadEmptyStatePresentation';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import {
  buildWorkloadSummaryGroupScopeMap,
  createWorkloadSortComparator,
  filterWorkloads,
  type FilterWorkloadsParams,
} from './workloadSelectors';
import {
  type WorkloadsSortKey,
} from './workloadsFilterModel';
import { useWorkloadsControlsState } from './useWorkloadsControlsState';
import { useWorkloadGuestMetadataState } from './useWorkloadGuestMetadataState';
import { useWorkloadSelectionState } from './useWorkloadSelectionState';
import { useWorkloadsDerivedState } from './useWorkloadsDerivedState';
import { useWorkloadRouteState } from './useWorkloadRouteState';

export interface WorkloadsSurfaceProps {
  vms: VM[];
  containers: Container[];
  nodes: Node[];
  useWorkloads?: boolean;
}

export type WorkloadSortKey = WorkloadsSortKey;

export function useWorkloadsState(props: WorkloadsSurfaceProps) {
  const navigate = useNavigate();
  const ws = useWebSocket();
  const { connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = ws;
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');

  const kioskMode = useKioskMode();

  const { guestMetadata, handleCustomUrlUpdate } = useWorkloadGuestMetadataState();

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
    'workloadsShowFilters',
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
    setSelectedPlatform,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setViewMode,
    viewMode,
    workloadNodeOptions,
    containerRuntimeOptions,
  } = useWorkloadRouteState({
    allGuests,
    showFilters,
    setShowFilters,
  });

  const {
    columnVisibility,
    workloadsFilterColumnVisibility,
    groupingMode,
    handleBeforeAutoFocus,
    handleSort,
    handleTagClick,
    isMobile,
    isSearchLocked,
    resetWorkloadsControls,
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
    workloadTableVisibleColumnIds,
    workloadTableVisibleColumns,
    workloadTableLayoutMode,
    workloadsSummaryCollapsed,
    workloadsSummaryRange,
    setWorkloadsSummaryCollapsed,
    setWorkloadsSummaryRange,
  } = useWorkloadsControlsState({
    setShowFilters,
    showFilters,
    viewMode,
  });

  const workloadsInfrastructureEmptyState = createMemo(() => getWorkloadsInfrastructureEmptyState());
  const workloadsGuestsEmptyState = createMemo(() => getWorkloadsGuestsEmptyState(search()));
  const workloadsLoadingState = createMemo(() => getWorkloadsLoadingState(reconnecting()));
  const workloadsDisconnectedState = createMemo(() => getWorkloadsDisconnectedState(reconnecting()));
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
  let hasSeenConnectedState = connected();
  createEffect(() => {
    const isConnected = connected();
    if (isConnected) {
      if (workloadsEnabled() && !lastConnected && hasSeenConnectedState) {
        void workloads.refetch();
      }
      hasSeenConnectedState = true;
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
  const summaryGroupScopes = createMemo(() =>
    buildWorkloadSummaryGroupScopeMap({
      guests: filteredGuests(),
      nodes: props.nodes,
      groupingMode: groupingMode(),
      sortComparator: guestSortComparator(),
    }),
  );

  const {
    activeSummaryWorkloadGroupScope,
    activeSummaryWorkloadId,
    chartHoverSync,
    clearPinnedSummaryScope,
    focusedSummaryWorkloadGroupScope,
    focusedSummaryWorkloadGroupId,
    hoveredWorkloadId,
    hoveredSummaryWorkloadGroupScope,
    jumpToActiveWorkloadRow,
    revealedGuestId,
    selectedGuestId,
    setChartHoverSync,
    setClearSurfaceRootRef,
    setFocusedWorkloadGroupScope,
    setHoveredWorkloadGroupScope,
    setHoveredWorkloadId,
    setSelectedGuestId,
    setTableBodyRef,
    setTableRootRef,
    setTableWrapperRef,
    shouldShowJumpToActiveWorkloadRow,
    tableBodyRef,
  } = useWorkloadSelectionState({
    clearAdditionalPageStateOnEscape: () => {
      resetWorkloadsControls();
      resetWorkloadRouteFilters();
    },
    filteredGuests,
    summaryGroupScopes,
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
  } = useWorkloadsDerivedState({
    activeAlerts: () => activeAlerts,
    alertsEnabled,
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
    activeSummaryWorkloadGroupScope,
    activeSummaryWorkloadId,
    clearPinnedSummaryScope,
    bottomSpacerHeight,
    chartHoverSync,
    columnVisibility,
    connected,
    containerRuntime,
    containerRuntimeFilterConfig,
    containerRuntimeOptions,
    workloadsFilterColumnVisibility,
    workloadsDisconnectedState,
    workloadsGuestsEmptyState,
    workloadsInfrastructureEmptyState,
    workloadsLoadingState,
    filteredGuests,
    focusedSummaryWorkloadGroupScope,
    focusedSummaryWorkloadGroupId,
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
    hoveredSummaryWorkloadGroupScope,
    hoveredWorkloadId,
    initialDataReceived,
    isMobile,
    isSearchLocked,
    isWorkloadsRoute,
    jumpToActiveWorkloadRow,
    kioskMode,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
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
    setClearSurfaceRootRef,
    setFocusedWorkloadGroupScope,
    setGroupingMode,
    setHoveredWorkloadGroupScope,
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
    setTableRootRef,
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
    workloadTableVisibleColumnIds,
    workloadTableVisibleColumns,
    workloadTableLayoutMode,
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

export type WorkloadsState = ReturnType<typeof useWorkloadsState>;
