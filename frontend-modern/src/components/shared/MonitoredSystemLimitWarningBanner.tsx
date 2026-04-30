import { Component, Show } from 'solid-js';
import {
  MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_LABEL,
  MONITORED_SYSTEM_LIMIT_REVIEW_POLICY_LABEL,
} from './monitoredSystemLimitWarningBannerModel';
import { UpgradeLink } from './UpgradeLink';
import { useMonitoredSystemLimitWarningBannerState } from './useMonitoredSystemLimitWarningBannerState';

export const MonitoredSystemLimitWarningBanner: Component = () => {
  const state = useMonitoredSystemLimitWarningBannerState();

  return (
    <Show when={state.showBanner()}>
      <div
        class={`mb-2 rounded-md border px-3 py-2 text-sm ${state.toneClass()}`}
        role="status"
        aria-live="polite"
      >
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span class={state.isUrgent() ? 'font-medium' : ''}>{state.monitoredSystemSummary()}</span>
          <UpgradeLink
            class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
            destination={state.reviewPolicyDestination()}
          >
            {MONITORED_SYSTEM_LIMIT_REVIEW_POLICY_LABEL}
          </UpgradeLink>
          <Show when={state.migrationGap()}>
            <UpgradeLink
              class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
              destination={state.installCollectorsDestination()}
            >
              {MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_LABEL}
            </UpgradeLink>
          </Show>
        </div>
      </div>
    </Show>
  );
};
