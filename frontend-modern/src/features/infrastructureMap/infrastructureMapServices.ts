import {
  MAP_CATEGORY_LABEL,
  MAP_CATEGORY_ORDER,
  type MapAppRole,
  type MapCategoryId,
} from './infrastructureMapCatalogue';
import { mapRouteLabel } from './infrastructureMapConnections';
import type { MapGlyphKey } from './infrastructureMapLayout';
import type {
  InfrastructureMapModel,
  MapHealth,
  MapHost,
  MapItem,
  MapPool,
} from './infrastructureMapModel';
import {
  OTHER_SEGMENT_ID,
  type MapSegment,
  type MapSegmentKind,
} from './infrastructureMapNetworks';

/**
 * The estate as a service diagram: what it runs, boxed by what each thing is
 * for, between how traffic gets in at the top and the storage and hardware
 * underneath. It is a second reading of the same map model, so it draws only
 * what Pulse monitors. Two drawn things are not monitored: the Internet
 * itself, shown only when an app whose whole job is to reach it (a tunnel, a
 * VPN, a reverse proxy) is running, and a default gateway that machines name
 * in their own route tables and Pulse knows only by its address.
 *
 * A link between two tiles is a connection a collected value states: a
 * container that uses another container's network, or a reverse proxy and a
 * container it routes. No line is drawn for one. It is written under the
 * tile and on its hover, and the layout tells each end the other. Where both
 * ends are not drawn there is no link.
 */

export interface DiagramTile {
  /** A resource id, or a made-up key for a tile that stands for several. */
  id: string;
  name: string;
  caption: string;
  typeLabel: string;
  status: string;
  health: MapHealth;
  /** A logo key for an app the catalogue knows. */
  icon?: string;
  /** An outline glyph, for hardware, storage and the Internet. */
  platform?: MapGlyphKey;
  /** The first network the thing is on, as an id into `networks`. */
  segmentId?: string;
  cpu?: number;
  memory?: number;
  address?: string;
  detail?: string;
  /** What the thing's connections say about it, one sentence each, for the hover. */
  links?: string[];
  /** False for a tile that stands for several things and opens none. */
  navigable: boolean;
}

/** A stated connection between two drawn tiles. No line is drawn for it. */
export interface DiagramLink {
  fromId: string;
  toId: string;
}

export interface DiagramGroup {
  key: string;
  label: string;
  /** How many things the box holds, including any folded into one tile. */
  count: number;
  tiles: DiagramTile[];
}

export interface DiagramNetwork {
  id: string;
  label: string;
  /** What the network was read from: a Proxmox bridge counts what is plugged in. */
  kind: MapSegmentKind;
  /** The subnet a bridge carries, whose bridge it is, or the VLANs folded into it. */
  detail?: string;
  memberCount: number;
  colorIndex: number;
}

/**
 * How many networks the diagram names. Each one has a colour of its own, and
 * a tile is marked only with the colour of a network that is named, so two
 * unrelated networks never share a mark.
 */
export const DIAGRAM_MAX_NETWORKS = 6;

export interface ServiceDiagram {
  /** Set when something running exists to reach the Internet. */
  internet?: { label: string };
  /** Default gateways no monitored thing answers on, known only by address. */
  gateways: DiagramTile[];
  access?: DiagramGroup;
  links: DiagramLink[];
  networks: DiagramNetwork[];
  groups: DiagramGroup[];
  storage?: DiagramGroup;
  backups?: DiagramGroup;
  hardware?: DiagramGroup;
  serviceCount: number;
}

export interface ServiceDiagramOptions {
  /** The most tiles one group draws before the rest fold into one. */
  maxTilesPerGroup?: number;
}

export const DIAGRAM_MAX_TILES_PER_GROUP = 24;
/** The most gateway tiles drawn. An estate has one or two. */
export const DIAGRAM_MAX_GATEWAYS = 3;

const SERVICE_SECTION = /^(vms|lxcs|containers|ns:.*)$/;
const ROLE_ORDER: readonly MapAppRole[] = [
  'tunnel',
  'VPN',
  'reverse proxy',
  'DNS',
  'remote desktop',
];
// The roles that carry traffic between the estate and the Internet. DNS and
// remote desktop are ways in to the estate without being a path to it.
const INTERNET_ROLES = new Set<MapAppRole>(['tunnel', 'VPN', 'reverse proxy']);

