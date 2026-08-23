import { Component, For, Show, createMemo } from 'solid-js';
import ArrowDownIcon from 'lucide-solid/icons/arrow-down';
import ArrowUpIcon from 'lucide-solid/icons/arrow-up';
import ArrowUpDownIcon from 'lucide-solid/icons/arrow-up-down';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { getPlatformTableHeadClassForKind } from '@/features/platformPage/sharedPlatformPage';
import {
  getStoragePoolColumnWidthPercent,
  getStoragePoolTableColumns,
  getStoragePoolTableLayoutModeForContainer,
  getStorageEmptyStateMessage,
  getStorageLoadingMessage,
  STORAGE_POOLS_BODY_CLASS,
  STORAGE_POOLS_EMPTY_STATE_CLASS,
  STORAGE_POOLS_HEADER_ROW_CLASS,
  STORAGE_POOLS_LOADING_STATE_CLASS,
  STORAGE_POOLS_TABLE_CLASS,
  isStoragePoolColumnVisible,
} from '@/features/storageBackups/storagePagePresentation';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
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
import { useStoragePoolsTableWindowing } from './useStoragePoolsTableWindowing';

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
  const tableWidth = useObservedElementWidth();
  const layoutMode = createMemo(() =>
    getStoragePoolTableLayoutModeForContainer(tableWidth.width() ?? 0),
  );
  const columnClass = (
    baseClass: string,
    columnId: Parameters<typeof isStoragePoolColumnVisible>[1],
    visibleClass: 'table-column' | 'table-cell',
  ) =>
    `${baseClass} ${isStoragePoolColumnVisible(layoutMode(), columnId) ? visibleClass : 'hidden'}`.trim();
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
  const tableWindow = useStoragePoolsTableWindowing({
    groups: model.groups,
    expandedPoolId: () => props.expandedPoolId,
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
          <Table
            class={STORAGE_POOLS_TABLE_CLASS}
            wrapperRef={tableWidth.setElement}
            data-storage-table="pools"
            data-storage-layout={layoutMode()}
          >
            <colgroup>
              <For each={getStoragePoolTableColumns(props.storageGrowthColumnLabel)}>
                {(column) => (
                  <col
                    class={columnClass(column.colClassName, column.id, 'table-column')}
                    style={{
                      width: `${getStoragePoolColumnWidthPercent(layoutMode(), column.id)}%`,
                    }}
                    data-storage-column={column.id}
                  />
                )}
              </For>
            </colgroup>
            <TableHeader>
              <TableRow class={STORAGE_POOLS_HEADER_ROW_CLASS}>
                <For each={getStoragePoolTableColumns(props.storageGrowthColumnLabel)}>
                  {(column) => (
                    <TableHead
                      class={columnClass(
                        `${getPlatformTableHeadClassForKind(column.kind)} ${column.className}`,
                        column.id,
                        'table-cell',
                      )}
                      data-storage-column={column.id}
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
                        <Show
                          when={layoutMode() === 'full'}
                          fallback={<span class="min-w-0 truncate">{column.compactLabel}</span>}
                        >
                          <span class="min-w-0 truncate">{column.label}</span>
                        </Show>
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
            <TableBody ref={tableWindow.setBodyRef} class={STORAGE_POOLS_BODY_CLASS}>
              <Show when={tableWindow.topSpacerHeight() > 0}>
                <TableRow aria-hidden="true" class="h-0 !border-0">
                  <TableCell colspan={99} class="h-0 !border-0 !p-0 leading-[0]">
                    <svg
                      aria-hidden="true"
                      width="1"
                      height={String(tableWindow.topSpacerHeight())}
                      class="pointer-events-none block w-px"
                    />
                  </TableCell>
                </TableRow>
              </Show>
              <For each={tableWindow.visibleItems()}>
                {(item) => {
                  if (item.kind === 'group') {
                    const groupSummaryScope = buildStorageSummaryGroupScope(
                      item.group,
                      props.groupBy,
                    );
                    return (
                      <StorageGroupRow
                        group={item.group}
                        groupBy={props.groupBy}
                        expanded={item.group.expanded}
                        onToggle={() => props.toggleGroup(item.group.key)}
                        summaryGroupScope={groupSummaryScope}
                        summaryActive={props.activeSummaryGroupScope?.id === groupSummaryScope?.id}
                        summaryFocused={props.focusedSummaryGroupId === groupSummaryScope?.id}
                        onFocusChange={props.onGroupFocusChange}
                        onHoverChange={props.onGroupHoverChange}
                      />
                    );
                  }

                  const record = item.record;
                  const metricResourceId = resolveStorageRecordMetricResourceId(record);
                  const rowModel = createMemo(() => model.buildRowModel(record.id, record));
                  return (
                    <StoragePoolRow
                      layoutMode={layoutMode()}
                      record={record}
                      growthDelta={props.storageGrowthBySeriesId.get(metricResourceId) ?? null}
                      summarySeriesId={metricResourceId}
                      expanded={rowModel().expanded}
                      summaryHighlighted={props.highlightedSummarySeriesId === metricResourceId}
                      summaryGroupMemberState={resolveSummaryGroupMemberInteractionState({
                        seriesId: metricResourceId,
                        hoveredGroupScope: props.hoveredSummaryGroupScope,
                        focusedGroupScope: props.focusedSummaryGroupScope,
                      })}
                      onToggleExpand={() => model.togglePool(record.id)}
                      onHoverChange={props.onHoverChange}
                      rowClass={rowModel().rowClass}
                      physicalDisks={props.physicalDisks}
                      alertDataAttrs={rowModel().alertDataAttrs}
                    />
                  );
                }}
              </For>
              <Show when={tableWindow.bottomSpacerHeight() > 0}>
                <TableRow aria-hidden="true" class="h-0 !border-0">
                  <TableCell colspan={99} class="h-0 !border-0 !p-0 leading-[0]">
                    <svg
                      aria-hidden="true"
                      width="1"
                      height={String(tableWindow.bottomSpacerHeight())}
                      class="pointer-events-none block w-px"
                    />
                  </TableCell>
                </TableRow>
              </Show>
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
