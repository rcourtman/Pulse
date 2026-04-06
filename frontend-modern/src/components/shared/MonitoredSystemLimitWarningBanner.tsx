import { Component, Show } from 'solid-js';
import {
  MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_LABEL,
  MONITORED_SYSTEM_LIMIT_LEARN_MORE_LABEL,
  MONITORED_SYSTEM_LIMIT_UPGRADE_LABEL,
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
          <Show when={state.overflowSummary()}>
            <span class="text-xs opacity-80">{state.overflowSummary()}</span>
          </Show>
          <Show when={state.migrationGap()}>
            <span class={`text-xs ${state.migrationTextClass()}`}>{state.migrationMessage()}</span>
          </Show>
          <UpgradeLink
            class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
            destination={state.learnMoreDestination()}
          >
            {MONITORED_SYSTEM_LIMIT_LEARN_MORE_LABEL}
          </UpgradeLink>
          <Show when={state.migrationGap()}>
            <UpgradeLink
              class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
              destination={state.installCollectorsDestination()}
              onClick={state.handleInstallCollectorsClick}
            >
              {MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_LABEL}
            </UpgradeLink>
          </Show>
          <Show when={state.isUrgent()}>
            <UpgradeLink
              class="text-xs font-semibold underline underline-offset-2 hover:opacity-90"
              destination={state.upgradeDestination()}
              onClick={state.handleUpgradeClick}
            >
              {MONITORED_SYSTEM_LIMIT_UPGRADE_LABEL}
            </UpgradeLink>
          </Show>
        </div>
      </div>
    </Show>
  );
};
