import type { Resource } from '@/types/resource';

export type OutdatedSensorSetupNode = {
  id: string;
  name: string;
};

const getNodeName = (node: Resource): string =>
  (node.proxmox?.node || node.proxmox?.nodeName || node.name || '').trim();

// Lists PVE nodes whose SSH temperature monitoring still runs the pre-rc.6
// setup (authorized_keys locked to `sensors -j`). Such a payload parses fine
// and delivers CPU/NVMe temps, but SMART (SATA/SAS) disk temperatures can
// never arrive, so they silently stay blank. The registry derives the verdict
// on the node (`proxmox.sensorSetupOutdated`) from the node's last collection,
// its legacy payload and the disks under it, so every Proxmox tab that lists
// nodes can show the notice without loading the disk inventory.
export function collectOutdatedSensorSetupNodes(nodes: Resource[]): OutdatedSensorSetupNode[] {
  return nodes
    .filter((node) => node.proxmox?.sensorSetupOutdated === true)
    .map((node) => ({ id: node.id, name: getNodeName(node) }))
    .sort((a, b) => a.name.localeCompare(b.name));
}
