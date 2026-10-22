import { guestOverrideIdCandidates } from '@/features/alerts/guestOverrideIdentity';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { getAlertsForUnifiedResource } from '@/utils/alerts';
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
  const machine = findPhysicalDiskReportingMachine(disk, owners);
  return machine ? getAgentMachineAlertOverrideKeys(machine) : [];
};

/**
 * The machine whose Pulse agent reports a disk: the agent machine the disk is
 * parented to or, for an Unraid array or cache disk parented to its pool, the
 * pool's machine. Null when no agent reports it or its machine is not loaded.
 */
export const findPhysicalDiskReportingMachine = (
  disk: Resource,
  owners: ReadonlyMap<string, Resource>,
): Resource | null => {
  let parentId = disk.parentId?.trim();
  for (let hops = 0; parentId && hops < 2; hops += 1) {
    const parent = owners.get(parentId);
    if (!parent) break;
    if (getExplicitAgentIdFromResource(parent)) return parent;
    if (parent.type !== 'storage') break;
    parentId = parent.parentId?.trim();
  }
  return null;
};

// The backend's sanitizeHostComponent: lower case, runs of anything but
// letters and digits become one hyphen, trimmed of hyphens.
const sanitizeHostAlertComponent = (value: string): string => {
  const sanitized = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return sanitized || 'unknown';
};

const ALERT_LEVEL_RANK: Record<Alert['level'], number> = { critical: 0, warning: 1, info: 2 };

/**
 * The alert keys a physical disk answers to beyond its unified resource keys:
 * the path-shaped key Proxmox disk health alerts keep ("<instance>:<node>:
 * disk:<device>"), which the registry records among the disk's aliases, and
 * the SMART temperature alert of the agent that reports it
 * ("agent:<host>/disk_temp:<device>").
 */
export const getPhysicalDiskExtraAlertKeys = (
  disk: Resource,
  owners: ReadonlyMap<string, Resource>,
  /** The reporting agent when the caller knows it (a guest's own agent). */
  reportingAgentId?: string,
): string[] => {
  const keys = new Set<string>(
    (disk.canonicalIdentity?.aliases ?? [])
      .map((alias) => alias.trim())
      .filter((alias) => alias.includes(':disk:')),
  );
  const machine = reportingAgentId?.trim()
    ? undefined
    : findPhysicalDiskReportingMachine(disk, owners);
  const agentId =
    reportingAgentId?.trim() || (machine ? getExplicitAgentIdFromResource(machine)?.trim() : '');
  const devPath = disk.physicalDisk?.devPath?.trim() ?? '';
  if (agentId && devPath) {
    const prefix = `${agentId.startsWith('agent:') ? agentId : `agent:${agentId}`}/disk_temp:`;
    // The agent names a disk by its block device ("sda"); accept the full
    // path too so either spelling reaches the same disk.
    for (const device of [devPath, devPath.replace(/^\/dev\//, '')]) {
      keys.add(prefix + sanitizeHostAlertComponent(device));
    }
  }
  return [...keys];
};

/**
 * Open alerts on one physical disk, most severe first: those keyed on the
 * disk's own unified identity, its Proxmox disk alert key and its agent's
 * SMART temperature key for this device.
 */
export const getPhysicalDiskAlerts = (
  disk: Resource,
  owners: ReadonlyMap<string, Resource>,
  activeAlerts: Record<string, Alert>,
  alertsEnabled: boolean,
  reportingAgentId?: string,
): Alert[] => {
  if (!alertsEnabled) return [];
  const own = getAlertsForUnifiedResource(disk, activeAlerts, alertsEnabled);
  const extraKeys = new Set(getPhysicalDiskExtraAlertKeys(disk, owners, reportingAgentId));
  const ownIds = new Set(own.map((alert) => alert.id));
  const extra = Object.values(activeAlerts).filter(
    (alert) => !ownIds.has(alert.id) && extraKeys.has(alert.resourceId),
  );
  return [...own, ...extra].sort(
    (a, b) =>
      (ALERT_LEVEL_RANK[a.level] ?? 3) - (ALERT_LEVEL_RANK[b.level] ?? 3) ||
      Date.parse(a.startTime) - Date.parse(b.startTime),
  );
};
