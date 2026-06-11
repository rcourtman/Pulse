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

// Signature of the cluster-internal pool rows synthesized from Ceph cluster
// telemetry (models.StorageFromCephPool): type "ceph" homed on the "cluster"
// pseudo-node, as opposed to the rbd/cephfs storage entries PVE mounts.
export const isCephClusterPoolStorageRecord = (record: StorageRecord): boolean => {
  if (getStorageRecordType(record) !== 'ceph') return false;
  const details = (record.details || {}) as Record<string, unknown>;
  return details.node === 'cluster';
};

const STORAGE_HEALTH_SEVERITY_RANK: Record<string, number> = {
  critical: 4,
  offline: 3,
  warning: 2,
  unknown: 1,
  healthy: 0,
};

const storageHealthRank = (health: string): number => STORAGE_HEALTH_SEVERITY_RANK[health] ?? 0;

const getRecordPoolDetail = (record: StorageRecord): string => {
  const pool = ((record.details || {}) as Record<string, unknown>).pool;
  return typeof pool === 'string' ? pool.trim() : '';
};

const getRecordStatusDetail = (record: StorageRecord): string => {
  const status = ((record.details || {}) as Record<string, unknown>).status;
  return typeof status === 'string' ? status.trim() : '';
};

/**
 * Collapse cluster-internal Ceph pool rows into the PVE storage rows that
 * mount them, so the same storage does not appear twice with conflicting
 * usage accounting (raw pool bytes vs PVE-visible bytes). The pool row's
 * health is lifted onto the surviving row when it is worse, so a degraded
 * cluster stays visible on the storage table. Pool rows with no mounting
 * sibling are kept: hiding them would hide the only capacity row for
 * clusters monitored without PVE storage entries.
 */
export const consolidateCephClusterPoolRecords = (records: StorageRecord[]): StorageRecord[] => {
  const poolRecords = records.filter(isCephClusterPoolStorageRecord);
  if (poolRecords.length === 0) return records;

  const consumedPoolIds = new Set<string>();
  const liftBySiblingId = new Map<string, StorageRecord>();

  for (const pool of poolRecords) {
    const sibling = records.find(
      (candidate) =>
        candidate !== pool &&
        !isCephClusterPoolStorageRecord(candidate) &&
        isCephStorageRecord(candidate) &&
        (getRecordPoolDetail(candidate) === pool.name || candidate.name === pool.name),
    );
    if (!sibling) continue;
    consumedPoolIds.add(pool.id);
    const existing = liftBySiblingId.get(sibling.id);
    if (!existing || storageHealthRank(pool.health) > storageHealthRank(existing.health)) {
      liftBySiblingId.set(sibling.id, pool);
    }
  }

  if (consumedPoolIds.size === 0) return records;

  return records
    .filter((record) => !consumedPoolIds.has(record.id))
    .map((record) => {
      const pool = liftBySiblingId.get(record.id);
      if (!pool) return record;
      if (storageHealthRank(pool.health) <= storageHealthRank(record.health)) return record;
      const poolStatus = getRecordStatusDetail(pool) || pool.statusLabel || pool.health;
      return {
        ...record,
        health: pool.health,
        statusLabel: poolStatus,
        issueSummary:
          pool.issueSummary?.trim() || `Ceph reports pool ${pool.name} ${poolStatus}`,
        details: { ...(record.details || {}), status: poolStatus },
      };
    });
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
