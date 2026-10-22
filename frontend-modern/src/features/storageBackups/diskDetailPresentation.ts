import {
  getPhysicalDiskTemperaturePresentation,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';
import { formatPowerOnHours } from '@/utils/format';
import { getMetricSeverity, type MetricDisplayThresholds } from '@/utils/metricThresholds';
import type { StatusIndicatorVariant } from '@/utils/status';

export function getLinkedDiskHealthDotVariant(hasIssue: boolean): StatusIndicatorVariant {
  return hasIssue ? 'warning' : 'success';
}

/** How hot a linked disk's current reading is under its disk heat policy. */
export function getLinkedDiskTemperatureSeverity(
  tempCelsius: number,
  thresholds?: MetricDisplayThresholds | null,
): 'critical' | 'warning' | null {
  // Null thresholds mean disk temperature alerting is off for this disk.
  if (!Number.isFinite(tempCelsius) || tempCelsius <= 0 || thresholds === null) {
    return null;
  }
  const severity = getMetricSeverity(tempCelsius, 'diskTemperature', thresholds);
  return severity === 'critical' || severity === 'warning' ? severity : null;
}

export function getLinkedDiskTemperatureTextClass(
  tempCelsius: number,
  thresholds?: MetricDisplayThresholds | null,
): string {
  const severity = getLinkedDiskTemperatureSeverity(tempCelsius, thresholds);
  if (severity === 'critical') {
    return 'text-red-500';
  }
  if (severity === 'warning') {
    return 'text-yellow-500';
  }
  return 'text-muted';
}

export type DiskDetailAttributeCard = {
  label: string;
  value: string;
  ok: boolean;
  /** A retained value that is not a current reading. */
  lastKnown?: boolean;
};

export function getDiskDetailAttributeCards(
  disk: PhysicalDiskPresentationData,
  diskTempThresholds?: MetricDisplayThresholds | null,
): DiskDetailAttributeCard[] {
  // Temperature is reported independently of optional extended SMART attributes.
  const attrs = disk.smartAttributes ?? {};

  const cards: DiskDetailAttributeCard[] = [];
  const isNvme = disk.type?.toLowerCase() === 'nvme';

  if (attrs.powerOnHours != null) {
    cards.push({
      label: 'Power-On Time',
      value: formatPowerOnHours(attrs.powerOnHours),
      ok: true,
    });
  }

  const temperature = getPhysicalDiskTemperaturePresentation(disk);
  if (temperature?.current) {
    cards.push({
      label: 'Temperature',
      value: temperature.label,
      ok:
        diskTempThresholds === null ||
        getMetricSeverity(disk.temperature, 'diskTemperature', diskTempThresholds) !== 'critical',
    });
  } else if (temperature) {
    // A retained reading is neither healthy nor hot now. The collection
    // message above the cards says why it is not current.
    cards.push({
      label: 'Last known temperature',
      value: temperature.label,
      ok: true,
      lastKnown: true,
    });
  }

  if (attrs.powerCycles != null) {
    cards.push({
      label: 'Power Cycles',
      value: attrs.powerCycles.toLocaleString(),
      ok: true,
    });
  }

  if (!isNvme && attrs.reallocatedSectors != null) {
    cards.push({
      label: 'Reallocated Sectors',
      value: attrs.reallocatedSectors.toString(),
      ok: attrs.reallocatedSectors === 0,
    });
  }
  if (!isNvme && attrs.pendingSectors != null) {
    cards.push({
      label: 'Pending Sectors',
      value: attrs.pendingSectors.toString(),
      ok: attrs.pendingSectors === 0,
    });
  }
  if (!isNvme && attrs.offlineUncorrectable != null) {
    cards.push({
      label: 'Offline Uncorrectable',
      value: attrs.offlineUncorrectable.toString(),
      ok: attrs.offlineUncorrectable === 0,
    });
  }
  if (!isNvme && attrs.udmaCrcErrors != null) {
    cards.push({
      label: 'CRC Errors',
      value: attrs.udmaCrcErrors.toString(),
      ok: attrs.udmaCrcErrors === 0,
    });
  }

  if (isNvme && attrs.percentageUsed != null) {
    cards.push({
      label: 'Life Used',
      value: `${attrs.percentageUsed}%`,
      ok: attrs.percentageUsed <= 90,
    });
  }
  if (isNvme && attrs.availableSpare != null) {
    cards.push({
      label: 'Available Spare',
      value: `${attrs.availableSpare}%`,
      ok: attrs.availableSpare >= 20,
    });
  }
  if (isNvme && attrs.mediaErrors != null) {
    cards.push({
      label: 'Media Errors',
      value: attrs.mediaErrors.toString(),
      ok: attrs.mediaErrors === 0,
    });
  }
  if (isNvme && attrs.unsafeShutdowns != null) {
    cards.push({
      label: 'Unsafe Shutdowns',
      value: attrs.unsafeShutdowns.toLocaleString(),
      ok: true,
    });
  }

  return cards;
}
