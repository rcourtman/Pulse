import { Component, Show, createEffect, createMemo } from 'solid-js';
import {
  entitlements,
  getLimit,
  getUpgradeActionUrlOrFallback,
  hasMigrationGap,
  legacyConnections,
} from '@/stores/license';
import {
  trackUpgradeClicked,
  trackUpgradeMetricEvent,
  UPGRADE_METRIC_EVENTS,
} from '@/utils/upgradeMetrics';

export const MonitoredSystemLimitWarningBanner: Component = () => {
  // No onMount load — TrialBanner (mounted above) already calls loadLicenseStatus().

  const monitoredSystemLimit = createMemo(() => getLimit('max_monitored_systems'));

  const isUrgent = createMemo(() => {
    const state = monitoredSystemLimit()?.state;
    return state === 'warning' || state === 'enforced';
  });

  const migrationGap = createMemo(() => hasMigrationGap());
  const migrationCounts = createMemo(() => legacyConnections());
  const legacyConnectionTotal = createMemo(() => {
    const counts = migrationCounts();
    return counts.proxmox_nodes + counts.docker_hosts + counts.kubernetes_clusters;
  });
  const showBanner = createMemo(() => Boolean(monitoredSystemLimit()) && isUrgent());
  const monitoredSystemSummary = createMemo(() => {
    const limit = monitoredSystemLimit();
    if (!limit) return '';
    return `Monitored systems: ${limit.current}/${limit.limit}`;
  });
  const legacyBreakdown = createMemo(() => {
    const counts = migrationCounts();
    const parts: string[] = [];
    if (counts.proxmox_nodes > 0) {
      parts.push(
        `${counts.proxmox_nodes} Proxmox ${counts.proxmox_nodes === 1 ? 'node' : 'nodes'}`,
      );
    }
    if (counts.docker_hosts > 0) {
      parts.push(`${counts.docker_hosts} Docker ${counts.docker_hosts === 1 ? 'host' : 'hosts'}`);
    }
    if (counts.kubernetes_clusters > 0) {
      parts.push(
        `${counts.kubernetes_clusters} Kubernetes ${counts.kubernetes_clusters === 1 ? 'cluster' : 'clusters'}`,
      );
    }
    return parts.join(', ');
  });
  const migrationMessage = createMemo(() => {
    const total = legacyConnectionTotal();
    if (total <= 0) return '';
    const noun = total === 1 ? 'resource' : 'resources';
    const breakdown = legacyBreakdown();
    return `You also have ${total} ${noun} connected via API or legacy collectors${breakdown ? ` (${breakdown})` : ''} that count once toward your monitored-system cap when the same top-level system is discovered canonically.`;
  });
  const overflowDaysRemaining = createMemo(() => entitlements()?.overflow_days_remaining);
  const overflowSummary = createMemo(() => {
    const days = overflowDaysRemaining();
    if (!days) return '';
    return `Includes 1 temporary onboarding slot (${days}d remaining)`;
  });

  // Emit limit_warning_shown once when the banner transitions to warning/enforced.
  // Uses previous-state tracking to avoid re-emitting on every reactive update.
  let prevUrgent = false;
  createEffect(() => {
    const urgent = isUrgent();
    const limit = monitoredSystemLimit();
    if (urgent && !prevUrgent && limit) {
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.LIMIT_WARNING_SHOWN,
        surface: 'monitored_system_limit_banner',
        limit_key: 'max_monitored_systems',
        current_value: limit.current,
        limit_value: limit.limit,
      });
    }
    prevUrgent = !!urgent;
  });

  return (
    <Show when={showBanner()}>
      <div
        class={`mb-2 rounded-md border px-3 py-2 text-sm ${
          isUrgent()
            ? 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-900 dark:text-amber-100'
            : 'border-sky-200 bg-sky-50 text-sky-950 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-100'
        }`}
        role="status"
        aria-live="polite"
      >
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span class={isUrgent() ? 'font-medium' : ''}>{monitoredSystemSummary()}</span>
          <Show when={overflowSummary()}>
            <span class="text-xs opacity-80">{overflowSummary()}</span>
          </Show>
          <Show when={migrationGap()}>
            <span
              class={`text-xs ${isUrgent() ? 'text-amber-800 dark:text-amber-200' : 'text-sky-800 dark:text-sky-200'}`}
            >
              {migrationMessage()}
            </span>
          </Show>
          <a
            class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
            href="/settings/system/billing"
          >
            Learn more
          </a>
          <Show when={migrationGap()}>
            <a
              class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
              href="/settings"
              onClick={() =>
                trackUpgradeClicked(
                  'monitored_system_limit_banner_install_v6_collectors',
                  'max_monitored_systems',
                )
              }
            >
              Install v6 collectors
            </a>
          </Show>
          <Show when={isUrgent()}>
            <a
              class="text-xs font-semibold underline underline-offset-2 hover:opacity-90"
              href={getUpgradeActionUrlOrFallback('max_monitored_systems')}
              target="_blank"
              rel="noreferrer"
              onClick={() =>
                trackUpgradeClicked('monitored_system_limit_banner_upgrade', 'max_monitored_systems')
              }
            >
              Upgrade to add more
            </a>
          </Show>
        </div>
      </div>
    </Show>
  );
};
