import { For } from 'solid-js';

import { TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import {
  getPlatformColumnAlign,
  type PlatformTableColumnKind,
} from '@/features/platformPage/columnAlignment';

import { getGuestColumnStyle } from './guestRowModel';
import type { WorkloadsState, WorkloadSortKey } from './useWorkloadsState';

type WorkloadTableHeaderProps = Pick<
  WorkloadsState,
  | 'handleSort'
  | 'isMobile'
  | 'sortDirection'
  | 'sortKey'
  | 'visibleColumns'
  | 'workloadTableLayoutMode'
  | 'workloadTableVisibleColumnIds'
  | 'workloadTableVisibleColumns'
>;

// Canonical alignment per column kind (see
// frontend-modern/src/features/platformPage/columnAlignment.ts).
// 'name' is forced to left because the first column is always the
// primary identifier regardless of how the column model labels it.
// Unknown / unset kinds fall back to text → left.
const resolveAlignClasses = (
  kind: PlatformTableColumnKind | undefined,
  isFirst: boolean,
): { textAlign: string; flexJustify: string } => {
  const effectiveKind: PlatformTableColumnKind = isFirst ? 'name' : (kind ?? 'text');
  const align = getPlatformColumnAlign(effectiveKind);
  if (align === 'right') {
    return { textAlign: 'text-right', flexJustify: 'justify-end' };
  }
  if (align === 'center') {
    return { textAlign: 'text-center', flexJustify: 'justify-center' };
  }
  return { textAlign: 'text-left', flexJustify: 'justify-start' };
};

export function WorkloadTableHeader(props: WorkloadTableHeaderProps) {
  return (
    <TableHeader>
      <TableRow class="bg-surface-alt text-muted border-b border-border">
        <For each={props.workloadTableVisibleColumns()}>
          {(col) => {
            const isFirst = () => col.id === props.visibleColumns()[0]?.id;
            const alignClasses = () => resolveAlignClasses(col.kind, isFirst());
            const sortKeyForCol = col.sortKey as WorkloadSortKey | undefined;
            const isSortable = !!sortKeyForCol;
            const isSorted = () => sortKeyForCol && props.sortKey() === sortKeyForCol;

            return (
              <TableHead
                class={`py-0.5 text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap
 ${isFirst() ? 'pl-2 sm:pl-3 pr-1.5 sm:pr-2' : 'px-1.5 sm:px-2'} ${alignClasses().textAlign} align-middle
 ${isSortable ? 'cursor-pointer hover:bg-surface-hover' : ''}`}
                data-workload-col={col.id}
                style={getGuestColumnStyle(
                  col.id,
                  props.isMobile(),
                  props.workloadTableLayoutMode(),
                  props.workloadTableVisibleColumnIds(),
                )}
                onClick={() => isSortable && props.handleSort(sortKeyForCol!)}
                title={col.icon ? col.label : undefined}
              >
                <div class={`flex min-h-[14px] items-center gap-0.5 ${alignClasses().flexJustify}`}>
                  {col.icon ? (
                    <>
                      <span class="flex items-center" aria-hidden="true">
                        {col.icon}
                      </span>
                      <span class="sr-only">{col.label}</span>
                    </>
                  ) : (
                    col.label
                  )}
                  {isSorted() && (props.sortDirection() === 'asc' ? ' ▲' : ' ▼')}
                </div>
              </TableHead>
            );
          }}
        </For>
      </TableRow>
    </TableHeader>
  );
}
