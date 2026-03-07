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

export const AgentLimitWarningBanner: Component = () => {
  // No onMount load — TrialBanner (mounted above) already calls loadLicenseStatus().

  const nodeLimit = createMemo(() => getLimit('max_agents'));

  const isUrgent = createMemo(() => {
    const state = nodeLimit()?.state;
    return state === 'warning' || state === 'enforced';
  });

  const migrationGap = createMemo(() => hasMigrationGap());
  const migrationCounts = createMemo(() => legacyConnections());
  const legacyConnectionTotal = createMemo(() => {
    const counts = migrationCounts();
    return counts.proxmox_nodes + counts.docker_hosts + counts.kubernetes_clusters;
  });
  const showBanner = createMemo(() => Boolean(nodeLimit()) && isUrgent());
  const hostAgentSummary = createMemo(() => {
    const limit = nodeLimit();
    if (!limit) return '';
    return `v6 Host Agents: ${limit.current}/${limit.limit}`;
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
    return `You also have ${total} ${noun} connected via API or legacy agents${breakdown ? ` (${breakdown})` : ''} that do not count toward Host Agents.`;
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
    const limit = nodeLimit();
    if (urgent && !prevUrgent && limit) {
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.LIMIT_WARNING_SHOWN,
        surface: 'agent_limit_banner',
        limit_key: 'max_agents',
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
          <span class={isUrgent() ? 'font-medium' : ''}>{hostAgentSummary()}</span>
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
            href="/settings/system-pro"
          >
            Learn more
          </a>
          <Show when={migrationGap()}>
            <a
              class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
              href="/settings"
              onClick={() =>
                trackUpgradeClicked('agent_limit_banner_install_v6_agents', 'max_agents')
              }
            >
              Install v6 host agents
            </a>
          </Show>
          <Show when={isUrgent()}>
            <a
              class="text-xs font-semibold underline underline-offset-2 hover:opacity-90"
              href={getUpgradeActionUrlOrFallback('max_agents')}
              target="_blank"
              rel="noreferrer"
              onClick={() => trackUpgradeClicked('agent_limit_banner_upgrade', 'max_agents')}
            >
              Upgrade to add more
            </a>
          </Show>
        </div>
      </div>
    </Show>
  );
};
