import { Show } from 'solid-js';

import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';
import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import type { Alert } from '@/types/api';
import {
  getAlertOverviewAcknowledgedBadgeClass,
  getAlertOverviewAcknowledgedBadgeLabel,
  getAlertOverviewCardPresentation,
  getAlertOverviewNodeLabel,
  getAlertOverviewPrimaryActionLabel,
  getAlertOverviewPrimaryActionClass,
  getAlertOverviewSecondaryActionClass,
  getAlertOverviewStartedAtLabel,
  getAlertOverviewStartedAtClass,
  getAlertOverviewTimelineActionLabel,
} from '@/utils/alertOverviewPresentation';

import { alertTypeDisplayLabel } from './helpers';
import { getCanonicalAlertId } from './identity';
import type { AlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import type { AlertOverviewState } from './useAlertOverviewState';

interface AlertOverviewAlertCardProps {
  alert: Alert;
  state: AlertOverviewState;
  timelineState: AlertIncidentTimelineState;
}

export function AlertOverviewAlertCard(props: AlertOverviewAlertCardProps) {
  const alertKey = () => getCanonicalAlertId(props.alert);
  const alertCardPresentation = () =>
    getAlertOverviewCardPresentation(
      props.alert.level ?? 'warning',
      props.alert.acknowledged,
      props.state.processingAlerts().has(alertKey()),
    );

  return (
    <div id={`alert-${alertKey()}`} class={alertCardPresentation().cardClassName}>
      <div class="flex flex-col sm:flex-row sm:items-start">
        <div class="flex items-start flex-1">
          <div class={alertCardPresentation().iconClassName}>
            {props.alert.acknowledged ? (
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            ) : (
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            )}
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class={alertCardPresentation().resourceClassName}>
                {props.alert.resourceName}
              </span>
              <span class="text-xs text-muted">({alertTypeDisplayLabel(props.alert.type)})</span>
              <Show when={!props.alert.acknowledged}>
                <span
                  class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide"
                  classList={{
                    'bg-red-100 text-red-700 dark:bg-red-900/50 dark:text-red-300':
                      props.alert.level === 'critical',
                    'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/50 dark:text-yellow-300':
                      props.alert.level !== 'critical',
                  }}
                >
                  {props.alert.level === 'critical' ? 'Critical' : 'Warning'}
                </span>
              </Show>
              <Show when={props.alert.node}>
                <span class="text-xs text-muted">
                  {getAlertOverviewNodeLabel(props.alert.nodeDisplayName || props.alert.node)}
                </span>
              </Show>
              <Show when={props.alert.acknowledged}>
                <span class={getAlertOverviewAcknowledgedBadgeClass()}>
                  {getAlertOverviewAcknowledgedBadgeLabel()}
                </span>
              </Show>
            </div>
            <p class="text-sm text-base-content mt-1 break-words">{props.alert.message}</p>
            <div class="flex flex-wrap items-center gap-x-3 gap-y-0.5 mt-1">
              <p class={getAlertOverviewStartedAtClass()}>
                {getAlertOverviewStartedAtLabel(new Date(props.alert.startTime).toLocaleString())}
              </p>
              <Show when={props.alert.threshold > 0}>
                <span class="text-xs text-muted">
                  limit: {props.alert.threshold}
                  {props.alert.type === 'temperature' || props.alert.type === 'diskTemperature'
                    ? '°C'
                    : '%'}
                </span>
              </Show>
            </div>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-1.5 sm:gap-2 mt-3 sm:mt-0 sm:ml-4 self-end sm:self-start justify-end">
          <button
            class={getAlertOverviewPrimaryActionClass(props.alert.acknowledged)}
            disabled={props.state.processingAlerts().has(alertKey())}
            onClick={async (e) => {
              e.preventDefault();
              e.stopPropagation();
              await props.state.handleAlertAcknowledgement(props.alert);
            }}
          >
            {getAlertOverviewPrimaryActionLabel({
              acknowledged: props.alert.acknowledged,
              processing: props.state.processingAlerts().has(alertKey()),
            })}
          </button>
          <button
            class={getAlertOverviewSecondaryActionClass()}
            onClick={() => {
              void props.timelineState.toggleIncidentTimeline(alertKey(), props.alert.startTime);
            }}
          >
            {getAlertOverviewTimelineActionLabel(
              props.timelineState.expandedIncidents().has(alertKey()),
            )}
          </button>
          <InvestigateAlertButton
            alert={props.alert}
            resourceType={
              typeof props.alert.metadata?.resourceType === 'string'
                ? (props.alert.metadata.resourceType as string)
                : undefined
            }
            variant="text"
            size="sm"
            patrolOption
          />
        </div>
      </div>
      <Show when={props.timelineState.expandedIncidents().has(alertKey())}>
        <div class="mt-3 border-t border-border pt-3">
          <IncidentTimelinePanel
            loading={props.timelineState.incidentLoading()[alertKey()]}
            error={props.timelineState.incidentErrors()[alertKey()]}
            timeline={props.timelineState.incidentTimelines()[alertKey()]}
            filters={props.timelineState.eventFilters}
            setFilters={props.timelineState.setEventFilters}
            filterVariant="panel"
            eventCardVariant="alt"
            noteDraft={props.timelineState.incidentNoteDrafts()[alertKey()] || ''}
            onNoteDraftChange={(value) =>
              props.timelineState.setIncidentNoteDraft(alertKey(), value)
            }
            noteSaving={props.timelineState.incidentNoteSaving().has(alertKey())}
            onSaveNote={() => {
              void props.timelineState.saveIncidentNote(alertKey(), props.alert.startTime);
            }}
            onRetry={() => {
              void props.timelineState.loadIncidentTimeline(alertKey(), props.alert.startTime);
            }}
          />
        </div>
      </Show>
    </div>
  );
}