const HEALTH_RANK: Record<MapHealth, number> = {
  critical: 5,
  attention: 4,
  stale: 3,
  unknown: 2,
  ok: 1,
  off: 0,
};
const worst = (a: MapHealth, b: MapHealth): MapHealth => (HEALTH_RANK[a] >= HEALTH_RANK[b] ? a : b);

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

const plural = (count: number, one: string, many: string): string =>
  `${count.toLocaleString()} ${count === 1 ? one : many}`;

// Up to three names, then how many more: enough to recognise a short list
// and short enough for one line of a hover.
const nameList = (names: string[]): string => {
  const sorted = [...names].sort(collator.compare);
  return sorted.length <= 3
    ? sorted.join(', ')
    : `${sorted.slice(0, 3).join(', ')} +${sorted.length - 3}`;
};

const hostsOf = (model: InfrastructureMapModel): MapHost[] =>
  model.blocks.flatMap((block) => (block.kind === 'group' ? block.group.hosts : [block.host]));

interface NetworkPlan {
  networks: DiagramNetwork[];
  /** The named network a tile on a segment is marked with. */
  networkOf: (segmentId: string | undefined) => string | undefined;
  /** The address a tile answers on, with the bridge or port group it is on. */
  where: (
    address: string | undefined,
    segmentIds: readonly string[] | undefined,
  ) => string | undefined;
}

// The tags a bridge carries, short enough for a pill. The bridge's own name
// already stands for its untagged network.
const vlanPhrase = (vlans: number[]): string => {
  const tags = vlans.filter((vlan) => vlan > 0).sort((a, b) => a - b);
  if (tags.length === 0) return '';
  const shown = tags.slice(0, 4).join(', ') + (tags.length > 4 ? ` +${tags.length - 4}` : '');
  return `${tags.length === 1 ? 'VLAN' : 'VLANs'} ${shown}`;
};

/**
 * Which networks the diagram names. Up to the cap, each network is one. Past
 * it, the VLANs of one Proxmox bridge are named together, the way its owner
 * says it ("vmbr0, VLANs 20, 30 and 40"), and what still does not fit is left
 * to the "+N more" pill, unmarked. A tile's hover always names the exact
 * bridge and VLAN, so folding hides nothing.
 */
const planNetworks = (model: InfrastructureMapModel): NetworkPlan => {
  const segments = model.segments.filter((segment) => segment.id !== OTHER_SEGMENT_ID);
  // A subnet or range only repeats the address beside it.
  const names = new Map(
    segments
      .filter((segment) => segment.kind === 'bridge' || segment.kind === 'portgroup')
      .map((segment) => [segment.id, segment.label]),
  );
  const where: NetworkPlan['where'] = (address, segmentIds) => {
    const on = (segmentIds ?? []).flatMap((id) => names.get(id) ?? []);
    return (
      [address, on.length > 0 ? `on ${on.join(' and ')}` : ''].filter(Boolean).join(' ') ||
      undefined
    );
  };
  const plain = ({ id, label, kind, detail, memberCount, colorIndex }: MapSegment) => ({
    id,
    label,
    kind,
    ...(detail ? { detail } : {}),
    memberCount,
    colorIndex,
  });
  if (segments.length <= DIAGRAM_MAX_NETWORKS) {
    return { networks: segments.map(plain), networkOf: (segmentId) => segmentId, where };
  }

  // Fold the networks of one bridge. A bridge with a single network gains
  // nothing from it and keeps its own name.
  const byBridge = new Map<string, MapSegment[]>();
  for (const segment of segments) {
    if (!segment.bridge) continue;
    const key = `bridge:${segment.bridge.scope}:${segment.bridge.name}`;
    byBridge.set(key, [...(byBridge.get(key) ?? []), segment]);
  }
  const foldOf = new Map<string, string>();
  for (const [key, members] of byBridge) {
    if (members.length < 2) continue;
    for (const member of members) foldOf.set(member.id, key);
  }
  // A guest on two VLANs of one bridge is connected to that bridge once.
  const connected = new Map<string, Set<string>>();
  const count = (id: string, segmentIds: readonly string[] | undefined) => {
    for (const segmentId of segmentIds ?? []) {
      const fold = foldOf.get(segmentId);
      if (!fold) continue;
      connected.set(fold, (connected.get(fold) ?? new Set<string>()).add(id));
    }
  };
  for (const host of hostsOf(model)) {
    count(host.id, host.ownSegmentIds);
    for (const section of host.sections) {
      for (const item of section.items) count(item.id, item.segmentIds);
    }
  }
  const folded: DiagramNetwork[] = [...byBridge]
    .filter(([, members]) => members.length > 1)
    .map(([key, members]) => ({
      id: key,
      label: members[0].bridge?.name ?? members[0].label,
      kind: 'bridge' as const,
      detail: vlanPhrase(members.map((member) => member.bridge?.vlan ?? 0)),
      memberCount:
        connected.get(key)?.size ?? members.reduce((sum, member) => sum + member.memberCount, 0),
      colorIndex: 0,
    }));
  const scopeOf = new Map(
    [...byBridge].map(([key, members]) => [key, members[0].bridge?.scope ?? '']),
  );
  const kept = segments.filter((segment) => !foldOf.has(segment.id)).map(plain);
  // Two clusters can both have a `vmbr0`; say whose each one is.
  const uses = new Map<string, number>();
  for (const network of [...folded, ...kept]) {
    uses.set(network.label, (uses.get(network.label) ?? 0) + 1);
  }
  const networks = [
    ...folded.map((network) => {
      // One fact per pill: whose bridge it is where that tells two apart,
      // otherwise the tags. A tile's hover has the exact VLAN either way.
      const whose = (uses.get(network.label) ?? 0) > 1 ? scopeOf.get(network.id) : '';
      const { detail: tags, ...rest } = network;
      const detail = whose || tags;
      return { ...rest, ...(detail ? { detail } : {}) };
    }),
    ...kept,
  ]
    .sort((a, b) => b.memberCount - a.memberCount || collator.compare(a.label, b.label))
    .map((network, index) => ({ ...network, colorIndex: index }));
  return { networks, networkOf: (segmentId) => foldOf.get(segmentId ?? '') ?? segmentId, where };
};

