import type { CephCluster } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from './models';
import {
  collectCephClusterNodes,
  getCephClusterKeyFromStorageRecord,
  isCephStorageRecord,
} from './cephRecordPresentation';

export interface CephSummaryStats {
  clusters: CephCluster[];
  totalBytes: number;
  usedBytes: number;
  availableBytes: number;
  usagePercent: number;
}

export const buildExplicitCephClusters = (resources: Resource[]): CephCluster[] => {
  if (!resources.length) return [];

  return resources.map((resource) => {
    const platformData = (resource.platformData as Record<string, any>) || {};
    const cephMeta = platformData.ceph || {};

    return {
      id: resource.id,
      instance: platformData.proxmox?.instance || resource.platformId || '',
      name: resource.name,
      fsid: cephMeta.fsid,
      health: cephMeta.healthStatus || 'HEALTH_UNKNOWN',
      healthMessage: cephMeta.healthMessage || '',
      totalBytes: resource.disk?.total || 0,
      usedBytes: resource.disk?.used || 0,
      availableBytes: resource.disk?.free || 0,
      usagePercent: resource.disk?.current || 0,
      numMons: cephMeta.numMons || 0,
      numMgrs: cephMeta.numMgrs || 0,
      numOsds: cephMeta.numOsds || 0,
      numOsdsUp: cephMeta.numOsdsUp || 0,
      numOsdsIn: cephMeta.numOsdsIn || 0,
      numPGs: cephMeta.numPGs || 0,
      pools: cephMeta.pools?.map((pool: any) => ({
        id: 0,
        name: pool.name || '',
        storedBytes: pool.storedBytes || 0,
        availableBytes: pool.availableBytes || 0,
        objects: pool.objects || 0,
        percentUsed: pool.percentUsed || 0,
      })),
      services: cephMeta.services?.map((service: any) => ({
        type: service.type || '',
        running: service.running || 0,
        total: service.total || 0,
      })),
      lastUpdated: resource.lastSeen || Date.now(),
    } as CephCluster;
  });
};

export const deriveCephClustersFromStorageRecords = (
  records: StorageRecord[],
): CephCluster[] => {
  const summaryByKey = new Map<
    string,
    { total: number; used: number; available: number; records: number; nodes: Set<string> }
  >();

  records.forEach((record) => {
    if (!isCephStorageRecord(record)) return;
    const key = getCephClusterKeyFromStorageRecord(record);
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
      nodes: collectCephClusterNodes(current.nodes, record),
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
};

export const summarizeCephClusters = (clusters: CephCluster[]): CephSummaryStats => {
  const totals = clusters.reduce(
    (acc, cluster) => {
      acc.total += Math.max(0, cluster.totalBytes || 0);
      acc.used += Math.max(0, cluster.usedBytes || 0);
      acc.available += Math.max(0, cluster.availableBytes || 0);
      return acc;
    },
    { total: 0, used: 0, available: 0 },
  );

  return {
    clusters,
    totalBytes: totals.total,
    usedBytes: totals.used,
    availableBytes: totals.available,
    usagePercent: totals.total > 0 ? (totals.used / totals.total) * 100 : 0,
  };
};

export const buildCephClusterLookup = (
  clusters: CephCluster[],
): Record<string, CephCluster> => {
  const map: Record<string, CephCluster> = {};
  clusters.forEach((cluster) => {
    [cluster.instance, cluster.id, cluster.name].forEach((key) => {
      if (key) map[key] = cluster;
    });
  });
  return map;
};

export const resolveCephClusterForStorageRecord = (
  record: StorageRecord,
  lookup: Record<string, CephCluster>,
): CephCluster | null => {
  const key = getCephClusterKeyFromStorageRecord(record);
  return lookup[key] || null;
};
