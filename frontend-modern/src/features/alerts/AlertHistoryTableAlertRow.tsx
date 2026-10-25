import { Show } from 'solid-js';

import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getPlatformTableCellClassForKind } from '@/features/platformPage/sharedPlatformPage';
import { getAlertHistoryResourceTypeBadgeClass } from '@/utils/alertHistoryPresentation';
import {
  getAlertHistoryStatusPresentation,
  getAlertIncidentLevelBadgeClass,
} from '@/utils/alertIncidentPresentation';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';

import { AlertHistoryItemActions } from './AlertHistoryItemActions';
import { getAlertHistoryRowCopy } from './alertHistoryModel';
import { AlertResourceIncidentsPanel } from './AlertResourceIncidentsPanel';
import type { AlertHistoryState } from './useAlertHistoryState';

type AlertHistoryAlert = ReturnType<AlertHistoryState['groupedAlerts']>[number]['alerts'][number];

// Below a 50rem card (a 1024px window) the message track is about 110px, too
// narrow to tell alerts apart on one line, so the cells that say which
// resource and what happened wrap and Message holds two lines on every row:
// the windowing hook sizes its spacers from one measured row, and rows of
// mixed heights make a long history jump as it scrolls. From a 50rem card the
// icon row actions and compact status leave Message and Resource enough room
// for one line, the single-line row the other platform tables keep. Text past
// its line ends in an ellipsis and keeps its title.
const ALERT_HISTORY_MESSAGE_TEXT_CLASS =
  'line-clamp-2 min-h-[2lh] @min-[50rem]:line-clamp-1 @min-[50rem]:min-h-0 whitespace-normal wrap-break-word text-base-content';
const ALERT_HISTORY_RESOURCE_TEXT_CLASS =
  'line-clamp-2 @min-[50rem]:line-clamp-1 whitespace-normal wrap-break-word text-base-content';

interface AlertHistoryTableAlertRowProps {
  alert: AlertHistoryAlert;
  state: AlertHistoryState;
}

export function AlertHistoryTableAlertRow(props: AlertHistoryTableAlertRowProps) {
  const rowKey = () => props.state.getIncidentRowKey(props.alert);
  const historyStatusPresentation = () =>
    getAlertHistoryStatusPresentation(props.alert.status, { compact: true });
  // An open threshold alert's live reading turns stale when evaluations stop,
  // so it rechecks on the shared clock; the last breach moves to the hover.
  const now = useRelativeTimeNow();
  const descriptionCopy = () => getAlertHistoryRowCopy(props.alert, now);

  return (
    <>
      <TableRow
        class={`border-b border-border hover:bg-surface-hover ${historyStatusPresentation().rowClassName}`}
      >
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} alert-history-timestamp-column font-mono whitespace-nowrap text-muted`}
          title={props.state.formatAlertRowTimestamp(props.alert.startTime)}
        >
          {props.state.formatAlertRowTime(props.alert.startTime)}
        </TableCell>

        {/* The row has no disclosure of its own, so the two cells that say
            which resource and what happened wrap instead of ending in an
            ellipsis; the title still carries the full text. The wrapper opts
            back into wrapping because `.table-fixed td` forces nowrap. */}
        <TableCell class={`${getPlatformTableCellClassForKind('name')} font-medium`}>
          <div
            class={ALERT_HISTORY_RESOURCE_TEXT_CLASS}
            data-alert-history-cell="resource"
            title={props.alert.resourceName}
          >
            {props.alert.resourceName}
          </div>
        </TableCell>

        <TableCell
          class={`${getPlatformTableCellClassForKind('badge')} alert-history-full-detail-column`}
        >
          <span
            class={getAlertHistoryResourceTypeBadgeClass(props.alert.resourceType)}
            title={props.alert.resourceType}
          >
            {props.alert.title}
          </span>
        </TableCell>

        <TableCell class={getPlatformTableCellClassForKind('badge')}>
          <span class={getAlertIncidentLevelBadgeClass(props.alert.severity)}>
            {props.alert.severity}
          </span>
        </TableCell>

        <TableCell class={getPlatformTableCellClassForKind('text')}>
          <div
            class={ALERT_HISTORY_MESSAGE_TEXT_CLASS}
            data-alert-history-cell="message"
            title={descriptionCopy().title}
          >
            {descriptionCopy().text}
          </div>
        </TableCell>

        <TableCell
          class={`${getPlatformTableCellClassForKind('numeric-value')} alert-history-context-column text-muted`}
        >
          {props.alert.duration}
        </TableCell>

        <TableCell class={getPlatformTableCellClassForKind('badge')}>
          <span class={historyStatusPresentation().className} title={props.alert.closeDetail}>
            {historyStatusPresentation().label}
          </span>
        </TableCell>

        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} alert-history-full-detail-column truncate text-muted`}
        >
          <span title={props.alert.nodeDisplayName || props.alert.node || ''}>
            {props.alert.nodeDisplayName || props.alert.node || '—'}
          </span>
        </TableCell>

        <TableCell class={getPlatformTableCellClassForKind('badge')}>
          <AlertHistoryItemActions
            alert={props.alert}
            state={props.state}
            class="justify-center"
            iconOnly
          />
        </TableCell>
      </TableRow>

      <Show when={props.alert.source === 'alert' && props.state.expandedIncidents().has(rowKey())}>
        <TableRow class="border-b border-border bg-surface-alt">
          <TableCell colspan={9} class="p-3">
            <IncidentTimelinePanel
              loading={() => props.state.incidentLoading()[rowKey()]}
              error={() => props.state.incidentErrors()[rowKey()]}
              timeline={() => props.state.incidentTimelines()[rowKey()]}
              filters={props.state.historyIncidentEventFilters}
              setFilters={props.state.setHistoryIncidentEventFilters}
              filterVariant="compact"
              eventCardVariant="surface"
              noteDraft={() => props.state.incidentNoteDrafts()[rowKey()] || ''}
              onNoteDraftChange={(value) => props.state.setIncidentNoteDraft(rowKey(), value)}
              noteSaving={() => props.state.incidentNoteSaving().has(rowKey())}
              onSaveNote={() => {
                void props.state.saveIncidentNote(rowKey(), props.alert.id, props.alert.startTime);
              }}
              onRetry={() => {
                void props.state.loadIncidentTimeline(
                  rowKey(),
                  props.alert.id,
                  props.alert.startTime,
                );
              }}
            />
          </TableCell>
        </TableRow>
      </Show>

      {/* The incidents panel opens under the row that asked for it. Rendering
          it at page level put the panel thousands of pixels above a reader
          scrolled into the history, so the row button read as inert (#1687). */}
      <Show when={props.state.resourceIncidentPanel()?.rowKey === rowKey()}>
        <TableRow class="border-b border-border bg-surface-alt">
          <TableCell colspan={9} class="p-3">
            <AlertResourceIncidentsPanel state={props.state} />
          </TableCell>
        </TableRow>
      </Show>
    </>
  );
}
