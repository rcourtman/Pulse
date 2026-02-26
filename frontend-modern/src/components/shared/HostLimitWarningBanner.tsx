import { Component, Show, createMemo } from 'solid-js';
import { entitlements, getLimit, getUpgradeActionUrlOrFallback } from '@/stores/license';

export const HostLimitWarningBanner: Component = () => {
  // No onMount load — TrialBanner (mounted above) already calls loadLicenseStatus().

  const nodeLimit = createMemo(() => getLimit('max_agents'));

  const isUrgent = createMemo(() => {
    const state = nodeLimit()?.state;
    return state === 'warning' || state === 'enforced';
  });

  const overflowDaysRemaining = createMemo(() => entitlements()?.overflow_days_remaining);

  return (
    <Show when={nodeLimit()}>
      <div
        class={`mb-2 rounded-md border px-3 py-2 text-sm ${
          isUrgent()
            ? 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-900 dark:text-amber-100'
            : 'border-border bg-surface text-muted'
        }`}
        role="status"
        aria-live="polite"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class={isUrgent() ? 'font-medium' : ''}>
            Agents: {nodeLimit()!.current}/{nodeLimit()!.limit}
          </span>
          <div class="flex items-center gap-3">
            <Show when={overflowDaysRemaining()}>
              <span class="text-xs text-muted">
                Includes 1 bonus agent ({overflowDaysRemaining()}d remaining)
              </span>
            </Show>
            <a
              class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
              href="/settings/system-pro"
            >
              See what's counted
            </a>
            <Show when={isUrgent()}>
              <a
                class="text-xs font-semibold underline underline-offset-2 hover:opacity-90"
                href={getUpgradeActionUrlOrFallback('max_agents')}
                target="_blank"
                rel="noreferrer"
              >
                Upgrade to add more
              </a>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default HostLimitWarningBanner;
