import { createMemo, For, Show } from 'solid-js';

import type { Resource } from '@/types/resource';
import { useWebSocket } from '@/App';
import { IncidentEventFilters } from '@/components/Alerts/IncidentEventFilters';
import { IncidentTimelineEventCard } from '@/components/Alerts/IncidentTimelineEventCard';
import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';
import { Card } from '@/components/shared/Card';
import { LabeledFilterSelect } from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
import { SectionHeader } from '@/components/shared/SectionHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import {
  getAlertAdministrationClearHistoryLabel,
  getAlertAdministrationSectionDescription,
  getAlertAdministrationSectionTitle,
} from '@/utils/alertAdministrationPresentation';
import {
  getAlertBucketCountLabel,
  getAlertHistoryEmptyState,
  getAlertHistoryLoadingState,
  getAlertHistorySearchPlaceholder,
} from '@/utils/alertOverviewPresentation';
import {
  getAlertFrequencyClearFilterButtonClass,
  getAlertFrequencySelectionPresentation,
} from '@/utils/alertFrequencyPresentation';
import {
  getAlertHistoryResourceTypeBadgeClass,
  getAlertHistorySourcePresentation,
} from '@/utils/alertHistoryPresentation';
import {
  getAlertHistoryStatusPresentation,
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
  getAlertResourceIncidentViewTitle,
} from '@/utils/alertIncidentPresentation';
import { getAlertSeverityDotClass } from '@/utils/alertSeverityPresentation';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';

import { MS_PER_HOUR } from '../alertHistoryModel';
import { useAlertHistoryState } from '../useAlertHistoryState';
import { filterIncidentEvents, summarizeIncidentEvents } from '../types';

export interface HistoryTabProps {
  hasAIAlertsFeature: () => boolean;
  licenseLoading: () => boolean;
  getResource: (resourceId: string) => Resource | undefined;
  allResources: () => Resource[];
}

