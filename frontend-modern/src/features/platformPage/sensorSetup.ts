import type { Resource } from '@/types/resource';
import { matchesPhysicalDiskNode } from '@/components/Storage/diskResourceUtils';

// Disk transports whose temperatures only arrive via SMART (the pulse-sensors
// wrapper). NVMe temps come from kernel hwmon and work even on legacy setups.
const SMART_ONLY_DISK_TYPES = new Set(['sata', 'sas']);

export type OutdatedSensorSetupNode = {
  id: string;
  name: string;
};

const getNodeName = (node: Resource): string =>
  (node.proxmox?.node || node.proxmox?.nodeName || node.name || '').trim();

const isSMARTOnlyDiskWithoutTemperature = (disk: Resource): boolean => {
  const meta = disk.physicalDisk;
  if (!meta) return false;
  const diskType = (meta.diskType || '').trim().toLowerCase();
  if (!SMART_ONLY_DISK_TYPES.has(diskType)) return false;
  return (meta.temperature ?? 0) <= 0;
};

// Flags PVE nodes whose SSH temperature monitoring still runs the pre-rc.6
// setup (authorized_keys locked to `sensors -j`). Such a payload parses fine
// and delivers CPU/NVMe temps, but SMART (SATA/SAS) disk temperatures can
// never arrive, so they silently stay blank. Data-gated three ways: the last
// collection succeeded, the payload was legacy-format, and the node actually
// has SATA/SAS disks sitting without a temperature. Nodes whose disk temps
// arrive elsewhere (e.g. a linked host agent) fall out via the last check.
export function collectOutdatedSensorSetupNodes(
  nodes: Resource[],
  physicalDisks: Resource[],
): OutdatedSensorSetupNode[] {
  if (!nodes.length || !physicalDisks.length) return [];

  return nodes
    .filter((node) => {
      const details = node.proxmox?.temperatureDetails;
      if (!details?.available || !details.legacySensorsFormat) return false;
      const nodeName = getNodeName(node);
      return physicalDisks.some(
        (disk) =>
          isSMARTOnlyDiskWithoutTemperature(disk) &&
          matchesPhysicalDiskNode(disk, {
            id: node.id,
            name: nodeName,
            instance: node.proxmox?.instance,
          }),
      );
    })
    .map((node) => ({ id: node.id, name: getNodeName(node) }))
    .sort((a, b) => a.name.localeCompare(b.name));
}