const serviceTile = (item: MapItem, host: MapHost, plan: NetworkPlan): DiagramTile => ({
  id: item.id,
  name: item.name,
  caption: item.app?.role ?? host.name,
  typeLabel: item.app ? `${item.app.label} · ${item.typeLabel}` : item.typeLabel,
  status: item.status,
  health: item.health,
  ...(item.app?.icon ? { icon: item.app.icon } : {}),
  ...(item.segmentIds?.[0] ? { segmentId: plan.networkOf(item.segmentIds[0]) } : {}),
  cpu: item.cpu,
  memory: item.memory,
  address: plan.where(item.address, item.segmentIds),
  detail: item.detail,
  navigable: true,
});

const foldTile = (key: string, name: string, caption: string): DiagramTile => ({
  id: key,
  name,
  caption,
  typeLabel: caption,
  status: '',
  health: 'off',
  navigable: false,
});

// Running things first, then by name, so the tiles that fold away when a
// group is large are the ones that are switched off.
const byRunningThenName = (a: DiagramTile, b: DiagramTile): number =>
  Number(a.health === 'off') - Number(b.health === 'off') || collator.compare(a.name, b.name);

const capTiles = (key: string, tiles: DiagramTile[], max: number): DiagramTile[] => {
  if (tiles.length <= max) return tiles;
  const rest = tiles.length - (max - 1);
  return [...tiles.slice(0, max - 1), foldTile(`${key}:more`, `+${rest}`, 'more')];
};

// A box of tiles, cut to size, that still knows how many things it stands for.
const box = (
  key: string,
  label: string,
  tiles: DiagramTile[],
  max: number,
  folded = 0,
): DiagramGroup => ({
  key,
  label,
  count: tiles.filter((tile) => tile.navigable).length + folded,
  tiles: capTiles(key, tiles, max),
});

const poolTiles = (
  pools: Array<{ pool: MapPool; owner: string }>,
  platform: MapGlyphKey,
): DiagramTile[] => {
  // The same pool name on several hosts is one tile: a cluster's `local` is
  // three disks, and three identical tiles would say less than one.
  const byName = new Map<string, { pool: MapPool; owners: Set<string>; health: MapHealth }>();
  for (const { pool, owner } of pools) {
    const entry = byName.get(pool.name);
    if (entry) {
      if (owner) entry.owners.add(owner);
      entry.health = worst(entry.health, pool.health);
    } else {
      byName.set(pool.name, { pool, owners: new Set(owner ? [owner] : []), health: pool.health });
    }
  }
  return [...byName.values()]
    .map(({ pool, owners, health }): DiagramTile => {
      const caption =
        owners.size > 1 ? plural(owners.size, 'host', 'hosts') : ([...owners][0] ?? 'Shared');
      return {
        id: pool.id,
        name: pool.name,
        caption,
        typeLabel: [pool.backup ? 'Backup target' : 'Storage', pool.sizeLabel]
          .filter(Boolean)
          .join(' · '),
        status: pool.usedPercent === undefined ? '' : `${Math.round(pool.usedPercent)}% used`,
        health,
        platform,
        navigable: true,
      };
    })
    .sort((a, b) => collator.compare(a.name, b.name));
};

