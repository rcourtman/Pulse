import { For, Show } from 'solid-js';

import { IncidentEventFilters } from '@/components/Alerts/IncidentEventFilters';
import { IncidentTimelineEventCard } from '@/components/Alerts/IncidentTimelineEventCard';
import { Card } from '@/components/shared/Card';
import {
  getAlertIncidentLevelBadgeClass,
  getAlertIncidentStatusPresentation,
  getAlertIncidentTimelineHeadingClass,
  getAlertIncidentTimelineMetaRowClass,
  getAlertIncidentTimelineOutputClass,
  getAlertResourceIncidentAcknowledgedByLabel,
  getAlertResourceIncidentActivityChipClass,
  getAlertResourceIncidentActivitySummaryClass,
  getAlertResourceIncidentCardClass,
  getAlertResourceIncidentCountLabel,
  getAlertResourceIncidentEmptyState,
  getAlertResourceIncidentFilteredEventsEmptyState,
  getAlertResourceIncidentLoadingState,
  getAlertResourceIncidentPanelTitle,
  getAlertResourceIncidentRefreshLabel,
  getAlertResourceIncidentSummaryRowClass,
  getAlertResourceIncidentToggleButtonClass,
  getAlertResourceIncidentToggleLabel,
  getAlertResourceIncidentTruncatedEventsLabel,
} from '@/utils/alertIncidentPresentation';

import { filterIncidentEvents, summarizeIncidentEvents } from './types';
import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertResourceIncidentsPanelProps {
  state: AlertHistoryState;
}

