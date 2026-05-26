import { Component, createEffect } from 'solid-js';
import StorageContentCard from '@/components/Storage/StorageContentCard';
import StoragePageControls from '@/components/Storage/StoragePageControls';
import { StorageViewSegmentedControl } from '@/components/Storage/StorageViewSegmentedControl';
import { DEFAULT_STORAGE_SELECTED_NODE_ID } from './storagePageState';
import { useStoragePageModel } from './useStoragePageModel';

type StorageProps = {
  forcedView?: 'pools' | 'disks';
  forcedSourceFilter?: string;
  // `suppressSourceFilter` drops the redundant Source chip from the
  // controls toolbar since the platform page already locks source scope
  // through `forcedSourceFilter`. `suppressNodeFilter` lets platform
  // pages use the search input for host/node scoping, matching their
  // overview-page filter contract.
  suppressSourceFilter?: boolean;
  suppressNodeFilter?: boolean;
  filterAriaLabel?: string;
  filterSearchPlaceholder?: string;
  filterSearchEmptyMessage?: string;
};

const Storage: Component<StorageProps> = (props) => {
  const {
    kioskMode,
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
    diskRoleOptions,
    diskGroupFilter,
    setDiskGroupFilter,
    diskGroupOptions,
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
    clearPinnedSummaryScope,
    activeSummaryStorageGroupScope,
    activeSummaryStorageResourceId,
    focusedSummaryStorageGroupScope,
    focusedSummaryStorageGroupId,
    hoveredSummaryStorageGroupScope,
    physicalDisks,
    nodes,
    groupedRecords,
    expandedGroups,
    toggleGroup,
    expandedPoolId,
    setExpandedPoolId,
    storageGrowthBySeriesId,
    storageGrowthColumnLabel,
    nodeOnlineByLabel,
    highlightedRecordId,
    getRecordAlertState,
    hoveredStorageResourceId,
    isLoadingPools,
    selectedDiskId,
    setClearSurfaceRootRef,
    setFocusedStorageGroupScope,
    setHoveredStorageGroupScope,
    setHoveredStorageResourceId,
    setSelectedDiskId,
    setSummaryTableRootRef,
  } = useStoragePageModel({
    forcedSourceFilter: () => props.forcedSourceFilter,
  });

  createEffect(() => {
    const forcedSource = props.forcedSourceFilter?.trim();
    if (!forcedSource) return;
    if (sourceFilter() !== forcedSource) {
      setSourceFilter(forcedSource);
    }
  });

  createEffect(() => {
    if (!props.forcedView) return;
    if (view() !== props.forcedView) {
      setView(props.forcedView);
    }
  });

  createEffect(() => {
    if (!props.suppressNodeFilter) return;
    if (selectedNodeId() !== DEFAULT_STORAGE_SELECTED_NODE_ID) {
      setSelectedNodeId(DEFAULT_STORAGE_SELECTED_NODE_ID);
    }
  });

  // Namespace saved views per platform context. Every live consumer is a
  // platform-embedded storage tab that locks source scope via
  // forcedSourceFilter; views never leak across platforms.
  const savedViewsKey = `storage-${(props.forcedSourceFilter ?? '').trim().toLowerCase()}`;

  return (
    <div ref={setClearSurfaceRootRef} class="space-y-4" data-testid="storage-page">
      <div class="space-y-4" data-testid="storage-interaction-surface">
        <div data-summary-clear-ignore>
          <StoragePageControls
            kioskMode={kioskMode}
            savedViewsKey={savedViewsKey}
            view={view}
            setView={setView}
            search={search}
            setSearch={setSearch}
            filterAriaLabel={props.filterAriaLabel}
            searchPlaceholder={props.filterSearchPlaceholder}
            searchEmptyMessage={props.filterSearchEmptyMessage}
            groupBy={groupBy}
            setGroupBy={setGroupBy}
            sortKey={sortKey}
            setSortKey={setSortKey}
            sortDirection={sortDirection}
            setSortDirection={setSortDirection}
            statusFilter={storageFilterStatus}
            setStatusFilter={setStorageFilterStatus}
            sourceFilter={sourceFilter}
            setSourceFilter={setSourceFilter}
            sourceOptions={sourceFilterOptions}
            suppressSourceFilter={props.suppressSourceFilter || Boolean(props.forcedSourceFilter)}
            suppressNodeFilter={props.suppressNodeFilter}
            diskRoleFilter={diskRoleFilter}
            setDiskRoleFilter={setDiskRoleFilter}
            diskRoleOptions={diskRoleOptions}
            diskGroupFilter={diskGroupFilter}
            setDiskGroupFilter={setDiskGroupFilter}
            diskGroupOptions={diskGroupOptions}
            nodeFilterOptions={nodeFilterOptions()}
            selectedNodeId={selectedNodeId}
            setSelectedNodeId={setSelectedNodeId}
            storageFilterGroupBy={storageFilterGroupBy}
          />
        </div>

        <StorageContentCard
          view={view}
          physicalDisks={physicalDisks}
          nodes={nodes}
          sourceFilter={sourceFilter}
          healthFilter={healthFilter}
          diskRoleFilter={diskRoleFilter}
          diskGroupFilter={diskGroupFilter}
          selectedNodeId={selectedNodeId}
          search={search}
          groupedRecords={groupedRecords}
          groupBy={groupBy}
          sortKey={sortKey}
          setSortKey={setSortKey}
          sortDirection={sortDirection}
          setSortDirection={setSortDirection}
          expandedGroups={expandedGroups}
          toggleGroup={toggleGroup}
          expandedPoolId={expandedPoolId}
          setExpandedPoolId={setExpandedPoolId}
          storageGrowthBySeriesId={storageGrowthBySeriesId}
          storageGrowthColumnLabel={storageGrowthColumnLabel}
          nodeOnlineByLabel={nodeOnlineByLabel}
          highlightedRecordId={highlightedRecordId}
          getRecordAlertState={getRecordAlertState}
          isLoadingPools={isLoadingPools}
          activeSummaryGroupScope={activeSummaryStorageGroupScope}
          clearPinnedSummaryScope={clearPinnedSummaryScope}
          hoveredSummaryGroupScope={hoveredSummaryStorageGroupScope}
          focusedSummaryGroupScope={focusedSummaryStorageGroupScope}
          focusedSummaryGroupId={focusedSummaryStorageGroupId}
          onGroupFocusChange={setFocusedStorageGroupScope}
          onGroupHoverChange={setHoveredStorageGroupScope}
          highlightedSummaryResourceId={activeSummaryStorageResourceId}
          hoveredStorageResourceId={hoveredStorageResourceId}
          setTableRootRef={setSummaryTableRootRef}
          setHoveredStorageResourceId={setHoveredStorageResourceId}
          selectedDiskId={selectedDiskId}
          setSelectedDiskId={setSelectedDiskId}
          actions={
            !props.forcedView && !kioskMode() ? (
              <StorageViewSegmentedControl value={view()} onChange={setView} />
            ) : undefined
          }
        />
      </div>
    </div>
  );
};

export default Storage;
