import { Component } from 'solid-js';
import StorageSummary from '@/components/Storage/StorageSummary';
import type { StorageSummaryChartsResponse } from '@/api/charts';
import type { SummaryTimeRange } from '@/components/shared/summaryTimeRange';
import type { Resource } from '@/types/resource';
import type { StorageHealthFilter, StorageRecord } from '@/features/storageBackups/models';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import type { StoragePageNodeOption } from './storagePageState';
import { useStoragePageSummary } from './useStoragePageSummary';
import type { SummaryChartHoverSync } from '@/components/shared/contextualFocus';

type StoragePageSummaryProps = {
  filteredRecords: () => StorageRecord[];
  search: () => string;
  sourceFilter: () => string;
  healthFilter: () => StorageHealthFilter;
  diskRoleFilter: () => string;
  diskGroupFilter: () => string;
  selectedNodeId: () => string;
  nodeOptions: () => StoragePageNodeOption[];
  physicalDisks: () => Resource[];
  summaryTimeRange: () => SummaryTimeRange;
  setSummaryTimeRange: (range: SummaryTimeRange) => void;
  storageSummaryData: () => StorageSummaryChartsResponse | null;
  storageSummaryLoaded: () => boolean;
  storageSummaryFetchFailed: () => boolean;
  hoveredResourceId: () => string | null;
  hoveredGroupScope: () => SummarySeriesGroupScope | null;
  focusedResourceId: () => string | null;
  focusedGroupScope: () => SummarySeriesGroupScope | null;
  chartHoverSync: () => SummaryChartHoverSync | null;
  onChartHoverSyncChange: (value: SummaryChartHoverSync | null) => void;
  showJumpToActiveRow: () => boolean;
  onJumpToActiveRow: () => void;
  onScopeToDegradedPools?: () => void;
  onScopeToFailingDisks?: () => void;
};

export const StoragePageSummary: Component<StoragePageSummaryProps> = (props) => {
  const { poolCount, diskCount, poolsDegraded, disksFailing } = useStoragePageSummary({
    filteredRecords: props.filteredRecords,
    search: props.search,
    sourceFilter: props.sourceFilter,
    healthFilter: props.healthFilter,
    diskRoleFilter: props.diskRoleFilter,
    diskGroupFilter: props.diskGroupFilter,
    selectedNodeId: props.selectedNodeId,
    nodeOptions: props.nodeOptions,
    physicalDisks: props.physicalDisks,
  });

  return (
    <StorageSummary
      poolCount={poolCount()}
      diskCount={diskCount()}
      poolsDegraded={poolsDegraded()}
      disksFailing={disksFailing()}
      data={props.storageSummaryData()}
      loaded={props.storageSummaryLoaded()}
      fetchFailed={props.storageSummaryFetchFailed()}
      timeRange={props.summaryTimeRange()}
      onTimeRangeChange={props.setSummaryTimeRange}
      hoveredResourceId={props.hoveredResourceId()}
      hoveredGroupScope={props.hoveredGroupScope()}
      focusedResourceId={props.focusedResourceId()}
      focusedGroupScope={props.focusedGroupScope()}
      chartHoverSync={props.chartHoverSync()}
      onChartHoverSyncChange={props.onChartHoverSyncChange}
      showJumpToActiveRow={props.showJumpToActiveRow()}
      onJumpToActiveRow={props.onJumpToActiveRow}
      onScopeToDegradedPools={props.onScopeToDegradedPools}
      onScopeToFailingDisks={props.onScopeToFailingDisks}
    />
  );
};

export default StoragePageSummary;
