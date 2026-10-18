export interface ResourceInventoryOwnership {
  ownerLabel: string;
  providerOwned: boolean;
  retirementDescription: string;
}

const PROXMOX_OWNERSHIP: ResourceInventoryOwnership = {
  ownerLabel: 'Proxmox',
  providerOwned: true,
  retirementDescription:
    'Proxmox owns this inventory record. Retiring it stops Pulse attention and automation without deleting it from Proxmox. Restore active monitoring here at any time.',
};

const KUBERNETES_OWNERSHIP: ResourceInventoryOwnership = {
  ownerLabel: 'Kubernetes',
  providerOwned: true,
  retirementDescription:
    'Kubernetes owns this object. Retiring it stops Pulse attention and automation without deleting the object from the cluster.',
};

const VMWARE_OWNERSHIP: ResourceInventoryOwnership = {
  ownerLabel: 'VMware vSphere',
  providerOwned: true,
  retirementDescription:
    'vSphere owns this inventory record. Retiring it changes Pulse monitoring only and does not delete anything from vCenter.',
};

const TRUENAS_OWNERSHIP: ResourceInventoryOwnership = {
  ownerLabel: 'TrueNAS',
  providerOwned: true,
  retirementDescription:
    'TrueNAS owns this inventory record. Retiring it changes Pulse monitoring only and does not delete anything from TrueNAS.',
};

const CONTAINER_RUNTIME_OWNERSHIP: ResourceInventoryOwnership = {
  ownerLabel: 'container runtime',
  providerOwned: true,
  retirementDescription:
    'The container runtime owns this inventory record. Retiring it changes Pulse monitoring only and does not remove the runtime object.',
};

const AGENT_OWNERSHIP: ResourceInventoryOwnership = {
  ownerLabel: 'Pulse agent',
  providerOwned: false,
  retirementDescription:
    'Retiring this machine stops Pulse attention and automation while keeping its history. Agent removal remains available from Machines.',
};

const SOURCE_SYSTEM_OWNERSHIP: ResourceInventoryOwnership = {
  ownerLabel: 'source system',
  providerOwned: true,
  retirementDescription:
    'The source system owns this inventory record. Retiring it changes Pulse monitoring only and preserves the resource history.',
};

// Callers pass either a platform family ('proxmox', 'vmware') or a canonical
// platform key ('proxmox-pve', 'vmware-vsphere'); both name the same owner.
function ownershipForPlatform(platform: string): ResourceInventoryOwnership | undefined {
  if (platform === 'proxmox' || platform.startsWith('proxmox-')) return PROXMOX_OWNERSHIP;
  if (platform === 'vmware' || platform.startsWith('vmware-')) return VMWARE_OWNERSHIP;
  if (platform === 'kubernetes') return KUBERNETES_OWNERSHIP;
  if (platform === 'docker') return CONTAINER_RUNTIME_OWNERSHIP;
  if (platform === 'truenas') return TRUENAS_OWNERSHIP;
  return undefined;
}

// The agent copy speaks about the agent's own machine and points at agent
// removal. Everything else an agent reports (libvirt VMs, Unraid arrays) is a
// record the host keeps, so it gets the neutral source-system copy instead.
function ownershipForAgentPlatform(type: string): ResourceInventoryOwnership {
  return type === '' || type === 'agent' ? AGENT_OWNERSHIP : SOURCE_SYSTEM_OWNERSHIP;
}

// Fallback for callers that know only the resource type. Resource types such
// as 'vm' are shared across platforms, so this guess must never outrank an
// explicit platform.
function ownershipForResourceType(type: string): ResourceInventoryOwnership {
  if (['vm', 'system-container', 'oci-container', 'pbs', 'pmg'].includes(type)) {
    return PROXMOX_OWNERSHIP;
  }
  if (type.startsWith('k8s-') || type === 'pod') return KUBERNETES_OWNERSHIP;
  if (type.startsWith('docker-') || type === 'app-container') return CONTAINER_RUNTIME_OWNERSHIP;
  if (type === 'agent') return AGENT_OWNERSHIP;
  return SOURCE_SYSTEM_OWNERSHIP;
}

export function describeResourceInventoryOwnership(
  resourceType?: string,
  platformType?: string,
): ResourceInventoryOwnership {
  const type = (resourceType || '').toLowerCase();
  const platform = (platformType || '').trim().toLowerCase();

  if (platform === 'agent') return ownershipForAgentPlatform(type);
  return ownershipForPlatform(platform) ?? ownershipForResourceType(type);
}
