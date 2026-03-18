import type { HistoryTimeRange, ResourceType as HistoryChartResourceType } from '@/api/charts';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getStorageRecordContent,
  getStorageRecordNodeLabel,
  getStorageRecordShared,
  getStorageRecordStatus,
  getStorageRecordType,
  getStorageRecordUsagePercent,
  getStorageRecordZfsPool,
} from '@/features/storageBackups/recordPresentation';
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
  temperature: number;
  hasIssue: boolean;
};

export type StoragePoolDetailChartTarget = {
  resourceType: HistoryChartResourceType;
  resourceId: string | null;
};

export type StoragePoolDetailZfsSummary = {
  state: string;
  scan: string;
  errorSummary: string | null;
};

export const STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS: readonly {
  value: HistoryTimeRange;
  label: string;
}[] = [
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
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
    resourceId: record.metricsTarget?.resourceId || record.refs?.resourceId || record.id,
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
    { label: 'Type', value: getStorageRecordType(record) },
    { label: 'Status', value: getStorageRecordStatus(record) },
    { label: 'Shared', value: getStorageRecordShared(record) === null ? '-' : getStorageRecordShared(record) ? 'Yes' : 'No' },
    { label: 'Used', value: totalBytes > 0 ? formatBytes(usedBytes) : 'n/a' },
    { label: 'Free', value: totalBytes > 0 ? formatBytes(freeBytes) : 'n/a' },
    { label: 'Total', value: totalBytes > 0 ? formatBytes(totalBytes) : 'n/a' },
    { label: 'Usage', value: formatPercent(getStorageRecordUsagePercent(record)) },
  ];

  if (content) {
    rows.splice(2, 0, { label: 'Content', value: content });
  }

  return rows;
}

export function buildStoragePoolDetailZfsSummary(record: StorageRecord): StoragePoolDetailZfsSummary | null {
  const pool = getStorageRecordZfsPool(record);
  if (!pool) return null;

  const hasErrors = pool.readErrors > 0 || pool.writeErrors > 0 || pool.checksumErrors > 0;

  return {
    state: pool.state,
    scan: pool.scan && pool.scan !== 'none' ? pool.scan : '',
    errorSummary: hasErrors
      ? getZfsErrorSummary(pool.readErrors, pool.writeErrors, pool.checksumErrors)
      : null,
  };
}

const readDiskDevPath = (disk: Resource): string => {
  const physicalDisk = (disk.platformData as Record<string, unknown>)?.physicalDisk as
    | Record<string, unknown>
    | undefined;
  return typeof physicalDisk?.devPath === 'string' ? physicalDisk.devPath : '';
};

const readDiskModel = (disk: Resource): string => {
  const physicalDisk = (disk.platformData as Record<string, unknown>)?.physicalDisk as
    | Record<string, unknown>
    | undefined;
  return typeof physicalDisk?.model === 'string' && physicalDisk.model.trim()
    ? physicalDisk.model
    : 'Unknown';
};

const readDiskTemperature = (disk: Resource): number => {
  const physicalDisk = (disk.platformData as Record<string, unknown>)?.physicalDisk as
    | Record<string, unknown>
    | undefined;
  return typeof physicalDisk?.temperature === 'number' ? physicalDisk.temperature : 0;
};

const readDiskHasIssue = (disk: Resource): boolean => {
  const physicalDisk = (disk.platformData as Record<string, unknown>)?.physicalDisk as
    | Record<string, unknown>
    | undefined;
  const smart = physicalDisk?.smart as Record<string, unknown> | undefined;
  const reallocated = smart?.reallocatedSectors;
  return typeof reallocated === 'number' && reallocated > 0;
};

export function getStoragePoolLinkedDisks(
  record: StorageRecord,
  physicalDisks: Resource[],
): StoragePoolDetailLinkedDisk[] {
  const pool = getStorageRecordZfsPool(record);
  if (!pool?.devices?.length) return [];

  return physicalDisks
    .filter((disk) => {
      const devPath = readDiskDevPath(disk);
      return devPath && pool.devices.some((device) => devPath.endsWith(device.name));
    })
    .map((disk) => ({
      id: disk.id,
      devPath: readDiskDevPath(disk) || disk.name,
      model: readDiskModel(disk),
      temperature: readDiskTemperature(disk),
      hasIssue: readDiskHasIssue(disk),
    }));
}