export function HistoryTab(props: HistoryTabProps) {
  const { activeAlerts } = useWebSocket();
  const alertFrequencySelectionPresentation = createMemo(() =>
    getAlertFrequencySelectionPresentation(),
  );
  const { isMobile } = useBreakpoint();
  const {
    STORAGE_KEYS,
    timeFilter,
    setTimeFilter,
    severityFilter,
    setSeverityFilter,
    searchTerm,
    setSearchTerm,
    alertHistory,
    loading,
    selectedBarIndex,
    setSelectedBarIndex,
    resourceIncidentPanel,
    setResourceIncidentPanel,
    resourceIncidents,
    resourceIncidentLoading,
    expandedResourceIncidentIds,
    resourceIncidentEventFilters,
    setResourceIncidentEventFilters,
    filtersOpen,
    setFiltersOpen,
    activeFilterCount,
    incidentTimelines,
    incidentLoading,
    incidentErrors,
    expandedIncidents,
    incidentNoteDrafts,
    incidentNoteSaving,
    historyIncidentEventFilters,
    setHistoryIncidentEventFilters,
    loadIncidentTimeline,
    toggleIncidentTimeline,
    setIncidentNoteDraft,
    saveIncidentNote,
    openResourceIncidentPanel,
    refreshResourceIncidentPanel,
    toggleResourceIncidentDetails,
    alertData,
    groupedAlerts,
    alertTrends,
    bucketDurationLabel,
    rangeSummary,
    axisTicks,
    selectedBucketDetails,
    formatBucketRange,
    getIncidentRowKey,
    clearAlertHistory,
  } = useAlertHistoryState({
    activeAlerts: () => activeAlerts || {},
    getResource: props.getResource,
    allResources: props.allResources,
  });

  return (
    <div class="space-y-4">
      <Card padding="md">
        <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
          <SectionHeader
            title="Alert frequency"
            description={<span class="text-xs text-muted">{alertData().length} alerts</span>}
            size="sm"
            class="flex-1"
          />
          <div class="flex flex-col items-start gap-2 sm:items-end">
            <Show when={selectedBucketDetails()}>
              {(selection) => (
                <div class={alertFrequencySelectionPresentation().containerClass}>
                  <span class={alertFrequencySelectionPresentation().labelClass}>
                    Filtered Range
                  </span>
                  <span class="font-mono text-[11px]">{selection().rangeLabel}</span>
                </div>
              )}
            </Show>
            <div class="flex flex-col items-start gap-1 text-xs text-muted sm:items-end">
              <div>
                <span class="font-medium text-muted">Bar size:</span> {bucketDurationLabel()}
              </div>
              <Show when={rangeSummary()}>
                {(summary) => (
                  <div class="flex items-center gap-1 whitespace-nowrap">
                    <span class="font-medium text-muted">Range:</span>
                    <span>{summary().startLabel}</span>
                    <span class="text-muted">→</span>
                    <span>{summary().endLabel}</span>
                  </div>
                )}
              </Show>
            </div>
            <div class="flex flex-wrap items-center justify-end gap-2">
              <Show when={selectedBarIndex() !== null}>
                <button
                  type="button"
                  onClick={() => setSelectedBarIndex(null)}
                  class={getAlertFrequencyClearFilterButtonClass()}
                >
                  Clear filter
                </button>
              </Show>
              <div class="flex items-center gap-2 text-xs text-muted">
                <span class="flex items-center gap-1">
                  <div class={getAlertSeverityDotClass('warning')}></div>
                  {alertData().filter((alert) => alert.severity === 'warning').length} warnings
                </span>
                <span class="flex items-center gap-1">
                  <div class={getAlertSeverityDotClass('critical')}></div>
                  {alertData().filter((alert) => alert.severity === 'critical').length} critical
                </span>
              </div>
            </div>
          </div>
        </div>

        <div class="mb-1 text-[10px] text-muted">
          Showing {alertTrends().buckets.length} time periods ({bucketDurationLabel()} each) ·
          Total: {alertData().length} alerts
        </div>

        {(() => {
          const trends = alertTrends();
          return (
            <div class="rounded bg-surface-alt p-1">
              <div class="flex h-12 items-end gap-1">
                {trends.buckets.map((value, index) => {
                  const scaledHeight =
                    value > 0 ? Math.min(100, Math.max(20, Math.log(value + 1) * 20)) : 0;
                  const pixelHeight = value > 0 ? Math.max(8, (scaledHeight / 100) * 40) : 0;
                  const isSelected = selectedBarIndex() === index;
                  const bucketStart = trends.bucketTimes[index];
                  const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;
                  const bucketRangeLabel = formatBucketRange(bucketStart, bucketEnd);
                  const bucketDurationText =
                    trends.bucketSize % 24 === 0
                      ? `${trends.bucketSize / 24} day${trends.bucketSize / 24 === 1 ? '' : 's'}`
                      : `${trends.bucketSize} hour${trends.bucketSize === 1 ? '' : 's'}`;
                  const countLabel = getAlertBucketCountLabel(value);
                  const tooltipContent = [
                    countLabel,
                    `${bucketDurationText} period`,
                    bucketRangeLabel,
                  ].join('\n');
                  return (
                    <div
                      class="relative flex flex-1 cursor-pointer items-end"
                      role="button"
                      tabIndex={0}
                      aria-pressed={isSelected}
                      aria-label={`${countLabel} between ${bucketRangeLabel}`}
                      onClick={() => setSelectedBarIndex(index === selectedBarIndex() ? null : index)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          setSelectedBarIndex(index === selectedBarIndex() ? null : index);
                        }
                      }}
                    >
                      <div class="absolute bottom-0 h-1 w-full rounded-full bg-slate-300 opacity-30"></div>
                      <div
                        class="relative w-full rounded-sm transition-all"
                        style={{
                          height: `${pixelHeight}px`,
                          'background-color':
                            value > 0 ? (isSelected ? '#2563eb' : '#3b82f6') : 'transparent',
                          opacity: isSelected ? '1' : '0.8',
                          'box-shadow': isSelected ? '0 0 0 2px rgba(37, 99, 235, 0.4)' : 'none',
                        }}
                        title={bucketRangeLabel}
                        onMouseEnter={(event) => {
                          if (value <= 0) {
                            hideTooltip();
                            return;
                          }
                          const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
                          showTooltip(tooltipContent, rect.left + rect.width / 2, rect.top, {
                            align: 'center',
                            direction: 'up',
                          });
                        }}
                        onMouseLeave={() => hideTooltip()}
                      />
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })()}

        <Show when={axisTicks().length > 0}>
          <div class="relative mt-3 h-10">
            <div class="absolute inset-x-0 top-0 h-px bg-surface-hover"></div>
            <For each={axisTicks()}>
              {(tick) => (
                <div
                  class="pointer-events-none absolute top-0 flex h-full flex-col items-center"
                  style={{ left: `${tick.position * 100}%` }}
                >
                  <div class="h-3 w-px bg-slate-300"></div>
                  <div
                    class="mt-1 whitespace-nowrap text-[10px] text-muted transform"
                    classList={{
                      '-translate-x-1/2': tick.align === 'center',
                      '-translate-x-full': tick.align === 'end',
                    }}
                  >
                    {tick.label}
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>
      </Card>

      <Card padding="sm" class="mb-4">
        <PageControls
          search={
            <SearchInput
              value={searchTerm}
              onChange={setSearchTerm}
              placeholder={getAlertHistorySearchPlaceholder()}
              class="w-full"
              clearOnEscape
              history={{ storageKey: STORAGE_KEYS.ALERTS_SEARCH_HISTORY }}
            />
          }
          mobileFilters={{
            enabled: isMobile(),
            onToggle: () => setFiltersOpen((open) => !open),
            count: activeFilterCount(),
          }}
          showFilters={!isMobile() || filtersOpen()}
        >
          <LabeledFilterSelect
            id="alert-time-filter"
            label="Period"
            value={timeFilter()}
            onChange={(event) =>
              setTimeFilter(event.currentTarget.value as '24h' | '7d' | '30d' | 'all')
            }
            selectClass="min-w-[7rem]"
          >
            <option value="24h">Last 24h</option>
            <option value="7d">Last 7d</option>
            <option value="30d">Last 30d</option>
            <option value="all">All Time</option>
          </LabeledFilterSelect>
          <LabeledFilterSelect
            id="alert-severity-filter"
            label="Severity"
            value={severityFilter()}
            onChange={(event) =>
              setSeverityFilter(event.currentTarget.value as 'warning' | 'critical' | 'all')
            }
            selectClass="min-w-[7rem]"
          >
            <option value="all">All</option>
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
          </LabeledFilterSelect>
        </PageControls>
      </Card>

      <Show when={resourceIncidentPanel()}>
        {(selection) => {
          const resourceId = selection().resourceId;
          const incidents = () => resourceIncidents()[resourceId] || [];
          const isLoading = () => resourceIncidentLoading()[resourceId];
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
                      <span>
                        {' '}
                        · {getAlertResourceIncidentCountLabel(incidents().length)}
                      </span>
                    </Show>
                  </p>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    class="px-2 py-1 text-xs border rounded-md border-border text-muted hover:bg-surface-hover disabled:opacity-50"
                    disabled={isLoading()}
                    onClick={() => {
                      void refreshResourceIncidentPanel();
                    }}
                  >
                    {getAlertResourceIncidentRefreshLabel(isLoading())}
                  </button>
                  <button
                    type="button"
                    class="px-2 py-1 text-xs border rounded-md border-border text-muted hover:bg-surface-hover"
                    onClick={() => setResourceIncidentPanel(null)}
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
                      filters={resourceIncidentEventFilters}
                      setFilters={setResourceIncidentEventFilters}
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
                        const isExpanded = expandedResourceIncidentIds().has(incident.id);
                        const events = incident.events || [];
                        const filteredEvents = filterIncidentEvents(
                          events,
                          resourceIncidentEventFilters(),
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
                                  onClick={() => toggleResourceIncidentDetails(incident.id)}
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

      <Show
        when={loading()}
        fallback={
          <Show
            when={alertData().length > 0}
            fallback={
              <div class="py-12 text-center text-muted">
                <p class="text-sm">{getAlertHistoryEmptyState().title}</p>
                <p class="mt-1 text-xs">{getAlertHistoryEmptyState().description}</p>
              </div>
            }
          >
            <div class="mb-2 overflow-hidden rounded border border-border">
              <div class="overflow-x-auto">
                <Table class="w-full min-w-[max-content] text-[11px] sm:text-sm">
                  <TableHeader>
                    <TableRow class="border-b border-border bg-surface-hover text-muted">
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Timestamp
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Source
                      </TableHead>
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Resource
                      </TableHead>
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        {getTypeColumnLabel()}
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Severity
                      </TableHead>
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Message
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Duration
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Status
                      </TableHead>
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Node
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Actions
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <For each={groupedAlerts()}>
                      {(group) => (
                        <>
                          <TableRow class="bg-surface-alt">
                            <TableCell
                              colspan={10}
                              class="py-1.5 pr-3 pl-4 text-[12px] font-semibold sm:text-sm"
                            >
                              <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
                                <span class="truncate" title={group.fullLabel}>
                                  {group.label}
                                </span>
                                <span class="text-[10px] font-medium text-muted">
                                  {(() => {
                                    const alertCount = group.alerts.filter(
                                      (alert) => alert.source === 'alert',
                                    ).length;
                                    const aiCount = group.alerts.filter(
                                      (alert) => alert.source === 'ai',
                                    ).length;
                                    const parts = [];
                                    if (alertCount > 0) {
                                      parts.push(
                                        `${alertCount} alert${alertCount === 1 ? '' : 's'}`,
                                      );
                                    }
                                    if (aiCount > 0) {
                                      parts.push(
                                        `${aiCount} patrol insight${aiCount === 1 ? '' : 's'}`,
                                      );
                                    }
                                    return (
                                      parts.join(', ') ||
                                      `${group.alerts.length} item${group.alerts.length === 1 ? '' : 's'}`
                                    );
                                  })()}
                                </span>
                              </div>
                            </TableCell>
                          </TableRow>

                          <For each={group.alerts}>
                            {(alert) => {
                              const rowKey = getIncidentRowKey(alert);
                              const historyStatusPresentation = getAlertHistoryStatusPresentation(
                                alert.status,
                              );
                              const sourcePresentation = getAlertHistorySourcePresentation(
                                alert.source,
                              );
                              return (
                                <>
                                  <TableRow
                                    class={`border-b border-border hover:bg-surface-hover ${historyStatusPresentation.rowClassName}`}
                                  >
                                    <TableCell class="p-1 px-1 font-mono whitespace-nowrap text-muted sm:p-1.5 sm:px-2">
                                      {new Date(alert.startTime).toLocaleTimeString('en-US', {
                                        hour: '2-digit',
                                        minute: '2-digit',
                                      })}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
                                      <span class={sourcePresentation.className}>
                                        {sourcePresentation.label}
                                      </span>
                                    </TableCell>

                                    <TableCell class="max-w-[150px] truncate p-1 px-1 font-medium text-base-content sm:p-1.5 sm:px-2">
                                      {alert.resourceName}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 sm:p-1.5 sm:px-2">
                                      <span class={getAlertHistoryResourceTypeBadgeClass(alert.resourceType)}>
                                        {alert.resourceType}
                                      </span>
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
                                      <span class={getAlertIncidentLevelBadgeClass(alert.severity)}>
                                        {alert.severity}
                                      </span>
                                    </TableCell>

                                    <TableCell
                                      class="max-w-[300px] truncate p-1 px-1 text-base-content sm:p-1.5 sm:px-2"
                                      title={alert.description}
                                    >
                                      {alert.description}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center text-muted sm:p-1.5 sm:px-2">
                                      {alert.duration}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
                                      <span class={historyStatusPresentation.className}>
                                        {historyStatusPresentation.label}
                                      </span>
                                    </TableCell>

                                    <TableCell class="truncate p-1 px-1 text-muted sm:p-1.5 sm:px-2">
                                      {alert.nodeDisplayName || alert.node || '—'}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
                                      <div class="flex items-center justify-center gap-1">
                                        <Show when={alert.source === 'alert'}>
                                          <button
                                            type="button"
                                            class="rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover"
                                            onClick={() => {
                                              void toggleIncidentTimeline(
                                                rowKey,
                                                alert.id,
                                                alert.startTime,
                                              );
                                            }}
                                          >
                                            {expandedIncidents().has(rowKey) ? 'Hide' : 'Timeline'}
                                          </button>
                                        </Show>
                                        <Show when={alert.source === 'alert' && alert.resourceId}>
                                          <button
                                            type="button"
                                            class="rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover"
                                            title={getAlertResourceIncidentViewTitle()}
                                            onClick={() => {
                                              void openResourceIncidentPanel(
                                                alert.resourceId as string,
                                                alert.resourceName,
                                              );
                                            }}
                                          >
                                            Resource
                                          </button>
                                        </Show>
                                        <Show
                                          when={
                                            alert.source === 'alert' &&
                                            (alert.status === 'active' ||
                                              alert.status === 'acknowledged')
                                          }
                                        >
                                          <InvestigateAlertButton
                                            alert={{
                                              id: alert.id,
                                              type: alert.rawAlertType || alert.title,
                                              level: alert.severity as 'warning' | 'critical',
                                              resourceId: alert.resourceId || '',
                                              resourceName: alert.resourceName,
                                              node: alert.node || '',
                                              nodeDisplayName: alert.nodeDisplayName,
                                              instance: '',
                                              message: alert.description || '',
                                              value: 0,
                                              threshold: 0,
                                              startTime: alert.startTime,
                                              lastSeen: alert.startTime,
                                              acknowledged: alert.status === 'acknowledged',
                                            }}
                                            resourceType={alert.resourceType}
                                            variant="icon"
                                            size="sm"
                                            licenseLocked={
                                              !props.hasAIAlertsFeature() && !props.licenseLoading()
                                            }
                                          />
                                        </Show>
                                      </div>
                                    </TableCell>
                                  </TableRow>
                                  <Show
                                    when={
                                      alert.source === 'alert' && expandedIncidents().has(rowKey)
                                    }
                                  >
                                    <TableRow class="border-b border-border bg-surface-alt">
                                      <TableCell colspan={11} class="p-3">
                                        <IncidentTimelinePanel
                                          loading={incidentLoading()[rowKey]}
                                          error={incidentErrors()[rowKey]}
                                          timeline={incidentTimelines()[rowKey]}
                                          filters={historyIncidentEventFilters}
                                          setFilters={setHistoryIncidentEventFilters}
                                          filterVariant="compact"
                                          eventCardVariant="surface"
                                          noteDraft={incidentNoteDrafts()[rowKey] || ''}
                                          onNoteDraftChange={(value) =>
                                            setIncidentNoteDraft(rowKey, value)
                                          }
                                          noteSaving={incidentNoteSaving().has(rowKey)}
                                          onSaveNote={() => {
                                            void saveIncidentNote(rowKey, alert.id, alert.startTime);
                                          }}
                                          onRetry={() => {
                                            void loadIncidentTimeline(
                                              rowKey,
                                              alert.id,
                                              alert.startTime,
                                            );
                                          }}
                                        />
                                      </TableCell>
                                    </TableRow>
                                  </Show>
                                </>
                              );
                            }}
                          </For>
                        </>
                      )}
                    </For>
                  </TableBody>
                </Table>
              </div>
            </div>
          </Show>
        }
      >
        <div class="py-12 text-center text-muted">
          <p class="text-sm">{getAlertHistoryLoadingState().text}</p>
        </div>
      </Show>

      <Show when={alertHistory().length > 0}>
        <div class="mt-8 border-t border-border pt-6">
          <div class="rounded-md bg-surface-alt p-4">
            <div class="flex items-start justify-between">
              <div>
                <h4 class="mb-1 text-sm font-medium text-base-content">
                  {getAlertAdministrationSectionTitle()}
                </h4>
                <p class="text-xs text-muted">{getAlertAdministrationSectionDescription()}</p>
              </div>
              <button
                type="button"
                onClick={() => {
                  void clearAlertHistory();
                }}
                class="flex-shrink-0 rounded-md border border-red-300 px-3 py-2 text-xs text-red-600 transition-colors hover:bg-red-50 dark:border-red-600 dark:text-red-400 dark:hover:bg-red-900"
              >
                {getAlertAdministrationClearHistoryLabel()}
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
}
