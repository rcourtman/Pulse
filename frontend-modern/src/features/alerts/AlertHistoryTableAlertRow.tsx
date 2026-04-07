import { Show } from 'solid-js';

import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  getAlertHistoryResourceTypeBadgeClass,
  getAlertHistorySourcePresentation,
} from '@/utils/alertHistoryPresentation';
import {
  getAlertHistoryStatusPresentation,
  getAlertIncidentLevelBadgeClass,
  getAlertResourceIncidentViewTitle,
} from '@/utils/alertIncidentPresentation';

import type { AlertHistoryState } from './useAlertHistoryState';

type AlertHistoryAlert = ReturnType<AlertHistoryState['groupedAlerts']>[number]['alerts'][number];

interface AlertHistoryTableAlertRowProps {
  alert: AlertHistoryAlert;
  state: AlertHistoryState;
  hasAIAlertsFeature: () => boolean;
  runtimeCapabilitiesLoading: () => boolean;
}

export function AlertHistoryTableAlertRow(props: AlertHistoryTableAlertRowProps) {
  const rowKey = () => props.state.getIncidentRowKey(props.alert);
  const historyStatusPresentation = () => getAlertHistoryStatusPresentation(props.alert.status);
  const sourcePresentation = () => getAlertHistorySourcePresentation(props.alert.source);

  return (
    <>
      <TableRow
        class={`border-b border-border hover:bg-surface-hover ${historyStatusPresentation().rowClassName}`}
      >
        <TableCell class="p-1 px-1 font-mono whitespace-nowrap text-muted sm:p-1.5 sm:px-2">
          {new Date(props.alert.startTime).toLocaleTimeString('en-US', {
            hour: '2-digit',
            minute: '2-digit',
          })}
        </TableCell>

        <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
          <span class={sourcePresentation().className}>{sourcePresentation().label}</span>
        </TableCell>

        <TableCell class="max-w-[150px] truncate p-1 px-1 font-medium text-base-content sm:p-1.5 sm:px-2">
          {props.alert.resourceName}
        </TableCell>

        <TableCell class="p-1 px-1 sm:p-1.5 sm:px-2">
          <span class={getAlertHistoryResourceTypeBadgeClass(props.alert.resourceType)}>
            {props.alert.resourceType}
          </span>
        </TableCell>

        <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
          <span class={getAlertIncidentLevelBadgeClass(props.alert.severity)}>
            {props.alert.severity}
          </span>
        </TableCell>

        <TableCell
          class="max-w-[300px] truncate p-1 px-1 text-base-content sm:p-1.5 sm:px-2"
          title={props.alert.description}
        >
          {props.alert.description}
        </TableCell>

        <TableCell class="p-1 px-1 text-center text-muted sm:p-1.5 sm:px-2">
          {props.alert.duration}
        </TableCell>

        <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
          <span class={historyStatusPresentation().className}>
            {historyStatusPresentation().label}
          </span>
        </TableCell>

        <TableCell class="truncate p-1 px-1 text-muted sm:p-1.5 sm:px-2">
          {props.alert.nodeDisplayName || props.alert.node || '—'}
        </TableCell>

        <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
          <div class="flex items-center justify-center gap-1">
            <Show when={props.alert.source === 'alert'}>
              <button
                type="button"
                class="rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover"
                onClick={() => {
                  void props.state.toggleIncidentTimeline(
                    rowKey(),
                    props.alert.id,
                    props.alert.startTime,
                  );
                }}
              >
                {props.state.expandedIncidents().has(rowKey()) ? 'Hide' : 'Timeline'}
              </button>
            </Show>
            <Show when={props.alert.source === 'alert' && props.alert.resourceId}>
              <button
                type="button"
                class="rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover"
                title={getAlertResourceIncidentViewTitle()}
                onClick={() => {
                  void props.state.openResourceIncidentPanel(
                    props.alert.resourceId as string,
                    props.alert.resourceName,
                  );
                }}
              >
                Resource
              </button>
            </Show>
            <Show
              when={
                props.alert.source === 'alert' &&
                (props.alert.status === 'active' || props.alert.status === 'acknowledged')
              }
            >
              <InvestigateAlertButton
                alert={{
                  id: props.alert.id,
                  type: props.alert.rawAlertType || props.alert.title,
                  level: props.alert.severity as 'warning' | 'critical',
                  resourceId: props.alert.resourceId || '',
                  resourceName: props.alert.resourceName,
                  node: props.alert.node || '',
                  nodeDisplayName: props.alert.nodeDisplayName,
                  instance: '',
                  message: props.alert.description || '',
                  value: 0,
                  threshold: 0,
                  startTime: props.alert.startTime,
                  lastSeen: props.alert.startTime,
                  acknowledged: props.alert.status === 'acknowledged',
                }}
                resourceType={props.alert.resourceType}
                variant="icon"
                size="sm"
                licenseLocked={!props.hasAIAlertsFeature() && !props.runtimeCapabilitiesLoading()}
              />
            </Show>
          </div>
        </TableCell>
      </TableRow>

      <Show when={props.alert.source === 'alert' && props.state.expandedIncidents().has(rowKey())}>
        <TableRow class="border-b border-border bg-surface-alt">
          <TableCell colspan={11} class="p-3">
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
