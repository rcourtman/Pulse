import {
  formatMonitoredSystemLegacyConnectionBreakdown,
  formatMonitoredSystemLimitSummary,
  formatMonitoredSystemMigrationMessage,
  formatMonitoredSystemOverflowSummary,
  getMonitoredSystemLimitInstallCollectorsLabel,
  getMonitoredSystemLimitViewCapacityLabel,
  isMonitoredSystemLimitUrgent as isCanonicalMonitoredSystemLimitUrgent,
  isMonitoredSystemLimitUsageAvailable as isCanonicalMonitoredSystemLimitUsageAvailable,
  type MonitoredSystemCapacityStatus,
  type MonitoredSystemLimitUsageStatus,
  type MonitoredSystemLegacyConnectionCounts,
} from '@/utils/monitoredSystemPresentation';
import { SELF_HOSTED_PRO_BILLING_PLAN_HREF } from '@/utils/pricingHandoff';

type LimitState = MonitoredSystemLimitUsageStatus & {
  current: number;
  limit: number;
  state?: string;
};

export const MONITORED_SYSTEM_LIMIT_KEY = 'max_monitored_systems';
export const MONITORED_SYSTEM_LIMIT_VIEW_CAPACITY_HREF = SELF_HOSTED_PRO_BILLING_PLAN_HREF;
export const MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_HREF = '/settings';
export const MONITORED_SYSTEM_LIMIT_VIEW_CAPACITY_LABEL =
  getMonitoredSystemLimitViewCapacityLabel();
export const MONITORED_SYSTEM_LIMIT_INSTALL_COLLECTORS_LABEL =
  getMonitoredSystemLimitInstallCollectorsLabel();

export function isMonitoredSystemLimitUsageAvailable(limit: LimitState | undefined): boolean {
  return isCanonicalMonitoredSystemLimitUsageAvailable(limit);
}

export function isMonitoredSystemLimitUrgent(
  limit: LimitState | undefined,
  capacity?: MonitoredSystemCapacityStatus | null,
): boolean {
  return isCanonicalMonitoredSystemLimitUrgent(limit, capacity);
}

export function shouldShowMonitoredSystemLimitBanner(
  limit: LimitState | undefined,
  capacity?: MonitoredSystemCapacityStatus | null,
): boolean {
  return Boolean(limit || capacity) && isMonitoredSystemLimitUrgent(limit, capacity);
}

export function getMonitoredSystemSummary(
  limit: LimitState | undefined,
  capacity?: MonitoredSystemCapacityStatus | null,
): string {
  if (!limit && !capacity) return '';
  if (limit && !isMonitoredSystemLimitUsageAvailable(limit)) return '';
  return formatMonitoredSystemLimitSummary(
    limit ?? {
      current: capacity?.current ?? 0,
      limit: capacity?.limit ?? 0,
      current_available: capacity?.current_available ?? true,
      current_unavailable_reason: capacity?.current_unavailable_reason,
      state: capacity?.urgency ?? 'ok',
    },
    capacity,
  );
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
