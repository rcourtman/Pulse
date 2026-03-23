type LimitState = {
  current: number;
  limit: number;
  state?: string;
};

type LegacyConnectionCounts = {
  proxmox_nodes: number;
  docker_hosts: number;
  kubernetes_clusters: number;
};

export const MONITORED_SYSTEM_LIMIT_KEY = 'max_monitored_systems';
export const MONITORED_SYSTEM_LIMIT_BILLING_HREF = '/settings/system/billing';
export const MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_HREF = '/settings';
export const MONITORED_SYSTEM_LIMIT_LEARN_MORE_LABEL = 'Learn more';
export const MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_LABEL = 'Install v6 collectors';
export const MONITORED_SYSTEM_LIMIT_UPGRADE_LABEL = 'Upgrade to add more';

export function isMonitoredSystemLimitUrgent(limit: LimitState | undefined): boolean {
  const state = limit?.state;
  return state === 'warning' || state === 'enforced';
}

export function shouldShowMonitoredSystemLimitBanner(limit: LimitState | undefined): boolean {
  return Boolean(limit) && isMonitoredSystemLimitUrgent(limit);
}

export function getMonitoredSystemSummary(limit: LimitState | undefined): string {
  if (!limit) return '';
  return `Monitored systems: ${limit.current}/${limit.limit}`;
}

export function getMonitoredSystemLegacyConnectionTotal(counts: LegacyConnectionCounts): number {
  return counts.proxmox_nodes + counts.docker_hosts + counts.kubernetes_clusters;
}

export function getMonitoredSystemLegacyBreakdown(counts: LegacyConnectionCounts): string {
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
      `${counts.kubernetes_clusters} Kubernetes ${
        counts.kubernetes_clusters === 1 ? 'cluster' : 'clusters'
      }`,
    );
  }

  return parts.join(', ');
}

export function getMonitoredSystemMigrationMessage(counts: LegacyConnectionCounts): string {
  const total = getMonitoredSystemLegacyConnectionTotal(counts);
  if (total <= 0) return '';

  const noun = total === 1 ? 'resource' : 'resources';
  const breakdown = getMonitoredSystemLegacyBreakdown(counts);
  return `You also have ${total} ${noun} connected via API or legacy collectors${
    breakdown ? ` (${breakdown})` : ''
  } that count once toward your monitored-system cap when the same top-level system is discovered canonically.`;
}

export function getMonitoredSystemOverflowSummary(daysRemaining: number | undefined): string {
  if (!daysRemaining) return '';
  return `Includes 1 temporary onboarding slot (${daysRemaining}d remaining)`;
}

export function getMonitoredSystemBannerToneClass(isUrgent: boolean): string {
  return isUrgent
    ? 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-900 dark:text-amber-100'
    : 'border-sky-200 bg-sky-50 text-sky-950 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-100';
}

export function getMonitoredSystemMigrationTextClass(isUrgent: boolean): string {
  return isUrgent
    ? 'text-amber-800 dark:text-amber-200'
    : 'text-sky-800 dark:text-sky-200';
}
