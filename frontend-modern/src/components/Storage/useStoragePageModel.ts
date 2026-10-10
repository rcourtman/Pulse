import { createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import {
  SUMMARY_TIME_RANGE_LABEL,
  type SummaryTimeRange,
} from '@/components/shared/summaryTimeRange';
import { buildStorageCapacityDeltaPresentation } from '@/features/storageBackups/storageCapacityDeltaPresentation';
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
import { useStoragePageResources, type StoragePageResourceSource } from './useStoragePageResources';
import { useStoragePageStatus } from './useStoragePageStatus';
import { useStorageResourceHighlight } from './useStorageResourceHighlight';
import { useStorageSummaryCharts } from './useStorageSummaryCharts';
import {
  DEFAULT_STORAGE_SELECTED_NODE_ID,
  DEFAULT_STORAGE_SOURCE_FILTER,
  DEFAULT_STORAGE_DISK_GROUP_FILTER,
  DEFAULT_STORAGE_DISK_ROLE_FILTER,
  isStorageRecordCeph,
} from './storagePageState';

type UseStoragePageModelOptions = {
  forcedSourceFilter?: () => string | undefined;
  resourceSource?: StoragePageResourceSource;
};

export const useStoragePageModel = (options: UseStoragePageModelOptions = {}) => {
  const navigate = useNavigate();
  const location = useLocation();
  const kioskMode = useKioskMode();
  const [hoveredStorageRowId, setHoveredStorageResourceId] = createSignal<string | null>(null);
  const [selectedDiskId, setSelectedDiskId] = createSignal<string | null>(null);
  // Growth column on the storage table is anchored to a 24h window. The
  // standalone summary chart that let operators retune the range was
  // retired with the platform-first IA migration; nothing reads or sets
  // this now, so the previous persistent signal collapsed to a constant.
  const summaryTimeRange = (): SummaryTimeRange => '24h';
  const {
    state,
    activeAlerts,
    reconnect,
    storageResources,
    nodes,
    physicalDisks,
    getDiskAlertResourceIds,
    cephResources,
    alertsEnabled,
  } = useStoragePageResources({ resourceSource: options.resourceSource });

  const {
    search,
    setSearch,
    sourceFilter,
    setSourceFilter,
    healthFilter,
    diskRoleFilter,
    setDiskRoleFilter,
    diskGroupFilter,
    setDiskGroupFilter,
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
    lockedSourceFilter: options.forcedSourceFilter,
  });
  const storageSummaryCharts = useStorageSummaryCharts({
    timeRange: summaryTimeRange,
    nodeId: selectedNodeId,
    caller: 'useStoragePageModel',
    deferInitialLoad: true,
  });
  const storageGrowthRangeLabel = createMemo(
    () => SUMMARY_TIME_RANGE_LABEL[summaryTimeRange()] ?? summaryTimeRange(),
  );
  const storageGrowthColumnLabel = createMemo(() => `Growth (${storageGrowthRangeLabel()})`);
  const storageGrowthBySeriesId = createMemo(() => {
    const growth = new Map<string, ReturnType<typeof buildStorageCapacityDeltaPresentation>>();
    const pools = storageSummaryCharts.data()?.pools ?? {};
    for (const [seriesId, pool] of Object.entries(pools)) {
      growth.set(
        seriesId,
        buildStorageCapacityDeltaPresentation(pool.used ?? [], storageGrowthRangeLabel()),
      );
    }
    return growth;
  });

  const {
    records,
    getRecordAlertState,
    nodeOptions,
    diskNodeOptions,
    nodeOnlineByLabel,
    sourceOptions,
    diskSourceOptions,
    diskRoleOptions,
    diskGroupOptions,
    filteredRecords,
    groupedRecords,
  } = useStoragePageData({
    state: () => state,
    resources: storageResources.resources,
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
    storageGrowthBySeriesId,
    groupBy,
  });

  const surfaceInitialDataReceived = createMemo(
    () => records().length > 0 || !storageResources.loading() || Boolean(storageResources.error()),
  );
  const surfaceConnected = createMemo(
    () => storageResources.loading() || records().length > 0 || !storageResources.error(),
  );
  const reconnectSurface = () => {
    void storageResources.refetch();
    reconnect();
  };
  const { expandedGroups, expandedPoolId, setExpandedPoolId, toggleGroup } =
    useStorageExpansionState({
      groupBy,
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
  const clearPinnedSummaryScope = () => {
    setExpandedPoolId(null);
    setSelectedDiskId(null);
  };
  const clearStorageFilters = () => {
    setSearch('');
    setSourceFilter(DEFAULT_STORAGE_SOURCE_FILTER);
    setDiskRoleFilter(DEFAULT_STORAGE_DISK_ROLE_FILTER);
    setDiskGroupFilter(DEFAULT_STORAGE_DISK_GROUP_FILTER);
    setStorageFilterStatus('all');
    setSelectedNodeId(DEFAULT_STORAGE_SELECTED_NODE_ID);
  };
  const clearAllPageStateOnEscape = () => {
    clearPinnedSummaryScope();
    clearStorageFilters();
  };

  createEffect(() => {
    view();
    setHoveredStorageResourceId(null);
  });

  const {
    nodeFilterOptions,
    sourceFilterOptions,
    diskRoleFilterOptions,
    diskGroupFilterOptions,
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
    diskSourceOptions,
    diskRoleOptions,
    diskGroupOptions,
    sourceFilter,
    setSourceFilter,
    lockedSourceFilter: options.forcedSourceFilter,
    healthFilter,
    setHealthFilter,
    diskRoleFilter,
    setDiskRoleFilter,
    diskGroupFilter,
    setDiskGroupFilter,
    groupBy,
  });

  const { isLoadingPools } = useStoragePageStatus({
    loading: storageResources.loading,
    filteredRecordCount: () => filteredRecords().length,
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
    hoveredSeriesId: hoveredStorageResourceId,
    focusedSeriesId: focusedStorageResourceId,
    onEscapeClear: clearAllPageStateOnEscape,
    revealActiveSeries: (seriesId) => {
      // Focus only ever names a row in the active view, so reveal never
      // switches views; it reopens a focused pool's collapsed group.
      if (view() !== 'pools') {
        return;
      }
      const groupKey = storageGroupKeyByMetricSeriesId().get(seriesId);
      if (groupKey && !expandedGroups().has(groupKey)) {
        toggleGroup(groupKey);
      }
    },
  });

  return {
    activeSummaryStorageResourceId: summaryInteraction.activeSeriesId,
    clearPinnedSummaryScope,
    kioskMode,
    reconnect: reconnectSurface,
    storageGrowthBySeriesId,
    storageGrowthColumnLabel,
    selectedNodeId,
    setSelectedNodeId,
    view,
    setView,
    search,
    setSearch,
    sourceFilter,
    setSourceFilter,
    healthFilter,
    diskRoleFilter,
    setDiskRoleFilter,
    diskRoleOptions: diskRoleFilterOptions,
    diskGroupFilter,
    setDiskGroupFilter,
    diskGroupOptions: diskGroupFilterOptions,
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
    connected: surfaceConnected,
    filteredRecords,
    initialDataReceived: surfaceInitialDataReceived,
    nodeOptions,
    physicalDisks,
    getDiskAlertResourceIds,
    nodes,
    groupedRecords,
    expandedGroups,
    toggleGroup,
    expandedPoolId,
    setExpandedPoolId,
    nodeOnlineByLabel,
    highlightedRecordId,
    getRecordAlertState,
    hoveredStorageResourceId,
    isLoadingPools,
    selectedDiskId,
    setClearSurfaceRootRef: summaryInteraction.setClearSurfaceRootRef,
    setHoveredStorageResourceId,
    setSelectedDiskId,
    setSummaryTableRootRef: summaryInteraction.setTableRootRef,
  };
};
