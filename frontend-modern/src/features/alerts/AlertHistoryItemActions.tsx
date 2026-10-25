import { Show } from 'solid-js';
import BoxIcon from 'lucide-solid/icons/box';
import HistoryIcon from 'lucide-solid/icons/history';

import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';
import {
  getAlertHistoryResourceActionLabel,
  getAlertHistoryTimelineActionLabel,
  getAlertResourceIncidentViewTitle,
} from '@/utils/alertIncidentPresentation';

import type { AlertHistoryState } from './useAlertHistoryState';

type AlertHistoryAlert = ReturnType<AlertHistoryState['groupedAlerts']>[number]['alerts'][number];

// Table rows show Timeline and Resource as 24px icon buttons beside the
// Investigate icon, so the three actions take about 100px instead of 177px
// and Message and Resource get that width on one line. Their accessible
// names and titles still say what each opens; cards keep the worded buttons.
const ALERT_HISTORY_ICON_ACTION_CLASS =
  'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500';
const ALERT_HISTORY_TEXT_ACTION_CLASS =
  'rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover';

interface AlertHistoryItemActionsProps {
  alert: AlertHistoryAlert;
  state: AlertHistoryState;
  class?: string;
  touchSized?: boolean;
  iconOnly?: boolean;
  onTimelineClick?: () => void;
  onResourceClick?: () => void;
  timelineOpen?: boolean;
}

export function AlertHistoryItemActions(props: AlertHistoryItemActionsProps) {
  const rowKey = () => props.state.getIncidentRowKey(props.alert);
  const timelineOpen = () => props.timelineOpen ?? props.state.expandedIncidents().has(rowKey());
  // Every row repeats the same controls, so each name says which alert.
  const timelineLabel = () =>
    getAlertHistoryTimelineActionLabel({
      open: timelineOpen(),
      alertTitle: props.alert.title,
      resourceName: props.alert.resourceName,
      startedAt: props.state.formatAlertRowTimestamp(props.alert.startTime),
    });

  return (
    <div
      class={`flex ${props.iconOnly ? 'flex-nowrap' : 'flex-wrap'} items-center gap-1.5 ${props.class ?? ''}`.trim()}
    >
      <Show when={props.alert.source === 'alert'}>
        <button
          type="button"
          class={
            props.iconOnly
              ? `${ALERT_HISTORY_ICON_ACTION_CLASS} ${timelineOpen() ? 'bg-surface-hover text-base-content' : ''}`
              : `${ALERT_HISTORY_TEXT_ACTION_CLASS} ${props.touchSized ? 'min-h-11' : ''}`
          }
          data-alert-history-action="timeline"
          aria-label={timelineLabel()}
          aria-expanded={timelineOpen()}
          title={props.iconOnly ? (timelineOpen() ? 'Hide timeline' : 'Timeline') : undefined}
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
          <Show when={props.iconOnly} fallback={timelineOpen() ? 'Hide' : 'Timeline'}>
            <HistoryIcon class="h-3.5 w-3.5" aria-hidden="true" />
          </Show>
        </button>
      </Show>
      <Show when={props.alert.source === 'alert' && props.alert.resourceId}>
        <button
          type="button"
          class={
            props.iconOnly
              ? ALERT_HISTORY_ICON_ACTION_CLASS
              : `${ALERT_HISTORY_TEXT_ACTION_CLASS} ${props.touchSized ? 'min-h-11' : ''}`
          }
          data-alert-history-action="resource"
          aria-label={getAlertHistoryResourceActionLabel(props.alert.resourceName)}
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
          <Show when={props.iconOnly} fallback="Resource">
            <BoxIcon class="h-3.5 w-3.5" aria-hidden="true" />
          </Show>
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
            metadata: props.alert.systemAlert ? { systemAlert: true } : undefined,
          }}
          subjectLabel={props.alert.title}
          subjectTime={props.state.formatAlertRowTimestamp(props.alert.startTime)}
          variant="icon"
          size="sm"
          class={props.touchSized ? 'min-h-11 min-w-11' : undefined}
        />
      </Show>
    </div>
  );
}
