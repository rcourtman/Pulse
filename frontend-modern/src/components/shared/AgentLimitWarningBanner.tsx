import { Component, Show, createEffect, createMemo, createSignal } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import {
  entitlements,
  getLimit,
  getUpgradeActionUrlOrFallback,
  hasMigrationGap,
  legacyConnections,
} from '@/stores/license';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  trackUpgradeClicked,
  trackUpgradeMetricEvent,
  UPGRADE_METRIC_EVENTS,
} from '@/utils/upgradeMetrics';

export const AgentLimitWarningBanner: Component = () => {
  // No onMount load — TrialBanner (mounted above) already calls loadLicenseStatus().

  const nodeLimit = createMemo(() => getLimit('max_agents'));

  // Org-scoped dismissal so multi-tenant users don't accidentally hide the
  // notice for a different org.
  const dismissalKey = createMemo(() => {
    const orgId = sessionStorage.getItem(STORAGE_KEYS.ORG_ID) ?? 'default';
    return `${STORAGE_KEYS.AGENT_MIGRATION_NOTICE_DISMISSED}:${orgId}`;
  });
  const [dismissed, setDismissed] = createSignal(false);
  // Hydrate from localStorage on key change.
  createEffect(() => {
    setDismissed(localStorage.getItem(dismissalKey()) === 'true');
  });
  const migrationDismissed = () => dismissed();
  const setMigrationDismissed = (v: boolean) => {
    localStorage.setItem(dismissalKey(), String(v));
    setDismissed(v);
  };

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
  const migrationOnly = createMemo(() => migrationGap() && !isUrgent());
  const showBanner = createMemo(
    () => Boolean(nodeLimit()) && (isUrgent() || (migrationGap() && !migrationDismissed())),
  );
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

  const overflowDaysRemaining = createMemo(() => entitlements()?.overflow_days_remaining);

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
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div
              class={`flex flex-wrap items-center gap-x-3 gap-y-1 ${isUrgent() ? 'font-medium' : ''}`}
            >
              <span>
                Host Agents: {nodeLimit()!.current}/{nodeLimit()!.limit}
              </span>
              <Show when={overflowDaysRemaining()}>
                <span class="text-xs opacity-80">
                  Includes 1 bonus agent ({overflowDaysRemaining()}d remaining)
                </span>
              </Show>
            </div>
            <Show when={migrationGap()}>
              <p
                class={`mt-1 text-xs ${isUrgent() ? 'text-amber-800 dark:text-amber-200' : 'text-sky-800 dark:text-sky-200'}`}
              >
                {migrationMessage()}
              </p>
            </Show>
            <div class="mt-2 flex flex-wrap items-center gap-3">
              <a
                class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
                href="/settings/system-pro"
              >
                Learn more
              </a>
              <Show when={migrationGap()}>
                <a
                  class="text-xs font-medium underline underline-offset-2 hover:opacity-90"
                  href="/settings/workloads"
                  onClick={() =>
                    trackUpgradeClicked('agent_limit_banner_install_v6_agents', 'max_agents')
                  }
                >
                  Install v6 agents
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
          <Show when={migrationOnly()}>
            <button
              type="button"
              class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-sky-700 hover:bg-sky-100 hover:text-sky-900 dark:text-sky-300 dark:hover:bg-sky-900 dark:hover:text-sky-100"
              title="Dismiss"
              aria-label="Dismiss agent migration notice"
              onClick={() => setMigrationDismissed(true)}
            >
              <XIcon class="h-3.5 w-3.5" />
            </button>
          </Show>
        </div>
      </div>
    </Show>
  );
};
