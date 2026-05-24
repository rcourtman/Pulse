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
    resource.docker?.image,
    resource.docker?.imageId,
    resource.docker?.volumeName,
    resource.docker?.networkId,
    resource.docker?.driver,
    resource.docker?.serviceName,
    resource.docker?.taskId,
    resource.kubernetes?.clusterName,
    resource.kubernetes?.nodeName,
    resource.kubernetes?.namespace,
    resource.kubernetes?.resourceKind,
    resource.kubernetes?.serviceType,
    resource.kubernetes?.serviceName,
    resource.kubernetes?.clusterIp,
    resource.kubernetes?.storageClass,
    resource.kubernetes?.provisioner,
    resource.kubernetes?.volumeBindingMode,
    resource.kubernetes?.addressType,
    resource.kubernetes?.phase,
    resource.kubernetes?.reason,
    resource.kubernetes?.involvedName,
    resource.kubernetes?.volumeName,
    resource.vmware?.runtimeHostName,
    resource.vmware?.clusterName,
    resource.vmware?.datacenterName,
    resource.proxmox?.node,
    resource.proxmox?.nodeName,
    resource.proxmox?.clusterName,
    ...(resource.docker?.repoTags ?? []),
    ...(resource.docker?.repoDigests ?? []),
    ...(resource.kubernetes?.externalIps ?? []),
    ...(resource.kubernetes?.hosts ?? []),
    ...(resource.kubernetes?.addresses ?? []),
    ...(resource.kubernetes?.policyTypes ?? []),
    ...(resource.kubernetes?.parameterKeys ?? []),
    ...(resource.kubernetes?.dataKeys ?? []),
    ...(resource.kubernetes?.binaryDataKeys ?? []),
    ...(resource.kubernetes?.imagePullSecrets ?? []),
    ...(resource.kubernetes?.accessModes ?? []),
    ...(resource.canonicalIdentity?.aliases ?? []),
    ...(resource.tags ?? []),
  ];
  return raw
    .filter((value): value is string => typeof value === 'string')
    .map((value) => value.trim().toLowerCase())
    .filter((value) => value.length > 0);
};

export function resourceMatchesSearch(
  resource: Resource,
  term: string | null | undefined,
): boolean {
  const needle = (term ?? '').trim().toLowerCase();
  if (!needle) return true;
  return collectSearchCandidates(resource).some((value) => value.includes(needle));
}
