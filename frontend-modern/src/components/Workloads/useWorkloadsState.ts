import { createEffect, createMemo, onCleanup, type Accessor } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { ConnectionsAPI, type ConnectionsListResponse } from '@/api/connections';
import type { VM, Container, Node } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { ViewMode, WorkloadGuest } from '@/types/workloads';
import { useWebSocket } from '@/contexts/appRuntime';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { useWorkloads } from '@/hooks/useWorkloads';
import { useKioskMode } from '@/hooks/useKioskMode';
import {
  getWorkloadsDisconnectedState,
  getWorkloadsGuestsEmptyState,
  getWorkloadsInfrastructureEmptyState,
  getWorkloadsLoadingState,
  getWorkloadsNoInventoryState,
} from '@/utils/workloadEmptyStatePresentation';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import { nodeFromResource } from '@/utils/resourceStateAdapters';
import {
  buildWorkloadSummaryGroupScopeMap,
  createWorkloadSortComparator,
  filterWorkloads,
  type FilterWorkloadsParams,
} from './workloadSelectors';
import {
  type WorkloadsGroupingMode,
  type WorkloadsMetricDisplayMode,
  type WorkloadsSortKey,
} from './workloadsFilterModel';
import { type WorkloadTableMetricHistoryRange } from './workloadMetricHistoryModel';
import { useWorkloadsControlsState } from './useWorkloadsControlsState';
import { useWorkloadGuestMetadataState } from './useWorkloadGuestMetadataState';
import { useWorkloadSelectionState } from './useWorkloadSelectionState';
import { useWorkloadsDerivedState } from './useWorkloadsDerivedState';
import { useWorkloadRouteState } from './useWorkloadRouteState';
import { buildWorkloadInventorySourceIssues } from './workloadInventorySourceIssues';
import { useWorkloadTableMetricHistory } from './useWorkloadTableMetricHistory';

const WORKLOADS_INFRASTRUCTURE_SOURCES_QUERY =
  'type=agent,docker-host,k8s-cluster,k8s-node,pbs,pmg,storage,physical_disk,ceph';
const WORKLOADS_CONNECTIONS_POLL_INTERVAL_MS = 15000;
const EMPTY_CONNECTIONS_RESPONSE: ConnectionsListResponse = {
  connections: [],
  systems: [],
};

const isProxmoxNodeResource = (resource: Resource): boolean =>
  resource.type === 'agent' &&
  (resource.platformType === 'proxmox-pve' ||
    Boolean(resource.proxmox) ||
    Boolean(resource.platformData?.proxmox));

