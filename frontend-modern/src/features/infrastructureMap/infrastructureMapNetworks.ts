import type { Resource, ResourceType } from '@/types/resource';

/**
 * Which network each machine is on, from the addresses and attachments Pulse
 * already collects. Four sources, in order of how much they prove:
 *
 * - the Proxmox bridge and VLAN tag a guest's device is plugged into, from the
 *   guest's configuration, which places a guest that reports no address;
 * - a vSphere port group a VM's adapter is connected to;
 * - a subnet an interface reports with its prefix (`192.168.10.5/24`);
 * - an address reported without a prefix, which joins the reported subnet
 *   that contains it, or else the other addresses sharing its first three
 *   numbers. That last grouping is a range, not a proven subnet, and is
 *   labelled as one (`10.0.30.x`).
 *
 * A bridge also takes in the subnet or range its guests' addresses fall in,
 * so a VM on `vmbr0` and a NAS on the same LAN are one network instead of
 * looking separated.
 *
 * Pulse does not see switches, routers or firewall rules, so two networks
 * drawn apart here are separate bridges, VLANs or address ranges, not a tested
 * isolation.
 */

export type MapSegmentKind = 'subnet' | 'range' | 'portgroup' | 'bridge' | 'other';

export interface MapSegment {
  id: string;
  label: string;
  kind: MapSegmentKind;
  /**
   * What else is known about the network: the subnet a bridge carries, or the
   * cluster a bridge belongs to when two clusters use the same bridge name.
   */
  detail?: string;
  /**
   * The one Proxmox bridge and tag behind the network, when exactly one is.
   * It lets the diagram fold a bridge's VLANs together. A `vlan` of 0 is
   * untagged.
   */
  bridge?: MapBridgePlacement;
  memberCount: number;
  /** Index into the map's network colours; -1 draws in the neutral tone. */
  colorIndex: number;
}

