import { For, Show } from 'solid-js';

import { SectionHeader } from '@/components/shared/SectionHeader';
import type { Alert } from '@/types/api';
import {
  ALERTS_EMPTY_STATE,
  ALERTS_THRESHOLD_HINT,
  getAlertListEmptyState,
} from '@/utils/alertOverviewPresentation';

import { AlertOverviewAlertCard } from './AlertOverviewAlertCard';
import type { AlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import type { AlertOverviewState } from './useAlertOverviewState';

interface AlertOverviewActiveAlertsSectionProps {
  state: AlertOverviewState;
  timelineState: AlertIncidentTimelineState;
  activeAlerts: Record<string, Alert>;
  alertsDisabled: boolean;
  hasAIAlertsFeature: boolean;
  licenseLoading: boolean;
  showAcknowledged: boolean;
  setShowAcknowledged: (value: boolean) => void;
}

export function AlertOverviewActiveAlertsSection(props: AlertOverviewActiveAlertsSectionProps) {
  return (
    <div>
      <SectionHeader title="Active Alerts" size="md" class="mb-3" />
      <Show
        when={Object.keys(props.activeAlerts).length > 0}
        fallback={
          <div class="text-center py-8 text-muted">
            <Show
              when={!props.alertsDisabled}
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
        <Show when={props.state.alertStats().acknowledged > 0 || props.state.alertStats().active > 0}>
          <div class="flex flex-wrap items-center justify-between gap-1.5 p-1.5 bg-surface-alt rounded-t-lg border border-border">
            <Show when={props.state.alertStats().acknowledged > 0}>
              <button
                onClick={() => props.setShowAcknowledged(!props.showAcknowledged)}
                class="text-xs text-muted hover:text-base-content transition-colors"
              >
                {props.showAcknowledged ? 'Hide' : 'Show'} acknowledged
              </button>
            </Show>
            <Show when={props.state.alertStats().active > 0}>
              <button
                type="button"
                class="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-md border border-blue-200 dark:border-blue-700 bg-blue-50 dark:bg-blue-900 text-blue-700 dark:text-blue-200 transition-colors hover:bg-blue-100 dark:hover:bg-blue-900 disabled:opacity-60 disabled:cursor-not-allowed"
                disabled={props.state.bulkAckProcessing()}
                onClick={() => {
                  void props.state.handleBulkAcknowledge();
                }}
              >
                {props.state.bulkAckProcessing()
                  ? 'Acknowledging…'
                  : `Acknowledge all (${props.state.alertStats().active})`}
              </button>
            </Show>
          </div>
        </Show>
        <div class="space-y-2">
          <Show when={props.state.filteredAlerts().length === 0}>
            <div class="text-center py-8 text-muted">
              {getAlertListEmptyState(props.showAcknowledged)}
            </div>
          </Show>
          <For each={props.state.filteredAlerts()}>
            {(alert) => (
              <AlertOverviewAlertCard
                alert={alert}
                state={props.state}
                timelineState={props.timelineState}
                hasAIAlertsFeature={props.hasAIAlertsFeature}
                licenseLoading={props.licenseLoading}
              />
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}
