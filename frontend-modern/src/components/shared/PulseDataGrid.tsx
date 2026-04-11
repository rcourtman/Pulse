import { For, Show, createMemo, splitProps } from 'solid-js';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import {
  getPulseDataGridAlignClass,
  getPulseDataGridWidthAttr,
  isPulseDataGridInteractiveTarget,
  type PulseDataGridProps,
} from './pulseDataGridModel';
import { usePulseDataGridState } from './usePulseDataGridState';

export type { PulseDataGridProps, TableColumn } from './pulseDataGridModel';

/**
 * A standardized, responsive datagrid component for Pulse.
 *
 * Enforces unified table header styling (uppercase, tracking, specific padding/font size),
 * modern row hover effects, and proper mobile responsive horizontal scrolling behavior.
 */
export function PulseDataGrid<T>(props: PulseDataGridProps<T>) {
  const [local, _] = splitProps(props, [
    'data',
    'columns',
    'keyExtractor',
    'onRowClick',
    'emptyState',
    'isLoading',
    'isRowExpanded',
    'expandedRender',
    'desktopMinWidth',
    'mobileMinWidth',
    'class',
  ]);

  const grid = usePulseDataGridState(local);

  return (
    <div class={`overflow-hidden rounded-md border border-border bg-surface ${local.class || ''}`}>
      <div class="overflow-x-auto touch-scroll scrollbar-hide">
        <Table class="w-full border-collapse" width={grid.effectiveWidthAttr()}>
          <TableHeader class="bg-surface-alt border-b border-border">
            <TableRow>
              <For each={local.columns}>
                {(col) => (
                  <TableHead
                    class={`
                                            px-3 sm:px-4 py-2.5 
                                            text-[11px] sm:text-xs font-semibold uppercase tracking-wider whitespace-nowrap text-muted
                                            ${getPulseDataGridAlignClass(col.align)}
                                            ${col.hiddenOnMobile ? 'hidden sm:table-cell' : ''}
                                        `}
                    width={getPulseDataGridWidthAttr(col.width)}
                  >
                    {col.label}
                  </TableHead>
                )}
              </For>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border transition-colors">
            <Show when={!local.isLoading && local.data.length > 0}>
              <For each={grid.stableRows}>
                {(stableRow) => {
                  const row = () => stableRow.value;
                  const expanded = createMemo(() => local.isRowExpanded?.(row()));

                  return (
                    <>
                      <TableRow
                        class={`
                                                    group transition-colors duration-150 animate-enter
                                                    ${
                                                      local.onRowClick
                                                        ? 'cursor-pointer hover:bg-blue-50 dark:hover:bg-blue-900'
                                                        : 'hover:bg-surface-hover'
                                                    }
                                                `}
                        onClick={(event) => {
                          if (isPulseDataGridInteractiveTarget(event.target)) {
                            return;
                          }
                          local.onRowClick?.(row());
                        }}
                      >
                        <For each={local.columns}>
                          {(col) => (
                            <TableCell
                              class={`
                                                                px-3 sm:px-4 py-2 sm:py-3.5 
                                                                text-sm text-base-content align-middle
                                                                ${getPulseDataGridAlignClass(col.align)}
                                                                ${col.hiddenOnMobile ? 'hidden sm:table-cell' : ''}
                                                            `}
                            >
                              <Show
                                when={col.render}
                                fallback={<span>{(row() as any)[col.key]}</span>}
                              >
                                {col.render!(row())}
                              </Show>
                            </TableCell>
                          )}
                        </For>
                      </TableRow>
                      <Show when={expanded() && local.expandedRender}>
                        <TableRow
                          class={
                            local.onRowClick
                              ? 'bg-surface-alt hover:bg-blue-50 dark:hover:bg-blue-900'
                              : 'bg-surface-alt hover:bg-surface-hover'
                          }
                        >
                          <TableCell colspan={local.columns.length} class="px-0 py-0 border-t-0">
                            {local.expandedRender!(row())}
                          </TableCell>
                        </TableRow>
                      </Show>
                    </>
                  );
                }}
              </For>
            </Show>

            <Show when={local.isLoading}>
              <TableRow>
                <TableCell
                  colspan={local.columns.length}
                  class="px-4 py-8 text-center text-sm text-slate-500"
                >
                  <div class="flex items-center justify-center gap-2">
                    <div class="w-4 h-4 rounded-full border-2 border-slate-300 border-t-blue-600 animate-spin"></div>
                    Loading...
                  </div>
                </TableCell>
              </TableRow>
            </Show>

            <Show when={!local.isLoading && local.data.length === 0}>
              <TableRow>
                <TableCell
                  colspan={local.columns.length}
                  class="px-4 py-8 text-center text-sm text-slate-500 italic bg-surface-alt"
                >
                  <Show when={local.emptyState} fallback="No items available.">
                    {local.emptyState}
                  </Show>
                </TableCell>
              </TableRow>
            </Show>
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
