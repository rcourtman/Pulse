import {
  Component,
  createEffect,
  createMemo,
  createSignal,
  For,
  Show,
  type Accessor,
} from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import type { Connection } from '@/api/connections';
import {
  CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS,
  connectionsTableVisibilityState,
  fleetSignalClassName,
  type InfrastructureSystemRow,
  visibleConnectionsTableRows,
} from './connectionsTableModel';
import type { ConnectionRowActions } from './useConnectionRowActions';
import { getInfrastructureEmptyStateSummary } from '@/utils/infrastructureOnboardingPresentation';

export interface ConnectionsTableHeaderAction {
  label: string;
  onSelect: () => void;
  tone?: 'primary' | 'secondary';
}

export interface AgentUninstallCommands {
  linux: string;
  windows: string;
}

interface ConnectionsTableProps {
  rows: Accessor<readonly InfrastructureSystemRow[]>;
  headerActions?: readonly ConnectionsTableHeaderAction[];
  actions?: ConnectionRowActions;
  onEdit?: (connection: Connection) => void;
  agentUninstallCommands?: AgentUninstallCommands;
  onCopyText?: (text: string) => void;
}

const actionColumnClass =
  'w-[12%] px-4 py-2 text-right text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[14%]';

const inlineButtonClass =
  'inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';

const removeButtonClass =
  'inline-flex items-center rounded-md border border-rose-300 px-2.5 py-1 text-xs font-medium text-rose-700 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950';

const removeConfirmClass =
  'inline-flex items-center rounded-md bg-rose-600 px-2.5 py-1 text-xs font-medium text-white transition-colors hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60';

