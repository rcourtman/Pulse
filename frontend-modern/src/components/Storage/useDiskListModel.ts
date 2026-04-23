import { createMemo } from 'solid-js';
import type { Resource } from '@/types/resource';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import {
  buildPhysicalDiskPresentationDataMap,
  extractPhysicalDiskPresentationData,
  filterAndSortPhysicalDisks,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';
import { matchesPhysicalDiskNode } from './diskResourceUtils';

type UseDiskListModelOptions = {
  disks: () => Resource[];
  nodes: () => Resource[];
  selectedNode: () => string | null;
  sourceFilter?: () => string;
  healthFilter?: () => StorageHealthFilter;
  searchTerm: () => string;
  selectedDiskId: () => string | null;
  setSelectedDiskId: (diskId: string | null) => void;
};

export const useDiskListModel = (options: UseDiskListModelOptions) => {
  const hasPVENodes = createMemo(() => options.nodes().length > 0);

  const diskDataById = createMemo(() => buildPhysicalDiskPresentationDataMap(options.disks()));

  const getDiskData = (disk: Resource): PhysicalDiskPresentationData =>
    diskDataById().get(disk.id) ?? extractPhysicalDiskPresentationData(disk);

  const selectedNodeResource = createMemo(
    () => options.nodes().find((node) => node.id === options.selectedNode()) ?? null,
  );

  const filteredDisks = createMemo(() =>
    filterAndSortPhysicalDisks(options.disks(), {
      selectedNode: selectedNodeResource(),
      sourceFilter: options.sourceFilter?.() ?? 'all',
      healthFilter: options.healthFilter?.() ?? 'all',
      searchTerm: options.searchTerm(),
      getDiskData,
      matchesNode: matchesPhysicalDiskNode,
    }),
  );

  const selectedNodeName = createMemo(() => selectedNodeResource()?.name || null);
  const selectedDisk = createMemo(
    () => options.disks().find((disk) => disk.id === options.selectedDiskId()) ?? null,
  );

  const toggleSelectedDisk = (disk: Resource) => {
    options.setSelectedDiskId(selectedDisk()?.id === disk.id ? null : disk.id);
  };

  return {
    selectedDisk,
    hasPVENodes,
    getDiskData,
    filteredDisks,
    selectedNodeName,
    toggleSelectedDisk,
  };
};
