import { createMemo, createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
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
  searchTerm: () => string;
};

export const useDiskListModel = (options: UseDiskListModelOptions) => {
  const [selectedDisk, setSelectedDisk] = createSignal<Resource | null>(null);

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
      searchTerm: options.searchTerm(),
      getDiskData,
      matchesNode: matchesPhysicalDiskNode,
    }),
  );

  const selectedNodeName = createMemo(() => selectedNodeResource()?.name || null);

  const toggleSelectedDisk = (disk: Resource) => {
    setSelectedDisk((current) => (current?.id === disk.id ? null : disk));
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