export const ConnectionsTable: Component<ConnectionsTableProps> = (props) => {
  const [visibleLimit, setVisibleLimit] = createSignal(CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS);
  const hasActions = () => Boolean(props.actions) || Boolean(props.onEdit);

  const colSpan = () => (hasActions() ? 6 : 5);
  const rows = createMemo(() => props.rows());
  const visibility = createMemo(() =>
    connectionsTableVisibilityState(rows().length, visibleLimit()),
  );
  const visibleRows = createMemo(() =>
    visibleConnectionsTableRows(rows(), visibility().visibleLimit),
  );
  const shouldShowVisibilityStatus = () =>
    visibility().totalRows > CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS;
  const showMoreRows = () => {
    setVisibleLimit(visibility().nextVisibleLimit);
  };

  createEffect(() => {
    const totalRows = rows().length;
    setVisibleLimit((current) => {
      if (totalRows <= CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS) {
        return CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS;
      }
      return Math.min(Math.max(current, CONNECTIONS_TABLE_INITIAL_VISIBLE_ROWS), totalRows);
    });
  });

  return (
    <Card padding="none" tone="card" class="rounded-md">
      <div class="flex flex-col gap-3 border-b border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 class="text-base font-semibold text-base-content">Monitored systems</h3>
          <p class="text-xs text-muted">{getInfrastructureEmptyStateSummary()}</p>
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
        when={rows().length > 0}
        fallback={
          <div class="space-y-3 px-4 py-10 text-center">
            <div class="text-base font-semibold text-base-content">
              Start monitoring infrastructure
            </div>
            <div class="mx-auto max-w-3xl text-sm text-muted">
              Add your first server, cluster, or appliance to begin. Supported sources include
              Proxmox VE, TrueNAS SCALE, Unraid, network endpoints, and any host running Pulse
              Agent.
            </div>
            <Show when={(props.headerActions ?? []).find((action) => action.tone === 'primary')}>
              {(primary) => (
                <div class="pt-2">
                  <button
                    type="button"
                    onClick={primary().onSelect}
                    class="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700"
                  >
                    {primary().label}
                  </button>
                </div>
              )}
            </Show>
          </div>
        }
      >
        <Show when={shouldShowVisibilityStatus()}>
          <div
            id="connections-table-visibility-status"
            class="border-b border-border px-4 py-2 text-xs text-muted"
            aria-live="polite"
          >
            {visibility().statusText}
          </div>
        </Show>
        <Table
          aria-describedby={
            shouldShowVisibilityStatus() ? 'connections-table-visibility-status' : undefined
          }
          class="w-full table-fixed divide-y divide-border text-sm !whitespace-normal"
        >
          <TableHeader class="bg-surface-alt">
            <TableRow>
              <TableHead class="w-[22%] py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[18%]">
                System
              </TableHead>
              <TableHead class="w-[26%] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[22%]">
                Coverage
              </TableHead>
              <TableHead class="w-[26%] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[24%]">
                Posture
              </TableHead>
              <TableHead class="w-[14%] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:w-[12%]">
                Status
              </TableHead>
              <TableHead class="hidden w-[10%] px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted whitespace-nowrap 2xl:table-cell">
                Last activity
              </TableHead>
              <Show when={hasActions()}>
                <TableHead class={actionColumnClass}>Actions</TableHead>
              </Show>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border bg-surface">
            <For each={visibleRows()}>
              {(row) => {
                const pauseLabel = () => (row.enabled ? 'Pause' : 'Resume');
                const isPauseBusy = () => props.actions?.pendingAction(row.id) === 'pause';
                const isRemoveBusy = () => props.actions?.pendingAction(row.id) === 'remove';
                const anyBusy = () => props.actions?.pendingAction(row.id) !== null;
                const isConfirmingRemove = () => Boolean(props.actions?.confirmingRemove(row.id));
                const rowError = () => props.actions?.actionError(row.id) ?? null;

                return (
                  <>
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
                          <Show when={row.lastErrorMessage}>
                            <div class="break-words text-xs text-rose-700 dark:text-rose-300">
                              {row.lastErrorMessage}
                            </div>
                          </Show>
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

                      <TableCell class="px-3 py-3 align-top">
                        <div class="flex flex-wrap gap-1.5">
                          <For each={row.fleetHighlights}>
                            {(signal) => (
                              <span class={fleetSignalClassName(signal.tone)} title={signal.detail}>
                                {signal.label}
                              </span>
                            )}
                          </For>
                        </div>
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

                      <Show when={hasActions()}>
                        <TableCell class="px-4 py-3 align-top text-right">
                          <div class="flex flex-wrap items-center justify-end gap-1.5">
                            <Show when={row.canEdit && props.onEdit}>
                              <button
                                type="button"
                                disabled={anyBusy()}
                                onClick={() => props.onEdit?.(row.connection)}
                                class={inlineButtonClass}
                              >
                                Edit
                              </button>
                            </Show>
                            <Show when={row.canPause && props.actions}>
                              <button
                                type="button"
                                disabled={anyBusy()}
                                onClick={() => void props.actions?.togglePause(row.connection)}
                                class={inlineButtonClass}
                              >
                                {isPauseBusy() ? 'Working…' : pauseLabel()}
                              </button>
                            </Show>
                            <Show when={row.canRemove && props.actions}>
                              <button
                                type="button"
                                disabled={anyBusy()}
                                onClick={() => void props.actions?.requestRemove(row.connection)}
                                class={
                                  isConfirmingRemove() ? removeConfirmClass : removeButtonClass
                                }
                              >
                                {isRemoveBusy()
                                  ? 'Removing…'
                                  : isConfirmingRemove()
                                    ? 'Click again to confirm'
                                    : 'Remove'}
                              </button>
                            </Show>
                          </div>
                        </TableCell>
                      </Show>
                    </TableRow>

                    <Show when={rowError()}>
                      <TableRow>
                        <TableCell colspan={colSpan()} class="bg-surface px-4 pb-3 pt-0">
                          <div
                            role="alert"
                            class="rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                          >
                            {rowError()}
                          </div>
                        </TableCell>
                      </TableRow>
                    </Show>

                    <Show when={!row.isAgent && isConfirmingRemove()}>
                      <TableRow>
                        <TableCell colspan={colSpan()} class="bg-surface-alt px-4 pb-3 pt-1">
                          <p class="text-xs text-muted">
                            Removing forgets this connection from Pulse; history is retained.
                            Credentials on the platform itself are untouched — revoke them there
                            separately if you want to invalidate access.
                          </p>
                        </TableCell>
                      </TableRow>
                    </Show>

                    <Show
                      when={row.isAgent && isConfirmingRemove() && props.agentUninstallCommands}
                    >
                      <TableRow>
                        <TableCell colspan={colSpan()} class="bg-surface-alt px-4 pb-4 pt-1">
                          <div class="space-y-3">
                            <p class="text-xs text-muted">
                              Removing forgets this agent from the ledger; history is retained. To
                              fully detach, run the uninstall command on the host:
                            </p>
                            <div class="space-y-1">
                              <span class="text-xs font-medium text-muted">
                                Linux / macOS / FreeBSD
                              </span>
                              <div class="relative">
                                <Show when={props.onCopyText}>
                                  <button
                                    type="button"
                                    onClick={() =>
                                      props.onCopyText?.(props.agentUninstallCommands!.linux)
                                    }
                                    class="absolute right-2 top-2 inline-flex items-center justify-center rounded-md bg-surface-hover px-2 py-1 text-xs font-medium text-slate-400 transition-colors hover:bg-slate-700 hover:text-slate-200"
                                    title="Copy command"
                                  >
                                    Copy
                                  </button>
                                </Show>
                                <pre class="overflow-x-auto rounded-md bg-slate-950 p-3 pr-16 font-mono text-xs text-red-400">
                                  <code>{props.agentUninstallCommands!.linux}</code>
                                </pre>
                              </div>
                            </div>
                            <div class="space-y-1">
                              <span class="text-xs font-medium text-muted">
                                Windows (PowerShell as Administrator)
                              </span>
                              <div class="relative">
                                <Show when={props.onCopyText}>
                                  <button
                                    type="button"
                                    onClick={() =>
                                      props.onCopyText?.(props.agentUninstallCommands!.windows)
                                    }
                                    class="absolute right-2 top-2 inline-flex items-center justify-center rounded-md bg-surface-hover px-2 py-1 text-xs font-medium text-slate-400 transition-colors hover:bg-slate-700 hover:text-slate-200"
                                    title="Copy command"
                                  >
                                    Copy
                                  </button>
                                </Show>
                                <pre class="overflow-x-auto rounded-md bg-slate-950 p-3 pr-16 font-mono text-xs text-red-400">
                                  <code>{props.agentUninstallCommands!.windows}</code>
                                </pre>
                              </div>
                            </div>
                          </div>
                        </TableCell>
                      </TableRow>
                    </Show>
                  </>
                );
              }}
            </For>
          </TableBody>
        </Table>
        <Show when={visibility().isBounded}>
          <div class="flex flex-col gap-2 border-t border-border px-4 py-3 text-xs text-muted sm:flex-row sm:items-center sm:justify-between">
            <span>{visibility().statusText}</span>
            <button
              type="button"
              onClick={showMoreRows}
              aria-label={visibility().showMoreAriaLabel}
              class={inlineButtonClass}
            >
              {visibility().showMoreLabel}
            </button>
          </div>
        </Show>
      </Show>
    </Card>
  );
};
