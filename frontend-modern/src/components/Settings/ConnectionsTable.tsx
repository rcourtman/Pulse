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
import type { InfrastructureSystemRow } from './connectionsTableModel';

export interface ConnectionsTableHeaderAction {
  label: string;
  onSelect: () => void;
  tone?: 'primary' | 'secondary';
}

interface ConnectionsTableProps {
  rows: Accessor<readonly InfrastructureSystemRow[]>;
  headerActions?: readonly ConnectionsTableHeaderAction[];
  onManageRow?: (row: InfrastructureSystemRow) => void;
}

export const ConnectionsTable: Component<ConnectionsTableProps> = (props) => {
  return (
    <Card padding="none" tone="card" class="rounded-md">
      <div class="flex flex-col gap-3 border-b border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 class="text-base font-semibold text-base-content">Monitored systems</h3>
          <p class="text-xs text-muted">One row per top-level monitored system.</p>
        </div>
        <Show when={(props.headerActions?.length ?? 0) > 0}>
          <div class="flex flex-wrap items-center gap-2">
            <For each={props.headerActions ?? []}>
              {(action) => (
                <button
                  type="button"
                  onClick={action.onSelect}
                  class={
                    action.tone === 'primary'
                      ? 'inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700'
                      : 'inline-flex items-center rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover'
                  }
                >
                  {action.label}
                </button>
              )}
            </For>
          </div>
        </Show>
      </div>

      <Show
        when={props.rows().length > 0}
        fallback={
          <div class="px-4 py-10 text-center text-sm text-muted">No monitored systems yet.</div>
        }
      >
        <Table class="w-full table-fixed divide-y divide-border text-sm !whitespace-normal">
          <TableHeader class="bg-surface-alt">
            <TableRow>
              <TableHead class="w-[30%] py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[20%]">
                System
              </TableHead>
              <TableHead class="w-[36%] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[30%]">
                Coverage
              </TableHead>
              <TableHead class="hidden w-[14%] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:table-cell">
                Collection
              </TableHead>
              <TableHead class="w-[16%] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[10%]">
                Status
              </TableHead>
              <TableHead class="hidden w-[14%] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:table-cell">
                Last activity
              </TableHead>
              <Show when={props.onManageRow}>
                <TableHead class="w-[18%] px-4 py-2 text-right text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[12%]">
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
                      <div class="break-words font-medium text-base-content">{row.name}</div>
                      <Show when={row.host}>
                        <div class="break-words text-xs text-muted">{row.host}</div>
                      </Show>
                      <Show when={row.subtitle}>
                        <div class="break-words text-xs text-muted">{row.subtitle}</div>
                      </Show>
                      <div class="text-xs text-muted 2xl:hidden">{row.collectionLabel}</div>
                    </div>
                  </TableCell>

                  <TableCell class="px-3 py-3 align-top">
                    <div class="flex flex-wrap gap-1.5">
                      <For each={row.coverageLabels}>
                        {(label) => (
                          <span class="inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-xs font-medium text-base-content whitespace-nowrap">
                            {label}
                          </span>
                        )}
                      </For>
                    </div>
                  </TableCell>

                  <TableCell class="hidden px-3 py-3 align-top whitespace-nowrap text-base-content 2xl:table-cell">
                    {row.collectionLabel}
                  </TableCell>

                  <TableCell class="px-3 py-3 align-top">
                    <div class="space-y-1">
                      <span
                        class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap ${row.statusClassName}`}
                      >
                        {row.statusLabel}
                      </span>
                      <div class="text-xs text-muted 2xl:hidden">{row.lastActivityText}</div>
                    </div>
                  </TableCell>

                  <TableCell class="hidden px-3 py-3 align-top whitespace-nowrap text-muted 2xl:table-cell">
                    {row.lastActivityText}
                  </TableCell>

                  <Show when={props.onManageRow}>
                    <TableCell class="px-4 py-3 align-top text-right">
                      <button
                        type="button"
                        onClick={() => props.onManageRow?.(row)}
                        class="inline-flex min-h-10 w-full items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content whitespace-nowrap transition-colors hover:bg-surface-hover sm:min-h-9 2xl:w-auto"
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
      </Show>
    </Card>
  );
};
