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
import { formatRelativeTime } from '@/utils/format';
import type { ConnectionRow, ConnectionStatus } from './connectionsTableModel';

interface ConnectionsTableProps {
  rows: Accessor<readonly ConnectionRow[]>;
  onAddSystem?: () => void;
}

const STATUS_DOT_CLASS: Record<ConnectionStatus, string> = {
  reporting: 'bg-emerald-500',
  pending: 'bg-amber-400',
  offline: 'bg-slate-400',
  error: 'bg-rose-500',
  unknown: 'bg-slate-300',
};

export const ConnectionsTable: Component<ConnectionsTableProps> = (props) => {
  return (
    <Card padding="none" tone="card" class="rounded-md">
      <div class="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
        <div>
          <h3 class="text-base font-semibold text-base-content">Connections</h3>
          <p class="text-xs text-muted">
            Every system Pulse is configured to monitor, regardless of how it reports.
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
            No systems connected yet. Use
            <span class="mx-1 font-medium text-base-content">+ Add a system</span>
            to connect your first one.
          </div>
        }
      >
        <div class="overflow-auto">
          <Table class="min-w-[max-content] w-full divide-y divide-border text-sm">
            <TableHeader class="bg-surface-alt">
              <TableRow>
                <TableHead class="py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Name
                </TableHead>
                <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Kind
                </TableHead>
                <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Method
                </TableHead>
                <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Status
                </TableHead>
                <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Last reported
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border bg-surface">
              <For each={props.rows()}>
                {(r) => (
                  <TableRow class="even:bg-surface-alt">
                    <TableCell class="py-2.5 pl-4 pr-3">
                      <div class="font-medium text-base-content">{r.name}</div>
                      <Show when={r.host && r.host !== r.name}>
                        <div class="text-xs text-muted">{r.host}</div>
                      </Show>
                    </TableCell>
                    <TableCell class="px-3 py-2.5 text-base-content">{r.kindLabel}</TableCell>
                    <TableCell class="px-3 py-2.5 text-base-content">{r.methodLabel}</TableCell>
                    <TableCell class="px-3 py-2.5">
                      <span class="inline-flex items-center gap-2">
                        <span
                          aria-hidden="true"
                          class={`inline-block h-2 w-2 rounded-full ${STATUS_DOT_CLASS[r.status]}`}
                        />
                        <span class="text-base-content">{r.statusLabel}</span>
                      </span>
                    </TableCell>
                    <TableCell class="px-3 py-2.5 text-muted">
                      {formatRelativeTime(r.lastReportedMs, { emptyText: '—' })}
                    </TableCell>
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
