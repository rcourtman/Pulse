import { Component, Show, createMemo } from 'solid-js';
import { getLimit, getUpgradeActionUrlOrFallback } from '@/stores/license';

export const HostLimitWarningBanner: Component = () => {
  // No onMount load — TrialBanner (mounted above) already calls loadLicenseStatus().

  const nodeLimit = createMemo(() => getLimit('max_nodes'));

  const isWarning = createMemo(() => nodeLimit()?.state === 'warning');

  return (
    <Show when={isWarning()}>
      <div
        class="mb-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-900 dark:text-amber-100"
        role="status"
        aria-live="polite"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="font-medium">
            You're using {nodeLimit()!.current}/{nodeLimit()!.limit} hosts.
          </span>
          <a
            class="text-xs font-semibold underline underline-offset-2 hover:opacity-90"
            href={getUpgradeActionUrlOrFallback('max_nodes')}
            target="_blank"
            rel="noreferrer"
          >
            Upgrade to add more
          </a>
        </div>
      </div>
    </Show>
  );
};

export default HostLimitWarningBanner;
