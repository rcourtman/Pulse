import { For, Show } from 'solid-js';

import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import {
  getAlertHistoryEmptyState,
  getAlertHistoryLoadingState,
} from '@/utils/alertOverviewPresentation';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';

import { AlertHistoryTableAlertRow } from './AlertHistoryTableAlertRow';
import { AlertHistoryTableGroupRow } from './AlertHistoryTableGroupRow';
import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryTableSectionProps {
  state: AlertHistoryState;
}

export function AlertHistoryTableSection(props: AlertHistoryTableSectionProps) {
  return (
    <Show
      when={props.state.loading()}
      fallback={
        <Show
          when={props.state.alertData().length > 0}
          fallback={
            <div class="py-12 text-center text-muted">
              <p class="text-sm">{getAlertHistoryEmptyState().title}</p>
              <p class="mt-1 text-xs">{getAlertHistoryEmptyState().description}</p>
            </div>
          }
        >
          <TableCard class="mb-2">
            <Table class="w-full text-[11px] sm:text-sm">
              <TableHeader>
                <TableRow class="border-b border-border bg-surface-hover text-muted">
                  <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                    Timestamp
                  </TableHead>
                  <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                    Source
                  </TableHead>
                  <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                    Resource
                  </TableHead>
                  <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                    {getTypeColumnLabel()}
                  </TableHead>
                  <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                    Severity
                  </TableHead>
                  <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                    Message
                  </TableHead>
                  <TableHead class="hidden p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider lg:table-cell sm:p-1.5 sm:px-2 sm:text-xs">
                    Duration
                  </TableHead>
                  <TableHead class="hidden p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider lg:table-cell sm:p-1.5 sm:px-2 sm:text-xs">
                    Status
                  </TableHead>
                  <TableHead class="hidden p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider lg:table-cell sm:p-1.5 sm:px-2 sm:text-xs">
                    Node
                  </TableHead>
                  <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                    Actions
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <For each={props.state.groupedAlerts()}>
                  {(group) => (
                    <>
                      <AlertHistoryTableGroupRow group={group} />

                      <For each={group.alerts}>
                        {(alert) => <AlertHistoryTableAlertRow alert={alert} state={props.state} />}
                      </For>
                    </>
                  )}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      }
    >
      <div class="py-12 text-center text-muted">
        <p class="text-sm">{getAlertHistoryLoadingState().text}</p>
      </div>
    </Show>
  );
}
