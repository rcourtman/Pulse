import { Component, Show, createEffect } from 'solid-js';
import StorageCephSection from '@/components/Storage/StorageCephSection';
import StorageContentCard from '@/components/Storage/StorageContentCard';
import StoragePageBanners from '@/components/Storage/StoragePageBanners';
import StoragePageControls from '@/components/Storage/StoragePageControls';
import StoragePageSummary from '@/components/Storage/StoragePageSummary';
import { StorageViewSegmentedControl } from '@/components/Storage/StorageViewSegmentedControl';
import { PageHeader } from '@/components/shared/PageHeader';
import { StickySummarySection } from '@/components/shared/StickySummarySection';
import { isStorageRecordCeph } from './storagePageState';
import { useStoragePageModel } from './useStoragePageModel';

type StorageProps = {
  embedded?: boolean;
  tableOnly?: boolean;
  forcedView?: 'pools' | 'disks';
  forcedSourceFilter?: string;
};

const Storage: Component<StorageProps> = (props) => {
  const {
    kioskMode,
    reconnect,
    summaryTimeRange,
    setSummaryTimeRange,
    storageSummaryCollapsed,
    setStorageSummaryCollapsed,
    storageGrowthBySeriesId,
    storageGrowthColumnLabel,
    storageSummaryData,
    storageSummaryLoaded,
    storageSummaryFetchFailed,
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

  return (
    <div ref={setClearSurfaceRootRef} class="space-y-4" data-testid="storage-page">
      <Show when={!props.embedded}>
        <PageHeader
          title="Storage"
          description="Review capacity, topology, protection, and physical media across connected storage platforms."
        />
      </Show>

      <Show when={!props.tableOnly && !storageSummaryCollapsed()}>
        <StickySummarySection desktopOnly={false} stickyDesktopOnly>
          <div class="space-y-2">
            <StoragePageSummary
              filteredRecords={filteredRecords}
              search={search}
              sourceFilter={sourceFilter}
              healthFilter={healthFilter}
              diskRoleFilter={diskRoleFilter}
              diskGroupFilter={diskGroupFilter}
              selectedNodeId={selectedNodeId}
              nodeOptions={nodeOptions}
              physicalDisks={physicalDisks}
              summaryTimeRange={summaryTimeRange}
              setSummaryTimeRange={setSummaryTimeRange}
              storageSummaryData={storageSummaryData}
              storageSummaryLoaded={storageSummaryLoaded}
              storageSummaryFetchFailed={storageSummaryFetchFailed}
              hoveredResourceId={hoveredStorageResourceId}
              hoveredGroupScope={hoveredSummaryStorageGroupScope}
              focusedResourceId={focusedStorageResourceId}
              focusedGroupScope={focusedSummaryStorageGroupScope}
              chartHoverSync={chartHoverSync}
              onChartHoverSyncChange={setChartHoverSync}
              showJumpToActiveRow={shouldShowJumpToActiveStorageRow}
              onJumpToActiveRow={jumpToActiveStorageRow}
              onScopeToDegradedPools={() => {
                setView('pools');
                setStorageFilterStatus('attention');
              }}
              onScopeToFailingDisks={() => {
                setView('disks');
                setStorageFilterStatus('attention');
              }}
            />
          </div>
        </StickySummarySection>
      </Show>

      <Show when={!props.tableOnly}>
        <StorageCephSection
          view={view}
          summary={cephSummaryStats}
          filteredRecords={filteredRecords}
          isCephRecord={isStorageRecordCeph}
        />
      </Show>

      <div class="space-y-4" data-testid="storage-interaction-surface">
        <Show when={!props.tableOnly}>
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
              chartsCollapsed={storageSummaryCollapsed}
              onChartsToggle={() => setStorageSummaryCollapsed((collapsed) => !collapsed)}
            />
          </div>
        </Show>
        <Show when={!props.tableOnly}>
          <StoragePageBanners kind={activeBannerKind} reconnect={reconnect} />
        </Show>

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
            props.tableOnly && !props.forcedView && !kioskMode() ? (
              <StorageViewSegmentedControl value={view()} onChange={setView} />
            ) : undefined
          }
        />
      </div>
    </div>
  );
};

export default Storage;
