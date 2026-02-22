import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';

import { createSignal, createMemo, onCleanup, For, Show, createEffect } from 'solid-js';
import { useLocation } from '@solidjs/router';
import type { Incident } from '@/types/api';

import { AlertsAPI } from '@/api/alerts';
import type { Alert, IncidentEvent } from '@/types/api';
import type { Override } from './types';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';



const INCIDENT_EVENT_TYPES = [
  'alert_fired',
  'alert_acknowledged',
  'alert_unacknowledged',
  'alert_resolved',
  'ai_analysis',
  'command',
  'runbook',
  'note',
] as const;

const INCIDENT_EVENT_LABELS: Record<(typeof INCIDENT_EVENT_TYPES)[number], string> = {
  alert_fired: 'Fired',
  alert_acknowledged: 'Ack',
  alert_unacknowledged: 'Unack',
  alert_resolved: 'Resolved',
  ai_analysis: 'Patrol',
  command: 'Cmd',
  runbook: 'Runbook',
  note: 'Note',
};

function filterIncidentEvents(
  events: IncidentEvent[] | undefined,
  filters: Set<string>,
): IncidentEvent[] {
  if (!events) return [];
  if (filters.size === 0) return events;
  return events.filter((e) => filters.has(e.type));
}

function IncidentEventFilters(props: {
  filters: () => Set<string>;
  setFilters: (next: Set<string>) => void;
}) {
  const toggleFilter = (type: (typeof INCIDENT_EVENT_TYPES)[number]) => {
    const next = new Set(props.filters());
    if (next.has(type)) {
      next.delete(type);
    } else {
      next.add(type);
    }
    props.setFilters(next);
  };

  return (
    <div class="flex flex-wrap items-center gap-1.5 p-2 bg-surface-alt/50 rounded border border-border">
      <span class="text-xs font-medium text-muted mr-1">Filter events:</span>
      <For each={INCIDENT_EVENT_TYPES}>
        {(type) => {
          const selected = () => props.filters().has(type);
          return (
            <button
              onClick={() => toggleFilter(type)}
              class={`px-2 py-0.5 rounded text-[10px] font-medium transition-colors ${
 selected()
 ? 'bg-blue-100 text-blue-700 border-blue-200 dark:bg-blue-900/60 dark:text-blue-300 dark:border-blue-800'
 : ' text-slate-600 border-slate-200 hover:bg-surface-alt dark:text-slate-400 dark:border-slate-700 dark:hover:bg-slate-700'
 } border`}
            >
              {INCIDENT_EVENT_LABELS[type]}
            </button>
          );
        }}
      </For>
    </div>
  );
}

