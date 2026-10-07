import { createMemo } from 'solid-js';
import type { Resource } from '@/types/resource';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import {
  buildPhysicalDiskPresentationDataMap,
  buildPhysicalDiskGroupFilterOptions,
  buildPhysicalDiskRoleFilterOptions,
  extractPhysicalDiskPresentationData,
  filterAndSortPhysicalDisks,
  type PhysicalDiskAlertResourceIdResolver,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { matchesPhysicalDiskNode } from './diskResourceUtils';

type UseDiskListModelOptions = {
  disks: () => Resource[];
  getDiskAlertResourceIds?: PhysicalDiskAlertResourceIdResolver;
  nodes: () => Resource[];
  selectedNode: () => string | null;
  sourceFilter?: () => string;
  healthFilter?: () => StorageHealthFilter;
  roleFilter?: () => string;
  groupFilter?: () => string;
  searchTerm: () => string;
  selectedDiskId: () => string | null;
  setSelectedDiskId: (diskId: string | null) => void;
};

export const useDiskListModel = (options: UseDiskListModelOptions) => {
  const hasPVENodes = createMemo(() => options.nodes().length > 0);
  const { getDiskTemperatureThresholds } = useAlertsActivation();

  // Each disk's heat is judged by the user's alert disk temperature thresholds,
  // under the Disk Temp override of the machine that reports it, so the
  // presentation data recomputes when the alert configuration loads.
  const diskDataById = createMemo(() =>
    buildPhysicalDiskPresentationDataMap(
      options.disks(),
      getDiskTemperatureThresholds,
      options.getDiskAlertResourceIds,
    ),
  );
  const roleFilterOptions = createMemo(() => buildPhysicalDiskRoleFilterOptions(options.disks()));
  const groupFilterOptions = createMemo(() => buildPhysicalDiskGroupFilterOptions(options.disks()));

  const getDiskData = (disk: Resource): PhysicalDiskPresentationData =>
    diskDataById().get(disk.id) ??
    extractPhysicalDiskPresentationData(
      disk,
      getDiskTemperatureThresholds,
      options.getDiskAlertResourceIds?.(disk),
    );

  const selectedNodeResource = createMemo(
    () => options.nodes().find((node) => node.id === options.selectedNode()) ?? null,
  );

  const filteredDisks = createMemo(() =>
    filterAndSortPhysicalDisks(options.disks(), {
      selectedNode: selectedNodeResource(),
      sourceFilter: options.sourceFilter?.() ?? 'all',
      healthFilter: options.healthFilter?.() ?? 'all',
      roleFilter: options.roleFilter?.() ?? 'all',
      groupFilter: options.groupFilter?.() ?? 'all',
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
    roleFilterOptions,
    groupFilterOptions,
    filteredDisks,
    selectedNodeName,
    toggleSelectedDisk,
  };
};
