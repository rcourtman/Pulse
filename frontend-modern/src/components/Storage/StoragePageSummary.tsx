import { Component } from 'solid-js';
import StorageSummary from '@/components/Storage/StorageSummary';
import type { Resource } from '@/types/resource';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import type { StoragePageNodeOption } from './storagePageState';
import { useStoragePageSummary } from './useStoragePageSummary';
import type { SummaryChartHoverSync } from '@/components/shared/contextualFocus';

type StoragePageSummaryProps = {
  filteredRecordCount: () => number;
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
  const { summaryTimeRange, setSummaryTimeRange, poolCount, diskCount } = useStoragePageSummary({
    filteredRecordCount: props.filteredRecordCount,
    selectedNodeId: props.selectedNodeId,
    nodeOptions: props.nodeOptions,
    physicalDisks: props.physicalDisks,
  });

  return (
    <StorageSummary
      poolCount={poolCount()}
      diskCount={diskCount()}
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
