import { Accessor, createMemo } from 'solid-js';
import type { Resource } from '@/types/resource';
import type { StorageHealthFilter, StorageRecord } from '@/features/storageBackups/models';
import type { StoragePageNodeOption } from './storagePageState';
import {
  buildPhysicalDiskPresentationDataMap,
  extractPhysicalDiskPresentationData,
  getPhysicalDiskNormalizedHealth,
  matchesPhysicalDiskFilterState,
  matchesPhysicalDiskHealthFilter,
} from '@/features/storageBackups/diskPresentation';
import { matchesPhysicalDiskNode } from './diskResourceUtils';

type UseStoragePageSummaryOptions = {
  filteredRecords: Accessor<StorageRecord[]>;
  search: Accessor<string>;
  sourceFilter: Accessor<string>;
  healthFilter: Accessor<StorageHealthFilter>;
  diskRoleFilter: Accessor<string>;
  diskGroupFilter: Accessor<string>;
  selectedNodeId: Accessor<string>;
  nodeOptions: Accessor<StoragePageNodeOption[]>;
  physicalDisks: Accessor<Resource[]>;
};

const POOL_DEGRADED_HEALTHS = new Set(['warning', 'critical', 'offline']);

export const useStoragePageSummary = (options: UseStoragePageSummaryOptions) => {
  const poolCount = createMemo(() => options.filteredRecords().length);
  const diskDataById = createMemo(() =>
    buildPhysicalDiskPresentationDataMap(options.physicalDisks()),
  );
  const getDiskData = (disk: Resource) =>
    diskDataById().get(disk.id) ?? extractPhysicalDiskPresentationData(disk);

  const filteredPhysicalDisks = createMemo(() => {
    const nodeId = options.selectedNodeId();
    const selectedNode =
      nodeId && nodeId !== 'all'
        ? (options.nodeOptions().find((option) => option.id === nodeId) ?? null)
        : null;

    return options.physicalDisks().filter((disk) => {
      if (
        selectedNode &&
        !matchesPhysicalDiskNode(disk, {
          id: selectedNode.id,
          name: selectedNode.label,
          instance: selectedNode.instance,
        })
      ) {
        return false;
      }
      return matchesPhysicalDiskFilterState(disk, getDiskData(disk), {
        sourceFilter: options.sourceFilter(),
        healthFilter: options.healthFilter(),
        roleFilter: options.diskRoleFilter(),
        groupFilter: options.diskGroupFilter(),
        searchTerm: options.search(),
      });
    });
  });

  const diskCount = createMemo(() => filteredPhysicalDisks().length);

  const poolsDegraded = createMemo(
    () =>
      options.filteredRecords().filter((record) => POOL_DEGRADED_HEALTHS.has(record.health)).length,
  );
  const disksFailing = createMemo(() => {
    return filteredPhysicalDisks().filter((disk) =>
      matchesPhysicalDiskHealthFilter(
        getPhysicalDiskNormalizedHealth(disk, getDiskData(disk)),
        'attention',
      ),
    ).length;
  });

  return {
    poolCount,
    diskCount,
    poolsDegraded,
    disksFailing,
  };
};
