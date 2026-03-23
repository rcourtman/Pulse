import { Component, Show } from 'solid-js';
import {
  getTrialBannerStatusLabel,
  TRIAL_BANNER_SNOOZE_LABEL,
  TRIAL_BANNER_TITLE,
  TRIAL_BANNER_UPGRADE_LABEL,
} from './trialBannerModel';
import { useTrialBannerState } from './useTrialBannerState';

export const TrialBanner: Component = () => {
  const state = useTrialBannerState();

  return (
    <Show when={state.isTrial()}>
      <div
        class={`mb-2 rounded-md border px-3 py-2 text-sm ${state.toneClass()}`}
        role="status"
        aria-live="polite"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="font-medium">
            {TRIAL_BANNER_TITLE}
            <span class="ml-2">{getTrialBannerStatusLabel(state.daysRemaining())}</span>
          </div>
          <Show when={state.showActions()}>
            <div class="flex items-center gap-2">
              <a
                class="text-xs font-semibold underline underline-offset-2 hover:opacity-90"
                href={state.upgradeHref()}
                target="_blank"
                rel="noreferrer"
              >
                {TRIAL_BANNER_UPGRADE_LABEL}
              </a>
              <button
                type="button"
                class="text-xs opacity-70 hover:opacity-100"
                onClick={state.handleSnooze}
              >
                {TRIAL_BANNER_SNOOZE_LABEL}
              </button>
            </div>
          </Show>
        </div>
      </div>
    </Show>
  );
};

export default TrialBanner;
