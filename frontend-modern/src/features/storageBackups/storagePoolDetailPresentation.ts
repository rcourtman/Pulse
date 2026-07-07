import type { HistoryTimeRange, ResourceType as HistoryChartResourceType } from '@/api/charts';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getStorageRecordContent,
  getStorageRecordNodeLabel,
  getStorageRecordPlatformLabel,
  getStorageRecordShared,
  getStorageRecordStatus,
  getStorageRecordType,
  getStorageRecordUsagePercent,
  getStorageRecordZfsPool,
} from '@/features/storageBackups/recordPresentation';
import { resolveStorageRecordMetricResourceId } from '@/features/storageBackups/storageMetricsIdentity';
import type { Resource } from '@/types/resource';
import { formatBytes, formatPercent } from '@/utils/format';

export type StoragePoolDetailConfigRow = {
  label: string;
  value: string;
};

export type StoragePoolDetailLinkedDisk = {
  id: string;
  devPath: string;
  model: string;
  role: string;
  state: string;
  sizeLabel: string;
  diskType: string;
  temperature: number;
  hasIssue: boolean;
  spunDown: boolean;
  errorCount: number;
  ioLabel: string;
};

export type StoragePoolDetailTopologyRow = {
  label: string;
  value: string;
};

export type StoragePoolDetailChartTarget = {
  resourceType: HistoryChartResourceType;
  resourceId: string | null;
};

export type StoragePoolDetailZfsDevice = {
  name: string;
  type: string;
  state: string;
  errorSummary: string;
  message: string;
};

export type StoragePoolDetailZfsSummary = {
  state: string;
  scan: string;
  errorSummary: string | null;
  devices: StoragePoolDetailZfsDevice[];
};

export const STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS: readonly {
  value: HistoryTimeRange;
  label: string;
}[] = [
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '14d', label: '14d' },
  { value: '30d', label: '30d' },
  { value: '90d', label: '90d' },
] as const;

export function getZfsScanTextClass(): string {
  return 'text-yellow-600 dark:text-yellow-400 italic';
}

export function getZfsErrorSummary(
  readErrors: number,
  writeErrors: number,
  checksumErrors: number,
): string {
  return `Errors: R:${readErrors} W:${writeErrors} C:${checksumErrors}`;
}

export function getZfsErrorTextClass(): string {
  return 'font-medium text-red-600 dark:text-red-400';
}

export function resolveStoragePoolDetailChartTarget(
  record: StorageRecord,
): StoragePoolDetailChartTarget {
  return {
    resourceType: (record.metricsTarget?.resourceType as HistoryChartResourceType) || 'storage',
    resourceId: resolveStorageRecordMetricResourceId(record),
  };
}

export function buildStoragePoolDetailConfigRows(
  record: StorageRecord,
): StoragePoolDetailConfigRow[] {
  const totalBytes = record.capacity.totalBytes || 0;
  const usedBytes = record.capacity.usedBytes || 0;
  const freeBytes =
    record.capacity.freeBytes ?? (totalBytes > 0 ? Math.max(totalBytes - usedBytes, 0) : 0);
  const content = getStorageRecordContent(record);
  const rows: StoragePoolDetailConfigRow[] = [
    { label: 'Node', value: getStorageRecordNodeLabel(record) },
    { label: 'Source', value: getStorageRecordPlatformLabel(record) },
    { label: 'Type', value: getStorageRecordType(record) },
    { label: 'Status', value: getStorageRecordStatus(record) },
    {
      label: 'Shared',
      value:
        getStorageRecordShared(record) === null
          ? '-'
          : getStorageRecordShared(record)
            ? 'Yes'
            : 'No',
    },
    { label: 'Used', value: totalBytes > 0 ? formatBytes(usedBytes) : 'n/a' },
    { label: 'Free', value: totalBytes > 0 ? formatBytes(freeBytes) : 'n/a' },
    { label: 'Total', value: totalBytes > 0 ? formatBytes(totalBytes) : 'n/a' },
    { label: 'Usage', value: formatPercent(getStorageRecordUsagePercent(record)) },
  ];

  if (content) {
    rows.splice(3, 0, { label: 'Content', value: content });
  }

  return rows;
}

const getRecordDetails = (record: StorageRecord): Record<string, unknown> =>
  (record.details || {}) as Record<string, unknown>;

const readRecordDetailString = (record: StorageRecord, key: string): string => {
  const value = getRecordDetails(record)[key];
  return typeof value === 'string' ? value.trim() : '';
};

const readRecordDetailNumber = (record: StorageRecord, key: string): number => {
  const value = getRecordDetails(record)[key];
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
};

const isUnraidStorageRecord = (record: StorageRecord): boolean => {
  const platform = readRecordDetailString(record, 'platform').toLowerCase();
  const type = getStorageRecordType(record).toLowerCase();
  return record.source.platform === 'unraid' || platform === 'unraid' || type.startsWith('unraid-');
};

