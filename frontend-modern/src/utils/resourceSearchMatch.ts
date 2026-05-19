import type { Resource } from '@/types/resource';

const collectSearchCandidates = (resource: Resource): string[] => {
  const raw: Array<string | undefined> = [
    resource.id,
    resource.name,
    resource.displayName,
    resource.parentName,
    resource.agent?.hostname,
    resource.identity?.hostname,
    resource.canonicalIdentity?.displayName,
    resource.canonicalIdentity?.hostname,
    resource.canonicalIdentity?.primaryId,
    resource.docker?.hostname,
    resource.kubernetes?.clusterName,
    resource.kubernetes?.nodeName,
    resource.kubernetes?.namespace,
    resource.vmware?.runtimeHostName,
    resource.vmware?.clusterName,
    resource.vmware?.datacenterName,
    resource.proxmox?.node,
    resource.proxmox?.nodeName,
    resource.proxmox?.clusterName,
    ...(resource.canonicalIdentity?.aliases ?? []),
    ...(resource.tags ?? []),
  ];
  return raw
    .filter((value): value is string => typeof value === 'string')
    .map((value) => value.trim().toLowerCase())
    .filter((value) => value.length > 0);
};

export function resourceMatchesSearch(resource: Resource, term: string | null | undefined): boolean {
  const needle = (term ?? '').trim().toLowerCase();
  if (!needle) return true;
  return collectSearchCandidates(resource).some((value) => value.includes(needle));
}
