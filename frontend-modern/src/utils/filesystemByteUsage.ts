import type { Disk } from '@/types/api';

// A capacity or explicit unknown sentinel is not a healthy empty filesystem.
// DiskInfo omits genuine zero used bytes; retain that wire contract while
// requiring finite non-negative used bytes and a finite positive capacity.
export const getFilesystemByteUsagePercent = (disk: Disk): number | null => {
  if (typeof disk.usage === 'number' && disk.usage < 0) return null;
  const total = disk.total;
  const used = disk.used ?? 0;
  if (
    typeof total !== 'number' ||
    !Number.isFinite(total) ||
    total <= 0 ||
    !Number.isFinite(used) ||
    used < 0
  ) {
    return null;
  }
  const percent = (used / total) * 100;
  return Number.isFinite(percent) ? percent : null;
};
