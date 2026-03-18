import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import type { ZFSPool } from '@/types/api';
import { getSourcePlatformLabel } from '@/utils/sourcePlatforms';

export interface StorageRecordStats {
  totalBytes: number;
  usedBytes: number;
  usagePercent: number;
  byHealth: Record<NormalizedHealth, number>;
}

const getRecordDetails = (record: StorageRecord): Record<string, unknown> =>
  (record.details || {}) as Record<string, unknown>;

const getRecordStringDetail = (record: StorageRecord, key: string): string => {
  const value = getRecordDetails(record)[key];
  return typeof value === 'string' ? value : '';
};

const getRecordStringArrayDetail = (record: StorageRecord, key: string): string[] => {
  const value = getRecordDetails(record)[key];
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : [];
};

export const getStorageRecordNodeHints = (record: StorageRecord): string[] => {
  const details = getRecordDetails(record);
  const detailNode = typeof details.node === 'string' ? details.node : '';
  const detailParent = typeof details.parentId === 'string' ? details.parentId : '';
  const detailParentName = typeof details.parentName === 'string' ? details.parentName : '';
  const locationRoot = record.location.label.split('/')[0]?.trim() || '';
  return [
    detailNode,
    detailParent,
    detailParentName,
    ...getRecordStringArrayDetail(record, 'nodeHints'),
    locationRoot,
    record.location.label,
    record.refs?.platformEntityId,
  ]
    .map((value) => (value ? value.toLowerCase().trim() : ''))
    .filter((value) => value.length > 0);
};

export const getStorageRecordType = (record: StorageRecord): string =>
  getRecordStringDetail(record, 'type') || record.category || 'other';

export const getStorageRecordContent = (record: StorageRecord): string =>
  getRecordStringDetail(record, 'content');

export const getStorageRecordStatus = (record: StorageRecord): string => {
  const status = getRecordStringDetail(record, 'status');
  if (status) return status;
  if (record.health === 'healthy') return 'available';
  if (record.health === 'warning') return 'degraded';
  if (record.health === 'offline') return 'offline';
  if (record.health === 'critical') return 'critical';
  return 'unknown';
};

export const getStorageRecordPlatformLabel = (record: StorageRecord): string =>
  record.platformLabel?.trim() || getSourcePlatformLabel(record.source.platform);

export const getStorageRecordNodeLabel = (record: StorageRecord): string => {
  const parentName = getRecordStringDetail(record, 'parentName');
  if (parentName.trim()) return parentName;
  const node = getRecordStringDetail(record, 'node');
  if (node.trim()) return node;
  return record.location.label || 'unassigned';
};

export const getStorageRecordHostLabel = (record: StorageRecord): string =>
  record.hostLabel?.trim() || getStorageRecordNodeLabel(record);

export const getStorageRecordTopologyLabel = (record: StorageRecord): string =>
  record.topologyLabel?.trim() || getStorageRecordType(record);

export const getStorageRecordProtectionLabel = (record: StorageRecord): string =>
  record.protectionLabel?.trim() || 'Healthy';

export const getStorageRecordIssueLabel = (record: StorageRecord): string =>
  record.issueLabel?.trim() || 'Healthy';

export const getStorageRecordIssueSummary = (record: StorageRecord): string =>
  record.issueSummary?.trim() || record.issueLabel?.trim() || '';

export const getStorageRecordImpactSummary = (record: StorageRecord): string =>
  record.impactSummary?.trim() || 'No dependent resources';

export const getStorageRecordActionSummary = (record: StorageRecord): string =>
  record.actionSummary?.trim() || 'Monitor';

export const getStorageRecordShared = (record: StorageRecord): boolean | null => {
  const shared = getRecordDetails(record).shared;
  return typeof shared === 'boolean' ? shared : null;
};

export const getStorageRecordUsagePercent = (record: StorageRecord): number => {
  if (
    typeof record.capacity.usagePercent === 'number' &&
    Number.isFinite(record.capacity.usagePercent)
  ) {
    return record.capacity.usagePercent;
  }
  const total = record.capacity.totalBytes || 0;
  const used = record.capacity.usedBytes || 0;
  if (total <= 0) return 0;
  return (used / total) * 100;
};

const toZfsPool = (value: unknown): ZFSPool | null => {
  if (!value || typeof value !== 'object') return null;
  const candidate = value as Partial<ZFSPool>;
  if (typeof candidate.state !== 'string' || !Array.isArray(candidate.devices)) return null;
  return candidate as ZFSPool;
};

export const getStorageRecordZfsPool = (record: StorageRecord): ZFSPool | null =>
  toZfsPool(getRecordDetails(record).zfsPool);

export const getStorageRecordStats = (items: StorageRecord[]): StorageRecordStats => {
  const seenShared = new Set<string>();
  const totals = items.reduce(
    (acc, record) => {
      acc.byHealth[record.health] = (acc.byHealth[record.health] || 0) + 1;

      const isShared = getStorageRecordShared(record) === true;
      if (isShared) {
        const sharedKey = `${record.source.platform}|${record.name}`;
        if (seenShared.has(sharedKey)) {
          return acc;
        }
        seenShared.add(sharedKey);
      }

      acc.total += record.capacity.totalBytes || 0;
      acc.used += record.capacity.usedBytes || 0;
      return acc;
    },
    {
      total: 0,
      used: 0,
      byHealth: {
        healthy: 0,
        warning: 0,
        critical: 0,
        offline: 0,
        unknown: 0,
      } as Record<NormalizedHealth, number>,
    },
  );

  return {
    totalBytes: totals.total,
    usedBytes: totals.used,
    usagePercent: totals.total > 0 ? (totals.used / totals.total) * 100 : 0,
    byHealth: totals.byHealth,
  };
};