const getUnraidStorageGroup = (record: StorageRecord): string => {
  if (!isUnraidStorageRecord(record)) return '';
  const type = getStorageRecordType(record).toLowerCase();
  const topology = readRecordDetailString(record, 'topology').toLowerCase();
  if (type === 'unraid-array' || topology === 'array') return 'unraid-array';
  if (type === 'unraid-cache-pool') return record.name.trim();
  return '';
};

const pluralize = (count: number, singular: string, plural = `${singular}s`): string =>
  `${count} ${count === 1 ? singular : plural}`;

const titleizeStorageState = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

export function buildStoragePoolDetailTopologyRows(
  record: StorageRecord,
  linkedDisks: StoragePoolDetailLinkedDisk[],
): StoragePoolDetailTopologyRow[] {
  if (!isUnraidStorageRecord(record)) return [];

  const type = getStorageRecordType(record).toLowerCase();
  const arrayState = readRecordDetailString(record, 'arrayState');
  const syncAction = readRecordDetailString(record, 'syncAction');
  const syncProgress = readRecordDetailNumber(record, 'syncProgress');
  const parityDisks = linkedDisks.filter((disk) => disk.role.toLowerCase() === 'parity').length;
  const dataDisks = linkedDisks.filter((disk) => disk.role.toLowerCase() === 'data').length;
  const cacheDisks = linkedDisks.filter((disk) => disk.role.toLowerCase() === 'cache').length;
  const errorCount = linkedDisks.reduce((sum, disk) => sum + disk.errorCount, 0);
  const spunDownCount = linkedDisks.filter((disk) => disk.spunDown).length;

  const rows: StoragePoolDetailTopologyRow[] = [];
  rows.push({ label: 'Kind', value: type === 'unraid-cache-pool' ? 'Cache pool' : 'Array' });
  if (arrayState) rows.push({ label: 'State', value: titleizeStorageState(arrayState) });
  if (type === 'unraid-array') {
    rows.push({
      label: 'Parity',
      value: parityDisks > 0 ? pluralize(parityDisks, 'disk') : 'None configured',
    });
    rows.push({ label: 'Data disks', value: pluralize(dataDisks, 'disk') });
  } else {
    rows.push({ label: 'Devices', value: pluralize(cacheDisks || linkedDisks.length, 'disk') });
  }
  if (spunDownCount > 0) {
    rows.push({ label: 'Spun down', value: pluralize(spunDownCount, 'disk') });
  }
  if (errorCount > 0) {
    rows.push({ label: 'Disk errors', value: errorCount.toLocaleString() });
  }
  if (syncAction) {
    rows.push({
      label: 'Sync',
      value: syncProgress > 0 ? `${syncAction} (${Math.round(syncProgress)}%)` : syncAction,
    });
  }
  return rows;
}

export function getZfsDeviceErrorSummary(
  readErrors: number,
  writeErrors: number,
  checksumErrors: number,
): string {
  if (readErrors <= 0 && writeErrors <= 0 && checksumErrors <= 0) return '';
  return `${readErrors}R/${writeErrors}W/${checksumErrors}C errors`;
}

export function getZfsDeviceStateTextClass(state: string): string {
  const normalized = (state || '').trim().toUpperCase();
  if (!normalized || normalized === 'ONLINE') return 'text-muted';
  if (normalized === 'DEGRADED') return 'font-medium text-amber-700 dark:text-amber-300';
  return 'font-medium text-red-700 dark:text-red-300';
}

export function buildStoragePoolDetailZfsSummary(
  record: StorageRecord,
): StoragePoolDetailZfsSummary | null {
  const pool = getStorageRecordZfsPool(record);
  if (!pool) return null;

  const hasErrors = pool.readErrors > 0 || pool.writeErrors > 0 || pool.checksumErrors > 0;

  return {
    state: pool.state,
    scan: pool.scan && pool.scan !== 'none' ? pool.scan : '',
    errorSummary: hasErrors
      ? getZfsErrorSummary(pool.readErrors, pool.writeErrors, pool.checksumErrors)
      : null,
    devices: (pool.devices || []).map((device) => ({
      name: device.name,
      type: device.type || '',
      state: device.state || '',
      errorSummary: getZfsDeviceErrorSummary(
        device.readErrors || 0,
        device.writeErrors || 0,
        device.checksumErrors || 0,
      ),
      message: (device.message || '').trim(),
    })),
  };
}

const readPhysicalDisk = (disk: Resource): NonNullable<Resource['physicalDisk']> => {
  const platformData = (disk.platformData || {}) as Record<string, unknown>;
  const platformDisk = platformData.physicalDisk as
    | NonNullable<Resource['physicalDisk']>
    | undefined;
  return (disk.physicalDisk || platformDisk || {}) as NonNullable<Resource['physicalDisk']>;
};

