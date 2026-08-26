import { For, Show } from 'solid-js';

import { TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import { getTableSortIndicator } from '@/components/shared/tableSortPresentation';
import {
  getPlatformColumnAlign,
  type PlatformTableColumnKind,
} from '@/features/platformPage/columnAlignment';
import type { PlatformTableCellAlign } from '@/features/platformPage/sharedPlatformPage';

import { getGuestColumnStyle } from './guestRowModel';
import { shouldEngageColumnResize, WORKLOAD_COLUMN_MIN_WIDTH } from './workloadColumnWidths';
import type { WorkloadsMemoryDisplayBasis } from './workloadsFilterModel';
import type { WorkloadsState, WorkloadSortKey } from './useWorkloadsState';

type WorkloadTableHeaderProps = Pick<
  WorkloadsState,
  | 'handleSort'
  | 'isMobile'
  | 'sortDirection'
  | 'sortKey'
  | 'visibleColumns'
  | 'workloadMemoryDisplayBasis'
  | 'workloadTableLayoutMode'
  | 'workloadTableVisibleColumnIds'
  | 'workloadTableVisibleColumns'
  | 'workloadColumnWidths'
  | 'workloadManualColumnSizingSupported'
  | 'beginWorkloadColumnResize'
  | 'previewWorkloadColumnWidth'
  | 'commitWorkloadColumnResize'
  | 'cancelWorkloadColumnResize'
  | 'clearWorkloadColumnWidth'
>;

export const getWorkloadColumnHeaderLabel = (
  columnId: string,
  defaultLabel: string,
  memoryDisplayBasis: WorkloadsMemoryDisplayBasis,
  compact = false,
): string => {
  if (compact && columnId === 'availability') return 'Up';
  if (compact && columnId === 'memory') return 'Mem';
  if (compact && columnId === 'info') return 'ID';
  if (compact && columnId === 'uptime') return 'Age';
  return columnId === 'memory' && memoryDisplayBasis === 'host'
    ? `${defaultLabel} · Host`
    : defaultLabel;
};

// Canonical alignment per column kind (see
// frontend-modern/src/features/platformPage/columnAlignment.ts).
// 'name' is forced to left because the first column is always the
// primary identifier regardless of how the column model labels it.
// Unknown / unset kinds fall back to text → left.
const CENTERED_WORKLOAD_COLUMN_IDS = new Set(['info', 'vmid', 'netIo', 'diskIo']);

export const getWorkloadColumnHeaderAlign = (
  columnId: string,
  kind: PlatformTableColumnKind | undefined,
  isFirst: boolean,
): PlatformTableCellAlign => {
  const effectiveKind: PlatformTableColumnKind = isFirst ? 'name' : (kind ?? 'text');
  if (CENTERED_WORKLOAD_COLUMN_IDS.has(columnId)) return 'center';
  return getPlatformColumnAlign(effectiveKind);
};

const resolveAlignClasses = (
  columnId: string,
  kind: PlatformTableColumnKind | undefined,
  isFirst: boolean,
): { textAlign: string; flexJustify: string } => {
  const align = getWorkloadColumnHeaderAlign(columnId, kind, isFirst);
  if (align === 'right') {
    return { textAlign: 'text-right', flexJustify: 'justify-end' };
  }
  if (align === 'center') {
    return { textAlign: 'text-center', flexJustify: 'justify-center' };
  }
  return { textAlign: 'text-left', flexJustify: 'justify-start' };
};

const measureRenderedColumnWidths = (headerCell: HTMLElement): Record<string, number> => {
  const widths: Record<string, number> = {};
  const row = headerCell.closest('tr');
  if (!row) return widths;
  for (const cell of Array.from(row.querySelectorAll<HTMLElement>('th[data-workload-col]'))) {
    const columnId = cell.dataset.workloadCol;
    if (columnId) widths[columnId] = cell.getBoundingClientRect().width;
  }
  return widths;
};

export function WorkloadTableHeader(props: WorkloadTableHeaderProps) {
  return (
    <TableHeader>
      <TableRow class="bg-surface-alt text-muted border-b border-border">
        <For each={props.workloadTableVisibleColumns()}>
          {(col) => {
            const isFirst = () => col.id === props.visibleColumns()[0]?.id;
            const alignClasses = () => resolveAlignClasses(col.id, col.kind, isFirst());
            const sortKeyForCol = col.sortKey as WorkloadSortKey | undefined;
            const isSortable = !!sortKeyForCol;
            const isSorted = () => sortKeyForCol && props.sortKey() === sortKeyForCol;
            const usesCompactHeader = () =>
              props.isMobile() ||
              props.workloadTableLayoutMode() === 'narrow' ||
              props.workloadTableLayoutMode() === 'phone' ||
              props.workloadTableLayoutMode() === 'mobile';
            const label = () =>
              getWorkloadColumnHeaderLabel(
                col.id,
                col.label,
                props.workloadMemoryDisplayBasis(),
                usesCompactHeader(),
              );
            const isResizable = () => props.workloadManualColumnSizingSupported();
            let headerCell: HTMLTableCellElement | undefined;
            let dragPointerId: number | null = null;
            let dragStartX = 0;
            let dragStartWidth = 0;
            let dragMeasuredWidths: Record<string, number> = {};
            let dragEngaged = false;

            const endDrag = (handle: HTMLElement): void => {
              if (dragPointerId === null) return;
              if (handle.hasPointerCapture(dragPointerId))
                handle.releasePointerCapture(dragPointerId);
              dragPointerId = null;
            };
            const onPointerDown = (event: PointerEvent & { currentTarget: HTMLElement }): void => {
              if (!isResizable() || event.button !== 0 || !headerCell) return;
              event.preventDefault();
              event.stopPropagation();
              dragPointerId = event.pointerId;
              dragStartX = event.clientX;
              dragStartWidth = headerCell.getBoundingClientRect().width;
              dragMeasuredWidths = measureRenderedColumnWidths(headerCell);
              dragEngaged = false;
              event.currentTarget.setPointerCapture(event.pointerId);
            };
            const onPointerMove = (event: PointerEvent & { currentTarget: HTMLElement }): void => {
              if (dragPointerId === null || event.pointerId !== dragPointerId) return;
              const delta = event.clientX - dragStartX;
              if (!dragEngaged) {
                if (!shouldEngageColumnResize(delta, false)) return;
                dragEngaged = true;
                props.beginWorkloadColumnResize(dragMeasuredWidths);
              }
              event.preventDefault();
              props.previewWorkloadColumnWidth(
                col.id,
                Math.max(WORKLOAD_COLUMN_MIN_WIDTH, dragStartWidth + delta),
              );
            };
            const onPointerUp = (event: PointerEvent & { currentTarget: HTMLElement }): void => {
              if (dragPointerId === null || event.pointerId !== dragPointerId) return;
              event.preventDefault();
              event.stopPropagation();
              const engaged = dragEngaged;
              dragEngaged = false;
              endDrag(event.currentTarget);
              if (engaged) props.commitWorkloadColumnResize();
            };
            const onPointerCancel = (
              event: PointerEvent & { currentTarget: HTMLElement },
            ): void => {
              if (dragPointerId === null || event.pointerId !== dragPointerId) return;
              dragEngaged = false;
              endDrag(event.currentTarget);
              props.cancelWorkloadColumnResize();
            };

            return (
              <TableHead
                ref={headerCell}
                class={`relative py-0.5 text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap
 ${isFirst() ? 'pl-2 sm:pl-3 pr-1.5 sm:pr-2' : 'px-1.5 sm:px-2'} ${alignClasses().textAlign} align-middle
 ${isSortable ? 'cursor-pointer hover:bg-surface-hover' : ''}`}
                data-workload-col={col.id}
                style={getGuestColumnStyle(
                  col.id,
                  props.isMobile(),
                  props.workloadTableLayoutMode(),
                  props.workloadTableVisibleColumnIds(),
                  props.workloadColumnWidths(),
                )}
                onClick={() => isSortable && props.handleSort(sortKeyForCol!)}
                title={
                  col.id === 'memory' && props.workloadMemoryDisplayBasis() === 'host'
                    ? 'Memory as a percentage of host capacity'
                    : col.icon
                      ? col.label
                      : undefined
                }
              >
                <div class={`flex min-h-[14px] items-center gap-0.5 ${alignClasses().flexJustify}`}>
                  {col.icon && !(usesCompactHeader() && col.id === 'uptime') ? (
                    <>
                      <span class="flex items-center" aria-hidden="true">
                        {col.icon}
                      </span>
                      <span class="sr-only">{label()}</span>
                    </>
                  ) : (
                    label()
                  )}
                  {getTableSortIndicator(Boolean(isSorted()), props.sortDirection())}
                </div>
                <Show when={isResizable()}>
                  <span
                    class="workload-col-resizer"
                    data-workload-col-resizer={col.id}
                    role="presentation"
                    aria-hidden="true"
                    title={`Drag to resize · double-click to reset ${col.label}`}
                    onPointerDown={onPointerDown}
                    onPointerMove={onPointerMove}
                    onPointerUp={onPointerUp}
                    onPointerCancel={onPointerCancel}
                    onDblClick={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      props.clearWorkloadColumnWidth(col.id);
                    }}
                    onClick={(event) => event.stopPropagation()}
                  />
                </Show>
              </TableHead>
            );
          }}
        </For>
      </TableRow>
    </TableHeader>
  );
}
