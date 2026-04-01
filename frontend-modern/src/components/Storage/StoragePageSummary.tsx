import { Component } from 'solid-js';
import StorageSummary from '@/components/Storage/StorageSummary';
import type { Resource } from '@/types/resource';
import type { StoragePageNodeOption } from './storagePageState';
import { useStoragePageSummary } from './useStoragePageSummary';
import type { SummaryChartHoverSync } from '@/components/shared/contextualFocus';

type StoragePageSummaryProps = {
  filteredRecordCount: () => number;
  selectedNodeId: () => string;
  nodeOptions: () => StoragePageNodeOption[];
  physicalDisks: () => Resource[];
  hoveredResourceId: () => string | null;
  focusedResourceId: () => string | null;
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
      focusedResourceId={props.focusedResourceId()}
      chartHoverSync={props.chartHoverSync()}
      onChartHoverSyncChange={props.onChartHoverSyncChange}
      showJumpToActiveRow={props.showJumpToActiveRow()}
      onJumpToActiveRow={props.onJumpToActiveRow}
    />
  );
};

export default StoragePageSummary;
