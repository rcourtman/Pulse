import { For, Show } from 'solid-js';

import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { getPlatformTableHeadClassForKind } from '@/features/platformPage/sharedPlatformPage';
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
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                  >
                    Timestamp
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('badge')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                  >
                    Source
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('name')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                  >
                    Resource
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('badge')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                  >
                    {getTypeColumnLabel()}
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('badge')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                  >
                    Severity
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                  >
                    Message
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden text-[10px] font-medium uppercase tracking-wider lg:table-cell sm:text-xs`}
                  >
                    Duration
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('badge')} hidden text-[10px] font-medium uppercase tracking-wider lg:table-cell sm:text-xs`}
                  >
                    Status
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden text-[10px] font-medium uppercase tracking-wider lg:table-cell sm:text-xs`}
                  >
                    Node
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('badge')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                  >
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
