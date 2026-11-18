import type { Resource, ResourceType } from '@/types/resource';

/**
 * Which network each machine is on, from the addresses and attachments Pulse
 * already collects. Three sources, in order of how much they prove:
 *
 * - a vSphere port group a VM's adapter is connected to;
 * - a subnet an interface reports with its prefix (`192.168.10.5/24`);
 * - an address reported without a prefix, which joins the reported subnet
 *   that contains it, or else the other addresses sharing its first three
 *   numbers. That last grouping is a range, not a proven subnet, and is
 *   labelled as one (`10.0.30.x`).
 *
 * Pulse does not see switches, VLAN tags or firewall rules, so two networks
 * drawn apart here are separate address ranges, not a tested isolation.
 */

export type MapSegmentKind = 'subnet' | 'range' | 'portgroup' | 'other';

export interface MapSegment {
  id: string;
  label: string;
  kind: MapSegmentKind;
  memberCount: number;
  /** Index into the map's network colours; -1 draws in the neutral tone. */
  colorIndex: number;
}

export interface MapAttachment {
  segmentId: string;
  address?: string;
}

export interface MapNetworks {
  segments: MapSegment[];
  attachments: Map<string, MapAttachment[]>;
}

export const OTHER_SEGMENT_ID = 'other';
export const MAP_NETWORK_COLOR_COUNT = 6;

// Machines and guests have a place on a network. Containers and pods live on
// networks private to their host, which the host's own card draws.
const ADDRESSED_TYPES = new Set<ResourceType>([
  'agent',
  'docker-host',
  'vm',
  'system-container',
  'oci-container',
  'jail',
  'pbs',
  'pmg',
  'k8s-node',
  'network-endpoint',
]);

// Interfaces every machine has to itself: loopback, and the bridges and
// virtual pairs a container runtime or hypervisor creates locally.
const HOST_LOCAL_INTERFACE =
  /^(lo|docker\d*|podman\d*|br-[0-9a-f]{12}|veth.*|virbr\d*|cni.*|flannel.*|cali.*|kube-ipvs\d*)$/i;

// macvlan and ipvlan put containers straight onto the network the host itself
// is on, so their subnet is a real network and not a range private to one host.
const LAN_DOCKER_DRIVERS = new Set(['macvlan', 'ipvlan']);

const bareAddress = (raw: string): string => raw.trim().split('/')[0];

const IPV4 = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})(?:\/(\d{1,2}))?$/;

interface ParsedAddress {
  ip: string;
  value: number;
  prefix?: number;
}

export const parseMapAddress = (raw: string | undefined | null): ParsedAddress | null => {
  const match = IPV4.exec((raw ?? '').trim());
  if (!match) return null;
  const octets = match.slice(1, 5).map(Number);
  if (octets.some((octet) => octet > 255)) return null;
  // Loopback, link-local and the unspecified address place nothing.
  if (octets[0] === 127 || octets[0] === 0 || (octets[0] === 169 && octets[1] === 254)) return null;
  const prefix = match[5] === undefined ? undefined : Number(match[5]);
  const value = ((octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3]) >>> 0;
  return {
    ip: octets.join('.'),
    value,
    prefix: prefix !== undefined && prefix >= 8 && prefix <= 30 ? prefix : undefined,
  };
};

const maskFor = (prefix: number): number => (0xffffffff << (32 - prefix)) >>> 0;

const formatIp = (value: number): string =>
  [value >>> 24, (value >>> 16) & 255, (value >>> 8) & 255, value & 255].join('.');

interface Subnet {
  network: number;
  prefix: number;
}

const subnetOf = (address: ParsedAddress, prefix: number): Subnet => ({
  network: (address.value & maskFor(prefix)) >>> 0,
  prefix,
});
const subnetId = (subnet: Subnet): string => `net:${formatIp(subnet.network)}/${subnet.prefix}`;
const subnetContains = (subnet: Subnet, address: ParsedAddress): boolean =>
  (address.value & maskFor(subnet.prefix)) >>> 0 === subnet.network;

// A Docker host reports its interfaces and core count on the wire
// (`DockerData` in internal/unifiedresources), but the shared frontend type
// does not list them yet.
export type DockerHostWireFields = {
  cpus?: number;
  networkInterfaces?: Array<{ name?: string; addresses?: string[] }>;
};
export const dockerHostWire = (resource: Resource): DockerHostWireFields =>
  (resource.docker ?? {}) as DockerHostWireFields;

const interfaceAddresses = (resource: Resource): string[] => {
  const out: string[] = [];
  // The identity list repeats every address without saying which interface
  // holds it, so the ones seen on a host-local interface are dropped by value.
  const hostLocal = new Set<string>();
  const lists = [
    resource.agent?.networkInterfaces,
    dockerHostWire(resource).networkInterfaces,
    resource.proxmox?.networkInterfaces,
  ];
  for (const list of lists) {
    for (const nic of list ?? []) {
      if (HOST_LOCAL_INTERFACE.test((nic.name ?? '').trim())) {
        for (const address of nic.addresses ?? []) hostLocal.add(bareAddress(address));
        continue;
      }
      out.push(...(nic.addresses ?? []));
    }
  }
  out.push(...(resource.identity?.ips ?? []));
  out.push(...(resource.vmware?.guestIpAddresses ?? []));
  if (resource.type === 'network-endpoint') out.push(resource.availability?.address ?? '');
  return out.filter((raw) => !hostLocal.has(bareAddress(raw)));
};

