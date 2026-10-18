import { Show, createMemo } from 'solid-js';

import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
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

/**
 * The narrowest card the history table is sized for. Below it the three row
 * actions stack, rows lose the single height the windowed rows need, and
 * names and messages break mid-word, so the history shows as cards instead.
 */
export const ALERT_HISTORY_TABLE_MIN_WIDTH = 800;

type AlertHistoryGroup = ReturnType<AlertHistoryState['groupedAlerts']>[number];
type AlertHistoryAlert = AlertHistoryGroup['alerts'][number];
type AlertHistoryRenderItem =
  | { kind: 'group'; key: string; group: AlertHistoryGroup }
  | { kind: 'alert'; key: string; alert: AlertHistoryAlert };

export function AlertHistoryTableSection(props: AlertHistoryTableSectionProps) {
  // The card's own width decides, not the viewport: a laptop window with the
  // Alerts rail open leaves the table far less room than the screen suggests.
  // Until the card is measured, the phone breakpoint decides as before.
  const cardWidth = useObservedElementWidth();
  const { isMobile } = useBreakpoint();
  // A memo, so readers hear about a change of layout, not every resized pixel.
  const showCards = createMemo(() => {
    const width = cardWidth.width();
    return width === null ? isMobile() : width < ALERT_HISTORY_TABLE_MIN_WIDTH;
  });
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
            <Show when={!props.state.historyLoadError()}>
              <div class="py-12 text-center text-muted">
                <p class="text-sm">{getAlertHistoryEmptyState().title}</p>
                <p class="mt-1 text-xs">{getAlertHistoryEmptyState().description}</p>
              </div>
            </Show>
          }
        >
          <div
            ref={cardWidth.setElement}
            data-alert-history-layout={showCards() ? 'cards' : 'table'}
          >
            <AlertHistoryMobileList state={props.state} active={showCards} />
            <TableCard class={showCards() ? 'mb-2 hidden' : 'mb-2'}>
              <Table class="alert-history-responsive-table table-fixed min-w-0 text-[11px] sm:text-sm">
                {/* Each track pairs its density visibility with its own width (index.css). */}
                <colgroup>
                  <col class="alert-history-core-track alert-history-timestamp-track" />
                  <col class="alert-history-core-track alert-history-resource-track" />
                  <col class="alert-history-full-detail-track alert-history-type-track" />
                  <col class="alert-history-core-track alert-history-severity-track" />
                  <col class="alert-history-core-track alert-history-message-track" />
                  <col class="alert-history-context-track alert-history-duration-track" />
                  <col class="alert-history-core-track alert-history-status-track" />
                  <col class="alert-history-full-detail-track alert-history-node-track" />
                  <col class="alert-history-core-track alert-history-actions-track" />
                </colgroup>
                <TableHeader>
                  <TableRow class="border-b border-border bg-surface-hover text-muted">
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} alert-history-timestamp-column text-[10px] font-medium uppercase tracking-wider sm:text-xs`}
                    >
                      Time
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
          </div>
        </Show>
      }
    >
      <div class="py-12 text-center text-muted" role="status">
        <p class="text-sm">{getAlertHistoryLoadingState().text}</p>
      </div>
    </Show>
  );
}
