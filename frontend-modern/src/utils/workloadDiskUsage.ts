import type { Disk } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { getWorkloadGuestDiskRead } from '@/utils/workloadGuestPresentation';
import { resolveWorkloadType } from '@/utils/workloads';

// Workload aggregate metrics use percentages, including values below 1%.
// Unlike DiskInfo filesystem entries (whose zero bytes are omitted on the
// wire), an aggregate capacity alone is not an observed usage measurement.
export const getWorkloadDiskUsagePercent = (disk?: Disk | null): number | null => {
  if (!disk) return null;
  if (disk.usage !== undefined) {
    return typeof disk.usage === 'number' && Number.isFinite(disk.usage) && disk.usage >= 0
      ? Math.min(100, disk.usage)
      : null;
  }
  const { used, total } = disk;
  if (
    typeof used !== 'number' ||
    !Number.isFinite(used) ||
    used < 0 ||
    typeof total !== 'number' ||
    !Number.isFinite(total) ||
    total <= 0
  ) {
    return null;
  }
  const percent = (used / total) * 100;
  return Number.isFinite(percent) ? Math.min(100, percent) : null;
};

export const getCurrentWorkloadDiskUsage = (guest: WorkloadGuest): number | null =>
  !guest ||
  guest.telemetryAvailability?.disk === false ||
  getWorkloadGuestDiskRead(guest, resolveWorkloadType(guest) === 'vm').state !== 'current'
    ? null
    : getWorkloadDiskUsagePercent(guest.disk);
