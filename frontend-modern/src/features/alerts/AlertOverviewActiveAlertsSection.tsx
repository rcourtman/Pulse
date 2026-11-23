import { For, Show } from 'solid-js';

import { FilterBar, filterChipStatusDot, type FilterDef } from '@/components/shared/FilterBar';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { PlatformWindowedList } from '@/features/platformPage/PlatformWindowedList';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { t } from '@/i18n';
import type { Alert } from '@/types/api';
import {
  getAlertHistoryEmptyState,
  getAlertHistorySearchPlaceholder,
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
import type {
  AlertGroup,
  AlertOverviewSeverityFilter,
  AlertOverviewState,
} from './useAlertOverviewState';

function pmgMessageAgeScope(alert: Alert): 'gateway' | 'node' | undefined {
  if (alert.type !== 'message-age' || alert.metadata?.resourceType !== 'pmg' || !alert.resourceId) {
    return undefined;
  }
  // The PMG evaluator uses the gateway resource for both readings. The
  // canonical spec names scope. REST sends it at the top level; connected
  // state retains the same identifier in metadata.
  const specId = alert.canonicalSpecId ?? alert.metadata.canonicalSpecID;
  if (specId === `${alert.resourceId}-oldest-message`) return 'gateway';
  if (alert.node && specId === `${alert.resourceId}-${alert.node}-oldest-message`) {
    return 'node';
  }
  return undefined;
}

function pmgMessageAgeCompanion(group: AlertGroup) {
  const scope = pmgMessageAgeScope(group.primary);
  if (group.correlated || !scope) return undefined;
  // Keep the existing severity/time order, and use only rows already shown
  // by the acknowledgement filter. Other related alerts stay in the group.
  const companionScope = scope === 'gateway' ? 'node' : 'gateway';
  const alert = group.related.find((alert) => pmgMessageAgeScope(alert) === companionScope);
  return alert ? ({ alert, scope, companionScope } as const) : undefined;
}

const pmgScopeLabel = (scope: 'gateway' | 'node') =>
  t(scope === 'gateway' ? 'alerts.overview.group.pmgGateway' : 'alerts.overview.group.pmgNode');

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
  const { isMobile } = useBreakpoint();
  const buildFilters = (): FilterDef[] => [
    {
      id: 'active-alert-severity',
      label: 'Severity',
      group: 'status',
      inline: true,
      value: props.state.severityFilter,
      setValue: (value: string) =>
        props.state.setSeverityFilter(value as AlertOverviewSeverityFilter),
      defaultValue: 'all',
      options: () => {
        const infoCount = props.state.countForSeverity('info');
        return [
          { value: 'all', label: 'All', count: props.state.countForSeverity('all') },
          {
            value: 'critical',
            label: 'Critical',
            leading: filterChipStatusDot('bg-red-500'),
            tone: 'danger' as const,
            count: props.state.countForSeverity('critical'),
          },
          {
            value: 'warning',
            label: 'Warning',
            leading: filterChipStatusDot('bg-amber-500'),
            tone: 'warning' as const,
            count: props.state.countForSeverity('warning'),
          },
          ...(infoCount > 0 || props.state.severityFilter() === 'info'
            ? [
                {
                  value: 'info',
                  label: 'Info',
                  leading: filterChipStatusDot('bg-blue-500'),
                  count: infoCount,
                },
              ]
            : []),
        ];
      },
    },
  ];
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
        <div class="mb-2">
          <FilterBar
            role="group"
            ariaLabel="Active alert filters"
            isMobile={isMobile}
            search={{
              value: props.state.searchTerm,
              setValue: props.state.setSearchTerm,
              placeholder: getAlertHistorySearchPlaceholder(),
              clearOnEscape: true,
            }}
            filters={buildFilters()}
            onClearAll={props.state.clearFilters}
            showClearAll={() => props.state.activeFilterCount() > 0}
          />
        </div>
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
              <Show
                when={props.state.activeFilterCount() > 0}
                fallback={getAlertListEmptyState(props.showAcknowledged)}
              >
                <p class="text-sm">{getAlertHistoryEmptyState().title}</p>
                <p class="mt-1 text-xs">{getAlertHistoryEmptyState().description}</p>
                <button
                  type="button"
                  onClick={props.state.clearFilters}
                  class="mt-2 rounded-md border border-border px-3 py-1.5 text-xs font-medium text-base-content hover:bg-surface-hover"
                >
                  Clear filters
                </button>
              </Show>
            </div>
          </Show>
          <PlatformWindowedList
            items={props.state.groupedAlerts}
            estimatedItemHeight={104}
            enableThreshold={24}
            windowSize={32}
          >
            {(group) => {
              const companion = pmgMessageAgeCompanion(group);
              const folded = companion
                ? group.related.filter((alert) => alert !== companion.alert)
                : group.related;
              return (
                <div>
                  <AlertIncidentSynthesisSummary alert={group.primary} />
                  <AlertOverviewAlertCard
                    alert={group.primary}
                    scopeLabel={companion && pmgScopeLabel(companion.scope)}
                    state={props.state}
                    timelineState={props.timelineState}
                    deliveryPausedGlobally={props.deliveryPausedGlobally}
                  />
                  <Show when={group.related.length > 0}>
                    <div class="ml-4 border-l-2 border-border pl-3 mt-1">
                      <Show
                        when={
                          folded.length > 0 ||
                          [group.primary, ...group.related].some((a) => !a.acknowledged)
                        }
                      >
                        <div class="flex items-center gap-3 py-1">
                          <Show when={folded.length > 0}>
                            <button
                              type="button"
                              class="text-xs text-muted hover:text-base-content transition-colors"
                              onClick={() => toggleGroup(group.key)}
                            >
                              {getAlertOverviewGroupDisclosureLabel({
                                count: folded.length,
                                correlated: group.correlated,
                                expanded: isGroupExpanded(group.key),
                              })}
                            </button>
                          </Show>
                          <Show
                            when={[group.primary, ...group.related].some((a) => !a.acknowledged)}
                          >
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
                              {getAlertOverviewGroupAcknowledgeLabel(group.related.length + 1)}
                            </button>
                          </Show>
                        </div>
                      </Show>
                      <Show when={companion}>
                        {(row) => (
                          <AlertOverviewAlertCard
                            alert={row().alert}
                            scopeLabel={pmgScopeLabel(row().companionScope)}
                            state={props.state}
                            timelineState={props.timelineState}
                            deliveryPausedGlobally={props.deliveryPausedGlobally}
                          />
                        )}
                      </Show>
                      <Show when={folded.length > 0 && isGroupExpanded(group.key)}>
                        <div class="space-y-2 mt-1">
                          <For each={folded}>
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
