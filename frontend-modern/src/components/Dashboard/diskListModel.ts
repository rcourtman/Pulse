import type { Disk } from '@/types/api';

import { formatBytes } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';

export interface DiskListProps {
  disks: Disk[];
  diskStatusReason?: string;
}

export interface DashboardDiskPresentation {
  key: string;
  label: string;
  labelTitle?: string;
  progressClass: string;
  progressWidth: string;
  typeLabel: string;
  usageText: string;
  usagePercentLabel: string;
}

export const hasDashboardDiskCapacity = (disk: Disk): boolean =>
  typeof disk.total === 'number' && disk.total > 0;

export const getDashboardDiskUsagePercent = (disk: Disk): number => {
  if (!hasDashboardDiskCapacity(disk)) {
    return 0;
  }

  return (disk.used / disk.total) * 100;
};

export const getDashboardDiskLabel = (disk: Disk): string =>
  disk.mountpoint || disk.device || 'Unknown';

export const getDashboardDiskLabelTitle = (label: string): string | undefined =>
  label !== 'Unknown' ? label : undefined;

export const getDashboardDiskUsageText = (disk: Disk): string =>
  hasDashboardDiskCapacity(disk)
    ? `${formatBytes(disk.used)}/${formatBytes(disk.total)}`
    : 'Usage unavailable';

export const getDashboardDiskUsagePercentLabel = (disk: Disk): string =>
  hasDashboardDiskCapacity(disk) ? `${getDashboardDiskUsagePercent(disk).toFixed(0)}%` : '—';

export const getDashboardDiskProgressClass = (disk: Disk): string =>
  getMetricColorClass(getDashboardDiskUsagePercent(disk), 'disk');

export const getDashboardDiskProgressWidth = (disk: Disk): string =>
  `${Math.min(getDashboardDiskUsagePercent(disk), 100)}%`;

export const getDashboardDiskTypeLabel = (disk: Disk): string => disk.type?.toUpperCase() ?? '';

export const buildDashboardDiskPresentation = (
  disk: Disk,
  index: number,
): DashboardDiskPresentation => {
  const label = getDashboardDiskLabel(disk);

  return {
    key: `${disk.mountpoint ?? ''}:${disk.device ?? ''}:${index}`,
    label,
    labelTitle: getDashboardDiskLabelTitle(label),
    progressClass: getDashboardDiskProgressClass(disk),
    progressWidth: getDashboardDiskProgressWidth(disk),
    typeLabel: getDashboardDiskTypeLabel(disk),
    usageText: getDashboardDiskUsageText(disk),
    usagePercentLabel: getDashboardDiskUsagePercentLabel(disk),
  };
};
