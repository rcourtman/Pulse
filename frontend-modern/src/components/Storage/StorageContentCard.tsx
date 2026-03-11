import { Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { DiskList } from '@/components/Storage/DiskList';
import StoragePoolsTable from '@/components/Storage/StoragePoolsTable';
import {
  STORAGE_CONTENT_CARD_BODY_CLASS,
  STORAGE_CONTENT_CARD_HEADER_CLASS,
} from '@/features/storageBackups/storagePagePresentation';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { StorageGroupKey } from './useStorageModel';
import type { StorageAlertRowState } from '@/features/storageBackups/storageAlertState';
import type { StorageView } from './storagePageState';
import { useStorageContentCardModel } from './useStorageContentCardModel';

type StorageContentCardProps = {
  view: () => StorageView;
  physicalDisks: () => Resource[];
  nodes: () => Resource[];
  selectedNodeId: () => string;
  search: () => string;
  groupedRecords: () => Array<{ key: string; title: string; records: StorageRecord[] }>;
  groupBy: () => StorageGroupKey;
  expandedGroups: () => Set<string>;
  toggleGroup: (key: string) => void;
  expandedPoolId: () => string | null;
  setExpandedPoolId: (value: string | null) => void;
  nodeOnlineByLabel: () => Map<string, boolean>;
  highlightedRecordId: () => string | null;
  getRecordAlertState: (recordId: string) => StorageAlertRowState;
  isLoadingPools: () => boolean;
};

export const StorageContentCard: Component<StorageContentCardProps> = (props) => {
  const model = useStorageContentCardModel({
    view: props.view,
    selectedNodeId: props.selectedNodeId,
  });

  return (
    <Card padding="none" tone="card" class="overflow-hidden">
      <div class={STORAGE_CONTENT_CARD_HEADER_CLASS}>
        {model.heading()}
      </div>
      <Show when={model.showDisks()}>
        <div class={STORAGE_CONTENT_CARD_BODY_CLASS}>
          <DiskList
            disks={props.physicalDisks()}
            nodes={props.nodes()}
            selectedNode={model.selectedDiskNodeId()}
            searchTerm={props.search()}
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
        />
      </Show>
    </Card>
  );
};

export default StorageContentCard;
