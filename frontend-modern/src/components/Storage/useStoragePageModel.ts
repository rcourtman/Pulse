import { createEffect, createMemo, createSignal, onCleanup, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import {
  resolvePhysicalDiskMetricResourceId,
  resolveStorageRecordMetricResourceId,
} from '@/features/storageBackups/storageMetricsIdentity';
import { useKioskMode } from '@/hooks/useKioskMode';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import {
  isSummarySeriesInGroupScope,
  type SummarySeriesGroupScope,
} from '@/components/shared/summaryCardInteraction';
import { createRouteStateNavigateScheduler } from '@/utils/routeStateNavigation';
import { areSearchParamsEquivalent } from '@/utils/searchParams';
import { parseStorageLinkSearch, STORAGE_QUERY_PARAMS } from '@/routing/resourceLinks';
import { useStorageExpansionState } from './useStorageExpansionState';
import { useStorageFilterState } from './useStorageFilterState';
import { useStoragePageData } from './useStoragePageData';
import { useStoragePageFilters } from './useStoragePageFilters';
import { useStoragePageResources } from './useStoragePageResources';
import { useStoragePageStatus } from './useStoragePageStatus';
import { useStorageResourceHighlight } from './useStorageResourceHighlight';
import { isStorageRecordCeph } from './storagePageState';
import { buildStorageSummaryGroupScopeMap } from './storageSummaryGroups';

export const useStoragePageModel = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const routeStateNavigate = createRouteStateNavigateScheduler(
    navigate,
    () => `${untrack(() => location.pathname)}${untrack(() => location.search)}`,
  );
  const kioskMode = useKioskMode();
  const [hoveredStorageRowId, setHoveredStorageResourceId] = createSignal<string | null>(null);
  const [hoveredStorageGroupScope, setHoveredStorageGroupScope] =
    createSignal<SummarySeriesGroupScope | null>(null);
  const [selectedStorageGroupId, setSelectedStorageGroupIdRaw] = createSignal<string | null>(null);
  const [handledSummaryGroupId, setHandledSummaryGroupId] = createSignal<string | null>(null);
  const [selectedDiskId, setSelectedDiskId] = createSignal<string | null>(null);
  const {
    state,
    activeAlerts,
    reconnecting,
    reconnect,
    storageRecoveryResources,
    nodes,
    physicalDisks,
    cephResources,
    alertsEnabled,
  } = useStoragePageResources();

  const {
    search,
    setSearch,
    sourceFilter,
    setSourceFilter,
    healthFilter,
    setHealthFilter,
    view,
    setView,
    selectedNodeId,
    setSelectedNodeId,
    sortKey,
    setSortKey,
    sortDirection,
    setSortDirection,
    groupBy,
    setGroupBy,
  } = useStoragePageFilters({
    location,
    navigate,
  });

  const {
    records,
    getRecordAlertState,
    nodeOptions,
    diskNodeOptions,
    nodeOnlineByLabel,
    sourceOptions,
    filteredRecords,
    groupedRecords,
    cephSummaryStats,
  } = useStoragePageData({
    state: () => state,
    resources: storageRecoveryResources.resources,
    activeAlerts,
    alertsEnabled,
    nodes,
    physicalDisks,
    cephResources,
    search,
    sourceFilter,
    healthFilter,
    selectedNodeId,
    sortKey,
    sortDirection,
    groupBy,
  });

  const surfaceInitialDataReceived = createMemo(
    () =>
      records().length > 0 ||
      !storageRecoveryResources.loading() ||
      Boolean(storageRecoveryResources.error()),
  );
  const surfaceConnected = createMemo(
    () =>
      storageRecoveryResources.loading() ||
      records().length > 0 ||
      !storageRecoveryResources.error(),
  );
  const reconnectSurface = () => {
    void storageRecoveryResources.refetch();
    reconnect();
  };

  const { expandedGroups, expandedPoolId, setExpandedPoolId: setExpandedPoolIdRaw, toggleGroup } =
    useStorageExpansionState({
      groupedKeys: () => groupedRecords().map((group) => group.key),
      view,
    });
  const storageRecordMetricIds = createMemo(() => {
    const ids = new Map<string, string>();
    for (const record of records()) {
      ids.set(record.id, resolveStorageRecordMetricResourceId(record));
    }
    return ids;
  });
  const physicalDiskMetricIds = createMemo(() => {
    const ids = new Map<string, string>();
    for (const disk of physicalDisks()) {
      ids.set(disk.id, resolvePhysicalDiskMetricResourceId(disk));
    }
    return ids;
  });
  const hoveredStorageResourceId = createMemo(() => {
    const hoveredId = hoveredStorageRowId();
    if (!hoveredId) return null;
    if (view() === 'disks') {
      return physicalDiskMetricIds().get(hoveredId) ?? hoveredId;
    }
    return storageRecordMetricIds().get(hoveredId) ?? hoveredId;
  });
  const focusedStorageResourceId = createMemo(() => {
    if (view() === 'disks') {
      const selectedId = selectedDiskId();
      if (!selectedId) return null;
      return physicalDiskMetricIds().get(selectedId) ?? selectedId;
    }
    const expandedId = expandedPoolId();
    if (!expandedId) return null;
    return storageRecordMetricIds().get(expandedId) ?? expandedId;
  });
  const storageGroupKeyByMetricSeriesId = createMemo(() => {
    const keys = new Map<string, string>();
    for (const group of groupedRecords()) {
      for (const record of group.items) {
        keys.set(resolveStorageRecordMetricResourceId(record), group.key);
      }
    }
    return keys;
  });
  const physicalDiskSeriesIds = createMemo(() => {
    const ids = new Set<string>();
    for (const disk of physicalDisks()) {
      ids.add(resolvePhysicalDiskMetricResourceId(disk));
    }
    return ids;
  });
  const storageSummaryGroupScopes = createMemo<Map<string, SummarySeriesGroupScope>>(() => {
    if (view() !== 'pools' || groupBy() === 'none') {
      return new Map<string, SummarySeriesGroupScope>();
    }
    return buildStorageSummaryGroupScopeMap(groupedRecords(), groupBy());
  });
  const focusedStorageGroupScope = createMemo<SummarySeriesGroupScope | null>(() => {
    const groupId = selectedStorageGroupId();
    if (!groupId) {
      return null;
    }
    return storageSummaryGroupScopes().get(groupId) ?? null;
  });

  const scheduleStorageSummaryGroupPath = (summaryGroupId: string | null) => {
    const currentParams = new URLSearchParams(location.search);
    const nextParams = new URLSearchParams(location.search);
    nextParams.delete(STORAGE_QUERY_PARAMS.summaryGroup);
    const normalizedSummaryGroupId = summaryGroupId?.trim() || '';
    if (normalizedSummaryGroupId) {
      nextParams.set(STORAGE_QUERY_PARAMS.summaryGroup, normalizedSummaryGroupId);
    }
    if (areSearchParamsEquivalent(currentParams, nextParams)) {
      return;
    }
    const nextSearch = nextParams.toString();
    const nextPath = nextSearch ? `${location.pathname}?${nextSearch}` : location.pathname;
    routeStateNavigate.schedule(nextPath);
  };

  const clearFocusedStorageGroup = () => {
    setSelectedStorageGroupIdRaw(null);
    scheduleStorageSummaryGroupPath(null);
  };

  const clearPinnedSummaryScope = () => {
    setExpandedPoolIdRaw(null);
    setSelectedDiskId(null);
    clearFocusedStorageGroup();
  };

  const setExpandedPoolId = (
    value: string | null | ((current: string | null) => string | null),
  ) => {
    const nextValue = typeof value === 'function' ? value(expandedPoolId()) : value;
    const focusedScope = focusedStorageGroupScope();
    const nextResourceId =
      nextValue && storageRecordMetricIds().get(nextValue) ? storageRecordMetricIds().get(nextValue)! : null;
    if (focusedScope && nextResourceId && !isSummarySeriesInGroupScope(focusedScope, nextResourceId)) {
      clearFocusedStorageGroup();
    }
    setExpandedPoolIdRaw(nextValue);
  };

  const setFocusedStorageGroupScope = (scope: SummarySeriesGroupScope | null) => {
    const nextGroupId = scope?.id ?? null;
    setSelectedStorageGroupIdRaw(nextGroupId);
    if (scope && !isSummarySeriesInGroupScope(scope, focusedStorageResourceId())) {
      setExpandedPoolIdRaw(null);
    }
    scheduleStorageSummaryGroupPath(nextGroupId);
  };

  createEffect(() => {
    view();
    setHoveredStorageResourceId(null);
    if (view() !== 'pools') {
      setHoveredStorageGroupScope(null);
      if (selectedStorageGroupId()) {
        setFocusedStorageGroupScope(null);
      }
    }
  });

  createEffect(() => {
    const { summaryGroup } = parseStorageLinkSearch(location.search);
    if (!summaryGroup) {
      if (handledSummaryGroupId() === null) {
        return;
      }
      setSelectedStorageGroupIdRaw(null);
      setHandledSummaryGroupId(null);
      return;
    }
    if (summaryGroup === handledSummaryGroupId()) {
      return;
    }
    setSelectedStorageGroupIdRaw(summaryGroup);
    setHandledSummaryGroupId(summaryGroup);
  });

  createEffect(() => {
    const hoveredGroup = hoveredStorageGroupScope();
    if (!hoveredGroup) {
      return;
    }
    if (!storageSummaryGroupScopes().has(hoveredGroup.id)) {
      setHoveredStorageGroupScope(null);
    }
  });

  createEffect(() => {
    const selectedGroupId = selectedStorageGroupId();
    if (!selectedGroupId) {
      return;
    }
    if (!focusedStorageGroupScope()) {
      setFocusedStorageGroupScope(null);
    }
  });

  createEffect(() => {
    const focusedScope = focusedStorageGroupScope();
    const focusedResourceId = focusedStorageResourceId();
    if (!focusedScope || !focusedResourceId) {
      return;
    }
    if (!isSummarySeriesInGroupScope(focusedScope, focusedResourceId)) {
      clearFocusedStorageGroup();
    }
  });

  const {
    nodeFilterOptions,
    sourceFilterOptions,
    storageFilterGroupBy,
    storageFilterStatus,
    setStorageFilterStatus,
  } = useStorageFilterState({
    view,
    nodeOptions,
    diskNodeOptions,
    selectedNodeId,
    setSelectedNodeId,
    sourceOptions,
    healthFilter,
    setHealthFilter,
    groupBy,
  });

  const { activeBannerKind, isLoadingPools } = useStoragePageStatus({
    loading: storageRecoveryResources.loading,
    error: storageRecoveryResources.error,
    filteredRecordCount: () => filteredRecords().length,
    connected: surfaceConnected,
    initialDataReceived: surfaceInitialDataReceived,
    reconnecting,
    view,
  });

  const highlightedRecordId = useStorageResourceHighlight({
    locationPathname: () => location.pathname,
    locationSearch: () => location.search,
    navigate,
    records,
    isStorageRecordCeph,
    setExpandedPoolId,
  });
  const summaryInteraction = useSummaryPageInteractionState({
    clearPinnedScope: clearPinnedSummaryScope,
    hoveredGroupScope: hoveredStorageGroupScope,
    hoveredSeriesId: hoveredStorageResourceId,
    focusedGroupScope: focusedStorageGroupScope,
    focusedGroupId: selectedStorageGroupId,
    focusedSeriesId: focusedStorageResourceId,
    revealActiveSeries: (seriesId) => {
      if (physicalDiskSeriesIds().has(seriesId)) {
        if (view() !== 'disks') {
          setView('disks');
        }
        return;
      }

      const groupKey = storageGroupKeyByMetricSeriesId().get(seriesId);
      if (!groupKey) {
        return;
      }
      if (view() !== 'pools') {
        setView('pools');
      }
      if (!expandedGroups().has(groupKey)) {
        toggleGroup(groupKey);
      }
    },
  });

  onCleanup(() => {
    routeStateNavigate.cleanup();
  });

  return {
    activeSummaryScopeState: summaryInteraction.activeScopeState,
    activeSummaryStorageGroupScope: summaryInteraction.activeGroupScope,
    activeSummaryStorageResourceId: summaryInteraction.activeSeriesId,
    clearPinnedSummaryScope,
    kioskMode,
    reconnect: reconnectSurface,
    selectedNodeId,
    setSelectedNodeId,
    view,
    setView,
    search,
    setSearch,
    sourceFilter,
    setSourceFilter,
    sortKey,
    setSortKey,
    sortDirection,
    setSortDirection,
    groupBy,
    setGroupBy,
    storageFilterStatus,
    setStorageFilterStatus,
    storageFilterGroupBy,
    sourceFilterOptions,
    nodeFilterOptions,
    activeBannerKind,
    cephSummaryStats,
    connected: surfaceConnected,
    filteredRecords,
    initialDataReceived: surfaceInitialDataReceived,
    nodeOptions,
    physicalDisks,
    nodes,
    groupedRecords,
    expandedGroups,
    toggleGroup,
    expandedPoolId,
    setExpandedPoolId,
    chartHoverSync: summaryInteraction.chartHoverSync,
    focusedStorageResourceId,
    nodeOnlineByLabel,
    highlightedRecordId,
    getRecordAlertState,
    hoveredStorageResourceId,
    isLoadingPools,
    jumpToActiveStorageRow: summaryInteraction.jumpToActiveRow,
    focusedSummaryStorageGroupScope: focusedStorageGroupScope,
    focusedSummaryStorageGroupId: selectedStorageGroupId,
    hoveredSummaryStorageGroupScope: hoveredStorageGroupScope,
    selectedDiskId,
    setChartHoverSync: summaryInteraction.setChartHoverSync,
    setClearSurfaceRootRef: summaryInteraction.setClearSurfaceRootRef,
    setFocusedStorageGroupScope,
    setHoveredStorageGroupScope,
    setHoveredStorageResourceId,
    setSelectedDiskId,
    setSummaryTableRootRef: summaryInteraction.setTableRootRef,
    shouldShowJumpToActiveStorageRow: summaryInteraction.shouldShowJumpToActiveRow,
  };
};
