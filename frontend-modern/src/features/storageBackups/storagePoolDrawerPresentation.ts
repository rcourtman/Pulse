import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/detailSectionModel';
import type {
  GuestDrawerHistoryDeferredMetric,
  GuestDrawerHistoryGroupConfig,
} from '@/components/Workloads/guestDrawerModel';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getStorageRecordContent,
  getStorageRecordHostLabel,
  getStorageRecordShared,
  getStorageRecordTopologyLabel,
} from '@/features/storageBackups/recordPresentation';
import {
  getStoragePoolStateLabel,
  getStoragePoolStateTitle,
  getStoragePoolStateTone,
} from '@/features/storageBackups/rowPresentation';
import type {
  StoragePoolDetailConfigRow,
  StoragePoolDetailLinkedDisk,
  StoragePoolDetailTopologyRow,
  StoragePoolDetailZfsSummary,
} from '@/features/storageBackups/storagePoolDetailPresentation';
import { formatBytes } from '@/utils/format';
import { formatTemperature } from '@/utils/temperature';

/** A linked disk's current heat under its disk heat policy. */
export type StoragePoolLinkedDiskHeat = 'critical' | 'warning' | null;

// A pool's stored history is its used capacity, the alert's own metric. The
// same catalog the resource drawer charts storage with.
export const STORAGE_POOL_DRAWER_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] = [
  {
    id: 'capacity',
    label: 'Capacity',
    unit: '%',
    series: [{ metric: 'usage', label: 'Used', unit: '%', color: '#22c55e' }],
  },
];

export const STORAGE_POOL_DRAWER_CHARTED_METRICS = STORAGE_POOL_DRAWER_HISTORY_GROUPS.flatMap(
  (group) => group.series.map((series) => series.metric),
);

const observed = (value: number | null | undefined): number | null =>
  typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : null;

const getUsagePercent = (record: StorageRecord): number | null => {
  const total = observed(record.capacity.totalBytes);
  const used = observed(record.capacity.usedBytes);
  const percent =
    observed(record.capacity.usagePercent) ??
    (total !== null && total > 0 && used !== null ? (used / total) * 100 : null);
  return percent === null ? null : Math.min(100, percent);
};

const STALE_POOL_MESSAGE = 'Retained last-known storage values, not a current reading.';

/** The reading the capacity chart shows before any history is stored. */
export function getStoragePoolDrawerCurrentMetrics(
  record: StorageRecord,
): Record<string, number | undefined> {
  if (record.freshness === 'stale') return {};
  return { usage: getUsagePercent(record) ?? undefined };
}

/** A stale pool's retained usage is shown as last known, not as live. */
export function getStoragePoolDrawerDeferredMetrics(
  record: StorageRecord,
): Record<string, GuestDrawerHistoryDeferredMetric> | undefined {
  if (record.freshness !== 'stale') return undefined;
  const usage = getUsagePercent(record);
  return { usage: { lastKnownValue: usage ?? undefined, message: STALE_POOL_MESSAGE } };
}

const titleize = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const getZfsStateTone = (state: string): DetailValueTone => {
  const normalized = (state || '').trim().toUpperCase();
  if (!normalized || normalized === 'ONLINE') return 'default';
  if (normalized === 'DEGRADED') return 'warning';
  return 'danger';
};

const isLiftedTone = (tone: DetailValueTone | undefined): boolean =>
  tone === 'warning' || tone === 'danger';

const plural = (count: number, singular: string, pluralLabel = `${singular}s`): string =>
  `${count.toLocaleString()} ${count === 1 ? singular : pluralLabel}`;

/** The pool's own ZFS state, scan and error counters. */
const buildZfsPoolSection = (zfs: StoragePoolDetailZfsSummary): DetailSection => ({
  label: 'ZFS pool',
  rows: compactDetailRows([
    // The pool's State in front already judges this word; here it is a fact.
    makeDetailRow('State', zfs.state),
    makeDetailRow('Scan', zfs.scan, {
      wrap: true,
      tone: zfs.scanErrors > 0 ? 'danger' : 'default',
    }),
    makeDetailRow('Errors', zfs.errorSummary, { tone: 'danger', wrap: true }),
  ]),
});

