import { Component, Show } from 'solid-js';
import StorageControls from '@/components/Storage/StorageControls';
import type { StorageView } from './storagePageState';
import type { StorageGroupKey, StorageSortKey } from './useStorageModel';
import type { StorageGroupByFilter, StorageStatusFilter } from './StorageFilter';
import { useStoragePageControlsModel } from './useStoragePageControlsModel';

type StoragePageControlsProps = {
  kioskMode: () => boolean;
  view: () => StorageView;
  setView: (value: StorageView) => void;
  search: () => string;
  setSearch: (value: string) => void;
  groupBy: () => StorageGroupKey;
  setGroupBy: (value: StorageGroupKey) => void;
  sortKey: () => StorageSortKey;
  setSortKey: (value: StorageSortKey) => void;
  sortDirection: () => 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  statusFilter: () => StorageStatusFilter;
  setStatusFilter: (value: StorageStatusFilter) => void;
  sourceFilter: () => string;
  setSourceFilter: (value: string) => void;
  sourceOptions: Array<{ value: string; label: string }>;
  nodeFilterOptions: Array<{ value: string; label: string }>;
  selectedNodeId: () => string;
  setSelectedNodeId: (value: string) => void;
  storageFilterGroupBy: () => StorageGroupByFilter;
};

export const StoragePageControls: Component<StoragePageControlsProps> = (props) => {
  const model = useStoragePageControlsModel({
    kioskMode: props.kioskMode,
    view: props.view,
    setGroupBy: props.setGroupBy,
    setSortKey: props.setSortKey,
    storageFilterGroupBy: props.storageFilterGroupBy,
  });

  return (
    <Show when={model.showControls()}>
      <StorageControls
        view={props.view()}
        onViewChange={props.setView}
        search={props.search}
        setSearch={props.setSearch}
        groupBy={model.groupBy()}
        setGroupBy={model.setGroupBy()}
        sortKey={props.sortKey}
        setSortKey={model.setNormalizedSortKey}
        sortDirection={props.sortDirection}
        setSortDirection={props.setSortDirection}
        sortDisabled={model.sortDisabled()}
        statusFilter={props.statusFilter}
        setStatusFilter={props.setStatusFilter}
        sourceFilter={props.sourceFilter}
        setSourceFilter={props.setSourceFilter}
        sourceOptions={props.sourceOptions}
        nodeFilterOptions={props.nodeFilterOptions}
        selectedNodeId={props.selectedNodeId}
        setSelectedNodeId={props.setSelectedNodeId}
      />
    </Show>
  );
};

export default StoragePageControls;
