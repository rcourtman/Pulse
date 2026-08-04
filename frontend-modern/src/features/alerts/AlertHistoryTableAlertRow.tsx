import { Show } from 'solid-js';

import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getPlatformTableCellClassForKind } from '@/features/platformPage/sharedPlatformPage';
import { getAlertHistoryResourceTypeBadgeClass } from '@/utils/alertHistoryPresentation';
import {
  getAlertHistoryStatusPresentation,
  getAlertIncidentLevelBadgeClass,
} from '@/utils/alertIncidentPresentation';

import { AlertHistoryItemActions } from './AlertHistoryItemActions';
import type { AlertHistoryState } from './useAlertHistoryState';

type AlertHistoryAlert = ReturnType<AlertHistoryState['groupedAlerts']>[number]['alerts'][number];

interface AlertHistoryTableAlertRowProps {
  alert: AlertHistoryAlert;
  state: AlertHistoryState;
}

export function AlertHistoryTableAlertRow(props: AlertHistoryTableAlertRowProps) {
  const rowKey = () => props.state.getIncidentRowKey(props.alert);
  const historyStatusPresentation = () => getAlertHistoryStatusPresentation(props.alert.status);

  return (
    <>
      <TableRow
        class={`border-b border-border hover:bg-surface-hover ${historyStatusPresentation().rowClassName}`}
      >
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} alert-history-context-column font-mono whitespace-nowrap text-muted`}
        >
          {new Date(props.alert.startTime).toLocaleTimeString('en-US', {
            hour: '2-digit',
            minute: '2-digit',
          })}
        </TableCell>

        <TableCell
          class={`${getPlatformTableCellClassForKind('name')} max-w-[150px] truncate font-medium text-base-content`}
          title={props.alert.resourceName}
        >
          {props.alert.resourceName}
        </TableCell>

        <TableCell class={`${getPlatformTableCellClassForKind('badge')} hidden md:table-cell`}>
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

        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} max-w-[300px] truncate text-base-content`}
          title={props.alert.description}
        >
          {props.alert.description}
        </TableCell>

        <TableCell
          class={`${getPlatformTableCellClassForKind('numeric-value')} alert-history-context-column text-muted`}
        >
          {props.alert.duration}
        </TableCell>

        <TableCell class={getPlatformTableCellClassForKind('badge')}>
          <span class={historyStatusPresentation().className}>
            {historyStatusPresentation().label}
          </span>
        </TableCell>

        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden truncate text-muted md:table-cell`}
          title={props.alert.nodeDisplayName || props.alert.node || ''}
        >
          {props.alert.nodeDisplayName || props.alert.node || '—'}
        </TableCell>

        <TableCell class={getPlatformTableCellClassForKind('badge')}>
          <AlertHistoryItemActions alert={props.alert} state={props.state} class="justify-center" />
        </TableCell>
      </TableRow>

      <Show when={props.alert.source === 'alert' && props.state.expandedIncidents().has(rowKey())}>
        <TableRow class="border-b border-border bg-surface-alt">
          <TableCell colspan={9} class="p-3">
            <IncidentTimelinePanel
              loading={props.state.incidentLoading()[rowKey()]}
              error={props.state.incidentErrors()[rowKey()]}
              timeline={props.state.incidentTimelines()[rowKey()]}
              filters={props.state.historyIncidentEventFilters}
              setFilters={props.state.setHistoryIncidentEventFilters}
              filterVariant="compact"
              eventCardVariant="surface"
              noteDraft={props.state.incidentNoteDrafts()[rowKey()] || ''}
              onNoteDraftChange={(value) => props.state.setIncidentNoteDraft(rowKey(), value)}
              noteSaving={props.state.incidentNoteSaving().has(rowKey())}
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
    </>
  );
}
