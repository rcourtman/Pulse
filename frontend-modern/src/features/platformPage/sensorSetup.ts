import type { Resource } from '@/types/resource';

export type OutdatedSensorSetupNode = {
  id: string;
  // The label the Proxmox nodes table shows for the node's row outside its
  // narrowest layout, which shows the Proxmox node name instead.
  name: string;
  // The Proxmox node name, which the setup script is re-run on, when `name`
  // does not already contain it as a whole word or a domain's host part.
  nodeName?: string;
};

const trimmed = (value: string | undefined): string => (value ?? '').trim();

const getProxmoxNodeName = (node: Resource): string =>
  trimmed(node.proxmox?.nodeName) || trimmed(node.proxmox?.node);

// Mirrors the nodes table's row label: the resource name, which carries the
// operator-set display name when one exists and the Proxmox node name
// otherwise, falling back to the id.
const getNodeLabel = (node: Resource): string => trimmed(node.name) || node.id;

// Folds only ASCII letters, the alphabet of Proxmox node names, so a
// non-ASCII lookalike (the Kelvin sign for K) cannot pass for one.
const foldAscii = (value: string): string => value.replace(/[A-Z]/g, (c) => c.toLowerCase());

// True when the label already contains the node name as a whole word, ignoring
// ASCII case (`PVE1`, `Rack 4 (pve3)`), or as the host part of a domain name
// (`pve3.lan`). Words are runs of ASCII letters, digits, dots, hyphens and
// underscores, so a label that differs inside the name (`PVE3` or `PVE 3` for
// `pve-3`, `pve3-backup` for `pve3`) keeps the exact name. Stricter than the
// guest table's decorative node group header (`hasAlternateDisplayName`),
// because the notice names the host the user must act on.
const labelShowsNodeName = (label: string, nodeName: string): boolean => {
  const target = foldAscii(nodeName);
  return foldAscii(label)
    .split(/[^a-z0-9._-]+/)
    .some((word) => word === target || word.startsWith(`${target}.`));
};

const toOutdatedSensorSetupNode = (node: Resource): OutdatedSensorSetupNode => {
  const name = getNodeLabel(node);
  const nodeName = getProxmoxNodeName(node);
  if (!nodeName || labelShowsNodeName(name, nodeName)) {
    return { id: node.id, name };
  }
  return { id: node.id, name, nodeName };
};

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
    .map(toOutdatedSensorSetupNode)
    .sort((a, b) => a.name.localeCompare(b.name));
}
