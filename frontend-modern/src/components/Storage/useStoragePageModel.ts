import { createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import {
  resolvePhysicalDiskMetricResourceId,
  resolveStorageRecordMetricResourceId,
} from '@/features/storageBackups/storageMetricsIdentity';
import { useKioskMode } from '@/hooks/useKioskMode';
import { useSummaryPageInteractionState } from '@/components/shared/summaryTableFocus';
import { useStorageExpansionState } from './useStorageExpansionState';
import { useStorageFilterState } from './useStorageFilterState';
import { useStoragePageData } from './useStoragePageData';
import { useStoragePageFilters } from './useStoragePageFilters';
import { useStoragePageResources } from './useStoragePageResources';
import { useStoragePageStatus } from './useStoragePageStatus';
import { useStorageResourceHighlight } from './useStorageResourceHighlight';
import { isStorageRecordCeph } from './storagePageState';

export const useStoragePageModel = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const kioskMode = useKioskMode();
  const [hoveredStorageRowId, setHoveredStorageResourceId] = createSignal<string | null>(null);
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

  const { expandedGroups, expandedPoolId, setExpandedPoolId, toggleGroup } =
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

  createEffect(() => {
    view();
    setHoveredStorageResourceId(null);
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
    hoveredSeriesId: hoveredStorageResourceId,
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

  return {
    activeSummaryStorageResourceId: summaryInteraction.activeSeriesId,
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
    selectedDiskId,
    setChartHoverSync: summaryInteraction.setChartHoverSync,
    setHoveredStorageResourceId,
    setSelectedDiskId,
    setSummaryTableRootRef: summaryInteraction.setTableRootRef,
    shouldShowJumpToActiveStorageRow: summaryInteraction.shouldShowJumpToActiveRow,
  };
};
