import { Component, For, Show, type Accessor } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import type { ConnectionRow } from './connectionsTableModel';

interface ConnectionsTableProps {
  rows: Accessor<readonly ConnectionRow[]>;
  onAddSystem?: () => void;
  onManageRow?: (row: ConnectionRow) => void;
}

export const ConnectionsTable: Component<ConnectionsTableProps> = (props) => {
  return (
    <Card padding="none" tone="card" class="rounded-md">
      <div class="flex flex-col gap-3 border-b border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 class="text-base font-semibold text-base-content">Connections and inventory</h3>
          <p class="text-xs text-muted">
            Configured platform connections, active reporting items, and ignored systems in one
            ledger.
          </p>
        </div>
        <Show when={props.onAddSystem}>
          <button
            type="button"
            onClick={props.onAddSystem}
            class="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
          >
            + Add a system
          </button>
        </Show>
      </div>

      <Show
        when={props.rows().length > 0}
        fallback={
          <div class="px-4 py-10 text-center text-sm text-muted">
            Nothing is configured or reporting yet. Use
            <span class="mx-1 font-medium text-base-content">+ Add a system</span>
            to connect the first one.
          </div>
        }
      >
        <div class="overflow-auto">
          <Table class="min-w-[1040px] w-full divide-y divide-border text-sm">
            <TableHeader class="bg-surface-alt">
              <TableRow>
                <TableHead class="py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  System
                </TableHead>
                <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Coverage
                </TableHead>
                <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Collection
                </TableHead>
                <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Status
                </TableHead>
                <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Last activity
                </TableHead>
                <Show when={props.onManageRow}>
                  <TableHead class="px-4 py-2 text-right text-xs font-semibold uppercase tracking-wide text-muted">
                    Manage
                  </TableHead>
                </Show>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border bg-surface">
              <For each={props.rows()}>
                {(row) => (
                  <TableRow class="even:bg-surface-alt">
                    <TableCell class="py-3 pl-4 pr-3 align-top">
                      <div class="min-w-0 space-y-1">
                        <div class="font-medium text-base-content">{row.name}</div>
                        <Show when={row.host}>
                          <div class="truncate text-xs text-muted">{row.host}</div>
                        </Show>
                        <div class="text-xs text-muted">{row.subtitle}</div>
                      </div>
                    </TableCell>

                    <TableCell class="px-3 py-3 align-top">
                      <div class="flex flex-wrap gap-1.5">
                        <For each={row.coverageLabels}>
                          {(label) => (
                            <span class="inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-xs font-medium text-base-content">
                              {label}
                            </span>
                          )}
                        </For>
                      </div>
                    </TableCell>

                    <TableCell class="px-3 py-3 align-top text-base-content">
                      {row.collectionLabel}
                    </TableCell>

                    <TableCell class="px-3 py-3 align-top">
                      <span
                        class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${row.statusClassName}`}
                      >
                        {row.statusLabel}
                      </span>
                    </TableCell>

                    <TableCell class="px-3 py-3 align-top text-muted">
                      {row.lastActivityText}
                    </TableCell>

                    <Show when={props.onManageRow}>
                      <TableCell class="px-4 py-3 align-top text-right">
                        <button
                          type="button"
                          onClick={() => props.onManageRow?.(row)}
                          class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                        >
                          {row.manageLabel}
                        </button>
                      </TableCell>
                    </Show>
                  </TableRow>
                )}
              </For>
            </TableBody>
          </Table>
        </div>
      </Show>
    </Card>
  );
};
