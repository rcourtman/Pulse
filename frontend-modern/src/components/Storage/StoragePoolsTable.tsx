import { Component, For, Index, Show } from 'solid-js';
import ArrowDownIcon from 'lucide-solid/icons/arrow-down';
import ArrowUpIcon from 'lucide-solid/icons/arrow-up';
import ArrowUpDownIcon from 'lucide-solid/icons/arrow-up-down';
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import {
  getStoragePoolTableColumns,
  getStorageEmptyStateMessage,
  getStorageLoadingMessage,
  STORAGE_POOLS_BODY_CLASS,
  STORAGE_POOLS_EMPTY_STATE_CLASS,
  STORAGE_POOLS_HEADER_ROW_CLASS,
  STORAGE_POOLS_LOADING_STATE_CLASS,
  STORAGE_POOLS_TABLE_CLASS,
} from '@/features/storageBackups/storagePagePresentation';
import { resolveStorageRecordMetricResourceId } from '@/features/storageBackups/storageMetricsIdentity';
import type { StorageCapacityDeltaPresentation } from '@/features/storageBackups/storageCapacityDeltaPresentation';
import type { StorageAlertRowState } from '@/features/storageBackups/storageAlertState';
import type { Resource } from '@/types/resource';
import { StorageGroupRow } from './StorageGroupRow';
import { StoragePoolRow } from './StoragePoolRow';
import { getDefaultStorageSortDirection } from './storagePageState';
import type { StorageGroupedRecords, StorageGroupKey, StorageSortKey } from './useStorageModel';
import { useStoragePoolsTableModel } from './useStoragePoolsTableModel';
import type { SummarySeriesGroupScope } from '@/components/shared/summaryCardInteraction';
import { resolveSummaryGroupMemberInteractionState } from '@/components/shared/summaryCardInteraction';
import { buildStorageSummaryGroupScope } from './storageSummaryGroups';

type StoragePoolsTableProps = {
  groupedRecords: StorageGroupedRecords[];
  groupBy: StorageGroupKey;
  sortKey: StorageSortKey;
  setSortKey: (value: StorageSortKey) => void;
  sortDirection: 'asc' | 'desc';
  setSortDirection: (value: 'asc' | 'desc') => void;
  expandedGroups: Set<string>;
  toggleGroup: (key: string) => void;
  expandedPoolId: string | null;
  setExpandedPoolId: (value: string | null | ((current: string | null) => string | null)) => void;
  storageGrowthBySeriesId: Map<string, StorageCapacityDeltaPresentation>;
  storageGrowthColumnLabel: string;
  physicalDisks: Resource[];
  nodeOnlineByLabel: Map<string, boolean>;
  highlightedRecordId: string | null;
  getRecordAlertState: (recordId: string) => StorageAlertRowState;
  isLoading: boolean;
  activeSummaryGroupScope?: SummarySeriesGroupScope | null;
  hoveredSummaryGroupScope?: SummarySeriesGroupScope | null;
  focusedSummaryGroupScope?: SummarySeriesGroupScope | null;
  focusedSummaryGroupId?: string | null;
  onGroupFocusChange?: (scope: SummarySeriesGroupScope | null) => void;
  onGroupHoverChange?: (scope: SummarySeriesGroupScope | null) => void;
  highlightedSummarySeriesId?: string | null;
  onHoverChange?: (recordId: string | null) => void;
};

const STORAGE_POOL_HEADER_SORT_BUTTON_CLASS =
  'inline-flex min-w-0 max-w-full items-center gap-1 rounded-sm text-left outline-none transition-colors hover:text-base-content focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface';
const STORAGE_POOL_HEADER_SORT_ICON_CLASS = 'h-3 w-3 shrink-0';

const getNextStorageColumnSortDirection = (
  currentSortKey: StorageSortKey,
  currentSortDirection: 'asc' | 'desc',
  columnSortKey: StorageSortKey,
): 'asc' | 'desc' => {
  if (currentSortKey !== columnSortKey) {
    return getDefaultStorageSortDirection(columnSortKey);
  }
  return currentSortDirection === 'asc' ? 'desc' : 'asc';
};