const readDiskDevPath = (disk: Resource): string => {
  const physicalDisk = readPhysicalDisk(disk);
  return typeof physicalDisk.devPath === 'string' ? physicalDisk.devPath : '';
};

const readDiskModel = (disk: Resource): string => {
  const physicalDisk = readPhysicalDisk(disk);
  return typeof physicalDisk.model === 'string' && physicalDisk.model.trim()
    ? physicalDisk.model
    : 'Unknown';
};

const readDiskTemperature = (disk: Resource): number => {
  const physicalDisk = readPhysicalDisk(disk);
  return typeof physicalDisk.temperature === 'number' ? physicalDisk.temperature : 0;
};

const readDiskType = (disk: Resource): string => {
  const physicalDisk = readPhysicalDisk(disk);
  return typeof physicalDisk.diskType === 'string' ? physicalDisk.diskType.trim() : '';
};

const readDiskHasIssue = (disk: Resource): boolean => {
  const physicalDisk = readPhysicalDisk(disk);
  const smart = physicalDisk.smart as Record<string, unknown> | undefined;
  const reallocated = smart?.reallocatedSectors;
  const errorCount = typeof physicalDisk.errorCount === 'number' ? physicalDisk.errorCount : 0;
  return errorCount > 0 || (typeof reallocated === 'number' && reallocated > 0);
};

const readDiskStorageGroup = (disk: Resource): string => {
  const physicalDisk = readPhysicalDisk(disk);
  return typeof physicalDisk.storageGroup === 'string' ? physicalDisk.storageGroup.trim() : '';
};

const readDiskRole = (disk: Resource): string => {
  const physicalDisk = readPhysicalDisk(disk);
  return typeof physicalDisk.storageRole === 'string' ? physicalDisk.storageRole.trim() : '';
};

const readDiskState = (disk: Resource): string => {
  const physicalDisk = readPhysicalDisk(disk);
  return typeof physicalDisk.storageState === 'string' ? physicalDisk.storageState.trim() : '';
};

const readDiskSizeLabel = (disk: Resource): string => {
  const physicalDisk = readPhysicalDisk(disk);
  return typeof physicalDisk.sizeBytes === 'number' && physicalDisk.sizeBytes > 0
    ? formatBytes(physicalDisk.sizeBytes)
    : '';
};

const readDiskSpunDown = (disk: Resource): boolean => readPhysicalDisk(disk).spunDown === true;

const readDiskErrorCount = (disk: Resource): number => {
  const count = readPhysicalDisk(disk).errorCount;
  return typeof count === 'number' && Number.isFinite(count) ? count : 0;
};

const readDiskIOLabel = (disk: Resource): string => {
  const physicalDisk = readPhysicalDisk(disk);
  const readCount = typeof physicalDisk.readCount === 'number' ? physicalDisk.readCount : 0;
  const writeCount = typeof physicalDisk.writeCount === 'number' ? physicalDisk.writeCount : 0;
  if (readCount <= 0 && writeCount <= 0) return '';
  return `R ${readCount.toLocaleString()} / W ${writeCount.toLocaleString()}`;
};

const diskRoleRank = (role: string): number => {
  switch (role.toLowerCase()) {
    case 'parity':
      return 0;
    case 'data':
      return 1;
    case 'cache':
      return 2;
    default:
      return 3;
  }
};

export function getStoragePoolLinkedDisks(
  record: StorageRecord,
  physicalDisks: Resource[],
): StoragePoolDetailLinkedDisk[] {
  const pool = getStorageRecordZfsPool(record);
  const unraidStorageGroup = getUnraidStorageGroup(record).toLowerCase();
  const directParentIds = new Set(
    [record.id, record.refs?.resourceId].filter((value): value is string => Boolean(value)),
  );

  return physicalDisks
    .filter((disk) => {
      const devPath = readDiskDevPath(disk);
      const group = readDiskStorageGroup(disk).toLowerCase();
      if (directParentIds.has(disk.parentId || '')) return true;
      if (pool?.devices?.length && devPath) {
        return pool.devices.some((device) => devPath.endsWith(device.name));
      }
      return Boolean(unraidStorageGroup && group === unraidStorageGroup);
    })
    .map((disk) => ({
      id: disk.id,
      devPath: readDiskDevPath(disk) || disk.name,
      model: readDiskModel(disk),
      role: readDiskRole(disk),
      state: readDiskState(disk),
      sizeLabel: readDiskSizeLabel(disk),
      diskType: readDiskType(disk),
      temperature: readDiskTemperature(disk),
      hasIssue: readDiskHasIssue(disk),
      spunDown: readDiskSpunDown(disk),
      errorCount: readDiskErrorCount(disk),
      ioLabel: readDiskIOLabel(disk),
    }))
    .sort((left, right) => {
      const roleDelta = diskRoleRank(left.role) - diskRoleRank(right.role);
      if (roleDelta !== 0) return roleDelta;
      return left.devPath.localeCompare(right.devPath);
    });
}
