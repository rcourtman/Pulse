import type { Resource } from '@/types/resource';
import { resolvePhysicalDiskMetricResourceId as resolveCanonicalPhysicalDiskMetricResourceId } from '@/features/storageBackups/storageMetricsIdentity';
import { getLinkedAgentId, getProxmoxData } from '@/utils/resourcePlatformData';

type DiskPlatformData = {
  proxmox?: {
    nodeName?: string;
    instance?: string;
  };
  physicalDisk?: {
    serial?: string;
    wwn?: string;
  };
};

export interface PhysicalDiskNodeIdentity {
  node: string;
  instance: string;
}

const normalize = (value: string | null | undefined): string => value?.trim().toLowerCase() || '';

export const getPhysicalDiskNodeIdentity = (resource: Resource): PhysicalDiskNodeIdentity => {
  const platformData = ((resource.platformData as DiskPlatformData | undefined) ||
    {}) as DiskPlatformData;
  const proxmox = platformData.proxmox || {};
  const node =
    proxmox.nodeName || resource.identity?.hostname || resource.canonicalIdentity?.hostname || '';

  return {
    node: node.trim(),
    instance: (proxmox.instance || '').trim(),
  };
};

export const matchesPhysicalDiskNode = (
  disk: Resource,
  target: { id?: string | null; name?: string | null; instance?: string | null },
): boolean => {
  if (target.id && disk.parentId === target.id) return true;

  const diskIdentity = getPhysicalDiskNodeIdentity(disk);
  const diskNode = normalize(diskIdentity.node);
  const targetNode = normalize(target.name);
  if (!diskNode || !targetNode || diskNode !== targetNode) {
    return false;
  }

  const diskInstance = normalize(diskIdentity.instance);
  const targetInstance = normalize(target.instance);

  // Host-agent SMART disks carry no Proxmox scope, so the disk side has no
  // instance to discriminate on and the matching node name is the only
  // evidence available. Requiring an instance here dropped agent-reported
  // disks on Proxmox nodes out of the node filter, grouping, and metric
  // target resolution entirely (#1487, #1516).
  if (!diskInstance) {
    return true;
  }

  // A Proxmox-scoped disk only belongs to a node in the same instance. A node
  // with no instance is not that scope, so it must not absorb the disk.
  return Boolean(targetInstance && diskInstance === targetInstance);
};

export const resolvePhysicalDiskMetricResourceId = (
  disk: Resource,
  nodes: Resource[],
  devPath: string,
): string | null => {
  if (disk.metricsTarget?.resourceId) {
    return disk.metricsTarget.resourceId;
  }

  const node = nodes.find((candidate) =>
    matchesPhysicalDiskNode(disk, {
      id: candidate.id,
      name: candidate.name,
      instance: getProxmoxData(candidate)?.instance,
    }),
  );
  const agentId = node ? getLinkedAgentId(node) : undefined;

  if (!agentId) return null;
  const deviceName = devPath.replace('/dev/', '');
  return `${agentId}:${deviceName}`;
};

export const resolvePhysicalDiskHistoryResourceId = (disk: Resource): string | null => {
  return resolveCanonicalPhysicalDiskMetricResourceId(disk);
};
