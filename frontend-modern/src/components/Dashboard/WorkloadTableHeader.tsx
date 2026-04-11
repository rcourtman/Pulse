import { For } from 'solid-js';

import { TableHead, TableHeader, TableRow } from '@/components/shared/Table';

import type { DashboardState, WorkloadSortKey } from './useDashboardState';

type WorkloadTableHeaderProps = Pick<
  DashboardState,
  | 'handleSort'
  | 'isMobile'
  | 'mobileVisibleColumns'
  | 'sortDirection'
  | 'sortKey'
  | 'visibleColumns'
>;

export function WorkloadTableHeader(props: WorkloadTableHeaderProps) {
  return (
    <TableHeader>
      <TableRow class="bg-surface-alt text-muted border-b border-border">
        <For each={props.mobileVisibleColumns()}>
          {(col) => {
            const isFirst = () => col.id === props.visibleColumns()[0]?.id;
            const sortKeyForCol = col.sortKey as WorkloadSortKey | undefined;
            const isSortable = !!sortKeyForCol;
            const isSorted = () => sortKeyForCol && props.sortKey() === sortKeyForCol;

            return (
              <TableHead
                class={`py-0.5 text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap
 ${isFirst() ? 'pl-2 sm:pl-3 pr-1.5 sm:pr-2 text-left' : 'px-1.5 sm:px-2 text-center'} align-middle
 ${isSortable ? 'cursor-pointer hover:bg-surface-hover' : ''}`}
                data-workload-col={col.id}
                onClick={() => isSortable && props.handleSort(sortKeyForCol!)}
                title={col.icon ? col.label : undefined}
              >
                <div
                  class={`flex min-h-[14px] items-center gap-0.5 ${isFirst() ? 'justify-start' : 'justify-center'}`}
                >
                  {col.icon ? <span class="flex items-center">{col.icon}</span> : col.label}
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
