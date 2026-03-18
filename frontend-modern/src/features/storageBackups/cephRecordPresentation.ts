import type { CephCluster } from '@/types/api';
import { formatBytes, formatPercent } from '@/utils/format';
import type { StorageRecord } from './models';
import { isCephType } from './storageDomain';
import {
  getStorageRecordNodeLabel,
  getStorageRecordType,
  getStorageRecordUsagePercent,
} from './recordPresentation';

export const isCephStorageRecord = (record: StorageRecord): boolean => {
  if (isCephType(getStorageRecordType(record))) return true;
  return record.capabilities.includes('replication') && record.source.platform.includes('proxmox');
};

export const getCephClusterKeyFromStorageRecord = (record: StorageRecord): string => {
  const details = (record.details || {}) as Record<string, unknown>;
  const parent = typeof details.parentId === 'string' ? details.parentId : '';
  return record.refs?.platformEntityId || parent || record.location.label || record.source.platform;
};

export const getCephSummaryText = (
  record: StorageRecord,
  cluster: CephCluster | null,
): string => {
  if (cluster && Number.isFinite(cluster.totalBytes)) {
    const total = Math.max(0, cluster.totalBytes || 0);
    const used = Math.max(0, cluster.usedBytes || 0);
    const percent = total > 0 ? (used / total) * 100 : 0;
    const parts = [`${formatBytes(used)} / ${formatBytes(total)} (${formatPercent(percent)})`];
    if (Number.isFinite(cluster.numOsds) && Number.isFinite(cluster.numOsdsUp)) {
      parts.push(`OSDs ${cluster.numOsdsUp}/${cluster.numOsds}`);
    }
    if (Number.isFinite(cluster.numPGs) && cluster.numPGs > 0) {
      parts.push(`PGs ${cluster.numPGs.toLocaleString()}`);
    }
    return parts.join(' • ');
  }

  const total = Math.max(0, record.capacity.totalBytes || 0);
  const used = Math.max(0, record.capacity.usedBytes || 0);
  if (total <= 0) return '';
  const percent = (used / total) * 100;
  return `${formatBytes(used)} / ${formatBytes(total)} (${formatPercent(percent)})`;
};

export const getCephPoolsText = (record: StorageRecord, cluster: CephCluster | null): string => {
  if (cluster?.pools?.length) {
    return cluster.pools
      .slice(0, 2)
      .map((pool) => {
        const total = Math.max(1, pool.storedBytes + pool.availableBytes);
        const percent = (pool.storedBytes / total) * 100;
        return `${pool.name}: ${formatPercent(percent)}`;
      })
      .join(', ');
  }

  const percent = getStorageRecordUsagePercent(record);
  return `${record.name}: ${formatPercent(percent)}`;
};

export const collectCephClusterNodes = (
  nodes: Set<string>,
  record: StorageRecord,
): Set<string> => new Set([...nodes, getStorageRecordNodeLabel(record)]);
