import { createMemo } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { useKioskMode } from '@/hooks/useKioskMode';
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

  return {
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
    nodeOnlineByLabel,
    highlightedRecordId,
    getRecordAlertState,
    isLoadingPools,
  };
};