export interface MapBridgePlacement {
  /** The cluster or standalone node the bridge belongs to. */
  scope: string;
  name: string;
  vlan: number;
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

const interfaceAddresses = (resource: Resource): string[] => {
  const out: string[] = [];
  // The identity list repeats every address without saying which interface
  // holds it, so the ones seen on a host-local interface are dropped by value.
  const hostLocal = new Set<string>();
  const lists = [
    resource.agent?.networkInterfaces,
    resource.docker?.networkInterfaces,
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

const PROXMOX_GUEST_TYPES = new Set<ResourceType>(['vm', 'system-container', 'oci-container']);

interface BridgeDevice {
  /** `bridge:<scope>:<bridge>:<vlan>`; a tag of 0 is untagged. */
  key: string;
  label: string;
  scope: string;
  bridge: string;
  vlan: number;
  addresses: string[];
}

// A bridge is one network only inside the cluster, or on the standalone node,
// that names it: `vmbr0` on two unrelated nodes is two bridges.
const proxmoxScope = (resource: Resource): string =>
  (resource.proxmox?.clusterName ?? '').trim() ||
  (resource.proxmox?.instance ?? '').trim() ||
  (resource.proxmox?.nodeName ?? resource.proxmox?.node ?? '').trim();

export const formatMapBridgeLabel = (bridge: string, vlan: number): string =>
  vlan > 0 ? `${bridge} · VLAN ${vlan}` : bridge;

/** The devices of a Proxmox guest whose bridge its configuration names. */
const proxmoxBridgeDevices = (resource: Resource): BridgeDevice[] => {
  if (resource.platformType !== 'proxmox-pve' || !PROXMOX_GUEST_TYPES.has(resource.type)) return [];
  const scope = proxmoxScope(resource);
  const byKey = new Map<string, BridgeDevice>();
  for (const nic of resource.proxmox?.networkInterfaces ?? []) {
    const bridge = (nic.bridge ?? '').trim();
    if (!bridge) continue;
    const tag = Number(nic.vlan);
    const vlan = Number.isInteger(tag) && tag >= 1 && tag <= 4094 ? tag : 0;
    const key = `bridge:${scope}:${bridge}:${vlan}`;
    const device: BridgeDevice = byKey.get(key) ?? {
      key,
      label: formatMapBridgeLabel(bridge, vlan),
      scope,
      bridge,
      vlan,
      addresses: [],
    };
    device.addresses.push(...(nic.addresses ?? []));
    byKey.set(key, device);
  }
  return [...byKey.values()];
};

export function resolveMapNetworks(resources: readonly Resource[]): MapNetworks {
  const containerRanges = dockerSubnets(resources);
  const parsedById = new Map<string, ParsedAddress[]>();
  const known = new Map<string, Subnet>();
  const labels = new Map<
    string,
    { label: string; kind: MapSegmentKind; detail?: string; bridge?: MapBridgePlacement }
  >();

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

  // The network an address alone points to: the subnet it reports or falls
  // inside, or else the range it shares its first three numbers with.
  const addressSegment = (
    address: ParsedAddress,
  ): { id: string; label: string; kind: MapSegmentKind } => {
    const subnet =
      address.prefix !== undefined
        ? subnetOf(address, address.prefix)
        : knownByWidth.find((candidate) => subnetContains(candidate, address));
    if (subnet) {
      const id = subnetId(subnet);
      return { id, label: id.slice(4), kind: 'subnet' };
    }
    const range = address.ip.split('.').slice(0, 3).join('.');
    return { id: `range:${range}`, label: `${range}.x`, kind: 'range' };
  };
  const placeable = (raw: string): ParsedAddress | null => {
    const parsed = parseMapAddress(raw);
    if (!parsed || containerRanges.some((range) => subnetContains(range, parsed))) return null;
    return parsed;
  };

  // Proxmox bridges first: each device votes, with its addresses, for the
  // subnet or range its bridge carries.
  interface Bridge {
    label: string;
    scope: string;
    votes: Map<string, { count: number; label: string; kind: MapSegmentKind }>;
    placement: MapBridgePlacement;
    devices: Array<{ resourceId: string; address?: string }>;
  }
  const bridges = new Map<string, Bridge>();
  const bridged = new Set<string>();
  for (const resource of resources) {
    const devices = proxmoxBridgeDevices(resource);
    if (devices.length === 0) continue;
    bridged.add(resource.id);
    devices.forEach((device) => {
      const bridge: Bridge = bridges.get(device.key) ?? {
        label: device.label,
        scope: device.scope,
        placement: { scope: device.scope, name: device.bridge, vlan: device.vlan },
        votes: new Map(),
        devices: [],
      };
      let parsed = device.addresses
        .map(placeable)
        .filter((entry): entry is ParsedAddress => entry !== null);
      // A guest with one device and a flat address list has its addresses there.
      if (parsed.length === 0 && devices.length === 1) parsed = parsedById.get(resource.id) ?? [];
      for (const address of parsed) {
        const target = addressSegment(address);
        const vote = bridge.votes.get(target.id) ?? {
          count: 0,
          label: target.label,
          kind: target.kind,
        };
        vote.count += 1;
        bridge.votes.set(target.id, vote);
      }
      bridge.devices.push({ resourceId: resource.id, address: parsed[0]?.ip });
      bridges.set(device.key, bridge);
    });
  }

  // A bridge whose guests report addresses is the network those addresses
  // name, so machines placed by address alone join the same network. That takes
  // more than half of the bridge's addresses agreeing: a bridge carrying
  // several ranges is not named after one of them. A bridge with no such
  // majority, or with no address reported on it, is a network of its own.
  const collator = new Intl.Collator(undefined, { numeric: true });
  interface Carried {
    address?: string;
    names: Map<string, Set<string>>;
    placements: MapBridgePlacement[];
  }
  const carried = new Map<string, Carried>();
  for (const [key, bridge] of [...bridges].sort(([a], [b]) => collator.compare(a, b))) {
    const leader = [...bridge.votes.entries()].sort(
      ([aId, a], [bId, b]) =>
        b.count - a.count ||
        Number(b.kind === 'subnet') - Number(a.kind === 'subnet') ||
        collator.compare(aId, bId),
    )[0];
    const cast = [...bridge.votes.values()].reduce((sum, vote) => sum + vote.count, 0);
    const winner = leader && leader[1].count * 2 > cast ? leader : undefined;
    const segmentId = winner ? winner[0] : key;
    const entry: Carried = carried.get(segmentId) ?? {
      address: winner?.[1].label,
      names: new Map(),
      placements: [],
    };
    entry.placements.push(bridge.placement);
    const scopes = entry.names.get(bridge.label) ?? new Set<string>();
    scopes.add(bridge.scope);
    entry.names.set(bridge.label, scopes);
    carried.set(segmentId, entry);
    for (const device of bridge.devices) attach(device.resourceId, segmentId, device.address);
  }

  for (const resource of resources) {
    const portGroups = vspherePortGroups(resource);
    const addresses = parsedById.get(resource.id) ?? [];
    for (const id of portGroups) {
      labels.set(id, { label: id.split(':').slice(2).join(':'), kind: 'portgroup' });
      attach(resource.id, id, addresses[0]?.ip);
    }
    // A port group or a bridge is where the device is plugged in; it stands
    // for the guest's addresses rather than being drawn beside them.
    if (portGroups.length > 0 || bridged.has(resource.id)) continue;
    for (const address of addresses) {
      const target = addressSegment(address);
      labels.set(target.id, { label: target.label, kind: target.kind });
      attach(resource.id, target.id, address.ip);
    }
  }

  // Name each bridge the way a Proxmox user says it. One bridge name on the
  // network leads, with the subnet beside it; several share the subnet's name.
  const bridgeLabelUses = new Map<string, number>();
  for (const entry of carried.values()) {
    if (entry.names.size !== 1) continue;
    const [label] = entry.names.keys();
    bridgeLabelUses.set(label, (bridgeLabelUses.get(label) ?? 0) + 1);
  }
  for (const [segmentId, entry] of carried) {
    const names = [...entry.names.keys()].sort(collator.compare);
    if (names.length === 1) {
      const scopes = [...(entry.names.get(names[0]) ?? [])].filter(Boolean);
      // Two unrelated clusters can both have a `vmbr0`; say whose this is.
      const whose =
        (bridgeLabelUses.get(names[0]) ?? 0) > 1 && scopes.length === 1 ? scopes[0] : '';
      const detail = [whose, entry.address ?? ''].filter(Boolean).join(' · ');
      labels.set(segmentId, {
        label: names[0],
        kind: 'bridge',
        ...(detail ? { detail } : {}),
        ...(entry.placements.length === 1 ? { bridge: entry.placements[0] } : {}),
      });
      continue;
    }
    const shown = names.slice(0, 2).join(', ') + (names.length > 2 ? ` +${names.length - 2}` : '');
    labels.set(segmentId, { label: entry.address ?? names[0], kind: 'bridge', detail: shown });
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
