import { Component } from 'solid-js';
import StorageCephSection from '@/components/Storage/StorageCephSection';
import StorageContentCard from '@/components/Storage/StorageContentCard';
import StoragePageBanners from '@/components/Storage/StoragePageBanners';
import StoragePageControls from '@/components/Storage/StoragePageControls';
import StoragePageSummary from '@/components/Storage/StoragePageSummary';
import { PageHeader } from '@/components/shared/PageHeader';
import { StickySummarySection } from '@/components/shared/StickySummarySection';
import { isStorageRecordCeph } from './storagePageState';
import { useStoragePageModel } from './useStoragePageModel';

const Storage: Component = () => {
  const {
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
    clearPinnedSummaryScope,
    filteredRecords,
    activeSummaryStorageGroupScope,
    activeSummaryStorageResourceId,
    focusedSummaryStorageGroupScope,
    focusedSummaryStorageGroupId,
    hoveredSummaryStorageGroupScope,
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
    chartHoverSync,
    hoveredStorageResourceId,
    isLoadingPools,
    focusedStorageResourceId,
    jumpToActiveStorageRow,
    selectedDiskId,
    setChartHoverSync,
    setClearSurfaceRootRef,
    setFocusedStorageGroupScope,
    setHoveredStorageGroupScope,
    setHoveredStorageResourceId,
    setSelectedDiskId,
    setSummaryTableRootRef,
    shouldShowJumpToActiveStorageRow,
  } = useStoragePageModel();

  return (
    <div
      ref={setClearSurfaceRootRef}
      class="space-y-4"
      data-testid="storage-page"
    >
      <PageHeader
        title="Storage"
        description="Track capacity, cluster health, and storage alerts across local and distributed systems."
      />

      <StickySummarySection desktopOnly={false}>
        <StoragePageSummary
          filteredRecordCount={() => filteredRecords().length}
          selectedNodeId={selectedNodeId}
          nodeOptions={nodeOptions}
          physicalDisks={physicalDisks}
          hoveredResourceId={hoveredStorageResourceId}
          hoveredGroupScope={hoveredSummaryStorageGroupScope}
          focusedResourceId={focusedStorageResourceId}
          focusedGroupScope={focusedSummaryStorageGroupScope}
          chartHoverSync={chartHoverSync}
          onChartHoverSyncChange={setChartHoverSync}
          showJumpToActiveRow={shouldShowJumpToActiveStorageRow}
          onJumpToActiveRow={jumpToActiveStorageRow}
        />
      </StickySummarySection>

      <StorageCephSection
        view={view}
        summary={cephSummaryStats}
        filteredRecords={filteredRecords}
        isCephRecord={isStorageRecordCeph}
      />

      <div class="space-y-4" data-testid="storage-interaction-surface">
        <div data-summary-clear-ignore>
          <StoragePageControls
            kioskMode={kioskMode}
            view={view}
            setView={setView}
            search={search}
            setSearch={setSearch}
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
            nodeFilterOptions={nodeFilterOptions()}
            selectedNodeId={selectedNodeId}
            setSelectedNodeId={setSelectedNodeId}
            storageFilterGroupBy={storageFilterGroupBy}
          />
        </div>

        <StoragePageBanners kind={activeBannerKind} reconnect={reconnect} />

        <StorageContentCard
          view={view}
          physicalDisks={physicalDisks}
          nodes={nodes}
          selectedNodeId={selectedNodeId}
          search={search}
          groupedRecords={groupedRecords}
          groupBy={groupBy}
          expandedGroups={expandedGroups}
          toggleGroup={toggleGroup}
          expandedPoolId={expandedPoolId}
          setExpandedPoolId={setExpandedPoolId}
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
        />
      </div>
    </div>
  );
};

export default Storage;
