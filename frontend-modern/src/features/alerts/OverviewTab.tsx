import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';

import { createSignal, onCleanup, For, Show, createEffect } from 'solid-js';
import { useLocation } from '@solidjs/router';

import type { Alert } from '@/types/api';
import type { Override } from './types';
import { alertTypeDisplayLabel } from './helpers';
import { getCanonicalAlertId } from './identity';
import { useAlertOverviewState } from './useAlertOverviewState';
import { useAlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import {
  ALERTS_EMPTY_STATE,
  ALERTS_THRESHOLD_HINT,
  getAlertListEmptyState,
  getAlertOverviewAcknowledgedBadgeClass,
  getAlertOverviewCardPresentation,
  getAlertOverviewPrimaryActionClass,
  getAlertOverviewSecondaryActionClass,
  getAlertOverviewStartedAtClass,
} from '@/utils/alertOverviewPresentation';
 
export function OverviewTab(props: {
  overrides: Override[];
  activeAlerts: Record<string, Alert>;
  updateAlert: (alertIdentifier: string, updates: Partial<Alert>) => void;
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
  const [lastHashScrolled, setLastHashScrolled] = createSignal<string | null>(null);
  const {
    alertStats,
    filteredAlerts,
    processingAlerts,
    bulkAckProcessing,
    handleAlertAcknowledgement,
    handleBulkAcknowledge,
  } = useAlertOverviewState({
    activeAlerts: () => props.activeAlerts,
    overrides: () => props.overrides,
    showAcknowledged: props.showAcknowledged,
    updateAlert: props.updateAlert,
  });
  const {
    incidentTimelines,
    incidentLoading,
    incidentErrors,
    expandedIncidents,
    incidentNoteDrafts,
    incidentNoteSaving,
    eventFilters: incidentEventFilters,
    setEventFilters: setIncidentEventFilters,
    loadIncidentTimeline,
    toggleIncidentTimeline,
    setIncidentNoteDraft,
    saveIncidentNote,
  } = useAlertIncidentTimelineState();

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
  });

  return (
    <div class="space-y-4 sm:space-y-6">
      {/* Stats Cards - only show cards not duplicated in sub-tabs */}
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2 sm:gap-4">
        <Card padding="sm" class="sm:p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">
                Acknowledged
              </p>
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
              <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">
                Last 24 Hours
              </p>
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
              <p class="text-[10px] sm:text-sm text-muted uppercase tracking-wider sm:normal-case">
                Guest Overrides
              </p>
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
              <Show
                when={!props.alertsDisabled()}
                fallback={
                  <>
                    <div class="flex justify-center mb-3">
                      <svg
                        class="w-12 h-12 text-muted"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <circle cx="12" cy="12" r="10" stroke-width="2" />
                        <line
                          x1="4"
                          y1="4"
                          x2="20"
                          y2="20"
                          stroke-width="2"
                          stroke-linecap="round"
                        />
                      </svg>
                    </div>
                    <p class="text-sm">Alerting is paused</p>
                    <p class="text-xs mt-1">
                      Toggle alerts on to resume monitoring and unlock configuration tabs
                    </p>
                  </>
                }
              >
                <div class="flex justify-center mb-3">
                  <svg
                    class="w-12 h-12 text-green-500 dark:text-green-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="2"
                      fill="none"
                    />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 12l2 2 4-4"
                    />
                  </svg>
                </div>
                <p class="text-sm">{ALERTS_EMPTY_STATE}</p>
                <p class="text-xs mt-1">{ALERTS_THRESHOLD_HINT}</p>
              </Show>
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
                  onClick={() => {
                    void handleBulkAcknowledge();
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
                {getAlertListEmptyState(props.showAcknowledged())}
              </div>
            </Show>
            <For each={filteredAlerts()}>
              {(alert) => {
                const alertCardPresentation = () =>
                  getAlertOverviewCardPresentation(
                    alert.level ?? 'warning',
                    alert.acknowledged,
                    processingAlerts().has(getCanonicalAlertId(alert)),
                  );

                return (
                  <div
                    id={`alert-${getCanonicalAlertId(alert)}`}
                    class={alertCardPresentation().cardClassName}
                  >
                  <div class="flex flex-col sm:flex-row sm:items-start">
                    <div class="flex items-start flex-1">
                      {/* Status icon */}
                      <div class={alertCardPresentation().iconClassName}>
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
                          <span class={alertCardPresentation().resourceClassName}>
                            {alert.resourceName}
                          </span>
                          <span class="text-xs text-muted">
                            ({alertTypeDisplayLabel(alert.type)})
                          </span>
                          <Show when={alert.node}>
                            <span class="text-xs text-muted">
                              on {alert.nodeDisplayName || alert.node}
                            </span>
                          </Show>
                          <Show when={alert.acknowledged}>
                            <span class={getAlertOverviewAcknowledgedBadgeClass()}>
                              Acknowledged
                            </span>
                          </Show>
                        </div>
                        <p class="text-sm text-base-content mt-1 break-words">{alert.message}</p>
                        <p class={getAlertOverviewStartedAtClass()}>
                          Started: {new Date(alert.startTime).toLocaleString()}
                        </p>
                      </div>
                    </div>
                    <div class="flex flex-wrap items-center gap-1.5 sm:gap-2 mt-3 sm:mt-0 sm:ml-4 self-end sm:self-start justify-end">
                      <button
                        class={getAlertOverviewPrimaryActionClass(alert.acknowledged)}
                        disabled={processingAlerts().has(getCanonicalAlertId(alert))}
                        onClick={async (e) => {
                          e.preventDefault();
                          e.stopPropagation();
                          await handleAlertAcknowledgement(alert);
                        }}
                      >
                        {processingAlerts().has(getCanonicalAlertId(alert))
                          ? 'Processing...'
                          : alert.acknowledged
                            ? 'Unacknowledge'
                            : 'Acknowledge'}
                      </button>
                      <button
                        class={getAlertOverviewSecondaryActionClass()}
                        onClick={() => {
                          void toggleIncidentTimeline(getCanonicalAlertId(alert), alert.startTime);
                        }}
                      >
                        {expandedIncidents().has(getCanonicalAlertId(alert))
                          ? 'Hide Timeline'
                          : 'Timeline'}
                      </button>
                      <InvestigateAlertButton
                        alert={alert}
                        resourceType={
                          typeof alert.metadata?.resourceType === 'string'
                            ? (alert.metadata.resourceType as string)
                            : undefined
                        }
                        variant="text"
                        size="sm"
                        licenseLocked={!props.hasAIAlertsFeature() && !props.licenseLoading()}
                      />
                    </div>
                  </div>
                  <Show when={expandedIncidents().has(getCanonicalAlertId(alert))}>
                    <div class="mt-3 border-t border-border pt-3">
                      <IncidentTimelinePanel
                        loading={incidentLoading()[getCanonicalAlertId(alert)]}
                        error={incidentErrors()[getCanonicalAlertId(alert)]}
                        timeline={incidentTimelines()[getCanonicalAlertId(alert)]}
                        filters={incidentEventFilters}
                        setFilters={setIncidentEventFilters}
                        filterVariant="panel"
                        eventCardVariant="alt"
                        noteDraft={incidentNoteDrafts()[getCanonicalAlertId(alert)] || ''}
                        onNoteDraftChange={(value) =>
                          setIncidentNoteDraft(getCanonicalAlertId(alert), value)
                        }
                        noteSaving={incidentNoteSaving().has(getCanonicalAlertId(alert))}
                        onSaveNote={() => {
                          void saveIncidentNote(getCanonicalAlertId(alert), alert.startTime);
                        }}
                        onRetry={() => {
                          void loadIncidentTimeline(getCanonicalAlertId(alert), alert.startTime);
                        }}
                      />
                    </div>
                  </Show>
                  </div>
                );
              }}
            </For>
          </div>
        </Show>
      </div>
    </div>
  );
}