const getZfsDeviceValue = (device: StoragePoolDetailZfsSummary['devices'][number]): string =>
  [device.type, device.state, device.errorSummary, device.message].filter(Boolean).join(' · ');

const buildZfsDevicesSection = (zfs: StoragePoolDetailZfsSummary): DetailSection => ({
  label: `ZFS devices (${zfs.devices.length})`,
  rows: compactDetailRows(
    zfs.devices.map((device) =>
      // Device names are paths and by-id names operators copy: they wrap.
      makeDetailRow(device.name, getZfsDeviceValue(device) || 'Reported', {
        layout: 'stacked',
        wrap: true,
        tone: device.errorSummary ? 'danger' : getZfsStateTone(device.state),
      }),
    ),
  ),
});

/**
 * The pool's content cards: its ZFS state and its device tree. Rows that say
 * something is wrong are lifted into Worth checking instead of repeating here;
 * the device list stays whole so a failing member is read in its place.
 */
export function buildStoragePoolDrawerZfsSections(
  zfs: StoragePoolDetailZfsSummary | null,
): DetailSection[] {
  if (!zfs) return [];
  const pool = buildZfsPoolSection(zfs);
  return compactDetailSections([
    { ...pool, rows: pool.rows.filter((row) => !isLiftedTone(row.tone)) },
    zfs.devices.length > 0 ? buildZfsDevicesSection(zfs) : null,
  ]);
}

/** A linked disk's facts in one line, for its row's value and hover text. */
export function getStoragePoolLinkedDiskSummary(disk: StoragePoolDetailLinkedDisk): string {
  return [
    disk.model,
    disk.role,
    disk.sizeLabel,
    disk.state,
    disk.ioLabel,
    disk.spunDown ? 'spun down' : '',
    disk.errorCount > 0 ? `${disk.errorCount.toLocaleString()} errors` : '',
    disk.temperature > 0 ? formatTemperature(disk.temperature) : '',
  ]
    .filter(Boolean)
    .join(' · ');
}

export interface StoragePoolDrawerExceptionInput {
  record: StorageRecord;
  zfsSummary: StoragePoolDetailZfsSummary | null;
  linkedDisks: StoragePoolDetailLinkedDisk[];
  getDiskHeat: (disk: StoragePoolDetailLinkedDisk) => StoragePoolLinkedDiskHeat;
}

/**
 * "Worth checking" for a pool: a state that is not healthy, reduced
 * protection, ZFS pool and scrub errors, devices that are not online, and
 * linked disks with errors or running hot under the disk heat policy.
 */
export function buildStoragePoolDrawerExceptionRows(
  input: StoragePoolDrawerExceptionInput,
): DetailRow[] {
  const { record, zfsSummary } = input;
  const rows: Array<DetailRow | null> = [];

  const stateTone = getStoragePoolStateTone(record);
  if (stateTone === 'danger' || stateTone === 'warning') {
    // The State cell's word, then the reason its hover title gives.
    const label = getStoragePoolStateLabel(record);
    const reason = getStoragePoolStateTitle(record);
    rows.push(
      makeDetailRow('State', reason && reason !== label ? `${label}: ${reason}` : label, {
        tone: stateTone,
        wrap: true,
      }),
    );
  }

  if (record.protectionReduced) {
    rows.push(
      makeDetailRow(
        'Protection',
        record.protectionSummary?.trim() || record.protectionLabel?.trim() || 'Reduced',
        { tone: 'danger', wrap: true },
      ),
    );
  }

  if (zfsSummary) {
    rows.push(...buildZfsPoolSection(zfsSummary).rows.filter((row) => isLiftedTone(row.tone)));
    for (const device of zfsSummary.devices) {
      const tone = device.errorSummary ? 'danger' : getZfsStateTone(device.state);
      if (tone === 'default') continue;
      const problem = [
        device.state ? titleize(device.state) : '',
        device.errorSummary,
        device.message,
      ]
        .filter(Boolean)
        .join(', ');
      rows.push(makeDetailRow('Device', `${device.name}: ${problem}`, { tone, wrap: true }));
    }
  }

  for (const disk of input.linkedDisks) {
    if (disk.errorCount > 0) {
      rows.push(
        makeDetailRow('Disk', `${disk.devPath}: ${plural(disk.errorCount, 'error')}`, {
          tone: 'warning',
          wrap: true,
        }),
      );
    } else if (disk.hasIssue) {
      rows.push(
        makeDetailRow('Disk', `${disk.devPath}: reallocated sectors`, {
          tone: 'warning',
          wrap: true,
        }),
      );
    }
    // A disk runs hot from its alert trigger (the disk heat policy); the
    // band below it is a colour, not heat. A retained reading is not hot now.
    if (
      disk.temperature > 0 &&
      !disk.temperatureLastKnownTitle &&
      input.getDiskHeat(disk) === 'critical'
    ) {
      rows.push(
        makeDetailRow(
          'Disk temperature',
          `${disk.devPath}: ${formatTemperature(disk.temperature)}`,
          {
            tone: 'danger',
            wrap: true,
          },
        ),
      );
    }
  }

  return compactDetailRows(rows);
}

