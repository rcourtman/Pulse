import { guestOverrideIdCandidates } from '@/features/alerts/guestOverrideIdentity';
import type { Resource } from '@/types/resource';
import { getExplicitAgentIdFromResource } from '@/utils/agentResources';

/**
 * Indexes the resources a physical disk can reach the machine that reports it
 * through: machines running a Pulse agent, and the storage pools disks are
 * parented to.
 */
export const indexPhysicalDiskOwners = (resources: readonly Resource[]): Map<string, Resource> => {
  const owners = new Map<string, Resource>();
  for (const resource of resources) {
    if (resource.type === 'storage' || getExplicitAgentIdFromResource(resource)) {
      owners.set(resource.id, resource);
    }
  }
  return owners;
};

const uniqueKeys = (...keys: Array<string | undefined>): string[] => [
  ...new Set(keys.map((key) => key?.trim() ?? '').filter((key) => key.length > 0)),
];

/**
 * The alert override keys of a machine running a Pulse agent, in the order
 * CheckHost reads them for that agent: its own ID, the machine's canonical
 * resource ID (which the backend resolves the agent ID to), then the Proxmox
 * node or guest the agent is merged with. The first key with an override
 * decides.
 */
export const getAgentMachineAlertOverrideKeys = (machine: Resource): string[] => {
  const linkedGuestKeys =
    machine.type === 'vm' || machine.type === 'system-container'
      ? guestOverrideIdCandidates(machine)
      : [];
  return uniqueKeys(
    getExplicitAgentIdFromResource(machine),
    machine.id,
    machine.type === 'agent' ? machine.proxmox?.sourceId : undefined,
    ...linkedGuestKeys,
  );
};

/**
 * The alert override keys that judge a physical disk's heat: those of the
 * machine whose Pulse agent reports the disk, which its disk temperature
 * alerts are evaluated under. That is the agent machine the disk is parented
 * to or, for an Unraid array or cache disk parented to its pool, the pool's
 * machine. A Disk Temp override set on that machine therefore replaces the
 * per-type thresholds for every disk it reports. Empty when no agent reports
 * the disk or its machine is not loaded, which leaves the per-type thresholds.
 */
export const getPhysicalDiskAlertResourceIds = (
  disk: Resource,
  owners: ReadonlyMap<string, Resource>,
): string[] => {
  let parentId = disk.parentId?.trim();
  for (let hops = 0; parentId && hops < 2; hops += 1) {
    const parent = owners.get(parentId);
    if (!parent) break;
    if (getExplicitAgentIdFromResource(parent)) return getAgentMachineAlertOverrideKeys(parent);
    if (parent.type !== 'storage') break;
    parentId = parent.parentId?.trim();
  }
  return [];
};
