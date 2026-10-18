import type { PhysicalDiskCollectionStatus, PhysicalDiskFieldStatus } from '@/types/resource';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import { formatTemperature } from '@/utils/temperature';

// The canonical decision on whether a disk temperature is a current reading.
// It lives apart from diskPresentation so the Machines table, the machine
// drawer and the guest drawer can use it without pulling the Storage page
// presentation module into their chunks.

// A retained reading takes no threshold colour, so it cannot read as a disk
// running hot or cool right now. The dotted underline marks the value as one
// its title explains.
export const PHYSICAL_DISK_TEMPERATURE_LAST_KNOWN_CLASS =
  'text-muted underline decoration-dotted underline-offset-2 cursor-help';

// Normalization may keep the last known temperature when the current
// observation is not available, such as a disk in standby or a host agent that
// stopped reporting. The value is a current reading only when its collection
// state is available, or when the source predates collection state.
export function isPhysicalDiskTemperatureCurrent(
  collection: PhysicalDiskCollectionStatus | null | undefined,
): boolean {
  const state = collection?.temperature?.state;
  return !state || state === 'available';
}

export function getPhysicalDiskLastKnownTemperatureTitle(
  status: PhysicalDiskFieldStatus | null | undefined,
): string {
  const reason = status?.reason?.trim();
  return reason ? `Last known reading, not current: ${reason}` : 'Last known reading, not current';
}

export interface PhysicalDiskTemperaturePresentation {
  label: string;
  current: boolean;
  /** Set only for a last-known reading: why the value is not current. */
  title?: string;
}

export function getPhysicalDiskTemperaturePresentation(disk: {
  temperature: number;
  collection?: PhysicalDiskCollectionStatus | null;
}): PhysicalDiskTemperaturePresentation | null {
  if (!Number.isFinite(disk.temperature) || disk.temperature <= 0) return null;
  const current = isPhysicalDiskTemperatureCurrent(disk.collection);
  return {
    label: formatTemperature(disk.temperature),
    current,
    title: current
      ? undefined
      : getPhysicalDiskLastKnownTemperatureTitle(disk.collection?.temperature),
  };
}

// Disk heat is judged by the alert disk temperature policy, through the
// thresholds the Temp column colours a reading by: a disk runs hot from its
// type's alert trigger (`critical`), the reading its disk temperature alert
// fires at. The band just below the trigger is a colour, not heat. A retained
// reading is not current, and null thresholds mean disk temperature alerting
// is off for the disk, so neither is ever hot. Disk risk never carries heat.
export function isPhysicalDiskRunningHot(
  disk: { temperature: number; collection?: PhysicalDiskCollectionStatus | null },
  thresholds: MetricDisplayThresholds | null,
): boolean {
  if (!thresholds || !(thresholds.critical > 0)) return false;
  if (!Number.isFinite(disk.temperature) || disk.temperature <= 0) return false;
  if (!isPhysicalDiskTemperatureCurrent(disk.collection)) return false;
  return disk.temperature >= thresholds.critical;
}

export function getPhysicalDiskHeatSummary(
  temperature: number,
  thresholds: MetricDisplayThresholds | null,
): string {
  const reading = formatTemperature(temperature);
  return thresholds && thresholds.critical > 0
    ? `Disk temperature is ${reading}, at or above its ${formatTemperature(thresholds.critical)} alert threshold.`
    : `Disk temperature is ${reading}.`;
}
