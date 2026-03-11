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
    connected,
    initialDataReceived,
    reconnecting,
    reconnect,
    nodes,
    physicalDisks,
    cephResources,
    storageRecoveryResources,
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
    connected,
    initialDataReceived,
    reconnecting,
    view,
  });

  const highlightedRecordId = useStorageResourceHighlight({
    locationSearch: () => location.search,
    records,
    isStorageRecordCeph,
    setExpandedPoolId,
  });

  return {
    kioskMode,
    reconnect,
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
    filteredRecords,
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