// Overview Tab - Shows current alert status
export function OverviewTab(props: {
  overrides: Override[];
  activeAlerts: Record<string, Alert>;
  updateAlert: (alertId: string, updates: Partial<Alert>) => void;
  showQuickTip: () => boolean;
  dismissQuickTip: () => void;
  showAcknowledged: () => boolean;
  setShowAcknowledged: (value: boolean) => void;
  alertsDisabled: () => boolean;
  hasAIAlertsFeature: () => boolean;
  licenseLoading: () => boolean;
}) {
  const location = useLocation();
  let hashScrollRafId: number | undefined;
  const pendingProcessingResetTimeouts = new Set<number>();
  // Loading states for buttons
  const [processingAlerts, setProcessingAlerts] = createSignal<Set<string>>(new Set());
  const [incidentTimelines, setIncidentTimelines] = createSignal<Record<string, Incident | null>>({});
  const [incidentLoading, setIncidentLoading] = createSignal<Record<string, boolean>>({});
  const [expandedIncidents, setExpandedIncidents] = createSignal<Set<string>>(new Set());
  const [incidentNoteDrafts, setIncidentNoteDrafts] = createSignal<Record<string, string>>({});
  const [incidentNoteSaving, setIncidentNoteSaving] = createSignal<Set<string>>(new Set());
  const [incidentEventFilters, setIncidentEventFilters] = createSignal<Set<string>>(
    new Set(INCIDENT_EVENT_TYPES),
  );
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);
  const processingReleaseTimers = new Map<string, ReturnType<typeof setTimeout>>();

  const clearProcessingReleaseTimer = (alertId: string) => {
    const timer = processingReleaseTimers.get(alertId);
    if (timer === undefined) {
      return;
    }
    clearTimeout(timer);
    processingReleaseTimers.delete(alertId);
  };

  onCleanup(() => {
    processingReleaseTimers.forEach((timer) => clearTimeout(timer));
    processingReleaseTimers.clear();
  });

  const loadIncidentTimeline = async (alertId: string, startedAt?: string) => {
    setIncidentLoading((prev) => ({ ...prev, [alertId]: true }));
    try {
      const timeline = await AlertsAPI.getIncidentTimeline(alertId, startedAt);
      setIncidentTimelines((prev) => ({ ...prev, [alertId]: timeline }));
    } catch (error) {
      logger.error('Failed to load incident timeline', error);
      notificationStore.error('Failed to load incident timeline');
    } finally {
      setIncidentLoading((prev) => ({ ...prev, [alertId]: false }));
    }
  };

  const toggleIncidentTimeline = async (alertId: string, startedAt?: string) => {
    const expanded = expandedIncidents();
    const next = new Set(expanded);
    if (next.has(alertId)) {
      next.delete(alertId);
      setExpandedIncidents(next);
      return;
    }
    next.add(alertId);
    setExpandedIncidents(next);
    if (!(alertId in incidentTimelines())) {
      await loadIncidentTimeline(alertId, startedAt);
    }
  };

  const saveIncidentNote = async (alertId: string, startedAt?: string) => {
    const note = (incidentNoteDrafts()[alertId] || '').trim();
    if (!note) {
      return;
    }
    setIncidentNoteSaving((prev) => new Set(prev).add(alertId));
    try {
      const incidentId = incidentTimelines()[alertId]?.id;
      await AlertsAPI.addIncidentNote({ alertId, incidentId, note });
      setIncidentNoteDrafts((prev) => ({ ...prev, [alertId]: '' }));
      await loadIncidentTimeline(alertId, startedAt);
      notificationStore.success('Incident note saved');
    } catch (error) {
      logger.error('Failed to save incident note', error);
      notificationStore.error('Failed to save incident note');
    } finally {
      setIncidentNoteSaving((prev) => {
        const next = new Set(prev);
        next.delete(alertId);
        return next;
      });
    }
  };

  // Get alert stats from actual active alerts
  const alertStats = createMemo(() => {
    // Access the store properly for reactivity
    const alertIds = Object.keys(props.activeAlerts);
    const alerts = alertIds.map((id) => props.activeAlerts[id]);
    return {
      active: alerts.filter((a) => !a.acknowledged).length,
      acknowledged: alerts.filter((a) => a.acknowledged).length,
      total24h: alerts.length, // In real app, would filter by time
      overrides: props.overrides.length,
    };
  });


  const filteredAlerts = createMemo(() => {
    const alerts = Object.values(props.activeAlerts);
    // Sort: unacknowledged first, then by start time (newest first)
    return alerts
      .filter((alert) => props.showAcknowledged() || !alert.acknowledged)
      .sort((a, b) => {
        // Acknowledged status comparison first
        if (a.acknowledged !== b.acknowledged) {
          return a.acknowledged ? 1 : -1; // Unacknowledged first
        }
        // Then by time
        return new Date(b.startTime).getTime() - new Date(a.startTime).getTime();
      });
  });

  const unacknowledgedAlerts = createMemo(() =>
    Object.values(props.activeAlerts).filter((alert) => !alert.acknowledged),
  );

  const [bulkAckProcessing, setBulkAckProcessing] = createSignal(false);

  const scrollToAlertHash = () => {
    const hash = location.hash;
    if (!hash || !hash.startsWith('#alert-')) {
      setLastHashScrolled(null);
      return;
    }
    if (hash === lastHashScrolled()) {
      return;
    }
    const target = document.getElementById(hash.slice(1));
    if (!target) {
      return;
    }
    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setLastHashScrolled(hash);
  };

  createEffect(() => {
    location.hash;
    filteredAlerts().length;
    props.showAcknowledged();
    if (hashScrollRafId !== undefined) {
      cancelAnimationFrame(hashScrollRafId);
    }
    hashScrollRafId = requestAnimationFrame(() => {
      hashScrollRafId = undefined;
      scrollToAlertHash();
    });
  });

  onCleanup(() => {
    if (hashScrollRafId !== undefined) {
      cancelAnimationFrame(hashScrollRafId);
      hashScrollRafId = undefined;
    }
    pendingProcessingResetTimeouts.forEach((timeoutId) => {
      window.clearTimeout(timeoutId);
    });
    pendingProcessingResetTimeouts.clear();
  });

  return (
    <div class="space-y-4 sm:space-y-6">
      {/* Stats Cards - only show cards not duplicated in sub-tabs */}
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2 sm:gap-4">

        <Card padding="sm" class="sm:p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">Acknowledged</p>
              <p class="text-lg sm:text-2xl font-semibold text-yellow-600 dark:text-yellow-400">
                {alertStats().acknowledged}
              </p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-yellow-100 dark:bg-yellow-900 rounded-full flex items-center justify-center">
              <svg
                width="16"
                height="16"
                class="sm:w-5 sm:h-5 text-yellow-600 dark:text-yellow-400"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M9 11L12 14L22 4"></path>
                <path d="M21 12V19C21 20.1046 20.1046 21 19 21H5C3.89543 21 3 20.1046 3 19V5C3 3.89543 3.89543 3 5 3H16"></path>
              </svg>
            </div>
          </div>
        </Card>

        <Card padding="sm" class="sm:p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">Last 24 Hours</p>
              <p class="text-lg sm:text-2xl font-semibold text-base-content">
                {alertStats().total24h}
              </p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-surface-hover rounded-full flex items-center justify-center">
              <svg
                width="16"
                height="16"
                class="sm:w-5 sm:h-5 text-muted"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <polyline points="12 6 12 12 16 14"></polyline>
              </svg>
            </div>
          </div>
        </Card>

        <Card padding="sm" class="sm:p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">Guest Overrides</p>
              <p class="text-lg sm:text-2xl font-semibold text-blue-600 dark:text-blue-400">
                {alertStats().overrides}
              </p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-blue-100 dark:bg-blue-900 rounded-full flex items-center justify-center">
              <svg
                width="16"
                height="16"
                class="sm:w-5 sm:h-5 text-blue-600 dark:text-blue-400"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
              </svg>
            </div>
          </div>
        </Card>
      </div>

      {/* Active Alerts */}
      <div>
        <SectionHeader title="Active Alerts" size="md" class="mb-3" />
        <Show
          when={Object.keys(props.activeAlerts).length > 0}
          fallback={
            <div class="text-center py-8 text-muted">
              <div class="flex justify-center mb-3">
                <svg class="w-12 h-12 text-green-500 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4" />
                </svg>
              </div>
              <p class="text-sm">No active alerts</p>
              <p class="text-xs mt-1">Alerts will appear here when thresholds are exceeded</p>
            </div>
          }
        >
          <Show when={alertStats().acknowledged > 0 || alertStats().active > 0}>
            <div class="flex flex-wrap items-center justify-between gap-1.5 p-1.5 bg-surface-alt rounded-t-lg border border-border">
              <Show when={alertStats().acknowledged > 0}>
                <button
                  onClick={() => props.setShowAcknowledged(!props.showAcknowledged())}
                  class="text-xs text-muted hover:text-base-content transition-colors"
                >
                  {props.showAcknowledged() ? 'Hide' : 'Show'} acknowledged
                </button>
              </Show>
              <Show when={alertStats().active > 0}>
                <button
                  type="button"
                  class="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-md border border-blue-200 dark:border-blue-700 bg-blue-50 dark:bg-blue-900 text-blue-700 dark:text-blue-200 transition-colors hover:bg-blue-100 dark:hover:bg-blue-900 disabled:opacity-60 disabled:cursor-not-allowed"
                  disabled={bulkAckProcessing()}
                  onClick={async () => {
                    if (bulkAckProcessing()) return;
                    const pending = unacknowledgedAlerts();
                    if (pending.length === 0) {
                      return;
                    }
                    setBulkAckProcessing(true);
                    try {
                      const result = await AlertsAPI.bulkAcknowledge(pending.map((alert) => alert.id));
                      const successes = result.results.filter((r) => r.success);
                      const failures = result.results.filter((r) => !r.success);

                      successes.forEach((res) => {
                        props.updateAlert(res.alertId, {
                          acknowledged: true,
                          ackTime: new Date().toISOString(),
                        });
                      });

                      if (successes.length > 0) {
                        notificationStore.success(
                          `Acknowledged ${successes.length} ${successes.length === 1 ? 'alert' : 'alerts'}.`,
                        );
                      }

                      if (failures.length > 0) {
                        notificationStore.error(
                          `Failed to acknowledge ${failures.length} ${failures.length === 1 ? 'alert' : 'alerts'}.`,
                        );
                      }
                    } catch (error) {
                      logger.error('Bulk acknowledge failed', error);
                      notificationStore.error('Failed to acknowledge alerts');
                    } finally {
                      setBulkAckProcessing(false);
                    }
                  }}
                >
                  {bulkAckProcessing()
                    ? 'Acknowledging…'
                    : `Acknowledge all (${alertStats().active})`}
                </button>
              </Show>
            </div>
          </Show>
          <div class="space-y-2">
            <Show when={filteredAlerts().length === 0}>
              <div class="text-center py-8 text-muted">
                {props.showAcknowledged() ? 'No active alerts' : 'No unacknowledged alerts'}
              </div>
            </Show>
            <For each={filteredAlerts()}>
              {(alert) => (
                <div
                  id={`alert-${alert.id}`}
                  class={`border rounded-md p-3 sm:p-4 transition-all ${processingAlerts().has(alert.id) ? 'opacity-50' : ''
 } ${alert.acknowledged
 ? 'opacity-60 border-border bg-surface-alt'
 : alert.level === 'critical'
 ? 'border-red-300 dark:border-red-800 bg-red-50 dark:bg-red-900'
 : 'border-yellow-300 dark:border-yellow-800 bg-yellow-50 dark:bg-yellow-900'
 }`}
                >
                  <div class="flex flex-col sm:flex-row sm:items-start">
                    <div class="flex items-start flex-1">
                      {/* Status icon */}
                      <div
                        class={`mr-3 mt-0.5 transition-all ${alert.acknowledged
 ? 'text-green-600 dark:text-green-400'
 : alert.level === 'critical'
 ? 'text-red-600 dark:text-red-400'
 : 'text-yellow-600 dark:text-yellow-400'
 }`}
                      >
                        {alert.acknowledged ? (
                          // Checkmark for acknowledged
                          <svg
                            class="w-5 h-5"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                            />
                          </svg>
                        ) : (
                          // Warning/Alert icon
                          <svg
                            class="w-5 h-5"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
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
                          <span
                            class={`text-sm font-medium truncate ${alert.level === 'critical'
 ? 'text-red-700 dark:text-red-400'
 : 'text-yellow-700 dark:text-yellow-400'
 }`}
                          >
                            {alert.resourceName}
                          </span>
                          <span class="text-xs text-muted">
                            ({alert.type})
                          </span>
                          <Show when={alert.node}>
                            <span class="text-xs text-muted">
                              on {alert.nodeDisplayName || alert.node}
                            </span>
                          </Show>
                          <Show when={alert.acknowledged}>
                            <span class="px-2 py-0.5 text-xs bg-yellow-200 dark:bg-yellow-800 text-yellow-800 dark:text-yellow-200 rounded">
                              Acknowledged
                            </span>
                          </Show>
                        </div>
                        <p class="text-sm text-base-content mt-1 break-words">
                          {alert.message}
                        </p>
                        <p class="text-xs text-muted mt-1">
                          Started: {new Date(alert.startTime).toLocaleString()}
                        </p>
                      </div>
                    </div>
                    <div class="flex flex-wrap items-center gap-1.5 sm:gap-2 mt-3 sm:mt-0 sm:ml-4 self-end sm:self-start justify-end">
                      <button
                        class={`px-3 py-1.5 text-xs font-medium border rounded-md transition-all disabled:opacity-50 disabled:cursor-not-allowed ${alert.acknowledged
 ? 'bg-white dark:bg-slate-700 text-base-content border-border hover:bg-slate-50 dark:hover:bg-slate-600'
 : 'bg-white dark:bg-slate-700 text-yellow-700 dark:text-yellow-300 border-yellow-300 dark:border-yellow-700 hover:bg-yellow-50 dark:hover:bg-yellow-900'
 }`}
                        disabled={processingAlerts().has(alert.id)}
                        onClick={async (e) => {
                          e.preventDefault();
                          e.stopPropagation();

                          // Prevent double-clicks
                          if (processingAlerts().has(alert.id)) return;

                          setProcessingAlerts((prev) => new Set(prev).add(alert.id));

                          // Store current state to avoid race conditions
                          const wasAcknowledged = alert.acknowledged;

                          try {
                            if (wasAcknowledged) {
                              // Call API first, only update local state if successful
                              await AlertsAPI.unacknowledge(alert.id);
                              // Only update local state after successful API call
                              props.updateAlert(alert.id, {
                                acknowledged: false,
                                ackTime: undefined,
                                ackUser: undefined,
                              });
                              notificationStore.success('Alert restored');
                            } else {
                              // Call API first, only update local state if successful
                              await AlertsAPI.acknowledge(alert.id);
                              // Only update local state after successful API call
                              props.updateAlert(alert.id, {
                                acknowledged: true,
                                ackTime: new Date().toISOString(),
                              });
                              notificationStore.success('Alert acknowledged');
                            }
                          } catch (err) {
                            logger.error(
                              `Failed to ${wasAcknowledged ? 'unacknowledge' : 'acknowledge'} alert:`,
                              err,
                            );
                            notificationStore.error(
                              `Failed to ${wasAcknowledged ? 'restore' : 'acknowledge'} alert`,
                            );
                            // Don't update local state on error - let WebSocket keep the correct state
                          } finally {
                            // Keep button disabled for longer to prevent race conditions with WebSocket updates
                            clearProcessingReleaseTimer(alert.id);
                            const timer = setTimeout(() => {
                              processingReleaseTimers.delete(alert.id);
                              setProcessingAlerts((prev) => {
                                const next = new Set(prev);
                                next.delete(alert.id);
                                return next;
                              });
                            }, 1500); // 1.5 seconds to allow server to process and WebSocket to sync
                            processingReleaseTimers.set(alert.id, timer);
                          }
                        }}
                      >
                        {processingAlerts().has(alert.id)
                          ? 'Processing...'
                          : alert.acknowledged
                            ? 'Unacknowledge'
                            : 'Acknowledge'}
                      </button>
                      <button
                        class="px-3 py-1.5 text-xs font-medium border rounded-md transition-all bg-surface text-base-content border-border hover:bg-slate-50 dark:hover:bg-slate-600"
                        onClick={() => {
                          void toggleIncidentTimeline(alert.id, alert.startTime);
                        }}
                      >
                        {expandedIncidents().has(alert.id) ? 'Hide Timeline' : 'Timeline'}
                      </button>
                      <InvestigateAlertButton
                        alert={alert}
                        variant="text"
                        size="sm"
                        licenseLocked={!props.hasAIAlertsFeature() && !props.licenseLoading()}
                      />
                    </div>
                  </div>
                  <Show when={expandedIncidents().has(alert.id)}>
                    <div class="mt-3 border-t border-border pt-3">
                      <Show when={incidentLoading()[alert.id]}>
                        <p class="text-xs text-muted">Loading timeline...</p>
                      </Show>
                      <Show when={!incidentLoading()[alert.id]}>
                        <Show when={incidentTimelines()[alert.id]}>
                          {(timeline) => (
                            <div class="space-y-3">
                              <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
                                <span class="font-medium text-base-content">Incident</span>
                                <span>{timeline().status}</span>
                                <Show when={timeline().acknowledged}>
                                  <span class="px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                                    acknowledged
                                  </span>
                                </Show>
                                <Show when={timeline().openedAt}>
                                  <span>opened {new Date(timeline().openedAt).toLocaleString()}</span>
                                </Show>
                                <Show when={timeline().closedAt}>
                                  <span>closed {new Date(timeline().closedAt as string).toLocaleString()}</span>
                                </Show>
                              </div>
                              {(() => {
                                const events = timeline().events || [];
                                const filteredEvents = filterIncidentEvents(events, incidentEventFilters());
                                return (
                                  <>
                                    <Show when={events.length > 0}>
                                      <IncidentEventFilters
                                        filters={incidentEventFilters}
                                        setFilters={setIncidentEventFilters}
                                      />
                                    </Show>
                                    <Show when={filteredEvents.length > 0}>
                                      <div class="space-y-2">
                                        <For each={filteredEvents}>
                                          {(event) => (
                                            <div class="rounded border border-border bg-surface-alt p-2">
                                              <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
                                                <span class="font-medium text-base-content">
                                                  {event.summary}
                                                </span>
                                                <span>{new Date(event.timestamp).toLocaleString()}</span>
                                              </div>
                                              <Show when={event.details && (event.details as { note?: string }).note}>
                                                <p class="text-xs text-base-content mt-1">
                                                  {(event.details as { note?: string }).note}
                                                </p>
                                              </Show>
                                              <Show when={event.details && (event.details as { command?: string }).command}>
                                                <p class="text-xs text-base-content mt-1 font-mono">
                                                  {(event.details as { command?: string }).command}
                                                </p>
                                              </Show>
                                              <Show when={event.details && (event.details as { output_excerpt?: string }).output_excerpt}>
                                                <p class="text-xs text-muted mt-1">
                                                  {(event.details as { output_excerpt?: string }).output_excerpt}
                                                </p>
                                              </Show>
                                            </div>
                                          )}
                                        </For>
                                      </div>
                                    </Show>
                                    <Show when={events.length > 0 && filteredEvents.length === 0}>
                                      <p class="text-xs text-muted">
                                        No timeline events match the selected filters.
                                      </p>
                                    </Show>
                                    <Show when={events.length === 0}>
                                      <p class="text-xs text-muted">No timeline events yet.</p>
                                    </Show>
                                  </>
                                );
                              })()}
                              <div class="flex flex-col gap-2">
                                <textarea
                                  class="w-full rounded border border-border bg-surface p-2 text-xs text-base-content"
                                  rows={2}
                                  placeholder="Add a note for this incident..."
                                  value={incidentNoteDrafts()[alert.id] || ''}
                                  onInput={(e) => {
                                    const value = e.currentTarget.value;
                                    setIncidentNoteDrafts((prev) => ({ ...prev, [alert.id]: value }));
                                  }}
                                />
                                <div class="flex justify-end">
                                  <button
                                    class="px-3 py-1.5 text-xs font-medium border rounded-md transition-all bg-surface text-base-content border-border hover:bg-slate-50 dark:hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed"
                                    disabled={incidentNoteSaving().has(alert.id) || !(incidentNoteDrafts()[alert.id] || '').trim()}
                                    onClick={() => {
                                      void saveIncidentNote(alert.id, alert.startTime);
                                    }}
                                  >
                                    {incidentNoteSaving().has(alert.id) ? 'Saving...' : 'Save Note'}
                                  </button>
                                </div>
                              </div>
                            </div>
                          )}
                        </Show>
                        <Show when={!incidentTimelines()[alert.id]}>
                          <p class="text-xs text-muted">No incident timeline available.</p>
                        </Show>
                      </Show>
                    </div>
                  </Show>
                </div>
              )}
            </For>
          </div>
        </Show>
      </div>
    </div>
  );
}