const dockerSubnets = (resources: readonly Resource[]): Subnet[] => {
  const out: Subnet[] = [];
  for (const resource of resources) {
    if (resource.type !== 'docker-network') continue;
    if (LAN_DOCKER_DRIVERS.has((resource.docker?.driver ?? '').trim().toLowerCase())) continue;
    for (const entry of resource.docker?.subnets ?? []) {
      const parsed = parseMapAddress(entry.subnet);
      if (parsed?.prefix) out.push(subnetOf(parsed, parsed.prefix));
    }
  }
  return out;
};

const vspherePortGroups = (resource: Resource): string[] => {
  if (resource.platformType !== 'vmware-vsphere' || resource.type !== 'vm') return [];
  const connection = resource.vmware?.connectionId ?? '';
  const names = new Set<string>();
  for (const adapter of resource.vmware?.networkAdapters ?? []) {
    const name = (adapter.networkName ?? '').trim();
    if (name) names.add(`pg:${connection}:${name}`);
  }
  return [...names];
};

export function resolveMapNetworks(resources: readonly Resource[]): MapNetworks {
  const containerRanges = dockerSubnets(resources);
  const parsedById = new Map<string, ParsedAddress[]>();
  const known = new Map<string, Subnet>();
  const labels = new Map<string, { label: string; kind: MapSegmentKind }>();

  for (const resource of resources) {
    if (!ADDRESSED_TYPES.has(resource.type)) continue;
    const byIp = new Map<string, ParsedAddress>();
    for (const raw of interfaceAddresses(resource)) {
      const parsed = parseMapAddress(raw);
      if (!parsed) continue;
      // A host's gateway address on one of its own container networks.
      if (containerRanges.some((range) => subnetContains(range, parsed))) continue;
      const existing = byIp.get(parsed.ip);
      if (!existing || (existing.prefix === undefined && parsed.prefix !== undefined)) {
        byIp.set(parsed.ip, parsed);
      }
    }
    const addresses = [...byIp.values()];
    parsedById.set(resource.id, addresses);
    for (const address of addresses) {
      if (address.prefix === undefined) continue;
      const subnet = subnetOf(address, address.prefix);
      known.set(subnetId(subnet), subnet);
    }
  }
  // The narrowest reported subnet that holds an address wins.
  const knownByWidth = [...known.values()].sort((a, b) => b.prefix - a.prefix);

  const attachments = new Map<string, MapAttachment[]>();
  const members = new Map<string, Set<string>>();
  const attach = (resourceId: string, segmentId: string, address?: string) => {
    const list = attachments.get(resourceId) ?? [];
    if (!list.some((entry) => entry.segmentId === segmentId)) list.push({ segmentId, address });
    attachments.set(resourceId, list);
    const set = members.get(segmentId) ?? new Set<string>();
    set.add(resourceId);
    members.set(segmentId, set);
  };

  for (const resource of resources) {
    const portGroups = vspherePortGroups(resource);
    const addresses = parsedById.get(resource.id) ?? [];
    for (const id of portGroups) {
      labels.set(id, { label: id.split(':').slice(2).join(':'), kind: 'portgroup' });
      attach(resource.id, id, addresses[0]?.ip);
    }
    // A port group is where the adapter is plugged in; it stands for the
    // guest's addresses rather than being drawn beside them.
    if (portGroups.length > 0) continue;
    for (const address of addresses) {
      const subnet =
        address.prefix !== undefined
          ? subnetOf(address, address.prefix)
          : knownByWidth.find((candidate) => subnetContains(candidate, address));
      if (subnet) {
        const id = subnetId(subnet);
        labels.set(id, { label: id.slice(4), kind: 'subnet' });
        attach(resource.id, id, address.ip);
        continue;
      }
      const range = address.ip.split('.').slice(0, 3).join('.');
      const id = `range:${range}`;
      labels.set(id, { label: `${range}.x`, kind: 'range' });
      attach(resource.id, id, address.ip);
    }
  }

  // A range only one machine uses says nothing about what it shares a network
  // with. Those addresses are kept on the row, under one neutral heading.
  for (const [segmentId, set] of [...members]) {
    if (labels.get(segmentId)?.kind !== 'range' || set.size > 1) continue;
    members.delete(segmentId);
    for (const resourceId of set) {
      const list = attachments.get(resourceId) ?? [];
      const moved = list.find((entry) => entry.segmentId === segmentId);
      const rest = list.filter((entry) => entry.segmentId !== segmentId);
      if (moved && !rest.some((entry) => entry.segmentId === OTHER_SEGMENT_ID)) {
        rest.push({ segmentId: OTHER_SEGMENT_ID, address: moved.address });
        const other = members.get(OTHER_SEGMENT_ID) ?? new Set<string>();
        other.add(resourceId);
        members.set(OTHER_SEGMENT_ID, other);
      }
      attachments.set(resourceId, rest);
    }
  }
  labels.set(OTHER_SEGMENT_ID, { label: 'Other addresses', kind: 'other' });

  const collator = new Intl.Collator(undefined, { numeric: true });
  const segments = [...members.entries()]
    .map(([id, set]) => ({ id, ...labels.get(id)!, memberCount: set.size }))
    .sort(
      (a, b) =>
        Number(a.kind === 'other') - Number(b.kind === 'other') ||
        b.memberCount - a.memberCount ||
        collator.compare(a.label, b.label),
    )
    .map((segment, index) => ({
      ...segment,
      colorIndex: segment.kind === 'other' ? -1 : index % MAP_NETWORK_COLOR_COUNT,
    }));

  return { segments, attachments };
}
