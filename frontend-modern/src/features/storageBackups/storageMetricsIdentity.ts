import type { Resource } from '@/types/resource';
import type { StorageRecord } from './models';

type DiskPlatformData = {
  physicalDisk?: {
    serial?: string;
    wwn?: string;
  };
};

const trim = (value: string | null | undefined): string => value?.trim() || '';
const normalize = (value: string | null | undefined): string => value?.trim().toLowerCase() || '';

export const resolveStorageRecordMetricResourceId = (record: StorageRecord): string =>
  trim(record.metricsTarget?.resourceId) || trim(record.refs?.resourceId) || trim(record.id);

export const resolvePhysicalDiskMetricResourceId = (disk: Resource): string => {
  const metricsTargetType = normalize(disk.metricsTarget?.resourceType);
  const metricsTargetId = trim(disk.metricsTarget?.resourceId);
  if (metricsTargetId && (!metricsTargetType || metricsTargetType === 'disk')) {
    return metricsTargetId;
  }

  const platformData = ((disk.platformData as DiskPlatformData | undefined) || {}) as DiskPlatformData;
  const serial = trim(disk.physicalDisk?.serial || platformData.physicalDisk?.serial);
  if (serial) return serial;

  const wwn = trim(disk.physicalDisk?.wwn || platformData.physicalDisk?.wwn);
  return wwn || trim(disk.id);
};
