import { createEffect, createMemo, onCleanup, type Accessor } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import {
  RuntimeInventorySourcesAPI,
  type RuntimeInventorySourcesResponse,
} from '@/api/runtimeInventorySources';
import { recordWorkloadHistoryActivity } from '@/api/workloadHistoryActivity';
import { nodeOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import type { VM, Container, Node } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { ViewMode, WorkloadGuest, WorkloadType } from '@/types/workloads';
import { useWebSocket } from '@/contexts/appRuntime';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { sessionCanReadInfrastructureSettings } from '@/stores/sessionSettingsCapabilities';
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
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  buildWorkloadSummaryGroupScopeMap,
  createWorkloadSortComparator,
  filterWorkloads,
  selectVisibleWorkloadInventory,
  type FilterWorkloadsParams,
} from './workloadSelectors';
import {
  type WorkloadsGroupingMode,
  type WorkloadsMemoryDisplayBasis,
  type WorkloadsMetricDisplayMode,
  type WorkloadsMetricHoverMode,
  type WorkloadsStatusOption,
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
import {
  buildNestedWorkloadContextByGuestId,
  type NestedWorkloadContextByGuestId,
} from './nestedWorkloadContext';
import { buildGuestParentNodeMapFromNodes } from './workloadTopology';

const WORKLOADS_INFRASTRUCTURE_SOURCES_QUERY =
  'type=agent,docker-host,k8s-cluster,k8s-node,pbs,pmg,storage,physical_disk,ceph';
const WORKLOADS_INVENTORY_SOURCES_POLL_INTERVAL_MS = 15000;
const EMPTY_INVENTORY_SOURCES_RESPONSE: RuntimeInventorySourcesResponse = {
  sources: [],
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
  layoutWidth?: Accessor<number | null | undefined>;
  useWorkloads?: boolean;
  forcedPlatform?: string;
  forcedViewMode?: ViewMode;
  excludedWorkloadTypes?: readonly WorkloadType[];
  showNestedExcludedWorkloads?: boolean;
  forcedGroupingMode?: WorkloadsGroupingMode;
  defaultSortKey?: WorkloadsSortKey;
  filterAriaLabel?: string;
  filterSearchPlaceholder?: string;
  filterSearchEmptyMessage?: string;
  filterStatusOptions?: readonly WorkloadsStatusOption[];
  // When the surface is mounted inside a platform-first page, the page owns
  // platform scope through `forcedPlatform`. `suppressPlatformFilter`
  // removes the redundant Platform chip from the filter row since the
  // platform is already fixed by the owning page.
  suppressPlatformFilter?: boolean;
  // When a platform page renders its own shared WorkloadsFilter above the
  // embedded surface (so one toolbar drives both the page's top table and
  // this surface), set `suppressFilterToolbar` so the surface skips its
  // internal filter row and avoids a duplicate.
  suppressFilterToolbar?: boolean;
  // An owning platform page may provide the canonical unified-resource
  // snapshot it already fetched. This avoids a second workload/infrastructure
  // request and keeps both surfaces on the same refresh generation.
  resourceSnapshot?: Accessor<Resource[] | undefined>;
  resourceSnapshotRefetch?: () => Promise<unknown>;
  statusModeStorageScope?: string;
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
  metricHoverMode?: Accessor<WorkloadsMetricHoverMode>;
  onMetricHoverModeChange?: (value: WorkloadsMetricHoverMode) => void;
  metricHistoryRange?: Accessor<WorkloadTableMetricHistoryRange>;
  onMetricHistoryRangeChange?: (value: WorkloadTableMetricHistoryRange) => void;
  // Proxmox can compare guest memory use with either the allocation assigned
  // to that guest or the total memory of its resolved parent node.
  memoryDisplayBasis?: Accessor<WorkloadsMemoryDisplayBasis>;
  // Platform pages can scope column preferences when a shared workload type
  // needs different defaults or labels on that platform-owned page.
  columnVisibilityStorageScope?: string;
  additionalDefaultHiddenColumnIds?: string[];
  columnLabelOverrides?: Partial<Record<string, string>>;
  groupLabelBadges?: Record<string, WorkloadGroupLabelBadge>;
  /** Lets an owning platform page share one viewer-safe source-health poll. */
  inventorySourcesQuery?: WorkloadsInventorySourcesQuery;
  /** Keeps a warmed, hidden platform tab from parsing or rewriting another tab's URL state. */
  routeStateEnabled?: Accessor<boolean>;
}

export interface WorkloadsInventorySourcesQuery {
  error: Accessor<unknown>;
  refetch: (options?: { background?: boolean }) => Promise<RuntimeInventorySourcesResponse>;
  value: Accessor<RuntimeInventorySourcesResponse>;
}

export type WorkloadSortKey = WorkloadsSortKey;

export interface WorkloadGroupLabelBadge {
  label: string;
  classes: string;
  title?: string;
}

export function useWorkloadsState(props: WorkloadsSurfaceProps) {
  const navigate = useNavigate();
  const ws = useWebSocket();
  const { connected, activeAlerts, initialDataReceived, reconnecting, reconnect } = ws;
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = alertsActivation.detectionEnabled;

  const kioskMode = useKioskMode();

  const { guestMetadata, handleCustomUrlUpdate } = useWorkloadGuestMetadataState();

  const workloadsEnabled = createMemo(() => props.useWorkloads === true);
  const workloads = useWorkloads(workloadsEnabled, {
    resourceSnapshot: props.resourceSnapshot,
    refetchSnapshot: props.resourceSnapshotRefetch,
  });
  const infrastructureSources = useUnifiedResources({
    query: WORKLOADS_INFRASTRUCTURE_SOURCES_QUERY,
    cacheKey: 'workloads-infrastructure-sources',
    enabled: () => workloadsEnabled() && !props.resourceSnapshot,
  });
  const infrastructureResources = createMemo(() =>
    props.resourceSnapshot ? (props.resourceSnapshot() ?? []) : infrastructureSources.resources(),
  );
  const infrastructureLoading = createMemo(() =>
    props.resourceSnapshot
      ? props.resourceSnapshot() === undefined
      : infrastructureSources.loading(),
  );
  const inventorySourcesResourceKey = createMemo(() =>
    workloadsEnabled() && !props.inventorySourcesQuery ? 'enabled' : null,
  );
  const ownedInventorySourcesSnapshot = createNonSuspendingQuery<
    RuntimeInventorySourcesResponse,
    string
  >({
    source: inventorySourcesResourceKey,
    fetcher: () => RuntimeInventorySourcesAPI.list(),
    initialValue: EMPTY_INVENTORY_SOURCES_RESPONSE,
    cacheKey: (key) => `workloads-inventory-sources:${key}`,
  });
  const inventorySourcesSnapshot = props.inventorySourcesQuery ?? ownedInventorySourcesSnapshot;

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

  const excludedWorkloadTypeSet = createMemo(() => new Set(props.excludedWorkloadTypes ?? []));
  const rawGuests = createMemo<WorkloadGuest[]>(() =>
    workloadsEnabled() ? workloads.workloads() : [],
  );
  const allGuests = createMemo<WorkloadGuest[]>(() =>
    workloadsEnabled()
      ? dedupeGuests(
          selectVisibleWorkloadInventory({
            guests: rawGuests(),
            excludedTypes: excludedWorkloadTypeSet(),
            platformScope: props.forcedPlatform,
          }),
        )
      : [],
  );
  const nestedWorkloadContextByGuestId = createMemo<NestedWorkloadContextByGuestId>(() =>
    props.showNestedExcludedWorkloads
      ? buildNestedWorkloadContextByGuestId({
          guests: rawGuests(),
          visibleGuests: allGuests(),
          excludedWorkloadTypes: props.excludedWorkloadTypes,
          platformScope: props.forcedPlatform,
        })
      : {},
  );

  const [showFilters, setShowFilters] = usePersistentSignal<boolean>(
    'workloadsShowFilters',
    false,
    {
      deserialize: (raw) => raw === 'true',
      serialize: (value) => String(value),
    },
  );

  // Drives the Availability column's presence. Availability checks are opt-in
  // per resource, so a surface where nothing is probed would otherwise carry a
  // permanently empty column. Read the unfiltered guest set so narrowing the
  // table with a search or status filter never makes the column vanish.
  const hasAvailabilityData = createMemo(() =>
    allGuests().some(
      (guest) => Boolean(guest.availability) || (guest.availabilityChecks?.length ?? 0) > 0,
    ),
  );

  const {
    clusterFilterConfig,
    clusterOptions,
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
    selectedCluster,
    selectedHostHint,
    selectedPlatform,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setSelectedCluster,
    setSelectedPlatform,
    setSelectedKubernetesContext,
    setSelectedKubernetesNamespace,
    setViewMode,
    viewMode,
    workloadNodeOptions,
    containerRuntimeOptions,
  } = useWorkloadRouteState({
    allGuests,
    forcedPlatform: props.forcedPlatform,
    forcedViewMode: props.forcedViewMode,
    showFilters,
    setShowFilters,
    routeStateEnabled: props.routeStateEnabled,
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
    workloadColumnWidths,
    workloadManualColumnSizing,
    workloadManualColumnSizingSupported,
    workloadTableManualWidth,
    beginWorkloadColumnResize,
    previewWorkloadColumnWidth,
    commitWorkloadColumnResize,
    cancelWorkloadColumnResize,
    clearWorkloadColumnWidth,
    resetWorkloadColumnWidths,
    workloadMetricDisplayMode,
    workloadMetricHoverMode,
    workloadMetricHistoryRange,
    setWorkloadMetricDisplayMode,
    setWorkloadMetricHoverMode,
    setWorkloadMetricHistoryRange,
  } = useWorkloadsControlsState({
    defaultSortKey: props.defaultSortKey,
    forcedGroupingMode: props.forcedGroupingMode,
    statusModeStorageScope: props.statusModeStorageScope,
    metricDisplayMode: props.metricDisplayMode,
    onMetricDisplayModeChange: props.onMetricDisplayModeChange,
    metricHoverMode: props.metricHoverMode,
    onMetricHoverModeChange: props.onMetricHoverModeChange,
    metricHistoryRange: props.metricHistoryRange,
    onMetricHistoryRangeChange: props.onMetricHistoryRangeChange,
    columnVisibilityStorageScope: props.columnVisibilityStorageScope,
    additionalDefaultHiddenColumnIds: props.additionalDefaultHiddenColumnIds,
    excludedWorkloadTypes: props.excludedWorkloadTypes,
    hasAvailabilityData,
    columnLabelOverrides: props.columnLabelOverrides,
    layoutWidth: props.layoutWidth,
    setShowFilters,
    showFilters,
    viewMode: effectiveViewMode,
    routeStateEnabled: props.routeStateEnabled,
  });

  const infrastructureNodes = createMemo<Node[]>(() => {
    const merged = new Map<string, Node>();
    props.nodes.forEach((node) => merged.set(node.id, node));

    if (workloadsEnabled()) {
      infrastructureResources()
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
  const workloadMemoryDisplayBasis: Accessor<WorkloadsMemoryDisplayBasis> =
    props.memoryDisplayBasis ?? (() => 'guest');
  const memoryParentNodeByGuestId = createMemo(() =>
    buildGuestParentNodeMapFromNodes(allGuests(), infrastructureNodes()),
  );

  const workloadsInfrastructureEmptyState = createMemo(() =>
    getWorkloadsInfrastructureEmptyState(),
  );
  const workloadsGuestsEmptyState = createMemo(() => getWorkloadsGuestsEmptyState(search()));
  const workloadsLoadingState = createMemo(() => getWorkloadsLoadingState(reconnecting()));
  const workloadsNoInventoryState = createMemo(() =>
    getWorkloadsNoInventoryState(sessionCanReadInfrastructureSettings()),
  );
  const workloadsDisconnectedState = createMemo(() =>
    getWorkloadsDisconnectedState(reconnecting()),
  );
  const workloadInventoryIssues = createMemo(() =>
    buildWorkloadInventorySourceIssues(inventorySourcesSnapshot.value().sources ?? []),
  );
  const hasWorkloadsData = createMemo(() => allGuests().length > 0);
  const hasInfrastructureSources = createMemo(() =>
    workloadsEnabled()
      ? infrastructureNodes().length > 0 || infrastructureResources().length > 0
      : infrastructureNodes().length > 0,
  );
  const infrastructureSourceStateReady = createMemo(() =>
    workloadsEnabled() ? hasInfrastructureSources() || !infrastructureLoading() : true,
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
      void inventorySourcesSnapshot.refetch({ background: true });
    }
    reconnect();
  };

  const getNodeTemperatureThresholds = (node: Node) =>
    alertsActivation.getMetricThresholds('node', 'temperature', nodeOverrideIdCandidates(node));

  createEffect(() => {
    if (!workloadsEnabled() || props.inventorySourcesQuery) return;
    const handle = window.setInterval(() => {
      void inventorySourcesSnapshot.refetch({ background: true });
    }, WORKLOADS_INVENTORY_SOURCES_POLL_INTERVAL_MS);
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
    createWorkloadSortComparator(sortKey() || '', sortDirection(), {
      memoryValue: (guest) => {
        if (workloadMemoryDisplayBasis() !== 'host') {
          return guest.memory?.usage ?? 0;
        }
        const hostTotal =
          memoryParentNodeByGuestId()[getCanonicalWorkloadId(guest)]?.memory?.total ?? 0;
        const used = guest.memory?.used ?? 0;
        return hostTotal > 0 && Number.isFinite(used) ? (used / hostTotal) * 100 : 0;
      },
    }),
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
      selectedCluster: selectedCluster(),
      containerRuntime: containerRuntime().trim() || null,
    };
    return filterWorkloads(params);
  });
  const groupLabelBadges = createMemo<Record<string, WorkloadGroupLabelBadge>>(
    () => props.groupLabelBadges ?? {},
  );
  const summaryGroupScopes = createMemo(() =>
    buildWorkloadSummaryGroupScopeMap({
      guests: filteredGuests(),
      nodes: infrastructureNodes(),
      groupingMode: groupingMode(),
      sortComparator: guestSortComparator(),
      groupLabelBadges: groupLabelBadges(),
    }),
  );

  const {
    activeSummaryWorkloadGroupScope,
    activeSummaryWorkloadId,
    clearPinnedSummaryScope,
    focusedSummaryWorkloadGroupScope,
    focusedSummaryWorkloadGroupId,
    hoveredSummaryWorkloadGroupScope,
    hoveredWorkloadId,
    revealedGuestId,
    selectedGuestId,
    setClearSurfaceRootRef,
    setFocusedWorkloadGroupScope,
    setHoveredWorkloadGroupScope,
    setHoveredWorkloadId,
    setSelectedGuestId,
    setTableBodyRef,
    setTableRootRef,
    setTableWrapperRef,
    tableBodyRef,
  } = useWorkloadSelectionState({
    clearAdditionalPageStateOnEscape: () => {
      resetWorkloadsControls();
      resetWorkloadRouteFilters();
    },
    filteredGuests,
    summaryGroupScopes,
    routeStateEnabled: props.routeStateEnabled,
  });

  const activeHistoryGuest = createMemo(() => {
    const activeId = hoveredWorkloadId();
    if (!activeId) return null;
    return allGuests().find((guest) => getCanonicalWorkloadId(guest) === activeId) ?? null;
  });
  createEffect(() => {
    if (
      workloadMetricDisplayMode() === 'bars' &&
      workloadMetricHoverMode() === 'details' &&
      hoveredWorkloadId() !== null
    ) {
      setHoveredWorkloadId(null);
    }
  });
  const {
    bottomSpacerHeight,
    getGroupLabel,
    groupedGuests,
    groupedWindowing,
    guestParentNodeMap,
    inventoryStats,
    isScrollToTopVisible,
    nodeByInstance,
    topSpacerHeight,
    scrollToTop,
    totalStats,
    visibleGroupKeys,
    windowedGroupedGuests,
    workloadIOEmphasis,
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
    groupLabelBadges,
  });

  const metricHistoryPrefetchGuests = createMemo(() =>
    visibleGroupKeys().flatMap((groupKey) => windowedGroupedGuests()[groupKey] ?? []),
  );
  const workloadMetricHistory = useWorkloadTableMetricHistory({
    activeGuest: activeHistoryGuest,
    // Persistent Trends owns the estate-wide batch reader. Bars mode warms a
    // small visible/adjacent window so sequential row hover remains immediate.
    enabled: () => workloadMetricDisplayMode() === 'sparklines',
    onDemand: () =>
      workloadMetricDisplayMode() === 'bars' && workloadMetricHoverMode() === 'history',
    prefetchGuests: metricHistoryPrefetchGuests,
    range: workloadMetricHistoryRange,
    selectedNode,
  });
  const [workloadHistoryHintSeen, setWorkloadHistoryHintSeen] = usePersistentSignal<boolean>(
    STORAGE_KEYS.WORKLOADS_HISTORY_HINT_SEEN,
    false,
    { deserialize: (raw) => raw === 'true' },
  );
  const workloadMetricHistoryHintVisible = createMemo(
    () =>
      !workloadHistoryHintSeen() &&
      workloadMetricDisplayMode() === 'bars' &&
      workloadMetricHoverMode() === 'history' &&
      filteredGuests().length > 0,
  );
  createEffect(() => {
    const active = activeHistoryGuest();
    if (!active || !workloadMetricHistory.hasGuestHistory?.(active)) return;
    recordWorkloadHistoryActivity('preview');
    if (!workloadHistoryHintSeen()) {
      setWorkloadHistoryHintSeen(true);
    }
  });

  return {
    activeAlerts,
    alertsEnabled,
    allGuests,
    inventoryStats,
    isScrollToTopVisible,
    activeSummaryWorkloadGroupScope,
    activeSummaryWorkloadId,
    clearPinnedSummaryScope,
    bottomSpacerHeight,
    clusterFilterConfig,
    clusterOptions,
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
    getNodeTemperatureThresholds,
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
    infrastructureSourceStateReady,
    infrastructureNodes,
    initialDataReceived,
    isMobile,
    isSearchLocked,
    isWorkloadsRoute,
    kioskMode,
    kubernetesContextOptions,
    kubernetesNamespaceOptions,
    navigate,
    nodeByInstance,
    namespaceFilterConfig,
    nestedWorkloadContextByGuestId,
    platformFilterConfig: props.suppressPlatformFilter ? () => undefined : platformFilterConfig,
    platformOptions,
    reconnect,
    reconnectSurface,
    search,
    scrollToTop,
    selectedCluster,
    selectedGuestId,
    selectedHostHint,
    selectedPlatform,
    selectedKubernetesContext,
    selectedKubernetesNamespace,
    selectedNode,
    setContainerRuntime,
    setClearSurfaceRootRef,
    setFocusedWorkloadGroupScope,
    setGroupingMode,
    setHoveredWorkloadGroupScope,
    setHoveredWorkloadId,
    setSearch,
    setSelectedCluster,
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
    setWorkloadMetricHoverMode,
    setWorkloadMetricHistoryRange,
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
    workloadMetricHistoryHintVisible,
    workloadMetricDisplayMode,
    workloadMetricHoverMode,
    workloadMemoryDisplayBasis,
    workloadMetricHistory,
    workloadTableVisibleColumnIds,
    workloadTableVisibleColumns,
    workloadTableLayoutMode,
    workloadColumnWidths,
    workloadManualColumnSizing,
    workloadManualColumnSizingSupported,
    workloadTableManualWidth,
    beginWorkloadColumnResize,
    previewWorkloadColumnWidth,
    commitWorkloadColumnResize,
    cancelWorkloadColumnResize,
    clearWorkloadColumnWidth,
    resetWorkloadColumnWidths,
    workloadNodeOptions,
    workloads,
    workloadInventoryIssues,
    workloadsNoInventoryState,
    ws,
    groupingMode,
    compactGroupHeaders: () => props.compactGroupHeaders === true,
    groupNodeDrawerMode: () => props.groupNodeDrawerMode ?? 'inline',
    groupLabelBadges,
  } as const;
}

export type WorkloadsState = ReturnType<typeof useWorkloadsState>;
