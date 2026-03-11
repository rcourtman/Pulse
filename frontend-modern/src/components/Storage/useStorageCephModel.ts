import { Accessor, createMemo } from 'solid-js';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { CephCluster } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  getCephClusterKeyFromStorageRecord,
  getCephPoolsText,
  getCephSummaryText,
} from '@/features/storageBackups/cephRecordPresentation';
import {
  buildCephClusterLookup,
  buildExplicitCephClusters,
  deriveCephClustersFromStorageRecords,
  resolveCephClusterForStorageRecord,
  summarizeCephClusters,
} from '@/features/storageBackups/cephSummaryPresentation';

type UseStorageCephModelOptions = {
  records: Accessor<StorageRecord[]>;
  cephResources: Accessor<Resource[]>;
};

export const useStorageCephModel = (options: UseStorageCephModelOptions) => {
  const explicitCephClusters = createMemo<CephCluster[]>(() =>
    buildExplicitCephClusters(options.cephResources() || []),
  );

  const visibleCephClusters = createMemo<CephCluster[]>(() => {
    const explicit = explicitCephClusters();
    if (explicit.length > 0) return explicit;
    return deriveCephClustersFromStorageRecords(options.records());
  });

  const cephClusterByKey = createMemo<Record<string, CephCluster>>(() =>
    buildCephClusterLookup(visibleCephClusters()),
  );

  const cephSummaryStats = createMemo(() => summarizeCephClusters(visibleCephClusters()));

  const resolveCephCluster = (record: StorageRecord): CephCluster | null => {
    return resolveCephClusterForStorageRecord(record, cephClusterByKey());
  };

  return {
    visibleCephClusters,
    cephSummaryStats,
    resolveCephCluster,
    getCephSummaryText,
    getCephPoolsText,
  };
};
