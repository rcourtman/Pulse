import {
  formatMonitoredSystemLegacyConnectionBreakdown,
  formatMonitoredSystemLimitSummary,
  formatMonitoredSystemMigrationMessage,
  formatMonitoredSystemOverflowSummary,
  getMonitoredSystemLimitInstallCollectorsLabel,
  getMonitoredSystemLimitLearnMoreLabel,
  getMonitoredSystemLimitUpgradeLabel,
  isMonitoredSystemLimitUrgent as isCanonicalMonitoredSystemLimitUrgent,
  isMonitoredSystemLimitUsageAvailable as isCanonicalMonitoredSystemLimitUsageAvailable,
  type MonitoredSystemLimitUsageStatus,
  type MonitoredSystemLegacyConnectionCounts,
} from '@/utils/monitoredSystemPresentation';
import { SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF } from '@/utils/pricingHandoff';

type LimitState = MonitoredSystemLimitUsageStatus & {
  current: number;
  limit: number;
  state?: string;
};

export const MONITORED_SYSTEM_LIMIT_KEY = 'max_monitored_systems';
export const MONITORED_SYSTEM_LIMIT_LEARN_MORE_HREF =
  SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF;
export const MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_HREF = '/settings';
export const MONITORED_SYSTEM_LIMIT_LEARN_MORE_LABEL = getMonitoredSystemLimitLearnMoreLabel();
export const MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_LABEL =
  getMonitoredSystemLimitInstallCollectorsLabel();
export const MONITORED_SYSTEM_LIMIT_UPGRADE_LABEL = getMonitoredSystemLimitUpgradeLabel();

export function isMonitoredSystemLimitUsageAvailable(limit: LimitState | undefined): boolean {
  return isCanonicalMonitoredSystemLimitUsageAvailable(limit);
}

export function isMonitoredSystemLimitUrgent(limit: LimitState | undefined): boolean {
  return isCanonicalMonitoredSystemLimitUrgent(limit);
}

export function shouldShowMonitoredSystemLimitBanner(limit: LimitState | undefined): boolean {
  return Boolean(limit) && isMonitoredSystemLimitUrgent(limit);
}

export function getMonitoredSystemSummary(limit: LimitState | undefined): string {
  if (!limit || !isMonitoredSystemLimitUsageAvailable(limit)) return '';
  return formatMonitoredSystemLimitSummary(limit);
}

export function getMonitoredSystemLegacyConnectionTotal(
  counts: MonitoredSystemLegacyConnectionCounts,
): number {
  return counts.proxmox_nodes + counts.docker_hosts + counts.kubernetes_clusters;
}

export function getMonitoredSystemLegacyBreakdown(
  counts: MonitoredSystemLegacyConnectionCounts,
): string {
  return formatMonitoredSystemLegacyConnectionBreakdown(counts);
}

export function getMonitoredSystemMigrationMessage(
  counts: MonitoredSystemLegacyConnectionCounts,
): string {
  return formatMonitoredSystemMigrationMessage(counts);
}

export function getMonitoredSystemOverflowSummary(
  daysRemaining: number | null | undefined,
): string {
  return formatMonitoredSystemOverflowSummary(daysRemaining ?? undefined);
}

export function getMonitoredSystemBannerToneClass(isUrgent: boolean): string {
  return isUrgent
    ? 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-900 dark:text-amber-100'
    : 'border-sky-200 bg-sky-50 text-sky-950 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-100';
}

export function getMonitoredSystemMigrationTextClass(isUrgent: boolean): string {
  return isUrgent ? 'text-amber-800 dark:text-amber-200' : 'text-sky-800 dark:text-sky-200';
}
