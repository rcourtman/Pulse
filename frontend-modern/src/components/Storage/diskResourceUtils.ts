import type { Resource } from '@/types/resource';

type DiskPlatformData = {
  proxmox?: {
    nodeName?: string;
    instance?: string;
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
  if (!diskInstance || !targetInstance) {
    return true;
  }

  return diskInstance === targetInstance;
};
