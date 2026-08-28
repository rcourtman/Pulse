import { Show } from 'solid-js';

import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';
import { getAlertResourceIncidentViewTitle } from '@/utils/alertIncidentPresentation';

import type { AlertHistoryState } from './useAlertHistoryState';

type AlertHistoryAlert = ReturnType<AlertHistoryState['groupedAlerts']>[number]['alerts'][number];

interface AlertHistoryItemActionsProps {
  alert: AlertHistoryAlert;
  state: AlertHistoryState;
  class?: string;
  touchSized?: boolean;
  onTimelineClick?: () => void;
  onResourceClick?: () => void;
}

export function AlertHistoryItemActions(props: AlertHistoryItemActionsProps) {
  const rowKey = () => props.state.getIncidentRowKey(props.alert);

  return (
    <div class={`flex flex-wrap items-center gap-1.5 ${props.class ?? ''}`.trim()}>
      <Show when={props.alert.source === 'alert'}>
        <button
          type="button"
          class={`rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover ${
            props.touchSized ? 'min-h-11' : ''
          }`}
          data-alert-history-action="timeline"
          onClick={() => {
            if (props.onTimelineClick) {
              props.onTimelineClick();
              return;
            }
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
          class={`rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover ${
            props.touchSized ? 'min-h-11' : ''
          }`}
          data-alert-history-action="resource"
          title={getAlertResourceIncidentViewTitle()}
          onClick={() => {
            if (props.onResourceClick) {
              props.onResourceClick();
              return;
            }
            void props.state.openResourceIncidentPanel(
              props.alert.resourceId as string,
              props.alert.resourceName,
              rowKey(),
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
            level: props.alert.severity as 'info' | 'warning' | 'critical',
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
          class={props.touchSized ? 'min-h-11 min-w-11' : undefined}
        />
      </Show>
    </div>
  );
}