export interface StoragePoolDrawerDetailInput {
  configRows: StoragePoolDetailConfigRow[];
  topologyRows: StoragePoolDetailTopologyRow[];
  zfsSummary: StoragePoolDetailZfsSummary | null;
}

const getDatasetValue = (dataset: StoragePoolDetailZfsSummary['datasets'][number]): string =>
  [
    `Used ${dataset.usedLabel}`,
    `Available ${dataset.availableLabel}`,
    `Referenced ${dataset.referencedLabel}`,
    dataset.mountpoint || (dataset.type === 'volume' ? 'zvol' : ''),
  ]
    .filter(Boolean)
    .join(' · ');

/** The long tail kept in the closed Details disclosure. */
export function buildStoragePoolDrawerDetailSections(
  input: StoragePoolDrawerDetailInput,
): DetailSection[] {
  const datasets = input.zfsSummary?.datasets ?? [];
  return compactDetailSections([
    {
      label: 'Configuration',
      rows: compactDetailRows(
        input.configRows.map((row) => makeDetailRow(row.label, row.value, { wrap: true })),
      ),
    },
    {
      label: 'Topology',
      rows: compactDetailRows(
        input.topologyRows.map((row) => makeDetailRow(row.label, row.value, { wrap: true })),
      ),
    },
    {
      label: `Datasets (${datasets.length})`,
      testId: 'storage-pool-datasets',
      rows: compactDetailRows(
        datasets.map((dataset) =>
          // Dataset names and mountpoints are copied, so they wrap.
          makeDetailRow(dataset.name, getDatasetValue(dataset), {
            layout: 'stacked',
            wrap: true,
          }),
        ),
      ),
    },
  ]);
}

const isPlaceholder = (value: string): boolean => {
  const trimmed = value.trim();
  return !trimmed || trimmed === '—' || trimmed === '-' || trimmed === 'unassigned';
};

/** The short facts strip under the charts. */
export function getStoragePoolDrawerFacts(record: StorageRecord): string[] {
  const total = observed(record.capacity.totalBytes);
  const used = observed(record.capacity.usedBytes);
  // A stale pool states its size only, not a used amount that reads as live.
  const capacity =
    total !== null && total > 0
      ? used !== null && record.freshness !== 'stale'
        ? `${formatBytes(used)} of ${formatBytes(total)} used`
        : `${formatBytes(total)} total`
      : '';
  const host = getStorageRecordHostLabel(record);
  const content = getStorageRecordContent(record);
  return [
    getStorageRecordTopologyLabel(record),
    capacity,
    isPlaceholder(host) ? '' : `On ${host}`,
    content
      ? `Holds ${content
          .split(',')
          .map((part) => part.trim())
          .join(', ')}`
      : '',
    getStorageRecordShared(record) === true ? 'Shared' : '',
  ].filter((fact) => !isPlaceholder(fact));
}
