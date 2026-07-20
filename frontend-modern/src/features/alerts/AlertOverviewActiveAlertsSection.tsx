import { For, Show } from 'solid-js';

import { SectionHeader } from '@/components/shared/SectionHeader';
import type { Alert } from '@/types/api';
import {
  getAlertOverviewAcknowledgedToggleLabel,
  getAlertOverviewActiveSectionTitle,
  getAlertOverviewBulkAcknowledgeLabel,
  getAlertOverviewEmptyState,
  getAlertOverviewPausedState,
  getAlertListEmptyState,
} from '@/utils/alertOverviewPresentation';

import { AlertOverviewAlertCard } from './AlertOverviewAlertCard';
import { useAlertGroupExpansion } from './useAlertGroupExpansion';
import type { AlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import type { AlertOverviewState } from './useAlertOverviewState';

interface AlertOverviewActiveAlertsSectionProps {
  state: AlertOverviewState;
  timelineState: AlertIncidentTimelineState;
  activeAlerts: Record<string, Alert>;
  alertsDisabled: boolean;
  showAcknowledged: boolean;
  setShowAcknowledged: (value: boolean) => void;
}

export function AlertOverviewActiveAlertsSection(props: AlertOverviewActiveAlertsSectionProps) {
  const { isGroupExpanded, toggleGroup } = useAlertGroupExpansion();
  return (
    <div>
      <SectionHeader title={getAlertOverviewActiveSectionTitle()} size="md" class="mb-3" />
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
                      <line x1="4" y1="4" x2="20" y2="20" stroke-width="2" stroke-linecap="round" />
                    </svg>
                  </div>
                  <p class="text-sm">{getAlertOverviewPausedState().title}</p>
                  <p class="text-xs mt-1">{getAlertOverviewPausedState().description}</p>
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
              <p class="text-sm">{getAlertOverviewEmptyState().title}</p>
              <p class="text-xs mt-1">{getAlertOverviewEmptyState().description}</p>
            </Show>
          </div>
        }
      >
        <Show
          when={props.state.alertStats().acknowledged > 0 || props.state.alertStats().active > 0}
        >
          <div class="flex flex-wrap items-center justify-between gap-1.5 p-1.5 bg-surface-alt rounded-t-lg border border-border">
            <Show when={props.state.alertStats().acknowledged > 0}>
              <button
                onClick={() => props.setShowAcknowledged(!props.showAcknowledged)}
                class="text-xs text-muted hover:text-base-content transition-colors"
              >
                {getAlertOverviewAcknowledgedToggleLabel(props.showAcknowledged)}
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
                {getAlertOverviewBulkAcknowledgeLabel(
                  props.state.alertStats().active,
                  props.state.bulkAckProcessing(),
                )}
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
          <For each={props.state.groupedAlerts()}>
            {(group) => (
              <div>
                <AlertOverviewAlertCard
                  alert={group.primary}
                  state={props.state}
                  timelineState={props.timelineState}
                />
                <Show when={group.related.length > 0}>
                  <div class="ml-4 border-l-2 border-border pl-3 mt-1">
                    <div class="flex items-center gap-3 py-1">
                      <button
                        type="button"
                        class="text-xs text-muted hover:text-base-content transition-colors"
                        onClick={() => toggleGroup(group.key)}
                      >
                        {isGroupExpanded(group.key) ? 'Hide' : `+${group.related.length} related`}
                      </button>
                      <Show when={[group.primary, ...group.related].some((a) => !a.acknowledged)}>
                        <button
                          type="button"
                          class="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors"
                          onClick={() => {
                            void props.state.handleGroupAcknowledge([
                              group.primary,
                              ...group.related,
                            ]);
                          }}
                        >
                          Ack all ({group.related.length + 1})
                        </button>
                      </Show>
                    </div>
                    <Show when={isGroupExpanded(group.key)}>
                      <div class="space-y-2 mt-1">
                        <For each={group.related}>
                          {(alert) => (
                            <AlertOverviewAlertCard
                              alert={alert}
                              state={props.state}
                              timelineState={props.timelineState}
                            />
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </Show>
              </div>
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}
