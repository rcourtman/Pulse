export interface ResourceInventoryOwnership {
  ownerLabel: string;
  providerOwned: boolean;
  retirementDescription: string;
}

export function describeResourceInventoryOwnership(
  resourceType?: string,
  platformType?: string,
): ResourceInventoryOwnership {
  const type = (resourceType || '').toLowerCase();
  const platform = (platformType || '').toLowerCase();

  if (
    platform === 'proxmox' ||
    ['vm', 'system-container', 'oci-container', 'pbs', 'pmg'].includes(type)
  ) {
    return {
      ownerLabel: 'Proxmox',
      providerOwned: true,
      retirementDescription:
        'Proxmox owns this inventory record. Retiring it stops Pulse attention and automation without deleting it from Proxmox. Restore active monitoring here at any time.',
    };
  }
  if (platform === 'kubernetes' || type.startsWith('k8s-') || type === 'pod') {
    return {
      ownerLabel: 'Kubernetes',
      providerOwned: true,
      retirementDescription:
        'Kubernetes owns this object. Retiring it stops Pulse attention and automation without deleting the object from the cluster.',
    };
  }
  if (platform === 'vmware') {
    return {
      ownerLabel: 'VMware vSphere',
      providerOwned: true,
      retirementDescription:
        'vSphere owns this inventory record. Retiring it changes Pulse monitoring only and does not delete anything from vCenter.',
    };
  }
  if (platform === 'docker' || type.startsWith('docker-') || type === 'app-container') {
    return {
      ownerLabel: 'container runtime',
      providerOwned: true,
      retirementDescription:
        'The container runtime owns this inventory record. Retiring it changes Pulse monitoring only and does not remove the runtime object.',
    };
  }
  if (type === 'agent' || platform === 'agent') {
    return {
      ownerLabel: 'Pulse agent',
      providerOwned: false,
      retirementDescription:
        'Retiring this machine stops Pulse attention and automation while keeping its history. Agent removal remains available from Machines.',
    };
  }
  return {
    ownerLabel: 'source system',
    providerOwned: true,
    retirementDescription:
      'The source system owns this inventory record. Retiring it changes Pulse monitoring only and preserves the resource history.',
  };
}
