import { For, createSignal } from 'solid-js';

import { Button } from '@/components/shared/Button';
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
  const [submittingPreset, setSubmittingPreset] = createSignal(false);
  const id = () => getCanonicalAlertId(props.alert);
  const busy = () =>
    submittingPreset() ||
    props.state.snoozeProcessingAlerts().has(id()) ||
    props.state.processingAlerts().has(id());
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
      <Button
        variant="outline"
        size="sm"
        class="text-muted hover:text-base-content"
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
      </Button>
      <Dialog
        isOpen={open()}
        onClose={() => setOpen(false)}
        panelClass="max-w-md"
        ariaLabel={getAlertOverviewSnoozeTitle()}
      >
        <div class="p-5">
          <h2 class="text-lg font-semibold text-base-content">{getAlertOverviewSnoozeTitle()}</h2>
          <p class="mt-1 text-sm text-muted">{getAlertOverviewSnoozeDescription()}</p>
          <div class="mt-4 grid gap-2" aria-busy={submittingPreset()}>
            <For each={presets}>
              {(preset) => (
                <Button
                  variant="outline"
                  size="md"
                  class="w-full justify-start rounded-lg text-left hover:border-primary hover:bg-primary/5"
                  disabled={busy()}
                  onClick={async () => {
                    if (busy()) return;
                    setSubmittingPreset(true);
                    try {
                      await props.state.handleSnooze(props.alert, snoozePresetExpiry(preset));
                      await refreshOpenTimeline();
                      setOpen(false);
                    } catch {
                      // State owns rollback and user-visible failure reporting.
                    } finally {
                      setSubmittingPreset(false);
                    }
                  }}
                >
                  {getAlertOverviewSnoozeOptionLabel(preset)}
                </Button>
              )}
            </For>
          </div>
          <div class="mt-4 flex justify-end">
            <Button variant="ghost" size="sm" onClick={() => setOpen(false)}>
              {getAlertOverviewCancelLabel()}
            </Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
