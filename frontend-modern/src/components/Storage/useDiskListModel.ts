import { createMemo } from 'solid-js';
import type { Resource } from '@/types/resource';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import {
  buildPhysicalDiskPresentationDataMap,
  buildPhysicalDiskGroupFilterOptions,
  buildPhysicalDiskRoleFilterOptions,
  extractPhysicalDiskPresentationData,
  filterAndSortPhysicalDisks,
  getPhysicalDiskHostLabel,
  type PhysicalDiskAlertResourceIdResolver,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { getProxmoxData } from '@/utils/resourcePlatformData';
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

const getNodeDisplayName = (node: Resource | undefined): string => {
  if (!node) return '';
  const proxmoxDisplay = node.proxmox?.nodeDisplayName?.trim();
  if (proxmoxDisplay) return proxmoxDisplay;
  return (node.displayName || node.name || '').trim();
};

const getDiskProxmoxDisplayName = (disk: Resource): string => {
  const direct = disk.proxmox?.nodeDisplayName?.trim();
  if (direct) return direct;
  const platformProxmox = getProxmoxData(disk) as { nodeDisplayName?: string } | undefined;
  return platformProxmox?.nodeDisplayName?.trim() || '';
};

export const useDiskListModel = (options: UseDiskListModelOptions) => {
  const hasPVENodes = createMemo(() => options.nodes().length > 0);
  const { getDiskTemperatureThresholds } = useAlertsActivation();

  const nodeById = createMemo(() => {
    const map = new Map<string, Resource>();
    for (const node of options.nodes()) {
      map.set(node.id, node);
    }
    return map;
  });

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

  const getRawDiskData = (disk: Resource): PhysicalDiskPresentationData =>
    diskDataById().get(disk.id) ??
    extractPhysicalDiskPresentationData(
      disk,
      getDiskTemperatureThresholds,
      options.getDiskAlertResourceIds?.(disk),
    );

  const getDiskHostLabel = (disk: Resource): string => {
    const proxmoxDisplay = getDiskProxmoxDisplayName(disk);
    if (proxmoxDisplay) return proxmoxDisplay;

    const parentNode = disk.parentId ? nodeById().get(disk.parentId) : undefined;
    const parentNodeDisplay = getNodeDisplayName(parentNode);
    if (parentNodeDisplay) return parentNodeDisplay;

    const matchedNode = options.nodes().find((node) => {
      const proxmox = getProxmoxData(node);
      return matchesPhysicalDiskNode(disk, {
        id: node.id,
        name: proxmox?.nodeName || node.name,
        instance: proxmox?.instance,
      });
    });
    const matchedDisplay = getNodeDisplayName(matchedNode);
    if (matchedDisplay) return matchedDisplay;

    const parentName = disk.parentName?.trim();
    if (parentName) return parentName;

    return getPhysicalDiskHostLabel(getRawDiskData(disk), disk);
  };

  const getDiskHostTitle = (disk: Resource): string => {
    const label = getDiskHostLabel(disk);
    const rawHost = getPhysicalDiskHostLabel(getRawDiskData(disk), disk);
    if (label && rawHost && rawHost !== label) {
      return `${label} (${rawHost})`;
    }
    return label;
  };

  const getDiskData = (disk: Resource): PhysicalDiskPresentationData => {
    const raw = getRawDiskData(disk);
    const hostLabel = getDiskHostLabel(disk);
    if (!hostLabel || hostLabel === raw.node) return raw;
    return { ...raw, node: hostLabel };
  };

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

  const selectedNodeName = createMemo(
    () => getNodeDisplayName(selectedNodeResource() ?? undefined) || null,
  );
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
    getDiskHostLabel,
    getDiskHostTitle,
    roleFilterOptions,
    groupFilterOptions,
    filteredDisks,
    selectedNodeName,
    toggleSelectedDisk,
  };
};
