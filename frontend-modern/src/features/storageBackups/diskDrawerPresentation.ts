import {
  compactDetailRows,
  compactDetailSections,
  formatDetailIntegerValue,
  makeDetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/detailSectionModel';
import type {
  GuestDrawerHistoryDeferredMetric,
  GuestDrawerHistoryGroupConfig,
} from '@/components/Workloads/guestDrawerModel';
import {
  getDiskDetailAttributeCards,
  type DiskDetailAttributeCard,
} from '@/features/storageBackups/diskDetailPresentation';
import {
  getPhysicalDiskCollectionMessages,
  getPhysicalDiskHealthIndicator,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskHostLabel,
  getPhysicalDiskLifeLabel,
  getPhysicalDiskLifeTextClass,
  getPhysicalDiskParentLabel,
  getPhysicalDiskRoleLabel,
  getPhysicalDiskSourceBadgePresentation,
  getPhysicalDiskTemperaturePresentation,
  getPhysicalDiskTemperatureThresholds,
  isPhysicalDiskWearoutReported,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';
import type { Resource } from '@/types/resource';
import { formatBytes } from '@/utils/format';

// The disk drawer charts with the drawer history component, from the series
// the metrics store keeps for a physical disk. Temperature leads (it is the
// disk's first panel on a phone), then I/O, then the SMART counter that shows
// wear for the disk's kind. Module constants keep the chart grid from
// remounting on every disk snapshot.
const DISK_TEMPERATURE_GROUP: GuestDrawerHistoryGroupConfig = {
  id: 'disk-thermal',
  label: 'Temperature',
  unit: 'C',
  series: [{ metric: 'smart_temp', label: 'Disk', unit: 'C', color: '#ef4444' }],
};

const DISK_IO_GROUPS: readonly GuestDrawerHistoryGroupConfig[] = [
  {
    id: 'disk-io',
    label: 'Disk I/O',
    unit: 'B/s',
    series: [
      { metric: 'diskread', label: 'Read', unit: 'B/s', color: '#3b82f6' },
      { metric: 'diskwrite', label: 'Write', unit: 'B/s', color: '#f59e0b' },
    ],
  },
  {
    id: 'disk-activity',
    label: 'Busy',
    unit: '%',
    series: [{ metric: 'disk', label: 'Busy', unit: '%', color: '#8b5cf6' }],
  },
];

const ATA_WEAR_GROUP: GuestDrawerHistoryGroupConfig = {
  id: 'disk-reallocated',
  label: 'Reallocated sectors',
  unit: 'sectors',
  series: [
    {
      metric: 'smart_reallocated_sectors',
      label: 'Reallocated',
      unit: 'sectors',
      color: '#f59e0b',
    },
  ],
};

const NVME_WEAR_GROUP: GuestDrawerHistoryGroupConfig = {
  id: 'disk-wear',
  label: 'Wear',
  unit: '%',
  series: [
    { metric: 'smart_percentage_used', label: 'Life used', unit: '%', color: '#f59e0b' },
    { metric: 'smart_available_spare', label: 'Spare', unit: '%', color: '#10b981' },
  ],
};

const DISK_DRAWER_GROUPS = {
  ata: [DISK_TEMPERATURE_GROUP, ...DISK_IO_GROUPS, ATA_WEAR_GROUP],
  ataNoIO: [DISK_TEMPERATURE_GROUP, ATA_WEAR_GROUP],
  nvme: [DISK_TEMPERATURE_GROUP, ...DISK_IO_GROUPS, NVME_WEAR_GROUP],
  nvmeNoIO: [DISK_TEMPERATURE_GROUP, NVME_WEAR_GROUP],
} satisfies Record<string, GuestDrawerHistoryGroupConfig[]>;

const isNvme = (disk: PhysicalDiskPresentationData): boolean => disk.type?.toLowerCase() === 'nvme';

/**
 * The disk's chart panels. I/O panels are left out where the collector says
 * per-disk I/O is not available, so no empty chart reads as an idle disk.
 */
export function getDiskDrawerHistoryGroups(
  disk: PhysicalDiskPresentationData,
  ioAvailable: boolean,
): GuestDrawerHistoryGroupConfig[] {
  if (isNvme(disk)) return ioAvailable ? DISK_DRAWER_GROUPS.nvme : DISK_DRAWER_GROUPS.nvmeNoIO;
  return ioAvailable ? DISK_DRAWER_GROUPS.ata : DISK_DRAWER_GROUPS.ataNoIO;
}

export function isDiskDrawerIOAvailable(disk: PhysicalDiskPresentationData): boolean {
  const state = disk.collection?.io?.state;
  return !state || state === 'available';
}

const finite = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

/** Current readings the charts show before any history is stored. */
export function getDiskDrawerCurrentMetrics(
  disk: PhysicalDiskPresentationData,
): Record<string, number | undefined> {
  const temperature = getPhysicalDiskTemperaturePresentation(disk);
  const attrs = disk.smartAttributes ?? {};
  return {
    smart_temp: temperature?.current ? disk.temperature : undefined,
    smart_reallocated_sectors: finite(attrs.reallocatedSectors),
    smart_percentage_used: finite(attrs.percentageUsed),
    smart_available_spare: finite(attrs.availableSpare),
  };
}

/** A retained temperature is charted as last known, never as live. */
export function getDiskDrawerDeferredMetrics(
  disk: PhysicalDiskPresentationData,
): Record<string, GuestDrawerHistoryDeferredMetric> | undefined {
  const temperature = getPhysicalDiskTemperaturePresentation(disk);
  if (!temperature || temperature.current || !temperature.title) return undefined;
  return {
    smart_temp: { lastKnownValue: disk.temperature, message: `${temperature.title}.` },
  };
}

const lifeTone = (disk: PhysicalDiskPresentationData): DetailValueTone => {
  if (!isPhysicalDiskWearoutReported(disk)) return 'muted';
  const textClass = getPhysicalDiskLifeTextClass(disk);
  if (textClass.includes('red')) return 'danger';
  if (textClass.includes('amber')) return 'warning';
  return 'success';
};

const attributeTone = (card: DiskDetailAttributeCard): DetailValueTone => {
  if (card.lastKnown) return 'muted';
  return card.ok ? 'default' : 'danger';
};

/**
 * The SMART health card in front: the verdict the Health cell shows with its
 * reason, the temperature and, for SSDs, the life left.
 */
export function buildDiskDrawerHealthSection(disk: PhysicalDiskPresentationData): DetailSection {
  const status = getPhysicalDiskHealthStatus(disk);
  const indicator = getPhysicalDiskHealthIndicator(disk);
  const summary = getPhysicalDiskHealthSummary(status);
  const temperature = getPhysicalDiskTemperaturePresentation(disk);
  return {
    label: 'SMART health',
    testId: 'disk-drawer-health',
    rows: compactDetailRows([
      makeDetailRow('Health', summary ? `${status.label}: ${summary}` : status.label, {
        tone:
          indicator.variant === 'danger'
            ? 'danger'
            : indicator.variant === 'warning'
              ? 'warning'
              : indicator.variant === 'success'
                ? 'success'
                : 'default',
        wrap: true,
      }),
      temperature
        ? makeDetailRow(
            temperature.current ? 'Temperature' : 'Last known temperature',
            temperature.label,
            // A hot disk is stated by the verdict above; a retained reading
            // carries no health tone at all.
            temperature.current ? {} : { tone: 'muted', title: temperature.title },
          )
        : null,
      // Worn flash already turns the verdict above; the life left then stays
      // in this card instead of repeating the verdict in Worth checking.
      makeDetailRow('Life remaining', getPhysicalDiskLifeLabel(disk), {
        tone:
          indicator.variant === 'warning' || indicator.variant === 'danger'
            ? 'default'
            : lifeTone(disk),
      }),
    ]),
  };
}

/**
 * Every SMART attribute the disk reports, for Details. An attribute out of
 * range carries a danger tone, so the drawer lifts it into Worth checking.
 * Temperature lives in the health card instead.
 */
export function buildDiskDrawerSmartSection(disk: PhysicalDiskPresentationData): DetailSection {
  const cards = getDiskDetailAttributeCards(disk, getPhysicalDiskTemperatureThresholds(disk));
  return {
    label: 'SMART attributes',
    testId: 'disk-drawer-smart-attributes',
    rows: compactDetailRows(
      cards
        .filter((card) => card.label !== 'Temperature' && card.label !== 'Last known temperature')
        .map((card) => makeDetailRow(card.label, card.value, { tone: attributeTone(card) })),
    ),
  };
}

/** Identity and placement facts operators copy and compare: they wrap. */
export function buildDiskDrawerIdentitySection(
  disk: PhysicalDiskPresentationData,
  resource: Resource,
): DetailSection {
  return {
    label: 'Disk',
    rows: compactDetailRows([
      makeDetailRow('Model', disk.model, { wrap: true }),
      makeDetailRow('Vendor', disk.vendor, { wrap: true }),
      makeDetailRow('Serial', disk.serial, { wrap: true }),
      makeDetailRow('WWN', disk.wwn, { wrap: true }),
      makeDetailRow('Device', disk.devPath, { wrap: true }),
      makeDetailRow('Host', getPhysicalDiskHostLabel(disk, resource), { wrap: true }),
      makeDetailRow('Source', getPhysicalDiskSourceBadgePresentation(resource).label),
      makeDetailRow('Type', getPhysicalDiskRoleLabel(disk)),
      makeDetailRow('Size', disk.size > 0 ? formatBytes(disk.size) : null),
      makeDetailRow('Speed', disk.rpm > 0 ? `${disk.rpm.toLocaleString()} rpm` : null),
      makeDetailRow('Belongs to', getPhysicalDiskParentLabel(disk), { wrap: true }),
      makeDetailRow('State', disk.storageState, { wrap: true }),
      makeDetailRow('Controller', disk.controller, { wrap: true }),
      makeDetailRow('Target', disk.target, { wrap: true }),
    ]),
  };
}

/** The disk's own counters; an error count is lifted into Worth checking. */
export function buildDiskDrawerActivitySection(disk: PhysicalDiskPresentationData): DetailSection {
  return {
    label: 'Activity',
    rows: compactDetailRows([
      makeDetailRow('Reads', formatDetailIntegerValue(disk.readCount)),
      makeDetailRow('Writes', formatDetailIntegerValue(disk.writeCount)),
      makeDetailRow(
        'Errors',
        typeof disk.errorCount === 'number' && disk.errorCount > 0
          ? formatDetailIntegerValue(disk.errorCount)
          : null,
        { tone: 'warning' },
      ),
      makeDetailRow('Spun down', disk.spunDown ? 'Yes' : null),
    ]),
  };
}

export function buildDiskDrawerDetailSections(
  disk: PhysicalDiskPresentationData,
  resource: Resource,
): DetailSection[] {
  return compactDetailSections([
    buildDiskDrawerIdentitySection(disk, resource),
    buildDiskDrawerSmartSection(disk),
    buildDiskDrawerActivitySection(disk),
  ]);
}

/**
 * What the collector could not read, and a disk that reports no SMART detail
 * at all, each said once in plain words above the charts.
 */
export function getDiskDrawerNotices(disk: PhysicalDiskPresentationData): string[] {
  const messages = getPhysicalDiskCollectionMessages(disk);
  const cards = getDiskDetailAttributeCards(disk, getPhysicalDiskTemperatureThresholds(disk));
  return cards.length === 0 && messages.length === 0
    ? ['Detailed SMART attributes are not available for this disk.']
    : messages;
}

const isPlaceholder = (value: string | undefined): boolean => {
  const trimmed = (value ?? '').trim();
  return !trimmed || trimmed === '—' || trimmed === '-';
};

/** The short facts strip under the charts. */
export function getDiskDrawerFacts(
  disk: PhysicalDiskPresentationData,
  resource: Resource,
): string[] {
  const host = getPhysicalDiskHostLabel(disk, resource);
  return [
    [getPhysicalDiskRoleLabel(disk), disk.size > 0 ? formatBytes(disk.size) : '']
      .filter(Boolean)
      .join(' · '),
    isPlaceholder(host) ? '' : `On ${host}`,
    disk.devPath,
    getPhysicalDiskSourceBadgePresentation(resource).label,
  ].filter((fact) => !isPlaceholder(fact));
}
