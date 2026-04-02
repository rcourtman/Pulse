import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { DiskList } from '@/components/Storage/DiskList';
import StoragePoolsTable from '@/components/Storage/StoragePoolsTable';
import {
  STORAGE_CONTENT_CARD_BODY_CLASS,
  STORAGE_CONTENT_CARD_HEADER_CLASS,
} from '@/features/storageBackups/storagePagePresentation';
import type { Resource } from '@/types/resource';
import type { StorageGroupKey, StorageGroupedRecords } from './useStorageModel';
import type { StorageAlertRowState } from '@/features/storageBackups/storageAlertState';
import type { StorageView } from './storagePageState';
import { useStorageContentCardModel } from './useStorageContentCardModel';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';

type StorageContentCardProps = {
  view: () => StorageView;
  physicalDisks: () => Resource[];
  nodes: () => Resource[];
  selectedNodeId: () => string;
  search: () => string;
  groupedRecords: () => StorageGroupedRecords[];
  groupBy: () => StorageGroupKey;
  expandedGroups: () => Set<string>;
  toggleGroup: (key: string) => void;
  expandedPoolId: () => string | null;
  setExpandedPoolId: (value: string | ((current: string | null) => string | null) | null) => void;
  nodeOnlineByLabel: () => Map<string, boolean>;
  highlightedRecordId: () => string | null;
  getRecordAlertState: (recordId: string) => StorageAlertRowState;
  isLoadingPools: () => boolean;
  activeSummaryGroupScope: () => SummarySeriesGroupScope | null;
  hoveredSummaryGroupScope: () => SummarySeriesGroupScope | null;
  focusedSummaryGroupScope: () => SummarySeriesGroupScope | null;
  focusedSummaryGroupId: () => string | null;
  onGroupFocusChange: (scope: SummarySeriesGroupScope | null) => void;
  onGroupHoverChange: (scope: SummarySeriesGroupScope | null) => void;
  highlightedSummaryResourceId: () => string | null;
  hoveredStorageResourceId: () => string | null;
  setTableRootRef: (element: HTMLDivElement | undefined) => void;
  setHoveredStorageResourceId: (value: string | null) => void;
  selectedDiskId: () => string | null;
  setSelectedDiskId: (value: string | null) => void;
};

export const StorageContentCard: Component<StorageContentCardProps> = (props) => {
  const model = useStorageContentCardModel({
    view: props.view,
    selectedNodeId: props.selectedNodeId,
  });

  return (
    <Card ref={props.setTableRootRef} padding="none" tone="card" class="overflow-hidden">
      <div class={STORAGE_CONTENT_CARD_HEADER_CLASS}>{model.heading()}</div>
      <Show when={model.showDisks()}>
        <div class={STORAGE_CONTENT_CARD_BODY_CLASS}>
          <DiskList
            disks={props.physicalDisks()}
            nodes={props.nodes()}
            selectedNode={model.selectedDiskNodeId()}
            searchTerm={props.search()}
            selectedDiskId={props.selectedDiskId()}
            highlightedSummarySeriesId={props.highlightedSummaryResourceId()}
            onSelectedDiskChange={props.setSelectedDiskId}
            onHoverChange={props.setHoveredStorageResourceId}
          />
        </div>
      </Show>
      <Show when={model.showPools()}>
        <StoragePoolsTable
          groupedRecords={props.groupedRecords()}
          groupBy={props.groupBy()}
          expandedGroups={props.expandedGroups()}
          toggleGroup={props.toggleGroup}
          expandedPoolId={props.expandedPoolId()}
          setExpandedPoolId={props.setExpandedPoolId}
          physicalDisks={props.physicalDisks()}
          nodeOnlineByLabel={props.nodeOnlineByLabel()}
          highlightedRecordId={props.highlightedRecordId()}
          getRecordAlertState={props.getRecordAlertState}
          isLoading={props.isLoadingPools()}
          activeSummaryGroupScope={props.activeSummaryGroupScope()}
          hoveredSummaryGroupScope={props.hoveredSummaryGroupScope()}
          focusedSummaryGroupScope={props.focusedSummaryGroupScope()}
          focusedSummaryGroupId={props.focusedSummaryGroupId()}
          onGroupFocusChange={props.onGroupFocusChange}
          onGroupHoverChange={props.onGroupHoverChange}
          highlightedSummarySeriesId={props.highlightedSummaryResourceId()}
          onHoverChange={props.setHoveredStorageResourceId}
        />
      </Show>
    </Card>
  );
};

export default StorageContentCard;
