import { createMemo, createUniqueId, For, Show } from 'solid-js';

import { SectionHeader } from '@/components/shared/SectionHeader';
import { PlatformWindowedList } from '@/features/platformPage/PlatformWindowedList';
import type { Alert } from '@/types/api';
import {
  formatAlertSeverityLabel,
  getAlertSeverityBadgeClass,
} from '@/utils/alertSeverityPresentation';
import {
  getAlertOverviewAcknowledgedToggleLabel,
  getAlertOverviewActiveSectionTitle,
  getAlertOverviewBulkAcknowledgeLabel,
  getAlertOverviewEmptyState,
  getAlertOverviewGroupAcknowledgeLabel,
  getAlertOverviewGroupDisclosureLabel,
  getAlertOverviewPausedState,
  getAlertListEmptyState,
} from '@/utils/alertOverviewPresentation';

import { AlertOverviewAlertCard } from './AlertOverviewAlertCard';
import { AlertOverviewStatsCards } from './AlertOverviewStatsCards';
import { useAlertGroupExpansion } from './useAlertGroupExpansion';
import { AlertIncidentSynthesisSummary } from './AlertIncidentSynthesisSummary';
import type { AlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import type { AlertOverviewState } from './useAlertOverviewState';

interface AlertOverviewActiveAlertsSectionProps {
  state: AlertOverviewState;
  timelineState: AlertIncidentTimelineState;
  activeAlerts: Record<string, Alert>;
  alertsDisabled: boolean;
  showAcknowledged: boolean;
  setShowAcknowledged: (value: boolean) => void;
  deliveryPausedGlobally?: boolean;
}

export function AlertOverviewActiveAlertsSection(props: AlertOverviewActiveAlertsSectionProps) {
  const { isGroupExpanded, toggleGroup } = useAlertGroupExpansion();
  return (
    <div>
      <div class="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <SectionHeader title={getAlertOverviewActiveSectionTitle()} size="md" />
        <AlertOverviewStatsCards state={props.state} />
      </div>
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
                class="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-md border border-blue-200 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/25 text-blue-700 dark:text-blue-200 transition-colors hover:bg-blue-100 dark:hover:bg-blue-900/25 disabled:opacity-60 disabled:cursor-not-allowed"
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
          <PlatformWindowedList
            items={props.state.groupedAlerts}
            estimatedItemHeight={104}
            enableThreshold={24}
            windowSize={32}
          >
            {(group) => {
              const relatedAlertsId = createUniqueId();
              // The correlation's designated primary may be less severe than
              // its supporting signals. Keep that identity, but do not hide
              // unacknowledged critical alerts behind an unqualified count.
              const relatedSeverities = createMemo(() =>
                (['critical', 'warning', 'info'] as const)
                  .map((level) => ({
                    level,
                    count: group.related.filter(
                      (alert) => !alert.acknowledged && alert.level === level,
                    ).length,
                  }))
                  .filter(({ count }) => count > 0),
              );
              return (
                <div>
                  <AlertIncidentSynthesisSummary alert={group.primary} />
                  <AlertOverviewAlertCard
                    alert={group.primary}
                    state={props.state}
                    timelineState={props.timelineState}
                    deliveryPausedGlobally={props.deliveryPausedGlobally}
                  />
                  <Show when={group.related.length > 0}>
                    <div class="ml-4 border-l-2 border-border pl-3 mt-1">
                      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 py-1">
                        <button
                          type="button"
                          class="inline-flex min-h-11 flex-wrap items-center gap-2 text-left text-xs text-muted hover:text-base-content transition-colors sm:min-h-6"
                          aria-expanded={isGroupExpanded(group.key)}
                          aria-controls={isGroupExpanded(group.key) ? relatedAlertsId : undefined}
                          onClick={() => toggleGroup(group.key)}
                        >
                          <span>
                            {getAlertOverviewGroupDisclosureLabel({
                              count: group.related.length,
                              correlated: group.correlated,
                              expanded: isGroupExpanded(group.key),
                            })}
                          </span>
                          <For each={relatedSeverities()}>
                            {({ level, count }) => (
                              <span class={getAlertSeverityBadgeClass(level)}>
                                {count} {formatAlertSeverityLabel(level)}
                              </span>
                            )}
                          </For>
                        </button>
                        <Show when={[group.primary, ...group.related].some((a) => !a.acknowledged)}>
                          <button
                            type="button"
                            class="min-h-11 text-xs text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors sm:min-h-6"
                            onClick={() => {
                              void props.state.handleGroupAcknowledge([
                                group.primary,
                                ...group.related,
                              ]);
                            }}
                          >
                            {getAlertOverviewGroupAcknowledgeLabel(group.related.length + 1)}
                          </button>
                        </Show>
                      </div>
                      <Show when={isGroupExpanded(group.key)}>
                        <div id={relatedAlertsId} class="space-y-2 mt-1">
                          <For each={group.related}>
                            {(alert) => (
                              <AlertOverviewAlertCard
                                alert={alert}
                                state={props.state}
                                timelineState={props.timelineState}
                                deliveryPausedGlobally={props.deliveryPausedGlobally}
                              />
                            )}
                          </For>
                        </div>
                      </Show>
                    </div>
                  </Show>
                </div>
              );
            }}
          </PlatformWindowedList>
        </div>
      </Show>
    </div>
  );
}
