import { Show, createMemo } from 'solid-js';

import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { getPlatformTableHeadClassForKind } from '@/features/platformPage/sharedPlatformPage';
import { PlatformWindowedRows } from '@/features/platformPage/PlatformWindowedRows';
import {
  getAlertHistoryEmptyState,
  getAlertHistoryLoadingState,
} from '@/utils/alertOverviewPresentation';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';

import { AlertHistoryMobileList } from './AlertHistoryMobileList';
import { AlertHistoryTableAlertRow } from './AlertHistoryTableAlertRow';
import { AlertHistoryTableGroupRow } from './AlertHistoryTableGroupRow';
import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryTableSectionProps {
  state: AlertHistoryState;
}

type AlertHistoryGroup = ReturnType<AlertHistoryState['groupedAlerts']>[number];
type AlertHistoryAlert = AlertHistoryGroup['alerts'][number];
type AlertHistoryRenderItem =
  | { kind: 'group'; key: string; group: AlertHistoryGroup }
  | { kind: 'alert'; key: string; alert: AlertHistoryAlert };

export function AlertHistoryTableSection(props: AlertHistoryTableSectionProps) {
  const renderItems = createMemo<AlertHistoryRenderItem[]>(() => {
    const items: AlertHistoryRenderItem[] = [];
    for (const [groupIndex, group] of props.state.groupedAlerts().entries()) {
      items.push({ kind: 'group', key: `group:${groupIndex}:${group.fullLabel}`, group });
      for (const alert of group.alerts) {
        items.push({ kind: 'alert', key: `alert:${alert.id}:${alert.startTime}`, alert });
      }
    }
    return items;
  });

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
          <>
            <AlertHistoryMobileList state={props.state} />
            <TableCard class="mb-2 hidden md:block">
              <Table class="alert-history-responsive-table table-fixed min-w-0 text-[11px] sm:text-sm">
                <colgroup>
                  <col class="alert-history-core-track" />
                  <col class="alert-history-core-track" />
                  <col class="alert-history-full-detail-track" />
                  <col class="alert-history-core-track" />
                  <col class="alert-history-core-track" />
                  <col class="alert-history-context-track" />
                  <col class="alert-history-core-track" />
                  <col class="alert-history-full-detail-track" />
                  <col class="alert-history-core-track" />
                </colgroup>
                <TableHeader>
                  <TableRow class="border-b border-border bg-surface-hover text-muted">
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} alert-history-timestamp-column text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                    >
                      Timestamp
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('name')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                    >
                      Resource
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('badge')} alert-history-full-detail-column text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
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
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} alert-history-context-column text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                    >
                      Duration
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('badge')} text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                    >
                      Status
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} alert-history-full-detail-column text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
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
                  <PlatformWindowedRows items={renderItems} estimatedRowHeight={40}>
                    {(item) =>
                      item.kind === 'group' ? (
                        <AlertHistoryTableGroupRow group={item.group} />
                      ) : (
                        <AlertHistoryTableAlertRow alert={item.alert} state={props.state} />
                      )
                    }
                  </PlatformWindowedRows>
                </TableBody>
              </Table>
            </TableCard>
          </>
        </Show>
      }
    >
      <div class="py-12 text-center text-muted">
        <p class="text-sm">{getAlertHistoryLoadingState().text}</p>
      </div>
    </Show>
  );
}
