import { Accessor, createMemo } from 'solid-js';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { StoragePageNodeOption } from './storagePageState';
import { countVisiblePhysicalDisksForNode } from './storagePageState';

type UseStoragePageSummaryOptions = {
  filteredRecords: Accessor<StorageRecord[]>;
  selectedNodeId: Accessor<string>;
  nodeOptions: Accessor<StoragePageNodeOption[]>;
  physicalDisks: Accessor<Resource[]>;
};

const POOL_DEGRADED_HEALTHS = new Set(['warning', 'critical', 'offline']);

export const useStoragePageSummary = (options: UseStoragePageSummaryOptions) => {
  const poolCount = createMemo(() => options.filteredRecords().length);
  const diskCount = createMemo(() =>
    countVisiblePhysicalDisksForNode(
      options.selectedNodeId(),
      options.nodeOptions(),
      options.physicalDisks(),
    ),
  );

  const poolsDegraded = createMemo(
    () => options.filteredRecords().filter((record) => POOL_DEGRADED_HEALTHS.has(record.health)).length,
  );
  const disksFailing = createMemo(() => {
    const nodeId = options.selectedNodeId();
    const isForSelectedNode = (disk: Resource): boolean => {
      if (!nodeId || nodeId === 'all') return true;
      const nodeOption = options.nodeOptions().find((option) => option.id === nodeId);
      if (!nodeOption) return true;
      const hostname = disk.identity?.hostname ?? disk.canonicalIdentity?.hostname;
      if (!hostname) return false;
      if (nodeOption.label === hostname) return true;
      return (nodeOption.aliases ?? []).includes(hostname);
    };
    return options.physicalDisks().filter((disk) => {
      if (!isForSelectedNode(disk)) return false;
      if (disk.status === 'degraded' || disk.status === 'offline') return true;
      return (disk.alerts?.length ?? 0) > 0;
    }).length;
  });

  return {
    poolCount,
    diskCount,
    poolsDegraded,
    disksFailing,
  };
};
