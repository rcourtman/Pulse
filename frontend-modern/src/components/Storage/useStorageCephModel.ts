import { Accessor, createMemo } from 'solid-js';
import {
  isCephType,
} from '@/features/storageBackups/storageDomain';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { CephCluster } from '@/types/api';
import type { Resource } from '@/types/resource';
import { formatBytes, formatPercent } from '@/utils/format';
import {
  getRecordNodeLabel,
  getRecordType,
  getRecordUsagePercent,
} from './useStorageModel';

type UseStorageCephModelOptions = {
  records: Accessor<StorageRecord[]>;
  cephResources: Accessor<Resource[]>;
};

export const isCephRecord = (record: StorageRecord): boolean => {
  if (isCephType(getRecordType(record))) return true;
  return record.capabilities.includes('replication') && record.source.platform.includes('proxmox');
};

export const getCephClusterKeyFromRecord = (record: StorageRecord): string => {
  const details = (record.details || {}) as Record<string, unknown>;
  const parent = typeof details.parentId === 'string' ? details.parentId : '';
  return record.refs?.platformEntityId || parent || record.location.label || record.source.platform;
};

export const useStorageCephModel = (options: UseStorageCephModelOptions) => {
  const explicitCephClusters = createMemo<CephCluster[]>(() => {
    const resources = options.cephResources() || [];
    if (!resources.length) return [];

    return resources.map((r) => {
      const cephMeta = (r.platformData as any)?.ceph || {};
      return {
        id: r.id,
        instance: (r.platformData as any)?.proxmox?.instance || r.platformId || '',
        name: r.name,
        fsid: cephMeta.fsid,
        health: cephMeta.healthStatus || 'HEALTH_UNKNOWN',
        healthMessage: cephMeta.healthMessage || '',
        totalBytes: r.disk?.total || 0,
        usedBytes: r.disk?.used || 0,
        availableBytes: r.disk?.free || 0,
        usagePercent: r.disk?.current || 0,
        numMons: cephMeta.numMons || 0,
        numMgrs: cephMeta.numMgrs || 0,
        numOsds: cephMeta.numOsds || 0,
        numOsdsUp: cephMeta.numOsdsUp || 0,
        numOsdsIn: cephMeta.numOsdsIn || 0,
        numPGs: cephMeta.numPGs || 0,
        pools: cephMeta.pools?.map((p: any) => ({
          id: 0,
          name: p.name || '',
          storedBytes: p.storedBytes || 0,
          availableBytes: p.availableBytes || 0,
          objects: p.objects || 0,
          percentUsed: p.percentUsed || 0,
        })),
        services: cephMeta.services?.map((s: any) => ({
          type: s.type || '',
          running: s.running || 0,
          total: s.total || 0,
        })),
        lastUpdated: r.lastSeen || Date.now(),
      } as CephCluster;
    });
  });

  const visibleCephClusters = createMemo<CephCluster[]>(() => {
    const explicit = explicitCephClusters();
    if (explicit.length > 0) return explicit;

    const summaryByKey = new Map<
      string,
      { total: number; used: number; available: number; records: number; nodes: Set<string> }
    >();

    options.records().forEach((record) => {
      if (!isCephRecord(record)) return;
      const key = getCephClusterKeyFromRecord(record);
      const current =
        summaryByKey.get(key) ||
        ({ total: 0, used: 0, available: 0, records: 0, nodes: new Set<string>() } as const);
      const totalBytes = Math.max(0, record.capacity.totalBytes || 0);
      const usedBytes = Math.max(0, record.capacity.usedBytes || 0);
      const freeBytes = Math.max(0, record.capacity.freeBytes ?? totalBytes - usedBytes);

      summaryByKey.set(key, {
        total: current.total + totalBytes,
        used: current.used + usedBytes,
        available: current.available + freeBytes,
        records: current.records + 1,
        nodes: new Set([...current.nodes, getRecordNodeLabel(record)]),
      });
    });

    return Array.from(summaryByKey.entries()).map(([instance, item], index) => {
      const usagePercent = item.total > 0 ? (item.used / item.total) * 100 : 0;
      const numOsds = Math.max(1, item.records * 2);
      const numMons = Math.min(3, Math.max(1, item.nodes.size));
      return {
        id: `derived-ceph-${index}`,
        instance,
        name: `${instance} Ceph`,
        health: 'HEALTH_UNKNOWN',
        healthMessage: 'Derived from storage metrics - live Ceph telemetry unavailable.',
        totalBytes: item.total,
        usedBytes: item.used,
        availableBytes: item.available,
        usagePercent,
        numMons,
        numMgrs: numMons > 1 ? 2 : 1,
        numOsds,
        numOsdsUp: numOsds,
        numOsdsIn: numOsds,
        numPGs: Math.max(128, item.records * 64),
        pools: undefined,
        services: undefined,
        lastUpdated: Date.now(),
      } as CephCluster;
    });
  });

  const cephClusterByKey = createMemo<Record<string, CephCluster>>(() => {
    const map: Record<string, CephCluster> = {};
    visibleCephClusters().forEach((cluster) => {
      [cluster.instance, cluster.id, cluster.name].forEach((key) => {
        if (key) map[key] = cluster;
      });
    });
    return map;
  });

  const cephSummaryStats = createMemo(() => {
    const clusters = visibleCephClusters();
    const totals = clusters.reduce(
      (acc, cluster) => {
        acc.total += Math.max(0, cluster.totalBytes || 0);
        acc.used += Math.max(0, cluster.usedBytes || 0);
        acc.available += Math.max(0, cluster.availableBytes || 0);
        return acc;
      },
      { total: 0, used: 0, available: 0 },
    );

    const usagePercent = totals.total > 0 ? (totals.used / totals.total) * 100 : 0;
    return {
      clusters,
      totalBytes: totals.total,
      usedBytes: totals.used,
      availableBytes: totals.available,
      usagePercent,
    };
  });

  const resolveCephCluster = (record: StorageRecord): CephCluster | null => {
    const key = getCephClusterKeyFromRecord(record);
    return cephClusterByKey()[key] || null;
  };

  const getCephSummaryText = (record: StorageRecord, cluster: CephCluster | null): string => {
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

  const getCephPoolsText = (record: StorageRecord, cluster: CephCluster | null): string => {
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

    const percent = getRecordUsagePercent(record);
    return `${record.name}: ${formatPercent(percent)}`;
  };

  return {
    visibleCephClusters,
    cephSummaryStats,
    resolveCephCluster,
    getCephSummaryText,
    getCephPoolsText,
  };
};
