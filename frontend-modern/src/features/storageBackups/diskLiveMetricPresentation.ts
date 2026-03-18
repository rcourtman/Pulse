import { formatBytes } from '@/utils/format';

export type DiskLiveMetricType = 'read' | 'write' | 'ioTime';

export const getDiskLiveMetricTextClass = (
  value: number,
  type: DiskLiveMetricType,
): string => {
  if (type === 'ioTime') {
    if (value > 90) return 'text-red-600 dark:text-red-400 font-bold';
    if (value > 50) return 'text-yellow-600 dark:text-yellow-400 font-semibold';
    return 'text-muted';
  }

  if (value > 100 * 1024 * 1024) {
    return 'text-blue-600 dark:text-blue-400 font-semibold';
  }

  return 'text-muted';
};

export const getDiskLiveMetricFormattedValue = (
  value: number,
  type: DiskLiveMetricType,
): string => {
  if (type === 'ioTime') return `${Math.round(value)}%`;
  return `${formatBytes(value)}/s`;
};
