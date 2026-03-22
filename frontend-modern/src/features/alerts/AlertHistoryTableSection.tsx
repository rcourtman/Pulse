import { For, Show } from 'solid-js';

import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import {
  getAlertHistoryEmptyState,
  getAlertHistoryLoadingState,
} from '@/utils/alertOverviewPresentation';
import {
  getAlertHistoryResourceTypeBadgeClass,
  getAlertHistorySourcePresentation,
} from '@/utils/alertHistoryPresentation';
import {
  getAlertHistoryStatusPresentation,
  getAlertIncidentLevelBadgeClass,
  getAlertResourceIncidentViewTitle,
} from '@/utils/alertIncidentPresentation';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';

import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryTableSectionProps {
  state: AlertHistoryState;
  hasAIAlertsFeature: () => boolean;
  licenseLoading: () => boolean;
}

export function AlertHistoryTableSection(props: AlertHistoryTableSectionProps) {
  return (
    <Show
      when={props.state.loading()}
      fallback={
        <Show
          when={props.state.alertData().length > 0}
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
                  <For each={props.state.groupedAlerts()}>
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
                                    parts.push(`${alertCount} alert${alertCount === 1 ? '' : 's'}`);
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
                            const rowKey = props.state.getIncidentRowKey(alert);
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
                                            void props.state.toggleIncidentTimeline(
                                              rowKey,
                                              alert.id,
                                              alert.startTime,
                                            );
                                          }}
                                        >
                                          {props.state.expandedIncidents().has(rowKey)
                                            ? 'Hide'
                                            : 'Timeline'}
                                        </button>
                                      </Show>
                                      <Show when={alert.source === 'alert' && alert.resourceId}>
                                        <button
                                          type="button"
                                          class="rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover"
                                          title={getAlertResourceIncidentViewTitle()}
                                          onClick={() => {
                                            void props.state.openResourceIncidentPanel(
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
                                <Show when={alert.source === 'alert' && props.state.expandedIncidents().has(rowKey)}>
                                  <TableRow class="border-b border-border bg-surface-alt">
                                    <TableCell colspan={11} class="p-3">
                                      <IncidentTimelinePanel
                                        loading={props.state.incidentLoading()[rowKey]}
                                        error={props.state.incidentErrors()[rowKey]}
                                        timeline={props.state.incidentTimelines()[rowKey]}
                                        filters={props.state.historyIncidentEventFilters}
                                        setFilters={props.state.setHistoryIncidentEventFilters}
                                        filterVariant="compact"
                                        eventCardVariant="surface"
                                        noteDraft={props.state.incidentNoteDrafts()[rowKey] || ''}
                                        onNoteDraftChange={(value) =>
                                          props.state.setIncidentNoteDraft(rowKey, value)
                                        }
                                        noteSaving={props.state.incidentNoteSaving().has(rowKey)}
                                        onSaveNote={() => {
                                          void props.state.saveIncidentNote(
                                            rowKey,
                                            alert.id,
                                            alert.startTime,
                                          );
                                        }}
                                        onRetry={() => {
                                          void props.state.loadIncidentTimeline(
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
  );
}
