import { For, Show } from 'solid-js';

import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import { getAlertHistoryResourceTypeBadgeClass } from '@/utils/alertHistoryPresentation';
import {
  getAlertHistoryStatusPresentation,
  getAlertIncidentLevelBadgeClass,
} from '@/utils/alertIncidentPresentation';

import { AlertHistoryItemActions } from './AlertHistoryItemActions';
import { getGroupSummaryLabel } from './AlertHistoryTableGroupRow';
import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryMobileListProps {
  state: AlertHistoryState;
}

export function AlertHistoryMobileList(props: AlertHistoryMobileListProps) {
  return (
    <div class="space-y-3 md:hidden" data-testid="alert-history-mobile-list">
      <For each={props.state.groupedAlerts()}>
        {(group) => (
          <section class="overflow-hidden rounded-lg border border-border bg-surface">
            <header class="flex items-start justify-between gap-3 border-b border-border bg-surface-alt px-3 py-2">
              <h3
                class="min-w-0 truncate text-xs font-semibold text-base-content"
                title={group.fullLabel}
              >
                {group.label}
              </h3>
              <span class="shrink-0 text-[10px] text-muted">{getGroupSummaryLabel(group)}</span>
            </header>

            <div class="divide-y divide-border-subtle">
              <For each={group.alerts}>
                {(alert) => {
                  const rowKey = () => props.state.getIncidentRowKey(alert);
                  const status = () => getAlertHistoryStatusPresentation(alert.status);

                  return (
                    <article class={`p-3 ${status().rowClassName}`}>
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <h4
                            class="truncate text-sm font-semibold text-base-content"
                            title={alert.resourceName}
                          >
                            {alert.resourceName}
                          </h4>
                          <span
                            class={`${getAlertHistoryResourceTypeBadgeClass(alert.resourceType)} mt-1 inline-flex`}
                            title={alert.resourceType}
                          >
                            {alert.title}
                          </span>
                        </div>
                        <div class="flex shrink-0 flex-col items-end gap-1">
                          <span class={getAlertIncidentLevelBadgeClass(alert.severity)}>
                            {alert.severity}
                          </span>
                          <span class={status().className}>{status().label}</span>
                        </div>
                      </div>

                      <p class="mt-2 text-xs leading-relaxed text-base-content">
                        {alert.description}
                      </p>

                      <dl class="mt-3 grid grid-cols-3 gap-2 text-[10px] text-muted">
                        <div>
                          <dt class="uppercase tracking-wide">Time</dt>
                          <dd class="mt-0.5 font-mono text-base-content">
                            {new Date(alert.startTime).toLocaleTimeString('en-US', {
                              hour: '2-digit',
                              minute: '2-digit',
                            })}
                          </dd>
                        </div>
                        <div>
                          <dt class="uppercase tracking-wide">Duration</dt>
                          <dd class="mt-0.5 text-base-content">{alert.duration}</dd>
                        </div>
                        <div class="min-w-0">
                          <dt class="uppercase tracking-wide">Node</dt>
                          <dd
                            class="mt-0.5 truncate text-base-content"
                            title={alert.nodeDisplayName || alert.node || ''}
                          >
                            {alert.nodeDisplayName || alert.node || '—'}
                          </dd>
                        </div>
                      </dl>

                      <AlertHistoryItemActions alert={alert} state={props.state} class="mt-3" />

                      <Show
                        when={
                          alert.source === 'alert' && props.state.expandedIncidents().has(rowKey())
                        }
                      >
                        <div class="mt-3 border-t border-border pt-3">
                          <IncidentTimelinePanel
                            loading={props.state.incidentLoading()[rowKey()]}
                            error={props.state.incidentErrors()[rowKey()]}
                            timeline={props.state.incidentTimelines()[rowKey()]}
                            filters={props.state.historyIncidentEventFilters}
                            setFilters={props.state.setHistoryIncidentEventFilters}
                            filterVariant="compact"
                            eventCardVariant="surface"
                            noteDraft={props.state.incidentNoteDrafts()[rowKey()] || ''}
                            onNoteDraftChange={(value) =>
                              props.state.setIncidentNoteDraft(rowKey(), value)
                            }
                            noteSaving={props.state.incidentNoteSaving().has(rowKey())}
                            onSaveNote={() => {
                              void props.state.saveIncidentNote(
                                rowKey(),
                                alert.id,
                                alert.startTime,
                              );
                            }}
                            onRetry={() => {
                              void props.state.loadIncidentTimeline(
                                rowKey(),
                                alert.id,
                                alert.startTime,
                              );
                            }}
                          />
                        </div>
                      </Show>
                    </article>
                  );
                }}
              </For>
            </div>
          </section>
        )}
      </For>
    </div>
  );
}
