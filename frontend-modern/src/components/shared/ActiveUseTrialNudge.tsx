import { Component, Show } from 'solid-js';
import {
  ACTIVE_USE_TRIAL_NUDGE_SNOOZE_LABEL,
  ACTIVE_USE_TRIAL_NUDGE_STARTING_LABEL,
  ACTIVE_USE_TRIAL_NUDGE_START_LABEL,
  ACTIVE_USE_TRIAL_NUDGE_TITLE,
} from './activeUseTrialNudgeModel';
import { useActiveUseTrialNudgeState } from './useActiveUseTrialNudgeState';

export const ActiveUseTrialNudge: Component = () => {
  const state = useActiveUseTrialNudgeState();

  return (
    <Show when={state.shouldShow()}>
      <div
        class="mb-2 rounded-md border border-indigo-200 bg-indigo-50 dark:border-indigo-900 dark:bg-indigo-900/30 px-3 py-2 text-sm text-indigo-900 dark:text-indigo-100"
        role="status"
        aria-live="polite"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="font-medium">{ACTIVE_USE_TRIAL_NUDGE_TITLE}</span>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="text-xs font-semibold text-indigo-700 dark:text-indigo-300 hover:underline disabled:opacity-60"
              disabled={state.startingTrial()}
              onClick={state.handleStartTrial}
            >
              {state.startingTrial()
                ? ACTIVE_USE_TRIAL_NUDGE_STARTING_LABEL
                : ACTIVE_USE_TRIAL_NUDGE_START_LABEL}
            </button>
            <button
              type="button"
              class="text-xs opacity-70 hover:opacity-100"
              onClick={state.handleSnooze}
            >
              {ACTIVE_USE_TRIAL_NUDGE_SNOOZE_LABEL}
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default ActiveUseTrialNudge;