export function buildServiceDiagram(
  model: InfrastructureMapModel,
  options: ServiceDiagramOptions = {},
): ServiceDiagram {
  const max = Math.max(2, options.maxTilesPerGroup ?? DIAGRAM_MAX_TILES_PER_GROUP);
  const byCategory = new Map<MapCategoryId, DiagramTile[]>();
  const roles = new Set<MapAppRole>();
  const pools: Array<{ pool: MapPool; owner: string }> = [];
  const hardware: DiagramTile[] = [];
  const backupServers: DiagramTile[] = [];
  const checks: DiagramTile[] = [];
  let serviceCount = 0;
  let foldedOff = 0;
  const plan = planNetworks(model);

  for (const block of model.blocks) {
    if (block.kind === 'group') {
      for (const pool of block.group.pools) pools.push({ pool, owner: block.group.name });
    }
  }
  for (const host of hostsOf(model)) {
    let services = 0;
    for (const section of host.sections) {
      if (!SERVICE_SECTION.test(section.key)) continue;
      for (const item of section.items) {
        services += 1;
        const category = item.category ?? 'other';
        // Something switched off that the catalogue cannot name says nothing
        // on a diagram of what the estate does, so those fold into one tile.
        if (category === 'other' && !item.app && item.health === 'off') {
          foldedOff += 1;
          continue;
        }
        if (item.app?.role && item.health !== 'off') roles.add(item.app.role);
        const list = byCategory.get(category) ?? [];
        list.push(serviceTile(item, host, plan));
        byCategory.set(category, list);
      }
    }
    serviceCount += services;
    // What Pulse probes from outside (a router, a site) is not a service on
    // any host here, so each check is a tile in a box of its own.
    if (host.platform === 'checks' && !host.isResource) {
      for (const section of host.sections) {
        for (const item of section.items) {
          checks.push({
            id: item.id,
            name: item.name,
            caption: item.address ?? 'check',
            typeLabel: item.typeLabel,
            status: item.status,
            health: item.health,
            platform: 'checks',
            address: item.address,
            navigable: true,
          });
        }
      }
    }
    for (const pool of host.pools) pools.push({ pool, owner: host.isResource ? host.name : '' });
    if (!host.isResource) continue;
    const tile: DiagramTile = {
      id: host.id,
      name: host.name,
      caption: services > 0 ? plural(services, 'service', 'services') : host.typeLabel,
      typeLabel: host.typeLabel,
      status: host.status,
      health: host.health,
      platform: host.platform,
      ...(host.ownSegmentIds[0] ? { segmentId: plan.networkOf(host.ownSegmentIds[0]) } : {}),
      cpu: host.cpu,
      memory: host.memory,
      address: plan.where(host.address, host.ownSegmentIds),
      navigable: true,
    };
    (host.platform === 'backup' ? backupServers : hardware).push(tile);
  }

  const group = (category: MapCategoryId): DiagramGroup | undefined => {
    const tiles = [...(byCategory.get(category) ?? [])].sort(byRunningThenName);
    const folded = category === 'other' ? foldedOff : 0;
    if (folded > 0) tiles.push(foldTile('other:off', `+${folded}`, 'switched off'));
    if (tiles.length === 0) return undefined;
    return box(category, MAP_CATEGORY_LABEL[category], tiles, max, folded);
  };

  const groups: DiagramGroup[] = [];
  for (const category of MAP_CATEGORY_ORDER) {
    if (category === 'access' || category === 'backup') continue;
    const built = group(category);
    if (built) groups.push(built);
  }
  if (checks.length > 0) groups.push(box('checks', 'Checks', checks, max));

  const storageTiles = poolTiles(
    pools.filter(({ pool }) => !pool.backup),
    'storage',
  );
  const backupTiles = [
    ...backupServers,
    ...(group('backup')?.tiles ?? []),
    ...poolTiles(
      pools.filter(({ pool }) => pool.backup),
      'backup',
    ),
  ];
  const access = group('access');
  const internetRoles = ROLE_ORDER.filter((role) => roles.has(role) && INTERNET_ROLES.has(role));

  // Everything drawn as a tile of its own, by id, so a connection can find
  // both of its ends and say something on each.
  const drawn = new Map<string, DiagramTile>();
  const boxes = [access, ...groups, { tiles: backupTiles }, { tiles: hardware }];
  for (const entry of boxes) {
    for (const tile of entry?.tiles ?? []) if (tile.navigable) drawn.set(tile.id, tile);
  }
  const say = (tile: DiagramTile, sentence: string) => {
    tile.links = [...(tile.links ?? []), sentence];
  };

  const links: DiagramLink[] = [];
  const carried = new Map<string, number>();
  const routedBy = new Map<string, number>();
  for (const link of model.connections.links) {
    const from = drawn.get(link.fromId);
    const to = drawn.get(link.toId);
    // An end folded into a `+N` tile is not drawn, so the link has no place.
    if (!from || !to) continue;
    if (link.kind === 'network') {
      say(from, `Uses the network of ${to.name}`);
      // The container it reaches the world through says more than the host
      // it runs on, which the hardware box already shows.
      from.caption = `via ${to.name}`;
      carried.set(to.id, (carried.get(to.id) ?? 0) + 1);
    } else {
      say(
        to,
        link.hosts.length > 0
          ? `Routed by ${from.name} at ${link.hosts.join(', ')}`
          : `Routed by ${from.name}`,
      );
      // Where it answers says more than the host it runs on.
      if (link.hosts.length > 0) to.caption = mapRouteLabel(link.hosts);
      routedBy.set(from.id, (routedBy.get(from.id) ?? 0) + 1);
    }
    links.push({ fromId: link.fromId, toId: link.toId });
  }
  for (const [id, count] of carried) {
    const tile = drawn.get(id);
    if (tile) say(tile, `Carries the network of ${plural(count, 'container', 'containers')}`);
  }
  for (const [id, count] of routedBy) {
    const tile = drawn.get(id);
    if (tile) say(tile, `Routes ${plural(count, 'container', 'containers')}`);
  }

  const machineNames = (ids: string[]): string[] =>
    ids.map((id) => drawn.get(id)?.name).filter((name): name is string => Boolean(name));
  const gateways: DiagramTile[] = [];
  const gatewayTiles = new Map<number, DiagramTile>();
  model.connections.gateways.forEach((gateway, index) => {
    const names = machineNames(gateway.machineIds);
    // A gateway only machines that are not drawn point at says nothing here.
    if (names.length === 0) return;
    const owner = gateway.resourceId ? drawn.get(gateway.resourceId) : undefined;
    if (owner) {
      say(owner, `Default gateway of ${nameList(names)}`);
      return;
    }
    const tile: DiagramTile = {
      id: `gateway:${index}`,
      name: gateway.address || 'Gateway',
      caption: 'default gateway',
      typeLabel: 'Default gateway, not monitored',
      status: '',
      health: 'unknown',
      platform: 'gateway',
      links: [`Used by ${nameList(names)}`],
      navigable: false,
    };
    gateways.push(tile);
    gatewayTiles.set(index, tile);
  });
  // A resolver is only said where it is already on the map: a monitored
  // thing, or a gateway with the same address. An address nothing else
  // names would be one more box of nothing.
  for (const resolver of model.connections.resolvers) {
    const owner =
      (resolver.resourceId ? drawn.get(resolver.resourceId) : undefined) ??
      (resolver.gateway === undefined ? undefined : gatewayTiles.get(resolver.gateway));
    const names = machineNames(resolver.machineIds.filter((id) => id !== resolver.resourceId));
    if (owner && names.length > 0) say(owner, `Resolves names for ${nameList(names)}`);
  }

  return {
    ...(internetRoles.length > 0 ? { internet: { label: internetRoles.join(' · ') } } : {}),
    gateways: gateways.slice(0, DIAGRAM_MAX_GATEWAYS),
    ...(access ? { access } : {}),
    links,
    networks: plan.networks,
    groups,
    ...(storageTiles.length > 0 ? { storage: box('storage', 'Storage', storageTiles, max) } : {}),
    ...(backupTiles.length > 0 ? { backups: box('backups', 'Backups', backupTiles, max) } : {}),
    ...(hardware.length > 0
      ? {
          hardware: box(
            'hardware',
            'Hardware',
            hardware.sort((a, b) => collator.compare(a.name, b.name)),
            max,
          ),
        }
      : {}),
    serviceCount,
  };
}