export function AlertResourceIncidentsPanel(props: AlertResourceIncidentsPanelProps) {
  return (
    <Show when={props.state.resourceIncidentPanel()}>
      {(selection) => {
        const resourceId = selection().resourceId;
        const incidents = () => props.state.resourceIncidents()[resourceId] || [];
        const isLoading = () => props.state.resourceIncidentLoading()[resourceId];

        return (
          <Card padding="md">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 class="text-sm font-semibold text-base-content">
                  {getAlertResourceIncidentPanelTitle()}
                </h3>
                <p class="text-xs text-muted">
                  {selection().resourceName}
                  <Show when={incidents().length > 0}>
                    <span> · {getAlertResourceIncidentCountLabel(incidents().length)}</span>
                  </Show>
                </p>
              </div>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="px-2 py-1 text-xs border rounded-md border-border text-muted hover:bg-surface-hover disabled:opacity-50"
                  disabled={isLoading()}
                  onClick={() => {
                    void props.state.refreshResourceIncidentPanel();
                  }}
                >
                  {getAlertResourceIncidentRefreshLabel(isLoading())}
                </button>
                <button
                  type="button"
                  class="px-2 py-1 text-xs border rounded-md border-border text-muted hover:bg-surface-hover"
                  onClick={() => props.state.setResourceIncidentPanel(null)}
                >
                  Close
                </button>
              </div>
            </div>
            <Show when={isLoading()}>
              <p class="mt-2 text-xs text-muted">{getAlertResourceIncidentLoadingState().text}</p>
            </Show>
            <Show when={!isLoading()}>
              <Show when={incidents().length > 0}>
                <div class="mt-2">
                  <IncidentEventFilters
                    filters={props.state.resourceIncidentEventFilters}
                    setFilters={props.state.setResourceIncidentEventFilters}
                    variant="compact"
                    showQuickSelection
                  />
                </div>
              </Show>
              <Show
                when={incidents().length > 0}
                fallback={
                  <p class="mt-2 text-xs text-muted">
                    {getAlertResourceIncidentEmptyState().text}
                  </p>
                }
              >
                <div class="mt-3 space-y-3">
                  <For each={incidents()}>
                    {(incident) => {
                      const statusPresentation = getAlertIncidentStatusPresentation(
                        incident.status,
                        incident.acknowledged,
                      );
                      const isExpanded = props.state.expandedResourceIncidentIds().has(incident.id);
                      const events = incident.events || [];
                      const filteredEvents = filterIncidentEvents(
                        events,
                        props.state.resourceIncidentEventFilters(),
                      );
                      const eventSummary = summarizeIncidentEvents(filteredEvents);
                      const recentEvents =
                        filteredEvents.length > 6
                          ? filteredEvents.slice(filteredEvents.length - 6)
                          : filteredEvents;
                      const lastEvent =
                        filteredEvents.length > 0
                          ? filteredEvents[filteredEvents.length - 1]
                          : undefined;
                      const filteredLabel =
                        filteredEvents.length !== events.length
                          ? `${filteredEvents.length}/${events.length}`
                          : `${events.length}`;

                      return (
                        <div class={getAlertResourceIncidentCardClass()}>
                          <div class={getAlertIncidentTimelineMetaRowClass()}>
                            <span class={getAlertIncidentTimelineHeadingClass()}>
                              {incident.alertType}
                            </span>
                            <span class={getAlertIncidentLevelBadgeClass(incident.level)}>
                              {incident.level}
                            </span>
                            <span class={statusPresentation.className}>
                              {statusPresentation.label}
                            </span>
                            <span>opened {new Date(incident.openedAt).toLocaleString()}</span>
                            <Show when={incident.closedAt}>
                              <span>
                                closed {new Date(incident.closedAt as string).toLocaleString()}
                              </span>
                            </Show>
                          </div>
                          <Show when={incident.message}>
                            <p class={getAlertIncidentTimelineOutputClass()}>{incident.message}</p>
                          </Show>
                          <Show when={incident.acknowledged && incident.ackUser}>
                            <p class={getAlertIncidentTimelineOutputClass()}>
                              {getAlertResourceIncidentAcknowledgedByLabel(
                                incident.ackUser ?? '',
                              )}
                            </p>
                          </Show>
                          <Show when={events.length > 0}>
                            <div class={getAlertResourceIncidentSummaryRowClass()}>
                              <Show
                                when={filteredEvents.length > 0}
                                fallback={
                                  <span>
                                    {getAlertResourceIncidentFilteredEventsEmptyState().text}
                                  </span>
                                }
                              >
                                <div class={getAlertResourceIncidentActivitySummaryClass()}>
                                  <span class="text-[10px] font-medium uppercase tracking-wide text-muted">
                                    Activity
                                  </span>
                                  <For each={eventSummary}>
                                    {(summary) => (
                                      <span class={getAlertResourceIncidentActivityChipClass()}>
                                        {summary.label} {summary.count}
                                      </span>
                                    )}
                                  </For>
                                  <span>
                                    {filteredEvents.length !== events.length
                                      ? `${filteredEvents.length}/${events.length} events`
                                      : `${events.length} event${events.length === 1 ? '' : 's'}`}
                                  </span>
                                  <Show when={lastEvent}>
                                    <span>Latest: {lastEvent?.summary}</span>
                                  </Show>
                                </div>
                              </Show>
                              <button
                                type="button"
                                class={getAlertResourceIncidentToggleButtonClass()}
                                onClick={() =>
                                  props.state.toggleResourceIncidentDetails(incident.id)
                                }
                              >
                                {getAlertResourceIncidentToggleLabel(isExpanded, filteredLabel)}
                              </button>
                            </div>
                          </Show>
                          <Show when={isExpanded}>
                            <div class="mt-2 space-y-2">
                              <Show
                                when={filteredEvents.length > 0}
                                fallback={
                                  <p class="text-[10px] text-muted">
                                    {getAlertResourceIncidentFilteredEventsEmptyState().text}
                                  </p>
                                }
                              >
                                <For each={recentEvents}>
                                  {(event) => (
                                    <IncidentTimelineEventCard event={event} variant="alt" />
                                  )}
                                </For>
                                <Show when={filteredEvents.length > 0}>
                                  <p class="text-[10px] text-muted">
                                    {getAlertResourceIncidentTruncatedEventsLabel(
                                      recentEvents.length,
                                      filteredEvents.length,
                                    )}
                                  </p>
                                </Show>
                              </Show>
                            </div>
                          </Show>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </Show>
            </Show>
          </Card>
        );
      }}
    </Show>
  );
}
