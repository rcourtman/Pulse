import { Component } from 'solid-js';
import StorageSummary from '@/components/Storage/StorageSummary';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import type { StoragePageNodeOption } from './storagePageState';
import { useStoragePageSummary } from './useStoragePageSummary';
import type { SummaryChartHoverSync } from '@/components/shared/contextualFocus';

type StoragePageSummaryProps = {
  filteredRecords: () => StorageRecord[];
  selectedNodeId: () => string;
  nodeOptions: () => StoragePageNodeOption[];
  physicalDisks: () => Resource[];
  hoveredResourceId: () => string | null;
  hoveredGroupScope: () => SummarySeriesGroupScope | null;
  focusedResourceId: () => string | null;
  focusedGroupScope: () => SummarySeriesGroupScope | null;
  chartHoverSync: () => SummaryChartHoverSync | null;
  onChartHoverSyncChange: (value: SummaryChartHoverSync | null) => void;
  showJumpToActiveRow: () => boolean;
  onJumpToActiveRow: () => void;
};

export const StoragePageSummary: Component<StoragePageSummaryProps> = (props) => {
  const {
    summaryTimeRange,
    setSummaryTimeRange,
    poolCount,
    diskCount,
    poolsDegraded,
    disksFailing,
  } = useStoragePageSummary({
    filteredRecords: props.filteredRecords,
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
      timeRange={summaryTimeRange()}
      onTimeRangeChange={setSummaryTimeRange}
      nodeId={props.selectedNodeId()}
      hoveredResourceId={props.hoveredResourceId()}
      hoveredGroupScope={props.hoveredGroupScope()}
      focusedResourceId={props.focusedResourceId()}
      focusedGroupScope={props.focusedGroupScope()}
      chartHoverSync={props.chartHoverSync()}
      onChartHoverSyncChange={props.onChartHoverSyncChange}
      showJumpToActiveRow={props.showJumpToActiveRow()}
      onJumpToActiveRow={props.onJumpToActiveRow}
    />
  );
};

export default StoragePageSummary;
