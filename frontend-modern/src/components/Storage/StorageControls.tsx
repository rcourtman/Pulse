import { Component, For, JSX, Show } from 'solid-js';
import { Subtabs } from '@/components/shared/Subtabs';
import {
  STORAGE_CONTROLS_NODE_DIVIDER_CLASS,
  STORAGE_CONTROLS_NODE_SELECT_CLASS,
} from '@/features/storageBackups/storagePagePresentation';
import {
  StorageFilter,
  type StorageGroupByFilter,
  type StorageStatusFilter,
} from './StorageFilter';
import type { StorageView } from './storagePageState';
import type { StorageSortKey } from './useStorageModel';
import { DEFAULT_STORAGE_SORT_OPTIONS } from './storagePageState';
import { useStorageControlsModel } from './useStorageControlsModel';

type StorageControlsProps = {
  view: StorageView;
  onViewChange: (value: StorageView) => void;
  search: () => string;
  setSearch: (value: string) => void;
  groupBy?: () => StorageGroupByFilter;
  setGroupBy?: (value: StorageGroupByFilter) => void;
  sortKey: () => StorageSortKey;
  setSortKey: (value: StorageSortKey) => void;
  sortDirection: () => 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  sortDisabled?: boolean;
  statusFilter: () => StorageStatusFilter;
  setStatusFilter: (value: StorageStatusFilter) => void;
  sourceFilter: () => string;
  setSourceFilter: (value: string) => void;
  sourceOptions: Array<{ value: string; label: string }>;
  nodeFilterOptions: Array<{ value: string; label: string }>;
  selectedNodeId: () => string;
  setSelectedNodeId: (value: string) => void;
};

export const StorageControls: Component<StorageControlsProps> = (props) => {
  const model = useStorageControlsModel({
    selectedNodeId: props.selectedNodeId,
    setSelectedNodeId: props.setSelectedNodeId,
    onViewChange: props.onViewChange,
  });

  const leadingFilters = (): JSX.Element => (
    <>
      <select
        value={props.selectedNodeId()}
        onChange={(event) => model.handleNodeFilterChange(event.currentTarget.value)}
        class={STORAGE_CONTROLS_NODE_SELECT_CLASS}
        aria-label="Node"
      >
        <For each={props.nodeFilterOptions}>
          {(option) => <option value={option.value}>{option.label}</option>}
        </For>
      </select>
      <div class={STORAGE_CONTROLS_NODE_DIVIDER_CLASS}></div>
    </>
  );

  return (
    <>
      <Subtabs
        value={props.view}
        onChange={model.handleViewChange}
        ariaLabel="Storage view"
        tabs={model.viewTabs}
      />

      <StorageFilter
        search={props.search}
        setSearch={props.setSearch}
        groupBy={props.groupBy}
        setGroupBy={props.setGroupBy}
        sortKey={props.sortKey}
        setSortKey={props.setSortKey}
        sortDirection={props.sortDirection}
        setSortDirection={props.setSortDirection}
        sortOptions={DEFAULT_STORAGE_SORT_OPTIONS}
        sortDisabled={props.sortDisabled}
        statusFilter={props.statusFilter}
        setStatusFilter={props.setStatusFilter}
        sourceFilter={props.sourceFilter}
        setSourceFilter={props.setSourceFilter}
        sourceOptions={props.sourceOptions}
        leadingFilters={leadingFilters()}
      />
    </>
  );
};

export default StorageControls;