const getStorageColumnSortButtonLabel = (
  label: string,
  isSorted: boolean,
  direction: 'asc' | 'desc',
): string => {
  if (!isSorted) return `Sort ${label} column`;
  return `Sort ${label} column ${direction === 'asc' ? 'descending' : 'ascending'}`;
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

  const handleSort = (sortKey: StorageSortKey) => {
    props.setSortDirection(
      getNextStorageColumnSortDirection(props.sortKey, props.sortDirection, sortKey),
    );
    props.setSortKey(sortKey);
  };

  return (
    <Show
      when={props.isLoading}
      fallback={
        <Show
          when={props.groupedRecords.length > 0}
          fallback={
            <div class={STORAGE_POOLS_EMPTY_STATE_CLASS}>{getStorageEmptyStateMessage()}</div>
          }
        >
          <Table class={STORAGE_POOLS_TABLE_CLASS}>
            <colgroup>
              <For each={getStoragePoolTableColumns(props.storageGrowthColumnLabel)}>
                {(column) => <col class={column.colClassName} />}
              </For>
            </colgroup>
            <TableHeader>
              <TableRow class={STORAGE_POOLS_HEADER_ROW_CLASS}>
                <For each={getStoragePoolTableColumns(props.storageGrowthColumnLabel)}>
                  {(column) => (
                    <TableHead
                      class={column.className}
                      aria-label={column.label}
                      aria-sort={
                        props.sortKey === column.sortKey
                          ? props.sortDirection === 'asc'
                            ? 'ascending'
                            : 'descending'
                          : undefined
                      }
                      title={column.label}
                    >
                      <button
                        type="button"
                        class={STORAGE_POOL_HEADER_SORT_BUTTON_CLASS}
                        onClick={() => handleSort(column.sortKey)}
                        aria-label={getStorageColumnSortButtonLabel(
                          column.label,
                          props.sortKey === column.sortKey,
                          props.sortDirection,
                        )}
                        title={getStorageColumnSortButtonLabel(
                          column.label,
                          props.sortKey === column.sortKey,
                          props.sortDirection,
                        )}
                      >
                        <span class="hidden min-w-0 truncate xl:inline">{column.label}</span>
                        <span class="min-w-0 truncate xl:hidden">{column.compactLabel}</span>
                        <Show
                          when={props.sortKey === column.sortKey}
                          fallback={
                            <ArrowUpDownIcon
                              aria-hidden="true"
                              class={`${STORAGE_POOL_HEADER_SORT_ICON_CLASS} text-muted/70`}
                            />
                          }
                        >
                          <Show
                            when={props.sortDirection === 'asc'}
                            fallback={
                              <ArrowDownIcon
                                aria-hidden="true"
                                class={`${STORAGE_POOL_HEADER_SORT_ICON_CLASS} text-base-content`}
                              />
                            }
                          >
                            <ArrowUpIcon
                              aria-hidden="true"
                              class={`${STORAGE_POOL_HEADER_SORT_ICON_CLASS} text-base-content`}
                            />
                          </Show>
                        </Show>
                      </button>
                    </TableHead>
                  )}
                </For>
              </TableRow>
            </TableHeader>
            <TableBody class={STORAGE_POOLS_BODY_CLASS}>
              <For each={model.groups()}>
                {(group) => (
                  <>
                    <Show when={group.showHeader}>
                      {(() => {
                        const groupSummaryScope = buildStorageSummaryGroupScope(
                          group,
                          props.groupBy,
                        );
                        return (
                          <StorageGroupRow
                            group={group}
                            groupBy={props.groupBy}
                            expanded={group.expanded}
                            onToggle={() => props.toggleGroup(group.key)}
                            summaryGroupScope={groupSummaryScope}
                            summaryActive={
                              props.activeSummaryGroupScope?.id === groupSummaryScope?.id
                            }
                            summaryFocused={props.focusedSummaryGroupId === groupSummaryScope?.id}
                            onFocusChange={props.onGroupFocusChange}
                            onHoverChange={props.onGroupHoverChange}
                          />
                        );
                      })()}
                    </Show>
                    <Show when={group.expanded}>
                      <Index each={group.items}>
                        {(record) => {
                          const rowModel = () => model.buildRowModel(record().id, record());

                          return (
                            <StoragePoolRow
                              record={record()}
                              growthDelta={
                                props.storageGrowthBySeriesId.get(
                                  resolveStorageRecordMetricResourceId(record()),
                                ) ?? null
                              }
                              summarySeriesId={resolveStorageRecordMetricResourceId(record())}
                              expanded={rowModel().expanded}
                              summaryHighlighted={
                                props.highlightedSummarySeriesId ===
                                resolveStorageRecordMetricResourceId(record())
                              }
                              summaryGroupMemberState={resolveSummaryGroupMemberInteractionState({
                                seriesId: resolveStorageRecordMetricResourceId(record()),
                                hoveredGroupScope: props.hoveredSummaryGroupScope,
                                focusedGroupScope: props.focusedSummaryGroupScope,
                              })}
                              onToggleExpand={() => model.togglePool(record().id)}
                              onHoverChange={props.onHoverChange}
                              rowClass={rowModel().rowClass}
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
        </Show>
      }
    >
      <div class={STORAGE_POOLS_LOADING_STATE_CLASS}>{getStorageLoadingMessage()}</div>
    </Show>
  );
};

export default StoragePoolsTable;
