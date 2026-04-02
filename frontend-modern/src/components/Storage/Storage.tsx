import { Component, Show } from 'solid-js';
import StorageCephSection from '@/components/Storage/StorageCephSection';
import StorageContentCard from '@/components/Storage/StorageContentCard';
import StoragePageBanners from '@/components/Storage/StoragePageBanners';
import StoragePageControls from '@/components/Storage/StoragePageControls';
import StoragePageSummary from '@/components/Storage/StoragePageSummary';
import { SummaryScopeBar } from '@/components/shared/SummaryScopeBar';
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
    clearPinnedSummaryScope,
    setFocusedStorageGroupScope,
    setHoveredStorageGroupScope,
    setHoveredStorageResourceId,
    setSelectedDiskId,
    setSummaryTableRootRef,
    hasPinnedSummaryScope,
    pinnedSummaryScopePresentation,
    shouldShowPinnedSummaryScopeFallback,
    shouldShowJumpToActiveStorageRow,
  } = useStoragePageModel();

  return (
    <div class="space-y-4">
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

      <StoragePageBanners kind={activeBannerKind} reconnect={reconnect} />

      <Show when={hasPinnedSummaryScope() && shouldShowPinnedSummaryScopeFallback()}>
        <SummaryScopeBar
          testId="storage-summary-scope"
          scope={pinnedSummaryScopePresentation()}
          onClear={clearPinnedSummaryScope}
        />
      </Show>

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
  );
};

export default Storage;
