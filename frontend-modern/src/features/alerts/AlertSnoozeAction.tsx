import { For, createSignal } from 'solid-js';

import { Dialog } from '@/components/shared/Dialog';
import type { Alert } from '@/types/api';
import {
  getAlertOverviewCancelLabel,
  getAlertOverviewResumeLabel,
  getAlertOverviewSnoozeDescription,
  getAlertOverviewSnoozeLabel,
  getAlertOverviewSnoozeOptionLabel,
  getAlertOverviewSnoozeTitle,
} from '@/utils/alertOverviewPresentation';

import { getCanonicalAlertId } from './identity';
import { isAlertSnoozed } from './useAlertSnoozeState';
import type { AlertOverviewState } from './useAlertOverviewState';
import type { AlertIncidentTimelineState } from './useAlertIncidentTimelineState';

type SnoozePreset = 'oneHour' | 'twoHours' | 'eightHours' | 'tomorrowMorning' | 'sevenDays';

export function snoozePresetExpiry(preset: SnoozePreset, now = new Date()): Date {
  if (preset === 'tomorrowMorning') {
    const result = new Date(now);
    result.setDate(result.getDate() + 1);
    result.setHours(9, 0, 0, 0);
    return result;
  }
  const hours = { oneHour: 1, twoHours: 2, eightHours: 8, sevenDays: 24 * 7 }[preset];
  return new Date(now.getTime() + hours * 60 * 60 * 1000);
}

export function AlertSnoozeAction(props: {
  alert: Alert;
  state: AlertOverviewState;
  timelineState: AlertIncidentTimelineState;
}) {
  const [open, setOpen] = createSignal(false);
  const id = () => getCanonicalAlertId(props.alert);
  const busy = () =>
    props.state.snoozeProcessingAlerts().has(id()) || props.state.processingAlerts().has(id());
  const presets: SnoozePreset[] = [
    'oneHour',
    'twoHours',
    'eightHours',
    'tomorrowMorning',
    'sevenDays',
  ];
  const refreshOpenTimeline = async () => {
    if (props.timelineState.expandedIncidents().has(id())) {
      await props.timelineState.loadIncidentTimeline(id(), id(), props.alert.startTime);
    }
  };

  return (
    <>
      <button
        class="px-2.5 py-1.5 text-xs font-medium rounded border border-border text-muted hover:text-base-content hover:bg-surface-alt transition-colors disabled:opacity-50"
        disabled={busy()}
        onClick={() => {
          if (isAlertSnoozed(props.alert)) {
            void props.state
              .handleUnsnooze(props.alert)
              .then(refreshOpenTimeline)
              .catch(() => undefined);
          } else {
            setOpen(true);
          }
        }}
      >
        {isAlertSnoozed(props.alert)
          ? getAlertOverviewResumeLabel()
          : getAlertOverviewSnoozeLabel()}
      </button>
      <Dialog
        isOpen={open()}
        onClose={() => setOpen(false)}
        panelClass="max-w-md"
        ariaLabel={getAlertOverviewSnoozeTitle()}
      >
        <div class="p-5">
          <h2 class="text-lg font-semibold text-base-content">{getAlertOverviewSnoozeTitle()}</h2>
          <p class="mt-1 text-sm text-muted">{getAlertOverviewSnoozeDescription()}</p>
          <div class="mt-4 grid gap-2">
            <For each={presets}>
              {(preset) => (
                <button
                  class="w-full rounded-lg border border-border px-3 py-2 text-left text-sm text-base-content hover:border-primary hover:bg-primary/5 transition-colors"
                  onClick={async () => {
                    try {
                      await props.state.handleSnooze(props.alert, snoozePresetExpiry(preset));
                      await refreshOpenTimeline();
                      setOpen(false);
                    } catch {
                      // State owns rollback and user-visible failure reporting.
                    }
                  }}
                >
                  {getAlertOverviewSnoozeOptionLabel(preset)}
                </button>
              )}
            </For>
          </div>
          <div class="mt-4 flex justify-end">
            <button class="btn btn-ghost btn-sm" onClick={() => setOpen(false)}>
              {getAlertOverviewCancelLabel()}
            </button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
