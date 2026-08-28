import { Show, createMemo, createSignal } from 'solid-js';

import { getAlertHistoryResourceTypeBadgeClass } from '@/utils/alertHistoryPresentation';
import {
  getAlertHistoryStatusPresentation,
  getAlertIncidentLevelBadgeClass,
} from '@/utils/alertIncidentPresentation';

import { AlertHistoryItemActions } from './AlertHistoryItemActions';
import { PlatformWindowedList } from '@/features/platformPage/PlatformWindowedList';
import { getGroupSummaryLabel } from './AlertHistoryTableGroupRow';
import {
  MobileAlertHistoryInvestigationDialog,
  type MobileAlertHistoryInvestigation,
} from './MobileAlertHistoryInvestigationDialog';
import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryMobileListProps {
  state: AlertHistoryState;
}

type AlertHistoryGroup = ReturnType<AlertHistoryState['groupedAlerts']>[number];
type AlertHistoryAlert = AlertHistoryGroup['alerts'][number];
type AlertHistoryMobileItem =
  | { kind: 'group'; key: string; group: AlertHistoryGroup }
  | { kind: 'alert'; key: string; alert: AlertHistoryAlert };

export function AlertHistoryMobileList(props: AlertHistoryMobileListProps) {
  const [investigation, setInvestigation] = createSignal<MobileAlertHistoryInvestigation | null>(
    null,
  );

  const mobileItems = createMemo<AlertHistoryMobileItem[]>(() => {
    const items: AlertHistoryMobileItem[] = [];
    for (const [groupIndex, group] of props.state.groupedAlerts().entries()) {
      items.push({ kind: 'group', key: `group:${groupIndex}:${group.fullLabel}`, group });
      for (const alert of group.alerts) {
        items.push({ kind: 'alert', key: `alert:${alert.id}:${alert.startTime}`, alert });
      }
    }
    return items;
  });

  const getInvestigationReturnFocusTarget = (
    current: MobileAlertHistoryInvestigation,
  ): HTMLElement | null => {
    const card = Array.from(
      document.querySelectorAll<HTMLElement>('[data-alert-history-row-key]'),
    ).find((candidate) => candidate.dataset.alertHistoryRowKey === current.rowKey);
    const action = card?.querySelector<HTMLElement>(
      `[data-alert-history-action="${current.kind}"]`,
    );
    const list = document.querySelector<HTMLElement>('[data-testid="alert-history-mobile-list"]');
    return action ?? list;
  };

  const closeInvestigation = () => {
    const current = investigation();
    if (!current) return;
    const currentFocusTarget = getInvestigationReturnFocusTarget(current);
    setInvestigation(null);
    currentFocusTarget?.focus({ preventScroll: true });

    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        getInvestigationReturnFocusTarget(current)?.focus({ preventScroll: true });
      });
    });

    if (current.kind === 'timeline') {
      if (props.state.expandedIncidents().has(current.rowKey)) {
        void props.state.toggleIncidentTimeline(
          current.rowKey,
          current.alert.id,
          current.alert.startTime,
        );
      }
      return;
    }

    if (props.state.resourceIncidentPanel()?.rowKey === current.rowKey) {
      props.state.setResourceIncidentPanel(null);
    }
  };

  const closePreviousInvestigationState = () => {
    const current = investigation();
    if (!current) return;

    if (current.kind === 'timeline' && props.state.expandedIncidents().has(current.rowKey)) {
      void props.state.toggleIncidentTimeline(
        current.rowKey,
        current.alert.id,
        current.alert.startTime,
      );
    }
    if (
      current.kind === 'resource' &&
      props.state.resourceIncidentPanel()?.rowKey === current.rowKey
    ) {
      props.state.setResourceIncidentPanel(null);
    }
  };

  const openTimelineInvestigation = (alert: AlertHistoryAlert, rowKey: string) => {
    const current = investigation();
    if (current?.kind === 'timeline' && current.rowKey === rowKey) {
      closeInvestigation();
      return;
    }

    closePreviousInvestigationState();
    setInvestigation({ kind: 'timeline', alert, rowKey });
    if (!props.state.expandedIncidents().has(rowKey)) {
      void props.state.toggleIncidentTimeline(rowKey, alert.id, alert.startTime);
    }
  };

  const openResourceInvestigation = (alert: AlertHistoryAlert, rowKey: string) => {
    const current = investigation();
    if (current?.kind === 'resource' && current.rowKey === rowKey) {
      closeInvestigation();
      return;
    }

    closePreviousInvestigationState();
    setInvestigation({ kind: 'resource', alert, rowKey });
    void props.state.openResourceIncidentPanel(
      alert.resourceId as string,
      alert.resourceName,
      rowKey,
    );
  };

  return (
    <>
      <div class="space-y-3 md:hidden" data-testid="alert-history-mobile-list" tabindex="-1">
        <PlatformWindowedList
          items={mobileItems}
          estimatedItemHeight={260}
          enableThreshold={18}
          windowSize={24}
        >
          {(item) => {
            if (item.kind === 'group') {
              return (
                <header class="flex items-start justify-between gap-3 rounded-lg border border-border bg-surface-alt px-3 py-2">
                  <h3
                    class="min-w-0 truncate text-xs font-semibold text-base-content"
                    title={item.group.fullLabel}
                  >
                    {item.group.label}
                  </h3>
                  <span class="shrink-0 text-[10px] text-muted">
                    {getGroupSummaryLabel(item.group)}
                  </span>
                </header>
              );
            }

            const alert = item.alert;
            const rowKey = () => props.state.getIncidentRowKey(alert);
            const status = () => getAlertHistoryStatusPresentation(alert.status);

            return (
              <article
                class={`overflow-hidden rounded-lg border border-border bg-surface p-3 ${status().rowClassName}`}
                data-alert-history-row-key={rowKey()}
              >
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

                <p class="mt-2 text-xs leading-relaxed text-base-content">{alert.description}</p>

                <dl class="mt-3 grid grid-cols-3 gap-2 text-[10px] text-muted">
                  <div>
                    <dt class="uppercase tracking-wide">Time</dt>
                    <dd
                      class="mt-0.5 font-mono text-base-content"
                      title={props.state.formatAlertRowTimestamp(alert.startTime)}
                    >
                      {props.state.formatAlertRowTime(alert.startTime)}
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

                <AlertHistoryItemActions
                  alert={alert}
                  state={props.state}
                  class="mt-3"
                  touchSized
                  onTimelineClick={() => openTimelineInvestigation(alert, rowKey())}
                  onResourceClick={() => openResourceInvestigation(alert, rowKey())}
                />
              </article>
            );
          }}
        </PlatformWindowedList>
      </div>

      <Show when={investigation()}>
        {(selectedInvestigation) => (
          <MobileAlertHistoryInvestigationDialog
            investigation={selectedInvestigation()}
            state={props.state}
            onClose={closeInvestigation}
          />
        )}
      </Show>
    </>
  );
}
