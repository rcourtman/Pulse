import { Component, For, Index, Show } from 'solid-js';
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import {
  getStorageEmptyStateMessage,
  getStorageLoadingMessage,
  STORAGE_POOLS_BODY_CLASS,
  STORAGE_POOLS_EMPTY_STATE_CLASS,
  STORAGE_POOLS_HEADER_ROW_CLASS,
  STORAGE_POOLS_LOADING_STATE_CLASS,
  STORAGE_POOLS_SCROLL_WRAP_CLASS,
  STORAGE_POOLS_TABLE_CLASS,
  STORAGE_POOL_TABLE_COLUMNS,
} from '@/features/storageBackups/storagePagePresentation';
import type { StorageAlertRowState } from '@/features/storageBackups/storageAlertState';
import type { Resource } from '@/types/resource';
import { StorageGroupRow } from './StorageGroupRow';
import { StoragePoolRow } from './StoragePoolRow';
import type { StorageGroupedRecords, StorageGroupKey } from './useStorageModel';
import { useStoragePoolsTableModel } from './useStoragePoolsTableModel';

type StoragePoolsTableProps = {
  groupedRecords: StorageGroupedRecords[];
  groupBy: StorageGroupKey;
  expandedGroups: Set<string>;
  toggleGroup: (key: string) => void;
  expandedPoolId: string | null;
  setExpandedPoolId: (value: string | null | ((current: string | null) => string | null)) => void;
  physicalDisks: Resource[];
  nodeOnlineByLabel: Map<string, boolean>;
  highlightedRecordId: string | null;
  getRecordAlertState: (recordId: string) => StorageAlertRowState;
  isLoading: boolean;
};

export const StoragePoolsTable: Component<StoragePoolsTableProps> = (props) => {
  const model = useStoragePoolsTableModel({
    groupedRecords: () => props.groupedRecords,
    groupBy: () => props.groupBy,
    expandedGroups: () => props.expandedGroups,
    expandedPoolId: () => props.expandedPoolId,
    highlightedRecordId: () => props.highlightedRecordId,
    nodeOnlineByLabel: () => props.nodeOnlineByLabel,
    getRecordAlertState: props.getRecordAlertState,
    setExpandedPoolId: props.setExpandedPoolId,
  });

  return (
    <Show
      when={props.isLoading}
      fallback={
        <Show
          when={props.groupedRecords.length > 0}
          fallback={<div class={STORAGE_POOLS_EMPTY_STATE_CLASS}>{getStorageEmptyStateMessage()}</div>}
        >
          <div class={STORAGE_POOLS_SCROLL_WRAP_CLASS}>
            <Table class={STORAGE_POOLS_TABLE_CLASS}>
              <TableHeader>
                <TableRow class={STORAGE_POOLS_HEADER_ROW_CLASS}>
                  <For each={STORAGE_POOL_TABLE_COLUMNS}>
                    {(column) => <TableHead class={column.className}>{column.label}</TableHead>}
                  </For>
                </TableRow>
              </TableHeader>
              <TableBody class={STORAGE_POOLS_BODY_CLASS}>
                <For each={model.groups()}>
                  {(group) => (
                    <>
                      <Show when={group.showHeader}>
                        <StorageGroupRow
                          group={group}
                          groupBy={props.groupBy}
                          expanded={group.expanded}
                          onToggle={() => props.toggleGroup(group.key)}
                        />
                      </Show>
                      <Show when={group.expanded}>
                        <Index each={group.items}>
                          {(record) => {
                            const rowModel = () => model.buildRowModel(record().id, record());

                            return (
                              <StoragePoolRow
                                record={record()}
                                expanded={rowModel().expanded}
                                onToggleExpand={() => model.togglePool(record().id)}
                                rowClass={rowModel().rowClass}
                                rowStyle={rowModel().rowStyle}
                                physicalDisks={props.physicalDisks}
                                alertDataAttrs={rowModel().alertDataAttrs}
                              />
                            );
                          }}
                        </Index>
                      </Show>
                    </>
                  )}
                </For>
              </TableBody>
            </Table>
          </div>
        </Show>
      }
    >
      <div class={STORAGE_POOLS_LOADING_STATE_CLASS}>{getStorageLoadingMessage()}</div>
    </Show>
  );
};

export default StoragePoolsTable;
