import { getMetricColorClass } from '@/utils/metricThresholds';

export const DASHBOARD_STORAGE_EMPTY_STATE = 'No storage resources';

export interface DashboardStorageIssueBadge {
  label: string;
  className: string;
}

export function computeDashboardStorageCapacityPercent(used: number, total: number): number {
  if (!Number.isFinite(used) || !Number.isFinite(total) || total <= 0) return 0;
  return Math.min(100, Math.max(0, (used / total) * 100));
}

export function getDashboardStorageCapacityBarClass(percent: number): string {
  return getMetricColorClass(percent, 'disk');
}

export function getDashboardStorageIssueBadges(counts: {
  warningCount: number;
  criticalCount: number;
}): DashboardStorageIssueBadge[] {
  const badges: DashboardStorageIssueBadge[] = [];
  if (counts.warningCount > 0) {
    badges.push({
      label: `${counts.warningCount} warnings`,
      className: 'font-medium text-amber-600 dark:text-amber-400',
    });
  }
  if (counts.criticalCount > 0) {
    badges.push({
      label: `${counts.criticalCount} critical`,
      className: 'font-medium text-red-600 dark:text-red-400',
    });
  }
  return badges;
}