export interface WorkloadsSurfaceProps {
  vms: VM[];
  containers: Container[];
  nodes: Node[];
  useWorkloads?: boolean;
  embedded?: boolean;
  tableOnly?: boolean;
  forcedPlatform?: string;
  forcedViewMode?: ViewMode;
  forcedGroupingMode?: WorkloadsGroupingMode;
  defaultSortKey?: WorkloadsSortKey;
  // When the surface is mounted inside a platform-first page, the page owns
  // platform scope through `forcedPlatform`. Setting `showFilterToolbar`
  // keeps the operator-facing WorkloadsFilter visible alongside the table
  // even when `tableOnly` hides the summary cards/strip, and
  // `suppressPlatformFilter` removes the redundant Platform chip from that
  // filter row since the platform is already fixed by the owning page.
  showFilterToolbar?: boolean;
  suppressPlatformFilter?: boolean;
  // Platform pages that render their own hosts table above the embedded
  // workloads surface (e.g. Proxmox overview) own the per-host CPU / Memory
  // / Disk / Temperature / uptime / version stats. Setting
  // `compactGroupHeaders` strips those stats from the NodeGroupHeader rows
  // in grouped mode so the section dividers don't duplicate the info.
  compactGroupHeaders?: boolean;
  // Default Workloads behavior owns grouped host row drawers inline. Platform
  // pages with a dedicated host table can disable that drawer so host details
  // open from the host-owned table instead of the embedded guest table.
  groupNodeDrawerMode?: 'inline' | 'disabled';
  // When a platform page owns the metric display mode + sparkline range
  // (so the same toggle drives both the page's hosts table and this
  // embedded workloads surface), pass the accessors + change handlers.
  // The page is responsible for persisting the values; when omitted, the
  // surface falls back to its own persistent signals.
  metricDisplayMode?: Accessor<WorkloadsMetricDisplayMode>;
  onMetricDisplayModeChange?: (value: WorkloadsMetricDisplayMode) => void;
  metricHistoryRange?: Accessor<WorkloadTableMetricHistoryRange>;
  onMetricHistoryRangeChange?: (value: WorkloadTableMetricHistoryRange) => void;
  // Platform pages can scope column preferences when a shared workload type
  // needs different defaults or labels on that platform-owned page.
  columnVisibilityStorageScope?: string;
  additionalDefaultHiddenColumnIds?: string[];
  columnLabelOverrides?: Partial<Record<string, string>>;
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
  const infrastructureSources = useUnifiedResources({
    query: WORKLOADS_INFRASTRUCTURE_SOURCES_QUERY,
    cacheKey: 'workloads-infrastructure-sources',
    enabled: workloadsEnabled,
  });
  const connectionsResourceKey = createMemo(() => (workloadsEnabled() ? 'enabled' : null));
  const connectionsSnapshot = createNonSuspendingQuery<ConnectionsListResponse, string>({
    source: connectionsResourceKey,
    fetcher: () => ConnectionsAPI.list(),
    initialValue: EMPTY_CONNECTIONS_RESPONSE,
    cacheKey: (key) => `workloads-connections:${key}`,
  });

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
  const effectiveViewMode = createMemo<ViewMode>(() => props.forcedViewMode ?? viewMode());
  const setEffectiveViewMode = (value: ViewMode): void => {
    if (props.forcedViewMode) return;
    setViewMode(value);
  };

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
    workloadMetricDisplayMode,
    workloadMetricHistoryRange,
    workloadsSummaryCollapsed,
    workloadsSummaryRange,
    setWorkloadMetricDisplayMode,
    setWorkloadMetricHistoryRange,
    setWorkloadsSummaryCollapsed,
    setWorkloadsSummaryRange,
  } = useWorkloadsControlsState({
    defaultSortKey: props.defaultSortKey,
    forcedGroupingMode: props.forcedGroupingMode,
    metricDisplayMode: props.metricDisplayMode,
    onMetricDisplayModeChange: props.onMetricDisplayModeChange,
    metricHistoryRange: props.metricHistoryRange,
    onMetricHistoryRangeChange: props.onMetricHistoryRangeChange,
    columnVisibilityStorageScope: props.columnVisibilityStorageScope,
    additionalDefaultHiddenColumnIds: props.additionalDefaultHiddenColumnIds,
    columnLabelOverrides: props.columnLabelOverrides,
    setShowFilters,
    showFilters,
    viewMode: effectiveViewMode,
  });

  const infrastructureNodes = createMemo<Node[]>(() => {
    const merged = new Map<string, Node>();
    props.nodes.forEach((node) => merged.set(node.id, node));

    if (workloadsEnabled()) {
      infrastructureSources
        .resources()
        .filter(isProxmoxNodeResource)
        .map(nodeFromResource)
        .filter((node): node is Node => Boolean(node))
        .forEach((node) => {
          const existing = merged.get(node.id);
          merged.set(node.id, existing ? { ...existing, ...node } : node);
        });
    }

    return Array.from(merged.values());
  });

  const workloadsInfrastructureEmptyState = createMemo(() =>
    getWorkloadsInfrastructureEmptyState(),
  );
  const workloadsGuestsEmptyState = createMemo(() => getWorkloadsGuestsEmptyState(search()));
  const workloadsLoadingState = createMemo(() => getWorkloadsLoadingState(reconnecting()));
  const workloadsNoInventoryState = createMemo(() => getWorkloadsNoInventoryState());
  const workloadsDisconnectedState = createMemo(() =>
    getWorkloadsDisconnectedState(reconnecting()),
  );
  const workloadInventoryIssues = createMemo(() =>
    buildWorkloadInventorySourceIssues(connectionsSnapshot.value().connections ?? []),
  );
  const workloadMetricHistory = useWorkloadTableMetricHistory({
    enabled: () => workloadMetricDisplayMode() === 'sparklines',
    range: workloadMetricHistoryRange,
    selectedNode,
  });
  const hasWorkloadsData = createMemo(() => allGuests().length > 0);
  const hasInfrastructureSources = createMemo(() =>
    workloadsEnabled()
      ? infrastructureNodes().length > 0 || infrastructureSources.resources().length > 0
      : infrastructureNodes().length > 0,
  );
  const infrastructureSourceStateReady = createMemo(() =>
    workloadsEnabled() ? hasInfrastructureSources() || !infrastructureSources.loading() : true,
  );
  const surfaceConnected = createMemo(() =>
    workloadsEnabled()
      ? workloads.loading() || hasWorkloadsData() || !workloads.error()
      : connected(),
  );
  const surfaceInitialDataReceived = createMemo(() =>
    workloadsEnabled()
      ? hasWorkloadsData() ||
        ((!workloads.loading() || Boolean(workloads.error())) && infrastructureSourceStateReady())
      : initialDataReceived(),
  );

  const reconnectSurface = () => {
    if (workloadsEnabled()) {
      void workloads.refetch();
      void connectionsSnapshot.refetch({ background: true });
    }
    reconnect();
  };

  createEffect(() => {
    if (!workloadsEnabled()) return;
    const handle = window.setInterval(() => {
      void connectionsSnapshot.refetch({ background: true });
    }, WORKLOADS_CONNECTIONS_POLL_INTERVAL_MS);
    onCleanup(() => window.clearInterval(handle));
  });

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
      viewMode: effectiveViewMode(),
      statusMode: statusMode(),
      searchTerm: search().trim(),
      selectedNode: selectedNode(),
      selectedHostHint: selectedHostHint(),
      selectedPlatform: props.forcedPlatform?.trim() || selectedPlatform(),
      selectedKubernetesContext: selectedKubernetesContext(),
      selectedKubernetesNamespace: selectedKubernetesNamespace(),
      containerRuntime: containerRuntime().trim() || null,
    };
    return filterWorkloads(params);
  });
  const summaryGroupScopes = createMemo(() =>
    buildWorkloadSummaryGroupScopeMap({
      guests: filteredGuests(),
      nodes: infrastructureNodes(),
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
    nodes: infrastructureNodes,
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
    hasInfrastructureSources,
    hostFilterConfig,
    hoveredSummaryWorkloadGroupScope,
    hoveredWorkloadId,
    infrastructureSourceStateReady,
    infrastructureNodes,
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
    platformFilterConfig: props.suppressPlatformFilter ? () => undefined : platformFilterConfig,
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
    setViewMode: setEffectiveViewMode,
    setWorkloadMetricDisplayMode,
    setWorkloadMetricHistoryRange,
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
    viewMode: effectiveViewMode,
    visibleColumns,
    visibleGroupKeys,
    windowedGroupedGuests,
    workloadIOEmphasis,
    workloadMetricHistoryRange,
    workloadMetricDisplayMode,
    workloadMetricHistory,
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
    workloadInventoryIssues,
    workloadsNoInventoryState,
    ws,
    groupingMode,
    compactGroupHeaders: () => props.compactGroupHeaders === true,
    groupNodeDrawerMode: () => props.groupNodeDrawerMode ?? 'inline',
  } as const;
}

export type WorkloadsState = ReturnType<typeof useWorkloadsState>;
